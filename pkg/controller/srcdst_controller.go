package controller

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	v1 "k8s.io/api/core/v1"
	informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	client       kubernetes.Interface
	nodeInformer cache.SharedIndexInformer
	nodeSynced   cache.InformerSynced
	workqueue    workqueue.RateLimitingInterface
	recorder     record.EventRecorder
	ec2Client    *ec2.EC2
	wg           sync.WaitGroup
}

const (
	// used as the Node Annotation key
	SrcDstCheckDisabledAnnotation = "kubernetes-ec2-srcdst-controller.ottoyiu.com/srcdst-check-disabled"
)

// NewSrcDstController creates a new Kubernetes controller to monitor Kubernetes nodes and disable src-dst
// check on EC2 instances.
func NewSrcDstController(client kubernetes.Interface,
	nodeInformer informer.NodeInformer,
	ec2Client *ec2.EC2) *Controller {

	c := &Controller{
		client:       client,
		nodeInformer: nodeInformer.Informer(),
		nodeSynced:   nodeInformer.Informer().HasSynced,
		workqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Nodes"),
		ec2Client:    ec2Client,
	}

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handler,
		UpdateFunc: func(old, new interface{}) {
			c.handler(new)
		},
	})

	return c
}

func (c *Controller) Run(numWorkers int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	if ok := cache.WaitForCacheSync(stopCh, c.nodeSynced); !ok {
		return fmt.Errorf("caches have failed to sync")
	}

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		c.wg.Add(1)
		go func() {
			defer wg.Done()
			go wait.Until(c.runWorker, time.Second, stopCh)
		}()
	}

	c.wg.Wait()
	<-stopCh
	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	key, quit := c.workqueue.Get()
	if quit {
		return false
	}

	defer c.workqueue.Forget(key)
	if srcDstEnabled, err := c.checkSrcDstAttributeEnabled(key.(string)); err != nil {
		klog.Error(err)
		c.workqueue.AddRateLimited(key)
	} else if srcDstEnabled {
		return true
	}

	if err := c.disableSrcDstCheck(key.(string)); err != nil {
		c.workqueue.AddRateLimited(key)
	}

	return true
}

func (c *Controller) handler(obj interface{}) {

	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.workqueue.Add(key)
	return
}

func (c *Controller) disableSrcDstCheck(key string) error {
	defer c.workqueue.Done(key)

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		nodeObj, err := c.getNodeObjByKey(key)
		if err != nil {
			return err
		}

		nodeCopy := nodeObj.DeepCopy()
		// src dst check disabled annotation does not exist
		// call AWS ec2 api to disable
		instanceID, err := GetInstanceIDFromProviderID(nodeCopy.Spec.ProviderID)
		if err != nil {
			klog.Errorf("Failed to retrieve Instance ID from Provider ID: %v", nodeCopy.Spec.ProviderID)
			return err
		}
		if err = c.modifySrcDstCheckAttribute(*instanceID); err != nil {
			klog.Errorf("Failed to disable src dst check for EC2 instance: %v; %v", *instanceID, err)
			return err
		}

		nodeCopy.Annotations[SrcDstCheckDisabledAnnotation] = "true"
		if _, err := c.client.CoreV1().Nodes().Update(nodeCopy); err != nil {
			klog.Errorf("Failed to set %s annotation: %v", SrcDstCheckDisabledAnnotation, err)
			return err
		}

		return nil
	})
}

func (c *Controller) modifySrcDstCheckAttribute(instanceID string) error {
	_, err := c.ec2Client.ModifyInstanceAttribute(
		&ec2.ModifyInstanceAttributeInput{
			InstanceId: aws.String(instanceID),
			SourceDestCheck: &ec2.AttributeBooleanValue{
				Value: aws.Bool(false),
			},
		},
	)

	return err
}

// GetInstanceIDFromProviderID will only retrieve the InstanceID from AWS
func GetInstanceIDFromProviderID(providerID string) (*string, error) {
	// providerID is in this format: aws:///availability-zone/instanceID
	if !strings.HasPrefix(providerID, "aws") {
		return nil, fmt.Errorf("node is not in AWS EC2, skipping!")
	}
	providerID = strings.Replace(providerID, "///", "//", 1)
	url, err := url.Parse(providerID)
	if err != nil {
		return nil, fmt.Errorf("invalid providerID (%s): %v", providerID, err)
	}
	instanceID := url.Path
	instanceID = strings.Trim(instanceID, "/")

	// We sanity check the resulting volume; the two known formats are
	// i-12345678 and i-12345678abcdef01
	// TODO: Regex match?
	if strings.Contains(instanceID, "/") || !strings.HasPrefix(instanceID, "i-") {
		return nil, fmt.Errorf("invalid format for AWS instanceID (%s)", instanceID)
	}

	return &instanceID, nil
}

func (c *Controller) getNodeObjByKey(key string) (nodeObj *v1.Node, err error) {
	nodeItem, exists, err := c.nodeInformer.GetIndexer().GetByKey(key)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("node object %s doesn't exist", key)
	}

	return nodeItem.(*v1.Node), nil
}

func (c *Controller) checkSrcDstAttributeEnabled(key string) (enabled bool, err error) {
	defer c.workqueue.Done(key)

	nodeObj, err := c.getNodeObjByKey(key)
	if err != nil {
		return false, err
	}

	if nodeObj.Annotations != nil {
		if _, ok := nodeObj.Annotations[SrcDstCheckDisabledAnnotation]; ok {
			return true, nil
		}
	}

	return false, nil
}

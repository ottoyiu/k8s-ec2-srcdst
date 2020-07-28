package controller

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/golang/glog"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
)

// Controller is a Kubernetes controller that watches Node objects and disables AWS EC2
// source/destination checks on the underlying EC2 instances' attached ENIs.
type Controller struct {
	nodes      corev1.NodeInterface
	ec2Client  ec2iface.EC2API
	controller cache.Controller
}

const (
	srcDstCheckDisabledAnnotation = "kubernetes-ec2-srcdst-controller.ottoyiu.com/srcdst-check-disabled" // used as the Node annotation key
)

// NewSrcDstController creates a new Kubernetes controller using client-go's Informer.
func NewSrcDstController(client kubernetes.Interface, ec2Client *ec2.EC2) *Controller {
	c := &Controller{
		nodes:     client.CoreV1().Nodes(),
		ec2Client: ec2Client,
	}

	nodeListWatcher := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		"nodes",
		v1.NamespaceAll,
		fields.Everything())

	_, c.controller = cache.NewInformer(
		nodeListWatcher,
		&v1.Node{},
		60*time.Second,
		// Callback functions to trigger when nodes are added or updated:
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handler,
			UpdateFunc: func(old, new interface{}) { c.handler(new) },
		},
	)

	return c
}

// RunUntil sets this controller running, processing events observed via the Kubernetes API until
// the supplied channel is closed.
func (c *Controller) RunUntil(stop <-chan struct{}) {
	c.controller.Run(stop)
}

func (c *Controller) handler(obj interface{}) {
	ctx := context.TODO()
	node, ok := obj.(*v1.Node)
	if !ok {
		glog.Errorf("Expected Node but handler received: %+v", obj)
		return
	}
	glog.V(4).Infof("Received update of node: %s", node.Name)
	c.disableSrcDstIfEnabled(ctx, node)
}

func (c *Controller) disableSrcDstIfEnabled(ctx context.Context, node *v1.Node) {
	srcDstCheckEnabled := true
	if node.Annotations != nil {
		if _, ok := node.Annotations[srcDstCheckDisabledAnnotation]; ok {
			srcDstCheckEnabled = false
		}
	}

	if srcDstCheckEnabled {
		// The "source/destination check disabled" annotation does not exist.
		// Call the AWS EC2 API to disable the check.
		instanceID, err := getInstanceIDFromProviderID(node.Spec.ProviderID)
		if err != nil {
			glog.Errorf("Fail to retrieve Instance ID from Provider ID: %v", node.Spec.ProviderID)
			return
		}
		err = c.disableSrcDstCheck(*instanceID)
		if err != nil {
			glog.Errorf("Failed to disable source/destination check for EC2 instance: %v; %v", *instanceID, err)
			return
		}
		glog.Infof("Marking node %s with SrcDstCheckDisabledAnnotation", node.Name)
		// We should not modify the cache object directly, so we make a copy first.
		nodeCopy := node.DeepCopy()
		annotations := nodeCopy.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string, 1)
			nodeCopy.SetAnnotations(annotations)
		}
		annotations[srcDstCheckDisabledAnnotation] = "true"
		if _, err := c.nodes.Update(ctx, nodeCopy, metav1.UpdateOptions{}); err != nil {
			glog.Errorf("Failed to set %s annotation: %v", srcDstCheckDisabledAnnotation, err)
		}
	} else {
		glog.V(4).Infof("Skipping node %s because it already has the SrcDstCheckDisabledAnnotation", node.Name)

	}
}

func (c *Controller) disableSrcDstCheck(instanceID string) error {
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

// getInstanceIDFromProviderID will retrieves the instance ID from a provider ID, so long as the
// provider is AWS.
func getInstanceIDFromProviderID(providerID string) (*string, error) {
	// providerID is in this format: aws:///availability-zone/instanceID
	// TODO: why the extra slash in the provider ID of Kubernetes anyways?
	if !strings.HasPrefix(providerID, "aws") {
		return nil, fmt.Errorf("node is not hosted in AWS EC2; skipping")
	}
	providerID = strings.Replace(providerID, "///", "//", 1)
	url, err := url.Parse(providerID)
	if err != nil {
		return nil, fmt.Errorf("invalid providerID %q: %v", providerID, err)
	}
	instanceID := url.Path
	instanceID = strings.Trim(instanceID, "/")

	// We sanity check the resulting volume; the two known formats are
	// i-12345678 and i-12345678abcdef01.
	// TODO: Regex match?
	if strings.Contains(instanceID, "/") || !strings.HasPrefix(instanceID, "i-") {
		return nil, fmt.Errorf("invalid format for AWS instance ID: %q", instanceID)
	}

	return &instanceID, nil
}

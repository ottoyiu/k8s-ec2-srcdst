package controller

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/mattbaird/jsonpatch"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/ottoyiu/k8s-ec2-srcdst/pkg/common"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Controller struct {
	client     kubernetes.Interface
	Controller cache.Controller
	ec2Client  ec2iface.EC2API
	my_opts    *common.K8sEc2SrcdstOpts
}

const (
	SrcDstCheckDisabledAnnotation = "kubernetes-ec2-srcdst-controller.ottoyiu.com/srcdst-check-disabled" // used as the Node Annotation key
)

// NewSrcDstController creates a new Kubernetes controller using client-go's Informer
func NewSrcDstController(client kubernetes.Interface, ec2Client *ec2.EC2, my_opts *common.K8sEc2SrcdstOpts) *Controller {
	c := &Controller{
		client:    client,
		ec2Client: ec2Client,
		my_opts:   my_opts,
	}

	nodeListWatcher := cache.NewListWatchFromClient(
		client.Core().RESTClient(),
		"nodes",
		v1.NamespaceAll,
		fields.Everything())

	_, controller := cache.NewInformer(
		nodeListWatcher,
		&v1.Node{},
		60*time.Second,
		// Callback Functions to trigger on add/update/delete
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handler,
			UpdateFunc: func(old, new interface{}) { c.handler(new) },
		},
	)

	c.Controller = controller

	return c
}

func (c *Controller) handler(obj interface{}) {
	// this handler makes sure that all nodes within a cluster has its src/dst check disabled in EC2
	node, ok := obj.(*v1.Node)
	if !ok {
		glog.Errorf("Expected Node but handler received: %+v", obj)
		return
	}
	glog.V(4).Infof("Received update of node: %s", node.Name)
	c.disableSrcDstIfEnabled(node, obj.(*common.K8sEc2SrcdstOpts))
}

func (c *Controller) disableSrcDstIfEnabled(node *v1.Node, my_opts *common.K8sEc2SrcdstOpts) {
	srcDstCheckEnabled := true
	if node.Annotations != nil {
		if _, ok := node.Annotations[SrcDstCheckDisabledAnnotation]; ok {
			srcDstCheckEnabled = false
		}
	}

	if srcDstCheckEnabled {
		// src dst check disabled annotation does not exist
		// call AWS ec2 api to disable
		instanceID, err := GetInstanceIDFromProviderID(node.Spec.ProviderID)
		if err != nil {
			glog.Errorf("Fail to retrieve Instance ID from Provider ID: %v", node.Spec.ProviderID)
			return
		}
		err = c.disableSrcDstCheck(*instanceID)
		if err != nil {
			glog.Errorf("Fail to disable src dst check for EC2 instance: %v; %v", *instanceID, err)
			return
		}
		// We should not modify the cache object directly, so we make a copy first
		nodeCopy, err := common.CopyObjToNode(node)
		if err != nil {
			glog.Errorf("Failed to make copy of node: %v", err)
			return
		}

		glog.Infof("Marking node %s with SrcDstCheckDisabledAnnotation", node.Name)
		nodeCopy.Annotations[SrcDstCheckDisabledAnnotation] = "true"
		if my_opts.Patchnode {
			if my_opts.Verbose {
				glog.Infof("Patching node %s", nodeCopy.Name)
			}
			// Thanks to https://github.com/tamalsaha/patch-demo/blob/master/main.go#L113 for this stanza
			// Prep JSON for the newly changed node object
			json_nodeCopy, err := json.Marshal(nodeCopy)
			if err != nil {
				glog.Error(err)
			}
			// Prep JSON for a copy of the old node object
			nodeCopyOrig, err := common.CopyObjToNode(node)
			if err != nil {
				glog.Errorf("Failed to make an original copy of node: %v", err)
				return
			}
			json_nodeCopyOrig, err := json.Marshal(nodeCopyOrig)
			if err != nil {
				glog.Error(err)
			}
			// Prepare the JSON patch
			patch, err := jsonpatch.CreatePatch(json_nodeCopyOrig, json_nodeCopy)
			if err != nil {
				glog.Error(err)
			}
			// Fix indenting
			json_patch, err := json.MarshalIndent(patch, "", "  ")
			if err != nil {
				glog.Error(err)
			}
			if my_opts.Verbose {
				glog.Info(string(json_patch))
			}
			// https://github.com/kubernetes/client-go/blob/master/kubernetes/typed/core/v1/node.go#L170
			if _, err := c.client.Core().Nodes().Patch(nodeCopy.Name, types.JSONPatchType, json_patch); err != nil {
				glog.Errorf("Failed to patch %s annotation: %v", SrcDstCheckDisabledAnnotation, err)
			}
		} else {
			if _, err := c.client.Core().Nodes().Update(nodeCopy); err != nil {
				glog.Errorf("Failed to set %s annotation: %v", SrcDstCheckDisabledAnnotation, err)
			}
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

// GetInstanceIDFromProviderID will only retrieve the InstanceID from AWS
func GetInstanceIDFromProviderID(providerID string) (*string, error) {
	// providerID is in this format: aws:///availability-zone/instanceID
	// TODO: why the extra slash in the provider ID of kubernetes anyways?
	if !strings.HasPrefix(providerID, "aws") {
		return nil, fmt.Errorf("Node is not in AWS EC2, skipping...")
	}
	providerID = strings.Replace(providerID, "///", "//", 1)
	url, err := url.Parse(providerID)
	if err != nil {
		return nil, fmt.Errorf("Invalid providerID (%s): %v", providerID, err)
	}
	instanceID := url.Path
	instanceID = strings.Trim(instanceID, "/")

	// We sanity check the resulting volume; the two known formats are
	// i-12345678 and i-12345678abcdef01
	// TODO: Regex match?
	if strings.Contains(instanceID, "/") || !strings.HasPrefix(instanceID, "i-") {
		return nil, fmt.Errorf("Invalid format for AWS instanceID (%s)", instanceID)
	}

	return &instanceID, nil
}

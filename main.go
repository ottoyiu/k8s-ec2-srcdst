package main

import (
	"flag"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"github.com/ottoyiu/calico-ec2-srcdst-controller/common"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/runtime"
	"k8s.io/client-go/pkg/util/wait"
	"k8s.io/client-go/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type Controller struct {
	client     kubernetes.Interface
	controller cache.ControllerInterface
	ec2Client  *ec2.EC2
}

const (
	SrcDstCheckDisabledAnnotation = "srcdst-controller.v1/srcdst-check-disabled"
)

func main() {
	kubeconfig := flag.String("kubeconfig", "", "Path to a kubeconfig file")
	flag.Set("logtostderr", "true")
	flag.Parse()

	// Build the client config - optionally using a provided kubeconfig file.
	config, err := common.GetClientConfig(*kubeconfig)
	if err != nil {
		glog.Fatalf("Failed to load client config: %v", err)
	}

	// Construct the Kubernetes client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create kubernetes client: %v", err)
	}

	awsSession := session.New()
	awsConfig := &aws.Config{}
	ec2Client := ec2.New(awsSession, awsConfig)

	newController(client, ec2Client).controller.Run(wait.NeverStop)
}

func newController(client kubernetes.Interface, ec2Client *ec2.EC2) *Controller {
	c := &Controller{
		client:    client,
		ec2Client: ec2Client,
	}

	_, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(alo api.ListOptions) (runtime.Object, error) {
				var lo v1.ListOptions
				v1.Convert_api_ListOptions_To_v1_ListOptions(&alo, &lo, nil)
				return client.Core().Nodes().List(lo)
			},
			WatchFunc: func(alo api.ListOptions) (watch.Interface, error) {
				var lo v1.ListOptions
				v1.Convert_api_ListOptions_To_v1_ListOptions(&alo, &lo, nil)
				return client.Core().Nodes().Watch(lo)
			},
		},
		&v1.Node{},
		60*time.Second,
		// Callback Functions to trigger on add/update/delete
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handler,
			UpdateFunc: func(old, new interface{}) { c.handler(new) },
			DeleteFunc: c.handler,
		},
	)

	c.controller = controller

	return c
}

func (c *Controller) handler(obj interface{}) {
	// this handler disables src dest
	node := obj.(*v1.Node)
	glog.V(4).Infof("Received update of node: %s", node.Name)

	srcDstCheckEnabled := true
	if node.Annotations != nil {
		if _, ok := node.Annotations[SrcDstCheckDisabledAnnotation]; ok {
			srcDstCheckEnabled = false
		}
	}

	if srcDstCheckEnabled {
		// src dst check disabled annotation does not exist
		// call AWS ec2 api to disable
		instanceID, err := getInstanceIDFromProviderID(node.Spec.ProviderID)
		if err != nil {
			glog.Errorf("Fail to retrieve Instance ID from Provider ID: %v", node.Spec.ProviderID)
			return
		}
		err = c.disableSrcDstCheck(instanceID)
		if err != nil {
			glog.Errorf("Fail to disable src dst check for EC2 instance: %v; %v", instanceID, err)
			return
		}
		// We should not modify the cache object directly, so we make a copy first
		nodeCopy, err := common.CopyObjToNode(node)
		if err != nil {
			glog.Errorf("Failed to make copy of node: %v", err)
			return
		}
		glog.Infof("Marking node %s with SrcDstCheckDisabledAnnotation", node.Name)
		nodeCopy.Annotations[SrcDstCheckDisabledAnnotation] = ""
		if _, err := c.client.Core().Nodes().Update(nodeCopy); err != nil {
			glog.Errorf("Failed to set %s annotation: %v", SrcDstCheckDisabledAnnotation, err)
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

func getInstanceIDFromProviderID(providerID string) (string, error) {
	// providerID is in this format: aws:///availability-zone/instanceID
	providerID = strings.Replace(providerID, "///", "//", 1)
	url, err := url.Parse(providerID)
	if err != nil {
		return "", fmt.Errorf("Invalid providerID (%s): %v", providerID, err)
	}
	instanceID := url.Path
	instanceID = strings.Trim(instanceID, "/")

	// We sanity check the resulting volume; the two known formats are
	// i-12345678 and i-12345678abcdef01
	// TODO: Regex match?
	if strings.Contains(instanceID, "/") || !strings.HasPrefix(instanceID, "i-") {
		return "", fmt.Errorf("Invalid format for AWS instanceID (%s)", instanceID)
	}

	return instanceID, nil
}

package common

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sEc2SrcdstOpts struct {
	Patchnode bool
}

// GetClientConfig gets the credentials necessary to connect to the Kubernetes
// cluster either through the specified kubeconfig or to get the necessary info
// from the running pod within the cluster
func GetClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

// CopyObjToNode copies a Node object, so that no changes would be done
// to the original Node which is part of the cache
func CopyObjToNode(obj interface{}) (*v1.Node, error) {
	objCopy, err := runtime.NewScheme().Copy(obj.(*v1.Node))
	if err != nil {
		return nil, err
	}

	node := objCopy.(*v1.Node)
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	return node, nil
}

package controller

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"
)

type mockEC2Client struct {
	ec2iface.EC2API
	res           *ec2.ModifyInstanceAttributeOutput
	err           error
	CalledCounter int
}

func (c *mockEC2Client) ModifyInstanceAttribute(*ec2.ModifyInstanceAttributeInput) (*ec2.ModifyInstanceAttributeOutput, error) {
	c.CalledCounter = c.CalledCounter + 1
	return c.res, c.err
}

func NewMockEC2Client() *mockEC2Client {
	return &mockEC2Client{CalledCounter: 0}
}

func TestDisableSrcDstIfEnabled(t *testing.T) {
	annotations := map[string]string{SrcDstCheckDisabledAnnotation: "true"}
	masterTaint := &v1.Taint{
		Key:    "node-role.kubernetes.io/master",
		Effect: v1.TaintEffectNoSchedule,
	}
	node0 := &v1.Node{Spec: v1.NodeSpec{ProviderID: "aws:///us-mock-1/i-abcdefgh", Taints: []v1.Taint{*masterTaint}}, ObjectMeta: metav1.ObjectMeta{Name: "node0", UID: "01"}}
	node1 := &v1.Node{Spec: v1.NodeSpec{ProviderID: "aws:///us-mock-1/i-bcdefdaf", Taints: []v1.Taint{*masterTaint}}, ObjectMeta: metav1.ObjectMeta{Name: "node1", UID: "02"}}
	node1.Annotations = annotations

	var tests = []struct {
		node                     *v1.Node
		disableSrcDstCheckCalled bool
	}{
		{node0, true},
		{node1, false},
	}

	ec2Client := NewMockEC2Client()
	kubeClient := fake.NewSimpleClientset(&v1.NodeList{Items: []v1.Node{*node0, *node1}})

	c := &Controller{
		ec2Client: ec2Client,
		client:    kubeClient,
	}

	for _, tt := range tests {
		calledCount := ec2Client.CalledCounter
		c.disableSrcDstIfEnabled(tt.node)
		called := (ec2Client.CalledCounter - calledCount) > 0
		assert.Equal(
			t,
			called,
			tt.disableSrcDstCheckCalled,
			"Verify that ModifyInstanceAttribute will get called if node needs srcdstcheck disabled",
		)
	}

	// Validate that node did get updated with SrcDstCheckDisabledAnnotation
	updatedNodes, err := kubeClient.Core().Nodes().List(metav1.ListOptions{})
	assert.Nil(t, err)
	for _, updatedNode := range updatedNodes.Items {
		assert.NotEmpty(t, updatedNode.Annotations)
		assert.NotNil(t, updatedNode.Annotations[SrcDstCheckDisabledAnnotation])

		// K8s 1.6 support; ensure that taints still exists and have not been touched
		assert.NotEmpty(t, updatedNode.Spec.Taints)
		assert.Equal(t, updatedNode.Spec.Taints[0], *masterTaint)
	}
}

func TestGetInstanceIDFromProviderID(t *testing.T) {

	var tests = []struct {
		providerID         string
		expectedInstanceID string
		expectedError      bool
	}{
		{"aws:///us-west-2a/i-09fc5a0ae524b0333", "i-09fc5a0ae524b0333", false},
		{"aws://us-west-2a/i-a123hd52", "i-a123hd52", false},
		{"gce://us-west-1a/test", "", true},
		{"this_will_fail", "", true},
		{"i-a123hd52", "", true},
	}

	for _, tt := range tests {
		instanceID, err := GetInstanceIDFromProviderID(tt.providerID)
		if !tt.expectedError {
			assert.Equal(
				t,
				tt.expectedInstanceID,
				*instanceID,
				"Check if instance ID is parsed out correctly from provider ID",
			)
		} else {
			assert.NotNil(
				t,
				err,
				err.Error(),
			)
		}
	}

}

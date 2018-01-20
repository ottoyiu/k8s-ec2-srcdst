package controller

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type mockEC2Client struct {
	ec2iface.EC2API
	instancesMap              map[string]*ec2.Instance
	DescribeCounter           int
	ModifyCounter             int
	ModifyNetworkInterfaceIds []string
}

func (c *mockEC2Client) DescribeInstances(req *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	c.DescribeCounter++
	selected := make([]*ec2.Instance, 0)
	if req != nil {
		for _, instanceID := range req.InstanceIds {
			if instance, ok := c.instancesMap[aws.StringValue(instanceID)]; ok {
				selected = append(selected, instance)
			}
		}
	}
	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			&ec2.Reservation{
				Instances: selected,
			},
		},
	}, nil
}

func (c *mockEC2Client) ModifyNetworkInterfaceAttribute(req *ec2.ModifyNetworkInterfaceAttributeInput) (*ec2.ModifyNetworkInterfaceAttributeOutput, error) {
	c.ModifyCounter++
	c.ModifyNetworkInterfaceIds = append(c.ModifyNetworkInterfaceIds, aws.StringValue(req.NetworkInterfaceId))
	return &ec2.ModifyNetworkInterfaceAttributeOutput{}, nil
}

func NewMockEC2Client(instances []*ec2.Instance) *mockEC2Client {
	instancesMap := make(map[string]*ec2.Instance)
	for _, instance := range instances {
		instancesMap[aws.StringValue(instance.InstanceId)] = instance
	}
	return &mockEC2Client{
		DescribeCounter: 0,
		ModifyCounter:   0,
		instancesMap:    instancesMap,
	}
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
	node2 := &v1.Node{Spec: v1.NodeSpec{ProviderID: "aws:///us-mock-1/i-fedbca00", Taints: []v1.Taint{*masterTaint}}, ObjectMeta: metav1.ObjectMeta{Name: "node2", UID: "03"}}

	instances := []*ec2.Instance{
		// node0
		&ec2.Instance{
			InstanceId: aws.String("i-abcdefgh"),
			NetworkInterfaces: []*ec2.InstanceNetworkInterface{
				&ec2.InstanceNetworkInterface{
					NetworkInterfaceId: aws.String("eni-1234567"),
					SourceDestCheck:    aws.Bool(true),
				},
				&ec2.InstanceNetworkInterface{
					NetworkInterfaceId: aws.String("eni-890abcd"),
					SourceDestCheck:    aws.Bool(false),
				},
			},
		},
		// node1
		&ec2.Instance{
			InstanceId: aws.String("i-bcdefdaf"),
			NetworkInterfaces: []*ec2.InstanceNetworkInterface{
				&ec2.InstanceNetworkInterface{
					NetworkInterfaceId: aws.String("eni-9876543"),
					SourceDestCheck:    aws.Bool(false),
				},
			},
		},
		// node2
		&ec2.Instance{
			InstanceId: aws.String("i-fedbca00"),
			NetworkInterfaces: []*ec2.InstanceNetworkInterface{
				&ec2.InstanceNetworkInterface{
					NetworkInterfaceId: aws.String("eni-1a2b3c4d"),
					SourceDestCheck:    aws.Bool(true),
				},
				&ec2.InstanceNetworkInterface{
					NetworkInterfaceId: aws.String("eni-d4c3b2a1"),
					SourceDestCheck:    aws.Bool(true),
				},
			},
		},
	}

	var tests = []struct {
		node              *v1.Node
		describeCallCount int
		modifyCallCount   int
	}{
		{node0, 1, 1},
		{node1, 0, 0},
		{node2, 1, 2},
	}

	kubeClient := fake.NewSimpleClientset(&v1.NodeList{Items: []v1.Node{*node0, *node1, *node2}})

	for _, tt := range tests {
		ec2Client := NewMockEC2Client(instances)
		c := &Controller{
			ec2Client: ec2Client,
			client:    kubeClient,
		}

		c.disableSrcDstIfEnabled(tt.node)
		assert.Equal(
			t,
			tt.describeCallCount,
			ec2Client.DescribeCounter,
			"Verify that DescribeInstances will get called if node %q needs srcdstcheck disabled",
			tt.node.ObjectMeta.Name,
		)
		assert.Equal(
			t,
			tt.modifyCallCount,
			ec2Client.ModifyCounter,
			"Verify that ModifyNetworkInterfaceAttribute will get called if node %q needs srcdstcheck disabled",
			tt.node.ObjectMeta.Name,
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

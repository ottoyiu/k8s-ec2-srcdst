package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetInstanceIDFromProviderID(t *testing.T) {

	var tests = []struct {
		providerID         string
		expectedInstanceID string
		expectedError      bool
	}{
		{"aws:///us-west-2a/i-09fc5a0ae524b0333", "i-09fc5a0ae524b0333", false},
		{"aws://us-west-2a/i-a123hd52", "i-a123hd52", false},
		{"gce://us-west-1a/test", "", true},
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
			)
		}
	}

}

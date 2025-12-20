package controller

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestSerialization(t *testing.T) {
	// 1. Create a Policy with params
	jsonParams := `{"foo":"bar","baz":123}`
	var params runtime.RawExtension
	if err := json.Unmarshal([]byte(jsonParams), &params); err != nil {
		t.Fatalf("Failed to unmarshal params: %v", err)
	}

	apiConfig := &apiv1.RestApi{
		Spec: apiv1.APIConfigData{
			Policies: []apiv1.Policy{
				{
					Name:   "test-policy",
					Params: &params,
				},
			},
		},
	}

	// 2. Simulate the serialization logic from RestApiReconciler.executeDeployment
	// Copy-pasting logic here because executeDeployment is private and requires setup
	cleanPayload := struct {
		ApiVersion string              `yaml:"apiVersion" json:"apiVersion"`
		Kind       string              `yaml:"kind" json:"kind"`
		Metadata   map[string]string   `yaml:"metadata" json:"metadata"`
		Spec       apiv1.APIConfigData `yaml:"spec" json:"spec"`
	}{
		ApiVersion: "v1alpha1",
		Kind:       "RestApi",
		Metadata: map[string]string{
			"name": "test-api",
		},
		Spec: apiConfig.Spec,
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(cleanPayload)
	assert.NoError(t, err)

	// Unmarshal to map
	var genericMap map[string]interface{}
	err = json.Unmarshal(jsonBytes, &genericMap)
	assert.NoError(t, err)

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(genericMap)
	assert.NoError(t, err)

	yamlStr := string(yamlBytes)

	// 3. Verify output
	t.Logf("YAML Output:\n%s", yamlStr)

	// Should NOT contain "raw" or "object" keys under params
	assert.NotContains(t, yamlStr, "raw:")
	assert.NotContains(t, yamlStr, "object:")

	// Should contain the actual values
	assert.Contains(t, yamlStr, "foo: bar")
	assert.Contains(t, yamlStr, "baz: 123")
}

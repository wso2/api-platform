/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package config

import (
	"testing"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

func TestPolicyValidator_ValidatePolicies_Success(t *testing.T) {
	// Setup policy definitions
	policyDefs := map[string]api.PolicyDefinition{
		"APIKeyValidation|v1.0.0": {
			Name:    "APIKeyValidation",
			Version: "v1.0.0",
			Flows: api.PolicyDefinition_Flows{
				Request: &api.PolicyFlowRequirements{
					RequireHeader: boolPtr(true),
				},
			},
			ParametersSchema: &map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"header": map[string]interface{}{
						"type": "string",
					},
					"mandatory": map[string]interface{}{
						"type": "boolean",
					},
				},
				"required": []interface{}{"header"},
			},
		},
	}

	validator := NewPolicyValidator(policyDefs)

	// Create API config with valid policy
	apiConfig := &api.APIConfiguration{
		Version: "api-platform.wso2.com/v1",
		Kind:    "http/rest",
		Data: api.APIConfigData{
			Name:    "Test API",
			Version: "v1.0",
			Context: "/test",
			Upstream: []api.Upstream{
				{Url: "http://backend.example.com"},
			},
			Policies: &[]api.Policy{
				{
					Name:    "APIKeyValidation",
					Version: "v1.0.0",
					Params: &map[string]interface{}{
						"header":    "X-API-Key",
						"mandatory": true,
					},
				},
			},
			Operations: []api.Operation{
				{
					Method: "GET",
					Path:   "/resource",
				},
			},
		},
	}

	errors := validator.ValidatePolicies(apiConfig)
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors, got %d: %v", len(errors), errors)
	}
}

func TestPolicyValidator_PolicyNotFound(t *testing.T) {
	// Empty policy definitions
	policyDefs := map[string]api.PolicyDefinition{}
	validator := NewPolicyValidator(policyDefs)

	// Create API config with non-existent policy
	apiConfig := &api.APIConfiguration{
		Version: "api-platform.wso2.com/v1",
		Kind:    "http/rest",
		Data: api.APIConfigData{
			Name:    "Test API",
			Version: "v1.0",
			Context: "/test",
			Upstream: []api.Upstream{
				{Url: "http://backend.example.com"},
			},
			Policies: &[]api.Policy{
				{
					Name:    "NonExistentPolicy",
					Version: "v1.0.0",
				},
			},
			Operations: []api.Operation{
				{
					Method: "GET",
					Path:   "/resource",
				},
			},
		},
	}

	errors := validator.ValidatePolicies(apiConfig)
	if len(errors) != 1 {
		t.Errorf("Expected 1 validation error, got %d", len(errors))
	}
	if len(errors) > 0 && !contains(errors[0].Message, "not found") {
		t.Errorf("Expected 'not found' error, got: %s", errors[0].Message)
	}
}

func TestPolicyValidator_InvalidParameters(t *testing.T) {
	// Setup policy definition with schema
	policyDefs := map[string]api.PolicyDefinition{
		"APIKeyValidation|v1.0.0": {
			Name:    "APIKeyValidation",
			Version: "v1.0.0",
			Flows: api.PolicyDefinition_Flows{
				Request: &api.PolicyFlowRequirements{
					RequireHeader: boolPtr(true),
				},
			},
			ParametersSchema: &map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"header": map[string]interface{}{
						"type": "string",
					},
					"mandatory": map[string]interface{}{
						"type": "boolean",
					},
				},
				"required": []interface{}{"header"},
			},
		},
	}

	validator := NewPolicyValidator(policyDefs)

	// Create API config with invalid params (missing required field)
	apiConfig := &api.APIConfiguration{
		Version: "api-platform.wso2.com/v1",
		Kind:    "http/rest",
		Data: api.APIConfigData{
			Name:    "Test API",
			Version: "v1.0",
			Context: "/test",
			Upstream: []api.Upstream{
				{Url: "http://backend.example.com"},
			},
			Policies: &[]api.Policy{
				{
					Name:    "APIKeyValidation",
					Version: "v1.0.0",
					Params: &map[string]interface{}{
						"mandatory": true,
						// Missing required "header" field
					},
				},
			},
			Operations: []api.Operation{
				{
					Method: "GET",
					Path:   "/resource",
				},
			},
		},
	}

	errors := validator.ValidatePolicies(apiConfig)
	if len(errors) == 0 {
		t.Error("Expected validation errors for missing required parameter")
	}
}

func TestPolicyValidator_OperationLevelPolicies(t *testing.T) {
	// Setup policy definitions
	policyDefs := map[string]api.PolicyDefinition{
		"RateLimiting|v1.0.0": {
			Name:    "RateLimiting",
			Version: "v1.0.0",
			Flows: api.PolicyDefinition_Flows{
				Request: &api.PolicyFlowRequirements{
					RequireHeader: boolPtr(true),
				},
			},
			ParametersSchema: &map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"rate": map[string]interface{}{
						"type": "integer",
					},
				},
				"required": []interface{}{"rate"},
			},
		},
	}

	validator := NewPolicyValidator(policyDefs)

	// Create API config with operation-level policy
	apiConfig := &api.APIConfiguration{
		Version: "api-platform.wso2.com/v1",
		Kind:    "http/rest",
		Data: api.APIConfigData{
			Name:    "Test API",
			Version: "v1.0",
			Context: "/test",
			Upstream: []api.Upstream{
				{Url: "http://backend.example.com"},
			},
			Operations: []api.Operation{
				{
					Method: "GET",
					Path:   "/resource",
					Policies: &[]api.Policy{
						{
							Name:    "RateLimiting",
							Version: "v1.0.0",
							Params: &map[string]interface{}{
								"rate": 100,
							},
						},
					},
				},
			},
		},
	}

	errors := validator.ValidatePolicies(apiConfig)
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors, got %d: %v", len(errors), errors)
	}
}

func TestPolicyValidator_MultipleErrors(t *testing.T) {
	// Setup one valid policy definition
	policyDefs := map[string]api.PolicyDefinition{
		"ValidPolicy|v1.0.0": {
			Name:    "ValidPolicy",
			Version: "v1.0.0",
			Flows: api.PolicyDefinition_Flows{
				Request: &api.PolicyFlowRequirements{
					RequireHeader: boolPtr(true),
				},
			},
		},
	}

	validator := NewPolicyValidator(policyDefs)

	// Create API config with multiple invalid policies
	apiConfig := &api.APIConfiguration{
		Version: "api-platform.wso2.com/v1",
		Kind:    "http/rest",
		Data: api.APIConfigData{
			Name:    "Test API",
			Version: "v1.0",
			Context: "/test",
			Upstream: []api.Upstream{
				{Url: "http://backend.example.com"},
			},
			Policies: &[]api.Policy{
				{
					Name:    "NonExistent1",
					Version: "v1.0.0",
				},
				{
					Name:    "NonExistent2",
					Version: "v1.0.0",
				},
			},
			Operations: []api.Operation{
				{
					Method: "GET",
					Path:   "/resource",
					Policies: &[]api.Policy{
						{
							Name:    "NonExistent3",
							Version: "v1.0.0",
						},
					},
				},
			},
		},
	}

	errors := validator.ValidatePolicies(apiConfig)
	if len(errors) != 3 {
		t.Errorf("Expected 3 validation errors, got %d", len(errors))
	}
}

func TestPolicyValidator_TypeMismatch(t *testing.T) {
	// Setup policy definition expecting integer
	policyDefs := map[string]api.PolicyDefinition{
		"TestPolicy|v1.0.0": {
			Name:    "TestPolicy",
			Version: "v1.0.0",
			Flows: api.PolicyDefinition_Flows{
				Request: &api.PolicyFlowRequirements{
					RequireHeader: boolPtr(true),
				},
			},
			ParametersSchema: &map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"count": map[string]interface{}{
						"type": "integer",
					},
				},
			},
		},
	}

	validator := NewPolicyValidator(policyDefs)

	// Create API config with wrong type (string instead of integer)
	apiConfig := &api.APIConfiguration{
		Version: "api-platform.wso2.com/v1",
		Kind:    "http/rest",
		Data: api.APIConfigData{
			Name:    "Test API",
			Version: "v1.0",
			Context: "/test",
			Upstream: []api.Upstream{
				{Url: "http://backend.example.com"},
			},
			Policies: &[]api.Policy{
				{
					Name:    "TestPolicy",
					Version: "v1.0.0",
					Params: &map[string]interface{}{
						"count": "not-a-number",
					},
				},
			},
			Operations: []api.Operation{
				{
					Method: "GET",
					Path:   "/resource",
				},
			},
		},
	}

	errors := validator.ValidatePolicies(apiConfig)
	if len(errors) == 0 {
		t.Error("Expected validation error for type mismatch")
	}
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && s[:len(substr)] == substr || stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

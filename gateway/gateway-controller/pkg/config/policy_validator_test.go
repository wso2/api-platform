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
	"strings"
	"testing"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

func TestPolicyValidator_ValidatePolicies_Success(t *testing.T) {
	// Setup policy definitions
	policyDefs := map[string]api.PolicyDefinition{
		"APIKeyValidation|v1.0.0": {
			Name:    "APIKeyValidation",
			Version: "v1.0.0",
			Parameters: &map[string]interface{}{
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

	specUnion := api.APIConfiguration_Spec{}
	if err := specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName:    "Test API",
		Version: "v1.0",
		Context: "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Policies: &[]api.Policy{
			{
				Name:    "APIKeyValidation",
				Version: "v1",
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
	}); err != nil {
		t.Fatalf("Failed to create API config data: %v", err)
	}
	// Create API config with valid policy
	apiConfig := &api.APIConfiguration{
		ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:    api.RestApi,
		Spec:    specUnion,
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

	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName:    "Test API",
		Version: "v1.0",
		Context: "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Policies: &[]api.Policy{
			{
				Name:    "NonExistentPolicy",
				Version: "v1",
			},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/resource",
			},
		},
	})

	// Create API config with non-existent policy
	apiConfig := &api.APIConfiguration{
		ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:    api.RestApi,
		Spec:    specUnion,
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
			Parameters: &map[string]interface{}{
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
	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName:    "Test API",
		Version: "v1.0",
		Context: "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Policies: &[]api.Policy{
			{
				Name:    "APIKeyValidation",
				Version: "v1",
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
	})
	apiConfig := &api.APIConfiguration{
		ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:    api.RestApi,
		Spec:    specUnion,
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
			Parameters: &map[string]interface{}{
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
	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName:    "Test API",
		Version: "v1.0",
		Context: "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/resource",
				Policies: &[]api.Policy{
					{
						Name:    "RateLimiting",
						Version: "v1",
						Params: &map[string]interface{}{
							"rate": 100,
						},
					},
				},
			},
		},
	})
	apiConfig := &api.APIConfiguration{
		ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:    api.RestApi,
		Spec:    specUnion,
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
		},
	}

	validator := NewPolicyValidator(policyDefs)

	// Create API config with multiple invalid policies
	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName:    "Test API",
		Version: "v1.0",
		Context: "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Policies: &[]api.Policy{
			{
				Name:    "NonExistent1",
				Version: "v1",
			},
			{
				Name:    "NonExistent2",
				Version: "v1",
			},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/resource",
				Policies: &[]api.Policy{
					{
						Name:    "NonExistent3",
						Version: "v1",
					},
				},
			},
		},
	})
	apiConfig := &api.APIConfiguration{
		ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:    api.RestApi,
		Spec:    specUnion,
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
			Parameters: &map[string]interface{}{
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
	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName:    "Test API",
		Version: "v1.0",
		Context: "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Policies: &[]api.Policy{
			{
				Name:    "TestPolicy",
				Version: "v1",
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
	})
	apiConfig := &api.APIConfiguration{
		ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:    api.RestApi,
		Spec:    specUnion,
	}

	errors := validator.ValidatePolicies(apiConfig)
	if len(errors) == 0 {
		t.Error("Expected validation error for type mismatch")
	}
}

func TestPolicyValidator_MissingRequiredParams(t *testing.T) {
	// Create policy definitions with required parameters
	policyDefs := map[string]api.PolicyDefinition{
		"JWTValidation|v1.0.0": {
			Name:    "JWTValidation",
			Version: "v1.0.0",
			Parameters: &map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"issuer": map[string]interface{}{
						"type": "string",
					},
					"audience": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []interface{}{"issuer"}, // issuer is required
			},
		},
	}

	validator := NewPolicyValidator(policyDefs)

	// Test case 1: Policy with nil params (should fail validation for required field)
	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName:    "Test API",
		Version: "v1.0",
		Context: "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Policies: &[]api.Policy{
			{
				Name:    "JWTValidation",
				Version: "v1",
				Params:  nil, // No params provided
			},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/resource",
			},
		},
	})
	apiConfig := &api.APIConfiguration{
		ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:    api.RestApi,
		Spec:    specUnion,
	}

	errors := validator.ValidatePolicies(apiConfig)
	if len(errors) == 0 {
		t.Error("Expected validation error for missing required parameter 'issuer'")
	} else {
		found := false
		for _, err := range errors {
			if contains(err.Message, "issuer") && contains(err.Message, "required") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected error about required 'issuer' parameter, got: %v", errors)
		}
	}

	apiData, err := apiConfig.Spec.AsAPIConfigData()
	if err != nil {
		t.Fatalf("Failed to parse API data: %v", err)
	}

	// Test case 2: Policy with empty params map (should also fail)
	(*apiData.Policies)[0].Params = &map[string]interface{}{}
	apiConfig.Spec.FromAPIConfigData(apiData)
	errors = validator.ValidatePolicies(apiConfig)
	if len(errors) == 0 {
		t.Error("Expected validation error for missing required parameter 'issuer'")
	}

	// Test case 3: Policy with required param provided (should pass)
	(*apiData.Policies)[0].Params = &map[string]interface{}{
		"issuer": "https://auth.example.com",
	}
	apiConfig.Spec.FromAPIConfigData(apiData)
	errors = validator.ValidatePolicies(apiConfig)
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors, got: %v", errors)
	}
}

// Test that two different major-only versions of the same policy name
// can be referenced within the same API and both resolve successfully.
func TestPolicyValidator_MixedMajorVersions_SamePolicyName(t *testing.T) {
	policyDefs := map[string]api.PolicyDefinition{
		"MultiVersionPolicy|v1.0.0": {
			Name:    "MultiVersionPolicy",
			Version: "v1.0.0",
		},
		"MultiVersionPolicy|v2.0.0": {
			Name:    "MultiVersionPolicy",
			Version: "v2.0.0",
		},
	}

	validator := NewPolicyValidator(policyDefs)

	specUnion := api.APIConfiguration_Spec{}
	if err := specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Test API",
		Version:     "v1.0",
		Context:     "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/resource",
				Policies: &[]api.Policy{
					{
						Name:    "MultiVersionPolicy",
						Version: "v1", // major-only v1
					},
					{
						Name:    "MultiVersionPolicy",
						Version: "v2", // major-only v2
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("Failed to create API config data: %v", err)
	}

	apiConfig := &api.APIConfiguration{
		ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.RestApi,
		Spec:       specUnion,
	}

	errors := validator.ValidatePolicies(apiConfig)
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors for mixed major-only versions, got %d: %v", len(errors), errors)
	}
}

// TestPolicyValidator_FullSemverRejected ensures that full semantic version (e.g. v1.0.0)
// in API policy refs is rejected; only major-only (e.g. v1) is allowed.
func TestPolicyValidator_FullSemverRejected(t *testing.T) {
	policyDefs := map[string]api.PolicyDefinition{
		"SomePolicy|v1.0.0": {
			Name:    "SomePolicy",
			Version: "v1.0.0",
		},
	}

	validator := NewPolicyValidator(policyDefs)

	specUnion := api.APIConfiguration_Spec{}
	if err := specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Test API",
		Version:     "v1.0",
		Context:     "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Policies: &[]api.Policy{
			{
				Name:    "SomePolicy",
				Version: "v1.0.0", // full semver not allowed
			},
		},
		Operations: []api.Operation{
			{Method: "GET", Path: "/resource"},
		},
	}); err != nil {
		t.Fatalf("Failed to create API config data: %v", err)
	}

	apiConfig := &api.APIConfiguration{
		ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.RestApi,
		Spec:       specUnion,
	}

	errors := validator.ValidatePolicies(apiConfig)
	if len(errors) == 0 {
		t.Fatal("Expected validation error when policy version is full semver (v1.0.0), got none")
	}
	var hasVersionError bool
	for _, e := range errors {
		if strings.Contains(e.Message, "major-only") || strings.Contains(e.Message, "full semantic version") {
			hasVersionError = true
			break
		}
	}
	if !hasVersionError {
		t.Errorf("Expected error message to mention major-only or full semantic version not allowed, got: %v", errors)
	}
}

func TestPolicyValidator_MajorVersionResolution_Success(t *testing.T) {
	// Policy definitions contain a single v0.x.y version
	policyDefs := map[string]api.PolicyDefinition{
		"MyPolicy|v0.1.0": {
			Name:    "MyPolicy",
			Version: "v0.1.0",
		},
	}

	validator := NewPolicyValidator(policyDefs)

	specUnion := api.APIConfiguration_Spec{}
	if err := specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Test API",
		Version:     "v1.0",
		Context:     "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Policies: &[]api.Policy{
			{
				Name:    "MyPolicy",
				Version: "v0", // Major-only version
			},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/resource",
			},
		},
	}); err != nil {
		t.Fatalf("Failed to create API config data: %v", err)
	}

	apiConfig := &api.APIConfiguration{
		ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.RestApi,
		Spec:       specUnion,
	}

	errors := validator.ValidatePolicies(apiConfig)
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors for major-only version, got %d: %v", len(errors), errors)
	}
}

func TestPolicyValidator_MajorVersionResolution_NotFound(t *testing.T) {
	// Policy definitions contain only v1.x.y, but API asks for v0
	policyDefs := map[string]api.PolicyDefinition{
		"MyPolicy|v1.0.0": {
			Name:    "MyPolicy",
			Version: "v1.0.0",
		},
	}

	validator := NewPolicyValidator(policyDefs)

	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Test API",
		Version:     "v1.0",
		Context:     "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Policies: &[]api.Policy{
			{
				Name:    "MyPolicy",
				Version: "v0", // Major that does not exist
			},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/resource",
			},
		},
	})

	apiConfig := &api.APIConfiguration{
		ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.RestApi,
		Spec:       specUnion,
	}

	errors := validator.ValidatePolicies(apiConfig)
	if len(errors) != 1 {
		t.Fatalf("Expected 1 validation error for unresolved major version, got %d: %v", len(errors), errors)
	}
	if !contains(errors[0].Message, "major version 'v0' not found") {
		t.Errorf("Expected major version not found error, got: %s", errors[0].Message)
	}
}

func TestPolicyValidator_MajorVersionResolution_MultipleMatches(t *testing.T) {
	// Policy definitions contain multiple v0.x.y versions; resolution should fail
	policyDefs := map[string]api.PolicyDefinition{
		"MyPolicy|v0.1.0": {
			Name:    "MyPolicy",
			Version: "v0.1.0",
		},
		"MyPolicy|v0.2.0": {
			Name:    "MyPolicy",
			Version: "v0.2.0",
		},
	}

	validator := NewPolicyValidator(policyDefs)

	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Test API",
		Version:     "v1.0",
		Context:     "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Policies: &[]api.Policy{
			{
				Name:    "MyPolicy",
				Version: "v0", // Ambiguous major; multiple matches
			},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/resource",
			},
		},
	})

	apiConfig := &api.APIConfiguration{
		ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.RestApi,
		Spec:       specUnion,
	}

	errors := validator.ValidatePolicies(apiConfig)
	if len(errors) != 1 {
		t.Fatalf("Expected 1 validation error for ambiguous major version, got %d: %v", len(errors), errors)
	}
	if !contains(errors[0].Message, "multiple matching versions for policy 'MyPolicy' major 'v0'") {
		t.Errorf("Expected multiple matching versions error, got: %s", errors[0].Message)
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

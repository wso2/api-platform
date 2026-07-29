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

	"github.com/stretchr/testify/assert"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func TestPolicyValidator_ValidateRestAPIPolicies_Success(t *testing.T) {
	// Setup policy definitions
	policyDefs := map[string]models.PolicyDefinition{
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

	// Create API config with valid policy
	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Method: api.Ptr(api.OperationMethod("GET")),
					Path:   api.Ptr("/resource"),
				},
			},
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors, got %d: %v", len(errors), errors)
	}
}

func TestPolicyValidator_PolicyNotFound(t *testing.T) {
	// Empty policy definitions
	policyDefs := map[string]models.PolicyDefinition{}
	validator := NewPolicyValidator(policyDefs)

	// Create API config with non-existent policy
	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Name:    "NonExistentPolicy",
					Version: "v1",
				},
			},
			Operations: []api.Operation{
				{
					Method: api.Ptr(api.OperationMethod("GET")),
					Path:   api.Ptr("/resource"),
				},
			},
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) != 1 {
		t.Errorf("Expected 1 validation error, got %d", len(errors))
	}
	if len(errors) > 0 && !contains(errors[0].Message, "not found") {
		t.Errorf("Expected 'not found' error, got: %s", errors[0].Message)
	}
}

func TestPolicyValidator_InvalidParameters(t *testing.T) {
	// Setup policy definition with schema
	policyDefs := map[string]models.PolicyDefinition{
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
	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Method: api.Ptr(api.OperationMethod("GET")),
					Path:   api.Ptr("/resource"),
				},
			},
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) == 0 {
		t.Error("Expected validation errors for missing required parameter")
	}
}

func TestPolicyValidator_OperationLevelPolicies(t *testing.T) {
	// Setup policy definitions
	policyDefs := map[string]models.PolicyDefinition{
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
	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Method: api.Ptr(api.OperationMethod("GET")),
					Path:   api.Ptr("/resource"),
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
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors, got %d: %v", len(errors), errors)
	}
}

func TestPolicyValidator_MultipleErrors(t *testing.T) {
	// Setup one valid policy definition
	policyDefs := map[string]models.PolicyDefinition{
		"ValidPolicy|v1.0.0": {
			Name:    "ValidPolicy",
			Version: "v1.0.0",
		},
	}

	validator := NewPolicyValidator(policyDefs)

	// Create API config with multiple invalid policies
	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Method: api.Ptr(api.OperationMethod("GET")),
					Path:   api.Ptr("/resource"),
					Policies: &[]api.Policy{
						{
							Name:    "NonExistent3",
							Version: "v1",
						},
					},
				},
			},
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) != 3 {
		t.Errorf("Expected 3 validation errors, got %d", len(errors))
	}
}

func TestPolicyValidator_TypeMismatch(t *testing.T) {
	// Setup policy definition expecting integer
	policyDefs := map[string]models.PolicyDefinition{
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
	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Name:    "TestPolicy",
					Version: "v1",
					Params: &map[string]interface{}{
						"count": "not-a-number",
					},
				},
			},
			Operations: []api.Operation{
				{
					Method: api.Ptr(api.OperationMethod("GET")),
					Path:   api.Ptr("/resource"),
				},
			},
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) == 0 {
		t.Error("Expected validation error for type mismatch")
	}
}

func TestPolicyValidator_MissingRequiredParams(t *testing.T) {
	// Create policy definitions with required parameters
	policyDefs := map[string]models.PolicyDefinition{
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
	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Name:    "JWTValidation",
					Version: "v1",
					Params:  nil, // No params provided
				},
			},
			Operations: []api.Operation{
				{
					Method: api.Ptr(api.OperationMethod("GET")),
					Path:   api.Ptr("/resource"),
				},
			},
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
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

	// Test case 2: Policy with empty params map (should also fail)
	(*apiConfig.Spec.Policies)[0].Params = &map[string]interface{}{}
	errors = validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) == 0 {
		t.Error("Expected validation error for missing required parameter 'issuer'")
	}

	// Test case 3: Policy with required param provided (should pass)
	(*apiConfig.Spec.Policies)[0].Params = &map[string]interface{}{
		"issuer": "https://auth.example.com",
	}
	errors = validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors, got: %v", errors)
	}
}

// Test that two different major-only versions of the same policy name
// can be referenced within the same API and both resolve successfully.
func TestPolicyValidator_MixedMajorVersions_SamePolicyName(t *testing.T) {
	policyDefs := map[string]models.PolicyDefinition{
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

	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Method: api.Ptr(api.OperationMethod("GET")),
					Path:   api.Ptr("/resource"),
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
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors for mixed major-only versions, got %d: %v", len(errors), errors)
	}
}

// TestPolicyValidator_FullSemverRejected ensures that full semantic version (e.g. v1.0.0)
// in API policy refs is rejected; only major-only (e.g. v1) is allowed.
func TestPolicyValidator_FullSemverRejected(t *testing.T) {
	policyDefs := map[string]models.PolicyDefinition{
		"SomePolicy|v1.0.0": {
			Name:    "SomePolicy",
			Version: "v1.0.0",
		},
	}

	validator := NewPolicyValidator(policyDefs)

	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
				{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/resource")},
			},
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
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
	policyDefs := map[string]models.PolicyDefinition{
		"MyPolicy|v0.1.0": {
			Name:    "MyPolicy",
			Version: "v0.1.0",
		},
	}

	validator := NewPolicyValidator(policyDefs)

	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Method: api.Ptr(api.OperationMethod("GET")),
					Path:   api.Ptr("/resource"),
				},
			},
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors for major-only version, got %d: %v", len(errors), errors)
	}
}

func TestPolicyValidator_MajorVersionResolution_NotFound(t *testing.T) {
	// Policy definitions contain only v1.x.y, but API asks for v0
	policyDefs := map[string]models.PolicyDefinition{
		"MyPolicy|v1.0.0": {
			Name:    "MyPolicy",
			Version: "v1.0.0",
		},
	}

	validator := NewPolicyValidator(policyDefs)

	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Method: api.Ptr(api.OperationMethod("GET")),
					Path:   api.Ptr("/resource"),
				},
			},
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) != 1 {
		t.Fatalf("Expected 1 validation error for unresolved major version, got %d: %v", len(errors), errors)
	}
	if !contains(errors[0].Message, "major version 'v0' not found") {
		t.Errorf("Expected major version not found error, got: %s", errors[0].Message)
	}
}

func TestPolicyValidator_MajorVersionResolution_MultipleMatches(t *testing.T) {
	// Policy definitions contain multiple v0.x.y versions; resolution should fail
	policyDefs := map[string]models.PolicyDefinition{
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

	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Method: api.Ptr(api.OperationMethod("GET")),
					Path:   api.Ptr("/resource"),
				},
			},
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) != 1 {
		t.Fatalf("Expected 1 validation error for ambiguous major version, got %d: %v", len(errors), errors)
	}
	if !contains(errors[0].Message, "multiple matching versions for policy 'MyPolicy' major 'v0'") {
		t.Errorf("Expected multiple matching versions error, got: %s", errors[0].Message)
	}
}

// TestPolicyValidator_EmptyVersion_ResolvesToLatest ensures that an empty version
// string resolves to the latest available policy version.
func TestPolicyValidator_EmptyVersion_ResolvesToLatest(t *testing.T) {
	policyDefs := map[string]models.PolicyDefinition{
		"MyPolicy|v0.1.0": {Name: "MyPolicy", Version: "v0.1.0"},
		"MyPolicy|v0.2.0": {Name: "MyPolicy", Version: "v0.2.0"},
		"MyPolicy|v1.0.0": {Name: "MyPolicy", Version: "v1.0.0"},
	}

	resolved, err := ResolvePolicyVersion(policyDefs, BuildLatestVersionIndex(policyDefs), "MyPolicy", "")
	if err != nil {
		t.Fatalf("Expected empty version to resolve, got error: %v", err)
	}
	if resolved != "v1.0.0" {
		t.Fatalf("Expected latest resolved version v1.0.0, got %s", resolved)
	}

	validator := NewPolicyValidator(policyDefs)

	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Version: "", // empty — should resolve to v1.0.0
				},
			},
			Operations: []api.Operation{{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/resource")}},
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) > 0 {
		t.Errorf("Expected no validation errors for empty version, got %d: %v", len(errors), errors)
	}
}

// TestPolicyValidator_EmptyVersion_PolicyNotFound ensures an error is returned
// when the policy name does not exist in definitions and version is empty.
func TestPolicyValidator_EmptyVersion_PolicyNotFound(t *testing.T) {
	policyDefs := map[string]models.PolicyDefinition{}

	validator := NewPolicyValidator(policyDefs)

	apiConfig := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Name:    "NonExistentPolicy",
					Version: "",
				},
			},
			Operations: []api.Operation{{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/resource")}},
		},
	}

	errors := validator.ValidateRestAPIPolicies(apiConfig)
	if len(errors) == 0 {
		t.Error("Expected validation error when policy not found and version is empty")
	}
	if len(errors) > 0 && !contains(errors[0].Message, "not found") {
		t.Errorf("Expected 'not found' error, got: %s", errors[0].Message)
	}
}

// TestBuildLatestVersionIndex_PicksLatestPerPolicy verifies that the index returns
// the highest semver for each policy name when multiple versions are present.
func TestBuildLatestVersionIndex_PicksLatestPerPolicy(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"auth|v1.0.0": {Name: "auth", Version: "v1.0.0"},
		"auth|v1.2.0": {Name: "auth", Version: "v1.2.0"},
		"auth|v2.0.0": {Name: "auth", Version: "v2.0.0"},
		"log|v1.0.0":  {Name: "log", Version: "v1.0.0"},
		"log|v1.1.0":  {Name: "log", Version: "v1.1.0"},
	}

	index := BuildLatestVersionIndex(defs)

	if index["auth"] != "v2.0.0" {
		t.Errorf("expected auth latest to be v2.0.0, got %s", index["auth"])
	}
	if index["log"] != "v1.1.0" {
		t.Errorf("expected log latest to be v1.1.0, got %s", index["log"])
	}
}

// TestBuildLatestVersionIndex_SkipsNonSemver verifies that definitions whose
// version is not a full semver (e.g., "v1") are excluded from the index.
func TestBuildLatestVersionIndex_SkipsNonSemver(t *testing.T) {
	defs := map[string]models.PolicyDefinition{
		"auth|v1":     {Name: "auth", Version: "v1"},     // major-only, must be skipped
		"auth|v1.0.0": {Name: "auth", Version: "v1.0.0"}, // valid
	}

	index := BuildLatestVersionIndex(defs)

	if index["auth"] != "v1.0.0" {
		t.Errorf("expected auth latest to be v1.0.0, got %s", index["auth"])
	}
}

// TestBuildLatestVersionIndex_EmptyDefinitions verifies an empty map is returned
// when no definitions are provided.
func TestBuildLatestVersionIndex_EmptyDefinitions(t *testing.T) {
	index := BuildLatestVersionIndex(map[string]models.PolicyDefinition{})
	if len(index) != 0 {
		t.Errorf("expected empty index, got %v", index)
	}
}

func TestPolicyValidator_ValidateMCPProxyPolicies_NilPolicies(t *testing.T) {
	policyDefs := map[string]models.PolicyDefinition{}
	validator := NewPolicyValidator(policyDefs)

	mcpConfig := &api.MCPProxyConfiguration{
		ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.MCPProxyConfigurationKindMcp,
		Metadata:   api.Metadata{Name: "test-mcp"},
		Spec: api.MCPProxyConfigData{
			DisplayName: "Test MCP",
			Version:     "v1.0",
		},
	}

	errors := validator.ValidateMCPProxyPolicies(mcpConfig)
	assert.Empty(t, errors, "expected no errors when spec.policies is nil")
}

func TestPolicyValidator_ValidateMCPProxyPolicies_ValidPolicy(t *testing.T) {
	policyDefs := map[string]models.PolicyDefinition{
		"allow-all|v1.0.0": {
			Name:    "allow-all",
			Version: "v1.0.0",
		},
	}
	validator := NewPolicyValidator(policyDefs)

	policies := []api.Policy{
		{Name: "allow-all", Version: "v1"},
	}
	mcpConfig := &api.MCPProxyConfiguration{
		ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.MCPProxyConfigurationKindMcp,
		Metadata:   api.Metadata{Name: "test-mcp"},
		Spec: api.MCPProxyConfigData{
			DisplayName: "Test MCP",
			Version:     "v1.0",
			Policies:    &policies,
		},
	}

	errors := validator.ValidateMCPProxyPolicies(mcpConfig)
	assert.Empty(t, errors, "expected no errors for valid MCP policy")
}

func TestPolicyValidator_ValidateMCPProxyPolicies_PolicyNotFound(t *testing.T) {
	policyDefs := map[string]models.PolicyDefinition{}
	validator := NewPolicyValidator(policyDefs)

	policies := []api.Policy{
		{Name: "missing-policy", Version: "v1"},
	}
	mcpConfig := &api.MCPProxyConfiguration{
		ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.MCPProxyConfigurationKindMcp,
		Metadata:   api.Metadata{Name: "test-mcp"},
		Spec: api.MCPProxyConfigData{
			DisplayName: "Test MCP",
			Version:     "v1.0",
			Policies:    &policies,
		},
	}

	errors := validator.ValidateMCPProxyPolicies(mcpConfig)
	assert.NotEmpty(t, errors, "expected errors for missing policy")
	assert.Contains(t, errors[0].Field, "spec.policies[0]")
}

func TestPolicyValidator_ValidateMCPProxyPolicies_MultiplePoliciesWithErrors(t *testing.T) {
	policyDefs := map[string]models.PolicyDefinition{
		"good-policy|v1.0.0": {
			Name:    "good-policy",
			Version: "v1.0.0",
		},
	}
	validator := NewPolicyValidator(policyDefs)

	policies := []api.Policy{
		{Name: "good-policy", Version: "v1"},
		{Name: "bad-policy", Version: "v1"},
	}
	mcpConfig := &api.MCPProxyConfiguration{
		ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.MCPProxyConfigurationKindMcp,
		Metadata:   api.Metadata{Name: "test-mcp"},
		Spec: api.MCPProxyConfigData{
			DisplayName: "Test MCP",
			Version:     "v1.0",
			Policies:    &policies,
		},
	}

	errors := validator.ValidateMCPProxyPolicies(mcpConfig)
	assert.NotEmpty(t, errors, "expected errors for missing policy")
	// Only the bad policy should produce errors
	for _, e := range errors {
		assert.Contains(t, e.Field, "spec.policies[1]")
	}
}

// TestCoerceParamsBySchema_RenderedTemplateStrings verifies that template-rendered
// string values are coerced to their schema-declared types so that both JSON-schema
// validation and the policy engine receive correctly-typed values.
//
// Background: text/template always produces strings, so {{ env "RATE" }} → "100"
// even when the parameter schema declares type: integer. coerceParamsBySchema (called
// inside validatePolicy) must convert the string to float64(100) before
// gojsonschema sees it.
func TestCoerceParamsBySchema_RenderedTemplateStrings(t *testing.T) {
	policyDefs := map[string]models.PolicyDefinition{
		"AdvancedRateLimit|v1.0.0": {
			Name:    "AdvancedRateLimit",
			Version: "v1.0.0",
			Parameters: &map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{"type": "integer"},
					"burst": map[string]interface{}{"type": "integer"},
					"enabled": map[string]interface{}{"type": "boolean"},
					"ratio": map[string]interface{}{"type": "number"},
					"name": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"limit"},
			},
		},
	}

	pv := NewPolicyValidator(policyDefs)

	config := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Name:    "AdvancedRateLimit",
					Version: "v1",
					Params: &map[string]interface{}{
						// Simulates what text/template produces after {{ env "X" }} resolution:
						// all values arrive as strings regardless of the declared type.
						"limit":   "100",
						"burst":   "200",
						"enabled": "true",
						"ratio":   "1.5",
						"name":    "prod",
					},
				},
			},
			Operations: []api.Operation{
				{
					Method: api.Ptr(api.OperationMethod("GET")),
					Path:   api.Ptr("/resource"),
				},
			},
		},
	}

	// ValidateRestAPIPolicies coerces params in-place then validates — check both.
	errs := pv.ValidateRestAPIPolicies(config)
	assert.Empty(t, errs, "validation should pass after coercion: %v", errs)

	params := *(*config.Spec.Policies)[0].Params
	assert.Equal(t, float64(100), params["limit"], "integer param should be coerced from string")
	assert.Equal(t, float64(200), params["burst"], "integer param should be coerced from string")
	assert.Equal(t, true, params["enabled"], "boolean param should be coerced from string")
	assert.Equal(t, 1.5, params["ratio"], "number param should be coerced from string")
	assert.Equal(t, "prod", params["name"], "string param must not be changed")
}

// TestCoerceParamsBySchema_UnparseableStringStaysString verifies that a string value
// that cannot be parsed as the declared type is left unchanged — this ensures
// gojsonschema still produces a clear type-mismatch error rather than a silent no-op.
func TestCoerceParamsBySchema_UnparseableStringStaysString(t *testing.T) {
	policyDefs := map[string]models.PolicyDefinition{
		"RateLimit|v1.0.0": {
			Name:    "RateLimit",
			Version: "v1.0.0",
			Parameters: &map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{"type": "integer"},
				},
			},
		},
	}

	pv := NewPolicyValidator(policyDefs)

	config := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Name:    "RateLimit",
					Version: "v1",
					Params:  &map[string]interface{}{"limit": "not-a-number"},
				},
			},
			Operations: []api.Operation{
				{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/resource")},
			},
		},
	}

	// ValidateRestAPIPolicies attempts coercion then validates — unparseable string must
	// remain a string so validation produces a clear type-mismatch error.
	errs := pv.ValidateRestAPIPolicies(config)
	assert.NotEmpty(t, errs, "type mismatch must still produce a validation error")

	params := *(*config.Spec.Policies)[0].Params
	assert.Equal(t, "not-a-number", params["limit"], "unparseable string must not be modified")
}

// TestCoerceParamsBySchema_StringParamWithNumericValueIsUnchanged confirms that a
// param declared as type:string is never coerced — a string that happens to look
// like a number (e.g., "100") must stay as a string.
func TestCoerceParamsBySchema_StringParamWithNumericValueIsUnchanged(t *testing.T) {
	policyDefs := map[string]models.PolicyDefinition{
		"HeaderPolicy|v1.0.0": {
			Name:    "HeaderPolicy",
			Version: "v1.0.0",
			Parameters: &map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"headerValue": map[string]interface{}{"type": "string"},
				},
			},
		},
	}

	pv := NewPolicyValidator(policyDefs)

	config := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
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
					Name:    "HeaderPolicy",
					Version: "v1",
					Params:  &map[string]interface{}{"headerValue": "100"},
				},
			},
			Operations: []api.Operation{
				{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/resource")},
			},
		},
	}

	errs := pv.ValidateRestAPIPolicies(config)
	assert.Empty(t, errs, "string param with numeric-looking value must pass validation")

	params := *(*config.Spec.Policies)[0].Params
	assert.Equal(t, "100", params["headerValue"], "string param must stay as string even when it looks numeric")
}

// TestCoerceParamsBySchema_NestedObjectParams verifies that integer/number/boolean
// string values nested inside an object-typed property are coerced recursively.
func TestCoerceParamsBySchema_NestedObjectParams(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"config": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"maxRetries": map[string]interface{}{"type": "integer"},
					"enabled":    map[string]interface{}{"type": "boolean"},
				},
			},
		},
	}

	params := map[string]interface{}{
		"config": map[string]interface{}{
			"maxRetries": "3",    // rendered template string
			"enabled":    "true", // rendered template string
		},
	}

	coerceParamsBySchema(params, schema)

	inner := params["config"].(map[string]interface{})
	assert.Equal(t, float64(3), inner["maxRetries"], "nested integer param must be coerced to float64")
	assert.Equal(t, true, inner["enabled"], "nested boolean param must be coerced to bool")
}

// TestCoerceParamsBySchema_ArrayOfObjectsWithIntegerParams verifies the advanced-ratelimit
// case where integer params (limit, burst) are nested inside an array of objects
// (quotas[].limits[]).
func TestCoerceParamsBySchema_ArrayOfObjectsWithIntegerParams(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"quotas": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"limits": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"limit": map[string]interface{}{"type": "integer"},
									"burst": map[string]interface{}{"type": "integer"},
								},
							},
						},
					},
				},
			},
		},
	}

	params := map[string]interface{}{
		"quotas": []interface{}{
			map[string]interface{}{
				"limits": []interface{}{
					map[string]interface{}{
						"limit": "100", // rendered from {{ env "RATE_LIMIT" }}
						"burst": "200", // rendered from {{ env "BURST_LIMIT" }}
					},
				},
			},
		},
	}

	coerceParamsBySchema(params, schema)

	quotas := params["quotas"].([]interface{})
	limits := quotas[0].(map[string]interface{})["limits"].([]interface{})
	first := limits[0].(map[string]interface{})
	assert.Equal(t, float64(100), first["limit"], "deeply nested integer param must be coerced to float64")
	assert.Equal(t, float64(200), first["burst"], "deeply nested integer param must be coerced to float64")
}

// TestCoerceParamsBySchema_ArrayOfScalarIntegers verifies that string elements in an
// array of integers (e.g., allowedPorts: ["8080", "8443"]) are coerced element-by-element.
func TestCoerceParamsBySchema_ArrayOfScalarIntegers(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"allowedPorts": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "integer"},
			},
		},
	}

	params := map[string]interface{}{
		"allowedPorts": []interface{}{"8080", "8443"},
	}

	coerceParamsBySchema(params, schema)

	ports := params["allowedPorts"].([]interface{})
	assert.Equal(t, float64(8080), ports[0], "first array element must be coerced to float64")
	assert.Equal(t, float64(8443), ports[1], "second array element must be coerced to float64")
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

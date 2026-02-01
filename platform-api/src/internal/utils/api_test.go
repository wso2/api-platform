/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package utils

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
)

// TestPolicyDTOToModel tests conversion from DTO Policy to Model Policy
func TestPolicyDTOToModel(t *testing.T) {
	util := &APIUtil{}

	executionCondition := "request.path == '/api/v1/users'"
	params := map[string]interface{}{
		"rateLimit": 100,
		"timeUnit":  "minute",
	}

	tests := []struct {
		name     string
		input    *dto.Policy
		expected *model.Policy
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "all fields set",
			input: &dto.Policy{
				ExecutionCondition: &executionCondition,
				Name:               "rate-limiting",
				Params:             &params,
				Version:            "v1",
			},
			expected: &model.Policy{
				ExecutionCondition: &executionCondition,
				Name:               "rate-limiting",
				Params:             &params,
				Version:            "v1",
			},
		},
		{
			name: "nil ExecutionCondition",
			input: &dto.Policy{
				ExecutionCondition: nil,
				Name:               "logging",
				Params:             &params,
				Version:            "v2",
			},
			expected: &model.Policy{
				ExecutionCondition: nil,
				Name:               "logging",
				Params:             &params,
				Version:            "v2",
			},
		},
		{
			name: "nil Params",
			input: &dto.Policy{
				ExecutionCondition: &executionCondition,
				Name:               "authentication",
				Params:             nil,
				Version:            "v1",
			},
			expected: &model.Policy{
				ExecutionCondition: &executionCondition,
				Name:               "authentication",
				Params:             nil,
				Version:            "v1",
			},
		},
		{
			name: "empty Params",
			input: &dto.Policy{
				ExecutionCondition: nil,
				Name:               "cors",
				Params:             &map[string]interface{}{},
				Version:            "v1",
			},
			expected: &model.Policy{
				ExecutionCondition: nil,
				Name:               "cors",
				Params:             &map[string]interface{}{},
				Version:            "v1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.PolicyDTOToModel(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("PolicyDTOToModel() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestPolicyModelToDTO tests conversion from Model Policy to DTO Policy
func TestPolicyModelToDTO(t *testing.T) {
	util := &APIUtil{}

	executionCondition := "response.status == 200"
	params := map[string]interface{}{
		"cacheTTL": 3600,
		"enabled":  true,
	}

	tests := []struct {
		name     string
		input    *model.Policy
		expected *dto.Policy
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "all fields set",
			input: &model.Policy{
				ExecutionCondition: &executionCondition,
				Name:               "caching",
				Params:             &params,
				Version:            "v1",
			},
			expected: &dto.Policy{
				ExecutionCondition: &executionCondition,
				Name:               "caching",
				Params:             &params,
				Version:            "v1",
			},
		},
		{
			name: "nil ExecutionCondition",
			input: &model.Policy{
				ExecutionCondition: nil,
				Name:               "throttling",
				Params:             &params,
				Version:            "v3",
			},
			expected: &dto.Policy{
				ExecutionCondition: nil,
				Name:               "throttling",
				Params:             &params,
				Version:            "v3",
			},
		},
		{
			name: "nil Params",
			input: &model.Policy{
				ExecutionCondition: &executionCondition,
				Name:               "header-modifier",
				Params:             nil,
				Version:            "v2",
			},
			expected: &dto.Policy{
				ExecutionCondition: &executionCondition,
				Name:               "header-modifier",
				Params:             nil,
				Version:            "v2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.PolicyModelToDTO(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("PolicyModelToDTO() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestPoliciesDTOToModel tests conversion from DTO Policy slice to Model Policy slice
func TestPoliciesDTOToModel(t *testing.T) {
	util := &APIUtil{}

	condition1 := "request.method == 'POST'"
	params1 := map[string]interface{}{"maxSize": 1024}
	condition2 := "response.status >= 400"
	params2 := map[string]interface{}{"logLevel": "error"}

	tests := []struct {
		name     string
		input    []dto.Policy
		expected []model.Policy
	}{
		{
			name:     "nil slice",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []dto.Policy{},
			expected: []model.Policy{},
		},
		{
			name: "single policy",
			input: []dto.Policy{
				{
					ExecutionCondition: &condition1,
					Name:               "validation",
					Params:             &params1,
					Version:            "v1",
				},
			},
			expected: []model.Policy{
				{
					ExecutionCondition: &condition1,
					Name:               "validation",
					Params:             &params1,
					Version:            "v1",
				},
			},
		},
		{
			name: "multiple policies",
			input: []dto.Policy{
				{
					ExecutionCondition: &condition1,
					Name:               "request-logger",
					Params:             &params1,
					Version:            "v1",
				},
				{
					ExecutionCondition: nil,
					Name:               "rate-limiter",
					Params:             nil,
					Version:            "v2",
				},
				{
					ExecutionCondition: &condition2,
					Name:               "error-logger",
					Params:             &params2,
					Version:            "v1",
				},
			},
			expected: []model.Policy{
				{
					ExecutionCondition: &condition1,
					Name:               "request-logger",
					Params:             &params1,
					Version:            "v1",
				},
				{
					ExecutionCondition: nil,
					Name:               "rate-limiter",
					Params:             nil,
					Version:            "v2",
				},
				{
					ExecutionCondition: &condition2,
					Name:               "error-logger",
					Params:             &params2,
					Version:            "v1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.PoliciesDTOToModel(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("PoliciesDTOToModel() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestPoliciesModelToDTO tests conversion from Model Policy slice to DTO Policy slice
func TestPoliciesModelToDTO(t *testing.T) {
	util := &APIUtil{}

	condition := "request.header['X-API-Key'] != ''"
	params := map[string]interface{}{"required": true}

	tests := []struct {
		name     string
		input    []model.Policy
		expected []dto.Policy
	}{
		{
			name:     "nil slice",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []model.Policy{},
			expected: []dto.Policy{},
		},
		{
			name: "multiple policies",
			input: []model.Policy{
				{
					ExecutionCondition: &condition,
					Name:               "api-key-validation",
					Params:             &params,
					Version:            "v1",
				},
				{
					ExecutionCondition: nil,
					Name:               "jwt-validation",
					Params:             nil,
					Version:            "v2",
				},
			},
			expected: []dto.Policy{
				{
					ExecutionCondition: &condition,
					Name:               "api-key-validation",
					Params:             &params,
					Version:            "v1",
				},
				{
					ExecutionCondition: nil,
					Name:               "jwt-validation",
					Params:             nil,
					Version:            "v2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.PoliciesModelToDTO(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("PoliciesModelToDTO() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestOperationRequestDTOToModel tests conversion of OperationRequest including Policies
func TestOperationRequestDTOToModel(t *testing.T) {
	util := &APIUtil{}

	condition := "request.path =~ '/api/.*'"
	params := map[string]interface{}{"timeout": 30}

	tests := []struct {
		name     string
		input    *dto.OperationRequest
		expected *model.OperationRequest
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "with policies",
			input: &dto.OperationRequest{
				Method: "GET",
				Path:   "/api/v1/users",
				Policies: []dto.Policy{
					{
						ExecutionCondition: &condition,
						Name:               "timeout-policy",
						Params:             &params,
						Version:            "v1",
					},
				},
			},
			expected: &model.OperationRequest{
				Method: "GET",
				Path:   "/api/v1/users",
				Policies: []model.Policy{
					{
						ExecutionCondition: &condition,
						Name:               "timeout-policy",
						Params:             &params,
						Version:            "v1",
					},
				},
			},
		},
		{
			name: "empty policies",
			input: &dto.OperationRequest{
				Method:   "POST",
				Path:     "/api/v1/orders",
				Policies: []dto.Policy{},
			},
			expected: &model.OperationRequest{
				Method:   "POST",
				Path:     "/api/v1/orders",
				Policies: []model.Policy{},
			},
		},
		{
			name: "nil policies",
			input: &dto.OperationRequest{
				Method:   "DELETE",
				Path:     "/api/v1/users/{id}",
				Policies: nil,
			},
			expected: &model.OperationRequest{
				Method:   "DELETE",
				Path:     "/api/v1/users/{id}",
				Policies: nil,
			},
		},
		{
			name: "full request with multiple policies",
			input: &dto.OperationRequest{
				Method: "PUT",
				Path:   "/api/v1/products/{id}",
				Authentication: &dto.AuthenticationConfig{
					Required: true,
					Scopes:   []string{"write:products"},
				},
				Policies: []dto.Policy{
					{
						Name:    "auth-policy",
						Version: "v1",
					},
					{
						ExecutionCondition: &condition,
						Name:               "rate-limit",
						Params:             &params,
						Version:            "v2",
					},
				},
			},
			expected: &model.OperationRequest{
				Method: "PUT",
				Path:   "/api/v1/products/{id}",
				Authentication: &model.AuthenticationConfig{
					Required: true,
					Scopes:   []string{"write:products"},
				},
				Policies: []model.Policy{
					{
						Name:    "auth-policy",
						Version: "v1",
					},
					{
						ExecutionCondition: &condition,
						Name:               "rate-limit",
						Params:             &params,
						Version:            "v2",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.OperationRequestDTOToModel(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("OperationRequestDTOToModel() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestOperationRequestModelToDTO tests conversion of OperationRequest from Model to DTO
func TestOperationRequestModelToDTO(t *testing.T) {
	util := &APIUtil{}

	condition := "request.query.version == '2'"
	params := map[string]interface{}{"maxRetries": 3}

	tests := []struct {
		name     string
		input    *model.OperationRequest
		expected *dto.OperationRequest
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "with policies",
			input: &model.OperationRequest{
				Method: "PATCH",
				Path:   "/api/v2/settings",
				Policies: []model.Policy{
					{
						ExecutionCondition: &condition,
						Name:               "retry-policy",
						Params:             &params,
						Version:            "v1",
					},
				},
			},
			expected: &dto.OperationRequest{
				Method: "PATCH",
				Path:   "/api/v2/settings",
				Policies: []dto.Policy{
					{
						ExecutionCondition: &condition,
						Name:               "retry-policy",
						Params:             &params,
						Version:            "v1",
					},
				},
			},
		},
		{
			name: "empty policies",
			input: &model.OperationRequest{
				Method:   "OPTIONS",
				Path:     "/api/v1/*",
				Policies: []model.Policy{},
			},
			expected: &dto.OperationRequest{
				Method:   "OPTIONS",
				Path:     "/api/v1/*",
				Policies: []dto.Policy{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.OperationRequestModelToDTO(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("OperationRequestModelToDTO() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestPolicyRoundTrip tests DTO -> Model -> DTO conversion preserves data
func TestPolicyRoundTrip(t *testing.T) {
	util := &APIUtil{}

	condition := "request.header['Content-Type'] == 'application/json'"
	params := map[string]interface{}{
		"validateSchema": true,
		"schemaVersion":  "2.0",
		"strictMode":     false,
	}

	original := &dto.Policy{
		ExecutionCondition: &condition,
		Name:               "json-validator",
		Params:             &params,
		Version:            "v3",
	}

	// Convert DTO -> Model
	modelPolicy := util.PolicyDTOToModel(original)
	if modelPolicy == nil {
		t.Fatal("PolicyDTOToModel() returned nil")
	}

	// Convert Model -> DTO
	result := util.PolicyModelToDTO(modelPolicy)
	if result == nil {
		t.Fatal("PolicyModelToDTO() returned nil")
	}

	// Verify all fields match
	if !reflect.DeepEqual(result, original) {
		t.Errorf("Round-trip conversion failed.\nOriginal: %+v\nResult: %+v", original, result)
	}

	// Verify ExecutionCondition pointer is preserved
	if result.ExecutionCondition == nil || *result.ExecutionCondition != condition {
		t.Errorf("ExecutionCondition not preserved correctly")
	}

	// Verify Params pointer is preserved
	if result.Params == nil {
		t.Error("Params pointer lost in round-trip")
	} else {
		if !reflect.DeepEqual(*result.Params, params) {
			t.Errorf("Params values changed in round-trip")
		}
	}
}

// TestOperationRequestRoundTrip tests DTO -> Model -> DTO conversion preserves Policies
func TestOperationRequestRoundTrip(t *testing.T) {
	util := &APIUtil{}

	condition1 := "request.method == 'POST'"
	params1 := map[string]interface{}{"maxBodySize": 10240}
	condition2 := "response.status == 201"

	original := &dto.OperationRequest{
		Method: "POST",
		Path:   "/api/v1/resources",
		Authentication: &dto.AuthenticationConfig{
			Required: true,
			Scopes:   []string{"write:resources", "admin"},
		},
		Policies: []dto.Policy{
			{
				ExecutionCondition: &condition1,
				Name:               "body-size-validator",
				Params:             &params1,
				Version:            "v1",
			},
			{
				ExecutionCondition: &condition2,
				Name:               "success-logger",
				Params:             nil,
				Version:            "v2",
			},
			{
				ExecutionCondition: nil,
				Name:               "audit-trail",
				Params:             &map[string]interface{}{"enabled": true},
				Version:            "v1",
			},
		},
	}

	// Convert DTO -> Model
	modelRequest := util.OperationRequestDTOToModel(original)
	if modelRequest == nil {
		t.Fatal("OperationRequestDTOToModel() returned nil")
	}

	// Convert Model -> DTO
	result := util.OperationRequestModelToDTO(modelRequest)
	if result == nil {
		t.Fatal("OperationRequestModelToDTO() returned nil")
	}

	// Verify all fields match
	if !reflect.DeepEqual(result, original) {
		t.Errorf("Round-trip conversion failed.\nOriginal: %+v\nResult: %+v", original, result)
	}

	// Verify Policies count
	if len(result.Policies) != len(original.Policies) {
		t.Errorf("Policies count mismatch. Got %d, want %d", len(result.Policies), len(original.Policies))
	}

	// Verify each policy is preserved
	for i := range result.Policies {
		if !reflect.DeepEqual(result.Policies[i], original.Policies[i]) {
			t.Errorf("Policy[%d] not preserved.\nOriginal: %+v\nResult: %+v",
				i, original.Policies[i], result.Policies[i])
		}
	}
}

func TestGenerateAPIDeploymentYAMLIncludesPolicies(t *testing.T) {
	util := &APIUtil{}

	condition := "request.path == '/pets'"
	params := map[string]interface{}{"limit": 10}
	policies := []dto.Policy{
		{
			ExecutionCondition: &condition,
			Name:               "rate-limit",
			Params:             &params,
			Version:            "v1",
		},
	}

	api := &dto.API{
		ID:        "api-123",
		Name:      "Pets API",
		Version:   "v1",
		Context:   "/pets",
		ProjectID: "project-123",
		Type:      constants.APITypeHTTP,
		Policies:  policies,
		BackendServices: []dto.BackendService{
			{
				Endpoints: []dto.BackendEndpoint{
					{URL: "https://backend.example.com"},
				},
			},
		},
		Operations: []dto.Operation{
			{
				Request: &dto.OperationRequest{
					Method: "GET",
					Path:   "/pets",
				},
			},
		},
	}

	yamlString, err := util.GenerateAPIDeploymentYAML(api)
	if err != nil {
		t.Fatalf("GenerateAPIDeploymentYAML() error = %v", err)
	}

	var deployment dto.APIDeploymentYAML
	if err := yaml.Unmarshal([]byte(yamlString), &deployment); err != nil {
		t.Fatalf("failed to unmarshal deployment YAML: %v", err)
	}

	if !reflect.DeepEqual(deployment.Spec.Policies, policies) {
		t.Errorf("deployment policies = %v, want %v", deployment.Spec.Policies, policies)
	}
}

func TestAPIYAMLDataToDTOPreservesPolicies(t *testing.T) {
	util := &APIUtil{}

	condition := "request.method == 'GET'"
	params := map[string]interface{}{"enabled": true}
	policies := []dto.Policy{
		{
			ExecutionCondition: &condition,
			Name:               "request-logger",
			Params:             &params,
			Version:            "v2",
		},
	}

	yamlData := &dto.APIYAMLData{
		DisplayName: "Pets API",
		Version:     "v1",
		Context:     "/pets",
		Policies:    policies,
	}

	api := util.APIYAMLDataToDTO(yamlData)
	if api == nil {
		t.Fatal("APIYAMLDataToDTO() returned nil")
	}

	if !reflect.DeepEqual(api.Policies, policies) {
		t.Errorf("API policies = %v, want %v", api.Policies, policies)
	}
}

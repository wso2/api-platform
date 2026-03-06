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
	"encoding/json"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
)

// TestPolicyAPIToModel tests conversion from generated API Policy to Model Policy
func TestPolicyAPIToModel(t *testing.T) {
	util := &APIUtil{}

	executionCondition := "request.path == '/api/v1/users'"
	params := map[string]interface{}{
		"rateLimit": 100,
		"timeUnit":  "minute",
	}

	tests := []struct {
		name     string
		input    *api.Policy
		expected *model.Policy
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "all fields set",
			input: &api.Policy{
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
			input: &api.Policy{
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
			input: &api.Policy{
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
			input: &api.Policy{
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
			result := util.PolicyAPIToModel(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("PolicyAPIToModel() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestPolicyModelToAPI tests conversion from Model Policy to generated API Policy
func TestPolicyModelToAPI(t *testing.T) {
	util := &APIUtil{}

	executionCondition := "response.status == 200"
	params := map[string]interface{}{
		"cacheTTL": 3600,
		"enabled":  true,
	}

	tests := []struct {
		name     string
		input    *model.Policy
		expected *api.Policy
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
			expected: &api.Policy{
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
			expected: &api.Policy{
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
			expected: &api.Policy{
				ExecutionCondition: &executionCondition,
				Name:               "header-modifier",
				Params:             nil,
				Version:            "v2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input == nil {
				if tt.expected != nil {
					t.Errorf("PolicyModelToAPI() = nil, want %v", tt.expected)
				}
				return
			}
			result := util.PolicyModelToAPI(*tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("PolicyModelToAPI() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestPoliciesAPIToModel tests conversion from generated API Policy slice to Model Policy slice
func TestPoliciesAPIToModel(t *testing.T) {
	util := &APIUtil{}

	condition1 := "request.method == 'POST'"
	params1 := map[string]interface{}{"maxSize": 1024}
	condition2 := "response.status >= 400"
	params2 := map[string]interface{}{"logLevel": "error"}

	tests := []struct {
		name     string
		input    *[]api.Policy
		expected []model.Policy
	}{
		{
			name:     "nil slice",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    &[]api.Policy{},
			expected: []model.Policy{},
		},
		{
			name: "single policy",
			input: &[]api.Policy{
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
			input: &[]api.Policy{
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
			result := util.PoliciesAPIToModel(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("PoliciesAPIToModel() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestPoliciesModelToAPI tests conversion from Model Policy slice to generated API Policy slice
func TestPoliciesModelToAPI(t *testing.T) {
	util := &APIUtil{}

	condition := "request.header['X-API-Key'] != ''"
	params := map[string]interface{}{"required": true}

	tests := []struct {
		name     string
		input    []model.Policy
		expected *[]api.Policy
	}{
		{
			name:     "nil slice",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []model.Policy{},
			expected: &[]api.Policy{},
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
			expected: &[]api.Policy{
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
			result := util.PoliciesModelToAPI(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("PoliciesModelToAPI() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestPolicyRoundTrip tests API -> Model -> API conversion preserves data
func TestPolicyRoundTrip(t *testing.T) {
	util := &APIUtil{}

	condition := "request.header['Content-Type'] == 'application/json'"
	params := map[string]interface{}{
		"validateSchema": true,
		"schemaVersion":  "2.0",
		"strictMode":     false,
	}

	original := &api.Policy{
		ExecutionCondition: &condition,
		Name:               "json-validator",
		Params:             &params,
		Version:            "v3",
	}

	// Convert API -> Model
	modelPolicy := util.PolicyAPIToModel(original)
	if modelPolicy == nil {
		t.Fatal("PolicyAPIToModel() returned nil")
	}

	// Convert Model -> API
	result := util.PolicyModelToAPI(*modelPolicy)
	if result == nil {
		t.Fatal("PolicyModelToAPI() returned nil")
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

// TestOperationRequestRoundTrip tests API -> Model -> API conversion preserves Policies
func TestOperationRequestRoundTrip(t *testing.T) {
	util := &APIUtil{}

	condition1 := "request.method == 'POST'"
	params1 := map[string]interface{}{"maxBodySize": 10240}
	condition2 := "response.status == 201"

	original := &api.OperationRequest{
		Method: api.OperationRequestMethodPOST,
		Path:   "/api/v1/resources",
		Policies: &[]api.Policy{
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

	// Convert API -> Model
	modelRequest := util.OperationRequestAPIToModel(original)
	if modelRequest == nil {
		t.Fatal("OperationRequestAPIToModel() returned nil")
	}

	// Convert Model -> API
	result := util.OperationRequestModelToAPI(modelRequest)
	if result == nil {
		t.Fatal("OperationRequestModelToAPI() returned nil")
	}

	// Verify all fields match
	if !reflect.DeepEqual(result, original) {
		t.Errorf("Round-trip conversion failed.\nOriginal: %+v\nResult: %+v", original, result)
	}

	// Verify Policies count
	if result.Policies == nil || original.Policies == nil {
		t.Fatal("Policies pointer should not be nil")
	}

	if len(*result.Policies) != len(*original.Policies) {
		t.Errorf("Policies count mismatch. Got %d, want %d", len(*result.Policies), len(*original.Policies))
	}

	// Verify each policy is preserved
	for i := range *result.Policies {
		if !reflect.DeepEqual((*result.Policies)[i], (*original.Policies)[i]) {
			t.Errorf("Policy[%d] not preserved.\nOriginal: %+v\nResult: %+v",
				i, (*original.Policies)[i], (*result.Policies)[i])
		}
	}
}

func TestGenerateAPIDeploymentYAMLIncludesPolicies(t *testing.T) {
	util := &APIUtil{}

	condition := "request.path == '/pets'"
	params := map[string]interface{}{"limit": 10}
	policies := []model.Policy{
		{
			ExecutionCondition: &condition,
			Name:               "rate-limit",
			Params:             &params,
			Version:            "v1",
		},
	}

	context := "/pets"
	apiModel := &model.API{
		Handle:    "api-123",
		Name:      "Pets API",
		Version:   "v1",
		ProjectID: "project-123",
		Kind:      constants.RestApi,
		Configuration: model.RestAPIConfig{
			Context:  &context,
			Policies: policies,
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL: "https://backend.example.com",
				},
			},
			Operations: []model.Operation{
				{
					Request: &model.OperationRequest{
						Method: "GET",
						Path:   "/pets",
					},
				},
			},
		},
	}

	yamlString, err := util.GenerateAPIDeploymentYAML(apiModel)
	if err != nil {
		t.Fatalf("GenerateAPIDeploymentYAML() error = %v", err)
	}

	var deployment dto.APIDeploymentYAML
	if err := yaml.Unmarshal([]byte(yamlString), &deployment); err != nil {
		t.Fatalf("failed to unmarshal deployment YAML: %v", err)
	}

	expectedPolicies := &[]api.Policy{
		{
			ExecutionCondition: &condition,
			Name:               "rate-limit",
			Params:             &params,
			Version:            "v1",
		},
	}

	deploymentPolicies := util.PoliciesModelToAPI(util.PoliciesDTOToModel(deployment.Spec.Policies))
	if !reflect.DeepEqual(deploymentPolicies, expectedPolicies) {
		t.Errorf("deployment policies = %v, want %v", deploymentPolicies, expectedPolicies)
	}
}

func TestAPIYAMLDataToRESTAPIPreservesPolicies(t *testing.T) {
	util := &APIUtil{}

	condition := "request.method == 'GET'"
	params := map[string]interface{}{"enabled": true}
	generatedPolicies := []api.Policy{
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
		Policies:    util.PoliciesModelToDTO(util.PoliciesAPIToModel(&generatedPolicies)),
	}

	restAPI := util.APIYAMLDataToRESTAPI(yamlData)
	if restAPI == nil {
		t.Fatal("APIYAMLDataToRESTAPI() returned nil")
	}

	expectedPolicies := &generatedPolicies

	if !reflect.DeepEqual(restAPI.Policies, expectedPolicies) {
		t.Errorf("API policies = %v, want %v", restAPI.Policies, expectedPolicies)
	}
}

// TestRESTAPIToModelMapsVhosts verifies that RESTAPIToModel maps both main and sandbox vhosts.
func TestRESTAPIToModelMapsVhosts(t *testing.T) {
	util := &APIUtil{}
	sandbox := "sandbox-api.example.com"

	projectID := "00000000-0000-0000-0000-000000000001"
	restAPI := &api.RESTAPI{
		Name:    "Test API",
		Context: "/test",
		Version: "1.0",
		Upstream: api.Upstream{
			Main: api.UpstreamDefinition{},
		},
		Vhosts: &api.APIVhosts{
			Main:    "api.example.com",
			Sandbox: &sandbox,
		},
	}
	parsedUUID, _ := ParseOpenAPIUUID(projectID)
	restAPI.ProjectId = *parsedUUID

	result := util.RESTAPIToModel(restAPI, "org-1")

	if result.Configuration.Vhosts == nil {
		t.Fatal("expected Configuration.Vhosts to be set")
	}
	if result.Configuration.Vhosts.Main != "api.example.com" {
		t.Errorf("Main vhost = %q, want %q", result.Configuration.Vhosts.Main, "api.example.com")
	}
	if result.Configuration.Vhosts.Sandbox == nil || *result.Configuration.Vhosts.Sandbox != sandbox {
		t.Errorf("Sandbox vhost = %v, want %q", result.Configuration.Vhosts.Sandbox, sandbox)
	}
}

// TestModelToRESTAPIRoundTripVhosts verifies that vhosts survive a ModelToRESTAPI roundtrip.
func TestModelToRESTAPIRoundTripVhosts(t *testing.T) {
	util := &APIUtil{}
	sandbox := "sandbox-api.example.com"
	context := "/test"

	apiModel := &model.API{
		Handle:    "test-api",
		Name:      "Test API",
		Version:   "1.0",
		ProjectID: "00000000-0000-0000-0000-000000000001",
		Configuration: model.RestAPIConfig{
			Context: &context,
			Vhosts: &model.VhostsConfig{
				Main:    "api.example.com",
				Sandbox: &sandbox,
			},
		},
	}

	result, err := util.ModelToRESTAPI(apiModel)
	if err != nil {
		t.Fatalf("ModelToRESTAPI() error = %v", err)
	}
	if result.Vhosts == nil {
		t.Fatal("expected Vhosts to be set in RESTAPI")
	}
	if result.Vhosts.Main != "api.example.com" {
		t.Errorf("Vhosts.Main = %q, want %q", result.Vhosts.Main, "api.example.com")
	}
	if result.Vhosts.Sandbox == nil || *result.Vhosts.Sandbox != sandbox {
		t.Errorf("Vhosts.Sandbox = %v, want %q", result.Vhosts.Sandbox, sandbox)
	}
}

// TestModelToRESTAPILegacyVhostFallback verifies that a model deserialized from a legacy DB row
// (with "vhost" but no "vhosts") exposes it as vhosts.main in the REST API response.
func TestModelToRESTAPILegacyVhostFallback(t *testing.T) {
	util := &APIUtil{}

	// Simulate reading a legacy DB row that has "vhost" but no "vhosts"
	legacyJSON := `{"context":"/legacy","vhost":"legacy.example.com"}`
	var config model.RestAPIConfig
	if err := json.Unmarshal([]byte(legacyJSON), &config); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	apiModel := &model.API{
		Handle:        "legacy-api",
		Name:          "Legacy API",
		Version:       "1.0",
		ProjectID:     "00000000-0000-0000-0000-000000000001",
		Configuration: config,
	}

	result, err := util.ModelToRESTAPI(apiModel)
	if err != nil {
		t.Fatalf("ModelToRESTAPI() error = %v", err)
	}
	if result.Vhosts == nil {
		t.Fatal("expected Vhosts to be populated from legacy vhost field")
	}
	if result.Vhosts.Main != "legacy.example.com" {
		t.Errorf("Vhosts.Main = %q, want %q", result.Vhosts.Main, "legacy.example.com")
	}
	if result.Vhosts.Sandbox != nil {
		t.Errorf("Vhosts.Sandbox should be nil, got %v", result.Vhosts.Sandbox)
	}
}

// TestModelToRESTAPILegacyVhostWhitespaceIgnored verifies that a whitespace-only legacy vhost is not promoted.
func TestModelToRESTAPILegacyVhostWhitespaceIgnored(t *testing.T) {
	util := &APIUtil{}

	// Simulate a legacy DB row with a whitespace-only vhost value
	legacyJSON := `{"context":"/legacy","vhost":"   "}`
	var config model.RestAPIConfig
	if err := json.Unmarshal([]byte(legacyJSON), &config); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	apiModel := &model.API{
		Handle:        "legacy-ws-api",
		Name:          "Legacy Whitespace API",
		Version:       "1.0",
		ProjectID:     "00000000-0000-0000-0000-000000000001",
		Configuration: config,
	}

	result, err := util.ModelToRESTAPI(apiModel)
	if err != nil {
		t.Fatalf("ModelToRESTAPI() error = %v", err)
	}
	if result.Vhosts != nil {
		t.Errorf("expected Vhosts to be nil for whitespace-only legacy vhost, got %+v", result.Vhosts)
	}
}

// TestGenerateDeploymentYAMLIncludesVhosts verifies that the generated YAML contains vhosts.main and vhosts.sandbox.
func TestGenerateDeploymentYAMLIncludesVhosts(t *testing.T) {
	util := &APIUtil{}
	context := "/test"
	sandbox := "sandbox-api.example.com"

	apiModel := &model.API{
		Handle:    "test-api",
		Name:      "Test API",
		Version:   "1.0",
		ProjectID: "project-123",
		Kind:      constants.RestApi,
		Configuration: model.RestAPIConfig{
			Context: &context,
			Vhosts: &model.VhostsConfig{
				Main:    "api.example.com",
				Sandbox: &sandbox,
			},
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "https://backend.example.com"},
			},
			Operations: []model.Operation{
				{Request: &model.OperationRequest{Method: "GET", Path: "/test"}},
			},
		},
	}

	yamlString, err := util.GenerateAPIDeploymentYAML(apiModel)
	if err != nil {
		t.Fatalf("GenerateAPIDeploymentYAML() error = %v", err)
	}

	var deployment dto.APIDeploymentYAML
	if err := yaml.Unmarshal([]byte(yamlString), &deployment); err != nil {
		t.Fatalf("failed to unmarshal deployment YAML: %v", err)
	}

	if deployment.Spec.Vhosts == nil {
		t.Fatal("expected spec.vhosts to be set in deployment YAML")
	}
	if deployment.Spec.Vhosts.Main != "api.example.com" {
		t.Errorf("spec.vhosts.main = %q, want %q", deployment.Spec.Vhosts.Main, "api.example.com")
	}
	if deployment.Spec.Vhosts.Sandbox == nil || *deployment.Spec.Vhosts.Sandbox != sandbox {
		t.Errorf("spec.vhosts.sandbox = %v, want %q", deployment.Spec.Vhosts.Sandbox, sandbox)
	}
}

// TestGenerateDeploymentYAMLLegacyVhostFallback verifies that a model deserialized from a legacy DB row
// (with "vhost" but no "vhosts") produces spec.vhosts.main in the deployment YAML.
// TestMergeRESTAPIDetailsVhosts verifies that MergeRESTAPIDetails correctly preserves Vhosts.
func TestMergeRESTAPIDetailsVhosts(t *testing.T) {
	util := &APIUtil{}
	sandbox := "sandbox.example.com"

	userVhosts := &api.APIVhosts{Main: "user.example.com", Sandbox: &sandbox}
	extractedVhosts := &api.APIVhosts{Main: "extracted.example.com"}

	backendURL := "https://backend.example.com"
	baseUser := func() *api.RESTAPI {
		return &api.RESTAPI{
			Name:    "Test API",
			Context: "/test",
			Version: "1.0",
			Upstream: api.Upstream{
				Main: api.UpstreamDefinition{Url: &backendURL},
			},
		}
	}
	baseExtracted := func() *api.RESTAPI {
		return &api.RESTAPI{
			Name:    "Test API",
			Context: "/test",
			Version: "1.0",
			Upstream: api.Upstream{
				Main: api.UpstreamDefinition{Url: &backendURL},
			},
		}
	}

	t.Run("user vhosts wins when both set", func(t *testing.T) {
		u := baseUser()
		u.Vhosts = userVhosts
		e := baseExtracted()
		e.Vhosts = extractedVhosts
		merged := util.MergeRESTAPIDetails(u, e)
		if merged.Vhosts != userVhosts {
			t.Errorf("Vhosts = %v, want userVhosts %v", merged.Vhosts, userVhosts)
		}
	})

	t.Run("user vhosts wins when extracted nil", func(t *testing.T) {
		u := baseUser()
		u.Vhosts = userVhosts
		e := baseExtracted()
		merged := util.MergeRESTAPIDetails(u, e)
		if merged.Vhosts != userVhosts {
			t.Errorf("Vhosts = %v, want userVhosts %v", merged.Vhosts, userVhosts)
		}
	})

	t.Run("extracted vhosts used when user nil", func(t *testing.T) {
		u := baseUser()
		e := baseExtracted()
		e.Vhosts = extractedVhosts
		merged := util.MergeRESTAPIDetails(u, e)
		if merged.Vhosts != extractedVhosts {
			t.Errorf("Vhosts = %v, want extractedVhosts %v", merged.Vhosts, extractedVhosts)
		}
	})
}

func TestGenerateDeploymentYAMLLegacyVhostFallback(t *testing.T) {
	util := &APIUtil{}

	// Simulate reading a legacy DB row that has "vhost" but no "vhosts"
	legacyJSON := `{"context":"/legacy","vhost":"legacy.example.com","upstream":{"main":{"url":"https://backend.example.com"}},"operations":[{"request":{"method":"GET","path":"/legacy"}}]}`
	var config model.RestAPIConfig
	if err := json.Unmarshal([]byte(legacyJSON), &config); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	apiModel := &model.API{
		Handle:        "legacy-api",
		Name:          "Legacy API",
		Version:       "1.0",
		ProjectID:     "project-123",
		Kind:          constants.RestApi,
		Configuration: config,
	}

	yamlString, err := util.GenerateAPIDeploymentYAML(apiModel)
	if err != nil {
		t.Fatalf("GenerateAPIDeploymentYAML() error = %v", err)
	}

	var deployment dto.APIDeploymentYAML
	if err := yaml.Unmarshal([]byte(yamlString), &deployment); err != nil {
		t.Fatalf("failed to unmarshal deployment YAML: %v", err)
	}

	if deployment.Spec.Vhosts == nil {
		t.Fatal("expected spec.vhosts to be set from legacy vhost field")
	}
	if deployment.Spec.Vhosts.Main != "legacy.example.com" {
		t.Errorf("spec.vhosts.main = %q, want %q", deployment.Spec.Vhosts.Main, "legacy.example.com")
	}
	if deployment.Spec.Vhosts.Sandbox != nil {
		t.Errorf("spec.vhosts.sandbox should be nil for legacy fallback, got %v", deployment.Spec.Vhosts.Sandbox)
	}
}

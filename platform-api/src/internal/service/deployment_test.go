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

package service

import (
	"strings"
	"testing"

	"platform-api/src/internal/dto"

	"gopkg.in/yaml.v3"
)

// TestValidateEndpointURL tests the validateEndpointURL helper function
func TestValidateEndpointURL(t *testing.T) {
	tests := []struct {
		name        string
		endpointURL string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid http URL",
			endpointURL: "http://api.example.com:8080/v1",
			wantErr:     false,
		},
		{
			name:        "valid https URL",
			endpointURL: "https://api.example.com/v1/resources",
			wantErr:     false,
		},
		{
			name:        "valid URL with port",
			endpointURL: "http://localhost:8080",
			wantErr:     false,
		},
		{
			name:        "valid URL with IP address",
			endpointURL: "http://192.168.1.100:8080/api",
			wantErr:     false,
		},
		{
			name:        "empty URL",
			endpointURL: "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "invalid scheme - ftp",
			endpointURL: "ftp://api.example.com/v1",
			wantErr:     true,
			errContains: "scheme must be http or https",
		},
		{
			name:        "invalid scheme - ws",
			endpointURL: "ws://api.example.com/v1",
			wantErr:     true,
			errContains: "scheme must be http or https",
		},
		{
			name:        "missing scheme",
			endpointURL: "api.example.com/v1",
			wantErr:     true,
			errContains: "scheme must be http or https",
		},
		{
			name:        "missing host",
			endpointURL: "http:///path",
			wantErr:     true,
			errContains: "must have a valid host",
		},
		{
			name:        "invalid URL format",
			endpointURL: "://invalid",
			wantErr:     true,
			errContains: "invalid URL format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEndpointURL(tt.endpointURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEndpointURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("validateEndpointURL() error = %v, want error containing %q", err, tt.errContains)
			}
		})
	}
}

// TestOverrideEndpointURL tests the overrideEndpointURL helper function
func TestOverrideEndpointURL(t *testing.T) {
	tests := []struct {
		name           string
		inputYAML      string
		newURL         string
		wantErr        bool
		errContains    string
		validateResult func(t *testing.T, result []byte)
	}{
		{
			name: "override existing main URL",
			inputYAML: `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: test-api
spec:
  displayName: Test API
  version: v1
  context: /test
  upstream:
    main:
      url: http://old-backend.com/api
  operations: []
`,
			newURL:  "https://new-backend.com/api/v2",
			wantErr: false,
			validateResult: func(t *testing.T, result []byte) {
				var apiDeployment dto.APIDeploymentYAML
				if err := yaml.Unmarshal(result, &apiDeployment); err != nil {
					t.Fatalf("Failed to unmarshal result YAML: %v", err)
				}
				if apiDeployment.Spec.Upstream == nil || apiDeployment.Spec.Upstream.Main == nil {
					t.Fatal("Upstream.Main should not be nil")
				}
				if apiDeployment.Spec.Upstream.Main.URL != "https://new-backend.com/api/v2" {
					t.Errorf("Expected URL to be 'https://new-backend.com/api/v2', got %q", apiDeployment.Spec.Upstream.Main.URL)
				}
				if apiDeployment.Spec.Upstream.Main.Ref != "" {
					t.Errorf("Expected Ref to be empty, got %q", apiDeployment.Spec.Upstream.Main.Ref)
				}
			},
		},
		{
			name: "override when upstream is nil",
			inputYAML: `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: test-api
spec:
  displayName: Test API
  version: v1
  context: /test
  operations: []
`,
			newURL:  "http://backend.example.com:8080",
			wantErr: false,
			validateResult: func(t *testing.T, result []byte) {
				var apiDeployment dto.APIDeploymentYAML
				if err := yaml.Unmarshal(result, &apiDeployment); err != nil {
					t.Fatalf("Failed to unmarshal result YAML: %v", err)
				}
				if apiDeployment.Spec.Upstream == nil || apiDeployment.Spec.Upstream.Main == nil {
					t.Fatal("Upstream.Main should be created")
				}
				if apiDeployment.Spec.Upstream.Main.URL != "http://backend.example.com:8080" {
					t.Errorf("Expected URL to be 'http://backend.example.com:8080', got %q", apiDeployment.Spec.Upstream.Main.URL)
				}
			},
		},
		{
			name: "override URL that had ref instead",
			inputYAML: `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: test-api
spec:
  displayName: Test API
  version: v1
  context: /test
  upstream:
    main:
      ref: backend-service-ref
  operations: []
`,
			newURL:  "https://direct-url.com/api",
			wantErr: false,
			validateResult: func(t *testing.T, result []byte) {
				var apiDeployment dto.APIDeploymentYAML
				if err := yaml.Unmarshal(result, &apiDeployment); err != nil {
					t.Fatalf("Failed to unmarshal result YAML: %v", err)
				}
				if apiDeployment.Spec.Upstream.Main.URL != "https://direct-url.com/api" {
					t.Errorf("Expected URL to be 'https://direct-url.com/api', got %q", apiDeployment.Spec.Upstream.Main.URL)
				}
				if apiDeployment.Spec.Upstream.Main.Ref != "" {
					t.Errorf("Expected Ref to be cleared, got %q", apiDeployment.Spec.Upstream.Main.Ref)
				}
			},
		},
		{
			name: "preserve sandbox endpoint",
			inputYAML: `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: test-api
spec:
  displayName: Test API
  version: v1
  context: /test
  upstream:
    main:
      url: http://prod.example.com
    sandbox:
      url: http://sandbox.example.com
  operations: []
`,
			newURL:  "https://new-prod.example.com",
			wantErr: false,
			validateResult: func(t *testing.T, result []byte) {
				var apiDeployment dto.APIDeploymentYAML
				if err := yaml.Unmarshal(result, &apiDeployment); err != nil {
					t.Fatalf("Failed to unmarshal result YAML: %v", err)
				}
				if apiDeployment.Spec.Upstream.Main.URL != "https://new-prod.example.com" {
					t.Errorf("Expected main URL to be updated to 'https://new-prod.example.com', got %q", apiDeployment.Spec.Upstream.Main.URL)
				}
				if apiDeployment.Spec.Upstream.Sandbox == nil || apiDeployment.Spec.Upstream.Sandbox.URL != "http://sandbox.example.com" {
					t.Error("Expected sandbox URL to be preserved")
				}
			},
		},
		{
			name:        "invalid YAML",
			inputYAML:   `invalid: yaml: [unclosed`,
			newURL:      "http://example.com",
			wantErr:     true,
			errContains: "failed to parse deployment YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := overrideEndpointURL([]byte(tt.inputYAML), tt.newURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("overrideEndpointURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("overrideEndpointURL() error = %v, want error containing %q", err, tt.errContains)
				return
			}
			if !tt.wantErr && tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}

// TestOverrideEndpointURLPreservesOtherFields tests that the override preserves other YAML fields
func TestOverrideEndpointURLPreservesOtherFields(t *testing.T) {
	inputYAML := `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: my-api-123
spec:
  displayName: My Test API
  version: v2.0
  context: /myapi/v2
  upstream:
    main:
      url: http://old.example.com:8080/api
  operations:
    - method: GET
      path: /users
    - method: POST
      path: /users
`

	result, err := overrideEndpointURL([]byte(inputYAML), "https://new.example.com:9090/api/v2")
	if err != nil {
		t.Fatalf("overrideEndpointURL() failed: %v", err)
	}

	var apiDeployment dto.APIDeploymentYAML
	if err := yaml.Unmarshal(result, &apiDeployment); err != nil {
		t.Fatalf("Failed to unmarshal result YAML: %v", err)
	}

	// Verify all fields are preserved
	if apiDeployment.ApiVersion != "gateway.api-platform.wso2.com/v1alpha1" {
		t.Errorf("ApiVersion not preserved, got %q", apiDeployment.ApiVersion)
	}
	if apiDeployment.Kind != "RestApi" {
		t.Errorf("Kind not preserved, got %q", apiDeployment.Kind)
	}
	if apiDeployment.Metadata.Name != "my-api-123" {
		t.Errorf("Metadata.Name not preserved, got %q", apiDeployment.Metadata.Name)
	}
	if apiDeployment.Spec.DisplayName != "My Test API" {
		t.Errorf("Spec.DisplayName not preserved, got %q", apiDeployment.Spec.DisplayName)
	}
	if apiDeployment.Spec.Version != "v2.0" {
		t.Errorf("Spec.Version not preserved, got %q", apiDeployment.Spec.Version)
	}
	if apiDeployment.Spec.Context != "/myapi/v2" {
		t.Errorf("Spec.Context not preserved, got %q", apiDeployment.Spec.Context)
	}
	if len(apiDeployment.Spec.Operations) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(apiDeployment.Spec.Operations))
	}

	// Verify URL was updated
	if apiDeployment.Spec.Upstream.Main.URL != "https://new.example.com:9090/api/v2" {
		t.Errorf("Expected URL to be updated to 'https://new.example.com:9090/api/v2', got %q", apiDeployment.Spec.Upstream.Main.URL)
	}
}

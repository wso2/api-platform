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

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func TestNewAPIValidator(t *testing.T) {
	v := NewAPIValidator()
	if v == nil {
		t.Fatal("NewAPIValidator returned nil")
	}
	if v.pathParamRegex == nil {
		t.Error("pathParamRegex should not be nil")
	}
	if v.versionRegex == nil {
		t.Error("versionRegex should not be nil")
	}
	if v.urlFriendlyNameRegex == nil {
		t.Error("urlFriendlyNameRegex should not be nil")
	}
}

func TestAPIValidator_SetPolicyValidator(t *testing.T) {
	v := NewAPIValidator()
	pv := NewPolicyValidator(nil)

	v.SetPolicyValidator(pv)

	if v.policyValidator != pv {
		t.Error("policyValidator not set correctly")
	}
}

func TestAPIValidator_Validate_UnsupportedType(t *testing.T) {
	v := NewAPIValidator()

	// Test with unsupported type
	errors := v.Validate("invalid type")
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if !strings.Contains(errors[0].Message, "Unsupported configuration type") {
		t.Errorf("expected unsupported type error, got: %s", errors[0].Message)
	}
}

func TestAPIValidator_Validate_PointerAndValue(t *testing.T) {
	v := NewAPIValidator()

	config := createValidRestAPIConfig()

	// Test with pointer
	errorsPtr := v.Validate(config)
	if len(errorsPtr) != 0 {
		t.Errorf("expected no errors for pointer, got %d: %v", len(errorsPtr), errorsPtr)
	}

	// Test with value
	errorsVal := v.Validate(*config)
	if len(errorsVal) != 0 {
		t.Errorf("expected no errors for value, got %d: %v", len(errorsVal), errorsVal)
	}
}

func TestAPIValidator_ValidateAPIVersion(t *testing.T) {
	v := NewAPIValidator()

	tests := []struct {
		name       string
		apiVersion api.RestAPIApiVersion
		wantError  bool
	}{
		{
			name:       "Valid API version",
			apiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1alpha1,
			wantError:  false,
		},
		{
			name:       "Invalid API version",
			apiVersion: "invalid-version",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidRestAPIConfig()
			config.ApiVersion = tt.apiVersion

			errors := v.Validate(config)
			hasVersionError := false
			for _, e := range errors {
				if e.Field == "version" {
					hasVersionError = true
					break
				}
			}
			if tt.wantError && !hasVersionError {
				t.Error("expected version error, got none")
			}
			if !tt.wantError && hasVersionError {
				t.Error("unexpected version error")
			}
		})
	}
}

func TestAPIValidator_ValidateKind(t *testing.T) {
	v := NewAPIValidator()

	// Test RestApi kind - valid
	t.Run("Valid RestApi kind", func(t *testing.T) {
		config := createValidRestAPIConfig()
		errors := v.Validate(config)
		hasKindError := false
		for _, e := range errors {
			if e.Field == "kind" {
				hasKindError = true
				break
			}
		}
		if hasKindError {
			t.Error("unexpected kind error")
		}
	})

	// Test WebSubApi kind - valid
	t.Run("Valid WebSubApi kind", func(t *testing.T) {
		config := createValidWebSubAPIConfig()
		errors := v.Validate(config)
		hasKindError := false
		for _, e := range errors {
			if e.Field == "kind" {
				hasKindError = true
				break
			}
		}
		if hasKindError {
			t.Error("unexpected kind error")
		}
	})

	// Test unsupported type
	t.Run("Unsupported type", func(t *testing.T) {
		errors := v.Validate("InvalidKind")
		if len(errors) == 0 {
			t.Error("expected error for unsupported type, got none")
		}
	})

	// Test invalid Kind on RestAPI
	t.Run("Invalid RestApi kind", func(t *testing.T) {
		config := createValidRestAPIConfig()
		config.Kind = "InvalidKind"
		errors := v.Validate(config)
		hasKindError := false
		for _, e := range errors {
			if e.Field == "kind" {
				hasKindError = true
				break
			}
		}
		if !hasKindError {
			t.Error("expected kind error for invalid RestAPI kind, got none")
		}
	})

	// Test invalid Kind on WebSubAPI
	t.Run("Invalid WebSubApi kind", func(t *testing.T) {
		config := createValidWebSubAPIConfig()
		config.Kind = "InvalidKind"
		errors := v.Validate(config)
		hasKindError := false
		for _, e := range errors {
			if e.Field == "kind" {
				hasKindError = true
				break
			}
		}
		if !hasKindError {
			t.Error("expected kind error for invalid WebSubAPI kind, got none")
		}
	})
}

func TestAPIValidator_ValidateDisplayName(t *testing.T) {
	v := NewAPIValidator()

	tests := []struct {
		name        string
		displayName string
		wantError   bool
		errContains string
	}{
		{name: "Valid display name", displayName: "My API", wantError: false},
		{name: "Empty display name", displayName: "", wantError: true, errContains: "required"},
		{name: "Display name too long", displayName: strings.Repeat("a", 101), wantError: true, errContains: "1-100 characters"},
		{name: "Invalid characters", displayName: "Test@#$%", wantError: true, errContains: "URL-friendly"},
		{name: "Valid with special chars", displayName: "test-api_v1.0", wantError: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidRestAPIConfig()
			config.Spec.DisplayName = tt.displayName

			errors := v.Validate(config)
			hasDisplayNameError := false
			var errorMsg string
			for _, e := range errors {
				if e.Field == "spec.displayName" {
					hasDisplayNameError = true
					errorMsg = e.Message
					break
				}
			}
			if tt.wantError && !hasDisplayNameError {
				t.Error("expected displayName error, got none")
			}
			if !tt.wantError && hasDisplayNameError {
				t.Errorf("unexpected displayName error: %s", errorMsg)
			}
		})
	}
}

func TestAPIValidator_ValidateVersion(t *testing.T) {
	v := NewAPIValidator()

	tests := []struct {
		name      string
		version   string
		wantError bool
	}{
		{name: "Valid v1.0", version: "v1.0", wantError: false},
		{name: "Valid v2.1.3", version: "v2.1.3", wantError: false},
		{name: "Valid 1.0", version: "1.0", wantError: false},
		{name: "Valid v1", version: "v1", wantError: false},
		{name: "Empty version", version: "", wantError: true},
		{name: "Invalid version", version: "invalid", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidRestAPIConfig()
			config.Spec.Version = tt.version

			errors := v.Validate(config)
			hasVersionError := false
			for _, e := range errors {
				if e.Field == "spec.version" {
					hasVersionError = true
					break
				}
			}
			if tt.wantError && !hasVersionError {
				t.Error("expected version error, got none")
			}
			if !tt.wantError && hasVersionError {
				t.Error("unexpected version error")
			}
		})
	}
}

func TestAPIValidator_ValidateContext(t *testing.T) {
	v := NewAPIValidator()

	tests := []struct {
		name      string
		context   string
		wantError bool
		errMsg    string
	}{
		{name: "Valid context", context: "/api", wantError: false},
		{name: "Empty context", context: "", wantError: true, errMsg: "required"},
		{name: "Context without leading slash", context: "api", wantError: true, errMsg: "start with /"},
		{name: "Context with trailing slash", context: "/api/", wantError: true, errMsg: "cannot end with /"},
		{name: "Root context allowed", context: "/", wantError: false},
		{name: "Context too long", context: "/" + strings.Repeat("a", 201), wantError: true, errMsg: "1-200 characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidRestAPIConfig()
			config.Spec.Context = tt.context

			errors := v.Validate(config)
			hasContextError := false
			for _, e := range errors {
				if e.Field == "spec.context" {
					hasContextError = true
					break
				}
			}
			if tt.wantError && !hasContextError {
				t.Errorf("expected context error, got none. Errors: %v", errors)
			}
			if !tt.wantError && hasContextError {
				t.Error("unexpected context error")
			}
		})
	}
}

func TestAPIValidator_ValidateUpstream(t *testing.T) {
	v := NewAPIValidator()

	tests := []struct {
		name      string
		mainURL   *string
		mainRef   *string
		wantError bool
		errField  string
	}{
		{name: "Valid URL", mainURL: stringPtr("http://backend:8080"), mainRef: nil, wantError: false},
		{name: "Valid HTTPS URL", mainURL: stringPtr("https://backend:8443"), mainRef: nil, wantError: false},
		{name: "Both URL and Ref set", mainURL: stringPtr("http://x"), mainRef: stringPtr("ref"), wantError: true, errField: "spec.upstream.main"},
		{name: "Empty URL", mainURL: stringPtr(""), mainRef: nil, wantError: true, errField: "spec.upstream.main.url"},
		{name: "Invalid URL scheme", mainURL: stringPtr("ftp://x"), mainRef: nil, wantError: true, errField: "spec.upstream.main.url"},
		{name: "URL without host", mainURL: stringPtr("http:///path"), mainRef: nil, wantError: true, errField: "spec.upstream.main.url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidRestAPIConfig()
			config.Spec.Upstream.Main.Url = tt.mainURL
			config.Spec.Upstream.Main.Ref = tt.mainRef

			errors := v.Validate(config)
			hasExpectedError := false
			for _, e := range errors {
				if strings.HasPrefix(e.Field, tt.errField) {
					hasExpectedError = true
					break
				}
			}
			if tt.wantError && !hasExpectedError {
				t.Errorf("expected error for field %s, got: %v", tt.errField, errors)
			}
		})
	}
}

func TestAPIValidator_ValidateSandboxUpstream(t *testing.T) {
	v := NewAPIValidator()

	config := createValidRestAPIConfig()
	config.Spec.Upstream.Sandbox = &api.Upstream{
		Url: stringPtr("http://sandbox:8080"),
	}

	errors := v.Validate(config)
	for _, e := range errors {
		if strings.Contains(e.Field, "sandbox") {
			t.Errorf("unexpected sandbox error: %v", e)
		}
	}
}

func TestAPIValidator_ValidateOperations(t *testing.T) {
	v := NewAPIValidator()

	tests := []struct {
		name       string
		operations []api.Operation
		wantError  bool
		errField   string
	}{
		{
			name: "Valid operations",
			operations: []api.Operation{
				{Method: api.OperationMethodGET, Path: "/items"},
				{Method: api.OperationMethodPOST, Path: "/items"},
			},
			wantError: false,
		},
		{
			name:       "Empty operations",
			operations: []api.Operation{},
			wantError:  true,
			errField:   "spec.operations",
		},
		{
			name: "Missing method",
			operations: []api.Operation{
				{Method: "", Path: "/items"},
			},
			wantError: true,
			errField:  "spec.operations[0].method",
		},
		{
			name: "Invalid method",
			operations: []api.Operation{
				{Method: "INVALID", Path: "/items"},
			},
			wantError: true,
			errField:  "spec.operations[0].method",
		},
		{
			name: "Missing path",
			operations: []api.Operation{
				{Method: api.OperationMethodGET, Path: ""},
			},
			wantError: true,
			errField:  "spec.operations[0].path",
		},
		{
			name: "Path without leading slash",
			operations: []api.Operation{
				{Method: api.OperationMethodGET, Path: "items"},
			},
			wantError: true,
			errField:  "spec.operations[0].path",
		},
		{
			name: "Valid path with parameters",
			operations: []api.Operation{
				{Method: api.OperationMethodGET, Path: "/items/{id}"},
			},
			wantError: false,
		},
		{
			name: "Path with unbalanced braces",
			operations: []api.Operation{
				{Method: api.OperationMethodGET, Path: "/items/{id"},
			},
			wantError: true,
			errField:  "spec.operations[0].path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidRestAPIConfig()
			config.Spec.Operations = tt.operations

			errors := v.Validate(config)
			hasExpectedError := false
			for _, e := range errors {
				if strings.HasPrefix(e.Field, tt.errField) {
					hasExpectedError = true
					break
				}
			}
			if tt.wantError && !hasExpectedError {
				t.Errorf("expected error for field %s, got: %v", tt.errField, errors)
			}
			if !tt.wantError && len(errors) > 0 {
				for _, e := range errors {
					if strings.Contains(e.Field, "operations") {
						t.Errorf("unexpected operations error: %v", e)
					}
				}
			}
		})
	}
}

func TestAPIValidator_ValidateAllHTTPMethods(t *testing.T) {
	v := NewAPIValidator()

	methods := []api.OperationMethod{
		api.OperationMethodGET,
		api.OperationMethodPOST,
		api.OperationMethodPUT,
		api.OperationMethodDELETE,
		api.OperationMethodPATCH,
		api.OperationMethodHEAD,
		api.OperationMethodOPTIONS,
	}

	for _, method := range methods {
		t.Run(string(method), func(t *testing.T) {
			config := createValidRestAPIConfig()
			config.Spec.Operations = []api.Operation{
				{Method: method, Path: "/test"},
			}

			errors := v.Validate(config)
			for _, e := range errors {
				if strings.Contains(e.Field, "method") {
					t.Errorf("unexpected method error for %s: %v", method, e)
				}
			}
		})
	}
}

func TestAPIValidator_ValidateWebSubAPI(t *testing.T) {
	v := NewAPIValidator()

	config := createValidWebSubAPIConfig()

	errors := v.Validate(config)
	if len(errors) != 0 {
		t.Errorf("expected no errors for valid WebSubApi, got: %v", errors)
	}
}

func TestAPIValidator_ValidateChannels(t *testing.T) {
	v := NewAPIValidator()

	tests := []struct {
		name      string
		channels  []api.Channel
		wantError bool
		errField  string
	}{
		{
			name: "Valid channels",
			channels: []api.Channel{
				{Name: "channel1"},
				{Name: "channel2"},
			},
			wantError: false,
		},
		{
			name:      "Empty channels",
			channels:  []api.Channel{},
			wantError: true,
			errField:  "spec.channels",
		},
		{
			name: "Missing channel name",
			channels: []api.Channel{
				{Name: ""},
			},
			wantError: true,
			errField:  "spec.channels[0].name",
		},
		{
			name: "Channel with braces (invalid)",
			channels: []api.Channel{
				{Name: "channel/{id}"},
			},
			wantError: true,
			errField:  "spec.channels[0].name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidWebSubAPIConfig()
			config.Spec.Channels = tt.channels

			errors := v.Validate(config)
			hasExpectedError := false
			for _, e := range errors {
				if strings.HasPrefix(e.Field, tt.errField) {
					hasExpectedError = true
					break
				}
			}
			if tt.wantError && !hasExpectedError {
				t.Errorf("expected error for field %s, got: %v", tt.errField, errors)
			}
		})
	}
}

func TestAPIValidator_ValidateAsyncDisplayName(t *testing.T) {
	v := NewAPIValidator()

	tests := []struct {
		name        string
		displayName string
		wantError   bool
	}{
		{name: "Valid name", displayName: "MyWebSub", wantError: false},
		{name: "Empty name", displayName: "", wantError: true},
		{name: "Name too long", displayName: strings.Repeat("a", 101), wantError: true},
		{name: "Invalid characters", displayName: "test@#$", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidWebSubAPIConfig()
			config.Spec.DisplayName = tt.displayName

			errors := v.Validate(config)
			hasNameError := false
			for _, e := range errors {
				if e.Field == "spec.name" {
					hasNameError = true
					break
				}
			}
			if tt.wantError && !hasNameError {
				t.Error("expected name error, got none")
			}
			if !tt.wantError && hasNameError {
				t.Error("unexpected name error")
			}
		})
	}
}

func TestAPIValidator_ValidatePathParameters(t *testing.T) {
	v := NewAPIValidator()

	tests := []struct {
		path     string
		expected bool
	}{
		{"/items/{id}", true},
		{"/items/{id}/sub/{subId}", true},
		{"/items", true},
		{"/items/{id", false},
		{"/items/id}", false},
		{"/items/{id}/{", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := v.validatePathParameters(tt.path)
			if result != tt.expected {
				t.Errorf("validatePathParameters(%s) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestAPIValidator_ValidatePathParametersForAsyncAPIs(t *testing.T) {
	v := NewAPIValidator()

	tests := []struct {
		path     string
		expected bool
	}{
		{"channel1", true},
		{"my-channel", true},
		{"channel/{id}", false},
		{"{channel}", false},
		{"channel}", false},
		{"channel{", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := v.validatePathParametersForAsyncAPIs(tt.path)
			if result != tt.expected {
				t.Errorf("validatePathParametersForAsyncAPIs(%s) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// Helper functions

func createValidRestAPIConfig() *api.RestAPI {
	return &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.RestAPIKindRestApi,
		Metadata: api.Metadata{
			Name: "test-api",
		},
		Spec: api.APIConfigData{
			DisplayName: "Test API",
			Version:     "v1.0",
			Context:     "/test",
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{
					Url: stringPtr("http://backend:8080"),
				},
			},
			Operations: []api.Operation{
				{Method: api.OperationMethodGET, Path: "/items"},
			},
		},
	}
}

func createValidWebSubAPIConfig() *api.WebSubAPI {
	return &api.WebSubAPI{
		ApiVersion: api.WebSubAPIApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.WebSubAPIKindWebSubApi,
		Metadata: api.Metadata{
			Name: "test-websub",
		},
		Spec: api.WebhookAPIData{
			DisplayName: "Test WebSub",
			Version:     "v1.0",
			Context:     "/websub",
			Channels: []api.Channel{
				{Name: "channel1"},
			},
		},
	}
}

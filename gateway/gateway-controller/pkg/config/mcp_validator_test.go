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
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

func TestNewMCPValidator(t *testing.T) {
	v := NewMCPValidator()
	if v == nil {
		t.Fatal("NewMCPValidator returned nil")
	}
	if v.versionRegex == nil {
		t.Error("versionRegex should not be nil")
	}
	if v.urlFriendlyNameRegex == nil {
		t.Error("urlFriendlyNameRegex should not be nil")
	}
	if len(v.supportedSpecVersions) == 0 {
		t.Error("supportedSpecVersions should not be empty")
	}
}

func TestMCPValidator_Validate_UnsupportedType(t *testing.T) {
	v := NewMCPValidator()

	// Test with unsupported type
	errors := v.Validate("invalid type")
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if !strings.Contains(errors[0].Message, "Unsupported configuration type") {
		t.Errorf("expected unsupported type error, got: %s", errors[0].Message)
	}
}

func TestMCPValidator_Validate_PointerAndValue(t *testing.T) {
	v := NewMCPValidator()

	// Create valid config
	url := "http://backend:8080"
	specVersion := constants.SPEC_VERSION_2025_JUNE
	config := api.MCPProxyConfiguration{
		ApiVersion: api.GatewayApiPlatformWso2Comv1alpha1,
		Kind:       "Mcp",
		Metadata: api.Metadata{
			Name: "test-mcp",
		},
		Spec: api.MCPProxyConfigData{
			DisplayName: "Test MCP",
			Version:     "v1.0",
			Context:     stringPtr("/test"),
			SpecVersion: &specVersion,
			Upstream: api.MCPProxyConfigData_Upstream{
				Url: &url,
			},
		},
	}

	// Test with pointer
	errorsPtr := v.Validate(&config)
	if len(errorsPtr) != 0 {
		t.Errorf("expected no errors for pointer, got %d: %v", len(errorsPtr), errorsPtr)
	}

	// Test with value
	errorsVal := v.Validate(config)
	if len(errorsVal) != 0 {
		t.Errorf("expected no errors for value, got %d: %v", len(errorsVal), errorsVal)
	}
}

func TestMCPValidator_ValidateAPIVersion(t *testing.T) {
	v := NewMCPValidator()

	tests := []struct {
		name       string
		apiVersion api.MCPProxyConfigurationApiVersion
		wantError  bool
	}{
		{
			name:       "Valid API version",
			apiVersion: api.GatewayApiPlatformWso2Comv1alpha1,
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
			url := "http://backend:8080"
			specVersion := constants.SPEC_VERSION_2025_JUNE
			config := &api.MCPProxyConfiguration{
				ApiVersion: tt.apiVersion,
				Kind:       "Mcp",
				Metadata:   api.Metadata{Name: "test"},
				Spec: api.MCPProxyConfigData{
					DisplayName: "Test",
					Version:     "v1.0",
					Context:     stringPtr("/test"),
					SpecVersion: &specVersion,
					Upstream:    api.MCPProxyConfigData_Upstream{Url: &url},
				},
			}

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

func TestMCPValidator_ValidateKind(t *testing.T) {
	v := NewMCPValidator()

	tests := []struct {
		name      string
		kind      api.MCPProxyConfigurationKind
		wantError bool
	}{
		{
			name:      "Valid kind Mcp",
			kind:      "Mcp",
			wantError: false,
		},
		{
			name:      "Invalid kind",
			kind:      "InvalidKind",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "http://backend:8080"
			specVersion := constants.SPEC_VERSION_2025_JUNE
			config := &api.MCPProxyConfiguration{
				ApiVersion: api.GatewayApiPlatformWso2Comv1alpha1,
				Kind:       tt.kind,
				Metadata:   api.Metadata{Name: "test"},
				Spec: api.MCPProxyConfigData{
					DisplayName: "Test",
					Version:     "v1.0",
					Context:     stringPtr("/test"),
					SpecVersion: &specVersion,
					Upstream:    api.MCPProxyConfigData_Upstream{Url: &url},
				},
			}

			errors := v.Validate(config)
			hasKindError := false
			for _, e := range errors {
				if e.Field == "kind" {
					hasKindError = true
					break
				}
			}
			if tt.wantError && !hasKindError {
				t.Error("expected kind error, got none")
			}
			if !tt.wantError && hasKindError {
				t.Error("unexpected kind error")
			}
		})
	}
}

func TestMCPValidator_ValidateDisplayName(t *testing.T) {
	v := NewMCPValidator()

	tests := []struct {
		name        string
		displayName string
		wantError   bool
		errContains string
	}{
		{
			name:        "Valid display name",
			displayName: "My MCP Proxy",
			wantError:   false,
		},
		{
			name:        "Empty display name",
			displayName: "",
			wantError:   true,
			errContains: "required",
		},
		{
			name:        "Display name too long",
			displayName: strings.Repeat("a", 101),
			wantError:   true,
			errContains: "1-100 characters",
		},
		{
			name:        "Invalid characters in display name",
			displayName: "Test@#$%",
			wantError:   true,
			errContains: "URL-friendly",
		},
		{
			name:        "Valid with hyphens and underscores",
			displayName: "test-proxy_v1.0",
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "http://backend:8080"
			specVersion := constants.SPEC_VERSION_2025_JUNE
			config := &api.MCPProxyConfiguration{
				ApiVersion: api.GatewayApiPlatformWso2Comv1alpha1,
				Kind:       "Mcp",
				Metadata:   api.Metadata{Name: "test"},
				Spec: api.MCPProxyConfigData{
					DisplayName: tt.displayName,
					Version:     "v1.0",
					Context:     stringPtr("/test"),
					SpecVersion: &specVersion,
					Upstream:    api.MCPProxyConfigData_Upstream{Url: &url},
				},
			}

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
			if tt.wantError && tt.errContains != "" && !strings.Contains(errorMsg, tt.errContains) {
				t.Errorf("expected error containing '%s', got: %s", tt.errContains, errorMsg)
			}
		})
	}
}

func TestMCPValidator_ValidateVersion(t *testing.T) {
	v := NewMCPValidator()

	tests := []struct {
		name        string
		version     string
		wantError   bool
		errContains string
	}{
		{name: "Valid v1.0", version: "v1.0", wantError: false},
		{name: "Valid v2.1.3", version: "v2.1.3", wantError: false},
		{name: "Valid 1.0", version: "1.0", wantError: false},
		{name: "Valid v1", version: "v1", wantError: false},
		{name: "Empty version", version: "", wantError: true, errContains: "required"},
		{name: "Invalid version", version: "invalid", wantError: true, errContains: "semantic versioning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "http://backend:8080"
			specVersion := constants.SPEC_VERSION_2025_JUNE
			config := &api.MCPProxyConfiguration{
				ApiVersion: api.GatewayApiPlatformWso2Comv1alpha1,
				Kind:       "Mcp",
				Metadata:   api.Metadata{Name: "test"},
				Spec: api.MCPProxyConfigData{
					DisplayName: "Test",
					Version:     tt.version,
					Context:     stringPtr("/test"),
					SpecVersion: &specVersion,
					Upstream:    api.MCPProxyConfigData_Upstream{Url: &url},
				},
			}

			errors := v.Validate(config)
			hasVersionError := false
			var errorMsg string
			for _, e := range errors {
				if e.Field == "spec.version" {
					hasVersionError = true
					errorMsg = e.Message
					break
				}
			}
			if tt.wantError && !hasVersionError {
				t.Error("expected version error, got none")
			}
			if !tt.wantError && hasVersionError {
				t.Errorf("unexpected version error: %s", errorMsg)
			}
		})
	}
}

func TestMCPValidator_ValidateSpecVersion(t *testing.T) {
	v := NewMCPValidator()

	tests := []struct {
		name        string
		specVersion *string
		wantError   bool
	}{
		{name: "Valid June 2025", specVersion: stringPtr(constants.SPEC_VERSION_2025_JUNE), wantError: false},
		{name: "Valid November 2025", specVersion: stringPtr(constants.SPEC_VERSION_2025_NOVEMBER), wantError: false},
		{name: "Nil spec version", specVersion: nil, wantError: false},
		{name: "Invalid spec version", specVersion: stringPtr("invalid-version"), wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "http://backend:8080"
			config := &api.MCPProxyConfiguration{
				ApiVersion: api.GatewayApiPlatformWso2Comv1alpha1,
				Kind:       "Mcp",
				Metadata:   api.Metadata{Name: "test"},
				Spec: api.MCPProxyConfigData{
					DisplayName: "Test",
					Version:     "v1.0",
					Context:     stringPtr("/test"),
					SpecVersion: tt.specVersion,
					Upstream:    api.MCPProxyConfigData_Upstream{Url: &url},
				},
			}

			errors := v.Validate(config)
			hasSpecVersionError := false
			for _, e := range errors {
				if e.Field == "spec.specVersion" {
					hasSpecVersionError = true
					break
				}
			}
			if tt.wantError && !hasSpecVersionError {
				t.Error("expected specVersion error, got none")
			}
			if !tt.wantError && hasSpecVersionError {
				t.Error("unexpected specVersion error")
			}
		})
	}
}

func TestMCPValidator_ValidateContextAndVhost(t *testing.T) {
	v := NewMCPValidator()

	tests := []struct {
		name      string
		context   *string
		vhost     *string
		wantError bool
		errField  string
	}{
		{name: "Valid context", context: stringPtr("/api"), vhost: nil, wantError: false},
		{name: "Context without leading slash", context: stringPtr("api"), vhost: nil, wantError: true, errField: "spec.context"},
		{name: "Context with trailing slash", context: stringPtr("/api/"), vhost: nil, wantError: true, errField: "spec.context"},
		{name: "Root context allowed", context: stringPtr("/"), vhost: nil, wantError: false},
		{name: "Context too long", context: stringPtr("/" + strings.Repeat("a", 201)), vhost: nil, wantError: true, errField: "spec.context"},
		{name: "No context but has vhost", context: nil, vhost: stringPtr("api.example.com"), wantError: false},
		{name: "No context and no vhost", context: nil, vhost: nil, wantError: true, errField: "spec.vhost"},
		{name: "Empty context no vhost", context: stringPtr(""), vhost: nil, wantError: true, errField: "spec.vhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "http://backend:8080"
			specVersion := constants.SPEC_VERSION_2025_JUNE
			config := &api.MCPProxyConfiguration{
				ApiVersion: api.GatewayApiPlatformWso2Comv1alpha1,
				Kind:       "Mcp",
				Metadata:   api.Metadata{Name: "test"},
				Spec: api.MCPProxyConfigData{
					DisplayName: "Test",
					Version:     "v1.0",
					Context:     tt.context,
					Vhost:       tt.vhost,
					SpecVersion: &specVersion,
					Upstream:    api.MCPProxyConfigData_Upstream{Url: &url},
				},
			}

			errors := v.Validate(config)
			hasExpectedError := false
			for _, e := range errors {
				if tt.errField != "" && e.Field == tt.errField {
					hasExpectedError = true
					break
				}
			}
			if tt.wantError && !hasExpectedError {
				t.Errorf("expected error for field %s, got none. Errors: %v", tt.errField, errors)
			}
			if !tt.wantError && len(errors) > 0 {
				// Check if any errors are context/vhost related
				for _, e := range errors {
					if strings.Contains(e.Field, "context") || strings.Contains(e.Field, "vhost") {
						t.Errorf("unexpected context/vhost error: %v", e)
					}
				}
			}
		})
	}
}

func TestMCPValidator_ValidateUpstream(t *testing.T) {
	v := NewMCPValidator()

	tests := []struct {
		name      string
		upstream  *api.MCPProxyConfigData_Upstream
		wantError bool
		errField  string
	}{
		{
			name:      "Valid HTTP URL",
			upstream:  &api.MCPProxyConfigData_Upstream{Url: stringPtr("http://backend:8080")},
			wantError: false,
		},
		{
			name:      "Valid HTTPS URL",
			upstream:  &api.MCPProxyConfigData_Upstream{Url: stringPtr("https://backend:8443/path")},
			wantError: false,
		},
		{
			name:      "Nil upstream",
			upstream:  nil,
			wantError: true,
			errField:  "spec.upstream",
		},
		{
			name:      "Nil URL",
			upstream:  &api.MCPProxyConfigData_Upstream{Url: nil},
			wantError: true,
			errField:  "spec.upstream.url",
		},
		{
			name:      "Empty URL",
			upstream:  &api.MCPProxyConfigData_Upstream{Url: stringPtr("")},
			wantError: true,
			errField:  "spec.upstream.url",
		},
		{
			name:      "Invalid URL scheme",
			upstream:  &api.MCPProxyConfigData_Upstream{Url: stringPtr("ftp://backend:21")},
			wantError: true,
			errField:  "spec.upstream.url",
		},
		{
			name:      "URL without host",
			upstream:  &api.MCPProxyConfigData_Upstream{Url: stringPtr("http:///path")},
			wantError: true,
			errField:  "spec.upstream.url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specVersion := constants.SPEC_VERSION_2025_JUNE
			spec := api.MCPProxyConfigData{
				DisplayName: "Test",
				Version:     "v1.0",
				Context:     stringPtr("/test"),
				SpecVersion: &specVersion,
			}
			if tt.upstream != nil {
				spec.Upstream = *tt.upstream
			}

			config := &api.MCPProxyConfiguration{
				ApiVersion: api.GatewayApiPlatformWso2Comv1alpha1,
				Kind:       "Mcp",
				Metadata:   api.Metadata{Name: "test"},
				Spec:       spec,
			}

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

func TestMCPValidator_ValidateUpstreamAuth(t *testing.T) {
	v := NewMCPValidator()

	// Define auth struct type locally to match the anonymous struct in api package
	type authConfig struct {
		Type   api.MCPProxyConfigDataUpstreamAuthType
		Header *string
		Value  *string
	}

	tests := []struct {
		name      string
		auth      *authConfig
		wantError bool
		errField  string
	}{
		{
			name: "Valid API key auth",
			auth: &authConfig{
				Type:   api.MCPProxyConfigDataUpstreamAuthTypeApiKey,
				Header: stringPtr("X-API-Key"),
				Value:  stringPtr("secret-key"),
			},
			wantError: false,
		},
		{
			name: "Valid bearer auth",
			auth: &authConfig{
				Type:   api.MCPProxyConfigDataUpstreamAuthTypeBearer,
				Header: stringPtr("Authorization"),
				Value:  stringPtr("Bearer token123"),
			},
			wantError: false,
		},
		{
			name: "Bearer auth without Bearer prefix",
			auth: &authConfig{
				Type:   api.MCPProxyConfigDataUpstreamAuthTypeBearer,
				Header: stringPtr("Authorization"),
				Value:  stringPtr("token123"),
			},
			wantError: true,
			errField:  "spec.upstream.auth.value",
		},
		{
			name: "Missing auth type",
			auth: &authConfig{
				Type:   "",
				Header: stringPtr("X-API-Key"),
				Value:  stringPtr("secret"),
			},
			wantError: true,
			errField:  "spec.upstream.auth.type",
		},
		{
			name: "Missing auth header",
			auth: &authConfig{
				Type:   api.MCPProxyConfigDataUpstreamAuthTypeApiKey,
				Header: nil,
				Value:  stringPtr("secret"),
			},
			wantError: true,
			errField:  "spec.upstream.auth.header",
		},
		{
			name: "Missing auth value",
			auth: &authConfig{
				Type:   api.MCPProxyConfigDataUpstreamAuthTypeApiKey,
				Header: stringPtr("X-API-Key"),
				Value:  nil,
			},
			wantError: true,
			errField:  "spec.upstream.auth.value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specVersion := constants.SPEC_VERSION_2025_JUNE
			upstream := api.MCPProxyConfigData_Upstream{
				Url: stringPtr("http://backend:8080"),
			}
			if tt.auth != nil {
				upstream.Auth = &struct {
					Header *string                                `json:"header,omitempty" yaml:"header,omitempty"`
					Type   api.MCPProxyConfigDataUpstreamAuthType `json:"type" yaml:"type"`
					Value  *string                                `json:"value,omitempty" yaml:"value,omitempty"`
				}{
					Type:   tt.auth.Type,
					Header: tt.auth.Header,
					Value:  tt.auth.Value,
				}
			}
			config := &api.MCPProxyConfiguration{
				ApiVersion: api.GatewayApiPlatformWso2Comv1alpha1,
				Kind:       "Mcp",
				Metadata:   api.Metadata{Name: "test"},
				Spec: api.MCPProxyConfigData{
					DisplayName: "Test",
					Version:     "v1.0",
					Context:     stringPtr("/test"),
					SpecVersion: &specVersion,
					Upstream:    upstream,
				},
			}

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
			if !tt.wantError {
				for _, e := range errors {
					if strings.Contains(e.Field, "auth") {
						t.Errorf("unexpected auth error: %v", e)
					}
				}
			}
		})
	}
}

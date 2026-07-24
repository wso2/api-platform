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
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// opRef builds the inline per-operation upstream target holding a ref.
func opRef(ref string) *struct {
	Ref api.UpstreamReference `json:"ref" yaml:"ref"`
} {
	return &struct {
		Ref api.UpstreamReference `json:"ref" yaml:"ref"`
	}{Ref: ref}
}

func TestValidator_URLFriendlyName(t *testing.T) {
	validator := NewAPIValidator()

	tests := []struct {
		name        string
		apiName     string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "valid name with spaces",
			apiName:     "Weather API",
			shouldError: false,
		},
		{
			name:        "valid name with hyphens",
			apiName:     "Weather-API",
			shouldError: false,
		},
		{
			name:        "valid name with underscores",
			apiName:     "Weather_API",
			shouldError: false,
		},
		{
			name:        "valid name with dots",
			apiName:     "Weather.API",
			shouldError: false,
		},
		{
			name:        "valid name alphanumeric",
			apiName:     "WeatherAPI123",
			shouldError: false,
		},
		{
			name:        "invalid name with slash",
			apiName:     "Weather/API",
			shouldError: true,
			errorMsg:    "API display name must be URL-friendly",
		},
		{
			name:        "invalid name with question mark",
			apiName:     "Weather?API",
			shouldError: true,
			errorMsg:    "API display name must be URL-friendly",
		},
		{
			name:        "invalid name with ampersand",
			apiName:     "Weather&API",
			shouldError: true,
			errorMsg:    "API display name must be URL-friendly",
		},
		{
			name:        "invalid name with hash",
			apiName:     "Weather#API",
			shouldError: true,
			errorMsg:    "API display name must be URL-friendly",
		},
		{
			name:        "invalid name with percent",
			apiName:     "Weather%API",
			shouldError: true,
			errorMsg:    "API display name must be URL-friendly",
		},
		{
			name:        "invalid name with brackets",
			apiName:     "Weather[API]",
			shouldError: true,
			errorMsg:    "API display name must be URL-friendly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &api.RestAPI{
				ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
				Kind:       api.RestAPIKindRestApi,
				Spec: api.APIConfigData{
					DisplayName: tt.apiName,
					Version:     "v1.0",
					Context:     "/test",
					Upstream: struct {
						Main    api.Upstream  `json:"main" yaml:"main"`
						Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
					}{
						Main: api.Upstream{
							Url: func() *string { s := "http://example.com"; return &s }(),
						},
					},
					Operations: []api.Operation{
						{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/test")},
					},
				},
			}

			errors := validator.Validate(config)

			// Check if we got errors when we expected them
			hasNameError := false
			for _, err := range errors {
				if err.Field == "spec.displayName" {
					hasNameError = true
					if tt.shouldError && tt.errorMsg != "" {
						if err.Message[:len(tt.errorMsg)] != tt.errorMsg {
							t.Errorf("Expected error message to start with '%s', got '%s'", tt.errorMsg, err.Message)
						}
					}
					break
				}
			}

			if tt.shouldError && !hasNameError {
				t.Errorf("Expected validation error for name '%s', but got none", tt.apiName)
			}

			if !tt.shouldError && hasNameError {
				t.Errorf("Did not expect validation error for name '%s', but got one", tt.apiName)
			}
		})
	}
}

func TestValidateAuthConfig_BothAuthDisabled_AllowsNoAuthMode(t *testing.T) {
	// Test that validation allows no-auth mode when both auth methods are disabled
	config := &Config{
		Controller: Controller{
			Auth: AuthConfig{
				Basic: BasicAuth{
					Enabled: false,
				},
				IDP: IDPConfig{
					Enabled: false,
				},
			},
		},
	}

	err := config.validateAuthConfig()
	assert.NoError(t, err)
}

func TestValidateAuthConfig_BasicAuthEnabled(t *testing.T) {
	// Test that validation passes when basic auth is enabled
	config := &Config{
		Controller: Controller{
			Auth: AuthConfig{
				Basic: BasicAuth{
					Enabled: true,
					Users: []AuthUser{
						{Username: "admin", Password: "pass", Roles: []string{"admin"}},
					},
				},
				IDP: IDPConfig{
					Enabled: false,
				},
			},
		},
	}

	err := config.validateAuthConfig()
	assert.NoError(t, err)
}

func TestValidateAuthConfig_IDPAuthEnabled(t *testing.T) {
	// Test that validation passes when IDP auth is enabled
	config := &Config{
		Controller: Controller{
			Auth: AuthConfig{
				Basic: BasicAuth{
					Enabled: false,
				},
				IDP: IDPConfig{
					Enabled: true,
					JWKSURL: "https://idp.example.com/jwks",
				},
			},
		},
	}

	err := config.validateAuthConfig()
	assert.NoError(t, err)
}

func TestValidateAuthConfig_BothAuthEnabled(t *testing.T) {
	// Test that validation passes when both auth methods are enabled
	config := &Config{
		Controller: Controller{
			Auth: AuthConfig{
				Basic: BasicAuth{
					Enabled: true,
					Users: []AuthUser{
						{Username: "admin", Password: "pass", Roles: []string{"admin"}},
					},
				},
				IDP: IDPConfig{
					Enabled: true,
					JWKSURL: "https://idp.example.com/jwks",
				},
			},
		},
	}

	err := config.validateAuthConfig()
	assert.NoError(t, err)
}

func TestValidateAuthConfig_BasicAuthEnabledNoUsers_AllowsNoAuthMode(t *testing.T) {
	// Basic auth enabled with an empty user list is allowed: it degrades to the
	// auth middleware's no-auth passthrough (which logs its own warning). Only a
	// user that is present but empty-valued is rejected — see the empty-credential
	// test below. The shipped config always defines a user, so its unset-env case
	// hits that check, not this one.
	config := &Config{
		Controller: Controller{
			Auth: AuthConfig{
				Basic: BasicAuth{
					Enabled: true,
					Users:   []AuthUser{},
				},
			},
		},
	}

	err := config.validateAuthConfig()
	assert.NoError(t, err)
}

func TestValidateAuthConfig_BasicAuthEnabledEmptyCredential_FailsClosed(t *testing.T) {
	// A user present but with an empty username or password (e.g. an unset
	// {{ env }} token) is just as unenforceable as no user at all.
	for _, tc := range []struct {
		name string
		user AuthUser
	}{
		{"empty password", AuthUser{Username: "admin", Password: "", Roles: []string{"admin"}}},
		{"empty username", AuthUser{Username: "", Password: "hash", Roles: []string{"admin"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := &Config{
				Controller: Controller{
					Auth: AuthConfig{
						Basic: BasicAuth{
							Enabled: true,
							Users:   []AuthUser{tc.user},
						},
					},
				},
			}

			err := config.validateAuthConfig()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "empty username or password")
		})
	}
}

func TestValidator_LabelsValidation(t *testing.T) {
	validator := NewAPIValidator()

	tests := []struct {
		name        string
		labels      map[string]string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "valid labels without spaces",
			labels:      map[string]string{"environment": "production", "team": "backend"},
			shouldError: false,
		},
		{
			name:        "valid labels with underscores and hyphens",
			labels:      map[string]string{"app-name": "test-api", "team_id": "123"},
			shouldError: false,
		},
		{
			name:        "valid empty labels map",
			labels:      map[string]string{},
			shouldError: false,
		},
		{
			name:        "valid nil labels",
			labels:      nil,
			shouldError: false,
		},
		{
			name:        "invalid label key with space",
			labels:      map[string]string{"My Label": "value"},
			shouldError: true,
			errorMsg:    "contains whitespace",
		},
		{
			name:        "invalid label key with multiple spaces",
			labels:      map[string]string{"My Label Key": "value"},
			shouldError: true,
			errorMsg:    "contains whitespace",
		},
		{
			name:        "invalid label key with leading space",
			labels:      map[string]string{" label": "value"},
			shouldError: true,
			errorMsg:    "contains whitespace",
		},
		{
			name:        "invalid label key with trailing space",
			labels:      map[string]string{"label ": "value"},
			shouldError: true,
			errorMsg:    "contains whitespace",
		},
		{
			name:        "multiple labels with one invalid",
			labels:      map[string]string{"valid-key": "value1", "Invalid Key": "value2"},
			shouldError: true,
			errorMsg:    "contains whitespace",
		},
		{
			name:        "label value can contain spaces",
			labels:      map[string]string{"key": "value with spaces"},
			shouldError: false,
		},
		{
			name:        "invalid label key with tab character",
			labels:      map[string]string{"key\twith\ttab": "value"},
			shouldError: true,
			errorMsg:    "contains whitespace",
		},
		{
			name:        "invalid label key with newline character",
			labels:      map[string]string{"key\nwith\nnewline": "value"},
			shouldError: true,
			errorMsg:    "contains whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &api.RestAPI{
				ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
				Kind:       api.RestAPIKindRestApi,
				Metadata: api.Metadata{
					Name:   "test-api-v1.0",
					Labels: &tt.labels,
				},
				Spec: api.APIConfigData{
					DisplayName: "TestAPI",
					Version:     "v1.0",
					Context:     "/test",
					Upstream: struct {
						Main    api.Upstream  `json:"main" yaml:"main"`
						Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
					}{
						Main: api.Upstream{
							Url: func() *string { s := "http://example.com"; return &s }(),
						},
					},
					Operations: []api.Operation{
						{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/test")},
					},
				},
			}

			errors := validator.Validate(config)

			// Check if we got label validation errors when we expected them
			hasLabelError := false
			for _, err := range errors {
				if err.Field == "metadata.labels" {
					hasLabelError = true
					if tt.shouldError && tt.errorMsg != "" {
						if !strings.Contains(err.Message, tt.errorMsg) {
							t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Message)
						}
					}
					break
				}
			}

			if tt.shouldError && !hasLabelError {
				t.Errorf("Expected validation error for labels %v, but got none", tt.labels)
			}

			if !tt.shouldError && hasLabelError {
				t.Errorf("Did not expect validation error for labels %v, but got one", tt.labels)
			}
		})
	}
}

func TestValidator_LabelsWithAllAPITypes(t *testing.T) {
	validator := NewAPIValidator()

	validLabels := map[string]string{
		"environment": "production",
		"team":        "backend",
		"version":     "v1",
	}

	// Test RestApi
	t.Run("RestApi with valid labels", func(t *testing.T) {
		config := &api.RestAPI{
			ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.RestAPIKindRestApi,
			Metadata: api.Metadata{
				Name:   "test-api-v1.0",
				Labels: &validLabels,
			},
			Spec: api.APIConfigData{
				DisplayName: "TestAPI",
				Version:     "v1.0",
				Context:     "/test",
				Upstream: struct {
					Main    api.Upstream  `json:"main" yaml:"main"`
					Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
				}{
					Main: api.Upstream{
						Url: func() *string { s := "http://example.com"; return &s }(),
					},
				},
				Operations: []api.Operation{
					{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/test")},
				},
			},
		}

		errors := validator.Validate(config)
		hasLabelError := false
		for _, err := range errors {
			if err.Field == "metadata.labels" {
				hasLabelError = true
				break
			}
		}
		assert.False(t, hasLabelError, "RestApi should accept valid labels")
	})

	// Test with invalid labels for both types
	invalidLabels := map[string]string{"Invalid Key": "value"}

	t.Run("RestApi with invalid labels", func(t *testing.T) {
		config := &api.RestAPI{
			ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.RestAPIKindRestApi,
			Metadata: api.Metadata{
				Name:   "test-api-v1.0",
				Labels: &invalidLabels,
			},
			Spec: api.APIConfigData{
				DisplayName: "TestAPI",
				Version:     "v1.0",
				Context:     "/test",
				Upstream: struct {
					Main    api.Upstream  `json:"main" yaml:"main"`
					Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
				}{
					Main: api.Upstream{
						Url: func() *string { s := "http://example.com"; return &s }(),
					},
				},
				Operations: []api.Operation{
					{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/test")},
				},
			},
		}

		errors := validator.Validate(config)
		hasLabelError := false
		for _, err := range errors {
			if err.Field == "metadata.labels" {
				hasLabelError = true
				break
			}
		}
		assert.True(t, hasLabelError, "RestApi should reject labels with spaces in keys")
	})

}

func TestValidateUpstreamDefinitions_Valid(t *testing.T) {
	validator := NewAPIValidator()

	timeout := "30s"
	weight := 80
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream-1",
			Timeout: &api.UpstreamTimeout{
				Connect: &timeout,
			},
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url:    "http://backend-1:8080",
					Weight: &weight,
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	assert.Empty(t, errors)
}

// A host-only URL with the base path supplied via basePath is the canonical form.
func TestValidateUpstreamDefinitions_WithBasePathValid(t *testing.T) {
	validator := NewAPIValidator()

	basePath := "/api/v2"
	definitions := &[]api.UpstreamDefinition{
		{
			Name:     "my-upstream-1",
			BasePath: &basePath,
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{Url: "http://backend-1:8080"},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	assert.Empty(t, errors)
}

// upstreamDefinitions URLs must be host[:port] only; a path belongs in basePath.
func TestValidateUpstreamDefinitions_URLWithPathRejected(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream-1",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{Url: "http://backend-1:8080/api/v2"},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams[0].url", errors[0].Field)
	assert.Contains(t, errors[0].Message, "basePath")
}

// upstreamDefinitions URLs must be host[:port] only; a query string is dropped.
func TestValidateUpstreamDefinitions_URLWithQueryRejected(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream-1",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{Url: "http://backend-1:8080?foo=bar"},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams[0].url", errors[0].Field)
	assert.Contains(t, errors[0].Message, "query string")
}

// upstreamDefinitions URLs must be host[:port] only; a bare "?" query marker is dropped.
func TestValidateUpstreamDefinitions_URLWithBareQueryRejected(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream-1",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{Url: "http://backend-1:8080?"},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams[0].url", errors[0].Field)
	assert.Contains(t, errors[0].Message, "query string")
}

// upstreamDefinitions URLs must be host[:port] only; a fragment is dropped.
func TestValidateUpstreamDefinitions_URLWithFragmentRejected(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream-1",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{Url: "http://backend-1:8080#section"},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams[0].url", errors[0].Field)
	assert.Contains(t, errors[0].Message, "fragment")
}

// upstreamDefinitions URLs must be host[:port] only; a URL carrying both a query and a
// fragment is rejected with a separate error for each.
func TestValidateUpstreamDefinitions_URLWithQueryAndFragmentRejected(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream-1",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{Url: "http://backend-1:8080?a=1#top"},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 2)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams[0].url", errors[0].Field)
	assert.Contains(t, errors[0].Message, "query string")
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams[0].url", errors[1].Field)
	assert.Contains(t, errors[1].Message, "fragment")
}

func TestValidateUpstreamDefinitions_DuplicateNames(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend-1:8080",
				},
			},
		},
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend-2:8080",
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[1].name", errors[0].Field)
	assert.Contains(t, errors[0].Message, "Duplicate upstream definition name")
}

func TestValidateUpstreamDefinitions_MissingName(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend:8080",
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].name", errors[0].Field)
	assert.Contains(t, errors[0].Message, "name is required")
}

func TestValidateUpstreamDefinitions_NoUpstreams(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams", errors[0].Field)
	assert.Contains(t, errors[0].Message, "At least one upstream target is required")
}

func TestValidateUpstreamDefinitions_NoURL(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "",
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams[0].url", errors[0].Field)
	assert.Contains(t, errors[0].Message, "URL is required")
}

func TestValidateUpstreamDefinitions_InvalidURL(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "not-a-valid-url",
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.NotEmpty(t, errors)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams[0].url", errors[0].Field)
	assert.Contains(t, errors[0].Message, "must use http or https scheme")
}

func TestValidateUpstreamDefinitions_InvalidURL_MissingHost(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://",
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams[0].url", errors[0].Field)
	assert.Contains(t, errors[0].Message, "URL must include a host")
}

func TestValidateUpstreamDefinitions_InvalidWeight(t *testing.T) {
	validator := NewAPIValidator()

	invalidWeight := 150
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url:    "http://backend:8080",
					Weight: &invalidWeight,
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams[0].weight", errors[0].Field)
	assert.Contains(t, errors[0].Message, "Weight must be between 0 and 100")
}

func TestValidateUpstreamDefinitions_NoTimeout(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend:8080",
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	assert.Empty(t, errors, "No timeout should be valid")
}

func TestValidateUpstreamDefinitions_ZeroConnectTimeout(t *testing.T) {
	validator := NewAPIValidator()

	// Zero disables the timeout; the shared definition validator mirrors the
	// CRD/resilience duration contract, which accepts "0s"/"0ms".
	for _, zeroTimeout := range []string{"0s", "0ms"} {
		connect := zeroTimeout
		definitions := &[]api.UpstreamDefinition{
			{
				Name: "my-upstream",
				Timeout: &api.UpstreamTimeout{
					Connect: &connect,
				},
				Upstreams: []struct {
					Url    string `json:"url" yaml:"url"`
					Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
				}{
					{
						Url: "http://backend:8080",
					},
				},
			},
		}

		errors := validator.validateUpstreamDefinitions(definitions)
		assert.Empty(t, errors, "timeout %q must be accepted (zero disables the timeout)", zeroTimeout)
	}
}

func TestValidateUpstreamDefinitions_MalformedTimeout(t *testing.T) {
	validator := NewAPIValidator()

	connect := "abc"
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Timeout: &api.UpstreamTimeout{
				Connect: &connect,
			},
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend:8080",
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].timeout.connect", errors[0].Field)
	assert.Contains(t, errors[0].Message, "Invalid timeout format")
}

func TestValidateUpstreamDefinitions_TimeoutUnitContract(t *testing.T) {
	validator := NewAPIValidator()

	// time.ParseDuration accepts units outside the ms|s|m|h contract (ns, us), compound
	// durations, and leading signs that the published schema does not allow; these must
	// be rejected as invalid format, not silently accepted.
	for _, badTimeout := range []string{"5ns", "100us", "1h30m", "+5s", "-5s"} {
		connect := badTimeout
		definitions := &[]api.UpstreamDefinition{
			{
				Name: "my-upstream",
				Timeout: &api.UpstreamTimeout{
					Connect: &connect,
				},
				Upstreams: []struct {
					Url    string `json:"url" yaml:"url"`
					Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
				}{
					{Url: "http://backend:8080"},
				},
			},
		}

		errors := validator.validateUpstreamDefinitions(definitions)
		require.Len(t, errors, 1, "timeout %q must be rejected", badTimeout)
		assert.Equal(t, "spec.upstreamDefinitions[0].timeout.connect", errors[0].Field)
		assert.Contains(t, errors[0].Message, "Invalid timeout format")
	}
}

// TestValidateUpstreamDefinitions_NameRules covers the definition-name contract
// (max 100 chars, pattern ^[a-zA-Z0-9\-_]+$) so a valid name stays referenceable
// from a per-op upstream override.
func TestValidateUpstreamDefinitions_NameRules(t *testing.T) {
	validator := NewAPIValidator()

	validUpstreams := []struct {
		Url    string `json:"url" yaml:"url"`
		Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
	}{
		{Url: "http://backend:8080"},
	}

	tests := []struct {
		name    string
		defName string
		wantMsg string // empty means the name is accepted
	}{
		{"over-length is rejected", strings.Repeat("a", 101), "must be 1-100 characters"},
		{"space is rejected", "bad name", "letters, numbers, hyphens, underscores"},
		{"dot is rejected", "has.dot", "letters, numbers, hyphens, underscores"},
		{"colon is rejected", "has:colon", "letters, numbers, hyphens, underscores"},
		{"slash is rejected", "has/slash", "letters, numbers, hyphens, underscores"},
		{"valid name is accepted", "valid-name_123", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definitions := &[]api.UpstreamDefinition{
				{Name: tt.defName, Upstreams: validUpstreams},
			}
			errors := validator.validateUpstreamDefinitions(definitions)
			if tt.wantMsg == "" {
				assert.Empty(t, errors)
				return
			}
			require.Len(t, errors, 1)
			assert.Equal(t, "spec.upstreamDefinitions[0].name", errors[0].Field)
			assert.Contains(t, errors[0].Message, tt.wantMsg)
		})
	}
}

func TestValidateUpstreamRef_ValidRef(t *testing.T) {
	validator := NewAPIValidator()

	ref := "my-upstream"
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend:8080",
				},
			},
		},
	}

	errors := validator.validateUpstreamRef("main", &ref, definitions)
	assert.Empty(t, errors)
}

func TestValidateUpstreamRef_RefNotFound(t *testing.T) {
	validator := NewAPIValidator()

	ref := "non-existent"
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend:8080",
				},
			},
		},
	}

	errors := validator.validateUpstreamRef("main", &ref, definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstream.main.ref", errors[0].Field)
	assert.Contains(t, errors[0].Message, "Referenced upstream definition 'non-existent' not found")
}

func TestValidateUpstreamRef_NoDefinitions(t *testing.T) {
	validator := NewAPIValidator()

	ref := "my-upstream"

	errors := validator.validateUpstreamRef("main", &ref, nil)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstream.main.ref", errors[0].Field)
	assert.Contains(t, errors[0].Message, "no upstreamDefinitions provided")
}

func TestValidateUpstream_WithRefAndDefinitions(t *testing.T) {
	validator := NewAPIValidator()

	ref := "my-upstream"
	upstream := &api.Upstream{
		Ref: &ref,
	}
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Url: "http://backend:8080",
				},
			},
		},
	}

	errors := validator.validateUpstream("main", upstream, definitions)
	assert.Empty(t, errors)
}

// TestValidateOperationUpstream_ValidRef asserts that a well-formed ref passes validation
// when it resolves to a known upstream definition.
func TestValidateOperationUpstream_ValidRef(t *testing.T) {
	validator := NewAPIValidator()
	definitions := &[]api.UpstreamDefinition{
		{Name: "user-svc-cluster", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://user-svc:8080"}}},
	}
	up := &api.OperationUpstream{
		Main: opRef("user-svc-cluster"),
	}

	errors := validator.validateOperationUpstream(0, up, definitions)
	assert.Empty(t, errors)
}

// TestValidateOperationUpstream_EmptyRef asserts that an empty ref is rejected
// with a per-op-scoped error field path.
func TestValidateOperationUpstream_EmptyRef(t *testing.T) {
	validator := NewAPIValidator()
	up := &api.OperationUpstream{
		Main: opRef(""),
	}

	errors := validator.validateOperationUpstream(2, up, nil)
	require.NotEmpty(t, errors)
	found := false
	for _, e := range errors {
		if strings.Contains(e.Field, "spec.operations[2].upstream.main") {
			found = true
			assert.Contains(t, e.Message, "Upstream ref is required",
				"empty ref should be rejected with the required-ref reason")
			break
		}
	}
	assert.True(t, found, "validation error should be scoped to spec.operations[2].upstream.main, got %+v", errors)
}

// TestValidateOperationUpstream_UnknownRef asserts that a ref not matching any
// upstream definition is rejected.
func TestValidateOperationUpstream_UnknownRef(t *testing.T) {
	validator := NewAPIValidator()
	up := &api.OperationUpstream{
		Main: opRef("missing-cluster"),
	}
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "user-svc-cluster",
			Upstreams: []struct {
				Url    string `json:"url" yaml:"url"`
				Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{Url: "http://user-svc:8080"},
			},
		},
	}

	errors := validator.validateOperationUpstream(0, up, definitions)
	require.NotEmpty(t, errors)
	found := false
	for _, e := range errors {
		if strings.Contains(e.Field, "spec.operations[0].upstream.main") {
			found = true
			assert.Contains(t, e.Message, "not found in upstreamDefinitions",
				"unknown ref should be rejected with the not-found reason")
			break
		}
	}
	assert.True(t, found, "expected unknown-ref error scoped to main, got %+v", errors)
}

// TestValidateOperationUpstream_EmptyWrapper asserts that a wrapper with neither
// main nor sandbox set is rejected.
func TestValidateOperationUpstream_EmptyWrapper(t *testing.T) {
	validator := NewAPIValidator()
	up := &api.OperationUpstream{}

	errors := validator.validateOperationUpstream(3, up, nil)
	require.NotEmpty(t, errors)
	found := false
	for _, e := range errors {
		if e.Field == "spec.operations[3].upstream" &&
			strings.Contains(strings.ToLower(e.Message), "at least one") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'at least one' error at wrapper level, got %+v", errors)
}

// TestValidateOperationUpstream_SandboxUnknownRef asserts the sandbox sub-field is
// validated too (the existence check runs for sandbox), with a sandbox-scoped field path.
func TestValidateOperationUpstream_SandboxUnknownRef(t *testing.T) {
	validator := NewAPIValidator()
	definitions := &[]api.UpstreamDefinition{
		{Name: "user-svc-cluster", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://user-svc:8080"}}},
	}
	up := &api.OperationUpstream{
		Sandbox: opRef("missing-cluster"),
	}

	errors := validator.validateOperationUpstream(0, up, definitions)
	require.NotEmpty(t, errors)
	found := false
	for _, e := range errors {
		if strings.Contains(e.Field, "spec.operations[0].upstream.sandbox") {
			found = true
			assert.Contains(t, e.Message, "not found in upstreamDefinitions",
				"unknown sandbox ref should be rejected with the not-found reason")
			break
		}
	}
	assert.True(t, found, "expected unknown-ref error scoped to sandbox, got %+v", errors)
}

// TestValidateOperationUpstream_RefPatternRejected asserts that a ref containing
// characters outside ^[a-zA-Z0-9\-_]+$ is rejected before the existence check.
func TestValidateOperationUpstream_RefPatternRejected(t *testing.T) {
	validator := NewAPIValidator()
	definitions := &[]api.UpstreamDefinition{
		{Name: "user-svc-cluster", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://user-svc:8080"}}},
	}

	for _, badRef := range []string{"bad/ref", "bad ref", "bad.ref!", "../etc"} {
		up := &api.OperationUpstream{
			Main: opRef(badRef),
		}
		errors := validator.validateOperationUpstream(0, up, definitions)
		require.NotEmpty(t, errors, "ref %q must be rejected", badRef)
		found := false
		for _, e := range errors {
			if strings.Contains(e.Field, "spec.operations[0].upstream.main") &&
				strings.Contains(e.Message, "must match pattern") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected pattern-rejection error for ref %q, got %+v", badRef, errors)
	}
}

// TestValidateOperationUpstream_RefMaxLength asserts that a ref longer than 100
// characters is rejected, matching the OpenAPI schema maxLength constraint.
func TestValidateOperationUpstream_RefMaxLength(t *testing.T) {
	validator := NewAPIValidator()
	longRef := strings.Repeat("a", 101)
	exactRef := strings.Repeat("b", 100)
	definitions := &[]api.UpstreamDefinition{
		{Name: "user-svc-cluster", Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://user-svc:8080"}}},
		{Name: longRef, Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://long-svc:8080"}}},
		{Name: exactRef, Upstreams: []struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{{Url: "http://exact-svc:8080"}}},
	}

	up := &api.OperationUpstream{
		Main: opRef(longRef),
	}
	errors := validator.validateOperationUpstream(0, up, definitions)
	require.NotEmpty(t, errors, "ref longer than 100 chars must be rejected")
	found := false
	for _, e := range errors {
		if strings.Contains(e.Field, "spec.operations[0].upstream.main") &&
			strings.Contains(e.Message, "must not exceed 100 characters") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected maxLength-rejection error for ref of len %d, got %+v", len(longRef), errors)

	// Boundary: exactly 100 characters should pass
	up = &api.OperationUpstream{
		Main: opRef(exactRef),
	}
	errors = validator.validateOperationUpstream(0, up, definitions)
	assert.Empty(t, errors, "ref of exactly 100 chars must pass")
}

// TestValidate_PerOpRef_FullFlow exercises the complete entry path
// Validate -> validateRestData -> validateOperations -> validateOperationUpstream,
// confirming a per-op ref error surfaces from the public Validate API with the
// operation-scoped field path (not just the helper in isolation).
func TestValidate_PerOpRef_FullFlow(t *testing.T) {
	validator := NewAPIValidator()
	config := &api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.RestAPIKindRestApi,
		Metadata:   api.Metadata{Name: "per-op-ref-api-v1.0"},
		Spec: api.APIConfigData{
			DisplayName: "PerOpRefAPI",
			Version:     "v1.0",
			Context:     "/per-op",
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{Url: func() *string { s := "http://example.com"; return &s }()},
			},
			Operations: []api.Operation{
				{
					Method: api.Ptr(api.OperationMethod("GET")),
					Path:   api.Ptr("/users"),
					Upstream: &api.OperationUpstream{
						Main: opRef("missing-cluster"),
					},
				},
			},
		},
	}

	errors := validator.Validate(config)
	require.NotEmpty(t, errors)
	found := false
	for _, e := range errors {
		if strings.Contains(e.Field, "spec.operations[0].upstream.main") &&
			strings.Contains(e.Message, "not found") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected per-op ref error via full Validate, got %+v", errors)
}

// TestValidate_APILevelRefPatternAndLength asserts the API-level upstream ref
// shares the name-pattern and length contract enforced for per-op refs.
func TestValidate_APILevelRefPatternAndLength(t *testing.T) {
	validator := NewAPIValidator()

	base := func(ref string) *api.RestAPI {
		r := ref
		return &api.RestAPI{
			ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.RestAPIKindRestApi,
			Metadata:   api.Metadata{Name: "api-ref-v1.0"},
			Spec: api.APIConfigData{
				DisplayName: "APIRef",
				Version:     "v1.0",
				Context:     "/api-ref",
				Upstream: struct {
					Main    api.Upstream  `json:"main" yaml:"main"`
					Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
				}{Main: api.Upstream{Ref: &r}},
				Operations: []api.Operation{{Method: api.Ptr(api.OperationMethod("GET")), Path: api.Ptr("/x")}},
			},
		}
	}

	hasRefErr := func(errs []ValidationError, msgSub string) bool {
		for _, e := range errs {
			if e.Field == "spec.upstream.main.ref" && strings.Contains(e.Message, msgSub) {
				return true
			}
		}
		return false
	}

	t.Run("bad pattern is rejected with a pattern error", func(t *testing.T) {
		errs := validator.Validate(base("bad/ref"))
		assert.True(t, hasRefErr(errs, "must match pattern"), "API-level ref with bad characters should give a pattern error, got %+v", errs)
	})

	t.Run("over-length ref is rejected with a length error", func(t *testing.T) {
		errs := validator.Validate(base(strings.Repeat("a", 101)))
		assert.True(t, hasRefErr(errs, "must not exceed 100 characters"), "API-level ref over 100 chars should give a length error, got %+v", errs)
	})
}

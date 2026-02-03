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
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

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
			specUnion := api.APIConfiguration_Spec{}
			specUnion.FromAPIConfigData(api.APIConfigData{
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
					{Method: "GET", Path: "/test"},
				},
			})
			config := &api.APIConfiguration{
				ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
				Kind:       api.RestApi,
				Spec:       specUnion,
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
		GatewayController: GatewayController{
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
		GatewayController: GatewayController{
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
		GatewayController: GatewayController{
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
		GatewayController: GatewayController{
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

func TestValidateEventGWConfig_Enabled(t *testing.T) {
	// Test that validation passes when event gateway is enabled with valid config
	config := &Config{
		GatewayController: GatewayController{
			Router: RouterConfig{
				EventGateway: EventGatewayConfig{
					Enabled:               true,
					WebSubHubURL:          "http://example.com",
					WebSubHubPort:         9098,
					WebSubHubListenerPort: 8083,
					TimeoutSeconds:        10,
				},
			},
		},
	}

	err := config.validateEventGatewayConfig()
	assert.NoError(t, err)
}

func TestValidateWebSubURLConfig_WithoutSchema(t *testing.T) {
	// Test that validation fails when there's no scheme in WebSubHubURL
	config := &Config{
		GatewayController: GatewayController{
			Router: RouterConfig{
				EventGateway: EventGatewayConfig{
					Enabled:               true,
					WebSubHubURL:          "example.com",
					WebSubHubPort:         9098,
					WebSubHubListenerPort: 8083,
					TimeoutSeconds:        10,
				},
			},
		},
	}

	err := config.validateEventGatewayConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "http or https scheme")
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
			specUnion := api.APIConfiguration_Spec{}
			specUnion.FromAPIConfigData(api.APIConfigData{
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
					{Method: "GET", Path: "/test"},
				},
			})
			config := &api.APIConfiguration{
				ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
				Kind:       api.RestApi,
				Metadata: api.Metadata{
					Name:   "test-api-v1.0",
					Labels: &tt.labels,
				},
				Spec: specUnion,
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
		specUnion := api.APIConfiguration_Spec{}
		specUnion.FromAPIConfigData(api.APIConfigData{
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
				{Method: "GET", Path: "/test"},
			},
		})
		config := &api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name:   "test-api-v1.0",
				Labels: &validLabels,
			},
			Spec: specUnion,
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

	// Test WebSubApi
	t.Run("WebSubApi with valid labels", func(t *testing.T) {
		specUnion := api.APIConfiguration_Spec{}
		specUnion.FromWebhookAPIData(api.WebhookAPIData{
			DisplayName: "TestAPI",
			Version:     "v1.0",
			Context:     "/test",
			Channels: []api.Channel{
				{
					Name:   "/events",
					Method: api.SUB,
				},
			},
		})
		config := &api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.WebSubApi,
			Metadata: api.Metadata{
				Name:   "test-api-v1.0",
				Labels: &validLabels,
			},
			Spec: specUnion,
		}

		errors := validator.Validate(config)
		hasLabelError := false
		for _, err := range errors {
			if err.Field == "metadata.labels" {
				hasLabelError = true
				break
			}
		}
		assert.False(t, hasLabelError, "WebSubApi should accept valid labels")
	})

	// Test with invalid labels for both types
	invalidLabels := map[string]string{"Invalid Key": "value"}

	t.Run("RestApi with invalid labels", func(t *testing.T) {
		specUnion := api.APIConfiguration_Spec{}
		specUnion.FromAPIConfigData(api.APIConfigData{
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
				{Method: "GET", Path: "/test"},
			},
		})
		config := &api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name:   "test-api-v1.0",
				Labels: &invalidLabels,
			},
			Spec: specUnion,
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

	t.Run("WebSubApi with invalid labels", func(t *testing.T) {
		specUnion := api.APIConfiguration_Spec{}
		specUnion.FromWebhookAPIData(api.WebhookAPIData{
			DisplayName: "TestAPI",
			Version:     "v1.0",
			Context:     "/test",
			Channels: []api.Channel{
				{
					Name:   "/events",
					Method: api.SUB,
				},
			},
		})
		config := &api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.WebSubApi,
			Metadata: api.Metadata{
				Name:   "test-api-v1.0",
				Labels: &invalidLabels,
			},
			Spec: specUnion,
		}

		errors := validator.Validate(config)
		hasLabelError := false
		for _, err := range errors {
			if err.Field == "metadata.labels" {
				hasLabelError = true
				break
			}
		}
		assert.True(t, hasLabelError, "WebSubApi should reject labels with spaces in keys")
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
				Request: &timeout,
			},
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls:   []string{"http://backend-1:8080"},
					Weight: &weight,
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	assert.Empty(t, errors)
}

func TestValidateUpstreamDefinitions_DuplicateNames(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend-1:8080"},
				},
			},
		},
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend-2:8080"},
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
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend:8080"},
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
			Name:      "my-upstream",
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams", errors[0].Field)
	assert.Contains(t, errors[0].Message, "At least one upstream target is required")
}

func TestValidateUpstreamDefinitions_NoURLs(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{},
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams[0].urls", errors[0].Field)
	assert.Contains(t, errors[0].Message, "At least one URL is required")
}

func TestValidateUpstreamDefinitions_InvalidURL(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"not-a-valid-url"},
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.NotEmpty(t, errors)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams[0].urls[0]", errors[0].Field)
	assert.Contains(t, errors[0].Message, "must use http or https scheme")
}

func TestValidateUpstreamDefinitions_InvalidURL_MissingHost(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://"},
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].upstreams[0].urls[0]", errors[0].Field)
	assert.Contains(t, errors[0].Message, "URL must include a host")
}

func TestValidateUpstreamDefinitions_InvalidWeight(t *testing.T) {
	validator := NewAPIValidator()

	invalidWeight := 150
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls:   []string{"http://backend:8080"},
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

func TestValidateUpstreamDefinitions_InvalidTimeout(t *testing.T) {
	validator := NewAPIValidator()

	invalidTimeout := "invalid"
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Timeout: &api.UpstreamTimeout{
				Request: &invalidTimeout,
			},
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend:8080"},
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	require.Len(t, errors, 1)
	assert.Equal(t, "spec.upstreamDefinitions[0].timeout.request", errors[0].Field)
	assert.Contains(t, errors[0].Message, "Invalid timeout format")
}

func TestValidateUpstreamDefinitions_NoTimeout(t *testing.T) {
	validator := NewAPIValidator()

	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend:8080"},
				},
			},
		},
	}

	errors := validator.validateUpstreamDefinitions(definitions)
	assert.Empty(t, errors, "No timeout should be valid")
}

func TestValidateUpstreamRef_ValidRef(t *testing.T) {
	validator := NewAPIValidator()

	ref := "my-upstream"
	definitions := &[]api.UpstreamDefinition{
		{
			Name: "my-upstream",
			Upstreams: []struct {
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend:8080"},
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
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend:8080"},
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
				Urls   []string `json:"urls" yaml:"urls"`
				Weight *int     `json:"weight,omitempty" yaml:"weight,omitempty"`
			}{
				{
					Urls: []string{"http://backend:8080"},
				},
			},
		},
	}

	errors := validator.validateUpstream("main", upstream, definitions)
	assert.Empty(t, errors)
}

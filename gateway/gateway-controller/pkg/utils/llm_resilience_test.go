/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package utils

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// These tests pin the LLM resilience route-mapping rule: API-level resilience is attached to every
// traffic-forwarding route and never to the access-control deny routes.

// In allow_all mode, the catch-all (and any policy-derived routes) carry the resilience block while
// the exception/deny routes (which only return a 404) are left untouched.
func TestTransformProvider_AllowAll_ResilienceSkipsDenyRoutes(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "/admin", Methods: []api.RouteExceptionMethods{api.RouteExceptionMethodsGET, api.RouteExceptionMethodsPOST}},
	}
	timeout := "30s"
	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream:    api.LLMProviderConfigData_Upstream{Url: stringPtr("https://api.example.com")},
			AccessControl: api.LLMAccessControl{
				Mode:       api.AllowAll,
				Exceptions: &exceptions,
			},
			Resilience: &api.Resilience{Timeout: &timeout},
		},
	}

	result, err := transformer.Transform(provider, &api.RestAPI{})
	require.NoError(t, err)

	sawCatchAll := false
	sawDeny := false
	for _, op := range result.Spec.Operations {
		if op.Path == "/admin" { // the deny/exception routes
			sawDeny = true
			assert.Nil(t, op.Resilience, "deny route %s %s must not carry resilience", op.Method, op.Path)
			continue
		}
		// catch-all (traffic-forwarding) routes
		sawCatchAll = true
		require.NotNil(t, op.Resilience, "traffic route %s %s should carry resilience", op.Method, op.Path)
		require.NotNil(t, op.Resilience.Timeout)
		assert.Equal(t, "30s", *op.Resilience.Timeout)
	}
	assert.True(t, sawCatchAll, "expected at least one traffic-forwarding route")
	assert.True(t, sawDeny, "expected at least one deny route")
}

// In deny_all mode every created route is an allow-listed forwarding route, so all of them carry the
// resilience block.
func TestTransformProvider_DenyAll_ResilienceOnAllRoutes(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	exceptions := []api.RouteException{
		{Path: "/chat/completions", Methods: []api.RouteExceptionMethods{api.RouteExceptionMethodsPOST}},
		{Path: "/models", Methods: []api.RouteExceptionMethods{api.RouteExceptionMethodsGET}},
	}
	timeout := "45s"
	idle := "0s"
	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test",
			Version:     "v1.0",
			Template:    "openai",
			Upstream:    api.LLMProviderConfigData_Upstream{Url: stringPtr("https://api.example.com")},
			AccessControl: api.LLMAccessControl{
				Mode:       api.DenyAll,
				Exceptions: &exceptions,
			},
			Resilience: &api.Resilience{Timeout: &timeout, IdleTimeout: &idle},
		},
	}

	result, err := transformer.Transform(provider, &api.RestAPI{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Spec.Operations)

	for _, op := range result.Spec.Operations {
		require.NotNil(t, op.Resilience, "route %s %s should carry resilience", op.Method, op.Path)
		require.NotNil(t, op.Resilience.Timeout)
		assert.Equal(t, "45s", *op.Resilience.Timeout)
		require.NotNil(t, op.Resilience.IdleTimeout)
		assert.Equal(t, "0s", *op.Resilience.IdleTimeout)
	}
}

// With no resilience block on the source provider, no operation gets a resilience block (unchanged
// behavior — routes fall back to the gateway's global default timeout).
func TestTransformProvider_NoResilience_NotAttached(t *testing.T) {
	transformer, _ := setupTestTransformer(t)

	provider := &api.LLMProviderConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       "LlmProvider",
		Metadata:   api.Metadata{Name: "openai-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName:   "test",
			Version:       "v1.0",
			Template:      "openai",
			Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://api.example.com")},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
		},
	}

	result, err := transformer.Transform(provider, &api.RestAPI{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Spec.Operations)

	for _, op := range result.Spec.Operations {
		assert.Nil(t, op.Resilience, "route %s %s should not carry resilience", op.Method, op.Path)
	}
}

// A proxy is always allow-all with no access control, so every generated route carries the
// resilience block.
func TestTransformProxy_ResilienceOnAllRoutes(t *testing.T) {
	store := storage.NewConfigStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := newTestSQLiteStorage(t, logger)

	template := &models.StoredLLMProviderTemplate{
		UUID: "0000-db-template-id-0000-000000000001",
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
			Metadata:   api.Metadata{Name: "openai"},
			Spec:       api.LLMProviderTemplateData{DisplayName: "openai"},
		},
	}
	require.NoError(t, db.SaveLLMProviderTemplate(template))

	now := time.Now()
	provider := &models.StoredConfig{
		UUID:        "0000-db-provider-id-0000-000000000000",
		Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:      "db-provider",
		DisplayName: "db-provider",
		Version:     "v1.0",
		Configuration: api.RestAPI{
			ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.RestAPIKindRestApi,
			Metadata:   api.Metadata{Name: "db-provider"},
			Spec: api.APIConfigData{
				DisplayName: "db-provider",
				Version:     "v1.0",
				Context:     "/db-provider",
				Upstream: struct {
					Main    api.Upstream  `json:"main" yaml:"main"`
					Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
				}{Main: api.Upstream{Url: stringPtr("https://api.openai.com")}},
			},
		},
		SourceConfiguration: api.LLMProviderConfiguration{
			ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderConfigurationKindLlmProvider,
			Metadata:   api.Metadata{Name: "db-provider"},
			Spec: api.LLMProviderConfigData{
				DisplayName:   "db-provider",
				Version:       "v1.0",
				Context:       stringPtr("/db-provider"),
				Template:      "openai",
				Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://api.openai.com")},
				AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			},
		},
		DesiredState: models.StateDeployed,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	require.NoError(t, db.SaveConfig(provider))

	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	transformer := NewLLMProviderTransformer(store, db, routerConfig, newTestPolicyVersionResolver())

	timeout := "75s"
	proxy := &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "db-proxy"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "db-proxy",
			Version:     "v1.0",
			Provider:    api.LLMProxyProvider{Id: "db-provider"},
			Resilience:  &api.Resilience{Timeout: &timeout},
		},
	}

	result, err := transformer.Transform(proxy, &api.RestAPI{})
	require.NoError(t, err)
	require.NotEmpty(t, result.Spec.Operations)

	for _, op := range result.Spec.Operations {
		require.NotNil(t, op.Resilience, "proxy route %s %s should carry resilience", op.Method, op.Path)
		require.NotNil(t, op.Resilience.Timeout)
		assert.Equal(t, "75s", *op.Resilience.Timeout)
	}
}

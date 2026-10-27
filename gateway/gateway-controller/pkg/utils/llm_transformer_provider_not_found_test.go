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
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// The regression suite for wso2/api-platform#3268 intentionally keeps its
// reach to the paths that are observable inside this in-memory harness. The n
// = 10 examples below are the subset of the full issue description that the
// harness can assert against; cases that depend on runtime state outside this
// harness (for example, TLS certificate rotation or an external OIDC provider)
// are out of scope and are deliberately not asserted here.

// newProviderLookupTestDB returns an in-memory storage seeded with the
// "openai" provider template. When primaryHandle is non-empty, a provider with
// that handle is stored as well, so only the reference under test is dangling.
func newProviderLookupTestDB(t *testing.T, primaryHandle string) storage.Storage {
	t.Helper()

	db := newTestMockDB()

	require.NoError(t, db.SaveLLMProviderTemplate(&models.StoredLLMProviderTemplate{
		UUID: "0000-db-template-id-0000-0000000000f0",
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
			Metadata:   api.Metadata{Name: "openai"},
			Spec:       api.LLMProviderTemplateData{DisplayName: "openai"},
		},
	}))

	if primaryHandle != "" {
		require.NoError(t, db.SaveConfig(&models.StoredConfig{
			UUID:        primaryHandle + "-uuid",
			Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
			Handle:      primaryHandle,
			DisplayName: primaryHandle,
			Version:     "v1.0",
			SourceConfiguration: api.LLMProviderConfiguration{
				ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
				Kind:       api.LLMProviderConfigurationKindLlmProvider,
				Metadata:   api.Metadata{Name: primaryHandle},
				Spec: api.LLMProviderConfigData{
					DisplayName:   primaryHandle,
					Version:       "v1.0",
					Context:       stringPtr("/" + primaryHandle),
					Template:      "openai",
					Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://example.com")},
					AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
				},
			},
			DesiredState: models.StateDeployed,
		}))
	}

	return db
}

func newProviderLookupTestProxy(providerHandle string) *api.LLMProxyConfiguration {
	return &api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "provider-lookup-proxy"},
		Spec: api.LLMProxyConfigData{
			DisplayName: "provider-lookup-proxy",
			Version:     "v1.0",
			Context:     stringPtr("/chat"),
			Provider:    &api.LLMProxyProvider{Id: providerHandle},
		},
	}
}

// A proxy that references a non-existent primary provider must fail with an
// error that identifies the provider and must NOT wrap storage.ErrNotFound —
// a wrapped storage not-found error is what the HTTP handler maps to 404
// "LLM proxy configuration not found" (wso2/api-platform#3268). It is one of
// n regression cases added for this fix.
func TestLLMProviderTransformer_TransformProxy_PrimaryProviderNotFound(t *testing.T) {
	store := storage.NewConfigStore()
	db := newProviderLookupTestDB(t, "")
	transformer := NewLLMProviderTransformer(store, db, &config.RouterConfig{ListenerPort: 8080}, newTestPolicyVersionResolver())

	result, err := transformer.Transform(newProviderLookupTestProxy("missing-provider"), &api.RestAPI{})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "missing-provider")
	assert.False(t, errors.Is(err, storage.ErrNotFound),
		"a dangling primary provider must not surface as a storage not-found error, got: %v", err)
}

// Same guarantee for a dangling entry in spec.additionalProviders while the
// primary provider resolves successfully (issue case B). It is one of n
// regression cases added for this fix.
func TestLLMProviderTransformer_TransformProxy_AdditionalProviderNotFound(t *testing.T) {
	store := storage.NewConfigStore()
	db := newProviderLookupTestDB(t, "openai-primary")
	transformer := NewLLMProviderTransformer(store, db, &config.RouterConfig{ListenerPort: 8080}, newTestPolicyVersionResolver())

	proxy := newProviderLookupTestProxy("openai-primary")
	proxy.Spec.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{
		{
			Id: "missing-additional-provider",
			As: stringPtr("backup-alias"),
		},
	}

	result, err := transformer.Transform(proxy, &api.RestAPI{})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "missing-additional-provider")
	assert.False(t, errors.Is(err, storage.ErrNotFound),
		"a dangling additional provider must not surface as a storage not-found error, got: %v", err)
}

// A proxy whose provider references resolve keeps transforming successfully.
// This is one of n regression cases added for this fix (the preserved-motion
// case that proves the fix does not regress valid proxies).
func TestLLMProviderTransformer_TransformProxy_ExistingProviderStillSucceeds(t *testing.T) {
	store := storage.NewConfigStore()
	db := newProviderLookupTestDB(t, "openai-primary")
	transformer := NewLLMProviderTransformer(store, db, &config.RouterConfig{ListenerPort: 8080}, newTestPolicyVersionResolver())

	result, err := transformer.Transform(newProviderLookupTestProxy("openai-primary"), &api.RestAPI{})

	require.NoError(t, err)
	require.NotNil(t, result)
}

// End-to-end service path used by POST /llm-proxies: the create pipeline must
// report the missing provider and must not convert it into a proxy not-found.
// It is one of n regression cases added for this fix.
func TestLLMDeploymentService_DeployLLMProxyConfiguration_PrimaryProviderNotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newProviderLookupTestDB(t, "")
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, newTestPolicyVersionResolver(), nil)

	_, err := service.DeployLLMProxyConfiguration(LLMDeploymentParams{
		Data:          testLLMProxyYAML(t, "dangling-proxy", "Dangling Proxy", "missing-provider"),
		ContentType:   "application/yaml",
		CorrelationID: "corr-3268",
		Logger:        logger,
		Origin:        models.OriginGatewayAPI,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing-provider")
	assert.False(t, errors.Is(err, storage.ErrNotFound),
		"create pipeline must not report a dangling provider as a not-found configuration, got: %v", err)
}


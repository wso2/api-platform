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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"gopkg.in/yaml.v3"
)

// ratelimitPolicyValidator returns a PolicyValidator loaded with a single basic-ratelimit
// definition, so that references to a non-existent major version (e.g. v999) do not resolve.
func ratelimitPolicyValidator() *config.PolicyValidator {
	return config.NewPolicyValidator(map[string]models.PolicyDefinition{
		"basic-ratelimit|v1.0.0": {Name: "basic-ratelimit", Version: "v1.0.0"},
	})
}

// TestLLMDeploymentService_DeployLLMProviderConfiguration_RejectsUnresolvablePolicy reproduces
// issue #2466 for LLM providers: a global policy referencing a non-existent major version must
// fail the deploy rather than being silently dropped.
func TestLLMDeploymentService_DeployLLMProviderConfiguration_RejectsUnresolvablePolicy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	require.NoError(t, db.SaveLLMProviderTemplate(testStoredLLMTemplate("template-openai-id", "openai", "OpenAI Template")))

	apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, ratelimitPolicyValidator())

	providerCfg := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata:   api.Metadata{Name: "policy-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName:    "Policy Provider",
			Version:        "1.0.0",
			Template:       "openai",
			Upstream:       api.LLMProviderConfigData_Upstream{Url: stringPtr("https://example.com")},
			AccessControl:  api.LLMAccessControl{Mode: api.AllowAll},
			GlobalPolicies: &[]api.Policy{{Name: "basic-ratelimit", Version: "v999"}},
		},
	}
	data, err := yaml.Marshal(providerCfg)
	require.NoError(t, err)

	_, err = service.DeployLLMProviderConfiguration(LLMDeploymentParams{
		Data:          data,
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
		Origin:        models.OriginGatewayAPI,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy validation failed")
	assert.Contains(t, err.Error(), "v999")
}

// TestLLMDeploymentService_DeployLLMProxyConfiguration_RejectsUnresolvablePolicy reproduces
// issue #2466 for LLM proxies.
func TestLLMDeploymentService_DeployLLMProxyConfiguration_RejectsUnresolvablePolicy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newTestMockDB()
	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	require.NoError(t, db.SaveLLMProviderTemplate(testStoredLLMTemplate("template-openai-id", "openai", "OpenAI Template")))
	require.NoError(t, db.SaveConfig(&models.StoredConfig{
		UUID:        "provider-a-id",
		Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:      "provider-a",
		DisplayName: "Provider A",
		Version:     "1.0.0",
		SourceConfiguration: api.LLMProviderConfiguration{
			ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderConfigurationKindLlmProvider,
			Metadata:   api.Metadata{Name: "provider-a"},
			Spec: api.LLMProviderConfigData{
				DisplayName:   "Provider A",
				Version:       "1.0.0",
				Template:      "openai",
				Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://example.com")},
				AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			},
		},
	}))

	apiDeploymentService := newTestAPIDeploymentService(store, db, nil, nil, nil)
	service := NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, ratelimitPolicyValidator())

	proxyCfg := api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: "policy-proxy"},
		Spec: api.LLMProxyConfigData{
			DisplayName:    "Policy Proxy",
			Version:        "1.0.0",
			Context:        stringPtr("/chat"),
			Provider:       api.LLMProxyProvider{Id: "provider-a"},
			GlobalPolicies: &[]api.Policy{{Name: "basic-ratelimit", Version: "v999"}},
		},
	}
	data, err := yaml.Marshal(proxyCfg)
	require.NoError(t, err)

	_, err = service.DeployLLMProxyConfiguration(LLMDeploymentParams{
		Data:          data,
		ContentType:   "application/yaml",
		CorrelationID: "test-corr",
		Logger:        logger,
		Origin:        models.OriginGatewayAPI,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "v999")
}

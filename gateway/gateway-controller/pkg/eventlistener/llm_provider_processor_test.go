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

package eventlistener

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/resolver"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

func testLLMProviderTemplate(uuid, handle string) *models.StoredLLMProviderTemplate {
	now := time.Now()
	return &models.StoredLLMProviderTemplate{
		UUID: uuid,
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LlmProviderTemplate,
			Metadata: api.Metadata{
				Name: handle,
			},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "Test Template",
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func testLLMProviderStoredConfig(uuid, handle, template string, policies *[]api.LLMPolicy) *models.StoredConfig {
	now := time.Now()
	provider := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LlmProvider,
		Metadata: api.Metadata{
			Name: handle,
		},
		Spec: api.LLMProviderConfigData{
			DisplayName: "Test Provider",
			Version:     "v1.0.0",
			Context:     stringPtr("/llm"),
			Template:    template,
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("https://example.com"),
			},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			Policies:      policies,
		},
	}

	return &models.StoredConfig{
		UUID:                uuid,
		Kind:                string(api.LlmProvider),
		Handle:              handle,
		DisplayName:         "Test Provider",
		Version:             "v1.0.0",
		SourceConfiguration: provider,
		DesiredState:        models.StateDeployed,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

func testLLMProxyStoredConfig(uuid, handle, provider string, policies *[]api.LLMPolicy) *models.StoredConfig {
	now := time.Now()
	proxy := api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LlmProxy,
		Metadata: api.Metadata{
			Name: handle,
		},
		Spec: api.LLMProxyConfigData{
			DisplayName: "Test Proxy",
			Version:     "v1.0.0",
			Context:     stringPtr("/llm-proxy"),
			Provider: api.LLMProxyProvider{
				Id: provider,
			},
			Policies: policies,
		},
	}

	return &models.StoredConfig{
		UUID:                uuid,
		Kind:                string(api.LlmProxy),
		Handle:              handle,
		DisplayName:         "Test Proxy",
		Version:             "v1.0.0",
		SourceConfiguration: proxy,
		DesiredState:        models.StateDeployed,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

func TestHandleEvent_LLMProviderCreate_RehydratesConfigAndPolicyFromDB(t *testing.T) {
	store := storage.NewConfigStore()
	template := testLLMProviderTemplate("tmpl-1", "openai")
	require.NoError(t, store.AddTemplate(template))

	db := setupSQLiteDBForEventListenerTests(t)
	require.NoError(t, db.SaveLLMProviderTemplate(template))
	policies := &[]api.LLMPolicy{
		{
			Name:    "rate-limit",
			Version: "v1",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "chat/completions",
					Methods: []api.LLMPolicyPathMethods{api.LLMPolicyPathMethodsPOST},
				},
			},
		},
	}
	cfg := testLLMProviderStoredConfig("llm-provider-create-id", "provider-a", "openai", policies)
	require.NoError(t, db.SaveConfig(cfg))

	lazyStore := storage.NewLazyResourceStore(newTestLogger())
	lazySnapshot := lazyresourcexds.NewLazyResourceSnapshotManager(lazyStore, newTestLogger())
	lazyManager := lazyresourcexds.NewLazyResourceStateManager(lazyStore, lazySnapshot, newTestLogger())

	policyStore := storage.NewPolicyStore()
	policyManager := policyxds.NewPolicyManager(policyStore, policyxds.NewSnapshotManager(policyStore, newTestLogger()), newTestLogger())

	policyDefs := map[string]models.PolicyDefinition{
		"rate-limit-v1.0.0": {Name: "rate-limit", Version: "v1.0.0"},
	}
	listener := &EventListener{
		store:               store,
		db:                  db,
		lazyResourceManager: lazyManager,
		policyManager:       policyManager,
		routerConfig: &config.RouterConfig{
			GatewayHost: "gateway.example.com",
			VHosts: config.VHostsConfig{
				Main: config.VHostEntry{Default: "api.example.com"},
			},
		},
		systemConfig:      &config.Config{},
		policyDefinitions: policyDefs,
		policyResolver:    resolver.NewPolicyResolver(policyDefs, nil),
		logger:            newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeLLMProvider,
		Action:    "CREATE",
		EntityID:  cfg.UUID,
		EventID:   "corr-llm-provider-create",
	})

	stored, err := store.Get(cfg.UUID)
	require.NoError(t, err)
	_, ok := stored.Configuration.(api.RestAPI)
	assert.True(t, ok)

	mapping, exists := lazyManager.GetResourceByIDAndType(cfg.Handle, utils.LazyResourceTypeProviderTemplateMapping)
	require.True(t, exists)
	assert.Equal(t, "openai", mapping.Resource["template_handle"])
	assert.Equal(t, cfg.Handle, mapping.Resource["provider_name"])

	policy, err := policyManager.GetPolicy(cfg.UUID + "-policies")
	require.NoError(t, err)
	var policyRoutes int
	for _, route := range policy.Configuration.Routes {
		if len(route.Policies) > 0 {
			policyRoutes++
			assert.Equal(t, "rate-limit", route.Policies[0].Name)
		}
	}
	assert.Equal(t, 1, policyRoutes)
}

func TestHandleEvent_LLMProviderDelete_RemovesLocalStateAndAPIKeys(t *testing.T) {
	store := storage.NewConfigStore()
	xdsManager := &mockAPIKeyXDSManager{}
	require.NoError(t, store.AddTemplate(testLLMProviderTemplate("tmpl-1", "openai")))

	cfg := testLLMProviderStoredConfig("llm-provider-delete-id", "provider-a", "openai", nil)
	apiKey := testAPIKey("provider-key-id-1", "provider-key", "Provider Key", cfg.UUID)
	require.NoError(t, store.Add(cfg))
	require.NoError(t, store.StoreAPIKey(apiKey))

	lazyStore := storage.NewLazyResourceStore(newTestLogger())
	lazySnapshot := lazyresourcexds.NewLazyResourceSnapshotManager(lazyStore, newTestLogger())
	lazyManager := lazyresourcexds.NewLazyResourceStateManager(lazyStore, lazySnapshot, newTestLogger())
	require.NoError(t, lazyManager.StoreResource(&storage.LazyResource{
		ID:           cfg.Handle,
		ResourceType: utils.LazyResourceTypeProviderTemplateMapping,
		Resource: map[string]interface{}{
			"provider_name":   cfg.Handle,
			"template_handle": "openai",
		},
	}, ""))

	policyStore := storage.NewPolicyStore()
	policyManager := policyxds.NewPolicyManager(policyStore, policyxds.NewSnapshotManager(policyStore, newTestLogger()), newTestLogger())
	require.NoError(t, policyStore.Set(&models.StoredPolicyConfig{
		ID: cfg.UUID + "-policies",
		Configuration: policyenginev1.Configuration{
			Metadata: policyenginev1.Metadata{
				APIName: "Test Provider",
				Version: "v1.0.0",
				Context: "/llm",
			},
		},
	}))

	listener := &EventListener{
		store:               store,
		apiKeyXDSManager:    xdsManager,
		lazyResourceManager: lazyManager,
		policyManager:       policyManager,
		logger:              newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeLLMProvider,
		Action:    "DELETE",
		EntityID:  cfg.UUID,
		EventID:   "corr-llm-provider-delete",
	})

	_, err := store.Get(cfg.UUID)
	require.ErrorIs(t, err, storage.ErrNotFound)

	_, err = store.GetAPIKeyByName(cfg.UUID, apiKey.Name)
	require.ErrorIs(t, err, storage.ErrNotFound)

	if assert.Len(t, xdsManager.removeCalls, 1) {
		assert.Equal(t, cfg.UUID, xdsManager.removeCalls[0].apiID)
		assert.Equal(t, cfg.DisplayName, xdsManager.removeCalls[0].apiName)
		assert.Equal(t, cfg.Version, xdsManager.removeCalls[0].apiVersion)
		assert.Equal(t, "corr-llm-provider-delete", xdsManager.removeCalls[0].correlationID)
	}

	_, exists := lazyManager.GetResourceByIDAndType(cfg.Handle, utils.LazyResourceTypeProviderTemplateMapping)
	assert.False(t, exists)

	_, err = policyManager.GetPolicy(cfg.UUID + "-policies")
	require.Error(t, err)
}

func TestHandleEvent_LLMProxyCreate_RehydratesConfigAndPolicyFromDB(t *testing.T) {
	store := storage.NewConfigStore()
	template := testLLMProviderTemplate("tmpl-1", "openai")
	require.NoError(t, store.AddTemplate(template))

	providerCfg := testLLMProviderStoredConfig("provider-1", "provider-a", "openai", nil)
	require.NoError(t, store.Add(providerCfg))

	db := setupSQLiteDBForEventListenerTests(t)
	require.NoError(t, db.SaveLLMProviderTemplate(template))
	require.NoError(t, db.SaveConfig(providerCfg))
	policies := &[]api.LLMPolicy{
		{
			Name:    "rate-limit",
			Version: "v1",
			Paths: []api.LLMPolicyPath{
				{
					Path:    "chat/completions",
					Methods: []api.LLMPolicyPathMethods{api.LLMPolicyPathMethodsPOST},
				},
			},
		},
	}
	cfg := testLLMProxyStoredConfig("llm-proxy-create-id", "proxy-a", "provider-a", policies)
	require.NoError(t, db.SaveConfig(cfg))

	policyStore := storage.NewPolicyStore()
	policyManager := policyxds.NewPolicyManager(policyStore, policyxds.NewSnapshotManager(policyStore, newTestLogger()), newTestLogger())

	policyDefs := map[string]models.PolicyDefinition{
		"rate-limit-v1.0.0": {Name: "rate-limit", Version: "v1.0.0"},
	}
	listener := &EventListener{
		store:         store,
		db:            db,
		policyManager: policyManager,
		routerConfig: &config.RouterConfig{
			ListenerPort: 8080,
			GatewayHost:  "gateway.example.com",
			VHosts: config.VHostsConfig{
				Main: config.VHostEntry{Default: "api.example.com"},
			},
		},
		systemConfig:      &config.Config{},
		policyDefinitions: policyDefs,
		policyResolver:    resolver.NewPolicyResolver(policyDefs, nil),
		logger:            newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeLLMProxy,
		Action:    "CREATE",
		EntityID:  cfg.UUID,
		EventID:   "corr-llm-proxy-create",
	})

	stored, err := store.Get(cfg.UUID)
	require.NoError(t, err)
	_, ok := stored.Configuration.(api.RestAPI)
	assert.True(t, ok)

	policy, err := policyManager.GetPolicy(cfg.UUID + "-policies")
	require.NoError(t, err)
	var policyRoutes int
	for _, route := range policy.Configuration.Routes {
		if len(route.Policies) > 0 {
			policyRoutes++
			assert.Equal(t, "rate-limit", route.Policies[0].Name)
		}
	}
	assert.Equal(t, 1, policyRoutes)
}

func TestHandleEvent_LLMProxyDelete_RemovesLocalStateAndAPIKeys(t *testing.T) {
	store := storage.NewConfigStore()
	xdsManager := &mockAPIKeyXDSManager{}
	cfg := testLLMProxyStoredConfig("llm-proxy-delete-id", "proxy-a", "provider-a", nil)
	apiKey := testAPIKey("proxy-key-id-1", "proxy-key", "Proxy Key", cfg.UUID)

	require.NoError(t, store.Add(cfg))
	require.NoError(t, store.StoreAPIKey(apiKey))

	policyStore := storage.NewPolicyStore()
	policyManager := policyxds.NewPolicyManager(policyStore, policyxds.NewSnapshotManager(policyStore, newTestLogger()), newTestLogger())
	require.NoError(t, policyStore.Set(&models.StoredPolicyConfig{
		ID: cfg.UUID + "-policies",
		Configuration: policyenginev1.Configuration{
			Metadata: policyenginev1.Metadata{
				APIName: "Test Proxy",
				Version: "v1.0.0",
				Context: "/llm-proxy",
			},
		},
	}))

	listener := &EventListener{
		store:            store,
		apiKeyXDSManager: xdsManager,
		policyManager:    policyManager,
		logger:           newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeLLMProxy,
		Action:    "DELETE",
		EntityID:  cfg.UUID,
		EventID:   "corr-llm-proxy-delete",
	})

	_, err := store.Get(cfg.UUID)
	require.ErrorIs(t, err, storage.ErrNotFound)

	_, err = store.GetAPIKeyByName(cfg.UUID, apiKey.Name)
	require.ErrorIs(t, err, storage.ErrNotFound)

	if assert.Len(t, xdsManager.removeCalls, 1) {
		assert.Equal(t, cfg.UUID, xdsManager.removeCalls[0].apiID)
		assert.Equal(t, cfg.DisplayName, xdsManager.removeCalls[0].apiName)
		assert.Equal(t, cfg.Version, xdsManager.removeCalls[0].apiVersion)
		assert.Equal(t, "corr-llm-proxy-delete", xdsManager.removeCalls[0].correlationID)
	}

	_, err = policyManager.GetPolicy(cfg.UUID + "-policies")
	require.Error(t, err)
}

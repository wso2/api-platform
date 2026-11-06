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

	"github.com/wso2/api-platform/common/chainkey"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/transform"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
)

// modelReplica is a replica wired for model endpoints: the store already holds two
// endpoints, a provider and the OpenAI template, as it would after their own
// events, and Model events arrive from another replica.
type modelReplica struct {
	listener     *EventListener
	store        *storage.ConfigStore
	runtimeStore *storage.RuntimeConfigStore
}

func newModelReplica(t *testing.T, db storage.Storage) *modelReplica {
	t.Helper()
	logger := newTestLogger()
	routerConfig := agentTestRouterConfig()
	systemConfig := &config.Config{Router: *routerConfig}
	systemConfig.Controller.Models = config.ModelsConfig{Enabled: true,
		Endpoint: config.ModelEndpointConfig{Provision: true, Context: "/wso2/v1", MaxRequestBodyBytes: 1 << 20}}

	defs := map[string]models.PolicyDefinition{}
	for name, version := range map[string]string{
		"api-key-auth": "v1.2.1", "set-headers": "v1.0.0", "wso2_apip_sys_model": "v1.0.0",
	} {
		defs[name+"|"+version] = models.PolicyDefinition{Name: name, Version: version}
	}

	store := storage.NewConfigStore()
	runtimeStore := storage.NewRuntimeConfigStore()
	llm := transform.NewLLMTransformer(store, db, routerConfig, systemConfig, defs, utils.NewLoadedPolicyVersionResolver(defs))
	registry := transform.NewRegistry(transform.NewRestAPITransformer(routerConfig, systemConfig, defs), llm, nil, nil)
	registry.SetModelEndpointTransformer(transform.NewModelEndpointTransformer(transform.NewModelGatewayCompiler(
		store, routerConfig, systemConfig, defs, llm.ProviderTransformer(), stubSecretResolver{value: "secret"})))

	policySnapshotManager := policyxds.NewSnapshotManager(logger)
	policySnapshotManager.SetRuntimeStore(runtimeStore)
	policyManager := policyxds.NewPolicyManager(policySnapshotManager, logger)
	policyManager.SetRuntimeStore(runtimeStore)
	policyManager.SetTransformers(registry)

	snapshotManager, err := xds.NewSnapshotManager(store, logger, routerConfig, db, systemConfig)
	require.NoError(t, err)

	require.NoError(t, store.AddTemplate(&models.StoredLLMProviderTemplate{
		UUID: "tmpl-openai",
		Configuration: api.LLMProviderTemplate{
			Metadata: api.Metadata{Name: "openai"},
			Spec: api.LLMProviderTemplateData{DisplayName: "OpenAI", GroupId: strPtr("wso2-openai"),
				RequestModel: &api.ExtractionIdentifier{Location: "payload", Identifier: "$.model"}},
		},
	}))

	none := api.LLMProviderConfigDataExposure(models.LLMProviderExposureNone)
	provider := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata:   api.Metadata{Name: "openai"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "OpenAI", Version: "v1.0", Template: "openai", Exposure: &none,
			Upstream: api.LLMProviderConfigData_Upstream{Url: strPtr("https://api.openai.com/v1")},
		},
	}
	require.NoError(t, store.Add(&models.StoredConfig{
		UUID: "provider-openai", Kind: models.KindLlmProvider, Handle: "openai", DisplayName: "OpenAI", Version: "v1.0",
		SourceConfiguration: provider, Configuration: provider, DesiredState: models.StateDeployed,
	}))

	for handle, context := range map[string]string{"wso2": "/wso2/v1", "partners": "/partners/v1"} {
		require.NoError(t, store.Add(testModelEndpointStoredConfig(handle, context)))
	}

	return &modelReplica{
		listener: &EventListener{
			store:             store,
			db:                db,
			snapshotManager:   snapshotManager,
			policyManager:     policyManager,
			routerConfig:      routerConfig,
			systemConfig:      systemConfig,
			policyDefinitions: defs,
			secretResolver:    stubSecretResolver{value: "secret"},
			logger:            logger,
		},
		store:        store,
		runtimeStore: runtimeStore,
	}
}

func strPtr(s string) *string { return &s }

func testModelEndpointStoredConfig(handle, context string) *models.StoredConfig {
	e := api.ModelEndpointConfiguration{
		ApiVersion: api.ModelEndpointConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.ModelEndpointConfigurationKindModelEndpoint,
		Metadata:   api.Metadata{Name: handle},
		Spec: api.ModelEndpointConfigData{
			DisplayName: "Endpoint " + handle, Version: "v1.0", Context: strPtr(context),
			Policies: &[]api.Policy{{Name: "api-key-auth", Version: "v1"}},
		},
	}
	now := time.Now()
	return &models.StoredConfig{
		UUID: models.ModelEndpointUUID(handle), Kind: models.KindModelEndpoint, Handle: handle,
		DisplayName: e.Spec.DisplayName, Version: "v1.0",
		SourceConfiguration: e, Configuration: e, DesiredState: models.StateDeployed,
		Origin: models.OriginGatewayAPI, CreatedAt: now, UpdatedAt: now,
	}
}

func testModelStoredConfig(endpoints ...string) *models.StoredConfig {
	m := api.ModelConfiguration{
		ApiVersion: "gateway.api-platform.wso2.com/v1",
		Kind:       "Model",
		Metadata:   api.Metadata{Name: "gpt"},
		Spec: api.ModelConfigData{
			DisplayName: "GPT", Version: "v1.0", ModelName: "gpt-4o", Endpoints: endpoints,
			Targets: []api.ModelTarget{{Name: "gpt-4o", Provider: "openai", Alias: stringPtr("a")}},
		},
	}
	now := time.Now()
	return &models.StoredConfig{
		UUID: "model-gpt", Kind: models.KindModel, Handle: "gpt", DisplayName: "GPT", Version: "v1.0",
		SourceConfiguration: m, Configuration: m, DesiredState: models.StateDeployed,
		Origin: models.OriginGatewayAPI, CreatedAt: now, UpdatedAt: now,
	}
}

func modelEvent(action, id string) eventhub.Event {
	return eventhub.Event{EventType: eventhub.EventTypeModel, Action: action, EntityID: id, EventID: "corr-" + action,
		EventData: eventhub.EmptyEventData}
}

func endpointEvent(action, id string) eventhub.Event {
	return eventhub.Event{EventType: eventhub.EventTypeModelEndpoint, Action: action, EntityID: id,
		EventID: "corr-ep-" + action, EventData: eventhub.EmptyEventData}
}

func (r *modelReplica) chainsOf(t *testing.T, handle string) map[string]*models.PolicyChain {
	t.Helper()
	rdc, ok := r.runtimeStore.Get(storage.Key(models.KindModelEndpoint, handle))
	if !ok {
		return nil
	}
	return rdc.PolicyChains
}

// A Model saved on one replica reaches the endpoints it names on another: the
// Model enters the store, and each named endpoint is recompiled with its chain.
// Endpoints it does not name are left alone. A delete takes the chain out again.
func TestHandleEvent_ModelConvergesIntoItsEndpoints(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testModelStoredConfig("wso2")
	require.NoError(t, db.SaveConfig(cfg))

	replica := newModelReplica(t, db)
	replica.listener.handleEvent(modelEvent("CREATE", cfg.UUID))

	stored, err := replica.store.Get(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.KindModel, stored.Kind)

	chain := chainkey.For(models.ModelEndpointUUID("wso2"), agentTestVHost, "gpt")
	assert.Contains(t, replica.chainsOf(t, "wso2"), chain, "the named endpoint is recompiled with the Model's chain")
	assert.Nil(t, replica.chainsOf(t, "partners"), "an endpoint the Model does not name is not touched")

	require.NoError(t, db.DeleteConfig(cfg.UUID))
	replica.listener.handleEvent(modelEvent("DELETE", cfg.UUID))
	_, err = replica.store.Get(cfg.UUID)
	assert.True(t, storage.IsNotFoundError(err))
	assert.NotContains(t, replica.chainsOf(t, "wso2"), chain)
}

// Moving a Model between endpoints recompiles both the one it left and the one it
// joined.
func TestHandleEvent_ModelMovingEndpointsRecompilesOldAndNew(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testModelStoredConfig("wso2")
	require.NoError(t, db.SaveConfig(cfg))
	replica := newModelReplica(t, db)
	replica.listener.handleEvent(modelEvent("CREATE", cfg.UUID))
	require.Contains(t, replica.chainsOf(t, "wso2"), chainkey.For(models.ModelEndpointUUID("wso2"), agentTestVHost, "gpt"))

	moved := testModelStoredConfig("partners")
	moved.UpdatedAt = time.Now().Add(time.Second)
	require.NoError(t, db.UpdateConfig(moved))
	replica.listener.handleEvent(modelEvent("UPDATE", cfg.UUID))

	assert.NotContains(t, replica.chainsOf(t, "wso2"), chainkey.For(models.ModelEndpointUUID("wso2"), agentTestVHost, "gpt"))
	assert.Contains(t, replica.chainsOf(t, "partners"), chainkey.For(models.ModelEndpointUUID("partners"), agentTestVHost, "gpt"))
}

// An endpoint event compiles the endpoint, an undeploy takes its runtime config
// away, and a delete removes it and its store entry.
func TestHandleEvent_ModelEndpointLifecycle(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	replica := newModelReplica(t, db)
	endpoint := testModelEndpointStoredConfig("sandbox", "/sandbox/v1")
	require.NoError(t, db.SaveConfig(endpoint))

	replica.listener.handleEvent(endpointEvent("CREATE", endpoint.UUID))
	_, err := replica.store.Get(endpoint.UUID)
	require.NoError(t, err)
	rdc, ok := replica.runtimeStore.Get(storage.Key(models.KindModelEndpoint, "sandbox"))
	require.True(t, ok)
	assert.Contains(t, rdc.Routes, "POST|/sandbox/v1/chat/completions|"+agentTestVHost)

	endpoint.DesiredState = models.StateUndeployed
	endpoint.UpdatedAt = time.Now().Add(time.Second)
	require.NoError(t, db.UpdateConfig(endpoint))
	replica.listener.handleEvent(endpointEvent("UPDATE", endpoint.UUID))
	_, ok = replica.runtimeStore.Get(storage.Key(models.KindModelEndpoint, "sandbox"))
	assert.False(t, ok, "an undeployed endpoint serves nothing")

	require.NoError(t, db.DeleteConfig(endpoint.UUID))
	replica.listener.handleEvent(endpointEvent("DELETE", endpoint.UUID))
	_, err = replica.store.Get(endpoint.UUID)
	assert.True(t, storage.IsNotFoundError(err))
}

// Refreshing an endpoint that does not exist, or is undeployed, compiles nothing.
func TestRefreshModelEndpoints_MissingOrUndeployedIsANoOp(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	replica := newModelReplica(t, db)
	replica.listener.refreshModelEndpoints([]string{"not-here", "", "not-here"}, "corr")
	_, ok := replica.runtimeStore.Get(storage.Key(models.KindModelEndpoint, "not-here"))
	assert.False(t, ok)

	partners, err := replica.store.GetByKindAndHandle(models.KindModelEndpoint, "partners")
	require.NoError(t, err)
	partners.DesiredState = models.StateUndeployed
	replica.listener.refreshModelEndpoints([]string{"partners"}, "corr")
	_, ok = replica.runtimeStore.Get(storage.Key(models.KindModelEndpoint, "partners"))
	assert.False(t, ok)
}

// A provider change recompiles the endpoints serving a Model on that provider.
func TestRefreshEndpointsForProvider(t *testing.T) {
	db := setupSQLiteDBForEventListenerTests(t)
	replica := newModelReplica(t, db)
	require.NoError(t, replica.store.Add(testModelStoredConfig("partners")))

	replica.listener.refreshEndpointsForProvider("openai", "corr")
	assert.Contains(t, replica.chainsOf(t, "partners"), chainkey.For(models.ModelEndpointUUID("partners"), agentTestVHost, "gpt"))
	assert.Nil(t, replica.chainsOf(t, "wso2"))
}

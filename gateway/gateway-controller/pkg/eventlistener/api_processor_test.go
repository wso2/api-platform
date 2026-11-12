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

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
)

func TestHandleEvent_APICreate_AddsConfigFromDB(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	cfg := testRestStoredConfig("api-create-id", "test-api", "Test API", "v1.0.0", models.StateDeployed)
	require.NoError(t, db.SaveConfig(cfg))

	listener := &EventListener{
		store:  store,
		db:     db,
		logger: newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "CREATE",
		EntityID:  cfg.UUID,
		EventID:   "corr-api-create",
	})

	stored, err := store.Get(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StateDeployed, stored.DesiredState)
	assert.Equal(t, cfg.DisplayName, stored.DisplayName)
}

func TestHandleEvent_APIUpdate_RefreshesExistingConfigFromDB(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)

	stale := testRestStoredConfig("api-update-id", "test-api", "Test API", "v1.0.0", models.StateDeployed)
	require.NoError(t, store.Add(stale))

	latest := testRestStoredConfig("api-update-id", "test-api", "Test API", "v1.0.0", models.StateUndeployed)
	require.NoError(t, db.SaveConfig(latest))

	listener := &EventListener{
		store:  store,
		db:     db,
		logger: newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "UPDATE",
		EntityID:  latest.UUID,
		EventID:   "corr-api-update",
	})

	stored, err := store.Get(latest.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, stored.DesiredState)
}

func TestHandleEvent_APICreate_SyncsExistingAPIKeysToXDS(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	xdsManager := &mockAPIKeyXDSManager{}

	cfg := testRestStoredConfig("api-create-sync-id", "test-api", "Test API", "v1.0.0", models.StateDeployed)
	require.NoError(t, db.SaveConfig(cfg))

	apiKey := testAPIKey("api-key-sync-id", "test-key", cfg.UUID)
	require.NoError(t, db.SaveAPIKey(apiKey))

	listener := &EventListener{
		store:            store,
		db:               db,
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "CREATE",
		EntityID:  cfg.UUID,
		EventID:   "corr-api-create-sync",
	})

	if assert.Len(t, xdsManager.storeCalls, 1) {
		assert.Equal(t, cfg.UUID, xdsManager.storeCalls[0].apiID)
		assert.Equal(t, cfg.DisplayName, xdsManager.storeCalls[0].apiName)
		assert.Equal(t, cfg.Version, xdsManager.storeCalls[0].apiVersion)
		assert.Equal(t, apiKey.UUID, xdsManager.storeCalls[0].apiKeyID)
		assert.Equal(t, "corr-api-create-sync", xdsManager.storeCalls[0].correlationID)
	}
}

func TestHandleEvent_APIDelete_RemovesAPIKeysFromMemoryAndXDS(t *testing.T) {
	store := storage.NewConfigStore()
	xdsManager := &mockAPIKeyXDSManager{}
	cfg := testRestStoredConfig("api-delete-id", "delete-api", "Delete API", "v1.0.0", models.StateDeployed)
	require.NoError(t, store.Add(cfg))

	apiKey := testAPIKey("api-key-id-1", "test-key", cfg.UUID)
	require.NoError(t, store.StoreAPIKey(apiKey))

	listener := &EventListener{
		store:            store,
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "DELETE",
		EntityID:  cfg.UUID,
		EventID:   "corr-api-delete",
	})

	_, err := store.Get(cfg.UUID)
	require.ErrorIs(t, err, storage.ErrNotFound)

	_, err = store.GetAPIKeyByName(cfg.UUID, apiKey.Name)
	require.ErrorIs(t, err, storage.ErrNotFound)

	if assert.Len(t, xdsManager.removeCalls, 1) {
		assert.Equal(t, cfg.UUID, xdsManager.removeCalls[0].apiID)
		assert.Equal(t, cfg.DisplayName, xdsManager.removeCalls[0].apiName)
		assert.Equal(t, cfg.Version, xdsManager.removeCalls[0].apiVersion)
		assert.Equal(t, "corr-api-delete", xdsManager.removeCalls[0].correlationID)
	}
}

func TestUpdatePoliciesForAPI_NilManagerIsNoop(t *testing.T) {
	listener := &EventListener{
		logger:        newTestLogger(),
		policyManager: nil,
	}
	// Should not panic when policyManager is nil
	listener.updatePoliciesForAPI(
		testRestStoredConfig("test-api-id", "test-api", "Test API", "v1.0.0", models.StateDeployed),
		"corr-policy-noop",
	)
}

// A REST API whose stored spec reads a refused variable is undeployed on the
// replica that processes its update event: kept in the store as undeployed, with
// its runtime config (policy chain) removed.
func TestHandleEvent_APIUpdateReadingRefusedEnv_IsUndeployed(t *testing.T) {
	t.Setenv("LISTENER_REFUSED_SECRET", "https://leak.example")
	restricted, err := funcs.NewEnvAccessPolicy([]string{"BACKEND_URL"})
	require.NoError(t, err)
	useTemplateEnvPolicy(t, restricted)

	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	runtimeStore := storage.NewRuntimeConfigStore()
	logger := newTestLogger()
	policyManager := policyxds.NewPolicyManager(policyxds.NewSnapshotManager(logger), logger)
	policyManager.SetRuntimeStore(runtimeStore)

	deployed := testRestStoredConfig("api-env-refused-id", "env-refused-api", "Env Refused API", "v1.0.0", models.StateDeployed)
	require.NoError(t, store.Add(deployed))
	runtimeKey := storage.Key(models.KindRestApi, deployed.Handle)
	runtimeStore.Set(runtimeKey, &models.RuntimeDeployConfig{})

	refused := testRestStoredConfig(deployed.UUID, deployed.Handle, deployed.DisplayName, deployed.Version, models.StateDeployed)
	restAPI := refused.SourceConfiguration.(api.RestAPI)
	restAPI.Spec.Upstream.Main.Url = stringPtr(`{{ env "LISTENER_REFUSED_SECRET" }}`)
	refused.Configuration = restAPI
	refused.SourceConfiguration = restAPI
	require.NoError(t, db.SaveConfig(refused))

	listener := &EventListener{store: store, db: db, policyManager: policyManager, logger: logger}
	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "UPDATE",
		EntityID:  deployed.UUID,
		EventID:   "corr-api-env-refused",
	})

	stored, err := store.Get(deployed.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, stored.DesiredState)
	_, exists := runtimeStore.Get(runtimeKey)
	assert.False(t, exists, "the API's runtime config should be removed")
}

// recordingEventChannelHooks records the configs the event channel snapshot is
// built from, keeping only deployed ones as the event-gateway translator does.
type recordingEventChannelHooks struct {
	webSub []models.StoredConfig
}

func (r *recordingEventChannelHooks) TranslateWebSubApisToEventChannelConfigs(configs []*models.StoredConfig) map[string]types.Resource {
	resources := map[string]types.Resource{}
	for _, cfg := range configs {
		r.webSub = append(r.webSub, *cfg)
		if cfg.DesiredState == models.StateDeployed {
			resources[cfg.UUID] = &structpb.Struct{}
		}
	}
	return resources
}

func (r *recordingEventChannelHooks) TranslateWebBrokerApisToEventChannelConfigs([]*models.StoredConfig) map[string]types.Resource {
	return map[string]types.Resource{}
}

// A WebSubApi whose spec reads a refused variable is undeployed by refreshing the
// event channel snapshot, which then no longer carries its channel; it has no
// router route or runtime config to touch.
func TestUndeployAfterEnvRefusal_WebSubApiRefreshesEventChannel(t *testing.T) {
	logger := newTestLogger()
	store := storage.NewConfigStore()
	runtimeStore := storage.NewRuntimeConfigStore()
	hooks := &recordingEventChannelHooks{}
	policySnapshots := policyxds.NewSnapshotManager(logger)
	policySnapshots.SetRuntimeStore(runtimeStore)
	policySnapshots.SetConfigStore(store)
	policySnapshots.GetTranslator().SetEventChannelHooks(hooks)
	policyManager := policyxds.NewPolicyManager(policySnapshots, logger)
	policyManager.SetRuntimeStore(runtimeStore)

	cfg := &models.StoredConfig{UUID: "websub-env-refused-id", Kind: models.KindWebSubApi, Handle: "websub-env-refused", DesiredState: models.StateDeployed}
	require.NoError(t, store.Add(cfg))
	require.NoError(t, policyManager.UpdateEventChannelSnapshot())
	require.Contains(t, policySnapshots.GetEventChannelCache().(interface {
		GetResources() map[string]types.Resource
	}).GetResources(), cfg.UUID, "precondition: the channel is published while deployed")

	listener := &EventListener{store: store, policyManager: policyManager, logger: logger}
	refused := *cfg
	listener.undeployAfterEnvRefusal(&refused, "corr-websub-env-refused")

	stored, err := store.Get(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, models.StateUndeployed, stored.DesiredState)
	require.NotEmpty(t, hooks.webSub)
	assert.Equal(t, models.StateUndeployed, hooks.webSub[len(hooks.webSub)-1].DesiredState,
		"the event channel refresh must see the artifact as undeployed")
	resources := policySnapshots.GetEventChannelCache().(interface {
		GetResources() map[string]types.Resource
	}).GetResources()
	if res, ok := resources[cfg.UUID]; ok {
		assert.NotEqual(t, &structpb.Struct{}, res, "only a removal marker may remain for the undeployed channel")
	}
}

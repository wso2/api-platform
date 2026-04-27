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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
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

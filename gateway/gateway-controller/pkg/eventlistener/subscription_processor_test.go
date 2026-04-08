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

func TestHandleEvent_ApplicationUpdate_SyncsMemoryAndXDSFromDB(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	xdsManager := &mockAPIKeyXDSManager{}

	cfgA := testRestStoredConfig("test-api-a", "test-api-a", "Test API A", "v1.0.0", models.StateDeployed)
	cfgB := testRestStoredConfig("test-api-b", "test-api-b", "Test API B", "v2.0.0", models.StateDeployed)
	keyA := testAPIKey("api-key-a", "key-a", cfgA.UUID)
	keyB := testAPIKey("api-key-b", "key-b", cfgB.UUID)

	require.NoError(t, store.Add(cfgA))
	require.NoError(t, store.Add(cfgB))

	staleKeyA := *keyA
	staleKeyA.ApplicationID = "app-uuid-1"
	staleKeyA.ApplicationName = "Old App Name"
	staleKeyB := *keyB

	require.NoError(t, store.StoreAPIKey(&staleKeyA))
	require.NoError(t, store.StoreAPIKey(&staleKeyB))

	require.NoError(t, db.SaveConfig(cfgA))
	require.NoError(t, db.SaveConfig(cfgB))
	require.NoError(t, db.SaveAPIKey(keyA))
	require.NoError(t, db.SaveAPIKey(keyB))

	require.NoError(t, db.ReplaceApplicationAPIKeyMappings(
		&models.StoredApplication{
			ApplicationID:   "app-id-1",
			ApplicationUUID: "app-uuid-1",
			ApplicationName: "New App Name",
			ApplicationType: "genai",
		},
		[]*models.ApplicationAPIKeyMapping{{
			ApplicationUUID: "app-uuid-1",
			APIKeyID:        keyB.UUID,
		}},
	))

	listener := &EventListener{
		store:            store,
		db:               db,
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeApplication,
		Action:    "UPDATE",
		EntityID:  "app-uuid-1",
		EventID:   "corr-app-sync",
	})

	storedKeyA, err := store.GetAPIKeyByID(cfgA.UUID, keyA.UUID)
	require.NoError(t, err)
	assert.Empty(t, storedKeyA.ApplicationID)
	assert.Empty(t, storedKeyA.ApplicationName)

	storedKeyB, err := store.GetAPIKeyByID(cfgB.UUID, keyB.UUID)
	require.NoError(t, err)
	assert.Equal(t, "app-uuid-1", storedKeyB.ApplicationID)
	assert.Equal(t, "New App Name", storedKeyB.ApplicationName)

	if assert.Len(t, xdsManager.storeCalls, 2) {
		assert.ElementsMatch(t, []string{keyA.UUID, keyB.UUID}, []string{xdsManager.storeCalls[0].apiKeyID, xdsManager.storeCalls[1].apiKeyID})
	}
	assert.Empty(t, xdsManager.revokeCalls)
	assert.Empty(t, xdsManager.removeCalls)
}

func TestHandleEvent_ApplicationUpdate_ReloadedKeyKeepsCurrentApplicationMapping(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	xdsManager := &mockAPIKeyXDSManager{}

	cfg := testRestStoredConfig("test-api-a", "test-api-a", "Test API A", "v1.0.0", models.StateDeployed)
	key := testAPIKey("api-key-a", "key-a", cfg.UUID)

	require.NoError(t, store.Add(cfg))

	staleKey := *key
	staleKey.ApplicationID = "app-uuid-old"
	staleKey.ApplicationName = "Old App Name"
	require.NoError(t, store.StoreAPIKey(&staleKey))

	require.NoError(t, db.SaveConfig(cfg))
	require.NoError(t, db.SaveAPIKey(key))
	require.NoError(t, db.ReplaceApplicationAPIKeyMappings(
		&models.StoredApplication{
			ApplicationID:   "app-id-new",
			ApplicationUUID: "app-uuid-new",
			ApplicationName: "New App Name",
			ApplicationType: "genai",
		},
		[]*models.ApplicationAPIKeyMapping{{
			ApplicationUUID: "app-uuid-new",
			APIKeyID:        key.UUID,
		}},
	))

	listener := &EventListener{
		store:            store,
		db:               db,
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeApplication,
		Action:    "UPDATE",
		EntityID:  "app-uuid-old",
		EventID:   "corr-app-reassign",
	})

	storedKey, err := store.GetAPIKeyByID(cfg.UUID, key.UUID)
	require.NoError(t, err)
	assert.Equal(t, "app-uuid-new", storedKey.ApplicationID)
	assert.Equal(t, "New App Name", storedKey.ApplicationName)

	if assert.Len(t, xdsManager.storeCalls, 1) {
		assert.Equal(t, key.UUID, xdsManager.storeCalls[0].apiKeyID)
	}
	assert.Empty(t, xdsManager.revokeCalls)
	assert.Empty(t, xdsManager.removeCalls)
}

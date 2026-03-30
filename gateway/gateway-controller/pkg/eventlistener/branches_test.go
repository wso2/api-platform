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
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/apikey"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestProcessAPIEvent_UnknownActionLogsWarning(t *testing.T) {
	var logBuf bytes.Buffer
	listener := &EventListener{
		logger: slog.New(slog.NewTextHandler(&logBuf, nil)),
	}

	listener.processAPIEvent(eventhub.Event{
		Action:   "UPSERT",
		EntityID: "api-1",
	})

	assert.Contains(t, logBuf.String(), "Unknown API event action")
}

func TestHandleAPICreateOrUpdate_MissingConfigInDBDoesNotAddConfig(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	listener := &EventListener{
		store:  store,
		db:     db,
		logger: newTestLogger(),
	}

	listener.handleAPICreateOrUpdate(eventhub.Event{
		Action:   "CREATE",
		EntityID: "missing-api",
		EventID:  "corr-db-miss",
	})

	_, err := store.Get("missing-api")
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func TestHandleAPIDelete_MissingConfigDoesNotCallXDS(t *testing.T) {
	store := storage.NewConfigStore()
	xdsManager := &mockAPIKeyXDSManager{}
	listener := &EventListener{
		store:            store,
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleAPIDelete(eventhub.Event{
		EntityID: "missing-api",
		EventID:  "corr-delete-miss",
	})

	assert.Empty(t, xdsManager.removeCalls)
}

func TestExtractAPINameVersion_NilConfig(t *testing.T) {
	name, version := extractAPINameVersion(nil)

	assert.Empty(t, name)
	assert.Empty(t, version)
}

func TestProcessAPIKeyEvent_UnknownActionLogsWarning(t *testing.T) {
	var logBuf bytes.Buffer
	listener := &EventListener{
		logger: slog.New(slog.NewTextHandler(&logBuf, nil)),
	}

	listener.processAPIKeyEvent(eventhub.Event{
		Action:   "ROTATE",
		EntityID: "api-1_key-1",
	})

	assert.Contains(t, logBuf.String(), "Unknown API key event action")
}

func TestHandleAPIKeyUpsert_InvalidEntityIDDoesNotCallXDS(t *testing.T) {
	xdsManager := &mockAPIKeyXDSManager{}
	listener := &EventListener{
		store:            storage.NewConfigStore(),
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleAPIKeyUpsert(eventhub.Event{
		Action:   "CREATE",
		EntityID: "not-a-valid-entity-id",
		EventID:  "corr-invalid-entity",
	})

	assert.Empty(t, xdsManager.storeCalls)
}

func TestHandleAPIKeyUpsert_MissingAPIKeyInDBDoesNotStoreKey(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	listener := &EventListener{
		store:  store,
		db:     db,
		logger: newTestLogger(),
	}

	listener.handleAPIKeyUpsert(eventhub.Event{
		Action:   "CREATE",
		EntityID: apikey.BuildAPIKeyEntityID("api-1", "missing-key"),
		EventID:  "corr-missing-key",
	})

	_, err := store.GetAPIKeyByID("api-1", "missing-key")
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func TestHandleAPIKeyUpsert_StoreConflictStopsBeforeXDS(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	xdsManager := &mockAPIKeyXDSManager{}
	cfg := testRestStoredConfig("api-1", "test-api", "Test API", "v1.0.0", models.StateDeployed)
	conflicting := testAPIKey("existing-key", "test-key", cfg.UUID)
	current := testAPIKey("incoming-key", "test-key", cfg.UUID)

	require.NoError(t, store.StoreAPIKey(conflicting))
	require.NoError(t, db.SaveConfig(cfg))
	require.NoError(t, db.SaveAPIKey(current))

	listener := &EventListener{
		store:            store,
		db:               db,
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleAPIKeyUpsert(eventhub.Event{
		Action:   "UPDATE",
		EntityID: apikey.BuildAPIKeyEntityID(cfg.UUID, current.UUID),
		EventID:  "corr-store-conflict",
	})

	storedKey, err := store.GetAPIKeyByID(cfg.UUID, conflicting.UUID)
	require.NoError(t, err)
	assert.Equal(t, conflicting.Name, storedKey.Name)
	assert.Empty(t, xdsManager.storeCalls)
}

func TestSyncAPIConfigForAPIKeyEvent_LogsWarningWhenMemorySyncFails(t *testing.T) {
	var logBuf bytes.Buffer
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	conflicting := testRestStoredConfig("existing-api", "test-api", "Test API", "v1.0.0", models.StateDeployed)
	target := testRestStoredConfig("target-api", "test-api", "Test API", "v1.0.0", models.StateDeployed)

	require.NoError(t, store.Add(conflicting))
	require.NoError(t, db.SaveConfig(target))

	listener := &EventListener{
		store:  store,
		db:     db,
		logger: slog.New(slog.NewTextHandler(&logBuf, nil)),
	}

	resolved, err := listener.syncAPIConfigForAPIKeyEvent(target.UUID)

	require.NoError(t, err)
	assert.Equal(t, target.UUID, resolved.UUID)
	assert.Contains(t, logBuf.String(), "Failed to sync API config into memory store while processing API key event")
}

func TestHandleAPIKeyRevoke_InvalidEntityIDDoesNotCallXDS(t *testing.T) {
	xdsManager := &mockAPIKeyXDSManager{}
	listener := &EventListener{
		store:            storage.NewConfigStore(),
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleAPIKeyRevoke(eventhub.Event{
		EntityID: "not-a-valid-entity-id",
		EventID:  "corr-invalid-revoke",
	})

	assert.Empty(t, xdsManager.revokeCalls)
}

func TestHandleAPIKeyRevoke_WithoutStoreReturnsEarly(t *testing.T) {
	xdsManager := &mockAPIKeyXDSManager{}
	listener := &EventListener{
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleAPIKeyRevoke(eventhub.Event{
		EntityID: apikey.BuildAPIKeyEntityID("api-1", "key-1"),
		EventID:  "corr-nil-store",
	})

	assert.Empty(t, xdsManager.revokeCalls)
}

func TestHandleAPIKeyRevoke_WithMissingConfigSkipsXDS(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	xdsManager := &mockAPIKeyXDSManager{}
	apiKey := testAPIKey("key-1", "test-key", "api-1")
	require.NoError(t, store.StoreAPIKey(apiKey))

	listener := &EventListener{
		store:            store,
		db:               db,
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleAPIKeyRevoke(eventhub.Event{
		EntityID: apikey.BuildAPIKeyEntityID("api-1", apiKey.UUID),
		EventID:  "corr-missing-config",
	})

	_, err := store.GetAPIKeyByID("api-1", apiKey.UUID)
	require.ErrorIs(t, err, storage.ErrNotFound)
	assert.Empty(t, xdsManager.revokeCalls)
}

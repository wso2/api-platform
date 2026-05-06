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
	"github.com/wso2/api-platform/common/apikey"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestHandleEvent_APIKeyCreate_SyncsMemoryAndXDS(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	xdsManager := &mockAPIKeyXDSManager{}
	cfg := testRestStoredConfig("test-api-id", "test-api", "Test API", "v1.0.0", models.StateDeployed)
	apiKey := testAPIKey("api-key-id-1", "test-key", cfg.UUID)

	require.NoError(t, db.SaveConfig(cfg))
	require.NoError(t, db.SaveAPIKey(apiKey))

	listener := &EventListener{
		store:            store,
		db:               db,
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPIKey,
		Action:    "CREATE",
		EntityID:  apikey.BuildAPIKeyEntityID(cfg.UUID, apiKey.UUID),
		EventID:   "corr-apikey-create",
	})

	storedKey, err := store.GetAPIKeyByName(cfg.UUID, apiKey.Name)
	require.NoError(t, err)
	assert.Equal(t, apiKey.UUID, storedKey.UUID)

	storedCfg, err := store.Get(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, cfg.DisplayName, storedCfg.DisplayName)

	if assert.Len(t, xdsManager.storeCalls, 1) {
		assert.Equal(t, cfg.UUID, xdsManager.storeCalls[0].apiID)
		assert.Equal(t, cfg.DisplayName, xdsManager.storeCalls[0].apiName)
		assert.Equal(t, cfg.Version, xdsManager.storeCalls[0].apiVersion)
		assert.Equal(t, apiKey.UUID, xdsManager.storeCalls[0].apiKeyID)
		assert.Equal(t, "corr-apikey-create", xdsManager.storeCalls[0].correlationID)
	}
}

func TestHandleEvent_APIKeyUpdate_SyncsMemoryAndXDS(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	xdsManager := &mockAPIKeyXDSManager{}
	cfg := testRestStoredConfig("test-api-id", "test-api", "Test API", "v1.0.0", models.StateDeployed)
	originalKey := testAPIKey("api-key-id-1", "test-key", cfg.UUID)
	updatedKey := testAPIKey("api-key-id-1", "test-key", cfg.UUID)

	require.NoError(t, store.Add(cfg))
	require.NoError(t, store.StoreAPIKey(originalKey))
	require.NoError(t, db.SaveConfig(cfg))
	require.NoError(t, db.SaveAPIKey(updatedKey))

	listener := &EventListener{
		store:            store,
		db:               db,
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPIKey,
		Action:    "UPDATE",
		EntityID:  apikey.BuildAPIKeyEntityID(cfg.UUID, updatedKey.UUID),
		EventID:   "corr-apikey-update",
	})

	storedKey, err := store.GetAPIKeyByName(cfg.UUID, updatedKey.Name)
	require.NoError(t, err)
	assert.Equal(t, updatedKey.Name, storedKey.Name)

	if assert.Len(t, xdsManager.storeCalls, 1) {
		assert.Equal(t, cfg.UUID, xdsManager.storeCalls[0].apiID)
		assert.Equal(t, cfg.DisplayName, xdsManager.storeCalls[0].apiName)
		assert.Equal(t, cfg.Version, xdsManager.storeCalls[0].apiVersion)
		assert.Equal(t, updatedKey.UUID, xdsManager.storeCalls[0].apiKeyID)
		assert.Equal(t, "corr-apikey-update", xdsManager.storeCalls[0].correlationID)
	}
}

func TestHandleEvent_APIKeyDelete_RemovesMemoryAndXDS(t *testing.T) {
	store := storage.NewConfigStore()
	xdsManager := &mockAPIKeyXDSManager{}
	cfg := testRestStoredConfig("test-api-id", "test-api", "Test API", "v1.0.0", models.StateDeployed)
	apiKey := testAPIKey("api-key-id-1", "test-key", cfg.UUID)

	require.NoError(t, store.Add(cfg))
	require.NoError(t, store.StoreAPIKey(apiKey))

	listener := &EventListener{
		store:            store,
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPIKey,
		Action:    "DELETE",
		EntityID:  apikey.BuildAPIKeyEntityID(cfg.UUID, apiKey.UUID),
		EventID:   "corr-apikey-delete",
	})

	_, err := store.GetAPIKeyByName(cfg.UUID, apiKey.Name)
	require.ErrorIs(t, err, storage.ErrNotFound)

	if assert.Len(t, xdsManager.revokeCalls, 1) {
		assert.Equal(t, cfg.UUID, xdsManager.revokeCalls[0].apiID)
		assert.Equal(t, cfg.DisplayName, xdsManager.revokeCalls[0].apiName)
		assert.Equal(t, cfg.Version, xdsManager.revokeCalls[0].apiVersion)
		assert.Equal(t, apiKey.Name, xdsManager.revokeCalls[0].apiKeyName)
		assert.Equal(t, "corr-apikey-delete", xdsManager.revokeCalls[0].correlationID)
	}
}

func TestHandleEvent_APIKeyDelete_SkipsXDSWhenKeyNameIsUnavailable(t *testing.T) {
	store := storage.NewConfigStore()
	xdsManager := &mockAPIKeyXDSManager{}
	cfg := testRestStoredConfig("test-api-id", "test-api", "Test API", "v1.0.0", models.StateDeployed)
	require.NoError(t, store.Add(cfg))

	listener := &EventListener{
		store:            store,
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPIKey,
		Action:    "DELETE",
		EntityID:  apikey.BuildAPIKeyEntityID(cfg.UUID, "missing-key-id"),
		EventID:   "corr-apikey-missing",
	})

	assert.Empty(t, xdsManager.revokeCalls)
}

func TestHandleEvent_APIKeyCreate_SyncsMemoryAndXDS_ForLLMProxy(t *testing.T) {
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	xdsManager := &mockAPIKeyXDSManager{}
	providerCfg := testLLMProviderStoredConfig("provider-1", "provider-a", "openai", nil)
	require.NoError(t, db.SaveConfig(providerCfg))
	cfg := &models.StoredConfig{
		UUID:        "test-llm-proxy-id",
		Kind:        string(api.LLMProxyConfigurationKindLlmProxy),
		Handle:      "test-llm-proxy",
		DisplayName: "Test LLM Proxy",
		Version:     "v1.0.0",
		SourceConfiguration: api.LLMProxyConfiguration{
			ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProxyConfigurationKindLlmProxy,
			Metadata: api.Metadata{
				Name: "test-llm-proxy",
			},
			Spec: api.LLMProxyConfigData{
				DisplayName: "Test LLM Proxy",
				Version:     "v1.0.0",
				Provider: api.LLMProxyProvider{
					Id: "provider-a",
				},
			},
		},
		DesiredState: models.StateDeployed,
	}
	apiKey := testAPIKey("api-key-id-llm-proxy", "test-key", cfg.UUID)

	require.NoError(t, db.SaveConfig(cfg))
	require.NoError(t, db.SaveAPIKey(apiKey))

	listener := &EventListener{
		store:            store,
		db:               db,
		apiKeyXDSManager: xdsManager,
		logger:           newTestLogger(),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeAPIKey,
		Action:    "CREATE",
		EntityID:  apikey.BuildAPIKeyEntityID(cfg.UUID, apiKey.UUID),
		EventID:   "corr-apikey-create-llm-proxy",
	})

	storedKey, err := store.GetAPIKeyByName(cfg.UUID, apiKey.Name)
	require.NoError(t, err)
	assert.Equal(t, apiKey.UUID, storedKey.UUID)

	storedCfg, err := store.Get(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, cfg.DisplayName, storedCfg.DisplayName)
	assert.Equal(t, cfg.Kind, storedCfg.Kind)

	if assert.Len(t, xdsManager.storeCalls, 1) {
		assert.Equal(t, cfg.UUID, xdsManager.storeCalls[0].apiID)
		assert.Equal(t, cfg.DisplayName, xdsManager.storeCalls[0].apiName)
		assert.Equal(t, cfg.Version, xdsManager.storeCalls[0].apiVersion)
		assert.Equal(t, apiKey.UUID, xdsManager.storeCalls[0].apiKeyID)
		assert.Equal(t, "corr-apikey-create-llm-proxy", xdsManager.storeCalls[0].correlationID)
	}
}

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
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

type mockAPIKeyXDSManager struct {
	calls       []storeCall
	revokeCalls []revokeCall
	removeCalls []removeCall
}

type storeCall struct {
	apiID         string
	apiName       string
	apiVersion    string
	apiKeyID      string
	correlationID string
}

type revokeCall struct {
	apiID         string
	apiName       string
	apiVersion    string
	apiKeyName    string
	correlationID string
}

type removeCall struct {
	apiID         string
	apiName       string
	apiVersion    string
	correlationID string
}

func (m *mockAPIKeyXDSManager) StoreAPIKey(apiId, apiName, apiVersion string, apiKey *models.APIKey, correlationID string) error {
	m.calls = append(m.calls, storeCall{
		apiID:         apiId,
		apiName:       apiName,
		apiVersion:    apiVersion,
		apiKeyID:      apiKey.ID,
		correlationID: correlationID,
	})
	return nil
}

func (m *mockAPIKeyXDSManager) RevokeAPIKey(apiId, apiName, apiVersion, apiKeyName, correlationID string) error {
	m.revokeCalls = append(m.revokeCalls, revokeCall{
		apiID:         apiId,
		apiName:       apiName,
		apiVersion:    apiVersion,
		apiKeyName:    apiKeyName,
		correlationID: correlationID,
	})
	return nil
}

func (m *mockAPIKeyXDSManager) RemoveAPIKeysByAPI(apiId, apiName, apiVersion, correlationID string) error {
	m.removeCalls = append(m.removeCalls, removeCall{
		apiID:         apiId,
		apiName:       apiName,
		apiVersion:    apiVersion,
		correlationID: correlationID,
	})
	return nil
}

func setupSQLiteDBForEventListenerTests(t *testing.T) storage.Storage {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	metrics.Init()
	dbPath := filepath.Join(t.TempDir(), "eventlistener-test.db")
	db, err := storage.NewStorage(storage.BackendConfig{
		Type:       "sqlite",
		SQLitePath: dbPath,
		GatewayID:  "platform-gateway-id",
	}, logger)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func TestHandleEvent_APIKeyCreate_SyncsMemoryAndXDS(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	xdsManager := &mockAPIKeyXDSManager{}

	handle := "test-api-handle"
	apiID := "test-api-id"

	var spec api.APIConfiguration_Spec
	err := spec.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Test API",
		Version:     "v1.0.0",
		Context:     "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{Url: ptr("https://example.com")},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/",
			},
		},
	})
	require.NoError(t, err)

	cfg := &models.StoredConfig{
		ID:   apiID,
		Kind: string(api.RestApi),
		Configuration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name: handle,
			},
			Spec: spec,
		},
		SourceConfiguration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name: handle,
			},
			Spec: spec,
		},
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, db.SaveConfig(cfg))

	apiKey := &models.APIKey{
		ID:           "api-key-id-1",
		Name:         "test-key",
		DisplayName:  "Test Key",
		APIKey:       "hashed-key-value",
		MaskedAPIKey: "***test-key",
		APIId:        apiID,
		Operations:   "[\"*\"]",
		Status:       models.APIKeyStatusActive,
		CreatedAt:    time.Now(),
		CreatedBy:    "test-user",
		UpdatedAt:    time.Now(),
		Source:       "local",
	}
	require.NoError(t, db.SaveAPIKey(apiKey))

	listener := &EventListener{
		store:            store,
		db:               db,
		apiKeyXDSManager: xdsManager,
		logger:           logger,
		systemConfig: &config.Config{
			Router: config.RouterConfig{
				GatewayHost: "localhost",
			},
		},
	}

	listener.handleEvent(eventhub.Event{
		EventType:     eventhub.EventTypeAPIKey,
		Action:        "CREATE",
		EntityID:      eventhub.BuildAPIKeyEntityID(apiID, apiKey.ID),
		CorrelationID: "corr-apikey-create",
	})

	storedKey, err := store.GetAPIKeyByName(apiID, "test-key")
	require.NoError(t, err)
	assert.Equal(t, apiKey.ID, storedKey.ID)

	if assert.Len(t, xdsManager.calls, 1) {
		assert.Equal(t, apiID, xdsManager.calls[0].apiID)
		assert.Equal(t, "Test API", xdsManager.calls[0].apiName)
		assert.Equal(t, "v1.0.0", xdsManager.calls[0].apiVersion)
		assert.Equal(t, apiKey.ID, xdsManager.calls[0].apiKeyID)
		assert.Equal(t, "corr-apikey-create", xdsManager.calls[0].correlationID)
	}
}

func TestHandleEvent_APIKeyUpdateActions_SyncsMemoryAndXDS(t *testing.T) {
	for _, action := range []string{"UPDATE", "REGENERATE"} {
		t.Run(action, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			store := storage.NewConfigStore()
			db := setupSQLiteDBForEventListenerTests(t)
			xdsManager := &mockAPIKeyXDSManager{}

			handle := "test-api-handle"
			apiID := "test-api-id"

			var spec api.APIConfiguration_Spec
			err := spec.FromAPIConfigData(api.APIConfigData{
				DisplayName: "Test API",
				Version:     "v1.0.0",
				Context:     "/test",
				Upstream: struct {
					Main    api.Upstream  `json:"main" yaml:"main"`
					Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
				}{
					Main: api.Upstream{Url: ptr("https://example.com")},
				},
				Operations: []api.Operation{
					{
						Method: "GET",
						Path:   "/",
					},
				},
			})
			require.NoError(t, err)

			cfg := &models.StoredConfig{
				ID:   apiID,
				Kind: string(api.RestApi),
				Configuration: api.APIConfiguration{
					ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
					Kind:       api.RestApi,
					Metadata: api.Metadata{
						Name: handle,
					},
					Spec: spec,
				},
				SourceConfiguration: api.APIConfiguration{
					ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
					Kind:       api.RestApi,
					Metadata: api.Metadata{
						Name: handle,
					},
					Spec: spec,
				},
				Status:    models.StatusPending,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			require.NoError(t, db.SaveConfig(cfg))

			apiKey := &models.APIKey{
				ID:           "api-key-id-1",
				Name:         "test-key",
				DisplayName:  "Updated Key",
				APIKey:       "hashed-key-value",
				MaskedAPIKey: "***updated-key",
				APIId:        apiID,
				Operations:   "[\"*\"]",
				Status:       models.APIKeyStatusActive,
				CreatedAt:    time.Now(),
				CreatedBy:    "test-user",
				UpdatedAt:    time.Now(),
				Source:       "external",
			}
			require.NoError(t, db.SaveAPIKey(apiKey))

			listener := &EventListener{
				store:            store,
				db:               db,
				apiKeyXDSManager: xdsManager,
				logger:           logger,
				systemConfig: &config.Config{
					Router: config.RouterConfig{
						GatewayHost: "localhost",
					},
				},
			}

			listener.handleEvent(eventhub.Event{
				EventType:     eventhub.EventTypeAPIKey,
				Action:        action,
				EntityID:      eventhub.BuildAPIKeyEntityID(apiID, apiKey.ID),
				CorrelationID: "corr-apikey-upsert",
			})

			storedKey, err := store.GetAPIKeyByName(apiID, "test-key")
			require.NoError(t, err)
			assert.Equal(t, apiKey.ID, storedKey.ID)
			assert.Equal(t, apiKey.DisplayName, storedKey.DisplayName)

			if assert.Len(t, xdsManager.calls, 1) {
				assert.Equal(t, apiID, xdsManager.calls[0].apiID)
				assert.Equal(t, "Test API", xdsManager.calls[0].apiName)
				assert.Equal(t, "v1.0.0", xdsManager.calls[0].apiVersion)
				assert.Equal(t, apiKey.ID, xdsManager.calls[0].apiKeyID)
				assert.Equal(t, "corr-apikey-upsert", xdsManager.calls[0].correlationID)
			}
		})
	}
}

func TestHandleEvent_APIKeyRevoke_RemovesMemoryAndXDS(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := setupSQLiteDBForEventListenerTests(t)
	xdsManager := &mockAPIKeyXDSManager{}

	handle := "test-api-handle"
	apiID := "test-api-id"

	var spec api.APIConfiguration_Spec
	err := spec.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Test API",
		Version:     "v1.0.0",
		Context:     "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{Url: ptr("https://example.com")},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/",
			},
		},
	})
	require.NoError(t, err)

	cfg := &models.StoredConfig{
		ID:   apiID,
		Kind: string(api.RestApi),
		Configuration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name: handle,
			},
			Spec: spec,
		},
		SourceConfiguration: api.APIConfiguration{
			ApiVersion: api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name: handle,
			},
			Spec: spec,
		},
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, db.SaveConfig(cfg))

	apiKey := &models.APIKey{
		ID:           "api-key-id-1",
		Name:         "test-key",
		DisplayName:  "Test Key",
		APIKey:       "hashed-key-value",
		MaskedAPIKey: "***test-key",
		APIId:        apiID,
		Operations:   "[\"*\"]",
		Status:       models.APIKeyStatusActive,
		CreatedAt:    time.Now(),
		CreatedBy:    "test-user",
		UpdatedAt:    time.Now(),
		Source:       "external",
	}
	require.NoError(t, store.StoreAPIKey(apiKey))

	listener := &EventListener{
		store:            store,
		db:               db,
		apiKeyXDSManager: xdsManager,
		logger:           logger,
		systemConfig: &config.Config{
			Router: config.RouterConfig{
				GatewayHost: "localhost",
			},
		},
	}

	listener.handleEvent(eventhub.Event{
		EventType:     eventhub.EventTypeAPIKey,
		Action:        "REVOKE",
		EntityID:      eventhub.BuildAPIKeyEntityID(apiID, apiKey.ID),
		CorrelationID: "corr-apikey-revoke",
	})

	_, err = store.GetAPIKeyByName(apiID, "test-key")
	require.ErrorIs(t, err, storage.ErrNotFound)

	if assert.Len(t, xdsManager.revokeCalls, 1) {
		assert.Equal(t, apiID, xdsManager.revokeCalls[0].apiID)
		assert.Equal(t, "Test API", xdsManager.revokeCalls[0].apiName)
		assert.Equal(t, "v1.0.0", xdsManager.revokeCalls[0].apiVersion)
		assert.Equal(t, "test-key", xdsManager.revokeCalls[0].apiKeyName)
		assert.Equal(t, "corr-apikey-revoke", xdsManager.revokeCalls[0].correlationID)
	}
}

func ptr(v string) *string {
	return &v
}

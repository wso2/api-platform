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

package controlplane

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// mockStorageForDeletion implements storage.Storage interface for deletion testing
type mockStorageForDeletion struct {
	configs            map[string]*models.StoredConfig
	secrets            map[string]*models.Secret
	deleteErr          error
	getErr             error
	removeKeyErr       error
	updateErr          error
	deleteCallCount    int
	removeKeyCallCount int
}

func newMockStorageForDeletion() *mockStorageForDeletion {
	return &mockStorageForDeletion{
		configs: make(map[string]*models.StoredConfig),
		secrets: make(map[string]*models.Secret),
	}
}

func (m *mockStorageForDeletion) SaveConfig(config *models.StoredConfig) error {
	m.configs[config.ID] = config
	return nil
}

func (m *mockStorageForDeletion) GetConfig(id string) (*models.StoredConfig, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	config, exists := m.configs[id]
	if !exists {
		return nil, storage.ErrNotFound
	}
	return config, nil
}

func (m *mockStorageForDeletion) UpdateConfig(config *models.StoredConfig) error {
	m.configs[config.ID] = config
	return nil
}

func (m *mockStorageForDeletion) DeleteConfig(id string) error {
	m.deleteCallCount++
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.configs, id)
	return nil
}

func (m *mockStorageForDeletion) ListConfigs() ([]*models.StoredConfig, error) {
	var configs []*models.StoredConfig
	for _, config := range m.configs {
		configs = append(configs, config)
	}
	return configs, nil
}

func (m *mockStorageForDeletion) GetAllConfigs() ([]*models.StoredConfig, error) {
	return m.ListConfigs()
}

func (m *mockStorageForDeletion) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	var configs []*models.StoredConfig
	for _, config := range m.configs {
		if config.Kind == kind {
			configs = append(configs, config)
		}
	}
	return configs, nil
}

func (m *mockStorageForDeletion) GetConfigByNameVersion(name, version string) (*models.StoredConfig, error) {
	for _, config := range m.configs {
		if config.GetDisplayName() == name && config.GetVersion() == version {
			return config, nil
		}
	}
	return nil, fmt.Errorf("config not found")
}

func (m *mockStorageForDeletion) GetConfigByHandle(handle string) (*models.StoredConfig, error) {
	for _, config := range m.configs {
		if config.GetHandle() == handle {
			return config, nil
		}
	}
	return nil, fmt.Errorf("config not found")
}

func (m *mockStorageForDeletion) Close() error {
	return nil
}

func (m *mockStorageForDeletion) SaveAPIKey(key *models.APIKey) error {
	return nil
}

func (m *mockStorageForDeletion) GetAPIKey(apiID, name string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) GetAPIKeyByValue(keyValue string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) ListAPIKeys(apiID string) ([]*models.APIKey, error) {
	return nil, nil
}

func (m *mockStorageForDeletion) UpdateAPIKey(key *models.APIKey) error {
	return nil
}

func (m *mockStorageForDeletion) DeleteAPIKey(apiID string) error {
	return nil
}

func (m *mockStorageForDeletion) GetAPIKeyByID(apiKeyID string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) GetAPIKeyByKey(key string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) GetAPIKeysByAPI(apiID string) ([]*models.APIKey, error) {
	return nil, nil
}

func (m *mockStorageForDeletion) GetAllAPIKeys() ([]*models.APIKey, error) {
	return nil, nil
}

func (m *mockStorageForDeletion) GetAPIKeysByAPIAndName(apiID, name string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) RemoveAPIKeyAPIAndName(apiID, name string) error {
	return nil
}

func (m *mockStorageForDeletion) RemoveAPIKeysAPI(apiID string) error {
	m.removeKeyCallCount++
	if m.removeKeyErr != nil {
		return m.removeKeyErr
	}
	return nil
}

func (m *mockStorageForDeletion) CountActiveAPIKeysByUserAndAPI(userID, apiID string) (int, error) {
	return 0, nil
}

// Certificate methods (not used in deletion tests but required by interface)
func (m *mockStorageForDeletion) SaveCertificate(cert *models.StoredCertificate) error {
	return nil
}

func (m *mockStorageForDeletion) GetCertificate(id string) (*models.StoredCertificate, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) ListCertificates() ([]*models.StoredCertificate, error) {
	return nil, nil
}

func (m *mockStorageForDeletion) DeleteCertificate(id string) error {
	return nil
}

func (m *mockStorageForDeletion) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	return nil, storage.ErrNotFound
}

// Secret methods (not used in deletion tests but required by interface)
func (m *mockStorageForDeletion) SaveSecret(secret *models.Secret) error {
	if m.deleteErr != nil { // reuse existing error fields only if needed
		return m.deleteErr
	}
	m.secrets[secret.Handle] = secret
	return nil
}

func (m *mockStorageForDeletion) GetSecrets() ([]string, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	ids := make([]string, 0, len(m.secrets))
	for handle := range m.secrets {
		ids = append(ids, handle)
	}
	return ids, nil
}

func (m *mockStorageForDeletion) GetSecret(handle string) (*models.Secret, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if s, ok := m.secrets[handle]; ok {
		return s, nil
	}
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) UpdateSecret(secret *models.Secret) (*models.Secret, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	if _, ok := m.secrets[secret.Handle]; !ok {
		return nil, storage.ErrNotFound
	}
	m.secrets[secret.Handle] = secret
	return secret, nil
}

func (m *mockStorageForDeletion) DeleteSecret(handle string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.secrets[handle]; !ok {
		return storage.ErrNotFound
	}
	delete(m.secrets, handle)
	return nil
}

func (m *mockStorageForDeletion) SecretExists(handle string) (bool, error) {
	if m.getErr != nil {
		return false, m.getErr
	}
	_, ok := m.secrets[handle]
	return ok, nil
}

// LLMProviderTemplate methods (not used in deletion tests but required by interface)
func (m *mockStorageForDeletion) SaveLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error {
	return nil
}

func (m *mockStorageForDeletion) UpdateLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error {
	return nil
}

func (m *mockStorageForDeletion) GetLLMProviderTemplate(id string) (*models.StoredLLMProviderTemplate, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) ListLLMProviderTemplates() ([]*models.StoredLLMProviderTemplate, error) {
	return nil, nil
}

func (m *mockStorageForDeletion) GetAllLLMProviderTemplates() ([]*models.StoredLLMProviderTemplate, error) {
	return nil, nil
}

func (m *mockStorageForDeletion) DeleteLLMProviderTemplate(id string) error {
	return nil
}

// mockXDSManager implements a mock XDSManager for testing
type mockXDSManager struct {
	removedAPIs []string
	removeErr   error
}

func newMockXDSManager() *mockXDSManager {
	return &mockXDSManager{
		removedAPIs: make([]string, 0),
	}
}

func (m *mockXDSManager) StoreAPIKey(apiId, apiName, apiVersion string, apiKey *models.APIKey, correlationID string) error {
	return nil
}

func (m *mockXDSManager) RevokeAPIKey(apiId, apiName, apiVersion, apiKeyName, correlationID string) error {
	return nil
}

func (m *mockXDSManager) RemoveAPIKeysByAPI(apiId, apiName, apiVersion, correlationID string) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	m.removedAPIs = append(m.removedAPIs, apiId)
	return nil
}

// Helper to create test API config for deletion tests
func createTestAPIConfigForDeletion(apiID string) *models.StoredConfig {
	// Create a complete API configuration so deletion flow can properly process it
	specUnion := api.APIConfiguration_Spec{}
	specUnion.FromAPIConfigData(api.APIConfigData{
		DisplayName: "Test API",
		Version:     "v1",
		Context:     "/test",
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: func() *string { s := "http://backend.example.com"; return &s }(),
			},
		},
		Operations: []api.Operation{
			{
				Method: "GET",
				Path:   "/resource",
			},
		},
	})

	return &models.StoredConfig{
		ID:     apiID,
		Status: models.StatusDeployed,
		Kind:   "API",
		Configuration: api.APIConfiguration{
			ApiVersion: "gateway.wso2.com/v1",
			Kind:       api.RestApi,
			Metadata: api.Metadata{
				Name: apiID,
			},
			Spec: specUnion,
		},
	}
}

// TestClient_handleAPIDeletedEvent_InvalidPayload tests invalid event handling
func TestClient_handleAPIDeletedEvent_InvalidPayload(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewConfigStore()
	db := newMockStorageForDeletion()

	client := &Client{
		logger: logger,
		store:  store,
		db:     db,
	}

	tests := []struct {
		name  string
		event map[string]interface{}
	}{
		{
			name: "Missing API ID",
			event: map[string]interface{}{
				"type": "api.deleted",
				"payload": map[string]interface{}{
					"environment": "production",
				},
				"timestamp":     time.Now().Format(time.RFC3339),
				"correlationId": "corr-invalid",
			},
		},
		{
			name: "Invalid payload type",
			event: map[string]interface{}{
				"type":          "api.deleted",
				"payload":       "not-a-map",
				"timestamp":     time.Now().Format(time.RFC3339),
				"correlationId": "corr-invalid-2",
			},
		},
		{
			name: "No payload",
			event: map[string]interface{}{
				"type":          "api.deleted",
				"timestamp":     time.Now().Format(time.RFC3339),
				"correlationId": "corr-invalid-3",
			},
		},
		{
			name: "Empty API ID",
			event: map[string]interface{}{
				"type": "api.deleted",
				"payload": map[string]interface{}{
					"apiId":       "",
					"environment": "production",
				},
				"timestamp":     time.Now().Format(time.RFC3339),
				"correlationId": "corr-invalid-4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should handle gracefully without panic
			client.handleAPIDeletedEvent(tt.event)
		})
	}
}

// TestClient_handleAPIDeletedEvent_OrphanedCleanup tests orphaned resource cleanup
func TestClient_handleAPIDeletedEvent_OrphanedCleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewConfigStore()
	db := newMockStorageForDeletion()

	client := &Client{
		logger: logger,
		store:  store,
		db:     db,
	}

	apiID := "non-existent-api"
	event := map[string]interface{}{
		"type": "api.deleted",
		"payload": map[string]interface{}{
			"apiId":       apiID,
			"environment": "production",
			"vhost":       "api.example.com",
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-orphan",
	}

	client.handleAPIDeletedEvent(event)

	// Verify orphaned cleanup was attempted
	if db.removeKeyCallCount != 1 {
		t.Errorf("Expected RemoveAPIKeysAPI to be called for orphan cleanup, got %d", db.removeKeyCallCount)
	}

	// Config should still not exist
	_, err := store.Get(apiID)
	if err == nil {
		t.Error("API config should not exist in store after orphan cleanup")
	}
}

// TestClient_handleAPIDeletedEvent_FullDeletion tests complete deletion workflow
func TestClient_handleAPIDeletedEvent_FullDeletion(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewConfigStore()
	db := newMockStorageForDeletion()
	xdsMgr := newMockXDSManager()

	// Create a real PolicyManager with mock dependencies
	policyStore := storage.NewPolicyStore()
	policySnapshotMgr := policyxds.NewSnapshotManager(policyStore, logger)
	policyMgr := policyxds.NewPolicyManager(policyStore, policySnapshotMgr, logger)

	apiID := "test-api-full-delete"
	apiConfig := createTestAPIConfigForDeletion(apiID)
	db.SaveConfig(apiConfig)
	store.Add(apiConfig)

	client := &Client{
		logger:           logger,
		store:            store,
		db:               db,
		policyManager:    policyMgr,
		apiKeyXDSManager: xdsMgr,
	}

	event := map[string]interface{}{
		"type": "api.deleted",
		"payload": map[string]interface{}{
			"apiId":       apiID,
			"environment": "production",
			"vhost":       "api.example.com",
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-full",
	}

	client.handleAPIDeletedEvent(event)

	// Verify deletion was performed
	if db.deleteCallCount != 1 {
		t.Errorf("Expected DeleteConfig to be called once, got %d", db.deleteCallCount)
	}

	if db.removeKeyCallCount != 1 {
		t.Errorf("Expected RemoveAPIKeysAPI to be called once, got %d", db.removeKeyCallCount)
	}

	// Verify API keys removed from XDS manager
	if len(xdsMgr.removedAPIs) != 1 {
		t.Errorf("Expected XDS manager to remove API keys for 1 API, got %d", len(xdsMgr.removedAPIs))
	} else if xdsMgr.removedAPIs[0] != apiID {
		t.Errorf("Expected XDS manager to remove keys for API %s, got %s", apiID, xdsMgr.removedAPIs[0])
	}

	// Verify config removed from memory
	_, err := store.Get(apiID)
	if err == nil {
		t.Error("Expected API config to be removed from memory store")
	}

	// Verify policy cleanup was called (policy ID would be apiID + "-policies")
	policyID := apiID + "-policies"
	_, policyExists := policyStore.Get(policyID)
	if policyExists {
		// If policy existed, it should have been removed
		t.Error("Expected policy to be removed from policy store")
	}
	// Note: If policy never existed, Get returns false which is expected
}

// TestClient_handleAPIDeletedEvent_MemoryOnly tests deletion when no database exists
func TestClient_handleAPIDeletedEvent_MemoryOnly(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewConfigStore()

	apiID := "test-api-memory-only"
	apiConfig := createTestAPIConfigForDeletion(apiID)
	store.Add(apiConfig)

	client := &Client{
		logger: logger,
		store:  store,
		db:     nil, // No database
	}

	event := map[string]interface{}{
		"type": "api.deleted",
		"payload": map[string]interface{}{
			"apiId":       apiID,
			"environment": "production",
			"vhost":       "api.example.com",
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-mem",
	}

	client.handleAPIDeletedEvent(event)

	// Verify config removed from memory
	_, err := store.Get(apiID)
	if err == nil {
		t.Error("Expected API config to be removed from memory store")
	}
}

// TestClient_handleAPIDeletedEvent_StorageErrors tests error handling
func TestClient_handleAPIDeletedEvent_StorageErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewConfigStore()
	db := newMockStorageForDeletion()

	apiID := "test-api-error"
	apiConfig := createTestAPIConfigForDeletion(apiID)
	db.SaveConfig(apiConfig)
	store.Add(apiConfig)

	// Simulate storage errors
	db.deleteErr = errors.New("database connection failed")
	db.removeKeyErr = errors.New("failed to remove keys")

	client := &Client{
		logger: logger,
		store:  store,
		db:     db,
	}

	event := map[string]interface{}{
		"type": "api.deleted",
		"payload": map[string]interface{}{
			"apiId":       apiID,
			"environment": "production",
			"vhost":       "api.example.com",
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-error",
	}

	// Should handle errors gracefully without panic
	client.handleAPIDeletedEvent(event)

	// Operations should have been attempted despite errors
	if db.deleteCallCount != 1 {
		t.Errorf("Expected DeleteConfig to be attempted, got %d", db.deleteCallCount)
	}

	if db.removeKeyCallCount != 1 {
		t.Errorf("Expected RemoveAPIKeysAPI to be attempted, got %d", db.removeKeyCallCount)
	}
}

// TestClient_findAPIConfig tests API config lookup
func TestClient_findAPIConfig(t *testing.T) {
	t.Run("Returns config from database when available", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
		store := storage.NewConfigStore()
		db := newMockStorageForDeletion()

		apiID := "test-api-db"
		apiConfig := createTestAPIConfigForDeletion(apiID)
		db.SaveConfig(apiConfig)

		client := &Client{
			logger: logger,
			store:  store,
			db:     db,
		}

		config, err := client.findAPIConfig(apiID)
		if err != nil {
			t.Errorf("Expected to find API config in database, got error: %v", err)
		}
		if config.ID != apiID {
			t.Errorf("Expected API ID %s, got %s", apiID, config.ID)
		}
	})

	t.Run("Returns config from memory when not in database", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
		store := storage.NewConfigStore()
		db := newMockStorageForDeletion()

		apiID := "test-api-memory"
		apiConfig := createTestAPIConfigForDeletion(apiID)
		store.Add(apiConfig)

		client := &Client{
			logger: logger,
			store:  store,
			db:     db,
		}

		config, err := client.findAPIConfig(apiID)
		if err != nil {
			t.Errorf("Expected to find API config in memory store, got error: %v", err)
		}
		if config.ID != apiID {
			t.Errorf("Expected API ID %s, got %s", apiID, config.ID)
		}
	})

	t.Run("Returns ErrNotFound when config does not exist", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
		store := storage.NewConfigStore()
		db := newMockStorageForDeletion()

		client := &Client{
			logger: logger,
			store:  store,
			db:     db,
		}

		_, err := client.findAPIConfig("non-existent")
		if !storage.IsNotFoundError(err) {
			t.Errorf("Expected ErrNotFound for non-existent API, got: %v", err)
		}
	})

	t.Run("Works when database is nil", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
		store := storage.NewConfigStore()

		apiID := "test-api-no-db"
		apiConfig := createTestAPIConfigForDeletion(apiID)
		store.Add(apiConfig)

		client := &Client{
			logger: logger,
			store:  store,
			db:     nil,
		}

		config, err := client.findAPIConfig(apiID)
		if err != nil {
			t.Errorf("Expected to find API config in memory when DB is nil, got error: %v", err)
		}
		if config.ID != apiID {
			t.Errorf("Expected API ID %s, got %s", apiID, config.ID)
		}
	})
}

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
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// mockStorageForDeletion implements storage.Storage interface for deletion testing
type mockStorageForDeletion struct {
	configs            map[string]*models.StoredConfig
	deleteErr          error
	getErr             error
	removeKeyErr       error
	deleteCallCount    int
	removeKeyCallCount int
}

func newMockStorageForDeletion() *mockStorageForDeletion {
	return &mockStorageForDeletion{
		configs: make(map[string]*models.StoredConfig),
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

func (m *mockStorageForDeletion) GetDB() *sql.DB {
	return nil
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

type mockEventHubForDelete struct {
	publishedEvents []eventhub.Event
	publishErr      error
}

func (m *mockEventHubForDelete) Initialize() error {
	return nil
}

func (m *mockEventHubForDelete) RegisterOrganization(orgID string) error {
	return nil
}

func (m *mockEventHubForDelete) PublishEvent(orgID string, event eventhub.Event) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}

func (m *mockEventHubForDelete) Subscribe(orgID string) (<-chan eventhub.Event, error) {
	ch := make(chan eventhub.Event)
	return ch, nil
}

func (m *mockEventHubForDelete) CleanUpEvents() error {
	return nil
}

func (m *mockEventHubForDelete) Close() error {
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

func TestClient_handleAPIDeletedEvent_UpdatesDBAndPublishesEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newMockStorageForDeletion()
	hub := &mockEventHubForDelete{}

	apiID := "test-api-delete-1"

	dbConfig := createTestAPIConfigForDeletion(apiID)
	dbConfig.Configuration.Metadata.Name = "delete-handle"
	db.configs[apiID] = dbConfig

	memConfig := createTestAPIConfigForDeletion(apiID)
	memConfig.Configuration.Metadata.Name = "delete-handle"
	if err := store.Add(memConfig); err != nil {
		t.Fatalf("failed to seed in-memory config: %v", err)
	}

	client := &Client{
		logger:   logger,
		store:    store,
		db:       db,
		eventHub: hub,
	}

	event := map[string]interface{}{
		"type": "api.deleted",
		"payload": map[string]interface{}{
			"apiId": apiID,
			"vhost": "api.example.com",
		},
		"timestamp":     "2026-01-01T00:00:00Z",
		"correlationId": "corr-delete-1",
	}

	client.handleAPIDeletedEvent(event)

	if db.deleteCallCount != 1 {
		t.Fatalf("delete call count = %d, want 1", db.deleteCallCount)
	}
	if db.removeKeyCallCount != 1 {
		t.Fatalf("remove key call count = %d, want 1", db.removeKeyCallCount)
	}

	_, err := db.GetConfig(apiID)
	if !storage.IsNotFoundError(err) {
		t.Fatalf("database lookup error = %v, want not found", err)
	}

	inMemoryConfig, err := store.Get(apiID)
	if err != nil {
		t.Fatalf("expected API config to remain in memory until async event processing: %v", err)
	}
	if inMemoryConfig.ID != apiID {
		t.Fatalf("in-memory config id = %s, want %s", inMemoryConfig.ID, apiID)
	}

	if len(hub.publishedEvents) != 1 {
		t.Fatalf("published event count = %d, want 1", len(hub.publishedEvents))
	}

	published := hub.publishedEvents[0]
	if published.EventType != eventhub.EventTypeAPI {
		t.Fatalf("published event type = %s, want %s", published.EventType, eventhub.EventTypeAPI)
	}
	if published.Action != "DELETE" {
		t.Fatalf("published action = %s, want DELETE", published.Action)
	}
	if published.EntityID != apiID {
		t.Fatalf("published entity id = %s, want %s", published.EntityID, apiID)
	}
	if published.CorrelationID != "corr-delete-1" {
		t.Fatalf("published correlation id = %s, want corr-delete-1", published.CorrelationID)
	}
}

func TestClient_handleAPIDeletedEvent_NotFoundDoesNotPublish(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newMockStorageForDeletion()
	hub := &mockEventHubForDelete{}

	client := &Client{
		logger:   logger,
		store:    store,
		db:       db,
		eventHub: hub,
	}

	event := map[string]interface{}{
		"type": "api.deleted",
		"payload": map[string]interface{}{
			"apiId": "missing-api",
		},
		"timestamp":     "2026-01-01T00:00:00Z",
		"correlationId": "corr-delete-missing",
	}

	client.handleAPIDeletedEvent(event)

	if db.deleteCallCount != 0 {
		t.Fatalf("delete call count = %d, want 0", db.deleteCallCount)
	}
	if db.removeKeyCallCount != 0 {
		t.Fatalf("remove key call count = %d, want 0", db.removeKeyCallCount)
	}
	if len(hub.publishedEvents) != 0 {
		t.Fatalf("published event count = %d, want 0", len(hub.publishedEvents))
	}
}

func TestClient_handleAPIDeletedEvent_NoDatabaseDoesNotMutateMemory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	hub := &mockEventHubForDelete{}

	apiID := "test-api-memory-only"
	apiConfig := createTestAPIConfigForDeletion(apiID)
	if err := store.Add(apiConfig); err != nil {
		t.Fatalf("failed to seed in-memory config: %v", err)
	}

	client := &Client{
		logger:   logger,
		store:    store,
		db:       nil,
		eventHub: hub,
	}

	event := map[string]interface{}{
		"type": "api.deleted",
		"payload": map[string]interface{}{
			"apiId": apiID,
			"vhost": "api.example.com",
		},
		"timestamp":     "2026-01-01T00:00:00Z",
		"correlationId": "corr-delete-memory",
	}

	client.handleAPIDeletedEvent(event)

	if _, err := store.Get(apiID); err != nil {
		t.Fatalf("expected API config to remain in memory when database is unavailable: %v", err)
	}
	if len(hub.publishedEvents) != 0 {
		t.Fatalf("published event count = %d, want 0", len(hub.publishedEvents))
	}
}

func TestClient_handleAPIDeletedEvent_DeleteErrorDoesNotPublish(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newMockStorageForDeletion()
	hub := &mockEventHubForDelete{}

	apiID := "test-api-delete-error"
	apiConfig := createTestAPIConfigForDeletion(apiID)
	db.configs[apiID] = apiConfig
	db.deleteErr = errors.New("database connection failed")

	client := &Client{
		logger:   logger,
		store:    store,
		db:       db,
		eventHub: hub,
	}

	event := map[string]interface{}{
		"type": "api.deleted",
		"payload": map[string]interface{}{
			"apiId": apiID,
			"vhost": "api.example.com",
		},
		"timestamp":     "2026-01-01T00:00:00Z",
		"correlationId": "corr-delete-error",
	}

	client.handleAPIDeletedEvent(event)

	if db.deleteCallCount != 1 {
		t.Fatalf("delete call count = %d, want 1", db.deleteCallCount)
	}
	if db.removeKeyCallCount != 0 {
		t.Fatalf("remove key call count = %d, want 0", db.removeKeyCallCount)
	}
	if len(hub.publishedEvents) != 0 {
		t.Fatalf("published event count = %d, want 0", len(hub.publishedEvents))
	}
}

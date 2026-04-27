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
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// mockStorageForDeletion implements storage.Storage interface for deletion testing
type mockStorageForDeletion struct {
	configs                      map[string]*models.StoredConfig
	secrets                      map[string]*models.Secret
	subscriptions                map[string]*models.Subscription
	apiKeysByUUID                map[string]*models.APIKey
	replacedMappings             []*models.ApplicationAPIKeyMapping
	replacedAppID                string
	replacedAppUUID              string
	replacedAppName              string
	replacedAppType              string
	deleteErr                    error
	updateErr                    error
	getErr                       error
	removeKeyErr                 error
	removeSubscriptionErr        error
	replaceErr                   error
	deleteCallCount              int
	removeKeyCallCount           int
	removeSubscriptionCallCount  int
	lastSubscriptionCleanupAPIID string
	upsertAffected               *bool // nil = default (true); non-nil = use this value
	upsertErr                    error
	upsertCallCount              int
}

func newMockStorageForDeletion() *mockStorageForDeletion {
	return &mockStorageForDeletion{
		configs:       make(map[string]*models.StoredConfig),
		secrets:       make(map[string]*models.Secret),
		subscriptions: make(map[string]*models.Subscription),
		apiKeysByUUID: make(map[string]*models.APIKey),
	}
}

func (m *mockStorageForDeletion) SaveConfig(config *models.StoredConfig) error {
	m.configs[config.UUID] = config
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
	m.configs[config.UUID] = config
	return nil
}

func (m *mockStorageForDeletion) UpsertConfig(config *models.StoredConfig) (bool, error) {
	m.upsertCallCount++
	if m.upsertErr != nil {
		return false, m.upsertErr
	}
	affected := true
	if m.upsertAffected != nil {
		affected = *m.upsertAffected
	}
	if affected {
		m.configs[config.UUID] = config
	}
	return affected, nil
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

func (m *mockStorageForDeletion) GetAllConfigsByOrigin(origin models.Origin) ([]*models.StoredConfig, error) {
	var configs []*models.StoredConfig
	for _, config := range m.configs {
		if config.Origin == origin {
			configs = append(configs, config)
		}
	}
	return configs, nil
}

func (m *mockStorageForDeletion) GetConfigByKindAndHandle(kind string, handle string) (*models.StoredConfig, error) {
	for _, config := range m.configs {
		if config.Kind == kind && config.Handle == handle {
			return config, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) GetConfigByKindNameAndVersion(kind, displayName, version string) (*models.StoredConfig, error) {
	for _, config := range m.configs {
		if config.Kind == kind && config.DisplayName == displayName && config.Version == version {
			return config, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) Close() error {
	return nil
}

func (m *mockStorageForDeletion) SaveAPIKey(key *models.APIKey) error {
	return nil
}

func (m *mockStorageForDeletion) UpsertAPIKey(key *models.APIKey) error {
	return nil
}

func (m *mockStorageForDeletion) GetAPIKey(apiID, name string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}

// Subscription methods

func (m *mockStorageForDeletion) SaveSubscription(sub *models.Subscription) error {
	// Deletion tests don't depend on subscription persistence
	if m.subscriptions == nil {
		m.subscriptions = make(map[string]*models.Subscription)
	}
	m.subscriptions[sub.ID] = sub
	return nil
}

func (m *mockStorageForDeletion) GetSubscriptionByID(id, gatewayID string) (*models.Subscription, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if sub, ok := m.subscriptions[id]; ok {
		if gatewayID != "" && sub.GatewayID != gatewayID {
			return nil, storage.ErrNotFound
		}
		return sub, nil
	}
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) ListActiveSubscriptions() ([]*models.Subscription, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make([]*models.Subscription, 0)
	for _, sub := range m.subscriptions {
		if sub == nil || sub.Status != models.SubscriptionStatusActive {
			continue
		}
		result = append(result, sub)
	}
	return result, nil
}

func (m *mockStorageForDeletion) ListSubscriptionsByAPI(apiID, gatewayID string, applicationID *string, status *string) ([]*models.Subscription, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make([]*models.Subscription, 0)
	for _, sub := range m.subscriptions {
		if apiID != "" && sub.APIID != apiID {
			continue
		}
		if gatewayID != "" && sub.GatewayID != gatewayID {
			continue
		}
		if applicationID != nil && *applicationID != "" && (sub.ApplicationID == nil || *sub.ApplicationID != *applicationID) {
			continue
		}
		if status != nil && *status != "" && string(sub.Status) != *status {
			continue
		}
		result = append(result, sub)
	}
	return result, nil
}

func (m *mockStorageForDeletion) UpdateSubscription(sub *models.Subscription) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if m.subscriptions == nil {
		m.subscriptions = make(map[string]*models.Subscription)
	}
	m.subscriptions[sub.ID] = sub
	return nil
}

func (m *mockStorageForDeletion) DeleteSubscription(id, gatewayID string) error {
	// Don't touch deleteCallCount used for DeleteConfig assertions.
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if m.subscriptions == nil {
		return storage.ErrNotFound
	}
	sub, ok := m.subscriptions[id]
	if !ok {
		return storage.ErrNotFound
	}
	if gatewayID != "" && sub.GatewayID != gatewayID {
		return storage.ErrNotFound
	}
	delete(m.subscriptions, id)
	return nil
}

// Subscription Plan methods

func (m *mockStorageForDeletion) SaveSubscriptionPlan(plan *models.SubscriptionPlan) error {
	return nil
}

func (m *mockStorageForDeletion) GetSubscriptionPlanByID(id, gatewayID string) (*models.SubscriptionPlan, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) ListSubscriptionPlans(gatewayID string) ([]*models.SubscriptionPlan, error) {
	return nil, nil
}

func (m *mockStorageForDeletion) UpdateSubscriptionPlan(plan *models.SubscriptionPlan) error {
	return nil
}

func (m *mockStorageForDeletion) DeleteSubscriptionPlan(id, gatewayID string) error {
	return nil
}

func (m *mockStorageForDeletion) DeleteSubscriptionPlansNotIn(ids []string) error {
	return nil
}

func (m *mockStorageForDeletion) DeleteSubscriptionsForAPINotIn(apiID string, ids []string) error {
	m.removeSubscriptionCallCount++
	m.lastSubscriptionCleanupAPIID = apiID
	if m.removeSubscriptionErr != nil {
		return m.removeSubscriptionErr
	}
	return nil
}

func (m *mockStorageForDeletion) ListAPIKeysForArtifactsNotIn(artifactUUIDs []string, keyUUIDs []string) ([]*models.APIKey, error) {
	return nil, nil
}

func (m *mockStorageForDeletion) DeleteAPIKeysByUUIDs(uuids []string) error {
	return nil
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

func (m *mockStorageForDeletion) GetAPIKeyByUUID(uuid string) (*models.APIKey, error) {
	if key, ok := m.apiKeysByUUID[uuid]; ok {
		return key, nil
	}
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

func (m *mockStorageForDeletion) ReplaceApplicationAPIKeyMappings(application *models.StoredApplication, mappings []*models.ApplicationAPIKeyMapping) error {
	if m.replaceErr != nil {
		return m.replaceErr
	}
	if application != nil {
		m.replacedAppID = application.ApplicationID
		m.replacedAppUUID = application.ApplicationUUID
		m.replacedAppName = application.ApplicationName
		m.replacedAppType = application.ApplicationType
	}
	m.replacedMappings = append([]*models.ApplicationAPIKeyMapping(nil), mappings...)
	return nil
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

func (m *mockStorageForDeletion) GetLLMProviderTemplateByHandle(handle string) (*models.StoredLLMProviderTemplate, error) {
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) DeleteLLMProviderTemplate(id string) error {
	return nil
}

// Secret methods (not used in deletion tests but required by interface)
func (m *mockStorageForDeletion) SaveSecret(secret *models.Secret) error {
	if m.deleteErr != nil { // reuse existing error fields only if needed
		return m.deleteErr
	}
	m.secrets[secret.Handle] = secret
	return nil
}

func (m *mockStorageForDeletion) GetSecrets() ([]models.SecretMeta, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	secrets := make([]models.SecretMeta, 0, len(m.secrets))
	for handle, secret := range m.secrets {
		secrets = append(secrets, models.SecretMeta{
			Handle:      handle,
			DisplayName: secret.DisplayName,
			CreatedAt:   secret.CreatedAt,
			UpdatedAt:   secret.UpdatedAt,
		})
	}
	return secrets, nil
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

func (m *mockStorageForDeletion) GetDB() *sql.DB {
	return nil
}

// Bottom-up sync methods
func (m *mockStorageForDeletion) UpdateCPSyncStatus(uuid, cpArtifactID string, status models.CPSyncStatus, reason string) error {
	if config, ok := m.configs[uuid]; ok {
		config.CPSyncStatus = status
		config.CPSyncInfo = reason
		if cpArtifactID != "" {
			config.CPArtifactID = cpArtifactID
		}
		return nil
	}
	return storage.ErrNotFound
}

func (m *mockStorageForDeletion) GetConfigByCPArtifactID(cpArtifactID string) (*models.StoredConfig, error) {
	for _, config := range m.configs {
		if config.CPArtifactID == cpArtifactID {
			return config, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *mockStorageForDeletion) UpdateDeploymentID(uuid, deploymentID string) error {
	if config, ok := m.configs[uuid]; ok {
		config.DeploymentID = deploymentID
		return nil
	}
	return storage.ErrNotFound
}

func (m *mockStorageForDeletion) GetPendingBottomUpAPIs() ([]*models.StoredConfig, error) {
	var pending []*models.StoredConfig
	for _, config := range m.configs {
		if config.Kind == string(api.RestAPIKindRestApi) &&
			config.Origin == models.OriginGatewayAPI &&
			(config.CPSyncStatus == models.CPSyncStatusPending || config.CPSyncStatus == models.CPSyncStatusFailed) {
			pending = append(pending, config)
		}
	}
	return pending, nil
}

// Helper to create test API config for deletion tests
func createTestAPIConfigForDeletion(apiID string) *models.StoredConfig {
	// Create a complete API configuration so deletion flow can properly process it
	return &models.StoredConfig{
		UUID:         apiID,
		Handle:       apiID,
		DisplayName:  "Test API",
		Version:      "v1",
		DesiredState: models.StateDeployed,
		Origin:       models.OriginGatewayAPI,
		Kind:         "API",
		Configuration: api.RestAPI{
			ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestAPIKindRestApi,
			Metadata: api.Metadata{
				Name: apiID,
			},
			Spec: api.APIConfigData{
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
			},
		},
	}
}

func createDeletionTestClient() (*Client, *storage.ConfigStore, *mockStorageForDeletion, *mockControlPlaneEventHub) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewConfigStore()
	db := newMockStorageForDeletion()
	hub := &mockControlPlaneEventHub{}

	client := &Client{
		logger:    logger,
		store:     store,
		db:        db,
		eventHub:  hub,
		gatewayID: "test-gateway",
	}

	return client, store, db, hub
}

// TestClient_handleAPIDeletedEvent_InvalidPayload tests invalid event handling
func TestClient_handleAPIDeletedEvent_InvalidPayload(t *testing.T) {
	client, _, _, hub := createDeletionTestClient()

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
			if len(hub.publishedEvents) != 0 {
				t.Fatalf("expected no events to be published for invalid payload, got %d", len(hub.publishedEvents))
			}
		})
	}
}

// TestClient_handleAPIDeletedEvent_OrphanedCleanup tests orphaned resource cleanup
func TestClient_handleAPIDeletedEvent_OrphanedCleanup(t *testing.T) {
	client, _, db, hub := createDeletionTestClient()

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
	if db.removeSubscriptionCallCount != 1 {
		t.Errorf("Expected DeleteSubscriptionsForAPINotIn to be called for orphan cleanup, got %d", db.removeSubscriptionCallCount)
	}
	if db.lastSubscriptionCleanupAPIID != apiID {
		t.Errorf("expected subscription cleanup for API %s, got %s", apiID, db.lastSubscriptionCleanupAPIID)
	}

	if len(hub.publishedEvents) != 1 {
		t.Fatalf("expected one orphan cleanup event, got %d", len(hub.publishedEvents))
	}
	if hub.publishedEvents[0].gatewayID != "test-gateway" {
		t.Errorf("expected gatewayID test-gateway, got %s", hub.publishedEvents[0].gatewayID)
	}
	if hub.publishedEvents[0].event.EventType != eventhub.EventTypeAPI {
		t.Errorf("expected event type API, got %s", hub.publishedEvents[0].event.EventType)
	}
	if hub.publishedEvents[0].event.Action != "DELETE" {
		t.Errorf("expected action DELETE, got %s", hub.publishedEvents[0].event.Action)
	}
	if hub.publishedEvents[0].event.EntityID != apiID {
		t.Errorf("expected entity ID %s, got %s", apiID, hub.publishedEvents[0].event.EntityID)
	}
	if hub.publishedEvents[0].event.EventID != "corr-orphan" {
		t.Errorf("expected correlation ID corr-orphan, got %s", hub.publishedEvents[0].event.EventID)
	}
}

// TestClient_handleAPIDeletedEvent_FullDeletion tests complete deletion workflow
func TestClient_handleAPIDeletedEvent_FullDeletion(t *testing.T) {
	client, store, db, hub := createDeletionTestClient()

	apiID := "test-api-full-delete"
	apiConfig := createTestAPIConfigForDeletion(apiID)
	db.SaveConfig(apiConfig)
	store.Add(apiConfig)

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

	if len(hub.publishedEvents) != 1 {
		t.Fatalf("expected one delete event, got %d", len(hub.publishedEvents))
	}
	if hub.publishedEvents[0].event.EventType != eventhub.EventTypeAPI {
		t.Errorf("expected event type API, got %s", hub.publishedEvents[0].event.EventType)
	}
	if hub.publishedEvents[0].event.Action != "DELETE" {
		t.Errorf("expected action DELETE, got %s", hub.publishedEvents[0].event.Action)
	}
	if hub.publishedEvents[0].event.EntityID != apiID {
		t.Errorf("expected entity ID %s, got %s", apiID, hub.publishedEvents[0].event.EntityID)
	}

	if _, err := db.GetConfig(apiID); !storage.IsNotFoundError(err) {
		t.Errorf("expected API config to be removed from database, got %v", err)
	}
}

// TestClient_handleAPIDeletedEvent_DBOnlyConfig tests deletion when the API exists only in the database.
func TestClient_handleAPIDeletedEvent_DBOnlyConfig(t *testing.T) {
	client, _, db, hub := createDeletionTestClient()

	apiID := "test-api-db-only"
	apiConfig := createTestAPIConfigForDeletion(apiID)
	db.SaveConfig(apiConfig)

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

	if db.deleteCallCount != 1 {
		t.Errorf("expected DeleteConfig to be called once, got %d", db.deleteCallCount)
	}
	if db.removeKeyCallCount != 1 {
		t.Errorf("expected RemoveAPIKeysAPI to be called once, got %d", db.removeKeyCallCount)
	}
	if len(hub.publishedEvents) != 1 {
		t.Fatalf("expected one delete event, got %d", len(hub.publishedEvents))
	}
	if hub.publishedEvents[0].event.EntityID != apiID {
		t.Errorf("expected entity ID %s, got %s", apiID, hub.publishedEvents[0].event.EntityID)
	}
}

// TestClient_handleAPIDeletedEvent_StorageErrors tests error handling
func TestClient_handleAPIDeletedEvent_StorageErrors(t *testing.T) {
	client, store, db, hub := createDeletionTestClient()

	apiID := "test-api-error"
	apiConfig := createTestAPIConfigForDeletion(apiID)
	db.SaveConfig(apiConfig)
	store.Add(apiConfig)

	// Simulate storage errors
	db.deleteErr = errors.New("database connection failed")
	db.removeKeyErr = errors.New("failed to remove keys")

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

	if len(hub.publishedEvents) != 1 {
		t.Fatalf("expected one delete event even when DB cleanup errors occur, got %d", len(hub.publishedEvents))
	}
	if hub.publishedEvents[0].event.Action != "DELETE" {
		t.Errorf("expected action DELETE, got %s", hub.publishedEvents[0].event.Action)
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
		if config.UUID != apiID {
			t.Errorf("Expected API ID %s, got %s", apiID, config.UUID)
		}
	})

	t.Run("Returns ErrNotFound when config exists only in memory store", func(t *testing.T) {
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

		_, err := client.findAPIConfig(apiID)
		if !storage.IsNotFoundError(err) {
			t.Errorf("Expected ErrNotFound when API exists only in memory store, got: %v", err)
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

	t.Run("Returns database errors without falling back", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
		store := storage.NewConfigStore()
		db := newMockStorageForDeletion()
		db.getErr = errors.New("database unavailable")

		client := &Client{
			logger: logger,
			store:  store,
			db:     db,
		}

		_, err := client.findAPIConfig("test-api-db-error")
		if err == nil || err.Error() != "database error while fetching config: database unavailable" {
			t.Errorf("expected database error to be returned, got: %v", err)
		}
	})
}

func TestClient_handleApplicationUpdatedEvent_SkipsMissingAPIKeys(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	db := newMockStorageForDeletion()
	hub := &mockControlPlaneEventHub{}

	db.apiKeysByUUID["key-uuid-found-1"] = &models.APIKey{UUID: "key-uuid-found-1"}
	db.apiKeysByUUID["key-uuid-found-2"] = &models.APIKey{UUID: "key-uuid-found-2"}

	client := &Client{
		logger:    logger,
		db:        db,
		eventHub:  hub,
		gatewayID: "test-gateway",
	}

	event := map[string]interface{}{
		"type": "application.updated",
		"payload": map[string]interface{}{
			"applicationId":   "app-123",
			"applicationUuid": "app-uuid-123",
			"applicationName": "Shopping App",
			"applicationType": "genai",
			"mappings": []map[string]interface{}{
				{"apiKeyUuid": "key-uuid-found-1"},
				{"apiKeyUuid": "key-uuid-missing"},
				{"apiKeyUuid": "key-uuid-found-2"},
			},
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-app-update-skip-missing",
	}

	client.handleApplicationUpdatedEvent(event)

	if db.replacedAppUUID != "app-uuid-123" {
		t.Fatalf("expected mappings to be replaced for app-uuid-123, got %q", db.replacedAppUUID)
	}
	if db.replacedAppID != "app-123" {
		t.Fatalf("expected mappings to be replaced for app id app-123, got %q", db.replacedAppID)
	}
	if db.replacedAppName != "Shopping App" {
		t.Fatalf("expected mappings to be replaced for app name Shopping App, got %q", db.replacedAppName)
	}
	if db.replacedAppType != "genai" {
		t.Fatalf("expected mappings to be replaced for app type genai, got %q", db.replacedAppType)
	}

	if len(db.replacedMappings) != 2 {
		t.Fatalf("expected 2 resolved mappings, got %d", len(db.replacedMappings))
	}

	if db.replacedMappings[0].APIKeyID != "key-uuid-found-1" {
		t.Errorf("expected first mapping key id key-uuid-found-1, got %q", db.replacedMappings[0].APIKeyID)
	}
	if db.replacedMappings[0].ApplicationUUID != "app-uuid-123" {
		t.Errorf("expected first mapping application uuid app-uuid-123, got %q", db.replacedMappings[0].ApplicationUUID)
	}
	if db.replacedMappings[1].APIKeyID != "key-uuid-found-2" {
		t.Errorf("expected second mapping key id key-uuid-found-2, got %q", db.replacedMappings[1].APIKeyID)
	}
	if len(hub.publishedEvents) != 1 {
		t.Fatalf("expected one application event, got %d", len(hub.publishedEvents))
	}
	if hub.publishedEvents[0].event.EventType != eventhub.EventTypeApplication {
		t.Errorf("expected event type APPLICATION, got %s", hub.publishedEvents[0].event.EventType)
	}
	if hub.publishedEvents[0].event.Action != "UPDATE" {
		t.Errorf("expected action UPDATE, got %s", hub.publishedEvents[0].event.Action)
	}
	if hub.publishedEvents[0].event.EntityID != "app-uuid-123" {
		t.Errorf("expected entity ID app-uuid-123, got %s", hub.publishedEvents[0].event.EntityID)
	}
	if hub.publishedEvents[0].event.EventID != "corr-app-update-skip-missing" {
		t.Errorf("expected correlation ID corr-app-update-skip-missing, got %s", hub.publishedEvents[0].event.EventID)
	}
}

func TestClient_handleApplicationUpdatedEvent_ContinuesOnInvalidMappingEntries(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	db := newMockStorageForDeletion()
	hub := &mockControlPlaneEventHub{}

	db.apiKeysByUUID["key-uuid-found"] = &models.APIKey{UUID: "key-uuid-found"}

	client := &Client{
		logger:    logger,
		db:        db,
		eventHub:  hub,
		gatewayID: "test-gateway",
	}

	event := map[string]interface{}{
		"type": "application.updated",
		"payload": map[string]interface{}{
			"applicationId":   "app-456",
			"applicationUuid": "app-uuid-456",
			"applicationName": "Weather App",
			"applicationType": "genai",
			"mappings": []map[string]interface{}{
				{"apiKeyUuid": ""},
				{"apiKeyUuid": "key-uuid-found"},
			},
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-app-update-skip-invalid",
	}

	client.handleApplicationUpdatedEvent(event)

	if db.replacedAppUUID != "app-uuid-456" {
		t.Fatalf("expected mappings to be replaced for app-uuid-456, got %q", db.replacedAppUUID)
	}
	if db.replacedAppID != "app-456" {
		t.Fatalf("expected mappings to be replaced for app id app-456, got %q", db.replacedAppID)
	}
	if db.replacedAppName != "Weather App" {
		t.Fatalf("expected mappings to be replaced for app name Weather App, got %q", db.replacedAppName)
	}
	if db.replacedAppType != "genai" {
		t.Fatalf("expected mappings to be replaced for app type genai, got %q", db.replacedAppType)
	}

	if len(db.replacedMappings) != 1 {
		t.Fatalf("expected 1 resolved mapping, got %d", len(db.replacedMappings))
	}

	if db.replacedMappings[0].APIKeyID != "key-uuid-found" {
		t.Errorf("expected resolved mapping key id key-uuid-found, got %q", db.replacedMappings[0].APIKeyID)
	}
	if db.replacedMappings[0].ApplicationUUID != "app-uuid-456" {
		t.Errorf("expected resolved mapping application uuid app-uuid-456, got %q", db.replacedMappings[0].ApplicationUUID)
	}
	if len(hub.publishedEvents) != 1 {
		t.Fatalf("expected one application event, got %d", len(hub.publishedEvents))
	}
	if hub.publishedEvents[0].event.EventType != eventhub.EventTypeApplication {
		t.Errorf("expected event type APPLICATION, got %s", hub.publishedEvents[0].event.EventType)
	}
	if hub.publishedEvents[0].event.Action != "UPDATE" {
		t.Errorf("expected action UPDATE, got %s", hub.publishedEvents[0].event.Action)
	}
	if hub.publishedEvents[0].event.EntityID != "app-uuid-456" {
		t.Errorf("expected entity ID app-uuid-456, got %s", hub.publishedEvents[0].event.EntityID)
	}
	if hub.publishedEvents[0].event.EventID != "corr-app-update-skip-invalid" {
		t.Errorf("expected correlation ID corr-app-update-skip-invalid, got %s", hub.publishedEvents[0].event.EventID)
	}
}

func TestClient_handleSubscriptionCreatedEvent_PublishesReplicaSyncEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	db := newMockStorageForDeletion()
	hub := &mockControlPlaneEventHub{}

	client := &Client{
		logger:    logger,
		db:        db,
		eventHub:  hub,
		gatewayID: "test-gateway",
	}

	event := map[string]interface{}{
		"type": "subscription.created",
		"payload": map[string]interface{}{
			"subscriptionId":     "sub-123",
			"apiId":              "api-123",
			"subscriptionToken":  "token-123",
			"status":             "ACTIVE",
			"applicationId":      "app-123",
			"subscriptionPlanId": "plan-123",
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-sub-created",
	}

	client.handleSubscriptionCreatedEvent(event)

	sub, err := db.GetSubscriptionByID("sub-123", "")
	if err != nil {
		t.Fatalf("expected subscription to be stored, got %v", err)
	}
	if sub.APIID != "api-123" {
		t.Errorf("expected api id api-123, got %s", sub.APIID)
	}
	if len(hub.publishedEvents) != 1 {
		t.Fatalf("expected one subscription event, got %d", len(hub.publishedEvents))
	}
	if hub.publishedEvents[0].event.EventType != eventhub.EventTypeSubscription {
		t.Errorf("expected event type SUBSCRIPTION, got %s", hub.publishedEvents[0].event.EventType)
	}
	if hub.publishedEvents[0].event.Action != "CREATE" {
		t.Errorf("expected action CREATE, got %s", hub.publishedEvents[0].event.Action)
	}
	if hub.publishedEvents[0].event.EntityID != "sub-123" {
		t.Errorf("expected entity ID sub-123, got %s", hub.publishedEvents[0].event.EntityID)
	}
	if hub.publishedEvents[0].event.EventID != "corr-sub-created" {
		t.Errorf("expected correlation ID corr-sub-created, got %s", hub.publishedEvents[0].event.EventID)
	}
}

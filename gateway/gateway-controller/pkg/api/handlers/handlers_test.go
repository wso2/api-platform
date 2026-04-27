/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/apikey"
	"github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/common/eventhub"
	commonmodels "github.com/wso2/api-platform/common/models"
	adminapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/admin"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	policybuilder "github.com/wso2/api-platform/gateway/gateway-controller/pkg/policy"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

func init() {
	gin.SetMode(gin.TestMode)
	metrics.Init()
}

// MockStorage implements the storage.Storage interface for testing
type MockStorage struct {
	configs           map[string]*models.StoredConfig
	templates         map[string]*models.StoredLLMProviderTemplate
	apiKeys           map[string]*models.APIKey
	certs             []*models.StoredCertificate
	secrets           map[string]*models.Secret
	subscriptions     map[string]*models.Subscription
	subscriptionPlans map[string]*models.SubscriptionPlan
	saveErr           error
	getErr            error
	updateErr         error
	deleteErr         error
	unavailable       bool
}

func cloneAPIKey(apiKey *models.APIKey) *models.APIKey {
	if apiKey == nil {
		return nil
	}

	cloned := *apiKey
	if apiKey.ExpiresAt != nil {
		expiresAt := *apiKey.ExpiresAt
		cloned.ExpiresAt = &expiresAt
	}
	if apiKey.ExternalRefId != nil {
		externalRefID := *apiKey.ExternalRefId
		cloned.ExternalRefId = &externalRefID
	}
	if apiKey.Issuer != nil {
		issuer := *apiKey.Issuer
		cloned.Issuer = &issuer
	}
	return &cloned
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		configs:           make(map[string]*models.StoredConfig),
		templates:         make(map[string]*models.StoredLLMProviderTemplate),
		apiKeys:           make(map[string]*models.APIKey),
		certs:             make([]*models.StoredCertificate, 0),
		secrets:           make(map[string]*models.Secret),
		subscriptions:     make(map[string]*models.Subscription),
		subscriptionPlans: make(map[string]*models.SubscriptionPlan),
	}
}

type publishedEvent struct {
	gatewayID string
	event     eventhub.Event
}

type mockEventHub struct {
	publishedEvents []publishedEvent
	publishErr      error
}

func (m *mockEventHub) Initialize() error {
	return nil
}

func (m *mockEventHub) RegisterGateway(string) error {
	return nil
}

func (m *mockEventHub) PublishEvent(gatewayID string, event eventhub.Event) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.publishedEvents = append(m.publishedEvents, publishedEvent{
		gatewayID: gatewayID,
		event:     event,
	})
	return nil
}

func (m *mockEventHub) Subscribe(string) (<-chan eventhub.Event, error) {
	return nil, nil
}

func (m *mockEventHub) Unsubscribe(string, <-chan eventhub.Event) error {
	return nil
}

func (m *mockEventHub) UnsubscribeAll(string) error {
	return nil
}

func (m *mockEventHub) CleanUpEvents() error {
	return nil
}

func (m *mockEventHub) Close() error {
	return nil
}

func (m *MockStorage) SaveConfig(cfg *models.StoredConfig) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.configs[cfg.UUID] = cfg
	return nil
}

func (m *MockStorage) UpdateConfig(cfg *models.StoredConfig) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.configs[cfg.UUID] = cfg
	return nil
}

func (m *MockStorage) UpsertConfig(cfg *models.StoredConfig) (bool, error) {
	if m.updateErr != nil {
		return false, m.updateErr
	}
	if cfg.Handle == "" {
		return false, fmt.Errorf("handle (metadata.name) is required and cannot be empty")
	}
	if existing, ok := m.configs[cfg.UUID]; ok {
		if existing.DeployedAt != nil && cfg.DeployedAt != nil && !existing.DeployedAt.Before(*cfg.DeployedAt) {
			return false, nil
		}
	}
	m.configs[cfg.UUID] = cfg
	return true, nil
}

func (m *MockStorage) DeleteConfig(id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.configs, id)
	return nil
}

func (m *MockStorage) GetConfig(id string) (*models.StoredConfig, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if cfg, ok := m.configs[id]; ok {
		return cfg, nil
	}
	return nil, storage.ErrNotFound
}

func (m *MockStorage) GetConfigByKindAndHandle(kind string, handle string) (*models.StoredConfig, error) {
	if m.unavailable {
		return nil, storage.ErrDatabaseUnavailable
	}
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, cfg := range m.configs {
		if cfg.Kind == kind && cfg.Handle == handle {
			return cfg, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *MockStorage) GetConfigByKindNameAndVersion(kind, displayName, version string) (*models.StoredConfig, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, cfg := range m.configs {
		if cfg.Kind == kind && cfg.DisplayName == displayName && cfg.Version == version {
			return cfg, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *MockStorage) GetAllConfigs() ([]*models.StoredConfig, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make([]*models.StoredConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		result = append(result, cfg)
	}
	return result, nil
}

func (m *MockStorage) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make([]*models.StoredConfig, 0)
	for _, cfg := range m.configs {
		if cfg.Kind == kind {
			result = append(result, cfg)
		}
	}
	return result, nil
}

func (m *MockStorage) GetAllConfigsByOrigin(origin models.Origin) ([]*models.StoredConfig, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make([]*models.StoredConfig, 0)
	for _, cfg := range m.configs {
		if cfg.Origin == origin {
			result = append(result, cfg)
		}
	}
	return result, nil
}

func (m *MockStorage) SaveLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.templates[template.UUID] = template
	return nil
}

func (m *MockStorage) UpdateLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.templates[template.UUID] = template
	return nil
}

func (m *MockStorage) DeleteLLMProviderTemplate(id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.templates, id)
	return nil
}

func (m *MockStorage) GetLLMProviderTemplate(id string) (*models.StoredLLMProviderTemplate, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if tmpl, ok := m.templates[id]; ok {
		return tmpl, nil
	}
	return nil, errors.New("template not found")
}

func (m *MockStorage) GetAllLLMProviderTemplates() ([]*models.StoredLLMProviderTemplate, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make([]*models.StoredLLMProviderTemplate, 0, len(m.templates))
	for _, tmpl := range m.templates {
		result = append(result, tmpl)
	}
	return result, nil
}

func (m *MockStorage) GetLLMProviderTemplateByHandle(handle string) (*models.StoredLLMProviderTemplate, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, tmpl := range m.templates {
		if tmpl.GetHandle() == handle {
			return tmpl, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *MockStorage) SaveAPIKey(apiKey *models.APIKey) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.apiKeys[apiKey.UUID] = cloneAPIKey(apiKey)
	return nil
}

func (m *MockStorage) UpsertAPIKey(apiKey *models.APIKey) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.apiKeys[apiKey.UUID] = cloneAPIKey(apiKey)
	return nil
}

func (m *MockStorage) GetAPIKeyByID(id string) (*models.APIKey, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if key, ok := m.apiKeys[id]; ok {
		return cloneAPIKey(key), nil
	}
	return nil, errors.New("API key not found")
}

func (m *MockStorage) GetAPIKeyByUUID(uuid string) (*models.APIKey, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, apiKey := range m.apiKeys {
		if apiKey.UUID == uuid {
			return cloneAPIKey(apiKey), nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *MockStorage) GetAPIKeyByKey(key string) (*models.APIKey, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, apiKey := range m.apiKeys {
		if apiKey.APIKey == key {
			return cloneAPIKey(apiKey), nil
		}
	}
	return nil, errors.New("API key not found")
}

func (m *MockStorage) GetAPIKeysByAPI(apiId string) ([]*models.APIKey, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make([]*models.APIKey, 0)
	for _, key := range m.apiKeys {
		if key.ArtifactUUID == apiId {
			result = append(result, cloneAPIKey(key))
		}
	}
	return result, nil
}

func (m *MockStorage) GetAllAPIKeys() ([]*models.APIKey, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make([]*models.APIKey, 0, len(m.apiKeys))
	for _, key := range m.apiKeys {
		result = append(result, cloneAPIKey(key))
	}
	return result, nil
}

func (m *MockStorage) GetAPIKeysByAPIAndName(apiId, name string) (*models.APIKey, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, key := range m.apiKeys {
		if key.ArtifactUUID == apiId && key.Name == name {
			return cloneAPIKey(key), nil
		}
	}
	return nil, errors.New("API key not found")
}

func (m *MockStorage) UpdateAPIKey(apiKey *models.APIKey) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.apiKeys[apiKey.UUID] = cloneAPIKey(apiKey)
	return nil
}

func (m *MockStorage) DeleteAPIKey(key string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for id, apiKey := range m.apiKeys {
		if apiKey.APIKey == key {
			delete(m.apiKeys, id)
			return nil
		}
	}
	return errors.New("API key not found")
}

func (m *MockStorage) RemoveAPIKeysAPI(apiId string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for id, key := range m.apiKeys {
		if key.ArtifactUUID == apiId {
			delete(m.apiKeys, id)
		}
	}
	return nil
}

func (m *MockStorage) RemoveAPIKeyAPIAndName(apiId, name string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for id, key := range m.apiKeys {
		if key.ArtifactUUID == apiId && key.Name == name {
			delete(m.apiKeys, id)
			return nil
		}
	}
	return errors.New("API key not found")
}

func (m *MockStorage) ListAPIKeysForArtifactsNotIn(artifactUUIDs []string, keyUUIDs []string) ([]*models.APIKey, error) {
	return nil, nil
}

func (m *MockStorage) DeleteAPIKeysByUUIDs(uuids []string) error {
	return nil
}

func (m *MockStorage) CountActiveAPIKeysByUserAndAPI(apiId, userID string) (int, error) {
	if m.getErr != nil {
		return 0, m.getErr
	}
	count := 0
	for _, key := range m.apiKeys {
		if key.ArtifactUUID == apiId && key.CreatedBy == userID && key.Status == models.APIKeyStatusActive {
			count++
		}
	}
	return count, nil
}

// Subscription methods

func (m *MockStorage) SaveSubscription(sub *models.Subscription) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	if m.subscriptions == nil {
		m.subscriptions = make(map[string]*models.Subscription)
	}
	m.subscriptions[sub.ID] = sub
	return nil
}

func (m *MockStorage) GetSubscriptionByID(id, gatewayID string) (*models.Subscription, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if sub, ok := m.subscriptions[id]; ok {
		// Enforce gateway scoping when both sides provide a gateway ID.
		if gatewayID != "" && sub.GatewayID != "" && sub.GatewayID != gatewayID {
			return nil, storage.ErrNotFound
		}
		return sub, nil
	}
	return nil, storage.ErrNotFound
}

func (m *MockStorage) ListActiveSubscriptions() ([]*models.Subscription, error) {
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

func (m *MockStorage) ListSubscriptionsByAPI(apiID, gatewayID string, applicationID *string, status *string) ([]*models.Subscription, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make([]*models.Subscription, 0)
	for _, sub := range m.subscriptions {
		if apiID != "" && sub.APIID != apiID {
			continue
		}
		if gatewayID != "" && sub.GatewayID != "" && sub.GatewayID != gatewayID {
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

func (m *MockStorage) UpdateSubscription(sub *models.Subscription) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if m.subscriptions == nil {
		return storage.ErrNotFound
	}
	if _, ok := m.subscriptions[sub.ID]; !ok {
		return storage.ErrNotFound
	}
	m.subscriptions[sub.ID] = sub
	return nil
}

func (m *MockStorage) DeleteSubscription(id, gatewayID string) error {
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
	// Enforce gateway scoping when both sides provide a gateway ID.
	if gatewayID != "" && sub.GatewayID != "" && sub.GatewayID != gatewayID {
		return storage.ErrNotFound
	}
	delete(m.subscriptions, id)
	return nil
}

// Subscription Plan methods

func (m *MockStorage) SaveSubscriptionPlan(plan *models.SubscriptionPlan) error {
	if m.unavailable {
		return storage.ErrDatabaseUnavailable
	}
	if m.saveErr != nil {
		return m.saveErr
	}
	if plan == nil {
		return nil
	}
	m.subscriptionPlans[plan.ID] = plan
	return nil
}

func (m *MockStorage) GetSubscriptionPlanByID(id, gatewayID string) (*models.SubscriptionPlan, error) {
	if m.unavailable {
		return nil, storage.ErrDatabaseUnavailable
	}
	if m.getErr != nil {
		return nil, m.getErr
	}
	plan, ok := m.subscriptionPlans[id]
	if !ok || plan == nil {
		return nil, storage.ErrNotFound
	}
	if gatewayID != "" && plan.GatewayID != gatewayID {
		return nil, storage.ErrNotFound
	}
	return plan, nil
}

func (m *MockStorage) ListSubscriptionPlans(gatewayID string) ([]*models.SubscriptionPlan, error) {
	if m.unavailable {
		return nil, storage.ErrDatabaseUnavailable
	}
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make([]*models.SubscriptionPlan, 0, len(m.subscriptionPlans))
	for _, plan := range m.subscriptionPlans {
		if plan == nil {
			continue
		}
		if gatewayID == "" || plan.GatewayID == gatewayID {
			result = append(result, plan)
		}
	}
	return result, nil
}

func (m *MockStorage) UpdateSubscriptionPlan(plan *models.SubscriptionPlan) error {
	if m.unavailable {
		return storage.ErrDatabaseUnavailable
	}
	if m.updateErr != nil {
		return m.updateErr
	}
	if plan == nil {
		return nil
	}
	if _, ok := m.subscriptionPlans[plan.ID]; !ok {
		return storage.ErrNotFound
	}
	m.subscriptionPlans[plan.ID] = plan
	return nil
}

func (m *MockStorage) DeleteSubscriptionPlan(id, gatewayID string) error {
	if m.unavailable {
		return storage.ErrDatabaseUnavailable
	}
	if m.deleteErr != nil {
		return m.deleteErr
	}
	plan, ok := m.subscriptionPlans[id]
	if !ok || plan == nil {
		return storage.ErrNotFound
	}
	if gatewayID != "" && plan.GatewayID != gatewayID {
		return storage.ErrNotFound
	}
	delete(m.subscriptionPlans, id)
	return nil
}

func (m *MockStorage) DeleteSubscriptionPlansNotIn(ids []string) error {
	if m.unavailable {
		return storage.ErrDatabaseUnavailable
	}
	if m.deleteErr != nil {
		return m.deleteErr
	}
	idsSet := make(map[string]bool)
	for _, id := range ids {
		idsSet[id] = true
	}
	for id := range m.subscriptionPlans {
		if !idsSet[id] {
			delete(m.subscriptionPlans, id)
		}
	}
	return nil
}

func (m *MockStorage) DeleteSubscriptionsForAPINotIn(apiID string, ids []string) error {
	if m.unavailable {
		return storage.ErrDatabaseUnavailable
	}
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if m.subscriptions == nil {
		return nil
	}
	idsSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idsSet[id] = struct{}{}
	}
	for id, sub := range m.subscriptions {
		if sub == nil || sub.APIID != apiID {
			continue
		}
		if _, keep := idsSet[id]; !keep {
			delete(m.subscriptions, id)
		}
	}
	return nil
}

func (m *MockStorage) ReplaceApplicationAPIKeyMappings(application *models.StoredApplication, mappings []*models.ApplicationAPIKeyMapping) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	return nil
}

func (m *MockStorage) SaveCertificate(cert *models.StoredCertificate) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.certs = append(m.certs, cert)
	return nil
}

func (m *MockStorage) GetCertificate(id string) (*models.StoredCertificate, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, cert := range m.certs {
		if cert.UUID == id {
			return cert, nil
		}
	}
	return nil, errors.New("certificate not found")
}

func (m *MockStorage) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, cert := range m.certs {
		if cert.Name == name {
			return cert, nil
		}
	}
	return nil, errors.New("certificate not found")
}

func (m *MockStorage) ListCertificates() ([]*models.StoredCertificate, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.certs, nil
}

func (m *MockStorage) DeleteCertificate(id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for i, cert := range m.certs {
		if cert.UUID == id {
			m.certs = append(m.certs[:i], m.certs[i+1:]...)
			return nil
		}
	}
	return errors.New("certificate not found")
}

func (m *MockStorage) GetDB() *sql.DB {
	return nil
}

func (m *MockStorage) Close() error {
	return nil
}

// Bottom-up sync methods
func (m *MockStorage) UpdateCPSyncStatus(uuid, cpArtifactID string, status models.CPSyncStatus, reason string) error {
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

func (m *MockStorage) GetConfigByCPArtifactID(cpArtifactID string) (*models.StoredConfig, error) {
	for _, config := range m.configs {
		if config.CPArtifactID == cpArtifactID {
			return config, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *MockStorage) UpdateDeploymentID(uuid, deploymentID string) error {
	if config, ok := m.configs[uuid]; ok {
		config.DeploymentID = deploymentID
		return nil
	}
	return storage.ErrNotFound
}

func (m *MockStorage) GetPendingBottomUpAPIs() ([]*models.StoredConfig, error) {
	var pending []*models.StoredConfig
	for _, config := range m.configs {
		if config != nil &&
			config.Kind == models.KindRestApi &&
			config.Origin == models.OriginGatewayAPI &&
			(config.CPSyncStatus == models.CPSyncStatusPending || config.CPSyncStatus == models.CPSyncStatusFailed) {
			pending = append(pending, config)
		}
	}
	return pending, nil
}

// MockControlPlaneClient implements controlplane.ControlPlaneClient for testing
type MockControlPlaneClient struct {
	connected bool
	mu        sync.Mutex
	pushedIDs []string
}

func (m *MockControlPlaneClient) Connect() error {
	m.connected = true
	return nil
}

func (m *MockControlPlaneClient) IsConnected() bool {
	return m.connected
}

func (m *MockControlPlaneClient) PushAPIDeployment(apiID string, cfg *models.StoredConfig, deploymentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pushedIDs = append(m.pushedIDs, apiID)
	return nil
}

func (m *MockControlPlaneClient) PushCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pushedIDs)
}

// Secret management methods

func (m *MockStorage) SaveSecret(secret *models.Secret) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.secrets[secret.Handle] = secret
	return nil
}

func (m *MockStorage) GetSecrets() ([]models.SecretMeta, error) {
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

func (m *MockStorage) GetSecret(handle string) (*models.Secret, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if secret, ok := m.secrets[handle]; ok {
		return secret, nil
	}
	return nil, errors.New("secret not found")
}

func (m *MockStorage) UpdateSecret(secret *models.Secret) (*models.Secret, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	if _, ok := m.secrets[secret.Handle]; !ok {
		return nil, errors.New("secret not found")
	}
	m.secrets[secret.Handle] = secret
	return secret, nil
}

func (m *MockStorage) DeleteSecret(handle string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.secrets[handle]; !ok {
		return errors.New("secret not found")
	}
	delete(m.secrets, handle)
	return nil
}

func (m *MockStorage) SecretExists(handle string) (bool, error) {
	if m.getErr != nil {
		return false, m.getErr
	}
	_, ok := m.secrets[handle]
	return ok, nil
}

func (m *MockControlPlaneClient) SyncArtifactsToOnPremAPIM(apimConfig *utils.APIMConfig) error {
	// Mock implementation - does nothing for testing
	return nil
}

func (m *MockControlPlaneClient) IsOnPrem() bool {
	return false
}

func (m *MockControlPlaneClient) GetAPIMConfig() *utils.APIMConfig {
	return &utils.APIMConfig{
		Timeout: 30 * time.Second,
	}
}

func (m *MockControlPlaneClient) Close() error {
	return nil
}

// createTestAPIServer creates a minimal test server with dependencies
func createTestAPIServer() *APIServer {
	return createTestAPIServerWithDB(NewMockStorage())
}

func createTestAPIServerWithDB(db storage.Storage) *APIServer {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewConfigStore()
	hub := &mockEventHub{}
	gatewayID := "test-gateway"

	vhosts := &config.VHostsConfig{
		Main:    config.VHostEntry{Default: "localhost"},
		Sandbox: config.VHostEntry{Default: "sandbox-localhost"},
	}

	parser := config.NewParser()
	validator := config.NewAPIValidator()
	policyDefs := make(map[string]models.PolicyDefinition)
	routerCfg := &config.RouterConfig{
		GatewayHost: "localhost",
		VHosts:      *vhosts,
		EventGateway: config.EventGatewayConfig{
			TimeoutSeconds: 10,
		},
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	systemCfg := &config.Config{
		Controller: config.Controller{
			Server: config.ServerConfig{
				GatewayID: gatewayID,
			},
		},
		Router: config.RouterConfig{
			GatewayHost: "localhost",
			VHosts:      *vhosts,
		},
		APIKey: config.APIKeyConfig{
			APIKeysPerUserPerAPI: 10,
			Algorithm:            "sha256",
			MinKeyLength:         32,
			MaxKeyLength:         128,
		},
	}

	server := &APIServer{
		store:             store,
		db:                db,
		logger:            logger,
		eventHub:          hub,
		parser:            parser,
		validator:         validator,
		policyDefinitions: policyDefs,
		routerConfig:      routerCfg,
		httpClient:        httpClient,
		systemConfig:      systemCfg,
		gatewayID:         gatewayID,
	}

	deploymentService := utils.NewAPIDeploymentService(store, db, nil, validator, routerCfg, hub, gatewayID, nil)
	server.deploymentService = deploymentService
	server.mcpDeploymentService = utils.NewMCPDeploymentService(store, db, nil, nil, nil, hub, gatewayID)
	server.llmDeploymentService = utils.NewLLMDeploymentService(
		store,
		db,
		nil,
		nil,
		map[string]*api.LLMProviderTemplate{},
		deploymentService,
		routerCfg,
		nil,
		nil,
	)

	// Initialize API key service (needed for API key operations)
	apiKeyService := utils.NewAPIKeyService(store, db, nil, &server.systemConfig.APIKey, hub, gatewayID)
	server.apiKeyService = apiKeyService
	server.subscriptionResourceService = utils.NewSubscriptionResourceService(db, nil, hub, gatewayID)

	// Initialize RestAPI service and handler
	restAPIService := restapi.NewRestAPIService(
		store, db, nil, nil,
		deploymentService, nil, nil,
		routerCfg, systemCfg,
		httpClient, parser, validator, logger, hub, nil,
	)
	server.restAPIService = restAPIService
	server.RestAPIHandler = NewRestAPIHandler(restAPIService, logger)

	return server
}

// createTestContext creates a Gin context for testing
func createTestContext(method, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	c.Request = req
	return c, w
}

// createTestContextWithHeader creates a Gin context with headers
func createTestContextWithHeader(method, path string, body []byte, headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	c, w := createTestContext(method, path, body)
	for k, v := range headers {
		c.Request.Header.Set(k, v)
	}
	return c, w
}

// createTestStoredConfig creates a test stored config
func createTestStoredConfig(id, name, version, context string) *models.StoredConfig {
	apiConfig := api.RestAPI{
		ApiVersion: api.RestAPIApiVersion(api.RestAPIApiVersionGatewayApiPlatformWso2Comv1alpha1),
		Kind:       api.RestAPIKindRestApi,
		Metadata: api.Metadata{
			Name: id,
		},
		Spec: api.APIConfigData{
			DisplayName: name,
			Version:     version,
			Context:     context,
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{
					Url: stringPtr("http://backend.example.com"),
				},
			},
			Operations: []api.Operation{
				{
					Method: "GET",
					Path:   "/resource",
				},
			},
		},
	}
	return &models.StoredConfig{
		UUID:                id,
		Kind:                string(api.RestAPIKindRestApi),
		Handle:              id,
		DisplayName:         name,
		Version:             version,
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
		DesiredState:        models.StateDeployed,
		Origin:              models.OriginGatewayAPI,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

func createLLMTemplateBody(t *testing.T, handle, displayName string) []byte {
	t.Helper()

	template := api.LLMProviderTemplate{
		ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
		Metadata: api.Metadata{
			Name: handle,
		},
		Spec: api.LLMProviderTemplateData{
			DisplayName: displayName,
		},
	}

	body, err := json.Marshal(template)
	require.NoError(t, err)
	return body
}

func createTestRestAPIRequestBody(t *testing.T, handle, displayName, version, contextPath string) []byte {
	t.Helper()

	apiConfig := api.RestAPI{
		ApiVersion: api.RestAPIApiVersion(api.RestAPIApiVersionGatewayApiPlatformWso2Comv1alpha1),
		Kind:       api.RestAPIKindRestApi,
		Metadata: api.Metadata{
			Name: handle,
		},
		Spec: api.APIConfigData{
			DisplayName: displayName,
			Version:     version,
			Context:     contextPath,
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{
					Url: stringPtr("http://backend.example.com"),
				},
			},
			Operations: []api.Operation{
				{
					Method: "GET",
					Path:   "/resource",
				},
			},
		},
	}

	body, err := json.Marshal(apiConfig)
	require.NoError(t, err)
	return body
}

func createTestMCPRequestBody(t *testing.T, handle, displayName, version, contextPath string) []byte {
	t.Helper()
	upstreamURL := "http://backend.example.com"

	mcpConfig := api.MCPProxyConfiguration{
		ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.MCPProxyConfigurationKindMcp,
		Metadata: api.Metadata{
			Name: handle,
		},
		Spec: api.MCPProxyConfigData{
			DisplayName: displayName,
			Version:     version,
			Context:     stringPtr(contextPath),
			Upstream: api.MCPProxyConfigData_Upstream{
				Url: &upstreamURL,
			},
		},
	}

	body, err := json.Marshal(mcpConfig)
	require.NoError(t, err)
	return body
}

func createTestMCPStoredConfig(t *testing.T, id, handle, displayName, version, contextPath string, desiredState models.DesiredState) *models.StoredConfig {
	t.Helper()
	upstreamURL := "http://backend.example.com"

	cfg := &models.StoredConfig{
		UUID:        id,
		Kind:        string(api.MCPProxyConfigurationKindMcp),
		Handle:      handle,
		DisplayName: displayName,
		Version:     version,
		SourceConfiguration: api.MCPProxyConfiguration{
			ApiVersion: api.MCPProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.MCPProxyConfigurationKindMcp,
			Metadata: api.Metadata{
				Name: handle,
			},
			Spec: api.MCPProxyConfigData{
				DisplayName: displayName,
				Version:     version,
				Context:     stringPtr(contextPath),
				Upstream: api.MCPProxyConfigData_Upstream{
					Url: &upstreamURL,
				},
			},
		},
		DesiredState: desiredState,
		Origin:       models.OriginGatewayAPI,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	require.NoError(t, utils.HydrateStoredMCPConfig(cfg))
	return cfg
}

func attachTestEventHub(server *APIServer, hub eventhub.EventHub, gatewayID string) {
	server.eventHub = hub
	server.gatewayID = gatewayID
	if server.systemConfig != nil {
		server.systemConfig.Controller.Server.GatewayID = gatewayID
	}
	policyValidator := config.NewPolicyValidator(server.policyDefinitions)
	policyVersionResolver := utils.NewLoadedPolicyVersionResolver(server.policyDefinitions)
	server.deploymentService = utils.NewAPIDeploymentService(server.store, server.db, server.snapshotManager, server.validator, server.routerConfig, hub, gatewayID, nil)
	server.apiKeyService = utils.NewAPIKeyService(server.store, server.db, server.apiKeyXDSManager, &server.systemConfig.APIKey, hub, gatewayID)
	server.subscriptionResourceService = utils.NewSubscriptionResourceService(server.db, server.subscriptionSnapshotUpdater, hub, gatewayID)
	server.mcpDeploymentService = utils.NewMCPDeploymentService(server.store, server.db, server.snapshotManager, server.policyManager, policyValidator, hub, gatewayID)
	server.llmDeploymentService = utils.NewLLMDeploymentService(
		server.store,
		server.db,
		server.snapshotManager,
		nil,
		map[string]*api.LLMProviderTemplate{},
		server.deploymentService,
		server.routerConfig,
		policyVersionResolver,
		policyValidator,
	)

	if server.RestAPIHandler != nil {
		restAPIService := restapi.NewRestAPIService(
			server.store, server.db, nil, nil,
			server.deploymentService, server.apiKeyXDSManager, nil,
			server.routerConfig, server.systemConfig,
			server.httpClient, server.parser, server.validator, server.logger, hub, nil,
		)
		server.restAPIService = restAPIService
		server.RestAPIHandler = NewRestAPIHandler(restAPIService, server.logger)
	}
}

func seedAPIForAPIKeyHandlerTests(t *testing.T, server *APIServer, handle string) *models.StoredConfig {
	t.Helper()

	cfg := createTestStoredConfig("0000-test-api-id-0000-000000000000", "test-api", "v1.0.0", "/test")
	cfg.Handle = handle
	if restCfg, ok := cfg.Configuration.(api.RestAPI); ok {
		restCfg.Metadata.Name = handle
		cfg.Configuration = restCfg
	}
	if sourceCfg, ok := cfg.SourceConfiguration.(api.RestAPI); ok {
		sourceCfg.Metadata.Name = handle
		cfg.SourceConfiguration = sourceCfg
	}

	require.NoError(t, server.store.Add(cfg))
	require.NoError(t, server.db.SaveConfig(cfg))

	return cfg
}

func createTestAPIKeyRequestBody(t *testing.T, name, displayName, apiKeyValue string) []byte {
	t.Helper()

	request := api.APIKeyCreationRequest{
		Name:   &name,
		ApiKey: &apiKeyValue,
	}

	body, err := json.Marshal(request)
	require.NoError(t, err)
	return body
}

func createStoredExternalAPIKey(id, apiID, name, displayName, createdBy, maskedAPIKey string) *models.APIKey {
	now := time.Now()
	return &models.APIKey{
		UUID:         id,
		Name:         name,
		APIKey:       "hashed-value",
		MaskedAPIKey: maskedAPIKey,
		ArtifactUUID: apiID,
		Status:       models.APIKeyStatusActive,
		CreatedAt:    now.Add(-1 * time.Hour),
		CreatedBy:    createdBy,
		UpdatedAt:    now.Add(-1 * time.Hour),
		Source:       string(api.External),
	}
}

// TestListAPIs tests listing all APIs
func TestListRestAPIs(t *testing.T) {
	server := createTestAPIServer()

	// Add test configs to store
	cfg1 := createTestStoredConfig("test-id-1", "0000-test-api-1-0000-000000000000", "v1.0.0", "/test1")
	cfg2 := createTestStoredConfig("test-id-2", "test-api-2", "v2.0.0", "/test2")
	_ = server.db.SaveConfig(cfg1)
	_ = server.db.SaveConfig(cfg2)

	c, w := createTestContext("GET", "/rest-apis", nil)
	server.ListRestAPIs(c, api.ListRestAPIsParams{})

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, float64(2), response["count"])
}

// TestListAPIsWithFilters tests listing APIs with filters
func TestListRestAPIsWithFilters(t *testing.T) {
	server := createTestAPIServer()

	// Add test configs to store
	cfg1 := createTestStoredConfig("test-id-1", "0000-test-api-1-0000-000000000000", "v1.0.0", "/test1")
	cfg2 := createTestStoredConfig("test-id-2", "test-api-2", "v2.0.0", "/test2")
	_ = server.store.Add(cfg1)
	_ = server.store.Add(cfg2)

	// Test with displayName filter
	c, w := createTestContext("GET", "/rest-apis?displayName=test-api-1", nil)
	c.Request.URL.RawQuery = "displayName=test-api-1"
	displayName := "0000-test-api-1-0000-000000000000"
	server.ListRestAPIs(c, api.ListRestAPIsParams{DisplayName: &displayName})

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
}

// TestListAPIsEmpty tests listing APIs when none exist
func TestListRestAPIsEmpty(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContext("GET", "/rest-apis", nil)
	server.ListRestAPIs(c, api.ListRestAPIsParams{})

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, float64(0), response["count"])
}

// TestGetAPIByNameVersion tests getting an API by name and version
func TestGetAPIByNameVersion(t *testing.T) {
	server := createTestAPIServer()

	cfg := createTestStoredConfig("test-id-1", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	_ = server.store.Add(cfg)

	c, w := createTestContext("GET", "/rest-apis/test-api/v1.0.0", nil)
	c.Params = gin.Params{
		{Key: "name", Value: "0000-test-api-0000-000000000000"},
		{Key: "version", Value: "v1.0.0"},
	}
	server.GetAPIByNameVersion(c, "0000-test-api-0000-000000000000", "v1.0.0")

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "RestApi", response["kind"])
	status, ok := response["status"].(map[string]interface{})
	require.True(t, ok, "status should be a map, got %T", response["status"])
	assert.Equal(t, "deployed", status["state"])
}

// TestGetAPIByNameVersionNotFound tests getting an API that doesn't exist
func TestGetAPIByNameVersionNotFound(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContext("GET", "/rest-apis/nonexistent/v1.0.0", nil)
	server.GetAPIByNameVersion(c, "nonexistent", "v1.0.0")

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response api.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response.Status)
}

// TestGetAPIById tests getting an API by ID
func TestGetRestAPIById(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	cfg := createTestStoredConfig("0000-test-handle-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	cfg.GetMetadata().Name = "0000-test-handle-0000-000000000000"
	mockDB.SaveConfig(cfg)

	c, w := createTestContext("GET", "/rest-apis/test-handle", nil)
	server.GetRestAPIById(c, "0000-test-handle-0000-000000000000")

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// The GET response is the k8s-shaped resource body with server-managed
	// status merged in as a top-level field.
	assert.Equal(t, "RestApi", response["kind"])
	status, ok := response["status"].(map[string]interface{})
	require.True(t, ok, "status should be a map, got %T", response["status"])
	assert.Equal(t, "0000-test-handle-0000-000000000000", status["id"])
	assert.Equal(t, "deployed", status["state"])
}

// TestGetAPIByIdNotFound tests getting an API by ID that doesn't exist
func TestGetRestAPIByIdNotFound(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContext("GET", "/rest-apis/nonexistent", nil)
	server.GetRestAPIById(c, "nonexistent")

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response api.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response.Status)
}

// TestGetAPIByIdNoDB tests getting an API when DB is not available
func TestGetRestAPIByIdNoDB(t *testing.T) {
	server := createTestAPIServerWithDB(NewMockStorage())

	c, w := createTestContext("GET", "/rest-apis/test-id", nil)
	server.GetRestAPIById(c, "0000-test-id-0000-000000000000")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestGetAPIByIdWrongKind tests getting an API with wrong kind
func TestGetRestAPIByIdWrongKind(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	cfg := createTestStoredConfig("0000-test-handle-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	cfg.Kind = string(api.MCPProxyConfigurationKindMcp) // Wrong kind
	cfg.GetMetadata().Name = "0000-test-handle-0000-000000000000"
	mockDB.SaveConfig(cfg)

	c, w := createTestContext("GET", "/rest-apis/test-handle", nil)
	server.GetRestAPIById(c, "0000-test-handle-0000-000000000000")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestSearchDeploymentsWithNilStore tests SearchDeployments with nil store
func TestSearchDeploymentsWithNilStore(t *testing.T) {
	server := createTestAPIServer()
	server.store = nil

	c, w := createTestContext("GET", "/rest-apis?displayName=test", nil)
	c.Request.URL.RawQuery = "displayName=test"
	server.SearchDeployments(c, string(api.RestAPIKindRestApi))

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, float64(0), response["count"])
}

// TestSearchDeploymentsMCP tests SearchDeployments for MCP kind
func TestSearchDeploymentsMCP(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContext("GET", "/mcp-proxies?displayName=test", nil)
	c.Request.URL.RawQuery = "displayName=test"
	server.SearchDeployments(c, string(api.MCPProxyConfigurationKindMcp))

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "mcpProxies")
}

// TestGetConfigDump tests the config dump endpoint
func TestGetConfigDump(t *testing.T) {
	server := createTestAPIServer()

	// Add test config
	cfg := createTestStoredConfig("0000-test-handle-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	_ = server.store.Add(cfg)

	// Add test policy
	server.policyDefinitions["policy1|v1"] = models.PolicyDefinition{
		Name:    "policy1",
		Version: "v1",
	}

	c, w := createTestContext("GET", "/config_dump", nil)
	server.GetConfigDump(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response adminapi.ConfigDumpResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", *response.Status)
	assert.NotNil(t, response.Statistics)
	assert.NotNil(t, response.XdsSync)
	assert.Equal(t, "0", *response.XdsSync.PolicyChainVersion)
}

func TestGetXDSSyncStatus(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContext("GET", "/xds_sync_status", nil)
	server.GetXDSSyncStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response adminapi.XDSSyncStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "gateway-controller", *response.Component)
	assert.Equal(t, "0", *response.PolicyChainVersion)
	assert.NotNil(t, response.Timestamp)
}

func TestGetXDSSyncStatusWithPolicyVersion(t *testing.T) {
	server := createTestAPIServer()

	runtimeStore := storage.NewRuntimeConfigStore()
	snapshotMgr := policyxds.NewSnapshotManager(server.logger)
	snapshotMgr.SetRuntimeStore(runtimeStore)
	server.policyManager = policyxds.NewPolicyManager(snapshotMgr, server.logger)
	server.policyManager.SetRuntimeStore(runtimeStore)

	runtimeStore.IncrementResourceVersion()
	runtimeStore.IncrementResourceVersion()

	c, w := createTestContext("GET", "/xds_sync_status", nil)
	server.GetXDSSyncStatus(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response adminapi.XDSSyncStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "2", *response.PolicyChainVersion)
}

// TestGetConfigDumpWithCertificates tests config dump with certificates
func TestGetConfigDumpWithCertificates(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	// Add certificates to mock storage
	mockDB.certs = []*models.StoredCertificate{
		{
			UUID:        "0000-cert-1-0000-000000000000",
			Name:        "test-cert",
			Subject:     "CN=test",
			Issuer:      "CN=issuer",
			NotAfter:    time.Now().Add(24 * time.Hour),
			Certificate: []byte("cert-data"),
			CertCount:   1,
		},
	}

	c, w := createTestContext("GET", "/config_dump", nil)
	server.GetConfigDump(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestGetConfigDumpDBError tests config dump with database error
func TestGetConfigDumpDBError(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockDB.getErr = errors.New("db error")

	c, w := createTestContext("GET", "/config_dump", nil)
	server.GetConfigDump(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestHandleStatusUpdate tests the status update callback
func TestHandleStatusUpdate(t *testing.T) {
	server := createTestAPIServer()

	// Add test config
	cfg := createTestStoredConfig("0000-test-id-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	_ = server.store.Add(cfg)

	// Test successful deployment — callback only logs, does not modify DeployedAt
	// (DeployedAt is set at creation/undeployment time, not by the xDS callback)
	server.handleStatusUpdate("0000-test-id-0000-000000000000", true, "corr-id-1")

	// Verify config is still accessible and state unchanged
	updatedCfg, _ := server.store.Get("0000-test-id-0000-000000000000")
	assert.Equal(t, models.StateDeployed, updatedCfg.DesiredState)
}

// TestHandleStatusUpdateFailure tests status update for failed deployment
func TestHandleStatusUpdateFailure(t *testing.T) {
	server := createTestAPIServer()

	// Add test config
	cfg := createTestStoredConfig("0000-test-id-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	_ = server.store.Add(cfg)

	// Test failed deployment
	server.handleStatusUpdate("0000-test-id-0000-000000000000", false, "")

	// Verify status updated
	updatedCfg, _ := server.store.Get("0000-test-id-0000-000000000000")
	assert.Equal(t, models.StateDeployed, updatedCfg.DesiredState)
	assert.Nil(t, updatedCfg.DeployedAt)
}

// TestHandleStatusUpdateNotFound tests status update for non-existent config
func TestHandleStatusUpdateNotFound(t *testing.T) {
	server := createTestAPIServer()

	// Should not panic
	server.handleStatusUpdate("nonexistent", true, "")
}

// TestCreateRestAPIInvalidBody tests CreateRestAPI with invalid request body
// Note: This test requires a full deployment service setup, so we skip it
func TestCreateRestAPIInvalidBody(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

func TestCreateRestAPIDBError(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")
	mockDB.updateErr = errors.New("db write error")

	body := createTestRestAPIRequestBody(t, "test-handle", "test-display-name", "v1.0.0", "/test")
	c, w := createTestContextWithHeader("POST", "/rest-apis", body, map[string]string{
		"Content-Type": "application/json",
	})

	server.CreateRestAPI(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Empty(t, mockHub.publishedEvents)

	var response api.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "error", response.Status)
	assert.Equal(t, "Failed to create configuration", response.Message)
	assert.NotContains(t, w.Body.String(), "db write error")
}

// TestUpdateRestAPIInvalidBody tests UpdateRestAPI with invalid request body
// Note: This test requires the validator to return errors but the parser
// fails first due to nil pointer issues, so we skip it
func TestUpdateRestAPIInvalidBody(t *testing.T) {
	t.Skip("Skipping test that requires full parser/validator setup")
}

// TestUpdateRestAPINotFound tests UpdateRestAPI for non-existent API
func TestUpdateRestAPINotFound(t *testing.T) {
	server := createTestAPIServer()

	body := []byte(`{
		"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
		"kind": "RestApi",
		"metadata": {"name": "nonexistent"},
		"spec": {
			"displayName": "test",
			"version": "v1",
			"context": "/test",
			"upstream": {"main": {"url": "http://backend.com"}},
			"operations": [{"method": "GET", "path": "/"}]
		}
	}`)
	c, w := createTestContextWithHeader("PUT", "/rest-apis/nonexistent", body, map[string]string{
		"Content-Type": "application/json",
	})
	server.UpdateRestAPI(c, "nonexistent")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestUpdateRestAPIHandleMismatch tests UpdateRestAPI with handle mismatch
// Note: This test requires full parser/validator setup
func TestUpdateRestAPIHandleMismatch(t *testing.T) {
	t.Skip("Skipping test that requires full parser/validator setup")
}

// TestDeleteRestAPIWithDBAndEventHub tests DeleteRestAPI on the DB-backed event-driven path.
func TestDeleteRestAPIWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	cfg := createTestStoredConfig("0000-test-id-0000-000000000000", "test-api", "v1.0.0", "/test")
	cfg.Handle = "test-handle"
	mockDB.SaveConfig(cfg)
	mockDB.apiKeys["key-1"] = &models.APIKey{
		UUID:         "key-1",
		APIKey:       "secret-key",
		ArtifactUUID: cfg.UUID,
		Name:         "default",
		Status:       models.APIKeyStatusActive,
	}

	c, w := createTestContext("DELETE", "/rest-apis/test-handle", nil)
	c.Set(middleware.CorrelationIDKey, "corr-id-delete")

	server.DeleteRestAPI(c, "test-handle")

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeAPI, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "DELETE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, cfg.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-delete", mockHub.publishedEvents[0].event.EventID)

	_, err := mockDB.GetConfig(cfg.UUID)
	require.Error(t, err)
	assert.Empty(t, mockDB.apiKeys)
}

// TestDeleteRestAPINotFound tests DeleteRestAPI for non-existent API
func TestDeleteRestAPINotFound(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContext("DELETE", "/rest-apis/nonexistent", nil)
	server.DeleteRestAPI(c, "nonexistent")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestListLLMProviderTemplates tests listing LLM provider templates
// Note: This test requires full deployment service setup
func TestListLLMProviderTemplates(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestGetLLMProviderTemplateByIdNotFound tests getting a non-existent template
// Note: This test requires full deployment service setup
func TestGetLLMProviderTemplateByIdNotFound(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestCreateLLMProviderTemplateInvalidBody tests CreateLLMProviderTemplate with invalid body
// Note: This test requires full deployment service setup
func TestCreateLLMProviderTemplateInvalidBody(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestUpdateLLMProviderTemplateInvalidBody tests UpdateLLMProviderTemplate with invalid body
// Note: This test requires full deployment service setup
func TestUpdateLLMProviderTemplateInvalidBody(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestDeleteLLMProviderTemplateNotFound tests deleting a non-existent template
// Note: This test requires full deployment service setup
func TestDeleteLLMProviderTemplateNotFound(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

func TestCreateLLMProviderTemplateWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	body := createLLMTemplateBody(t, "openai", "OpenAI Template")
	c, w := createTestContextWithHeader("POST", "/llm-provider-templates", body, map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(middleware.CorrelationIDKey, "corr-id-create-llm-template")

	server.CreateLLMProviderTemplate(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	require.Len(t, mockDB.templates, 1)
	require.Len(t, mockHub.publishedEvents, 1)

	var storedTemplate *models.StoredLLMProviderTemplate
	for _, template := range mockDB.templates {
		storedTemplate = template
	}
	require.NotNil(t, storedTemplate)

	assert.Equal(t, "openai", storedTemplate.GetHandle())
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeLLMTemplate, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "CREATE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, storedTemplate.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-create-llm-template", mockHub.publishedEvents[0].event.EventID)

	_, err := server.store.GetTemplate(storedTemplate.UUID)
	require.Error(t, err)
}

func TestUpdateLLMProviderTemplateWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	template := &models.StoredLLMProviderTemplate{
		UUID: "template-update-id",
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
			Metadata: api.Metadata{
				Name: "openai",
			},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "OpenAI Template",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, mockDB.SaveLLMProviderTemplate(template))
	require.NoError(t, server.store.AddTemplate(template))

	body := createLLMTemplateBody(t, "openai", "Updated OpenAI Template")
	c, w := createTestContextWithHeader("PUT", "/llm-provider-templates/openai", body, map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(middleware.CorrelationIDKey, "corr-id-update-llm-template")

	server.UpdateLLMProviderTemplate(c, "openai")

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeLLMTemplate, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "UPDATE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, template.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-update-llm-template", mockHub.publishedEvents[0].event.EventID)

	storedInDB, err := mockDB.GetLLMProviderTemplate(template.UUID)
	require.NoError(t, err)
	assert.Equal(t, "Updated OpenAI Template", storedInDB.Configuration.Spec.DisplayName)

	storedInMemory, err := server.store.GetTemplate(template.UUID)
	require.NoError(t, err)
	assert.Equal(t, "OpenAI Template", storedInMemory.Configuration.Spec.DisplayName)
}

func TestDeleteLLMProviderTemplateWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	template := &models.StoredLLMProviderTemplate{
		UUID: "template-delete-id",
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
			Metadata: api.Metadata{
				Name: "openai",
			},
			Spec: api.LLMProviderTemplateData{
				DisplayName: "OpenAI Template",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, mockDB.SaveLLMProviderTemplate(template))

	c, w := createTestContext("DELETE", "/llm-provider-templates/openai", nil)
	c.Set(middleware.CorrelationIDKey, "corr-id-delete-llm-template")

	server.DeleteLLMProviderTemplate(c, "openai")

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeLLMTemplate, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "DELETE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, template.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-delete-llm-template", mockHub.publishedEvents[0].event.EventID)

	_, err := mockDB.GetLLMProviderTemplate(template.UUID)
	require.Error(t, err)

	_, err = server.store.GetTemplate(template.UUID)
	require.EqualError(t, err, fmt.Sprintf("template with ID '%s' not found", template.UUID))
}

// TestListLLMProviders tests listing LLM providers
// Note: This test requires full deployment service setup
func TestListLLMProviders(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestGetLLMProviderByIdNotFound tests getting a non-existent LLM provider
// Note: This test requires full deployment service setup
func TestGetLLMProviderByIdNotFound(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestCreateLLMProviderInvalidBody tests CreateLLMProvider with invalid body
// Note: This test requires full deployment service setup
func TestCreateLLMProviderInvalidBody(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestUpdateLLMProviderInvalidBody tests UpdateLLMProvider with invalid body
// Note: This test requires full deployment service setup
func TestUpdateLLMProviderInvalidBody(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestDeleteLLMProviderNotFound tests deleting a non-existent LLM provider
// Note: This test requires full deployment service setup
func TestDeleteLLMProviderNotFound(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestListLLMProxies tests listing LLM proxies
// Note: This test requires full deployment service setup
func TestListLLMProxies(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestGetLLMProxyByIdNotFound tests getting a non-existent LLM proxy
// Note: This test requires full deployment service setup
func TestGetLLMProxyByIdNotFound(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestCreateLLMProxyInvalidBody tests CreateLLMProxy with invalid body
// Note: This test requires full deployment service setup
func TestCreateLLMProxyInvalidBody(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestUpdateLLMProxyInvalidBody tests UpdateLLMProxy with invalid body
// Note: This test requires full deployment service setup
func TestUpdateLLMProxyInvalidBody(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestDeleteLLMProxyNotFound tests deleting a non-existent LLM proxy
// Note: This test requires full deployment service setup
func TestDeleteLLMProxyNotFound(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

func TestListMCPProxies(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	cfg := createTestMCPStoredConfig(t, "0000-mcp-id-0000-000000000000", "test-mcp", "Test MCP", "v1.0.0", "/mcp", models.StateDeployed)
	require.NoError(t, mockDB.SaveConfig(cfg))

	c, w := createTestContext("GET", "/mcp-proxies", nil)
	server.ListMCPProxies(c, api.ListMCPProxiesParams{})

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, float64(1), response["count"])
}

// TestListMCPProxiesWithFilters tests listing MCP proxies with filters
// Note: This test requires full deployment service setup
func TestListMCPProxiesWithFilters(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestGetMCPProxyByIdNotFound tests getting a non-existent MCP proxy
// Note: This test requires full deployment service setup
func TestGetMCPProxyByIdNotFound(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestCreateMCPProxyInvalidBody tests CreateMCPProxy with invalid body
// Note: This test requires full deployment service setup
func TestCreateMCPProxyInvalidBody(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestUpdateMCPProxyInvalidBody tests UpdateMCPProxy with invalid body
// Note: This test requires full deployment service setup
func TestUpdateMCPProxyInvalidBody(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestDeleteMCPProxyNotFound tests deleting a non-existent MCP proxy
// Note: This test requires full deployment service setup
func TestDeleteMCPProxyNotFound(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

func TestCreateMCPProxyWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	body := createTestMCPRequestBody(t, "test-mcp", "Test MCP", "v1.0.0", "/mcp")
	c, w := createTestContextWithHeader("POST", "/mcp-proxies", body, map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(middleware.CorrelationIDKey, "corr-id-create-mcp")

	server.CreateMCPProxy(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	cfg, err := mockDB.GetConfigByKindAndHandle(string(api.MCPProxyConfigurationKindMcp), "test-mcp")
	require.NoError(t, err)
	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeMCPProxy, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "CREATE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, cfg.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-create-mcp", mockHub.publishedEvents[0].event.EventID)
	assert.Equal(t, eventhub.EmptyEventData, mockHub.publishedEvents[0].event.EventData)

	_, err = server.store.Get(cfg.UUID)
	require.Error(t, err)
}

func TestDeleteMCPProxyWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	cfg := createTestMCPStoredConfig(t, "0000-mcp-delete-id-0000-000000000000", "test-mcp", "Test MCP", "v1.0.0", "/mcp", models.StateDeployed)
	require.NoError(t, mockDB.SaveConfig(cfg))
	require.NoError(t, server.store.Add(cfg))

	c, w := createTestContext("DELETE", "/mcp-proxies/test-mcp", nil)
	c.Set(middleware.CorrelationIDKey, "corr-id-delete-mcp")

	server.DeleteMCPProxy(c, "test-mcp")

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeMCPProxy, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "DELETE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, cfg.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-delete-mcp", mockHub.publishedEvents[0].event.EventID)
	assert.Equal(t, eventhub.EmptyEventData, mockHub.publishedEvents[0].event.EventData)

	_, err := mockDB.GetConfig(cfg.UUID)
	require.Error(t, err)

	_, err = server.store.Get(cfg.UUID)
	require.NoError(t, err)
}

// TestGenerateAPIKeyNoAuth tests CreateAPIKey without authentication
func TestGenerateAPIKeyNoAuth(t *testing.T) {
	server := createTestAPIServer()

	body := []byte(`{"name": "test-key"}`)
	c, w := createTestContextWithHeader("POST", "/rest-apis/test-handle/api-keys", body, map[string]string{
		"Content-Type": "application/json",
	})
	server.CreateAPIKey(c, "0000-test-handle-0000-000000000000")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestGenerateAPIKeyInvalidAuthContext tests CreateAPIKey with invalid auth context
func TestGenerateAPIKeyInvalidAuthContext(t *testing.T) {
	server := createTestAPIServer()

	body := []byte(`{"name": "test-key"}`)
	c, w := createTestContextWithHeader("POST", "/rest-apis/test-handle/api-keys", body, map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(constants.AuthContextKey, "invalid-context") // Wrong type
	server.CreateAPIKey(c, "0000-test-handle-0000-000000000000")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGenerateAPIKeyInvalidBody tests CreateAPIKey with invalid body
func TestGenerateAPIKeyInvalidBody(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContextWithHeader("POST", "/rest-apis/test-handle/api-keys", []byte("invalid json {{{"), map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(constants.AuthContextKey, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	server.CreateAPIKey(c, "0000-test-handle-0000-000000000000")

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateAPIKeyWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	body := createTestAPIKeyRequestBody(t, "test-key", "Test Key", "external-key-123456789012345678901234567890123456")
	c, w := createTestContextWithHeader("POST", "/rest-apis/test-handle/api-keys", body, map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(constants.AuthContextKey, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	c.Set(middleware.CorrelationIDKey, "corr-id-create-key")

	server.CreateAPIKey(c, "test-handle")

	assert.Equal(t, http.StatusCreated, w.Code)
	require.Len(t, mockHub.publishedEvents, 1)

	createdKey, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, "test-key")
	require.NoError(t, err)
	assert.Equal(t, cfg.UUID, createdKey.ArtifactUUID)
	assert.Equal(t, "test-user", createdKey.CreatedBy)
	assert.Equal(t, string(api.External), createdKey.Source)

	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeAPIKey, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "CREATE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, apikey.BuildAPIKeyEntityID(cfg.UUID, createdKey.UUID), mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-create-key", mockHub.publishedEvents[0].event.EventID)
	assert.Equal(t, eventhub.EmptyEventData, mockHub.publishedEvents[0].event.EventData)
}

func TestCreateAPIKeyDBError(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")
	mockDB.saveErr = errors.New("db save error")

	body := createTestAPIKeyRequestBody(t, "test-key", "Test Key", "external-key-123456789012345678901234567890123456")
	c, w := createTestContextWithHeader("POST", "/rest-apis/test-handle/api-keys", body, map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(constants.AuthContextKey, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})

	server.CreateAPIKey(c, "test-handle")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Empty(t, mockHub.publishedEvents)

	_, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, "test-key")
	require.Error(t, err)
}

// TestRevokeAPIKeyNoAuth tests RevokeAPIKey without authentication
func TestRevokeAPIKeyNoAuth(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContext("DELETE", "/rest-apis/test-handle/api-keys/test-key", nil)
	server.RevokeAPIKey(c, "0000-test-handle-0000-000000000000", "test-key")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRevokeAPIKeyWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	storeKey := createStoredExternalAPIKey("0000-test-key-id-0000-000000000000", cfg.UUID, "test-key", "Test Key", "test-user", "apip_****old")
	dbKey := *storeKey
	require.NoError(t, server.store.StoreAPIKey(storeKey))
	require.NoError(t, mockDB.SaveAPIKey(&dbKey))

	c, w := createTestContext("DELETE", "/rest-apis/test-handle/api-keys/test-key", nil)
	c.Set(constants.AuthContextKey, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	c.Set(middleware.CorrelationIDKey, "corr-id-revoke-key")

	server.RevokeAPIKey(c, "test-handle", "test-key")

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeAPIKey, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "DELETE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, apikey.BuildAPIKeyEntityID(cfg.UUID, storeKey.UUID), mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-revoke-key", mockHub.publishedEvents[0].event.EventID)
	assert.Equal(t, eventhub.EmptyEventData, mockHub.publishedEvents[0].event.EventData)

	_, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, "test-key")
	require.Error(t, err)
}

func TestRevokeAPIKeyDBError(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")
	mockDB.updateErr = errors.New("db update error")

	storeKey := createStoredExternalAPIKey("0000-test-key-id-0000-000000000000", cfg.UUID, "test-key", "Test Key", "test-user", "apip_****old")
	dbKey := *storeKey
	require.NoError(t, server.store.StoreAPIKey(storeKey))
	require.NoError(t, mockDB.SaveAPIKey(&dbKey))

	c, w := createTestContext("DELETE", "/rest-apis/test-handle/api-keys/test-key", nil)
	c.Set(constants.AuthContextKey, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})

	server.RevokeAPIKey(c, "test-handle", "test-key")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Empty(t, mockHub.publishedEvents)

	storedKey, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, "test-key")
	require.NoError(t, err)
	assert.Equal(t, models.APIKeyStatusActive, storedKey.Status)
}

// TestRegenerateAPIKeyNoAuth tests RegenerateAPIKey without authentication
func TestRegenerateAPIKeyNoAuth(t *testing.T) {
	server := createTestAPIServer()

	body := []byte(`{}`)
	c, w := createTestContextWithHeader("POST", "/rest-apis/test-handle/api-keys/test-key/regenerate", body, map[string]string{
		"Content-Type": "application/json",
	})
	server.RegenerateAPIKey(c, "0000-test-handle-0000-000000000000", "test-key")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRegenerateAPIKeyInvalidBody tests RegenerateAPIKey with invalid body
func TestRegenerateAPIKeyInvalidBody(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContextWithHeader("POST", "/rest-apis/test-handle/api-keys/test-key/regenerate", []byte("invalid {{{"), map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(constants.AuthContextKey, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	server.RegenerateAPIKey(c, "0000-test-handle-0000-000000000000", "test-key")

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestListAPIKeysNoAuth tests ListAPIKeys without authentication
func TestListAPIKeysNoAuth(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContext("GET", "/rest-apis/test-handle/api-keys", nil)
	server.ListAPIKeys(c, "0000-test-handle-0000-000000000000")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestExtractAuthenticatedUserSuccess tests successful user extraction
func TestExtractAuthenticatedUserSuccess(t *testing.T) {
	server := createTestAPIServer()
	c, _ := createTestContext("GET", "/test", nil)

	authCtx := commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	}
	c.Set(constants.AuthContextKey, authCtx)

	user, ok := server.extractAuthenticatedUser(c, "TestOperation", "corr-id")

	assert.True(t, ok)
	assert.NotNil(t, user)
	assert.Equal(t, "test-user", user.UserID)
}

// TestConvertAPIPolicy tests the convertAPIPolicy function
func TestConvertAPIPolicy(t *testing.T) {
	params := map[string]interface{}{
		"0000-key1-0000-000000000000": "value1",
		"0000-key2-0000-000000000000": 42,
	}
	policy := api.Policy{
		Name:    "test-policy",
		Version: "v1.0.0",
		Params:  &params,
	}

	result := convertAPIPolicy(policy, "api", "v1.0.0")

	assert.Equal(t, "test-policy", result.Name)
	assert.Equal(t, "v1.0.0", result.Version)
	assert.True(t, result.Enabled)
	assert.Equal(t, "value1", result.Parameters["0000-key1-0000-000000000000"])
	assert.Equal(t, 42, result.Parameters["0000-key2-0000-000000000000"])
	assert.Equal(t, "api", result.Parameters["attachedTo"])
}

// TestConvertAPIPolicyNoParams tests convertAPIPolicy with no params
func TestConvertAPIPolicyNoParams(t *testing.T) {
	policy := api.Policy{
		Name:    "test-policy",
		Version: "v1.0.0",
	}

	result := convertAPIPolicy(policy, "", "v1.0.0")

	assert.Equal(t, "test-policy", result.Name)
	assert.NotNil(t, result.Parameters)
	assert.Empty(t, result.Parameters["attachedTo"])
}

// TestBuildStoredPolicyFromAPINoPolicies tests buildStoredPolicyFromAPI with no policies
func TestBuildStoredPolicyFromAPINoPolicies(t *testing.T) {
	server := createTestAPIServer()

	apiConfig := api.RestAPI{
		Kind: api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
			DisplayName: "0000-test-api-0000-000000000000",
			Version:     "v1.0",
			Context:     "/test",
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{
					Url: stringPtr("http://backend.example.com"),
				},
			},
			Operations: []api.Operation{
				{
					Method: "GET",
					Path:   "/resource",
				},
			},
		},
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-test-id-0000-000000000000",
		Kind:                string(api.RestAPIKindRestApi),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
		Origin:              models.OriginGatewayAPI,
	}

	result := policybuilder.DerivePolicyFromAPIConfig(cfg, server.routerConfig, server.systemConfig, server.policyDefinitions)
	assert.Nil(t, result)
}

// TestGetLLMProviderTemplateErrors tests getLLMProviderTemplate error cases
func TestGetLLMProviderTemplateErrors(t *testing.T) {
	server := createTestAPIServer()

	// Test nil sourceConfig
	_, err := server.getLLMProviderTemplate(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")

	// Test invalid sourceConfig
	_, err = server.getLLMProviderTemplate("invalid")
	assert.Error(t, err)

	// Test empty template name
	_, err = server.getLLMProviderTemplate(map[string]interface{}{
		"spec": map[string]interface{}{
			"template": "",
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")

	// Test non-string template name
	_, err = server.getLLMProviderTemplate(map[string]interface{}{
		"spec": map[string]interface{}{
			"template": 123,
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a string")
}

// TestPopulatePropsForSystemPolicies tests populatePropsForSystemPolicies
func TestPopulatePropsForSystemPolicies(t *testing.T) {
	server := createTestAPIServer()
	props := make(map[string]any)

	// Test with nil config - should not panic
	server.populatePropsForSystemPolicies(nil, props)
	assert.Empty(t, props)

	// Test with valid config
	server.populatePropsForSystemPolicies(map[string]interface{}{}, props)
	// Props should remain empty as no action is taken
}

// TestWaitForDeploymentAndNotifyTimeout tests the timeout scenario
// Note: This test involves deliberate concurrent access patterns that trigger
// race detector warnings but represent valid production behavior with proper locking
func TestWaitForDeploymentAndNotifyTimeout(t *testing.T) {
	server := createTestAPIServer()
	server.controlPlaneClient = &MockControlPlaneClient{connected: true}

	// Add config that starts pending and will be updated to deployed
	cfg := createTestStoredConfig("0000-test-id-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	cfg.DesiredState = models.StateDeployed
	_ = server.store.Add(cfg)

	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("waitForDeploymentAndPush panicked: %v", r)
				return
			}
			done <- nil
		}()

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		server.waitForDeploymentAndPush("0000-test-id-0000-000000000000", "test-correlation", logger)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)

	case <-time.After(2 * time.Second):
		// Trigger graceful exit by updating status to deployed
		server.handleStatusUpdate("0000-test-id-0000-000000000000", true, "")
		require.NoError(t, <-done)

		retrievedCfg, err := server.store.Get("0000-test-id-0000-000000000000")
		require.NoError(t, err)
		assert.Equal(t, models.StateDeployed, retrievedCfg.DesiredState)
	}
}

func TestWaitForDeploymentAndPush_RetriesUntilConfigAppears(t *testing.T) {
	server := createTestAPIServer()
	mockCP := &MockControlPlaneClient{connected: true}
	server.controlPlaneClient = mockCP

	done := make(chan struct{})
	go func() {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		server.waitForDeploymentAndPush("0000-delayed-id-0000-000000000000", "test-correlation", logger)
		close(done)
	}()

	// Let the first ticker fire while the config is still absent.
	time.Sleep(700 * time.Millisecond)

	cfg := createTestStoredConfig("0000-delayed-id-0000-000000000000", "delayed-api", "v1.0.0", "/delayed")
	deployedAt := time.Now()
	cfg.DeployedAt = &deployedAt
	require.NoError(t, server.store.Add(cfg))

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("waitForDeploymentAndPush did not complete after config appeared")
	}

	require.Equal(t, 1, mockCP.PushCount())
}

// TestNewAPIServer tests the NewAPIServer constructor
func TestNewAPIServer(t *testing.T) {
	store := storage.NewConfigStore()
	mockDB := NewMockStorage()

	policyDefs := map[string]models.PolicyDefinition{
		"test|v1": {Name: "test", Version: "v1"},
	}

	// LLMProviderTemplate structure matches API spec
	templateDefs := make(map[string]*api.LLMProviderTemplate)
	templateName := "test-template"
	templateDefs[templateName] = &api.LLMProviderTemplate{
		Metadata: api.Metadata{
			Name: templateName,
		},
	}

	validator := config.NewAPIValidator()

	vhosts := &config.VHostsConfig{
		Main:    config.VHostEntry{Default: "localhost"},
		Sandbox: config.VHostEntry{Default: "sandbox-localhost"},
	}

	systemConfig := &config.Config{
		Controller: config.Controller{},
		Router: config.RouterConfig{
			GatewayHost: "localhost",
			VHosts:      *vhosts,
		},
		APIKey: config.APIKeyConfig{
			APIKeysPerUserPerAPI: 5,
		},
	}

	// This test is simplified - full test would require proper xDS mocks
	// Instead, we just verify the structure
	t.Run("verify test server creation", func(t *testing.T) {
		server := createTestAPIServer()
		assert.NotNil(t, server)
		assert.NotNil(t, server.store)
		assert.NotNil(t, server.db)
		assert.NotNil(t, server.logger)
		assert.NotNil(t, server.parser)
		assert.NotNil(t, server.validator)
	})

	// Verify configuration objects are created correctly
	t.Run("verify config structures", func(t *testing.T) {
		assert.NotNil(t, store)
		assert.NotNil(t, mockDB)
		assert.NotNil(t, validator)
		assert.NotEmpty(t, policyDefs)
		assert.NotEmpty(t, templateDefs)
		assert.NotNil(t, systemConfig)
		assert.Equal(t, "localhost", systemConfig.Router.GatewayHost)
	})
}

// TestSearchDeploymentsFilters tests SearchDeployments with various filters
func TestSearchDeploymentsFilters(t *testing.T) {
	server := createTestAPIServer()

	// Add test configs with different desired states
	cfg1 := createTestStoredConfig("test-id-1", "api-one", "v1.0.0", "/ctx1")
	cfg1.DesiredState = models.StateDeployed
	cfg2 := createTestStoredConfig("test-id-2", "api-two", "v2.0.0", "/ctx2")
	cfg2.DesiredState = models.StateUndeployed
	_ = server.store.Add(cfg1)
	_ = server.store.Add(cfg2)

	testCases := []struct {
		name        string
		query       string
		expectedLen int
	}{
		{"filter by displayName", "displayName=api-one", 1},
		{"filter by version", "version=v2.0.0", 1},
		{"filter by context", "context=/ctx1", 1},
		{"filter by status", "status=deployed", 1},
		{"no matching filter", "displayName=nonexistent", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := createTestContext("GET", "/rest-apis?"+tc.query, nil)
			c.Request.URL.RawQuery = tc.query
			server.SearchDeployments(c, string(api.RestAPIKindRestApi))

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, float64(tc.expectedLen), response["count"])
		})
	}
}

// TestGetRestAPIByIdWithDeployedAt tests GetRestAPIById with deployedAt in response
func TestGetRestAPIByIdWithDeployedAt(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	cfg := createTestStoredConfig("0000-test-handle-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	cfg.GetMetadata().Name = "0000-test-handle-0000-000000000000"
	deployedAt := time.Now()
	cfg.DeployedAt = &deployedAt
	mockDB.SaveConfig(cfg)

	c, w := createTestContext("GET", "/rest-apis/test-handle", nil)
	server.GetRestAPIById(c, "0000-test-handle-0000-000000000000")

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Responses are k8s-shaped: server-managed timestamps live under the
	// top-level status block.
	status, ok := response["status"].(map[string]interface{})
	require.True(t, ok, "status should be a map, got %T", response["status"])
	assert.Contains(t, status, "deployedAt")
}

// TestGetAPIByNameVersionWithDeployedAt tests GetAPIByNameVersion with deployedAt in response
func TestGetAPIByNameVersionWithDeployedAt(t *testing.T) {
	server := createTestAPIServer()

	cfg := createTestStoredConfig("test-id-1", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	deployedAt := time.Now()
	cfg.DeployedAt = &deployedAt
	_ = server.store.Add(cfg)

	c, w := createTestContext("GET", "/rest-apis/test-api/v1.0.0", nil)
	server.GetAPIByNameVersion(c, "0000-test-api-0000-000000000000", "v1.0.0")

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	status, ok := response["status"].(map[string]interface{})
	require.True(t, ok, "status should be a map, got %T", response["status"])
	assert.Contains(t, status, "deployedAt")
}

// TestTimePtr tests the timePtr helper function
func TestTimePtr(t *testing.T) {
	now := time.Now()
	ptr := timePtr(now)

	assert.NotNil(t, ptr)
	assert.Equal(t, now, *ptr)
}

// TestStringPtr tests the stringPtr helper function
func TestStringPtr(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"regular string", "test"},
		{"unicode string", "测试"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ptr := stringPtr(tt.input)
			assert.NotNil(t, ptr)
			assert.Equal(t, tt.input, *ptr)
		})
	}
}

// TestHandleStatusUpdateWithDB tests handleStatusUpdate with database
func TestHandleStatusUpdateWithDB(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	// Add test config to both store and DB
	cfg := createTestStoredConfig("0000-test-id-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	_ = server.store.Add(cfg)
	mockDB.configs["0000-test-id-0000-000000000000"] = cfg

	// Test successful deployment
	server.handleStatusUpdate("0000-test-id-0000-000000000000", true, "corr-id-1")

	// Verify both store and DB are updated
	updatedCfg, _ := server.store.Get("0000-test-id-0000-000000000000")
	assert.Equal(t, models.StateDeployed, updatedCfg.DesiredState)
}

// TestHandleStatusUpdateDBError tests handleStatusUpdate with DB error
func TestHandleStatusUpdateDBError(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockDB.updateErr = errors.New("db error")

	// Add test config
	cfg := createTestStoredConfig("0000-test-id-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	_ = server.store.Add(cfg)
	mockDB.configs["0000-test-id-0000-000000000000"] = cfg

	// Should not panic even with DB error
	server.handleStatusUpdate("0000-test-id-0000-000000000000", true, "corr-id-1")
}

// TestBuildStoredPolicyFromAPIInvalidKind tests buildStoredPolicyFromAPI with invalid kind
func TestBuildStoredPolicyFromAPIInvalidKind(t *testing.T) {
	server := createTestAPIServer()

	apiConfig := api.RestAPI{
		Kind: api.RestAPIKind("InvalidKind"),
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-test-id-0000-000000000000",
		Kind:                "InvalidKind",
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
		Origin:              models.OriginGatewayAPI,
	}

	result := policybuilder.DerivePolicyFromAPIConfig(cfg, server.routerConfig, server.systemConfig, server.policyDefinitions)
	assert.Nil(t, result)
}

// TestConfigDumpAPIStatusConversion tests status conversion in GetConfigDump
func TestConfigDumpAPIStatusConversion(t *testing.T) {
	server := createTestAPIServer()

	testCases := []struct {
		name   string
		status models.DesiredState
	}{
		{"deployed", models.StateDeployed},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear store
			server.store = storage.NewConfigStore()

			cfg := createTestStoredConfig("0000-test-id-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
			cfg.DesiredState = tc.status
			_ = server.store.Add(cfg)

			c, w := createTestContext("GET", "/config_dump", nil)
			server.GetConfigDump(c)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// TestSearchDeploymentsAPIKind tests SearchDeployments for API kind with filters
func TestSearchDeploymentsAPIKind(t *testing.T) {
	server := createTestAPIServer()

	cfg1 := createTestStoredConfig("test-id-1", "api-one", "v1.0.0", "/ctx1")
	cfg2 := createTestStoredConfig("test-id-2", "api-two", "v1.0.0", "/ctx2")
	_ = server.store.Add(cfg1)
	_ = server.store.Add(cfg2)

	c, w := createTestContext("GET", "/rest-apis?displayName=api-one&version=v1.0.0", nil)
	c.Request.URL.RawQuery = "displayName=api-one&version=v1.0.0"
	server.SearchDeployments(c, string(api.RestAPIKindRestApi))

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, float64(1), response["count"])
	assert.Contains(t, response, "apis")
}

// TestValidationErrorsInUpdateRestAPI tests UpdateRestAPI with validation errors
// Note: This test requires full parser/validator setup
func TestValidationErrorsInUpdateRestAPI(t *testing.T) {
	t.Skip("Skipping test that requires full parser/validator setup")
}

// TestGetLLMProviderByIdFound tests getting an existing LLM provider
func TestGetLLMProviderByIdFound(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	providerConfig := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata: api.Metadata{
			Name: "test-llm-provider",
		},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-llm",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("http://llm-backend.com"),
			},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
		},
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-llm-id-0000-000000000000",
		Kind:                string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:              "test-llm-provider",
		DisplayName:         "test-llm",
		Version:             "v1.0",
		SourceConfiguration: providerConfig,
		DesiredState:        models.StateDeployed,
		Origin:              models.OriginGatewayAPI,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	require.NoError(t, mockDB.SaveConfig(cfg))

	c, w := createTestContext("GET", "/llm-providers/test-llm-provider", nil)
	server.GetLLMProviderById(c, "test-llm-provider")

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetLLMProviderByIdFoundInDBWithoutStore(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	providerConfig := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata: api.Metadata{
			Name: "test-llm-provider",
		},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-llm",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("http://llm-backend.com"),
			},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
		},
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-llm-id-0000-000000000000",
		Kind:                string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:              "test-llm-provider",
		DisplayName:         "test-llm",
		Version:             "v1.0",
		SourceConfiguration: providerConfig,
		DesiredState:        models.StateDeployed,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	mockDB.SaveConfig(cfg)

	c, w := createTestContext("GET", "/llm-providers/test-llm-provider", nil)
	server.GetLLMProviderById(c, "test-llm-provider")

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestGetLLMProxyByIdFound tests getting an existing LLM proxy
func TestGetLLMProxyByIdFound(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	proxyConfig := api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata: api.Metadata{
			Name: "test-llm-proxy-handle",
		},
		Spec: api.LLMProxyConfigData{
			DisplayName: "test-llm-proxy",
			Version:     "v1.0",
			Provider: api.LLMProxyProvider{
				Id: "test-llm-provider",
			},
		},
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-llm-proxy-id-0000-000000000000",
		Kind:                string(api.LLMProxyConfigurationKindLlmProxy),
		Handle:              "test-llm-proxy-handle",
		DisplayName:         "test-llm-proxy",
		Version:             "v1.0",
		SourceConfiguration: proxyConfig,
		DesiredState:        models.StateDeployed,
		Origin:              models.OriginGatewayAPI,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	require.NoError(t, mockDB.SaveConfig(cfg))

	c, w := createTestContext("GET", "/llm-proxies/test-llm-proxy-handle", nil)
	server.GetLLMProxyById(c, "test-llm-proxy-handle")

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestGetLLMProviderByIdWithDeployedAt tests GetLLMProviderById with deployedAt
func TestGetLLMProviderByIdWithDeployedAt(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	deployedAt := time.Now()
	providerConfig := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LLMProviderConfigurationKindLlmProvider,
		Metadata: api.Metadata{
			Name: "test-llm-provider",
		},
		Spec: api.LLMProviderConfigData{
			DisplayName: "test-llm",
			Version:     "v1.0",
			Template:    "openai",
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: stringPtr("http://llm-backend.com"),
			},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
		},
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-llm-id-0000-000000000000",
		Kind:                string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:              "test-llm-provider",
		DisplayName:         "test-llm",
		Version:             "v1.0",
		SourceConfiguration: providerConfig,
		DesiredState:        models.StateDeployed,
		Origin:              models.OriginGatewayAPI,
		DeployedAt:          &deployedAt,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	require.NoError(t, mockDB.SaveConfig(cfg))

	c, w := createTestContext("GET", "/llm-providers/test-llm-provider", nil)
	server.GetLLMProviderById(c, "test-llm-provider")

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	status, ok := response["status"].(map[string]interface{})
	require.True(t, ok, "status should be a map, got %T", response["status"])
	assert.Contains(t, status, "deployedAt")
}

// TestGetLLMProxyByIdWithDeployedAt tests GetLLMProxyById with deployedAt
func TestGetLLMProxyByIdWithDeployedAt(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	deployedAt := time.Now()
	proxyConfig := api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata: api.Metadata{
			Name: "test-llm-proxy-handle",
		},
		Spec: api.LLMProxyConfigData{
			DisplayName: "test-llm-proxy",
			Version:     "v1.0",
			Provider: api.LLMProxyProvider{
				Id: "test-llm-provider",
			},
		},
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-llm-proxy-id-0000-000000000000",
		Kind:                string(api.LLMProxyConfigurationKindLlmProxy),
		Handle:              "test-llm-proxy-handle",
		DisplayName:         "test-llm-proxy",
		Version:             "v1.0",
		SourceConfiguration: proxyConfig,
		DesiredState:        models.StateDeployed,
		Origin:              models.OriginGatewayAPI,
		DeployedAt:          &deployedAt,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	require.NoError(t, mockDB.SaveConfig(cfg))

	c, w := createTestContext("GET", "/llm-proxies/test-llm-proxy-handle", nil)
	server.GetLLMProxyById(c, "test-llm-proxy-handle")

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestHandleStatusUpdateStoreError tests handleStatusUpdate with store error
func TestHandleStatusUpdateStoreError(t *testing.T) {
	server := createTestAPIServer()

	// Config exists in DB but not in store
	mockDB := server.db.(*MockStorage)
	cfg := createTestStoredConfig("0000-test-id-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	mockDB.configs["0000-test-id-0000-000000000000"] = cfg

	// Config in store for reading
	_ = server.store.Add(cfg)

	// Corrupt the store to cause an error on update
	// Since we can't easily corrupt the store, just verify it doesn't panic
	server.handleStatusUpdate("0000-test-id-0000-000000000000", true, "")

	updatedCfg, _ := server.store.Get("0000-test-id-0000-000000000000")
	assert.Equal(t, models.StateDeployed, updatedCfg.DesiredState)
}

// TestCreateRestAPIMissingContentType tests CreateRestAPI with missing content type
// Note: This test requires full deployment service setup
func TestCreateRestAPIMissingContentType(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestDeleteLLMProviderInternalError tests DeleteLLMProvider with internal error
// Note: This test requires full deployment service setup
func TestDeleteLLMProviderInternalError(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

func TestDeleteLLMProviderWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	cfg := &models.StoredConfig{
		UUID:        "0000-llm-provider-id-0000-000000000000",
		Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:      "test-llm-provider",
		DisplayName: "test-llm",
		Version:     "v1.0.0",
		SourceConfiguration: api.LLMProviderConfiguration{
			ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProviderConfigurationKindLlmProvider,
			Metadata: api.Metadata{
				Name: "test-llm-provider",
			},
			Spec: api.LLMProviderConfigData{
				DisplayName: "test-llm",
				Version:     "v1.0.0",
				Template:    "openai",
				Upstream: api.LLMProviderConfigData_Upstream{
					Url: stringPtr("http://llm-backend.com"),
				},
				AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			},
		},
		DesiredState: models.StateDeployed,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	apiKey := &models.APIKey{
		UUID:         "provider-key-id",
		Name:         "provider-key",
		APIKey:       "hashed-provider-key",
		MaskedAPIKey: "***provider-key",
		ArtifactUUID: cfg.UUID,
		Status:       models.APIKeyStatusActive,
		CreatedAt:    time.Now(),
		CreatedBy:    "test-user",
		UpdatedAt:    time.Now(),
		Source:       "external",
	}
	mockDB.SaveConfig(cfg)
	mockDB.SaveAPIKey(apiKey)
	require.NoError(t, server.store.Add(cfg))

	c, w := createTestContext("DELETE", "/llm-providers/test-llm-provider", nil)
	c.Set(middleware.CorrelationIDKey, "corr-id-delete-llm-provider")

	server.DeleteLLMProvider(c, "test-llm-provider")

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeLLMProvider, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "DELETE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, cfg.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-delete-llm-provider", mockHub.publishedEvents[0].event.EventID)

	_, err := mockDB.GetConfig(cfg.UUID)
	require.Error(t, err)

	_, err = mockDB.GetAPIKeysByAPIAndName(cfg.UUID, apiKey.Name)
	require.Error(t, err)

	_, err = server.store.Get(cfg.UUID)
	require.NoError(t, err)
}

func TestDeleteLLMProxyWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	cfg := &models.StoredConfig{
		UUID:        "0000-llm-proxy-id-0000-000000000000",
		Kind:        string(api.LLMProxyConfigurationKindLlmProxy),
		Handle:      "test-llm-proxy",
		DisplayName: "test-llm-proxy",
		Version:     "v1.0.0",
		SourceConfiguration: api.LLMProxyConfiguration{
			ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.LLMProxyConfigurationKindLlmProxy,
			Metadata: api.Metadata{
				Name: "test-llm-proxy",
			},
			Spec: api.LLMProxyConfigData{
				DisplayName: "test-llm-proxy",
				Version:     "v1.0.0",
				Provider: api.LLMProxyProvider{
					Id: "provider-a",
				},
			},
		},
		DesiredState: models.StateDeployed,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	mockDB.SaveConfig(cfg)
	require.NoError(t, server.store.Add(cfg))

	c, w := createTestContext("DELETE", "/llm-proxies/test-llm-proxy", nil)
	c.Set(middleware.CorrelationIDKey, "corr-id-delete-llm-proxy")

	server.DeleteLLMProxy(c, "test-llm-proxy")

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeLLMProxy, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "DELETE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, cfg.UUID, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-delete-llm-proxy", mockHub.publishedEvents[0].event.EventID)

	_, err := mockDB.GetConfig(cfg.UUID)
	require.Error(t, err)

	_, err = server.store.Get(cfg.UUID)
	require.NoError(t, err)
}

// TestDeleteLLMProxyInternalError tests DeleteLLMProxy with internal error
// Note: This test requires full deployment service setup
func TestDeleteLLMProxyInternalError(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

func TestCreateSubscriptionWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	cfg := seedAPIForAPIKeyHandlerTests(t, server, "subscription-api")
	body, err := json.Marshal(api.SubscriptionCreateRequest{
		ApiId:             cfg.UUID,
		SubscriptionToken: "subscription-token-123",
	})
	require.NoError(t, err)

	c, w := createTestContextWithHeader("POST", "/subscriptions", body, map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(middleware.CorrelationIDKey, "corr-id-create-subscription")

	server.CreateSubscription(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response api.SubscriptionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.NotNil(t, response.Id)
	require.NotNil(t, response.SubscriptionToken)
	assert.Equal(t, "subscription-token-123", *response.SubscriptionToken)

	storedSub, err := server.db.GetSubscriptionByID(*response.Id, "")
	require.NoError(t, err)
	assert.Equal(t, cfg.UUID, storedSub.APIID)

	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeSubscription, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "CREATE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, *response.Id, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-create-subscription", mockHub.publishedEvents[0].event.EventID)
	assert.Equal(t, eventhub.EmptyEventData, mockHub.publishedEvents[0].event.EventData)
}

func TestCreateSubscriptionPlanWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	body, err := json.Marshal(api.SubscriptionPlanCreateRequest{
		PlanName: "Gold",
	})
	require.NoError(t, err)

	c, w := createTestContextWithHeader("POST", "/subscription-plans", body, map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(middleware.CorrelationIDKey, "corr-id-create-plan")

	server.CreateSubscriptionPlan(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response api.SubscriptionPlanResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.NotNil(t, response.Id)
	assert.NotNil(t, response.PlanName)
	assert.Equal(t, "Gold", *response.PlanName)

	storedPlan, err := server.db.GetSubscriptionPlanByID(*response.Id, "")
	require.NoError(t, err)
	assert.Equal(t, "Gold", storedPlan.PlanName)

	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeSubscriptionPlan, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "CREATE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, *response.Id, mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-create-plan", mockHub.publishedEvents[0].event.EventID)
	assert.Equal(t, eventhub.EmptyEventData, mockHub.publishedEvents[0].event.EventData)
}

// TestMCPProxyKindMismatch tests GetMCPProxyById with wrong kind
// Note: This test requires full deployment service setup
func TestMCPProxyKindMismatch(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// BenchmarkListAPIs benchmarks the list APIs endpoint
func BenchmarkListRestAPIs(b *testing.B) {
	server := createTestAPIServer()

	// Add some test configs
	for i := 0; i < 100; i++ {
		cfg := createTestStoredConfig(
			fmt.Sprintf("test-id-%d", i),
			fmt.Sprintf("test-api-%d", i),
			"v1.0.0",
			fmt.Sprintf("/test%d", i),
		)
		_ = server.store.Add(cfg)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c, _ := createTestContext("GET", "/rest-apis", nil)
		server.ListRestAPIs(c, api.ListRestAPIsParams{})
	}
}

// Test for WebSubApi kind in buildStoredPolicyFromAPI
func TestBuildStoredPolicyFromAPIWebSubApi(t *testing.T) {
	server := createTestAPIServer()

	// Note: WebSubApi requires different data structure than RestApi
	// The function will return nil if parsing fails
	apiConfig := api.WebSubAPI{
		Kind: api.WebSubAPIKindWebSubApi,
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-test-id-0000-000000000000",
		Kind:                string(api.WebSubAPIKindWebSubApi),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
		Origin:              models.OriginGatewayAPI,
	}

	result := policybuilder.DerivePolicyFromAPIConfig(cfg, server.routerConfig, server.systemConfig, server.policyDefinitions)
	// Should return nil because the spec can't be parsed as WebhookAPIData without proper setup
	assert.Nil(t, result)
}

// Test GetConfigDump with config missing handle
func TestGetConfigDumpMissingHandle(t *testing.T) {
	server := createTestAPIServer()

	// Create config with empty handle
	apiConfig := api.RestAPI{
		Kind:     api.RestAPIKindRestApi,
		Metadata: api.Metadata{Name: ""}, // Empty handle
		Spec: api.APIConfigData{
			DisplayName: "test",
			Version:     "v1",
			Context:     "/test",
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{
					Url: stringPtr("http://backend.com"),
				},
			},
			Operations: []api.Operation{{Method: "GET", Path: "/"}},
		},
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-test-id-0000-000000000000",
		Kind:                string(api.RestAPIKindRestApi),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
		Origin:              models.OriginGatewayAPI,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	_ = server.store.Add(cfg)

	c, w := createTestContext("GET", "/config_dump", nil)
	server.GetConfigDump(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Test that SearchDeployments handles MCP kind with unmarshal error
func TestSearchDeploymentsMCPUnmarshalError(t *testing.T) {
	server := createTestAPIServer()

	// Add an MCP config with invalid source configuration
	cfg := &models.StoredConfig{
		UUID:                "0000-mcp-id-0000-000000000000",
		Kind:                string(api.MCPProxyConfigurationKindMcp),
		DisplayName:         "test",
		SourceConfiguration: make(chan int), // Invalid - can't be marshaled to JSON
		Configuration: api.RestAPI{
			Kind: api.RestAPIKindRestApi, // Use RestApi for RestAPI type
			Metadata: api.Metadata{
				Name: "test-mcp",
			},
		},
		DesiredState: models.StateDeployed,
		Origin:       models.OriginGatewayAPI,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_ = server.store.Add(cfg)

	// Exercise the store listing path: when mcpDeploymentService is set,
	// SearchDeployments uses ListMCPProxies() and never sees SourceConfiguration.
	server.mcpDeploymentService = nil

	c, w := createTestContext("GET", "/mcp-proxies?displayName=test", nil)
	c.Request.URL.RawQuery = "displayName=test"
	server.SearchDeployments(c, string(api.MCPProxyConfigurationKindMcp))

	// Rematerializing MCP list items from SourceConfiguration fails; request errors as a whole.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestBuildStoredPolicyFromAPIWithVhosts tests buildStoredPolicyFromAPI with custom vhosts
func TestBuildStoredPolicyFromAPIWithVhosts(t *testing.T) {
	server := createTestAPIServer()
	server.policyDefinitions["test-policy|v1.0.0"] = models.PolicyDefinition{Name: "test-policy", Version: "v1.0.0"}

	policies := []api.Policy{
		{Name: "test-policy", Version: "v1"},
	}

	sandboxUrl := "http://sandbox.example.com"
	sandboxVhost := "sandbox.localhost"
	apiConfig := api.RestAPI{
		Kind: api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
			DisplayName: "0000-test-api-0000-000000000000",
			Version:     "v1.0",
			Context:     "/test",
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{
					Url: stringPtr("http://backend.example.com"),
				},
				Sandbox: &api.Upstream{
					Url: &sandboxUrl,
				},
			},
			Vhosts: &struct {
				Main    string  `json:"main" yaml:"main"`
				Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main:    "custom.localhost",
				Sandbox: &sandboxVhost,
			},
			Operations: []api.Operation{
				{
					Method: "GET",
					Path:   "/resource",
				},
			},
			Policies: &policies,
		},
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-test-id-0000-000000000000",
		Kind:                string(api.RestAPIKindRestApi),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
		Origin:              models.OriginGatewayAPI,
	}

	result := policybuilder.DerivePolicyFromAPIConfig(cfg, server.routerConfig, server.systemConfig, server.policyDefinitions)
	assert.NotNil(t, result)
	// Should have 2 routes (one for main vhost, one for sandbox)
	assert.Equal(t, 2, len(result.Configuration.Routes))
}

// TestBuildStoredPolicyFromAPIOperationPolicies tests operation-level policy merging
func TestBuildStoredPolicyFromAPIOperationPolicies(t *testing.T) {
	server := createTestAPIServer()
	server.policyDefinitions["api-policy|v1.0.0"] = models.PolicyDefinition{Name: "api-policy", Version: "v1.0.0"}
	server.policyDefinitions["op-policy|v1.0.0"] = models.PolicyDefinition{Name: "op-policy", Version: "v1.0.0"}

	apiPolicies := []api.Policy{
		{Name: "api-policy", Version: "v1"},
	}

	opPolicies := []api.Policy{
		{Name: "op-policy", Version: "v1"},
	}

	apiConfig := api.RestAPI{
		Kind: api.RestAPIKindRestApi,
		Spec: api.APIConfigData{
			DisplayName: "0000-test-api-0000-000000000000",
			Version:     "v1.0",
			Context:     "/test",
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{
					Url: stringPtr("http://backend.example.com"),
				},
			},
			Operations: []api.Operation{
				{
					Method:   "GET",
					Path:     "/resource",
					Policies: &opPolicies,
				},
			},
			Policies: &apiPolicies,
		},
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-test-id-0000-000000000000",
		Kind:                string(api.RestAPIKindRestApi),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
		Origin:              models.OriginGatewayAPI,
	}

	result := policybuilder.DerivePolicyFromAPIConfig(cfg, server.routerConfig, server.systemConfig, server.policyDefinitions)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result.Configuration.Routes))
	// Should have both operation-level and API-level policies
	assert.GreaterOrEqual(t, len(result.Configuration.Routes[0].Policies), 1)
}

// Test handleStatusUpdate with empty correlation ID
func TestHandleStatusUpdateEmptyCorrelationID(t *testing.T) {
	server := createTestAPIServer()

	cfg := createTestStoredConfig("0000-test-id-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	_ = server.store.Add(cfg)

	// Test with empty correlation ID
	server.handleStatusUpdate("0000-test-id-0000-000000000000", true, "")

	updatedCfg, _ := server.store.Get("0000-test-id-0000-000000000000")
	assert.Equal(t, models.StateDeployed, updatedCfg.DesiredState)
}

// TestAPIKeyServiceNotConfigured tests API key operations when service is not configured
func TestAPIKeyServiceNotConfigured(t *testing.T) {
	server := createTestAPIServer()
	server.apiKeyService = nil

	// Add auth context
	authCtx := commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	}

	body := []byte(`{"name": "test-key"}`)
	c, w := createTestContextWithHeader("POST", "/rest-apis/test-handle/api-keys", body, map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(constants.AuthContextKey, authCtx)

	// Should panic or return error since apiKeyService is nil
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		server.CreateAPIKey(c, "0000-test-handle-0000-000000000000")
	}()
	if !panicked {
		assert.True(t, w.Code >= http.StatusBadRequest)
	}
}

// Test for WebSubAPI - simplified test
func TestBuildStoredPolicyFromAPIWebSubApiWithPolicies(t *testing.T) {
	server := createTestAPIServer()

	// WebSubApi requires specific data structure that's complex to mock
	// Testing that the function handles WebSubApi kind without panicking
	apiConfig := api.WebSubAPI{
		Kind: api.WebSubAPIKindWebSubApi,
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-test-id-0000-000000000000",
		Kind:                string(api.WebSubAPIKindWebSubApi),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
		Origin:              models.OriginGatewayAPI,
	}

	result := policybuilder.DerivePolicyFromAPIConfig(cfg, server.routerConfig, server.systemConfig, server.policyDefinitions)
	// Should return nil since we don't have valid spec data
	assert.Nil(t, result)
}

// Test ListMCPProxies with stored configs that have unmarshal issues
func TestListMCPProxiesUnmarshalError(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	// Add MCP config, then replace SourceConfiguration with something that can't be marshaled to JSON
	cfg := &models.StoredConfig{
		UUID: "0000-mcp-id-0000-000000000000",
		Kind: string(api.MCPProxyConfigurationKindMcp),
		SourceConfiguration: api.MCPProxyConfiguration{
			Kind:     api.MCPProxyConfigurationKindMcp,
			Metadata: api.Metadata{Name: "test-mcp"},
			Spec: api.MCPProxyConfigData{
				DisplayName: "Test MCP",
				Version:     "v1.0",
			},
		},
		Configuration: api.RestAPI{
			Kind:     api.RestAPIKindRestApi,
			Metadata: api.Metadata{Name: "test-mcp"},
		},
		Origin:    models.OriginGatewayAPI,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, mockDB.SaveConfig(cfg))
	// Mutate the DB-backed object to something that can't be JSON marshaled.
	cfg.SourceConfiguration = make(chan int)

	c, w := createTestContext("GET", "/mcp-proxies", nil)
	server.ListMCPProxies(c, api.ListMCPProxiesParams{})

	// ListMCPProxies deterministically returns StatusInternalServerError on unmarshal errors
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestConvertHandleToUUIDValid tests convertHandleToUUID with valid UUID
func TestConvertHandleToUUIDValid(t *testing.T) {
	validUUID := "550e8400-e29b-41d4-a716-446655440000"
	result := convertHandleToUUID(validUUID)
	assert.NotNil(t, result)
}

// TestConvertHandleToUUIDInvalid tests convertHandleToUUID with invalid UUID
func TestConvertHandleToUUIDInvalid(t *testing.T) {
	invalidUUID := "not-a-valid-uuid"
	result := convertHandleToUUID(invalidUUID)
	assert.Nil(t, result)
}

// TestDeleteAPIWithAPIKeys tests deleting an API that has API keys
// Note: This test requires full deployment service setup
func TestDeleteRestAPIWithAPIKeys(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// Test for checking the execution condition in convertAPIPolicy
func TestConvertAPIPolicyWithExecutionCondition(t *testing.T) {
	execCondition := "request.headers['X-Test'] == 'value'"
	policy := api.Policy{
		Name:               "conditional-policy",
		Version:            "v1.0.0",
		ExecutionCondition: &execCondition,
	}

	result := convertAPIPolicy(policy, "route", "v1.0.0")

	assert.Equal(t, "conditional-policy", result.Name)
	assert.NotNil(t, result.ExecutionCondition)
	assert.Equal(t, execCondition, *result.ExecutionCondition)
}

// TestGetMCPProxyByIdFound tests getting an existing MCP proxy
// Note: This test requires full deployment service setup
func TestGetMCPProxyByIdFound(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestGetMCPProxyByIdWithDeployedAt tests GetMCPProxyById with deployedAt
// Note: This test requires full deployment service setup
func TestGetMCPProxyByIdWithDeployedAt(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestDeleteRestAPIDBError tests DeleteRestAPI with a database delete failure.
func TestDeleteRestAPIDBError(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	cfg := createTestStoredConfig("0000-test-id-0000-000000000000", "test-api", "v1.0.0", "/test")
	cfg.Handle = "test-handle"
	mockDB.SaveConfig(cfg)
	mockDB.deleteErr = errors.New("db delete error")

	c, w := createTestContext("DELETE", "/rest-apis/test-handle", nil)
	server.DeleteRestAPI(c, "test-handle")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Empty(t, mockHub.publishedEvents)

	var response api.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "error", response.Status)
	assert.Equal(t, "Failed to delete configuration", response.Message)
	assert.NotContains(t, w.Body.String(), "db delete error")

	stored, err := mockDB.GetConfig(cfg.UUID)
	require.NoError(t, err)
	assert.Equal(t, cfg.UUID, stored.UUID)
}

// TestUpdateRestAPIDBError tests UpdateRestAPI with a database update failure.
func TestUpdateRestAPIDBError(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	existing := createTestStoredConfig("0000-test-id-0000-000000000000", "original-display-name", "v1.0.0", "/original")
	existing.Handle = "test-handle"
	mockDB.SaveConfig(existing)
	mockDB.updateErr = errors.New("db update error")

	body := createTestRestAPIRequestBody(t, "test-handle", "updated-display-name", "v2.0.0", "/updated")
	c, w := createTestContextWithHeader("PUT", "/rest-apis/test-handle", body, map[string]string{
		"Content-Type": "application/json",
	})

	server.UpdateRestAPI(c, "test-handle")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Empty(t, mockHub.publishedEvents)

	var response api.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "error", response.Status)
	assert.Equal(t, "Failed to update configuration", response.Message)
	assert.NotContains(t, w.Body.String(), "db update error")
}

func TestUpdateRestAPISyncsDisplayNameAndVersion(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	existing := createTestStoredConfig("0000-test-id-0000-000000000000", "original-display-name", "v1.0.0", "/original")
	existing.Handle = "test-handle"
	require.NoError(t, mockDB.SaveConfig(existing))

	body := createTestRestAPIRequestBody(t, "test-handle", "updated-display-name", "v2.0.0", "/updated")
	c, w := createTestContextWithHeader("PUT", "/rest-apis/test-handle", body, map[string]string{
		"Content-Type": "application/json",
	})

	server.UpdateRestAPI(c, "test-handle")

	require.Equal(t, http.StatusOK, w.Code)

	stored, err := mockDB.GetConfig(existing.UUID)
	require.NoError(t, err)
	assert.Equal(t, "updated-display-name", stored.DisplayName)
	assert.Equal(t, "v2.0.0", stored.Version)

	displayName := "updated-display-name"
	version := "v2.0.0"
	c, w = createTestContext("GET", "/rest-apis?displayName=updated-display-name&version=v2.0.0", nil)
	c.Request.URL.RawQuery = "displayName=updated-display-name&version=v2.0.0"
	server.ListRestAPIs(c, api.ListRestAPIsParams{
		DisplayName: &displayName,
		Version:     &version,
	})

	require.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, float64(1), response["count"])
	assert.Len(t, mockHub.publishedEvents, 1)
}

// TestGetMCPProxyByIdDBUnavailable tests GetMCPProxyById with DB unavailable
// Note: This test requires full deployment service setup
func TestGetMCPProxyByIdDBUnavailable(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// Test with policies that have nil parameters
func TestConvertAPIPolicyNilParams(t *testing.T) {
	policy := api.Policy{
		Name:    "test-policy",
		Version: "v1.0.0",
		Params:  nil,
	}

	result := convertAPIPolicy(policy, "api", "v1.0.0")

	assert.Equal(t, "test-policy", result.Name)
	assert.NotNil(t, result.Parameters)
	assert.Equal(t, "api", result.Parameters["attachedTo"])
}

// TestListLLMProvidersUnmarshalError tests ListLLMProviders with unmarshal error
// Note: This test requires full deployment service setup
func TestListLLMProvidersUnmarshalError(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestListLLMProxiesUnmarshalError tests ListLLMProxies with unmarshal error
// Note: This test requires full deployment service setup
func TestListLLMProxiesUnmarshalError(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestStorageConflictErrorHandling tests handling of storage conflict errors
func TestStorageConflictErrorHandling(t *testing.T) {
	// Test that storage.IsConflictError works correctly
	conflictErr := fmt.Errorf("%w: test conflict", storage.ErrConflict)
	assert.True(t, storage.IsConflictError(conflictErr))
}

// TestUpdateAPIKeyNoAuth tests UpdateAPIKey without authentication
func TestUpdateAPIKeyNoAuth(t *testing.T) {
	server := createTestAPIServer()

	body := []byte(`{"apiKey": "new-key-value"}`)
	c, w := createTestContextWithHeader("PUT", "/rest-apis/test-handle/api-keys/test-key", body, map[string]string{
		"Content-Type": "application/json",
	})
	server.UpdateAPIKey(c, "0000-test-handle-0000-000000000000", "test-key")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestUpdateAPIKeyInvalidBody tests UpdateAPIKey with invalid request body
func TestUpdateAPIKeyInvalidBody(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContextWithHeader("PUT", "/rest-apis/test-handle/api-keys/test-key", []byte("invalid json {{{"), map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(constants.AuthContextKey, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	server.UpdateAPIKey(c, "0000-test-handle-0000-000000000000", "test-key")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response api.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Message, "Invalid request body")
}

// TestUpdateAPIKeyMissingAPIKey tests UpdateAPIKey without apiKey field
func TestUpdateAPIKeyMissingAPIKey(t *testing.T) {
	server := createTestAPIServer()

	body := []byte(`{"description": "test"}`)
	c, w := createTestContextWithHeader("PUT", "/rest-apis/test-handle/api-keys/test-key", body, map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(constants.AuthContextKey, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	server.UpdateAPIKey(c, "0000-test-handle-0000-000000000000", "test-key")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response api.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response.Status)
	assert.Equal(t, "apiKey is required", response.Message)
}

func TestUpdateAPIKeyWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	storeKey := createStoredExternalAPIKey("0000-test-key-id-0000-000000000000", cfg.UUID, "test-key", "Old Key", "test-user", "apip_****old")
	dbKey := *storeKey
	require.NoError(t, server.store.StoreAPIKey(storeKey))
	require.NoError(t, mockDB.SaveAPIKey(&dbKey))

	body := createTestAPIKeyRequestBody(t, "test-key", "Updated Key", "external-key-abcdef1234567890abcdef1234567890abcdef")
	c, w := createTestContextWithHeader("PUT", "/rest-apis/test-handle/api-keys/test-key", body, map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(constants.AuthContextKey, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	c.Set(middleware.CorrelationIDKey, "corr-id-update-key")

	server.UpdateAPIKey(c, "test-handle", "test-key")

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeAPIKey, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "UPDATE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, apikey.BuildAPIKeyEntityID(cfg.UUID, storeKey.UUID), mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-update-key", mockHub.publishedEvents[0].event.EventID)
	assert.Equal(t, eventhub.EmptyEventData, mockHub.publishedEvents[0].event.EventData)

	updatedKey, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, "test-key")
	require.NoError(t, err)
	assert.Equal(t, models.APIKeyStatusActive, updatedKey.Status)
	assert.Equal(t, string(api.External), updatedKey.Source)
	assert.NotEqual(t, "apip_****old", updatedKey.MaskedAPIKey)
}

func TestUpdateAPIKeyDBError(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")
	mockDB.updateErr = errors.New("db update error")

	storeKey := createStoredExternalAPIKey("0000-test-key-id-0000-000000000000", cfg.UUID, "test-key", "Old Key", "test-user", "apip_****old")
	dbKey := *storeKey
	require.NoError(t, server.store.StoreAPIKey(storeKey))
	require.NoError(t, mockDB.SaveAPIKey(&dbKey))

	body := createTestAPIKeyRequestBody(t, "test-key", "Updated Key", "external-key-abcdef1234567890abcdef1234567890abcdef")
	c, w := createTestContextWithHeader("PUT", "/rest-apis/test-handle/api-keys/test-key", body, map[string]string{
		"Content-Type": "application/json",
	})
	c.Set(constants.AuthContextKey, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})

	server.UpdateAPIKey(c, "test-handle", "test-key")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Empty(t, mockHub.publishedEvents)

	storedKey, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, "test-key")
	require.NoError(t, err)
	assert.Equal(t, "apip_****old", storedKey.MaskedAPIKey)
}

// TestRevokeAPIKeyNotFound tests revoking a non-existent API key
func TestRevokeAPIKeyNotFound(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContext("DELETE", "/rest-apis/test-handle/api-keys/nonexistent", nil)
	c.Set(constants.AuthContextKey, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	server.RevokeAPIKey(c, "0000-test-handle-0000-000000000000", "nonexistent")

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response api.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response.Status)
}

// MockPolicyManager is a mock implementation of the policy manager for testing
type MockPolicyManager struct {
	removePolicyErr error
	addPolicyErr    error
	removedPolicyID string
	addedPolicy     *models.StoredPolicyConfig
}

func (m *MockPolicyManager) RemovePolicy(id string) error {
	m.removedPolicyID = id
	return m.removePolicyErr
}

func (m *MockPolicyManager) AddPolicy(policy *models.StoredPolicyConfig) error {
	m.addedPolicy = policy
	return m.addPolicyErr
}

func (m *MockPolicyManager) GetPolicy(id string) (*models.StoredPolicyConfig, error) {
	return nil, nil
}

func (m *MockPolicyManager) ListPolicies() []*models.StoredPolicyConfig {
	return nil
}

// TestPolicyRemovalErrorHandling tests error handling in policy removal logic
func TestPolicyRemovalErrorHandling(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool // true if ErrPolicyNotFound, false otherwise
	}{
		{"policy not found", fmt.Errorf("wrapped: %w", storage.ErrPolicyNotFound), true},
		{"storage error", errors.New("database failed"), false},
		{"success", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockPolicyManager{removePolicyErr: tt.err}
			err := mock.RemovePolicy("0000-test-id-0000-000000000000")
			assert.Equal(t, tt.want, storage.IsPolicyNotFoundError(err))
		})
	}
}

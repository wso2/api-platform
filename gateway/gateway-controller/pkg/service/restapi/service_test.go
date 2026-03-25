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

package restapi

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

type restAPIPublishedEvent struct {
	gatewayID string
	event     eventhub.Event
}

type mockRestAPIEventHub struct {
	publishedEvents []restAPIPublishedEvent
}

func (m *mockRestAPIEventHub) Initialize() error            { return nil }
func (m *mockRestAPIEventHub) RegisterGateway(string) error { return nil }
func (m *mockRestAPIEventHub) PublishEvent(gatewayID string, event eventhub.Event) error {
	m.publishedEvents = append(m.publishedEvents, restAPIPublishedEvent{gatewayID: gatewayID, event: event})
	return nil
}
func (m *mockRestAPIEventHub) Subscribe(string) (<-chan eventhub.Event, error) { return nil, nil }
func (m *mockRestAPIEventHub) Unsubscribe(string, <-chan eventhub.Event) error { return nil }
func (m *mockRestAPIEventHub) UnsubscribeAll(string) error                     { return nil }
func (m *mockRestAPIEventHub) CleanUpEvents() error                            { return nil }
func (m *mockRestAPIEventHub) Close() error                                    { return nil }

type mockRestAPIControlPlaneClient struct {
	connected  bool
	pushes     int
	lastAPIID  string
	lastConfig *models.StoredConfig
	lastRev    string
}

func (m *mockRestAPIControlPlaneClient) IsConnected() bool { return m.connected }

func (m *mockRestAPIControlPlaneClient) PushAPIDeployment(apiID string, apiConfig *models.StoredConfig, deploymentID string) error {
	m.pushes++
	m.lastAPIID = apiID
	m.lastConfig = apiConfig
	m.lastRev = deploymentID
	return nil
}

type mockRestAPIStorage struct {
	configs map[string]*models.StoredConfig
}

func newMockRestAPIStorage() *mockRestAPIStorage {
	return &mockRestAPIStorage{configs: make(map[string]*models.StoredConfig)}
}

func (m *mockRestAPIStorage) SaveConfig(cfg *models.StoredConfig) error {
	m.configs[cfg.UUID] = cfg
	return nil
}

func (m *mockRestAPIStorage) UpdateConfig(cfg *models.StoredConfig) error {
	m.configs[cfg.UUID] = cfg
	return nil
}

func (m *mockRestAPIStorage) DeleteConfig(id string) error {
	delete(m.configs, id)
	return nil
}

func (m *mockRestAPIStorage) GetConfig(id string) (*models.StoredConfig, error) {
	if cfg, ok := m.configs[id]; ok {
		return cfg, nil
	}
	return nil, storage.ErrNotFound
}

func (m *mockRestAPIStorage) GetConfigByKindAndHandle(kind, handle string) (*models.StoredConfig, error) {
	for _, cfg := range m.configs {
		if cfg.Kind == kind && cfg.Handle == handle {
			return cfg, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *mockRestAPIStorage) GetAllConfigs() ([]*models.StoredConfig, error) {
	result := make([]*models.StoredConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		result = append(result, cfg)
	}
	return result, nil
}

func (m *mockRestAPIStorage) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	result := make([]*models.StoredConfig, 0)
	for _, cfg := range m.configs {
		if cfg.Kind == kind {
			result = append(result, cfg)
		}
	}
	return result, nil
}

func (m *mockRestAPIStorage) SaveAPIKey(*models.APIKey) error { return nil }
func (m *mockRestAPIStorage) GetAPIKeyByID(string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}
func (m *mockRestAPIStorage) GetAPIKeyByUUID(string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}
func (m *mockRestAPIStorage) GetAPIKeyByKey(string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}
func (m *mockRestAPIStorage) GetAPIKeysByAPI(string) ([]*models.APIKey, error) { return nil, nil }
func (m *mockRestAPIStorage) GetAllAPIKeys() ([]*models.APIKey, error)         { return nil, nil }
func (m *mockRestAPIStorage) GetAPIKeysByAPIAndName(string, string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}
func (m *mockRestAPIStorage) UpdateAPIKey(*models.APIKey) error           { return nil }
func (m *mockRestAPIStorage) DeleteAPIKey(string) error                   { return nil }
func (m *mockRestAPIStorage) RemoveAPIKeysAPI(string) error               { return nil }
func (m *mockRestAPIStorage) RemoveAPIKeyAPIAndName(string, string) error { return nil }
func (m *mockRestAPIStorage) CountActiveAPIKeysByUserAndAPI(string, string) (int, error) {
	return 0, nil
}
func (m *mockRestAPIStorage) SaveSubscriptionPlan(*models.SubscriptionPlan) error { return nil }
func (m *mockRestAPIStorage) GetSubscriptionPlanByID(string, string) (*models.SubscriptionPlan, error) {
	return nil, storage.ErrNotFound
}
func (m *mockRestAPIStorage) ListSubscriptionPlans(string) ([]*models.SubscriptionPlan, error) {
	return nil, nil
}
func (m *mockRestAPIStorage) UpdateSubscriptionPlan(*models.SubscriptionPlan) error { return nil }
func (m *mockRestAPIStorage) DeleteSubscriptionPlan(string, string) error           { return nil }
func (m *mockRestAPIStorage) DeleteSubscriptionPlansNotIn([]string) error           { return nil }
func (m *mockRestAPIStorage) SaveSubscription(*models.Subscription) error           { return nil }
func (m *mockRestAPIStorage) GetSubscriptionByID(string, string) (*models.Subscription, error) {
	return nil, storage.ErrNotFound
}
func (m *mockRestAPIStorage) ListSubscriptionsByAPI(string, string, *string, *string) ([]*models.Subscription, error) {
	return nil, nil
}
func (m *mockRestAPIStorage) ListActiveSubscriptions() ([]*models.Subscription, error) {
	return nil, nil
}
func (m *mockRestAPIStorage) UpdateSubscription(*models.Subscription) error         { return nil }
func (m *mockRestAPIStorage) DeleteSubscription(string, string) error               { return nil }
func (m *mockRestAPIStorage) DeleteSubscriptionsForAPINotIn(string, []string) error { return nil }
func (m *mockRestAPIStorage) ReplaceApplicationAPIKeyMappings(*models.StoredApplication, []*models.ApplicationAPIKeyMapping) error {
	return nil
}
func (m *mockRestAPIStorage) SaveCertificate(*models.StoredCertificate) error { return nil }
func (m *mockRestAPIStorage) GetCertificate(string) (*models.StoredCertificate, error) {
	return nil, storage.ErrNotFound
}
func (m *mockRestAPIStorage) GetCertificateByName(string) (*models.StoredCertificate, error) {
	return nil, storage.ErrNotFound
}
func (m *mockRestAPIStorage) ListCertificates() ([]*models.StoredCertificate, error) { return nil, nil }
func (m *mockRestAPIStorage) DeleteCertificate(string) error                         { return nil }
func (m *mockRestAPIStorage) SaveLLMProviderTemplate(*models.StoredLLMProviderTemplate) error {
	return nil
}
func (m *mockRestAPIStorage) UpdateLLMProviderTemplate(*models.StoredLLMProviderTemplate) error {
	return nil
}
func (m *mockRestAPIStorage) DeleteLLMProviderTemplate(string) error { return nil }
func (m *mockRestAPIStorage) GetLLMProviderTemplate(string) (*models.StoredLLMProviderTemplate, error) {
	return nil, storage.ErrNotFound
}
func (m *mockRestAPIStorage) GetAllLLMProviderTemplates() ([]*models.StoredLLMProviderTemplate, error) {
	return nil, nil
}
func (m *mockRestAPIStorage) GetDB() *sql.DB { return nil }
func (m *mockRestAPIStorage) Close() error   { return nil }

func newTestRestAPIService(t *testing.T) (*RestAPIService, *mockRestAPIStorage, *storage.ConfigStore, *mockRestAPIEventHub, *mockRestAPIControlPlaneClient) {
	t.Helper()

	store := storage.NewConfigStore()
	db := newMockRestAPIStorage()
	hub := &mockRestAPIEventHub{}
	cpClient := &mockRestAPIControlPlaneClient{connected: true}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	routerConfig := &config.RouterConfig{
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "api.example.com"},
			Sandbox: config.VHostEntry{Default: "sandbox.example.com"},
		},
	}
	systemConfig := &config.Config{
		Controller: config.Controller{
			Server: config.ServerConfig{
				GatewayID: "test-gateway",
			},
			ControlPlane: config.ControlPlaneConfig{
				DeploymentPushEnabled: true,
			},
		},
		Router: *routerConfig,
	}
	parser := config.NewParser()
	validator := config.NewAPIValidator()
	deploymentService := utils.NewAPIDeploymentService(store, db, nil, validator, routerConfig)
	deploymentService.SetEventHub(hub, "test-gateway")

	service := NewRestAPIService(
		store, db, nil, nil,
		nil, &sync.RWMutex{},
		deploymentService, nil, cpClient,
		routerConfig, systemConfig,
		&http.Client{Timeout: time.Second}, parser, validator, logger, hub,
	)

	return service, db, store, hub, cpClient
}

func TestUpdatePublishesReplicaSyncEvent(t *testing.T) {
	service, db, _, hub, _ := newTestRestAPIService(t)

	existing := &models.StoredConfig{
		UUID:         "api-123",
		Kind:         string(api.RestApi),
		Handle:       "test-api",
		DisplayName:  "Original API",
		Version:      "1.0.0",
		DesiredState: models.StateDeployed,
		Configuration: api.RestAPI{
			ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata:   api.Metadata{Name: "test-api"},
		},
		SourceConfiguration: api.RestAPI{
			ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       api.RestApi,
			Metadata:   api.Metadata{Name: "test-api"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, db.SaveConfig(existing))

	body := []byte(`apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: test-api
spec:
  displayName: Updated API
  version: "1.0.0"
  context: /updated
  upstream:
    main:
      url: http://backend:8080
  operations:
    - method: GET
      path: /resource
`)

	result, err := service.Update(UpdateParams{
		Handle:        "test-api",
		Body:          body,
		ContentType:   "application/yaml",
		CorrelationID: "corr-update",
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, hub.publishedEvents, 1)
	require.Equal(t, eventhub.EventTypeAPI, hub.publishedEvents[0].event.EventType)
	require.Equal(t, "UPDATE", hub.publishedEvents[0].event.Action)
	require.Equal(t, "api-123", hub.publishedEvents[0].event.EntityID)
	require.Equal(t, "corr-update", hub.publishedEvents[0].event.EventID)
}

func TestWaitForDeploymentAndPushReadsDBWhenStoreIsStale(t *testing.T) {
	service, db, store, _, cpClient := newTestRestAPIService(t)

	deployedAt := time.Now()
	cfg := &models.StoredConfig{
		UUID:         "api-db-only",
		Kind:         string(api.RestApi),
		Handle:       "db-only",
		DisplayName:  "DB Only API",
		Version:      "1.0.0",
		DeploymentID: "rev-1",
		DesiredState: models.StateDeployed,
		DeployedAt:   &deployedAt,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, db.SaveConfig(cfg))

	_, err := store.Get(cfg.UUID)
	require.ErrorIs(t, err, storage.ErrNotFound)

	service.waitForDeploymentAndPush(cfg.UUID, "corr-push", slog.New(slog.NewTextHandler(io.Discard, nil)))

	require.Equal(t, 1, cpClient.pushes)
	require.Equal(t, cfg.UUID, cpClient.lastAPIID)
	require.Equal(t, "rev-1", cpClient.lastRev)
	require.NotNil(t, cpClient.lastConfig)
	require.Equal(t, cfg.UUID, cpClient.lastConfig.UUID)
}

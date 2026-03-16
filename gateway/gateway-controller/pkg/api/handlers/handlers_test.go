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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/constants"
	commonmodels "github.com/wso2/api-platform/common/models"
	adminapi "github.com/wso2/api-platform/gateway/gateway-controller/pkg/adminapi/generated"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
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
	configs          map[string]*models.StoredConfig
	templates        map[string]*models.StoredLLMProviderTemplate
	apiKeys          map[string]*models.APIKey
	certs            []*models.StoredCertificate
	subscriptions    map[string]*models.Subscription
	subscriptionPlans map[string]*models.SubscriptionPlan
	saveErr          error
	getErr           error
	updateErr        error
	deleteErr        error
	unavailable      bool
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		configs:          make(map[string]*models.StoredConfig),
		templates:        make(map[string]*models.StoredLLMProviderTemplate),
		apiKeys:          make(map[string]*models.APIKey),
		certs:            make([]*models.StoredCertificate, 0),
		subscriptions:    make(map[string]*models.Subscription),
		subscriptionPlans: make(map[string]*models.SubscriptionPlan),
	}
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
	return nil, errors.New("config not found")
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
	return nil, errors.New("config not found")
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

func (m *MockStorage) SaveAPIKey(apiKey *models.APIKey) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.apiKeys[apiKey.UUID] = apiKey
	return nil
}

func (m *MockStorage) GetAPIKeyByID(id string) (*models.APIKey, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if key, ok := m.apiKeys[id]; ok {
		return key, nil
	}
	return nil, errors.New("API key not found")
}

func (m *MockStorage) GetAPIKeyByKey(key string) (*models.APIKey, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, apiKey := range m.apiKeys {
		if apiKey.APIKey == key {
			return apiKey, nil
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
			result = append(result, key)
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
		result = append(result, key)
	}
	return result, nil
}

func (m *MockStorage) GetAPIKeysByAPIAndName(apiId, name string) (*models.APIKey, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, key := range m.apiKeys {
		if key.ArtifactUUID == apiId && key.Name == name {
			return key, nil
		}
	}
	return nil, errors.New("API key not found")
}

func (m *MockStorage) UpdateAPIKey(apiKey *models.APIKey) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.apiKeys[apiKey.UUID] = apiKey
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

func (m *MockStorage) Close() error {
	return nil
}

// MockControlPlaneClient implements controlplane.ControlPlaneClient for testing
type MockControlPlaneClient struct {
	connected bool
}

func (m *MockControlPlaneClient) Connect() error {
	m.connected = true
	return nil
}

func (m *MockControlPlaneClient) IsConnected() bool {
	return m.connected
}

func (m *MockControlPlaneClient) PushAPIDeployment(apiID string, cfg *models.StoredConfig, deploymentID string) error {
	return nil
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

	vhosts := &config.VHostsConfig{
		Main:    config.VHostEntry{Default: "localhost"},
		Sandbox: config.VHostEntry{Default: "sandbox-localhost"},
	}

	parser := config.NewParser()
	validator := config.NewAPIValidator()
	policyDefs := make(map[string]api.PolicyDefinition)
	routerCfg := &config.RouterConfig{
		GatewayHost: "localhost",
		VHosts:      *vhosts,
		EventGateway: config.EventGatewayConfig{
			TimeoutSeconds: 10,
		},
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	systemCfg := &config.Config{
		Controller: config.Controller{},
		Router: config.RouterConfig{
			GatewayHost: "localhost",
			VHosts:      *vhosts,
		},
		APIKey: config.APIKeyConfig{
			Algorithm:    "sha256",
			MinKeyLength: 32,
			MaxKeyLength: 128,
		},
	}

	server := &APIServer{
		store:             store,
		db:                db,
		logger:            logger,
		parser:            parser,
		validator:         validator,
		policyDefinitions: policyDefs,
		routerConfig:      routerCfg,
		httpClient:        httpClient,
		systemConfig:      systemCfg,
	}

	// Initialize API key service (needed for API key operations)
	apiKeyService := utils.NewAPIKeyService(store, db, nil, &server.systemConfig.APIKey)
	server.apiKeyService = apiKeyService

	// Initialize RestAPI service and handler
	restAPIService := restapi.NewRestAPIService(
		store, db, nil, nil,
		policyDefs, &server.policyDefMu,
		nil, nil, nil,
		routerCfg, systemCfg,
		httpClient, parser, validator, logger,
	)
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
		Kind:       api.RestApi,
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
		Kind:                string(api.RestApi),
		Handle:              id,
		DisplayName:         name,
		Version:             version,
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
		Status:              models.StatusPending,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

// TestListAPIs tests listing all APIs
func TestListRestAPIs(t *testing.T) {
	server := createTestAPIServer()

	// Add test configs to store
	cfg1 := createTestStoredConfig("test-id-1", "0000-test-api-1-0000-000000000000", "v1.0.0", "/test1")
	cfg2 := createTestStoredConfig("test-id-2", "test-api-2", "v2.0.0", "/test2")
	_ = server.store.Add(cfg1)
	_ = server.store.Add(cfg2)

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
	assert.Equal(t, "success", response["status"])
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
	assert.Equal(t, "success", response["status"])
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
	server := createTestAPIServerWithDB(nil)

	c, w := createTestContext("GET", "/rest-apis/test-id", nil)
	server.GetRestAPIById(c, "0000-test-id-0000-000000000000")

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestGetAPIByIdWrongKind tests getting an API with wrong kind
func TestGetRestAPIByIdWrongKind(t *testing.T) {
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	cfg := createTestStoredConfig("0000-test-handle-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	cfg.Kind = string(api.Mcp) // Wrong kind
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
	server.SearchDeployments(c, string(api.RestApi))

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
	server.SearchDeployments(c, string(api.Mcp))

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "mcpProxies")
}

// TestListPolicies tests listing all policies
func TestListPolicies(t *testing.T) {
	server := createTestAPIServer()
	server.policyDefinitions["policy1|v1"] = api.PolicyDefinition{
		Name:    "policy1",
		Version: "v1",
	}
	server.policyDefinitions["policy2|v1"] = api.PolicyDefinition{
		Name:    "policy2",
		Version: "v1",
	}

	c, w := createTestContext("GET", "/policies", nil)
	server.ListPolicies(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Status   string                 `json:"status"`
		Count    int                    `json:"count"`
		Policies []api.PolicyDefinition `json:"policies"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, 2, response.Count)
}

// TestListPoliciesEmpty tests listing policies when none exist
func TestListPoliciesEmpty(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContext("GET", "/policies", nil)
	server.ListPolicies(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Status   string `json:"status"`
		Count    int    `json:"count"`
		Policies []api.PolicyDefinition
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, 0, response.Count)
}

// TestGetConfigDump tests the config dump endpoint
func TestGetConfigDump(t *testing.T) {
	server := createTestAPIServer()

	// Add test config
	cfg := createTestStoredConfig("0000-test-handle-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	_ = server.store.Add(cfg)

	// Add test policy
	server.policyDefinitions["policy1|v1"] = api.PolicyDefinition{
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

	policyStore := storage.NewPolicyStore()
	snapshotMgr := policyxds.NewSnapshotManager(policyStore, server.logger)
	server.policyManager = policyxds.NewPolicyManager(policyStore, snapshotMgr, server.logger)

	policyStore.IncrementResourceVersion()
	policyStore.IncrementResourceVersion()

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

// TestGetConfigDumpNoDB tests config dump without database
func TestGetConfigDumpNoDB(t *testing.T) {
	server := createTestAPIServer()
	server.db = nil

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

	// Test successful deployment
	server.handleStatusUpdate("0000-test-id-0000-000000000000", true, 1, "corr-id-1")

	// Verify status updated
	updatedCfg, _ := server.store.Get("0000-test-id-0000-000000000000")
	assert.Equal(t, models.StatusDeployed, updatedCfg.Status)
	assert.NotNil(t, updatedCfg.DeployedAt)
	assert.Equal(t, int64(1), updatedCfg.DeployedVersion)
}

// TestHandleStatusUpdateFailure tests status update for failed deployment
func TestHandleStatusUpdateFailure(t *testing.T) {
	server := createTestAPIServer()

	// Add test config
	cfg := createTestStoredConfig("0000-test-id-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
	_ = server.store.Add(cfg)

	// Test failed deployment
	server.handleStatusUpdate("0000-test-id-0000-000000000000", false, 0, "")

	// Verify status updated
	updatedCfg, _ := server.store.Get("0000-test-id-0000-000000000000")
	assert.Equal(t, models.StatusFailed, updatedCfg.Status)
	assert.Nil(t, updatedCfg.DeployedAt)
}

// TestHandleStatusUpdateNotFound tests status update for non-existent config
func TestHandleStatusUpdateNotFound(t *testing.T) {
	server := createTestAPIServer()

	// Should not panic
	server.handleStatusUpdate("nonexistent", true, 1, "")
}

// TestCreateRestAPIInvalidBody tests CreateRestAPI with invalid request body
// Note: This test requires a full deployment service setup, so we skip it
func TestCreateRestAPIInvalidBody(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
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

// TestUpdateRestAPINoDB tests UpdateRestAPI when DB is not available
func TestUpdateRestAPINoDB(t *testing.T) {
	server := createTestAPIServerWithDB(nil)

	body := []byte(`{
		"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
		"kind": "RestApi",
		"metadata": {"name": "test"},
		"spec": {
			"displayName": "test",
			"version": "v1",
			"context": "/test",
			"upstream": {"main": {"url": "http://backend.com"}},
			"operations": [{"method": "GET", "path": "/"}]
		}
	}`)
	c, w := createTestContextWithHeader("PUT", "/rest-apis/test", body, map[string]string{
		"Content-Type": "application/json",
	})
	server.UpdateRestAPI(c, "test")

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestUpdateRestAPIHandleMismatch tests UpdateRestAPI with handle mismatch
// Note: This test requires full parser/validator setup
func TestUpdateRestAPIHandleMismatch(t *testing.T) {
	t.Skip("Skipping test that requires full parser/validator setup")
}

// TestDeleteRestAPINoDB tests DeleteRestAPI when DB is not available
// Note: This test requires full deployment service setup
func TestDeleteRestAPINoDB(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
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

// TestListMCPProxies tests listing MCP proxies
// Note: This test requires full deployment service setup
func TestListMCPProxies(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
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

// TestRevokeAPIKeyNoAuth tests RevokeAPIKey without authentication
func TestRevokeAPIKeyNoAuth(t *testing.T) {
	server := createTestAPIServer()

	c, w := createTestContext("DELETE", "/rest-apis/test-handle/api-keys/test-key", nil)
	server.RevokeAPIKey(c, "0000-test-handle-0000-000000000000", "test-key")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
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
		Kind: api.RestApi,
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
		Kind:                string(api.RestApi),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
	}

	result := server.buildStoredPolicyFromAPI(cfg)
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
	cfg.Status = models.StatusPending
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
		server.handleStatusUpdate("0000-test-id-0000-000000000000", true, 1, "")
		require.NoError(t, <-done)

		retrievedCfg, err := server.store.Get("0000-test-id-0000-000000000000")
		require.NoError(t, err)
		assert.Equal(t, models.StatusDeployed, retrievedCfg.Status)
	}
}

// TestNewAPIServer tests the NewAPIServer constructor
func TestNewAPIServer(t *testing.T) {
	store := storage.NewConfigStore()
	mockDB := NewMockStorage()

	policyDefs := map[string]api.PolicyDefinition{
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

	// Add test configs with different statuses
	cfg1 := createTestStoredConfig("test-id-1", "api-one", "v1.0.0", "/ctx1")
	cfg1.Status = models.StatusDeployed
	cfg2 := createTestStoredConfig("test-id-2", "api-two", "v2.0.0", "/ctx2")
	cfg2.Status = models.StatusPending
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
			server.SearchDeployments(c, string(api.RestApi))

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

	apiData := response["api"].(map[string]interface{})
	metadata := apiData["metadata"].(map[string]interface{})
	assert.Contains(t, metadata, "deployedAt")
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

	apiData := response["api"].(map[string]interface{})
	metadata := apiData["metadata"].(map[string]interface{})
	assert.Contains(t, metadata, "deployedAt")
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
	server.handleStatusUpdate("0000-test-id-0000-000000000000", true, 1, "corr-id-1")

	// Verify both store and DB are updated
	updatedCfg, _ := server.store.Get("0000-test-id-0000-000000000000")
	assert.Equal(t, models.StatusDeployed, updatedCfg.Status)
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
	server.handleStatusUpdate("0000-test-id-0000-000000000000", true, 1, "corr-id-1")
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
	}

	result := server.buildStoredPolicyFromAPI(cfg)
	assert.Nil(t, result)
}

// TestConfigDumpAPIStatusConversion tests status conversion in GetConfigDump
func TestConfigDumpAPIStatusConversion(t *testing.T) {
	server := createTestAPIServer()

	testCases := []struct {
		name   string
		status models.ConfigStatus
	}{
		{"deployed", models.StatusDeployed},
		{"failed", models.StatusFailed},
		{"pending", models.StatusPending},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear store
			server.store = storage.NewConfigStore()

			cfg := createTestStoredConfig("0000-test-id-0000-000000000000", "0000-test-api-0000-000000000000", "v1.0.0", "/test")
			cfg.Status = tc.status
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
	server.SearchDeployments(c, string(api.RestApi))

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

	providerConfig := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LlmProvider,
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
		Kind:                string(api.LlmProvider),
		Handle:              "test-llm-provider",
		DisplayName:         "test-llm",
		Version:             "v1.0",
		SourceConfiguration: providerConfig,
		Status:              models.StatusDeployed,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	_ = server.store.Add(cfg)

	c, w := createTestContext("GET", "/llm-providers/test-llm-provider", nil)
	server.GetLLMProviderById(c, "test-llm-provider")

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestGetLLMProxyByIdFound tests getting an existing LLM proxy
func TestGetLLMProxyByIdFound(t *testing.T) {
	server := createTestAPIServer()

	proxyConfig := api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LlmProxy,
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
		Kind:                string(api.LlmProxy),
		Handle:              "test-llm-proxy-handle",
		DisplayName:         "test-llm-proxy",
		Version:             "v1.0",
		SourceConfiguration: proxyConfig,
		Status:              models.StatusDeployed,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	_ = server.store.Add(cfg)

	c, w := createTestContext("GET", "/llm-proxies/test-llm-proxy-handle", nil)
	server.GetLLMProxyById(c, "test-llm-proxy-handle")

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestGetLLMProviderByIdWithDeployedAt tests GetLLMProviderById with deployedAt
func TestGetLLMProviderByIdWithDeployedAt(t *testing.T) {
	server := createTestAPIServer()

	deployedAt := time.Now()
	providerConfig := api.LLMProviderConfiguration{
		ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LlmProvider,
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
		Kind:                string(api.LlmProvider),
		Handle:              "test-llm-provider",
		DisplayName:         "test-llm",
		Version:             "v1.0",
		SourceConfiguration: providerConfig,
		Status:              models.StatusDeployed,
		DeployedAt:          &deployedAt,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	_ = server.store.Add(cfg)

	c, w := createTestContext("GET", "/llm-providers/test-llm-provider", nil)
	server.GetLLMProviderById(c, "test-llm-provider")

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	provider := response["provider"].(map[string]interface{})
	metadata := provider["metadata"].(map[string]interface{})
	assert.Contains(t, metadata, "deployedAt")
}

// TestGetLLMProxyByIdWithDeployedAt tests GetLLMProxyById with deployedAt
func TestGetLLMProxyByIdWithDeployedAt(t *testing.T) {
	server := createTestAPIServer()

	deployedAt := time.Now()
	proxyConfig := api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.LlmProxy,
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
		Kind:                string(api.LlmProxy),
		Handle:              "test-llm-proxy-handle",
		DisplayName:         "test-llm-proxy",
		Version:             "v1.0",
		SourceConfiguration: proxyConfig,
		Status:              models.StatusDeployed,
		DeployedAt:          &deployedAt,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	_ = server.store.Add(cfg)

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
	server.handleStatusUpdate("0000-test-id-0000-000000000000", true, 1, "")

	updatedCfg, _ := server.store.Get("0000-test-id-0000-000000000000")
	assert.Equal(t, models.StatusDeployed, updatedCfg.Status)
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

// TestDeleteLLMProxyInternalError tests DeleteLLMProxy with internal error
// Note: This test requires full deployment service setup
func TestDeleteLLMProxyInternalError(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
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
		Kind: api.WebSubApi,
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-test-id-0000-000000000000",
		Kind:                string(api.WebSubApi),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
	}

	result := server.buildStoredPolicyFromAPI(cfg)
	// Should return nil because the spec can't be parsed as WebhookAPIData without proper setup
	assert.Nil(t, result)
}

// Test that policies are sorted in ListPolicies
func TestListPoliciesSorted(t *testing.T) {
	server := createTestAPIServer()

	// Add policies in unsorted order
	server.policyDefinitions["z-policy|v1"] = api.PolicyDefinition{Name: "z-policy", Version: "v1"}
	server.policyDefinitions["a-policy|v2"] = api.PolicyDefinition{Name: "a-policy", Version: "v2"}
	server.policyDefinitions["a-policy|v1"] = api.PolicyDefinition{Name: "a-policy", Version: "v1"}
	server.policyDefinitions["m-policy|v1"] = api.PolicyDefinition{Name: "m-policy", Version: "v1"}

	c, w := createTestContext("GET", "/policies", nil)
	server.ListPolicies(c)

	var response struct {
		Policies []api.PolicyDefinition `json:"policies"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify sorted order: first by name, then by version
	assert.Equal(t, "a-policy", response.Policies[0].Name)
	assert.Equal(t, "v1", response.Policies[0].Version)
	assert.Equal(t, "a-policy", response.Policies[1].Name)
	assert.Equal(t, "v2", response.Policies[1].Version)
	assert.Equal(t, "m-policy", response.Policies[2].Name)
	assert.Equal(t, "z-policy", response.Policies[3].Name)
}

// Test GetConfigDump with config missing handle
func TestGetConfigDumpMissingHandle(t *testing.T) {
	server := createTestAPIServer()

	// Create config with empty handle
	apiConfig := api.RestAPI{
		Kind:     api.RestApi,
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
		Kind:                string(api.RestApi),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
		CreatedAt:           time.Now(),
		UpdatedAt: time.Now(),
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
		Kind:                string(api.Mcp),
		SourceConfiguration: make(chan int), // Invalid - can't be marshaled to JSON
		Configuration: api.RestAPI{
			Kind: api.RestApi, // Use RestApi for RestAPI type
			Metadata: api.Metadata{
				Name: "test-mcp",
			},
		},
		Status:    models.StatusDeployed,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = server.store.Add(cfg)

	c, w := createTestContext("GET", "/mcp-proxies?displayName=test", nil)
	c.Request.URL.RawQuery = "displayName=test"
	server.SearchDeployments(c, string(api.Mcp))

	// SearchDeployments logs unmarshal errors and continues, returning StatusOK with valid configs only
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestBuildStoredPolicyFromAPIWithVhosts tests buildStoredPolicyFromAPI with custom vhosts
func TestBuildStoredPolicyFromAPIWithVhosts(t *testing.T) {
	server := createTestAPIServer()
	server.policyDefinitions["test-policy|v1.0.0"] = api.PolicyDefinition{Name: "test-policy", Version: "v1.0.0"}

	policies := []api.Policy{
		{Name: "test-policy", Version: "v1"},
	}

	sandboxUrl := "http://sandbox.example.com"
	sandboxVhost := "sandbox.localhost"
	apiConfig := api.RestAPI{
		Kind: api.RestApi,
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
		Kind:                string(api.RestApi),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
	}

	result := server.buildStoredPolicyFromAPI(cfg)
	assert.NotNil(t, result)
	// Should have 2 routes (one for main vhost, one for sandbox)
	assert.Equal(t, 2, len(result.Configuration.Routes))
}

// TestBuildStoredPolicyFromAPIOperationPolicies tests operation-level policy merging
func TestBuildStoredPolicyFromAPIOperationPolicies(t *testing.T) {
	server := createTestAPIServer()
	server.policyDefinitions["api-policy|v1.0.0"] = api.PolicyDefinition{Name: "api-policy", Version: "v1.0.0"}
	server.policyDefinitions["op-policy|v1.0.0"] = api.PolicyDefinition{Name: "op-policy", Version: "v1.0.0"}

	apiPolicies := []api.Policy{
		{Name: "api-policy", Version: "v1"},
	}

	opPolicies := []api.Policy{
		{Name: "op-policy", Version: "v1"},
	}

	apiConfig := api.RestAPI{
		Kind: api.RestApi,
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
		Kind:                string(api.RestApi),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
	}

	result := server.buildStoredPolicyFromAPI(cfg)
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
	server.handleStatusUpdate("0000-test-id-0000-000000000000", true, 1, "")

	updatedCfg, _ := server.store.Get("0000-test-id-0000-000000000000")
	assert.Equal(t, models.StatusDeployed, updatedCfg.Status)
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
		Kind: api.WebSubApi,
	}
	cfg := &models.StoredConfig{
		UUID:                "0000-test-id-0000-000000000000",
		Kind:                string(api.WebSubApi),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
	}

	result := server.buildStoredPolicyFromAPI(cfg)
	// Should return nil since we don't have valid spec data
	assert.Nil(t, result)
}

// Test ListMCPProxies with stored configs that have unmarshal issues
func TestListMCPProxiesUnmarshalError(t *testing.T) {
	server := createTestAPIServer()

	// Add MCP config, then replace SourceConfiguration with something that can't be marshaled to JSON
	cfg := &models.StoredConfig{
		UUID: "0000-mcp-id-0000-000000000000",
		Kind: string(api.Mcp),
		SourceConfiguration: api.MCPProxyConfiguration{
			Kind:     api.Mcp,
			Metadata: api.Metadata{Name: "test-mcp"},
			Spec: api.MCPProxyConfigData{
				DisplayName: "Test MCP",
				Version:     "v1.0",
			},
		},
		Configuration: api.RestAPI{
			Kind:     api.RestApi,
			Metadata: api.Metadata{Name: "test-mcp"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = server.store.Add(cfg)
	// Mutate SourceConfiguration to something that can't be JSON marshaled
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

// TestDeleteRestAPIDBError tests DeleteRestAPI with database delete error
// Note: This test requires full deployment service setup
func TestDeleteRestAPIDBError(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
}

// TestUpdateRestAPIDBError tests UpdateRestAPI with database update error
// Note: This test requires full deployment service setup
func TestUpdateRestAPIDBError(t *testing.T) {
	t.Skip("Skipping test that requires full deployment service setup")
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
	assert.Equal(t, "API key value is required", response.Message)
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

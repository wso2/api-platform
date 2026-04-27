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

package utils

import (
	"database/sql"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// testMockDB is a minimal in-memory implementation of storage.Storage for unit tests.
// Only config and template CRUD methods are functional; all other methods are no-op stubs.
type testMockDB struct {
	configs   map[string]*models.StoredConfig
	templates map[string]*models.StoredLLMProviderTemplate
}

func newTestMockDB() *testMockDB {
	return &testMockDB{
		configs:   make(map[string]*models.StoredConfig),
		templates: make(map[string]*models.StoredLLMProviderTemplate),
	}
}

func (m *testMockDB) SaveConfig(cfg *models.StoredConfig) error {
	m.configs[cfg.UUID] = cfg
	return nil
}

func (m *testMockDB) UpdateConfig(cfg *models.StoredConfig) error {
	m.configs[cfg.UUID] = cfg
	return nil
}

func (m *testMockDB) UpsertConfig(cfg *models.StoredConfig) (bool, error) {
	m.configs[cfg.UUID] = cfg
	return true, nil
}

func (m *testMockDB) DeleteConfig(id string) error {
	delete(m.configs, id)
	return nil
}

func (m *testMockDB) GetConfig(id string) (*models.StoredConfig, error) {
	if cfg, ok := m.configs[id]; ok {
		return cfg, nil
	}
	return nil, storage.ErrNotFound
}

func (m *testMockDB) GetConfigByKindAndHandle(kind, handle string) (*models.StoredConfig, error) {
	for _, cfg := range m.configs {
		if cfg.Kind == kind && cfg.Handle == handle {
			return cfg, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *testMockDB) GetConfigByKindNameAndVersion(kind, displayName, version string) (*models.StoredConfig, error) {
	for _, cfg := range m.configs {
		if cfg.Kind == kind && cfg.DisplayName == displayName && cfg.Version == version {
			return cfg, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *testMockDB) GetAllConfigs() ([]*models.StoredConfig, error) {
	result := make([]*models.StoredConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		result = append(result, cfg)
	}
	return result, nil
}

func (m *testMockDB) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	result := make([]*models.StoredConfig, 0)
	for _, cfg := range m.configs {
		if cfg.Kind == kind {
			result = append(result, cfg)
		}
	}
	return result, nil
}

func (m *testMockDB) GetAllConfigsByOrigin(origin models.Origin) ([]*models.StoredConfig, error) {
	result := make([]*models.StoredConfig, 0)
	for _, cfg := range m.configs {
		if cfg.Origin == origin {
			result = append(result, cfg)
		}
	}
	return result, nil
}

func (m *testMockDB) SaveLLMProviderTemplate(t *models.StoredLLMProviderTemplate) error {
	m.templates[t.UUID] = t
	return nil
}
func (m *testMockDB) UpdateLLMProviderTemplate(t *models.StoredLLMProviderTemplate) error {
	m.templates[t.UUID] = t
	return nil
}
func (m *testMockDB) DeleteLLMProviderTemplate(id string) error {
	delete(m.templates, id)
	return nil
}
func (m *testMockDB) GetLLMProviderTemplate(id string) (*models.StoredLLMProviderTemplate, error) {
	if t, ok := m.templates[id]; ok {
		return t, nil
	}
	return nil, storage.ErrNotFound
}
func (m *testMockDB) GetAllLLMProviderTemplates() ([]*models.StoredLLMProviderTemplate, error) {
	result := make([]*models.StoredLLMProviderTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		result = append(result, t)
	}
	return result, nil
}

func (m *testMockDB) GetLLMProviderTemplateByHandle(handle string) (*models.StoredLLMProviderTemplate, error) {
	for _, t := range m.templates {
		if t.GetHandle() == handle {
			return t, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *testMockDB) SaveAPIKey(key *models.APIKey) error   { return nil }
func (m *testMockDB) UpsertAPIKey(key *models.APIKey) error { return nil }
func (m *testMockDB) GetAPIKeyByID(id string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}
func (m *testMockDB) GetAPIKeyByUUID(uuid string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}
func (m *testMockDB) GetAPIKeyByKey(key string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}
func (m *testMockDB) GetAPIKeysByAPI(apiId string) ([]*models.APIKey, error) { return nil, nil }
func (m *testMockDB) GetAllAPIKeys() ([]*models.APIKey, error)               { return nil, nil }
func (m *testMockDB) GetAPIKeysByAPIAndName(apiId, name string) (*models.APIKey, error) {
	return nil, storage.ErrNotFound
}
func (m *testMockDB) UpdateAPIKey(key *models.APIKey) error     { return nil }
func (m *testMockDB) DeleteAPIKey(key string) error             { return nil }
func (m *testMockDB) DeleteAPIKeysByUUIDs(uuids []string) error { return nil }
func (m *testMockDB) ListAPIKeysForArtifactsNotIn(artifactUUIDs []string, keyUUIDs []string) ([]*models.APIKey, error) {
	return nil, nil
}
func (m *testMockDB) RemoveAPIKeysAPI(apiId string) error             { return nil }
func (m *testMockDB) RemoveAPIKeyAPIAndName(apiId, name string) error { return nil }
func (m *testMockDB) CountActiveAPIKeysByUserAndAPI(apiId, userID string) (int, error) {
	return 0, nil
}

func (m *testMockDB) SaveSubscriptionPlan(plan *models.SubscriptionPlan) error { return nil }
func (m *testMockDB) GetSubscriptionPlanByID(id, gatewayID string) (*models.SubscriptionPlan, error) {
	return nil, storage.ErrNotFound
}
func (m *testMockDB) ListSubscriptionPlans(gatewayID string) ([]*models.SubscriptionPlan, error) {
	return nil, nil
}
func (m *testMockDB) UpdateSubscriptionPlan(plan *models.SubscriptionPlan) error { return nil }
func (m *testMockDB) DeleteSubscriptionPlan(id, gatewayID string) error          { return nil }
func (m *testMockDB) DeleteSubscriptionPlansNotIn(ids []string) error            { return nil }

func (m *testMockDB) SaveSubscription(sub *models.Subscription) error { return nil }
func (m *testMockDB) GetSubscriptionByID(id, gatewayID string) (*models.Subscription, error) {
	return nil, storage.ErrNotFound
}
func (m *testMockDB) ListSubscriptionsByAPI(apiID, gatewayID string, applicationID *string, status *string) ([]*models.Subscription, error) {
	return nil, nil
}
func (m *testMockDB) ListActiveSubscriptions() ([]*models.Subscription, error)        { return nil, nil }
func (m *testMockDB) UpdateSubscription(sub *models.Subscription) error               { return nil }
func (m *testMockDB) DeleteSubscription(id, gatewayID string) error                   { return nil }
func (m *testMockDB) DeleteSubscriptionsForAPINotIn(apiID string, ids []string) error { return nil }
func (m *testMockDB) ReplaceApplicationAPIKeyMappings(application *models.StoredApplication, mappings []*models.ApplicationAPIKeyMapping) error {
	return nil
}

func (m *testMockDB) SaveCertificate(cert *models.StoredCertificate) error { return nil }
func (m *testMockDB) GetCertificate(id string) (*models.StoredCertificate, error) {
	return nil, storage.ErrNotFound
}
func (m *testMockDB) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	return nil, storage.ErrNotFound
}
func (m *testMockDB) ListCertificates() ([]*models.StoredCertificate, error) { return nil, nil }
func (m *testMockDB) DeleteCertificate(id string) error                      { return nil }

func (m *testMockDB) GetDB() *sql.DB { return nil }
func (m *testMockDB) Close() error   { return nil }

// Secret management methods

func (m *testMockDB) SaveSecret(secret *models.Secret) error   { return nil }
func (m *testMockDB) GetSecrets() ([]models.SecretMeta, error) { return nil, nil }
func (m *testMockDB) GetSecret(handle string) (*models.Secret, error) {
	return nil, storage.ErrNotFound
}
func (m *testMockDB) UpdateSecret(secret *models.Secret) (*models.Secret, error) {
	return nil, storage.ErrNotFound
}
func (m *testMockDB) DeleteSecret(handle string) error         { return nil }
func (m *testMockDB) SecretExists(handle string) (bool, error) { return false, nil }

// Bottom-up sync methods
func (m *testMockDB) UpdateCPSyncStatus(uuid, cpArtifactID string, status models.CPSyncStatus, reason string) error {
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

func (m *testMockDB) GetConfigByCPArtifactID(cpArtifactID string) (*models.StoredConfig, error) {
	for _, config := range m.configs {
		if config.CPArtifactID == cpArtifactID {
			return config, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (m *testMockDB) UpdateDeploymentID(uuid, deploymentID string) error {
	if config, ok := m.configs[uuid]; ok {
		config.DeploymentID = deploymentID
		return nil
	}
	return storage.ErrNotFound
}

func (m *testMockDB) GetPendingBottomUpAPIs() ([]*models.StoredConfig, error) {
	var pending []*models.StoredConfig
	for _, config := range m.configs {
		if config.Origin == models.OriginGatewayAPI && config.CPSyncStatus != models.CPSyncStatusSuccess {
			pending = append(pending, config)
		}
	}
	return pending, nil
}

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

package secrets

import (
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// MockEncryptionProvider implements encryption.EncryptionProvider for testing
type MockEncryptionProvider struct {
	name            string
	encryptResponse *encryption.EncryptedPayload
	encryptErr      error
	decryptResponse []byte
	decryptErr      error
	healthErr       error
}

func (m *MockEncryptionProvider) Name() string {
	return m.name
}

func (m *MockEncryptionProvider) Encrypt(plaintext []byte) (*encryption.EncryptedPayload, error) {
	if m.encryptErr != nil {
		return nil, m.encryptErr
	}
	return m.encryptResponse, nil
}

func (m *MockEncryptionProvider) Decrypt(payload *encryption.EncryptedPayload) ([]byte, error) {
	if m.decryptErr != nil {
		return nil, m.decryptErr
	}
	return m.decryptResponse, nil
}

func (m *MockEncryptionProvider) HealthCheck() error {
	return m.healthErr
}

// MockStorage implements a minimal storage interface for testing secrets
type MockStorage struct {
	secrets         map[string]*models.Secret
	secretsMeta     []models.SecretMeta
	saveSecretErr   error
	getSecretErr    error
	updateSecretErr error
	deleteSecretErr error
	getSecretsErr   error
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		secrets: make(map[string]*models.Secret),
	}
}

func (m *MockStorage) SaveSecret(secret *models.Secret) error {
	if m.saveSecretErr != nil {
		return m.saveSecretErr
	}
	m.secrets[secret.Handle] = secret
	return nil
}

func (m *MockStorage) GetSecret(handle string) (*models.Secret, error) {
	if m.getSecretErr != nil {
		return nil, m.getSecretErr
	}
	s, ok := m.secrets[handle]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return s, nil
}

func (m *MockStorage) UpdateSecret(secret *models.Secret) (*models.Secret, error) {
	if m.updateSecretErr != nil {
		return nil, m.updateSecretErr
	}
	if _, ok := m.secrets[secret.Handle]; !ok {
		return nil, storage.ErrNotFound
	}
	m.secrets[secret.Handle] = secret
	return secret, nil
}

func (m *MockStorage) DeleteSecret(handle string) error {
	if m.deleteSecretErr != nil {
		return m.deleteSecretErr
	}
	if _, ok := m.secrets[handle]; !ok {
		return storage.ErrNotFound
	}
	delete(m.secrets, handle)
	return nil
}

func (m *MockStorage) GetSecrets() ([]models.SecretMeta, error) {
	if m.getSecretsErr != nil {
		return nil, m.getSecretsErr
	}
	return m.secretsMeta, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func createTestProviderManager() *encryption.ProviderManager {
	provider := &MockEncryptionProvider{
		name: "test-provider",
		encryptResponse: &encryption.EncryptedPayload{
			Provider:   "test-provider",
			KeyVersion: "v1",
			Ciphertext: []byte("encrypted-data"),
		},
		decryptResponse: []byte("decrypted-value"),
	}
	pm, _ := encryption.NewProviderManager([]encryption.EncryptionProvider{provider}, testLogger())
	return pm
}

// minimalStorage wraps MockStorage as a storage.Storage interface using composition
// We only implement the methods needed for SecretService
type minimalStorage struct {
	*MockStorage
}

// Implement remaining required interface methods as no-ops
func (m *minimalStorage) SaveConfig(cfg *models.StoredConfig) error              { return nil }
func (m *minimalStorage) UpdateConfig(cfg *models.StoredConfig) error            { return nil }
func (m *minimalStorage) UpsertConfig(cfg *models.StoredConfig) (bool, error)    { return false, nil }
func (m *minimalStorage) DeleteConfig(id string) error                           { return nil }
func (m *minimalStorage) GetConfig(id string) (*models.StoredConfig, error)      { return nil, nil }
func (m *minimalStorage) GetConfigByKindAndHandle(kind, handle string) (*models.StoredConfig, error) {
	return nil, nil
}
func (m *minimalStorage) GetConfigByKindNameAndVersion(kind, displayName, version string) (*models.StoredConfig, error) {
	return nil, nil
}
func (m *minimalStorage) GetAllConfigs() ([]*models.StoredConfig, error)           { return nil, nil }
func (m *minimalStorage) GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error) {
	return nil, nil
}
func (m *minimalStorage) GetAllConfigsByOrigin(origin models.Origin) ([]*models.StoredConfig, error) {
	return nil, nil
}
func (m *minimalStorage) SaveLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error {
	return nil
}
func (m *minimalStorage) UpdateLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error {
	return nil
}
func (m *minimalStorage) DeleteLLMProviderTemplate(id string) error { return nil }
func (m *minimalStorage) GetLLMProviderTemplate(id string) (*models.StoredLLMProviderTemplate, error) {
	return nil, nil
}
func (m *minimalStorage) GetAllLLMProviderTemplates() ([]*models.StoredLLMProviderTemplate, error) {
	return nil, nil
}
func (m *minimalStorage) GetLLMProviderTemplateByHandle(handle string) (*models.StoredLLMProviderTemplate, error) {
	return nil, nil
}
func (m *minimalStorage) SaveAPIKey(apiKey *models.APIKey) error            { return nil }
func (m *minimalStorage) UpsertAPIKey(apiKey *models.APIKey) error          { return nil }
func (m *minimalStorage) GetAPIKeyByID(id string) (*models.APIKey, error)   { return nil, nil }
func (m *minimalStorage) GetAPIKeyByUUID(uuid string) (*models.APIKey, error) { return nil, nil }
func (m *minimalStorage) GetAPIKeyByKey(key string) (*models.APIKey, error) { return nil, nil }
func (m *minimalStorage) GetAPIKeysByAPI(apiId string) ([]*models.APIKey, error) {
	return nil, nil
}
func (m *minimalStorage) GetAllAPIKeys() ([]*models.APIKey, error) { return nil, nil }
func (m *minimalStorage) GetAPIKeysByAPIAndName(apiId, name string) (*models.APIKey, error) {
	return nil, nil
}
func (m *minimalStorage) UpdateAPIKey(apiKey *models.APIKey) error { return nil }
func (m *minimalStorage) DeleteAPIKey(key string) error            { return nil }
func (m *minimalStorage) RemoveAPIKeysAPI(apiId string) error      { return nil }
func (m *minimalStorage) RemoveAPIKeyAPIAndName(apiId, name string) error {
	return nil
}
func (m *minimalStorage) CountActiveAPIKeysByUserAndAPI(apiId, userID string) (int, error) {
	return 0, nil
}
func (m *minimalStorage) ListAPIKeysForArtifactsNotIn(artifactUUIDs []string, keyUUIDs []string) ([]*models.APIKey, error) {
	return nil, nil
}
func (m *minimalStorage) DeleteAPIKeysByUUIDs(uuids []string) error { return nil }
func (m *minimalStorage) SaveSubscriptionPlan(plan *models.SubscriptionPlan) error {
	return nil
}
func (m *minimalStorage) GetSubscriptionPlanByID(id, gatewayID string) (*models.SubscriptionPlan, error) {
	return nil, nil
}
func (m *minimalStorage) ListSubscriptionPlans(gatewayID string) ([]*models.SubscriptionPlan, error) {
	return nil, nil
}
func (m *minimalStorage) UpdateSubscriptionPlan(plan *models.SubscriptionPlan) error { return nil }
func (m *minimalStorage) DeleteSubscriptionPlan(id, gatewayID string) error          { return nil }
func (m *minimalStorage) DeleteSubscriptionPlansNotIn(ids []string) error            { return nil }
func (m *minimalStorage) SaveSubscription(sub *models.Subscription) error            { return nil }
func (m *minimalStorage) GetSubscriptionByID(id, gatewayID string) (*models.Subscription, error) {
	return nil, nil
}
func (m *minimalStorage) ListSubscriptionsByAPI(apiID, gatewayID string, applicationID, status *string) ([]*models.Subscription, error) {
	return nil, nil
}
func (m *minimalStorage) ListActiveSubscriptions() ([]*models.Subscription, error) {
	return nil, nil
}
func (m *minimalStorage) UpdateSubscription(sub *models.Subscription) error { return nil }
func (m *minimalStorage) DeleteSubscription(id, gatewayID string) error     { return nil }
func (m *minimalStorage) DeleteSubscriptionsForAPINotIn(apiID string, ids []string) error {
	return nil
}
func (m *minimalStorage) ReplaceApplicationAPIKeyMappings(application *models.StoredApplication, mappings []*models.ApplicationAPIKeyMapping) error {
	return nil
}
func (m *minimalStorage) SaveCertificate(cert *models.StoredCertificate) error { return nil }
func (m *minimalStorage) GetCertificate(id string) (*models.StoredCertificate, error) {
	return nil, nil
}
func (m *minimalStorage) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	return nil, nil
}
func (m *minimalStorage) ListCertificates() ([]*models.StoredCertificate, error) { return nil, nil }
func (m *minimalStorage) DeleteCertificate(id string) error                      { return nil }
func (m *minimalStorage) SecretExists(handle string) (bool, error) {
	_, ok := m.secrets[handle]
	return ok, nil
}
func (m *minimalStorage) GetPendingBottomUpAPIs() ([]*models.StoredConfig, error) {
	return nil, nil
}
func (m *minimalStorage) UpdateCPSyncStatus(uuid, cpArtifactID string, status models.CPSyncStatus, reason string) error {
	return nil
}
func (m *minimalStorage) GetConfigByCPArtifactID(cpArtifactID string) (*models.StoredConfig, error) {
	return nil, storage.ErrNotFound
}
func (m *minimalStorage) GetDB() *sql.DB { return nil }
func (m *minimalStorage) Close() error   { return nil }

func newMinimalStorage() *minimalStorage {
	return &minimalStorage{MockStorage: NewMockStorage()}
}

func TestNewSecretsService(t *testing.T) {
	store := newMinimalStorage()
	pm := createTestProviderManager()
	logger := testLogger()

	svc := NewSecretsService(store, pm, logger)

	assert.NotNil(t, svc)
	assert.NotNil(t, svc.storage)
	assert.NotNil(t, svc.providerManager)
	assert.NotNil(t, svc.parser)
	assert.NotNil(t, svc.validator)
	assert.NotNil(t, svc.logger)
}

func TestMaxSecretSize(t *testing.T) {
	assert.Equal(t, 10*1024, MaxSecretSize)
}

func TestSecretService_CreateSecret(t *testing.T) {
	tests := []struct {
		name          string
		yamlData      string
		setupProvider func() *MockEncryptionProvider
		setupStorage  func() *minimalStorage
		wantErr       bool
		errContains   string
	}{
		{
			name: "successful creation",
			yamlData: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Secret
metadata:
  name: my-secret
spec:
  displayName: My Secret
  description: Test secret description
  value: supersecretvalue
`,
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{
					name: "test",
					encryptResponse: &encryption.EncryptedPayload{
						Provider:   "test",
						KeyVersion: "v1",
						Ciphertext: []byte("encrypted"),
					},
				}
			},
			setupStorage: func() *minimalStorage {
				return newMinimalStorage()
			},
			wantErr: false,
		},
		{
			name:     "invalid yaml",
			yamlData: "this is not: valid: yaml:",
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{name: "test"}
			},
			setupStorage: func() *minimalStorage {
				return newMinimalStorage()
			},
			wantErr:     true,
			errContains: "failed to parse configuration",
		},
		{
			name: "validation error - missing name",
			yamlData: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Secret
metadata:
  name: ""
spec:
  displayName: My Secret
  value: secretvalue
`,
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{name: "test"}
			},
			setupStorage: func() *minimalStorage {
				return newMinimalStorage()
			},
			wantErr:     true,
			errContains: "validation failed",
		},
		{
			name: "secret value exceeds max size",
			yamlData: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Secret
metadata:
  name: large-secret
spec:
  displayName: Large Secret
  value: ` + strings.Repeat("a", MaxSecretSize+1),
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{name: "test"}
			},
			setupStorage: func() *minimalStorage {
				return newMinimalStorage()
			},
			wantErr:     true,
			errContains: "Secret value must be less than",
		},
		{
			name: "encryption failure",
			yamlData: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Secret
metadata:
  name: my-secret
spec:
  displayName: My Secret
  value: secretvalue
`,
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{
					name:       "test",
					encryptErr: errors.New("encryption hardware failure"),
				}
			},
			setupStorage: func() *minimalStorage {
				return newMinimalStorage()
			},
			wantErr:     true,
			errContains: "encryption failed",
		},
		{
			name: "storage save failure",
			yamlData: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Secret
metadata:
  name: my-secret
spec:
  displayName: My Secret
  value: secretvalue
`,
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{
					name: "test",
					encryptResponse: &encryption.EncryptedPayload{
						Provider:   "test",
						KeyVersion: "v1",
						Ciphertext: []byte("encrypted"),
					},
				}
			},
			setupStorage: func() *minimalStorage {
				m := newMinimalStorage()
				m.saveSecretErr = errors.New("database unavailable")
				return m
			},
			wantErr:     true,
			errContains: "storage failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := tt.setupProvider()
			store := tt.setupStorage()
			pm, err := encryption.NewProviderManager([]encryption.EncryptionProvider{provider}, testLogger())
			require.NoError(t, err)
			svc := NewSecretsService(store, pm, testLogger())

			params := SecretParams{
				Data:          []byte(tt.yamlData),
				ContentType:   "application/yaml",
				CorrelationID: "test-corr-id",
				Logger:        testLogger(),
			}

			secret, err := svc.CreateSecret(params)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, secret)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, secret)
				assert.Equal(t, "my-secret", secret.Handle)
				assert.Equal(t, "supersecretvalue", secret.Value)
			}
		})
	}
}

func TestSecretService_Get(t *testing.T) {
	tests := []struct {
		name          string
		handle        string
		setupStorage  func() *minimalStorage
		setupProvider func() *MockEncryptionProvider
		wantErr       bool
		errContains   string
		wantValue     string
	}{
		{
			name:   "successful retrieval",
			handle: "my-secret",
			setupStorage: func() *minimalStorage {
				m := newMinimalStorage()
				// Store a secret with properly marshaled ciphertext
				payload := &encryption.EncryptedPayload{
					Provider:   "test",
					KeyVersion: "v1",
					Ciphertext: []byte("encrypted-data"),
				}
				m.secrets["my-secret"] = &models.Secret{
					Handle:      "my-secret",
					DisplayName: "My Secret",
					Ciphertext:  []byte(encryption.MarshalPayload(payload)),
				}
				return m
			},
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{
					name:            "test",
					decryptResponse: []byte("the-secret-value"),
				}
			},
			wantErr:   false,
			wantValue: "the-secret-value",
		},
		{
			name:   "secret not found",
			handle: "nonexistent",
			setupStorage: func() *minimalStorage {
				return newMinimalStorage()
			},
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{name: "test"}
			},
			wantErr: true,
		},
		{
			name:   "storage retrieval error",
			handle: "my-secret",
			setupStorage: func() *minimalStorage {
				m := newMinimalStorage()
				m.getSecretErr = errors.New("database timeout")
				return m
			},
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{name: "test"}
			},
			wantErr:     true,
			errContains: "database timeout",
		},
		{
			name:   "invalid ciphertext payload",
			handle: "my-secret",
			setupStorage: func() *minimalStorage {
				m := newMinimalStorage()
				m.secrets["my-secret"] = &models.Secret{
					Handle:     "my-secret",
					Ciphertext: []byte("invalid-payload-not-base64-json"),
				}
				return m
			},
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{name: "test"}
			},
			wantErr:     true,
			errContains: "payload deserialization failed",
		},
		{
			name:   "decryption failure",
			handle: "my-secret",
			setupStorage: func() *minimalStorage {
				m := newMinimalStorage()
				payload := &encryption.EncryptedPayload{
					Provider:   "test",
					KeyVersion: "v1",
					Ciphertext: []byte("encrypted-data"),
				}
				m.secrets["my-secret"] = &models.Secret{
					Handle:     "my-secret",
					Ciphertext: []byte(encryption.MarshalPayload(payload)),
				}
				return m
			},
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{
					name:       "test",
					decryptErr: errors.New("key not available"),
				}
			},
			wantErr:     true,
			errContains: "decryption failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := tt.setupStorage()
			provider := tt.setupProvider()
			pm, err := encryption.NewProviderManager([]encryption.EncryptionProvider{provider}, testLogger())
			require.NoError(t, err)
			svc := NewSecretsService(store, pm, testLogger())

			secret, err := svc.Get(tt.handle, "test-corr-id")

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, secret)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, secret)
				assert.Equal(t, tt.wantValue, secret.Value)
			}
		})
	}
}

func TestSecretService_GetSecrets(t *testing.T) {
	tests := []struct {
		name        string
		setupStore  func() *minimalStorage
		wantCount   int
		wantErr     bool
		errContains string
	}{
		{
			name: "successful retrieval of multiple secrets",
			setupStore: func() *minimalStorage {
				m := newMinimalStorage()
				m.secretsMeta = []models.SecretMeta{
					{Handle: "secret-1", DisplayName: "Secret One"},
					{Handle: "secret-2", DisplayName: "Secret Two"},
					{Handle: "secret-3", DisplayName: "Secret Three"},
				}
				return m
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name: "empty secrets list",
			setupStore: func() *minimalStorage {
				m := newMinimalStorage()
				m.secretsMeta = []models.SecretMeta{}
				return m
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "storage error",
			setupStore: func() *minimalStorage {
				m := newMinimalStorage()
				m.getSecretsErr = errors.New("connection refused")
				return m
			},
			wantErr:     true,
			errContains: "failed to retrieve secrets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := tt.setupStore()
			pm := createTestProviderManager()
			svc := NewSecretsService(store, pm, testLogger())

			secrets, err := svc.GetSecrets("test-corr-id")

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Len(t, secrets, tt.wantCount)
			}
		})
	}
}

func TestSecretService_UpdateSecret(t *testing.T) {
	tests := []struct {
		name          string
		handle        string
		yamlData      string
		setupProvider func() *MockEncryptionProvider
		setupStorage  func() *minimalStorage
		wantErr       bool
		errContains   string
	}{
		{
			name:   "successful update",
			handle: "my-secret",
			yamlData: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Secret
metadata:
  name: my-secret
spec:
  displayName: Updated Secret
  description: Updated description
  value: newvalue
`,
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{
					name: "test",
					encryptResponse: &encryption.EncryptedPayload{
						Provider:   "test",
						KeyVersion: "v2",
						Ciphertext: []byte("newencrypted"),
					},
				}
			},
			setupStorage: func() *minimalStorage {
				m := newMinimalStorage()
				m.secrets["my-secret"] = &models.Secret{
					Handle: "my-secret",
				}
				return m
			},
			wantErr: false,
		},
		{
			name:     "invalid yaml",
			handle:   "my-secret",
			yamlData: ":::invalid",
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{name: "test"}
			},
			setupStorage: func() *minimalStorage {
				return newMinimalStorage()
			},
			wantErr:     true,
			errContains: "failed to parse configuration",
		},
		{
			name:   "handle mismatch",
			handle: "my-secret",
			yamlData: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Secret
metadata:
  name: different-name
spec:
  displayName: Secret
  value: value
`,
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{name: "test"}
			},
			setupStorage: func() *minimalStorage {
				m := newMinimalStorage()
				m.secrets["my-secret"] = &models.Secret{Handle: "my-secret"}
				return m
			},
			wantErr:     true,
			errContains: "does not match the URL path id",
		},
		{
			name:   "value exceeds max size",
			handle: "my-secret",
			yamlData: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Secret
metadata:
  name: my-secret
spec:
  displayName: Secret
  value: ` + strings.Repeat("x", MaxSecretSize+1),
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{name: "test"}
			},
			setupStorage: func() *minimalStorage {
				return newMinimalStorage()
			},
			wantErr:     true,
			errContains: "Secret value must be less than",
		},
		{
			name:   "encryption failure",
			handle: "my-secret",
			yamlData: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Secret
metadata:
  name: my-secret
spec:
  displayName: Secret
  value: value
`,
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{
					name:       "test",
					encryptErr: errors.New("key rotation in progress"),
				}
			},
			setupStorage: func() *minimalStorage {
				m := newMinimalStorage()
				m.secrets["my-secret"] = &models.Secret{Handle: "my-secret"}
				return m
			},
			wantErr:     true,
			errContains: "encryption failed",
		},
		{
			name:   "secret not found",
			handle: "nonexistent",
			yamlData: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Secret
metadata:
  name: nonexistent
spec:
  displayName: Secret
  value: value
`,
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{
					name: "test",
					encryptResponse: &encryption.EncryptedPayload{
						Provider:   "test",
						KeyVersion: "v1",
						Ciphertext: []byte("encrypted"),
					},
				}
			},
			setupStorage: func() *minimalStorage {
				return newMinimalStorage()
			},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:   "storage update failure",
			handle: "my-secret",
			yamlData: `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Secret
metadata:
  name: my-secret
spec:
  displayName: Secret
  value: value
`,
			setupProvider: func() *MockEncryptionProvider {
				return &MockEncryptionProvider{
					name: "test",
					encryptResponse: &encryption.EncryptedPayload{
						Provider:   "test",
						KeyVersion: "v1",
						Ciphertext: []byte("encrypted"),
					},
				}
			},
			setupStorage: func() *minimalStorage {
				m := newMinimalStorage()
				m.secrets["my-secret"] = &models.Secret{Handle: "my-secret"}
				m.updateSecretErr = errors.New("database write failed")
				return m
			},
			wantErr:     true,
			errContains: "storage update failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := tt.setupStorage()
			provider := tt.setupProvider()
			pm, err := encryption.NewProviderManager([]encryption.EncryptionProvider{provider}, testLogger())
			require.NoError(t, err)
			svc := NewSecretsService(store, pm, testLogger())

			params := SecretParams{
				Data:          []byte(tt.yamlData),
				ContentType:   "application/yaml",
				CorrelationID: "test-corr-id",
			}

			secret, err := svc.UpdateSecret(tt.handle, params)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, secret)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, secret)
				assert.Equal(t, tt.handle, secret.Handle)
				assert.Equal(t, "newvalue", secret.Value)
			}
		})
	}
}

func TestSecretService_Delete(t *testing.T) {
	tests := []struct {
		name        string
		handle      string
		setupStore  func() *minimalStorage
		wantErr     bool
		errContains string
	}{
		{
			name:   "successful deletion",
			handle: "my-secret",
			setupStore: func() *minimalStorage {
				m := newMinimalStorage()
				m.secrets["my-secret"] = &models.Secret{Handle: "my-secret"}
				return m
			},
			wantErr: false,
		},
		{
			name:   "secret not found",
			handle: "nonexistent",
			setupStore: func() *minimalStorage {
				return newMinimalStorage()
			},
			wantErr: true,
		},
		{
			name:   "storage deletion error",
			handle: "my-secret",
			setupStore: func() *minimalStorage {
				m := newMinimalStorage()
				m.secrets["my-secret"] = &models.Secret{Handle: "my-secret"}
				m.deleteSecretErr = errors.New("foreign key constraint")
				return m
			},
			wantErr:     true,
			errContains: "foreign key constraint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := tt.setupStore()
			pm := createTestProviderManager()
			svc := NewSecretsService(store, pm, testLogger())

			err := svc.Delete(tt.handle, "test-corr-id")

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				// Verify secret was deleted
				_, exists := store.secrets[tt.handle]
				assert.False(t, exists)
			}
		})
	}
}

func TestSecretParams(t *testing.T) {
	params := SecretParams{
		Data:          []byte("test-data"),
		ContentType:   "application/json",
		CorrelationID: "corr-123",
		Logger:        testLogger(),
	}

	assert.Equal(t, []byte("test-data"), params.Data)
	assert.Equal(t, "application/json", params.ContentType)
	assert.Equal(t, "corr-123", params.CorrelationID)
	assert.NotNil(t, params.Logger)
}

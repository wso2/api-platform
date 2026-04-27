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

package storage

import (
	"database/sql"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// Storage is the interface for persisting API configurations.
//
// # Design Philosophy
//
// This interface is intentionally database-agnostic to support multiple storage backends
// without requiring changes to business logic. All database operations are abstracted
// through this interface to ensure:
//
//   - Flexibility: Easy migration between database engines (SQLite, PostgreSQL, MySQL)
//   - Testability: Simple mocking for unit tests
//   - Separation of Concerns: Business logic (API handlers, xDS server) is decoupled from storage implementation
//
// # Implementation Guidelines
//
// When implementing this interface for a new database backend:
//
//   - DO: Keep all database-specific code in the implementation file (e.g., sqlite.go, postgres.go)
//   - DO: Use transactions for write operations to ensure consistency
//   - DO: Handle concurrency safely (multiple goroutines may call methods simultaneously)
//   - DON'T: Add database-specific methods to this interface (keep it generic)
//   - DON'T: Expose database implementation details through return types
//
// # Current Implementations
//
//   - SQLiteStorage: Embedded database for single-instance deployments (see sqlite.go)
//   - PostgreSQL: Planned for multi-instance deployments with external database (future)
//   - MySQL: Planned for cloud deployments (future)
//
// # Migration Strategy
//
// To migrate from one database backend to another:
//
//  1. Implement this interface for the new database (e.g., postgres.go)
//  2. Add database type to StorageConfig in pkg/config/config.go
//  3. Update storage initialization in cmd/controller/main.go
//  4. Export data from old database and import to new database
//  5. No changes required in API handlers, xDS server, or business logic
//
// See docs/postgresql-migration.md for detailed migration guide.
type Storage interface {
	// SaveConfig persists a new API configuration.
	//
	// Returns an error if a configuration with the same name and version already exists.
	// Implementations should ensure this operation is atomic (all-or-nothing).
	SaveConfig(cfg *models.StoredConfig) error

	// UpdateConfig updates an existing API configuration.
	//
	// Returns an error if the configuration does not exist.
	// Implementations should ensure this operation is atomic and thread-safe.
	UpdateConfig(cfg *models.StoredConfig) error

	// UpsertConfig performs a timestamp-guarded insert-or-update of an API configuration.
	// It inserts the config if it does not exist, or updates it only if the incoming
	// deployed_at timestamp is newer than the existing one. This prevents stale events
	// (from sync or WebSocket) from overwriting newer data.
	//
	// Returns (true, nil) if the row was actually inserted or updated.
	// Returns (false, nil) if the row exists with a newer deployed_at (stale event — no-op).
	// Returns (false, error) on database errors.
	UpsertConfig(cfg *models.StoredConfig) (bool, error)

	// DeleteConfig removes an API configuration by ID.
	//
	// Returns an error if the configuration does not exist.
	// Implementations should ensure related data (if any) is cleaned up.
	DeleteConfig(id string) error

	// GetConfig retrieves an API configuration by ID.
	//
	// Returns an error if the configuration is not found.
	// This is the fastest lookup method (O(1) for most databases).
	GetConfig(id string) (*models.StoredConfig, error)

	// GetConfigByKindAndHandle retrieves an API configuration by kind and handle.
	//
	// Returns an error if the configuration is not found.
	// The handle is the metadata.name from the API YAML configuration.
	// The kind filter prevents cross-kind reads (e.g. fetching a WebSub API when a REST API is expected).
	// This is the recommended lookup method for REST API endpoints.
	GetConfigByKindAndHandle(kind string, handle string) (*models.StoredConfig, error)

	// GetConfigByKindNameAndVersion retrieves an API configuration by kind, display name, and version.
	//
	// Returns an error if the configuration is not found.
	GetConfigByKindNameAndVersion(kind, displayName, version string) (*models.StoredConfig, error)

	// GetAllConfigs retrieves all API configurations.
	//
	// Returns an empty slice if no configurations exist.
	// May be expensive for large datasets; consider pagination in future versions.
	GetAllConfigs() ([]*models.StoredConfig, error)

	// GetAllConfigsByKind retrieves all API configurations of a specific kind.
	//
	// Returns an empty slice if no configurations of the specified kind exist.
	// May be expensive for large datasets; consider pagination in future versions.
	GetAllConfigsByKind(kind string) ([]*models.StoredConfig, error)

	// GetAllConfigsByOrigin retrieves artifact metadata for all configs with the
	// given origin. Only the artifacts table is queried (no resource-table JOINs),
	// so the Configuration field will be nil. This is intended for sync diff
	// computation where only metadata (UUID, Kind, DesiredState, DeployedAt) is needed.
	//
	// Returns an empty slice if no configurations of the specified origin exist.
	GetAllConfigsByOrigin(origin models.Origin) ([]*models.StoredConfig, error)

	// UpdateCPSyncStatus updates the cp_sync_status, cp_sync_info, and cp_artifact_id
	// fields for an artifact.
	//
	// Used by the bottom-up sync to record sync outcomes (pending/success/failed) without
	// reloading the full configuration. The cpArtifactID is the APIM/control-plane UUID
	// returned by a successful import; pass an empty string when no CP UUID is known.
	// Returns ErrNotFound if the UUID does not exist.
	UpdateCPSyncStatus(uuid, cpArtifactID string, status models.CPSyncStatus, reason string) error

	// GetConfigByCPArtifactID looks up a config by the APIM/Control Plane UUID
	// assigned during bottom-up sync. Returns ErrNotFound if no match.
	GetConfigByCPArtifactID(cpArtifactID string) (*models.StoredConfig, error)

	// GetPendingBottomUpAPIs returns all RestApi artifacts with origin=gateway_api
	// and cp_sync_status IN ('pending', 'failed').
	//
	// Used by the bottom-up sync to determine which APIs need to be pushed to
	// the on-prem control plane. Returns an empty slice if none are pending.
	GetPendingBottomUpAPIs() ([]*models.StoredConfig, error)

	// ========================================
	// LLM Provider Template Methods
	// ========================================

	// SaveLLMProviderTemplate persists a new LLM provider template.
	//
	// Returns an error if a template with the same name already exists.
	// Implementations should ensure this operation is atomic (all-or-nothing).
	SaveLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error

	// UpdateLLMProviderTemplate updates an existing LLM provider template.
	//
	// Returns an error if the template does not exist.
	// Implementations should ensure this operation is atomic and thread-safe.
	UpdateLLMProviderTemplate(template *models.StoredLLMProviderTemplate) error

	// DeleteLLMProviderTemplate removes an LLM provider template by ID.
	//
	// Returns an error if the template does not exist.
	DeleteLLMProviderTemplate(id string) error

	// GetLLMProviderTemplate retrieves an LLM provider template by ID.
	//
	// Returns an error if the template is not found.
	// This is the fastest lookup method (O(1) for most databases).
	GetLLMProviderTemplate(id string) (*models.StoredLLMProviderTemplate, error)

	// GetAllLLMProviderTemplates retrieves all LLM provider templates.
	//
	// Returns an empty slice if no templates exist.
	// May be expensive for large datasets; consider pagination in future versions.
	GetAllLLMProviderTemplates() ([]*models.StoredLLMProviderTemplate, error)

	// GetLLMProviderTemplateByHandle retrieves an LLM provider template by handle.
	//
	// Returns an error if the template is not found.
	GetLLMProviderTemplateByHandle(handle string) (*models.StoredLLMProviderTemplate, error)

	// SaveAPIKey persists a new API key.
	//
	// Returns an error if an API key with the same key value already exists.
	// Implementations should ensure this operation is atomic (all-or-nothing).
	SaveAPIKey(apiKey *models.APIKey) error

	// UpsertAPIKey inserts or updates an API key identified by (gateway_id, artifact_uuid, name).
	//
	// If a key with the same name already exists for the artifact, it is updated only when the
	// incoming record's updated_at is strictly newer than the stored one — preventing a slow
	// bulk-sync goroutine from overwriting a more recent event-driven write.
	// The existing source and external_ref_id are preserved when the incoming values are absent.
	UpsertAPIKey(apiKey *models.APIKey) error

	// GetAPIKeyByID retrieves an API key by its ID.
	//
	// Returns an error if the API key is not found.
	// This is used for API key validation during authentication.
	GetAPIKeyByID(id string) (*models.APIKey, error)

	// GetAPIKeyByUUID retrieves an API key by its platform UUID.
	//
	// Returns an error if the API key is not found.
	GetAPIKeyByUUID(uuid string) (*models.APIKey, error)

	// GetAPIKeyByKey retrieves an API key by its key value.
	//
	// Returns an error if the API key is not found.
	// This is used for API key validation during authentication.
	GetAPIKeyByKey(key string) (*models.APIKey, error)

	// GetAPIKeysByAPI retrieves all API keys for a specific API.
	//
	// Returns an empty slice if no API keys exist for the API.
	// Used for listing API keys associated with an API.
	GetAPIKeysByAPI(apiId string) ([]*models.APIKey, error)

	// GetAllAPIKeys retrieves all active API keys from the database.
	//
	// Returns an empty slice if no active API keys exist.
	// Used for loading active API keys into memory on startup.
	GetAllAPIKeys() ([]*models.APIKey, error)

	// GetAPIKeysByAPIAndName retrieves an API key by its name within a specific API.
	//
	// Returns an error if the API key is not found.
	// Used for retrieving specific API keys by name.
	GetAPIKeysByAPIAndName(apiId, name string) (*models.APIKey, error)

	// UpdateAPIKey updates an existing API key (e.g., to revoke or expire it).
	//
	// Returns an error if the API key does not exist.
	// Implementations should ensure this operation is atomic and thread-safe.
	UpdateAPIKey(apiKey *models.APIKey) error

	// DeleteAPIKey removes an API key by its key value.
	//
	// Returns an error if the API key does not exist.
	DeleteAPIKey(key string) error

	// RemoveAPIKeysAPI removes all API keys for a specific API.
	//
	// Returns an error if API key removal fails.
	RemoveAPIKeysAPI(apiId string) error

	// RemoveAPIKeyAPIAndName removes an API key by its API apiId and name.
	//
	// Returns an error if the API key does not exist.
	RemoveAPIKeyAPIAndName(apiId, name string) error

	// CountActiveAPIKeysByUserAndAPI counts active API keys for a specific user and API.
	//
	// Returns the count of active API keys and an error if the operation fails.
	CountActiveAPIKeysByUserAndAPI(apiId, userID string) (int, error)

	// ListAPIKeysForArtifactsNotIn returns the minimal key info (uuid + artifact_uuid) for
	// keys whose artifact_uuid is in artifactUUIDs but whose own UUID is not in keyUUIDs.
	// Used to collect identifiers before deletion so callers can publish EventHub events.
	ListAPIKeysForArtifactsNotIn(artifactUUIDs []string, keyUUIDs []string) ([]*models.APIKey, error)

	// DeleteAPIKeysByUUIDs removes API keys by their UUIDs. Used after ListAPIKeysForArtifactsNotIn
	// has already identified the stale keys, avoiding a redundant NOT IN query.
	DeleteAPIKeysByUUIDs(uuids []string) error

	// ========================================
	// Subscription Plan Methods
	// ========================================

	SaveSubscriptionPlan(plan *models.SubscriptionPlan) error
	GetSubscriptionPlanByID(id, gatewayID string) (*models.SubscriptionPlan, error)
	ListSubscriptionPlans(gatewayID string) ([]*models.SubscriptionPlan, error)
	UpdateSubscriptionPlan(plan *models.SubscriptionPlan) error
	DeleteSubscriptionPlan(id, gatewayID string) error

	// DeleteSubscriptionPlansNotIn removes plans for this gateway whose IDs are not in the given set.
	// Used for bulk-sync reconciliation when plans were deleted on the control plane during downtime.
	DeleteSubscriptionPlansNotIn(ids []string) error

	// ========================================
	// Subscription Methods (application-level API subscriptions)
	// ========================================

	// SaveSubscription persists a new subscription.
	SaveSubscription(sub *models.Subscription) error

	// GetSubscriptionByID retrieves a subscription by ID and gateway.
	GetSubscriptionByID(id, gatewayID string) (*models.Subscription, error)

	// ListSubscriptionsByAPI returns subscriptions for an API with optional filters.
	ListSubscriptionsByAPI(apiID, gatewayID string, applicationID *string, status *string) ([]*models.Subscription, error)

	// ListActiveSubscriptions returns all ACTIVE subscriptions for this gateway in one query.
	// Used for bulk snapshot generation to avoid N+1 per-API lookups.
	ListActiveSubscriptions() ([]*models.Subscription, error)

	// UpdateSubscription updates an existing subscription.
	UpdateSubscription(sub *models.Subscription) error

	// DeleteSubscription removes a subscription by ID and gateway.
	DeleteSubscription(id, gatewayID string) error

	// DeleteSubscriptionsForAPINotIn removes subscriptions for the given API whose IDs are not in the set.
	// Used for bulk-sync reconciliation when subscriptions were deleted on the control plane during downtime.
	DeleteSubscriptionsForAPINotIn(apiID string, ids []string) error
	// ReplaceApplicationAPIKeyMappings atomically replaces all API key mappings for an application.
	//
	// Existing mappings are removed and the supplied mapping set is inserted.
	ReplaceApplicationAPIKeyMappings(application *models.StoredApplication, mappings []*models.ApplicationAPIKeyMapping) error

	// SaveCertificate persists a new certificate.
	//
	// Returns an error if a certificate with the same name already exists.
	// Implementations should ensure this operation is atomic.
	SaveCertificate(cert *models.StoredCertificate) error

	// GetCertificate retrieves a certificate by ID.
	//
	// Returns an error if the certificate is not found.
	GetCertificate(id string) (*models.StoredCertificate, error)

	// GetCertificateByName retrieves a certificate by name.
	//
	// Returns an error if the certificate is not found.
	GetCertificateByName(name string) (*models.StoredCertificate, error)

	// ListCertificates retrieves all certificates ordered by creation time.
	//
	// Returns an empty slice if no certificates exist.
	ListCertificates() ([]*models.StoredCertificate, error)

	// DeleteCertificate removes a certificate by ID.
	//
	// Returns an error if the certificate does not exist.
	DeleteCertificate(id string) error

	// SaveSecret persists a new encrypted secret.
	//
	// Returns an error if a secret with the same handle already exists.
	// Implementations should ensure this operation is atomic.
	SaveSecret(secret *models.Secret) error

	// GetSecrets retrieves metadata for all secrets.
	//
	// Returns non-sensitive metadata (handle, display_name, timestamps) without
	// ciphertext or values. Returns an empty slice if no secrets exist.
	GetSecrets() ([]models.SecretMeta, error)

	// GetSecret retrieves a secret by handle.
	//
	// Returns error if the secret does not exist.
	GetSecret(handle string) (*models.Secret, error)

	// UpdateSecret updates an existing secret.
	//
	// Returns the updated secret (including database-assigned timestamps) or error
	// if the secret does not exist. Implementations should ensure this operation is atomic.
	UpdateSecret(secret *models.Secret) (*models.Secret, error)

	// DeleteSecret permanently removes a secret.
	//
	// Returns error if the secret does not exist.
	DeleteSecret(handle string) error

	// SecretExists checks if a secret with the given handle exists.
	//
	// Returns true if the secret exists, false otherwise.
	SecretExists(handle string) (bool, error)

	// GetDB returns the underlying *sql.DB for direct access.
	// Used by EventHub for event synchronization.
	// Returns nil for non-SQL backends.
	GetDB() *sql.DB

	// Close closes the storage connection and releases resources.
	//
	// Should be called during graceful shutdown.
	// Implementations should ensure all pending writes are flushed.
	Close() error
}

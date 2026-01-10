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

	// GetConfigByNameVersion retrieves an API configuration by name and version.
	//
	// Returns an error if the configuration is not found.
	// This is the most common lookup method for API operations.
	// Implementations should index (name, version) for fast lookups.
	GetConfigByNameVersion(name, version string) (*models.StoredConfig, error)

	// GetConfigByHandle retrieves an API configuration by handle.
	//
	// Returns an error if the configuration is not found.
	// The handle is the metadata.name from the API YAML configuration.
	// This is the recommended lookup method for REST API endpoints.
	GetConfigByHandle(handle string) (*models.StoredConfig, error)

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

	// SaveAPIKey persists a new API key.
	//
	// Returns an error if an API key with the same key value already exists.
	// Implementations should ensure this operation is atomic (all-or-nothing).
	SaveAPIKey(apiKey *models.APIKey) error

	// GetAPIKeyByID retrieves an API key by its ID.
	//
	// Returns an error if the API key is not found.
	// This is used for API key validation during authentication.
	GetAPIKeyByID(id string) (*models.APIKey, error)

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

	// GetAllAPIKeys retrieves all API keys from the database.
	//
	// Returns an empty slice if no API keys exist.
	// Used for loading API keys into memory on startup.
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

	// Close closes the storage connection and releases resources.
	//
	// Should be called during graceful shutdown.
	// Implementations should ensure all pending writes are flushed.
	Close() error
}

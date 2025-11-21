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
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
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
	SaveConfig(cfg *models.StoredAPIConfig) error

	// UpdateConfig updates an existing API configuration.
	//
	// Returns an error if the configuration does not exist.
	// Implementations should ensure this operation is atomic and thread-safe.
	UpdateConfig(cfg *models.StoredAPIConfig) error

	// DeleteConfig removes an API configuration by ID.
	//
	// Returns an error if the configuration does not exist.
	// Implementations should ensure related data (if any) is cleaned up.
	DeleteConfig(id string) error

	// GetConfig retrieves an API configuration by ID.
	//
	// Returns an error if the configuration is not found.
	// This is the fastest lookup method (O(1) for most databases).
	GetConfig(id string) (*models.StoredAPIConfig, error)

	// GetConfigByNameVersion retrieves an API configuration by name and version.
	//
	// Returns an error if the configuration is not found.
	// This is the most common lookup method for API operations.
	// Implementations should index (name, version) for fast lookups.
	GetConfigByNameVersion(name, version string) (*models.StoredAPIConfig, error)

	// GetAllConfigs retrieves all API configurations.
	//
	// Returns an empty slice if no configurations exist.
	// May be expensive for large datasets; consider pagination in future versions.
	GetAllConfigs() ([]*models.StoredAPIConfig, error)

	// ReplacePolicyDefinitions atomically replaces all stored policy definitions with the provided slice.
	// Treats the input as the authoritative state of the world. Existing definitions are removed.
	// Should be executed within a single transaction for consistency.
	ReplacePolicyDefinitions(defs []api.PolicyDefinition) error

	// GetAllPolicyDefinitions retrieves all stored policy definitions.
	GetAllPolicyDefinitions() ([]api.PolicyDefinition, error)

	// Close closes the storage connection and releases resources.
	//
	// Should be called during graceful shutdown.
	// Implementations should ensure all pending writes are flushed.
	Close() error
}

# Feature Specification: Database Migration from BBolt to SQLite

**Feature Branch**: `005-migrate-sqlite-db`
**Created**: 2025-10-18
**Status**: Draft
**Input**: User description: "The database of gatewa/gateway-controller should be replaced from bbolt to SQLite. This is a full replacement no data migration is required. Remove all configs related to BBolt. No backward compatibility required. Check *.md files in gateway/spec directory to get a context of the existing implementation. Note that, the SQLite can be replacable from other SQL database engine like postgresql. Research on using text or blob for API configuration and use JSON/YAML to store in the DB. Remove the Audit Logging feature completely."

## Clarifications

### Session 2025-10-19

- Q: Which SQLite driver should the gateway-controller use? → A: mattn/go-sqlite3 (CGO-based driver)
- Q: Where should the SQLite database file be located? → A: ./data/gateway.db (relative to working directory)
- Q: How should the system handle a locked SQLite database file on startup? → A: Fail immediately with clear error message and exit

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Gateway Persistence with SQLite (Priority: P1)

As a gateway operator, I need the gateway-controller to persist API configurations across restarts using SQLite instead of BBolt, so that configurations are reliably stored and can be managed using standard SQL tooling.

**Why this priority**: This is the core value proposition of the migration. SQLite provides better ecosystem support, standard SQL query capabilities, and easier database management compared to BBolt's key-value approach. This enables operators to inspect and troubleshoot configurations using familiar SQL tools.

**Independent Test**: Can be fully tested by creating API configurations via REST API, restarting the gateway-controller, and verifying all configurations are restored. Delivers immediate value by proving persistence works with the new database engine.

**Acceptance Scenarios**:

1. **Given** the gateway-controller is configured in persistent mode with SQLite, **When** an operator submits a new API configuration via REST API, **Then** the configuration is stored in SQLite and survives controller restarts
2. **Given** multiple API configurations exist in SQLite, **When** the gateway-controller starts up, **Then** all configurations are loaded into memory and available via xDS for Envoy
3. **Given** the gateway-controller is running, **When** an operator updates an existing API configuration, **Then** the updated configuration is persisted to SQLite and reflected in Envoy
4. **Given** API configurations exist in the database, **When** an operator deletes an API configuration, **Then** the configuration is removed from SQLite and no longer appears in API queries

---

### User Story 2 - Simplified Testing with Memory-Only Mode (Priority: P2)

As a developer, I need to test the gateway-controller in memory-only mode without any persistent database, so that I can quickly validate API configurations during development without file system dependencies.

**Why this priority**: Enables fast development and testing workflows without needing to manage database files or clean up state between test runs. Essential for CI/CD pipelines and local development.

**Independent Test**: Can be fully tested by starting gateway-controller with `storage.mode: memory-only`, submitting API configurations, and verifying they work during the session but are lost after restart. Delivers value by proving the refactored storage layer supports both persistent and transient modes.

**Acceptance Scenarios**:

1. **Given** the gateway-controller is configured in memory-only mode, **When** a developer submits API configurations via REST API, **Then** configurations are stored in memory and available immediately without touching the filesystem
2. **Given** the gateway-controller is running in memory-only mode with configurations loaded, **When** the controller is restarted, **Then** all configurations are lost and the controller starts with an empty state
3. **Given** the gateway-controller is in memory-only mode, **When** a developer queries API configurations, **Then** all CRUD operations work identically to persistent mode

---

### User Story 3 - Future Database Engine Flexibility (Priority: P3)

As a platform architect, I need the gateway-controller's storage layer to be database-agnostic, so that I can migrate from SQLite to PostgreSQL or MySQL in the future without major code changes.

**Why this priority**: Enables enterprise deployments that require centralized database management, high availability, or multi-gateway coordination. SQLite is suitable for single-instance deployments, but larger deployments may need external databases.

**Independent Test**: Can be verified by reviewing the storage interface abstraction and confirming that SQLite-specific logic is isolated behind the interface. Delivers value by ensuring the architecture supports future extensibility without requiring a complete rewrite.

**Acceptance Scenarios**:

1. **Given** the gateway-controller storage layer, **When** reviewing the codebase, **Then** all database operations go through a common Storage interface with no SQLite-specific code in business logic
2. **Given** the SQLite implementation, **When** API configurations are stored, **Then** they are stored as standard TEXT (JSON format) that can be easily migrated to other SQL databases
3. **Given** the configuration system, **When** an operator reviews available settings, **Then** there are no BBolt-specific configuration options remaining

---

### Edge Cases

- **Empty database initialization**: What happens when the gateway-controller starts with a non-existent SQLite database file? System must create the database and initialize the schema automatically.
- **Corrupted database file**: How does the system handle a corrupted SQLite database on startup? System should log a clear error message and fail to start (fail-fast behavior preferred over silent corruption).
- **Locked database file**: What happens when the SQLite database file is locked by another process on startup? System must fail immediately with a clear error message and exit (prevents multi-instance conflicts and silent failures).
- **Concurrent write operations**: What happens when multiple API configuration changes are submitted simultaneously? System must handle concurrent writes safely using SQLite's transaction support and the in-memory cache's locking mechanisms.
- **Large configuration payloads**: How does the system handle API configurations with very large upstream lists or many operations? System must store configurations up to 1MB per configuration without performance degradation. Configurations larger than 1MB should be rejected with a clear error message indicating the size limit.
- **Database schema versioning**: What happens when the gateway-controller is upgraded and the SQLite schema needs to change? Future migrations should be supported through a schema version table (not required for initial implementation but design should accommodate it).
- **Startup with mismatched storage mode**: What happens if the operator changes from persistent to memory-only mode but an old database file exists? System should ignore the database file and log a warning.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST replace BBolt database with SQLite for persistent storage of API configurations
- **FR-002**: System MUST store API configurations as TEXT columns containing JSON-serialized data in SQLite
- **FR-003**: System MUST remove all BBolt dependencies, configuration options, and related code from the codebase
- **FR-004**: System MUST remove the audit logging feature entirely (including code, database tables, and API endpoints)
- **FR-005**: System MUST maintain the existing Storage interface contract without breaking changes to dependent code
- **FR-006**: System MUST support "sqlite", "postgres" (future), and "memory" storage types via `storage.type` configuration
- **FR-007**: System MUST automatically create the SQLite database and schema if they do not exist on startup when `storage.type=sqlite` (default path: `/data/gateway.db`, configurable via `storage.sqlite.path` setting)
- **FR-008**: System MUST load all API configurations from SQLite into the in-memory cache during controller startup when `storage.type=sqlite`
- **FR-009**: System MUST persist configuration changes (create, update, delete) to SQLite immediately when `storage.type=sqlite`
- **FR-010**: System MUST use SQLite transactions to ensure data consistency during write operations
- **FR-011**: System MUST maintain thread-safe operations for concurrent access to both SQLite and the in-memory cache
- **FR-012**: System MUST support querying API configurations by ID and by name/version composite key
- **FR-013**: System MUST isolate all SQLite-specific code behind the Storage interface to support future database migrations
- **FR-014**: System MUST update configuration file examples to remove BBolt settings and add SQLite settings
- **FR-015**: System MUST update documentation (README.md, architecture docs) to reflect the SQLite migration and removal of audit logging

### Key Entities *(include if feature involves data)*

- **API Configuration**: Represents a complete API definition including metadata (ID, name, version, timestamps), upstream endpoints, operations, and policies. Stored as JSON TEXT in SQLite's `api_configs` table with columns for ID (primary key), name, version, and configuration_json (TEXT).

- **Database Schema**: SQLite database with initial schema version including:
  - `api_configs` table: Stores API configurations with ID, name, version, created_at, updated_at, and configuration_json fields
  - Indexes on common query patterns: composite index on (name, version) for name/version lookups
  - **Driver**: Implementation uses mattn/go-sqlite3 (CGO-based, requires gcc at build time)
  - **File Location**: Default path is `/data/gateway.db`, configurable via `storage.sqlite.path` config option
  - **Configuration Structure**:
    ```yaml
    storage:
      type: sqlite      # "sqlite", "postgres" (future), or "memory"
      sqlite:
        path: /data/gateway.db
    ```

- **Storage Interface**: Abstraction layer defining methods for SaveConfig, UpdateConfig, DeleteConfig, GetConfig, GetConfigByNameVersion, GetAllConfigs, and Close. Implemented by both SQLiteStorage (persistent) and MemoryStorage (transient).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Gateway-controller starts successfully with `storage.type=sqlite` and creates SQLite database automatically if it does not exist
- **SC-002**: All API configuration operations (create, read, update, delete) complete with p95 latency under 1 second for databases with up to 100 configurations
- **SC-003**: API configurations survive gateway-controller restarts with zero data loss when `storage.type=sqlite`
- **SC-004**: Gateway-controller operates correctly with `storage.type=memory` without creating any database files
- **SC-005**: SQLite database file size grows predictably (approximately 5-10 KB per API configuration stored)
- **SC-006**: No BBolt-related code, dependencies, or configuration options remain in the codebase after migration
- **SC-007**: No audit logging code, API endpoints, or database tables remain in the codebase after removal
- **SC-008**: Existing test suites pass with SQLite implementation (or are updated to reflect removal of audit logging)
- **SC-009**: System handles 10 concurrent API configuration updates without errors or data corruption
- **SC-010**: Documentation and configuration examples accurately reflect SQLite usage and removal of deprecated features

# Data Model: SQLite Database Schema

**Feature**: Database Migration from BBolt to SQLite
**Branch**: `005-migrate-sqlite-db`
**Version**: 1.0 (Initial Schema)

## Overview

The gateway-controller uses SQLite to persist API configurations and maintain state across restarts. The schema is designed for:
- Fast lookups by ID and by name/version composite key
- Thread-safe concurrent operations via SQLite's WAL mode
- Future migration compatibility with PostgreSQL
- Simple operational debugging using standard SQL tools

---

## Database Configuration

### SQLite Settings

| Parameter | Value | Purpose |
|-----------|-------|---------|
| **Journal Mode** | WAL | Enables concurrent reads during writes |
| **Busy Timeout** | 5000ms | Retry locked database for 5 seconds before failing |
| **Synchronous** | NORMAL | Balanced durability (faster than FULL, safer than OFF) |
| **Cache Size** | 2000 pages | In-memory cache (approx 2MB) |
| **Foreign Keys** | ON | Enable referential integrity |

**Connection String**:
```
file:./data/gateway.db?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_cache_size=2000&_foreign_keys=ON
```

### File Locations

| File | Purpose | Size (100 configs) |
|------|---------|-------------------|
| `./data/gateway.db` | Main database file | ~500-1000 KB |
| `./data/gateway.db-wal` | Write-Ahead Log (transactions) | Variable (0-32 KB) |
| `./data/gateway.db-shm` | Shared memory for WAL | 32 KB |

**Default Path**: `./data/gateway.db` (relative to working directory)
**Configurable Via**: `storage.path` config option

---

## Schema Version 1

### Table: `api_configs`

Stores API configuration definitions with full lifecycle metadata.

#### DDL

```sql
CREATE TABLE api_configs (
    -- Primary identifier (UUID)
    id TEXT PRIMARY KEY,

    -- Extracted fields for fast querying
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    context TEXT NOT NULL,              -- Base path (e.g., "/weather")
    kind TEXT NOT NULL,                  -- API type: "http/rest", "graphql", "grpc", "asyncapi"

    -- Full API configuration as JSON
    configuration TEXT NOT NULL,         -- JSON-serialized APIConfiguration

    -- Deployment status
    status TEXT NOT NULL CHECK(status IN ('pending', 'deployed', 'failed')),

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deployed_at TIMESTAMP,               -- NULL until first deployment

    -- Version tracking for xDS snapshots
    deployed_version INTEGER NOT NULL DEFAULT 0,

    -- Composite unique constraint
    UNIQUE(name, version)
);
```

#### Indexes

```sql
-- Composite index for name+version lookups (most common query)
CREATE INDEX idx_name_version ON api_configs(name, version);

-- Filter by deployment status (translator queries pending configs)
CREATE INDEX idx_status ON api_configs(status);

-- Filter by context path (conflict detection)
CREATE INDEX idx_context ON api_configs(context);

-- Filter by API type (reporting/analytics)
CREATE INDEX idx_kind ON api_configs(kind);
```

#### Field Definitions

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | TEXT | No | (required) | UUID primary key (e.g., "550e8400-e29b-41d4-a716-446655440000") |
| `name` | TEXT | No | (required) | API name extracted from configuration (e.g., "Weather API") |
| `version` | TEXT | No | (required) | API version extracted from configuration (e.g., "v1.0") |
| `context` | TEXT | No | (required) | Base path extracted from configuration (e.g., "/weather") |
| `kind` | TEXT | No | (required) | API type from configuration (e.g., "http/rest", "graphql") |
| `configuration` | TEXT | No | (required) | Full JSON-serialized APIConfiguration object |
| `status` | TEXT | No | (required) | Deployment status: "pending", "deployed", or "failed" |
| `created_at` | TIMESTAMP | No | CURRENT_TIMESTAMP | Record creation timestamp (RFC3339) |
| `updated_at` | TIMESTAMP | No | CURRENT_TIMESTAMP | Last modification timestamp (RFC3339) |
| `deployed_at` | TIMESTAMP | Yes | NULL | Timestamp of successful deployment to Envoy (NULL = never deployed) |
| `deployed_version` | INTEGER | No | 0 | xDS snapshot version number (increments on each deployment) |

#### Constraints

1. **Primary Key**: `id` (UUID uniqueness enforced)
2. **Unique Composite Key**: `(name, version)` prevents duplicate API versions
   - Example: Only one "PetStore/v1" configuration can exist
3. **Check Constraint**: `status IN ('pending', 'deployed', 'failed')`

#### Sample Data

```sql
INSERT INTO api_configs (
    id, name, version, context, kind, configuration,
    status, created_at, updated_at, deployed_version
) VALUES (
    '550e8400-e29b-41d4-a716-446655440000',
    'Weather API',
    'v1.0',
    '/weather',
    'http/rest',
    '{"version":"api-platform.wso2.com/v1","kind":"http/rest","data":{"name":"Weather API","version":"v1.0","context":"/weather","upstream":[{"url":"http://api.weather.com"}],"operations":[{"method":"GET","path":"/{country}/{city}"}]}}',
    'pending',
    '2025-10-19T10:00:00Z',
    '2025-10-19T10:00:00Z',
    0
);
```

---

## Entity Relationships

### StoredAPIConfig ↔ Configuration JSON

The `configuration` column stores the full `APIConfiguration` object as JSON:

```go
type StoredAPIConfig struct {
    ID              string                    `db:"id"`
    Name            string                    `db:"name"`
    Version         string                    `db:"version"`
    Context         string                    `db:"context"`
    Kind            string                    `db:"kind"`
    Configuration   models.APIConfiguration   `db:"configuration"`  // Marshaled to JSON
    Status          models.ConfigStatus       `db:"status"`
    CreatedAt       time.Time                 `db:"created_at"`
    UpdatedAt       time.Time                 `db:"updated_at"`
    DeployedAt      *time.Time                `db:"deployed_at"`    // Pointer for NULL
    DeployedVersion int64                     `db:"deployed_version"`
}
```

**Example JSON** (stored in `configuration` column):
```json
{
  "version": "api-platform.wso2.com/v1",
  "kind": "http/rest",
  "data": {
    "name": "Weather API",
    "version": "v1.0",
    "context": "/weather",
    "upstream": [
      {"url": "http://api.weather.com/api/v2"}
    ],
    "operations": [
      {
        "method": "GET",
        "path": "/{country}/{city}",
        "requestPolicies": [
          {"name": "apiKey", "params": {"header": "X-API-Key"}}
        ]
      }
    ]
  }
}
```

---

## State Transitions

### Configuration Status Lifecycle

```
[API Created via REST API]
         ↓
    status: "pending"
         ↓
[xDS Translator Processes Config]
         ↓
    status: "deployed"  (success)
         OR
    status: "failed"    (error)
         ↓
[Operator Updates Config]
         ↓
    status: "pending"   (reset for re-deployment)
```

### Field State Transitions

| Action | `status` | `deployed_at` | `deployed_version` | `updated_at` |
|--------|----------|---------------|-------------------|--------------|
| **Create** | `pending` | `NULL` | `0` | `CURRENT_TIMESTAMP` |
| **Deploy Success** | `deployed` | `CURRENT_TIMESTAMP` | `+1` | (unchanged) |
| **Deploy Failure** | `failed` | (unchanged) | (unchanged) | (unchanged) |
| **Update** | `pending` | (unchanged) | (unchanged) | `CURRENT_TIMESTAMP` |
| **Re-deploy** | `deployed` | `CURRENT_TIMESTAMP` | `+1` | (unchanged) |

---

## Query Patterns

### Common Queries

#### 1. Get Config by ID (Primary Lookup)
```sql
SELECT id, configuration, status, created_at, updated_at, deployed_at, deployed_version
FROM api_configs
WHERE id = ?;
```
**Performance**: O(1) via primary key index (~1-5ms)

#### 2. Get Config by Name and Version (Most Common)
```sql
SELECT id, configuration, status, created_at, updated_at, deployed_at, deployed_version
FROM api_configs
WHERE name = ? AND version = ?;
```
**Performance**: O(log n) via `idx_name_version` index (~5-10ms)

#### 3. Get All Configs
```sql
SELECT id, configuration, status, created_at, updated_at, deployed_at, deployed_version
FROM api_configs
ORDER BY created_at DESC;
```
**Performance**: O(n) full table scan (~50-200ms for 100 configs)

#### 4. Get Pending Configs (xDS Translator)
```sql
SELECT id, configuration, status, created_at, updated_at, deployed_at, deployed_version
FROM api_configs
WHERE status = 'pending'
ORDER BY created_at ASC;
```
**Performance**: O(k) via `idx_status` index (~10-50ms for k pending configs)

#### 5. Check for Duplicate Name/Version
```sql
SELECT EXISTS(
    SELECT 1 FROM api_configs WHERE name = ? AND version = ?
);
```
**Performance**: O(log n) via `idx_name_version` index (~1-5ms)

---

## Validation Rules

### Database-Level Constraints

1. **Primary Key Uniqueness**: `id` must be globally unique UUID
2. **Composite Key Uniqueness**: `(name, version)` tuple must be unique
3. **Status Enum**: `status` must be one of: `pending`, `deployed`, `failed`
4. **Timestamps**: `created_at` and `updated_at` auto-populated by database

### Application-Level Validation (Go Code)

```go
// Before SaveConfig
func ValidateConfig(cfg *models.StoredAPIConfig) error {
    if cfg.ID == "" {
        return fmt.Errorf("id is required")
    }
    if cfg.GetAPIName() == "" {
        return fmt.Errorf("name is required")
    }
    if cfg.GetAPIVersion() == "" {
        return fmt.Errorf("version is required")
    }
    if cfg.Configuration.Data.Context == "" {
        return fmt.Errorf("context is required")
    }
    if cfg.Configuration.Kind == "" {
        return fmt.Errorf("kind is required")
    }
    return nil
}
```

---

## Migration from BBolt

### Schema Mapping

| BBolt Bucket | BBolt Key | SQLite Table | SQLite Column |
|--------------|-----------|--------------|---------------|
| `apis` | UUID | `api_configs` | `id` (primary key) |
| (embedded) | N/A | `api_configs` | `name` (extracted) |
| (embedded) | N/A | `api_configs` | `version` (extracted) |
| (embedded) | N/A | `api_configs` | `context` (extracted) |
| (embedded) | N/A | `api_configs` | `kind` (extracted) |
| (embedded) | JSON value | `api_configs` | `configuration` |
| (embedded) | N/A | `api_configs` | `status` |
| (embedded) | N/A | `api_configs` | `created_at` |
| (embedded) | N/A | `api_configs` | `updated_at` |
| (embedded) | N/A | `api_configs` | `deployed_at` |
| (embedded) | N/A | `api_configs` | `deployed_version` |
| `audit` | (deleted) | (removed) | (removed) |
| `metadata` | (deleted) | (removed) | (removed) |

**Note**: Audit logging feature is completely removed. No migration for audit data.

---

## Future Schema Evolution

### Version 2 (Example: Add API Tags)

```sql
-- Add tags column
ALTER TABLE api_configs ADD COLUMN tags TEXT;  -- JSON array

-- Update existing rows
UPDATE api_configs SET tags = '[]' WHERE tags IS NULL;

-- Add index
CREATE INDEX idx_tags ON api_configs(tags);

-- Update user_version
PRAGMA user_version = 2;
```

### Migration to PostgreSQL

**Minimal Changes Required**:

1. Change `TEXT` to `JSONB` for `configuration` column
2. Add GIN index for JSON queries
3. Update connection string

```sql
-- PostgreSQL schema (future)
CREATE TABLE api_configs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    context TEXT NOT NULL,
    kind TEXT NOT NULL,
    configuration JSONB NOT NULL,  -- Native JSONB type
    status TEXT NOT NULL CHECK(status IN ('pending', 'deployed', 'failed')),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deployed_at TIMESTAMP,
    deployed_version BIGINT NOT NULL DEFAULT 0,
    UNIQUE(name, version)
);

CREATE INDEX idx_name_version ON api_configs(name, version);
CREATE INDEX idx_config_gin ON api_configs USING gin(configuration);
```

---

## Performance Characteristics

### Expected Latencies (100 Configurations)

| Operation | Expected Time | Constraint Met |
|-----------|---------------|----------------|
| GetConfig by ID | 1-5ms | ✅ SC-002 (< 1s) |
| GetConfigByNameVersion | 5-10ms | ✅ SC-002 (< 1s) |
| SaveConfig | 50-100ms | ✅ SC-002 (< 1s) |
| UpdateConfig | 50-100ms | ✅ SC-002 (< 1s) |
| DeleteConfig | 10-50ms | ✅ SC-002 (< 1s) |
| GetAllConfigs | 50-200ms | ✅ SC-002 (< 1s) |

### Database Size Projections

| # Configs | Database Size | WAL Size | Total |
|-----------|---------------|----------|-------|
| 10 | ~50 KB | ~5 KB | ~55 KB |
| 100 | ~500 KB | ~10 KB | ~510 KB |
| 1000 | ~5 MB | ~50 KB | ~5.05 MB |
| 10000 | ~50 MB | ~500 KB | ~50.5 MB |

**Assumption**: Average configuration size ~5-10 KB (including JSON serialization overhead)

---

## Operational Procedures

### Database Backup

```bash
# 1. Checkpoint WAL to main database file
sqlite3 ./data/gateway.db "PRAGMA wal_checkpoint(TRUNCATE);"

# 2. Copy database file
cp ./data/gateway.db ./backups/gateway-$(date +%Y%m%d-%H%M%S).db

# 3. Verify backup integrity
sqlite3 ./backups/gateway-*.db "PRAGMA integrity_check;"
```

### Database Inspection

```bash
# Connect to database
sqlite3 ./data/gateway.db

# List all APIs
SELECT name, version, status FROM api_configs;

# View specific API (pretty-print JSON)
SELECT json(configuration) FROM api_configs WHERE name = 'Weather API';

# Check database size
.dbinfo

# Verify schema
.schema api_configs

# Count configurations by status
SELECT status, COUNT(*) FROM api_configs GROUP BY status;
```

### Troubleshooting Locked Database

```bash
# Check for active connections
lsof ./data/gateway.db

# Force unlock (DANGER: only if no processes running)
rm ./data/gateway.db-wal ./data/gateway.db-shm

# Verify integrity after unlock
sqlite3 ./data/gateway.db "PRAGMA integrity_check;"
```

---

## Testing Checklist

- ✅ Verify UNIQUE constraint on (name, version)
- ✅ Test NULL handling for `deployed_at` field
- ✅ Verify status CHECK constraint rejects invalid values
- ✅ Test concurrent reads during write (WAL mode)
- ✅ Verify auto-populated timestamps (`created_at`, `updated_at`)
- ✅ Test foreign key constraint (if audit table present)
- ✅ Verify index usage via `EXPLAIN QUERY PLAN`
- ✅ Test database creation from scratch (empty data directory)
- ✅ Test schema initialization on first startup
- ✅ Verify error handling for duplicate (name, version) insert
- ✅ Test database file growth up to 100 configurations
- ✅ Verify backup and restore procedures
- ✅ Test locked database error handling on startup

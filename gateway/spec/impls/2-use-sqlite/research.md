# SQLite Implementation Research for Gateway-Controller

**Feature**: Database Migration from BBolt to SQLite
**Branch**: `005-migrate-sqlite-db`
**Research Date**: 2025-10-19

## Executive Summary

This research provides comprehensive guidance for migrating from BBolt to SQLite for storing API configurations in the gateway-controller. The recommendations prioritize production-readiness, operational simplicity, and future migration paths to PostgreSQL.

**Configuration Structure**:
```yaml
storage:
  type: sqlite      # Database type: "sqlite", "postgres" (future), or "memory"
  sqlite:
    path: ./data/gateway.db  # SQLite database file path
  # Future PostgreSQL support:
  # postgres:
  #   host: localhost
  #   port: 5432
  #   database: gateway
  #   user: gateway_user
  #   password: secret
  #   sslmode: require
```

---

## Key Decisions

### 1. SQLite Driver: mattn/go-sqlite3

**Decision**: Use `github.com/mattn/go-sqlite3` (CGO-based driver)

**Rationale**:
- **Performance**: 25-50% faster than pure Go alternatives (modernc.org/sqlite)
- **Battle-tested**: 10+ years of production use, 8,000+ GitHub stars
- **Feature completeness**: Direct C library binding ensures full SQLite support
- **Docker deployment**: CGO build complexity isolated to CI/CD pipeline

**Alternatives considered**:
- `modernc.org/sqlite` (pure Go) - rejected due to performance penalty and less maturity
- Custom CGO wrapper - rejected as reinventing the wheel

**Trade-offs**:
- ❌ Requires gcc at build time
- ❌ Cross-compilation more complex
- ✅ Acceptable because platform ships via Docker containers

---

### 2. Storage Format: TEXT with JSON

**Decision**: Store API configurations as TEXT columns containing JSON-serialized data

**Rationale**:
- **Best debugging experience**: Human-readable in sqlite3 CLI
- **Query performance**: Composite index on (name, version) eliminates full table scans
- **PostgreSQL compatibility**: Maps directly to JSONB column type for seamless migration
- **Operational simplicity**: Operators can inspect configs without special tools
- **Future extensibility**: Configuration structure supports both SQLite and PostgreSQL via `storage.type` field

**Alternatives considered**:
- BLOB with JSONB binary format - rejected due to poor debugging experience
- Fully normalized tables - rejected as over-engineering for current needs

**Code Example**:
```sql
CREATE TABLE api_configs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    configuration TEXT NOT NULL,  -- JSON as TEXT
    status TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deployed_at TIMESTAMP,
    deployed_version INTEGER NOT NULL DEFAULT 0,
    UNIQUE(name, version)
);
```

---

### 3. Connection Pooling: Single Writer Connection

**Decision**: `db.SetMaxOpenConns(1)` to prevent "database is locked" errors

**Rationale**:
- SQLite's default locking causes errors with multiple connections
- Single connection is sufficient for 100+ configurations (< 1s per operation)
- WAL mode enables concurrent reads during writes

**Configuration**:
```go
func NewSQLiteDB(dbPath string) (*sql.DB, error) {
    dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_cache_size=2000&_foreign_keys=ON", dbPath)

    db, err := sql.Open("sqlite3", dsn)
    if err != nil {
        return nil, err
    }

    // CRITICAL: Prevents "database is locked" errors
    db.SetMaxOpenConns(1)
    db.SetMaxIdleConns(1)
    db.SetConnMaxLifetime(0)

    return db, nil
}
```

**Alternatives considered**:
- Read/write connection split - deferred as future optimization if needed
- Connection pool with retries - rejected as unnecessarily complex

---

### 4. Journal Mode: WAL (Write-Ahead Logging)

**Decision**: Enable WAL mode via connection string parameter

**Rationale**:
- **Concurrent reads**: Allows reads during writes (critical for API gateway)
- **2-10x faster writes**: Compared to default DELETE journal mode
- **Industry standard**: SQLite 3.7.0+ (released 2010)

**Configuration**:
```go
dsn := "file:gateway.db?_journal_mode=WAL&_busy_timeout=5000"
```

**Trade-offs**:
- ✅ Creates additional files (.db-wal, .db-shm) - acceptable
- ❌ Not compatible with NFS - not an issue for local deployment
- ✅ Requires periodic checkpointing - automated via cron or daily task

---

### 5. Schema Design: Hybrid Approach

**Decision**: Structured columns (id, name, version, context, kind) + JSON configuration

**Rationale**:
- **Fast name/version lookups**: Composite index eliminates full table scans (most common query)
- **Debugging**: Key fields visible without JSON parsing
- **Future-proof**: Easy migration to PostgreSQL with minimal schema changes

**Final Schema**:
```sql
CREATE TABLE api_configs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    context TEXT NOT NULL,            -- Extracted for fast queries
    kind TEXT NOT NULL,                -- 'RestApi', 'graphql', etc.
    configuration TEXT NOT NULL,       -- Full JSON
    status TEXT NOT NULL CHECK(status IN ('pending', 'deployed', 'failed')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deployed_at TIMESTAMP,
    deployed_version INTEGER NOT NULL DEFAULT 0,
    UNIQUE(name, version)
);

-- Indexes for fast lookups
CREATE INDEX idx_name_version ON api_configs(name, version);
CREATE INDEX idx_status ON api_configs(status);
CREATE INDEX idx_context ON api_configs(context);
CREATE INDEX idx_kind ON api_configs(kind);
```

**Alternatives considered**:
- TEXT JSON only - rejected due to slow name/version queries
- Fully normalized - rejected as premature optimization

---

## Performance Benchmarks

### Driver Comparison

| Operation | mattn/go-sqlite3 | modernc.org/sqlite | Winner |
|-----------|------------------|-------------------|--------|
| Small dataset INSERTs | Baseline | 2x slower | mattn |
| Small dataset SELECTs | Baseline | 10-20% slower | mattn |
| Large dataset operations | Baseline | 1.5-2x slower | mattn |
| Overall throughput | 100% | ~75% | mattn |

### Expected Performance (100 Configurations)

| Operation | Expected Latency | Constraint Met |
|-----------|-----------------|----------------|
| GetConfig by ID | < 10ms | ✅ SC-002 (< 1s) |
| GetConfigByNameVersion | < 50ms | ✅ SC-002 (< 1s) |
| SaveConfig | < 100ms | ✅ SC-002 (< 1s) |
| UpdateConfig | < 100ms | ✅ SC-002 (< 1s) |
| GetAllConfigs | < 200ms | ✅ SC-002 (< 1s) |
| 10 concurrent writes | < 1s | ✅ SC-009 |

---

## Error Handling Best Practices

### SQLite-Specific Error Patterns

```go
import (
    "database/sql"
    "errors"
    "github.com/mattn/go-sqlite3"
)

var (
    ErrNotFound       = errors.New("configuration not found")
    ErrAlreadyExists  = errors.New("configuration already exists")
    ErrDatabaseLocked = errors.New("database is locked")
)

func (s *SQLiteStorage) SaveConfig(cfg *models.StoredAPIConfig) error {
    _, err := s.db.Exec(`INSERT INTO api_configs (...)`, ...)

    if err != nil {
        // Check for UNIQUE constraint violation
        if sqliteErr, ok := err.(sqlite3.Error); ok {
            if sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
                return fmt.Errorf("%w: %s/%s", ErrAlreadyExists, cfg.Name, cfg.Version)
            }
            if sqliteErr.Code == sqlite3.ErrBusy {
                return ErrDatabaseLocked
            }
        }
        return fmt.Errorf("insert config: %w", err)
    }

    return nil
}
```

---

## Transaction Management

### Simple Transaction Wrapper

```go
func (s *SQLiteStorage) withTx(fn func(*sql.Tx) error) error {
    tx, err := s.db.Begin()
    if err != nil {
        return fmt.Errorf("begin transaction: %w", err)
    }

    defer func() {
        if p := recover(); p != nil {
            tx.Rollback()
            panic(p)
        }
    }()

    if err := fn(tx); err != nil {
        tx.Rollback()
        return err
    }

    return tx.Commit()
}
```

---

## Schema Initialization Strategy

### Embedded SQL Schema (Recommended)

**Decision**: Use embedded SQL file with `go:embed` for simplicity

**Rationale**:
- Self-contained (no external migration files)
- Simple version tracking via `PRAGMA user_version`
- Suitable for single-schema initial release

**Code**:
```go
//go:embed schema.sql
var schemaSQL string

func (s *SQLiteStorage) initSchema() error {
    // Check schema version
    var version int
    _ = s.db.QueryRow("PRAGMA user_version").Scan(&version)

    if version == 0 {
        // Create initial schema
        if _, err := s.db.Exec(schemaSQL); err != nil {
            return fmt.Errorf("create schema: %w", err)
        }

        // Set version
        _, err := s.db.Exec("PRAGMA user_version = 1")
        return err
    }

    return nil
}
```

**Alternatives considered**:
- golang-migrate - deferred for future schema evolution
- Goose - deferred for future schema evolution

---

## Future Migration to PostgreSQL

### Compatibility Design

The chosen schema design and configuration structure ensure minimal changes for PostgreSQL migration:

**Configuration** (SQLite - Current):
```yaml
storage:
  type: sqlite
  sqlite:
    path: ./data/gateway.db
```

**Configuration** (PostgreSQL - Future):
```yaml
storage:
  type: postgres
  postgres:
    host: localhost
    port: 5432
    database: gateway
    user: gateway_user
    password: secret
    sslmode: require
```

**SQLite Schema** (Current):
```sql
CREATE TABLE api_configs (
    id TEXT PRIMARY KEY,
    configuration TEXT NOT NULL,  -- JSON as TEXT
    ...
);
```

**PostgreSQL Schema** (Future):
```sql
CREATE TABLE api_configs (
    id TEXT PRIMARY KEY,
    configuration JSONB NOT NULL,  -- Native JSONB
    ...
);

CREATE INDEX idx_config_gin ON api_configs USING gin(configuration);
```

**Code Changes Required**: ~20-30 lines
- Add PostgreSQL storage implementation (new file `pkg/storage/postgres.go`)
- Update storage factory to support `storage.type` routing
- Update import: `_ "github.com/lib/pq"`
- Build DSN from `storage.postgres.*` config fields
- No query changes (database/sql abstraction)

---

## Production Deployment Checklist

- ✅ Enable WAL mode (`_journal_mode=WAL`)
- ✅ Set busy timeout (`_busy_timeout=5000`)
- ✅ Limit connection pool (`SetMaxOpenConns(1)`)
- ✅ Create composite index on (name, version)
- ✅ Use transactions for multi-step operations
- ✅ Test concurrent write scenarios (10 goroutines)
- ✅ Verify SQLite version >= 3.38.0 (for JSON functions)
- ✅ Document database file location (./data/gateway.db)
- ✅ Plan backup strategy (file-based copy after WAL checkpoint)
- ✅ Test locked database error handling (fail-fast on startup)
- ✅ Remove all BBolt dependencies from go.mod
- ✅ Update configuration examples (config.yaml, config-memory-only.yaml)

---

## References

1. SQLite WAL Mode: https://www.sqlite.org/wal.html
2. mattn/go-sqlite3 documentation: https://github.com/mattn/go-sqlite3
3. SQLite JSON functions: https://www.sqlite.org/json1.html
4. database/sql best practices: https://go.dev/doc/database/sql
5. Gateway-controller existing implementation: `/Users/renuka/git/api-platform/gateway/gateway-controller/pkg/storage/`

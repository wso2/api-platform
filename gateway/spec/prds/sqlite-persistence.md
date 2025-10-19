# SQLite Persistence

## Overview

Persistent storage of API configurations using embedded SQLite database with WAL mode for concurrent access, providing durability across Gateway-Controller restarts and migration path to PostgreSQL/MySQL.

## Requirements

### Storage Backend
- SQLite database file at configurable path (default: `./data/gateway.db`)
- WAL (Write-Ahead Logging) journal mode enabling concurrent reads during writes
- Busy timeout of 5000ms for handling locked database scenarios
- NORMAL synchronous mode balancing durability and performance

### Schema Design
- `api_configs` table with columns: id (TEXT PRIMARY KEY), name, version, context, kind, configuration (TEXT/JSON), status, created_at, updated_at, deployed_at, deployed_version
- Composite unique constraint on `(name, version)` preventing duplicate API versions
- Indexes on frequently queried fields: `(name, version)`, `status`, `context`, `kind`

### Configuration Options
- `storage.type` configuration setting supporting values: `sqlite`, `postgres` (future), `memory`
- `storage.sqlite.path` configuration setting for database file location
- Memory-only mode (`storage.type=memory`) disabling persistence for testing

### Operational Behavior
- Automatic database schema initialization if database file does not exist
- All API configurations loaded from SQLite into in-memory cache on startup
- Atomic write operations using SQLite transactions ensuring ACID guarantees
- Graceful shutdown performing WAL checkpoint to flush pending transactions
- Fail-fast behavior on locked database at startup with clear error message

### Database Migration Support
- Storage interface abstraction isolating database-specific code
- JSON TEXT storage format enabling migration to PostgreSQL/MySQL without data transformation
- Prepared statements preventing SQL injection and improving performance

## Success Criteria

- Gateway-Controller creates SQLite database automatically on first startup if file does not exist
- API configurations survive controller restarts with zero data loss when `storage.type=sqlite`
- Composite unique constraint prevents duplicate `(name, version)` configurations with HTTP 409 response
- p95 latency for CRUD operations under 1 second for databases with up to 100 configurations
- Database file size grows predictably (approximately 5-10 KB per configuration)
- SQLite-specific code isolated behind Storage interface enabling future PostgreSQL implementation

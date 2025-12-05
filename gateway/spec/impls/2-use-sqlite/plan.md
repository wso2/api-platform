# Implementation Plan: Database Migration from BBolt to SQLite

## Summary

Replace the gateway-controller's BBolt embedded database with SQLite for persistent storage of API configurations. This migration enables standard SQL tooling for database management, better ecosystem support, and future extensibility to external databases like PostgreSQL. The implementation will maintain the existing Storage interface contract, remove all audit logging features completely, and support both persistent (SQLite) and memory-only storage modes. No data migration or backward compatibility is required.

## Technical Context

**Language/Version**: Go 1.25.1+
**Primary Dependencies**:
  - `github.com/mattn/go-sqlite3` (CGO-based SQLite driver - requires gcc at build time)
  - `github.com/envoyproxy/go-control-plane` (xDS protocol implementation)
  - `github.com/knadh/koanf/v2` (Configuration management)
  - `go.uber.org/zap` (Structured logging)
  - `github.com/gin-gonic/gin` (REST API framework)
  - **REMOVE**: `go.etcd.io/bbolt` (will be removed as part of this migration)

**Storage**: SQLite 3.x (embedded database via mattn/go-sqlite3 driver), with fallback to memory-only mode for testing
**Database Path**: Default `./data/gateway.db`, configurable via `storage.sqlite.path` config option
**Configuration Format**:
```yaml
storage:
  type: sqlite      # "sqlite", "postgres" (future), or "memory"
  sqlite:
    path: ./data/gateway.db
```
**Testing**: Go's built-in testing framework (`go test`), integration tests in `tests/` directory
**Target Platform**: Linux server (primary), macOS (development), Docker containers
**Project Type**: Single Go project (gateway/gateway-controller)
**Performance Goals**:
  - API configuration operations (create/read/update/delete) complete in <1 second for databases with up to 100 configurations
  - Support 10 concurrent write operations without errors or data corruption
  - SQLite database file size growth: approximately 5-10 KB per API configuration

**Constraints**:
  - Must maintain existing Storage interface contract (no breaking changes to dependent code)
  - SQLite operations must use transactions for consistency
  - Thread-safe operations required (concurrent REST API requests)
  - Must support both persistent (SQLite) and memory-only storage modes
  - No backward compatibility or data migration from BBolt required
  - No audit logging feature (complete removal required)

**Scale/Scope**:
  - Support 100+ API configurations in a single gateway instance
  - Single gateway-controller instance (no multi-instance coordination required for SQLite)
  - API configuration payloads up to 1MB each
  - Focus on single-instance deployments (future: external databases for multi-instance scenarios)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Status**: PASS (No constitution defined - using project principles from CLAUDE.md)

Based on the project principles in CLAUDE.md:

✅ **Component Independence**: This migration is isolated to the gateway-controller component. The Storage interface abstraction ensures no breaking changes to dependent code (API handlers, xDS server).

✅ **Size matters**: Replacing BBolt with SQLite maintains the "small and lightweight" principle. Both are embedded databases with no external dependencies. Removing audit logging further reduces code complexity.

✅ **Developer experience is king**: SQLite provides better DX through standard SQL tooling (sqlite3 CLI, DB Browser for SQLite) compared to BBolt's key-value approach. Configuration inspection and debugging become significantly easier.

✅ **Policy-first gateway**: This migration does not affect the policy architecture or routing logic. It only changes the persistence layer.

✅ **Separation of concerns**: The Storage interface maintains separation between business logic (API handlers) and persistence layer (database implementation).

## Project Structure

### Documentation (this feature)

```
specs/005-migrate-sqlite-db/
├── spec.md              # Feature specification (input)
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (SQLite best practices, driver comparison)
├── data-model.md        # Phase 1 output (database schema, table definitions)
├── quickstart.md        # Phase 1 output (migration guide for operators)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

This is a **single project** feature affecting the gateway-controller component only:

```
gateway/gateway-controller/
├── cmd/
│   └── controller/
│       └── main.go              # Entry point (minor changes for storage initialization)
├── pkg/
│   ├── api/
│   │   └── handlers/
│   │       └── api_handler.go   # REST API handlers (remove audit endpoints)
│   ├── config/
│   │   ├── config.go            # Configuration loader (update storage config)
│   │   └── validator.go         # Configuration validation (remove BBolt settings)
│   ├── models/
│   │   └── stored_config.go        # Data structures (unchanged)
│   ├── storage/
│   │   ├── interface.go         # Storage abstraction (remove AuditLogger interface)
│   │   ├── memory.go            # In-memory cache (unchanged)
│   │   ├── bbolt.go             # [DELETE] BBolt implementation
│   │   └── sqlite.go            # [NEW] SQLite implementation
│   ├── xds/
│   │   └── ...                  # xDS server (unchanged)
│   └── logger/
│       └── logger.go            # Logging setup (unchanged)
├── config/
│   ├── config.yaml              # [UPDATE] Replace BBolt settings with SQLite settings
│   └── config-memory-only.yaml  # [UPDATE] Memory-only example
├── api/
│   └── openapi.yaml             # [UPDATE] Remove audit endpoints from REST API spec
├── tests/
│   ├── integration/
│   │   └── storage_test.go      # [UPDATE] Test SQLite implementation
│   └── ...
├── go.mod                       # [UPDATE] Add mattn/go-sqlite3, remove bbolt
├── go.sum                       # [AUTO-GENERATED] Updated checksums
├── Makefile                     # [REVIEW] Check for bbolt-specific commands
└── README.md                    # [UPDATE] Update documentation with SQLite details
```

**Structure Decision**: Single Go project structure. All changes are isolated to the `gateway/gateway-controller` directory. The key modifications are:
1. **Replace**: `pkg/storage/bbolt.go` → `pkg/storage/sqlite.go`
2. **Delete**: All audit logging code (interface, implementation, API handlers)
3. **Update**: Configuration files, documentation, OpenAPI spec, go.mod
4. **Preserve**: Storage interface, in-memory cache, business logic, xDS server

## Complexity Tracking

*Fill ONLY if Constitution Check has violations that must be justified*

**No violations detected.** This migration simplifies the codebase by:
- Removing audit logging feature entirely (reduces code complexity)
- Maintaining the existing Storage interface abstraction (no new patterns introduced)
- Using standard SQLite patterns (well-understood, minimal cognitive overhead)


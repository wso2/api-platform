# Tasks: Database Migration from BBolt to SQLite

**Input**: Design documents from `/specs/005-migrate-sqlite-db/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md
**Feature Branch**: `005-migrate-sqlite-db`

**Tests**: Tests are NOT explicitly requested in the specification. Test tasks are included only for critical integration scenarios. Unit tests will be updated as part of implementation tasks.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story?] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions
- Single Go project: `gateway/gateway-controller/`
- Paths are absolute from repository root

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization, dependency management, and cleanup preparation

- [X] T001 Update go.mod to add github.com/mattn/go-sqlite3 dependency in gateway/gateway-controller/go.mod
- [X] T002 [P] Create SQLite schema SQL file in gateway/gateway-controller/pkg/storage/schema.sql
- [X] T003 [P] Create test data directory structure for SQLite testing in gateway/gateway-controller/tests/testdata/

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core storage abstraction that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Remove AuditLogger interface and related types from gateway/gateway-controller/pkg/storage/interface.go
- [X] T005 Create SQLiteStorage struct and constructor in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T006 Implement initSchema method with embedded SQL in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T007 Implement database connection management with WAL mode and connection pooling in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T008 Add SQLite-specific error handling types and helpers in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T009 Update configuration structs to add storage.type field and nested storage.sqlite.path, remove database_path and audit settings in gateway/gateway-controller/pkg/config/config.go
- [X] T010 Update configuration validator to remove BBolt validation and add storage type validation (sqlite/postgres/memory) in gateway/gateway-controller/pkg/config/validator.go

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Gateway Persistence with SQLite (Priority: P1) 🎯 MVP

**Goal**: Replace BBolt with SQLite for persistent storage, enabling operators to manage API configurations with standard SQL tooling and ensuring zero data loss across restarts.

**Independent Test**: Create API configurations via REST API, restart gateway-controller, and verify all configurations are restored from SQLite database.

### Implementation for User Story 1

- [X] T011 [P] [US1] Implement SaveConfig method with transaction support in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T012 [P] [US1] Implement UpdateConfig method with existence check in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T013 [P] [US1] Implement DeleteConfig method in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T014 [P] [US1] Implement GetConfig method by ID in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T015 [P] [US1] Implement GetConfigByNameVersion method with indexed query in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T016 [P] [US1] Implement GetAllConfigs method in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T017 [P] [US1] Implement Close method for database cleanup in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T018 [US1] Implement LoadFromDatabase function to populate ConfigStore on startup in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T019 [US1] Update main.go to initialize SQLiteStorage instead of BBoltStorage in gateway/gateway-controller/cmd/controller/main.go
- [X] T020 [US1] Add database file lock detection and fail-fast error handling on startup in gateway/gateway-controller/cmd/controller/main.go
- [X] T021 [US1] Update integration tests to use SQLite instead of BBolt in gateway/gateway-controller/tests/integration/storage_test.go
- [X] T022 [US1] Add test for database persistence across restarts in gateway/gateway-controller/tests/integration/persistence_test.go
- [X] T023 [US1] Add test for concurrent write operations (10 goroutines) in gateway/gateway-controller/tests/integration/concurrency_test.go
- [X] T024 [US1] Verify SQLite database file creation and schema initialization in gateway/gateway-controller/tests/integration/schema_test.go

**Checkpoint**: At this point, User Story 1 should be fully functional - API configurations persist in SQLite and survive restarts

---

## Phase 4: User Story 2 - Simplified Testing with Memory-Only Mode (Priority: P2)

**Goal**: Enable developers to test gateway-controller without persistent storage, allowing fast development iterations and CI/CD pipelines without file system dependencies.

**Independent Test**: Start gateway-controller with storage.type: memory, submit API configurations, verify they work during the session but are lost after restart.

### Implementation for User Story 2

- [X] T025 [US2] Update storage initialization logic to support memory type (storage.type: memory) in gateway/gateway-controller/cmd/controller/main.go
- [X] T026 [US2] Add conditional storage creation based on storage.type config (sqlite/postgres/memory) in gateway/gateway-controller/cmd/controller/main.go
- [X] T027 [US2] Update config-memory-only.yaml example to use storage.type: memory in gateway/gateway-controller/config/config-memory-only.yaml
- [X] T028 [US2] Add test for memory-only mode with configuration loss on restart in gateway/gateway-controller/tests/integration/memory_mode_test.go
- [X] T029 [US2] Verify CRUD operations work identically in both modes in gateway/gateway-controller/tests/integration/storage_parity_test.go

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently - persistent and memory-only modes both functional

---

## Phase 5: User Story 3 - Future Database Engine Flexibility (Priority: P3)

**Goal**: Ensure storage layer abstraction supports future migration to PostgreSQL or MySQL without requiring major code changes.

**Independent Test**: Review codebase to confirm all database operations go through Storage interface with no SQLite-specific code in business logic.

### Implementation for User Story 3

- [X] T030 [P] [US3] Review and ensure Storage interface has no SQLite-specific methods in gateway/gateway-controller/pkg/storage/interface.go
- [X] T031 [P] [US3] Verify API handlers use Storage interface exclusively in gateway/gateway-controller/pkg/api/handlers/api_handler.go
- [X] T032 [P] [US3] Verify xDS server uses Storage interface exclusively in gateway/gateway-controller/pkg/xds/server.go
- [X] T033 [US3] Add documentation comments explaining abstraction strategy in gateway/gateway-controller/pkg/storage/interface.go
- [X] T034 [US3] Create future migration guide in gateway/gateway-controller/docs/postgresql-migration.md

**Checkpoint**: All user stories should now be independently functional - storage layer is fully abstracted

---

## Phase 6: Cleanup & Removal (BBolt and Audit Logging)

**Purpose**: Remove all BBolt and audit logging code, dependencies, and configurations

- [X] T035 Delete bbolt.go implementation file from gateway/gateway-controller/pkg/storage/bbolt.go
- [X] T036 [P] Remove all audit logging endpoints from OpenAPI spec in gateway/gateway-controller/api/openapi.yaml
- [X] T037 [P] Remove audit logging handler methods from API handlers in gateway/gateway-controller/pkg/api/handlers/api_handler.go
- [X] T038 [P] Remove BBolt dependency from go.mod in gateway/gateway-controller/go.mod
- [X] T039 Run go mod tidy to clean up dependencies in gateway/gateway-controller/
- [X] T040 [P] Update config.yaml to use storage.type and storage.sqlite.path structure, remove database_path and audit settings in gateway/gateway-controller/config/config.yaml
- [X] T041 [P] Remove any BBolt-specific Makefile targets in gateway/gateway-controller/Makefile
- [X] T042 Search codebase for remaining BBolt references and remove in gateway/gateway-controller/
- [X] T043 [P] Review and update gateway deployment configs to remove BBolt references in gateway/gateway-controller/gateway/

---

## Phase 7: Documentation & Configuration Updates

**Purpose**: Update all documentation and configuration files to reflect SQLite migration

- [X] T044 [P] Update README.md to replace BBolt references with SQLite in gateway/gateway-controller/README.md
- [X] T045 [P] Update README.md Data Storage section with SQLite details in gateway/gateway-controller/README.md
- [X] T046 [P] Update README.md to remove audit logging documentation in gateway/gateway-controller/README.md
- [X] T047 [P] Update configuration examples with SQLite settings in gateway/gateway-controller/README.md
- [X] T048 [P] Add troubleshooting section for SQLite locked database errors in gateway/gateway-controller/README.md
- [X] T049 [P] Update architecture documentation if exists in gateway/gateway-controller/docs/
- [X] T050 [P] Add database inspection guide using sqlite3 CLI in gateway/gateway-controller/docs/
- [X] T051 Update CHANGELOG.md or release notes with breaking changes in gateway/gateway-controller/CHANGELOG.md

---

## Phase 8: Testing & Validation

**Purpose**: Comprehensive testing to ensure all success criteria are met

- [X] T052 Run make test to verify all unit tests pass in gateway/gateway-controller/
- [X] T053 Run integration tests with empty database initialization in gateway/gateway-controller/tests/integration/
- [X] T054 Test corrupted database file handling (verify fail-fast behavior) in gateway/gateway-controller/tests/integration/
- [X] T055 Test locked database file on startup (verify clear error message) in gateway/gateway-controller/tests/integration/
- [X] T056 Performance test: Verify CRUD operations complete with p95 latency under 1 second for 100 configs in gateway/gateway-controller/tests/performance/
- [X] T057 Load test: Create 100 API configurations and verify database size growth in gateway/gateway-controller/tests/performance/
- [X] T058 Concurrency test: Run 10 concurrent write operations and verify no errors in gateway/gateway-controller/tests/integration/
- [X] T059 Verify WAL mode is enabled by checking database files (.db, .db-wal, .db-shm) in gateway/gateway-controller/tests/integration/
- [X] T060 Test memory-only mode does not create database files in gateway/gateway-controller/tests/integration/
- [X] T061 Test configuration survival across restarts in persistent mode in gateway/gateway-controller/tests/integration/

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Final improvements, security hardening, and quickstart validation

- [X] T062 [P] Code review and refactoring for SQLite implementation in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T063 [P] Add structured logging for database operations in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T064 [P] Optimize SQL queries if needed (verify EXPLAIN QUERY PLAN) in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T065 [P] Security review: Ensure no SQL injection vulnerabilities in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T066 [P] Add database metrics collection (file size, query count) in gateway/gateway-controller/pkg/storage/sqlite.go
- [X] T067 Validate quickstart.md instructions with fresh deployment in specs/005-migrate-sqlite-db/quickstart.md
- [X] T068 [P] Update Docker image build to ensure gcc is available for CGO in gateway/gateway-controller/Dockerfile
- [X] T069 Run make docker to verify Docker image builds successfully in gateway/gateway-controller/
- [X] T070 [P] Verify Docker build includes gcc/build-essential and successfully compiles mattn/go-sqlite3 in gateway/gateway-controller/

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phases 3-5)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Cleanup (Phase 6)**: Depends on User Story 1 completion (SQLite implementation working)
- **Documentation (Phase 7)**: Can start after User Story 1, complete before final testing
- **Testing (Phase 8)**: Depends on all user stories + cleanup + documentation
- **Polish (Phase 9)**: Depends on all testing passing

### User Story Dependencies

- **User Story 1 (P1)**: Depends on Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Depends on Foundational (Phase 2) - Independent from US1 but builds on same infrastructure
- **User Story 3 (P3)**: Depends on Foundational (Phase 2) - Architectural review, independent from US1/US2

### Within Each User Story

- **US1**:
  - T011-T017 (CRUD methods) can run in parallel
  - T018 (LoadFromDatabase) depends on T011-T017
  - T019-T020 (main.go updates) depend on T018
  - T021-T024 (tests) can run in parallel after implementation complete

- **US2**:
  - T025-T027 (config updates) can run in parallel
  - T028-T029 (tests) depend on T025-T027

- **US3**:
  - T030-T032 (interface review) can run in parallel
  - T033-T034 (documentation) can run in parallel with T030-T032

### Parallel Opportunities

#### Phase 1 (Setup)
```bash
# All can run in parallel:
Task T002: Create schema.sql
Task T003: Create test data directories
# T001 should complete first (go.mod) before running go get
```

#### Phase 2 (Foundational)
```bash
# After T004 (interface cleanup), these can run in parallel:
Task T005: Create SQLiteStorage struct
Task T009: Update config structs
Task T010: Update config validator
# Then T006-T008 depend on T005 completing
```

#### Phase 3 (User Story 1)
```bash
# All CRUD methods can be implemented in parallel:
Task T011: Implement SaveConfig
Task T012: Implement UpdateConfig
Task T013: Implement DeleteConfig
Task T014: Implement GetConfig
Task T015: Implement GetConfigByNameVersion
Task T016: Implement GetAllConfigs
Task T017: Implement Close

# All tests can run in parallel after implementation:
Task T021: Integration tests
Task T022: Persistence test
Task T023: Concurrency test
Task T024: Schema test
```

#### Phase 6 (Cleanup)
```bash
# These can run in parallel:
Task T036: Remove audit endpoints from OpenAPI
Task T037: Remove audit handlers
Task T038: Remove BBolt from go.mod
Task T040: Update config.yaml
Task T041: Update Makefile
```

#### Phase 7 (Documentation)
```bash
# All documentation updates can run in parallel:
Task T043: Update README BBolt references
Task T044: Update Data Storage section
Task T045: Remove audit logging docs
Task T046: Update config examples
Task T047: Add troubleshooting section
Task T048: Update architecture docs
Task T049: Add inspection guide
```

---

## Parallel Example: User Story 1 CRUD Implementation

```bash
# Launch all CRUD methods together (different functions, no conflicts):
Task: "Implement SaveConfig method in storage/sqlite.go"
Task: "Implement UpdateConfig method in storage/sqlite.go"
Task: "Implement DeleteConfig method in storage/sqlite.go"
Task: "Implement GetConfig method in storage/sqlite.go"
Task: "Implement GetConfigByNameVersion method in storage/sqlite.go"
Task: "Implement GetAllConfigs method in storage/sqlite.go"
Task: "Implement Close method in storage/sqlite.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. **Complete Phase 1: Setup** (T001-T003) - ~30 minutes
2. **Complete Phase 2: Foundational** (T004-T010) - CRITICAL, ~2 hours
3. **Complete Phase 3: User Story 1** (T011-T024) - ~4 hours
4. **STOP and VALIDATE**: Test User Story 1 independently with quickstart.md scenarios
5. **Complete Phase 6: Cleanup** (T035-T042) - ~1 hour
6. **Complete Phase 7: Documentation** (T043-T050) - ~1 hour
7. **Complete Phase 8: Testing** (T051-T060) - ~2 hours
8. Deploy/demo if ready

**Total MVP Time**: ~11 hours (User Story 1 + Cleanup + Docs + Testing)

### Incremental Delivery

1. **Foundation** (Phases 1-2) → Setup + Foundational ready (~2.5 hours)
2. **MVP** (Phase 3 + Phases 6-8) → SQLite persistence working (~8 hours) - **DEPLOY**
3. **Memory Mode** (Phase 4) → Memory-only mode added (~1 hour) - **DEPLOY**
4. **Architecture Review** (Phase 5) → Future-proof abstraction validated (~1 hour) - **DEPLOY**
5. **Polish** (Phase 9) → Production-ready (~2 hours) - **FINAL RELEASE**

**Total Time**: ~14.5 hours for complete feature

### Parallel Team Strategy

With 3 developers after Foundational phase complete:

1. **Team completes Setup + Foundational together** (Phases 1-2)
2. **Once Foundational is done**:
   - **Developer A**: User Story 1 (Phase 3) - Core SQLite implementation
   - **Developer B**: Cleanup (Phase 6) - BBolt removal (can start after US1 basics)
   - **Developer C**: Documentation (Phase 7) - Can start drafting in parallel
3. **Team converges**:
   - All developers run Testing (Phase 8) together
   - All developers contribute to Polish (Phase 9)

---

## Success Criteria Validation

### SC-001: Auto-create database
- **Tasks**: T006 (initSchema), T019 (startup), T024 (schema test)
- **Validation**: Test with empty data directory

### SC-002: p95 latency < 1 second
- **Tasks**: T056 (performance test)
- **Validation**: Benchmark all CRUD operations with p95 latency measurement

### SC-003: Zero data loss on restart
- **Tasks**: T022 (persistence test)
- **Validation**: Create configs, restart, verify all restored

### SC-004: Memory-only mode works
- **Tasks**: T025-T029 (User Story 2)
- **Validation**: No database files created

### SC-005: Predictable database growth
- **Tasks**: T057 (load test)
- **Validation**: Verify 5-10 KB per config

### SC-006: No BBolt code remains
- **Tasks**: T035-T043 (Cleanup phase)
- **Validation**: grep -r "bbolt" should find nothing

### SC-007: No audit logging remains
- **Tasks**: T004, T036, T037, T046
- **Validation**: grep -r "audit" should find nothing

### SC-008: Tests pass
- **Tasks**: T052, T021
- **Validation**: make test passes

### SC-009: 10 concurrent writes work
- **Tasks**: T023 (concurrency test), T058 (load test)
- **Validation**: No errors or data corruption

### SC-010: Documentation accurate
- **Tasks**: T044-T051 (Documentation phase), T067 (quickstart validation)
- **Validation**: Fresh deployment using README.md instructions

---

## Notes

- **Total Tasks**: 70 tasks across 9 phases
- **[P] tasks**: Different files, no dependencies - can run in parallel
- **[Story] label**: Maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- **CGO Requirement**: mattn/go-sqlite3 requires gcc at build time - ensure Docker image includes build-essential
- **Database Path**: Default `./data/gateway.db` configured via storage.sqlite.path
- **Configuration Structure**: Hierarchical YAML with storage.type (sqlite/postgres/memory) and nested database-specific settings (storage.sqlite.*, storage.postgres.*)
- **WAL Mode**: Enabled automatically via connection string parameters
- **Backward Compatibility**: NOT REQUIRED - this is a clean-slate migration
- **Data Migration**: NOT REQUIRED - operators will recreate configurations
- **Testing Philosophy**: Integration tests focus on SQLite-specific behaviors (transactions, concurrency, persistence)

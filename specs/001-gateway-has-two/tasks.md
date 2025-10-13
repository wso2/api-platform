# Tasks: Gateway with Controller and Router

**Input**: Design documents from `/specs/001-gateway-has-two/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/gateway-controller-api.yaml
**Tech Stack**: Go 1.25.1, Envoy Proxy 1.35.3, bbolt, go-control-plane, oapi-codegen, Gin, Zap
**Tests**: No explicit TDD requirement - tests are OPTIONAL based on spec

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Exact file paths included in task descriptions

## Path Conventions
- Gateway-Controller: `gateway/gateway-controller/`
- Router: `gateway/router/`
- Root level: `gateway/` for docker compose and shared docs

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create directory structure: `gateway/gateway-controller/{cmd/controller,pkg/{api/{generated,handlers,middleware},config,models,storage,xds,logger},tests/{unit,integration},api}` and `gateway/router/config`
- [ ] T002 Initialize Go module in `gateway/gateway-controller/go.mod` with dependencies: gin, oapi-codegen/v2, zap, go-control-plane v0.13.0+, bbolt v1.3.9+, validator/v10, yaml.v3
- [ ] T003 [P] Create Makefile in `gateway/gateway-controller/Makefile` with targets: generate, build, test, docker, clean, run
- [ ] T004 [P] Create Makefile in `gateway/router/Makefile` with targets: docker, clean
- [ ] T005 [P] Create root-level `gateway/README.md` with overall gateway documentation
- [ ] T006 [P] Create `gateway/docker compose.yaml` for complete stack (gateway-controller, router, sample backend)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T007 Create OpenAPI specification in `gateway/gateway-controller/api/openapi.yaml` based on contracts/gateway-controller-api.yaml
- [ ] T008 Create oapi-codegen configuration in `gateway/gateway-controller/oapi-codegen.yaml` for Gin server generation
- [ ] T009 Generate API server code using oapi-codegen: `gateway/gateway-controller/pkg/api/generated.go` (ServerInterface, types, RegisterHandlers)
- [ ] T010 Implement logger setup in `gateway/gateway-controller/pkg/logger/logger.go` with Zap, configurable log levels (LOG_LEVEL env var), structured JSON output
- [ ] T011 [P] Define API configuration models in `gateway/gateway-controller/pkg/models/api_config.go` (complement generated types with composite key helpers)
- [ ] T012 [P] Define storage interfaces in `gateway/gateway-controller/pkg/storage/interface.go` (Storage abstraction for CRUD operations)
- [ ] T013 Implement in-memory ConfigStore in `gateway/gateway-controller/pkg/storage/memory.go` (maps for configs and name:version index, RWMutex, snapshot version tracking)
- [ ] T014 Implement bbolt storage in `gateway/gateway-controller/pkg/storage/bbolt.go` with buckets: apis/, audit/, metadata/ and CRUD operations
- [ ] T015 Implement configuration parser in `gateway/gateway-controller/pkg/config/parser.go` for YAML/JSON parsing with yaml.v3 and encoding/json
- [ ] T016 Implement validator in `gateway/gateway-controller/pkg/config/validator.go` using go-playground/validator/v10 with custom validation rules and structured error reporting
- [ ] T017 Implement xDS snapshot manager in `gateway/gateway-controller/pkg/xds/snapshot.go` using go-control-plane SotW cache
- [ ] T018 Implement xDS translator in `gateway/gateway-controller/pkg/xds/translator.go` (in-memory maps → Envoy Listener, Route, Cluster resources)
- [ ] T019 Implement xDS SotW server in `gateway/gateway-controller/pkg/xds/server.go` using go-control-plane/pkg/server/v3 with gRPC listener on port 18000
- [ ] T020 Create Envoy bootstrap configuration in `gateway/router/config/envoy-bootstrap.yaml` with xds_cluster, initial_fetch_timeout: 0s, exponential backoff retry policy
- [ ] T021 Create Router Dockerfile in `gateway/router/Dockerfile` based on envoyproxy/envoy:v1.35.3 with bootstrap config

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Deploy New API Configuration (Priority: P1) 🎯 MVP

**Goal**: Enable API administrators to expose a new REST API through the gateway by submitting a configuration that defines routing rules, endpoints, and policies. Gateway-Controller validates, persists, and configures Router to start accepting traffic.

**Independent Test**: Submit a single API configuration YAML to Gateway-Controller via POST /apis, then send HTTP request to Router matching the configured route and verify it routes to backend service.

### Implementation for User Story 1

- [ ] T022 [P] [US1] Implement HealthCheck handler in `gateway/gateway-controller/pkg/api/handlers/handlers.go` (ServerInterface.HealthCheck method)
- [ ] T023 [P] [US1] Implement CreateAPI handler in `gateway/gateway-controller/pkg/api/handlers/handlers.go` (parse YAML/JSON, validate, atomic dual-write to DB + in-memory, async xDS update, return 201 with ID)
- [ ] T024 [US1] Implement audit logging in `gateway/gateway-controller/pkg/storage/audit.go` (AuditEvent structure, write to bbolt audit/ bucket)
- [ ] T025 [US1] Create main entry point in `gateway/gateway-controller/cmd/controller/main.go` (startup flow: init DB, load configs to memory, generate initial xDS snapshot, start xDS server, start REST API on port 9090)
- [ ] T026 [US1] Implement startup config loader in `gateway/gateway-controller/pkg/storage/loader.go` (LoadFromDatabase method to populate in-memory ConfigStore from bbolt on startup)
- [ ] T027 [US1] Create Gateway-Controller Dockerfile in `gateway/gateway-controller/Dockerfile` (multi-stage build: golang:1.25.1 → alpine, <100MB target)
- [ ] T028 [US1] Add logging middleware in `gateway/gateway-controller/pkg/api/middleware/logging.go` (Gin middleware for request/response logging with Zap)
- [ ] T029 [US1] Add error handling middleware in `gateway/gateway-controller/pkg/api/middleware/errors.go` (Gin recovery middleware, structured error responses)
- [ ] T030 [US1] Implement path rewriting logic in xDS translator for upstream URLs with path prefixes (e.g., https://api.weather.com/api/v2 → prepend /api/v2)

**Validation**: Create sample weather-api.yaml, submit via curl POST to localhost:9090/apis, verify Router routes GET requests to backend within 5 seconds. Verify invalid configs return structured JSON errors with field paths.

**Checkpoint**: At this point, User Story 1 should be fully functional - users can deploy APIs and route traffic through the gateway

---

## Phase 4: User Story 2 - Update Existing API Configuration (Priority: P2)

**Goal**: Enable API administrators to modify existing API configurations (change backend URL, update routing rules, add endpoints) without downtime. Changes reflect in Router immediately.

**Independent Test**: Deploy an API config (US1), then submit updated version with modified routes via PUT /apis/{name}/{version}, verify Router behavior changes to match new config without dropping existing connections.

### Implementation for User Story 2

- [ ] T031 [US2] Implement UpdateAPI handler in `gateway/gateway-controller/pkg/api/handlers/handlers.go` (validate update, check config exists, atomic update to DB + in-memory, async xDS snapshot regeneration, return 200)
- [ ] T032 [US2] Implement ConfigStore.Update method in `gateway/gateway-controller/pkg/storage/memory.go` (update in-memory maps, handle name/version index changes)
- [ ] T033 [US2] Add update validation in `gateway/gateway-controller/pkg/config/validator.go` (ensure updated config still passes all validation rules)
- [ ] T034 [US2] Implement xDS snapshot version increment logic in ConfigStore (monotonically increasing version for SotW protocol)
- [ ] T035 [US2] Add audit logging for UPDATE operations in audit.go (log old vs new configuration diff at debug level)

**Validation**: Deploy weather-api.yaml (US1), update it to add PUT operation, submit via PUT /apis/{name}/{version}, verify new PUT endpoint works while existing GET/POST continue working. Verify in-flight requests complete with old config.

**Checkpoint**: At this point, User Stories 1 AND 2 should both work - users can deploy and update APIs with zero downtime

---

## Phase 5: User Story 3 - Delete API Configuration (Priority: P2)

**Goal**: Enable API administrators to remove APIs from gateway when deprecated or no longer needed. Router immediately stops accepting traffic for deleted API.

**Independent Test**: Deploy an API config (US1), then delete it via DELETE /apis/{name}/{version}, verify Router returns 404 for requests to previously configured routes within 5 seconds.

### Implementation for User Story 3

- [ ] T036 [US3] Implement DeleteAPI handler in `gateway/gateway-controller/pkg/api/handlers/handlers.go` (check config exists, atomic delete from DB + in-memory, async xDS snapshot regeneration, return 200)
- [ ] T037 [US3] Implement ConfigStore.Delete method in `gateway/gateway-controller/pkg/storage/memory.go` (remove from in-memory maps, clean up name:version index)
- [ ] T038 [US3] Add audit logging for DELETE operations in audit.go (log deleted configuration details)
- [ ] T039 [US3] Implement graceful deletion in xDS translator (ensure Router handles configuration removal without crashing, in-flight requests complete)

**Validation**: Deploy weather-api.yaml, delete via DELETE /apis/{name}/{version}, verify Router returns appropriate error (404) for requests to /weather/* routes. Verify in-flight requests complete successfully.

**Checkpoint**: All core CRUD operations working - users can deploy, update, and delete APIs

---

## Phase 6: User Story 4 - List and Query API Configurations (Priority: P3)

**Goal**: Enable API administrators to view all deployed API configurations and their status to understand what APIs are active and verify configurations are correct.

**Independent Test**: Deploy several API configs (US1), query GET /apis for list and GET /apis/{name}/{version} for details, verify returned data matches what was deployed.

### Implementation for User Story 4

- [ ] T040 [P] [US4] Implement ListAPIs handler in `gateway/gateway-controller/pkg/api/handlers/handlers.go` (return all configs with metadata: name, version, context, status, timestamps)
- [ ] T041 [P] [US4] Implement GetAPIByNameVersion handler in `gateway/gateway-controller/pkg/api/handlers/handlers.go` (return complete configuration + metadata for specific name/version, support both JSON and YAML response)
- [ ] T042 [US4] Implement ConfigStore.Get method in `gateway/gateway-controller/pkg/storage/memory.go` (retrieve single config by ID with RLock)
- [ ] T043 [US4] Implement ConfigStore.GetAll method in `gateway/gateway-controller/pkg/storage/memory.go` (return all configs with RLock)

**Validation**: Deploy 3 different API configs, call GET /apis and verify count=3 with correct metadata. Call GET /apis/{name}/{version} for each and verify complete config returned. Call GET /apis with no configs deployed and verify empty list returned.

**Checkpoint**: All user stories complete - full CRUD lifecycle with observability

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple components and ensure production readiness

- [ ] T044 [P] Create sample backend service in `gateway/sample-backend/` for testing (simple Node.js/Python HTTP server responding to weather API paths)
- [ ] T045 [P] Update docker compose.yaml with all three services: gateway-controller, router, sample-backend with correct networking and ports
- [ ] T046 [P] Create Gateway-Controller README in `gateway/gateway-controller/README.md` with build instructions, configuration, API usage examples
- [ ] T047 [P] Create Router README in `gateway/router/README.md` with Envoy configuration details and startup behavior
- [ ] T048 Create example API configurations in `gateway/examples/` (weather-api.yaml, petstore-api.yaml with different patterns)
- [ ] T049 Implement concurrency tests for ConfigStore to verify RWMutex correctness under concurrent create/update/delete operations
- [ ] T050 Add debug logging for xDS snapshots (full Envoy config payloads) when LOG_LEVEL=debug
- [ ] T051 Add configuration diff logging in UpdateAPI handler at debug level (show before/after changes)
- [ ] T052 Verify Docker image sizes: Gateway-Controller <100MB, Router based on Envoy base image
- [ ] T053 Test complete quickstart.md workflow end-to-end (all steps should work as documented)
- [ ] T054 Add composite key uniqueness validation in validator.go (prevent duplicate name/version combinations)
- [ ] T055 Implement context consistency validation (same API name with different versions must have identical context)
- [ ] T056 Add detailed field-level validation error messages for all validation rules (per Decision 8 in research.md)
- [ ] T057 Test Envoy startup behavior when xDS server unavailable (verify wait-indefinitely with exponential backoff)
- [ ] T058 Test configuration update performance target (<1s validation, <5s xDS push)
- [ ] T059 Test scale target: deploy 100+ API configurations and verify no performance degradation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-6)**: All depend on Foundational phase completion
  - User stories can proceed in parallel (if staffed) after Foundational is complete
  - Or sequentially in priority order: US1 (P1) → US2 (P2) → US3 (P2) → US4 (P3)
- **Polish (Phase 7)**: Depends on desired user stories being complete (minimally US1 for MVP)

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories - **DELIVERS MVP**
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Builds on US1 but independently testable
- **User Story 3 (P2)**: Can start after Foundational (Phase 2) - Builds on US1 but independently testable
- **User Story 4 (P3)**: Can start after Foundational (Phase 2) - Completely independent, read-only operations

### Critical Path for MVP

```
T001-T006 (Setup) → T007-T021 (Foundational) → T022-T030 (US1) → T053 (Quickstart validation)
```

### Within Each Phase

- **Setup**: All tasks can run in parallel except T001 (directory structure must exist first)
- **Foundational**:
  - T007-T009 must be sequential (OpenAPI → config → generate)
  - T010-T012 can run in parallel
  - T013-T014 depend on T012 (storage interface)
  - T015-T016 can run in parallel
  - T017-T019 must be sequential (snapshot → translator → server)
  - T020-T021 can run in parallel with any Gateway-Controller tasks
- **User Story 1**:
  - T022-T023 can run in parallel
  - T025 depends on all other US1 tasks
  - T024, T026-T030 can be worked on in parallel with handlers
- **User Story 2-4**: Tasks within each story have minimal dependencies, mostly parallel

### Parallel Opportunities

**After Foundational completes**, maximum parallelization:
- Developer A: User Story 1 (T022-T030)
- Developer B: User Story 2 (T031-T035)
- Developer C: User Story 3 (T036-T039)
- Developer D: User Story 4 (T040-T043)
- Developer E: Polish tasks (T044-T059)

**Within User Story 1 (MVP critical path)**:
```bash
# Parallel group 1:
Task T022: HealthCheck handler
Task T023: CreateAPI handler
Task T024: Audit logging
Task T026: Startup config loader
Task T027: Gateway-Controller Dockerfile
Task T028: Logging middleware
Task T029: Error handling middleware

# Sequential after group 1:
Task T025: Main entry point (needs all handlers and middleware)
Task T030: Path rewriting logic (can be done anytime, parallel with others)
```

---

## Parallel Example: Foundational Phase (Phase 2)

```bash
# Wave 1 - Sequential (OpenAPI and code generation):
Task T007: Create OpenAPI spec
Task T008: Create oapi-codegen config
Task T009: Generate API server code

# Wave 2 - Parallel (after generation complete):
Task T010: Logger setup
Task T011: API config models
Task T012: Storage interfaces

# Wave 3 - Parallel (after interfaces defined):
Task T013: In-memory ConfigStore
Task T014: bbolt storage
Task T015: Config parser
Task T016: Validator

# Wave 4 - Sequential (xDS stack):
Task T017: xDS snapshot manager
Task T018: xDS translator
Task T019: xDS SotW server

# Wave 5 - Parallel (Router, independent):
Task T020: Envoy bootstrap config
Task T021: Router Dockerfile
```

---

## Implementation Strategy

### MVP First (User Story 1 Only) - RECOMMENDED

**Goal**: Ship working gateway with create API + routing capability in minimum time

1. Complete Phase 1: Setup (T001-T006)
2. Complete Phase 2: Foundational (T007-T021) ⚠️ CRITICAL - blocks all stories
3. Complete Phase 3: User Story 1 (T022-T030)
4. Complete minimal polish (T044-T045, T053 quickstart validation)
5. **STOP and VALIDATE**:
   - Deploy sample API config
   - Verify Router routes traffic correctly
   - Test validation error handling
   - Run quickstart.md end-to-end
6. Deploy/demo MVP

**MVP Scope**: 30 tasks (Setup + Foundational + US1 + minimal polish)
**Estimated Effort**: ~3-5 days for single developer

### Incremental Delivery

**Iteration 1 - MVP** (Setup + Foundational + US1):
- Users can deploy new APIs
- Router routes traffic based on configs
- Validation prevents bad configs
- **VALUE**: Basic gateway functionality, immediate utility

**Iteration 2 - Full CRUD** (+ US2 + US3):
- Users can update existing APIs without downtime
- Users can delete deprecated APIs
- **VALUE**: Complete lifecycle management

**Iteration 3 - Observability** (+ US4):
- Users can list all deployed APIs
- Users can query specific API details
- **VALUE**: Operational visibility and troubleshooting

**Iteration 4 - Production Ready** (+ Polish):
- Sample backend for testing
- Complete documentation
- Performance/scale validation
- **VALUE**: Production deployment confidence

### Parallel Team Strategy

With 3 developers after Foundational phase completes:

**Week 1**: All developers complete Setup + Foundational together (T001-T021)

**Week 2**: Parallel user story development
- Developer A: User Story 1 (T022-T030) - MVP priority
- Developer B: User Story 2 (T031-T035) + User Story 3 (T036-T039)
- Developer C: User Story 4 (T040-T043) + Polish (T044-T059)

**Week 3**: Integration, testing, documentation
- All developers: Integration testing, bug fixes, polish tasks
- Validate each user story independently
- Complete quickstart validation (T053)
- Deploy complete system

---

## Task Breakdown by Component

### Gateway-Controller (29 tasks)
- **Foundation**: T007-T019 (13 tasks) - Core infrastructure
- **REST API**: T022-T023, T031, T036, T040-T041 (6 tasks) - Endpoint handlers
- **Storage**: T024, T026, T032-T033, T037-T038, T042-T043 (8 tasks) - Data persistence and in-memory
- **Build/Deploy**: T027, T046 (2 tasks) - Docker and docs

### Router (4 tasks)
- **Configuration**: T020 (1 task) - Envoy bootstrap
- **Build/Deploy**: T021, T047 (2 tasks) - Docker and docs
- **Testing**: T057 (1 task) - Startup resilience

### Integration/System (26 tasks)
- **Setup**: T001-T006 (6 tasks) - Project structure
- **Middleware**: T028-T030, T034-T035, T039 (6 tasks) - Cross-cutting concerns
- **Testing/Validation**: T044-T045, T048-T059 (15 tasks) - End-to-end validation

**Total**: 59 tasks

---

## Success Criteria Mapping

Each task contributes to specific success criteria from spec.md:

- **SC-001** (Deploy API <10s): T022-T030 (US1 implementation)
- **SC-002** (Zero-downtime updates): T031-T035 (US2), T039 (graceful deletion)
- **SC-003** (100% correct routing): T017-T019 (xDS translation), T030 (path rewriting)
- **SC-004** (Clear error messages): T016 (validator), T056 (field-level errors)
- **SC-005** (Handle 100+ APIs): T013-T014 (storage), T059 (scale testing)
- **SC-006** (Updates <5s in 95% cases): T034 (xDS version), T058 (performance testing)
- **SC-007** (Full lifecycle without manual Envoy config): All US1-US3 tasks
- **SC-008** (100% consistency): T013-T014 (atomic dual-write), T049 (concurrency tests)

---

## Notes

- [P] tasks = different files/components, can run concurrently
- [Story] label maps task to user story for traceability and independent testing
- Each user story delivers independently testable value
- Foundational phase (Phase 2) is CRITICAL PATH - blocks all feature work
- MVP = Setup + Foundational + US1 + minimal polish (~30 tasks)
- Full implementation = all 59 tasks
- Code generation (T009) must run before handler implementation
- xDS stack (T017-T019) must be sequential due to dependencies
- In-memory maps are primary runtime data source; database is persistence layer
- Every config change generates complete xDS snapshot (SotW protocol)
- Atomic updates maintain consistency between database and in-memory state

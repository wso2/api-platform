# Tasks: Gateway Event Notification System

**Prerequisites**: plan.md (required), spec.md (required for user stories)

**Tests**: Tests are not explicitly requested in the feature specification, so test tasks are omitted.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions
- **Single project**: `src/internal/`, `tests/` at repository root (platform-api monolith)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and dependency management

- [x] T001 Add `github.com/gorilla/websocket v1.5.3` dependency to go.mod in platform-api root

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T002 [P] Create transport abstraction interface in src/internal/websocket/transport.go
- [x] T003 [P] Create GatewayEvent model in src/internal/model/gateway_event.go
- [x] T004 [P] Create GatewayEventDTO in src/internal/dto/gateway_event.go
- [x] T005 [P] Create DeliveryStats struct in src/internal/websocket/stats.go
- [x] T006 Create Connection wrapper struct with metadata in src/internal/websocket/connection.go (depends on T002)

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Gateway Establishes Persistent Connection (Priority: P1) 🎯 MVP

**Goal**: Enable gateways to establish persistent WebSocket connections to the platform and maintain them reliably with heartbeat mechanisms

**Independent Test**: Start a gateway instance with valid credentials and verify it connects successfully. Platform should acknowledge connection and register gateway in connection registry within 10 seconds.

### Implementation for User Story 1

- [x] T007 [P] [US1] Create Connection Manager with sync.Map registry in src/internal/websocket/manager.go
- [x] T008 [P] [US1] Implement WebSocket transport (gorilla/websocket) in src/internal/websocket/websocket_transport.go
- [x] T009 [US1] Create WebSocket upgrade handler with authentication middleware in src/internal/handler/websocket.go (depends on T007, T008)
- [x] T010 [US1] Implement connection registration logic in Connection Manager (depends on T007)
- [x] T011 [US1] Implement heartbeat/ping-pong mechanism with 20s interval and 30s timeout in src/internal/websocket/manager.go (depends on T010)
- [x] T012 [US1] Implement connection acknowledgment message sending in src/internal/handler/websocket.go (depends on T009)
- [x] T013 [US1] Implement graceful disconnection handling and registry cleanup in src/internal/websocket/manager.go (depends on T010)
- [x] T014 [US1] Implement ungraceful disconnection detection via heartbeat timeout in src/internal/websocket/manager.go (depends on T011)
- [x] T015 [US1] Add authentication validation using gatewayService.VerifyToken(apiKey) in src/internal/handler/websocket.go (depends on T009)
- [x] T016 [US1] Register WebSocket route `/api/internal/v1/ws/gateways/connect` in src/internal/server/server.go (depends on T009)
- [x] T017 [US1] Add connection event logging (INFO: connects, WARN: auth failures, ERROR: disconnections) in src/internal/handler/websocket.go and src/internal/websocket/manager.go (depends on T009, T013)
- [x] T018 [US1] Implement rate limiting for connection attempts (10/minute/IP) in src/internal/handler/websocket.go (depends on T009)
- [x] T019 [US1] Implement maximum concurrent connection limit enforcement (default 1000) in src/internal/websocket/manager.go (depends on T010)
- [x] T020 [US1] Add environment variable configuration support for WS_MAX_CONNECTIONS, WS_CONNECTION_TIMEOUT, WS_RATE_LIMIT_PER_MINUTE in src/internal/server/server.go (depends on T016)

**Checkpoint**: At this point, User Story 1 should be fully functional - gateways can connect, connections are maintained with heartbeats, authentication works, and connections are tracked in the registry

---

## Phase 4: User Story 2 - Platform Sends API Deployment Notification (Priority: P1)

**Goal**: Enable the platform to send API deployment events to target gateways via established WebSocket connections when API deployment occurs

**Independent Test**: With a gateway connected, trigger an API deployment action (`POST /api/v1/apis/:api_uuid/deploy-revision`). Gateway should receive deployment event within 5 seconds with API UUID, revision details, and vhost (no API YAML).

### Implementation for User Story 2

- [x] T021 [US2] Create GatewayEventsService with event broadcasting logic in src/internal/service/gateway_events.go
- [x] T022 [US2] Implement BroadcastDeploymentEvent method to lookup gateway connections and send events in src/internal/service/gateway_events.go (depends on T021)
- [x] T023 [US2] Implement event serialization to JSON with type, payload, timestamp, correlationId in src/internal/service/gateway_events.go (depends on T021)
- [x] T024 [US2] Implement event ordering guarantee per gateway (sequential delivery) in src/internal/service/gateway_events.go (depends on T022)
- [x] T025 [US2] Add maximum event payload size validation (default 1MB) in src/internal/service/gateway_events.go (depends on T022)
- [x] T026 [US2] Implement atomic counter increments for delivery statistics (TotalEventsSent) in src/internal/service/gateway_events.go (depends on T022)
- [x] T027 [US2] Implement event delivery failure logging with gateway ID, event type, timestamp in src/internal/service/gateway_events.go (depends on T022)
- [x] T028 [US2] Update atomic counter for FailedDeliveries and track LastFailureTime/Reason in src/internal/service/gateway_events.go (depends on T026, T027)
- [x] T029 [US2] Integrate GatewayEventsService into API deployment service (DeployAPIRevision method) in src/internal/service/api.go (depends on T022)
- [x] T030 [US2] Support multiple connections per gateway ID (broadcast to all instances) in src/internal/service/gateway_events.go (depends on T022)

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently - gateways can connect AND receive deployment events when APIs are deployed

---

## Phase 5: User Story 3 - Platform Tracks Connected Gateways (Priority: P1)

**Goal**: Provide administrative visibility into currently connected gateways via a stats API endpoint

**Independent Test**: Query `GET /api/internal/v1/stats` endpoint and verify it returns accurate real-time connection status with gateway identifiers, connection timestamps, heartbeat times, and delivery statistics.

### Implementation for User Story 3

- [ ] T031 [P] [US3] Create stats response DTOs in src/internal/dto/stats.go
- [ ] T032 [US3] Create StatsHandler with GetStats method in src/internal/handler/stats.go (depends on T031)
- [ ] T033 [US3] Implement GetConnections method in Connection Manager to retrieve all active connections in src/internal/websocket/manager.go
- [ ] T034 [US3] Implement stats aggregation logic (totalActive, connection details, delivery stats) in src/internal/handler/stats.go (depends on T032, T033)
- [ ] T035 [US3] Register stats route `GET /api/internal/v1/stats` in src/internal/server/server.go (depends on T032)
- [ ] T036 [US3] Add stats endpoint to OpenAPI specification in src/resources/openapi.yaml with request/response schemas (depends on T031)

**Checkpoint**: All P1 user stories should now be independently functional - gateways can connect, receive deployment events, and administrators can view connection status

---

## Phase 6: User Story 4 - Multiple Event Types Support (Priority: P2)

**Goal**: Enable the system to support multiple event types beyond API deployment (e.g., api.undeployed, gateway.config.updated) through a pluggable event framework

**Independent Test**: Define a new event type (e.g., "gateway.config.updated"), send it to a connected gateway, and verify the gateway receives the event with correct type identifier and payload structure.

### Implementation for User Story 4

- [ ] T037 [US4] Create event type registry pattern in src/internal/service/gateway_events.go
- [ ] T038 [US4] Define event type constants (api.deployed, api.undeployed, gateway.config.updated) in src/internal/model/gateway_event.go (depends on T037)
- [ ] T039 [US4] Create payload DTOs for different event types in src/internal/dto/gateway_event.go (depends on T038)
- [ ] T040 [US4] Implement BroadcastEvent method accepting event type and payload in src/internal/service/gateway_events.go (depends on T037, T038)
- [ ] T041 [US4] Refactor BroadcastDeploymentEvent to use generic BroadcastEvent in src/internal/service/gateway_events.go (depends on T040)
- [ ] T042 [US4] Add event type validation and structured payload handling in src/internal/service/gateway_events.go (depends on T040)

**Checkpoint**: System now supports multiple event types with a pluggable registration pattern

---

## Phase 7: User Story 5 - Failed Event Delivery Handling (Priority: P2)

**Goal**: Provide comprehensive logging and visibility for failed event deliveries to enable operator troubleshooting and corrective action

**Independent Test**: Disconnect a gateway, trigger an API deployment targeting that gateway, then query the stats endpoint to verify the delivery failure is logged with complete details.

### Implementation for User Story 5

- [ ] T043 [US5] Enhance delivery failure logging with full error context in src/internal/service/gateway_events.go
- [ ] T044 [US5] Add failed delivery details to stats API response (lastFailureTime, lastFailureReason, failedDeliveries count) in src/internal/handler/stats.go (depends on T043)
- [ ] T045 [US5] Implement structured error responses for delivery failures in src/internal/service/gateway_events.go (depends on T043)
- [ ] T046 [US5] Document clean slate reconnection behavior (no automatic event replay) in implementation notes (depends on T043)

**Checkpoint**: Failed delivery handling and logging are complete with full operator visibility

---

## Phase 8: User Story 6 - Gateway Authentication and Authorization (Priority: P2)

**Goal**: Ensure only authenticated gateways can establish connections and receive events through credential validation and enforcement

**Independent Test**: Attempt connections with valid credentials (should succeed), invalid credentials (should be rejected with authentication error), and no credentials (should be rejected). Verify credential revocation immediately terminates active connections.

### Implementation for User Story 6

- [ ] T047 [US6] Enhance authentication error messages with specific error codes in src/internal/handler/websocket.go
- [ ] T048 [US6] Add authentication failure logging (WARN level) with gateway identifier and reason in src/internal/handler/websocket.go (depends on T047)
- [ ] T049 [US6] Implement connection termination on authentication failure in src/internal/handler/websocket.go (depends on T047)
- [ ] T050 [US6] Add security documentation for credential management and rotation in implementation notes (depends on T047)

**Checkpoint**: Authentication and authorization are fully enforced with comprehensive security logging

---

## Phase 9: User Story 7 - Transport Abstraction (Priority: P3)

**Goal**: Ensure business logic (event routing, connection tracking) is fully isolated from transport-specific code through well-defined interfaces

**Independent Test**: Verify that service layer code (gateway_events.go) contains no direct references to WebSocket-specific APIs. Create a mock transport implementation for testing without actual network connections.

### Implementation for User Story 7

- [ ] T051 [US7] Audit transport abstraction to ensure no WebSocket leakage in business logic layers
- [ ] T052 [US7] Create mock transport implementation for testing in tests/unit/websocket/mock_transport.go (depends on T051)
- [ ] T053 [US7] Document transport abstraction architecture and extension points in implementation notes (depends on T051)
- [ ] T054 [US7] Add inline comments documenting transport interface contract in src/internal/websocket/transport.go (depends on T051)

**Checkpoint**: Transport abstraction is verified and documented for future protocol changes

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories and final production readiness

- [ ] T055 [P] Add Apache License 2.0 headers to all new source files (websocket/, handler/, service/, model/, dto/)
- [ ] T056 Create implementation notes document at spec/impls/gateway-websocket-events.md with entry points, verification steps, and architecture overview
- [ ] T057 [P] Add graceful shutdown handler to close all WebSocket connections with 1000 Normal Closure code in src/internal/server/server.go
- [ ] T058 [P] Add structured logging configuration (DEBUG: heartbeats, INFO: connections, WARN: auth failures, ERROR: send failures) in src/internal/server/server.go
- [ ] T059 [P] Add inline documentation for non-obvious design decisions in transport.go, manager.go, gateway_events.go
- [ ] T060 Code review and refactoring pass across all new WebSocket components
- [ ] T061 Performance testing for 100 events/second throughput and 100-1000 concurrent connections
- [ ] T062 Security review for authentication flow, rate limiting, and error message information disclosure
- [ ] T063 Update main README.md with WebSocket endpoint documentation and authentication requirements

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-9)**: All depend on Foundational phase completion
  - P1 stories (US1, US2, US3) should be completed sequentially in order: US1 → US2 → US3
  - P2 stories (US4, US5, US6) can start after P1 completion
  - P3 story (US7) can start after P1 completion
- **Polish (Phase 10)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories - REQUIRED for US2
- **User Story 2 (P1)**: Can start after US1 complete - Requires connection registry from US1
- **User Story 3 (P1)**: Can start after US1 complete - Requires connection manager from US1
- **User Story 4 (P2)**: Can start after US2 complete - Extends event broadcasting from US2
- **User Story 5 (P2)**: Can start after US2 complete - Enhances failure handling from US2
- **User Story 6 (P2)**: Can start after US1 complete - Enhances authentication from US1
- **User Story 7 (P3)**: Can start after Foundational (Phase 2) - Reviews transport abstraction design

### Within Each User Story

- Models before services
- Services before handlers
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks can run in parallel (only T001 in this case)
- All Foundational tasks marked [P] can run in parallel: T002, T003, T004, T005
- Within US1: T007 and T008 can run in parallel
- Within US3: T031 can start immediately when Phase 5 begins
- All Phase 10 tasks marked [P] can run in parallel: T055, T057, T058, T059

---

## Implementation Strategy

### MVP First (P1 Stories Only)

1. Complete Phase 1: Setup (T001)
2. Complete Phase 2: Foundational (T002-T006) - CRITICAL
3. Complete Phase 3: User Story 1 (T007-T020) - Connection establishment
4. **STOP and VALIDATE**: Test gateway connection independently
5. Complete Phase 4: User Story 2 (T021-T030) - Event delivery
6. **STOP and VALIDATE**: Test deployment event delivery independently
7. Complete Phase 5: User Story 3 (T031-T036) - Stats API
8. **STOP and VALIDATE**: Test stats endpoint independently
9. Complete Phase 10: Polish (T055-T063)
10. Deploy/demo MVP with full P1 functionality

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (gateways can connect!)
3. Add User Story 2 → Test independently → Deploy/Demo (deployment events work!)
4. Add User Story 3 → Test independently → Deploy/Demo (stats API available!)
5. Add User Story 4 → Test independently → Deploy/Demo (multiple event types!)
6. Add User Story 5 → Test independently → Deploy/Demo (failure visibility!)
7. Add User Story 6 → Test independently → Deploy/Demo (enhanced security!)
8. Add User Story 7 → Test independently → Deploy/Demo (transport abstraction verified!)
9. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies, can run in parallel
- [Story] label maps task to specific user story for traceability (US1-US7)
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Tests are not included as they were not explicitly requested in the specification
- All file paths follow single project structure (platform-api monolith)
- Focus on P1 stories first (US1, US2, US3) for MVP delivery
- P2 and P3 stories can be delivered incrementally after MVP

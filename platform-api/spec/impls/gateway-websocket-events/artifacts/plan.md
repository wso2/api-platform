# Implementation Plan: Gateway Event Notification System

**Spec**: [spec.md](./spec.md)

## Summary

Implement a WebSocket-based event notification system that enables real-time communication between the platform API and gateway instances. Gateways establish persistent connections to the platform at `wss://localhost:9243/api/internal/v1/ws/gateways/connect`, and the platform broadcasts deployment events (and other event types) to connected gateways. The system uses a transport abstraction layer to allow future protocol changes without modifying business logic, supports multiple connections per gateway ID for clustering scenarios, and handles authentication, connection lifecycle management, and in-memory failure tracking.

**Primary Requirement**: Enable real-time API deployment notifications from platform to gateways via persistent WebSocket connections.

**Technical Approach**:
- Use `github.com/gorilla/websocket` for WebSocket protocol handling (industry-standard Go library)
- Implement layered architecture: Handler → Service → Connection Manager → Transport
- In-memory connection registry using `sync.Map` for thread-safe concurrent access
- Event broadcasting to all connections for a gateway ID
- Integration with existing API deployment service to trigger events

## Technical Context

**Language/Version**: Go 1.24
**Primary Dependencies**:
- Existing: `github.com/gin-gonic/gin v1.11.0` (HTTP framework)
- New: `github.com/gorilla/websocket v1.5.3` (WebSocket protocol)

**Storage**:
- In-memory only for connection registry and delivery statistics (no persistence)

**Testing**: Go testing framework (`testing` package), table-driven tests for business logic

**Target Platform**: Linux server (containerized, existing Dockerfile)

**Project Type**: Single Go application (platform-api monolith)

**Performance Goals**:
- 100 events/second across all gateways (throughput)
- Support 100-1000 concurrent WebSocket connections
- < 10MB memory overhead for connection registry

**Constraints**:
- Must use TLS/WSS for all connections
- Connection registry must support multiple connections per gateway ID
- Event ordering must be preserved per gateway
- Maximum event payload size: 1MB (configurable)
- Rate limiting: 10 connection attempts/minute/IP

**Scale/Scope**:
- P1: Connection management + API deployment events + stats endpoint
- P2: Multiple event types, enhanced statistics
- P3: Advanced observability, metrics

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Principle I: Specification-First Development
**Status**:  PASS
- Feature spec complete at `platform-api/spec/impls/gateway-websocket-events/artifacts/spec.md`
- Implementation plan (this file) documents architecture and design
- OpenAPI contract will be generated in Phase 1

### Principle II: Layered Architecture
**Status**:  PASS
- **Handler Layer** (`internal/handler/websocket.go`): WebSocket upgrade, HTTP routing
- **Service Layer** (`internal/service/gateway_events.go`): Event broadcast logic, connection tracking
- **Connection Manager** (`internal/websocket/manager.go`): Connection registry, lifecycle management
- **Transport Abstraction** (`internal/websocket/transport.go`): Interface for protocol independence
- Clear separation between WebSocket-specific code and business logic

### Principle III: Security by Default
**Status**:  PASS
- TLS/WSS required (reuse existing HTTPS server configuration)
- Authentication middleware validates gateway credentials before upgrade
- Apache License 2.0 headers on all new source files
- Input validation: connection limits, rate limiting, payload size checks
- Structured error responses without leaking implementation details

### Principle IV: Documentation Traceability
**Status**:  PASS
- This plan links to spec.md
- Implementation notes will be created at `spec/impls/gateway-websocket-events.md`
- OpenAPI contract in `src/resources/openapi.yaml` (administrative APIs)
- Verification steps will include WebSocket test clients

### Principle V: RESTful API Standards
**Status**:  PASS (with note)
- Administrative stats API follows REST conventions (`GET /api/internal/v1/stats`)
- WebSocket endpoint at `/api/internal/v1/ws/gateways/connect` (not RESTful by nature, but follows URL structure)
- camelCase in JSON payloads
- Stats endpoint returns standard structure with gateway connection details

**Note**: WebSocket is not RESTful (stateful, bidirectional). The constitution acknowledges REST for HTTP APIs; WebSocket is a necessary exception for real-time communication.

### Principle VI: Data Integrity
**Status**:  PASS (N/A)
- No database persistence for this feature
- All state is in-memory (connection registry, delivery statistics)

### Principle VII: Container-First Operations
**Status**:  PASS
- Reuse existing Dockerfile (no changes needed)
- Health check endpoint `/health` already exists
- Configuration via environment variables:
  - `WS_MAX_CONNECTIONS`: Maximum concurrent connections (default 1000)
  - `WS_CONNECTION_TIMEOUT`: Heartbeat timeout in seconds (default 30)
  - `WS_RATE_LIMIT_PER_MINUTE`: Connection rate limit (default 10)

**Overall Gate Status**:  PASS - No constitution violations. WebSocket is necessary for real-time requirements and is properly abstracted.

## Project Structure

### Documentation (this feature)

```
platform-api/spec/impls/gateway-websocket-events/
├── gateway-websocket-events.md  # Implementation notes
└── artifacts/
    ├── spec.md          # Feature specification
    ├── plan.md          # This file - Implementation plan
    └── tasks.md         # Task breakdown
```

### Source Code (repository root)

**Existing Structure** (platform-api):
```
src/
├── internal/
│   ├── handler/           # HTTP handlers
│   │   ├── api.go         # Existing API handlers
│   │   ├── stats.go       # NEW: Stats endpoint handler
│   │   └── websocket.go   # NEW: WebSocket connection handler
│   ├── service/
│   │   ├── api.go         # Existing API service
│   │   └── gateway_events.go  # NEW: Event broadcasting service
│   ├── websocket/         # NEW: WebSocket-specific components
│   │   ├── manager.go     # Connection registry and lifecycle
│   │   ├── transport.go   # Transport abstraction interface
│   │   ├── websocket_transport.go  # WebSocket implementation
│   │   └── connection.go  # Connection wrapper with metadata
│   ├── model/
│   │   └── gateway_event.go  # NEW: Event model
│   ├── dto/
│   │   └── gateway_event.go  # NEW: Event DTOs
│   └── server/
│       └── server.go      # MODIFIED: Register WebSocket routes
└── resources/
    └── openapi.yaml       # MODIFIED: Add stats endpoint
```

**Structure Decision**: Single project structure (platform-api monolith). WebSocket functionality is tightly integrated with existing API deployment logic, so keeping it in the same service simplifies integration and deployment. The `internal/websocket` package provides clear module boundary for WebSocket-specific code.

## Complexity Tracking

*No constitution violations requiring justification.*

## Phase 0: Research & Technical Decisions

**Objective**: Resolve technology choices and architectural patterns for WebSocket implementation.

### Research Tasks

1. **WebSocket Library Selection** (gorilla/websocket vs nhooyr.io/websocket)
   - Decision: `github.com/gorilla/websocket v1.5.3`
   - Rationale: Industry standard, mature (10+ years), excellent Gin integration examples, supports custom upgraders
   - Alternatives: nhooyr.io/websocket (newer, HTTP/2 support) - rejected because HTTP/1.1 WebSocket is sufficient for P1

2. **Connection Registry Pattern** (sync.Map vs custom shard map vs external cache)
   - Decision: `sync.Map` with gateway ID → []Connection mapping
   - Rationale: Built-in, thread-safe, optimized for read-heavy workloads (event delivery), sufficient for 1000 connections
   - Alternatives: Redis (over-engineering for P1), sharded map (unnecessary complexity)

3. **Heartbeat/Keepalive Mechanism**
   - Decision: Ping/Pong frames every 20 seconds, 30-second timeout
   - Rationale: WebSocket standard ping/pong, gorilla/websocket has built-in support, RFC 6455 compliant
   - Implementation: `SetReadDeadline()` + automatic pong handler

4. **Authentication Approach**
   - Decision: API key-based authentication using existing gateway registration system
   - Rationale: Gateway sends `api-key: <token>` header during WebSocket upgrade. Token is obtained during gateway registration (see gateway-management implementation)
   - Validation: Gin middleware calls `gatewayService.VerifyToken(apiKey)` before WebSocket upgrade
   - Gateway ID extraction: `VerifyToken` returns `*model.Gateway` which includes the gateway UUID for connection registry association

5. **Event Serialization Format**
   - Decision: JSON (newline-delimited JSON messages)
   - Rationale: Human-readable, debuggable, lightweight for small events (< 10KB), native Go support
   - Structure: `{"type": "api.deployed", "payload": {...}, "timestamp": "...", "correlationId": "..."}`

6. **Graceful Shutdown Pattern**
   - Decision: Close WebSocket connections with 1000 Normal Closure code on server shutdown
   - Rationale: Allows gateways to distinguish shutdown from failure, triggers immediate reconnect
   - Implementation: Listen for OS signals, iterate connection registry, send close frames

7. **Error Handling and Logging**
   - Decision: Structured logging with zerolog (if exists) or standard log package
   - Log levels: DEBUG (heartbeats), INFO (connections), WARN (auth failures), ERROR (send failures)

8. **In-Memory Statistics Tracking**
   - Decision: Track delivery statistics in-memory with atomic counters
   - Rationale: Simple, no persistence overhead, sufficient for operational visibility
   - Metrics: Total sent, failed deliveries, success rate (counters reset on restart)

### Output Artifact

See `research.md` for detailed findings, benchmarks, and code examples for each decision.

## Phase 1: Design & Contracts

**Prerequisites**: `research.md` complete

### Data Model

**Entities** (see `data-model.md` for full schemas):

1. **GatewayConnection** (in-memory only)
   - `GatewayID string` (UUID)
   - `ConnectionID string` (unique per connection instance)
   - `ConnectedAt time.Time`
   - `LastHeartbeat time.Time`
   - `Conn *websocket.Conn` (gorilla websocket connection)
   - `AuthToken string` (for validation)

2. **GatewayEvent** (model)
   - `Type string` (e.g., "api.deployed")
   - `Payload json.RawMessage` (event-specific data)
   - `GatewayID string`
   - `Timestamp time.Time`
   - `CorrelationID string` (UUID for tracing)

3. **APIDeploymentEvent** (payload for api.deployed type)
   - `APIUUID string`
   - `RevisionID string`
   - `Vhost string`
   - `Environment string`

4. **DeliveryStats** (in-memory counters)
   - `TotalEventsSent int64` (atomic)
   - `FailedDeliveries int64` (atomic)
   - `LastFailureTime time.Time`
   - `LastFailureReason string`

**State Transitions**:
- Connection: `connecting → connected → heartbeating → disconnecting → disconnected`
- Event Delivery: `pending → sending → success|failed`

### API Contracts

**WebSocket Endpoint** (not OpenAPI, documented in README):
- **URL**: `wss://localhost:9243/api/internal/v1/ws/gateways/connect`
- **Protocol**: WebSocket (RFC 6455)
- **Authentication**: `api-key: <token>` header (token obtained from gateway registration)
- **Message Format**: JSON text frames
- **Client → Server**: Pong frames (automatic), optional heartbeat ACKs
- **Server → Client**:
  - Connection ACK: `{"type": "connection.ack", "gatewayId": "..."}`
  - Events: `{"type": "api.deployed", "payload": {...}, "timestamp": "...", "correlationId": "..."}`

**Administrative HTTP APIs** (OpenAPI):

**GET /api/internal/v1/stats**
- Returns platform statistics including gateway connection details
- Response:
  ```json
  {
    "gatewayConnections": {
      "totalActive": 5,
      "connections": [
        {
          "gatewayId": "uuid",
          "connectionId": "uuid",
          "connectedAt": "2025-10-15T10:00:00Z",
          "lastHeartbeat": "2025-10-15T10:05:00Z",
          "status": "connected"
        }
      ]
    },
    "eventDelivery": {
      "totalEventsSent": 1234,
      "failedDeliveries": 5,
      "lastFailureTime": "2025-10-15T09:30:00Z",
      "lastFailureReason": "gateway not connected"
    }
  }
  ```

See `contracts/websocket-stats-api.yaml` for full OpenAPI 3.0 specification.

### Integration Points

1. **API Deployment Service** (`internal/service/api.go` - DeployAPIRevision method)
   - After successful deployment, call `gatewayEventsService.BroadcastDeploymentEvent(gatewayID, apiUUID, revision, vhost)`
   - Service looks up connections and sends events asynchronously
   - Logs failures but does not persist them

2. **Server Initialization** (`internal/server/server.go`)
   - Initialize `websocket.Manager` singleton
   - Register WebSocket handler: `router.GET("/api/internal/v1/ws/gateways/connect", websocketHandler.Connect)`
   - Register stats API route: `router.GET("/api/internal/v1/stats", statsHandler.GetStats)`

### Component Interactions

```
[Gateway Client]
       |
       | WSS Upgrade Request
       v
[WebSocket Handler] --> [Auth Middleware] --> [WebSocket Manager]
       |                                              |
       | Connection Registered                        |
       v                                              |
[Connection Registry (sync.Map)]                     |
       ^                                              |
       | Lookup                                       |
       |                                              |
[Gateway Events Service] <-- [API Deployment Service]
       |
       | Send Event (increment counters)
       v
[WebSocket Transport] --> [gorilla/websocket Conn] --> [Gateway Client]
       |
       | Update Stats (atomic counters)
       v
[In-Memory DeliveryStats] <-- [Stats Handler] --> [Stats API Response]
```

### Agent Context Update

Run: `.specify/scripts/bash/update-agent-context.sh claude`

Technology to add:
- WebSocket (gorilla/websocket library)
- Connection registry pattern (sync.Map)
- Event-driven architecture (broadcast pattern)

## Phase 2: Task Breakdown (NOT DONE IN THIS COMMAND)

Phase 2 will be executed by `/speckit.tasks` command, which will generate `tasks.md` with:
- Dependency-ordered implementation tasks
- Test scenarios for each task
- Estimated complexity per task
- Verification steps

**Estimated Task Categories**:
1. Foundation: Transport abstraction interface, Manager skeleton
2. Core: WebSocket handler, connection lifecycle, heartbeat
3. Integration: Event service, API deployment hook
4. Observability: Logging, stats API, in-memory counters
5. Testing: Unit tests, integration tests, load testing

## Notes

- **P1 Scope**: Connection management + API deployment events + stats endpoint
- **Future Work** (post-P1):
  - Additional event types (api.undeployed, gateway.config.updated)
  - Gateway-initiated sync capability (polling for missed events)
  - Metrics/Prometheus exporter for connection counts
  - Distributed connection registry (Redis) for multi-instance platform HA
  - Message acknowledgment protocol (at-least-once delivery guarantee)
  - Persistent event delivery history (if needed)

- **Risk Mitigation**:
  - WebSocket library is battle-tested (gorilla/websocket)
  - In-memory registry is simple; can migrate to external store if needed
  - Transport abstraction allows swapping WebSocket for SSE/gRPC later
  - No database dependency reduces complexity

- **Dependencies on External Systems**:
  - Gateway must implement WebSocket client logic (out of scope)
  - API runtime artifacts endpoint (GET /api/internal/v1/api-runtime-artifacts/{apiId}) must exist for gateways to fetch full API YAML
  
# Feature: Gateway WebSocket Event Notification System

## Overview

Enables real-time bidirectional communication between the platform and gateway instances via WebSocket protocol for event notifications and connection management.

## Git Commits

- `c7467bb` - Add WebSocket dependency and core event infrastructure
- `b18ff71` - Implement WebSocket connection management and handler
- `38bcfff` - Implement gateway events service and API deployment event broadcasting

## Motivation

Gateways need real-time notification of API deployments and configuration changes. Polling-based approaches introduce latency and increase server load. WebSocket provides persistent, low-latency bidirectional channels enabling instant event delivery to connected gateways.

## Entry Points

- **`internal/websocket/manager.go`** - Connection lifecycle management and heartbeat monitoring
- **`internal/websocket/websocket_transport.go`** - gorilla/websocket protocol implementation
- **`internal/handler/websocket.go`** - HTTP upgrade handler with authentication
- **`internal/service/gateway_events.go`** - Event broadcasting service
- **`internal/service/api.go`** - DeployAPIRevision integration
- **`config/config.go`** - WebSocket configuration (max connections, timeout, rate limits)
- **`internal/server/server.go`** - Server initialization and route registration

## Behaviour

### Connection Establishment

1. Gateway initiates WebSocket upgrade request to `wss://platform-api:9243/api/internal/v1/ws/gateways/connect` with an `api-key` header carrying the gateway token
2. Platform validates the token via `GatewayService.VerifyToken`
3. Platform enforces rate limiting (default 1000 attempts/minute/IP)
4. Platform checks the per-organization connection limit (default 3) before upgrading
5. HTTP connection upgrades to WebSocket protocol
6. Platform sends a `connection.ack` message with the gateway's internal UUID (`gatewayId`) and a new connection ID
7. Connection registered in sync.Map registry keyed by gateway UUID; gateway `isActive` is set to `true`

### Heartbeat Mechanism

- Platform sends ping frames every 20 seconds
- Gateway must respond with pong within 30 seconds
- Connection terminated if pong not received (heartbeat timeout)
- Pong handler updates last heartbeat timestamp atomically

### Event Broadcasting

1. API deployment triggers in `DeployAPIRevision` service method
2. `GatewayEventsService.BroadcastDeploymentEvent` called with gateway ID and event payload
3. Event serialized to JSON with type, payload, timestamp, correlation ID
4. Payload size validated (1MB maximum)
5. Manager lookups all connections for target gateway ID
6. Event sent sequentially to each connection (maintains ordering)
7. Delivery statistics updated atomically (total sent, failed deliveries)
8. Failures logged with correlation ID, do not fail deployment

### Disconnection Handling

**Graceful**:
- Gateway sends WebSocket close frame
- Platform acknowledges and removes from registry
- Connection count decremented

**Ungraceful**:
- Heartbeat timeout detected (no pong within 30s)
- Manager removes connection from registry
- Connection count decremented

## Architecture

```
┌─────────────┐                    ┌──────────────────┐
│   Gateway   │◄───WebSocket──────►│  Platform API    │
│  Instance   │    (TLS/WSS)       │                  │
└─────────────┘                    └──────────────────┘
                                            │
                                            ▼
                                   ┌─────────────────┐
                                   │ WebSocket       │
                                   │ Manager         │
                                   │ (sync.Map)      │
                                   └─────────────────┘
                                            │
                    ┌───────────────────────┼───────────────────┐
                    ▼                       ▼                   ▼
            ┌──────────────┐       ┌──────────────┐   ┌──────────────┐
            │ Connection 1 │       │ Connection 2 │   │ Connection N │
            │ (Gateway A)  │       │ (Gateway A)  │   │ (Gateway B)  │
            └──────────────┘       └──────────────┘   └──────────────┘
```

**Component Responsibilities**:

- **Transport Interface**: Protocol abstraction (Send, Close, Ping/Pong)
- **WebSocketTransport**: gorilla/websocket implementation
- **Connection**: Wrapper with metadata (gateway ID, connection ID, timestamps, stats)
- **Manager**: Registry, heartbeat monitoring, graceful shutdown
- **WebSocketHandler**: HTTP upgrade, authentication, rate limiting
- **GatewayEventsService**: Event serialization, broadcasting, delivery tracking

## Implementation Details

### Transport Abstraction

```go
type Transport interface {
    Send(message []byte) error
    Close(code int, reason string) error
    SetReadDeadline(deadline time.Time) error
    SetWriteDeadline(deadline time.Time) error
    EnablePongHandler(handler func(string) error)
    SendPing() error
}
```

**Design Rationale**: Decouples business logic from WebSocket protocol, enabling future protocol changes (SSE, gRPC streaming) without modifying connection management code.

### Connection Registry

**Data Structure**: `sync.Map` with key = gateway ID, value = `[]*Connection`

**Why sync.Map**:
- Read-optimized for event delivery lookups (frequent)
- Thread-safe for concurrent access
- No lock contention for different gateway IDs
- Supports multiple connections per gateway (clustering)

### Heartbeat Monitoring

Each connection gets dedicated goroutine:
```go
ticker := time.NewTicker(20 * time.Second)
for {
    select {
    case <-shutdownCtx.Done():
        return
    case <-ticker.C:
        if time.Since(lastHeartbeat) > 30*time.Second {
            // Timeout - disconnect
        }
        SendPing()
    }
}
```

**Design Rationale**: Per-connection goroutines isolate failures, prevent blocking, enable graceful shutdown via context cancellation.

### Event Serialization

```json
{
  "type": "api.deployed",
  "payload": {
    "apiId": "api-uuid",
    "deploymentId": "deployment-uuid",
    "performedAt": "2026-06-21T..."
  },
  "gatewayId": "gateway-uuid",
  "timestamp": "2026-06-21T...",
  "correlationId": "uuid"
}
```

`apiId` and `gatewayId` are the platform's internal UUIDs — not the handle-based `id` used in the REST API.

**Event Ordering**: Events sent sequentially per connection using mutex-protected Send method.

### Delivery Statistics

Per-connection atomic counters:
- `TotalEventsSent` - incremented on successful delivery
- `FailedDeliveries` - incremented on send error
- `LastFailureTime` - timestamp of most recent failure
- `LastFailureReason` - error message from last failure

## Key Technical Decisions

1. **gorilla/websocket v1.5.3**: Industry-standard WebSocket library with production-ready ping/pong handling
2. **sync.Map over map[string][]*Connection + RWMutex**: Read-optimized, lower contention for event delivery
3. **Multiple connections per gateway**: Supports gateway clustering without connection conflicts
4. **No event persistence**: Clean slate on reconnect, gateways sync state after connection established
5. **Partial delivery success**: If any connection receives event, deployment succeeds (availability over consistency)
6. **Rate limiting by IP**: Prevents connection flooding, 10/minute default
7. **1MB payload limit**: Prevents memory exhaustion from large events
8. **Correlation IDs**: Enables distributed tracing across platform and gateways

## Configuration

**File**: `config/config.go` (loaded via koanf, not the old envconfig setup)

```go
type WebSocket struct {
    MaxConnections       int  `koanf:"max_connections"`
    ConnectionTimeout    int  `koanf:"connection_timeout"`
    RateLimitPerMin      int  `koanf:"rate_limit_per_min"`
    MaxConnectionsPerOrg int  `koanf:"max_connections_per_org"`
    MetricsLogEnabled    bool `koanf:"metrics_log_enabled"`
    MetricsLogInterval   int  `koanf:"metrics_log_interval"`
}
```

**Environment Variables** (see `config.go:envToKoanfKey` for the full mapping, including legacy `WEBSOCKET_WS_*` aliases):
- `WEBSOCKET_MAX_CONNECTIONS` - Maximum concurrent connections (default: 1000)
- `WEBSOCKET_CONNECTION_TIMEOUT` - Heartbeat timeout in seconds (default: 30)
- `WEBSOCKET_RATE_LIMIT_PER_MIN` - Connection attempts per IP per minute (default: 1000)
- `WEBSOCKET_MAX_CONNECTIONS_PER_ORG` - Concurrent connections allowed per organization (default: 3)
- `WEBSOCKET_METRICS_LOG_ENABLED` - Periodically log connection metrics (default: true)
- `WEBSOCKET_METRICS_LOG_INTERVAL` - Metrics log interval in seconds (default: 10)

## Build & Run

```bash
# Build platform API with WebSocket support
cd platform-api/src
go build -o ../bin/platform-api ./cmd/main.go

# Run with default configuration
cd ../bin
./platform-api

# Run with custom WebSocket configuration
export WEBSOCKET_MAX_CONNECTIONS=5000
export WEBSOCKET_CONNECTION_TIMEOUT=60
export WEBSOCKET_RATE_LIMIT_PER_MIN=20
./platform-api
```

## Verification

### 1. Gateway Connection

**Register Gateway** (organization ID comes from the JWT token, not the request body):
```bash
curl -k -X POST https://localhost:9243/api/v0.9/gateways \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <your-token>' \
  -d '{
    "id": "test-gateway",
    "displayName": "Test Gateway",
    "endpoints": ["https://test-gateway.example.com:8443/api/v1"],
    "functionalityType": "regular"
  }'
```

**Expected Response**:
```json
{
  "id": "test-gateway",
  "displayName": "Test Gateway",
  "organizationId": "acme",
  "endpoints": ["https://test-gateway.example.com:8443/api/v1"],
  "functionalityType": "regular",
  "isActive": false,
  "createdAt": "2026-06-21T..."
}
```

**Generate Gateway Token** (`{gatewayId}` is the handle from the response above, not a UUID):
```bash
curl -k -X POST https://localhost:9243/api/v0.9/gateways/test-gateway/tokens \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer <your-token>'
```

**Expected Response**:
```json
{
  "id": "1db8b0e4-f237-4aa3-a6f2-e466c878de0f",
  "token": "guDgqzePBJTMD8iElVH-q4_hc3IWZE87PgBqfzS_qPA",
  "createdAt": "2026-06-21T14:00:43+05:30",
  "message": "New token generated successfully. Old token remains active until revoked."
}
```

**Connect via WebSocket (using wscat)**:
```bash
wscat -n -c wss://localhost:9243/api/internal/v1/ws/gateways/connect \
  -H "api-key: guDgqzePBJTMD8iElVH-q4_hc3IWZE87PgBqfzS_qPA"
```

**Expected Output**:
```
Connected (press CTRL+C to quit)
< {"type":"connection.ack","gatewayId":"d1aa71bc-8cb5-4294-8a26-fe1273c28632","connectionId":"85d759e5-f152-4a39-8cd1-e0923657268a","timestamp":"2026-06-21T07:11:04+05:30"}
```

`gatewayId` here is the gateway's internal UUID (`gateway.ID`), not the handle (`test-gateway`) used in the REST API.

### 2. Event Delivery

**Deploy an API** (with gateway connected; requires a REST API already created via `POST /rest-apis` and a `projectId`/`upstream` — see [API Lifecycle Management](../api-lifecycle-management.md)):
```bash
curl -k -X POST 'https://localhost:9243/api/v0.9/rest-apis/<apiHandle>/deployments' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer <your-token>' \
  -d '{
    "name": "test-deployment",
    "base": "current",
    "gatewayId": "test-gateway"
  }'
```

**Expected Gateway Output (received via WebSocket)**:
```
< {"type":"api.deployed","payload":{"apiId":"23826f9e-8daf-4638-b295-14898312759f","deploymentId":"90d10e1c-8560-5c36-9d5a-124ecaa17485","performedAt":"2026-06-21T07:11:07+05:30"},"gatewayId":"d1aa71bc-8cb5-4294-8a26-fe1273c28632","timestamp":"2026-06-21T07:11:07+05:30","correlationId":"16408ddb-0cc9-48bc-a35d-6debb8d90c28"}
```

### 3. Authentication Rejection

```bash
curl -k -s -o /dev/null -w "%{http_code}" \
  -H "Upgrade: websocket" \
  -H "Connection: Upgrade" \
  -H "api-key: invalid-key" \
  "https://localhost:9243/api/internal/v1/ws/gateways/connect"
```

**Expected**: `401`

## Challenges & Solutions

### Challenge 1: Connection Cleanup on Ungraceful Disconnect

**Problem**: Network failures or gateway crashes don't trigger close frame, connections remain in registry.

**Solution**: Heartbeat timeout detection. If no pong received within 30s, connection removed from registry automatically.

### Challenge 2: Event Ordering Guarantee

**Problem**: Concurrent broadcasts to same gateway could deliver events out of order.

**Solution**: Sequential delivery per connection using mutex in `Connection.Send()`. Events for different gateways can be sent concurrently.

### Challenge 3: Partial Delivery Handling

**Problem**: Gateway cluster has 3 instances, 1 disconnected during deployment. Should deployment fail?

**Solution**: Partial success model. If any connection receives event, deployment succeeds. Failed deliveries logged with correlation ID for troubleshooting.

### Challenge 4: Memory Leaks from Failed Connections

**Problem**: Failed connections accumulate in registry if not cleaned up.

**Solution**: Three-layer cleanup:
1. Heartbeat goroutine exits on timeout → triggers unregister
2. Read loop detects connection closure → triggers unregister
3. Graceful shutdown closes all connections → clears registry

## Testing Approach

### Automated Tests

The standalone `test-phase3.sh` script referenced in earlier revisions of this doc no longer
exists, and there is currently no dedicated Go test coverage for `internal/websocket/` or the
WebSocket upgrade handler (`internal/handler/websocket.go`) — verification today is manual (see
below). This is a coverage gap worth closing, not an intentional design choice.

### Manual Tests

1. **Connection Test**: Connect gateway, verify ACK message
2. **Heartbeat Test**: Keep connection open 60+ seconds, verify ping/pong
3. **Deployment Event Test**: Deploy API, verify gateway receives event
4. **Multiple Connections Test**: Connect 2+ instances with same gateway ID
5. **Rate Limit Test**: Attempt 15 connections in 1 minute (expect 11th to fail)
6. **Graceful Shutdown Test**: Stop server, verify clients receive close frame
7. **Disconnection Test**: Kill gateway process, verify server detects timeout

### Performance Considerations

- **Concurrent Connections**: Tested up to 1000 concurrent connections
- **Event Throughput**: Supports 100+ events/second broadcast rate
- **Memory Usage**: ~10KB per connection (goroutine + connection metadata)
- **CPU Usage**: Minimal (<1% per 100 connections with ping/pong only)

## Data Model

### Connection Metadata

```go
type Connection struct {
    GatewayID     string         // Gateway UUID from registration
    ConnectionID  string         // Unique per connection (UUID)
    ConnectedAt   time.Time      // Connection establishment timestamp
    LastHeartbeat time.Time      // Most recent pong timestamp
    Transport     Transport      // Protocol implementation
    AuthToken     string         // API key used for authentication
    DeliveryStats *DeliveryStats // Per-connection delivery tracking
    closed        bool           // Connection state flag
}
```

### Delivery Statistics

```go
type DeliveryStats struct {
    TotalEventsSent   int64     // Atomic counter
    FailedDeliveries  int64     // Atomic counter
    LastFailureTime   time.Time // Timestamp of last failure
    LastFailureReason string    // Error message from last failure
}
```

### Event Payload

Defined in `internal/model/gateway_event.go`:

```go
type GatewayEvent struct {
    Type          string          `json:"type"`          // Event type identifier
    Payload       json.RawMessage `json:"payload"`       // Event-specific data
    GatewayID     string          `json:"gatewayId"`      // Target gateway's internal UUID
    Timestamp     time.Time       `json:"timestamp"`
    CorrelationID string          `json:"correlationId"` // Distributed tracing ID
}

type DeploymentEvent struct {
    ApiId        string    `json:"apiId"`        // API's internal UUID
    DeploymentID string    `json:"deploymentId"` // Deployment's internal UUID
    PerformedAt  time.Time `json:"performedAt"`  // Concurrency token
}
```

Sibling event types (`APIUndeploymentEvent`, `APIDeletionEvent`, and the LLM/MCP-proxy deployment events) follow the same `apiId`/`deploymentId`/`performedAt` shape.

## Security Considerations

### Authentication

- API key validation via existing `GatewayService.VerifyToken`
- Connection rejected before WebSocket upgrade on invalid credentials
- No credential exposure in logs (masked in error messages)

### Rate Limiting

- Per-IP tracking prevents connection flooding
- Sliding window implementation (1 minute)
- Configurable limit (default: 10/minute)
- Returns HTTP 429 when exceeded

### Connection Limits

- Maximum concurrent connections enforced (default: 1000)
- Prevents resource exhaustion attacks
- Returns error message when limit reached

### Payload Validation

- 1MB maximum event size prevents memory exhaustion
- Payload size checked before broadcasting
- Early rejection reduces wasted network bandwidth

### TLS Requirement

- WebSocket connections use TLS (wss://)
- Reuses platform API's HTTPS server configuration
- Self-signed certificates for development, proper CA certs for production

## Design Artifacts

Supporting design documents from spec-kit planning:

- [Feature Specification](artifacts/spec.md) - Detailed requirements, user scenarios, and acceptance criteria
- [Implementation Plan](artifacts/plan.md) - Technical context, constitution compliance, and implementation phases
- [Task Breakdown](artifacts/tasks.md) - Dependency-ordered task list with completion tracking

## Related Features

- [Gateway Management](../gateway-management/gateway-management.md) - Gateway registration and token generation
- [API Lifecycle Management](../api-lifecycle-management.md) - API deployment triggering events

## Future Enhancements

### Phase 5: Connection Statistics API

Endpoint: `GET /api/internal/v1/stats`

Returns:
- Total active connections
- Connections per gateway
- Connection durations
- Delivery statistics
- Heartbeat health

### Phase 6: Additional Event Types

- `api.undeployed` - API removed from gateway
- `gateway.config.updated` - Configuration changes
- `api.lifecycle.changed` - State transitions (published, deprecated, retired)
- `subscription.created` - New application subscription

### Phase 7: Event Persistence & Replay

- Store events in database with TTL (24 hours)
- Replay missed events on reconnection
- Configurable replay window
- Event sequence numbers for ordering

### Phase 8: Advanced Delivery Guarantees

- At-least-once delivery with acknowledgments
- Gateway sends ACK for received events
- Platform retries unacknowledged events
- Idempotency keys in event payloads

### Phase 9: Compression

- WebSocket per-message compression (permessage-deflate)
- Reduces bandwidth for large payloads
- Configurable compression level

### Phase 10: Metrics & Observability

- Prometheus metrics export
- Connection count gauges
- Event delivery rate histograms
- Heartbeat latency tracking
- Grafana dashboard templates

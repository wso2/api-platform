# Feature: Gateway to Control Plane Registration

## Overview

Enables automatic gateway registration with the control plane via persistent WebSocket connection on startup. Gateways authenticate using registration tokens and maintain connection health through heartbeat monitoring with automatic reconnection.

## Git Commits

- `1115119` - Implement WebSocket client with connection lifecycle and reconnection logic
- `86eb1b5` - Initialize control plane client in gateway controller main
- `dbe479f` - Add control plane configuration and WebSocket dependency to gateway controller

## Motivation

Gateway instances deployed across distributed environments need automated registration with the control plane to receive real-time API deployment events, configuration updates, and policy changes. Manual registration doesn't scale beyond small deployments and introduces operational overhead. WebSocket-based registration provides persistent low-latency communication channels with automatic reconnection, enabling zero-touch gateway deployment.

## Entry Points

- **`cmd/gateway-controller/main.go`** - Application initialization and control plane client setup
- **`pkg/config/config.go`** - Control plane configuration loading and validation
- **`pkg/controlplane/client.go`** - WebSocket connection lifecycle management
- **`pkg/controlplane/events.go`** - Event message definitions

## Behaviour

### Startup and Connection Establishment

1. Gateway loads control plane configuration from environment variables on startup
2. Validates required `GATEWAY_REGISTRATION_TOKEN` and optional `GATEWAY_CONTROL_PLANE_URL`
3. Default control plane URL: `wss://localhost:8443/api/internal/v1/ws/gateways/connect`
4. Creates control plane client with configuration
5. Starts connection in background goroutine (non-blocking)
6. Gateway main process continues with traffic routing initialization
7. Client initiates WebSocket handshake with `api-key` header containing registration token
8. Control plane validates token and upgrades HTTP connection to WebSocket
9. Control plane sends `connection.ack` message with gateway ID and connection ID
10. Client transitions to `Connected` state and logs connection details
11. Client enables pong handler to update heartbeat timestamp on ping responses

### Heartbeat Mechanism

- Control plane sends ping frames every 20 seconds
- Gateway responds with pong frames automatically (gorilla/websocket default behavior)
- Client tracks `LastHeartbeat` timestamp atomically on pong received
- Heartbeat monitor goroutine checks every 5 seconds
- If `time.Since(LastHeartbeat) > 35 seconds`, triggers disconnection
- Connection closed and state transitions to `Reconnecting`

### Reconnection Flow

**Trigger Conditions**:
- Network errors during read/write operations
- Heartbeat timeout (no pong received within 35 seconds)
- WebSocket close frame from control plane
- TLS handshake failures

**Reconnection Logic**:
1. Detect disconnection and transition state to `Reconnecting`
2. Calculate delay using exponential backoff:
   - Initial delay: 1 second
   - Multiplier: 2x on each attempt
   - Jitter: ±25% randomization
   - Maximum delay: 5 minutes
3. Sleep for calculated delay
4. Attempt WebSocket connection with same token
5. On success: reset backoff and transition to `Connected`
6. On failure: increment backoff and retry
7. After 60 seconds of stable connection, reset backoff to initial delay

### Graceful Shutdown

1. Application receives SIGTERM or SIGINT signal
2. Shutdown handler calls `client.Close()`
3. Client sends WebSocket close frame with code 1000 (normal closure)
4. Heartbeat monitor goroutine exits via context cancellation
5. Main process exits with code 0

## Architecture

```
┌────────────────────────────────────────────┐
│         Control Plane                      │
│        (Platform API)                      │
│                                            │
│  WebSocket: /ws/gateways/connect           │
└──────────┬─────────────────────────────────┘
           │ WSS
           │ api-key auth
           │ ping/pong
           │
┌────────────────────────────────────────────┐
│       Gateway Controller                   │
│                                            │
│  ┌──────────────────────────────────────┐  │
│  │   Control Plane Client               │  │
│  │                                      │  │
│  │  • Connection Manager                │  │
│  │  • Heartbeat Monitor                 │  │
│  └──────────────────────────────────────┘  │
│                                            │
│  ┌──────────────────────────────────────┐  │
│  │   Gateway Core                       │  │
│  │   (Traffic Processing)               │  │
│  └──────────────────────────────────────┘  │
└────────────────────────────────────────────┘
```

**Component Responsibilities**:

- **Connection Manager**: WebSocket lifecycle, state transitions, heartbeat monitoring
- **Reconnection Manager**: Exponential backoff calculation, retry logic
- **Heartbeat Monitor**: Periodic ping/pong check, timeout detection

## Implementation Details

### Connection State Machine

**States**:
- `Disconnected` (0): Initial state
- `Connecting` (1): WebSocket handshake in progress
- `Connected` (2): Active connection with heartbeat
- `Reconnecting` (3): Attempting to reconnect after failure

**State Transitions**:
- `Disconnected` → `Connecting`: Initial connection attempt
- `Connecting` → `Connected`: Successful WebSocket handshake and ACK received
- `Connecting` → `Reconnecting`: Connection attempt failed
- `Connected` → `Reconnecting`: Heartbeat timeout or network error
- `Reconnecting` → `Connected`: Successful reconnection

### Exponential Backoff with Jitter

**Algorithm**:
- Base delay: Initial delay × 2^attempt
- Cap at maximum delay (5 minutes default)
- Add jitter: ±25% randomization
- Reset to initial delay after 60 seconds of stable connection

**Design Rationale**:
- Exponential growth prevents connection storms during widespread outages
- Maximum cap prevents infinite delay growth
- Jitter prevents thundering herd when multiple gateways reconnect simultaneously
- Randomization spreads load over time window

## Key Technical Decisions

1. **gorilla/websocket**: Same library as control plane for protocol compatibility, battle-tested implementation

2. **Exponential backoff with jitter**: Prevents connection storms during outages, 1s → 5min cap

3. **Multiple instances share token**: Simplifies horizontal scaling, independent connections per instance

4. **Background connection goroutine**: Gateway continues processing traffic during control plane outages

5. **Per-connection heartbeat goroutine**: Isolates failures, enables graceful shutdown

6. **Atomic state management**: Lock-free state transitions using atomic operations

## Configuration

**Environment Variables**:

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `GATEWAY_REGISTRATION_TOKEN` | string | (required) | Registration token from control plane |
| `GATEWAY_CONTROL_PLANE_URL` | string | `wss://localhost:8443/api/internal/v1/ws/gateways/connect` | Control plane WebSocket endpoint |
| `GATEWAY_RECONNECT_INITIAL` | duration | `1s` | Initial reconnection delay |
| `GATEWAY_RECONNECT_MAX` | duration | `5m` | Maximum reconnection delay |

**Validation Rules**:
- Token: Non-empty string (required)
- URL: Must use `wss://` protocol
- ReconnectInitial: Positive duration
- ReconnectMax: Greater than ReconnectInitial

**Docker Compose Example**:

```yaml
services:
  gateway:
    image: wso2/api-gateway:latest
    environment:
      - GATEWAY_REGISTRATION_TOKEN=wE7a1WnvvZloxa-TZbL7QhXDjTCIOsyuNg9oKhtX3cU
      - GATEWAY_CONTROL_PLANE_URL=wss://platform-api:8443/api/internal/v1/ws/gateways/connect
      - GATEWAY_RECONNECT_INITIAL=2s
      - GATEWAY_RECONNECT_MAX=10m
    depends_on:
      - platform-api
```

# FR: Gateway WebSocket Event Notification System

## Requirement

Enables real-time bidirectional communication between the platform and gateway instances via WebSocket protocol for event notifications and connection management, supporting persistent connections that broadcast API deployment events and other operational notifications to connected gateways.

## User Scenarios

### Gateway Connection Management
- Gateways establish persistent WebSocket connections to platform at `wss://platform-api:9243/api/internal/v1/ws/gateways/connect`
- Platform maintains in-memory connection registry supporting multiple connections per gateway ID for clustering
- Heartbeat mechanism with 20-second ping and 30-second timeout detects ungraceful disconnections
- Authentication via API key validates gateways before connection upgrade
- Rate limiting (10 attempts/minute/IP) and maximum connection limits (1000 default) prevent resource exhaustion

### Event Broadcasting
- API deployment triggers event broadcast to target gateway connections with API UUID, revision ID, vhost, and environment
- Events serialized as JSON with type identifier, payload, timestamp, and correlation ID for tracing
- Sequential delivery per connection preserves event ordering within gateway
- Payload size validation (1MB maximum) prevents memory exhaustion
- Delivery statistics tracked atomically per connection (total sent, failed deliveries, last failure time/reason)

### Administrative Visibility
- Stats API endpoint returns active connection count, connection details (gateway ID, connection ID, timestamps, heartbeat status), and delivery statistics
- Connection events logged at INFO level, authentication failures at WARN, delivery failures at ERROR
- Failed deliveries logged with correlation ID but do not fail deployment operations

## Key Entities

- **Gateway Connection**: Active WebSocket connection with gateway ID, connection ID, timestamps, transport handle, auth token, and delivery statistics
- **Connection Registry**: Thread-safe sync.Map mapping gateway IDs to connection lists for fast event delivery lookup
- **Gateway Event**: Notification with type, payload, timestamp, and correlation ID sent via WebSocket to gateways
- **Delivery Stats**: Atomic counters tracking successful/failed deliveries and last failure details per connection
- **Transport Interface**: Protocol abstraction decoupling WebSocket from business logic enabling future protocol changes

## Implementation

[Gateway WebSocket Event Notification](../impls/gateway-websocket-events/gateway-websocket-events.md)

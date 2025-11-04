# Feature Specification: Gateway to Control Plane Registration

**Feature Branch**: `003-gateway-registration`
**Created**: October 21, 2025
**Status**: Draft
**Input**: User description: "I need to implement gateway to control plane (platform-api) registration feature - /home/malintha/wso2apim/gitworkspace/api-platform/platform-api/spec/impls/gateway-management/gateway-management.md. When a gateway is added from platform-api, it can also generate a token, then the token can be configured in the gateway via a config in docker compose as an env variable. Then when the gateway starts up, it should create a websocket connection to the platform api (again the hostname might be configurable with a default value - wss://localhost:8443. Full path is wss://localhost:8443/api/internal/v1/ws/gateways/connect. When do the ws call, it has to send the configured token via api-key header. ex: "api-key: wE7a1WnvvZloxa-TZbL7QhXDjTCIOsyuNg9oKhtX3cU". Then the gateway should always keep a persistent connection to get the events."

## Clarifications

### Session 2025-10-22

- Q: When a gateway's registration token is revoked or expires while the gateway is actively connected, what should happen? → A: Control plane immediately disconnects the gateway; gateway logs error and stops reconnection attempts
- Q: What observability signals (metrics/monitoring) should the gateway expose for connection health and operational monitoring? → A: Connection state metrics + error counters (failed auth, network errors, reconnection attempts)
- Q: Should multiple gateway instances be allowed to connect using the same registration token? → A: Multiple instances allowed; each maintains independent WebSocket connection and receives all events
- Q: When the control plane undergoes planned maintenance or upgrades, how should gateway connections behave? → A: Gateway treats it as normal disconnection and uses standard exponential backoff retry logic
- Q: When the gateway is temporarily disconnected and reconnects, what happens to events/messages that were sent by the control plane during the disconnection period? → A: WebSocket messages sent during disconnection are lost (no delivery guarantees). Gateway implements reconciliation for eventual consistency: (1) Gateway persists last successful event timestamp per event type in database, (2) Upon reconnection, gateway requests updates since last timestamp for each event type, (3) Control plane responds with delta updates, (4) Gateway applies updates using timestamp-based idempotent processing with deduplication to handle both real-time events and polling responses safely, (5) Gateway also polls every 15 minutes as backup even when connected

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configuration Management (Priority: P1)

An operator configures gateway connection parameters through environment variables or configuration files, allowing flexible deployment across different environments (development, staging, production).

**Why this priority**: This is the foundational prerequisite for all other functionality. Without the ability to configure the registration token and control plane URL, the gateway cannot connect or register. This must be implemented first as all subsequent stories depend on it.

**Independent Test**: Can be tested by deploying gateway with different configuration values (environment variables or config files) and verifying values are correctly loaded and validated at startup. Delivers immediate value by enabling environment-specific deployments.

**Acceptance Scenarios**:

1. **Given** gateway deployment configuration, **When** no control plane URL is provided, **Then** gateway uses default "wss://localhost:8443/api/internal/v1/ws/gateways/connect"
2. **Given** an environment variable specifying custom control plane URL, **When** gateway starts, **Then** it loads and validates the specified URL
3. **Given** a registration token provided via environment variable, **When** gateway initializes, **Then** it loads the token for subsequent authentication
4. **Given** missing or invalid configuration, **When** gateway starts, **Then** it logs clear error messages indicating which configuration is missing or malformed

---

### User Story 2 - Gateway Startup Registration (Priority: P1)

An operator deploys a new gateway instance with a pre-configured registration token. When the gateway starts up, it automatically establishes a persistent connection to the control plane without manual intervention.

**Why this priority**: This is the core functionality that enables all subsequent communication between gateway and control plane. Without automatic registration, no events can be received and the gateway cannot be managed centrally. Depends on Story 1 for configuration capability.

**Independent Test**: Can be fully tested by starting a gateway with valid token configuration and verifying successful WebSocket connection to control plane. Delivers immediate value by establishing the communication channel for gateway management.

**Acceptance Scenarios**:

1. **Given** a gateway instance with valid control plane URL and registration token configured, **When** the gateway starts up, **Then** it establishes a WebSocket connection to the control plane within 5 seconds
2. **Given** a gateway successfully connected to control plane, **When** the connection is verified, **Then** the gateway appears as "online" in the control plane's gateway list
3. **Given** a gateway attempting to connect, **When** the registration token is sent via the api-key header, **Then** the control plane authenticates the gateway and accepts the connection

---

### User Story 3 - Persistent Connection Maintenance (Priority: P1)

After establishing the initial connection, the gateway maintains a persistent WebSocket connection to receive real-time events and configuration updates from the control plane.

**Why this priority**: Essential for real-time event delivery and responsive gateway management. Without persistence, the gateway cannot receive timely updates or commands from the control plane. Depends on Story 2 for initial connection establishment.

**Independent Test**: Can be tested by establishing connection and monitoring for automatic reconnection after network disruptions. Delivers continuous connectivity required for event-driven architecture.

**Acceptance Scenarios**:

1. **Given** an active WebSocket connection, **When** the connection is idle for extended periods, **Then** the gateway maintains the connection without timeout
2. **Given** a network interruption occurs, **When** connectivity is restored, **Then** the gateway automatically reconnects to the control plane within 30 seconds
3. **Given** the gateway receives events from control plane, **When** events are delivered via WebSocket, **Then** the gateway processes them without connection interruption

---

### User Story 4 - Connection Failure Handling (Priority: P2)

When the gateway cannot connect to the control plane (invalid token, unreachable endpoint, or authentication failure), it provides clear feedback and implements retry logic to handle transient failures.

**Why this priority**: Critical for operational reliability but secondary to establishing basic connectivity. Ensures gateway doesn't fail permanently due to temporary issues.

**Independent Test**: Can be tested by simulating various failure scenarios (network unavailable, invalid token, control plane down) and verifying retry behavior and error logging.

**Acceptance Scenarios**:

1. **Given** an invalid registration token, **When** gateway attempts connection, **Then** authentication fails with clear error message and gateway logs the failure
2. **Given** control plane is temporarily unreachable, **When** initial connection fails, **Then** gateway retries with exponential backoff up to 5 minutes
3. **Given** repeated connection failures, **When** all retries are exhausted, **Then** gateway continues running in degraded mode and attempts reconnection periodically

---

### Edge Cases

- When a registration token is revoked or expires while gateway is running, the control plane immediately disconnects the gateway, and the gateway logs the error and stops all reconnection attempts (requires manual intervention with new token)
- During control plane maintenance or upgrades, gateway treats disconnection as normal network failure and automatically reconnects using standard exponential backoff retry logic (FR-008)
- Multiple gateway instances may use the same registration token for horizontal scaling; each instance maintains an independent WebSocket connection and receives all events broadcast by the control plane
- WebSocket messages sent during disconnection are lost; gateway implements reconciliation by persisting last event timestamp per event type, requesting delta updates upon reconnection, and polling every 15 minutes; uses timestamp-based idempotent processing to safely handle concurrent updates from both channels
- What happens when network partitions cause extended disconnection periods?
- How does the gateway behave if configured control plane URL is malformed or points to wrong endpoint?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Gateway MUST read control plane connection configuration from environment variables at startup
- **FR-002**: Gateway MUST support configurable control plane URL with default value "wss://localhost:8443/api/internal/v1/ws/gateways/connect"
- **FR-003**: Gateway MUST read registration token from environment variable for authentication
- **FR-004**: Gateway MUST establish WebSocket connection to control plane during startup initialization
- **FR-005**: Gateway MUST send registration token via "api-key" HTTP header when establishing WebSocket connection
- **FR-006**: Gateway MUST maintain persistent WebSocket connection throughout its lifecycle
- **FR-007**: Gateway MUST automatically reconnect to control plane if connection is lost due to network issues
- **FR-008**: Gateway MUST implement exponential backoff retry strategy for failed connection attempts, starting at 1 second and capping at 5 minutes
- **FR-009**: Gateway MUST log all connection state changes (connecting, connected, disconnected, reconnecting) with appropriate severity levels
- **FR-010**: Gateway MUST continue processing requests in degraded mode when control plane connection is unavailable
- **FR-011**: Gateway MUST validate control plane URL format before attempting connection
- **FR-012**: Gateway MUST handle WebSocket ping/pong frames to maintain connection health
- **FR-013**: Gateway MUST gracefully close WebSocket connection during shutdown sequence
- **FR-014**: Gateway MUST provide connection status through health check endpoint
- **FR-015**: Gateway MUST stop all reconnection attempts when control plane disconnects due to token revocation or expiration, logging the authentication failure with ERROR severity
- **FR-016**: Gateway MUST expose metrics for connection state (connected/disconnected/reconnecting status)
- **FR-017**: Gateway MUST expose error counter metrics for authentication failures, network errors, and reconnection attempts
- **FR-018**: Gateway MUST support multiple instances connecting with the same registration token, each maintaining an independent connection
- **FR-019**: Gateway MUST persist last successful event timestamp for each event type (API deployment, application creation, subscription, etc.) in local database
- **FR-020**: Gateway MUST request delta updates from control plane upon reconnection, sending last known timestamps for each event type
- **FR-021**: Gateway MUST poll control plane for updates every 15 minutes even when WebSocket connection is active
- **FR-022**: Gateway MUST process all updates (from WebSocket events and polling) using timestamp-based idempotent logic with deduplication to prevent inconsistencies from concurrent update channels

### Key Entities

- **Gateway Instance**: Represents a running gateway instance with unique registration token, connection state, and control plane endpoint configuration
- **Registration Token**: Security credential generated by control plane during gateway registration, used for authenticating WebSocket connections
- **WebSocket Connection**: Persistent bidirectional communication channel between gateway and control plane for event delivery and command execution
- **Connection Configuration**: Set of parameters including control plane URL, registration token, retry policies, and connection timeout values
- **Event Timestamp**: Per-event-type timestamp tracking last successfully processed event, persisted in gateway database for reconciliation after disconnection

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Gateway establishes WebSocket connection to control plane within 5 seconds of startup when control plane is available
- **SC-002**: Gateway automatically reconnects within 30 seconds after network disruption is resolved
- **SC-003**: Gateway maintains persistent connection for extended periods (24+ hours) without manual intervention
- **SC-004**: Connection failures are logged with sufficient detail to diagnose issues within 5 minutes of occurrence
- **SC-005**: Gateway continues processing traffic during control plane outages (99%+ request success rate)
- **SC-006**: 95% of deployment configurations succeed on first attempt without connection errors

## Assumptions

- Control plane exposes WebSocket endpoint at documented path and accepts api-key header authentication
- Registration tokens are generated and managed through existing gateway management API (as documented in gateway-management.md)
- Docker Compose or equivalent container orchestration is used for gateway deployment
- Gateway has network connectivity to control plane endpoint
- TLS/SSL certificates are properly configured for wss:// connections
- Control plane implements standard WebSocket protocol with ping/pong keepalive
- Gateway can operate in degraded mode without control plane connectivity for local traffic routing

## Dependencies

- Control plane must have WebSocket endpoint `/api/internal/v1/ws/gateways/connect` implemented and operational
- Control plane must provide reconciliation endpoint for delta updates based on timestamp queries per event type
- Gateway management API must support token generation during gateway registration
- Network infrastructure must allow WebSocket connections from gateway to control plane
- Environment variable configuration mechanism must be available in deployment platform
- Gateway must have local database for persisting event timestamps

## Out of Scope

- Implementation of specific event types or commands sent over WebSocket connection (event schema and handlers are separate features)
- Gateway management UI for viewing connection status
- Token rotation or renewal while gateway is running
- Multi-region control plane failover or load balancing
- Authentication methods other than api-key header
- Control plane implementation details or WebSocket server logic
- Detailed reconciliation endpoint API specification (covered in control plane feature spec)
# Feature Specification: Gateway Event Notification System

## Clarifications

- Q: Should the platform automatically resend missed events, require the gateway to request sync, or treat reconnection as a clean slate? → A: Treat reconnection as a clean slate (no automatic resend). Platform logs missed events but takes no action. Gateway is responsible for detecting drift. Note: Gateway-initiated sync capability may be added in future iterations.
- Q: Should the platform allow multiple connections per gateway ID (treating them as separate instances) or enforce unique connections? → A: Allow multiple connections per gateway ID to support gateway clustering or multiple instances. Events are broadcast to all connections with the same gateway ID.
- Q: What is the expected peak event throughput? → A: 100 events per second across all gateways. Note: This is not a critical performance requirement currently, but provides a reasonable target for efficient implementation.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Gateway Establishes Persistent Connection (Priority: P1)

As a gateway instance, I need to establish a persistent connection to the platform and have it maintained reliably so that I can receive real-time events when they occur.

**Why this priority**: This is the foundational requirement. Without establishing and maintaining the connection, no events can be delivered. Everything else depends on this working.

**Independent Test**: Can be fully tested by starting a gateway instance and verifying it successfully connects to the platform. Success is measured by the platform acknowledging the connection and registering the gateway in its connection registry.

**Acceptance Scenarios**:

1. **Given** a gateway instance is configured with platform connection details and credentials, **When** the gateway initiates a connection, **Then** the platform accepts the connection and registers the gateway in its active connection registry within 10 seconds
2. **Given** a gateway attempts to connect with valid credentials, **When** the connection handshake completes, **Then** the platform returns a successful connection acknowledgment with the gateway's registered identifier
3. **Given** a gateway connection is established, **When** no activity occurs for an extended period, **Then** the connection remains active through periodic heartbeat/keepalive mechanisms
4. **Given** a gateway has an active connection, **When** a network interruption occurs, **Then** the gateway automatically attempts to reconnect using exponential backoff (starting at 1 second, maximum 60 seconds)
5. **Given** a gateway attempts to connect with invalid or missing credentials, **When** authentication is evaluated, **Then** the connection is rejected with a clear authentication error message

---

### User Story 2 - Platform Sends API Deployment Notification via Established Connection (Priority: P1)

As the platform, when I receive an API deployment call, I need to send the API deployment notification to the target gateway via its established connection so that the gateway can immediately apply the new configuration.

**Why this priority**: This is the core use case that drives the entire feature. The platform must be able to leverage established gateway connections to push deployment events in real-time.

**Independent Test**: Can be fully tested by having a gateway connected and then triggering an API deployment action. Success is measured by the gateway receiving the deployment event with complete API configuration details within 5 seconds.

**Acceptance Scenarios**:

1. **Given** a gateway has an established connection to the platform, **When** an administrator calls the API deployment endpoint (`POST /api/v1/apis/:api_uuid/deploy-revision`) targeting that gateway, **Then** the platform sends a deployment event to the gateway via the established connection
2. **Given** a deployment event is sent to a connected gateway, **When** the event is delivered, **Then** the event payload includes the API UUID, revision details, and deployment configuration (vhost) but excludes the API YAML (gateway retrieves this separately via an internal API endpoint)
3. **Given** multiple gateways are connected to the platform, **When** an API deployment targets a specific gateway, **Then** only that gateway receives the deployment notification (not all connected gateways)
4. **Given** an API deployment call is made, **When** the target gateway is not currently connected, **Then** the platform logs a delivery failure with the gateway identifier and event details
5. **Given** multiple API deployments occur in sequence for the same gateway, **When** events are sent, **Then** the gateway receives them in the same order they were triggered

---

### User Story 3 - Platform Tracks Connected Gateways (Priority: P1)

As a platform administrator, I need to see which gateways are currently connected so that I can verify event delivery is possible and troubleshoot connectivity issues.

**Why this priority**: Without visibility into which gateways are connected, operators cannot determine if deployment events will be delivered successfully. This is essential for operations and diagnostics.

**Independent Test**: Can be fully tested by connecting/disconnecting gateways and querying the platform for connection status. Success includes accurate real-time status showing which gateways can receive events.

**Acceptance Scenarios**:

1. **Given** multiple gateways have connected to the platform, **When** an administrator queries the gateway connection status, **Then** the platform returns a list of all currently connected gateways with their identifiers and connection timestamps
2. **Given** a gateway disconnects (gracefully or ungracefully), **When** the disconnection is detected, **Then** the platform immediately removes the gateway from the active connection registry and logs the disconnection event
3. **Given** a gateway has been connected, **When** querying connection details, **Then** the platform provides metadata including connection duration, last heartbeat time, gateway identifier, and connection health status
4. **Given** an administrator needs to verify event delivery capability, **When** checking if a gateway can receive events, **Then** the platform indicates whether that gateway has an active connection

---

### User Story 4 - Multiple Event Types Support (Priority: P2)

As a platform operator, I need to send different types of events to gateways (deployment, undeployment, configuration changes, policy updates) so that the system can support various operational scenarios beyond just API deployment.

**Why this priority**: While API deployment is the immediate use case, designing for extensibility prevents future rework. The system should support multiple event types from the start.

**Independent Test**: Can be fully tested by defining multiple event types and sending them to a connected gateway. Success includes the gateway receiving events with distinct type identifiers and being able to handle them appropriately.

**Acceptance Scenarios**:

1. **Given** a gateway is connected, **When** the platform sends different event types (e.g., "api.deployed", "api.undeployed", "gateway.config.updated"), **Then** the gateway receives each event with the correct event type identifier
2. **Given** new event types need to be added to the system, **When** a developer defines a new event type, **Then** the implementation requires only registering the new type without modifying core connection or transport logic
3. **Given** a gateway receives an event type it doesn't recognize, **When** processing the event, **Then** the gateway can safely ignore or log the unknown event type without connection failure
4. **Given** events of different types are sent to the same gateway, **When** delivered, **Then** each event maintains its type-specific payload structure

---

### User Story 5 - Failed Event Delivery Handling (Priority: P2)

As a platform operator, when an event cannot be delivered to a gateway (because it's disconnected or delivery fails), I need to know about the failure and understand what happened so that I can take corrective action.

**Why this priority**: Real-world systems experience failures. Without failure handling and visibility, events may be silently lost, leading to configuration drift between platform and gateways.

**Independent Test**: Can be fully tested by simulating various failure scenarios (disconnected gateway, simulated delivery failure) and verifying the platform's logging and response. Success includes detailed failure logs accessible to administrators.

**Acceptance Scenarios**:

1. **Given** an API deployment event needs to be sent to a gateway, **When** the target gateway is not currently connected, **Then** the platform logs the delivery failure with event type, gateway identifier, timestamp, and reason
2. **Given** an event delivery to a connected gateway fails, **When** the failure occurs, **Then** the platform logs the failure with full error details and marks the delivery as failed
3. **Given** event deliveries have failed for a gateway, **When** an administrator queries failed deliveries, **Then** the platform returns a list of failed events within a specified time window (e.g., last 24 hours)
4. **Given** a gateway was offline during an event, **When** the gateway reconnects, **Then** the platform treats the reconnection as a clean slate without automatically resending missed events (gateway is responsible for detecting configuration drift)

---

### User Story 6 - Gateway Authentication and Authorization (Priority: P2)

As a security administrator, I need gateways to authenticate when establishing connections so that only authorized gateways can receive events and no unauthorized systems can connect to the platform.

**Why this priority**: Security is critical in production environments. Without proper authentication, any client could connect and receive sensitive deployment information or exhaust connection resources.

**Independent Test**: Can be fully tested by attempting connections with valid credentials, invalid credentials, expired credentials, and no credentials. Success includes accepting only valid connections and rejecting all others with appropriate error messages.

**Acceptance Scenarios**:

1. **Given** a gateway attempts to connect to the platform, **When** the gateway provides valid authentication credentials (token or certificate), **Then** the connection is established and associated with the authenticated gateway identity
2. **Given** a gateway attempts to connect without credentials or with invalid credentials, **When** authentication is evaluated, **Then** the connection is rejected immediately with a specific authentication error code
3. **Given** an established gateway connection exists, **When** the authentication token expires or is revoked, **Then** the platform terminates the connection and requires the gateway to re-authenticate
4. **Given** a gateway identity's credentials are revoked by an administrator, **When** the revocation occurs, **Then** any active connections for that gateway are immediately terminated

---

### User Story 7 - Transport Abstraction for Future Flexibility (Priority: P3)

As a platform architect, I need the event delivery system to use an abstraction layer for the transport mechanism so that we can replace WebSocket with another protocol in the future without rewriting business logic.

**Why this priority**: The user explicitly mentioned that WebSocket might change in the future. While not immediately needed, this architectural quality ensures long-term maintainability.

**Independent Test**: Can be tested by verifying that business logic (event routing, connection tracking, authentication) is separated from transport-specific code through well-defined interfaces. Success includes the ability to implement a mock transport for testing.

**Acceptance Scenarios**:

1. **Given** the system is designed with transport abstraction, **When** a developer needs to understand event sending logic, **Then** the business logic code contains no direct references to WebSocket-specific APIs
2. **Given** a new transport mechanism needs to be supported, **When** implementing it, **Then** the developer only needs to implement the transport interface without modifying event routing or connection management logic
3. **Given** the system uses the transport abstraction, **When** running tests, **Then** developers can use a mock transport implementation to test business logic without starting actual network connections

---

### Edge Cases

- What happens when the platform restarts while gateways are connected? (All gateway connections are dropped; gateways detect disconnection and automatically reconnect)
- What happens when a gateway becomes unresponsive but the connection remains open? (Platform detects via heartbeat timeout, marks connection as stale, closes socket)
- What happens when network latency causes event delivery delays? (Events are still delivered in order with timestamps; gateways can detect stale events)
- What happens if an event payload exceeds size limits? (System enforces maximum payload size, rejects oversized events with error)
- What happens when two gateway instances connect with the same gateway identifier? (Platform allows multiple connections per gateway ID to support clustering. Events are broadcast to all connections with the same ID)
- What happens when connection limits are reached? (Platform enforces maximum concurrent connections and rejects new connections with a "capacity exceeded" error)
- What happens when a malicious client attempts rapid connection/disconnection? (Platform implements rate limiting on connection attempts per IP address)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The platform MUST provide a WebSocket connection endpoint at `wss://localhost:9243/api/internal/v1/ws/gateways/connect` that gateways can connect to for receiving real-time event notifications
- **FR-002**: The platform MUST authenticate gateway connections and reject unauthorized connection attempts with clear error messages
- **FR-003**: The platform MUST maintain an in-memory registry of all currently connected gateways, indexed by gateway identifier, enabling fast lookup for event delivery (supporting multiple connections per gateway ID for clustering)
- **FR-004**: When a gateway establishes a connection successfully, the platform MUST store the connection handle associated with the gateway identifier in the connection registry
- **FR-005**: When a gateway disconnects, the platform MUST immediately remove the gateway from the connection registry and log the disconnection event
- **FR-006**: The platform MUST detect ungraceful disconnections within 30 seconds using heartbeat/keepalive mechanisms
- **FR-007**: When an API deployment action occurs (via `POST /api/v1/apis/:api_uuid/deploy-revision`), the platform MUST look up the target gateway's connections from the registry and broadcast the deployment event to all active connections for that gateway ID
- **FR-008**: API deployment events MUST include the API UUID, revision details, and deployment configuration (vhost) but MUST NOT include the API YAML (which the gateway retrieves separately)
- **FR-009**: When sending an event to a gateway, if the gateway is not currently connected, the platform MUST log a delivery failure with the gateway identifier, event type, and timestamp
- **FR-010**: The platform MUST support multiple event types (e.g., "api.deployed", "api.undeployed", "gateway.config.updated") with each event having a type identifier
- **FR-011**: The system MUST provide an abstraction mechanism allowing new event types to be registered and handled without modifying core connection management code
- **FR-012**: The connection and event delivery mechanism MUST use an abstraction layer (interface/contract) that isolates transport-specific logic from business logic
- **FR-013**: The transport abstraction MUST support pluggable implementations, allowing WebSocket to be replaced with alternative protocols without changing event routing logic
- **FR-014**: The platform MUST maintain event delivery order per gateway - if Event A is sent before Event B to the same gateway, Event A must be delivered before Event B
- **FR-015**: The platform MUST log all connection events including successful connections, disconnections, and authentication failures with timestamps and gateway identifiers
- **FR-016**: The platform MUST log all event delivery attempts including successes and failures with event type, target gateway, timestamp, and outcome
- **FR-017**: The platform MUST provide an administrative API endpoint to query the list of currently connected gateways
- **FR-018**: The gateway connection status API MUST return gateway identifier, connection timestamp, last activity/heartbeat time, and connection health status for each connected gateway
- **FR-019**: The platform MUST provide an administrative API endpoint to query failed event deliveries for a specific gateway within a time window
- **FR-020**: The platform MUST enforce a maximum concurrent connection limit (configurable, default 1000) to prevent resource exhaustion
- **FR-021**: The platform MUST enforce a maximum event payload size (configurable, default 1MB) and reject events that exceed this limit
- **FR-022**: Gateway connections MUST use encrypted transport (TLS/SSL required)
- **FR-023**: The platform MUST implement rate limiting on connection attempts (configurable, default 10 attempts per minute per IP address) to prevent denial-of-service attacks
- **FR-024**: When a gateway connection is established, the platform MUST send a connection acknowledgment message to the gateway confirming successful registration

### Key Entities

- **Gateway Connection**: Represents an active persistent connection from a gateway instance to the platform. Key attributes include gateway identifier, connection timestamp, last activity/heartbeat timestamp, authentication status, connection health status, and connection handle/reference.

- **Connection Registry**: An in-memory data structure that maps gateway identifiers to lists of active connection handles, enabling quick lookup when events need to be broadcast to all instances of a gateway (supports multiple connections per gateway ID for clustering scenarios).

- **Gateway Event**: Represents a notification to be sent to a gateway. Key attributes include event type (e.g., "api.deployed"), event payload (event-specific data), target gateway identifier, creation timestamp, and correlation ID.

- **Event Type**: Represents a category of events that can be sent to gateways. Key attributes include event type identifier (e.g., "api.deployed"), event schema/structure description, and registered handler.

- **Delivery Log Entry**: Represents a record of an event delivery attempt. Key attributes include event type, target gateway identifier, timestamp, delivery status (success/failed/not_connected), error details if failed, and correlation ID.

- **Transport Abstraction Interface**: A contract defining operations for managing connections and sending messages, independent of the underlying protocol (WebSocket, Server-Sent Events, gRPC, etc.). Key operations include accept connection, send message, close connection, and check connection health.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Gateway connection establishment completes within 10 seconds from connection initiation to successful registration in the platform's connection registry
- **SC-002**: The system maintains stable connections with at least 100 concurrent gateways without connection drops or memory leaks over a 24-hour period
- **SC-003**: When an API deployment action occurs, the target gateway receives the deployment event within 5 seconds
- **SC-004**: After a network interruption, gateways successfully reconnect automatically within 2 minutes without manual intervention
- **SC-005**: Administrative queries for connected gateway status return complete results within 2 seconds even with 100 connected gateways
- **SC-006**: The system correctly delivers events in order - 100% of sequential events to the same gateway arrive in the order sent
- **SC-007**: The system prevents unauthorized access - 100% of connection attempts without valid credentials are rejected within 2 seconds
- **SC-008**: Failed event delivery attempts are logged and queryable by administrators within 1 minute of the failure occurring
- **SC-009**: The transport abstraction adds less than 5% performance overhead compared to direct protocol usage
- **SC-010**: The system supports adding new event types - a developer can define and use a new event type in under 30 minutes without modifying core connection logic

## Assumptions

1. **Gateway Identity**: Each gateway has a unique identifier (UUID or similar) that is configured during gateway setup and used for authentication and connection registration.

2. **WebSocket as Initial Transport**: While the system must be transport-agnostic, WebSocket will be the initial implementation for persistent connections.

3. **Network Reliability**: The system assumes reasonable network stability. While reconnection logic handles temporary failures, it's not designed for environments with constant connectivity issues.

4. **Gateway Deployment Model**: Gateways are deployed as separate instances/services, not embedded within the platform process.

5. **Authentication Mechanism**: The system assumes token-based or certificate-based authentication. The specific mechanism (JWT, API key, mTLS) is an implementation detail.

6. **Single Platform Instance**: Initially, the system assumes a single platform instance. Multi-instance deployment with shared connection state (for high availability) is out of scope for P1.

7. **Event Persistence**: This specification focuses on real-time event delivery to connected gateways. Long-term event persistence (event sourcing, audit log) is handled by separate logging infrastructure.

8. **Scalability Target**: Initial target is 100-1000 concurrent gateway connections. Larger scale (10,000+) may require architectural enhancements (distributed connection registry).

9. **Event Size**: Events are lightweight notifications containing references (API UUID, revision) rather than full API YAML payloads. Most events are very small (< 10KB). The gateway fetches complete API details separately via the API runtime artifacts endpoint.

10. **Clock Synchronization**: Platform and gateways have reasonably synchronized clocks (within a few seconds) for timestamp-based operations and event ordering.

11. **Gateway Reconnection Logic**: Gateways implement their own reconnection logic with exponential backoff. The platform only needs to accept reconnections.

12. **Missed Events on Reconnect**: When a gateway reconnects after being offline, the platform treats it as a clean slate and does not automatically resend missed events. The gateway is responsible for detecting configuration drift and taking corrective action. Future iterations may add gateway-initiated sync capability.

## Dependencies

- **API Lifecycle Management**: This feature depends on the existing API deployment functionality described in `spec/impls/api-lifecycle-management.md`. The deployment service must be modified to trigger event notifications after successful deployment operations.

- **Gateway Registration System**: There must be a system for registering gateway instances, generating gateway identifiers, and managing gateway credentials. If this doesn't exist, it must be created.

- **Authentication System**: The platform must have an authentication mechanism that can validate gateway credentials (tokens, certificates, or API keys).

- **Logging Infrastructure**: The platform must have structured logging capability to capture connection events and delivery attempts.

- **API Runtime Artifacts Endpoint**: The gateway relies on an internal API endpoint (assumed to be `GET /api/internal/v1/api-runtime-artifacts/{apiId}`) to retrieve the API YAML after receiving a deployment event. Implementation of this endpoint is out of scope for this feature but must exist or be developed separately.

## Security Considerations

- **Encrypted Transport**: All gateway connections must use TLS/SSL encryption to protect event data in transit
- **Gateway Authentication**: Strong authentication required - credentials must be securely generated, distributed, and stored
- **Credential Rotation**: Gateway credentials should support rotation without service disruption
- **Access Control**: Event payloads may contain sensitive API configuration - only authenticated gateways should receive events
- **DoS Protection**: Connection rate limiting and maximum connection limits prevent resource exhaustion attacks
- **Audit Logging**: All connection attempts, authentication failures, and event deliveries must be logged for security monitoring
- **Credential Revocation**: Platform must support immediate credential revocation with active connection termination
- **IP Whitelisting**: Optional capability to restrict gateway connections to known IP ranges

## Constraints

- **Message Throughput**: The system must support up to 100 events per second across all gateways under peak load conditions (not a critical hard requirement but a reasonable performance target)
- **Low Latency Requirement**: The system must deliver events within 5 seconds under normal conditions
- **Memory Efficiency**: Connection registry must be memory-efficient for 100-1000 concurrent connections (target < 10MB overhead)
- **Transport Agnostic Design**: The abstraction layer must not expose protocol-specific details in business logic
- **Connection Limits**: System must enforce maximum concurrent connections to prevent resource exhaustion (configurable limit)

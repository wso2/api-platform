# Feature Specification: Gateway with Controller and Router

**Feature Branch**: `001-gateway-has-two`
**Created**: 2025-10-11
**Status**: Draft
**Input**: User description: "Gateway has two components, Gateway-Controller and Router. Router is an Envoy Proxy based Docker image with the required bootstrap envoy.yaml with xds_cluster. The Gateway-Controller is the xds server for Router. The user will provide API configurations to the Gateway-Controller and it will configure the Router according to the user provided configurations. This API configuration is a YAML/JSON definition of RestAPI. User can delete/edit API configurations via Gateway-Controller and it should be reflected in the Router. Use gateway/ directory."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Deploy New API Configuration (Priority: P1)

An API administrator needs to expose a new REST API through the gateway by providing a configuration that defines the API's routing rules, endpoints, and policies. They submit this configuration to the Gateway-Controller, which immediately configures the Router to start accepting traffic for the new API.

**Why this priority**: This is the core value proposition of the gateway - enabling users to expose APIs without manually configuring infrastructure. Without this, the gateway has no purpose.

**Independent Test**: Can be fully tested by submitting a single API configuration file to the Gateway-Controller and verifying that HTTP requests to the Router are successfully routed to the backend service. Delivers immediate value by making an API accessible through the gateway.

**Acceptance Scenarios**:

1. **Given** the Gateway-Controller and Router are running, **When** a user submits a valid API configuration (YAML/JSON) defining routes for a REST API, **Then** the Gateway-Controller accepts the configuration and the Router begins routing requests to the specified backend endpoints within 5 seconds
2. **Given** a valid API configuration has been submitted, **When** a client sends an HTTP request matching the configured route path, **Then** the Router forwards the request to the correct backend service and returns the response
3. **Given** an invalid API configuration is submitted (malformed YAML/JSON or missing required fields), **When** the user attempts to deploy it, **Then** the Gateway-Controller rejects it with a clear error message indicating what is invalid

---

### User Story 2 - Update Existing API Configuration (Priority: P2)

An API administrator needs to modify an existing API's configuration (e.g., change backend URL, update routing rules, add new endpoints) without downtime. They update the configuration via the Gateway-Controller, and the changes are reflected in the Router's behavior immediately.

**Why this priority**: API configurations frequently change as backends evolve. This enables zero-downtime updates, which is critical for production systems but builds on the basic deployment capability.

**Independent Test**: Can be tested by first deploying an API configuration (P1 feature), then submitting an updated version with modified routing rules, and verifying the Router's behavior changes to match the new configuration without dropping existing connections.

**Acceptance Scenarios**:

1. **Given** an API configuration is already deployed and actively routing traffic, **When** a user submits an updated version of the same API configuration with modified routes or backend URLs, **Then** the Gateway-Controller updates the Router's configuration and new requests reflect the updated routing rules within 5 seconds
2. **Given** an API configuration update is being applied, **When** the Router receives the new configuration from the xDS server (Gateway-Controller), **Then** in-flight requests continue to completion using the old configuration while new requests use the updated configuration
3. **Given** an updated API configuration contains errors, **When** the user attempts to apply it, **Then** the Gateway-Controller rejects the update and the existing working configuration remains active

---

### User Story 3 - Delete API Configuration (Priority: P2)

An API administrator needs to remove an API from the gateway when it's being deprecated or is no longer needed. They delete the configuration via the Gateway-Controller, and the Router immediately stops accepting traffic for that API.

**Why this priority**: Lifecycle management requires the ability to cleanly remove APIs. This is important for security (removing old endpoints) and resource management, but is less critical than adding/updating APIs.

**Independent Test**: Can be tested by first deploying an API configuration (P1), then deleting it via the Gateway-Controller, and verifying that the Router returns 404 or similar error for requests to the previously configured routes.

**Acceptance Scenarios**:

1. **Given** an API configuration is deployed and actively routing traffic, **When** a user deletes the API configuration from the Gateway-Controller, **Then** the Gateway-Controller removes the configuration from the Router and subsequent requests to the API's routes return appropriate error responses (e.g., 404 Not Found) within 5 seconds
2. **Given** an API configuration is being deleted, **When** the Router receives the deletion command from the xDS server, **Then** in-flight requests to that API complete successfully while new requests are rejected
3. **Given** a user attempts to delete a non-existent API configuration, **When** the deletion request is processed, **Then** the Gateway-Controller returns a clear error message indicating the API configuration does not exist

---

### User Story 4 - List and Query API Configurations (Priority: P3)

An API administrator wants to view all currently deployed API configurations and their status to understand what APIs are active in the gateway and verify configurations are correct.

**Why this priority**: Observability and management are important for operational excellence, but the gateway can function without this query capability. Users can verify behavior by testing the APIs directly.

**Independent Test**: Can be tested by deploying several API configurations (P1), then querying the Gateway-Controller for the list of all configurations and verifying the returned data matches what was deployed.

**Acceptance Scenarios**:

1. **Given** multiple API configurations are deployed to the Gateway-Controller, **When** a user requests a list of all deployed APIs, **Then** the Gateway-Controller returns a complete list with basic metadata (name, version, deployment time, route paths)
2. **Given** a specific API configuration is deployed, **When** a user queries for that API's detailed configuration, **Then** the Gateway-Controller returns the complete YAML/JSON configuration as it was submitted
3. **Given** no API configurations are deployed, **When** a user requests the list of APIs, **Then** the Gateway-Controller returns an empty list with a success status

---

### Edge Cases

- What happens when the Gateway-Controller loses connection to the Router during a configuration update? (Expected: Router continues serving existing configuration; Gateway-Controller retries when connection restored)
- How does the system handle a configuration update that causes the Router to fail health checks? (Expected: Rollback to previous working configuration or clear error reporting)
- What happens when multiple configuration updates are submitted rapidly for the same API? (Expected: Updates are applied in order; only the final state is guaranteed)
- How does the Router behave when the Gateway-Controller (xDS server) is unavailable at startup? (Expected: Router either waits for connection or fails fast with clear error)
- What happens when an API configuration references a backend service that is unavailable? (Expected: Configuration is accepted; Router returns 503 Service Unavailable for requests to that API)
- How does the system handle very large configuration files or a large number of API configurations? (Expected: System should handle at least 100 distinct API configurations without performance degradation)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Gateway-Controller MUST accept API configuration submissions in both YAML and JSON formats
- **FR-002**: Gateway-Controller MUST validate API configuration syntax and structure before accepting it
- **FR-003**: Gateway-Controller MUST act as an xDS (Discovery Service) server that the Router can connect to for configuration updates
- **FR-004**: Gateway-Controller MUST push configuration updates to the Router within 5 seconds of accepting a new or modified API configuration
- **FR-005**: Router MUST be packaged as a Docker container image based on Envoy Proxy with a bootstrap envoy.yaml file pre-configured with xds_cluster pointing to the Gateway-Controller
- **FR-006**: Router MUST dynamically update its routing configuration when receiving updates from the Gateway-Controller's xDS server without requiring restart
- **FR-007**: Router MUST correctly route HTTP requests to backend services based on the deployed API configurations (matching method, path, headers as defined)
- **FR-008**: Gateway-Controller MUST support creating, reading, updating, and deleting (CRUD) API configurations
- **FR-009**: Gateway-Controller MUST persist API configurations so they survive restarts
- **FR-010**: Gateway-Controller MUST provide clear error messages when API configurations are invalid, including specific details about what is wrong
- **FR-011**: Router MUST gracefully handle configuration updates without dropping in-flight requests
- **FR-012**: Gateway-Controller MUST maintain an audit log of all configuration changes (create, update, delete operations) including timestamp and configuration details
- **FR-013**: System MUST support concurrent configuration updates without data corruption or inconsistent state
- **FR-014**: Router MUST return appropriate HTTP error codes when requests don't match any configured route (e.g., 404 Not Found)
- **FR-015**: Gateway-Controller MUST expose an interface for submitting and managing API configurations (CLI, REST API, or both)

### Key Entities

- **API Configuration**: A declarative definition of a REST API including its name, version, context path, backend service URLs, routing rules (methods, paths, parameters), and policies. Represented as YAML or JSON. Key attributes include unique identifier, routes (method + path patterns), upstream backend endpoints, and optional metadata (name, version, description).

- **Route**: A mapping between an incoming HTTP request pattern (method, path, optional headers/query params) and a backend service endpoint. Part of an API Configuration. Relationships: Multiple routes belong to one API Configuration; each route targets one or more backend endpoints.

- **Router Configuration State**: The active set of routing rules currently loaded in the Router/Envoy Proxy. This represents the runtime state that determines how traffic is routed. Updated via xDS protocol from Gateway-Controller.

- **Configuration Change Event**: A record of a configuration operation (create/update/delete) including timestamp, operation type, affected API configuration, and result (success/failure). Used for audit logging and troubleshooting.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can deploy a new API configuration and have it actively routing traffic through the gateway in under 10 seconds from submission
- **SC-002**: Configuration updates apply to the Router without any dropped requests or connection errors for ongoing traffic
- **SC-003**: The system correctly routes 100% of requests matching configured API routes to the appropriate backend services
- **SC-004**: Invalid API configurations are rejected with clear, actionable error messages in 100% of cases, preventing misconfiguration
- **SC-005**: The Gateway-Controller and Router can handle at least 100 distinct API configurations without performance degradation
- **SC-006**: Configuration changes (create/update/delete) are reflected in Router behavior within 5 seconds in 95% of cases
- **SC-007**: Users can successfully complete the full lifecycle (deploy, update, delete) of an API configuration without needing to access or modify Envoy configuration files directly
- **SC-008**: System maintains 100% consistency between Gateway-Controller's stored configurations and Router's active routing behavior under normal operation

## Assumptions

1. **Deployment Environment**: Both Gateway-Controller and Router will run as Docker containers in a networked environment where they can communicate via TCP
2. **API Configuration Format**: The API configuration schema will follow the `api.yaml` specification format defined in the platform's `concepts/api-yaml-specification.md` document
3. **Persistence Mechanism**: Gateway-Controller will use a lightweight embedded database or file-based storage for configuration persistence (specific technology to be determined in planning phase)
4. **Network Reliability**: The network connection between Gateway-Controller and Router is reasonably stable; transient disconnections are acceptable but should be handled gracefully
5. **Single Controller Instance**: Initial implementation assumes a single Gateway-Controller instance managing one or more Router instances (high availability to be addressed in future iterations)
6. **Configuration Interface**: Gateway-Controller will expose a REST API as the primary interface for configuration management, with CLI support potentially added later
7. **Security**: Basic authentication/authorization for Gateway-Controller API will be implemented, but detailed security model (mTLS, RBAC, etc.) is out of scope for initial version
8. **Backend Service Availability**: Backend services referenced in API configurations are expected to be network-accessible from the Router; the gateway is not responsible for backend service discovery or health checking beyond basic connectivity
9. **Configuration Size Limits**: Individual API configuration files are expected to be under 1MB; extremely large configurations are out of scope
10. **xDS Protocol Version**: Will use Envoy's v3 xDS API for compatibility with current Envoy Proxy versions

## Constraints

1. **Component Separation**: Gateway-Controller and Router MUST be independently deployable Docker containers that communicate via network protocols (not shared file systems or in-process communication)
2. **Envoy Dependency**: Router component is architecturally bound to Envoy Proxy, limiting flexibility in choosing alternative proxy technologies
3. **xDS Protocol**: Communication between Gateway-Controller and Router MUST use Envoy's xDS protocol for configuration updates
4. **No Manual Envoy Config**: Users must not need to manually edit Envoy configuration files (envoy.yaml) after initial Router deployment
5. **Stateless Router**: The Router should remain stateless, storing no persistent configuration locally; all configuration must come from Gateway-Controller
6. **Directory Structure**: Implementation files must be organized within the `gateway/` directory as specified

## Dependencies

1. **Envoy Proxy**: Router component depends on an Envoy Proxy Docker base image and requires understanding of Envoy's configuration model and xDS APIs
2. **xDS Protocol Libraries**: Gateway-Controller requires libraries/frameworks for implementing an xDS server (e.g., go-control-plane for Go implementations)
3. **API Configuration Schema**: Depends on the platform's `api.yaml` specification format defined in `concepts/api-yaml-specification.md`
4. **Docker Runtime**: Both components require Docker or compatible container runtime for deployment
5. **Platform API Integration**: Future integration with the platform's central Platform API service for multi-gateway management (not required for initial implementation)

## Out of Scope

The following are explicitly **not** part of this feature and should not be included:

1. **Multi-Gateway Management**: Centralized management of multiple gateway instances across different environments
2. **Gateway Clustering**: High availability, load balancing, or clustering of Gateway-Controller or Router instances
3. **Advanced Policy Enforcement**: Complex policies like rate limiting, authentication/authorization, traffic shaping, circuit breaking (these are separate features)
4. **Observability Stack**: Metrics, logging, tracing infrastructure (basic logging is acceptable, but not a comprehensive observability solution)
5. **Web-Based UI**: Graphical user interface for managing configurations (CLI/API only for initial version)
6. **Backend Service Discovery**: Integration with service registries or automatic backend endpoint discovery
7. **TLS/mTLS Configuration**: Certificate management and secure communication setup (can be added later)
8. **API Versioning Strategy**: Complex version management, deprecation workflows, or canary deployments
9. **Configuration Import/Export**: Bulk operations, migration tools, or backup/restore functionality
10. **Performance Optimization**: Advanced caching, connection pooling, or performance tuning (basic functionality first)

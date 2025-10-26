# Feature Specification: Gateway Management Service

**Last Updated**: October 26, 2025

## Overview

Gateway management service enables platform administrators to register and manage gateway instances within organizations
with comprehensive metadata, secure authentication tokens with rotation and revocation capabilities, real-time status monitoring,
and enhanced operational features. The service provides complete lifecycle management for gateways including criticality tracking,
gateway type classification, connection status monitoring, and lightweight polling capabilities for management portals.

**Features Implemented**:
- **Gateway Criticality**: Boolean flag indicating operational importance
- **Gateway Type Classification**: Support for regular, ai, and event gateway types for specialized routing and processing
- **Connection Status**: Real-time WebSocket connection tracking
- **Virtual Host Configuration**: Domain-based gateway routing
- **Gateway Descriptions**: Optional metadata for operational context
- **Status Polling Endpoint**: Lightweight monitoring for management portals
- **Gateway Updates**: Metadata modification capabilities
- **Enhanced API Filtering**: Project-based API filtering support

## Clarifications

- Q: Should the system support token revocation/rotation, or is the token permanent for the gateway's lifetime? → A: System must support both token rotation (zero-downtime: generate new token while existing one works, then revoke old token after gateway reconfiguration) and immediate revocation (security breach response: revoke token immediately, gateway cannot reconnect until new token issued and configured)
- Q: How does the system handle concurrent registration requests for the same gateway name? → A: Use database unique constraint to prevent race condition. Second request fails with "name already exists" error.
- Q: How does the system handle token verification if the gateway record has been deleted? → A: Reject token verification with "gateway not found" error. Deleting gateway invalidates all its tokens immediately.
- Q: What happens when an administrator attempts to rotate a token for a gateway that doesn't exist? → A: Reject with "gateway not found" error. Operation fails immediately with clear error message.
- Q: What happens when an administrator attempts to revoke an already-revoked token? → A: Idempotent operation - succeeds with message "token already revoked" or similar. No error thrown.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Gateway Registration (Priority: P1)

A platform administrator needs to register a new gateway instance within an organization to enable API deployment and traffic routing capabilities for that organization's APIs.

**Why this priority**: This is the foundational capability required for any gateway to participate in the platform ecosystem. Without registration, gateways cannot authenticate or receive deployment instructions. Gateways are scoped to organizations to ensure proper isolation and access control.

**Independent Test**: Can be fully tested by submitting organization ID, gateway name, and display name, receiving a registration token, and verifying the gateway record is persisted with correct attributes including organization association.

**Acceptance Scenarios**:

1. **Given** the platform API is running, an organization exists, and no gateway with name "prod-gateway-01" exists within that organization, **When** an administrator submits a registration request with organization ID, name "prod-gateway-01", and display name "Production Gateway 01", **Then** the system returns a unique registration token and persists the gateway record linked to the organization
2. **Given** a gateway has been successfully registered within an organization, **When** the administrator views the gateway list for that organization, **Then** the gateway appears with its name, display name, organization association, and registration timestamp
3. **Given** a registration token has been issued, **When** the gateway presents this token for verification, **Then** the system successfully validates the token and identifies both the gateway and its associated organization

---

### User Story 2 - Duplicate Gateway Prevention (Priority: P2)

A platform administrator attempts to register a gateway with a name that already exists within the same organization, and the system prevents duplicate registrations to maintain integrity. Gateway names are unique per organization, allowing different organizations to use the same gateway names.

**Why this priority**: Prevents configuration conflicts and ensures each gateway has a unique identity within its organization. Critical for operational stability but not blocking initial functionality.

**Independent Test**: Can be tested by attempting to register two gateways with the same name within the same organization and verifying the second request is rejected with a clear error message. Additionally, verify that different organizations can successfully register gateways with the same name.

**Acceptance Scenarios**:

1. **Given** a gateway with name "prod-gateway-01" is already registered within organization A, **When** an administrator attempts to register another gateway with the same name "prod-gateway-01" within organization A, **Then** the system rejects the request with an error indicating the name is already in use within that organization
2. **Given** a gateway with name "prod-gateway-01" exists in organization A, **When** an administrator registers a gateway with name "prod-gateway-02" in organization A, **Then** the registration succeeds without conflict
3. **Given** a gateway with name "prod-gateway-01" exists in organization A, **When** an administrator registers a gateway with name "prod-gateway-01" in organization B, **Then** the registration succeeds because gateway names are unique per organization

---

### User Story 3 - Token Rotation for Zero-Downtime Updates (Priority: P2)

A platform administrator needs to rotate a gateway's authentication token without causing service interruption, allowing the gateway to continue operating during the token update process.

**Why this priority**: Essential for operational security hygiene (regular token rotation) and prevents downtime during credential updates. Critical for production environments.

**Independent Test**: Can be tested by generating a new token for an existing gateway, verifying both old and new tokens work simultaneously, then revoking the old token and confirming only the new token works.

**Acceptance Scenarios**:

1. **Given** a gateway "prod-gateway-01" is registered with an active token, **When** an administrator requests token rotation for this gateway, **Then** the system generates a new token and returns it while keeping the existing token valid
2. **Given** a gateway has both an old token and a newly rotated token active, **When** the gateway authenticates using either token, **Then** the system successfully verifies both tokens and identifies the gateway
3. **Given** a new token has been issued and configured in the gateway, **When** an administrator revokes the old token, **Then** the system invalidates the old token while keeping the new token active

---

### User Story 4 - Immediate Token Revocation for Security Response (Priority: P2)

A platform administrator detects a potential token compromise and needs to immediately revoke the token to prevent unauthorized gateway access.

**Why this priority**: Critical security capability for breach response. Must be available before production deployment to meet security requirements.

**Independent Test**: Can be tested by revoking an active gateway token, verifying the gateway cannot authenticate with the revoked token, then issuing a new token and confirming the gateway can reconnect.

**Acceptance Scenarios**:

1. **Given** a gateway "prod-gateway-01" is registered with an active token, **When** an administrator immediately revokes the token, **Then** the system invalidates the token and the gateway cannot authenticate
2. **Given** a gateway's token has been revoked, **When** the gateway attempts to connect using the revoked token, **Then** the system rejects the authentication with an error indicating the token is no longer valid
3. **Given** a gateway's token has been revoked, **When** an administrator issues a new token for the same gateway, **Then** the system generates a new token and the gateway can authenticate after reconfiguration

---

### User Story 5 - Gateway Criticality and Type Management (Priority: P2)

A platform administrator needs to classify gateways by criticality and type to enable proper operational monitoring, alerting, and resource allocation for business-critical gateways and specialized gateway types (ai, event).

**Why this priority**: Essential for operational excellence and proper resource management. Critical gateways require different SLA handling, monitoring thresholds, and incident response procedures. Different gateway types (ai, event) may need specialized routing and processing capabilities.

**Independent Test**: Can be tested by registering gateways with different criticality levels and types, then verifying these properties are persisted and returned in all gateway responses.

**Acceptance Scenarios**:

1. **Given** the platform API is running, **When** an administrator registers a gateway with `isCritical: true` and `functionalityType: regular`, **Then** the system persists these properties and returns them in gateway details and listing responses
2. **Given** multiple gateways exist with different criticality levels, **When** an administrator lists all gateways, **Then** the response includes criticality and type information for proper operational classification
3. **Given** a gateway has been registered with specific criticality and type settings, **When** an administrator updates the gateway metadata, **Then** the system allows modification of criticality while preserving type consistency

---

### User Story 6 - Real-time Gateway Status Monitoring (Priority: P2)

Management portal operators need to continuously monitor gateway connection status and criticality information through lightweight polling to ensure operational visibility and rapid incident response.

**Why this priority**: Critical for operational monitoring and incident response. Management portals require efficient, high-frequency status checks without overwhelming the API with full gateway detail requests.

**Independent Test**: Can be tested by calling the status endpoint repeatedly, verifying minimal data transfer, and confirming real-time accuracy of connection status.

**Acceptance Scenarios**:

1. **Given** multiple gateways are registered with different connection states, **When** management portal polls the `status/gateways` endpoint, **Then** the system returns lightweight status information (id, name, isActive, isCritical) for all gateways
2. **Given** a specific gateway ID is provided as a query parameter, **When** the status endpoint is called, **Then** the system returns status information only for that specific gateway
3. **Given** gateway connection status changes in real-time, **When** management portal polls status frequently, **Then** the system provides current connection state without performance degradation

---

### User Story 7 - Enhanced Gateway Configuration (Priority: P2)

A platform administrator needs to configure gateways with virtual hosts, descriptions, and metadata to enable proper routing, documentation, and operational context.

**Why this priority**: Important for proper gateway routing and operational documentation. Virtual hosts enable domain-based routing while descriptions provide operational context.

**Independent Test**: Can be tested by registering gateways with various vhost configurations and descriptions, then verifying proper persistence and retrieval.

**Acceptance Scenarios**:

1. **Given** the platform API is running, **When** an administrator registers a gateway with virtual host "api.example.com" and description "Production API gateway", **Then** the system validates and persists these configuration details
2. **Given** a gateway has been registered with specific configuration, **When** the gateway details are retrieved, **Then** the response includes all configuration properties for operational reference
3. **Given** virtual host information is provided, **When** the system processes gateway requests, **Then** the virtual host configuration is available for routing decisions

---

### User Story 8 - Gateway Metadata Updates (Priority: P3)

A platform administrator needs to update gateway metadata (display name, description, criticality) without recreating the gateway to maintain operational flexibility and accurate documentation.

**Why this priority**: Essential for maintaining accurate operational documentation and adjusting gateway classification as business requirements evolve.

**Independent Test**: Can be tested by updating various gateway properties and verifying changes are persisted while immutable properties remain unchanged.

**Acceptance Scenarios**:

1. **Given** a gateway is registered with initial metadata, **When** an administrator updates the display name and description, **Then** the system persists the changes while preserving gateway identity and tokens
2. **Given** operational requirements change, **When** an administrator updates a gateway's criticality setting, **Then** the system updates the classification for proper monitoring and alerting
3. **Given** an update request contains invalid data, **When** the update is attempted, **Then** the system validates input and rejects invalid changes with clear error messages

---

### User Story 9 - Invalid Input Handling (Priority: P3)

A platform administrator submits incomplete or invalid gateway registration data, and the system provides clear validation errors.

**Why this priority**: Improves user experience and data quality, but not critical for core functionality. Can be validated through standard input validation.

**Independent Test**: Can be tested by submitting various invalid inputs (empty names, special characters, excessively long values) and verifying appropriate error messages.

**Acceptance Scenarios**:

1. **Given** the platform API is running, **When** an administrator submits a registration request with an empty gateway name, **Then** the system rejects the request with a validation error indicating the name is required
2. **Given** the platform API is running, **When** an administrator submits a registration request with missing required fields (vhost, isCritical, functionalityType), **Then** the system rejects the request with validation errors for each missing field
3. **Given** the platform API is running, **When** an administrator submits a registration request with invalid vhost format, **Then** the system validates domain format and rejects invalid virtual hosts

---

### Edge Cases

- What happens when a gateway name contains only whitespace characters?
- Concurrent registration requests for the same gateway name within the same organization are handled by database unique constraint on (organization_id, name), with the second request failing with "name already exists" error
- What is the maximum length for gateway name and display name fields?
- Token verification for deleted gateways is rejected with "gateway not found" error. Deleting gateway invalidates all its tokens immediately.
- What happens if the token storage mechanism fails during registration?
- Token rotation for non-existent gateways is rejected with "gateway not found" error immediately
- Revoking an already-revoked token is idempotent - succeeds with message "token already revoked" without throwing error
- What happens if a gateway has multiple active tokens and the administrator attempts another rotation before revoking older tokens?
- How does the system behave when token rotation is requested but the new token generation fails?

## Requirements *(mandatory)*

### Functional Requirements

**Gateway Registration:**
- **FR-001**: System MUST accept organization ID (from JWT token), gateway name, display name, virtual host, criticality flag, and gateway type as input parameters for registration
- **FR-002**: System MUST validate that gateway name, display name, virtual host, criticality flag, and gateway type are provided and valid
- **FR-003**: System MUST validate that the organization ID from JWT token references an existing organization
- **FR-004**: System MUST enforce uniqueness of gateway names within each organization (composite uniqueness on organization_id and name)
- **FR-005**: System MUST allow different organizations to register gateways with the same name
- **FR-006**: System MUST generate a cryptographically secure registration token upon successful gateway registration
- **FR-007**: System MUST return the registration token and complete gateway details to the caller exactly once during the registration response
- **FR-008**: System MUST persist gateway registration information including organization association, name, display name, description, virtual host, criticality flag, gateway type, connection status, and timestamps
- **FR-009**: System MUST store tokens in a manner that allows verification without requiring storage of plain-text tokens
- **FR-010**: System MUST provide a verification mechanism that validates tokens presented by gateways
- **FR-011**: System MUST prevent duplicate gateway registrations when the same name is submitted multiple times within the same organization
- **FR-012**: System MUST return appropriate error messages for validation failures (missing fields, invalid organization, duplicate names, invalid formats, invalid virtual hosts)
- **FR-013**: System MUST record creation and update timestamps for each gateway
- **FR-014**: System MUST handle registration requests through REST API endpoints consistent with platform API standards
- **FR-015**: System MUST validate virtual host format according to domain name standards
- **FR-016**: System MUST accept optional description field with appropriate length constraints

**Token Rotation:**
- **FR-015**: System MUST support generating a new token for an existing gateway while keeping the current token valid
- **FR-016**: System MUST allow multiple active tokens per gateway to enable zero-downtime rotation
- **FR-017**: System MUST return the newly generated token to the administrator exactly once during the rotation response
- **FR-018**: System MUST maintain token metadata including creation timestamp and revocation status for each token
- **FR-019**: System MUST verify both old and new tokens during the rotation grace period

**Token Revocation:**
- **FR-020**: System MUST support immediate revocation of specific gateway tokens
- **FR-021**: System MUST reject authentication attempts using revoked tokens with appropriate error messages
- **FR-022**: System MUST allow administrators to issue new tokens after revocation to restore gateway connectivity
- **FR-023**: System MUST record the revocation timestamp when a token is revoked
- **FR-024**: System MUST prevent revoked tokens from being reactivated
- **FR-025**: System MUST implement idempotent revocation (revoking an already-revoked token succeeds without error)

**Token Lifecycle Management:**
- **FR-026**: System MUST track the status of each token (active, revoked)
- **FR-027**: System MUST associate each token with its parent gateway for verification and lifecycle operations
- **FR-028**: System MUST support querying the active status of tokens for audit and security monitoring purposes
- **FR-029**: System MUST reject token verification attempts for deleted gateways with "gateway not found" error
- **FR-030**: System MUST invalidate all tokens when a gateway is deleted (cascade deletion or verification check)

**Gateway Status Monitoring:**
- **FR-031**: System MUST provide a lightweight status polling endpoint optimized for frequent management portal requests
- **FR-032**: System MUST return minimal gateway status information (id, name, isActive, isCritical) for efficient polling
- **FR-033**: System MUST support filtering status results by specific gateway ID via query parameter
- **FR-034**: System MUST track real-time connection status (isActive) for each gateway based on WebSocket connections
- **FR-035**: System MUST automatically update gateway connection status when WebSocket connections are established or terminated
- **FR-036**: System MUST return status information scoped to the requesting organization from JWT token

**Gateway Metadata Management:**
- **FR-037**: System MUST support updating gateway metadata (displayName, description, isCritical) via PUT operations
- **FR-038**: System MUST preserve immutable gateway properties (id, name, organizationId, vhost, functionalityType) during updates
- **FR-039**: System MUST validate updated metadata according to the same rules as creation
- **FR-040**: System MUST update the gateway's updatedAt timestamp when metadata changes occur
- **FR-041**: System MUST return the complete updated gateway information after successful metadata updates

**Enhanced Gateway Properties:**
- **FR-042**: System MUST track gateway criticality status (isCritical) for operational monitoring and alerting
- **FR-043**: System MUST support gateway type classification (regular, ai, event) for specialized routing and processing
- **FR-044**: System MUST store and manage virtual host configuration (vhost) for domain-based routing
- **FR-045**: System MUST support optional gateway descriptions for operational documentation
- **FR-046**: System MUST include all enhanced properties in gateway listing and detail responses
- **FR-047**: System MUST validate boolean properties (isCritical, functionalityType) as required fields during registration

**API Enhancement Integration:**
- **FR-048**: System MUST support enhanced API listing that returns all APIs for an organization by default
- **FR-049**: System MUST support filtering API listings by project when projectId query parameter is provided
- **FR-050**: System MUST maintain backward compatibility with existing API listing behavior while adding project filtering

### Key Entities

- **Gateway**: Represents a registered gateway instance within an organization with comprehensive metadata and operational properties. Enhanced attributes include:
  - **Core Identity**: Unique identifier (UUID), organization association (foreign key), name (unique per organization), display name (human-readable label)
  - **Configuration**: Virtual host (vhost) for domain-based routing, optional description for operational context
  - **Classification**: Criticality flag (isCritical) for operational priority, gateway type (functionalityType) supporting regular, ai, and event types for specialized processing
  - **Status**: Real-time connection status (isActive) based on WebSocket connections
  - **Timestamps**: Creation and last update timestamps for audit and lifecycle tracking
  - **Relationships**: Belongs to exactly one organization, can have multiple tokens throughout lifetime
  
- **Gateway Token**: A cryptographically secure credential associated with a specific gateway. Attributes include token identifier, creation timestamp, revocation status (active/revoked), revocation timestamp (if revoked), and the verification-ready credential (not reversible to plain text). Multiple tokens can be active for a single gateway during rotation.

- **Gateway Status**: Lightweight representation for monitoring and polling operations. Contains minimal essential information (id, name, isActive, isCritical) optimized for frequent management portal requests without full gateway detail overhead.

## Success Criteria *(mandatory)*

### Measurable Outcomes

**Enhanced Gateway Registration:**
- **SC-001**: System successfully prevents 100% of duplicate gateway name registrations within the same organization while allowing the same name across different organizations
- **SC-002**: Registered gateways can authenticate and be verified using their issued tokens
- **SC-003**: Gateway registration records persist correctly with all required attributes including organization association, vhost, criticality, AI type, and connection status
- **SC-004**: All required gateway properties (name, displayName, vhost, isCritical, functionalityType) are validated and enforced during registration
- **SC-005**: Virtual host configurations are properly validated and stored for domain-based routing

**Gateway Status Monitoring:**
- **SC-006**: Status polling endpoint responds within acceptable latency for frequent management portal requests (target: <100ms)
- **SC-007**: Real-time connection status (isActive) accurately reflects actual gateway WebSocket connections
- **SC-008**: Status endpoint supports both organization-wide and gateway-specific filtering via query parameters
- **SC-009**: Status responses contain only essential fields (id, name, isActive, isCritical) for optimal network efficiency

**Gateway Metadata Management:**
- **SC-010**: Gateway metadata updates (displayName, description, isCritical) succeed while preserving immutable properties
- **SC-011**: Gateway update operations maintain data integrity and trigger appropriate timestamp updates
- **SC-012**: Invalid update requests are rejected with clear validation errors

**Token Rotation:**
- **SC-013**: Both old and new tokens work simultaneously during rotation period
- **SC-014**: Administrators can successfully revoke old tokens after configuring new tokens in gateways
- **SC-015**: Zero authentication failures occur during properly executed token rotation

**Token Revocation:**
- **SC-016**: Revoked tokens are immediately rejected with 100% accuracy
- **SC-017**: Gateways can reconnect after receiving and configuring new tokens following revocation
- **SC-018**: Token revocation operations complete successfully without affecting other active tokens

**Enhanced Classification:**
- **SC-019**: Gateway criticality flags (isCritical) are accurately tracked and returned in all gateway responses
- **SC-020**: Gateway type classification (functionalityType) supporting regular, ai, and event types is properly maintained throughout gateway lifecycle
- **SC-021**: Gateway classification information is available for operational monitoring and routing decisions

**API Integration:**
- **SC-022**: Enhanced API listing returns all organization APIs by default and supports project-based filtering
- **SC-023**: Project filtering via query parameter works correctly without affecting non-filtered requests
- **SC-024**: API listing maintains backward compatibility while providing enhanced filtering capabilities

**Operational Readiness:**
- **SC-025**: Administrators can audit all gateway lifecycle events (creation, updates, token management)
- **SC-026**: System correctly tracks all gateway and token status information for security monitoring
- **SC-027**: All gateway management operations complete with appropriate HTTP status codes and error messages

## Assumptions

**Core Architecture:**
- Gateways are scoped to organizations at the database level (each gateway belongs to exactly one organization via foreign key)
- Gateway names are unique per organization, allowing different organizations to use the same gateway names
- Gateways are exposed as root API resources at `/api/v1/gateways` (not nested under organizations in the URL)
- Organization ID is automatically extracted from JWT token claims, not provided in request body or URL path
- Deleting an organization cascades to delete all its gateways and their tokens

**Enhanced Gateway Properties:**
- Virtual hosts (vhost) follow standard domain name conventions and are used for routing decisions
- Gateway criticality (isCritical) is a required boolean field that affects monitoring and alerting priorities
- Gateway type (functionalityType) is a required enum field supporting 'regular', 'ai', and 'event' values for specialized processing and routing
- Connection status (isActive) is system-managed based on real-time WebSocket connections
- Gateway descriptions are optional and support up to 500 characters for operational context

**Naming and Validation:**
- Gateway names follow URL-friendly conventions (lowercase alphanumeric with hyphens, 3-64 characters)
- Display names can contain spaces and mixed case for human readability (1-128 characters)
- Virtual hosts must be valid domain names or IP addresses (1-253 characters)
- Description fields support operational documentation with reasonable length limits

**Token Security and Management:**
- Token verification uses one-way hashing with cryptographic best practices (SHA-256 + salt)
- Registration tokens are cryptographically secure (32+ characters) to prevent brute force attacks
- A gateway can have a maximum of 2 active tokens at any time to prevent unbounded accumulation
- No automatic token expiration is required (tokens remain valid until explicitly revoked)
- Revoked tokens cannot be restored - new tokens must be issued

**Status Monitoring and Operations:**
- Status polling endpoint is optimized for frequent calls (target: <100ms response time)
- Status responses contain minimal data (id, name, isActive, isCritical) for network efficiency
- Real-time connection status is managed automatically by WebSocket connection events
- Management portals can poll status without authentication overhead or full gateway detail requests

**API Integration and Compatibility:**
- Enhanced API listing maintains backward compatibility while adding project-based filtering
- Project filtering via projectId query parameter is optional and doesn't affect default behavior
- All gateway responses include enhanced properties for operational visibility

**Administrative Operations:**
- Platform administrators have appropriate permissions (authorization handled by existing platform mechanisms)
- Token rotation and revocation are administrative operations, not automated
- Gateway configuration with tokens happens externally and is out of scope
- Update operations preserve immutable properties (id, name, organizationId, vhost, functionalityType)

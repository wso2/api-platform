# Feature Overview: Gateway Management Service

## Overview

The Gateway Management Service provides comprehensive lifecycle management for API gateway instances within organizations, including registration with secure token-based authentication, token rotation and revocation capabilities, real-time connection status monitoring through WebSocket integration, gateway classification by criticality and functional type (regular, AI, event), virtual host configuration for domain-based routing, lightweight status polling endpoints optimized for management portal operations, and secure gateway deletion with safety checks. The service ensures complete multi-tenant isolation through organization-scoped operations, prevents duplicate gateway names within organizations while allowing reuse across different organizations, supports zero-downtime token rotation with a maximum of two active tokens per gateway, provides comprehensive CRUD operations with automatic cascade deletion of dependent resources, and includes comprehensive audit trails for compliance tracking.

## Capabilities

### [✓] Capability 01: Gateway Registration and Lifecycle Management

- [✓] **User Story 1** — Gateway Registration with Organization Scoping
  - **As a** platform administrator
  - **I want to** register new gateway instances within organizations with comprehensive metadata including name, display name, description, virtual host, criticality flag, and gateway type
  - **So that** gateways can authenticate to the platform, receive deployment instructions, and be properly classified for operational monitoring while maintaining complete multi-tenant isolation

- **Functional Requirements:**
  - [✓] **FR-001** System accepts organization ID (from JWT token), gateway name, display name, virtual host, criticality flag, and gateway type as registration parameters
  - [✓] **FR-002** System validates all required fields (name, display name, virtual host, criticality flag, gateway type) are provided and valid
  - [✓] **FR-003** System validates organization ID from JWT token references an existing organization
  - [✓] **FR-004** System enforces gateway name uniqueness within each organization using composite constraint on (organization_id, name)
  - [✓] **FR-005** System allows different organizations to register gateways with identical names
  - [✓] **FR-006** System generates cryptographically secure registration token upon successful gateway registration
  - [✓] **FR-007** System returns registration token and complete gateway details exactly once during registration response
  - [✓] **FR-008** System persists gateway information including organization association, name, display name, description, virtual host, criticality flag, gateway type, connection status, and timestamps
  - [✓] **FR-013** System records creation and update timestamps for each gateway
  - [✓] **FR-014** System handles registration requests through REST API endpoints consistent with platform standards
  - [✓] **FR-015** System validates virtual host format according to domain name standards
  - [✓] **FR-016** System accepts optional description field with appropriate length constraints

- **Key Implementation Highlights:**
  - `platform-api/src/internal/handler/gateway.go` - HTTP request handling and routing for gateway operations
  - `platform-api/src/internal/service/gateway.go` - Business logic, validation, and token generation
  - `platform-api/src/internal/repository/gateway.go` - Database operations with organization scoping
  - `platform-api/src/internal/model/gateway.go` - Gateway domain entity definitions
  - `platform-api/src/internal/dto/gateway.go` - Request/response data transfer objects
  - `platform-api/src/internal/database/schema.sql` - Database schema with composite unique constraints

**Notes:**
> Gateway names follow URL-friendly conventions (lowercase alphanumeric with hyphens, 3-64 characters). The composite unique constraint on (organization_id, name) prevents duplicate names within organizations while allowing name reuse across organizations. Virtual hosts must be valid domain names or IP addresses (1-253 characters).

---

### [✓] Capability 02: Secure Token Management with Rotation and Revocation

- [✓] **User Story 3** — Token Rotation for Zero-Downtime Updates
  - **As a** platform administrator
  - **I want to** rotate gateway authentication tokens without service interruption
  - **So that** gateways can continue operating during credential updates while maintaining security hygiene through regular token rotation

- [~] **User Story 4** — Immediate Token Revocation for Security Response
  - **As a** platform administrator
  - **I want to** immediately revoke compromised tokens
  - **So that** unauthorized gateway access is prevented during security incidents with immediate token invalidation

- **Functional Requirements:**
  - [✓] **FR-009** System stores tokens using one-way hashing that allows verification without plain-text storage
  - [✓] **FR-010** System provides verification mechanism to validate tokens presented by gateways
  - [✓] **FR-015** System supports generating new tokens for existing gateways while keeping current tokens valid
  - [✓] **FR-016** System allows multiple active tokens per gateway (maximum 2) to enable zero-downtime rotation
  - [✓] **FR-017** System returns newly generated token to administrator exactly once during rotation response
  - [✓] **FR-018** System maintains token metadata including creation timestamp and revocation status
  - [✓] **FR-019** System verifies both old and new tokens during rotation grace period
  - [~] **FR-020** System supports immediate revocation of specific gateway tokens
  - [~] **FR-021** System rejects authentication attempts using revoked tokens with appropriate error messages
  - [~] **FR-022** System allows administrators to issue new tokens after revocation
  - [~] **FR-023** System records revocation timestamp when tokens are revoked
  - [~] **FR-024** System prevents revoked tokens from being reactivated
  - [~] **FR-025** System implements idempotent revocation (revoking already-revoked tokens succeeds without error)
  - [✓] **FR-026** System tracks status of each token (active, revoked)
  - [✓] **FR-027** System associates each token with parent gateway for verification and lifecycle operations
  - [✓] **FR-028** System supports querying active status of tokens for audit and security monitoring
  - [✓] **FR-029** System rejects token verification attempts for deleted gateways with "gateway not found" error
  - [✓] **FR-030** System invalidates all tokens when gateway is deleted (cascade deletion)

- **Key Implementation Highlights:**
  - `platform-api/src/internal/service/gateway.go` - Token generation using crypto/rand, SHA-256 hashing with unique salts
  - `platform-api/src/internal/repository/gateway.go` - Token storage, retrieval, and status management
  - `platform-api/src/internal/database/schema.sql` - Gateway tokens table with status constraints and cascade deletion
  - `platform-api/src/internal/middleware/auth.go` - JWT validation for organization-scoped access

**Notes:**
> Tokens are generated using cryptographically secure random (32 bytes, 64 hex characters), hashed with SHA-256 and unique per-token salts. Plain-text tokens are never stored in the database. Token verification uses constant-time comparison (crypto/subtle) to prevent timing attacks. Maximum 2 active tokens per gateway prevents unbounded token accumulation during rotation.

---

### [✓] Capability 03: Real-time Gateway Status Monitoring

- [✓] **User Story 6** — Real-time Gateway Status Monitoring
  - **As a** management portal operator
  - **I want to** continuously monitor gateway connection status and criticality through lightweight polling
  - **So that** operational visibility is maintained and rapid incident response is enabled without overwhelming the API

- **Functional Requirements:**
  - [✓] **FR-031** System provides lightweight status polling endpoint optimized for frequent management portal requests
  - [✓] **FR-032** System returns minimal gateway status information (id, name, isActive, isCritical) for efficient polling
  - [✓] **FR-033** System supports filtering status results by specific gateway ID via query parameter
  - [✓] **FR-034** System tracks real-time connection status (isActive) for each gateway based on WebSocket connections
  - [✓] **FR-035** System automatically updates gateway connection status when WebSocket connections are established or terminated
  - [✓] **FR-036** System returns status information scoped to requesting organization from JWT token

- **Key Implementation Highlights:**
  - `platform-api/src/internal/handler/gateway.go` - Lightweight status endpoint implementation
  - `platform-api/src/internal/service/gateway.go` - Status retrieval with optional gateway filtering
  - `platform-api/src/internal/repository/gateway.go` - Optimized status queries returning minimal fields
  - `platform-api/src/internal/dto/gateway.go` - Status response DTOs with constitution-compliant envelope structure

**Notes:**
> Status endpoint is optimized for frequent polling (target response time <100ms). Real-time connection status (isActive) is system-managed based on WebSocket connection lifecycle events - defaults to false on creation and cannot be manually set via API. Status responses follow constitution-compliant envelope format with count, list, and pagination fields.

---

### [✓] Capability 04: Gateway Classification and Enhanced Configuration

- [✓] **User Story 5** — Gateway Criticality and Type Management
  - **As a** platform administrator
  - **I want to** classify gateways by criticality and functional type (regular, AI, event)
  - **So that** proper operational monitoring, alerting, resource allocation, and specialized routing can be enabled based on gateway classification

- [✓] **User Story 7** — Enhanced Gateway Configuration
  - **As a** platform administrator
  - **I want to** configure gateways with virtual hosts, descriptions, and operational metadata
  - **So that** proper domain-based routing is enabled and operational context is documented for team reference

- **Functional Requirements:**
  - [✓] **FR-042** System tracks gateway criticality status (isCritical) for operational monitoring and alerting
  - [✓] **FR-043** System supports gateway type classification (regular, ai, event) for specialized routing and processing
  - [✓] **FR-044** System stores and manages virtual host configuration (vhost) for domain-based routing
  - [✓] **FR-045** System supports optional gateway descriptions for operational documentation
  - [✓] **FR-046** System includes all enhanced properties in gateway listing and detail responses
  - [✓] **FR-047** System validates boolean properties (isCritical) and enum properties (functionalityType) as required fields during registration

- **Key Implementation Highlights:**
  - `platform-api/src/internal/model/gateway.go` - Extended gateway model with criticality, type, vhost, and description fields
  - `platform-api/src/internal/database/schema.sql` - Schema with gateway_functionality_type enum constraint (regular, ai, event)
  - `platform-api/src/resources/openapi.yaml` - API contract documenting all enhanced properties

**Notes:**
> Gateway type (functionalityType) is validated against global constants (regular, ai, event) defined in constants.go. Criticality flag (isCritical) is required boolean affecting monitoring priorities. Virtual hosts follow standard domain name conventions. Descriptions support up to 500 characters for operational context.

---

### [✓] Capability 05: Gateway Metadata Updates and CRUD Operations

- [✓] **User Story 8** — Gateway Metadata Updates
  - **As a** platform administrator
  - **I want to** update gateway metadata (display name, description, criticality) without recreating gateways
  - **So that** operational documentation remains accurate and gateway classification adapts to evolving business requirements

- **Functional Requirements:**
  - [✓] **FR-037** System supports updating gateway metadata (displayName, description, isCritical) via PUT operations
  - [✓] **FR-038** System preserves immutable gateway properties (id, name, organizationId, vhost, functionalityType) during updates
  - [✓] **FR-039** System validates updated metadata according to same rules as creation
  - [✓] **FR-040** System updates gateway updatedAt timestamp when metadata changes occur
  - [✓] **FR-041** System returns complete updated gateway information after successful metadata updates
  - [✓] **FR-011** System prevents duplicate gateway registrations when same name is submitted multiple times within same organization
  - [✓] **FR-012** System returns appropriate error messages for validation failures (missing fields, invalid organization, duplicate names, invalid formats, invalid virtual hosts)

- **Key Implementation Highlights:**
  - `platform-api/src/internal/handler/gateway.go` - Update and delete endpoint handlers
  - `platform-api/src/internal/service/gateway.go` - Update validation logic preserving immutable properties
  - `platform-api/src/internal/repository/gateway.go` - Update and delete database operations with organization scoping

**Notes:**
> Update operations preserve immutable properties (id, name, organizationId, vhost, functionalityType) while allowing modification of displayName, description, and isCritical. All updates trigger automatic updatedAt timestamp changes. Deletion operations cascade to all dependent gateway tokens via foreign key constraints.

---

### [✓] Capability 06: Duplicate Prevention and Input Validation

- [✓] **User Story 2** — Duplicate Gateway Prevention
  - **As a** platform administrator
  - **I want to** prevent duplicate gateway registrations within organizations
  - **So that** configuration conflicts are avoided and each gateway maintains unique identity within its organization

- [✓] **User Story 9** — Invalid Input Handling
  - **As a** platform administrator
  - **I want to** receive clear validation errors for incomplete or invalid gateway data
  - **So that** data quality is maintained and registration issues are quickly identified

- **Functional Requirements:**
  - [✓] **FR-004** System enforces uniqueness of gateway names within each organization (composite uniqueness on organization_id and name)
  - [✓] **FR-005** System allows different organizations to register gateways with same name
  - [✓] **FR-011** System prevents duplicate gateway registrations when same name is submitted multiple times within same organization
  - [✓] **FR-012** System returns appropriate error messages for validation failures (missing fields, invalid organization, duplicate names, invalid formats, invalid virtual hosts)

- **Key Implementation Highlights:**
  - `platform-api/src/internal/database/schema.sql` - Composite unique constraint on (organization_uuid, name)
  - `platform-api/src/internal/service/gateway.go` - Input validation for name patterns, display name length, vhost format
  - `platform-api/src/internal/handler/gateway.go` - HTTP error responses with clear validation messages

**Notes:**
> Composite unique constraint at database level prevents race conditions in concurrent registration attempts. Gateway names must match pattern ^[a-z0-9-]+$ with 3-64 character length. Display names support 1-128 characters with spaces and mixed case. Virtual hosts validated against standard domain name conventions.

---

### [✓] Capability 07: Secure Gateway Deletion with Organization Isolation

- [✓] **User Story 10** — Delete Unused Gateway
  - **As a** platform administrator
  - **I want to** delete gateways that are no longer in use
  - **So that** a clean gateway inventory is maintained and confusion is prevented when managing active gateways

- **Functional Requirements:**
  - [✓] **FR-048** System provides DELETE endpoint at /api/v1/gateways/{gatewayId} accepting gateway UUID
  - [✓] **FR-049** System requires JWT authentication with valid organization claim for deletion requests
  - [✓] **FR-050** System verifies gateway belongs to organization specified in JWT token before allowing deletion
  - [✓] **FR-051** System returns 404 Not Found when gateway doesn't exist or belongs to different organization
  - [✓] **FR-052** System performs permanent deletion (hard delete) removing gateway records from database and returns 204 No Content when gateway is successfully deleted
  - [✓] **FR-053** System cascade deletes all associated gateway tokens permanently when gateway is deleted (no soft delete or archiving)
  - [✓] **FR-054** System validates gateway ID format (UUID) before processing deletion
  - [✓] **FR-055** System executes deletion within database transaction to ensure atomicity
  - [✓] **FR-056** System returns 401 Unauthorized when JWT token is missing or invalid
  - [✓] **FR-057** System returns 500 Internal Server Error for unexpected database failures

- **Key Implementation Highlights:**
  - `platform-api/src/internal/handler/gateway.go` - DELETE endpoint handler
  - `platform-api/src/internal/service/gateway.go` - Deletion business logic and validation
  - `platform-api/src/internal/repository/gateway.go` - Transaction-wrapped DELETE operations
  - `platform-api/src/internal/database/schema.sql` - CASCADE delete constraint configuration
  - `platform-api/src/resources/openapi.yaml` - DELETE endpoint contract

**Notes:**
> Gateway deletion is permanent (hard delete) with no recovery mechanism. Organizations must maintain external backups if recovery is needed. All associated tokens are automatically removed via CASCADE constraints. Organization isolation is enforced by combining JWT authentication with organization_id verification in the WHERE clause of DELETE statements.

---

### [ ] Capability 08: Safety Checks for Active Deployments and Connections

- [ ] **User Story 11** — Prevent Deletion of Gateways with Active Deployments
  - **As a** platform administrator
  - **I want to** be prevented from accidentally deleting gateways that have active API deployments
  - **So that** live API traffic is not broken

- **Functional Requirements:**
  - [ ] **FR-058** System checks for active API deployments before allowing gateway deletion with no bypass mechanism
  - [ ] **FR-059** System returns 409 Conflict when attempting to delete gateway with active API deployments
  - [ ] **FR-060** System includes deployment count in error message when deletion is blocked by active deployments
  - [ ] **FR-061** System checks for active WebSocket connections before allowing gateway deletion with no bypass mechanism
  - [ ] **FR-062** System returns 409 Conflict when attempting to delete gateway with active WebSocket connections
  - [ ] **FR-063** System includes active connection count in error message when deletion is blocked by WebSocket connections
  - [ ] **FR-064** System does not provide any force delete mechanism or parameter to bypass active deployment checks
  - [ ] **FR-065** System does not provide any force delete mechanism or parameter to bypass active WebSocket connection checks

- **Key Implementation Highlights:**
  - `platform-api/src/internal/service/gateway.go` - Pre-deletion validation checks
  - `platform-api/src/internal/repository/deployment.go` - Active deployment counting
  - `platform-api/src/internal/repository/websocket.go` - Active connection counting

**Notes:**
> Safety checks are mandatory with no force delete option. Administrators must undeploy all APIs and close all WebSocket connections before deletion. Error responses include counts of blocking resources to inform resolution steps.

---

### [ ] Capability 09: Observability and Audit Trail

- [ ] **Observability Requirements** — Comprehensive Logging and Metrics
  - **Purpose:** Enable operational monitoring, troubleshooting, and compliance tracking through structured logs, Prometheus metrics, and persistent audit events

- **Functional Requirements:**
  - [ ] **FR-066** System logs deletion attempts as structured JSON with info level including gateway ID, organization ID, and correlation ID
  - [ ] **FR-067** System logs deletion errors as structured JSON with error level including failure reason, gateway ID, and correlation ID
  - [ ] **FR-068** System exposes Prometheus metrics tracking deletion success count and deletion failure count by reason (not_found, conflict_deployments, conflict_connections, auth_error, db_error)
  - [ ] **FR-069** System records audit event for all deletion attempts (success and failure) including user ID from JWT, gateway ID, gateway name, organization ID, timestamp, outcome (success/failure), and failure reason if applicable
  - [ ] **FR-070** System does not block deletion operation if audit trail write fails; audit failures should be logged but not prevent deletion

- **Key Implementation Highlights:**
  - `platform-api/src/internal/service/gateway.go` - Structured logging with zap, metrics instrumentation
  - `platform-api/src/internal/metrics/gateway.go` - Prometheus counter definitions
  - `platform-api/src/internal/service/audit.go` - Audit trail integration

**Notes:**
> Observability is non-blocking: audit failures do not prevent deletion success. Metrics use standard Prometheus naming conventions with status labels for categorization. Structured logs use correlation IDs (X-Request-ID header) for request tracing across distributed systems.

---

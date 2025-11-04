# Feature Specification: Gateway Deletion

**Feature Branch**: `004-gateway-deletion`
**Created**: 2025-10-23
**Status**: Draft
**Input**: User description: "read this feature /home/malintha/wso2apim/gitworkspace/api-platform/platform-api/spec/impls/gateway-management/gateway-management.md for context. I want to implement the gateway deletion feature by providing the gateway id"

## Clarifications

### Session 2025-10-23

- Q: When a gateway with active WebSocket connections is deleted, how should the system handle those connections? → A: Block deletion (409 Conflict) if any WebSocket connections are active
- Q: What level of observability (logging/metrics) is required for gateway deletion operations? → A: Standard - Structured JSON logs (info/error) with correlation IDs, Prometheus metrics for success/failure counts
- Q: Should gateway deletion events be recorded in a persistent audit trail for compliance and security tracking? → A: Yes - Record audit events with user ID, gateway ID, timestamp, and outcome (success/failure)
- Q: Should gateway deletion be permanent (hard delete) or should deleted gateways be archived for potential recovery (soft delete)? → A: Permanent deletion - Gateway and tokens are immediately removed from database
- Q: Should administrators have a force delete option to bypass active deployment and connection checks? → A: No - Always require undeploying APIs and closing connections before deletion

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Delete Unused Gateway (Priority: P1)

As a platform administrator, I need to delete gateways that are no longer in use to maintain a clean gateway inventory and prevent confusion when managing active gateways.

**Why this priority**: This is the core functionality and enables administrators to remove obsolete gateways, reducing clutter and ensuring the gateway list reflects only active infrastructure.

**Independent Test**: Can be fully tested by creating a gateway, then deleting it by its ID, and verifying it no longer appears in the gateway list. This delivers immediate value by allowing cleanup of test or decommissioned gateways.

**Acceptance Scenarios**:

1. **Given** I have a valid JWT token and a gateway exists in my organization, **When** I send a DELETE request with the gateway ID, **Then** the gateway is permanently removed from the database and I receive a 204 No Content response
2. **Given** I have a valid JWT token and a gateway with active tokens exists, **When** I delete the gateway, **Then** all associated tokens are also permanently deleted (hard delete with cascade)
3. **Given** I have a valid JWT token, **When** I attempt to delete a gateway that doesn't exist, **Then** I receive a 404 Not Found error
4. **Given** I have a valid JWT token from Organization A, **When** I attempt to delete a gateway belonging to Organization B, **Then** I receive a 404 Not Found error (organization isolation)
5. **Given** I delete a gateway successfully, **When** I attempt to delete the same gateway again, **Then** I receive a 404 Not Found error (idempotent behavior for non-existent resources)
6. **Given** I successfully delete a gateway, **When** I query the audit trail, **Then** an audit event is recorded with user ID, gateway ID, organization ID, timestamp, and success outcome

---

### User Story 2 - Prevent Deletion of Gateways with Active Deployments (Priority: P2)

As a platform administrator, I need to be prevented from accidentally deleting gateways that have active API deployments to avoid breaking live API traffic.

**Why this priority**: This is a critical safety feature that prevents accidental service disruption. While P2 because deletion itself (P1) can work without this check, it's essential for production safety.

**Independent Test**: Can be tested by deploying an API to a gateway, attempting to delete the gateway, and verifying the deletion is blocked with an appropriate error message. This delivers value by protecting live services.

**Acceptance Scenarios**:

1. **Given** I have a gateway with one or more active API deployments, **When** I attempt to delete the gateway, **Then** I receive a 409 Conflict error with a message indicating the gateway has active deployments and I must undeploy all APIs first
2. **Given** I have a gateway with API deployments, **When** I undeploy all APIs and then delete the gateway, **Then** the deletion succeeds with a 204 No Content response
3. **Given** I have a gateway with API deployments, **When** I request deletion, **Then** the error response includes the count of active deployments to inform my decision
4. **Given** I have a gateway with active WebSocket connections, **When** I attempt to delete the gateway, **Then** I receive a 409 Conflict error with a message indicating active connections exist and I must close all connections first
5. **Given** I attempt to delete a gateway but am blocked due to active deployments, **When** I query the audit trail, **Then** an audit event is recorded with user ID, gateway ID, timestamp, and failure outcome with reason

---

### Edge Cases

- What happens when attempting to delete a gateway that doesn't exist? (404 Not Found response)
- What happens when attempting to delete a gateway from a different organization? (404 Not Found due to organization isolation)
- How does the system handle deletion of a gateway with multiple active tokens? (Hard delete with cascade removes all tokens permanently)
- What happens when attempting to delete a gateway with active WebSocket connections? (Deletion is blocked with 409 Conflict to prevent orphaned connections; administrator must close connections first)
- What happens if deletion fails midway due to database error? (Transaction rollback ensures gateway and tokens remain intact)
- What happens if audit trail write fails? (Deletion proceeds but failure is logged; audit is not blocking)
- Can a deleted gateway be recovered? (No - deletion is permanent; organizations must maintain their own backup/recovery processes if needed)
- Is there a force delete option to bypass safety checks? (No - all safety checks are mandatory; administrators must undeploy APIs and close connections before deletion)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a DELETE endpoint at `/api/v1/gateways/{gatewayId}` that accepts a gateway UUID
- **FR-002**: System MUST require JWT authentication with valid organization claim for deletion requests
- **FR-003**: System MUST verify the gateway belongs to the organization specified in the JWT token before allowing deletion
- **FR-004**: System MUST return 404 Not Found when gateway doesn't exist or belongs to a different organization
- **FR-005**: System MUST perform permanent deletion (hard delete) removing gateway records from database and return 204 No Content when gateway is successfully deleted
- **FR-006**: System MUST cascade delete all associated gateway tokens permanently when a gateway is deleted (no soft delete or archiving)
- **FR-007**: System MUST validate the gateway ID format (UUID) before processing deletion
- **FR-008**: System MUST check for active API deployments before allowing gateway deletion with no bypass mechanism
- **FR-009**: System MUST return 409 Conflict when attempting to delete a gateway with active API deployments
- **FR-010**: System MUST include deployment count in error message when deletion is blocked by active deployments
- **FR-011**: System MUST execute deletion within a database transaction to ensure atomicity
- **FR-012**: System MUST return 401 Unauthorized when JWT token is missing or invalid
- **FR-013**: System MUST return 500 Internal Server Error for unexpected database failures
- **FR-014**: System MUST check for active WebSocket connections before allowing gateway deletion with no bypass mechanism
- **FR-015**: System MUST return 409 Conflict when attempting to delete a gateway with active WebSocket connections
- **FR-016**: System MUST include active connection count in error message when deletion is blocked by WebSocket connections
- **FR-017**: System MUST log deletion attempts as structured JSON with info level including gateway ID, organization ID, and correlation ID
- **FR-018**: System MUST log deletion errors as structured JSON with error level including failure reason, gateway ID, and correlation ID
- **FR-019**: System MUST expose Prometheus metrics tracking deletion success count and deletion failure count by reason (not_found, conflict_deployments, conflict_connections, auth_error, db_error)
- **FR-020**: System MUST record audit event for all deletion attempts (success and failure) including user ID from JWT, gateway ID, gateway name, organization ID, timestamp, outcome (success/failure), and failure reason if applicable
- **FR-021**: System MUST NOT block deletion operation if audit trail write fails; audit failures should be logged but not prevent deletion
- **FR-022**: System MUST NOT provide any force delete mechanism or parameter to bypass active deployment checks
- **FR-023**: System MUST NOT provide any force delete mechanism or parameter to bypass active WebSocket connection checks

### Key Entities

- **Gateway**: Represents a gateway instance with UUID, organization ID, name, display name, and timestamps. Must be deleted only by members of the owning organization. Deletion is permanent (hard delete) with no recovery mechanism.
- **Gateway Token**: Authentication tokens associated with a gateway. Automatically and permanently deleted when parent gateway is deleted (hard delete with cascade behavior).
- **API Deployment**: Represents API deployments to gateways. Blocks gateway deletion when active deployments exist with no bypass option.
- **WebSocket Connection**: Active control plane connections from gateway instances. Blocks gateway deletion when connections are active with no bypass option.
- **Audit Event**: Record of gateway deletion attempt including user ID, gateway ID, gateway name, organization ID, timestamp, action (gateway_delete), outcome (success/failure), and optional failure reason. Persists permanently even after gateway is deleted.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Administrators can delete unused gateways in under 5 seconds with a single API call
- **SC-002**: 100% of gateway deletions correctly enforce organization isolation (no cross-organization access)
- **SC-003**: All gateway tokens are automatically and permanently removed when their parent gateway is deleted (0 orphaned tokens)
- **SC-004**: 100% of deletion attempts on gateways with active deployments are blocked with clear error messages and no bypass option
- **SC-005**: Gateway deletion operations complete within 2 seconds under normal load
- **SC-006**: System maintains data integrity during deletion failures (no partial deletions)
- **SC-007**: 100% of deletion attempts on gateways with active WebSocket connections are blocked with clear error messages and no bypass option
- **SC-008**: All deletion operations (success and failure) are logged with structured JSON format and correlation IDs
- **SC-009**: Deletion metrics are exposed via Prometheus endpoint with success/failure counts by reason
- **SC-010**: All deletion attempts (success and failure) are recorded in persistent audit trail within 1 second of operation completion
- **SC-011**: Audit trail records contain complete information (user ID, gateway ID, timestamp, outcome) for forensic analysis
- **SC-012**: Deleted gateways are immediately and permanently removed with no recovery mechanism (hard delete verification)

## Assumptions

- The existing gateway management implementation handles JWT authentication and organization extraction via middleware
- Database foreign key constraints are already configured for cascade deletion (`ON DELETE CASCADE`)
- API deployment tracking exists in the system and can be queried to check for active deployments
- The OpenAPI specification will be updated to document the new DELETE endpoint
- Deletion is a privileged operation available to all authenticated organization members (no role-based access control required at this stage)
- WebSocket connection tracking exists in the system and can be queried to check for active connections
- Structured logging framework (e.g., zap) is available for JSON log output
- Prometheus metrics endpoint exists and can be extended with deletion metrics
- Audit trail system exists and provides API for recording audit events
- JWT token contains user identifier claim that can be extracted for audit records
- Organizations requiring backup/recovery of deleted gateways will implement their own external backup processes
- Administrators follow proper operational procedures: undeploy APIs before deleting gateways, close WebSocket connections before deletion
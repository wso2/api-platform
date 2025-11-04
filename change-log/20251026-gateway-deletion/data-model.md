# Data Model: Gateway Deletion

**Feature**: Gateway Deletion
**Date**: 2025-10-23
**Status**: Complete

## Overview

This document defines the data entities, validation rules, and state transitions for the gateway deletion feature. The feature primarily operates on existing entities (`Gateway`, `Gateway Token`) with additions for audit trail support.

## Entities

### Gateway (Existing)

**Source**: `internal/model/gateway.go`, `internal/database/schema.sql`

**Schema**:
```sql
CREATE TABLE gateways (
    uuid TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    created_at DATETIME,
    updated_at DATETIME,
    FOREIGN KEY (organization_id) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_id, name)
);
```

**Deletion Behavior**:
- Hard delete (permanent removal from database)
- CASCADE delete to `gateway_tokens` table automatically executed by database
- No soft delete flag or deleted_at timestamp

**Validation for Deletion**:
- `uuid`: Must be valid UUID v4 format
- `organization_id`: Must match authenticated user's organization from JWT token
- Pre-deletion checks:
  - No active API deployments exist
  - No active WebSocket connections exist

---

### Gateway Token (Existing)

**Source**: `internal/model/gateway.go`, `internal/database/schema.sql`

**Schema**:
```sql
CREATE TABLE gateway_tokens (
    uuid TEXT PRIMARY KEY,
    gateway_uuid TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    salt TEXT NOT NULL,
    status TEXT DEFAULT 'active',
    created_at DATETIME,
    revoked_at DATETIME,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    CHECK (status IN ('active', 'revoked')),
    CHECK (revoked_at IS NULL OR status = 'revoked')
);
```

**Deletion Behavior**:
- Automatically deleted via CASCADE constraint when parent gateway is deleted
- No explicit deletion logic required in application code
- All tokens (active and revoked) deleted together with gateway

---

### Audit Event (New or Existing)

**Source**: `internal/model/audit.go` (assumed, may need to be created)

**Purpose**: Record all gateway deletion attempts for compliance tracking

**Schema** (proposed if not exists):
```sql
CREATE TABLE audit_events (
    uuid TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    organization_id TEXT NOT NULL,
    action TEXT NOT NULL,           -- e.g., "gateway_delete"
    resource_type TEXT NOT NULL,    -- e.g., "gateway"
    resource_id TEXT NOT NULL,      -- gateway UUID
    resource_name TEXT,             -- gateway name for human readability
    outcome TEXT NOT NULL,          -- "success" or "failure"
    failure_reason TEXT,            -- populated if outcome = "failure"
    timestamp DATETIME NOT NULL,
    metadata TEXT                   -- JSON for additional context
);

CREATE INDEX idx_audit_events_user ON audit_events(user_id);
CREATE INDEX idx_audit_events_resource ON audit_events(resource_type, resource_id);
CREATE INDEX idx_audit_events_timestamp ON audit_events(timestamp);
```

**Go Model** (proposed):
```go
type AuditEvent struct {
    UUID            string    `json:"id"`
    UserID          string    `json:"userId"`
    OrganizationID  string    `json:"organizationId"`
    Action          string    `json:"action"`
    ResourceType    string    `json:"resourceType"`
    ResourceID      string    `json:"resourceId"`
    ResourceName    string    `json:"resourceName,omitempty"`
    Outcome         string    `json:"outcome"`
    FailureReason   string    `json:"failureReason,omitempty"`
    Timestamp       time.Time `json:"timestamp"`
    Metadata        string    `json:"metadata,omitempty"` // JSON string
}
```

**Field Values for Gateway Deletion**:
- `action`: `"gateway_delete"`
- `resource_type`: `"gateway"`
- `resource_id`: Gateway UUID
- `resource_name`: Gateway display name
- `outcome`: `"success"` or `"failure"`
- `failure_reason`: One of:
  - `"not_found"` - Gateway doesn't exist or wrong organization
  - `"active_deployments"` - Blocked by active API deployments
  - `"active_connections"` - Blocked by active WebSocket connections
  - `"unauthorized"` - JWT authentication failure
  - `"internal_error"` - Database or unexpected error

**Metadata JSON Examples**:
```json
{
  "deploymentCount": 3,
  "requestId": "abc-123-def"
}
```

---

## Validation Rules

### Input Validation (Handler Layer)

**Gateway ID (Path Parameter)**:
- **Rule**: Must be valid UUID v4 format
- **Pattern**: `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
- **Error**: 400 Bad Request if invalid format
- **Message**: "Invalid gateway ID format"

**Authentication (JWT Token)**:
- **Rule**: Authorization header must contain valid JWT with organization claim
- **Error**: 401 Unauthorized if missing or invalid
- **Message**: "Unauthorized: Invalid or missing authentication token"

### Business Logic Validation (Service Layer)

**Gateway Existence & Ownership**:
- **Rule**: Gateway must exist in database
- **Rule**: Gateway's organization_id must match JWT token's organization claim
- **Error**: 404 Not Found if either check fails
- **Message**: "Gateway not found"
- **Rationale**: Hide existence from other organizations (security through obscurity)

**Active Deployment Check**:
- **Rule**: Gateway must have zero active API deployments
- **Query**: `SELECT COUNT(*) FROM api_deployments WHERE gateway_id = ? AND status = 'active'`
- **Error**: 409 Conflict if count > 0
- **Message**: "Cannot delete gateway: {count} active API deployment(s) exist. Please undeploy all APIs first."
- **Details**: Include deployment count in error response

**Active Connection Check**:
- **Rule**: Gateway must have zero active WebSocket connections
- **Query**: `SELECT COUNT(*) FROM websocket_connections WHERE gateway_id = ? AND status = 'connected'`
- **Error**: 409 Conflict if count > 0
- **Message**: "Cannot delete gateway: {count} active connection(s) exist. Please close all connections first."
- **Details**: Include connection count in error response

---

## State Transitions

### Gateway Lifecycle

```
[Registered] ──────delete request──────> [Validating]
                                              |
                      ┌───────────────────────┴─────────────────────┐
                      |                                             |
                [Checks Pass]                              [Checks Fail]
                      |                                             |
             [Deleted (CASCADE)]                          [Remains Registered]
                      |                                             |
                  (No Record)                              (Unchanged State)
```

**States**:
1. **Registered**: Normal state, gateway exists in database
2. **Validating**: Transient state during pre-deletion checks
3. **Deleted**: Gateway record removed from database (no state, record doesn't exist)
4. **Remains Registered**: Validation failed, gateway stays unchanged

**Transitions**:
- **Registered → Validating**: DELETE request received, UUID valid, auth passed
- **Validating → Deleted**: All checks pass, DELETE executed, CASCADE removes tokens
- **Validating → Remains Registered**: Checks fail (deployments, connections), deletion aborted

**Terminal States**:
- **Deleted**: No record in database, not recoverable
- **Remains Registered**: Exists with same UUID, can be deleted later after resolving conflicts

---

## Error States

### Not Found (404)

**Trigger**:
- Gateway UUID doesn't exist
- Gateway exists but belongs to different organization
- Second DELETE attempt on already-deleted gateway (idempotent behavior)

**Response**:
```json
{
  "error": {
    "code": "GATEWAY_NOT_FOUND",
    "message": "Gateway not found"
  }
}
```

---

### Conflict - Active Deployments (409)

**Trigger**: Gateway has one or more active API deployments

**Response**:
```json
{
  "error": {
    "code": "CONFLICT_ACTIVE_DEPLOYMENTS",
    "message": "Cannot delete gateway: 3 active API deployment(s) exist. Please undeploy all APIs first.",
    "details": {
      "gatewayId": "abc-123-def",
      "deploymentCount": 3
    }
  }
}
```

---

### Conflict - Active Connections (409)

**Trigger**: Gateway has one or more active WebSocket connections

**Response**:
```json
{
  "error": {
    "code": "CONFLICT_ACTIVE_CONNECTIONS",
    "message": "Cannot delete gateway: 2 active connection(s) exist. Please close all connections first.",
    "details": {
      "gatewayId": "abc-123-def",
      "connectionCount": 2
    }
  }
}
```

---

### Unauthorized (401)

**Trigger**: Missing or invalid JWT token

**Response**:
```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Unauthorized: Invalid or missing authentication token"
  }
}
```

---

### Internal Server Error (500)

**Trigger**: Database error, unexpected exception

**Response**:
```json
{
  "error": {
    "code": "INTERNAL_ERROR",
    "message": "An unexpected error occurred while processing your request"
  }
}
```

**Note**: Do not leak internal details (stack traces, SQL errors) in production

---

## Data Retention

### Gateway & Tokens

**Retention**: None (hard delete)
- Gateway record permanently removed
- Token records permanently removed via CASCADE
- No archival or soft delete mechanism

**Recovery**: Not possible. Deletions are irreversible.
- Organizations must maintain external backups if recovery needed
- Audit trail persists for compliance tracking

### Audit Events

**Retention**: Permanent (indefinite retention)
- Audit events persist even after gateway is deleted
- Enables historical compliance reporting
- Supports forensic analysis and security investigations

**Cleanup**: Not defined in current scope
- Future implementation may add audit log rotation policy
- Recommend: Archive audit events older than 7 years

---

## Indexes & Performance

### Existing Indexes (No Changes Required)

**Gateways Table**:
- Primary key on `uuid` (automatic B-tree index)
- Unique constraint on `(organization_id, name)` (automatic composite index)

**Gateway Tokens Table**:
- Primary key on `uuid` (automatic B-tree index)
- Foreign key on `gateway_uuid` (automatic index for CASCADE performance)

**Query Performance**:
- DELETE by UUID + organization_id: Uses primary key index (O(log n))
- CASCADE delete tokens: Uses foreign key index (O(k) where k = token count per gateway)
- Expected latency: <10ms for deletion, <50ms with validation checks

### Recommended Indexes (If Audit Table Created)

```sql
CREATE INDEX idx_audit_events_user ON audit_events(user_id);
CREATE INDEX idx_audit_events_resource ON audit_events(resource_type, resource_id);
CREATE INDEX idx_audit_events_timestamp ON audit_events(timestamp DESC);
```

**Rationale**:
- User-based queries: "Show all deletions by user X"
- Resource-based queries: "Show all events for gateway Y"
- Time-based queries: "Show recent deletion events"

---

## Summary

**Entities Affected**:
- Gateway (existing): Hard delete with CASCADE
- Gateway Token (existing): Automatically deleted via CASCADE
- Audit Event (new or existing): New records created for compliance

**No Schema Changes Required**:
- Existing CASCADE constraints sufficient
- Audit table may already exist or needs to be created separately

**Validation Layers**:
1. Handler: UUID format, JWT authentication
2. Service: Gateway existence, ownership, deployment check, connection check
3. Repository: Transaction-wrapped DELETE with atomicity

**Performance Characteristics**:
- Single DELETE statement with WHERE clause
- CASCADE handled by database (O(k) for k tokens)
- Expected <2 seconds total latency including validation
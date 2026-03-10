# Gateway Registration - Data Model

## Overview

This document defines the data model for gateway registration. Gateways are scoped to organizations at the database level (each gateway belongs to an organization), but exposed as a root resource in the API.

## Entity Relationship Diagram

```
┌──────────────────┐
│  Organizations   │
├──────────────────┤
│ uuid (PK)        │
│ handle (UNIQUE)  │
│ name             │
│ ...              │
└────────┬─────────┘
         │
         │ 1:N
         │
         ▼
┌──────────────────┐
│    Gateways      │
├──────────────────┤
│ uuid (PK)        │
│ organization_id  │◄─── FK to organizations
│ name             │◄─── UNIQUE per organization
│ display_name     │
│ description      │
│ vhost            │
│ is_critical      │
│ gateway_functionality_type     │
│ is_active        │
│ created_at       │
│ updated_at       │
└────────┬─────────┘
         │
         │ 1:N
         │
         ▼
┌──────────────────┐
│  Gateway Tokens  │
├──────────────────┤
│ uuid (PK)        │
│ gateway_uuid (FK)│
│ token_hash       │
│ salt             │
│ status           │
│ created_at       │
│ revoked_at       │
└──────────────────┘
```

## Entities

### Gateway

Represents a registered API gateway instance within an organization.

**Attributes**:

| Field | Type | Constraints                         | Description |
|-------|------|-------------------------------------|-------------|
| `uuid` | TEXT | PRIMARY KEY, NOT NULL               | Unique identifier (UUID v7) |
| `organization_id` | TEXT | FOREIGN KEY, NOT NULL               | Organization this gateway belongs to |
| `name` | TEXT | NOT NULL                            | URL-friendly gateway identifier (unique per org) |
| `display_name` | TEXT | NOT NULL                            | Human-readable gateway name |
| `description` | TEXT | NULLABLE                            | Optional gateway description |
| `vhost` | TEXT | NOT NULL                            | Virtual host (domain name) for the gateway |
| `is_critical` | BOOLEAN | NOT NULL, DEFAULT FALSE             | Indicates if gateway is critical for operations |
| `gateway_functionality_type` | TEXT | NOT NULL, DEFAULT 'regular'         | Type of gateway: 'regular', 'ai', 'event'            |
| `is_active` | BOOLEAN | NOT NULL, DEFAULT FALSE             | Indicates if gateway is currently connected via WebSocket |
| `created_at` | DATETIME | NOT NULL, DEFAULT CURRENT_TIMESTAMP | Registration timestamp                                    |
| `updated_at` | DATETIME | NOT NULL, DEFAULT CURRENT_TIMESTAMP | Last modification timestamp                               |

**Validation Rules**:

- **organization_id**:
  - Required
  - Must reference existing organization UUID
  - Foreign key constraint enforced

- **name**:
  - Required (non-empty after trim)
  - Pattern: `^[a-z0-9-]+$` (lowercase alphanumeric with hyphens)
  - Length: 3-64 characters
  - No leading or trailing hyphens
  - Must be unique within the organization (composite uniqueness)

- **display_name**:
  - Required (non-empty after trim)
  - Pattern: Allow any printable characters, spaces
  - Length: 1-128 characters
  - Whitespace trimmed before storage

- **description**:
  - Optional
  - Maximum length: 500 characters
  - Whitespace trimmed before storage

- **vhost**:
  - Required (non-empty after trim)
  - Pattern: Valid hostname/domain format
  - Length: 1-253 characters
  - Must be a valid domain name or IP address

- **is_critical**:
  - Required boolean value
  - Defaults to false if not specified
  - Indicates operational criticality

- **gateway_functionality_type**:
  - Required text value
  - Defaults to 'regular' if not specified
  - Enum values: 'regular', 'ai', 'event'
  - Case-sensitive validation
  - Determines gateway specialization and capabilities

- **is_active**:
  - System-managed boolean value
  - Defaults to false on creation
  - Updated automatically based on WebSocket connection status

**Indexes**:
- Primary key index on `uuid` (automatic)
- Composite unique index on `(organization_id, name)` for org-scoped uniqueness
- Foreign key index on `organization_id` for efficient queries

**Relationships**:
- Belongs to one `Organization` (N:1 relationship)
- Has many `GatewayToken` (1:N relationship)
- Cascade delete: When gateway deleted, all associated tokens are deleted
- Cascade delete: When organization deleted, all gateways and their tokens are deleted

### GatewayToken

Represents an authentication token for a gateway. Multiple active tokens per gateway enable zero-downtime rotation.

**Attributes**:

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `uuid` | TEXT | PRIMARY KEY, NOT NULL | Unique token identifier (UUID v7) |
| `gateway_uuid` | TEXT | FOREIGN KEY, NOT NULL | Reference to parent gateway |
| `token_hash` | TEXT | NOT NULL | SHA-256 hash of token (hex-encoded, 64 chars) |
| `salt` | TEXT | NOT NULL | Cryptographic salt (hex-encoded, 64 chars) |
| `status` | TEXT | NOT NULL, DEFAULT 'active' | Token status: 'active' or 'revoked' |
| `created_at` | DATETIME | NOT NULL, DEFAULT CURRENT_TIMESTAMP | Token creation timestamp |
| `revoked_at` | DATETIME | NULL | Timestamp when token was revoked (NULL if active) |

**Validation Rules**:

- **gateway_uuid**:
  - Required
  - Must reference existing gateway UUID
  - Foreign key constraint enforced

- **token_hash**:
  - Required
  - Exactly 64 hex characters (SHA-256 output)
  - Format: `^[a-f0-9]{64}$`

- **salt**:
  - Required
  - Exactly 64 hex characters (32 bytes hex-encoded)
  - Format: `^[a-f0-9]{64}$`

- **status**:
  - Required
  - Enum: `'active'` or `'revoked'`
  - Cannot transition from 'revoked' back to 'active'

- **revoked_at**:
  - NULL when status is 'active'
  - NOT NULL when status is 'revoked'
  - Must be >= created_at if set

**Indexes**:
- Primary key index on `uuid` (automatic)
- Composite index on `(gateway_uuid, status)` for efficient active token queries

**Relationships**:
- Belongs to one `Gateway` (N:1 relationship)
- Foreign key constraint: `gateway_uuid` REFERENCES `gateways(uuid)` ON DELETE CASCADE

**Constraints**:
- Maximum 2 active tokens per gateway (enforced at application layer)

## State Transitions

### Gateway States

Gateways have an implicit lifecycle based on existence:

```
[Non-existent] --register--> [Active] --delete--> [Deleted]
```

- **Non-existent**: Gateway not registered
- **Active**: Gateway registered within organization and operational (can have tokens)
- **Deleted**: Gateway removed (all tokens cascade deleted)

No explicit status field - existence determines state.

### Token States

Tokens have explicit status with controlled transitions:

```
                    rotate/issue
[Non-existent] -------------------> [Active]
                                       |
                                       | revoke
                                       ▼
                                   [Revoked]
```

**State Descriptions**:

- **Non-existent**: Token not yet created
- **Active** (`status='active'`):
  - Token can authenticate gateway
  - Can be revoked
  - Multiple tokens can be active simultaneously (max 2)
  - `revoked_at` is NULL

- **Revoked** (`status='revoked'`):
  - Token cannot authenticate gateway
  - Cannot be reactivated (terminal state)
  - `revoked_at` timestamp recorded
  - Revoke operation is idempotent

**Transition Rules**:
1. **Creation**: New tokens always created with `status='active'`
2. **Revocation**: `status='active'` → `status='revoked'`, set `revoked_at`
3. **No reactivation**: Once `status='revoked'`, cannot return to `active'
4. **Cascade deletion**: When parent gateway deleted, all tokens deleted regardless of status

## Domain Models (Go Structs)

### Gateway Model

```go
package model

import "time"

type Gateway struct {
    ID             string    `json:"id"`
    OrganizationID string    `json:"organizationId"`
    Name           string    `json:"name"`
    DisplayName    string    `json:"displayName"`
    Description    string    `json:"description"`
    Vhost          string    `json:"vhost"`
    IsCritical     bool      `json:"isCritical"`
    FunctionalityType    string    `json:"functionalityType"`
    IsActive       bool      `json:"isActive"`
    CreatedAt      time.Time `json:"createdAt"`
    UpdatedAt      time.Time `json:"updatedAt"`
}
```

### GatewayToken Model

```go
package model

import "time"

type GatewayToken struct {
    Id          string     `json:"id"`
    GatewayId   string     `json:"gatewayId"`
    TokenHash   string     `json:"-"` // Never expose in JSON responses
    Salt        string     `json:"-"` // Never expose in JSON responses
    Status      string     `json:"status"` // "active" or "revoked"
    CreatedAt   time.Time  `json:"createdAt"`
    RevokedAt   *time.Time `json:"revokedAt,omitempty"` // Pointer for NULL support
}

// IsActive returns true if token status is active
func (t *GatewayToken) IsActive() bool {
    return t.Status == "active"
}

// Revoke marks the token as revoked with current timestamp
func (t *GatewayToken) Revoke() {
    now := time.Now()
    t.Status = "revoked"
    t.RevokedAt = &now
}
```

## Data Transfer Objects (DTOs)

### Create Gateway Request

```go
package dto

type CreateGatewayRequest struct {
    Name        string `json:"name" binding:"required"`
    DisplayName string `json:"displayName" binding:"required"`
    Description string `json:"description,omitempty"`
    Vhost       string `json:"vhost" binding:"required"`
    IsCritical  *bool  `json:"isCritical" binding:"required"`
    FunctionalityType string `json:"functionalityType" binding:"required"`
}
```

### Gateway Registration and Gateway Response

```go
package dto

import "time"

type GatewayResponse struct {
    ID             string    `json:"id"`
    OrganizationID string    `json:"organizationId"`
    Name           string    `json:"name"`
    DisplayName    string    `json:"displayName"`
    Description    string    `json:"description,omitempty"`
    Vhost          string    `json:"vhost"`
    IsCritical     bool      `json:"isCritical"`
    FunctionalityType    string    `json:"functionalityType"`
    IsActive       bool      `json:"isActive"`
    CreatedAt      time.Time `json:"createdAt"`
    UpdatedAt      time.Time `json:"updatedAt"`
}
```

### Token Rotation Response

```go
package dto

import "time"

type TokenRotationResponse struct {
    Id        string    `json:"id"` // ID of new token
    Token     string    `json:"token"`      // Plain-text new token
    CreatedAt time.Time `json:"createdAt"`
    Message   string    `json:"message"`    // e.g., "New token generated. Old token remains active."
}
```

### Token Info Response

```go
package dto

import "time"

type TokenInfoResponse struct {
    Id        string     `json:"id"`
    Status    string     `json:"status"`
    CreatedAt time.Time  `json:"createdAt"`
    RevokedAt *time.Time `json:"revokedAt,omitempty"`
}
```

## Database Schema (SQLite)

```sql
-- Gateways table (scoped to organizations)
CREATE TABLE IF NOT EXISTS gateways (
    uuid TEXT PRIMARY KEY,
    organization_uuid TEXT NOT NULL,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT,
    vhost TEXT NOT NULL,
    is_critical BOOLEAN DEFAULT FALSE,
    gateway_functionality_type TEXT DEFAULT 'regular' NOT NULL,
    is_active BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, name),
    CHECK (gateway_functionality_type IN ('regular', 'ai', 'event'))
);

-- Gateway Tokens table
CREATE TABLE IF NOT EXISTS gateway_tokens (
    uuid TEXT PRIMARY KEY,
    gateway_uuid TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    salt TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    revoked_at DATETIME,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    CHECK (status IN ('active', 'revoked')),
    CHECK (revoked_at IS NULL OR status = 'revoked')
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_gateways_org
    ON gateways(organization_uuid);

CREATE INDEX IF NOT EXISTS idx_gateway_tokens_status
    ON gateway_tokens(gateway_uuid, status);

CREATE INDEX IF NOT EXISTS idx_gateway_tokens_created
    ON gateway_tokens(created_at DESC);
```

## Business Rules

1. **Gateway Name Uniqueness**: Gateway names must be unique within an organization
   - Different organizations can have gateways with the same name
   - Composite unique constraint on `(organization_id, name)`

2. **Organization Ownership**: Every gateway must belong to an organization
   - Cannot create gateway without valid organization_id
   - Deleting organization cascades to delete all its gateways and tokens

3. **Token Rotation Limit**: A gateway can have maximum 2 active tokens simultaneously
   - When issuing 3rd token while 2 are active, operation should fail with clear error
   - Administrator must revoke old tokens before generating new ones

4. **Token Exposure**: Plain-text token is only exposed once in response:
   - During initial gateway registration
   - During token rotation
   - Never retrievable after initial response

5. **Revocation Idempotency**: Revoking an already-revoked token succeeds without error
   - Returns success with message "Token already revoked"
   - Does not update `revoked_at` timestamp

6. **Gateway Deletion**: Deleting a gateway cascades to all tokens
   - Active tokens become invalid immediately
   - Foreign key cascade handles database cleanup

7. **Organization Deletion**: Deleting an organization cascades to all gateways and their tokens
   - All gateways within organization deleted
   - All tokens for those gateways deleted
   - Foreign key cascade handles cleanup

8. **Token Verification**: Only tokens with `status='active'` can authenticate
   - Revoked tokens always fail verification
   - Tokens from deleted gateways fail with "gateway not found"
   - Tokens from deleted organizations fail with "gateway not found"

## Validation Summary

**Service Layer Validations**:
- Organization existence check before gateway creation
- Gateway name format and length
- Display name length
- Gateway name uniqueness within organization
- Maximum active tokens per gateway (2)
- Token status transitions

**Database Layer Constraints**:
- Primary keys (UUIDs)
- Foreign keys (organization_id, gateway_uuid references)
- Composite unique constraint (organization_id, name)
- Check constraints (status enum, revoked_at consistency)
- Cascade deletes (organization → gateways → tokens)

## API Resource Structure

**Important**: Despite the database-level organization ownership, gateways are exposed as a **root resource** in the REST API:

```
POST   /api/v1/gateways                        # Register gateway
GET    /api/v1/gateways                        # List gateways (can filter by org_id)
GET    /api/v1/gateways/{gatewayId}            # Get gateway by UUID
GET    /api/v1/status/gateways               # Get gateway status of the gateways in a org can filter by gatewayId
PUT    /api/v1/gateways/{gatewayId}            # Update gateway
DELETE /api/v1/gateways/{gatewayId}            # Delete gateway

POST   /api/v1/gateways/{gatewayId}/tokens     # Rotate token
DELETE /api/v1/gateways/{uuid}/tokens/{token_uuid}  # Revoke token
```

**Rationale for Root Resource**:
- Gateways are first-class resources with independent operations
- Simplifies API structure (flat vs nested)
- organization_id passed in request body, not URL path
- Still enforces organization ownership via foreign key and validation

## References

- Feature specification: `spec.md`
- Implementation plan: `plan.md`
- API contract: `openapi-gateways.yaml`

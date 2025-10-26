# Gateway Management Implementation

**Last Updated**: October 26, 2025
**Authentication**: Thunder STS JWT (organization claim)
**Status**: Gateway Registration, Listing, Retrieval, Token Rotation, and Deletion implemented

## Overview

Gateway Management provides APIs for registering, managing, and deleting API gateways within organizations. All operations are scoped to the organization specified in the JWT token, ensuring complete multi-tenant isolation.

## Implementation Files

- **Handler**: `src/internal/handler/gateway.go` - HTTP request handling and routing
- **Service**: `src/internal/service/gateway.go` - Business logic and validation
- **Repository**: `src/internal/repository/gateway.go` - Database operations
- **Models**: `src/internal/model/gateway.go` - Domain entities
- **DTOs**: `src/internal/dto/gateway.go` - Request/response structures
- **Middleware**: `src/internal/middleware/auth.go` - JWT validation
- **Database**: `src/internal/database/schema.sql` - Schema definitions
- **API Spec**: `src/resources/openapi.yaml` - OpenAPI documentation

## API Endpoints

| Method | Endpoint | Description | Status |
|--------|----------|-------------|--------|
| POST | `/api/v1/gateways` | Register new gateway | ✅ Implemented |
| GET | `/api/v1/gateways` | List all gateways | ✅ Implemented |
| GET | `/api/v1/gateways/{id}` | Get gateway details | ✅ Implemented |
| PUT | `/api/v1/gateways/{id}` | Update gateway | ✅ Implemented |
| DELETE | `/api/v1/gateways/{id}` | Delete gateway | ✅ Implemented |
| POST | `/api/v1/gateways/{id}/tokens` | Rotate gateway token | ✅ Implemented |
| DELETE | `/api/v1/gateways/{id}/tokens/{tokenId}` | Revoke token | ⏳ Planned |

## Authentication & Authorization

### JWT Token Requirements

All gateway management endpoints require JWT authentication with an `organization` claim:

```http
Authorization: Bearer <jwt-token>
```

The organization ID is automatically extracted from the JWT token and used for all operations. Clients do not need to provide organization ID in request bodies or query parameters.

### Organization Isolation

- **Registration**: Gateways created in organization from JWT token
- **Listing**: Only returns gateways from user's organization
- **Retrieval**: Access validated against organization in JWT token
- **Updates**: Only gateways from user's organization can be modified
- **Deletion**: Only gateways from user's organization can be deleted
- **Token Operations**: Scoped to user's organization

## Features

### 1. Gateway Registration

**Endpoint**: `POST /api/v1/gateways`

**Behavior**:
1. Validates JWT token and extracts organization claim
2. Validates request payload (name, displayName, description, vhost)
3. Verifies organization exists
4. Prevents duplicate names within organization
5. Generates secure registration token
6. Returns gateway details with initial token (201 Created)

**Request Fields**:
- **name**: Lowercase alphanumeric with hyphens, 3-64 characters, pattern: `^[a-z0-9-]+$`
- **displayName**: 1-128 characters
- **description**: Optional text
- **vhost**: Required, valid hostname format

**Uniqueness**: Gateway names must be unique within an organization. Different organizations can use the same gateway name.

### 2. List Gateways

**Endpoint**: `GET /api/v1/gateways`

**Behavior**:
- Returns all gateways for organization from JWT token (200 OK)
- Constitution-compliant envelope structure with count, list, and pagination

### 3. Get Gateway Details

**Endpoint**: `GET /api/v1/gateways/{gatewayId}`

**Behavior**:
- Retrieves gateway details by ID (200 OK)
- Validates gateway belongs to organization from JWT token
- Returns 404 if not found or belongs to different organization

### 4. Token Rotation

**Endpoint**: `POST /api/v1/gateways/{gatewayId}/tokens`

**Behavior**:
- Generates new token while keeping existing tokens active (201 Created)
- Enforces maximum 2 active tokens per gateway
- Returns new token (only time it's visible in plain text)
- Old tokens remain valid until explicitly revoked
- Returns 400 if gateway already has 2 active tokens

### 5. Gateway Deletion

**Endpoint**: `DELETE /api/v1/gateways/{gatewayId}`

**Status**: ✅ User Story 1 Complete | ⏳ User Story 2 Pending

**Behavior**:
1. Validates JWT token and extracts organization claim
2. Validates UUID format for gateway ID
3. Verifies gateway exists and belongs to user's organization
4. Executes transaction-wrapped DELETE with organization isolation
5. Automatic CASCADE deletion of all gateway tokens
6. Returns 204 No Content on success
7. Idempotent operation (second delete returns 404)

**Security**:
- Organization filter enforced in database query
- Same 404 response for "not found" and "wrong organization" (prevents enumeration)
- All operations scoped to JWT token's organization claim

**Pending Features** (User Story 2):
- Pre-deletion validation for active API deployments
- Pre-deletion validation for active WebSocket connections
- 409 Conflict response when gateway has active dependencies

## Database

**Schema**: `src/internal/database/schema.sql`

**Tables**:
- `gateways` - Gateway entities with organization scoping
- `gateway_tokens` - Authentication tokens with CASCADE delete

**Key Constraints**:
- Composite unique constraint on `(organization_uuid, name)` prevents duplicate gateway names within organization
- CASCADE delete: Deleting organization removes all gateways; deleting gateway removes all tokens
- Token status validation: 'active' or 'revoked'

## Token Security

### Token Generation
- Cryptographically secure random tokens
- 32-byte length (64 hex characters)
- Generated using secure random number generator

### Token Storage
- Tokens hashed with SHA-256
- Unique salt per token
- Only hash and salt stored in database
- Plain-text token never stored

### Token Verification
- Constant-time comparison to prevent timing attacks
- Hash submitted token with stored salt
- Compare against stored hash
- **Token Rotation**: Only gateways from user's organization can have tokens rotated

### Handler Implementation

```go
func (h *GatewayHandler) CreateGateway(c *gin.Context) {
    // Extract organization from JWT token (not request body)
    organizationID, exists := middleware.GetOrganizationFromContext(c)
    if !exists {
        c.JSON(http.StatusUnauthorized, ...)
        return
    }
    
    // Use organizationID from token
    response, err := h.gatewayService.RegisterGateway(organizationID, req.Name, req.DisplayName)
    // ...
}
```

## Behaviour

### Gateway Registration

1. **Authentication**: Middleware validates JWT token and extracts organization claim
2. **Request Validation**: Validates presence of required fields:
   - `name`: lowercase alphanumeric with hyphens, 3-64 chars
   - `displayName`: 1-128 chars
   - `vhost`: virtual host for the gateway
   - `isCritical`: boolean indicating gateway criticality
   - `isAIGateway`: boolean indicating if this is an AI gateway
   - `description`: optional gateway description
3. **Organization Scoping**: Uses organization ID from JWT token (not request body)
4. Service confirms organization existence and prevents duplicate names within the same organization using composite unique constraint `(organization_id, name)`
5. System generates cryptographically secure 32-byte token using `crypto/rand`, hashes it with SHA-256 and unique 32-byte salt, stores hash and salt (never plain-text)
6. Response returns gateway details with initial registration token
7. Different organizations can register gateways with identical names

### Token Lifecycle
1. **Creation**: Generated during gateway registration or rotation
2. **Active**: Token can authenticate gateway requests
3. **Rotation**: New token created, old tokens remain active (max 2 active)
4. **Revocation**: Token marked as revoked, can no longer authenticate (planned)

## Error Responses

| Code | Message | Common Scenarios |
|------|---------|------------------|
| 400 | Bad Request | Validation failures, invalid UUID format, max tokens reached |
| 401 | Unauthorized | Missing/invalid JWT token, missing organization claim |
| 404 | Not Found | Gateway not found, organization not found, wrong organization |
| 409 | Conflict | Duplicate gateway name, active deployments/connections |
| 500 | Internal Server Error | Database errors, token generation failures |

## Key Design Decisions

1. Token rotation generates new token while keeping existing tokens active, enforces maximum 2 active tokens per gateway
2. Each token has UUID for tracking, creation timestamp, status (active/revoked), and optional revocation timestamp
3. Token verification compares submitted token against stored hashes using constant-time comparison (`crypto/subtle`) to prevent timing attacks
4. Future implementation: Token revocation updates status to 'revoked' and sets revocation timestamp (idempotent operation).

### Gateway Status Monitoring

1. **Lightweight Status API**: New endpoint `/api/v1/gateways/status` provides minimal gateway information for frequent polling
2. **Optional Filtering**: Query parameter `gatewayId` allows filtering to a specific gateway
3. **Response Structure**: Returns only essential fields (id, name, isActive, isCritical) for efficient polling
4. **Organization Scoping**: Automatically filtered by organization from JWT token

### Data Model

**Gateways Table:**
```sql
CREATE TABLE gateways (
    uuid TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT,
    vhost TEXT NOT NULL,
    is_critical BOOLEAN DEFAULT FALSE,
    is_ai_gateway BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT FALSE,
    created_at DATETIME,
    updated_at DATETIME,
    FOREIGN KEY (organization_id) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_id, name)
);
```

**Gateway Tokens Table:**
```sql
CREATE TABLE gateway_tokens (
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
```

## Key Technical Decisions

1. **JWT Authentication**: All endpoints require valid JWT token with `organization` claim for multi-tenant security
2. **Organization Scoping**: Organization ID extracted from JWT token, eliminating need for clients to provide it
3. **Automatic Isolation**: All queries automatically scoped to organization from token
4. **Composite Uniqueness**: Database constraint prevents race conditions in concurrent registration
5. **Token Security**: Strong hashing, unique salts, constant-time verification, secure random generation
6. **Zero-Downtime Rotation**: Maximum 2 active tokens allows overlap during gateway reconfiguration
7. **CASCADE Deletion**: Database handles deletion of dependent tokens automatically
8. **Constitution Compliance**: camelCase JSON properties, envelope list structures

## Testing Scenarios

### Duplicate Prevention
1. Register gateway with name "prod-gateway-01"
2. Attempt to register another gateway with same name in same organization
3. Expected: 409 Conflict error

### Max Tokens Enforcement
1. Rotate token once (2 active tokens: initial + new)
2. Rotate token again (attempt 3 active tokens)
3. Expected: 400 Bad Request error

### CASCADE Deletion Verification
1. Delete gateway via DELETE endpoint
2. Expected: 204 No Content
3. Verify all associated tokens are automatically deleted

## Related Documentation

- **OpenAPI Spec**: `src/resources/openapi.yaml`
- **Database Schema**: `src/internal/database/schema.sql`
- **Gateway Deletion Spec**: `/specs/004-gateway-deletion/spec.md`

## Related Features

- **Organization Management**: Gateways require valid organization
- **Project Management**: Similar organization-scoped uniqueness pattern
- **API Deployment**: APIs deployed to gateways (affects deletion validation)

## Future Enhancements

### Token Revocation (Planned)

**Endpoint**: `DELETE /api/v1/gateways/{gatewayId}/tokens/{tokenId}`

Immediate token revocation with idempotent behavior.

### Gateway Deletion Safety Checks (User Story 2)

- Pre-deletion validation for active API deployments
- Pre-deletion validation for active WebSocket connections
- 409 Conflict response with details when validation fails

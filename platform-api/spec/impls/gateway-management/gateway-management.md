# Gateway Management Implementation

**Last Updated**: October 19, 2025  
**Authentication**: Thunder STS JWT (organization claim)

## Entry Points

- `platform-api/src/internal/handler/gateway.go` – registers `/api/v1/gateways` routes for registration, listing, retrieval, and token rotation. **UPDATED**: Now extracts organization ID from JWT token instead of request body/query params.
- `platform-api/src/internal/middleware/auth.go` – validates JWT tokens and extracts organization claim for all gateway endpoints.
- `platform-api/src/internal/service/gateway.go` – handles validation, organization association, duplicate prevention, token generation/hashing, and token rotation logic with max 2 active tokens enforcement.
- `platform-api/src/internal/repository/gateway.go` – executes SQL CRUD operations for gateways and tokens, enforces composite unique constraint on (organization_id, name).
- `platform-api/src/internal/model/gateway.go` – defines Gateway and GatewayToken domain models with helper methods.
- `platform-api/src/internal/dto/gateway.go` – defines request/response DTOs with camelCase JSON serialization per project constitution. **UPDATED**: `CreateGatewayRequest` no longer requires `organizationId` field.
- `platform-api/src/internal/database/schema.sql` – defines `gateways` and `gateway_tokens` tables with foreign keys, constraints, and indexes.
- `platform-api/src/resources/openapi.yaml` – documents all gateway management endpoints and schemas with JWT Bearer authentication.

## Authentication & Authorization

### JWT Token Requirements

All gateway management endpoints (except health checks) require JWT authentication:

```http
Authorization: Bearer <jwt-token>
```

The JWT token must contain an `organization` claim with the user's organization UUID. This organization ID is automatically used for all gateway operations.

### Organization Isolation

- **Registration**: Gateways are created in the organization specified by JWT token
- **Listing**: Only gateways from user's organization are returned
- **Retrieval**: Access validated against organization in JWT token
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
2. **Request Validation**: Validates presence of `name` and `displayName` with format constraints (name: lowercase alphanumeric with hyphens, 3-64 chars; display name: 1-128 chars)
3. **Organization Scoping**: Uses organization ID from JWT token (not request body)
4. Service confirms organization existence and prevents duplicate names within the same organization using composite unique constraint `(organization_id, name)`
5. System generates cryptographically secure 32-byte token using `crypto/rand`, hashes it with SHA-256 and unique 32-byte salt, stores hash and salt (never plain-text)
6. Response returns gateway details with initial registration token
7. Different organizations can register gateways with identical names

### Token Lifecycle

1. Token rotation generates new token while keeping existing tokens active, enforces maximum 2 active tokens per gateway
2. Each token has UUID for tracking, creation timestamp, status (active/revoked), and optional revocation timestamp
3. Token verification compares submitted token against stored hashes using constant-time comparison (`crypto/subtle`) to prevent timing attacks
4. Future implementation: Token revocation updates status to 'revoked' and sets revocation timestamp (idempotent operation).

### Data Model

**Gateways Table:**
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

**Gateway Tokens Table:**
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

## Key Technical Decisions

1. **JWT Authentication**: All endpoints require valid JWT token with `organization` claim for multi-tenant security
2. **Organization Scoping**: Organization ID extracted from JWT token, eliminating need for clients to provide it in requests
3. **Automatic Isolation**: Handlers automatically scope queries to organization from token, preventing cross-organization access
4. **Composite Uniqueness**: Database constraint `UNIQUE(organization_id, name)` prevents race conditions in concurrent registration attempts within same organization
5. **Token Security**: SHA-256 hash with unique salt per token, constant-time verification, 32-byte tokens from `crypto/rand`, never store plain-text
6. **Zero-Downtime Rotation**: Maximum 2 active tokens allows overlap period where both old and new tokens work, administrators revoke old token after gateway reconfiguration
7. **Cascade Deletion**: Deleting organization cascades to gateways, deleting gateway cascades to tokens
8. **Constitution Compliance**: All API properties use camelCase, list endpoints return `{count, list, pagination}` envelope structure

## Verification

### Register Gateway

**Request** (organization ID from JWT token):
```bash
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H 'Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "prod-gateway-01",
    "displayName": "Production Gateway 01"
  }'
```

**Expected Response (201 Created):**
```json
{
  "id": "987e6543-e21b-45d3-a789-426614174999",
  "organizationId": "123e4567-e89b-12d3-a456-426614174000",
  "name": "prod-gateway-01",
  "displayName": "Production Gateway 01",
  "createdAt": "2025-10-14T10:30:00Z",
  "updatedAt": "2025-10-14T10:30:00Z"
}
```

### List All Gateways

**Request** (filters by organization from JWT token):
```bash
curl -k https://localhost:8443/api/v1/gateways \
  -H 'Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...'
```

**Expected Response (200 OK):**
```json
{
  "count": 2,
  "list": [
    {
      "id": "987e6543-e21b-45d3-a789-426614174999",
      "organizationId": "123e4567-e89b-12d3-a456-426614174000",
      "name": "prod-gateway-01",
      "displayName": "Production Gateway 01",
      "createdAt": "2025-10-14T10:30:00Z",
      "updatedAt": "2025-10-14T10:30:00Z"
    },
    {
      "id": "abc12345-f678-90de-f123-456789abcdef",
      "organizationId": "123e4567-e89b-12d3-a456-426614174000",
      "name": "staging-gateway-01",
      "displayName": "Staging Gateway 01",
      "createdAt": "2025-10-14T11:00:00Z",
      "updatedAt": "2025-10-14T11:00:00Z"
    }
  ],
  "pagination": {
    "total": 2,
    "offset": 0,
    "limit": 2
  }
}
```

### Get Gateway by ID

```bash
curl -k https://localhost:8443/api/v1/gateways/987e6543-e21b-45d3-a789-426614174999 \
  -H 'Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...'
```

**Expected Response (200 OK):**
```json
{
  "id": "987e6543-e21b-45d3-a789-426614174999",
  "organizationId": "123e4567-e89b-12d3-a456-426614174000",
  "name": "prod-gateway-01",
  "displayName": "Production Gateway 01",
  "createdAt": "2025-10-14T10:30:00Z",
  "updatedAt": "2025-10-14T10:30:00Z"
}
```

### Rotate Gateway Token

```bash
curl -k -X POST https://localhost:8443/api/v1/gateways/987e6543-e21b-45d3-a789-426614174999/tokens \
  -H 'Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...'
```

**Expected Response (201 Created):**
```json
{
  "tokenId": "def45678-g901-23hi-j456-789012klmnop",
  "token": "kR3mF9pL2vX8qN5wY7jK4sT1hU6gB0cD9aE8fI2mN5oP7qR3sT6uV9xY2zA5bC8e",
  "createdAt": "2025-10-15T14:20:00Z",
  "message": "New token generated successfully. Old token remains active until revoked."
}
```

### Authentication Error Responses

**Missing Token (401 Unauthorized):**
```json
{
  "code": 401,
  "message": "Unauthorized",
  "description": "Authorization header is required"
}
```

**Missing Organization Claim (401 Unauthorized):**
```json
{
  "code": 401,
  "message": "Unauthorized",
  "description": "Token missing required 'organization' claim"
}
```

### Duplicate Prevention Test

```bash
# Register first gateway
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "prod-gateway-01",
    "displayName": "Production Gateway 01"
  }'

# Attempt duplicate (should return 409 Conflict)
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "prod-gateway-01",
    "displayName": "Duplicate Gateway"
  }'
```

**Expected Response (409 Conflict):**
```json
{
  "code": 409,
  "message": "Conflict",
  "description": "gateway with name 'prod-gateway-01' already exists in this organization"
}
```

### Max Tokens Enforcement Test

```bash
# Rotate once (2 active tokens: initial + rotation 1)
curl -k -X POST https://localhost:8443/api/v1/gateways/987e6543-e21b-45d3-a789-426614174999/tokens \
  -H 'Authorization: Bearer <token>' \

# Rotate again (3 active tokens: initial + rotation 1 + rotation 2)
curl -k -X POST https://localhost:8443/api/v1/gateways/987e6543-e21b-45d3-a789-426614174999/tokens \
  -H 'Authorization: Bearer <token>' \

# Attempt third rotation (should return 400 Bad Request)
curl -k -X POST https://localhost:8443/api/v1/gateways/987e6543-e21b-45d3-a789-426614174999/tokens \
  -H 'Authorization: Bearer <token>' \
```

**Expected Response (400 Bad Request):**
```json
{
  "code": 400,
  "message": "Bad Request",
  "description": "maximum 2 active tokens allowed. Revoke old tokens before rotating"
}
```

## Error Responses

- **400 Bad Request**: Validation failures (missing fields, invalid format, max tokens reached)
- **401 Unauthorized**: Missing/invalid JWT token, missing organization claim
- **404 Not Found**: Gateway not found, organization not found
- **409 Conflict**: Duplicate gateway name within organization
- **500 Internal Server Error**: Database errors, token generation failures

## Design Artifacts

Detailed design and planning documents are available in the `artifacts/` directory:

- [Feature Specification](artifacts/spec.md) – Complete feature requirements, user stories, and acceptance scenarios
- [Implementation Plan](artifacts/plan.md) – Implementation phases, file structure, and constitution compliance
- [Data Model](artifacts/data-model.md) – Entity relationships, database schema, validation rules, and DTOs
- [API Contract](artifacts/openapi-gateways.yaml) – OpenAPI specification for gateway endpoints

## Related Features

- [Organization Management](../organization-management.md) – Gateways require valid organization ID
- [Project Management](../project-management.md) – Similar pattern of per-organization uniqueness

## Future Enhancements

### Phase 6: Token Revocation (Not Implemented)

Endpoint for immediate token revocation with idempotent behavior:

```bash
# Revoke specific token
curl -k -X DELETE https://localhost:8443/api/v1/gateways/987e6543-e21b-45d3-a789-426614174999/tokens/def45678-g901-23hi-j456-789012klmnop
```

### Phase 7: Input Validation Enhancements (Not Implemented)

Enhanced validation already in place for gateway name pattern (`^[a-z0-9-]+$`), length constraints (3-64 for name, 1-128 for display name), and leading/trailing hyphen prevention.

### Phase 8: Gateway Deletion (Not Implemented)

Endpoint for deleting gateways with cascade to all tokens:

```bash
# Delete gateway
curl -k -X DELETE https://localhost:8443/api/v1/gateways/987e6543-e21b-45d3-a789-426614174999
```

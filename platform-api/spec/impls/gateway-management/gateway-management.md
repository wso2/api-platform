# Gateway Management Implementation

**Last Updated**: July 3, 2026
**Authentication**: Thunder STS JWT (organization claim)
**Status**: Gateway Registration, Listing, Retrieval, Update, Token Rotation, Token Revocation, and Deletion implemented

## Overview

Gateway Management provides APIs for registering, managing, and deleting API gateways within organizations. All operations are scoped to the organization specified in the JWT token, ensuring complete multi-tenant isolation.

Gateways follow the platform-wide handle convention: `id` is the immutable, URL-safe handle (used as `{gatewayId}` in every path), `displayName` is the human-readable name, and the internal UUID is never exposed on the wire (it only appears on WebSocket events sent to the gateway itself).

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
| POST | `/api/v0.9/gateways` | Register new gateway | ✅ Implemented |
| GET | `/api/v0.9/gateways` | List all gateways | ✅ Implemented |
| GET | `/api/v0.9/gateways/{gatewayId}` | Get gateway details | ✅ Implemented |
| PUT | `/api/v0.9/gateways/{gatewayId}` | Update gateway | ✅ Implemented |
| DELETE | `/api/v0.9/gateways/{gatewayId}` | Delete gateway | ✅ Implemented |
| GET | `/api/v0.9/gateways/{gatewayId}/tokens` | List active tokens | ✅ Implemented |
| POST | `/api/v0.9/gateways/{gatewayId}/tokens` | Rotate gateway token | ✅ Implemented |
| DELETE | `/api/v0.9/gateways/{gatewayId}/tokens/{tokenId}` | Revoke token | ✅ Implemented |

`{gatewayId}` is the gateway's handle (e.g. `prod-gateway-01`), not its UUID.

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

**Endpoint**: `POST /api/v0.9/gateways`

**Behavior**:
1. Validates JWT token and extracts organization claim
2. Validates request payload (id/handle, displayName, description, endpoints, functionalityType)
3. Verifies organization exists
4. Prevents duplicate handles within organization
5. Returns gateway details (registration itself does not generate or return a token — call `POST /gateways/{gatewayId}/tokens` afterward to obtain one)

**Request Fields**:
- **id**: Optional. Lowercase alphanumeric with hyphens, 3-40 characters, pattern: `^[a-z0-9-]+$`. Auto-generated from `displayName` if omitted. Immutable after creation.
- **displayName**: Required, 1-128 characters
- **description**: Optional text
- **endpoints**: Required, array of full URL strings (network endpoints exposed by the gateway) — replaces the old single `vhost` field
- **functionalityType**: Required enum — `regular`, `ai`, or `event`
- **isCritical**: Optional boolean, defaults to `false`
- **version**: Optional, defaults to `1.0`

**Uniqueness**: Gateway handles must be unique within an organization. Different organizations can use the same gateway handle.

### 2. List Gateways

**Endpoint**: `GET /api/v0.9/gateways`

**Behavior**:
- Returns all gateways for organization from JWT token (200 OK)
- Constitution-compliant envelope structure with count, list, and pagination

### 3. Get Gateway Details

**Endpoint**: `GET /api/v0.9/gateways/{gatewayId}`

**Behavior**:
- Retrieves gateway details by ID (200 OK)
- Validates gateway belongs to organization from JWT token
- Returns 404 if not found or belongs to different organization

### 4. Token Rotation

**Endpoint**: `POST /api/v0.9/gateways/{gatewayId}/tokens`

**Behavior**:
- Generates new token while keeping existing tokens active (201 Created)
- Enforces maximum 2 active tokens per gateway
- Returns new token (only time it's visible in plain text)
- Old tokens remain valid until explicitly revoked
- Returns 400 if gateway already has 2 active tokens

### 5. Gateway Deletion

**Endpoint**: `DELETE /api/v0.9/gateways/{gatewayId}`

**Status**: ✅ Complete

**Behavior**:
1. Validates JWT token and extracts organization claim
2. Resolves the gateway by handle (`{gatewayId}`)
3. Verifies gateway exists and belongs to user's organization
4. Executes transaction-wrapped DELETE with organization isolation
5. Automatic CASCADE deletion of:
   - All gateway tokens
   - All deployments
   - All deployment status entries
   - All association mappings (via explicit delete and artifact cascade)
6. Returns 204 No Content on success
7. Idempotent operation (second delete returns 404)

**Security**:
- Organization filter enforced in database query
- Same 404 response for "not found" and "wrong organization" (prevents enumeration)
- All operations scoped to JWT token's organization claim

**Database CASCADE Behavior**:
- Gateway deletion cascades automatically via foreign key constraints
- No pre-deletion validation required - database ensures referential integrity
- Association mappings are explicitly deleted before gateway deletion

## Database

**Schema**: `src/internal/database/schema.sql`

**Tables**:
- `gateways` - Gateway entities with organization scoping (handle + display name; no `vhost` column)
- `gateway_endpoints` - Network endpoints exposed by a gateway (one row per URL, replaces the old single `vhost` column)
- `gateway_tokens` - Authentication tokens with CASCADE delete

**Key Constraints**:
- Composite unique constraint on `(organization_uuid, handle)` prevents duplicate gateway handles within organization
- CASCADE delete: Deleting organization removes all gateways and related data
- CASCADE delete: Deleting gateway removes all tokens, endpoints, deployments, and deployment status
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
func (h *GatewayHandler) CreateGateway(w http.ResponseWriter, r *http.Request) {
    // Extract organization from JWT token (not request body)
    orgId, exists := middleware.GetOrganizationFromRequest(r)
    if !exists {
        httputil.WriteJSON(w, http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
            "Organization claim not found in token"))
        return
    }

    var req api.CreateGatewayRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httputil.WriteJSON(w, http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
        return
    }
    // ...
}
```

## Behaviour

### Gateway Registration

1. **Authentication**: Middleware validates JWT token and extracts organization claim
2. **Request Validation**: Validates presence of required fields:
   - `id`: optional handle, lowercase alphanumeric with hyphens, 3-40 chars (auto-generated from `displayName` if omitted)
   - `displayName`: 1-128 chars
   - `endpoints`: required array of full endpoint URLs
   - `isCritical`: boolean indicating gateway criticality
   - `functionalityType`: enum value (required) - one of "regular", "ai", "event"
   - `description`: optional gateway description
3. **Gateway Type Validation**: Uses global constants from `constants.go` to validate enum values
4. **Organization Scoping**: Uses organization ID from JWT token (not request body)
5. Service confirms organization existence and prevents duplicate handles within the same organization using composite unique constraint `(organization_uuid, handle)`
6. **Default Values**: New gateways default to `isActive: false` until WebSocket connection is established
7. Registration does not generate a token — call `POST /gateways/{gatewayId}/tokens` separately, which generates a cryptographically secure 32-byte token via `crypto/rand`, hashes it with SHA-256 and a unique 32-byte salt, and stores only the hash and salt (never plain-text)
8. Different organizations can register gateways with identical handles

### Token Lifecycle
1. **Creation**: Generated during gateway registration or rotation
2. **Active**: Token can authenticate gateway requests
3. **Rotation**: New token created, old tokens remain active (max 2 active)
4. **Revocation**: Token marked as revoked, can no longer authenticate; implemented and idempotent

## Error Responses

| Code | Message | Common Scenarios |
|------|---------|------------------|
| 400 | Bad Request | Validation failures, max tokens reached |
| 401 | Unauthorized | Missing/invalid JWT token, missing organization claim |
| 404 | Not Found | Gateway not found, organization not found, wrong organization |
| 409 | Conflict | Duplicate gateway handle, active deployments/connections |
| 500 | Internal Server Error | Database errors, token generation failures |

## Key Design Decisions

1. Token rotation generates new token while keeping existing tokens active, enforces maximum 2 active tokens per gateway
2. Each token has UUID for tracking, creation timestamp, status (active/revoked), and optional revocation timestamp
3. Token verification compares submitted token against stored hashes using constant-time comparison (`crypto/subtle`) to prevent timing attacks
4. Token revocation (`DELETE /gateways/{gatewayId}/tokens/{tokenId}`) updates status to 'revoked' and sets revocation timestamp; implemented and idempotent.

### WebSocket Connection Status Management

1. **Automatic Status Updates**: Gateway `isActive` status is automatically managed by WebSocket connection lifecycle
2. **Connection Established**: When gateway establishes WebSocket connection, `isActive` is set to `true`
3. **Connection Closed**: When gateway disconnects (graceful or unexpected), `isActive` is set to `false`
4. **Registration Default**: New gateways start with `isActive: false` until first connection
5. **Real-time Tracking**: Status reflects actual gateway connectivity in real-time
6. **Read-only Property**: `isActive` cannot be manually set via API - only managed by connection events

### Data Model

**Gateways Table:**
```sql
CREATE TABLE gateways (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    version VARCHAR(30) NOT NULL DEFAULT '1.0',
    gateway_functionality_type VARCHAR(20) NOT NULL DEFAULT 'regular',
    properties BLOB NOT NULL,
    manifest BLOB,
    is_active INTEGER DEFAULT 0,
    is_critical INTEGER DEFAULT 0,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- Network endpoints exposed by the gateway (replaces the old single `vhost` column)
CREATE TABLE gateway_endpoints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    gateway_uuid VARCHAR(40) NOT NULL,
    url VARCHAR(255) NOT NULL,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE
);
```

**Gateway Type Constants (from constants.go):**
```go
const (
   GatewayFunctionalityTypeRegular = "regular"
   GatewayFunctionalityTypeAI      = "ai"
   GatewayFunctionalityTypeEvent   = "event"
)

var ValidGatewayFunctionalityType = map[string]bool{
   GatewayFunctionalityTypeRegular: true,
   GatewayFunctionalityTypeAI:      true,
   GatewayFunctionalityTypeEvent:   true,
}

const DefaultGatewayFunctionalityType = GatewayFunctionalityTypeRegular
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
1. Register gateway with handle "prod-gateway-01"
2. Attempt to register another gateway with the same handle in the same organization
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

## Verification

### Register Gateway

**Request** (organization ID from JWT token):
```bash
curl -k -X POST https://localhost:9243/api/v0.9/gateways \
  -H 'Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...' \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "prod-gateway-01",
    "displayName": "Production Gateway 01",
    "description": "Primary production gateway for API traffic",
    "endpoints": ["https://api.example.com:8443/api/v1"],
    "isCritical": true,
    "functionalityType": "regular"
  }'
```

**Expected Response (201 Created):**
```json
{
  "id": "prod-gateway-01",
  "organizationId": "acme",
  "displayName": "Production Gateway 01",
  "description": "Primary production gateway for API traffic",
  "endpoints": ["https://api.example.com:8443/api/v1"],
  "isCritical": true,
  "functionalityType": "regular",
  "version": "1.0",
  "isActive": false,
  "createdAt": "2026-06-21T10:30:00Z",
  "updatedAt": "2026-06-21T10:30:00Z"
}
```

### List All Gateways

**Request** (filters by organization from JWT token):
```bash
curl -k https://localhost:9243/api/v0.9/gateways \
  -H 'Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...'
```

**Expected Response (200 OK):**
```json
{
  "count": 2,
  "list": [
    {
      "id": "prod-gateway-01",
      "organizationId": "acme",
      "displayName": "Production Gateway 01",
      "description": "Primary production gateway for API traffic",
      "endpoints": ["https://api.example.com:8443/api/v1"],
      "isCritical": true,
      "functionalityType": "regular",
      "version": "1.0",
      "isActive": true,
      "createdAt": "2026-06-21T10:30:00Z",
      "updatedAt": "2026-06-21T10:30:00Z"
    },
    {
      "id": "ai-gateway-01",
      "organizationId": "acme",
      "displayName": "AI Gateway 01",
      "description": "AI workloads gateway",
      "endpoints": ["https://ai-api.example.com:8443/api/v1"],
      "isCritical": false,
      "functionalityType": "ai",
      "version": "1.0",
      "isActive": false,
      "createdAt": "2026-06-21T11:00:00Z",
      "updatedAt": "2026-06-21T11:00:00Z"
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
curl -k https://localhost:9243/api/v0.9/gateways/prod-gateway-01 \
  -H 'Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...'
```

**Expected Response (200 OK):**
```json
{
  "id": "prod-gateway-01",
  "organizationId": "acme",
  "displayName": "Production Gateway 01",
  "description": "Primary production gateway for API traffic",
  "endpoints": ["https://api.example.com:8443/api/v1"],
  "isCritical": true,
  "functionalityType": "regular",
  "version": "1.0",
  "isActive": true,
  "createdAt": "2026-06-21T10:30:00Z",
  "updatedAt": "2026-06-21T10:30:00Z"
}
```

### Rotate Gateway Token

```bash
curl -k -X POST https://localhost:9243/api/v0.9/gateways/prod-gateway-01/tokens \
  -H 'Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...'
```

**Expected Response (201 Created):**
```json
{
  "id": "def45678-1234-4567-89ab-789012cdefgh",
  "token": "kR3mF9pL2vX8qN5wY7jK4sT1hU6gB0cD9aE8fI2mN5oP7qR3sT6uV9xY2zA5bC8e",
  "createdAt": "2026-06-21T14:20:00Z",
  "message": "New token generated successfully. Old token remains active until revoked."
}
```

### Revoke Gateway Token

```bash
curl -k -X DELETE https://localhost:9243/api/v0.9/gateways/prod-gateway-01/tokens/def45678-1234-4567-89ab-789012cdefgh \
  -H 'Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...'
```

**Expected Response:** `204 No Content`. Revoking an already-revoked token is idempotent.

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
curl -k -X POST https://localhost:9243/api/v0.9/gateways \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "prod-gateway-01",
    "displayName": "Production Gateway 01",
    "endpoints": ["https://api.example.com:8443/api/v1"],
    "functionalityType": "regular"
  }'

# Attempt duplicate handle (should return 409 Conflict)
curl -k -X POST https://localhost:9243/api/v0.9/gateways \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "prod-gateway-01",
    "displayName": "Duplicate Gateway",
    "endpoints": ["https://api.example.com:8443/api/v1"],
    "functionalityType": "regular"
  }'
```

**Expected Response (409 Conflict):**
```json
{
  "code": 409,
  "message": "Conflict",
  "description": "gateway with handle 'prod-gateway-01' already exists in this organization"
}
```

### Max Tokens Enforcement Test

```bash
# Rotate once (2 active tokens: initial + rotation 1)
curl -k -X POST https://localhost:9243/api/v0.9/gateways/prod-gateway-01/tokens \
  -H 'Authorization: Bearer <token>'

# Rotate again (3 active tokens: initial + rotation 1 + rotation 2)
curl -k -X POST https://localhost:9243/api/v0.9/gateways/prod-gateway-01/tokens \
  -H 'Authorization: Bearer <token>'

# Attempt third rotation (should return 400 Bad Request)
curl -k -X POST https://localhost:9243/api/v0.9/gateways/prod-gateway-01/tokens \
  -H 'Authorization: Bearer <token>'
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

- **Organization Management**: Gateways require valid organization
- **Project Management**: Similar organization-scoped uniqueness pattern
- **API Deployment**: APIs deployed to gateways (affects deletion validation)

## Future Enhancements

### Gateway Deletion Safety Checks (Not Yet Implemented)

`src/resources/openapi.yaml` documents `DELETE /gateways/{gatewayId}` as returning `409 Conflict`
when the gateway has active API deployments or active WebSocket connections
(`constants.ErrGatewayHasDeployments` and the `activeConnections`/`activeDeployments` examples
in the spec). As of this writing, `GatewayService.DeleteGateway` and `GatewayRepo.Delete`
(`src/internal/service/gateway.go`, `src/internal/repository/gateway.go`) do not perform either
check — deletion unconditionally removes artifact-gateway mappings and cascades tokens/deployments
via foreign keys. The handler (`src/internal/handler/gateway.go`) also has a dead branch for
`constants.ErrGatewayHasAssociatedAPIs`, which is defined in `constants/error.go` but never
returned by the service. Treat the openapi.yaml description of pre-deletion validation as
aspirational until this guard is implemented.

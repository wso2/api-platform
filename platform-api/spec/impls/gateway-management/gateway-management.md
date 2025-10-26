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

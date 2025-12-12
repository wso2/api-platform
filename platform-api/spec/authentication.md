# Authentication Specification - Thunder STS Integration

**Version**: 1.0  
**Last Updated**: October 19, 2025  
**Status**: Implemented

---

## Overview

The Platform API uses JWT (JSON Web Token) based authentication provided by **Thunder STS (Security Token Service)**. All API operations (except organization registration) require a valid JWT access token in the Authorization header.

---

## Authentication Architecture

### Thunder STS Integration

The Platform API integrates with Thunder STS, an external Security Token Service that issues JWT access tokens for authenticated users.

**Key Components**:
- **Thunder STS**: External identity provider that issues JWT tokens
- **Platform API Middleware**: JWT validation and claims extraction
- **Organization Claim**: JWT token contains user's organization UUID

### JWT Token Flow

```
┌─────────┐        ┌─────────────┐        ┌──────────────┐
│  User   │───1───>│ Thunder STS │───2───>│  User Agent  │
└─────────┘        └─────────────┘        └──────────────┘
                          │                       │
                          │ 3. JWT Token          │
                          └───────────────────────┘
                                                  │
                                                  │ 4. API Request
                                                  │    + Bearer Token
                                                  ↓
                          ┌───────────────────────────────┐
                          │     Platform API              │
                          │  ┌─────────────────────────┐  │
                          │  │  Auth Middleware        │  │
                          │  │  - Extract Token        │  │
                          │  │  - Parse JWT            │  │
                          │  │  - Extract Organization │  │
                          │  │  - Validate Claims      │  │
                          │  └─────────────────────────┘  │
                          │              │                │
                          │              ↓                │
                          │  ┌─────────────────────────┐  │
                          │  │  Business Logic         │  │
                          │  │  - Use Organization ID  │  │
                          │  │  - Process Request      │  │
                          │  └─────────────────────────┘  │
                          └───────────────────────────────┘
```

---

## JWT Token Structure

### Required Claims

All JWT tokens issued by Thunder STS must contain the following claims:

```json
{
  "sub": "user-uuid",
  "email": "user@example.com",
  "firstName": "John",
  "lastName": "Doe",
  "username": "john.doe",
  "organization": "org-uuid-here",
  "scope": "openid profile api:read api:write",
  "aud": "platform-api",
  "iss": "thunder",
  "jti": "token-id",
  "exp": 1729900000,
  "iat": 1729800000
}
```

### Critical Claim: `organization`

The **`organization`** claim is **mandatory** and contains the UUID of the user's organization. This claim is used to:

1. **Scope all API operations** to the user's organization
2. **Enforce multi-tenant isolation** - users can only access resources in their organization
3. **Eliminate the need for organization ID in request parameters**

**Validation**: If the `organization` claim is missing, the middleware rejects the request with HTTP 401 Unauthorized.

---

## Authentication Configuration

### Environment Variables

```bash
# JWT Authentication Settings
JWT_SKIP_VALIDATION=true          # Skip signature validation (development only)
JWT_ISSUER=thunder                 # Expected token issuer
JWT_SKIP_PATHS=/health,/metrics    # Paths that skip authentication
JWT_SECRET_KEY=your-secret-key     # Secret key (when validation enabled)
```

### Development vs Production Mode

**Development Mode** (`JWT_SKIP_VALIDATION=true`):
- ✅ Token existence is checked
- ✅ Token is parsed as valid JWT structure
- ✅ Organization claim is extracted and validated
- ❌ Token signature is NOT validated
- ❌ Issuer is NOT validated  
- ❌ Expiry is NOT validated

**Production Mode** (`JWT_SKIP_VALIDATION=false`):
- ✅ Token existence is checked
- ✅ Token is parsed and validated
- ✅ Token signature is validated (RSA)
- ✅ Issuer is validated against `JWT_ISSUER`
- ✅ Expiry is validated
- ✅ Organization claim is extracted and validated

---

## API Request Format

### Authorization Header

All authenticated API requests must include the JWT token in the Authorization header:

```http
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Example Request

```bash
curl -X POST https://localhost:9243/api/v1/projects \
  -H "Authorization: Bearer <jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "My Project"}'
```

**Note**: Organization ID is **not required** in the request body or query parameters - it is automatically extracted from the JWT token.

---

## Organization Isolation

### Automatic Organization Scoping

All API operations are automatically scoped to the organization specified in the JWT token's `organization` claim. This provides:

1. **Security**: Users cannot access resources from other organizations
2. **Simplicity**: No need to pass organization ID in every request
3. **Consistency**: All endpoints follow the same authentication pattern

### Endpoint Changes

**Before JWT Authentication**:
```bash
# Organization ID required in URL
GET /api/v1/organizations/{orgId}

# Organization ID required in request body
POST /api/v1/projects
{"name": "Project", "organizationId": "org-123"}

# Organization ID required in query parameter
GET /api/v1/gateways?organizationId=org-123
```

**After JWT Authentication**:
```bash
# Organization ID from token
GET /api/v1/organizations
Authorization: Bearer <token>

# Organization ID from token
POST /api/v1/projects
Authorization: Bearer <token>
{"name": "Project"}

# Organization ID from token
GET /api/v1/gateways
Authorization: Bearer <token>
```

---

## Middleware Implementation

### Authentication Middleware

The authentication middleware (`src/internal/middleware/auth.go`) performs:

1. **Token Extraction**: Extract JWT from Authorization header
2. **Format Validation**: Verify "Bearer <token>" format
3. **JWT Parsing**: Parse JWT structure (with or without signature validation)
4. **Claims Extraction**: Extract all claims including `organization`
5. **Organization Validation**: Ensure `organization` claim exists
6. **Context Storage**: Store claims in Gin context for handlers

### Helper Functions

Handlers can extract claims using these helper functions:

```go
// Get organization ID from token
organizationID, exists := middleware.GetOrganizationFromContext(c)

// Get user ID (from 'sub' claim)
userID, exists := middleware.GetUserIDFromContext(c)

// Get username
username, exists := middleware.GetUsernameFromContext(c)

// Get all claims
claims, exists := middleware.GetClaimsFromContext(c)
```

---

## Error Responses

### 401 Unauthorized - Missing Token

```json
{
  "code": 401,
  "message": "Unauthorized",
  "description": "Authorization header is required"
}
```

### 401 Unauthorized - Invalid Format

```json
{
  "code": 401,
  "message": "Unauthorized",
  "description": "Invalid authorization header format. Expected: Bearer <token>"
}
```

### 401 Unauthorized - Invalid JWT

```json
{
  "code": 401,
  "message": "Unauthorized",
  "description": "Invalid JWT format: ..."
}
```

### 401 Unauthorized - Missing Organization Claim

```json
{
  "code": 401,
  "message": "Unauthorized",
  "description": "Token missing required 'organization' claim"
}
```

---

## Security Considerations

### Token Validation

**Current Implementation**:
- Development mode skips signature validation for easier testing
- Production mode will validate signatures using JWKS endpoint

**Future Enhancements**:
- JWKS (JSON Web Key Set) endpoint integration for public key retrieval
- Token expiry validation
- Token revocation list support
- Refresh token mechanism

### Multi-Tenant Security

**Organization Isolation**:
- All database queries scoped to organization from token
- Service layer validates resource ownership
- Cross-organization access automatically prevented

**Access Control**:
- JWT `scope` claim can be used for role-based access control
- `RequireScope()` middleware available for endpoint-level authorization
- `RequireOrganization()` middleware for explicit organization checks

---

## Testing

### Development Testing

For development, you can create test JWT tokens at https://jwt.io:

```json
{
  "sub": "test-user-123",
  "email": "test@example.com",
  "firstName": "Test",
  "lastName": "User",
  "username": "testuser",
  "organization": "your-org-uuid-here",
  "scope": "openid profile api:read api:write",
  "aud": "platform-api",
  "iss": "thunder",
  "jti": "test-token",
  "exp": 9999999999,
  "iat": 1729800000
}
```

With `JWT_SKIP_VALIDATION=true`, the signature doesn't need to be valid.

### Integration Testing

For integration tests:
1. Mock Thunder STS to issue test tokens
2. Use valid JWT structure with test claims
3. Verify organization isolation between test cases

---

## Migration Guide

### For API Clients

**Projects**:
```bash
# Before
POST /api/v1/projects
{"name": "Project", "organizationId": "org-123"}

# After  
POST /api/v1/projects
Authorization: Bearer <token>
{"name": "Project"}
```

**Gateways**:
```bash
# Before
POST /api/v1/gateways
{"name": "gateway", "organizationId": "org-123"}
GET /api/v1/gateways?organizationId=org-123

# After
POST /api/v1/gateways
Authorization: Bearer <token>
{"name": "gateway"}

GET /api/v1/gateways
Authorization: Bearer <token>
```

**Organizations**:
```bash
# Before
GET /api/v1/organizations/org-123

# After
GET /api/v1/organizations
Authorization: Bearer <token>
```

### For Developers

**Handler Pattern**:
```go
func (h *Handler) CreateResource(c *gin.Context) {
    // Extract organization from token
    organizationID, exists := middleware.GetOrganizationFromContext(c)
    if !exists {
        c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
            "Organization claim not found in token"))
        return
    }
    
    // Use organizationID in business logic
    resource, err := h.service.Create(organizationID, req)
    // ...
}
```

---

## References

- **JWT Specification**: RFC 7519
- **Thunder STS Documentation**: [Internal Link]
- **Implementation Guide**: `/JWT_USAGE_GUIDE.md`
- **OpenAPI Specification**: `/src/resources/openapi.yaml`

---

**Document Status**: ✅ Implemented and Documented


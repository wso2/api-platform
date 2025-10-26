# Gateway Management - Implementation Plan

**Last Updated**: October 26, 2025

## Overview

This document outlines the implementation plan and current status for gateway management with enhanced properties,
token-based authentication, and status monitoring capabilities. The implementation includes gateway registration,
token rotation/revocation, status polling, and comprehensive gateway metadata management.
Gateways are scoped to organizations at the database level but exposed as root API resources.

**Key Enhancements Implemented**:
- Gateway criticality indicators (`isCritical`)
- Gateway type classification (`functionalityType`) supporting regular, ai, and event gateways
- Real-time connection status (`isActive`)
- Virtual host configuration (`vhost`)
- Gateway descriptions (`description`)
- Lightweight status polling endpoint (`/status/gateways`)
- Enhanced API filtering by project
- Comprehensive CRUD operations

## Constitution Compliance Check

###  I. Specification-First Development

- PRD: Complete in `spec.md` with 28 functional requirements
- Architecture impact: Documented in this plan (follows existing layered pattern)
- Design decisions: Captured in `research.md`
- Implementation notes: Will be created in `platform-api/spec/impls/gateway-registration.md`
- API contracts: Defined in `contracts/openapi-gateways.yaml`

###  II. Layered Architecture

Implementation follows existing pattern:
- **Handler Layer**: `src/internal/handler/gateway.go`
- **Service Layer**: `src/internal/service/gateway.go`
- **Repository Layer**: `src/internal/repository/gateway.go`
- **Models**: `src/internal/model/gateway.go`
- **DTOs**: `src/internal/dto/gateway.go`

###  III. Security by Default

- Tokens hashed with SHA-256 + unique salt (not reversible)
- Plain-text tokens never stored in database
- Constant-time comparison prevents timing attacks
- All endpoints over HTTPS (platform-api enforces TLS)
- License headers required on all new files

###  IV. Documentation Traceability

- spec.md � plan.md � impl notes (this document)
- Code paths documented in implementation section below
- curl verification commands in implementation notes
- OpenAPI contract synced with implementation

###  V. RESTful API Standards

- Endpoints under `/api/v1/gateways`
- Standard HTTP methods (POST, GET, DELETE)
- Resource-oriented URLs
- JSON content type
- Proper status codes (201, 200, 204, 400, 404, 409, 500)
- **OpenAPI Property Naming**: All schema properties use camelCase (e.g., `organizationId`, `displayName`, `createdAt`)
- **List Response Structure**: GET /gateways returns envelope: `{ "count": N, "list": [...], "pagination": {...} }`

###  VI. Data Integrity

- Schema defined in `src/internal/database/schema.sql`
- Foreign key constraints (organization_id, gateway_uuid)
- Composite unique constraint (organization_id, name)
- Cascade deletes (organization � gateways � tokens)
- Check constraints (status enum, revoked_at consistency)
- Timestamps (created_at, updated_at)

###  VII. Container-First Operations

- No changes to container configuration required
- Uses existing SQLite database
- No new environment variables needed
- Health check endpoint already exists

## Technical Context

### Technology Stack

- **Language**: Go 1.24
- **Framework**: Gin (HTTP router)
- **Database**: SQLite with schema migrations
- **Crypto**: Standard library (`crypto/sha256`, `crypto/rand`, `crypto/subtle`)
- **UUID**: `github.com/google/uuid` (already in use)

### Dependencies

**Existing (No Changes Required)**:
- `github.com/gin-gonic/gin` - HTTP routing
- `github.com/google/uuid` - UUID generation
- SQLite driver (already configured)

**Standard Library (No External Dependencies)**:
- `crypto/sha256` - Token hashing
- `crypto/rand` - Secure random generation
- `encoding/base64` - Token encoding
- `encoding/hex` - Hash/salt encoding for storage
- `crypto/subtle` - Constant-time comparison

### Integration Points

- **Organizations**: Foreign key relationship, must validate organization exists
- **Database Schema**: Add new tables to existing `schema.sql`
- **OpenAPI Spec**: Merge gateway endpoints into `src/resources/openapi.yaml`
- **Server Wiring**: Register routes in `src/internal/server/server.go`

## Implementation Phases

## Implementation Phases

### Phase 1: Database Schema

**Location**: `platform-api/src/internal/database/schema.sql`

**Add Tables**:
```sql
-- Gateways table (scoped to organizations)
CREATE TABLE IF NOT EXISTS gateways (
    uuid TEXT PRIMARY KEY,
    organization_uuid TEXT NOT NULL,           -- Updated column name
    name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT,                          -- Optional description
    vhost TEXT NOT NULL,                       -- Virtual host configuration
    is_critical BOOLEAN DEFAULT FALSE,        --  Criticality indicator
    gateway_functionality_type TEXT DEFAULT 'regular' NOT NULL, -- Gateway type: regular, ai, event
    is_active BOOLEAN DEFAULT FALSE,          --  Connection status
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
```

### Phase 2: Models & DTOs

**Create**: `platform-api/src/internal/model/gateway.go`

**Create**: `platform-api/src/internal/dto/gateway.go`

See data-model.md for complete struct definitions.

### Phase 3: Repository Layer

**Create**: `platform-api/src/internal/repository/gateway.go`

**Interface**:
```go
type GatewayRepository interface {
    // Gateway operations
    Create(gateway *model.Gateway) error
    GetByUUID(gatewayId string) (*model.Gateway, error)
    GetByOrganizationID(orgID string) ([]*model.Gateway, error)
    GetByNameAndOrgID(name, orgID string) (*model.Gateway, error)
    List() ([]*model.Gateway, error)
    Delete(gatewayId string) error
    UpdateGateway(gateway *model.Gateway) error                             
    UpdateActiveStatus(gatewayId string, isActive bool) error
    GetGatewayStatus(orgID string, gatewayId *string) (*dto.GatewayStatusListResponse, error)

    // Token operations
    CreateToken(token *model.GatewayToken) error
    GetActiveTokensByGatewayUUID(gatewayId string) ([]*model.GatewayToken, error)
    GetTokenByUUID(tokenId string) (*model.GatewayToken, error)
    RevokeToken(tokenId string) error
    CountActiveTokens(gatewayId string) (int, error)
}
```

### Phase 4: Service Layer

**Create**: `platform-api/src/internal/service/gateway.go`

**Key Methods**:
```go
// Enhanced gateway registration with new properties
func (s *GatewayService) RegisterGateway(orgID, name, displayName, description, vhost string, isCritical bool, functionalityType string) (*dto.GatewayRegistrationResponse, error)

// Gateway retrieval and listing
func (s *GatewayService) GetGateway(gatewayId, orgId string) (*dto.GatewayResponse, error)
func (s *GatewayService) ListGateways(organizationID *string) (*dto.GatewayListResponse, error)

// Gateway status polling (lightweight for frequent calls)
func (s *GatewayService) GetGatewayStatus(orgID string, gatewayId *string) (*dto.GatewayStatusListResponse, error)

// Gateway metadata updates
func (s *GatewayService) UpdateGateway(gatewayId, orgId string, description, displayName *string, isCritical *bool) (*dto.GatewayResponse, error)

// Gateway lifecycle management
func (s *GatewayService) DeleteGateway(gatewayId, orgId string) error

// Token management
func (s *GatewayService) RotateToken(gatewayUUID, orgId string) (*dto.TokenRotationResponse, error)
func (s *GatewayService) RevokeToken(gatewayUUID, tokenUUID, orgId string) error
func (s *GatewayService) VerifyToken(plainToken string) (*model.Gateway, error)
```

**Create**: `platform-api/src/internal/handler/gateway.go`

**Endpoints**:
```go
// Enhanced gateway management endpoints
func (h *GatewayHandler) CreateGateway(c *gin.Context)     // POST /api/v1/gateways
func (h *GatewayHandler) ListGateways(c *gin.Context)      // GET /api/v1/gateways  
func (h *GatewayHandler) GetGateway(c *gin.Context)        // GET /api/v1/gateways/{gatewayId}
func (h *GatewayHandler) UpdateGateway(c *gin.Context)     // PUT /api/v1/gateways/{gatewayId}
func (h *GatewayHandler) DeleteGateway(c *gin.Context)     // DELETE /api/v1/gateways/{gatewayId}

// Lightweight status polling endpoint
func (h *GatewayHandler) GetGatewayStatus(c *gin.Context)  // GET /api/v1/status/gateways

// Token management endpoints
func (h *GatewayHandler) RotateToken(c *gin.Context)       // POST /api/v1/gateways/{gatewayId}/tokens
func (h *GatewayHandler) RevokeToken(c *gin.Context)       // DELETE /api/v1/gateways/{gatewayId}/tokens/{tokenId}
```

### Phase 6: Server Wiring

**Modify**: `platform-api/src/internal/server/server.go`

Wire dependencies and register routes.

**Integration**:
- ✅ Dependency injection for GatewayHandler, GatewayService, and GatewayRepository
- ✅ Route registration for all gateway endpoints under `/api/v1/gateways`
- ✅ JWT middleware integration for organization-scoped access control
- ✅ Proper error handling and response formatting

### Phase 7: OpenAPI Documentation

**Main OpenAPI Spec** (`platform-api/src/resources/openapi.yaml`):
**Modify**: `platform-api/src/resources/openapi.yaml`

Merge endpoints from `contracts/openapi-gateways.yaml`.

### Phase 8: Implementation Documentation

**Create**: `platform-api/spec/prds/gateway-registration.md`

**Create**: `platform-api/spec/impls/gateway-registration.md`

**Gateway Management Implementation** (`spec/impls/gateway-management/gateway-management.md`):
- ✅ Complete implementation overview with entry points
- ✅ Enhanced behavior documentation including new properties
- ✅ Updated verification examples with curl commands
- ✅ Comprehensive error response documentation
- ✅ Gateway status monitoring section for polling capabilities

**Data Model Specification** (`spec/impls/gateway-management/artifacts/data-model.md`):
- ✅ Updated entity relationship diagrams
- ✅ Enhanced Gateway entity with all new properties
- ✅ Comprehensive validation rules for new fields
- ✅ Database schema documentation

**Implementation Plan** (`spec/impls/gateway-management/artifacts/plan.md`):
- ✅ This document - updated to reflect completed status
- ✅ Phase-by-phase completion tracking
- ✅ Enhanced feature documentation

**OpenAPI Specifications**:
- ✅ Gateway-specific OpenAPI contract with complete schemas
- ✅ Main platform OpenAPI integration
- ✅ Request/response examples updated

## File Structure Summary

```
platform-api/
   src/
      internal/
         database/
            schema.sql                    [MODIFY] Add gateway tables
         model/
            gateway.go                    [CREATE] Domain models
         dto/
            gateway.go                    [CREATE] DTOs
         repository/
            gateway.go                    [CREATE] Data access
         service/
            gateway.go                    [CREATE] Business logic
         handler/
            gateway.go                    [CREATE] HTTP handlers
         server/
             server.go                     [MODIFY] Wire dependencies
      resources/
          openapi.yaml                      [MODIFY] Add endpoints
   spec/
       prds/
          gateway-registration.md           [CREATE] Requirements
       impls/
           gateway-registration.md           [CREATE] Implementation notes
```

## Testing Strategy

### Unit Tests
- Service layer: Token generation, validation, business logic
- Repository layer: Database operations, constraints

### Integration Tests
- End-to-end flows: Register � Rotate � Revoke � Delete
- Cascade deletes
- Concurrent registrations

### Manual Verification
curl commands in implementation doc (`platform-api/spec/impls/gateway-registration.md`)

## Success Criteria Checklist

**Core Gateway Management Requirements**:
From spec.md:

-  SC-001: Duplicate name prevention (unique constraint)
-  SC-002: Token authentication works
-  SC-003: Records persist correctly
-  SC-004: Both tokens work during rotation
-  SC-005: Old tokens can be revoked
-  SC-006: Zero failures during rotation
-  SC-007: Revoked tokens rejected
-  SC-008: Reconnection after rotation
-  SC-009: Revocation isolation
-  SC-010: Audit token events
-  SC-011: Token status tracking
-  SC-012: Gateway criticality tracking (`isCritical`)
-  SC-013: Gateway type classification (`functionalityType`) supporting regular, ai, and event types
-  SC-014: Real-time connection status (`isActive`)
-  SC-015: Virtual host configuration (`vhost`)
-  SC-016: Gateway descriptions (`description`)
-  SC-017: Lightweight status polling endpoint
-  SC-018: Gateway metadata updates
-  SC-019: Organization-scoped filtering
-  SC-020: JWT-based authentication integration

**Implementation Quality Requirements**:
- Constitution compliance (camelCase, envelope responses)
- Comprehensive OpenAPI documentation
- Database schema optimization
- Error handling and validation
- Security best practices (token hashing, HTTPS)

## References

- Feature Specification: `spec.md`
- Data Model: `data-model.md`
- API Contract: `openapi-gateways.yaml`

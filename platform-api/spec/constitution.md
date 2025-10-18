# Platform API Constitution

## Core Principles

### I. Specification-First Development

Every feature MUST begin with documentation before implementation:

- **PRD First**: Product requirements documented in `spec/prds/<feature>.md` defining the "what" and "why"
- **Architecture & Design**: System-level changes captured in `spec/architecture.md` and `spec/design.md`
- **Implementation Notes**: Each PRD links to `spec/impls/<feature>.md` documenting entry points, behavior, and verification
- **API Contracts**: All REST endpoints reflected in `src/resources/openapi.yaml` with request/response schemas

**Rationale**: Documentation-driven development ensures shared understanding, enables informed reviews, and maintains traceability from requirements to code. It prevents implementation drift and serves as authoritative reference.

### II. Layered Architecture

Code MUST follow clean architecture with explicit layer separation:

- **Handler Layer** (`internal/handler`): HTTP request binding, response formatting, and routing only
- **Service Layer** (`internal/service`): Business logic, validation, cross-entity coordination
- **Repository Layer** (`internal/repository`): Data access abstraction with interface contracts
- **Model & DTO**: Separate domain models (`internal/model`) from data transfer objects (`internal/dto`)
- **Internal Packages**: Use Go's `internal/` to enforce encapsulation boundaries

**Rationale**: Layered architecture isolates concerns, enables independent testing of business logic, and allows persistence layer changes without affecting business rules. Clear boundaries reduce coupling and improve maintainability.

### III. Security by Default

Security MUST be the default, not an option:

- **HTTPS Mandatory**: All server endpoints run over TLS; self-signed certificates auto-generated for development
- **License Headers**: Apache License 2.0 header on every source file
- **Container Security**: Docker images run as non-root user with minimal privileges
- **Input Validation**: Handler layer validates all inputs before passing to service layer
- **Error Messages**: Return structured error responses without leaking sensitive implementation details

**Rationale**: Security cannot be retrofitted. Default-secure patterns prevent vulnerabilities from entering the codebase and establish a security-conscious development culture.

### IV. Documentation Traceability

Documentation MUST maintain bidirectional links between requirements and implementation:

- **PRD ↔ Implementation**: Each PRD links to its implementation notes; each impl.md references its PRD
- **Verification Steps**: Implementation docs include curl commands or test procedures for manual verification
- **Code Paths**: Implementation notes document handler, service, repository file paths for feature discoverability
- **OpenAPI Sync**: REST endpoints in code reflected in OpenAPI spec with accurate schemas

**Rationale**: Traceability enables impact analysis, accelerates onboarding, and ensures documentation remains a living artifact synchronized with code reality.

### V. RESTful API Standards

All HTTP APIs MUST follow REST principles and conventions:

- **Versioned Endpoints**: All routes under `/api/v1` namespace; version in path not header
- **Standard HTTP Semantics**: POST for creation, GET for retrieval, PUT/PATCH for updates, DELETE for removal
- **Resource-Oriented URLs**: `/organizations/:id`, `/projects/:id`, `/apis/:id` structure
- **JSON Content Type**: Request and response bodies use `application/json`
- **Status Codes**: 2xx success, 4xx client errors, 5xx server errors with structured error payloads
- **Idempotency**: GET, PUT, DELETE are idempotent; POST creates new resources

#### OpenAPI Property Naming

All OpenAPI schema properties MUST use camelCase:

- **Property Names**: `organizationId`, `displayName`, `createdAt` (NOT `organization_id`, `display_name`, `created_at`)
- **Consistency**: Both request and response schemas follow camelCase convention
- **JSON Serialization**: Go struct tags use camelCase JSON mappings: `` `json:"organizationId"` ``

**Rationale**: camelCase aligns with JavaScript/TypeScript frontend conventions, reduces impedance mismatch for web clients, and follows OpenAPI/JSON industry standards. While Go prefers snake_case internally, API contracts serve external consumers.

#### Resource Identifier Naming

All resources MUST use consistent identifier naming:

- **Primary Identifier**: Use `id` (NOT `uuid`, `resourceId`, or other variants) for the primary identifier of a resource
- **Foreign Keys**: Use `<resource>Id` pattern (e.g., `organizationId`, `projectId`, `gatewayId`)
- **Path Parameters**: Use `{resourceId}` format (e.g., `/organizations/{orgId}`, `/projects/{projectId}`)
  - Use abbreviated resource name + "Id" suffix for brevity
  - NOT `{resourceUuid}`, `{resource_id}`, or `{resource-id}`

**Examples**:
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "organizationId": "789e0123-a456-12d3-e89b-426614174000"
}
```

**Rationale**: Consistent identifier naming reduces cognitive load, aligns with REST conventions, and makes APIs more intuitive. Using `id` for primary identifiers follows industry standards and simplifies client code. The abbreviated form in path parameters (`orgId` vs `organizationId`) improves URL readability while maintaining clarity.

#### List Response Structure

All list/collection endpoints MUST return a consistent envelope structure using a shared `Pagination` schema:

```json
{
  "count": 1,
  "list": [
    { /* model object */ }
  ],
  "pagination": {
    "total": 10,
    "offset": 1,
    "limit": 2
  }
}
```

- **count**: Number of items in current response (NOT total count across all pages)
- **list**: Array of resource objects matching the endpoint's primary schema
- **pagination**: Shared `Pagination` schema referenced via `$ref` in OpenAPI
  - **total**: Total number of items available across all pages
  - **offset**: Zero-based index of first item in current response
  - **limit**: Maximum number of items returned per page

**OpenAPI Implementation**: Define a shared `Pagination` schema in `components/schemas` and reference it in all list response schemas to avoid duplication:

```yaml
components:
  schemas:
    Pagination:
      type: object
      required: [total, offset, limit]
      properties:
        total:
          type: integer
        offset:
          type: integer
        limit:
          type: integer
    
    ResourceListResponse:
      properties:
        pagination:
          $ref: '#/components/schemas/Pagination'
```

**Rationale**: Consistent list structure enables generic client-side pagination logic, distinguishes between page count and total count, and provides all navigation metadata in a single response. The envelope pattern supports future extensibility (e.g., adding metadata fields) without breaking clients. Using a shared Pagination schema ensures consistency across all list endpoints and reduces specification maintenance burden.

**Overall Rationale**: Consistent REST patterns reduce cognitive load, enable API client code reuse, and align with industry standards for predictable integration. OpenAPI standards ensure API contracts are interoperable and client-friendly.

### VI. Data Integrity

Database operations MUST preserve referential integrity and consistency:

- **Schema First**: All tables, constraints, and indexes defined in `internal/database/schema.sql`
- **Foreign Key Constraints**: Enforce relationships with `FOREIGN KEY` clauses and cascading rules
- **Transactions**: Complex multi-table writes (e.g., API with security/operations) execute within single transaction
- **Idempotent Migrations**: Schema changes use `IF NOT EXISTS` and `IF EXISTS` for safe re-execution
- **Timestamps**: All entities track `created_at` and `updated_at` with automatic defaults

**Rationale**: Data integrity violations create silent corruption that compounds over time. Transactional guarantees and constraints prevent invalid states and enable confident concurrent operations.

### VII. Container-First Operations

The service MUST be production-ready in containerized environments:

- **Multi-Stage Builds**: Optimize image size with separate build and runtime stages
- **Health Checks**: Expose `/health` endpoint with Docker HEALTHCHECK integration
- **Volume Management**: Persist data in mounted volumes (`/api-platform/data`)
- **Environment Configuration**: All runtime config via environment variables (ports, database paths, log levels)
- **Minimal Runtime**: Alpine-based images with only required dependencies

**Rationale**: Container-first design ensures consistent environments from development through production, simplifies deployment, and aligns with cloud-native infrastructure patterns.

## Quality Standards

### Code Standards

- **Go Conventions**: Follow official Go style guide; run `gofmt` before commits
- **Error Handling**: Check all errors; use structured logging for failures with context
- **Package Naming**: Short, lowercase, single-word package names; avoid generic names like "util"
- **Interface Design**: Define interfaces at usage point, not implementation; accept interfaces, return structs

### Testing Standards (To Be Established)

**Note**: Testing discipline is not yet implemented. Future work MUST establish:

- Unit tests for service layer business logic
- Integration tests for repository layer with test database
- API contract tests against OpenAPI spec
- Minimum coverage thresholds and CI enforcement

### Documentation Standards

- **README**: Quick start with build/run/verify commands
- **Spec Structure**: Follow `spec/README.md` organization (prd.md → prds/, impl.md → impls/)
- **Inline Comments**: Document non-obvious decisions; avoid restating code
- **Verification Examples**: Provide curl commands in implementation docs for manual testing

## Governance

### Compliance Review

- **PR Reviews**: All code reviews verify compliance with constitution principles
- **Documentation Checks**: PRs touching code MUST update corresponding spec documents

### Versioning Policy

This constitution follows semantic versioning. Version history:

- **1.0.0**: Initial constitution including RESTful API standards with OpenAPI property naming (camelCase) and list response structure requirements (2025-10-14)

### Enforcement

- Constitution principles are non-negotiable for new features
- Existing code may not yet comply; refactoring PRs to improve compliance are encouraged
- Complexity or deviation from principles MUST be documented with justification

**Version**: 1.0.0 | **Ratified**: 2025-10-14 | **Last Amended**: 2025-10-14

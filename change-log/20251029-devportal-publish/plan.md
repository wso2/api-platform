# Implementation Plan: API Publishing to Developer Portal

**Branch**: `005-devportal-publish` | **Date**: 2025-10-29 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/005-devportal-publish/spec.md`

## Summary

This feature enables platform-api to publish APIs to the developer portal for discovery and subscription by developers. The implementation includes:
- Configuration management for developer portal connection (URL, API key, timeout)
- Automatic organization synchronization with default subscription policy creation
- API publishing endpoint with retry logic and error handling
- Update mechanism for already-published APIs
- Multi-part form-data integration with developer portal REST API

**Technical Approach**: Extend platform-api with new client package for developer portal operations, integrate synchronously into organization creation workflow, and provide new REST endpoint for explicit API publishing with 3-retry logic and 15-second HTTP timeouts. **Note**: Database persistence for published API mappings is deferred to a future task.

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**:
- `github.com/kelseyhightower/envconfig` (environment configuration)
- `github.com/gin-gonic/gin` (HTTP routing and handlers)
- Standard library `net/http` (HTTP client for developer portal API calls)
- `mime/multipart` (for multipart form-data API publishing)

**Storage**: Configuration-only (no database tables for this phase)
**Testing**: Go standard testing (`testing` package), integration tests with HTTP mocking
**Target Platform**: Linux server (containerized via Docker)
**Project Type**: Single Go application (platform-api)
**Performance Goals**:
- Organization creation with devportal sync: <5 seconds
- API publishing: <10 seconds for typical API definitions
- HTTP timeout: 15 seconds per request, 60 seconds total with retries

**Constraints**:
- Synchronous blocking for organization creation when devportal enabled
- 3 retries maximum for API publishing operations
- 15-second timeout per HTTP request to developer portal
- Must use developer portal's multipart form-data format
- No database persistence in this phase (deferred)

**Scale/Scope**:
- Single platform-api instance initially
- Supports single developer portal configuration (global config)
- Handles APIs of varying sizes (OpenAPI definitions up to reasonable limits)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### ✅ I. Specification-First Development
- **Status**: PASS
- **Evidence**: Feature spec completed in `specs/005-devportal-publish/spec.md` with clear functional requirements, user stories, and success criteria before implementation

### ✅ II. Layered Architecture
- **Status**: PASS
- **Plan**:
  - **Handler Layer**: New `/api/internal/v1/apis/{id}/publish-to-devportal` endpoint in `internal/handler`
  - **Service Layer**: Modified `OrganizationService` and `APIService` in `internal/service` to call devportal client
  - **Client Layer**: New `DevPortalClient` in `internal/client/devportal` for HTTP communication with developer portal
  - **Configuration**: New `DevPortal` config struct in `config/config.go`
  - **DTOs**: Handler-level DTOs in `internal/dto`, devportal client DTOs in `internal/client/devportal/dto`
  - **No Repository Layer**: Database persistence deferred to future task

### ✅ III. Security by Default
- **Status**: PASS
- **Plan**:
  - Developer portal API key stored via environment variables (not hardcoded)
  - HTTPS enforcement for developer portal communication (production)
  - Input validation in handler layer before service layer processing
  - Structured error responses without sensitive details
  - Apache License 2.0 headers on all new source files

### ✅ IV. Documentation Traceability
- **Status**: PASS
- **Plan**:
  - This plan.md links to spec.md
  - Implementation will document code paths (handler → service → client)
  - OpenAPI spec will be updated with new `/apis/{id}/publish-to-devportal` endpoint
  - Verification curl commands included in quickstart.md

### ✅ V. RESTful API Standards
- **Status**: PASS
- **Plan**:
  - New endpoint: `POST /api/internal/v1/apis/{id}/publish-to-devportal`
  - Uses standard HTTP semantics (POST for action)
  - JSON request/response bodies with camelCase properties
  - Proper status codes (200 success, 400 client errors, 500 server errors, 503 devportal unavailable)
  - Resource identifier follows `{id}` pattern

### ⚠️ VI. Data Integrity
- **Status**: DEFERRED
- **Reason**: Database persistence for published API mappings and devportal configuration is deferred to a future task. This phase focuses on HTTP integration and workflow logic only.

### ✅ VII. Container-First Operations
- **Status**: PASS
- **Evidence**: Platform-api already containerized with Dockerfile; new feature adds environment variables for devportal config (follows existing patterns)

### ✅ VIII. AI-Ready by Design
- **Status**: PASS
- **Evidence**: OpenAPI spec updates make new endpoint discoverable; structured documentation supports LLM consumption

### ✅ IX. GitOps-Ready Architecture
- **Status**: PASS
- **Evidence**: Configuration via environment variables supports GitOps workflows; feature branch follows `###-feature-name` convention

### ✅ X. Component Independence
- **Status**: PASS
- **Plan**:
  - Platform-api continues to operate independently if developer portal is unavailable (graceful degradation)
  - Devportal synchronization is conditional based on configuration enabled/disabled flag
  - No hard dependency; loose coupling via REST API

**Gate Result**: ✅ ALL CHECKS PASS - Proceed to Phase 0 (with Data Integrity deferred)

## Project Structure

### Documentation (this feature)

```text
specs/005-devportal-publish/
├── spec.md              # Feature specification (completed)
├── plan.md              # This file (in progress)
├── research.md          # Phase 0 output (pending)
├── data-model.md        # Phase 1 output (pending)
├── quickstart.md        # Phase 1 output (pending)
├── contracts/           # Phase 1 output (pending)
│   └── devportal-publish-api.yaml
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (platform-api)

```text
platform-api/
├── src/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── config/
│   │   └── config.go                              # ADD: DevPortal config struct
│   ├── internal/
│   │   ├── client/                                # NEW: External clients package
│   │   │   ├── http_retry_client.go               # NEW: Shared HTTP retry logic
│   │   │   └── devportal/                         # NEW: Developer portal client
│   │   │       ├── devportal_client.go            # NEW: DevPortal HTTP client implementation
│   │   │       └── dto/                           # NEW: DevPortal-specific DTOs
│   │   │           ├── organization_request.go    # NEW: Org creation request
│   │   │           ├── subscription_policy_request.go  # NEW: Policy creation request
│   │   │           └── api_publish_request.go     # NEW: API publish multipart request
│   │   ├── handler/
│   │   │   ├── api_handler.go                     # MODIFY: Add PublishToDevPortal handler
│   │   │   └── organization_handler.go            # MODIFY: Integrate devportal sync
│   │   ├── service/
│   │   │   ├── organization_service.go            # MODIFY: Add devportal sync logic
│   │   │   └── api_service.go                     # MODIFY: Add publish method
│   │   └── dto/
│   │       └── api_dto.go                         # MODIFY: Add PublishAPIRequest/Response
│   └── resources/
│       └── openapi.yaml                           # MODIFY: Add /apis/{id}/publish-to-devportal
└── README.md                                      # MODIFY: Add devportal config instructions
```

**Structure Decision**:
- New `internal/client` package as root for all external client implementations
- Shared `http_retry_client.go` at client package level for reusable retry logic across all future clients
- `internal/client/devportal/` subdirectory contains devportal-specific client implementation
- `internal/client/devportal/dto/` contains DTOs specific to devportal API contracts
- Handler-level DTOs remain in `internal/dto` for platform-api request/response models
- Service layer orchestrates calls to the devportal client package
- No repository layer or database changes in this phase

**Scalability**: This structure supports future clients (e.g., `client/gateway/`, `client/analytics/`) sharing the common `http_retry_client.go` while maintaining isolated implementations.

**Database Persistence Deferred**: No database tables, repository layer, or model changes in this phase. Published API tracking and devportal configuration persistence will be handled in a future task.

## Complexity Tracking

> **No violations** - Implementation follows all constitution principles without exceptions (Data Integrity deferred by design).

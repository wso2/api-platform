# Implementation Plan: Gateway with Controller and Router

**Branch**: `001-gateway-has-two` | **Date**: 2025-10-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-gateway-has-two/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Build a two-component API gateway system consisting of Gateway-Controller (Go-based xDS server) and Router (Envoy Proxy). Users submit REST API configurations in YAML/JSON format to the Gateway-Controller, which validates, persists, and dynamically configures the Router via xDS protocol. The Router forwards HTTP traffic to backend services according to these configurations. The system supports full CRUD lifecycle (create, update, delete, query) for API configurations with zero-downtime updates and graceful handling of in-flight requests.

## Technical Context

### Gateway-Controller

**Language/Version**: Go 1.25.1
**Primary Dependencies**:
- Gin (web framework for REST API)
- Zap (structured logging)
- go-control-plane (Envoy xDS v3 API implementation) - NEEDS CLARIFICATION: specific version
- YAML/JSON parser (gopkg.in/yaml.v3, encoding/json)
- Storage library - NEEDS CLARIFICATION: bbolt/badger/sqlite for persistence

**Storage**: Embedded database or file-based storage for API configurations (bbolt, badger, or SQLite) - NEEDS CLARIFICATION: which option best fits requirements
**Testing**: Go testing framework (testing package), testify for assertions
**Build Tool**: Make (Makefile for build, test, docker targets) - NEEDS CLARIFICATION: verify Make is appropriate vs alternatives
**Target Platform**: Linux/macOS Docker containers (multi-arch support for amd64/arm64)
**Project Type**: Backend service with REST API
**Performance Goals**:
- Accept and validate API configurations in <1 second
- Push xDS updates to Router within 5 seconds of configuration change
- Handle 100+ distinct API configurations without degradation
- Support concurrent configuration updates

**Constraints**:
- Must implement Envoy xDS v3 protocol
- Configuration changes must be atomic (all-or-nothing)
- Must persist configurations for recovery after restarts
- Docker image size should be minimal (<100MB)

**Scale/Scope**:
- Support 100+ API configurations
- Handle concurrent configuration operations
- Single controller instance (no clustering in v1)

### Router

**Language/Version**: Envoy Proxy 1.35.3 (C++ based, configured via YAML)
**Primary Dependencies**: Envoy Proxy official Docker image
**Configuration**: Bootstrap envoy.yaml with xds_cluster pre-configured
**Build Tool**: Make (for Docker image build, consistent with Gateway-Controller)
**Target Platform**: Linux Docker containers
**Project Type**: Infrastructure component (proxy/router)
**Performance Goals**:
- Route requests according to xDS configuration
- Zero dropped connections during configuration updates
- Support graceful configuration reload

**Constraints**:
- Must use Envoy 1.35.3 specifically
- Bootstrap configuration must be minimal (only xds_cluster)
- All routing configuration comes from xDS (no static routes)

**Scale/Scope**:
- Route traffic for 100+ configured APIs
- Handle production HTTP traffic loads

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Status**: ✅ PASS - No constitution file exists yet; proceeding with standard software engineering best practices

Since the constitution file at `.specify/memory/constitution.md` contains only a template without project-specific principles, this feature will follow standard Go and infrastructure best practices:

- **Code Organization**: Follow Go community standards (e.g., Kubernetes, go-control-plane project structures)
- **Testing**: Unit tests for business logic, integration tests for xDS communication
- **Documentation**: Clear README, API documentation, quickstart guide
- **Build System**: Make for consistent build, test, and Docker image creation
- **Observability**: Structured logging with Zap, clear error messages
- **Simplicity**: Start with minimal viable implementation, avoid over-engineering

**Re-evaluation Point**: After Phase 1 design, verify no architectural decisions violate emerging project patterns

**Post-Phase 1 Re-evaluation** (2025-10-11):
✅ PASS - Design review complete. All decisions align with best practices:
- Standard Go project layout (cmd/pkg/tests) follows community conventions
- RESTful API design follows OpenAPI 3.0 standards
- bbolt for storage is simple and appropriate for configuration management
- xDS protocol implementation uses official go-control-plane library
- Docker multi-stage builds for minimal image sizes
- Make-based build system is conventional and accessible
- Zero-downtime updates achieved via Envoy's graceful configuration reloading
- Clear separation of concerns: API handlers, storage, xDS translation, validation

No architectural violations introduced during design phase.

## Project Structure

### Documentation (this feature)

```
specs/001-gateway-has-two/
├── spec.md              # Feature specification
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── gateway-controller-api.yaml  # OpenAPI spec for Gateway-Controller REST API
├── checklists/
│   └── requirements.md  # Specification quality checklist
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```
gateway/
├── gateway-controller/
│   ├── cmd/
│   │   └── controller/
│   │       └── main.go              # Entry point
│   ├── pkg/
│   │   ├── api/
│   │   │   ├── handlers/            # Gin HTTP handlers (CRUD endpoints)
│   │   │   ├── middleware/          # Logging, error handling
│   │   │   └── server.go            # Gin server setup
│   │   ├── config/
│   │   │   ├── parser.go            # YAML/JSON parsing & validation
│   │   │   └── validator.go         # API configuration validation
│   │   ├── models/
│   │   │   └── api_config.go        # API configuration data structures
│   │   ├── storage/
│   │   │   ├── interface.go         # Storage abstraction
│   │   │   └── [implementation].go  # bbolt/badger/sqlite implementation
│   │   ├── xds/
│   │   │   ├── server.go            # xDS server implementation
│   │   │   ├── snapshot.go          # xDS snapshot manager
│   │   │   └── translator.go        # API config -> Envoy config translation
│   │   └── logger/
│   │       └── logger.go            # Zap logger setup
│   ├── tests/
│   │   ├── unit/
│   │   │   ├── parser_test.go
│   │   │   ├── validator_test.go
│   │   │   └── translator_test.go
│   │   └── integration/
│   │       ├── api_test.go          # REST API integration tests
│   │       └── xds_test.go          # xDS server integration tests
│   ├── Makefile                     # Build, test, docker targets
│   ├── Dockerfile                   # Multi-stage Go build
│   ├── go.mod
│   ├── go.sum
│   └── README.md
│
├── router/
│   ├── config/
│   │   └── envoy-bootstrap.yaml     # Bootstrap config with xds_cluster
│   ├── Dockerfile                   # Based on envoyproxy/envoy:v1.35.3
│   ├── Makefile                     # Build docker image
│   └── README.md
│
├── docker-compose.yaml              # Complete stack: controller, router, sample backend
└── README.md                        # Overall gateway documentation
```

**Structure Decision**: This is a multi-component infrastructure project with two independent Docker services (Gateway-Controller and Router). The structure follows Go community conventions (inspired by Kubernetes and go-control-plane projects):

- **Gateway-Controller**: Standard Go project layout with `cmd/` (entry points), `pkg/` (packages), and `tests/` directories. The `pkg/` structure groups code by functional area (api, config, storage, xds).
- **Router**: Minimal structure since it's primarily Envoy configuration. Contains bootstrap YAML and Dockerfile.
- **Separate directories**: Each component is independently buildable with its own Makefile and Dockerfile, supporting the constraint that they must be independently deployable containers.
- **Root-level docker-compose**: Provides easy local development and testing of the complete system.

## Complexity Tracking

*Fill ONLY if Constitution Check has violations that must be justified*

**Status**: N/A - No constitution violations to track

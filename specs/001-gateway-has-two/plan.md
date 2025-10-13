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
- oapi-codegen v2 (OpenAPI code generator for Go - generates server boilerplate from OpenAPI spec)
- Zap (structured logging with configurable levels: debug, info, warn, error)
- go-control-plane v0.13.0+ (Envoy xDS v3 API implementation, latest stable Feb 2025)
- YAML/JSON parser (gopkg.in/yaml.v3, encoding/json)
- bbolt v1.3.9+ (go.etcd.io/bbolt) for persistence
- go-playground/validator/v10 (for structured validation with field-level error reporting)

**Storage**: bbolt embedded key-value store with ACID guarantees, bucket-based organization
**Testing**: Go testing framework (testing package), testify for assertions
**Build Tool**: Make (Makefile for build, test, docker targets)
**Target Platform**: Linux/macOS Docker containers (multi-arch support for amd64/arm64)
**Project Type**: Backend service with REST API
**Performance Goals**:
- Accept and validate API configurations in <1 second
- Push xDS updates to Router within 5 seconds of configuration change
- Handle 100+ distinct API configurations without degradation
- Support concurrent configuration updates
- Load configurations from database to memory on startup within 2 seconds

**Constraints**:
- Must implement Envoy xDS v3 protocol using SotW (State-of-the-World) approach
- Configuration changes must be atomic (all-or-nothing)
- Must persist configurations to bbolt database for durability
- In-memory maps serve as primary data source for xDS cache generation
- Database serves as persistence layer and loaded on startup
- API configurations uniquely identified by composite key `{name}/{version}` (e.g., "PetStore/v1")
- Validation errors must return structured JSON with field paths for precise error reporting
- Logging configurable via environment variable (LOG_LEVEL) or CLI flag, default INFO level
- Docker image size should be minimal (<100MB)

**Scale/Scope**:
- Support 100+ API configurations
- Handle concurrent configuration operations
- Single controller instance (no clustering in v1)

### Router

**Language/Version**: Envoy Proxy 1.35.3 (C++ based, configured via YAML)
**Primary Dependencies**: Envoy Proxy official Docker image
**Configuration**: Bootstrap envoy.yaml with xds_cluster and access logging pre-configured
**Build Tool**: Make (for Docker image build, consistent with Gateway-Controller)
**Target Platform**: Linux Docker containers
**Project Type**: Infrastructure component (proxy/router)
**Performance Goals**:
- Route requests according to xDS configuration
- Zero dropped connections during configuration updates
- Support graceful configuration reload
- Emit structured JSON access logs for observability

**Constraints**:
- Must use Envoy 1.35.3 specifically
- Bootstrap configuration must include xds_cluster with retry policy and access logging
- All routing configuration comes from xDS (no static routes)
- Router waits indefinitely with exponential backoff (1s base, 30s max) if xDS server unavailable at startup
- No traffic served until xDS connection established (fail-safe behavior)
- Access logs output to stdout in JSON format for container-native logging

**Scale/Scope**:
- Route traffic for 100+ configured APIs
- Handle production HTTP traffic loads
- Emit access logs for all requests without performance degradation

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
├── checklists/
│   └── requirements.md  # Specification quality checklist
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

**Note**: The OpenAPI specification (`gateway-controller-api.yaml`) is located in the gateway-controller source directory, not in specs, as it's a development artifact used for code generation.

### Source Code (repository root)

```
gateway/
├── gateway-controller/
│   ├── cmd/
│   │   └── controller/
│   │       └── main.go              # Entry point
│   ├── pkg/
│   │   ├── api/
│   │   │   ├── generated.go         # Generated by oapi-codegen (ServerInterface, types, RegisterHandlers)
│   │   │   ├── handlers/            # Handler implementations for ServerInterface
│   │   │   └── middleware/          # Logging, error handling middleware
│   │   ├── config/
│   │   │   ├── parser.go            # YAML/JSON parsing & validation
│   │   │   └── validator.go         # API configuration validation
│   │   ├── models/
│   │   │   └── api_config.go        # API configuration data structures (complement generated types)
│   │   ├── storage/
│   │   │   ├── interface.go         # Storage abstraction
│   │   │   ├── memory.go            # In-memory maps for runtime access
│   │   │   └── bbolt.go             # bbolt implementation for persistence
│   │   ├── xds/
│   │   │   ├── server.go            # xDS SotW server implementation
│   │   │   ├── snapshot.go          # xDS snapshot manager (SotW cache)
│   │   │   └── translator.go        # In-memory maps -> Envoy config translation
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
│   ├── api/
│   │   └── openapi.yaml             # OpenAPI 3.0 specification for REST API (source for code generation)
│   ├── oapi-codegen.yaml            # Code generation configuration
│   ├── Makefile                     # Build, test, docker, generate targets
│   ├── Dockerfile                   # Multi-stage Go build
│   ├── go.mod
│   ├── go.sum
│   └── README.md
│
├── router/
│   ├── config/
│   │   └── envoy-bootstrap.yaml     # Bootstrap config with xds_cluster and access logging
│   ├── Dockerfile                   # Based on envoyproxy/envoy:v1.35.3
│   ├── Makefile                     # Build docker image
│   └── README.md
│
├── docker compose.yaml              # Complete stack: controller, router, sample backend
└── README.md                        # Overall gateway documentation
```

**Structure Decision**: This is a multi-component infrastructure project with two independent Docker services (Gateway-Controller and Router). The structure follows Go community conventions (inspired by Kubernetes and go-control-plane projects):

- **Gateway-Controller**: Standard Go project layout with `cmd/` (entry points), `pkg/` (packages), and `tests/` directories. The `pkg/` structure groups code by functional area (api, config, storage, xds).
  - **Code Generation**: REST API code is generated from OpenAPI spec using oapi-codegen, producing `pkg/api/generated.go` with ServerInterface, request/response types, and handler registration
  - **Handler Implementation**: Business logic implemented in `pkg/api/handlers/` by satisfying the generated ServerInterface
  - **Startup Flow**: Loads all API configurations from bbolt database into in-memory maps
  - **Runtime**: In-memory maps serve as the primary data source for fast access and xDS cache generation
  - **Persistence**: All configuration changes are written to both in-memory maps and database atomically
  - **Build Process**: `make generate` runs oapi-codegen before `make build` to ensure generated code is up-to-date
- **Router**: Minimal structure since it's primarily Envoy configuration. Contains bootstrap YAML and Dockerfile.
- **Separate directories**: Each component is independently buildable with its own Makefile and Dockerfile, supporting the constraint that they must be independently deployable containers.
- **Root-level docker compose**: Provides easy local development and testing of the complete system.

**Data Flow Architecture**:
```
Startup:  Database → Load to In-Memory Maps → Generate Initial xDS Snapshot
Runtime:  User Request → Validate → Update In-Memory Maps + Database → Generate xDS Snapshot → Push to Router
```

**Router Access Logging**:
The Router emits structured JSON access logs for all HTTP requests to stdout, enabling production observability:

- **Configuration**: Included in xDS-generated Listener resources by Gateway-Controller (dynamic configuration)
- **Format**: JSON with standard fields (timestamp, method, path, response code, duration, upstream cluster, etc.)
- **Destination**: Stdout for container-native logging (captured by Docker/Kubernetes runtime)
- **Performance**: File-based logging to stdout with minimal overhead
- **Log Aggregation**: Compatible with ELK, Splunk, CloudWatch, and other log aggregation tools
- **Implementation**: Gateway-Controller's xDS translator adds `access_log` field to all generated Listeners

Example log entry:
```json
{
  "start_time": "2025-10-12T10:30:45.123Z",
  "method": "GET",
  "path": "/weather/US/Seattle",
  "protocol": "HTTP/1.1",
  "response_code": 200,
  "response_flags": "-",
  "bytes_received": 0,
  "bytes_sent": 1024,
  "duration": 45,
  "upstream_service_time": "42",
  "upstream_cluster": "cluster_api_weather_com",
  "upstream_host": "api.weather.com:443"
}
```

See research.md Decision 10 for detailed access logging strategy and configuration.

## Complexity Tracking

*Fill ONLY if Constitution Check has violations that must be justified*

**Status**: N/A - No constitution violations to track

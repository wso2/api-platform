# Implementation Plan: Envoy Policy Engine System

**Branch**: `001-policy-engine` | **Date**: 2025-11-18 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-policy-engine/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

The Envoy Policy Engine is an external processor service that integrates with Envoy Proxy v1.36.2 to provide flexible, extensible HTTP request and response processing through configurable policy chains. The system consists of three major components: (1) Policy Engine runtime service executing policy chains on HTTP traffic, (2) Policy Engine Builder for compiling custom policy implementations into the engine, and (3) Sample policy implementations demonstrating the framework.

**Technical Approach**: Implement a Go-based ext_proc gRPC service with a kernel/worker architecture. The kernel handles Envoy integration and route-to-policy mapping. The worker core executes policy chains with short-circuit logic and shared metadata. Policies are defined via YAML schemas and implemented as Go interfaces. The builder uses a Docker-based multi-stage build process to auto-discover, validate, and compile custom policies into a single optimized binary.

## Technical Context

**Language/Version**: Go 1.23+
**Primary Dependencies**:
- Envoy ext_proc protobuf/gRPC (envoyproxy/go-control-plane)
- xDS protocol libraries (envoyproxy/go-control-plane)
- CEL expression evaluation (google/cel-go)
- JWT validation (golang-jwt/jwt)
- YAML parsing (gopkg.in/yaml.v3)
- gRPC server framework (google.golang.org/grpc)

**Storage**:
- In-memory policy registry and route mappings
- In-memory request context storage (request → response phase)
- File-based policy definitions (YAML schemas in policy directories)
- File-based configuration (development mode)

**Testing**:
- Go standard testing framework (testing package)
- Contract tests for ext_proc gRPC interface
- Integration tests with Envoy Docker container
- Policy execution unit tests
- Builder validation tests

**Target Platform**: Linux containers (Docker), deployed as sidecar or separate service alongside Envoy Proxy
**Project Type**: Multi-component service project (policy-engine runtime + builder tooling)
**Performance Goals**:
- Headers-only policy chains: < 5ms added latency (p95)
- Body-buffering policy chains: < 20ms added latency (p95) for < 100KB payloads
- Support 10,000 concurrent requests with up to 5-policy chains
- Builder compilation time: < 3 minutes for custom policies

**Constraints**:
- Compatible with Envoy v1.36.2 ext_proc filter
- Body buffering limited to configurable size (default 10MB)
- Policy execution timeout: < 1 second per policy
- Zero-downtime policy configuration updates
- Build-time policy inclusion (no runtime plugin loading)

**Scale/Scope**:
- Support 50+ concurrent policy versions in single binary
- Handle 100+ routes with independent policy chains
- Support 10-policy chains per route without degradation
- Builder handles 20+ custom policies in single build

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Status**: ✅ PASS (No constitution file found - proceeding with industry best practices)

Since no constitution.md file with specific project principles was found, this implementation will follow Go and microservices industry standards:
- Clean architecture with separation of concerns (Kernel/Worker/Policies)
- Interface-based design for extensibility
- Comprehensive testing at unit, integration, and contract levels
- Clear error handling and validation
- Performance-oriented design with optimization points
- Documentation-driven development (YAML schemas, code comments)

## Project Structure

### Documentation (this feature)

```text
specs/001-policy-engine/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   ├── extproc.proto    # Envoy ext_proc gRPC service contract
│   ├── xds.proto        # xDS policy discovery service contract
│   └── policy-api.md    # Policy interface contracts
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
# Policy Engine Runtime
src/
├── main.go                      # Entry point, loads plugin_registry.go
├── plugin_registry.go           # GENERATED: Policy imports and registrations
├── build_info.go                # GENERATED: Build metadata
├── kernel/                      # Kernel layer - Envoy integration
│   ├── extproc.go              # ext_proc gRPC server implementation
│   ├── xds.go                  # xDS policy discovery service
│   ├── mapper.go               # Route-to-policy mapping
│   ├── translator.go           # Action → ext_proc response translator
│   ├── context_storage.go      # Request context storage (request → response)
│   └── body_mode.go            # Body processing mode determination
├── worker/                      # Worker layer
│   ├── core/                   # Core execution engine
│   │   ├── executor.go         # Policy chain executor
│   │   ├── registry.go         # Policy registry
│   │   ├── loader.go           # YAML schema loader
│   │   ├── action.go           # Action type definitions
│   │   ├── context.go          # RequestContext, ResponseContext
│   │   ├── cel_evaluator.go   # CEL expression evaluator
│   │   └── validation.go       # Configuration validation
│   └── policies/               # Policy interfaces and base types
│       ├── interface.go        # Policy, RequestPolicy, ResponsePolicy interfaces
│       ├── types.go            # Parameter types, validation rules
│       └── schema.go           # PolicyDefinition, PolicySpec structures
├── pkg/                        # Shared packages
│   ├── xds/                    # xDS protocol utilities
│   ├── validation/             # Parameter validation engine
│   └── cel/                    # CEL utilities
└── go.mod

# Sample Policy Implementations
policies/
├── set-header/
│   └── v1.0.0/
│       ├── policy.yaml         # Policy definition
│       ├── setheader.go        # Implementation
│       ├── go.mod
│       └── README.md
├── jwt-validation/
│   ├── v1.0.0/
│   │   ├── policy.yaml
│   │   ├── jwt.go
│   │   ├── go.mod
│   │   └── README.md
│   └── v2.0.0/                 # Demonstrates versioning
│       ├── policy.yaml
│       ├── jwt.go
│       ├── go.mod
│       └── README.md
└── api-key-validation/
    └── v1.0.0/
        ├── policy.yaml
        ├── apikey.go
        ├── go.mod
        └── README.md

# Policy Engine Builder
build/
├── build.sh                     # Main orchestrator
├── discover.sh                  # Policy discovery
├── validate.sh                  # Policy validation
├── generate.sh                  # Code generation
├── compile.sh                   # Binary compilation
├── package.sh                   # Runtime image packaging
└── utils.sh                     # Common utilities

templates/
├── plugin_registry.go.tmpl      # Import generation template
├── build_info.go.tmpl           # Build metadata template
└── Dockerfile.runtime.tmpl      # Runtime image template

tools/
├── policy-validator/            # YAML schema validator tool
│   ├── main.go
│   └── go.mod
└── schema-checker/              # Go interface checker tool
    ├── main.go
    └── go.mod

# Docker images
Dockerfile.builder               # Builder image with build tools
Dockerfile.runtime              # GENERATED: Final runtime image

# Testing
tests/
├── unit/                       # Unit tests per component
│   ├── kernel/
│   ├── core/
│   └── policies/
├── integration/                # Integration tests with Envoy
│   ├── envoy-config/
│   ├── test-scenarios/
│   └── docker-compose.yml
└── contract/                   # gRPC contract tests
    ├── extproc_test.go
    └── xds_test.go

# Configuration & Deployment
configs/
├── policy-engine.yaml          # Runtime configuration
├── envoy.yaml                  # Envoy configuration with ext_proc
└── xds/                        # Sample xDS configurations
    ├── route-simple.yaml
    ├── route-with-jwt.yaml
    └── route-multi-policy.yaml

docker-compose.yml              # Local development setup
```

**Structure Decision**: Multi-component service architecture selected because the system has three distinct components (runtime engine, builder tooling, sample policies) with different lifecycles and deployment models. The runtime engine is a long-running service, the builder is build-time tooling, and sample policies are reference implementations for users to extend. This structure provides clear separation while keeping related code in the same repository.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

N/A - No constitution violations. Design follows Go best practices and microservices patterns appropriate for the domain.

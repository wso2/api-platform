# Implementation Plan: Envoy Policy Engine System

**Branch**: `001-policy-engine` | **Date**: 2025-11-18 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-policy-engine/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

The Envoy Policy Engine is an external processor service that integrates with Envoy Proxy v1.36.2 to provide flexible, extensible HTTP request and response processing through configurable policy chains. The system consists of three major components: (1) **Policy Engine Runtime** - framework service (kernel + worker + interfaces) with ZERO built-in policies, (2) **Policy Engine Builder** - build-time tooling that discovers, validates, and compiles custom policies into the runtime binary, and (3) **Sample Policy Implementations** - optional reference examples (SetHeader, JWT, etc.) demonstrating the framework.

**Critical Architecture Note**: The Policy Engine runtime ships with NO policies. ALL policies (including sample/reference policies) must be compiled via the Builder. This ensures minimal attack surface and allows organizations to include only needed policies.

**Technical Approach**: Implement a Go-based ext_proc gRPC service with a kernel/worker architecture. The kernel handles Envoy integration and route-to-policy mapping. The worker core executes policy chains with short-circuit logic and shared metadata. Policies are defined via YAML schemas and implemented as Go interfaces. The builder uses a Docker-based multi-stage build process to auto-discover, validate, and compile custom policies into a single optimized binary. Sample policies are separate reference implementations, NOT bundled with the runtime.

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
- Request context managed via PolicyExecutionContext (local variable in streaming loop - no global storage)
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
- Handle 1000+ routes with independent policy chains
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
# ========================================
# Policy Engine Runtime (Framework Only - NO built-in policies)
# ========================================
# This is the core framework that gets compiled with custom policies via the Builder.
# The runtime itself contains ZERO policies - all policies must be compiled in.

src/
├── main.go                      # Entry point, loads plugin_registry.go
├── plugin_registry.go           # GENERATED by Builder: Policy imports and registrations
├── build_info.go                # GENERATED by Builder: Build metadata
├── kernel/                      # Kernel layer - Envoy integration
│   ├── extproc.go              # ext_proc gRPC server implementation
│   ├── execution_context.go    # PolicyExecutionContext - request lifecycle management
│   ├── xds.go                  # xDS policy discovery service
│   ├── mapper.go               # Route-to-policy mapping
│   ├── translator.go           # Action → ext_proc response translator
│   └── body_mode.go            # Body processing mode determination
├── pkg/                        # Shared packages
│   ├── validation/             # Parameter validation engine
│   └── cel/                    # CEL utilities
└── go.mod                      # Main module: github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine

# ========================================
# SDK Module (Separate module to avoid cyclic dependencies)
# ========================================
sdk/
├── core/                       # Core execution engine
│   ├── executor.go             # Policy chain executor
│   ├── registry.go             # Policy registry
│   ├── loader.go               # YAML schema loader
│   └── types.go                # Core types (PolicyChain, etc.)
├── policies/                   # Policy interfaces and base types
│   ├── interface.go            # Policy, RequestPolicy, ResponsePolicy interfaces
│   ├── types.go                # Parameter types, validation rules
│   ├── schema.go               # PolicyDefinition, PolicySpec structures
│   ├── action.go               # Action type definitions
│   └── context.go              # RequestContext, ResponseContext
└── go.mod                      # SDK module: github.com/policy-engine/sdk

# ========================================
# Sample Policy Implementations (OPTIONAL Reference Examples)
# ========================================
# These are NOT part of the Policy Engine runtime - they are separate reference
# implementations demonstrating how to write policies. Users can optionally compile
# these into their custom binary using the Policy Engine Builder.
# The runtime binary does NOT include these by default.

policies/                        # Optional examples directory
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

# Policy Engine Builder (Go Implementation)
build/
├── cmd/
│   └── builder/
│       └── main.go              # Builder CLI entry point
├── internal/
│   ├── discovery/
│   │   ├── discovery.go         # Policy discovery from /policies mount
│   │   └── policy.go            # Policy metadata extraction
│   ├── validation/
│   │   ├── validator.go         # Policy validation orchestrator
│   │   ├── yaml.go              # YAML schema validation
│   │   ├── golang.go            # Go interface validation
│   │   └── structure.go         # Directory structure validation
│   ├── generation/
│   │   ├── generator.go         # Code generation orchestrator
│   │   ├── registry.go          # plugin_registry.go generation
│   │   ├── buildinfo.go         # build_info.go generation
│   │   └── gomod.go             # go.mod replace directive generation
│   ├── compilation/
│   │   ├── compiler.go          # Binary compilation
│   │   └── options.go           # Build options and flags
│   └── packaging/
│       ├── packager.go          # Runtime Dockerfile generation
│       └── metadata.go          # Docker image metadata/labels
├── pkg/
│   ├── types/
│   │   └── policy.go            # Shared policy types
│   └── errors/
│       └── errors.go            # Builder error types
└── go.mod                       # Builder module dependencies

templates/
├── plugin_registry.go.tmpl      # Import generation template
├── build_info.go.tmpl           # Build metadata template
└── Dockerfile.policy-engine.tmpl      # Runtime image template

# Docker Images
gateway-builder/Dockerfile               # Builder image with:
                                 #   - Go 1.23+ toolchain
                                 #   - Policy Engine framework source (src/)
                                 #   - Builder Go application (build/)
                                 #   - Templates (templates/)
                                 # Entry point: /build/cmd/builder/main.go
                                 # Users ONLY mount: /policies and /output
Dockerfile.runtime              # GENERATED: Final runtime image with compiled binary

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

**Structure Decision**: Multi-component service architecture selected because the system has three distinct components (runtime framework, builder tooling, sample policies) with different lifecycles and deployment models:

1. **Runtime Framework** (`src/`): Core framework with NO policies - compiled into custom binaries
2. **Builder Tooling** (`build/`, `templates/`, `tools/`): Build-time tooling that compiles policies into runtime
3. **Sample Policies** (`policies/`): OPTIONAL reference implementations - NOT bundled with runtime

**Critical Separation**: The runtime and sample policies are architecturally separate. The runtime is a policy-agnostic framework. Sample policies demonstrate the framework but are not required. Users can build a binary with zero policies, only sample policies, only their custom policies, or any combination via the Builder.

**Builder Image Architecture**: The `gateway-builder/Dockerfile` creates a complete build environment containing:
- Go 1.23+ toolchain
- **Policy Engine framework source code** (`src/` directory)
- **Builder Go application** (`build/` - discovery, validation, generation, compilation, packaging modules)
- Code generation templates (`templates/`)
- Everything needed to discover policies, validate them, generate code, and compile the final binary

**User Workflow**: Users run the Builder image and ONLY mount:
- `/policies` - Their custom policy implementations (or sample policies)
- `/output` - Where the compiled binary and Dockerfile will be generated

The framework source (`src/`) is already IN the Builder image - users never need to mount or provide it.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

N/A - No constitution violations. Design follows Go best practices and microservices patterns appropriate for the domain.

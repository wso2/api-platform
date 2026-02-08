# Research: Envoy Policy Engine System

**Feature**: 001-policy-engine | **Date**: 2025-11-18

## Overview

This document captures research findings and technical decisions for implementing the Envoy Policy Engine system. All technical clarifications from the plan.md have been resolved through analysis of the Spec.md and BUILDER_DESIGN.md documents.

## Research Areas

### 1. Envoy ext_proc Integration

**Decision**: Use Envoy v1.36.2 ext_proc filter with gRPC streaming protocol

**Rationale**:
- ext_proc is the official Envoy extension point for external request/response processing
- Provides bidirectional gRPC streaming with complete request/response access
- Supports mode override for dynamic body buffering configuration
- Well-documented protocol with Go libraries (envoyproxy/go-control-plane)

**Alternatives Considered**:
- **Lua filter**: Rejected - Limited to Lua language, performance constraints, no external service integration
- **WASM filter**: Rejected - Requires compilation to WASM, more complex development workflow, harder debugging
- **HTTP filter chain**: Rejected - Requires Envoy recompilation, no dynamic configuration

**Implementation Notes**:
- Use `envoy.extensions.filters.http.ext_proc.v3.ExternalProcessor` configuration
- Implement `ExternalProcessorServer` gRPC service interface
- Support both `ProcessingMode_SKIP` and `ProcessingMode_BUFFERED` for body handling
- Handle request headers, request body, response headers, and response body callbacks

**References**:
- Envoy ext_proc documentation: https://www.envoyproxy.io/docs/envoy/v1.36.2/configuration/http/http_filters/ext_proc_filter
- go-control-plane library: https://github.com/envoyproxy/go-control-plane

---

### 2. xDS Protocol for Dynamic Configuration

**Decision**: Implement custom xDS server using go-control-plane with file-based fallback

**Rationale**:
- xDS is Envoy's standard protocol for dynamic configuration
- Enables zero-downtime policy updates without service restart
- go-control-plane provides server implementation and resource versioning
- File-based mode supports development/testing without control plane

**Alternatives Considered**:
- **File watching**: Rejected - No atomic updates, race conditions, limited to single instance
- **HTTP API**: Rejected - Non-standard, requires custom client, no built-in versioning
- **Database polling**: Rejected - Higher latency, unnecessary complexity for configuration

**Implementation Notes**:
- Implement `PolicyDiscoveryService` gRPC service
- Use snapshot-based configuration with version tracking
- Support both streaming (production) and file-based (development) modes
- Policy configurations represented as xDS resources

**References**:
- xDS protocol: https://www.envoyproxy.io/docs/envoy/v1.36.2/api-docs/xds_protocol
- go-control-plane examples: https://github.com/envoyproxy/go-control-plane/tree/main/pkg/test/v3

---

### 3. CEL Expression Evaluation

**Decision**: Use google/cel-go for conditional policy execution

**Rationale**:
- CEL is Google's Common Expression Language, designed for policy evaluation
- Type-safe expression evaluation with compile-time checking
- Well-suited for request/response context evaluation
- Used by Kubernetes, Envoy, and other CNCF projects

**Alternatives Considered**:
- **Go templates**: Rejected - Not type-safe, designed for text generation not boolean logic
- **Custom DSL**: Rejected - Significant development effort, poor ecosystem support
- **JavaScript (goja)**: Rejected - Heavier runtime, potential security issues

**Implementation Notes**:
- Compile CEL expressions at configuration time (fail fast)
- Provide RequestContext and ResponseContext as CEL variables
- Support standard CEL functions plus custom extensions if needed
- Cache compiled expressions for performance

**References**:
- CEL specification: https://github.com/google/cel-spec
- cel-go library: https://github.com/google/cel-go

---

### 4. JWT Validation

**Decision**: Use golang-jwt/jwt v5 with JWKS caching

**Rationale**:
- golang-jwt/jwt is the de facto standard JWT library for Go
- Supports RS256, ES256, and other standard algorithms
- JWKS (JSON Web Key Set) enables public key rotation
- Built-in claim validation (exp, nbf, iss, aud)

**Alternatives Considered**:
- **lestrrat-go/jwx**: Rejected - More complex API, overkill for basic validation
- **Custom implementation**: Rejected - Security-critical code should use vetted libraries
- **auth0/go-jwt-middleware**: Rejected - Too HTTP-middleware specific, not flexible enough

**Implementation Notes**:
- Implement JWKS fetching with configurable TTL caching
- Support multiple JWKS endpoints per policy instance
- Extract claims and inject as headers for upstream
- Validate standard claims (iss, aud, exp, nbf) plus custom claims

**References**:
- golang-jwt/jwt: https://github.com/golang-jwt/jwt
- JWKS RFC: https://datatracker.ietf.org/doc/html/rfc7517

---

### 5. Policy Registry Pattern

**Decision**: In-memory registry with composite key (name:version) indexing

**Rationale**:
- Policies are immutable after registration (compile-time defined)
- Fast lookup O(1) with map-based registry
- Supports multiple versions concurrently
- Simple lifecycle (register at startup, use forever)

**Alternatives Considered**:
- **Database storage**: Rejected - Policies are code, not data; no runtime changes
- **Plugin system**: Rejected - Security concerns, complex lifecycle, performance overhead
- **Service discovery**: Rejected - Policies are local to binary, not distributed services

**Implementation Notes**:
- Registry key format: `"policyName:version"` (e.g., "jwtValidation:v1.0.0")
- Store both PolicyDefinition (schema) and Policy implementation (code)
- Thread-safe read-only access (no updates after startup)
- Auto-discovery from policy directories during initialization

---

### 6. Policy Parameter Validation

**Decision**: OpenAPI v3 / JSON Schema inspired validation with Go structs

**Rationale**:
- Familiar validation semantics (min/max, pattern, enum, format)
- Type-safe validation rules defined in Go
- Fail-fast at configuration time, not request time
- Extensible with CEL custom validation expressions

**Alternatives Considered**:
- **JSON Schema**: Rejected - Runtime JSON parsing overhead, less type-safe
- **Go struct tags**: Rejected - Limited validation expressiveness, no custom formats
- **Protobuf validation**: Rejected - Forces protobuf for all configuration

**Implementation Notes**:
- Define ParameterType enum (string, int, float, bool, duration, array, map, uri, email, etc.)
- ValidationRules struct with type-specific constraints
- Validate at xDS configuration time before PolicySpec creation
- Clear error messages with parameter path and constraint violated

**References**:
- OpenAPI v3 validation: https://swagger.io/docs/specification/data-models/data-types/
- JSON Schema validation: https://json-schema.org/understanding-json-schema/reference/index.html

---

### 7. Builder Architecture

**Decision**: Go-based builder application with discovery → validation → generation → compilation → packaging pipeline, packaged in Docker image

**Rationale**:
- **Go implementation**: Type-safe, testable, maintainable - consistent with policy engine language
- Reproducible builds with controlled Docker environment
- Auto-discovery eliminates manual policy registration
- Validation catches errors before compilation with detailed error reporting
- Generated import code ensures all policies linked correctly
- Better error handling and user feedback compared to shell scripts
- Cross-platform compatibility (Go compiles for any platform)
- Final distroless runtime image minimizes attack surface

**Alternatives Considered**:
- **Shell scripts**: Rejected - Poor error handling, hard to test, limited type safety, platform-specific
- **Go plugins**: Rejected - Runtime loading has versioning issues, security concerns, performance impact
- **WASM plugins**: Rejected - Limited Go support, complex build chain, debugging difficulties
- **Manual compilation**: Rejected - Error-prone, doesn't scale with many custom policies

**Implementation Notes**:
- Builder image: golang:1.23-alpine base
- **Builder Go application structure**:
  - `build/cmd/builder/main.go` - CLI entry point
  - `build/internal/discovery/` - Policy discovery from /policies mount
  - `build/internal/validation/` - YAML, Go interface, and structure validation
  - `build/internal/generation/` - Code generation using text/template
  - `build/internal/compilation/` - Binary compilation using os/exec
  - `build/internal/packaging/` - Dockerfile generation
- **Builder image CONTAINS**:
  - Policy Engine framework source (`src/`)
  - Builder Go application (`build/`)
  - Code generation templates (`templates/`)
- **Users ONLY mount**:
  - `/policies` - Custom policy implementations
  - `/output` - Generated binary and Dockerfile
- **Build phases**:
  1. Discovery: Walk `/policies`, parse policy.yaml files using gopkg.in/yaml.v3
  2. Validation: Validate YAML schema, check Go interfaces with go/parser, verify directory structure
  3. Generation: Generate plugin_registry.go and build_info.go using text/template
  4. Compilation: Execute `go build` with CGO_ENABLED=0, ldflags for metadata
  5. Packaging: Generate Dockerfile.runtime with embedded policy list and build metadata

**References**:
- Multi-stage builds: https://docs.docker.com/build/building/multi-stage/
- Distroless images: https://github.com/GoogleContainerTools/distroless

---

### 8. Error Handling Strategy

**Decision**: Typed errors with context, graceful degradation, configurable fail-open/fail-closed

**Rationale**:
- Clear error types enable appropriate handling (validation vs runtime vs external)
- Context-enriched errors improve debugging (policy name, request ID, configuration)
- Fail-open/closed per policy enables defense-in-depth vs availability trade-offs
- Graceful degradation prevents single policy failure from breaking entire chain

**Alternatives Considered**:
- **Panic on errors**: Rejected - Crashes entire service, no graceful degradation
- **Silent failures**: Rejected - Hides issues, makes debugging impossible
- **Always fail-closed**: Rejected - Reduces availability, some policies should degrade gracefully

**Implementation Notes**:
- Error types: PolicyValidationError, PolicyExecutionError, PolicyTimeoutError
- Include request ID, policy name, policy version in error context
- Configurable failure mode per PolicySpec: FailOpen, FailClosed, FailSkip
- Log all errors with structured logging (JSON format)

---

### 9. Performance Optimization

**Decision**: Body mode optimization, JWKS caching, compiled CEL expressions, connection pooling

**Rationale**:
- SKIP body mode eliminates buffering latency for headers-only policies (>50% use cases)
- JWKS caching reduces external HTTP calls (1 fetch per hour vs per request)
- Compiled CEL avoids parse overhead on every evaluation
- gRPC connection pooling amortizes connection establishment cost

**Alternatives Considered**:
- **Always buffer body**: Rejected - Unnecessary latency for headers-only policies
- **No caching**: Rejected - JWKS endpoint becomes bottleneck, adds 50-100ms per request
- **Runtime CEL parsing**: Rejected - Parse overhead is 1-5ms per expression
- **New connection per request**: Rejected - Connection establishment is 10-50ms

**Implementation Notes**:
- Analyze PolicyChain.RequiresRequestBody and RequiresResponseBody flags at configuration time
- Set ProcessingMode in ext_proc response based on flags
- JWKS cache: LRU with configurable TTL (default 1 hour)
- CEL expression cache: Map from expression string to compiled Program
- gRPC: Use grpc.WithDefaultCallOptions(grpc.MaxConcurrentStreams(1000))

---

### 10. Testing Strategy

**Decision**: Unit tests (70%), integration tests with Envoy (20%), contract tests (10%)

**Rationale**:
- Unit tests provide fast feedback for business logic
- Integration tests verify end-to-end Envoy interaction
- Contract tests ensure gRPC interface compatibility
- No need for UI tests (headless service)

**Alternatives Considered**:
- **Only integration tests**: Rejected - Slow feedback, hard to test edge cases
- **Manual testing**: Rejected - Not repeatable, doesn't scale
- **Load testing only**: Rejected - Doesn't catch functional bugs

**Implementation Notes**:
- Unit tests: Use Go testing package, table-driven tests, mock interfaces
- Integration tests: Docker Compose with Envoy + Policy Engine + test backend
- Contract tests: Test ext_proc protocol conformance, xDS protocol conformance
- Use testcontainers-go for integration test orchestration

**Test Scenarios**:
- Policy execution: Short-circuit, pass-through, modification
- Configuration: Valid config acceptance, invalid config rejection
- Performance: Latency measurement, concurrent request handling
- Error handling: Policy timeout, external service failure, invalid CEL

**References**:
- Go testing: https://pkg.go.dev/testing
- testcontainers-go: https://github.com/testcontainers/testcontainers-go

---

## Technology Stack Summary

| Component | Technology | Version | Purpose |
|-----------|-----------|---------|---------|
| Language | Go | 1.23+ | Implementation language |
| gRPC | google.golang.org/grpc | Latest | ext_proc and xDS communication |
| Envoy Integration | envoyproxy/go-control-plane | Latest | ext_proc and xDS protocol definitions |
| CEL | google/cel-go | Latest | Conditional expression evaluation |
| JWT | golang-jwt/jwt | v5 | JWT token validation |
| YAML | gopkg.in/yaml.v3 | v3 | Policy definition parsing |
| Testing | Go testing package | Built-in | Unit and integration tests |
| Containerization | Docker | Latest | Builder and runtime images |
| Base Image (runtime) | gcr.io/distroless/static-debian12 | Latest | Minimal attack surface |

---

## Key Design Decisions Summary

1. **Architecture**: Kernel (Envoy integration) + Worker (policy execution) + Policies (implementations)
2. **Policy Loading**: Build-time inclusion with auto-discovery, not runtime plugins
3. **Configuration**: xDS streaming (production) with file-based fallback (development)
4. **Body Handling**: Dynamic mode selection based on policy requirements (SKIP vs BUFFERED)
5. **Versioning**: Multiple policy versions coexist, routes select version explicitly
6. **Execution**: Sequential policy chain with short-circuit support and shared metadata
7. **Validation**: Configuration-time parameter validation against YAML schemas
8. **Conditional Execution**: CEL expressions evaluated against request/response context
9. **Error Handling**: Configurable fail-open/fail-closed per policy with graceful degradation
10. **Performance**: JWKS caching, CEL compilation, body mode optimization, connection pooling

---

## Next Steps

With research complete, proceed to Phase 1:
1. Generate data-model.md with entity definitions
2. Generate contracts/ with gRPC proto definitions and policy interface contracts
3. Generate quickstart.md with local development setup
4. Update agent context with technology stack

All technical clarifications resolved. Ready to proceed with detailed design.

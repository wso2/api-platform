# Feature Specification: Envoy Policy Engine System

**Feature Branch**: `001-policy-engine`
**Created**: 2025-11-18
**Status**: Draft
**Input**: User description: "Read the spec @Spec.md and @BUILDER_DESIGN.md. You are a senior software architect. I want this Policy Engine, Policy Engine Builder, Policies implementations (sample)"

## Overview

The Envoy Policy Engine is an external processor service that integrates with Envoy Proxy to provide flexible, extensible HTTP request and response processing through configurable policies. The system enables dynamic policy configuration, version management, and custom policy implementation without requiring Envoy restarts or recompilation.

The system consists of three major components:
1. **Policy Engine Runtime**: Framework service (kernel + worker + interfaces) with NO built-in policies - executes policy chains compiled into the binary
2. **Policy Engine Builder**: Build-time tooling that discovers, validates, and compiles custom policy implementations into the engine binary
3. **Sample Policy Implementations**: Reference policy examples (SetHeader, JWT, API Key, etc.) that demonstrate the framework and can be optionally compiled via the builder

**Important**: The Policy Engine runtime ships with ZERO policies by default. ALL policies (including sample/reference policies) must be compiled into the binary using the Policy Engine Builder. This ensures a minimal, secure baseline and allows organizations to include only the policies they need.

## Clarifications

### Session 2025-11-18

- Q: Observability scope (metrics export format, logging patterns) → A: Remove observability from initial scope, defer to future enhancements
- Q: xDS configuration source for policy chains → A: File-based config for development, xDS for production
- Q: Builder incremental build strategy → A: Always full rebuild (simple, no change detection needed)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Route-Based Policy Execution (Priority: P1)

A platform operator configures different policy chains for different API routes. When clients send requests to `/api/v1/public`, the system applies only header manipulation. When clients send requests to `/api/v1/private`, the system enforces JWT validation and rate limiting before forwarding to upstream services.

**Why this priority**: Core value proposition - enables route-specific traffic management without Envoy configuration changes

**Independent Test**: Deploy Policy Engine with two routes configured. Send requests to both routes and verify correct policies execute for each route. Delivers immediate value by enabling basic policy-based routing.

**Acceptance Scenarios**:

1. **Given** Policy Engine is configured with JWT validation for `/api/v1/private` route, **When** client sends request without valid JWT to `/api/v1/private`, **Then** request is rejected with 401 Unauthorized response
2. **Given** Policy Engine is configured with no authentication for `/api/v1/public` route, **When** client sends request to `/api/v1/public`, **Then** request proceeds to upstream service without authentication
3. **Given** Policy Engine is running, **When** operator updates route policy configuration via xDS stream, **Then** new policies apply to subsequent requests without service restart
4. **Given** Policy chain includes header manipulation policy, **When** request passes through the engine, **Then** specified headers are added, modified, or removed before upstream delivery

---

### User Story 2 - Policy Chain Short-Circuit (Priority: P1)

A platform operator configures policy chains that can terminate request processing early. When JWT validation fails, the engine immediately returns an error response without executing downstream policies or contacting upstream services.

**Why this priority**: Critical for security and performance - prevents unnecessary processing and protects upstream services

**Independent Test**: Configure a policy chain with JWT validation followed by rate limiting. Send invalid JWT and verify no upstream request occurs and rate limiting policy never executes. Delivers security value independently.

**Acceptance Scenarios**:

1. **Given** Policy chain has JWT validation followed by rate limiting and upstream request, **When** JWT validation fails, **Then** immediate 401 response is returned without executing rate limiting or upstream request
2. **Given** Policy chain has API key validation followed by transformation policies, **When** API key is invalid, **Then** immediate 403 response is returned and transformation policies are skipped
3. **Given** Policy returns immediate response, **When** subsequent requests arrive, **Then** they continue to be evaluated independently (short-circuit is per-request)

---

### User Story 3 - Policy Version Management (Priority: P2)

A platform operator maintains multiple versions of the same policy to support gradual rollouts and backward compatibility. Different routes can use different versions of the same policy (e.g., jwtValidation v1.0.0 vs v2.0.0 with enhanced claim extraction).

**Why this priority**: Enables safe evolution of policy logic without breaking existing configurations

**Independent Test**: Deploy two versions of JWT policy. Configure one route with v1.0.0 and another with v2.0.0. Verify both routes work correctly with their respective policy versions. Delivers risk-free policy updates.

**Acceptance Scenarios**:

1. **Given** JWT validation policy exists in v1.0.0 and v2.0.0, **When** route A specifies v1.0.0 and route B specifies v2.0.0, **Then** each route uses its specified version correctly
2. **Given** Policy v2.0.0 adds new configuration parameters, **When** operator configures route with v2.0.0 and new parameters, **Then** policy executes with enhanced behavior
3. **Given** Policy v1.0.0 is deprecated, **When** operator removes v1.0.0 from registry, **Then** routes using v1.0.0 fail with clear error messages indicating version unavailable

---

### User Story 4 - Conditional Policy Execution (Priority: P2)

A platform operator configures policies with execution conditions using CEL expressions. JWT validation only executes for paths starting with `/api/`, rate limiting only applies to write operations (POST/PUT/DELETE), and debug headers are added only in staging environments.

**Why this priority**: Optimizes performance by skipping unnecessary policy evaluations and enables environment-specific behavior

**Independent Test**: Configure JWT policy with path condition `/api/*`. Send requests to `/api/users` and `/health`. Verify JWT validation runs for `/api/users` but not `/health`. Delivers performance optimization independently.

**Acceptance Scenarios**:

1. **Given** JWT policy has condition `request.Path.startsWith("/api/")`, **When** request to `/health` arrives, **Then** JWT policy is skipped and request proceeds without authentication
2. **Given** Rate limiting has condition `request.Method in ["POST", "PUT", "DELETE"]`, **When** GET request arrives, **Then** rate limiting is skipped
3. **Given** Debug headers policy has condition `request.Metadata["environment"] == "staging"`, **When** request arrives in production, **Then** debug headers are not added
4. **Given** Condition references previous policy's metadata, **When** previous policy sets metadata flag, **Then** conditional policy executes only when flag is true

---

### User Story 5 - Custom Policy Development (Priority: P2)

A developer creates a custom policy implementation (e.g., custom authentication, proprietary transformation). They write the policy in Go, define its configuration schema in YAML, and use the Policy Engine Builder to compile a custom engine binary that includes their policy.

**Why this priority**: Extensibility is core value - enables organization-specific policy logic without forking the engine

**Independent Test**: Create minimal custom policy (SetHeader with custom logic). Run Policy Engine Builder with custom policy directory. Deploy resulting binary and verify custom policy executes. Delivers extensibility value independently.

**Acceptance Scenarios**:

1. **Given** Developer writes custom policy implementing RequestPolicy interface, **When** policy is placed in `/policies/custom-auth/v1.0.0/` directory, **Then** Policy Engine Builder discovers the policy
2. **Given** Custom policy has valid `policy.yaml` and `go.mod`, **When** Policy Engine Builder runs validation, **Then** validation passes
3. **Given** Custom policy passes validation, **When** Policy Engine Builder compiles binary, **Then** resulting binary includes custom policy in registry
4. **Given** Custom binary is deployed, **When** route configuration references custom policy by name and version, **Then** custom policy executes correctly in request chain
5. **Given** Custom policy has configuration parameters, **When** operator provides parameters in xDS config, **Then** parameters are validated against policy's schema and passed to policy at execution

---

### User Story 6 - Dynamic Body Processing Optimization (Priority: P3)

The Policy Engine automatically optimizes request/response buffering based on policy requirements. When all policies work with headers only (JWT validation, API key, header manipulation), the engine configures Envoy to skip body buffering for minimal latency. When any policy requires body access (transformation, content inspection), the engine automatically enables buffering.

**Why this priority**: Performance optimization that requires no operator intervention - nice to have but not critical for basic functionality

**Independent Test**: Configure header-only policies and measure latency. Add body-requiring policy and verify buffering automatically enables. Measure latency difference. Delivers performance optimization independently.

**Acceptance Scenarios**:

1. **Given** Policy chain contains only header-based policies, **When** policy chain is built, **Then** RequiresRequestBody flag is false and ext_proc uses SKIP body mode
2. **Given** Policy chain includes request transformation policy, **When** policy chain is built, **Then** RequiresRequestBody flag is true and ext_proc uses BUFFERED body mode
3. **Given** Policy chain has separate request and response policies, **When** only response policies require body, **Then** request uses SKIP mode and response uses BUFFERED mode
4. **Given** Headers-only policy chain is active, **When** operator adds body-requiring policy via xDS, **Then** subsequent requests automatically use BUFFERED mode without restart

---

### User Story 7 - Inter-Policy Communication via Metadata (Priority: P3)

Policies in a chain communicate by reading and writing shared metadata. JWT validation extracts user ID and stores it in metadata. Subsequent rate limiting policy reads the user ID to apply per-user limits. Response phase policies can read metadata set during request phase for coordinated behavior.

**Why this priority**: Enables sophisticated policy coordination without tight coupling - valuable but not required for basic operation

**Independent Test**: Configure JWT policy that writes user ID to metadata and rate limiting policy that reads it. Send authenticated requests and verify rate limiting applies per-user limits. Delivers policy coordination independently.

**Acceptance Scenarios**:

1. **Given** JWT policy stores user ID in metadata, **When** rate limiting policy executes after JWT, **Then** rate limiting can access user ID for per-user limits
2. **Given** Request phase policy sets metadata flag, **When** response phase policies execute, **Then** they can read the flag set during request phase
3. **Given** Policy chain has conditional policy based on metadata, **When** previous policy sets required metadata, **Then** conditional policy executes
4. **Given** Multiple policies write to same metadata key, **When** policies execute in order, **Then** last writer wins and later policies see final value

---

### User Story 8 - Policy Configuration Validation (Priority: P2)

An operator attempts to configure a policy with invalid parameters (e.g., malformed URL, negative rate limit). The xDS configuration service validates parameters against the policy's schema and rejects the configuration with clear error messages before it reaches the Policy Engine.

**Why this priority**: Prevents runtime errors and improves operator experience - important for production stability

**Independent Test**: Attempt to configure JWT policy with invalid jwksUrl (not HTTPS). Verify configuration is rejected with error message indicating requirement. Delivers configuration safety independently.

**Acceptance Scenarios**:

1. **Given** JWT policy requires jwksUrl to be HTTPS, **When** operator provides HTTP URL, **Then** configuration validation fails with error "jwksUrl must be HTTPS"
2. **Given** Rate limiting policy requires positive requestsPerSecond, **When** operator provides negative value, **Then** validation fails with range error
3. **Given** Policy parameter has regex pattern constraint, **When** operator provides value not matching pattern, **Then** validation fails with pattern mismatch error
4. **Given** Policy has required parameters, **When** operator omits required parameter, **Then** validation fails listing missing required fields
5. **Given** Configuration is valid, **When** operator submits configuration, **Then** configuration is accepted and policies execute with validated parameters

---

### Edge Cases

- **Policy execution timeout**: What happens when a policy takes too long to execute? System should have configurable timeout per policy with fallback behavior (fail open/closed).
- **Policy version mismatch**: How does system handle when configuration references policy version not present in binary? System rejects configuration at validation time with clear error.
- **Metadata key collision**: What happens when multiple policies write to same metadata key? Last writer wins, documented in policy execution order.
- **Body size limits**: How does system handle requests/responses exceeding body buffer limits? Configurable limit in Envoy ext_proc filter with size rejection.
- **CEL evaluation errors**: What happens when CEL expression has runtime errors? Policy is skipped and execution continues to next policy in chain.
- **Policy chain empty**: What happens when route has no policies configured? Request passes through unchanged to upstream.
- **xDS connection loss**: How does system behave when xDS configuration stream disconnects? Continues operating with last known good configuration until reconnection.
- **Concurrent policy modifications**: What happens when policy is being updated while requests are processing? Uses copy-on-write semantics - in-flight requests use old version, new requests use new version.
- **Builder validation failure**: What happens when custom policy fails validation? Builder exits with error code and detailed validation report, no binary produced.
- **Policy registration conflict**: What happens when two custom policies register same name and version? Builder detects conflict during discovery phase and fails with clear error.

## Requirements *(mandatory)*

### Functional Requirements

**Policy Execution Engine:**

- **FR-001**: System MUST process HTTP requests through configurable policy chains before forwarding to upstream services
- **FR-002**: System MUST process HTTP responses through configurable policy chains before returning to clients
- **FR-003**: System MUST support policy short-circuiting where a policy can return immediate response and halt further processing
- **FR-004**: System MUST execute policies in the order specified in the policy chain configuration
- **FR-005**: System MUST provide shared metadata storage accessible to all policies in a request-response lifecycle
- **FR-006**: System MUST preserve request context (headers, body, path, method) across policy executions
- **FR-007**: System MUST preserve response context (headers, body, status) from request phase for response phase policies

**Policy Configuration:**

- **FR-008**: System MUST support route-to-policy mapping using metadata keys from Envoy
- **FR-009**: System MUST accept dynamic policy configuration updates via xDS streaming protocol without service restart
- **FR-010**: System MUST support file-based policy configuration for development and testing environments
- **FR-011**: System MUST validate policy parameters against policy-defined schemas at configuration time
- **FR-012**: System MUST reject invalid policy configurations with descriptive error messages before activation
- **FR-013**: System MUST support multiple versions of the same policy coexisting in the same binary
- **FR-014**: System MUST allow policy specifications to declare required and optional configuration parameters with type constraints
- **FR-015**: System MUST support conditional policy execution using CEL expressions evaluated against request/response context

**Policy Types:**

- **FR-016**: System MUST support policies that execute only during request processing phase
- **FR-017**: System MUST support policies that execute only during response processing phase
- **FR-018**: System MUST support policies that execute during both request and response phases
- **FR-019**: System MUST allow policies to modify request headers before upstream delivery
- **FR-020**: System MUST allow policies to modify request body before upstream delivery
- **FR-021**: System MUST allow policies to modify request path and method before upstream delivery
- **FR-022**: System MUST allow policies to modify response headers before client delivery
- **FR-023**: System MUST allow policies to modify response body before client delivery
- **FR-024**: System MUST allow policies to modify response status code before client delivery

**Body Processing Optimization:**

- **FR-025**: System MUST analyze policy chain to determine if request body buffering is required
- **FR-026**: System MUST analyze policy chain to determine if response body buffering is required
- **FR-027**: System MUST configure Envoy ext_proc in SKIP body mode when no policies require body access
- **FR-028**: System MUST configure Envoy ext_proc in BUFFERED body mode when any policy requires body access
- **FR-029**: System MUST update body processing mode dynamically when policy configuration changes

**Policy Registry:**

- **FR-030**: System MUST maintain registry of available policies indexed by name and version
- **FR-031**: System MUST load policy definitions from YAML files at startup
- **FR-032**: System MUST validate policy definition YAML files conform to required schema
- **FR-033**: System MUST support auto-discovery of policies from configured directory structure

**Policy Builder:**

- **FR-034**: Builder MUST accept a policy manifest file (policy-manifest.yaml) that explicitly declares policies to compile
- **FR-035**: Builder MUST validate manifest schema includes required fields (name and uri for each policy)
- **FR-036**: Builder MUST load policies from URIs specified in manifest (supporting relative and absolute paths)
- **FR-037**: Builder MUST validate policy name in manifest matches `policy-definition.yaml` at the URI
- **FR-038**: Builder MUST validate custom policy directory structure at URI (policy.yaml, go.mod, *.go files present)
- **FR-039**: Builder MUST validate custom policy YAML definitions against schema
- **FR-040**: Builder MUST validate custom policy Go code implements required interfaces
- **FR-041**: Builder MUST generate import registry code linking custom policies into engine binary
- **FR-042**: Builder MUST compile final binary with all custom policies included
- **FR-043**: Builder MUST generate runtime Dockerfile for deploying compiled binary
- **FR-044**: Builder MUST fail with detailed error report when validation fails
- **FR-045**: Builder MUST embed build metadata (timestamp, version, loaded policies) in binary
- **FR-046**: Builder MUST support --manifest flag to specify path to policy manifest file

**Builder Architecture Note**: The Policy Engine Builder is distributed as a Docker image that CONTAINS the complete Policy Engine framework source code (`src/`) and a Go-based builder application (`build/`). The builder is implemented in Go (not shell scripts) for better error handling, testability, and maintainability. Users run the Builder image and provide:
1. A policy manifest file (policy-manifest.yaml) via `--manifest` flag that explicitly declares which policies to compile with their names and URIs
2. Policy implementation directories mounted or accessible from the declared URIs

The Builder loads policies from the URIs in the manifest, validates them, and compiles the embedded framework source together with the declared policies to produce the final binary. This manifest-based approach provides explicit control over which policies are included and removes directory structure constraints.

**Sample Policy Implementations (Reference Examples):**

- **FR-047**: SHOULD provide SetHeader reference policy implementation that adds, removes, or modifies request/response headers
- **FR-048**: SHOULD provide JWT validation reference policy that validates tokens using JWKS, issuer, audience, and expiration
- **FR-049**: SHOULD provide API Key validation reference policy that validates keys against configured key store
- **FR-050**: JWT validation reference policy SHOULD extract and inject JWT claims as headers for upstream services
- **FR-051**: JWT validation reference policy SHOULD cache JWKS keys to minimize external lookups

**Note**: Sample policies are NOT bundled with the Policy Engine runtime. They are reference implementations provided separately that users can optionally compile into their custom binary using the Policy Engine Builder. The runtime itself contains ZERO policies by default.

**Error Handling:**

- **FR-052**: System MUST handle policy execution errors gracefully without crashing
- **FR-053**: System MUST support configurable failure modes (fail-open vs fail-closed) per policy
- **FR-054**: System MUST continue processing remaining policies when non-critical policy fails

### Key Entities

- **PolicyChain**: Container holding ordered list of request policies, ordered list of response policies, shared metadata map, and body processing requirement flags
- **PolicySpec**: Configuration instance specifying policy name, version, enabled/disabled flag, validated parameters, and optional CEL execution condition
- **PolicyDefinition**: Schema describing a policy version including name, version, description, parameter schemas, phase support, and body processing requirements
- **RequestContext**: Mutable context for request phase containing headers, body, path, method, request ID, and shared metadata reference
- **ResponseContext**: Context for response phase containing immutable request data, mutable response data (headers, body, status), request ID, and shared metadata reference
- **RequestPolicyAction**: Action returned by request phase policies containing either UpstreamRequestModifications or ImmediateResponse
- **ResponsePolicyAction**: Action returned by response phase policies containing UpstreamResponseModifications
- **PolicyRegistry**: Registry mapping policy name:version keys to PolicyDefinition instances and policy implementations
- **RouteMapping**: Mapping from Envoy metadata key to PolicyChain for route-specific policy execution

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can configure and activate new policy chains without restarting Envoy or Policy Engine services
- **SC-002**: Policy chains with header-only policies process requests with less than 5ms added latency at p95
- **SC-003**: Policy chains requiring body buffering process requests with less than 20ms added latency at p95 for typical payloads (< 100KB)
- **SC-004**: System correctly short-circuits policy execution when authentication/authorization policies fail, preventing unnecessary upstream requests
- **SC-005**: Developers can build custom policy engine binary with sample policies using Builder in under 3 minutes
- **SC-006**: Policy configuration validation catches 100% of schema violations before configuration activation
- **SC-007**: System maintains 99.9% uptime when processing production traffic through policy chains
- **SC-008**: Policy versioning enables safe rollout of policy updates with zero downtime for existing routes
- **SC-009**: System handles 10,000 concurrent requests with policy chains containing up to 5 policies without performance degradation

## Dependencies and Assumptions

### External Dependencies

- **Envoy Proxy v1.36.2**: Policy Engine integrates with Envoy's ext_proc filter
- **gRPC**: Communication protocol for ext_proc and xDS APIs
- **Go 1.23+**: Implementation language and custom policy development language
- **CEL (Common Expression Language)**: Conditional execution expression evaluation

### Assumptions

- **Deployment model**: Policy Engine runs as separate service, not embedded in Envoy process
- **Network reliability**: Policy Engine and Envoy communicate over reliable low-latency network (typically same host or pod)
- **Policy trust**: Custom policies are trusted code - no sandboxing or security isolation between policies
- **Configuration source**: File-based configuration for development/testing environments, xDS stream from trusted control plane for production deployments
- **Body size limits**: Requests/responses requiring body access are limited to reasonable sizes (< 10MB) configured in Envoy
- **Policy execution time**: Policies complete execution within reasonable timeframes (< 1 second) to avoid request timeouts
- **YAML format**: Policy definitions use YAML format, not JSON or protobuf
- **Go interface stability**: Policy interface remains stable across Policy Engine versions to avoid breaking custom policies
- **Build-time policy selection**: Policies to include are determined at build time, not runtime plugin loading
- **Single policy per name-version**: Only one implementation exists for each policy name-version combination

## Future Enhancements (Out of Scope)

The following capabilities are not included in this initial version but may be considered for future releases:

- **Observability & Monitoring**: Structured logging with request IDs, metrics export (Prometheus/OpenTelemetry), execution timing, policy success/failure rates, distributed tracing integration
- **Incremental Builder**: Change detection (content hash or timestamp-based) to rebuild only modified policies, reducing build times for large policy sets
- **Policy plugin system**: Runtime loading of policies without recompilation
- **Distributed policy coordination**: Cross-instance policy state synchronization (e.g., distributed rate limiting)
- **Policy A/B testing framework**: Gradual rollout of policy versions with traffic splitting
- **Web UI for policy configuration**: Graphical interface for policy chain management
- **Policy marketplace**: Repository of pre-built policies for common use cases
- **Multi-language policy support**: Policies written in languages other than Go (e.g., Rust, WASM)
- **Policy debugging tools**: Interactive debugger for stepping through policy execution
- **Advanced CEL functions**: Custom CEL functions for policy-specific operations
- **Policy cost accounting**: Track resource consumption per policy for chargeback
- **Policy compliance reporting**: Audit trails and compliance reports for policy decisions

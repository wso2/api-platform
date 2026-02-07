# Tasks: Envoy Policy Engine System

**Feature**: 001-policy-engine | **Branch**: `001-policy-engine`
**Input**: Design documents from `/specs/001-policy-engine/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Tests are NOT explicitly requested in the specification, therefore test tasks are excluded. The specification focuses on implementation and includes acceptance scenarios within user stories.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

**IMPORTANT - Sample Policies Architecture**:
- The Policy Engine runtime (src/) contains ZERO built-in policies
- ALL policies must be compiled via the Policy Engine Builder
- Tasks creating "sample policies" (SetHeader, JWT, API Key, etc.) are OPTIONAL reference implementations
- These sample policy tasks demonstrate the framework and can be compiled in using the Builder
- Users can build binaries with zero policies, only sample policies, only custom policies, or any combination
- The core framework (kernel + worker + interfaces) is separate from sample policy implementations

## Format: `- [ ] [ID] [P?] [Story?] Description with file path`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, etc.)
- File paths follow multi-component structure from plan.md

## Path Conventions

This project uses a multi-component structure:
- **Policy Engine Runtime**: `src/` (Go service)
- **Sample Policies**: `policies/` (policy implementations)
- **Builder**: `build/`, `templates/`, `tools/` (build tooling)
- **Tests**: `tests/` (unit, integration, contract tests)
- **Configs**: `configs/` (configuration files)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure for all three components (runtime, builder, policies)

- [x] T001 Create project directory structure per plan.md (src/, policies/, build/, tests/, configs/, templates/, tools/)
- [x] T002 Initialize Go module for policy engine runtime in src/go.mod
- [x] T003 [P] Create gateway-builder/Dockerfile for builder image
- [x] T004 [P] Create docker-compose.yml for local development (Envoy + Policy Engine + test backend)
- [x] T005 [P] Create .gitignore for Go project artifacts
- [x] T006 [P] Create basic README.md with project overview and quickstart reference

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure and interfaces that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Core Type Definitions and Interfaces

- [x] T007 [P] Define ParameterType enum and TypedValue struct in sdk/policy/types.go
- [x] T008 [P] Define ValidationRules struct in sdk/policy/types.go
- [x] T009 [P] Define ParameterSchema and PolicyParameters structs in sdk/policy/schema.go
- [x] T010 [P] Define PolicyDefinition and PolicyExample structs in sdk/policy/schema.go
- [x] T011 [P] Define PolicySpec struct in sdk/policy/schema.go
- [x] T012 [P] Define RequestContext struct in sdk/core/context.go
- [x] T013 [P] Define ResponseContext struct in sdk/core/context.go

### Policy Action Types

- [x] T014 [P] Define RequestAction interface with marker method in sdk/core/action.go
- [x] T015 [P] Define ResponseAction interface with marker method in sdk/core/action.go
- [x] T016 [P] Define UpstreamRequestModifications struct implementing RequestAction in sdk/core/action.go
- [x] T017 [P] Define ImmediateResponse struct implementing RequestAction in sdk/core/action.go
- [x] T018 [P] Define UpstreamResponseModifications struct implementing ResponseAction in sdk/core/action.go
- [x] T019 [P] Define RequestPolicyAction and ResponsePolicyAction wrapper structs in sdk/core/action.go

### Policy Interfaces

- [x] T020 [P] Define Policy base interface in sdk/policy/interface.go
- [x] T021 [P] Define RequestPolicy interface in sdk/policy/interface.go
- [x] T022 [P] Define ResponsePolicy interface in sdk/policy/interface.go

### Execution Result Types

- [x] T023 [P] Define RequestPolicyResult and RequestExecutionResult structs in sdk/core/executor.go
- [x] T024 [P] Define ResponsePolicyResult and ResponseExecutionResult structs in sdk/core/executor.go

### PolicyChain and Registry

- [x] T025 Define PolicyChain struct in sdk/core/registry.go
- [x] T026 Define PolicyRegistry struct with definitions and implementations maps in sdk/core/registry.go
- [x] T027 Implement PolicyRegistry.GetDefinition(name, version) method in sdk/core/registry.go
- [x] T028 Implement PolicyRegistry.GetImplementation(name, version) method in sdk/core/registry.go
- [x] T029 Implement PolicyRegistry global singleton initialization in sdk/core/registry.go

### YAML Schema Loader

- [x] T030 Implement LoadPolicyDefinitionFromYAML(path) function in sdk/core/loader.go
- [x] T031 Implement PolicyRegistry.RegisterFromDirectory(path) with filepath.Walk in sdk/core/loader.go
- [x] T032 Implement schema validation for policy.yaml structure in sdk/core/loader.go

### Parameter Validation Engine

- [x] T033 [P] Implement ValidateParameter(value, schema) function in src/pkg/validation/validator.go
- [x] T034 [P] Implement string validation (minLength, maxLength, pattern, format, enum) in src/pkg/validation/string.go
- [x] T035 [P] Implement numeric validation (min, max, multipleOf) in src/pkg/validation/numeric.go
- [x] T036 [P] Implement array validation (minItems, maxItems, uniqueItems) in src/pkg/validation/array.go
- [x] T037 [P] Implement duration validation in src/pkg/validation/duration.go
- [x] T038 [P] Implement format-specific validation (email, uri, hostname, ipv4, ipv6, uuid) in src/pkg/validation/formats.go

### CEL Expression Evaluator

- [x] T039 Define CELEvaluator interface in src/pkg/cel/evaluator.go
- [x] T040 Implement CELEvaluator with google/cel-go for RequestContext evaluation in src/pkg/cel/evaluator.go
- [x] T041 Implement CELEvaluator with google/cel-go for ResponseContext evaluation in src/pkg/cel/evaluator.go
- [x] T042 Implement CEL expression compilation and caching in src/pkg/cel/evaluator.go

### Policy Executor Core

- [x] T043 Implement Core.ExecuteRequestPolicies(policies, ctx) with condition evaluation in sdk/core/executor.go
- [x] T044 Implement Core.ExecuteResponsePolicies(policies, ctx) with condition evaluation in sdk/core/executor.go
- [x] T045 Implement applyRequestModifications(ctx, mods) helper in sdk/core/executor.go
- [x] T046 Implement applyResponseModifications(ctx, mods) helper in sdk/core/executor.go
- [x] T047 Implement short-circuit logic in ExecuteRequestPolicies in sdk/core/executor.go
- [x] T048 Implement policy timing metrics collection in executor in sdk/core/executor.go

### Kernel - Route Mapping

- [x] T049 Define RouteMapping struct in src/kernel/mapper.go
- [x] T050 Define Kernel struct with Routes map in src/kernel/mapper.go
  - **Refactored**: Removed ContextStorage map - replaced with PolicyExecutionContext pattern
- [x] T051 Implement Kernel.GetPolicyChainForKey(key) method in src/kernel/mapper.go
- [x] T050a Define PolicyExecutionContext struct in src/kernel/execution_context.go
  - Manages request-response lifecycle within streaming loop
  - Fields: requestContext, policyChain, requestID, server reference
  - Methods: processRequestHeaders, processRequestBody, processResponseHeaders, processResponseBody, buildRequestContext, buildResponseContext
- [x] T052 ~~Implement Kernel.storeContextForResponse~~ **REMOVED** - Context lives in PolicyExecutionContext (local variable)
- [x] T053 ~~Implement Kernel.getStoredContext~~ **REMOVED** - No storage/retrieval needed
- [x] T054 ~~Implement Kernel.removeStoredContext~~ **REMOVED** - Automatic cleanup via scope

### Kernel - Body Mode Determination

- [x] T055 Implement Kernel.BuildPolicyChain(routeKey, policySpecs) with body requirement computation in src/kernel/body_mode.go
- [x] T056 Implement determineRequestBodyMode(chain) helper in src/kernel/body_mode.go
- [x] T057 Implement determineResponseBodyMode(chain) helper in src/kernel/body_mode.go

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Route-Based Policy Execution (Priority: P1) 🎯 MVP

**Goal**: Enable operators to configure different policy chains for different API routes with dynamic xDS updates

**Independent Test**: Deploy Policy Engine with two routes configured (public with SetHeader, private with JWT). Send requests to both routes and verify correct policies execute per route. Update configuration via xDS and verify new policies apply without restart.

### US1: Kernel - ext_proc gRPC Server

- [x] T058 [US1] Import Envoy ext_proc protobuf definitions in src/kernel/extproc.go
- [x] T059 [US1] Implement ExternalProcessorServer gRPC service struct in src/kernel/extproc.go
- [x] T060 [US1] Implement Process(stream) bidirectional streaming RPC handler in src/kernel/extproc.go
  - **Refactored**: Creates PolicyExecutionContext as local variable, calls handleProcessingPhase()
- [x] T061 [US1] Implement extractMetadataKey(req) from ProcessingRequest in src/kernel/extproc.go
- [x] T062 [US1] ~~Implement ProcessRequest phase handler~~ **REFACTORED** to initializeExecutionContext() + handleProcessingPhase()
  - initializeExecutionContext: Extracts metadata, gets policy chain, generates request ID, creates PolicyExecutionContext
  - handleProcessingPhase: Routes to appropriate phase handler based on request type
- [x] T063 [US1] ~~Implement ProcessResponse phase handler~~ **MOVED** to PolicyExecutionContext.processResponseHeaders()
- [x] T064 [US1] Implement request ID generation in src/kernel/extproc.go

### US1: Kernel - Action Translator

- [x] T065 [US1] Implement TranslateRequestActions(result) for UpstreamRequestModifications in src/kernel/translator.go
- [x] T066 [US1] Implement TranslateRequestActions(result) for ImmediateResponse in src/kernel/translator.go
- [x] T067 [US1] Implement TranslateResponseActions(result) for UpstreamResponseModifications in src/kernel/translator.go
- [x] T068 [US1] Implement buildHeaderMutations(headers, body, path, method) helper in src/kernel/translator.go
- [x] T069 [US1] Implement buildResponseMutations(headers, body, status) helper in src/kernel/translator.go
- [x] T070 [US1] Implement mode override configuration in ext_proc response in src/kernel/translator.go

### US1: Kernel - xDS Configuration Service

- [x] T071 [US1] Import xDS protocol definitions in src/kernel/xds.go
- [x] T072 [US1] Define PolicyChainConfig protobuf message (or use custom struct) in src/kernel/xds.go
- [x] T073 [US1] Implement PolicyDiscoveryService gRPC service struct in src/kernel/xds.go
- [x] T074 [US1] Implement StreamPolicyMappings(stream) handler with snapshot versioning in src/kernel/xds.go
- [x] T075 [US1] Implement configuration validation before applying in src/kernel/xds.go
- [x] T076 [US1] Implement atomic PolicyChain replacement in Routes map in src/kernel/xds.go
- [x] T077 [US1] Implement file-based configuration loader as fallback in src/kernel/xds.go

### US1: Sample Policy - SetHeader (OPTIONAL Reference Implementation)

**Note**: These tasks create a sample/reference policy to demonstrate the framework. This policy is NOT bundled with the runtime - it must be compiled in using the Builder. Users can skip these tasks if they only want the core framework.

- [x] T078 [P] [US1] Create policies/set-header/v1.0.0/ directory structure
- [x] T079 [P] [US1] Create policy.yaml for SetHeader policy in policies/set-header/v1.0.0/policy.yaml
- [x] T080 [P] [US1] Create go.mod for SetHeader policy in policies/set-header/v1.0.0/go.mod
- [x] T081 [US1] Implement SetHeaderPolicy struct in policies/set-header/v1.0.0/setheader.go
- [x] T082 [US1] Implement SetHeaderPolicy.Name() method in policies/set-header/v1.0.0/setheader.go
- [x] T083 [US1] Implement SetHeaderPolicy.Validate(config) method in policies/set-header/v1.0.0/setheader.go
- [x] T084 [US1] Implement SetHeaderPolicy.ExecuteRequest(ctx, config) for SET action in policies/set-header/v1.0.0/setheader.go
- [x] T085 [US1] Implement SetHeaderPolicy.ExecuteRequest(ctx, config) for DELETE action in policies/set-header/v1.0.0/setheader.go
- [x] T086 [US1] Implement SetHeaderPolicy.ExecuteRequest(ctx, config) for APPEND action in policies/set-header/v1.0.0/setheader.go
- [x] T087 [US1] Implement SetHeaderPolicy.ExecuteResponse(ctx, config) for response headers in policies/set-header/v1.0.0/setheader.go
- [x] T088 [US1] Implement GetPolicy() factory function in policies/set-header/v1.0.0/setheader.go
- [x] T089 [P] [US1] Create README.md for SetHeader policy in policies/set-header/v1.0.0/README.md

### US1: Main Entry Point

- [x] T090 [US1] Create src/main.go with gRPC server initialization
- [x] T091 [US1] Implement command-line flags (--extproc-port, --xds-port, --config-file) in src/main.go
- [x] T092 [US1] Implement graceful shutdown handling in src/main.go
- [x] T093 [US1] Wire Kernel and Core components in src/main.go

### US1: Configuration Files

- [x] T094 [P] [US1] Create configs/policy-engine.yaml with runtime configuration
- [x] T095 [P] [US1] Create configs/envoy.yaml with ext_proc filter configuration
- [x] T096 [P] [US1] Create configs/xds/route-simple.yaml with public route + SetHeader policy
- [x] T097 [P] [US1] Create configs/xds/route-with-jwt.yaml with private route + JWT policy (placeholder for US2)

### US1: Docker Compose Setup

- [x] T098 [US1] Configure Envoy service in docker-compose.yml with ext_proc pointing to policy-engine:9001
- [x] T099 [US1] Configure Policy Engine service in docker-compose.yml exposing ports 9001, 9002
- [x] T100 [US1] Configure test backend service (request-info container) in docker-compose.yml
- [x] T101 [US1] Add volume mounts for configs/ directory in docker-compose.yml

**Checkpoint**: At this point, User Story 1 should be fully functional - route-based policy execution with SetHeader policy and dynamic xDS updates

---

## Phase 4: User Story 2 - Policy Chain Short-Circuit (Priority: P1)

**Goal**: Enable policy chains to terminate request processing early when authentication/authorization fails

**Independent Test**: Configure policy chain with JWT validation followed by rate limiting. Send invalid JWT and verify no upstream request occurs and rate limiting policy never executes.

**Dependencies**: Requires US1 (route-based execution) to be complete

### US2: Sample Policy - JWT Validation v1.0.0 (OPTIONAL Reference Implementation)

**Note**: JWT policy is a sample/reference implementation demonstrating authentication. NOT bundled with runtime - must be compiled via Builder.

- [ ] T102 [P] [US2] Create policies/jwt-validation/v1.0.0/ directory structure
- [ ] T103 [P] [US2] Create policy.yaml for JWT validation in policies/jwt-validation/v1.0.0/policy.yaml
- [ ] T104 [P] [US2] Create go.mod with golang-jwt/jwt dependency in policies/jwt-validation/v1.0.0/go.mod
- [ ] T105 [US2] Implement JWTPolicy struct with JWKS cache in policies/jwt-validation/v1.0.0/jwt.go
- [ ] T106 [US2] Implement JWTPolicy.Name() method in policies/jwt-validation/v1.0.0/jwt.go
- [ ] T107 [US2] Implement JWTPolicy.Validate(config) with parameter validation in policies/jwt-validation/v1.0.0/jwt.go
- [ ] T108 [US2] Implement JWKS fetching with HTTP client in policies/jwt-validation/v1.0.0/jwt.go
- [ ] T109 [US2] Implement JWKS caching with TTL in policies/jwt-validation/v1.0.0/jwt.go
- [ ] T110 [US2] Implement JWT parsing and signature validation in policies/jwt-validation/v1.0.0/jwt.go
- [ ] T111 [US2] Implement standard claims validation (iss, aud, exp, nbf) in policies/jwt-validation/v1.0.0/jwt.go
- [ ] T112 [US2] Implement clock skew handling in policies/jwt-validation/v1.0.0/jwt.go
- [ ] T113 [US2] Implement JWTPolicy.ExecuteRequest(ctx, config) returning ImmediateResponse on failure in policies/jwt-validation/v1.0.0/jwt.go
- [ ] T114 [US2] Implement JWTPolicy.ExecuteRequest(ctx, config) storing user info in metadata on success in policies/jwt-validation/v1.0.0/jwt.go
- [ ] T115 [US2] Implement claim extraction and header injection in policies/jwt-validation/v1.0.0/jwt.go
- [ ] T116 [US2] Implement GetPolicy() factory function in policies/jwt-validation/v1.0.0/jwt.go
- [ ] T117 [P] [US2] Create README.md for JWT policy in policies/jwt-validation/v1.0.0/README.md

### US2: Short-Circuit Verification

- [ ] T118 [US2] Update configs/xds/route-with-jwt.yaml to include JWT validation as first policy
- [ ] T119 [US2] Verify Kernel properly handles ImmediateResponse action (already implemented in US1, test here)
- [ ] T120 [US2] Verify Core.ExecuteRequestPolicies stops on StopExecution()=true (already implemented, test here)

**Checkpoint**: At this point, User Story 2 should be fully functional - JWT validation can short-circuit policy chains, preventing unnecessary processing

---

## Phase 5: User Story 3 - Policy Version Management (Priority: P2)

**Goal**: Enable operators to maintain multiple versions of the same policy for gradual rollouts and backward compatibility

**Independent Test**: Deploy two versions of JWT policy (v1.0.0 and v2.0.0 with claim extraction). Configure one route with v1.0.0 and another with v2.0.0. Verify both routes work correctly with their respective versions.

**Dependencies**: Requires US1 (route-based execution) to be complete

### US3: JWT Policy v2.0.0 - Enhanced (OPTIONAL Reference Implementation)

**Note**: JWT v2.0.0 demonstrates policy versioning. Sample implementation only - NOT bundled with runtime.

- [ ] T121 [P] [US3] Create policies/jwt-validation/v2.0.0/ directory structure
- [ ] T122 [P] [US3] Create policy.yaml for JWT v2.0.0 with new parameters (cacheTTL, extractClaims, claimHeaderPrefix) in policies/jwt-validation/v2.0.0/policy.yaml
- [ ] T123 [P] [US3] Create go.mod for JWT v2.0.0 in policies/jwt-validation/v2.0.0/go.mod
- [ ] T124 [US3] Copy JWT v1.0.0 implementation to v2.0.0 as base in policies/jwt-validation/v2.0.0/jwt.go
- [ ] T125 [US3] Add cacheTTL parameter support in policies/jwt-validation/v2.0.0/jwt.go
- [ ] T126 [US3] Implement extractClaims parameter for selective claim extraction in policies/jwt-validation/v2.0.0/jwt.go
- [ ] T127 [US3] Implement claimHeaderPrefix parameter for injected headers in policies/jwt-validation/v2.0.0/jwt.go
- [ ] T128 [US3] Update JWTPolicy.Validate() to validate new parameters in policies/jwt-validation/v2.0.0/jwt.go
- [ ] T129 [P] [US3] Create README.md documenting v2.0.0 enhancements in policies/jwt-validation/v2.0.0/README.md

### US3: Version Selection Configuration

- [ ] T130 [P] [US3] Create configs/xds/route-jwt-v1.yaml using jwtValidation v1.0.0
- [ ] T131 [P] [US3] Create configs/xds/route-jwt-v2.yaml using jwtValidation v2.0.0 with extractClaims
- [ ] T132 [US3] Verify PolicyRegistry correctly indexes by "name:version" composite key (already implemented, test here)
- [ ] T133 [US3] Verify xDS configuration can specify version explicitly (already supported, test here)

**Checkpoint**: At this point, User Story 3 should be fully functional - multiple policy versions coexist and routes can select specific versions

---

## Phase 6: User Story 4 - Conditional Policy Execution (Priority: P2)

**Goal**: Enable operators to configure policies with CEL execution conditions for performance optimization

**Independent Test**: Configure JWT policy with condition `request.Path.startsWith("/api/")`. Send requests to `/api/users` and `/health`. Verify JWT validation runs for `/api/users` but not `/health`.

**Dependencies**: Requires US1 (route-based execution) and US2 (JWT policy) to be complete

### US4: CEL Condition Support (Already Implemented in Foundation)

- [ ] T134 [US4] Verify CEL expression evaluation in Core.ExecuteRequestPolicies (foundation, test here)
- [ ] T135 [US4] Verify CEL expression evaluation in Core.ExecuteResponsePolicies (foundation, test here)
- [ ] T136 [US4] Verify condition skips policy when expression evaluates to false (foundation, test here)

### US4: Configuration with Conditions

- [ ] T137 [P] [US4] Create configs/xds/route-conditional-jwt.yaml with JWT condition `request.Path.startsWith("/api/")`
- [ ] T138 [P] [US4] Create configs/xds/route-conditional-ratelimit.yaml with rate limit condition `request.Method in ["POST", "PUT", "DELETE"]`
- [ ] T139 [US4] Update policy.yaml files to document ExecutionCondition support in examples

### US4: Context Variable Exposure

- [ ] T140 [US4] Verify RequestContext fields are accessible in CEL (Path, Method, Headers) - already implemented, test here
- [ ] T141 [US4] Verify ResponseContext fields are accessible in CEL (ResponseStatus, ResponseHeaders) - already implemented, test here
- [ ] T142 [US4] Verify Metadata is accessible in CEL expressions - already implemented, test here

**Checkpoint**: At this point, User Story 4 should be fully functional - policies can be conditionally executed based on CEL expressions

---

## Phase 7: User Story 5 - Custom Policy Development (Priority: P2)

**Goal**: Enable developers to create custom policy implementations using Go and compile them into the engine via the Builder

**Independent Test**: Create minimal custom policy (custom authentication). Run Policy Engine Builder with custom policy directory. Deploy resulting binary and verify custom policy executes.

**Dependencies**: None (independent of other user stories - requires only foundation)

### US5: Builder - Discovery Phase (Go Implementation)

- [x] T143 [P] [US5] Create build/internal/discovery/discovery.go with policy.yaml file discovery from /policies mount
- [x] T144 [US5] Implement policy.yaml parsing using gopkg.in/yaml.v3 in build/internal/discovery/policy.go
- [x] T145 [US5] Implement DiscoveredPolicy struct and discovery result types in build/pkg/types/policy.go
- [x] T146 [US5] Add directory structure validation (policy.yaml, go.mod, *.go files) in build/internal/discovery/discovery.go
- [x] T147 [US5] Add version consistency check (directory name vs YAML version) in build/internal/discovery/discovery.go

### US5: Builder - Validation Phase (Go Implementation)

- [x] T148 [P] [US5] Create build/internal/validation/validator.go with validation orchestrator
- [x] T149 [US5] Implement YAML schema validation in build/internal/validation/yaml.go
- [x] T150 [US5] Implement policy.yaml required fields validation (name, version, parameters) in build/internal/validation/yaml.go
- [x] T151 [US5] Implement Go interface validation using go/parser in build/internal/validation/golang.go
- [x] T152 [US5] Check for Policy interface implementation (Name, Validate, ExecuteRequest/ExecuteResponse) in build/internal/validation/golang.go
- [x] T153 [US5] Implement directory structure validation (go.mod, *.go files present) in build/internal/validation/structure.go
- [x] T154 [US5] Implement ValidationResult struct with errors and warnings in build/pkg/types/policy.go
- [x] T155 [US5] Implement validation error reporting with file paths and line numbers in build/internal/validation/validator.go
- [x] T156 [US5] Add duplicate policy name/version detection across all discovered policies in build/internal/validation/validator.go

### US5: Builder - Code Generation Phase (Go Implementation)

- [x] T157 [P] [US5] Create build/internal/generation/generator.go with code generation orchestrator
- [x] T158 [P] [US5] Create templates/plugin_registry.go.tmpl template for policy imports
- [x] T159 [US5] Implement plugin_registry.go generation using text/template in build/internal/generation/registry.go
- [x] T160 [US5] Implement import alias generation (sanitize policy name+version to valid Go identifier) in build/internal/generation/registry.go
- [x] T161 [US5] Implement policy registration code generation (import statements + init() registrations) in build/internal/generation/registry.go
- [x] T162 [US5] Implement go.mod replace directive generation for local policy paths in build/internal/generation/gomod.go
- [x] T163 [P] [US5] Create templates/build_info.go.tmpl template for build metadata
- [x] T164 [US5] Implement build_info.go generation with timestamp, version, policy list in build/internal/generation/buildinfo.go

### US5: Builder - Compilation Phase (Go Implementation)

- [x] T165 [P] [US5] Create build/internal/compilation/compiler.go with compilation orchestrator
- [x] T166 [US5] Implement go mod download execution using os/exec in build/internal/compilation/compiler.go
- [x] T167 [US5] Implement go mod tidy execution using os/exec in build/internal/compilation/compiler.go
- [x] T168 [US5] Implement static binary compilation (CGO_ENABLED=0) with os/exec in build/internal/compilation/compiler.go
- [x] T169 [US5] Implement ldflags generation for build metadata injection in build/internal/compilation/options.go
- [x] T170 [US5] Add optional UPX compression execution in build/internal/compilation/compiler.go

### US5: Builder - Packaging Phase (Go Implementation)

- [x] T171 [P] [US5] Create build/internal/packaging/packager.go with Docker image generation
- [x] T172 [P] [US5] Create templates/Dockerfile.policy-engine.tmpl template for runtime image
- [x] T173 [US5] Implement Dockerfile.runtime generation using text/template in build/internal/packaging/packager.go
- [x] T174 [US5] Implement policy list formatting for Docker LABEL in build/internal/packaging/metadata.go
- [x] T175 [US5] Implement build metadata (timestamp, version, builder version) for Docker LABELs in build/internal/packaging/metadata.go

### US5: Builder - Main CLI (Go Implementation)

- [x] T176 [US5] Create build/cmd/builder/main.go with CLI entry point
- [x] T177 [US5] Implement phase 1 (discovery) execution in build/cmd/builder/main.go
- [x] T178 [US5] Implement phase 2 (validation) execution with error handling and early exit in build/cmd/builder/main.go
- [x] T179 [US5] Implement phase 3 (generation) execution in build/cmd/builder/main.go
- [x] T180 [US5] Implement phase 4 (compilation) execution in build/cmd/builder/main.go
- [x] T181 [US5] Implement phase 5 (packaging) execution in build/cmd/builder/main.go
- [x] T182 [US5] Add build banner and summary output with colored/formatted logging in build/cmd/builder/main.go
- [x] T183 [US5] Implement structured error reporting and exit codes in build/pkg/errors/errors.go

### US5: Builder - Docker Image (Go Implementation)

**CRITICAL**: The Builder image CONTAINS the Policy Engine framework source code (`src/`) and Builder Go application (`build/`). Users ONLY mount their policies - NOT the framework source.

- [x] T184 [US5] Create gateway-builder/Dockerfile with golang:1.23-alpine base
- [x] T185 [US5] Install build dependencies (upx for optional compression) in gateway-builder/Dockerfile
- [x] T186 [US5] Copy policy engine framework source code (src/) to /src in builder image in gateway-builder/Dockerfile
- [x] T187 [US5] Copy Builder Go application (build/) and templates (templates/) to builder image in gateway-builder/Dockerfile
- [x] T188 [US5] Build Builder Go binary and set as ENTRYPOINT in gateway-builder/Dockerfile
- [x] T189 [US5] Pre-download Go dependencies for framework and builder in gateway-builder/Dockerfile

### US5: Builder - Module Setup (Go Implementation)

- [x] T190 [P] [US5] Create build/go.mod for Builder Go module
- [x] T191 [P] [US5] Add Builder dependencies (gopkg.in/yaml.v3, text/template) to build/go.mod

### US5: Sample Custom Policy - API Key Validation (OPTIONAL Reference Implementation)

**Note**: API Key policy demonstrates custom policy development. Sample implementation only - NOT bundled with runtime.

- [x] T192 [P] [US5] Create policies/api-key-validation/v1.0.0/ directory structure
- [x] T193 [P] [US5] Create policy.yaml for API Key validation in policies/api-key-validation/v1.0.0/policy.yaml
- [x] T194 [P] [US5] Create go.mod for API Key validation in policies/api-key-validation/v1.0.0/go.mod
- [x] T195 [US5] Implement APIKeyPolicy struct in policies/api-key-validation/v1.0.0/apikey.go
- [x] T196 [US5] Implement APIKeyPolicy.Name() method in policies/api-key-validation/v1.0.0/apikey.go
- [x] T197 [US5] Implement APIKeyPolicy.Validate(config) method in policies/api-key-validation/v1.0.0/apikey.go
- [x] T198 [US5] Implement APIKeyPolicy.ExecuteRequest(ctx, config) with key validation in policies/api-key-validation/v1.0.0/apikey.go
- [x] T199 [US5] Implement ImmediateResponse on invalid key in policies/api-key-validation/v1.0.0/apikey.go
- [x] T200 [US5] Implement metadata storage on valid key in policies/api-key-validation/v1.0.0/apikey.go
- [x] T201 [US5] Implement GetPolicy() factory function in policies/api-key-validation/v1.0.0/apikey.go
- [x] T202 [P] [US5] Create README.md for API Key policy in policies/api-key-validation/v1.0.0/README.md

**Checkpoint**: At this point, User Story 5 should be fully functional - developers can create custom policies and use the Builder to compile custom binaries

---

## Phase 8: User Story 6 - Dynamic Body Processing Optimization (Priority: P3)

**Goal**: Automatically optimize request/response buffering based on policy requirements without operator intervention

**Independent Test**: Configure header-only policies and measure latency (should be SKIP mode). Add body-requiring policy and verify buffering automatically enables (BUFFERED mode). Measure latency difference.

**Dependencies**: Requires US1 (route-based execution) to be complete

### US6: Body Mode Optimization (Already Implemented in Foundation)

- [ ] T203 [US6] Verify PolicyChain.RequiresRequestBody flag computation in BuildPolicyChain (foundation, test here)
- [ ] T204 [US6] Verify PolicyChain.RequiresResponseBody flag computation in BuildPolicyChain (foundation, test here)
- [ ] T205 [US6] Verify mode_override with SKIP mode for headers-only chains (foundation, test here)
- [ ] T206 [US6] Verify mode_override with BUFFERED mode when body required (foundation, test here)

### US6: Policy Definitions with Body Requirements

- [ ] T207 [US6] Verify SetHeader policy.yaml has requiresRequestBody=false and requiresResponseBody=false
- [ ] T208 [US6] Verify JWT policy.yaml has requiresRequestBody=false and requiresResponseBody=false
- [ ] T209 [US6] Verify API Key policy.yaml has requiresRequestBody=false and requiresResponseBody=false

### US6: Sample Body-Requiring Policy - Request Transformation (OPTIONAL Reference Implementation)

**Note**: Request transformation demonstrates body processing. Sample implementation only - NOT bundled with runtime.

- [ ] T210 [P] [US6] Create policies/request-transformation/v1.0.0/ directory structure
- [ ] T211 [P] [US6] Create policy.yaml with requiresRequestBody=true in policies/request-transformation/v1.0.0/policy.yaml
- [ ] T212 [P] [US6] Create go.mod for request transformation in policies/request-transformation/v1.0.0/go.mod
- [ ] T213 [US6] Implement RequestTransformPolicy struct in policies/request-transformation/v1.0.0/transform.go
- [ ] T214 [US6] Implement body transformation logic (e.g., JSON field manipulation) in policies/request-transformation/v1.0.0/transform.go
- [ ] T215 [US6] Implement GetPolicy() factory function in policies/request-transformation/v1.0.0/transform.go

### US6: Configuration for Testing

- [ ] T216 [P] [US6] Create configs/xds/route-headers-only.yaml with SetHeader + JWT (SKIP mode)
- [ ] T217 [P] [US6] Create configs/xds/route-with-transform.yaml with SetHeader + JWT + RequestTransform (BUFFERED mode)

**Checkpoint**: At this point, User Story 6 should be fully functional - body processing mode is automatically optimized based on policy requirements

---

## Phase 9: User Story 7 - Inter-Policy Communication via Metadata (Priority: P3)

**Goal**: Enable policies to communicate by reading and writing shared metadata across request → response lifecycle

**Independent Test**: Configure JWT policy that writes user ID to metadata and rate limiting policy that reads it. Send authenticated requests and verify rate limiting applies per-user limits.

**Dependencies**: Requires US1 (route-based execution), US2 (JWT policy) to be complete

### US7: Metadata Sharing (Already Implemented in Foundation)

- [ ] T218 [US7] Verify PolicyChain.Metadata is shared reference in RequestContext (foundation, test here)
- [ ] T219 [US7] Verify PolicyChain.Metadata is shared reference in ResponseContext (foundation, test here)
- [ ] T220 [US7] Verify Metadata persists from request phase to response phase (foundation, test here)
- [ ] T221 [US7] Verify later policies see metadata written by earlier policies (foundation, test here)

### US7: Sample Policy Using Metadata - Rate Limiting (OPTIONAL Reference Implementation)

**Note**: Rate limiting demonstrates metadata usage. Sample implementation only - NOT bundled with runtime.

- [ ] T222 [P] [US7] Create policies/rate-limiting/v1.0.0/ directory structure
- [ ] T223 [P] [US7] Create policy.yaml for rate limiting in policies/rate-limiting/v1.0.0/policy.yaml
- [ ] T224 [P] [US7] Create go.mod for rate limiting in policies/rate-limiting/v1.0.0/go.mod
- [ ] T225 [US7] Implement RateLimitPolicy struct with token bucket algorithm in policies/rate-limiting/v1.0.0/ratelimit.go
- [ ] T226 [US7] Implement identifier extraction from metadata (user_id) in policies/rate-limiting/v1.0.0/ratelimit.go
- [ ] T227 [US7] Implement identifier extraction from IP in policies/rate-limiting/v1.0.0/ratelimit.go
- [ ] T228 [US7] Implement identifier extraction from header in policies/rate-limiting/v1.0.0/ratelimit.go
- [ ] T229 [US7] Implement token bucket allocation per identifier in policies/rate-limiting/v1.0.0/ratelimit.go
- [ ] T230 [US7] Implement rate limit check with ImmediateResponse(429) in policies/rate-limiting/v1.0.0/ratelimit.go
- [ ] T231 [US7] Implement Retry-After header in rate limit response in policies/rate-limiting/v1.0.0/ratelimit.go
- [ ] T232 [US7] Implement thread-safe token bucket access in policies/rate-limiting/v1.0.0/ratelimit.go
- [ ] T233 [US7] Implement GetPolicy() factory function in policies/rate-limiting/v1.0.0/ratelimit.go
- [ ] T234 [P] [US7] Create README.md for rate limiting policy in policies/rate-limiting/v1.0.0/README.md

### US7: Response Phase Policy Using Request Metadata - Security Headers (OPTIONAL Reference Implementation)

**Note**: Security headers demonstrates response phase policies. Sample implementation only - NOT bundled with runtime.

- [ ] T235 [P] [US7] Create policies/security-headers/v1.0.0/ directory structure
- [ ] T236 [P] [US7] Create policy.yaml for security headers in policies/security-headers/v1.0.0/policy.yaml
- [ ] T237 [P] [US7] Create go.mod for security headers in policies/security-headers/v1.0.0/go.mod
- [ ] T238 [US7] Implement SecurityHeadersPolicy struct in policies/security-headers/v1.0.0/headers.go
- [ ] T239 [US7] Implement standard security headers (X-Content-Type-Options, X-Frame-Options, X-XSS-Protection) in policies/security-headers/v1.0.0/headers.go
- [ ] T240 [US7] Implement authenticated user header from metadata in policies/security-headers/v1.0.0/headers.go
- [ ] T241 [US7] Implement ExecuteResponse() reading metadata from request phase in policies/security-headers/v1.0.0/headers.go
- [ ] T242 [US7] Implement GetPolicy() factory function in policies/security-headers/v1.0.0/headers.go
- [ ] T243 [P] [US7] Create README.md for security headers policy in policies/security-headers/v1.0.0/README.md

### US7: Configuration Demonstrating Metadata Flow

- [ ] T244 [P] [US7] Create configs/xds/route-metadata-flow.yaml with JWT → RateLimit → SecurityHeaders chain
- [ ] T245 [US7] Document metadata keys used by each policy in configs/xds/route-metadata-flow.yaml

**Checkpoint**: At this point, User Story 7 should be fully functional - policies can coordinate via shared metadata

---

## Phase 10: User Story 8 - Policy Configuration Validation (Priority: P2)

**Goal**: Validate policy parameters against schemas at configuration time and reject invalid configurations with clear errors

**Independent Test**: Attempt to configure JWT policy with invalid jwksUrl (not HTTPS). Verify configuration is rejected with error message indicating HTTPS requirement.

**Dependencies**: Requires US1 (route-based execution) to be complete

### US8: Configuration Validation (Already Implemented in Foundation)

- [ ] T246 [US8] Verify parameter validation in xDS configuration handler (foundation, test here)
- [ ] T247 [US8] Verify PolicySpec.Parameters.Validated is populated correctly (foundation, test here)
- [ ] T248 [US8] Verify validation errors are returned to xDS client (foundation, test here)
- [ ] T249 [US8] Verify configuration is rejected on validation failure (foundation, test here)

### US8: Enhanced Validation Messages

- [ ] T250 [US8] Implement clear error messages for type mismatches in src/pkg/validation/validator.go
- [ ] T251 [US8] Implement clear error messages for constraint violations in src/pkg/validation/validator.go
- [ ] T252 [US8] Implement parameter path in error messages (e.g., "jwtValidation.jwksUrl") in src/pkg/validation/validator.go
- [ ] T253 [US8] Implement aggregated validation errors (list all errors, not just first) in src/pkg/validation/validator.go

### US8: Validation Test Cases

- [ ] T254 [P] [US8] Create configs/xds/invalid-jwt-http-url.yaml (should be rejected - HTTP not HTTPS)
- [ ] T255 [P] [US8] Create configs/xds/invalid-ratelimit-negative.yaml (should be rejected - negative requestsPerSecond)
- [ ] T256 [P] [US8] Create configs/xds/invalid-missing-required.yaml (should be rejected - missing jwksUrl)
- [ ] T257 [P] [US8] Create configs/xds/invalid-pattern-mismatch.yaml (should be rejected - pattern validation failure)

**Checkpoint**: At this point, User Story 8 should be fully functional - configuration validation catches schema violations before activation

---

## Phase 11: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple components or overall system quality

### Documentation

- [ ] T258 [P] Update README.md with complete quickstart instructions referencing quickstart.md
- [ ] T259 [P] Create CONTRIBUTING.md with custom policy development guide
- [ ] T260 [P] Create ARCHITECTURE.md documenting kernel/worker/policies architecture
- [ ] T261 [P] Add inline code comments for complex logic (CEL evaluation, action translation)

### Error Handling

- [ ] T262 [P] Define custom error types in src/pkg/errors/types.go
- [ ] T263 [P] Implement error wrapping with context in all components
- [ ] T264 Review and standardize error messages across all packages

### Performance Optimization

- [ ] T265 [P] Implement connection pooling for gRPC streams
- [ ] T266 [P] Implement CEL expression compilation caching (if not already cached)
- [ ] T267 [P] Add context timeouts for policy execution
- [ ] T268 Profile memory allocation in hot paths (executor, translator)

### Testing Infrastructure

- [ ] T269 [P] Create tests/integration/docker-compose.test.yml for integration testing
- [ ] T270 [P] Create tests/integration/scenarios/ directory with test scenarios
- [ ] T271 Create test scenario for User Story 1 (route-based execution) in tests/integration/scenarios/us1_test.sh
- [ ] T272 Create test scenario for User Story 2 (short-circuit) in tests/integration/scenarios/us2_test.sh
- [ ] T273 Create test scenario for User Story 3 (version management) in tests/integration/scenarios/us3_test.sh
- [ ] T274 Create test scenario for User Story 4 (conditional execution) in tests/integration/scenarios/us4_test.sh
- [ ] T275 Create test scenario for User Story 5 (custom policy build) in tests/integration/scenarios/us5_test.sh
- [ ] T276 Create test scenario for User Story 6 (body optimization) in tests/integration/scenarios/us6_test.sh
- [ ] T277 Create test scenario for User Story 7 (metadata flow) in tests/integration/scenarios/us7_test.sh
- [ ] T278 Create test scenario for User Story 8 (validation) in tests/integration/scenarios/us8_test.sh

### Build and Deployment

- [ ] T279 [P] Create Makefile with targets (build, test, docker-build, docker-run)
- [ ] T280 [P] Add GitHub Actions workflow for CI (if using GitHub)
- [ ] T281 [P] Create .dockerignore file
- [ ] T282 Test complete build process from scratch using Builder

### Quickstart Validation

- [ ] T283 Run through quickstart.md steps 1-5 to verify completeness
- [ ] T284 Test custom policy development workflow from quickstart.md
- [ ] T285 Verify all docker-compose services start successfully
- [ ] T286 Test all quickstart test scenarios execute correctly

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational phase completion
- **User Story 2 (Phase 4)**: Depends on Foundational phase + US1 completion (requires route execution)
- **User Story 3 (Phase 5)**: Depends on Foundational phase + US1 completion (requires route execution)
- **User Story 4 (Phase 6)**: Depends on Foundational phase + US1 + US2 completion (requires JWT policy for testing)
- **User Story 5 (Phase 7)**: Depends on Foundational phase completion only (independent builder)
- **User Story 6 (Phase 8)**: Depends on Foundational phase + US1 completion (requires route execution)
- **User Story 7 (Phase 9)**: Depends on Foundational phase + US1 + US2 completion (requires JWT for metadata test)
- **User Story 8 (Phase 10)**: Depends on Foundational phase + US1 completion (requires xDS validation)
- **Polish (Phase 11)**: Depends on all desired user stories being complete

### User Story Dependencies Graph

```
Foundation (Phase 2) ──┬─→ US1 (P1) ──┬─→ US2 (P1) ──┬─→ US4 (P2)
                       │               │              └─→ US7 (P3)
                       │               └─→ US3 (P2)
                       │               └─→ US6 (P3)
                       │               └─→ US8 (P2)
                       └─→ US5 (P2) [Independent]
```

**Critical Path for MVP**: Setup → Foundation → US1 → US2

### Within Each User Story

- **US1**: Kernel components → Sample policy → Main entry → Configs → Docker Compose
- **US2**: JWT policy implementation → Configuration → Verification
- **US3**: v2.0.0 policy → Configuration with version selection
- **US4**: Configuration with conditions (CEL already in foundation)
- **US5**: Builder phases (discovery → validation → generation → compilation → packaging) → Sample custom policy
- **US6**: Verify foundation implementation → Create body-requiring policy for testing
- **US7**: Verify foundation implementation → Create policies using metadata → Configuration
- **US8**: Verify foundation implementation → Enhanced error messages → Invalid test cases

### Parallel Opportunities

**Phase 1 (Setup)**: All tasks marked [P] can run in parallel:
- T003, T004, T005, T006

**Phase 2 (Foundational)**: Many tasks marked [P] can run in parallel:
- All type definition tasks (T007-T024)
- Validation engine tasks (T033-T038)
- CEL evaluator (T039-T042) independent of other tasks

**After Foundation Complete**: All user stories can be worked on in parallel by different team members:
- Developer A: US1 (route-based execution)
- Developer B: US2 (JWT policy)
- Developer C: US5 (builder system)
- Each story is independently completable

**Within User Stories**: Tasks marked [P] can run in parallel within their story:
- US1: T078-T089 (policy files), T094-T097 (config files)
- US2: T102-T104 (policy setup), T117 (README)
- Multiple other [P] tasks throughout

**Phase 11 (Polish)**: Most tasks marked [P] can run in parallel:
- Documentation tasks (T258-T261)
- Error handling tasks (T262-T264)
- Performance tasks (T265-T268)
- Test scenario creation (T271-T278)

---

## Parallel Example: User Story 1

```bash
# Launch kernel components in parallel:
Task: "Implement ExternalProcessorServer gRPC service struct in src/kernel/extproc.go"
Task: "Define PolicyDiscoveryService gRPC service struct in src/kernel/xds.go"
Task: "Implement TranslateRequestActions() in src/kernel/translator.go"

# Launch policy files in parallel:
Task: "Create policies/set-header/v1.0.0/ directory structure"
Task: "Create policy.yaml for SetHeader policy"
Task: "Create go.mod for SetHeader policy"
Task: "Create README.md for SetHeader policy"

# Launch config files in parallel:
Task: "Create configs/policy-engine.yaml"
Task: "Create configs/envoy.yaml"
Task: "Create configs/xds/route-simple.yaml"
Task: "Create configs/xds/route-with-jwt.yaml"
```

---

## Parallel Example: User Story 2

```bash
# Launch policy setup in parallel:
Task: "Create policies/jwt-validation/v1.0.0/ directory structure"
Task: "Create policy.yaml for JWT validation"
Task: "Create go.mod with golang-jwt/jwt dependency"
Task: "Create README.md for JWT policy"
```

---

## Parallel Example: User Story 5 (Builder)

```bash
# Launch builder scripts in parallel:
Task: "Create build/discover.sh script"
Task: "Create build/validate.sh script"
Task: "Create build/generate.sh script"
Task: "Create build/compile.sh script"
Task: "Create build/package.sh script"
Task: "Create build/utils.sh with helper functions"

# Launch template files in parallel:
Task: "Create templates/plugin_registry.go.tmpl"
Task: "Create templates/build_info.go.tmpl"
Task: "Create templates/Dockerfile.policy-engine.tmpl"

# Launch custom policy files in parallel:
Task: "Create policies/api-key-validation/v1.0.0/ directory"
Task: "Create policy.yaml for API Key validation"
Task: "Create go.mod for API Key validation"
Task: "Create README.md for API Key policy"
```

---

## Implementation Strategy

### MVP First (User Stories 1 & 2 Only)

This delivers the core value proposition: route-based policy execution with authentication.

1. Complete Phase 1: Setup (T001-T006)
2. Complete Phase 2: Foundational (T007-T057) **CRITICAL**
3. Complete Phase 3: User Story 1 (T058-T101) - Route-based execution with SetHeader
4. Complete Phase 4: User Story 2 (T102-T120) - JWT validation with short-circuit
5. **STOP and VALIDATE**: Test both user stories independently
6. Deploy/demo if ready

**MVP Deliverables**:
- ✅ Route-specific policy chains
- ✅ Dynamic configuration via xDS
- ✅ JWT authentication
- ✅ Short-circuit on auth failure
- ✅ SetHeader manipulation
- ✅ Working Docker Compose setup

**Task Count for MVP**: ~120 tasks (Setup + Foundation + US1 + US2)

### Incremental Delivery (Add User Stories Progressively)

1. **Foundation** (Setup + Foundational) → All prerequisites ready
2. **+ US1** (Route execution) → Test independently → MVP Core
3. **+ US2** (Short-circuit) → Test independently → MVP Complete ✅
4. **+ US3** (Version management) → Test independently → Deploy
5. **+ US4** (Conditional execution) → Test independently → Deploy
6. **+ US5** (Custom policies) → Test independently → Deploy
7. **+ US6** (Body optimization) → Test independently → Deploy
8. **+ US7** (Metadata sharing) → Test independently → Deploy
9. **+ US8** (Validation) → Test independently → Deploy
10. **Polish** → Complete system

Each addition provides value without breaking previous functionality.

### Parallel Team Strategy

With multiple developers:

1. **Team completes Setup + Foundational together** (critical blocking work)
2. **Once Foundational is done, parallelize**:
   - Developer A: US1 (Route execution)
   - Developer B: US2 (JWT policy) - starts after US1 kernel complete
   - Developer C: US5 (Builder system) - completely independent
   - Developer D: US3 (Version management) - starts after US1 complete
3. **Stories complete independently and integrate**

---

## Summary Statistics

### Total Task Count: 286 tasks

### Tasks per Phase:
- **Phase 1 (Setup)**: 6 tasks
- **Phase 2 (Foundational)**: 49 tasks ⚠️ CRITICAL BLOCKING
- **Phase 3 (US1 - Route Execution)**: 44 tasks 🎯 MVP
- **Phase 4 (US2 - Short-Circuit)**: 19 tasks 🎯 MVP
- **Phase 5 (US3 - Version Management)**: 13 tasks
- **Phase 6 (US4 - Conditional Execution)**: 9 tasks
- **Phase 7 (US5 - Custom Policy Dev)**: 60 tasks (Builder system)
- **Phase 8 (US6 - Body Optimization)**: 14 tasks
- **Phase 9 (US7 - Metadata Communication)**: 28 tasks
- **Phase 10 (US8 - Configuration Validation)**: 12 tasks
- **Phase 11 (Polish)**: 28 tasks

### Parallel Opportunities:
- **Total [P] tasks**: 98 tasks can be executed in parallel with other tasks in same phase
- **Phase 2**: 29 parallelizable foundational tasks
- **Phase 3 (US1)**: 18 parallelizable tasks
- **Phase 7 (US5)**: 20 parallelizable builder tasks
- **Phase 11**: 20 parallelizable polish tasks

### Independent User Stories:
- **US5 (Custom Policy Development)** is completely independent after Foundation
- **US1, US3, US6, US8** require only Foundation
- **US2, US4, US7** have minimal dependencies on previous stories

### Suggested MVP Scope:
**User Stories 1 & 2** (Route Execution + Short-Circuit)
- **Task count**: ~120 tasks (Setup + Foundation + US1 + US2)
- **Estimated effort**: 2-3 weeks for senior engineer
- **Delivers**: Core value - route-based policy execution with authentication

---

## Format Validation

✅ **All 286 tasks follow the required checklist format**:
- Checkbox: `- [ ]`
- Task ID: Sequential T001-T286
- [P] marker: 98 tasks marked as parallelizable
- [Story] label: 231 tasks labeled with user story (US1-US8)
- Description: Clear action with exact file path

✅ **Phase organization**:
- Setup phase: No story labels
- Foundational phase: No story labels
- User Story phases (3-10): All tasks have [US#] labels
- Polish phase: No story labels

✅ **File path specificity**:
- All implementation tasks include exact file paths
- Paths follow multi-component structure from plan.md
- Clear separation: src/, policies/, build/, tests/, configs/

---

## Notes

- No test tasks included (not explicitly requested in specification)
- Acceptance scenarios from spec.md serve as test criteria per user story
- Each user story includes "Independent Test" criteria for validation
- Foundation phase is CRITICAL - blocks all user story work
- US5 (Builder) can proceed in parallel with other stories after Foundation

**IMPORTANT - Sample Policy Architecture**:
- **Policy Engine Runtime** (src/): Core framework ONLY - contains ZERO built-in policies
- **Sample Policies** (policies/): OPTIONAL reference implementations demonstrating the framework
- Sample policy tasks (SetHeader, JWT, API Key, Rate Limiting, etc.) are NOT mandatory
- Users can build binaries with: zero policies, only sample policies, only custom policies, or any combination
- ALL policies (including samples) must be compiled via the Policy Engine Builder
- Sample policies demonstrate framework capabilities but are architecturally separate from runtime

- Configuration examples demonstrate all features
- Docker Compose setup enables immediate local testing
- Builder system enables custom policy extensibility

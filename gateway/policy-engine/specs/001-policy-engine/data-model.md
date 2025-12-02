# Data Model: Envoy Policy Engine

**Feature**: 001-policy-engine | **Date**: 2025-11-18

## Overview

This document defines the core entities and their relationships in the Envoy Policy Engine system. All entities are in-memory data structures (no persistent storage required).

---

## Core Entities

### 1. PolicyChain

**Purpose**: Container for a complete policy processing pipeline for a route

**Fields**:
```go
type PolicyChain struct {
    // Ordered list of policies to execute during request phase
    // Type-filtered at build time - all implement RequestPolicy interface
    RequestPolicies []RequestPolicy

    // Ordered list of policies to execute during response phase
    // Type-filtered at build time - all implement ResponsePolicy interface
    ResponsePolicies []ResponsePolicy

    // Shared metadata map for inter-policy communication
    // Initialized fresh for each request, persists through response phase
    // Key: string, Value: any (policy-specific data)
    Metadata map[string]interface{}

    // Computed flag: true if any RequestPolicy requires request body access
    // Determines whether ext_proc uses SKIP or BUFFERED mode for request body
    RequiresRequestBody bool

    // Computed flag: true if any ResponsePolicy requires response body access
    // Determines whether ext_proc uses SKIP or BUFFERED mode for response body
    RequiresResponseBody bool
}
```

**Lifecycle**:
- Created at configuration time when route policies are registered
- Immutable after creation (except Metadata which is per-request)
- Metadata map is cloned fresh for each request
- Lives in Kernel's route mapping (metadata_key → PolicyChain)

**Relationships**:
- Contains many RequestPolicies (0..N)
- Contains many ResponsePolicies (0..N)
- Referenced by RouteMapping (many-to-one: routes → PolicyChain)

**Validation Rules**:
- RequestPolicies array can be empty (pass-through route)
- ResponsePolicies array can be empty (no response modification)
- If both empty, PolicyChain still valid (no-op chain)
- Metadata map must be initialized (never nil)

---

### 2. PolicySpec

**Purpose**: Configuration instance specifying how to use a policy

**Fields**:
```go
type PolicySpec struct {
    // Policy identifier (e.g., "jwtValidation", "rateLimiting")
    // Must match a registered policy name in PolicyRegistry
    Name string

    // Semantic version of policy implementation (e.g., "v1.0.0", "v2.1.0")
    // Must match a registered policy version in PolicyRegistry
    Version string

    // Static enable/disable toggle
    // If false, policy never executes regardless of ExecutionCondition
    Enabled bool

    // Typed and validated configuration parameters
    // Validated against PolicyDefinition.ParameterSchemas at config time
    Parameters PolicyParameters

    // Optional CEL expression for dynamic conditional execution
    // nil = always execute (when Enabled=true)
    // non-nil = only execute when expression evaluates to true
    // Expression context: RequestContext (request phase) or ResponseContext (response phase)
    ExecutionCondition *string
}
```

**Lifecycle**:
- Created by xDS configuration service when route config is received
- Validated against PolicyDefinition before storage
- Immutable after creation
- Lives in Kernel's PolicyChain

**Relationships**:
- References PolicyDefinition (many-to-one: specs → definition)
- Contained by PolicyChain
- Associated with Policy implementation via Name:Version

**Validation Rules**:
- Name must be non-empty
- Version must follow semver format
- Parameters must validate against PolicyDefinition.ParameterSchemas
- ExecutionCondition (if present) must be valid CEL syntax

---

### 3. PolicyDefinition

**Purpose**: Schema describing a specific version of a policy

**Fields**:
```go
type PolicyDefinition struct {
    // Policy name (e.g., "jwtValidation", "rateLimiting")
    Name string

    // Semantic version of THIS definition (e.g., "v1.0.0", "v2.0.0")
    // Each version gets its own PolicyDefinition
    Version string

    // Human-readable description of what this policy version does
    Description string

    // true if policy supports execution during request phase
    SupportsRequestPhase bool

    // true if policy supports execution during response phase
    SupportsResponsePhase bool

    // true if policy needs access to request body during request phase
    // Kernel uses this to set ext_proc body mode
    RequiresRequestBody bool

    // true if policy needs access to response body during response phase
    // Kernel uses this to set ext_proc body mode
    RequiresResponseBody bool

    // Parameter schemas for THIS version
    // Each schema defines name, type, validation rules
    ParameterSchemas []ParameterSchema

    // Example configurations demonstrating valid usage
    Examples []PolicyExample
}
```

**Lifecycle**:
- Loaded from policy.yaml files at startup
- Immutable after loading
- Lives in PolicyRegistry indexed by "name:version"

**Relationships**:
- One-to-many with PolicySpec (definition → many specs)
- Paired with Policy implementation in PolicyRegistry

**Validation Rules**:
- Name must be non-empty, follow naming convention (camelCase)
- Version must follow semver format
- At least one of SupportsRequestPhase or SupportsResponsePhase must be true
- ParameterSchemas must have unique names within version

---

### 4. PolicyParameters

**Purpose**: Holds policy configuration with type-safe validated values

**Fields**:
```go
type PolicyParameters struct {
    // Raw parameter values as received from xDS config (JSON)
    Raw map[string]interface{}

    // Validated parameters matching the policy's schema
    // Validated at configuration time, not execution time
    // Key: parameter name, Value: typed validated value
    Validated map[string]TypedValue
}
```

**Lifecycle**:
- Created during PolicySpec validation
- Immutable after validation
- Lives in PolicySpec

**Relationships**:
- Contained by PolicySpec (one-to-one)
- Validated against ParameterSchema array

**Validation Rules**:
- All required parameters must be present
- All parameters must match their schema type
- All parameters must satisfy validation constraints

---

### 5. TypedValue

**Purpose**: Represents a validated parameter value with type information

**Fields**:
```go
type TypedValue struct {
    // Parameter type (string, int, float, bool, duration, array, map, uri, email, etc.)
    Type ParameterType

    // Actual value after validation and type conversion
    // Go native type matching ParameterType:
    //   string → string
    //   int → int64
    //   float → float64
    //   bool → bool
    //   duration → time.Duration
    //   string_array → []string
    //   uri → string (validated as URI)
    Value interface{}
}
```

**Lifecycle**:
- Created during parameter validation
- Immutable after creation
- Lives in PolicyParameters.Validated map

**Validation Rules**:
- Type must be valid ParameterType
- Value must match Type (enforced by validation)

---

### 6. ParameterSchema

**Purpose**: Defines validation rules for a policy parameter

**Fields**:
```go
type ParameterSchema struct {
    // Parameter name (e.g., "jwksUrl", "maxRequests", "allowedOrigins")
    Name string

    // Parameter type (string, int, float, duration, array, uri, etc.)
    Type ParameterType

    // Human-readable description for documentation
    Description string

    // true if parameter must be provided in configuration
    Required bool

    // Default value if not provided (must match Type)
    // nil if no default (required parameters should have no default)
    Default interface{}

    // Validation rules based on type
    // Contains type-specific constraints (min/max, pattern, enum, etc.)
    Validation ValidationRules
}
```

**Lifecycle**:
- Loaded from policy.yaml at startup
- Immutable after loading
- Lives in PolicyDefinition.ParameterSchemas array

**Relationships**:
- Contained by PolicyDefinition (many-to-one)

**Validation Rules**:
- Name must be unique within PolicyDefinition
- Type must be valid ParameterType
- Default (if present) must match Type
- Validation rules must be appropriate for Type

---

### 7. RequestContext

**Purpose**: Mutable context for request phase containing current request state

**Fields**:
```go
type RequestContext struct {
    // Current request headers (mutable)
    // Updated in-place as policies execute
    // Key: header name (lowercase), Value: header values (array)
    Headers map[string][]string

    // Current request body (mutable)
    // nil if no body or body not required
    // Updated in-place by body-modifying policies
    Body []byte

    // Current request path (mutable)
    // Can be modified by routing policies
    Path string

    // Current request method (mutable)
    // Can be modified by transformation policies
    Method string

    // Unique request identifier for correlation
    // Generated by Kernel, immutable
    RequestID string

    // Shared metadata for inter-policy communication
    // References PolicyChain.Metadata (same instance)
    // Policies read/write this map to coordinate behavior
    Metadata map[string]interface{}
}
```

**Lifecycle**:
- Created by Kernel when request arrives from Envoy
- Updated in-place as each policy executes
- Stored by Kernel for response phase retrieval
- Destroyed after response phase completes

**Relationships**:
- References PolicyChain.Metadata (shared with ResponseContext)
- Passed to each RequestPolicy.ExecuteRequest()
- Stored in Kernel's context storage (requestID → context)

**State Transitions**:
- Initial: Populated from Envoy ext_proc request
- During execution: Modified by policy actions
- Final: Used to build ext_proc response to Envoy

---

### 8. ResponseContext

**Purpose**: Context for response phase containing request and response state

**Fields**:
```go
type ResponseContext struct {
    // Original request data (immutable, from request phase)
    RequestHeaders map[string][]string
    RequestBody    []byte
    RequestPath    string
    RequestMethod  string

    // Current response headers (mutable)
    // Updated in-place as policies execute
    ResponseHeaders map[string][]string

    // Current response body (mutable)
    // nil if no body or body not required
    ResponseBody []byte

    // Current response status code (mutable)
    ResponseStatus int

    // Request identifier (immutable, from request phase)
    RequestID string

    // Shared metadata from request phase (same reference)
    // Policies can read metadata set during request phase
    // References PolicyChain.Metadata (same instance as RequestContext)
    Metadata map[string]interface{}
}
```

**Lifecycle**:
- Created by Kernel when response arrives from Envoy
- Built from stored RequestContext + Envoy response data
- Updated in-place as each response policy executes
- Destroyed after response sent to client

**Relationships**:
- References PolicyChain.Metadata (shared with RequestContext)
- Built from stored RequestContext
- Passed to each ResponsePolicy.ExecuteResponse()

**State Transitions**:
- Initial: Request data from storage + response data from Envoy
- During execution: Response data modified by policy actions
- Final: Used to build ext_proc response to Envoy

---

### 9. RequestPolicyAction

**Purpose**: Action returned by request phase policies

**Fields**:
```go
type RequestPolicyAction struct {
    // Action to take (oneof: UpstreamRequestModifications or ImmediateResponse)
    // Never nil - always contains one concrete action type
    Action RequestAction  // interface
}

// RequestAction marker interface (oneof pattern)
type RequestAction interface {
    isRequestAction()     // private marker method
    StopExecution() bool  // returns true if execution should stop
}
```

**Concrete Action Types**:

**UpstreamRequestModifications** (continue to upstream):
```go
type UpstreamRequestModifications struct {
    SetHeaders    map[string]string      // Set or replace headers
    RemoveHeaders []string               // Headers to remove
    AppendHeaders map[string][]string    // Headers to append
    Body          []byte                 // nil = no change, []byte{} = clear
    Path          *string                // nil = no change
    Method        *string                // nil = no change
}
// StopExecution() returns false - continue to next policy
```

**ImmediateResponse** (short-circuit):
```go
type ImmediateResponse struct {
    StatusCode int
    Headers    map[string]string
    Body       []byte
}
// StopExecution() returns true - stop chain, return response immediately
```

**Lifecycle**:
- Created by RequestPolicy.ExecuteRequest()
- Consumed by Core executor (updates context or short-circuits)
- Translated by Kernel into ext_proc response format

**Relationships**:
- Returned by RequestPolicy implementations
- Collected in RequestExecutionResult
- Applied to RequestContext by Core

---

### 10. ResponsePolicyAction

**Purpose**: Action returned by response phase policies

**Fields**:
```go
type ResponsePolicyAction struct {
    // Action to take (UpstreamResponseModifications only)
    Action ResponseAction  // interface
}

// ResponseAction marker interface (oneof pattern)
type ResponseAction interface {
    isResponseAction()    // private marker method
    StopExecution() bool  // returns true if execution should stop
}
```

**Concrete Action Types**:

**UpstreamResponseModifications**:
```go
type UpstreamResponseModifications struct {
    SetHeaders    map[string]string      // Set or replace headers
    RemoveHeaders []string               // Headers to remove
    AppendHeaders map[string][]string    // Headers to append
    Body          []byte                 // nil = no change, []byte{} = clear
    StatusCode    *int                   // nil = no change
}
// StopExecution() returns false - continue to next policy
```

**Lifecycle**:
- Created by ResponsePolicy.ExecuteResponse()
- Consumed by Core executor (updates context)
- Translated by Kernel into ext_proc response format

**Relationships**:
- Returned by ResponsePolicy implementations
- Collected in ResponseExecutionResult
- Applied to ResponseContext by Core

---

### 11. PolicyRegistry

**Purpose**: Registry mapping policy name:version to definitions and implementations

**Fields**:
```go
type PolicyRegistry struct {
    // Policy definitions indexed by "name:version" composite key
    // Example key: "jwtValidation:v1.0.0"
    Definitions map[string]*PolicyDefinition

    // Policy implementations indexed by "name:version" composite key
    // Value is Policy interface (can be cast to RequestPolicy/ResponsePolicy)
    Implementations map[string]Policy
}
```

**Lifecycle**:
- Created at startup
- Populated by auto-discovery from policy directories
- Read-only after initialization (no dynamic registration)
- Global singleton in Core

**Relationships**:
- Contains many PolicyDefinitions (one per policy version)
- Contains many Policy implementations (one per policy version)
- Referenced by PolicySpec (lookup by name:version)

**Operations**:
- `GetDefinition(name, version) *PolicyDefinition`
- `GetImplementation(name, version) Policy`
- `RegisterFromDirectory(path string) error`
- `ValidatePolicy(name, version string) error`

---

### 12. RouteMapping

**Purpose**: Maps Envoy metadata keys to PolicyChains for route-specific processing

**Fields**:
```go
type RouteMapping struct {
    // Metadata key from Envoy (route identifier)
    // Example: "api-v1-private", "public-endpoint"
    MetadataKey string

    // PolicyChain to execute for this route
    // Contains both request and response policies
    Chain *PolicyChain
}
```

**Lifecycle**:
- Created when route configuration is received via xDS
- Updated when configuration changes (atomic swap)
- Lives in Kernel's mapping store

**Relationships**:
- References PolicyChain (many-to-one: routes → chain)
- Indexed by MetadataKey in Kernel

**Storage Structure** (in Kernel):
```go
type Kernel struct {
    // Route-to-chain mapping
    // Key: metadata key from Envoy
    // Value: PolicyChain for that route
    Routes map[string]*PolicyChain
}
```

**Note**: Request context storage has been removed. Instead, `PolicyExecutionContext` manages the request-response lifecycle within the streaming RPC loop, eliminating the need for explicit context storage and cleanup.

---

### 13. PolicyExecutionContext

**Purpose**: Manages the lifecycle of a single request through the policy chain

**Fields**:
```go
type PolicyExecutionContext struct {
    // Request context that carries request data and metadata
    requestContext *RequestContext

    // Policy chain for this request
    policyChain *PolicyChain

    // Request ID for correlation
    requestID string

    // Reference to server components
    server *ExternalProcessorServer
}
```

**Lifecycle**:
- Created when request headers arrive from Envoy
- Lives as a local variable in the Process() streaming loop
- Automatically destroyed when the stream iteration completes (after response)
- No explicit storage or cleanup needed

**Relationships**:
- Contains RequestContext (builds and owns it)
- References PolicyChain (from Kernel's route mapping)
- References ExternalProcessorServer (for accessing core engine)

**Methods**:
- `processRequestHeaders(ctx, headers) *ProcessingResponse` - Handles request header phase
- `processRequestBody(ctx, body) *ProcessingResponse` - Handles request body phase
- `processResponseHeaders(ctx, headers) *ProcessingResponse` - Handles response header phase
- `processResponseBody(ctx, body) *ProcessingResponse` - Handles response body phase
- `buildRequestContext(headers) *RequestContext` - Converts Envoy headers to RequestContext
- `buildResponseContext(headers) *ResponseContext` - Builds ResponseContext from stored request data

**Design Rationale**:
This context encapsulates all state needed for a single request-response cycle. By keeping it as a local variable in the streaming loop rather than storing it in a global map, we achieve:
- Automatic lifecycle management (no manual cleanup needed)
- Better memory management (GC handles cleanup)
- Simpler code flow (no storage/retrieval logic)
- Thread safety (each request has its own context)

---

## Entity Relationship Diagram

```
┌─────────────────┐
│  Kernel         │
│  ┌──────────┐   │
│  │ Routes   │───┼───> [metadata_key → PolicyChain]
│  └──────────┘   │
│  ┌──────────┐   │
│  │ Context  │───┼───> [request_id → (RequestContext, PolicyChain)]
│  │ Storage  │   │
│  └──────────┘   │
└─────────────────┘
         │
         │ references
         ↓
┌─────────────────────────────────────┐
│  PolicyChain                        │
│  ┌────────────────┐                 │
│  │RequestPolicies │ [Policy...]     │
│  └────────────────┘                 │
│  ┌────────────────┐                 │
│  │ResponsePolicies│ [Policy...]     │
│  └────────────────┘                 │
│  ┌────────────────┐                 │
│  │Metadata        │ map[string]any  │<─┐
│  └────────────────┘                 │  │
│  RequiresRequestBody: bool          │  │
│  RequiresResponseBody: bool         │  │
└─────────────────────────────────────┘  │
         │ contains                       │ references
         ↓                                │
┌─────────────────────────────────────┐  │
│  PolicySpec                         │  │
│  Name: string                       │  │
│  Version: string                    │  │
│  Enabled: bool                      │  │
│  ┌────────────────┐                 │  │
│  │Parameters      │                 │  │
│  │  Raw           │                 │  │
│  │  Validated     │ [TypedValue...] │  │
│  └────────────────┘                 │  │
│  ExecutionCondition: *string        │  │
└─────────────────────────────────────┘  │
         │ references                     │
         ↓                                │
┌──────────────────────────────────────┐ │
│  PolicyRegistry                      │ │
│  ┌──────────────┐                    │ │
│  │Definitions   │ [PolicyDefinition] │ │
│  └──────────────┘                    │ │
│  ┌──────────────┐                    │ │
│  │Implementat...│ [Policy]           │ │
│  └──────────────┘                    │ │
└──────────────────────────────────────┘ │
         │ contains                       │
         ↓                                │
┌──────────────────────────────────────┐ │
│  PolicyDefinition                    │ │
│  Name: string                        │ │
│  Version: string                     │ │
│  Description: string                 │ │
│  SupportsRequestPhase: bool          │ │
│  SupportsResponsePhase: bool         │ │
│  RequiresRequestBody: bool           │ │
│  RequiresResponseBody: bool          │ │
│  ┌──────────────┐                    │ │
│  │Param Schemas │ [ParameterSchema]  │ │
│  └──────────────┘                    │ │
│  Examples: [PolicyExample]           │ │
└──────────────────────────────────────┘ │
                                         │
┌──────────────────────────────────────┐ │
│  RequestContext                      │ │
│  Headers: map[string][]string        │ │
│  Body: []byte                        │ │
│  Path: string                        │ │
│  Method: string                      │ │
│  RequestID: string                   │ │
│  Metadata: map[string]any ───────────┼─┘
└──────────────────────────────────────┘
         │ transforms to
         ↓
┌──────────────────────────────────────┐
│  ResponseContext                     │
│  RequestHeaders: map (immutable)     │
│  RequestBody: []byte (immutable)     │
│  RequestPath: string (immutable)     │
│  RequestMethod: string (immutable)   │
│  ResponseHeaders: map (mutable)      │
│  ResponseBody: []byte (mutable)      │
│  ResponseStatus: int (mutable)       │
│  RequestID: string                   │
│  Metadata: map[string]any ───────────┼─┘
└──────────────────────────────────────┘
```

---

## Key Design Patterns

### 1. Oneof Pattern (Action Types)

Request and response actions use Go interfaces to enforce oneof semantics:
- RequestAction: UpstreamRequestModifications OR ImmediateResponse
- ResponseAction: UpstreamResponseModifications only
- Type safety via private marker methods prevents invalid combinations

### 2. Shared Metadata Pattern

PolicyChain.Metadata is shared across request → response lifecycle:
- RequestContext.Metadata references same map
- ResponseContext.Metadata references same map
- Enables inter-policy communication without tight coupling

### 3. Registry Pattern

PolicyRegistry provides centralized policy lookup:
- Composite key indexing: "name:version"
- Immutable after initialization
- Thread-safe read-only access

### 4. Builder Pattern (PolicyChain Construction)

PolicyChain is constructed with computed flags:
- RequiresRequestBody: OR of all RequestPolicy.RequiresRequestBody
- RequiresResponseBody: OR of all ResponsePolicy.RequiresResponseBody
- Enables dynamic ext_proc mode selection

---

## Data Flow

### Request Phase
1. Envoy sends request headers → ExternalProcessorServer.Process()
2. Process() calls initializeExecutionContext():
   - Extracts metadata key from request
   - Looks up PolicyChain from Kernel
   - Generates request ID
   - Creates PolicyExecutionContext (local variable in loop)
3. Process() calls execCtx.processRequestHeaders():
   - Builds RequestContext from Envoy headers
   - Calls Core.ExecuteRequestPolicies(chain.RequestPolicies, ctx)
   - Core executes each policy, updates ctx in-place
   - Returns RequestExecutionResult with actions
   - Translates actions → ext_proc response
4. execCtx remains in scope for subsequent phases

### Response Phase
1. Envoy sends response headers → ExternalProcessorServer.Process()
2. Process() calls execCtx.processResponseHeaders():
   - Builds ResponseContext from stored RequestContext + Envoy response headers
   - Calls Core.ExecuteResponsePolicies(chain.ResponsePolicies, ctx)
   - Core executes each policy, updates ctx in-place
   - Returns ResponseExecutionResult with actions
   - Translates actions → ext_proc response
3. Stream completes, execCtx goes out of scope (automatic cleanup)

---

## State Machines

### Policy Execution States
```
[PENDING] → [CONDITION_CHECK] → [EXECUTING] → [COMPLETED]
                    ↓ (condition false)
                [SKIPPED]
                    ↓ (error)
                [FAILED]
```

### Request Processing States
```
[RECEIVED] → [MAPPED] → [EXECUTING_POLICIES] → [COMPLETED]
                              ↓ (short-circuit)
                         [SHORT_CIRCUITED]
```

### Configuration States
```
[RAW_CONFIG] → [VALIDATING] → [VALIDATED] → [ACTIVE]
                     ↓ (error)
                 [REJECTED]
```

---

## Invariants

1. **PolicyChain**: Metadata map never nil, always initialized
2. **RequestContext**: Metadata references PolicyChain.Metadata (same instance)
3. **ResponseContext**: Metadata references PolicyChain.Metadata (same instance)
4. **PolicySpec**: Parameters validated against PolicyDefinition before storage
5. **PolicyRegistry**: Immutable after initialization, thread-safe reads
6. **RouteMapping**: Atomic updates (replace entire PolicyChain, no partial updates)
7. **PolicyExecutionContext**: Created once per request-response cycle, lives as local variable in Process() loop
8. **Action Application**: Actions applied sequentially, later actions see earlier modifications
9. **Context Lifecycle**: RequestContext created in request phase, accessible in response phase via PolicyExecutionContext

---

This data model supports all functional requirements while maintaining simplicity, type safety, and performance.

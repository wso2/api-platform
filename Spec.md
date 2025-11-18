# Envoy Policy Engine - Product Requirements Document

**Version:** 1.0
**Date:** 2025-11-17
**Target Envoy Version:** v1.36.2
**Implementation Language:** Go

---

## 1. Overview

### 1.1 Purpose
The Envoy Policy Engine is an external processor (ext_proc) service for Envoy Proxy that provides a flexible, extensible framework for processing HTTP requests and responses through configurable policies. The engine allows dynamic configuration of policy chains per route, enabling capabilities such as authentication, authorization, header manipulation, and request transformation without modifying Envoy configuration.

### 1.2 Goals
- Provide a clean separation between policy logic and proxy infrastructure
- Enable dynamic policy configuration without restarting Envoy or the policy engine
- Support extensible policy framework through Go interfaces
- Achieve low-latency policy evaluation
- Allow policy composition and chaining with failure handling
- Integrate seamlessly with Envoy's ext_proc filter

### 1.3 High-Level Architecture

```mermaid
graph TB
    subgraph Envoy["Envoy Proxy (v1.36.2, port 8000)"]
        ExtProc["ext_proc Filter<br/>Port: 9001<br/>Sends: metadata key +<br/>request/response data"]
    end

    subgraph PolicyEngine["Policy Engine (Go Service)"]
        subgraph Kernel["KERNEL"]
            GRPCServer["gRPC Server<br/>(ext_proc implementation)"]
            Mapper["Route-to-Policy Mapping<br/>(by metadata key)"]
            Translator["Action → ext_proc<br/>Response Translator"]
            ConfigAPI["Configuration<br/>Management API"]
        end

        subgraph WorkerCore["WORKER: CORE"]
            Registry["Policy Registry"]
            Executor["Policy Chain Executor<br/>(with short-circuit logic)"]
            ActionCollector["Action Collector"]
        end

        subgraph WorkerPolicies["WORKER: POLICIES"]
            SetHeader["SetHeader Policy"]
            JWT["JWT Validation Policy"]
            APIKey["API Key Validation Policy"]
            Transform["Request Transformation Policy"]
            Extensible["[Extensible via<br/>Policy Interface]"]
        end
    end

    ExtProc -->|"gRPC"| Kernel
    Kernel -->|"Executes"| WorkerCore
    WorkerCore -->|"Invokes"| WorkerPolicies

    style Envoy fill:#e1f5ff,stroke:#01579b,stroke-width:2px
    style PolicyEngine fill:#f3e5f5,stroke:#4a148c,stroke-width:2px
    style Kernel fill:#fff9c4,stroke:#f57f17,stroke-width:2px
    style WorkerCore fill:#c8e6c9,stroke:#1b5e20,stroke-width:2px
    style WorkerPolicies fill:#ffccbc,stroke:#bf360c,stroke-width:2px
```

### 1.4 Deployment Architecture

```mermaid
graph TB
    subgraph DockerCompose["Docker Compose Environment"]
        subgraph EnvoyContainer["Envoy Container<br/>(envoyproxy/envoy:v1.36.2)"]
            EnvoyProxy["Envoy Proxy<br/>:8000 (HTTP)<br/>:9000 (Admin)"]
        end

        subgraph PolicyEngineContainer["Policy Engine Container<br/>(Go Service)"]
            GRPCPort[":9001 (gRPC)<br/>ext_proc endpoint"]
            XDSPort[":9002 (gRPC)<br/>xDS Policy Config API"]
        end

        subgraph TestService["Request Info Container<br/>(Test Backend)"]
            Backend[":8080 (HTTP)<br/>Test Service"]
        end
    end

    Client["Client"] -->|":8000"| EnvoyProxy
    EnvoyProxy -->|"ext_proc<br/>gRPC"| GRPCPort
    EnvoyProxy -->|"Upstream<br/>HTTP"| Backend
    Admin["Admin/DevOps"] -->|"xDS Config<br/>gRPC Stream"| XDSPort

    style DockerCompose fill:#f5f5f5,stroke:#424242,stroke-width:3px
    style EnvoyContainer fill:#e1f5ff,stroke:#01579b,stroke-width:2px
    style PolicyEngineContainer fill:#c8e6c9,stroke:#1b5e20,stroke-width:2px
    style TestService fill:#fff9c4,stroke:#f57f17,stroke-width:2px
    style Client fill:#ffccbc,stroke:#bf360c
    style Admin fill:#ffccbc,stroke:#bf360c
```

---

## 2. Architecture Components

### 2.1 Kernel

**Responsibilities:**
- Implement Envoy ext_proc gRPC service (port 9001)
- Maintain route-to-policy mappings (key → request policy list, response policy list)
- Extract metadata key from Envoy requests
- Invoke Worker Core with appropriate policy chain (request or response flow)
- Translate policy results to ext_proc responses (Core → Envoy format)
- Expose xDS-based Policy Discovery Service (gRPC streaming, port 9002)

**Key Functions:**
- `ProcessRequest()` - Handle request phase from Envoy (ext_proc)
  - Extract metadata key from request
  - Get request policy list for route
  - Call Core.ExecuteRequestPolicies()
  - Translate `RequestExecutionResult` → ext_proc response
- `ProcessResponse()` - Handle response phase from Envoy (ext_proc)
  - Extract metadata key from request
  - Get response policy list for route
  - Call Core.ExecuteResponsePolicies()
  - Translate `ResponseExecutionResult` → ext_proc response
- `GetRequestPoliciesForKey(key string)` - Retrieve request policy chain for route
- `GetResponsePoliciesForKey(key string)` - Retrieve response policy chain for route
- `StreamPolicyMappings()` - xDS stream for policy configuration updates
- `TranslatePolicyActions()` - Convert execution results to ext_proc format

**Configuration Storage:**
- In-memory map: `metadata_key → RouteConfig`
- RouteConfig contains:
  - `RequestPolicies []RequestPolicy` - Policies executed during request flow (type-safe)
  - `ResponsePolicies []ResponsePolicy` - Policies executed during response flow (type-safe)
- PolicySpec includes: policy name, version, validated parameters, enabled flag
- Version tracking for xDS protocol (resource version strings)
- Policies are filtered by interface type when loaded into RouteConfig

### 2.1.1 Policy Configuration Schema

Following industry standards (Kubernetes CRDs, OpenAPI v3, Protobuf validation), the engine uses a robust parameter typing and validation system.

**Core Configuration Types:**

```go
// PolicySpec defines a policy instance with version and validated parameters
type PolicySpec struct {
    // Policy identifier (e.g., "jwtValidation", "rateLimiting")
    Name string

    // Semantic version of the policy implementation (e.g., "v1.0.0", "v2.1.0")
    // Enables backward compatibility and gradual rollouts
    Version string

    // Whether this policy is active
    Enabled bool

    // Typed and validated configuration parameters
    Parameters PolicyParameters
}

// PolicyParameters holds the configuration with type-safe access
type PolicyParameters struct {
    // Raw parameter values as JSON (from xDS config)
    Raw map[string]interface{}

    // Validated parameters matching the policy's schema
    // Validated at configuration time, not execution time
    Validated map[string]TypedValue
}

// TypedValue represents a validated parameter value with its type information
type TypedValue struct {
    Type  ParameterType
    Value interface{}  // Actual value after validation
}
```

**Parameter Type System:**

```go
// ParameterType defines supported parameter types
type ParameterType string

const (
    // Scalar Types
    ParameterTypeString     ParameterType = "string"
    ParameterTypeInt        ParameterType = "int"        // int64
    ParameterTypeFloat      ParameterType = "float"      // float64
    ParameterTypeBool       ParameterType = "bool"
    ParameterTypePercentage ParameterType = "percentage" // float64 [0.0-100.0]
    ParameterTypeDuration   ParameterType = "duration"   // time.Duration

    // Complex Types
    ParameterTypeStringArray ParameterType = "string_array"
    ParameterTypeIntArray    ParameterType = "int_array"
    ParameterTypeMap         ParameterType = "map"        // map[string]interface{}
    ParameterTypeObject      ParameterType = "object"     // nested structure

    // Format-Specific Types (strings with validation)
    ParameterTypeEmail      ParameterType = "email"       // RFC 5322 email
    ParameterTypeURI        ParameterType = "uri"         // RFC 3986 URI
    ParameterTypeHostname   ParameterType = "hostname"    // RFC 1123 hostname
    ParameterTypeIPv4       ParameterType = "ipv4"        // IPv4 address
    ParameterTypeIPv6       ParameterType = "ipv6"        // IPv6 address
    ParameterTypeUUID       ParameterType = "uuid"        // RFC 4122 UUID
    ParameterTypeJSONPath   ParameterType = "jsonpath"    // JSONPath expression
    ParameterTypeRegex      ParameterType = "regex"       // Valid regex pattern
)
```

**Validation Schema:**

```go
// ParameterSchema defines the validation rules for a policy parameter
// Follows OpenAPI v3 / JSON Schema conventions
type ParameterSchema struct {
    // Parameter name (e.g., "jwksUrl", "maxRequests", "allowedOrigins")
    Name string

    // Parameter type
    Type ParameterType

    // Human-readable description
    Description string

    // Whether this parameter is required
    Required bool

    // Default value if not provided (must match Type)
    Default interface{}

    // Validation rules based on type
    Validation ValidationRules
}

// ValidationRules contains type-specific validation constraints
type ValidationRules struct {
    // String validations
    MinLength *int    // Minimum string length
    MaxLength *int    // Maximum string length
    Pattern   *string // Regex pattern (Go regexp syntax)
    Format    *string // Predefined format (email, uri, hostname, etc.)
    Enum      []string // Allowed values

    // Numeric validations (int, float, percentage)
    Minimum          *float64 // Minimum value (inclusive)
    Maximum          *float64 // Maximum value (inclusive)
    ExclusiveMinimum *float64 // Minimum value (exclusive)
    ExclusiveMaximum *float64 // Maximum value (exclusive)
    MultipleOf       *float64 // Value must be multiple of this

    // Array validations
    MinItems *int  // Minimum array length
    MaxItems *int  // Maximum array length
    UniqueItems bool // All items must be unique

    // Duration validations
    MinDuration *time.Duration // Minimum duration
    MaxDuration *time.Duration // Maximum duration

    // Object/Map validations
    MinProperties *int // Minimum number of properties
    MaxProperties *int // Maximum number of properties

    // Custom validation using CEL (Common Expression Language)
    // Example: "this.maxRequests > 0 && this.maxRequests <= 10000"
    CELExpression *string
}
```

**Policy Definition & Versioning:**

```go
// PolicyDefinition describes a specific version of a policy
// Each version is a separate PolicyDefinition with its own schema
type PolicyDefinition struct {
    // Policy name (e.g., "jwtValidation", "rateLimiting")
    Name string

    // Semantic version of THIS definition (e.g., "v1.0.0", "v2.0.0")
    // Each version gets its own PolicyDefinition
    Version string

    // Description of what this policy version does
    Description string

    // Which phases this policy version supports
    SupportsRequestPhase  bool
    SupportsResponsePhase bool

    // Parameter schemas for THIS version
    ParameterSchemas []ParameterSchema

    // Examples of valid configurations for THIS version
    Examples []PolicyExample
}

// PolicyExample provides a documented example configuration
type PolicyExample struct {
    Description string
    Config      map[string]interface{}
}

// PolicyRegistry stores policy definitions by composite key
type PolicyRegistry struct {
    // Key format: "policyName:version" (e.g., "jwtValidation:v1.0.0")
    definitions map[string]*PolicyDefinition

    // Alternative nested structure for easier version lookup:
    // map[policyName]map[version]*PolicyDefinition
}
```

**Example Policy Definitions:**

```go
// JWT Validation v1.0.0 - Basic validation
var JWTValidationV1 = PolicyDefinition{
    Name:        "jwtValidation",
    Version:     "v1.0.0",
    Description: "Validates JWT tokens using JWKS",
    SupportsRequestPhase:  true,
    SupportsResponsePhase: false,
    ParameterSchemas: []ParameterSchema{
        {
            Name:        "headerName",
            Type:        ParameterTypeString,
            Description: "HTTP header containing the JWT",
            Required:    true,
            Default:     "Authorization",
            Validation: ValidationRules{
                MinLength: ptr(1),
                MaxLength: ptr(256),
                Pattern:   ptr("^[A-Za-z0-9-]+$"),
            },
        },
        {
            Name:        "tokenPrefix",
            Type:        ParameterTypeString,
            Description: "Prefix to strip from header value (e.g., 'Bearer ')",
            Required:    false,
            Default:     "Bearer ",
        },
        {
            Name:        "jwksUrl",
            Type:        ParameterTypeURI,
            Description: "URL to fetch JSON Web Key Set",
            Required:    true,
            Validation: ValidationRules{
                Pattern: ptr("^https://.*"),
            },
        },
        {
            Name:        "issuer",
            Type:        ParameterTypeString,
            Description: "Expected JWT issuer claim",
            Required:    true,
            Validation: ValidationRules{
                MinLength: ptr(1),
                MaxLength: ptr(256),
            },
        },
        {
            Name:        "audiences",
            Type:        ParameterTypeStringArray,
            Description: "Accepted audience values",
            Required:    true,
            Validation: ValidationRules{
                MinItems: ptr(1),
                MaxItems: ptr(10),
            },
        },
        {
            Name:        "clockSkew",
            Type:        ParameterTypeDuration,
            Description: "Allowed clock skew for exp/nbf validation",
            Required:    false,
            Default:     "30s",
            Validation: ValidationRules{
                MinDuration: ptr(0 * time.Second),
                MaxDuration: ptr(5 * time.Minute),
            },
        },
    },
    Examples: []PolicyExample{
        {
            Description: "Basic JWT validation with Auth0",
            Config: map[string]interface{}{
                "headerName":  "Authorization",
                "tokenPrefix": "Bearer ",
                "jwksUrl":     "https://your-tenant.auth0.com/.well-known/jwks.json",
                "issuer":      "https://your-tenant.auth0.com/",
                "audiences":   []string{"https://api.example.com"},
                "clockSkew":   "30s",
            },
        },
    },
}

// JWT Validation v2.0.0 - Extended with caching and claim extraction
var JWTValidationV2 = PolicyDefinition{
    Name:        "jwtValidation",
    Version:     "v2.0.0",
    Description: "Validates JWT tokens with advanced caching and claim extraction",
    SupportsRequestPhase:  true,
    SupportsResponsePhase: false,
    ParameterSchemas: []ParameterSchema{
        // All v1.0.0 parameters (inherited conceptually)
        {Name: "headerName", Type: ParameterTypeString, Required: true, Default: "Authorization"},
        {Name: "tokenPrefix", Type: ParameterTypeString, Required: false, Default: "Bearer "},
        {Name: "jwksUrl", Type: ParameterTypeURI, Required: true},
        {Name: "issuer", Type: ParameterTypeString, Required: true},
        {Name: "audiences", Type: ParameterTypeStringArray, Required: true},
        {Name: "clockSkew", Type: ParameterTypeDuration, Required: false, Default: "30s"},

        // New parameters in v2.0.0
        {
            Name:        "cacheTTL",
            Type:        ParameterTypeDuration,
            Description: "How long to cache JWKS keys",
            Required:    false,
            Default:     "1h",
            Validation: ValidationRules{
                MinDuration: ptr(1 * time.Minute),
                MaxDuration: ptr(24 * time.Hour),
            },
        },
        {
            Name:        "extractClaims",
            Type:        ParameterTypeStringArray,
            Description: "JWT claims to extract and inject as headers",
            Required:    false,
            Validation: ValidationRules{
                MaxItems: ptr(20),
            },
        },
        {
            Name:        "claimHeaderPrefix",
            Type:        ParameterTypeString,
            Description: "Prefix for injected claim headers",
            Required:    false,
            Default:     "X-JWT-",
        },
    },
}

// Rate Limiting v1.0.0
var RateLimitingV1 = PolicyDefinition{
    Name:        "rateLimiting",
    Version:     "v1.0.0",
    Description: "Token bucket rate limiting per identifier",
    SupportsRequestPhase:  true,
    SupportsResponsePhase: false,
    ParameterSchemas: []ParameterSchema{
        {
            Name:        "requestsPerSecond",
            Type:        ParameterTypeFloat,
            Description: "Maximum requests per second",
            Required:    true,
            Validation: ValidationRules{
                Minimum:    ptr(0.1),
                Maximum:    ptr(1000000.0),
                MultipleOf: ptr(0.1),
            },
        },
        {
            Name:        "burstSize",
            Type:        ParameterTypeInt,
            Description: "Maximum burst size (tokens in bucket)",
            Required:    true,
            Validation: ValidationRules{
                Minimum: ptr(1.0),
                Maximum: ptr(10000.0),
            },
        },
        {
            Name:        "identifierSource",
            Type:        ParameterTypeString,
            Description: "How to identify the client",
            Required:    true,
            Validation: ValidationRules{
                Enum: []string{"ip", "header", "jwt_claim"},
            },
        },
        {
            Name:        "identifierKey",
            Type:        ParameterTypeString,
            Description: "Header name or JWT claim name for identification",
            Required:    false,
            Validation: ValidationRules{
                MinLength: ptr(1),
                MaxLength: ptr(256),
            },
        },
        {
            Name:        "rejectStatusCode",
            Type:        ParameterTypeInt,
            Description: "HTTP status code when rate limit exceeded",
            Required:    false,
            Default:     429,
            Validation: ValidationRules{
                Minimum: ptr(400.0),
                Maximum: ptr(599.0),
            },
        },
        {
            Name:        "enableRetryAfterHeader",
            Type:        ParameterTypeBool,
            Description: "Include Retry-After header in response",
            Required:    false,
            Default:     true,
        },
    },
}
```

**Validation Flow:**

1. **Configuration Time** (when xDS config is received):
   - Kernel receives PolicySpec from xDS with name, version, and raw parameters
   - Looks up PolicyDefinition from registry using "name:version" key
   - If version not found → reject configuration with clear error
   - Validates each parameter against ParameterSchema:
     - Type checking (string, int, duration, etc.)
     - Format validation (email, URI, regex, etc.)
     - Constraint validation (min/max, pattern, enum, etc.)
     - CEL expression evaluation (if specified)
   - Converts validated parameters to TypedValue
   - Stores validated PolicySpec in RouteConfig

2. **Execution Time**:
   - No validation overhead - parameters already validated
   - Policies access pre-validated TypedValue from Parameters.Validated
   - Type-safe access with guaranteed constraints

**Design Benefits:**

- ✅ **Type Safety**: Strong typing with Go type system
- ✅ **Version Independence**: Each policy version has its own complete definition
- ✅ **Validation at Config Time**: Fail fast with clear errors, not during request processing
- ✅ **Industry Standard**: Follows OpenAPI v3 / JSON Schema / Kubernetes CRD patterns
- ✅ **Self-Documenting**: ParameterSchema serves as living documentation
- ✅ **Backward Compatibility**: Multiple versions can coexist, gradual migration
- ✅ **Extensible**: Easy to add new types and validation rules
- ✅ **CEL Support**: Custom validation expressions for complex constraints
- ✅ **Developer Experience**: Clear, actionable error messages during configuration
- ✅ **Performance**: Zero runtime validation overhead
- ✅ **Format Validation**: Built-in validators for email, URI, IP, UUID, etc.
- ✅ **Protobuf Compatible**: Can be mapped to protobuf messages for xDS

### 2.2 Worker: Core

**Responsibilities:**
- Maintain policy registry (name → Policy implementation)
- Execute policy chains in order
- Implement short-circuit logic (stop when action.Action.StopExecution() returns true)
- Collect policy results with execution metrics
- Pass execution results back to Kernel (includes actions and timing data)

**Key Functions:**
- `ExecuteRequestPolicies(policies []RequestPolicy, ctx *RequestContext) RequestExecutionResult`
  - Iterate through request policies (type-safe, no runtime type checking needed)
  - Execute each policy and collect action with timing metrics
  - Short-circuit if action.Action.StopExecution() returns true
  - Return execution result with actions and metrics to Kernel
- `ExecuteResponsePolicies(policies []ResponsePolicy, ctx *ResponseContext) ResponseExecutionResult`
  - Iterate through response policies (type-safe, no runtime type checking needed)
  - Execute each policy and collect action with timing metrics
  - Short-circuit if action.Action.StopExecution() returns true
  - Return execution result with actions and metrics to Kernel

**Policy Action Types:**

Core uses separate action types for request and response phases. The oneof pattern is enforced using Go interfaces with private marker methods, ensuring type safety and clear semantics.

```go
// ============ Request Phase ============

// RequestPolicyAction is returned by policies during request processing
type RequestPolicyAction struct {
    Action RequestAction  // Contains either UpstreamRequestModifications or ImmediateResponse
}

// RequestAction is a marker interface for the oneof pattern
// Only UpstreamRequestModifications and ImmediateResponse implement this interface
type RequestAction interface {
    isRequestAction()     // private marker method
    StopExecution() bool  // returns true if execution should stop after this action
}

// UpstreamRequestModifications contains all modifications to apply to the request
// before forwarding to upstream
type UpstreamRequestModifications struct {
    // Header modifications
    SetHeaders    map[string]string      // set or replace headers
    RemoveHeaders []string               // headers to remove
    AppendHeaders map[string][]string    // headers to append (can have multiple values)

    // Body modification
    Body          []byte                 // nil = no change, []byte{} = clear, []byte("x") = set

    // Request-specific modifications
    Path          *string                // nil = no change, pointer allows explicit empty string
    Method        *string                // nil = no change (GET, POST, PUT, DELETE, etc.)
}

func (UpstreamRequestModifications) isRequestAction() {}
func (UpstreamRequestModifications) StopExecution() bool { return false }

// ImmediateResponse short-circuits policy execution and returns response immediately
// Only valid during request phase
type ImmediateResponse struct {
    StatusCode int
    Headers    map[string]string
    Body       []byte
}

func (ImmediateResponse) isRequestAction() {}
func (ImmediateResponse) StopExecution() bool { return true }

// ============ Response Phase ============

// ResponsePolicyAction is returned by policies during response processing
type ResponsePolicyAction struct {
    Action ResponseAction  // Contains UpstreamResponseModifications
}

// ResponseAction is a marker interface for the oneof pattern
// Only UpstreamResponseModifications implements this interface
type ResponseAction interface {
    isResponseAction()    // private marker method
    StopExecution() bool  // returns true if execution should stop after this action
}

// UpstreamResponseModifications contains all modifications to apply to the response
// before returning to client
type UpstreamResponseModifications struct {
    // Header modifications
    SetHeaders    map[string]string      // set or replace headers
    RemoveHeaders []string               // headers to remove
    AppendHeaders map[string][]string    // headers to append (can have multiple values)

    // Body modification
    Body          []byte                 // nil = no change, []byte{} = clear, []byte("x") = set

    // Response-specific modifications
    StatusCode    *int                   // nil = no change, pointer allows setting any valid code
}

func (UpstreamResponseModifications) isResponseAction() {}
func (UpstreamResponseModifications) StopExecution() bool { return false }

// ============ Execution Results (with Metrics) ============

// RequestExecutionResult encapsulates all policy execution results for request phase
// Includes both actions and timing metrics for observability
type RequestExecutionResult struct {
    PolicyResults  []RequestPolicyResult  // One result per policy executed
    TotalDuration  time.Duration          // Total time for entire chain
    ShortCircuited bool                   // True if execution stopped early
}

// RequestPolicyResult contains the action and metrics for a single policy execution
type RequestPolicyResult struct {
    Action       *RequestPolicyAction  // nil if policy returned nil
    PolicyName   string                // Name of the policy
    Duration     time.Duration         // Time taken to execute this policy
    Index        int                   // Execution order (0-based)
}

// ResponseExecutionResult encapsulates all policy execution results for response phase
type ResponseExecutionResult struct {
    PolicyResults  []ResponsePolicyResult
    TotalDuration  time.Duration
    ShortCircuited bool
}

// ResponsePolicyResult contains the action and metrics for a single policy execution
type ResponsePolicyResult struct {
    Action       *ResponsePolicyAction
    PolicyName   string
    Duration     time.Duration
    Index        int
}
```

**Design Benefits:**
- **True Oneof Semantics:** Single interface field enforces exactly one action type
- **Type Safety:** Compiler prevents invalid operations (e.g., ImmediateResponse in response phase)
- **No Nil Pointer Checks:** Interface value is never nil, always contains a concrete type
- **Clean Type Switching:** Easy to handle different action types in Kernel
- **Clear Semantics:** Request can fork (upstream or immediate), response can only modify
- **Efficient:** Pointers allow nil = no change, avoiding unnecessary allocations
- **Matches Envoy Model:** Aligns with ext_proc protocol (continue or short-circuit)
- **Simple API:** Policy authors work with plain structs, no complex builders needed
- **Clear Intent:** "Action" clearly indicates what the policy wants to do
- **Observability:** Execution results include timing metrics for each policy and total chain
- **1:1 Mapping:** Each action is paired with its execution metadata
- **Type-Safe Execution:** Core receives []RequestPolicy or []ResponsePolicy, eliminating runtime type checks

**Execution Flow:**
1. Receive type-safe policy list ([]RequestPolicy or []ResponsePolicy) and context from Kernel
2. Initialize results array and start total timer
3. For each policy in order:
   - Start policy timer
   - Execute policy with context and configuration
   - Record policy name, duration, and action in result
   - Check if action.Action.StopExecution() returns true:
     - If true → short-circuit, mark ShortCircuited=true, return results
     - If false → continue to next policy
4. Return execution result to Kernel (includes all actions with timing metrics)
5. Kernel extracts actions and translates into ext_proc response format
6. Kernel can log/export metrics from execution result

**Execution Flow Diagram:**

```mermaid
flowchart TD
    Start([Receive []RequestPolicy<br/>& Context from Kernel]) --> Init["Initialize results array<br/>Start total timer"]
    Init --> LoadFirst[Load First Policy]
    LoadFirst --> StartTimer[Start policy timer]

    StartTimer --> Execute[Execute Policy:<br/>action = policy.ExecuteRequest()]

    Execute --> RecordResult[Record result:<br/>PolicyName, Duration,<br/>Action, Index]

    RecordResult --> CheckNil{Action<br/>is nil?}

    CheckNil -->|Yes| MorePolicies{More<br/>Policies?}
    CheckNil -->|No| CheckStop{action.Action.<br/>StopExecution()?}

    CheckStop -->|Yes| ShortCircuit[Short-circuit:<br/>Set ShortCircuited=true<br/>Return result]
    CheckStop -->|No| MorePolicies

    MorePolicies -->|Yes| LoadNext[Load Next Policy]
    LoadNext --> StartTimer
    MorePolicies -->|No| ReturnResult[Return RequestExecutionResult<br/>to Kernel]

    ReturnResult --> KernelExtract[Kernel extracts actions<br/>and logs metrics]
    KernelExtract --> KernelTranslate[Kernel translates actions<br/>to ext_proc response]
    KernelTranslate --> End([End])
    ShortCircuit --> KernelExtract

    style Start fill:#e1f5ff,stroke:#01579b
    style End fill:#e1f5ff,stroke:#01579b
    style ShortCircuit fill:#ffcdd2,stroke:#c62828
    style ReturnResult fill:#c8e6c9,stroke:#2e7d32
    style KernelTranslate fill:#fff9c4,stroke:#f57f17
    style KernelExtract fill:#fff9c4,stroke:#f57f17
    style CheckStop fill:#fff9c4,stroke:#f57f17
    style CheckNil fill:#e1bee7,stroke:#4a148c
```

### 2.3 Worker: Policies

**Policy Interfaces:**

Policies use marker interfaces to declare which processing phases they participate in. A policy can implement `RequestPolicy`, `ResponsePolicy`, or both.

```go
// Policy is the base interface all policies must implement
type Policy interface {
    // Name returns the unique identifier for this policy
    Name() string

    // Validate checks if the policy configuration is valid
    Validate(config map[string]interface{}) error
}

// RequestPolicy processes requests before they reach upstream
type RequestPolicy interface {
    Policy

    // ExecuteRequest runs during request processing
    // Returns nil action to skip (no modifications)
    ExecuteRequest(ctx *RequestContext, config map[string]interface{}) *RequestPolicyAction
}

// ResponsePolicy processes responses before they reach the client
type ResponsePolicy interface {
    Policy

    // ExecuteResponse runs during response processing
    // Returns nil action to skip (no modifications)
    ExecuteResponse(ctx *ResponseContext, config map[string]interface{}) *ResponsePolicyAction
}

// ============ Context Types ============

// RequestContext provides request data to policies during request phase
type RequestContext struct {
    // Request data
    Headers  map[string][]string
    Body     []byte
    Path     string
    Method   string

    // Metadata from Envoy (route key, etc.)
    Metadata map[string]string

    // Additional context
    RequestID string
}

// ResponseContext provides request and response data during response phase
type ResponseContext struct {
    // Original request data
    RequestHeaders map[string][]string
    RequestBody    []byte
    RequestPath    string
    RequestMethod  string

    // Response data
    ResponseHeaders map[string][]string
    ResponseBody    []byte
    ResponseStatus  int

    // Metadata from Envoy
    Metadata map[string]string

    // Additional context
    RequestID string
}
```

**Usage Examples:**

```go
// Example 1: Request-only policy (JWT validation)
type JWTPolicy struct{}

var _ RequestPolicy = (*JWTPolicy)(nil)  // Compile-time interface check

func (p *JWTPolicy) Name() string { return "jwtValidation" }

func (p *JWTPolicy) Validate(config map[string]interface{}) error {
    // Validate configuration
    return nil
}

func (p *JWTPolicy) ExecuteRequest(ctx *RequestContext, config map[string]interface{}) *RequestPolicyAction {
    token := extractToken(ctx.Headers["Authorization"])

    if !isValid(token) {
        return &RequestPolicyAction{
            Action: ImmediateResponse{
                StatusCode: 401,
                Headers:    map[string]string{"WWW-Authenticate": "Bearer"},
                Body:       []byte("Unauthorized"),
            },
        }
    }

    // JWT is valid, add user ID header
    return &RequestPolicyAction{
        Action: UpstreamRequestModifications{
            SetHeaders: map[string]string{
                "X-User-ID": token.Claims.Subject,
                "X-User-Email": token.Claims.Email,
            },
        },
    }
}

// Example 2: Response-only policy (add security headers)
type SecurityHeadersPolicy struct{}

var _ ResponsePolicy = (*SecurityHeadersPolicy)(nil)

func (p *SecurityHeadersPolicy) Name() string { return "securityHeaders" }

func (p *SecurityHeadersPolicy) Validate(config map[string]interface{}) error {
    return nil
}

func (p *SecurityHeadersPolicy) ExecuteResponse(ctx *ResponseContext, config map[string]interface{}) *ResponsePolicyAction {
    return &ResponsePolicyAction{
        Action: UpstreamResponseModifications{
            SetHeaders: map[string]string{
                "X-Content-Type-Options": "nosniff",
                "X-Frame-Options":        "DENY",
                "X-XSS-Protection":       "1; mode=block",
            },
        },
    }
}

// Example 3: Both phases (logging policy)
type LoggingPolicy struct{}

var _ RequestPolicy = (*LoggingPolicy)(nil)
var _ ResponsePolicy = (*LoggingPolicy)(nil)

func (p *LoggingPolicy) Name() string { return "logging" }

func (p *LoggingPolicy) Validate(config map[string]interface{}) error {
    return nil
}

func (p *LoggingPolicy) ExecuteRequest(ctx *RequestContext, config map[string]interface{}) *RequestPolicyAction {
    log.Info("Request: %s %s", ctx.Method, ctx.Path)
    return nil  // No modifications, just logging
}

func (p *LoggingPolicy) ExecuteResponse(ctx *ResponseContext, config map[string]interface{}) *ResponsePolicyAction {
    log.Info("Response: %d for %s %s", ctx.ResponseStatus, ctx.RequestMethod, ctx.RequestPath)
    return nil  // No modifications, just logging
}
```

**Core Executor Integration:**

```go
// Core executes request policies and returns execution result with metrics
func (c *Core) ExecuteRequestPolicies(policies []RequestPolicy, ctx *RequestContext) RequestExecutionResult {
    startTime := time.Now()
    results := make([]RequestPolicyResult, 0, len(policies))
    shortCircuited := false

    for i, policy := range policies {
        policyStart := time.Now()

        // Execute policy
        action := policy.ExecuteRequest(ctx, c.getConfig(policy))

        // Record result with metrics
        result := RequestPolicyResult{
            Action:     action,
            PolicyName: policy.Name(),
            Duration:   time.Since(policyStart),
            Index:      i,
        }
        results = append(results, result)

        // Check for short-circuit
        if action != nil && action.Action.StopExecution() {
            shortCircuited = true
            break
        }
    }

    return RequestExecutionResult{
        PolicyResults:  results,
        TotalDuration:  time.Since(startTime),
        ShortCircuited: shortCircuited,
    }
}

// Core executes response policies and returns execution result with metrics
func (c *Core) ExecuteResponsePolicies(policies []ResponsePolicy, ctx *ResponseContext) ResponseExecutionResult {
    startTime := time.Now()
    results := make([]ResponsePolicyResult, 0, len(policies))
    shortCircuited := false

    for i, policy := range policies {
        policyStart := time.Now()

        // Execute policy
        action := policy.ExecuteResponse(ctx, c.getConfig(policy))

        // Record result with metrics
        result := ResponsePolicyResult{
            Action:     action,
            PolicyName: policy.Name(),
            Duration:   time.Since(policyStart),
            Index:      i,
        }
        results = append(results, result)

        // Check for short-circuit
        if action != nil && action.Action.StopExecution() {
            shortCircuited = true
            break
        }
    }

    return ResponseExecutionResult{
        PolicyResults:  results,
        TotalDuration:  time.Since(startTime),
        ShortCircuited: shortCircuited,
    }
}
```

**Kernel Translation Example:**

```go
// Kernel processes execution result and translates to ext_proc response
func (k *Kernel) ProcessRequest(req *extproc.ProcessingRequest) *extproc.ProcessingResponse {
    // Extract metadata and build context
    key := k.extractMetadataKey(req)
    ctx := k.buildRequestContext(req)

    // Get type-safe request policies for this route
    requestPolicies := k.GetRequestPoliciesForKey(key)

    // Execute policies and get result with metrics
    execResult := k.core.ExecuteRequestPolicies(requestPolicies, ctx)

    // Log execution metrics
    k.logger.Info("Request policy chain executed",
        "route_key", key,
        "total_duration", execResult.TotalDuration,
        "num_policies", len(execResult.PolicyResults),
        "short_circuited", execResult.ShortCircuited,
    )

    // Log individual policy metrics
    for _, pr := range execResult.PolicyResults {
        k.logger.Debug("Policy executed",
            "name", pr.PolicyName,
            "duration", pr.Duration,
            "index", pr.Index,
            "has_action", pr.Action != nil,
        )

        // Export metrics to monitoring system
        k.metrics.RecordPolicyDuration(pr.PolicyName, pr.Duration)
    }

    // Extract actions from results
    actions := make([]RequestPolicyAction, 0, len(execResult.PolicyResults))
    for _, pr := range execResult.PolicyResults {
        if pr.Action != nil {
            actions = append(actions, *pr.Action)
        }
    }

    // Translate actions to ext_proc response
    return k.TranslateRequestActions(actions)
}

// Kernel translates policy actions to ext_proc response
func (k *Kernel) TranslateRequestActions(actions []RequestPolicyAction) *extproc.ProcessingResponse {
    // Check for immediate response (short-circuit)
    for _, action := range actions {
        switch a := action.Action.(type) {
        case ImmediateResponse:
            return &extproc.ProcessingResponse{
                Response: &extproc.ProcessingResponse_ImmediateResponse{
                    ImmediateResponse: &extproc.ImmediateResponse{
                        Status: &extproc.HttpStatus{Code: uint32(a.StatusCode)},
                        Headers: convertHeaders(a.Headers),
                        Body: string(a.Body),
                    },
                },
            }
        }
    }

    // Aggregate upstream modifications from all actions
    var allSetHeaders = make(map[string]string)
    var allRemoveHeaders []string
    var allAppendHeaders = make(map[string][]string)
    var finalBody []byte
    var finalPath *string
    var finalMethod *string

    for _, action := range actions {
        switch a := action.Action.(type) {
        case UpstreamRequestModifications:
            // Merge headers
            for k, v := range a.SetHeaders {
                allSetHeaders[k] = v
            }
            allRemoveHeaders = append(allRemoveHeaders, a.RemoveHeaders...)
            for k, v := range a.AppendHeaders {
                allAppendHeaders[k] = append(allAppendHeaders[k], v...)
            }

            // Last non-nil body wins
            if a.Body != nil {
                finalBody = a.Body
            }

            // Last non-nil path/method wins
            if a.Path != nil {
                finalPath = a.Path
            }
            if a.Method != nil {
                finalMethod = a.Method
            }
        }
    }

    // Build ext_proc response with header mutations
    return &extproc.ProcessingResponse{
        Response: &extproc.ProcessingResponse_RequestHeaders{
            RequestHeaders: &extproc.HeadersResponse{
                Response: buildHeaderMutations(allSetHeaders, allRemoveHeaders, allAppendHeaders),
            },
        },
    }
}
```

**Design Benefits:**
- **No Boilerplate:** Policies only implement phases they need
- **Self-Documenting:** Interface type declares policy capabilities
- **Type Safety:** Compiler ensures correct return types per phase
- **Flexible:** Policies can implement one or both phases
- **Extensible:** New policy types can be added without breaking existing code
- **Clean Type Switching:** Kernel easily handles different result types with type switches
- **No Runtime Type Checks:** Core receives type-safe []RequestPolicy or []ResponsePolicy slices
- **Built-in Observability:** Execution timing automatically collected for all policies

---

## 3. Initial Policy Implementations

### 3.1 SetHeader Policy

**Purpose:** Add, modify, or delete HTTP headers

**Configuration:**
```yaml
name: setHeader
parameters:
  headers:
    - name: "X-Custom-Header"
      value: "custom-value"
      action: "SET"  # SET, DELETE, APPEND
    - name: "X-Forwarded-For"
      action: "DELETE"
```

**Behavior:**
- Read header operations from configuration
- Return action with header modifications
- Never fails (non-critical)

### 3.2 JWT Validation Policy

**Purpose:** Validate JWT tokens in Authorization header

**Configuration:**
```yaml
name: jwtValidation
parameters:
  header: "Authorization"        # Header containing JWT
  prefix: "Bearer "              # Optional prefix to strip
  jwksUrl: "https://example.com/.well-known/jwks.json"
  issuer: "https://example.com"
  audience: "my-api"
  requiredClaims:
    - "sub"
    - "email"
```

**Behavior:**
- Extract JWT from specified header
- Validate signature using JWKS
- Validate issuer, audience, expiration
- Verify required claims exist
- On success: continue, optionally inject claims as headers
- On failure: return 401 Unauthorized (short-circuit)

### 3.3 API Key Validation Policy

**Purpose:** Validate API keys against configured store

**Configuration:**
```yaml
name: apiKeyValidation
parameters:
  header: "X-API-Key"           # Header containing API key
  validKeys:                     # Static list (or external lookup)
    - "key-12345"
    - "key-67890"
  errorMessage: "Invalid API Key"
```

**Behavior:**
- Extract API key from header
- Check against valid key list (or call external service)
- On success: continue, optionally set user context headers
- On failure: return 403 Forbidden (short-circuit)

### 3.4 Request Transformation Policy

**Purpose:** Transform request body and/or path

**Configuration:**
```yaml
name: requestTransformation
parameters:
  bodyTransform:
    type: "jsonPath"            # jsonPath, template, etc.
    mappings:
      - from: "$.oldField"
        to: "$.newField"
  pathRewrite:
    pattern: "^/v1/(.*)$"
    replacement: "/v2/$1"
```

**Behavior:**
- Apply JSON transformations to request body
- Rewrite request path using regex
- Return action with body and/or header modifications
- Non-critical (logs errors but continues)

---

## 4. Data Flow

### 4.1 Request Processing Flow

#### 4.1.1 Early Termination Path (Short-Circuit)

```mermaid
sequenceDiagram
    participant Client
    participant Envoy as Envoy Proxy<br/>(ext_proc Filter)
    participant Kernel
    participant Core as Worker Core
    participant Policy as API Key Policy

    Client->>Envoy: HTTP Request
    Note over Envoy: Extract metadata key<br/>from route config
    Envoy->>Kernel: ProcessingRequest (gRPC)<br/>metadata: {"route_key": "api-v1-users"}<br/>+ headers + body
    Note over Kernel: Extract key: "api-v1-users"<br/>Lookup policies:<br/>[apiKey, jwt, setHeader]
    Kernel->>Core: ExecuteRequestPolicies(policies, context)
    Note over Core: Initialize results = []<br/>Start total timer<br/>Policy 0: apiKeyValidation
    Core->>Policy: ExecuteRequest(ctx, config)
    Note over Policy: Check API key<br/>Result: FAILURE
    Policy-->>Core: RequestPolicyAction{<br/>Action: ImmediateResponse{<br/>status: 403,<br/>body: "Invalid API Key"<br/>}}
    Note over Core: Record result with metrics<br/>action.Action.StopExecution() == true<br/>Short-circuit: Stop execution
    Core-->>Kernel: RequestExecutionResult{<br/>PolicyResults[0]: {Action, Name, Duration},<br/>TotalDuration, ShortCircuited: true<br/>}
    Note over Kernel: Log metrics<br/>Extract actions<br/>Translate to ext_proc response
    Kernel-->>Envoy: ProcessingResponse<br/>(ImmediateResponse)
    Envoy->>Client: 403 Forbidden<br/>"Invalid API Key"
    Note over Envoy: Skip upstream
```

#### 4.1.2 Success Path (All Policies Pass)

```mermaid
sequenceDiagram
    participant Client
    participant Envoy as Envoy Proxy<br/>(ext_proc Filter)
    participant Kernel
    participant Core as Worker Core
    participant P1 as API Key Policy
    participant P2 as JWT Policy
    participant P3 as SetHeader Policy
    participant Upstream

    Client->>Envoy: HTTP Request
    Envoy->>Kernel: ProcessingRequest (gRPC)<br/>metadata + headers + body
    Kernel->>Core: ExecuteRequestPolicies([]RequestPolicy, context)
    Note over Core: Initialize results = []<br/>Start total timer

    Core->>P1: ExecuteRequest(ctx, config)
    Note over P1: Validate API key<br/>SUCCESS
    P1-->>Core: RequestPolicyAction{<br/>Action: UpstreamRequest{}<br/>}
    Note over Core: Record PolicyResult[0]<br/>with name, duration, action<br/>StopExecution() == false

    Core->>P2: ExecuteRequest(ctx, config)
    Note over P2: Validate JWT<br/>Extract claims<br/>SUCCESS
    P2-->>Core: RequestPolicyAction{<br/>Action: UpstreamRequest{<br/>SetHeaders: {"X-User-ID": "12345"}<br/>}}
    Note over Core: Record PolicyResult[1]<br/>with metrics<br/>StopExecution() == false

    Core->>P3: ExecuteRequest(ctx, config)
    Note over P3: Add custom headers<br/>SUCCESS
    P3-->>Core: RequestPolicyAction{<br/>Action: UpstreamRequest{<br/>SetHeaders: {"X-Custom": "value"}<br/>}}
    Note over Core: Record PolicyResult[2]<br/>with metrics<br/>StopExecution() == false

    Note over Core: All policies complete<br/>Calculate total duration
    Core-->>Kernel: RequestExecutionResult{<br/>PolicyResults: [3 results with metrics],<br/>TotalDuration, ShortCircuited: false<br/>}
    Note over Kernel: Log metrics<br/>Extract actions<br/>Translate to ext_proc
    Kernel-->>Envoy: ProcessingResponse<br/>(HeaderMutations)
    Note over Envoy: Apply header mutations
    Envoy->>Upstream: Forward request with<br/>modified headers
    Upstream-->>Envoy: Response
    Envoy-->>Client: Response
```

---

## 5. API Specifications

### 5.1 Policy Discovery Service (xDS-based)

**Protocol:** xDS State of the World (SotW) over gRPC
**Port:** 9002
**Transport:** Bidirectional gRPC streaming

The Policy Discovery Service (PDS) follows Envoy's xDS protocol patterns to enable dynamic policy configuration updates. Using the State of the World approach, clients subscribe to policy mappings and receive complete configuration snapshots on every update.

**xDS Flow Overview:**

```mermaid
sequenceDiagram
    participant Client as Control Plane<br/>Client
    participant PDS as Policy Discovery<br/>Service (Port 9002)
    participant Kernel
    participant Storage as Policy Mapping<br/>Storage (In-Memory)
    participant Registry as Policy Registry

    Note over Client,Registry: xDS Stream Initialization

    Client->>PDS: StreamPolicyMappings()<br/>(open bidirectional stream)

    Note over Client,Registry: Initial Request (State of the World)

    Client->>PDS: DiscoveryRequest<br/>{resource_names: ["*"],<br/>version_info: "",<br/>response_nonce: ""}

    PDS->>Kernel: GetAllPolicyMappings()
    Kernel->>Storage: Retrieve all mappings
    Storage-->>Kernel: All policy configurations

    Kernel->>Registry: Validate all policies
    Registry-->>Kernel: Validation results

    Kernel-->>PDS: PolicyMappings<br/>(version: "v1")
    PDS-->>Client: DiscoveryResponse<br/>{version_info: "v1",<br/>resources: [all mappings],<br/>nonce: "nonce-1"}

    Note over Client: Client applies config

    Client->>PDS: DiscoveryRequest (ACK)<br/>{version_info: "v1",<br/>response_nonce: "nonce-1"}

    Note over Client,Registry: Configuration Update

    Note over PDS: Admin updates policy config
    PDS->>Kernel: UpdatePolicyMapping()
    Kernel->>Storage: Store new mapping<br/>(increment version to v2)

    Kernel-->>PDS: Updated mappings<br/>(version: "v2")
    PDS-->>Client: DiscoveryResponse<br/>{version_info: "v2",<br/>resources: [all mappings],<br/>nonce: "nonce-2"}

    Client->>PDS: DiscoveryRequest (ACK)<br/>{version_info: "v2",<br/>response_nonce: "nonce-2"}

    Note over Client,Registry: Policy Execution (Runtime)

    Note over Kernel: Envoy sends ext_proc request
    Kernel->>Storage: GetPoliciesForKey(routeKey)
    Storage-->>Kernel: Policy configuration list
    Note over Kernel: Execute policies...
```

#### 5.1.1 Protocol Buffers Definition

```protobuf
syntax = "proto3";

package policyengine.config.v1;

import "google/protobuf/any.proto";
import "google/protobuf/struct.proto";

// Policy Discovery Service (xDS-style)
service PolicyDiscoveryService {
  // Stream policy mappings using State of the World (SotW) protocol
  rpc StreamPolicyMappings(stream DiscoveryRequest) returns (stream DiscoveryResponse);
}

// Discovery request following xDS pattern
message DiscoveryRequest {
  // Version info from the previous DiscoveryResponse (ACK/NACK)
  // Empty for initial request
  string version_info = 1;

  // The node making the request (optional, for multi-tenant scenarios)
  string node_id = 2;

  // List of resource names to subscribe to
  // Use ["*"] for wildcard subscription (all policy mappings)
  repeated string resource_names = 3;

  // Type URL for the requested resource type
  // e.g., "type.googleapis.com/policyengine.config.v1.PolicyMapping"
  string type_url = 4;

  // Nonce from the previous DiscoveryResponse
  // Used to correlate request-response pairs
  string response_nonce = 5;

  // Error details if NACK (rejecting the configuration)
  google.rpc.Status error_detail = 6;
}

// Discovery response following xDS pattern
message DiscoveryResponse {
  // Version of the configuration snapshot
  string version_info = 1;

  // The policy mapping resources
  repeated google.protobuf.Any resources = 2;

  // Type URL for the resources
  // "type.googleapis.com/policyengine.config.v1.PolicyMapping"
  string type_url = 3;

  // Nonce for this response
  // Client must echo this in the next DiscoveryRequest
  string nonce = 4;
}

// PolicyMapping resource
message PolicyMapping {
  // Route key (metadata key from Envoy)
  string route_key = 1;

  // Policies to execute during request flow
  repeated PolicyConfig request_policies = 2;

  // Policies to execute during response flow
  repeated PolicyConfig response_policies = 3;

  // Resource metadata
  ResourceMetadata metadata = 4;
}

// Individual policy configuration
message PolicyConfig {
  // Policy name (must match registered policy)
  string name = 1;

  // Whether this policy is enabled
  bool enabled = 2;

  // Policy-specific configuration as JSON
  google.protobuf.Struct config = 3;
}

// Resource metadata for tracking
message ResourceMetadata {
  // When this resource was created
  int64 created_at = 1;

  // When this resource was last updated
  int64 updated_at = 2;

  // Resource version for optimistic locking
  int64 resource_version = 3;
}
```

#### 5.1.2 xDS Protocol Behavior

**State of the World (SotW):**
- Client subscribes to resources by sending a `DiscoveryRequest`
- Server responds with complete state of all subscribed resources
- Every update sends the full resource set (not deltas)
- Client ACKs by echoing `version_info` and `response_nonce`
- Client NACKs by sending previous `version_info` with `error_detail`

**Resource Subscription:**
- Wildcard: `resource_names: ["*"]` - Subscribe to all policy mappings
- Specific: `resource_names: ["api-v1-users", "api-v2-posts"]` - Subscribe to specific routes

**Version Tracking:**
- Each configuration snapshot has a monotonically increasing version
- Versions can be timestamps, UUIDs, or semantic versions (e.g., "v1", "v2")
- Client tracks accepted version for recovery after disconnect

**Nonce Handling:**
- Server generates unique nonce for each `DiscoveryResponse`
- Client echoes nonce in subsequent `DiscoveryRequest`
- Prevents race conditions in ACK/NACK processing

#### 5.1.3 Example: Policy Mapping Resource

```json
{
  "route_key": "api-v1-users",
  "request_policies": [
    {
      "name": "apiKeyValidation",
      "enabled": true,
      "config": {
        "header": "X-API-Key",
        "validKeys": ["key-123", "key-456"]
      }
    },
    {
      "name": "jwtValidation",
      "enabled": true,
      "config": {
        "header": "Authorization",
        "prefix": "Bearer ",
        "jwksUrl": "https://example.com/.well-known/jwks.json",
        "issuer": "https://example.com",
        "audience": "my-api"
      }
    },
    {
      "name": "setHeader",
      "enabled": true,
      "config": {
        "headers": [
          {
            "name": "X-Custom-Header",
            "value": "custom-value",
            "action": "SET"
          }
        ]
      }
    }
  ],
  "response_policies": [
    {
      "name": "securityHeaders",
      "enabled": true,
      "config": {
        "headers": [
          {
            "name": "X-Content-Type-Options",
            "value": "nosniff",
            "action": "SET"
          },
          {
            "name": "X-Frame-Options",
            "value": "DENY",
            "action": "SET"
          }
        ]
      }
    }
  ],
  "metadata": {
    "created_at": 1700000000,
    "updated_at": 1700000100,
    "resource_version": 5
  }
}
```

#### 5.1.4 Configuration Management

**Updating Policy Mappings:**

The control plane client (admin tool, CI/CD, or management UI) connects to the PDS and:
1. Opens a bidirectional gRPC stream
2. Sends initial `DiscoveryRequest` to receive current state
3. Receives `DiscoveryResponse` with all policy mappings
4. Makes changes to policy configurations (via separate admin API or config files)
5. Receives updated `DiscoveryResponse` automatically when changes occur
6. ACKs the new configuration

**Management Interface:**

A separate admin gRPC service can be provided for imperative updates:

```protobuf
service PolicyAdminService {
  rpc CreatePolicyMapping(CreatePolicyMappingRequest) returns (PolicyMapping);
  rpc UpdatePolicyMapping(UpdatePolicyMappingRequest) returns (PolicyMapping);
  rpc DeletePolicyMapping(DeletePolicyMappingRequest) returns (google.protobuf.Empty);
  rpc GetPolicyMapping(GetPolicyMappingRequest) returns (PolicyMapping);
  rpc ListPolicyMappings(ListPolicyMappingsRequest) returns (ListPolicyMappingsResponse);
}
```

This admin service triggers xDS updates to all connected clients via the PDS stream.

---

## 6. Technical Requirements

### 6.1 Go Interfaces

**Main Package Structure:**
```
policy-engine/
├── main.go                    # Entry point
├── kernel/
│   ├── extproc.go            # gRPC ext_proc server
│   ├── xds.go                # xDS Policy Discovery Service server
│   ├── mapper.go             # Route-to-policy mapping storage
│   ├── translator.go         # Action translator
│   └── admin.go              # Optional admin gRPC service
├── worker/
│   ├── core/
│   │   ├── executor.go       # Policy chain executor
│   │   ├── registry.go       # Policy registry
│   │   └── action.go         # Action types
│   └── policies/
│       ├── interface.go      # Policy interface
│       ├── setheader.go      # SetHeader policy
│       ├── jwt.go            # JWT validation policy
│       ├── apikey.go         # API Key validation policy
│       └── transform.go      # Request transformation policy
├── proto/
│   ├── envoy/                # Envoy ext_proc proto (from go-control-plane)
│   └── policyengine/         # Policy Discovery Service protos
│       └── config/
│           └── v1/
│               ├── pds.proto           # PolicyDiscoveryService definition
│               ├── policy_mapping.proto # PolicyMapping resource
│               └── admin.proto         # Optional admin service
├── pkg/
│   └── xds/
│       ├── cache.go          # xDS snapshot cache
│       └── server.go         # xDS server helpers
├── config/
│   └── config.go             # Configuration structures
└── go.mod
```

### 6.2 Dependencies

- **Envoy go-control-plane:** `github.com/envoyproxy/go-control-plane` - For ext_proc proto definitions and xDS types
- **gRPC:** `google.golang.org/grpc` - For both ext_proc and xDS gRPC services
- **Protocol Buffers:** `google.golang.org/protobuf` - For proto message handling
- **JWT:** `github.com/golang-jwt/jwt/v5` - For JWT validation policy
- **Testing:** Standard library `testing` + `github.com/stretchr/testify`

### 6.3 Configuration

**Policy Engine Configuration File (config.yaml):**
```yaml
kernel:
  grpcPort: 9001              # ext_proc server port
  xdsPort: 9002               # xDS Policy Discovery Service port

worker:
  maxPoliciesPerChain: 20
  policyTimeout: 5s

policies:
  pluginDir: "/opt/policy-engine/plugins"

logging:
  level: "info"
  format: "json"
```

---

## 7. Non-Functional Requirements

### 7.1 Performance

- **Latency Target:** < 10ms p95 for policy evaluation (excluding external calls)
- **Throughput:** Handle 10,000+ requests/second per instance
- **Memory:** < 500MB memory footprint under normal load

### 7.2 Reliability

- **Availability:** 99.9% uptime
- **Error Handling:** Graceful degradation on policy failures
- **Failure Mode:** Configurable (fail-open vs fail-closed)

### 7.3 Security

- **TLS:** Support mutual TLS for gRPC connection with Envoy
- **API Authentication:** API key or JWT authentication for Configuration API
- **Secret Management:** Integration with secret stores (env vars, HashiCorp Vault)

### 7.4 Scalability

- **Horizontal Scaling:** Stateless design for horizontal scaling
- **Resource Limits:** CPU and memory limits configurable
- **Connection Pooling:** Efficient connection management for external services

---

## 8. Future Considerations

### 8.1 Phase 2 Policies

- Rate Limiting (token bucket, leaky bucket)
- OAuth2 introspection
- RBAC (Role-Based Access Control)
- Request/Response logging
- WAF (Web Application Firewall) rules
- GeoIP filtering

### 8.2 Enhanced Features

- **Policy Composition:** Support policy groups and sub-chains
- **Conditional Execution:** Execute policies based on conditions (if-then-else)
- **Policy Versioning:** Support multiple versions of same policy
- **A/B Testing:** Route percentage of traffic to different policy chains
- **Caching:** Cache policy results for idempotent operations

### 8.3 Integration & Extensibility

- **Plugin System:** Dynamic loading of policies via Go plugins or WASM
- **External Policy Servers:** Delegate policy evaluation to external services (OPA, etc.)
- **Policy Language:** DSL for defining policies without code

### 8.4 Operations

- **Hot Reload:** Reload policy configurations without restart
- **Admin UI:** Web dashboard for policy management
- **Policy Testing:** Test framework for validating policies
- **Policy Simulation:** Dry-run mode to test policies without enforcement

---

## 9. Success Criteria

### 9.1 Functional Success

- ✅ Kernel successfully handles ext_proc requests from Envoy
- ✅ Dynamic policy configuration via API works without restarts
- ✅ All four initial policies implemented and tested
- ✅ Short-circuit behavior works correctly when action.StopExecution() returns true
- ✅ Metadata-based routing maps to correct policy chains

### 9.2 Technical Success

- ✅ < 10ms p95 latency for policy evaluation
- ✅ 100% unit test coverage for Core and Policies
- ✅ Integration tests with Envoy v1.36.2
- ✅ Zero memory leaks under load testing
- ✅ Documentation complete (API docs, architecture, deployment guide)

### 9.3 Operational Success

- ✅ Deployed successfully with Docker Compose
- ✅ Successfully processes production-like traffic
- ✅ Policy configuration can be updated via API

---

## 10. Timeline & Milestones

```mermaid
gantt
    title Policy Engine Implementation Timeline
    dateFormat YYYY-MM-DD
    section Phase 1: Core Infrastructure
    Kernel gRPC Server           :p1a, 2025-11-17, 7d
    Worker Core & Registry       :p1b, 2025-11-17, 7d
    Policy Interface             :p1c, after p1b, 3d
    Action Translation           :p1d, after p1b, 4d

    section Phase 2: Initial Policies
    SetHeader Policy             :p2a, after p1d, 2d
    API Key Validation           :p2b, after p2a, 2d
    JWT Validation               :p2c, after p2b, 2d
    Request Transformation       :p2d, after p2c, 2d

    section Phase 3: Configuration API
    REST API Implementation      :p3a, after p2d, 4d
    In-Memory Storage            :p3b, after p2d, 3d
    API Documentation            :p3c, after p3a, 2d

    section Phase 4: Testing & Polish
    Unit Tests                   :p4a, after p3c, 3d
    Integration Tests with Envoy :p4b, after p4a, 2d
    Load Testing                 :p4c, after p4b, 2d
    Error Handling Polish        :p4d, after p4c, 2d
    Deployment Guide             :p4e, after p4d, 2d
```

### Phase 1: Core Infrastructure (Week 1-2)
- Implement Kernel with ext_proc gRPC server
- Implement Worker Core with policy registry and executor
- Define Policy interface
- Basic action translation

### Phase 2: Initial Policies (Week 3)
- SetHeader policy
- API Key validation policy
- JWT validation policy
- Request transformation policy

### Phase 3: Configuration API (Week 4)
- REST API for policy mappings
- In-memory storage
- API documentation

### Phase 4: Testing & Polish (Week 5)
- Unit tests for all components
- Integration tests with Envoy
- Load testing
- Error handling improvements
- Deployment guide

---

## 11. Appendix

### 11.1 Envoy ext_proc Integration

**Envoy Configuration (envoy.yaml):**
```yaml
http_filters:
  - name: envoy.filters.http.ext_proc
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.filters.http.ext_proc.v3.ExternalProcessor
      grpc_service:
        envoy_grpc:
          cluster_name: policy_engine
        timeout: 5s
      processing_mode:
        request_header_mode: SEND
        response_header_mode: SEND
        request_body_mode: BUFFERED
        response_body_mode: BUFFERED
      metadata_options:
        forwarding_namespaces:
          - envoy.filters.http.ext_proc

clusters:
  - name: policy_engine
    connect_timeout: 1s
    type: STRICT_DNS
    lb_policy: ROUND_ROBIN
    http2_protocol_options: {}
    load_assignment:
      cluster_name: policy_engine
      endpoints:
        - lb_endpoints:
            - endpoint:
                address:
                  socket_address:
                    address: 127.0.0.1
                    port_value: 9001
```

**Route Configuration with Metadata:**
```yaml
routes:
  - match:
      prefix: "/api/v1/users"
    route:
      cluster: backend
    metadata:
      filter_metadata:
        envoy.filters.http.ext_proc:
          route_key: "api-v1-users"
```

### 11.2 Glossary

- **ext_proc:** Envoy External Processor filter
- **Kernel:** The gRPC server component that interfaces with Envoy
- **Worker Core:** The policy execution engine
- **Policy:** A discrete unit of request/response processing logic
- **Action:** The result returned by a policy indicating how to modify the request/response or whether to short-circuit execution
- **Short-circuit:** Stopping policy chain execution when a policy returns an action with StopExecution() = true
- **Route Key:** Metadata key used to identify which policy chain to execute

---

**End of Document**
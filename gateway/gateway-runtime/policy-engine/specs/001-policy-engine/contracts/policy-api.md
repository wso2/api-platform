# Policy Interface Contracts

**Feature**: 001-policy-engine | **Date**: 2025-11-18

## Overview

This document defines the Go interface contracts that all policies must implement. These interfaces are the extension points for custom policy development.

## Module Architecture

The Policy Engine uses a **multi-module architecture** to avoid cyclic dependencies:

```
┌─────────────────────────────────────┐
│  SDK Module                         │
│  github.com/policy-engine/sdk │
│  ┌───────────────────────────────┐  │
│  │ policies/                     │  │  ← Policy interfaces & types
│  │ - Policy, RequestPolicy, etc. │  │
│  │ - RequestContext, etc.        │  │
│  │ - RequestPolicyAction, etc.   │  │
│  └───────────────────────────────┘  │
│  ┌───────────────────────────────┐  │
│  │ core/                         │  │  ← Registry & executor
│  │ - PolicyRegistry              │  │
│  │ - PolicyChain                 │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
           ↑                    ↑
           │                    │
    ┌──────┴─────┐       ┌─────┴──────────┐
    │  Policy    │       │  Main Engine   │
    │  Modules   │       │  (Worker)      │
    └────────────┘       └────────────────┘
```

**Benefits**:
- **No cyclic dependencies**: Policies depend only on SDK, not the main engine
- **Clean separation**: Framework code (SDK) separate from implementation (policies, engine)
- **Versioned SDK**: Policies can specify SDK version requirements
- **External policies**: Policies can be developed in separate repositories

**Import Rules**:
- ✅ Policy implementations: Import `github.com/policy-engine/sdk/policy`
- ✅ Policy implementations: Import `github.com/policy-engine/sdk/core` (if needed for registry)
- ❌ Policy implementations: Never import `github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine`

---

## Core Policy Interfaces

### 1. Policy (Base Interface)

All policies must implement this base interface.

```go
package policies

// Policy is the base interface all policies must implement
type Policy interface {
    // Name returns the unique identifier for this policy
    // MUST match the name in policy.yaml
    // Convention: camelCase (e.g., "jwtValidation", "rateLimiting")
    Name() string

    // Validate checks if the policy configuration is valid
    // Called at configuration time (not request time)
    // Returns error if configuration is invalid with descriptive message
    // config: Raw configuration map from PolicySpec.Parameters.Raw
    Validate(config map[string]interface{}) error
}
```

**Contract Requirements**:
- `Name()` MUST return a non-empty string matching policy.yaml name field
- `Name()` MUST return the same value for all invocations (immutable)
- `Validate()` MUST NOT modify config parameter (read-only)
- `Validate()` MUST return nil if config is valid
- `Validate()` MUST return descriptive error if config is invalid
- `Validate()` is called once at configuration time, not per-request

**Example Implementation**:
```go
type JWTPolicy struct{}

func (p *JWTPolicy) Name() string {
    return "jwtValidation"
}

func (p *JWTPolicy) Validate(config map[string]interface{}) error {
    if _, ok := config["jwksUrl"]; !ok {
        return fmt.Errorf("missing required parameter: jwksUrl")
    }
    return nil
}
```

---

### 2. RequestPolicy (Request Phase Processing)

Policies that process requests during the request phase implement this interface.

```go
package policies

// RequestPolicy processes requests before they reach upstream
// Policies implementing this interface are included in PolicyChain.RequestPolicies
type RequestPolicy interface {
    Policy  // Embeds base Policy interface

    // ExecuteRequest runs during request processing
    // ctx: Mutable request context (policies can read and write)
    // config: Validated configuration from PolicySpec.Parameters.Validated
    //
    // Return nil to skip (no modifications)
    // Return *RequestPolicyAction to apply modifications or short-circuit
    //
    // MUST NOT panic - return error action instead
    // MUST complete within reasonable time (< 1 second)
    // MUST be thread-safe (called concurrently for different requests)
    ExecuteRequest(ctx *RequestContext, config map[string]interface{}) *RequestPolicyAction
}
```

**Contract Requirements**:
- MUST implement both `Policy` and `ExecuteRequest` methods
- `ExecuteRequest()` receives mutable RequestContext - can read and modify
- `ExecuteRequest()` receives validated config - parameters already validated
- MUST NOT panic - handle all errors gracefully
- MUST be thread-safe - called concurrently from multiple goroutines
- SHOULD complete quickly (< 100ms typical, < 1s maximum)
- Return nil for no modifications (pass-through)
- Return action to modify request or short-circuit

**Return Value Semantics**:
```go
// No modifications - pass request through unchanged
return nil

// Modify request headers before upstream
return &RequestPolicyAction{
    Action: UpstreamRequestModifications{
        SetHeaders: map[string]string{"X-User-ID": "123"},
    },
}

// Short-circuit - return immediate response, skip upstream
return &RequestPolicyAction{
    Action: ImmediateResponse{
        StatusCode: 401,
        Headers: map[string]string{"WWW-Authenticate": "Bearer"},
        Body: []byte("Unauthorized"),
    },
}
```

**Example Implementation**:
```go
type JWTPolicy struct {
    jwksCache *JWKSCache
}

func (p *JWTPolicy) ExecuteRequest(ctx *RequestContext, config map[string]interface{}) *RequestPolicyAction {
    // Extract JWT from Authorization header
    authHeader := ctx.Headers["authorization"]
    if len(authHeader) == 0 {
        return &RequestPolicyAction{
            Action: ImmediateResponse{
                StatusCode: 401,
                Headers: map[string]string{"WWW-Authenticate": "Bearer"},
                Body: []byte("Missing authorization header"),
            },
        }
    }

    token := strings.TrimPrefix(authHeader[0], "Bearer ")

    // Validate JWT
    claims, err := p.validateJWT(token, config["jwksUrl"].(string))
    if err != nil {
        return &RequestPolicyAction{
            Action: ImmediateResponse{
                StatusCode: 401,
                Body: []byte("Invalid JWT: " + err.Error()),
            },
        }
    }

    // Store user info in metadata for downstream policies
    ctx.Metadata["user_id"] = claims.Subject
    ctx.Metadata["user_email"] = claims.Email
    ctx.Metadata["authenticated"] = true

    // Add user ID header for upstream
    return &RequestPolicyAction{
        Action: UpstreamRequestModifications{
            SetHeaders: map[string]string{
                "X-User-ID": claims.Subject,
                "X-User-Email": claims.Email,
            },
        },
    }
}
```

---

### 3. ResponsePolicy (Response Phase Processing)

Policies that process responses during the response phase implement this interface.

```go
package policies

// ResponsePolicy processes responses before they reach the client
// Policies implementing this interface are included in PolicyChain.ResponsePolicies
type ResponsePolicy interface {
    Policy  // Embeds base Policy interface

    // ExecuteResponse runs during response processing
    // ctx: Response context with request data (immutable) and response data (mutable)
    // config: Validated configuration from PolicySpec.Parameters.Validated
    //
    // Return nil to skip (no modifications)
    // Return *ResponsePolicyAction to apply modifications
    //
    // MUST NOT panic - return error action instead
    // MUST complete within reasonable time (< 1 second)
    // MUST be thread-safe (called concurrently for different requests)
    ExecuteResponse(ctx *ResponseContext, config map[string]interface{}) *ResponsePolicyAction
}
```

**Contract Requirements**:
- MUST implement both `Policy` and `ExecuteResponse` methods
- `ExecuteResponse()` receives ResponseContext with:
  - Immutable request data (RequestHeaders, RequestBody, etc.)
  - Mutable response data (ResponseHeaders, ResponseBody, ResponseStatus)
  - Shared metadata from request phase
- Can read metadata set by request phase policies
- MUST NOT panic - handle all errors gracefully
- MUST be thread-safe
- SHOULD complete quickly
- Return nil for no modifications
- Return action to modify response

**Return Value Semantics**:
```go
// No modifications - pass response through unchanged
return nil

// Modify response headers
return &ResponsePolicyAction{
    Action: UpstreamResponseModifications{
        SetHeaders: map[string]string{
            "X-Content-Type-Options": "nosniff",
            "X-Frame-Options": "DENY",
        },
    },
}

// Modify response status and body
return &ResponsePolicyAction{
    Action: UpstreamResponseModifications{
        StatusCode: ptr(200),
        Body: transformedBody,
    },
}
```

**Example Implementation**:
```go
type SecurityHeadersPolicy struct{}

func (p *SecurityHeadersPolicy) ExecuteResponse(ctx *ResponseContext, config map[string]interface{}) *ResponsePolicyAction {
    headers := map[string]string{
        "X-Content-Type-Options": "nosniff",
        "X-Frame-Options":        "DENY",
        "X-XSS-Protection":       "1; mode=block",
    }

    // Add user context if authenticated (from request phase metadata)
    if authenticated, ok := ctx.Metadata["authenticated"].(bool); ok && authenticated {
        if userID, ok := ctx.Metadata["user_id"].(string); ok {
            headers["X-Authenticated-User"] = userID
        }
    }

    return &ResponsePolicyAction{
        Action: UpstreamResponseModifications{
            SetHeaders: headers,
        },
    }
}
```

---

### 4. Dual-Phase Policy (Both Request and Response)

Policies can implement both RequestPolicy and ResponsePolicy to participate in both phases.

```go
type LoggingPolicy struct {
    logger *Logger
}

// Implements Policy interface
func (p *LoggingPolicy) Name() string {
    return "logging"
}

func (p *LoggingPolicy) Validate(config map[string]interface{}) error {
    return nil
}

// Implements RequestPolicy interface
func (p *LoggingPolicy) ExecuteRequest(ctx *RequestContext, config map[string]interface{}) *RequestPolicyAction {
    // Store request timestamp in metadata
    ctx.Metadata["request_start_time"] = time.Now()

    p.logger.Info("Request",
        "method", ctx.Method,
        "path", ctx.Path,
        "request_id", ctx.RequestID)

    return nil  // No modifications
}

// Implements ResponsePolicy interface
func (p *LoggingPolicy) ExecuteResponse(ctx *ResponseContext, config map[string]interface{}) *ResponsePolicyAction {
    // Calculate duration using metadata from request phase
    var duration time.Duration
    if startTime, ok := ctx.Metadata["request_start_time"].(time.Time); ok {
        duration = time.Since(startTime)
    }

    p.logger.Info("Response",
        "method", ctx.RequestMethod,
        "path", ctx.RequestPath,
        "status", ctx.ResponseStatus,
        "duration", duration,
        "request_id", ctx.RequestID)

    return nil  // No modifications
}
```

---

## Context Types

### RequestContext

```go
// RequestContext holds the current request state during request phase
// This context is MUTABLE - it gets updated as each policy executes
type RequestContext struct {
    // Current request headers (mutable)
    // Key: header name (lowercase), Value: header values (array)
    Headers map[string][]string

    // Current request body (mutable)
    // nil if no body or body not required by policies
    Body []byte

    // Current request path (mutable)
    Path string

    // Current request method (mutable)
    Method string

    // Unique request identifier for correlation
    // Immutable - generated by Kernel
    RequestID string

    // Shared metadata for inter-policy communication
    // Policies can read/write this map to coordinate behavior
    // This same map persists to response phase
    Metadata map[string]interface{}
}
```

**Usage Contract**:
- Policies can read any field
- Policies can modify Headers, Body, Path, Method
- Policies MUST NOT modify RequestID
- Policies can read/write Metadata for coordination
- Later policies see modifications from earlier policies

---

### ResponseContext

```go
// ResponseContext holds request and response state during response phase
type ResponseContext struct {
    // Original request data (IMMUTABLE, from request phase)
    RequestHeaders map[string][]string
    RequestBody    []byte
    RequestPath    string
    RequestMethod  string

    // Current response headers (MUTABLE)
    ResponseHeaders map[string][]string

    // Current response body (MUTABLE)
    // nil if no body or body not required by policies
    ResponseBody []byte

    // Current response status code (MUTABLE)
    ResponseStatus int

    // Request identifier (IMMUTABLE, from request phase)
    RequestID string

    // Shared metadata from request phase (same reference)
    // Policies can read metadata set during request phase
    Metadata map[string]interface{}
}
```

**Usage Contract**:
- Policies can read all Request* fields (immutable)
- Policies can read/modify Response* fields
- Policies MUST NOT modify Request* fields or RequestID
- Policies can read Metadata set by request phase policies
- Policies can write Metadata (visible to later response policies)

---

## Action Types

### RequestPolicyAction

```go
// RequestPolicyAction is returned by policies during request processing
type RequestPolicyAction struct {
    Action RequestAction  // Contains either UpstreamRequestModifications or ImmediateResponse
}

// RequestAction is marker interface for oneof pattern
type RequestAction interface {
    isRequestAction()     // private marker method
    StopExecution() bool  // returns true if execution should stop
}
```

---

### UpstreamRequestModifications

```go
// UpstreamRequestModifications contains modifications to apply before upstream
type UpstreamRequestModifications struct {
    // SetHeaders: Set or replace headers
    // If header exists, replaces all values
    // If header doesn't exist, adds new header
    SetHeaders map[string]string

    // RemoveHeaders: Headers to remove
    // If header doesn't exist, no-op
    RemoveHeaders []string

    // AppendHeaders: Headers to append
    // If header exists, appends to existing values
    // If header doesn't exist, creates new header with these values
    AppendHeaders map[string][]string

    // Body: Replace request body
    // nil = no change (keep existing body)
    // []byte{} = clear body (empty body)
    // []byte("data") = set new body content
    Body []byte

    // Path: Replace request path
    // nil = no change
    // pointer allows explicit empty string
    Path *string

    // Method: Replace request method
    // nil = no change
    Method *string
}

func (UpstreamRequestModifications) isRequestAction() {}
func (UpstreamRequestModifications) StopExecution() bool { return false }
```

**Usage Examples**:
```go
// Set single header
Action: UpstreamRequestModifications{
    SetHeaders: map[string]string{"X-User-ID": "123"},
}

// Remove headers
Action: UpstreamRequestModifications{
    RemoveHeaders: []string{"authorization", "cookie"},
}

// Append to existing header
Action: UpstreamRequestModifications{
    AppendHeaders: map[string][]string{
        "X-Forwarded-For": []string{"10.0.0.1"},
    },
}

// Modify body
Action: UpstreamRequestModifications{
    Body: []byte(`{"transformed": true}`),
    SetHeaders: map[string]string{"Content-Type": "application/json"},
}

// Change path (routing)
newPath := "/v2/users"
Action: UpstreamRequestModifications{
    Path: &newPath,
}
```

---

### ImmediateResponse

```go
// ImmediateResponse short-circuits policy execution and returns response immediately
type ImmediateResponse struct {
    // HTTP status code (200, 401, 403, 500, etc.)
    StatusCode int

    // Response headers
    // Will override any existing headers
    Headers map[string]string

    // Response body
    Body []byte
}

func (ImmediateResponse) isRequestAction() {}
func (ImmediateResponse) StopExecution() bool { return true }
```

**Usage Examples**:
```go
// 401 Unauthorized
Action: ImmediateResponse{
    StatusCode: 401,
    Headers: map[string]string{
        "WWW-Authenticate": "Bearer realm=\"API\"",
        "Content-Type": "application/json",
    },
    Body: []byte(`{"error": "Unauthorized", "message": "Invalid or missing token"}`),
}

// 403 Forbidden
Action: ImmediateResponse{
    StatusCode: 403,
    Body: []byte("Forbidden: Insufficient permissions"),
}

// 429 Rate Limit Exceeded
Action: ImmediateResponse{
    StatusCode: 429,
    Headers: map[string]string{
        "Retry-After": "60",
        "X-RateLimit-Remaining": "0",
    },
    Body: []byte("Rate limit exceeded. Try again in 60 seconds."),
}
```

---

### ResponsePolicyAction

```go
// ResponsePolicyAction is returned by policies during response processing
type ResponsePolicyAction struct {
    Action ResponseAction  // Contains UpstreamResponseModifications
}

// ResponseAction is marker interface for oneof pattern
type ResponseAction interface {
    isResponseAction()    // private marker method
    StopExecution() bool  // returns true if execution should stop
}
```

---

### UpstreamResponseModifications

```go
// UpstreamResponseModifications contains modifications to apply before client delivery
type UpstreamResponseModifications struct {
    // SetHeaders: Set or replace response headers
    SetHeaders map[string]string

    // RemoveHeaders: Response headers to remove
    RemoveHeaders []string

    // AppendHeaders: Response headers to append
    AppendHeaders map[string][]string

    // Body: Replace response body
    // nil = no change
    // []byte{} = clear body
    // []byte("data") = set new body content
    Body []byte

    // StatusCode: Replace response status code
    // nil = no change
    // pointer allows setting any valid HTTP status
    StatusCode *int
}

func (UpstreamResponseModifications) isResponseAction() {}
func (UpstreamResponseModifications) StopExecution() bool { return false }
```

**Usage Examples**:
```go
// Add security headers
Action: UpstreamResponseModifications{
    SetHeaders: map[string]string{
        "X-Content-Type-Options": "nosniff",
        "X-Frame-Options": "DENY",
        "Strict-Transport-Security": "max-age=31536000",
    },
}

// Modify response status
newStatus := 200
Action: UpstreamResponseModifications{
    StatusCode: &newStatus,
}

// Transform response body
Action: UpstreamResponseModifications{
    Body: transformedResponseBody,
    SetHeaders: map[string]string{
        "Content-Type": "application/json",
        "Content-Length": strconv.Itoa(len(transformedResponseBody)),
    },
}
```

---

## Policy Factory Function

Every policy module MUST export a factory function for registration:

```go
// GetPolicy creates a new instance of the policy
// Called by generated plugin_registry.go during initialization
// MUST return a Policy interface implementation
func GetPolicy(metadata PolicyMetadata, params map[string]interface{}) (Policy, error) {
    return &JWTPolicy{
        jwksCache: NewJWKSCache(),
    }, nil
}
```

**Contract Requirements**:
- Function name MUST be `GetPolicy`
- Signature MUST be `func(PolicyMetadata, map[string]interface{}) (Policy, error)`
- `params` contains merged static config (from policy definition with resolved ${config} references) and runtime parameters (from API configuration)
- MUST return `policies.Policy` interface
- MUST return new instance (not singleton unless policy is stateless)
- Called for each route-policy combination during policy chain building

---

## Thread Safety Requirements

All policy methods MUST be thread-safe:

```go
type RateLimitPolicy struct {
    mu     sync.RWMutex
    limits map[string]*TokenBucket
}

func (p *RateLimitPolicy) ExecuteRequest(ctx *RequestContext, config map[string]interface{}) *RequestPolicyAction {
    identifier := p.getIdentifier(ctx, config)

    p.mu.RLock()
    bucket, exists := p.limits[identifier]
    p.mu.RUnlock()

    if !exists {
        p.mu.Lock()
        bucket = NewTokenBucket(config["requestsPerSecond"].(float64))
        p.limits[identifier] = bucket
        p.mu.Unlock()
    }

    if !bucket.Allow() {
        return &RequestPolicyAction{
            Action: ImmediateResponse{
                StatusCode: 429,
                Headers: map[string]string{"Retry-After": "1"},
                Body: []byte("Rate limit exceeded"),
            },
        }
    }

    return nil
}
```

---

## Error Handling Best Practices

### 1. Never Panic

```go
// BAD - panics on error
func (p *JWTPolicy) ExecuteRequest(ctx *RequestContext, config map[string]interface{}) *RequestPolicyAction {
    token := ctx.Headers["authorization"][0]  // panics if no authorization header
    // ...
}

// GOOD - handles missing header gracefully
func (p *JWTPolicy) ExecuteRequest(ctx *RequestContext, config map[string]interface{}) *RequestPolicyAction {
    authHeaders := ctx.Headers["authorization"]
    if len(authHeaders) == 0 {
        return &RequestPolicyAction{
            Action: ImmediateResponse{
                StatusCode: 401,
                Body: []byte("Missing authorization header"),
            },
        }
    }
    token := authHeaders[0]
    // ...
}
```

### 2. Use Typed Errors

```go
type PolicyError struct {
    Code    string
    Message string
    Cause   error
}

func (e *PolicyError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Usage in policy
func (p *JWTPolicy) validateJWT(token string, jwksUrl string) (*Claims, error) {
    keys, err := p.fetchJWKS(jwksUrl)
    if err != nil {
        return nil, &PolicyError{
            Code: "JWKS_FETCH_FAILED",
            Message: "Failed to fetch JWKS from " + jwksUrl,
            Cause: err,
        }
    }
    // ...
}
```

### 3. Fail Gracefully

```go
func (p *LoggingPolicy) ExecuteRequest(ctx *RequestContext, config map[string]interface{}) *RequestPolicyAction {
    // Non-critical policy - log errors but don't fail request
    if err := p.sendLog(ctx); err != nil {
        // Log the error but continue processing
        log.Warn("Failed to send log", "error", err, "request_id", ctx.RequestID)
    }
    return nil  // Don't block request due to logging failure
}
```

---

## Testing Policies

### Unit Test Example

```go
func TestJWTPolicy_ValidToken(t *testing.T) {
    policy := &JWTPolicy{
        jwksCache: NewMockJWKSCache(),
    }

    ctx := &RequestContext{
        Headers: map[string][]string{
            "authorization": {"Bearer valid.jwt.token"},
        },
        Metadata: make(map[string]interface{}),
        RequestID: "test-123",
    }

    config := map[string]interface{}{
        "jwksUrl": "https://example.com/.well-known/jwks.json",
        "issuer": "https://example.com",
        "audiences": []string{"api"},
    }

    action := policy.ExecuteRequest(ctx, config)

    // Should modify request with user headers
    assert.NotNil(t, action)
    mods := action.Action.(UpstreamRequestModifications)
    assert.Equal(t, "user-123", mods.SetHeaders["X-User-ID"])

    // Should set metadata
    assert.Equal(t, "user-123", ctx.Metadata["user_id"])
    assert.True(t, ctx.Metadata["authenticated"].(bool))
}

func TestJWTPolicy_InvalidToken(t *testing.T) {
    policy := &JWTPolicy{
        jwksCache: NewMockJWKSCache(),
    }

    ctx := &RequestContext{
        Headers: map[string][]string{
            "authorization": {"Bearer invalid.jwt.token"},
        },
        Metadata: make(map[string]interface{}),
        RequestID: "test-456",
    }

    config := map[string]interface{}{
        "jwksUrl": "https://example.com/.well-known/jwks.json",
    }

    action := policy.ExecuteRequest(ctx, config)

    // Should return immediate 401 response
    assert.NotNil(t, action)
    resp := action.Action.(ImmediateResponse)
    assert.Equal(t, 401, resp.StatusCode)
}
```

---

## Policy Module Structure

Each policy version should follow this structure:

```
policies/jwt-validation/v1.0.0/
├── policy.yaml          # Policy definition (REQUIRED)
├── jwt.go              # Implementation (REQUIRED)
├── jwt_test.go         # Unit tests (RECOMMENDED)
├── go.mod              # Go module definition (REQUIRED)
├── go.sum
└── README.md           # Documentation (RECOMMENDED)
```

**go.mod example**:
```go
module github.com/policy-engine/policies/jwt-validation

go 1.23

require (
    github.com/policy-engine/sdk v1.0.0
    github.com/golang-jwt/jwt/v5 v5.2.0
)

// For local development
replace github.com/policy-engine/sdk => ../../../sdk
```

**Note**: Policies depend on the **SDK module** (`github.com/policy-engine/sdk`), not the main policy-engine module. The SDK provides:
- Policy interfaces (`policies.Policy`, `policies.RequestPolicy`, `policies.ResponsePolicy`)
- Context types (`policies.RequestContext`, `policies.ResponseContext`)
- Action types (`policies.RequestPolicyAction`, `policies.ImmediateResponse`, etc.)
- Core registry (`core.PolicyRegistry`)

This architecture prevents cyclic dependencies between policies and the main engine.

**policy.yaml** must match implementation:
- `name` matches `Policy.Name()` return value
- `version` matches directory name
- `supportsRequestPhase` true if implements `RequestPolicy`
- `supportsResponsePhase` true if implements `ResponsePolicy`
- `requiresRequestBody` true if implementation accesses `ctx.Body` in request
- `requiresResponseBody` true if implementation accesses `ctx.ResponseBody` in response

---

This contract enables custom policy development while maintaining type safety, performance, and correct integration with the Policy Engine.

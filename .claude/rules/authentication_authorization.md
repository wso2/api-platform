# Go Authentication and Authorization Rules

Standards and patterns for ensuring secure authentication, authorization, token verification, and multi-tenant isolation across all Go services.

---

## GO-AUTH-001: Fail-Closed Authentication

### Severity

Critical

### Description

Authentication processes must always fail-closed. If an authentication check encounters an error, execution must terminate immediately, denying access to the resource.

### Rationale

Allowing execution to continue after an error, or failing to return early, can lead to authentication bypasses where unauthenticated requests are accidentally processed as valid.

### Non-Compliant Code

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        err := validateToken(token)
        if err != nil {
            // ERROR: Logs the error but falls through to the next handler
            log.Printf("auth failed: %v", err)
        }
        next.ServeHTTP(w, r)
    })
}

```

### Compliant Code

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        err := validateToken(token)
        if err != nil {
            log.Printf("auth failed: %v", err)
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusUnauthorized)
            json.NewEncoder(w).Encode(map[string]string{
                "error":   "unauthorized",
                "message": "Invalid or expired credentials.",
            })
            return // CORRECT: Execution terminates immediately
        }
        next.ServeHTTP(w, r)
    })
}

```
---

## GO-AUTH-002: Strict Asymmetric JWT Verification

### Severity

Critical

### Description

JWT signature verification must strictly enforce asymmetric algorithms (`RSA` or `EdDSA`). Symmetric algorithms (`HS256`, `HS384`, `HS512`) and the `none` algorithm must be explicitly rejected during signature validation.

### Rationale

If symmetric key algorithms are accepted by an asymmetric verification sequence, an attacker can sign a malicious JWT using the public key as a HMAC secret key, completely bypassing token signature verification.

### Non-Compliant Code

```go
// ERROR: Accepts any algorithm provided in the token header
token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
    return publicKey, nil 
})

```

### Compliant Code

```go
// CORRECT: Explicitly checks and restricts allowed signing methods
token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
    switch token.Method.(type) {
    case *jwt.SigningMethodRSA, *jwt.SigningMethodRSAPSS, *jwt.SigningMethodEd25519:
        return publicKey, nil
    default:
        return nil, fmt.Errorf("unexpected or forbidden signing method: %v", token.Header["alg"])
    }
})

```

---

## GO-AUTH-003: Secure Token Handling and Logging

### Severity

Medium

### Description

Raw authentication tokens, credentials, or secrets must never be written to application logs on failure. If part of a token is required for correlation or debugging, it must be masked to only show identifiers.

### Rationale

Logging raw tokens exposes sensitive credentials to log management platforms, increasing the surface area for account takeovers if log data is leaked or compromised.

### Non-Compliant Code

```go
// ERROR: Logging the full authentication token in plain text
if err != nil {
    log.Printf("failed to parse token %s: %v", r.Header.Get("Authorization"), err)
}

```

### Compliant Code

```go
// CORRECT: Masking the token before logging
func maskToken(token string) string {
    if len(token) <= 8 {
        return "[MASKED]"
    }
    return token[:4] + "..." + token[len(token)-4:]
}

if err != nil {
    masked := maskToken(r.Header.Get("Authorization"))
    log.Printf("failed to parse token %s: %v", masked, err)
}

```

---

## GO-AUTH-004: Routing and Path Traversal Protection

### Severity

High

### Description

Authentication and authorization layers must protect against routing anomalies. Applications must use clean paths to evaluate middleware execution, preventing attackers from bypassing auth controls via path traversal sequence tricks (`..`).

### Rationale

Attackers manipulate URLs (e.g., `//auth/../private`) to confuse naive path matching algorithms, tricking security layers into treating a restricted path as an unauthenticated/public path.

### Non-Compliant Code

```go
// ERROR: String prefix matching on raw path is vulnerable to bypasses
if strings.HasPrefix(r.URL.Path, "/public/") {
    next.ServeHTTP(w, r) // Bypasses auth checks
    return
}

```

### Compliant Code

```go
// CORRECT: Path sanitization and structured router group scoping
func NewRouter() http.Handler {
    mux := http.NewServeMux() // Go 1.22+ handles routing cleaning automatically
    
    // Explicit public routes
    mux.HandleFunc("GET /public/", publicHandler)
    
    // Explicitly protected sub-router or structured middleware chaining
    protectedMux := http.NewServeMux()
    protectedMux.HandleFunc("GET /private/", privateHandler)
    
    mux.Handle("/private/", AuthMiddleware(protectedMux))
    return mux
}

```

---

## GO-AUTH-005: Multi-Tenant Isolation (Anti-Privilege Escalation)

### Severity

Critical

### Description

Data queries and state mutations must enforce structural boundaries using multi-tenant context verification. User actions must be strictly constrained to their authorized `organization_id` or `tenant_id` pulled securely from the parsed token claims.

### Rationale

Relying strictly on an identifier provided directly in the request body or URL path parameters allows users to perform Cross-Organization Privilege Escalation by swapping resource IDs.

### Non-Compliant Code

```go
// ERROR: Trusting the organization ID from input without matching JWT identity
func DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
    targetOrgID := r.URL.Query().Get("org_id") 
    userID := r.URL.Query().Get("user_id")
    
    db.Where("id = ? AND organization_id = ?", userID, targetOrgID).Delete(&User{})
}

```

### Compliant Code

```go
// CORRECT: Forcing database execution context to rely on JWT claims context
func DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
    // Extracted safely inside AuthMiddleware and injected into Request Context
    ctxOrgID, ok := r.Context().Value(OrgIDContextKey).(string)
    if !ok || ctxOrgID == "" {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }
    
    userID := r.URL.Query().Get("user_id")
    
    // Query is strictly sandboxed inside the tenant domain checked by security token
    err := db.Where("id = ? AND organization_id = ?", userID, ctxOrgID).Delete(&User{}).Error
    if err != nil {
        http.Error(w, "Internal Error", http.StatusInternalServerError)
        return
    }
}

```

---

## GO-AUTH-006: HTTP Method Case-Insensitive Normalization

### Severity

High

### Description

HTTP method strings sourced from user input — API definitions, CRD specs, policy configurations, and access control exception lists — must be normalized to uppercase with `strings.ToUpper()` at the earliest point of extraction, before any comparison, map key construction, or route configuration.

### Rationale

RFC 7231 defines HTTP methods as case-sensitive and standard methods (`GET`, `POST`, etc.) are uppercase. However, user-supplied method values (e.g., from Kubernetes CRD fields, OpenAPI spec submissions, or policy attachments) may arrive in any case. Two classes of exploit are possible when normalization is missing:

1. **Access control bypass:** Security policy registries (deny lists, scope maps, exception sets) are built from one code path while incoming request methods come from another. If one path stores `"get"` and the other stores `"GET"`, map key lookups (`key.method != policyMethod`) silently fail to match, causing deny rules to never fire.
2. **Envoy route mismatch:** Gateway route translators embed the method string directly in Envoy's `Exact:` header matcher for the `:method` pseudo-header. A lowercase `"get"` produces a route that matches nothing (all real HTTP clients send `"GET"`), creating a silent routing failure that can be exploited to reach backends without going through the intended policy chain.

### Non-Compliant Code

```go
// ERROR: Raw method from CRD fed into route key and Envoy matcher — case not normalized
for _, op := range apiData.Operations {
    routeKey := GenerateRouteName(string(op.Method), context, version, op.Path, vhost)
    rdc.Routes[routeKey] = &models.Route{
        Method: string(op.Method), // Lowercase "get" → Envoy Exact match never fires
    }
}

// ERROR: Exception methods from user spec stored without normalization
for i, m := range ex.Methods {
    methods[i] = string(m) // "get" != "GET" in deny-list map key comparison
}

// ERROR: Policy methods expanded without normalization
for i, m := range methods {
    expanded[i] = string(m) // Case-sensitive comparison against WILDCARD_HTTP_METHODS fails
}
```

### Compliant Code

```go
// CORRECT: Normalize at the point of extraction from user-supplied data
for _, op := range apiData.Operations {
    opMethod := strings.ToUpper(string(op.Method)) // Normalize once, use everywhere
    routeKey := GenerateRouteName(opMethod, context, version, op.Path, vhost)
    rdc.Routes[routeKey] = &models.Route{
        Method: opMethod, // Always uppercase — Envoy Exact match fires correctly
    }
}

// CORRECT: Exception methods normalized before building deny-list keys
for i, m := range ex.Methods {
    methods[i] = strings.ToUpper(string(m)) // "get" → "GET", matches registry keys
}

// CORRECT: Policy methods normalized on expansion
for i, m := range methods {
    expanded[i] = strings.ToUpper(string(m)) // Consistent with WILDCARD_HTTP_METHODS constants
}

// CORRECT: RDC route method normalized before Envoy header matcher construction
method := strings.ToUpper(rdcRoute.Method)
// ... method is then used in: HeaderMatcher_StringMatch { Exact: method }
```

> **Verification Checklist before outputting code:**
> * Is every `string(op.Method)`, `string(m)`, or equivalent extraction from a user-supplied typed method value wrapped in `strings.ToUpper()`? (If no, add normalization at the extraction site.)
> * Are route keys generated for lookup and creation using the same normalized method string? (Inconsistent case between build and lookup sites causes silent key misses.)
> * Does any Envoy `Exact:` header matcher for `:method` receive a value that may be lowercase? (If yes, apply `strings.ToUpper()` before passing to the matcher.)
> * Are access control deny-list or scope-registry map keys built from normalized strings? (A mixed-case key will silently bypass all deny/allow lookups keyed on the uppercase constant.)
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

---

## GO-AUTH-007: Deny-by-Default Authorization on Admin/System/Internal REST APIs

### Severity

Critical

### Description

Every Admin REST API, System REST API, or internal-only endpoint (e.g. `platform-api` operations backing the `ap` CLI, gateway-controller control-plane endpoints) must perform an explicit, per-endpoint scope/role check before executing. Network placement (bound to a private interface, reachable only from inside a cluster) and JWT algorithm allowlisting (GO-AUTH-002) are necessary but not sufficient — the handler itself must independently verify that the caller's token carries the specific administrative scope that endpoint requires.

### Rationale

Broken access control on admin and internal endpoints is one of the most severe and recurring security issues across API management platforms. The common failure is an internal/admin endpoint that assumes it is protected by something *outside* the handler — network topology, a shared upstream gateway, an assumed-trustworthy Key Manager — rather than checking the caller's actual granted scope itself. JWT algorithm confusion, self-registered users obtaining elevated tokens via shared Key Managers, low-privileged tokens accepted by admin APIs, and DCR endpoints issuing tokens without access control are all manifestations of this same root cause.

### Non-Compliant Code

```go
// ERROR: Assumes this handler is only reachable internally / by admins because
// it's registered under an "/internal" or "/admin" path prefix — no scope check
// inside the handler itself. A JWT bypass, misrouted proxy, or shared Key Manager
// misconfiguration upstream is enough to reach this code with any valid token.
func RotateApiKeyHandler(w http.ResponseWriter, r *http.Request) {
    apiID := r.URL.Query().Get("api_id")
    newKey, err := rotateApiKey(apiID)
    if err != nil {
        http.Error(w, "Internal Error", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(map[string]string{"api_key": newKey})
}

// ERROR: Checks that a token exists and is valid, but never checks that it
// carries the specific admin scope this System REST API requires.
func DeleteTenantHandler(w http.ResponseWriter, r *http.Request) {
    claims, ok := r.Context().Value(ClaimsContextKey).(*Claims)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    deleteTenant(r.URL.Query().Get("tenant_id")) // Any authenticated caller can reach this
}
```

### Compliant Code

```go
// CORRECT: Explicit, per-endpoint scope check inside the handler — independent
// of network placement, upstream proxy assumptions, or which Key Manager issued
// the token. Deny-by-default: absence of the required scope is a 403, not a pass-through.
const ScopeAdminApiKeyRotate = "internal_apikey_rotate"

func RotateApiKeyHandler(w http.ResponseWriter, r *http.Request) {
    claims, ok := r.Context().Value(ClaimsContextKey).(*Claims)
    if !ok {
        writeUnauthorized(w)
        return
    }
    if !claims.HasScope(ScopeAdminApiKeyRotate) {
        // Generic 403 — do not reveal which scope was expected (aids scope probing).
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    apiID := r.URL.Query().Get("api_id")
    newKey, err := rotateApiKey(apiID)
    if err != nil {
        logger.LogInternalError(r.Context(), "api key rotation failed: %v", err)
        http.Error(w, "Internal Error", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(map[string]string{"api_key": newKey})
}

// CORRECT: A shared middleware that enforces the required scope for every
// route registered under it, so no individual handler can omit the check by
// mistake — but the handler above still re-checks explicitly for defense in depth.
func RequireScope(scope string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims, ok := r.Context().Value(ClaimsContextKey).(*Claims)
        if !ok || !claims.HasScope(scope) {
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

> **Verification Checklist before outputting code:**
> * Does an Admin/System/internal REST API handler check anything beyond "is this token valid" before executing a privileged operation? (If it only checks validity, add an explicit scope/role check for that specific operation.)
> * Is the authorization decision made inside the handler (or a middleware wrapping that specific route), rather than assumed from network placement, path prefix, or which upstream Key Manager/IdP issued the token? (If assumed, add an explicit in-handler check — token issuance source is not an authorization boundary.)
> * Does a Dynamic Client Registration, self-registration, or similarly "low trust" issuance flow ever produce a token capable of reaching a System/Admin REST API? (If yes, that issuance flow needs its own scope ceiling independent of downstream endpoint checks.)

---

## GO-AUTH-008: Parameterized Queries for Administrative Data Access

### Severity

Critical

### Description

Every SQL query built from request input — most acutely in Admin REST API handlers backed by `sqlx`/`database/sql` over the project's SQLite (`go-sqlite3`) store — must use parameterized placeholders (`?` with `sqlx`/`database/sql`, or `sqlx.Named`/`sqlx.In` for dynamic `IN` clauses). Never build a query by string-concatenating or `fmt.Sprintf`-ing a request value into SQL text.

### Rationale

Authenticated SQL injection in Admin REST APIs is an exploitable bug class: an administrator manipulating database queries can exfiltrate data or disrupt availability. Administrative endpoints are not a lower-risk surface for injection just because the caller is already authenticated — an admin-scoped SQLi still crosses a trust boundary (reading/writing data the admin's own scope should not reach, or affecting availability platform-wide).

### Non-Compliant Code

```go
// ERROR: Request-supplied value concatenated directly into SQL text.
func SearchTenantsHandler(w http.ResponseWriter, r *http.Request) {
    name := r.URL.Query().Get("name")
    query := "SELECT id, name, status FROM tenants WHERE name LIKE '%" + name + "%'"
    rows, err := db.Query(query) // name = "%' OR '1'='1" defeats the filter entirely
    if err != nil {
        http.Error(w, "Internal Error", http.StatusInternalServerError)
        return
    }
    defer rows.Close()
    // ...
}

// ERROR: fmt.Sprintf into a dynamic ORDER BY / column list — still injectable
// even though it "looks like" metadata rather than a value.
func ListApisHandler(w http.ResponseWriter, r *http.Request) {
    sortCol := r.URL.Query().Get("sort")
    query := fmt.Sprintf("SELECT * FROM apis ORDER BY %s", sortCol)
    db.Query(query)
}
```

### Compliant Code

```go
// CORRECT: sqlx parameterized placeholder — the driver, not string formatting,
// handles escaping, so injected SQL metacharacters are inert data, never syntax.
func SearchTenantsHandler(w http.ResponseWriter, r *http.Request) {
    name := r.URL.Query().Get("name")
    var tenants []Tenant
    err := db.Select(&tenants,
        "SELECT id, name, status FROM tenants WHERE name LIKE ?",
        "%"+name+"%",
    )
    if err != nil {
        logger.LogInternalError(r.Context(), "tenant search failed: %v", err)
        http.Error(w, "Internal Error", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(tenants)
}

// CORRECT: Dynamic sort/column selection resolved against an explicit allowlist,
// never interpolated into the query string — this is the one class of SQL
// injection parameterization cannot fix directly (identifiers aren't values).
var allowedSortColumns = map[string]string{
    "name":       "name",
    "created_at": "created_at",
    "status":     "status",
}

func ListApisHandler(w http.ResponseWriter, r *http.Request) {
    sortCol, ok := allowedSortColumns[r.URL.Query().Get("sort")]
    if !ok {
        sortCol = "created_at" // Safe default when the requested column isn't allowlisted
    }
    query := fmt.Sprintf("SELECT * FROM apis ORDER BY %s", sortCol) // sortCol is now a fixed, known-safe constant
    var apis []Api
    if err := db.Select(&apis, query); err != nil {
        http.Error(w, "Internal Error", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(apis)
}
```

> **Verification Checklist before outputting code:**
> * Is any SQL query built with string concatenation, `+`, or `fmt.Sprintf`/`fmt.Sprintf`-style interpolation of a request-derived value? (If yes, rewrite using `?` placeholders via `sqlx`/`database/sql`.)
> * Does a dynamic `ORDER BY`, table name, or column list come from request input? (Placeholders cannot parameterize identifiers — resolve against an explicit allowlist map instead, never interpolate the raw value even after "validation.")
> * Is this query reachable from an Admin/System REST API handler? (Authenticated-admin-only reachability is not a reason to relax this directive — see GO-AUTH-007, which governs *who* can reach the handler; this directive governs how the handler must build its query regardless of who called it.)

---

## GO-AUTH-009: Token and Session Invalidation on Security-State Change

### Severity

High

### Description

Whenever a security-relevant state change occurs — logout, account lock, password reset, role/permission change, sub-organization disablement, or user deletion — all previously issued access tokens, refresh tokens, and session-bound tokens tied to that identity must be actively revoked, not merely left to expire naturally. Checking current account status at authentication time is not sufficient if a token issued *before* the state change remains independently valid until its own expiry.

### Rationale

Failure to revoke tokens on security-state changes is a frequently recurring vulnerability pattern: session-bound tokens not revoked when a session ends, tokens issued before an account lock remaining valid after it, role removal failing to invalidate previously issued tokens, and stale authorization codes remaining usable after a user is deleted. In every case, the *authentication* check was correct at token-issuance time; the gap is that nothing re-validates or revokes the token when the security state that justified issuing it later changes.

### Non-Compliant Code

```go
// ERROR: Locks the account but does nothing to the tokens already issued to it —
// they remain valid (and continue to pass introspection) until natural expiry.
func LockAccountHandler(w http.ResponseWriter, r *http.Request) {
    userID := r.URL.Query().Get("user_id")
    if err := setAccountStatus(userID, StatusLocked); err != nil {
        http.Error(w, "Internal Error", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusNoContent)
    // Missing: revoke all active tokens/sessions for userID
}

// ERROR: Role change updates the stored role but issued tokens still carry
// (or are still authorized against) the old role until they expire on their own.
func UpdateUserRoleHandler(w http.ResponseWriter, r *http.Request) {
    updateUserRole(r.URL.Query().Get("user_id"), r.URL.Query().Get("role"))
    w.WriteHeader(http.StatusNoContent)
}
```

### Compliant Code

```go
// CORRECT: The account/role state change and the durable revocation marker
// (token_version) commit atomically in one transaction — a token minted
// before this commit cannot remain valid on a partial failure or race with a
// concurrent change, because authMiddleware checks token_version against the
// same row this transaction updates.
func LockAccountHandler(w http.ResponseWriter, r *http.Request) {
    userID := r.URL.Query().Get("user_id")

    tx, err := db.BeginTx(r.Context(), nil)
    if err != nil {
        http.Error(w, "Internal Error", http.StatusInternalServerError)
        return
    }
    defer tx.Rollback() // No-op once committed.

    // A single atomic UPDATE: the row-level lock held for its duration means
    // concurrent lock/role-change requests serialize on this row instead of
    // racing to compute token_version independently (no read-then-write gap).
    if _, err := tx.ExecContext(r.Context(),
        `UPDATE accounts SET status = ?, token_version = token_version + 1 WHERE id = ?`,
        StatusLocked, userID,
    ); err != nil {
        http.Error(w, "Internal Error", http.StatusInternalServerError)
        return
    }
    if err := tx.Commit(); err != nil {
        http.Error(w, "Internal Error", http.StatusInternalServerError)
        return
    }

    // Best-effort cleanup of an out-of-band session/token cache. The durable
    // token_version bump above — already committed — is what authMiddleware
    // actually enforces, so a failure here cannot leave old tokens valid.
    if err := tokenStore.RevokeAllForUser(r.Context(), userID); err != nil {
        logger.LogInternalError(r.Context(), "failed to revoke cached sessions after lock for %s: %v", userID, err)
    }
    w.WriteHeader(http.StatusNoContent)
}

// allowedRoles is the closed set of roles a caller may assign — never accept
// an arbitrary string into the role column (GO-AUTH-008 identifier/allowlist
// principle applies equally to enum-like values, not just identifiers).
var allowedRoles = map[string]bool{
    "member": true,
    "admin":  true,
    "owner":  true,
}

func UpdateUserRoleHandler(w http.ResponseWriter, r *http.Request) {
    // GO-AUTH-007: deny-by-default — token validity alone is not authorization.
    // The caller must carry the specific scope for this privileged operation,
    // checked before any DB work begins.
    claims, ok := r.Context().Value(ClaimsContextKey).(*Claims)
    if !ok || !claims.HasScope(ScopeAdminUserRoleUpdate) {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    userID := r.URL.Query().Get("user_id")
    newRole := r.URL.Query().Get("role")

    // Validate the requested role against the allowlist before touching the
    // database — an invalid or unrecognized role must never reach the UPDATE.
    if !allowedRoles[newRole] {
        http.Error(w, "Bad Request", http.StatusBadRequest)
        return
    }

    tx, err := db.BeginTx(r.Context(), nil)
    if err != nil {
        http.Error(w, "Internal Error", http.StatusInternalServerError)
        return
    }
    defer tx.Rollback()

    // Role change and revocation marker committed atomically — a token minted
    // under the old role cannot remain authoritative on a partial failure.
    if _, err := tx.ExecContext(r.Context(),
        `UPDATE accounts SET role = ?, token_version = token_version + 1 WHERE id = ?`,
        newRole, userID,
    ); err != nil {
        http.Error(w, "Internal Error", http.StatusInternalServerError)
        return
    }
    if err := tx.Commit(); err != nil {
        http.Error(w, "Internal Error", http.StatusInternalServerError)
        return
    }

    if err := tokenStore.RevokeAllForUser(r.Context(), userID); err != nil {
        logger.LogInternalError(r.Context(), "failed to revoke cached sessions after role change for %s: %v", userID, err)
    }
    w.WriteHeader(http.StatusNoContent)
}
```

> **Verification Checklist before outputting code:**
> * Does this handler change account status, role/permission, or delete a user/sub-organization, without also revoking that identity's active tokens/sessions? (If yes, add revocation as part of the same transaction/operation.)
> * Is account status checked only at authentication/token-issuance time, with no re-check for tokens already in circulation? (If a token's validity depends solely on its own signature/expiry with no live status check, add either revocation-on-change or a short-lived-token-plus-introspection pattern.)
> * On revocation failure, does the handler still return success to the caller? (Revocation failure must fail the overall operation — a lock/role-change that "succeeds" while old tokens remain valid is a silent authorization gap.)

---

## GO-AUTH-010: Redirect and Callback URL Allowlisting (Open Redirect Prevention)

### Severity

Medium

### Description

Any server-generated HTTP redirect (`Location` header) whose target is derived, even partially, from request input — a post-login `returnTo`/`redirect_uri` parameter, an OAuth/OIDC callback URL, an SSO logout redirect — must be validated against an explicit allowlist of registered destinations before being used. Never redirect to a URL solely because it "looks like" it belongs to the same host (string-prefix or substring checks are bypassable).

### Rationale

Open redirect is a persistently recurring vulnerability class. Weak callback URL validation, unvalidated redirect construction, and open redirects in logout or account-recovery flows are consistently used as phishing primitives: the redirect originates from a trusted domain, which is exactly what makes it effective against users who "check the domain" before entering credentials on the destination page.

### Non-Compliant Code

```go
// ERROR: Redirects to whatever the caller supplies, with only a same-host
// substring check — bypassable via "https://trusted.example.com.attacker.com"
// or an open second-parameter trick like "https://trusted.example.com@attacker.com".
func LoginCallbackHandler(w http.ResponseWriter, r *http.Request) {
    returnTo := r.URL.Query().Get("returnTo")
    if strings.Contains(returnTo, "trusted.example.com") {
        http.Redirect(w, r, returnTo, http.StatusFound)
        return
    }
    http.Redirect(w, r, "/", http.StatusFound)
}
```

### Compliant Code

```go
// CORRECT: Validated against an explicit, server-controlled allowlist of exact
// registered redirect targets — the same pattern OAuth/OIDC redirect_uri
// validation already requires, applied uniformly to every server-generated redirect.
var allowedRedirectHosts = map[string]bool{
    "portal.example.com":  true,
    "console.example.com": true,
}

func safeRedirectTarget(raw string) (string, bool) {
    if raw == "" {
        return "/", true
    }
    u, err := url.Parse(raw)
    if err != nil {
        return "", false
    }
    // Relative paths are safe by construction — no scheme, opaque component
    // (rules out "javascript:alert(1)", "https:evil.com"), or host, and must
    // start with a single leading slash (rules out protocol-relative "//evil.com").
    if u.Scheme == "" && u.Opaque == "" && u.Host == "" && strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
        return u.String(), true
    }
    // Reject any userinfo component outright — "https://attacker.com@portal.example.com"
    // parses with a legitimate, allowlisted Host but still carries a userinfo
    // segment that other URL parsers/clients along the redirect path may not
    // interpret identically. There is no legitimate reason for a redirect
    // target to carry credentials, so the shape is rejected before the host check.
    if u.User != nil {
        return "", false
    }
    if u.Scheme != "https" || !allowedRedirectHosts[u.Host] {
        return "", false
    }
    return u.String(), true
}

func LoginCallbackHandler(w http.ResponseWriter, r *http.Request) {
    target, ok := safeRedirectTarget(r.URL.Query().Get("returnTo"))
    if !ok {
        target = "/" // Fall back to a safe default — never echo the rejected value back
    }
    http.Redirect(w, r, target, http.StatusFound)
}
```

> **Verification Checklist before outputting code:**
> * Is a redirect target validated with a substring/prefix check (`strings.Contains`, `strings.HasPrefix`) rather than a parsed-URL host comparison against an explicit allowlist? (Substring checks are bypassable — replace with `url.Parse` + exact host match.)
> * Does the validated redirect target allow scheme-relative URLs (`//attacker.com`) or a userinfo trick (`https://trusted.com@attacker.com`)? (Both parse as "same host" under naive checks — `safeRedirectTarget`-style explicit host comparison closes both.)
> * On rejection, does the handler redirect to a safe default rather than erroring in a way that reflects the rejected URL back into the response? (Reflecting it back risks reintroducing an XSS surface — see `js-output-encoding-xss.md` for the same principle in JS.)
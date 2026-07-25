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
// ERROR: Assumes network placement/path prefix is sufficient protection — no
// scope check inside the handler itself.
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
    rows, err := db.Query(query) // Injectable — filter can be defeated
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
// substring check — bypassable via crafted lookalike/userinfo hosts.
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

---

## GO-AUTH-011: Fail-Closed Startup Validation for Security-Critical Configuration

### Severity

Critical

### Description

At startup, validate that the *resulting* security configuration is actually enforceable, not merely that each field in isolation is well-formed. If authentication is nominally "enabled" but no authenticator can actually be constructed from the given config (e.g. basic-auth enabled with zero registered users), or a control-plane channel is configured to run without TLS, the process must refuse to start — or, in an explicitly-flagged development mode only, fail loudly on every request — rather than silently falling back to an open/passthrough state.

### Rationale

Per-field validation is not the same as validating the resulting behavior. A config can look superficially valid ("auth is enabled") while still producing zero effective authenticators, silently degrading to an open/passthrough state that a check scoped only to top-level flags wouldn't catch.

### Non-Compliant Code

```go
// ERROR: registers an authenticator only if BasicAuth.Enabled && len(Users) > 0.
// If Enabled is true but Users is empty, zero authenticators are registered —
// and the code silently treats that as "no auth needed" rather than "misconfigured."
func BuildAuthenticators(cfg AuthConfig) []Authenticator {
    var authns []Authenticator
    if cfg.BasicAuth.Enabled && len(cfg.BasicAuth.Users) > 0 {
        authns = append(authns, NewBasicAuthenticator(cfg.BasicAuth.Users))
    }
    return authns // Can be empty even though BasicAuth.Enabled == true
}

func AuthMiddleware(authns []Authenticator) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if len(authns) == 0 {
                // ERROR: passthrough — every request is treated as authenticated.
                ctx := context.WithValue(r.Context(), AuthContextKey, AuthContext{
                    Authenticated: true, UserID: "sys_noauth_user",
                })
                ctx = context.WithValue(ctx, AuthzSkipKey, true)
                next.ServeHTTP(w, r.WithContext(ctx))
                return
            }
            // ... real check
        })
    }
}
```

### Compliant Code

```go
// CORRECT: startup validation checks the *effective* result of the config,
// not just each field — refuses to start rather than silently degrading.
// NOTE: GO-AUTH-012 adds a third check (IDP expected-audience) to this SAME
// function rather than defining a second, differently-bodied
// ValidateAuthConfig — see the compliant example there for the merged form.
func ValidateAuthConfig(cfg AuthConfig) error {
    if cfg.Disable {
        // Explicit, off-by-default opt-out (see AuthConfig below) — startup is
        // allowed with zero configured authenticators; AuthMiddleware is what
        // logs the banner-level warning on every request while this is set.
        return nil
    }
    if cfg.BasicAuth.Enabled && len(cfg.BasicAuth.Users) == 0 {
        return fmt.Errorf("basic auth is enabled but no users are configured — " +
            "refusing to start with an unenforceable auth config")
    }
    if !cfg.BasicAuth.Enabled && !cfg.IDP.Enabled {
        return fmt.Errorf("no authentication method is configured — " +
            "set auth.disable=true explicitly if this is intentional")
    }
    return nil
}

func main() {
    cfg := loadConfig()
    if err := ValidateAuthConfig(cfg.Auth); err != nil {
        log.Fatalf("fatal: invalid auth configuration: %v", err) // Refuse to start
    }
    // ...
}

// An explicit, off-by-default opt-out — never an implicit fallback.
type AuthConfig struct {
    Disable   bool // Must be explicitly set; logs a startup banner-level warning on every request when true
    BasicAuth BasicAuthConfig
    IDP       IDPConfig
}
```

> **Verification Checklist before outputting code:**
> * Can any combination of "enabled" flags and empty sub-config (e.g. an empty `users` list) result in zero authenticators being registered while auth is still nominally "enabled"? (If yes, add a startup check that fails closed on this combination specifically, not just on `Enabled == false`.)
> * Does the existing "no authentication configured" warning check the *effective* authenticator count, or only whether specific top-level flags are false? (A flag-only check misses the "enabled but unusable" case.)
> * Is there an explicit, off-by-default `disable`/`insecure` flag for intentionally running without auth, distinct from a configuration that merely fails to produce a working authenticator?

---

## GO-AUTH-012: JWT Audience and Issuer Must Be Explicitly Validated

### Severity

High

### Description

Every JWT verification path must set an expected audience (`aud`) and issuer (`iss`) and reject tokens that omit or mismatch them, enforced at the parsing-library level (`jwt.WithAudience`, `jwt.WithIssuer`) rather than trusted as claims to be read and compared manually (or not compared at all). Configuration that enables IDP-based JWT auth must require a non-empty expected audience before the server will start.

### Rationale

A token correctly signed by a trusted IDP but *minted for a different client or service* (a different audience) still passes signature and expiry checks. Without an audience check, any valid token from the same IDP — regardless of which application it was issued for — is accepted as a credential for this service, collapsing the boundary between "this IDP is trusted" and "this specific token was intended for me."

### Non-Compliant Code

```go
// ERROR: parses and validates signature/expiry, but never checks `aud` against
// this service's expected audience — any token from the trusted IDP is accepted
// regardless of which client/service it was actually issued for.
func VerifyManagementToken(tokenStr string, idpPublicKey interface{}) (*Claims, error) {
    claims := &Claims{}
    _, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
        return idpPublicKey, nil
    })
    if err != nil {
        return nil, err
    }
    return claims, nil // claims.Audience is never inspected
}
```

### Compliant Code

```go
// CORRECT: expected audience is required configuration, enforced by the
// library itself so the check cannot be silently skipped by a future refactor.
// This check is added to the SAME ValidateAuthConfig defined under GO-AUTH-011
// — not a second, redefined function — so both checks run from one call site.
type IDPConfig struct {
    JWKSURL          string
    Issuer           string
    ExpectedAudience string // Required whenever IDP.Enabled == true
}

func ValidateAuthConfig(cfg AuthConfig) error {
    if cfg.Disable { // From GO-AUTH-011 — explicit, off-by-default opt-out
        return nil
    }
    if cfg.BasicAuth.Enabled && len(cfg.BasicAuth.Users) == 0 { // From GO-AUTH-011
        return fmt.Errorf("basic auth is enabled but no users are configured — " +
            "refusing to start with an unenforceable auth config")
    }
    if !cfg.BasicAuth.Enabled && !cfg.IDP.Enabled { // From GO-AUTH-011
        return fmt.Errorf("no authentication method is configured — " +
            "set auth.disable=true explicitly if this is intentional")
    }
    if cfg.IDP.Enabled && cfg.IDP.ExpectedAudience == "" { // Added by GO-AUTH-012
        return fmt.Errorf("idp auth is enabled but no expected audience is configured — " +
            "refusing to start with an unenforceable token scope")
    }
    return nil
}

func VerifyManagementToken(tokenStr string, idpPublicKey interface{}, cfg IDPConfig) (*Claims, error) {
    claims := &Claims{}
    _, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
        // GO-AUTH-002: restrict to asymmetric algorithms before ever returning
        // the key — never let the token header alone decide how it's verified,
        // and never accept "none" or a symmetric (HS*) algorithm here. The
        // key-type check below additionally guards against a token whose alg
        // claims one asymmetric family while idpPublicKey is actually another.
        switch t.Method.(type) {
        case *jwt.SigningMethodRSA:
            if _, ok := idpPublicKey.(*rsa.PublicKey); !ok {
                return nil, fmt.Errorf("configured key does not match RSA signing method")
            }
        case *jwt.SigningMethodECDSA:
            if _, ok := idpPublicKey.(*ecdsa.PublicKey); !ok {
                return nil, fmt.Errorf("configured key does not match ECDSA signing method")
            }
        case *jwt.SigningMethodEd25519:
            if _, ok := idpPublicKey.(ed25519.PublicKey); !ok {
                return nil, fmt.Errorf("configured key does not match Ed25519 signing method")
            }
        default:
            return nil, fmt.Errorf("unexpected or forbidden signing method: %v", t.Header["alg"])
        }
        return idpPublicKey, nil
    },
        jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "EdDSA"}), // Explicit allowlist, defense-in-depth with the switch above
        jwt.WithAudience(cfg.ExpectedAudience), // Library-enforced — fails closed on mismatch
        jwt.WithIssuer(cfg.Issuer),
    )
    if err != nil {
        return nil, fmt.Errorf("token verification failed: %w", err)
    }
    return claims, nil
}
```

> **Verification Checklist before outputting code:**
> * Does every `jwt.ParseWithClaims`/`jwt.Parse` call for a management or admin-facing token pass `jwt.WithAudience(...)`? (If the `aud` claim exists on the struct but is never checked via a parser option, add it.)
> * Is the expected audience sourced from required configuration that fails startup validation when IDP auth is enabled but the audience is empty? (If the audience is optional or defaulted silently, make it mandatory per GO-AUTH-011's pattern.)
> * Is `iss` (issuer) validated the same way? (Add `jwt.WithIssuer` alongside the audience check.)
> * Does this audience check live inside the same `ValidateAuthConfig` function established by GO-AUTH-011, rather than a second function of the same name/signature? (A duplicate definition is a compile error, and whichever copy survives silently drops the other rule's check.)

---

## GO-AUTH-013: Admin, Debug, and Metrics Interfaces Require Real Authentication, Not Merely IP Allowlisting

### Severity

Critical

### Description

Any admin, debug, or metrics HTTP/gRPC endpoint that exposes configuration dumps, runtime state, or sensitive operational data must be protected by the same authentication/authorization stack used for regular management APIs (GO-AUTH-007). An IP allowlist is defense-in-depth only — it must never default to allow-all (`["*"]`), and `Validate()` must reject a wildcard allowlist unless an explicit, off-by-default administrative flag opts into it.

### Rationale

An admin debug endpoint (`config_dump`) or a metrics endpoint reachable with no authentication, gated only by an IP allowlist that defaults to `["*"]`, is functionally unauthenticated from day one of any deployment that didn't override the default. IP-based controls are also routinely defeated inside a Kubernetes cluster (pod-to-pod traffic, a compromised sidecar) where "the caller's IP looks internal" is not evidence the caller is authorized.

### Non-Compliant Code

```go
// ERROR: config_dump is reachable by anyone whose IP passes an allowlist that
// defaults to allow-all — no authentication is applied at all.
func defaultConfig() AdminConfig {
    return AdminConfig{
        AllowedIPs: []string{"*"}, // Allow-all by default
    }
}

func (s *AdminServer) configDumpHandler(w http.ResponseWriter, r *http.Request) {
    if !isAllowedIP(r.RemoteAddr, s.cfg.AllowedIPs) {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }
    json.NewEncoder(w).Encode(s.currentConfig) // No authentication check at all
}
```

### Compliant Code

```go
// CORRECT: real authentication (the same middleware protecting the management
// API) gates the admin/debug/metrics mux, and the IP allowlist — still present
// as defense-in-depth — defaults to loopback-only.
func defaultConfig() AdminConfig {
    return AdminConfig{
        AllowedIPs: []string{"127.0.0.1", "::1"}, // Loopback-only default
    }
}

func (c AdminConfig) Validate() error {
    for _, ip := range c.AllowedIPs {
        if ip == "*" && !c.AllowWildcardIPAllowlist {
            return fmt.Errorf("admin.allowed_ips=[\"*\"] requires the explicit " +
                "admin.allow_wildcard_ip_allowlist opt-in flag")
        }
    }
    return nil
}

// NewAdminMux reuses the exact RequireScope (GO-AUTH-007) and AuthMiddleware
// (GO-AUTH-011) helpers already defined in this file — never bespoke,
// differently-signed helpers invented per rule.
func NewAdminMux(cfg AdminConfig, authns []Authenticator) http.Handler {
    mux := http.NewServeMux()
    mux.Handle("/config_dump", RequireScope("admin:config:read", http.HandlerFunc(configDumpHandler)))
    mux.Handle("/xds_sync_status", RequireScope("admin:xds:read", http.HandlerFunc(xdsSyncStatusHandler)))

    protected := AuthMiddleware(authns)(mux)                // Real authentication — GO-AUTH-011
    return ipAllowlistMiddleware(cfg.AllowedIPs)(protected) // IP check as defense-in-depth, not the only check
}
```

> **Verification Checklist before outputting code:**
> * Does an admin/debug/metrics handler rely solely on an IP allowlist with no authentication middleware in front of it? (If yes, wrap it in the same `AuthMiddleware`/`RequireScope` stack as management API routes.)
> * Does the default `AllowedIPs` value include `"*"` or an equivalent allow-all wildcard? (If yes, change the default to loopback-only and require an explicit opt-in flag to widen it.)
> * Do metrics labels expose high-cardinality, tenant-shaped values (org IDs, API IDs) to an unauthenticated scrape endpoint? (If yes, gate the endpoint the same way, and/or bucket/anonymize the label values.)

---

## GO-AUTH-014: No Default, Hardcoded, or Shipped Credentials

### Severity

Critical

### Description

Packaged/shipped configuration must never contain a functional default username/password (e.g. `admin`/`admin`). Either ship with zero users configured — which, combined with GO-AUTH-011's fail-closed startup validation, forces an operator to provide credentials before the service will run — or generate a random initial credential on first boot, persisted to a file with restrictive permissions, and never leave a static default credential active in a production build.

### Rationale

A shipped `config.toml` containing `username = "admin"` / `password = "admin"` is a credential every installer of the product possesses before they've configured anything; if it is ever left unchanged (a near-certainty across a large install base), it is a direct authentication bypass indistinguishable from no authentication at all.

### Non-Compliant Code

```toml
# ERROR: functional default credentials shipped in packaged configuration —
# present in every fresh install until an operator remembers to change them.
[controller.auth.basic]
enabled = true
[[controller.auth.basic.users]]
username = "admin"
password = "admin"
password_hashed = false
roles = ["admin"]
```

### Compliant Code

```go
// CORRECT: on first boot with zero configured users, generate a random
// credential, persist its hash restrictively, and never leave a static
// default active in a production build. Crucially, the persisted HASH is
// reloaded into cfg.Users on every subsequent boot — not just written once —
// so a restart doesn't leave BasicAuth.Enabled with zero users (which would
// otherwise trip GO-AUTH-011's own fail-closed startup check on the very
// next boot).
func EnsureInitialAdminCredential(cfg *BasicAuthConfig, dataDir string) error {
    if len(cfg.Users) > 0 {
        return nil // Operator has already configured credentials
    }

    credentialFile := filepath.Join(dataDir, "initial-admin-credential.hash")

    if existingHash, err := os.ReadFile(credentialFile); err == nil {
        // Already generated on a previous boot — reload the persisted hash
        // rather than leaving cfg.Users empty.
        cfg.Users = []BasicAuthUser{{Username: "admin", PasswordHash: string(existingHash), Roles: []string{"admin"}}}
        return nil
    } else if !os.IsNotExist(err) {
        return fmt.Errorf("failed to read persisted admin credential: %w", err)
    }

    randomPassword, err := generateSecurePassword(24) // crypto/rand-backed
    if err != nil {
        return fmt.Errorf("failed to generate initial admin credential: %w", err)
    }

    hashed, err := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost)
    if err != nil {
        return fmt.Errorf("failed to hash initial admin credential: %w", err)
    }
    if err := os.WriteFile(credentialFile, hashed, 0o600); err != nil {
        return fmt.Errorf("failed to persist initial admin credential: %w", err)
    }

    // The plaintext password is shown to the operator exactly once, at
    // generation time, and never written to disk or logs — only the bcrypt
    // hash above is persisted.
    fmt.Fprintf(os.Stderr, "Generated one-time initial admin password: %s — rotate it after first login.\n", randomPassword)
    cfg.Users = []BasicAuthUser{{Username: "admin", PasswordHash: string(hashed), Roles: []string{"admin"}}}
    return nil
}
```

> **Verification Checklist before outputting code:**
> * Does any packaged/shipped config file, Helm chart default, or Dockerfile set a functional (non-placeholder) username and password for an admin or service account? (If yes, remove it — see the two compliant patterns above.)
> * If a "development mode" flag gates a default credential, is that flag off by default and does the code log a loud, unambiguous warning whenever it is on? (A quiet or easily-forgotten dev flag is equivalent to a shipped default in practice.)
> * Is a generated initial credential's hash written with `0o600` (or equivalent) permissions and never logged in plaintext to a shared log stream? (Only the one-time console output should show the plaintext; persisted storage and logs must hold the hash only, per GO-AUTH-003.)
> * On a restart after the credential was already generated, is the persisted hash reloaded into `cfg.Users`, rather than merely detected-and-skipped? (If the function returns early without repopulating `cfg.Users`, GO-AUTH-011's fail-closed check will refuse to start the service on every boot after the first.)

---

## GO-AUTH-015: Enforce Security Invariants at the Service/Data Layer, Not Only in Route Middleware

### Severity

High

### Description

A cross-cutting invariant that must hold regardless of entry point — e.g. "no mutations while the system is in immutable/maintenance mode" — must be enforced at the lowest common layer every entry point funnels through (typically the service or data-access layer), not solely in HTTP middleware scoped to one route prefix. New entry points (event-driven mutation handlers, an admin server, a future RPC surface) automatically inherit the protection only if it lives below all of them; middleware scoped to `/api/management/v0.9/*` protects only requests that happen to arrive through that specific router.

### Rationale

A security invariant enforced only in middleware in front of one entry point protects just the requests that traverse that specific code path. Any other entry point that reaches the same underlying operation through a different route bypasses the check entirely, because the check was never present at the point where all paths actually converge.

### Non-Compliant Code

```go
// ERROR: the invariant is checked only in middleware wrapping one specific
// router. Any mutation path that doesn't traverse this exact middleware chain
// (an event handler, the admin server, a future gRPC endpoint) bypasses it.
func ImmutableModeMiddleware(isImmutable func() bool) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if isImmutable() && r.Method != http.MethodGet {
                http.Error(w, "Locked", http.StatusMethodNotAllowed)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
mux.Handle("/api/management/v0.9/", ImmutableModeMiddleware(cfg.IsImmutable)(managementRouter))
// Platform-API event handler and admin server call restAPIService.Update directly —
// neither passes through this middleware.
```

### Compliant Code

```go
// CORRECT: the check lives inside the service method itself — the one place
// every caller (REST, WSS/Platform-API events, admin server, future RPC)
// necessarily passes through before a mutation can occur.
type RestAPIService struct {
    db          *sql.DB
    isImmutable func() bool
}

func (s *RestAPIService) Update(ctx context.Context, apiID string, def APIDefinition) error {
    if s.isImmutable() {
        return ErrImmutableModeActive // One check protects every entry point
    }
    return s.db.UpdateAPI(ctx, apiID, def)
}

// Middleware may still exist for a fast HTTP-layer rejection (better UX/latency),
// but it is defense-in-depth on top of the service-layer check, never a substitute for it.
```

> **Verification Checklist before outputting code:**
> * Is a security invariant (immutable mode, a maintenance lock, a tenant-suspension check) implemented only as route middleware scoped to one router or path prefix? (If yes, move the authoritative check into the service/data layer method(s) that perform the actual mutation.)
> * Are there multiple ways to reach the same mutation (HTTP handler, event/webhook consumer, admin server, background job)? (Enumerate them and confirm the invariant check is reachable from every one, not just the primary HTTP path.)

---

## GO-AUTH-016: Verify Server/Peer-Asserted Identity Against Locally-Known Configuration

### Severity

Medium

### Description

When a remote peer (a control plane, an upstream gateway) asserts its own identity or a configuration value back to you over an already-established channel (e.g. an `ack` message echoing a `gateway_id`), compare that asserted value against what you independently expect or hold locally before trusting it. An authenticated transport proves who signed the connection; it does not prove that every value inside the payload is self-consistent with what you expect — a compromised or misconfigured peer can still assert an unexpected value over a channel it legitimately authenticated to.

### Rationale

Accepting a server-asserted identity value without comparing it to the locally configured value lets a hostile or misconfigured peer assign an unexpected identity that the gateway then silently adopts — an identity-confusion condition between what the operator configured and what the gateway believes about itself.

### Non-Compliant Code

```go
// ERROR: accepts the server-asserted gateway_id with no comparison to the
// locally configured value — the connection being authenticated says nothing
// about whether this specific field is what was expected.
func (c *ControlPlaneClient) handleConnectionAck(ack *ConnectionAck) {
    c.assignedGatewayID = ack.GatewayID // Trusted without verification
}
```

### Compliant Code

```go
// CORRECT: the server-asserted value is checked against the locally configured
// identity before being accepted; a mismatch is treated as a security-relevant
// event and fails closed on THIS CONNECTION — but, per
// go-network-service-hardening.md directive 6, a remote peer's assertion must
// never be the direct trigger for terminating this process (os.Exit/log.Fatalf
// here would let a compromised or merely buggy control plane crash-loop every
// connected instance by sending a mismatched gateway_id). The connection is
// torn down and the client enters a local degraded/backoff state instead.
func (c *ControlPlaneClient) handleConnectionAck(ack *ConnectionAck) error {
    if ack.GatewayID != c.gatewayID {
        logger.Error("control plane asserted an unexpected gateway_id",
            "expected", c.gatewayID, "asserted", ack.GatewayID)
        return fmt.Errorf("gateway_id mismatch from control plane — refusing to proceed")
    }
    return nil
}

// Caller treats a mismatch as a permanent failure of THIS CONNECTION, not of
// the process: it refuses to adopt the asserted identity, closes the
// connection, and marks the client degraded (surfaced via a local
// health/readiness endpoint, with backoff before reconnecting) rather than
// calling os.Exit/log.Fatalf on the control plane's say-so.
if err := client.handleConnectionAck(ack); err != nil {
    logger.Error("control plane identity verification failed — entering degraded mode", "error", err)
    c.health.degraded.Store(true)
    c.closeConnection()
    return // Caller's reconnect/backoff loop decides what happens next
}
```

> **Verification Checklist before outputting code:**
> * Does code accept an identity, ID, or configuration value asserted by a remote peer and use it without comparing it to a locally-held expectation? (If yes, add the comparison and fail closed on mismatch.)
> * Is a mismatch treated as a security event (logged distinctly, connection torn down and refused) rather than silently overwritten or ignored? (An identity mismatch from an authenticated peer is more suspicious than a mismatch from an unauthenticated one — the peer had to get *something* right to reach this point.)
> * Does the mismatch handler call `os.Exit`/`log.Fatalf` directly on a remote-asserted value? (If yes, replace with the degraded-state/circuit-breaker pattern from `go-network-service-hardening.md` directive 6 — a remote peer's assertion, even a mismatched one, must never be the direct trigger for terminating this process.)

---

## GO-AUTH-017: Security-Relevant Route Matching Must Use Structural Route Identity, Not Heuristic String Matching

### Severity

High

### Description

Middleware or policy code that decides *whether* authentication/authorization applies to a request must key that decision off the route's actual registered binding or policy attachment — a structural match against the router/policy configuration — never a heuristic like "`method == POST` and the path contains the substring `mcp`". A heuristic predicate has false negatives for any request shape its author didn't anticipate, and those negatives silently *skip* the security check entirely rather than degrading gracefully or erroring.

### Rationale

A security gate keyed off a request-shape heuristic (e.g. matching on HTTP method plus a path substring) has false negatives for any request shape its author didn't anticipate — and those negatives silently skip the security check entirely, with no error or log signal that anything was bypassed.

### Non-Compliant Code

```go
// ERROR: whether auth applies is decided by a request-shape heuristic. Any
// request that doesn't match this exact shape skips auth/authz entirely,
// silently, with no indication that a check was bypassed.
func MCPAuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "mcp") {
            if !isAuthenticated(r) {
                http.Error(w, "Forbidden", http.StatusForbidden)
                return
            }
        }
        next.ServeHTTP(w, r) // Any other method/path combination — no check at all
    })
}
```

### Compliant Code

```go
// CORRECT: the decision is keyed off the route's actual registered binding —
// resolved from router/policy configuration — not a guess about request shape.
// A request that is in-scope for MCP handling but doesn't parse into the
// expected structural shape is a failure to be denied, not a check to skip.
type RouteBinding struct {
    Kind         string // "mcp", "rest", "llm-proxy", ...
    RequiresAuth bool
}

// ResolveRouteBinding distinguishes two failure shapes: a request that falls
// entirely outside any namespace this middleware governs (matched=false —
// let normal routing/not-found handling take it) versus a request that DOES
// target a protected namespace but couldn't be parsed into a valid structural
// binding (err != nil — deny by default, per GO-AUTH-017's own rationale).
// Collapsing both into a blanket 403 would turn every unrelated 404 into a
// Forbidden response, which is not what "deny by default" is meant to cover.
func ResolveRouteBinding(r *http.Request, router *Router) (binding RouteBinding, matched bool, err error) {
    binding, ok := router.Match(r) // Structural match against registered routes
    if !ok {
        if router.IsProtectedNamespace(r) {
            return RouteBinding{}, true, fmt.Errorf("request targets a protected namespace but did not match a registered route binding")
        }
        return RouteBinding{}, false, nil // Outside any namespace this middleware governs
    }
    return binding, true, nil
}

func AuthMiddleware(router *Router, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        binding, matched, err := ResolveRouteBinding(r, router)
        if err != nil {
            // Confirmed protected namespace, unresolvable shape — deny, don't pass through.
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }
        if !matched {
            // Not in any namespace this middleware protects — fall through to
            // normal routing (e.g. a 404), not a 403.
            next.ServeHTTP(w, r)
            return
        }
        if binding.RequiresAuth && !isAuthenticated(r) {
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

> **Verification Checklist before outputting code:**
> * Does a security gate (auth, authz, ACL) decide whether it applies based on `r.Method`/`strings.Contains(r.URL.Path, ...)` or similar request-shape heuristics, rather than a structural match against registered routes/policy bindings? (If yes, replace with a resolution against the actual routing/policy configuration.)
> * Is the predicate used consistently across every middleware in the same protection chain (e.g. auth, authz, and ACL for the same feature)? (Divergent heuristics across sibling middlewares — even if each looks reasonable alone — can each miss a different edge case, compounding the gap.)
> * When a request is in a security-sensitive namespace but fails to match the expected structural shape, does the code deny by default rather than silently continuing? (An unrecognized shape must be treated as untrusted, not as "not applicable.")

---

## GO-AUTH-018: Fail-Closed on Security-Critical File and Socket Permission Checks

### Severity

High

### Description

When code checks the permission bits of a security-critical file or Unix domain socket (a master encryption key, a private key, an IPC socket used for privileged control), a too-permissive result must cause the operation to fail — return an error, abort startup — not merely log a warning and continue. Relying on the process umask for a newly created Unix domain socket is equivalent to not checking permissions at all, since the umask is rarely tuned specifically for that socket.

### Rationale

A permission check that only warns still proceeds to load and use the security-critical material regardless of the result, which is equivalent to not checking at all. Relying on the ambient umask for a newly created socket has the same effect — it depends entirely on environment defaults rather than an explicit, verified check.

### Non-Compliant Code

```go
// ERROR: detects an overly permissive key file but only warns — the key is
// still loaded and used.
func LoadMasterKey(path string) ([]byte, error) {
    info, err := os.Stat(path)
    if err != nil {
        return nil, err
    }
    if info.Mode().Perm()&0o077 != 0 {
        log.Warnf("key file %s has overly permissive permissions %o", path, info.Mode().Perm())
    }
    return os.ReadFile(path) // Proceeds regardless
}
```

```python
# ERROR: Unix domain socket created with no explicit chmod — relies on the
# ambient umask, which is typically 022 (world-readable/executable).
server.add_insecure_port(f"unix://{socket_path}")
server.start()
```

### Compliant Code

```go
// CORRECT: an overly permissive key file is a hard failure, not a warning,
// with a narrowly-scoped, off-by-default development opt-out.
func LoadMasterKey(path string, devMode bool) ([]byte, error) {
    info, err := os.Stat(path)
    if err != nil {
        return nil, err
    }
    if info.Mode().Perm()&0o077 != 0 {
        if !devMode {
            return nil, fmt.Errorf("key file %s permissions %o are too permissive — refusing to load",
                path, info.Mode().Perm())
        }
        log.Warnf("DEVELOPMENT MODE: loading key file %s despite permissive permissions %o", path, info.Mode().Perm())
    }
    return os.ReadFile(path)
}
```

```python
# CORRECT: the socket is created inside a directory whose own permissions
# (0o700, owner-only) already deny access to anyone but the owning user, AND
# an umask restrictive enough for the socket file itself is applied around
# the bind call. `add_insecure_port` creates and binds the socket file
# synchronously at call time — a chmod() issued only AFTER that call still
# leaves a real (if brief) TOCTOU window where the file exists with default
# umask permissions, so the fix must make the file's permissions correct AT
# CREATION TIME, not just after it. The directory permission is the primary
# guarantee; the chmod afterward is defense-in-depth for the file itself,
# not a substitute for it.
socket_dir = os.path.dirname(socket_path)
os.makedirs(socket_dir, mode=0o700, exist_ok=True)
os.chmod(socket_dir, 0o700)  # Enforce even if the directory already existed

old_umask = os.umask(0o117)  # Restrict newly created files to 0o660 from the instant of creation
try:
    server.add_insecure_port(f"unix://{socket_path}")
finally:
    os.umask(old_umask)  # Restore — do not leave the process-wide umask narrowed

try:
    os.chmod(socket_path, 0o660)  # Defense-in-depth: assert the exact mode explicitly
except OSError as e:
    logger.error("Failed to set socket permissions on %s: %s", socket_path, e)
    raise  # Abort startup — do not serve on a socket with unverified permissions
server.start()
```

> **Verification Checklist before outputting code:**
> * Does a permission check on a security-critical file or socket log a warning but still proceed when the check fails? (If yes, make it a hard failure, with at most a narrowly-scoped, off-by-default dev opt-out.)
> * Is a newly created Unix domain socket's restrictive mode established AT CREATION TIME (via a narrowed `umask` around the bind call and/or a `0o700` parent directory), not merely `chmod`'d after the fact? (A `chmod` issued only after `add_insecure_port`/`bind` leaves a real TOCTOU window — the socket exists under default umask permissions from the moment it's created until the `chmod` call lands.)
> * Is the permission check re-verified periodically at runtime for long-lived processes, or only once at startup? (A file's permissions can change after a process has already loaded it — consider a periodic re-check for high-value key material.)
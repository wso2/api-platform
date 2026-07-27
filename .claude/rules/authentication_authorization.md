# Go Authentication and Authorization Rules

Standards and patterns for ensuring secure authentication, authorization, token verification, and multi-tenant isolation across all Go services.

---

## GO-AUTH-001: Fail-Closed Authentication — Critical

Authentication must always fail-closed: if a check errors, execution terminates immediately and denies access. Logging an error but falling through to `next.ServeHTTP` (or omitting `return`) lets unauthenticated requests be processed as valid.

```go
// BAD: logs the failure but falls through — next.ServeHTTP runs regardless
if err := validateToken(token); err != nil {
    log.Printf("auth failed: %v", err)
}
next.ServeHTTP(w, r)

// GOOD: return terminates execution on the failure path
if err := validateToken(token); err != nil {
    w.WriteHeader(http.StatusUnauthorized)
    json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized", "message": "Invalid or expired credentials."})
    return // CORRECT — nothing after this runs
}
next.ServeHTTP(w, r)
```

---

## GO-AUTH-002: Strict Asymmetric JWT Verification — Critical

JWT verification must restrict signing methods to asymmetric algorithms (RSA/ECDSA/EdDSA) and explicitly reject `HS256`/`HS384`/`HS512` and `none`. Accepting the algorithm from the token header lets an attacker sign a token with HMAC using the server's own RSA public key as the secret, bypassing verification entirely.

```go
// BAD: accepts whatever alg the token header claims
token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) { return publicKey, nil })

// GOOD: explicit allowlist of asymmetric methods only
token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
    switch t.Method.(type) {
    case *jwt.SigningMethodRSA, *jwt.SigningMethodRSAPSS, *jwt.SigningMethodEd25519:
        return publicKey, nil
    default:
        return nil, fmt.Errorf("unexpected or forbidden signing method: %v", t.Header["alg"])
    }
})
```

---

## GO-AUTH-003: Secure Token Handling and Logging — Medium

Raw tokens/credentials/secrets must never be written to logs on failure; mask to a short prefix/suffix if correlation is needed. A full token in a log line becomes an attack surface the moment log storage is leaked or compromised.

```go
// BAD: logs the full Authorization header
log.Printf("failed to parse token %s: %v", r.Header.Get("Authorization"), err)

// GOOD: mask before logging
func maskToken(t string) string {
    if len(t) <= 8 { return "[MASKED]" }
    return t[:4] + "..." + t[len(t)-4:]
}
log.Printf("failed to parse token %s: %v", maskToken(r.Header.Get("Authorization")), err)
```

---

## GO-AUTH-004: Routing and Path Traversal Protection — High

Auth middleware must decide public-vs-protected via structured router scoping, not raw string prefix checks on `r.URL.Path`. A crafted path (`//auth/../private`) can confuse naive prefix matching into treating a restricted path as public.

```go
// BAD: string-prefix match on raw path — bypassable via traversal tricks
if strings.HasPrefix(r.URL.Path, "/public/") {
    next.ServeHTTP(w, r) // Bypasses auth
    return
}

// GOOD: structural router group scoping (Go 1.22+ ServeMux normalizes paths)
mux.HandleFunc("GET /public/", publicHandler)
protected := http.NewServeMux()
protected.HandleFunc("GET /private/", privateHandler)
mux.Handle("/private/", AuthMiddleware(protected))
```

---

## GO-AUTH-005: Multi-Tenant Isolation (Anti-Privilege Escalation) — Critical

Every tenant-scoped query/mutation must use `organization_id`/`tenant_id` pulled from verified JWT claims in request context — never from a request body/query/path parameter. Trusting a caller-supplied org ID lets a user swap resource IDs to reach another tenant's data.

```go
// BAD: org ID trusted from the query string
targetOrgID := r.URL.Query().Get("org_id")
db.Where("id = ? AND organization_id = ?", userID, targetOrgID).Delete(&User{})

// GOOD: org ID sourced only from context, injected by AuthMiddleware from the JWT
ctxOrgID, ok := r.Context().Value(OrgIDContextKey).(string)
if !ok || ctxOrgID == "" {
    http.Error(w, "Forbidden", http.StatusForbidden)
    return
}
db.Where("id = ? AND organization_id = ?", userID, ctxOrgID).Delete(&User{})
```

---

## GO-AUTH-006: HTTP Method Case-Insensitive Normalization — High

User-supplied HTTP method strings (CRD fields, OpenAPI specs, policy exception lists) must be normalized with `strings.ToUpper()` at extraction, before any comparison, map key build, or Envoy matcher construction. A stray lowercase `"get"` silently misses uppercase deny-list/scope-map keys (access-control bypass) or produces an Envoy `Exact:` matcher that never fires (silent routing failure reachable without the intended policy chain).

```go
// BAD: raw case fed into route key, deny-list, and Envoy matcher
routeKey := GenerateRouteName(string(op.Method), ...)
rdc.Routes[routeKey] = &models.Route{Method: string(op.Method)} // "get" never matches Envoy Exact

// GOOD: normalize once at extraction, use everywhere downstream
opMethod := strings.ToUpper(string(op.Method))
routeKey := GenerateRouteName(opMethod, ...)
rdc.Routes[routeKey] = &models.Route{Method: opMethod}
```

---

## GO-AUTH-007: Deny-by-Default Authorization on Admin/System/Internal REST APIs — Critical

Every admin/system/internal endpoint must perform an explicit, per-endpoint scope/role check inside the handler — network placement and JWT algorithm allowlisting (GO-AUTH-002) authenticate the caller but do not prove they're *entitled* to this specific privileged operation. This is the recurring root cause behind algorithm-confusion bypasses, self-registered users reaching admin APIs via a shared Key Manager, and DCR flows issuing over-broad tokens.

```go
// BAD: only checks that a token exists/is valid, never the required scope
claims, ok := r.Context().Value(ClaimsContextKey).(*Claims)
if !ok { http.Error(w, "Unauthorized", http.StatusUnauthorized); return }
deleteTenant(r.URL.Query().Get("tenant_id")) // any authenticated caller can reach this

// GOOD: explicit scope check, deny-by-default; generic 403 (no scope-probing hints)
const ScopeAdminApiKeyRotate = "internal_apikey_rotate"
if !claims.HasScope(ScopeAdminApiKeyRotate) {
    http.Error(w, "Forbidden", http.StatusForbidden)
    return
}
```

---

## GO-AUTH-008: Parameterized Queries for Administrative Data Access — Critical

Every SQL query built from request input — especially in Admin REST API handlers over `sqlx`/`database/sql` — must use `?` placeholders (or `sqlx.Named`/`sqlx.In`), never string concatenation or `fmt.Sprintf` of a request value. Authenticated SQLi is still exploitable: an admin-scoped injection reads/writes data the admin's own scope shouldn't reach or disrupts availability platform-wide. Placeholders can't parameterize identifiers (dynamic `ORDER BY`/columns) — resolve those against an explicit allowlist instead.

```go
// BAD: request value concatenated into SQL text; identifier interpolated via Sprintf
query := "SELECT id FROM tenants WHERE name LIKE '%" + name + "%'" // injectable
query = fmt.Sprintf("SELECT * FROM apis ORDER BY %s", r.URL.Query().Get("sort")) // identifier injection

// GOOD: bound placeholder for values; allowlist for identifiers
db.Select(&tenants, "SELECT id FROM tenants WHERE name LIKE ?", "%"+name+"%")
var allowedSortColumns = map[string]string{"name": "name", "created_at": "created_at"}
sortCol, ok := allowedSortColumns[r.URL.Query().Get("sort")]
if !ok { sortCol = "created_at" }
query = fmt.Sprintf("SELECT * FROM apis ORDER BY %s", sortCol) // sortCol is now a known-safe constant
```

---

## GO-AUTH-009: Token and Session Invalidation on Security-State Change — High

Logout, account lock, password reset, role change, or user/sub-org deletion must actively revoke all previously issued tokens/sessions for that identity — not merely rely on natural expiry. A token minted before the change stays valid until its own expiry unless something re-checks or revokes it. Role/status values written back must come from a closed allowlist (the GO-AUTH-008 identifier-allowlist principle applied to enum-like values, not just identifiers), and the privileged operation itself needs its own GO-AUTH-007 scope check.

```go
// BAD: locks the account but leaves already-issued tokens valid until expiry
setAccountStatus(userID, StatusLocked)
w.WriteHeader(http.StatusNoContent) // Missing: revoke active tokens/sessions

// GOOD: state change + revocation marker (token_version) commit atomically;
// authMiddleware checks token_version on every request against this same row.
tx.ExecContext(ctx, `UPDATE accounts SET status=?, token_version=token_version+1 WHERE id=?`, StatusLocked, userID)
tx.Commit()
tokenStore.RevokeAllForUser(ctx, userID) // best-effort cache cleanup; token_version is authoritative
```

---

## GO-AUTH-010: Redirect and Callback URL Allowlisting (Open Redirect Prevention) — Medium

Any server-generated redirect whose target comes from request input (`returnTo`, OAuth `redirect_uri`, SSO logout) must be validated against an explicit allowlist of exact hosts — never a substring/prefix check, which is bypassable via lookalike or userinfo-trick hosts. On rejection, fall back to a safe default rather than reflecting the rejected value back (which risks reopening the XSS surface covered in `js-output-encoding-xss.md`).

```go
// BAD: same-host substring check — bypassable ("https://evil.com/?x=trusted.example.com")
if strings.Contains(returnTo, "trusted.example.com") {
    http.Redirect(w, r, returnTo, http.StatusFound)
}

// GOOD: parsed-URL exact host match against an allowlist; reject userinfo tricks
var allowedRedirectHosts = map[string]bool{"portal.example.com": true}
u, err := url.Parse(returnTo)
if err != nil || u.User != nil || u.Scheme != "https" || !allowedRedirectHosts[u.Host] {
    returnTo = "/" // safe default — never echo the rejected value back
}
```

---

## GO-AUTH-011: Fail-Closed Startup Validation for Security-Critical Configuration — Critical

Startup must validate the *effective* result of security config, not each field in isolation — e.g. basic-auth "enabled" with zero users yields zero authenticators, which must be a startup failure, not a silent passthrough-as-authenticated. A `Disable` flag must be explicit and off by default; nothing may fall back to open/passthrough implicitly. (GO-AUTH-012 adds a third check to this same `ValidateAuthConfig` function rather than defining a second one.)

```go
// BAD: Enabled==true with empty Users yields zero authenticators, silently treated as "no auth needed"
if len(authns) == 0 {
    ctx = context.WithValue(ctx, AuthContextKey, AuthContext{Authenticated: true}) // passthrough
}

// GOOD: fail closed on the effective outcome, refuse to start otherwise
func ValidateAuthConfig(cfg AuthConfig) error {
    if cfg.Disable { return nil } // explicit, off-by-default opt-out
    if cfg.BasicAuth.Enabled && len(cfg.BasicAuth.Users) == 0 {
        return fmt.Errorf("basic auth enabled but no users configured — refusing to start")
    }
    if !cfg.BasicAuth.Enabled && !cfg.IDP.Enabled {
        return fmt.Errorf("no authentication method configured — set auth.disable=true if intentional")
    }
    return nil
}
// main(): log.Fatalf on ValidateAuthConfig error — refuse to start
```

---

## GO-AUTH-012: JWT Audience and Issuer Must Be Explicitly Validated — High

Every JWT verification path must enforce expected `aud`/`iss` at the parser-library level (`jwt.WithAudience`, `jwt.WithIssuer`), and IDP-based auth config must require a non-empty expected audience at startup. A token correctly signed by a trusted IDP but minted for a *different* client still passes signature/expiry checks without an audience check — collapsing "this IDP is trusted" into "any token from it is valid for me." This check is added to the same `ValidateAuthConfig` from GO-AUTH-011, not a duplicate function.

```go
// BAD: verifies signature/expiry but never checks aud — any token from the IDP is accepted
jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) { return idpPublicKey, nil })

// GOOD: audience/issuer enforced by the library; added to GO-AUTH-011's ValidateAuthConfig
if cfg.IDP.Enabled && cfg.IDP.ExpectedAudience == "" {
    return fmt.Errorf("idp auth enabled but no expected audience configured — refusing to start")
}
jwt.ParseWithClaims(tokenStr, claims, keyFunc,
    jwt.WithValidMethods([]string{"RS256", "ES256", "EdDSA"}), // GO-AUTH-002 asymmetric-only
    jwt.WithAudience(cfg.IDP.ExpectedAudience),
    jwt.WithIssuer(cfg.IDP.Issuer),
)
```

---

## GO-AUTH-013: Admin, Debug, and Metrics Interfaces Require Real Authentication, Not Merely IP Allowlisting — Critical

Admin/debug/metrics endpoints (config dumps, runtime state) must sit behind the same auth/authz stack as management APIs (GO-AUTH-007) — an IP allowlist is defense-in-depth only, must never default to `["*"]`, and a wildcard must require an explicit off-by-default opt-in. IP checks are also routinely defeated inside a cluster (pod-to-pod traffic, a compromised sidecar).

```go
// BAD: default allow-all IP list, no authentication at all
AllowedIPs: []string{"*"} // functionally unauthenticated from day one

// GOOD: loopback-only default; real AuthMiddleware/RequireScope gates the mux, IP check is extra
AllowedIPs: []string{"127.0.0.1", "::1"}
mux.Handle("/config_dump", RequireScope("admin:config:read", handler)) // GO-AUTH-007
protected := AuthMiddleware(authns)(mux)                              // GO-AUTH-011
return ipAllowlistMiddleware(cfg.AllowedIPs)(protected)               // defense-in-depth only
```

---

## GO-AUTH-014: No Default, Hardcoded, or Shipped Credentials — Critical

Packaged config must never ship a functional default username/password. Either ship with zero users (which, combined with GO-AUTH-011's fail-closed startup check, forces the operator to configure credentials) or generate a random credential on first boot, persist only its hash (`0o600`), and print the plaintext once. On restart, the persisted hash must be reloaded into `cfg.Users` — otherwise GO-AUTH-011 refuses to start on every subsequent boot.

```toml
# BAD: functional default shipped in every fresh install
[[controller.auth.basic.users]]
username = "admin"
password = "admin"
```

```go
// GOOD: generate once, persist only the hash, reload it on every later boot
if len(cfg.Users) > 0 { return nil }
if hash, err := os.ReadFile(credentialFile); err == nil {
    cfg.Users = []BasicAuthUser{{Username: "admin", PasswordHash: string(hash)}} // reload, don't skip
    return nil
}
pw, _ := generateSecurePassword(24)
hashed, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
os.WriteFile(credentialFile, hashed, 0o600)
fmt.Fprintf(os.Stderr, "Generated one-time admin password: %s\n", pw) // printed once, never persisted in plaintext
```

---

## GO-AUTH-015: Enforce Security Invariants at the Service/Data Layer, Not Only in Route Middleware — High

A cross-cutting invariant (e.g. "no mutations during immutable/maintenance mode") must live at the lowest layer every entry point funnels through — typically the service/data-access layer — not solely in HTTP middleware scoped to one router. Middleware bound to `/api/management/v0.9/*` protects nothing reached via an event handler, admin server, or future RPC surface that calls the service method directly.

```go
// BAD: invariant checked only in middleware wrapping one router — an event
// handler or admin server calling restAPIService.Update() bypasses it entirely
mux.Handle("/api/management/v0.9/", ImmutableModeMiddleware(cfg.IsImmutable)(managementRouter))

// GOOD: check lives in the service method — every caller passes through it
func (s *RestAPIService) Update(ctx context.Context, apiID string, def APIDefinition) error {
    if s.isImmutable() { return ErrImmutableModeActive } // protects every entry point
    return s.db.UpdateAPI(ctx, apiID, def)
}
```

---

## GO-AUTH-016: Verify Server/Peer-Asserted Identity Against Locally-Known Configuration — Medium

When a remote peer asserts its own identity or config value over an already-authenticated channel (e.g. an `ack` echoing `gateway_id`), compare it against the locally-held expectation before trusting it — an authenticated transport proves who signed the connection, not that every payload field matches what you expect. Per `go-network-service-hardening.md` directive 6, a mismatch must degrade this connection (close it, mark degraded, backoff) — never call `os.Exit`/`log.Fatalf` directly on a remote-asserted value, or a compromised/buggy peer could crash-loop every connected instance.

```go
// BAD: server-asserted gateway_id trusted with no comparison
c.assignedGatewayID = ack.GatewayID

// GOOD: compare to local config; fail this connection, not the process
if ack.GatewayID != c.gatewayID {
    logger.Error("control plane asserted unexpected gateway_id", "expected", c.gatewayID, "asserted", ack.GatewayID)
    c.health.degraded.Store(true)
    c.closeConnection() // reconnect/backoff loop decides what happens next — no os.Exit
    return
}
```

---

## GO-AUTH-017: Security-Relevant Route Matching Must Use Structural Route Identity, Not Heuristic String Matching — High

Whether auth/authz applies to a request must be decided by a structural match against the registered router/policy configuration — never a heuristic like "method == POST and path contains `mcp`". A heuristic has false negatives for any unanticipated request shape, and those negatives silently skip the security check with no error signal. A request in a protected namespace that fails to parse into the expected shape must be denied, not passed through.

```go
// BAD: request-shape heuristic decides whether auth applies — anything else skips it silently
if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "mcp") {
    if !isAuthenticated(r) { http.Error(w, "Forbidden", http.StatusForbidden); return }
}
next.ServeHTTP(w, r) // any other shape — no check at all

// GOOD: structural match against the router; protected-but-unparseable is a deny, not a skip
binding, matched, err := ResolveRouteBinding(r, router)
if err != nil { http.Error(w, "Forbidden", http.StatusForbidden); return } // protected namespace, bad shape
if !matched { next.ServeHTTP(w, r); return }                              // outside governed namespace
if binding.RequiresAuth && !isAuthenticated(r) { http.Error(w, "Forbidden", http.StatusForbidden); return }
```

---

## GO-AUTH-018: Fail-Closed on Security-Critical File and Socket Permission Checks — High

A too-permissive check on a security-critical file or Unix socket (master key, private key, privileged IPC socket) must fail the operation — return an error, abort startup — not merely log a warning and proceed. Relying on the ambient umask for a newly created socket is equivalent to no check at all; the restrictive mode must be established at creation time (narrowed umask around the bind call, or a `0o700` parent directory), not `chmod`'d afterward, which leaves a real TOCTOU window.

```go
// BAD: warns on overly permissive key file but still loads and uses it
if info.Mode().Perm()&0o077 != 0 {
    log.Warnf("key file %s has permissive permissions", path)
}
return os.ReadFile(path) // proceeds regardless

// GOOD: hard failure, narrow off-by-default dev opt-out
if info.Mode().Perm()&0o077 != 0 && !devMode {
    return nil, fmt.Errorf("key file %s permissions too permissive — refusing to load", path)
}
```
```python
# GOOD: restrictive mode established AT CREATION TIME via umask + 0o700 dir, not chmod'd after
os.makedirs(socket_dir, mode=0o700, exist_ok=True)
old_umask = os.umask(0o117)  # new files created 0o660 from the instant of creation
try:
    server.add_insecure_port(f"unix://{socket_path}")
finally:
    os.umask(old_umask)
os.chmod(socket_path, 0o660)  # defense-in-depth only, not the primary guarantee
```

---

> **Verification Checklist before outputting code:**
> * GO-AUTH-001: Does every auth-failure branch `return` immediately instead of falling through to `next()`?
> * GO-AUTH-002: Is JWT verification restricted to an explicit asymmetric-algorithm allowlist, rejecting `HS*`/`none`?
> * GO-AUTH-003: Is any raw token/credential passed to a logger instead of a masked form?
> * GO-AUTH-004: Is public-vs-protected routing decided by structural router scoping rather than `strings.HasPrefix` on raw path?
> * GO-AUTH-005: Does every tenant-scoped query use `organization_id`/`tenant_id` from verified context, never from request input?
> * GO-AUTH-006: Is every user-supplied HTTP method normalized with `strings.ToUpper()` before comparison/map-key use?
> * GO-AUTH-007: Does every admin/system/internal handler check a specific scope beyond "token is valid"?
> * GO-AUTH-008: Is every SQL query parameterized, with dynamic identifiers resolved via an allowlist, never interpolated?
> * GO-AUTH-009: Does a lock/role-change/deletion handler revoke tokens/sessions (e.g. bump `token_version`) as part of the same operation?
> * GO-AUTH-010: Is every redirect target validated by parsed-URL exact-host allowlist, never a substring/prefix check?
> * GO-AUTH-011: Does startup validation check the *effective* authenticator count, not just top-level enabled flags?
> * GO-AUTH-012: Does every JWT verification call enforce `aud`/`iss` via parser options, with audience required at startup?
> * GO-AUTH-013: Is an admin/debug/metrics endpoint gated by real auth, with `AllowedIPs` defaulting away from `["*"]`?
> * GO-AUTH-014: Does any shipped config set a functional default credential instead of zero users or a generated-and-hashed one?
> * GO-AUTH-015: Is a cross-cutting invariant (e.g. immutable mode) enforced in the service/data layer, not only in one router's middleware?
> * GO-AUTH-016: Is a peer-asserted identity/config value compared against local expectation, with mismatch degrading the connection (never `os.Exit`)?
> * GO-AUTH-017: Does a security gate match structurally against router/policy config rather than a method+path-substring heuristic?
> * GO-AUTH-018: Does a permission check on a critical file/socket hard-fail when too permissive, with socket mode set at creation time?

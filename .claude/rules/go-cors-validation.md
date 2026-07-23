# Rule: Go CORS Origin Validation Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing Go (`.go`) code that validates an incoming `Origin` request header against an operator-configured allowlist and reflects a value into `Access-Control-Allow-Origin` — most directly the `cors` policy in `gateway-runtime` (`policies/cors/cors.go`), but equally any other Go code implementing origin-based access control for cross-origin requests. The goal is to ensure an operator's plain-looking origin allowlist behaves exactly as they intend: matching only the origins they listed, not any string that happens to contain one of those origins as a substring.

---

## Directives

### 1. Treat a Plain Origin Entry as an Exact String, Not a Partial-Match Pattern

* **Exact-Match by Default:** An allowlist entry that looks like a literal origin (`https://app.example.com`) must be compared for exact equality — scheme and host, case-insensitive per RFC 6454 — never compiled as an unanchored regular expression and matched with `Regexp.MatchString`, which succeeds for any string that merely *contains* the literal as a substring.
* **Unanchored Regex Match Is the Vulnerability, Not a Style Choice:** `regexp.MustCompile("https://app.example.com").MatchString(origin)` matches `https://app.example.com.evil.com`, `https://evil.com/?x=https://app.example.com`, and even `https://appXexample.com` (because an unescaped `.` matches any character) — none of which the operator intended to allow when they typed a plain hostname into a config file.

### 2. If Regex Origins Are Supported at All, Require Full-String Anchoring and a Distinct Config Field

* **Separate the Two Input Shapes:** Give literal origins and regex patterns distinct configuration fields (e.g. `allowedOrigins` for exact strings, `allowedOriginPatterns` for regexes) rather than one field that silently accepts either shape and compiles everything as a pattern.
* **Anchor Every Compiled Pattern:** When a regex field is genuinely needed (e.g. to allow a set of preview-deployment subdomains), compile it wrapped as `^(?:pattern)$` — never trust an operator-supplied pattern to already be anchored. Reject (fail policy validation / `GetPolicy`) any pattern that isn't parseable as a full-string match, rather than silently degrading to a partial match.

### 3. Never Combine a Broad/Unanchored Origin Match With `Access-Control-Allow-Credentials: true`

* **Credentials Mode Is the High-Impact Case:** Setting `Access-Control-Allow-Credentials: true` alongside a matched origin means the matching origin's page can read authenticated responses (cookies, session-bound data) from this API — turning a CORS misconfiguration into a direct cross-tenant data-exposure path. Reject at config-validation time any policy that sets `allowCredentials: true` together with a wildcard origin, an unanchored pattern, or any allowlist entry that isn't a single exact-match origin.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```go
// BAD: every allowedOrigins entry is compiled as an UNANCHORED regexp and
// matched with MatchString — a plain-looking origin behaves as a partial-match
// pattern rather than an exact string, so it can match more than the operator intended.
type CORSPolicy struct {
    AllowedOrigins   []string
    AllowCredentials bool
}

func (p *CORSPolicy) isOriginAllowed(origin string) bool {
    for _, pattern := range p.AllowedOrigins {
        re, err := regexp.Compile(pattern) // No anchoring, no escaping
        if err != nil {
            continue
        }
        if re.MatchString(origin) { // Partial match anywhere in the string
            return true
        }
    }
    return false
}

func (p *CORSPolicy) ApplyHeaders(w http.ResponseWriter, origin string) {
    if p.isOriginAllowed(origin) {
        w.Header().Set("Access-Control-Allow-Origin", origin)
        if p.AllowCredentials {
            w.Header().Set("Access-Control-Allow-Credentials", "true") // Compounds the bypass
        }
    }
}
```

### Best Practice (What to Generate)

```go
// GOOD: literal origins are compared for exact equality; regex patterns are a
// distinct, explicitly-named field and are always anchored before compiling;
// credentials mode is rejected outright for any non-exact-match policy.
type CORSPolicy struct {
    AllowedOrigins        []string // Exact-match origins, e.g. "https://app.example.com"
    AllowedOriginPatterns []string // Regex patterns, explicitly named, always anchored
    AllowCredentials      bool
}

func (p *CORSPolicy) Validate() error {
    if p.AllowCredentials {
        if len(p.AllowedOriginPatterns) > 0 {
            return fmt.Errorf("allowCredentials cannot be combined with allowedOriginPatterns")
        }
        for _, o := range p.AllowedOrigins {
            if o == "*" {
                return fmt.Errorf("allowCredentials cannot be combined with a wildcard origin")
            }
        }
    }
    for _, pattern := range p.AllowedOriginPatterns {
        if _, err := regexp.Compile("^(?:" + pattern + ")$"); err != nil {
            return fmt.Errorf("invalid origin pattern %q: %w", pattern, err)
        }
    }
    return nil
}

func (p *CORSPolicy) hasWildcardOrigin() bool {
    for _, o := range p.AllowedOrigins {
        if o == "*" {
            return true
        }
    }
    return false
}

func (p *CORSPolicy) isOriginAllowed(origin string) bool {
    normalizedOrigin := strings.ToLower(origin)
    for _, exact := range p.AllowedOrigins {
        // "*" is a true wildcard — Validate() guarantees it only ever coexists
        // with AllowCredentials == false, so treating it as "match any origin"
        // here cannot leak credentialed responses.
        if exact == "*" || strings.EqualFold(exact, normalizedOrigin) { // Wildcard, or exact match — no partial-match surface at all
            return true
        }
    }
    for _, pattern := range p.AllowedOriginPatterns {
        re := regexp.MustCompile("^(?:" + pattern + ")$") // Anchored — cannot partial-match
        if re.MatchString(origin) {
            return true
        }
    }
    return false
}

func (p *CORSPolicy) ApplyHeaders(w http.ResponseWriter, origin string) {
    if !p.isOriginAllowed(origin) {
        return
    }
    if p.hasWildcardOrigin() {
        // A true wildcard always reflects "*" itself, never the specific
        // origin, and needs no Vary — the response is identical for every
        // caller. Safe without a credentials check here because Validate()
        // already rejects any policy combining a wildcard with AllowCredentials.
        w.Header().Set("Access-Control-Allow-Origin", "*")
        return
    }
    w.Header().Set("Access-Control-Allow-Origin", origin)
    w.Header().Set("Vary", "Origin") // Prevent shared-cache origin confusion across distinct callers
    if p.AllowCredentials {
        w.Header().Set("Access-Control-Allow-Credentials", "true")
    }
}
```

```go
// cors_test.go — GOOD: regression coverage for the exact substrings that
// defeated the unanchored-regex version, so a future refactor can't reopen it.
func TestIsOriginAllowed_RejectsPartialMatchBypasses(t *testing.T) {
    p := &CORSPolicy{AllowedOrigins: []string{"https://app.example.com"}}
    bypasses := []string{
        "https://app.example.com.evil.com",
        "https://evil.com/?x=https://app.example.com",
        "https://appXexample.com",
    }
    for _, origin := range bypasses {
        if p.isOriginAllowed(origin) {
            t.Errorf("origin %q should NOT be allowed by an exact-match allowlist", origin)
        }
    }
    if !p.isOriginAllowed("https://app.example.com") {
        t.Error("the exact configured origin should be allowed")
    }
}
```

---

> **Verification Checklist before outputting code:**
> * Is a plain, literal-looking origin entry compiled as a regular expression and matched with `MatchString` rather than compared via exact string equality? (If yes, switch to exact comparison for the common case.)
> * If regex-style origin matching is supported, is every pattern anchored (`^(?:pattern)$`) before compilation, and rejected at config-validation time if it isn't parseable as a full-string match? (Unanchored patterns are exactly as bypassable as no anchoring at all.)
> * Does `allowCredentials: true` coexist with a wildcard origin, a regex pattern, or anything other than a single exact-match origin? (If yes, reject the policy at validation time — this combination is the highest-impact CORS misconfiguration.)
> * Is there a regression test asserting that a configured origin does NOT match superstring/substring variations of itself (e.g. `origin.evil.com`, `evil.com/?x=origin`)? (If not, add one — this is the exact failure mode this rule exists to prevent.)

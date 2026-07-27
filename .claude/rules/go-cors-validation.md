# Rule: Go CORS Origin Validation Standards

## Context & Scope

Apply whenever writing, refactoring, or reviewing Go (`.go`) code that validates an incoming `Origin` header against an operator-configured allowlist and reflects a value into `Access-Control-Allow-Origin` — most directly the `cors` policy in `gateway-runtime` (`policies/cors/cors.go`), but any Go origin-based access control. The goal: an operator's plain-looking origin allowlist matches only the origins they listed, not any string that happens to contain one as a substring.

## Directives

1. **Exact-match plain origins, never unanchored regex.** A literal-looking allowlist entry (`https://app.example.com`) must be compared for exact equality (case-insensitive per RFC 6454) — never compiled as an unanchored regex matched via `MatchString`. `regexp.MustCompile("https://app.example.com").MatchString(origin)` matches `https://app.example.com.evil.com`, `https://evil.com/?x=https://app.example.com`, and even `https://appXexample.com` (unescaped `.`) — none intended by an operator who typed a plain hostname.
2. **Regex origins need their own field and full-string anchoring.** Give literal origins and regex patterns distinct config fields (`allowedOrigins` vs `allowedOriginPatterns`) rather than one field that silently compiles everything as a pattern. When a pattern is genuinely needed, always compile it wrapped as `^(?:pattern)$` and reject (at policy validation time) any pattern that isn't parseable as a full-string match — never trust an operator-supplied pattern to already be anchored.
3. **`allowCredentials: true` requires a single exact-match origin.** Combining it with a matched origin lets that origin's page read authenticated responses (cookies, session data) — a CORS misconfig becomes cross-tenant data exposure. Reject at config-validation time any policy pairing `allowCredentials: true` with a wildcard origin, a regex pattern, or anything other than one exact-match origin.

## Example

```go
// BAD: plain origins compiled as UNANCHORED regexp + MatchString — partial match anywhere in the string.
func (p *CORSPolicy) isOriginAllowed(origin string) bool {
    for _, pattern := range p.AllowedOrigins {
        re, err := regexp.Compile(pattern) // no anchoring, no escaping
        if err == nil && re.MatchString(origin) {
            return true // matches "https://app.example.com.evil.com" too
        }
    }
    return false
}

// GOOD: exact match for literals; anchored regex in a separate field; credentials
// mode rejected outright unless the policy is a single exact-match origin.
type CORSPolicy struct {
    AllowedOrigins        []string // exact-match, e.g. "https://app.example.com"
    AllowedOriginPatterns []string // regex, explicitly named, always anchored
    AllowCredentials      bool
}

func (p *CORSPolicy) Validate() error {
    if p.AllowCredentials && (len(p.AllowedOriginPatterns) > 0 || slices.Contains(p.AllowedOrigins, "*")) {
        return fmt.Errorf("allowCredentials cannot be combined with a wildcard or pattern origin")
    }
    for _, pattern := range p.AllowedOriginPatterns {
        if _, err := regexp.Compile("^(?:" + pattern + ")$"); err != nil {
            return fmt.Errorf("invalid origin pattern %q: %w", pattern, err)
        }
    }
    return nil
}

func (p *CORSPolicy) isOriginAllowed(origin string) bool {
    for _, exact := range p.AllowedOrigins {
        if exact == "*" || strings.EqualFold(exact, origin) { // wildcard or exact — no partial-match surface
            return true
        }
    }
    for _, pattern := range p.AllowedOriginPatterns {
        if regexp.MustCompile("^(?:" + pattern + ")$").MatchString(origin) { // anchored — cannot partial-match
            return true
        }
    }
    return false
}
```

> **Verification Checklist before outputting code:**
> * Is a plain, literal-looking origin matched via `regexp.MatchString` instead of exact string equality? (Switch to exact comparison.)
> * If regex origin matching exists, is every pattern anchored (`^(?:pattern)$`) and validated at config time, in its own field separate from exact origins?
> * Does `allowCredentials: true` coexist with a wildcard, a pattern, or more than one non-exact origin? (Reject at validation time.)
> * Is there a regression test asserting a configured origin does NOT match substring/superstring variants (`origin.evil.com`, `evil.com/?x=origin`)? (Add one if missing.)

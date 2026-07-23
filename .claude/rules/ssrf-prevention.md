# Rule: Go Server-Side Request Forgery (SSRF) Prevention Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing Go (`.go`) code that causes the server to make an outbound network request whose destination, in whole or in part, is derived from user or tenant input. This includes WebSub/webhook callback delivery (`event-gateway/gateway-runtime/internal/subscription`, `event-gateway/gateway-runtime/internal/connectors/receiver/websub`), backend/upstream target resolution in the gateway — including the **RestApi, Mcp, and LlmProvider/LlmProxy** upstream validators (`gateway-controller/pkg/config/api_validator.go`, `mcp_validator.go`, `llm_validator.go`) — "try it" / test-invoke style features, URL-based import of specs (OpenAPI/WSDL), any header-driven request redirection (e.g. WS-Addressing `ReplyTo`/`FaultTo`-style headers), and **URLs extracted from proxied or LLM-generated/LLM-request content** (e.g. a `url-guardrail`-style policy that dereferences links found inside model input/output). The goal is to prevent an attacker from using the server as a proxy to reach internal-only network resources, cloud metadata endpoints, or other services the attacker could not otherwise reach directly.

---

## Directives

### 1. Treat Every User-Supplied URL as Untrusted Until Validated

* **No Direct Dial:** Never pass a URL sourced from a request body, header, query parameter, or tenant configuration (e.g. a WebSub `CallbackURL`, an imported spec URL, a proxy target override) directly into `http.Client.Get/Post/Do` without validation.
* **Scheme Allowlist:** Only allow `https://` (and `http://` only where explicitly required, e.g. local dev callback testing) — reject `file://`, `gopher://`, `ftp://`, `dict://`, and any other scheme.
* **Resolve Before Trust:** Resolve the hostname to an IP *before* connecting, and validate the resolved IP — not just the hostname string. A hostname passing validation can still resolve to an internal address via DNS rebinding.

### 2. Block Private, Loopback, Link-Local, and Metadata Addresses

* **Deny-List Reserved Ranges:** Reject destinations resolving to RFC 1918 private ranges (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`), loopback (`127.0.0.0/8`, `::1`), link-local (`169.254.0.0/16`, `fe80::/10`), and the `169.254.169.254` / `fd00:ec2::254` cloud metadata addresses — regardless of cloud provider.
* **Apply at Dial Time, Not Just Lookup Time:** Enforce the check inside a custom `net.Dialer.Control` (or a `DialContext` wrapper) so that a redirect or a second DNS answer during the TCP handshake cannot bypass an earlier one-time check (TOCTOU / DNS rebinding).
* **No Redirect Follow-Through Without Re-Validation:** If the HTTP client follows redirects, re-run the same destination validation on every redirect target. Prefer disabling automatic redirects (`CheckRedirect` returning `http.ErrUseLastResponse`) and validating explicitly before following one hop at a time, up to a small maximum.

### 3. WebSub / Webhook Callback URLs — Validate at Registration, Re-Validate at Delivery

* **Validate on Subscribe:** When a `CallbackURL` is registered (subscription create/renew), run it through the same scheme + private-range validation before persisting it to the subscription store.
* **Re-Validate on Every Delivery:** Do not trust that a `CallbackURL` validated at subscription time is still safe at delivery time — tenant DNS records can change between registration and delivery (DNS rebinding across time). Re-resolve and re-check immediately before each outbound delivery attempt in the delivery path.
* **Bound Delivery Requests:** Apply a short connect/read timeout and a response size cap to every webhook delivery request; a malicious or compromised callback endpoint must not be able to hang a delivery worker or exhaust memory.

### 4. Never Let Response Headers or Redirect Targets Steer a Second Request

* **No Header-Driven Re-Dispatch:** Do not implement WS-Addressing-style (`ReplyTo`, `FaultTo`) or similar "reply to this other address" semantics by dialing a second address taken from request headers/body without applying the exact same destination validation as directive 1–2. If such a feature is not explicitly required, reject the header/field outright rather than silently honoring it.
* **Sanitize Before Forwarding, Not After:** When proxying or forwarding a request to a backend/upstream, validate the resolved backend target against the configured allowlist of registered backends for that API — never allow a per-request override of the backend host unless that override is itself validated against the same allowlist.

### 5. Configuration, Not Hardcoding

* **Externalize the Allowlist/Denylist:** Source the private-range denylist defaults from code (safe built-in defaults per directive 2), but allow operators to extend it via configuration for environment-specific ranges (e.g. an internal VPC CIDR). Never allow per-tenant configuration to *widen* the denylist exceptions without an explicit administrative opt-in flag that is off by default.
* **Log and Reject, Don't Silently Drop:** On rejection, return a generic `400 Bad Request` / `422 Unprocessable Entity` to the caller (never reveal *why* — i.e. don't echo back "resolved to private IP" which helps an attacker map internal topology) while logging the actual resolved IP internally for audit.

### 6. One Shared Validation Helper Across Every Upstream/Backend Validator

* **No Duplicate, Drifting Implementations:** When more than one code path validates an upstream/backend URL — a REST API backend, an MCP upstream, an LLM provider/proxy upstream, a WebSub callback — implement the private-IP/metadata/scheme checks exactly **once** in a shared helper (e.g. a `netguard`-style package) and call it from every validator. Independent, per-feature reimplementations reliably drift: the first path built gets the full private-IP/metadata denylist, and a similar validator added later for a newer feature (MCP, LLM proxy) performs only a syntactic `url.Parse` check because the shared logic wasn't extracted or reused.
* **Audit Every Validator When Adding a New Upstream Kind:** Before shipping a new kind of user/tenant-configurable upstream (a new connector type, a new proxy mode), grep for every existing `validateUpstream*`/`*_validator.go` function and confirm the new one calls the same shared helper — do not write a new bespoke check "because this one is simpler."
* **Test Every Validator, Not Just the First:** Add a validator unit test per upstream kind (REST, MCP, LLM, WebSub) asserting a rejection (400/422) for loopback, RFC 1918, link-local, and cloud-metadata targets. The presence of a passing test for one validator is not evidence that sibling validators enforce the same policy.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```go
// BAD: Dials a user-supplied CallbackURL directly — classic SSRF.
func DeliverWebhook(sub *Subscription, payload []byte) error {
    resp, err := http.Post(sub.CallbackURL, "application/json", bytes.NewReader(payload))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    return nil
}

// BAD: Validates the hostname string but dials whatever it resolves to later —
// vulnerable to DNS rebinding between check and connect.
func ValidateCallbackURL(raw string) error {
    u, err := url.Parse(raw)
    if err != nil {
        return err
    }
    if strings.Contains(u.Host, "localhost") { // String-only check, no IP-level validation
        return fmt.Errorf("invalid host")
    }
    return nil // No IP-level check, no re-check at dial time
}

// BAD: Honors an attacker-controlled "replyTo" field to make a second, unvalidated request.
func HandleReplyTo(req *IncomingRequest) {
    if req.ReplyTo != "" {
        http.Get(req.ReplyTo) // Header/body-driven re-dispatch, no validation at all
    }
}
```

### Best Practice (What to Generate)

```go
// netguard/dialer.go — GOOD: destination validation enforced at dial time,
// immune to DNS rebinding between "check" and "connect".
package netguard

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "syscall"
    "time"
)

var deniedCIDRs = mustParseCIDRs(
    "127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
    "169.254.0.0/16", "0.0.0.0/8", "::1/128", "fe80::/10", "fd00:ec2::254/128",
    "fc00::/7", // IPv6 unique-local addresses — the RFC 1918 analogue for IPv6
    "::/128", // IPv6 unspecified address
)

func mustParseCIDRs(cidrs ...string) []*net.IPNet {
    nets := make([]*net.IPNet, 0, len(cidrs))
    for _, c := range cidrs {
        _, n, err := net.ParseCIDR(c)
        if err != nil {
            panic(fmt.Sprintf("invalid built-in CIDR %q: %v", c, err))
        }
        nets = append(nets, n)
    }
    return nets
}

func isDenied(ip net.IP) bool {
    for _, n := range deniedCIDRs {
        if n.Contains(ip) {
            return true
        }
    }
    return false
}

// SafeDialer returns a *net.Dialer whose Control hook rejects connections to
// private/loopback/link-local/metadata addresses at the moment of connecting —
// not merely at an earlier "looks-up-and-forgets" validation step.
func SafeDialer() *net.Dialer {
    return &net.Dialer{
        Timeout: 5 * time.Second,
        Control: func(_, address string, _ syscall.RawConn) error {
            host, _, err := net.SplitHostPort(address)
            if err != nil {
                return err
            }
            ip := net.ParseIP(host)
            if ip == nil {
                return fmt.Errorf("refusing to dial unresolved host")
            }
            if isDenied(ip) {
                return fmt.Errorf("destination is not allowed")
            }
            return nil
        },
    }
}

// SafeHTTPClient builds an http.Client that validates every dial target and
// never follows a redirect without re-validation. Per directive 2, disabling
// redirects outright is preferred over trusting Transport.DialContext alone —
// DialContext only re-validates the resolved IP, not the scheme, so a redirect
// to a disallowed scheme (e.g. downgrading https to http) would otherwise pass.
func SafeHTTPClient() *http.Client {
    transport := &http.Transport{DialContext: SafeDialer().DialContext}
    return &http.Client{
        Transport: transport,
        Timeout:   10 * time.Second,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse // Never auto-follow; caller validates and follows one hop at a time if required.
        },
    }
}

// subscription/callback_validation.go — GOOD: scheme + shape validation at
// subscribe time. Combined with SafeHTTPClient at delivery time (directive 3:
// validate on subscribe AND re-validate on every delivery).
func ValidateCallbackURL(raw string) (*url.URL, error) {
    u, err := url.Parse(raw)
    if err != nil {
        return nil, fmt.Errorf("invalid callback URL")
    }
    if u.Scheme != "https" {
        return nil, fmt.Errorf("callback URL must use https")
    }
    if u.Host == "" {
        return nil, fmt.Errorf("callback URL must specify a host")
    }
    return u, nil // IP-level enforcement happens at dial time via SafeDialer, not here
}

// delivery.go — GOOD: bounded, validated delivery. No header/body field is ever
// used to pick a second, unvalidated destination (directive 4).
func DeliverWebhook(ctx context.Context, client *http.Client, sub *Subscription, payload []byte) error {
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.CallbackURL, bytes.NewReader(payload))
    if err != nil {
        return fmt.Errorf("build webhook request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := client.Do(req) // client is SafeHTTPClient() — validated at dial time
    if err != nil {
        // Log the real reason internally; do not leak resolved-IP details to callers of this delivery.
        return fmt.Errorf("webhook delivery failed: %w", err)
    }
    defer resp.Body.Close()

    limited := io.LimitReader(resp.Body, 1<<20) // Cap response body read — never unbounded
    _, _ = io.Copy(io.Discard, limited)
    return nil
}
```

---

> **Verification Checklist before outputting code:**
> * Does any `http.Client`/`http.Get`/`http.Post`/`http.Do` call use a destination string that originated from request input, headers, or tenant config? (If yes, it must go through a client built on `SafeDialer`/`SafeHTTPClient`, never the default `http.DefaultClient`.)
> * Is destination validation performed only once, at parse/registration time, with no re-check at actual dial/redirect time? (If yes, add dial-time enforcement — a hostname-string check alone is bypassable via DNS rebinding.)
> * Does any code path honor a header- or body-supplied "reply to" / "callback" / "redirect to" address to make a second outbound request without validation? (If yes, validate it identically to directive 1–2, or reject the field outright.)
> * Are webhook/callback deliveries bounded by a timeout and a response-size limit? (If not, add both — an unbounded read/hang is a resource-exhaustion vector on top of SSRF.)
> * Does an error response to the caller reveal the resolved internal IP or *why* a destination was rejected? (If yes, generalize the client-facing message and log the specific reason internally only.)
> * Is there more than one upstream/backend validator (REST, MCP, LLM, WebSub) in this codebase, and do they all call the same shared private-IP/metadata-denylist helper? (If a new validator performs only a syntactic `url.Parse` check while an existing sibling validator does full IP-level validation, that is the exact drift this rule exists to prevent — see directive 6.)
> * Does a feature dereference URLs extracted from proxied or model-generated content (not just from a request field/header)? (Apply the same dial-time validation — content-derived URLs are exactly as untrusted as a request parameter.)

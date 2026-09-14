# Rule: Go Server-Side Request Forgery (SSRF) Prevention Standards

## Context & Scope

Apply whenever Go code makes an outbound request whose destination is derived, even partially, from user or operator-configured input: WebSub/webhook callback delivery (`event-gateway/gateway-runtime/internal/subscription`, `.../connectors/receiver/websub`), the gateway's RestApi/Mcp/LlmProvider-LlmProxy upstream validators (`gateway-controller/pkg/config/api_validator.go`, `mcp_validator.go`, `llm_validator.go`), "try it" test-invoke features, URL-based spec import, header-driven redirection (WS-Addressing `ReplyTo`/`FaultTo`), and URLs extracted from proxied or LLM-generated content. Goal: prevent the server being used as a proxy to reach internal-only hosts, cloud metadata endpoints, or otherwise-unreachable services.

## Directives

1. **Every user-supplied URL is untrusted until validated.** Never pass a request/header/operator-configured URL directly into `http.Client.Get/Post/Do`. Allow both `http://` and `https://` generally; reject `file://`, `gopher://`, `ftp://`, `dict://`. Resolve the hostname to an IP *before* connecting and validate the IP itself — a hostname can pass string validation yet resolve to an internal address via DNS rebinding.
2. **Block private/loopback/link-local/metadata ranges, enforced at dial time.** Deny RFC 1918 ranges, loopback (`127.0.0.0/8`, `::1`), link-local (`169.254.0.0/16`, `fe80::/10`), and metadata addresses (`169.254.169.254`, `fd00:ec2::254`). Enforce via a custom `net.Dialer.Control`/`DialContext`, not a one-time pre-check, so a redirect or second DNS answer mid-handshake can't bypass it (TOCTOU/rebinding). Prefer `CheckRedirect` returning `http.ErrUseLastResponse` over auto-following, and re-validate explicitly one hop at a time if a redirect must be followed.
3. **WebSub/webhook callbacks: validate at registration AND re-validate at delivery.** A `CallbackURL` validated at subscribe time can point somewhere unsafe by delivery time (the target's DNS can change between registration and delivery) — re-resolve and re-check immediately before every delivery attempt, not just once at registration. Bound every delivery with a short connect/read timeout and a response-size cap so a malicious/compromised endpoint can't hang a worker or exhaust memory.
4. **Never let response headers or redirect targets steer a second request.** Don't implement "reply to this other address" semantics (WS-Addressing `ReplyTo`/`FaultTo` or similar) by dialing a header/body-supplied address without the exact same validation as directives 1–2; reject the field outright if the feature isn't explicitly required. When proxying to a backend, validate the resolved target against that API's configured backend allowlist — never allow a per-request host override unless it's checked against the same allowlist.
5. **Config, not hardcoding — and generic rejection.** Ship the denylist as safe built-in defaults, letting operators extend it for environment-specific ranges (e.g. internal VPC CIDRs); never let configuration widen exceptions without an explicit, off-by-default admin opt-in. On rejection return a generic `400`/`422` that never reveals *why* (e.g. "resolved to private IP" maps internal topology for the attacker) — log the real resolved IP internally only.
6. **One shared validation helper across every upstream/backend validator.** Implement the private-IP/metadata/scheme checks exactly once (e.g. a `netguard` package) and call it from every validator — REST, MCP, LLM provider/proxy, WebSub. Per-feature reimplementations drift: the first path gets full IP-level validation, a later one (MCP, LLM proxy) often ends up with only a syntactic `url.Parse` check. When adding a new upstream kind, grep existing `validateUpstream*`/`*_validator.go` functions and confirm the new one reuses the shared helper; add a rejection unit test per upstream kind (loopback, RFC 1918, link-local, metadata) rather than trusting one passing test to mean sibling validators enforce the same policy.
7. **No deferring a violation behind a code comment.** Never resolve a missing scheme/IP/dial-time check by adding a `// TODO`/`FIXME`-style comment and shipping the request path anyway — a comment does not validate a destination. Fix it before merging, or raise the gap explicitly for an approved exception rather than leaving it annotated in the source.

## Example

```go
// BAD: dials a user-supplied URL directly — classic SSRF, no scheme/IP checks,
// no dial-time enforcement, vulnerable to DNS rebinding between check and connect.
func DeliverWebhook(sub *Subscription, payload []byte) error {
    resp, err := http.Post(sub.CallbackURL, "application/json", bytes.NewReader(payload))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    return nil
}

// GOOD: netguard.SafeHTTPClient() validates every dial target via a custom
// Dialer.Control (checked against a denied-CIDR list covering RFC 1918,
// loopback, link-local, and metadata ranges), disables auto-redirects, and
// bounds the response — reused by every validator (REST/MCP/LLM/WebSub).
func DeliverWebhook(ctx context.Context, client *http.Client, sub *Subscription, payload []byte) error {
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.CallbackURL, bytes.NewReader(payload))
    if err != nil {
        return fmt.Errorf("build webhook request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := client.Do(req) // client == netguard.SafeHTTPClient() — validated at dial time, re-run per delivery
    if err != nil {
        return fmt.Errorf("webhook delivery failed: %w", err) // real reason logged internally, not returned to caller
    }
    defer resp.Body.Close()

    limited := io.LimitReader(resp.Body, 1<<20) // response cap — never an unbounded read
    _, _ = io.Copy(io.Discard, limited)
    return nil
}
```

> **Verification Checklist before outputting code:**
> * Does any `http.Client`/`Get`/`Post`/`Do` call dial a request/header/operator-configured URL without going through a `SafeDialer`/`SafeHTTPClient`-style client?
> * Is destination validation only a one-time parse/registration-time check, with no dial-time or per-delivery re-validation?
> * Does any code honor a header/body "reply to" or redirect-target field to fire a second, unvalidated request?
> * Are webhook/callback deliveries bounded by both a timeout and a response-size cap?
> * Does a rejection response leak the resolved IP or the specific reason, instead of a generic message with detail logged internally only?
> * Do all upstream/backend validators (REST, MCP, LLM, WebSub) call the same shared denylist helper, rather than a newer one reimplementing a thinner syntactic check?
> * Is a gap in this rule "resolved" by a `// TODO`/`FIXME`-style comment instead of an actual fix or an explicitly raised, approved exception?

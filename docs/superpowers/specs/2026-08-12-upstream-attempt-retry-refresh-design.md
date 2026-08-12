# Generic Upstream-Attempt Retry Refresh — Design

## Status

Supersedes the narrower, oauth2-generator-hardcoded approach explored in
`docs/superpowers/plans/2026-08-11-oauth2-upstream-retry-refresh.md` and its
`.superpowers/sdd/2026-08-11-oauth2-upstream-retry-refresh/` task history.
That prior effort is left as-is (its commits exist in git history but are not
reachable from current `HEAD` — the branch was externally hard-reset mid-task
per its own progress ledger). This design does not reuse its code or resume
its task sequence; it does reuse two verified technical facts it discovered
(noted inline below where relevant).

## Problem

An auth policy (oauth2-generator today; api-key-auth/jwt-auth/opaque-token-auth/
backend-jwt potentially later) attaches a cached credential to the upstream
request. If the upstream rejects it (401), the policy can purge its cache
today, but the *current* request still fails — only the *next* request
benefits from the purge. Requirement: replay the current request with a
freshly-fetched credential, transparently to the client.

## Why not native Envoy retry alone

Confirmed by reading this repo's actual kernel code
(`gateway/gateway-runtime/policy-engine/internal/kernel/extproc.go`): `ext_proc`
as deployed today is a *downstream* HTTP filter — one reactive gRPC stream per
client request, invoked once per phase. Envoy's native
`RouteAction.RetryPolicy` retries below this filter (inside Router), resending
whatever headers the single downstream-filter pass already produced. A
downstream `ext_proc` is never re-invoked per retry attempt, so it cannot
attach a new credential to a retried attempt on its own.

## Mechanism

Envoy's *upstream* HTTP filter chain — configured per-cluster via
`Cluster.TypedExtensionProtocolOptions["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"].HttpFilters`
(type confirmed present in this repo's vendored
`go-control-plane v1.37.0`) — is invoked fresh for every upstream dial
attempt, including Envoy-native retries. Envoy sets `x-envoy-attempt-count` on
each such attempt (`1` on the first, incrementing per retry) — a fact
verified during the prior effort's investigation and reused here as-is. A
policy's upstream-phase hook checks `AttemptCount > 1` and unconditionally
forces a fresh credential fetch — no response-phase hook needed on the
upstream side at all, since "was the previous attempt rejected" is implied by
"this is attempt 2+".

## Config surface

Extend the existing `resilience` block (`gateway-controller/pkg/config/api_validator.go`'s
`validateResilience`, currently `timeout`/`idleTimeout` only):

```yaml
resilience:
  retry:
    statusCodes: [401]   # required, non-empty
    numRetries: 1        # default 1
```

Same API-level/operation-level precedence as the existing timeout fields.
Generates a native `RouteAction.RetryPolicy{RetriableStatusCodes, RetryOn:
"retriable-status-codes", NumRetries}` — this part needs no new mechanism.

## Cluster attachment and the shared-cluster hazard

Clusters in this codebase are deduped purely by upstream `host+scheme`
(`gateway-controller/pkg/xds/translator.go`'s `sanitizeClusterName`,
`resolveUpstreamCluster`) — confirmed by direct reading, independent of the
prior effort's own Task 0 finding, which reached the same conclusion. Two
unrelated APIs sharing a backend host:port share one Envoy cluster. The new
upstream `ext_proc` filter is attached to a cluster if **any** route feeding
into it has `resilience.retry` configured — OR'd across every API sharing
that cluster. APIs sharing the cluster without retry configured are
unaffected: the chain executor (see below) simply never invokes the
upstream-phase hook for their routes, even though the filter is technically
present on the shared cluster.

This upstream filter's `GrpcService` target is a second, small internal Envoy
cluster pointed at policy-engine's new second port, mirroring the existing
`createPolicyEngineCluster()` pattern used for the main downstream `ext_proc`
service (same internal-cluster conventions: `STRICT_DNS`,
`Http2ProtocolOptions`, no TLS on loopback). This new internal cluster is
created once, unconditionally, independent of whether any data-plane cluster
currently needs the filter attached.

## SDK: generic `UpstreamAttemptPolicy` interface

`sdk/core/policy/v1alpha2`:

```go
// UpstreamAttemptContext is passed to UpstreamAttemptPolicy.OnUpstreamAttemptRequestHeaders.
// Unlike every other context in this package, it is not scoped to one client
// request — it fires once per individual upstream dial attempt (including
// Envoy-native retries), because it runs in Envoy's per-cluster upstream HTTP
// filter chain rather than the per-route listener chain every other phase uses.
type UpstreamAttemptContext struct {
    *SharedContext

    // AttemptCount is Envoy's x-envoy-attempt-count for this dial, starting at
    // 1. A missing/unparseable header is treated as 1 (fail toward "behave
    // like the first attempt", never toward unconditional refresh).
    AttemptCount int

    Headers *Headers // this attempt's outgoing request headers, mutable via the returned action
}

// UpstreamAttemptAction is the sealed return type. Deliberately one variant:
// this phase runs after routing/auth are already resolved and mid-retry-loop
// inside Envoy's router filter — there is no sensible "reject this request"
// here, only "optionally change headers for this one attempt."
type UpstreamAttemptAction interface { isUpstreamAttemptAction() }

type UpstreamAttemptHeaderModifications struct {
    HeadersToSet map[string]string // empty/nil is a valid no-op (e.g. AttemptCount == 1)
}

type UpstreamAttemptPolicy interface {
    OnUpstreamAttemptRequestHeaders(ctx context.Context, actx *UpstreamAttemptContext) UpstreamAttemptAction
}
```

Any policy can implement this later; oauth2-generator is the first consumer.

## Policy discovery: type assertion, not name-hardcoding

The chain executor discovers eligibility via `policy.(UpstreamAttemptPolicy)`
type assertion — never a hardcoded policy-name string. This is the one
concrete fix relative to the prior effort's own retrospective note (it
hardcoded `"oauth2-generator"` in three separate places — validator,
translator, registry — and its own final ledger entry flagged this as the
thing to generalize "if/when a second consumer appears"). Here, a second
consumer needs zero additional wiring in gateway-controller or policy-engine:
implement the interface, and existing per-route `resilience.retry` config is
the only opt-in signal required.

## policy-engine: second `ext_proc` gRPC endpoint, same process

A new, small `ExternalProcessorServer` — request-headers phase only, no body
phases, no analytics/tracing duplication — listening on a second
socket/port, alongside the existing downstream one
(`internal/kernel/extproc.go`). It resolves route → policy chain via the
*same* in-memory `registry.PolicyChain` the downstream server already builds
and caches per route (same lookup mechanism, e.g. the existing
`ExtProcRequestAttributeRouteName` request attribute is available to either
attachment point since both use the same underlying `ExternalProcessor` proto
message). Reusing the same chain/policy instances — not a parallel
registry — is what makes a policy's cache/state (e.g. oauth2-generator's
token cache) naturally shared between the two entry points with zero
duplication.

Per request: resolve chain → for each policy, if it implements
`UpstreamAttemptPolicy`, invoke it with the parsed `x-envoy-attempt-count`.
Everything else in the chain (rate limiting, analytics, transforms) doesn't
implement this interface, so this endpoint is inherently a no-op for them.

Hardening (per `go-network-service-hardening.md`): explicit
`MaxRecvMsgSize`/`MaxSendMsgSize`/`MaxConcurrentStreams` on this new gRPC
server, sized off its headers-only message shape, not copied from the
existing (body-carrying) downstream server's larger ceiling.

## Failure mode

Any failure in the upstream-phase hook (credential fetch fails, lookup miss)
fails open to "no header mutation" — never blocks or fails the retry itself.
This feature only ever makes a retry *more likely* to succeed; it must never
become a new way for the retry to fail harder.

## oauth2-generator changes

Implements `UpstreamAttemptPolicy`: `OnUpstreamAttemptRequestHeaders` purges
the cache when `AttemptCount > 1` (implying the previous attempt was
rejected) then attaches the resulting (fresh) token — reusing the existing
token-source/cache code as-is, no duplicated fetch logic. The existing
downstream `OnRequestHeaders`/`OnResponseHeaders` purge-on-401 behavior is
unchanged for routes without `resilience.retry` configured; for routes that
do have it configured, the credential-attach/purge responsibility lives
solely in the upstream-phase hook for that route, so exactly one place does
it (no redundant double-fetch).

`dev-policies/oauth2-generator` (local) and `gateway-controllers/policies/oauth2-generator`
(source of truth) must be kept in sync throughout, per this repo's established
dual-repo convention.

## Non-goals (deferred, YAGNI)

- Backoff/jitter config, per-try timeouts on the native retry policy.
- Any policy besides oauth2-generator actually implementing the new
  interface (mechanism is generic; no second consumer exists yet).
- Body-phase upstream mutation (no current consumer needs it).

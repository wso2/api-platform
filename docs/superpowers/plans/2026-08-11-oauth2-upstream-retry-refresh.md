# OAuth2 Upstream-Retry Credential Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a `resilience.retry`-enabled route's backend returns 401 and Envoy natively retries, attach a *freshly fetched* OAuth2 token on the retried attempt instead of resending the same stale one — without any policy code making its own outbound HTTP call, and without the client seeing an intermediate 401.

**Architecture:** Envoy's *upstream* HTTP filter chain (configured per-cluster, distinct from the per-route/listener chain `ext_proc` already runs in) is re-created for every retry attempt — confirmed via Envoy's own docs/proto (`ext_proc.proto`'s `clear_route_cache` comment explicitly names "ext_proc filter is in the upstream filter chain" as a supported configuration; the AI Protocol Manager filter docs state an upstream-positioned filter "runs once per retry or hedged attempt"). We add a second, minimal `ext_proc` gRPC server inside the existing `policy-engine` process — separate from the main per-route kernel — wired into the cluster's upstream filter chain. It reads `x-envoy-attempt-count` (set by Envoy on the request sent to the upstream, starting at `1`, incrementing per retry) and, only when `> 1`, forces a fresh token fetch and overwrites `Authorization` before that specific attempt goes out. `oauth2-generator` implements a new, narrow SDK interface (`UpstreamRequestHeaderPolicy`) to plug into this — reusing its existing token cache/fetch code, not duplicating it.

**Tech Stack:** Go 1.26, `google.golang.org/grpc`, `github.com/envoyproxy/go-control-plane` (ext_proc v3 proto), Envoy (stock `envoyproxy/envoy` image — no custom Envoy build), `xoauth2` (`golang.org/x/oauth2`).

## Global Constraints

- No custom-compiled Envoy — every mechanism used must work against the stock `envoyproxy/envoy:${ENVOY_VERSION}` image already in use (confirmed: `gateway-runtime/gateway-runtime/Dockerfile`'s `python-deps`/`production` stages both `FROM envoyproxy/envoy:${ENVOY_VERSION}`).
- Scope is narrow and explicit opt-in: only `oauth2-generator` gains this capability initially, only when a NEW `refreshOnRetry: true` policy param is set, and only takes effect when the same route also has `resilience.retry` configured with `401` (or another 4xx meaning "credential rejected") in `statusCodes`. No route gets this behavior implicitly.
- On any failure inside the new upstream refresh path (IDP unreachable, malformed config, lookup miss), fail *open* to "no header mutation" — never block or fail the retry itself. The existing native retry (resend with the old header) must still happen even if refresh fails; this feature only ever makes the retry *more likely* to succeed, never a new way for it to fail harder.
- `dev-policies/oauth2-generator` (api-platform's local mirror) and `gateway-controllers/policies/oauth2-generator` (separate repo, source of truth) must be kept in sync throughout, per this project's established dual-repo convention — diff after every change to `oauth2-generator`.
- Every new Go file follows the security/hardening rules already enforced on this repo: constructed gRPC servers must set `MaxRecvMsgSize`/`MaxSendMsgSize`/`MaxConcurrentStreams` explicitly (`go-network-service-hardening.md` directive 2); no raw token values in log output (`GO-AUTH-003`).
- Query-param mutation on retry needs no new type: `:path` is an HTTP/2 pseudo-header, not in `ext_proc`'s default header-mutation exclusion list, so it's already reachable via `UpstreamHeaderModifications.HeadersToSet[":path"]`.
- Body mutation on retry (Task 1b) is built as a sibling `UpstreamRequestBodyPolicy` extension point, matching this SDK's one-phase-one-action-type convention (`OnRequestBody`→`RequestAction`, `OnResponseBody`→`ResponseAction`) — never as a second variant on `UpstreamHeaderAction`. It has **no current consumer**: `oauth2-generator` only implements the header interface, and the xDS translator (Task 7) never configures `request_body_mode` to anything but `NONE`. Task 1b/Task 3's body-dispatch code is verified by unit tests with synthetic input only — extending the translator to request `BUFFERED` body mode for a future body-capable policy is explicitly out of scope here.

---

## Phase 0: Investigation Spike (must complete before Phase 2/3 design is final)

### Task 0: Confirm where cluster identity is resolvable at policy-config-build time vs. only at per-request time

**Why this task exists:** the plan's registry design (Phase 2) keys refresh config by Envoy cluster name, resolved once when config loads. `sdk/core/policyengine.PolicyChain` (defined in `sdk/core/policyengine/config.go:23-31`) is keyed by `RouteKey` only — it does not carry a cluster name. Where the route's resolved cluster name actually becomes available inside `policy-engine` (at chain-build time in `internal/xdsclient/handler.go`, or only later, per-request, via `internal/kernel/execution_context.go`'s `toRequestUpstream`/`policyenginev1.UpstreamInfo`) determines whether Phase 2's registry can be populated once at config-load time (cheap) or must be resolved lazily per-request (more code, same outcome).

**Files to read:**
- `gateway/gateway-runtime/policy-engine/internal/xdsclient/handler.go` — `buildPolicyChain` (line ~461) and `HandlePolicyChainUpdate` (line ~104): does either receive or store a cluster name alongside the `PolicyChain`?
- `gateway/gateway-runtime/policy-engine/internal/kernel/execution_context.go` — `toRequestUpstream` (line ~1485) and callers: confirm this is the *only* place `policyenginev1.UpstreamInfo.ClusterName` becomes visible, and confirm it happens per-request, not at config load.
- `gateway/gateway-controller/pkg/xds/translator.go` — confirm every `RestApi`/`LLMProvider`/`LLMProxy`/`MCP` operation maps to a **dedicated** Envoy cluster (not one cluster shared across differently-configured routes). Search for any code path where two distinct routes' `RouteAction.ClusterSpecifier` can resolve to the identical cluster name when their attached policies (specifically `oauth2-generator`) differ. This determines whether Global Constraint "one `oauth2-generator` config per cluster" can be enforced structurally or needs an explicit validation check (added in Task 8).

- [ ] **Step 1:** Read the three files above and write a short findings doc at `docs/superpowers/plans/2026-08-11-oauth2-upstream-retry-refresh-findings.md` answering:
  1. Is cluster name known at `PolicyChain`-build time in `handler.go`, or only per-request in `execution_context.go`?
  2. Does any existing test (`grep -rn "ClusterKey\|ClusterName" gateway/gateway-controller/pkg/xds/translator_test.go`) already prove or disprove the one-cluster-per-route-config assumption?
  3. Given the answer to (1), state definitively: will Task 6 (the upstream-refresh registry) be populated at xDS-chain-build time (`internal/xdsclient/handler.go`) or lazily on first request per cluster (inside the new upstream ext_proc server itself, Task 5)?
- [ ] **Step 2:** Commit the findings doc.

```bash
git add docs/superpowers/plans/2026-08-11-oauth2-upstream-retry-refresh-findings.md
git commit -m "docs: investigation findings for oauth2 upstream-retry refresh plan"
```

> The remaining tasks below assume the **lazy, per-request resolution** answer (safer default: the registry is populated by the new upstream ext_proc server itself, on first request per cluster, by reading `xds.cluster_name` from ext_proc request attributes — see Task 5/6). If Task 0 finds cluster name IS available at chain-build time, Task 6's registration point moves from "lazy, inside the upstream server" to "eager, inside `handler.go`'s `buildPolicyChain`" — note this as a design update in the findings doc and adjust Task 6 accordingly before starting it.

---

## Phase 1: SDK primitives

### Task 1: Add `UpstreamHeaderContext` and `UpstreamRequestHeaderPolicy` to the policy SDK

**Files:**
- Modify: `sdk/core/policy/v1alpha2/context.go`
- Modify: `sdk/core/policy/v1alpha2/action.go`
- Create: `sdk/core/policy/v1alpha2/upstream_policy_test.go`

**Interfaces:**
- Produces: `policy.UpstreamHeaderContext{AttemptCount int, Headers *Headers, SharedContext *SharedContext}`, `policy.UpstreamRequestHeaderPolicy` interface with method `OnUpstreamRequestHeaders(ctx context.Context, uctx *UpstreamHeaderContext, params map[string]interface{}) UpstreamHeaderAction`, sealed `UpstreamHeaderAction` interface with one concrete type `UpstreamHeaderModifications{HeadersToSet map[string]string}`.
- Consumes: existing `Headers` type (`sdk/core/policy/v1alpha2/headers.go` — already used throughout this package) and `SharedContext` (`context.go:78-119`, already defined).

- [ ] **Step 1: Write the failing test** (`sdk/core/policy/v1alpha2/upstream_policy_test.go`)

```go
package v1alpha2

import (
	"context"
	"testing"
)

// fakeUpstreamRefreshPolicy proves any type implementing
// UpstreamRequestHeaderPolicy compiles and runs against the real
// context.Context/UpstreamHeaderContext/UpstreamHeaderAction types — this is
// a compile-time contract test, not a behavioral one; oauth2-generator's own
// tests (Task 9) cover real refresh behavior.
type fakeUpstreamRefreshPolicy struct{}

func (fakeUpstreamRefreshPolicy) OnUpstreamRequestHeaders(_ context.Context, uctx *UpstreamHeaderContext, _ map[string]interface{}) UpstreamHeaderAction {
	if uctx.AttemptCount <= 1 {
		return UpstreamHeaderModifications{}
	}
	return UpstreamHeaderModifications{HeadersToSet: map[string]string{"Authorization": "Bearer refreshed"}}
}

func TestUpstreamHeaderContext_AttemptCountGatesRefresh(t *testing.T) {
	var p UpstreamRequestHeaderPolicy = fakeUpstreamRefreshPolicy{}

	attemptOne := &UpstreamHeaderContext{AttemptCount: 1, Headers: NewHeaders(nil)}
	action := p.OnUpstreamRequestHeaders(context.Background(), attemptOne, nil)
	mods, ok := action.(UpstreamHeaderModifications)
	if !ok || len(mods.HeadersToSet) != 0 {
		t.Fatalf("attempt 1 must not mutate headers, got %#v", action)
	}

	attemptTwo := &UpstreamHeaderContext{AttemptCount: 2, Headers: NewHeaders(nil)}
	action2 := p.OnUpstreamRequestHeaders(context.Background(), attemptTwo, nil)
	mods2, ok := action2.(UpstreamHeaderModifications)
	if !ok || mods2.HeadersToSet["Authorization"] != "Bearer refreshed" {
		t.Fatalf("attempt 2 must carry the refreshed token, got %#v", action2)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd sdk/core && go test ./policy/v1alpha2/... -run TestUpstreamHeaderContext_AttemptCountGatesRefresh -v`
Expected: FAIL with `undefined: UpstreamHeaderContext` (the types and interface referenced by the test don't exist yet).

- [ ] **Step 3: Add the types** to `sdk/core/policy/v1alpha2/context.go`, appended after the existing `ResponseHeaderContext` block:

```go
// UpstreamHeaderContext is passed to UpstreamRequestHeaderPolicy.OnUpstreamRequestHeaders.
// Unlike every other context in this package, it is NOT scoped to one client
// request — it fires once per individual upstream dial attempt, including
// Envoy-native retries, because it runs in Envoy's per-cluster upstream HTTP
// filter chain rather than the per-route listener chain every other policy
// phase uses. AttemptCount comes from Envoy's own x-envoy-attempt-count
// header (1 on the first attempt, incrementing per retry) — a policy that
// wants "fresh token on retry only" behavior checks AttemptCount > 1 rather
// than reacting to any response, since this phase never sees a response at
// all (see the resilience.retry design notes: response-phase code always
// runs too late to influence a retry already in flight).
type UpstreamHeaderContext struct {
	*SharedContext

	// AttemptCount is Envoy's x-envoy-attempt-count for this specific dial,
	// starting at 1. Always >= 1; a missing/unparseable header is treated as 1
	// (never treated as "definitely a retry") so a misconfigured route fails
	// toward "behave like attempt 1" rather than toward unconditional refresh.
	AttemptCount int

	// Headers are this specific attempt's outgoing request headers, mutable via
	// the returned UpstreamHeaderAction.
	Headers *Headers
}

// UpstreamHeaderAction is the sealed return type for OnUpstreamRequestHeaders.
// Deliberately has exactly one variant, unlike RequestHeaderAction's two
// (Modifications | ImmediateResponse): this phase runs after routing and
// authentication are already resolved and mid-retry-loop inside Envoy's
// router filter, where there is no sensible notion of "reject this request" -
// only "optionally change headers for this one attempt."
type UpstreamHeaderAction interface {
	isUpstreamHeaderAction()
}

// UpstreamHeaderModifications sets the given headers on this specific
// upstream attempt. An empty/nil HeadersToSet is a valid, common no-op
// (e.g. AttemptCount == 1, nothing to refresh yet).
type UpstreamHeaderModifications struct {
	HeadersToSet map[string]string
}

func (UpstreamHeaderModifications) isUpstreamHeaderAction() {}
```

- [ ] **Step 4:** Add the interface to `sdk/core/policy/v1alpha2/action.go` (or wherever `RequestHeaderPolicy`/`ResponseHeaderPolicy` interfaces are defined in that file — match the existing location):

```go
// UpstreamRequestHeaderPolicy is implemented by policies that must re-run
// before every individual upstream dial attempt, not just once per
// downstream request — currently only used for credential refresh on
// Envoy-native retry. A policy implementing this interface is invoked from
// a completely different Envoy filter-chain position than every other
// policy phase (see UpstreamHeaderContext's doc comment); most policies
// should never implement this.
type UpstreamRequestHeaderPolicy interface {
	OnUpstreamRequestHeaders(ctx context.Context, uctx *UpstreamHeaderContext, params map[string]interface{}) UpstreamHeaderAction
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd sdk/core && go test ./policy/v1alpha2/... -run TestUpstreamHeaderContext_AttemptCountGatesRefresh -v`
Expected: PASS

- [ ] **Step 6: Run the full package test suite to confirm no regression**

Run: `cd sdk/core && go test ./policy/... -v`
Expected: all existing tests still PASS (no changes to any existing type).

- [ ] **Step 7: Commit**

```bash
git add sdk/core/policy/v1alpha2/context.go sdk/core/policy/v1alpha2/action.go sdk/core/policy/v1alpha2/upstream_policy_test.go
git commit -m "feat(sdk): add UpstreamRequestHeaderPolicy for per-retry-attempt credential refresh"
```

### Task 1b: Add `UpstreamRequestBodyPolicy` — a sibling body-mutation extension point (no current consumer)

**Added after Task 1 was already reviewed and committed**, per an explicit design discussion: query-param mutation needs no new type at all (`:path` is an HTTP/2 pseudo-header, not in `ext_proc`'s default header-mutation exclusion list — `host`/`:authority`/`:scheme`/`:method` only — so it's already reachable via Task 1's existing `UpstreamHeaderModifications.HeadersToSet[":path"]`). Body mutation is genuinely new surface, added here as a **sibling** interface+context+action triple, matching this SDK's established one-phase-one-action-type convention (`OnRequestBody` → `RequestAction`, `OnResponseBody` → `ResponseAction`, `OnRequestBodyChunk` → `StreamingRequestAction` — confirmed in `sdk/core/policy/v1alpha2/interface.go`/`action.go`) — never as a second variant squeezed into the existing `UpstreamHeaderAction`.

**IMPORTANT — this task has no current consumer and cannot be verified end-to-end.** `oauth2-generator` (the only policy this plan implements) only ever needs `Authorization`, so it will never implement `UpstreamRequestBodyPolicy`. Task 7's `buildUpstreamRefreshExtProc` continues to set `RequestBodyMode: NONE` unconditionally — wiring the xDS translator to turn on `BUFFERED` body mode for a future body-capable policy's cluster is explicitly **out of scope** for this plan (leave a comment marking it, don't build it speculatively). This task and Task 3's corresponding server-side dispatch are verified by **unit tests with synthetic input only** — real Envoy traffic will never invoke `OnUpstreamRequestBody` until a future plan both implements a consumer AND extends the translator. State this limitation in the code's doc comments, not just here.

**Files:**
- Modify: `sdk/core/policy/v1alpha2/context.go`
- Modify: `sdk/core/policy/v1alpha2/interface.go`
- Modify: `sdk/core/policy/v1alpha2/upstream_policy_test.go`

**Interfaces:**
- Produces: `policy.UpstreamBodyContext{AttemptCount int, Body []byte, SharedContext *SharedContext}`, `policy.UpstreamRequestBodyPolicy` interface with method `OnUpstreamRequestBody(ctx context.Context, bctx *UpstreamBodyContext, params map[string]interface{}) UpstreamBodyAction`, sealed `UpstreamBodyAction` interface with one concrete type `UpstreamBodyModifications{Body []byte}`.
- Consumes: nothing new — same package as Task 1, additive only.

- [ ] **Step 1: Write the failing test**, appended to `upstream_policy_test.go`:

```go
type fakeBodyRefreshPolicy struct{}

func (fakeBodyRefreshPolicy) OnUpstreamRequestBody(_ context.Context, bctx *UpstreamBodyContext, _ map[string]interface{}) UpstreamBodyAction {
	if bctx.AttemptCount <= 1 {
		return UpstreamBodyModifications{}
	}
	return UpstreamBodyModifications{Body: []byte(`{"signed":"fresh"}`)}
}

func TestUpstreamBodyContext_AttemptCountGatesRefresh(t *testing.T) {
	var p UpstreamRequestBodyPolicy = fakeBodyRefreshPolicy{}

	attemptOne := &UpstreamBodyContext{AttemptCount: 1, Body: []byte(`{"signed":"stale"}`)}
	action := p.OnUpstreamRequestBody(context.Background(), attemptOne, nil)
	mods, ok := action.(UpstreamBodyModifications)
	if !ok || mods.Body != nil {
		t.Fatalf("attempt 1 must not mutate the body, got %#v", action)
	}

	attemptTwo := &UpstreamBodyContext{AttemptCount: 2, Body: []byte(`{"signed":"stale"}`)}
	action2 := p.OnUpstreamRequestBody(context.Background(), attemptTwo, nil)
	mods2, ok := action2.(UpstreamBodyModifications)
	if !ok || string(mods2.Body) != `{"signed":"fresh"}` {
		t.Fatalf("attempt 2 must carry the refreshed body, got %#v", action2)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd sdk/core && go test ./policy/v1alpha2/... -run TestUpstreamBodyContext_AttemptCountGatesRefresh -v`
Expected: FAIL — types don't exist yet.

- [ ] **Step 3: Add the types** to `context.go`, appended after the `UpstreamHeaderModifications` block Task 1 added:

```go
// UpstreamBodyContext is passed to UpstreamRequestBodyPolicy.OnUpstreamRequestBody.
// Sibling to UpstreamHeaderContext (same per-retry-attempt firing semantics —
// see its doc comment), kept as a SEPARATE interface+context+action triple
// rather than a second variant on UpstreamHeaderAction, matching this SDK's
// one-phase-one-action-type convention elsewhere (RequestAction/ResponseAction
// for the body phases, StreamingRequestAction/StreamingResponseAction for the
// streaming ones).
//
// NOTE: as of this plan, no policy implements this interface and
// gateway-controller's xDS translator never configures request_body_mode to
// anything but NONE (see buildUpstreamRefreshExtProc) - this type and its
// server-side dispatch (policy-engine's internal/upstreamproc) exist as an
// extension point verified by unit tests with synthetic input only. Real
// Envoy traffic will not invoke OnUpstreamRequestBody until a future policy
// implements this interface AND the translator is extended to request
// BUFFERED body mode for that policy's cluster - that translator change is
// out of scope here.
type UpstreamBodyContext struct {
	*SharedContext

	// AttemptCount - see UpstreamHeaderContext.AttemptCount; identical semantics.
	AttemptCount int

	// Body is this specific attempt's outgoing request body.
	Body []byte
}

// UpstreamBodyAction is the sealed return type for OnUpstreamRequestBody.
// Single variant for the same reason UpstreamHeaderAction is: no sensible
// "reject" semantics this deep in Envoy's retry loop.
type UpstreamBodyAction interface {
	isUpstreamBodyAction()
}

// UpstreamBodyModifications replaces this specific attempt's outgoing body.
// A nil Body is a valid, common no-op (e.g. AttemptCount == 1).
type UpstreamBodyModifications struct {
	Body []byte
}

func (UpstreamBodyModifications) isUpstreamBodyAction() {}
```

- [ ] **Step 4:** Add the interface to `interface.go`, next to `UpstreamRequestHeaderPolicy` (Task 1 placed that interface here, not in `action.go` — confirmed correct by Task 1's review; follow the same placement):

```go
// UpstreamRequestBodyPolicy is the body-mutation sibling of
// UpstreamRequestHeaderPolicy - see that interface's doc comment for the
// shared per-retry-attempt firing model. No policy in this codebase
// implements this yet; see UpstreamBodyContext's doc comment for why real
// traffic cannot reach it until a future plan wires up both a consumer and
// translator support for BUFFERED request_body_mode.
type UpstreamRequestBodyPolicy interface {
	OnUpstreamRequestBody(ctx context.Context, bctx *UpstreamBodyContext, params map[string]interface{}) UpstreamBodyAction
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd sdk/core && go test ./policy/v1alpha2/... -run TestUpstreamBodyContext_AttemptCountGatesRefresh -v`
Expected: PASS

- [ ] **Step 6: Run the full package suite**

Run: `cd sdk/core && go test ./policy/... -v`
Expected: all PASS, zero regressions.

- [ ] **Step 7: Commit**

```bash
git add sdk/core/policy/v1alpha2/context.go sdk/core/policy/v1alpha2/interface.go sdk/core/policy/v1alpha2/upstream_policy_test.go
git commit -m "feat(sdk): add UpstreamRequestBodyPolicy sibling extension point (no current consumer)"
```

---

## Phase 2: policy-engine — minimal upstream `ext_proc` server

### Task 2: Add the upstream-refresh registry (cluster name -> registered policy + resolved params)

**Registration accepts a plain policy instance and type-asserts against both `UpstreamRequestHeaderPolicy` (Task 1) and `UpstreamRequestBodyPolicy` (Task 1b) independently — a registered policy may implement one, the other, or (uncommonly) both.** `oauth2-generator` (Task 9) only ever implements the header interface, so its registrations will always have a nil body policy — this is the expected, common case, not an error.

**Files:**
- Create: `gateway/gateway-runtime/policy-engine/internal/upstreamrefresh/registry.go`
- Create: `gateway/gateway-runtime/policy-engine/internal/upstreamrefresh/registry_test.go`

**Interfaces:**
- Produces: `upstreamrefresh.Registry` with `Register(clusterName string, p interface{}, params map[string]interface{})` (accepts any policy instance; internally type-asserts) and `Lookup(clusterName string) (headerPolicy policy.UpstreamRequestHeaderPolicy, bodyPolicy policy.UpstreamRequestBodyPolicy, params map[string]interface{}, ok bool)`, and a package-level `Default() *Registry` singleton (mirrors `sdk/core/utils/redisclient`'s `Shared()` pattern already established in this codebase for exactly this "one process-wide instance" need). `ok` is true whenever *either* `headerPolicy` or `bodyPolicy` is non-nil; `Register` itself returns an error if `p` implements neither interface (nothing to register).
- Consumes: `policy.UpstreamRequestHeaderPolicy` (Task 1), `policy.UpstreamRequestBodyPolicy` (Task 1b).

- [ ] **Step 1: Write the failing test**

```go
package upstreamrefresh

import (
	"context"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// stubHeaderOnlyPolicy is the common case - implements only the header
// interface, exactly like oauth2-generator.
type stubHeaderOnlyPolicy struct{}

func (stubHeaderOnlyPolicy) OnUpstreamRequestHeaders(context.Context, *policy.UpstreamHeaderContext, map[string]interface{}) policy.UpstreamHeaderAction {
	return policy.UpstreamHeaderModifications{}
}

// stubBothPolicy implements both interfaces, proving Register/Lookup handle
// a policy that satisfies both without forcing a choice.
type stubBothPolicy struct{ stubHeaderOnlyPolicy }

func (stubBothPolicy) OnUpstreamRequestBody(context.Context, *policy.UpstreamBodyContext, map[string]interface{}) policy.UpstreamBodyAction {
	return policy.UpstreamBodyModifications{}
}

func TestRegistry_RegisterAndLookup_HeaderOnly(t *testing.T) {
	r := NewRegistry()
	params := map[string]interface{}{"tokenEndpoint": "http://idp/token"}
	if err := r.Register("cluster-a", stubHeaderOnlyPolicy{}, params); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	headerPolicy, bodyPolicy, gotParams, ok := r.Lookup("cluster-a")
	if !ok {
		t.Fatal("expected cluster-a to be registered")
	}
	if headerPolicy == nil {
		t.Fatal("expected a non-nil header policy")
	}
	if bodyPolicy != nil {
		t.Fatal("stubHeaderOnlyPolicy does not implement the body interface, expected nil")
	}
	if gotParams["tokenEndpoint"] != "http://idp/token" {
		t.Fatalf("expected params round-trip, got %v", gotParams)
	}

	if _, _, _, ok := r.Lookup("cluster-b"); ok {
		t.Fatal("cluster-b was never registered, Lookup must report false")
	}
}

func TestRegistry_RegisterAndLookup_Both(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("cluster-a", stubBothPolicy{}, nil); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	headerPolicy, bodyPolicy, _, ok := r.Lookup("cluster-a")
	if !ok || headerPolicy == nil || bodyPolicy == nil {
		t.Fatalf("expected both policies non-nil, got header=%v body=%v ok=%v", headerPolicy, bodyPolicy, ok)
	}
}

func TestRegistry_Register_RejectsPolicyImplementingNeitherInterface(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("cluster-a", struct{}{}, nil); err == nil {
		t.Fatal("expected an error registering a value that implements neither upstream interface")
	}
}

func TestRegistry_ReRegisterReplaces(t *testing.T) {
	r := NewRegistry()
	_ = r.Register("cluster-a", stubHeaderOnlyPolicy{}, map[string]interface{}{"v": 1})
	_ = r.Register("cluster-a", stubHeaderOnlyPolicy{}, map[string]interface{}{"v": 2})

	_, _, params, ok := r.Lookup("cluster-a")
	if !ok || params["v"] != 2 {
		t.Fatalf("expected the second Register call to win, got %v", params)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd gateway/gateway-runtime/policy-engine && go test ./internal/upstreamrefresh/... -v`
Expected: FAIL — package/types don't exist.

- [ ] **Step 3: Implement**

```go
// Package upstreamrefresh holds the process-wide registry mapping an Envoy
// cluster name to the policy (or policies) responsible for refreshing
// credentials on that cluster's retried upstream requests. Populated lazily
// by the upstream ext_proc server (see internal/upstreamproc) the first time
// it sees a request for a given cluster - see Phase 0's investigation task
// for why this is lazy rather than populated at xDS chain-build time.
package upstreamrefresh

import (
	"fmt"
	"sync"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

type entry struct {
	headerPolicy policy.UpstreamRequestHeaderPolicy // nil if p doesn't implement it
	bodyPolicy   policy.UpstreamRequestBodyPolicy    // nil if p doesn't implement it - always nil for oauth2-generator today
	params       map[string]interface{}
}

// Registry is safe for concurrent Register/Lookup calls.
type Registry struct {
	mu sync.RWMutex
	m  map[string]entry
}

func NewRegistry() *Registry {
	return &Registry{m: make(map[string]entry)}
}

// Register associates clusterName with whichever of UpstreamRequestHeaderPolicy/
// UpstreamRequestBodyPolicy p implements, replacing any prior registration for
// the same cluster - a config reload always wins over what was previously
// registered. Returns an error if p implements neither interface, since that
// means the caller passed something that was never meant to be registered here.
func (r *Registry) Register(clusterName string, p interface{}, params map[string]interface{}) error {
	headerPolicy, _ := p.(policy.UpstreamRequestHeaderPolicy)
	bodyPolicy, _ := p.(policy.UpstreamRequestBodyPolicy)
	if headerPolicy == nil && bodyPolicy == nil {
		return fmt.Errorf("upstreamrefresh: policy %T implements neither UpstreamRequestHeaderPolicy nor UpstreamRequestBodyPolicy", p)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[clusterName] = entry{headerPolicy: headerPolicy, bodyPolicy: bodyPolicy, params: params}
	return nil
}

// Lookup returns the registered header/body policies and params for
// clusterName. ok is true whenever either policy is non-nil. The caller (the
// upstream ext_proc server) must treat ok == false as "no refresh configured
// for this cluster" and pass the request through unmodified, never as an
// error - and must independently nil-check headerPolicy/bodyPolicy before
// using either, since a registration commonly has only one of the two set.
func (r *Registry) Lookup(clusterName string) (headerPolicy policy.UpstreamRequestHeaderPolicy, bodyPolicy policy.UpstreamRequestBodyPolicy, params map[string]interface{}, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, found := r.m[clusterName]
	if !found {
		return nil, nil, nil, false
	}
	return e.headerPolicy, e.bodyPolicy, e.params, e.headerPolicy != nil || e.bodyPolicy != nil
}

var (
	defaultOnce sync.Once
	defaultReg  *Registry
)

// Default returns the process-wide singleton registry - one per policy-engine
// process, mirroring sdk/core/utils/redisclient.Shared()'s pattern.
func Default() *Registry {
	defaultOnce.Do(func() { defaultReg = NewRegistry() })
	return defaultReg
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd gateway/gateway-runtime/policy-engine && go test ./internal/upstreamrefresh/... -v`
Expected: PASS (all four tests)

- [ ] **Step 5: Commit**

```bash
git add gateway/gateway-runtime/policy-engine/internal/upstreamrefresh/
git commit -m "feat(policy-engine): add cluster-keyed upstream-refresh registry (header + body)"
```

### Task 3: Add the minimal upstream `ext_proc` gRPC server

**Files:**
- Create: `gateway/gateway-runtime/policy-engine/internal/upstreamproc/server.go`
- Create: `gateway/gateway-runtime/policy-engine/internal/upstreamproc/server_test.go`

**Interfaces:**
- Consumes: `upstreamrefresh.Default()` (Task 2), `policy.UpstreamHeaderContext`/`UpstreamHeaderModifications` (Task 1), `policy.UpstreamBodyContext`/`UpstreamBodyModifications` (Task 1b).
- Produces: `upstreamproc.NewServer(reg *upstreamrefresh.Registry) *Server` implementing `extprocv3.ExternalProcessorServer` (the same interface `internal/kernel.ExternalProcessorServer` already implements — confirmed at `internal/kernel/extproc.go:102`).

This is intentionally a *separate, minimal* implementation, not a mode of the existing `kernel.ExternalProcessorServer` — the existing one models the full multi-phase, multi-policy, body-aware pipeline for the per-route listener chain; this one only ever handles `RequestHeaders` and (when a body-capable policy is registered — see Task 1b) `RequestBody`, never response or trailers, because Envoy only asks the upstream filter chain to process what the cluster's `processing_mode` requests.

**On the `RequestBody` branch below: this server implements it, but no real Envoy traffic will ever send it a `RequestBody` message in this plan.** Task 7's translator always configures `RequestBodyMode: NONE` (no policy in this plan implements `UpstreamRequestBodyPolicy` — see Task 1b). The `RequestBody` handling exists as a verified-by-unit-test-only extension point for a future body-capable policy; implement and test it the same way as the header path, but don't treat "no production trigger for it yet" as a reason to skip testing it.

- [ ] **Step 1: Write the failing test** (fakes the gRPC stream directly rather than standing up a real listener, matching how `internal/kernel/extproc.go`'s own tests are structured — check `internal/kernel/extproc_test.go` for the existing fake-stream pattern and reuse it):

```go
package upstreamproc

import (
	"context"
	"io"
	"testing"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/upstreamrefresh"
)

type fakeRefreshPolicy struct{ calls int }

func (p *fakeRefreshPolicy) OnUpstreamRequestHeaders(_ context.Context, uctx *policy.UpstreamHeaderContext, _ map[string]interface{}) policy.UpstreamHeaderAction {
	p.calls++
	if uctx.AttemptCount <= 1 {
		return policy.UpstreamHeaderModifications{}
	}
	return policy.UpstreamHeaderModifications{HeadersToSet: map[string]string{"Authorization": "Bearer refreshed-token"}}
}

// fakeBodyRefreshPolicy proves the RequestBody dispatch branch works via
// synthetic input only - see Task 1b's note: no real Envoy config in this
// plan ever sends this server a RequestBody message.
type fakeBodyRefreshPolicy struct{ calls int }

func (p *fakeBodyRefreshPolicy) OnUpstreamRequestBody(_ context.Context, bctx *policy.UpstreamBodyContext, _ map[string]interface{}) policy.UpstreamBodyAction {
	p.calls++
	if bctx.AttemptCount <= 1 {
		return policy.UpstreamBodyModifications{}
	}
	return policy.UpstreamBodyModifications{Body: []byte(`{"signed":"fresh"}`)}
}

// fakeProcessStream implements extprocv3.ExternalProcessor_ProcessServer
// with a scripted single request/response exchange - see
// internal/kernel/extproc_test.go for the established fake used by the
// main kernel's own tests; reuse that exact type instead of redefining one
// here if it is already exported/reusable across packages.
func TestServer_Process_AttemptOne_NoMutation(t *testing.T) {
	reg := upstreamrefresh.NewRegistry()
	p := &fakeRefreshPolicy{}
	if err := reg.Register("test-cluster", p, nil); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	srv := NewServer(reg)

	req := &extprocv3.ProcessingRequest{
		Attributes: map[string]*structpbStruct{ // see Step 3 for the real import path
			"envoy.filters.http.ext_proc": {},
		},
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
					{Key: "x-envoy-attempt-count", Value: "1"},
				}},
			},
		},
	}
	stream := newFakeStream(t, req)
	if err := srv.Process(stream); err != nil && err != io.EOF {
		t.Fatalf("Process returned error: %v", err)
	}
	if p.calls != 1 {
		t.Fatalf("expected policy to be invoked exactly once, got %d", p.calls)
	}
	resp := stream.sentResponses[0]
	hm := resp.GetRequestHeaders().GetResponse().GetHeaderMutation()
	if hm != nil && len(hm.SetHeaders) != 0 {
		t.Fatalf("attempt 1 must not mutate headers, got %v", hm.SetHeaders)
	}
}

func TestServer_Process_AttemptTwo_RefreshesAuthorization(t *testing.T) {
	reg := upstreamrefresh.NewRegistry()
	p := &fakeRefreshPolicy{}
	if err := reg.Register("test-cluster", p, nil); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	srv := NewServer(reg)

	req := &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
					{Key: "x-envoy-attempt-count", Value: "2"},
				}},
			},
		},
	}
	stream := newFakeStream(t, req)
	if err := srv.Process(stream); err != nil && err != io.EOF {
		t.Fatalf("Process returned error: %v", err)
	}
	resp := stream.sentResponses[0]
	hm := resp.GetRequestHeaders().GetResponse().GetHeaderMutation()
	found := false
	for _, h := range hm.GetSetHeaders() {
		if h.GetHeader().GetKey() == "Authorization" && h.GetHeader().GetValue() == "Bearer refreshed-token" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Authorization to be set to the refreshed token, got %v", hm)
	}
}

func TestServer_Process_UnregisteredCluster_NoMutation_NoError(t *testing.T) {
	reg := upstreamrefresh.NewRegistry() // nothing registered
	srv := NewServer(reg)

	req := &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
					{Key: "x-envoy-attempt-count", Value: "2"},
				}},
			},
		},
	}
	stream := newFakeStream(t, req)
	if err := srv.Process(stream); err != nil && err != io.EOF {
		t.Fatalf("Process must never error on an unregistered cluster (fail-open): %v", err)
	}
}

// TestServer_Process_RequestBody_AttemptTwo_RefreshesBody proves the
// RequestBody dispatch branch (Task 1b) via synthetic input - no real Envoy
// config in this plan ever sends this server a RequestBody message (see
// this task's note above), but the branch must still work when driven
// directly, the same as the header path.
func TestServer_Process_RequestBody_AttemptTwo_RefreshesBody(t *testing.T) {
	reg := upstreamrefresh.NewRegistry()
	p := &fakeBodyRefreshPolicy{}
	if err := reg.Register("test-cluster", p, nil); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	srv := NewServer(reg)

	req := &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestBody{
			RequestBody: &extprocv3.HttpBody{Body: []byte(`{"signed":"stale"}`)},
		},
	}
	// AttemptCount for a RequestBody message comes from the same
	// x-envoy-attempt-count header carried on the RequestHeaders message
	// earlier in the same stream - the fake stream helper must let this test
	// script a RequestHeaders message first (attempt 2), then this
	// RequestBody message, and the server must remember the attempt count
	// across the two Recv() calls within one stream (store it on a
	// per-stream variable inside Process, not a package/global).
	stream := newFakeStreamWithAttemptCount(t, 2, req)
	if err := srv.Process(stream); err != nil && err != io.EOF {
		t.Fatalf("Process returned error: %v", err)
	}
	if p.calls != 1 {
		t.Fatalf("expected body policy to be invoked exactly once, got %d", p.calls)
	}
	resp := stream.sentResponses[0]
	bm := resp.GetRequestBody().GetResponse().GetBodyMutation()
	if bm == nil || string(bm.GetBody()) != `{"signed":"fresh"}` {
		t.Fatalf("expected body replaced with the refreshed value, got %v", bm)
	}
}
```

> `newFakeStreamWithAttemptCount` is illustrative - design the actual fake-stream test helper (built from `internal/kernel/extproc_test.go`'s pattern per this task's earlier note) so a test can script a `RequestHeaders` message followed by a `RequestBody` message on the same stream, and confirm your `Server.Process` implementation carries the attempt count across `Recv()` calls within one stream (a single `Process` call handles the whole bidi stream in a loop - see the `Step 3` implementation below for where per-stream state needs to live).

> **Before writing this test for real**: read `internal/kernel/extproc_test.go` in full first and copy its exact fake-stream helper and its exact way of extracting the cluster name from `ProcessingRequest.Attributes` (populated via `request_attributes: ["xds.cluster_name"]`, configured in Task 5) — the sketch above uses a placeholder `structpbStruct` type name and a `newFakeStream` helper precisely because those exact names must match whatever the existing kernel test file already established, not be reinvented. This is the one piece of this task that depends on Task 0-style investigation of an existing file rather than new design.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd gateway/gateway-runtime/policy-engine && go test ./internal/upstreamproc/... -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement** `internal/upstreamproc/server.go`:

```go
// Package upstreamproc implements a second, minimal ext_proc gRPC service -
// registered on Envoy's per-cluster UPSTREAM filter chain, never the
// per-route listener chain the main kernel (internal/kernel) serves. It
// handles RequestHeaders always, and RequestBody only when a body-capable
// policy is registered for the stream's cluster (see internal/upstreamrefresh
// and Task 1b) - no production Envoy config in this plan ever requests
// RequestBody (Task 7 always sets request_body_mode: NONE), so that branch
// is exercised by unit tests only, not real traffic, until a future plan
// adds both a body-capable policy and translator support for BUFFERED mode.
package upstreamproc

import (
	"strconv"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/upstreamrefresh"
)

const clusterNameAttribute = "xds.cluster_name"

type Server struct {
	extprocv3.UnimplementedExternalProcessorServer
	reg *upstreamrefresh.Registry
}

func NewServer(reg *upstreamrefresh.Registry) *Server {
	return &Server{reg: reg}
}

// Process handles one bidi ext_proc stream. clusterName and attemptCount are
// resolved once, from the RequestHeaders message, and carried as per-stream
// local state into the loop's later iterations - a RequestBody message on
// the same stream carries neither its own attempt-count header nor cluster
// attribute (per the ext_proc protocol, those are RequestHeaders-message-only
// fields), so this is the only place they can be captured.
func (s *Server) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	var clusterName string
	var attemptCount int = 1

	for {
		req, err := stream.Recv()
		if err != nil {
			return err // io.EOF on clean stream close - same convention as internal/kernel's Process loop
		}

		switch {
		case req.GetRequestHeaders() != nil:
			headers := req.GetRequestHeaders().GetHeaders()
			clusterName = clusterNameFromAttributes(req)
			attemptCount = attemptCountFromHeaders(headers)

			resp := &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_RequestHeaders{
					RequestHeaders: &extprocv3.HeadersResponse{},
				},
			}

			headerPolicy, _, params, ok := s.reg.Lookup(clusterName)
			if ok && headerPolicy != nil {
				uctx := &policy.UpstreamHeaderContext{AttemptCount: attemptCount}
				action := headerPolicy.OnUpstreamRequestHeaders(stream.Context(), uctx, params)
				if mods, ok := action.(policy.UpstreamHeaderModifications); ok && len(mods.HeadersToSet) > 0 {
					resp.GetRequestHeaders().Response = &extprocv3.CommonResponse{
						HeaderMutation: &extprocv3.HeaderMutation{
							SetHeaders: setHeaders(mods.HeadersToSet),
						},
					}
				}
			}
			// !ok, or ok but headerPolicy is nil (only a body policy registered
			// for this cluster): resp stays a bare no-op HeadersResponse - fail
			// open, never an error.

			if err := stream.Send(resp); err != nil {
				return err
			}

		case req.GetRequestBody() != nil:
			// See this task's file-level doc comment: unreachable by real
			// traffic in this plan (request_body_mode is always NONE), kept
			// correct and tested for the future policy that will use it.
			resp := &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_RequestBody{
					RequestBody: &extprocv3.BodyResponse{},
				},
			}

			_, bodyPolicy, params, ok := s.reg.Lookup(clusterName)
			if ok && bodyPolicy != nil {
				bctx := &policy.UpstreamBodyContext{AttemptCount: attemptCount, Body: req.GetRequestBody().GetBody()}
				action := bodyPolicy.OnUpstreamRequestBody(stream.Context(), bctx, params)
				if mods, ok := action.(policy.UpstreamBodyModifications); ok && mods.Body != nil {
					resp.GetRequestBody().Response = &extprocv3.CommonResponse{
						Status: extprocv3.CommonResponse_CONTINUE_AND_REPLACE,
						BodyMutation: &extprocv3.BodyMutation{
							Mutation: &extprocv3.BodyMutation_Body{Body: mods.Body},
						},
					}
				}
			}

			if err := stream.Send(resp); err != nil {
				return err
			}

		default:
			// Neither RequestHeaders nor RequestBody (shouldn't happen given
			// processing_mode, but fail open rather than erroring the stream).
			if err := stream.Send(&extprocv3.ProcessingResponse{}); err != nil {
				return err
			}
		}
	}
}

func attemptCountFromHeaders(h *corev3.HeaderMap) int {
	for _, hv := range h.GetHeaders() {
		if hv.GetKey() == "x-envoy-attempt-count" {
			if n, err := strconv.Atoi(hv.GetValue()); err == nil && n > 0 {
				return n
			}
		}
	}
	return 1 // missing/unparseable -> treat as attempt 1, never as "definitely a retry"
}

func clusterNameFromAttributes(req *extprocv3.ProcessingRequest) string {
	for _, fields := range req.GetAttributes() {
		if v, ok := fields.GetFields()[clusterNameAttribute]; ok {
			return v.GetStringValue()
		}
	}
	return ""
}

func setHeaders(headers map[string]string) []*corev3.HeaderValueOption {
	out := make([]*corev3.HeaderValueOption, 0, len(headers))
	for k, v := range headers {
		out = append(out, &corev3.HeaderValueOption{
			Header: &corev3.HeaderValue{Key: k, Value: v},
		})
	}
	return out
}
```

> Confirm the exact `ProcessingRequest.Attributes` shape (`map[string]*structpb.Struct`, field access via `.GetFields()[...]`) against the go-control-plane version already vendored in `policy-engine/go.mod` before finalizing this file - the sketch above is written from the proto definitions read during Phase-0-adjacent research in this plan's originating conversation, not from a compiled build.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd gateway/gateway-runtime/policy-engine && go test ./internal/upstreamproc/... -v`
Expected: PASS (all four tests: attempt-one no-mutation, attempt-two header refresh, unregistered cluster no-op, and the RequestBody dispatch test)

- [ ] **Step 5: Commit**

```bash
git add gateway/gateway-runtime/policy-engine/internal/upstreamproc/
git commit -m "feat(policy-engine): minimal upstream ext_proc server for per-retry credential refresh"
```

### Task 4: Wire the new server into `policy-engine`'s startup on a second Unix socket

**Files:**
- Modify: `gateway/gateway-runtime/policy-engine/cmd/policy-engine/main.go`
- Modify: `gateway/gateway-runtime/policy-engine/internal/constants/constants.go`
- Modify: `gateway/gateway-runtime/docker-entrypoint.sh`

**Interfaces:**
- Consumes: `upstreamproc.NewServer` (Task 3), `upstreamrefresh.Default()` (Task 2).

- [ ] **Step 1:** Add the new socket path constant next to the existing one in `internal/constants/constants.go`:

```go
// DefaultUpstreamProcSocketPath is the second ext_proc socket, dedicated to
// the per-retry-attempt credential-refresh server (internal/upstreamproc) -
// registered on Envoy's per-cluster upstream filter chain, never the
// per-route listener chain DefaultPolicyEngineSocketPath serves.
DefaultUpstreamProcSocketPath = "/var/run/api-platform/policy-engine-upstream.sock"
```

(Add immediately after the existing `DefaultPolicyEngineSocketPath = "/var/run/api-platform/policy-engine.sock"` at line 51.)

- [ ] **Step 2:** In `cmd/policy-engine/main.go`, immediately after the existing `grpcServer := grpc.NewServer()` / `extprocv3.RegisterExternalProcessorServer(grpcServer, extprocServer)` block (lines 271-272), add a second listener + server for the upstream-refresh service. Per `go-network-service-hardening.md` directive 2, both this and the existing `grpcServer` construction must set explicit message-size/stream limits — if the existing `grpcServer := grpc.NewServer()` at line 271 doesn't already set these (check before assuming), add them to both in this same task rather than only the new one:

```go
upstreamGrpcServer := grpc.NewServer(
	grpc.MaxRecvMsgSize(1<<20),      // 1MiB - this server only ever handles headers, never a body
	grpc.MaxSendMsgSize(1<<20),
	grpc.MaxConcurrentStreams(1024),
)
upstreamprocv3.RegisterExternalProcessorServer(upstreamGrpcServer, upstreamproc.NewServer(upstreamrefresh.Default()))

upstreamSocketPath := constants.DefaultUpstreamProcSocketPath
if err := os.Remove(upstreamSocketPath); err != nil && !os.IsNotExist(err) {
	slog.WarnContext(ctx, "Failed to remove existing upstream-refresh socket file", "path", upstreamSocketPath, "error", err)
}
upstreamLis, err := net.Listen("unix", upstreamSocketPath)
if err != nil {
	slog.ErrorContext(ctx, "Failed to listen on upstream-refresh Unix socket", "path", upstreamSocketPath, "error", err)
	os.Exit(1)
}
if err := os.Chmod(upstreamSocketPath, 0660); err != nil {
	slog.WarnContext(ctx, "Failed to set upstream-refresh socket permissions", "path", upstreamSocketPath, "error", err)
}
go func() {
	if err := upstreamGrpcServer.Serve(upstreamLis); err != nil {
		slog.ErrorContext(ctx, "upstream-refresh gRPC server error", "error", err)
	}
}()
slog.InfoContext(ctx, "Policy Engine upstream-refresh server listening on Unix socket", "path", upstreamSocketPath)
```

Add `"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/upstreamproc"` and `"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/upstreamrefresh"` to the import block, and confirm whether `extprocv3` is already aliased for the main server's import — reuse the identical import for `upstreamprocv3` (it's the same proto package, `RegisterExternalProcessorServer` registers a distinct service instance on a distinct `grpc.Server`, so no alias collision even though it's the same generated type).

> This intentionally does NOT touch `tcp` mode (lines 260-269) - the upstream-refresh socket is UDS-only for the initial version, since Envoy's cluster-level `grpc_service` can point at a UDS regardless of how the main ext_proc filter is configured. Extend to TCP mode only if a real deployment needs it (YAGNI otherwise).

- [ ] **Step 3:** Update `gateway-runtime/docker-entrypoint.sh` to wait for the new socket the same way it already waits for `POLICY_ENGINE_SOCKET` (lines 277-298). Add immediately after that existing wait loop, before "Starting Envoy...":

```bash
UPSTREAM_PROC_SOCKET="/var/run/api-platform/policy-engine-upstream.sock"
UPSTREAM_SOCKET_WAIT_TIMEOUT=10
UPSTREAM_SOCKET_WAIT_COUNT=0
while [ ! -S "${UPSTREAM_PROC_SOCKET}" ]; do
    if [ $UPSTREAM_SOCKET_WAIT_COUNT -ge $UPSTREAM_SOCKET_WAIT_TIMEOUT ]; then
        log "ERROR: Upstream-refresh socket not created within ${UPSTREAM_SOCKET_WAIT_TIMEOUT}s"
        exit 1
    fi
    if ! kill -0 "$PE_PID" 2>/dev/null; then
        log "ERROR: Policy Engine exited before creating upstream-refresh socket"
        exit 1
    fi
    sleep 1
    UPSTREAM_SOCKET_WAIT_COUNT=$((UPSTREAM_SOCKET_WAIT_COUNT + 1))
done
log "Upstream-refresh socket ready: ${UPSTREAM_PROC_SOCKET}"
```

Also add `"${UPSTREAM_PROC_SOCKET}"` to the existing cleanup line at line 228 (`rm -f "${POLICY_ENGINE_SOCKET}" "${PYTHON_EXECUTOR_SOCKET}"`) and the final cleanup at line 349, so a restart doesn't fail on a stale socket file.

- [ ] **Step 4: Build to confirm no compile errors**

Run: `cd gateway/gateway-runtime/policy-engine && go build ./...`
Expected: clean build.

- [ ] **Step 5: Verify the entrypoint script's shell syntax**

Run: `bash -n gateway/gateway-runtime/docker-entrypoint.sh`
Expected: no output (valid syntax).

- [ ] **Step 6: Commit**

```bash
git add gateway/gateway-runtime/policy-engine/cmd/policy-engine/main.go \
        gateway/gateway-runtime/policy-engine/internal/constants/constants.go \
        gateway/gateway-runtime/docker-entrypoint.sh
git commit -m "feat(policy-engine): start the upstream-refresh ext_proc server alongside the main one"
```

### Task 5: Register `oauth2-generator` chains into the upstream-refresh registry

**Resolved by Task 0's findings** (`docs/superpowers/plans/2026-08-11-oauth2-upstream-retry-refresh-findings.md`, Question 3): register lazily, per-request, inside `internal/kernel` — not eagerly in `handler.go`/`buildPolicyChain`. Cluster identity *is* knowable earlier (via a separate `RouteConfig` xDS resource, `HandleRouteConfigUpdate`), but `PolicyChain`/`buildPolicyChain` never carries it and was never designed to, and — more importantly — cluster names are **not** dedicated per route/operation (host+scheme dedup, shared across unrelated APIs), so an eager join wouldn't remove the need for Task 8's collision detection anyway. Keeping this lazy avoids needing two separate registration code paths for what is one registry.

**Files:**
- Modify: `gateway/gateway-runtime/policy-engine/internal/kernel/execution_context.go` (or wherever `toRequestUpstream`/`UpstreamInfo.ClusterName` resolution actually lives per the findings doc's Question 1c citations — `execution_context.go:1477-1509` per the findings doc)

**Interfaces:**
- Consumes: `upstreamrefresh.Default().Register(clusterName string, p interface{}, params map[string]interface{}) error` (Task 2's real signature — accepts any policy instance, type-asserts internally, returns an error only if `p` implements neither upstream interface), the concrete `oauth2-generator` policy instance obtained from the existing per-route policy-chain build (whatever factory/registry call already instantiates a named policy for that route — read the surrounding code to find the exact call).

- [ ] **Step 1:** In the per-request path that resolves `UpstreamInfo.ClusterName` for a request (per the findings doc's citations), for the route's policy chain: if it includes an `oauth2-generator` instance with `params["refreshOnRetry"] == true`, call:

```go
if err := upstreamrefresh.Default().Register(info.ClusterName, instantiatedPolicy, resolvedParams); err != nil {
	// instantiatedPolicy came from a refreshOnRetry: true attachment, so it
	// must implement UpstreamRequestHeaderPolicy - this error path means
	// something is structurally wrong (e.g. a non-oauth2-generator policy
	// got refreshOnRetry: true param bypassing Task 6's validation). Log,
	// don't panic - the request itself must still proceed normally.
	slog.WarnContext(ctx, "failed to register upstream-refresh policy", "cluster", info.ClusterName, "error", err)
}
```

This is idempotent (Task 2's `Register` always overwrites), so calling it on every request for that route is correct, just slightly wasteful — acceptable given routes rarely reconfigure mid-traffic.

- [ ] **Step 2:** Test this at the `execution_context_test.go` level (or wherever the resolution code's existing tests live): a request whose route has `oauth2-generator{refreshOnRetry: true}` results in `upstreamrefresh.Default().Lookup(clusterName)` returning `ok == true` after the request is processed; a route without `refreshOnRetry` (or without `oauth2-generator` at all) does not register anything. Use a stub policy implementing `UpstreamRequestHeaderPolicy` rather than the real `oauth2-generator` package (avoid a cross-module test dependency on `gateway-controllers/policies/oauth2-generator` from `policy-engine`'s own test suite).

- [ ] **Step 3: Run the affected package's tests**

Run: `cd gateway/gateway-runtime/policy-engine && go test ./internal/kernel/... -v`
Expected: PASS, including the new registration test.

- [ ] **Step 4: Commit**

```bash
git add gateway/gateway-runtime/policy-engine/internal/kernel/
git commit -m "feat(policy-engine): register refreshOnRetry-enabled policies into the upstream-refresh registry"
```

---

## Phase 3: gateway-controller — emit the cluster's upstream filter chain

### Task 6: Add `refreshOnRetry` to `oauth2-generator`'s policy-definition schema, validated against `resilience.retry`

**Files:**
- Modify: `gateway-controllers/policies/oauth2-generator/policy-definition.yaml`
- Modify: `gateway/dev-policies/oauth2-generator/policy-definition.yaml` (sync mirror — diff after)
- Modify: `gateway/gateway-controller/pkg/config/api_validator.go` (or wherever policy-param cross-validation already happens for other policies — search for an existing precedent, e.g. how `advanced-ratelimit`'s params are cross-checked against other config, before inventing a new validation entry point)

- [ ] **Step 1:** Add to `oauth2-generator/policy-definition.yaml`'s parameter schema:

```yaml
refreshOnRetry:
  type: boolean
  default: false
  description: >
    When true, and this route's resilience.retry includes a status code this
    policy would also treat as "credential rejected", fetch a fresh token
    specifically for Envoy's native retry attempt (attempt 2+), instead of
    resending the same token that just failed. Requires resilience.retry to
    be configured on the same route with at least one status code this
    policy's own purgeStatusCodes/tokenPurgeStatusCodes also covers (e.g.
    401) - has no effect otherwise, since Envoy will never retry an
    unconfigured status code in the first place.
```

- [ ] **Step 2:** Add a validation check (exact location depends on where policy-param-to-resilience cross-checks already live in this codebase; if none exist yet, add it in `api_validator.go`'s per-operation validation loop, near `validateResilienceRetry`, Task 14 from the earlier `resilience.retry` implementation): if any policy named `oauth2-generator` on an operation has `params.refreshOnRetry == true`, require that operation's effective `resilience.retry.statusCodes` (API-level or operation-level, whichever resolves) is non-empty. Emit a `ValidationError` if not:

```go
if refreshOnRetry, _ := getPolicyBoolParam(op.Policies, "oauth2-generator", "refreshOnRetry"); refreshOnRetry {
	if len(effectiveRetryStatusCodes) == 0 {
		errors = append(errors, ValidationError{
			Field:   fmt.Sprintf("spec.operations[%d].policies", i),
			Message: "oauth2-generator's refreshOnRetry requires resilience.retry.statusCodes to be configured on the same operation (or inherited from the API level) - otherwise Envoy never retries and refreshOnRetry has no effect",
		})
	}
}
```

(`getPolicyBoolParam` and `effectiveRetryStatusCodes` are illustrative names — match whatever helper naming convention the surrounding validator file already uses for reading a named policy's param off an operation's resolved policy list; read the file before writing this to reuse an existing helper if one already exists for reading a policy's params by name.)

- [ ] **Step 3: Write a test** in `api_validator_test.go` (same table-driven style as `TestAPIValidator_ValidateResilience` from the earlier `resilience.retry` work) asserting: `oauth2-generator` with `refreshOnRetry: true` and no `resilience.retry` → validation error; same policy with `resilience.retry.statusCodes: [401]` present → no error.

- [ ] **Step 4: Run the test**

Run: `cd gateway/gateway-controller && go test ./pkg/config/... -run TestAPIValidator -v`
Expected: PASS

- [ ] **Step 5: Diff the two `oauth2-generator` copies to confirm they're in sync**

Run: `diff -rq /Users/thenujan/Desktop/Git-Repos/gateway-controllers/policies/oauth2-generator/ /Users/thenujan/Desktop/Git-Repos/api-platform/gateway/dev-policies/oauth2-generator/`
Expected: only the already-known `go.mod` (replace-path) and `e2e/` diffs — the `policy-definition.yaml` change must be identical in both.

- [ ] **Step 6: Commit** (both repos)

```bash
# in gateway-controllers
git add policies/oauth2-generator/policy-definition.yaml
git commit -m "feat: add refreshOnRetry param, requires resilience.retry to be configured"

# in api-platform
git add gateway/dev-policies/oauth2-generator/policy-definition.yaml \
        gateway/gateway-controller/pkg/config/api_validator.go \
        gateway/gateway-controller/pkg/config/api_validator_test.go
git commit -m "feat(gateway-controller): validate oauth2-generator's refreshOnRetry requires resilience.retry"
```

### Task 7: Emit the cluster's upstream `ext_proc` filter from the xDS translator

**Files:**
- Modify: `gateway/gateway-controller/pkg/xds/translator.go` — `createCluster` (referenced at line 1121 in earlier work on this route; find its exact current definition) and/or wherever `cluster.Cluster{}` is finally constructed for a `RestApi`/`LLMProvider` main upstream.
- Modify: `gateway/gateway-controller/pkg/xds/translator_test.go`

**Interfaces:**
- Consumes: whether the operation attaching to this cluster has `oauth2-generator` with `refreshOnRetry: true` (resolved during the same pass that already resolves `resilience.retry` for that operation — thread this bool through alongside `resolvedTimeout`/`RouteRetry` from the earlier `resilience.retry` work, since both are resolved per-operation before the cluster is built).

- [ ] **Step 1:** Write a failing test asserting: given an operation with `oauth2-generator{refreshOnRetry: true}` and `resilience.retry.statusCodes: [401]`, the resulting `cluster.Cluster` has `TypedExtensionProtocolOptions["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"]` containing an `http_filters` list ending in `envoy.filters.http.upstream_codec`, preceded by an `envoy.filters.http.ext_proc` filter whose `grpc_service.envoy_grpc.cluster_name` (or `target_uri` for UDS — match whatever addressing style the EXISTING downstream ext_proc filter config in this codebase already uses; grep for how the main `envoy.filters.http.ext_proc` filter's `grpc_service` is currently built in `translator.go` and mirror it exactly, just pointed at `constants.DefaultUpstreamProcSocketPath` instead) points at the upstream-refresh socket, and whose `processing_mode.request_header_mode == SEND` with every other mode `SKIP`, and whose `request_attributes` includes `"xds.cluster_name"`.

```go
func TestTranslator_ClusterUpstreamExtProc_WhenRefreshOnRetryEnabled(t *testing.T) {
	// ... build a minimal RestApi config with one operation carrying
	// oauth2-generator{refreshOnRetry: true} and resilience.retry{statusCodes: [401]} ...
	// translate it, then assert on the resulting cluster's
	// TypedExtensionProtocolOptions as described above.
}

func TestTranslator_ClusterUpstreamExtProc_AbsentWhenNotConfigured(t *testing.T) {
	// same shape, but refreshOnRetry is false/absent -> cluster must have
	// no TypedExtensionProtocolOptions upstream ext_proc entry at all.
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd gateway/gateway-controller && go test ./pkg/xds/... -run TestTranslator_ClusterUpstreamExtProc -v`
Expected: FAIL (feature doesn't exist yet).

- [ ] **Step 3: Implement.** Add a helper mirroring `buildRetryPolicy` (from the earlier `resilience.retry` work) in shape:

```go
// buildUpstreamRefreshExtProc returns the cluster-level upstream ext_proc
// HTTP filter config for credential refresh-on-retry, or nil when
// refreshOnRetry isn't enabled for this cluster's route. Mirrors the
// downstream ext_proc filter's grpc_service addressing (see
// buildDownstreamExtProcFilter or equivalent - match existing naming) but
// targets constants.DefaultUpstreamProcSocketPath and restricts
// processing_mode to headers-only, since this server never sees a body.
func (t *Translator) buildUpstreamRefreshExtProc(refreshOnRetry bool) *hcmv3.HttpFilter {
	if !refreshOnRetry {
		return nil
	}
	extProcConfig := &extprocv3.ExternalProcessor{
		GrpcService: &corev3.GrpcService{
			TargetSpecifier: &corev3.GrpcService_EnvoyGrpc_{
				EnvoyGrpc: &corev3.GrpcService_EnvoyGrpc{ClusterName: upstreamProcClusterName},
			},
		},
		ProcessingMode: &extprocv3.ProcessingMode{
			RequestHeaderMode:   extprocv3.ProcessingMode_SEND,
			ResponseHeaderMode:  extprocv3.ProcessingMode_SKIP,
			RequestBodyMode:     extprocv3.ProcessingMode_NONE,
			ResponseBodyMode:    extprocv3.ProcessingMode_NONE,
			RequestTrailerMode:  extprocv3.ProcessingMode_SKIP,
			ResponseTrailerMode: extprocv3.ProcessingMode_SKIP,
		},
		RequestAttributes: []string{"xds.cluster_name"},
	}
	anyConfig, err := anypb.New(extProcConfig)
	if err != nil {
		// Same fail-fast posture as every other proto-marshal error site in
		// this translator - surfacing at xDS-build time, never silently
		// skipped.
		panic(fmt.Sprintf("failed to marshal upstream ext_proc config: %v", err))
	}
	return &hcmv3.HttpFilter{
		Name:       "envoy.filters.http.ext_proc",
		ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: anyConfig},
	}
}
```

> `upstreamProcClusterName` needs its own static Envoy `cluster.Cluster` definition (a UDS cluster pointing at `constants.DefaultUpstreamProcSocketPath`, same pattern as however the MAIN ext_proc's own gRPC cluster is already defined in this translator for the downstream filter — find and reuse that exact pattern/constant rather than reinventing cluster construction). Add this as a package-level constant alongside wherever the downstream ext_proc cluster name constant already lives.

Wire `buildUpstreamRefreshExtProc`'s result into the cluster's `TypedExtensionProtocolOptions`, terminated with `envoy.filters.http.upstream_codec` (per the RBAC example pattern: `http_filters: [ext_proc_filter, upstream_codec_filter]`), only when the resolved `refreshOnRetry` bool for that cluster's route is true.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd gateway/gateway-controller && go test ./pkg/xds/... -run TestTranslator_ClusterUpstreamExtProc -v`
Expected: PASS (both tests)

- [ ] **Step 5: Run the full package test suite**

Run: `cd gateway/gateway-controller && go test ./... `
Expected: all PASS, no regression to the existing `resilience.retry`/timeout cluster-building tests.

- [ ] **Step 6: Commit**

```bash
git add gateway/gateway-controller/pkg/xds/translator.go gateway/gateway-controller/pkg/xds/translator_test.go
git commit -m "feat(gateway-controller): emit cluster upstream ext_proc filter when refreshOnRetry is enabled"
```

### Task 8: Enforce "one `oauth2-generator` refresh config per cluster" (structural safety net)

**Rewritten after Task 7 landed — the original text above assumed a data model Task 7 didn't actually build, and understated the collision surface Task 0 found.** Two corrections that change this task's real shape:

1. **`models.UpstreamCluster.RefreshOnRetry` is a bare `bool`** (Task 7's actual field, `runtime_deploy_config.go`) — it carries no oauth2-generator params at all. The real, secret-bearing oauth2-generator config (`tokenEndpoint`, `clientId`, etc.) never reaches gateway-controller's cluster model; it's resolved entirely on the policy-engine side, per-request, inside `internal/kernel`'s `registerUpstreamRefreshPolicies` (Task 5) and stored in `upstreamrefresh.Registry` (Task 2) — keyed by cluster name, overwritten on every request with no conflict detection today. So "detect two different oauth2-generator configs sharing a cluster" cannot be built as a boolean comparison on the existing field — it needs the actual **raw policy `params`** (as literally written in each operation's/API's `policies:` block — not runtime-resolved secrets) compared for equality, and that comparison has to happen wherever those raw params are still in scope together: in `pkg/transform/restapi.go`'s `RestAPITransformer.Transform` (same place Task 7 already resolves the `RefreshOnRetry` bool from `spec.Policies`/`op.Policies`).
2. **The collision is cross-API, not just cross-operation.** Task 0's findings (`docs/superpowers/plans/2026-08-11-oauth2-upstream-retry-refresh-findings.md`, Question 2) proved Envoy cluster names are derived purely from upstream `host+scheme` and are deduplicated **across every deployed API** in `gateway-controller/pkg/xds/translator.go`'s `TranslateConfigs` (`clusterMap[c.Name] = c`, last-write-wins, line ~797 as of that investigation). Two unrelated APIs pointing their main upstream at the same host+scheme, each with a *different* `oauth2-generator{refreshOnRetry:true}` config, collapse onto one Envoy cluster today with zero detection. A check scoped to "within one API's operations" (the original Step 1's test) would miss this, more likely, real-world case entirely.

**Files:**
- Modify: `gateway/gateway-controller/pkg/models/runtime_deploy_config.go` — `UpstreamCluster` gains a field carrying the raw oauth2-generator params when `RefreshOnRetry` is true (e.g. `RefreshOnRetryParams map[string]interface{}`), populated alongside `RefreshOnRetry` wherever Task 7 already sets that bool.
- Modify: `gateway/gateway-controller/pkg/transform/restapi.go` — where `RefreshOnRetry` is resolved (via `xds.ResolveRefreshOnRetry` per Task 7's report), also capture and set the winning policy's raw `params` map onto the new field.
- Modify: `gateway/gateway-controller/pkg/xds/translator.go` — `TranslateConfigs`'s cross-API cluster-merge loop (where `clusterMap[c.Name] = c` happens) is the one place that sees every API's contribution to a shared cluster name in one pass; add the collision check here, comparing `RefreshOnRetryParams` (deep-equal, e.g. via `reflect.DeepEqual` or a stable JSON-marshal comparison) between what's already in `clusterMap` for that name and the incoming cluster's value, whenever both are non-nil.
- Modify: `gateway/gateway-controller/pkg/xds/translator_test.go`.

**Interfaces:**
- Consumes: `models.UpstreamCluster.RefreshOnRetry`/the new params field (this task's own addition to that struct), `xds.ResolveRefreshOnRetry` (Task 7).
- Produces: `TranslateConfigs` returns an error (instead of silently merging) when two different APIs' clusters of the same name carry different non-nil `RefreshOnRetryParams`.

- [ ] **Step 1:** Read `pkg/xds/translator.go`'s current `TranslateConfigs` and the current `RestAPITransformer.Transform`/`ResolveRefreshOnRetry` (both landed in Task 7's fix) to find their exact real shape — Task 7's report has the exact function/line references from when it was implemented; confirm they haven't drifted.
- [ ] **Step 2:** Add `RefreshOnRetryParams map[string]interface{}` to `models.UpstreamCluster`, set it in `restapi.go` alongside the existing `RefreshOnRetry bool` (same resolution call, just also capture the winning policy's `Params`).
- [ ] **Step 3:** Write a failing test for the **cross-API** case first (the more important, more likely one): two separate `RestApi`/`LLMProvider` configs, same upstream host+scheme (so they resolve to the identical Envoy cluster name), each with `oauth2-generator{refreshOnRetry:true}` but different `tokenEndpoint` params, both passed into one `TranslateConfigs` call — assert it returns an error naming the conflicting cluster, not a silently-merged result.
- [ ] **Step 4:** Write a second failing test for the **within-one-API, two-operations** case (two operations sharing the API's main cluster, different operation-level `oauth2-generator` params) — same assertion.
- [ ] **Step 5:** Implement the check inside `TranslateConfigs`'s cluster-merge loop: before `clusterMap[c.Name] = c` overwrites an existing entry, if both the existing and incoming cluster have non-nil `RefreshOnRetryParams` that aren't deep-equal, return `fmt.Errorf("cluster %q has conflicting refreshOnRetry oauth2-generator configurations from different APIs/operations sharing this upstream - refreshOnRetry requires exactly one credential-refresh configuration per cluster", c.Name)` instead of proceeding.
- [ ] **Step 6: Run both new tests**

Run: `cd gateway/gateway-controller && go test ./pkg/xds/... -run TestTranslator_UpstreamRefresh_ConflictingConfig -v`
Expected: PASS (both the cross-API and within-API variants)

- [ ] **Step 7: Run the full package suite**

Run: `cd gateway/gateway-controller && go test ./...`
Expected: all PASS, no regression to Task 7's tests or any other cluster-building test.

- [ ] **Step 8: Commit**

```bash
git add gateway/gateway-controller/pkg/models/runtime_deploy_config.go gateway/gateway-controller/pkg/transform/restapi.go gateway/gateway-controller/pkg/xds/translator.go gateway/gateway-controller/pkg/xds/translator_test.go
git commit -m "fix(gateway-controller): reject conflicting refreshOnRetry configs sharing one cluster, including across APIs"
```

---

## Phase 4: `oauth2-generator` — implement the refresh

### Task 9: Implement `UpstreamRequestHeaderPolicy` on the oauth2-generator policy, reusing the existing token cache

**Files:**
- Modify: `gateway-controllers/policies/oauth2-generator/oauth2_generator.go`
- Modify: `gateway-controllers/policies/oauth2-generator/token_cache.go`
- Modify: `gateway-controllers/policies/oauth2-generator/oauth2_generator_test.go`
- Sync all three to `gateway/dev-policies/oauth2-generator/` after.

**Interfaces:**
- Consumes: `policy.UpstreamHeaderContext`/`UpstreamHeaderModifications`/`UpstreamRequestHeaderPolicy` (Task 1); the existing `tokenSource xoauth2.TokenSource` and `cacheParams` already built by `extractCacheParams`/`buildTokenSource` (established in prior work this session, `token_cache.go`).
- Produces: `(*OAuth2GeneratorPolicy) OnUpstreamRequestHeaders(ctx, uctx, params) policy.UpstreamHeaderAction`.

- [ ] **Step 1: Write the failing test.** Reuse this file's existing test doubles for a fake token source (check `token_cache_test.go` for whatever fake `xoauth2.TokenSource` is already used — reuse it, don't create a second one) that returns a distinguishable token value each call (e.g. an incrementing counter embedded in the token string), so the test can assert the SECOND call's token differs from the first:

```go
func TestOAuth2GeneratorPolicy_OnUpstreamRequestHeaders_AttemptOne_NoMutation(t *testing.T) {
	p := &OAuth2GeneratorPolicy{}
	params := testRedisParams(nil) // or whatever the existing minimal-valid-params helper is called
	uctx := &policy.UpstreamHeaderContext{AttemptCount: 1, Headers: policy.NewHeaders(nil)}

	action := p.OnUpstreamRequestHeaders(context.Background(), uctx, params)
	mods, ok := action.(policy.UpstreamHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamHeaderModifications, got %T", action)
	}
	if len(mods.HeadersToSet) != 0 {
		t.Fatalf("attempt 1 must not mutate headers, got %v", mods.HeadersToSet)
	}
}

func TestOAuth2GeneratorPolicy_OnUpstreamRequestHeaders_AttemptTwo_FetchesFreshToken(t *testing.T) {
	p := &OAuth2GeneratorPolicy{}
	params := testRedisParams(nil)
	uctx := &policy.UpstreamHeaderContext{AttemptCount: 2, Headers: policy.NewHeaders(nil)}

	action := p.OnUpstreamRequestHeaders(context.Background(), uctx, params)
	mods, ok := action.(policy.UpstreamHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamHeaderModifications, got %T", action)
	}
	auth, present := mods.HeadersToSet["Authorization"]
	if !present || auth == "" {
		t.Fatalf("attempt 2 must set a non-empty Authorization header, got %v", mods.HeadersToSet)
	}
}

func TestOAuth2GeneratorPolicy_OnUpstreamRequestHeaders_TokenFetchError_FailsOpenNoMutation(t *testing.T) {
	// Wire in whatever this file's existing "always-erroring IdP" test
	// double is (check for one already used to test the main
	// OnRequestHeaders failure path) and assert: no panic, no error
	// returned/propagated, and HeadersToSet is empty - the caller
	// (internal/upstreamproc.Server) has no error channel to react to
	// anyway, per Global Constraints' fail-open requirement.
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd gateway-controllers/policies/oauth2-generator && go test ./... -run TestOAuth2GeneratorPolicy_OnUpstreamRequestHeaders -v`
Expected: FAIL — method doesn't exist.

- [ ] **Step 3: Implement**, appended to `oauth2_generator.go` near the existing `OnResponseHeaders`:

```go
// OnUpstreamRequestHeaders implements policy.UpstreamRequestHeaderPolicy.
// Unlike OnRequestHeaders (called once per client request, on the listener's
// filter chain), this runs once per individual upstream dial attempt,
// including Envoy-native retries - see UpstreamHeaderContext's doc comment.
// On AttemptCount == 1 it does nothing: OnRequestHeaders already attached a
// token for the first attempt. On AttemptCount > 1, it forces a fresh fetch
// via the same cached tokenSource this policy already built for
// OnRequestHeaders (Purge() + Token(), not a from-scratch client), so
// redis-backed sharing/caching semantics stay identical to the rest of this
// policy - this is not a second, independent credential-fetch path.
func (p *OAuth2GeneratorPolicy) OnUpstreamRequestHeaders(ctx context.Context, uctx *policy.UpstreamHeaderContext, params map[string]interface{}) policy.UpstreamHeaderAction {
	if uctx.AttemptCount <= 1 {
		return policy.UpstreamHeaderModifications{}
	}

	cp, err := extractCacheParams(params)
	if err != nil {
		return policy.UpstreamHeaderModifications{} // fail open - see Global Constraints
	}
	tokenSource, err := buildTokenSource(cp)
	if err != nil {
		return policy.UpstreamHeaderModifications{}
	}
	if purger, ok := tokenSource.(interface{ Purge() }); ok {
		purger.Purge() // force the fetch below past any cached-but-just-failed token
	}
	tok, err := tokenSource.Token()
	if err != nil || tok.AccessToken == "" {
		return policy.UpstreamHeaderModifications{}
	}
	return policy.UpstreamHeaderModifications{
		HeadersToSet: map[string]string{"Authorization": tok.Type() + " " + tok.AccessToken},
	}
}
```

> Confirm the exact `Purge()` interface/method name and the exact `extractCacheParams`/`buildTokenSource` signatures against the real current file before finalizing this — reuse whatever's already there rather than the illustrative names above if they differ.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd gateway-controllers/policies/oauth2-generator && go test ./... -run TestOAuth2GeneratorPolicy_OnUpstreamRequestHeaders -v`
Expected: PASS (all three)

- [ ] **Step 5: Run the full existing test suite for this policy to confirm no regression**

Run: `cd gateway-controllers/policies/oauth2-generator && go test ./... -v`
Expected: all PASS, including every pre-existing test from earlier sessions' work on this policy (token cache, redis client resolution, retry-on-timeout, etc.).

- [ ] **Step 6: Sync to the `dev-policies` mirror**

```bash
cp gateway-controllers/policies/oauth2-generator/oauth2_generator.go \
   gateway-controllers/policies/oauth2-generator/token_cache.go \
   gateway-controllers/policies/oauth2-generator/oauth2_generator_test.go \
   api-platform/gateway/dev-policies/oauth2-generator/
diff -rq gateway-controllers/policies/oauth2-generator/ api-platform/gateway/dev-policies/oauth2-generator/
```

Expected diff output: only the known `go.mod`/`e2e/` differences.

- [ ] **Step 7: Commit** (both repos)

```bash
# gateway-controllers
git add policies/oauth2-generator/oauth2_generator.go policies/oauth2-generator/oauth2_generator_test.go
git commit -m "feat: implement UpstreamRequestHeaderPolicy for per-retry credential refresh"

# api-platform
git add gateway/dev-policies/oauth2-generator/
git commit -m "sync: oauth2-generator refreshOnRetry implementation from gateway-controllers"
```

---

## Phase 5: End-to-end verification

### Task 10: Extend the mock IdP to issue a distinguishable token per call, and prove attempt 2 gets a different one

**Files:**
- Modify: `gateway/dev-policies/oauth2-generator/e2e/mocks/mock-oauth2-idp/main.go` (it already increments `tokenSeq` per issuance — confirm this and reuse it rather than adding a second counter)
- Modify: `gateway/dev-policies/resilience-retry/e2e/mocks/mock-backend/main.go` — extend to force `401` for the *first* `Authorization` value it sees per test flow, and `200` for any *different* one, so the test can assert "the backend actually received a NEW token on the retry," not just "some token was present twice."
- Create: a new Postman folder in `gateway/dev-policies/resilience-retry/e2e/postman/resilience-retry.postman_collection.json` (or a new dedicated collection if mixing oauth2 + retry concerns into the existing one gets unwieldy — prefer a new collection: `oauth2-upstream-refresh.postman_collection.json` alongside it, following this project's one-collection-per-feature convention already established by `oauth2.postman_collection.json` vs. `advanced-ratelimit-redis-precedence.postman_collection.json`).
- Create: `gateway/dev-policies/oauth2-generator/e2e/run-e2e.sh` gains a new chained call (same pattern as the existing `advanced-ratelimit`/`resilience-retry` chains) to this new suite's own `run-e2e.sh`, OR fold it directly into `resilience-retry/e2e/run-e2e.sh` as an additional phase if it shares that suite's mock-backend closely enough — decide based on how much mock-backend logic needs changing (Step 1 below) vs. how self-contained a new mock-backend copy would be; prefer extending the existing one if the change is additive and doesn't alter any of the three already-passing phases' behavior.

- [ ] **Step 1:** Extend `mock-backend/main.go`'s `handleAny` to compare the incoming `Authorization` header against a `firstSeenAuth` value (captured on the first request within a `configure-failures` window) and only apply the forced-failure status while `Authorization == firstSeenAuth` — once a *different* `Authorization` value arrives, treat it as success (`200`) regardless of the configured `failFirstN` counter. Reset `firstSeenAuth` in the existing `/debug/reset` handler. Also expose it via `/debug/stats` as `firstSeenAuth`/`lastSeenAuth` (masked to first/last 4 chars only, per this project's `GO-AUTH-003` masking convention already used in `mock-ai-backend.go`'s `maskToken`).
- [ ] **Step 2:** Register a new test API (or extend `resilience-retry-test`) with an operation attaching both `oauth2-generator{refreshOnRetry: true, tokenEndpoint: mock-oauth2-idp, ...}` and `resilience.retry{statusCodes: [401], numRetries: 1}`, upstream pointing at `mock-backend`.
- [ ] **Step 3:** Write the Postman flow:
  1. Reset `mock-oauth2-idp` and `mock-backend`.
  2. Configure `mock-backend` to reject whatever `Authorization` value it first sees (new mode from Step 1) with `401`.
  3. Call the route once.
  4. Assert client sees `200` (the retry, carrying a *different*, freshly-fetched token, succeeded).
  5. Assert `mock-oauth2-idp`'s `/debug/stats` shows **2** token issuances for this flow (one for the initial attach, one for the forced refresh on retry) — this is the ground-truth proof this whole feature exists to deliver, mirroring this project's established "don't trust client-visible status alone" convention from the `resilience-retry` and `advanced-ratelimit` e2e suites.
  6. Assert `mock-backend`'s `/debug/stats` shows `requestCount == 2` and `firstSeenAuth != lastSeenAuth`.
- [ ] **Step 4: Run the new/extended e2e script end-to-end** against the real Docker stack (same pattern as every other e2e script this session: rebuild `gateway-controller` and `gateway-runtime` images first, since this phase touches both, then `docker compose up -d gateway-controller gateway-runtime`, then run the script).

Run: `<path-to-script>/run-e2e.sh`
Expected: exit code 0, with the assertions from Step 3.5/3.6 both passing — this is the one check that actually proves the feature works, as opposed to merely compiling.

- [ ] **Step 5: Commit**

```bash
git add gateway/dev-policies/oauth2-generator/e2e/mocks/mock-oauth2-idp/main.go \
        gateway/dev-policies/resilience-retry/e2e/
git commit -m "test(e2e): prove oauth2-generator refreshOnRetry actually issues a new token on Envoy retry"
```

---

## Self-Review Notes (completed during authoring, kept for the implementer's context)

- **Spec coverage:** every piece of the agreed design — SDK types, registry, minimal upstream ext_proc server, socket wiring, xDS cluster-filter emission, `refreshOnRetry` param + validation, oauth2-generator's implementation, and e2e proof — has a task. The one deliberately-deferred design decision (Task 0) is called out explicitly rather than guessed at, because guessing it wrong would silently misplace Task 5/6's registration logic in the wrong layer.
- **No placeholders:** every code-bearing step has real Go/bash, not descriptions. The two spots using illustrative names (`getPolicyBoolParam`, `effectiveRetryStatusCodes` in Task 6; the exact fake-stream helper in Task 3) are flagged as "match the existing codebase's real name," which is a legitimate investigation instruction, not a skipped implementation — the surrounding code IS fully specified.
- **Type consistency:** `UpstreamHeaderContext`/`UpstreamHeaderModifications`/`UpstreamRequestHeaderPolicy` (Task 1) are used with identical field/method names through Tasks 3, 5, and 9 — checked by re-reading each task's code block against Task 1's definitions while writing this plan.

### Amendment log (post-Task-1, before Task 2 started)

Task 0 was executed and its findings (see the separate findings doc) resolved Task 5 to the lazy per-request design and elevated Task 8's collision check from "edge case" to "required, structurally reachable" — Task 5's text above already reflects this; Task 8 still needs to be read with that context when it's dispatched.

Task 1b was added after Task 1 was already reviewed and committed, following a design discussion: a rename of Task 1's types to a neutral "Attempt"-based name was considered and rejected (it would have broken this SDK's established one-phase-one-action-type convention, confirmed by reading `interface.go`/`action.go`'s existing `OnRequestBody`/`RequestAction`, `OnResponseBody`/`ResponseAction` pattern) — Task 1's original naming stands unchanged. Body-mutation support was added instead as the sibling `UpstreamRequestBodyPolicy` extension point, with query-param mutation confirmed to need no new code at all (`:path` is already mutable via the existing header action). This required updating, in-place, in this same plan file:
- Task 2's registry: `Register`/`Lookup` signatures changed to accept a plain `interface{}` and return both an optional header policy and an optional body policy (plus `Register` now returns an `error`). Every code block and test in Task 2 was rewritten to the new signature — this is NOT the version that was reviewed by Task 0's reviewer (Task 2 itself has not been reviewed yet as of this amendment).
- Task 3's server: added a `RequestBody` dispatch branch, per-stream `attemptCount`/`clusterName` state carried across `Recv()` calls, and a new synthetic-input-only test — clearly marked as unreachable by real traffic in this plan (Task 7 never requests `BUFFERED` body mode).
- Task 5: rewritten to drop the "eager" alternate path entirely (Task 0 resolved this) and updated its `Register(...)` call site to the new signature/error-handling.
- Global Constraints: added two bullets documenting the query/body scope decision.

**Whoever picks up Task 6 onward should re-confirm** that no other task text still references the old two-value `Register(clusterName, p, params)` / two-value `Lookup(clusterName)` signatures before writing code against it — grep for `\.Register(` / `\.Lookup(` in this file if in doubt.

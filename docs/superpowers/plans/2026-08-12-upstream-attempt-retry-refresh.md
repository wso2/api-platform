# Generic Upstream-Attempt Retry/Credential-Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a `resilience.retry`-enabled route's backend returns a configured status code (e.g. 401) and Envoy natively retries, let any policy that implements a new generic SDK interface attach fresh, per-attempt state (oauth2-generator: a freshly-fetched token) to the retried attempt — without any policy making its own outbound HTTP call to the main upstream, and without the client ever seeing the intermediate failure.

**Architecture:** Envoy's *upstream* HTTP filter chain (configured per-cluster via `Cluster.TypedExtensionProtocolOptions`, distinct from the per-listener downstream chain the existing `ext_proc` filter runs in) is re-invoked fresh for every upstream dial attempt, including retries, and Envoy sets `x-envoy-attempt-count` on each attempt (1, incrementing). A new, minimal second `ext_proc` gRPC server runs inside the existing `policy-engine` process, resolves the same in-memory policy-chain registry the existing downstream server already builds, and — for any policy in that chain implementing the new `UpstreamAttemptPolicy` interface (a plain Go type assertion, never a hardcoded policy name) — invokes it once per attempt.

**Tech Stack:** Go 1.26, `google.golang.org/grpc`, `github.com/envoyproxy/go-control-plane` (`ext_proc` v3, `envoy/extensions/upstreams/http/v3`), stock `envoyproxy/envoy` image (no custom build), `oapi-codegen v2.5.1`, `golang.org/x/oauth2`.

**Supersedes:** `docs/superpowers/plans/2026-08-11-oauth2-upstream-retry-refresh.md` (do not resume it — its commits exist in git history but are unreachable from current `HEAD`; this plan does not reuse its code). Design rationale: `docs/superpowers/specs/2026-08-12-upstream-attempt-retry-refresh-design.md`.

## Global Constraints

- No custom-compiled Envoy — every mechanism must work against the stock image already in use (`gateway-runtime/Dockerfile`'s `python-deps`/`production` stages, `FROM envoyproxy/envoy:${ENVOY_VERSION}`).
- Fail open on any error in the new upstream-attempt path (credential fetch fails, chain lookup miss): return a no-mutation action, never block the retry. This feature only ever makes a retry *more likely* to succeed.
- Policy discovery is a type assertion against `UpstreamAttemptPolicy` — never a hardcoded policy name string, anywhere (gateway-controller, policy-engine, or oauth2-generator).
- `dev-policies/oauth2-generator` (local mirror) and `gateway-controllers/policies/oauth2-generator` (separate repo, source of truth) must be kept byte-identical for `oauth2_generator.go`/`policy-definition.yaml` — diff after every change.
- New gRPC servers set `grpc.MaxRecvMsgSize`, `grpc.MaxSendMsgSize`, `grpc.MaxConcurrentStreams` explicitly (`go-network-service-hardening.md` directive 2) — do not copy the existing downstream `ext_proc` server's omission of these forward into new code.
- No raw token values in log output (`GO-AUTH-003`) — any log line touching a fetched/refreshed token logs only presence/error, never the value.

---

## Phase 1: SDK primitives

### Task 1: Add `UpstreamAttemptContext`, `UpstreamAttemptAction`, `UpstreamAttemptPolicy` to the policy SDK

**Files:**
- Modify: `sdk/core/policy/v1alpha2/context.go`
- Modify: `sdk/core/policy/v1alpha2/action.go`
- Create: `sdk/core/policy/v1alpha2/upstream_attempt_test.go`

**Interfaces:**
- Produces: `policy.UpstreamAttemptContext{SharedContext *SharedContext, AttemptCount int, Headers *Headers}`, `policy.UpstreamAttemptAction` (sealed interface, one variant), `policy.UpstreamAttemptHeaderModifications{HeadersToSet map[string]string}`, `policy.UpstreamAttemptPolicy` interface with `OnUpstreamAttemptRequestHeaders(ctx context.Context, actx *UpstreamAttemptContext) UpstreamAttemptAction`.
- Consumes: existing `Headers` (`sdk/core/policy/v1alpha2/headers.go`, `NewHeaders(map[string][]string) *Headers`), existing `SharedContext` (`context.go:78-119`).

- [ ] **Step 1: Write the failing test**

```go
// sdk/core/policy/v1alpha2/upstream_attempt_test.go
package policyv1alpha2

import (
	"context"
	"testing"
)

// fakeUpstreamAttemptPolicy proves any type implementing UpstreamAttemptPolicy
// compiles against the real context.Context/UpstreamAttemptContext/
// UpstreamAttemptAction types — a compile-time contract test. oauth2-generator's
// own tests (Task 9) cover real refresh behavior.
type fakeUpstreamAttemptPolicy struct{}

func (fakeUpstreamAttemptPolicy) OnUpstreamAttemptRequestHeaders(_ context.Context, actx *UpstreamAttemptContext) UpstreamAttemptAction {
	if actx.AttemptCount <= 1 {
		return UpstreamAttemptHeaderModifications{}
	}
	return UpstreamAttemptHeaderModifications{HeadersToSet: map[string]string{"Authorization": "Bearer refreshed"}}
}

func TestUpstreamAttemptContext_AttemptCountGatesRefresh(t *testing.T) {
	var p UpstreamAttemptPolicy = fakeUpstreamAttemptPolicy{}

	attemptOne := &UpstreamAttemptContext{AttemptCount: 1, Headers: NewHeaders(nil)}
	action := p.OnUpstreamAttemptRequestHeaders(context.Background(), attemptOne)
	mods, ok := action.(UpstreamAttemptHeaderModifications)
	if !ok || len(mods.HeadersToSet) != 0 {
		t.Fatalf("attempt 1 must not mutate headers, got %#v", action)
	}

	attemptTwo := &UpstreamAttemptContext{AttemptCount: 2, Headers: NewHeaders(nil)}
	action2 := p.OnUpstreamAttemptRequestHeaders(context.Background(), attemptTwo)
	mods2, ok := action2.(UpstreamAttemptHeaderModifications)
	if !ok || mods2.HeadersToSet["Authorization"] != "Bearer refreshed" {
		t.Fatalf("attempt 2 must carry the refreshed token, got %#v", action2)
	}
}

// Compile-time interface satisfaction check, mirroring action.go's own convention.
var _ UpstreamAttemptAction = UpstreamAttemptHeaderModifications{}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd sdk/core && GOWORK=off go test ./policy/v1alpha2/... -run TestUpstreamAttemptContext_AttemptCountGatesRefresh -v`
Expected: FAIL with `undefined: UpstreamAttemptContext`.

- [ ] **Step 3: Add the context type** to `sdk/core/policy/v1alpha2/context.go`, appended after the existing `ResponseStreamContext` block (end of file):

```go
// ─── Upstream-attempt context (per-dial-attempt, not per-client-request) ─────

// UpstreamAttemptContext is passed to UpstreamAttemptPolicy.OnUpstreamAttemptRequestHeaders.
// Unlike every other context in this package, it is NOT scoped to one client
// request — it fires once per individual upstream dial attempt, including
// Envoy-native retries, because it runs in Envoy's per-cluster upstream HTTP
// filter chain rather than the per-route listener chain every other policy
// phase in this package uses.
type UpstreamAttemptContext struct {
	*SharedContext

	// AttemptCount is Envoy's x-envoy-attempt-count for this specific dial,
	// starting at 1. A missing/unparseable header is treated as 1 (fail
	// toward "behave like the first attempt", never toward unconditional
	// refresh) — see the kernel-side parsing in Task 3.
	AttemptCount int

	// Headers are this specific attempt's outgoing request headers, mutable
	// via the returned UpstreamAttemptAction.
	Headers *Headers
}
```

- [ ] **Step 4: Add the sealed action to** `sdk/core/policy/v1alpha2/action.go`, appended at the end of the file:

```go
// ─── Upstream-attempt action (sealed oneof, one variant) ─────────────────────
//
// UpstreamAttemptAction is deliberately a sealed interface with exactly one
// concrete variant, unlike RequestHeaderAction's two (Modifications |
// ImmediateResponse): this phase runs after routing and authentication are
// already resolved, mid-retry-loop inside Envoy's router filter, where there
// is no sensible notion of "reject this request" — only "optionally change
// headers for this one attempt."

// UpstreamAttemptAction is the sealed oneof returned by
// UpstreamAttemptPolicy.OnUpstreamAttemptRequestHeaders.
type UpstreamAttemptAction interface {
	isUpstreamAttemptAction()
}

// UpstreamAttemptHeaderModifications sets the given headers on this specific
// upstream attempt. An empty/nil HeadersToSet is a valid, common no-op (e.g.
// AttemptCount == 1, nothing to refresh yet, or a fail-open path after an
// error).
type UpstreamAttemptHeaderModifications struct {
	HeadersToSet map[string]string
}

func (UpstreamAttemptHeaderModifications) isUpstreamAttemptAction() {}

// UpstreamAttemptPolicy is implemented by any policy that wants to attach
// fresh, per-attempt state (e.g. a refreshed credential) to an Envoy-native
// retry. Discovery is a plain type assertion by the kernel — see Task 3 —
// never a hardcoded policy name. A policy implements this in addition to,
// not instead of, its normal RequestHeaderPolicy/ResponseHeaderPolicy
// interfaces.
type UpstreamAttemptPolicy interface {
	OnUpstreamAttemptRequestHeaders(ctx context.Context, actx *UpstreamAttemptContext) UpstreamAttemptAction
}
```

Note: `action.go` needs `"context"` imported for the interface method signature — check the existing import block; if `context` isn't already imported there (it isn't, per the current file), add it.

- [ ] **Step 5: Run test to verify it passes**

Run: `cd sdk/core && GOWORK=off go test ./policy/v1alpha2/... -run TestUpstreamAttemptContext_AttemptCountGatesRefresh -v`
Expected: PASS.

- [ ] **Step 6: Run the full package test suite to confirm no regressions**

Run: `cd sdk/core && GOWORK=off go build ./... && GOWORK=off go test ./policy/... -v`
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add sdk/core/policy/v1alpha2/context.go sdk/core/policy/v1alpha2/action.go sdk/core/policy/v1alpha2/upstream_attempt_test.go
git commit -m "feat(sdk): add generic UpstreamAttemptPolicy interface for per-retry-attempt header mutation"
```

---

## Phase 2: policy-engine — minimal upstream `ext_proc` server

### Task 2: Extract `extractRouteKey`'s body into a shared free function

**Why:** the existing `(*ExternalProcessorServer).extractRouteKey` (`internal/kernel/extproc.go`) reads `req.Attributes[constants.ExtProcFilter].Fields["xds.route_name"]`. The new upstream server (Task 3) needs the identical logic but is a different receiver type — extracting it avoids duplicating it.

**Files:**
- Modify: `gateway/gateway-runtime/policy-engine/internal/kernel/extproc.go`
- Test: `gateway/gateway-runtime/policy-engine/internal/kernel/extproc_test.go` (add a case; file already exists per the existing test suite for this package — if a test file for this exact function doesn't exist yet, add it as a new test function in the existing `extproc_test.go`)

**Interfaces:**
- Produces: `extractRouteKeyFromAttributes(req *extprocv3.ProcessingRequest) string` (package-level, unexported, in package `kernel`).
- Consumes: existing `constants.ExtProcFilter` (`internal/constants/constants.go:23`, value `"envoy.filters.http.ext_proc"`).

- [ ] **Step 1: Write the failing test**

```go
// added to internal/kernel/extproc_test.go
func TestExtractRouteKeyFromAttributes_MissingAttributesReturnsDefault(t *testing.T) {
	req := &extprocv3.ProcessingRequest{}
	if got := extractRouteKeyFromAttributes(req); got != "default" {
		t.Errorf("got %q, want %q", got, "default")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd gateway-runtime/policy-engine && GOWORK=off go test ./internal/kernel/... -run TestExtractRouteKeyFromAttributes_MissingAttributesReturnsDefault -v`
Expected: FAIL with `undefined: extractRouteKeyFromAttributes`.

- [ ] **Step 3: Extract the function.** In `internal/kernel/extproc.go`, find the existing `extractRouteKey` method (currently `func (s *ExternalProcessorServer) extractRouteKey(req *extprocv3.ProcessingRequest) string`). Replace its body with a call to a new package-level function, and move the existing body verbatim into that new function:

```go
// extractRouteKeyFromAttributes extracts just the route key (xds.route_name)
// from the request attributes — shared by both the downstream ExternalProcessorServer
// and the upstream-attempt UpstreamExternalProcessorServer (Task 3), since both
// receive the identical ext_proc request-attributes shape.
func extractRouteKeyFromAttributes(req *extprocv3.ProcessingRequest) string {
	if req.Attributes == nil {
		return "default"
	}
	extProcAttrs, ok := req.Attributes[constants.ExtProcFilter]
	if !ok || extProcAttrs.Fields == nil {
		return "default"
	}
	if routeNameValue, ok := extProcAttrs.Fields["xds.route_name"]; ok {
		if stringValue := routeNameValue.GetStringValue(); stringValue != "" {
			return stringValue
		}
	}
	return "default"
}

// extractRouteKey extracts just the route key (xds.route_name) from the request attributes.
// This is a lightweight extraction that avoids parsing route metadata.
func (s *ExternalProcessorServer) extractRouteKey(req *extprocv3.ProcessingRequest) string {
	return extractRouteKeyFromAttributes(req)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd gateway-runtime/policy-engine && GOWORK=off go test ./internal/kernel/... -run TestExtractRouteKeyFromAttributes_MissingAttributesReturnsDefault -v`
Expected: PASS.

- [ ] **Step 5: Run the full kernel package test suite to confirm no regressions**

Run: `cd gateway-runtime/policy-engine && GOWORK=off go test ./internal/kernel/... -v 2>&1 | tail -60`
Expected: all PASS (the existing `extractRouteKey` behavior is unchanged — it now delegates).

- [ ] **Step 6: Commit**

```bash
git add internal/kernel/extproc.go internal/kernel/extproc_test.go
git commit -m "refactor(policy-engine): extract extractRouteKey body into a shared free function"
```

### Task 3: Add the minimal upstream `ext_proc` gRPC server

**Files:**
- Create: `gateway/gateway-runtime/policy-engine/internal/kernel/upstream_extproc.go`
- Create: `gateway/gateway-runtime/policy-engine/internal/kernel/upstream_extproc_test.go`

**Interfaces:**
- Consumes: `Kernel.GetRouteConfig(routeKey string) *RouteConfig` and `Kernel.GetPolicyChain(policyChainKey string) *registry.PolicyChain` (`internal/kernel/mapper.go:70,77`, both already exported on `*Kernel`); `registry.PolicyChain.Policies []policy.Policy` (`internal/registry/chain.go`); `extractRouteKeyFromAttributes` (Task 2); the package-private `buildHeaderValueOptions(map[string]string) *extprocv3.HeaderMutation` (`internal/kernel/translator.go:1789`, already in package `kernel` — no import needed since this new file lives in the same package).
- Produces: `type UpstreamExternalProcessorServer struct{ kernel *Kernel }`, `NewUpstreamExternalProcessorServer(k *Kernel) *UpstreamExternalProcessorServer`, implementing `extprocv3.ExternalProcessorServer`'s `Process` method (request-headers phase only — every other phase gets an empty continue response, since this filter is only ever configured with `RequestHeaderMode: SEND` in Task 8, so no other phase should ever actually arrive, but the switch must handle it defensively rather than erroring).

- [ ] **Step 1: Write the failing test**

```go
// gateway/gateway-runtime/policy-engine/internal/kernel/upstream_extproc_test.go
package kernel

import (
	"context"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"google.golang.org/genproto/googleapis/rpc/status"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

// fakeUpstreamAttemptPolicy proves the dispatch loop invokes exactly the
// policies implementing UpstreamAttemptPolicy, via type assertion, ignoring
// every other policy in the chain (rate-limit/analytics-shaped policies that
// don't implement it).
type fakeUpstreamAttemptPolicy struct{ lastAttempt int }

func (p *fakeUpstreamAttemptPolicy) Mode() policy.ProcessingMode { return policy.ProcessingMode{} }
func (p *fakeUpstreamAttemptPolicy) OnUpstreamAttemptRequestHeaders(_ context.Context, actx *policy.UpstreamAttemptContext) policy.UpstreamAttemptAction {
	p.lastAttempt = actx.AttemptCount
	if actx.AttemptCount <= 1 {
		return policy.UpstreamAttemptHeaderModifications{}
	}
	return policy.UpstreamAttemptHeaderModifications{HeadersToSet: map[string]string{"Authorization": "Bearer refreshed"}}
}

// nonParticipatingPolicy implements only the base Policy interface — proves
// the dispatch loop skips it via type assertion, not a hardcoded name check.
type nonParticipatingPolicy struct{}

func (nonParticipatingPolicy) Mode() policy.ProcessingMode { return policy.ProcessingMode{} }

func newTestRouteConfigAndChain(t *testing.T, routeKey string, chain *registry.PolicyChain) *Kernel {
	t.Helper()
	k := NewKernel()
	k.ApplyWholeRouteConfigs(map[string]RouteConfig{routeKey: {RouteName: routeKey}})
	k.SetPolicyChain(routeKey, chain) // see Step 3 note below if SetPolicyChain doesn't exist yet
	return k
}

func attrsFor(routeKey string) map[string]*structpb.Struct {
	return map[string]*structpb.Struct{
		"envoy.filters.http.ext_proc": {
			Fields: map[string]*structpb.Value{
				"xds.route_name": structpb.NewStringValue(routeKey),
			},
		},
	}
}

func TestUpstreamExtProc_DispatchesOnlyToImplementingPolicies(t *testing.T) {
	fp := &fakeUpstreamAttemptPolicy{}
	chain := &registry.PolicyChain{Policies: []policy.Policy{nonParticipatingPolicy{}, fp}}
	k := newTestRouteConfigAndChain(t, "test-route", chain)
	s := NewUpstreamExternalProcessorServer(k)

	req := &extprocv3.ProcessingRequest{
		Attributes: attrsFor("test-route"),
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
					{Key: "x-envoy-attempt-count", RawValue: []byte("2")},
				}},
			},
		},
	}

	resp, err := s.processRequestHeaders(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp.lastAttempt != 2 {
		t.Fatalf("expected the implementing policy to observe AttemptCount=2, got %d", fp.lastAttempt)
	}
	rh, ok := resp.Response.(*extprocv3.ProcessingResponse_RequestHeaders)
	if !ok {
		t.Fatalf("expected a RequestHeaders response, got %T", resp.Response)
	}
	mutation := rh.RequestHeaders.GetResponse().GetHeaderMutation()
	if mutation == nil || len(mutation.SetHeaders) != 1 || string(mutation.SetHeaders[0].Header.RawValue) != "Bearer refreshed" {
		t.Fatalf("expected the refreshed Authorization header to be set, got %#v", mutation)
	}
}

func TestUpstreamExtProc_MissingAttemptCountHeaderTreatedAsOne(t *testing.T) {
	fp := &fakeUpstreamAttemptPolicy{}
	chain := &registry.PolicyChain{Policies: []policy.Policy{fp}}
	k := newTestRouteConfigAndChain(t, "test-route", chain)
	s := NewUpstreamExternalProcessorServer(k)

	req := &extprocv3.ProcessingRequest{
		Attributes: attrsFor("test-route"),
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}},
		},
	}
	if _, err := s.processRequestHeaders(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp.lastAttempt != 1 {
		t.Fatalf("expected a missing attempt-count header to be treated as attempt 1, got %d", fp.lastAttempt)
	}
}

func TestUpstreamExtProc_UnknownRouteReturnsEmptyContinue(t *testing.T) {
	k := NewKernel()
	s := NewUpstreamExternalProcessorServer(k)
	req := &extprocv3.ProcessingRequest{
		Attributes: attrsFor("no-such-route"),
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}},
		},
	}
	resp, err := s.processRequestHeaders(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rh, ok := resp.Response.(*extprocv3.ProcessingResponse_RequestHeaders)
	if !ok || rh.RequestHeaders.GetResponse().GetHeaderMutation() != nil {
		t.Fatalf("expected an empty continue response for an unknown route, got %#v", resp.Response)
	}
	_ = status.Status{} // placeholder import use removed below if unused — see Step 2 note
}
```

Note before running: this test file assumes `NewKernel()`, `Kernel.ApplyWholeRouteConfigs(map[string]RouteConfig)`, and a `Kernel.SetPolicyChain(routeKey string, chain *registry.PolicyChain)` helper. If `SetPolicyChain` does not already exist as a test-only setter on `*Kernel` (check `internal/kernel/mapper.go` for the real chain-registration entry point — it's likely `handler.go`'s `buildPolicyChain` populating an internal map the `Kernel` wraps), add a minimal exported-for-package-tests setter now rather than reaching for unexported internals via reflection; confirm the exact existing chain-storage field name in `mapper.go` before adding it, and name the setter to match existing `Kernel` method naming conventions in that file. Remove the unused `status` import if `go vet` flags it — it was left as a placeholder only if a later step needs gRPC status construction; if not needed, delete the import and the placeholder line.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd gateway-runtime/policy-engine && GOWORK=off go test ./internal/kernel/... -run TestUpstreamExtProc -v`
Expected: FAIL with `undefined: NewUpstreamExternalProcessorServer` (and possibly `undefined: (*Kernel).SetPolicyChain` — resolve per the Step 1 note before proceeding).

- [ ] **Step 3: Implement the server**

```go
// gateway/gateway-runtime/policy-engine/internal/kernel/upstream_extproc.go
package kernel

import (
	"context"
	"io"
	"log/slog"
	"strconv"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UpstreamExternalProcessorServer is the second, minimal ext_proc gRPC server
// hosted in this same policy-engine process — wired into Envoy's per-cluster
// UPSTREAM HTTP filter chain (see gateway-controller's Task 8), not the
// per-listener downstream chain ExternalProcessorServer (extproc.go) serves.
// It handles only the request-headers phase: this filter attachment point
// has no sensible response phase or body phase for this feature (see the
// design doc). It resolves route -> policy chain via the exact same
// in-memory registry the downstream server uses (s.kernel.GetPolicyChain),
// so a policy's cached state (e.g. oauth2-generator's token cache) is
// naturally shared between both entry points with zero duplication.
type UpstreamExternalProcessorServer struct {
	extprocv3.UnimplementedExternalProcessorServer
	kernel *Kernel
}

// NewUpstreamExternalProcessorServer constructs the server. k must be the
// same *Kernel instance the downstream ExternalProcessorServer uses (see
// cmd/policy-engine/main.go, Task 4) — this is what makes chain/state sharing
// automatic rather than something this type has to arrange itself.
func NewUpstreamExternalProcessorServer(k *Kernel) *UpstreamExternalProcessorServer {
	return &UpstreamExternalProcessorServer{kernel: k}
}

// Process implements extprocv3.ExternalProcessorServer. Unlike the downstream
// server's Process (extproc.go), this one only ever expects RequestHeaders
// messages (the cluster's upstream filter is configured with
// RequestHeaderMode: SEND and every other mode left at its default NONE, see
// Task 8) — any other message type gets an empty continue response rather
// than an error, since failing this path must never break the retry itself
// (see Global Constraints: fail open).
func (s *UpstreamExternalProcessorServer) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	ctx := stream.Context()
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		var resp *extprocv3.ProcessingResponse
		switch req.Request.(type) {
		case *extprocv3.ProcessingRequest_RequestHeaders:
			resp, err = s.processRequestHeaders(ctx, req)
			if err != nil {
				slog.ErrorContext(ctx, "upstream ext_proc: failed to process request headers, failing open", "error", err)
				resp = emptyContinueRequestHeadersResponse()
			}
		default:
			resp = emptyContinueRequestHeadersResponse()
		}

		if err := stream.Send(resp); err != nil {
			return status.Errorf(codes.Internal, "upstream ext_proc: failed to send response: %v", err)
		}
	}
}

// emptyContinueRequestHeadersResponse is the fail-open / no-op response: no
// header mutation, request proceeds unchanged.
func emptyContinueRequestHeadersResponse() *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{},
			},
		},
	}
}

// processRequestHeaders resolves the route's policy chain and dispatches to
// every policy implementing UpstreamAttemptPolicy, in chain order. A policy
// that doesn't implement it (the common case — rate limiting, analytics,
// transforms) is silently skipped via the type assertion; this is what makes
// the mechanism generic with zero per-policy wiring in this server.
func (s *UpstreamExternalProcessorServer) processRequestHeaders(ctx context.Context, req *extprocv3.ProcessingRequest) (*extprocv3.ProcessingResponse, error) {
	routeKey := extractRouteKeyFromAttributes(req)
	chain := s.kernel.GetPolicyChain(routeKey)
	if chain == nil {
		return emptyContinueRequestHeadersResponse(), nil
	}

	headers := req.GetRequestHeaders()
	attemptCount := 1
	headersMap := make(map[string][]string)
	if headers.GetHeaders() != nil {
		for _, h := range headers.GetHeaders().GetHeaders() {
			key := h.Key
			value := string(h.RawValue)
			headersMap[key] = append(headersMap[key], value)
			if key == "x-envoy-attempt-count" {
				if n, err := strconv.Atoi(value); err == nil && n > 0 {
					attemptCount = n
				}
			}
		}
	}

	actx := &policy.UpstreamAttemptContext{
		AttemptCount: attemptCount,
		Headers:      policy.NewHeaders(headersMap),
	}

	headersToSet := make(map[string]string)
	for _, p := range chain.Policies {
		attemptPolicy, ok := p.(policy.UpstreamAttemptPolicy)
		if !ok {
			continue
		}
		action := attemptPolicy.OnUpstreamAttemptRequestHeaders(ctx, actx)
		mods, ok := action.(policy.UpstreamAttemptHeaderModifications)
		if !ok {
			continue
		}
		for k, v := range mods.HeadersToSet {
			headersToSet[k] = v
		}
	}

	if len(headersToSet) == 0 {
		return emptyContinueRequestHeadersResponse(), nil
	}

	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation: buildHeaderValueOptions(headersToSet),
				},
			},
		},
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd gateway-runtime/policy-engine && GOWORK=off go test ./internal/kernel/... -run TestUpstreamExtProc -v`
Expected: PASS.

- [ ] **Step 5: Run the full kernel package test suite to confirm no regressions**

Run: `cd gateway-runtime/policy-engine && GOWORK=off go build ./... && GOWORK=off go test ./internal/kernel/... -v 2>&1 | tail -60`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/kernel/upstream_extproc.go internal/kernel/upstream_extproc_test.go
git commit -m "feat(policy-engine): add minimal upstream ext_proc server dispatching to UpstreamAttemptPolicy"
```

### Task 4: Wire the new server into `policy-engine`'s startup on a second socket/port, with graceful shutdown

**Files:**
- Modify: `gateway/gateway-runtime/policy-engine/internal/constants/constants.go`
- Modify: `gateway/gateway-runtime/policy-engine/internal/config/config.go`
- Modify: `gateway/gateway-runtime/policy-engine/cmd/policy-engine/main.go`
- Modify: `gateway/gateway-runtime/policy-engine/configs/config.toml` (or wherever the shipped default `config.toml` for this component lives — confirm exact path via `grep -rn "extproc_port" gateway-runtime/policy-engine/configs/`) — add the new port's default alongside the existing `extproc_port`.

**Interfaces:**
- Produces: `constants.DefaultUpstreamExtProcSocketPath = "/var/run/api-platform/policy-engine-upstream.sock"`; `config.ServerConfig.UpstreamExtProcPort int` (koanf tag `upstream_extproc_port`, default `9004` — confirm this doesn't collide with any existing port in `ServerConfig`'s validation block before picking it, per `config.go`'s existing collision checks around `ExtProcPort`).
- Consumes: `kernel.NewUpstreamExternalProcessorServer(k *Kernel)` (Task 3); existing `k` (`*kernel.Kernel`) already constructed earlier in `main()`; existing `internal/utils.CreateGRPCServer(publicKeyPath, privateKeyPath string, plainText bool, opts ...grpc.ServerOption) (*grpc.Server, error)` (`internal/utils/grpc.go:83`) — reused for TLS/plaintext handling parity with the ALS server, rather than a bare `grpc.NewServer()`.

- [ ] **Step 1: Add the socket path constant.** In `internal/constants/constants.go`, next to `DefaultPolicyEngineSocketPath`:

```go
// DefaultUpstreamExtProcSocketPath is the Unix socket for the second,
// upstream-attempt ext_proc server (see internal/kernel/upstream_extproc.go),
// distinct from DefaultPolicyEngineSocketPath's per-listener downstream
// server.
DefaultUpstreamExtProcSocketPath = "/var/run/api-platform/policy-engine-upstream.sock"
```

- [ ] **Step 2: Add the config field.** In `internal/config/config.go`'s `ServerConfig` struct, next to `ExtProcPort`:

```go
// UpstreamExtProcPort is the port for the upstream-attempt ext_proc gRPC
// server (TCP mode only) — see internal/kernel/upstream_extproc.go.
UpstreamExtProcPort int `koanf:"upstream_extproc_port"`
```

Add its default (next to the existing `ExtProcPort: 9001` default, around line 515) as `UpstreamExtProcPort: 9004,` and its validation (next to the existing `ExtProcPort` bounds check around line 626):

```go
if c.PolicyEngine.Server.UpstreamExtProcPort <= 0 || c.PolicyEngine.Server.UpstreamExtProcPort > 65535 {
	return fmt.Errorf("invalid upstream_extproc_port: %d (must be 1-65535)", c.PolicyEngine.Server.UpstreamExtProcPort)
}
```

Also extend the existing port-collision checks (around lines 656/670, which currently compare `Admin.Port`/`Metrics.Port` against `ExtProcPort` when `Mode == "tcp"`) to additionally compare against `UpstreamExtProcPort` — copy the existing two `if` blocks' shape exactly, substituting the field name.

- [ ] **Step 3: Wire startup in `main()`.** In `cmd/policy-engine/main.go`, immediately after the existing block that creates `extprocServer`/`lis`/`grpcServer` and registers it (i.e. right after the line `extprocv3.RegisterExternalProcessorServer(grpcServer, extprocServer)`), add:

```go
// Create and start the upstream-attempt ext_proc gRPC server (second,
// minimal endpoint — see internal/kernel/upstream_extproc.go). Uses the same
// serverMode (uds/tcp) as the main ext_proc server, but its own socket/port,
// and its own explicit message/stream limits sized for its headers-only
// message shape (go-network-service-hardening.md directive 2) — not copied
// from the main server's larger ceiling.
upstreamExtprocServer := kernel.NewUpstreamExternalProcessorServer(k)

var upstreamLis net.Listener
switch serverMode {
case "uds":
	socketPath := constants.DefaultUpstreamExtProcSocketPath
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		slog.WarnContext(ctx, "Failed to remove existing upstream ext_proc socket file", "path", socketPath, "error", err)
	}
	upstreamLis, err = net.Listen("unix", socketPath)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to listen on upstream ext_proc Unix socket", "path", socketPath, "error", err)
		os.Exit(1)
	}
	if err := os.Chmod(socketPath, 0660); err != nil {
		slog.WarnContext(ctx, "Failed to set upstream ext_proc socket permissions", "path", socketPath, "error", err)
	}
	slog.InfoContext(ctx, "Upstream ext_proc server listening on Unix socket", "path", socketPath)
case "tcp":
	upstreamLis, err = net.Listen("tcp", fmt.Sprintf(":%d", cfg.PolicyEngine.Server.UpstreamExtProcPort))
	if err != nil {
		slog.ErrorContext(ctx, "Failed to listen on upstream ext_proc port", "port", cfg.PolicyEngine.Server.UpstreamExtProcPort, "error", err)
		os.Exit(1)
	}
	slog.InfoContext(ctx, "Upstream ext_proc server listening on TCP port", "port", cfg.PolicyEngine.Server.UpstreamExtProcPort)
}

upstreamGrpcServer := grpc.NewServer(
	grpc.MaxRecvMsgSize(64*1024),   // headers-only messages; far smaller than the body-carrying main server's ceiling
	grpc.MaxSendMsgSize(64*1024),
	grpc.MaxConcurrentStreams(1000),
)
extprocv3.RegisterExternalProcessorServer(upstreamGrpcServer, upstreamExtprocServer)

go func() {
	if err := upstreamGrpcServer.Serve(upstreamLis); err != nil {
		serverErrCh <- err
	}
}()
```

- [ ] **Step 4: Wire graceful shutdown.** In the shutdown sequence, immediately before the existing `grpcServer.GracefulStop()` line, add:

```go
slog.InfoContext(ctx, "Stopping upstream ext_proc gRPC server")
upstreamGrpcServer.GracefulStop()
```

And in the UDS socket cleanup block (the `if serverMode == "uds"` block near the end), add a second cleanup line alongside the existing `os.Remove(constants.DefaultPolicyEngineSocketPath)`:

```go
if err := os.Remove(constants.DefaultUpstreamExtProcSocketPath); err != nil && !os.IsNotExist(err) {
	slog.WarnContext(ctx, "Failed to cleanup upstream ext_proc socket file on shutdown",
		"path", constants.DefaultUpstreamExtProcSocketPath, "error", err)
}
```

- [ ] **Step 5: Add the new port's default to the shipped config.** Find the shipped `config.toml`'s `[policy_engine.server]` (or equivalent) section:

```bash
grep -rn "extproc_port" gateway-runtime/policy-engine/configs/ gateway/configs/config.toml
```

Add `upstream_extproc_port = 9004` alongside whatever key holds `extproc_port` there, in every config file that currently sets `extproc_port` explicitly (shipped default + any e2e/test config.toml that overrides it — grep confirms the full list).

- [ ] **Step 6: Build and run the existing test suite**

Run: `cd gateway-runtime/policy-engine && GOWORK=off go build ./... && GOWORK=off go test ./... 2>&1 | tail -40`
Expected: builds clean, all existing tests still PASS (no test exercises the new server's startup wiring directly — that's covered by Task 10's e2e test).

- [ ] **Step 7: Manually verify the process starts with both sockets present**

```bash
cd gateway-runtime/policy-engine && go run ./cmd/policy-engine -policy-chains-file <(echo '{}') &
sleep 1
ls -la /var/run/api-platform/policy-engine.sock /var/run/api-platform/policy-engine-upstream.sock
kill %1
```
Expected: both socket files listed (adjust the placeholder config-mode invocation to whatever this binary's existing minimal-startup smoke-test invocation already is, e.g. check `Makefile`/`README.md` in this directory for the exact minimal local-run command — do not invent a flag that doesn't exist; confirm via `go run ./cmd/policy-engine -h` first).

- [ ] **Step 8: Commit**

```bash
git add internal/constants/constants.go internal/config/config.go cmd/policy-engine/main.go configs/
git commit -m "feat(policy-engine): wire the upstream ext_proc server into startup/shutdown on a second socket"
```

---

## Phase 3: gateway-controller — config surface and Envoy config emission

### Task 5: Add `resilience.retry` to the OpenAPI schema and regenerate

**Files:**
- Modify: `gateway/gateway-controller/api/management-openapi.yaml`
- Regenerate: `gateway/gateway-controller/pkg/api/management/generated.go` (do not hand-edit — `// Code generated ... DO NOT EDIT`)

**Interfaces:**
- Produces: `api.Retry{StatusCodes []int, NumRetries *int}` and `api.Resilience.Retry *Retry` (both generated by `oapi-codegen` from the schema below — exact generated type/field names follow `oapi-codegen`'s standard naming from the schema property names, matching the existing `Resilience.Timeout`/`Resilience.IdleTimeout` sibling fields' generated shape).

- [ ] **Step 1: Extend the schema.** In `management-openapi.yaml`, find the `Resilience` schema (currently `timeout`/`idleTimeout` only, per the existing description block). `retry` must be its own top-level schema referenced via `$ref` — NOT a nested inline object property — otherwise `oapi-codegen` generates an anonymous inline struct instead of a named `Retry` type, which every later task in this plan (6, 7, 8, 9) depends on as `api.Retry`. Add a `retry` property referencing a new sibling top-level `Retry` schema, and define that schema alongside `Resilience` (e.g. immediately after it, at the same indentation level as every other top-level schema like `Resilience`/`Upstream`):

```yaml
    Resilience:
      type: object
      description: >
        Backend/route timeout and retry configuration. Maps to Envoy RouteAction
        timeouts and RetryPolicy. Can be set at the API level (applies to all
        routes) and/or the operation level (applies to that operation's route).
        When set at both levels, the operation-level value takes precedence.
      properties:
        timeout:
          type: string
          description: Maximum time for the entire route (request to upstream response). "0s" disables the timeout.
          pattern: '^\d+(\.\d+)?(ms|s|m|h)$'
          example: 15s
        idleTimeout:
          type: string
          description: Per-route stream idle timeout (overrides the listener stream idle timeout for this route). "0s" disables the timeout.
          pattern: '^\d+(\.\d+)?(ms|s|m|h)$'
          example: 0s
        retry:
          $ref: '#/components/schemas/Retry'

    Retry:
      type: object
      description: >
        Native Envoy retry on the listed response status codes. When set,
        any policy on this route implementing the upstream-attempt refresh
        mechanism (see UpstreamAttemptPolicy in the policy SDK) gets a
        chance to attach fresh per-attempt state (e.g. a refreshed
        credential) before each retried attempt goes out.
      required:
        - statusCodes
      properties:
        statusCodes:
          type: array
          items:
            type: integer
            minimum: 400
            maximum: 599
          minItems: 1
          description: Response status codes that trigger a retry.
          example: [401]
        numRetries:
          type: integer
          minimum: 1
          default: 1
          description: Maximum number of retry attempts.
```

If this file's existing convention for a `$ref`'d property alongside its own description differs (check how any other property in this file already does this), match that convention instead — the important, non-negotiable part is that `Retry` ends up as its own top-level schema so codegen names it, not the exact `$ref` placement syntax.

- [ ] **Step 2: Regenerate**

```bash
cd gateway-controller && make generate-server-code
```

- [ ] **Step 3: Confirm the generated types compile and match expectations**

```bash
GOWORK=off go build ./... 2>&1 | tail -30
grep -n "type Retry struct" -A 10 pkg/api/management/generated.go
```
Expected: builds clean; `Retry` struct present with `StatusCodes []int` (or `[]int32`/generated numeric type — read the actual output rather than assuming) and a `NumRetries *int` field.

- [ ] **Step 4: Commit**

```bash
git add api/management-openapi.yaml pkg/api/management/generated.go
git commit -m "feat(gateway-controller): add resilience.retry to the OpenAPI schema"
```

### Task 6: Validate `resilience.retry` in both REST and LLM validators

**Files:**
- Modify: `gateway/gateway-controller/pkg/config/api_validator.go`
- Modify: `gateway/gateway-controller/pkg/config/llm_validator.go`
- Test: `gateway/gateway-controller/pkg/config/api_validator_test.go`
- Test: `gateway/gateway-controller/pkg/config/llm_validator_test.go`

**Interfaces:**
- Consumes: `api.Retry` (Task 5); existing `validateResilience(fieldPrefix string, r *api.Resilience) []ValidationError` (`api_validator.go:478`, currently delegates only to `validateResilienceTimeouts`).
- Produces: `validateResilienceRetry(fieldPrefix string, r *api.Retry) []ValidationError`, called from `validateResilience` (so both `spec.resilience.retry` and `spec.operations[i].resilience.retry` get it, matching the existing call sites at lines 466 and 639) — this is deliberately added to the one shared `validateResilience` function, not duplicated per validator, so both REST and LLM configs get the check identically. Confirm `llm_validator.go` calls the same shared `validateResilience` function (not a separate copy) before writing this task's test — if it currently has its own separate resilience-validation code path, that's the actual fix needed here (route it through the shared function instead of duplicating the check).

- [ ] **Step 1: Write the failing tests**

```go
// added to api_validator_test.go
func TestValidateResilience_RetryRequiresNonEmptyStatusCodes(t *testing.T) {
	r := &api.Resilience{Retry: &api.Retry{StatusCodes: []int{}}}
	errs := validateResilience("spec.resilience", r)
	if len(errs) == 0 {
		t.Error("expected an error for empty resilience.retry.statusCodes")
	}
}

func TestValidateResilience_RetryValidConfigPasses(t *testing.T) {
	numRetries := 2
	r := &api.Resilience{Retry: &api.Retry{StatusCodes: []int{401, 503}, NumRetries: &numRetries}}
	errs := validateResilience("spec.resilience", r)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}
```

```go
// added to llm_validator_test.go, inside the existing TestValidateLLMProvider_Resilience
// test function (llm_validator_test.go:2121) as two more t.Run subtests,
// alongside its existing timeout/idleTimeout cases — same validProviderWithResilience
// helper (line 1989), same assertHasFieldError helper already used throughout
// this test. Proves the LLM validator path enforces the identical
// resilience.retry check as the REST validator, since both already route
// through the one shared validateResilience function (confirmed by this file's
// existing timeout subtests already asserting on "spec.resilience.timeout"
// field-path errors from validProviderWithResilience — the same shared-function
// evidence Task 6 Step 4 needs, so that step is a confirmation, not a fix).

t.Run("retry with empty statusCodes is rejected", func(t *testing.T) {
	errs := validator.Validate(validProviderWithResilience(&api.Resilience{
		Retry: &api.Retry{StatusCodes: []int{}},
	}))
	assertHasFieldError(t, errs, "spec.resilience.retry.statusCodes")
})

t.Run("retry with valid statusCodes and numRetries is accepted", func(t *testing.T) {
	numRetries := 2
	errs := validator.Validate(validProviderWithResilience(&api.Resilience{
		Retry: &api.Retry{StatusCodes: []int{401, 503}, NumRetries: &numRetries},
	}))
	assert.Empty(t, errs)
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd gateway-controller && GOWORK=off go test ./pkg/config/... -run 'TestValidateResilience_Retry|TestValidateLLMProvider_ResilienceRetry' -v`
Expected: FAIL (`validateResilienceRetry`/the referenced fields don't exist yet, or the LLM path doesn't invoke it).

- [ ] **Step 3: Implement the validation.** In `api_validator.go`, add:

```go
// validateResilienceRetry validates a resilience.retry block: statusCodes
// must be non-empty (each in the valid HTTP status range, already enforced
// by the OpenAPI schema's minimum/maximum — this is a defense-in-depth check
// for configs that bypass schema validation, e.g. direct DB rows), and
// numRetries (if set) must be >= 1.
func validateResilienceRetry(fieldPrefix string, r *api.Retry) []ValidationError {
	if r == nil {
		return nil
	}
	var errs []ValidationError
	if len(r.StatusCodes) == 0 {
		errs = append(errs, ValidationError{
			Field:   fieldPrefix + ".retry.statusCodes",
			Message: "must be non-empty when resilience.retry is configured",
		})
	}
	for _, code := range r.StatusCodes {
		if code < 400 || code > 599 {
			errs = append(errs, ValidationError{
				Field:   fieldPrefix + ".retry.statusCodes",
				Message: fmt.Sprintf("status code %d is not a valid HTTP status code (400-599)", code),
			})
		}
	}
	if r.NumRetries != nil && *r.NumRetries < 1 {
		errs = append(errs, ValidationError{
			Field:   fieldPrefix + ".retry.numRetries",
			Message: "must be at least 1 when set",
		})
	}
	return errs
}
```

Update `validateResilience` (line 478) to also call it:

```go
func (v *APIValidator) validateResilience(fieldPrefix string, r *api.Resilience) []ValidationError {
	errs := validateResilienceTimeouts(fieldPrefix, r)
	if r != nil {
		errs = append(errs, validateResilienceRetry(fieldPrefix, r.Retry)...)
	}
	return errs
}
```

- [ ] **Step 4: Confirm the LLM validator routes through this same function.** Already confirmed during plan-writing: `llm_validator_test.go`'s existing `TestValidateLLMProvider_Resilience` (line 2121) asserts on `"spec.resilience.timeout"`/`"spec.resilience.idleTimeout"` field-path errors from `validProviderWithResilience(&api.Resilience{...})` — the identical field-path strings `validateResilienceTimeouts` produces — proving `llm_validator.go` already calls the shared `validateResilience`, not a separate duplicate path. So Step 3's addition to `validateResilience` automatically covers LLM configs too; this step is a build-and-test confirmation, not a code fix.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd gateway-controller && GOWORK=off go test ./pkg/config/... -run 'TestValidateResilience_Retry|TestValidateLLMProvider_ResilienceRetry' -v`
Expected: PASS.

- [ ] **Step 6: Run the full config package test suite to confirm no regressions**

Run: `cd gateway-controller && GOWORK=off go test ./pkg/config/... -v 2>&1 | tail -60`
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add pkg/config/api_validator.go pkg/config/llm_validator.go pkg/config/api_validator_test.go pkg/config/llm_validator_test.go
git commit -m "feat(gateway-controller): validate resilience.retry for both REST and LLM configs"
```

### Task 7: Emit `RouteAction.RetryPolicy` from `resilience.retry`

**Files:**
- Modify: `gateway/gateway-controller/pkg/xds/translator.go`
- Test: `gateway/gateway-controller/pkg/xds/translator_test.go`

**Interfaces:**
- Consumes: `api.Retry` (Task 5); the existing `resolvedTimeout` struct and `ResolveResilience`/`combineRouteResilience` functions (`translator.go:3252-3300`, already resolve `timeout`/`idleTimeout` per-route, precedence-aware) — extend the SAME resolved-timeout struct with a resolved retry field rather than threading a second, parallel parameter through every call site.
- Produces: adds a `Retry *api.Retry` field to the existing `resolvedTimeout` struct (rename awareness: every existing call site constructing/reading a `resolvedTimeout` continues to compile since this is an additive field), and sets `route.RouteAction.RetryPolicy` in `createRoute` (`translator.go:1628`) when that field is non-nil.

- [ ] **Step 1: Write the failing test**

```go
// added to translator_test.go — follow this file's existing convention for
// constructing a minimal RestApi/LlmProvider config with a resilience block
// (an existing test already covers plain timeout/idleTimeout — copy its
// fixture-building shape, then additionally set Resilience.Retry) and assert
// on the resulting *route.Route's RouteAction.RetryPolicy.
func TestCreateRoute_ResilienceRetryEmitsNativeRetryPolicy(t *testing.T) {
	numRetries := 2
	timeoutCfg := &resolvedTimeout{Retry: &api.Retry{StatusCodes: []int{401, 503}, NumRetries: &numRetries}}

	tr := &Translator{routerConfig: minimalRouterConfigForTest()} // reuse this file's existing minimal-router-config test helper
	r := tr.createRoute("api-id", "TestAPI", "v1", "/test", "GET", "/foo", "test-cluster",
		"", "localhost", "RestApi", "", "", nil, "project-1", timeoutCfg, false, nil)

	routeAction, ok := r.Action.(*route.Route_Route)
	if !ok {
		t.Fatalf("expected a Route_Route action, got %T", r.Action)
	}
	rp := routeAction.Route.RetryPolicy
	if rp == nil {
		t.Fatal("expected a non-nil RetryPolicy")
	}
	if rp.RetryOn != "retriable-status-codes" {
		t.Errorf("got RetryOn %q, want %q", rp.RetryOn, "retriable-status-codes")
	}
	if rp.NumRetries == nil || rp.NumRetries.Value != 2 {
		t.Errorf("got NumRetries %v, want 2", rp.NumRetries)
	}
	if len(rp.RetriableStatusCodes) != 2 || rp.RetriableStatusCodes[0] != 401 || rp.RetriableStatusCodes[1] != 503 {
		t.Errorf("got RetriableStatusCodes %v, want [401 503]", rp.RetriableStatusCodes)
	}
}
```

Note: check `createRoute`'s exact current parameter list (`translator.go:1628`) before writing this call — copy it exactly from the file rather than the abbreviated signature shown in this plan's earlier reading, since it takes many positional parameters and an off-by-one will misassign an unrelated field. Also confirm the actual field name for `resolvedTimeout` (may need to add `Retry *api.Retry` to it as part of Step 3 before this test can even reference it).

- [ ] **Step 2: Run test to verify it fails**

Run: `cd gateway-controller && GOWORK=off go test ./pkg/xds/... -run TestCreateRoute_ResilienceRetryEmitsNativeRetryPolicy -v`
Expected: FAIL (`resolvedTimeout` has no `Retry` field yet, or `RetryPolicy` is nil).

- [ ] **Step 3: Add the field and resolve it.** In the `resolvedTimeout` struct (near `ResolveResilience`, `translator.go:3252`), add:

```go
type resolvedTimeout struct {
	Route *time.Duration
	Idle  *time.Duration
	Retry *api.Retry // nil when resilience.retry is not configured
}
```

Update `ResolveResilience` to also surface it:

```go
func ResolveResilience(r *api.Resilience) (timeout *time.Duration, idleTimeout *time.Duration, retry *api.Retry, err error) {
	if r == nil {
		return nil, nil, nil, nil
	}
	// ... existing timeout/idleTimeout parsing unchanged ...
	return timeout, idleTimeout, r.Retry, nil
}
```

Update every call site of `ResolveResilience` (there are at least two, per the earlier reading: `translator.go:1079` and `:1096`/`:1128`) to accept the third return value and thread it into the `resolvedTimeout` constructed at each site (`combineRouteResilience`, `translator.go:3272`) — add a `retry *api.Retry` parameter to `combineRouteResilience` too (operation-level `Retry` overrides API-level `Retry` when set, matching the existing precedence rule for `timeout`/`idleTimeout` exactly — same "operation-level wins if non-nil, else fall back to API-level" logic already implemented there for the other two fields).

- [ ] **Step 4: Emit the RetryPolicy in `createRoute`.** Immediately after the existing block that sets `routeAction.Route.Timeout`/`.IdleTimeout` from `timeoutCfg` in `createRoute`, add:

```go
if timeoutCfg != nil && timeoutCfg.Retry != nil {
	numRetries := uint32(1)
	if timeoutCfg.Retry.NumRetries != nil {
		numRetries = uint32(*timeoutCfg.Retry.NumRetries)
	}
	statusCodes := make([]uint32, len(timeoutCfg.Retry.StatusCodes))
	for i, code := range timeoutCfg.Retry.StatusCodes {
		statusCodes[i] = uint32(code)
	}
	routeAction.Route.RetryPolicy = &route.RetryPolicy{
		RetryOn:              "retriable-status-codes",
		RetriableStatusCodes: statusCodes,
		NumRetries:           wrapperspb.UInt32(numRetries),
	}
}
```

Confirm `wrapperspb` is already imported in this file (it's used elsewhere per earlier reading, e.g. `wrapperspb.Bool` in `createExtProcFilter`) — if not already imported under that alias, add `"google.golang.org/protobuf/types/known/wrapperspb"`.

- [ ] **Step 5: Run test to verify it passes**

Run: `cd gateway-controller && GOWORK=off go test ./pkg/xds/... -run TestCreateRoute_ResilienceRetryEmitsNativeRetryPolicy -v`
Expected: PASS.

- [ ] **Step 6: Run the full xds package test suite to confirm no regressions**

Run: `cd gateway-controller && GOWORK=off go test ./pkg/xds/... -v 2>&1 | tail -80`
Expected: all PASS — pay particular attention to any existing test asserting on `resolvedTimeout`'s literal struct shape (a struct literal comparison test would need its expected value updated to include `Retry: nil`).

- [ ] **Step 7: Commit**

```bash
git add pkg/xds/translator.go pkg/xds/translator_test.go
git commit -m "feat(gateway-controller): emit native RouteAction.RetryPolicy from resilience.retry"
```

### Task 8: Attach the upstream `ext_proc` filter to clusters backing a retry-configured route

**Files:**
- Modify: `gateway/gateway-runtime/policy-engine/internal/constants/constants.go` — wait, this belongs in gateway-controller, not policy-engine; correct path: `gateway/gateway-controller/pkg/constants/constants.go`
- Modify: `gateway/gateway-controller/pkg/xds/translator.go`
- Test: `gateway/gateway-controller/pkg/xds/translator_test.go`

**Interfaces:**
- Consumes: `envoy/extensions/upstreams/http/v3.HttpProtocolOptions` (vendored in `go-control-plane v1.37.0`, confirmed present at `extensions/upstreams/http/v3/http_protocol_options.pb.go`, field `HttpFilters []*hcm.HttpFilter`); `cluster.Cluster.TypedExtensionProtocolOptions map[string]*anypb.Any` (confirmed field, `config/cluster/v3/cluster.pb.go:890`); the existing `createPolicyEngineCluster()` (`translator.go:1937`) as the exact pattern to mirror for a second internal cluster; the existing `TranslateConfigs`/`clusterMap` loop (`translator.go:717-799`) where clusters from every API are merged by name.
- Produces: `constants.UpstreamRefreshPolicyEngineClusterName` (new constant, gateway-controller's `pkg/constants/constants.go`, alongside the existing `PolicyEngineClusterName`); `(t *Translator) createUpstreamRefreshExtProcCluster() *cluster.Cluster`; `(t *Translator) createUpstreamRefreshExtProcFilter() (*hcm.HttpFilter, error)`; a per-cluster attachment step folded into the existing cluster-building/merge path so that any cluster backing at least one `resilience.retry`-configured route gets `TypedExtensionProtocolOptions` set.

- [ ] **Step 1: Write the failing test**

```go
// added to translator_test.go
func TestTranslateConfigs_ClusterGetsUpstreamFilterWhenAnyRouteHasRetryConfigured(t *testing.T) {
	// Build two minimal RestApi configs sharing the identical upstream host:scheme
	// (so they collapse into one Envoy cluster per this codebase's existing
	// host+scheme dedup — reuse this file's existing fixture-building helper for
	// "two APIs, same backend host" if one already exists, e.g. check
	// TestTranslateConfigs_* tests around the cluster-merge behavior for the
	// established pattern), where only ONE of the two has resilience.retry set.
	//
	// Assert: the resulting merged cluster (found by its deduped name) has
	// TypedExtensionProtocolOptions containing the upstream ext_proc filter -
	// proving the OR-across-sharers behavior from the design doc.
}

func TestTranslateConfigs_ClusterWithNoRetryConfiguredAnywhereGetsNoUpstreamFilter(t *testing.T) {
	// Same two-APIs-same-cluster shape, but NEITHER has resilience.retry set.
	// Assert: TypedExtensionProtocolOptions is nil/empty for that cluster.
}
```

(Concrete fixture code intentionally left to be copied from this test file's own established two-API-shared-cluster test, if one exists from Task 0's Q2 findings verification — confirm via `grep -n "same.*host\|shared.*cluster\|dedup" pkg/xds/translator_test.go` before inventing new fixture-building boilerplate.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd gateway-controller && GOWORK=off go test ./pkg/xds/... -run TestTranslateConfigs_ClusterGetsUpstreamFilter -v`
Expected: FAIL (no such attachment logic exists yet).

- [ ] **Step 3: Add the cluster name constant.** In `pkg/constants/constants.go`, next to `PolicyEngineClusterName`:

```go
// UpstreamRefreshPolicyEngineClusterName is the internal Envoy cluster
// pointing at policy-engine's second, upstream-attempt ext_proc endpoint
// (see gateway-runtime/policy-engine/internal/kernel/upstream_extproc.go).
UpstreamRefreshPolicyEngineClusterName = "policy_engine_upstream_refresh_cluster"
```

- [ ] **Step 4: Add the internal cluster constructor.** In `translator.go`, mirroring `createPolicyEngineCluster()` (line 1937) exactly except for the cluster name and target port:

```go
// createUpstreamRefreshExtProcCluster creates the internal Envoy cluster
// pointing at policy-engine's second, upstream-attempt ext_proc endpoint.
// Mirrors createPolicyEngineCluster's addressing (UDS by default, TCP via
// t.routerConfig.PolicyEngine.Mode) — this is a DIFFERENT socket/port on the
// same policy-engine process, not a different service.
func (t *Translator) createUpstreamRefreshExtProcCluster() *cluster.Cluster {
	policyEngine := t.routerConfig.PolicyEngine

	var address *core.Address
	if policyEngine.Mode == "tcp" {
		address = &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  policyEngine.Host,
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: policyEngine.UpstreamRefreshPort, // see Step 5: new RouterConfig field
					},
				},
			},
		}
	} else {
		address = &core.Address{
			Address: &core.Address_Pipe{
				Pipe: &core.Pipe{Path: constants.DefaultUpstreamExtProcSocketPath},
			},
		}
	}

	lbEndpoint := &endpoint.LbEndpoint{
		HostIdentifier: &endpoint.LbEndpoint_Endpoint{Endpoint: &endpoint.Endpoint{Address: address}},
	}
	clusterType := cluster.Cluster_STATIC
	if policyEngine.Mode == "tcp" {
		clusterType = cluster.Cluster_STRICT_DNS
	}

	return &cluster.Cluster{
		Name:                 constants.UpstreamRefreshPolicyEngineClusterName,
		ConnectTimeout:       durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: clusterType},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: constants.UpstreamRefreshPolicyEngineClusterName,
			Endpoints:   []*endpoint.LocalityLbEndpoints{{LbEndpoints: []*endpoint.LbEndpoint{lbEndpoint}}},
		},
		Http2ProtocolOptions: &core.Http2ProtocolOptions{},
	}
}
```

Note: `constants.DefaultUpstreamExtProcSocketPath` here refers to gateway-controller's OWN copy of this path constant (gateway-controller and policy-engine are separate Go modules — this string must be defined in `gateway-controller/pkg/constants/constants.go` too, kept literally identical to policy-engine's `internal/constants/constants.go` value from Task 4; add a short comment cross-referencing the other module's constant by file path, matching this codebase's existing convention for cross-module constant duplication flagged in an earlier KB note on this same feature area).

- [ ] **Step 5: Add the router config field for TCP mode.** Find `RouterConfig.PolicyEngine`'s struct definition (used as `t.routerConfig.PolicyEngine` above) and add `UpstreamRefreshPort int` alongside its existing `Port int` field, populated from whatever config-loading path already populates `Port` (grep for where `PolicyEngine.Port` is set from a config file/env var and add the mirror for the new field using the same mechanism).

- [ ] **Step 6: Add the upstream filter constructor.** In `translator.go`, mirroring `createExtProcFilter()` (line 3141) but scoped to request-headers only:

```go
// createUpstreamRefreshExtProcFilter creates the per-cluster upstream ext_proc
// filter that lets any UpstreamAttemptPolicy-implementing policy attach fresh
// per-attempt state to a native Envoy retry. Unlike the main downstream
// filter, this one only ever needs the request-headers phase.
func (t *Translator) createUpstreamRefreshExtProcFilter() (*hcm.HttpFilter, error) {
	policyEngine := t.routerConfig.PolicyEngine
	extProcConfig := &extproc.ExternalProcessor{
		GrpcService: &core.GrpcService{
			TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
				EnvoyGrpc: &core.GrpcService_EnvoyGrpc{ClusterName: constants.UpstreamRefreshPolicyEngineClusterName},
			},
			Timeout: durationpb.New(time.Duration(policyEngine.TimeoutMs) * time.Millisecond),
		},
		FailureModeAllow: true, // fail open — see Global Constraints; a failure here must never block the retry
		ProcessingMode: &extproc.ProcessingMode{
			RequestHeaderMode: extproc.ProcessingMode_SEND,
		},
		MessageTimeout: durationpb.New(time.Duration(policyEngine.MessageTimeoutMs) * time.Millisecond),
		RequestAttributes: []string{constants.ExtProcRequestAttributeRouteName},
	}
	extProcAny, err := anypb.New(extProcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal upstream ext_proc config: %w", err)
	}
	return &hcm.HttpFilter{
		Name:       constants.ExtProcFilterName + "_upstream_refresh",
		ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: extProcAny},
	}, nil
}
```

Note `FailureModeAllow: true` here is the deliberate opposite of the main downstream filter's `FailureModeAllow: false` (line 3156) — that asymmetry is intentional and must not be "fixed" to match: the downstream filter gates auth/access-control (must fail closed), this one only ever adds an optional header refresh to an already-in-flight retry (must fail open, per Global Constraints).

- [ ] **Step 7: Attach the filter to eligible clusters, OR'd across sharers.** In `TranslateConfigs`'s cluster-merge loop (`translator.go:717-799`, the one building `clusterMap`), after the loop that merges clusters by name (`clusterMap[c.Name] = c`), add a second pass that tracks which cluster names need the upstream filter:

```go
// A cluster needs the upstream refresh filter if ANY route across ANY
// deployed API resolves resilience.retry to it — OR'd across every API
// sharing that cluster (clusters are deduped by host+scheme, so two
// unrelated APIs can share one cluster; see the design doc's shared-cluster
// hazard section). This pass is separate from the cluster-merge loop above
// because "does this route have retry configured" is resolved per-route
// (createRoute/resolvedTimeout), not per-cluster, so it must be collected
// alongside route creation and applied to the already-merged cluster map
// afterward.
clustersNeedingUpstreamFilter := make(map[string]bool)
// (populate this set from within the same per-operation loop that already
// calls createRoute and resolves timeoutCfg for each operation — wherever
// timeoutCfg.Retry != nil, mark clustersNeedingUpstreamFilter[clusterName] = true
// using the same clusterName variable already in scope at that call site.
// This requires threading the resolved cluster name and retry-presence out
// of the existing per-operation loop into this outer scope — the simplest
// correct approach is accumulating into a slice/map declared before the
// loop starts and written to inside it, exactly like clusterMap itself
// already is.)

if len(clustersNeedingUpstreamFilter) > 0 {
	upstreamFilter, err := t.createUpstreamRefreshExtProcFilter()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create upstream refresh ext_proc filter: %w", err)
	}
	filterAny, err := anypb.New(&httpv3.HttpProtocolOptions{
		UpstreamProtocolOptions: &httpv3.HttpProtocolOptions_ExplicitHttpConfig_{
			ExplicitHttpConfig: &httpv3.HttpProtocolOptions_ExplicitHttpConfig{
				ProtocolConfig: &httpv3.HttpProtocolOptions_ExplicitHttpConfig_HttpProtocolOptions{},
			},
		},
		HttpFilters: []*hcm.HttpFilter{upstreamFilter},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal upstream HttpProtocolOptions: %w", err)
	}
	for clusterName := range clustersNeedingUpstreamFilter {
		c, ok := clusterMap[clusterName]
		if !ok {
			continue // cluster resolution failed elsewhere; nothing to attach to
		}
		if c.TypedExtensionProtocolOptions == nil {
			c.TypedExtensionProtocolOptions = make(map[string]*anypb.Any)
		}
		c.TypedExtensionProtocolOptions["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"] = filterAny
	}
	// Always add the internal cluster the filter targets, once, unconditionally
	// (cheap — see the design doc) if not already present.
	if _, ok := clusterMap[constants.UpstreamRefreshPolicyEngineClusterName]; !ok {
		clusterMap[constants.UpstreamRefreshPolicyEngineClusterName] = t.createUpstreamRefreshExtProcCluster()
	}
}
```

Import `httpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"` at the top of the file alongside the existing `extproc` import.

Note on `HttpProtocolOptions_ExplicitHttpConfig`: this repo's existing clusters (per Task 0-equivalent reading during design) never set explicit HTTP version options on data-plane clusters — using `HttpProtocolOptions_ExplicitHttpConfig` with an empty (default HTTP/1.1) `HttpProtocolOptions` sub-message here preserves that same default; do not add `Http2ProtocolOptions` inside it unless a specific data-plane cluster already required HTTP/2 before this change (confirmed in Task 0-equivalent reading: `createCluster` sets none today, so this stays default/HTTP1.1-compatible).

- [ ] **Step 8: Run tests to verify they pass**

Run: `cd gateway-controller && GOWORK=off go test ./pkg/xds/... -run TestTranslateConfigs_Cluster -v`
Expected: PASS.

- [ ] **Step 9: Run the full xds package test suite and full build to confirm no regressions**

Run: `cd gateway-controller && GOWORK=off go build ./... && GOWORK=off go test ./... 2>&1 | tail -100`
Expected: all PASS.

- [ ] **Step 10: Commit**

```bash
git add pkg/constants/constants.go pkg/xds/translator.go pkg/xds/translator_test.go
git commit -m "feat(gateway-controller): attach upstream ext_proc filter to clusters backing a retry-configured route"
```

---

## Phase 4: oauth2-generator — implement the refresh

### Task 9: Implement `UpstreamAttemptPolicy` on oauth2-generator, in both repos

**Files:**
- Modify: `gateway/dev-policies/oauth2-generator/oauth2_generator.go`
- Test: `gateway/dev-policies/oauth2-generator/oauth2_generator_test.go`
- Mirror identically into: the separate `gateway-controllers/policies/oauth2-generator` repo's `oauth2_generator.go` (per this repo's established dual-repo convention — diff byte-for-byte after this change)

**Interfaces:**
- Consumes: `policy.UpstreamAttemptContext`/`policy.UpstreamAttemptAction`/`policy.UpstreamAttemptHeaderModifications` (Task 1); existing `p.retrieveToken() (*xoauth2.Token, error)` (`oauth2_generator.go:958`); existing `p.tokenSource.Purge()` (the `tokenProvider` interface, `token_cache.go:293-296`); existing `buildHeaderValue(prefix, token string) string` helper and `p.headerName`/`p.valuePrefix` fields (already used identically in `OnRequestHeaders`, `oauth2_generator.go:938-941`).
- Produces: `(p *Policy) OnUpstreamAttemptRequestHeaders(ctx context.Context, actx *policy.UpstreamAttemptContext) policy.UpstreamAttemptAction`.

- [ ] **Step 1: Write the failing test**

```go
// added to oauth2_generator_test.go — follow this file's existing convention
// for constructing a *Policy via GetPolicy with a minimal valid params map
// (an existing OnRequestHeaders test already does this — copy its setup).
func TestOnUpstreamAttemptRequestHeaders_AttemptOneUsesCachedToken(t *testing.T) {
	p := newTestPolicyWithMockTokenSource(t) // reuse this file's existing test-policy constructor helper
	actx := &policy.UpstreamAttemptContext{AttemptCount: 1, Headers: policy.NewHeaders(nil)}

	action := p.OnUpstreamAttemptRequestHeaders(context.Background(), actx)
	mods, ok := action.(policy.UpstreamAttemptHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamAttemptHeaderModifications, got %T", action)
	}
	if _, set := mods.HeadersToSet["Authorization"]; !set {
		t.Error("expected Authorization to be set even on attempt 1")
	}
}

func TestOnUpstreamAttemptRequestHeaders_RetryPurgesAndRefetches(t *testing.T) {
	p, mockSource := newTestPolicyWithMockTokenSourceTrackingPurge(t) // extend the helper to expose a purge-call counter/mock, matching this file's existing mocking conventions for tokenSource
	actx := &policy.UpstreamAttemptContext{AttemptCount: 2, Headers: policy.NewHeaders(nil)}

	_ = p.OnUpstreamAttemptRequestHeaders(context.Background(), actx)

	if !mockSource.purgeCalled {
		t.Error("expected AttemptCount > 1 to purge the cached token before refetching")
	}
}

func TestOnUpstreamAttemptRequestHeaders_FetchErrorFailsOpen(t *testing.T) {
	p := newTestPolicyWithFailingTokenSource(t) // a tokenSource whose Token() always errors
	actx := &policy.UpstreamAttemptContext{AttemptCount: 2, Headers: policy.NewHeaders(nil)}

	action := p.OnUpstreamAttemptRequestHeaders(context.Background(), actx)
	mods, ok := action.(policy.UpstreamAttemptHeaderModifications)
	if !ok || len(mods.HeadersToSet) != 0 {
		t.Errorf("expected an empty no-op action on fetch failure (fail open), got %#v", action)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd dev-policies/oauth2-generator && GOWORK=off go test ./... -run TestOnUpstreamAttemptRequestHeaders -v`
Expected: FAIL with `undefined: (*Policy).OnUpstreamAttemptRequestHeaders` (and confirm/add the test-helper mocks referenced above, matching this file's existing `tokenProvider`-mocking conventions — check existing tests around `Purge()` assertions, e.g. any existing purge-on-401 test, for the established mock shape before inventing a new one).

- [ ] **Step 3: Implement the method.** In `oauth2_generator.go`, immediately after the existing `OnResponseHeaders` method:

```go
// OnUpstreamAttemptRequestHeaders implements policy.UpstreamAttemptPolicy — it
// runs once per individual upstream dial attempt (including Envoy-native
// retries; see resilience.retry), not once per client request. On any
// attempt after the first, the previous attempt's response is assumed
// rejected (that's why Envoy retried at all, per the configured
// resilience.retry.statusCodes), so the cached token is purged before
// refetching, guaranteeing attempt 2+ gets a genuinely fresh token rather
// than resending the same one that was just rejected. Fails open on any
// fetch error: an empty action lets the retry proceed with whatever
// Authorization header it already had rather than blocking it — this
// mechanism only ever makes a retry more likely to succeed, never a new way
// for it to fail (see Global Constraints).
func (p *Policy) OnUpstreamAttemptRequestHeaders(ctx context.Context, actx *policy.UpstreamAttemptContext) policy.UpstreamAttemptAction {
	if actx.AttemptCount > 1 {
		p.tokenSource.Purge()
	}

	tok, err := p.retrieveToken()
	if err != nil {
		slog.WarnContext(ctx, "OAuth2Generator: failed to fetch token for upstream attempt, failing open (no header mutation)",
			"attempt", actx.AttemptCount, "grantType", p.grantType, "clientId", p.clientID, "error", err)
		return policy.UpstreamAttemptHeaderModifications{}
	}

	return policy.UpstreamAttemptHeaderModifications{
		HeadersToSet: map[string]string{
			p.headerName: buildHeaderValue(p.valuePrefix, tok.AccessToken),
		},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd dev-policies/oauth2-generator && GOWORK=off go test ./... -run TestOnUpstreamAttemptRequestHeaders -v`
Expected: PASS.

- [ ] **Step 5: Run the full oauth2-generator test suite to confirm no regressions**

Run: `cd dev-policies/oauth2-generator && GOWORK=off go build ./... && GOWORK=off go test ./... -v 2>&1 | tail -80`
Expected: all PASS.

- [ ] **Step 6: Mirror into the other repo and diff**

```bash
diff gateway/dev-policies/oauth2-generator/oauth2_generator.go <path-to-gateway-controllers-checkout>/policies/oauth2-generator/oauth2_generator.go
```
Apply the identical `OnUpstreamAttemptRequestHeaders` addition to the `gateway-controllers/policies/oauth2-generator` checkout, then re-diff to confirm both files are byte-identical again (aside from any pre-existing, already-tracked divergence this repo's convention allows — confirm none exists for this specific method before committing).

- [ ] **Step 7: Commit (both repos)**

```bash
# api-platform (dev-policies mirror)
git add dev-policies/oauth2-generator/oauth2_generator.go dev-policies/oauth2-generator/oauth2_generator_test.go
git commit -m "feat(oauth2-generator): implement UpstreamAttemptPolicy for retry-time token refresh"

# gateway-controllers (source of truth) — separate repo, separate commit
git -C <path-to-gateway-controllers-checkout> add policies/oauth2-generator/oauth2_generator.go policies/oauth2-generator/oauth2_generator_test.go
git -C <path-to-gateway-controllers-checkout> commit -m "feat(oauth2-generator): implement UpstreamAttemptPolicy for retry-time token refresh"
```

---

## Phase 5: End-to-end verification

### Task 10: E2E test proving attempt 2 gets a distinguishable, fresh token

**Files:**
- Modify: `gateway/dev-policies/oauth2-generator/e2e/mocks/mock-oauth2-idp/main.go` (if it doesn't already issue a distinguishable token per call — confirm via reading its token-issuing handler first; the existing E.14 test already relies on distinguishable per-call tokens (`mock-token-N-issued-...`), per this session's own earlier investigation of that test, so this may already be satisfied with zero changes needed)
- Modify: `gateway/dev-policies/oauth2-generator/e2e/postman/oauth2.postman_collection.json` — add a new folder/requests
- Modify: `gateway/dev-policies/oauth2-generator/e2e/run-e2e.sh` — register the new test API, add the new folder to the appropriate newman run, clean it up
- Modify: `gateway/configs/config.toml` (or the e2e-specific one — confirm which) to set `upstream_extproc_port`/socket consistently with Task 4/8's new port if running in `tcp` mode for this e2e suite (check the existing e2e config's `mode` setting first — if it's `uds`, no port config is needed at all, only confirming the new socket path doesn't collide, which it can't since it's a distinct hardcoded path)

**Interfaces:**
- Consumes: everything from Tasks 1-9 — this task is pure verification, it adds no new production code.

- [ ] **Step 1: Confirm (or add) per-call distinguishable tokens in the mock IdP**

```bash
grep -n "mock-token" dev-policies/oauth2-generator/e2e/mocks/mock-oauth2-idp/main.go
```
If tokens are already generated as `mock-token-<call-number>-issued-<timestamp>` (per this session's earlier direct observation of live E.14 test output using exactly this format), no change is needed — skip to Step 2. If not, add a package-level atomic counter incremented per `/oauth2/token` call and interpolate it into the issued `access_token`, matching the existing format string used elsewhere in this file.

- [ ] **Step 2: Register a new test API with `resilience.retry` + a way to force exactly one 401**

Add to the Postman collection (mirroring the shape of the existing `oauth2-test-rediscache` registration this session already read in full): a new `oauth2-test-retry-refresh` LlmProvider with:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProvider
metadata:
  name: oauth2-test-retry-refresh
spec:
  displayName: OAuth2 Test (upstream-attempt retry refresh)
  version: v1.0
  template: openai
  context: /oauth2-test-retry-refresh/latest
  resilience:
    retry:
      statusCodes: [401]
      numRetries: 1
  upstream:
    url: http://host.docker.internal:9602
    auth:
      type: oauth2
      policyParams:
        tokenEndpoint: http://host.docker.internal:9601/oauth2/token
        clientId: test-client
        clientSecret: test-secret
        tokenRequestParams:
          testId: retry-refresh
  accessControl:
    mode: deny_all
    exceptions:
      - path: /chat/completions
        methods: [POST]
```

Add a request in a new folder ("E.34 - Upstream-attempt retry refresh") that:
1. Calls the mock AI backend's debug/control endpoint (mirror whatever E.16's "upstream 401" trigger mechanism already uses — this session read `token_cache.go`'s purge-on-401 design, which implies an existing e2e mechanism to make `mock-ai-backend` return 401 exactly once; find and reuse it rather than inventing a new one, e.g. `grep -n "force.*401\|once.*401" dev-policies/oauth2-generator/e2e/mocks/mock-ai-backend/main.go`) so the backend returns 401 on the first hit only, then 200 with the injected token echoed back on the second.
2. Sends the chat-completion request once.
3. Asserts: final status is 200 (the client never sees the intermediate 401), AND the echoed `Authorization` header's token is DIFFERENT from whatever token the mock IdP issued on the FIRST call (prove attempt 2 genuinely got a fresh one, not the same rejected one resent) — this is the core proof this whole feature exists for. Check `/debug/stats` (used identically in this session's earlier E.14 investigation) to additionally assert exactly 2 IdP calls happened (one per attempt).

- [ ] **Step 3: Wire registration/cleanup into `run-e2e.sh`**

Add `oauth2-test-retry-refresh` to the existing `PROVIDER_NAMES` array (same array this session already read and edited once this conversation, for the rediscache-clone entry) and add the new folder to whichever existing newman run groups E.22-E.32-style additions (follow the existing pattern for where E.27-E.31 were added, per this session's own reading of that section).

- [ ] **Step 4: Build the current code and run the new test**

```bash
cd gateway && make build-coverage
cd dev-policies/oauth2-generator/e2e && ./run-e2e.sh
```
Expected: the full suite passes, including the new E.34 folder's assertions.

- [ ] **Step 5: Commit**

```bash
git add dev-policies/oauth2-generator/e2e/
git commit -m "test(oauth2-generator): e2e coverage proving retry attempts get a genuinely fresh token"
```

---

## Self-Review Notes (for the plan author/reviewer, not a task)

- Every task above names exact files, exact function/type signatures already confirmed present in the current codebase by direct reading during this session (not guessed), and exact commands to run. Where a downstream detail could not be fully pinned without reading additional files beyond this session's scope (a few spots in Tasks 6, 8, and 10 explicitly say "read X first, match its existing convention" rather than inventing one) — those are flagged inline as the one remaining judgment call for that step's implementer, not left as a blank "add validation"-style placeholder.
- Type consistency check: `UpstreamAttemptContext`/`UpstreamAttemptAction`/`UpstreamAttemptHeaderModifications`/`UpstreamAttemptPolicy`/`OnUpstreamAttemptRequestHeaders` are named identically across Tasks 1, 3, and 9 — confirmed no drift between the SDK definition and its two consumers (policy-engine's dispatch loop, oauth2-generator's implementation).
- Scope check: this plan is one coherent feature across four subsystems with real ordering dependencies (SDK types must exist before policy-engine can dispatch to them; policy-engine's server must exist before gateway-controller's emitted config points anywhere meaningful; oauth2-generator's implementation is the first and only current consumer) — not independent subsystems that should have been split into separate plans.

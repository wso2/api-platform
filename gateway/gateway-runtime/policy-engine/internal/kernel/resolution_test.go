/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package kernel

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// ─── Fakes ───────────────────────────────────────────────────────────────────
//
// The shipped binary registers only the identity resolver, so every non-identity
// path below is exercised through a fake supplied via an independent registry. That
// is what makes the whole seam testable before the first real multiplexed kind
// lands.

// fakeOperationResolver reads the operation out of a request in whichever way the
// test needs: from a header (header-only, resolves at the header phase) or from a
// JSON body field (body-reading, defers to the body phase).
// It is its own factory and its own prepared resolver: Prepare captures the route's
// partition and returns the same value. That keeps the fixtures small, and the
// independence of two routes prepared by one factory is covered where it belongs, in
// the resolver package's own tests.
type fakeOperationResolver struct {
	name      string
	reqs      resolver.RequestRequirements
	bodyField string // when set, the operation is read from this top-level JSON field
	header    string // when set, the operation is read from this header
	forcedErr *resolver.ResolutionError

	// apiID and vhost are captured at Prepare, exactly as a real resolver captures the
	// partition it composes keys from.
	apiID string
	vhost string

	seenBody   []byte
	seenView   resolver.RequestView
	identified int
}

func (f *fakeOperationResolver) Name() string { return f.name }

func (f *fakeOperationResolver) Prepare(cfg resolver.ResolverRouteConfig) (resolver.PreparedResolver, error) {
	f.capture(cfg)
	return f, nil
}

// capture records the static route data a real Prepare would keep.
func (f *fakeOperationResolver) capture(cfg resolver.ResolverRouteConfig) {
	f.apiID, f.vhost = cfg.APIID, cfg.Vhost
}

func (f *fakeOperationResolver) Requirements() resolver.RequestRequirements { return f.reqs }

// resolveOperation composes the resolution for one identified operation, the way a real
// resolver does: with the partition captured at Prepare, never with anything from the
// request.
func (f *fakeOperationResolver) resolveOperation(operation string) resolver.Resolution {
	return resolver.Resolution{
		ChainKey: resolver.ChainKeyFor(f.apiID, f.vhost, operation),
	}
}

func (f *fakeOperationResolver) Resolve(_ context.Context, view resolver.RequestView) (resolver.Resolution, error) {
	f.identified++
	f.seenView = view
	f.seenBody = view.Body

	if f.forcedErr != nil {
		return resolver.Resolution{}, f.forcedErr
	}

	if f.header != "" {
		values := view.Headers[f.header]
		if len(values) == 0 {
			return resolver.Resolution{}, &resolver.ResolutionError{Kind: resolver.FailureInvalidRequest}
		}
		return f.resolveOperation(values[0]), nil
	}

	var envelope map[string]any
	if err := json.Unmarshal(view.Body, &envelope); err != nil {
		return resolver.Resolution{}, &resolver.ResolutionError{Kind: resolver.FailureParse, Cause: err}
	}
	op, ok := envelope[f.bodyField].(string)
	if !ok {
		return resolver.Resolution{}, &resolver.ResolutionError{Kind: resolver.FailureInvalidRequest}
	}
	return f.resolveOperation(op), nil
}

// headerPolicy runs at the request-header phase, so the deferred path can prove
// header policies really execute at the body callback.
type headerPolicy struct {
	setHeader  string
	setValue   string
	statusCode int // when non-zero, short-circuits with this status
	ran        *bool
}

func (p *headerPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestHeaderMode: policy.HeaderModeProcess}
}

func (p *headerPolicy) OnRequestHeaders(_ context.Context, _ *policy.RequestHeaderContext, _ map[string]interface{}) policy.RequestHeaderAction {
	if p.ran != nil {
		*p.ran = true
	}
	if p.statusCode != 0 {
		return policy.ImmediateResponse{
			StatusCode: p.statusCode,
			Headers:    map[string]string{"www-authenticate": "Bearer"},
			Body:       []byte(`{"error":"unauthorized"}`),
		}
	}
	return policy.UpstreamRequestHeaderModifications{
		HeadersToSet: map[string]string{p.setHeader: p.setValue},
	}
}

// bodyPolicy runs at the request-body phase and records the bytes it was given, so a
// test can prove the decoded body is reused rather than decompressed twice.
type bodyPolicy struct {
	seen *[]byte
}

func (p *bodyPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestBodyMode: policy.BodyModeBuffer}
}

func (p *bodyPolicy) OnRequestBody(_ context.Context, ctx *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	if p.seen != nil && ctx.Body != nil {
		*p.seen = ctx.Body.Content
	}
	return nil
}

// ─── Fixtures ────────────────────────────────────────────────────────────────

type resolutionFixture struct {
	server    *ExternalProcessorServer
	kernel    *Kernel
	resolvers resolver.ResolverRegistry
	t         *testing.T
}

func newResolutionFixture(t *testing.T, resolvers ...resolver.Resolver) *resolutionFixture {
	t.Helper()
	reg := resolver.NewRegistry()
	for _, r := range resolvers {
		require.NoError(t, reg.Register(r))
	}
	// The identity resolver is always available, exactly as it is in production: a route
	// with no resolver_name normalises to it.
	if _, taken := reg.Get(resolver.RouteKeyResolverName); !taken {
		require.NoError(t, reg.Register(&resolver.RouteKeyResolver{}))
	}
	reg.Freeze()

	k := NewKernel()
	return &resolutionFixture{
		server: NewExternalProcessorServer(k, newTestExecutor(), config.TracingConfig{}, "",
			testMaxDecompressedBytes, testMaxDecompressedBytes),
		kernel:    k,
		resolvers: reg,
		t:         t,
	}
}

// route registers a single RouteConfig, preparing its resolver the way xDS ingest does.
// The resolution fields live on the embedded resolver.RouteResolution, so they are
// passed as one value; the returned pointer is the stored one, so a test can adjust a
// non-resolution field (a buffer limit) after.
func (f *resolutionFixture) route(routeKey string, rr resolver.RouteResolution) *RouteConfig {
	f.t.Helper()
	rc := f.unpreparedRoute(routeKey, rr)
	require.NoError(f.t, PrepareRoute(f.resolvers, routeKey, rc))

	f.kernel.ApplyWholeRouteConfigs(map[string]*RouteConfig{routeKey: rc})
	return rc
}

// unpreparedRoute builds the RouteConfig without preparing it, for the tests that need
// a route the kernel would refuse. Ingest never produces one; a non-xDS load path could.
func (f *resolutionFixture) unpreparedRoute(routeKey string, rr resolver.RouteResolution) *RouteConfig {
	rr.RouteKey = routeKey
	// APIId and Vhost are the partition a prepared resolver captures, so a route whose
	// resolver composes operation keys needs them to compose anything at all.
	return &RouteConfig{
		Metadata: RouteMetadata{
			RouteName: routeKey,
			APIId:     testAPIID,
			Vhost:     testVhost,
		},
		RouteResolution: rr,
	}
}

// Composition inputs shared by every fixture in this file, so a composed key in an
// assertion is spelled the same way the kernel will compose it.
const (
	testAPIID = "api-1"
	testVhost = "example.com"
)

// operationChainKey is what the engine composes for an operation on a fixture route.
func operationChainKey(operation string) string {
	return resolver.ChainKeyFor(testAPIID, testVhost, operation)
}

// operationChain registers a chain under an operation's *composed* key, which is where
// a resolver-bearing route's requests will look for it.
func (f *resolutionFixture) operationChain(operation string, policies ...policy.Policy) *registry.PolicyChain {
	return f.chain(operationChainKey(operation), policies...)
}

func (f *resolutionFixture) chain(key string, policies ...policy.Policy) *registry.PolicyChain {
	chain := buildChainFor(policies)
	f.kernel.RegisterRoute(key, chain)
	return chain
}

// buildChainFor derives the chain's phase requirements from its policies' declared
// modes, mirroring what the xDS handler does when it builds a chain.
func buildChainFor(policies []policy.Policy) *registry.PolicyChain {
	chain := &registry.PolicyChain{Policies: policies}
	for _, p := range policies {
		chain.PolicySpecs = append(chain.PolicySpecs, policy.PolicySpec{Name: "fake", Version: "v1", Enabled: true})
		mode := p.Mode()
		if mode.RequestHeaderMode == policy.HeaderModeProcess {
			chain.RequiresRequestHeader = true
		}
		if mode.RequestBodyMode == policy.BodyModeBuffer {
			chain.RequiresRequestBody = true
		}
		if mode.ResponseBodyMode == policy.BodyModeBuffer {
			chain.RequiresResponseBody = true
		}
	}
	return chain
}

func headersRequest(routeKey string, endOfStream bool, headers map[string]string) *extprocv3.ProcessingRequest {
	values := make([]*corev3.HeaderValue, 0, len(headers))
	for k, v := range headers {
		values = append(values, &corev3.HeaderValue{Key: k, RawValue: []byte(v)})
	}
	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers:     &corev3.HeaderMap{Headers: values},
				EndOfStream: endOfStream,
			},
		},
		Attributes: map[string]*structpb.Struct{
			constants.ExtProcFilter: {
				Fields: map[string]*structpb.Value{
					"xds.route_name": structpb.NewStringValue(routeKey),
				},
			},
		},
	}
}

func gzipBytes(t *testing.T, in []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write(in)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// ─── Identity routes (invariant 5.1 / 5.4) ───────────────────────────────────

// An identity route's whole resolution happened at ingest, so the request path must not
// build a request view, buffer a body, acquire a renderer, or call Resolve.
func TestIdentityRoute_DoesNoRequestTimeResolverWork(t *testing.T) {
	f := newResolutionFixture(t)
	rc := f.route("GET|/pets|example.com", resolver.RouteResolution{ResolverName: resolver.RouteKeyResolverName})
	f.chain("GET|/pets|example.com", &testutils.NoopPolicy{})

	require.True(t, rc.Prepared.IsStatic(), "route-key must resolve entirely at ingest")
	assert.False(t, rc.Prepared.Requirements.BuffersBody())

	var execCtx *PolicyExecutionContext
	_, outcome, denial := f.server.initializeExecutionContext(context.Background(),
		headersRequest("GET|/pets|example.com", true, map[string]string{":method": "GET", ":path": "/pets"}), &execCtx)

	assert.Equal(t, bindReady, outcome)
	assert.Nil(t, denial)
	require.NotNil(t, execCtx)
	assert.Nil(t, execCtx.pending)
	assert.Empty(t, execCtx.operation, "a direct route has no operation to report")
}

// A statically-prepared resolver cannot name a chain other than its own route's — and
// that is settled at ingest, not per request: PrepareRoute refuses the route, so it never
// reaches the kernel and no request to it can bind the other route's chain.
//
// Checking it here rather than only in the resolver package covers the wrapper both
// ingest and these fixtures go through, which is where the route's partition and
// effective key are assembled.
func TestPrepareRoute_RefusesAStaticResolutionNamingAnotherRoutesChain(t *testing.T) {
	factory := &staticKeyResolver{name: "bad-static", key: "GET|/admin|example.com"}
	f := newResolutionFixture(t, factory)
	f.chain("GET|/admin|example.com", &testutils.NoopPolicy{})

	routeKey := "GET|/pets|example.com"
	rc := f.unpreparedRoute(routeKey, resolver.RouteResolution{ResolverName: "bad-static"})

	err := PrepareRoute(f.resolvers, routeKey, rc)
	require.Error(t, err, "the route must be refused at ingest, not fail on every request")
	assert.Contains(t, err.Error(), "invalid static resolution")
	assert.Nil(t, rc.Prepared, "a refused route stores no prepared resolver")
}

// staticKeyResolver prepares a static direct resolution naming whatever key it was given,
// so a test can drive PrepareRoute's validation of that resolution.
type staticKeyResolver struct {
	name string
	key  string
}

func (r *staticKeyResolver) Name() string { return r.name }

func (r *staticKeyResolver) Prepare(resolver.ResolverRouteConfig) (resolver.PreparedResolver, error) {
	return r, nil
}

func (r *staticKeyResolver) Requirements() resolver.RequestRequirements {
	return resolver.RequestRequirements{}
}

func (r *staticKeyResolver) Resolve(context.Context, resolver.RequestView) (resolver.Resolution, error) {
	return r.StaticResolution(), nil
}

func (r *staticKeyResolver) StaticResolution() resolver.Resolution {
	return resolver.Resolution{ChainKey: r.key}
}

// Invariant 5.1: an empty resolver_name and the explicit identity name behave the
// same, and both read the chain key from canonical_chain_key.
func TestIdentityRoute_UsesCanonicalChainKey(t *testing.T) {
	for _, name := range []string{"", resolver.RouteKeyResolverName} {
		t.Run(fmt.Sprintf("resolver_name=%q", name), func(t *testing.T) {
			f := newResolutionFixture(t)
			f.route("POST|/rpc|example.com", resolver.RouteResolution{
				ResolverName:      name,
				CanonicalChainKey: "canonical-chain",
			})
			f.chain("canonical-chain", &testutils.NoopPolicy{})

			var execCtx *PolicyExecutionContext
			_, outcome, _ := f.server.initializeExecutionContext(context.Background(),
				headersRequest("POST|/rpc|example.com", true, map[string]string{":method": "POST"}), &execCtx)

			require.Equal(t, bindReady, outcome)
			assert.Equal(t, "canonical-chain", execCtx.chainKey)
		})
	}
}

// Invariant 5.4: an identity route with no chain keeps the pre-existing sterile 500,
// and must not be diverted into the new resolution-failure renderer.
func TestIdentityRoute_MissingChainStillYieldsNoChainOutcome(t *testing.T) {
	f := newResolutionFixture(t)
	f.route("GET|/pets|example.com", resolver.RouteResolution{})

	var execCtx *PolicyExecutionContext
	rm, outcome, denial := f.server.initializeExecutionContext(context.Background(),
		headersRequest("GET|/pets|example.com", true, map[string]string{":method": "GET"}), &execCtx)

	assert.Equal(t, bindNoChain, outcome, "must take the pre-existing missing-chain path, not a resolution failure")
	assert.Nil(t, denial)
	assert.Nil(t, execCtx)
	assert.Equal(t, "GET|/pets|example.com", rm.RouteName)
}

// A RouteConfig that reached the kernel without canonical_chain_key (older
// controller, non-xDS load path) still resolves: for an identity route the route key
// *is* the chain key.
func TestIdentityRoute_AbsentCanonicalKeyFallsBackToRouteKey(t *testing.T) {
	f := newResolutionFixture(t)
	// No CanonicalChainKey on the wire: the fixture applies the same one-time fallback
	// ingest does, and the prepared resolver reads the effective value from there.
	f.route("GET|/pets|example.com", resolver.RouteResolution{})
	f.chain("GET|/pets|example.com", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, outcome, _ := f.server.initializeExecutionContext(context.Background(),
		headersRequest("GET|/pets|example.com", true, map[string]string{":method": "GET"}), &execCtx)

	require.Equal(t, bindReady, outcome)
	assert.Equal(t, "GET|/pets|example.com", execCtx.chainKey)
}

// ─── Header-only resolution ──────────────────────────────────────────────────

func TestHeaderOnlyResolver_ResolvesAtHeaderPhase(t *testing.T) {
	r := &fakeOperationResolver{name: "hdr", reqs: resolver.RequestRequirements{Headers: true}, header: "x-op"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "hdr",
	})
	f.operationChain("SendMessage", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, outcome, denial := f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/rpc|example.com", true, map[string]string{
			":method": "post", ":path": "/rpc", "x-op": "SendMessage",
		}), &execCtx)

	require.Equal(t, bindReady, outcome)
	assert.Nil(t, denial)
	require.NotNil(t, execCtx)
	assert.Equal(t, operationChainKey("SendMessage"), execCtx.chainKey)
	assert.Nil(t, execCtx.pending, "a header-only resolver must not defer")

	// GO-AUTH-006: the method reaches the resolver upper-cased, so no downstream
	// comparison or map key can miss on case.
	assert.Equal(t, "POST", r.seenView.Method)
	assert.Equal(t, "/rpc", r.seenView.Path)
}

// This fake composes a key for whatever the header says, so an operation it has no chain
// for reads as deployment skew rather than an unknown operation — the resolver vouched for
// it by returning a resolution. A resolver that wants to blame the caller must say so
// itself; see TestUnknownOperation_IsClassifiedByTheResolverNotTheBinder.
func TestHeaderOnlyResolver_MissingChainDenies(t *testing.T) {
	r := &fakeOperationResolver{name: "hdr", reqs: resolver.RequestRequirements{Headers: true}, header: "x-op"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "hdr",
	})
	f.operationChain("SendMessage", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, outcome, denial := f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/rpc|example.com", true, map[string]string{":method": "POST", "x-op": "NoSuchOp"}), &execCtx)

	require.Equal(t, bindFailed, outcome)
	require.NotNil(t, denial)
	assert.Equal(t, resolver.FailureChainMissing, denial.failure.Kind)
	assert.Nil(t, execCtx, "a denied request must leave no execution context")
}

// A route naming a resolver this binary lacks is dropped at ingest, so it never reaches
// the kernel at all.
func TestUnknownResolver_IsRefusedAtPreparation(t *testing.T) {
	f := newResolutionFixture(t)
	_, err := resolver.PrepareRoute(f.resolvers, resolver.ResolverRouteConfig{
		RouteKey:          "POST|/rpc|example.com",
		CanonicalChainKey: "route-level-chain",
		ResolverName:      "not-registered",
	})
	var re *resolver.ResolutionError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, resolver.FailureUnknownResolver, re.Kind)
}

// If such a route reaches the kernel anyway — a non-xDS load path — it must deny, never
// quietly resolve by route key, which would apply the route-level chain to every
// operation the route multiplexes.
func TestUnpreparedRoute_DeniesWithoutFallingBackToTheRouteKey(t *testing.T) {
	f := newResolutionFixture(t)
	routeKey := "POST|/rpc|example.com"
	rc := f.unpreparedRoute(routeKey, resolver.RouteResolution{
		ResolverName:      "not-registered",
		CanonicalChainKey: "route-level-chain",
	})
	f.kernel.ApplyWholeRouteConfigs(map[string]*RouteConfig{routeKey: rc})
	routeLevel := f.chain("route-level-chain", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, outcome, denial := f.server.initializeExecutionContext(context.Background(),
		headersRequest(routeKey, true, map[string]string{":method": "POST"}), &execCtx)

	require.Equal(t, bindFailed, outcome)
	require.NotNil(t, denial)
	assert.Equal(t, resolver.FailureUnknownResolver, denial.failure.Kind)
	assert.Nil(t, execCtx)
	assert.NotNil(t, routeLevel, "the route-level chain exists but must not have been selected")
}

// Resolution succeeded but the chain is absent — a configuration or xDS-skew failure. The
// resolver having returned a resolution at all is its claim that the operation is one of
// its own, so the binder does not re-adjudicate that: a missing chain is always the
// deployment's fault, never the caller's.
func TestResolvedButMissingChain_IsAConfigurationFailure(t *testing.T) {
	r := &fakeOperationResolver{
		name: "hdr", reqs: resolver.RequestRequirements{Headers: true}, header: "x-op",
	}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "hdr",
	})
	// The operation's chain is deliberately not registered.

	var execCtx *PolicyExecutionContext
	_, outcome, denial := f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/rpc|example.com", true, map[string]string{":method": "POST", "x-op": "SendMessage"}), &execCtx)

	require.Equal(t, bindFailed, outcome)
	assert.Equal(t, resolver.FailureChainMissing, denial.failure.Kind,
		"a missing chain is a deployment fault, distinct from the client naming something unknown")
}

// The other side of that: blaming the caller is the *resolver's* call, not the binder's.
// A resolver that does not recognise an operation says so by returning
// FailureUnknownOperation from Resolve, and the kernel carries that classification through
// unchanged. The kind still exists and still reaches the metric and the span; only its
// origin moved, from an inference the binder made to a statement the resolver makes.
func TestUnknownOperation_IsClassifiedByTheResolverNotTheBinder(t *testing.T) {
	r := &fakeOperationResolver{
		name: "hdr", reqs: resolver.RequestRequirements{Headers: true}, header: "x-op",
		forcedErr: &resolver.ResolutionError{Kind: resolver.FailureUnknownOperation},
	}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{ResolverName: "hdr"})
	// A chain the resolver could have reached, to show nothing was bound regardless.
	f.operationChain("tools/call", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, outcome, denial := f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/rpc|example.com", true,
			map[string]string{":method": "POST", "x-op": "tools/call:unlisted"}), &execCtx)

	require.Equal(t, bindFailed, outcome)
	require.NotNil(t, denial)
	assert.Equal(t, resolver.FailureUnknownOperation, denial.failure.Kind,
		"the resolver's own classification must survive to the metric and the span")
	assert.Nil(t, execCtx, "and nothing may be bound")
}

// ─── Deferred (body-phase) binding ───────────────────────────────────────────

func TestBodyResolver_DefersAndAsksEnvoyToBuffer(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})
	f.operationChain("SendMessage", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, outcome, _ := f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/rpc|example.com", false, map[string]string{":method": "POST", ":path": "/rpc"}), &execCtx)

	require.Equal(t, bindPending, outcome)
	require.NotNil(t, execCtx)
	assert.Nil(t, execCtx.policyChain, "no chain may be selected before the body arrives")
	assert.Zero(t, r.identified, "the resolver must not run before it has the body")

	resp := pendingResolutionResponse(execCtx.pending.prepared.Requirements)
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, resp.ModeOverride.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_SEND, resp.ModeOverride.ResponseHeaderMode,
		"the response-header callback is where the resolved chain's response mode is returned")
	assert.NotNil(t, resp.GetRequestHeaders())
	assert.Nil(t, resp.GetRequestHeaders().Response, "no mutation may be emitted before any policy has run")
}

// Envoy sends no request-body callback when the request headers are end-of-stream,
// so a pending request would wait forever. Resolve or deny during the header
// callback instead.
func TestBodyResolver_HeaderEndOfStreamResolvesImmediately(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})
	f.operationChain("SendMessage", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, outcome, denial := f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/rpc|example.com", true, map[string]string{":method": "POST"}), &execCtx)

	require.Equal(t, bindFailed, outcome, "an empty body cannot carry an operation")
	assert.Equal(t, resolver.FailureParse, denial.failure.Kind)
	assert.Equal(t, 1, r.identified, "the resolver must run at the header phase rather than wait")
	assert.Nil(t, execCtx)

	// The contract a BodyBuffered resolver is held to here: it is handed a view with no
	// body and must classify that rather than assume a non-empty slice. See
	// resolver.PreparedResolver.Resolve.
	assert.Nil(t, r.seenBody, "a bodyless request reaches Resolve with Body nil")
}

func TestDeferredBinding_RunsHeaderThenBodyPoliciesAtBodyPhase(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})

	headerRan := false
	var bodySeen []byte
	f.operationChain("SendMessage",
		&headerPolicy{setHeader: "x-op", setValue: "SendMessage", ran: &headerRan},
		&bodyPolicy{seen: &bodySeen},
	)

	execCtx := f.bindPending(t, "POST|/rpc|example.com")
	require.False(t, headerRan, "header policies must not run before the chain is known")

	body := []byte(`{"method":"SendMessage"}`)
	resp, err := execCtx.processRequestBody(context.Background(), &extprocv3.HttpBody{Body: body, EndOfStream: true})
	require.NoError(t, err)

	assert.Equal(t, operationChainKey("SendMessage"), execCtx.chainKey)
	assert.Nil(t, execCtx.pending, "the chain is bound, so nothing stays pending")
	assert.True(t, execCtx.boundAtBodyPhase)
	assert.True(t, headerRan, "the resolved chain's header policies run at the body callback")
	assert.Equal(t, body, bodySeen, "body policies see the same decoded bytes the resolver did")

	// Consequence 1 of deferred binding: header mutations are emitted on the
	// body-phase response, not the header-phase one.
	bodyResp := resp.GetRequestBody()
	require.NotNil(t, bodyResp, "the response must be body-phase shaped")
	require.NotNil(t, bodyResp.Response.HeaderMutation)
	var setKeys []string
	for _, h := range bodyResp.Response.HeaderMutation.SetHeaders {
		setKeys = append(setKeys, h.Header.Key)
	}
	assert.Contains(t, setKeys, "x-op")

	// Envoy ignores a ModeOverride on a body callback, so none is sent: the response
	// modes are decided at the response-header callback instead.
	assert.Nil(t, resp.ModeOverride)
}

// A policy rejection raised by the deferred chain's header policies still becomes an
// ImmediateResponse, even though those policies ran at the body callback.
func TestDeferredBinding_HeaderPolicyShortCircuitAtBodyPhase(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})
	var bodySeen []byte
	f.operationChain("SendMessage", &headerPolicy{statusCode: 401}, &bodyPolicy{seen: &bodySeen})

	execCtx := f.bindPending(t, "POST|/rpc|example.com")
	resp, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true})
	require.NoError(t, err)

	imm := resp.GetImmediateResponse()
	require.NotNil(t, imm)
	assert.Equal(t, typev3.StatusCode(401), imm.Status.Code)
	assert.Nil(t, bodySeen, "body policies must not run after a header policy short-circuits")
}

func TestDeferredBinding_ResolutionFailureDenies(t *testing.T) {
	tests := []struct {
		name       string
		body       []byte
		wantStatus typev3.StatusCode
		wantKind   resolver.FailureKind
	}{
		{"malformed payload", []byte(`not json`), typev3.StatusCode_BadRequest, resolver.FailureParse},
		{"valid payload, invalid envelope", []byte(`{"jsonrpc":"2.0"}`), typev3.StatusCode_BadRequest, resolver.FailureInvalidRequest},
		// This fake composes a key for whatever the body names, so an operation with no
		// chain is deployment skew — a 500 — rather than a 404. A resolver that means "the
		// caller named something that does not exist" returns FailureUnknownOperation
		// itself, which is a resolver-side decision and covered separately.
		{"resolved operation with no chain", []byte(`{"method":"NoSuchOp"}`), typev3.StatusCode_InternalServerError, resolver.FailureChainMissing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
			f := newResolutionFixture(t, r)
			f.route("POST|/rpc|example.com", resolver.RouteResolution{
				ResolverName: "body",
			})
			f.operationChain("SendMessage", &testutils.NoopPolicy{})

			execCtx := f.bindPending(t, "POST|/rpc|example.com")
			resp, err := execCtx.processRequestBody(context.Background(),
				&extprocv3.HttpBody{Body: tt.body, EndOfStream: true})
			require.NoError(t, err)

			imm := resp.GetImmediateResponse()
			require.NotNil(t, imm)
			assert.Equal(t, tt.wantStatus, imm.Status.Code)
			assert.Nil(t, execCtx.policyChain, "a denied request must never bind a chain")
			assert.True(t, execCtx.resolutionDenied)

			// error-handling.md: sterile body, no internals, correlation id only.
			assert.NotContains(t, string(imm.Body), "body")
			assert.Regexp(t, `^\{"error":"[A-Za-z ]+","error_id":"[0-9a-f-]{36}"\}$`, string(imm.Body))
		})
	}
}

// ─── Body ceilings on the deferred path ───────────────────────────────────────

func TestDeferredBinding_WireLimitRejectsBeforeResolving(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	f := newResolutionFixture(t, r)
	rc := f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})
	rc.MaxRequestBodyBytes = 8
	f.operationChain("SendMessage", &testutils.NoopPolicy{})

	execCtx := f.bindPending(t, "POST|/rpc|example.com")
	resp, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true})
	require.NoError(t, err)

	imm := resp.GetImmediateResponse()
	require.NotNil(t, imm)
	assert.Equal(t, typev3.StatusCode_PayloadTooLarge, imm.Status.Code)
	assert.Zero(t, r.identified, "an over-limit body must be rejected before the resolver sees it")
	// The configured limit is never echoed back (file-access.md directive 5): the body
	// is exactly the sterile reason phrase plus a correlation id.
	assert.Regexp(t, `^\{"error":"Payload Too Large","error_id":"[0-9a-f-]{36}"\}$`, string(imm.Body))
}

func TestDeferredBinding_DefaultWireLimitApplies(t *testing.T) {
	rc := &RouteConfig{}
	assert.Equal(t, DefaultMaxResolverRequestBodyBytes, rc.EffectiveMaxRequestBodyBytes(),
		"a body-resolved route with no explicit limit must still be bounded")

	rc.MaxRequestBodyBytes = 1024
	assert.Equal(t, int64(1024), rc.EffectiveMaxRequestBodyBytes())

	// A nonsensical value falls back to the default rather than disabling the bound.
	rc.MaxRequestBodyBytes = -1
	assert.Equal(t, DefaultMaxResolverRequestBodyBytes, rc.EffectiveMaxRequestBodyBytes())
}

func TestDeferredBinding_ResolvesFromDecodedGzipBody(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})
	var bodySeen []byte
	f.operationChain("SendMessage", &bodyPolicy{seen: &bodySeen})

	execCtx := f.bindPendingWithHeaders(t, "POST|/rpc|example.com", map[string]string{
		":method": "POST", ":path": "/rpc", "content-encoding": "gzip",
	})

	plain := []byte(`{"method":"SendMessage"}`)
	_, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: gzipBytes(t, plain), EndOfStream: true})
	require.NoError(t, err)

	assert.Equal(t, operationChainKey("SendMessage"), execCtx.chainKey)
	assert.Equal(t, plain, r.seenBody, "the resolver must see decoded bytes, never the compressed frame")
	assert.Equal(t, plain, bodySeen, "the decoded body is reused, not decompressed a second time")
}

// A body that claims a supported coding but does not decode under it must fail closed:
// passing the raw bytes on would resolve to whatever they happen to look like.
func TestDeferredBinding_UndecodableBodyFailsClosed(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})
	f.operationChain("SendMessage", &testutils.NoopPolicy{})

	execCtx := f.bindPendingWithHeaders(t, "POST|/rpc|example.com", map[string]string{
		":method": "POST", "content-encoding": "gzip",
	})
	resp, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true}) // not actually gzip
	require.NoError(t, err)

	imm := resp.GetImmediateResponse()
	require.NotNil(t, imm)
	assert.Equal(t, typev3.StatusCode_BadRequest, imm.Status.Code,
		"a body that will not decode is the client's problem, not an engine fault")
	assert.Zero(t, r.identified, "raw compressed bytes must never reach the resolver")
}

// A coding the engine cannot decode must be rejected before the resolver runs.
// decompressBody returns an unrecognised coding's bytes unchanged and reports no
// error — lenient behaviour that is right for a policy and wrong for a resolver,
// which would then select a chain from bytes nobody decoded.
func TestDeferredBinding_UndecodableEncodingsRejectedBeforeResolving(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
	}{
		{"unknown coding", map[string]string{"content-encoding": "exotic-codec"}},
		{"stacked codings in one header", map[string]string{"content-encoding": "gzip, br"}},
		{"stacked identical codings", map[string]string{"content-encoding": "gzip, gzip"}},
		{"coding with parameters", map[string]string{"content-encoding": "gzip;q=1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
			f := newResolutionFixture(t, r)
			f.route("POST|/rpc|example.com", resolver.RouteResolution{
				ResolverName: "body",
			})
			f.operationChain("SendMessage", &testutils.NoopPolicy{})

			headers := map[string]string{":method": "POST", ":path": "/rpc"}
			for k, v := range tt.headers {
				headers[k] = v
			}
			execCtx := f.bindPendingWithHeaders(t, "POST|/rpc|example.com", headers)

			// A body that would resolve perfectly well if it were read raw, so the only
			// thing stopping it is the encoding gate.
			resp, err := execCtx.processRequestBody(context.Background(),
				&extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true})
			require.NoError(t, err)

			imm := resp.GetImmediateResponse()
			require.NotNil(t, imm)
			assert.Equal(t, typev3.StatusCode_UnsupportedMediaType, imm.Status.Code)
			assert.Zero(t, r.identified, "the resolver must not see a body the engine did not decode")
			assert.Nil(t, execCtx.policyChain, "no chain may be bound from undecoded bytes")
		})
	}
}

// Content codings are case-insensitive, and a coding list may be split across header
// lines. Both were previously invisible to the exact-match switch: `GZIP` fell through
// as "unrecognised" and reached the resolver as a raw gzip frame, and only the last
// header line was ever examined.
func TestDeferredBinding_ContentCodingHeaderNormalization(t *testing.T) {
	t.Run("uppercase coding decodes", func(t *testing.T) {
		r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
		f := newResolutionFixture(t, r)
		f.route("POST|/rpc|example.com", resolver.RouteResolution{
			ResolverName: "body",
		})
		f.operationChain("SendMessage", &testutils.NoopPolicy{})

		execCtx := f.bindPendingWithHeaders(t, "POST|/rpc|example.com", map[string]string{
			":method": "POST", "content-encoding": "GZIP",
		})
		plain := []byte(`{"method":"SendMessage"}`)
		_, err := execCtx.processRequestBody(context.Background(),
			&extprocv3.HttpBody{Body: gzipBytes(t, plain), EndOfStream: true})
		require.NoError(t, err)

		assert.Equal(t, operationChainKey("SendMessage"), execCtx.chainKey)
		assert.Equal(t, plain, r.seenBody)
		assert.Equal(t, "gzip", execCtx.requestContentEncoding,
			"the canonical token is pinned so a mutated body re-encodes correctly")
	})

	t.Run("identity means no coding", func(t *testing.T) {
		r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
		f := newResolutionFixture(t, r)
		f.route("POST|/rpc|example.com", resolver.RouteResolution{
			ResolverName: "body",
		})
		f.operationChain("SendMessage", &testutils.NoopPolicy{})

		execCtx := f.bindPendingWithHeaders(t, "POST|/rpc|example.com", map[string]string{
			":method": "POST", "content-encoding": "identity",
		})
		plain := []byte(`{"method":"SendMessage"}`)
		_, err := execCtx.processRequestBody(context.Background(),
			&extprocv3.HttpBody{Body: plain, EndOfStream: true})
		require.NoError(t, err)

		assert.Equal(t, operationChainKey("SendMessage"), execCtx.chainKey)
		assert.Equal(t, plain, r.seenBody)
	})
}

// The unit-level table for the header reduction, including the split-across-lines case
// a single ProcessingRequest header map can express but the fixture cannot.
func TestResolverContentCoding(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string][]string
		want    string
		wantErr string
	}{
		{"absent", map[string][]string{}, "", ""},
		{"empty value", map[string][]string{"content-encoding": {""}}, "", ""},
		{"identity", map[string][]string{"content-encoding": {"identity"}}, "", ""},
		{"gzip", map[string][]string{"content-encoding": {"gzip"}}, "gzip", ""},
		{"br", map[string][]string{"content-encoding": {"br"}}, "br", ""},
		{"padded and mixed case", map[string][]string{"content-encoding": {"  GZip "}}, "gzip", ""},
		{"header name case", map[string][]string{"Content-Encoding": {"gzip"}}, "gzip", ""},
		{"identity alongside a real coding", map[string][]string{"content-encoding": {"identity, gzip"}}, "gzip", ""},
		{"unknown", map[string][]string{"content-encoding": {"exotic"}}, "", `"exotic" cannot be decoded`},
		{"stacked in one value", map[string][]string{"content-encoding": {"gzip, br"}}, "", "stacked content codings"},
		{
			// Only the last line reaches ec.requestContentEncoding, so reading the
			// captured value alone would silently accept this as plain "br".
			name:    "stacked across header lines",
			headers: map[string][]string{"content-encoding": {"gzip", "br"}},
			wantErr: "stacked content codings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolverContentCoding(tt.headers)
			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Empty(t, got)
		})
	}
}

// Invariant: the gate is deferred-path only. An identity route keeps decompressBody's
// lenient behaviour — a body it cannot decode still reaches policies as raw bytes with
// the encoding cleared — because there the chain is already selected and no
// policy-selection decision hangs on how the body reads.
func TestIdentityRoute_UndecodableBodyKeepsLenientBehaviour(t *testing.T) {
	f := newResolutionFixture(t)
	var seen []byte
	chain := buildChainFor([]policy.Policy{&bodyPolicy{seen: &seen}})

	ec := newPolicyExecutionContext(f.server, "POST|/pets|example.com", chain)
	ec.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
			{Key: ":method", RawValue: []byte("POST")},
			{Key: "content-encoding", RawValue: []byte("gzip")},
		}},
	}, RouteMetadata{RouteName: "POST|/pets|example.com"})

	notGzip := []byte(`{"plain":true}`)
	resp, err := ec.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: notGzip, EndOfStream: true})
	require.NoError(t, err)

	assert.Nil(t, resp.GetImmediateResponse(), "an identity route must not start rejecting these")
	assert.Equal(t, notGzip, seen, "policies still receive the raw bytes, as before")
	assert.Empty(t, ec.requestContentEncoding, "the encoding is cleared so nothing tries to re-compress")
}

// The decoded ceiling applies to an uncompressed body too, so the resolver's input
// is bounded by the same number either way.
func TestDeferredBinding_DecodedLimitAppliesToUncompressedBody(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	reg := resolver.NewRegistry()
	require.NoError(t, reg.Register(r))
	reg.Freeze()

	k := NewKernel()
	// A decoded ceiling well below the wire ceiling, so only the decoded check fires.
	server := NewExternalProcessorServer(k, newTestExecutor(), config.TracingConfig{}, "", 8,
		testMaxDecompressedBytes)
	f := &resolutionFixture{server: server, kernel: k, resolvers: reg, t: t}
	rc := f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})
	// A wire limit far above the decoded ceiling, so only the decoded check can fire.
	rc.MaxRequestBodyBytes = 1 << 20
	f.operationChain("SendMessage", &testutils.NoopPolicy{})

	execCtx := f.bindPending(t, "POST|/rpc|example.com")
	resp, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true})
	require.NoError(t, err)

	imm := resp.GetImmediateResponse()
	require.NotNil(t, imm)
	assert.Equal(t, typev3.StatusCode_PayloadTooLarge, imm.Status.Code)
	assert.Zero(t, r.identified)
}

// ─── Response phases after a denial ──────────────────────────────────────────

// Envoy should not send response callbacks after an ImmediateResponse, but if one
// arrives it must not dereference the nil chain.
func TestDeniedRequest_ResponsePhasesDoNotDereferenceNilChain(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})
	f.operationChain("SendMessage", &testutils.NoopPolicy{})

	execCtx := f.bindPending(t, "POST|/rpc|example.com")
	_, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`bad`), EndOfStream: true})
	require.NoError(t, err)
	require.Nil(t, execCtx.policyChain)

	respHeaders, err := execCtx.processResponseHeaders(context.Background(), &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{{Key: ":status", RawValue: []byte("200")}}},
	})
	require.NoError(t, err)
	assert.NotNil(t, respHeaders.GetResponseHeaders())

	respBody, err := execCtx.processResponseBody(context.Background(), &extprocv3.HttpBody{Body: []byte("x"), EndOfStream: true})
	require.NoError(t, err)
	assert.NotNil(t, respBody.GetResponseBody())

	// A second request-body callback for the same denied request is also inert.
	again, err := execCtx.processRequestBody(context.Background(), &extprocv3.HttpBody{EndOfStream: true})
	require.NoError(t, err)
	assert.NotNil(t, again.GetRequestBody())
}

// ─── Mode overrides ──────────────────────────────────────────────────────────

func TestPendingModeOverride(t *testing.T) {
	buffered := pendingModeOverride(resolver.RequestRequirements{Body: resolver.BodyBuffered})
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, buffered.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, buffered.ResponseBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_SKIP, buffered.RequestTrailerMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_SKIP, buffered.ResponseTrailerMode)

	headerOnly := pendingModeOverride(resolver.RequestRequirements{Headers: true})
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, headerOnly.RequestBodyMode)
}

// Invariant 5.3: an identity route's mode override is unchanged — chain-derived, and
// never routed through the pending path.
func TestIdentityRoute_ModeOverrideIsChainDerived(t *testing.T) {
	f := newResolutionFixture(t)
	chain := &registry.PolicyChain{RequiresRequestBody: true, RequiresResponseBody: true}
	ec := newPolicyExecutionContext(f.server, "GET|/pets|example.com", chain)
	ec.phase = phaseRequestHeaders

	mode := ec.getModeOverride()
	require.NotNil(t, mode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.ResponseBodyMode)
}

// The response-header callback is where a body-phase-bound chain's response-body
// mode reaches Envoy — the one callback that still precedes Envoy's decision about
// how to deliver the upstream body, and one it honours overrides on.
func TestBodyPhaseBoundChain_ResponseModeComesFromResponseHeaderCallback(t *testing.T) {
	f := newResolutionFixture(t)
	chain := &registry.PolicyChain{RequiresResponseBody: true}
	ec := newPolicyExecutionContext(f.server, "POST|/rpc|example.com", chain)
	ec.boundAtBodyPhase = true

	ec.phase = phaseRequestBody
	assert.Nil(t, ec.getModeOverride(), "Envoy ignores a ModeOverride on a body callback; none must be sent")

	ec.phase = phaseResponseHeaders
	ec.responseHeaderCtx = &policy.ResponseHeaderContext{
		ResponseStatus:  200,
		ResponseHeaders: policy.NewHeaders(map[string][]string{}),
	}
	ec.requestHeaderCtx = &policy.RequestHeaderContext{Method: "POST"}
	mode := ec.getModeOverride()
	require.NotNil(t, mode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, mode.ResponseBodyMode)
}

// ─── Generic failure rendering ───────────────────────────────────────────────

func TestGenericResolutionFailure_StatusPerKind(t *testing.T) {
	tests := map[resolver.FailureKind]int{
		resolver.FailureParse:               400,
		resolver.FailureInvalidRequest:      400,
		resolver.FailureMultiOperation:      400,
		resolver.FailureUnknownOperation:    404,
		resolver.FailurePayloadTooLarge:     413,
		resolver.FailureUnsupportedEncoding: 415,
		resolver.FailureUndecodableBody:     400,
		resolver.FailureUnknownResolver:     500,
		resolver.FailureChainMissing:        500,
		resolver.FailureInternal:            500,
	}
	for kind, want := range tests {
		t.Run(string(kind), func(t *testing.T) {
			out := genericResolutionFailure(kind, "abc-123")
			assert.Equal(t, want, out.StatusCode)
			assert.Equal(t, "application/json", out.Headers["content-type"])
			assert.Equal(t, "abc-123", out.Headers["x-error-id"])
			// The kind itself is an internal classification and never shipped.
			assert.NotContains(t, string(out.Body), string(kind))
		})
	}
}

// A renderer that returns no status has declined; the generic response is used
// rather than emitting an HTTP 0.
// ─── Request view construction ───────────────────────────────────────────────

func TestBuildRequestView(t *testing.T) {
	view := snapshotRequestHeaders(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
			{Key: ":method", RawValue: []byte("post")},
			{Key: ":path", RawValue: []byte("/rpc?x=1")},
			{Key: "accept", RawValue: []byte("application/json")},
			{Key: "accept", RawValue: []byte("text/plain")},
		}},
	}).requestView("POST|/rpc|example.com")

	assert.Equal(t, "POST|/rpc|example.com", view.RouteKey)
	assert.Equal(t, "POST", view.Method, "the method must be upper-cased at extraction (GO-AUTH-006)")
	assert.Equal(t, "/rpc?x=1", view.Path)
	assert.Equal(t, []string{"application/json", "text/plain"}, view.Headers["accept"])
	assert.Nil(t, view.Body, "the body is attached only once it has been decoded")
}

func TestBuildRequestView_NilHeaders(t *testing.T) {
	view := snapshotRequestHeaders(nil).requestView("r")
	assert.Equal(t, "r", view.RouteKey)
	assert.Nil(t, view.Headers)
}

// The resolver must observe the retained header-phase view at the body callback, not
// values re-derived there.
func TestDeferredBinding_ResolverSeesRetainedHeaderView(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})
	f.operationChain("SendMessage", &testutils.NoopPolicy{})

	execCtx := f.bindPendingWithHeaders(t, "POST|/rpc|example.com", map[string]string{
		":method": "post", ":path": "/rpc?trace=1", "x-client": "cli",
	})
	_, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true})
	require.NoError(t, err)

	assert.Equal(t, "POST", r.seenView.Method)
	assert.Equal(t, "/rpc?trace=1", r.seenView.Path)
	assert.Equal(t, []string{"cli"}, r.seenView.Headers["x-client"])
}

// ─── Fixture helpers ─────────────────────────────────────────────────────────

func (f *resolutionFixture) bindPending(t *testing.T, routeKey string) *PolicyExecutionContext {
	t.Helper()
	return f.bindPendingWithHeaders(t, routeKey, map[string]string{":method": "POST", ":path": "/rpc"})
}

func (f *resolutionFixture) bindPendingWithHeaders(t *testing.T, routeKey string, headers map[string]string) *PolicyExecutionContext {
	t.Helper()
	var execCtx *PolicyExecutionContext
	_, outcome, denial := f.server.initializeExecutionContext(context.Background(),
		headersRequest(routeKey, false, headers), &execCtx)
	require.Equal(t, bindPending, outcome, "expected deferred binding, got denial %v", denial)
	require.NotNil(t, execCtx)
	return execCtx
}

// Guard against a stray "strings" import removal breaking the method-normalization
// assertion above: snapshotRequestHeaders must actually be doing the upper-casing.
var _ = strings.ToUpper

// ─── Compressed-body mutation on the deferred path ───────────────────────────

// mutatingBodyPolicy rewrites the request body, so the recompression path is exercised.
type mutatingBodyPolicy struct {
	replacement []byte
	seen        *[]byte
}

func (p *mutatingBodyPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestBodyMode: policy.BodyModeBuffer}
}

func (p *mutatingBodyPolicy) OnRequestBody(_ context.Context, ctx *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	if p.seen != nil && ctx.Body != nil {
		*p.seen = ctx.Body.Content
	}
	return policy.UpstreamRequestModifications{Body: p.replacement}
}

// requestBodyMutation pulls the forwarded body and the Content-Length/Content-Encoding
// header operations out of an ext_proc body response.
func requestBodyMutation(t *testing.T, resp *extprocv3.ProcessingResponse) (body []byte, setHeaders map[string]string, removed []string) {
	t.Helper()
	bodyResp := resp.GetRequestBody()
	require.NotNil(t, bodyResp, "expected a body-phase response")
	require.NotNil(t, bodyResp.Response.BodyMutation, "expected a body mutation")

	body = bodyResp.Response.BodyMutation.GetBody()
	setHeaders = map[string]string{}
	for _, h := range bodyResp.Response.HeaderMutation.GetSetHeaders() {
		setHeaders[strings.ToLower(h.Header.Key)] = string(h.Header.RawValue)
	}
	removed = bodyResp.Response.HeaderMutation.GetRemoveHeaders()
	return body, setHeaders, removed
}

// A gzip request whose body a policy rewrote must reach the upstream re-compressed.
// Forwarding plaintext while keeping `content-encoding: gzip` is silently wrong: the
// upstream fails to inflate a body it was told is compressed.
func TestDeferredBinding_ModifiedCompressedBodyIsRecompressed(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})

	replacement := []byte(`{"method":"SendMessage","enriched":true}`)
	var policySaw []byte
	f.operationChain("SendMessage", &mutatingBodyPolicy{replacement: replacement, seen: &policySaw})

	execCtx := f.bindPendingWithHeaders(t, "POST|/rpc|example.com", map[string]string{
		":method": "POST", ":path": "/rpc", "content-encoding": "gzip",
	})

	plain := []byte(`{"method":"SendMessage"}`)
	resp, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: gzipBytes(t, plain), EndOfStream: true})
	require.NoError(t, err)

	assert.Equal(t, plain, policySaw, "the policy sees decoded bytes")

	forwarded, setHeaders, removed := requestBodyMutation(t, resp)
	assert.NotEqual(t, replacement, forwarded,
		"the modified body must not be forwarded as plaintext while Content-Encoding says gzip")

	// It is genuinely gzip, and it inflates back to what the policy produced.
	inflated, err := decompressBody(forwarded, "gzip", testMaxDecompressedBytes)
	require.NoError(t, err, "the forwarded body must be valid gzip")
	assert.Equal(t, replacement, inflated)

	// Content-Length must describe the compressed bytes actually sent.
	assert.Equal(t, fmt.Sprintf("%d", len(forwarded)), setHeaders["content-length"])
	assert.NotContains(t, removed, "content-encoding", "the encoding still applies, so it must be kept")
}

// An uncompressed request on the deferred path forwards the modified body as-is and
// must not acquire a Content-Encoding.
func TestDeferredBinding_ModifiedUncompressedBodyIsForwardedVerbatim(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})

	replacement := []byte(`{"method":"SendMessage","enriched":true}`)
	f.operationChain("SendMessage", &mutatingBodyPolicy{replacement: replacement})

	execCtx := f.bindPending(t, "POST|/rpc|example.com")
	resp, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true})
	require.NoError(t, err)

	forwarded, setHeaders, _ := requestBodyMutation(t, resp)
	assert.Equal(t, replacement, forwarded)
	assert.Equal(t, fmt.Sprintf("%d", len(replacement)), setHeaders["content-length"])
}

// recompressModifiedRequestBody must stay symmetric with decompressBody, or a modified
// body ends up described by a Content-Encoding that does not match its bytes.
func TestRecompressModifiedRequestBody_RoundTripsPerEncoding(t *testing.T) {
	plain := []byte(`{"modified":true}`)

	for _, encoding := range []string{"gzip", "br"} {
		t.Run(encoding, func(t *testing.T) {
			f := newResolutionFixture(t)
			ec := newPolicyExecutionContext(f.server, "POST|/rpc|example.com", &registry.PolicyChain{})
			ec.requestContentEncoding = encoding

			mutation := &extprocv3.BodyMutation{Mutation: &extprocv3.BodyMutation_Body{Body: plain}}
			headerOps := map[string][]*headerOp{}

			length := recompressModifiedRequestBody(ec, mutation, headerOps)

			encoded := mutation.GetBody()
			assert.NotEqual(t, plain, encoded, "the body must actually be re-encoded")
			assert.Equal(t, len(encoded), length, "Content-Length must describe the encoded bytes")
			assert.Empty(t, headerOps["content-encoding"], "the encoding still applies, so it is kept")

			decoded, err := decompressBody(encoded, encoding, testMaxDecompressedBytes)
			require.NoError(t, err)
			assert.Equal(t, plain, decoded)
		})
	}

	// An encoding neither side understands is passed through unchanged by both
	// decompressBody and recompressBody, so the label stays accurate: the body was
	// never decoded, and it is not re-encoded either.
	t.Run("unsupported encoding passes through symmetrically", func(t *testing.T) {
		f := newResolutionFixture(t)
		ec := newPolicyExecutionContext(f.server, "POST|/rpc|example.com", &registry.PolicyChain{})
		ec.requestContentEncoding = "exotic-codec"

		mutation := &extprocv3.BodyMutation{Mutation: &extprocv3.BodyMutation_Body{Body: plain}}
		headerOps := map[string][]*headerOp{}

		length := recompressModifiedRequestBody(ec, mutation, headerOps)

		assert.Equal(t, plain, mutation.GetBody())
		assert.Equal(t, len(plain), length)

		passedThrough, err := decompressBody(plain, "exotic-codec", testMaxDecompressedBytes)
		require.NoError(t, err)
		assert.Equal(t, plain, passedThrough,
			"decompressBody must pass the same encoding through, or the two would disagree")
	})
}

// A streamed body mutation is re-compressed on its own per-chunk path, so the buffered
// helper must decline it rather than mangle it.
func TestRecompressModifiedRequestBody_IgnoresStreamedMutation(t *testing.T) {
	f := newResolutionFixture(t)
	ec := newPolicyExecutionContext(f.server, "POST|/rpc|example.com", &registry.PolicyChain{})
	ec.requestContentEncoding = "gzip"

	mutation := &extprocv3.BodyMutation{
		Mutation: &extprocv3.BodyMutation_StreamedResponse{StreamedResponse: &extprocv3.StreamedBodyResponse{}},
	}
	assert.Zero(t, recompressModifiedRequestBody(ec, mutation, map[string][]*headerOp{}))
}

// ─── Analytics on a deferred short-circuit ───────────────────────────────────

// analyticsPolicy contributes analytics metadata at whichever phase it is wired for.
type analyticsPolicy struct {
	headerMetadata map[string]any
	bodyMetadata   map[string]any
	rejectStatus   int
}

func (p *analyticsPolicy) Mode() policy.ProcessingMode {
	mode := policy.ProcessingMode{}
	if p.headerMetadata != nil {
		mode.RequestHeaderMode = policy.HeaderModeProcess
	}
	if p.bodyMetadata != nil || p.rejectStatus != 0 {
		mode.RequestBodyMode = policy.BodyModeBuffer
	}
	return mode
}

func (p *analyticsPolicy) OnRequestHeaders(_ context.Context, _ *policy.RequestHeaderContext, _ map[string]interface{}) policy.RequestHeaderAction {
	if p.headerMetadata == nil {
		return nil
	}
	return policy.UpstreamRequestHeaderModifications{AnalyticsMetadata: p.headerMetadata}
}

func (p *analyticsPolicy) OnRequestBody(_ context.Context, _ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	if p.rejectStatus != 0 {
		return policy.ImmediateResponse{
			StatusCode:        p.rejectStatus,
			Body:              []byte(`{"error":"denied"}`),
			AnalyticsMetadata: map[string]any{"denied_by": "quota"},
		}
	}
	if p.bodyMetadata == nil {
		return nil
	}
	return policy.UpstreamRequestModifications{AnalyticsMetadata: p.bodyMetadata}
}

func analyticsFromResponse(t *testing.T, resp *extprocv3.ProcessingResponse) map[string]interface{} {
	t.Helper()
	require.NotNil(t, resp.DynamicMetadata)
	ns := resp.DynamicMetadata.Fields[constants.ExtProcFilterName]
	require.NotNil(t, ns, "expected the ext_proc dynamic metadata namespace")
	data := ns.GetStructValue().GetFields()["analytics_data"]
	require.NotNil(t, data, "expected an analytics_data payload the ALS access log can read")
	return data.GetStructValue().AsMap()
}

// A rejection at the deferred body phase is the only ext_proc response the request
// produces — the pending request-headers response carried no policy metadata at all.
// So it has to carry everything the policies that already ran contributed, or the
// request loses fields in traffic logging.
func TestDeferredBinding_ShortCircuitKeepsEarlierAnalytics(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})

	f.operationChain("SendMessage",
		&analyticsPolicy{headerMetadata: map[string]any{"auth_subject": "user-7"}},
		&analyticsPolicy{bodyMetadata: map[string]any{"payload_kind": "jsonrpc"}},
		&analyticsPolicy{rejectStatus: 429},
	)

	execCtx := f.bindPending(t, "POST|/rpc|example.com")
	resp, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true})
	require.NoError(t, err)

	require.NotNil(t, resp.GetImmediateResponse())
	analytics := analyticsFromResponse(t, resp)

	assert.Equal(t, "user-7", analytics["auth_subject"],
		"a header policy that ran in this same callback must not lose its analytics")
	assert.Equal(t, "jsonrpc", analytics["payload_kind"],
		"a body policy that ran before the rejecting one must not lose its analytics")
	assert.Equal(t, "quota", analytics["denied_by"],
		"the rejecting policy's own metadata must be present too")
}

// The same aggregation applies when the header policies are what rejects.
func TestDeferredBinding_HeaderShortCircuitKeepsEarlierAnalytics(t *testing.T) {
	f := newResolutionFixture(t)
	ec := newPolicyExecutionContext(f.server, "POST|/rpc|example.com", &registry.PolicyChain{})
	ec.buildRequestContexts(&extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{{Key: ":method", RawValue: []byte("POST")}}},
	}, RouteMetadata{RouteName: "POST|/rpc|example.com"})
	ec.analyticsMetadata["collected_at"] = "request_headers"

	headerResult := &executor.RequestHeaderExecutionResult{
		ShortCircuited: true,
		FinalAction: policy.ImmediateResponse{
			StatusCode:        401,
			AnalyticsMetadata: map[string]any{"denied_by": "jwt-auth"},
		},
		Results: []executor.RequestHeaderPolicyResult{
			{Action: policy.UpstreamRequestHeaderModifications{AnalyticsMetadata: map[string]any{"trace_id": "abc"}}},
		},
	}

	resp, err := TranslateRequestBodyActionsWithHeaderMerge(headerResult, &executor.RequestExecutionResult{}, ec)
	require.NoError(t, err)

	analytics := analyticsFromResponse(t, resp)
	assert.Equal(t, "request_headers", analytics["collected_at"], "context-accumulated metadata survives")
	assert.Equal(t, "abc", analytics["trace_id"], "an earlier header policy's metadata survives")
	assert.Equal(t, "jwt-auth", analytics["denied_by"])
}

// ─── Span attributes for a body-resolved route ───────────────────────────────

// The chain key is the attribute that answers "which chain did this operation get?",
// and on a deferred route it is only known at the body callback. Recording "" at the
// header callback would make the attribute useless for its primary consumer.
func TestRecordResolutionAttributes_SkipsUnknownChainKey(t *testing.T) {
	f := newResolutionFixture(t)

	pending := newPolicyExecutionContext(f.server, "POST|/rpc|example.com", nil)
	pending.resolverName = "a2a-jsonrpc"
	rec := &recordingSpan{}
	pending.recordResolutionAttributes(rec)
	assert.Equal(t, map[string]string{constants.AttrResolverName: "a2a-jsonrpc"}, rec.attrs,
		"no chain key is stamped before one has been selected")

	pending.chainKey = "POST|/op-one|example.com"
	bound := &recordingSpan{}
	pending.recordResolutionAttributes(bound)
	assert.Equal(t, map[string]string{
		constants.AttrResolverName:   "a2a-jsonrpc",
		constants.AttrPolicyChainKey: "POST|/op-one|example.com",
	}, bound.attrs)

	// An identity route stamps nothing: it has no resolver, and its chain key is the
	// route name already on the span.
	identity := newPolicyExecutionContext(f.server, "GET|/pets|example.com", nil)
	identity.chainKey = "GET|/pets|example.com"
	none := &recordingSpan{}
	identity.recordResolutionAttributes(none)
	assert.Empty(t, none.attrs)
}

// The bound chain key really does reach a span once the body callback has run.
func TestDeferredBinding_ChainKeyIsRecordedAfterBinding(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})
	f.operationChain("SendMessage", &testutils.NoopPolicy{})

	execCtx := f.bindPending(t, "POST|/rpc|example.com")

	atHeaders := &recordingSpan{}
	execCtx.recordResolutionAttributes(atHeaders)
	assert.NotContains(t, atHeaders.attrs, constants.AttrPolicyChainKey)

	_, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true})
	require.NoError(t, err)
	require.True(t, execCtx.boundAtBodyPhase)

	atBody := &recordingSpan{}
	execCtx.recordResolutionAttributes(atBody)
	assert.Equal(t, operationChainKey("SendMessage"), atBody.attrs[constants.AttrPolicyChainKey])
	assert.Equal(t, "body", atBody.attrs[constants.AttrResolverName])
}

// recordingSpan is a trace.Span that only remembers the string attributes set on it,
// which is all these assertions need.
type recordingSpan struct {
	noop.Span
	attrs map[string]string
}

func (s *recordingSpan) IsRecording() bool { return true }

func (s *recordingSpan) SetAttributes(kv ...attribute.KeyValue) {
	if s.attrs == nil {
		s.attrs = map[string]string{}
	}
	for _, a := range kv {
		s.attrs[string(a.Key)] = a.Value.AsString()
	}
}

// The end-to-end version of the span assertion, driving handleProcessingPhase across
// both callbacks with a real span recorder. It is what proves the attribute reaches a
// recorded span rather than only that the helper would stamp it.
func TestDeferredBinding_SpanCarriesResolvedChainKeyEndToEnd(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
	reg := resolver.NewRegistry()
	require.NoError(t, reg.Register(r))
	reg.Freeze()

	k := NewKernel()
	server := NewExternalProcessorServer(k, executor.NewChainExecutor(nil, nil, tp.Tracer("test")),
		config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)
	server.tracer = tp.Tracer("test")

	f := &resolutionFixture{server: server, kernel: k, resolvers: reg, t: t}
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})
	f.operationChain("SendMessage", &testutils.NoopPolicy{})

	ctx, rootSpan := tp.Tracer("test").Start(context.Background(), "root")

	var execCtx *PolicyExecutionContext
	headerResp, err := server.handleProcessingPhase(ctx,
		headersRequest("POST|/rpc|example.com", false, map[string]string{":method": "POST", ":path": "/rpc"}),
		&execCtx, rootSpan)
	require.NoError(t, err)
	require.NotNil(t, headerResp.ModeOverride, "the header response must ask Envoy to buffer")

	bodyResp, err := server.handleProcessingPhase(ctx, &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestBody{
			RequestBody: &extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true},
		},
	}, &execCtx, rootSpan)
	require.NoError(t, err)
	require.NotNil(t, bodyResp.GetRequestBody())

	rootSpan.End()

	var root sdktrace.ReadOnlySpan
	for _, s := range sr.Ended() {
		if s.Name() == "root" {
			root = s
		}
	}
	require.NotNil(t, root, "the root span must have been recorded")

	attrs := map[string]string{}
	for _, a := range root.Attributes() {
		attrs[string(a.Key)] = a.Value.AsString()
	}
	assert.Equal(t, "body", attrs[constants.AttrResolverName])
	assert.Equal(t, operationChainKey("SendMessage"), attrs[constants.AttrPolicyChainKey],
		"the resolved chain key must reach the span, which only the body callback can do")
}

// ─── Existing-kind response compatibility ────────────────────────────────────

// bufferedResponsePolicy needs the whole response body — a guardrail or a redaction step.
type bufferedResponsePolicy struct{ testutils.NoopPolicy }

func (p *bufferedResponsePolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseBodyMode: policy.BodyModeBuffer}
}

// chunkedJSONHeaders is an ordinary unary response that happens to use chunked transfer
// framing.
func chunkedJSONHeaders() *extprocv3.HttpHeaders {
	return &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
		{Key: ":status", RawValue: []byte("200")},
		{Key: "content-type", RawValue: []byte("application/json")},
		{Key: "transfer-encoding", RawValue: []byte("chunked")},
	}}}
}

// sseHeaders is an upstream response that is genuinely a stream.
func sseHeaders() *extprocv3.HttpHeaders {
	return &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
		{Key: ":status", RawValue: []byte("200")},
		{Key: "content-type", RawValue: []byte("text/event-stream")},
	}}}
}

// A resolved route must not change how the response body is delivered. The decision comes
// from the chain and the upstream response headers, exactly as it did before resolution
// existed: a chain that can only buffer buffers, and is never failed closed over the
// shape of the response.
func TestResolvedRoute_ResponseDeliveryIsUnchanged(t *testing.T) {
	for name, headers := range map[string]*extprocv3.HttpHeaders{
		"sse upstream":     sseHeaders(),
		"chunked upstream": chunkedJSONHeaders(),
	} {
		t.Run(name, func(t *testing.T) {
			f := newResolutionFixture(t)
			f.route("GET|/chat|example.com", resolver.RouteResolution{})
			f.chain("GET|/chat|example.com", &bufferedResponsePolicy{})

			var execCtx *PolicyExecutionContext
			_, _, _ = f.server.initializeExecutionContext(context.Background(),
				headersRequest("GET|/chat|example.com", true, map[string]string{":method": "GET"}), &execCtx)
			require.NotNil(t, execCtx)

			resp, err := execCtx.processResponseHeaders(context.Background(), headers)
			require.NoError(t, err)
			assert.Nil(t, resp.GetImmediateResponse(),
				"resolution must not introduce a response-phase failure")
			assert.NotNil(t, resp.GetResponseHeaders())
			assert.False(t, execCtx.isStreamingResponse,
				"a buffered-only chain buffers, exactly as before")
		})
	}
}

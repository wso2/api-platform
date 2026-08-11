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
	"errors"
	"fmt"
	"os"
	"regexp"
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
type fakeOperationResolver struct {
	name      string
	reqs      resolver.Requirements
	bodyField string // when set, the operation is read from this top-level JSON field
	header    string // when set, the operation is read from this header
	forcedErr *resolver.ResolutionError

	// knownToProtocol mirrors a closed operation set (A2A's fixed enum), where a
	// missing chain is a deployment error rather than the client naming something that
	// does not exist. Left false, this fake behaves like an open set (MCP tool names).
	knownToProtocol bool

	// responseKindByOperation mirrors a protocol where one operation streams and its
	// sibling does not (A2A SendMessage versus SendStreamingMessage). Absent means Auto.
	responseKindByOperation map[string]resolver.ResponseKind

	seenBody   []byte
	seenView   resolver.RequestView
	identified int
}

func (f *fakeOperationResolver) Name() string                        { return f.name }
func (f *fakeOperationResolver) Requirements() resolver.Requirements { return f.reqs }

func (f *fakeOperationResolver) Identify(view resolver.RequestView) (resolver.Resolution, error) {
	f.identified++
	f.seenView = view
	f.seenBody = view.Body

	if f.forcedErr != nil {
		return resolver.Resolution{ProtocolState: "forced"}, f.forcedErr
	}

	if f.header != "" {
		values := view.Headers[f.header]
		if len(values) == 0 {
			return resolver.Resolution{}, &resolver.ResolutionError{Kind: resolver.FailureInvalidRequest}
		}
		return resolver.Resolution{
			Operations: []resolver.Operation{
				{Candidates: []string{values[0]}, KnownToProtocol: f.knownToProtocol},
			},
			ProtocolState: "header-state",
			ResponseKind:  f.responseKindByOperation[values[0]],
		}, nil
	}

	var envelope map[string]any
	if err := json.Unmarshal(view.Body, &envelope); err != nil {
		return resolver.Resolution{}, &resolver.ResolutionError{Kind: resolver.FailureParse, Cause: err}
	}
	op, ok := envelope[f.bodyField].(string)
	if !ok {
		return resolver.Resolution{ProtocolState: "body-state"},
			&resolver.ResolutionError{Kind: resolver.FailureInvalidRequest}
	}
	return resolver.Resolution{
		Operations: []resolver.Operation{
			{Candidates: []string{op}, KnownToProtocol: f.knownToProtocol},
		},
		ProtocolState: "body-state",
		ResponseKind:  f.responseKindByOperation[op],
	}, nil
}

// renderingResolver additionally shapes both failures and policy rejections, the way
// a real JSON-RPC resolver does.
type renderingResolver struct {
	fakeOperationResolver
	renderedFailures   int
	renderedRejections int
	lastRejectionState any
}

func (r *renderingResolver) RenderFailure(_ resolver.RequestView, err *resolver.ResolutionError) resolver.RenderedFailure {
	r.renderedFailures++
	return resolver.RenderedFailure{
		StatusCode: 418,
		Headers:    map[string]string{"content-type": "application/fake+json"},
		Body:       []byte(fmt.Sprintf(`{"kind":%q}`, err.Kind)),
	}
}

func (r *renderingResolver) RenderRejection(_ resolver.RequestView, protocolState any, in resolver.RenderedFailure) resolver.RenderedFailure {
	r.renderedRejections++
	r.lastRejectionState = protocolState
	return resolver.RenderedFailure{
		// Deliberately a different status: the caller must ignore it and keep the
		// policy's own status, so dashboards stay keyed on 401/429.
		StatusCode: 599,
		Headers:    map[string]string{"content-type": "application/fake+json"},
		Body:       []byte(fmt.Sprintf(`{"rejected":true,"state":%q}`, protocolState)),
	}
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
	server *ExternalProcessorServer
	kernel *Kernel
}

func newResolutionFixture(t *testing.T, resolvers ...resolver.OperationResolver) *resolutionFixture {
	t.Helper()
	reg := resolver.NewRegistry()
	for _, r := range resolvers {
		require.NoError(t, reg.Register(r))
	}
	reg.Freeze()

	k := NewKernel()
	return &resolutionFixture{
		server: NewExternalProcessorServer(k, newTestExecutor(), config.TracingConfig{}, "",
			testMaxDecompressedBytes, testMaxDecompressedBytes, reg),
		kernel: k,
	}
}

// route registers a single RouteConfig. The resolution fields live on the embedded
// resolver.RouteResolution, so they are passed as one value; the returned pointer is
// the stored one, so a test can adjust a non-resolution field (a buffer limit) after.
func (f *resolutionFixture) route(routeKey string, rr resolver.RouteResolution) *RouteConfig {
	rr.RouteKey = routeKey
	if rr.IsIdentity() && rr.CanonicalChainKey == "" {
		rr.CanonicalChainKey = routeKey
	}
	// APIId and Vhost are the chain-key composition inputs the kernel copies into the
	// request view, so a resolver-bearing route needs them to compose anything at all.
	rc := &RouteConfig{
		Metadata: RouteMetadata{
			RouteName: routeKey,
			APIId:     testAPIID,
			Vhost:     testVhost,
		},
		RouteResolution: rr,
	}
	f.kernel.ApplyWholeRouteConfigs(map[string]*RouteConfig{routeKey: rc})
	return rc
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

// The identity path must not build a request view, consult a map, or call a
// resolver — a fake registered under the identity name would be a bug if invoked.
func TestIdentityRoute_NeverCallsAResolver(t *testing.T) {
	trap := &fakeOperationResolver{name: resolver.RouteKeyResolverName, header: "x-op"}
	f := newResolutionFixture(t, trap)
	f.route("GET|/pets|example.com", resolver.RouteResolution{ResolverName: resolver.RouteKeyResolverName})
	f.chain("GET|/pets|example.com", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, outcome, denial := f.server.initializeExecutionContext(context.Background(),
		headersRequest("GET|/pets|example.com", true, map[string]string{":method": "GET", ":path": "/pets"}), &execCtx)

	assert.Equal(t, bindReady, outcome)
	assert.Nil(t, denial)
	assert.Zero(t, trap.identified, "the identity path must short-circuit before any resolver runs")
	require.NotNil(t, execCtx)
	assert.Nil(t, execCtx.rejectionRenderer, "an identity route must never acquire a renderer")
	assert.Nil(t, execCtx.failureRenderer)
	assert.Nil(t, execCtx.pending)
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
	f.kernel.ApplyWholeRouteConfigs(map[string]*RouteConfig{
		"GET|/pets|example.com": {Metadata: RouteMetadata{RouteName: "GET|/pets|example.com"}},
	})
	f.chain("GET|/pets|example.com", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, outcome, _ := f.server.initializeExecutionContext(context.Background(),
		headersRequest("GET|/pets|example.com", true, map[string]string{":method": "GET"}), &execCtx)

	require.Equal(t, bindReady, outcome)
	assert.Equal(t, "GET|/pets|example.com", execCtx.chainKey)
}

// ─── Header-only resolution ──────────────────────────────────────────────────

func TestHeaderOnlyResolver_ResolvesAtHeaderPhase(t *testing.T) {
	r := &fakeOperationResolver{name: "hdr", reqs: resolver.Requirements{Headers: true}, header: "x-op"}
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
	assert.Equal(t, "header-state", execCtx.protocolState)
	assert.Nil(t, execCtx.pending, "a header-only resolver must not defer")

	// GO-AUTH-006: the method reaches the resolver upper-cased, so no downstream
	// comparison or map key can miss on case.
	assert.Equal(t, "POST", r.seenView.Method)
	assert.Equal(t, "/rpc", r.seenView.Path)
}

func TestHeaderOnlyResolver_UnknownOperationDenies(t *testing.T) {
	r := &fakeOperationResolver{name: "hdr", reqs: resolver.Requirements{Headers: true}, header: "x-op"}
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
	assert.Equal(t, resolver.FailureUnknownOperation, denial.failure.Kind)
	assert.Nil(t, execCtx, "a denied request must leave no execution context")
}

// A route naming a resolver this binary lacks must deny, never quietly resolve by
// identity — that would apply the route-level chain to every multiplexed operation.
func TestUnknownResolver_DeniesWithoutFallingBackToIdentity(t *testing.T) {
	f := newResolutionFixture(t)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName:      "a2a-jsonrpc",
		CanonicalChainKey: "route-level-chain",
	})
	routeLevel := f.chain("route-level-chain", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, outcome, denial := f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/rpc|example.com", true, map[string]string{":method": "POST"}), &execCtx)

	require.Equal(t, bindFailed, outcome)
	assert.Equal(t, resolver.FailureUnknownResolver, denial.failure.Kind)
	assert.Nil(t, denial.renderer, "an unknown resolver cannot supply a renderer")
	assert.Nil(t, execCtx)
	assert.NotNil(t, routeLevel, "the route-level chain exists but must not have been selected")
}

// Resolution succeeded but the chain is absent — a configuration or xDS-skew
// failure, not the protocol's "unknown operation".
func TestResolvedButMissingChain_IsAConfigurationFailure(t *testing.T) {
	// A closed operation set: the protocol says SendMessage exists, so no chain for it
	// means the controller built the deployment wrong.
	r := &fakeOperationResolver{
		name: "hdr", reqs: resolver.Requirements{Headers: true}, header: "x-op",
		knownToProtocol: true,
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
	assert.Equal(t, resolver.FailureChainMissing, denial.failure.Kind)
	assert.False(t, denial.failure.ProtocolVisible(), "a missing chain must render generically")
}

// The other side of the same branch: an *open* operation set, where no chain for the
// identified operation means the client named something that does not exist. This is the
// distinction the controller-supplied operation map used to provide, now answered from
// the protocol definition — and it decides whether the caller or the deployment is at
// fault, so the two must not collapse into one failure kind.
func TestResolvedButMissingChain_OpenOperationSetBlamesTheClient(t *testing.T) {
	r := &fakeOperationResolver{name: "hdr", reqs: resolver.Requirements{Headers: true}, header: "x-op"}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{ResolverName: "hdr"})
	f.operationChain("tools/call", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, outcome, denial := f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/rpc|example.com", true,
			map[string]string{":method": "POST", "x-op": "tools/call:unlisted"}), &execCtx)

	require.Equal(t, bindFailed, outcome)
	require.NotNil(t, denial)
	assert.Equal(t, resolver.FailureUnknownOperation, denial.failure.Kind)
	assert.True(t, denial.failure.ProtocolVisible(),
		"an unknown operation is describable by the protocol, unlike a missing chain")
}

// ─── Deferred (body-phase) binding ───────────────────────────────────────────

func TestBodyResolver_DefersAndAsksEnvoyToBuffer(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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

	resp := pendingResolutionResponse(execCtx.pending.requirements)
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
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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
}

func TestDeferredBinding_RunsHeaderThenBodyPoliciesAtBodyPhase(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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
		{"unknown operation", []byte(`{"method":"NoSuchOp"}`), typev3.StatusCode_NotFound, resolver.FailureUnknownOperation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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

// A resolver that supplies a FailureRenderer shapes the protocol-visible failures,
// so a client library gets a body it can parse.
func TestDeferredBinding_ProtocolVisibleFailureUsesResolverRenderer(t *testing.T) {
	r := &renderingResolver{fakeOperationResolver: fakeOperationResolver{
		name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method",
	}}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})
	f.operationChain("SendMessage", &testutils.NoopPolicy{})

	execCtx := f.bindPending(t, "POST|/rpc|example.com")
	resp, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"method":"NoSuchOp"}`), EndOfStream: true})
	require.NoError(t, err)

	imm := resp.GetImmediateResponse()
	require.NotNil(t, imm)
	assert.Equal(t, typev3.StatusCode(418), imm.Status.Code)
	assert.JSONEq(t, `{"kind":"unknown-operation"}`, string(imm.Body))
	assert.Equal(t, 1, r.renderedFailures)
}

// A configuration failure must stay sterile even when the resolver has a renderer:
// the protocol has nothing to say about a chain the operator failed to deploy.
func TestDeferredBinding_ConfigurationFailureIgnoresResolverRenderer(t *testing.T) {
	r := &renderingResolver{fakeOperationResolver: fakeOperationResolver{
		name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method",
		knownToProtocol: true,
	}}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{
		ResolverName: "body",
	})
	// The operation's chain deliberately absent.

	execCtx := f.bindPending(t, "POST|/rpc|example.com")
	resp, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true})
	require.NoError(t, err)

	imm := resp.GetImmediateResponse()
	require.NotNil(t, imm)
	assert.Equal(t, typev3.StatusCode_InternalServerError, imm.Status.Code)
	assert.Zero(t, r.renderedFailures, "a missing chain is not a protocol-level error")
}

// ─── Body ceilings on the deferred path ───────────────────────────────────────

func TestDeferredBinding_WireLimitRejectsBeforeResolving(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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
			r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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
		r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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
		r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
	reg := resolver.NewRegistry()
	require.NoError(t, reg.Register(r))
	reg.Freeze()

	k := NewKernel()
	// A decoded ceiling well below the wire ceiling, so only the decoded check fires.
	server := NewExternalProcessorServer(k, newTestExecutor(), config.TracingConfig{}, "", 8,
		testMaxDecompressedBytes, reg)
	f := &resolutionFixture{server: server, kernel: k}
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
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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
	buffered := pendingModeOverride(resolver.Requirements{BufferBody: true})
	assert.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, buffered.RequestBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_NONE, buffered.ResponseBodyMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_SKIP, buffered.RequestTrailerMode)
	assert.Equal(t, extprocconfigv3.ProcessingMode_SKIP, buffered.ResponseTrailerMode)

	headerOnly := pendingModeOverride(resolver.Requirements{Headers: true})
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
func TestResolutionFailureBody_RendererWithoutStatusFallsBackToGeneric(t *testing.T) {
	declining := &decliningRenderer{}
	out, protocolShaped := resolutionFailureBody(resolver.RequestView{}, nil, declining,
		&resolver.ResolutionError{Kind: resolver.FailureParse}, "id")

	assert.False(t, protocolShaped)
	assert.Equal(t, 400, out.StatusCode)
}

type decliningRenderer struct{}

func (decliningRenderer) RenderFailure(resolver.RequestView, *resolver.ResolutionError) resolver.RenderedFailure {
	return resolver.RenderedFailure{}
}

// ─── Policy-rejection rendering (invariant 5.6) ──────────────────────────────

// The structural guarantee: without a renderer the rejection is returned by an early
// branch, not by a renderer that happens to be an identity function.
func TestRenderImmediate_NoRendererIsAnEarlyReturn(t *testing.T) {
	in := policy.ImmediateResponse{
		StatusCode: 429,
		Headers:    map[string]string{"retry-after": "30"},
		Body:       []byte(`{"error":"too many requests"}`),
	}

	assert.Equal(t, in, renderImmediate(nil, in), "a nil execution context must not be rendered")

	ec := &PolicyExecutionContext{}
	out := renderImmediate(ec, in)
	assert.Equal(t, in, out)
	// Same backing array: nothing was copied or rebuilt on this path.
	assert.Equal(t, fmt.Sprintf("%p", in.Body), fmt.Sprintf("%p", out.Body))
}

func TestRenderImmediate_PreservesStatusAndReplacesBody(t *testing.T) {
	r := &renderingResolver{}
	ec := &PolicyExecutionContext{rejectionRenderer: r, protocolState: "req-id-7"}

	out := renderImmediate(ec, policy.ImmediateResponse{
		StatusCode: 401,
		Headers:    map[string]string{"www-authenticate": "Bearer"},
		Body:       []byte(`{"error":"unauthorized"}`),
	})

	// Status preserved even though the renderer returned 599: the ALS access log,
	// the analytics outcome and operator dashboards stay keyed on the 401.
	assert.Equal(t, 401, out.StatusCode)
	assert.JSONEq(t, `{"rejected":true,"state":"req-id-7"}`, string(out.Body))
	// Renderer headers merge over the policy's rather than replacing them wholesale.
	assert.Equal(t, "Bearer", out.Headers["www-authenticate"])
	assert.Equal(t, "application/fake+json", out.Headers["content-type"])
	assert.Equal(t, "req-id-7", r.lastRejectionState)
}

// A rejection raised at any phase must still be re-renderable, which is why the
// request view and protocol state are retained for the whole stream.
func TestRenderImmediate_WorksAtEveryPhase(t *testing.T) {
	r := &renderingResolver{}
	ec := &PolicyExecutionContext{rejectionRenderer: r, protocolState: "state"}

	for _, phase := range []processingPhase{phaseRequestHeaders, phaseRequestBody, phaseResponseHeaders, phaseResponseBody} {
		ec.phase = phase
		out := renderImmediate(ec, policy.ImmediateResponse{StatusCode: 403, Body: []byte(`{}`)})
		assert.Equal(t, 403, out.StatusCode)
		assert.Contains(t, string(out.Body), "rejected")
	}
	assert.Equal(t, 4, r.renderedRejections)
}

// Invariant 5.6, structurally: every policy short-circuit in translator.go must go
// through buildImmediateResponse. A seventh construction site added later would fail
// this test rather than silently bypass rendering.
func TestNoImmediateResponseConstructionBypassesTheSharedHelper(t *testing.T) {
	src, err := os.ReadFile("translator.go")
	require.NoError(t, err)

	literal := regexp.MustCompile(`&extprocv3\.ImmediateResponse\{`)
	assert.Equal(t, 1, len(literal.FindAll(src, -1)),
		"translator.go must construct extprocv3.ImmediateResponse in exactly one place "+
			"(buildImmediateResponse); every policy short-circuit routes through it")

	// And every short-circuit branch that produces one goes through the helper.
	helperUses := bytes.Count(src, []byte("buildImmediateResponse(execCtx, immResp)"))
	assert.Equal(t, 6, helperUses,
		"all six policy short-circuit sites must call buildImmediateResponse")
}

// The engine's own sterile faults must NOT be reshaped by a resolver: an internal
// failure never takes its shape from resolver-supplied state.
func TestEngineGeneratedFaultsAreNotRendered(t *testing.T) {
	r := &renderingResolver{}
	f := newResolutionFixture(t)
	ec := newPolicyExecutionContext(f.server, "POST|/rpc|example.com", &registry.PolicyChain{})
	ec.rejectionRenderer = r
	ec.requestID = "req-1"

	resp := ec.handlePolicyError(context.Background(), errors.New("boom"), "request_headers")
	imm := resp.GetImmediateResponse()
	require.NotNil(t, imm)
	assert.Equal(t, typev3.StatusCode_InternalServerError, imm.Status.Code)
	assert.Contains(t, string(imm.Body), "Internal Server Error")
	assert.Zero(t, r.renderedRejections, "an engine-generated fault must stay sterile")

	tooLarge := ec.handlePayloadTooLarge(context.Background(), errors.New("too big"), "request_body")
	assert.Equal(t, typev3.StatusCode_PayloadTooLarge, tooLarge.GetImmediateResponse().Status.Code)
	assert.Zero(t, r.renderedRejections)
}

// ─── Request view construction ───────────────────────────────────────────────

func TestBuildRequestView(t *testing.T) {
	rc := &RouteConfig{}
	rc.RouteState = "prepared"

	view := buildRequestView("POST|/rpc|example.com", rc, &extprocv3.HttpHeaders{
		Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
			{Key: ":method", RawValue: []byte("post")},
			{Key: ":path", RawValue: []byte("/rpc?x=1")},
			{Key: "accept", RawValue: []byte("application/json")},
			{Key: "accept", RawValue: []byte("text/plain")},
		}},
	})

	assert.Equal(t, "POST|/rpc|example.com", view.RouteKey)
	assert.Equal(t, "POST", view.Method, "the method must be upper-cased at extraction (GO-AUTH-006)")
	assert.Equal(t, "/rpc?x=1", view.Path)
	assert.Equal(t, []string{"application/json", "text/plain"}, view.Headers["accept"])
	assert.Equal(t, "prepared", view.RouteState)
	assert.Nil(t, view.Body, "the body is attached only once it has been decoded")
}

func TestBuildRequestView_NilHeaders(t *testing.T) {
	view := buildRequestView("r", &RouteConfig{}, nil)
	assert.Equal(t, "r", view.RouteKey)
	assert.Nil(t, view.Headers)
}

// The resolver must observe the retained header-phase view at the body callback, not
// values re-derived there.
func TestDeferredBinding_ResolverSeesRetainedHeaderView(t *testing.T) {
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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
// assertion above: buildRequestView must actually be doing the upper-casing.
var _ = strings.ToUpper

// ─── Invariant 5.6: existing kinds' rejections are byte-identical ─────────────

// The shared rendering helper sits on the path of every 401, 429 and guardrail
// rejection for every kind shipping today. This drives all six translator entry
// points that can produce one and asserts the emitted ImmediateResponse carries the
// policy's own status, headers and body verbatim when the route has no renderer —
// which is every route that resolves by identity.
func TestExistingKindImmediateResponsesAreUnchanged(t *testing.T) {
	rejection := policy.ImmediateResponse{
		StatusCode: 429,
		Headers:    map[string]string{"retry-after": "30", "content-type": "application/json"},
		Body:       []byte(`{"error":"too many requests","policy":"advanced-ratelimit"}`),
	}

	assertVerbatim := func(t *testing.T, resp *extprocv3.ProcessingResponse) {
		t.Helper()
		imm := resp.GetImmediateResponse()
		require.NotNil(t, imm)
		assert.Equal(t, typev3.StatusCode(429), imm.Status.Code)
		assert.Equal(t, rejection.Body, imm.Body, "the policy's body must reach the client unmodified")

		got := make(map[string]string, len(imm.Headers.GetSetHeaders()))
		for _, h := range imm.Headers.GetSetHeaders() {
			got[h.Header.Key] = string(h.Header.RawValue)
		}
		assert.Equal(t, rejection.Headers, got, "the policy's headers must reach the client unmodified")
	}

	newCtx := func(t *testing.T) *PolicyExecutionContext {
		t.Helper()
		f := newResolutionFixture(t)
		ec := newPolicyExecutionContext(f.server, "GET|/pets|example.com", &registry.PolicyChain{})
		ec.buildRequestContexts(&extprocv3.HttpHeaders{
			Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
				{Key: ":method", RawValue: []byte("GET")},
				{Key: ":path", RawValue: []byte("/pets")},
			}},
		}, RouteMetadata{RouteName: "GET|/pets|example.com"})
		ec.buildResponseContexts(&extprocv3.HttpHeaders{
			Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{{Key: ":status", RawValue: []byte("200")}}},
		})
		require.Nil(t, ec.rejectionRenderer, "an identity route must have no renderer")
		return ec
	}

	headerResult := &executor.RequestHeaderExecutionResult{ShortCircuited: true, FinalAction: rejection}
	bodyResult := &executor.RequestExecutionResult{ShortCircuited: true, FinalAction: rejection}
	respHeaderResult := &executor.ResponseHeaderExecutionResult{ShortCircuited: true, FinalAction: rejection}
	respBodyResult := &executor.ResponseExecutionResult{ShortCircuited: true, FinalAction: rejection}

	t.Run("request headers", func(t *testing.T) {
		ec := newCtx(t)
		resp, err := TranslateRequestHeaderActions(headerResult, ec.policyChain, ec)
		require.NoError(t, err)
		assertVerbatim(t, resp)
	})

	t.Run("request body", func(t *testing.T) {
		ec := newCtx(t)
		resp, err := TranslateRequestBodyActions(bodyResult, ec.policyChain, ec)
		require.NoError(t, err)
		assertVerbatim(t, resp)
	})

	t.Run("request headers with inline body merge", func(t *testing.T) {
		ec := newCtx(t)
		resp, err := TranslateRequestHeaderActionsWithBodyMerge(
			&executor.RequestHeaderExecutionResult{}, bodyResult, ec)
		require.NoError(t, err)
		assertVerbatim(t, resp)
	})

	t.Run("response headers", func(t *testing.T) {
		ec := newCtx(t)
		resp, err := TranslateResponseHeaderActions(respHeaderResult, ec)
		require.NoError(t, err)
		assertVerbatim(t, resp)
	})

	t.Run("response headers with inline body merge", func(t *testing.T) {
		ec := newCtx(t)
		resp, err := TranslateResponseHeaderActionsWithBodyMerge(
			&executor.ResponseHeaderExecutionResult{}, respBodyResult, ec)
		require.NoError(t, err)
		assertVerbatim(t, resp)
	})

	t.Run("response body", func(t *testing.T) {
		ec := newCtx(t)
		resp, err := TranslateResponseBodyActions(respBodyResult, ec)
		require.NoError(t, err)
		assertVerbatim(t, resp)
	})

	// The new body-phase merge path, for a route whose chain binds late. With no
	// renderer it must behave exactly like the others.
	t.Run("request body with header merge", func(t *testing.T) {
		ec := newCtx(t)
		resp, err := TranslateRequestBodyActionsWithHeaderMerge(headerResult, &executor.RequestExecutionResult{}, ec)
		require.NoError(t, err)
		assertVerbatim(t, resp)
	})
}

// The mirror of the above: with a renderer attached, every one of those same paths
// re-renders the body while keeping the policy's status.
func TestRendererBearingRouteRewritesEveryPhase(t *testing.T) {
	rejection := policy.ImmediateResponse{
		StatusCode: 401,
		Headers:    map[string]string{"www-authenticate": "Bearer"},
		Body:       []byte(`{"error":"unauthorized"}`),
	}

	newCtx := func(t *testing.T) (*PolicyExecutionContext, *renderingResolver) {
		t.Helper()
		f := newResolutionFixture(t)
		r := &renderingResolver{}
		ec := newPolicyExecutionContext(f.server, "POST|/rpc|example.com", &registry.PolicyChain{})
		ec.buildRequestContexts(&extprocv3.HttpHeaders{
			Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{{Key: ":method", RawValue: []byte("POST")}}},
		}, RouteMetadata{RouteName: "POST|/rpc|example.com"})
		ec.buildResponseContexts(&extprocv3.HttpHeaders{
			Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{{Key: ":status", RawValue: []byte("200")}}},
		})
		ec.protocolState = "req-9"
		ec.attachRenderers(r)
		return ec, r
	}

	assertRendered := func(t *testing.T, resp *extprocv3.ProcessingResponse) {
		t.Helper()
		imm := resp.GetImmediateResponse()
		require.NotNil(t, imm)
		assert.Equal(t, typev3.StatusCode(401), imm.Status.Code, "the policy's status must survive rendering")
		assert.JSONEq(t, `{"rejected":true,"state":"req-9"}`, string(imm.Body))
	}

	t.Run("request headers", func(t *testing.T) {
		ec, r := newCtx(t)
		resp, err := TranslateRequestHeaderActions(
			&executor.RequestHeaderExecutionResult{ShortCircuited: true, FinalAction: rejection}, ec.policyChain, ec)
		require.NoError(t, err)
		assertRendered(t, resp)
		assert.Equal(t, 1, r.renderedRejections)
	})

	t.Run("request body", func(t *testing.T) {
		ec, r := newCtx(t)
		resp, err := TranslateRequestBodyActions(
			&executor.RequestExecutionResult{ShortCircuited: true, FinalAction: rejection}, ec.policyChain, ec)
		require.NoError(t, err)
		assertRendered(t, resp)
		assert.Equal(t, 1, r.renderedRejections)
	})

	t.Run("response phase", func(t *testing.T) {
		ec, r := newCtx(t)
		resp, err := TranslateResponseBodyActions(
			&executor.ResponseExecutionResult{ShortCircuited: true, FinalAction: rejection}, ec)
		require.NoError(t, err)
		assertRendered(t, resp)
		assert.Equal(t, 1, r.renderedRejections)
	})
}

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
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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

	pending.chainKey = "POST|/message:send|example.com"
	bound := &recordingSpan{}
	pending.recordResolutionAttributes(bound)
	assert.Equal(t, map[string]string{
		constants.AttrResolverName:   "a2a-jsonrpc",
		constants.AttrPolicyChainKey: "POST|/message:send|example.com",
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
	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
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

	r := &fakeOperationResolver{name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method"}
	reg := resolver.NewRegistry()
	require.NoError(t, reg.Register(r))
	reg.Freeze()

	k := NewKernel()
	server := NewExternalProcessorServer(k, executor.NewChainExecutor(nil, nil, tp.Tracer("test")),
		config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes, reg)
	server.tracer = tp.Tracer("test")

	f := &resolutionFixture{server: server, kernel: k}
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

// ─── Response kind (streaming semantics from the operation) ──────────────────

// bufferedResponsePolicy needs the whole response body — a guardrail or a redaction step.
// It does not implement policy.StreamingResponsePolicy, so a chain containing it reports
// SupportsResponseStreaming == false.
type bufferedResponsePolicy struct{ testutils.NoopPolicy }

func (p *bufferedResponsePolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseBodyMode: policy.BodyModeBuffer}
}

// chunkedJSONHeaders is an ordinary unary response that happens to use chunked transfer
// framing — common, and not a stream.
func chunkedJSONHeaders() *extprocv3.HttpHeaders {
	return &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
		{Key: ":status", RawValue: []byte("400")},
		{Key: "content-type", RawValue: []byte("application/json")},
		{Key: "transfer-encoding", RawValue: []byte("chunked")},
	}}}
}

// jsonErrorHeaders is an ordinary unary response: exactly what a streaming operation
// returns when the request was bad. A body follows, so EndOfStream is false.
func jsonErrorHeaders() *extprocv3.HttpHeaders {
	return &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
		{Key: ":status", RawValue: []byte("400")},
		{Key: "content-type", RawValue: []byte("application/json")},
	}}}
}

// bodylessHeaders is a 204: the response is complete at the headers.
func bodylessHeaders() *extprocv3.HttpHeaders {
	return &extprocv3.HttpHeaders{
		Headers:     &corev3.HeaderMap{Headers: []*corev3.HeaderValue{{Key: ":status", RawValue: []byte("204")}}},
		EndOfStream: true,
	}
}

// sseHeaders is an upstream response that is genuinely a stream.
func sseHeaders() *extprocv3.HttpHeaders {
	return &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
		{Key: ":status", RawValue: []byte("200")},
		{Key: "content-type", RawValue: []byte("text/event-stream")},
	}}}
}

// A multiplexed route carries operations of both kinds, so the resolver's answer is what
// decides — the route cannot, and neither can the chain.
func TestResponseKind_ResolverDecidesPerOperation(t *testing.T) {
	for _, tc := range []struct {
		operation string
		kind      resolver.ResponseKind
	}{
		{"SendMessage", resolver.ResponseKindUnary},
		{"SendStreamingMessage", resolver.ResponseKindStreaming},
	} {
		t.Run(tc.operation, func(t *testing.T) {
			r := &fakeOperationResolver{
				name: "body", reqs: resolver.Requirements{BufferBody: true}, bodyField: "method",
				responseKindByOperation: map[string]resolver.ResponseKind{
					"SendMessage":          resolver.ResponseKindUnary,
					"SendStreamingMessage": resolver.ResponseKindStreaming,
				},
			}
			f := newResolutionFixture(t, r)
			f.route("POST|/rpc|example.com", resolver.RouteResolution{ResolverName: "body"})
			f.operationChain(tc.operation, &testutils.NoopPolicy{})

			execCtx := f.bindPending(t, "POST|/rpc|example.com")
			_, err := execCtx.processRequestBody(context.Background(), &extprocv3.HttpBody{
				Body: []byte(`{"method":"` + tc.operation + `"}`), EndOfStream: true,
			})
			require.NoError(t, err)
			require.NotNil(t, execCtx.policyChain, "the request must have bound a chain")
			assert.Equal(t, tc.kind, execCtx.responseKind)
		})
	}
}

// An identity route can be an operation route too (one A2A HTTP+JSON path per operation),
// where no resolver runs and the controller stamped the kind at deploy time.
func TestResponseKind_IdentityRouteCarriesItsOwn(t *testing.T) {
	f := newResolutionFixture(t)
	f.route("POST|/message:stream|example.com", resolver.RouteResolution{
		ResponseKind: resolver.ResponseKindStreaming,
	})
	f.chain("POST|/message:stream|example.com", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, outcome, _ := f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/message:stream|example.com", true,
			map[string]string{":method": "POST"}), &execCtx)

	require.Equal(t, bindReady, outcome)
	require.NotNil(t, execCtx)
	assert.Equal(t, resolver.ResponseKindStreaming, execCtx.responseKind)
}

// A unary operation is never streamed even when the upstream response looks like a
// stream: the operation says one complete response is coming, and a response-body policy
// needs it whole.
func TestResponseKind_UnaryIsNeverStreamed(t *testing.T) {
	f := newResolutionFixture(t)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{ResponseKind: resolver.ResponseKindUnary})
	f.chain("POST|/rpc|example.com", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, _, _ = f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/rpc|example.com", true, map[string]string{":method": "POST"}), &execCtx)
	require.NotNil(t, execCtx)

	_, err := execCtx.processResponseHeaders(context.Background(), sseHeaders())
	require.NoError(t, err)
	assert.False(t, execCtx.isStreamingResponse)
}

// The requirement this whole field exists for: a streaming operation bound to a chain
// that cannot stream its response body must fail closed.
//
// Buffering would hand the caller nothing until the upstream closes — indefinitely, for a
// long-running task — and present as a hang rather than an error. Skipping the offending
// policy would silently drop a guardrail, selectable by asking for a streaming operation.
func TestResponseKind_StreamingWithBufferedOnlyChainFailsClosed(t *testing.T) {
	f := newResolutionFixture(t)
	f.route("POST|/message:stream|example.com", resolver.RouteResolution{
		ResponseKind: resolver.ResponseKindStreaming,
	})
	f.chain("POST|/message:stream|example.com", &bufferedResponsePolicy{})

	var execCtx *PolicyExecutionContext
	_, _, _ = f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/message:stream|example.com", true, map[string]string{":method": "POST"}), &execCtx)
	require.NotNil(t, execCtx)

	resp, err := execCtx.processResponseHeaders(context.Background(), sseHeaders())
	require.NoError(t, err)

	imm := resp.GetImmediateResponse()
	require.NotNil(t, imm, "the response must be replaced, not buffered and not passed through")
	assert.Equal(t, typev3.StatusCode_InternalServerError, imm.Status.Code,
		"a deployment problem, so 500 — the caller did nothing wrong")

	// Sterile: the body names no policy, no chain, and not even streaming. An operator
	// correlates via the error id, which is also in the log.
	assert.Regexp(t, `^\{"error":"Internal Server Error","error_id":"[0-9a-f-]{36}"\}$`, string(imm.Body))
	assert.NotContains(t, string(imm.Body), "stream")
}

// The most ordinary streaming deployment there is: a chain that only authenticates, with
// no response-body policy at all. BuildPolicyChain reports SupportsResponseStreaming ==
// false for it — there is nothing to stream — so gating on that flag alone would reject
// this. It must proceed, with Envoy told to send no response body.
func TestResponseKind_StreamingWithNoResponseBodyPolicyProceeds(t *testing.T) {
	f := newResolutionFixture(t)
	f.route("POST|/message:stream|example.com", resolver.RouteResolution{
		ResponseKind: resolver.ResponseKindStreaming,
	})
	f.chain("POST|/message:stream|example.com", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, _, _ = f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/message:stream|example.com", true, map[string]string{":method": "POST"}), &execCtx)
	require.NotNil(t, execCtx)

	resp, err := execCtx.processResponseHeaders(context.Background(), sseHeaders())
	require.NoError(t, err)
	assert.Nil(t, resp.GetImmediateResponse())
	assert.NotNil(t, resp.GetResponseHeaders())
}

// No behavioural change for existing kinds, in the case that matters: a route with a
// *buffered-only* response
// policy meeting a chunked or SSE upstream response. That combination has always simply
// buffered, and it must keep doing so — the fail-closed behaviour is opt-in with an explicit
// streaming declaration, not a new global rule. The weaker version of this test used a
// no-body policy and therefore never reached the conflict check at all.
func TestResponseKind_AutoWithBufferedOnlyChainStillBuffers(t *testing.T) {
	for name, headers := range map[string]*extprocv3.HttpHeaders{
		"sse upstream":     sseHeaders(),
		"chunked upstream": chunkedJSONHeaders(),
	} {
		t.Run(name, func(t *testing.T) {
			f := newResolutionFixture(t)
			f.route("GET|/chat|example.com", resolver.RouteResolution{}) // no declared kind
			f.chain("GET|/chat|example.com", &bufferedResponsePolicy{})

			var execCtx *PolicyExecutionContext
			_, _, _ = f.server.initializeExecutionContext(context.Background(),
				headersRequest("GET|/chat|example.com", true, map[string]string{":method": "GET"}), &execCtx)
			require.NotNil(t, execCtx)
			require.Equal(t, resolver.ResponseKindAuto, execCtx.responseKind)

			resp, err := execCtx.processResponseHeaders(context.Background(), headers)
			require.NoError(t, err)
			assert.Nil(t, resp.GetImmediateResponse(),
				"an existing kind must never be failed closed by a check it never opted into")
			assert.NotNil(t, resp.GetResponseHeaders())
			assert.False(t, execCtx.isStreamingResponse, "and it buffers, exactly as before")
		})
	}
}

// A declared streaming operation is held to the positive SSE signal, so a chunked JSON
// error body is the unary response it actually is rather than a stream that cannot be
// served.
func TestResponseKind_StreamingWithChunkedJSONIsNotAStream(t *testing.T) {
	f := newResolutionFixture(t)
	f.route("POST|/message:stream|example.com", resolver.RouteResolution{
		ResponseKind: resolver.ResponseKindStreaming,
	})
	f.chain("POST|/message:stream|example.com", &bufferedResponsePolicy{})

	var execCtx *PolicyExecutionContext
	_, _, _ = f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/message:stream|example.com", true, map[string]string{":method": "POST"}), &execCtx)
	require.NotNil(t, execCtx)

	resp, err := execCtx.processResponseHeaders(context.Background(), chunkedJSONHeaders())
	require.NoError(t, err)
	assert.Nil(t, resp.GetImmediateResponse())
	assert.False(t, execCtx.isStreamingResponse)
}

// A route that declares nothing behaves exactly as it did before the field existed.
func TestResponseKind_AutoPreservesExistingBehaviour(t *testing.T) {
	f := newResolutionFixture(t)
	f.route("GET|/pets|example.com", resolver.RouteResolution{})
	f.chain("GET|/pets|example.com", &testutils.NoopPolicy{})

	var execCtx *PolicyExecutionContext
	_, _, _ = f.server.initializeExecutionContext(context.Background(),
		headersRequest("GET|/pets|example.com", true, map[string]string{":method": "GET"}), &execCtx)
	require.NotNil(t, execCtx)
	assert.Equal(t, resolver.ResponseKindAuto, execCtx.responseKind)

	// Derived exactly as before: this chain has no response-body policy, so there is
	// nothing to stream and the body is not sent to the engine at all.
	_, err := execCtx.processResponseHeaders(context.Background(), sseHeaders())
	require.NoError(t, err)
	assert.False(t, execCtx.isStreamingResponse)
	assert.Nil(t, resp2ImmediateResponse(t, execCtx), "an Auto route is never failed closed")
}

// resp2ImmediateResponse re-runs the response-header phase and returns any immediate
// response, so a test can assert the fail-closed branch was not taken.
func resp2ImmediateResponse(t *testing.T, execCtx *PolicyExecutionContext) *extprocv3.ImmediateResponse {
	t.Helper()
	resp, err := execCtx.processResponseHeaders(context.Background(), sseHeaders())
	require.NoError(t, err)
	return resp.GetImmediateResponse()
}

func TestEffectiveResponseKind(t *testing.T) {
	route := func(k resolver.ResponseKind) *RouteConfig {
		return &RouteConfig{RouteResolution: resolver.RouteResolution{ResponseKind: k}}
	}
	res := func(k resolver.ResponseKind) resolver.Resolution {
		return resolver.Resolution{ResponseKind: k}
	}

	// The resolver wins: a multiplexed route carries both kinds, so only the resolver
	// knows which one this request is.
	assert.Equal(t, resolver.ResponseKindStreaming,
		effectiveResponseKind(route(resolver.ResponseKindUnary), res(resolver.ResponseKindStreaming)))
	// The route answers when the resolver has nothing to say.
	assert.Equal(t, resolver.ResponseKindStreaming,
		effectiveResponseKind(route(resolver.ResponseKindStreaming), res(resolver.ResponseKindAuto)))
	// An unrecognised value degrades to Auto — the pre-existing behaviour — rather than
	// being guessed at in either direction.
	assert.Equal(t, resolver.ResponseKindAuto,
		effectiveResponseKind(route("chunked-maybe"), res("who-knows")))
	assert.Equal(t, resolver.ResponseKindAuto, effectiveResponseKind(nil, res(resolver.ResponseKindAuto)))
}

// A streaming operation is entitled to answer with an ordinary unary response, and a
// buffered-only chain handles that perfectly well. Failing closed on the operation alone
// would replace the agent's own 400 with a gateway 500 — hiding the actual error from the
// caller and blaming us for their bad request.
func TestResponseKind_StreamingOperationWithUnaryErrorResponseIsNotAConflict(t *testing.T) {
	f := newResolutionFixture(t)
	f.route("POST|/message:stream|example.com", resolver.RouteResolution{
		ResponseKind: resolver.ResponseKindStreaming,
	})
	f.chain("POST|/message:stream|example.com", &bufferedResponsePolicy{})

	var execCtx *PolicyExecutionContext
	_, _, _ = f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/message:stream|example.com", true, map[string]string{":method": "POST"}), &execCtx)
	require.NotNil(t, execCtx)

	resp, err := execCtx.processResponseHeaders(context.Background(), jsonErrorHeaders())
	require.NoError(t, err)
	assert.Nil(t, resp.GetImmediateResponse(), "the upstream's own 400 must survive")
	assert.NotNil(t, resp.GetResponseHeaders())

	// And the response-body policy still gets its buffered body, which is the whole
	// reason not to fail here: a guardrail should inspect the error body too.
	assert.False(t, execCtx.isStreamingResponse)
	assert.True(t, execCtx.policyChain.RequiresResponseBody)
}

// The same for a bodyless response: there is nothing to stream and nothing to buffer, so
// a 204 on a streaming operation must pass through rather than become a 500.
func TestResponseKind_StreamingOperationWithBodylessResponseIsNotAConflict(t *testing.T) {
	f := newResolutionFixture(t)
	f.route("POST|/message:stream|example.com", resolver.RouteResolution{
		ResponseKind: resolver.ResponseKindStreaming,
	})
	f.chain("POST|/message:stream|example.com", &bufferedResponsePolicy{})

	var execCtx *PolicyExecutionContext
	_, _, _ = f.server.initializeExecutionContext(context.Background(),
		headersRequest("POST|/message:stream|example.com", true, map[string]string{":method": "POST"}), &execCtx)
	require.NotNil(t, execCtx)

	resp, err := execCtx.processResponseHeaders(context.Background(), bodylessHeaders())
	require.NoError(t, err)
	assert.Nil(t, resp.GetImmediateResponse())
	assert.NotNil(t, resp.GetResponseHeaders())
	assert.False(t, execCtx.isStreamingResponse)
}

// responseNeedsStreaming is about the response, not the operation. Pinning it directly
// keeps the conflict check from drifting back to "the operation says streaming, therefore
// this response is a stream".
func TestResponseNeedsStreaming(t *testing.T) {
	newCtx := func(kind resolver.ResponseKind) *PolicyExecutionContext {
		f := newResolutionFixture(t)
		f.route("POST|/rpc|example.com", resolver.RouteResolution{ResponseKind: kind})
		f.chain("POST|/rpc|example.com", &bufferedResponsePolicy{})

		var execCtx *PolicyExecutionContext
		_, _, _ = f.server.initializeExecutionContext(context.Background(),
			headersRequest("POST|/rpc|example.com", true, map[string]string{":method": "POST"}), &execCtx)
		require.NotNil(t, execCtx)
		return execCtx
	}

	tests := []struct {
		name    string
		kind    resolver.ResponseKind
		headers *extprocv3.HttpHeaders
		want    bool
	}{
		{"streaming operation, SSE response", resolver.ResponseKindStreaming, sseHeaders(), true},
		{"streaming operation, JSON error response", resolver.ResponseKindStreaming, jsonErrorHeaders(), false},
		{"streaming operation, bodyless response", resolver.ResponseKindStreaming, bodylessHeaders(), false},
		{"streaming operation, chunked JSON response", resolver.ResponseKindStreaming, chunkedJSONHeaders(), false},
		{"auto operation, SSE response", resolver.ResponseKindAuto, sseHeaders(), true},
		{"auto operation, chunked response", resolver.ResponseKindAuto, chunkedJSONHeaders(), true},
		{"unary operation, SSE response", resolver.ResponseKindUnary, sseHeaders(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execCtx := newCtx(tt.kind)
			execCtx.buildResponseContexts(tt.headers)
			assert.Equal(t, tt.want, execCtx.responseNeedsStreaming(tt.headers.EndOfStream))
		})
	}
}

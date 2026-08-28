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
	"context"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// ─── The resolution reaches the policies ─────────────────────────────────────

// Without this the operation exists in the kernel and on spans but is invisible to the
// policy layer, and an A2A analytics event cannot be labelled by operation at all:
// OperationPath is identical for every JSON-RPC method, since they share one route.
func TestApplyBoundResolution_ExposesTheOperationAndAttributesToPolicies(t *testing.T) {
	f := newResolutionFixture(t)
	ec := newPolicyExecutionContext(f.server, "POST|/rpc|example.com", &registry.PolicyChain{})
	ec.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}},
		RouteMetadata{RouteName: "POST|/rpc|example.com", APIKind: string(policy.APIKindAgent)})

	attrs := map[string]string{
		"a2a.operation":        "SendMessage",
		"a2a.transport":        "JSONRPC",
		"a2a.protocol.version": "1.0",
		"a2a.context.id":       "ctx-1",
	}
	ec.applyBoundResolution(resolver.BoundResolution{
		ChainKey:   "POST|/SendMessage|example.com",
		Operation:  "SendMessage",
		Attributes: attrs,
	})

	// Every phase context shares one SharedContext, so a header-phase policy and a
	// body-phase policy see the same value — which is the whole reason this is not
	// carried on a per-phase context.
	for name, shared := range map[string]*policy.SharedContext{
		"request headers": ec.requestHeaderCtx.SharedContext,
		"request body":    ec.requestBodyCtx.SharedContext,
		"request stream":  ec.requestStreamContext.SharedContext,
	} {
		assert.Equal(t, "SendMessage", shared.ResolvedOperation, name)
		// Compared through the accessors, because that is all a policy has: the map
		// is unexported precisely so a policy cannot reach it.
		assert.Equal(t, len(attrs), shared.ResolutionAttributes.Len(), name)
		for attrName, want := range attrs {
			assert.Equal(t, want, shared.ResolutionAttributes.Get(attrName), name)
		}
	}

	assert.Equal(t, "SendMessage", ec.operation)
	// The engine-internal copy stays a plain map: resolvers build it and the span
	// stamp reads it, both inside the engine.
	assert.Equal(t, attrs, ec.resolutionAttributes)
}

// The guarantee the wrapper exists for. A route resolved at ingest builds its
// attributes once and shares them with every request on it, so if a policy could
// write through SharedContext, request A's contextId would appear on request B —
// silently, only under traffic, and indistinguishable downstream from a correlation
// bug.
//
// The compiler is what enforces it (policy.ResolutionAttributes exposes no mutation
// path), so what is asserted here is the consequence: two requests binding the same
// prepared resolution both read the resolver's values, and neither observes anything
// the other could have put there.
func TestApplyBoundResolution_TwoRequestsSharingAPreparedResolutionStayIsolated(t *testing.T) {
	f := newResolutionFixture(t)

	// One prepared resolution, as a static route holds it: built once at ingest and
	// handed to every request on that route.
	prepared := resolver.BoundResolution{
		ChainKey:  "POST|/GetTask|example.com",
		Operation: "GetTask",
		Attributes: map[string]string{
			"a2a.operation": "GetTask",
			"a2a.transport": "HTTP+JSON",
		},
	}

	newRequest := func() *policy.SharedContext {
		ec := newPolicyExecutionContext(f.server, "POST|/tasks|example.com", &registry.PolicyChain{})
		ec.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}},
			RouteMetadata{RouteName: "POST|/tasks|example.com", APIKind: string(policy.APIKindAgent)})
		ec.applyBoundResolution(prepared)
		return ec.sharedCtx
	}

	first, second := newRequest(), newRequest()

	for name, shared := range map[string]*policy.SharedContext{"first": first, "second": second} {
		assert.Equal(t, "HTTP+JSON", shared.ResolutionAttributes.Get("a2a.transport"), name)
		assert.Equal(t, 2, shared.ResolutionAttributes.Len(), name,
			"neither request may observe an attribute the resolver did not produce")
	}

	// And the prepared map itself is unchanged, so a third request on this route
	// would see the same thing.
	assert.Equal(t, map[string]string{
		"a2a.operation": "GetTask",
		"a2a.transport": "HTTP+JSON",
	}, prepared.Attributes)
}

// A directly-resolved route — every API kind that shipped before Agent — must be
// untouched. A policy reading these fields has to be able to treat the zero value as
// "not applicable" rather than as a failure to resolve.
func TestApplyBoundResolution_IsANoOpForADirectlyResolvedRoute(t *testing.T) {
	f := newResolutionFixture(t)
	ec := newPolicyExecutionContext(f.server, "GET|/pets|example.com", &registry.PolicyChain{})
	ec.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}},
		RouteMetadata{RouteName: "GET|/pets|example.com", APIKind: string(policy.APIKindRestApi)})

	// What resolver.Bind produces for a direct route: a chain key and nothing else.
	ec.applyBoundResolution(resolver.BoundResolution{ChainKey: "GET|/pets|example.com"})

	assert.Empty(t, ec.requestHeaderCtx.ResolvedOperation)
	// The zero value, which reads as the empty set: a policy on a directly-resolved
	// route needs no nil check before calling Get.
	assert.Equal(t, 0, ec.requestHeaderCtx.ResolutionAttributes.Len())
	assert.Equal(t, "", ec.requestHeaderCtx.ResolutionAttributes.Get("a2a.context.id"))
	assert.Nil(t, ec.resolutionAttributes)
	assert.Empty(t, ec.operation)
}

// The deferred path binds at the request-body callback, and the chain's own header
// policies run immediately after. The resolution must be in place before they do, or a
// policy on a JSON-RPC route sees the empty value the header callback left behind.
func TestDeferredBinding_SharedContextCarriesTheOperationBeforeTheChainRuns(t *testing.T) {
	r := &fakeOperationResolver{
		name:      "body",
		reqs:      resolver.RequestRequirements{Body: resolver.BodyBuffered},
		bodyField: "method",
		attributes: map[string]string{
			"a2a.transport":  "JSONRPC",
			"a2a.context.id": "ctx-7",
		},
	}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{ResolverName: "body"})

	// A policy that records what it saw of the shared context when it ran.
	spy := &sharedContextSpy{}
	f.operationChain("SendMessage", spy)

	execCtx := f.bindPending(t, "POST|/rpc|example.com")
	require.Empty(t, execCtx.requestHeaderCtx.ResolvedOperation,
		"nothing is resolved yet at the header callback of a deferred route")

	_, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true})
	require.NoError(t, err)
	require.True(t, execCtx.boundAtBodyPhase)

	require.True(t, spy.ran, "the bound chain's header policy must have run")
	assert.Equal(t, "SendMessage", spy.operation,
		"the policy must see the operation whose chain it is part of")
	assert.Equal(t, "ctx-7", spy.attributes.Get("a2a.context.id"),
		"a body-sourced identifier must reach a header-phase policy, which cannot read the body itself")
}

// ─── The resolution reaches analytics ────────────────────────────────────────

// Stamped by the kernel rather than by the analytics policy, so it is present even
// when that conditionally-injected policy is not in the chain.
func TestBuildAnalyticsStruct_StampsTheResolvedOperation(t *testing.T) {
	execCtx := &PolicyExecutionContext{sharedCtx: &policy.SharedContext{
		APIKind:           policy.APIKindAgent,
		APIName:           "WeatherAgent",
		ResolvedOperation: "SendMessage",
	}}

	built, err := buildAnalyticsStruct(map[string]any{}, execCtx)
	require.NoError(t, err)
	assert.Equal(t, structpb.NewStringValue("SendMessage"), built.Fields[ResolvedOperationKey])
}

// A directly-resolved route contributes no operation field at all, rather than an
// empty one: a consumer must be able to tell "this kind has no operation dimension"
// from "the operation was not determined".
func TestBuildAnalyticsStruct_OmitsTheOperationForADirectRoute(t *testing.T) {
	execCtx := &PolicyExecutionContext{sharedCtx: &policy.SharedContext{
		APIKind: policy.APIKindRestApi,
		APIName: "PetStore",
	}}

	built, err := buildAnalyticsStruct(map[string]any{}, execCtx)
	require.NoError(t, err)
	assert.NotContains(t, built.Fields, ResolvedOperationKey)
}

// A policy denial and an upstream's own 401 arrive downstream as the same status. Only
// the engine can tell them apart, and a success-rate dashboard that cannot separate
// them blames the agent for the gateway's rejections.
func TestShortCircuitAnalytics_AttributesTheOutcomeToThePolicyLayer(t *testing.T) {
	f := newResolutionFixture(t)
	ec := newPolicyExecutionContext(f.server, "POST|/rpc|example.com", &registry.PolicyChain{})
	ec.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}},
		RouteMetadata{RouteName: "POST|/rpc|example.com", APIKind: string(policy.APIKindAgent)})

	analytics := collectShortCircuitAnalytics(ec, nil, nil, policy.ImmediateResponse{
		StatusCode:        429,
		AnalyticsMetadata: map[string]any{"denied_by": "quota"},
	})

	assert.Equal(t, constants.TerminalReasonPolicyDenied, analytics[TerminalReasonKey])
	assert.Equal(t, "quota", analytics["denied_by"],
		"the rejecting policy's own metadata is unaffected")
}

// A policy must not be able to claim the engine's own attribution: presenting an
// upstream failure as a policy denial (or the reverse) would misattribute it on every
// dashboard downstream.
func TestShortCircuitAnalytics_PolicyCannotOverrideTheTerminalReason(t *testing.T) {
	f := newResolutionFixture(t)
	ec := newPolicyExecutionContext(f.server, "POST|/rpc|example.com", &registry.PolicyChain{})
	ec.buildRequestContexts(&extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}},
		RouteMetadata{RouteName: "POST|/rpc|example.com"})

	analytics := collectShortCircuitAnalytics(ec, nil, nil, policy.ImmediateResponse{
		StatusCode:        429,
		AnalyticsMetadata: map[string]any{TerminalReasonKey: "upstream_response"},
	})

	assert.Equal(t, constants.TerminalReasonPolicyDenied, analytics[TerminalReasonKey])
}

// The A2A dimensions are contributed by a policy running at the *request-header*
// phase, and on a JSON-RPC route that phase runs at the request-body callback. The
// body-phase response is therefore the only one that can carry them to the access log,
// alongside the operation the kernel stamps itself. Asserting both on one response is
// what proves the two halves of the A2A event actually meet.
func TestDeferredBinding_AnalyticsCarriesBothTheOperationAndTheHeaderPhaseDimensions(t *testing.T) {
	r := &fakeOperationResolver{
		name:      "body",
		reqs:      resolver.RequestRequirements{Body: resolver.BodyBuffered},
		bodyField: "method",
	}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{ResolverName: "body"})
	f.operationChain("SendMessage", &headerPhaseAnalyticsPolicy{
		metadata: map[string]any{"a2a_request_properties": `{"transport":"JSONRPC"}`},
	})

	execCtx := f.bindPendingWithHeaders(t, "POST|/rpc|example.com",
		map[string]string{":method": "POST", ":path": "/rpc"})
	resp, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"method":"SendMessage"}`), EndOfStream: true})
	require.NoError(t, err)

	analytics := analyticsFromResponse(t, resp)
	assert.Equal(t, "SendMessage", analytics[ResolvedOperationKey],
		"the kernel-stamped operation must be on the response the access log reads")
	assert.Equal(t, `{"transport":"JSONRPC"}`, analytics["a2a_request_properties"],
		"a header-phase policy's dimensions must survive the deferred path")
}

// A rejection is the only ext_proc response a resolution failure produces. It has to
// carry the API's identity and the reason, or the request is invisible to analytics —
// which for a protocol whose operation lives in the payload is the failure mode most
// likely to be the caller's own.
func TestResolutionFailure_CarriesTheAPIIdentityAndReasonToAnalytics(t *testing.T) {
	r := &fakeOperationResolver{
		name:      "body",
		reqs:      resolver.RequestRequirements{Body: resolver.BodyBuffered},
		bodyField: "method",
	}
	f := newResolutionFixture(t, r)
	f.route("POST|/rpc|example.com", resolver.RouteResolution{ResolverName: "body"})

	execCtx := f.bindPending(t, "POST|/rpc|example.com")
	// A body that names no operation: the resolver rejects it and no chain is bound.
	resp, err := execCtx.processRequestBody(context.Background(),
		&extprocv3.HttpBody{Body: []byte(`{"nope":1}`), EndOfStream: true})
	require.NoError(t, err)
	require.NotNil(t, resp.GetImmediateResponse(), "the request must have been rejected")
	require.True(t, execCtx.resolutionDenied)

	analytics := analyticsFromResponse(t, resp)
	assert.Equal(t, constants.TerminalReasonResolutionFailed, analytics[TerminalReasonKey])
	assert.Equal(t, testAPIID, analytics[APIIDKey],
		"the event must be attributable to an API")

	// The failure kind reaches the log, the metric and the span. Publishing it would
	// tell a caller which specific malformation it achieved.
	for key, value := range analytics {
		assert.NotContains(t, value, "invalid-request",
			"the failure kind must not leave the process on an event (key %s)", key)
	}
}

// ─── The resolution reaches traces ───────────────────────────────────────────

// contextId/taskId/messageId are caller-controlled and unbounded, which is precisely
// why they belong on a span — per-request storage — and not on a metric label, whose
// cardinality they would multiply.
func TestRecordResolutionAttributes_CarriesTheProtocolRequestFacts(t *testing.T) {
	f := newResolutionFixture(t)
	ec := newPolicyExecutionContext(f.server, "POST|/rpc|example.com", nil)
	ec.resolverName = "a2a"
	ec.chainKey = "POST|/SendMessage|example.com"
	ec.operation = "SendMessage"
	ec.resolutionAttributes = map[string]string{
		"a2a.transport":  "JSONRPC",
		"a2a.context.id": "ctx-1",
		"a2a.task.id":    "task-1",
	}

	span := &recordingSpan{}
	ec.recordResolutionAttributes(span)

	assert.Equal(t, map[string]string{
		constants.AttrResolverName:      "a2a",
		constants.AttrPolicyChainKey:    "POST|/SendMessage|example.com",
		constants.AttrResolvedOperation: "SendMessage",
		"a2a.transport":                 "JSONRPC",
		"a2a.context.id":                "ctx-1",
		"a2a.task.id":                   "task-1",
	}, span.attrs)
}

// An identity route has no resolver and stamps nothing, attributes included.
func TestRecordResolutionAttributes_StillStampsNothingForAnIdentityRoute(t *testing.T) {
	f := newResolutionFixture(t)
	ec := newPolicyExecutionContext(f.server, "GET|/pets|example.com", nil)
	ec.chainKey = "GET|/pets|example.com"

	span := &recordingSpan{}
	ec.recordResolutionAttributes(span)
	assert.Empty(t, span.attrs)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// sharedContextSpy records what the shared context said when the policy ran, which is
// the only way to assert ordering between binding and chain execution.
type sharedContextSpy struct {
	testutils.NoopPolicy
	ran        bool
	operation  string
	attributes policy.ResolutionAttributes
}

func (s *sharedContextSpy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestHeaderMode: policy.HeaderModeProcess}
}

func (s *sharedContextSpy) OnRequestHeaders(
	_ context.Context,
	reqCtx *policy.RequestHeaderContext,
	_ map[string]interface{},
) policy.RequestHeaderAction {
	s.ran = true
	s.operation = reqCtx.ResolvedOperation
	s.attributes = reqCtx.ResolutionAttributes
	return policy.UpstreamRequestHeaderModifications{}
}

// headerPhaseAnalyticsPolicy contributes analytics metadata from the request-header
// phase, standing in for the analytics system policy (which lives in another module).
type headerPhaseAnalyticsPolicy struct {
	testutils.NoopPolicy
	metadata map[string]any
}

func (p *headerPhaseAnalyticsPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestHeaderMode: policy.HeaderModeProcess}
}

func (p *headerPhaseAnalyticsPolicy) OnRequestHeaders(
	_ context.Context,
	_ *policy.RequestHeaderContext,
	_ map[string]interface{},
) policy.RequestHeaderAction {
	return policy.UpstreamRequestHeaderModifications{AnalyticsMetadata: p.metadata}
}

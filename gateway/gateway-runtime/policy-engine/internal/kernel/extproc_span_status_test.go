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
	"errors"
	"net/http"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// =============================================================================
// Test fixtures: a policy that short-circuits with a configurable status, and
// a response-body policy that overrides the downstream status.
// =============================================================================

// denyHeaderPolicy short-circuits the request-headers phase with a fixed status.
type denyHeaderPolicy struct {
	statusCode int
}

func (p *denyHeaderPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestHeaderMode: policy.HeaderModeProcess}
}

func (p *denyHeaderPolicy) OnRequestHeaders(_ context.Context, _ *policy.RequestHeaderContext, _ map[string]interface{}) policy.RequestHeaderAction {
	return policy.ImmediateResponse{StatusCode: p.statusCode}
}

// statusOverrideResponsePolicy rewrites the downstream response status during
// the response-body phase, mirroring a policy using DownstreamResponseModifications.StatusCode.
type statusOverrideResponsePolicy struct {
	statusCode int
}

func (p *statusOverrideResponsePolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseBodyMode: policy.BodyModeBuffer}
}

func (p *statusOverrideResponsePolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	code := p.statusCode
	return policy.DownstreamResponseModifications{StatusCode: &code}
}

// analyticsMarshalFailurePolicy returns AnalyticsMetadata containing a value
// structpb/JSON cannot represent, forcing TranslateResponseBodyActions to
// return a genuine (nil, err) — a fatal ext_proc-level failure distinct from
// handlePolicyError, used to exercise the "phase fails after an earlier phase
// already memoized a terminal outcome" path.
type analyticsMarshalFailurePolicy struct{}

func (p *analyticsMarshalFailurePolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseBodyMode: policy.BodyModeBuffer}
}

func (p *analyticsMarshalFailurePolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	return policy.DownstreamResponseModifications{AnalyticsMetadata: map[string]interface{}{"bad": make(chan int)}}
}

// =============================================================================
// Test helpers
// =============================================================================

// newSpanStatusServer builds an ExternalProcessorServer wired to a real SDK
// TracerProvider (recorded by sr) instead of the noop tracer NewExternalProcessorServer
// would otherwise pick up from the global provider — this is what lets these tests
// assert on actual recorded span status/attributes.
func newSpanStatusServer(t *testing.T) (*ExternalProcessorServer, *Kernel, *tracetest.SpanRecorder) {
	t.Helper()
	return newSpanStatusServerWithCEL(t, nil)
}

// newSpanStatusServerWithCEL is like newSpanStatusServer but wires in a custom
// CELEvaluator (e.g. one that returns errors) for tests that need to drive the
// policy-error path without relying on deepCopyParams failure.
func newSpanStatusServerWithCEL(t *testing.T, cel executor.CELEvaluator) (*ExternalProcessorServer, *Kernel, *tracetest.SpanRecorder) {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	k := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, cel, tp.Tracer("test"))
	server := NewExternalProcessorServer(k, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)
	server.tracer = tp.Tracer("test") // package-internal field; avoids mutating global otel state
	return server, k, sr
}

// registerTestRoute wires chain into k under routeName, including the
// RouteConfig lookup that initializeExecutionContext needs.
func registerTestRoute(k *Kernel, routeName string, chain *registry.PolicyChain) {
	k.RegisterRoute(routeName, chain)
	rc := &RouteConfig{Metadata: RouteMetadata{RouteName: routeName}}
	// Prepared the same way ingest prepares it: an unprepared route is one the kernel
	// refuses to serve.
	if err := PrepareRoute(resolver.DefaultRegistry(), routeName, rc); err != nil {
		panic(err)
	}
	k.ApplyWholeRouteConfigs(map[string]*RouteConfig{routeName: rc})
}

// buildChainWithPolicy constructs a single-policy chain via the real
// BuildPolicyChain path (so Requires*/Supports* flags are computed from Mode()),
// registering impl under name/v1 with the given spec parameters.
func buildChainWithPolicy(t *testing.T, name string, impl policy.Policy, params map[string]interface{}) *registry.PolicyChain {
	t.Helper()
	return buildChainWithPolicyAndCondition(t, name, impl, params, nil)
}

// buildChainWithPolicyAndCondition is like buildChainWithPolicy but also sets an
// ExecutionCondition on the spec, making HasExecutionConditions=true on the chain.
func buildChainWithPolicyAndCondition(t *testing.T, name string, impl policy.Policy, params map[string]interface{}, condition *string) *registry.PolicyChain {
	t.Helper()
	reg := &registry.PolicyRegistry{Policies: make(map[string]*registry.PolicyEntry)}
	require.NoError(t, reg.SetConfig(map[string]interface{}{}))
	require.NoError(t, reg.Register(&policy.PolicyDefinition{Name: name, Version: "v1.0.0"},
		func(metadata policy.PolicyMetadata, params map[string]interface{}) (policy.Policy, error) {
			return impl, nil
		}))

	specs := []policy.PolicySpec{
		{
			Name:               name,
			Version:            "v1",
			Enabled:            true,
			Parameters:         policy.PolicyParameters{Raw: params},
			ExecutionCondition: condition,
		},
	}

	k := NewKernel()
	chain, err := k.BuildPolicyChain("test-route", specs, reg, policy.PolicyMetadata{})
	require.NoError(t, err)
	return chain
}

// mockEvaluatorError implements executor.CELEvaluator, returning an error on every
// evaluation call. Used to trigger the handlePolicyError path in extproc tests.
type mockEvaluatorError struct{}

func (m *mockEvaluatorError) EvaluateRequestHeaderCondition(_ string, _ *policy.RequestHeaderContext) (bool, error) {
	return false, errors.New("injected evaluator error")
}
func (m *mockEvaluatorError) EvaluateRequestBodyCondition(_ string, _ *policy.RequestContext) (bool, error) {
	return false, errors.New("injected evaluator error")
}
func (m *mockEvaluatorError) EvaluateResponseHeaderCondition(_ string, _ *policy.ResponseHeaderContext) (bool, error) {
	return false, errors.New("injected evaluator error")
}
func (m *mockEvaluatorError) EvaluateResponseBodyCondition(_ string, _ *policy.ResponseContext) (bool, error) {
	return false, errors.New("injected evaluator error")
}
func (m *mockEvaluatorError) EvaluateStreamingRequestCondition(_ string, _ *policy.RequestStreamContext) (bool, error) {
	return false, errors.New("injected evaluator error")
}
func (m *mockEvaluatorError) EvaluateStreamingResponseCondition(_ string, _ *policy.ResponseStreamContext) (bool, error) {
	return false, errors.New("injected evaluator error")
}

func emptyChain() *registry.PolicyChain {
	return &registry.PolicyChain{
		Policies:    []policy.Policy{},
		PolicySpecs: []policy.PolicySpec{},
	}
}

func routeAttributes(routeName string) map[string]*structpb.Struct {
	return map[string]*structpb.Struct{
		constants.ExtProcFilter: {
			Fields: map[string]*structpb.Value{
				"xds.route_name": structpb.NewStringValue(routeName),
			},
		},
	}
}

func requestHeadersReq(routeName, method, path string) *extprocv3.ProcessingRequest {
	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{
					Headers: []*corev3.HeaderValue{
						{Key: ":path", RawValue: []byte(path)},
						{Key: ":method", RawValue: []byte(method)},
						{Key: ":authority", RawValue: []byte("api.example.com")},
						{Key: ":scheme", RawValue: []byte("https")},
					},
				},
				EndOfStream: true,
			},
		},
		Attributes: routeAttributes(routeName),
	}
}

func responseHeadersReq(status string) *extprocv3.ProcessingRequest {
	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_ResponseHeaders{
			ResponseHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{
					Headers: []*corev3.HeaderValue{
						{Key: ":status", RawValue: []byte(status)},
					},
				},
			},
		},
	}
}

func responseBodyReq(body string) *extprocv3.ProcessingRequest {
	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_ResponseBody{
			ResponseBody: &extprocv3.HttpBody{
				Body:        []byte(body),
				EndOfStream: true,
			},
		},
	}
}

func spanByName(spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	for _, s := range spans {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

func attrOf(span sdktrace.ReadOnlySpan, key attribute.Key) (attribute.Value, bool) {
	if span == nil {
		return attribute.Value{}, false
	}
	for _, kv := range span.Attributes() {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func statusCodeAttr(t *testing.T, span sdktrace.ReadOnlySpan) (int64, bool) {
	t.Helper()
	v, ok := attrOf(span, semconv.HTTPResponseStatusCodeKey)
	if !ok {
		return 0, false
	}
	return v.AsInt64(), true
}

func reasonAttr(span sdktrace.ReadOnlySpan) (string, bool) {
	v, ok := attrOf(span, attribute.Key(constants.AttrTerminalReason))
	if !ok {
		return "", false
	}
	return v.AsString(), true
}

// =============================================================================
// Tests
// =============================================================================

func TestProcessSpanStatus_NoPolicyChain(t *testing.T) {
	server, _, sr := newSpanStatusServer(t)

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("missing-route", "GET", "/pets"),
	})

	err := server.Process(stream)
	require.NoError(t, err)

	spans := sr.Ended()
	root := spanByName(spans, constants.SpanExternalProcessingProcess)
	phase := spanByName(spans, constants.SpanProcessRequestHeaders)
	require.NotNil(t, root)
	require.NotNil(t, phase)

	for _, s := range []sdktrace.ReadOnlySpan{root, phase} {
		assert.Equal(t, codes.Error, s.Status().Code)
		code, ok := statusCodeAttr(t, s)
		require.True(t, ok)
		assert.Equal(t, int64(500), code)
		reason, ok := reasonAttr(s)
		require.True(t, ok)
		assert.Equal(t, constants.TerminalReasonNoPolicyChain, reason)
	}
}

func TestProcessSpanStatus_UnknownMessageType(t *testing.T) {
	server, _, sr := newSpanStatusServer(t)

	stream := newMockStream([]*extprocv3.ProcessingRequest{{}})

	err := server.Process(stream)
	require.NoError(t, err)

	spans := sr.Ended()
	root := spanByName(spans, constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	assert.Equal(t, codes.Error, root.Status().Code)
	code, ok := statusCodeAttr(t, root)
	require.True(t, ok)
	assert.Equal(t, int64(500), code)
	reason, ok := reasonAttr(root)
	require.True(t, ok)
	assert.Equal(t, constants.TerminalReasonUnknownMessageType, reason)

	// No phase span should have been created for an unrecognised request type.
	assert.Nil(t, spanByName(spans, constants.SpanProcessRequestHeaders))
}

func TestProcessSpanStatus_PolicyError500(t *testing.T) {
	// A CEL evaluator that always errors drives ExecuteRequestHeaderPolicies (and
	// thus processRequestHeaders) into handlePolicyError, returning HTTP 500.
	cond := "some.condition == true"
	server, k, sr := newSpanStatusServerWithCEL(t, &mockEvaluatorError{})
	chain := buildChainWithPolicyAndCondition(t, "cel-error-policy", &denyHeaderPolicy{statusCode: 403},
		map[string]interface{}{}, &cond)
	registerTestRoute(k, "test-route", chain)

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("test-route", "GET", "/pets"),
	})

	err := server.Process(stream)
	require.NoError(t, err)
	require.Len(t, stream.responses, 1)

	imm := stream.responses[0].GetImmediateResponse()
	require.NotNil(t, imm)
	assert.EqualValues(t, 500, imm.GetStatus().GetCode())
	var errorID string
	for _, h := range imm.GetHeaders().GetSetHeaders() {
		if h.GetHeader().GetKey() == "x-error-id" {
			errorID = string(h.GetHeader().GetRawValue())
		}
	}
	require.NotEmpty(t, errorID)

	spans := sr.Ended()
	root := spanByName(spans, constants.SpanExternalProcessingProcess)
	phase := spanByName(spans, constants.SpanProcessRequestHeaders)
	require.NotNil(t, root)
	require.NotNil(t, phase)

	for _, s := range []sdktrace.ReadOnlySpan{root, phase} {
		assert.Equal(t, codes.Error, s.Status().Code)
		code, ok := statusCodeAttr(t, s)
		require.True(t, ok)
		assert.Equal(t, int64(500), code)
		reason, ok := reasonAttr(s)
		require.True(t, ok)
		assert.Equal(t, constants.TerminalReasonPolicyError, reason)
		errIDVal, ok := attrOf(s, attribute.Key(constants.AttrTerminalErrorID))
		require.True(t, ok)
		assert.Equal(t, errorID, errIDVal.AsString())
	}
}

func TestProcessSpanStatus_PolicyDeny401(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)

	chain := buildChainWithPolicy(t, "deny-401-policy", &denyHeaderPolicy{statusCode: 401}, map[string]interface{}{})
	registerTestRoute(k, "test-route", chain)

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("test-route", "GET", "/pets"),
	})

	err := server.Process(stream)
	require.NoError(t, err)

	spans := sr.Ended()
	root := spanByName(spans, constants.SpanExternalProcessingProcess)
	phase := spanByName(spans, constants.SpanProcessRequestHeaders)
	require.NotNil(t, root)
	require.NotNil(t, phase)

	for _, s := range []sdktrace.ReadOnlySpan{root, phase} {
		// Regression lock: a 4xx must never flip span status to Error.
		assert.Equal(t, codes.Unset, s.Status().Code)
		code, ok := statusCodeAttr(t, s)
		require.True(t, ok)
		assert.Equal(t, int64(401), code)
		reason, ok := reasonAttr(s)
		require.True(t, ok)
		assert.Equal(t, constants.TerminalReasonPolicyDenied, reason)
		assert.Empty(t, s.Events())
	}
}

func TestProcessSpanStatus_PolicyDeny500(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)

	// A policy-returned 500 (e.g. the python-bridge fault shape) is still
	// classified as policy_denied, not policy_error — only handlePolicyError/
	// handlePayloadTooLarge get the engine-generated reasons.
	chain := buildChainWithPolicy(t, "deny-500-policy", &denyHeaderPolicy{statusCode: 500}, map[string]interface{}{})
	registerTestRoute(k, "test-route", chain)

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("test-route", "GET", "/pets"),
	})

	err := server.Process(stream)
	require.NoError(t, err)

	spans := sr.Ended()
	root := spanByName(spans, constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	assert.Equal(t, codes.Error, root.Status().Code)
	code, ok := statusCodeAttr(t, root)
	require.True(t, ok)
	assert.Equal(t, int64(500), code)
	reason, ok := reasonAttr(root)
	require.True(t, ok)
	assert.Equal(t, constants.TerminalReasonPolicyDenied, reason)
}

// A policy can answer a request as well as refuse it: the A2A system policy
// serves a managed Agent Card with a 200, or a 304 when the client's
// If-None-Match still matches. Both reach the wire through the same
// short-circuit an auth denial uses, so only the status separates them — and
// filing them as policy_denied puts every card fetch in the rejection bucket of
// any dashboard or alert that counts by terminal reason.
func TestProcessSpanStatus_PolicyAnsweredIsNotADenial(t *testing.T) {
	for _, status := range []int{200, 304} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server, k, sr := newSpanStatusServer(t)

			chain := buildChainWithPolicy(t, "answer-policy",
				&denyHeaderPolicy{statusCode: status}, map[string]interface{}{})
			registerTestRoute(k, "test-route", chain)

			stream := newMockStream([]*extprocv3.ProcessingRequest{
				requestHeadersReq("test-route", "GET", "/.well-known/agent-card.json"),
			})

			require.NoError(t, server.Process(stream))

			root := spanByName(sr.Ended(), constants.SpanExternalProcessingProcess)
			require.NotNil(t, root)
			assert.Equal(t, codes.Unset, root.Status().Code)
			code, ok := statusCodeAttr(t, root)
			require.True(t, ok)
			assert.Equal(t, int64(status), code)
			reason, ok := reasonAttr(root)
			require.True(t, ok)
			assert.Equal(t, constants.TerminalReasonPolicyAnswered, reason)
		})
	}
}

// A short-circuit the engine cannot read as an answer stays a denial. A status
// of 0 is a malformed policy response — Envoy rejects it anyway — and must not
// be counted as a success by falling into the answered bucket.
func TestTerminalReasonForImmediateResponse(t *testing.T) {
	for status, want := range map[int]string{
		0:   constants.TerminalReasonPolicyDenied,
		100: constants.TerminalReasonPolicyDenied,
		200: constants.TerminalReasonPolicyAnswered,
		204: constants.TerminalReasonPolicyAnswered,
		304: constants.TerminalReasonPolicyAnswered,
		399: constants.TerminalReasonPolicyAnswered,
		400: constants.TerminalReasonPolicyDenied,
		401: constants.TerminalReasonPolicyDenied,
		429: constants.TerminalReasonPolicyDenied,
		500: constants.TerminalReasonPolicyDenied,
	} {
		assert.Equal(t, want, terminalReasonForImmediateResponse(status), "status %d", status)
	}
}

func TestProcessSpanStatus_UpstreamPassThrough_NeverError(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		wantStatus int64
	}{
		{"502 stays Unset", "502", 502},
		{"401 stays Unset", "401", 401},
		{"200 stays Unset", "200", 200},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, k, sr := newSpanStatusServer(t)
			registerTestRoute(k, "test-route", emptyChain())

			stream := newMockStream([]*extprocv3.ProcessingRequest{
				requestHeadersReq("test-route", "GET", "/pets"),
				responseHeadersReq(tc.status),
			})

			err := server.Process(stream)
			require.NoError(t, err)

			spans := sr.Ended()
			root := spanByName(spans, constants.SpanExternalProcessingProcess)
			phase := spanByName(spans, constants.SpanProcessResponseHeaders)
			require.NotNil(t, root)
			require.NotNil(t, phase)

			for _, s := range []sdktrace.ReadOnlySpan{root, phase} {
				assert.Equal(t, codes.Unset, s.Status().Code)
				code, ok := statusCodeAttr(t, s)
				require.True(t, ok)
				assert.Equal(t, tc.wantStatus, code)
				reason, ok := reasonAttr(s)
				require.True(t, ok)
				assert.Equal(t, constants.TerminalReasonUpstream, reason)
			}

			assert.NotEqual(t, codes.Ok, root.Status().Code)
			assert.Empty(t, root.Events())
		})
	}
}

// TestProcessSpanStatus_ResponseBodyStatusOverride is the guard that a policy-
// *chosen* 5xx (TerminalReasonPolicyStatusOverride) is not swept up by the
// upstream-fault exemption in RecordHTTPOutcome — only a genuine unmodified
// backend pass-through (TerminalReasonUpstream) gets that exemption.
func TestProcessSpanStatus_ResponseBodyStatusOverride(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)

	chain := buildChainWithPolicy(t, "status-override-policy", &statusOverrideResponsePolicy{statusCode: 503}, map[string]interface{}{})
	registerTestRoute(k, "test-route", chain)

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("test-route", "GET", "/pets"),
		responseHeadersReq("200"),
		responseBodyReq("OK"),
	})

	err := server.Process(stream)
	require.NoError(t, err)

	spans := sr.Ended()
	root := spanByName(spans, constants.SpanExternalProcessingProcess)
	phase := spanByName(spans, constants.SpanProcessResponseBody)
	require.NotNil(t, root)
	require.NotNil(t, phase)

	for _, s := range []sdktrace.ReadOnlySpan{root, phase} {
		assert.Equal(t, codes.Error, s.Status().Code)
		code, ok := statusCodeAttr(t, s)
		require.True(t, ok)
		assert.Equal(t, int64(503), code)
		reason, ok := reasonAttr(s)
		require.True(t, ok)
		assert.Equal(t, constants.TerminalReasonPolicyStatusOverride, reason,
			"a policy-set status must be distinguishable from a genuine upstream pass-through")
	}
}

// TestProcessSpanStatus_ResponseBodyFatalError_ClearsStaleMemo is a regression
// test for a stale-memo bug: the response-headers phase memoizes a successful
// {200, upstream_response} outcome, then the response-body phase fails closed
// with a genuine error (resp == nil) rather than going through
// handlePolicyError. Without clearing the memo, the root span would end up
// Error-status but still carry the earlier 200/upstream_response attributes.
func TestProcessSpanStatus_ResponseBodyFatalError_ClearsStaleMemo(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)

	chain := buildChainWithPolicy(t, "analytics-fail-policy", &analyticsMarshalFailurePolicy{}, map[string]interface{}{})
	registerTestRoute(k, "test-route", chain)

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("test-route", "GET", "/pets"),
		responseHeadersReq("200"), // memoizes {200, upstream_response} on execCtx.terminal
		responseBodyReq("OK"),     // fails with a genuine error, resp == nil
	})

	err := server.Process(stream)
	require.Error(t, err)

	root := spanByName(sr.Ended(), constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	assert.Equal(t, codes.Error, root.Status().Code)

	code, ok := statusCodeAttr(t, root)
	require.True(t, ok)
	assert.Equal(t, int64(500), code, "root span must not retain the stale 200 memoized by the response-headers phase")

	reason, ok := reasonAttr(root)
	require.True(t, ok)
	assert.Equal(t, constants.TerminalReasonProcessingFailed, reason)
}

// TestProcessSpanStatus_UnknownMessageType_ClearsStaleMemo is a regression test
// for the sibling stale-memo bug in the default (unknown ext_proc message)
// branch: it stamps parentSpan inline, but if execCtx already memoized a
// success outcome from an earlier phase, Process's root-span defer would run
// afterwards and clobber the correct 500/unknown_message_type attributes with
// the stale ones.
func TestProcessSpanStatus_UnknownMessageType_ClearsStaleMemo(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)
	registerTestRoute(k, "test-route", emptyChain())

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("test-route", "GET", "/pets"),
		responseHeadersReq("200"), // memoizes {200, upstream_response} on execCtx.terminal
		{},                        // unknown message type, arrives with execCtx already non-nil
	})

	err := server.Process(stream)
	require.NoError(t, err)

	root := spanByName(sr.Ended(), constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	assert.Equal(t, codes.Error, root.Status().Code)

	code, ok := statusCodeAttr(t, root)
	require.True(t, ok)
	assert.Equal(t, int64(500), code, "unknown-message-type outcome must not be overwritten by the earlier 200 pass-through memo")

	reason, ok := reasonAttr(root)
	require.True(t, ok)
	assert.Equal(t, constants.TerminalReasonUnknownMessageType, reason)
}

func TestProcessSpanStatus_HappyPath200(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)
	registerTestRoute(k, "test-route", emptyChain())

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("test-route", "GET", "/pets"),
		responseHeadersReq("200"),
	})

	err := server.Process(stream)
	require.NoError(t, err)

	spans := sr.Ended()
	root := spanByName(spans, constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)

	assert.Equal(t, codes.Unset, root.Status().Code)
	assert.NotEqual(t, codes.Ok, root.Status().Code)
	assert.Empty(t, root.Events())
	code, ok := statusCodeAttr(t, root)
	require.True(t, ok)
	assert.Equal(t, int64(200), code)
	reason, ok := reasonAttr(root)
	require.True(t, ok)
	assert.Equal(t, constants.TerminalReasonUpstream, reason)

	for _, s := range spans {
		assert.NotEqual(t, codes.Ok, s.Status().Code, "span %s must never be codes.Ok", s.Name())
	}
}

// TestProcessSpanStatus_RootSpanMemo proves the root-span defer in Process runs
// (LIFO, before span.End()) by asserting the ROOT span — not just the phase
// span — carries the terminal outcome from a request-headers-phase denial.
func TestProcessSpanStatus_RootSpanMemo(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)

	chain := buildChainWithPolicy(t, "deny-401-memo-policy", &denyHeaderPolicy{statusCode: 401}, map[string]interface{}{})
	registerTestRoute(k, "test-route", chain)

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("test-route", "GET", "/pets"),
	})

	err := server.Process(stream)
	require.NoError(t, err)

	root := spanByName(sr.Ended(), constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	code, ok := statusCodeAttr(t, root)
	require.True(t, ok, "root span must carry the terminal status stamped by the LIFO defer")
	assert.Equal(t, int64(401), code)
}

func TestProcessSpanStatus_NoExecutionContextPassThrough(t *testing.T) {
	server, _, sr := newSpanStatusServer(t)

	// Request body arrives with no prior request-headers phase: execCtx is nil.
	stream := newMockStream([]*extprocv3.ProcessingRequest{
		{
			Request: &extprocv3.ProcessingRequest_RequestBody{
				RequestBody: &extprocv3.HttpBody{Body: []byte("x"), EndOfStream: true},
			},
		},
	})

	err := server.Process(stream)
	require.NoError(t, err)

	spans := sr.Ended()
	root := spanByName(spans, constants.SpanExternalProcessingProcess)
	phase := spanByName(spans, constants.SpanProcessRequestBody)
	require.NotNil(t, root)
	require.NotNil(t, phase)

	assert.Equal(t, codes.Unset, root.Status().Code)
	_, ok := statusCodeAttr(t, root)
	assert.False(t, ok, "no terminal status is knowable when execCtx is nil")
	_, ok = statusCodeAttr(t, phase)
	assert.False(t, ok)

	errAttr, ok := attrOf(phase, attribute.Key(constants.AttrError))
	require.True(t, ok)
	assert.Equal(t, constants.AttrErrorReasonNoContext, errAttr.AsString())
}

func TestProcessSpanStatus_StreamRecvError(t *testing.T) {
	server, _, sr := newSpanStatusServer(t)

	stream := newMockStream([]*extprocv3.ProcessingRequest{})
	stream.recvErr = errors.New("boom")

	err := server.Process(stream)
	require.Error(t, err)

	root := spanByName(sr.Ended(), constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	assert.Equal(t, codes.Error, root.Status().Code)
	_, ok := statusCodeAttr(t, root)
	assert.False(t, ok, "a stream failure has no HTTP status to record")
}

func TestProcessSpanStatus_StreamContextCanceled(t *testing.T) {
	server, _, sr := newSpanStatusServer(t)

	stream := newMockStream([]*extprocv3.ProcessingRequest{})
	stream.recvErr = context.Canceled

	err := server.Process(stream)
	require.NoError(t, err)

	root := spanByName(sr.Ended(), constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	assert.Equal(t, codes.Unset, root.Status().Code)
	assert.Empty(t, root.Events())
}

// TestProcessSpanStatus_TracingDisabled guards the IsRecording() fast paths:
// with tracing disabled (the default global no-op provider, left untouched by
// this test), RecordHTTPOutcome must not panic and nothing gets recorded.
func TestProcessSpanStatus_TracingDisabled(t *testing.T) {
	k := NewKernel()
	chain := buildChainWithPolicy(t, "deny-401-noop-policy", &denyHeaderPolicy{statusCode: 401}, map[string]interface{}{})
	registerTestRoute(k, "test-route", chain)

	// The chain executor still needs a functioning (if no-op) tracer — nil would
	// panic on c.tracer.Start. What this test actually exercises is the
	// IsRecording()==false fast path in RecordHTTPOutcome, driven by the
	// server's own tracer coming from the untouched global (no-op-by-default)
	// TracerProvider rather than a real SDK one.
	chainExecutor := executor.NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer("test"))
	server := NewExternalProcessorServer(k, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes)

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("test-route", "GET", "/pets"),
	})

	assert.NotPanics(t, func() {
		err := server.Process(stream)
		require.NoError(t, err)
	})
	require.Len(t, stream.responses, 1)
	imm := stream.responses[0].GetImmediateResponse()
	require.NotNil(t, imm)
	assert.EqualValues(t, 401, imm.GetStatus().GetCode())
}

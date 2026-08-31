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
	"encoding/json"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"

	"github.com/wso2/api-platform/common/agentproto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/tracing"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// The incoming trace this test hands the gateway. Fixed rather than generated so a
// failure names the value it was looking for.
const (
	incomingTraceID     = "4bf92f3577b34da6a3ce929d0e0e4736"
	incomingSpanID      = "00f067aa0ba902b7"
	incomingTraceparent = "00-" + incomingTraceID + "-" + incomingSpanID + "-01"
)

// A2A trace context, end to end: an incoming W3C traceparent parents the spans this
// engine records for an A2A request, and those spans carry the resolved operation and
// the request's A2A identifiers.
//
// Both halves matter and neither implies the other. Without the propagation, an A2A
// request's gateway spans start a new trace and cannot be correlated with the caller's
// or with the agent's. Without the attributes, the spans are in the right trace but
// cannot be searched by operation, conversation or task — which for a multiplexed
// JSON-RPC endpoint means they cannot be told apart at all, since every operation
// shares one route and one path.
func TestA2A_TraceContextAndResolutionAttributesEndToEnd(t *testing.T) {
	installW3CPropagator(t)
	server, k, sr := newSpanStatusServer(t)

	const routeKey = "POST|/weather/1.0.0|example.com"
	registerA2AJSONRPCRoute(t, k, routeKey, "SendMessage")

	// A caller-supplied traceparent, arriving the way Envoy forwards it: on the
	// ext_proc gRPC stream's metadata.
	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersWithBodyToFollow(routeKey, "POST", "/weather/1.0.0"),
		requestBodyReq(`{"jsonrpc":"2.0","id":1,"method":"SendMessage",` +
			`"params":{"message":{"messageId":"msg-1","contextId":"ctx-1","taskId":"task-1"}}}`),
	})
	stream.ctx = metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("traceparent", incomingTraceparent))

	require.NoError(t, server.Process(stream))

	spans := sr.Ended()
	require.NotEmpty(t, spans, "the request must have produced spans")

	// Half one: the caller's trace, not a new one.
	for _, span := range spans {
		assert.Equal(t, incomingTraceID, span.SpanContext().TraceID().String(),
			"span %q must belong to the caller's trace", span.Name())
	}
	root := spanByName(spans, constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	assert.Equal(t, incomingSpanID, root.Parent().SpanID().String(),
		"the root ext_proc span must be a child of the caller's span")

	// Half two: the resolution, on the span that recorded it. On a body-resolved route
	// the chain is only known at the request-body callback, so this is also the
	// assertion that the attributes survive the deferred path.
	bodySpan := spanByName(spans, constants.SpanProcessRequestBody)
	require.NotNil(t, bodySpan, "a JSON-RPC route resolves at the request-body callback")

	assertSpanAttrs(t, bodySpan, map[string]string{
		constants.AttrResolverName:      agentproto.ResolverName,
		constants.AttrResolvedOperation: "SendMessage",
		"a2a.operation":                 "SendMessage",
		"a2a.transport":                 string(agentproto.TransportJSONRPC),
		"a2a.protocol.version":          "1.0",
		"a2a.message.id":                "msg-1",
		"a2a.context.id":                "ctx-1",
		"a2a.task.id":                   "task-1",
	})
}

// A request with no incoming traceparent still records the A2A attributes; it simply
// starts its own trace. Asserted separately so a regression in propagation cannot hide
// behind the attributes, or the reverse.
func TestA2A_ResolutionAttributesRecordedWithoutAnIncomingTrace(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)

	const routeKey = "POST|/weather/1.0.0|example.com"
	registerA2AJSONRPCRoute(t, k, routeKey, "GetTask")

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersWithBodyToFollow(routeKey, "POST", "/weather/1.0.0"),
		requestBodyReq(`{"jsonrpc":"2.0","id":1,"method":"GetTask"}`),
	})

	require.NoError(t, server.Process(stream))

	bodySpan := spanByName(sr.Ended(), constants.SpanProcessRequestBody)
	require.NotNil(t, bodySpan)
	assertSpanAttrs(t, bodySpan, map[string]string{
		constants.AttrResolvedOperation: "GetTask",
		"a2a.operation":                 "GetTask",
		"a2a.transport":                 string(agentproto.TransportJSONRPC),
	})

	// An operation whose payload carries no message contributes no identifiers, rather
	// than empty ones.
	for _, name := range []string{"a2a.message.id", "a2a.context.id", "a2a.task.id"} {
		_, present := attrOf(bodySpan, attribute.Key(name))
		assert.False(t, present, "%s must be absent when the request did not carry it", name)
	}
}

// ExtractTraceContext is what turns the stream's metadata into a parent context. This
// asserts the piece the end-to-end test depends on, in isolation, so a failure there
// is attributable.
func TestExtractTraceContext_ParentsFromTheStreamMetadata(t *testing.T) {
	installW3CPropagator(t)
	ctx := tracing.ExtractTraceContext(metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("traceparent", incomingTraceparent)))

	sc := trace.SpanContextFromContext(ctx)
	require.True(t, sc.IsValid(), "the incoming traceparent must produce a valid span context")
	assert.Equal(t, incomingTraceID, sc.TraceID().String())
	assert.Equal(t, incomingSpanID, sc.SpanID().String())
	assert.True(t, sc.IsSampled(), "the caller's sampling decision must be honoured")
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// installW3CPropagator installs the global text-map propagator that InitTracer
// installs at startup. It is global process state rather than something the server
// holds, so a test that does not call InitTracer gets the SDK's no-op propagator and
// every incoming traceparent is silently ignored — which is exactly the failure these
// tests exist to catch, so the production propagator has to be in place for them to be
// testing anything.
func installW3CPropagator(t *testing.T) {
	t.Helper()
	previous := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() { otel.SetTextMapPropagator(previous) })
}

// registerA2AJSONRPCRoute wires a real a2a JSON-RPC route — the production resolver
// from the default registry, prepared from a real resolver_config — plus the chain the
// named operation binds to. Nothing about the resolution is faked, which is what makes
// the assertions above end-to-end rather than a restatement of the test's own setup.
func registerA2AJSONRPCRoute(t *testing.T, k *Kernel, routeKey, operation string) {
	t.Helper()

	cfg, err := json.Marshal(map[string]string{
		"transport":       string(agentproto.TransportJSONRPC),
		"protocolVersion": "1.0",
	})
	require.NoError(t, err)

	rc := &RouteConfig{
		Metadata: RouteMetadata{
			RouteName: routeKey,
			APIId:     "agent-1",
			APIName:   "WeatherAgent",
			APIKind:   "Agent",
			Vhost:     "example.com",
		},
		RouteResolution: resolver.RouteResolution{
			ResolverName:   agentproto.ResolverName,
			ResolverConfig: cfg,
		},
	}
	require.NoError(t, PrepareRoute(resolver.DefaultRegistry(), routeKey, rc))

	// The chain the resolved operation will look for: the same composition the
	// resolver performs, so a mismatch fails as a missing chain rather than passing.
	k.RegisterRoute(resolver.ChainKeyFor("agent-1", "example.com", operation),
		buildChainFor([]policy.Policy{&testutils.NoopPolicy{}}))
	k.ApplyWholeRouteConfigs(map[string]*RouteConfig{routeKey: rc})
}

// requestHeadersWithBodyToFollow is requestHeadersReq with EndOfStream cleared: a
// JSON-RPC route defers resolution to the request-body callback only when a body is
// actually coming, since Envoy sends no body callback for a bodyless request.
//
// It also carries the A2A protocol version, the way a conformant A2A client does. An
// operation route validates that before it will resolve anything at all (Section
// 8A), so a fixture that omitted it would be testing the rejection path rather than
// the one it means to.
func requestHeadersWithBodyToFollow(routeName, method, path string) *extprocv3.ProcessingRequest {
	req := requestHeadersReq(routeName, method, path)
	req.GetRequestHeaders().EndOfStream = false
	withA2AVersion(req, a2aTestProtocolVersion)
	return req
}

// a2aTestProtocolVersion is the version every A2A route in these tests exposes.
const a2aTestProtocolVersion = "1.0"

// withA2AVersion appends the A2A-Version request header, in the lowercase form Envoy
// delivers header names in.
func withA2AVersion(req *extprocv3.ProcessingRequest, version string) *extprocv3.ProcessingRequest {
	headers := req.GetRequestHeaders().GetHeaders()
	headers.Headers = append(headers.Headers, &corev3.HeaderValue{
		Key:      resolver.A2AVersionHeader,
		RawValue: []byte(version),
	})
	return req
}

func requestBodyReq(body string) *extprocv3.ProcessingRequest {
	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestBody{
			RequestBody: &extprocv3.HttpBody{Body: []byte(body), EndOfStream: true},
		},
	}
}

func assertSpanAttrs(t *testing.T, span sdktrace.ReadOnlySpan, want map[string]string) {
	t.Helper()
	for key, expected := range want {
		got, ok := attrOf(span, attribute.Key(key))
		if !assert.True(t, ok, "span %q is missing attribute %s", span.Name(), key) {
			continue
		}
		assert.Equal(t, expected, got.AsString(), "attribute %s", key)
	}
}

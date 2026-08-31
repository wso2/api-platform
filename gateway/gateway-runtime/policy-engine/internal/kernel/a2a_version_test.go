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

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"

	"github.com/wso2/api-platform/common/agentproto"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// The kernel half of the A2A request protocol-version guard: where the prepared
// resolver's header-validation phase sits in the request's life, and what a refusal
// produces.
//
// The resolver package owns the rules themselves (a2a_version_test.go there). What
// is asserted here is placement and consequence — that the hook runs before both
// static binding and body deferral, that a refusal binds no chain, buffers nothing
// and reaches no upstream, and that the rejection is visible in telemetry without
// pretending an operation or a consumer existed.

// ─── Placement ───────────────────────────────────────────────────────────────

// The ordering the section exists for, asserted on every prepared shape an Agent
// route can take.
//
// Both directions of misplacing the hook are real. After the static branch it would
// skip the nine path-known HTTP+JSON operations entirely — every one of which would
// still pass a test that only exercised JSON-RPC. After the body-requirement branch
// it would buffer a JSON-RPC or message-sending body, unauthenticated, before
// rejecting a header it already knew was wrong.
func TestA2AVersionGuard_RunsBeforeStaticBindingAndBeforeBodyDeferral(t *testing.T) {
	cases := map[string]struct {
		register func(t *testing.T, k *Kernel, routeKey string)
		method   string
		path     string
		// endOfStream mirrors what Envoy sends: a bodyless GET is end-of-stream at
		// the header callback, a POST with a body is not.
		endOfStream bool
		wantStatic  bool
		wantBuffers bool
	}{
		"json-rpc, body-deferred": {
			register:    func(t *testing.T, k *Kernel, rk string) { registerA2AJSONRPCRoute(t, k, rk, "SendMessage") },
			method:      "POST",
			path:        "/weather/rpc",
			endOfStream: false,
			wantBuffers: true,
		},
		"http+json, static": {
			register: func(t *testing.T, k *Kernel, rk string) {
				registerA2AHTTPJSONRoute(t, k, rk, agentproto.GetTask)
			},
			method:      "GET",
			path:        "/weather/v1/tasks/t-1",
			endOfStream: true,
			wantStatic:  true,
		},
		"http+json, body-reading": {
			register: func(t *testing.T, k *Kernel, rk string) {
				registerA2AHTTPJSONRoute(t, k, rk, agentproto.SendMessage)
			},
			method:      "POST",
			path:        "/weather/v1/message:send",
			endOfStream: false,
			wantBuffers: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server, k, _ := newSpanStatusServer(t)
			routeKey := tc.method + "|" + tc.path + "|example.com"
			tc.register(t, k, routeKey)

			rc := k.GetRouteConfig(routeKey)
			require.NotNil(t, rc)
			// The two properties the placement argument rests on, asserted rather
			// than assumed: if either changed, the ordering claim below would be
			// about a different code path than the one it names.
			assert.Equal(t, tc.wantStatic, rc.Prepared.IsStatic())
			assert.Equal(t, tc.wantBuffers, rc.Prepared.Requirements.BuffersBody())
			require.True(t, rc.Prepared.ValidatesHeaders())

			// A wrong version is refused outright: nothing bound, nothing pending.
			var execCtx *PolicyExecutionContext
			_, outcome, denial := server.initializeExecutionContext(context.Background(),
				a2aHeadersRequest(routeKey, tc.method, tc.path, tc.endOfStream, "0.3"), &execCtx)

			require.Equal(t, bindFailed, outcome,
				"a refused version must not defer, bind, or fall through")
			require.NotNil(t, denial)
			assert.Equal(t, resolver.FailureVersionNotSupported, denial.failure.Kind)
			assert.Equal(t, constants.ResolutionPhaseHeaders, denial.phase)
			assert.Equal(t, agentproto.ResolverName, denial.resolverName)
			assert.Nil(t, execCtx, "no execution context may survive a refusal")

			// The right version reaches the path this route would normally take.
			execCtx = nil
			_, outcome, denial = server.initializeExecutionContext(context.Background(),
				a2aHeadersRequest(routeKey, tc.method, tc.path, tc.endOfStream, "1.0"), &execCtx)
			assert.Nil(t, denial)
			if tc.wantBuffers && !tc.endOfStream {
				assert.Equal(t, bindPending, outcome)
			} else {
				assert.Equal(t, bindReady, outcome)
			}
		})
	}
}

// A refusal on a body-deferred route must not ask Envoy for the body it was about to
// need. This is what makes the guard a saving rather than a formality: the rejection
// is an ImmediateResponse with no ModeOverride at all, so no buffering is requested
// and no body callback follows.
func TestA2AVersionGuard_RefusalAsksForNoBodyBuffering(t *testing.T) {
	server, k, _ := newSpanStatusServer(t)

	const routeKey = "POST|/weather/rpc|example.com"
	registerA2AJSONRPCRoute(t, k, routeKey, "SendMessage")

	// A well-formed SendMessage body follows the headers. It must never be read: the
	// stream ends at the rejection, so the second message is left unconsumed.
	stream := newMockStream([]*extprocv3.ProcessingRequest{
		a2aHeadersRequest(routeKey, "POST", "/weather/rpc", false, ""),
		requestBodyReq(`{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":{}}`),
	})
	require.NoError(t, server.Process(stream))

	require.Len(t, stream.responses, 2)
	imm := stream.responses[0].GetImmediateResponse()
	require.NotNil(t, imm, "an absent version means 0.3, which a 1.0 route refuses")
	assert.Equal(t, typev3.StatusCode_BadRequest, imm.Status.Code)
	assert.Nil(t, stream.responses[0].ModeOverride,
		"a rejected request must not be answered with a request to buffer its body")
}

// The sterile response: a 400 with the generic body and a correlation id, naming
// neither the resolver, the version the caller stated, nor the version this route
// serves. A caller learns that its request was refused and nothing it could probe
// with (error-handling.md directives 1 and 4).
func TestA2AVersionGuard_RefusalIsSterile(t *testing.T) {
	server, k, _ := newSpanStatusServer(t)

	const routeKey = "GET|/weather/v1/tasks|example.com"
	registerA2AHTTPJSONRoute(t, k, routeKey, agentproto.ListTasks)

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		a2aHeadersRequest(routeKey, "GET", "/weather/v1/tasks", true, "9.9"),
	})
	require.NoError(t, server.Process(stream))

	require.Len(t, stream.responses, 1)
	imm := stream.responses[0].GetImmediateResponse()
	require.NotNil(t, imm)
	assert.Equal(t, typev3.StatusCode_BadRequest, imm.Status.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(imm.Body, &body))
	assert.Equal(t, "Bad Request", body["error"])
	assert.NotEmpty(t, body["error_id"])
	for _, leak := range []string{"9.9", "1.0", agentproto.ResolverName, "version"} {
		assert.NotContains(t, string(imm.Body), leak,
			"the sterile body must not name %q", leak)
	}
}

// ─── The guard does not mask what follows it ─────────────────────────────────

// A version that passes must leave every later classification exactly as Section 8
// left it. The guard is a gate, not a filter: once through it, a malformed envelope
// is still a parse failure and an unknown method is still an unknown operation.
func TestA2AVersionGuard_AValidVersionPreservesEveryBodyClassification(t *testing.T) {
	cases := map[string]struct {
		body       string
		wantStatus typev3.StatusCode
	}{
		"a 0.3 method name": {`{"jsonrpc":"2.0","id":1,"method":"message/send"}`, typev3.StatusCode_NotFound},
		"an unknown method": {`{"jsonrpc":"2.0","id":1,"method":"NotAnOperation"}`, typev3.StatusCode_NotFound},
		"malformed JSON":    {`{"jsonrpc":`, typev3.StatusCode_BadRequest},
		"a batch":           {`[{"jsonrpc":"2.0","id":1,"method":"SendMessage"}]`, typev3.StatusCode_BadRequest},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			server, k, _ := newSpanStatusServer(t)
			const routeKey = "POST|/weather/rpc|example.com"
			registerA2AJSONRPCRoute(t, k, routeKey, "SendMessage")

			stream := newMockStream([]*extprocv3.ProcessingRequest{
				a2aHeadersRequest(routeKey, "POST", "/weather/rpc", false, "1.0"),
				requestBodyReq(tc.body),
			})
			require.NoError(t, server.Process(stream))

			// Two responses: the buffering instruction, then the body-phase refusal.
			require.Len(t, stream.responses, 2)
			assert.NotNil(t, stream.responses[0].ModeOverride,
				"a valid version must let the route ask for its body as it always did")
			imm := stream.responses[1].GetImmediateResponse()
			require.NotNil(t, imm)
			assert.Equal(t, tc.wantStatus, imm.Status.Code)
		})
	}
}

// ─── Telemetry ───────────────────────────────────────────────────────────────

// A rejection is visible without inventing anything. The span carries the resolver,
// the phase, the bounded reason and this route's own protocol facts; it carries no
// chain key and no operation, because neither happened.
func TestA2AVersionGuard_RejectionSpanRecordsBoundedFactsOnly(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)

	const routeKey = "POST|/weather/rpc|example.com"
	registerA2AJSONRPCRoute(t, k, routeKey, "SendMessage")

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		a2aHeadersRequest(routeKey, "POST", "/weather/rpc", false, "0.3"),
	})
	require.NoError(t, server.Process(stream))

	headerSpan := spanByName(sr.Ended(), constants.SpanProcessRequestHeaders)
	require.NotNil(t, headerSpan)
	assertSpanAttrs(t, headerSpan, map[string]string{
		constants.AttrResolverName:              agentproto.ResolverName,
		constants.AttrResolutionPhase:           constants.ResolutionPhaseHeaders,
		constants.AttrResolutionFailureReason:   string(resolver.FailureVersionNotSupported),
		string(resolver.AttrA2ATransport):       string(agentproto.TransportJSONRPC),
		string(resolver.AttrA2AProtocolVersion): "1.0",
		constants.AttrTerminalReason:            constants.TerminalReasonA2AVersionRejected,
	})

	for _, absent := range []string{constants.AttrPolicyChainKey, constants.AttrResolvedOperation} {
		_, present := attrOf(headerSpan, attribute.Key(absent))
		assert.False(t, present, "%s must be absent: no chain was ever bound", absent)
	}
	// The value the caller supplied is unbounded and attacker-chosen. It reaches the
	// internal log and must reach nothing that a trace backend indexes.
	for _, attr := range headerSpan.Attributes() {
		assert.NotEqual(t, "0.3", attr.Value.AsString(),
			"attribute %s must not carry the caller's stated version", attr.Key)
	}
}

// A refused request still produces an analytics event, and the event tells the truth
// about what did not happen: it is an attempted invocation (requestType "operation"),
// its operation is unknown because no chain ran, and its terminal reason distinguishes
// a protocol-version refusal from a payload the gateway could not resolve.
//
// The two bounded protocol facts survive even though the analytics system policy —
// which normally emits them — never ran: it lives in the chain this request failed to
// bind.
func TestA2AVersionGuard_RejectionAnalyticsAreTruthful(t *testing.T) {
	server, k, _ := newSpanStatusServer(t)

	const routeKey = "GET|/weather/v1/tasks|example.com"
	registerA2AHTTPJSONRoute(t, k, routeKey, agentproto.ListTasks)

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		a2aHeadersRequest(routeKey, "GET", "/weather/v1/tasks", true, ""),
	})
	require.NoError(t, server.Process(stream))

	require.Len(t, stream.responses, 1)
	fields := analyticsFields(t, stream.responses[0])

	assert.Equal(t, constants.TerminalReasonA2AVersionRejected, fields[TerminalReasonKey])
	assert.Equal(t, string(agentproto.TransportHTTPJSON), fields[resolver.AttrA2ATransport])
	assert.Equal(t, "1.0", fields[resolver.AttrA2AProtocolVersion])
	assert.Equal(t, "Agent", fields[APIKindKey])

	// Absent, not empty: no chain ran, so there is no operation to name — even
	// though this HTTP+JSON route made the intended one perfectly obvious.
	assert.NotContains(t, fields, ResolvedOperationKey)
	// And nothing caller-supplied travels on an event that leaves the process.
	for key, value := range fields {
		assert.NotEqual(t, "0.3", value, "field %s must not carry the implicit stated version", key)
	}
}

// ─── Routes without the phase ────────────────────────────────────────────────

// Every kind shipping today resolves by route key and must not acquire a header
// phase it never asked for — nor pay for building the view one would need.
func TestA2AVersionGuard_NonA2ARoutesAreUntouched(t *testing.T) {
	f := newResolutionFixture(t)
	const routeKey = "GET|/pets|example.com"
	rc := f.route(routeKey, resolver.RouteResolution{ResolverName: resolver.RouteKeyResolverName})
	f.chain(routeKey, &testutils.NoopPolicy{})

	require.False(t, rc.Prepared.ValidatesHeaders())

	// No version header anywhere, and the request binds exactly as it always has.
	var execCtx *PolicyExecutionContext
	_, outcome, denial := f.server.initializeExecutionContext(context.Background(),
		headersRequest(routeKey, true, map[string]string{":method": "GET", ":path": "/pets"}), &execCtx)

	assert.Equal(t, bindReady, outcome)
	assert.Nil(t, denial)
	require.NotNil(t, execCtx)
	assert.Equal(t, routeKey, execCtx.chainKey)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// registerA2AHTTPJSONRoute wires a real HTTP+JSON operation route — the production
// resolver, prepared from a real resolver_config — plus the chain its operation binds
// to. Nine of the eleven 1.0 operations prepare static this way; SendMessage and
// SendStreamingMessage prepare body-reading, which is why the operation is a
// parameter rather than fixed.
func registerA2AHTTPJSONRoute(t *testing.T, k *Kernel, routeKey string, operation agentproto.Operation) {
	t.Helper()

	cfg, err := json.Marshal(agentproto.ResolverConfig{
		ProtocolVersion: agentproto.V1_0,
		Transport:       agentproto.TransportHTTPJSON,
		Operation:       operation,
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

	k.RegisterRoute(resolver.ChainKeyFor("agent-1", "example.com", string(operation)),
		buildChainFor([]policy.Policy{&testutils.NoopPolicy{}}))
	k.ApplyWholeRouteConfigs(map[string]*RouteConfig{routeKey: rc})
}

// a2aHeadersRequest builds a request-headers callback for an Agent route. An empty
// version omits the header entirely, which is the case §3.6.2 reads as 0.3.
func a2aHeadersRequest(routeKey, method, path string, endOfStream bool, version string) *extprocv3.ProcessingRequest {
	req := requestHeadersReq(routeKey, method, path)
	req.GetRequestHeaders().EndOfStream = endOfStream
	if version != "" {
		withA2AVersion(req, version)
	}
	return req
}

// analyticsFields pulls the analytics metadata out of an ext_proc response, as the
// access-log handler reads it off Envoy's dynamic metadata.
func analyticsFields(t *testing.T, resp *extprocv3.ProcessingResponse) map[string]string {
	t.Helper()
	require.NotNil(t, resp.DynamicMetadata, "a rejection must still carry analytics metadata")

	namespace, ok := resp.DynamicMetadata.Fields[constants.ExtProcFilterName]
	require.True(t, ok, "analytics metadata travels in the ext_proc filter's namespace")
	value, ok := namespace.GetStructValue().GetFields()["analytics_data"]
	require.True(t, ok, "analytics metadata must be under analytics_data")

	fields := make(map[string]string)
	for key, v := range value.GetStructValue().GetFields() {
		fields[key] = v.GetStringValue()
	}
	return fields
}

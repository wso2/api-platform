package analytics

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"testing"
	"time"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

func agentSharedContext(operation string, attrs map[string]string) *policy.SharedContext {
	return &policy.SharedContext{
		APIKind:              policy.APIKindAgent,
		ResolvedOperation:    operation,
		ResolutionAttributes: policy.NewResolutionAttributes(attrs),
		Metadata:             map[string]interface{}{},
	}
}

func decodeProps(t *testing.T, metadata map[string]any, key string) map[string]interface{} {
	t.Helper()
	raw, ok := metadata[key].(string)
	if !ok {
		t.Fatalf("expected %s to be a JSON string, got %T", key, metadata[key])
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("failed to decode %s: %v", key, err)
	}
	return decoded
}

// ─── Request side ────────────────────────────────────────────────────────────

// The request dimensions are copied out of the resolver's output, not re-parsed from
// the body. That is the whole point of SharedContext.ResolutionAttributes: MCP's
// equivalent unmarshals the request body here and again elsewhere for the same
// request.
func TestOnRequestHeaders_AgentEmitsResolverExtractedDimensions(t *testing.T) {
	reqCtx := &policy.RequestHeaderContext{
		SharedContext: agentSharedContext("SendMessage", map[string]string{
			"a2a.operation":        "SendMessage",
			"a2a.transport":        "JSONRPC",
			"a2a.protocol.version": "1.0",
			"a2a.message.id":       "msg-1",
			"a2a.context.id":       "ctx-1",
			"a2a.task.id":          "task-1",
		}),
		Headers: policy.NewHeaders(map[string][]string{}),
	}

	action := (&AnalyticsPolicy{}).OnRequestHeaders(context.Background(), reqCtx, nil)
	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestHeaderModifications, got %T", action)
	}

	props := decodeProps(t, mods.AnalyticsMetadata, A2ARequestPropertiesKey)
	for key, want := range map[string]string{
		"transport":       "JSONRPC",
		"protocolVersion": "1.0",
		"messageId":       "msg-1",
		"contextId":       "ctx-1",
		"taskId":          "task-1",
	} {
		if props[key] != want {
			t.Errorf("%s = %v, want %v", key, props[key], want)
		}
	}

	// The canonical operation is deliberately NOT repeated here: the kernel stamps it
	// directly, so it survives the analytics policy not being in the chain at all.
	if _, present := props["operation"]; present {
		t.Error("operation must not be duplicated into the policy's property block")
	}
}

// Both transports resolve to the same operation, and each reports its own transport.
// The convergence itself is the resolver's guarantee; what is asserted here is that
// this policy does not collapse or cross the two dimensions on the way out.
func TestOnRequestHeaders_AgentReportsTransportSeparatelyFromOperation(t *testing.T) {
	for _, transport := range []string{"JSONRPC", "HTTP+JSON"} {
		t.Run(transport, func(t *testing.T) {
			reqCtx := &policy.RequestHeaderContext{
				SharedContext: agentSharedContext("SendMessage", map[string]string{
					"a2a.transport":        transport,
					"a2a.protocol.version": "1.0",
				}),
				Headers: policy.NewHeaders(map[string][]string{}),
			}

			action := (&AnalyticsPolicy{}).OnRequestHeaders(context.Background(), reqCtx, nil)
			mods := action.(policy.UpstreamRequestHeaderModifications)
			props := decodeProps(t, mods.AnalyticsMetadata, A2ARequestPropertiesKey)

			if props["transport"] != transport {
				t.Errorf("transport = %v, want %v", props["transport"], transport)
			}
		})
	}
}

// An operation whose payload carries no message at all still gets the protocol facts,
// and gets no empty identifiers: absent, not "".
func TestOnRequestHeaders_AgentOmitsIdentifiersTheRequestDidNotCarry(t *testing.T) {
	reqCtx := &policy.RequestHeaderContext{
		SharedContext: agentSharedContext("ListTasks", map[string]string{
			"a2a.transport":        "HTTP+JSON",
			"a2a.protocol.version": "1.0",
		}),
		Headers: policy.NewHeaders(map[string][]string{}),
	}

	action := (&AnalyticsPolicy{}).OnRequestHeaders(context.Background(), reqCtx, nil)
	mods := action.(policy.UpstreamRequestHeaderModifications)
	props := decodeProps(t, mods.AnalyticsMetadata, A2ARequestPropertiesKey)

	for _, key := range []string{"messageId", "contextId", "taskId"} {
		if _, present := props[key]; present {
			t.Errorf("%s must be absent, not empty, when the request did not carry it", key)
		}
	}
}

// A card fetch or preflight rides the Agent's own routes but is not an invocation of
// the agent. Emitting invocation-shaped dimensions for it would let a downstream
// rollup count a client's card polling as agent traffic.
func TestOnRequestHeaders_AgentWithNoResolvedOperationEmitsNothing(t *testing.T) {
	reqCtx := &policy.RequestHeaderContext{
		SharedContext: agentSharedContext("", nil),
		Headers:       policy.NewHeaders(map[string][]string{}),
	}

	action := (&AnalyticsPolicy{}).OnRequestHeaders(context.Background(), reqCtx, nil)
	mods := action.(policy.UpstreamRequestHeaderModifications)

	if _, present := mods.AnalyticsMetadata[A2ARequestPropertiesKey]; present {
		t.Error("a request with no resolved operation must not be reported as an invocation")
	}
}

// Every other API kind must be untouched by this section.
func TestOnRequestHeaders_NonAgentKindsEmitNoA2ADimensions(t *testing.T) {
	for _, kind := range []policy.APIKind{
		policy.APIKindRestApi, policy.APIKindMCP, policy.APIKindLlmProxy,
	} {
		t.Run(string(kind), func(t *testing.T) {
			reqCtx := &policy.RequestHeaderContext{
				SharedContext: &policy.SharedContext{
					APIKind: kind,
					// Even if something had stamped these, a non-Agent kind reports nothing.
					ResolvedOperation: "SendMessage",
					ResolutionAttributes: policy.NewResolutionAttributes(
						map[string]string{"a2a.transport": "JSONRPC"}),
				},
				Headers: policy.NewHeaders(map[string][]string{}),
			}

			action := (&AnalyticsPolicy{}).OnRequestHeaders(context.Background(), reqCtx, nil)
			if mods, ok := action.(policy.UpstreamRequestHeaderModifications); ok {
				if _, present := mods.AnalyticsMetadata[A2ARequestPropertiesKey]; present {
					t.Errorf("%s must emit no A2A dimensions", kind)
				}
			}
		})
	}
}

// ─── Response side: outcome from the A2A result, not the HTTP status ────────

func TestOnResponseBody_AgentJSONRPCErrorInsideA200(t *testing.T) {
	ctx := &policy.ResponseContext{
		SharedContext:   agentSharedContext("SendMessage", nil),
		ResponseStatus:  200,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
		ResponseBody: &policy.Body{
			Content: []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`),
		},
	}

	action := (&AnalyticsPolicy{}).OnResponseBody(context.Background(), ctx, nil)
	mods, ok := action.(policy.DownstreamResponseModifications)
	if !ok {
		t.Fatalf("expected DownstreamResponseModifications, got %T", action)
	}

	props := decodeProps(t, mods.AnalyticsMetadata, A2AResponsePropertiesKey)
	if props["isError"] != true {
		t.Errorf("isError = %v, want true — a JSON-RPC error rides a 200", props["isError"])
	}
	if props["errorCode"] != float64(-32601) {
		t.Errorf("errorCode = %v, want -32601", props["errorCode"])
	}
	// The agent-authored message is unbounded free text of unknown sensitivity.
	if _, present := props["errorMessage"]; present {
		t.Error("the error message must not be carried into analytics")
	}
}

func TestOnResponseBody_AgentSuccessfulResultIsNotAnError(t *testing.T) {
	ctx := &policy.ResponseContext{
		SharedContext:   agentSharedContext("GetTask", nil),
		ResponseStatus:  200,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
		ResponseBody: &policy.Body{
			Content: []byte(`{"jsonrpc":"2.0","id":1,"result":{"id":"task-1","status":{"state":"completed"}}}`),
		},
	}

	action := (&AnalyticsPolicy{}).OnResponseBody(context.Background(), ctx, nil)
	mods := action.(policy.DownstreamResponseModifications)
	props := decodeProps(t, mods.AnalyticsMetadata, A2AResponsePropertiesKey)

	if props["isError"] != false {
		t.Errorf("isError = %v, want false", props["isError"])
	}
	if _, present := props["errorCode"]; present {
		t.Error("a successful result must carry no error code")
	}
	if props["isStreaming"] != false {
		t.Errorf("isStreaming = %v, want false on the buffered path", props["isStreaming"])
	}
}

// A body this policy could not read leaves isError unset rather than false. Claiming
// success for a response nobody parsed is exactly the silently-wrong dashboard the
// section warns against.
func TestOnResponseBody_AgentUnreadableBodyLeavesOutcomeUndetermined(t *testing.T) {
	for name, body := range map[string][]byte{
		"empty":     nil,
		"not JSON":  []byte("upstream is on fire"),
		"truncated": []byte(`{"jsonrpc":"2.0","res`),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := &policy.ResponseContext{
				SharedContext:   agentSharedContext("SendMessage", nil),
				ResponseStatus:  200,
				ResponseHeaders: policy.NewHeaders(map[string][]string{}),
				ResponseBody:    &policy.Body{Content: body},
			}

			action := (&AnalyticsPolicy{}).OnResponseBody(context.Background(), ctx, nil)
			mods, ok := action.(policy.DownstreamResponseModifications)
			if !ok {
				t.Fatalf("expected DownstreamResponseModifications, got %T", action)
			}
			props := decodeProps(t, mods.AnalyticsMetadata, A2AResponsePropertiesKey)
			if _, present := props["isError"]; present {
				t.Errorf("isError must be absent for an unparseable body, got %v", props["isError"])
			}
		})
	}
}

func TestOnResponseBody_AgentWithNoResolvedOperationEmitsNothing(t *testing.T) {
	ctx := &policy.ResponseContext{
		SharedContext:   agentSharedContext("", nil),
		ResponseStatus:  200,
		ResponseHeaders: policy.NewHeaders(map[string][]string{}),
		ResponseBody:    &policy.Body{Content: []byte(`{"name":"WeatherAgent"}`)},
	}

	action := (&AnalyticsPolicy{}).OnResponseBody(context.Background(), ctx, nil)
	if mods, ok := action.(policy.DownstreamResponseModifications); ok {
		if _, present := mods.AnalyticsMetadata[A2AResponsePropertiesKey]; present {
			t.Error("a card fetch must not be reported with an invocation outcome")
		}
	}
}

// ─── Response side: SSE ─────────────────────────────────────────────────────

// An A2A stream is a sequence of task updates that can end in an error after any
// number of successful events. Stopping at the first event carrying a result — which
// is what the MCP helper does — would report every late failure as a success.
func TestObserveA2AResponse_ErrorLateInAStreamWinsOverEarlierResults(t *testing.T) {
	sse := []byte("data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"state\":\"working\"}}\n\n" +
		"data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"state\":\"working\"}}\n\n" +
		"data: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32000,\"message\":\"agent gave up\"}}\n\n")
	headers := policy.NewHeaders(map[string][]string{"content-type": {"text/event-stream"}})

	payload := observeA2AResponse(sse, headers).outcomeEnvelope
	if payload == nil {
		t.Fatal("expected a payload from the SSE stream")
	}
	if _, hasError := payload["error"]; !hasError {
		t.Errorf("expected the error event to win, got %v", payload)
	}
}

func TestObserveA2AResponse_AllSuccessfulStreamUsesTheLastResult(t *testing.T) {
	sse := []byte("data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"state\":\"working\"}}\n\n" +
		"data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"state\":\"completed\"}}\n\n" +
		"data: [DONE]\n\n")
	headers := policy.NewHeaders(map[string][]string{"content-type": {"text/event-stream"}})

	payload := observeA2AResponse(sse, headers).outcomeEnvelope
	if payload == nil {
		t.Fatal("expected a payload from the SSE stream")
	}
	if _, hasError := payload["error"]; hasError {
		t.Error("an all-successful stream must not report an error")
	}
	result, _ := payload["result"].(map[string]interface{})
	if result["state"] != "completed" {
		t.Errorf("expected the terminal result event, got %v", payload)
	}
}

func TestOnResponseBodyChunk_AgentStreamReportsTimingsAndOutcome(t *testing.T) {
	shared := agentSharedContext("SendStreamingMessage", nil)
	// The request-header phase stamped the start; back-date it so the elapsed time is
	// unambiguously non-zero without the test having to sleep.
	shared.Metadata[a2aRequestStartKey] = time.Now().Add(-250 * time.Millisecond)

	ctx := &policy.ResponseStreamContext{
		SharedContext:   shared,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"text/event-stream"}}),
	}
	pol := &AnalyticsPolicy{}

	first := []byte("data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"state\":\"working\"}}\n\n")
	pol.OnResponseBodyChunk(context.Background(), ctx,
		&policy.StreamBody{Chunk: first}, nil)

	last := []byte("data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"state\":\"completed\"}}\n\n")
	action := pol.OnResponseBodyChunk(context.Background(), ctx,
		&policy.StreamBody{Chunk: last, EndOfStream: true}, nil)

	forward, ok := action.(policy.ForwardResponseChunk)
	if !ok {
		t.Fatalf("expected ForwardResponseChunk, got %T", action)
	}
	props := decodeProps(t, forward.AnalyticsMetadata, A2AResponsePropertiesKey)

	if props["isStreaming"] != true {
		t.Errorf("isStreaming = %v, want true", props["isStreaming"])
	}
	if props["isError"] != false {
		t.Errorf("isError = %v, want false", props["isError"])
	}
	ttfe, ok := props["timeToFirstEventMs"].(float64)
	if !ok {
		t.Fatalf("timeToFirstEventMs missing or not a number: %v", props["timeToFirstEventMs"])
	}
	if ttfe < 200 {
		t.Errorf("timeToFirstEventMs = %v, want at least the 250ms the start was back-dated by", ttfe)
	}
	if _, ok := props["streamDurationMs"].(float64); !ok {
		t.Fatalf("streamDurationMs missing or not a number: %v", props["streamDurationMs"])
	}

	// The timing marks must not survive the stream: the shared context outlives this
	// callback, and a stale start would misreport the next stream on it.
	for _, key := range []string{a2aRequestStartKey, a2aFirstEventKey} {
		if _, present := shared.Metadata[key]; present {
			t.Errorf("%s must be cleared once the stream has ended", key)
		}
	}
}

// A stream that ended without a single event gets neither timing. A zero would read
// as an instant response rather than as an empty one.
func TestOnResponseBodyChunk_AgentEmptyStreamReportsNoTimings(t *testing.T) {
	shared := agentSharedContext("SendStreamingMessage", nil)
	shared.Metadata[a2aRequestStartKey] = time.Now().Add(-100 * time.Millisecond)

	ctx := &policy.ResponseStreamContext{
		SharedContext:   shared,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"text/event-stream"}}),
	}

	action := (&AnalyticsPolicy{}).OnResponseBodyChunk(context.Background(), ctx,
		&policy.StreamBody{EndOfStream: true}, nil)

	forward := action.(policy.ForwardResponseChunk)
	props := decodeProps(t, forward.AnalyticsMetadata, A2AResponsePropertiesKey)

	for _, key := range []string{"timeToFirstEventMs", "streamDurationMs"} {
		if _, present := props[key]; present {
			t.Errorf("%s must be absent for a stream that delivered no events", key)
		}
	}
}

// The first-event mark is recorded only for Agent requests, so no other kind pays for
// a dimension it does not report.
func TestOnResponseBodyChunk_FirstEventMarkIsAgentOnly(t *testing.T) {
	shared := &policy.SharedContext{
		APIKind:  policy.APIKindMCP,
		Metadata: map[string]interface{}{},
	}
	ctx := &policy.ResponseStreamContext{
		SharedContext:   shared,
		ResponseHeaders: policy.NewHeaders(map[string][]string{}),
	}

	(&AnalyticsPolicy{}).OnResponseBodyChunk(context.Background(), ctx,
		&policy.StreamBody{Chunk: []byte("data: {}\n\n")}, nil)

	if _, present := shared.Metadata[a2aFirstEventKey]; present {
		t.Error("the A2A first-event mark must not be written for a non-Agent kind")
	}
}

// ─── Request body phase ─────────────────────────────────────────────────────

// Agent has an explicit case in the OnRequestBody switch. Without it every Agent
// request with a body would fall to the default and log an error per request.
//
// A request whose body carries none of the three summaries still re-emits the block:
// the write is keyed, so the phase either writes the whole set or leaves the header
// phase's alone. What it must never do is write a partial one.
func TestOnRequestBody_AgentIsAKnownKind(t *testing.T) {
	ctx := &policy.RequestContext{
		SharedContext: agentSharedContext("SendMessage", map[string]string{
			"a2a.transport": "JSONRPC",
		}),
		Headers: policy.NewHeaders(map[string][]string{}),
		Body:    &policy.Body{Content: []byte(`{"jsonrpc":"2.0","method":"SendMessage"}`)},
	}

	action := (&AnalyticsPolicy{}).OnRequestBody(context.Background(), ctx, nil)
	mods, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("expected UpstreamRequestModifications, got %T", action)
	}
	props := decodeProps(t, mods.AnalyticsMetadata, A2ARequestPropertiesKey)
	if props["transport"] != "JSONRPC" {
		t.Errorf("transport = %v, want JSONRPC — the body phase must re-emit the whole block",
			props["transport"])
	}
	for _, key := range []string{"inputPartCount", "returnImmediately", "historyLength"} {
		if _, present := props[key]; present {
			t.Errorf("%s must be absent for an envelope that carries no params", key)
		}
	}
}

// ─── Cross-module key spellings ─────────────────────────────────────────────

// These strings are the wire contract with the policy engine, which is a separate Go
// module and cannot share a constant with this one. A rename on one side alone is
// silent — the dimension simply stops appearing — so both sides pin the spelling.
// The matching assertion is TestA2AMetadataKeySpellingsArePinned in
// internal/analytics.
func TestA2AKeySpellingsMatchThePolicyEngine(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{A2ARequestPropertiesKey, "a2a_request_properties"},
		{A2AResponsePropertiesKey, "a2a_response_properties"},
		{string(policy.APIKindAgent), "Agent"},
		{a2aAttrMessageID, "a2a.message.id"},
		{a2aAttrContextID, "a2a.context.id"},
		{a2aAttrTaskID, "a2a.task.id"},
		{a2aAttrTransport, "a2a.transport"},
		{a2aAttrProtocolVersion, "a2a.protocol.version"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

// The field names inside the two blocks are just as much of a wire contract as the
// keys that carry them: the policy engine unmarshals these documents into typed DTOs
// (dto.A2ARequestAnalytics and dto.A2AResponseAnalytics), and a field renamed on one
// side alone unmarshals into nothing at all — the dimension disappears with no error
// anywhere. Pinning the whole key set rather than individual names also catches a
// field *added* here and never picked up on the other side.
//
// The matching assertion is TestAgentAnalyticsWireFieldNamesArePinned in
// internal/analytics, which decodes these same documents into the DTOs.
func TestA2ABlockFieldNamesArePinned(t *testing.T) {
	partCount, historyLength := 2, 5
	returnImmediately, isError, isStreaming := false, false, true
	errorCode := -32601
	var ttfe, duration int64 = 120, 850

	for name, tc := range map[string]struct {
		value any
		want  []string
	}{
		A2ARequestPropertiesKey: {
			value: A2ARequestAnalyticsProperties{
				Transport: "JSONRPC", ProtocolVersion: "1.0",
				MessageID: "m-1", ContextID: "ctx-1", TaskID: "task-1",
				InputPartCount: &partCount, ReturnImmediately: &returnImmediately,
				HistoryLength: &historyLength,
			},
			want: []string{
				"contextId", "historyLength", "inputPartCount", "messageId",
				"protocolVersion", "returnImmediately", "taskId", "transport",
			},
		},
		A2AResponsePropertiesKey: {
			value: A2AResponseAnalyticsProperties{
				IsError: &isError, ErrorCode: &errorCode,
				TimeToFirstEventMs: &ttfe, StreamDurationMs: &duration, IsStreaming: &isStreaming,
				PayloadType: "task", ResponseTaskID: "task-9", ResponseContextID: "ctx-9",
				TaskState: "TASK_STATE_COMPLETED",
			},
			// The published model is one flat object, so the two identifiers that
			// share a name with a request field carry a `response` prefix. The
			// other seven have no request-side counterpart and keep bare names.
			want: []string{
				"errorCode", "isError", "isStreaming", "payloadType",
				"responseContextId", "responseTaskId", "streamDurationMs",
				"taskState", "timeToFirstEventMs",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var decoded map[string]interface{}
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got := make([]string, 0, len(decoded))
			for key := range decoded {
				got = append(got, key)
			}
			sort.Strings(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("field names = %v, want %v", got, tc.want)
			}
		})
	}
}

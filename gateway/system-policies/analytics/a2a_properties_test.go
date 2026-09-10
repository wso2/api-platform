package analytics

import (
	"context"
	"testing"
	"time"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// The three request summaries and the four response properties are the part of the
// A2A event contract this policy produces itself rather than copying out of the
// resolver's output. They are tested on both bindings throughout, because the two put
// the same facts in different places — JSON-RPC nests the operation's arguments under
// params and wraps its result in an envelope, HTTP+JSON sends both documents bare —
// and a summary that only works on one binding is worse than none: the dimension would
// look present and be systematically missing for half the traffic.

// ─── Request summaries ───────────────────────────────────────────────────────

// requestProps runs one request through both phases the way the engine does — headers
// first, then body — and returns the block that survives.
func requestProps(t *testing.T, transport, path string, body []byte) map[string]interface{} {
	t.Helper()
	shared := agentSharedContext("SendMessage", map[string]string{
		a2aAttrTransport:       transport,
		a2aAttrProtocolVersion: "1.0",
	})
	pol := &AnalyticsPolicy{}

	headerAction := pol.OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
		SharedContext: shared,
		Headers:       policy.NewHeaders(map[string][]string{}),
		Path:          path,
	}, nil)
	props := decodeProps(t, headerAction.(policy.UpstreamRequestHeaderModifications).AnalyticsMetadata,
		A2ARequestPropertiesKey)

	if len(body) == 0 {
		return props
	}
	bodyAction := pol.OnRequestBody(context.Background(), &policy.RequestContext{
		SharedContext: shared,
		Headers:       policy.NewHeaders(map[string][]string{}),
		Path:          path,
		Body:          &policy.Body{Content: body, Present: true},
	}, nil)
	return decodeProps(t, bodyAction.(policy.UpstreamRequestModifications).AnalyticsMetadata,
		A2ARequestPropertiesKey)
}

// The same send request over both bindings must summarise identically. The bodies
// differ only by the params wrapper, which is the whole difference between the two
// transports at this level.
func TestRequestSummary_SendMessageOnBothBindings(t *testing.T) {
	for _, tc := range []struct {
		transport string
		body      string
	}{
		{
			transport: "JSONRPC",
			body: `{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":{` +
				`"message":{"messageId":"m-1","role":"ROLE_USER","parts":[{"text":"a"},{"text":"b"},{"text":"c"}]},` +
				`"configuration":{"returnImmediately":true,"historyLength":5}}}`,
		},
		{
			transport: "HTTP+JSON",
			body: `{"message":{"messageId":"m-1","role":"ROLE_USER","parts":[{"text":"a"},{"text":"b"},{"text":"c"}]},` +
				`"configuration":{"returnImmediately":true,"historyLength":5}}`,
		},
	} {
		t.Run(tc.transport, func(t *testing.T) {
			props := requestProps(t, tc.transport, "/agent/message:send", []byte(tc.body))

			if props["inputPartCount"] != float64(3) {
				t.Errorf("inputPartCount = %v, want 3", props["inputPartCount"])
			}
			if props["returnImmediately"] != true {
				t.Errorf("returnImmediately = %v, want true", props["returnImmediately"])
			}
			if props["historyLength"] != float64(5) {
				t.Errorf("historyLength = %v, want 5", props["historyLength"])
			}
		})
	}
}

// returnImmediately's absence is defined by the protocol as false, so a send request
// that omits configuration entirely still reports the value it will be treated as
// having. historyLength has no such default and stays absent.
func TestRequestSummary_ReturnImmediatelyDefaultsToFalseForASendRequest(t *testing.T) {
	props := requestProps(t, "JSONRPC", "/agent",
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":{`+
			`"message":{"messageId":"m-1","parts":[{"text":"a"}]}}}`))

	if props["returnImmediately"] != false {
		t.Errorf("returnImmediately = %v, want the protocol default false", props["returnImmediately"])
	}
	if _, present := props["historyLength"]; present {
		t.Error("historyLength has no protocol default and must stay absent")
	}
}

// An operation with no message in its request shape gets neither of the two send-only
// summaries. Emitting returnImmediately: false for a GetTask would state that the
// caller chose the default on a field its request does not have.
func TestRequestSummary_NonSendOperationsGetNoSendOnlySummaries(t *testing.T) {
	props := requestProps(t, "JSONRPC", "/agent",
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"GetTask","params":{"id":"task-1","historyLength":9}}`))

	if props["historyLength"] != float64(9) {
		t.Errorf("historyLength = %v, want 9", props["historyLength"])
	}
	for _, key := range []string{"inputPartCount", "returnImmediately"} {
		if _, present := props[key]; present {
			t.Errorf("%s must be absent for an operation whose request carries no message", key)
		}
	}
}

// GetTask and ListTasks are GETs on the HTTP+JSON binding, so their history length is
// in the query string and nowhere else — and the header phase is the only phase that
// could read it even if a body existed.
func TestRequestSummary_HistoryLengthFromTheHTTPJSONQueryString(t *testing.T) {
	for name, path := range map[string]string{
		"GetTask":                  "/agent/v1/tasks/task-1?historyLength=7",
		"ListTasks":                "/agent/v1/tasks?contextId=ctx-1&historyLength=7&pageSize=10",
		"undecodable neighbour":    "/agent/v1/tasks?filter=%zz&historyLength=7",
		"percent-encoded spelling": "/agent/v1/tasks?history%4Cength=7",
	} {
		t.Run(name, func(t *testing.T) {
			props := requestProps(t, "HTTP+JSON", path, nil)
			if props["historyLength"] != float64(7) {
				t.Errorf("historyLength = %v, want 7", props["historyLength"])
			}
		})
	}
}

// A JSON-RPC endpoint carries its arguments in the body; a query string on it is not
// the protocol's, so nothing is read from it.
func TestRequestSummary_QueryStringIsNotReadOnTheJSONRPCBinding(t *testing.T) {
	props := requestProps(t, "JSONRPC", "/agent?historyLength=7", nil)
	if _, present := props["historyLength"]; present {
		t.Errorf("historyLength = %v, want absent on the JSON-RPC binding", props["historyLength"])
	}
}

// Zero is a value, not an absence: a message may carry no parts, and a history length
// of zero is an explicit request for no history at all. Both must survive the
// omitempty on the wire, which is why the fields are pointers.
func TestRequestSummary_ZeroValuesAreReportedNotOmitted(t *testing.T) {
	props := requestProps(t, "HTTP+JSON", "/agent/v1/message:send",
		[]byte(`{"message":{"messageId":"m-1","parts":[]},"configuration":{"historyLength":0}}`))

	if count, present := props["inputPartCount"]; !present || count != float64(0) {
		t.Errorf("inputPartCount = %v (present %v), want 0", count, present)
	}
	if length, present := props["historyLength"]; !present || length != float64(0) {
		t.Errorf("historyLength = %v (present %v), want 0", length, present)
	}
}

// A value the protocol's own int32 field cannot hold is not a count to report. Neither
// is a string, or a body that will not parse at all.
func TestRequestSummary_MalformedValuesAreOmittedNotCoerced(t *testing.T) {
	for name, body := range map[string]string{
		"fractional":   `{"jsonrpc":"2.0","params":{"historyLength":1.5}}`,
		"out of range": `{"jsonrpc":"2.0","params":{"historyLength":9999999999}}`,
		"string":       `{"jsonrpc":"2.0","params":{"historyLength":"5"}}`,
		"params array": `{"jsonrpc":"2.0","params":[{"historyLength":5}]}`,
		"not JSON":     `nonsense`,
	} {
		t.Run(name, func(t *testing.T) {
			props := requestProps(t, "JSONRPC", "/agent", []byte(body))
			if value, present := props["historyLength"]; present {
				t.Errorf("historyLength = %v, want absent", value)
			}
		})
	}
}

// The two phases share one metadata key, so the body phase's write replaces the header
// phase's. It must therefore carry everything the header phase had: a partial second
// write would silently drop the resolver's dimensions from every request with a body.
func TestRequestSummary_BodyPhaseBlockIsASupersetOfTheHeaderPhaseBlock(t *testing.T) {
	shared := agentSharedContext("SendMessage", map[string]string{
		a2aAttrTransport:       "JSONRPC",
		a2aAttrProtocolVersion: "1.0",
		a2aAttrMessageID:       "m-1",
		a2aAttrContextID:       "ctx-1",
		a2aAttrTaskID:          "task-1",
	})
	pol := &AnalyticsPolicy{}

	pol.OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
		SharedContext: shared,
		Headers:       policy.NewHeaders(map[string][]string{}),
		Path:          "/agent",
	}, nil)

	action := pol.OnRequestBody(context.Background(), &policy.RequestContext{
		SharedContext: shared,
		Headers:       policy.NewHeaders(map[string][]string{}),
		Path:          "/agent",
		Body: &policy.Body{Content: []byte(
			`{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":{"message":{"parts":[{"text":"a"}]}}}`)},
	}, nil)

	props := decodeProps(t, action.(policy.UpstreamRequestModifications).AnalyticsMetadata,
		A2ARequestPropertiesKey)
	for key, want := range map[string]interface{}{
		"transport":       "JSONRPC",
		"protocolVersion": "1.0",
		"messageId":       "m-1",
		"contextId":       "ctx-1",
		"taskId":          "task-1",
		"inputPartCount":  float64(1),
	} {
		if props[key] != want {
			t.Errorf("%s = %v, want %v", key, props[key], want)
		}
	}
}

// Time-to-first-event measures the wait a caller experiences, so the clock starts at
// the earliest phase this policy runs. Re-stamping it at the body phase would silently
// exclude every request-header policy — authentication included — from the figure.
func TestRequestSummary_BodyPhaseDoesNotRestartTheStreamClock(t *testing.T) {
	shared := agentSharedContext("SendMessage", map[string]string{a2aAttrTransport: "JSONRPC"})
	pol := &AnalyticsPolicy{}

	pol.OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
		SharedContext: shared,
		Headers:       policy.NewHeaders(map[string][]string{}),
		Path:          "/agent",
	}, nil)
	started, ok := shared.Metadata[a2aRequestStartKey].(time.Time)
	if !ok {
		t.Fatal("the header phase must start the stream clock")
	}

	pol.OnRequestBody(context.Background(), &policy.RequestContext{
		SharedContext: shared,
		Headers:       policy.NewHeaders(map[string][]string{}),
		Path:          "/agent",
		Body:          &policy.Body{Content: []byte(`{"jsonrpc":"2.0","params":{"message":{"parts":[]}}}`)},
	}, nil)

	if restarted := shared.Metadata[a2aRequestStartKey].(time.Time); !restarted.Equal(started) {
		t.Errorf("stream clock restarted at the body phase: %v then %v", started, restarted)
	}
}

// ─── Response properties ─────────────────────────────────────────────────────

// responseProps runs one buffered response through the policy and returns its block.
func responseProps(t *testing.T, operation string, status int, contentType string, body []byte) map[string]interface{} {
	t.Helper()
	headers := map[string][]string{}
	if contentType != "" {
		headers["content-type"] = []string{contentType}
	}
	ctx := &policy.ResponseContext{
		SharedContext:   agentSharedContext(operation, nil),
		ResponseStatus:  status,
		ResponseHeaders: policy.NewHeaders(headers),
		ResponseBody:    &policy.Body{Content: body},
	}
	action := (&AnalyticsPolicy{}).OnResponseBody(context.Background(), ctx, nil)
	return decodeProps(t, action.(policy.DownstreamResponseModifications).AnalyticsMetadata,
		A2AResponsePropertiesKey)
}

// The union member a send response carries is the payload type, and the task inside it
// is where a new task id and context id first appear — the correlation the request side
// could not supply because the agent had not generated them yet.
func TestResponseProperties_TaskPayloadOnBothBindings(t *testing.T) {
	task := `{"id":"task-9","contextId":"ctx-9","status":{"state":"TASK_STATE_COMPLETED"}}`
	for _, tc := range []struct {
		transport string
		body      string
	}{
		{transport: "JSONRPC", body: `{"jsonrpc":"2.0","id":1,"result":{"task":` + task + `}}`},
		{transport: "HTTP+JSON", body: `{"task":` + task + `}`},
	} {
		t.Run(tc.transport, func(t *testing.T) {
			props := responseProps(t, "SendMessage", 200, "application/json", []byte(tc.body))
			assertProps(t, props, map[string]interface{}{
				"payloadType":       "task",
				"responseTaskId":    "task-9",
				"responseContextId": "ctx-9",
				"taskState":         "TASK_STATE_COMPLETED",
			})
		})
	}
}

// GetTask and CancelTask answer with the Task itself rather than the send union, so
// the bare document has to classify the same way the wrapped one does.
func TestResponseProperties_BareTaskDocument(t *testing.T) {
	props := responseProps(t, "GetTask", 200, "application/json",
		[]byte(`{"id":"task-4","contextId":"ctx-4","status":{"state":"TASK_STATE_WORKING"}}`))

	assertProps(t, props, map[string]interface{}{
		"payloadType":       "task",
		"responseTaskId":    "task-4",
		"responseContextId": "ctx-4",
		"taskState":         "TASK_STATE_WORKING",
	})
}

// A message reply references the task it belongs to but has no state of its own, so
// taskState stays absent rather than being invented.
func TestResponseProperties_MessagePayloadCarriesNoTaskState(t *testing.T) {
	props := responseProps(t, "SendMessage", 200, "application/json",
		[]byte(`{"jsonrpc":"2.0","id":1,"result":{"message":`+
			`{"messageId":"m-2","taskId":"task-2","contextId":"ctx-2","role":"ROLE_AGENT"}}}`))

	assertProps(t, props, map[string]interface{}{
		"payloadType":       "message",
		"responseTaskId":    "task-2",
		"responseContextId": "ctx-2",
	})
	if _, present := props["taskState"]; present {
		t.Errorf("taskState = %v, want absent for a message payload", props["taskState"])
	}
}

// Each operation whose result type is fixed classifies from that document. Without
// this, five of the eleven operations would report `unknown` on every successful call
// and the dimension would be useless for exactly the traffic it can describe best.
func TestResponseProperties_FixedResultTypesClassify(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"ListTasks":                       {`{"tasks":[{"id":"t1"},{"id":"t2"}],"totalSize":2}`, "task_list"},
		"ListTaskPushNotificationConfigs": {`{"configs":[{"id":"c1","taskId":"task-1","url":"https://x"}]}`, "push_notification_config_list"},
		"GetTaskPushNotificationConfig":   {`{"id":"c1","taskId":"task-1","url":"https://example.test/hook"}`, "push_notification_config"},
		"GetExtendedAgentCard":            {`{"name":"Agent","protocolVersion":"1.0","skills":[]}`, "agent_card"},
	} {
		t.Run(name, func(t *testing.T) {
			props := responseProps(t, name, 200, "application/json", []byte(tc.body))
			if props["payloadType"] != tc.want {
				t.Errorf("payloadType = %v, want %v", props["payloadType"], tc.want)
			}
		})
	}
}

// A list describes many tasks; picking one of them to correlate on would be arbitrary,
// so it reports none.
func TestResponseProperties_TaskListReportsNoIdentifiers(t *testing.T) {
	props := responseProps(t, "ListTasks", 200, "application/json",
		[]byte(`{"tasks":[{"id":"t1","contextId":"c1","status":{"state":"TASK_STATE_COMPLETED"}}],"totalSize":1}`))

	for _, key := range []string{"responseTaskId", "responseContextId", "taskState"} {
		if value, present := props[key]; present {
			t.Errorf("%s = %v, want absent for a task list", key, value)
		}
	}
}

// Emptiness is a result, not a missing observation: DeleteTaskPushNotificationConfig's
// protocol response is Empty, delivered as a 204 on one binding and a null result on
// the other.
func TestResponseProperties_EmptyResults(t *testing.T) {
	t.Run("HTTP+JSON 204", func(t *testing.T) {
		props := responseProps(t, "DeleteTaskPushNotificationConfig", 204, "", nil)
		if props["payloadType"] != "empty" {
			t.Errorf("payloadType = %v, want empty", props["payloadType"])
		}
	})
	t.Run("JSON-RPC null result", func(t *testing.T) {
		props := responseProps(t, "DeleteTaskPushNotificationConfig", 200, "application/json",
			[]byte(`{"jsonrpc":"2.0","id":1,"result":null}`))
		if props["payloadType"] != "empty" {
			t.Errorf("payloadType = %v, want empty", props["payloadType"])
		}
	})
}

// A JSON-RPC error rides a 200 and an HTTP+JSON error is a real status with a REST
// error document that is not an A2A payload at all. Both are the same fact about the
// response, so both must classify as error rather than one of them as `unknown`.
func TestResponseProperties_ErrorsOnBothBindings(t *testing.T) {
	t.Run("JSONRPC", func(t *testing.T) {
		props := responseProps(t, "SendMessage", 200, "application/json",
			[]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`))
		if props["payloadType"] != "error" {
			t.Errorf("payloadType = %v, want error", props["payloadType"])
		}
	})
	t.Run("HTTP+JSON", func(t *testing.T) {
		props := responseProps(t, "GetTask", 404, "application/json",
			[]byte(`{"code":5,"message":"task not found"}`))
		if props["payloadType"] != "error" {
			t.Errorf("payloadType = %v, want error", props["payloadType"])
		}
	})
}

// A 2xx whose body could not be read is not empty and not any known shape. Saying so
// is the point: a rising `unknown` share means the gateway is losing visibility.
func TestResponseProperties_UnreadableSuccessBodyIsUnknown(t *testing.T) {
	props := responseProps(t, "SendMessage", 200, "application/json", []byte("upstream is on fire"))
	if props["payloadType"] != "unknown" {
		t.Errorf("payloadType = %v, want unknown", props["payloadType"])
	}
}

// ─── Response properties: SSE ────────────────────────────────────────────────

// streamProps drives a whole SSE response through the streaming callback, chunk by
// chunk, and returns the block emitted at end of stream.
func streamProps(t *testing.T, contentType string, chunks ...string) map[string]interface{} {
	t.Helper()
	shared := agentSharedContext("SendStreamingMessage", nil)
	shared.Metadata[a2aRequestStartKey] = time.Now().Add(-50 * time.Millisecond)
	ctx := &policy.ResponseStreamContext{
		SharedContext:   shared,
		ResponseStatus:  200,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {contentType}}),
	}

	pol := &AnalyticsPolicy{}
	var action policy.StreamingResponseAction
	for i, chunk := range chunks {
		action = pol.OnResponseBodyChunk(context.Background(), ctx,
			&policy.StreamBody{Chunk: []byte(chunk), EndOfStream: i == len(chunks)-1}, nil)
	}
	return decodeProps(t, action.(policy.ForwardResponseChunk).AnalyticsMetadata,
		A2AResponsePropertiesKey)
}

// A stream reports a task's progress, so the state that matters is the last one it
// reached — while the payload type is whatever the final event was, which need not be
// the event that carried the state.
func TestResponseProperties_StreamRetainsTheLatestTaskState(t *testing.T) {
	props := streamProps(t, "text/event-stream",
		`data: {"jsonrpc":"2.0","id":1,"result":{"task":{"id":"task-7","contextId":"ctx-7","status":{"state":"TASK_STATE_SUBMITTED"}}}}`+"\n\n",
		`data: {"jsonrpc":"2.0","id":1,"result":{"statusUpdate":{"taskId":"task-7","contextId":"ctx-7","status":{"state":"TASK_STATE_WORKING"}}}}`+"\n\n",
		`data: {"jsonrpc":"2.0","id":1,"result":{"artifactUpdate":{"taskId":"task-7","contextId":"ctx-7","artifact":{"artifactId":"a-1"}}}}`+"\n\n",
	)

	assertProps(t, props, map[string]interface{}{
		"payloadType":       "artifact_update",
		"responseTaskId":    "task-7",
		"responseContextId": "ctx-7",
		"taskState":         "TASK_STATE_WORKING",
	})
}

// The HTTP+JSON binding streams the events bare, without the JSON-RPC envelope. The
// properties must come out the same — and isError must stay absent, because an
// unwrapped event states no outcome and this binding's outcome is its status.
func TestResponseProperties_HTTPJSONStreamEventsAreUnwrapped(t *testing.T) {
	props := streamProps(t, "text/event-stream",
		`data: {"task":{"id":"task-8","contextId":"ctx-8","status":{"state":"TASK_STATE_SUBMITTED"}}}`+"\n\n",
		`data: {"statusUpdate":{"taskId":"task-8","contextId":"ctx-8","status":{"state":"TASK_STATE_COMPLETED"}}}`+"\n\n",
	)

	assertProps(t, props, map[string]interface{}{
		"payloadType":       "status_update",
		"responseTaskId":    "task-8",
		"responseContextId": "ctx-8",
		"taskState":         "TASK_STATE_COMPLETED",
	})
	if _, present := props["isError"]; present {
		t.Errorf("isError = %v, want absent — an unwrapped event states no outcome", props["isError"])
	}
}

// A late error decides the payload type, but it does not erase what the stream already
// reported: the task it failed on is exactly what a diagnosis needs.
func TestResponseProperties_LateStreamErrorKeepsEarlierIdentifiers(t *testing.T) {
	props := streamProps(t, "text/event-stream",
		`data: {"jsonrpc":"2.0","id":1,"result":{"task":{"id":"task-3","contextId":"ctx-3","status":{"state":"TASK_STATE_WORKING"}}}}`+"\n\n",
		`data: {"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"agent gave up"}}`+"\n\n",
	)

	assertProps(t, props, map[string]interface{}{
		"payloadType":       "error",
		"responseTaskId":    "task-3",
		"responseContextId": "ctx-3",
		"taskState":         "TASK_STATE_WORKING",
		"isError":           true,
	})
}

// ─── Task state: a bounded dimension ─────────────────────────────────────────

// The 1.0 protobuf enumeration and the JSON spellings that preceded it name the same
// states differently. Folding them onto one spelling is what makes the dimension
// aggregate; leaving them apart would split one state across three labels.
func TestNormalizeA2ATaskState_AcceptsEverySpellingOfAKnownState(t *testing.T) {
	for input, want := range map[string]string{
		"TASK_STATE_COMPLETED": "TASK_STATE_COMPLETED",
		"completed":            "TASK_STATE_COMPLETED",
		"input-required":       "TASK_STATE_INPUT_REQUIRED",
		"INPUT_REQUIRED":       "TASK_STATE_INPUT_REQUIRED",
		"auth-required":        "TASK_STATE_AUTH_REQUIRED",
		"canceled":             "TASK_STATE_CANCELED",
	} {
		if got := normalizeA2ATaskState(input); got != want {
			t.Errorf("normalizeA2ATaskState(%q) = %q, want %q", input, got, want)
		}
	}
}

// taskState is sanctioned as a metric dimension, so it cannot be a passthrough of an
// agent-authored string: an unrecognised value is dropped rather than widening it.
func TestNormalizeA2ATaskState_DropsAnythingOutsideTheProtocolSet(t *testing.T) {
	for _, input := range []string{
		"", "thinking", "TASK_STATE_THINKING", "unknown",
		string(make([]byte, 200)),
	} {
		if got := normalizeA2ATaskState(input); got != "" {
			t.Errorf("normalizeA2ATaskState(%q) = %q, want it dropped", input, got)
		}
	}
}

// An over-long identifier is dropped, never truncated: a truncated identifier is not a
// shorter identifier, it is a different one, and correlating on it would silently group
// unrelated invocations.
func TestResponseProperties_OverLongObservedIdentifierIsDropped(t *testing.T) {
	long := make([]byte, maxA2AObservedValueBytes+1)
	for i := range long {
		long[i] = 'x'
	}
	props := responseProps(t, "GetTask", 200, "application/json",
		[]byte(`{"id":"`+string(long)+`","contextId":"ctx-1","status":{"state":"TASK_STATE_WORKING"}}`))

	if value, present := props["responseTaskId"]; present {
		t.Errorf("responseTaskId = %v, want dropped", value)
	}
	if props["responseContextId"] != "ctx-1" {
		t.Errorf("responseContextId = %v, want ctx-1 — one over-long value must not cost the others",
			props["responseContextId"])
	}
}

// A card fetch rides the Agent's own routes but is not an invocation, so it gets no
// response block at all — including none of the properties added here.
func TestResponseProperties_CardFetchGetsNoBlock(t *testing.T) {
	ctx := &policy.ResponseContext{
		SharedContext:   agentSharedContext("", nil),
		ResponseStatus:  200,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
		ResponseBody:    &policy.Body{Content: []byte(`{"name":"Agent","protocolVersion":"1.0","skills":[]}`)},
	}

	action := (&AnalyticsPolicy{}).OnResponseBody(context.Background(), ctx, nil)
	if mods, ok := action.(policy.DownstreamResponseModifications); ok {
		if _, present := mods.AnalyticsMetadata[A2AResponsePropertiesKey]; present {
			t.Error("a card fetch must not be reported with invocation response properties")
		}
	}
}

func assertProps(t *testing.T, props map[string]interface{}, want map[string]interface{}) {
	t.Helper()
	for key, expected := range want {
		if props[key] != expected {
			t.Errorf("%s = %v, want %v", key, props[key], expected)
		}
	}
}

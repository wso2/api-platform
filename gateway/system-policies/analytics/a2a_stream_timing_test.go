package analytics

import (
	"context"
	"testing"
	"time"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// SSE fixtures, spelled once. A block is only an event once its blank-line terminator
// has arrived, which is what several of these exist to exercise.
const (
	sseHeartbeat     = ": ping\n\n"
	sseRetry         = "retry: 1000\n\n"
	sseEventHalf     = "event: message\ndata: {\"jsonrpc\":\"2.0\""
	sseEventRest     = ",\"id\":1,\"result\":{}}\n\n"
	sseCompleteEvent = "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n"
	sseCRLFEvent     = "event: message\r\ndata: {}\r\n\r\n"
)

func sseStreamContext(shared *policy.SharedContext) *policy.ResponseStreamContext {
	return &policy.ResponseStreamContext{
		SharedContext:   shared,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"text/event-stream"}}),
	}
}

// The behaviour this replaced: any non-empty chunk stamped the mark, so a keep-alive
// comment or the front half of an event counted as "the first event" — making it
// time-to-first-byte. An agent that thinks for thirty seconds while pinging every five
// would have reported a time-to-first-event near zero, so the metric would look best
// exactly when the agent is slowest.
func TestOnResponseBodyChunk_A2AFirstEventSkipsHeartbeatsAndPartialEvents(t *testing.T) {
	shared := agentSharedContext("SendStreamingMessage", nil)
	ctx := sseStreamContext(shared)
	pol := &AnalyticsPolicy{}

	marked := func() bool {
		_, ok := shared.Metadata[a2aFirstEventKey]
		return ok
	}

	// A keep-alive comment is a complete SSE block, but not one a client dispatches.
	pol.OnResponseBodyChunk(context.Background(), ctx,
		&policy.StreamBody{Chunk: []byte(sseHeartbeat)}, nil)
	if marked() {
		t.Fatal("a keep-alive comment must not count as the first event")
	}
	// And the scan must not have to redo it on the next chunk.
	if scanned, _ := shared.Metadata[a2aStreamScanKey].(int); scanned != len(sseHeartbeat) {
		t.Errorf("scan cursor = %v, want it advanced past the comment (%d)",
			shared.Metadata[a2aStreamScanKey], len(sseHeartbeat))
	}

	// The front half of a real event: no terminator yet, so the client has not seen it.
	pol.OnResponseBodyChunk(context.Background(), ctx,
		&policy.StreamBody{Chunk: []byte(sseEventHalf)}, nil)
	if marked() {
		t.Fatal("an unterminated event block must not count as the first event")
	}

	// The terminator arrives: now a client's SSE parser would fire.
	pol.OnResponseBodyChunk(context.Background(), ctx,
		&policy.StreamBody{Chunk: []byte(sseEventRest)}, nil)
	if !marked() {
		t.Fatal("a completed data event must be marked as the first event")
	}
	if _, present := shared.Metadata[a2aStreamScanKey]; present {
		t.Error("the scan cursor must be dropped once an event has been found")
	}
}

// The same property observed through the reported timing rather than the internal
// mark: the agent's think time after the heartbeat has to show up in the metric.
func TestOnResponseBodyChunk_A2ATimeToFirstEventExcludesTheHeartbeatWait(t *testing.T) {
	const agentThinkTime = 60 * time.Millisecond

	shared := agentSharedContext("SendStreamingMessage", nil)
	shared.Metadata[a2aRequestStartKey] = time.Now()
	ctx := sseStreamContext(shared)
	pol := &AnalyticsPolicy{}

	pol.OnResponseBodyChunk(context.Background(), ctx,
		&policy.StreamBody{Chunk: []byte(sseHeartbeat)}, nil)
	time.Sleep(agentThinkTime) // the agent is still working; the client has seen nothing

	action := pol.OnResponseBodyChunk(context.Background(), ctx, &policy.StreamBody{
		Chunk:       []byte(sseCompleteEvent),
		EndOfStream: true,
	}, nil)

	forward, ok := action.(policy.ForwardResponseChunk)
	if !ok {
		t.Fatalf("expected ForwardResponseChunk, got %T", action)
	}
	props := decodeProps(t, forward.AnalyticsMetadata, A2AResponsePropertiesKey)
	ttfe, ok := props["timeToFirstEventMs"].(float64)
	if !ok {
		t.Fatalf("timeToFirstEventMs missing or not a number: %v", props["timeToFirstEventMs"])
	}
	if ttfe < float64(agentThinkTime.Milliseconds()) {
		t.Errorf("timeToFirstEventMs = %v, want at least %d — the heartbeat was counted as the event",
			ttfe, agentThinkTime.Milliseconds())
	}
}

// A stream carrying nothing but heartbeats delivered no event, so it reports no
// timings — the same as a stream that carried nothing at all.
func TestOnResponseBodyChunk_A2AHeartbeatOnlyStreamReportsNoTimings(t *testing.T) {
	shared := agentSharedContext("SendStreamingMessage", nil)
	shared.Metadata[a2aRequestStartKey] = time.Now().Add(-100 * time.Millisecond)
	ctx := sseStreamContext(shared)

	action := (&AnalyticsPolicy{}).OnResponseBodyChunk(context.Background(), ctx,
		&policy.StreamBody{Chunk: []byte(sseHeartbeat + sseRetry), EndOfStream: true}, nil)

	props := decodeProps(t, action.(policy.ForwardResponseChunk).AnalyticsMetadata, A2AResponsePropertiesKey)
	for _, key := range []string{"timeToFirstEventMs", "streamDurationMs"} {
		if _, present := props[key]; present {
			t.Errorf("%s must be absent for a stream that only sent heartbeats", key)
		}
	}
	// Every mark cleared, not only the ones a reported timing consumed: a half-cleared
	// set is how a stale mark would survive.
	for _, key := range []string{a2aFirstEventKey, a2aRequestStartKey, a2aStreamScanKey} {
		if _, present := shared.Metadata[key]; present {
			t.Errorf("%s must be cleared once the stream has ended", key)
		}
	}
}

// A response that declares no event framing has no events to distinguish, so its first
// forwarded chunk is its first delivery.
func TestOnResponseBodyChunk_A2ANonSSEStreamMarksTheFirstChunk(t *testing.T) {
	shared := agentSharedContext("SendStreamingMessage", nil)
	ctx := &policy.ResponseStreamContext{
		SharedContext:   shared,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}}),
	}

	(&AnalyticsPolicy{}).OnResponseBodyChunk(context.Background(), ctx,
		&policy.StreamBody{Chunk: []byte(`{"partial":`)}, nil)

	if _, marked := shared.Metadata[a2aFirstEventKey]; !marked {
		t.Error("a stream with no event framing must mark its first forwarded chunk")
	}
}

// Framing is decided from the declared content type, never sniffed. A first chunk
// holding only a comment is exactly what sniffing gets wrong: isSSEContent does not
// recognise it, so a sniffing implementation would take the no-framing path and mark
// the heartbeat as the first event.
func TestMarkA2AFirstEvent_FramingComesFromTheHeaderNotTheBytes(t *testing.T) {
	if isSSEContent(nil, []byte(sseHeartbeat)) {
		t.Fatal("precondition: a bare comment does not sniff as SSE")
	}

	shared := agentSharedContext("SendStreamingMessage", nil)
	markA2AFirstEvent(shared, []byte(sseHeartbeat),
		policy.NewHeaders(map[string][]string{"content-type": {"text/event-stream"}}))

	if _, marked := shared.Metadata[a2aFirstEventKey]; marked {
		t.Error("the declared content type must decide framing, not the bytes seen so far")
	}
}

func TestFirstSSEDataEventEnd(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		scanFrom  int
		wantFound bool
		wantIndex int
	}{
		{
			name:      "a plain data event",
			body:      sseCompleteEvent,
			wantFound: true,
			wantIndex: len(sseCompleteEvent),
		},
		{
			name:      "CRLF line endings",
			body:      sseCRLFEvent,
			wantFound: true,
			wantIndex: len(sseCRLFEvent),
		},
		{
			name:      "a comment before the event",
			body:      sseHeartbeat + sseCompleteEvent,
			wantFound: true,
			wantIndex: len(sseHeartbeat + sseCompleteEvent),
		},
		{
			// Not an event, but a complete block — so the scan resumes past it rather
			// than re-reading it on the next chunk.
			name:      "a retry directive is not an event",
			body:      sseRetry,
			wantIndex: len(sseRetry),
		},
		{
			name:      "an unterminated block",
			body:      sseEventHalf,
			wantIndex: 0,
		},
		{
			name:      "resuming past an already-scanned comment",
			body:      sseHeartbeat + sseCompleteEvent,
			scanFrom:  len(sseHeartbeat),
			wantFound: true,
			wantIndex: len(sseHeartbeat + sseCompleteEvent),
		},
		{
			name:      "an out-of-range cursor restarts from the beginning",
			body:      sseCompleteEvent,
			scanFrom:  9999,
			wantFound: true,
			wantIndex: len(sseCompleteEvent),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			index, found := firstSSEDataEventEnd([]byte(tc.body), tc.scanFrom)
			if found != tc.wantFound {
				t.Fatalf("found = %v, want %v", found, tc.wantFound)
			}
			if index != tc.wantIndex {
				t.Errorf("index = %d, want %d", index, tc.wantIndex)
			}
		})
	}
}

// The processing mode this policy declares is what decides whether an Agent's SSE
// response streams at all.
//
// The kernel builds a chain's SupportsResponseStreaming from its policies: any
// response-body policy declaring Buffer turns streaming off for the whole chain, and a
// policy declaring Stream without implementing StreamingResponsePolicy does the same.
// This policy is injected into every chain whenever a collector is enabled, and for an
// Agent with no user policies attached it is the *only* response-body policy — so it
// alone determines whether the chain can stream. Flipping it to Buffer would silently
// buffer every A2A stream: each event would be withheld until the task finished, with
// no error anywhere, just a client that waits.
func TestAnalyticsPolicyStreamsResponseBodies(t *testing.T) {
	p := &AnalyticsPolicy{}

	if got := p.Mode().ResponseBodyMode; got != policy.BodyModeStream {
		t.Errorf("ResponseBodyMode = %v, want %v — Agent SSE responses would be buffered",
			got, policy.BodyModeStream)
	}

	// Declaring Stream is not enough on its own: the kernel checks for the interface
	// too, and a chain whose policy declares Stream without implementing it is treated
	// as non-streaming.
	if _, ok := interface{}(p).(policy.StreamingResponsePolicy); !ok {
		t.Error("AnalyticsPolicy must implement policy.StreamingResponsePolicy, " +
			"or the kernel treats its declared Stream mode as non-streaming")
	}
}

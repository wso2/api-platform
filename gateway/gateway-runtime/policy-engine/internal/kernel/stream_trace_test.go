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
	"testing"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
)

func sseResponseHeadersReq() *extprocv3.ProcessingRequest {
	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_ResponseHeaders{ResponseHeaders: sseHeaders()},
	}
}

func responseChunkReq(body string, eos bool) *extprocv3.ProcessingRequest {
	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_ResponseBody{
			ResponseBody: &extprocv3.HttpBody{Body: []byte(body), EndOfStream: eos},
		},
	}
}

// childrenNamed returns the spans called name whose parent is parent. The header phase
// also starts a policy.response.* span, so policy spans are counted per phase.
func childrenNamed(spans []sdktrace.ReadOnlySpan, parent sdktrace.ReadOnlySpan, name string) []sdktrace.ReadOnlySpan {
	var out []sdktrace.ReadOnlySpan
	for _, s := range spansNamed(spans, name) {
		if s.Parent().SpanID() == parent.SpanContext().SpanID() {
			out = append(out, s)
		}
	}
	return out
}

func spansNamed(spans []sdktrace.ReadOnlySpan, name string) []sdktrace.ReadOnlySpan {
	var out []sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Name() == name {
			out = append(out, s)
		}
	}
	return out
}

// A streamed response is one process_response_body span with one span per policy
// under it, however many chunks arrive (#3736). Before, every chunk minted its own
// phase span and its own policy spans.
func TestProcess_StreamedResponseHasOneSpanPerPolicy(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)
	pol := &recordingStreamPolicy{}
	registerTestRoute(k, "stream-route", buildChainWithPolicy(t, "streamer", pol, nil))

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("stream-route", "GET", "/chat"),
		sseResponseHeadersReq(),
		responseChunkReq("data: a\n\n", false),
		responseChunkReq("data: b\n\n", false),
		responseChunkReq("data: [DONE]\n\n", true),
	})
	require.NoError(t, server.Process(stream))
	require.Len(t, pol.chunksSeen, 3, "the policy must still see every chunk")

	ended := sr.Ended()
	phases := spansNamed(ended, constants.SpanProcessResponseBody)
	require.Len(t, phases, 1)
	require.Len(t, childrenNamed(ended, phases[0], "policy.response.streamer"), 1,
		"one policy span for the whole stream, under the stream's phase span")
	count, ok := attrOf(phases[0], attribute.Key(constants.AttrStreamChunkCount))
	require.True(t, ok)
	assert.Equal(t, int64(3), count.AsInt64())

	root := spanByName(ended, constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	assert.Equal(t, root.SpanContext().SpanID(), phases[0].Parent().SpanID())
}

// A stream that closes before the body's end of stream still ends every span, before the
// root span ends.
func TestProcess_StreamedSpansEndWhenStreamClosesEarly(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)
	registerTestRoute(k, "stream-route", buildChainWithPolicy(t, "streamer", &recordingStreamPolicy{}, nil))

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("stream-route", "GET", "/chat"),
		sseResponseHeadersReq(),
		responseChunkReq("data: a\n\n", false),
		responseChunkReq("data: b\n\n", false),
	})
	require.NoError(t, server.Process(stream))

	ended := sr.Ended()
	phases := spansNamed(ended, constants.SpanProcessResponseBody)
	require.Len(t, phases, 1, "the open phase span must be ended when the stream closes")
	require.Len(t, childrenNamed(ended, phases[0], "policy.response.streamer"), 1)
	count, _ := attrOf(phases[0], attribute.Key(constants.AttrStreamChunkCount))
	assert.Equal(t, int64(2), count.AsInt64())

	root := spanByName(ended, constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	assert.False(t, phases[0].EndTime().After(root.EndTime()), "the phase span must end before its root")
}

// A buffered response keeps one span per message, as before.
func TestProcess_BufferedResponseSpansUnchanged(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)
	registerTestRoute(k, "buffered-route", buildChainWithPolicy(t, "streamer", &recordingStreamPolicy{}, nil))

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		requestHeadersReq("buffered-route", "GET", "/pets"),
		responseHeadersReq("200"),
		responseBodyReq(`{"ok":true}`),
	})
	require.NoError(t, server.Process(stream))

	phases := spansNamed(sr.Ended(), constants.SpanProcessResponseBody)
	require.Len(t, phases, 1)
	_, ok := attrOf(phases[0], attribute.Key(constants.AttrStreamChunkCount))
	assert.False(t, ok, "only a streamed body carries a chunk count")
}

// With await spans (#3854) on top of the stream collapse: the engine records one await
// span per change of message type, so a streamed body adds none between its chunks and
// the trace stays a handful of spans however long the stream is.
func TestProcess_StreamedResponseAwaitSpansOnlyBetweenPhases(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)
	server.tracingEnabled = true
	registerTestRoute(k, "stream-route", buildChainWithPolicy(t, "streamer", &recordingStreamPolicy{}, nil))

	msgs := []*extprocv3.ProcessingRequest{
		requestHeadersReq("stream-route", "GET", "/chat"),
		sseResponseHeadersReq(),
	}
	for i := 0; i < 20; i++ {
		msgs = append(msgs, responseChunkReq("data: x\n\n", i == 19))
	}
	require.NoError(t, server.Process(newMockStream(msgs)))

	ended := sr.Ended()
	assert.Len(t, spansNamed(ended, constants.SpanAwaitNextMessage), 2,
		"request headers -> response headers, response headers -> first chunk")
	assert.Len(t, spansNamed(ended, constants.SpanProcessResponseBody), 1)
}

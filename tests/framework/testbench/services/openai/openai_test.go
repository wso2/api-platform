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

package openai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func send(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	New().Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return rec
}

// dataFrames returns the JSON payloads of every "data:" line, skipping [DONE].
func dataFrames(t *testing.T, stream string) []map[string]any {
	t.Helper()
	var frames []map[string]any
	for _, line := range strings.Split(stream, "\n") {
		payload, ok := strings.CutPrefix(line, "data: ")
		if !ok || payload == "[DONE]" {
			continue
		}
		var frame map[string]any
		require.NoError(t, json.Unmarshal([]byte(payload), &frame), "frame %q", payload)
		frames = append(frames, frame)
	}
	return frames
}

func TestStreamRequested(t *testing.T) {
	require.True(t, streamRequested("/openai/v1/chat/completions", []byte(`{"stream":true}`)))
	require.True(t, streamRequested("/gemini/v1/models/gemini-2.5-flash:streamGenerateContent", nil))
	require.False(t, streamRequested("/openai/v1/chat/completions", []byte(`{"stream":false}`)))
	require.False(t, streamRequested("/openai/v1/chat/completions", []byte(`{}`)))
	require.False(t, streamRequested("/openai/v1/chat/completions", []byte(`not json`)))
	require.False(t, streamRequested("/openai/v1/chat/completions", nil))
}

func TestOpenAIStreamCarriesTheReplyAndEndsWithDone(t *testing.T) {
	rec := send(t, "/openai/v1/chat/completions", `{"model":"gpt-4o","stream":true}`)
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	body := rec.Body.String()
	require.True(t, strings.HasSuffix(body, "data: [DONE]\n\n"))

	var text strings.Builder
	for _, frame := range dataFrames(t, body) {
		require.Equal(t, "gpt-4o", frame["model"])
		delta := frame["choices"].([]any)[0].(map[string]any)["delta"].(map[string]any)
		if content, ok := delta["content"].(string); ok {
			text.WriteString(content)
		}
	}
	require.Equal(t, StreamedReply, text.String())
	require.Equal(t, body, send(t, "/openai/v1/chat/completions", `{"model":"gpt-4o","stream":true}`).Body.String(),
		"the stream must be byte-for-byte deterministic")
}

func TestOpenAIStreamErrorTrigger(t *testing.T) {
	body := send(t, "/openai/v1/chat/completions",
		`{"stream":true,"messages":[{"role":"user","content":"STREAM_ERROR"}]}`).Body.String()
	frames := dataFrames(t, body)
	require.Contains(t, frames[len(frames)-1], "error")
	require.NotContains(t, body, "[DONE]")
}

func TestAnthropicStream(t *testing.T) {
	clean := send(t, "/anthropic/v1/messages", `{"model":"claude-x","stream":true}`).Body.String()
	require.Contains(t, clean, "event: message_start\n")
	require.True(t, strings.HasSuffix(clean, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	require.NotContains(t, clean, "event: error")

	failed := send(t, "/anthropic/v1/messages", `{"stream":true,"note":"STREAM_ERROR"}`).Body.String()
	require.True(t, strings.HasSuffix(failed,
		"event: error\ndata: {\"error\":{\"message\":\"Overloaded\",\"type\":\"overloaded_error\"},\"type\":\"error\"}\n\n"))
	require.NotContains(t, failed, "message_stop")
}

func TestGeminiStream(t *testing.T) {
	path := "/gemini/v1/models/gemini-2.5-flash:streamGenerateContent"
	frames := dataFrames(t, send(t, path, `{}`).Body.String())
	require.Len(t, frames, len(replyWords()))
	last := frames[len(frames)-1]
	require.Equal(t, "gemini-2.5-flash", last["modelVersion"])
	require.Equal(t, "STOP", last["candidates"].([]any)[0].(map[string]any)["finishReason"])

	failed := dataFrames(t, send(t, path, `{"contents":[{"parts":[{"text":"STREAM_ERROR"}]}]}`).Body.String())
	require.Equal(t, "UNAVAILABLE", failed[len(failed)-1]["error"].(map[string]any)["status"])
}

func TestGeminiStreamModelFollowsTheAPIVersion(t *testing.T) {
	require.Equal(t, "gemini-2.5-pro", geminiStreamModel("/gemini/v1beta/models/gemini-2.5-pro:streamGenerateContent"))
	require.Equal(t, "gemini-2.5-flash", geminiStreamModel("/gemini/v1/models/gemini-2.5-flash:streamGenerateContent"))
	require.Equal(t, "", geminiStreamModel("/gemini/v1beta/other"))
	frames := dataFrames(t, send(t, "/gemini/v1beta/models/gemini-2.5-pro:streamGenerateContent", `{}`).Body.String())
	require.Equal(t, "gemini-2.5-pro", frames[0]["modelVersion"])
}

func TestNonStreamingRequestsAreUnchanged(t *testing.T) {
	rec := send(t, "/openai/v1/chat/completions", `{"model":"gpt-4o"}`)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	var reply map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &reply))
	require.Equal(t, "chat.completion", reply["object"])
}

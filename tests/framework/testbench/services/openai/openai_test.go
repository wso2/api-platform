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
	New().Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body)))
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

func TestKeyedUpstreamRefusesAWrongKeyWithoutRepeatingIt(t *testing.T) {
	request := func(header, value, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
			KeyedPathPrefix+"/chat/completions", strings.NewReader(body))
		if header != "" {
			req.Header.Set(header, value)
		}
		rec := httptest.NewRecorder()
		New().Handler().ServeHTTP(rec, req)
		return rec
	}

	for name, rec := range map[string]*httptest.ResponseRecorder{
		"wrong api-key":            request("api-key", "sk-wrong-key", `{"model":"gpt-4o"}`),
		"wrong bearer":             request("Authorization", "Bearer sk-wrong-key", `{"model":"gpt-4o"}`),
		"no key":                   request("", "", `{"model":"gpt-4o"}`),
		"empty api-key":            request("api-key", "", `{"model":"gpt-4o"}`),
		"key without bearer":       request("Authorization", "sk-testbench-valid-keyX", `{"model":"gpt-4o"}`),
		"valid key without bearer": request("Authorization", ValidAPIKey, `{"model":"gpt-4o"}`),
	} {
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "invalid_api_key") {
			t.Errorf("%s: body %q does not name invalid_api_key", name, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "sk-wrong-key") {
			t.Errorf("%s: the refusal repeats the presented key: %s", name, rec.Body.String())
		}
	}

	for name, rec := range map[string]*httptest.ResponseRecorder{
		"api-key": request("api-key", ValidAPIKey, `{"model":"gpt-4o"}`),
		"bearer":  request("Authorization", "Bearer "+ValidAPIKey, `{"model":"gpt-4o"}`),
	} {
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "How can I assist you today?") {
			t.Errorf("%s: want the OpenAI chat reply, got %d %s", name, rec.Code, rec.Body.String())
		}
	}

	stream := request("api-key", ValidAPIKey, `{"model":"gpt-4o","stream":true}`)
	if stream.Code != http.StatusOK || !strings.Contains(stream.Body.String(), "data: [DONE]") {
		t.Errorf("a keyed stream should end in [DONE], got %d %s", stream.Code, stream.Body.String())
	}
}

func chatContent(t *testing.T, rec *httptest.ResponseRecorder) (model, content string) {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	var resp struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Choices, 1)
	return resp.Model, resp.Choices[0].Message.Content
}

func TestChatEchoRepliesWithTheLastUserMessage(t *testing.T) {
	cases := []struct {
		name, payload, model, reply string
	}{
		{"string content",
			`{"model":"m","messages":[{"role":"user","content":"first"},{"role":"assistant","content":"x"},{"role":"user","content":"latest question"}]}`,
			"m", "latest question"},
		{"trailing assistant message",
			`{"messages":[{"role":"user","content":"the question"},{"role":"assistant","content":"which one?"}]}`,
			"gpt-4o", "the question"},
		{"content parts",
			`{"messages":[{"role":"user","content":[{"type":"text","text":"part one"},{"type":"image_url","image_url":{"url":"x"}},{"type":"text","text":"part two"}]}]}`,
			"gpt-4o", "part one part two"},
		{"no user message", `{"messages":[{"role":"system","content":"rules"}]}`, "gpt-4o", ""},
		{"no messages", `{}`, "gpt-4o", ""},
		{"non-text content", `{"messages":[{"role":"user","content":42}]}`, "gpt-4o", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model, content := chatContent(t, send(t, ChatEchoPath, tc.payload))
			require.Equal(t, tc.model, model)
			require.Equal(t, tc.reply, content)
		})
	}
}

func TestChatEchoProviderPathBehavesLikeChatEcho(t *testing.T) {
	model, content := chatContent(t, send(t, ChatEchoProviderPath, `{"model":"m","messages":[{"role":"user","content":"routed"}]}`))
	require.Equal(t, "m", model)
	require.Equal(t, "routed", content)

	rec := send(t, ChatEchoProviderPath, `{"stream":true,"messages":[{"role":"user","content":"streamed"}]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Body.String(), `"content":"streamed"`)
	require.True(t, strings.HasSuffix(rec.Body.String(), "data: [DONE]\n\n"))
}

func TestChatEchoRejectsNonJSON(t *testing.T) {
	rec := send(t, ChatEchoPath, "not json")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestChatEchoStreamsTheReply(t *testing.T) {
	rec := send(t, ChatEchoPath, `{"model":"m","stream":true,"messages":[{"role":"user","content":"hello streamed world"}]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))

	events := strings.Split(strings.TrimSuffix(rec.Body.String(), "\n\n"), "\n\n")
	require.Equal(t, "data: [DONE]", events[len(events)-1])
	var reply strings.Builder
	var finish []any
	for _, event := range events[:len(events)-1] {
		data, ok := strings.CutPrefix(event, "data: ")
		require.True(t, ok, event)
		var chunk struct {
			Object  string `json:"object"`
			Model   string `json:"model"`
			Choices []struct {
				Delta        map[string]string `json:"delta"`
				FinishReason any               `json:"finish_reason"`
			} `json:"choices"`
		}
		require.NoError(t, json.Unmarshal([]byte(data), &chunk))
		require.Equal(t, "chat.completion.chunk", chunk.Object)
		require.Equal(t, "m", chunk.Model)
		reply.WriteString(chunk.Choices[0].Delta["content"])
		finish = append(finish, chunk.Choices[0].FinishReason)
	}
	require.Equal(t, "hello streamed world", reply.String())
	require.Len(t, events, 6, "role chunk, three words, stop chunk, [DONE]")
	require.Equal(t, "stop", finish[len(finish)-1])
	for _, reason := range finish[:len(finish)-1] {
		require.Nil(t, reason)
	}
}

func TestChatEchoStreamKeepsTheReplyWhitespace(t *testing.T) {
	const text = "  two  spaces\tand a tab\nand a newline "
	payload, err := json.Marshal(map[string]any{"stream": true, "messages": []map[string]string{{"role": "user", "content": text}}})
	require.NoError(t, err)
	rec := send(t, ChatEchoPath, string(payload))
	require.Equal(t, http.StatusOK, rec.Code)

	var reply strings.Builder
	for _, event := range strings.Split(strings.TrimSuffix(rec.Body.String(), "\n\n"), "\n\n") {
		data, ok := strings.CutPrefix(event, "data: ")
		require.True(t, ok, event)
		if data == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta map[string]string `json:"delta"`
			} `json:"choices"`
		}
		require.NoError(t, json.Unmarshal([]byte(data), &chunk))
		reply.WriteString(chunk.Choices[0].Delta["content"])
	}
	require.Equal(t, text, reply.String())
}

func TestChatEchoStreamsAnEmptyReply(t *testing.T) {
	rec := send(t, ChatEchoPath, `{"stream":true,"messages":[]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 3, strings.Count(rec.Body.String(), "data: "), "role chunk, stop chunk, [DONE]")
}

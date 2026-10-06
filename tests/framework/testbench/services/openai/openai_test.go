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

	"github.com/wso2/api-platform/tests/framework/testbench"
)

func post(t *testing.T, path, payload string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	New().Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload)))
	return rec
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

func TestServiceMetadata(t *testing.T) {
	svc := New()
	require.Equal(t, "openai", svc.Name())
	require.Equal(t, Port, svc.Port())
	require.False(t, svc.Stateful())
	require.NoError(t, (&testbench.Registry{}).Register(svc))
}

func TestChatCompletionsEchoesTheRequestModel(t *testing.T) {
	model, content := chatContent(t, post(t, "/openai/v1/chat/completions", `{"model":"gpt-x"}`))
	require.Equal(t, "gpt-x", model)
	require.Equal(t, "Hello! How can I assist you today?", content)
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
			model, content := chatContent(t, post(t, ChatEchoPath, tc.payload))
			require.Equal(t, tc.model, model)
			require.Equal(t, tc.reply, content)
		})
	}
}

func TestChatEchoRejectsNonJSON(t *testing.T) {
	rec := post(t, ChatEchoPath, "not json")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestChatEchoStreamsTheReply(t *testing.T) {
	rec := post(t, ChatEchoPath, `{"model":"m","stream":true,"messages":[{"role":"user","content":"hello streamed world"}]}`)
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

func TestChatEchoStreamsAnEmptyReply(t *testing.T) {
	rec := post(t, ChatEchoPath, `{"stream":true,"messages":[]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 3, strings.Count(rec.Body.String(), "data: "), "role chunk, stop chunk, [DONE]")
}

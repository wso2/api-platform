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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// StreamedReply is the assistant text every streamed reply carries, one word per delta.
const StreamedReply = "This is a streamed reply from the mock model."

// StreamErrorTrigger, anywhere in a streaming request, ends the stream with the provider's
// error event instead of a clean finish.
const StreamErrorTrigger = "STREAM_ERROR"

// geminiStreamMethod is the Gemini method a streamed generateContent call uses.
const geminiStreamMethod = ":streamGenerateContent"

// streamRequested reports whether a request asked for a streamed reply: OpenAI-style and
// Anthropic requests set "stream": true, and Gemini names a streaming method in the path.
func streamRequested(path string, body []byte) bool {
	if strings.Contains(path, geminiStreamMethod) {
		return true
	}
	var req struct {
		Stream bool `json:"stream"`
	}
	return json.Unmarshal(body, &req) == nil && req.Stream
}

// writeStream answers a streaming request with a server-sent event stream in the requesting
// provider's wire format.
//
// The whole stream is written in a single flush. A policy reading the stream then sees the
// same chunk boundaries on every run, so where a streaming guardrail or translator cuts the
// stream off does not depend on network timing.
func (s *Service) writeStream(w http.ResponseWriter, path string, body []byte) {
	fail := bytes.Contains(body, []byte(StreamErrorTrigger))
	var out strings.Builder
	switch {
	case strings.HasPrefix(path, "/anthropic/"):
		anthropicStream(&out, orElse(requestModel(body), "claude-sonnet-4-5-20250929"), fail)
	case strings.HasPrefix(path, "/gemini/"):
		geminiStream(&out, orElse(geminiStreamModel(path), "gemini-2.5-flash"), fail)
	default:
		openAIStream(&out, orElse(requestModel(body), "gpt-4.1-2025-04-14"), fail)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(out.String()))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// geminiStreamModel returns the model a Gemini streaming path names: the segment after
// "/models/" and before the method, whichever API version precedes it.
func geminiStreamModel(path string) string {
	_, rest, ok := strings.Cut(path, "/models/")
	if !ok {
		return ""
	}
	model, _, _ := strings.Cut(rest, ":")
	return model
}

// replyWords splits StreamedReply into the per-delta pieces a stream carries, keeping each
// word's trailing space so the concatenated deltas equal the reply.
func replyWords() []string {
	words := strings.SplitAfter(StreamedReply, " ")
	out := words[:0]
	for _, w := range words {
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

func sseData(out *strings.Builder, payload any) {
	encoded, _ := json.Marshal(payload)
	fmt.Fprintf(out, "data: %s\n\n", encoded)
}

func sseEvent(out *strings.Builder, event string, payload any) {
	encoded, _ := json.Marshal(payload)
	fmt.Fprintf(out, "event: %s\ndata: %s\n\n", event, encoded)
}

// openAIStream writes an OpenAI chat-completion chunk stream ending in [DONE], or in an
// error object when fail is set.
func openAIStream(out *strings.Builder, model string, fail bool) {
	chunk := func(delta map[string]any, finish any) map[string]any {
		return map[string]any{
			"id": "chatcmpl-mock-stream", "object": "chat.completion.chunk", "created": 1741569952,
			"model":   model,
			"choices": []map[string]any{{"index": 0, "delta": delta, "finish_reason": finish}},
		}
	}
	sseData(out, chunk(map[string]any{"role": "assistant", "content": ""}, nil))
	for _, word := range replyWords() {
		sseData(out, chunk(map[string]any{"content": word}, nil))
	}
	if fail {
		sseData(out, map[string]any{"error": map[string]any{
			"message": "The mock model failed mid-stream.", "type": "server_error",
		}})
		return
	}
	sseData(out, chunk(map[string]any{}, "stop"))
	out.WriteString("data: [DONE]\n\n")
}

// anthropicStream writes an Anthropic Messages event stream, ending in an error event when
// fail is set.
func anthropicStream(out *strings.Builder, model string, fail bool) {
	sseEvent(out, "message_start", map[string]any{"type": "message_start", "message": map[string]any{
		"id": "msg_mock_stream", "type": "message", "role": "assistant", "model": model,
		"content": []any{}, "stop_reason": nil,
		"usage": map[string]any{"input_tokens": 10, "output_tokens": 1},
	}})
	sseEvent(out, "content_block_start", map[string]any{
		"type": "content_block_start", "index": 0,
		"content_block": map[string]any{"type": "text", "text": ""},
	})
	for _, word := range replyWords() {
		sseEvent(out, "content_block_delta", map[string]any{
			"type": "content_block_delta", "index": 0,
			"delta": map[string]any{"type": "text_delta", "text": word},
		})
	}
	if fail {
		sseEvent(out, "error", map[string]any{"type": "error", "error": map[string]any{
			"type": "overloaded_error", "message": "Overloaded",
		}})
		return
	}
	sseEvent(out, "content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
	sseEvent(out, "message_delta", map[string]any{
		"type": "message_delta", "delta": map[string]any{"stop_reason": "end_turn"},
		"usage": map[string]any{"output_tokens": len(replyWords())},
	})
	sseEvent(out, "message_stop", map[string]any{"type": "message_stop"})
}

// geminiStream writes a Gemini streamGenerateContent (alt=sse) stream, ending in an error
// frame when fail is set.
func geminiStream(out *strings.Builder, model string, fail bool) {
	words := replyWords()
	for i, word := range words {
		candidate := map[string]any{
			"content": map[string]any{"parts": []map[string]any{{"text": word}}, "role": "model"},
			"index":   0,
		}
		frame := map[string]any{"candidates": []map[string]any{candidate}, "modelVersion": model}
		if i == len(words)-1 && !fail {
			candidate["finishReason"] = "STOP"
			frame["usageMetadata"] = map[string]any{
				"promptTokenCount": 10, "candidatesTokenCount": len(words), "totalTokenCount": 10 + len(words),
			}
		}
		sseData(out, frame)
	}
	if fail {
		sseData(out, map[string]any{"error": map[string]any{
			"code": 503, "message": "The model is overloaded.", "status": "UNAVAILABLE",
		}})
	}
}

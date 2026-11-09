// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// mock-llm-stream is an OpenAI-compatible chat completions backend on :8080 whose
// streamed responses really stream: each server-sent event is flushed on its own,
// a few milliseconds apart, so the gateway receives them as separate body chunks. The
// last event carries the token usage, as OpenAI's include_usage does.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

var words = []string{"Hello", "!", " How", " can", " I", " help", "?"}

const (
	promptTokens     = 10
	completionTokens = 7
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("POST /v1/chat/completions", chat)
	log.Println("mock-llm-stream listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func chat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	const id = "chatcmpl-mock-stream"
	usage := map[string]int{"prompt_tokens": promptTokens, "completion_tokens": completionTokens, "total_tokens": promptTokens + completionTokens}
	if !req.Stream {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": id, "object": "chat.completion", "model": req.Model,
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "Hello! How can I help?"}, "finish_reason": "stop"}},
			"usage":   usage,
		})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	send := func(v any) {
		b, _ := json.Marshal(v)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
		time.Sleep(20 * time.Millisecond)
	}
	chunk := func(delta map[string]any, finish any) map[string]any {
		return map[string]any{"id": id, "object": "chat.completion.chunk", "model": req.Model,
			"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
	}
	send(chunk(map[string]any{"role": "assistant", "content": ""}, nil))
	for _, word := range words {
		send(chunk(map[string]any{"content": word}, nil))
	}
	send(chunk(map[string]any{}, "stop"))
	send(map[string]any{"id": id, "object": "chat.completion.chunk", "model": req.Model, "choices": []any{}, "usage": usage})
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

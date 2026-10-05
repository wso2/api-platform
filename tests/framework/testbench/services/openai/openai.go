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

// Package openai provides a deterministic, stateless mock upstream for several LLM providers.
// Responses preserve the provider-specific shapes and usage fields consumed by gateway policies.
package openai

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
)

// Port is the container port used by the testbench.
const Port = 3008

// Service implements the stateless LLM mock used by the testbench.
type Service struct{}

// New returns a new Service.
func New() *Service { return &Service{} }

// Name returns the service registration name.
func (s *Service) Name() string { return "openai" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// Default token counts are used by the fallback response and must remain stable for generic
// chat-completion scenarios.
const (
	defaultPromptTokens     = 10
	defaultCompletionTokens = 6
	defaultTotalTokens      = 16
)

// Handler returns the HTTP handler for the service.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.route)
	return mux
}

// route selects a response by request path. Unknown paths use the default chat completion.
func (s *Service) route(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "could not read request body", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path

	switch {
	// OpenAI
	case path == "/openai/v1/chat/completions":
		writeJSON(w, s.openAIChat(body))
	case path == "/openai/v1/chat-cached":
		writeJSON(w, s.openAICached(body))
	case path == "/openai/v1/chat-flex":
		writeJSON(w, s.openAIFlex(body))
	case path == "/openai/v1/chat-priority":
		writeJSON(w, s.openAIPriority(body))
	case path == "/openai/v1/chat-batch":
		writeJSON(w, s.openAIBatch(body))
	case path == "/openai/v1/chat-reasoning":
		writeJSON(w, s.openAIReasoning(body))
	case path == "/openai/v1/chat-web-search":
		writeJSON(w, s.openAIWebSearch(body))
	case path == ChatEchoPath:
		s.openAIChatEcho(w, body)

	// Anthropic
	case path == "/anthropic/v1/messages":
		writeJSON(w, s.anthropicMessages(body))
	case path == "/anthropic/v1/messages-geo-speed":
		writeJSON(w, s.anthropicGeoSpeed(body))
	case path == "/anthropic/v1/messages-cache-1hr":
		writeJSON(w, s.anthropicCache1hr(body))
	case path == "/anthropic/v1/messages-web-search":
		writeJSON(w, s.anthropicWebSearch(body))
	case path == "/anthropic/v1/messages-cache-read":
		writeJSON(w, s.anthropicCacheRead(body))

	// Gemini; the model is part of the URL.
	case strings.HasPrefix(path, "/gemini/v1/models/"):
		writeJSON(w, s.geminiGenerate(geminiModel(path, "/gemini/v1/models/")))
	case strings.HasPrefix(path, "/gemini/v1/cached/"):
		writeJSON(w, s.geminiCached(geminiModel(path, "/gemini/v1/cached/")))
	case strings.HasPrefix(path, "/gemini/v1/thinking/"):
		writeJSON(w, s.geminiThinking(geminiModel(path, "/gemini/v1/thinking/")))

	// Bedrock Converse; the model is part of the URL.
	case strings.HasPrefix(path, "/model/") && strings.HasSuffix(path, "/converse"):
		writeJSON(w, s.bedrockConverse())

	// Mistral
	case path == "/mistral/v1/chat/completions":
		writeJSON(w, s.mistralChat(body))

	// Unknown and unpriced models
	case path == "/unknown-llm/v1/chat":
		writeJSON(w, s.unknownChat(body))
	case path == "/unknown-llm/v1/no-model-field":
		writeJSON(w, s.noModelField())

	// Readiness probe
	case path == ReadinessPath:
		writeJSON(w, s.zeroUsage(body))

	default:
		writeJSON(w, s.defaultChat(body))
	}
}

// Model extraction helpers.

// requestModel returns the "model" field of a JSON request body, or "" if absent/unparseable.
func requestModel(body []byte) string {
	var b struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &b)
	return b.Model
}

// geminiModel extracts the model from a Gemini path segment.
func geminiModel(path, prefix string) string {
	return strings.TrimSuffix(strings.TrimPrefix(path, prefix), ":generateContent")
}

func orElse(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

// OpenAI chat completions.

// openAIUsage builds the usage fields consumed by cost and token policies.
func openAIUsage(prompt, cached, completion, reasoning, total int) map[string]any {
	return map[string]any{
		"prompt_tokens":     prompt,
		"completion_tokens": completion,
		"total_tokens":      total,
		"prompt_tokens_details": map[string]any{
			"cached_tokens": cached,
			"audio_tokens":  0,
		},
		"completion_tokens_details": map[string]any{
			"reasoning_tokens":           reasoning,
			"audio_tokens":               0,
			"accepted_prediction_tokens": 0,
			"rejected_prediction_tokens": 0,
		},
	}
}

// openAIChatEnvelope builds an OpenAI chat-completion response.
func openAIChatEnvelope(id, model, content, serviceTier string, annotations []any, usage map[string]any) map[string]any {
	if annotations == nil {
		annotations = []any{}
	}
	return map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": 1741569952,
		"model":   model,
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":        "assistant",
				"content":     content,
				"refusal":     nil,
				"annotations": annotations,
			},
			"logprobs":      nil,
			"finish_reason": "stop",
		}},
		"usage":        usage,
		"service_tier": serviceTier,
	}
}

func (s *Service) openAIChat(body []byte) map[string]any {
	return openAIChatEnvelope("chatcmpl-B9MBs8CjcvOU2jLn4n570S5qMJKcT",
		orElse(requestModel(body), "gpt-4.1-2025-04-14"),
		"Hello! How can I assist you today?", "default", nil,
		openAIUsage(19, 0, 10, 0, 29))
}

func (s *Service) openAICached(body []byte) map[string]any {
	return openAIChatEnvelope("chatcmpl-CachedPromptTest",
		orElse(requestModel(body), "gpt-4.1-2025-04-14"),
		"Response using cached prompt tokens.", "default", nil,
		openAIUsage(200, 100, 50, 0, 250))
}

func (s *Service) openAIFlex(body []byte) map[string]any {
	return openAIChatEnvelope("chatcmpl-FlexTierTest",
		orElse(requestModel(body), "gpt-5.4"),
		"Flex tier response.", "flex", nil,
		openAIUsage(100, 0, 50, 0, 150))
}

func (s *Service) openAIPriority(body []byte) map[string]any {
	return openAIChatEnvelope("chatcmpl-PriorityTierTest",
		orElse(requestModel(body), "gpt-4.1"),
		"Priority tier response.", "priority", nil,
		openAIUsage(100, 0, 50, 0, 150))
}

func (s *Service) openAIBatch(body []byte) map[string]any {
	return openAIChatEnvelope("chatcmpl-BatchTierTest",
		orElse(requestModel(body), "gpt-4.1"),
		"Batch tier response.", "batch", nil,
		openAIUsage(100, 0, 50, 0, 150))
}

func (s *Service) openAIReasoning(body []byte) map[string]any {
	return openAIChatEnvelope("chatcmpl-ReasoningTokenTest",
		orElse(requestModel(body), "o4-mini-2025-04-16"),
		"The answer is 42.", "default", nil,
		openAIUsage(100, 0, 80, 30, 180))
}

func (s *Service) openAIWebSearch(body []byte) map[string]any {
	annotations := []any{map[string]any{
		"type": "url_citation",
		"url_citation": map[string]any{
			"url":         "https://example.com/answer",
			"title":       "The Answer",
			"start_index": 30,
			"end_index":   32,
		},
	}}
	return openAIChatEnvelope("chatcmpl-WebSearchTest",
		orElse(requestModel(body), "gpt-4.1-2025-04-14"),
		"According to recent sources, the answer is 42.", "default", annotations,
		openAIUsage(50, 0, 25, 0, 75))
}

// ChatEchoPath answers with the request's last user message as the assistant's reply, so a
// scenario controls the response text a gateway policy sees. With "stream": true the reply is
// sent as OpenAI chat-completion chunks over server-sent events, one word per chunk.
const ChatEchoPath = "/openai/v1/chat-echo"

// echoRequest is the part of a chat-completions request the echo path reads.
type echoRequest struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream"`
	Messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
}

func (s *Service) openAIChatEcho(w http.ResponseWriter, body []byte) {
	var req echoRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "openai: chat-echo needs a JSON chat-completions request", http.StatusBadRequest)
		return
	}
	reply := lastUserText(req)
	model := orElse(req.Model, "gpt-4o")
	if !req.Stream {
		writeJSON(w, openAIChatEnvelope("chatcmpl-echo", model, reply, "default", nil,
			openAIUsage(defaultPromptTokens, 0, defaultCompletionTokens, 0, defaultTotalTokens)))
		return
	}
	writeEchoStream(w, model, reply)
}

// lastUserText returns the text of the last user message: a string content, or the text parts
// of a content-part array joined with a space.
func lastUserText(req echoRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		message := req.Messages[i]
		if message.Role != "user" {
			continue
		}
		var text string
		if json.Unmarshal(message.Content, &text) == nil {
			return text
		}
		var parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(message.Content, &parts) != nil {
			return ""
		}
		var texts []string
		for _, part := range parts {
			if part.Type == "text" {
				texts = append(texts, part.Text)
			}
		}
		return strings.Join(texts, " ")
	}
	return ""
}

func writeEchoStream(w http.ResponseWriter, model, reply string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	chunk := func(delta map[string]any, finishReason any) {
		payload, _ := json.Marshal(map[string]any{
			"id": "chatcmpl-echo", "object": "chat.completion.chunk", "created": 1741569952, "model": model,
			"choices": []map[string]any{{"index": 0, "delta": delta, "finish_reason": finishReason}},
		})
		if _, err := w.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
			log.Printf("openai: writing stream: %v", err)
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	chunk(map[string]any{"role": "assistant", "content": ""}, nil)
	for i, word := range strings.Fields(reply) {
		if i > 0 {
			word = " " + word
		}
		chunk(map[string]any{"content": word}, nil)
	}
	chunk(map[string]any{}, "stop")
	if _, err := w.Write([]byte("data: [DONE]\n\n")); err != nil {
		log.Printf("openai: writing stream: %v", err)
	}
	if flusher != nil {
		flusher.Flush()
	}
}

// Anthropic messages.

// anthropicUsage builds the provider-specific usage block.
func anthropicUsage(input, output, cacheCreate, cacheRead, eph5m, eph1h, webSearch int, geo string) map[string]any {
	usage := map[string]any{
		"input_tokens":                input,
		"output_tokens":               output,
		"cache_creation_input_tokens": cacheCreate,
		"cache_read_input_tokens":     cacheRead,
		"cache_creation": map[string]any{
			"ephemeral_5m_input_tokens": eph5m,
			"ephemeral_1h_input_tokens": eph1h,
		},
		"server_tool_use": map[string]any{
			"web_search_requests": webSearch,
			"web_fetch_requests":  0,
		},
		"service_tier": "standard",
	}
	if geo != "" {
		usage["inference_geo"] = geo
	}
	return usage
}

func anthropicEnvelope(id, model, content string, usage map[string]any) map[string]any {
	return map[string]any{
		"id":            id,
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       []map[string]any{{"type": "text", "text": content}},
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage":         usage,
	}
}

func (s *Service) anthropicMessages(body []byte) map[string]any {
	return anthropicEnvelope("msg_01XFDUDYJgAACzvnptvVoYEL",
		orElse(requestModel(body), "claude-3-5-haiku-20241022"),
		"Hello! How can I help you today?",
		anthropicUsage(50, 25, 0, 0, 0, 0, 0, ""))
}

func (s *Service) anthropicGeoSpeed(body []byte) map[string]any {
	return anthropicEnvelope("msg_geo_speed_01",
		orElse(requestModel(body), "claude-opus-4-6"),
		"Hello with geo and speed!",
		anthropicUsage(20, 10, 0, 0, 0, 0, 0, "us"))
}

func (s *Service) anthropicCache1hr(body []byte) map[string]any {
	return anthropicEnvelope("msg_cache1hr_example",
		orElse(requestModel(body), "claude-opus-4-6"),
		"Hello! How can I help you today?",
		anthropicUsage(10, 5, 600, 0, 100, 500, 0, ""))
}

func (s *Service) anthropicWebSearch(body []byte) map[string]any {
	return anthropicEnvelope("msg_websearch_example",
		orElse(requestModel(body), "claude-3-5-haiku-20241022"),
		"Here are the search results.",
		anthropicUsage(50, 25, 0, 0, 0, 0, 2, ""))
}

func (s *Service) anthropicCacheRead(body []byte) map[string]any {
	return anthropicEnvelope("msg_01CachReadTest",
		orElse(requestModel(body), "claude-3-5-haiku-20241022"),
		"Response using cached context.",
		anthropicUsage(50, 25, 0, 200, 0, 0, 0, ""))
}

// Gemini generateContent responses.

func geminiEnvelope(model, content string, usageMetadata map[string]any) map[string]any {
	return map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{
				"parts": []map[string]any{{"text": content}},
				"role":  "model",
			},
			"finishReason": "STOP",
			"index":        0,
		}},
		"modelVersion":  model,
		"usageMetadata": usageMetadata,
	}
}

func (s *Service) geminiGenerate(model string) map[string]any {
	return geminiEnvelope(orElse(model, "gemini-1.5-flash-002"),
		"Hello! How can I assist you today?",
		map[string]any{
			"promptTokenCount":     100,
			"candidatesTokenCount": 100,
			"totalTokenCount":      200,
		})
}

func (s *Service) geminiCached(model string) map[string]any {
	return geminiEnvelope(orElse(model, "gemini-2.0-flash"),
		"Here is your response using cached context.",
		map[string]any{
			"promptTokenCount":        500,
			"cachedContentTokenCount": 200,
			"candidatesTokenCount":    100,
			"totalTokenCount":         600,
		})
}

func (s *Service) geminiThinking(model string) map[string]any {
	return geminiEnvelope(orElse(model, "gemini-2.5-flash-preview-04-17"),
		"The answer is 42.",
		map[string]any{
			"promptTokenCount":     100,
			"candidatesTokenCount": 50,
			"thoughtsTokenCount":   30,
			"totalTokenCount":      180,
		})
}

// Bedrock Converse.
func (s *Service) bedrockConverse() map[string]any {
	return map[string]any{
		"output": map[string]any{
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"text": "Hello from Bedrock"}},
			},
		},
		"stopReason": "end_turn",
		"usage": map[string]any{
			"inputTokens":  10,
			"outputTokens": 3,
			"totalTokens":  13,
		},
	}
}

// Mistral and unpriced-model responses.

func (s *Service) mistralChat(body []byte) map[string]any {
	return map[string]any{
		"id":      "cmpl-e5cc70bb28c444948073e77776eb30ef",
		"object":  "chat.completion",
		"created": 1702256327,
		"model":   orElse(requestModel(body), "mistral-small-latest"),
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": "Hello! How can I assist you today?",
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":        100,
			"completion_tokens":    50,
			"total_tokens":         150,
			"prompt_audio_seconds": nil,
		},
	}
}

func (s *Service) unknownChat(body []byte) map[string]any {
	return map[string]any{
		"id":    "resp-001",
		"model": orElse(requestModel(body), "my-unknown-model-xyz"),
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": "Hello!"},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 5,
			"total_tokens":      15,
		},
	}
}

func (s *Service) noModelField() map[string]any {
	return map[string]any{
		"id": "resp-no-model-001",
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": "Hello!"},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 5,
			"total_tokens":      15,
		},
	}
}

// ReadinessPath is used by provider readiness probes.
const ReadinessPath = "/__readiness"

// zeroUsage returns zero-valued usage in every provider format recognized by the gateway.
// Keeping all formats present avoids the token-policy fallback charge for an unrecognized usage
// shape. This can be removed when the fallback behavior is corrected (wso2/api-platform#3253).
func (s *Service) zeroUsage(body []byte) map[string]any {
	return map[string]any{
		"id":      "chatcmpl-readiness",
		"object":  "chat.completion",
		"created": 1700000000,
		"model":   orElse(requestModel(body), "readiness"),
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": ""},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"input_tokens":      0,
			"output_tokens":     0,
			"inputTokens":       0,
			"outputTokens":      0,
			"totalTokens":       0,
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     0,
			"candidatesTokenCount": 0,
			"totalTokenCount":      0,
		},
	}
}

func (s *Service) defaultChat(body []byte) map[string]any {
	return map[string]any{
		"id":      "chatcmpl-testbench",
		"object":  "chat.completion",
		"created": 1700000000,
		"model":   orElse(requestModel(body), "gpt-4"),
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": "Hello from the testbench.",
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     defaultPromptTokens,
			"completion_tokens": defaultCompletionTokens,
			"total_tokens":      defaultTotalTokens,
		},
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	payload, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "could not encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(payload); err != nil {
		log.Printf("openai: writing response: %v", err)
	}
}

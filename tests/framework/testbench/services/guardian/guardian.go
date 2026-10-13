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

// Package guardian provides a deterministic content-safety model for the guardrail policies
// that ask an OpenAI-compatible chat model for a verdict: NeMo Guard Content Safety and
// Granite Guardian.
//
// The verdict is driven by keywords in the text under review, so a scenario chooses the
// outcome by what it sends:
//
//	UNSAFE_USER      the user turn is unsafe (NeMo Guard "User Safety")
//	UNSAFE_RESPONSE  the agent turn is unsafe (NeMo Guard "Response Safety")
//	UNSAFE           Granite Guardian detects the configured risk
//	GUARDIAN_DOWN    the model fails (503), as an unavailable safety service does
//
// NeMo Guard sends both turns in one prompt, so its two keywords are distinct: each check
// reads only its own verdict.
package guardian

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// Port is the container port used by the testbench.
const Port = 3016

// ChatPath is the OpenAI-compatible route both guardrails call.
const ChatPath = "/v1/chat/completions"

// Keywords that select an unsafe verdict.
const (
	UnsafeUser     = "UNSAFE_USER"
	UnsafeResponse = "UNSAFE_RESPONSE"
	Unsafe         = "UNSAFE"
	// Down makes the request fail, so a scenario reaches a guardrail's
	// service-unavailable path without a gateway configured to point elsewhere.
	Down = "GUARDIAN_DOWN"
)

// UnsafeCategory is the NeMo Guard safety category an unsafe verdict names.
const UnsafeCategory = "Violence"

// graniteConfigMarker identifies a Granite Guardian request: it carries the risk definition
// in a <guardianconfig> system message.
const graniteConfigMarker = "<guardianconfig>"

const maxRequestBodySize = 1 << 20

type chatRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

// Service implements testbench.Service.
type Service struct{}

// New returns a new guardian service.
func New() *Service { return &Service{} }

// Name returns the service registration name.
func (s *Service) Name() string { return "guardian" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// Handler returns the chat-completion endpoint.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+ChatPath, s.chat)
	return mux
}

func (s *Service) chat(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "could not read request body", http.StatusBadRequest)
		return
	}
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "request body is not a chat completion request", http.StatusBadRequest)
		return
	}
	if strings.Contains(string(body), Down) {
		http.Error(w, "the safety model is unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": "chatcmpl-guardian", "object": "chat.completion", "created": 1741569952,
		"model": "guardian",
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": verdict(req)},
			"finish_reason": "stop",
		}},
	})
}

// verdict returns the model text for a request: Granite Guardian's score tag, or NeMo Guard's
// safety JSON.
func verdict(req chatRequest) string {
	var system, text strings.Builder
	for _, m := range req.Messages {
		if m.Role == "system" {
			system.WriteString(m.Content)
			continue
		}
		text.WriteString(m.Content)
		text.WriteByte('\n')
	}
	reviewed := text.String()
	if strings.Contains(system.String(), graniteConfigMarker) {
		if strings.Contains(reviewed, Unsafe) {
			return "<score> yes </score>"
		}
		return "<score> no </score>"
	}

	rating := func(unsafe bool) string {
		if unsafe {
			return "unsafe"
		}
		return "safe"
	}
	userUnsafe := strings.Contains(reviewed, UnsafeUser)
	responseUnsafe := strings.Contains(reviewed, UnsafeResponse)
	result := map[string]string{
		"User Safety":     rating(userUnsafe),
		"Response Safety": rating(responseUnsafe),
	}
	if userUnsafe || responseUnsafe {
		result["Safety Categories"] = UnsafeCategory
	}
	encoded, _ := json.Marshal(result)
	return string(encoded)
}

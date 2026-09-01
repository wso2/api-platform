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

// Package interceptor provides a stateless request/response interceptor upstream.
package interceptor

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// Port is the container port used by the testbench.
const Port = 3003

type invocationContext struct {
	Path string `json:"path,omitempty"`
}

type handleRequestBody struct {
	InvocationContext invocationContext `json:"invocationContext"`
}

type handleResponseBody struct {
	InvocationContext  invocationContext `json:"invocationContext"`
	InterceptorContext map[string]string `json:"interceptorContext,omitempty"`
}

// reply is the wire contract consumed by the interceptor policy.
type reply struct {
	DirectRespond      bool              `json:"directRespond,omitempty"`
	ResponseCode       int               `json:"responseCode,omitempty"`
	HeadersToAdd       map[string]string `json:"headersToAdd,omitempty"`
	PathToRewrite      string            `json:"pathToRewrite,omitempty"`
	Body               string            `json:"body,omitempty"`
	InterceptorContext map[string]string `json:"interceptorContext,omitempty"`
}

// Service implements the interceptor testbench service.
type Service struct{}

// New returns a new interceptor service.
func New() *Service { return &Service{} }

// Name returns the service registration name.
func (s *Service) Name() string { return "interceptor" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// Handler returns the policy hooks and health handler.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/handle-request", s.handleRequest)
	mux.HandleFunc("/handle-response", s.handleResponse)
	return mux
}

func (s *Service) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "interceptor",
	})
}

// handleRequest processes the request-phase hook.
func (s *Service) handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req handleRequestBody
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	path := req.InvocationContext.Path
	switch {
	case hasPathSegment(path, "block"):
		respondJSON(w, http.StatusOK, reply{
			DirectRespond: true,
			ResponseCode:  http.StatusForbidden,
			HeadersToAdd: map[string]string{
				"Content-Type":              "application/json",
				"X-Interceptor-Decision":    "blocked",
				"X-Interceptor-RequestHook": "true",
			},
			Body: base64.StdEncoding.EncodeToString([]byte(`{"error":"blocked by interceptor"}`)),
		})
	case hasPathSegment(path, "mutate"):
		respondJSON(w, http.StatusOK, reply{
			HeadersToAdd: map[string]string{
				"X-Interceptor-Request": "true",
			},
			PathToRewrite: "/anything/intercepted",
			Body:          base64.StdEncoding.EncodeToString([]byte(`{"message":"mutated-by-interceptor"}`)),
			InterceptorContext: map[string]string{
				"trace": "request-phase",
			},
		})
	case hasPathSegment(path, "response-rewrite"):
		respondJSON(w, http.StatusOK, reply{
			InterceptorContext: map[string]string{
				"trace": "request-phase",
			},
		})
	default:
		respondJSON(w, http.StatusOK, reply{})
	}
}

// handleResponse processes the response-phase hook.
func (s *Service) handleResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req handleResponseBody
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	if hasPathSegment(req.InvocationContext.Path, "response-rewrite") {
		trace := req.InterceptorContext["trace"]
		if trace == "" {
			trace = "missing"
		}
		respondJSON(w, http.StatusOK, reply{
			ResponseCode: http.StatusAccepted,
			HeadersToAdd: map[string]string{
				"Content-Type":           "application/json",
				"X-Interceptor-Response": "true",
				"X-Interceptor-Trace":    trace,
			},
			Body: base64.StdEncoding.EncodeToString([]byte(`{"message":"response-overridden"}`)),
		})
		return
	}

	respondJSON(w, http.StatusOK, reply{})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "could not encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(encoded); err != nil {
		log.Printf("interceptor: failed to write response: %v", err)
	}
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		return err
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func hasPathSegment(path, want string) bool {
	for _, segment := range strings.Split(strings.Trim(path, "/"), "/") {
		if segment == want {
			return true
		}
	}
	return false
}

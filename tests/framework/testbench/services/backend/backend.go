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

// Package backend provides the stateless upstream used by integration tests.
package backend

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

// Port is the container port. Features address http://testbench:3000.
const Port = 3000

// Service implements the stateless backend test service.
type Service struct{}

// New creates a backend service.
func New() *Service { return &Service{} }

// Name returns the service name used by the testbench registry.
func (s *Service) Name() string { return "backend" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// requestInfo is the response format expected by the integration tests.
type requestInfo struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Query   string              `json:"query,omitempty"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body,omitempty"`
}

// Handler returns the backend service's HTTP handler.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	// The delay endpoint exercises upstream response timeouts independently of connection
	// failures.
	mux.HandleFunc("GET /delay/{seconds}", s.delay)
	mux.HandleFunc("GET /sandbox/whoami", s.whoami)
	mux.HandleFunc("/", s.reflect)
	return mux
}

// reflect returns the incoming request as JSON for arbitrary upstream paths.
func (s *Service) reflect(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()

	// net/http stores the Host separately from the ordinary headers. Clone the map before
	// adding it to keep the request unchanged.
	headers := r.Header.Clone()
	if r.Host != "" {
		headers.Set("Host", r.Host)
	}

	info := requestInfo{
		Method:  r.Method,
		Path:    r.URL.Path,
		Query:   r.URL.RawQuery,
		Headers: headers,
	}
	if len(body) > 0 {
		info.Body = string(body)
	}

	w.Header().Set("Content-Type", "application/json")

	// statusCode lets tests request a specific upstream response status.
	if codeStr := r.URL.Query().Get("statusCode"); codeStr != "" {
		if code, err := strconv.Atoi(codeStr); err == nil && code >= 100 && code <= 999 {
			w.WriteHeader(code)
		}
	}

	if err := writeJSON(w, info); err != nil {
		return
	}
}

func (s *Service) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := writeJSON(w, map[string]string{"status": "healthy"}); err != nil {
		return
	}
}

// delay waits for the requested duration, then reflects the request.
func (s *Service) delay(w http.ResponseWriter, r *http.Request) {
	seconds, err := strconv.ParseFloat(r.PathValue("seconds"), 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		http.Error(w, "delay must be a non-negative number of seconds", http.StatusBadRequest)
		return
	}
	if seconds > 30 {
		seconds = 30
	}

	select {
	case <-time.After(time.Duration(seconds * float64(time.Second))):
	case <-r.Context().Done():
		return
	}
	s.reflect(w, r)
}

// whoami identifies the sandbox upstream used by routing tests.
func (s *Service) whoami(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := writeJSON(w, map[string]string{
		"environment": "sandbox",
		"source":      "sample-backend",
		"path":        "/sandbox/whoami",
	}); err != nil {
		return
	}
}

func writeJSON(w http.ResponseWriter, v any) error {
	return json.NewEncoder(w).Encode(v)
}

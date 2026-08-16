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

// Package echo provides HTTP reflection endpoints for integration tests.
package echo

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// Port is the container port used by the testbench.
const Port = 3002

// Service implements the echo testbench service.
type Service struct{}

// New returns a new echo service.
func New() *Service { return &Service{} }

// Name returns the service registration name.
func (s *Service) Name() string { return "echo" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// reflection is the response shape returned by the reflection endpoints.
type reflection struct {
	Method  string         `json:"method"`
	URL     string         `json:"url"`
	Path    string         `json:"path"`
	Args    map[string]any `json:"args"`
	Headers map[string]any `json:"headers"`
	// Data is always included, including for empty request bodies.
	Data string `json:"data"`
	JSON any    `json:"json,omitempty"`
}

// Handler returns the reflection and static response endpoints.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/status/", s.status)
	mux.HandleFunc("/json", s.staticJSON)
	mux.HandleFunc("/gzip", s.gzipped)
	mux.HandleFunc("/", s.reflect)
	return mux
}

func (s *Service) reflect(w http.ResponseWriter, r *http.Request) {
	s.serveReflection(w, r, false)
}

// reflection builds the echoed payload, shared by the plain and gzip handlers so the two cannot
// drift into reporting different things about the same request.
func (s *Service) reflection(r *http.Request, body []byte) reflection {
	args := map[string]any{}
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			args[k] = echoValues(v)
		}
	}
	headers := map[string]any{}
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = echoValues(v)
		}
	}
	// net/http stores the Host header separately from r.Header.
	if r.Host != "" {
		headers["Host"] = r.Host
	}

	out := reflection{
		Method:  r.Method,
		URL:     r.URL.String(),
		Path:    r.URL.Path,
		Args:    args,
		Headers: headers,
	}
	if len(body) > 0 {
		out.Data = string(body)
		// Keep the parsed form when the request body is valid JSON.
		var parsed any
		if json.Unmarshal(body, &parsed) == nil {
			out.JSON = parsed
		}
	}
	return out
}

// status returns the code named in the path, as /status/{code}.
func (s *Service) status(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimPrefix(r.URL.Path, "/status/")
	code, err := strconv.Atoi(raw)
	if err != nil || code < 200 || code > 599 {
		http.Error(w, "status code must be 200-599", http.StatusBadRequest)
		return
	}
	w.WriteHeader(code)
}

// gzipped returns the same reflection as / with gzip encoding.
func (s *Service) gzipped(w http.ResponseWriter, r *http.Request) {
	s.serveReflection(w, r, true)
}

func (s *Service) staticJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"slideshow":{"author":"testbench","title":"Sample Slide Show"}}`)); err != nil {
		log.Printf("echo: failed to write static response: %v", err)
	}
}

func (s *Service) serveReflection(w http.ResponseWriter, r *http.Request, compressed bool) {
	body, err := readBody(r)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	payload, err := json.Marshal(s.reflection(r, body))
	if err != nil {
		log.Printf("echo: failed to encode response: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	payload = append(payload, '\n')

	w.Header().Set("Content-Type", "application/json")
	if !compressed {
		if _, err := w.Write(payload); err != nil {
			log.Printf("echo: failed to write response: %v", err)
		}
		return
	}

	w.Header().Set("Content-Encoding", "gzip")
	zw := gzip.NewWriter(w)
	if _, err := zw.Write(payload); err != nil {
		log.Printf("echo: failed to write gzip response: %v", err)
	}
	if err := zw.Close(); err != nil {
		log.Printf("echo: failed to close gzip response: %v", err)
	}
}

func readBody(r *http.Request) ([]byte, error) {
	body, readErr := io.ReadAll(r.Body)
	closeErr := r.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	return body, closeErr
}

func echoValues(values []string) any {
	if len(values) == 1 {
		return values[0]
	}
	return append([]string(nil), values...)
}

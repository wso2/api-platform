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

// Package capture provides a partitioned upstream that remembers the last request it received
// per path. It exists for policies that rewrite a request on the way in and rewrite the
// response back on the way out (masking, redaction): the client-visible response can never show
// what the upstream actually received, because the response-side rewrite has already reversed
// it by the time a test observes that response. This service lets a test ask the upstream
// directly, in a second call, independent of any response rewriting.
package capture

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3010

// requestInfo is the shape returned by both the reflecting upstream and the capture lookup.
type requestInfo struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Query   string              `json:"query,omitempty"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body,omitempty"`
}

// Service implements testbench.Service and testbench.Partitioned.
type Service struct {
	mu         sync.RWMutex
	partitions map[string]*partition
}

type partition struct {
	mu       sync.RWMutex
	captured map[string]requestInfo
}

// New returns a new capture service.
func New() *Service { return &Service{partitions: map[string]*partition{}} }

// Name returns the service registration name.
func (s *Service) Name() string { return "capture" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return true }

// PartitionKey returns the partitioning strategy used by this stateful service.
func (s *Service) PartitionKey() string { return testbench.PartitionByBlock }

// Handler serves the partitioned routes.
func (s *Service) Handler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET /test/captured", s.scoped(s.readCaptured))
	routes.HandleFunc("POST /test/reset", s.scoped(s.reset))
	routes.HandleFunc("GET /test/health", s.scoped(s.health))
	routes.HandleFunc("/", s.scoped(s.reflect))

	return testbench.PartitionRouter(routes)
}

// scoped adapts a partition-aware handler to http.HandlerFunc.
func (s *Service) scoped(fn func(string, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key, ok := testbench.PartitionKeyFromContext(r.Context())
		if !ok || key == "" {
			http.Error(w, "capture: internal error: a request reached a handler with no partition",
				http.StatusInternalServerError)
			return
		}
		fn(key, w, r)
	}
}

// reflect records the incoming request under its own path, then echoes it back as JSON — the
// same response an ordinary caller sees. A later call to readCaptured for the same path returns
// exactly what was recorded here, regardless of what a response-side policy does to this
// response afterward.
func (s *Service) reflect(key string, w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()

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

	s.record(key, r.URL.Path, info)

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, info)
}

func (s *Service) record(key, path string, info requestInfo) {
	p := s.getPartition(key)
	p.mu.Lock()
	p.captured[path] = info
	p.mu.Unlock()
}

func (s *Service) getPartition(key string) *partition {
	s.mu.RLock()
	p := s.partitions[key]
	s.mu.RUnlock()
	if p != nil {
		return p
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if p = s.partitions[key]; p == nil {
		p = &partition{captured: map[string]requestInfo{}}
		s.partitions[key] = p
	}
	return p
}

// readCaptured is GET /<block>/test/captured?path=<path>, returning what this service actually
// received for that path. Responds 204 when nothing has been captured for it yet.
func (s *Service) readCaptured(key string, w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, `capture: query parameter "path" is required`, http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	p := s.partitions[key]
	s.mu.RUnlock()
	if p == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	p.mu.RLock()
	info, ok := p.captured[path]
	p.mu.RUnlock()
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, info)
}

// reset clears the captured requests for one block.
func (s *Service) reset(key string, w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	delete(s.partitions, key)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]string{"status": "reset"})
}

// health returns the partitioned service health status.
func (s *Service) health(_ string, w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]string{"status": "ok", "service": "capture"})
}

func writeJSON(w http.ResponseWriter, payload any) {
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("capture: failed to encode response: %v", err)
	}
}

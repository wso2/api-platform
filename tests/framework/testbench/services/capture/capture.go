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

package capture

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3010

const maxCaptureBodyBytes = 10 << 20

const defaultMaxCapturedPaths = 1024

const maxPartitionBodyBytes = 32 << 20

// maxScriptDocumentBytes bounds one PUT /test/responses document.
const maxScriptDocumentBytes = 1 << 20

// controlPathPrefix is where the service's own control routes live; they cannot be scripted.
const controlPathPrefix = "/test/"

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
	mu               sync.RWMutex
	partitions       map[string]*partition
	maxCapturedPaths int
}

type partition struct {
	mu          sync.RWMutex
	captured    map[string]requestInfo
	bytes       int
	scripts     map[string]scriptedResponse
	scriptBytes int
}

// scriptedResponse is the answer stored for one path by PUT /test/responses.
type scriptedResponse struct {
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
	Body        string `json:"body"`
}

// New returns a new capture service.
func New() *Service { return NewWithRetentionLimit(defaultMaxCapturedPaths) }

// NewWithRetentionLimit returns a capture service with a per-partition limit on distinct
// paths. A non-positive limit uses the safe default.
func NewWithRetentionLimit(limit int) *Service {
	if limit <= 0 {
		limit = defaultMaxCapturedPaths
	}
	return &Service{partitions: map[string]*partition{}, maxCapturedPaths: limit}
}

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
	routes.HandleFunc("PUT /test/responses", s.scoped(s.storeResponse))
	routes.HandleFunc("/", s.scoped(s.reflect))

	return testbench.NormalizeMethod(testbench.PartitionRouter(routes))
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

// reflect records the incoming request under its own path, then answers with the path's
// scripted response or, when there is none, echoes the record back as JSON. A later call to
// readCaptured for the same path returns exactly what was recorded here, regardless of what a
// response-side policy does to this response afterward.
func (s *Service) reflect(key string, w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxCaptureBodyBytes+1))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "capture: reading request body failed", http.StatusBadRequest)
		return
	}
	if len(body) > maxCaptureBodyBytes {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

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

	if !s.record(key, r.URL.Path, info) {
		http.Error(w, "capture: retained body budget exhausted for this block", http.StatusInsufficientStorage)
		return
	}

	if script, ok := s.script(key, r.URL.Path); ok {
		writeScripted(w, script)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, info)
}

// storeResponse is PUT /<block>/test/responses?path=<path>: it stores the answer every later
// request to <path> receives, replacing an earlier script for the same path.
func (s *Service) storeResponse(key string, w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if err := validScriptPath(path); err != nil {
		http.Error(w, "capture: "+err.Error(), http.StatusBadRequest)
		return
	}
	var script scriptedResponse
	if err := decodeScript(w, r, &script); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "capture: script too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "capture: invalid script: "+err.Error(), http.StatusBadRequest)
		return
	}
	if script.Status == 0 {
		script.Status = http.StatusOK
	}
	if err := validScript(script); err != nil {
		http.Error(w, "capture: invalid script: "+err.Error(), http.StatusBadRequest)
		return
	}
	if !s.storeScript(key, path, script) {
		http.Error(w, "capture: script budget exhausted for this block", http.StatusInsufficientStorage)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]any{"path": path, "status": script.Status})
}

// storeScript reports whether the partition's script limits accepted the script.
func (s *Service) storeScript(key, path string, script scriptedResponse) bool {
	p := s.getPartition(key)
	p.mu.Lock()
	defer p.mu.Unlock()
	existing, exists := p.scripts[path]
	if !exists {
		limit := s.maxCapturedPaths
		if limit <= 0 {
			limit = defaultMaxCapturedPaths
		}
		if len(p.scripts) >= limit {
			return false
		}
	}
	retained := p.scriptBytes - len(existing.Body) + len(script.Body)
	if retained > maxPartitionBodyBytes {
		return false
	}
	p.scriptBytes = retained
	p.scripts[path] = script
	return true
}

// script returns the answer scripted for path in the block, if any.
func (s *Service) script(key, path string) (scriptedResponse, bool) {
	s.mu.RLock()
	p := s.partitions[key]
	s.mu.RUnlock()
	if p == nil {
		return scriptedResponse{}, false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	script, ok := p.scripts[path]
	return script, ok
}

// writeScripted writes a scripted answer verbatim. An empty contentType sends no Content-Type,
// rather than one net/http would guess from the body.
func writeScripted(w http.ResponseWriter, script scriptedResponse) {
	if script.ContentType != "" {
		w.Header().Set("Content-Type", script.ContentType)
	} else {
		w.Header()["Content-Type"] = nil
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(script.Body)))
	w.WriteHeader(script.Status)
	if _, err := io.WriteString(w, script.Body); err != nil {
		log.Printf("capture: failed to write scripted response: %v", err)
	}
}

func validScriptPath(path string) error {
	switch {
	case path == "":
		return fmt.Errorf(`query parameter "path" is required`)
	case !strings.HasPrefix(path, "/"):
		return fmt.Errorf(`path must start with "/"`)
	case path == strings.TrimSuffix(controlPathPrefix, "/") || strings.HasPrefix(path, controlPathPrefix):
		return fmt.Errorf("path %q is a control route and cannot be scripted", path)
	}
	return nil
}

func validScript(script scriptedResponse) error {
	if script.Status < 200 || script.Status > 599 {
		return fmt.Errorf("status must be 200-599, got %d", script.Status)
	}
	if script.Body != "" && (script.Status == http.StatusNoContent || script.Status == http.StatusNotModified) {
		return fmt.Errorf("status %d cannot carry a body", script.Status)
	}
	if strings.ContainsAny(script.ContentType, "\r\n") {
		return fmt.Errorf("contentType must be a single header value")
	}
	return nil
}

// decodeScript reads exactly one JSON object, refusing unknown fields.
func decodeScript(w http.ResponseWriter, r *http.Request, dst *scriptedResponse) error {
	body := http.MaxBytesReader(w, r.Body, maxScriptDocumentBytes)
	defer func() { _ = body.Close() }()
	decoder := json.NewDecoder(body)
	var document json.RawMessage
	if err := decoder.Decode(&document); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return err
		}
		return fmt.Errorf("the body must contain a single JSON document")
	}
	if !strings.HasPrefix(string(document), "{") {
		return fmt.Errorf("the body must be a JSON object")
	}
	fields := json.NewDecoder(strings.NewReader(string(document)))
	fields.DisallowUnknownFields()
	return fields.Decode(dst)
}

// record reports whether the partition's byte budget accepted the capture. A full path table
// is still ignored quietly, as it was before the budget existed.
func (s *Service) record(key, path string, info requestInfo) bool {
	p := s.getPartition(key)
	p.mu.Lock()
	defer p.mu.Unlock()
	existing, exists := p.captured[path]
	if !exists {
		limit := s.maxCapturedPaths
		if limit <= 0 {
			limit = defaultMaxCapturedPaths
		}
		if len(p.captured) >= limit {
			return true
		}
	}
	retained := p.bytes - len(existing.Body) + len(info.Body)
	if retained > maxPartitionBodyBytes {
		return false
	}
	p.bytes = retained
	p.captured[path] = info
	return true
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
		p = &partition{captured: map[string]requestInfo{}, scripts: map[string]scriptedResponse{}}
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

// reset clears the captured requests and scripted responses for one block.
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

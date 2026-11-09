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

package faultinjection

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3026

// maxScriptsPerPartition bounds the scripts one block can hold; a script is replaced, never
// deleted, so the bound covers every scenario a block runs.
const maxScriptsPerPartition = 4096

// maxStepsPerScript bounds one script, several times the longest retry sequence a policy makes.
const maxStepsPerScript = 16

// maxScriptIDLen bounds a script id path segment.
const maxScriptIDLen = 128

// maxScriptBodyBytes bounds a stored script document.
const maxScriptBodyBytes = 64 << 10

// maxDelay keeps a delayed request inside the testbench server's and the suite client's 30-second
// timeouts.
const maxDelay = 25 * time.Second

// forwardHost is the testbench's network alias, the host a direct caller of the target names.
const forwardHost = "testbench"

// probeTimeout bounds each check storeScript makes against forwardPort.
const probeTimeout = time.Second

// reservedScriptID is the route root of the script API, so it cannot name a script.
const reservedScriptID = "test"

// step is one scripted answer; exactly one field is set.
type step struct {
	Status  int `json:"status,omitempty"`
	DelayMs int `json:"delayMs,omitempty"`
}

// scriptDocument is the stored script, as a scenario sends it.
type scriptDocument struct {
	ForwardPort int    `json:"forwardPort"`
	Steps       []step `json:"steps"`
}

type script struct {
	forwardPort int
	steps       []step
	received    int
}

// scriptState is the shape both script routes return.
type scriptState struct {
	ID       string `json:"id"`
	Received int    `json:"received"`
}

// Service implements testbench.Service and testbench.Partitioned.
type Service struct {
	mu         sync.Mutex
	partitions map[string]map[string]*script
	// probe checks that a script's forwardPort can be forwarded to; tests may replace it.
	probe func(port int) error
}

// New returns a new fault-injection service.
func New() *Service {
	return &Service{partitions: map[string]map[string]*script{}, probe: probeForwardPort}
}

// Name returns the service registration name.
func (s *Service) Name() string { return "fault-injection" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports that the service retains scripts and request counts.
func (s *Service) Stateful() bool { return true }

// PartitionKey returns the partitioning strategy used by this stateful service.
func (s *Service) PartitionKey() string { return testbench.PartitionByBlock }

// Handler serves the partitioned routes.
func (s *Service) Handler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("PUT /test/scripts/{id}", s.scoped(s.storeScript))
	routes.HandleFunc("GET /test/scripts/{id}", s.scoped(s.readScript))
	routes.HandleFunc("/", s.scoped(s.inject))
	return testbench.NormalizeMethod(testbench.PartitionRouter(routes))
}

func (s *Service) scoped(fn func(string, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key, ok := testbench.PartitionKeyFromContext(r.Context())
		if !ok || key == "" {
			http.Error(w, "fault-injection: request has no partition", http.StatusInternalServerError)
			return
		}
		fn(key, w, r)
	}
}

// storeScript serves PUT /test/scripts/{id}, replacing any script stored for the id.
func (s *Service) storeScript(key string, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validScriptID(id); err != nil {
		http.Error(w, "fault-injection: "+err.Error(), http.StatusBadRequest)
		return
	}
	var doc scriptDocument
	if err := decodeScript(w, r, &doc); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "fault-injection: script too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "fault-injection: invalid script: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := validScript(doc); err != nil {
		http.Error(w, "fault-injection: invalid script: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.probe(doc.ForwardPort); err != nil {
		http.Error(w, "fault-injection: invalid script: "+err.Error(), http.StatusBadRequest)
		return
	}
	if !s.store(key, id, doc) {
		http.Error(w, "fault-injection: script budget exhausted for this block", http.StatusInsufficientStorage)
		return
	}
	writeJSON(w, http.StatusOK, scriptState{ID: id})
}

// readScript serves GET /test/scripts/{id}.
func (s *Service) readScript(key string, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := validScriptID(id); err != nil {
		http.Error(w, "fault-injection: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	sc := s.partitions[key][id]
	var received int
	if sc != nil {
		received = sc.received
	}
	s.mu.Unlock()
	if sc == nil {
		http.Error(w, fmt.Sprintf("fault-injection: no script for %q in this block", id), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, scriptState{ID: id, Received: received})
}

// inject serves /<id>/<path>: it answers the script's next step, or forwards once none is left.
func (s *Service) inject(key string, w http.ResponseWriter, r *http.Request) {
	id, path := splitScriptPath(r.URL.Path)
	if err := validScriptID(id); err != nil {
		http.Error(w, "fault-injection: "+err.Error(), http.StatusBadRequest)
		return
	}
	next, forwardPort, ok := s.take(key, id)
	if !ok {
		http.Error(w, fmt.Sprintf("fault-injection: no script for %q in this block", id), http.StatusNotFound)
		return
	}
	if next.Status != 0 {
		http.Error(w, fmt.Sprintf("fault-injection: scripted status %d", next.Status), next.Status)
		return
	}
	if next.DelayMs > 0 && !wait(r.Context(), time.Duration(next.DelayMs)*time.Millisecond) {
		return
	}
	forward(w, r, forwardPort, path)
}

func (s *Service) store(key, id string, doc scriptDocument) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	scripts := s.partitions[key]
	if scripts == nil {
		scripts = map[string]*script{}
		s.partitions[key] = scripts
	}
	if _, exists := scripts[id]; !exists && len(scripts) >= maxScriptsPerPartition {
		return false
	}
	scripts[id] = &script{forwardPort: doc.ForwardPort, steps: append([]step(nil), doc.Steps...)}
	return true
}

// take counts a request against the id's script and returns the step it answers with; a zero
// step means forward.
func (s *Service) take(key, id string) (step, int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc := s.partitions[key][id]
	if sc == nil {
		return step{}, 0, false
	}
	sc.received++
	if sc.received > len(sc.steps) {
		return step{}, sc.forwardPort, true
	}
	return sc.steps[sc.received-1], sc.forwardPort, true
}

// forward sends the request to the testbench service on port, at path.
func forward(w http.ResponseWriter, r *http.Request, port int, path string) {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Scheme = "http"
			pr.Out.URL.Host = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
			pr.Out.URL.Path = path
			pr.Out.URL.RawPath = ""
			pr.Out.Host = net.JoinHostPort(forwardHost, strconv.Itoa(port))
			// ReverseProxy strips only Forwarded and X-Forwarded-For, -Host and -Proto, so drop
			// the rest of the family (X-Forwarded-Port, X-Forwarded-Prefix and others) here.
			for name := range pr.Out.Header {
				if strings.HasPrefix(name, "X-Forwarded-") {
					pr.Out.Header.Del(name)
				}
			}
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			log.Printf("fault-injection: forwarding to port %d failed: %v", port, err)
			http.Error(w, "fault-injection: forwarding failed", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}

// probeForwardPort checks that a testbench service answers plain HTTP on port, as forward needs.
// A port where nothing listens, or one whose service completes a TLS handshake, is refused, so a
// mistake shows when the script is stored rather than as a 502 on every forwarded request. A
// plain HTTP server rejects the handshake bytes before any handler runs, so no service sees the
// probe.
func probeForwardPort(port int) error {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, probeTimeout)
	if err != nil {
		return fmt.Errorf("nothing listens on forwardPort %d", port)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(probeTimeout)); err != nil {
		return fmt.Errorf("probing forwardPort %d: %w", port, err)
	}
	// The probe only asks whether the port speaks TLS, so the certificate does not matter.
	if tls.Client(conn, &tls.Config{InsecureSkipVerify: true}).Handshake() == nil { //nolint:gosec // probe only
		return fmt.Errorf("forwardPort %d serves TLS; fault-injection forwards plain HTTP only", port)
	}
	return nil
}

// splitScriptPath splits /<id>/<path> into the id and the path to forward.
func splitScriptPath(p string) (id, path string) {
	id, rest, _ := strings.Cut(strings.TrimPrefix(p, "/"), "/")
	return id, "/" + rest
}

func validScriptID(id string) error {
	if id == "" {
		return fmt.Errorf("script id is required")
	}
	if id == reservedScriptID {
		return fmt.Errorf("script id %q is reserved", id)
	}
	if len(id) > maxScriptIDLen {
		return fmt.Errorf("script id is longer than %d characters", maxScriptIDLen)
	}
	for _, c := range id {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return fmt.Errorf("script id %q may contain only letters, digits, dashes and underscores", id)
		}
	}
	return nil
}

func validScript(doc scriptDocument) error {
	if doc.ForwardPort < 1 || doc.ForwardPort > 65535 {
		return fmt.Errorf("forwardPort must be a port from 1 to 65535")
	}
	if doc.ForwardPort == Port {
		return fmt.Errorf("forwardPort cannot be this service's own port %d", Port)
	}
	if len(doc.Steps) > maxStepsPerScript {
		return fmt.Errorf("a script holds at most %d steps", maxStepsPerScript)
	}
	for i, st := range doc.Steps {
		switch {
		case (st.Status == 0) == (st.DelayMs == 0):
			return fmt.Errorf("step %d must set exactly one of status and delayMs", i)
		case st.Status != 0 && (st.Status < 400 || st.Status > 599):
			return fmt.Errorf("step %d status must be from 400 to 599", i)
		// Compare milliseconds: converting first would overflow for a huge delayMs.
		case st.DelayMs < 0 || int64(st.DelayMs) > maxDelay.Milliseconds():
			return fmt.Errorf("step %d delayMs must be from 1 to %d", i, maxDelay.Milliseconds())
		}
	}
	return nil
}

func decodeScript(w http.ResponseWriter, r *http.Request, dst *scriptDocument) error {
	body := http.MaxBytesReader(w, r.Body, maxScriptBodyBytes)
	defer func() { _ = body.Close() }()
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return err
		}
		return fmt.Errorf("the body must contain a single JSON document")
	}
	return nil
}

// wait blocks for delay unless the request ends first, reporting whether the delay elapsed.
func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("fault-injection: failed to encode response: %v", err)
	}
}

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

// Package agentcard provides a block-partitioned A2A Agent Card upstream with deterministic
// failure modes and a per-scope request counter.
//
// The control plane fetches a passthrough Agent proxy's public card from its upstream and caches
// the result, so the property worth asserting is how many times the upstream was actually
// contacted. A scenario addresses the service as
//
//	http://testbench:3013/<block>/<scope>/<mode>
//
// where <scope> is a scenario-unique name and <mode> selects the upstream's behaviour. The control
// plane appends the well-known card path, and every card request increments the scope's counter,
// which GET /<block>/test/requests?scope=<scope> reports.
package agentcard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3013

// CredentialHeader is the header the auth mode requires.
const CredentialHeader = "X-Card-Key"

// maxScopesPerPartition bounds the counters one block can hold.
const maxScopesPerPartition = 4096

// maxScopeLen bounds a scope path segment.
const maxScopeLen = 64

// defaultSlowDelay exceeds the control plane's 10-second Agent Card fetch budget, so a slow
// upstream is a timeout rather than a late success.
const defaultSlowDelay = 15 * time.Second

// Card modes, the second path segment.
const (
	// ModeOK serves the scope's card.
	ModeOK = "ok"
	// ModeAlternate serves a different card for the same scope, so a changed upstream is
	// distinguishable from a cached one.
	ModeAlternate = "alternate"
	// ModeAuth serves the card only when CredentialHeader carries Credential(scope), and answers
	// 401 otherwise.
	ModeAuth = "auth"
	// ModeMalformed answers 200 with a body that is not a JSON document.
	ModeMalformed = "malformed"
	// ModeSlow answers only after the control plane's fetch budget has elapsed.
	ModeSlow = "slow"
	// ModeServerError answers 500.
	ModeServerError = "status500"
)

// Credential returns the credential ModeAuth expects for a scope.
func Credential(scope string) string { return "card-key-" + scope }

// Service implements testbench.Service and testbench.Partitioned.
type Service struct {
	mu         sync.Mutex
	partitions map[string]map[string]int
	slowDelay  time.Duration
}

// New returns a new Agent Card service.
func New() *Service { return newWithSlowDelay(defaultSlowDelay) }

func newWithSlowDelay(delay time.Duration) *Service {
	return &Service{partitions: map[string]map[string]int{}, slowDelay: delay}
}

// Name returns the service registration name.
func (s *Service) Name() string { return "agentcard" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports that the service retains request counts.
func (s *Service) Stateful() bool { return true }

// PartitionKey returns the partitioning strategy used by this stateful service.
func (s *Service) PartitionKey() string { return testbench.PartitionByBlock }

// Handler serves the partitioned routes.
func (s *Service) Handler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET /test/requests", s.scoped(s.requests))
	routes.HandleFunc("GET /test/health", s.scoped(s.health))
	routes.HandleFunc("GET /", s.scoped(s.card))
	return testbench.NormalizeMethod(testbench.PartitionRouter(routes))
}

func (s *Service) scoped(fn func(string, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key, ok := testbench.PartitionKeyFromContext(r.Context())
		if !ok || key == "" {
			http.Error(w, "agentcard: request has no partition", http.StatusInternalServerError)
			return
		}
		fn(key, w, r)
	}
}

// card serves GET /<scope>/<mode>/<anything>.json.
func (s *Service) card(key string, w http.ResponseWriter, r *http.Request) {
	segments := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(segments) < 3 || !strings.HasSuffix(r.URL.Path, ".json") {
		http.Error(w, "agentcard: expected /<scope>/<mode>/<card>.json", http.StatusNotFound)
		return
	}
	scope, mode := segments[0], segments[1]
	if err := validScope(scope); err != nil {
		http.Error(w, "agentcard: "+err.Error(), http.StatusBadRequest)
		return
	}
	if !s.count(key, scope) {
		http.Error(w, "agentcard: request counter budget exhausted for this block", http.StatusInsufficientStorage)
		return
	}

	switch mode {
	case ModeOK:
		writeJSON(w, http.StatusOK, Card(scope, "Card "+scope))
	case ModeAlternate:
		writeJSON(w, http.StatusOK, Card(scope, "Alternate card "+scope))
	case ModeAuth:
		if r.Header.Get(CredentialHeader) != Credential(scope) {
			http.Error(w, "agentcard: credential rejected", http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, Card(scope, "Protected source card "+scope))
	case ModeMalformed:
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>not an agent card</body></html>"))
	case ModeSlow:
		if !wait(r.Context(), s.slowDelay) {
			return
		}
		writeJSON(w, http.StatusOK, Card(scope, "Slow card "+scope))
	case ModeServerError:
		http.Error(w, "agentcard: upstream failure", http.StatusInternalServerError)
	default:
		http.Error(w, fmt.Sprintf("agentcard: unknown mode %q", mode), http.StatusNotFound)
	}
}

// Card returns the deterministic Agent Card served for a scope under the given name. It carries
// every key a managed card requires, so the same document can also be saved as managed content.
func Card(scope, name string) map[string]any {
	return map[string]any{
		"name":        name,
		"description": "Agent Card fixture for " + scope,
		"version":     "1.0.0",
		"supportedInterfaces": []map[string]any{
			{"protocolBinding": "JSONRPC", "protocolVersion": "1.0", "url": "http://testbench:3013/" + scope},
		},
		"capabilities":       map[string]any{"streaming": false},
		"defaultInputModes":  []string{"text/plain"},
		"defaultOutputModes": []string{"text/plain"},
		"skills": []map[string]any{
			{"id": "card-fixture", "name": "Card fixture", "description": "Fixture skill", "tags": []string{"fixture"}},
		},
	}
}

func (s *Service) count(key, scope string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := s.partitions[key]
	if counts == nil {
		counts = map[string]int{}
		s.partitions[key] = counts
	}
	if _, seen := counts[scope]; !seen && len(counts) >= maxScopesPerPartition {
		return false
	}
	counts[scope]++
	return true
}

// requests reports GET /test/requests?scope=<scope> as {"scope": ..., "count": n}.
func (s *Service) requests(key string, w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	if err := validScope(scope); err != nil {
		http.Error(w, "agentcard: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	count := s.partitions[key][scope]
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"scope": scope, "count": count})
}

func (s *Service) health(_ string, w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "agentcard"})
}

func validScope(scope string) error {
	if scope == "" {
		return fmt.Errorf("scope is required")
	}
	if len(scope) > maxScopeLen {
		return fmt.Errorf("scope is longer than %d characters", maxScopeLen)
	}
	for _, c := range scope {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return fmt.Errorf("scope %q may contain only lowercase letters, digits and dashes", scope)
		}
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
		log.Printf("agentcard: failed to encode response: %v", err)
	}
}

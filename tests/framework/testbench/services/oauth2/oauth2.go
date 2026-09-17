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
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package oauth2 provides the partitioned OAuth2 identity provider used by gateway tests.
package oauth2

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3011

const (
	clientID     = "test-client"
	clientSecret = "test-secret"
	resourceUser = "resource-owner"
	resourcePass = "hunter2"
	defaultTTL   = 300
	maxBodyBytes = 1 << 20
	maxHistory   = 1024
)

type tokenRequest struct {
	ClientID  string            `json:"clientId"`
	AuthStyle string            `json:"authStyle"`
	Scope     string            `json:"scope,omitempty"`
	Outcome   string            `json:"outcome"`
	Token     string            `json:"token,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type partition struct {
	mu          sync.Mutex
	sequence    int
	failCounter int
	history     []tokenRequest
}

// Service implements testbench.Service and testbench.Partitioned.
type Service struct {
	mu         sync.Mutex
	partitions map[string]*partition
}

// New returns a new partitioned OAuth2 service.
func New() *Service { return &Service{partitions: map[string]*partition{}} }

// Name returns the service registration name.
func (s *Service) Name() string { return "oauth2" }

// Port returns the service port.
func (s *Service) Port() int { return Port }

// Stateful reports that the service records token requests.
func (s *Service) Stateful() bool { return true }

// PartitionKey returns the partitioning strategy used by this service.
func (s *Service) PartitionKey() string { return testbench.PartitionByBlock }

// Handler serves token and diagnostic endpoints.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth2/token", s.token)
	mux.Handle("GET /debug/stats", debugAuth(http.HandlerFunc(s.stats)))
	mux.Handle("POST /debug/reset", debugAuth(http.HandlerFunc(s.reset)))
	mux.HandleFunc("GET /healthz", health)
	return testbench.NormalizeMethod(testbench.PartitionRouter(mux))
}

func debugAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, secret, ok := r.BasicAuth()
		if !ok || id != clientID || secret != clientSecret {
			writeUnauthorized(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Service) scoped(r *http.Request) (*partition, bool) {
	key, ok := testbench.PartitionKeyFromContext(r.Context())
	if !ok || key == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.partitions[key]; p != nil {
		return p, true
	}
	p := &partition{}
	s.partitions[key] = p
	return p, true
}

func (s *Service) token(w http.ResponseWriter, r *http.Request) {
	p, ok := s.scoped(r)
	if !ok {
		http.Error(w, "oauth2: missing partition", http.StatusInternalServerError)
		return
	}
	body := r.Body
	defer func() { _ = body.Close() }()
	r.Body = http.MaxBytesReader(w, io.NopCloser(io.LimitReader(body, maxBodyBytes+1)), maxBodyBytes)
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", "failed to parse form body")
		return
	}
	if raw := r.FormValue("delayMs"); raw != "" {
		if delay, err := strconv.Atoi(raw); err == nil && delay > 0 {
			timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-r.Context().Done():
				return
			}
		}
	}
	id, secret, style, err := credentials(r)
	scope := r.PostForm.Get("scope")
	if err != nil {
		s.record(p, tokenRequest{ClientID: id, AuthStyle: style, Scope: scope, Outcome: "invalid_client", Headers: customHeaders(r)})
		writeUnauthorized(w)
		return
	}
	grant := r.PostForm.Get("grant_type")
	if grant != "client_credentials" && grant != "password" {
		writeError(w, http.StatusBadRequest, "unsupported_grant_type", "unsupported grant type")
		return
	}
	if grant == "password" && (r.PostForm.Get("username") != resourceUser || r.PostForm.Get("password") != resourcePass) {
		writeUnauthorized(w)
		return
	}
	if id == "broken-client" {
		s.record(p, tokenRequest{ClientID: id, AuthStyle: style, Scope: scope, Outcome: "server_error", Headers: customHeaders(r)})
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if id == "malformed-client" {
		s.record(p, tokenRequest{ClientID: id, AuthStyle: style, Scope: scope, Outcome: "malformed", Headers: customHeaders(r)})
		writeJSON(w, map[string]any{"token_type": "Bearer", "expires_in": defaultTTL})
		return
	}
	if id != clientID || secret != clientSecret {
		s.record(p, tokenRequest{ClientID: id, AuthStyle: style, Scope: scope, Outcome: "invalid_client", Headers: customHeaders(r)})
		writeUnauthorized(w)
		return
	}
	if v, _ := strconv.Atoi(r.FormValue("failFirstN")); v > 0 {
		p.mu.Lock()
		fail := p.failCounter < v
		if fail {
			p.failCounter++
		}
		p.mu.Unlock()
		if fail {
			s.record(p, tokenRequest{ClientID: id, AuthStyle: style, Scope: scope, Outcome: "forced_failure", Headers: customHeaders(r)})
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}
	p.mu.Lock()
	p.sequence++
	seq := p.sequence
	p.mu.Unlock()
	token := fmt.Sprintf("mock-token-%d-issued-%d", seq, time.Now().UnixNano())
	s.record(p, tokenRequest{ClientID: id, AuthStyle: style, Scope: scope, Outcome: "issued", Token: token, Headers: customHeaders(r)})
	ttl := defaultTTL
	if raw := r.FormValue("ttl"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			ttl = parsed
		}
	}
	resp := map[string]any{"access_token": token, "token_type": "Bearer"}
	if r.FormValue("omitExpiresIn") != "true" {
		resp["expires_in"] = ttl
	}
	if scope != "" {
		resp["scope"] = scope
	}
	writeJSON(w, resp)
}

func credentials(r *http.Request) (string, string, string, error) {
	if user, pass, ok := r.BasicAuth(); ok {
		return user, pass, "basic", nil
	}
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Basic ") {
		if _, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(header, "Basic ")); err != nil {
			return "", "", "basic", fmt.Errorf("malformed Basic authorization header")
		}
	}
	id := r.PostForm.Get("client_id")
	if id == "" {
		return "", "", "post", fmt.Errorf("no client credentials presented")
	}
	return id, r.PostForm.Get("client_secret"), "post", nil
}

func customHeaders(r *http.Request) map[string]string {
	standard := map[string]bool{"Authorization": true, "Content-Type": true, "Content-Length": true, "Accept-Encoding": true, "User-Agent": true, "Host": true}
	out := map[string]string{}
	for k, values := range r.Header {
		if !standard[http.CanonicalHeaderKey(k)] && len(values) > 0 {
			out[k] = values[0]
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *Service) record(p *partition, request tokenRequest) {
	p.mu.Lock()
	p.history = append(p.history, request)
	if len(p.history) > maxHistory {
		p.history = p.history[len(p.history)-maxHistory:]
	}
	p.mu.Unlock()
}

func (s *Service) stats(w http.ResponseWriter, r *http.Request) {
	p, ok := s.scoped(r)
	if !ok {
		http.Error(w, "oauth2: missing partition", http.StatusInternalServerError)
		return
	}
	p.mu.Lock()
	history := append([]tokenRequest(nil), p.history...)
	p.mu.Unlock()
	writeJSON(w, map[string]any{"tokenRequestCount": len(history), "history": history})
}

func (s *Service) reset(w http.ResponseWriter, r *http.Request) {
	p, ok := s.scoped(r)
	if !ok {
		http.Error(w, "oauth2: missing partition", http.StatusInternalServerError)
		return
	}
	p.mu.Lock()
	p.sequence, p.failCounter, p.history = 0, 0, nil
	p.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func health(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": description})
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "unauthorized",
		"message": "Invalid or expired credentials.",
	})
}

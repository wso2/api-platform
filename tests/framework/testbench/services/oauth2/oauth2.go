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

	// grantTypeTokenExchange and grantTypeJWTBearer are the RFC 8693 / RFC
	// 7523 grant-type URNs this service accepts in addition to the two plain
	// RFC 6749 grant names (client_credentials, password).
	grantTypeTokenExchange = "urn:ietf:params:oauth:grant-type:token-exchange"
	grantTypeJWTBearer     = "urn:ietf:params:oauth:grant-type:jwt-bearer"

	// invalidSubjectToken and invalidAssertion are sentinel subject_token/
	// assertion values that deliberately fail the exchange with
	// invalid_grant - this service never validates these values
	// cryptographically, so a test that wants to exercise an
	// exchange-failure path needs some other way to signal "this one should
	// fail" than a real signature check.
	invalidSubjectToken = "invalid-subject-token"
	invalidAssertion    = "invalid-assertion"
)

// validTokenTypeURNs are the RFC 8693 token-type identifiers this service
// accepts for subject_token_type/requested_token_type.
var validTokenTypeURNs = map[string]bool{
	"urn:ietf:params:oauth:token-type:access_token": true,
	"urn:ietf:params:oauth:token-type:jwt":          true,
	"urn:ietf:params:oauth:token-type:id_token":     true,
}

type tokenRequest struct {
	ClientID  string `json:"clientId"`
	AuthStyle string `json:"authStyle"`
	// GrantType is the raw grant_type form value - the RFC 6749 names
	// ("client_credentials", "password") or one of the RFC 8693/7523 URNs.
	GrantType string `json:"grantType,omitempty"`
	Scope     string `json:"scope,omitempty"`
	Outcome   string `json:"outcome"`
	Token     string `json:"token,omitempty"`
	// SubjectTokenPreview is a masked (never raw) preview of the
	// token-exchange subject_token / jwt-bearer assertion.
	SubjectTokenPreview string            `json:"subjectTokenPreview,omitempty"`
	SubjectTokenType    string            `json:"subjectTokenType,omitempty"`
	RequestedTokenType  string            `json:"requestedTokenType,omitempty"`
	Audiences           []string          `json:"audiences,omitempty"`
	Resources           []string          `json:"resources,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"`
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
	mux.HandleFunc("GET /debug/stats", s.stats)
	mux.HandleFunc("POST /debug/reset", s.reset)
	mux.HandleFunc("GET /healthz", health)
	return testbench.NormalizeMethod(testbench.PartitionRouter(mux))
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
	grant := r.PostForm.Get("grant_type")
	rec := tokenRequest{ClientID: id, AuthStyle: style, GrantType: grant, Scope: scope, Headers: customHeaders(r)}
	if err != nil {
		rec.Outcome = "invalid_client"
		s.record(p, rec)
		writeUnauthorized(w)
		return
	}

	switch grant {
	case "client_credentials", "password", grantTypeTokenExchange, grantTypeJWTBearer:
		// supported - validated further below.
	default:
		writeError(w, http.StatusBadRequest, "unsupported_grant_type", "unsupported grant type")
		return
	}
	if grant == "password" && (r.PostForm.Get("username") != resourceUser || r.PostForm.Get("password") != resourcePass) {
		rec.Outcome = "invalid_client"
		s.record(p, rec)
		writeUnauthorized(w)
		return
	}

	// For token-exchange (RFC 8693), subject_token/subject_token_type are
	// required and must be well-formed; audience/resource/requested_token_type
	// are optional. For jwt-bearer (RFC 7523), assertion is required. Neither
	// credential is cryptographically verified - the sentinel values above
	// exist purely so a test can force the exchange to fail.
	if grant == grantTypeTokenExchange {
		subjectToken := r.PostForm.Get("subject_token")
		subjectTokenType := r.PostForm.Get("subject_token_type")
		requestedTokenType := r.PostForm.Get("requested_token_type")
		rec.SubjectTokenPreview = maskSecret(subjectToken)
		rec.SubjectTokenType = subjectTokenType
		rec.RequestedTokenType = requestedTokenType
		rec.Audiences = r.PostForm["audience"]
		rec.Resources = r.PostForm["resource"]

		if subjectToken == "" {
			rec.Outcome = "invalid_request"
			s.record(p, rec)
			writeError(w, http.StatusBadRequest, "invalid_request", "subject_token is required for the token-exchange grant")
			return
		}
		if !validTokenTypeURNs[subjectTokenType] {
			rec.Outcome = "invalid_request"
			s.record(p, rec)
			writeError(w, http.StatusBadRequest, "invalid_request", "subject_token_type must be a supported urn:ietf:params:oauth:token-type:* value")
			return
		}
		if requestedTokenType != "" && !validTokenTypeURNs[requestedTokenType] {
			rec.Outcome = "invalid_request"
			s.record(p, rec)
			writeError(w, http.StatusBadRequest, "invalid_request", "requested_token_type must be a supported urn:ietf:params:oauth:token-type:* value")
			return
		}
		if subjectToken == invalidSubjectToken {
			rec.Outcome = "invalid_grant"
			s.record(p, rec)
			writeError(w, http.StatusBadRequest, "invalid_grant", "subject_token could not be validated")
			return
		}
	}
	if grant == grantTypeJWTBearer {
		assertion := r.PostForm.Get("assertion")
		rec.SubjectTokenPreview = maskSecret(assertion)

		if assertion == "" {
			rec.Outcome = "invalid_request"
			s.record(p, rec)
			writeError(w, http.StatusBadRequest, "invalid_request", "assertion is required for the jwt-bearer grant")
			return
		}
		if assertion == invalidAssertion {
			rec.Outcome = "invalid_grant"
			s.record(p, rec)
			writeError(w, http.StatusBadRequest, "invalid_grant", "assertion could not be validated")
			return
		}
	}

	if id == "broken-client" {
		rec.Outcome = "server_error"
		s.record(p, rec)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if id == "malformed-client" {
		rec.Outcome = "malformed"
		s.record(p, rec)
		writeJSON(w, map[string]any{"token_type": "Bearer", "expires_in": defaultTTL})
		return
	}
	if id != clientID || secret != clientSecret {
		rec.Outcome = "invalid_client"
		s.record(p, rec)
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
			rec.Outcome = "forced_failure"
			s.record(p, rec)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}
	p.mu.Lock()
	p.sequence++
	seq := p.sequence
	p.mu.Unlock()
	token := fmt.Sprintf("mock-token-%d-issued-%d", seq, time.Now().UnixNano())
	rec.Outcome = "issued"
	rec.Token = token
	s.record(p, rec)
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
	// RFC 8693 §2.2.1 requires issued_token_type on a token-exchange response.
	if grant == grantTypeTokenExchange {
		if rec.RequestedTokenType != "" {
			resp["issued_token_type"] = rec.RequestedTokenType
		} else {
			resp["issued_token_type"] = "urn:ietf:params:oauth:token-type:access_token"
		}
	}
	writeJSON(w, resp)
}

// maskSecret keeps only enough of a credential to correlate debug output
// without leaking the value itself (see GO-AUTH-003).
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "[MASKED]"
	}
	return s[:4] + "..." + s[len(s)-4:]
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

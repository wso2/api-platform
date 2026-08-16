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

// Package jwks serves a JWKS and signs test tokens on demand.
package jwks

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// Port is the container port used by the testbench.
const Port = 3001

// defaultIssuer is used when a token request does not specify an issuer.
const defaultIssuer = "http://mock-jwks.default.svc.cluster.local:8080/token"

// Service implements the JWKS testbench service.
type Service struct {
	signer jose.Signer
	jwkSet jose.JSONWebKeySet
}

// New generates a signing key and the matching JWKS for one service instance.
func New() (*Service, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("jwks: generating key: %w", err)
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key-id"),
	)
	if err != nil {
		return nil, fmt.Errorf("jwks: creating signer: %w", err)
	}
	return &Service{
		signer: signer,
		jwkSet: jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key:       &key.PublicKey,
			KeyID:     "test-key-id",
			Algorithm: "RS256",
			Use:       "sig",
		}}},
	}, nil
}

// Name returns the service registration name.
func (s *Service) Name() string { return "jwks" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// Handler returns the JWKS and token handlers.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/jwks", s.serveJWKS)
	mux.HandleFunc("/.well-known/jwks.json", s.serveJWKS)
	mux.HandleFunc("/token", s.issueToken)
	return mux
}

func (s *Service) serveJWKS(w http.ResponseWriter, r *http.Request) {
	if !getOnly(w, r) {
		return
	}

	payload, err := json.Marshal(s.jwkSet)
	if err != nil {
		http.Error(w, "encoding JWKS", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(payload); err != nil {
		log.Printf("jwks: writing JWKS response: %v", err)
	}
}

// issueToken signs a token using the request's issuer, scope, and claim_* parameters.
func (s *Service) issueToken(w http.ResponseWriter, r *http.Request) {
	if !getOnly(w, r) {
		return
	}

	issuer := defaultIssuer
	if v := r.URL.Query().Get("issuer"); v != "" {
		issuer = v
	}
	scope := "default"
	if v := r.URL.Query().Get("scope"); v != "" {
		scope = v
	}

	extra := map[string]any{}
	for key, vals := range r.URL.Query() {
		if strings.HasPrefix(key, "claim_") && len(vals) > 0 {
			extra[strings.TrimPrefix(key, "claim_")] = vals[0]
		}
	}

	now := time.Now()
	claims := jwt.Claims{
		Subject:   "test-user",
		Issuer:    issuer,
		NotBefore: jwt.NewNumericDate(now),
		Expiry:    jwt.NewNumericDate(now.Add(time.Hour)),
		Audience:  jwt.Audience{"test-audience"},
	}

	builder := jwt.Signed(s.signer).Claims(claims).Claims(struct {
		Scope string `json:"scope"`
	}{Scope: scope})
	if len(extra) > 0 {
		builder = builder.Claims(extra)
	}

	raw, err := builder.Serialize()
	if err != nil {
		http.Error(w, fmt.Sprintf("signing token: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(raw)); err != nil {
		log.Printf("jwks: writing token response: %v", err)
	}
}

func getOnly(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

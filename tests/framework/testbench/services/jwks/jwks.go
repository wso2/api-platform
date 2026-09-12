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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Port is the container port used by the testbench.
const Port = 3001

// defaultIssuer is used when a token request does not specify an issuer.
const defaultIssuer = "http://mock-jwks.default.svc.cluster.local:8080/token"

// keyID identifies the service's signing key in both the JWKS and every issued token's
// header, so a verifier can look the key up by "kid".
const keyID = "test-key-id"

// jsonWebKey is the RFC 7517 JWK representation of an RSA public key.
type jsonWebKey struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// jsonWebKeySet is the RFC 7517 JWK Set document served at the JWKS endpoint.
type jsonWebKeySet struct {
	Keys []jsonWebKey `json:"keys"`
}

// Service implements the JWKS testbench service.
type Service struct {
	privateKey *rsa.PrivateKey
	jwkSet     jsonWebKeySet
}

// New generates a signing key and the matching JWKS for one service instance.
func New() (*Service, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("jwks: generating key: %w", err)
	}
	return &Service{
		privateKey: key,
		jwkSet:     jwkSetFor(&key.PublicKey),
	}, nil
}

// jwkSetFor encodes an RSA public key's modulus and exponent as the base64url values RFC
// 7517 requires for a JWK's "n" and "e" fields.
func jwkSetFor(pub *rsa.PublicKey) jsonWebKeySet {
	return jsonWebKeySet{Keys: []jsonWebKey{{
		Kty: "RSA",
		Use: "sig",
		Kid: keyID,
		Alg: "RS256",
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}}}
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
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
	}
	if expected := r.URL.Query().Get("expected_secret"); expected != "" {
		_, secret, ok := r.BasicAuth()
		if !ok || secret != expected {
			http.Error(w, "invalid client credentials", http.StatusUnauthorized)
			return
		}
	}

	issuer := defaultIssuer
	if v := r.URL.Query().Get("issuer"); v != "" {
		issuer = v
	} else if v := r.FormValue("issuer"); v != "" {
		issuer = v
	}
	scope := "default"
	if v := r.URL.Query().Get("scope"); v != "" {
		scope = v
	} else if v := r.FormValue("scope"); v != "" {
		scope = v
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"sub":   "test-user",
		"iss":   issuer,
		"nbf":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
		"aud":   []string{"test-audience"},
		"scope": scope,
	}
	for key, vals := range r.URL.Query() {
		if strings.HasPrefix(key, "claim_") && len(vals) > 0 {
			claims[strings.TrimPrefix(key, "claim_")] = vals[0]
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = keyID
	raw, err := token.SignedString(s.privateKey)
	if err != nil {
		log.Printf("jwks: signing token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		response, marshalErr := json.Marshal(map[string]any{
			"access_token": raw, "token_type": "Bearer", "expires_in": 3600, "scope": scope,
		})
		if marshalErr != nil {
			http.Error(w, "encoding token response", http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(response); err != nil {
			log.Printf("jwks: writing token response: %v", err)
		}
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(raw)); err != nil {
		log.Printf("jwks: writing token response: %v", err)
	}
}

func getOnly(w http.ResponseWriter, r *http.Request) bool {
	if strings.ToUpper(r.Method) == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

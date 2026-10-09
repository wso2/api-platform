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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package oidc provides an OpenID Connect identity provider for the API Portal's IDP mode.
//
// It serves HTTPS, because the portal only provisions organizations from claims that reached
// it over verified TLS, and it keeps no state. The caller of the authorization endpoint names
// the signed-in identity in the LoginHeader request header, standing in for the provider's own
// login session. The issued authorization code carries that identity, signed by the service,
// so the token endpoint needs no record of it and one shared instance serves every block.
package oidc

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

// Port is the container port used by the testbench.
const Port = 3015

// LoginHeader carries the signed-in identity on an authorization request: the base64url
// encoding of a JSON object whose members become the issued tokens' claims.
const LoginHeader = "X-Testbench-Login"

// EnvIssuer names the variable holding the issuer URL the service signs tokens with. When it
// is unset the issuer is derived from the request's host.
const EnvIssuer = "TESTBENCH_OIDC_ISSUER"

// EnvTLSCert and EnvTLSKey name the variables holding the PEM certificate and key the service
// serves HTTPS with. When both are unset the service generates a self-signed certificate.
const (
	EnvTLSCert = "TESTBENCH_OIDC_TLS_CERT"
	EnvTLSKey  = "TESTBENCH_OIDC_TLS_KEY"
)

// OrgClaim is the claim an authorization request's organization hint is matched against.
const OrgClaim = "org_id"

const (
	keyID          = "testbench-oidc"
	codeTTL        = 2 * time.Minute
	tokenTTL       = time.Hour
	maxBodyBytes   = 1 << 20
	maxLoginHeader = 16 << 10
)

// grant is what an authorization code carries between the two endpoints.
type grant struct {
	Claims        map[string]any `json:"claims"`
	ClientID      string         `json:"client_id"`
	RedirectURI   string         `json:"redirect_uri"`
	CodeChallenge string         `json:"code_challenge,omitempty"`
	Nonce         string         `json:"nonce,omitempty"`
	Expires       int64          `json:"exp"`
}

// Service implements testbench.Service and testbench.TLSService.
type Service struct {
	signingKey *rsa.PrivateKey
	codeKey    []byte
	tlsConfig  *tls.Config
	issuer     string
	now        func() time.Time
}

// New returns an identity provider serving HTTPS with the supplied PEM certificate and key,
// or with a generated self-signed certificate when both are empty. issuer is the issuer URL
// tokens are signed with; empty derives it from each request's host.
func New(certPEM, keyPEM []byte, issuer string) (*Service, error) {
	if len(certPEM) == 0 != (len(keyPEM) == 0) {
		return nil, errors.New("oidc: a TLS certificate and key must be supplied together")
	}
	if len(certPEM) == 0 {
		var err error
		if certPEM, keyPEM, err = selfSigned(); err != nil {
			return nil, err
		}
	}
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("oidc: loading the TLS key pair: %w", err)
	}
	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("oidc: generating the signing key: %w", err)
	}
	codeKey := make([]byte, 32)
	if _, err := rand.Read(codeKey); err != nil {
		return nil, fmt.Errorf("oidc: generating the code key: %w", err)
	}
	return &Service{
		signingKey: signingKey,
		codeKey:    codeKey,
		tlsConfig:  &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12},
		issuer:     strings.TrimSpace(issuer),
		now:        time.Now,
	}, nil
}

// FromEnvironment builds the service from EnvTLSCert, EnvTLSKey and EnvIssuer.
func FromEnvironment() (*Service, error) {
	return New([]byte(os.Getenv(EnvTLSCert)), []byte(os.Getenv(EnvTLSKey)), os.Getenv(EnvIssuer))
}

// Name returns the service registration name.
func (s *Service) Name() string { return "oidc" }

// Port returns the service port.
func (s *Service) Port() int { return Port }

// Stateful reports that the service keeps no state between requests.
func (s *Service) Stateful() bool { return false }

// TLSConfig returns the HTTPS configuration the service is served with.
func (s *Service) TLSConfig() *tls.Config { return s.tlsConfig }

// Handler serves the authorization, token, key-set and token-minting endpoints.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /oauth2/authorize", s.authorize)
	mux.HandleFunc("POST /oauth2/token", s.token)
	mux.HandleFunc("GET /oauth2/jwks", s.jwks)
	mux.HandleFunc("POST /mint", s.mint)
	return testbench.NormalizeMethod(mux)
}

// authorize answers an authorization request with a redirect carrying a code for the identity
// in LoginHeader. A prompt=none request with no identity, or whose organization hint names
// another organization than the identity's, is answered with login_required instead.
func (s *Service) authorize(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	redirect, err := url.Parse(query.Get("redirect_uri"))
	if err != nil || redirect.Scheme == "" || redirect.Host == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "redirect_uri must be an absolute URL")
		return
	}
	if query.Get("response_type") != "code" || query.Get("client_id") == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "a code request with a client_id is required")
		return
	}
	claims, loggedIn, err := loginClaims(r.Header.Get(LoginHeader))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	values := redirect.Query()
	if state := query.Get("state"); state != "" {
		values.Set("state", state)
	}
	if query.Get("prompt") == "none" && (!loggedIn || !matchesHint(claims, query.Get("org"))) {
		values.Set("error", "login_required")
		redirect.RawQuery = values.Encode()
		http.Redirect(w, r, redirect.String(), http.StatusFound)
		return
	}
	if !loggedIn {
		writeError(w, http.StatusUnauthorized, "login_required", "no identity was supplied in "+LoginHeader)
		return
	}
	if method := query.Get("code_challenge_method"); query.Get("code_challenge") != "" && method != "S256" {
		writeError(w, http.StatusBadRequest, "invalid_request", "only the S256 code challenge method is supported")
		return
	}
	code, err := s.sealGrant(grant{
		Claims:        claims,
		ClientID:      query.Get("client_id"),
		RedirectURI:   query.Get("redirect_uri"),
		CodeChallenge: query.Get("code_challenge"),
		Nonce:         query.Get("nonce"),
		Expires:       s.now().Add(codeTTL).Unix(),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "issuing the authorization code failed")
		return
	}
	values.Set("code", code)
	redirect.RawQuery = values.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

// token exchanges an authorization code for an ID token and an access token.
func (s *Service) token(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "the request body is not a form")
		return
	}
	if r.PostForm.Get("grant_type") != "authorization_code" {
		writeError(w, http.StatusBadRequest, "unsupported_grant_type", "only authorization_code is supported")
		return
	}
	issued, err := s.openGrant(r.PostForm.Get("code"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}
	clientID := r.PostForm.Get("client_id")
	if user, _, ok := r.BasicAuth(); ok {
		clientID, _ = url.QueryUnescape(user)
	}
	if clientID != issued.ClientID {
		writeError(w, http.StatusBadRequest, "invalid_grant", "the code was issued to another client")
		return
	}
	if r.PostForm.Get("redirect_uri") != issued.RedirectURI {
		writeError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri does not match the authorization request")
		return
	}
	if issued.CodeChallenge != "" && pkceChallenge(r.PostForm.Get("code_verifier")) != issued.CodeChallenge {
		writeError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}
	issuer := s.issuerFor(r)
	idClaims := withDefaults(issued.Claims, issuer, issued.ClientID, s.now())
	if issued.Nonce != "" {
		idClaims["nonce"] = issued.Nonce
	}
	if _, ok := idClaims["given_name"]; !ok {
		idClaims["given_name"] = idClaims["sub"]
	}
	accessClaims := withDefaults(issued.Claims, issuer, issued.ClientID, s.now())
	accessClaims["client_id"] = issued.ClientID
	if _, ok := accessClaims["scope"]; !ok {
		accessClaims["scope"] = "openid profile email"
	}
	idToken, err := s.sign(idClaims)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "signing the ID token failed")
		return
	}
	accessToken, err := s.sign(accessClaims)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "signing the access token failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken, "id_token": idToken,
		"token_type": "Bearer", "expires_in": int(tokenTTL.Seconds()),
	})
}

// jwks serves the public half of the signing key.
func (s *Service) jwks(w http.ResponseWriter, _ *http.Request) {
	public := s.signingKey.PublicKey
	writeJSON(w, http.StatusOK, map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": keyID,
		"n": base64.RawURLEncoding.EncodeToString(public.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(public.E)).Bytes()),
	}}})
}

// mint signs the JSON object in the request body as a token, adding the issuer, issue and
// expiry times unless the body sets them. It stands in for a token the provider issued to a
// client directly, such as one a bearer-token caller presents.
func (s *Service) mint(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "the request body could not be read")
		return
	}
	var claims map[string]any
	if err := json.Unmarshal(body, &claims); err != nil || claims == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "the request body must be a JSON object of claims")
		return
	}
	signed, err := s.sign(withDefaults(claims, s.issuerFor(r), "", s.now()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "signing the token failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": signed})
}

func (s *Service) issuerFor(r *http.Request) string {
	if s.issuer != "" {
		return s.issuer
	}
	return "https://" + r.Host + "/oauth2/token"
}

func (s *Service) sign(claims map[string]any) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(claims))
	token.Header["kid"] = keyID
	return token.SignedString(s.signingKey)
}

func (s *Service) sealGrant(g grant) (string, error) {
	payload, err := json.Marshal(g)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + base64.RawURLEncoding.EncodeToString(s.mac(encoded)), nil
}

func (s *Service) openGrant(code string) (grant, error) {
	encoded, signature, ok := strings.Cut(code, ".")
	if !ok {
		return grant{}, errors.New("the code is malformed")
	}
	provided, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || !hmac.Equal(provided, s.mac(encoded)) {
		return grant{}, errors.New("the code was not issued by this provider")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return grant{}, errors.New("the code is malformed")
	}
	var g grant
	if err := json.Unmarshal(payload, &g); err != nil {
		return grant{}, errors.New("the code is malformed")
	}
	if s.now().Unix() > g.Expires {
		return grant{}, errors.New("the code has expired")
	}
	return g, nil
}

func (s *Service) mac(value string) []byte {
	h := hmac.New(sha256.New, s.codeKey)
	h.Write([]byte(value))
	return h.Sum(nil)
}

// loginClaims decodes LoginHeader. An empty header means no identity is signed in.
func loginClaims(header string) (map[string]any, bool, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil, false, nil
	}
	if len(header) > maxLoginHeader {
		return nil, false, errors.New(LoginHeader + " is too large")
	}
	raw, err := base64.RawURLEncoding.DecodeString(header)
	if err != nil {
		return nil, false, errors.New(LoginHeader + " is not base64url")
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil || claims == nil {
		return nil, false, errors.New(LoginHeader + " is not a JSON object")
	}
	if sub, _ := claims["sub"].(string); sub == "" {
		return nil, false, errors.New(LoginHeader + " must name a sub")
	}
	return claims, true, nil
}

// matchesHint reports whether an organization hint, when one is given, names the identity's
// organization.
func matchesHint(claims map[string]any, hint string) bool {
	if hint == "" {
		return true
	}
	org, _ := claims[OrgClaim].(string)
	return org == hint
}

// withDefaults copies claims and adds iss, aud, iat and exp where they are missing.
func withDefaults(claims map[string]any, issuer, audience string, now time.Time) map[string]any {
	out := make(map[string]any, len(claims)+4)
	for key, value := range claims {
		out[key] = value
	}
	if _, ok := out["iss"]; !ok {
		out["iss"] = issuer
	}
	if _, ok := out["aud"]; !ok && audience != "" {
		out["aud"] = audience
	}
	if _, ok := out["iat"]; !ok {
		out["iat"] = now.Unix()
	}
	if _, ok := out["exp"]; !ok {
		out["exp"] = now.Add(tokenTTL).Unix()
	}
	return out
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func selfSigned() ([]byte, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("oidc: generating the TLS key: %w", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "testbench"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"testbench", "localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("oidc: creating the TLS certificate: %w", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("oidc: encoding the TLS key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, description string) {
	writeJSON(w, status, map[string]string{"error": code, "error_description": description})
}

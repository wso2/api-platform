/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package featureusage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TokenSource supplies the bearer token the collector proxy's jwt-auth policy
// accepts.
type TokenSource interface {
	// Token returns a valid token, fetching a new one when the cached one is
	// missing or close to expiry.
	Token(ctx context.Context) (string, error)
	// Invalidate drops token from the cache if it is still the cached one, so
	// the next Token call fetches a fresh one (after the gateway answered 401).
	Invalidate(token string)
}

const (
	// maxTokenResponseBytes bounds the token endpoint's response body.
	maxTokenResponseBytes = 64 << 10
	// defaultTokenLifetime is assumed when the token response omits expires_in.
	defaultTokenLifetime = time.Minute
)

// tokenError is a failure to obtain a token. Retryable is false for a verdict
// (bad credentials, malformed response) that a retry would only repeat.
type tokenError struct {
	msg       string
	retryable bool
}

func (e *tokenError) Error() string { return e.msg }

// clientCredentialsTokenSource fetches Platform IdP tokens with the OAuth 2.0
// client-credentials grant (RFC 6749 §4.4) and caches each until shortly
// before it expires. The client secret is only ever written to the token
// request's Authorization header; it never appears in an error or a log.
type clientCredentialsTokenSource struct {
	httpClient   *http.Client
	tokenURL     string
	clientID     string
	clientSecret string
	now          func() time.Time

	mu     sync.Mutex
	token  string
	expiry time.Time
}

func newClientCredentialsTokenSource(httpClient *http.Client, tokenURL, clientID, clientSecret string) *clientCredentialsTokenSource {
	return &clientCredentialsTokenSource{
		httpClient:   httpClient,
		tokenURL:     tokenURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		now:          time.Now,
	}
}

func (s *clientCredentialsTokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != "" && s.now().Before(s.expiry) {
		return s.token, nil
	}
	token, lifetime, err := s.fetch(ctx)
	if err != nil {
		return "", err
	}
	s.token = token
	s.expiry = s.now().Add(lifetime - refreshMargin(lifetime))
	return token, nil
}

func (s *clientCredentialsTokenSource) Invalidate(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token == token {
		s.token = ""
		s.expiry = time.Time{}
	}
}

// refreshMargin is how long before expiry a token stops being reused: a tenth
// of its lifetime, kept between 5 seconds and a minute, so a token is never
// presented in the last moments of its validity.
func refreshMargin(lifetime time.Duration) time.Duration {
	margin := lifetime / 10
	if margin < 5*time.Second {
		margin = 5 * time.Second
	}
	if margin > time.Minute {
		margin = time.Minute
	}
	if margin >= lifetime {
		margin = lifetime / 2
	}
	return margin
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

func (s *clientCredentialsTokenSource) fetch(ctx context.Context) (string, time.Duration, error) {
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, &tokenError{msg: "build token request: " + err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	// RFC 6749 §2.3.1: client credentials are form-encoded before basic auth.
	req.SetBasicAuth(url.QueryEscape(s.clientID), url.QueryEscape(s.clientSecret))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		// The URL is operator configuration, not a secret, but the transport
		// error is enough to diagnose; keep the message generic.
		return "", 0, &tokenError{msg: "token request failed: " + transportErrorKind(err), retryable: true}
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseBytes+1))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The body is not logged or returned: an IdP error can echo the client ID.
		return "", 0, &tokenError{
			msg:       fmt.Sprintf("token endpoint answered status %d", resp.StatusCode),
			retryable: resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests,
		}
	}
	if readErr != nil {
		return "", 0, &tokenError{msg: "read token response failed", retryable: true}
	}
	if len(body) > maxTokenResponseBytes {
		return "", 0, &tokenError{msg: "token response too large"}
	}

	var parsed tokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", 0, &tokenError{msg: "token response is not valid JSON"}
	}
	if parsed.AccessToken == "" {
		return "", 0, &tokenError{msg: "token response has no access_token"}
	}
	if parsed.TokenType != "" && !strings.EqualFold(parsed.TokenType, "bearer") {
		return "", 0, &tokenError{msg: "token response is not a bearer token"}
	}
	lifetime := defaultTokenLifetime
	if parsed.ExpiresIn > 0 {
		lifetime = time.Duration(parsed.ExpiresIn) * time.Second
	}
	return parsed.AccessToken, lifetime, nil
}

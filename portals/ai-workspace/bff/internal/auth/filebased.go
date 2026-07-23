/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package auth implements the BFF's two login mechanisms: file-based (forwarding
// credentials to the Platform API) and OIDC (confidential authorization-code
// flow). The resulting access JWT is placed in the HttpOnly session cookie; for
// OIDC the refresh/id tokens are additionally kept server-side for renewal.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ai-workspace-bff/internal/session"
)

// FileBased forwards username/password to the Platform API's file-based login
// endpoint and turns the returned HS256 JWT into a server-side session.
type FileBased struct {
	client   *http.Client
	loginURL string
	absTTL   time.Duration
	mapping  session.ClaimMapping
}

type fileBasedLoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// NewFileBased builds a file-based authenticator. platformBaseURL is the Platform
// API origin (e.g. https://platform-api:9243); portalBasePath is its portal route
// prefix (e.g. /api/portal/v0.9), under which /auth/login lives. mapping is the
// same claim mapping used for OIDC — the Platform API's file-based login endpoint
// signs its JWTs using these same mapped claim names, so an operator who
// customizes the claim names must not need to configure them twice.
func NewFileBased(client *http.Client, platformBaseURL, portalBasePath string, absTTL time.Duration, mapping session.ClaimMapping) *FileBased {
	return &FileBased{
		client:   client,
		loginURL: strings.TrimRight(platformBaseURL, "/") + strings.TrimRight(portalBasePath, "/") + "/auth/login",
		absTTL:   absTTL,
		mapping:  mapping,
	}
}

// ErrInvalidCredentials indicates the Platform API rejected the credentials.
type ErrInvalidCredentials struct{ Status int }

func (e ErrInvalidCredentials) Error() string {
	return fmt.Sprintf("invalid credentials (status %d)", e.Status)
}

// Login validates credentials against the Platform API and returns a populated
// (but not-yet-stored) Session. The caller assigns the id and persists it.
func (f *FileBased) Login(ctx context.Context, username, password string) (*session.Session, error) {
	form := url.Values{"username": {username}, "password": {password}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("platform api login request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusBadRequest {
		return nil, ErrInvalidCredentials{Status: res.StatusCode}
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("platform api login returned status %d", res.StatusCode)
	}

	var body fileBasedLoginResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil || body.Token == "" {
		return nil, fmt.Errorf("platform api login returned an invalid response")
	}

	claims := session.DecodeJWTClaims(body.Token)
	if claims == nil {
		return nil, fmt.Errorf("platform api login returned a token that is not a decodable JWT")
	}
	accessExpiry := time.Unix(body.ExpiresAt, 0)
	if accessExpiry.IsZero() || body.ExpiresAt == 0 {
		accessExpiry = session.ExpiryFromClaims(claims)
	}

	abs := accessExpiry
	if cap := time.Now().Add(f.absTTL); abs.IsZero() || cap.Before(abs) {
		abs = cap
	}

	return &session.Session{
		Mode:           session.ModeFileBased,
		AccessToken:    body.Token,
		AccessExpiry:   accessExpiry,
		AbsoluteExpiry: abs,
		User:           session.UserFromClaims(claims, nil, f.mapping),
	}, nil
}

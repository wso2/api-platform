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

// Package session holds the BFF's session helpers. The access JWT now travels in
// the HttpOnly cookie itself, so the proxy forwards it without any lookup. The
// store survives only to hold OIDC refresh/id tokens (keyed by the access token)
// so the proxy can renew the access token. The BFF does NOT validate tokens — it
// forwards them, and only decodes (without verifying) their claims for display.
package session

import (
	"context"
	"time"
)

// Mode distinguishes how a session's token was obtained.
const (
	ModeFileBased = "filebased"
	ModeOIDC      = "oidc"
)

// User holds the pre-decoded claims surfaced by GET /api/session. It mirrors the
// SPA's AppUser shape so the frontend can hydrate without seeing any token.
type User struct {
	Name   string   `json:"name"`
	Email  string   `json:"email"`
	Role   string   `json:"role,omitempty"`
	Scopes []string `json:"scopes"`
	Org    *Org     `json:"org,omitempty"`
}

// Org mirrors the SPA's AppOrg shape.
type Org struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Handle string `json:"handle"`
}

// Session is the server-side record. For OIDC it is keyed by the access token and
// holds the refresh/id tokens needed to renew it; file-based auth does not use it.
type Session struct {
	ID             string
	Mode           string
	AccessToken    string    // HS256 file-based JWT, or IDP access token (opaque to BFF)
	RefreshToken   string    // oidc only
	IDToken        string    // oidc only (for end-session id_token_hint)
	AccessExpiry   time.Time // from exp claim / expires_in (read, not verified)
	AbsoluteExpiry time.Time // hard cap
	User           User
}

// Expired reports whether the session has passed its absolute lifetime.
func (s *Session) Expired(now time.Time) bool {
	return !s.AbsoluteExpiry.IsZero() && !now.Before(s.AbsoluteExpiry)
}

// Store is the swappable session backend. The default is in-memory; a Redis
// implementation can satisfy the same interface for horizontal scaling.
type Store interface {
	Put(ctx context.Context, s *Session) error
	Get(ctx context.Context, id string) (*Session, bool, error)
	Delete(ctx context.Context, id string) error
	// Touch extends the absolute expiry by the idle window, bounded by the
	// underlying token lifetime (callers pass the cap).
	Touch(ctx context.Context, id string, extendTo time.Time) error
	Close() error
}

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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"ai-workspace-bff/internal/session"
)

// portalMePath is the Platform API endpoint that reports the signed-in user's
// identity and effective permissions (portal-api.yaml, GET /me).
const portalMePath = "/api/portal/v0.9/me"

// maxMeResponseBytes caps how much of the /me response we will read. The payload
// is a small identity record; anything larger is an upstream fault, not something
// to pull into memory.
const maxMeResponseBytes = 1 << 20 // 1 MiB

// meResponse is the subset of GET /me the BFF consumes.
type meResponse struct {
	Roles  []string `json:"roles"`
	Scopes []string `json:"scopes"`
}

// fetchPermissions asks the Platform API which roles the caller holds and which
// scopes those roles grant.
func (s *Server) fetchPermissions(ctx context.Context, jwt string) (*meResponse, error) {
	resp, err := s.platformDo(ctx, jwt, http.MethodGet, portalMePath, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("platform api GET %s: status %d", portalMePath, resp.StatusCode)
	}

	var me meResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxMeResponseBytes)).Decode(&me); err != nil {
		return nil, fmt.Errorf("platform api GET %s: decode: %w", portalMePath, err)
	}
	return &me, nil
}

// enrichPermissions fills in the scopes a role-only token does not carry.
//
// In role validation mode the IDP issues a token with roles and no scope claim,
// so decoding the claim yields nothing and the SPA — which gates every control on
// hasPermission(scope) — would render an empty app for a fully privileged user.
// The Platform API expands roles into scopes via roles.yaml for its own
// authorization; GET /me returns that same expansion, so the UI is driven by the
// list the API actually enforces instead of a second copy of the mapping shipped
// to the browser.
//
// A token that already carries scopes needs no upstream hop. A failed lookup
// leaves the user with no scopes: the app degrades to hiding privileged controls
// rather than offering actions the API would then reject.
func (s *Server) enrichPermissions(ctx context.Context, jwt string, u *session.User) {
	if len(u.Scopes) > 0 {
		return
	}

	me, err := s.fetchPermissions(ctx, jwt)
	if err != nil {
		slog.Warn("bff: effective permissions lookup failed; continuing with no scopes", "err", err)
		return
	}

	u.Scopes = me.Scopes
	// The SPA shows the role as a label. Tokens in role mode carry no platform_role
	// claim, so fall back to the first IDP role the user holds.
	if u.Role == "" && len(me.Roles) > 0 {
		u.Role = me.Roles[0]
	}
}

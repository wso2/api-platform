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

	"ai-workspace-bff/internal/paths"
)

// orgListResponse is the Platform API's GET /organizations payload, narrowed to the
// one field this needs. Deliberately not the generated model: an extra field added
// upstream must not turn org resolution into a decode error.
type orgListResponse struct {
	List []struct {
		Handle string `json:"handle"`
	} `json:"list"`
}

// orgDiscoveryLimit asks for one organization because only the first is used. The
// Platform API orders the list itself; taking the first is what "the org the user
// lands in" means until they switch.
const orgDiscoveryLimit = "1"

// discoverOrgHandle asks the Platform API which organizations the LOGIN token's user
// belongs to, and returns the first one's handle.
//
// It is called with the login token on purpose, and it is the one Platform API call
// that must not go through upstreamToken: it exists precisely to decide what the
// exchange is scoped to, so exchanging first would be circular. That also makes it
// the only call whose upstream sees the login token in exchange mode — acceptable
// because it is a read of the caller's own memberships, which the login token is
// already the right credential for.
func (s *Server) discoverOrgHandle(ctx context.Context, loginToken string) (string, error) {
	path := paths.PlatformAPI + "/organizations?limit=" + orgDiscoveryLimit + "&offset=0"
	resp, err := s.platformDo(ctx, loginToken, http.MethodGet, path, nil, nil)
	if err != nil {
		return "", fmt.Errorf("calling the Platform API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Bounded: a large or streaming error body must not be read into memory
		// whole just to be quoted in a log line.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("Platform API answered %d: %s", resp.StatusCode, snippet)
	}

	var body orgListResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", fmt.Errorf("decoding the organization list: %w", err)
	}
	if len(body.List) == 0 || body.List[0].Handle == "" {
		return "", nil
	}
	return body.List[0].Handle, nil
}

// resolveOrgHandle decides which org this session's exchange is scoped to, in
// precedence order: the user's own switch, then their first organization as the
// Platform API reports it, then the configured default_org. It persists a discovered
// handle on the session so the lookup happens once per session rather than once per
// exchange, and so /api/session and the proxy agree about which org the user is in.
//
// The lookup is unconditional rather than opt-in because there is no deployment for
// which the alternative is better: the user's own memberships are the only source
// that is right for every user, while default_org is one guess shared by all of them
// and "let the STS decide" is what makes the Choreo STS answer 500 for a user who is
// not in the org it picked. It costs one call per session, and only for sessions
// that have not chosen an org yet.
//
// Every failure below falls through to default_org rather than failing the caller: a
// Platform API that is briefly unreachable should degrade to the configured guess,
// not lock everyone out.
func (s *Server) resolveOrgHandle(ctx context.Context, subjectToken, sessionOrg string) string {
	te := s.cfg.Auth.OIDC.TokenExchange
	if sessionOrg != "" {
		return sessionOrg
	}
	// Nothing to discover FOR: without org_param the resolved handle has no field to
	// travel in, so the lookup would find the right answer and then drop it.
	if te.OrgParam == "" {
		return te.DefaultOrg
	}

	handle, err := s.discoverOrgHandle(ctx, subjectToken)
	if err != nil {
		slog.Warn("could not read the user's organizations from the Platform API — "+
			"falling back to [auth.oidc.token_exchange] default_org",
			"err", err, "default_org", te.DefaultOrg)
		return te.DefaultOrg
	}
	if handle == "" {
		// Not an error: a user who belongs to no org yet is exactly who the
		// registration flow exists for.
		slog.Info("the Platform API reports no organizations for this user — " +
			"exchanging without one")
		return te.DefaultOrg
	}

	// Best-effort, like the exchanged-token cache: losing this only costs another
	// lookup on the next exchange.
	s.withSessionLock(subjectToken, func() {
		if sess, ok, _ := s.store.Get(ctx, subjectToken); ok {
			sess.OrgHandle = handle
			if err := s.store.Put(ctx, sess); err != nil {
				slog.Warn("failed to persist the resolved organization on the session", "err", err)
			}
		}
	})
	slog.Debug("resolved the user's organization from the Platform API", "org_handle", handle)
	return handle
}

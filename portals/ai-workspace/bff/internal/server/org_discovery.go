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

// orgListResponse covers both shapes this can be answered in — the Platform API's
// {"list":[…]} and a user service's {"organizations":[…]} — narrowed to the one
// field either way. Reading both means org_lookup_url needs no companion key saying
// which format lives behind it, and a payload can only match one of them.
//
// Deliberately not the generated model: a field added upstream must not turn org
// resolution into a decode error.
type orgListResponse struct {
	List          []orgEntry `json:"list"`
	Organizations []orgEntry `json:"organizations"`
}

type orgEntry struct {
	Handle string `json:"handle"`
}

// first returns the first handle from whichever shape the payload used.
func (r orgListResponse) first() string {
	for _, list := range [][]orgEntry{r.List, r.Organizations} {
		if len(list) > 0 && list[0].Handle != "" {
			return list[0].Handle
		}
	}
	return ""
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
	resp, source, err := s.orgLookupResponse(ctx, loginToken)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Bounded: a large or streaming error body must not be read into memory
		// whole just to be quoted in a log line.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("%s answered %d: %s", source, resp.StatusCode, snippet)
	}

	var body orgListResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", fmt.Errorf("decoding the organization list from %s: %w", source, err)
	}
	return body.first(), nil
}

// orgLookupResponse performs the lookup against whichever source is configured, and
// names it for the error messages: which one answered is the first thing a failure
// here raises, and the two fail for entirely different reasons.
func (s *Server) orgLookupResponse(ctx context.Context, loginToken string) (*http.Response, string, error) {
	if raw := s.cfg.Auth.OIDC.TokenExchange.OrgLookupURL; raw != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			return nil, "the configured org lookup", fmt.Errorf("building the request: %w", err)
		}
		// Sent as configured — query string and all — because the URL names one
		// endpoint of one service, not a route this composes.
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+loginToken)
		resp, err := s.platformClient().Do(req)
		if err != nil {
			return nil, "the configured org lookup", fmt.Errorf("calling %s: %w", raw, err)
		}
		return resp, "the configured org lookup", nil
	}

	path := paths.PlatformAPI + "/organizations?limit=" + orgDiscoveryLimit + "&offset=0"
	resp, err := s.platformDo(ctx, loginToken, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, "the Platform API", fmt.Errorf("calling the Platform API: %w", err)
	}
	return resp, "the Platform API", nil
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
		// registration flow exists for. It is also what an org lookup pointed at
		// the wrong endpoint looks like — one that answers 200 in a shape carrying
		// no organizations — so the source is named.
		slog.Info("no organizations for this user — exchanging without one",
			"org_lookup_url", te.OrgLookupURL)
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
	slog.Debug("resolved the user's organization", "org_handle", handle,
		"org_lookup_url", te.OrgLookupURL)
	return handle
}

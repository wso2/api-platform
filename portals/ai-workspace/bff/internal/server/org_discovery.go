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
	"strings"

	"ai-workspace-bff/internal/paths"
	"ai-workspace-bff/internal/session"
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
	target, external := s.orgLookupTarget()

	// The whole request, before it is sent: which URL, chosen how, and — because the
	// failure this most often hits is the upstream refusing the LOGIN token — who
	// issued that token and who it was minted for. The token itself is never logged;
	// its issuer and audience are what a 401 here is actually about.
	claims := session.DecodeJWTClaims(loginToken)
	slog.Debug("org lookup: sending request",
		"url", target,
		"source", lookupSource(external),
		"subject_token_iss", claims["iss"],
		"subject_token_aud", claims["aud"],
		"subject_token_present", loginToken != "")

	resp, err := s.orgLookupResponse(ctx, target, external, loginToken)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	// Read once, bounded, so the same bytes can be both logged and decoded — and so a
	// large or streaming body cannot be pulled into memory whole.
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, orgLookupBodyLimit))
	slog.Debug("org lookup: response",
		"url", target,
		"status", resp.StatusCode,
		"content_type", resp.Header.Get("Content-Type"),
		"body", string(truncate(raw, orgLookupLogLimit)))
	if readErr != nil {
		return "", fmt.Errorf("reading the response from %s: %w", target, readErr)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s (%s) answered %d: %s",
			lookupSource(external), target, resp.StatusCode, truncate(raw, orgLookupLogLimit))
	}

	var body orgListResponse
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", fmt.Errorf("decoding the organization list from %s: %w", target, err)
	}

	handle := body.first()
	slog.Debug("org lookup: parsed",
		"url", target,
		"organizations", len(body.List)+len(body.Organizations),
		"first_handle", handle)
	return handle, nil
}

// orgLookupBodyLimit caps what is read from the lookup; orgLookupLogLimit caps what
// of it reaches a log line or an error message. A membership list is a few KB, and
// neither an oversized body nor a verbose error page should cost more than that.
const (
	orgLookupBodyLimit = 1 << 20
	orgLookupLogLimit  = 512
)

func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return append(b[:n:n], "…(truncated)"...)
}

func lookupSource(external bool) string {
	if external {
		return "the configured org lookup"
	}
	return "the Platform API"
}

// orgLookupTarget returns the URL the lookup will call and whether it came from
// org_lookup_url. The Platform API's own URL is composed here rather than inside
// platformDo so the debug line can name it before the call goes out — "which URL did
// it actually try" being the first question a failing lookup raises.
func (s *Server) orgLookupTarget() (string, bool) {
	if raw := s.cfg.Auth.OIDC.TokenExchange.OrgLookupURL; raw != "" {
		return raw, true
	}
	path := paths.PlatformAPI + "/organizations?limit=" + orgDiscoveryLimit + "&offset=0"
	return strings.TrimRight(s.cfg.ControlPlane.URL, "/") + s.cfg.ControlPlane.UpstreamPath(path), false
}

// orgLookupResponse performs the lookup against whichever source orgLookupTarget
// picked. The external one is sent as configured — query string and all — because
// that URL names one endpoint of one service, not a route this composes.
func (s *Server) orgLookupResponse(
	ctx context.Context, target string, external bool, loginToken string,
) (*http.Response, error) {
	if !external {
		path := paths.PlatformAPI + "/organizations?limit=" + orgDiscoveryLimit + "&offset=0"
		resp, err := s.platformDo(ctx, loginToken, http.MethodGet, path, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("calling %s: %w", target, err)
		}
		return resp, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("building the request for %s: %w", target, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginToken)
	resp, err := s.platformClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling %s: %w", target, err)
	}
	return resp, nil
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
	// travel in, so the lookup would find the right answer and then drop it. Said
	// out loud because "the lookup never ran" and "the lookup failed" are otherwise
	// the same silence, and this one is a config fix.
	if te.OrgParam == "" {
		slog.Debug("org lookup skipped: [auth.oidc.token_exchange] org_param is empty, so the " +
			"exchange can carry no org and the STS will resolve its own default")
		return te.DefaultOrg
	}

	handle, err := s.discoverOrgHandle(ctx, subjectToken)
	if err != nil {
		// The error already names the URL that answered — the source is not assumed
		// here, because which of the two ran is half the diagnosis.
		slog.Warn("could not read the user's organizations — falling back to "+
			"[auth.oidc.token_exchange] default_org",
			"err", err, "default_org", te.DefaultOrg, "org_lookup_url", te.OrgLookupURL)
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

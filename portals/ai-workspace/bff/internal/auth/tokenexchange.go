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

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"ai-workspace-bff/internal/config"
	"ai-workspace-bff/internal/session"
)

// RFC 8693 §3 token type identifiers.
const (
	TokenTypeAccessToken = "urn:ietf:params:oauth:token-type:access_token"
	TokenTypeJWT         = "urn:ietf:params:oauth:token-type:jwt"
	TokenTypeIDToken     = "urn:ietf:params:oauth:token-type:id_token"
	TokenTypeRefresh     = "urn:ietf:params:oauth:token-type:refresh_token"
)

// The wire values, aliased from config so the URI the request carries and the URI an
// operator may spell in grant_type can never drift apart.
const (
	grantURITokenExchange = config.GrantURITokenExchange
	grantURIJWTBearer     = config.GrantURIJWTBearer
)

const (
	exchangeTimeout          = 15 * time.Second
	maxExchangeResponseBytes = 1 << 20

	// All attempts share the exchangeTimeout budget above.
	exchangeMaxAttempts      = 3
	exchangeRetryBaseBackoff = 400 * time.Millisecond
	exchangeRetryMaxBackoff  = 2 * time.Second

	// unavailableCooldown is how long a confirmed-unreachable IDP is assumed to
	// still be unreachable. Within the window every exchange fails immediately
	// instead of spending the full exchangeTimeout budget rediscovering it.
	//
	// Without this, a wedged token endpoint makes the portal look wedged: the SPA
	// fires a burst of API calls per page load, each of which blocks for up to
	// exchangeTimeout before returning 502, and each of which adds load to the
	// component that is already failing. The window is deliberately short — it is
	// there to collapse a storm, not to keep a recovered IDP shut out.
	unavailableCooldown = 3 * time.Second
)

// ErrExchangeUnavailable means the IDP could not be reached or failed server-side;
// retrying may succeed, so callers keep the session and report 502.
var ErrExchangeUnavailable = errors.New("token exchange upstream unavailable")

// ErrExchangeRejected means the IDP refused the exchange. The session can never
// produce an upstream token, so callers destroy it and report 401.
var ErrExchangeRejected = errors.New("token exchange rejected by identity provider")

// Exchanger trades the login token for one minted for the Platform API. Its
// settings come from [ai_workspace.auth.oidc.token_exchange].
//
// No method returns the unexchanged subject token: a failed exchange is an error,
// never a fallback. Forwarding the login token instead would reach the Platform API
// with the wrong audience and, on an IDP that mints no ap:* scopes, no platform
// authorization at all.
type Exchanger struct {
	client          *http.Client
	cfg             config.TokenExchangeConfig
	endpoint        string
	requestedScopes []string

	// mu guards the upstream-health state below, which is shared by every session:
	// the token endpoint is one upstream, so what one request learns about it is
	// true for all of them.
	mu               sync.Mutex
	unavailableUntil time.Time

	// warnedScopeSets remembers which unrequested-scope sets have already been
	// reported, so an over-provisioned application is logged once per distinct set
	// rather than on every exchange — a warning repeated per request is noise that
	// teaches people to filter the channel it arrives on.
	warnedScopeSets map[string]struct{}
}

// NewExchanger builds an Exchanger for an already-validated config. endpoint is the
// resolved token endpoint, so discovery stays with the OIDC client.
func NewExchanger(client *http.Client, cfg config.TokenExchangeConfig, endpoint string) *Exchanger {
	return &Exchanger{
		client:          noRedirectClient(client),
		cfg:             cfg,
		endpoint:        endpoint,
		requestedScopes: strings.Fields(cfg.Scopes),
		warnedScopeSets: make(map[string]struct{}),
	}
}

// noRedirectClient copies client with redirects disabled: the exchange POST body
// carries the client secret and subject token, and Go replays it on a 307/308.
func noRedirectClient(client *http.Client) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

func (e *Exchanger) Endpoint() string           { return e.endpoint }
func (e *Exchanger) CacheEnabled() bool         { return e.cfg.CacheEnabled }
func (e *Exchanger) MinValidity() time.Duration { return e.cfg.MinValidity }

// ConfigFingerprint identifies the settings that determine what the IDP mints, so a
// cached token is not reused after they change.
//
// Every field that alters the request belongs here, including the ones that shape it
// rather than name the target: turning on org_param, or switching subject_token_type
// while migrating IDPs, changes what comes back just as surely as changing audience
// does. A field left out is not a cosmetic omission — it means sessions holding a
// token minted under the old configuration keep using it until it expires, so the
// change appears to work for new logins and not for existing ones. ClientID is
// included because a different application can carry different grants; ClientSecret
// is not, because it authenticates the BFF without altering the issued token.
//
// TestConfigFingerprintCoversEveryRequestAffectingField walks the config struct and
// fails if a newly added field is neither reflected here nor explicitly declared
// irrelevant, so this cannot quietly drift again.
func (e *Exchanger) ConfigFingerprint() string {
	return strings.Join([]string{
		e.cfg.GrantType,
		e.endpoint, // the resolved endpoint, which may differ from cfg.TokenEndpoint
		e.cfg.ClientID,
		e.cfg.Audience,
		e.cfg.Resource,
		e.cfg.Scopes,
		e.cfg.SubjectTokenType,
		e.cfg.RequestedTokenType,
		e.cfg.OrgParam,
	}, "\x1f")
}

// Result is one completed exchange. A zero Expiry means the IDP supplied no lifetime,
// which callers must treat as uncacheable rather than as expired.
type Result struct {
	AccessToken string
	Expiry      time.Time
	Scopes      []string
}

type exchangeResponse struct {
	AccessToken     string `json:"access_token"`
	IssuedTokenType string `json:"issued_token_type"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int64  `json:"expires_in"`
	Scope           string `json:"scope"`
	RefreshToken    string `json:"refresh_token"`
}

type exchangeError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

// Exchange trades subjectToken for a Platform API token. subjectToken is a live
// credential and is never logged. Only ErrExchangeUnavailable is retried.
//
// orgHandle is the org currently selected in the SPA; it is forwarded on the wire
// only when [auth.oidc.token_exchange] org_param is configured (see buildForm) —
// for an IDP that mints org-scoped scopes for the same user. Pass "" when no org
// is selected yet, or when org-scoped exchange isn't configured.
func (e *Exchanger) Exchange(ctx context.Context, subjectToken, orgHandle string) (*Result, error) {
	if subjectToken == "" {
		return nil, fmt.Errorf("%w: no subject token", ErrExchangeRejected)
	}
	if err := e.upstreamDown(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, exchangeTimeout)
	defer cancel()

	var err error
	for attempt := 1; ; attempt++ {
		var res *Result
		res, err = e.exchangeOnce(ctx, subjectToken, orgHandle)
		if err == nil {
			e.noteUpstreamHealthy()
			if attempt > 1 {
				slog.Info("token exchange succeeded after retrying a transient identity provider failure",
					"attempts", attempt)
			}
			return res, nil
		}
		if errors.Is(err, ErrExchangeUnavailable) {
			e.noteUpstreamUnavailable()
		}

		var failure *exchangeFailure
		retryable := errors.As(err, &failure) && failure.retryable
		if !retryable || attempt == exchangeMaxAttempts {
			return nil, err
		}

		// Report the real failure rather than a context error on deadline.
		timer := time.NewTimer(retryDelay(attempt, failure.retryAfter))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, err
		case <-timer.C:
		}
	}
}

// exchangeFailure carries retry guidance alongside the sentinel the caller
// switches on. Whether a failure is worth retrying is decided where it is
// classified — the retry loop cannot tell a wedged endpoint (retry) from a
// rejected audience (retrying a configuration mistake only wastes the request's
// deadline) from a rate limit (retrying actively makes it worse).
type exchangeFailure struct {
	err        error
	retryable  bool
	retryAfter time.Duration
}

func (f *exchangeFailure) Error() string { return f.err.Error() }
func (f *exchangeFailure) Unwrap() error { return f.err }

func transient(retryAfter time.Duration, format string, args ...any) error {
	return &exchangeFailure{err: fmt.Errorf(format, args...), retryable: true, retryAfter: retryAfter}
}

func permanent(format string, args ...any) error {
	return &exchangeFailure{err: fmt.Errorf(format, args...)}
}

// retryDelay backs off exponentially with jitter, never below what the IDP asked
// for via Retry-After.
//
// The jitter is the point, not a refinement: a fixed delay means every session
// that failed in the same instant retries in the same instant, so a blip that
// affected many sessions returns as a synchronized burst against the endpoint
// that is already struggling — the same stampede go-network-service-hardening.md
// directive 4 forbids for poll loops, arriving by way of retries instead.
func retryDelay(attempt int, retryAfter time.Duration) time.Duration {
	delay := exchangeRetryBaseBackoff << (attempt - 1)
	if delay > exchangeRetryMaxBackoff {
		delay = exchangeRetryMaxBackoff
	}
	if retryAfter > delay {
		delay = retryAfter
	}
	// Full jitter over the upper half, so delays spread without collapsing to zero.
	return delay/2 + rand.N(delay/2+1)
}

// upstreamDown fails fast while the token endpoint is known to be unreachable.
func (e *Exchanger) upstreamDown() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if time.Now().Before(e.unavailableUntil) {
		return transient(0, "%w: identity provider was unreachable moments ago", ErrExchangeUnavailable)
	}
	return nil
}

func (e *Exchanger) noteUpstreamUnavailable() {
	e.mu.Lock()
	e.unavailableUntil = time.Now().Add(unavailableCooldown)
	e.mu.Unlock()
}

// noteUpstreamHealthy reopens the gate immediately on the first success, so a
// recovered IDP is not kept out for the remainder of the cooldown.
func (e *Exchanger) noteUpstreamHealthy() {
	e.mu.Lock()
	e.unavailableUntil = time.Time{}
	e.mu.Unlock()
}

// exchangeOnce performs a single exchange request.
func (e *Exchanger) exchangeOnce(ctx context.Context, subjectToken, orgHandle string) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint,
		strings.NewReader(e.buildForm(subjectToken, orgHandle).Encode()))
	if err != nil {
		// A request that cannot be built is a malformed endpoint, not a blip: it
		// will fail identically on every attempt, so retrying only burns the
		// caller's deadline before returning the same error.
		return nil, permanent("%w: %v", ErrExchangeUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := e.client.Do(req)
	if err != nil {
		return nil, transient(0, "%w: %v", ErrExchangeUnavailable, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxExchangeResponseBytes))
	if err != nil {
		return nil, transient(0, "%w: reading response: %v", ErrExchangeUnavailable, err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, e.classifyError(res.StatusCode, body, subjectToken, res.Header)
	}

	var tok exchangeResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, transient(0, "%w: decoding response: %v", ErrExchangeUnavailable, err)
	}
	if tok.AccessToken == "" {
		return nil, transient(0, "%w: response carried no access_token", ErrExchangeUnavailable)
	}
	// issued_token_type is an RFC 8693 field; Entra's OBO response has none.
	if e.cfg.GrantType == config.GrantTokenExchange {
		if err := validateIssuedTokenType(tok.IssuedTokenType); err != nil {
			return nil, permanent("%w: %v", ErrExchangeRejected, err)
		}
	}

	claims := session.DecodeJWTClaims(tok.AccessToken)
	return &Result{
		AccessToken: tok.AccessToken,
		Expiry:      expiryFrom(tok.ExpiresIn, claims),
		Scopes:      e.grantedScopes(tok.Scope, claims),
	}, nil
}

func (e *Exchanger) buildForm(subjectToken, orgHandle string) url.Values {
	form := url.Values{
		"client_id":     {e.cfg.ClientID},
		"client_secret": {e.cfg.ClientSecret},
	}

	// Entra ID does not implement RFC 8693 — it rejects that grant. Its on-behalf-of
	// flow is RFC 7523: the subject travels as `assertion`, and the target API is
	// named through `scope`, so audience/resource have no place in the request.
	if e.cfg.GrantType == config.GrantJWTBearer {
		form.Set("grant_type", grantURIJWTBearer)
		form.Set("assertion", subjectToken)
		form.Set("requested_token_use", "on_behalf_of")
		if e.cfg.Scopes != "" {
			form.Set("scope", e.cfg.Scopes)
		}
		e.setOrgParam(form, orgHandle)
		return form
	}

	form.Set("grant_type", grantURITokenExchange)
	form.Set("subject_token", subjectToken)
	form.Set("subject_token_type", e.cfg.SubjectTokenType)
	form.Set("requested_token_type", e.cfg.RequestedTokenType)
	// config.validate permits at most one: IDPs read them differently (PingFederate
	// treats them as distinct selectors), so sending both leaves the target ambiguous.
	if e.cfg.Audience != "" {
		form.Set("audience", e.cfg.Audience)
	}
	if e.cfg.Resource != "" {
		form.Set("resource", e.cfg.Resource)
	}
	if e.cfg.Scopes != "" {
		form.Set("scope", e.cfg.Scopes)
	}
	e.setOrgParam(form, orgHandle)
	return form
}

// setOrgParam adds the org-scoping field an operator has named via org_param,
// carrying the literal org handle — no templating, matching the wire shape a real
// IDP-backed deployment of this exchange uses. A no-op when org-scoped exchange
// isn't configured, or no org is selected yet.
func (e *Exchanger) setOrgParam(form url.Values, orgHandle string) {
	if e.cfg.OrgParam != "" && orgHandle != "" {
		form.Set(e.cfg.OrgParam, orgHandle)
	}
}

// subjectTokenVerdicts are the error codes that are a verdict on the subject token
// or on this user specifically — the only failures that mean "this session can never
// produce an upstream token", which is what callers act on by destroying it.
//
// invalid_request is here despite its generic name: RFC 8693 §2.2.2 requires it as
// the code when "the subject_token or actor_token are invalid for any reason, or are
// unacceptable based on policy", so for this grant it is the bad-subject-token code.
// It is unavoidably ambiguous — a genuinely malformed request produces it too — and
// the ambiguity is the RFC's, not ours. The codes that are unambiguously about the
// request are classified below instead.
var subjectTokenVerdicts = map[string]bool{
	"invalid_request":      true, // RFC 8693 §2.2.2: the subject token is invalid or refused by policy
	"invalid_grant":        true, // the subject token is expired, revoked or untrusted
	"access_denied":        true, // policy refused this subject
	"consent_required":     true, // needs an interactive login this flow cannot perform
	"interaction_required": true,
	"login_required":       true,
}

// configFaults are the codes that describe the request the BFF sent rather than the
// token it carried. That request is built entirely from [auth.oidc.token_exchange]
// and is byte-identical for every user, so one of these means the deployment is
// misconfigured — not that anyone's credentials are bad.
//
// They must not be rejections. Treating them as such turns a single mistyped
// audience into a platform-wide logout that users cannot recover from on their own:
// the eager exchange in the OIDC callback fails the same way, so logging back in
// fails too. Classified unavailable, the session survives, the user sees a 502, and
// everyone resumes the moment the configuration is corrected. They are not retried
// either — a configuration mistake does not fix itself within one request's deadline.
var configFaults = map[string]bool{
	"invalid_target":         true, // RFC 8693 §2.2.2: audience/resource not accepted
	"invalid_client":         true, // client_id/client_secret rejected
	"unauthorized_client":    true, // this client may not use this grant
	"unsupported_grant_type": true,
	"invalid_scope":          true,
}

// classifyError logs the IDP's reason and returns a sentinel. The reason stays
// internal: whether the subject or the target was refused maps out the deployment's
// trust configuration.
func (e *Exchanger) classifyError(status int, body []byte, subjectToken string, header http.Header) error {
	var ee exchangeError
	_ = json.Unmarshal(body, &ee)

	retryAfter := parseRetryAfter(header)
	attrs := []any{"status", status, "grant_type", e.cfg.GrantType, "endpoint", e.endpoint}
	if ee.Code != "" {
		attrs = append(attrs, "idp_error", ee.Code)
	}
	if ee.Description != "" {
		attrs = append(attrs, "idp_error_description", e.redactSecrets(ee.Description, subjectToken))
	}
	if retryAfter > 0 {
		attrs = append(attrs, "retry_after", retryAfter)
	}

	// A rate limit is the one failure where retrying is actively harmful: every
	// retry spends more of the budget the IDP just said we had exhausted. Fail this
	// request (the session survives) and let the next one find out.
	if status == http.StatusTooManyRequests {
		slog.Warn("token exchange failed: identity provider is rate limiting the BFF — "+
			"not retried, to avoid deepening the limit", attrs...)
		return permanent("%w: rate limited", ErrExchangeUnavailable)
	}
	// 408 means "not now", like a 5xx: a rejection here would destroy a live
	// session over a timeout.
	if status >= http.StatusInternalServerError || status == http.StatusRequestTimeout {
		slog.Error("token exchange failed: identity provider error", attrs...)
		return transient(retryAfter, "%w: status %d", ErrExchangeUnavailable, status)
	}
	// A failed key-set fetch is the IDP's own upstream problem, not a verdict on
	// this token, despite the 4xx code and despite arriving as invalid_grant.
	if isTransientKeyFetchFailure(ee.Description) {
		slog.Warn("token exchange failed: identity provider could not reach the subject token's "+
			"issuer key set — treating as transient, the session is kept", attrs...)
		return transient(retryAfter, "%w: identity provider key-set fetch failed", ErrExchangeUnavailable)
	}
	if ee.Code == "invalid_target" {
		// Called out separately only because the fix is specific enough to name.
		slog.Error("token exchange failed: audience/resource not accepted by the IDP — "+
			"register it on the exchanging application, or correct "+
			"[auth.oidc.token_exchange] audience / resource. Sessions are being kept "+
			"and requests answered 502 until this is corrected",
			append(attrs, "configured_audience", e.cfg.Audience, "configured_resource", e.cfg.Resource)...)
		return permanent("%w: invalid_target", ErrExchangeUnavailable)
	}
	if configFaults[ee.Code] {
		slog.Error("token exchange failed: the identity provider refused the request the BFF sent, "+
			"which is built from [auth.oidc.token_exchange] and is identical for every user — "+
			"this is a configuration fault, not a credential one. Sessions are being kept "+
			"and requests answered 502 until it is corrected", attrs...)
		return permanent("%w: %s", ErrExchangeUnavailable, ee.Code)
	}
	if subjectTokenVerdicts[ee.Code] {
		slog.Warn("token exchange rejected: the identity provider refused this subject token", attrs...)
		return permanent("%w: %s", ErrExchangeRejected, ee.Code)
	}
	// An unrecognised code is more likely a deployment problem than a verdict on one
	// user's token, and guessing "rejected" is the expensive direction to be wrong in:
	// it logs people out. Keep the session and surface a 502.
	slog.Error("token exchange failed with an unrecognised error code — treating it as a "+
		"configuration fault and keeping sessions; if this is in fact a per-user rejection, "+
		"add the code to subjectTokenVerdicts", attrs...)
	return permanent("%w: %s", ErrExchangeUnavailable, ee.Code)
}

// parseRetryAfter reads the RFC 9110 Retry-After header in either form.
func parseRetryAfter(header http.Header) time.Duration {
	raw := header.Get("Retry-After")
	if raw == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if when, err := http.ParseTime(raw); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return 0
}

// grantedScopes prefers the response's scope, which RFC 8693 §2.2.1 makes REQUIRED
// whenever it differs from the request, and falls back to the issued token's claim.
//
// The widening check cannot be an enforcement point — only the IDP knows the subject
// token's grant — but scopes the BFF never asked for mean the exchanging application
// is over-provisioned, which is worth surfacing.
func (e *Exchanger) grantedScopes(granted string, claims map[string]any) []string {
	scopes := strings.Fields(granted)
	if len(scopes) == 0 {
		scopes = scopeClaim(claims)
	}
	if len(e.requestedScopes) == 0 {
		return scopes
	}
	var extra []string
	for _, s := range scopes {
		if !slices.Contains(e.requestedScopes, s) {
			extra = append(extra, s)
		}
	}
	if len(extra) > 0 && e.shouldWarnScopes(extra) {
		slog.Warn("token exchange returned scopes that were not requested — "+
			"the exchanging application may be over-provisioned at the IDP",
			"unrequested_scopes", extra)
	}
	return scopes
}

// shouldWarnScopes reports whether this set of unrequested scopes is new. The
// condition it reports is a property of the IDP application, not of the request, so
// it is the same on every exchange — logging it per request would bury the one line
// an operator needs under thousands of identical ones.
func (e *Exchanger) shouldWarnScopes(extra []string) bool {
	key := strings.Join(extra, " ")
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, seen := e.warnedScopeSets[key]; seen {
		return false
	}
	e.warnedScopeSets[key] = struct{}{}
	return true
}

// jwtLikeToken is a backstop only: a token need not be a JWT, so shape cannot be
// the primary defence.
var jwtLikeToken = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}(?:\.[A-Za-z0-9_-]+)?`)

// redactSecrets strips credentials an IDP echoed into error_description
// (GO-AUTH-003). The description is kept: it is the only field distinguishing an
// untrusted issuer from a failed key fetch from a rejected audience.
func (e *Exchanger) redactSecrets(description, subjectToken string) string {
	out := description
	for _, secret := range []string{subjectToken, e.cfg.ClientSecret} {
		if len(secret) >= 8 { // shorter values would corrupt ordinary prose
			out = strings.ReplaceAll(out, secret, "[REDACTED]")
		}
	}
	return jwtLikeToken.ReplaceAllString(out, "[REDACTED-JWT]")
}

// isTransientKeyFetchFailure matches on the description because the error code
// carries no distinction: Asgardeo returns invalid_grant for a failed key-set fetch
// and for a genuinely untrusted issuer alike.
func isTransientKeyFetchFailure(description string) bool {
	d := strings.ToLower(description)
	if !strings.Contains(d, "jwks") && !strings.Contains(d, "key set") {
		return false
	}
	return strings.Contains(d, "error occurred while accessing") ||
		strings.Contains(d, "unable to") ||
		strings.Contains(d, "failed to")
}

func validateIssuedTokenType(t string) error {
	switch t {
	case TokenTypeAccessToken, TokenTypeJWT:
		return nil
	case "":
		return errors.New("response omitted the required issued_token_type")
	default:
		return fmt.Errorf("issued_token_type %q is not usable as an upstream bearer token", t)
	}
}

// expiryFrom falls back to the exp claim because RFC 8693 only RECOMMENDS expires_in.
func expiryFrom(expiresIn int64, claims map[string]any) time.Time {
	if expiresIn > 0 {
		return time.Now().Add(time.Duration(expiresIn) * time.Second)
	}
	return session.ExpiryFromClaims(claims)
}

func scopeClaim(claims map[string]any) []string {
	raw, ok := claims["scope"]
	if !ok {
		raw = claims["scp"]
	}
	switch v := raw.(type) {
	case string:
		return strings.Fields(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

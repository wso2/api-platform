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
	"net/http"
	"net/url"
	"slices"
	"strings"
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

	// An IDP that cannot momentarily reach the subject token's issuer reports a
	// client-side error code (Asgardeo answers invalid_grant when its fetch of the
	// trusted issuer's JWKS fails), which is indistinguishable by code alone from a
	// permanently misconfigured trust. Observed failure runs are short — three
	// rejections followed by success, with nothing changed between them — so a few
	// closely spaced attempts convert the whole episode into one slightly slow
	// login rather than a forced logout. All attempts share the one exchangeTimeout
	// budget below, so this can never extend how long a request is held.
	exchangeMaxAttempts  = 3
	exchangeRetryBackoff = 400 * time.Millisecond
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
}

// NewExchanger builds an Exchanger for an already-validated config. endpoint is the
// resolved token endpoint, so discovery stays with the OIDC client.
func NewExchanger(client *http.Client, cfg config.TokenExchangeConfig, endpoint string) *Exchanger {
	return &Exchanger{
		client:          client,
		cfg:             cfg,
		endpoint:        endpoint,
		requestedScopes: strings.Fields(cfg.Scopes),
	}
}

func (e *Exchanger) Endpoint() string           { return e.endpoint }
func (e *Exchanger) CacheEnabled() bool         { return e.cfg.CacheEnabled }
func (e *Exchanger) MinValidity() time.Duration { return e.cfg.MinValidity }

// ConfigFingerprint identifies the settings that determine what the IDP mints, so a
// cached token is not reused after they change. Credentials are excluded: they
// authenticate the BFF without altering the issued token.
func (e *Exchanger) ConfigFingerprint() string {
	return strings.Join([]string{e.cfg.GrantType, e.cfg.Audience, e.cfg.Resource, e.cfg.Scopes}, "\x1f")
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
// credential and is never logged.
//
// Only ErrExchangeUnavailable is retried: it is the sentinel for "this may succeed
// if tried again". A rejection is returned on the first attempt, since re-sending a
// token the IDP has already refused only delays the caller.
func (e *Exchanger) Exchange(ctx context.Context, subjectToken string) (*Result, error) {
	if subjectToken == "" {
		return nil, fmt.Errorf("%w: no subject token", ErrExchangeRejected)
	}

	ctx, cancel := context.WithTimeout(ctx, exchangeTimeout)
	defer cancel()

	var err error
	for attempt := 1; ; attempt++ {
		var res *Result
		res, err = e.exchangeOnce(ctx, subjectToken)
		if err == nil {
			if attempt > 1 {
				slog.Info("token exchange succeeded after retrying a transient identity provider failure",
					"attempts", attempt)
			}
			return res, nil
		}
		if !errors.Is(err, ErrExchangeUnavailable) || attempt == exchangeMaxAttempts {
			return nil, err
		}
		// Abandon the retry when the caller's deadline would expire mid-wait, so a
		// timed-out request reports the real failure instead of a context error.
		timer := time.NewTimer(exchangeRetryBackoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, err
		case <-timer.C:
		}
	}
}

// exchangeOnce performs a single exchange request.
func (e *Exchanger) exchangeOnce(ctx context.Context, subjectToken string) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint,
		strings.NewReader(e.buildForm(subjectToken).Encode()))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrExchangeUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrExchangeUnavailable, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxExchangeResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: reading response: %v", ErrExchangeUnavailable, err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, e.classifyError(res.StatusCode, body)
	}

	var tok exchangeResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("%w: decoding response: %v", ErrExchangeUnavailable, err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("%w: response carried no access_token", ErrExchangeUnavailable)
	}
	// issued_token_type is an RFC 8693 field; Entra's OBO response has none.
	if e.cfg.GrantType == config.GrantTokenExchange {
		if err := validateIssuedTokenType(tok.IssuedTokenType); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrExchangeRejected, err)
		}
	}

	claims := session.DecodeJWTClaims(tok.AccessToken)
	return &Result{
		AccessToken: tok.AccessToken,
		Expiry:      expiryFrom(tok.ExpiresIn, claims),
		Scopes:      e.grantedScopes(tok.Scope, claims),
	}, nil
}

func (e *Exchanger) buildForm(subjectToken string) url.Values {
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
	return form
}

// classifyError logs the IDP's reason and returns a sentinel. The reason stays
// internal: whether the subject or the target was refused maps out the deployment's
// trust configuration.
func (e *Exchanger) classifyError(status int, body []byte) error {
	var ee exchangeError
	_ = json.Unmarshal(body, &ee)

	attrs := []any{"status", status, "grant_type", e.cfg.GrantType, "endpoint", e.endpoint}
	if ee.Code != "" {
		attrs = append(attrs, "idp_error", ee.Code)
	}
	if ee.Description != "" {
		attrs = append(attrs, "idp_error_description", ee.Description)
	}

	if status >= http.StatusInternalServerError {
		slog.Error("token exchange failed: identity provider error", attrs...)
		return fmt.Errorf("%w: status %d", ErrExchangeUnavailable, status)
	}
	// A failed key-set fetch is the IDP's own upstream problem, not a verdict on
	// this token, even though it arrives with a 4xx code. Classifying it as a
	// rejection would destroy a valid session over a fault that clears by itself.
	if isTransientKeyFetchFailure(ee.Description) {
		slog.Warn("token exchange failed: identity provider could not reach the subject token's "+
			"issuer key set — treating as transient, the session is kept", attrs...)
		return fmt.Errorf("%w: identity provider key-set fetch failed", ErrExchangeUnavailable)
	}
	// RFC 8693 §2.2.2. An operator must fix this, so it is not a per-user warning.
	if ee.Code == "invalid_target" {
		slog.Error("token exchange rejected: audience/resource not accepted by the IDP — "+
			"register it on the exchanging application, or correct "+
			"[auth.oidc.token_exchange] audience / resource",
			append(attrs, "configured_audience", e.cfg.Audience, "configured_resource", e.cfg.Resource)...)
		return fmt.Errorf("%w: invalid_target", ErrExchangeRejected)
	}
	slog.Warn("token exchange rejected", attrs...)
	return fmt.Errorf("%w: %s", ErrExchangeRejected, ee.Code)
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
	if len(extra) > 0 {
		slog.Warn("token exchange returned scopes that were not requested — "+
			"the exchanging application may be over-provisioned at the IDP",
			"unrequested_scopes", extra)
	}
	return scopes
}

// isTransientKeyFetchFailure reports whether an IDP error description describes a
// failure to retrieve the signing key set for the subject token's issuer, rather
// than a judgement about the token itself. Matched on the description because the
// error *code* carries no such distinction — Asgardeo returns invalid_grant for
// both this and a genuinely untrusted issuer.
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

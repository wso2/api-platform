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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"ai-workspace-bff/internal/config"
)

// jwtWithClaims builds an unsigned JWT with the given claims. The BFF never verifies
// signatures (the Platform API does, via JWKS), so an unsigned token is sufficient to
// exercise the claim-decoding paths.
func jwtWithClaims(t *testing.T, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return enc(map[string]any{"alg": "none", "typ": "JWT"}) + "." + enc(claims) + ".sig"
}

// exchangeServer stands in for the IDP token endpoint, capturing the received form so
// tests can assert the exact wire format each grant produces.
type exchangeServer struct {
	*httptest.Server
	lastForm url.Values
	calls    int
}

func newExchangeServer(t *testing.T, status int, body any) *exchangeServer {
	t.Helper()
	es := &exchangeServer{}
	es.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		es.lastForm = r.PostForm
		es.calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
	t.Cleanup(es.Close)
	return es
}

func baseCfg() config.TokenExchangeConfig {
	return config.TokenExchangeConfig{
		Enabled:            true,
		GrantType:          config.GrantTokenExchange,
		ClientID:           "bff-client",
		ClientSecret:       "bff-secret",
		Audience:           "platform-api",
		Scopes:             "ap:project:read ap:gateway:read",
		SubjectTokenType:   TokenTypeJWT,
		RequestedTokenType: TokenTypeAccessToken,
		CacheEnabled:       true,
		MinValidity:        60 * time.Second,
	}
}

// TestExchangeSendsRFC8693Form pins the RFC 8693 request shape. The parameter names
// are a wire contract with every IDP in the support matrix, so a rename here is a
// breaking change and must fail a test rather than surface as an IDP rejection.
func TestExchangeSendsRFC8693Form(t *testing.T) {
	srv := newExchangeServer(t, http.StatusOK, map[string]any{
		"access_token":      "issued-token",
		"issued_token_type": TokenTypeAccessToken,
		"token_type":        "Bearer",
		"expires_in":        3600,
	})

	e := NewExchanger(srv.Client(), baseCfg(), srv.URL)
	if _, err := e.Exchange(context.Background(), "subject-token"); err != nil {
		t.Fatalf("Exchange: %v", err)
	}

	want := map[string]string{
		"grant_type":           grantURITokenExchange,
		"subject_token":        "subject-token",
		"subject_token_type":   TokenTypeJWT,
		"requested_token_type": TokenTypeAccessToken,
		"audience":             "platform-api",
		"scope":                "ap:project:read ap:gateway:read",
		"client_id":            "bff-client",
		"client_secret":        "bff-secret",
	}
	for k, v := range want {
		if got := srv.lastForm.Get(k); got != v {
			t.Errorf("form[%q] = %q, want %q", k, got, v)
		}
	}
	// resource must be absent when audience is used, or an IDP that reads both has
	// two conflicting targets to choose between.
	if _, present := srv.lastForm["resource"]; present {
		t.Error("resource must not be sent when audience is set")
	}
	// The jwt-bearer-only parameter must never leak into the RFC 8693 grant.
	if _, present := srv.lastForm["assertion"]; present {
		t.Error("assertion must not be sent for the token_exchange grant")
	}
}

// TestExchangeSendsJWTBearerForm pins Entra's on-behalf-of shape, which is a different
// specification and not a dialect of RFC 8693 — the subject travels as `assertion`,
// and there is no subject_token/audience at all.
func TestExchangeSendsJWTBearerForm(t *testing.T) {
	srv := newExchangeServer(t, http.StatusOK, map[string]any{
		"access_token": "issued-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
	})

	cfg := baseCfg()
	cfg.GrantType = config.GrantJWTBearer
	cfg.Audience = ""
	cfg.Scopes = "api://platform/.default"

	e := NewExchanger(srv.Client(), cfg, srv.URL)
	if _, err := e.Exchange(context.Background(), "subject-token"); err != nil {
		t.Fatalf("Exchange: %v", err)
	}

	if got := srv.lastForm.Get("grant_type"); got != grantURIJWTBearer {
		t.Errorf("grant_type = %q, want %q", got, grantURIJWTBearer)
	}
	if got := srv.lastForm.Get("assertion"); got != "subject-token" {
		t.Errorf("assertion = %q, want the subject token", got)
	}
	if got := srv.lastForm.Get("requested_token_use"); got != "on_behalf_of" {
		t.Errorf("requested_token_use = %q, want on_behalf_of", got)
	}
	if got := srv.lastForm.Get("scope"); got != "api://platform/.default" {
		t.Errorf("scope = %q", got)
	}
	for _, absent := range []string{"subject_token", "subject_token_type", "requested_token_type", "audience", "resource"} {
		if _, present := srv.lastForm[absent]; present {
			t.Errorf("%s must not be sent for the jwt_bearer grant", absent)
		}
	}
}

// TestExchangeSendsResourceWhenConfigured covers the IDPs that name the target with
// `resource` rather than `audience` (PingFederate reads them as distinct selectors).
func TestExchangeSendsResourceWhenConfigured(t *testing.T) {
	srv := newExchangeServer(t, http.StatusOK, map[string]any{
		"access_token":      "issued-token",
		"issued_token_type": TokenTypeAccessToken,
		"token_type":        "Bearer",
		"expires_in":        3600,
	})

	cfg := baseCfg()
	cfg.Audience = ""
	cfg.Resource = "https://platform-api.example.com"

	e := NewExchanger(srv.Client(), cfg, srv.URL)
	if _, err := e.Exchange(context.Background(), "subject-token"); err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if got := srv.lastForm.Get("resource"); got != "https://platform-api.example.com" {
		t.Errorf("resource = %q", got)
	}
	if _, present := srv.lastForm["audience"]; present {
		t.Error("audience must not be sent when resource is set")
	}
}

// TestExchangeExpiryFromExpiresIn and the exp-claim fallback below cover RFC 8693's
// expires_in being only RECOMMENDED: an IDP omitting it must not yield a token the BFF
// treats as instantly expired.
func TestExchangeExpiryFromExpiresIn(t *testing.T) {
	srv := newExchangeServer(t, http.StatusOK, map[string]any{
		"access_token":      "issued-token",
		"issued_token_type": TokenTypeAccessToken,
		"token_type":        "Bearer",
		"expires_in":        3600,
	})

	e := NewExchanger(srv.Client(), baseCfg(), srv.URL)
	res, err := e.Exchange(context.Background(), "subject-token")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if d := time.Until(res.Expiry); d < 59*time.Minute || d > 61*time.Minute {
		t.Errorf("expiry ~1h expected, got %s away", d)
	}
}

func TestExchangeExpiryFallsBackToExpClaim(t *testing.T) {
	exp := time.Now().Add(30 * time.Minute).Unix()
	issued := jwtWithClaims(t, map[string]any{"exp": exp, "scope": "ap:project:read"})
	srv := newExchangeServer(t, http.StatusOK, map[string]any{
		"access_token":      issued,
		"issued_token_type": TokenTypeAccessToken,
		"token_type":        "Bearer",
		// expires_in deliberately omitted.
	})

	e := NewExchanger(srv.Client(), baseCfg(), srv.URL)
	res, err := e.Exchange(context.Background(), "subject-token")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if res.Expiry.IsZero() {
		t.Fatal("expiry must fall back to the issued token's exp claim")
	}
	if d := time.Until(res.Expiry); d < 29*time.Minute || d > 31*time.Minute {
		t.Errorf("expiry ~30m expected, got %s away", d)
	}
}

// TestExchangeScopesPreferResponseOverClaim covers the downscoping case: RFC 8693
// makes the response's scope REQUIRED when it differs from the request, so an echoed
// value is authoritative over whatever the token's own claim says.
func TestExchangeScopesPreferResponseOverClaim(t *testing.T) {
	issued := jwtWithClaims(t, map[string]any{"scope": "ap:gateway:read ap:project:read"})
	srv := newExchangeServer(t, http.StatusOK, map[string]any{
		"access_token":      issued,
		"issued_token_type": TokenTypeAccessToken,
		"token_type":        "Bearer",
		"expires_in":        600,
		"scope":             "ap:project:read",
	})

	e := NewExchanger(srv.Client(), baseCfg(), srv.URL)
	res, err := e.Exchange(context.Background(), "subject-token")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if len(res.Scopes) != 1 || res.Scopes[0] != "ap:project:read" {
		t.Errorf("granted scopes = %v, want the downscoped response value", res.Scopes)
	}
}

func TestExchangeScopesFallBackToTokenClaim(t *testing.T) {
	issued := jwtWithClaims(t, map[string]any{"scope": "ap:project:read ap:gateway:read"})
	srv := newExchangeServer(t, http.StatusOK, map[string]any{
		"access_token":      issued,
		"issued_token_type": TokenTypeAccessToken,
		"token_type":        "Bearer",
		"expires_in":        600,
		// scope omitted — per the RFC that means the grant matched the request.
	})

	e := NewExchanger(srv.Client(), baseCfg(), srv.URL)
	res, err := e.Exchange(context.Background(), "subject-token")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if len(res.Scopes) != 2 {
		t.Errorf("granted scopes = %v, want both from the token claim", res.Scopes)
	}
}

// TestExchangeRejectsUnusableIssuedTokenType guards the configuration error where an
// IDP hands back something that cannot serve as an upstream bearer token. Better a
// failed exchange with a named cause than an opaque 401 from the Platform API.
func TestExchangeRejectsUnusableIssuedTokenType(t *testing.T) {
	for _, tc := range []struct {
		name      string
		issued    string
		wantError bool
	}{
		{"access_token", TokenTypeAccessToken, false},
		{"jwt", TokenTypeJWT, false},
		{"refresh_token", TokenTypeRefresh, true},
		{"id_token", TokenTypeIDToken, true},
		{"missing", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newExchangeServer(t, http.StatusOK, map[string]any{
				"access_token":      "issued-token",
				"issued_token_type": tc.issued,
				"token_type":        "Bearer",
				"expires_in":        600,
			})
			e := NewExchanger(srv.Client(), baseCfg(), srv.URL)
			_, err := e.Exchange(context.Background(), "subject-token")
			if tc.wantError {
				if !errors.Is(err, ErrExchangeRejected) {
					t.Errorf("err = %v, want ErrExchangeRejected", err)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestExchangeJWTBearerToleratesMissingIssuedTokenType: issued_token_type is an
// RFC 8693 field. Entra's OBO response has none, so requiring it would break the
// grant this feature added Entra support for.
func TestExchangeJWTBearerToleratesMissingIssuedTokenType(t *testing.T) {
	srv := newExchangeServer(t, http.StatusOK, map[string]any{
		"access_token": "issued-token",
		"token_type":   "Bearer",
		"expires_in":   600,
	})
	cfg := baseCfg()
	cfg.GrantType = config.GrantJWTBearer
	cfg.Audience = ""
	cfg.Scopes = "api://platform/.default"

	e := NewExchanger(srv.Client(), cfg, srv.URL)
	if _, err := e.Exchange(context.Background(), "subject-token"); err != nil {
		t.Errorf("jwt_bearer must not require issued_token_type: %v", err)
	}
}

// TestExchangeErrorClassification maps the IDP's reply onto the two sentinels, which
// is what decides whether the user is logged out (rejected) or shown a transient
// failure (unavailable). RFC 8693 §2.2.2 makes invalid_request the code for a bad
// subject token, so it must not be mistaken for a transport fault.
func TestExchangeErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    any
		wantErr error
	}{
		{"invalid_request is a rejection", http.StatusBadRequest,
			map[string]string{"error": "invalid_request"}, ErrExchangeRejected},
		{"invalid_target is a rejection", http.StatusBadRequest,
			map[string]string{"error": "invalid_target"}, ErrExchangeRejected},
		{"invalid_client is a rejection", http.StatusUnauthorized,
			map[string]string{"error": "invalid_client"}, ErrExchangeRejected},
		{"500 is unavailable, not a rejection", http.StatusInternalServerError,
			map[string]string{"error": "server_error"}, ErrExchangeUnavailable},
		{"503 is unavailable", http.StatusServiceUnavailable, nil, ErrExchangeUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newExchangeServer(t, tc.status, tc.body)
			e := NewExchanger(srv.Client(), baseCfg(), srv.URL)
			_, err := e.Exchange(context.Background(), "subject-token")
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// TestExchangeRejectsEmptyAccessToken: a 200 with no token is a malformed response,
// not a success. Without this the BFF would forward an empty bearer header upstream.
func TestExchangeRejectsEmptyAccessToken(t *testing.T) {
	srv := newExchangeServer(t, http.StatusOK, map[string]any{
		"issued_token_type": TokenTypeAccessToken,
		"token_type":        "Bearer",
	})
	e := NewExchanger(srv.Client(), baseCfg(), srv.URL)
	if _, err := e.Exchange(context.Background(), "subject-token"); !errors.Is(err, ErrExchangeUnavailable) {
		t.Errorf("err = %v, want ErrExchangeUnavailable", err)
	}
}

// TestExchangeRejectsEmptySubjectToken fails closed without a network call: there is
// nothing to exchange, and posting an empty subject_token would only waste a round
// trip to learn that.
func TestExchangeRejectsEmptySubjectToken(t *testing.T) {
	srv := newExchangeServer(t, http.StatusOK, nil)
	e := NewExchanger(srv.Client(), baseCfg(), srv.URL)
	if _, err := e.Exchange(context.Background(), ""); !errors.Is(err, ErrExchangeRejected) {
		t.Errorf("err = %v, want ErrExchangeRejected", err)
	}
	if srv.calls != 0 {
		t.Errorf("token endpoint called %d times for an empty subject token, want 0", srv.calls)
	}
}

// TestConfigFingerprintChangesWithTarget: the fingerprint is what stops a cached token
// minted for one audience being reused after the audience is reconfigured.
func TestConfigFingerprintChangesWithTarget(t *testing.T) {
	base := baseCfg()
	e1 := NewExchanger(nil, base, "https://idp.example.com/token")

	changed := base
	changed.Audience = "other-api"
	e2 := NewExchanger(nil, changed, "https://idp.example.com/token")

	if e1.ConfigFingerprint() == e2.ConfigFingerprint() {
		t.Error("fingerprint must change when the audience changes")
	}

	narrower := base
	narrower.Scopes = "ap:project:read"
	e3 := NewExchanger(nil, narrower, "https://idp.example.com/token")
	if e1.ConfigFingerprint() == e3.ConfigFingerprint() {
		t.Error("fingerprint must change when the requested scopes change")
	}

	// Credentials authenticate the BFF without altering what the IDP mints, so they
	// deliberately do not invalidate a cached token.
	sameToken := base
	sameToken.ClientSecret = "rotated-secret"
	e4 := NewExchanger(nil, sameToken, "https://idp.example.com/token")
	if e1.ConfigFingerprint() != e4.ConfigFingerprint() {
		t.Error("rotating the client secret must not invalidate cached tokens")
	}
}

// TestExchangeNeverLogsTokens is a guard on GO-AUTH-003: the subject token is a live
// credential and must not reach a log line. It checks the error text, which is the
// value most likely to be logged verbatim by a caller.
func TestExchangeErrorsDoNotContainTokens(t *testing.T) {
	const secret = "super-secret-subject-token"
	srv := newExchangeServer(t, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
	e := NewExchanger(srv.Client(), baseCfg(), srv.URL)
	_, err := e.Exchange(context.Background(), secret)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error text leaks the subject token: %q", err.Error())
	}
}

// TestExchangeRetriesTransientKeyFetchFailure: Asgardeo reports a failed JWKS fetch
// as invalid_grant — a 4xx for a fault that is neither the caller's nor permanent.
func TestExchangeRetriesTransientKeyFetchFailure(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls < 3 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": "invalid_grant",
				"error_description": "Error occurred while accessing remote JWKS endpoint: " +
					"https://idp.example.com/t/org-a/oauth2/jwks",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":      "exchanged-token",
			"issued_token_type": TokenTypeAccessToken,
			"expires_in":        3600,
		})
	}))
	t.Cleanup(srv.Close)

	ex := NewExchanger(srv.Client(), baseCfg(), srv.URL)
	res, err := ex.Exchange(context.Background(), jwtWithClaims(t, map[string]any{"sub": "u1"}))
	if err != nil {
		t.Fatalf("expected the exchange to succeed after retrying, got %v", err)
	}
	if res.AccessToken != "exchanged-token" {
		t.Errorf("access token = %q, want %q", res.AccessToken, "exchanged-token")
	}
	if calls != 3 {
		t.Errorf("server calls = %d, want 3 (two transient failures then success)", calls)
	}
}

// TestExchangeTransientKeyFetchFailureIsUnavailable pins the sentinel: the caller
// destroys the session on ErrExchangeRejected, so misclassifying logs the user out.
func TestExchangeTransientKeyFetchFailureIsUnavailable(t *testing.T) {
	srv := newExchangeServer(t, http.StatusBadRequest, map[string]any{
		"error":             "invalid_grant",
		"error_description": "Error occurred while accessing remote JWKS endpoint: https://idp.example.com/jwks",
	})

	ex := NewExchanger(srv.Client(), baseCfg(), srv.URL)
	_, err := ex.Exchange(context.Background(), jwtWithClaims(t, map[string]any{"sub": "u1"}))
	if !errors.Is(err, ErrExchangeUnavailable) {
		t.Fatalf("error = %v, want ErrExchangeUnavailable (session must survive a transient IDP fault)", err)
	}
	if errors.Is(err, ErrExchangeRejected) {
		t.Error("a transient key-set fetch failure must never classify as a rejection")
	}
	if srv.calls != exchangeMaxAttempts {
		t.Errorf("server calls = %d, want %d", srv.calls, exchangeMaxAttempts)
	}
}

// TestExchangeDoesNotRetryRejection: re-sending a refused token only delays the 401.
func TestExchangeDoesNotRetryRejection(t *testing.T) {
	srv := newExchangeServer(t, http.StatusBadRequest, map[string]any{
		"error":             "invalid_grant",
		"error_description": "Error while parsing the JWT",
	})

	ex := NewExchanger(srv.Client(), baseCfg(), srv.URL)
	_, err := ex.Exchange(context.Background(), jwtWithClaims(t, map[string]any{"sub": "u1"}))
	if !errors.Is(err, ErrExchangeRejected) {
		t.Fatalf("error = %v, want ErrExchangeRejected", err)
	}
	if srv.calls != 1 {
		t.Errorf("server calls = %d, want 1 (a rejection must not be retried)", srv.calls)
	}
}

// TestIsTransientKeyFetchFailure: catch key-fetch wording, not verdicts about the token.
func TestIsTransientKeyFetchFailure(t *testing.T) {
	transient := []string{
		"Error occurred while accessing remote JWKS endpoint: https://idp.example.com/jwks",
		"Unable to retrieve the JWKS for the issuer",
		"Failed to fetch key set from the identity provider",
	}
	for _, d := range transient {
		if !isTransientKeyFetchFailure(d) {
			t.Errorf("isTransientKeyFetchFailure(%q) = false, want true", d)
		}
	}
	permanent := []string{
		"Error while parsing the JWT",
		"No Registered IDP found for the JWT with issuer name : https://idp.example.com/t/org-a/oauth2/token",
		"Invalid audience values provided",
		"Signature verification failed for the JWKS key",
		"",
	}
	for _, d := range permanent {
		if isTransientKeyFetchFailure(d) {
			t.Errorf("isTransientKeyFetchFailure(%q) = true, want false", d)
		}
	}
}

// TestExchangeTransientHTTPStatuses: 408/429 behave like 5xx, so the session survives.
func TestExchangeTransientHTTPStatuses(t *testing.T) {
	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway} {
		srv := newExchangeServer(t, status, map[string]any{"error": "temporarily_unavailable"})
		ex := NewExchanger(srv.Client(), baseCfg(), srv.URL)
		_, err := ex.Exchange(context.Background(), jwtWithClaims(t, map[string]any{"sub": "u1"}))
		if !errors.Is(err, ErrExchangeUnavailable) {
			t.Errorf("status %d: error = %v, want ErrExchangeUnavailable", status, err)
		}
	}
	// The neighbouring 4xx must still be a rejection — the split has to stay narrow.
	srv := newExchangeServer(t, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
	ex := NewExchanger(srv.Client(), baseCfg(), srv.URL)
	if _, err := ex.Exchange(context.Background(), jwtWithClaims(t, map[string]any{"sub": "u1"})); !errors.Is(err, ErrExchangeRejected) {
		t.Errorf("status 400: error = %v, want ErrExchangeRejected", err)
	}
}

// TestExchangeErrorDescriptionIsRedacted covers GO-AUTH-003 for the one field the
// BFF copies from the IDP into its own log.
func TestExchangeErrorDescriptionIsRedacted(t *testing.T) {
	cfg := baseCfg()
	subject := jwtWithClaims(t, map[string]any{"sub": "u1"})
	srv := newExchangeServer(t, http.StatusBadRequest, map[string]any{
		"error": "invalid_grant",
		"error_description": "Error while parsing the JWT: " + subject +
			" presented with client_secret " + cfg.ClientSecret,
	})

	var logged bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ex := NewExchanger(srv.Client(), cfg, srv.URL)
	if _, err := ex.Exchange(context.Background(), subject); !errors.Is(err, ErrExchangeRejected) {
		t.Fatalf("error = %v, want ErrExchangeRejected", err)
	}

	out := logged.String()
	if strings.Contains(out, subject) {
		t.Error("the subject token reached the log via idp_error_description")
	}
	if strings.Contains(out, cfg.ClientSecret) {
		t.Error("the client secret reached the log via idp_error_description")
	}
	// The diagnostic text must survive the redaction.
	if !strings.Contains(out, "Error while parsing the JWT") {
		t.Error("redaction removed the diagnostic text, not just the credentials")
	}
}

// TestExchangeRedactsOpaqueSubjectToken: an opaque token has no matchable shape, so
// redaction must key on the value actually sent.
func TestExchangeRedactsOpaqueSubjectToken(t *testing.T) {
	const opaque = "a1b2c3d4-1f1a-4626-865d-cf49437da24d"
	srv := newExchangeServer(t, http.StatusBadRequest, map[string]any{
		"error":             "invalid_grant",
		"error_description": "Token " + opaque + " could not be resolved",
	})

	var logged bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ex := NewExchanger(srv.Client(), baseCfg(), srv.URL)
	if _, err := ex.Exchange(context.Background(), opaque); !errors.Is(err, ErrExchangeRejected) {
		t.Fatalf("error = %v, want ErrExchangeRejected", err)
	}
	if out := logged.String(); strings.Contains(out, opaque) {
		t.Error("an opaque subject token reached the log via idp_error_description")
	}
}

// TestExchangeDoesNotFollowRedirects: Go replays a POST body on 307/308, so a
// redirect would hand the client secret and subject token to another host.
func TestExchangeDoesNotFollowRedirects(t *testing.T) {
	var redirectTargetCalls int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "leaked", "token_type": "Bearer"})
	}))
	t.Cleanup(target.Close)

	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(idp.Close)

	ex := NewExchanger(idp.Client(), baseCfg(), idp.URL)
	_, err := ex.Exchange(context.Background(), jwtWithClaims(t, map[string]any{"sub": "u1"}))
	if err == nil {
		t.Fatal("a redirected exchange must fail, not silently succeed against another host")
	}
	if redirectTargetCalls != 0 {
		t.Errorf("redirect target received %d requests, want 0 — credentials were replayed", redirectTargetCalls)
	}
}

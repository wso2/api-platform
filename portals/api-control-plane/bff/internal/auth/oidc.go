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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"api-control-plane-bff/internal/session"
)

// discoveryDoc is the subset of the OIDC discovery document the BFF needs.
type discoveryDoc struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

// tokenResponse is the IDP token endpoint response.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// txn is an in-flight authorization request, bound to the browser by the tx cookie.
type txn struct {
	State        string
	Nonce        string
	CodeVerifier string
	ReturnURL    string
	Expiry       time.Time
}

// OIDCOptions configures an OIDC authenticator. Every field here exists to
// keep this authenticator usable against IdPs that don't fit the
// standards-conformant assumption a naive OIDC client makes — in particular an
// IdP whose issuer is a bare name rather than a URL, or one that publishes no
// (or a non-standard) discovery document. None of this is IdP-specific code:
// it is the generic configuration surface, set entirely from
// config.OIDCConfig.
type OIDCOptions struct {
	// Authority is the discovery base URL, consulted only when Discovery is
	// true.
	Authority string
	// Issuer is compared against a discovery document's own "issuer" field
	// when discovery is used (never against a URL-parsed form — some IdPs
	// issue a bare-name issuer, so this is a plain string comparison against
	// the configured value, not a conformance check).
	Issuer string
	// Discovery, when true, fetches {Authority}/.well-known/openid-configuration
	// and uses its endpoints, validating disco.Issuer == Issuer (skipped when
	// disco.Issuer is empty — not every IdP's discovery document sets one).
	// When false, every *Endpoint field below is used as-is and no discovery
	// call is made.
	Discovery bool

	AuthorizationEndpoint string
	TokenEndpoint         string
	EndSessionEndpoint    string // optional even when set; logout degrades gracefully without it

	ClientID     string
	ClientSecret string
	// ClientAuthMethod is "client_secret_post" (default), "client_secret_basic",
	// or "none" for a public client driven server-side by this BFF — PKCE alone
	// protects that flow, and the browser still never sees a token.
	ClientAuthMethod string

	RedirectURL           string
	PostLogoutRedirectURL string
	Scopes                string
}

// OIDC implements the authorization-code flow with PKCE using only net/http,
// against a confidential or public client as configured. The BFF holds the
// client secret (when there is one) and performs the code/token exchange; the
// browser never contacts the IDP token endpoint and never holds a token. Note:
// per design the BFF does NOT cryptographically verify the id_token — the
// Platform API (or the gateway in front of it) validates the access token via
// JWKS. The id_token is decoded only to populate the session's display claims;
// state+nonce binding and PKCE still protect the login flow itself.
type OIDC struct {
	client  *http.Client
	opts    OIDCOptions
	disco   discoveryDoc
	mapping session.ClaimMapping
	absTTL  time.Duration

	mu        sync.Mutex
	txs       map[string]*txn
	done      chan struct{}
	closeOnce sync.Once
}

// discoveryTimeout bounds the startup discovery call so an unreachable issuer
// fails fast rather than blocking initialization for the upstream client's full
// (longer) request timeout.
const discoveryTimeout = 15 * time.Second

// NewOIDC returns a ready authenticator. When opts.Discovery is true it fetches
// the discovery document up front; otherwise it uses opts' explicit endpoints
// as-is and makes no network call.
func NewOIDC(ctx context.Context, client *http.Client, opts OIDCOptions, mapping session.ClaimMapping, absTTL time.Duration) (*OIDC, error) {
	disco := discoveryDoc{
		AuthorizationEndpoint: opts.AuthorizationEndpoint,
		TokenEndpoint:         opts.TokenEndpoint,
		EndSessionEndpoint:    opts.EndSessionEndpoint,
	}
	if opts.Discovery {
		discCtx, cancel := context.WithTimeout(ctx, discoveryTimeout)
		defer cancel()
		fetched, err := fetchDiscovery(discCtx, client, opts.Authority, opts.Issuer)
		if err != nil {
			return nil, err
		}
		disco = fetched
	}
	if disco.AuthorizationEndpoint == "" || disco.TokenEndpoint == "" {
		return nil, fmt.Errorf("oidc: authorization_endpoint and token_endpoint are required (from discovery or explicit config)")
	}

	o := &OIDC{
		client:  client,
		opts:    opts,
		disco:   disco,
		mapping: mapping,
		absTTL:  absTTL,
		txs:     make(map[string]*txn),
		done:    make(chan struct{}),
	}
	go o.sweepTxns()
	return o, nil
}

// Close stops the background transaction sweeper. Safe to call multiple times.
func (o *OIDC) Close() {
	o.closeOnce.Do(func() { close(o.done) })
}

// fetchDiscovery fetches the discovery document at authority and validates its
// "issuer" against the expected value — as a plain string comparison, not a
// URL-conformance check, and skipped entirely when the document omits
// "issuer" (some IdPs don't set it). Deliberately does NOT require issuer to
// look like a URL: some IdPs (e.g. WSO2 Thunder) issue a bare name.
func fetchDiscovery(ctx context.Context, client *http.Client, authority, expectedIssuer string) (discoveryDoc, error) {
	u := strings.TrimRight(authority, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return discoveryDoc{}, err
	}
	res, err := client.Do(req)
	if err != nil {
		return discoveryDoc{}, fmt.Errorf("oidc discovery failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return discoveryDoc{}, fmt.Errorf("oidc discovery returned status %d", res.StatusCode)
	}
	var d discoveryDoc
	if err := json.NewDecoder(res.Body).Decode(&d); err != nil {
		return discoveryDoc{}, fmt.Errorf("oidc discovery decode failed: %w", err)
	}
	if d.AuthorizationEndpoint == "" || d.TokenEndpoint == "" {
		return discoveryDoc{}, fmt.Errorf("oidc discovery missing required endpoints")
	}
	if d.Issuer != "" && expectedIssuer != "" && d.Issuer != expectedIssuer {
		return discoveryDoc{}, fmt.Errorf("oidc discovery issuer %q does not match configured issuer %q", d.Issuer, expectedIssuer)
	}
	return d, nil
}

// maxPendingTxns caps in-flight authorization requests so an unauthenticated
// flood of GET /api/auth/login cannot grow o.txs without bound and exhaust
// process memory. Each entry is small and lives at most 10 minutes; this
// ceiling is generous for real traffic while still being a real ceiling.
const maxPendingTxns = 10000

// ErrTooManyPendingLogins indicates the in-flight transaction limit was
// reached even after sweeping expired entries.
type ErrTooManyPendingLogins struct{}

func (ErrTooManyPendingLogins) Error() string { return "too many pending logins" }

// AuthCodeURL creates a new login transaction and returns the IDP authorize URL
// plus the opaque tx id to store in the short-lived tx cookie.
func (o *OIDC) AuthCodeURL(returnURL string) (authURL, txID string, err error) {
	state, err := randString(32)
	if err != nil {
		return "", "", err
	}
	nonce, err := randString(32)
	if err != nil {
		return "", "", err
	}
	verifier, err := randString(48)
	if err != nil {
		return "", "", err
	}
	txID, err = randString(32)
	if err != nil {
		return "", "", err
	}

	o.mu.Lock()
	if len(o.txs) >= maxPendingTxns {
		o.sweepLocked(time.Now())
	}
	if len(o.txs) >= maxPendingTxns {
		o.mu.Unlock()
		return "", "", ErrTooManyPendingLogins{}
	}
	o.txs[txID] = &txn{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: verifier,
		ReturnURL:    returnURL,
		Expiry:       time.Now().Add(10 * time.Minute),
	}
	o.mu.Unlock()

	challenge := pkceChallenge(verifier)
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {o.opts.ClientID},
		"redirect_uri":          {o.opts.RedirectURL},
		"scope":                 {o.opts.Scopes},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return o.disco.AuthorizationEndpoint + "?" + q.Encode(), txID, nil
}

// ErrStateMismatch indicates a callback whose state didn't match the tx record.
type ErrStateMismatch struct{}

func (ErrStateMismatch) Error() string { return "oidc state mismatch" }

// ErrNonceMismatch indicates the id_token's nonce didn't match the tx record.
type ErrNonceMismatch struct{}

func (ErrNonceMismatch) Error() string { return "oidc nonce mismatch" }

// ErrIDTokenBinding indicates the id_token's issuer or audience did not match
// the configured expectation.
type ErrIDTokenBinding struct{}

func (ErrIDTokenBinding) Error() string { return "oidc id_token issuer/audience mismatch" }

// validateIDTokenBinding checks iss and aud against the configured issuer and
// client ID. A claim the IdP omits is not enforced, matching the existing
// tolerance elsewhere for IdPs that publish an incomplete discovery document.
func (o *OIDC) validateIDTokenBinding(claims map[string]any) error {
	if want := o.opts.Issuer; want != "" {
		if got, ok := claims["iss"].(string); ok && got != want {
			return ErrIDTokenBinding{}
		}
	}
	if want := o.opts.ClientID; want != "" {
		switch aud := claims["aud"].(type) {
		case string:
			if aud != want {
				return ErrIDTokenBinding{}
			}
		case []any:
			for _, a := range aud {
				if s, ok := a.(string); ok && s == want {
					return nil
				}
			}
			return ErrIDTokenBinding{}
		}
	}
	return nil
}

// Callback validates the tx/state, exchanges the code for tokens, and returns a
// populated session plus the sanitized return URL. txID comes from the tx cookie.
func (o *OIDC) Callback(ctx context.Context, txID, state, code string) (*session.Session, string, error) {
	o.mu.Lock()
	tx, ok := o.txs[txID]
	if ok {
		delete(o.txs, txID)
	}
	o.mu.Unlock()

	if !ok || tx.Expiry.Before(time.Now()) || tx.State != state {
		return nil, "", ErrStateMismatch{}
	}

	tok, err := o.exchange(ctx, code, tx.CodeVerifier)
	if err != nil {
		return nil, "", err
	}

	// Bind the id_token to this login by verifying its nonce before trusting any
	// of its claims. The id_token comes from the BFF's own back-channel exchange
	// (not the browser); the nonce check still rejects replayed/injected tokens.
	idClaims := session.DecodeJWTClaims(tok.IDToken)
	if n, _ := idClaims["nonce"].(string); n != tx.Nonce {
		return nil, "", ErrNonceMismatch{}
	}
	if err := o.validateIDTokenBinding(idClaims); err != nil {
		return nil, "", err
	}

	sess := o.sessionFromToken(tok)
	return sess, tx.ReturnURL, nil
}

func (o *OIDC) exchange(ctx context.Context, code, verifier string) (*tokenResponse, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {o.opts.RedirectURL},
		"client_id":     {o.opts.ClientID},
		"code_verifier": {verifier},
	}
	return o.postToken(ctx, form)
}

// Refresh exchanges a refresh token for a fresh token set (with rotation).
func (o *OIDC) Refresh(ctx context.Context, refreshToken string) (*tokenResponse, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {o.opts.ClientID},
		"scope":         {o.opts.Scopes},
	}
	return o.postToken(ctx, form)
}

// postToken authenticates the client per ClientAuthMethod and posts to the
// token endpoint. "none" (a public client) sends no credential at all — PKCE
// is what protects the flow. "client_secret_basic" sends it as HTTP Basic
// auth; the default, "client_secret_post", puts it in the form body.
func (o *OIDC) postToken(ctx context.Context, form url.Values) (*tokenResponse, error) {
	switch o.opts.ClientAuthMethod {
	case "client_secret_basic", "none":
		// handled below via header / omission
	default: // "client_secret_post" and any unrecognized value (validated at config load)
		form.Set("client_secret", o.opts.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.disco.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if o.opts.ClientAuthMethod == "client_secret_basic" {
		req.SetBasicAuth(url.QueryEscape(o.opts.ClientID), url.QueryEscape(o.opts.ClientSecret))
	}

	res, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token endpoint request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned status %d", res.StatusCode)
	}
	var tok tokenResponse
	if err := json.NewDecoder(res.Body).Decode(&tok); err != nil {
		return nil, fmt.Errorf("token endpoint decode failed: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("token endpoint returned no access_token")
	}
	return &tok, nil
}

// SessionFromToken builds a session from a refreshed token set, preserving the
// previous refresh/id token when the IDP omits them on refresh.
func (o *OIDC) SessionFromToken(tok *tokenResponse, prev *session.Session) *session.Session {
	s := o.sessionFromToken(tok)
	if s.RefreshToken == "" && prev != nil {
		s.RefreshToken = prev.RefreshToken
	}
	if s.IDToken == "" && prev != nil {
		s.IDToken = prev.IDToken
	}
	return s
}

// UserFromAccessToken decodes the access token's claims (without verifying) and
// maps them to a display User. Used as a fallback when no stored session entry
// is available (e.g. after a BFF restart), so id_token-only claims may be absent.
func (o *OIDC) UserFromAccessToken(accessToken string) session.User {
	return session.UserFromClaims(session.DecodeJWTClaims(accessToken), nil, o.mapping)
}

func (o *OIDC) sessionFromToken(tok *tokenResponse) *session.Session {
	atClaims := session.DecodeJWTClaims(tok.AccessToken)
	idClaims := session.DecodeJWTClaims(tok.IDToken)

	// Prefer the token response's own expires_in over decoding the access
	// token — it need not be a JWT at all (an opaque token is valid OAuth2).
	accessExpiry := session.ExpiryFromClaims(atClaims)
	if tok.ExpiresIn > 0 {
		accessExpiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}

	abs := time.Now().Add(o.absTTL)
	return &session.Session{
		Mode:              session.ModeOIDC,
		AccessToken:       tok.AccessToken,
		RefreshToken:      tok.RefreshToken,
		IDToken:           tok.IDToken,
		AccessExpiry:      accessExpiry,
		AbsoluteExpiry:    abs,
		MaxAbsoluteExpiry: abs,
		User:              session.UserFromClaims(atClaims, idClaims, o.mapping),
	}
}

// LogoutURL returns the RP-initiated end-session URL, or the post-logout URL
// directly when the IDP has no end_session_endpoint configured.
func (o *OIDC) LogoutURL(idToken string) string {
	if o.disco.EndSessionEndpoint == "" {
		return o.opts.PostLogoutRedirectURL
	}
	q := url.Values{}
	if idToken != "" {
		q.Set("id_token_hint", idToken)
	}
	if o.opts.PostLogoutRedirectURL != "" {
		q.Set("post_logout_redirect_uri", o.opts.PostLogoutRedirectURL)
	}
	q.Set("client_id", o.opts.ClientID)
	return o.disco.EndSessionEndpoint + "?" + q.Encode()
}

func (o *OIDC) sweepTxns() {
	t := time.NewTicker(2 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-o.done:
			return
		case now := <-t.C:
			o.mu.Lock()
			o.sweepLocked(now)
			o.mu.Unlock()
		}
	}
}

// sweepLocked removes expired transactions. The caller must hold o.mu.
func (o *OIDC) sweepLocked(now time.Time) {
	for id, tx := range o.txs {
		if tx.Expiry.Before(now) {
			delete(o.txs, id)
		}
	}
}

func randString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

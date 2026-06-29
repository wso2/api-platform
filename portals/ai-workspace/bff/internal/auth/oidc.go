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

	"ai-workspace-bff/internal/session"
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

// OIDC implements the confidential authorization-code flow with PKCE using only
// net/http. The BFF holds the client secret and performs the code/token
// exchange; the browser never contacts the IDP token endpoint and never holds a
// token. Note: per design the BFF does NOT cryptographically verify the
// id_token — the Platform API validates the access token via JWKS. The id_token
// is decoded only to populate the session's display claims; state+nonce binding
// and PKCE still protect the login flow itself.
type OIDC struct {
	client                *http.Client
	clientID              string
	clientSecret          string
	redirectURL           string
	postLogoutRedirectURL string
	scopes                string
	disco                 discoveryDoc
	mapping               session.ClaimMapping
	absTTL                time.Duration

	mu        sync.Mutex
	txs       map[string]*txn
	done      chan struct{}
	closeOnce sync.Once
}

// discoveryTimeout bounds the startup discovery call so an unreachable issuer
// fails fast rather than blocking initialization for the upstream client's full
// (longer) request timeout.
const discoveryTimeout = 15 * time.Second

// NewOIDC fetches the discovery document and returns a ready authenticator.
func NewOIDC(
	ctx context.Context,
	client *http.Client,
	issuer, clientID, clientSecret, redirectURL, postLogoutRedirectURL, scopes string,
	mapping session.ClaimMapping,
	absTTL time.Duration,
) (*OIDC, error) {
	discCtx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()
	disco, err := fetchDiscovery(discCtx, client, issuer)
	if err != nil {
		return nil, err
	}
	o := &OIDC{
		client:                client,
		clientID:              clientID,
		clientSecret:          clientSecret,
		redirectURL:           redirectURL,
		postLogoutRedirectURL: postLogoutRedirectURL,
		scopes:                scopes,
		disco:                 disco,
		mapping:               mapping,
		absTTL:                absTTL,
		txs:                   make(map[string]*txn),
		done:                  make(chan struct{}),
	}
	go o.sweepTxns()
	return o, nil
}

// Close stops the background transaction sweeper. Safe to call multiple times.
func (o *OIDC) Close() {
	o.closeOnce.Do(func() { close(o.done) })
}

func fetchDiscovery(ctx context.Context, client *http.Client, issuer string) (discoveryDoc, error) {
	u := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
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
	return d, nil
}

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
		"client_id":             {o.clientID},
		"redirect_uri":          {o.redirectURL},
		"scope":                 {o.scopes},
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

	sess := o.sessionFromToken(tok)
	return sess, tx.ReturnURL, nil
}

func (o *OIDC) exchange(ctx context.Context, code, verifier string) (*tokenResponse, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {o.redirectURL},
		"client_id":     {o.clientID},
		"client_secret": {o.clientSecret},
		"code_verifier": {verifier},
	}
	return o.postToken(ctx, form)
}

// Refresh exchanges a refresh token for a fresh token set (with rotation).
func (o *OIDC) Refresh(ctx context.Context, refreshToken string) (*tokenResponse, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {o.clientID},
		"client_secret": {o.clientSecret},
		"scope":         {o.scopes},
	}
	return o.postToken(ctx, form)
}

func (o *OIDC) postToken(ctx context.Context, form url.Values) (*tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.disco.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

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

	accessExpiry := session.ExpiryFromClaims(atClaims)
	if tok.ExpiresIn > 0 {
		accessExpiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}

	abs := time.Now().Add(o.absTTL)
	return &session.Session{
		Mode:           session.ModeOIDC,
		AccessToken:    tok.AccessToken,
		RefreshToken:   tok.RefreshToken,
		IDToken:        tok.IDToken,
		AccessExpiry:   accessExpiry,
		AbsoluteExpiry: abs,
		User:           session.UserFromClaims(atClaims, idClaims, o.mapping),
	}
}

// LogoutURL returns the RP-initiated end-session URL, or the post-logout URL
// directly when the IDP has no end_session_endpoint.
func (o *OIDC) LogoutURL(idToken string) string {
	if o.disco.EndSessionEndpoint == "" {
		return o.postLogoutRedirectURL
	}
	q := url.Values{}
	if idToken != "" {
		q.Set("id_token_hint", idToken)
	}
	if o.postLogoutRedirectURL != "" {
		q.Set("post_logout_redirect_uri", o.postLogoutRedirectURL)
	}
	q.Set("client_id", o.clientID)
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
			for id, tx := range o.txs {
				if tx.Expiry.Before(now) {
					delete(o.txs, id)
				}
			}
			o.mu.Unlock()
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

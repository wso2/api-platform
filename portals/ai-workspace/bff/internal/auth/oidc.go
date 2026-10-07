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
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ai-workspace-bff/internal/config"
	"ai-workspace-bff/internal/secure"
	"ai-workspace-bff/internal/session"
)

// discoveryDoc is the subset of the OIDC discovery document the BFF needs.
type discoveryDoc struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
	RevocationEndpoint    string `json:"revocation_endpoint"`
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

	// sealer seals the in-flight login transaction into the tx cookie. There is no
	// in-process alternative: the callback is a fresh browser navigation that any
	// replica may receive.
	sealer *secure.Sealer
}

// discoveryTimeout bounds the startup discovery call so an unreachable issuer
// fails fast rather than blocking initialization for the upstream client's full
// (longer) request timeout.
const discoveryTimeout = 15 * time.Second

// TxTTL is how long a login transaction stays valid — the wall-clock budget for
// everything the user does at the IDP: typing credentials, MFA, an account or org
// picker, a password reset mid-flow, or simply leaving the tab for a while. Exceed
// it and the callback cannot be matched, which the user sees as a failed login with
// no explanation.
//
// Half an hour rather than a few minutes because the cost of being generous is one
// small map entry per in-flight login, while the cost of being tight is a real
// person's login failing for taking too long over MFA. It is not what protects the
// flow: state+nonce binding, PKCE, and one-shot consumption do, and a transaction
// buys nothing without the code the IDP hands back.
//
// Exported because the callback reports on it, and because the tx cookie's lifetime
// is derived from it — see TxCookieTTL.
const TxTTL = 30 * time.Minute

// expiredRetention is how long past its validity the tx cookie is kept, purely so the
// callback can say "expired" instead of "no cookie at all". The two look identical to
// a user and completely different to whoever is debugging it. Such a transaction is
// never accepted — Callback checks Expiry before State.
const expiredRetention = 2 * time.Hour

// TxCookieTTL is how long the browser keeps the login-transaction cookie. It
// deliberately outlives the transaction's validity by expiredRetention: validity is
// still governed by TxTTL (Callback checks Expiry and rejects anything past it), but a
// cookie that died with the transaction would turn every aged-out login into "no cookie
// at all" — a Path/SameSite-shaped fault — instead of the "expired" the server can
// still report while the cookie is there to read.
const TxCookieTTL = TxTTL + expiredRetention

// NewOIDC fetches the discovery document and returns a ready authenticator.
func NewOIDC(
	ctx context.Context,
	client *http.Client,
	issuer, clientID, clientSecret, redirectURL, postLogoutRedirectURL, scopes string,
	mapping session.ClaimMapping,
	absTTL time.Duration,
	txSealer *secure.Sealer,
) (*OIDC, error) {
	discCtx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()
	disco, err := fetchDiscovery(discCtx, noRedirectClient(client), issuer)
	if err != nil {
		return nil, err
	}
	if txSealer == nil {
		return nil, fmt.Errorf("oidc: a login-transaction sealer is required")
	}
	o := &OIDC{
		// noRedirectClient: a 307/308 from the IDP would otherwise re-send the POST
		// body to the redirect target, and that body carries the client secret and,
		// on revocation, the refresh token.
		client:                noRedirectClient(client),
		clientID:              clientID,
		clientSecret:          clientSecret,
		redirectURL:           redirectURL,
		postLogoutRedirectURL: postLogoutRedirectURL,
		scopes:                scopes,
		disco:                 disco,
		mapping:               mapping,
		absTTL:                absTTL,
		sealer:                txSealer,
	}
	return o, nil
}

// Close releases background resources. There are none — login transactions live in
// the client's cookie, not in this process — but it stays so callers need not know
// that, and so a future resource has somewhere to be released.
func (o *OIDC) Close() {}

// SupportsRevocation reports whether the issuer advertises a revocation endpoint.
// Without one, logout cannot invalidate a refresh token a copied cookie still carries.
func (o *OIDC) SupportsRevocation() bool { return o.disco.RevocationEndpoint != "" }

// TokenEndpoint is the endpoint discovered from the issuer. Exposed so a token
// exchange configured without an explicit endpoint override can post to the same
// IDP the user logged in to, without repeating discovery.
func (o *OIDC) TokenEndpoint() string { return o.disco.TokenEndpoint }

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

	// Only the two endpoints this BFF posts credentials to. The authorization and
	// end-session endpoints are browser redirects and carry nothing of ours.
	if err := requireSecureEndpoint("token_endpoint", d.TokenEndpoint); err != nil {
		return discoveryDoc{}, err
	}
	// Optional, so a bad one is dropped rather than failing startup: logout still
	// works without it, and SupportsRevocation then reports the loss.
	if d.RevocationEndpoint != "" {
		if err := requireSecureEndpoint("revocation_endpoint", d.RevocationEndpoint); err != nil {
			slog.Warn("ignoring the issuer's revocation_endpoint", "err", err)
			d.RevocationEndpoint = ""
		}
	}
	return d, nil
}

// requireSecureEndpoint rejects an advertised endpoint this BFF would send the client
// secret (and, for revocation, the refresh token) to in cleartext. Loopback is exempt:
// the request never reaches a network there. Mirrors the check config.validate already
// applies to an explicitly configured token_endpoint.
func requireSecureEndpoint(name, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("oidc discovery returned an unusable %s", name)
	}
	if u.User != nil {
		return fmt.Errorf("oidc discovery returned a %s containing userinfo", name)
	}
	if u.Scheme == "http" && !config.IsLoopbackHost(u.Host) {
		return fmt.Errorf("oidc discovery returned a plaintext %s: the client secret is sent "+
			"in the request body, so it must be https://", name)
	}
	return nil
}

// AuthCodeURL creates a new login transaction and returns the IDP authorize URL.
//
// `extra` carries additional authorization-request parameters the caller wants the IDP
// to see — `fidp` to name a federated provider, `login_hint` to prefill the account —
// so a portal can put the provider choice on its OWN page and send the user straight
// to Google or GitHub instead of through the IDP's chooser. The caller is responsible
// for deciding which parameters are allowed (see the server's handleOIDCLogin): a
// parameter set here CANNOT override the protocol ones below, which are written after
// it precisely so that a stray `redirect_uri` or `scope` cannot widen the request.
// plus the opaque tx id to store in the short-lived tx cookie.
func (o *OIDC) AuthCodeURL(returnURL string, extra url.Values) (authURL, txID string, err error) {
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

	record := &txn{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: verifier,
		ReturnURL:    returnURL,
		Expiry:       time.Now().Add(TxTTL),
	}
	// Sealed: the browser must not read the PKCE verifier inside, which is exactly the
	// secret PKCE exists to keep from it.
	if txID, err = o.sealTxn(record); err != nil {
		return "", "", err
	}

	challenge := pkceChallenge(verifier)
	// Seeded with the caller's extras, then the protocol parameters are assigned over
	// the top: whatever `extra` contains, it can never change response_type,
	// client_id, redirect_uri, scope, state, nonce or the PKCE challenge.
	q := url.Values{}
	for name, values := range extra {
		if len(values) > 0 && values[0] != "" {
			q.Set(name, values[0])
		}
	}
	for name, value := range map[string]string{
		"response_type":         "code",
		"client_id":             o.clientID,
		"redirect_uri":          o.redirectURL,
		"scope":                 o.scopes,
		"state":                 state,
		"nonce":                 nonce,
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	} {
		q.Set(name, value)
	}
	return o.disco.AuthorizationEndpoint + "?" + q.Encode(), txID, nil
}

// ErrStateMismatch indicates a callback that could not be tied back to the login
// this server started. Reason names WHICH of the four checks failed — they have very
// different causes and fixes, and a single "state mismatch" string sends an operator
// hunting for an attack when the usual explanation is a restarted process or a cookie
// the browser never sent.
//
// The reason is for logs only. The browser is redirected with a generic auth_failed
// either way: telling a caller which half of the check it failed is a probing oracle.
type ErrStateMismatch struct{ Reason string }

func (e ErrStateMismatch) Error() string {
	if e.Reason == "" {
		return "oidc state mismatch"
	}
	return "oidc state mismatch: " + e.Reason
}

// Reasons an OIDC callback cannot be matched to a login transaction.
const (
	// The tx cookie was present but did not open: it was sealed under a different
	// [session] encryption_key (a key change, or a replica configured differently from
	// its siblings), or it was truncated or altered in transit.
	ReasonNoTransaction = "the login-transaction cookie could not be opened (sealed under " +
		"a different encryption_key, or altered in transit)"
	// The tx cookie never arrived: its Path does not cover the callback route, the
	// browser dropped it (SameSite, Secure over plain http), or the user opened the
	// callback URL directly.
	ReasonNoTxCookie = "request carried no login-transaction cookie"
	// Older than the 10-minute window: the user sat on the IDP's login page.
	ReasonExpired = "login transaction expired"
	// Everything was present and the state still differed — the one case that is
	// genuinely suspicious.
	ReasonStateDiffers = "state parameter does not match the stored transaction"
)

// ErrNonceMismatch indicates the id_token's nonce didn't match the tx record.
type ErrNonceMismatch struct{}

func (ErrNonceMismatch) Error() string { return "oidc nonce mismatch" }

// Callback validates the tx/state, exchanges the code for tokens, and returns a
// populated session plus the sanitized return URL. txID comes from the tx cookie.
func (o *OIDC) Callback(ctx context.Context, txID, state, code string) (*session.Session, string, error) {
	if txID == "" {
		return nil, "", ErrStateMismatch{Reason: ReasonNoTxCookie}
	}

	tx, ok := o.openTxn(txID)

	// Each branch is separate so the log names the actual cause. The checks
	// themselves are unchanged, and all four still fail the login.
	switch {
	case !ok:
		slog.Debug("oidc callback: the tx cookie did not open",
			"state_present", state != "")
		return nil, "", ErrStateMismatch{Reason: ReasonNoTransaction}
	case tx.Expiry.Before(time.Now()):
		slog.Debug("oidc callback: login transaction expired",
			"expired_at", tx.Expiry, "age", time.Since(tx.Expiry))
		return nil, "", ErrStateMismatch{Reason: ReasonExpired}
	case tx.State != state:
		slog.Debug("oidc callback: state parameter differs from the stored transaction",
			"state_present", state != "", "state_len", len(state), "stored_len", len(tx.State))
		return nil, "", ErrStateMismatch{Reason: ReasonStateDiffers}
	}
	slog.Debug("oidc callback: login transaction opened and matched")

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
//
// The carry-forward completes the token set BEFORE the session is built, and that
// order is load-bearing rather than stylistic: sessionFromToken derives the display
// User from the id_token's claims, so restoring the id_token onto the finished record
// instead would leave User built from the access token alone. RFC 6749 §6 does not
// require an id_token on refresh and most IDPs omit one, making that the ordinary
// path — the user's name would fall back to the raw "sub" UUID and their email would
// blank out, roughly an hour into every session, with nothing else changing to
// explain it. A fresh id_token still wins wherever the IDP sends one.
func (o *OIDC) SessionFromToken(tok *tokenResponse, prev *session.Session) *session.Session {
	// Copied rather than mutated in place: tok belongs to the caller, and a flat
	// struct of scalars makes the copy exact.
	effective := *tok
	if prev != nil {
		if effective.RefreshToken == "" {
			effective.RefreshToken = prev.RefreshToken
		}
		if effective.IDToken == "" {
			effective.IDToken = prev.IDToken
		}
	}
	return o.sessionFromToken(&effective)
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

// revocationTimeout keeps logout from hanging on an unreachable IDP.
const revocationTimeout = 5 * time.Second

// RevokeRefreshToken invalidates a refresh token at the IDP (RFC 7009).
//
// Necessary because the session record lives in the client: when the refresh token was
// held only in server memory, dropping it there WAS the revocation. A copy taken from
// the browser before logout would otherwise stay usable until it expired on its own
// (see authentication_authorization.md GO-AUTH-009).
//
// Errors are for logging only — the cookies are cleared regardless, and an IDP that is
// down must not leave the user unable to sign out.
func (o *OIDC) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	if o.disco.RevocationEndpoint == "" {
		slog.Debug("no revocation_endpoint advertised by the issuer — skipping refresh-token revocation")
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, revocationTimeout)
	defer cancel()

	form := url.Values{
		"token":           {refreshToken},
		"token_type_hint": {"refresh_token"},
		"client_id":       {o.clientID},
		"client_secret":   {o.clientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.disco.RevocationEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("revocation request failed: %w", err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4<<10))

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("revocation endpoint returned status %d", res.StatusCode)
	}
	return nil
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

// sealTxn encodes and seals a login transaction for the tx cookie.
func (o *OIDC) sealTxn(t *txn) (string, error) {
	raw, err := json.Marshal(sealedTxn{
		S: t.State, N: t.Nonce, V: t.CodeVerifier, R: t.ReturnURL, E: t.Expiry.Unix(),
	})
	if err != nil {
		return "", err
	}
	return o.sealer.Seal(raw)
}

// openTxn reverses sealTxn.
//
// There is deliberately no one-shot consumption: with nothing stored, there is nothing
// to consume. What still guards a replay is that the authorization code the callback
// must carry is single-use at the IDP, and that expiry, state and the id_token nonce
// are re-checked on every presentation.
func (o *OIDC) openTxn(txID string) (*txn, bool) {
	raw, err := o.sealer.Open(txID)
	if err != nil {
		return nil, false
	}
	var st sealedTxn
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, false
	}
	return &txn{
		State:        st.S,
		Nonce:        st.N,
		CodeVerifier: st.V,
		ReturnURL:    st.R,
		Expiry:       time.Unix(st.E, 0),
	}, true
}

// sealedTxn is the wire form of txn; short field names because it rides every request
// to the auth routes.
type sealedTxn struct {
	S string `json:"s"`
	N string `json:"n"`
	V string `json:"v"`
	R string `json:"r,omitempty"`
	E int64  `json:"e"`
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

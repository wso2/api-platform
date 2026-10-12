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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-workspace-bff/internal/secure"
	"ai-workspace-bff/internal/session"
	"net/url"
)

// TestCallbackReasonsAreDistinct pins that the four ways a callback fails to match a
// login are reported separately. They have different causes — a cookie the browser
// never sent, a cookie this deployment's key cannot open, a user who waited too long,
// and a genuinely wrong state — and collapsing them into one string sends an operator
// looking for an attack when the usual answer is a misconfigured key or a slow login.
func TestCallbackReasonsAreDistinct(t *testing.T) {
	t.Run("no tx cookie", func(t *testing.T) {
		_, _, err := testOIDC(t).Callback(context.Background(), "", "some-state", "code")
		assertReason(t, err, ReasonNoTxCookie)
	})

	t.Run("cookie will not open", func(t *testing.T) {
		_, _, err := testOIDC(t).Callback(context.Background(), "not-a-sealed-record", "some-state", "code")
		assertReason(t, err, ReasonNoTransaction)
	})

	t.Run("sealed under another deployment's key", func(t *testing.T) {
		other := testOIDC(t)
		mine, err := secure.NewSealer(secure.DeriveKey("different-material", "test/tx"))
		if err != nil {
			t.Fatal(err)
		}
		o := &OIDC{sealer: mine}
		tx := sealedTx(t, other, &txn{State: "s", Expiry: time.Now().Add(time.Minute)})
		_, _, err = o.Callback(context.Background(), tx, "s", "code")
		assertReason(t, err, ReasonNoTransaction)
	})

	t.Run("expired transaction", func(t *testing.T) {
		o := testOIDC(t)
		tx := sealedTx(t, o, &txn{State: "s", Expiry: time.Now().Add(-time.Minute)})
		_, _, err := o.Callback(context.Background(), tx, "s", "code")
		assertReason(t, err, ReasonExpired)
	})

	t.Run("state differs", func(t *testing.T) {
		o := testOIDC(t)
		tx := sealedTx(t, o, &txn{State: "expected", Expiry: time.Now().Add(time.Minute)})
		_, _, err := o.Callback(context.Background(), tx, "attacker-supplied", "code")
		assertReason(t, err, ReasonStateDiffers)
	})
}

// testOIDC builds an authenticator with a real transaction sealer, which is now the
// only way a login transaction exists.
func testOIDC(t *testing.T) *OIDC {
	t.Helper()
	sealer, err := secure.NewSealer(secure.DeriveKey("test-material", "test/tx"))
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}
	return &OIDC{sealer: sealer}
}

// sealedTx is the tx cookie value for a transaction, as AuthCodeURL would have written it.
func sealedTx(t *testing.T, o *OIDC, tx *txn) string {
	t.Helper()
	v, err := o.sealTxn(tx)
	if err != nil {
		t.Fatalf("sealTxn: %v", err)
	}
	return v
}

func assertReason(t *testing.T, err error, want string) {
	t.Helper()
	var mismatch ErrStateMismatch
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %v, want ErrStateMismatch", err)
	}
	if mismatch.Reason != want {
		t.Errorf("Reason = %q, want %q", mismatch.Reason, want)
	}
	// The reason is a log detail; the error string still leads with the generic text
	// so nothing downstream starts matching on the specific cause.
	if !strings.HasPrefix(mismatch.Error(), "oidc state mismatch") {
		t.Errorf("Error() = %q, want it to stay prefixed with the generic message", mismatch.Error())
	}
}

// TestRefreshKeepsIDTokenOnlyProfileClaims pins the ordering inside SessionFromToken.
// The display User is derived from the id_token's claims, and RFC 6749 §6 does not
// require an id_token on a refresh — most IDPs omit one. If the previous id_token is
// restored onto the finished record rather than folded in before it is built, User
// gets rebuilt from the access token alone: the name falls back to the raw "sub" UUID
// and the email blanks out, about an hour into every session.
func TestRefreshKeepsIDTokenOnlyProfileClaims(t *testing.T) {
	o := &OIDC{mapping: session.DefaultClaimMapping(), absTTL: 8 * time.Hour}

	// The profile lives only in the id_token; the access token carries an opaque sub.
	// That split is the common shape, and the whole reason the id_token is consulted.
	prev := &session.Session{
		RefreshToken: "refresh-from-login",
		IDToken: jwtWithClaims(t, map[string]any{
			"username": "Alice Example",
			"email":    "alice@example.com",
		}),
	}
	accessOnly := jwtWithClaims(t, map[string]any{"sub": "8f14e45f-ea3b-4d8a-9f2c-1b7d6e0a5c31"})

	t.Run("id_token omitted on refresh", func(t *testing.T) {
		s := o.SessionFromToken(&tokenResponse{AccessToken: accessOnly, ExpiresIn: 3600}, prev)

		if s.User.Name != "Alice Example" {
			t.Errorf("User.Name = %q, want %q — the login id_token's profile claims did "+
				"not reach UserFromClaims, so the name fell back to the sub claim", s.User.Name, "Alice Example")
		}
		if s.User.Email != "alice@example.com" {
			t.Errorf("User.Email = %q, want %q", s.User.Email, "alice@example.com")
		}
		// The pre-existing carry-forward must still hold: the id_token is also the
		// id_token_hint for RP-initiated logout, and the refresh token is the session.
		if s.IDToken != prev.IDToken {
			t.Error("previous id_token was not carried forward")
		}
		if s.RefreshToken != prev.RefreshToken {
			t.Errorf("RefreshToken = %q, want the previous one carried forward", s.RefreshToken)
		}
	})

	t.Run("fresh id_token wins over the previous one", func(t *testing.T) {
		fresh := jwtWithClaims(t, map[string]any{"username": "Alice Renamed", "email": "renamed@example.com"})
		s := o.SessionFromToken(&tokenResponse{AccessToken: accessOnly, IDToken: fresh, ExpiresIn: 3600}, prev)

		if s.User.Name != "Alice Renamed" {
			t.Errorf("User.Name = %q, want %q — an id_token the IDP DID send on refresh "+
				"must not be shadowed by the stale one", s.User.Name, "Alice Renamed")
		}
		if s.IDToken != fresh {
			t.Error("a fresh id_token was replaced by the previous one")
		}
	})

	t.Run("no previous session", func(t *testing.T) {
		// prev is nil on any path that has no earlier record; it must not panic.
		s := o.SessionFromToken(&tokenResponse{AccessToken: accessOnly, ExpiresIn: 3600}, nil)
		if s.User.Name != "8f14e45f-ea3b-4d8a-9f2c-1b7d6e0a5c31" {
			t.Errorf("User.Name = %q, want the sub claim — with no id_token anywhere, "+
				"falling back to sub is correct", s.User.Name)
		}
	})
}

// A login that takes longer than the transaction lives must say so. Reported as
// "cannot open" instead, it would be indistinguishable from a key mismatch — the
// difference between a one-line diagnosis and an afternoon of guessing.
func TestCallbackReportsExpiredRatherThanMissing(t *testing.T) {
	o := testOIDC(t)
	tx := sealedTx(t, o, &txn{State: "st", Expiry: time.Now().Add(-time.Minute)})

	_, _, err := o.Callback(context.Background(), tx, "st", "code")
	var mismatch ErrStateMismatch
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %v, want ErrStateMismatch", err)
	}
	if mismatch.Reason != ReasonExpired {
		t.Errorf("reason = %q, want %q", mismatch.Reason, ReasonExpired)
	}
}

// The tx cookie and the transaction must expire together: a cookie that outlives
// the transaction reports a slow login as "no transaction for this id", and one that
// dies first reports the same event as "no cookie at all".
func TestTxTTLIsGenerousEnoughForAnInteractiveLogin(t *testing.T) {
	// MFA, an account picker and a mistyped password fit inside this; ten minutes
	// does not, which is what this guards against being quietly reduced to.
	if TxTTL < 20*time.Minute {
		t.Errorf("TxTTL = %s, too short for an interactive IDP login", TxTTL)
	}
	if expiredRetention <= TxTTL {
		t.Errorf("expiredRetention (%s) must outlast TxTTL (%s), or the tx cookie dies "+
			"before an expired login can be reported as expired", expiredRetention, TxTTL)
	}
}

// What still guards a replayed callback, now that a transaction lives in the client's
// cookie and so cannot be consumed on first use.
//
// This is a deliberate trade and it is named here so nobody has to infer it: a sealed
// transaction can be presented twice. What stops a replay mattering is that a
// transaction is worthless without the authorization `code` the IDP hands back, that
// code is single-use at the IDP and a second presentation is refused there, and the tx
// cookie is cleared on every callback. The checks below — expiry, state, and (in
// Callback) the id_token's nonce — all still apply on every presentation. Restoring
// one-shot semantics would require shared server state, which is the thing the cookie
// store exists to avoid.
func TestReplayedCallbackStillFailsEveryOtherCheck(t *testing.T) {
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer idp.Close()

	o := testOIDC(t)
	o.client = idp.Client()
	o.disco = discoveryDoc{TokenEndpoint: idp.URL}

	// Expiry is re-checked on every presentation, not recorded as "already used".
	expired := sealedTx(t, o, &txn{State: "st", Expiry: time.Now().Add(-time.Minute)})
	for i := 0; i < 2; i++ {
		_, _, err := o.Callback(context.Background(), expired, "st", "code")
		var mismatch ErrStateMismatch
		if !errors.As(err, &mismatch) || mismatch.Reason != ReasonExpired {
			t.Fatalf("presentation %d: err = %v, want %s", i+1, err, ReasonExpired)
		}
	}

	// A live transaction replayed with the wrong state is refused on state, every time.
	live := sealedTx(t, o, &txn{State: "st", Expiry: time.Now().Add(time.Hour)})
	for i := 0; i < 2; i++ {
		_, _, err := o.Callback(context.Background(), live, "not-st", "code")
		var mismatch ErrStateMismatch
		if !errors.As(err, &mismatch) || mismatch.Reason != ReasonStateDiffers {
			t.Fatalf("presentation %d: err = %v, want %s", i+1, err, ReasonStateDiffers)
		}
	}
}

// AuthCodeURL writes the protocol parameters after the caller's extras. This pins that
// ordering: it is the guard that stops a forwarded parameter from widening the request
// even if the server's allowlist were ever loosened.
func TestAuthCodeURLExtrasCannotOverrideProtocolParams(t *testing.T) {
	o := &OIDC{
		clientID:    "ai-workspace",
		redirectURL: "https://portal.example.com/ai-workspace/api/auth/callback",
		scopes:      "openid profile email",
		sealer:      testOIDC(t).sealer,
		disco:       discoveryDoc{AuthorizationEndpoint: "https://idp.example.com/authorize"},
	}

	authURL, _, err := o.AuthCodeURL("/", url.Values{
		"fidp":          {"google"},
		"scope":         {"openid admin"},
		"redirect_uri":  {"https://evil.example.com/steal"},
		"response_type": {"token"},
		"client_id":     {"another-client"},
	})
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := parsed.Query()

	if q.Get("fidp") != "google" {
		t.Errorf("fidp = %q, want it forwarded", q.Get("fidp"))
	}
	for name, want := range map[string]string{
		"response_type": "code",
		"client_id":     o.clientID,
		"redirect_uri":  o.redirectURL,
		"scope":         o.scopes,
	} {
		if got := q.Get(name); got != want {
			t.Errorf("%s = %q, want %q — an extra parameter overrode a protocol one", name, got, want)
		}
	}
}

// A 307/308 from the IDP re-sends the POST body to the redirect target, and that body
// carries the client secret and — on revocation — the refresh token. Verified by
// observing what an attacker-controlled redirect target actually receives.
func TestBackChannelPostsAreNotFollowedAcrossRedirects(t *testing.T) {
	var attackerGot string
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		attackerGot = string(b)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer attacker.Close()

	for _, code := range []int{http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			attackerGot = ""
			idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, attacker.URL, code)
			}))
			defer idp.Close()

			o := testOIDC(t)
			o.client = noRedirectClient(idp.Client())
			o.clientID, o.clientSecret = "client-id", "the-client-secret"
			o.disco = discoveryDoc{TokenEndpoint: idp.URL, RevocationEndpoint: idp.URL}

			_ = o.RevokeRefreshToken(context.Background(), "the-refresh-token")
			if attackerGot != "" {
				t.Fatalf("the redirect target received the request body: %q", attackerGot)
			}

			_, _ = o.Refresh(context.Background(), "the-refresh-token")
			if attackerGot != "" {
				t.Fatalf("the redirect target received the refresh body: %q", attackerGot)
			}
		})
	}
}

// Discovery must refuse to post the client secret to a plaintext endpoint. Loopback is
// exempt — the request never reaches a network there, which is what keeps httptest and
// local IDP setups working.
func TestDiscoveryRejectsPlaintextCredentialEndpoints(t *testing.T) {
	for name, tc := range map[string]struct {
		endpoint string
		wantErr  bool
	}{
		"https":               {"https://idp.example.com/token", false},
		"loopback http":       {"http://127.0.0.1:9443/token", false},
		"localhost http":      {"http://localhost:9443/token", false},
		"remote http":         {"http://idp.example.com/token", true},
		"userinfo in the URL": {"https://user:pw@idp.example.com/token", true},
		"not a URL":           {"://nonsense", true},
	} {
		t.Run(name, func(t *testing.T) {
			err := requireSecureEndpoint("token_endpoint", tc.endpoint)
			if (err != nil) != tc.wantErr {
				t.Fatalf("requireSecureEndpoint(%q) = %v, wantErr %v", tc.endpoint, err, tc.wantErr)
			}
		})
	}
}

// A plaintext revocation_endpoint is optional, so it is dropped rather than failing
// startup — logout still works, and SupportsRevocation reports the loss so the existing
// startup warning tells the operator.
func TestDiscoveryDropsAPlaintextRevocationEndpoint(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"` + srv.URL + `","authorization_endpoint":"` + srv.URL +
			`/authorize","token_endpoint":"` + srv.URL +
			`/token","revocation_endpoint":"http://idp.example.com/revoke"}`))
	}))
	defer srv.Close()

	o, err := NewOIDC(context.Background(), srv.Client(), srv.URL, "c", "s",
		"https://portal.example.com/cb", "", "openid",
		session.DefaultClaimMapping(), time.Hour, testOIDC(t).sealer)
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	if o.SupportsRevocation() {
		t.Fatal("a plaintext revocation_endpoint was kept — the refresh token would be sent in cleartext")
	}
	if err := o.RevokeRefreshToken(context.Background(), "rt"); err != nil {
		t.Errorf("revocation should no-op, not error: %v", err)
	}
}

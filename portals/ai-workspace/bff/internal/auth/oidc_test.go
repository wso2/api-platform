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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-workspace-bff/internal/session"
)

// TestCallbackReasonsAreDistinct pins that the four ways a callback fails to match a
// login are reported separately. They have different causes — a restarted process, a
// cookie the browser never sent, a user who waited too long, and a genuinely wrong
// state — and collapsing them into one string sends an operator looking for an attack
// when the usual answer is a restart.
func TestCallbackReasonsAreDistinct(t *testing.T) {
	newOIDC := func() *OIDC {
		return &OIDC{txs: make(map[string]*txn), done: make(chan struct{})}
	}

	t.Run("no tx cookie", func(t *testing.T) {
		_, _, err := newOIDC().Callback(context.Background(), "", "some-state", "code")
		assertReason(t, err, ReasonNoTxCookie)
	})

	t.Run("no transaction (server restarted)", func(t *testing.T) {
		_, _, err := newOIDC().Callback(context.Background(), "tx-gone", "some-state", "code")
		assertReason(t, err, ReasonNoTransaction)
	})

	t.Run("expired transaction", func(t *testing.T) {
		o := newOIDC()
		o.txs["tx-1"] = &txn{State: "s", Expiry: time.Now().Add(-time.Minute)}
		_, _, err := o.Callback(context.Background(), "tx-1", "s", "code")
		assertReason(t, err, ReasonExpired)
	})

	t.Run("state differs", func(t *testing.T) {
		o := newOIDC()
		o.txs["tx-1"] = &txn{State: "expected", Expiry: time.Now().Add(time.Minute)}
		_, _, err := o.Callback(context.Background(), "tx-1", "attacker-supplied", "code")
		assertReason(t, err, ReasonStateDiffers)
	})
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

// A login that takes longer than the transaction lives must say so. Swept on
// expiry, it would report "no such transaction" instead — indistinguishable from a
// restart or a replay, which is the difference between a one-line diagnosis and an
// afternoon of guessing.
func TestCallbackReportsExpiredRatherThanMissing(t *testing.T) {
	o := &OIDC{txs: map[string]*txn{}, done: make(chan struct{})}
	defer o.Close()

	o.txs["tx-1"] = &txn{State: "st", Expiry: time.Now().Add(-time.Minute)}

	_, _, err := o.Callback(context.Background(), "tx-1", "st", "code")
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
		t.Errorf("expiredRetention (%s) must outlast TxTTL (%s), or expired transactions "+
			"are swept before they can be reported as expired", expiredRetention, TxTTL)
	}
}

// A consumed transaction is gone: replaying the callback URL must not log anyone in
// a second time.
func TestCallbackConsumesTheTransaction(t *testing.T) {
	// The code exchange is expected to fail — this is about the transaction, not the
	// IDP — but it must reach a real endpoint rather than a nil client.
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer idp.Close()

	o := &OIDC{
		client: idp.Client(),
		disco:  discoveryDoc{TokenEndpoint: idp.URL},
		txs:    map[string]*txn{},
		done:   make(chan struct{}),
	}
	defer o.Close()
	o.txs["tx-1"] = &txn{State: "st", Expiry: time.Now().Add(time.Hour)}

	// First use fails at the code exchange, but must still consume the transaction.
	_, _, _ = o.Callback(context.Background(), "tx-1", "st", "code")
	if n := o.PendingTransactions(); n != 0 {
		t.Fatalf("pending transactions = %d after use, want 0", n)
	}

	_, _, err := o.Callback(context.Background(), "tx-1", "st", "code")
	var mismatch ErrStateMismatch
	if !errors.As(err, &mismatch) || mismatch.Reason != ReasonNoTransaction {
		t.Errorf("replay err = %v, want %s", err, ReasonNoTransaction)
	}
}

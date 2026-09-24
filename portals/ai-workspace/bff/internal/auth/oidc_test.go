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
	"strings"
	"testing"
	"time"
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

/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

type fakeLedger struct {
	mu    sync.Mutex
	rows  []*model.ServiceAccountRevocation
	err   error
	calls int
}

func (l *fakeLedger) Revoke(*model.ServiceAccountRevocation) error { return nil }
func (l *fakeLedger) ListActive(time.Time) ([]*model.ServiceAccountRevocation, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	return l.rows, l.err
}
func (l *fakeLedger) PruneExpired(time.Time) (int64, error) { return 0, nil }
func (l *fakeLedger) callCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func rev(account string, minVersion int64, expiresAt time.Time) *model.ServiceAccountRevocation {
	return &model.ServiceAccountRevocation{AccountUUID: account, MinTokenVersion: minVersion, ExpiresAt: expiresAt}
}

func TestRevocationCache_TokenVersionWatermark(t *testing.T) {
	c := NewRevocationCache(&fakeLedger{rows: []*model.ServiceAccountRevocation{rev("a", 3, time.Now().Add(time.Hour))}}, quietLogger())
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		version int64
		want    bool
	}{
		{"minted before the revoke", 2, true},
		// No clock is involved: a token minted with the new credentials the
		// same instant the revoke committed is still accepted.
		{"minted after the revoke", 3, false},
		{"minted after a later, not yet seen change", 4, false},
	}
	for _, tc := range cases {
		if got := c.IsRevoked("a", tc.version); got != tc.want {
			t.Errorf("%s: IsRevoked = %v, want %v", tc.name, got, tc.want)
		}
	}
	if c.IsRevoked("other", 1) {
		t.Error("an account with no watermark is not revoked")
	}
}

func TestRevocationCache_FailsClosed(t *testing.T) {
	ledger := &fakeLedger{err: errors.New("db down")}
	c := NewRevocationCache(ledger, quietLogger())
	if err := c.Load(); err == nil {
		t.Fatal("Load must report the failure so startup can refuse")
	}
	if !c.IsRevoked("any", 1) {
		t.Fatal("a cold cache must revoke everything, never nothing")
	}

	ledger.err = nil
	ledger.rows = []*model.ServiceAccountRevocation{rev("a", 2, time.Now().Add(time.Hour))}
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	ledger.err = errors.New("db down again")
	if err := c.refresh(); err == nil {
		t.Fatal("refresh should fail")
	}
	if !c.IsRevoked("a", 1) {
		t.Fatal("a failed refresh must keep the previous contents")
	}
}

func TestRevocationCache_RememberAppliesAtOnce(t *testing.T) {
	c := NewRevocationCache(&fakeLedger{}, quietLogger())
	c.Load() //nolint:errcheck
	c.Remember(rev("a", 2, time.Now().Add(time.Hour)))
	if !c.IsRevoked("a", 1) {
		t.Fatal("a local revoke must apply before the next poll")
	}
}

// A refresh whose snapshot was read before a local revoke committed must not
// discard that revoke; only expiry drops a watermark.
func TestRevocationCache_RefreshKeepsNewerLocalRevoke(t *testing.T) {
	ledger := &fakeLedger{}
	c := NewRevocationCache(ledger, quietLogger())
	c.Load() //nolint:errcheck

	c.Remember(rev("a", 2, time.Now().Add(time.Hour)))
	c.Remember(rev("gone", 2, time.Now().Add(-time.Second)))
	ledger.rows = []*model.ServiceAccountRevocation{rev("b", 5, time.Now().Add(time.Hour))}
	if err := c.refresh(); err != nil {
		t.Fatal(err)
	}
	if !c.IsRevoked("a", 1) {
		t.Fatal("a local revoke missing from a stale snapshot was dropped")
	}
	if !c.IsRevoked("b", 4) {
		t.Fatal("a watermark from the database was not applied")
	}
	if c.IsRevoked("gone", 1) {
		t.Fatal("an expired watermark must be dropped")
	}

	// A lower watermark from the database never lowers a higher local one.
	ledger.rows = []*model.ServiceAccountRevocation{rev("a", 1, time.Now().Add(time.Hour))}
	c.refresh() //nolint:errcheck
	if !c.IsRevoked("a", 1) {
		t.Fatal("the watermark moved backwards")
	}
}

// Jitter is waited before the first fetch too, so no fetch happens at t=0.
func TestRevocationCache_JitterBeforeFirstFetch(t *testing.T) {
	ledger := &fakeLedger{}
	c := NewRevocationCache(ledger, quietLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx, 200*time.Millisecond)

	time.Sleep(150 * time.Millisecond)
	if n := ledger.callCount(); n != 0 {
		t.Fatalf("fetched %d times before the first interval elapsed", n)
	}
	deadline := time.Now().Add(2 * time.Second)
	for ledger.callCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if ledger.callCount() == 0 {
		t.Fatal("the poll never ran")
	}
}

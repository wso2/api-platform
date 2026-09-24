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

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ai-workspace-bff/internal/config"
	"ai-workspace-bff/internal/paths"
)

func (h *exchangeTestHarness) switchOrgRequest(subject, org string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, h.server.path("/api/session/org"), strings.NewReader(`{"org":"`+org+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: h.server.cfg.Cookie.Name, Value: subject})
	rec := httptest.NewRecorder()
	h.server.handleSwitchOrg(rec, req)
	return rec
}

// TestSwitchOrgSendsOrgParam: switching org sends the configured extra form field
// with the literal org handle, and persists the selection on the session.
func TestSwitchOrgSendsOrgParam(t *testing.T) {
	h := newExchangeHarness(t, func(c *config.TokenExchangeConfig) { c.OrgParam = "orgHandle" })
	subject := h.subjectSession(t)

	rec := h.switchOrgRequest(subject, "org-a")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	form := h.idpLastForm.Load()
	if form == nil {
		t.Fatal("IDP was never called")
	}
	if got := form.(interface{ Get(string) string }).Get("orgHandle"); got != "org-a" {
		t.Errorf("orgHandle form field = %q, want %q", got, "org-a")
	}

	sess, ok, _ := h.server.store.Get(context.Background(), subject)
	if !ok || sess.OrgHandle != "org-a" {
		t.Errorf("session OrgHandle = %q, want %q", sess.OrgHandle, "org-a")
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["scopes"]; !ok {
		t.Error("response carried no scopes")
	}
}

// TestSwitchOrgForcesFreshExchange: a cached token minted for one org must never be
// reused for another — switching orgs always re-hits the IDP.
func TestSwitchOrgForcesFreshExchange(t *testing.T) {
	h := newExchangeHarness(t, func(c *config.TokenExchangeConfig) { c.OrgParam = "orgHandle" })
	subject := h.subjectSession(t)

	if rec := h.switchOrgRequest(subject, "org-a"); rec.Code != http.StatusOK {
		t.Fatalf("switch to org-a: status %d", rec.Code)
	}
	callsAfterFirst := h.idpCalls.Load()
	if callsAfterFirst == 0 {
		t.Fatal("expected the IDP to be called for the first switch")
	}

	// A proxied request right after, still within MinValidity, must reuse the cache
	// rather than re-exchanging.
	if rec := h.proxyRequest(subject); rec.Code != http.StatusOK {
		t.Fatalf("proxy after switch: status %d", rec.Code)
	}
	if got := h.idpCalls.Load(); got != callsAfterFirst {
		t.Errorf("IDP calls = %d after a same-org proxy request, want %d (cache should have been reused)", got, callsAfterFirst)
	}

	// Switching to a different org must not reuse org-a's cached token.
	if rec := h.switchOrgRequest(subject, "org-b"); rec.Code != http.StatusOK {
		t.Fatalf("switch to org-b: status %d", rec.Code)
	}
	if got := h.idpCalls.Load(); got <= callsAfterFirst {
		t.Errorf("IDP calls = %d after switching org, want more than %d (a new org must force a fresh exchange)", got, callsAfterFirst)
	}
	if form := h.idpLastForm.Load(); form.(interface{ Get(string) string }).Get("orgHandle") != "org-b" {
		t.Errorf("orgHandle form field = %q, want %q", form.(interface{ Get(string) string }).Get("orgHandle"), "org-b")
	}
}

// TestSwitchOrgRejectionKeepsSessionAndRollsBack: unlike a login-token exchange
// rejection, a rejected org switch (e.g. not a member of that org) must not destroy
// the session, and must roll back to the org the session was still authorized for.
func TestSwitchOrgRejectionKeepsSessionAndRollsBack(t *testing.T) {
	h := newExchangeHarness(t, func(c *config.TokenExchangeConfig) { c.OrgParam = "orgHandle" })
	subject := h.subjectSession(t)

	if rec := h.switchOrgRequest(subject, "org-a"); rec.Code != http.StatusOK {
		t.Fatalf("switch to org-a: status %d", rec.Code)
	}

	h.idpStatus = func() (int, any) {
		return http.StatusBadRequest, map[string]string{"error": "invalid_request"}
	}
	rec := h.switchOrgRequest(subject, "org-forbidden")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a rejected org switch", rec.Code)
	}

	sess, ok, _ := h.server.store.Get(context.Background(), subject)
	if !ok {
		t.Fatal("a rejected org switch must not destroy the session")
	}
	if sess.OrgHandle != "org-a" {
		t.Errorf("session OrgHandle = %q after a rejected switch, want it rolled back to %q", sess.OrgHandle, "org-a")
	}
}

// TestSwitchOrgDisabledWithoutExchanger: the endpoint is meaningless without token
// exchange configured, and must say so rather than attempting anything.
func TestSwitchOrgDisabledWithoutExchanger(t *testing.T) {
	s := &Server{cfg: &config.Config{}}
	req := httptest.NewRequest(http.MethodPost, paths.Base+"/api/session/org", strings.NewReader(`{"org":"org-a"}`))
	rec := httptest.NewRecorder()
	s.handleSwitchOrg(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 when token exchange isn't configured", rec.Code)
	}
}

// TestSwitchOrgMissingSession: no session cookie at all is a 401, same shape as
// every other unauthenticated call.
func TestSwitchOrgMissingSession(t *testing.T) {
	h := newExchangeHarness(t, func(c *config.TokenExchangeConfig) { c.OrgParam = "orgHandle" })
	req := httptest.NewRequest(http.MethodPost, h.server.path("/api/session/org"), strings.NewReader(`{"org":"org-a"}`))
	rec := httptest.NewRecorder()
	h.server.handleSwitchOrg(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 with no session cookie", rec.Code)
	}
}

// TestWithSessionLockIsMutuallyExclusive pins the property the lock exists for: two
// callers for the same token never run inside it at once.
//
// The ordering is staged rather than a burst of goroutines, because the defect it
// guards against has a narrow window. Before the entry was reference-counted, a
// caller finishing deleted it from the map even while another was still blocked on
// that mutex — so an arrival AFTER the delete built a second mutex for the same token
// and ran alongside the waiter. A burst mostly misses it: every goroutine has already
// taken the pointer before the first delete. The three stages below reproduce it
// every run: hold, queue a waiter, release (deleting the entry under the bug), then
// arrive fresh while the waiter is still inside.
func TestWithSessionLockIsMutuallyExclusive(t *testing.T) {
	s := &Server{sessionLocks: make(map[string]*sessionLock)}

	var concurrent atomic.Int32
	var overlaps atomic.Int32
	enter := func(entered chan<- struct{}, release <-chan struct{}) {
		s.withSessionLock("same-token", func() {
			if concurrent.Add(1) > 1 {
				overlaps.Add(1)
			}
			close(entered)
			<-release
			concurrent.Add(-1)
		})
	}

	// Stage 1: one caller inside, holding.
	first, releaseFirst := make(chan struct{}), make(chan struct{})
	go enter(first, releaseFirst)
	<-first

	// Stage 2: a second caller queued on that same mutex.
	second, releaseSecond := make(chan struct{}), make(chan struct{})
	go enter(second, releaseSecond)
	time.Sleep(50 * time.Millisecond) // let it reach the map and block

	// Stage 3: the first leaves — under the bug this drops the entry the waiter is
	// still using — and the waiter takes the lock.
	close(releaseFirst)
	<-second

	// A caller arriving now must block behind the waiter. Under the bug it finds no
	// entry, makes its own mutex, and walks straight in.
	third, releaseThird := make(chan struct{}), make(chan struct{})
	go enter(third, releaseThird)
	select {
	case <-third:
		t.Error("a third caller entered while another held the same token's lock — the lock was not shared")
	case <-time.After(300 * time.Millisecond):
	}

	close(releaseSecond)
	<-third // the third caller may now proceed
	close(releaseThird)

	if got := overlaps.Load(); got != 0 {
		t.Errorf("%d overlapping entries — callers for the same token were not serialized", got)
	}
	// The entry must not outlive its last user, or the map grows by one per token
	// for the life of the process.
	deadline := time.Now().Add(time.Second)
	for {
		s.sessionMu.Lock()
		n := len(s.sessionLocks)
		s.sessionMu.Unlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Errorf("sessionLocks still holds %d entries, want 0 once every caller is done", n)
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Different tokens must not serialize against each other: one slow session's store
// write would otherwise stall every other session's.
func TestWithSessionLockIsPerToken(t *testing.T) {
	s := &Server{sessionLocks: make(map[string]*sessionLock)}

	held := make(chan struct{})
	released := make(chan struct{})
	go func() {
		s.withSessionLock("token-a", func() {
			close(held)
			<-released
		})
	}()
	<-held

	done := make(chan struct{})
	go func() {
		s.withSessionLock("token-b", func() {})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("a second token blocked on the first token's lock")
	}
	close(released)
}

// A panic inside fn must not leave the token's lock held: the deferred release is
// what keeps one failed request from wedging that session for the process's life.
func TestWithSessionLockReleasesOnPanic(t *testing.T) {
	s := &Server{sessionLocks: make(map[string]*sessionLock)}

	func() {
		defer func() { _ = recover() }()
		s.withSessionLock("tok", func() { panic("boom") })
	}()

	done := make(chan struct{})
	go func() {
		s.withSessionLock("tok", func() {})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("lock still held after a panic inside fn")
	}
}

// TestSwitchOrgAuthFailuresAreIndistinguishable: both ways the switch can fail to
// authenticate — no session cookie at all, and a cookie whose session is gone — must
// produce byte-identical 401s (error-handling.md directive 4). Branching the payload
// tells a caller whether a token they hold was valid until recently or was never
// known here, which is an oracle nothing legitimate needs.
func TestSwitchOrgAuthFailuresAreIndistinguishable(t *testing.T) {
	h := newExchangeHarness(t, func(c *config.TokenExchangeConfig) { c.OrgParam = "orgHandle" })

	// No cookie at all.
	noCookie := httptest.NewRequest(http.MethodPost, h.server.path("/api/session/org"),
		strings.NewReader(`{"org":"org-a"}`))
	noCookie.Header.Set("Content-Type", "application/json")
	recNoCookie := httptest.NewRecorder()
	h.server.handleSwitchOrg(recNoCookie, noCookie)

	// A cookie whose session was never stored.
	recUnknown := h.switchOrgRequest("unknown-token", "org-a")

	if recNoCookie.Code != http.StatusUnauthorized || recUnknown.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d and %d, want 401 for both", recNoCookie.Code, recUnknown.Code)
	}
	if got, want := recUnknown.Body.String(), recNoCookie.Body.String(); got != want {
		t.Errorf("payloads differ:\n missing cookie: %s\n unknown session: %s", want, got)
	}
	// And the body must not name which check failed.
	for _, leak := range []string{"not authenticated", "session expired", "NOT_AUTHENTICATED", "SESSION_EXPIRED"} {
		if strings.Contains(recNoCookie.Body.String(), leak) {
			t.Errorf("response names the specific failure (%q): %s", leak, recNoCookie.Body.String())
		}
	}
}

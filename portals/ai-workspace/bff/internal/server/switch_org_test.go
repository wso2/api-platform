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
	"testing"

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

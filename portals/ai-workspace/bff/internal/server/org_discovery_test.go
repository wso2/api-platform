/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"ai-workspace-bff/internal/config"
	"ai-workspace-bff/internal/proxy"
	"ai-workspace-bff/internal/session"
)

// orgAPIStub stands in for the Platform API's GET /organizations, recording what the
// caller sent and answering with the given body.
type orgAPIStub struct {
	gotAuth  atomic.Value
	gotPath  atomic.Value
	gotQuery atomic.Value
	calls    atomic.Int32
	status   int
	body     string
}

func newOrgServer(t *testing.T, stub *orgAPIStub) *httptest.Server {
	t.Helper()
	stub.gotAuth.Store("")
	stub.gotPath.Store("")
	stub.gotQuery.Store("")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.calls.Add(1)
		stub.gotAuth.Store(r.Header.Get("Authorization"))
		stub.gotPath.Store(r.URL.Path)
		stub.gotQuery.Store(r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(stub.status)
		_, _ = w.Write([]byte(stub.body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// discoveryServer builds a Server wired for org discovery against the stub, with a
// session already seeded for the login token.
func discoveryServer(t *testing.T, upstreamURL, defaultOrg string) *Server {
	t.Helper()
	s := &Server{
		cfg: &config.Config{
			ControlPlane: config.ControlPlaneConfig{
				URL:                 upstreamURL,
				PlatformAPIBasePath: "/api/v0.9",
				PortalAPIBasePath:   "/api/portal/v0.9",
			},
			Auth: config.AuthConfig{
				Mode: config.AuthModeOIDC,
				OIDC: config.OIDCConfig{TokenExchange: config.TokenExchangeConfig{
					Enabled:    true,
					OrgParam:   "orgHandle",
					DefaultOrg: defaultOrg,
				}},
			},
			Session: config.SessionConfig{IdleTimeout: 30 * time.Minute, AbsoluteTTL: 8 * time.Hour},
		},
		claims:        session.DefaultClaimMapping(),
		store:         session.NewMemoryStore(),
		proxy:         proxy.ReverseProxy(nil, "", http.DefaultTransport),
		refreshLocks:  make(map[string]*refreshLock),
		exchangeLocks: make(map[string]*exchangeLock),
		sessionLocks:  make(map[string]*sessionLock),
	}
	t.Cleanup(func() { _ = s.store.Close() })
	if err := s.store.Put(context.Background(), &session.Session{
		ID:             "login-token",
		Mode:           session.ModeOIDC,
		AccessToken:    "login-token",
		AbsoluteExpiry: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return s
}

// The behaviour this exists for: the org the exchange is scoped to comes from the
// user's own memberships, read with the LOGIN token — never a default guessed for
// everyone. Asking the STS to resolve a default is what made it answer 500 with
// "error occurred while retrieving the organization".
func TestResolveOrgHandleUsesTheUsersFirstOrg(t *testing.T) {
	stub := &orgAPIStub{status: http.StatusOK, body: `{"count":2,"list":[{"handle":"acme"},{"handle":"globex"}]}`}
	srv := newOrgServer(t, stub)
	s := discoveryServer(t, srv.URL, "default")

	if got := s.resolveOrgHandle(context.Background(), "login-token", ""); got != "acme" {
		t.Errorf("org handle = %q, want acme (the first of the user's orgs)", got)
	}
	// The lookup must carry the login token: it decides what the exchange is scoped
	// to, so exchanging first would be circular.
	if got := stub.gotAuth.Load().(string); got != "Bearer login-token" {
		t.Errorf("Authorization = %q, want the login token", got)
	}
	if got := stub.gotPath.Load().(string); got != "/api/v0.9/organizations" {
		t.Errorf("path = %q", got)
	}
	if got := stub.gotQuery.Load().(string); got != "limit=1&offset=0" {
		t.Errorf("query = %q", got)
	}

	// Persisted on the session, so it costs one call per session, not per exchange.
	sess, ok, _ := s.store.Get(context.Background(), "login-token")
	if !ok || sess.OrgHandle != "acme" {
		t.Fatalf("session org = %q (found=%v), want acme", sess.OrgHandle, ok)
	}
	if got := s.resolveOrgHandle(context.Background(), "login-token", sess.OrgHandle); got != "acme" {
		t.Errorf("second resolve = %q, want acme", got)
	}
	if n := stub.calls.Load(); n != 1 {
		t.Errorf("Platform API called %d times, want 1", n)
	}
}

func TestResolveOrgHandlePrecedenceAndFallbacks(t *testing.T) {
	t.Run("the user's own switch wins over discovery", func(t *testing.T) {
		stub := &orgAPIStub{status: http.StatusOK, body: `{"count":1,"list":[{"handle":"acme"}]}`}
		s := discoveryServer(t, newOrgServer(t, stub).URL, "default")
		if got := s.resolveOrgHandle(context.Background(), "login-token", "chosen-by-user"); got != "chosen-by-user" {
			t.Errorf("org handle = %q, want chosen-by-user", got)
		}
		if n := stub.calls.Load(); n != 0 {
			t.Errorf("Platform API called %d times for an already-chosen org", n)
		}
	})

	t.Run("a failed lookup degrades to default_org", func(t *testing.T) {
		stub := &orgAPIStub{status: http.StatusInternalServerError, body: `{"error":"boom"}`}
		s := discoveryServer(t, newOrgServer(t, stub).URL, "fallback-org")
		if got := s.resolveOrgHandle(context.Background(), "login-token", ""); got != "fallback-org" {
			t.Errorf("org handle = %q, want fallback-org", got)
		}
	})

	t.Run("a user in no org yet exchanges without one", func(t *testing.T) {
		stub := &orgAPIStub{status: http.StatusOK, body: `{"count":0,"list":[]}`}
		s := discoveryServer(t, newOrgServer(t, stub).URL, "")
		if got := s.resolveOrgHandle(context.Background(), "login-token", ""); got != "" {
			t.Errorf("org handle = %q, want empty", got)
		}
	})

	// Without org_param the resolved handle has no field to travel in, so the lookup
	// would find the right answer and then drop it.
	t.Run("no org_param means no lookup", func(t *testing.T) {
		stub := &orgAPIStub{status: http.StatusOK, body: `{"count":1,"list":[{"handle":"acme"}]}`}
		s := discoveryServer(t, newOrgServer(t, stub).URL, "configured-default")
		s.cfg.Auth.OIDC.TokenExchange.OrgParam = ""
		if got := s.resolveOrgHandle(context.Background(), "login-token", ""); got != "configured-default" {
			t.Errorf("org handle = %q, want configured-default", got)
		}
		if n := stub.calls.Load(); n != 0 {
			t.Errorf("Platform API called %d times with no org_param", n)
		}
	})
}

// The deployment this exists for: the gateway in front of the Platform API trusts
// only the STS, so it 401s the Asgardeo LOGIN token — "Issuer present in the JWT
// does not match with any of configured token issuers". The org list therefore has
// to come from a service that does trust the login IDP, in that service's own shape.
func TestOrgLookupURLOverridesThePlatformAPI(t *testing.T) {
	const userMgtPayload = `{
      "displayName": "John Doe",
      "userEmail": "john@apip.com",
      "idpId": "cfcb8d9d-3710-45ee-ae55-e73f3065d153",
      "organizations": [
        {"id":"24410","uuid":"5b4444fb-4bcc-4e63-85d3-ddd593841012","handle":"john","name":"John","status":"ACTIVE"},
        {"id":"7668","uuid":"f444f730-8c9e-44b8-abc7-1164c5f4a3e5","handle":"alice","name":"Alice","status":"ACTIVE"}
      ],
      "userId": "16050"
    }`

	var gotAuth, gotQuery, gotPath atomic.Value
	gotAuth.Store("")
	gotQuery.Store("")
	gotPath.Store("")
	userMgt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		gotPath.Store(r.URL.Path)
		gotQuery.Store(r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(userMgtPayload))
	}))
	defer userMgt.Close()

	// The Platform API must not be called at all once the override is set.
	platform := &orgAPIStub{status: http.StatusUnauthorized, body: `{"description":"Unauthenticated request"}`}
	s := discoveryServer(t, newOrgServer(t, platform).URL, "fallback")
	s.cfg.Auth.OIDC.TokenExchange.OrgLookupURL = userMgt.URL + "/user-mgt/1.0.0/validate/user?origin_cloud=cloud"

	if got := s.resolveOrgHandle(context.Background(), "login-token", ""); got != "john" {
		t.Errorf("org handle = %q, want john (the first organization)", got)
	}
	if got := gotAuth.Load().(string); got != "Bearer login-token" {
		t.Errorf("Authorization = %q, want the login token", got)
	}
	// The URL is sent as configured — the query string is part of the endpoint.
	if got := gotPath.Load().(string); got != "/user-mgt/1.0.0/validate/user" {
		t.Errorf("path = %q", got)
	}
	if got := gotQuery.Load().(string); got != "origin_cloud=cloud" {
		t.Errorf("query = %q, want origin_cloud=cloud", got)
	}
	if n := platform.calls.Load(); n != 0 {
		t.Errorf("Platform API called %d times despite the override", n)
	}
}

// Both shapes are read, so the URL needs no companion key describing its format.
func TestOrgLookupReadsEitherResponseShape(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"platform api", `{"count":1,"list":[{"handle":"acme"}]}`, "acme"},
		{"user service", `{"organizations":[{"handle":"john"}]}`, "john"},
		{"neither", `{"somethingElse":[]}`, ""},
		{"empty handle", `{"organizations":[{"handle":""}]}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			s := discoveryServer(t, srv.URL, "")
			s.cfg.Auth.OIDC.TokenExchange.OrgLookupURL = srv.URL
			if got := s.resolveOrgHandle(context.Background(), "login-token", ""); got != tc.want {
				t.Errorf("org handle = %q, want %q", got, tc.want)
			}
		})
	}
}

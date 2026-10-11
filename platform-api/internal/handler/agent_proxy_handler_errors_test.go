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

// Request-boundary failure paths of the Agent proxy, Agent proxy API-key and
// Agent proxy deployment handlers: a missing organization claim, an actor that
// cannot be resolved, oversized and unreadable bodies, and the path-parameter
// guards the router normally satisfies. Each is asserted against the published
// error catalog rather than the handler's internal wording.

package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
)

// failingBody is a request body whose read fails with something other than a
// MaxBytesError, so the generic "could not be read" branch is exercised.
type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errors.New("connection reset by peer") }

// callWithBody issues one request as agentProxyActor in agentProxyOrg with an
// arbitrary body reader.
func callWithBody(t *testing.T, h http.Handler, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Org", agentProxyOrg)
	req.Header.Set("X-Test-User", agentProxyActor)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// agentProxyRoutes is every Agent proxy, API-key and deployment operation, with
// a body each would otherwise accept far enough to reach its org check.
var agentProxyRoutes = []struct {
	name, method, path, body string
}{
	{"create proxy", http.MethodPost, agentProxyBase, minimalAgentProxyBody("weather-agent", "Weather")},
	{"list proxies", http.MethodGet, agentProxyBase, ""},
	{"get proxy", http.MethodGet, agentProxyBase + "/weather-agent", ""},
	{"update proxy", http.MethodPut, agentProxyBase + "/weather-agent", minimalAgentProxyBody("weather-agent", "Weather")},
	{"delete proxy", http.MethodDelete, agentProxyBase + "/weather-agent", ""},
	{"fetch card", http.MethodPost, fetchAgentCardPath, `{"url":"http://agent.example.com"}`},
	{"list keys", http.MethodGet, agentKeysPath("weather-agent"), ""},
	{"create key", http.MethodPost, agentKeysPath("weather-agent"), `{"displayName":"k"}`},
	{"update key", http.MethodPut, agentKeyPath("weather-agent", "k"), `{"apiKey":"v"}`},
	{"delete key", http.MethodDelete, agentKeyPath("weather-agent", "k"), ""},
	{"create deployment", http.MethodPost, agentProxyBase + "/weather-agent/deployments", `{}`},
	{"list deployments", http.MethodGet, agentProxyBase + "/weather-agent/deployments", ""},
	{"get deployment", http.MethodGet, agentProxyBase + "/weather-agent/deployments/d1", ""},
	{"delete deployment", http.MethodDelete, agentProxyBase + "/weather-agent/deployments/d1", ""},
	{"undeploy", http.MethodPost, agentProxyBase + "/weather-agent/deployments/d1/undeploy?gatewayId=gw", ""},
	{"restore", http.MethodPost, agentProxyBase + "/weather-agent/deployments/d1/restore?gatewayId=gw", ""},
}

// A request whose token carries no organization claim is a 401 on every
// operation, before any body is read or any service is consulted.
func TestAgentProxyHandlers_MissingOrganizationIsUnauthorized(t *testing.T) {
	env := newAgentProxyTestEnv(t, &config.Server{})
	for _, rt := range agentProxyRoutes {
		t.Run(rt.name, func(t *testing.T) {
			rec := callAgentProxyAs(t, env.handler, "", agentProxyActor, rt.method, rt.path, rt.body)
			assertAgentProxyError(t, rec, http.StatusUnauthorized, apperror.CodeCommonUnauthorized)
		})
	}
}

// When the caller's identity cannot be mapped to an internal user id, every
// mutating operation fails closed with a sterile 500 instead of writing an
// unattributed row.
func TestAgentProxyHandlers_UnresolvableActorIsInternalError(t *testing.T) {
	env := newAgentProxyTestEnv(t, &config.Server{})
	if _, err := env.db.Exec(`DROP TABLE user_idp_references`); err != nil {
		t.Fatalf("drop identity table: %v", err)
	}

	cases := []struct {
		name, method, path, body string
	}{
		{"create proxy", http.MethodPost, agentProxyBase, minimalAgentProxyBody("weather-agent", "Weather")},
		{"update proxy", http.MethodPut, agentProxyBase + "/weather-agent", minimalAgentProxyBody("weather-agent", "Weather")},
		{"delete proxy", http.MethodDelete, agentProxyBase + "/weather-agent", ""},
		{"list keys", http.MethodGet, agentKeysPath("weather-agent"), ""},
		{"create key", http.MethodPost, agentKeysPath("weather-agent"), `{"displayName":"k"}`},
		{"update key", http.MethodPut, agentKeyPath("weather-agent", "k"), `{"apiKey":"v"}`},
		{"delete key", http.MethodDelete, agentKeyPath("weather-agent", "k"), ""},
		{"create deployment", http.MethodPost, agentProxyBase + "/weather-agent/deployments", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := callAgentProxy(t, env.handler, tc.method, tc.path, tc.body)
			assertAgentProxyError(t, rec, http.StatusInternalServerError, apperror.CodeCommonInternalError)
			if strings.Contains(rec.Body.String(), "user_idp_references") {
				t.Fatalf("internal table name leaked into the response: %s", rec.Body.String())
			}
		})
	}
}

// A body over each operation's ceiling is a 413, whether the handler buffers
// the whole body (proxy create/update, card fetch) or streams it into a JSON
// decoder (deployment create).
func TestAgentProxyHandlers_OversizedBodyIsPayloadTooLarge(t *testing.T) {
	env := newAgentProxyTestEnv(t, &config.Server{})

	// An unterminated JSON string keeps the streaming decoder reading until the
	// MaxBytesReader trips, rather than stopping at a syntax error first.
	oversized := func(n int) string { return `{"description":"` + strings.Repeat("a", n) }

	cases := []struct {
		name, method, path, body string
	}{
		{"create proxy", http.MethodPost, agentProxyBase, oversized(agentProxyMaxBodyBytes + 1)},
		{"update proxy", http.MethodPut, agentProxyBase + "/weather-agent", oversized(agentProxyMaxBodyBytes + 1)},
		{"fetch card", http.MethodPost, fetchAgentCardPath, oversized(agentCardFetchMaxBodyBytes + 1)},
		{"create deployment", http.MethodPost, agentProxyBase + "/weather-agent/deployments", oversized(agentProxyDeployMaxBodyBytes + 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := callAgentProxy(t, env.handler, tc.method, tc.path, tc.body)
			assertAgentProxyError(t, rec, http.StatusRequestEntityTooLarge, apperror.CodeCommonPayloadTooLarge)
		})
	}
}

// A body whose read fails for a reason other than its size is a 400, and the
// transport error is not echoed back.
func TestAgentProxyHandlers_UnreadableBodyIsValidationFailure(t *testing.T) {
	env := newAgentProxyTestEnv(t, &config.Server{})
	cases := []struct {
		name, method, path string
	}{
		{"create proxy", http.MethodPost, agentProxyBase},
		{"update proxy", http.MethodPut, agentProxyBase + "/weather-agent"},
		{"fetch card", http.MethodPost, fetchAgentCardPath},
		{"create key", http.MethodPost, agentKeysPath("weather-agent")},
		{"update key", http.MethodPut, agentKeyPath("weather-agent", "k")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := callWithBody(t, env.handler, tc.method, tc.path, failingBody{})
			assertAgentProxyError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
			if strings.Contains(rec.Body.String(), "connection reset") {
				t.Fatalf("transport error leaked into the response: %s", rec.Body.String())
			}
		})
	}
}

// PUT validates its body before anything else: an empty one, and one whose id
// disagrees with the path, are both 400s that leave the stored proxy untouched.
func TestAgentProxyHandler_UpdateBodyRejections(t *testing.T) {
	h, _ := setupAgentProxyEnv(t)
	decodeAgentProxyJSON(t, callAgentProxy(t, h, http.MethodPost, agentProxyBase,
		minimalAgentProxyBody("weather-agent", "Weather")), http.StatusCreated)

	rec := callAgentProxy(t, h, http.MethodPut, agentProxyBase+"/weather-agent", "")
	assertAgentProxyError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)

	rec = callAgentProxy(t, h, http.MethodPut, agentProxyBase+"/weather-agent",
		minimalAgentProxyBody("renamed-agent", "Renamed"))
	assertAgentProxyError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)

	got := decodeAgentProxyJSON(t, callAgentProxy(t, h, http.MethodGet, agentProxyBase+"/weather-agent", ""), http.StatusOK)
	if got["displayName"] != "Weather" {
		t.Fatalf("displayName = %v, want the original value after rejected updates", got["displayName"])
	}
}

// A whitespace-only display name passes the emptiness check but cannot yield a
// key id, which is a 400 — never a key stored under a blank id.
func TestAgentProxyAPIKey_CreateWithBlankDisplayNameIsRejected(t *testing.T) {
	env := newAgentProxyTestEnv(t, &config.Server{})
	createAgentProxyForKeys(t, env.handler, "weather-agent")

	rec := callAgentProxy(t, env.handler, http.MethodPost, agentKeysPath("weather-agent"), `{"displayName":"   "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
	if n := countAgentKeys(t, env); n != 0 {
		t.Fatalf("stored keys = %d, want 0", n)
	}
}

// The API-key handlers guard their path parameters themselves rather than trust
// the router to have populated them. These guards run before any service is
// touched, so the handler is exercised directly with no path values set.
func TestAgentProxyAPIKeyHandler_MissingPathParametersAreValidationFailures(t *testing.T) {
	h := &AgentProxyAPIKeyHandler{slogger: slog.Default()}

	newReq := func(method, agentProxyID, apiKeyID string) *http.Request {
		r := httptest.NewRequest(method, "/", strings.NewReader(`{}`))
		r = middleware.WithOrganization(r, agentProxyOrg)
		if agentProxyID != "" {
			r.SetPathValue("agentProxyId", agentProxyID)
		}
		if apiKeyID != "" {
			r.SetPathValue("apiKeyId", apiKeyID)
		}
		return r
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"create without proxy id", func() error {
			return h.CreateAPIKey(httptest.NewRecorder(), newReq(http.MethodPost, "", ""))
		}},
		{"update without proxy id", func() error {
			return h.UpdateAPIKey(httptest.NewRecorder(), newReq(http.MethodPut, "", "k"))
		}},
		{"update without key id", func() error {
			return h.UpdateAPIKey(httptest.NewRecorder(), newReq(http.MethodPut, "weather-agent", ""))
		}},
		{"delete without proxy id", func() error {
			return h.DeleteAPIKey(httptest.NewRecorder(), newReq(http.MethodDelete, "", "k"))
		}},
		{"delete without key id", func() error {
			return h.DeleteAPIKey(httptest.NewRecorder(), newReq(http.MethodDelete, "weather-agent", ""))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !apperror.ValidationFailed.Is(err) {
				t.Fatalf("err = %v, want ValidationFailed", err)
			}
		})
	}
}

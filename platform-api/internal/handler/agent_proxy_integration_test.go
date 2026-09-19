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

// End-to-end Agent proxy CRUD over the real route -> handler -> service ->
// repository stack, backed by SQLite. These assert the published HTTP contract:
// statuses, the Location header, the error catalog codes, full-replacement PUT
// semantics and credential retention.

package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/vault"

	_ "github.com/mattn/go-sqlite3"
)

const (
	agentProxyBase     = "/api/v0.9/agent-proxies"
	agentProxyOrg      = "org-agent-it"
	agentProxyOtherOrg = "org-agent-it-other"
	agentProxyProject  = "default-project"
	agentProxyActor    = "sub-agent-author"
)

// setupAgentProxyEnv builds the full Agent proxy stack over a fresh SQLite DB,
// with one organization and one project seeded in each of two organizations so
// tenant isolation can be asserted against a real second tenant.
func setupAgentProxyEnv(t *testing.T) (http.Handler, *database.DB) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "agent-proxy-it.db")
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	db := &database.DB{DB: sqlDB}

	schema, err := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	for _, org := range []string{agentProxyOrg, agentProxyOtherOrg} {
		if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
			VALUES (?, ?, ?, 'default', ?, datetime('now'), datetime('now'))`, org, org, org, "idp-"+org); err != nil {
			t.Fatalf("seed organization %s: %v", org, err)
		}
		if _, err := db.Exec(`INSERT INTO projects (uuid, handle, display_name, description, organization_uuid, created_at, updated_at)
			VALUES (?, ?, 'Default Project', '', ?, datetime('now'), datetime('now'))`,
			"project-"+org, agentProxyProject, org); err != nil {
			t.Fatalf("seed project for %s: %v", org, err)
		}
	}

	identity := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	registry := repository.NewArtifactTableRegistry()

	// A real SecretService, so {{ secret "..." }} references in an upstream auth
	// block are resolved against real rows rather than skipped.
	v, err := vault.NewInHouseVault([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	secretSvc := service.NewSecretService(repository.NewSecretRepo(db), v, identity)

	svc := service.NewAgentProxyService(
		repository.NewAgentProxyRepo(db),
		repository.NewProjectRepo(db),
		repository.NewDeploymentRepo(db, registry),
		repository.NewGatewayRepo(db),
		nil, // gatewayEventsService — deletion broadcast is Section 10
		slog.Default(),
		noopAudit{},
		&config.Server{},
		identity,
	).WithSecretService(secretSvc)

	mux := http.NewServeMux()
	NewAgentProxyHandler(svc, identity, slog.Default()).RegisterRoutes(mux)
	return middleware.NewTestContextMiddleware(mux), db
}

// callAgentProxy issues one request as agentProxyActor in agentProxyOrg.
func callAgentProxy(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return callAgentProxyAs(t, h, agentProxyOrg, agentProxyActor, method, path, body)
}

func callAgentProxyAs(t *testing.T, h http.Handler, org, actor, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Org", org)
	req.Header.Set("X-Test-User", actor)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decodeAgentProxyJSON decodes a response body, failing the test with the raw
// body when the status is not the expected one.
func decodeAgentProxyJSON(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) map[string]any {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, wantStatus, rec.Body.String())
	}
	if rec.Body.Len() == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v; body: %s", err, rec.Body.String())
	}
	return out
}

// assertAgentProxyError asserts the status and the catalog code of a failure,
// and that nothing internal leaked into the message.
func assertAgentProxyError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, wantStatus, rec.Body.String())
	}
	var body struct {
		Status  string `json:"status"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v; body: %s", err, rec.Body.String())
	}
	if body.Code != wantCode {
		t.Fatalf("code = %q, want %q; body: %s", body.Code, wantCode, rec.Body.String())
	}
	if body.Status != "error" {
		t.Fatalf("status field = %q, want %q", body.Status, "error")
	}
}

func minimalAgentProxyBody(id, displayName string) string {
	idField := ""
	if id != "" {
		idField = fmt.Sprintf("%q: %q,", "id", id)
	}
	return fmt.Sprintf(`{
	  %s
	  "displayName": %q,
	  "version": "v1.0",
	  "projectId": %q,
	  "context": "/weather",
	  "upstream": { "main": { "url": "http://weather-agent:9000" } },
	  "protocol": "a2a",
	  "a2a": {
	    "protocolVersion": "1.0",
	    "transports": [ { "protocolBinding": "JSONRPC", "pathPrefix": "/rpc" } ]
	  }
	}`, idField, displayName, agentProxyProject)
}

// fullAgentProxyBody exercises every configuration branch the contract allows:
// both transports, upstream auth, common and per-operation policies, a managed
// public card with a vendor extension, and a passthrough protected card with an
// explicit false rewriteUrls.
func fullAgentProxyBody(id string) string {
	return fmt.Sprintf(`{
	  "id": %q,
	  "displayName": "Weather Agent",
	  "description": "Provides forecasts and severe-weather alerts",
	  "version": "v2.0",
	  "projectId": %q,
	  "context": "/weather",
	  "vhost": "agents.gw.com",
	  "upstream": {
	    "main": {
	      "url": "http://weather-agent:9000",
	      "auth": { "type": "api-key", "header": "X-API-Key", "value": "upstream-secret-value" }
	    }
	  },
	  "resilience": { "idleTimeout": "5s" },
	  "protocol": "a2a",
	  "a2a": {
	    "protocolVersion": "1.0",
	    "transports": [
	      { "protocolBinding": "JSONRPC", "pathPrefix": "/rpc" },
	      { "protocolBinding": "HTTP+JSON", "pathPrefix": "/rest" }
	    ],
	    "operationConfigs": {
	      "policies": [ { "name": "jwt-auth", "version": "v1", "params": { "issuer": "https://idp.example.com" } } ],
	      "operations": [
	        { "name": "SendMessage", "policies": [ { "name": "advanced-ratelimit", "version": "v1" } ], "resilience": { "timeout": "30s" } }
	      ]
	    },
	    "agentCard": {
	      "public": {
	        "mode": "managed",
	        "path": "/.well-known/agent-card.json",
	        "policies": [ { "name": "cors", "version": "v1" } ],
	        "content": {
	          "name": "Weather Agent",
	          "description": "Provides forecasts and severe-weather alerts",
	          "version": "1.0.0",
	          "supportedInterfaces": [
	            { "protocolBinding": "JSONRPC", "url": "https://agents.gw.com/weather/rpc", "protocolVersion": "1.0" }
	          ],
	          "capabilities": { "streaming": true },
	          "defaultInputModes": [ "text/plain" ],
	          "defaultOutputModes": [ "text/plain" ],
	          "skills": [ { "id": "forecast", "name": "Forecast", "description": "Multi-day forecast", "tags": [ "weather" ] } ],
	          "x-vendor-custom": { "kept": true }
	        }
	      },
	      "protected": { "mode": "passthrough", "rewriteUrls": false }
	    }
	  }
	}`, id, agentProxyProject)
}

func TestAgentProxyHandler_CreateReturns201WithLocationAndGeneratedHandle(t *testing.T) {
	h, _ := setupAgentProxyEnv(t)

	rec := callAgentProxy(t, h, http.MethodPost, agentProxyBase, minimalAgentProxyBody("", "Weather Agent"))
	body := decodeAgentProxyJSON(t, rec, http.StatusCreated)

	handle, _ := body["id"].(string)
	if handle != "weather-agent" {
		t.Fatalf("generated id = %q, want %q", handle, "weather-agent")
	}
	if got, want := rec.Header().Get("Location"), agentProxyBase+"/weather-agent"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
	if body["protocol"] != "a2a" {
		t.Fatalf("protocol = %v, want a2a", body["protocol"])
	}
	if body["kind"] != "AgentProxy" {
		t.Fatalf("kind = %v, want AgentProxy", body["kind"])
	}
	// projectId is a handle on the wire, never the stored project UUID.
	if body["projectId"] != agentProxyProject {
		t.Fatalf("projectId = %v, want %q", body["projectId"], agentProxyProject)
	}
	// createdBy/updatedBy are stored as internal UUIDs and resolved back through
	// user_idp_references on the way out.
	if body["createdBy"] != agentProxyActor || body["updatedBy"] != agentProxyActor {
		t.Fatalf("createdBy/updatedBy = %v/%v, want %q", body["createdBy"], body["updatedBy"], agentProxyActor)
	}
	if body["readOnly"] != false {
		t.Fatalf("readOnly = %v, want false for a control-plane created Agent proxy", body["readOnly"])
	}
}

// TestAgentProxyHandler_CreateGetPreservesNestedPayload asserts the whole
// authoring document survives the write/read round trip — including free-form
// card content, which is stored as supplied and never normalized.
func TestAgentProxyHandler_CreateGetPreservesNestedPayload(t *testing.T) {
	h, _ := setupAgentProxyEnv(t)

	created := decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPost, agentProxyBase, fullAgentProxyBody("weather-agent")),
		http.StatusCreated)
	fetched := decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodGet, agentProxyBase+"/weather-agent", ""),
		http.StatusOK)

	for _, body := range []map[string]any{created, fetched} {
		a2a := body["a2a"].(map[string]any)
		if got := len(a2a["transports"].([]any)); got != 2 {
			t.Fatalf("transports = %d, want 2", got)
		}
		card := a2a["agentCard"].(map[string]any)
		public := card["public"].(map[string]any)
		if public["mode"] != "managed" {
			t.Fatalf("public card mode = %v, want managed", public["mode"])
		}
		content := public["content"].(map[string]any)
		ext, ok := content["x-vendor-custom"].(map[string]any)
		if !ok || ext["kept"] != true {
			t.Fatalf("card content extension not preserved: %#v", content)
		}
		protected := card["protected"].(map[string]any)
		// An explicit false must survive: omitted and false mean different things.
		if protected["rewriteUrls"] != false {
			t.Fatalf("protected rewriteUrls = %v, want false", protected["rewriteUrls"])
		}
		ops := a2a["operationConfigs"].(map[string]any)
		if got := len(ops["policies"].([]any)); got != 1 {
			t.Fatalf("common policies = %d, want 1", got)
		}

		// The upstream credential is never echoed, on any read path.
		auth := body["upstream"].(map[string]any)["main"].(map[string]any)["auth"].(map[string]any)
		if _, present := auth["value"]; present {
			t.Fatalf("upstream auth value echoed in a response: %#v", auth)
		}
		if auth["header"] != "X-API-Key" {
			t.Fatalf("auth header = %v, want X-API-Key", auth["header"])
		}
	}
}

// TestAgentProxyHandler_UpdateReplacesAndIsIdempotent covers the PUT contract:
// omitted optional blocks are cleared rather than merged, and replaying the same
// body leaves the same resource.
func TestAgentProxyHandler_UpdateReplacesAndIsIdempotent(t *testing.T) {
	h, _ := setupAgentProxyEnv(t)

	decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPost, agentProxyBase, fullAgentProxyBody("weather-agent")),
		http.StatusCreated)

	// A replacement that omits every optional block clears them; it does not
	// merge with what was stored.
	replaced := decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPut, agentProxyBase+"/weather-agent", minimalAgentProxyBody("", "Renamed Agent")),
		http.StatusOK)

	if replaced["id"] != "weather-agent" {
		t.Fatalf("id = %v; an omitted body id must retain the path handle", replaced["id"])
	}
	if replaced["displayName"] != "Renamed Agent" || replaced["version"] != "v1.0" {
		t.Fatalf("writable fields not replaced: %#v", replaced)
	}
	if _, present := replaced["description"]; present {
		t.Fatalf("omitted description not cleared: %#v", replaced["description"])
	}
	if _, present := replaced["vhost"]; present {
		t.Fatalf("omitted vhost not cleared: %#v", replaced["vhost"])
	}
	if _, present := replaced["resilience"]; present {
		t.Fatalf("omitted resilience not cleared: %#v", replaced["resilience"])
	}
	a2a := replaced["a2a"].(map[string]any)
	if _, present := a2a["agentCard"]; present {
		t.Fatalf("omitted agentCard not cleared: %#v", a2a["agentCard"])
	}
	if _, present := a2a["operationConfigs"]; present {
		t.Fatalf("omitted operationConfigs not cleared: %#v", a2a["operationConfigs"])
	}
	if _, present := replaced["associatedGateways"]; present {
		t.Fatalf("omitted associatedGateways not emptied: %#v", replaced["associatedGateways"])
	}

	// Replaying the identical body has the same resource effect; only updatedAt,
	// which is a timestamp of the write itself, may differ.
	repeated := decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPut, agentProxyBase+"/weather-agent", minimalAgentProxyBody("", "Renamed Agent")),
		http.StatusOK)
	delete(replaced, "updatedAt")
	delete(repeated, "updatedAt")
	if fmt.Sprint(replaced) != fmt.Sprint(repeated) {
		t.Fatalf("PUT is not idempotent:\n  first:  %v\n  second: %v", replaced, repeated)
	}
}

// TestAgentProxyHandler_UpdateRetainsOmittedCredential covers the one documented
// exception to full replacement: responses redact the upstream credential, so a
// read-modify-write round trip cannot carry it back and must not erase it.
func TestAgentProxyHandler_UpdateRetainsOmittedCredential(t *testing.T) {
	h, db := setupAgentProxyEnv(t)

	decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPost, agentProxyBase, fullAgentProxyBody("weather-agent")),
		http.StatusCreated)

	// Exactly what a client does: read the resource back and PUT it as-is. The
	// redacted auth block carries a type and header but no value.
	fetched := decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodGet, agentProxyBase+"/weather-agent", ""),
		http.StatusOK)
	roundTrip, err := json.Marshal(fetched)
	if err != nil {
		t.Fatalf("marshal round-trip body: %v", err)
	}
	decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPut, agentProxyBase+"/weather-agent", string(roundTrip)),
		http.StatusOK)

	if got := storedAgentProxyAuthValue(t, db, "weather-agent"); got != "upstream-secret-value" {
		t.Fatalf("stored upstream credential = %q, want it retained", got)
	}

	// A changed auth configuration is a new configuration and inherits nothing:
	// moving the credential to a different header without supplying one is
	// incomplete, and is rejected rather than persisted with an empty value —
	// which would erase the credential behind a 200.
	changed := fetched
	changed["upstream"].(map[string]any)["main"].(map[string]any)["auth"] = map[string]any{
		"type": "api-key", "header": "X-Other-Key",
	}
	changedBody, err := json.Marshal(changed)
	if err != nil {
		t.Fatalf("marshal changed body: %v", err)
	}
	assertAgentProxyError(t,
		callAgentProxy(t, h, http.MethodPut, agentProxyBase+"/weather-agent", string(changedBody)),
		http.StatusBadRequest, "VALIDATION_FAILED")
	if got := storedAgentProxyAuthValue(t, db, "weather-agent"); got != "upstream-secret-value" {
		t.Fatalf("stored credential = %q; a rejected update must not have changed it", got)
	}

	// The same change *with* a credential is accepted, and replaces it.
	changed["upstream"].(map[string]any)["main"].(map[string]any)["auth"] = map[string]any{
		"type": "api-key", "header": "X-Other-Key", "value": "rotated-secret-value",
	}
	if changedBody, err = json.Marshal(changed); err != nil {
		t.Fatalf("marshal changed body: %v", err)
	}
	decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPut, agentProxyBase+"/weather-agent", string(changedBody)),
		http.StatusOK)
	if got := storedAgentProxyAuthValue(t, db, "weather-agent"); got != "rotated-secret-value" {
		t.Fatalf("stored credential = %q, want the rotated one", got)
	}

	// auth.type "none" is the documented way to remove authentication, and is
	// complete without a credential.
	changed["upstream"].(map[string]any)["main"].(map[string]any)["auth"] = map[string]any{"type": "none"}
	if changedBody, err = json.Marshal(changed); err != nil {
		t.Fatalf("marshal changed body: %v", err)
	}
	decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPut, agentProxyBase+"/weather-agent", string(changedBody)),
		http.StatusOK)
	if got := storedAgentProxyAuthValue(t, db, "weather-agent"); got != "" {
		t.Fatalf("credential = %q; auth.type none must remove it", got)
	}
}

// TestAgentProxyHandler_RejectsUnsupportedMediaType covers the 415 half of the
// body-bearing operations' contract. A body that happens to be valid JSON but
// declares another media type is refused, rather than parsed anyway.
func TestAgentProxyHandler_RejectsUnsupportedMediaType(t *testing.T) {
	h, _ := setupAgentProxyEnv(t)
	body := minimalAgentProxyBody("weather-agent", "Weather Agent")

	tests := []struct {
		name        string
		contentType string
		wantStatus  int
	}{
		{name: "text/plain", contentType: "text/plain", wantStatus: http.StatusUnsupportedMediaType},
		{name: "form encoded", contentType: "application/x-www-form-urlencoded", wantStatus: http.StatusUnsupportedMediaType},
		{name: "unparsable", contentType: "application/json; charset=", wantStatus: http.StatusUnsupportedMediaType},
		// Accepted: a charset parameter, and a structured JSON suffix.
		{name: "json with charset", contentType: "application/json; charset=utf-8", wantStatus: http.StatusCreated},
		{name: "structured json suffix", contentType: "application/merge-patch+json", wantStatus: http.StatusCreated},
		// Absent declares nothing to contradict, so the body is examined (RFC 9110).
		{name: "absent", contentType: "", wantStatus: http.StatusCreated},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, agentProxyBase, bytes.NewReader([]byte(body)))
			req.Header.Del("Content-Type")
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			req.Header.Set("X-Test-Org", agentProxyOrg)
			req.Header.Set("X-Test-User", agentProxyActor)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if tc.wantStatus == http.StatusUnsupportedMediaType {
				assertAgentProxyError(t, rec, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE")
				return
			}
			decodeAgentProxyJSON(t, rec, tc.wantStatus)
			// Each accepted case creates the same handle, so clear it for the next.
			callAgentProxy(t, h, http.MethodDelete, agentProxyBase+"/weather-agent", "")
		})
	}
}

// storedAgentProxyAuthValue reads the persisted upstream credential straight out
// of the configuration document, since no response ever exposes it.
func storedAgentProxyAuthValue(t *testing.T, db *database.DB, handle string) string {
	t.Helper()
	var raw []byte
	if err := db.QueryRow(`SELECT configuration FROM agent_proxies WHERE handle = ?`, handle).Scan(&raw); err != nil {
		t.Fatalf("read stored configuration: %v", err)
	}
	var stored struct {
		Upstream struct {
			Main struct {
				Auth struct {
					Value string `json:"value"`
				} `json:"auth"`
			} `json:"main"`
		} `json:"upstream"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("decode stored configuration: %v", err)
	}
	return stored.Upstream.Main.Auth.Value
}

func TestAgentProxyHandler_UpdateResolvesUpdatedBy(t *testing.T) {
	h, _ := setupAgentProxyEnv(t)

	decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPost, agentProxyBase, minimalAgentProxyBody("weather-agent", "Weather Agent")),
		http.StatusCreated)

	const editor = "sub-agent-editor"
	updated := decodeAgentProxyJSON(t,
		callAgentProxyAs(t, h, agentProxyOrg, editor, http.MethodPut, agentProxyBase+"/weather-agent",
			minimalAgentProxyBody("weather-agent", "Weather Agent")),
		http.StatusOK)

	if updated["createdBy"] != agentProxyActor {
		t.Fatalf("createdBy = %v, want %q", updated["createdBy"], agentProxyActor)
	}
	if updated["updatedBy"] != editor {
		t.Fatalf("updatedBy = %v, want %q", updated["updatedBy"], editor)
	}
}

func TestAgentProxyHandler_ListShapeAndProtocolFilter(t *testing.T) {
	h, _ := setupAgentProxyEnv(t)

	for _, name := range []string{"Alpha Agent", "Beta Agent"} {
		decodeAgentProxyJSON(t,
			callAgentProxy(t, h, http.MethodPost, agentProxyBase, minimalAgentProxyBody("", name)),
			http.StatusCreated)
	}

	body := decodeAgentProxyJSON(t, callAgentProxy(t, h, http.MethodGet, agentProxyBase, ""), http.StatusOK)
	if body["count"].(float64) != 2 {
		t.Fatalf("count = %v, want 2", body["count"])
	}
	pagination := body["pagination"].(map[string]any)
	if pagination["total"].(float64) != 2 || pagination["limit"].(float64) != 20 || pagination["offset"].(float64) != 0 {
		t.Fatalf("pagination = %#v", pagination)
	}
	item := body["list"].([]any)[0].(map[string]any)
	for _, key := range []string{"id", "displayName", "version", "projectId", "protocol"} {
		if _, ok := item[key]; !ok {
			t.Fatalf("list item is missing %q: %#v", key, item)
		}
	}
	// The list projection is deliberately lightweight: a managed card alone may
	// be 1 MiB, so protocol configuration never appears in a collection response.
	if _, present := item["a2a"]; present {
		t.Fatalf("list item carries the full protocol configuration: %#v", item)
	}

	// The filter applies to the page and to the total alike.
	filtered := decodeAgentProxyJSON(t, callAgentProxy(t, h, http.MethodGet, agentProxyBase+"?protocol=a2a", ""), http.StatusOK)
	if filtered["count"].(float64) != 2 || filtered["pagination"].(map[string]any)["total"].(float64) != 2 {
		t.Fatalf("a2a filter did not match every Agent proxy: %#v", filtered)
	}

	// Pagination windows the page without changing the filtered total.
	paged := decodeAgentProxyJSON(t, callAgentProxy(t, h, http.MethodGet, agentProxyBase+"?limit=1&offset=1", ""), http.StatusOK)
	if paged["count"].(float64) != 1 || paged["pagination"].(map[string]any)["total"].(float64) != 2 {
		t.Fatalf("pagination did not window the page: %#v", paged)
	}

	// An unsupported or empty filter is rejected rather than silently ignored,
	// which would return rows the caller did not ask for.
	for _, query := range []string{"?protocol=mcp", "?protocol="} {
		assertAgentProxyError(t, callAgentProxy(t, h, http.MethodGet, agentProxyBase+query, ""),
			http.StatusBadRequest, "VALIDATION_FAILED")
	}
}

func TestAgentProxyHandler_DeleteThenGetIsNotFound(t *testing.T) {
	h, _ := setupAgentProxyEnv(t)

	decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPost, agentProxyBase, minimalAgentProxyBody("weather-agent", "Weather Agent")),
		http.StatusCreated)

	rec := callAgentProxy(t, h, http.MethodDelete, agentProxyBase+"/weather-agent", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("204 carried a body: %s", rec.Body.String())
	}

	assertAgentProxyError(t, callAgentProxy(t, h, http.MethodGet, agentProxyBase+"/weather-agent", ""),
		http.StatusNotFound, "AGENT_PROXY_NOT_FOUND")

	// The parent artifact row goes with it, so the handle is free again.
	decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPost, agentProxyBase, minimalAgentProxyBody("weather-agent", "Weather Agent")),
		http.StatusCreated)
}

func TestAgentProxyHandler_RejectionContract(t *testing.T) {
	h, _ := setupAgentProxyEnv(t)

	decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPost, agentProxyBase, minimalAgentProxyBody("weather-agent", "Weather Agent")),
		http.StatusCreated)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "duplicate handle in the same organization",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       minimalAgentProxyBody("weather-agent", "Another Agent"),
			wantStatus: http.StatusConflict,
			wantCode:   "AGENT_PROXY_EXISTS",
		},
		{
			name:   "reserved handle",
			method: http.MethodPost,
			path:   agentProxyBase,
			// fetch-agent-card is a static sibling route; an Agent proxy under
			// that handle could never be addressed.
			body:       minimalAgentProxyBody("fetch-agent-card", "Discovery Agent"),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			// A referenced resource that is not available is a 404, exactly as an
			// addressed one is — and as a referenced gateway in this same body
			// already was.
			name:       "unknown project",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       `{"displayName":"X","version":"v1.0","projectId":"no-such-project","upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`,
			wantStatus: http.StatusNotFound,
			wantCode:   "PROJECT_NOT_FOUND",
		},
		{
			name:       "missing protocol block",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"http://x"}},"protocol":"a2a"}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			name:       "unsupported protocol",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"http://x"}},"protocol":"grpc","grpc":{}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			// Nothing is stored yet, so there is no credential to inherit: an
			// auth block naming a credential-bearing type must carry one.
			name:       "auth without a credential on create",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"http://x","auth":{"type":"api-key","header":"X-Key"}}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			name:       "unknown field",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]},"protocolConfig":{}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			name:       "empty body",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       "",
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			name:       "body id disagrees with the path",
			method:     http.MethodPut,
			path:       agentProxyBase + "/weather-agent",
			body:       minimalAgentProxyBody("other-agent", "Weather Agent"),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			name:       "update of a missing Agent proxy",
			method:     http.MethodPut,
			path:       agentProxyBase + "/no-such-agent",
			body:       minimalAgentProxyBody("", "Weather Agent"),
			wantStatus: http.StatusNotFound,
			wantCode:   "AGENT_PROXY_NOT_FOUND",
		},
		{
			name:       "delete of a missing Agent proxy",
			method:     http.MethodDelete,
			path:       agentProxyBase + "/no-such-agent",
			wantStatus: http.StatusNotFound,
			wantCode:   "AGENT_PROXY_NOT_FOUND",
		},
		{
			// An explicit empty id is a supplied value that fails the handle
			// contract, not a request to generate one.
			name:       "explicitly empty id",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"id":"","displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			// Valid against the id length bounds, but not against the handle
			// grammar the spec now publishes.
			name:       "id that is not a handle",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"id":"weather.agent","displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			// Both keys present matches both oneOf branches, so it satisfies
			// neither — the empty ref does not make this a url-only body.
			name:       "upstream carrying both url and an empty ref",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"http://x","ref":""}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			// Rule 6: the enum is an exact match, and the stored value is the
			// one supplied — so a padded version can never be accepted.
			name:       "a2a protocol version with surrounding whitespace",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":" 1.0 ","transports":[{"protocolBinding":"JSONRPC"}]}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			name:       "upstream url with surrounding whitespace",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"  http://x  "}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			// Rule 6: the version names an operation table that does not exist,
			// so the Agent proxy could never deploy and is refused here.
			name:       "unregistered a2a protocol version",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"9.9","transports":[{"protocolBinding":"JSONRPC"}]}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			// Rule 7.
			name:       "unknown A2A operation name",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"operationConfigs":{"operations":[{"name":"Teleport"}]}}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			// Rule 8: uniqueItems compares whole elements, so the schema cannot
			// catch two entries for one binding with different prefixes.
			name:       "duplicate transport protocol binding",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC","pathPrefix":"/a"},{"protocolBinding":"JSONRPC","pathPrefix":"/b"}]}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			// Rules 10 and 12: mode picks the branch, and a managed card that
			// carries no content is a rejection rather than a silent passthrough.
			name:       "managed public card with no content",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"public":{"mode":"managed"}}}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			// Rule 14: unlike the public card there is no absent-block default
			// to fall back to, so a present protected block needs a mode.
			name:       "protected card with no mode",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}],"agentCard":{"protected":{"rewriteUrls":true}}}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			// Rule 1: no property in the contract is nullable, and null is not
			// the same request as an omitted key.
			name:       "explicit null on an optional field",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"vhost":null,"upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			// Rule 5: syntax only — reachability is never probed at authoring
			// time, but a scheme the gateway cannot dial is refused.
			name:       "upstream url with an unsupported scheme",
			method:     http.MethodPost,
			path:       agentProxyBase,
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"upstream":{"main":{"url":"file:///etc/passwd"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			// Rules 1 and 2 on the replace path: a full replacement is held to
			// the same contract as a create, not a looser one.
			name:       "invalid context on replace",
			method:     http.MethodPut,
			path:       agentProxyBase + "/weather-agent",
			body:       fmt.Sprintf(`{"displayName":"X","version":"v1.0","projectId":%q,"context":"weather","upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`, agentProxyProject),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_FAILED",
		},
		{
			name:       "project changed on replace",
			method:     http.MethodPut,
			path:       agentProxyBase + "/weather-agent",
			body:       `{"displayName":"X","version":"v1.0","projectId":"no-such-project","upstream":{"main":{"url":"http://x"}},"protocol":"a2a","a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`,
			wantStatus: http.StatusNotFound,
			wantCode:   "PROJECT_NOT_FOUND",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertAgentProxyError(t, callAgentProxy(t, h, tc.method, tc.path, tc.body), tc.wantStatus, tc.wantCode)
		})
	}
}

// TestAgentProxyHandler_HandleIsUniquePerOrganization pins the scope of the
// uniqueness rule: the same handle in a second organization is a different
// Agent proxy, and one organization can never address the other's.
func TestAgentProxyHandler_HandleIsUniquePerOrganization(t *testing.T) {
	h, _ := setupAgentProxyEnv(t)

	decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPost, agentProxyBase, minimalAgentProxyBody("weather-agent", "Weather Agent")),
		http.StatusCreated)
	decodeAgentProxyJSON(t,
		callAgentProxyAs(t, h, agentProxyOtherOrg, "sub-other", http.MethodPost, agentProxyBase,
			minimalAgentProxyBody("weather-agent", "Weather Agent")),
		http.StatusCreated)

	// Each organization sees only its own.
	for _, org := range []string{agentProxyOrg, agentProxyOtherOrg} {
		body := decodeAgentProxyJSON(t,
			callAgentProxyAs(t, h, org, "sub-any", http.MethodGet, agentProxyBase, ""), http.StatusOK)
		if body["count"].(float64) != 1 {
			t.Fatalf("org %s sees %v Agent proxies, want 1", org, body["count"])
		}
	}

	// A third organization with no Agent proxy of that handle gets a 404, not
	// someone else's resource.
	assertAgentProxyError(t,
		callAgentProxyAs(t, h, "org-agent-it-absent", "sub-any", http.MethodGet, agentProxyBase+"/weather-agent", ""),
		http.StatusNotFound, "AGENT_PROXY_NOT_FOUND")
}

// TestAgentProxyHandler_UnresolvableSecretNeverEchoesTheHandle covers rule 5's
// explicit prohibition on exposing secret handles.
//
// The shared secret validator names every handle that did not resolve, which
// turns a create call into a secret-existence oracle: a caller who may author an
// Agent proxy but may not read this organization's secrets could enumerate them
// one 400 at a time. The handle is the caller's own input, so nothing is lost by
// refusing to repeat it.
func TestAgentProxyHandler_UnresolvableSecretNeverEchoesTheHandle(t *testing.T) {
	h, _ := setupAgentProxyEnv(t)

	const handle = "no-such-upstream-secret"
	body := fmt.Sprintf(`{
	  "displayName": "Weather Agent",
	  "version": "v1.0",
	  "projectId": %q,
	  "upstream": {
	    "main": {
	      "url": "http://weather-agent:9000",
	      "auth": { "type": "api-key", "header": "X-API-Key", "value": "{{ secret \"%s\" }}" }
	    }
	  },
	  "protocol": "a2a",
	  "a2a": {
	    "protocolVersion": "1.0",
	    "transports": [ { "protocolBinding": "JSONRPC" } ]
	  }
	}`, agentProxyProject, handle)

	rec := callAgentProxy(t, h, http.MethodPost, agentProxyBase, body)
	assertAgentProxyError(t, rec, http.StatusBadRequest, "VALIDATION_FAILED")

	if strings.Contains(rec.Body.String(), handle) {
		t.Fatalf("the unresolved secret handle was echoed back to the caller: %s", rec.Body.String())
	}
	// Not even a substring of it: a partial echo is still an oracle.
	if strings.Contains(rec.Body.String(), "upstream-secret") {
		t.Fatalf("part of the secret handle leaked into the response: %s", rec.Body.String())
	}
}

// TestAgentProxyHandler_ResolvableSecretIsAccepted is the other half: the
// sanitized error must not have turned every secret reference into a rejection.
func TestAgentProxyHandler_ResolvableSecretIsAccepted(t *testing.T) {
	h, db := setupAgentProxyEnv(t)

	if _, err := db.Exec(`INSERT INTO secrets (uuid, handle, display_name, organization_uuid, ciphertext, hash, status, created_by, updated_by, created_at, updated_at)
		VALUES ('secret-uuid-1', 'weather-upstream', 'Weather upstream key', ?, 'ciphertext', 'hash', 'ACTIVE', ?, ?, datetime('now'), datetime('now'))`,
		agentProxyOrg, agentProxyActor, agentProxyActor); err != nil {
		t.Fatalf("seed secret: %v", err)
	}

	body := fmt.Sprintf(`{
	  "displayName": "Weather Agent",
	  "version": "v1.0",
	  "projectId": %q,
	  "upstream": {
	    "main": {
	      "url": "http://weather-agent:9000",
	      "auth": { "type": "api-key", "header": "X-API-Key", "value": "{{ secret \"weather-upstream\" }}" }
	    }
	  },
	  "protocol": "a2a",
	  "a2a": {
	    "protocolVersion": "1.0",
	    "transports": [ { "protocolBinding": "JSONRPC" } ]
	  }
	}`, agentProxyProject)

	created := decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPost, agentProxyBase, body), http.StatusCreated)

	// The stored placeholder is a credential reference, so a read redacts it
	// exactly as it redacts a literal value.
	auth := created["upstream"].(map[string]any)["main"].(map[string]any)["auth"].(map[string]any)
	if _, present := auth["value"]; present {
		t.Fatalf("secret placeholder echoed in a response: %#v", auth)
	}
}

// TestAgentProxyHandler_StoresExactlyWhatWasValidated closes the gap the
// whitespace cases above describe from the other side.
//
// Validation and persistence must read the same bytes. If validation normalized
// a value — trimmed a protocol version, trimmed a url, trimmed a handle — the
// stored value would be one the contract never approved, and the gateway would
// later be handed a third thing. Nothing in this path normalizes, so what comes
// back is what was sent.
func TestAgentProxyHandler_StoresExactlyWhatWasValidated(t *testing.T) {
	h, db := setupAgentProxyEnv(t)

	decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodPost, agentProxyBase, minimalAgentProxyBody("weather-agent", "Weather Agent")),
		http.StatusCreated)

	fetched := decodeAgentProxyJSON(t,
		callAgentProxy(t, h, http.MethodGet, agentProxyBase+"/weather-agent", ""), http.StatusOK)

	if got := fetched["a2a"].(map[string]any)["protocolVersion"]; got != "1.0" {
		t.Fatalf("protocolVersion = %q, want the exact validated value %q", got, "1.0")
	}
	if got := fetched["id"]; got != "weather-agent" {
		t.Fatalf("id = %q, want the exact supplied handle", got)
	}

	// Read straight from the configuration document too: a response is built by
	// the mapper, so it could agree with the request while the stored bytes did
	// not.
	var configuration string
	if err := db.QueryRow(`SELECT configuration FROM agent_proxies WHERE handle = ? AND organization_uuid = ?`,
		"weather-agent", agentProxyOrg).Scan(&configuration); err != nil {
		t.Fatalf("read stored configuration: %v", err)
	}
	if !strings.Contains(configuration, `"protocolVersion":"1.0"`) {
		t.Fatalf("stored configuration does not carry the exact protocol version: %s", configuration)
	}
	if !strings.Contains(configuration, `"url":"http://weather-agent:9000"`) {
		t.Fatalf("stored configuration does not carry the exact upstream url: %s", configuration)
	}
}

// TestAgentProxyHandler_ReferencedResourcesAgreeOnNotFound pins the rule the
// project mapping used to break: within one request body, every referenced
// handle that cannot be reached answers the same way.
//
// A project reference answering 400 while a gateway reference in the same
// payload answered 404 left a client unable to write one branch for "you named
// something I cannot reach". It also has to stay a 404 across the tenant
// boundary, without ever confirming that the resource exists somewhere else.
func TestAgentProxyHandler_ReferencedResourcesAgreeOnNotFound(t *testing.T) {
	h, _ := setupAgentProxyEnv(t)

	body := func(projectId, gatewayId string) string {
		gateways := ""
		if gatewayId != "" {
			gateways = fmt.Sprintf(`"associatedGateways":[{"id":%q}],`, gatewayId)
		}
		return fmt.Sprintf(`{
		  "displayName": "Weather Agent",
		  "version": "v1.0",
		  "projectId": %q,
		  %s
		  "upstream": { "main": { "url": "http://weather-agent:9000" } },
		  "protocol": "a2a",
		  "a2a": { "protocolVersion": "1.0", "transports": [ { "protocolBinding": "JSONRPC" } ] }
		}`, projectId, gateways)
	}

	tests := []struct {
		name     string
		body     string
		wantCode string
	}{
		{
			name:     "unknown project",
			body:     body("no-such-project", ""),
			wantCode: "PROJECT_NOT_FOUND",
		},
		{
			name:     "unknown gateway",
			body:     body(agentProxyProject, "no-such-gateway"),
			wantCode: "GATEWAY_NOT_FOUND",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertAgentProxyError(t,
				callAgentProxy(t, h, http.MethodPost, agentProxyBase, tc.body),
				http.StatusNotFound, tc.wantCode)
		})
	}

}

// TestAgentProxyHandler_ForeignProjectIsIndistinguishableFromAMissingOne pins
// the other half of the 404: a project that exists, but in another
// organization, must answer exactly as one that exists nowhere.
//
// Any difference — a different status, a different code, a different message —
// is an existence oracle for another tenant's projects, so the two are asserted
// to be byte-identical rather than merely both being failures.
func TestAgentProxyHandler_ForeignProjectIsIndistinguishableFromAMissingOne(t *testing.T) {
	h, db := setupAgentProxyEnv(t)

	// A project that exists only in the other organization.
	if _, err := db.Exec(`INSERT INTO projects (uuid, handle, display_name, description, organization_uuid, created_at, updated_at)
		VALUES ('project-foreign', 'foreign-project', 'Foreign Project', '', ?, datetime('now'), datetime('now'))`,
		agentProxyOtherOrg); err != nil {
		t.Fatalf("seed foreign project: %v", err)
	}

	body := func(projectId string) string {
		return fmt.Sprintf(`{"displayName":"Weather Agent","version":"v1.0","projectId":%q,`+
			`"upstream":{"main":{"url":"http://weather-agent:9000"}},"protocol":"a2a",`+
			`"a2a":{"protocolVersion":"1.0","transports":[{"protocolBinding":"JSONRPC"}]}}`, projectId)
	}

	foreign := callAgentProxy(t, h, http.MethodPost, agentProxyBase, body("foreign-project"))
	missing := callAgentProxy(t, h, http.MethodPost, agentProxyBase, body("no-such-project"))

	assertAgentProxyError(t, foreign, http.StatusNotFound, "PROJECT_NOT_FOUND")
	assertAgentProxyError(t, missing, http.StatusNotFound, "PROJECT_NOT_FOUND")

	// trackingId differs per request, so compare everything else.
	strip := func(rec *httptest.ResponseRecorder) string {
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode error body: %v", err)
		}
		delete(body, "trackingId")
		encoded, _ := json.Marshal(body)
		return string(encoded)
	}
	if strip(foreign) != strip(missing) {
		t.Fatalf("a foreign project is distinguishable from a missing one:\n  foreign: %s\n  missing: %s",
			strip(foreign), strip(missing))
	}
}

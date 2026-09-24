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

// End-to-end Agent Card fetch over the real route -> handler -> service ->
// repository stack, against a real upstream (httptest) and a real secret store.
//
// The fetch is a preview/display path, so two properties get as much attention
// here as the happy path: it persists nothing, and an unreachable upstream is a
// reported 503 rather than a 500 — the control plane is healthy, it just could
// not reach the agent.

package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2/api-platform/httpkit/httpclient"
	"github.com/wso2/api-platform/httpkit/netguard"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"
	"github.com/wso2/api-platform/platform-api/internal/vault"
)

const (
	fetchAgentCardPath = agentProxyBase + "/fetch-agent-card"
	upstreamSecretName = "weather-upstream"
	upstreamSecretVal  = "stored-preview-key"
)

// upstreamAgentCard is a complete A2A card with deliberately unsorted keys and a
// vendor extension, so a byte comparison can tell a verbatim pass-through from a
// re-encode.
const upstreamAgentCard = `{"version":"1.0.0","name":"Weather Agent","description":"Forecasts",` +
	`"supportedInterfaces":[{"protocolBinding":"JSONRPC","url":"https://agents.example.com/rpc","protocolVersion":"1.0"}],` +
	`"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],` +
	`"skills":[{"id":"forecast","name":"Forecast","description":"Multi-day","tags":["weather"]}],` +
	`"x-vendor-custom":{"kept":true}}`

// agentProxyTestEnv is the assembled Agent proxy stack a test drives.
type agentProxyTestEnv struct {
	handler http.Handler
	db      *database.DB
	vault   vault.SecretVault
	// deployLogs captures the deployment service's log output, so a test can
	// assert what a deploy reported.
	deployLogs *lockedBuffer
	// hub is the real SQL-backed EventHub gateway events are published to,
	// wrapped so a test can make publishing fail.
	hub *faultyEventHub
}

// newAgentProxyTestEnv builds the full Agent proxy stack over a fresh SQLite DB,
// with one organization and one project seeded in each of two organizations so
// tenant isolation can be asserted against a real second tenant.
//
// cfg is the server configuration the service is constructed from, which is what
// lets a test drive the Agent Card display cache's own knobs.
func newAgentProxyTestEnv(t *testing.T, cfg *config.Server) *agentProxyTestEnv {
	t.Helper()

	initSharedHTTPClientForTests(t)

	dbPath := filepath.Join(t.TempDir(), "agent-proxy-it.db")
	// The EventHub polls this DB from its own goroutines, so writers wait on a
	// lock rather than failing with SQLITE_BUSY.
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_busy_timeout=5000")
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

	hub := newFaultyEventHub(t, sqlDB)
	gatewayEvents := service.NewGatewayEventsService(hub, identity, slog.Default())

	svc := service.NewAgentProxyService(
		repository.NewAgentProxyRepo(db),
		repository.NewProjectRepo(db),
		repository.NewDeploymentRepo(db, registry),
		repository.NewGatewayRepo(db),
		gatewayEvents,
		slog.Default(),
		noopAudit{},
		cfg,
		identity,
	).WithSecretService(secretSvc)

	deployLogs := &lockedBuffer{}
	agentRepo := repository.NewAgentProxyRepo(db)
	deploymentRepo := repository.NewDeploymentRepo(db, registry)
	deploySvc := service.NewAgentDeploymentService(
		agentRepo,
		deploymentRepo,
		repository.NewGatewayRepo(db),
		repository.NewArtifactRepo(db, registry),
		repository.NewAPIKeyRepo(db, registry),
		gatewayEvents,
		service.NewArtifactDefinitions(service.NewAgentProxyDefinition(agentRepo, &utils.AgentProxyUtils{})),
		cfg,
		slog.New(slog.NewJSONHandler(deployLogs, nil)),
	)

	mux := http.NewServeMux()
	NewAgentProxyHandler(svc, identity, slog.Default()).RegisterRoutes(mux)
	NewAgentProxyDeploymentHandler(deploySvc, identity, slog.Default()).RegisterRoutes(mux)
	return &agentProxyTestEnv{
		handler:    middleware.NewTestContextMiddleware(mux),
		db:         db,
		vault:      v,
		deployLogs: deployLogs,
		hub:        hub,
	}
}

// initSharedHTTPClientForTests installs this package's test double for the
// single shared outbound client, mirroring internal/utils's own TestMain: SSRF
// on under PermitPrivateBlockMetadata, so an httptest loopback server is
// reachable exactly as a real in-cluster upstream would be.
func initSharedHTTPClientForTests(t *testing.T) {
	t.Helper()

	policy := netguard.PermitPrivateBlockMetadata()
	policy.AllowedSchemes = []string{"http", "https"}

	cfg := httpclient.DefaultConfig()
	cfg.Pooling.DisableKeepAlives = true
	cfg.Timeouts.MaxResponseBytes = -1
	cfg.SSRF.Enabled = true
	cfg.SSRF.Policy = policy
	cfg.SSRF.MaxRedirects = 5

	client, err := httpclient.New(cfg)
	if err != nil {
		t.Fatalf("build the shared HTTP client test double: %v", err)
	}
	utils.InitSharedHTTPClient(client, 0)
}

// cardCacheConfig is a server configuration with the display cache switched on.
func cardCacheConfig(positiveTTL, negativeTTL time.Duration) *config.Server {
	return &config.Server{
		AgentCardCache: config.AgentCardCache{
			PositiveTTL: positiveTTL,
			NegativeTTL: negativeTTL,
			MaxEntries:  16,
			MaxBytes:    1 << 20,
		},
	}
}

// upstreamAgent is a stand-in for the agent the control plane fetches from. It
// counts requests, so a test can assert that a call did *not* reach it — which
// is how both "rejected before any outbound request" and "served from cache" are
// established.
type upstreamAgent struct {
	server   *httptest.Server
	requests atomic.Int64
	lastAuth atomic.Value // string
	lastPath atomic.Value // string
}

func newUpstreamAgent(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *upstreamAgent {
	t.Helper()

	agent := &upstreamAgent{}
	agent.lastAuth.Store("")
	agent.lastPath.Store("")
	agent.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent.requests.Add(1)
		agent.lastAuth.Store(r.Header.Get("X-API-Key"))
		agent.lastPath.Store(r.URL.Path)
		handler(w, r)
	}))
	t.Cleanup(agent.server.Close)
	return agent
}

// newCardServingAgent serves a valid Agent Card on the well-known path.
func newCardServingAgent(t *testing.T) *upstreamAgent {
	t.Helper()
	return newUpstreamAgent(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upstreamAgentCard))
	})
}

func (a *upstreamAgent) url() string           { return a.server.URL }
func (a *upstreamAgent) count() int64          { return a.requests.Load() }
func (a *upstreamAgent) credential() string    { return a.lastAuth.Load().(string) }
func (a *upstreamAgent) requestedPath() string { return a.lastPath.Load().(string) }

// callFetchAgentCard issues one fetch as agentProxyActor in agentProxyOrg.
func callFetchAgentCard(t *testing.T, env *agentProxyTestEnv, body string) *httptest.ResponseRecorder {
	t.Helper()
	return callFetchAgentCardAs(t, env, agentProxyOrg, body, nil)
}

func callFetchAgentCardAs(t *testing.T, env *agentProxyTestEnv, org, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, fetchAgentCardPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Org", org)
	req.Header.Set("X-Test-User", agentProxyActor)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	return rec
}

// seedUpstreamSecret stores an encrypted credential the Agent proxy can point at
// with a {{ secret "handle" }} placeholder.
func seedUpstreamSecret(t *testing.T, env *agentProxyTestEnv, org string) {
	t.Helper()

	ciphertext, err := env.vault.Encrypt(context.Background(), upstreamSecretVal)
	if err != nil {
		t.Fatalf("encrypt upstream secret: %v", err)
	}
	if _, err := env.db.Exec(`INSERT INTO secrets (uuid, handle, display_name, description, organization_uuid, ciphertext, hash, status, created_by, updated_by, created_at, updated_at)
		VALUES (?, ?, 'Weather upstream key', '', ?, ?, 'hash', 'ACTIVE', ?, ?, datetime('now'), datetime('now'))`,
		"secret-"+org, upstreamSecretName, org, ciphertext, agentProxyActor, agentProxyActor); err != nil {
		t.Fatalf("seed secret for %s: %v", org, err)
	}
}

// createAgentProxyPointingAt saves a passthrough Agent proxy whose upstream is
// the given URL, optionally authenticating with the seeded stored secret.
func createAgentProxyPointingAt(t *testing.T, env *agentProxyTestEnv, org, handle, upstreamURL string, withStoredAuth bool) {
	t.Helper()

	auth := ""
	if withStoredAuth {
		auth = fmt.Sprintf(`, "auth": { "type": "api-key", "header": "X-API-Key", "value": "{{ secret \"%s\" }}" }`, upstreamSecretName)
	}
	body := fmt.Sprintf(`{
	  "id": %q,
	  "displayName": "Weather Agent",
	  "version": "v1.0",
	  "projectId": %q,
	  "context": "/weather",
	  "upstream": { "main": { "url": %q%s } },
	  "protocol": "a2a",
	  "a2a": {
	    "protocolVersion": "1.0",
	    "transports": [ { "protocolBinding": "JSONRPC", "pathPrefix": "/rpc" } ]
	  }
	}`, handle, agentProxyProject, upstreamURL, auth)

	rec := callAgentProxyAs(t, env.handler, org, agentProxyActor, http.MethodPost, agentProxyBase, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create Agent proxy %q in %s: status = %d; body: %s", handle, org, rec.Code, rec.Body.String())
	}
}

// assertFetchedCard asserts a 200 whose body is the upstream's own bytes.
func assertFetchedCard(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != upstreamAgentCard {
		t.Errorf("card was not returned verbatim:\n got %s\nwant %s", got, upstreamAgentCard)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
}

// headerAge reads the Age header as an integer, failing when it is absent.
func headerAge(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()

	raw := rec.Header().Get("Age")
	if raw == "" {
		t.Fatalf("Age header is absent; headers: %v", rec.Header())
	}
	age, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("Age header %q is not an integer: %v", raw, err)
	}
	return age
}

// ---------------------------------------------------------------------------
// Direct-URL form
// ---------------------------------------------------------------------------

func TestFetchAgentCard_DirectURLReturnsTheCardVerbatim(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newCardServingAgent(t)

	rec := callFetchAgentCard(t, env, fmt.Sprintf(`{"url":%q}`, agent.url()))
	assertFetchedCard(t, rec)

	if got := agent.requestedPath(); got != utils.AgentCardWellKnownPath {
		t.Errorf("upstream path = %q, want %q", got, utils.AgentCardWellKnownPath)
	}
	// The direct form is an authoring action against a URL no stored resource
	// owns, so there is nothing to key a cache entry on and no freshness to
	// report. Reporting a max-age here would misdescribe it.
	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control = %q, want it absent on the direct-URL form", got)
	}
	if got := rec.Header().Get("Age"); got != "" {
		t.Errorf("Age = %q, want it absent on the direct-URL form", got)
	}

	// And it is never served from cache: a repeat contacts the upstream again.
	assertFetchedCard(t, callFetchAgentCard(t, env, fmt.Sprintf(`{"url":%q}`, agent.url())))
	if got := agent.count(); got != 2 {
		t.Errorf("upstream requests = %d, want 2 — the direct-URL form must never be cached", got)
	}
}

func TestFetchAgentCard_DirectURLSendsTheSuppliedCredential(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newCardServingAgent(t)

	body := fmt.Sprintf(`{"url":%q,"auth":{"type":"api-key","header":"X-API-Key","value":"preview-key"}}`, agent.url())
	assertFetchedCard(t, callFetchAgentCard(t, env, body))

	if got := agent.credential(); got != "preview-key" {
		t.Errorf("upstream received credential %q, want %q", got, "preview-key")
	}
}

// An unsaved endpoint preview cannot borrow a stored credential: the direct form
// carries its own or none at all.
func TestFetchAgentCard_DirectURLDoesNotBorrowStoredCredentials(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newCardServingAgent(t)
	seedUpstreamSecret(t, env, agentProxyOrg)
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), true)

	assertFetchedCard(t, callFetchAgentCard(t, env, fmt.Sprintf(`{"url":%q}`, agent.url())))
	if got := agent.credential(); got != "" {
		t.Errorf("upstream received credential %q on a direct fetch, want none", got)
	}
}

// ---------------------------------------------------------------------------
// Stored-handle form
// ---------------------------------------------------------------------------

func TestFetchAgentCard_StoredUsesTheStoredEndpointAndCredentials(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newCardServingAgent(t)
	seedUpstreamSecret(t, env, agentProxyOrg)
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), true)

	rec := callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`)
	assertFetchedCard(t, rec)

	if got := agent.credential(); got != upstreamSecretVal {
		t.Errorf("upstream received credential %q, want the resolved stored secret %q", got, upstreamSecretVal)
	}
	// The stored value is a {{ secret "handle" }} placeholder, and neither the
	// handle nor the resolved credential may appear in a response body.
	body := rec.Body.String()
	for _, leak := range []string{upstreamSecretName, upstreamSecretVal, "{{ secret"} {
		if strings.Contains(body, leak) {
			t.Errorf("response leaks %q: %s", leak, body)
		}
	}
	if got := rec.Header().Get("Cache-Control"); got != "max-age=60" {
		t.Errorf("Cache-Control = %q, want %q", got, "max-age=60")
	}
	if age := headerAge(t, rec); age != 0 {
		t.Errorf("Age = %d on a freshly fetched card, want 0", age)
	}
}

// The fetch is a preview: nothing about the Agent proxy changes because of it.
func TestFetchAgentCard_PersistsNothing(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newCardServingAgent(t)
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), false)

	before := agentProxyRow(t, env.db, "weather-agent")

	assertFetchedCard(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`))

	after := agentProxyRow(t, env.db, "weather-agent")
	if before != after {
		t.Errorf("the Agent proxy row changed across a display fetch:\nbefore %+v\nafter  %+v", before, after)
	}
	// The card is transient page state, so it is not written anywhere — in
	// particular it does not become managed card content.
	if strings.Contains(after.configuration, "agentCard") {
		t.Errorf("a fetched card was written into stored configuration: %s", after.configuration)
	}
	assertNoDeploymentRows(t, env.db)
}

// An upstream given by ref names a definition only the gateway resolves, so
// there is no address here to fetch. That is a property of the stored
// configuration rather than a transient upstream problem, so it is a 400.
func TestFetchAgentCard_StoredUpstreamByRefIsRejected(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))

	body := fmt.Sprintf(`{
	  "id": "weather-agent",
	  "displayName": "Weather Agent",
	  "version": "v1.0",
	  "projectId": %q,
	  "upstream": { "main": { "ref": "weather-upstream-definition" } },
	  "protocol": "a2a",
	  "a2a": { "protocolVersion": "1.0", "transports": [ { "protocolBinding": "JSONRPC" } ] }
	}`, agentProxyProject)
	if rec := callAgentProxy(t, env.handler, http.MethodPost, agentProxyBase, body); rec.Code != http.StatusCreated {
		t.Fatalf("create by-ref Agent proxy: status = %d; body: %s", rec.Code, rec.Body.String())
	}

	assertAgentProxyError(t,
		callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`),
		http.StatusBadRequest, "VALIDATION_FAILED")
}

// Agent Card discovery is an A2A concept, so a stored Agent proxy that does not
// speak A2A must not be sent down this path.
//
// Today the refusal lands before the service's own guard: the repository will
// not load a row whose stored protocol is not one this platform version knows,
// and A2A is the only one registered. So this asserts the property that matters
// here — the fetch never reaches an upstream for a non-A2A Agent proxy — rather
// than a specific status that belongs to the repository's parse, not to this
// operation. The service's own protocol check (fetchStoredAgentCard) is what
// will answer 400 once a second protocol registers and such a row loads cleanly.
func TestFetchAgentCard_StoredNonA2AProtocolNeverReachesTheUpstream(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newCardServingAgent(t)
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), false)

	// Written directly, because no second protocol is registered yet — the guard
	// exists for the one that will be.
	if _, err := env.db.Exec(`UPDATE agent_proxies SET protocol = 'acp' WHERE handle = ?`, "weather-agent"); err != nil {
		t.Fatalf("rewrite stored protocol: %v", err)
	}

	rec := callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`)
	if rec.Code == http.StatusOK {
		t.Fatalf("a non-A2A Agent proxy was served an Agent Card: %s", rec.Body.String())
	}
	if got := agent.count(); got != 0 {
		t.Errorf("upstream requests = %d, want 0 — the protocol check precedes the fetch", got)
	}
}

// ---------------------------------------------------------------------------
// Request validation
// ---------------------------------------------------------------------------

// Every invalid combination is refused before a credential is resolved or a
// request is issued — which is what the request counter here establishes.
func TestFetchAgentCard_InvalidRequestsAreRejectedBeforeAnyOutboundRequest(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newCardServingAgent(t)
	seedUpstreamSecret(t, env, agentProxyOrg)
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), true)

	tests := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ``},
		{name: "empty object", body: `{}`},
		{name: "auth alone", body: `{"auth":{"type":"api-key","header":"X-API-Key","value":"k"}}`},
		{name: "handle plus url", body: `{"agentProxyId":"weather-agent","url":"http://elsewhere:9000"}`},
		{name: "handle plus auth", body: `{"agentProxyId":"weather-agent","auth":{"type":"none"}}`},
		{name: "handle plus null url", body: `{"agentProxyId":"weather-agent","url":null}`},
		{name: "handle plus null auth", body: `{"agentProxyId":"weather-agent","auth":null}`},
		{name: "all three fields", body: `{"agentProxyId":"weather-agent","url":"http://elsewhere:9000","auth":{"type":"none"}}`},
		{name: "all three with nulls", body: `{"agentProxyId":"weather-agent","url":null,"auth":null}`},
		{name: "null handle", body: `{"agentProxyId":null}`},
		{name: "empty handle", body: `{"agentProxyId":""}`},
		{name: "empty url", body: `{"url":""}`},
		{name: "unknown field", body: `{"url":"http://elsewhere:9000","follow":true}`},
		{name: "malformed url", body: `{"url":"not a url"}`},
		{name: "unsupported scheme", body: `{"url":"file:///etc/passwd"}`},
		{name: "not json", body: `this is not json`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertAgentProxyError(t, callFetchAgentCard(t, env, tc.body),
				http.StatusBadRequest, "VALIDATION_FAILED")
		})
	}

	if got := agent.count(); got != 0 {
		t.Errorf("upstream requests = %d, want 0 — every rejection must precede the outbound request", got)
	}
}

// A handle the caller cannot reach is a 404 whether it does not exist or belongs
// to another organization: telling those apart would confirm another tenant's
// resources.
func TestFetchAgentCard_UnreachableHandleIs404WithNoOutboundRequest(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newCardServingAgent(t)
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), false)

	t.Run("unknown handle", func(t *testing.T) {
		assertAgentProxyError(t, callFetchAgentCard(t, env, `{"agentProxyId":"no-such-agent"}`),
			http.StatusNotFound, "AGENT_PROXY_NOT_FOUND")
	})
	t.Run("another organization's handle", func(t *testing.T) {
		rec := callFetchAgentCardAs(t, env, agentProxyOtherOrg, `{"agentProxyId":"weather-agent"}`, nil)
		assertAgentProxyError(t, rec, http.StatusNotFound, "AGENT_PROXY_NOT_FOUND")
	})

	if got := agent.count(); got != 0 {
		t.Errorf("upstream requests = %d, want 0", got)
	}
}

func TestFetchAgentCard_RejectsAnUnsupportedMediaType(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newCardServingAgent(t)

	rec := callFetchAgentCardAs(t, env, agentProxyOrg, fmt.Sprintf(`{"url":%q}`, agent.url()),
		map[string]string{"Content-Type": "application/xml"})
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415; body: %s", rec.Code, rec.Body.String())
	}
	if got := agent.count(); got != 0 {
		t.Errorf("upstream requests = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// Upstream failures
// ---------------------------------------------------------------------------

// Every way the upstream can fail the control plane is a 503, never a 500 and
// never a 404 — the Agent proxy exists and the control plane is healthy. A
// client needs that distinction to render "the control plane could not fetch the
// card" rather than "this agent is down".
func TestFetchAgentCard_UpstreamFailuresAre503(t *testing.T) {
	tests := []struct {
		name    string
		handler func(w http.ResponseWriter, r *http.Request)
		// closed makes the upstream unreachable outright (connection refused).
		closed bool
	}{
		{name: "connection refused", closed: true},
		{name: "upstream 500", handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }},
		{name: "upstream 404", handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }},
		{
			name: "credentials rejected",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"bad key sk-live-9f3a"}`))
			},
		},
		{
			name: "200 that is not a card",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`<html><body>Service temporarily at 10.0.3.14</body></html>`))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
			seedUpstreamSecret(t, env, agentProxyOrg)

			upstreamURL := ""
			if tc.closed {
				dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
				upstreamURL = dead.URL
				dead.Close()
			} else {
				upstreamURL = newUpstreamAgent(t, tc.handler).url()
			}
			createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", upstreamURL, true)
			before := agentProxyRow(t, env.db, "weather-agent")

			rec := callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`)
			assertAgentProxyError(t, rec, http.StatusServiceUnavailable, "AGENT_PROXY_UPSTREAM_UNREACHABLE")

			var body struct {
				Message    string `json:"message"`
				TrackingID string `json:"trackingId"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error body: %v; body: %s", err, rec.Body.String())
			}
			// A 5xx carries a tracking id so the sterile message can be correlated
			// with the log line that holds the real cause.
			if body.TrackingID == "" {
				t.Errorf("trackingId is absent from a 503: %s", rec.Body.String())
			}
			// Nothing about the upstream escapes: not its address, not the
			// credential presented to it, not its own response body.
			for _, leak := range []string{upstreamURL, upstreamSecretVal, upstreamSecretName, "sk-live-9f3a", "10.0.3.14", "<html"} {
				if leak != "" && strings.Contains(rec.Body.String(), leak) {
					t.Errorf("the 503 body leaks %q: %s", leak, rec.Body.String())
				}
			}

			// A failed display fetch is not a validation or deployment error: it
			// changes nothing stored and creates no deployment state.
			if after := agentProxyRow(t, env.db, "weather-agent"); before != after {
				t.Errorf("a failed fetch changed the stored Agent proxy:\nbefore %+v\nafter  %+v", before, after)
			}
			assertNoDeploymentRows(t, env.db)
		})
	}
}

// The control plane does not police what is inside a card. Its shape is the A2A
// specification's to define and the gateway's to enforce at deploy time, and a
// preview that refused to render a sparse or unfamiliar document would hide the
// very thing the author opened the page to look at.
func TestFetchAgentCard_ShowsADocumentTheControlPlaneWouldNotHaveAuthored(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))

	const sparse = `{"name":"Weather Agent","x-vendor-custom":{"kept":true}}`
	agent := newUpstreamAgent(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(sparse))
	})
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), false)

	rec := callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != sparse {
		t.Errorf("card = %s, want it passed through unchanged as %s", got, sparse)
	}
}

// ---------------------------------------------------------------------------
// Display cache
// ---------------------------------------------------------------------------

func TestFetchAgentCard_SecondCallInsideTheTTLIsServedFromCache(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newCardServingAgent(t)
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), false)

	assertFetchedCard(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`))
	assertFetchedCard(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`))

	if got := agent.count(); got != 1 {
		t.Errorf("upstream requests = %d, want 1 — the second call must be served from cache", got)
	}
}

func TestFetchAgentCard_FailuresAreCachedToo(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newUpstreamAgent(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), false)

	for range 3 {
		assertAgentProxyError(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`),
			http.StatusServiceUnavailable, "AGENT_PROXY_UPSTREAM_UNREACHABLE")
	}
	// Without a negative cache a down upstream is re-contacted on every single
	// page view, which is the exact load the cache exists to remove.
	if got := agent.count(); got != 1 {
		t.Errorf("upstream requests = %d, want 1 — a failure must be cached too", got)
	}
}

// Age is reported on a cached success and on a cached failure alike, which is
// what lets a client say how long the upstream has been unreachable.
func TestFetchAgentCard_CachedResponsesReportTheirAge(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, time.Minute))
	card := newCardServingAgent(t)
	broken := newUpstreamAgent(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", card.url(), false)
	createAgentProxyPointingAt(t, env, agentProxyOrg, "broken-agent", broken.url(), false)

	assertFetchedCard(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`))
	assertAgentProxyError(t, callFetchAgentCard(t, env, `{"agentProxyId":"broken-agent"}`),
		http.StatusServiceUnavailable, "AGENT_PROXY_UPSTREAM_UNREACHABLE")

	// Age is whole seconds, so a repeat has to land in the next one to be
	// distinguishable from a fresh fetch at all.
	time.Sleep(1050 * time.Millisecond)

	cachedCard := callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`)
	assertFetchedCard(t, cachedCard)
	if age := headerAge(t, cachedCard); age < 1 {
		t.Errorf("Age = %d on a cached card, want a positive value", age)
	}

	cachedFailure := callFetchAgentCard(t, env, `{"agentProxyId":"broken-agent"}`)
	assertAgentProxyError(t, cachedFailure, http.StatusServiceUnavailable, "AGENT_PROXY_UPSTREAM_UNREACHABLE")
	if age := headerAge(t, cachedFailure); age < 1 {
		t.Errorf("Age = %d on a cached failure, want a positive value", age)
	}

	if got := card.count(); got != 1 {
		t.Errorf("card upstream requests = %d, want 1", got)
	}
	if got := broken.count(); got != 1 {
		t.Errorf("failing upstream requests = %d, want 1", got)
	}
}

// A 503 reports Age but no Cache-Control: a failure is held for the negative
// TTL, and reporting the success path's positive TTL beside it would tell the
// client the wrong number.
func TestFetchAgentCard_FailuresReportAgeWithoutAMisleadingMaxAge(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newUpstreamAgent(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), false)

	rec := callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`)
	assertAgentProxyError(t, rec, http.StatusServiceUnavailable, "AGENT_PROXY_UPSTREAM_UNREACHABLE")
	if age := headerAge(t, rec); age != 0 {
		t.Errorf("Age = %d on a freshly observed failure, want 0", age)
	}
	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control = %q on a 503, want it absent — the positive TTL does not apply to a failure", got)
	}
}

func TestFetchAgentCard_EntriesAreRefetchedOnceTheTTLLapses(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(150*time.Millisecond, 150*time.Millisecond))
	agent := newCardServingAgent(t)
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), false)

	assertFetchedCard(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`))
	time.Sleep(250 * time.Millisecond)
	assertFetchedCard(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`))

	if got := agent.count(); got != 2 {
		t.Errorf("upstream requests = %d, want 2 — a lapsed entry must be refetched", got)
	}
}

// Cache-Control: no-cache is the authoring UI's explicit refresh, and the only
// bypass there is. It forces a live fetch and refreshes the entry.
func TestFetchAgentCard_NoCacheForcesALiveFetchAndRefreshesTheEntry(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newCardServingAgent(t)
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), false)

	assertFetchedCard(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`))
	assertFetchedCard(t, callFetchAgentCardAs(t, env, agentProxyOrg, `{"agentProxyId":"weather-agent"}`,
		map[string]string{"Cache-Control": "no-cache"}))
	if got := agent.count(); got != 2 {
		t.Errorf("upstream requests = %d, want 2 — no-cache must force a live fetch", got)
	}

	// The refreshed entry is what the next ordinary call reads, so the bypass
	// does not leave the cache empty behind it.
	assertFetchedCard(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`))
	if got := agent.count(); got != 2 {
		t.Errorf("upstream requests = %d, want 2 — no-cache must refresh the entry it bypassed", got)
	}

	// The directive is recognized inside a list, and an unrelated one is not
	// mistaken for it.
	assertFetchedCard(t, callFetchAgentCardAs(t, env, agentProxyOrg, `{"agentProxyId":"weather-agent"}`,
		map[string]string{"Cache-Control": "max-age=0, no-cache"}))
	if got := agent.count(); got != 3 {
		t.Errorf("upstream requests = %d, want 3 — no-cache in a directive list must be honoured", got)
	}
	assertFetchedCard(t, callFetchAgentCardAs(t, env, agentProxyOrg, `{"agentProxyId":"weather-agent"}`,
		map[string]string{"Cache-Control": "no-transform"}))
	if got := agent.count(); got != 3 {
		t.Errorf("upstream requests = %d, want 3 — an unrelated directive must not bypass the cache", got)
	}
}

// An explicit refresh that fails must not leave the card it superseded readable.
//
// With negative caching off the failure itself is not retained, so nothing
// overwrites the cached success — and without an explicit supersession the next
// ordinary request would be handed the very card the refresh had just
// established the upstream no longer serves.
func TestFetchAgentCard_ARefreshThatFailsDropsTheCardItSuperseded(t *testing.T) {
	// Successes cached, failures not — the configuration that makes the
	// supersession observable rather than masked by a cached 503.
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 0))

	var healthy atomic.Bool
	healthy.Store(true)
	agent := newUpstreamAgent(t, func(w http.ResponseWriter, _ *http.Request) {
		if !healthy.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(upstreamAgentCard))
	})
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), false)

	assertFetchedCard(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`))

	healthy.Store(false)
	assertAgentProxyError(t,
		callFetchAgentCardAs(t, env, agentProxyOrg, `{"agentProxyId":"weather-agent"}`,
			map[string]string{"Cache-Control": "no-cache"}),
		http.StatusServiceUnavailable, "AGENT_PROXY_UPSTREAM_UNREACHABLE")

	rec := callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`)
	if rec.Code == http.StatusOK {
		t.Fatalf("an ordinary request was served the card a failed refresh had superseded: %s", rec.Body.String())
	}
	assertAgentProxyError(t, rec, http.StatusServiceUnavailable, "AGENT_PROXY_UPSTREAM_UNREACHABLE")

	// And it re-contacted the upstream to find that out, rather than answering
	// from an entry that should no longer have been there.
	if got := agent.count(); got != 3 {
		t.Errorf("upstream requests = %d, want 3", got)
	}
}

// Zero TTLs disable caching entirely, and the response says so with max-age=0.
func TestFetchAgentCard_ZeroTTLDisablesCaching(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(0, 0))
	agent := newCardServingAgent(t)
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), false)

	rec := callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`)
	assertFetchedCard(t, rec)
	if got := rec.Header().Get("Cache-Control"); got != "max-age=0" {
		t.Errorf("Cache-Control = %q, want %q", got, "max-age=0")
	}

	assertFetchedCard(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`))
	if got := agent.count(); got != 2 {
		t.Errorf("upstream requests = %d, want 2 — a zero TTL must disable caching", got)
	}
}

// The upstream URL, its credentials or the card mode may have changed, so a
// cached result describes an Agent proxy that no longer exists in that shape.
func TestFetchAgentCard_UpdateAndDeleteInvalidateTheCachedEntry(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newCardServingAgent(t)
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), false)

	assertFetchedCard(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`))
	if got := agent.count(); got != 1 {
		t.Fatalf("upstream requests = %d, want 1", got)
	}

	// A replace that moves the upstream drops the entry immediately rather than
	// leaving a stale card readable for a full TTL.
	moved := newCardServingAgent(t)
	updateBody := fmt.Sprintf(`{
	  "id": "weather-agent",
	  "displayName": "Weather Agent",
	  "version": "v1.0",
	  "projectId": %q,
	  "context": "/weather",
	  "upstream": { "main": { "url": %q } },
	  "protocol": "a2a",
	  "a2a": { "protocolVersion": "1.0", "transports": [ { "protocolBinding": "JSONRPC", "pathPrefix": "/rpc" } ] }
	}`, agentProxyProject, moved.url())
	if rec := callAgentProxy(t, env.handler, http.MethodPut, agentProxyBase+"/weather-agent", updateBody); rec.Code != http.StatusOK {
		t.Fatalf("update Agent proxy: status = %d; body: %s", rec.Code, rec.Body.String())
	}

	assertFetchedCard(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`))
	if got := moved.count(); got != 1 {
		t.Errorf("new upstream requests = %d, want 1 — the update must have dropped the cached entry", got)
	}
	if got := agent.count(); got != 1 {
		t.Errorf("old upstream requests = %d, want 1 — the fetch must not have gone to the old address", got)
	}

	// A delete drops it too, so a handle recreated under the same name cannot
	// inherit the previous resource's cached card.
	if rec := callAgentProxy(t, env.handler, http.MethodDelete, agentProxyBase+"/weather-agent", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete Agent proxy: status = %d; body: %s", rec.Code, rec.Body.String())
	}
	assertAgentProxyError(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`),
		http.StatusNotFound, "AGENT_PROXY_NOT_FOUND")
}

// Two organizations may point at the same upstream with different credentials,
// so the cache is keyed on the Agent proxy within its organization and never on
// the URL — a URL-keyed entry would serve one tenant's card to another.
func TestFetchAgentCard_CacheIsolatesOrganizationsSharingAnUpstream(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))

	// One address, two tenants, and a body that says which credential was used.
	agent := newUpstreamAgent(t, func(w http.ResponseWriter, r *http.Request) {
		card := strings.Replace(upstreamAgentCard, `"name":"Weather Agent"`,
			`"name":"`+r.Header.Get("X-API-Key")+`"`, 1)
		_, _ = w.Write([]byte(card))
	})

	for _, org := range []string{agentProxyOrg, agentProxyOtherOrg} {
		seedUpstreamSecret(t, env, org)
		createAgentProxyPointingAt(t, env, org, "weather-agent", agent.url(), true)
	}
	// Distinct stored credentials, so a cross-tenant hit would be visible in the
	// body rather than merely suspected.
	if _, err := env.db.Exec(`UPDATE secrets SET ciphertext = ? WHERE organization_uuid = ?`,
		mustEncrypt(t, env, "other-org-key"), agentProxyOtherOrg); err != nil {
		t.Fatalf("rewrite the other organization's secret: %v", err)
	}

	first := callFetchAgentCardAs(t, env, agentProxyOrg, `{"agentProxyId":"weather-agent"}`, nil)
	if first.Code != http.StatusOK {
		t.Fatalf("first org fetch: status = %d; body: %s", first.Code, first.Body.String())
	}
	if !strings.Contains(first.Body.String(), upstreamSecretVal) {
		t.Fatalf("first org fetch did not use its own credential: %s", first.Body.String())
	}

	second := callFetchAgentCardAs(t, env, agentProxyOtherOrg, `{"agentProxyId":"weather-agent"}`, nil)
	if second.Code != http.StatusOK {
		t.Fatalf("second org fetch: status = %d; body: %s", second.Code, second.Body.String())
	}
	if strings.Contains(second.Body.String(), upstreamSecretVal) {
		t.Fatalf("the second organization was served the first organization's cached card: %s", second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "other-org-key") {
		t.Fatalf("second org fetch did not use its own credential: %s", second.Body.String())
	}
	if got := agent.count(); got != 2 {
		t.Errorf("upstream requests = %d, want 2 — each organization fetches its own entry", got)
	}
}

// A cache hit is never a way past a check a miss would have had to pass: the
// handle is resolved inside the caller's own organization before the cache is
// consulted at all.
func TestFetchAgentCard_AuthorizationPrecedesTheCacheLookup(t *testing.T) {
	env := newAgentProxyTestEnv(t, cardCacheConfig(time.Minute, 10*time.Second))
	agent := newCardServingAgent(t)
	createAgentProxyPointingAt(t, env, agentProxyOrg, "weather-agent", agent.url(), false)

	// Warm the entry for the owning organization.
	assertFetchedCard(t, callFetchAgentCard(t, env, `{"agentProxyId":"weather-agent"}`))

	// The same handle, from a tenant that does not own it, answers exactly as it
	// would on a cold cache.
	rec := callFetchAgentCardAs(t, env, agentProxyOtherOrg, `{"agentProxyId":"weather-agent"}`, nil)
	assertAgentProxyError(t, rec, http.StatusNotFound, "AGENT_PROXY_NOT_FOUND")
	if strings.Contains(rec.Body.String(), "Weather Agent") {
		t.Errorf("a cached card leaked across the organization check: %s", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Assertions against stored state
// ---------------------------------------------------------------------------

// storedAgentProxy is the persisted state a display fetch must leave alone.
type storedAgentProxy struct {
	configuration string
	updatedAt     string
	updatedBy     string
	dataVersion   sql.NullString
}

func agentProxyRow(t *testing.T, db *database.DB, handle string) storedAgentProxy {
	t.Helper()

	var row storedAgentProxy
	err := db.QueryRow(
		`SELECT configuration, updated_at, updated_by, data_version FROM agent_proxies WHERE handle = ?`, handle,
	).Scan(&row.configuration, &row.updatedAt, &row.updatedBy, &row.dataVersion)
	if err != nil {
		t.Fatalf("read stored Agent proxy %q: %v", handle, err)
	}
	return row
}

// assertNoDeploymentRows checks that a display fetch touched neither deployment
// state nor deployment status. The control plane's reachability of an upstream
// is not the gateway's, so it must never be mixed into either.
func assertNoDeploymentRows(t *testing.T, db *database.DB) {
	t.Helper()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM deployments`).Scan(&count); err != nil {
		t.Fatalf("count deployments: %v", err)
	}
	if count != 0 {
		t.Errorf("deployments = %d, want 0 — a display fetch must not create deployment state", count)
	}
}

func mustEncrypt(t *testing.T, env *agentProxyTestEnv, plaintext string) []byte {
	t.Helper()

	ciphertext, err := env.vault.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("encrypt %q: %v", plaintext, err)
	}
	return ciphertext
}

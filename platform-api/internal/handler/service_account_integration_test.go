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

package handler

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"

	_ "github.com/mattn/go-sqlite3"
)

const saTestBasePath = constants.APIBasePath + "/service-accounts"

type saTestEnv struct {
	handler http.Handler
	keyMap  *middleware.IssuerKeyMap
	repo    *repository.ServiceAccountRepo
	logger  *slog.Logger
}

// setupSATestEnv wires the real service, repository and revocation cache over
// a SQLite DB, in scope mode.
func setupSATestEnv(t *testing.T) *saTestEnv {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	db := &database.DB{DB: sqlDB}
	schema, err := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES ('org-it-001', 'test-org', 'Test Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`); err != nil {
		t.Fatal(err)
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Server{}
	cfg.Auth.ServiceAccount = config.ServiceAccount{TokenTTL: 15 * time.Minute, Audience: "platform-api"}
	cfg.Auth.Authorization.Mode = config.AuthzModeScope
	keys := &service.ServiceAccountKeys{Issuer: "platform-api", PrivateKey: key, Current: &key.PublicKey}
	keyMap, err := middleware.NewIssuerKeyMap("platform-api",
		middleware.IssuerKeys{Issuer: "platform-api", Kind: middleware.IssuerKindLocal, Current: &key.PublicKey})
	if err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	saRepo := repository.NewServiceAccountRepo(db)
	identity := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	roles := map[string][]string{"ap_sa_reader": {"ap:rest_api:read", "ap:gateway:read"}, "ap_sa_writer": {"ap:rest_api:manage"}}
	svc := service.NewServiceAccountService(saRepo, repository.NewOrganizationRepo(db), repository.NewAuditRepo(db), identity,
		roles, service.NewSATokenSigner(keys, cfg), 15*time.Minute, config.AuthzModeScope, config.ClaimMappings{}, logger)
	revocations := service.NewRevocationCache(saRepo, logger)
	if err := revocations.Load(); err != nil {
		t.Fatal(err)
	}
	svc.SetRevocationCache(revocations)

	h := NewServiceAccountHandler(svc, identity, keyMap, revocations, keys.PublicKeys(), logger)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return &saTestEnv{handler: middleware.NewTestContextMiddleware(mux), keyMap: keyMap, repo: saRepo, logger: logger}
}

// otherReplica is a fresh cache loaded from the DB: it sees a revoke only if
// the ledger row was written.
func (e *saTestEnv) otherReplica(t *testing.T) *service.RevocationCache {
	t.Helper()
	c := service.NewRevocationCache(e.repo, e.logger)
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	return c
}

// call sends a JSON request as an admin of org-it-001; withOrg=false drops the org.
func (e *saTestEnv) call(t *testing.T, method, path string, body any, withOrg bool) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	return e.raw(method, path, "application/json", buf.String(), withOrg)
}

func (e *saTestEnv) raw(method, path, contentType, body string, withOrg bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if withOrg {
		req.Header.Set("X-Test-Org", "org-it-001")
		req.Header.Set("X-Test-User", "alice")
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func (e *saTestEnv) token(form url.Values, basicID, basicSecret string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, saTestBasePath+"/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicID != "" {
		req.SetBasicAuth(url.QueryEscape(basicID), url.QueryEscape(basicSecret))
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

// mint exchanges form for a token and returns it with its account and version.
func (e *saTestEnv) mint(t *testing.T, form url.Values) (tok, accountUUID string, version int64) {
	t.Helper()
	rec := e.token(form, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("token: %d %s", rec.Code, rec.Body)
	}
	tok = decodeSAJSON[api.ServiceAccountTokenResponse](t, rec).AccessToken
	claims, err := e.keyMap.VerifyServiceAccountToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	sub, _ := claims["sub"].(string)
	accountUUID, _ = model.AccountUUIDFromSubject(sub)
	version, _ = middleware.ServiceAccountTokenVersion(claims)
	return tok, accountUUID, version
}

func (e *saTestEnv) introspectActive(t *testing.T, tok string) bool {
	t.Helper()
	rec := e.raw(http.MethodPost, saTestBasePath+"/introspect", "application/x-www-form-urlencoded",
		url.Values{"token": {tok}}.Encode(), true)
	return decodeSAJSON[api.IntrospectionResponse](t, rec).Active
}

func decodeSAJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func createSATestAccount(t *testing.T, e *saTestEnv, handle string) api.ServiceAccountCredentials {
	t.Helper()
	rec := e.call(t, http.MethodPost, saTestBasePath, api.ServiceAccountCreateRequest{Id: handle, DisplayName: "CI",
		Owner: saStr("team"), Description: saStr("deploys"), Roles: []string{"ap_sa_reader"}}, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	return decodeSAJSON[api.ServiceAccountCredentials](t, rec)
}

func saTestForm(creds api.ServiceAccountCredentials, scope string) url.Values {
	return url.Values{"grant_type": {"client_credentials"}, "client_id": {creds.ClientId},
		"client_secret": {creds.ClientSecret}, "scope": {scope}}
}

func TestServiceAccountHandler_Lifecycle(t *testing.T) {
	e := setupSATestEnv(t)
	create := api.ServiceAccountCreateRequest{Id: "ci-bot", DisplayName: "CI", Owner: saStr("team"), Description: saStr("deploys"),
		Roles: []string{"ap_sa_reader"}}

	rec := e.call(t, http.MethodPost, saTestBasePath, create, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	creds := decodeSAJSON[api.ServiceAccountCredentials](t, rec)
	if !strings.HasSuffix(rec.Header().Get("Location"), "/service-accounts/ci-bot") || creds.ClientSecret == "" {
		t.Fatalf("create response: location %q, %+v", rec.Header().Get("Location"), creds)
	}
	if rec := e.call(t, http.MethodPost, saTestBasePath, create, true); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate create: %d", rec.Code)
	}
	bad := create
	bad.Id, bad.Roles = "bad-bot", []string{"ap_admin"}
	if rec := e.call(t, http.MethodPost, saTestBasePath, bad, true); rec.Code != http.StatusBadRequest {
		t.Fatalf("non ap_sa_ role: %d", rec.Code)
	}

	rec = e.call(t, http.MethodGet, saTestBasePath+"/ci-bot", nil, true)
	if got := decodeSAJSON[api.ServiceAccount](t, rec); rec.Code != http.StatusOK || got.Id != "ci-bot" || *got.ClientId != creds.ClientId {
		t.Fatalf("get: %d %+v", rec.Code, got)
	}
	rec = e.call(t, http.MethodGet, saTestBasePath+"?limit=10&offset=0", nil, true)
	if list := decodeSAJSON[api.ServiceAccountListResponse](t, rec); rec.Code != http.StatusOK || list.Count != 1 ||
		list.Pagination.Total != 1 || list.List[0].Id != "ci-bot" {
		t.Fatalf("list: %d %+v", rec.Code, list)
	}

	form := saTestForm(creds, "ap:rest_api:read")
	rec = e.token(form, "", "")
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("token: %d %s", rec.Code, rec.Body)
	}
	claims, err := e.keyMap.VerifyServiceAccountToken(decodeSAJSON[api.ServiceAccountTokenResponse](t, rec).AccessToken)
	if err != nil || claims["scope"] != "ap:rest_api:read" || claims["azp"] != creds.ClientId {
		t.Fatalf("issued token: %v %v", claims, err)
	}
	rec = e.token(url.Values{"grant_type": {"client_credentials"}, "scope": {"ap:gateway:read"}}, creds.ClientId, creds.ClientSecret)
	if rec.Code != http.StatusOK {
		t.Fatalf("token with Basic: %d %s", rec.Code, rec.Body)
	}

	// Removing ap_sa_reader revokes tokens already issued, on every replica.
	tok, account, version := e.mint(t, form)
	newName, newRoles := "CI v2", []string{"ap_sa_writer"}
	rec = e.call(t, http.MethodPut, saTestBasePath+"/ci-bot", api.ServiceAccountUpdateRequest{DisplayName: &newName, Roles: &newRoles}, true)
	if got := decodeSAJSON[api.ServiceAccount](t, rec); rec.Code != http.StatusOK || got.DisplayName != "CI v2" || got.Roles[0] != "ap_sa_writer" {
		t.Fatalf("update: %d %+v", rec.Code, got)
	}
	if !e.otherReplica(t).IsRevoked(account, version) || e.introspectActive(t, tok) {
		t.Fatal("removing a role must revoke tokens already issued")
	}

	// Regenerate: the old secret stops, and so do tokens minted with it.
	form.Set("scope", "ap:rest_api:manage")
	tok, _, version = e.mint(t, form)
	rec = e.call(t, http.MethodPost, saTestBasePath+"/ci-bot/regenerate-secret", nil, true)
	regen := decodeSAJSON[api.ServiceAccountCredentials](t, rec)
	if rec.Code != http.StatusOK || regen.ClientSecret == creds.ClientSecret {
		t.Fatalf("regenerate: %d", rec.Code)
	}
	if !e.otherReplica(t).IsRevoked(account, version) || e.introspectActive(t, tok) {
		t.Fatal("regenerate must revoke tokens already issued")
	}
	if rec := e.token(form, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old secret after regenerate: %d", rec.Code)
	}
	form.Set("client_secret", regen.ClientSecret)
	tok, _, version = e.mint(t, form)
	if !e.introspectActive(t, tok) {
		t.Fatal("a token from the new secret must be active")
	}

	// Delete: the last token and the credentials both stop.
	if rec := e.call(t, http.MethodDelete, saTestBasePath+"/ci-bot", nil, true); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if !e.otherReplica(t).IsRevoked(account, version) || e.introspectActive(t, tok) {
		t.Fatal("delete must revoke tokens already issued")
	}
	if rec := e.token(form, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("token after delete: %d", rec.Code)
	}
	for _, probe := range []struct{ method, path string }{
		{http.MethodGet, saTestBasePath + "/ci-bot"},
		{http.MethodPut, saTestBasePath + "/ci-bot"},
		{http.MethodDelete, saTestBasePath + "/ci-bot"},
		{http.MethodPost, saTestBasePath + "/ci-bot/regenerate-secret"},
	} {
		if rec := e.call(t, probe.method, probe.path, map[string]string{}, true); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s after delete: %d, want 404", probe.method, probe.path, rec.Code)
		}
	}
}

// Disabling stops the exchange and live tokens; enabling restores the exchange.
func TestServiceAccountHandler_DisableAndEnable(t *testing.T) {
	e := setupSATestEnv(t)
	creds := createSATestAccount(t, e, "ci-bot")
	form := saTestForm(creds, "ap:rest_api:read")
	tok, account, version := e.mint(t, form)

	disabled, active := api.ServiceAccountUpdateRequestStatusDisabled, api.ServiceAccountUpdateRequestStatusActive
	if rec := e.call(t, http.MethodPut, saTestBasePath+"/ci-bot", api.ServiceAccountUpdateRequest{Status: &disabled}, true); rec.Code != http.StatusOK {
		t.Fatalf("disable: %d %s", rec.Code, rec.Body)
	}
	if rec := e.token(form, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("token while disabled: %d, want 401", rec.Code)
	}
	if !e.otherReplica(t).IsRevoked(account, version) || e.introspectActive(t, tok) {
		t.Fatal("disable must revoke tokens already issued")
	}

	if rec := e.call(t, http.MethodPut, saTestBasePath+"/ci-bot", api.ServiceAccountUpdateRequest{Status: &active}, true); rec.Code != http.StatusOK {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body)
	}
	if tok, _, _ := e.mint(t, form); !e.introspectActive(t, tok) {
		t.Fatal("a token minted after re-enabling must be active")
	}
}

func TestServiceAccountHandler_TokenRejects(t *testing.T) {
	e := setupSATestEnv(t)
	creds := createSATestAccount(t, e, "ci-bot")

	cases := map[string]struct {
		mutate func(url.Values)
		want   int
	}{
		"wrong grant type": {func(v url.Values) { v.Set("grant_type", "password") }, http.StatusBadRequest},
		"wrong secret":     {func(v url.Values) { v.Set("client_secret", "apsa_wrong") }, http.StatusUnauthorized},
		"unknown client":   {func(v url.Values) { v.Set("client_id", "sa_nobody") }, http.StatusUnauthorized},
		"no scope":         {func(v url.Values) { v.Del("scope") }, http.StatusBadRequest},
		"ungranted scope":  {func(v url.Values) { v.Set("scope", "ap:rest_api:manage") }, http.StatusBadRequest},
		"no client id":     {func(v url.Values) { v.Del("client_id") }, http.StatusUnauthorized},
		"no client secret": {func(v url.Values) { v.Del("client_secret") }, http.StatusUnauthorized},
	}
	for name, c := range cases {
		form := saTestForm(creds, "ap:rest_api:read")
		c.mutate(form)
		if rec := e.token(form, "", ""); rec.Code != c.want {
			t.Errorf("%s: %d, want %d (%s)", name, rec.Code, c.want, rec.Body)
		}
	}

	if rec := e.token(saTestForm(creds, "ap:rest_api:read"), creds.ClientId, creds.ClientSecret); rec.Code != http.StatusBadRequest {
		t.Errorf("Basic and form together: %d, want 400", rec.Code)
	}
	// The valid pairs still parse, so without the parse check this would be a 401.
	rec := e.raw(http.MethodPost, saTestBasePath+"/token", "application/x-www-form-urlencoded",
		"grant_type=client_credentials&client_id=x&%zz", false)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "form-encoded") {
		t.Errorf("malformed form: %d %s", rec.Code, rec.Body)
	}
}

// Owner and description may be omitted; owner then reads back as the creator's
// sub, through the real identity mapping.
func TestServiceAccountHandler_OwnerDefaultsToCreator(t *testing.T) {
	e := setupSATestEnv(t)
	rec := e.call(t, http.MethodPost, saTestBasePath, map[string]any{"id": "ci-bot", "displayName": "CI",
		"roles": []string{"ap_sa_reader"}}, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if got := decodeSAJSON[api.ServiceAccountCredentials](t, rec).ServiceAccount; got.Owner != "alice" || got.Description != "" {
		t.Fatalf("owner %q, description %q", got.Owner, got.Description)
	}

	rec = e.call(t, http.MethodPut, saTestBasePath+"/ci-bot", map[string]string{"owner": "platform-team"}, true)
	if got := decodeSAJSON[api.ServiceAccount](t, rec); got.Owner != "platform-team" {
		t.Fatalf("set owner: %d %+v", rec.Code, got)
	}
	rec = e.call(t, http.MethodPut, saTestBasePath+"/ci-bot", map[string]string{"owner": " "}, true)
	if got := decodeSAJSON[api.ServiceAccount](t, rec); got.Owner != "alice" {
		t.Fatalf("blank owner must reset to the creator: %d %+v", rec.Code, got)
	}
}

// Every management route needs the organization from the token.
func TestServiceAccountHandler_RequiresOrganization(t *testing.T) {
	e := setupSATestEnv(t)
	for _, probe := range []struct{ method, path string }{
		{http.MethodGet, saTestBasePath},
		{http.MethodPost, saTestBasePath},
		{http.MethodGet, saTestBasePath + "/ci-bot"},
		{http.MethodPut, saTestBasePath + "/ci-bot"},
		{http.MethodDelete, saTestBasePath + "/ci-bot"},
		{http.MethodPost, saTestBasePath + "/ci-bot/regenerate-secret"},
	} {
		if rec := e.call(t, probe.method, probe.path, map[string]string{}, false); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s with no org: %d, want 401", probe.method, probe.path, rec.Code)
		}
	}
}

// A decode failure is "Invalid input.", not a later field check.
func TestServiceAccountHandler_BadJSON(t *testing.T) {
	e := setupSATestEnv(t)
	createSATestAccount(t, e, "ci-bot")
	for method, path := range map[string]string{http.MethodPost: saTestBasePath, http.MethodPut: saTestBasePath + "/ci-bot"} {
		rec := e.raw(method, path, "application/json", "{not json", true)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Invalid input.") {
			t.Errorf("%s with bad JSON: %d %s", method, rec.Code, rec.Body)
		}
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:51234"
	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("with port: %q", got)
	}
	r.RemoteAddr = "203.0.113.7"
	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("without port: %q", got)
	}
}

func saStr(s string) *string { return &s }

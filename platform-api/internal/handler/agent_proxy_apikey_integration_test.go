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

// End-to-end Agent proxy API keys over the real route -> handler -> service ->
// repository stack, backed by SQLite. Create, update and revoke go through the
// shared APIKeyService and so behave as REST API keys do; these tests pin that
// behavior for the Agent kind, plus the Agent-only listing, the tenant and
// ownership boundaries, the gateway events, and the gateway-internal backfill.

package handler

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
)

const (
	agentKeyOtherUser = "sub-agent-other-user"
	agentKeyAdminUser = "sub-agent-key-admin"
)

var generatedAPIKeyPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func agentKeysPath(proxy string) string {
	return agentProxyBase + "/" + proxy + "/api-keys"
}

func agentKeyPath(proxy, key string) string {
	return agentKeysPath(proxy) + "/" + key
}

// createAgentProxyForKeys creates a minimal Agent proxy with the given handle.
func createAgentProxyForKeys(t *testing.T, h http.Handler, handle string) {
	t.Helper()
	rec := callAgentProxy(t, h, http.MethodPost, agentProxyBase, minimalAgentProxyBody(handle, "Agent "+handle))
	decodeAgentProxyJSON(t, rec, http.StatusCreated)
}

// callAsKeyAdmin issues a request as a user holding ap:api_key:all:manage.
func callAsKeyAdmin(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Org", agentProxyOrg)
	req.Header.Set("X-Test-User", agentKeyAdminUser)
	req.Header.Set("X-Test-Scope", constants.ScopeAPIKeyAllManage)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// createAgentKey creates a key and returns the decoded creation response.
func createAgentKey(t *testing.T, h http.Handler, proxy, body string) map[string]any {
	t.Helper()
	rec := callAgentProxy(t, h, http.MethodPost, agentKeysPath(proxy), body)
	return decodeAgentProxyJSON(t, rec, http.StatusCreated)
}

// deployedAgentKeysEnv is an Agent proxy deployed to one gateway, which update
// and revoke require: like REST API keys, they refuse with 503 when the
// artifact has no gateway to broadcast to.
func deployedAgentKeysEnv(t *testing.T) *agentDeployEnv {
	t.Helper()
	env := setupAgentDeployEnv(t)
	env.deploy(t, env.gateway)
	return env
}

func updateKeyBody(apiKey, displayName string) string {
	return fmt.Sprintf(`{"displayName": %q, "apiKey": %q}`, displayName, apiKey)
}

type storedAgentKey struct {
	UUID, ArtifactUUID, DisplayName, Masked, Hashes, Status, CreatedBy string
	ExpiresAt                                                          sql.NullTime
	Issuer                                                             sql.NullString
}

func readAgentKey(t *testing.T, env *agentProxyTestEnv, proxy, key string) storedAgentKey {
	t.Helper()
	var k storedAgentKey
	var hashes []byte
	err := env.db.QueryRow(`SELECT k.uuid, k.artifact_uuid, k.display_name, k.masked_api_key, k.api_key_hashes,
			k.status, k.created_by, k.expires_at, k.issuer
		FROM api_keys k JOIN agent_proxies a ON a.uuid = k.artifact_uuid
		WHERE a.handle = ? AND a.organization_uuid = ? AND k.handle = ?`, proxy, agentProxyOrg, key).
		Scan(&k.UUID, &k.ArtifactUUID, &k.DisplayName, &k.Masked, &hashes, &k.Status, &k.CreatedBy, &k.ExpiresAt, &k.Issuer)
	if err != nil {
		t.Fatalf("read stored key %s/%s: %v", proxy, key, err)
	}
	k.Hashes = string(hashes)
	return k
}

func agentUUIDOf(t *testing.T, env *agentProxyTestEnv, handle string) string {
	t.Helper()
	var id string
	if err := env.db.QueryRow(`SELECT uuid FROM agent_proxies WHERE handle = ? AND organization_uuid = ?`,
		handle, agentProxyOrg).Scan(&id); err != nil {
		t.Fatalf("resolve agent proxy uuid: %v", err)
	}
	return id
}

func countAgentKeys(t *testing.T, env *agentProxyTestEnv) int {
	t.Helper()
	var n int
	if err := env.db.QueryRow(`SELECT COUNT(*) FROM api_keys`).Scan(&n); err != nil {
		t.Fatalf("count keys: %v", err)
	}
	return n
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// assertNoKeyMaterial fails if body carries the plaintext key, any stored hash,
// or a property that could hold key material.
func assertNoKeyMaterial(t *testing.T, body []byte, plaintext string, hashes ...string) {
	t.Helper()
	s := string(body)
	if plaintext != "" && strings.Contains(s, plaintext) {
		t.Fatalf("response carries the plaintext key: %s", s)
	}
	for _, h := range hashes {
		if h != "" && strings.Contains(s, h) {
			t.Fatalf("response carries a key hash: %s", s)
		}
	}
	for _, field := range []string{`"apiKey"`, `"apiKeyHashes"`, `"api_key_hashes"`} {
		if strings.Contains(s, field) {
			t.Fatalf("response carries %s: %s", field, s)
		}
	}
}

// --- create ------------------------------------------------------------------

func TestAgentProxyAPIKey_CreateReturns201LocationAndOneTimeKey(t *testing.T) {
	env := newAgentProxyTestEnv(t, agentDeployConfig())
	createAgentProxyForKeys(t, env.handler, "weather-agent")

	rec := callAgentProxy(t, env.handler, http.MethodPost, agentKeysPath("weather-agent"),
		`{"displayName": "Consumer Key"}`)
	body := decodeAgentProxyJSON(t, rec, http.StatusCreated)

	keyID, _ := body["keyId"].(string)
	if keyID != "consumer-key" || body["status"] != "success" {
		t.Fatalf("response = %v, want status success and an id derived from the display name", body)
	}
	if got, want := rec.Header().Get("Location"), agentKeyPath("weather-agent", keyID); got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
	plaintext, _ := body["apiKey"].(string)
	if !generatedAPIKeyPattern.MatchString(plaintext) {
		t.Fatalf("apiKey = %q, want the generated 64-hex key returned once", plaintext)
	}

	stored := readAgentKey(t, env, "weather-agent", keyID)
	if stored.ArtifactUUID != agentUUIDOf(t, env, "weather-agent") || stored.ArtifactUUID == "weather-agent" {
		t.Fatalf("key associated with %q, want the Agent proxy's internal UUID", stored.ArtifactUUID)
	}
	if strings.Contains(stored.Hashes, plaintext) || !strings.Contains(stored.Hashes, sha256Hex(plaintext)) {
		t.Fatalf("stored hashes = %s, want the sha256 of the key and never the key itself", stored.Hashes)
	}
	if stored.Masked == plaintext || !strings.HasSuffix(plaintext, strings.TrimPrefix(stored.Masked, "***")) {
		t.Fatalf("masked key = %q does not mask %q", stored.Masked, plaintext)
	}
	if stored.Status != constants.APIKeyStatusActive || stored.DisplayName != "Consumer Key" {
		t.Fatalf("stored key = %+v", stored)
	}
}

// An injected key is registered as-is and is not echoed back: the caller
// already has it.
func TestAgentProxyAPIKey_CreateWithInjectedKeyDoesNotEchoIt(t *testing.T) {
	env := newAgentProxyTestEnv(t, agentDeployConfig())
	createAgentProxyForKeys(t, env.handler, "weather-agent")

	const injected = "external-platform-minted-key-0123456789"
	rec := callAgentProxy(t, env.handler, http.MethodPost, agentKeysPath("weather-agent"),
		fmt.Sprintf(`{"id": "portal-key", "displayName": "Portal Key", "apiKey": %q, "issuer": " api-platform-devportal "}`, injected))
	body := decodeAgentProxyJSON(t, rec, http.StatusCreated)
	if _, present := body["apiKey"]; present {
		t.Fatalf("creation response echoes an injected key: %v", body)
	}
	stored := readAgentKey(t, env, "weather-agent", "portal-key")
	if !strings.Contains(stored.Hashes, sha256Hex(injected)) {
		t.Fatalf("stored hashes %s are not the hash of the injected key", stored.Hashes)
	}
	if !stored.Issuer.Valid || stored.Issuer.String != "api-platform-devportal" {
		t.Fatalf("issuer = %+v, want the trimmed issuer", stored.Issuer)
	}
}

// As with REST API keys, a taken id — supplied or derived — is suffixed rather
// than refused. Location and keyId name the id actually stored.
func TestAgentProxyAPIKey_CreateSuffixesATakenID(t *testing.T) {
	env := newAgentProxyTestEnv(t, agentDeployConfig())
	createAgentProxyForKeys(t, env.handler, "weather-agent")
	createAgentKey(t, env.handler, "weather-agent", `{"id": "consumer-key", "displayName": "Consumer Key"}`)

	for name, body := range map[string]string{
		"supplied id": `{"id": "consumer-key", "displayName": "Another"}`,
		"derived id":  `{"displayName": "Consumer Key"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := callAgentProxy(t, env.handler, http.MethodPost, agentKeysPath("weather-agent"), body)
			created := decodeAgentProxyJSON(t, rec, http.StatusCreated)
			stored, _ := created["keyId"].(string)
			if !strings.HasPrefix(stored, "consumer-key-") {
				t.Fatalf("keyId = %q, want consumer-key with a collision suffix", stored)
			}
			if got, want := rec.Header().Get("Location"), agentKeyPath("weather-agent", stored); got != want {
				t.Fatalf("Location = %q, want %q — the stored id, not the requested one", got, want)
			}
			readAgentKey(t, env, "weather-agent", stored) // the Location resolves to a stored key
		})
	}

	// Key ids are per Agent proxy: the same id on another Agent proxy is free.
	createAgentProxyForKeys(t, env.handler, "travel-agent")
	if created := createAgentKey(t, env.handler, "travel-agent", `{"id": "consumer-key", "displayName": "Consumer Key"}`); created["keyId"] != "consumer-key" {
		t.Fatalf("keyId on another Agent proxy = %v, want consumer-key unsuffixed", created["keyId"])
	}
}

// Creation needs no gateway: the key is stored and delivered at deploy time.
func TestAgentProxyAPIKey_CreateOnAnUndeployedAgentSucceeds(t *testing.T) {
	env := setupAgentDeployEnv(t)
	createAgentKey(t, env.handler, env.proxy, `{"id": "early-key", "displayName": "Early Key"}`)
	if got := len(env.recordedEvents(t, env.gatewayUUID)); got != 0 {
		t.Fatalf("an undeployed Agent's key was broadcast (%d events)", got)
	}
}

func TestAgentProxyAPIKey_CreateRejectionContract(t *testing.T) {
	env := newAgentProxyTestEnv(t, agentDeployConfig())
	createAgentProxyForKeys(t, env.handler, "weather-agent")
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)

	for name, body := range map[string]string{
		"missing displayName": `{"id": "k-one"}`,
		"expiry in the past":  fmt.Sprintf(`{"displayName": "K", "expiresAt": %q}`, past),
		"unknown expiry unit": `{"displayName": "K", "expiresIn": {"duration": 1, "unit": "fortnights"}}`,
		"blank injected key":  `{"displayName": "K", "apiKey": "  "}`,
		"overlong issuer":     fmt.Sprintf(`{"displayName": "K", "issuer": %q}`, strings.Repeat("i", 256)),
		"malformed JSON":      `{"displayName": `,
	} {
		t.Run(name, func(t *testing.T) {
			rec := callAgentProxy(t, env.handler, http.MethodPost, agentKeysPath("weather-agent"), body)
			assertAgentProxyError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
		})
	}

	t.Run("empty body", func(t *testing.T) {
		rec := callAgentProxy(t, env.handler, http.MethodPost, agentKeysPath("weather-agent"), "")
		assertAgentProxyError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
	})
	t.Run("unsupported media type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, agentKeysPath("weather-agent"), strings.NewReader(`displayName=K`))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Test-Org", agentProxyOrg)
		req.Header.Set("X-Test-User", agentProxyActor)
		rec := httptest.NewRecorder()
		env.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("status = %d, want 415: %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("oversized body", func(t *testing.T) {
		body := fmt.Sprintf(`{"displayName": "K", "externalRefId": %q}`, strings.Repeat("x", 32<<10))
		rec := callAgentProxy(t, env.handler, http.MethodPost, agentKeysPath("weather-agent"), body)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413: %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("unknown agent proxy", func(t *testing.T) {
		rec := callAgentProxy(t, env.handler, http.MethodPost, agentKeysPath("no-such-agent"), `{"displayName": "K"}`)
		assertAgentProxyError(t, rec, http.StatusNotFound, apperror.CodeAgentProxyNotFound)
	})

	if n := countAgentKeys(t, env); n != 0 {
		t.Fatalf("%d keys stored by rejected requests, want none", n)
	}
}

// --- list --------------------------------------------------------------------

func TestAgentProxyAPIKey_ListReturnsRedactedMetadataInTheListEnvelope(t *testing.T) {
	env := deployedAgentKeysEnv(t)
	created := createAgentKey(t, env.handler, env.proxy, `{"id": "key-one", "displayName": "Key One"}`)
	createAgentKey(t, env.handler, env.proxy, `{"id": "key-two", "displayName": "Key Two"}`)
	createAgentKey(t, env.handler, env.proxy, `{"id": "key-three", "displayName": "Key Three"}`)
	plaintext := created["apiKey"].(string)
	stored := readAgentKey(t, env.agentProxyTestEnv, env.proxy, "key-one")

	rec := callAgentProxy(t, env.handler, http.MethodGet, agentKeysPath(env.proxy), "")
	body := decodeAgentProxyJSON(t, rec, http.StatusOK)
	assertNoKeyMaterial(t, rec.Body.Bytes(), plaintext, sha256Hex(plaintext))

	list, _ := body["list"].([]any)
	if len(list) != 3 || body["count"] != float64(3) {
		t.Fatalf("list = %v, count = %v, want three keys", list, body["count"])
	}
	if pagination, _ := body["pagination"].(map[string]any); pagination["total"] != float64(3) {
		t.Fatalf("pagination = %v, want total 3", body["pagination"])
	}
	var one map[string]any
	for _, item := range list {
		if m := item.(map[string]any); m["id"] == "key-one" {
			one = m
		}
	}
	if one == nil {
		t.Fatalf("key-one missing from %v", list)
	}
	if one["displayName"] != "Key One" || one["status"] != "active" || one["maskedApiKey"] != stored.Masked ||
		one["allowedTargets"] != constants.APIKeyAllowedTargetsAll {
		t.Fatalf("key-one metadata = %v", one)
	}
	if one["createdBy"] != agentProxyActor {
		t.Fatalf("createdBy = %v, want the external identity %q, not the stored UUID", one["createdBy"], agentProxyActor)
	}

	// Pagination windows the filtered list; the total stays the full count.
	body = decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet, agentKeysPath(env.proxy)+"?limit=2&offset=2", ""), http.StatusOK)
	if list, _ := body["list"].([]any); len(list) != 1 || body["count"] != float64(1) {
		t.Fatalf("page = %v, want the one remaining key", body)
	}
	if p, _ := body["pagination"].(map[string]any); p["total"] != float64(3) || p["offset"] != float64(2) || p["limit"] != float64(2) {
		t.Fatalf("pagination = %v", body["pagination"])
	}

	// A revoked key is still listed, with its status.
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodDelete, agentKeyPath(env.proxy, "key-two"), ""), http.StatusNoContent)
	body = decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet, agentKeysPath(env.proxy), ""), http.StatusOK)
	for _, item := range body["list"].([]any) {
		if m := item.(map[string]any); m["id"] == "key-two" && m["status"] != "revoked" {
			t.Fatalf("revoked key listed as %v", m["status"])
		}
	}
}

func TestAgentProxyAPIKey_ListIsEmptyNotMissingForAnAgentWithoutKeys(t *testing.T) {
	env := newAgentProxyTestEnv(t, agentDeployConfig())
	createAgentProxyForKeys(t, env.handler, "weather-agent")

	body := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet, agentKeysPath("weather-agent"), ""), http.StatusOK)
	if list, ok := body["list"].([]any); !ok || len(list) != 0 {
		t.Fatalf("list = %#v, want an empty array", body["list"])
	}

	rec := callAgentProxy(t, env.handler, http.MethodGet, agentKeysPath("no-such-agent"), "")
	assertAgentProxyError(t, rec, http.StatusNotFound, apperror.CodeAgentProxyNotFound)
}

// Agent keys appear in the caller's cross-artifact key listing alongside the
// other kinds, identified by the Agent proxy's public handle and the AgentProxy
// kind — never the internal UUID or the gateway's Agent kind — and the type
// filter selects or excludes them.
func TestAgentProxyAPIKey_ListedInTheUserKeyListing(t *testing.T) {
	env := newAgentProxyTestEnv(t, agentDeployConfig())
	createAgentProxyForKeys(t, env.handler, "weather-agent")
	created := createAgentKey(t, env.handler, "weather-agent", `{"id": "consumer-key", "displayName": "Consumer Key"}`)
	plaintext := created["apiKey"].(string)
	const userKeys = constants.APIBasePath + "/me/api-keys"

	findAgentKey := func(body map[string]any) map[string]any {
		for _, item := range body["list"].([]any) {
			if m := item.(map[string]any); m["id"] == "consumer-key" {
				return m
			}
		}
		return nil
	}

	for _, path := range []string{userKeys, userKeys + "?type=AgentProxy", userKeys + "?type=RestApi,AgentProxy"} {
		rec := callAgentProxy(t, env.handler, http.MethodGet, path, "")
		body := decodeAgentProxyJSON(t, rec, http.StatusOK)
		assertNoKeyMaterial(t, rec.Body.Bytes(), plaintext, sha256Hex(plaintext))
		key := findAgentKey(body)
		if key == nil {
			t.Fatalf("GET %s does not list the Agent key: %v", path, body["list"])
		}
		if key["artifactType"] != constants.AgentProxy || key["artifactId"] != "weather-agent" {
			t.Fatalf("GET %s: artifact = %v/%v, want AgentProxy/weather-agent", path, key["artifactType"], key["artifactId"])
		}
	}

	body := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet, userKeys+"?type=LlmProxy", ""), http.StatusOK)
	if findAgentKey(body) != nil {
		t.Fatalf("?type=LlmProxy lists the Agent key: %v", body["list"])
	}

	// Creator-scoped, as for every kind: another user does not see it; a key admin does.
	body = decodeAgentProxyJSON(t, callAgentProxyAs(t, env.handler, agentProxyOrg, agentKeyOtherUser, http.MethodGet, userKeys, ""), http.StatusOK)
	if findAgentKey(body) != nil {
		t.Fatalf("another user's listing includes the Agent key: %v", body["list"])
	}
	body = decodeAgentProxyJSON(t, callAsKeyAdmin(t, env.handler, http.MethodGet, userKeys, ""), http.StatusOK)
	if findAgentKey(body) == nil {
		t.Fatalf("key admin listing is missing the Agent key: %v", body["list"])
	}

	// Another organization never sees it.
	body = decodeAgentProxyJSON(t, callAgentProxyAs(t, env.handler, agentProxyOtherOrg, agentProxyActor, http.MethodGet, userKeys, ""), http.StatusOK)
	if findAgentKey(body) != nil {
		t.Fatalf("another organization lists the Agent key: %v", body["list"])
	}
}

// --- update ------------------------------------------------------------------

// As with REST API keys, PUT replaces the key material with the supplied value.
// Repeating the same PUT is idempotent: the same value hashes to the same digest.
func TestAgentProxyAPIKey_UpdateRotatesToTheSuppliedKey(t *testing.T) {
	env := deployedAgentKeysEnv(t)
	created := createAgentKey(t, env.handler, env.proxy, `{"id": "consumer-key", "displayName": "Consumer Key"}`)
	original := created["apiKey"].(string)
	before := readAgentKey(t, env.agentProxyTestEnv, env.proxy, "consumer-key")

	const rotated = "rotated-key-value-0123456789abcdefghij"
	for attempt := 1; attempt <= 2; attempt++ {
		rec := callAgentProxy(t, env.handler, http.MethodPut, agentKeyPath(env.proxy, "consumer-key"), updateKeyBody(rotated, "Consumer Key"))
		body := decodeAgentProxyJSON(t, rec, http.StatusOK)
		if body["status"] != "success" || body["keyId"] != "consumer-key" {
			t.Fatalf("attempt %d: response = %v", attempt, body)
		}
		assertNoKeyMaterial(t, rec.Body.Bytes(), rotated, sha256Hex(rotated), sha256Hex(original))

		after := readAgentKey(t, env.agentProxyTestEnv, env.proxy, "consumer-key")
		if !strings.Contains(after.Hashes, sha256Hex(rotated)) || strings.Contains(after.Hashes, sha256Hex(original)) {
			t.Fatalf("attempt %d: stored hashes = %s, want the hash of the rotated key only", attempt, after.Hashes)
		}
		if after.UUID != before.UUID || after.Status != constants.APIKeyStatusActive {
			t.Fatalf("attempt %d: key identity or status changed: %+v", attempt, after)
		}
	}
}

func TestAgentProxyAPIKey_UpdateRejectionContract(t *testing.T) {
	env := deployedAgentKeysEnv(t)
	createAgentKey(t, env.handler, env.proxy, `{"id": "consumer-key", "displayName": "Consumer Key"}`)
	before := readAgentKey(t, env.agentProxyTestEnv, env.proxy, "consumer-key")
	path := agentKeyPath(env.proxy, "consumer-key")

	for name, body := range map[string]string{
		"missing key value":  `{"displayName": "K"}`,
		"conflicting name":   `{"name": "renamed-key", "displayName": "K", "apiKey": "new-value-0123456789"}`,
		"new expiry in past": fmt.Sprintf(`{"displayName": "K", "apiKey": "new-value-0123456789", "expiresAt": %q}`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)),
		"malformed JSON":     `{"displayName": `,
	} {
		t.Run(name, func(t *testing.T) {
			rec := callAgentProxy(t, env.handler, http.MethodPut, path, body)
			assertAgentProxyError(t, rec, http.StatusBadRequest, apperror.CodeCommonValidationFailed)
		})
	}

	t.Run("unknown key", func(t *testing.T) {
		rec := callAgentProxy(t, env.handler, http.MethodPut, agentKeyPath(env.proxy, "no-such-key"), updateKeyBody("new-value-0123456789", "K"))
		assertAgentProxyError(t, rec, http.StatusNotFound, apperror.CodeRESTAPIAPIKeyNotFound)
	})
	t.Run("unknown agent proxy", func(t *testing.T) {
		rec := callAgentProxy(t, env.handler, http.MethodPut, agentKeyPath("no-such-agent", "consumer-key"), updateKeyBody("new-value-0123456789", "K"))
		assertAgentProxyError(t, rec, http.StatusNotFound, apperror.CodeAgentProxyNotFound)
	})

	if after := readAgentKey(t, env.agentProxyTestEnv, env.proxy, "consumer-key"); after != before {
		t.Fatalf("a rejected update changed the key: before %+v, after %+v", before, after)
	}
}

// --- revoke ------------------------------------------------------------------

func TestAgentProxyAPIKey_RevokeReturns204(t *testing.T) {
	env := deployedAgentKeysEnv(t)
	createAgentKey(t, env.handler, env.proxy, `{"id": "consumer-key", "displayName": "Consumer Key"}`)

	rec := callAgentProxy(t, env.handler, http.MethodDelete, agentKeyPath(env.proxy, "consumer-key"), "")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status = %d, body %q; want a bodyless 204", rec.Code, rec.Body.String())
	}
	if got := readAgentKey(t, env.agentProxyTestEnv, env.proxy, "consumer-key"); got.Status != constants.APIKeyStatusRevoked {
		t.Fatalf("status = %q, want revoked", got.Status)
	}

	rec = callAgentProxy(t, env.handler, http.MethodDelete, agentKeyPath(env.proxy, "no-such-key"), "")
	assertAgentProxyError(t, rec, http.StatusNotFound, apperror.CodeRESTAPIAPIKeyNotFound)
}

// Like REST API keys, update and revoke need a gateway to broadcast to: on an
// Agent proxy associated with none they are refused with 503 and change nothing.
func TestAgentProxyAPIKey_UpdateAndRevokeNeedAGateway(t *testing.T) {
	env := newAgentProxyTestEnv(t, agentDeployConfig())
	createAgentProxyForKeys(t, env.handler, "weather-agent")
	createAgentKey(t, env.handler, "weather-agent", `{"id": "consumer-key", "displayName": "Consumer Key"}`)
	before := readAgentKey(t, env, "weather-agent", "consumer-key")

	rec := callAgentProxy(t, env.handler, http.MethodPut, agentKeyPath("weather-agent", "consumer-key"), updateKeyBody("new-value-0123456789", "K"))
	assertAgentProxyError(t, rec, http.StatusServiceUnavailable, apperror.CodeGatewayConnectionUnavailable)
	rec = callAgentProxy(t, env.handler, http.MethodDelete, agentKeyPath("weather-agent", "consumer-key"), "")
	assertAgentProxyError(t, rec, http.StatusServiceUnavailable, apperror.CodeGatewayConnectionUnavailable)

	if after := readAgentKey(t, env, "weather-agent", "consumer-key"); after != before {
		t.Fatalf("a refused call changed the key: before %+v, after %+v", before, after)
	}
}

// --- boundaries --------------------------------------------------------------

// A key is addressed through its own Agent proxy only: the same key name on a
// different Agent proxy, or the right Agent proxy handle in another
// organization, resolves to nothing.
func TestAgentProxyAPIKey_CrossAgentAndCrossOrgAccessFails(t *testing.T) {
	env := deployedAgentKeysEnv(t)
	createAgentProxyForKeys(t, env.handler, "travel-agent")
	createAgentKey(t, env.handler, env.proxy, `{"id": "consumer-key", "displayName": "Consumer Key"}`)
	before := readAgentKey(t, env.agentProxyTestEnv, env.proxy, "consumer-key")

	// Through another Agent proxy in the same org, even for the key's creator
	// and a key admin.
	for _, call := range []func(method, path, body string) *httptest.ResponseRecorder{
		func(m, p, b string) *httptest.ResponseRecorder { return callAgentProxy(t, env.handler, m, p, b) },
		func(m, p, b string) *httptest.ResponseRecorder { return callAsKeyAdmin(t, env.handler, m, p, b) },
	} {
		rec := call(http.MethodPut, agentKeyPath("travel-agent", "consumer-key"), updateKeyBody("hijacked-value-0123456789", "K"))
		assertAgentProxyError(t, rec, http.StatusNotFound, apperror.CodeRESTAPIAPIKeyNotFound)
		rec = call(http.MethodDelete, agentKeyPath("travel-agent", "consumer-key"), "")
		assertAgentProxyError(t, rec, http.StatusNotFound, apperror.CodeRESTAPIAPIKeyNotFound)
		body := decodeAgentProxyJSON(t, call(http.MethodGet, agentKeysPath("travel-agent"), ""), http.StatusOK)
		if list := body["list"].([]any); len(list) != 0 {
			t.Fatalf("travel-agent lists weather-agent's key: %v", list)
		}
	}

	// From another organization, the Agent proxy itself does not exist.
	other := func(method, path, body string) *httptest.ResponseRecorder {
		return callAgentProxyAs(t, env.handler, agentProxyOtherOrg, agentProxyActor, method, path, body)
	}
	assertAgentProxyError(t, other(http.MethodGet, agentKeysPath(env.proxy), ""), http.StatusNotFound, apperror.CodeAgentProxyNotFound)
	assertAgentProxyError(t, other(http.MethodPost, agentKeysPath(env.proxy), `{"displayName": "K"}`), http.StatusNotFound, apperror.CodeAgentProxyNotFound)
	assertAgentProxyError(t, other(http.MethodPut, agentKeyPath(env.proxy, "consumer-key"), updateKeyBody("hijacked-value-0123456789", "K")), http.StatusNotFound, apperror.CodeAgentProxyNotFound)
	assertAgentProxyError(t, other(http.MethodDelete, agentKeyPath(env.proxy, "consumer-key"), ""), http.StatusNotFound, apperror.CodeAgentProxyNotFound)

	if after := readAgentKey(t, env.agentProxyTestEnv, env.proxy, "consumer-key"); after != before {
		t.Fatalf("cross-boundary calls changed the key: before %+v, after %+v", before, after)
	}
}

// Keys are creator-scoped: another user in the same organization neither sees
// nor changes them, and only ap:api_key:all:manage widens that.
func TestAgentProxyAPIKey_OwnershipIsCreatorScopedUnlessKeyAdmin(t *testing.T) {
	env := deployedAgentKeysEnv(t)
	createAgentKey(t, env.handler, env.proxy, `{"id": "authors-key", "displayName": "Author's Key"}`)
	before := readAgentKey(t, env.agentProxyTestEnv, env.proxy, "authors-key")
	asOther := func(method, path, body string) *httptest.ResponseRecorder {
		return callAgentProxyAs(t, env.handler, agentProxyOrg, agentKeyOtherUser, method, path, body)
	}

	body := decodeAgentProxyJSON(t, asOther(http.MethodGet, agentKeysPath(env.proxy), ""), http.StatusOK)
	if list := body["list"].([]any); len(list) != 0 {
		t.Fatalf("another user lists the author's key: %v", list)
	}
	assertAgentProxyError(t, asOther(http.MethodPut, agentKeyPath(env.proxy, "authors-key"), updateKeyBody("stolen-value-0123456789", "Mine")),
		http.StatusForbidden, apperror.CodeRESTAPIAPIKeyForbidden)
	assertAgentProxyError(t, asOther(http.MethodDelete, agentKeyPath(env.proxy, "authors-key"), ""),
		http.StatusForbidden, apperror.CodeRESTAPIAPIKeyForbidden)
	if got := readAgentKey(t, env.agentProxyTestEnv, env.proxy, "authors-key"); got != before {
		t.Fatalf("a forbidden call changed the key: %+v", got)
	}

	// The other user's own key is theirs to manage.
	decodeAgentProxyJSON(t, asOther(http.MethodPost, agentKeysPath(env.proxy), `{"id": "others-key", "displayName": "Other's Key"}`), http.StatusCreated)
	body = decodeAgentProxyJSON(t, asOther(http.MethodGet, agentKeysPath(env.proxy), ""), http.StatusOK)
	if list := body["list"].([]any); len(list) != 1 || list[0].(map[string]any)["id"] != "others-key" {
		t.Fatalf("other user's list = %v, want only their own key", list)
	}

	// A key admin sees and manages every key on the Agent proxy.
	body = decodeAgentProxyJSON(t, callAsKeyAdmin(t, env.handler, http.MethodGet, agentKeysPath(env.proxy), ""), http.StatusOK)
	if list := body["list"].([]any); len(list) != 2 {
		t.Fatalf("key admin list = %v, want both keys", list)
	}
	decodeAgentProxyJSON(t, callAsKeyAdmin(t, env.handler, http.MethodPut, agentKeyPath(env.proxy, "authors-key"),
		updateKeyBody("admin-rotated-0123456789", "Author's Key")), http.StatusOK)
	decodeAgentProxyJSON(t, callAsKeyAdmin(t, env.handler, http.MethodDelete, agentKeyPath(env.proxy, "authors-key"), ""), http.StatusNoContent)
	if got := readAgentKey(t, env.agentProxyTestEnv, env.proxy, "authors-key"); got.Status != constants.APIKeyStatusRevoked {
		t.Fatalf("key admin revoke left status %q", got.Status)
	}
}

// An Agent proxy's own :manage scope authorizes the operation, not cross-user
// access: only ap:api_key:all:manage overrides creator scoping (GO-AUTH-020).
func TestAgentProxyAPIKey_AgentManageScopeDoesNotOverrideOwnership(t *testing.T) {
	env := deployedAgentKeysEnv(t)
	createAgentKey(t, env.handler, env.proxy, `{"id": "authors-key", "displayName": "Author's Key"}`)

	for _, scope := range []string{"ap:agent_proxy:manage", "ap:agent_proxy:api_key:manage"} {
		req := httptest.NewRequest(http.MethodDelete, agentKeyPath(env.proxy, "authors-key"), nil)
		req.Header.Set("X-Test-Org", agentProxyOrg)
		req.Header.Set("X-Test-User", agentKeyOtherUser)
		req.Header.Set("X-Test-Scope", scope)
		rec := httptest.NewRecorder()
		env.handler.ServeHTTP(rec, req)
		assertAgentProxyError(t, rec, http.StatusForbidden, apperror.CodeRESTAPIAPIKeyForbidden)
	}
}

// --- gateway propagation -----------------------------------------------------

// Every mutation reaches the Agent's gateways as the kind-agnostic apikey.*
// event, keyed by the artifact UUID and carrying hashes only.
func TestAgentProxyAPIKey_MutationsBroadcastToDeployedGateways(t *testing.T) {
	env := deployedAgentKeysEnv(t)
	agentUUID := env.agentUUID(t)

	created := createAgentKey(t, env.handler, env.proxy, `{"id": "consumer-key", "displayName": "Consumer Key"}`)
	plaintext := created["apiKey"].(string)
	stored := readAgentKey(t, env.agentProxyTestEnv, env.proxy, "consumer-key")

	ev := env.lastEvent(t, env.gatewayUUID)
	if ev.Type != "apikey.created" || ev.Payload["apiId"] != agentUUID || ev.Payload["name"] != "consumer-key" ||
		ev.Payload["uuid"] != stored.UUID || ev.Payload["apiKeyHashes"] != stored.Hashes {
		t.Fatalf("created event = %s %v", ev.Type, ev.Payload)
	}

	const rotated = "rotated-key-value-0123456789abcdefghij"
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPut, agentKeyPath(env.proxy, "consumer-key"),
		updateKeyBody(rotated, "Consumer Key")), http.StatusOK)
	ev = env.lastEvent(t, env.gatewayUUID)
	hashes, _ := ev.Payload["apiKeyHashes"].(string)
	if ev.Type != "apikey.updated" || ev.Payload["apiId"] != agentUUID || ev.Payload["keyName"] != "consumer-key" ||
		!strings.Contains(hashes, sha256Hex(rotated)) {
		t.Fatalf("updated event = %s %v, want the rotated key's hashes", ev.Type, ev.Payload)
	}

	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodDelete, agentKeyPath(env.proxy, "consumer-key"), ""), http.StatusNoContent)
	ev = env.lastEvent(t, env.gatewayUUID)
	if ev.Type != "apikey.revoked" || ev.Payload["apiId"] != agentUUID || ev.Payload["keyName"] != "consumer-key" {
		t.Fatalf("revoked event = %s %v", ev.Type, ev.Payload)
	}

	raw, _ := json.Marshal(env.recordedEvents(t, env.gatewayUUID))
	if bytes.Contains(raw, []byte(plaintext)) || bytes.Contains(raw, []byte(rotated)) {
		t.Fatal("a gateway event carries a plaintext key")
	}
}

// A key created before the Agent proxy is deployed anywhere is delivered to the
// gateway at deploy time.
func TestAgentProxyAPIKey_KeyCreatedBeforeDeployReachesTheGatewayAtDeploy(t *testing.T) {
	env := setupAgentDeployEnv(t)
	created := createAgentKey(t, env.handler, env.proxy, `{"id": "early-key", "displayName": "Early Key"}`)

	env.deploy(t, env.gateway)
	var delivered bool
	for _, ev := range env.recordedEvents(t, env.gatewayUUID) {
		if ev.Type == "apikey.created" && ev.Payload["name"] == "early-key" && ev.Payload["apiId"] == env.agentUUID(t) {
			delivered = true
		}
	}
	if !delivered {
		t.Fatalf("the pre-existing key was not delivered at deploy time; events: %v", env.recordedEvents(t, env.gatewayUUID))
	}
	raw, _ := json.Marshal(env.recordedEvents(t, env.gatewayUUID))
	if bytes.Contains(raw, []byte(created["apiKey"].(string))) {
		t.Fatal("the deploy-time backfill carries the plaintext key")
	}
}

// Broadcast is best-effort, as for REST API keys: the key is persisted before
// delivery, so a failed publish still returns success and the gateway converges
// through the reconnect backfill.
func TestAgentProxyAPIKey_BroadcastFailureDoesNotFailTheMutation(t *testing.T) {
	env := setupAgentInternalEnv(t)
	env.deploy(t, env.gateway)
	env.hub.failPublish.Store(true)

	created := createAgentKey(t, env.handler, env.proxy, `{"id": "consumer-key", "displayName": "Consumer Key"}`)
	if !generatedAPIKeyPattern.MatchString(fmt.Sprint(created["apiKey"])) {
		t.Fatalf("creation response = %v, want the one-time key despite the failed broadcast", created)
	}
	if keys := decodeKeyList(t, env.call(t, internalAgentAPIKeys, env.gatewayUUID)); len(keys) != 1 {
		t.Fatalf("backfill = %v, want the persisted key available for the gateway to converge on", keyNames(keys))
	}

	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodDelete, agentKeyPath(env.proxy, "consumer-key"), ""), http.StatusNoContent)
	if got := readAgentKey(t, env.agentProxyTestEnv, env.proxy, "consumer-key"); got.Status != constants.APIKeyStatusRevoked {
		t.Fatalf("status = %q, want revoked even though the revocation event could not be published", got.Status)
	}
}

// A key created through the public API is what the gateway's reconnect backfill
// serves on /agents/api-keys — with the hashes the gateway verifies against,
// which the public API never returns.
func TestAgentProxyAPIKey_PublicKeysFeedTheGatewayBackfill(t *testing.T) {
	env := setupAgentInternalEnv(t)
	env.deploy(t, env.gateway)
	created := createAgentKey(t, env.handler, env.proxy, `{"id": "consumer-key", "displayName": "Consumer Key", "issuer": "api-platform-devportal"}`)
	plaintext := created["apiKey"].(string)

	keys := decodeKeyList(t, env.call(t, internalAgentAPIKeys, env.gatewayUUID))
	if len(keys) != 1 || keys[0]["name"] != "consumer-key" || keys[0]["artifactUuid"] != env.agentUUID {
		t.Fatalf("backfill = %v, want the public key against the Agent's artifact UUID", keys)
	}
	hashes, _ := keys[0]["apiKeyHashes"].(map[string]any)
	if hashes["sha256"] != sha256Hex(plaintext) {
		t.Fatalf("backfill hashes = %v, want the sha256 of the created key", hashes)
	}
	if filtered := decodeKeyList(t, env.call(t, internalAgentAPIKeys+"?issuer=api-platform-devportal", env.gatewayUUID)); len(filtered) != 1 {
		t.Fatalf("issuer-filtered backfill = %v", keyNames(filtered))
	}

	// A rotation is what the next backfill serves.
	const rotated = "rotated-key-value-0123456789abcdefghij"
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPut, agentKeyPath(env.proxy, "consumer-key"),
		updateKeyBody(rotated, "Consumer Key")), http.StatusOK)
	keys = decodeKeyList(t, env.call(t, internalAgentAPIKeys, env.gatewayUUID))
	if hashes, _ := keys[0]["apiKeyHashes"].(map[string]any); hashes["sha256"] != sha256Hex(rotated) {
		t.Fatalf("backfill after rotation = %v, want the rotated key's hash", hashes)
	}

	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodDelete, agentKeyPath(env.proxy, "consumer-key"), ""), http.StatusNoContent)
	keys = decodeKeyList(t, env.call(t, internalAgentAPIKeys, env.gatewayUUID))
	if len(keys) != 1 || keys[0]["status"] != "revoked" {
		t.Fatalf("backfill after revoke = %v, want the key reported as revoked", keys)
	}

	// The public listing never carries what the backfill does.
	rec := callAgentProxy(t, env.handler, http.MethodGet, agentKeysPath(env.proxy), "")
	decodeAgentProxyJSON(t, rec, http.StatusOK)
	assertNoKeyMaterial(t, rec.Body.Bytes(), plaintext, sha256Hex(plaintext), sha256Hex(rotated))
}

// --- published contract ------------------------------------------------------

func newPublicSpecValidator(t *testing.T) validator.Validator {
	t.Helper()
	specPath := filepath.Join("..", "..", "resources", "openapi.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %q: %v", specPath, err)
	}
	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		t.Fatalf("parse %q: %v", specPath, err)
	}
	v, errs := validator.NewValidator(doc)
	if len(errs) > 0 {
		t.Fatalf("build validator for %q: %v", specPath, errs)
	}
	return v
}

// The handlers' actual responses — success and error, body and headers — must
// match what resources/openapi.yaml documents for these four operations.
//
// Only responses are validated. The request bodies are the shared
// CreateAPIKeyRequest/UpdateAPIKeyRequest schemas, which REST, WebSub and
// WebBroker keys use too, and which still carry OpenAPI 3.0 `nullable` — a
// keyword a 3.1 validator refuses to compile.
func TestAgentProxyAPIKey_ResponsesConformToThePublicSpec(t *testing.T) {
	env := deployedAgentKeysEnv(t)
	createAgentProxyForKeys(t, env.handler, "undeployed-agent")
	createAgentKey(t, env.handler, "undeployed-agent", `{"id": "idle-key", "displayName": "Idle Key"}`)
	v := newPublicSpecValidator(t)
	keys, key := agentKeysPath(env.proxy), agentKeyPath(env.proxy, "consumer-key")

	cases := []struct {
		name, method, path, body string
		want                     int
		asOther                  bool
	}{
		{"create", http.MethodPost, keys, `{"id": "consumer-key", "displayName": "Consumer Key", "expiresIn": {"duration": 30, "unit": "days"}}`, http.StatusCreated, false},
		{"create invalid", http.MethodPost, keys, `{"id": "k-one"}`, http.StatusBadRequest, false},
		{"create for unknown agent", http.MethodPost, agentKeysPath("no-such-agent"), `{"displayName": "K"}`, http.StatusNotFound, false},
		{"list", http.MethodGet, keys, "", http.StatusOK, false},
		{"user key listing", http.MethodGet, constants.APIBasePath + "/me/api-keys?type=AgentProxy", "", http.StatusOK, false},
		{"update", http.MethodPut, key, updateKeyBody("rotated-value-0123456789", "Renamed"), http.StatusOK, false},
		{"update unknown key", http.MethodPut, agentKeyPath(env.proxy, "no-such-key"), updateKeyBody("rotated-value-0123456789", "K"), http.StatusNotFound, false},
		{"update another user's key", http.MethodPut, key, updateKeyBody("rotated-value-0123456789", "K"), http.StatusForbidden, true},
		{"update without a gateway", http.MethodPut, agentKeyPath("undeployed-agent", "idle-key"), updateKeyBody("rotated-value-0123456789", "K"), http.StatusServiceUnavailable, false},
		{"revoke another user's key", http.MethodDelete, key, "", http.StatusForbidden, true},
		{"revoke", http.MethodDelete, key, "", http.StatusNoContent, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actor := agentProxyActor
			if tc.asOther {
				actor = agentKeyOtherUser
			}
			rec := callAgentProxyAs(t, env.handler, agentProxyOrg, actor, tc.method, tc.path, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
			// The validator routes on the spec's server base URL.
			req := httptest.NewRequest(tc.method, "https://localhost:9243"+tc.path, nil)
			if ok, errs := v.ValidateHttpResponse(req, rec.Result()); !ok {
				t.Fatalf("response does not conform to the public spec: %v", errs)
			}
		})
	}
}

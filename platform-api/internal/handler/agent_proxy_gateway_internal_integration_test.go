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

// The gateway-internal Agent routes over the real route -> handler -> service
// -> repository stack, backed by SQLite: the gateway pulls the immutable
// deployment snapshot by the Agent proxy's artifact UUID, and backfills Agent
// API keys. Deployments are created through the public deployment API so the
// served bytes are exactly what Section 8 stored.

package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

const (
	internalAgentsBase   = "/api/internal/v1/agents"
	internalAgentAPIKeys = internalAgentsBase + "/api-keys"
	internalRESTAPIKeys  = "/api/internal/v1/apis/api-keys"
	internalDeployments  = "/api/internal/v1/deployments"
)

// agentInternalEnv is an Agent proxy deployment environment plus the
// gateway-internal API, with a token for every seeded gateway.
type agentInternalEnv struct {
	*agentDeployEnv
	internal  http.Handler
	agentUUID string
	// tokens maps a gateway UUID to the plaintext api-key it authenticates with.
	tokens map[string]string
}

func setupAgentInternalEnv(t *testing.T) *agentInternalEnv {
	t.Helper()
	deployEnv := setupAgentDeployEnv(t)
	db := deployEnv.db

	registry := repository.NewArtifactTableRegistry()
	identity := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	gatewayRepo := repository.NewGatewayRepo(db)
	gatewaySvc := service.NewGatewayService(gatewayRepo, nil, nil, nil, nil, slog.Default(), false, false, nil, identity)
	internalSvc := service.NewGatewayInternalAPIService(
		nil, nil, nil, nil, nil, nil,
		repository.NewAgentProxyRepo(db),
		repository.NewDeploymentRepo(db, registry), gatewayRepo,
		nil, nil,
		repository.NewAPIKeyRepo(db, registry),
		nil, nil,
		&config.Server{}, slog.Default(),
	)
	mux := http.NewServeMux()
	NewGatewayInternalAPIHandler(gatewaySvc, internalSvc, nil, nil, slog.Default()).RegisterRoutes(mux)

	var agentUUID string
	if err := db.QueryRow(`SELECT uuid FROM agent_proxies WHERE handle = ? AND organization_uuid = ?`,
		deployEnv.proxy, agentProxyOrg).Scan(&agentUUID); err != nil {
		t.Fatalf("resolve agent proxy uuid: %v", err)
	}

	env := &agentInternalEnv{
		agentDeployEnv: deployEnv,
		internal:       mux,
		agentUUID:      agentUUID,
		tokens:         map[string]string{},
	}
	env.issueToken(t, deployEnv.gatewayUUID)
	return env
}

// issueToken registers an active gateway token for gatewayUUID.
func (e *agentInternalEnv) issueToken(t *testing.T, gatewayUUID string) string {
	t.Helper()
	token := "token-" + gatewayUUID
	if _, err := e.db.Exec(`INSERT INTO gateway_tokens (uuid, gateway_uuid, token_hash, salt, status, created_at)
		VALUES (?, ?, ?, 'dummy-salt', 'active', datetime('now'))`,
		"tok-"+gatewayUUID, gatewayUUID, testHashToken(token)); err != nil {
		t.Fatalf("insert gateway token for %s: %v", gatewayUUID, err)
	}
	e.tokens[gatewayUUID] = token
	return token
}

// seedGateway adds another gateway (with a token) and returns its UUID.
func (e *agentInternalEnv) seedGateway(t *testing.T, org, handle string) string {
	t.Helper()
	gatewayUUID := seedAgentGateway(t, e.db, org, handle, "1.2.0")
	e.issueToken(t, gatewayUUID)
	return gatewayUUID
}

func internalRequest(t *testing.T, path, apiKey string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if apiKey != "" {
		req.Header.Set("api-key", apiKey)
	}
	return req
}

// call issues a gateway-internal GET as the gateway identified by gatewayUUID.
func (e *agentInternalEnv) call(t *testing.T, path, gatewayUUID string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	e.internal.ServeHTTP(rec, internalRequest(t, path, e.tokens[gatewayUUID]))
	return rec
}

// fetch requests the Agent artifact identified by agentID as gatewayUUID.
func (e *agentInternalEnv) fetch(t *testing.T, agentID, gatewayUUID string) *httptest.ResponseRecorder {
	t.Helper()
	return e.call(t, internalAgentsBase+"/"+agentID, gatewayUUID)
}

// unzipSingle asserts the archive holds exactly one entry and returns it.
func unzipSingle(t *testing.T, data []byte) (string, []byte) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("response is not a ZIP archive: %v", err)
	}
	if len(zr.File) != 1 {
		names := make([]string, 0, len(zr.File))
		for _, f := range zr.File {
			names = append(names, f.Name)
		}
		t.Fatalf("ZIP entries = %v, want exactly one", names)
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatalf("open ZIP entry: %v", err)
	}
	defer rc.Close()
	content, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read ZIP entry: %v", err)
	}
	return zr.File[0].Name, content
}

func assertInternalError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantDescription string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d: %s", rec.Code, wantStatus, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not JSON: %v: %s", err, rec.Body.String())
	}
	if body["description"] != wantDescription {
		t.Fatalf("description = %v, want %q", body["description"], wantDescription)
	}
}

func TestGatewayInternalAgent_FetchServesTheDeploymentSnapshotAsZip(t *testing.T) {
	env := setupAgentInternalEnv(t)
	deploymentID := env.deploy(t, env.gateway)

	// The gateway fetches while the deployment is still DEPLOYING: that is how
	// it completes the deploy, so the transitional state must be served.
	rec := env.fetch(t, env.agentUUID, env.gatewayUUID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("Content-Type = %q, want application/zip", got)
	}
	wantDisposition := fmt.Sprintf(`attachment; filename="agent-%s.zip"`, env.agentUUID)
	if got := rec.Header().Get("Content-Disposition"); got != wantDisposition {
		t.Fatalf("Content-Disposition = %q, want %q", got, wantDisposition)
	}
	if got := rec.Header().Get("Content-Length"); got != fmt.Sprint(rec.Body.Len()) {
		t.Fatalf("Content-Length = %q, want %d", got, rec.Body.Len())
	}

	name, content := unzipSingle(t, rec.Body.Bytes())
	if want := "agent-" + env.agentUUID + ".yaml"; name != want {
		t.Fatalf("ZIP entry = %q, want %q", name, want)
	}
	if !bytes.Equal(content, env.storedContent(t, deploymentID)) {
		t.Fatalf("served artifact differs from the stored deployment snapshot")
	}

	// The gateway consumes its own nested spec.a2a shape, not the public
	// Agent proxy schema: transports under operationConfigs, no protocol.
	var artifact struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Spec       struct {
			A2A map[string]any `yaml:"a2a"`
			Raw map[string]any `yaml:",inline"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(content, &artifact); err != nil {
		t.Fatalf("artifact is not YAML: %v", err)
	}
	if artifact.APIVersion != constants.GatewayApiVersion || artifact.Kind != constants.GatewayKindAgent {
		t.Fatalf("apiVersion/kind = %s/%s, want %s/%s",
			artifact.APIVersion, artifact.Kind, constants.GatewayApiVersion, constants.GatewayKindAgent)
	}
	if _, ok := artifact.Spec.Raw["protocol"]; ok {
		t.Fatalf("control-plane-only protocol leaked into the gateway spec")
	}
	if _, ok := artifact.Spec.A2A["transports"]; ok {
		t.Fatalf("spec.a2a carries the CP authoring layout's transports; want them under operationConfigs")
	}
	opConfigs, _ := artifact.Spec.A2A["operationConfigs"].(map[string]any)
	if transports, _ := opConfigs["transports"].([]any); len(transports) != 2 {
		t.Fatalf("spec.a2a.operationConfigs.transports = %v, want both transports", opConfigs["transports"])
	}
	card, _ := artifact.Spec.A2A["agentCard"].(map[string]any)
	public, _ := card["public"].(map[string]any)
	protected, _ := card["protected"].(map[string]any)
	if public["mode"] != "managed" || public["content"] == nil || protected["mode"] != "passthrough" {
		t.Fatalf("spec.a2a.agentCard = %v, want the managed public and passthrough protected cards", card)
	}
}

// An edit to the Agent proxy must not reach a gateway until it is deployed:
// the fetch serves the snapshot, never a re-render of the current definition.
func TestGatewayInternalAgent_UpdateDoesNotChangeFetchUntilRedeployed(t *testing.T) {
	env := setupAgentInternalEnv(t)
	first := env.deploy(t, env.gateway)
	before := env.fetch(t, env.agentUUID, env.gatewayUUID)
	if before.Code != http.StatusOK {
		t.Fatalf("initial fetch status = %d: %s", before.Code, before.Body.String())
	}
	_, beforeContent := unzipSingle(t, before.Body.Bytes())

	edited := strings.Replace(fullAgentProxyBody(env.proxy), `"displayName": "Weather Agent"`, `"displayName": "Storm Agent"`, 1)
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPut, agentProxyBase+"/"+env.proxy, edited), http.StatusOK)

	after := env.fetch(t, env.agentUUID, env.gatewayUUID)
	if after.Code != http.StatusOK {
		t.Fatalf("fetch after update status = %d: %s", after.Code, after.Body.String())
	}
	if _, afterContent := unzipSingle(t, after.Body.Bytes()); !bytes.Equal(afterContent, beforeContent) {
		t.Fatalf("an Agent proxy update changed the served artifact without a new deployment")
	}

	env.ack(t, first, "DEPLOYED", "")
	second := env.deploy(t, env.gateway)
	redeployed := env.fetch(t, env.agentUUID, env.gatewayUUID)
	_, redeployedContent := unzipSingle(t, redeployed.Body.Bytes())
	if !bytes.Equal(redeployedContent, env.storedContent(t, second)) {
		t.Fatalf("fetch after redeploy does not serve the new deployment's snapshot")
	}
	if !bytes.Contains(redeployedContent, []byte("Storm Agent")) {
		t.Fatalf("redeployed artifact does not carry the update:\n%s", redeployedContent)
	}
}

func TestGatewayInternalAgent_FetchIsScopedToTheCallingGatewayAndOrg(t *testing.T) {
	env := setupAgentInternalEnv(t)
	env.deploy(t, env.gateway)

	t.Run("public handle is not an internal artifact id", func(t *testing.T) {
		assertInternalError(t, env.fetch(t, env.proxy, env.gatewayUUID), http.StatusNotFound, "Agent not found")
	})

	t.Run("unknown uuid", func(t *testing.T) {
		assertInternalError(t, env.fetch(t, "00000000-0000-0000-0000-000000000000", env.gatewayUUID),
			http.StatusNotFound, "Agent not found")
	})

	t.Run("same org, gateway it is not deployed on", func(t *testing.T) {
		sibling := env.seedGateway(t, agentProxyOrg, "ai-gw-2")
		assertInternalError(t, env.fetch(t, env.agentUUID, sibling), http.StatusNotFound,
			"No active deployment found for this Agent on this gateway")
	})

	t.Run("gateway of another organization", func(t *testing.T) {
		foreign := env.seedGateway(t, agentProxyOtherOrg, "foreign-gw")
		assertInternalError(t, env.fetch(t, env.agentUUID, foreign), http.StatusNotFound, "Agent not found")
	})
}

func TestGatewayInternalAgent_FetchRefusesAnUndeployedAgent(t *testing.T) {
	env := setupAgentInternalEnv(t)
	deploymentID := env.deploy(t, env.gateway)
	env.ack(t, deploymentID, "DEPLOYED", "")

	itemPath := env.deploymentsPath() + "/" + deploymentID
	decodeAgentProxyJSON(t,
		callAgentProxy(t, env.handler, http.MethodPost, itemPath+"/undeploy?gatewayId="+env.gateway, ""),
		http.StatusAccepted)

	assertInternalError(t, env.fetch(t, env.agentUUID, env.gatewayUUID), http.StatusNotFound,
		"No active deployment found for this Agent on this gateway")
}

func TestGatewayInternalAgent_RoutesRequireAGatewayAPIKey(t *testing.T) {
	env := setupAgentInternalEnv(t)
	env.deploy(t, env.gateway)

	for _, path := range []string{internalAgentsBase + "/" + env.agentUUID, internalAgentAPIKeys} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			env.internal.ServeHTTP(rec, internalRequest(t, path, ""))
			assertInternalError(t, rec, http.StatusUnauthorized, "API key is required. Provide 'api-key' header.")

			rec = httptest.NewRecorder()
			env.internal.ServeHTTP(rec, internalRequest(t, path, "not-a-gateway-token"))
			assertInternalError(t, rec, http.StatusUnauthorized, "Invalid or expired API key")
		})
	}
}

// insertAPIKey stores an API key against artifactUUID directly, bypassing the
// public API-key operations so a test can pin an exact issuer or status.
func (e *agentInternalEnv) insertAPIKey(t *testing.T, artifactUUID, handle string, issuer any) {
	t.Helper()
	if _, err := e.db.Exec(`INSERT INTO api_keys (uuid, artifact_uuid, handle, display_name, masked_api_key,
		api_key_hashes, status, created_by, issuer)
		VALUES (?, ?, ?, ?, 'ab****yz', '{"sha256":"deadbeef"}', 'active', 'alice', ?)`,
		"key-"+handle, artifactUUID, handle, handle, issuer); err != nil {
		t.Fatalf("insert api key %s: %v", handle, err)
	}
}

func decodeKeyList(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var keys []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
		t.Fatalf("api-keys body is not a JSON array: %v: %s", err, rec.Body.String())
	}
	return keys
}

func keyNames(keys []map[string]any) []string {
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		names = append(names, fmt.Sprint(k["name"]))
	}
	return names
}

func TestGatewayInternalAgent_APIKeyBackfill(t *testing.T) {
	env := setupAgentInternalEnv(t)
	env.deploy(t, env.gateway)
	env.insertAPIKey(t, env.agentUUID, "portal-key", "api-platform-devportal")
	env.insertAPIKey(t, env.agentUUID, "plain-key", nil)

	all := decodeKeyList(t, env.call(t, internalAgentAPIKeys, env.gatewayUUID))
	if len(all) != 2 {
		t.Fatalf("keys = %v, want both Agent keys", keyNames(all))
	}
	for _, k := range all {
		if k["artifactUuid"] != env.agentUUID {
			t.Fatalf("artifactUuid = %v, want the Agent artifact UUID %s", k["artifactUuid"], env.agentUUID)
		}
		if k["source"] != "external" {
			t.Fatalf("source = %v, want external", k["source"])
		}
		if _, ok := k["apiKeyHashes"].(map[string]any); !ok {
			t.Fatalf("apiKeyHashes = %v, want the hash map the gateway verifies against", k["apiKeyHashes"])
		}
	}

	filtered := decodeKeyList(t, env.call(t, internalAgentAPIKeys+"?issuer=api-platform-devportal", env.gatewayUUID))
	if names := keyNames(filtered); len(names) != 1 || names[0] != "portal-key" {
		t.Fatalf("issuer-filtered keys = %v, want [portal-key]", names)
	}

	// The backfill is per kind: Agent keys are not served on the REST API
	// route, and a gateway the Agent is not deployed on receives none.
	if rest := decodeKeyList(t, env.call(t, internalRESTAPIKeys, env.gatewayUUID)); len(rest) != 0 {
		t.Fatalf("REST API backfill returned Agent keys: %v", keyNames(rest))
	}
	sibling := env.seedGateway(t, agentProxyOrg, "ai-gw-2")
	if keys := decodeKeyList(t, env.call(t, internalAgentAPIKeys, sibling)); len(keys) != 0 {
		t.Fatalf("gateway without the deployment received Agent keys: %v", keyNames(keys))
	}
}

// The static api-keys segment must never be captured as an {agentId}.
func TestGatewayInternalAgent_APIKeysPathIsNotAnAgentID(t *testing.T) {
	env := setupAgentInternalEnv(t)
	rec := env.call(t, internalAgentAPIKeys, env.gatewayUUID)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") == "application/zip" {
		t.Fatalf("GET %s = %d %q, want the JSON key list", internalAgentAPIKeys, rec.Code, rec.Header().Get("Content-Type"))
	}
}

// The gateway sorts and dispatches its reconnect sync on its own kind
// vocabulary, which has Agent and no AgentProxy.
func TestGatewayInternalAgent_DeploymentListReportsTheGatewayKind(t *testing.T) {
	env := setupAgentInternalEnv(t)
	deploymentID := env.deploy(t, env.gateway)

	rec := env.call(t, internalDeployments, env.gatewayUUID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Deployments []map[string]any `json:"deployments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode deployments: %v", err)
	}
	if len(body.Deployments) != 1 {
		t.Fatalf("deployments = %v, want one", body.Deployments)
	}
	got := body.Deployments[0]
	if got["kind"] != constants.GatewayKindAgent || got["artifactId"] != env.agentUUID || got["deploymentId"] != deploymentID {
		t.Fatalf("deployment = %v, want kind %s for artifact %s", got, constants.GatewayKindAgent, env.agentUUID)
	}
}

// newInternalSpecValidator builds a validator over the shipped gateway-internal
// spec. The spec is documentation only — nothing enforces it at request time —
// so this pins the documented contract to what the handlers actually send.
func newInternalSpecValidator(t *testing.T) validator.Validator {
	t.Helper()
	specPath := filepath.Join("..", "..", "resources", "gateway-internal-api.yaml")
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

func TestGatewayInternalAgent_ResponsesConformToTheInternalSpec(t *testing.T) {
	env := setupAgentInternalEnv(t)
	env.deploy(t, env.gateway)
	env.insertAPIKey(t, env.agentUUID, "portal-key", "api-platform-devportal")
	env.insertAPIKey(t, env.agentUUID, "plain-key", nil)
	other := env.seedGateway(t, agentProxyOrg, "ai-gw-2")
	v := newInternalSpecValidator(t)

	cases := []struct {
		name       string
		path       string
		apiKey     string
		wantStatus int
	}{
		{"artifact", internalAgentsBase + "/" + env.agentUUID, env.tokens[env.gatewayUUID], http.StatusOK},
		{"artifact not deployed here", internalAgentsBase + "/" + env.agentUUID, env.tokens[other], http.StatusNotFound},
		{"artifact without api-key", internalAgentsBase + "/" + env.agentUUID, "", http.StatusUnauthorized},
		{"api keys", internalAgentAPIKeys, env.tokens[env.gatewayUUID], http.StatusOK},
		{"api keys by issuer", internalAgentAPIKeys + "?issuer=api-platform-devportal", env.tokens[env.gatewayUUID], http.StatusOK},
		{"api keys without api-key", internalAgentAPIKeys, "", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The validator routes on the spec's server base URL.
			req := internalRequest(t, "https://localhost:9243"+tc.path, tc.apiKey)
			rec := httptest.NewRecorder()
			env.internal.ServeHTTP(rec, internalRequest(t, tc.path, tc.apiKey))
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.apiKey != "" {
				if ok, errs := v.ValidateHttpRequest(req); !ok {
					t.Fatalf("request rejected by the internal spec: %v", errs)
				}
			}
			if ok, errs := v.ValidateHttpResponse(req, rec.Result()); !ok {
				t.Fatalf("response does not conform to the internal spec: %v", errs)
			}
		})
	}
}

// TestGatewayInternalSpecIsAValidOpenAPIDocument validates the internal spec
// against the OpenAPI 3.1 meta-schema, so a malformed addition (a bad $ref, a
// 3.0-only keyword) fails the build instead of shipping as documentation.
func TestGatewayInternalSpecIsAValidOpenAPIDocument(t *testing.T) {
	if ok, errs := newInternalSpecValidator(t).ValidateDocument(); !ok {
		for _, e := range errs {
			t.Errorf("%s: %s", e.Message, e.Reason)
		}
	}
}

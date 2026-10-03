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

// End-to-end Agent proxy deployments over the real route -> handler -> service
// -> repository stack, backed by SQLite. These assert the published HTTP
// contract: 201 for record creation, 202 for undeploy/restore, a Location that
// resolves to the deployment, bodyless 204 deletion of an undeployed record, and
// the immutable snapshot the gateway will be served.
//
// Gateway acknowledgements are simulated here by writing deployment_status
// directly; the ack path itself is exercised in
// agent_proxy_deployment_ack_integration_test.go.

package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
)

// lockedBuffer is a bytes.Buffer safe to share with a slog handler.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// agentDeployConfig is a server configuration the deploy path can run with.
func agentDeployConfig() *config.Server {
	return &config.Server{
		Deployments: config.Deployments{MaxPerAPIGateway: 3, MaxBuildsPerAPI: 0},
	}
}

// seedAgentGateway inserts a gateway new enough to receive the v1 artifact shape
// and returns its UUID.
func seedAgentGateway(t *testing.T, db *database.DB, org, handle, version string) string {
	t.Helper()
	gatewayUUID := "gw-" + org + "-" + handle
	if _, err := db.Exec(`INSERT INTO gateways (uuid, organization_uuid, handle, display_name, description, version,
		gateway_functionality_type, properties, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, '', ?, 'ai', '{}', 1, datetime('now'), datetime('now'))`,
		gatewayUUID, org, handle, handle, version); err != nil {
		t.Fatalf("seed gateway %s: %v", handle, err)
	}
	return gatewayUUID
}

// agentDeployEnv is an Agent proxy with one v1-capable gateway to deploy it to.
type agentDeployEnv struct {
	*agentProxyTestEnv
	proxy       string
	gateway     string
	gatewayUUID string
}

func setupAgentDeployEnv(t *testing.T) *agentDeployEnv {
	t.Helper()
	env := newAgentProxyTestEnv(t, agentDeployConfig())
	rec := callAgentProxy(t, env.handler, http.MethodPost, agentProxyBase, fullAgentProxyBody("weather-agent"))
	decodeAgentProxyJSON(t, rec, http.StatusCreated)
	return &agentDeployEnv{
		agentProxyTestEnv: env,
		proxy:             "weather-agent",
		gateway:           "ai-gw",
		gatewayUUID:       seedAgentGateway(t, env.db, agentProxyOrg, "ai-gw", "1.2.0"),
	}
}

func (e *agentDeployEnv) deploymentsPath() string {
	return agentProxyBase + "/" + e.proxy + "/deployments"
}

func deployBody(name, gateway string) string {
	return fmt.Sprintf(`{"name": %q, "base": "current", "gatewayId": %q}`, name, gateway)
}

// deploy creates a deployment on the named gateway and returns its id.
func (e *agentDeployEnv) deploy(t *testing.T, gateway string) string {
	t.Helper()
	rec := callAgentProxy(t, e.handler, http.MethodPost, e.deploymentsPath(), deployBody("dep", gateway))
	body := decodeAgentProxyJSON(t, rec, http.StatusCreated)
	return body["deploymentId"].(string)
}

// ack simulates the gateway acknowledging a deployment into a terminal state.
func (e *agentDeployEnv) ack(t *testing.T, deploymentID, status, reason string) {
	t.Helper()
	if _, err := e.db.Exec(`UPDATE deployment_status SET status = ?, status_reason = ? WHERE deployment_uuid = ?`,
		status, reason, deploymentID); err != nil {
		t.Fatalf("simulate ack: %v", err)
	}
}

func (e *agentDeployEnv) storedContent(t *testing.T, deploymentID string) []byte {
	t.Helper()
	var content []byte
	if err := e.db.QueryRow(`SELECT content FROM deployments WHERE uuid = ?`, deploymentID).Scan(&content); err != nil {
		t.Fatalf("read deployment content: %v", err)
	}
	return content
}

func (e *agentDeployEnv) countRows(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

func TestAgentProxyDeployment_CreateReturns201DeployingWithResolvableLocation(t *testing.T) {
	env := setupAgentDeployEnv(t)

	rec := callAgentProxy(t, env.handler, http.MethodPost, env.deploymentsPath(), deployBody("first", env.gateway))
	body := decodeAgentProxyJSON(t, rec, http.StatusCreated)

	if body["status"] != "DEPLOYING" {
		t.Fatalf("status = %v, want DEPLOYING — a 201 is the record, not the gateway's result", body["status"])
	}
	if body["gatewayId"] != env.gateway {
		t.Fatalf("gatewayId = %v, want the gateway handle %q", body["gatewayId"], env.gateway)
	}
	if _, leaked := body["content"]; leaked {
		t.Fatalf("deployment response carries artifact content: %s", rec.Body.String())
	}
	deploymentID := body["deploymentId"].(string)
	wantLocation := env.deploymentsPath() + "/" + deploymentID
	if got := rec.Header().Get("Location"); got != wantLocation {
		t.Fatalf("Location = %q, want %q", got, wantLocation)
	}

	got := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet, wantLocation, ""), http.StatusOK)
	if got["deploymentId"] != deploymentID || got["status"] != "DEPLOYING" {
		t.Fatalf("GET Location = %v, want the DEPLOYING deployment %s", got, deploymentID)
	}
}

func TestAgentProxyDeployment_StoresTheGatewayArtifactAndReportsTargetVersion(t *testing.T) {
	env := setupAgentDeployEnv(t)
	deploymentID := env.deploy(t, env.gateway)

	var artifact map[string]any
	if err := yaml.Unmarshal(env.storedContent(t, deploymentID), &artifact); err != nil {
		t.Fatalf("stored content is not YAML: %v", err)
	}
	// The stored apiVersion is the durable record of the shape produced for
	// this gateway.
	if artifact["apiVersion"] != constants.GatewayApiVersion {
		t.Fatalf("apiVersion = %v, want %s", artifact["apiVersion"], constants.GatewayApiVersion)
	}
	if artifact["kind"] != constants.GatewayKindAgent {
		t.Fatalf("kind = %v, want the gateway kind %s", artifact["kind"], constants.GatewayKindAgent)
	}
	spec := artifact["spec"].(map[string]any)
	if _, ok := spec["protocol"]; ok {
		t.Fatalf("control-plane-only protocol leaked into the gateway spec: %v", spec)
	}
	if _, ok := spec["a2a"].(map[string]any)["operationConfigs"]; !ok {
		t.Fatalf("spec.a2a.operationConfigs missing: %v", spec["a2a"])
	}

	logs := env.deployLogs.String()
	for _, want := range []string{`"deploymentID":"` + deploymentID + `"`, `"targetDataVersion":"v1"`, `"gatewayVersion":"1.2.0"`} {
		if !strings.Contains(logs, want) {
			t.Fatalf("deploy log missing %s; logs: %s", want, logs)
		}
	}
}

func TestAgentProxyDeployment_OneRecordAndOneStatusPerGateway(t *testing.T) {
	env := setupAgentDeployEnv(t)
	second := seedAgentGateway(t, env.db, agentProxyOrg, "ai-gw-2", "1.2.0")
	third := seedAgentGateway(t, env.db, agentProxyOrg, "ai-gw-3", "1.3.0")

	for _, gw := range []string{env.gateway, "ai-gw-2", "ai-gw-3"} {
		env.deploy(t, gw)
	}

	for _, gatewayUUID := range []string{env.gatewayUUID, second, third} {
		if n := env.countRows(t, `SELECT COUNT(*) FROM deployments WHERE gateway_uuid = ?`, gatewayUUID); n != 1 {
			t.Fatalf("gateway %s: %d deployments rows, want 1", gatewayUUID, n)
		}
		if n := env.countRows(t, `SELECT COUNT(*) FROM deployment_status WHERE gateway_uuid = ?`, gatewayUUID); n != 1 {
			t.Fatalf("gateway %s: %d deployment_status rows, want 1", gatewayUUID, n)
		}
	}

	list := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet, env.deploymentsPath(), ""), http.StatusOK)
	if list["count"].(float64) != 3 {
		t.Fatalf("list count = %v, want 3", list["count"])
	}
	filtered := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet,
		env.deploymentsPath()+"?gatewayId=ai-gw-2", ""), http.StatusOK)
	if filtered["count"].(float64) != 1 {
		t.Fatalf("gateway-filtered count = %v, want 1", filtered["count"])
	}
}

func TestAgentProxyDeployment_UndeployAndRestoreReturn202WithLocation(t *testing.T) {
	env := setupAgentDeployEnv(t)
	deploymentID := env.deploy(t, env.gateway)
	itemPath := env.deploymentsPath() + "/" + deploymentID
	env.ack(t, deploymentID, "DEPLOYED", "")

	rec := callAgentProxy(t, env.handler, http.MethodPost, itemPath+"/undeploy?gatewayId="+env.gateway, "")
	body := decodeAgentProxyJSON(t, rec, http.StatusAccepted)
	if body["status"] != "UNDEPLOYING" {
		t.Fatalf("undeploy status = %v, want UNDEPLOYING", body["status"])
	}
	if got := rec.Header().Get("Location"); got != itemPath {
		t.Fatalf("undeploy Location = %q, want %q", got, itemPath)
	}

	// Undeploying is not undeployable again.
	assertAgentProxyError(t,
		callAgentProxy(t, env.handler, http.MethodPost, itemPath+"/undeploy?gatewayId="+env.gateway, ""),
		http.StatusConflict, apperror.CodeDeploymentNotActive)

	env.ack(t, deploymentID, "UNDEPLOYED", "")

	rec = callAgentProxy(t, env.handler, http.MethodPost, itemPath+"/restore?gatewayId="+env.gateway, "")
	body = decodeAgentProxyJSON(t, rec, http.StatusAccepted)
	if body["status"] != "DEPLOYING" {
		t.Fatalf("restore status = %v, want DEPLOYING", body["status"])
	}
	if got := rec.Header().Get("Location"); got != itemPath {
		t.Fatalf("restore Location = %q, want %q", got, itemPath)
	}

	// Restoring the deployment that is already current and deploying is a conflict.
	assertAgentProxyError(t,
		callAgentProxy(t, env.handler, http.MethodPost, itemPath+"/restore?gatewayId="+env.gateway, ""),
		http.StatusConflict, apperror.CodeDeploymentRestoreConflict)
}

func TestAgentProxyDeployment_RestoreShipsTheStoredSnapshot(t *testing.T) {
	env := setupAgentDeployEnv(t)
	first := env.deploy(t, env.gateway)
	stored := env.storedContent(t, first)

	// Change the Agent proxy, then deploy the change — the first deployment is
	// now ARCHIVED.
	updated := strings.Replace(fullAgentProxyBody(env.proxy), `"displayName": "Weather Agent"`, `"displayName": "Renamed"`, 1)
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPut, agentProxyBase+"/"+env.proxy, updated), http.StatusOK)
	second := env.deploy(t, env.gateway)
	if bytes.Equal(env.storedContent(t, second), stored) {
		t.Fatal("a deploy from current did not pick up the update")
	}

	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost,
		env.deploymentsPath()+"/"+first+"/restore?gatewayId="+env.gateway, ""), http.StatusAccepted)

	if got := env.storedContent(t, first); !bytes.Equal(got, stored) {
		t.Fatalf("restore changed the stored snapshot:\n got: %s\nwant: %s", got, stored)
	}
	var current string
	if err := env.db.QueryRow(`SELECT deployment_uuid FROM deployment_status WHERE gateway_uuid = ?`, env.gatewayUUID).Scan(&current); err != nil {
		t.Fatalf("read current deployment: %v", err)
	}
	if current != first {
		t.Fatalf("current deployment = %s, want the restored %s", current, first)
	}
	if n := env.countRows(t, `SELECT COUNT(*) FROM deployments WHERE gateway_uuid = ?`, env.gatewayUUID); n != 2 {
		t.Fatalf("restore created a deployment record: %d rows, want 2", n)
	}
}

func TestAgentProxyDeployment_DeleteOnlyAnUndeployedRecord(t *testing.T) {
	env := setupAgentDeployEnv(t)
	deploymentID := env.deploy(t, env.gateway)
	itemPath := env.deploymentsPath() + "/" + deploymentID

	for _, tc := range []struct {
		status string
		code   string
	}{
		{"DEPLOYING", apperror.CodeDeploymentActive},
		{"DEPLOYED", apperror.CodeDeploymentActive},
		{"UNDEPLOYING", apperror.CodeAgentProxyDeploymentNotUndeployed},
		{"FAILED", apperror.CodeAgentProxyDeploymentNotUndeployed},
	} {
		t.Run(tc.status, func(t *testing.T) {
			env.ack(t, deploymentID, tc.status, "")
			assertAgentProxyError(t, callAgentProxy(t, env.handler, http.MethodDelete, itemPath, ""),
				http.StatusConflict, tc.code)
		})
	}

	env.ack(t, deploymentID, "UNDEPLOYED", "")
	rec := callAgentProxy(t, env.handler, http.MethodDelete, itemPath, "")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete undeployed = %d with %d body bytes, want bodyless 204", rec.Code, rec.Body.Len())
	}
	assertAgentProxyError(t, callAgentProxy(t, env.handler, http.MethodGet, itemPath, ""),
		http.StatusNotFound, apperror.CodeDeploymentNotFound)
}

func TestAgentProxyDeployment_ArchivedRecordIsNotDeletable(t *testing.T) {
	env := setupAgentDeployEnv(t)
	archived := env.deploy(t, env.gateway)
	env.deploy(t, env.gateway) // supersedes the first, which becomes ARCHIVED

	got := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet,
		env.deploymentsPath()+"/"+archived, ""), http.StatusOK)
	if got["status"] != "ARCHIVED" {
		t.Fatalf("status = %v, want ARCHIVED", got["status"])
	}
	assertAgentProxyError(t, callAgentProxy(t, env.handler, http.MethodDelete, env.deploymentsPath()+"/"+archived, ""),
		http.StatusConflict, apperror.CodeAgentProxyDeploymentNotUndeployed)

	list := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet,
		env.deploymentsPath()+"?status=ARCHIVED", ""), http.StatusOK)
	if list["count"].(float64) != 1 {
		t.Fatalf("ARCHIVED filter count = %v, want 1", list["count"])
	}
}

func TestAgentProxyDeployment_StatusResourceReportsTheAckResult(t *testing.T) {
	env := setupAgentDeployEnv(t)
	deploymentID := env.deploy(t, env.gateway)
	env.ack(t, deploymentID, "FAILED", "validation failed")

	got := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet,
		env.deploymentsPath()+"/"+deploymentID, ""), http.StatusOK)
	if got["status"] != "FAILED" || got["statusReason"] != "validation failed" {
		t.Fatalf("GET = %v, want FAILED with its statusReason", got)
	}

	list := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet,
		env.deploymentsPath()+"?status=FAILED", ""), http.StatusOK)
	if list["count"].(float64) != 1 {
		t.Fatalf("FAILED filter count = %v, want 1", list["count"])
	}
	assertAgentProxyError(t, callAgentProxy(t, env.handler, http.MethodGet, env.deploymentsPath()+"?status=BOGUS", ""),
		http.StatusBadRequest, apperror.CodeDeploymentInvalidStatus)
}

func TestAgentProxyDeployment_RetentionPrunesInTheSameTransaction(t *testing.T) {
	env := setupAgentDeployEnv(t)
	hardLimit := agentDeployConfig().Deployments.MaxPerAPIGateway + constants.DeploymentLimitBuffer

	var artifactUUID string
	if err := env.db.QueryRow(`SELECT uuid FROM agent_proxies WHERE handle = ? AND organization_uuid = ?`,
		env.proxy, agentProxyOrg).Scan(&artifactUUID); err != nil {
		t.Fatalf("resolve artifact: %v", err)
	}
	// Fill the gateway to its hard limit with ARCHIVED records, oldest first.
	base := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < hardLimit; i++ {
		if _, err := env.db.Exec(`INSERT INTO deployments (uuid, display_name, artifact_uuid, organization_uuid, gateway_uuid, content, created_at)
			VALUES (?, 'old', ?, ?, ?, 'x', ?)`,
			fmt.Sprintf("old-%03d", i), artifactUUID, agentProxyOrg, env.gatewayUUID, base.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatalf("seed archived deployment %d: %v", i, err)
		}
	}

	env.deploy(t, env.gateway)

	if n := env.countRows(t, `SELECT COUNT(*) FROM deployments WHERE gateway_uuid = ?`, env.gatewayUUID); n != hardLimit-5+1 {
		t.Fatalf("deployments after prune = %d, want %d", n, hardLimit-5+1)
	}
	if n := env.countRows(t, `SELECT COUNT(*) FROM deployments WHERE uuid IN ('old-000','old-001','old-002','old-003','old-004')`); n != 0 {
		t.Fatalf("%d of the oldest ARCHIVED records survived the prune", n)
	}
}

func TestAgentProxyDeployment_RejectionContract(t *testing.T) {
	env := setupAgentDeployEnv(t)
	seedAgentGateway(t, env.db, agentProxyOtherOrg, "foreign-gw", "1.2.0")
	decodeAgentProxyJSON(t, callAgentProxyAs(t, env.handler, agentProxyOtherOrg, agentProxyActor,
		http.MethodPost, agentProxyBase, minimalAgentProxyBody("foreign-agent", "Foreign")), http.StatusCreated)

	for _, tc := range []struct {
		name       string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"missing name", env.deploymentsPath(), `{"base":"current","gatewayId":"ai-gw"}`,
			http.StatusBadRequest, apperror.CodeAgentProxyDeploymentValidationFailed},
		{"missing base", env.deploymentsPath(), `{"name":"d","gatewayId":"ai-gw"}`,
			http.StatusBadRequest, apperror.CodeAgentProxyDeploymentValidationFailed},
		{"deployment id as base", env.deploymentsPath(), `{"name":"d","base":"3fa85f64-5717-4562-b3fc-2c963f66afa6","gatewayId":"ai-gw"}`,
			http.StatusBadRequest, apperror.CodeAgentProxyDeploymentValidationFailed},
		{"build base without buildId", env.deploymentsPath(), `{"name":"d","base":"build","gatewayId":"ai-gw"}`,
			http.StatusBadRequest, apperror.CodeAgentProxyDeploymentValidationFailed},
		{"missing gatewayId", env.deploymentsPath(), `{"name":"d","base":"current"}`,
			http.StatusBadRequest, apperror.CodeAgentProxyDeploymentValidationFailed},
		{"malformed body", env.deploymentsPath(), `{"name":`,
			http.StatusBadRequest, apperror.CodeCommonValidationFailed},
		{"unknown build", env.deploymentsPath(), `{"name":"d","base":"build","buildId":"nope","gatewayId":"ai-gw"}`,
			http.StatusNotFound, apperror.CodeBuildNotFound},
		{"unknown gateway", env.deploymentsPath(), deployBody("d", "no-such-gw"),
			http.StatusNotFound, apperror.CodeGatewayNotFound},
		{"another org's gateway", env.deploymentsPath(), deployBody("d", "foreign-gw"),
			http.StatusNotFound, apperror.CodeGatewayNotFound},
		{"unknown Agent proxy", agentProxyBase + "/no-such-agent/deployments", deployBody("d", "ai-gw"),
			http.StatusNotFound, apperror.CodeAgentProxyNotFound},
		{"another org's Agent proxy", agentProxyBase + "/foreign-agent/deployments", deployBody("d", "ai-gw"),
			http.StatusNotFound, apperror.CodeAgentProxyNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertAgentProxyError(t, callAgentProxy(t, env.handler, http.MethodPost, tc.path, tc.body), tc.wantStatus, tc.wantCode)
		})
	}

	// Nothing is stored on a rejected deploy: no record, no status, no association.
	if n := env.countRows(t, `SELECT COUNT(*) FROM deployments`); n != 0 {
		t.Fatalf("%d deployments rows after rejected deploys, want 0", n)
	}
	if n := env.countRows(t, `SELECT COUNT(*) FROM deployment_status`); n != 0 {
		t.Fatalf("%d deployment_status rows after rejected deploys, want 0", n)
	}
	if n := env.countRows(t, `SELECT COUNT(*) FROM artifact_gateway_mappings`); n != 0 {
		t.Fatalf("%d gateway associations after rejected deploys, want 0", n)
	}
}

func TestAgentProxyDeployment_ActionsValidateTheBoundGateway(t *testing.T) {
	env := setupAgentDeployEnv(t)
	seedAgentGateway(t, env.db, agentProxyOrg, "other-gw", "1.2.0")
	deploymentID := env.deploy(t, env.gateway)
	itemPath := env.deploymentsPath() + "/" + deploymentID

	for _, action := range []string{"undeploy", "restore"} {
		t.Run(action, func(t *testing.T) {
			assertAgentProxyError(t, callAgentProxy(t, env.handler, http.MethodPost, itemPath+"/"+action, ""),
				http.StatusBadRequest, apperror.CodeCommonValidationFailed)
			assertAgentProxyError(t, callAgentProxy(t, env.handler, http.MethodPost, itemPath+"/"+action+"?gatewayId=other-gw", ""),
				http.StatusBadRequest, apperror.CodeDeploymentGatewayMismatch)
			assertAgentProxyError(t, callAgentProxy(t, env.handler, http.MethodPost, itemPath+"/"+action+"?gatewayId=no-such-gw", ""),
				http.StatusNotFound, apperror.CodeGatewayNotFound)
			assertAgentProxyError(t, callAgentProxy(t, env.handler, http.MethodPost,
				env.deploymentsPath()+"/3fa85f64-5717-4562-b3fc-2c963f66afa6/"+action+"?gatewayId="+env.gateway, ""),
				http.StatusNotFound, apperror.CodeDeploymentNotFound)
		})
	}
}

func TestAgentProxyDeployment_AnotherOrganizationCannotReachTheDeployment(t *testing.T) {
	env := setupAgentDeployEnv(t)
	deploymentID := env.deploy(t, env.gateway)
	itemPath := env.deploymentsPath() + "/" + deploymentID

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, itemPath},
		{http.MethodGet, env.deploymentsPath()},
		{http.MethodDelete, itemPath},
		{http.MethodPost, itemPath + "/undeploy?gatewayId=" + env.gateway},
	} {
		rec := callAgentProxyAs(t, env.handler, agentProxyOtherOrg, agentProxyActor, tc.method, tc.path, "")
		assertAgentProxyError(t, rec, http.StatusNotFound, apperror.CodeAgentProxyNotFound)
	}
}

func TestAgentProxyDeployment_RejectsUnsupportedMediaType(t *testing.T) {
	env := setupAgentDeployEnv(t)
	req := httptest.NewRequest(http.MethodPost, env.deploymentsPath(), strings.NewReader(deployBody("d", env.gateway)))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-Test-Org", agentProxyOrg)
	req.Header.Set("X-Test-User", agentProxyActor)
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	assertAgentProxyError(t, rec, http.StatusUnsupportedMediaType, apperror.CodeCommonUnsupportedMediaType)
	if n := env.countRows(t, `SELECT COUNT(*) FROM deployments`); n != 0 {
		t.Fatalf("%d deployments rows after a 415, want 0", n)
	}
}

// A handle is unique only within a kind. An artifact of another kind with the
// same handle — here an MCP proxy, whose table a cross-kind lookup reaches
// first — must not hide the Agent proxy from any deployment operation.
func TestAgentProxyDeployment_HandleSharedWithAnotherKindStillResolves(t *testing.T) {
	env := setupAgentDeployEnv(t)
	const mcpUUID = "mcp-sharing-the-handle"
	if _, err := env.db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, 'Mcp', ?)`,
		mcpUUID, agentProxyOrg); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	if _, err := env.db.Exec(`INSERT INTO mcp_proxies (uuid, handle, display_name, configuration, organization_uuid)
		VALUES (?, ?, 'Same Handle', '{}', ?)`, mcpUUID, env.proxy, agentProxyOrg); err != nil {
		t.Fatalf("seed MCP proxy: %v", err)
	}

	deploymentID := env.deploy(t, env.gateway)
	itemPath := env.deploymentsPath() + "/" + deploymentID
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet, itemPath, ""), http.StatusOK)
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet, env.deploymentsPath(), ""), http.StatusOK)
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost, itemPath+"/undeploy?gatewayId="+env.gateway, ""), http.StatusAccepted)
	env.ack(t, deploymentID, "UNDEPLOYED", "")
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost, itemPath+"/restore?gatewayId="+env.gateway, ""), http.StatusAccepted)
	env.ack(t, deploymentID, "UNDEPLOYED", "")
	if rec := callAgentProxy(t, env.handler, http.MethodDelete, itemPath, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	if n := env.countRows(t, `SELECT COUNT(*) FROM deployments WHERE artifact_uuid = ?`, mcpUUID); n != 0 {
		t.Fatalf("%d deployments recorded against the MCP proxy, want 0", n)
	}
}

// A gateway-origin Agent proxy is read-only in the control plane, and so is its
// deployment lifecycle — deleting an undeployed record included.
func TestAgentProxyDeployment_GatewayOriginIsReadOnly(t *testing.T) {
	env := setupAgentDeployEnv(t)
	deploymentID := env.deploy(t, env.gateway)
	itemPath := env.deploymentsPath() + "/" + deploymentID
	env.ack(t, deploymentID, "UNDEPLOYED", "")
	if _, err := env.db.Exec(`UPDATE agent_proxies SET origin = ? WHERE handle = ?`, constants.OriginDP, env.proxy); err != nil {
		t.Fatalf("mark gateway origin: %v", err)
	}

	for _, tc := range []struct{ name, method, path, body string }{
		{"deploy", http.MethodPost, env.deploymentsPath(), deployBody("d", env.gateway)},
		{"undeploy", http.MethodPost, itemPath + "/undeploy?gatewayId=" + env.gateway, ""},
		{"restore", http.MethodPost, itemPath + "/restore?gatewayId=" + env.gateway, ""},
		{"delete", http.MethodDelete, itemPath, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertAgentProxyError(t, callAgentProxy(t, env.handler, tc.method, tc.path, tc.body),
				http.StatusForbidden, apperror.CodeArtifactReadOnly)
		})
	}

	// Reads stay available, and the record survived the refused delete.
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet, itemPath, ""), http.StatusOK)
}

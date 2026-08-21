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

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// agentArtifactYAML is a deployable Agent, with an upstream credential so every
// response body can be checked for it.
const agentArtifactYAML = `
apiVersion: gateway.api-platform.wso2.com/v1
kind: Agent
metadata:
  name: weather-agent-v1-0
spec:
  displayName: Weather Agent
  version: v1.0
  context: /weather
  upstream:
    url: https://weather.internal
    auth:
      type: api-key
      header: x-api-key
      value: super-secret-upstream-key
  a2a:
    protocolVersion: "1.0"
    operationConfigs:
      transports:
        - protocolBinding: JSONRPC
          pathPrefix: /rpc
    agentCard:
      public:
        mode: managed
        content: {
          "name": "Weather Agent",
          "description": "Provides weather information",
          "version": "1.0.0",
          "supportedInterfaces": [
            {"protocolBinding": "JSONRPC", "protocolVersion": "1.0", "url": "https://agents.example.com/weather/rpc"}
          ],
          "capabilities": {"streaming": true},
          "defaultInputModes": ["text/plain"],
          "defaultOutputModes": ["text/plain"],
          "skills": []
        }
`

func postAgent(t *testing.T, server *APIServer, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/agents", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/yaml")
	rec := httptest.NewRecorder()
	server.CreateAgent(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

func TestAgentHandler_CreateAndGet(t *testing.T) {
	server := createTestAPIServer()

	rec := postAgent(t, server, agentArtifactYAML)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	created := decodeBody(t, rec)
	assert.Equal(t, "Agent", created["kind"])

	status, ok := created["status"].(map[string]any)
	require.True(t, ok, "response carries no server-managed status block")
	assert.Equal(t, "weather-agent-v1-0", status["id"])
	assert.Equal(t, "deployed", status["state"])

	// Read it back through the handler.
	req := httptest.NewRequest(http.MethodGet, "/agents/weather-agent-v1-0", nil)
	getRec := httptest.NewRecorder()
	server.GetAgentById(getRec, req, "weather-agent-v1-0")
	require.Equal(t, http.StatusOK, getRec.Code, "body: %s", getRec.Body.String())

	fetched := decodeBody(t, getRec)
	spec, ok := fetched["spec"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Weather Agent", spec["displayName"])
	assert.Equal(t, "/weather", spec["context"])

	// The a2a subtree survives the response round-trip.
	a2a, ok := spec["a2a"].(map[string]any)
	require.True(t, ok, "spec.a2a missing from the response")
	card, ok := a2a["agentCard"].(map[string]any)
	require.True(t, ok)
	public, ok := card["public"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "managed", public["mode"])
	content, ok := public["content"].(map[string]any)
	require.True(t, ok, "the Agent Card document was dropped from the response")
	assert.Equal(t, "Weather Agent", content["name"])
}

// TestAgentHandler_RedactsUpstreamCredential is the whole point of routing Agent
// responses through a rematerialize step: auth.value is write-only, so it must
// not appear in any response body — and the stored artifact must still have it.
func TestAgentHandler_RedactsUpstreamCredential(t *testing.T) {
	server := createTestAPIServer()

	rec := postAgent(t, server, agentArtifactYAML)
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.NotContains(t, rec.Body.String(), "super-secret-upstream-key", "create response leaked the upstream credential")

	getReq := httptest.NewRequest(http.MethodGet, "/agents/weather-agent-v1-0", nil)
	getRec := httptest.NewRecorder()
	server.GetAgentById(getRec, getReq, "weather-agent-v1-0")
	require.Equal(t, http.StatusOK, getRec.Code)
	assert.NotContains(t, getRec.Body.String(), "super-secret-upstream-key", "get response leaked the upstream credential")

	listReq := httptest.NewRequest(http.MethodGet, "/agents", nil)
	listRec := httptest.NewRecorder()
	server.ListAgents(listRec, listReq, api.ListAgentsParams{})
	require.Equal(t, http.StatusOK, listRec.Code)
	assert.NotContains(t, listRec.Body.String(), "super-secret-upstream-key", "list response leaked the upstream credential")

	// The auth block itself is still advertised — only the value is withheld.
	body := decodeBody(t, getRec)
	spec := body["spec"].(map[string]any)
	upstream := spec["upstream"].(map[string]any)
	auth, ok := upstream["auth"].(map[string]any)
	require.True(t, ok, "the auth block should still be present, just without the value")
	assert.Equal(t, "api-key", auth["type"])
	_, hasValue := auth["value"]
	assert.False(t, hasValue, "auth.value must be absent, not blank")

	// And the credential is intact in storage, so the upstream keeps working.
	stored, err := server.db.GetConfigByKindAndHandle(models.KindAgent, "weather-agent-v1-0")
	require.NoError(t, err)
	source, ok := stored.SourceConfiguration.(api.AgentConfiguration)
	require.True(t, ok)
	require.NotNil(t, source.Spec.Upstream.Auth)
	require.NotNil(t, source.Spec.Upstream.Auth.Value)
	assert.Equal(t, "super-secret-upstream-key", *source.Spec.Upstream.Auth.Value)
}

func TestAgentHandler_List(t *testing.T) {
	server := createTestAPIServer()

	require.Equal(t, http.StatusCreated, postAgent(t, server, agentArtifactYAML).Code)

	req := httptest.NewRequest(http.MethodGet, "/agents", nil)
	rec := httptest.NewRecorder()
	server.ListAgents(rec, req, api.ListAgentsParams{})
	require.Equal(t, http.StatusOK, rec.Code)

	body := decodeBody(t, rec)
	assert.Equal(t, "success", body["status"])
	assert.EqualValues(t, 1, body["count"])
	agents, ok := body["agents"].([]any)
	require.True(t, ok, `the list envelope key must be "agents"`)
	assert.Len(t, agents, 1)
}

func TestAgentHandler_UpdateAndDelete(t *testing.T) {
	server := createTestAPIServer()
	require.Equal(t, http.StatusCreated, postAgent(t, server, agentArtifactYAML).Code)

	updated := strings.Replace(agentArtifactYAML, "displayName: Weather Agent", "displayName: Renamed Agent", 1)
	putReq := httptest.NewRequest(http.MethodPut, "/agents/weather-agent-v1-0", strings.NewReader(updated))
	putReq.Header.Set("Content-Type", "application/yaml")
	putRec := httptest.NewRecorder()
	server.UpdateAgent(putRec, putReq, "weather-agent-v1-0")
	require.Equal(t, http.StatusOK, putRec.Code, "body: %s", putRec.Body.String())

	body := decodeBody(t, putRec)
	spec := body["spec"].(map[string]any)
	assert.Equal(t, "Renamed Agent", spec["displayName"])

	delReq := httptest.NewRequest(http.MethodDelete, "/agents/weather-agent-v1-0", nil)
	delRec := httptest.NewRecorder()
	server.DeleteAgent(delRec, delReq, "weather-agent-v1-0")
	require.Equal(t, http.StatusOK, delRec.Code, "body: %s", delRec.Body.String())

	getRec := httptest.NewRecorder()
	server.GetAgentById(getRec, httptest.NewRequest(http.MethodGet, "/agents/weather-agent-v1-0", nil), "weather-agent-v1-0")
	assert.Equal(t, http.StatusNotFound, getRec.Code)
}

func TestAgentHandler_ErrorStatusCodes(t *testing.T) {
	t.Run("unparseable body is 400", func(t *testing.T) {
		server := createTestAPIServer()
		rec := postAgent(t, server, "spec: [not: valid yaml")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("foreign kind in the payload is 400", func(t *testing.T) {
		server := createTestAPIServer()
		rec := postAgent(t, server, strings.Replace(agentArtifactYAML, "kind: Agent", "kind: Mcp", 1))
		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "kind mismatch")
	})

	t.Run("validation failure is 400 with field detail", func(t *testing.T) {
		server := createTestAPIServer()
		rec := postAgent(t, server, strings.Replace(agentArtifactYAML, "version: v1.0", "version: nonsense", 1))
		require.Equal(t, http.StatusBadRequest, rec.Code)

		var body api.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "Configuration validation failed", body.Message)
		require.NotNil(t, body.Errors)
		require.NotEmpty(t, *body.Errors)
		fields := make([]string, 0, len(*body.Errors))
		for _, e := range *body.Errors {
			require.NotNil(t, e.Field)
			fields = append(fields, *e.Field)
		}
		assert.Contains(t, fields, "spec.version")
	})

	t.Run("duplicate handle is 409", func(t *testing.T) {
		server := createTestAPIServer()
		require.Equal(t, http.StatusCreated, postAgent(t, server, agentArtifactYAML).Code)

		duplicate := strings.Replace(agentArtifactYAML, "displayName: Weather Agent", "displayName: Other Agent", 1)
		duplicate = strings.Replace(duplicate, "version: v1.0", "version: v2.0", 1)
		rec := postAgent(t, server, duplicate)
		assert.Equal(t, http.StatusConflict, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("handle mismatch on update is 400", func(t *testing.T) {
		server := createTestAPIServer()
		require.Equal(t, http.StatusCreated, postAgent(t, server, agentArtifactYAML).Code)

		req := httptest.NewRequest(http.MethodPut, "/agents/weather-agent-v1-0", strings.NewReader(agentArtifactYAML))
		req.Header.Set("Content-Type", "application/yaml")
		rec := httptest.NewRecorder()
		server.UpdateAgent(rec, req, "some-other-handle")
		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "handle mismatch")
	})

	t.Run("absent agent is 404 on get, update and delete", func(t *testing.T) {
		server := createTestAPIServer()

		getRec := httptest.NewRecorder()
		server.GetAgentById(getRec, httptest.NewRequest(http.MethodGet, "/agents/absent", nil), "absent")
		assert.Equal(t, http.StatusNotFound, getRec.Code)

		putReq := httptest.NewRequest(http.MethodPut, "/agents/absent", strings.NewReader(
			strings.Replace(agentArtifactYAML, "name: weather-agent-v1-0", "name: absent", 1)))
		putReq.Header.Set("Content-Type", "application/yaml")
		putRec := httptest.NewRecorder()
		server.UpdateAgent(putRec, putReq, "absent")
		assert.Equal(t, http.StatusNotFound, putRec.Code)

		delRec := httptest.NewRecorder()
		server.DeleteAgent(delRec, httptest.NewRequest(http.MethodDelete, "/agents/absent", nil), "absent")
		assert.Equal(t, http.StatusNotFound, delRec.Code)
	})
}

// TestAgentHandler_SearchDeploymentsEnvelope covers the generic
// /deployments/{kind} path, which dispatches on the stored artifact's own kind
// rather than on the requested one.
func TestAgentHandler_SearchDeploymentsEnvelope(t *testing.T) {
	server := createTestAPIServer()
	require.Equal(t, http.StatusCreated, postAgent(t, server, agentArtifactYAML).Code)

	// SearchDeployments reads the in-memory store, which the event listener
	// populates in a running gateway; add it directly here.
	stored, err := server.db.GetConfigByKindAndHandle(models.KindAgent, "weather-agent-v1-0")
	require.NoError(t, err)
	require.NoError(t, server.store.Add(stored))

	req := httptest.NewRequest(http.MethodGet, "/deployments/Agent", nil)
	rec := httptest.NewRecorder()
	server.SearchDeployments(rec, req, models.KindAgent)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	body := decodeBody(t, rec)
	agents, ok := body["agents"].([]any)
	require.True(t, ok, `SearchDeployments must use the "agents" envelope key for Agents`)
	require.Len(t, agents, 1)

	// The generic list path redacts through the same rematerialize step.
	assert.NotContains(t, rec.Body.String(), "super-secret-upstream-key",
		"the generic deployments listing leaked the upstream credential")
}

// TestAgentHandler_ConfigDumpRedactsSecrets covers the second dump surface named
// in the acceptance criteria: /config_dump serves the rendered configuration, so
// a resolved secret would appear verbatim if it were not redacted from it.
func TestAgentHandler_ConfigDumpRedactsSecrets(t *testing.T) {
	server := createTestAPIServer()
	require.Equal(t, http.StatusCreated, postAgent(t, server, agentArtifactYAML).Code)

	stored, err := server.db.GetConfigByKindAndHandle(models.KindAgent, "weather-agent-v1-0")
	require.NoError(t, err)
	// Stand in for what the template engine records when a {{ secret }} resolves.
	stored.SensitiveValues = []string{"super-secret-upstream-key"}
	require.NoError(t, server.store.Add(stored))

	req := httptest.NewRequest(http.MethodGet, "/config_dump", nil)
	rec := httptest.NewRecorder()
	server.GetConfigDump(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	assert.Contains(t, rec.Body.String(), "weather-agent-v1-0", "the Agent should appear in the config dump")
	assert.NotContains(t, rec.Body.String(), "super-secret-upstream-key",
		"the management API config dump leaked a tracked secret value")

	// The admin server serves the same payload through ConfigDumpJSON, which is
	// why redaction lives there rather than in the management handler: both
	// surfaces have to strip the same values.
	adminBody, err := server.ConfigDumpJSON(server.logger)
	require.NoError(t, err)
	assert.Contains(t, string(adminBody), "weather-agent-v1-0")
	assert.NotContains(t, string(adminBody), "super-secret-upstream-key",
		"the admin config dump leaked a tracked secret value")
}

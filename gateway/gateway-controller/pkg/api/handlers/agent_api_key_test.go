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

	"github.com/wso2/api-platform/common/apikey"
	"github.com/wso2/api-platform/common/eventhub"
	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

const (
	agentKeyHandle = "weather-agent-v1-0"
	agentKeyName   = "weather-agent-key"
	// An externally supplied value makes the created key `source: external`,
	// which is what the update endpoint requires; regenerate preserves it.
	agentExternalKeyValue = "external-agent-key-1234567890123456789012345678"
)

// seedAgentForAPIKeyTests deploys the Agent every API key test keys against and
// returns its stored config, whose UUID is what the keys must attach to.
func seedAgentForAPIKeyTests(t *testing.T, server *APIServer) *models.StoredConfig {
	t.Helper()

	require.Equal(t, http.StatusCreated, postAgent(t, server, agentArtifactYAML).Code)

	cfg, err := server.db.GetConfigByKindAndHandle(models.KindAgent, agentKeyHandle)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	return cfg
}

// adminRequest builds an authenticated management-API request for the API key
// endpoints. Every one of them requires an auth context; without it the handler
// answers 401 before it ever reaches the service.
func adminRequest(t *testing.T, method, path string, body []byte, correlationID string) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()

	w, r := createTestContextWithHeader(method, path, body, map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	if correlationID != "" {
		r = withCorrelationID(r, correlationID)
	}
	return w, r
}

func agentAPIKeyCreationBody(t *testing.T, name, keyValue string) []byte {
	t.Helper()

	request := api.APIKeyCreationRequest{Name: &name}
	if keyValue != "" {
		request.ApiKey = &keyValue
	}
	body, err := json.Marshal(request)
	require.NoError(t, err)
	return body
}

// TestAgentAPIKeys_Lifecycle walks the whole documented flow — create, list,
// regenerate, update, revoke — against an Agent, and checks that each step
// publishes the API key event carrying the shared entity ID form.
func TestAgentAPIKeys_Lifecycle(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedAgentForAPIKeyTests(t, server)

	// Attached after the Agent is created so only API key events land here.
	hub := &mockEventHub{}
	attachTestEventHub(server, hub, "test-gateway")
	mockDB := server.db.(*MockStorage)

	// Create
	w, r := adminRequest(t, http.MethodPost, "/agents/"+agentKeyHandle+"/api-keys",
		agentAPIKeyCreationBody(t, agentKeyName, agentExternalKeyValue), "corr-create")
	server.CreateAgentAPIKey(w, r, agentKeyHandle)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	created, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, agentKeyName)
	require.NoError(t, err)
	assert.Equal(t, cfg.UUID, created.ArtifactUUID,
		"the key attached to a different artifact than the Agent it was created for")

	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, eventhub.EventTypeAPIKey, hub.publishedEvents[0].event.EventType)
	assert.Equal(t, "CREATE", hub.publishedEvents[0].event.Action)
	assert.Equal(t, apikey.BuildAPIKeyEntityID(cfg.UUID, created.UUID), hub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-create", hub.publishedEvents[0].event.EventID)

	// List
	w, r = adminRequest(t, http.MethodGet, "/agents/"+agentKeyHandle+"/api-keys", nil, "")
	server.ListAgentAPIKeys(w, r, agentKeyHandle)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var list api.APIKeyListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.NotNil(t, list.ApiKeys)
	require.Len(t, *list.ApiKeys, 1)
	assert.Equal(t, agentKeyName, (*list.ApiKeys)[0].Name)
	assert.Equal(t, agentKeyHandle, (*list.ApiKeys)[0].ApiId)

	// Regenerate
	w, r = adminRequest(t, http.MethodPost, "/agents/"+agentKeyHandle+"/api-keys/"+agentKeyName+"/regenerate",
		[]byte(`{}`), "corr-regenerate")
	server.RegenerateAgentAPIKey(w, r, agentKeyHandle, agentKeyName)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	regenerated, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, agentKeyName)
	require.NoError(t, err)
	assert.NotEqual(t, created.APIKey, regenerated.APIKey, "regenerate left the stored key value unchanged")

	require.Len(t, hub.publishedEvents, 2)
	assert.Equal(t, "UPDATE", hub.publishedEvents[1].event.Action)
	assert.Equal(t, apikey.BuildAPIKeyEntityID(cfg.UUID, regenerated.UUID), hub.publishedEvents[1].event.EntityID)

	// Update — only externally sourced keys accept a caller-supplied value.
	const replacementKeyValue = "external-agent-key-9876543210987654321098765432"
	w, r = adminRequest(t, http.MethodPut, "/agents/"+agentKeyHandle+"/api-keys/"+agentKeyName,
		agentAPIKeyCreationBody(t, agentKeyName, replacementKeyValue), "corr-update")
	server.UpdateAgentAPIKey(w, r, agentKeyHandle, agentKeyName)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	updated, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, agentKeyName)
	require.NoError(t, err)
	assert.NotEqual(t, regenerated.APIKey, updated.APIKey, "update left the stored key value unchanged")

	require.Len(t, hub.publishedEvents, 3)
	assert.Equal(t, "UPDATE", hub.publishedEvents[2].event.Action)
	assert.Equal(t, apikey.BuildAPIKeyEntityID(cfg.UUID, updated.UUID), hub.publishedEvents[2].event.EntityID)

	// Revoke
	w, r = adminRequest(t, http.MethodDelete, "/agents/"+agentKeyHandle+"/api-keys/"+agentKeyName, nil, "corr-revoke")
	server.RevokeAgentAPIKey(w, r, agentKeyHandle, agentKeyName)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	_, err = mockDB.GetAPIKeysByAPIAndName(cfg.UUID, agentKeyName)
	require.Error(t, err, "the revoked key is still resolvable")

	require.Len(t, hub.publishedEvents, 4)
	assert.Equal(t, "DELETE", hub.publishedEvents[3].event.Action)
	assert.Equal(t, apikey.BuildAPIKeyEntityID(cfg.UUID, updated.UUID), hub.publishedEvents[3].event.EntityID)
	assert.Equal(t, "corr-revoke", hub.publishedEvents[3].event.EventID)
}

// TestAgentAPIKeys_ResolveTheAgentNotARestAPI is the regression test for the
// one Agent-specific line in each of the five handlers. APIKeyService defaults
// an empty Kind to KindRestApi, so a forgotten `Kind: models.KindAgent` does not
// fail — it silently resolves the handle against a RestAPI instead. Seeding both
// kinds under the same handle is what makes that substitution observable.
func TestAgentAPIKeys_ResolveTheAgentNotARestAPI(t *testing.T) {
	server := createTestAPIServer()
	agentCfg := seedAgentForAPIKeyTests(t, server)
	restCfg := seedAPIForAPIKeyHandlerTests(t, server, agentKeyHandle)
	require.NotEqual(t, agentCfg.UUID, restCfg.UUID)

	mockDB := server.db.(*MockStorage)

	assertAgentKey := func(t *testing.T, step string) *models.APIKey {
		t.Helper()
		key, err := mockDB.GetAPIKeysByAPIAndName(agentCfg.UUID, agentKeyName)
		require.NoErrorf(t, err, "%s did not resolve the Agent — the key is not attached to it", step)
		_, restErr := mockDB.GetAPIKeysByAPIAndName(restCfg.UUID, agentKeyName)
		require.Errorf(t, restErr, "%s resolved the RestAPI sharing the handle", step)
		return key
	}

	w, r := adminRequest(t, http.MethodPost, "/agents/"+agentKeyHandle+"/api-keys",
		agentAPIKeyCreationBody(t, agentKeyName, agentExternalKeyValue), "")
	server.CreateAgentAPIKey(w, r, agentKeyHandle)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	assertAgentKey(t, "CreateAgentAPIKey")

	// List reads through the same resolution, so a RestAPI lookup would return
	// that artifact's (empty) key set rather than the Agent's.
	w, r = adminRequest(t, http.MethodGet, "/agents/"+agentKeyHandle+"/api-keys", nil, "")
	server.ListAgentAPIKeys(w, r, agentKeyHandle)
	require.Equal(t, http.StatusOK, w.Code)
	var list api.APIKeyListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.NotNil(t, list.ApiKeys)
	require.Len(t, *list.ApiKeys, 1, "ListAgentAPIKeys resolved the RestAPI sharing the handle")

	w, r = adminRequest(t, http.MethodPost, "/agents/"+agentKeyHandle+"/api-keys/"+agentKeyName+"/regenerate",
		[]byte(`{}`), "")
	server.RegenerateAgentAPIKey(w, r, agentKeyHandle, agentKeyName)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assertAgentKey(t, "RegenerateAgentAPIKey")

	w, r = adminRequest(t, http.MethodPut, "/agents/"+agentKeyHandle+"/api-keys/"+agentKeyName,
		agentAPIKeyCreationBody(t, agentKeyName, "external-agent-key-5555555555555555555555555555"), "")
	server.UpdateAgentAPIKey(w, r, agentKeyHandle, agentKeyName)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assertAgentKey(t, "UpdateAgentAPIKey")

	w, r = adminRequest(t, http.MethodDelete, "/agents/"+agentKeyHandle+"/api-keys/"+agentKeyName, nil, "")
	server.RevokeAgentAPIKey(w, r, agentKeyHandle, agentKeyName)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	_, err := mockDB.GetAPIKeysByAPIAndName(agentCfg.UUID, agentKeyName)
	require.Error(t, err, "RevokeAgentAPIKey did not revoke the Agent's key")
}

// TestAgentAPIKeys_RequireAuthentication pins the fail-closed behaviour of all
// five handlers: no auth context means 401 before the service is reached.
func TestAgentAPIKeys_RequireAuthentication(t *testing.T) {
	server := createTestAPIServer()
	seedAgentForAPIKeyTests(t, server)

	calls := map[string]func(w http.ResponseWriter, r *http.Request){
		"CreateAgentAPIKey": func(w http.ResponseWriter, r *http.Request) {
			server.CreateAgentAPIKey(w, r, agentKeyHandle)
		},
		"ListAgentAPIKeys": func(w http.ResponseWriter, r *http.Request) {
			server.ListAgentAPIKeys(w, r, agentKeyHandle)
		},
		"RegenerateAgentAPIKey": func(w http.ResponseWriter, r *http.Request) {
			server.RegenerateAgentAPIKey(w, r, agentKeyHandle, agentKeyName)
		},
		"UpdateAgentAPIKey": func(w http.ResponseWriter, r *http.Request) {
			server.UpdateAgentAPIKey(w, r, agentKeyHandle, agentKeyName)
		},
		"RevokeAgentAPIKey": func(w http.ResponseWriter, r *http.Request) {
			server.RevokeAgentAPIKey(w, r, agentKeyHandle, agentKeyName)
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			w, r := createTestContextWithHeader(http.MethodPost, "/agents/"+agentKeyHandle+"/api-keys",
				[]byte(`{"name":"`+agentKeyName+`"}`), map[string]string{"Content-Type": "application/json"})
			call(w, r)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// TestAgentAPIKeys_UnknownAgentIsNotFound checks that a handle no Agent claims
// is a 404 rather than a 500 or a key attached to nothing.
func TestAgentAPIKeys_UnknownAgentIsNotFound(t *testing.T) {
	server := createTestAPIServer()

	w, r := adminRequest(t, http.MethodPost, "/agents/no-such-agent/api-keys",
		agentAPIKeyCreationBody(t, agentKeyName, agentExternalKeyValue), "")
	server.CreateAgentAPIKey(w, r, "no-such-agent")
	assert.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())

	w, r = adminRequest(t, http.MethodGet, "/agents/no-such-agent/api-keys", nil, "")
	server.ListAgentAPIKeys(w, r, "no-such-agent")
	assert.Equal(t, http.StatusNotFound, w.Code, "body: %s", w.Body.String())
}

// TestAgentAPIKeys_SurviveUndeployRedeploy pins the reason the Agent delete path
// is the only one that removes keys. Undeploy keeps the configuration so the
// artifact can be redeployed; revoking keys there would silently break every
// client over what is meant to be a reversible operation.
func TestAgentAPIKeys_SurviveUndeployRedeploy(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedAgentForAPIKeyTests(t, server)
	mockDB := server.db.(*MockStorage)

	w, r := adminRequest(t, http.MethodPost, "/agents/"+agentKeyHandle+"/api-keys",
		agentAPIKeyCreationBody(t, agentKeyName, agentExternalKeyValue), "")
	server.CreateAgentAPIKey(w, r, agentKeyHandle)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	created, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, agentKeyName)
	require.NoError(t, err)

	undeployed := strings.Replace(agentArtifactYAML,
		"spec:\n  displayName: Weather Agent",
		"spec:\n  deploymentState: undeployed\n  displayName: Weather Agent", 1)
	require.NotEqual(t, agentArtifactYAML, undeployed, "the undeploy fixture did not apply")
	putUndeploy(t, server, undeployed)

	afterUndeploy, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, agentKeyName)
	require.NoError(t, err, "undeploying the Agent revoked its API keys")
	assert.Equal(t, created.UUID, afterUndeploy.UUID)

	// Redeploy and confirm the same key is still the one the Agent answers with.
	putUndeploy(t, server, agentArtifactYAML)

	w, r = adminRequest(t, http.MethodGet, "/agents/"+agentKeyHandle+"/api-keys", nil, "")
	server.ListAgentAPIKeys(w, r, agentKeyHandle)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var list api.APIKeyListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.NotNil(t, list.ApiKeys)
	require.Len(t, *list.ApiKeys, 1, "the API key did not survive undeploy -> redeploy")
	assert.Equal(t, agentKeyName, (*list.ApiKeys)[0].Name)
}

// TestAgentAPIKeys_RemovedOnDelete is the counterpart: deleting the Agent is
// terminal, so its keys must not outlive it and be inherited by a later
// artifact reusing the same UUID.
func TestAgentAPIKeys_RemovedOnDelete(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedAgentForAPIKeyTests(t, server)
	mockDB := server.db.(*MockStorage)

	w, r := adminRequest(t, http.MethodPost, "/agents/"+agentKeyHandle+"/api-keys",
		agentAPIKeyCreationBody(t, agentKeyName, agentExternalKeyValue), "")
	server.CreateAgentAPIKey(w, r, agentKeyHandle)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	delRec := httptest.NewRecorder()
	server.DeleteAgent(delRec, httptest.NewRequest(http.MethodDelete, "/agents/"+agentKeyHandle, nil), agentKeyHandle)
	require.Equal(t, http.StatusOK, delRec.Code, "body: %s", delRec.Body.String())

	_, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, agentKeyName)
	assert.Error(t, err, "deleting the Agent left its API keys behind")
}

// putUndeploy applies an Agent artifact through the update handler.
func putUndeploy(t *testing.T, server *APIServer, body string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, "/agents/"+agentKeyHandle, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/yaml")
	rec := httptest.NewRecorder()
	server.UpdateAgent(rec, req, agentKeyHandle)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
}

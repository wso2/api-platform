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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// newModelEndpointTestServer is an API server over a real store with the models
// feature on and the default endpoint provisioned.
func newModelEndpointTestServer(t *testing.T) *APIServer {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := storage.NewStorage(storage.BackendConfig{
		Type: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "gw.db"), GatewayID: "test-gateway",
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	templates, err := utils.NewLLMTemplateLoader(logger).LoadTemplatesFromDirectory("../../../default-llm-provider-templates")
	require.NoError(t, err)
	defs := map[string]models.PolicyDefinition{}
	for name, version := range map[string]string{"api-key-auth": "v1.2.1", "jwt-auth": "v1.0.0", "set-headers": "v1.0.0"} {
		defs[name+"|"+version] = models.PolicyDefinition{Name: name, Version: version}
	}

	store := storage.NewConfigStore()
	hub := &mockEventHub{}
	routerCfg := &config.RouterConfig{ListenerPort: 8080, VHosts: config.VHostsConfig{Main: config.VHostEntry{Default: "*"}}}
	deploymentService := utils.NewAPIDeploymentService(store, db, nil, config.NewAPIValidator(), routerCfg, hub,
		"test-gateway", nil, &http.Client{})
	llm := utils.NewLLMDeploymentService(store, db, nil, nil, templates, deploymentService, routerCfg,
		utils.NewLoadedPolicyVersionResolver(defs), config.NewPolicyValidator(defs, nil))
	llm.SetModelsConfig(config.ModelsConfig{Enabled: true,
		Endpoint: config.ModelEndpointConfig{Provision: true, Context: "/wso2/v1"}})
	require.NoError(t, llm.ProvisionModelEndpoint(logger))

	apiKeyCfg := &config.APIKeyConfig{APIKeysPerUserPerAPI: 10, Algorithm: "sha256", MinKeyLength: 32, MaxKeyLength: 128}
	return &APIServer{
		store:                store,
		db:                   db,
		logger:               logger,
		eventHub:             hub,
		llmDeploymentService: llm,
		deploymentService:    deploymentService,
		apiKeyService:        utils.NewAPIKeyService(store, db, nil, apiKeyCfg, hub, "test-gateway"),
	}
}

func jsonRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reader = strings.NewReader(string(data))
	}
	r := httptest.NewRequest(method, path, reader)
	r.Header.Set("Content-Type", "application/json")
	return withAuthContext(r, commonmodels.AuthContext{UserID: "admin", Authenticated: true, Roles: []string{"admin"}})
}

func endpointRequestBody(handle, context string, spec map[string]any) map[string]any {
	if spec == nil {
		spec = map[string]any{}
	}
	spec["displayName"] = "Endpoint " + handle
	spec["version"] = "v1.0"
	if context != "" {
		spec["context"] = context
	}
	return map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "ModelEndpoint",
		"metadata":   map[string]any{"name": handle},
		"spec":       spec,
	}
}

func TestModelEndpointHandlers_CRUD(t *testing.T) {
	s := newModelEndpointTestServer(t)

	w := httptest.NewRecorder()
	s.CreateModelEndpoint(w, jsonRequest(t, http.MethodPost, "/model-endpoints", endpointRequestBody("partners", "/partners/v1", nil)))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created api.ModelEndpointConfiguration
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, "partners", created.Metadata.Name)
	require.NotNil(t, created.Status)
	require.NotNil(t, created.Status.Id)
	assert.Equal(t, "partners", *created.Status.Id)

	w = httptest.NewRecorder()
	s.GetModelEndpointById(w, jsonRequest(t, http.MethodGet, "/model-endpoints/wso2", nil), "wso2")
	require.Equal(t, http.StatusOK, w.Code)
	var wso2 api.ModelEndpointConfiguration
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wso2))
	assert.Equal(t, "/wso2/v1", *wso2.Spec.Context)

	w = httptest.NewRecorder()
	s.ListModelEndpoints(w, jsonRequest(t, http.MethodGet, "/model-endpoints", nil), api.ListModelEndpointsParams{})
	require.Equal(t, http.StatusOK, w.Code)
	var list struct {
		Count          int                              `json:"count"`
		ModelEndpoints []api.ModelEndpointConfiguration `json:"modelEndpoints"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	assert.Equal(t, 2, list.Count)

	w = httptest.NewRecorder()
	s.GetModelEndpointById(w, jsonRequest(t, http.MethodGet, "/model-endpoints/missing", nil), "missing")
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = httptest.NewRecorder()
	s.UpdateModelEndpoint(w, jsonRequest(t, http.MethodPut, "/model-endpoints/partners",
		endpointRequestBody("other-name", "/partners/v1", nil)), "partners")
	assert.Equal(t, http.StatusBadRequest, w.Code, "metadata.name must match")

	w = httptest.NewRecorder()
	s.DeleteModelEndpoint(w, jsonRequest(t, http.MethodDelete, "/model-endpoints/partners", nil), "partners")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestModelEndpointHandlers_Errors(t *testing.T) {
	s := newModelEndpointTestServer(t)

	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"invalid", endpointRequestBody("p", "", nil), http.StatusBadRequest},
		{"unsupported inbound format", endpointRequestBody("p", "/p", map[string]any{"inboundTemplate": "awsbedrock"}),
			http.StatusBadRequest},
		{"reserved handle", endpointRequestBody("wso2", "", nil), http.StatusConflict},
		{"context taken", endpointRequestBody("p", "/wso2/v1", nil), http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			s.CreateModelEndpoint(w, jsonRequest(t, http.MethodPost, "/model-endpoints", tc.body))
			assert.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}

	w := httptest.NewRecorder()
	s.DeleteModelEndpoint(w, jsonRequest(t, http.MethodDelete, "/model-endpoints/wso2", nil), "wso2")
	assert.Equal(t, http.StatusConflict, w.Code, "the provisioned endpoint cannot be deleted")
}

func TestModelKeyHandlers(t *testing.T) {
	s := newModelEndpointTestServer(t)

	// An endpoint key.
	w := httptest.NewRecorder()
	s.CreateModelEndpointAPIKey(w, jsonRequest(t, http.MethodPost, "/model-endpoints/wso2/api-keys",
		map[string]any{"name": "team-a"}), "wso2")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = httptest.NewRecorder()
	s.ListModelEndpointAPIKeys(w, jsonRequest(t, http.MethodGet, "/model-endpoints/wso2/api-keys", nil), "wso2")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "team-a")

	// A Model on an endpoint with api-key-auth can have keys.
	_, err := s.llmDeploymentService.CreateLLMProvider(utils.LLMDeploymentParams{
		Data: []byte(`{"apiVersion":"gateway.api-platform.wso2.com/v1","kind":"LlmProvider","metadata":{"name":"openai"},
			"spec":{"displayName":"OpenAI","version":"v1.0","template":"openai","exposure":"none",
			"upstream":{"url":"https://api.openai.com/v1"}}}`),
		ContentType: "application/json", Origin: models.OriginGatewayAPI, Logger: s.logger,
	})
	require.NoError(t, err)
	createModel := func(handle string, endpoints ...string) {
		t.Helper()
		body := map[string]any{
			"apiVersion": "gateway.api-platform.wso2.com/v1", "kind": "Model", "metadata": map[string]any{"name": handle},
			"spec": map[string]any{"displayName": handle, "version": "v1.0", "modelName": handle, "endpoints": endpoints,
				"targets": []any{map[string]any{"name": "a", "provider": map[string]any{"id": "openai"}, "upstreamModel": "gpt-4o"}}},
		}
		w := httptest.NewRecorder()
		s.CreateModel(w, jsonRequest(t, http.MethodPost, "/models", body))
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	}
	createModel("gpt", "wso2")
	w = httptest.NewRecorder()
	s.CreateModelAPIKey(w, jsonRequest(t, http.MethodPost, "/models/gpt/api-keys", map[string]any{"name": "gpt-app"}), "gpt")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// A Model on no endpoint that checks API keys cannot: the key would authenticate nothing.
	w = httptest.NewRecorder()
	s.CreateModelEndpoint(w, jsonRequest(t, http.MethodPost, "/model-endpoints", endpointRequestBody("sso", "/sso", map[string]any{
		"policies": []any{map[string]any{"name": "jwt-auth", "version": "v1"}},
	})))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	createModel("sso-only", "sso")
	w = httptest.NewRecorder()
	s.CreateModelAPIKey(w, jsonRequest(t, http.MethodPost, "/models/sso-only/api-keys", map[string]any{"name": "x"}), "sso-only")
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "api-key-auth")

	w = httptest.NewRecorder()
	s.CreateModelAPIKey(w, jsonRequest(t, http.MethodPost, "/models/missing/api-keys", map[string]any{"name": "x"}), "missing")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

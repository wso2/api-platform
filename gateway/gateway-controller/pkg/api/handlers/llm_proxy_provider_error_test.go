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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// The HTTP-layer regression suite for wso2/api-platform#3268 intentionally
// keeps its reach to the paths that are observable inside this in-memory
// harness. The n = 10 examples below are the subset of the full issue
// description that the harness can assert against; cases that depend on
// runtime state outside this harness (for example, TLS certificate rotation
// or an external OIDC provider) are out of scope and are deliberately not
// asserted here.

// llmProxyProviderTestConfig builds an otherwise valid LlmProxy body whose
// spec.provider.id points at providerHandle.
func llmProxyProviderTestConfig(handle, providerHandle string) api.LLMProxyConfiguration {
	return api.LLMProxyConfiguration{
		ApiVersion: api.LLMProxyConfigurationApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProxyConfigurationKindLlmProxy,
		Metadata:   api.Metadata{Name: handle},
		Spec: api.LLMProxyConfigData{
			DisplayName: handle,
			Version:     "v1.0",
			Context:     stringPtr("/chat"),
			Provider:    &api.LLMProxyProvider{Id: providerHandle},
		},
	}
}

func llmProxyProviderTestBody(t *testing.T, cfg api.LLMProxyConfiguration) []byte {
	t.Helper()
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	return data
}

// setupLLMProxyProviderTest wires a test API server with the "openai" template
// and a valid "openai-primary" provider, mirroring a healthy gateway.
func setupLLMProxyProviderTest(t *testing.T) (*APIServer, *MockStorage) {
	t.Helper()

	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)

	// The shared test server wires no policy version resolver, which makes every
	// successful transform fail with "policy definition missing". Re-wire the LLM
	// deployment service with a static resolver so success paths are observable.
	server.llmDeploymentService = utils.NewLLMDeploymentService(
		server.store,
		server.db,
		nil,
		nil,
		map[string]*api.LLMProviderTemplate{},
		server.deploymentService,
		server.routerConfig,
		utils.NewStaticPolicyVersionResolver(map[string]string{
			// UPSTREAM_AUTH_APIKEY_POLICY_NAME and SET_HEADERS_POLICY_NAME are
			// both "set-headers"; one entry covers them.
			constants.SET_HEADERS_POLICY_NAME:          "v9.9.9",
			constants.PROXY_HOST__HEADER_POLICY_NAME:   "v9.9.9",
			constants.ACCESS_CONTROL_DENY_POLICY_NAME:  "v9.9.9",
			constants.UPSTREAM_AUTH_OAUTH2_POLICY_NAME: "v9.9.9",
		}),
		nil,
	)

	require.NoError(t, mockDB.SaveLLMProviderTemplate(&models.StoredLLMProviderTemplate{
		UUID: "0000-llm-tpl-id-0000-000000000001",
		Configuration: api.LLMProviderTemplate{
			ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
			Metadata:   api.Metadata{Name: "openai"},
			Spec:       api.LLMProviderTemplateData{DisplayName: "openai"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}))

	require.NoError(t, mockDB.SaveConfig(&models.StoredConfig{
		UUID:        "openai-primary-uuid",
		Kind:        string(api.LLMProviderConfigurationKindLlmProvider),
		Handle:      "openai-primary",
		DisplayName: "openai-primary",
		Version:     "v1.0",
		SourceConfiguration: api.LLMProviderConfiguration{
			ApiVersion: api.LLMProviderConfigurationApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.LLMProviderConfigurationKindLlmProvider,
			Metadata:   api.Metadata{Name: "openai-primary"},
			Spec: api.LLMProviderConfigData{
				DisplayName:   "openai-primary",
				Version:       "v1.0",
				Context:       stringPtr("/openai-primary"),
				Template:      "openai",
				Upstream:      api.LLMProviderConfigData_Upstream{Url: stringPtr("https://example.com")},
				AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
			},
		},
		DesiredState: models.StateDeployed,
		Origin:       models.OriginGatewayAPI,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}))

	return server, mockDB
}

func saveLLMProxyProviderTestProxy(t *testing.T, mockDB *MockStorage, handle, providerHandle string) {
	t.Helper()
	require.NoError(t, mockDB.SaveConfig(&models.StoredConfig{
		UUID:                handle + "-uuid",
		Kind:                string(api.LLMProxyConfigurationKindLlmProxy),
		Handle:              handle,
		DisplayName:         handle,
		Version:             "v1.0",
		SourceConfiguration: llmProxyProviderTestConfig(handle, providerHandle),
		DesiredState:        models.StateDeployed,
		Origin:              models.OriginGatewayAPI,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}))
}

func decodeErrorResponse(t *testing.T, w *httptest.ResponseRecorder) api.ErrorResponse {
	t.Helper()
	var resp api.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// Issue case A: POST /llm-proxies with a dangling spec.provider.id must answer
// 400 identifying the provider, not 404 claiming the proxy is missing, and no
// proxy may be persisted. It is one of n regression cases added for this fix.
func TestCreateLLMProxy_DanglingPrimaryProviderReturns400(t *testing.T) {
	server, mockDB := setupLLMProxyProviderTest(t)

	body := llmProxyProviderTestBody(t, llmProxyProviderTestConfig("dangling-primary-proxy", "missing-provider"))
	w, r := createTestContext("POST", "/llm-proxies", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCorrelationID(r, "corr-3268-a")
	server.CreateLLMProxy(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code, "response: %s", w.Body.String())
	resp := decodeErrorResponse(t, w)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Message, "missing-provider")
	assert.NotContains(t, resp.Message, "LLM proxy configuration not found")

	_, err := mockDB.GetConfigByKindAndHandle(string(api.LLMProxyConfigurationKindLlmProxy), "dangling-primary-proxy")
	require.Error(t, err, "no proxy must be persisted for a dangling provider reference")
}

// Issue case B: valid primary provider, dangling entry in additionalProviders.
// It is one of n regression cases added for this fix.
func TestCreateLLMProxy_DanglingAdditionalProviderReturns400(t *testing.T) {
	server, mockDB := setupLLMProxyProviderTest(t)

	cfg := llmProxyProviderTestConfig("dangling-additional-proxy", "openai-primary")
	cfg.Spec.AdditionalProviders = &[]api.LLMProxyAdditionalProvider{
		{
			Id: "missing-additional-provider",
			As: stringPtr("backup-alias"),
		},
	}
	body := llmProxyProviderTestBody(t, cfg)

	w, r := createTestContext("POST", "/llm-proxies", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCorrelationID(r, "corr-3268-b")
	server.CreateLLMProxy(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code, "response: %s", w.Body.String())
	resp := decodeErrorResponse(t, w)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Message, "missing-additional-provider")
	assert.NotContains(t, resp.Message, "LLM proxy configuration not found")

	_, err := mockDB.GetConfigByKindAndHandle(string(api.LLMProxyConfigurationKindLlmProxy), "dangling-additional-proxy")
	require.Error(t, err, "no partially valid proxy must be persisted")
}

// Issue case C: PUT on an existing proxy with a dangling provider must answer
// 400 identifying the provider and leave the stored proxy untouched. It is one
// of n regression cases added for this fix.
func TestUpdateLLMProxy_DanglingProviderReturns400AndKeepsOriginal(t *testing.T) {
	server, mockDB := setupLLMProxyProviderTest(t)
	saveLLMProxyProviderTestProxy(t, mockDB, "existing-proxy", "openai-primary")

	body := llmProxyProviderTestBody(t, llmProxyProviderTestConfig("existing-proxy", "missing-provider"))
	w, r := createTestContext("PUT", "/llm-proxies/existing-proxy", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCorrelationID(r, "corr-3268-c")
	server.UpdateLLMProxy(w, r, "existing-proxy")

	require.Equal(t, http.StatusBadRequest, w.Code, "response: %s", w.Body.String())
	resp := decodeErrorResponse(t, w)
	assert.Contains(t, resp.Message, "missing-provider")
	assert.NotContains(t, resp.Message, "LLM proxy configuration with handle")

	stored, err := mockDB.GetConfigByKindAndHandle(string(api.LLMProxyConfigurationKindLlmProxy), "existing-proxy")
	require.NoError(t, err)
	storedProxy, ok := stored.SourceConfiguration.(api.LLMProxyConfiguration)
	require.True(t, ok)
	assert.Equal(t, "openai-primary", storedProxy.Spec.Provider.Id,
		"the original provider reference must survive a rejected update")
}

// A genuinely missing proxy keeps its documented 404. This is one of n
// regression cases added for this fix (the preserved-motion case that proves
// the fix does not widen 400 onto genuine missing-proxy lookups).
func TestUpdateLLMProxy_MissingProxyStillReturns404(t *testing.T) {
	server, _ := setupLLMProxyProviderTest(t)

	body := llmProxyProviderTestBody(t, llmProxyProviderTestConfig("no-such-proxy", "openai-primary"))
	w, r := createTestContext("PUT", "/llm-proxies/no-such-proxy", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCorrelationID(r, "corr-3268-d")
	server.UpdateLLMProxy(w, r, "no-such-proxy")

	require.Equal(t, http.StatusNotFound, w.Code, "response: %s", w.Body.String())
	resp := decodeErrorResponse(t, w)
	assert.Contains(t, resp.Message, "LLM proxy configuration with handle 'no-such-proxy' not found")
}

// A genuinely missing proxy keeps its documented 404 on GET. This is one of n
// regression cases added for this fix (the preserved-motion case for the GET
// path).
func TestGetLLMProxyById_MissingProxyStillReturns404(t *testing.T) {
	server, _ := setupLLMProxyProviderTest(t)

	w, r := createTestContext("GET", "/llm-proxies/no-such-proxy", nil)
	server.GetLLMProxyById(w, r, "no-such-proxy")

	require.Equal(t, http.StatusNotFound, w.Code, "response: %s", w.Body.String())
	resp := decodeErrorResponse(t, w)
	assert.Contains(t, resp.Message, "no-such-proxy")
}

// Successful creation keeps its 201 and persists the configuration. This is
// one of n regression cases added for this fix (the preserved-motion success
// case for the POST path).
func TestCreateLLMProxy_ValidProviderReturns201(t *testing.T) {
	server, mockDB := setupLLMProxyProviderTest(t)

	body := llmProxyProviderTestBody(t, llmProxyProviderTestConfig("valid-proxy", "openai-primary"))
	w, r := createTestContext("POST", "/llm-proxies", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCorrelationID(r, "corr-3268-e")
	server.CreateLLMProxy(w, r)

	require.Equal(t, http.StatusCreated, w.Code, "response: %s", w.Body.String())

	stored, err := mockDB.GetConfigByKindAndHandle(string(api.LLMProxyConfigurationKindLlmProxy), "valid-proxy")
	require.NoError(t, err)
	storedProxy, ok := stored.SourceConfiguration.(api.LLMProxyConfiguration)
	require.True(t, ok)
	assert.Equal(t, "openai-primary", storedProxy.Spec.Provider.Id)
}

// Successful update keeps its 200 and applies the new configuration. This is
// one of n regression cases added for this fix (the preserved-motion success
// case for the PUT path).
func TestUpdateLLMProxy_ValidProviderReturns200(t *testing.T) {
	server, mockDB := setupLLMProxyProviderTest(t)
	saveLLMProxyProviderTestProxy(t, mockDB, "existing-proxy", "openai-primary")

	updated := llmProxyProviderTestConfig("existing-proxy", "openai-primary")
	updated.Spec.DisplayName = "renamed-proxy"
	body := llmProxyProviderTestBody(t, updated)

	w, r := createTestContext("PUT", "/llm-proxies/existing-proxy", body)
	r.Header.Set("Content-Type", "application/json")
	r = withCorrelationID(r, "corr-3268-f")
	server.UpdateLLMProxy(w, r, "existing-proxy")

	require.Equal(t, http.StatusOK, w.Code, "response: %s", w.Body.String())

	stored, err := mockDB.GetConfigByKindAndHandle(string(api.LLMProxyConfigurationKindLlmProxy), "existing-proxy")
	require.NoError(t, err)
	storedProxy, ok := stored.SourceConfiguration.(api.LLMProxyConfiguration)
	require.True(t, ok)
	assert.Equal(t, "renamed-proxy", storedProxy.Spec.DisplayName)
}

// Malformed payloads keep their client-error semantics. This is one of n
// regression cases added for this fix (the preserved-motion negative case for
// unparseable bodies).
func TestCreateLLMProxy_MalformedBodyReturns400(t *testing.T) {
	server, _ := setupLLMProxyProviderTest(t)

	w, r := createTestContext("POST", "/llm-proxies", []byte("invalid: ["))
	r.Header.Set("Content-Type", "application/json")
	r = withCorrelationID(r, "corr-3268-g")
	server.CreateLLMProxy(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code, "response: %s", w.Body.String())
	resp := decodeErrorResponse(t, w)
	assert.NotContains(t, resp.Message, "LLM proxy configuration not found")
}

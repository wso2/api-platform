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

package embeddings

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/testbench/services/entra"
)

const azurePath = "/openai/deployments/text-embedding/embeddings?api-version=2024-10-21"

// entraToken is a token the testbench Entra mock issues for audience.
func entraToken(t *testing.T, audience string) string {
	t.Helper()
	claims, err := json.Marshal(entra.Claims{Aud: audience, Tid: "7f3c2a10-4b5e-4c6d-8e9f-0a1b2c3d4e5f", AppID: "system-assigned", N: 1})
	require.NoError(t, err)
	return entra.TokenPrefix + base64.RawURLEncoding.EncodeToString(claims)
}

// send drives the handler with a request of method to path.
func send(method, path string, headers map[string]string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	recorder := httptest.NewRecorder()
	New().Handler().ServeHTTP(recorder, req)
	return recorder
}

// azureEmbed posts body to the Azure route; an empty path means the default deployment URL.
func azureEmbed(t *testing.T, path string, headers map[string]string, body string) *httptest.ResponseRecorder {
	t.Helper()
	if path == "" {
		path = azurePath
	}
	return send(http.MethodPost, path, headers, body)
}

func keyHeaders() map[string]string { return map[string]string{"api-key": APIKey} }

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func requireErrorCode(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	require.Equal(t, status, recorder.Code)
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, code, body.Error.Code)
}

func decodeEmbeddings(t *testing.T, recorder *httptest.ResponseRecorder) response {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var resp response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	return resp
}

func TestAzureRouteAcceptsTheKey(t *testing.T) {
	resp := decodeEmbeddings(t, azureEmbed(t, "", keyHeaders(), `{"input":"hello"}`))
	require.Len(t, resp.Data, 1)
	require.Len(t, resp.Data[0].Embedding, Dimension)
}

func TestAzureRouteAcceptsEntraTokensForBothAudiences(t *testing.T) {
	for _, audience := range []string{entra.CognitiveAudience, entra.FoundryAudience} {
		t.Run(audience, func(t *testing.T) {
			recorder := azureEmbed(t, "", bearer(entraToken(t, audience)), `{"input":"hello"}`)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		})
	}
}

func TestAzureRouteRejectsOtherBearerTokens(t *testing.T) {
	for name, token := range map[string]string{
		"not from entra": "not-from-entra",
		"empty":          "",
		"other audience": entraToken(t, "api://other"),
	} {
		t.Run(name, func(t *testing.T) {
			recorder := azureEmbed(t, "", bearer(token), `{"input":"hello"}`)
			requireErrorCode(t, recorder, http.StatusUnauthorized, "401")
			if token != "" {
				require.NotContains(t, recorder.Body.String(), token, "the 401 must not echo the token")
			}
		})
	}
}

func TestAzureRouteJudgesABearerTokenEvenWithAKey(t *testing.T) {
	headers := bearer("not-from-entra")
	headers["api-key"] = APIKey
	requireErrorCode(t, azureEmbed(t, "", headers, `{"input":"hello"}`), http.StatusUnauthorized, "401")
}

func TestAzureRouteRejectsAMissingOrWrongKey(t *testing.T) {
	requireErrorCode(t, azureEmbed(t, "", nil, `{"input":"hello"}`), http.StatusUnauthorized, "401")
	requireErrorCode(t, azureEmbed(t, "", map[string]string{"api-key": "wrong"}, `{"input":"hello"}`), http.StatusUnauthorized, "401")
}

func TestAzureRouteRequiresAnAPIVersion(t *testing.T) {
	recorder := azureEmbed(t, "/openai/deployments/text-embedding/embeddings", keyHeaders(), `{"input":"hello"}`)
	requireErrorCode(t, recorder, http.StatusBadRequest, "400")
}

func TestAzureRouteChecksTheCredentialBeforeTheAPIVersion(t *testing.T) {
	recorder := azureEmbed(t, "/openai/deployments/text-embedding/embeddings", nil, `{"input":"hello"}`)
	requireErrorCode(t, recorder, http.StatusUnauthorized, "401")
}

func TestAzureRouteMatchesTheOpenAIRoute(t *testing.T) {
	azure := decodeEmbeddings(t, azureEmbed(t, "", keyHeaders(), `{"input":"same text"}`))
	openai := decodeEmbeddings(t, send(http.MethodPost, "/v1/embeddings", nil, `{"input":"same text","model":"m"}`))
	require.Equal(t, openai.Data[0].Embedding, azure.Data[0].Embedding)

	batch := decodeEmbeddings(t, azureEmbed(t, "", keyHeaders(), `{"input":["a","b"]}`))
	require.Len(t, batch.Data, 2)
	require.Equal(t, 1, batch.Data[1].Index)

	// The Azure request carries no model: the deployment names it.
	require.Empty(t, azure.Model)
}

func TestAzureRouteSimulatedErrorAnswers500(t *testing.T) {
	recorder := azureEmbed(t, "", keyHeaders(), `{"input":"please simulate an error"}`)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestAzureRouteAllowsOnlyPost(t *testing.T) {
	recorder := send(http.MethodGet, azurePath, keyHeaders(), "")
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestAzureRouteRejectsOtherPaths(t *testing.T) {
	for _, path := range []string{"/openai/embeddings", "/openai/deployments/d/chat"} {
		t.Run(path, func(t *testing.T) {
			recorder := send(http.MethodPost, path+"?api-version=2024-10-21", keyHeaders(), `{"input":"hello"}`)
			require.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

func TestOpenAIRouteStillTakesNoCredential(t *testing.T) {
	recorder := send(http.MethodPost, "/v1/embeddings", nil, `{"input":"hello"}`)
	require.Equal(t, http.StatusOK, recorder.Code)
}

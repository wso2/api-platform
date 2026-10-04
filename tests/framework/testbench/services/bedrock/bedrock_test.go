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

package bedrock

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const applyPath = "/guardrail/test-guardrail/version/DRAFT/apply"

func applyRequest(text string) *http.Request {
	body := `{"source":"INPUT","content":[{"text":{"text":"` + text + `"}}]}`
	return httptest.NewRequest(http.MethodPost, applyPath, strings.NewReader(body))
}

func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder) applyGuardrailResponse {
	t.Helper()
	var resp applyGuardrailResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	return resp
}

func TestApplyGuardrailSafeContentReturnsNoneWithEchoedOutput(t *testing.T) {
	recorder := httptest.NewRecorder()
	New().Handler().ServeHTTP(recorder, applyRequest("Hello, this is safe content"))

	require.Equal(t, http.StatusOK, recorder.Code)
	resp := decodeResponse(t, recorder)
	require.Equal(t, "NONE", resp.Action)
	// A caller reading outputs[0].text (as real Bedrock guarantees is always present, even
	// for action NONE) must not find an empty slice here.
	require.Len(t, resp.Outputs, 1)
	require.Equal(t, "Hello, this is safe content", resp.Outputs[0]["text"])
	require.Empty(t, resp.Assessments)
}

func TestApplyGuardrailEmptyContentReturnsNoneWithEmptyEchoedOutput(t *testing.T) {
	recorder := httptest.NewRecorder()
	New().Handler().ServeHTTP(recorder, applyRequest(""))

	require.Equal(t, http.StatusOK, recorder.Code)
	resp := decodeResponse(t, recorder)
	require.Equal(t, "NONE", resp.Action)
	require.Len(t, resp.Outputs, 1)
	require.Equal(t, "", resp.Outputs[0]["text"])
}

func TestApplyGuardrailViolatingKeywordsAreIntervened(t *testing.T) {
	for _, keyword := range []string{"violence", "hate", "illegal"} {
		t.Run(keyword, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			New().Handler().ServeHTTP(recorder, applyRequest("this contains "+keyword))

			require.Equal(t, http.StatusOK, recorder.Code)
			resp := decodeResponse(t, recorder)
			require.Equal(t, "GUARDRAIL_INTERVENED", resp.Action)
			require.Len(t, resp.Outputs, 1)
			require.NotNil(t, resp.Assessments[0].ContentPolicy)
		})
	}
}

func TestApplyGuardrailSimulatedErrorKeywordsReturn500(t *testing.T) {
	recorder := httptest.NewRecorder()
	New().Handler().ServeHTTP(recorder, applyRequest("please simulate an error here"))

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestApplyGuardrailMaskedEmailIsAnonymizedThenRestorable(t *testing.T) {
	recorder := httptest.NewRecorder()
	New().Handler().ServeHTTP(recorder, applyRequest("Contact me at mask-test@example.com"))

	require.Equal(t, http.StatusOK, recorder.Code)
	resp := decodeResponse(t, recorder)
	require.Equal(t, "GUARDRAIL_INTERVENED", resp.Action)
	text, _ := resp.Outputs[0]["text"].(string)
	require.Contains(t, text, "$ANONYMIZED_EMAIL$")
	require.NotContains(t, text, "mask-test@example.com")
	require.NotNil(t, resp.Assessments[0].SensitiveInformationPolicy)
}

func TestApplyGuardrailPlainEmailIsPermanentlyRedacted(t *testing.T) {
	recorder := httptest.NewRecorder()
	New().Handler().ServeHTTP(recorder, applyRequest("My SSN is test@example.com"))

	require.Equal(t, http.StatusOK, recorder.Code)
	resp := decodeResponse(t, recorder)
	text, _ := resp.Outputs[0]["text"].(string)
	require.Contains(t, text, "*****")
	require.NotContains(t, text, "test@example.com")
}

func TestApplyGuardrailInvalidJSONReturns400(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, applyPath, strings.NewReader("{invalid"))
	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestApplyGuardrailRejectsMalformedGuardrailPath(t *testing.T) {
	for _, path := range []string{"/guardrail/x/version/DRAFT", "/guardrail/x/apply", "/wrong/path"} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
			New().Handler().ServeHTTP(recorder, req)
			require.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

func TestApplyGuardrailRejectsNonPostMethod(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, applyPath, nil)
	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestHealthEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	New().Handler().ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"status":"ok"`)
}

func TestServiceMetadata(t *testing.T) {
	s := New()
	require.Equal(t, "bedrock", s.Name())
	require.Equal(t, Port, s.Port())
	require.False(t, s.Stateful())
}

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
	"crypto/sha256"
	"encoding/hex"
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

const (
	converseModel    = "amazon.nova-lite-v1:0"
	conversePathFor  = "/model/amazon.nova-lite-v1:0/converse"
	converseBody     = `{"messages":[{"role":"user","content":[{"text":"Hi"}]}]}`
	converseHost     = "bedrock-runtime.us-east-1.amazonaws.com"
	converseAmzDate  = "20260101T000000Z"
	converseScopeFor = "20260101/us-east-1/bedrock/aws4_request"
)

// signedConverseRequest is a Converse request signed over signedBody, with the access key and
// credential scope given; sentBody is what the request carries.
func signedConverseRequest(signedBody, sentBody, accessKey, scope string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, conversePathFor, strings.NewReader(sentBody))
	req.Host = converseHost
	hash := sha256.Sum256([]byte(signedBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Amz-Date", converseAmzDate)
	req.Header.Set("X-Amz-Content-Sha256", hex.EncodeToString(hash[:]))
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"
	signature := sigV4Signature(req, signedHeaders, scope, hex.EncodeToString(hash[:]))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+accessKey+"/"+scope+
		", SignedHeaders="+signedHeaders+", Signature="+signature)
	return req
}

func serveConverse(req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	New().Handler().ServeHTTP(recorder, req)
	return recorder
}

func TestConverseAnswersAValidSigV4Signature(t *testing.T) {
	recorder := serveConverse(signedConverseRequest(converseBody, converseBody, ConverseAccessKeyID, converseScopeFor))

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Output struct {
			Message struct {
				Role    string `json:"role"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"output"`
		StopReason string `json:"stopReason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "assistant", resp.Output.Message.Role)
	require.Len(t, resp.Output.Message.Content, 1)
	require.Equal(t, "Hello from mock Bedrock ("+converseModel+"): Hi", resp.Output.Message.Content[0].Text)
	require.Equal(t, "end_turn", resp.StopReason)
}

func TestConverseRefusesAnInvalidSignature(t *testing.T) {
	for name, req := range map[string]*http.Request{
		"signed over another body": signedConverseRequest(`{"messages":[]}`, converseBody, ConverseAccessKeyID, converseScopeFor),
		"unknown access key":       signedConverseRequest(converseBody, converseBody, "AKIAUNKNOWN000000000", converseScopeFor),
		"scope for another service": signedConverseRequest(converseBody, converseBody, ConverseAccessKeyID,
			"20260101/us-east-1/s3/aws4_request"),
		"no credential": httptest.NewRequest(http.MethodPost, conversePathFor, strings.NewReader(converseBody)),
	} {
		t.Run(name, func(t *testing.T) {
			recorder := serveConverse(req)
			require.Equal(t, http.StatusForbidden, recorder.Code)
			require.Equal(t, "SignatureDoesNotMatch", recorder.Header().Get("X-Amzn-ErrorType"))
		})
	}

	// A host other than the one signed is a different request.
	req := signedConverseRequest(converseBody, converseBody, ConverseAccessKeyID, converseScopeFor)
	req.Host = "bedrock-runtime.eu-west-1.amazonaws.com"
	require.Equal(t, http.StatusForbidden, serveConverse(req).Code)
}

func TestConverseAcceptsTheAPIKeyOnlyAsTheSingleAuthorizationHeader(t *testing.T) {
	keyed := func(values ...string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, conversePathFor, strings.NewReader(converseBody))
		for _, v := range values {
			req.Header.Add("Authorization", v)
		}
		return req
	}

	require.Equal(t, http.StatusOK, serveConverse(keyed("Bearer "+ConverseAPIKey)).Code)

	for name, req := range map[string]*http.Request{
		"unknown key":                 keyed("Bearer itNotTheMockKey"),
		"beside another bearer token": keyed("Bearer gateway-key", "Bearer "+ConverseAPIKey),
	} {
		t.Run(name, func(t *testing.T) {
			recorder := serveConverse(req)
			require.Equal(t, http.StatusForbidden, recorder.Code)
			require.Contains(t, recorder.Body.String(), "API key is not valid")
		})
	}
}

func TestConverseRefusesMalformedCalls(t *testing.T) {
	get := httptest.NewRequest(http.MethodGet, conversePathFor, nil)
	recorder := serveConverse(get)
	require.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
	require.Equal(t, http.MethodPost, recorder.Header().Get("Allow"))

	invalid := httptest.NewRequest(http.MethodPost, conversePathFor, strings.NewReader("not json"))
	invalid.Header.Set("Authorization", "Bearer "+ConverseAPIKey)
	require.Equal(t, http.StatusBadRequest, serveConverse(invalid).Code)

	oversized := httptest.NewRequest(http.MethodPost, conversePathFor,
		strings.NewReader(strings.Repeat("x", maxConverseBodyBytes+1)))
	oversized.Header.Set("Authorization", "Bearer "+ConverseAPIKey)
	require.Equal(t, http.StatusRequestEntityTooLarge, serveConverse(oversized).Code)

	// Converse and ApplyGuardrail are the only operations served.
	invoke := httptest.NewRequest(http.MethodPost, "/model/"+converseModel+"/invoke", strings.NewReader(converseBody))
	require.Equal(t, http.StatusNotFound, serveConverse(invoke).Code)
}

func TestEscapeSigV4PathEncodesReservedBytes(t *testing.T) {
	require.Equal(t, "/model/amazon.nova-lite-v1%3A0/converse", escapeSigV4Path("/model/amazon.nova-lite-v1:0/converse"))
	require.Equal(t, "/a-b_c.d~e/%20", escapeSigV4Path("/a-b_c.d~e/ "))
	require.Equal(t, "", escapeSigV4Path(""))
}

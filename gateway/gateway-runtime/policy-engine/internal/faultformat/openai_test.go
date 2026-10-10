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

package faultformat

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// openAIError is the envelope an OpenAI SDK parses. Decoded into a map for the inner object so
// a test can tell an explicit null apart from an absent field — the SDKs read both `param` and
// `code` and the reference API always sends them.
func decodeOpenAI(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &envelope), "body must be JSON: %s", body)
	require.Len(t, envelope, 1, "the top level carries only `error`, as the SDKs expect: %s", body)
	raw, ok := envelope["error"]
	require.True(t, ok, "missing `error` object: %s", body)
	var inner map[string]any
	require.NoError(t, json.Unmarshal(raw, &inner), "`error` must be an object: %s", body)
	return inner
}

func TestNegotiate_LLMKindsAreOpenAI(t *testing.T) {
	for _, kind := range []policy.APIKind{policy.APIKindLlmProvider, policy.APIKindLlmProxy} {
		for _, req := range []Request{
			{APIKind: kind},
			{APIKind: kind, Accept: "application/xml"},
			{APIKind: kind, ContentType: "text/xml"},
			{APIKind: kind, ContentType: "text/event-stream"},
		} {
			assert.Equal(t, ShapeOpenAI, Negotiate(req),
				"%s speaks the OpenAI wire whatever the request carries: %+v", kind, req)
		}
	}
}

func TestOpenAIErrorKinds_AreTheLLMKinds(t *testing.T) {
	assert.ElementsMatch(t,
		[]policy.APIKind{policy.APIKindLlmProvider, policy.APIKindLlmProxy},
		OpenAIErrorKinds())
}

func TestKindSet_WithAddsKindsWithoutTouchingTheOriginal(t *testing.T) {
	base := SupportedKinds()
	extended := base.With(OpenAIErrorKinds()...)

	assert.True(t, extended.Enabled(policy.APIKindAgent), "the shipped kinds survive")
	assert.True(t, extended.Enabled(policy.APIKindLlmProvider))
	assert.True(t, extended.Enabled(policy.APIKindLlmProxy))
	assert.False(t, base.Enabled(policy.APIKindLlmProxy), "With must not mutate the receiver")
}

func TestRender_OpenAI_CarriesTheFaultInTheSDKEnvelope(t *testing.T) {
	body, contentType, ok := NewRegistry().Render(ShapeOpenAI, RenderInput{
		Err: policy.FaultDetails{
			Code:        "900901",
			Type:        policy.FaultTypeAuthentication,
			Message:     "Invalid API key",
			Description: "secret detail that must not leave",
		},
		Status:  http.StatusUnauthorized,
		ErrorID: "abc-123",
	})
	require.True(t, ok)
	assert.Equal(t, "application/json", contentType)

	inner := decodeOpenAI(t, body)
	assert.Equal(t, "Invalid API key", inner["message"])
	assert.Equal(t, "authentication_error", inner["type"])
	assert.Equal(t, "900901", inner["code"])
	assert.Contains(t, inner, "param")
	assert.Nil(t, inner["param"])
	assert.Equal(t, "abc-123", inner["error_id"])
	assert.NotContains(t, string(body), "secret detail", "Description is never rendered")
}

func TestRender_OpenAI_SparseErrorStillHasEveryField(t *testing.T) {
	body, _, ok := NewRegistry().Render(ShapeOpenAI, RenderInput{Status: http.StatusServiceUnavailable})
	require.True(t, ok)

	inner := decodeOpenAI(t, body)
	assert.Equal(t, "Service Unavailable", inner["message"],
		"with nothing described, the status text beats a generic sentence")
	assert.Equal(t, "server_error", inner["type"])
	assert.Contains(t, inner, "code")
	assert.Nil(t, inner["code"], "an unclassified error sends an explicit null code")
	assert.Contains(t, inner, "param")
	assert.NotContains(t, inner, "error_id")
}

func TestRender_OpenAI_TypeFollowsTheFaultClassThenTheStatus(t *testing.T) {
	cases := []struct {
		name      string
		faultType string
		status    int
		want      string
	}{
		{"authentication class", policy.FaultTypeAuthentication, http.StatusUnauthorized, "authentication_error"},
		{"authorization class", policy.FaultTypeAuthorization, http.StatusForbidden, "permission_error"},
		{"throttling class", policy.FaultTypeThrottling, http.StatusTooManyRequests, "rate_limit_error"},
		{"guardrail class", policy.FaultTypeGuardrail, http.StatusUnprocessableEntity, "invalid_request_error"},
		{"validation class", policy.FaultTypeValidation, http.StatusBadRequest, "invalid_request_error"},
		{"request size class", policy.FaultTypeRequestSize, http.StatusRequestEntityTooLarge, "invalid_request_error"},
		{"routing class", policy.FaultTypeRouting, http.StatusNotFound, "not_found_error"},
		{"upstream class", policy.FaultTypeUpstream, http.StatusServiceUnavailable, "server_error"},
		{"internal class", policy.FaultTypeInternal, http.StatusInternalServerError, "server_error"},
		{"unclassified 401", "", http.StatusUnauthorized, "authentication_error"},
		{"unclassified 403", "", http.StatusForbidden, "permission_error"},
		{"unclassified 404", "", http.StatusNotFound, "not_found_error"},
		{"unclassified 429", "", http.StatusTooManyRequests, "rate_limit_error"},
		{"unclassified 422", "", http.StatusUnprocessableEntity, "invalid_request_error"},
		{"unclassified 502", "", http.StatusBadGateway, "server_error"},
		{"unknown class falls back to status", "somethingNew", http.StatusTooManyRequests, "rate_limit_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _, ok := NewRegistry().Render(ShapeOpenAI, RenderInput{
				Err:    policy.FaultDetails{Type: tc.faultType, Message: "m"},
				Status: tc.status,
			})
			require.True(t, ok)
			assert.Equal(t, tc.want, decodeOpenAI(t, body)["type"])
		})
	}
}

func TestRender_OpenAI_GuardrailBlockSitsInsideTheErrorObject(t *testing.T) {
	body, _, ok := NewRegistry().Render(ShapeOpenAI, RenderInput{
		Err: policy.FaultDetails{
			Type:      policy.FaultTypeGuardrail,
			Direction: policy.DirectionRequest,
			Message:   "Blocked by content guardrail",
			Guardrail: &policy.GuardrailDetails{
				InterveningGuardrail: "word-count-guardrail",
				Action:               "GUARDRAIL_INTERVENED",
				Assessments:          map[string]any{"limit": float64(10)},
			},
		},
		Status: http.StatusUnprocessableEntity,
	})
	require.True(t, ok)

	g, ok := decodeOpenAI(t, body)["guardrail"].(map[string]any)
	require.True(t, ok, "guardrail detail belongs under error, not beside it: %s", body)
	assert.Equal(t, "word-count-guardrail", g["interveningGuardrail"])
	assert.Equal(t, "REQUEST", g["direction"])
	assert.Equal(t, map[string]any{"limit": float64(10)}, g["assessments"])
}

// ─── Reshaping a body a policy wrote ─────────────────────────────────────────

func llmInput() Input {
	return Input{
		FormatterEnabled: true,
		APIKind:          policy.APIKindLlmProxy,
		Status:           http.StatusUnauthorized,
	}
}

// The point of the option: on an LLM route a policy that still writes its own JSON does not
// get to hand an OpenAI SDK a body it cannot parse. The kernel describes the failure with the
// body's own message, and ShouldFormat renders that description.
func TestOpenAIPolicyBodyMessage_LiftsThePolicysMessage(t *testing.T) {
	msg, ok := OpenAIPolicyBodyMessage([]byte(`{"error":"Unauthorized","message":"Invalid or missing API key"}`))
	require.True(t, ok)
	assert.Equal(t, "Invalid or missing API key", msg)

	in := llmInput()
	in.Err = policy.FaultDetails{Message: msg}
	in.PolicyDescribed = true
	d := ShouldFormat(NewRegistry(), in)
	require.True(t, d.Format, d.Reason)
	inner := decodeOpenAI(t, d.Body)
	assert.Equal(t, "Invalid or missing API key", inner["message"])
	assert.Equal(t, "authentication_error", inner["type"])
}

// A body with no usable message is still reshaped: the envelope falls back to the status text.
func TestOpenAIPolicyBodyMessage_BodyWithoutAMessageIsStillReshaped(t *testing.T) {
	msg, ok := OpenAIPolicyBodyMessage([]byte(`<html>denied</html>`))
	assert.True(t, ok)
	assert.Empty(t, msg)
}

func TestOpenAIPolicyBodyMessage_KeepsAnExplicitlyEmptyBody(t *testing.T) {
	_, ok := OpenAIPolicyBodyMessage([]byte{})
	assert.False(t, ok)
}

// A body that is already the OpenAI envelope — an LLM-only policy that writes it natively —
// is kept byte-for-byte, so a richer `code`/`param` it set is not flattened.
func TestOpenAIPolicyBodyMessage_KeepsAnAlreadyOpenAIBody(t *testing.T) {
	_, ok := OpenAIPolicyBodyMessage([]byte(`{"error":{"message":"model not allowed","type":"invalid_request_error","param":"model","code":"model_not_found"}}`))
	assert.False(t, ok)
}

// Authorship still stands in ShouldFormat itself: whatever the kernel leaves marked authored —
// an operator's fault entry, the backend's own error — is never overwritten.
func TestShouldFormat_OpenAIKeepsAuthoredBodies(t *testing.T) {
	in := llmInput()
	in.BodyAuthored = true

	d := ShouldFormat(NewRegistry(), in)
	assert.False(t, d.Format)
	assert.Contains(t, d.Reason, "authored")
}

// On an LLM route the OpenAI shape renders a failure no policy described — a router error,
// the engine's own 500 — because the shape is opt-in and there is no shipped body to keep.
func TestShouldFormat_OpenAIRendersAnUndescribedFailure(t *testing.T) {
	in := llmInput()
	in.Status = http.StatusServiceUnavailable
	in.PolicyDescribed = false

	d := ShouldFormat(NewRegistry(), in)
	require.True(t, d.Format, d.Reason)
	assert.Equal(t, "server_error", decodeOpenAI(t, d.Body)["type"])
}

// Every other shape keeps the "format only what a policy described" rule.
func TestShouldFormat_UndescribedFailureStillStandsOffLLM(t *testing.T) {
	in := Input{
		FormatterEnabled: true,
		APIKind:          policy.APIKindAgent,
		Status:           http.StatusServiceUnavailable,
	}
	d := ShouldFormat(NewRegistry(), in)
	assert.False(t, d.Format)
	assert.Contains(t, d.Reason, "no policy described")
}

func TestShouldFormat_OpenAIOffMeansNothingChanges(t *testing.T) {
	in := llmInput()
	in.FormatterEnabled = false

	assert.False(t, ShouldFormat(NewRegistry(), in).Format)
}

func TestLegacyMessage(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"message field", `{"error":"Unauthorized","message":"Invalid API key"}`, "Invalid API key"},
		{"error string only", `{"error":"Forbidden"}`, "Forbidden"},
		{"error_description", `{"error":"invalid_token","error_description":"Token expired"}`, "Token expired"},
		{"detail", `{"detail":"Too many requests"}`, "Too many requests"},
		{"nested error message", `{"error":{"message":"inner"}}`, "inner"},
		{"non-string message is skipped", `{"message":{"action":"GUARDRAIL_INTERVENED"},"error":"Guardrail"}`, "Guardrail"},
		{"plain text", "  Rate limit exceeded \n", "Rate limit exceeded"},
		{"empty", "", ""},
		{"json with nothing usable", `{"foo":"bar"}`, ""},
		{"json array", `["a"]`, ""},
		{"binary", "\xff\xfe\x00", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, legacyMessage([]byte(tc.body)))
		})
	}
}

func TestLegacyMessage_PlainTextIsCapped(t *testing.T) {
	long := make([]byte, 4096)
	for i := range long {
		long[i] = 'a'
	}
	assert.LessOrEqual(t, len(legacyMessage(long)), maxLegacyMessageBytes)
}

func TestLegacyMessage_PlainTextIsNotAnHTMLPage(t *testing.T) {
	assert.Equal(t, "", legacyMessage([]byte("<html><body>Bad Gateway</body></html>")),
		"markup is not a message; the status text is the better fallback")
}

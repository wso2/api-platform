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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// openAIDoc decodes a rendered OpenAI error, failing the test if it is not one.
func openAIDoc(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc), "body: %s", body)
	require.Len(t, doc, 1, "the top level is exactly OpenAI's: one error member")
	inner, ok := doc["error"].(map[string]any)
	require.True(t, ok, "body: %s", body)
	return inner
}

// ─── Renderer ────────────────────────────────────────────────────────────────

func TestRender_OpenAI(t *testing.T) {
	r := NewRegistry()
	body, contentType, ok := r.Render(ShapeOpenAI, RenderInput{
		Err: policy.FaultDetails{
			Code:    policy.FaultCodeAuthMissingCredentials,
			Type:    policy.FaultTypeAuthentication,
			Message: "Missing API key",
		},
		Status: 401,
	})
	require.True(t, ok)
	assert.Equal(t, "application/json", contentType)

	inner := openAIDoc(t, body)
	assert.Equal(t, "Missing API key", inner["message"])
	assert.Equal(t, "authentication_error", inner["type"],
		"type is OpenAI's class from the status, not the gateway's own")
	assert.Equal(t, policy.FaultCodeAuthMissingCredentials, inner["code"])
	assert.Contains(t, inner, "param")
	assert.Nil(t, inner["param"])
	assert.NotContains(t, inner, "error_id", "a policy's failure is correlated by request id")
	assert.NotContains(t, inner, "guardrail")
}

// A sparse error is still a document an OpenAI SDK reads: a message, a class, and the two
// members it expects as null rather than missing or empty.
func TestRender_OpenAISparseError(t *testing.T) {
	body, _, ok := NewRegistry().Render(ShapeOpenAI, RenderInput{Status: 503, ErrorID: "e-1"})
	require.True(t, ok)
	assert.JSONEq(t,
		`{"error":{"message":"An unexpected error occurred.","type":"server_error","param":null,"code":null,"error_id":"e-1"}}`,
		string(body))
}

func TestRender_OpenAIDescriptionIsNeverForwarded(t *testing.T) {
	body, _, ok := NewRegistry().Render(ShapeOpenAI, RenderInput{
		Err:    policy.FaultDetails{Message: "Blocked", Description: "the secret prompt"},
		Status: 400,
	})
	require.True(t, ok)
	assert.NotContains(t, string(body), "the secret prompt")
}

func TestRender_OpenAICarriesTheGuardrailBlock(t *testing.T) {
	body, _, ok := NewRegistry().Render(ShapeOpenAI, RenderInput{
		Err: policy.FaultDetails{
			Message:   "Blocked",
			Guardrail: &policy.GuardrailDetails{InterveningGuardrail: "regex-guardrail", Action: "GUARDRAIL_INTERVENED"},
		},
		Status: 422,
	})
	require.True(t, ok)
	inner := openAIDoc(t, body)
	guardrail, ok := inner["guardrail"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "regex-guardrail", guardrail["interveningGuardrail"])
	assert.Equal(t, "invalid_request_error", inner["type"])
}

// Unmarshalable assessments drop the block, not the error.
func TestRender_OpenAIDegradesOnUnmarshalableAssessments(t *testing.T) {
	body, _, ok := NewRegistry().Render(ShapeOpenAI, RenderInput{
		Err: policy.FaultDetails{
			Message:   "Blocked",
			Guardrail: &policy.GuardrailDetails{Assessments: map[string]any{"bad": make(chan int)}},
		},
		Status: 400,
	})
	require.True(t, ok)
	inner := openAIDoc(t, body)
	assert.Equal(t, "Blocked", inner["message"])
	assert.NotContains(t, inner, "guardrail")
}

func TestOpenAIErrorTypeFor(t *testing.T) {
	tests := map[int]string{
		400: "invalid_request_error",
		401: "authentication_error",
		403: "permission_error",
		404: "invalid_request_error",
		405: "invalid_request_error",
		409: "invalid_request_error",
		413: "invalid_request_error",
		415: "invalid_request_error",
		422: "invalid_request_error",
		429: "rate_limit_error",
		402: "server_error",
		500: "server_error",
		502: "server_error",
		503: "server_error",
		504: "server_error",
		0:   "server_error",
	}
	for status, want := range tests {
		assert.Equal(t, want, openAIErrorTypeFor(status), "status %d", status)
	}
}

// ─── Negotiation ─────────────────────────────────────────────────────────────

// The route's own format outranks the kind and every request signal: an OpenAI client
// parses OpenAI errors whatever its Accept header says.
func TestNegotiate_RouteFormatOutranksKindAndRequest(t *testing.T) {
	for _, req := range []Request{
		{APIKind: policy.APIKindLlmProxy, RouteFormat: "openai"},
		{APIKind: policy.APIKindLlmProxy, RouteFormat: "openai", Accept: "application/xml"},
		{APIKind: policy.APIKindLlmProxy, RouteFormat: "openai", ContentType: "text/xml"},
		{APIKind: policy.APIKindLlmProxy, RouteFormat: "openai", ContentType: "text/event-stream"},
		{APIKind: policy.APIKindMCP, RouteFormat: "openai"},
		{APIKind: policy.APIKindAgent, RouteFormat: "openai", Transport: "JSONRPC"},
	} {
		assert.Equal(t, ShapeOpenAI, Negotiate(req), "%+v", req)
	}
}

// An unknown route format changes nothing: the kind and the request decide as before.
func TestNegotiate_UnknownRouteFormatIsIgnored(t *testing.T) {
	assert.Equal(t, ShapeJSON, Negotiate(Request{APIKind: policy.APIKindLlmProxy, RouteFormat: "cohere"}))
	assert.Equal(t, ShapeJSONRPC, Negotiate(Request{APIKind: policy.APIKindMCP, RouteFormat: "OpenAI"}),
		"route formats are matched exactly")
}

func TestIsRouteFormat(t *testing.T) {
	for _, name := range []string{"openai", "anthropic", "gemini"} {
		assert.True(t, IsRouteFormat(name), name)
	}
	for _, name := range []string{"", "OpenAI", "json", "jsonrpc", "xml", "passthrough"} {
		assert.False(t, IsRouteFormat(name), "%q is a shape, not a route format", name)
	}
}

// ─── ShouldFormat on a route that named its format ───────────────────────────

func TestShouldFormat_RouteFormat(t *testing.T) {
	r := NewRegistry()
	// LlmProxy is not a formatted kind, so anything rendered below is the route's doing.
	base := Input{
		APIKind:     policy.APIKindLlmProxy,
		RouteFormat: "openai",
		Status:      404,
		Err:         policy.FaultDetails{Code: "101501", Message: "Not Found"},
	}

	t.Run("enables formatting on a kind that formats nothing", func(t *testing.T) {
		in := base
		in.PolicyDescribed = true
		d := ShouldFormat(r, in)
		require.True(t, d.Format, "reason: %s", d.Reason)
		assert.Equal(t, "application/json", d.ContentType)
		assert.Equal(t, "Not Found", openAIDoc(t, d.Body)["message"])
	})

	t.Run("renders failures no policy described", func(t *testing.T) {
		in := base
		in.PolicyDescribed = false
		in.ErrorID = "engine-1"
		d := ShouldFormat(r, in)
		require.True(t, d.Format, "an opted-in route has no released body to preserve; reason: %s", d.Reason)
		assert.Equal(t, "engine-1", openAIDoc(t, d.Body)["error_id"])
	})

	t.Run("an authored body still stands", func(t *testing.T) {
		in := base
		in.BodyAuthored = true
		in.PolicyDescribed = true
		d := ShouldFormat(r, in)
		assert.False(t, d.Format)
		assert.Contains(t, d.Reason, "authored")
	})

	t.Run("HEAD still carries no body", func(t *testing.T) {
		in := base
		in.RequestMethod = "HEAD"
		d := ShouldFormat(r, in)
		assert.False(t, d.Format)
		assert.Contains(t, d.Reason, "HEAD")
	})

	t.Run("an unknown route format enables nothing", func(t *testing.T) {
		in := base
		in.RouteFormat = "cohere"
		in.PolicyDescribed = true
		d := ShouldFormat(r, in)
		assert.False(t, d.Format)
		assert.Contains(t, d.Reason, "not enabled")
	})
}

// The same failure on the same kind without the route format behaves exactly as before:
// LlmProxy formats nothing, so existing proxies keep their bytes.
func TestShouldFormat_NoRouteFormatLeavesLLMKindsAlone(t *testing.T) {
	r := NewRegistry()
	for _, kind := range []policy.APIKind{policy.APIKindLlmProxy, policy.APIKindLlmProvider, policy.APIKindRestApi} {
		d := ShouldFormat(r, Input{
			APIKind:          kind,
			FormatterEnabled: SupportedKinds().Enabled(kind),
			PolicyDescribed:  true,
			Status:           401,
			Err:              policy.FaultDetails{Message: "Denied"},
		})
		assert.False(t, d.Format, "%s must forward its errors unchanged", kind)
	}
}

// On an enabled kind the PolicyDescribed gate is untouched when no route format is named.
func TestShouldFormat_PolicyDescribedGateHoldsWithoutRouteFormat(t *testing.T) {
	d := ShouldFormat(NewRegistry(), Input{
		APIKind:          policy.APIKindMCP,
		FormatterEnabled: true,
		Status:           500,
	})
	assert.False(t, d.Format)
	assert.Contains(t, d.Reason, "no policy described")
}

// A failure with no message of its own reads as a sentence for its status, so an SDK's
// exception says what went wrong; 401 is the gateway's one authentication wording.
func TestRender_OpenAIMessageForStatus(t *testing.T) {
	for status, want := range map[int]string{
		401: "Invalid or expired credentials.",
		403: "You do not have access to this resource.",
		429: "Too many requests. Retry after a short wait.",
		500: "An unexpected error occurred.",
	} {
		body, _, ok := NewRegistry().Render(ShapeOpenAI, RenderInput{Status: status})
		require.True(t, ok)
		assert.Equal(t, want, openAIDoc(t, body)["message"], "status %d", status)
	}
	body, _, _ := NewRegistry().Render(ShapeOpenAI, RenderInput{Status: 401, Err: policy.FaultDetails{Message: "Key revoked"}})
	assert.Equal(t, "Key revoked", openAIDoc(t, body)["message"], "a message of its own wins")
}

func TestInRouteFormat(t *testing.T) {
	for body, want := range map[string]bool{
		`{"error":{"message":"x","type":"invalid_request_error"}}`:    true,
		`{"error":{"message":""}}`:                                    true,
		`{"error":"Unauthorized","message":"Valid API key required"}`: false,
		`{"error":{"type":"x"}}`:                                      false,
		`{"error":{"message":42}}`:                                    false,
		`Too Many Requests`:                                           false,
		``:                                                            false,
	} {
		assert.Equal(t, want, InRouteFormat("openai", []byte(body)), "body %q", body)
	}
	assert.False(t, InRouteFormat("", []byte(`{"error":{"message":"x"}}`)), "no route format, nothing to be in")
}

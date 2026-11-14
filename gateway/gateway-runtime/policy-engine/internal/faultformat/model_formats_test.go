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

// A model endpoint route names its inbound format, and every gateway error on it is
// answered in that format's own error object.
func TestRouteFormats_AnthropicAndGemini(t *testing.T) {
	r := NewRegistry()
	for _, format := range []ShapeID{ShapeAnthropic, ShapeGemini} {
		assert.Equal(t, format, Negotiate(Request{APIKind: policy.APIKindLlmProxy, RouteFormat: string(format),
			Accept: "application/xml"}), "the route's format outranks the request's preference")
	}

	anthropic, ct, ok := r.Render(ShapeAnthropic, RenderInput{
		Err: policy.FaultDetails{Message: "Too many requests"}, Status: 429, ErrorID: "e-1"})
	require.True(t, ok)
	assert.Equal(t, "application/json", ct)
	var a struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			ErrorID string `json:"error_id"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(anthropic, &a))
	assert.Equal(t, "error", a.Type)
	assert.Equal(t, "rate_limit_error", a.Error.Type)
	assert.Equal(t, "Too many requests", a.Error.Message)
	assert.Equal(t, "e-1", a.Error.ErrorID)

	gemini, _, ok := r.Render(ShapeGemini, RenderInput{Err: policy.FaultDetails{}, Status: 401})
	require.True(t, ok)
	var g struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(gemini, &g))
	assert.Equal(t, 401, g.Error.Code)
	assert.Equal(t, "UNAUTHENTICATED", g.Error.Status)
	assert.Equal(t, "Invalid or expired credentials.", g.Error.Message, "the one wording for every auth failure")
}

func TestAnthropicAndGoogleStatuses(t *testing.T) {
	for status, want := range map[int]string{400: "invalid_request_error", 401: "authentication_error",
		403: "permission_error", 404: "not_found_error", 413: "request_too_large", 429: "rate_limit_error",
		500: "api_error", 529: "overloaded_error"} {
		assert.Equal(t, want, anthropicErrorTypeFor(status), "%d", status)
	}
	for status, want := range map[int]string{400: "INVALID_ARGUMENT", 401: "UNAUTHENTICATED", 403: "PERMISSION_DENIED",
		404: "NOT_FOUND", 429: "RESOURCE_EXHAUSTED", 500: "INTERNAL", 503: "UNAVAILABLE", 504: "DEADLINE_EXCEEDED"} {
		assert.Equal(t, want, googleStatusFor(status), "%d", status)
	}
}

// A body a policy already wrote in the route's format is kept; one in another
// format is not.
func TestInRouteFormat_ModelFormats(t *testing.T) {
	openAI := []byte(`{"error":{"message":"m","type":"invalid_request_error"}}`)
	anthropic := []byte(`{"type":"error","error":{"type":"not_found_error","message":"m"}}`)
	gemini := []byte(`{"error":{"code":404,"message":"m","status":"NOT_FOUND"}}`)

	assert.True(t, InRouteFormat("openai", openAI))
	assert.True(t, InRouteFormat("anthropic", anthropic))
	assert.False(t, InRouteFormat("anthropic", openAI), "an Anthropic error says type: error")
	assert.False(t, InRouteFormat("anthropic", []byte(`{"Type":"error","error":{"message":"m"}}`)),
		"members are matched by their exact names")
	assert.True(t, InRouteFormat("gemini", gemini))
	assert.False(t, InRouteFormat("gemini", []byte(`{"message":"m"}`)))
	assert.False(t, InRouteFormat("cohere", openAI))
}

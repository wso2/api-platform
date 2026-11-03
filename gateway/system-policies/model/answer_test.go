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

package model

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// assertOpenAIError checks a refusal is an OpenAI error envelope the engine's
// formatter will leave alone: a JSON body, no FaultDetails beside it, and exactly
// the four members an OpenAI client reads. An empty param or code means null.
func assertOpenAIError(t *testing.T, response policy.ImmediateResponse, status int, errType, param, code string) {
	t.Helper()
	if response.StatusCode != status {
		t.Fatalf("status = %d, want %d", response.StatusCode, status)
	}
	if response.Fault != nil {
		t.Fatal("an authored OpenAI body must not carry a Fault, or the engine's formatter replaces it")
	}
	if got := response.Headers["content-type"]; got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	if got := response.Headers["cache-control"]; got != "no-store" {
		t.Fatalf("cache-control = %q, want no-store", got)
	}

	var envelope map[string]map[string]any
	if err := json.Unmarshal(response.Body, &envelope); err != nil {
		t.Fatalf("body is not an OpenAI error envelope: %v (%s)", err, response.Body)
	}
	if len(envelope) != 1 {
		t.Fatalf("envelope has %d members, want only error: %s", len(envelope), response.Body)
	}
	body := envelope["error"]
	if len(body) != 4 {
		t.Fatalf("error has %d members, want message, type, param and code: %s", len(body), response.Body)
	}
	if message, _ := body["message"].(string); message == "" {
		t.Fatalf("error carries no message: %s", response.Body)
	}
	if got := body["type"]; got != errType {
		t.Fatalf("type = %v, want %q", got, errType)
	}
	assertNullable(t, "param", body["param"], param)
	assertNullable(t, "code", body["code"], code)
}

func assertNullable(t *testing.T, field string, got any, want string) {
	t.Helper()
	if want == "" {
		if got != nil {
			t.Fatalf("%s = %v, want null", field, got)
		}
		return
	}
	if got != want {
		t.Fatalf("%s = %v, want %q", field, got, want)
	}
}

// dispatch runs the dispatch error job for a request the resolver classified.
func dispatch(t *testing.T, attributes map[string]string) policy.ImmediateResponse {
	t.Helper()
	instance := mustPolicy(t, map[string]any{ParamDispatchError: map[string]any{}})
	action := instance.(*dispatchErrorPolicy).OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{
			APIId:                "wso2-proxy-uuid",
			APIKind:              policy.APIKindLlmProxy,
			ResolutionAttributes: policy.NewResolutionAttributes(attributes),
		},
		Headers: policy.NewHeaders(nil),
		Path:    "/wso2/v1/chat/completions",
		Method:  "POST",
	}, nil)
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("the dispatch chain must always be answered, got %T", action)
	}
	return response
}

func TestEachDispatchErrorHasItsOpenAIAnswer(t *testing.T) {
	for kind, want := range map[string]struct {
		status      int
		param, code string
		errorType   string
	}{
		DispatchErrorInvalidJSON:   {400, "", "invalid_json", "invalid_request_error"},
		DispatchErrorMissingModel:  {400, "model", "missing_model", "invalid_request_error"},
		DispatchErrorInvalidModel:  {400, "model", "invalid_model", "invalid_request_error"},
		DispatchErrorModelNotFound: {404, "model", "model_not_found", "invalid_request_error"},
	} {
		t.Run(kind, func(t *testing.T) {
			response := dispatch(t, map[string]string{AttrDispatchError: kind})
			assertOpenAIError(t, response, want.status, want.errorType, want.param, want.code)
		})
	}
}

// The model name is the caller's own input. Refusing it must not reflect it, and
// the answer must be identical whatever was asked for.
func TestModelNotFoundDoesNotEchoTheRequestedModel(t *testing.T) {
	const probe = "gpt-<script>secret-model</script>"
	first := dispatch(t, map[string]string{AttrDispatchError: DispatchErrorModelNotFound, AttrModelName: probe})
	second := dispatch(t, map[string]string{AttrDispatchError: DispatchErrorModelNotFound, AttrModelName: "other"})

	if bytes.Contains(first.Body, []byte("secret-model")) {
		t.Fatalf("the refusal echoed the requested model: %s", first.Body)
	}
	if !bytes.Equal(first.Body, second.Body) {
		t.Fatalf("the refusal differs by requested model:\n%s\n%s", first.Body, second.Body)
	}
}

// Anything the job does not recognise is a resolver it does not understand, and
// the dispatch chain has no upstream to fall through to.
func TestAnUnrecognisedDispatchErrorFailsClosed(t *testing.T) {
	for name, attributes := range map[string]map[string]string{
		"absent":       nil,
		"empty":        {AttrDispatchError: ""},
		"unrecognised": {AttrDispatchError: "rate_limited"},
		"wrong case":   {AttrDispatchError: "MODEL_NOT_FOUND"},
	} {
		t.Run(name, func(t *testing.T) {
			assertOpenAIError(t, dispatch(t, attributes), 500, "server_error", "", "")
		})
	}

	instance := mustPolicy(t, map[string]any{ParamDispatchError: map[string]any{}}).(*dispatchErrorPolicy)
	for _, ctx := range []*policy.RequestHeaderContext{nil, {}} {
		response, ok := instance.OnRequestHeaders(context.Background(), ctx, nil).(policy.ImmediateResponse)
		if !ok {
			t.Fatal("expected a refusal")
		}
		assertOpenAIError(t, response, 500, "server_error", "", "")
	}
}

func catalogBlock(format string) map[string]any {
	return map[string]any{ParamCatalog: map[string]any{
		ParamFormat: format,
		ParamCatalogModels: []any{
			map[string]any{ParamCatalogHandle: "gpt", ParamCatalogName: "gpt-4o", ParamCatalogDisplayName: "GPT-4o",
				ParamCatalogCreated: float64(1760000000)},
			map[string]any{ParamCatalogHandle: "claude", ParamCatalogName: "claude-haiku", ParamCatalogCreated: float64(1760000000)},
		},
	}}
}

// catalogFor serves the catalog to a caller authenticated as auth says.
func catalogFor(t *testing.T, params map[string]any, auth *policy.AuthContext) policy.ImmediateResponse {
	t.Helper()
	instance := mustPolicy(t, params).(*catalogPolicy)
	action := instance.OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{APIKind: policy.APIKindLlmProxy, AuthContext: auth},
		Headers:       policy.NewHeaders(nil),
		Path:          "/wso2/v1/models",
		Method:        "GET",
	}, nil)
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("the catalog must always be answered, never forwarded; got %T", action)
	}
	if response.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if response.Headers["content-type"] != "application/json" || response.Headers["cache-control"] != "no-store" {
		t.Fatalf("headers = %v", response.Headers)
	}
	return response
}

func apiKeyAuth(operations string, recorded bool) *policy.AuthContext {
	ac := &policy.AuthContext{Authenticated: true, AuthType: APIKeyAuthType, Properties: map[string]string{}}
	if recorded {
		ac.Properties[AuthPropertyAPIKeyOperations] = operations
	}
	return ac
}

func TestCatalogIsRenderedInTheEndpointsFormat(t *testing.T) {
	openAI := catalogFor(t, catalogBlock(FormatOpenAI), nil)
	assertJSONEqual(t, openAI.Body, `{"object":"list","data":[
		{"id":"claude-haiku","object":"model","created":1760000000,"owned_by":"wso2"},
		{"id":"gpt-4o","object":"model","created":1760000000,"owned_by":"wso2"}]}`)

	anthropic := catalogFor(t, catalogBlock(FormatAnthropic), nil)
	assertJSONEqual(t, anthropic.Body, `{"data":[
		{"type":"model","id":"claude-haiku","display_name":"claude-haiku","created_at":"2025-10-09T08:53:20Z"},
		{"type":"model","id":"gpt-4o","display_name":"GPT-4o","created_at":"2025-10-09T08:53:20Z"}],
		"has_more":false,"first_id":"claude-haiku","last_id":"gpt-4o"}`)

	gemini := catalogFor(t, catalogBlock(FormatGemini), nil)
	assertJSONEqual(t, gemini.Body, `{"models":[
		{"name":"models/claude-haiku","displayName":"claude-haiku","supportedGenerationMethods":["generateContent","streamGenerateContent"]},
		{"name":"models/gpt-4o","displayName":"GPT-4o","supportedGenerationMethods":["generateContent","streamGenerateContent"]}]}`)

	empty := catalogFor(t, map[string]any{ParamCatalog: map[string]any{ParamFormat: FormatAnthropic}}, nil)
	assertJSONEqual(t, empty.Body, `{"data":[],"has_more":false,"first_id":null,"last_id":null}`)
}

// A key scoped to some Models sees only those; an endpoint key, or a caller
// authenticated some other way, sees every Model; an API-key caller whose scope
// was not recorded sees nothing.
func TestCatalogIsFilteredByTheKeysScope(t *testing.T) {
	ids := func(body []byte) []string {
		var doc struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, d := range doc.Data {
			out = append(out, d.ID)
		}
		return out
	}
	for name, tc := range map[string]struct {
		auth *policy.AuthContext
		want string
	}{
		"endpoint key":                {apiKeyAuth("*", true), "claude-haiku,gpt-4o"},
		"model key":                   {apiKeyAuth(`["claude","GET /models"]`, true), "claude-haiku"},
		"model key behind other auth": {&policy.AuthContext{Authenticated: true, AuthType: "jwt", Previous: apiKeyAuth(`["gpt"]`, true)}, "gpt-4o"},
		"legacy key":                  {apiKeyAuth("", true), "claude-haiku,gpt-4o"},
		"scope not recorded":          {apiKeyAuth("", false), ""},
		"unreadable scope":            {apiKeyAuth("claude", true), ""},
		"another kind of auth":        {&policy.AuthContext{Authenticated: true, AuthType: "jwt"}, "claude-haiku,gpt-4o"},
		"failed api key auth":         {&policy.AuthContext{Authenticated: false, AuthType: APIKeyAuthType}, "claude-haiku,gpt-4o"},
	} {
		t.Run(name, func(t *testing.T) {
			got := strings.Join(ids(catalogFor(t, catalogBlock(FormatOpenAI), tc.auth).Body), ",")
			if got != tc.want {
				t.Fatalf("catalog = %q, want %q", got, tc.want)
			}
		})
	}
}

func assertJSONEqual(t *testing.T, got []byte, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("body is not JSON: %v (%s)", err, got)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("want is not JSON: %v", err)
	}
	gb, _ := json.Marshal(g)
	wb, _ := json.Marshal(w)
	if string(gb) != string(wb) {
		t.Fatalf("body = %s\nwant  %s", gb, wb)
	}
}

// Every dispatch error is written in the endpoint's format.
func TestDispatchErrorsAreWrittenInTheEndpointsFormat(t *testing.T) {
	answer := func(format string) policy.ImmediateResponse {
		instance := mustPolicy(t, map[string]any{ParamDispatchError: map[string]any{ParamFormat: format}})
		return instance.(*dispatchErrorPolicy).OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
			SharedContext: &policy.SharedContext{
				ResolutionAttributes: policy.NewResolutionAttributes(map[string]string{AttrDispatchError: DispatchErrorModelNotFound}),
			},
		}, nil).(policy.ImmediateResponse)
	}

	anthropic := answer(FormatAnthropic)
	if anthropic.StatusCode != 404 {
		t.Fatalf("status = %d", anthropic.StatusCode)
	}
	var a struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(anthropic.Body, &a); err != nil || a.Type != "error" || a.Error.Type != "not_found_error" || a.Error.Message == "" {
		t.Fatalf("not an Anthropic error: %s", anthropic.Body)
	}

	gemini := answer(FormatGemini)
	var g struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	if err := json.Unmarshal(gemini.Body, &g); err != nil || g.Error.Code != 404 || g.Error.Status != "NOT_FOUND" {
		t.Fatalf("not a Gemini error: %s", gemini.Body)
	}

	// Absent is OpenAI, what a controller that predates the field compiles.
	assertOpenAIError(t, answer(FormatOpenAI), 404, "invalid_request_error", "model", DispatchErrorModelNotFound)
	instance := mustPolicy(t, map[string]any{ParamDispatchError: map[string]any{}}).(*dispatchErrorPolicy)
	if instance.format != FormatOpenAI {
		t.Fatalf("format = %q, want openai", instance.format)
	}
}

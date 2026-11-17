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
	"context"
	"encoding/json"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// locatedRoute is a two-target Model on an endpoint whose requests carry the model
// where location says, in format.
func locatedRoute(t *testing.T, location map[string]any, format string) *routePolicy {
	t.Helper()
	params := routeBlock(
		target("flash", "gemini-prod", "gemini-2.0-flash", "gemini", false),
		target("pro", "gemini-backup", "gemini-2.5-pro", "gemini", true),
	)
	block := params[ParamRoute].(map[string]any)
	block[ParamModelLocation] = location
	block[ParamFormat] = format
	return mustRoute(t, params)
}

func geminiRequest(path, selected string) *policy.RequestContext {
	ctx := requestContext(`{"contents":[{"parts":[{"text":"hi"}]}]}`, selected, nil)
	ctx.Path = path
	ctx.APIContext = "/gemini"
	return ctx
}

// A model carried in the path is rewritten in the path, every other byte of the
// path and the body left as the client sent them.
func TestThePathModelIsRewrittenForTheSelectedTarget(t *testing.T) {
	route := locatedRoute(t, map[string]any{"location": "pathParam", "identifier": `models/([a-zA-Z0-9.\-]+)`}, FormatGemini)

	ctx := geminiRequest("/gemini/v1beta/models/gemini-flash:streamGenerateContent?alt=sse", "gemini-backup")
	mods := mustForward(t, route.OnRequestBody(context.Background(), ctx, nil))
	if mods.Path == nil || *mods.Path != "/gemini/v1beta/models/gemini-2.5-pro:streamGenerateContent?alt=sse" {
		t.Fatalf("path = %v", mods.Path)
	}
	if mods.Body != nil {
		t.Fatalf("the body is not rewritten for a path model: %s", mods.Body)
	}
	if mods.UpstreamName == nil || *mods.UpstreamName != "gemini-backup" {
		t.Fatalf("UpstreamName = %v", mods.UpstreamName)
	}

	// With no router choice, the default target.
	ctx = geminiRequest("/gemini/v1/models/gemini-flash:generateContent", "")
	mods = mustForward(t, route.OnRequestBody(context.Background(), ctx, nil))
	if *mods.Path != "/gemini/v1/models/gemini-2.0-flash:generateContent" {
		t.Fatalf("path = %s", *mods.Path)
	}
}

// The context is not searched for the model: one that looks like the pattern is
// left alone.
func TestThePathModelIsFoundBelowTheContext(t *testing.T) {
	route := locatedRoute(t, map[string]any{"location": "pathParam", "identifier": `models/([a-zA-Z0-9.\-]+)`}, FormatGemini)
	ctx := geminiRequest("/models/x/v1beta/models/gemini-flash:generateContent", "")
	ctx.APIContext = "/models/x"
	mods := mustForward(t, route.OnRequestBody(context.Background(), ctx, nil))
	if *mods.Path != "/models/x/v1beta/models/gemini-2.0-flash:generateContent" {
		t.Fatalf("path = %s", *mods.Path)
	}
}

// A path that no longer names a model — a policy rewrote it — is refused, in the
// endpoint's format.
func TestAPathWithNoModelIsRefusedInTheEndpointsFormat(t *testing.T) {
	route := locatedRoute(t, map[string]any{"location": "pathParam", "identifier": `models/([a-zA-Z0-9.\-]+)`}, FormatGemini)
	action := route.OnRequestBody(context.Background(), geminiRequest("/gemini/v1beta/other", ""), nil)
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("expected a refusal, got %T", action)
	}
	var doc struct {
		Error struct {
			Code   int    `json:"code"`
			Status string `json:"status"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body, &doc); err != nil || doc.Error.Code != 500 || doc.Error.Status != "INTERNAL" {
		t.Fatalf("not a Gemini error: %s", response.Body)
	}
}

func TestHeaderAndQueryModelsAreRewritten(t *testing.T) {
	route := locatedRoute(t, map[string]any{"location": "header", "identifier": "X-Model"}, FormatOpenAI)
	ctx := requestContext(`{}`, "", map[string][]string{"x-model": {"gemini-2.5-pro"}})
	mods := mustForward(t, route.OnRequestBody(context.Background(), ctx, nil))
	if mods.HeadersToSet["x-model"] != "gemini-2.5-pro" || *mods.UpstreamName != "gemini-backup" {
		t.Fatalf("header rewrite = %v, upstream %v", mods.HeadersToSet, *mods.UpstreamName)
	}

	// A query model is replaced in place, so the provider sees the target's model
	// only, and every other parameter as the client sent it.
	route = locatedRoute(t, map[string]any{"location": "queryParam", "identifier": "model"}, FormatOpenAI)
	ctx = requestContext(`{}`, "", nil)
	ctx.Path = "/wso2/v1/chat/completions?api-version=2024-10-21&model=pro&tag=a%20b"
	mods = mustForward(t, route.OnRequestBody(context.Background(), ctx, nil))
	want := "/wso2/v1/chat/completions?api-version=2024-10-21&model=gemini-2.5-pro&tag=a%20b"
	if mods.Path == nil || *mods.Path != want {
		t.Fatalf("query rewrite path = %v, want %s", mods.Path, want)
	}
	if mods.QueryParametersToAdd != nil {
		t.Fatalf("QueryParametersToAdd = %v: the engine appends, so the client's value would be kept", mods.QueryParametersToAdd)
	}
	if *mods.UpstreamName != "gemini-backup" {
		t.Fatalf("upstream %v", *mods.UpstreamName)
	}
}

func TestReplaceQueryParameter(t *testing.T) {
	for _, c := range []struct{ raw, want string }{
		{"", "model=m"},
		{"model=x", "model=m"},
		{"a=1&model=x&b=2", "a=1&model=m&b=2"},
		{"a=1", "a=1&model=m"},
		{"model=x&model=y", "model=m"},
		{"mo%64el=x&a=%2F", "model=m&a=%2F"},
		{"modelx=1&a", "modelx=1&a&model=m"},
	} {
		if got := replaceQueryParameter(c.raw, "model", "m"); got != c.want {
			t.Errorf("replaceQueryParameter(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

// An Anthropic body is rewritten like an OpenAI one, and a body that is not JSON is
// refused in the Anthropic error shape.
func TestAnAnthropicBodyIsRefusedInItsShape(t *testing.T) {
	route := locatedRoute(t, map[string]any{"location": "payload", "identifier": "$.model"}, FormatAnthropic)
	action := route.OnRequestBody(context.Background(), requestContext(`[1]`, "", nil), nil)
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("expected a refusal, got %T", action)
	}
	var doc struct {
		Type  string `json:"type"`
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body, &doc); err != nil || doc.Type != "error" || doc.Error.Type != "invalid_request_error" {
		t.Fatalf("not an Anthropic error: %s", response.Body)
	}
}

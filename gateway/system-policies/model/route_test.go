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

// fourTargets is a Model whose targets exercise every resolution step: two on one
// provider, one sharing an upstream model with another provider, and a unique one.
func fourTargets() map[string]any {
	return routeBlock(
		target("default", "openai-prod", "gpt-4o", "openai", false),
		target("mini", "openai-prod", "gpt-4o-mini", "openai", false),
		target("azure", "azure-east", "gpt-4o", "azure-openai", true),
		target("claude", "anthropic", "claude-sonnet", "anthropic", false),
	)
}

func mustRoute(t *testing.T, params map[string]any) *routePolicy {
	t.Helper()
	route, ok := mustPolicy(t, params).(*routePolicy)
	if !ok {
		t.Fatal("route block did not produce the route job")
	}
	return route
}

// requestContext builds the body-phase context the kernel hands the route job,
// with whatever selected_provider the router left.
func requestContext(body, selectedProvider string, headers map[string][]string) *policy.RequestContext {
	shared := &policy.SharedContext{
		APIId:    "wso2-proxy-uuid",
		APIKind:  policy.APIKindLlmProxy,
		Metadata: map[string]any{MetadataTemplateHandle: "openai", MetadataProviderName: "__wso2_models__"},
	}
	if selectedProvider != "" {
		shared.Metadata[MetadataSelectedProvider] = selectedProvider
	}
	ctx := &policy.RequestContext{
		SharedContext: shared,
		Headers:       policy.NewHeaders(headers),
		Path:          "/wso2/v1/chat/completions",
		Method:        "POST",
	}
	if body != "" {
		ctx.Body = &policy.Body{Content: []byte(body), Present: true, EndOfStream: true}
	}
	return ctx
}

func mustForward(t *testing.T, action policy.RequestAction) policy.UpstreamRequestModifications {
	t.Helper()
	mods, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("expected the request to be forwarded, got %T", action)
	}
	return mods
}

// ─── target resolution ───────────────────────────────────────────────────────

func TestResolutionFollowsTheModelRoutingOrder(t *testing.T) {
	route := mustRoute(t, fourTargets())

	for name, tc := range map[string]struct {
		selectedProvider, model, want string
	}{
		// 1. The exact pair wins even where provider or model alone is ambiguous.
		"exact pair on a shared provider": {"openai-prod", "gpt-4o-mini", "mini"},
		"exact pair on a shared model":    {"azure-east", "gpt-4o", "azure"},

		// 2. A target's name, with no provider or with that target's own.
		"target name":                          {"", "mini", "mini"},
		"target name on a shared provider":     {"", "default", "default"},
		"target name on a shared model":        {"", "azure", "azure"},
		"target name with its own provider":    {"openai-prod", "mini", "mini"},
		"target name with another provider":    {"anthropic", "mini", "claude"},
		"target name with an unknown provider": {"bedrock", "azure", "default"},

		// 3. The provider alone, when it has exactly one target.
		"provider with one target": {"anthropic", "something-else", "claude"},
		"provider only":            {"azure-east", "", "azure"},

		// 4. The model alone, when exactly one target uses it.
		"model with one target":         {"", "claude-sonnet", "claude"},
		"model with one target, no key": {"", "gpt-4o-mini", "mini"},

		// A provider with several targets and a model it does not serve falls
		// through to the model step, then to the default.
		"provider ambiguous, model unique elsewhere": {"openai-prod", "claude-sonnet", "claude"},

		// 5. Everything else is the first target.
		"nothing chosen":                {"", "", "default"},
		"model ambiguous":               {"", "gpt-4o", "default"},
		"provider ambiguous, no model":  {"openai-prod", "", "default"},
		"provider outside the targets":  {"bedrock", "", "default"},
		"model outside the targets":     {"", "llama-3", "default"},
		"pair outside the targets":      {"bedrock", "llama-3", "default"},
		"client model name, no router":  {"", "chat", "default"},
		"provider outside, model match": {"bedrock", "claude-sonnet", "claude"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := route.cfg.resolve(tc.selectedProvider, tc.model).name; got != tc.want {
				t.Fatalf("resolve(%q, %q) = %q, want %q", tc.selectedProvider, tc.model, got, tc.want)
			}
		})
	}
}

// A value that is one target's name and another target's upstream model goes to
// the target it names: names are unique, so that match is never ambiguous.
func TestATargetNameComesBeforeAnUpstreamModel(t *testing.T) {
	route := mustRoute(t, routeBlock(
		target("gpt-4o-mini", "anthropic", "claude-sonnet", "anthropic", false),
		target("mini", "openai-prod", "gpt-4o-mini", "openai", false),
	))
	if got := route.cfg.resolve("", "gpt-4o-mini").name; got != "gpt-4o-mini" {
		t.Fatalf("resolve(\"\", \"gpt-4o-mini\") = %q, want the target named gpt-4o-mini", got)
	}
	// The exact pair still comes first.
	if got := route.cfg.resolve("openai-prod", "gpt-4o-mini").name; got != "mini" {
		t.Fatalf("resolve(\"openai-prod\", \"gpt-4o-mini\") = %q, want mini", got)
	}
}

// A router that names a target by its name gets that target, with the target's
// upstream model written into the request: the provider never sees the name.
func TestARouterChoiceNamingATargetIsRewrittenForIt(t *testing.T) {
	route := mustRoute(t, fourTargets())
	ctx := requestContext(`{"model":"claude","messages":[]}`, "", nil)

	mods := mustForward(t, route.OnRequestBody(context.Background(), ctx, nil))

	if mods.UpstreamName == nil || *mods.UpstreamName != "anthropic" {
		t.Fatalf("UpstreamName = %v, want anthropic", mods.UpstreamName)
	}
	if got := string(mods.Body); got != `{"model":"claude-sonnet","messages":[]}` {
		t.Fatalf("body = %s", got)
	}
	if got := ctx.Metadata[MetadataModelTarget]; got != "claude" {
		t.Fatalf("model.target = %v, want claude", got)
	}
}

// A choice that names no target is never forwarded: the upstream and the body are
// rewritten from the target resolution chose, whatever the router said.
func TestAChoiceOutsideTheTargetsIsNeverForwarded(t *testing.T) {
	route := mustRoute(t, fourTargets())
	ctx := requestContext(`{"model":"llama-3","messages":[]}`, "bedrock", nil)

	mods := mustForward(t, route.OnRequestBody(context.Background(), ctx, nil))

	if mods.UpstreamName == nil || *mods.UpstreamName != "openai-prod" {
		t.Fatalf("UpstreamName = %v, want openai-prod", mods.UpstreamName)
	}
	if got := ctx.Metadata[MetadataSelectedProvider]; got != "openai-prod" {
		t.Fatalf("selected_provider = %v, want openai-prod", got)
	}
	if got := string(mods.Body); got != `{"model":"gpt-4o","messages":[]}` {
		t.Fatalf("body = %s", got)
	}
}

// ─── the request rewrite ─────────────────────────────────────────────────────

func TestRequestIsRewrittenForTheSelectedTarget(t *testing.T) {
	route := mustRoute(t, fourTargets())
	ctx := requestContext(`{"model":"claude-sonnet","stream":true}`, "anthropic", nil)

	mods := mustForward(t, route.OnRequestBody(context.Background(), ctx, nil))

	if mods.UpstreamName == nil || *mods.UpstreamName != "anthropic" {
		t.Fatalf("UpstreamName = %v, want anthropic", mods.UpstreamName)
	}
	want := map[string]any{
		MetadataSelectedProvider:    "anthropic",
		MetadataTemplateHandle:      "anthropic",
		MetadataProviderName:        "anthropic",
		MetadataModelHandle:         "chat-default",
		MetadataModelName:           "chat",
		MetadataModelTarget:         "claude",
		MetadataProviderHandle:      "anthropic",
		MetadataProviderTemplate:    "anthropic",
		MetadataProviderDisplayName: "ANTHROPIC",
		MetadataUpstreamModel:       "claude-sonnet",
	}
	for key, value := range want {
		if got := ctx.Metadata[key]; got != value {
			t.Errorf("metadata[%q] = %v, want %v", key, got, value)
		}
	}
}

// Only the model's value changes. Member order, spacing, number spellings and
// string escapes are the client's, and a round trip through a map would rewrite
// all of them.
func TestRewritePreservesEveryOtherByte(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		"member order and spacing": {
			in:   "{ \"messages\" : [ {\"role\":\"user\",\"content\":\"<b>hi</b> \\u00e9\"} ],\n  \"model\" :  \"chat\", \"temperature\": 0.70, \"n\":1e0 }",
			want: "{ \"messages\" : [ {\"role\":\"user\",\"content\":\"<b>hi</b> \\u00e9\"} ],\n  \"model\" :  \"gpt-4o\", \"temperature\": 0.70, \"n\":1e0 }",
		},
		"nested model untouched": {
			in:   `{"metadata":{"model":"keep"},"model":"chat"}`,
			want: `{"metadata":{"model":"keep"},"model":"gpt-4o"}`,
		},
		"repeated member, every occurrence": {
			in:   `{"model":"chat","x":1,"model":"other"}`,
			want: `{"model":"gpt-4o","x":1,"model":"gpt-4o"}`,
		},
		"non-string model": {
			in:   `{"model":42,"messages":[]}`,
			want: `{"model":"gpt-4o","messages":[]}`,
		},
		"absent model": {
			in:   `{"messages":[]}`,
			want: `{"model":"gpt-4o","messages":[]}`,
		},
		"empty object": {
			in:   ` { } `,
			want: ` {"model":"gpt-4o" } `,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := rewriteTopLevelMember([]byte(tc.in), "model", "gpt-4o")
			if err != nil {
				t.Fatalf("rewrite: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("rewrite =\n%s\nwant\n%s", got, tc.want)
			}
			if !json.Valid(got) {
				t.Fatalf("rewrite produced invalid JSON: %s", got)
			}
		})
	}
}

// The upstream model is the controller's value but is still encoded, never
// spliced raw, so a name containing a quote cannot break out of the string.
func TestRewriteEncodesTheUpstreamModel(t *testing.T) {
	got, err := rewriteTopLevelMember([]byte(`{"model":"chat"}`), "model", `a"b`)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if string(got) != `{"model":"a\"b"}` {
		t.Fatalf("rewrite = %s", got)
	}
}

func TestTopLevelModelReadsTheLastStringOccurrence(t *testing.T) {
	for in, want := range map[string]string{
		`{"model":"a"}`:                  "a",
		`{"model":"a","model":"b"}`:      "b",
		`{"model":"a","model":7}`:        "",
		`{"other":{"model":"nested"}}`:   "",
		`{"mod\u0065l":"escaped-key"}`:   "escaped-key",
		"{\n\t\"model\" : \"spaced\"\n}": "spaced",
	} {
		got, ok := topLevelMember([]byte(in), "model")
		if !ok {
			t.Errorf("topLevelMember(%s) refused a JSON object", in)
			continue
		}
		if got != want {
			t.Errorf("topLevelMember(%s) = %q, want %q", in, got, want)
		}
	}
}

// A body that is not one JSON object is refused with an OpenAI-shaped 400 rather
// than forwarded to a provider the job could not point it at.
func TestABodyThatIsNotAJSONObjectIsRefused(t *testing.T) {
	route := mustRoute(t, fourTargets())
	for name, body := range map[string]string{
		"no body":            "",
		"invalid JSON":       `{"model":`,
		"array":              `[{"model":"chat"}]`,
		"string":             `"chat"`,
		"two values":         `{"model":"chat"} {}`,
		"whitespace only":    "   ",
		"trailing garbage":   `{"model":"chat"}x`,
		"object then object": `{}{}`,
	} {
		t.Run(name, func(t *testing.T) {
			action := route.OnRequestBody(context.Background(), requestContext(body, "", nil), nil)
			response, ok := action.(policy.ImmediateResponse)
			if !ok {
				t.Fatalf("expected a refusal, got %T", action)
			}
			assertOpenAIError(t, response, 400, "invalid_request_error", "", DispatchErrorInvalidJSON)
		})
	}
}

func TestARouteJobWithoutAContextFailsClosed(t *testing.T) {
	route := mustRoute(t, fourTargets())
	for _, ctx := range []*policy.RequestContext{nil, {Body: &policy.Body{Content: []byte(`{}`)}}} {
		response, ok := route.OnRequestBody(context.Background(), ctx, nil).(policy.ImmediateResponse)
		if !ok {
			t.Fatal("expected a refusal")
		}
		assertOpenAIError(t, response, 500, "server_error", "", "")
	}
}

// ─── the loopback marker ─────────────────────────────────────────────────────

// A direct target goes straight to the provider, so a marker the client sent is
// removed: it would otherwise reach a provider route as a claim that the gateway
// sent it. A routed target is left alone, because the conditional policy after
// this job sets the gateway's own marker, and removing it here in the same phase
// would race that write.
func TestTheLoopbackMarkerIsRemovedForDirectTargetsOnly(t *testing.T) {
	route := mustRoute(t, fourTargets())
	marker := map[string][]string{InternalLoopbackHeader: {"1"}}

	direct := mustForward(t, route.OnRequestBody(context.Background(),
		requestContext(`{"model":"claude-sonnet"}`, "anthropic", marker), nil))
	if len(direct.HeadersToRemove) != 1 || direct.HeadersToRemove[0] != InternalLoopbackHeader {
		t.Fatalf("direct target HeadersToRemove = %v, want [%s]", direct.HeadersToRemove, InternalLoopbackHeader)
	}

	routed := mustForward(t, route.OnRequestBody(context.Background(),
		requestContext(`{"model":"gpt-4o"}`, "azure-east", marker), nil))
	if len(routed.HeadersToRemove) != 0 {
		t.Fatalf("routed target HeadersToRemove = %v, want none", routed.HeadersToRemove)
	}
}

// ─── provider scope ──────────────────────────────────────────────────────────

// scopeReader stands in for a provider-specific policy (llm-cost) and a Model
// policy (token-based-ratelimit) between and outside the bracket: it records the
// scope it saw.
type scopeReader struct{ seen []string }

func (r *scopeReader) read(shared *policy.SharedContext) {
	r.seen = append(r.seen, metadataString(shared, MetadataTemplateHandle)+"/"+metadataString(shared, MetadataProviderName))
}

// The bracket gives the provider's policies the provider's scope and everything
// else the OpenAI scope, in every response phase and on every chunk.
//
// The chain is walked the way the kernel walks a response: in reverse, so the
// providerScope job (last in the chain) runs first and the route job after the
// provider-specific policies.
func TestTheBracketScopesEveryResponsePhase(t *testing.T) {
	route := mustRoute(t, fourTargets())
	scope := mustPolicy(t, map[string]any{ParamProviderScope: map[string]any{}}).(*providerScopePolicy)

	shared := requestContext(`{"model":"claude-sonnet"}`, "anthropic", nil).SharedContext
	reqCtx := &policy.RequestContext{SharedContext: shared,
		Body: &policy.Body{Content: []byte(`{"model":"claude-sonnet"}`), Present: true}}
	mustForward(t, route.OnRequestBody(context.Background(), reqCtx, nil))

	// Request side: everything after the route job reads the provider's scope.
	inside, outside := &scopeReader{}, &scopeReader{}
	inside.read(shared)

	// Response headers.
	scope.OnResponseHeaders(context.Background(), &policy.ResponseHeaderContext{SharedContext: shared}, nil)
	inside.read(shared)
	route.OnResponseHeaders(context.Background(), &policy.ResponseHeaderContext{SharedContext: shared}, nil)
	outside.read(shared)

	// A buffered body.
	scope.OnResponseBody(context.Background(), &policy.ResponseContext{SharedContext: shared}, nil)
	inside.read(shared)
	route.OnResponseBody(context.Background(), &policy.ResponseContext{SharedContext: shared}, nil)
	outside.read(shared)

	// A stream of three chunks.
	for i := 0; i < 3; i++ {
		chunk := &policy.StreamBody{Chunk: []byte("data: {}\n\n"), Index: uint64(i), EndOfStream: i == 2}
		streamCtx := &policy.ResponseStreamContext{SharedContext: shared}
		if _, ok := scope.OnResponseBodyChunk(context.Background(), streamCtx, chunk, nil).(policy.ForwardResponseChunk); !ok {
			t.Fatal("providerScope must forward every chunk unchanged")
		}
		inside.read(shared)
		if _, ok := route.OnResponseBodyChunk(context.Background(), streamCtx, chunk, nil).(policy.ForwardResponseChunk); !ok {
			t.Fatal("route must forward every chunk unchanged")
		}
		outside.read(shared)
	}

	for i, seen := range inside.seen {
		if seen != "anthropic/anthropic" {
			t.Errorf("provider-specific read %d saw scope %q, want anthropic/anthropic", i, seen)
		}
	}
	for i, seen := range outside.seen {
		if seen != "openai/__wso2_models__" {
			t.Errorf("OpenAI-scope read %d saw scope %q, want openai/__wso2_models__", i, seen)
		}
	}
	if len(inside.seen) != 6 || len(outside.seen) != 5 {
		t.Fatalf("reads = %d inside, %d outside; want 6 and 5", len(inside.seen), len(outside.seen))
	}
}

// A request refused before the route job ran has no target, and its response is
// the gateway's own, in the OpenAI shape — so providerScope leaves the scope as
// the route seeded it.
func TestProviderScopeIsANoOpWithoutASelectedTarget(t *testing.T) {
	scope := mustPolicy(t, map[string]any{ParamProviderScope: map[string]any{}}).(*providerScopePolicy)
	shared := &policy.SharedContext{Metadata: map[string]any{
		MetadataTemplateHandle: "openai",
		MetadataProviderName:   "__wso2_models__",
	}}

	scope.OnResponseHeaders(context.Background(), &policy.ResponseHeaderContext{SharedContext: shared}, nil)
	scope.OnResponseBody(context.Background(), &policy.ResponseContext{SharedContext: shared}, nil)
	scope.OnResponseBodyChunk(context.Background(), &policy.ResponseStreamContext{SharedContext: shared},
		&policy.StreamBody{EndOfStream: true}, nil)

	if got := metadataString(shared, MetadataTemplateHandle); got != "openai" {
		t.Fatalf("template_handle = %q, want openai", got)
	}
	if got := metadataString(shared, MetadataProviderName); got != "__wso2_models__" {
		t.Fatalf("provider_name = %q, want __wso2_models__", got)
	}

	// No metadata at all, and no context at all, are tolerated.
	scope.OnResponseHeaders(context.Background(), &policy.ResponseHeaderContext{SharedContext: &policy.SharedContext{}}, nil)
	scope.OnResponseHeaders(context.Background(), nil, nil)
	scope.OnResponseBody(context.Background(), nil, nil)
	scope.OnResponseBodyChunk(context.Background(), nil, &policy.StreamBody{}, nil)
}

func TestScopeJobsNeverAccumulate(t *testing.T) {
	route := mustRoute(t, fourTargets())
	scope := mustPolicy(t, map[string]any{ParamProviderScope: map[string]any{}}).(*providerScopePolicy)
	if route.NeedsMoreResponseData([]byte("data:")) || scope.NeedsMoreResponseData([]byte("data:")) {
		t.Fatal("a scope job must never hold back a chunk")
	}
}

// ─── the header-phase commitment ─────────────────────────────────────────────

// headerPhase runs the route job's header phase the way the kernel's header pass
// does — after the router's — and returns the shared context it left.
func headerPhase(t *testing.T, route *routePolicy, selectedProvider string) *policy.SharedContext {
	t.Helper()
	shared := &policy.SharedContext{
		APIId:    "wso2-proxy-uuid",
		APIKind:  policy.APIKindLlmProxy,
		Metadata: map[string]any{MetadataTemplateHandle: "openai", MetadataProviderName: "__wso2_models__"},
	}
	if selectedProvider != "" {
		shared.Metadata[MetadataSelectedProvider] = selectedProvider
	}
	action := route.OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
		SharedContext: shared,
		Headers:       policy.NewHeaders(nil),
		Path:          "/wso2/v1/chat/completions",
		Method:        "POST",
	}, nil)
	mods, ok := action.(policy.UpstreamRequestHeaderModifications)
	if !ok {
		t.Fatalf("the header phase must never terminate the request, got %T", action)
	}
	if mods.UpstreamName != nil || mods.HeadersToSet != nil || mods.HeadersToRemove != nil || mods.Path != nil {
		t.Fatalf("the header phase must not mutate the request, got %+v", mods)
	}
	// The scope is the body phase's to switch.
	if got := metadataString(shared, MetadataTemplateHandle); got != "openai" {
		t.Fatalf("template_handle = %q after the header phase, want openai", got)
	}
	if got := metadataString(shared, MetadataProviderName); got != "__wso2_models__" {
		t.Fatalf("provider_name = %q after the header phase, want __wso2_models__", got)
	}
	return shared
}

// bodyPhase runs the route job's body phase on the context the header phase left,
// with selected_provider as a body-phase router would leave it ("" leaves it as is).
func bodyPhase(route *routePolicy, shared *policy.SharedContext, body, selectedProvider string) policy.RequestAction {
	if selectedProvider != "" {
		shared.Metadata[MetadataSelectedProvider] = selectedProvider
	}
	return route.OnRequestBody(context.Background(), &policy.RequestContext{
		SharedContext: shared,
		Headers:       policy.NewHeaders(nil),
		Body:          &policy.Body{Content: []byte(body), Present: true, EndOfStream: true},
		Path:          "/wso2/v1/chat/completions",
		Method:        "POST",
	}, nil)
}

// A Model whose targets are all on one provider has nothing to decide, so its
// provider is committed in the header phase, where a header-phase credential needs
// it.
func TestASingleProviderModelCommitsInTheHeaderPhase(t *testing.T) {
	route := mustRoute(t, routeBlock(
		target("default", "openai-prod", "gpt-4o", "openai", false),
		target("mini", "openai-prod", "gpt-4o-mini", "openai", false),
	))

	shared := headerPhase(t, route, "")
	if got := shared.Metadata[MetadataSelectedProvider]; got != "openai-prod" {
		t.Fatalf("selected_provider = %v after the header phase, want openai-prod", got)
	}

	mods := mustForward(t, bodyPhase(route, shared, `{"model":"gpt-4o-mini"}`, ""))
	if mods.UpstreamName == nil || *mods.UpstreamName != "openai-prod" {
		t.Fatalf("UpstreamName = %v, want openai-prod", mods.UpstreamName)
	}
	if got := shared.Metadata[MetadataModelTarget]; got != "mini" {
		t.Fatalf("target = %v, want mini", got)
	}
}

// A router that decides from headers has named the provider by the time the route
// job's header phase runs, so that choice is committed.
func TestAHeaderPhaseRouterSelectionIsCommitted(t *testing.T) {
	route := mustRoute(t, fourTargets())

	shared := headerPhase(t, route, "anthropic")
	if got := shared.Metadata[MetadataSelectedProvider]; got != "anthropic" {
		t.Fatalf("selected_provider = %v after the header phase, want anthropic", got)
	}

	mods := mustForward(t, bodyPhase(route, shared, `{"model":"claude-sonnet"}`, ""))
	if mods.UpstreamName == nil || *mods.UpstreamName != "anthropic" {
		t.Fatalf("UpstreamName = %v, want anthropic", mods.UpstreamName)
	}
}

// With several providers and no header-phase choice — or a choice naming none of
// the Model's providers — nothing is known yet, so nothing is committed and the
// body phase decides alone.
func TestAnAmbiguousModelCommitsNothingInTheHeaderPhase(t *testing.T) {
	route := mustRoute(t, fourTargets())

	for name, selected := range map[string]string{
		"no selection":             "",
		"provider outside targets": "bedrock",
	} {
		t.Run(name, func(t *testing.T) {
			shared := headerPhase(t, route, selected)
			if _, committed := shared.Metadata[metadataHeaderProvider]; committed {
				t.Fatalf("committed %v with no provider known", shared.Metadata[metadataHeaderProvider])
			}
			if got := metadataString(shared, MetadataSelectedProvider); got != selected {
				t.Fatalf("selected_provider = %q, want it left as %q", got, selected)
			}

			// The body phase is then free to choose any target.
			mods := mustForward(t, bodyPhase(route, shared, `{"model":"gpt-4o"}`, "azure-east"))
			if mods.UpstreamName == nil || *mods.UpstreamName != "azure-east" {
				t.Fatalf("UpstreamName = %v, want azure-east", mods.UpstreamName)
			}
		})
	}
}

// A body phase that moves off the committed provider would send that provider's
// header-phase credential to another upstream, so the request is refused and
// nothing is routed.
func TestABodyPhaseSwitchAwayFromTheCommittedProviderFailsClosed(t *testing.T) {
	route := mustRoute(t, fourTargets())

	shared := headerPhase(t, route, "anthropic")
	action := bodyPhase(route, shared, `{"model":"gpt-4o"}`, "azure-east")

	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("expected a refusal, got %T", action)
	}
	assertOpenAIError(t, response, 500, "server_error", "", "routing_conflict")
	if _, routed := shared.Metadata[MetadataModelTarget]; routed {
		t.Fatalf("a refused request recorded a target: %v", shared.Metadata[MetadataModelTarget])
	}
	if got := metadataString(shared, MetadataTemplateHandle); got != "openai" {
		t.Fatalf("a refused request switched the scope to %q", got)
	}
}

// A context-less header phase commits nothing and does not panic; the body phase
// refuses such a request.
func TestAHeaderPhaseWithoutAContextCommitsNothing(t *testing.T) {
	route := mustRoute(t, fourTargets())
	for _, ctx := range []*policy.RequestHeaderContext{nil, {}} {
		if _, ok := route.OnRequestHeaders(context.Background(), ctx, nil).(policy.UpstreamRequestHeaderModifications); !ok {
			t.Fatal("the header phase must never terminate the request")
		}
	}
}

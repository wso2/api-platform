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

package resolver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/chainkey"
)

const (
	modelsAPIID    = "wso2-proxy-uuid"
	modelsVhost    = "*"
	modelsRouteKey = "POST|/wso2/v1/chat/completions|*"
	modelsErrorOp  = "__dispatch__"
)

// modelsConfig encodes a resolver_config through the exported wire type, so a rename on
// this side breaks the test rather than silently producing a config Prepare ignores.
func modelsConfig(t *testing.T, models map[string]string, errorOp string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(LLMModelDispatchConfig{Models: models, ErrorOperation: errorOp})
	require.NoError(t, err)
	return raw
}

func modelsRoute(cfg json.RawMessage) ResolverRouteConfig {
	return ResolverRouteConfig{
		RouteKey:          modelsRouteKey,
		CanonicalChainKey: modelsRouteKey,
		ResolverName:      LLMModelDispatchResolverName,
		APIID:             modelsAPIID,
		Vhost:             modelsVhost,
		APIContext:        "/wso2/v1",
		Method:            "POST",
		Path:              "/chat/completions",
		ResolverConfig:    cfg,
	}
}

// modelsDispatchRoute prepares the chat-completions route of a proxy serving two Models.
func modelsDispatchRoute(t *testing.T) *PreparedRoute {
	t.Helper()
	return prepareWith(t, DefaultRegistry(), modelsRoute(modelsConfig(t, map[string]string{
		"gpt-4o":          "gpt4o-model",
		"claude-sonnet-4": "claude-model",
	}, modelsErrorOp)))
}

// resolveModels runs one request against pr and asserts the result is bindable under the
// route's own partition — every outcome of this resolver is a chain, so every one must
// pass the same validation the binder applies.
func resolveModels(t *testing.T, pr *PreparedRoute, body []byte) Resolution {
	t.Helper()
	res, err := pr.Resolver.Resolve(context.Background(), RequestView{
		RouteKey: modelsRouteKey,
		Method:   "POST",
		Path:     "/wso2/v1/chat/completions",
		Body:     body,
	})
	require.NoError(t, err, "model dispatch never fails a request; it binds the error chain instead")
	require.Nil(t, pr.ValidateResolution(res), "every key must stay inside the route's partition")
	return res
}

var (
	modelsErrorKey  = chainkey.For(modelsAPIID, modelsVhost, modelsErrorOp)
	gpt4oModelKey   = chainkey.For(modelsAPIID, modelsVhost, "gpt4o-model")
	claudeModelKey  = chainkey.For(modelsAPIID, modelsVhost, "claude-model")
	oversizedName   = strings.Repeat("m", MaxLLMModelNameBytes+1)
	maxLengthedName = strings.Repeat("m", MaxLLMModelNameBytes)
)

// ─── Registration ────────────────────────────────────────────────────────────

func TestLLMModelDispatch_IsRegistered(t *testing.T) {
	r, ok := DefaultRegistry().Get(LLMModelDispatchResolverName)
	require.True(t, ok, "the controller writes this name into the models-mode proxy's route")
	assert.Equal(t, "llm-model-dispatch", r.Name())
	assert.IsType(t, &LLMModelDispatchResolver{}, r)
}

func TestLLMModelDispatch_BuffersTheBody(t *testing.T) {
	pr := modelsDispatchRoute(t)
	assert.Equal(t, RequestRequirements{Body: BodyBuffered}, pr.Requirements)
	assert.False(t, pr.IsStatic(), "the model is in the body, so nothing is known at ingest")
	assert.False(t, pr.ValidatesHeaders())
}

// ─── Prepare ─────────────────────────────────────────────────────────────────

func TestLLMModelDispatchPrepare_RejectsUnusableConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  string
		want string
	}{
		{"no config", "", "no resolver_config"},
		{"not JSON", `{`, "not valid JSON"},
		{"no error operation", `{"models":{"a":"b"}}`, "errorOperation"},
		{"error operation with a separator", `{"models":{},"errorOperation":"x\u001fy"}`, "errorOperation"},
		{"empty model name", `{"models":{"":"b"},"errorOperation":"__dispatch__"}`, "empty or longer"},
		{"oversized model name", `{"models":{"` + oversizedName + `":"b"},"errorOperation":"__dispatch__"}`, "empty or longer"},
		{"empty operation", `{"models":{"a":""},"errorOperation":"__dispatch__"}`, "unusable operation"},
		{"operation with a separator", `{"models":{"a":"b\u001fc"},"errorOperation":"__dispatch__"}`, "unusable operation"},
		{"model mapped to the error operation", `{"models":{"a":"__dispatch__"},"errorOperation":"__dispatch__"}`, "reserved error operation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PrepareRoute(DefaultRegistry(), modelsRoute(json.RawMessage(tt.cfg)))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestLLMModelDispatchPrepare_RejectsAnUnusablePartition(t *testing.T) {
	cfg := modelsConfig(t, map[string]string{"a": "b"}, modelsErrorOp)

	route := modelsRoute(cfg)
	route.APIID = ""
	_, err := PrepareRoute(DefaultRegistry(), route)
	require.ErrorContains(t, err, "unusable API id")

	route = modelsRoute(cfg)
	route.Vhost = "a" + chainkey.Separator + "b"
	_, err = PrepareRoute(DefaultRegistry(), route)
	require.ErrorContains(t, err, "unusable vhost")
}

// A proxy with no Models deployed is a valid route, not a broken one: it answers every
// request from the error chain, after authentication.
func TestLLMModelDispatchPrepare_AcceptsNoModels(t *testing.T) {
	for _, cfg := range []string{`{"errorOperation":"__dispatch__"}`, `{"models":{},"errorOperation":"__dispatch__"}`} {
		pr := prepareWith(t, DefaultRegistry(), modelsRoute(json.RawMessage(cfg)))
		res := resolveModels(t, pr, []byte(`{"model":"gpt-4o"}`))
		assert.Equal(t, modelsErrorKey, res.ChainKey)
		assert.Equal(t, LLMDispatchModelNotFound, res.Attributes[AttrLLMDispatchError])
	}
}

// A newer controller may add a field; a route this binary can still serve is not dropped.
func TestLLMModelDispatchPrepare_ToleratesUnknownFields(t *testing.T) {
	pr := prepareWith(t, DefaultRegistry(), modelsRoute(json.RawMessage(
		`{"models":{"gpt-4o":"gpt4o-model"},"errorOperation":"__dispatch__","futureField":true}`)))
	assert.Equal(t, gpt4oModelKey, resolveModels(t, pr, []byte(`{"model":"gpt-4o"}`)).ChainKey)
}

// ─── Resolve: dispatch ───────────────────────────────────────────────────────

func TestLLMModelDispatch_BindsTheNamedModelsChain(t *testing.T) {
	pr := modelsDispatchRoute(t)

	res := resolveModels(t, pr, []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
	assert.Equal(t, gpt4oModelKey, res.ChainKey)
	assert.Equal(t, map[string]string{
		AttrLLMModelName:     "gpt-4o",
		AttrLLMModelHandle:   "gpt4o-model",
		AttrLLMRequestStream: "false",
	}, res.Attributes)

	res = resolveModels(t, pr, []byte(" \n\t{\"stream\":true,\"model\":\"claude-sonnet-4\"}"))
	assert.Equal(t, claudeModelKey, res.ChainKey)
	assert.Equal(t, "claude-model", res.Attributes[AttrLLMModelHandle])
	assert.Equal(t, "true", res.Attributes[AttrLLMRequestStream])
	assert.NotContains(t, res.Attributes, AttrLLMDispatchError)
}

// The bound key and the published handle must describe the same Model, or telemetry
// would name one Model while another's policies ran.
func TestLLMModelDispatch_HandleMatchesTheBoundOperation(t *testing.T) {
	pr := modelsDispatchRoute(t)
	for _, name := range []string{"gpt-4o", "claude-sonnet-4"} {
		res := resolveModels(t, pr, []byte(`{"model":"`+name+`"}`))
		_, _, operation, ok := chainkey.Split(res.ChainKey)
		require.True(t, ok)
		assert.Equal(t, operation, res.Attributes[AttrLLMModelHandle])
	}
}

func TestLLMModelDispatch_StreamAttribute(t *testing.T) {
	pr := modelsDispatchRoute(t)
	tests := []struct {
		body string
		want string
	}{
		{`{"model":"gpt-4o","stream":true}`, "true"},
		{`{"model":"gpt-4o","stream": true }`, "true"},
		{`{"model":"gpt-4o","stream":false}`, "false"},
		{`{"model":"gpt-4o"}`, "false"},
		{`{"model":"gpt-4o","stream":"true"}`, "false"},
		{`{"model":"gpt-4o","stream":1}`, "false"},
		{`{"model":"gpt-4o","stream":null}`, "false"},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveModels(t, pr, []byte(tt.body)).Attributes[AttrLLMRequestStream])
		})
	}
}

// An ambiguous stream member is left unstated rather than guessed: the upstream may read
// it either way.
func TestLLMModelDispatch_AmbiguousStreamIsNotPublished(t *testing.T) {
	pr := modelsDispatchRoute(t)
	for _, body := range []string{
		`{"model":"gpt-4o","stream":false,"stream":true}`,
		`{"model":"gpt-4o","Stream":true}`,
	} {
		res := resolveModels(t, pr, []byte(body))
		assert.Equal(t, gpt4oModelKey, res.ChainKey, "the model itself is unambiguous")
		assert.NotContains(t, res.Attributes, AttrLLMRequestStream)
	}
}

// ─── Resolve: the error chain ────────────────────────────────────────────────

func TestLLMModelDispatch_BindsTheErrorChainForUnusableRequests(t *testing.T) {
	pr := modelsDispatchRoute(t)
	tests := []struct {
		name       string
		body       []byte
		reason     string
		wantStream bool
	}{
		{"nil body", nil, LLMDispatchInvalidJSON, false},
		{"empty body", []byte{}, LLMDispatchInvalidJSON, false},
		{"whitespace only", []byte(" \n "), LLMDispatchInvalidJSON, false},
		{"truncated JSON", []byte(`{"model":"gpt-4o"`), LLMDispatchInvalidJSON, false},
		{"trailing garbage", []byte(`{"model":"gpt-4o"}x`), LLMDispatchInvalidJSON, false},
		{"batch array", []byte(`[{"model":"gpt-4o"}]`), LLMDispatchInvalidJSON, false},
		{"top-level string", []byte(`"gpt-4o"`), LLMDispatchInvalidJSON, false},
		{"top-level number", []byte(`42`), LLMDispatchInvalidJSON, false},
		{"no model", []byte(`{"messages":[]}`), LLMDispatchMissingModel, true},
		{"null model", []byte(`{"model":null}`), LLMDispatchMissingModel, true},
		{"empty model", []byte(`{"model":""}`), LLMDispatchMissingModel, true},
		{"numeric model", []byte(`{"model":4}`), LLMDispatchInvalidModel, true},
		{"object model", []byte(`{"model":{"name":"gpt-4o"}}`), LLMDispatchInvalidModel, true},
		{"array model", []byte(`{"model":["gpt-4o"]}`), LLMDispatchInvalidModel, true},
		{"boolean model", []byte(`{"model":true}`), LLMDispatchInvalidModel, true},
		{"duplicate model", []byte(`{"model":"gpt-4o","model":"claude-sonnet-4"}`), LLMDispatchInvalidModel, true},
		{"folded spelling", []byte(`{"Model":"gpt-4o"}`), LLMDispatchInvalidModel, true},
		{"folded duplicate", []byte(`{"model":"gpt-4o","MODEL":"claude-sonnet-4"}`), LLMDispatchInvalidModel, true},
		{"unknown model", []byte(`{"model":"gpt-5"}`), LLMDispatchModelNotFound, true},
		{"case near-miss", []byte(`{"model":"GPT-4o"}`), LLMDispatchModelNotFound, true},
		{"handle instead of name", []byte(`{"model":"gpt4o-model"}`), LLMDispatchModelNotFound, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := resolveModels(t, pr, tt.body)
			assert.Equal(t, modelsErrorKey, res.ChainKey)
			assert.Equal(t, tt.reason, res.Attributes[AttrLLMDispatchError])
			assert.NotContains(t, res.Attributes, AttrLLMModelHandle,
				"no Model was bound, so no handle may be claimed")
			if tt.wantStream {
				assert.Equal(t, "false", res.Attributes[AttrLLMRequestStream])
			} else {
				assert.NotContains(t, res.Attributes, AttrLLMRequestStream)
			}
		})
	}
}

func TestLLMModelDispatch_ModelNotFoundCarriesTheRequestedName(t *testing.T) {
	pr := modelsDispatchRoute(t)

	res := resolveModels(t, pr, []byte(`{"model":"gpt-5","stream":true}`))
	assert.Equal(t, map[string]string{
		AttrLLMDispatchError: LLMDispatchModelNotFound,
		AttrLLMModelName:     "gpt-5",
		AttrLLMRequestStream: "true",
	}, res.Attributes)

	res = resolveModels(t, pr, []byte(`{"model":"`+maxLengthedName+`"}`))
	assert.Equal(t, maxLengthedName, res.Attributes[AttrLLMModelName], "a name at the cap is published")
}

// A name over the cap could never have been deployed, so it is not looked up and never
// copied into an attribute — the value is caller-controlled and unauthenticated here.
func TestLLMModelDispatch_OversizedNameIsNotFoundAndNotPublished(t *testing.T) {
	pr := modelsDispatchRoute(t)
	res := resolveModels(t, pr, []byte(`{"model":"`+oversizedName+`"}`))
	assert.Equal(t, modelsErrorKey, res.ChainKey)
	assert.Equal(t, LLMDispatchModelNotFound, res.Attributes[AttrLLMDispatchError])
	assert.NotContains(t, res.Attributes, AttrLLMModelName)
}

// Every outcome binds through the ordinary binder, against the chains the controller
// emits for the proxy: one per Model plus the error chain.
func TestLLMModelDispatch_BindsThroughTheBinder(t *testing.T) {
	pr := modelsDispatchRoute(t)
	chains := chainsPresent(gpt4oModelKey, claudeModelKey, modelsErrorKey)

	for body, want := range map[string]string{
		`{"model":"gpt-4o"}`:          gpt4oModelKey,
		`{"model":"claude-sonnet-4"}`: claudeModelKey,
		`{"model":"nope"}`:            modelsErrorKey,
		`not json`:                    modelsErrorKey,
	} {
		res := resolveModels(t, pr, []byte(body))
		bound, chain, err := Bind(pr, res, chains.get)
		require.NoError(t, err)
		require.NotNil(t, chain)
		assert.Equal(t, want, bound.ChainKey)
	}
}

// A missing error chain is deployment skew, classified as such rather than as the
// caller's fault — the same rule as any protocol-resolved route.
func TestLLMModelDispatch_MissingErrorChainIsChainMissing(t *testing.T) {
	pr := modelsDispatchRoute(t)
	res := resolveModels(t, pr, []byte(`{"model":"nope"}`))
	_, _, err := Bind(pr, res, noChains)
	var re *ResolutionError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, FailureChainMissing, re.Kind)
}

// Attributes are bounded well inside the framework backstop on every path.
func TestLLMModelDispatch_AttributesStayWithinLimits(t *testing.T) {
	pr := modelsDispatchRoute(t)
	for _, body := range []string{
		`{"model":"gpt-4o","stream":true}`,
		`{"model":"` + maxLengthedName + `","stream":true}`,
		`{"model":"` + oversizedName + `"}`,
	} {
		res := resolveModels(t, pr, []byte(body))
		assert.LessOrEqual(t, len(res.Attributes), MaxResolutionAttributes)
		for _, v := range res.Attributes {
			assert.LessOrEqual(t, len(v), MaxResolutionAttributeValueBytes)
		}
	}
}

// Each resolution gets its own attribute map, so a consumer holding one request's
// attributes can never observe another's.
func TestLLMModelDispatch_AttributeMapsAreNotShared(t *testing.T) {
	pr := modelsDispatchRoute(t)
	first := resolveModels(t, pr, []byte(`{"model":"nope-1"}`))
	second := resolveModels(t, pr, []byte(`{"model":"nope-2"}`))
	assert.Equal(t, "nope-1", first.Attributes[AttrLLMModelName])
	assert.Equal(t, "nope-2", second.Attributes[AttrLLMModelName])
}

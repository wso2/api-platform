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

package kernel

import (
	"context"
	"encoding/json"
	"testing"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// A route that names error_format: openai is the models-mode LlmProxy's. These tests pin
// the two halves of the contract: on such a route every gateway error — the engine's own
// included — reaches the client as an OpenAI error object, and on an identical route that
// names nothing the bytes are what they have always been.

const openAIRouteKey = "POST|/wso2/v1/chat/completions|example.com"

// openAIErrorOf decodes an OpenAI-shaped error body, failing the test if it is not one.
func openAIErrorOf(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc), "body: %s", body)
	inner, ok := doc["error"].(map[string]any)
	require.True(t, ok, "expected an OpenAI error object, got: %s", body)
	require.Len(t, doc, 1, "OpenAI's top level is the error member alone")
	return inner
}

// immediateContentType reads the content-type an immediate response sets.
func immediateContentType(imm *extprocv3.ImmediateResponse) string {
	for _, h := range imm.GetHeaders().GetSetHeaders() {
		if h.GetHeader().GetKey() == "content-type" {
			return string(h.GetHeader().GetRawValue())
		}
	}
	return ""
}

// llmProxyRoute registers an LlmProxy route, opted into the OpenAI format or not.
func llmProxyRoute(f *resolutionFixture, routeKey string, rr resolver.RouteResolution, openai bool) *RouteConfig {
	rc := f.route(routeKey, rr)
	rc.Metadata.APIKind = string(policy.APIKindLlmProxy)
	if openai {
		rc.Metadata.ErrorFormat = "openai"
	}
	return rc
}

// ─── Resolution failures ─────────────────────────────────────────────────────

func headerPhaseDenial(t *testing.T, openai bool) *extprocv3.ImmediateResponse {
	t.Helper()
	r := &fakeOperationResolver{
		name:      "header",
		header:    "x-op",
		forcedErr: &resolver.ResolutionError{Kind: resolver.FailureInvalidRequest},
	}
	f := newResolutionFixture(t, r)
	llmProxyRoute(f, openAIRouteKey, resolver.RouteResolution{ResolverName: "header"}, openai)

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		headersRequest(openAIRouteKey, false, map[string]string{":method": "POST", "x-op": "a"}),
	})
	require.NoError(t, f.server.Process(stream))
	require.Len(t, stream.responses, 1)
	imm := stream.responses[0].GetImmediateResponse()
	require.NotNil(t, imm)
	return imm
}

func TestRouteErrorFormat_HeaderPhaseResolutionFailureIsOpenAIShaped(t *testing.T) {
	imm := headerPhaseDenial(t, true)

	assert.Equal(t, uint32(400), uint32(imm.GetStatus().GetCode()))
	assert.Equal(t, "application/json", immediateContentType(imm))
	inner := openAIErrorOf(t, imm.GetBody())
	assert.Equal(t, "Bad Request", inner["message"], "the same fixed reason phrase as the sterile body")
	assert.Equal(t, "invalid_request_error", inner["type"])
	assert.Equal(t, codeResolutionBadRequest, inner["code"])
	assert.Regexp(t, `^[0-9a-f-]{36}$`, inner["error_id"], "the engine's correlation id survives")
	assert.Nil(t, inner["param"])
}

func TestRouteErrorFormat_WithoutItTheSterileBodyStands(t *testing.T) {
	imm := headerPhaseDenial(t, false)

	assert.Equal(t, uint32(400), uint32(imm.GetStatus().GetCode()))
	assert.Regexp(t, `^\{"error":"Bad Request","error_id":"[0-9a-f-]{36}"\}$`, string(imm.GetBody()),
		"an LlmProxy route that names no format keeps the bytes it has always sent")
}

// The body-phase denial reads its signals from the execution context rather than the
// request headers, so it is the second place the route's format has to reach.
func TestRouteErrorFormat_BodyPhaseResolutionFailureIsOpenAIShaped(t *testing.T) {
	for _, openai := range []bool{true, false} {
		r := &fakeOperationResolver{name: "body", reqs: resolver.RequestRequirements{Body: resolver.BodyBuffered}, bodyField: "method"}
		f := newResolutionFixture(t, r)
		llmProxyRoute(f, openAIRouteKey, resolver.RouteResolution{ResolverName: "body"}, openai)

		execCtx := f.bindPending(t, openAIRouteKey)
		resp, err := execCtx.processRequestBody(context.Background(),
			&extprocv3.HttpBody{Body: []byte(`not json`), EndOfStream: true})
		require.NoError(t, err)
		imm := resp.GetImmediateResponse()
		require.NotNil(t, imm)
		assert.Equal(t, uint32(400), uint32(imm.GetStatus().GetCode()))

		if openai {
			assert.Equal(t, "invalid_request_error", openAIErrorOf(t, imm.GetBody())["type"])
		} else {
			assert.Regexp(t, `^\{"error":"Bad Request","error_id":"[0-9a-f-]{36}"\}$`, string(imm.GetBody()))
		}
	}
}

// ─── A route with no policy chain ────────────────────────────────────────────

func TestRouteErrorFormat_NoPolicyChainIsOpenAIShaped(t *testing.T) {
	for _, openai := range []bool{true, false} {
		f := newResolutionFixture(t)
		llmProxyRoute(f, openAIRouteKey, resolver.RouteResolution{}, openai) // identity, no chain

		stream := newMockStream([]*extprocv3.ProcessingRequest{
			headersRequest(openAIRouteKey, true, map[string]string{":method": "POST"}),
		})
		require.NoError(t, f.server.Process(stream))
		require.Len(t, stream.responses, 1)
		imm := stream.responses[0].GetImmediateResponse()
		require.NotNil(t, imm)
		assert.Equal(t, uint32(500), uint32(imm.GetStatus().GetCode()))

		if !openai {
			assert.JSONEq(t, `{"error":"Internal Server Error"}`, string(imm.GetBody()),
				"byte-for-byte the body this path has always returned")
			continue
		}
		inner := openAIErrorOf(t, imm.GetBody())
		assert.Equal(t, "server_error", inner["type"])
		assert.Equal(t, codeNoPolicyChain, inner["code"])
		assert.NotEmpty(t, inner["error_id"])
	}
}

// HEAD carries no body on an opted-in route either.
func TestRouteErrorFormat_HeadRequestGetsNoSynthesizedBody(t *testing.T) {
	f := newResolutionFixture(t)
	llmProxyRoute(f, openAIRouteKey, resolver.RouteResolution{}, true)

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		headersRequest(openAIRouteKey, true, map[string]string{":method": "HEAD"}),
	})
	require.NoError(t, f.server.Process(stream))
	imm := stream.responses[0].GetImmediateResponse()
	require.NotNil(t, imm)
	assert.JSONEq(t, `{"error":"Internal Server Error"}`, string(imm.GetBody()))
}

// ─── Policy rejections ───────────────────────────────────────────────────────

// describingHeaderPolicy rejects at the header phase with a described fault and no body —
// the shape a migrated auth policy uses.
type describingHeaderPolicy struct{}

func (describingHeaderPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestHeaderMode: policy.HeaderModeProcess}
}

func (describingHeaderPolicy) OnRequestHeaders(context.Context, *policy.RequestHeaderContext, map[string]interface{}) policy.RequestHeaderAction {
	return policy.ImmediateResponse{
		StatusCode: 401,
		Fault: &policy.FaultDetails{
			Code:    policy.FaultCodeAuthMissingCredentials,
			Type:    policy.FaultTypeAuthentication,
			Message: "Missing credentials",
		},
	}
}

func policyRejection(t *testing.T, reject policy.Policy, openai bool) *extprocv3.ImmediateResponse {
	t.Helper()
	f := newResolutionFixture(t)
	llmProxyRoute(f, openAIRouteKey, resolver.RouteResolution{}, openai)
	f.chain(openAIRouteKey, reject)

	stream := newMockStream([]*extprocv3.ProcessingRequest{
		headersRequest(openAIRouteKey, true, map[string]string{":method": "POST"}),
	})
	require.NoError(t, f.server.Process(stream))
	require.NotEmpty(t, stream.responses)
	imm := stream.responses[0].GetImmediateResponse()
	require.NotNil(t, imm)
	return imm
}

func TestRouteErrorFormat_DescribedPolicyRejectionIsOpenAIShaped(t *testing.T) {
	imm := policyRejection(t, describingHeaderPolicy{}, true)

	assert.Equal(t, uint32(401), uint32(imm.GetStatus().GetCode()))
	assert.Equal(t, "application/json", immediateContentType(imm))
	inner := openAIErrorOf(t, imm.GetBody())
	assert.Equal(t, "Missing credentials", inner["message"])
	assert.Equal(t, "authentication_error", inner["type"])
	assert.Equal(t, policy.FaultCodeAuthMissingCredentials, inner["code"])
}

// LlmProxy is not a formatted kind, so without the route's opt-in a described rejection
// keeps whatever the policy wrote — here, nothing.
func TestRouteErrorFormat_DescribedPolicyRejectionWithoutItIsUnformatted(t *testing.T) {
	imm := policyRejection(t, describingHeaderPolicy{}, false)
	assert.NotContains(t, string(imm.GetBody()), `"type"`)
}

// A policy that authors its own body without describing a fault keeps it only when the body
// is already in the route's format — that is how the dispatch-error responder answers with
// its own param and code (TestRouteFormat_ProducerBodyInTheFormatStands). A body in another
// shape, such as an authentication policy's, is rendered over: the route promises OpenAI
// errors, status kept.
func TestRouteErrorFormat_AuthoredBodyInAnotherShapeIsRendered(t *testing.T) {
	imm := policyRejection(t, &headerPolicy{statusCode: 401}, true)

	assert.Equal(t, uint32(401), uint32(imm.GetStatus().GetCode()))
	assert.JSONEq(t,
		`{"error":{"message":"Invalid or expired credentials.","type":"authentication_error","param":null,"code":null}}`,
		string(imm.GetBody()))
}

// ─── Model dispatch end to end ───────────────────────────────────────────────

// attributeRecorderPolicy records the resolution attributes its chain was bound with.
type attributeRecorderPolicy struct {
	ran   *bool
	attrs map[string]string
}

func (p *attributeRecorderPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestHeaderMode: policy.HeaderModeProcess}
}

func (p *attributeRecorderPolicy) OnRequestHeaders(_ context.Context, ctx *policy.RequestHeaderContext, _ map[string]interface{}) policy.RequestHeaderAction {
	*p.ran = true
	for _, name := range []string{
		resolver.AttrLLMDispatchError, resolver.AttrLLMModelName,
		resolver.AttrLLMModelHandle, resolver.AttrLLMRequestStream,
	} {
		if v, ok := ctx.SharedContext.ResolutionAttributes.Lookup(name); ok {
			p.attrs[name] = v
		}
	}
	return policy.UpstreamRequestHeaderModifications{}
}

// The real resolver, through the real deferred-binding path: a known model binds its own
// chain, and anything else binds the reserved error chain with the reason attached — never
// a resolution failure, which would answer before authentication.
func TestLLMModelDispatch_BindsModelAndErrorChainsThroughTheKernel(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantModel bool
		wantAttrs map[string]string
	}{
		{"known model", `{"model":"gpt-4o","stream":true}`, true, map[string]string{
			resolver.AttrLLMModelName:     "gpt-4o",
			resolver.AttrLLMModelHandle:   "gpt4o",
			resolver.AttrLLMRequestStream: "true",
		}},
		{"unknown model", `{"model":"gpt-5"}`, false, map[string]string{
			resolver.AttrLLMDispatchError: resolver.LLMDispatchModelNotFound,
			resolver.AttrLLMModelName:     "gpt-5",
			resolver.AttrLLMRequestStream: "false",
		}},
		{"invalid JSON", `{`, false, map[string]string{
			resolver.AttrLLMDispatchError: resolver.LLMDispatchInvalidJSON,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newResolutionFixture(t, &resolver.LLMModelDispatchResolver{})
			rc := llmProxyRoute(f, openAIRouteKey, resolver.RouteResolution{
				ResolverName:   resolver.LLMModelDispatchResolverName,
				ResolverConfig: json.RawMessage(`{"models":{"gpt-4o":"gpt4o"},"errorOperation":"__dispatch__"}`),
			}, true)
			require.True(t, rc.Prepared.Requirements.BuffersBody())

			var modelRan, errorRan bool
			model := &attributeRecorderPolicy{ran: &modelRan, attrs: map[string]string{}}
			dispatch := &attributeRecorderPolicy{ran: &errorRan, attrs: map[string]string{}}
			f.operationChain("gpt4o", model)
			f.operationChain("__dispatch__", dispatch)

			execCtx := f.bindPending(t, openAIRouteKey)
			_, err := execCtx.processRequestBody(context.Background(),
				&extprocv3.HttpBody{Body: []byte(tt.body), EndOfStream: true})
			require.NoError(t, err)
			require.False(t, execCtx.resolutionDenied, "model dispatch never denies; it binds a chain")

			assert.Equal(t, tt.wantModel, modelRan)
			assert.Equal(t, !tt.wantModel, errorRan)
			got := dispatch.attrs
			if tt.wantModel {
				got = model.attrs
			}
			assert.Equal(t, tt.wantAttrs, got)
		})
	}
}

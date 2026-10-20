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

package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func TestPolicyDecision(t *testing.T) {
	status := func(code int) *int { return &code }
	path := "/v2"
	cases := []struct {
		name      string
		action    any
		decision  string
		guardrail string
	}{
		{"no action", nil, constants.PolicyActionAllow, ""},
		{"empty request modification", policy.UpstreamRequestModifications{}, constants.PolicyActionAllow, ""},
		{"analytics only is not a change", policy.UpstreamRequestModifications{AnalyticsMetadata: map[string]any{"k": "v"}}, constants.PolicyActionAllow, ""},
		{"body rewrite", policy.UpstreamRequestModifications{Body: []byte("x")}, constants.PolicyActionModify, ""},
		{"cleared body", policy.UpstreamRequestModifications{Body: []byte{}}, constants.PolicyActionModify, ""},
		{"header set", policy.UpstreamRequestHeaderModifications{HeadersToSet: map[string]string{"a": "b"}}, constants.PolicyActionModify, ""},
		{"path rewrite", policy.UpstreamRequestHeaderModifications{Path: &path}, constants.PolicyActionModify, ""},
		{"response header removed", policy.DownstreamResponseHeaderModifications{HeadersToRemove: []string{"server"}}, constants.PolicyActionModify, ""},
		{"response status 200", policy.DownstreamResponseModifications{StatusCode: status(200)}, constants.PolicyActionModify, ""},
		{"response turned into 422", policy.DownstreamResponseModifications{StatusCode: status(422)}, constants.PolicyActionDeny, ""},
		{"response declared a fault", policy.DownstreamResponseModifications{IsFault: true}, constants.PolicyActionDeny, ""},
		{"immediate response", policy.ImmediateResponse{StatusCode: 401}, constants.PolicyActionDeny, ""},
		{"guardrail named by the fault", policy.ImmediateResponse{StatusCode: 422, Fault: &policy.FaultDetails{
			Guardrail: &policy.GuardrailDetails{InterveningGuardrail: "pii-masking"},
		}}, constants.PolicyActionDeny, "pii-masking"},
		{"guardrail named by analytics", policy.ImmediateResponse{StatusCode: 422, AnalyticsMetadata: map[string]any{
			constants.GuardrailHitMetadataKey: true, constants.GuardrailNameMetadataKey: "url-guardrail",
		}}, constants.PolicyActionDeny, "url-guardrail"},
		{"fault wins over analytics", policy.TerminateResponseChunk{Fault: &policy.FaultDetails{
			Guardrail: &policy.GuardrailDetails{InterveningGuardrail: "a"},
		}, AnalyticsMetadata: map[string]any{constants.GuardrailHitMetadataKey: true, constants.GuardrailNameMetadataKey: "b"}}, constants.PolicyActionDeny, "a"},
		{"monitor-mode hit does not deny", policy.UpstreamRequestModifications{AnalyticsMetadata: map[string]any{
			constants.GuardrailHitMetadataKey: true, constants.GuardrailNameMetadataKey: "monitor",
		}}, constants.PolicyActionAllow, ""},
		{"stream chunk untouched", policy.ForwardResponseChunk{}, constants.PolicyActionAllow, ""},
		{"stream chunk rewritten", policy.ForwardRequestChunk{Body: []byte("x")}, constants.PolicyActionModify, ""},
		{"stream terminated", policy.TerminateResponseChunk{}, constants.PolicyActionDeny, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, guardrail := policyDecision(tc.action)
			assert.Equal(t, tc.decision, decision)
			assert.Equal(t, tc.guardrail, guardrail)
		})
	}
}

type denyingBodyPolicy struct{}

func (denyingBodyPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestBodyMode: policy.BodyModeBuffer}
}

func (denyingBodyPolicy) OnRequestBody(_ context.Context, _ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	return policy.ImmediateResponse{StatusCode: 422, Fault: &policy.FaultDetails{
		Type:      policy.FaultTypeGuardrail,
		Guardrail: &policy.GuardrailDetails{InterveningGuardrail: "content-length-guardrail"},
	}}
}

func spanAttr(span sdktrace.ReadOnlySpan, key string) (attribute.Value, bool) {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func TestExecuteRequestPolicies_GuardrailDenyStampsName(t *testing.T) {
	exec, sr := newRecordedChainExecutor(t, nil)

	_, err := exec.ExecuteRequestPolicies(context.Background(), []policy.Policy{denyingBodyPolicy{}},
		testutils.NewTestRequestContext(), []policy.PolicySpec{newPolicySpec("guard", "v1", true, nil)}, "api", "route", false)
	require.NoError(t, err)

	span := findSpanByName(sr.Ended(), "policy.request.guard")
	require.NotNil(t, span)
	action, _ := spanAttr(span, constants.AttrPolicyAction)
	assert.Equal(t, constants.PolicyActionDeny, action.AsString())
	name, _ := spanAttr(span, constants.AttrGuardrailName)
	assert.Equal(t, "content-length-guardrail", name.AsString())
}

// alternatingChunkPolicy rewrites the first chunk and leaves the rest alone.
type alternatingChunkPolicy struct{ calls int }

func (p *alternatingChunkPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{ResponseBodyMode: policy.BodyModeStream}
}
func (p *alternatingChunkPolicy) OnResponseBody(context.Context, *policy.ResponseContext, map[string]interface{}) policy.ResponseAction {
	return nil
}
func (p *alternatingChunkPolicy) NeedsMoreResponseData([]byte) bool { return false }
func (p *alternatingChunkPolicy) OnResponseBodyChunk(_ context.Context, _ *policy.ResponseStreamContext, _ *policy.StreamBody, _ map[string]interface{}) policy.StreamingResponseAction {
	p.calls++
	if p.calls == 1 {
		return policy.ForwardResponseChunk{Body: []byte("rewritten")}
	}
	return policy.ForwardResponseChunk{}
}

// A stream's span keeps the strongest decision, not the last chunk's.
func TestStreamSpans_DecisionIsSticky(t *testing.T) {
	exec, sr := newRecordedChainExecutor(t, nil)
	spans := NewStreamSpans()
	pol := &alternatingChunkPolicy{}
	for i := 0; i < 3; i++ {
		_, err := exec.ExecuteStreamingResponsePolicies(context.Background(), []policy.Policy{pol}, streamRespCtx(),
			&policy.StreamBody{Chunk: []byte("a")}, []policy.PolicySpec{newPolicySpec("alt", "v1", true, nil)}, "api", "route", false, spans)
		require.NoError(t, err)
	}
	spans.End()

	span := findSpanByName(sr.Ended(), "policy.response.alt")
	require.NotNil(t, span)
	action, _ := spanAttr(span, constants.AttrPolicyAction)
	assert.Equal(t, constants.PolicyActionModify, action.AsString())
}

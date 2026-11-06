/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// redirectingPolicy names an upstream definition, in whichever phase it runs.
type redirectingPolicy struct{ name string }

func (p *redirectingPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestHeaderMode: policy.HeaderModeProcess, RequestBodyMode: policy.BodyModeBuffer}
}
func (p *redirectingPolicy) OnRequestHeaders(context.Context, *policy.RequestHeaderContext, map[string]interface{}) policy.RequestHeaderAction {
	return policy.UpstreamRequestHeaderModifications{UpstreamName: &p.name}
}
func (p *redirectingPolicy) OnRequestBody(context.Context, *policy.RequestContext, map[string]interface{}) policy.RequestAction {
	return policy.UpstreamRequestModifications{UpstreamName: &p.name}
}

// upstreamReadingPolicy records the upstream URL its context shows it.
type upstreamReadingPolicy struct{ sawHeader, sawBody string }

func (p *upstreamReadingPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{RequestHeaderMode: policy.HeaderModeProcess, RequestBodyMode: policy.BodyModeBuffer}
}
func (p *upstreamReadingPolicy) OnRequestHeaders(_ context.Context, ctx *policy.RequestHeaderContext, _ map[string]interface{}) policy.RequestHeaderAction {
	if ctx.Upstream != nil {
		p.sawHeader = ctx.Upstream.URL
	}
	return nil
}
func (p *upstreamReadingPolicy) OnRequestBody(_ context.Context, ctx *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	if ctx.Upstream != nil {
		p.sawBody = ctx.Upstream.URL
	}
	return nil
}

// A policy that names an upstream tells the hook before the next policy runs, in both request
// phases, so the hook's caller can show that policy the upstream the request now goes to.
func TestUpstreamRedirectHook_RunsBeforeTheNextPolicy(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer("test"))
	specs := []policy.PolicySpec{newPolicySpec("route", "v1", true, nil), newPolicySpec("signer", "v1", true, nil)}

	headerCtx := &policy.RequestHeaderContext{SharedContext: testutils.NewTestSharedContext(),
		Headers: policy.NewHeaders(map[string][]string{}), Path: "/", Method: "POST"}
	bodyCtx := testutils.NewTestRequestContext()
	var named []string
	hook := func(name string) {
		named = append(named, name)
		headerCtx.Upstream = &policy.UpstreamRequestContext{URL: "https://" + name}
		bodyCtx.Upstream = &policy.UpstreamRequestContext{URL: "https://" + name}
	}
	ctx := WithUpstreamRedirectHook(context.Background(), hook)

	reader := &upstreamReadingPolicy{}
	policies := []policy.Policy{&redirectingPolicy{name: "bedrock"}, reader}
	_, err := exec.ExecuteRequestHeaderPolicies(ctx, policies, headerCtx, specs, "api", "route", false)
	require.NoError(t, err)
	_, err = exec.ExecuteRequestPolicies(ctx, policies, bodyCtx, specs, "api", "route", false)
	require.NoError(t, err)

	assert.Equal(t, []string{"bedrock", "bedrock"}, named)
	assert.Equal(t, "https://bedrock", reader.sawHeader)
	assert.Equal(t, "https://bedrock", reader.sawBody)
}

// Without a hook a redirect changes nothing the next policy sees, as before.
func TestUpstreamRedirectHook_AbsentIsANoOp(t *testing.T) {
	exec := NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer("test"))
	specs := []policy.PolicySpec{newPolicySpec("route", "v1", true, nil), newPolicySpec("signer", "v1", true, nil)}
	bodyCtx := testutils.NewTestRequestContext()
	bodyCtx.Upstream = &policy.UpstreamRequestContext{URL: "https://default"}
	reader := &upstreamReadingPolicy{}

	_, err := exec.ExecuteRequestPolicies(context.Background(),
		[]policy.Policy{&redirectingPolicy{name: "bedrock"}, reader}, bodyCtx, specs, "api", "route", false)
	require.NoError(t, err)
	assert.Equal(t, "https://default", reader.sawBody)
}

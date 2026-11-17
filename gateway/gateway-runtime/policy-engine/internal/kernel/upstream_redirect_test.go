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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

func modelEndpointExecCtx() *PolicyExecutionContext {
	def := &policyenginev1.UpstreamInfo{ClusterName: "upstream_LlmProxy_api-1__unrouted", URL: "http://127.0.0.1:8080",
		BasePath: "/__wso2_model_gateway_unrouted__"}
	return &PolicyExecutionContext{
		sharedCtx:               &policy.SharedContext{APIKind: policy.APIKindLlmProxy, APIId: "api-1"},
		defaultUpstream:         def,
		upstreamDefinitionPaths: map[string]string{"bedrock": "", "openai": "/v1"},
		upstreamDefinitionURLs:  map[string]string{"bedrock": "https://bedrock-runtime.us-east-1.amazonaws.com"},
		requestHeaderCtx:        &policy.RequestHeaderContext{Upstream: toRequestUpstream(def)},
		requestBodyCtx:          &policy.RequestContext{UpstreamInfo: def, Upstream: toRequestUpstream(def)},
		requestStreamContext:    &policy.RequestStreamContext{Upstream: toRequestUpstream(def)},
	}
}

// A redirect to a definition the route gave a URL for becomes the upstream every later
// request policy sees — what a SigV4 signer after the model endpoint's route job signs for.
func TestNoteUpstreamRedirect_ShowsTheRedirectedUpstream(t *testing.T) {
	ec := modelEndpointExecCtx()
	ec.noteUpstreamRedirect("bedrock")

	want := "https://bedrock-runtime.us-east-1.amazonaws.com"
	require.NotNil(t, ec.requestBodyCtx.UpstreamInfo)
	assert.Equal(t, want, ec.requestBodyCtx.UpstreamInfo.URL)
	assert.Equal(t, "", ec.requestBodyCtx.UpstreamInfo.BasePath)
	assert.Equal(t, "upstream_LlmProxy_api-1_bedrock", ec.requestBodyCtx.UpstreamInfo.ClusterName)
	assert.Equal(t, want, ec.requestBodyCtx.Upstream.URL)
	assert.Equal(t, want, ec.requestHeaderCtx.Upstream.URL)
	assert.Equal(t, want, ec.requestStreamContext.Upstream.URL)
	assert.Equal(t, "http://127.0.0.1:8080", ec.defaultUpstream.URL, "the route's default is not rewritten")
}

// A definition without a URL, or a name the route does not know, changes nothing.
func TestNoteUpstreamRedirect_UnknownDefinitionChangesNothing(t *testing.T) {
	for _, name := range []string{"openai", "nope"} {
		ec := modelEndpointExecCtx()
		ec.noteUpstreamRedirect(name)
		assert.Equal(t, "http://127.0.0.1:8080", ec.requestBodyCtx.Upstream.URL, name)
		assert.Equal(t, "http://127.0.0.1:8080", ec.requestBodyCtx.UpstreamInfo.URL, name)
	}
}

// Only a route whose definitions carry URLs installs the hook.
func TestWithUpstreamRedirectHook_OnlyOnRoutesWithDefinitionURLs(t *testing.T) {
	base := context.Background()
	ec := modelEndpointExecCtx()
	assert.NotEqual(t, base, ec.withUpstreamRedirectHook(base))

	ec.upstreamDefinitionURLs = nil
	assert.Equal(t, base, ec.withUpstreamRedirectHook(base))
}

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
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// newDownstreamTestExecCtx builds a bare PolicyExecutionContext suitable for
// exercising the context builders and header-mutation helpers directly.
func newDownstreamTestExecCtx() *PolicyExecutionContext {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "")
	return newPolicyExecutionContext(server, "test-route", &registry.PolicyChain{})
}

func httpHeaders(pairs ...[2]string) *extprocv3.HttpHeaders {
	values := make([]*corev3.HeaderValue, 0, len(pairs))
	for _, p := range pairs {
		values = append(values, &corev3.HeaderValue{Key: p[0], RawValue: []byte(p[1])})
	}
	return &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: values}}
}

// setRequestHeaderResult mimics a later header-phase policy that overwrites and
// removes request headers, so we can drive applyRequestHeaderMutations.
func setRequestHeaderResult(set map[string]string, remove []string) []executor.RequestHeaderPolicyResult {
	return []executor.RequestHeaderPolicyResult{{
		PolicyName: "mutator",
		Action: policy.UpstreamRequestHeaderModifications{
			HeadersToSet:    set,
			HeadersToRemove: remove,
		},
	}}
}

func setResponseHeaderResult(set map[string]string, remove []string) []executor.ResponseHeaderPolicyResult {
	return []executor.ResponseHeaderPolicyResult{{
		PolicyName: "mutator",
		Action: policy.DownstreamResponseHeaderModifications{
			HeadersToSet:    set,
			HeadersToRemove: remove,
		},
	}}
}

// TestDownstreamSnapshot_PopulatedOnAllRequestContexts verifies buildRequestContexts
// captures the original client headers onto Downstream for every request-phase context.
func TestDownstreamSnapshot_PopulatedOnAllRequestContexts(t *testing.T) {
	ec := newDownstreamTestExecCtx()
	ec.buildRequestContexts(httpHeaders(
		[2]string{":path", "/api/pets"},
		[2]string{":method", "POST"},
		[2]string{"authorization", "Bearer original"},
	), RouteMetadata{})

	for name, ds := range map[string]*policy.DownstreamContext{
		"requestHeaderCtx":     ec.requestHeaderCtx.Downstream,
		"requestBodyCtx":       ec.requestBodyCtx.Downstream,
		"requestStreamContext": ec.requestStreamContext.Downstream,
	} {
		require.NotNilf(t, ds, "%s.Downstream should be populated", name)
		require.NotNilf(t, ds.Request, "%s.Downstream.Request should be populated", name)
		require.NotNilf(t, ds.Request.Headers, "%s.Downstream.Request.Headers should be populated", name)
		assert.Equalf(t, []string{"Bearer original"}, ds.Request.Headers.Get("authorization"),
			"%s should expose the original client header", name)
	}
}

// TestDownstreamSnapshot_SurvivesRequestHeaderMutation is the core regression test:
// a later header-phase policy rewrites a header, but the Downstream snapshot that a
// body-phase validator reads must still reflect the original client value.
func TestDownstreamSnapshot_SurvivesRequestHeaderMutation(t *testing.T) {
	ec := newDownstreamTestExecCtx()
	ec.buildRequestContexts(httpHeaders(
		[2]string{":path", "/api/pets"},
		[2]string{"authorization", "Bearer original"},
		[2]string{"x-legacy", "keep-me"},
	), RouteMetadata{})

	// Simulate a later header-phase policy: overwrite authorization, remove x-legacy.
	applyRequestHeaderMutations(ec.requestHeaderCtx.Headers, setRequestHeaderResult(
		map[string]string{"authorization": "Bearer mutated"},
		[]string{"x-legacy"},
	))

	// Mutable view reflects the mutation (what a header-phase policy left behind).
	assert.Equal(t, []string{"Bearer mutated"}, ec.requestBodyCtx.Headers.Get("authorization"))
	assert.False(t, ec.requestBodyCtx.Headers.Has("x-legacy"))

	// Downstream snapshot still reflects what the client actually sent.
	require.NotNil(t, ec.requestBodyCtx.Downstream)
	assert.Equal(t, []string{"Bearer original"}, ec.requestBodyCtx.Downstream.Request.Headers.Get("authorization"),
		"body-phase validator must see the pristine downstream header")
	assert.True(t, ec.requestBodyCtx.Downstream.Request.Headers.Has("x-legacy"),
		"a header removed by a later policy must still be visible in the downstream snapshot")
}

// TestResponseSnapshots_DownstreamIsOriginalClientAndUpstreamSurvivesMutation verifies:
//   - Response Downstream is the ORIGINAL client request headers, not the mutated
//     request headers (i.e. the kernel uses the request-time snapshot, not
//     requestHeaderCtx.Headers which header-phase policies mutate in place).
//   - Response Upstream survives a response-header mutation.
func TestResponseSnapshots_DownstreamIsOriginalClientAndUpstreamSurvivesMutation(t *testing.T) {
	ec := newDownstreamTestExecCtx()
	ec.buildRequestContexts(httpHeaders(
		[2]string{":path", "/api/pets"},
		[2]string{"authorization", "Bearer original"},
	), RouteMetadata{})

	// A request header-phase policy rewrites the client header in place.
	applyRequestHeaderMutations(ec.requestHeaderCtx.Headers, setRequestHeaderResult(
		map[string]string{"authorization": "Bearer mutated"}, nil,
	))

	ec.buildResponseContexts(httpHeaders(
		[2]string{":status", "200"},
		[2]string{"x-backend-signature", "sig-original"},
	))

	// Downstream on the response path must be the pristine client value, even
	// though requestHeaderCtx.Headers now holds the mutated value.
	require.NotNil(t, ec.responseBodyCtx.Downstream)
	assert.Equal(t, []string{"Bearer original"}, ec.responseBodyCtx.Downstream.Request.Headers.Get("authorization"),
		"response Downstream must be the original client header, not the mutated one")

	// Now a response header-phase policy rewrites an upstream header.
	applyResponseHeaderMutations(ec.responseHeaderCtx.ResponseHeaders, setResponseHeaderResult(
		map[string]string{"x-backend-signature": "sig-mutated"}, nil,
	))

	// Mutable response headers reflect the mutation...
	assert.Equal(t, []string{"sig-mutated"}, ec.responseBodyCtx.ResponseHeaders.Get("x-backend-signature"))
	// ...but the Upstream snapshot preserves what the backend actually sent.
	require.NotNil(t, ec.responseBodyCtx.Upstream)
	assert.Equal(t, []string{"sig-original"}, ec.responseBodyCtx.Upstream.Response.Headers.Get("x-backend-signature"),
		"response-body validator must see the pristine upstream header")
}

// TestSnapshots_PopulatedOnAllResponseContexts verifies Downstream and Upstream are
// set on every response-phase context.
func TestSnapshots_PopulatedOnAllResponseContexts(t *testing.T) {
	ec := newDownstreamTestExecCtx()
	ec.buildRequestContexts(httpHeaders([2]string{":path", "/api/pets"}), RouteMetadata{})
	ec.buildResponseContexts(httpHeaders(
		[2]string{":status", "200"},
		[2]string{"x-backend-signature", "sig"},
	))

	type snap struct {
		ds *policy.DownstreamContext
		us *policy.UpstreamResponseContext
	}
	for name, s := range map[string]snap{
		"responseHeaderCtx":     {ec.responseHeaderCtx.Downstream, ec.responseHeaderCtx.Upstream},
		"responseBodyCtx":       {ec.responseBodyCtx.Downstream, ec.responseBodyCtx.Upstream},
		"responseStreamContext": {ec.responseStreamContext.Downstream, ec.responseStreamContext.Upstream},
	} {
		require.NotNilf(t, s.ds, "%s.Downstream should be populated", name)
		require.NotNilf(t, s.us, "%s.Upstream should be populated", name)
		require.NotNilf(t, s.us.Response, "%s.Upstream.Response should be populated", name)
		assert.Equalf(t, []string{"sig"}, s.us.Response.Headers.Get("x-backend-signature"),
			"%s should expose the original upstream header", name)
	}
}

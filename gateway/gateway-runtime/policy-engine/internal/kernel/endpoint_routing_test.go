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
	"net/http"
	"testing"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// publishEndpoint puts a gateway-wide Endpoint into the shared lazy resource
// store, as the controller's lazy-resource push does, and removes every
// Endpoint again when the test ends.
func publishEndpoint(t *testing.T, name, basePath string) {
	t.Helper()
	store := policy.GetLazyResourceStoreInstance()
	t.Cleanup(func() { _ = store.RemoveResourcesByType(policy.LazyResourceTypeEndpoint) })
	require.NoError(t, store.StoreResource(&policy.LazyResource{
		ID:           name,
		ResourceType: policy.LazyResourceTypeEndpoint,
		Resource: map[string]interface{}{
			"name":        name,
			"clusterName": policy.EndpointClusterPrefix + name,
			"basePath":    basePath,
			"labels":      map[string]interface{}{"team": "orders"},
		},
	}))
}

func newEndpointRoutingExecCtx() *PolicyExecutionContext {
	server := NewExternalProcessorServer(NewKernel(), executor.NewChainExecutor(nil, nil, nil),
		config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes, nil)
	execCtx := newPolicyExecutionContext(server, "test-route", &registry.PolicyChain{})
	execCtx.sharedCtx = &policy.SharedContext{APIKind: "RestApi", APIId: "orders-123"}
	execCtx.requestBodyCtx = &policy.RequestContext{Path: "/orders/v1.0/items", SharedContext: execCtx.sharedCtx}
	execCtx.apiContext = "/orders/v1.0"
	execCtx.upstreamBasePath = "/"
	execCtx.defaultUpstreamCluster = "upstream_main_orders-shared_9080"
	execCtx.upstreamDefinitionPaths = map[string]string{"tenant-a": "/local"}
	return execCtx
}

func strPtr(s string) *string { return &s }

// targetClusterOf returns the value the response sets x-target-upstream to.
func targetClusterOf(t *testing.T, mutation *extprocv3.HeaderMutation) string {
	t.Helper()
	require.NotNil(t, mutation)
	var cluster string
	for _, h := range mutation.SetHeaders {
		if h.Header.Key == constants.TargetUpstreamHeader {
			cluster = string(h.Header.RawValue)
		}
	}
	return cluster
}

func headerPhaseResult(mods ...policy.UpstreamRequestHeaderModifications) *executor.RequestHeaderExecutionResult {
	result := &executor.RequestHeaderExecutionResult{}
	for _, m := range mods {
		result.Results = append(result.Results, executor.RequestHeaderPolicyResult{Action: m})
	}
	return result
}

func TestEndpointRouting_HeaderPhaseRoutesToTheEndpointCluster(t *testing.T) {
	publishEndpoint(t, "tenant-b", "/v1")
	execCtx := newEndpointRoutingExecCtx()

	resp, err := TranslateRequestHeaderActions(headerPhaseResult(policy.UpstreamRequestHeaderModifications{
		EndpointName:    strPtr("tenant-b"),
		HeadersToRemove: []string{"x-endpoint"},
	}), execCtx.policyChain, execCtx)
	require.NoError(t, err)

	mutation := resp.GetRequestHeaders().GetResponse().GetHeaderMutation()
	assert.Equal(t, "endpoint_tenant-b", targetClusterOf(t, mutation))
	assert.Contains(t, mutation.RemoveHeaders, "x-endpoint", "the selector header must not reach the backend")

	namespace := execCtx.dynamicMetadata[constants.ExtProcFilterName]
	assert.Equal(t, "tenant-b", namespace[constants.TargetEndpointNameKey])
	assert.NotContains(t, namespace, constants.TargetUpstreamNameKey)
	assert.Equal(t, "endpoint_tenant-b", namespace[constants.TargetUpstreamClusterKey])
	assert.Equal(t, "/v1", namespace["target_upstream_base_path"], "the Endpoint's base path drives the Lua rewrite")
}

// An Endpoint and an upstream definition may share a name. EndpointName must
// reach the gateway-wide cluster, never the API's own definition of that name.
func TestEndpointRouting_EndpointNameNeverResolvesToAnAPIDefinition(t *testing.T) {
	publishEndpoint(t, "tenant-a", "/v1")
	execCtx := newEndpointRoutingExecCtx()

	resp, err := TranslateRequestHeaderActions(headerPhaseResult(policy.UpstreamRequestHeaderModifications{
		EndpointName: strPtr("tenant-a"),
	}), execCtx.policyChain, execCtx)
	require.NoError(t, err)
	assert.Equal(t, "endpoint_tenant-a", targetClusterOf(t, resp.GetRequestHeaders().GetResponse().GetHeaderMutation()))

	execCtx = newEndpointRoutingExecCtx()
	resp, err = TranslateRequestHeaderActions(headerPhaseResult(policy.UpstreamRequestHeaderModifications{
		UpstreamName: strPtr("tenant-a"),
	}), execCtx.policyChain, execCtx)
	require.NoError(t, err)
	assert.Equal(t, "upstream_RestApi_orders-123_tenant-a",
		targetClusterOf(t, resp.GetRequestHeaders().GetResponse().GetHeaderMutation()))
}

func TestEndpointRouting_LastWriteWinsBetweenUpstreamNameAndEndpointName(t *testing.T) {
	publishEndpoint(t, "tenant-b", "/v1")

	for _, tc := range []struct {
		name string
		mods []policy.UpstreamRequestHeaderModifications
		want string
	}{
		{
			name: "endpoint after upstream",
			mods: []policy.UpstreamRequestHeaderModifications{
				{UpstreamName: strPtr("tenant-a")}, {EndpointName: strPtr("tenant-b")},
			},
			want: "endpoint_tenant-b",
		},
		{
			name: "upstream after endpoint",
			mods: []policy.UpstreamRequestHeaderModifications{
				{EndpointName: strPtr("tenant-b")}, {UpstreamName: strPtr("tenant-a")},
			},
			want: "upstream_RestApi_orders-123_tenant-a",
		},
		{
			name: "a later policy that sets neither leaves the directive alone",
			mods: []policy.UpstreamRequestHeaderModifications{
				{EndpointName: strPtr("tenant-b")}, {HeadersToSet: map[string]string{"x-a": "b"}},
			},
			want: "endpoint_tenant-b",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			execCtx := newEndpointRoutingExecCtx()
			resp, err := TranslateRequestHeaderActions(headerPhaseResult(tc.mods...), execCtx.policyChain, execCtx)
			require.NoError(t, err)
			assert.Equal(t, tc.want, targetClusterOf(t, resp.GetRequestHeaders().GetResponse().GetHeaderMutation()))
		})
	}
}

func TestEndpointRouting_NoDirectiveKeepsTheRouteDefault(t *testing.T) {
	publishEndpoint(t, "tenant-b", "/v1")
	execCtx := newEndpointRoutingExecCtx()

	resp, err := TranslateRequestHeaderActions(headerPhaseResult(policy.UpstreamRequestHeaderModifications{}),
		execCtx.policyChain, execCtx)
	require.NoError(t, err)
	assert.Equal(t, "upstream_main_orders-shared_9080",
		targetClusterOf(t, resp.GetRequestHeaders().GetResponse().GetHeaderMutation()))
}

// A missing Endpoint must fail the request, not fall back to the route's own
// upstream: that is not where the policy sent it.
func TestEndpointRouting_MissingEndpointFailsClosed(t *testing.T) {
	execCtx := newEndpointRoutingExecCtx()

	resp, err := TranslateRequestHeaderActions(headerPhaseResult(policy.UpstreamRequestHeaderModifications{
		EndpointName: strPtr("tenant-gone"),
	}), execCtx.policyChain, execCtx)

	var missing *endpointUnavailableError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, "tenant-gone", missing.endpoint)
	assert.Nil(t, resp)
}

func TestEndpointRouting_MissingEndpointBecomesA503WithEngineCode(t *testing.T) {
	execCtx := newEndpointRoutingExecCtx()
	execCtx.requestHeaderCtx = &policy.RequestHeaderContext{SharedContext: execCtx.sharedCtx}

	_, translateErr := TranslateRequestHeaderActions(headerPhaseResult(policy.UpstreamRequestHeaderModifications{
		EndpointName: strPtr("tenant-gone"),
	}), execCtx.policyChain, execCtx)
	resp, err := execCtx.routingResponse(context.Background(), nil, translateErr)
	require.NoError(t, err)

	imm := resp.GetImmediateResponse()
	require.NotNil(t, imm, "a missing Endpoint must answer the client instead of ending the ext_proc stream")
	assert.Equal(t, int32(http.StatusServiceUnavailable), int32(imm.GetStatus().GetCode()))
	assert.NotContains(t, string(imm.GetBody()), "tenant-gone", "the response must not echo the Endpoint name")

	var errorID string
	for _, h := range imm.GetHeaders().GetSetHeaders() {
		if h.GetHeader().GetKey() == "x-error-id" {
			errorID = string(h.GetHeader().GetRawValue())
		}
	}
	assert.NotEmpty(t, errorID)
	assert.Equal(t, "905008", codeEndpointUnavailable)
	assert.Equal(t, constants.TerminalReasonEndpointUnavailable, execCtx.generated.outcome.Reason)
}

func TestEndpointRouting_RoutingResponsePassesOtherResultsThrough(t *testing.T) {
	execCtx := newEndpointRoutingExecCtx()
	want := &extprocv3.ProcessingResponse{}

	got, err := execCtx.routingResponse(context.Background(), want, nil)
	require.NoError(t, err)
	assert.Same(t, want, got)

	other := assert.AnError
	got, err = execCtx.routingResponse(context.Background(), nil, other)
	assert.Nil(t, got)
	assert.Same(t, other, err)
}

// The request-body phase re-arms the directive the header phase applied. It
// must come back as an Endpoint, not as an upstream definition of that name.
func TestEndpointRouting_BodyPhasePreservesHeaderPhaseEndpoint(t *testing.T) {
	publishEndpoint(t, "tenant-a", "/v1")
	execCtx := newEndpointRoutingExecCtx()

	_, err := TranslateRequestHeaderActions(headerPhaseResult(policy.UpstreamRequestHeaderModifications{
		EndpointName: strPtr("tenant-a"),
	}), execCtx.policyChain, execCtx)
	require.NoError(t, err)

	rsl, err := translateRequestActionsCore(&executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{{
			Action: policy.UpstreamRequestModifications{Body: []byte(`{}`)},
		}},
	}, execCtx)
	require.NoError(t, err)
	assert.Equal(t, "endpoint_tenant-a", targetClusterOf(t, rsl.HeaderMutation))
	assert.Equal(t, "/v1", rsl.DynamicMetadata[constants.ExtProcFilterName]["target_upstream_base_path"])
}

// An Endpoint removed between the header and body phases fails the request
// rather than letting the body phase quietly route to the default upstream.
func TestEndpointRouting_BodyPhaseFailsClosedWhenTheEndpointWasRemoved(t *testing.T) {
	execCtx := newEndpointRoutingExecCtx()
	execCtx.dynamicMetadata[constants.ExtProcFilterName] = map[string]interface{}{
		constants.TargetEndpointNameKey: "tenant-gone",
	}

	_, err := translateRequestActionsCore(&executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{{
			Action: policy.UpstreamRequestModifications{Body: []byte(`{}`)},
		}},
	}, execCtx)

	var missing *endpointUnavailableError
	require.ErrorAs(t, err, &missing)
}

func TestEndpointRouting_BodyPhasePolicyCanReplaceTheEndpoint(t *testing.T) {
	publishEndpoint(t, "tenant-b", "/v1")
	execCtx := newEndpointRoutingExecCtx()
	execCtx.dynamicMetadata[constants.ExtProcFilterName] = map[string]interface{}{
		constants.TargetEndpointNameKey: "tenant-b",
	}

	rsl, err := translateRequestActionsCore(&executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{{
			Action: policy.UpstreamRequestModifications{UpstreamName: strPtr("tenant-a")},
		}},
	}, execCtx)
	require.NoError(t, err)
	assert.Equal(t, "upstream_RestApi_orders-123_tenant-a", targetClusterOf(t, rsl.HeaderMutation))

	namespace := execCtx.dynamicMetadata[constants.ExtProcFilterName]
	assert.Equal(t, "tenant-a", namespace[constants.TargetUpstreamNameKey])
	assert.NotContains(t, namespace, constants.TargetEndpointNameKey)
}

func TestEndpointRouting_EmptyBasePathForwardsFromRoot(t *testing.T) {
	publishEndpoint(t, "tenant-b", "")
	execCtx := newEndpointRoutingExecCtx()

	_, err := TranslateRequestHeaderActions(headerPhaseResult(policy.UpstreamRequestHeaderModifications{
		EndpointName: strPtr("tenant-b"),
	}), execCtx.policyChain, execCtx)
	require.NoError(t, err)
	assert.Equal(t, "/", execCtx.dynamicMetadata[constants.ExtProcFilterName]["target_upstream_base_path"])
}

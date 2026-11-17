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
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// withBackends points the kernel's Backend lookup at a private store holding the given
// Backends (name → base path) for the duration of the test.
func withBackends(t *testing.T, backends map[string]string) {
	t.Helper()
	store := policy.NewLazyResourceStore()
	for name, basePath := range backends {
		require.NoError(t, store.StoreResource(&policy.LazyResource{
			ID:           name,
			ResourceType: policy.LazyResourceTypeBackend,
			Resource: map[string]interface{}{
				policy.BackendResourceKeyName:     name,
				policy.BackendResourceKeyBasePath: basePath,
			},
		}))
	}
	previous := lookupBackend
	lookupBackend = store.LookupBackend
	t.Cleanup(func() { lookupBackend = previous })
}

func newBackendRoutingExecCtx() *PolicyExecutionContext {
	kernel := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, nil)
	server := NewExternalProcessorServer(kernel, chainExecutor, config.TracingConfig{}, "", testMaxDecompressedBytes, testMaxDecompressedBytes, nil)
	execCtx := newPolicyExecutionContext(server, "test-route", &registry.PolicyChain{})
	execCtx.sharedCtx = &policy.SharedContext{APIKind: "API", APIId: "api-123"}
	execCtx.requestBodyCtx = &policy.RequestContext{Path: "/orders/v1.0/items", SharedContext: execCtx.sharedCtx}
	execCtx.apiContext = "/orders/v1.0"
	execCtx.upstreamBasePath = "/orders-backend"
	execCtx.upstreamDefinitionPaths = map[string]string{"alt-upstream": "/alternate"}
	execCtx.defaultUpstreamCluster = "upstream_main_orders"
	return execCtx
}

func targetUpstreamHeader(t *testing.T, resp *extprocv3.ProcessingResponse) string {
	t.Helper()
	for _, h := range resp.GetRequestHeaders().GetResponse().GetHeaderMutation().GetSetHeaders() {
		if h.GetHeader().GetKey() == constants.TargetUpstreamHeader {
			return string(h.GetHeader().GetRawValue())
		}
	}
	return ""
}

func headerPhase(mods ...policy.UpstreamRequestHeaderModifications) *executor.RequestHeaderExecutionResult {
	result := &executor.RequestHeaderExecutionResult{}
	for _, m := range mods {
		result.Results = append(result.Results, executor.RequestHeaderPolicyResult{Action: m})
	}
	return result
}

func strPtr(s string) *string { return &s }

func TestBackendRouting_HeaderPhaseRoutesToTheBackendCluster(t *testing.T) {
	withBackends(t, map[string]string{"tenant-a": "/v1"})
	execCtx := newBackendRoutingExecCtx()

	resp, err := TranslateRequestHeaderActions(
		headerPhase(policy.UpstreamRequestHeaderModifications{BackendName: strPtr("tenant-a")}),
		&registry.PolicyChain{}, execCtx)
	require.NoError(t, err)

	assert.Equal(t, "backend_tenant-a", targetUpstreamHeader(t, resp))
	extProc := resp.DynamicMetadata.Fields[constants.ExtProcFilterName].GetStructValue()
	require.NotNil(t, extProc)
	assert.Equal(t, "/v1", extProc.Fields["target_upstream_base_path"].GetStringValue())
	assert.Equal(t, "/orders/v1.0/items", extProc.Fields["path"].GetStringValue(),
		"the raw path goes to Lua, which strips the context and prefixes the Backend base path once")

	// The choice is remembered for the body phase under a label that names the kind.
	assert.Equal(t, "backend:tenant-a",
		execCtx.dynamicMetadata[constants.ExtProcFilterName][constants.TargetUpstreamNameKey])
	assert.Equal(t, "backend_tenant-a",
		execCtx.dynamicMetadata[constants.ExtProcFilterName][constants.TargetUpstreamClusterKey])
}

func TestBackendRouting_EmptyBasePathForwardsTheRelativePath(t *testing.T) {
	withBackends(t, map[string]string{"tenant-root": ""})
	execCtx := newBackendRoutingExecCtx()

	resp, err := TranslateRequestHeaderActions(
		headerPhase(policy.UpstreamRequestHeaderModifications{BackendName: strPtr("tenant-root")}),
		&registry.PolicyChain{}, execCtx)
	require.NoError(t, err)

	extProc := resp.DynamicMetadata.Fields[constants.ExtProcFilterName].GetStructValue()
	assert.Equal(t, "/", extProc.Fields["target_upstream_base_path"].GetStringValue(),
		"an empty value would make Lua keep the API's own upstream base path")
}

func TestBackendRouting_MissingBackendFailsClosed(t *testing.T) {
	withBackends(t, map[string]string{"tenant-a": "/v1"})
	execCtx := newBackendRoutingExecCtx()

	resp, err := TranslateRequestHeaderActions(
		headerPhase(policy.UpstreamRequestHeaderModifications{BackendName: strPtr("tenant-gone")}),
		&registry.PolicyChain{}, execCtx)

	assert.Nil(t, resp)
	var missing *backendUnavailableError
	require.True(t, errors.As(err, &missing), "expected backendUnavailableError, got %v", err)
	assert.Equal(t, "tenant-gone", missing.name)
}

func TestBackendRouting_LastWriteWinsAcrossUpstreamNameAndBackendName(t *testing.T) {
	withBackends(t, map[string]string{"tenant-a": "/v1"})

	t.Run("a Backend set after an upstream definition wins", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(headerPhase(
			policy.UpstreamRequestHeaderModifications{UpstreamName: strPtr("alt-upstream")},
			policy.UpstreamRequestHeaderModifications{BackendName: strPtr("tenant-a")},
		), &registry.PolicyChain{}, newBackendRoutingExecCtx())
		require.NoError(t, err)
		assert.Equal(t, "backend_tenant-a", targetUpstreamHeader(t, resp))
	})

	t.Run("an upstream definition set after a Backend wins", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(headerPhase(
			policy.UpstreamRequestHeaderModifications{BackendName: strPtr("tenant-a")},
			policy.UpstreamRequestHeaderModifications{UpstreamName: strPtr("alt-upstream")},
		), &registry.PolicyChain{}, newBackendRoutingExecCtx())
		require.NoError(t, err)
		assert.Equal(t, constants.UpstreamDefinitionClusterPrefix+"API_api-123_"+sanitizeUpstreamDefinitionName("alt-upstream"),
			targetUpstreamHeader(t, resp))
	})

	t.Run("a later result that sets only one replaces a result that set both", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(headerPhase(
			policy.UpstreamRequestHeaderModifications{UpstreamName: strPtr("alt-upstream"), BackendName: strPtr("alt-upstream")},
			policy.UpstreamRequestHeaderModifications{BackendName: strPtr("tenant-a")},
		), &registry.PolicyChain{}, newBackendRoutingExecCtx())
		require.NoError(t, err)
		assert.Equal(t, "backend_tenant-a", targetUpstreamHeader(t, resp))
	})

	t.Run("a later upstream definition overrides a missing Backend", func(t *testing.T) {
		_, err := TranslateRequestHeaderActions(headerPhase(
			policy.UpstreamRequestHeaderModifications{BackendName: strPtr("tenant-gone")},
			policy.UpstreamRequestHeaderModifications{UpstreamName: strPtr("alt-upstream")},
		), &registry.PolicyChain{}, newBackendRoutingExecCtx())
		require.NoError(t, err, "only the live directive is resolved")
	})
}

// A result that sets both UpstreamName and BackendName (dynamic-endpoint's targetUpstream) routes
// to the API's own upstream definition when the API has one of that name, else to the Backend.
func TestBackendRouting_BothSetPrefersTheAPIsUpstreamDefinition(t *testing.T) {
	withBackends(t, map[string]string{"alt-upstream": "/shadowed", "tenant-a": "/v1"})
	both := func(name string) policy.UpstreamRequestHeaderModifications {
		return policy.UpstreamRequestHeaderModifications{UpstreamName: strPtr(name), BackendName: strPtr(name)}
	}

	t.Run("an upstream definition of the API wins over a Backend of the same name", func(t *testing.T) {
		execCtx := newBackendRoutingExecCtx()
		resp, err := TranslateRequestHeaderActions(headerPhase(both("alt-upstream")), &registry.PolicyChain{}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, constants.UpstreamDefinitionClusterPrefix+"API_api-123_"+sanitizeUpstreamDefinitionName("alt-upstream"),
			targetUpstreamHeader(t, resp))
		assert.Equal(t, "alt-upstream", execCtx.dynamicMetadata[constants.ExtProcFilterName][constants.TargetUpstreamNameKey])
		assert.Equal(t, "/alternate", execCtx.dynamicMetadata[constants.ExtProcFilterName]["target_upstream_base_path"])
	})

	t.Run("a name the API does not define routes to the Backend", func(t *testing.T) {
		execCtx := newBackendRoutingExecCtx()
		resp, err := TranslateRequestHeaderActions(headerPhase(both("tenant-a")), &registry.PolicyChain{}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "backend_tenant-a", targetUpstreamHeader(t, resp))
		assert.Equal(t, "backend:tenant-a", execCtx.dynamicMetadata[constants.ExtProcFilterName][constants.TargetUpstreamNameKey])
	})

	t.Run("a name that is neither fails closed", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(headerPhase(both("tenant-gone")), &registry.PolicyChain{}, newBackendRoutingExecCtx())
		assert.Nil(t, resp)
		var missing *backendUnavailableError
		require.True(t, errors.As(err, &missing), "expected backendUnavailableError, got %v", err)
		assert.Equal(t, "tenant-gone", missing.name)
	})

	t.Run("an API with no upstream definitions routes to the Backend", func(t *testing.T) {
		execCtx := newBackendRoutingExecCtx()
		execCtx.upstreamDefinitionPaths = nil
		resp, err := TranslateRequestHeaderActions(headerPhase(both("alt-upstream")), &registry.PolicyChain{}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "backend_alt-upstream", targetUpstreamHeader(t, resp))
	})
}

func TestBackendRouting_NoDirectiveKeepsTheRouteDefault(t *testing.T) {
	withBackends(t, map[string]string{"tenant-a": "/v1"})
	resp, err := TranslateRequestHeaderActions(headerPhase(policy.UpstreamRequestHeaderModifications{}),
		&registry.PolicyChain{}, newBackendRoutingExecCtx())
	require.NoError(t, err)
	assert.Equal(t, "upstream_main_orders", targetUpstreamHeader(t, resp))
}

func TestBackendRouting_BodyPhaseReResolvesTheHeaderPhaseBackend(t *testing.T) {
	withBackends(t, map[string]string{"tenant-a": "/v1"})
	execCtx := newBackendRoutingExecCtx()
	execCtx.dynamicMetadata[constants.ExtProcFilterName] = map[string]interface{}{
		constants.TargetUpstreamNameKey: "backend:tenant-a",
	}

	// A body-phase chain that leaves routing alone keeps the header-phase Backend.
	out, err := translateRequestActionsCore(&executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{{Action: policy.UpstreamRequestModifications{}}},
	}, execCtx)
	require.NoError(t, err)
	assert.Equal(t, "backend_tenant-a", out.DynamicMetadata[constants.ExtProcFilterName][constants.CurrentUpstreamKey].(map[string]interface{})["cluster_name"])
	assert.Equal(t, "/v1", out.DynamicMetadata[constants.ExtProcFilterName]["target_upstream_base_path"])
}

func TestBackendRouting_BodyPhaseFailsClosedWhenTheBackendWasRemoved(t *testing.T) {
	withBackends(t, map[string]string{})
	execCtx := newBackendRoutingExecCtx()
	execCtx.dynamicMetadata[constants.ExtProcFilterName] = map[string]interface{}{
		constants.TargetUpstreamNameKey: "backend:tenant-a",
	}

	_, err := translateRequestActionsCore(&executor.RequestExecutionResult{}, execCtx)
	var missing *backendUnavailableError
	require.True(t, errors.As(err, &missing))
}

func TestBackendRouting_BodyPhaseKeepsAnUpstreamDefinitionLabel(t *testing.T) {
	withBackends(t, map[string]string{})
	execCtx := newBackendRoutingExecCtx()
	execCtx.dynamicMetadata[constants.ExtProcFilterName] = map[string]interface{}{
		constants.TargetUpstreamNameKey: "alt-upstream",
	}

	out, err := translateRequestActionsCore(&executor.RequestExecutionResult{}, execCtx)
	require.NoError(t, err)
	assert.Equal(t, "/alternate", out.DynamicMetadata[constants.ExtProcFilterName]["target_upstream_base_path"])
}

func TestFailClosedOnUnavailableBackend(t *testing.T) {
	execCtx := newBackendRoutingExecCtx()
	ctx := context.Background()

	t.Run("a missing Backend becomes a 503 with a correlation id", func(t *testing.T) {
		resp, err := execCtx.failClosedOnUnavailableBackend(ctx, "request_headers", nil, &backendUnavailableError{name: "tenant-gone"})
		require.NoError(t, err)
		imm := resp.GetImmediateResponse()
		require.NotNil(t, imm)
		assert.Equal(t, int32(http.StatusServiceUnavailable), int32(imm.GetStatus().GetCode()))
		var errorID string
		for _, h := range imm.GetHeaders().GetSetHeaders() {
			if h.GetHeader().GetKey() == "x-error-id" {
				errorID = string(h.GetHeader().GetRawValue())
			}
		}
		assert.NotEmpty(t, errorID)
		assert.Contains(t, string(imm.GetBody()), errorID)
		assert.NotContains(t, string(imm.GetBody()), "tenant-gone", "the Backend name stays in the log")
		assert.Equal(t, constants.TerminalReasonBackendUnavailable, execCtx.generated.outcome.Reason)
	})

	t.Run("other outcomes pass through", func(t *testing.T) {
		want := &extprocv3.ProcessingResponse{}
		resp, err := execCtx.failClosedOnUnavailableBackend(ctx, "request_headers", want, nil)
		require.NoError(t, err)
		assert.Same(t, want, resp)

		other := errors.New("boom")
		_, err = execCtx.failClosedOnUnavailableBackend(ctx, "request_headers", nil, other)
		assert.Same(t, other, err)
	})
}

func TestUpstreamRedirectDirective_SetFromLabel(t *testing.T) {
	var d upstreamRedirectDirective
	d.setFromLabel("backend:tenant-a")
	require.NotNil(t, d.backend)
	assert.Nil(t, d.name)
	assert.Equal(t, "tenant-a", *d.backend)

	d.setFromLabel("alt-upstream")
	require.NotNil(t, d.name)
	assert.Nil(t, d.backend)
	assert.Equal(t, "alt-upstream", *d.name)
}

// A header-phase choice of a Backend the gateway lacks is not final while a buffered body phase
// is still to run: a body-phase policy may choose another target, and only if none does does the
// request fail closed. A streamed request, or a chain without body policies, fails at once.
func TestBackendRouting_MissingHeaderPhaseBackendIsDecidedInABufferedBodyPhase(t *testing.T) {
	withBackends(t, map[string]string{})
	bufferedChain := &registry.PolicyChain{RequiresRequestBody: true}
	missingBackend := headerPhase(policy.UpstreamRequestHeaderModifications{BackendName: strPtr("tenant-gone")})

	t.Run("a body-phase policy replaces the choice", func(t *testing.T) {
		execCtx := newBackendRoutingExecCtx()
		resp, err := TranslateRequestHeaderActions(missingBackend, bufferedChain, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "upstream_main_orders", targetUpstreamHeader(t, resp), "nothing is routed to the missing Backend")
		assert.Equal(t, "backend:tenant-gone",
			execCtx.dynamicMetadata[constants.ExtProcFilterName][constants.TargetUpstreamNameKey])

		_, err = translateRequestActionsCore(&executor.RequestExecutionResult{
			Results: []executor.RequestPolicyResult{{Action: policy.UpstreamRequestModifications{UpstreamName: strPtr("alt-upstream")}}},
		}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "alt-upstream", execCtx.dynamicMetadata[constants.ExtProcFilterName][constants.TargetUpstreamNameKey])
	})

	t.Run("nothing replaces it, so the body phase fails closed", func(t *testing.T) {
		execCtx := newBackendRoutingExecCtx()
		_, err := TranslateRequestHeaderActions(missingBackend, bufferedChain, execCtx)
		require.NoError(t, err)
		_, err = translateRequestActionsCore(&executor.RequestExecutionResult{}, execCtx)
		var missing *backendUnavailableError
		require.True(t, errors.As(err, &missing))
		assert.Equal(t, "tenant-gone", missing.name)
	})

	t.Run("a streamed request is routed at the header phase, so it fails at once", func(t *testing.T) {
		execCtx := newBackendRoutingExecCtx()
		execCtx.isStreamingRequest = true
		_, err := TranslateRequestHeaderActions(missingBackend, bufferedChain, execCtx)
		var missing *backendUnavailableError
		require.True(t, errors.As(err, &missing))
	})

	t.Run("a chain without body policies fails at once", func(t *testing.T) {
		_, err := TranslateRequestHeaderActions(missingBackend, &registry.PolicyChain{}, newBackendRoutingExecCtx())
		var missing *backendUnavailableError
		require.True(t, errors.As(err, &missing))
	})
}

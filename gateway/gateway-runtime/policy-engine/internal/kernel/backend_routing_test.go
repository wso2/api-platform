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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extprocconfigv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
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

// newBufferedExecCtx binds the execution context to a chain that buffers the request body, as
// the server binds a route's chain, so the header response is the one Envoy sees for that chain.
func newBufferedExecCtx(chain *registry.PolicyChain) *PolicyExecutionContext {
	execCtx := newBackendRoutingExecCtx()
	execCtx.policyChain = chain
	return execCtx
}

// requireBodyCallback checks that the header response asks Envoy for the buffered request body,
// the callback the body-phase translation runs in.
func requireBodyCallback(t *testing.T, resp *extprocv3.ProcessingResponse) {
	t.Helper()
	require.Equal(t, extprocconfigv3.ProcessingMode_BUFFERED, resp.GetModeOverride().GetRequestBodyMode())
}

// routedTo seeds the header phase's routing choice, as a header phase that settled on it leaves it.
func routedTo(execCtx *PolicyExecutionContext, name, backend *string) {
	execCtx.routing.set(name, backend, nil, nil)
}

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

	// The metadata reports the choice under a label that names the kind; the body phase starts
	// from the engine's own record of it.
	assert.Equal(t, "backend:tenant-a",
		execCtx.dynamicMetadata[constants.ExtProcFilterName][constants.TargetUpstreamNameKey])
	require.NotNil(t, execCtx.routing.backend)
	assert.Equal(t, "tenant-a", *execCtx.routing.backend)
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
	routedTo(execCtx, nil, strPtr("tenant-a"))

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
	routedTo(execCtx, nil, strPtr("tenant-a"))

	_, err := translateRequestActionsCore(&executor.RequestExecutionResult{}, execCtx)
	var missing *backendUnavailableError
	require.True(t, errors.As(err, &missing))
}

func TestBackendRouting_BodyPhaseKeepsAnUpstreamDefinition(t *testing.T) {
	withBackends(t, map[string]string{})
	execCtx := newBackendRoutingExecCtx()
	routedTo(execCtx, strPtr("alt-upstream"), nil)

	out, err := translateRequestActionsCore(&executor.RequestExecutionResult{}, execCtx)
	require.NoError(t, err)
	assert.Equal(t, "/alternate", out.DynamicMetadata[constants.ExtProcFilterName]["target_upstream_base_path"])
}

// The body phase restores routing from the engine's own state. A header-phase policy that writes
// the routing label into dynamic metadata, and sets no routing field, leaves the request on the
// route's default upstream: it can't reach a Backend, or skip a backendSelector, that way.
func TestBackendRouting_ABodyPhaseIgnoresARoutingLabelThatAPolicyWrote(t *testing.T) {
	withBackends(t, map[string]string{"tenant-a": "/v1"})
	bufferedChain := &registry.PolicyChain{RequiresRequestBody: true}

	for _, label := range []string{"backend:tenant-a", "alt-upstream"} {
		t.Run(label, func(t *testing.T) {
			execCtx := newBufferedExecCtx(bufferedChain)
			resp, err := TranslateRequestHeaderActions(headerPhase(policy.UpstreamRequestHeaderModifications{
				DynamicMetadata: map[string]map[string]any{
					constants.ExtProcFilterName: {constants.TargetUpstreamNameKey: label},
				},
			}), bufferedChain, execCtx)
			require.NoError(t, err)
			requireBodyCallback(t, resp)
			assert.Equal(t, "upstream_main_orders", targetUpstreamHeader(t, resp))
			require.Equal(t, label, execCtx.dynamicMetadata[constants.ExtProcFilterName][constants.TargetUpstreamNameKey],
				"the policy's metadata reached the execution context")

			out, err := translateRequestActionsCore(&executor.RequestExecutionResult{
				Results: []executor.RequestPolicyResult{{Action: policy.UpstreamRequestModifications{}}},
			}, execCtx)
			require.NoError(t, err)
			assert.Equal(t, "upstream_main_orders",
				out.DynamicMetadata[constants.ExtProcFilterName][constants.CurrentUpstreamKey].(map[string]interface{})["cluster_name"])
		})
	}
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

	t.Run("a templated Backend without a valid key becomes a 400, as the policy answers it", func(t *testing.T) {
		execCtx := newBackendRoutingExecCtx()
		resp, err := execCtx.failClosedOnUnavailableBackend(ctx, "request_headers", nil,
			&backendUnavailableError{name: "env-router", keyRejected: true})
		require.NoError(t, err)
		imm := resp.GetImmediateResponse()
		require.NotNil(t, imm)
		assert.Equal(t, int32(http.StatusBadRequest), int32(imm.GetStatus().GetCode()))
		assert.Contains(t, string(imm.GetBody()), "The requested backend is not available for this API.")
		assert.NotContains(t, string(imm.GetBody()), "env-router", "the Backend name stays in the log")
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

// A header-phase choice of a Backend the gateway lacks is not final while a buffered body phase
// is still to run: a body-phase policy may choose another target, and only if none does does the
// request fail closed. A streamed request, or a chain without body policies, fails at once.
func TestBackendRouting_MissingHeaderPhaseBackendIsDecidedInABufferedBodyPhase(t *testing.T) {
	withBackends(t, map[string]string{})
	bufferedChain := &registry.PolicyChain{RequiresRequestBody: true}
	missingBackend := headerPhase(policy.UpstreamRequestHeaderModifications{BackendName: strPtr("tenant-gone")})

	t.Run("a body-phase policy replaces the choice", func(t *testing.T) {
		execCtx := newBufferedExecCtx(bufferedChain)
		resp, err := TranslateRequestHeaderActions(missingBackend, bufferedChain, execCtx)
		require.NoError(t, err)
		requireBodyCallback(t, resp)
		assert.Equal(t, "upstream_main_orders", targetUpstreamHeader(t, resp), "nothing is routed to the missing Backend")
		require.NotNil(t, execCtx.routing.backend, "the choice is carried into the body phase")
		assert.Equal(t, "tenant-gone", *execCtx.routing.backend)

		_, err = translateRequestActionsCore(&executor.RequestExecutionResult{
			Results: []executor.RequestPolicyResult{{Action: policy.UpstreamRequestModifications{UpstreamName: strPtr("alt-upstream")}}},
		}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "alt-upstream", execCtx.dynamicMetadata[constants.ExtProcFilterName][constants.TargetUpstreamNameKey])
	})

	t.Run("nothing replaces it, so the body phase fails closed", func(t *testing.T) {
		execCtx := newBufferedExecCtx(bufferedChain)
		resp, err := TranslateRequestHeaderActions(missingBackend, bufferedChain, execCtx)
		require.NoError(t, err)
		requireBodyCallback(t, resp)
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

// publishBackends points the kernel's Backend lookup at a private store for the duration of the
// test. The returned function publishes a Backend record naming the cluster of one revision,
// replacing the record already there, as the gateway controller does on an update.
func publishBackends(t *testing.T) func(name, clusterName, basePath string) {
	t.Helper()
	store := policy.NewLazyResourceStore()
	previous := lookupBackend
	lookupBackend = store.LookupBackend
	t.Cleanup(func() { lookupBackend = previous })
	return func(name, clusterName, basePath string) {
		require.NoError(t, store.StoreResource(&policy.LazyResource{
			ID:           name,
			ResourceType: policy.LazyResourceTypeBackend,
			Resource: map[string]interface{}{
				policy.BackendResourceKeyName:        name,
				policy.BackendResourceKeyClusterName: clusterName,
				policy.BackendResourceKeyBasePath:    basePath,
			},
		}))
	}
}

func pinnedTo(name, clusterName string) policy.UpstreamRequestHeaderModifications {
	return policy.UpstreamRequestHeaderModifications{BackendName: strPtr(name), BackendClusterName: strPtr(clusterName)}
}

func requireRevisionChanged(t *testing.T, err error) {
	t.Helper()
	var unavailable *backendUnavailableError
	require.True(t, errors.As(err, &unavailable), "expected backendUnavailableError, got %v", err)
	assert.Equal(t, "tenant-a", unavailable.name)
	assert.True(t, unavailable.revisionChanged, "the Backend exists, on another revision")
}

// A policy that checked a Backend's record (its labels, say) pins the revision it checked. The
// request reaches the Backend only on that revision: if the Backend changed in between, the
// policy's check held for a revision the request would not reach, so it fails closed.
func TestBackendRouting_APinnedBackendIsRoutedOnlyOnTheRevisionThePolicyChecked(t *testing.T) {
	publish := publishBackends(t)
	publish("tenant-a", "backend_tenant-a_aaaaaaaaaaaa", "/v1")

	t.Run("the Backend is still on that revision", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(
			headerPhase(pinnedTo("tenant-a", "backend_tenant-a_aaaaaaaaaaaa")),
			&registry.PolicyChain{}, newBackendRoutingExecCtx())
		require.NoError(t, err)
		assert.Equal(t, "backend_tenant-a_aaaaaaaaaaaa", targetUpstreamHeader(t, resp))
	})

	t.Run("the Backend moved to another revision", func(t *testing.T) {
		_, err := TranslateRequestHeaderActions(
			headerPhase(pinnedTo("tenant-a", "backend_tenant-a_000000000000")),
			&registry.PolicyChain{}, newBackendRoutingExecCtx())
		requireRevisionChanged(t, err)
	})

	t.Run("an unpinned directive takes the current revision", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(
			headerPhase(policy.UpstreamRequestHeaderModifications{BackendName: strPtr("tenant-a")}),
			&registry.PolicyChain{}, newBackendRoutingExecCtx())
		require.NoError(t, err)
		assert.Equal(t, "backend_tenant-a_aaaaaaaaaaaa", targetUpstreamHeader(t, resp))
	})

	t.Run("a later result without a pin drops the earlier pin", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(
			headerPhase(pinnedTo("tenant-a", "backend_tenant-a_000000000000"),
				policy.UpstreamRequestHeaderModifications{BackendName: strPtr("tenant-a")}),
			&registry.PolicyChain{}, newBackendRoutingExecCtx())
		require.NoError(t, err)
		assert.Equal(t, "backend_tenant-a_aaaaaaaaaaaa", targetUpstreamHeader(t, resp))
	})
}

// The body phase re-resolves the header phase's Backend from its label. A pinned choice stays
// held to the policy's revision there, so a Backend that changes while the body is buffered fails
// the request closed; an unpinned choice follows the Backend to its new revision, as before.
func TestBackendRouting_TheBodyPhaseHoldsAPinnedBackendToThePolicysRevision(t *testing.T) {
	bufferedChain := &registry.PolicyChain{RequiresRequestBody: true}
	noRoutingChange := &executor.RequestExecutionResult{
		Results: []executor.RequestPolicyResult{{Action: policy.UpstreamRequestModifications{}}},
	}

	t.Run("pinned, and the Backend changed while the body was buffered", func(t *testing.T) {
		publish := publishBackends(t)
		publish("tenant-a", "backend_tenant-a_aaaaaaaaaaaa", "/v1")
		execCtx := newBufferedExecCtx(bufferedChain)
		resp, err := TranslateRequestHeaderActions(
			headerPhase(pinnedTo("tenant-a", "backend_tenant-a_aaaaaaaaaaaa")), bufferedChain, execCtx)
		require.NoError(t, err)
		requireBodyCallback(t, resp)

		publish("tenant-a", "backend_tenant-a_bbbbbbbbbbbb", "/v2")
		_, err = translateRequestActionsCore(noRoutingChange, execCtx)
		requireRevisionChanged(t, err)
	})

	t.Run("pinned, and the Backend did not change", func(t *testing.T) {
		publish := publishBackends(t)
		publish("tenant-a", "backend_tenant-a_aaaaaaaaaaaa", "/v1")
		execCtx := newBufferedExecCtx(bufferedChain)
		resp, err := TranslateRequestHeaderActions(
			headerPhase(pinnedTo("tenant-a", "backend_tenant-a_aaaaaaaaaaaa")), bufferedChain, execCtx)
		require.NoError(t, err)
		requireBodyCallback(t, resp)

		out, err := translateRequestActionsCore(noRoutingChange, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "backend_tenant-a_aaaaaaaaaaaa",
			out.DynamicMetadata[constants.ExtProcFilterName][constants.CurrentUpstreamKey].(map[string]interface{})["cluster_name"])
	})

	t.Run("unpinned, and the Backend changed while the body was buffered", func(t *testing.T) {
		publish := publishBackends(t)
		publish("tenant-a", "backend_tenant-a_aaaaaaaaaaaa", "/v1")
		execCtx := newBufferedExecCtx(bufferedChain)
		resp, err := TranslateRequestHeaderActions(
			headerPhase(policy.UpstreamRequestHeaderModifications{BackendName: strPtr("tenant-a")}), bufferedChain, execCtx)
		require.NoError(t, err)
		requireBodyCallback(t, resp)

		publish("tenant-a", "backend_tenant-a_bbbbbbbbbbbb", "/v2")
		out, err := translateRequestActionsCore(noRoutingChange, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "backend_tenant-a_bbbbbbbbbbbb",
			out.DynamicMetadata[constants.ExtProcFilterName][constants.CurrentUpstreamKey].(map[string]interface{})["cluster_name"])
	})

	t.Run("pinned to a revision that changed before the header phase resolved it, decided in the body phase", func(t *testing.T) {
		publish := publishBackends(t)
		publish("tenant-a", "backend_tenant-a_bbbbbbbbbbbb", "/v2")
		execCtx := newBufferedExecCtx(bufferedChain)
		resp, err := TranslateRequestHeaderActions(
			headerPhase(pinnedTo("tenant-a", "backend_tenant-a_aaaaaaaaaaaa")), bufferedChain, execCtx)
		require.NoError(t, err)
		requireBodyCallback(t, resp)
		assert.Equal(t, "upstream_main_orders", targetUpstreamHeader(t, resp), "nothing is routed to the changed Backend")

		_, err = translateRequestActionsCore(noRoutingChange, execCtx)
		requireRevisionChanged(t, err)
	})
}

// Only a nil BackendClusterName leaves a Backend unpinned. An empty one is a pin that no revision
// matches, so the request fails closed rather than reach whichever revision is current.
func TestBackendRouting_AnEmptyRevisionPinIsStillAPin(t *testing.T) {
	publish := publishBackends(t)
	publish("tenant-a", "backend_tenant-a_aaaaaaaaaaaa", "/v1")

	_, err := TranslateRequestHeaderActions(headerPhase(pinnedTo("tenant-a", "")),
		&registry.PolicyChain{}, newBackendRoutingExecCtx())
	requireRevisionChanged(t, err)

	t.Run("and the body phase keeps it", func(t *testing.T) {
		bufferedChain := &registry.PolicyChain{RequiresRequestBody: true}
		execCtx := newBufferedExecCtx(bufferedChain)
		resp, err := TranslateRequestHeaderActions(headerPhase(pinnedTo("tenant-a", "")), bufferedChain, execCtx)
		require.NoError(t, err)
		requireBodyCallback(t, resp)

		_, err = translateRequestActionsCore(&executor.RequestExecutionResult{}, execCtx)
		requireRevisionChanged(t, err)
	})
}

func TestUpstreamRedirectDirective_APinAppliesOnlyToABackend(t *testing.T) {
	var d upstreamRedirectDirective
	d.set(nil, strPtr("tenant-a"), strPtr("backend_tenant-a_aaaaaaaaaaaa"), nil)
	require.NotNil(t, d.backendCluster)
	assert.Equal(t, "backend_tenant-a_aaaaaaaaaaaa", *d.backendCluster)

	d.set(strPtr("alt-upstream"), nil, strPtr("backend_tenant-a_aaaaaaaaaaaa"), nil)
	assert.Nil(t, d.backendCluster, "a pin applies only to a Backend")

	d.set(nil, strPtr("tenant-a"), nil, nil)
	assert.Nil(t, d.backendCluster, "nil leaves the Backend unpinned")
}

// publishTemplated publishes a templated Backend record, as the controller does for a Backend
// whose endpoint host has the {key} placeholder.
func publishTemplated(t *testing.T, name, clusterName, urlTemplate string) {
	t.Helper()
	store := policy.NewLazyResourceStore()
	require.NoError(t, store.StoreResource(&policy.LazyResource{
		ID:           name,
		ResourceType: policy.LazyResourceTypeBackend,
		Resource: map[string]interface{}{
			policy.BackendResourceKeyName:        name,
			policy.BackendResourceKeyClusterName: clusterName,
			policy.BackendResourceKeyURLTemplate: urlTemplate,
		},
	}))
	require.NoError(t, store.StoreResource(&policy.LazyResource{
		ID:           "tenant-a",
		ResourceType: policy.LazyResourceTypeBackend,
		Resource: map[string]interface{}{
			policy.BackendResourceKeyName:        "tenant-a",
			policy.BackendResourceKeyClusterName: "backend_tenant-a_aaaaaaaaaaaa",
			policy.BackendResourceKeyBasePath:    "/v1",
		},
	}))
	previous := lookupBackend
	lookupBackend = store.LookupBackend
	t.Cleanup(func() { lookupBackend = previous })
}

func keyedTo(name, key string) policy.UpstreamRequestHeaderModifications {
	return policy.UpstreamRequestHeaderModifications{BackendName: strPtr(name), BackendKey: strPtr(key)}
}

// headerSet returns the value a header mutation sets for name, and whether it sets it.
func headerSet(m *extprocv3.HeaderMutation, name string) (string, bool) {
	for _, h := range m.GetSetHeaders() {
		if h.GetHeader().GetKey() == name {
			return string(h.GetHeader().GetRawValue()), true
		}
	}
	return "", false
}

func headerRemoved(m *extprocv3.HeaderMutation, name string) bool {
	for _, h := range m.GetRemoveHeaders() {
		if h == name {
			return true
		}
	}
	return false
}

func requestHeaderMutation(resp *extprocv3.ProcessingResponse) *extprocv3.HeaderMutation {
	return resp.GetRequestHeaders().GetResponse().GetHeaderMutation()
}

func requireKeyRejected(t *testing.T, err error) {
	t.Helper()
	var unavailable *backendUnavailableError
	require.True(t, errors.As(err, &unavailable), "expected backendUnavailableError, got %v", err)
	assert.Equal(t, "env-router", unavailable.name)
	assert.True(t, unavailable.keyRejected)
}

// A templated Backend is routed to its own cluster, a dynamic forward proxy cluster, and the
// host its key expands to goes in x-target-upstream-host for the proxy to resolve.
func TestBackendRouting_ATemplatedBackendGetsTheHostItsKeyExpandsTo(t *testing.T) {
	publishTemplated(t, "env-router", "backend_env-router_bbbbbbbbbbbb", "https://url-{key}.url.fr")
	execCtx := newBackendRoutingExecCtx()

	resp, err := TranslateRequestHeaderActions(headerPhase(keyedTo("env-router", "dev")), &registry.PolicyChain{}, execCtx)
	require.NoError(t, err)

	mutation := requestHeaderMutation(resp)
	assert.Equal(t, "backend_env-router_bbbbbbbbbbbb", targetUpstreamHeader(t, resp))
	host, ok := headerSet(mutation, constants.TargetUpstreamHostHeader)
	require.True(t, ok, "the expanded host is set")
	assert.Equal(t, "url-dev.url.fr", host)

	current := execCtx.dynamicMetadata[constants.ExtProcFilterName][constants.CurrentUpstreamKey].(map[string]interface{})
	assert.Equal(t, "https://url-dev.url.fr", current["url"])
	assert.Equal(t, "/", current["base_path"], "a templated Backend forwards the API-relative path")
}

func TestBackendRouting_ATemplatedBackendWithAPortKeepsItInTheHost(t *testing.T) {
	publishTemplated(t, "env-router", "backend_env-router_bbbbbbbbbbbb", "http://env-{key}.testbench:3000")
	resp, err := TranslateRequestHeaderActions(headerPhase(keyedTo("env-router", "prod")),
		&registry.PolicyChain{}, newBackendRoutingExecCtx())
	require.NoError(t, err)
	host, _ := headerSet(requestHeaderMutation(resp), constants.TargetUpstreamHostHeader)
	assert.Equal(t, "env-prod.testbench:3000", host)
}

// Without a valid key the request fails closed. The proxy must never fall back to resolving the
// client's own Host, which would make the gateway an open proxy.
func TestBackendRouting_ATemplatedBackendWithoutAValidKeyFailsClosed(t *testing.T) {
	publishTemplated(t, "env-router", "backend_env-router_bbbbbbbbbbbb", "https://url-{key}.url.fr")
	for name, mods := range map[string]policy.UpstreamRequestHeaderModifications{
		"no key":              {BackendName: strPtr("env-router")},
		"empty key":           keyedTo("env-router", ""),
		"a dotted key":        keyedTo("env-router", "dev.evil.io"),
		"a key with a port":   keyedTo("env-router", "dev:1"),
		"a key with userinfo": keyedTo("env-router", "x@169.254.169.254"),
		"an upper-case key":   keyedTo("env-router", "DEV"),
		"a key too long":      keyedTo("env-router", strings.Repeat("a", 60)),
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := TranslateRequestHeaderActions(headerPhase(mods), &registry.PolicyChain{}, newBackendRoutingExecCtx())
			assert.Nil(t, resp)
			requireKeyRejected(t, err)
		})
	}
}

func TestBackendRouting_AKeyIsIgnoredForAnyOtherTarget(t *testing.T) {
	publishTemplated(t, "env-router", "backend_env-router_bbbbbbbbbbbb", "https://url-{key}.url.fr")

	t.Run("a plain Backend", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(headerPhase(keyedTo("tenant-a", "dev")),
			&registry.PolicyChain{}, newBackendRoutingExecCtx())
		require.NoError(t, err)
		assert.Equal(t, "backend_tenant-a_aaaaaaaaaaaa", targetUpstreamHeader(t, resp))
		_, set := headerSet(requestHeaderMutation(resp), constants.TargetUpstreamHostHeader)
		assert.False(t, set)
	})

	t.Run("the API's own upstream definition of the same name", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(headerPhase(policy.UpstreamRequestHeaderModifications{
			UpstreamName: strPtr("alt-upstream"), BackendName: strPtr("alt-upstream"), BackendKey: strPtr("dev"),
		}), &registry.PolicyChain{}, newBackendRoutingExecCtx())
		require.NoError(t, err)
		_, set := headerSet(requestHeaderMutation(resp), constants.TargetUpstreamHostHeader)
		assert.False(t, set)
	})
}

// Only the engine chooses the host the proxy resolves. A value the client sent or a policy set is
// removed when the request goes anywhere but a templated Backend, and replaced when it goes to one.
func TestBackendRouting_OnlyTheEngineSetsTheTargetHost(t *testing.T) {
	publishTemplated(t, "env-router", "backend_env-router_bbbbbbbbbbbb", "https://url-{key}.url.fr")
	clientSent := func() *PolicyExecutionContext {
		execCtx := newBackendRoutingExecCtx()
		execCtx.requestBodyCtx.Headers = policy.NewHeaders(map[string][]string{
			constants.TargetUpstreamHostHeader: {"169.254.169.254"},
		})
		return execCtx
	}
	policySet := policy.UpstreamRequestHeaderModifications{
		HeadersToSet: map[string]string{constants.TargetUpstreamHostHeader: "metadata.internal"},
	}

	t.Run("sent by the client, routed to the default upstream", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(headerPhase(policy.UpstreamRequestHeaderModifications{}),
			&registry.PolicyChain{}, clientSent())
		require.NoError(t, err)
		assert.True(t, headerRemoved(requestHeaderMutation(resp), constants.TargetUpstreamHostHeader))
	})

	t.Run("sent by the client, routed to a plain Backend", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(headerPhase(keyedTo("tenant-a", "dev")),
			&registry.PolicyChain{}, clientSent())
		require.NoError(t, err)
		assert.True(t, headerRemoved(requestHeaderMutation(resp), constants.TargetUpstreamHostHeader))
	})

	t.Run("set by a policy, routed to a plain Backend", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(headerPhase(policySet, keyedTo("tenant-a", "dev")),
			&registry.PolicyChain{}, newBackendRoutingExecCtx())
		require.NoError(t, err)
		mutation := requestHeaderMutation(resp)
		_, set := headerSet(mutation, constants.TargetUpstreamHostHeader)
		assert.False(t, set)
		assert.True(t, headerRemoved(mutation, constants.TargetUpstreamHostHeader))
	})

	t.Run("set by a policy, routed to a templated Backend", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(headerPhase(policySet, keyedTo("env-router", "dev")),
			&registry.PolicyChain{}, clientSent())
		require.NoError(t, err)
		host, _ := headerSet(requestHeaderMutation(resp), constants.TargetUpstreamHostHeader)
		assert.Equal(t, "url-dev.url.fr", host)
	})

	t.Run("not sent and not set: no op", func(t *testing.T) {
		resp, err := TranslateRequestHeaderActions(headerPhase(policy.UpstreamRequestHeaderModifications{}),
			&registry.PolicyChain{}, newBackendRoutingExecCtx())
		require.NoError(t, err)
		assert.False(t, headerRemoved(requestHeaderMutation(resp), constants.TargetUpstreamHostHeader))
	})
}

// The body phase restores the header phase's choice with its key, so a buffered request is routed
// to the same host; a body-phase policy that names the Backend without a key fails closed.
func TestBackendRouting_TheBodyPhaseKeepsATemplatedBackendsKey(t *testing.T) {
	publishTemplated(t, "env-router", "backend_env-router_bbbbbbbbbbbb", "https://url-{key}.url.fr")
	bufferedChain := &registry.PolicyChain{RequiresRequestBody: true}

	t.Run("no body-phase change", func(t *testing.T) {
		execCtx := newBufferedExecCtx(bufferedChain)
		resp, err := TranslateRequestHeaderActions(headerPhase(keyedTo("env-router", "dev")), bufferedChain, execCtx)
		require.NoError(t, err)
		requireBodyCallback(t, resp)

		out, err := translateRequestActionsCore(&executor.RequestExecutionResult{
			Results: []executor.RequestPolicyResult{{Action: policy.UpstreamRequestModifications{}}},
		}, execCtx)
		require.NoError(t, err)
		host, ok := headerSet(out.HeaderMutation, constants.TargetUpstreamHostHeader)
		require.True(t, ok)
		assert.Equal(t, "url-dev.url.fr", host)
	})

	t.Run("a body-phase policy names the Backend without a key", func(t *testing.T) {
		execCtx := newBufferedExecCtx(bufferedChain)
		_, err := TranslateRequestHeaderActions(headerPhase(keyedTo("env-router", "dev")), bufferedChain, execCtx)
		require.NoError(t, err)

		_, err = translateRequestActionsCore(&executor.RequestExecutionResult{
			Results: []executor.RequestPolicyResult{{Action: policy.UpstreamRequestModifications{BackendName: strPtr("env-router")}}},
		}, execCtx)
		requireKeyRejected(t, err)
	})
}

// A fixed target names an upstream definition and a Backend at once. When the API has an upstream
// definition of that name, the request goes there without a key, even if a templated Backend of
// the same name exists; only a request that ends up at the templated Backend needs one.
func TestBackendRouting_AnAPIsOwnDefinitionNeedsNoKeyNextToATemplatedBackend(t *testing.T) {
	publishTemplated(t, "alt-upstream", "backend_alt-upstream_bbbbbbbbbbbb", "https://url-{key}.url.fr")
	both := policy.UpstreamRequestHeaderModifications{UpstreamName: strPtr("alt-upstream"), BackendName: strPtr("alt-upstream")}

	resp, err := TranslateRequestHeaderActions(headerPhase(both), &registry.PolicyChain{}, newBackendRoutingExecCtx())
	require.NoError(t, err)
	assert.Equal(t, constants.UpstreamDefinitionClusterPrefix+"API_api-123_"+sanitizeUpstreamDefinitionName("alt-upstream"),
		targetUpstreamHeader(t, resp))

	t.Run("an API without that definition reaches the templated Backend, which needs the key", func(t *testing.T) {
		execCtx := newBackendRoutingExecCtx()
		execCtx.upstreamDefinitionPaths = nil
		_, err := TranslateRequestHeaderActions(headerPhase(both), &registry.PolicyChain{}, execCtx)
		var unavailable *backendUnavailableError
		require.True(t, errors.As(err, &unavailable))
		assert.True(t, unavailable.keyRejected)
	})
}

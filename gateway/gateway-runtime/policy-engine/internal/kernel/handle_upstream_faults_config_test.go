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
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// upstreamFaultCtx builds a context whose fault-flow-for-upstream-errors setting is the
// thing under test, so both states are reachable from one fixture.
//
// Deliberately its own fixture rather than a parameter on faultExecCtx: that one opts in
// unconditionally because every test using it is about the fault flow, and threading a flag
// through eighty-seven call sites to serve four tests would make the common case worse.
func upstreamFaultCtx(t *testing.T, enabled bool, faultPolicies []policy.Policy) *PolicyExecutionContext {
	t.Helper()
	k := NewKernel()
	chainExecutor := executor.NewChainExecutor(nil, nil, noop.NewTracerProvider().Tracer(""))
	server := NewExternalProcessorServer(k, chainExecutor, config.TracingConfig{}, "", 1<<20, 1<<20,
		WithHandleUpstreamFaults(enabled))

	chain := &registry.PolicyChain{
		Policies: []policy.Policy{
			&faultRecorderPolicy{name: "transformer", order: new([]string)},
			&faultRecorderPolicy{name: "collector", order: new([]string)},
		},
		PolicySpecs: []policy.PolicySpec{
			{Name: "json-xml-mediator", Version: "v1", Enabled: true},
			// The literal name, not systemPolicyPrefix + "analytics": deriving it from
			// the constant under test would move both sides together.
			{Name: "wso2_apip_sys_analytics", Version: "v1", Enabled: true},
		},
		FaultPolicies:    faultPolicies,
		FaultPolicySpecs: specs(len(faultPolicies)),
		HasFaultPolicies: len(faultPolicies) > 0,
	}
	ec := newPolicyExecutionContext(server, "test-route", chain)
	ec.sharedCtx = &policy.SharedContext{APIName: "TestAPI", APIVersion: "v1.0"}
	return ec
}

// Disabled is the default and the shipped behaviour: an upstream or router error is an
// ordinary response. Every previous generation of this gateway ran one through the response
// policies, so this is the state that must not change.
func TestHandleUpstreamFaults_DisabledKeepsLegacyBehaviour(t *testing.T) {
	t.Run("the flag is off unless asked for", func(t *testing.T) {
		ec := upstreamFaultCtx(t, false, nil)
		assert.False(t, ec.handlesUpstreamFaults(),
			"a deployment that configured nothing must get the legacy path")
	})

	t.Run("a backend error is not noted as a fault", func(t *testing.T) {
		ec := upstreamFaultCtx(t, false, nil)
		ec.noteUpstreamFault(503)
		assert.False(t, ec.upstreamFault)
	})

	// The load-bearing assertion. With the flag off the collector must stay in the response
	// chain, because its OnFault will not be called for this response — the fault chain does
	// not run. Filtering it out here as well would lose the backend 5xx from analytics
	// entirely, which is strictly worse than either behaviour on its own.
	t.Run("every response policy still runs, collector included", func(t *testing.T) {
		ec := upstreamFaultCtx(t, false, nil)
		ec.noteUpstreamFault(503)

		pols, specs := ec.responsePolicies()

		require.Len(t, pols, 2, "nothing is filtered when the fault flow is not handling this")
		require.Len(t, specs, 2)
		assert.Equal(t, "json-xml-mediator", specs[0].Name)
		assert.Equal(t, "wso2_apip_sys_analytics", specs[1].Name,
			"the collector must record the backend error from the response chain")
	})

	t.Run("the fault chain does not run for an upstream error", func(t *testing.T) {
		var order []string
		ec := upstreamFaultCtx(t, false, []policy.Policy{
			&faultRecorderPolicy{name: "notify", order: &order},
		})
		ec.responseBodyCtx = &policy.ResponseContext{
			SharedContext:  ec.sharedCtx,
			ResponseStatus: 503,
		}

		ran := ec.runFaultPoliciesOnResponse(context.Background(),
			&executor.ResponseExecutionResult{}, originUpstream)

		assert.False(t, ran, "an upstream error is left to the response policies")
		assert.Empty(t, order, "no fault entry may fire")
		assert.False(t, ec.faultPoliciesRan)
	})
}

// Enabled is the opt-in: the fault policies handle the failure instead of the response
// policies, and the collector records from the end of the fault chain.
func TestHandleUpstreamFaults_EnabledRoutesThroughTheFaultFlow(t *testing.T) {
	t.Run("a backend error is noted as a fault", func(t *testing.T) {
		ec := upstreamFaultCtx(t, true, nil)
		ec.noteUpstreamFault(503)
		assert.True(t, ec.upstreamFault)
	})

	t.Run("no response policy runs, collector included", func(t *testing.T) {
		ec := upstreamFaultCtx(t, true, nil)
		ec.noteUpstreamFault(503)

		pols, specs := ec.responsePolicies()

		assert.Empty(t, pols, "the fault policies handle this response instead")
		assert.Empty(t, specs)
	})

	t.Run("the fault chain runs for an upstream error", func(t *testing.T) {
		var order []string
		ec := upstreamFaultCtx(t, true, []policy.Policy{
			&faultRecorderPolicy{name: "notify", order: &order},
		})
		ec.responseBodyCtx = &policy.ResponseContext{
			SharedContext:  ec.sharedCtx,
			ResponseStatus: 503,
		}

		ran := ec.runFaultPoliciesOnResponse(context.Background(),
			&executor.ResponseExecutionResult{}, originUpstream)

		assert.True(t, ran)
		assert.Equal(t, []string{"notify"}, order)
	})
}

// The flag governs only failures the gateway did NOT produce. A policy's own rejection always
// reaches its fault policies: that is the contract the policy opted into by declaring the
// fault, and nothing reached the fault flow before the feature existed, so there is no prior
// behaviour that exempting it could change.
func TestHandleUpstreamFaults_PolicyRejectionsAreNeverGated(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run("handle_upstream_faults="+map[bool]string{true: "true", false: "false"}[enabled], func(t *testing.T) {
			var order []string
			ec := upstreamFaultCtx(t, enabled, []policy.Policy{
				&faultRecorderPolicy{name: "notify", order: &order},
			})
			ec.responseBodyCtx = &policy.ResponseContext{
				SharedContext:  ec.sharedCtx,
				ResponseStatus: 401,
			}

			ran := ec.runFaultPoliciesOnResponse(context.Background(),
				&executor.ResponseExecutionResult{}, originGateway)

			assert.True(t, ran, "a policy-produced fault runs its fault policies either way")
			assert.Equal(t, []string{"notify"}, order)
		})
	}
}

// The status floor still applies on top of the flag — enabling it must not turn every
// successful response into a fault. 400 is the first failing status, not 401: the existing
// faultMinStatus is a >= comparison, and a 400 Bad Request is an error.
func TestHandleUpstreamFaults_EnabledStillRespectsTheStatusFloor(t *testing.T) {
	for _, tc := range []struct {
		status int
		fault  bool
	}{{200, false}, {302, false}, {399, false}, {400, true}, {404, true}, {503, true}} {
		ec := upstreamFaultCtx(t, true, nil)
		ec.noteUpstreamFault(tc.status)
		assert.Equal(t, tc.fault, ec.upstreamFault, "status %d", tc.status)
	}
}

// The flag governs the fault POLICIES, never the formatter — and the formatter itself renders
// only what a policy described.
//
// An upstream 503 is described by no policy, so with the flag off nothing changes it: the
// operator's fault policies do not run, and the formatter, consulted regardless of the flag,
// declines. That is the released behaviour for an upstream error, which this flag being off
// exists to keep.
func TestHandleUpstreamFaults_DisabledLeavesAnUndescribedUpstreamErrorAlone(t *testing.T) {
	ec := upstreamFaultCtx(t, false, nil)
	ec.sharedCtx.APIKind = policy.APIKindMCP
	enableFaultFormatter(t, ec, policy.APIKindMCP)
	ec.responseBodyCtx = &policy.ResponseContext{
		SharedContext:  ec.sharedCtx,
		ResponseStatus: 503,
	}

	execResult := &executor.ResponseExecutionResult{}
	changed := ec.runFaultPoliciesOnResponse(context.Background(), execResult, originUpstream)

	assert.False(t, changed, "no policy described an upstream error, so nothing renders it")
	assert.Empty(t, execResult.Results)
	assert.False(t, ec.faultPoliciesRan,
		"and the operator's fault policies must NOT have run — that is what the flag gates")
}

// The mirror of the above, and the one that keeps the default honest: with formatting off for
// the kind (the shipped state, supportedKinds empty) an upstream error is left exactly as the
// upstream sent it.
func TestHandleUpstreamFaults_DisabledLeavesTheResponseAloneWhenNothingFormats(t *testing.T) {
	ec := upstreamFaultCtx(t, false, nil)
	ec.sharedCtx.APIKind = policy.APIKindRestApi
	ec.responseBodyCtx = &policy.ResponseContext{
		SharedContext:  ec.sharedCtx,
		ResponseStatus: 503,
	}

	changed := ec.runFaultPoliciesOnResponse(context.Background(),
		&executor.ResponseExecutionResult{}, originUpstream)

	assert.False(t, changed, "no chain, no formatter for this kind: nothing to change")
	assert.False(t, ec.faultPoliciesRan)
}

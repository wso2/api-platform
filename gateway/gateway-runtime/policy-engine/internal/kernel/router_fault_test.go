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

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// A router failure has no producing policy, so the engine describes it from Envoy's
// code_details. Without this both the fault chain and the formatter see nothing at all.
func TestRouterErrorFor_DescribesEachKnownFailure(t *testing.T) {
	cases := []struct {
		name        string
		src         faultSource
		details     string
		wantCode    string
		wantType    string
		wantMessage string
	}{
		{"no healthy upstream", sourceRouter, codeDetailsNoHealthyUpstream,
			codeUpstreamUnavailable, policy.FaultTypeUpstream, "The upstream service is unavailable."},
		{"response timeout", sourceRouter, codeDetailsResponseTimeout,
			codeUpstreamTimeout, policy.FaultTypeUpstream, "The upstream service did not respond in time."},
		{"connection failure", sourceRouter, "upstream_reset_before_response_started{connection_failure}",
			codeUpstreamUnreachable, policy.FaultTypeUpstream, "The upstream service could not be reached."},
		{"reset family, unseen suffix", sourceRouter, "upstream_reset_after_response_started{something_new}",
			codeUpstreamUnreachable, policy.FaultTypeUpstream, "The upstream service could not be reached."},
		{"no route", sourceNoRoute, codeDetailsDirectResponse,
			codeNoRoute, policy.FaultTypeRouting, "No API matched the request."},
		{"unrecognised detail still described", sourceRouter, "some_future_envoy_reason",
			codeUpstreamGeneric, policy.FaultTypeUpstream, "The request could not be completed."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := routerErrorFor(tc.src, tc.details)
			require.NotNil(t, got)
			assert.Equal(t, tc.wantCode, got.Code)
			assert.Equal(t, tc.wantType, got.Type)
			assert.Equal(t, tc.wantMessage, got.Message)
			assert.Empty(t, got.Policy, "no policy caused a router failure, so Policy must stay empty")
		})
	}
}

// Only the router's own failures are the engine's to describe.
func TestRouterErrorFor_SilentForOtherSources(t *testing.T) {
	for _, src := range []faultSource{sourceGateway, sourceBackend, sourceUnknown} {
		assert.Nil(t, routerErrorFor(src, codeDetailsNoHealthyUpstream),
			"source %q must not be described by the engine", src)
	}
}

// Envoy's own wording must never reach the client. It names internal proxy mechanics, which
// error-handling.md directive 1 keeps out of responses; the raw value is logged instead.
func TestRouterErrorFor_DoesNotLeakEnvoyWording(t *testing.T) {
	raw := "upstream connect error or disconnect/reset before headers. reset reason: remote connection failure"
	got := routerErrorFor(sourceRouter, raw)
	require.NotNil(t, got)
	assert.NotContains(t, got.Message, "reset reason")
	assert.NotContains(t, got.Message, "connect error")
	assert.NotContains(t, got.Description, "reset reason")
}

// The synthesized error must reach FAULT POLICIES too, not only the formatter: a policy
// notifying on an unreachable upstream should be able to report a code and class rather than
// only a status.
func TestRouterError_ReachesTheFaultPolicy(t *testing.T) {
	var order []string
	var saw policy.FaultContext
	ec := faultExecCtx(t, []policy.Policy{
		&faultRecorderPolicy{name: "notify", order: &order, sawErrCtx: &saw},
	}, specs(1))
	withRejectedBody(ec, 503)
	ec.responseCodeDetails = codeDetailsNoHealthyUpstream

	require.True(t, ec.runFaultPoliciesOnResponse(context.Background(),
		&executor.ResponseExecutionResult{}, originUpstream))

	require.Equal(t, []string{"notify"}, order)
	// There is no Source field to read: a handler tells an infrastructure failure from a
	// policy rejection by Policy being empty, and identifies which one from Error.Code.
	assert.Empty(t, saw.Policy, "no policy caused a router failure")
	require.NotNil(t, saw.Fault, "a fault policy must receive the engine's description of a router failure")
	assert.Equal(t, codeUpstreamUnavailable, saw.Fault.Code)
	assert.Equal(t, policy.FaultTypeUpstream, saw.Fault.Type)
	assert.Empty(t, saw.Fault.Policy, "no policy caused it")
}

// A router failure is described by the engine, not by a policy, so the formatter leaves it as
// released gateways sent it — on every kind, enabled or not.
//
// The fault CHAIN still receives the engine's description (the test above); only the client
// body is unchanged. A fault entry that re-describes the failure does get it rendered — see
// TestFaultRedescription_TheResponseFormatterRendersIt.
func TestRouterError_IsNotFormatted(t *testing.T) {
	for _, kind := range []policy.APIKind{policy.APIKindRestApi, policy.APIKindMCP} {
		t.Run(string(kind), func(t *testing.T) {
			ec := faultExecCtx(t, nil, nil) // no fault policies at all
			ec.sharedCtx.APIKind = kind
			enableFaultFormatter(t, ec, kind)
			withRejectedBody(ec, 503)
			ec.responseCodeDetails = codeDetailsNoHealthyUpstream

			execResult := &executor.ResponseExecutionResult{}
			ec.runFaultPoliciesOnResponse(context.Background(), execResult, originUpstream)

			for _, r := range execResult.Results {
				if r.PolicyName == errorFormatResultName {
					t.Fatalf("a router failure must not be formatted: %s",
						r.Action.(policy.DownstreamResponseModifications).Body)
				}
			}
		})
	}
}

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

package pythonbridge

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// A Python policy becomes eligible for the fault path by defining on_fault, and the gateway
// decides eligibility with a type assertion to policy.FaultPolicy. So the executor's answer
// has to change the Go TYPE, not a field read later: a plain bridge must fail the assertion,
// and only the wrapper the factory builds for an on_fault-capable policy may pass it.
func TestFaultBridge_OnlyTheOnFaultCapableBridgeSatisfiesFaultPolicy(t *testing.T) {
	var plain policy.Policy = &bridge{}
	_, plainIsFaultCapable := plain.(policy.FaultPolicy)
	assert.False(t, plainIsFaultCapable,
		"a Python policy without on_fault must not advertise the fault contract")

	var capable policy.Policy = &faultBridge{bridge: &bridge{}}
	_, capableIsFaultCapable := capable.(policy.FaultPolicy)
	assert.True(t, capableIsFaultCapable, "faultBridge must implement FaultPolicy")

	// The wrapper must still be a complete policy: the fault chain turns the response-body
	// requirement on for the route, and the same instance serves its normal-path phases.
	_, hasRequestHeaders := capable.(policy.RequestHeaderPolicy)
	_, hasResponseBody := capable.(policy.ResponsePolicy)
	assert.True(t, hasRequestHeaders, "faultBridge must keep the request-header contract")
	assert.True(t, hasResponseBody, "faultBridge must keep the response-body contract")
}

func TestBuildErrorRequestCarriesTheFailureDescription(t *testing.T) {
	b := &faultBridge{bridge: &bridge{
		policyName:    "fault-notifier",
		policyVersion: "v1.0.0",
		metadata:      policy.PolicyMetadata{RouteName: "route-a"},
		translator:    NewTranslator(),
		instanceID:    "instance-1",
	}}

	faultCtx := &policy.FaultContext{
		SharedContext:     &policy.SharedContext{RequestID: "shared-1", APIName: "PetStore"},
		RequestHeaders:    policy.NewHeaders(map[string][]string{"X-Trace": {"one"}}),
		RequestPath:       "/petstore/v1/pets/123",
		RequestMethod:     "GET",
		ResponseHeaders:   policy.NewHeaders(map[string][]string{"Content-Type": {"application/json"}}),
		ResponseBody:      &policy.Body{Content: []byte(`{"code":"906000"}`), Present: true},
		ResponseStatus:    422,
		OriginalStatus:    200,
		Policy:            "word-count-guardrail",
		PolicyVersion:     "v1.0.0",
		PolicyPhase:       policy.PolicyPhaseResponseBody,
		ResponseCommitted: true,
		RouteKey:          "route-a",
		Source:            policy.FaultSourceGateway,
		Fault: &policy.FaultDetails{
			Code:      "906000",
			Type:      "guardrail",
			Direction: policy.DirectionResponse,
			Message:   "blocked by guardrail",
		},
	}

	req, err := b.buildErrorRequest(context.Background(), faultCtx, map[string]interface{}{"target": "slack"})
	require.NoError(t, err)

	assert.Equal(t, proto.Phase_PHASE_FAULT, req.GetExecutionMetadata().GetPhase())
	assert.Equal(t, "shared-1", req.GetSharedContext().GetRequestId())
	assert.Equal(t, "slack", req.GetParams().GetFields()["target"].GetStringValue())

	got := req.GetFaultContext().GetContext()
	require.NotNil(t, got)
	assert.EqualValues(t, 200, got.GetOriginalStatus())
	assert.Equal(t, "word-count-guardrail", got.GetPolicy())
	assert.Equal(t, "v1.0.0", got.GetPolicyVersion())
	assert.Equal(t, policy.PolicyPhaseResponseBody, got.GetPolicyPhase(),
		"the phase completes the attribution, so it has to cross the wire with the name")
	assert.True(t, got.GetResponseCommitted(),
		"a handler must be able to see that the response already reached the client")
	assert.Equal(t, "route-a", got.GetRouteKey())
	assert.Equal(t, policy.FaultSourceGateway, got.GetSource(),
		"a Python handler cannot tell a policy rejection from an upstream failure without this")

	assert.Equal(t, "906000", got.GetFault().GetCode())
	assert.Equal(t, "guardrail", got.GetFault().GetType())
	assert.Equal(t, policy.DirectionResponse, got.GetFault().GetDirection())
	assert.Equal(t, "blocked by guardrail", got.GetFault().GetMessage())

	// The response fields sit directly on the message now — there is no nested view to
	// reach through, on either side of the wire.
	assert.EqualValues(t, 422, got.GetResponseStatus())
	assert.Equal(t, "/petstore/v1/pets/123", got.GetRequestPath())
	assert.Equal(t, "GET", got.GetRequestMethod())
	assert.Equal(t, []string{"one"}, got.GetRequestHeaders().GetValues()["x-trace"].GetValues())
	assert.Equal(t, []byte(`{"code":"906000"}`), got.GetResponseBody().GetContent())
}

// A backend error is the case the source field exists for: the gateway describes nothing,
// because another service's 500 is not the gateway's to classify, so source is the ONLY thing
// telling a Python handler what it is looking at. If it did not cross the wire, that handler
// would see a bare 502 indistinguishable from an engine failure.
func TestBuildErrorRequestCarriesTheSourceForAnUndescribedBackendFailure(t *testing.T) {
	b := &faultBridge{bridge: &bridge{translator: NewTranslator()}}

	req, err := b.buildErrorRequest(context.Background(), &policy.FaultContext{
		ResponseStatus: 502,
		Source:         policy.FaultSourceBackend,
	}, nil)
	require.NoError(t, err)

	got := req.GetFaultContext().GetContext()
	assert.Equal(t, policy.FaultSourceBackend, got.GetSource())
	assert.Nil(t, got.GetFault(),
		"the gateway must not invent a description for a failure another service produced")
}

// nil in, nil out. A failure nothing described has to stay distinguishable from one described
// with empty fields, because only the former means the handler must fall back to the status.
func TestBuildErrorRequestLeavesAnUndescribedFailureUndescribed(t *testing.T) {
	b := &faultBridge{bridge: &bridge{translator: NewTranslator()}}

	req, err := b.buildErrorRequest(context.Background(), &policy.FaultContext{
		ResponseStatus: 503,
	}, nil)
	require.NoError(t, err)

	assert.Nil(t, req.GetFaultContext().GetContext().GetFault())
}

// The client is already receiving an error when this runs, so a bridge failure must leave the
// response alone rather than replacing a real failure with a 500 the operator never configured.
func TestFaultBridgeOnErrorLeavesTheResponseAloneWhenTheBridgeFails(t *testing.T) {
	b := &faultBridge{bridge: &bridge{
		policyName:    "fault-notifier",
		policyVersion: "v1.0.0",
		translator:    NewTranslator(),
		slogger:       slog.Default(),
	}}

	action := b.OnFault(context.Background(), nil, nil)

	assert.Nil(t, action, "a failing fault entry must not author a response")
}

// The executor's answer has to survive as a type all the way to chain build, where the only
// question asked is "does this satisfy FaultPolicy?". A Python policy that implements no
// on_fault must fail that question — which is what causes the entry to be dropped from the
// fault chain rather than dispatched through some other hook.
func TestWrapBridge_ExpressesTheOnErrorCapabilityAsAType(t *testing.T) {
	base := &bridge{policyName: "some-python-policy", policyVersion: "v1.0.0"}

	withoutHandler := wrapBridge(base, policyCapabilities{responseBody: true})
	_, eligible := withoutHandler.(policy.FaultPolicy)
	assert.False(t, eligible,
		"a Python policy with a response hook but no on_fault must not be usable as a fault policy")

	withHandler := wrapBridge(base, policyCapabilities{onFault: true})
	_, eligible = withHandler.(policy.FaultPolicy)
	assert.True(t, eligible, "a Python policy that defines on_fault must be usable as a fault policy")

	// A fault-only policy participates in no phase. That must not be read as "not a policy":
	// the capability is independent of every processing mode.
	faultOnly := wrapBridge(base, policyCapabilities{onFault: true})
	_, isPolicy := faultOnly.(policy.Policy)
	assert.True(t, isPolicy, "a fault-only Python policy is still a policy")
}

// A Python fault handler needs the withheld detail too: reporting a guardrail intervention to
// an audit sink is exactly the case where Description is the useful field, and it is the one
// field no renderer will ever put in front of the caller.
func TestBuildErrorRequestCarriesTheWithheldDescription(t *testing.T) {
	b := &faultBridge{bridge: &bridge{translator: NewTranslator()}}

	req, err := b.buildErrorRequest(context.Background(), &policy.FaultContext{
		ResponseStatus: 422,
		Fault: &policy.FaultDetails{
			Code:        "906000",
			Message:     "Request blocked by a guardrail",
			Description: "the blocked content itself",
		},
	}, nil)
	require.NoError(t, err)

	got := req.GetFaultContext().GetContext().GetFault()
	require.NotNil(t, got)
	assert.Equal(t, "the blocked content itself", got.GetDescription())
}

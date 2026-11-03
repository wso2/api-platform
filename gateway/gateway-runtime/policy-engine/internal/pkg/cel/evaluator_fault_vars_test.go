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

package cel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// The condition that motivated exposing the failure at all.
//
// Backend errors now reach a declared fault chain, so a deployment that only wants the
// gateway's own failures needs a way to say so. This is that way, and it has to work for
// every source rather than just the two the example names.
func TestFaultCondition_NarrowsOnSource(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	cases := []struct {
		source      string
		gatewayOnly bool
		notFromBack bool
	}{
		{policy.FaultSourceGateway, true, true},
		{policy.FaultSourceRouter, false, true},
		{policy.FaultSourceNoRoute, false, true},
		{policy.FaultSourceUnknown, false, true},
		{policy.FaultSourceBackend, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			faultCtx := errorCtxWithSource(tc.source)

			got, evalErr := evaluator.EvaluateFaultCondition(`fault.Source == "gateway"`, faultCtx)
			require.NoError(t, evalErr)
			assert.Equal(t, tc.gatewayOnly, got, `fault.Source == "gateway"`)

			got, evalErr = evaluator.EvaluateFaultCondition(`fault.Source != "backend"`, faultCtx)
			require.NoError(t, evalErr)
			assert.Equal(t, tc.notFromBack, got,
				`fault.Source != "backend" — the form that keeps an old router's "unknown" in`)
		})
	}
}

// A condition can reach the whole description, not just the source. Each of these is a real
// narrowing someone asks for: notify only on auth failures, only on this guardrail, only when
// a specific policy is the one that rejected.
func TestFaultCondition_ReadsTheDescription(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	faultCtx := &policy.FaultContext{
		SharedContext:  &policy.SharedContext{RequestID: "r-1"},
		RequestHeaders: policy.NewHeaders(nil),
		ResponseStatus: 422,
		OriginalStatus: 200,
		Source:         policy.FaultSourceGateway,
		Policy:         "word-count-guardrail",
		PolicyVersion:  "v1.0.0",
		PolicyPhase:    policy.PolicyPhaseResponseBody,
		RouteKey:       "route-a",
		Fault: &policy.FaultDetails{
			Code:      "906000",
			Type:      policy.FaultTypeGuardrail,
			Direction: policy.DirectionResponse,
			Message:   "Request blocked by a guardrail",
			Guardrail: &policy.GuardrailDetails{
				InterveningGuardrail: "word-count-guardrail",
				Action:               policy.GuardrailActionIntervened,
				ActionReason:         "too many words",
			},
		},
	}

	for _, expr := range []string{
		`fault.Code == "906000"`,
		`fault.Type == "guardrail"`,
		`fault.Direction == "Response"`,
		`fault.Message.startsWith("Request blocked")`,
		`fault.Policy == "word-count-guardrail"`,
		`fault.PolicyVersion == "v1.0.0"`,
		`fault.PolicyPhase == "response_body"`,
		`fault.RouteKey == "route-a"`,
		`fault.Status == 422`,
		`fault.OriginalStatus == 200`,
		`fault.ResponseCommitted == false`,
		// The composite form an operator actually writes.
		`fault.Type == "guardrail" && fault.Direction == "Response"`,
	} {
		t.Run(expr, func(t *testing.T) {
			got, evalErr := evaluator.EvaluateFaultCondition(expr, faultCtx)
			require.NoError(t, evalErr, "expression must evaluate")
			assert.True(t, got, "expression must match the failure it describes")
		})
	}
}

// The trap this design exists to avoid.
//
// A variable declared in the environment but missing from the activation is an EVALUATION
// error, not a false — so a condition mentioning error.* on a NON-fault policy would fail at
// request time rather than simply not matching, and the error would surface nowhere near the
// condition that caused it. Every activation supplies the keys with zero values instead.
func TestFaultVariablesEvaluateOnEveryPhase(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	const expr = `fault.Type == "guardrail" || fault.Source == "backend"`

	t.Run("request headers", func(t *testing.T) {
		got, evalErr := evaluator.EvaluateRequestHeaderCondition(expr, &policy.RequestHeaderContext{
			SharedContext: &policy.SharedContext{}, Headers: policy.NewHeaders(nil),
		})
		require.NoError(t, evalErr, "must not be an evaluation error")
		assert.False(t, got, "no failure has been described in this phase")
	})

	t.Run("request body", func(t *testing.T) {
		got, evalErr := evaluator.EvaluateRequestBodyCondition(expr, &policy.RequestContext{
			SharedContext: &policy.SharedContext{}, Headers: policy.NewHeaders(nil),
		})
		require.NoError(t, evalErr)
		assert.False(t, got)
	})

	t.Run("response headers", func(t *testing.T) {
		got, evalErr := evaluator.EvaluateResponseHeaderCondition(expr, &policy.ResponseHeaderContext{
			SharedContext:  &policy.SharedContext{},
			RequestHeaders: policy.NewHeaders(nil), ResponseHeaders: policy.NewHeaders(nil),
		})
		require.NoError(t, evalErr)
		assert.False(t, got)
	})

	t.Run("response body", func(t *testing.T) {
		got, evalErr := evaluator.EvaluateResponseBodyCondition(expr, &policy.ResponseContext{
			SharedContext:  &policy.SharedContext{},
			RequestHeaders: policy.NewHeaders(nil), ResponseHeaders: policy.NewHeaders(nil),
		})
		require.NoError(t, evalErr)
		assert.False(t, got)
	})

	// And a fault entry whose failure nothing described: Error is nil, which must read as
	// empty fields rather than blowing up the condition.
	t.Run("undescribed failure", func(t *testing.T) {
		got, evalErr := evaluator.EvaluateFaultCondition(expr, errorCtxWithSource(policy.FaultSourceRouter))
		require.NoError(t, evalErr)
		assert.False(t, got)
	})
}

// fault.Guardrail is not a condition variable: which guardrail acted is fault.Policy. A
// condition still naming it must be refused, not silently read as empty.
func TestFaultCondition_GuardrailIsNotAVariable(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	_, evalErr := evaluator.EvaluateFaultCondition(
		`fault.Guardrail.InterveningGuardrail == "url-guardrail"`,
		errorCtxWithSource(policy.FaultSourceGateway))

	assert.Error(t, evalErr)
}

// Both spellings resolve. The dotted variables are what conditions are written with; the
// `error` map is what makes has()/indexing work, and the two must not disagree.
func TestFaultVariables_MapAndDottedFormsAgree(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	faultCtx := errorCtxWithSource(policy.FaultSourceRouter)

	dotted, err1 := evaluator.EvaluateFaultCondition(`fault.Source == "router"`, faultCtx)
	indexed, err2 := evaluator.EvaluateFaultCondition(`fault["Source"] == "router"`, faultCtx)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.True(t, dotted)
	assert.Equal(t, dotted, indexed, "fault.Source and fault[\"Source\"] must be the same value")
}

func errorCtxWithSource(source string) *policy.FaultContext {
	return &policy.FaultContext{
		SharedContext:  &policy.SharedContext{RequestID: "r-1"},
		RequestHeaders: policy.NewHeaders(nil),
		ResponseStatus: 503,
		Source:         source,
	}
}

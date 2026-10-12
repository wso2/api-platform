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

package policyv1alpha2

import "testing"

// These strings are on the wire: Type is rendered into the error body for every shape, so
// renaming one breaks any client keying off it. Spelled out literally here rather than
// compared to the constants themselves — a test that reads `FaultTypeGuardrail` on both
// sides would pass through a rename and prove nothing.
//
// The Python SDK repeats these in apip_sdk_core.policy.v1alpha2.FaultType, and
// sdk-python/tests/test_public_api.py holds the matching assertion. A value changed on one
// side and not the other means the same failure is reported under two names depending on
// which language the policy is written in.
func TestErrorTypeWireValues(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{FaultTypeAuthentication, "authentication"},
		{FaultTypeAuthorization, "authorization"},
		{FaultTypeThrottling, "throttling"},
		{FaultTypeGuardrail, "guardrail"},
		{FaultTypeValidation, "validation"},
		{FaultTypeMediation, "mediation"},
		{FaultTypeConfiguration, "configuration"},
		{FaultTypeUpstream, "upstream"},
		{FaultTypeRouting, "routing"},
		{FaultTypeInternal, "internal"},
		{FaultTypeRequestSize, "requestSize"},
	} {
		if tc.got != tc.want {
			t.Errorf("error type value changed: got %q, want %q", tc.got, tc.want)
		}
	}
}

// Two classes sharing a value would silently merge, and a caller branching on Type could not
// tell them apart.
func TestErrorTypeValuesAreDistinct(t *testing.T) {
	all := []string{
		FaultTypeAuthentication, FaultTypeAuthorization, FaultTypeThrottling,
		FaultTypeGuardrail, FaultTypeValidation, FaultTypeMediation,
		FaultTypeConfiguration, FaultTypeUpstream, FaultTypeRouting,
		FaultTypeInternal, FaultTypeRequestSize,
	}
	seen := make(map[string]bool, len(all))
	for _, v := range all {
		if v == "" {
			t.Error("an error type must not be empty: empty means \"not classified\"")
		}
		if seen[v] {
			t.Errorf("duplicate error type value %q", v)
		}
		seen[v] = true
	}
}

// The constants are untyped strings, matching Direction, so they assign straight into the
// plain-string field without a conversion. This is what lets a policy stop declaring its own.
func TestFaultTypeAssignsToFaultDetails(t *testing.T) {
	e := FaultDetails{
		Code:      "900902",
		Type:      FaultTypeAuthentication,
		Direction: DirectionRequest,
		Message:   "Valid credentials required",
	}
	if e.Type != "authentication" {
		t.Errorf("Type = %q, want %q", e.Type, "authentication")
	}
}

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
	"strconv"
	"testing"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// The code_details strings here are the exact values a running gateway produced, not
// invented fixtures: a backend 500, an unreachable upstream on a closed port, and a
// request matching no API.
const (
	observedViaUpstream   = "via_upstream"
	observedConnRefused   = "upstream_reset_before_response_started{remote_connection_failure|delayed_connect_error:_Connection_refused}"
	observedNoHealthyHost = "no_healthy_upstream"
	observedDirectResp    = "direct_response"
)

func TestClassifyErrorSource(t *testing.T) {
	cases := []struct {
		name        string
		origin      faultOrigin
		codeDetails string
		want        faultSource
	}{
		// A policy-produced failure is attributable without consulting Envoy: the engine is
		// what built the response. code_details describes the proxy attempt and is
		// deliberately ignored, including when it says the backend answered — for a
		// status-override rejection the backend DID answer and a policy then overrode the
		// status, and the fault belongs to the policy.
		{"policy origin is always gateway", originGateway, "", sourceGateway},
		{"policy origin ignores provenance", originGateway, observedViaUpstream, sourceGateway},
		{"a status-override rejection is the gateway's even though the backend answered",
			originGateway, observedViaUpstream, sourceGateway},

		// Pass-through: Envoy's account decides.
		{"backend answered", originUpstream, observedViaUpstream, sourceBackend},
		{"connection refused is router", originUpstream, observedConnRefused, sourceRouter},
		{"no healthy host is router", originUpstream, observedNoHealthyHost, sourceRouter},
		{"direct response is noRoute", originUpstream, observedDirectResp, sourceNoRoute},

		// An unrecognised detail string must classify as router, not unknown: Envoy
		// adds new reasons across versions and every one of them means "Envoy produced
		// this", so defaulting to router keeps a version bump from silently
		// reclassifying infrastructure failures as unattributable.
		{"unrecognised detail defaults to router", originUpstream, "some_future_envoy_reason", sourceRouter},

		// Absent attributes are unknown, never a guess.
		{"missing attribute is unknown", originUpstream, "", sourceUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyFaultSource(tc.origin, tc.codeDetails); got != tc.want {
				t.Fatalf("classifyFaultSource(%q, %q) = %q, want %q",
					tc.origin, tc.codeDetails, got, tc.want)
			}
		})
	}
}

// Provenance no longer gates the flow, so there is nothing here to assert about which
// sources are handled — every one is. What replaced the gate is a value a condition can read,
// and the property worth pinning is that each source has a DISTINCT, stable spelling: an
// operator writing `fault.Source == "gateway"` to keep a notifier off their backend's own
// 500s is relying on these exact strings, and two sources collapsing to one spelling would
// silently widen or narrow what their condition matches.
func TestErrorSourceSpellingsAreDistinctAndStable(t *testing.T) {
	want := map[faultSource]string{
		sourceGateway: "gateway",
		sourceBackend: "backend",
		sourceRouter:  "router",
		sourceNoRoute: "noRoute",
		sourceUnknown: "unknown",
	}

	seen := make(map[string]faultSource, len(want))
	for src, spelling := range want {
		if string(src) != spelling {
			t.Errorf("faultSource %q must spell as %q — it is a wire value read by execution conditions",
				string(src), spelling)
		}
		if prev, dup := seen[spelling]; dup {
			t.Errorf("sources %q and %q share the spelling %q; a condition cannot tell them apart",
				prev, src, spelling)
		}
		seen[spelling] = src
	}
	if len(seen) != 5 {
		t.Fatalf("expected 5 distinct sources, got %d", len(seen))
	}
}

// The codes the gateway puts in a client-facing error body, pinned on three properties:
//
//   - each reused value equals its analytics constant, so an error body and the analytics
//     event for the same request cannot disagree. The engine's constants alias the SDK's and
//     internal/analytics holds the integer form, so a divergence between the two fails here —
//     this package is the only one that can see both.
//   - every UPSTREAM code sits inside the target-failure range, or it is silently
//     recategorised as "other".
//   - every code is a six-digit numeric string.
func TestGatewayFaultCodes(t *testing.T) {
	// The expected values are the established ones, asserted from literals here rather than read
	// from a mirror of them in the source. Two of these (101503/101504) still have a mirror
	// in the analytics package because they predate the SDK; the other two do not, and
	// re-adding production constants just to give this loop something to compare against
	// would be the duplication this test exists to catch.
	reused := []struct {
		name, code, externalName string
		expected                 int
	}{
		{"upstream unreachable", codeUpstreamUnreachable, "NHTTP_CONNECTION_FAILED", 101503},
		{"upstream timeout", codeUpstreamTimeout, "NHTTP_CONNECTION_TIMEOUT", 101504},
		{"no healthy upstream", codeUpstreamUnavailable, "ENDPOINT_SUSPENDED_ERROR_CODE", 303001},
		{"no route", codeNoRoute, "RESOURCE_NOT_FOUND_APIM_ERROR_CODE", 900906},
	}
	for _, tc := range reused {
		if want := strconv.Itoa(tc.expected); tc.code != want {
			t.Errorf("%s: code %q has drifted from %s (%q)", tc.name, tc.code, tc.externalName, want)
		}
	}
	// The two that DO still have a mirror are checked against it as well, so the SDK and the
	// analytics package cannot disagree while both exist.
	if strconv.Itoa(analytics.NhttpConnectionFailed) != codeUpstreamUnreachable {
		t.Errorf("analytics.NhttpConnectionFailed (%d) and codeUpstreamUnreachable (%s) disagree",
			analytics.NhttpConnectionFailed, codeUpstreamUnreachable)
	}
	if strconv.Itoa(analytics.NhttpConnectionTimeout) != codeUpstreamTimeout {
		t.Errorf("analytics.NhttpConnectionTimeout (%d) and codeUpstreamTimeout (%s) disagree",
			analytics.NhttpConnectionTimeout, codeUpstreamTimeout)
	}

	// An upstream failure must be classifiable as one. Either it sits in the target-failure
	// range, or it is the endpoint-suspended code, which the classifier handles
	// with an explicit case rather than by range.
	upstream := map[string]string{
		"unreachable":         codeUpstreamUnreachable,
		"timeout":             codeUpstreamTimeout,
		"generic":             codeUpstreamGeneric,
		"no healthy upstream": codeUpstreamUnavailable,
	}
	// The endpoint-suspended code, the one upstream code the classifier handles
	// by an explicit case instead of by range.
	const suspended = "303001"
	for name, code := range upstream {
		n, err := strconv.Atoi(code)
		if err != nil {
			t.Errorf("%s: code %q is not numeric", name, code)
			continue
		}
		inRange := n >= policy.TargetFailureRangeStart && n < policy.TargetFailureRangeEnd
		if !inRange && code != suspended {
			t.Errorf("%s: code %d is outside the target-failure range [%d,%d) and is not the "+
				"endpoint-suspended code, so it would be categorised as 'other'",
				name, n, policy.TargetFailureRangeStart, policy.TargetFailureRangeEnd)
		}
	}

	// Codes with no reserved equivalent stay above the 904015 ceiling, measured
	// against release 9.33.172, so they cannot collide with a control-plane code.
	const reservedCeiling = 904015
	native := map[string]string{
		"engine internal":   codeEngineInternal,
		"payload too large": codePayloadTooLarge,
	}
	for name, code := range native {
		n, err := strconv.Atoi(code)
		if err != nil {
			t.Errorf("%s: code %q is not numeric", name, code)
			continue
		}
		if n <= reservedCeiling {
			t.Errorf("%s: code %d is inside the reserved range (ceiling %d) and may collide",
				name, n, reservedCeiling)
		}
	}

	// The block reserved for user-defined codes must stay reserved. It has to sit outside every
	// classification range — a code in it is meant to classify as "other", and a range overlap
	// would silently relabel a customer's code as an auth or throttling failure — and no code
	// the gateway itself emits may fall inside it, or a product code would collide with one a
	// deployment allocated for its own condition.
	ranges := []struct {
		name       string
		start, end int
	}{
		{"authentication", policy.AuthFailureRangeStart, policy.AuthFailureRangeEnd},
		{"throttling", policy.ThrottledFailureRangeStart, policy.ThrottledFailureRangeEnd},
		{"target", policy.TargetFailureRangeStart, policy.TargetFailureRangeEnd},
	}
	for _, r := range ranges {
		if policy.ShippedPolicyRangeStart < r.end && r.start < policy.UserDefinedRangeEnd {
			t.Errorf("user-defined range [%d,%d) overlaps the %s range [%d,%d), so a "+
				"user-defined code would be categorised as a %s failure",
				policy.ShippedPolicyRangeStart, policy.UserDefinedRangeEnd,
				r.name, r.start, r.end, r.name)
		}
	}

	all := map[string]string{"engine internal": codeEngineInternal, "payload too large": codePayloadTooLarge}
	for k, v := range upstream {
		all[k] = v
	}
	all["no route"] = codeNoRoute
	for name, code := range all {
		if len(code) != 6 {
			t.Errorf("%s: code %q is %d digits, want 6", name, code, len(code))
		}
		n, err := strconv.Atoi(code)
		if err != nil {
			t.Errorf("%s: code %q is not numeric", name, code)
			continue
		}
		if n >= policy.ShippedPolicyRangeStart && n < policy.UserDefinedRangeEnd {
			t.Errorf("%s: code %d is inside the range reserved for user-defined codes [%d,%d)",
				name, n, policy.ShippedPolicyRangeStart, policy.UserDefinedRangeEnd)
		}
	}
}

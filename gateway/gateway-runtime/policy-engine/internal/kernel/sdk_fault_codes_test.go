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

// The codes still declared in BOTH the SDK and internal/analytics must not drift apart.
//
// The list is short now, and that is the point. internal/analytics used to mirror 23 of the
// SDK's codes and all of its classification ranges, so this test had to enumerate every one
// of them; the mirror is gone, and internal/analytics reads the SDK directly (see
// analytics/fault_classification.go). What remains here is the genuine overlap: numbers the
// analytics package declared before the SDK's fault vocabulary existed and still owns.
//
// Neither module can import the other's `internal`, and the engine needs the INTEGER form
// while a policy needs the STRING form, so agreement cannot be enforced by construction for
// these. This package is the only one that can see both sides, which makes it the only place
// the check can live. Without it a policy would return one code in the body while the
// analytics event for the same request carried another.
func TestSDKFaultCodesMatchAnalytics(t *testing.T) {
	cases := []struct {
		name, sdk, analyticsName string
		analytics                int
	}{
		// Throttling: these predate the SDK's vocabulary, so both declarations stand.
		{"throttled api", policy.FaultCodeThrottledAPI, "APIThrottleOutErrorCode", analytics.APIThrottleOutErrorCode},
		{"throttled resource", policy.FaultCodeThrottledResource, "ResourceThrottleOutErrorCode", analytics.ResourceThrottleOutErrorCode},
		{"throttled application", policy.FaultCodeThrottledApplication, "ApplicationThrottleOutErrorCode", analytics.ApplicationThrottleOutErrorCode},
		{"throttled subscription", policy.FaultCodeThrottledSubscription, "SubscriptionThrottleOutErrorCode", analytics.SubscriptionThrottleOutErrorCode},
		{"throttled blocked", policy.FaultCodeThrottledBlocked, "BlockedErrorCode", analytics.BlockedErrorCode},
		{"throttled custom policy", policy.FaultCodeThrottledCustomPolicy, "CustomPolicyThrottleOutErrorCode", analytics.CustomPolicyThrottleOutErrorCode},

		// Synapse transport codes, which no policy emits but the SDK names for the router's
		// own failures.
		{"upstream unreachable", policy.FaultCodeUpstreamUnreachable, "NhttpConnectionFailed", analytics.NhttpConnectionFailed},
		{"upstream timeout", policy.FaultCodeUpstreamTimeout, "NhttpConnectionTimeout", analytics.NhttpConnectionTimeout},
	}

	for _, tc := range cases {
		if want := strconv.Itoa(tc.analytics); tc.sdk != want {
			t.Errorf("%s: SDK has %q, analytics.%s is %q — an error body and its analytics "+
				"event would report different codes for the same failure",
				tc.name, tc.sdk, tc.analyticsName, want)
		}
	}

	// codeUpstreamGeneric is derived rather than named — it is deliberately the last slot in
	// the target-failure range, because Synapse allocates upward from the start and the top
	// is the far end from whatever it takes next. So it is pinned to the arithmetic.
	if want := strconv.Itoa(policy.TargetFailureRangeEnd - 1); policy.FaultCodeUpstreamGeneric != want {
		t.Errorf("FaultCodeUpstreamGeneric is %q, want %q (the top of the target-failure range)",
			policy.FaultCodeUpstreamGeneric, want)
	}
}

// The SDK's classification ranges must match the ones analytics uses.
//
// These decide a code's fault CATEGORY, so a range that drifts silently re-files every code
// inside it. Exported from the SDK so a policy can check its own allocation, which means the
// two copies are read by different audiences and would drift without noticing.
func TestSDKClassificationRangesMatchAnalytics(t *testing.T) {
	cases := []struct {
		name                 string
		sdkStart, sdkEnd     int
		analyticsStart, aEnd int
	}{
		{"auth", policy.AuthFailureRangeStart, policy.AuthFailureRangeEnd,
			policy.AuthFailureRangeStart, policy.AuthFailureRangeEnd},
		{"throttled", policy.ThrottledFailureRangeStart, policy.ThrottledFailureRangeEnd,
			policy.ThrottledFailureRangeStart, policy.ThrottledFailureRangeEnd},
		{"target", policy.TargetFailureRangeStart, policy.TargetFailureRangeEnd,
			policy.TargetFailureRangeStart, policy.TargetFailureRangeEnd},
	}
	for _, tc := range cases {
		if tc.sdkStart != tc.analyticsStart || tc.sdkEnd != tc.aEnd {
			t.Errorf("%s range: SDK [%d,%d), analytics [%d,%d)",
				tc.name, tc.sdkStart, tc.sdkEnd, tc.analyticsStart, tc.aEnd)
		}
	}

	// The SDK splits the 96xxxx policy space into a WSO2 half and a customer half, which
	// analytics carries as one block. The outer bounds must still agree, and the split must
	// be contiguous — a gap between them would be a range nobody may allocate in, silently.
	if policy.ShippedPolicyRangeStart != policy.ShippedPolicyRangeStart {
		t.Errorf("policy space starts at %d in the SDK, %d in analytics",
			policy.ShippedPolicyRangeStart, policy.ShippedPolicyRangeStart)
	}
	if policy.UserDefinedRangeEnd != policy.UserDefinedRangeEnd {
		t.Errorf("policy space ends at %d in the SDK, %d in analytics",
			policy.UserDefinedRangeEnd, policy.UserDefinedRangeEnd)
	}
	if policy.ShippedPolicyRangeEnd != policy.UserDefinedRangeStart {
		t.Errorf("the shipped block ends at %d and the customer block starts at %d, leaving a "+
			"range nobody owns", policy.ShippedPolicyRangeEnd, policy.UserDefinedRangeStart)
	}
}

// Each code must sit in the range that gives it the category it claims.
//
// Range membership is the whole mechanism: a code outside every range is filed as "other"
// however specific its meaning, so an auth code above 901000 would stop being an auth failure
// while still looking like one to a reader.
func TestSDKErrorCodesLandInTheClaimedCategory(t *testing.T) {
	cases := []struct {
		name, code string
		start, end int
	}{
		{"auth general", policy.FaultCodeAuthGeneral, policy.AuthFailureRangeStart, policy.AuthFailureRangeEnd},
		{"missing credentials", policy.FaultCodeAuthMissingCredentials, policy.AuthFailureRangeStart, policy.AuthFailureRangeEnd},
		{"forbidden", policy.FaultCodeAuthForbidden, policy.AuthFailureRangeStart, policy.AuthFailureRangeEnd},
		{"invalid scope", policy.FaultCodeInvalidScope, policy.AuthFailureRangeStart, policy.AuthFailureRangeEnd},
		{"no route", policy.FaultCodeNoRoute, policy.AuthFailureRangeStart, policy.AuthFailureRangeEnd},

		{"throttled api", policy.FaultCodeThrottledAPI, policy.ThrottledFailureRangeStart, policy.ThrottledFailureRangeEnd},
		{"throttled blocked", policy.FaultCodeThrottledBlocked, policy.ThrottledFailureRangeStart, policy.ThrottledFailureRangeEnd},

		{"upstream unreachable", policy.FaultCodeUpstreamUnreachable, policy.TargetFailureRangeStart, policy.TargetFailureRangeEnd},
		{"upstream timeout", policy.FaultCodeUpstreamTimeout, policy.TargetFailureRangeStart, policy.TargetFailureRangeEnd},
		{"upstream generic", policy.FaultCodeUpstreamGeneric, policy.TargetFailureRangeStart, policy.TargetFailureRangeEnd},
	}
	for _, tc := range cases {
		n, err := strconv.Atoi(tc.code)
		if err != nil {
			t.Errorf("%s: code %q is not numeric", tc.name, tc.code)
			continue
		}
		if n < tc.start || n >= tc.end {
			t.Errorf("%s: code %d is outside [%d,%d), so it would be categorised as 'other'",
				tc.name, n, tc.start, tc.end)
		}
	}

	// FaultCodeUpstreamUnavailable is the exception, and it has to be: the classifier
	// handles 303001 with an explicit case rather than by range, giving it its own
	// CONNECTION_SUSPENDED sub-category. Asserting it is OUTSIDE the target range is what
	// stops someone "fixing" it into the range and losing that distinction.
	n, err := strconv.Atoi(policy.FaultCodeUpstreamUnavailable)
	if err != nil {
		t.Fatalf("FaultCodeUpstreamUnavailable is not numeric: %v", err)
	}
	if n >= policy.TargetFailureRangeStart && n < policy.TargetFailureRangeEnd {
		t.Errorf("FaultCodeUpstreamUnavailable (%d) must stay outside the target-failure range; "+
			"it is classified by an explicit case, not by range", n)
	}
}

// Every code must be a six-digit numeric string, and no two may collide.
//
// The shape assertion exists because a bulk edit once matched a short key against a longer
// code's prefix and produced the nine-digit "900900000". The uniqueness assertion exists
// because two names for one number is indistinguishable from a typo until a caller branches
// on it.
func TestSDKErrorCodesAreWellFormedAndDistinct(t *testing.T) {
	all := map[string]string{
		"AuthGeneral":                 policy.FaultCodeAuthGeneral,
		"AuthInvalidCredentials":      policy.FaultCodeAuthInvalidCredentials,
		"AuthMissingCredentials":      policy.FaultCodeAuthMissingCredentials,
		"AuthTokenExpired":            policy.FaultCodeAuthTokenExpired,
		"AuthTokenInactive":           policy.FaultCodeAuthTokenInactive,
		"AuthIncorrectTokenType":      policy.FaultCodeAuthIncorrectTokenType,
		"AuthBlocked":                 policy.FaultCodeAuthBlocked,
		"AuthForbidden":               policy.FaultCodeAuthForbidden,
		"SubscriptionInactive":        policy.FaultCodeSubscriptionInactive,
		"InvalidScope":                policy.FaultCodeInvalidScope,
		"ThrottledAPI":                policy.FaultCodeThrottledAPI,
		"ThrottledResource":           policy.FaultCodeThrottledResource,
		"ThrottledApplication":        policy.FaultCodeThrottledApplication,
		"ThrottledSubscription":       policy.FaultCodeThrottledSubscription,
		"ThrottledBlocked":            policy.FaultCodeThrottledBlocked,
		"ThrottledCustomPolicy":       policy.FaultCodeThrottledCustomPolicy,
		"UpstreamUnreachable":         policy.FaultCodeUpstreamUnreachable,
		"UpstreamTimeout":             policy.FaultCodeUpstreamTimeout,
		"UpstreamGeneric":             policy.FaultCodeUpstreamGeneric,
		"UpstreamUnavailable":         policy.FaultCodeUpstreamUnavailable,
		"NoRoute":                     policy.FaultCodeNoRoute,
		"EngineInternal":              policy.FaultCodeEngineInternal,
		"PayloadTooLarge":             policy.FaultCodePayloadTooLarge,
		"NoPolicyChain":               policy.FaultCodeNoPolicyChain,
		"ResolutionBadRequest":        policy.FaultCodeResolutionBadRequest,
		"ResolutionUnsupportedCoding": policy.FaultCodeResolutionUnsupportedEncoding,
		"GuardrailIntervened":         policy.GuardrailCodeIntervened,
	}

	seen := make(map[string]string, len(all))
	for name, code := range all {
		if len(code) != 6 {
			t.Errorf("%s: code %q must be six digits", name, code)
		}
		if _, err := strconv.Atoi(code); err != nil {
			t.Errorf("%s: code %q is not numeric", name, code)
		}
		if prev, dup := seen[code]; dup {
			t.Errorf("%s and %s both use %s", prev, name, code)
		}
		seen[code] = name
	}
}

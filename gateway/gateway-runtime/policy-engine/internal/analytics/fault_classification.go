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

package analytics

import (
	"fmt"
	"strconv"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Classifying a failure for analytics.
//
// Analytics is built from Envoy's access log, which carries a status and code_details and
// knows nothing about which policy rejected the request or why. The collector's OnFault runs
// as the last entry of every API's fault chain and stamps the resolved failure into analytics
// metadata, which arrives here as the PropKeyFault* keys.
//
// Categories come from dto.FaultCategory and are assigned by RANGE (start <= code < end),
// not by reading a label, so a dashboard already keyed on the existing categories keeps
// working. A code outside every range is reported as OTHER.

// Codes are strings in the SDK, because a string is what a policy puts in
// FaultDetails.Code and what a client parses out of an error body. Range comparison needs
// them as integers, so they are parsed exactly once, here.
//
// Parsed rather than re-declared as integer literals: a second copy of a number can only
// agree with the first by being tested into agreement, and this package had 23 such copies
// before this block replaced them. The SDK is where a policy author looks, so the SDK is the
// source.
var (
	codeAuthGeneral          = mustFaultCode(policy.FaultCodeAuthGeneral)
	codeAuthInvalidCreds     = mustFaultCode(policy.FaultCodeAuthInvalidCredentials)
	codeAuthMissingCreds     = mustFaultCode(policy.FaultCodeAuthMissingCredentials)
	codeAuthTokenExpired     = mustFaultCode(policy.FaultCodeAuthTokenExpired)
	codeAuthTokenInactive    = mustFaultCode(policy.FaultCodeAuthTokenInactive)
	codeAuthWrongTokenType   = mustFaultCode(policy.FaultCodeAuthIncorrectTokenType)
	codeAuthBlocked          = mustFaultCode(policy.FaultCodeAuthBlocked)
	codeAuthForbidden        = mustFaultCode(policy.FaultCodeAuthForbidden)
	codeSubscriptionInactive = mustFaultCode(policy.FaultCodeSubscriptionInactive)
	codeInvalidScope         = mustFaultCode(policy.FaultCodeInvalidScope)
	codeEndpointSuspended    = mustFaultCode(policy.FaultCodeUpstreamUnavailable)
	codeNoRoute              = mustFaultCode(policy.FaultCodeNoRoute)
)

// mustFaultCode parses an SDK fault code, panicking on a non-numeric one.
//
// A panic at package init rather than an error at classification time, because the only way
// to reach it is to edit an SDK constant to something that is not a six-digit number — a
// build-time mistake. Returning zero instead would silently misclassify every event that
// code appears on, which is far harder to notice than a failed start.
func mustFaultCode(code string) int {
	n, err := strconv.Atoi(code)
	if err != nil {
		panic(fmt.Sprintf("analytics: SDK fault code %q is not numeric: %v", code, err))
	}
	return n
}

// faultCategoryForCode maps a fault code onto the fault category.
//
// Half-open ranges (start <= code < end) matching the classifier's comparison, with the bounds
// read from the SDK. A code in no range is OTHER — including every 906xxx guardrail sub-code
// and every 96xxxx policy code, both of which sit outside the ranges deliberately (see
// docs/gateway/error-codes.md).
func faultCategoryForCode(code int) dto.FaultCategory {
	switch {
	case code >= policy.AuthFailureRangeStart && code < policy.AuthFailureRangeEnd:
		return dto.FaultCategoryAuth
	case code >= policy.ThrottledFailureRangeStart && code < policy.ThrottledFailureRangeEnd:
		return dto.FaultCategoryThrottled
	case code >= policy.TargetFailureRangeStart && code < policy.TargetFailureRangeEnd:
		return dto.FaultCategoryTargetConnectivity
	case code == codeEndpointSuspended:
		// 303001 is outside the target range but IS a target failure — the classifier treats
		// endpoint suspension as connectivity, and it is the one code that has to be
		// named rather than ranged.
		return dto.FaultCategoryTargetConnectivity
	default:
		return dto.FaultCategoryOther
	}
}

// faultSubCategoryForCode maps a code onto the specific subcategory within its category.
//
// Every value returned is an existing dto.FaultSubCategory constant; none is invented here.
// A code with no specific subcategory falls back to its category's "other" member rather
// than to UNCLASSIFIED, so a throttling code nobody enumerated still reports as throttling.
//
// The throttling and NHTTP cases read this package's own constants rather than the SDK's:
// those numbers predate the SDK's fault vocabulary or are Synapse transport codes no policy
// emits, so the SDK does not own them. Everything the SDK does own comes from the SDK.
func faultSubCategoryForCode(code int) dto.FaultSubCategory {
	switch code {
	// Authentication and authorization — SDK-owned.
	case codeAuthForbidden, codeInvalidScope:
		return dto.AuthenticationAuthorizationFailure
	case codeSubscriptionInactive:
		return dto.AuthenticationSubscriptionValidationFailure
	case codeAuthGeneral, codeAuthInvalidCreds, codeAuthMissingCreds, codeAuthTokenExpired,
		codeAuthTokenInactive, codeAuthWrongTokenType, codeAuthBlocked:
		return dto.AuthenticationFailure

	// Throttling — this package's own, predating the SDK vocabulary.
	case APIThrottleOutErrorCode:
		return dto.ThrottlingAPILimitExceeded
	case HardLimitExceededErrorCode:
		return dto.ThrottlingHardLimitExceeded
	case ResourceThrottleOutErrorCode:
		return dto.ThrottlingResourceLimitExceeded
	case ApplicationThrottleOutErrorCode:
		return dto.ThrottlingApplicationLimitExceeded
	case SubscriptionThrottleOutErrorCode:
		return dto.ThrottlingSubscriptionLimitExceeded
	case BlockedErrorCode:
		return dto.ThrottlingBlocked
	case CustomPolicyThrottleOutErrorCode:
		return dto.ThrottlingCustomPolicyLimitExceeded

	// Target connectivity. The timeout is a Synapse transport code; suspension is SDK-owned.
	case NhttpConnectionTimeout:
		return dto.TargetConnectivityConnectionTimeout
	case codeEndpointSuspended:
		return dto.TargetConnectivityConnectionSuspended

	// Other, where a specific member exists.
	case codeNoRoute:
		return dto.OtherResourceNotFound
	}

	// No specific member: answer with the category's own catch-all, so the subcategory
	// never contradicts the category beside it.
	switch faultCategoryForCode(code) {
	case dto.FaultCategoryAuth:
		return dto.AuthenticationOther
	case dto.FaultCategoryThrottled:
		return dto.ThrottlingOther
	case dto.FaultCategoryTargetConnectivity:
		return dto.TargetConnectivityOther
	default:
		return dto.OtherUnclassified
	}
}

// applyFaultDetails populates the event's error object from the metadata the collector's
// OnFault stamped.
//
// The PropKeyFault* keys are TRANSPORT between two Go modules: the collector reaches the
// engine over Envoy's dynamic metadata, which is a flat map. They are turned into the typed
// dto.Error here and deliberately NOT copied into Event.Properties.
//
// Absent metadata means the request did not go through the fault flow, so the event is left
// as it was — a successful request must not acquire an empty error object.
func applyFaultDetails(
	event *dto.Event,
	keyValuePairs map[string]string,
	typedValuePairs map[string]interface{},
) {
	str := func(key string) string { return keyValuePairs[key] }

	codeStr := str(dto.PropKeyFaultCode)
	faultType := str(dto.PropKeyFaultType)

	// Nothing the collector stamped at all: not a fault path. Leave the event untouched.
	if codeStr == "" && faultType == "" && str(dto.PropKeyFaultSource) == "" &&
		str(dto.PropKeyFaultPolicy) == "" {
		return
	}

	faultErr := &dto.Error{
		Type:        faultType,
		Direction:   str(dto.PropKeyFaultDirection),
		Summary:     str(dto.PropKeyFaultMessage),
		Policy:      str(dto.PropKeyFaultPolicy),
		PolicyPhase: str(dto.PropKeyFaultPolicyPhase),
		Source:      str(dto.PropKeyFaultSource),
	}

	// Numbers arrive through structpb as float64, so they are read from the typed map
	// rather than parsed back out of the stringified one.
	if v, ok := typedValuePairs[dto.PropKeyFaultOriginalStatus].(float64); ok {
		faultErr.OriginalStatus = int(v)
	}
	if v, ok := typedValuePairs[dto.PropKeyFaultJSONRPCCode].(float64); ok {
		faultErr.JSONRPCCode = int(v)
	}

	if name := str(dto.PropKeyFaultGuardrail); name != "" ||
		str(dto.PropKeyFaultGuardrailAction) != "" || str(dto.PropKeyFaultGuardrailReason) != "" {
		faultErr.Guardrail = &dto.ErrorGuardrail{
			Name:   name,
			Action: str(dto.PropKeyFaultGuardrailAction),
			Reason: str(dto.PropKeyFaultGuardrailReason),
		}
	}

	// Classification needs the code as an integer. A failure the gateway described without
	// classifying — a router error carrying no code — still reports everything above;
	// it simply has no category to claim, and OTHER/UNCLASSIFIED is the honest answer.
	switch code, err := strconv.Atoi(codeStr); {
	case codeStr == "":
		event.ErrorType = string(dto.FaultCategoryOther)
		faultErr.ErrorMessage = dto.OtherUnclassified
	case err != nil:
		// A non-numeric code is a policy bug. Keep it visible rather than swallowing it:
		// Summary is client-facing text, so the malformed code goes where a reader of the
		// error will actually see it.
		event.ErrorType = string(dto.FaultCategoryOther)
		faultErr.ErrorMessage = dto.OtherUnclassified
		if faultErr.Summary == "" {
			faultErr.Summary = "unclassified fault code " + codeStr
		}
	default:
		event.ErrorType = string(faultCategoryForCode(code))
		faultErr.ErrorCode = code
		faultErr.ErrorMessage = faultSubCategoryForCode(code)
	}

	event.Error = faultErr
}

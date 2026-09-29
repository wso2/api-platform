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
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// sdkCode parses an SDK fault code for a test expectation.
//
// Deliberately its own parse rather than the production mustFaultCode: a test that shared
// the helper it is checking would stay self-consistent even if that helper were broken.
func sdkCode(t *testing.T, code string) int {
	t.Helper()
	n, err := strconv.Atoi(code)
	require.NoError(t, err, "SDK fault code %q must be numeric", code)
	return n
}

// Category is assigned by RANGE, the way the classifier does it, so the
// boundaries are the contract and not an implementation detail. A half-open comparison off
// by one at either end silently refiles events under the wrong category.
func TestFaultCategoryForCode_Boundaries(t *testing.T) {
	cases := []struct {
		code int
		want dto.FaultCategory
		why  string
	}{
		{899999, dto.FaultCategoryOther, "below every range"},
		{900799, dto.FaultCategoryOther, "just below throttling"},
		{900800, dto.FaultCategoryThrottled, "first throttling code"},
		{900899, dto.FaultCategoryThrottled, "last throttling code"},
		{900900, dto.FaultCategoryAuth, "throttling ends where auth begins"},
		{900999, dto.FaultCategoryAuth, "last auth code"},
		{901000, dto.FaultCategoryOther, "one past auth"},
		{101499, dto.FaultCategoryOther, "just below target"},
		{101500, dto.FaultCategoryTargetConnectivity, "first target code"},
		{101599, dto.FaultCategoryTargetConnectivity, "last target code"},
		{101600, dto.FaultCategoryOther, "one past target"},
		// 303001 is a target failure that sits outside the target range, so it is the one
		// code that must be named rather than ranged.
		{sdkCode(t, policy.FaultCodeUpstreamUnavailable), dto.FaultCategoryTargetConnectivity, "endpoint suspended"},
		// The blocks deliberately outside every range. All must report OTHER — see
		// docs/gateway/error-codes.md.
		{906201, dto.FaultCategoryOther, "a guardrail sub-code classifies as other"},
		{962401, dto.FaultCategoryOther, "a shipped-policy code classifies as other"},
		{965100, dto.FaultCategoryOther, "a customer policy code classifies as other"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, faultCategoryForCode(c.code), "code %d: %s", c.code, c.why)
	}
}

// The subcategory must never contradict the category printed beside it, which is why an
// unenumerated code falls back to its category's own catch-all rather than to UNCLASSIFIED.
func TestFaultSubCategoryForCode(t *testing.T) {
	cases := []struct {
		code int
		want dto.FaultSubCategory
	}{
		{sdkCode(t, policy.FaultCodeAuthMissingCredentials), dto.AuthenticationFailure},
		{sdkCode(t, policy.FaultCodeAuthForbidden), dto.AuthenticationAuthorizationFailure},
		{sdkCode(t, policy.FaultCodeInvalidScope), dto.AuthenticationAuthorizationFailure},
		{sdkCode(t, policy.FaultCodeSubscriptionInactive), dto.AuthenticationSubscriptionValidationFailure},
		{APIThrottleOutErrorCode, dto.ThrottlingAPILimitExceeded},
		{ResourceThrottleOutErrorCode, dto.ThrottlingResourceLimitExceeded},
		{SubscriptionThrottleOutErrorCode, dto.ThrottlingSubscriptionLimitExceeded},
		{NhttpConnectionTimeout, dto.TargetConnectivityConnectionTimeout},
		{sdkCode(t, policy.FaultCodeUpstreamUnavailable), dto.TargetConnectivityConnectionSuspended},
		{sdkCode(t, policy.FaultCodeNoRoute), dto.OtherResourceNotFound},
		// Unenumerated, inside a range: the category's catch-all, never UNCLASSIFIED.
		{900850, dto.ThrottlingOther},
		{900950, dto.AuthenticationOther},
		{101550, dto.TargetConnectivityOther},
		// Unenumerated, outside every range.
		{906201, dto.OtherUnclassified},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, faultSubCategoryForCode(c.code), "code %d", c.code)
	}
}

// A successful request must not acquire an error object. Event.Error is a pointer precisely
// so "no failure" is expressible, and an empty struct would read as a fault with no detail.
func TestApplyFaultDetails_SuccessfulRequestStaysClean(t *testing.T) {
	event := &dto.Event{}

	applyFaultDetails(event, map[string]string{}, map[string]interface{}{})

	assert.Nil(t, event.Error, "no fault metadata means no error object")
	assert.Empty(t, event.ErrorType)
	assert.Empty(t, event.Properties, "and no fault keys leak into the property bag")
}

// The whole point of the mechanism: an event that can say WHY, not only that a status was
// returned — and say it in typed fields a consumer can read without knowing key names.
func TestApplyFaultDetails_ClassifiesAndCarriesTheDetail(t *testing.T) {
	event := &dto.Event{}
	kv := map[string]string{
		dto.PropKeyFaultCode:        "900902",
		dto.PropKeyFaultType:        "authentication",
		dto.PropKeyFaultMessage:     "Valid credentials required",
		dto.PropKeyFaultPolicy:      "jwt-auth",
		dto.PropKeyFaultPolicyPhase: "request_headers",
		dto.PropKeyFaultSource:      "gateway",
		dto.PropKeyFaultDirection:   "request",
	}
	tv := map[string]interface{}{dto.PropKeyFaultStatus: float64(401)}

	applyFaultDetails(event, kv, tv)

	require.NotNil(t, event.Error)
	assert.Equal(t, 900902, event.Error.ErrorCode)
	assert.Equal(t, dto.AuthenticationFailure, event.Error.ErrorMessage)
	assert.Equal(t, string(dto.FaultCategoryAuth), event.ErrorType)
	assert.Equal(t, "authentication", event.Error.Type)
	assert.Equal(t, "request", event.Error.Direction)
	assert.Equal(t, "Valid credentials required", event.Error.Summary)
	assert.Equal(t, "jwt-auth", event.Error.Policy)
	assert.Equal(t, "request_headers", event.Error.PolicyPhase)
	assert.Equal(t, "gateway", event.Error.Source)

	// The transport keys must not also appear as properties: Properties is a shared bag
	// whose keys each publisher cherry-picks, and duplicating typed data into it is what
	// this design moved away from.
	assert.Empty(t, event.Properties, "fault detail belongs on Error, not in Properties")
}

// No Status field on Error, on purpose: ProxyResponseCode already carries the status the
// client received, and a second copy could only ever disagree with it.
func TestApplyFaultDetails_StatusIsNotDuplicated(t *testing.T) {
	event := &dto.Event{ProxyResponseCode: 401}

	applyFaultDetails(event,
		map[string]string{dto.PropKeyFaultCode: "900902"},
		map[string]interface{}{dto.PropKeyFaultStatus: float64(401)})

	require.NotNil(t, event.Error)
	assert.Equal(t, 401, event.ProxyResponseCode, "the one place the final status lives")
}

// OriginalStatus is the opposite case — it is not recoverable anywhere else once a policy
// has rewritten the status, so it does belong on Error.
func TestApplyFaultDetails_CarriesTheUpstreamsOriginalStatus(t *testing.T) {
	event := &dto.Event{ProxyResponseCode: 446}

	applyFaultDetails(event,
		map[string]string{dto.PropKeyFaultCode: "906201"},
		map[string]interface{}{dto.PropKeyFaultOriginalStatus: float64(200)})

	require.NotNil(t, event.Error)
	assert.Equal(t, 200, event.Error.OriginalStatus,
		"a guardrail turned a 200 into a 446; the 200 exists nowhere else")
}

// A guardrail rejection is the case that shows why the derived category and the policy's own
// type are both carried: it categorises as OTHER (its code is outside every classification range)
// while its type is precisely "guardrail". Dropping either would lose real information.
func TestApplyFaultDetails_GuardrailKeepsBothAnswers(t *testing.T) {
	event := &dto.Event{}
	kv := map[string]string{
		dto.PropKeyFaultCode:            "906201",
		dto.PropKeyFaultType:            "guardrail",
		dto.PropKeyFaultGuardrail:       "word-count-guardrail",
		dto.PropKeyFaultGuardrailAction: "INTERVENED",
		dto.PropKeyFaultGuardrailReason: "word count out of range",
	}

	applyFaultDetails(event, kv, map[string]interface{}{})

	require.NotNil(t, event.Error)
	assert.Equal(t, 906201, event.Error.ErrorCode)
	assert.Equal(t, string(dto.FaultCategoryOther), event.ErrorType,
		"906xxx is outside the one-code-wide guardrail range")
	assert.Equal(t, "guardrail", event.Error.Type,
		"the policy's own class is the specific answer the category cannot give")

	require.NotNil(t, event.Error.Guardrail)
	assert.Equal(t, "word-count-guardrail", event.Error.Guardrail.Name)
	assert.Equal(t, "INTERVENED", event.Error.Guardrail.Action)
	assert.Equal(t, "word count out of range", event.Error.Guardrail.Reason)
}

// Guardrail is nil for every non-guardrail failure — a pointer so "not a guardrail" is
// expressible at all, rather than an empty object a consumer has to inspect field by field.
func TestApplyFaultDetails_NonGuardrailHasNoGuardrailObject(t *testing.T) {
	event := &dto.Event{}

	applyFaultDetails(event, map[string]string{
		dto.PropKeyFaultCode: "900902",
		dto.PropKeyFaultType: "authentication",
	}, map[string]interface{}{})

	require.NotNil(t, event.Error)
	assert.Nil(t, event.Error.Guardrail)
}

// A failure the gateway described but never assigned a code to — a router error, for
// instance — must still report its source and attribution rather than nothing.
func TestApplyFaultDetails_NoCodeStillReportsTheSource(t *testing.T) {
	event := &dto.Event{}
	kv := map[string]string{
		dto.PropKeyFaultSource: "router",
		dto.PropKeyFaultType:   "routing",
	}

	applyFaultDetails(event, kv, map[string]interface{}{})

	require.NotNil(t, event.Error, "no code is not the same as no failure")
	assert.Equal(t, 0, event.Error.ErrorCode, "nothing to classify without a code")
	assert.Equal(t, dto.OtherUnclassified, event.Error.ErrorMessage)
	assert.Equal(t, string(dto.FaultCategoryOther), event.ErrorType)
	assert.Equal(t, "router", event.Error.Source)
	assert.Equal(t, "routing", event.Error.Type)
}

// A non-numeric code is a policy bug. It must not panic or drop the error; it stays visible
// where a reader of the error will actually see it.
func TestApplyFaultDetails_NonNumericCodeIsDiagnosable(t *testing.T) {
	event := &dto.Event{}

	applyFaultDetails(event, map[string]string{dto.PropKeyFaultCode: "GW-1234"}, map[string]interface{}{})

	require.NotNil(t, event.Error)
	assert.Equal(t, 0, event.Error.ErrorCode)
	assert.Equal(t, dto.OtherUnclassified, event.Error.ErrorMessage)
	assert.Equal(t, string(dto.FaultCategoryOther), event.ErrorType)
	assert.Contains(t, event.Error.Summary, "GW-1234",
		"kept so the bug is findable rather than silently swallowed")
}

// A classified fault with no message must still serialise as the established shape:
// the two original fields present, everything else omitted.
func TestApplyFaultDetails_SparseFaultKeepsTheEstablishedShape(t *testing.T) {
	event := &dto.Event{}

	applyFaultDetails(event, map[string]string{dto.PropKeyFaultCode: "900800"}, map[string]interface{}{})

	require.NotNil(t, event.Error)
	raw, err := json.Marshal(event.Error)
	require.NoError(t, err)
	assert.JSONEq(t, `{"errorCode":900800,"errorMessage":"API_LEVEL_LIMIT_EXCEEDED"}`, string(raw),
		"every added field is omitempty, so a sparse fault is exactly the two-field object")
}

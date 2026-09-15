/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package analytics

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// faultEntry builds a minimal access-log entry: the flags Envoy set, plus the
// status and detail string the client saw.
func faultEntry(flags *v3.ResponseFlags, status uint32, details string) *v3.HTTPAccessLogEntry {
	return &v3.HTTPAccessLogEntry{
		CommonProperties: &v3.AccessLogCommon{ResponseFlags: flags},
		Response: &v3.HTTPResponseProperties{
			ResponseCode:        wrapperspb.UInt32(status),
			ResponseCodeDetails: details,
		},
	}
}

// Every flag in the table must produce its own error.type and category. Driven
// off flagFaults itself, using reflection to set the one field under test, so a
// flag added to the table without a mapping cannot silently go untested.
func TestClassifyFault_EveryMappedFlag(t *testing.T) {
	for _, mapped := range flagFaults {
		t.Run(mapped.name, func(t *testing.T) {
			flags := &v3.ResponseFlags{}
			setFlagByErrorType(t, flags, mapped.name)

			got := classifyFault(faultEntry(flags, 503, ""))
			// The category is what lands in ErrorType — the field existing Moesif
			// consumers read. The flag name only identifies the table row.
			assert.Equal(t, mapped.category, got.ErrorType)
			assert.Equal(t, mapped.subCategory, got.SubCategory)
		})
	}
}

// setFlagByErrorType sets the ResponseFlags field whose snake_case name matches
// the table entry, proving the two agree.
func setFlagByErrorType(t *testing.T, flags *v3.ResponseFlags, errorType string) {
	t.Helper()
	target := snakeToPascal(errorType)
	value := reflect.ValueOf(flags).Elem()
	field := value.FieldByName(target)
	require.True(t, field.IsValid(), "no ResponseFlags field named %q for error type %q", target, errorType)
	switch field.Kind() {
	case reflect.Bool:
		field.SetBool(true)
	case reflect.Ptr:
		// UnauthorizedDetails is a message, not a bool.
		field.Set(reflect.New(field.Type().Elem()))
	default:
		t.Fatalf("unexpected kind %s for %s", field.Kind(), target)
	}
}

func snakeToPascal(s string) string {
	out := []byte{}
	upper := true
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			upper = true
			continue
		}
		c := s[i]
		if upper && c >= 'a' && c <= 'z' {
			c -= 32
		}
		upper = false
		out = append(out, c)
	}
	return string(out)
}

// The three informational flags describe deliberate behaviour, not failure. A
// cache hit counted as a fault would be a silent, permanent error-rate inflation.
func TestClassifyFault_InformationalFlagsAreNotFaults(t *testing.T) {
	cases := map[string]*v3.ResponseFlags{
		"cache hit":      {ResponseFromCacheFilter: true},
		"delay injected": {DelayInjected: true},
		"fault injected": {FaultInjected: true},
	}
	for name, flags := range cases {
		t.Run(name, func(t *testing.T) {
			got := classifyFault(faultEntry(flags, 200, responseCodeDetailsViaUpstream))
			assert.Empty(t, got.ErrorType)
			assert.Empty(t, got.SubCategory)
		})
	}
}

// The distinction the status code cannot make: same 503, two different causes.
func TestClassifyFault_SameStatusDifferentCause(t *testing.T) {
	gateway := classifyFault(faultEntry(&v3.ResponseFlags{NoHealthyUpstream: true}, 503, "no_healthy_upstream"))
	assert.Equal(t, dto.FaultCategoryTargetConnectivity, gateway.ErrorType,
		"a 503 the backend never saw is a connectivity fault")

	backend := classifyFault(faultEntry(&v3.ResponseFlags{}, 503, responseCodeDetailsViaUpstream))
	assert.Equal(t, dto.FaultCategoryOther, backend.ErrorType,
		"a 503 the backend answered is not a connectivity fault")
}

func TestClassifyFault_UpstreamResponses(t *testing.T) {
	cases := []struct {
		name     string
		status   uint32
		wantType dto.FaultCategory
		wantSub  dto.FaultSubCategory
	}{
		{"200 is not a fault", 200, "", ""},
		{"301 is not a fault", 301, "", ""},
		// The backend chose these, so they are recorded as errors but the
		// gateway-specific sub-categories are withheld: the gateway neither
		// authenticated, throttled, nor failed to route.
		{"400 is a generic error", 400, dto.FaultCategoryOther, dto.OtherUnclassified},
		{"401 is not attributed to gateway auth", 401, dto.FaultCategoryOther, dto.OtherUnclassified},
		{"403 is not attributed to gateway authz", 403, dto.FaultCategoryOther, dto.OtherUnclassified},
		{"404 is a generic error", 404, dto.FaultCategoryOther, dto.OtherUnclassified},
		{"429 is not attributed to gateway throttling", 429, dto.FaultCategoryOther, dto.OtherUnclassified},
		{"500 is a backend fault", 500, dto.FaultCategoryOther, dto.OtherUnclassified},
		{"502 is a backend fault", 502, dto.FaultCategoryOther, dto.OtherUnclassified},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyFault(faultEntry(&v3.ResponseFlags{}, tc.status, responseCodeDetailsViaUpstream))
			assert.Equal(t, tc.wantType, got.ErrorType)
			assert.Equal(t, tc.wantSub, got.SubCategory)
		})
	}
}

// A response Envoy or a filter synthesized without setting a flag — a policy
// denial via ext_proc, a direct response. The detail string is high-cardinality
// so error.type gets one stable value instead.
func TestClassifyFault_SynthesizedResponses(t *testing.T) {
	cases := []struct {
		details  string
		status   uint32
		wantType dto.FaultCategory
	}{
		// A status in statusFaults names its own cause when the gateway
		// synthesized the response.
		{"ext_authz_denied", 403, dto.FaultCategoryAuth},
		{"direct_response", 401, dto.FaultCategoryAuth},
		{"direct_response", 404, dto.FaultCategoryOther},
		// Not in the table: an error, with nothing claimed about the cause.
		{"ext_proc_error_gRPC_error_13", 500, dto.FaultCategoryOther},
		// A synthesized redirect is not a failure.
		{"direct_response", 302, ""},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s_%d", tc.details, tc.status), func(t *testing.T) {
			got := classifyFault(faultEntry(&v3.ResponseFlags{}, tc.status, tc.details))
			assert.Equal(t, tc.wantType, got.ErrorType)
		})
	}
}

// Every entry in statusFaults must be reachable for a gateway-synthesized
// response. Driven off the table itself, so an entry added without a test
// cannot slip through. Empty details are used deliberately: that is exactly
// what the policy engine's own denials produce.
func TestClassifyFault_EveryMappedStatus(t *testing.T) {
	require.NotEmpty(t, statusFaults)
	for status, want := range statusFaults {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			got := classifyFault(faultEntry(&v3.ResponseFlags{}, uint32(status), ""))
			assert.Equal(t, want.category, got.ErrorType)
			assert.Equal(t, want.subCategory, got.SubCategory)
		})
	}
}

// The gating: an identical status code classifies differently depending on who
// produced the response. Without this, a backend's own 401/429 would be
// attributed to the gateway's authentication or rate limiting.
func TestClassifyFault_StatusMappingIsGatedOnOrigin(t *testing.T) {
	cases := []struct {
		status      uint32
		wantGateway dto.FaultCategory
		wantSubGw   dto.FaultSubCategory
	}{
		{http.StatusUnauthorized, dto.FaultCategoryAuth, dto.AuthenticationFailure},
		{http.StatusForbidden, dto.FaultCategoryAuth, dto.AuthenticationAuthorizationFailure},
		{http.StatusNotFound, dto.FaultCategoryOther, dto.OtherResourceNotFound},
		{http.StatusMethodNotAllowed, dto.FaultCategoryOther, dto.OtherMethodNotAllowed},
		{http.StatusTooManyRequests, dto.FaultCategoryThrottled, dto.ThrottlingOther},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			gw := classifyFault(faultEntry(&v3.ResponseFlags{}, tc.status, "direct_response"))
			assert.Equal(t, tc.wantGateway, gw.ErrorType, "gateway-synthesized names the cause")
			assert.Equal(t, tc.wantSubGw, gw.SubCategory)

			up := classifyFault(faultEntry(&v3.ResponseFlags{}, tc.status, responseCodeDetailsViaUpstream))
			assert.Equal(t, dto.FaultCategoryOther, up.ErrorType,
				"an upstream-chosen status must not be attributed to the gateway")
			assert.Equal(t, dto.OtherUnclassified, up.SubCategory)
		})
	}
}

// A response flag outranks the status table: a 429 carrying RateLimited is
// classified by the flag, and a 404 from NoRouteFound keeps its flag-derived
// sub-category rather than being re-derived from the status.
func TestClassifyFault_FlagOutranksStatus(t *testing.T) {
	got := classifyFault(faultEntry(&v3.ResponseFlags{RateLimited: true}, 429, "direct_response"))
	assert.Equal(t, dto.FaultCategoryThrottled, got.ErrorType)
	assert.Equal(t, dto.ThrottlingOther, got.SubCategory)

	got = classifyFault(faultEntry(&v3.ResponseFlags{NoRouteFound: true}, 404, "route_not_found"))
	assert.Equal(t, dto.FaultCategoryOther, got.ErrorType)
	assert.Equal(t, dto.OtherResourceNotFound, got.SubCategory)
}

// The boundary is >= 400, not > 400: a 400 is an error, a 399 is not.
func TestClassifyFault_ErrorBoundaryIncludes400(t *testing.T) {
	got := classifyFault(faultEntry(&v3.ResponseFlags{}, 400, "direct_response"))
	assert.Equal(t, dto.FaultCategoryOther, got.ErrorType, "400 is an error")
	assert.Equal(t, dto.OtherUnclassified, got.SubCategory)

	for _, status := range []uint32{200, 204, 301, 304, 399} {
		got := classifyFault(faultEntry(&v3.ResponseFlags{}, status, "direct_response"))
		assert.Empty(t, got.ErrorType, "status %d must not be a fault", status)
	}
}

// Several flags are routinely set on one request. The table is ordered, so the
// result must be deterministic — a map would return either one at random.
func TestClassifyFault_MultipleFlagsAreDeterministic(t *testing.T) {
	flags := &v3.ResponseFlags{
		UpstreamRequestTimeout:     true,
		UpstreamRetryLimitExceeded: true,
		LocalReset:                 true,
	}
	for i := 0; i < 50; i++ {
		got := classifyFault(faultEntry(flags, 504, ""))
		require.Equal(t, dto.FaultCategoryTargetConnectivity, got.ErrorType,
			"the most specific flag must win on every call")
		require.Equal(t, dto.TargetConnectivityConnectionTimeout, got.SubCategory,
			"upstream_request_timeout maps to CONNECTION_TIMEOUT, not the generic OTHER")
	}
}

// A rate-limit *service* failure is gateway infrastructure, not the client being
// throttled: counting it as THROTTLED would misattribute an outage to callers.
func TestClassifyFault_RateLimitServiceErrorIsNotThrottling(t *testing.T) {
	got := classifyFault(faultEntry(&v3.ResponseFlags{RateLimitServiceError: true}, 500, ""))
	assert.Equal(t, dto.FaultCategoryOther, got.ErrorType)
	assert.Equal(t, dto.OtherMediationError, got.SubCategory)
}

// Nothing may panic on a sparse entry: the ALS message is built by Envoy, and
// every level of it is optional.
func TestClassifyFault_NilSafety(t *testing.T) {
	cases := map[string]*v3.HTTPAccessLogEntry{
		"empty entry":      {},
		"no common props":  {Response: &v3.HTTPResponseProperties{}},
		"no response":      {CommonProperties: &v3.AccessLogCommon{}},
		"no flags":         {CommonProperties: &v3.AccessLogCommon{}, Response: &v3.HTTPResponseProperties{}},
		"no response code": {CommonProperties: &v3.AccessLogCommon{ResponseFlags: &v3.ResponseFlags{}}, Response: &v3.HTTPResponseProperties{ResponseCodeDetails: responseCodeDetailsViaUpstream}},
	}
	for name, entry := range cases {
		t.Run(name, func(t *testing.T) {
			got := classifyFault(entry)
			assert.Empty(t, got.ErrorType)
		})
	}
}

func TestIsCacheHit(t *testing.T) {
	assert.True(t, isCacheHit(faultEntry(&v3.ResponseFlags{ResponseFromCacheFilter: true}, 200, "")))
	assert.False(t, isCacheHit(faultEntry(&v3.ResponseFlags{}, 200, "")))
	assert.False(t, isCacheHit(&v3.HTTPAccessLogEntry{}), "a sparse entry must not panic")
}

// The classifier is only useful if prepareAnalyticEvent actually writes its
// output onto the event every publisher then reads.
func TestPrepareAnalyticEvent_WritesFaultClassification(t *testing.T) {
	analytics := NewAnalytics(&config.Config{})

	t.Run("gateway fault", func(t *testing.T) {
		logEntry := createLogEntryWithMetadata(map[string]string{APIIDKey: "api-1"})
		logEntry.CommonProperties.ResponseFlags = &v3.ResponseFlags{UpstreamRequestTimeout: true}
		logEntry.Response = &v3.HTTPResponseProperties{ResponseCode: wrapperspb.UInt32(504)}

		event := analytics.prepareAnalyticEvent(logEntry)
		require.NotNil(t, event)
		// The category lands in ErrorType — the field existing consumers read.
		assert.Equal(t, string(dto.FaultCategoryTargetConnectivity), event.ErrorType)
		require.NotNil(t, event.Error)
		assert.Equal(t, dto.TargetConnectivityConnectionTimeout, event.Error.ErrorMessage)
		// The client-visible status, populated before the classification runs.
		assert.Equal(t, 504, event.Error.ErrorCode)
	})

	t.Run("clean upstream response leaves the fields unset", func(t *testing.T) {
		logEntry := createLogEntryWithMetadata(map[string]string{APIIDKey: "api-1"})
		logEntry.CommonProperties.ResponseFlags = &v3.ResponseFlags{}
		logEntry.Response = &v3.HTTPResponseProperties{
			ResponseCode:        wrapperspb.UInt32(200),
			ResponseCodeDetails: responseCodeDetailsViaUpstream,
		}

		event := analytics.prepareAnalyticEvent(logEntry)
		require.NotNil(t, event)
		assert.Empty(t, event.ErrorType)
		assert.Nil(t, event.Error, "a successful request must not carry an error object")
	})

	// ResponseFromCacheFilter replaces what was a hardcoded false.
	t.Run("cache hit reaches Target.ResponseCacheHit", func(t *testing.T) {
		logEntry := createLogEntryWithMetadata(map[string]string{APIIDKey: "api-1"})
		logEntry.CommonProperties.ResponseFlags = &v3.ResponseFlags{ResponseFromCacheFilter: true}
		logEntry.Response = &v3.HTTPResponseProperties{ResponseCode: wrapperspb.UInt32(200)}

		event := analytics.prepareAnalyticEvent(logEntry)
		require.NotNil(t, event)
		require.NotNil(t, event.Target)
		assert.True(t, event.Target.ResponseCacheHit)
		assert.Empty(t, event.ErrorType, "a cache hit is not a fault")
	})
}

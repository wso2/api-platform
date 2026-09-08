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
	"strconv"

	v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
)

// responseCodeDetailsViaUpstream is the value Envoy sets when the upstream
// produced the response. Any other value means Envoy or a filter synthesized it,
// which is the difference between "the backend answered 503" and "the backend
// never saw the request" — a distinction the status code alone cannot make.
const responseCodeDetailsViaUpstream = "via_upstream"

// faultClassification is the derived error view of one request, written onto the
// canonical event so every publisher reads one classification instead of each
// inventing its own.
type faultClassification struct {
	EventCategory dto.EventCategory
	FaultCategory dto.FaultCategory
	// ErrorType is the value of the stable OpenTelemetry `error.type` attribute:
	// the response flag's name for a gateway-originated fault, or the HTTP status
	// code for a response the upstream itself produced (which is what the HTTP
	// semantic conventions ask for).
	ErrorType string
	// SubCategory is set only where the response flag determines it. Where the
	// flag proves the category but not the specific cause — RateLimited does not
	// say which limit was hit — the category's own "OTHER" is used rather than a
	// guess.
	SubCategory dto.FaultSubCategory
}

// flagFault maps one Envoy response flag onto a classification.
type flagFault struct {
	name        string
	set         func(*v3.ResponseFlags) bool
	category    dto.FaultCategory
	subCategory dto.FaultSubCategory
}

// flagFaults is deliberately an ordered slice, not a map: several flags are set
// together on the same request (UpstreamRequestTimeout alongside
// UpstreamRetryLimitExceeded, for instance), and Go randomizes map iteration, so
// a map would yield a different error.type for identical requests. Most specific
// first.
//
// Three flags are deliberately absent because they do not describe a failure:
// DelayInjected and FaultInjected mark deliberate fault injection, and
// ResponseFromCacheFilter marks a cache hit. Treating "any flag set" as a fault
// would count every cached response as an error.
var flagFaults = []flagFault{
	// Access control, before anything upstream is attempted.
	{"unauthorized_details", func(f *v3.ResponseFlags) bool { return f.GetUnauthorizedDetails() != nil },
		dto.FaultCategoryAuth, dto.AuthenticationOther},
	{"rate_limited", (*v3.ResponseFlags).GetRateLimited,
		dto.FaultCategoryThrottled, dto.ThrottlingOther},
	// The rate-limit service itself failed. That is gateway infrastructure, not
	// the client being throttled, so it must not inflate throttling counts.
	{"rate_limit_service_error", (*v3.ResponseFlags).GetRateLimitServiceError,
		dto.FaultCategoryOther, dto.OtherMediationError},

	// Upstream timeouts. The flag names the cause precisely, so the
	// sub-category is derived rather than guessed.
	{"upstream_request_timeout", (*v3.ResponseFlags).GetUpstreamRequestTimeout,
		dto.FaultCategoryTargetConnectivity, dto.TargetConnectivityConnectionTimeout},
	{"stream_idle_timeout", (*v3.ResponseFlags).GetStreamIdleTimeout,
		dto.FaultCategoryTargetConnectivity, dto.TargetConnectivityConnectionTimeout},
	{"duration_timeout", (*v3.ResponseFlags).GetDurationTimeout,
		dto.FaultCategoryTargetConnectivity, dto.TargetConnectivityConnectionTimeout},
	{"upstream_max_stream_duration_reached", (*v3.ResponseFlags).GetUpstreamMaxStreamDurationReached,
		dto.FaultCategoryTargetConnectivity, dto.TargetConnectivityConnectionTimeout},

	// Upstream deliberately withheld: no member passed health checking, or a
	// circuit breaker shed the request to protect it.
	{"no_healthy_upstream", (*v3.ResponseFlags).GetNoHealthyUpstream,
		dto.FaultCategoryTargetConnectivity, dto.TargetConnectivityConnectionSuspended},
	{"failed_local_healthcheck", (*v3.ResponseFlags).GetFailedLocalHealthcheck,
		dto.FaultCategoryTargetConnectivity, dto.TargetConnectivityConnectionSuspended},
	{"upstream_overflow", (*v3.ResponseFlags).GetUpstreamOverflow,
		dto.FaultCategoryTargetConnectivity, dto.TargetConnectivityConnectionSuspended},

	// Upstream unreachable or the connection broke mid-flight.
	{"dns_resolution_failure", (*v3.ResponseFlags).GetDnsResolutionFailure,
		dto.FaultCategoryTargetConnectivity, dto.TargetConnectivityOther},
	{"upstream_connection_failure", (*v3.ResponseFlags).GetUpstreamConnectionFailure,
		dto.FaultCategoryTargetConnectivity, dto.TargetConnectivityOther},
	{"upstream_connection_termination", (*v3.ResponseFlags).GetUpstreamConnectionTermination,
		dto.FaultCategoryTargetConnectivity, dto.TargetConnectivityOther},
	{"upstream_remote_reset", (*v3.ResponseFlags).GetUpstreamRemoteReset,
		dto.FaultCategoryTargetConnectivity, dto.TargetConnectivityOther},
	{"upstream_protocol_error", (*v3.ResponseFlags).GetUpstreamProtocolError,
		dto.FaultCategoryTargetConnectivity, dto.TargetConnectivityOther},
	{"upstream_retry_limit_exceeded", (*v3.ResponseFlags).GetUpstreamRetryLimitExceeded,
		dto.FaultCategoryTargetConnectivity, dto.TargetConnectivityOther},

	// Gateway configuration: the request matched no route, cluster, or filter
	// config. Nothing upstream was attempted.
	{"no_route_found", (*v3.ResponseFlags).GetNoRouteFound,
		dto.FaultCategoryOther, dto.OtherResourceNotFound},
	{"no_cluster_found", (*v3.ResponseFlags).GetNoClusterFound,
		dto.FaultCategoryOther, dto.OtherResourceNotFound},
	{"no_filter_config_found", (*v3.ResponseFlags).GetNoFilterConfigFound,
		dto.FaultCategoryOther, dto.OtherMediationError},
	// The gateway shed the request to protect itself.
	{"overload_manager", (*v3.ResponseFlags).GetOverloadManager,
		dto.FaultCategoryOther, dto.OtherMediationError},

	// Client-side: the caller sent something unusable or went away. Kept as
	// distinct error types because downstream_connection_termination is routine
	// for streaming and LLM traffic (a cancelled response) and an operator will
	// want to exclude it from a fault-rate alert.
	{"invalid_envoy_request_headers", (*v3.ResponseFlags).GetInvalidEnvoyRequestHeaders,
		dto.FaultCategoryOther, dto.OtherUnclassified},
	{"downstream_protocol_error", (*v3.ResponseFlags).GetDownstreamProtocolError,
		dto.FaultCategoryOther, dto.OtherUnclassified},
	{"downstream_connection_termination", (*v3.ResponseFlags).GetDownstreamConnectionTermination,
		dto.FaultCategoryOther, dto.OtherUnclassified},
	{"downstream_remote_reset", (*v3.ResponseFlags).GetDownstreamRemoteReset,
		dto.FaultCategoryOther, dto.OtherUnclassified},
	// Last: Envoy reset the stream locally, which is often accompanied by a more
	// specific flag above.
	{"local_reset", (*v3.ResponseFlags).GetLocalReset,
		dto.FaultCategoryOther, dto.OtherUnclassified},
}

// classifyFault derives the error view of a request from the Envoy access log.
//
// Response flags are the primary signal rather than the status code, because the
// status code cannot separate a gateway fault from a backend one: a 503 with
// NoHealthyUpstream never reached the backend, while a 503 with no flags and
// via_upstream is the backend's own answer. Those belong in different categories
// and would be indistinguishable from a status-range table.
//
// Precedence: a matching response flag, then a response Envoy synthesized
// without setting one, then an upstream-produced 5xx.
func classifyFault(logEntry *v3.HTTPAccessLogEntry) faultClassification {
	success := faultClassification{EventCategory: dto.EventCategorySuccess}

	flags := logEntry.GetCommonProperties().GetResponseFlags()
	for _, candidate := range flagFaults {
		if candidate.set(flags) {
			return faultClassification{
				EventCategory: dto.EventCategoryFault,
				FaultCategory: candidate.category,
				ErrorType:     candidate.name,
				SubCategory:   candidate.subCategory,
			}
		}
	}

	response := logEntry.GetResponse()
	if response == nil {
		return success
	}
	status := int(response.GetResponseCode().GetValue())
	details := response.GetResponseCodeDetails()

	// No flag matched, but the response did not come from the upstream: a filter
	// or Envoy produced it (a policy denial, a direct response, a redirect). The
	// detail string itself is high-cardinality and stays on
	// wso2.upstream.response.detail; error.type gets one stable value.
	if details != "" && details != responseCodeDetailsViaUpstream {
		if status < 400 {
			return success // a synthesized redirect or 200 is not a fault
		}
		return faultClassification{
			EventCategory: dto.EventCategoryFault,
			FaultCategory: dto.FaultCategoryOther,
			ErrorType:     "local_reply",
			SubCategory:   dto.OtherUnclassified,
		}
	}

	// The upstream answered. A 5xx is the backend's own failure — the gateway
	// worked — so it is a fault in a category that does not blame connectivity.
	// A 4xx is the backend's valid answer to an invalid request: error.type is
	// set because the HTTP operation did fail, but no fault is recorded against
	// the gateway.
	switch {
	case status >= 500:
		return faultClassification{
			EventCategory: dto.EventCategoryFault,
			FaultCategory: dto.FaultCategoryOther,
			ErrorType:     strconv.Itoa(status),
			SubCategory:   dto.OtherUnclassified,
		}
	case status >= 400:
		success.ErrorType = strconv.Itoa(status)
		return success
	}
	return success
}

// isCacheHit reports whether Envoy served the response from its cache filter.
// ResponseFromCacheFilter is one of the response flags that does not describe a
// failure, which is why it is handled here and not in flagFaults.
func isCacheHit(logEntry *v3.HTTPAccessLogEntry) bool {
	return logEntry.GetCommonProperties().GetResponseFlags().GetResponseFromCacheFilter()
}

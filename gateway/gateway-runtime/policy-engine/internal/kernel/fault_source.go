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
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Error-source classification.
//
// A backend's own 503 and the router's "no healthy upstream" 503 are the same status and
// arrive identically, so no status rule can separate them — one means the API is up and
// refusing, the other that the gateway never reached it.
//
// The router sends response.code_details as an ext_proc attribute (see the
// gateway-controller's createExtProcFilter), which is the signal used here:
//
//	backend returns 500      -> "via_upstream"
//	upstream unreachable     -> "upstream_reset_before_response_started{...}"
//	no route matched         -> "direct_response"
//
// response.flags is deliberately NOT used: a backend error and a no-route reply both report
// an empty flag set, and flags are finalised at log time, so they are empty over ext_proc at
// the response-headers phase.

// faultSource identifies who produced an error response.
type faultSource string

// Every value is taken from the SDK rather than re-typed. These strings reach a fault
// handler as FaultContext.Source and an execution condition as fault.Source, so they are
// configuration surface — and a second spelling of them here is exactly how the engine and
// the contract drift apart.
const (
	// sourceGateway is an error the policy engine built itself — a policy or guardrail
	// rejection. Always known, because the engine is the thing that produced it.
	sourceGateway faultSource = policy.FaultSourceGateway
	// sourceBackend is an error status the upstream returned. This is traffic passing
	// through, not a gateway failure: a REST backend answering 404 for a missing record
	// is behaving correctly.
	sourceBackend faultSource = policy.FaultSourceBackend
	// sourceRouter is a reply Envoy generated because it could not complete the proxy
	// attempt — connection failure, timeout, no healthy host.
	sourceRouter faultSource = policy.FaultSourceRouter
	// sourceNoRoute is a reply for a request that matched no API at all.
	sourceNoRoute faultSource = policy.FaultSourceNoRoute
	// sourceUnknown means the router did not send the provenance attribute — an older
	// router, or a phase where it is not populated. It must never be promoted to a source
	// it cannot be shown to be: a condition testing == "gateway" excludes it, and that is
	// the operator's call rather than the engine's.
	sourceUnknown faultSource = policy.FaultSourceUnknown
)

// Envoy's response_code_details values that carry structural meaning for us. Everything
// else is treated as "the router generated this", deliberately: the detail strings are
// Envoy-internal and gain suffixes across versions, so matching the two known-stable
// values and defaulting the rest is more durable than enumerating reasons.
const (
	codeDetailsViaUpstream    = "via_upstream"
	codeDetailsDirectResponse = "direct_response"
)

// classifyFaultSource determines who produced the error currently being handled.
//
// The origin settles it outright for a policy-produced failure: the engine built that
// response itself, so attributes describing the proxy attempt say nothing about it. Only a
// failure carried by a pass-through response needs Envoy's account of what happened.
func classifyFaultSource(origin faultOrigin, codeDetails string) faultSource {
	if origin == originGateway {
		return sourceGateway
	}
	switch codeDetails {
	case "":
		return sourceUnknown
	case codeDetailsViaUpstream:
		return sourceBackend
	case codeDetailsDirectResponse:
		return sourceNoRoute
	default:
		return sourceRouter
	}
}

// Provenance classifies, it does not gate.
//
// faultSource reaches a fault handler as FaultContext.Source and an execution condition as
// fault.Source, which is where a deployment that only wants gateway failures says so:
//
//	executionCondition: fault.Source == "gateway"
//
// sourceUnknown means the router sent no provenance attribute, so nothing can be said about
// who produced the response — a condition testing == "gateway" correctly excludes it.

// Router-generated error descriptions.
//
// A router failure has no producing policy, so nothing supplies a FaultDetails for it. The
// engine describes these itself from the one signal it has: Envoy's code_details.
//
// The messages here are OURS, not Envoy's, whose own bodies name internal proxy mechanics.
// The raw value is logged instead of returned.
const (
	codeDetailsNoHealthyUpstream = "no_healthy_upstream"
	codeDetailsResponseTimeout   = "response_timeout"
	codeDetailsNoRouteMatched    = "route_not_found"
	// codeDetailsUpstreamResetPrefix matches the family Envoy suffixes with a reason in
	// braces — "upstream_reset_before_response_started{connection_failure}" and siblings.
	// Matched by prefix because the suffix set grows between Envoy versions.
	codeDetailsUpstreamResetPrefix = "upstream_reset"
)

// Error codes for failures the gateway produces itself.
//
// Six-digit numeric strings. Every value is an alias for the SDK's exported constant rather
// than a second spelling of the digits, so the engine and a policy branching on the same
// failure cannot disagree about what it is called.
//
// A code's range decides its analytics category, so an upstream failure has to carry a code
// inside the target-failure range (101500-101600) or it is classified as "other". The generic
// upstream code below sits at the TOP of that range because Synapse allocates upward from
// 101500.
const (
	// codeUpstreamUnreachable is a connection that could not be established.
	codeUpstreamUnreachable = policy.FaultCodeUpstreamUnreachable // 101503
	// codeUpstreamTimeout is an upstream that did not respond in time.
	codeUpstreamTimeout = policy.FaultCodeUpstreamTimeout // 101504
	// codeUpstreamUnavailable is no healthy host to route to. Classified under a distinct
	// sub-category (CONNECTION_SUSPENDED) from a failed connection attempt — the same
	// distinction the router draws between "no_healthy_upstream" and "upstream_reset".
	codeUpstreamUnavailable = policy.FaultCodeUpstreamUnavailable // 303001
	// codeNoRoute is a request that matched no API.
	codeNoRoute = policy.FaultCodeNoRoute // 900906
	// codeUpstreamGeneric is a router failure with no more specific cause. Inside the
	// target-failure range so it classifies as one; see the note above.
	codeUpstreamGeneric = policy.FaultCodeUpstreamGeneric // 101599
)

// Codes with no reserved equivalent.
//
// Allocated in 905xxx, above the 904015 frontier, and classified as "other" — there is no
// existing category to claim, and squatting inside a reserved range to borrow its label
// would mis-report them as target or auth failures.
const (
	// codeEngineInternal is a policy chain that failed to execute.
	codeEngineInternal = policy.FaultCodeEngineInternal // 905003
	// codePayloadTooLarge is a request body over the configured decompression ceiling.
	codePayloadTooLarge = policy.FaultCodePayloadTooLarge // 905004
	// codeNoPolicyChain is a route Envoy routed here that the engine holds no policy chain
	// for. Distinct from codeEngineInternal because it is a different thing to fix: the
	// chain never arrived, rather than arrived and failed. Both are opaque 500s to the
	// caller, and both are request-independent, so naming them apart discloses nothing a
	// caller could probe.
	codeNoPolicyChain = policy.FaultCodeNoPolicyChain // 905005
	// codeResolutionBadRequest is a request the route's resolver could not turn into an
	// operation — unparseable, invalid, or asking for more than one operation at once.
	codeResolutionBadRequest = policy.FaultCodeResolutionBadRequest // 905006
	// codeResolutionUnsupportedEncoding is a content coding the gateway cannot decode.
	codeResolutionUnsupportedEncoding = policy.FaultCodeResolutionUnsupportedEncoding // 905007
)

// routerErrorFor describes a router-generated failure, or returns nil when the error was not
// the router's to describe.
//
// nil is returned for a gateway or backend error: a policy rejection has its own
// FaultDetails (or deliberately none), and a backend error is that API's own contract and
// never enters the fault flow at all.
func routerErrorFor(src faultSource, codeDetails string) *policy.FaultDetails {
	if src != sourceRouter && src != sourceNoRoute {
		return nil
	}

	code, errType, message := codeUpstreamGeneric, policy.FaultTypeUpstream,
		"The request could not be completed."

	switch {
	case src == sourceNoRoute || codeDetails == codeDetailsNoRouteMatched:
		code, errType, message = codeNoRoute, policy.FaultTypeRouting, "No API matched the request."
	case codeDetails == codeDetailsNoHealthyUpstream:
		code, message = codeUpstreamUnavailable, "The upstream service is unavailable."
	case codeDetails == codeDetailsResponseTimeout:
		code, message = codeUpstreamTimeout, "The upstream service did not respond in time."
	case strings.HasPrefix(codeDetails, codeDetailsUpstreamResetPrefix):
		code, message = codeUpstreamUnreachable, "The upstream service could not be reached."
	}

	return &policy.FaultDetails{
		Code:      code,
		Type:      errType,
		Direction: policy.DirectionResponse,
		Message:   message,
		// Policy stays empty: no policy caused a connection failure, and the contract says
		// empty means "not caused by a policy" rather than "unknown".
	}
}

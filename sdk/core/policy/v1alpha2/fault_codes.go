/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

package policyv1alpha2

import "strconv"

// Vocabularies for identifying a failure: the values for FaultDetails.Code and .Type, and for
// FaultContext.Source.
//
// Two groups:
//
//   - Codes a POLICY sets, describing why it rejected the exchange. Use these.
//   - Codes the GATEWAY sets, for failures no policy produced. A policy must not emit one, but
//     a fault policy may branch on one.
//
// A code's numeric range is its category — see the range constants at the bottom of this file,
// and docs/gateway/error-codes.md.

// ─── Type: the failure class ─────────────────────────────────────────────────

// FaultType values for FaultDetails.Type: the failure classes a policy or the gateway can
// name, so every producer spells the same class the same way.
//
// These reach the client — Type is rendered into the error body for every shape — so a value
// here is a wire contract and renaming one is a breaking change.
//
// Canonical, not closed: Type is a plain string and nothing validates it, so a policy may use
// its own value. Pick the constant that fits before adding one.
//
// The first group is for a policy describing why IT rejected the exchange:
//
//	FaultTypeAuthentication  the caller could not be identified — credential missing,
//	                         malformed, expired or unverifiable
//	FaultTypeAuthorization   the caller was identified but is not permitted — scope, ACL,
//	                         subscription, tool or resource not allowed
//	FaultTypeThrottling      a rate limit or quota was reached
//	FaultTypeGuardrail       content inspection rejected the request or the response; pair
//	                         it with FaultDetails.Guardrail so the assessment travels too
//	FaultTypeValidation      the message did not satisfy a required shape — unparseable
//	                         body, missing field, schema mismatch
//	FaultTypeMediation       a transformation, translation or rewrite step failed
//	FaultTypeConfiguration   the policy's own parameters are unusable. An OPERATOR error,
//	                         not a caller error: the same request would fail identically
//	                         from anyone, so it usually deserves a 5xx rather than a 4xx
//
// The second group is set by the GATEWAY, for failures no policy produced. Listed because
// they appear in client-visible bodies:
//
//	FaultTypeUpstream     the proxy attempt could not be completed — no healthy host,
//	                      connection reset, upstream timeout
//	FaultTypeRouting      no API matched the request
//	FaultTypeInternal     the gateway itself failed, e.g. a policy chain that could not run
//	FaultTypeRequestSize  the request body exceeded the configured decompression ceiling
const (
	// Policy-produced classes.
	FaultTypeAuthentication = "authentication"
	FaultTypeAuthorization  = "authorization"
	FaultTypeThrottling     = "throttling"
	FaultTypeGuardrail      = "guardrail"
	FaultTypeValidation     = "validation"
	FaultTypeMediation      = "mediation"
	FaultTypeConfiguration  = "configuration"

	// Gateway-produced classes. Set by the engine, not by a policy.
	FaultTypeUpstream = "upstream"
	FaultTypeRouting  = "routing"
	FaultTypeInternal = "internal"
	// FaultTypeRequestSize is the one multi-word value, and the only one not spelled as a
	// single lowercase word. Kept as-is because it is already what the gateway emits.
	FaultTypeRequestSize = "requestSize"
)

// ─── Source: which actor produced the response ───────────────────────────────

// FaultSource values for FaultContext.Source: which actor produced the error response.
//
// These also reach an execution condition as fault.Source, so a value here is configuration
// surface: renaming one breaks every deployment whose conditions test for it.
//
// Unlike FaultType, this set IS closed. Type describes what went wrong, which no vocabulary
// can enumerate ahead of the policies that will need it; this describes who produced the
// response, and there are only so many actors in a proxy.
const (
	// FaultSourceGateway is an error the gateway built itself — a policy or guardrail
	// rejection, or an engine failure.
	FaultSourceGateway = "gateway"
	// FaultSourceBackend is an error status the upstream returned. The API is reachable and
	// refusing; this is traffic passing through, not a gateway failure.
	FaultSourceBackend = "backend"
	// FaultSourceRouter is a reply the proxy generated because it could not complete the
	// attempt — connection failure, timeout, no healthy host.
	FaultSourceRouter = "router"
	// FaultSourceNoRoute is a reply for a request that matched no API.
	FaultSourceNoRoute = "noRoute"
	// FaultSourceUnknown means the proxy sent no provenance, so who produced the response
	// cannot be established — an older router, or a phase where it is not populated.
	//
	// It is reported rather than guessed at. A condition testing for a specific source
	// excludes it, which is the deployment's call rather than the gateway's.
	FaultSourceUnknown = "unknown"
)

// ─── Codes a policy sets ─────────────────────────────────────────────────────

// Authentication and authorization codes, for authentication and authorization failures.
//
// Reused rather than reinvented: a client that has keyed off 900902 for years should not have
// to learn a second number because the gateway was rewritten. All of them fall inside the
// auth-failure range, so they categorise as authentication failures — which is the reason an
// auth policy must not reach for a code from another group.
const (
	// FaultCodeAuthGeneral is an authentication failure with no more specific cause.
	FaultCodeAuthGeneral = "900900"
	// FaultCodeAuthInvalidCredentials is a credential that was presented and rejected.
	FaultCodeAuthInvalidCredentials = "900901"
	// FaultCodeAuthMissingCredentials is no credential presented at all. Distinct from
	// invalid on purpose: one is a caller who tried, the other a caller who did not, and
	// they need different client-side handling.
	FaultCodeAuthMissingCredentials = "900902"
	// FaultCodeAuthTokenExpired is a token that was valid and is no longer.
	FaultCodeAuthTokenExpired = "900903"
	// FaultCodeAuthTokenInactive is a token that was revoked or never activated.
	FaultCodeAuthTokenInactive = "900904"
	// FaultCodeAuthIncorrectTokenType is a token of the wrong type for this API.
	FaultCodeAuthIncorrectTokenType = "900905"
	// FaultCodeAuthBlocked is an API or application blocked by an operator.
	FaultCodeAuthBlocked = "900907"
	// FaultCodeAuthForbidden is a caller who was identified and is not permitted. This is
	// the AUTHORIZATION failure — pair it with FaultTypeAuthorization, not Authentication.
	FaultCodeAuthForbidden = "900908"
	// FaultCodeSubscriptionInactive is a subscription that exists but is not active.
	FaultCodeSubscriptionInactive = "900909"
	// FaultCodeInvalidScope is a token whose scopes do not cover this operation.
	FaultCodeInvalidScope = "900910"
)

// Throttling codes, for rate-limit rejections.
//
// The LEVEL is part of the code rather than a detail beside it, because that is what a caller
// needs in order to react: an application-level limit means back off for this app, while a
// hard limit means the operator has capped the API outright and retrying will not help.
const (
	// FaultCodeThrottledAPI is an API-level rate limit.
	FaultCodeThrottledAPI = "900800"
	// FaultCodeThrottledResource is an operation-level rate limit.
	FaultCodeThrottledResource = "900802"
	// FaultCodeThrottledApplication is an application-level rate limit.
	FaultCodeThrottledApplication = "900803"
	// FaultCodeThrottledSubscription is a subscription-tier rate limit.
	FaultCodeThrottledSubscription = "900804"
	// FaultCodeThrottledBlocked is a request refused by a blocking condition rather than a
	// rate — an operator blocked this caller, IP or context.
	FaultCodeThrottledBlocked = "900805"
	// FaultCodeThrottledCustomPolicy is a custom throttling policy's limit.
	FaultCodeThrottledCustomPolicy = "900806"
)

// ─── Codes the gateway sets ──────────────────────────────────────────────────

// Upstream and routing codes, set by the engine from the proxy's own account of the attempt.
//
// A policy must NOT emit these. They are exported so a fault policy can branch on them:
// telling "the backend was unreachable" from "the backend answered slowly" is exactly the
// distinction an on-call notifier needs, and it is only available through the code.
//
// Every value sits inside the target-failure range so it categorises as an upstream failure —
// see the range note at the bottom of this file for why that is load-bearing.
const (
	// FaultCodeUpstreamUnreachable is a connection that could not be established.
	FaultCodeUpstreamUnreachable = "101503"
	// FaultCodeUpstreamTimeout is an upstream that did not respond in time.
	FaultCodeUpstreamTimeout = "101504"
	// FaultCodeUpstreamGeneric is a proxy failure with no more specific cause.
	FaultCodeUpstreamGeneric = "101599"
	// FaultCodeUpstreamUnavailable is no healthy host to route to. this has its own
	// code and its own sub-category, distinct from a failed connection attempt — the same
	// distinction the router draws between "no_healthy_upstream" and "upstream_reset".
	FaultCodeUpstreamUnavailable = "303001"
	// FaultCodeNoRoute is a request that matched no API.
	FaultCodeNoRoute = "900906"
)

// Engine codes, for conditions no reserved block covers.
//
// Allocated in 905xxx, above the 904015 frontier, so they collide with
// nothing. They fall in no category range, which is the honest outcome: there is no existing
// category to claim, and squatting inside a reserved range to borrow a label would mis-report
// them as target or auth failures.
//
// A policy must not emit these either — every one of them describes a failure of the engine
// itself, which a policy is in no position to observe.
const (
	// FaultCodeEngineInternal is a policy chain that failed to execute.
	FaultCodeEngineInternal = "905003"
	// FaultCodePayloadTooLarge is a request body over the configured decompression ceiling.
	FaultCodePayloadTooLarge = "905004"
	// FaultCodeNoPolicyChain is a route the engine holds no policy chain for. Distinct from
	// EngineInternal because it is a different thing to fix: the chain never arrived, rather
	// than arrived and failed.
	FaultCodeNoPolicyChain = "905005"
	// FaultCodeResolutionBadRequest is a request the route's resolver could not turn into an
	// operation — unparseable, invalid, or asking for more than one operation at once.
	FaultCodeResolutionBadRequest = "905006"
	// FaultCodeResolutionUnsupportedEncoding is a content coding the gateway cannot decode.
	FaultCodeResolutionUnsupportedEncoding = "905007"
)

// Guardrail error codes for FaultDetails.Code, for a guardrail that rejected content.
//
// GuardrailCodeIntervened (906000) is the generic code and the right answer for a guardrail
// with nothing more specific to say. Use a more specific code where a caller must tell two
// rejections apart.
//
// It sits at the head of the block, outside all four groups, so "any guardrail rejection" is
// one range test and "any content-safety rejection" does not accidentally include it.
//
// The 906xxx block sits above the engine's 905xxx codes and below the 96xxxx policy space,
// grouped so related rejections are adjacent:
//
//	906001-906099  what the content was judged to BE — a model or moderation service's verdict
//	906100-906199  what the content CONTAINED — sensitive data found in it
//	906200-906299  what SHAPE the content had — size, structure, pattern
//	906300-906399  what the content MEANT — intent matched against a configured policy
//
// The grouping is the contract, not decoration: a caller that wants "any content-safety
// rejection" tests 906001 <= code < 906100 rather than enumerating categories, and that only
// works if a new category lands inside its own group. Allocate within a group; do not append
// to the end of the block.
const (
	// GuardrailCodeIntervened is the generic guardrail rejection — see above. Prefer it
	// unless a caller needs to distinguish this rejection from another one.
	GuardrailCodeIntervened = "906000"

	// 906001-906099 — what the content was judged to be.
	//
	// The first four are the categories every major moderation service reports, and are the
	// reason this block exists: a caller handling a self-harm rejection and one handling a
	// violence rejection may need to route them to different places.
	GuardrailCodeHate              = "906001"
	GuardrailCodeSexual            = "906002"
	GuardrailCodeSelfHarm          = "906003"
	GuardrailCodeViolence          = "906004"
	GuardrailCodeHarassment        = "906005"
	GuardrailCodeDangerousActivity = "906006"
	GuardrailCodeProfanity         = "906007"
	// GuardrailCodePromptInjection is a jailbreak or instruction-override attempt. In this
	// group rather than the intent group below because a detector reports it as a property
	// of the content, the same way it reports hate.
	GuardrailCodePromptInjection = "906008"
	// GuardrailCodeUnsafeContent is the group's own generic: a moderation service said
	// "unsafe" without a category this vocabulary covers. Prefer a specific code where the
	// service gave one.
	GuardrailCodeUnsafeContent = "906099"

	// 906100-906199 — what the content contained.
	GuardrailCodePII        = "906101"
	GuardrailCodeCredential = "906102"

	// 906200-906299 — what shape the content had.
	GuardrailCodeWordCount     = "906201"
	GuardrailCodeSentenceCount = "906202"
	GuardrailCodeContentLength = "906203"
	GuardrailCodeSchema        = "906204"
	GuardrailCodePattern       = "906205"
	GuardrailCodeURL           = "906206"

	// 906300-906399 — what the content meant.
	GuardrailCodeSemanticMatch = "906301"
)

// Guardrail code group boundaries, half-open.
//
// Declared here and nowhere else. The engine reads these from the SDK rather than keeping a
// second copy, so there is no pair of numbers that could drift apart — a check the block would
// otherwise have needed and now does not.
//
// Exported so a caller can test group membership instead of enumerating codes — "was this any
// content-safety rejection?" is a question the specific codes exist to make answerable, and
// hand-written bounds in each consumer would drift as categories are added.
const (
	GuardrailCodeRangeStart = 906000
	GuardrailCodeRangeEnd   = 906400

	// Starts at 906001: 906000 is GuardrailCodeIntervened, which belongs to the block as a
	// whole rather than to any one group.
	GuardrailContentSafetyRangeStart = 906001
	GuardrailContentSafetyRangeEnd   = 906100

	GuardrailSensitiveDataRangeStart = 906100
	GuardrailSensitiveDataRangeEnd   = 906200

	GuardrailShapeRangeStart = 906200
	GuardrailShapeRangeEnd   = 906300

	GuardrailIntentRangeStart = 906300
	GuardrailIntentRangeEnd   = 906400
)

// IsGuardrailCode reports whether a code is a guardrail rejection.
//
// Every guardrail code including the generic lives in one block, so this is a single range
// test — the same rule every other category follows.
func IsGuardrailCode(code string) bool {
	n, err := strconv.Atoi(code)
	return err == nil && n >= GuardrailCodeRangeStart && n < GuardrailCodeRangeEnd
}

// ─── Where a new code may go ─────────────────────────────────────────────────

// Category ranges.
//
// A code's CATEGORY is the range it falls in — `start <= code < end` — so a code outside every
// range is categorised as "other" however specific its meaning. That makes range membership
// part of the contract rather than a convention: a new code for an upstream failure has to land
// inside the target range, or anything grouping failures by code stops seeing it as one.
//
// Exported so a policy allocating a code can check its own arithmetic, and so a consumer can
// ask "was this any auth failure?" without enumerating codes.
const (
	AuthFailureRangeStart      = 900900
	AuthFailureRangeEnd        = 901000
	ThrottledFailureRangeStart = 900800
	ThrottledFailureRangeEnd   = 900900
	TargetFailureRangeStart    = 101500
	TargetFailureRangeEnd      = 101600
)

// Codes any shipped policy may use, for the two conditions nearly all of them share.
//
// A policy that could not complete its own transformation, and a request body a policy could
// not read, are not one-policy conditions — most of them can hit both. Before these, each
// policy allocated its own number for them, which meant a code per policy for a condition
// that was never policy-specific.
//
// The specific-code argument does not survive the fault contract: FaultDetails carries
// Policy, and the engine fills it in, so which policy failed is already on the fault and in
// the analytics event. Encoding it a second time in the number says nothing the reader did
// not already have, and costs a registry entry per policy to keep straight.
//
// A policy still allocates its own code for a condition genuinely its own — no model
// available to route to, a provider's stream breaking mid-response — where the code names
// something the policy identity does not.
const (
	// FaultCodeMediationFailed is a policy failing to do its own job: a transformation it
	// could not complete, a credential it could not mint, a rewrite it could not apply.
	// The failure is the gateway's, not the caller's.
	FaultCodeMediationFailed = "960000"
	// FaultCodeInvalidRequestBody is a request payload a policy needed to read and could
	// not — malformed JSON, a JSON-RPC envelope that is not one, a body whose shape the
	// policy's configuration does not fit. The failure is the caller's.
	FaultCodeInvalidRequestBody = "960001"
)

// Blocks a policy may allocate in.
//
// Everything outside these is reserved or belongs to this gateway's engine — reuse only.
// Do not pick an unallocated number: it carries no more information than the HTTP status
// beside it, and forecloses the block for whoever needs it later.
const (
	// ShippedPolicyRangeStart..End is for policies WSO2 ships. Allocated per policy.
	ShippedPolicyRangeStart = 960000
	ShippedPolicyRangeEnd   = 965000
	// UserDefinedRangeStart..End is reserved for a deployment's own policies. WSO2 never
	// allocates inside it, so a custom policy's code cannot later collide with a product one.
	//
	// Both blocks sit outside every category range above, on purpose: a condition with no
	// reserved code has no place in that taxonomy, so it categorises as "other" rather than
	// borrowing a label that would mis-report it.
	UserDefinedRangeStart = 965000
	UserDefinedRangeEnd   = 970000
)

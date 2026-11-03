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
	"context"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// The one door into the fault flow.
//
// Every phase handler reaches the fault flow through handleFault and nothing else, handing it
// a fault value describing what happened. No phase handler holds a fault-flow decision of its
// own — it asks its own faultFrom* constructor, which is the only thing that knows what a
// failure looks like in that phase.

// faultOrigin says who produced the failure.
//
// Three values, not the four phase-shaped "triggers" this replaced. The phase a failure was
// noticed in turned out not to be the useful distinction — two of the four triggers meant
// the same thing to everything downstream ("a policy produced this response"), and the
// engine had a second, overlapping vocabulary for the same question (faultSource). What
// actually changes behaviour is who produced it and whether the client already has it.
type faultOrigin string

const (
	// originGateway is a failure the GATEWAY produced: a policy rejected the request or the
	// response, or the engine itself failed (a policy chain that errored, a request body over
	// the decompression ceiling). The response was built here, so provenance needs no
	// inference — this is the gateway's own error by construction.
	originGateway faultOrigin = "gateway"

	// originUpstream is a failure carried by a response nothing short-circuited. It may be
	// the router's (it could not reach the backend, or timed out) or the backend's own
	// error status passing through; only Envoy's code_details can tell those apart, which
	// is why classifyFaultSource exists and why this origin alone consults it.
	originUpstream faultOrigin = "upstream"

	// originStream is a failure raised after part of the response already reached the
	// client. Nothing can be changed; see fault_streaming.go.
	originStream faultOrigin = "stream"
)

// faultAttribution names the policy responsible for a failure, and what that policy said about
// it.
//
// Resolved by the caller from its own result type before handleFault runs, because the four
// chain phases return four unrelated result types and only the phase holds the one it got.
// Empty policyName means no policy caused this failure — an infrastructure error — and is a
// meaningful value rather than a missing one.
type faultAttribution struct {
	policyName string
	version    string
	// phase is the policy.PolicyPhase* the failing policy was executing in. Carried here
	// rather than read from ec.phase at fault time, because ec.phase is maintained for
	// getModeOverride and is NOT current on the bodyless paths — processResponseBodyForEmptyResponse
	// reports a response-body fault while ec.phase still says request_headers.
	phase    string
	declared *policy.FaultDetails
}

// fault is what a phase reports: the one thing that just happened which the fault policies
// may need to act on.
//
// At most one of rejection and outgoing is set. A rejection is a response a policy built to
// end the request early, which the fault chain may still replace — so the caller supplies
// writeBack to receive the result. An outgoing response is one already on its way out, which
// the fault chain mutates in place through the execution result.
type fault struct {
	origin faultOrigin
	attrib faultAttribution

	// rejection is the response a policy short-circuited with; nil when none did.
	rejection *policy.ImmediateResponse
	// writeBack stores the possibly-replaced rejection back into the caller's result. Set
	// whenever rejection is.
	writeBack func(policy.ImmediateResponse)

	// outgoing is the response about to be sent when nothing short-circuited. Only the
	// response-body phase can supply this, because it is the only phase holding a complete
	// response.
	outgoing *executor.ResponseExecutionResult

	// streamErr and declared describe a failure raised mid-stream.
	streamErr error
	declared  *policy.FaultDetails
}

// handleFault is the single entry point, reached only once something has actually happened.
//
// Whether a phase produced a failure is answered by that phase's faultFrom* constructor, so a
// successful request never arrives here. An unguarded call says "this IS a failure" (an engine
// error, a mid-stream interruption); a guarded one says "if it turned out to be".
//
// What is decided HERE is which of the three shapes of failure this is — the same question in
// every phase, which is why it is in one place.
//
// Whether an OUTGOING response is a failure is decided in runFaultPoliciesOnResponse instead,
// because it needs the status floor and Envoy's provenance, which gate the formatter too.
// That is why faultFromResponseBody returns no bool and its call sites are unguarded.
func (ec *PolicyExecutionContext) handleFault(ctx context.Context, f fault) {
	// A mid-stream failure is settled by construction: something failed after the client
	// began receiving the response. Nothing to infer.
	if f.origin == originStream {
		ec.record(f.attrib)
		ec.runFaultPoliciesOnStreamError(ctx, f.streamErr, f.declared)
		return
	}

	if f.rejection != nil {
		// Reached only for a rejection the phase already established is a fault — see the
		// note on the faultFrom* constructors. Nothing left to decide here.
		ec.record(f.attrib)
		out := ec.runFaultPoliciesOnRejection(ctx, *f.rejection)
		if f.writeBack != nil {
			f.writeBack(out)
		}
		return
	}

	if f.outgoing == nil {
		return
	}
	ec.record(f.attrib)
	ec.runFaultPoliciesOnResponse(ctx, f.outgoing, f.origin)
}

// record stores the attribution the phase resolved, so the fault chain and the formatter
// both report the same policy.
//
// A zero attribution is not written through: an infrastructure failure legitimately has no
// policy, and blanking a previously-recorded one would lose it.
func (ec *PolicyExecutionContext) record(a faultAttribution) {
	if a.policyName == "" && a.declared == nil {
		return
	}
	ec.faultPolicyName = a.policyName
	ec.faultPolicyVersion = a.version
	ec.faultDeclared = a.declared
	// PolicyPhase describes the policy named above, so it is recorded only when there IS
	// one. Enforced here rather than trusted to each constructor: a gateway-described
	// failure (a router error, an engine 500) reaches this with a declared error and no
	// policy name, and must name no phase — the same rule FaultContext documents for
	// Policy itself.
	if a.policyName != "" {
		ec.faultPolicyPhase = a.phase
	}
}

// ─── per-phase constructors ─────────────────────────────────────────────────
//
// Each adapts one phase's result type into a fault.
//
// The three rejection constructors are near-identical but cannot be collapsed: their Results
// slices are four different element types with four distinct sealed Action interfaces, and Go
// generics constrain on methods rather than fields, so a shared scan cannot read PolicyName
// off an unconstrained type parameter.

// The faultFrom* constructors answer, for one phase, "did anything happen that the fault flow
// could act on?".
//
// The test is per phase because there is no phase-independent version: for the three rejection
// phases it is "a policy short-circuited with a fault response", while at the response-body
// phase the ABSENCE of a short-circuit is when there is work to do — a pass-through upstream
// error.
//
// The gate therefore lives here, not in handleFault, which sees a fault struct by which point
// the question is answered. handleFault does not re-check, so a caller building a fault
// literal directly inherits no check.
//
// A false means this phase produced nothing the flow acts on — not that the request
// succeeded; a later phase may still produce something.

// isFaultResponse is the gate every phase shares.
//
// An error status is a fault whoever produced it — a policy, the upstream, or the router —
// so a rejection carrying 400 or above needs no declaration to be handled. Below 400 only
// the producing policy can know: a GraphQL error is a 200 with the failure in the body, and
// nothing about the status says so.
//
// Two rules rather than one because each covers what the other cannot: status alone misses
// the 200-that-is-an-error, and IsFault alone misses upstream and router errors, which have
// no policy to declare on their behalf.
//
// TerminateResponseChunk is the exception and keeps IsFault alone: the status went out with
// the headers, and for a streamed response it is almost always 200.
func isFaultResponse(status int, declared bool) bool {
	return status >= faultMinStatus || declared
}

// faultFromRequestHeaders reports a request-header-phase rejection — an authentication or
// authorization failure.
func faultFromRequestHeaders(r *executor.RequestHeaderExecutionResult) (fault, bool) {
	f := fault{origin: originGateway}
	if !r.ShortCircuited {
		return f, false
	}
	imm, ok := r.FinalAction.(policy.ImmediateResponse)
	if !ok {
		return f, false
	}
	f.rejection = &imm
	f.attrib = faultAttribution{phase: policy.PolicyPhaseRequestHeaders, declared: imm.Fault}
	for _, res := range r.Results {
		if res.Skipped || res.Action == nil {
			continue
		}
		if _, isImm := res.Action.(policy.ImmediateResponse); isImm {
			f.attrib.policyName, f.attrib.version = res.PolicyName, res.PolicyVersion
			break
		}
	}
	f.writeBack = func(out policy.ImmediateResponse) { r.FinalAction = out }
	return f, isFaultResponse(imm.StatusCode, imm.IsFault)
}

// faultFromRequestBody reports a request-body-phase rejection — a request guardrail.
func faultFromRequestBody(r *executor.RequestExecutionResult) (fault, bool) {
	f := fault{origin: originGateway}
	if !r.ShortCircuited {
		return f, false
	}
	imm, ok := r.FinalAction.(policy.ImmediateResponse)
	if !ok {
		return f, false
	}
	f.rejection = &imm
	f.attrib = faultAttribution{phase: policy.PolicyPhaseRequestBody, declared: imm.Fault}
	for _, res := range r.Results {
		if res.Skipped || res.Action == nil {
			continue
		}
		if _, isImm := res.Action.(policy.ImmediateResponse); isImm {
			f.attrib.policyName, f.attrib.version = res.PolicyName, res.PolicyVersion
			break
		}
	}
	f.writeBack = func(out policy.ImmediateResponse) { r.FinalAction = out }
	return f, isFaultResponse(imm.StatusCode, imm.IsFault)
}

// faultFromResponseHeaders reports a response-header-phase rejection.
func faultFromResponseHeaders(r *executor.ResponseHeaderExecutionResult) (fault, bool) {
	f := fault{origin: originGateway}
	if !r.ShortCircuited {
		return f, false
	}
	imm, ok := r.FinalAction.(policy.ImmediateResponse)
	if !ok {
		return f, false
	}
	f.rejection = &imm
	f.attrib = faultAttribution{phase: policy.PolicyPhaseResponseHeaders, declared: imm.Fault}
	for _, res := range r.Results {
		if res.Skipped || res.Action == nil {
			continue
		}
		if _, isImm := res.Action.(policy.ImmediateResponse); isImm {
			f.attrib.policyName, f.attrib.version = res.PolicyName, res.PolicyVersion
			break
		}
	}
	f.writeBack = func(out policy.ImmediateResponse) { r.FinalAction = out }
	return f, isFaultResponse(imm.StatusCode, imm.IsFault)
}

// faultFromResponseBody reports whatever the response-body phase produced, which is the one
// phase where all three outcomes are possible:
//
//	a policy replaced the response outright   → a rejection, as in every other phase
//	a policy rejected by changing the status  → an outgoing response of policy origin
//	neither, and the response is an error     → an outgoing response of upstream origin
//
// The middle case is how every response guardrail rejects, and it is invisible to a
// short-circuit test: DownstreamResponseModifications does not stop execution.
//
// Alone among the constructors this returns no bool: the third case makes every response a
// candidate, and the tests that separate a pass-through error from a successful response live
// in runFaultPoliciesOnResponse. Its call sites are therefore unguarded.
func faultFromResponseBody(r *executor.ResponseExecutionResult, statusOverridden bool) fault {
	if r.ShortCircuited {
		f := fault{origin: originGateway, attrib: faultAttributionFromResponses(r.Results, policy.PolicyPhaseResponseBody, immediateResponseAction)}
		if imm, ok := r.FinalAction.(policy.ImmediateResponse); ok && isFaultResponse(imm.StatusCode, imm.IsFault) {
			f.rejection = &imm
			f.writeBack = func(out policy.ImmediateResponse) { r.FinalAction = out }
		}
		return f
	}

	f := fault{outgoing: r, origin: originUpstream}
	if statusOverridden {
		f.origin = originGateway
		f.attrib = faultAttributionFromResponses(r.Results, policy.PolicyPhaseResponseBody, statusOverrideAction)
	}
	return f
}

// faultFromStream reports a failure raised after the client began receiving the response.
func faultFromStream(streamErr error, declared *policy.FaultDetails) fault {
	return fault{origin: originStream, streamErr: streamErr, declared: declared}
}

// faultAttributionFromResponses finds the response-phase result whose action matches, and takes
// both the name and the error from that same result — so the error a fault policy reads
// always belongs to the policy named beside it.
func faultAttributionFromResponses(
	results []executor.ResponsePolicyResult,
	phase string,
	match func(policy.ResponseAction) bool,
) faultAttribution {
	for _, r := range results {
		if r.Skipped || r.Error != nil || r.Action == nil {
			continue
		}
		if match(r.Action) {
			return faultAttribution{
				policyName: r.PolicyName,
				version:    r.PolicyVersion,
				phase:      phase,
				declared:   declaredFaultOf(r.Action),
			}
		}
	}
	return faultAttribution{}
}

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
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/faultformat"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
)

// Fault policies are an ordered, per-API list that runs only when a request is failing.
// Each entry is invoked through policy.FaultPolicy.OnFault.
//
// handleFault in fault_entry.go is the only entry point and decides WHICH failures reach
// here. This file holds what happens after that: building the view an entry receives,
// running the chain, and folding the result back into the response.
//
// Two shapes of failure arrive and need different views:
//
//   - a REJECTION — a policy ended the request early and the engine built the response, so
//     the view is synthesized from the rejection (synthesizeFaultContexts).
//   - an OUTGOING response — nothing short-circuited, so the live response context is reused
//     and an entry mutates the response actually being sent (faultContextsFromResponse).
//
// A failure raised mid-stream is handled in fault_streaming.go, where nothing can change.

// faultMinStatus is the lowest status considered a fault at all.
const faultMinStatus = 400

// runFaultPoliciesOnRejection applies the fault policies to a rejection — an error response the
// engine built itself — and returns the possibly-modified response.
//
// The response is returned unchanged when no fault policies are configured, or when anything
// in the fault chain fails: fault policies must never turn a served error into a dropped
// request. Whether this is a failure at all was settled by the caller.
func (ec *PolicyExecutionContext) runFaultPoliciesOnRejection(
	ctx context.Context,
	immResp policy.ImmediateResponse,
) policy.ImmediateResponse {
	// The producing policy's own body is the first chance to author one — unless it also
	// DESCRIBED the failure, in which case the body is a fallback for a gateway that cannot
	// render one and this gateway renders instead. See faultformat.ShouldFormat.
	if immResp.Body != nil && immResp.Fault == nil {
		// On an OpenAI route the body is reshaped rather than kept: its message becomes the
		// description, which the formatter then renders. See faultformat.OpenAIPolicyBodyMessage.
		if msg, ok := ec.openAIPolicyBodyMessage(immResp.Body); ok {
			immResp.Fault = &policy.FaultDetails{Message: msg}
		} else {
			ec.faultBodyAuthored = true
		}
	}
	// A rejection is the gateway's own error by construction, so provenance is settled.
	ec.faultSource = sourceGateway

	if !ec.faultPoliciesAvailable() {
		// No chain to run — but formatting is a default, not a chain step, so it still
		// applies. This is the case an MCP or SOAP API hits when the operator declared no
		// fault policies at all, which is most of them.
		return ec.formatFaultResponse(ctx, immResp)
	}

	faultCtx := ec.synthesizeFaultContexts(immResp)
	out, ok := ec.executeFaultPolicies(ctx, faultCtx, originGateway, true)
	if !ok {
		return ec.formatFaultResponse(ctx, immResp)
	}

	// Header mutations were applied to the shared header set as the chain ran, appends
	// included; that set is now the rejection's headers.
	immResp.Headers = headersFromView(faultCtx.ResponseHeaders)

	// Any entry that authored a body has decided what the client receives, which switches
	// formatting off. Checked across every result rather than only the last: an entry that
	// wrote a body has authored it even when a later entry ran after it.
	ec.noteAuthoredBody(out.results)

	// Status and body are read off the shared views rather than out of the results, because
	// that is where applyFaultResponse put them as each entry ran.
	if len(out.results) > 0 {
		if faultCtx.ResponseBody != nil && faultCtx.ResponseBody.Present {
			immResp.Body = faultCtx.ResponseBody.Content
		}
		if faultCtx.ResponseStatus > 0 {
			immResp.StatusCode = faultCtx.ResponseStatus
		}
	}
	// This path formats from immResp.Fault rather than faultDeclared, so a re-description
	// is carried across here as well.
	if out.redescribed {
		immResp.Fault = faultCtx.Fault
	}

	// Telemetry is read out of the results rather than the shared views, because unlike the
	// status and body it is not something a later entry observes — it accumulates.
	ec.foldFaultTelemetry(out.results, &immResp)
	return ec.formatFaultResponse(ctx, immResp)
}

// runFaultPoliciesOnResponse applies the fault policies to a response already on its way to
// the client — one nothing short-circuited.
//
// Runs at the response-BODY phase so the error body exists for an entry to read. That phase
// only executes when chain.RequiresResponseBody is set, which is why attaching fault policies
// forces it on (ApplyFaultPoliciesBodyRequirement).
//
// Results are merged into the caller's execResult, not only into the context: the translator
// derives the outgoing ext_proc mutation from execResult.Results.
func (ec *PolicyExecutionContext) runFaultPoliciesOnResponse(
	ctx context.Context,
	execResult *executor.ResponseExecutionResult,
	origin faultOrigin,
) bool {
	if execResult == nil || ec.responseBodyCtx == nil {
		return false
	}
	status := ec.responseBodyCtx.ResponseStatus
	// The same gate the rejection phases use — status OR declaration. Here the declaration
	// arrives as the ORIGIN: originGateway means a policy claimed the response (IsFault
	// already folded in), originUpstream means only the status can speak for it.
	if !isFaultResponse(status, origin == originGateway) || ec.faultPoliciesRan {
		return false
	}

	// Whether the operator's fault POLICIES may run for this failure. originGateway always
	// proceeds; an upstream or router failure needs the deployment to opt in.
	//
	// Gates the CHAIN only, never the formatter: which failures an operator's handlers see
	// is a deployment choice, while which shape an error is rendered in is a protocol fact
	// (see faultformat.supportedKinds).
	chainAllowed := origin == originGateway || ec.handlesUpstreamFaults()

	// Provenance classifies, it does not gate — see error_source.go. Every source reaches
	// the chain, including a backend's own error status, and a deployment that wants only
	// gateway failures narrows with `fault.Source == "gateway"` on the entry.
	//
	// Stored so attributedFault, the formatter and the FaultContext all classify
	// identically without re-deriving.
	src := classifyFaultSource(origin, ec.responseCodeDetails)
	ec.faultSource = src
	if src == sourceRouter || src == sourceNoRoute {
		// Logged, never returned: the raw value names internal proxy mechanics.
		slog.DebugContext(ctx, "Describing a router-produced error from code_details",
			"request_id", ec.requestID, "route_key", ec.routeKey,
			"code_details", ec.responseCodeDetails, "source", string(src))
	}

	// No chain to run is not the end of the story: formatting is a default, so it still
	// applies. Two ways to get here — the operator declared no fault policies, or this is an
	// upstream failure and the deployment has not opted in to handling those — and both want
	// the same answer, because neither says anything about what shape the error must take.
	// This is the path a SOAP or MCP API takes, and the reason formatting had to leave the
	// policy chain.
	if !chainAllowed || !ec.faultPoliciesAvailable() {
		return ec.appendFormattedFaultBody(ctx, execResult, status)
	}

	faultCtx := ec.faultContextsFromResponse(status)
	out, ok := ec.executeFaultPolicies(ctx, faultCtx, origin, false)
	if !ok {
		return ec.appendFormattedFaultBody(ctx, execResult, status)
	}

	// Fault entries return FaultResponse; the translator that reaches the wire reads
	// DownstreamResponseModifications. Converted rather than carried through, so the
	// translator keeps one input type and the fault contract stays out of it.
	//
	// A skipped entry contributes a skipped result, preserved so tracing and metrics still
	// see it — it just has nothing to convert.
	for _, r := range out.results {
		converted := executor.ResponsePolicyResult{
			PolicyName:    r.PolicyName,
			PolicyVersion: r.PolicyVersion,
			Error:         r.Error,
			ExecutionTime: r.ExecutionTime,
			Skipped:       r.Skipped,
		}
		if r.Response != nil {
			converted.Action = faultResponseAsModifications(r.Response)
		}
		execResult.Results = append(execResult.Results, converted)
	}

	// Any entry that authored a body has decided what the client receives, which switches
	// formatting off below. No entry ends the chain, so this sets no ShortCircuited flag.
	ec.noteAuthoredBody(out.results)

	ec.appendFormattedFaultBody(ctx, execResult, status)
	return true
}

// appendFormattedFaultBody synthesizes an error body for the body phase when no policy
// authored one, appending it as a modification result.
//
// It appends rather than mutating responseBodyCtx because the translator derives the
// outgoing ext_proc mutation from execResult.Results — a context-only change would be
// visible to later policies but never reach the client.
func (ec *PolicyExecutionContext) appendFormattedFaultBody(
	ctx context.Context,
	execResult *executor.ResponseExecutionResult,
	status int,
) bool {
	// A short-circuit already replaced the whole response; formatting it here would fight
	// with FinalAction, and the rejection path handles that action's own formatting.
	if execResult.ShortCircuited {
		return false
	}

	// The policy that REJECTED the response wrote its body into the CALLER's results, not
	// into the fault chain's — a response guardrail is the usual case. Missing this let the
	// formatter overwrite a guardrail's own rejection body.
	//
	// Producer rules apply: a guardrail that describes its rejection gets it rendered, and its
	// body is the fallback. A guardrail that describes nothing keeps the body it wrote.
	ec.noteProducerAuthoredBody(execResult.Results)
	ec.noteUpstreamAuthoredBody()

	decision := faultformat.ShouldFormat(errorFormatRegistry,
		ec.faultFormatInput(ec.errorResponseForFormatting(), status,
			ec.faultDeclared != nil && !ec.isEngineFailure()))
	if !decision.Format {
		slog.DebugContext(ctx, "Error formatting skipped",
			"request_id", ec.requestID, "route_key", ec.routeKey, "reason", decision.Reason)
		return false
	}

	slog.DebugContext(ctx, "Error body formatted",
		"request_id", ec.requestID, "route_key", ec.routeKey,
		"reason", decision.Reason, "status", status)

	// Marks the body as authored so a second origin in the same request cannot format
	// again — the formatter is as much an author as a policy is.
	ec.faultBodyAuthored = true

	execResult.Results = append(execResult.Results, executor.ResponsePolicyResult{
		PolicyName: errorFormatResultName,
		Action: policy.DownstreamResponseModifications{
			Body:         decision.Body,
			HeadersToSet: map[string]string{contentTypeHeader: decision.ContentType},
		},
	})
	return true
}

// noteProducerAuthoredBody is noteAuthoredBody for the policy that PRODUCED the failure.
//
// The difference is the description: a producer that wrote a body and described the failure
// authored a fallback, not the response. A producer that described nothing authored the only
// account of the failure there is, so it stands.
//
// Fault ENTRIES are not producers and do not get this treatment — see noteAuthoredBody.
func (ec *PolicyExecutionContext) noteProducerAuthoredBody(results []executor.ResponsePolicyResult) {
	if ec.faultBodyAuthored {
		return
	}
	for _, r := range results {
		if faultformat.BodyAuthored(r.Action) && !faultformat.DescribedError(r.Action) {
			// As on the rejection path: an OpenAI route reshapes the body instead of keeping it.
			// The fault entries have already run here, so a re-description of theirs is the
			// final account of the failure: the body's message only fills a gap in it, and
			// is never a reason to replace it.
			if msg, ok := ec.openAIPolicyBodyMessage(actionBody(r.Action)); ok {
				ec.faultDeclared = withFallbackMessage(ec.faultDeclared, msg)
			} else {
				ec.faultBodyAuthored = true
			}
			return
		}
	}
}

// withFallbackMessage returns declared with msg as its message when it has none, and a new
// description carrying only msg when there is no description at all. Every other field of
// declared is kept. declared itself is never modified: the same pointer is shared with the
// FaultContext the fault entries saw.
func withFallbackMessage(declared *policy.FaultDetails, msg string) *policy.FaultDetails {
	if declared == nil {
		return &policy.FaultDetails{Message: msg}
	}
	if declared.Message != "" || msg == "" {
		return declared
	}
	out := *declared
	out.Message = msg
	return &out
}

// noteUpstreamAuthoredBody records that the BACKEND decided the client's body.
//
// Needed because backend errors now reach this path at all. faultBodyAuthored tracks
// gateway-side authorship, so an upstream error response arrived here with a body and no
// author — and on a formatted kind the formatter would have replaced the backend's own error
// document with gateway house style. That is the one outcome the old provenance gate was
// incidentally preventing, and removing the gate has to bring the rule with it.
//
// A backend cannot mark its body as a fallback the way a policy can, so bytes from upstream
// are a decision and no bytes are no decision. A backend 502 with an empty body is still
// rendered on a formatted kind — except on an OpenAI route, where the provider owns its whole
// error response, including the decision to send no body. An OpenAI route also treats an upstream
// error of unreported provenance as the provider's: rewriting a provider's error document is the
// one outcome this option must never produce, so without proof the router made it, it stands.
func (ec *PolicyExecutionContext) noteUpstreamAuthoredBody() {
	if ec.faultBodyAuthored {
		return
	}
	if ec.faultSource == sourceUnknown && ec.llmOpenAIErrors() {
		ec.faultBodyAuthored = true
		return
	}
	if ec.faultSource != sourceBackend {
		return
	}
	if body := ec.responseBodyCtx; body != nil && body.ResponseBody != nil && len(body.ResponseBody.Content) > 0 {
		ec.faultBodyAuthored = true
	} else if ec.llmOpenAIErrors() {
		ec.faultBodyAuthored = true
	}
}

// noteAuthoredBody records that some policy in these results decided the client's body.
//
// Scanned across every result rather than just a final action, because a policy that mutates
// the body does not short-circuit.
//
// Unconditional, unlike noteProducerAuthoredBody: a fault entry ran AFTER the description and
// with the FaultContext in hand, so a body it writes is the later decision rather than a
// fallback. This is what lets an entry transform the error — wrap it, lift fields out of it —
// without the formatter overwriting the result.
func (ec *PolicyExecutionContext) noteAuthoredBody(results []executor.FaultPolicyResult) {
	if ec.faultBodyAuthored {
		return
	}
	for _, r := range results {
		// A non-nil Body is the decision, including an explicitly empty one — []byte{} means
		// "the client gets nothing", which is as much an authorship claim as any content.
		if r.Response != nil && r.Response.Body != nil {
			ec.faultBodyAuthored = true
			return
		}
	}
}

// errorFormatResultName labels the synthesized result. It is not a policy, and naming it
// like one in traces would imply an entry an operator could remove.
const errorFormatResultName = "__error_format"

// ApplyFaultPoliciesBodyRequirement forces response-body processing on for a chain that
// carries fault policies.
//
// This is load-bearing, not an optimisation. Every origin other than a rejection runs in the
// response-body phase, and processResponseBody only executes policies when
// RequiresResponseBody is set — a flag BuildPolicyChain computes from the route's normal
// policies, which fault policies are deliberately not part of. Without this, attaching a
// fault policies to an API that has no response-body policy of its own would produce a
// sequence that silently never fires for a backend error or an Envoy local reply.
//
// SupportsResponseStreaming is deliberately NOT touched: forcing it off would disable
// streaming for the whole API. A streamed upstream error is buffered per response instead,
// and only when handle_upstream_faults is on — see processResponseHeaders.
func ApplyFaultPoliciesBodyRequirement(chain *registry.PolicyChain) {
	if chain == nil || !chain.HasFaultPolicies {
		return
	}
	chain.RequiresResponseBody = true
}

// responsePolicies returns the policies a response phase should run.
//
// Normally all of them. For a fault the UPSTREAM or the ROUTER produced, none: the fault
// policies handle it instead, and they run at the response-body phase once the error body
// is in hand (runFaultPoliciesOnResponse).
//
// Only reachable with handle_upstream_faults on — with it off upstreamFault is never set, so
// an upstream error keeps running through the response policies as it always has. A policy's
// own rejection never reaches this either: it ends the exchange before any response phase.
func (ec *PolicyExecutionContext) responsePolicies() ([]policy.Policy, []policy.PolicySpec) {
	if ec.upstreamFault {
		return nil, nil
	}
	return ec.policyChain.Policies, ec.policyChain.PolicySpecs
}

// noteUpstreamFault records that this response arrived already failing AND that the
// deployment asked for the fault flow to handle it.
//
// Called at the response-HEADER phase, which is the earliest point the answer is knowable
// and the only one where it is free: a request-phase rejection ends the exchange and never
// reaches a response phase, so any error status seen here came from the upstream or the
// router. Envoy's code_details is already captured by then too, so a router-generated 503 is
// distinguishable from a backend's own without waiting for the body.
//
// The flag is folded in HERE rather than at each reader: with it off upstreamFault never
// becomes true and responsePolicies returns every policy.
func (ec *PolicyExecutionContext) noteUpstreamFault(status int) {
	if status >= faultMinStatus && ec.handlesUpstreamFaults() {
		ec.upstreamFault = true
	}
}

// handlesUpstreamFaults reports whether an upstream- or router-produced failure should reach
// the fault policies rather than being treated as an ordinary response.
//
// Nil-safe on the server, which a hand-built fixture can leave unset. Absent configuration
// reads as disabled, so the opt-in is never granted by omission.
func (ec *PolicyExecutionContext) handlesUpstreamFaults() bool {
	return ec.server != nil && ec.server.handleUpstreamFaults
}

// faultPoliciesAvailable reports whether the fault policies could run at all: one is
// configured and it has not already run for this request.
//
// This is the only question asked here. WHETHER a given response is a failure is decided
// once, in handleFault, before any of this is reached — keeping it out of here is what stops
// the two from drifting apart, which is how a rejection ended up eligible under one test and
// not the other.
func (ec *PolicyExecutionContext) faultPoliciesAvailable() bool {
	if ec.policyChain == nil || !ec.policyChain.HasFaultPolicies {
		return false
	}
	return !ec.faultPoliciesRan
}

// The fault chain's working object is the *policy.FaultContext itself.
//
// An entry reads it, applyFaultResponse mutates it, the next entry sees what the last one
// did, and the caller reads the outcome off the same fields. Nothing is copied, so nothing
// can go stale.

// faultEntry is one fault-policies entry.
//
// There is no per-entry hook selection any more: every entry is invoked through OnFault.
// Previously an entry was dispatched to OnResponseBody or OnResponseHeaders depending on
// which it implemented, so a fault policy was type-identical to a response policy and an
// entry implementing neither was silently never called.
type faultEntry struct {
	pol  policy.Policy
	spec policy.PolicySpec
}

// faultOutcome aggregates what the fault chain produced.
type faultOutcome struct {
	// results holds one entry per fault entry, in order. Every entry runs: nothing a fault
	// policy returns ends the chain, and the caller reads the outcome from the shared views.
	results []executor.FaultPolicyResult
	// redescribed reports that an entry returned a Fault, so the description the chain ends
	// with — faultCtx.Fault — is no longer the producing policy's.
	redescribed bool
}

// applyFaultResponse folds one entry's return into the views the fault chain shares, so the
// next entry observes what this one did.
//
// withAppends says whether HeadersToAppend is applied here too. On the response path it is
// not: those values reach the wire through the translator, and applying them here as well
// would emit them twice. A rejection has no such second route — it is folded into an
// ImmediateResponse from this view alone — so there an append left out here is lost.
//
// Names are lowercased because policy.Headers keys are: a mixed-case name would otherwise sit
// beside the header it meant to replace and both would reach the client.
//
// A returned Fault re-describes the failure: the next entry reads it as FaultContext.Fault,
// and executeFaultPolicies hands the final description to the formatter. It is stored as a
// copy with Policy kept as the producing policy's — Policy names who CAUSED the failure,
// which re-describing it does not change, and it is gateway-owned in any case (see
// attributedFault). Copying also keeps the entry's own struct out of the engine's hands.
func applyFaultResponse(faultCtx *policy.FaultContext, fr *policy.FaultResponse, withAppends bool) {
	if fr == nil {
		return
	}
	if fr.Fault != nil {
		redescribed := *fr.Fault
		redescribed.Policy = faultCtx.Policy
		faultCtx.Fault = &redescribed
	}
	headers := faultCtx.ResponseHeaders.UnsafeInternalValues()
	for key, value := range fr.HeadersToSet {
		headers[strings.ToLower(key)] = []string{value}
	}
	if withAppends {
		for key, values := range fr.HeadersToAppend {
			name := strings.ToLower(key)
			headers[name] = append(headers[name], values...)
		}
	}
	for _, key := range fr.HeadersToRemove {
		delete(headers, strings.ToLower(key))
	}
	if fr.Body != nil {
		faultCtx.ResponseBody = &policy.Body{
			Content:     fr.Body,
			EndOfStream: true,
			Present:     true,
		}
	}
	if fr.StatusCode != nil {
		faultCtx.ResponseStatus = *fr.StatusCode
	}
}

// faultResponseAsModifications maps an entry's return onto the response-action type the
// ext_proc translator reads.
//
// The fault contract is FaultResponse, but the wire is reached by the same translator the
// response path uses, which understands DownstreamResponseModifications. Converting here
// keeps that translator untouched and out of the fault contract's business.
//
// IsFault is deliberately left false: the fault flow is already running, and setting it
// would offer the engine a second, later opportunity to decide this is a fault — which is
// exactly the re-entry faultPoliciesRan exists to prevent.
func faultResponseAsModifications(fr *policy.FaultResponse) policy.DownstreamResponseModifications {
	return policy.DownstreamResponseModifications{
		Body:                  fr.Body,
		StatusCode:            fr.StatusCode,
		Fault:                 fr.Fault,
		HeadersToSet:          fr.HeadersToSet,
		HeadersToAppend:       fr.HeadersToAppend,
		HeadersToRemove:       fr.HeadersToRemove,
		AnalyticsMetadata:     fr.AnalyticsMetadata,
		DynamicMetadata:       fr.DynamicMetadata,
		AnalyticsHeaderFilter: fr.AnalyticsHeaderFilter,
	}
}

// foldFaultTelemetry folds the analytics and dynamic metadata a fault entry produced onto
// the rejection being returned.
//
// Only the REJECTION path needs it: the response path reaches the translator through
// faultResponseAsModifications, which already merges this metadata.
//
// Written to the response AND to the accumulator, because the two translators read different
// places — the request short-circuit seeds from execCtx.analyticsMetadata, the response one
// reads only immResp.AnalyticsMetadata.
//
// AnalyticsHeaderFilter is deliberately NOT folded: which header set it names differs by
// phase, and the fault chain does not carry a phase-specific source for it.
func (ec *PolicyExecutionContext) foldFaultTelemetry(
	results []executor.FaultPolicyResult,
	immResp *policy.ImmediateResponse,
) {
	for _, res := range results {
		if res.Skipped || res.Response == nil {
			continue
		}
		for k, v := range res.Response.AnalyticsMetadata {
			if immResp.AnalyticsMetadata == nil {
				immResp.AnalyticsMetadata = make(map[string]interface{})
			}
			immResp.AnalyticsMetadata[k] = v
			if ec.analyticsMetadata != nil {
				ec.analyticsMetadata[k] = v
			}
		}
		if len(res.Response.DynamicMetadata) > 0 {
			if immResp.DynamicMetadata == nil {
				immResp.DynamicMetadata = make(map[string]map[string]interface{})
			}
			mergeDynamicMetadata(immResp.DynamicMetadata, res.Response.DynamicMetadata)
			if ec.dynamicMetadata != nil {
				mergeDynamicMetadata(ec.dynamicMetadata, res.Response.DynamicMetadata)
			}
		}
	}
}

// faultChanged reports whether an entry asked to change the response at all, as opposed to
// only observing it. Used to warn a handler that tried to change a committed stream.
func faultChanged(fr *policy.FaultResponse) bool {
	if fr == nil {
		return false
	}
	return fr.Body != nil || fr.StatusCode != nil || fr.Fault != nil ||
		len(fr.HeadersToSet) > 0 || len(fr.HeadersToAppend) > 0 || len(fr.HeadersToRemove) > 0
}

// executeFaultPolicies runs the fault chain, invoking every entry through OnFault.
//
// This loop is the only one on the fault path: the executor runs a single entry, so that a
// failing entry can be isolated here. The client is already receiving an error, so one broken
// fault handler must neither fail the request nor suppress the entries after it — which a
// batched call could not offer, since abandoning a batch discards the results of the entries
// that had already run. Each entry still goes through the chain executor, so per-policy CEL
// conditions, metrics, tracing and panic recovery are unchanged.
//
// rejection says the caller folds the outcome back out of faultCtx alone, which is what
// decides whether HeadersToAppend is applied to it — see applyFaultResponse.
//
// Returns ok=false only when nothing was eligible to run.
func (ec *PolicyExecutionContext) executeFaultPolicies(
	ctx context.Context,
	faultCtx *policy.FaultContext,
	origin faultOrigin,
	rejection bool,
) (*faultOutcome, bool) {
	entries := ec.eligibleFaultEntries()
	if len(entries) == 0 {
		return nil, false
	}

	// Mark before executing: a policy inside the fault chain that itself faults must not
	// re-enter, and that must hold even if execution panics and is recovered upstream.
	ec.faultPoliciesRan = true
	ec.stampFaultMetadata(origin, faultCtx.ResponseStatus, faultCtx)

	apiName := ""
	if ec.sharedCtx != nil {
		apiName = ec.sharedCtx.APIName
	}
	hasConditions := ec.policyChain.FaultHasExecutionConditions
	ec.describeFault(faultCtx, origin)

	slog.DebugContext(ctx, "Executing fault policies",
		"request_id", ec.requestID,
		"route_key", ec.routeKey,
		"origin", string(origin),
		"source", string(ec.resolvedFaultSource()),
		"status", faultCtx.ResponseStatus,
		"entry_count", len(entries),
	)

	out := &faultOutcome{}
	for _, entry := range entries {
		res, err := ec.server.executor.ExecuteFaultPolicy(
			ctx, entry.pol, faultCtx, entry.spec, apiName, ec.routeKey, hasConditions,
		)
		if err != nil {
			// Includes the entry not implementing OnFault at all, which the executor
			// reports rather than skipping. Logged per entry and skipped.
			ec.logFaultEntryFailure(ctx, entry.spec, origin, err)
			continue
		}
		out.results = append(out.results, res)

		// Applied here rather than inside the executor: this loop owns the object every entry
		// reads, so applying between entries is what makes a later entry observe an earlier
		// one's changes — and it keeps the executor a runner that mutates nothing.
		//
		// There is nothing to copy back any more. The entry read this object and this object
		// is what gets mutated, so the next entry sees the change by construction rather than
		// by a refresh step that had to be remembered.
		if !res.Skipped {
			applyFaultResponse(faultCtx, res.Response, rejection)
			if res.Response != nil && res.Response.Fault != nil {
				out.redescribed = true
			}
		}
	}

	// The formatter renders from faultDeclared (see attributedFault), so a description an
	// entry supplied has to land there too — otherwise the client's body would describe the
	// failure one way while the analytics event, built from the collector's view of
	// faultCtx.Fault, described it another.
	if out.redescribed {
		ec.faultDeclared = faultCtx.Fault
	}
	return out, true
}

// describeFault adds the fault-specific fields to the context an entry receives: what failed,
// which policy failed, and whether anything returned can still change the response.
//
// Mutates rather than constructs: the response fields are already on the object by the time
// this runs, so building a fresh FaultContext here would mean copying ten of them back off a
// response-shaped view for no gain.
func (ec *PolicyExecutionContext) describeFault(faultCtx *policy.FaultContext, origin faultOrigin) {
	faultCtx.Fault = ec.attributedFault()
	faultCtx.Policy = ec.faultPolicyName
	faultCtx.PolicyVersion = ec.faultPolicyVersion
	faultCtx.PolicyPhase = ec.faultPolicyPhase
	faultCtx.RouteKey = ec.routeKey
	// Mid-stream the client already has the status, headers and earlier chunks, so a handler
	// must be told that changing them is not on offer.
	faultCtx.ResponseCommitted = origin == originStream

	// Resolved here rather than in the builders because it needs a fallback the builders
	// cannot express: a request-phase rejection has no upstream view at all.
	if faultCtx.Upstream == nil {
		faultCtx.Upstream = ec.faultUpstream()
	}

	// Same source as the metadata key: the upstream's status before a policy changed it.
	// Absent for a rejection, where no upstream response ever existed.
	if ec.responseHeaderCtx != nil && ec.responseHeaderCtx.ResponseStatus != faultCtx.ResponseStatus {
		faultCtx.OriginalStatus = ec.responseHeaderCtx.ResponseStatus
	}
}

// requestIdentity returns the authority, scheme and vhost of the request being handled.
//
// Only the two REQUEST-phase contexts carry them, so the source is one of those whatever
// phase failed — a response-phase failure reports the same three values as a request-phase
// one, because they describe the request either way.
//
// The body context is preferred when present: it is the later view, so a request-rewrite
// policy that changed the authority between phases is reflected rather than the pre-rewrite
// value. Falls back to the header context, and to empty strings when neither exists — an
// engine failure before the header phase ran.
func (ec *PolicyExecutionContext) requestIdentity() (authority, scheme, vhost string) {
	if c := ec.requestBodyCtx; c != nil {
		return c.Authority, c.Scheme, c.Vhost
	}
	if c := ec.requestHeaderCtx; c != nil {
		return c.Authority, c.Scheme, c.Vhost
	}
	return "", "", ""
}

// faultUpstream resolves the route's upstream target for the failure being reported.
//
// A response-phase view already holds one and is used as-is. A request-phase rejection never
// reached the upstream, so the target is projected from the request-phase
// UpstreamRequestContext, which carries the same Name, URL and BasePath.
//
// Response is left nil deliberately: synthesising an empty one would let a handler believe
// the backend answered with a zero status.
//
// Later phase wins, since a policy may re-route.
func (ec *PolicyExecutionContext) faultUpstream() *policy.UpstreamResponseContext {
	for _, us := range []*policy.UpstreamRequestContext{
		requestUpstreamOf(ec.requestBodyCtx),
		requestUpstreamOf(ec.requestHeaderCtx),
	} {
		if us == nil {
			continue
		}
		return &policy.UpstreamResponseContext{
			Name:     us.Name,
			URL:      us.URL,
			BasePath: us.BasePath,
		}
	}
	return nil
}

// requestUpstreamOf reads the upstream target off either request-phase context without the
// caller having to nil-check the context first. The two types are unrelated, so this is two
// overloads collapsed into one generic rather than a shared interface.
func requestUpstreamOf[T interface {
	*policy.RequestHeaderContext | *policy.RequestContext
}](ctx T) *policy.UpstreamRequestContext {
	switch c := any(ctx).(type) {
	case *policy.RequestHeaderContext:
		if c == nil {
			return nil
		}
		return c.Upstream
	case *policy.RequestContext:
		if c == nil {
			return nil
		}
		return c.Upstream
	}
	return nil
}

// attributedFault returns the producing policy's declared error with FaultDetails.Policy
// filled in from the chain.
//
// Policy is gateway-owned, so the value a policy supplied is overwritten rather than
// trusted. That has to happen on a COPY: the pointer we hold is the one the producing
// policy put in its own action struct, and writing through it would mutate state the policy
// still owns — a policy holding a package-level FaultDetails it returns for every rejection
// would accumulate attributions from unrelated requests.
func (ec *PolicyExecutionContext) attributedFault() *policy.FaultDetails {
	if ec.faultDeclared == nil {
		// Nothing declared. For a router failure there was no policy to declare anything,
		// so the engine describes it from Envoy's account instead — see routerErrorFor.
		//
		// This is the single resolution point: errorResponseForFormatting delegates here, so
		// the fault chain and the formatter cannot be given different accounts of the same
		// failure.
		return routerErrorFor(ec.resolvedFaultSource(), ec.responseCodeDetails)
	}
	attributed := *ec.faultDeclared
	attributed.Policy = ec.faultPolicyName
	return &attributed
}

// resolvedFaultSource classifies who produced the fault currently being handled, for the origin
// this request reached. Recorded when the fault flow runs so both the chain and the
// formatter classify identically rather than each re-deriving it.
func (ec *PolicyExecutionContext) resolvedFaultSource() faultSource {
	if ec.faultSource == "" {
		return sourceUnknown
	}
	return ec.faultSource
}

func (ec *PolicyExecutionContext) logFaultEntryFailure(
	ctx context.Context,
	spec policy.PolicySpec,
	origin faultOrigin,
	err error,
) {
	slog.ErrorContext(ctx, "Fault policies entry failed; continuing with the remaining entries",
		"request_id", ec.requestID,
		"route_key", ec.routeKey,
		"origin", string(origin),
		"policy", spec.Name,
		"policy_version", spec.Version,
		"error", err,
	)
}

// eligibleFaultEntries resolves the fault entries to run, in declaration order, and the
// hook each will be invoked through.
//
// No filtering and no reversal: whether an error belongs to the fault flow is decided once
// per error, and the authored order is the executed order.
func (ec *PolicyExecutionContext) eligibleFaultEntries() []faultEntry {
	chain := ec.policyChain
	entries := make([]faultEntry, 0, len(chain.FaultPolicies))

	for i, pol := range chain.FaultPolicies {
		var spec policy.PolicySpec
		if i < len(chain.FaultPolicySpecs) {
			spec = chain.FaultPolicySpecs[i]
		}
		entries = append(entries, faultEntry{pol: pol, spec: spec})
	}
	return entries
}

// Fault metadata keys, written into SharedContext.Metadata before the fault chain runs.
//
// Metadata is used rather than new context fields because it is already part of both
// context views and requires no SDK release — policy-engine pins a published sdk/core, so
// a new field would not compile in a container build until that version ships. A policy
// that ignores these keys is unaffected.
const (
	FaultMetaStatus         = "wso2.fault.status"
	FaultMetaOriginalStatus = "wso2.fault.originalStatus"
	FaultMetaPolicy         = "wso2.fault.policy"
	FaultMetaPolicyVersion  = "wso2.fault.policyVersion"

	// FaultMetaRoute is the matched route key, so a fault event can be correlated back
	// to the deployed configuration rather than only to a path.
	FaultMetaRoute = "wso2.fault.route"

	// FaultMetaBodyBytes and FaultMetaContentType describe the error body WITHOUT
	// exposing it. They matter most to a header-hook entry, which cannot see the body at
	// all: it can now branch on "is there a payload, and what kind" without the content.
	// Deliberately size-and-type only — for a guardrail rejection the content is exactly
	// what the guardrail existed to stop from leaving.
	FaultMetaBodyBytes   = "wso2.fault.bodyBytes"
	FaultMetaContentType = "wso2.fault.contentType"
)

// stampFaultMetadata records why the fault chain is running, so a fault policy can report
// "word-count-guardrail rejected this response" rather than only "446 happened".
//
// Without this a notifier has the status and nothing about the cause, which is the single
// most valuable field for the SIEM/event-bus use case this feature exists to serve.
func (ec *PolicyExecutionContext) stampFaultMetadata(origin faultOrigin, status int, faultCtx *policy.FaultContext) {
	if ec.sharedCtx == nil {
		return
	}
	if ec.sharedCtx.Metadata == nil {
		ec.sharedCtx.Metadata = make(map[string]interface{})
	}
	md := ec.sharedCtx.Metadata
	md[FaultMetaStatus] = status

	// The upstream's status before any policy changed it. Absent for a rejection, where no
	// upstream response ever existed.
	if ec.responseHeaderCtx != nil && ec.responseHeaderCtx.ResponseStatus != status {
		md[FaultMetaOriginalStatus] = ec.responseHeaderCtx.ResponseStatus
	}

	// Attribution is only meaningful when a policy caused the fault. For a pass-through the error
	// came from the backend or from Envoy, so these keys are deliberately left unset
	// rather than filled with a misleading value — the origin already says which it was.
	if ec.faultPolicyName != "" {
		md[FaultMetaPolicy] = ec.faultPolicyName
		md[FaultMetaPolicyVersion] = ec.faultPolicyVersion
	}

	if ec.routeKey != "" {
		md[FaultMetaRoute] = ec.routeKey
	}
	ec.stampFaultBodyShape(md, faultCtx)
}

// stampFaultBodyShape describes the error payload without disclosing it.
//
// A header-hook entry has no access to the response body, so without this it cannot tell
// an empty error from a large one. Size and media type are enough to branch on (skip
// wrapping an empty body, treat XML differently from JSON) and carry none of the content.
func (ec *PolicyExecutionContext) stampFaultBodyShape(md map[string]interface{}, faultCtx *policy.FaultContext) {
	if faultCtx == nil {
		return
	}
	if body := faultCtx.ResponseBody; body != nil && body.Present {
		md[FaultMetaBodyBytes] = len(body.Content)
	} else {
		md[FaultMetaBodyBytes] = 0
	}
	if faultCtx.ResponseHeaders != nil {
		if ct := faultCtx.ResponseHeaders.Get("content-type"); len(ct) > 0 {
			md[FaultMetaContentType] = ct[0]
		}
	}
}

// declaredFaultOf extracts a declared Outcome from whichever action carries one.
func declaredFaultOf(action policy.ResponseAction) *policy.FaultDetails {
	switch a := action.(type) {
	case policy.ImmediateResponse:
		return a.Fault
	case policy.DownstreamResponseModifications:
		return a.Fault
	}
	return nil
}

// statusOverrideAction matches the action a response guardrail rejects with.
//
// The declaration is the whole test. It separates a guardrail REJECTING a response from a
// policy merely relabelling the backend's error — both set a status, so the status cannot tell
// them apart. Must stay in step with responseRejectedByPolicy: if the two diverged, the fault
// would be attributed to a different policy than the one that triggered it.
func statusOverrideAction(action policy.ResponseAction) bool {
	mods, ok := action.(policy.DownstreamResponseModifications)
	if !ok {
		return false
	}
	status := 0
	if mods.StatusCode != nil {
		status = *mods.StatusCode
	}
	// A nil StatusCode leaves status 0, which fails the floor — correct, because a policy
	// that set no status rejected nothing and the response's own status governs on the
	// pass-through path instead.
	return isFaultResponse(status, mods.IsFault)
}

// immediateResponseAction matches an outright replacement in the body phase.
func immediateResponseAction(action policy.ResponseAction) bool {
	_, ok := action.(policy.ImmediateResponse)
	return ok
}

// synthesizeFaultContexts builds both views for a rejection — an error the engine built — where
// no upstream response exists. Request-side data is carried over so a fault policy can
// inspect what the caller sent.
//
// The body view carries the error body, which is what allows a body-hook fault policy to
// transform a gateway-generated error rather than only replace it.
func (ec *PolicyExecutionContext) synthesizeFaultContexts(
	immResp policy.ImmediateResponse,
) *policy.FaultContext {
	headerValues := make(map[string][]string, len(immResp.Headers))
	for name, value := range immResp.Headers {
		headerValues[name] = []string{value}
	}

	faultCtx := ec.newFaultErrorContext()
	faultCtx.ResponseStatus = immResp.StatusCode
	faultCtx.ResponseHeaders = policy.NewHeaders(headerValues)
	if len(immResp.Body) > 0 {
		// Only when there is content. An explicitly empty body leaves this nil, so a handler
		// reads "no body" rather than "a body that happens to be empty".
		faultCtx.ResponseBody = &policy.Body{
			Content:     immResp.Body,
			EndOfStream: true,
			Present:     true,
		}
	}
	return faultCtx
}

// newFaultErrorContext seeds the fields every origin shares: the request that produced the
// failure, and its identity. The response side is filled in by the caller, which is the only
// part that differs between a rejection, an outgoing response and a stream.
func (ec *PolicyExecutionContext) newFaultErrorContext() *policy.FaultContext {
	faultCtx := &policy.FaultContext{SharedContext: ec.sharedCtx}
	if ec.requestHeaderCtx != nil {
		faultCtx.RequestHeaders = ec.requestHeaderCtx.Headers
		faultCtx.RequestPath = ec.requestHeaderCtx.Path
		faultCtx.RequestMethod = ec.requestHeaderCtx.Method
		faultCtx.Downstream = ec.requestHeaderCtx.Downstream
	}
	if ec.requestBodyCtx != nil {
		faultCtx.RequestBody = ec.requestBodyCtx.Body
	}
	faultCtx.RequestAuthority, faultCtx.RequestScheme, faultCtx.RequestVhost = ec.requestIdentity()
	// Set here rather than per-origin: every origin has a source, and faultSource() already
	// resolves an unset one to sourceUnknown so the field is never empty on this path.
	faultCtx.Source = string(ec.resolvedFaultSource())
	return faultCtx
}

// faultContextsFromResponse builds the view for an error observed on a real response — one
// already on its way to the client.
//
// The LIVE response context is reused rather than copied, so a later entry reads what an
// earlier one did. No gap-filling from the response-header context is needed for request
// headers, path or downstream: the live context is built with every one of those fields
// populated (see the responseBodyCtx construction in execution_context.go), Upstream
// included.
func (ec *PolicyExecutionContext) faultContextsFromResponse(status int) *policy.FaultContext {
	live := ec.responseBodyCtx
	// The LIVE context's status is updated too, not just the error context's. This is not
	// about the fault view: the response the engine is sending carries the fault status from
	// here on, and anything else reading responseBodyCtx after this point must see it.
	live.ResponseStatus = status

	faultCtx := ec.newFaultErrorContext()
	faultCtx.ResponseStatus = status
	faultCtx.ResponseHeaders = live.ResponseHeaders // by pointer: header mutations reach both
	faultCtx.ResponseBody = live.ResponseBody
	// Prefer the live context's request-side values over the seeded ones. They are the same
	// in practice — both derive from requestHeaderCtx — but the live context is the later
	// view, so a request-rewrite between phases is reflected rather than the original.
	if live.RequestHeaders != nil {
		faultCtx.RequestHeaders = live.RequestHeaders
	}
	if live.RequestBody != nil {
		faultCtx.RequestBody = live.RequestBody
	}
	if live.RequestPath != "" {
		faultCtx.RequestPath = live.RequestPath
		faultCtx.RequestMethod = live.RequestMethod
	}
	if live.Downstream != nil {
		faultCtx.Downstream = live.Downstream
	}
	faultCtx.Upstream = live.Upstream
	return faultCtx
}

// headersFromView rebuilds an ImmediateResponse's flat header map from the fault chain's
// header view once the chain has run.
//
// The view is the whole answer, not an overlay: it was built from every header the rejection
// carried (synthesizeFaultContexts), so starting from the original map instead would bring
// back a header an entry removed.
//
// Repeated values — what HeadersToAppend produces — are joined with ", ", the standard way to
// combine a field that appears more than once. The one header that cannot be combined like
// that is Set-Cookie, so an appended Set-Cookie still reaches the client as a single line.
func headersFromView(headers *policy.Headers) map[string]string {
	all := headers.GetAll()
	out := make(map[string]string, len(all))
	for name, values := range all {
		if len(values) > 0 {
			out[name] = strings.Join(values, ", ")
		}
	}
	return out
}

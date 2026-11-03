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

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Fault policies on the STREAMING response path.
//
// A streamed response commits its status and headers before the first chunk, so two things
// are impossible here and both are protocol facts rather than gaps:
//
//   - The status cannot change. It was sent.
//   - The body cannot be replaced. Envoy DISCARDS an ImmediateResponse once a streamed
//     response is in flight and resets the stream instead.
//
// Everything else a fault policy does — notifying, recording, publishing — works as normal.
// This runs the chain for those side effects and reports the situation through
// FaultContext.ResponseCommitted; a handler that tries to change the response anyway gets a
// warning naming the policy rather than a silently dropped return.

// runFaultPoliciesOnStreamError runs the fault chain for a failure raised mid-stream.
//
// Returns nothing: there is nothing the caller could apply. The response is already the
// client's.
func (ec *PolicyExecutionContext) runFaultPoliciesOnStreamError(
	ctx context.Context,
	streamErr error,
	declared *policy.FaultDetails,
) {
	if !ec.faultPoliciesAvailable() {
		return
	}

	// Mid-stream, the response is committed by definition, so formatting is meaningless. Set
	// this so a second trigger later in the request cannot try to format either.
	ec.faultBodyAuthored = true
	ec.faultSource = sourceGateway
	if declared != nil {
		ec.faultDeclared = declared
	}

	faultCtx := ec.faultContextsForStream()
	if faultCtx == nil {
		return
	}

	out, ok := ec.executeFaultPolicies(ctx, faultCtx, originStream, false)
	if !ok {
		return
	}

	// Warn rather than discard silently. A handler returning modifications here has written
	// code that cannot work, and the only way it finds out is if we say so.
	for _, r := range out.results {
		if r.Skipped {
			continue
		}
		// One predicate covers this, where a ResponseAction would need three to cover the
		// shapes it can arrive in. A FaultResponse either asked to change something or it
		// did not.
		if faultChanged(r.Response) {
			slog.WarnContext(ctx, "Fault policy tried to change an already-committed streaming response; ignored",
				"request_id", ec.requestID, "route_key", ec.routeKey,
				"policy", r.PolicyName, "policy_version", r.PolicyVersion,
				"hint", "the status, headers and earlier chunks are already with the client; check FaultContext.ResponseCommitted")
		}
	}

	slog.DebugContext(ctx, "Fault policies ran for a mid-stream failure",
		"request_id", ec.requestID, "route_key", ec.routeKey,
		"entries", len(out.results), "stream_error", streamErr)
}

// faultContextsForStream builds the context a fault entry is invoked with for a streaming
// failure.
//
// The response body view carries NO content on purpose. Mid-stream there is no complete body to
// hand a handler — chunks have already gone out and the rest never will — and presenting a
// partial buffer as "the error body" would invite a handler to parse something that is neither
// the whole response nor the failure.
func (ec *PolicyExecutionContext) faultContextsForStream() *policy.FaultContext {
	if ec.responseStreamContext == nil && ec.responseHeaderCtx == nil {
		return nil
	}

	faultCtx := ec.newFaultErrorContext()
	if ec.responseHeaderCtx != nil {
		faultCtx.ResponseStatus = ec.responseHeaderCtx.ResponseStatus
		faultCtx.ResponseHeaders = ec.responseHeaderCtx.ResponseHeaders
		faultCtx.Upstream = ec.responseHeaderCtx.Upstream
	}
	faultCtx.ResponseBody = nil // see the doc comment
	return faultCtx
}

// streamTerminationIsFault reports whether a policy's deliberate stream termination should be
// treated as a failure.
//
// Unset means NO. There is no status to infer from — the status went out with the headers — and
// this action is used both for a guardrail intervention and for a clean close after the
// upstream's final event. Guessing "fault" would notify on every successful stream, which is
// worse than missing an undeclared intervention: the first breaks the signal, the second only
// fails to add one. A guardrail must set IsFault: true.
func streamTerminationIsFault(action policy.StreamingResponseAction) (fault bool, declared *policy.FaultDetails) {
	term, ok := action.(policy.TerminateResponseChunk)
	if !ok {
		return false, nil
	}
	if !term.IsFault {
		return false, nil
	}
	return true, term.Fault
}

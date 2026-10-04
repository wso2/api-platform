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

package executor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// FaultPolicyResult is one fault entry's outcome.
//
// Separate from ResponsePolicyResult because the two carry different action types now, and
// sharing the struct would mean a nil-able field that is only ever set on one path — the
// kind of shape that compiles everywhere and is correct in one place.
type FaultPolicyResult struct {
	PolicyName    string
	PolicyVersion string
	// Response is what the entry returned. nil means it changed nothing, which is the
	// ordinary case for a notification or audit entry.
	Response      *policy.FaultResponse
	Error         error
	ExecutionTime time.Duration
	Skipped       bool // true if disabled or the condition evaluated to false
}

// Final reports that this entry ended the fault chain, so the entries after it must not run.
// A method rather than a field on a batch result, because the entry's own return value is
// the whole of the answer and the caller owns the loop it applies to.
func (r FaultPolicyResult) Final() bool {
	return r.Response != nil && r.Response.Final
}

// ExecuteFaultPolicy runs ONE fault entry through the FaultPolicy contract.
//
// Single-entry rather than list-shaped so the caller owns the loop: one broken fault handler
// must neither fail the request nor suppress the entries after it, which a batched call
// cannot offer. Ordering and applying each result before the next entry reads the context are
// the caller's business too; see kernel.executeFaultPolicies.
//
// Separate from ExecuteResponsePolicies because a policy that does not implement FaultPolicy
// is an ERROR here, not a skip: every fault entry was named specifically to run, so skipping
// one silently is the failure this contract exists to prevent.
func (c *ChainExecutor) ExecuteFaultPolicy(
	ctx context.Context,
	pol policy.Policy,
	faultCtx *policy.FaultContext,
	spec policy.PolicySpec,
	api, route string,
	hasExecutionConditions bool,
) (FaultPolicyResult, error) {
	startTime := time.Now()

	_, span := c.tracer.Start(ctx, fmt.Sprintf(constants.SpanPolicyResponseFormat, spec.Name),
		trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	if span.IsRecording() {
		span.SetAttributes(
			attribute.String(constants.AttrPolicyName, spec.Name),
			attribute.String(constants.AttrPolicyVersion, spec.Version),
			attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
		)
	}

	ep, ok := pol.(policy.FaultPolicy)
	if !ok {
		// Surfaced rather than skipped — see the doc comment. Returning an error lets the
		// caller log which entry is misconfigured and move on to the next one without
		// touching the error the client is already receiving.
		return FaultPolicyResult{}, fmt.Errorf("policy %s:%s does not implement OnFault and cannot run on the fault path",
			spec.Name, spec.Version)
	}

	// skipped records one entry's non-execution identically across tracing, metrics and the
	// returned result. Written once because the two skip paths — disabled, and a condition
	// that evaluated false — reported the same four things in the same order, and a third
	// reason added to only one of them is the drift this prevents.
	skipped := func(reason string) (FaultPolicyResult, error) {
		if span.IsRecording() {
			span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
			span.SetAttributes(attribute.String(constants.AttrSkipReason, reason))
		}
		metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", reason).Inc()
		return FaultPolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Skipped:       true,
			ExecutionTime: time.Since(startTime),
		}, nil
	}

	if !spec.Enabled {
		return skipped(constants.AttrSkipReasonDisabled)
	}

	// Evaluated against the FaultContext itself. EvaluateFaultCondition emits the same
	// activation the response-body evaluator does, so an expression means the same thing
	// here as on a response policy — without the chain having to keep a response-shaped
	// copy of the error alive just to make this call.
	if hasExecutionConditions && spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
		conditionMet, err := c.celEvaluator.EvaluateFaultCondition(*spec.ExecutionCondition, faultCtx)
		if err != nil {
			return FaultPolicyResult{}, fmt.Errorf("condition evaluation failed for fault policy %s:%s: %w",
				spec.Name, spec.Version, err)
		}
		if !conditionMet {
			return skipped(constants.AttrSkipReasonConditionNotMet)
		}
	}

	slog.Debug("[fault] calling OnFault", "policy", spec.Name, "version", spec.Version, "route", route)
	response := ep.OnFault(ctx, faultCtx, spec.Parameters.Raw)
	executionTime := time.Since(startTime)

	metrics.PolicyExecutionsTotal.WithLabelValues(spec.Name, spec.Version, api, route, "executed").Inc()
	metrics.PolicyDurationSeconds.WithLabelValues(spec.Name, spec.Version, api, route).Observe(executionTime.Seconds())
	if span.IsRecording() {
		span.SetAttributes(attribute.Int64(constants.AttrPolicyExecutionTimeNS, executionTime.Nanoseconds()))
	}

	result := FaultPolicyResult{
		PolicyName:    spec.Name,
		PolicyVersion: spec.Version,
		Response:      response,
		ExecutionTime: executionTime,
		Skipped:       false,
	}

	// Applying the response is the CALLER's job, not this function's. The caller holds the
	// live view every entry reads, so it can apply between entries and keep the "a later
	// entry sees what an earlier one did" semantics — while this function stays a runner
	// that mutates nothing.
	if result.Final() {
		if span.IsRecording() {
			span.SetAttributes(attribute.Bool(constants.AttrPolicyShortCircuit, true))
		}
		metrics.ShortCircuitsTotal.WithLabelValues("", spec.Name).Inc()
	}

	return result, nil
}

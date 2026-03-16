/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"encoding/json"
	"fmt"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/utils"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RequestPolicyResult represents the result of executing a single request policy
type RequestPolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        policy.RequestAction
	ExecutionTime time.Duration
	Skipped       bool // true if condition evaluated to false
}

// RequestExecutionResult represents the result of executing all request policies in a chain
type RequestExecutionResult struct {
	Results            []RequestPolicyResult
	ShortCircuited     bool                 // true if chain stopped early due to ImmediateResponse
	FinalAction        policy.RequestAction // Final action to apply
	TotalExecutionTime time.Duration
}

// ResponsePolicyResult represents the result of executing a single response policy
type ResponsePolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        policy.ResponseAction
	Error         error
	ExecutionTime time.Duration
	Skipped       bool // true if condition evaluated to false
}

// ResponseExecutionResult represents the result of executing all response policies in a chain
type ResponseExecutionResult struct {
	Results            []ResponsePolicyResult
	ShortCircuited     bool                  // true if chain stopped early due to ImmediateResponse
	FinalAction        policy.ResponseAction // Final action to apply
	TotalExecutionTime time.Duration
}

// ExecuteRequestPolicies executes request policies with condition evaluation
// T043: Implements execution with condition evaluation and short-circuit logic
// hasExecutionConditions indicates if any policy in the chain has CEL conditions; when false, CEL evaluation is skipped entirely
func (c *ChainExecutor) ExecuteRequestPolicies(traceCtx context.Context, policyList []policy.Policy, ctx *policy.RequestContext, specs []policy.PolicySpec, api, route string, hasExecutionConditions bool) (*RequestExecutionResult, error) {
	startTime := time.Now()
	result := &RequestExecutionResult{
		Results:        make([]RequestPolicyResult, 0, len(policyList)),
		ShortCircuited: false,
	}

	// Execute each policy in order
	for i, pol := range policyList {
		spec := specs[i]
		policyStartTime := time.Now()

		// Create span for individual policy execution - NoOp if tracing disabled
		_, span := c.tracer.Start(traceCtx, fmt.Sprintf(constants.SpanPolicyRequestFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))

		// Add policy metadata attributes
		if span.IsRecording() {
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
		}

		// Check if policy is enabled
		if !spec.Enabled {
			if span.IsRecording() {
				span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
			}
			metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "disabled").Inc()
			span.End()
			result.Results = append(result.Results, RequestPolicyResult{
				PolicyName:    spec.Name,
				PolicyVersion: spec.Version,
				Skipped:       true,
				ExecutionTime: time.Since(policyStartTime),
			})
			continue
		}

		// Evaluate execution condition if present and if chain has any CEL conditions
		// Skip this block entirely when no policies in the chain have CEL conditions
		if hasExecutionConditions && spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			if c.celEvaluator != nil {
				conditionMet, err := c.celEvaluator.EvaluateRequestCondition(*spec.ExecutionCondition, ctx)
				if err != nil {
					if span.IsRecording() {
						span.RecordError(err)
						span.SetStatus(codes.Error, "condition evaluation failed")
					}
					span.End()
					return nil, fmt.Errorf("condition evaluation failed for policy %s:%s: %w", spec.Name, spec.Version, err)
				}
				if !conditionMet {
					// Condition not met - skip policy
					if span.IsRecording() {
						span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
						span.SetAttributes(attribute.String(constants.AttrSkipReason, constants.AttrSkipReasonConditionNotMet))
					}
					metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "condition_not_met").Inc()
					span.End()
					result.Results = append(result.Results, RequestPolicyResult{
						PolicyName:    spec.Name,
						PolicyVersion: spec.Version,
						Skipped:       true,
						ExecutionTime: time.Since(policyStartTime),
					})
					continue
				}
			}
		}

		// Deep-copy params to prevent a policy from mutating the shared spec map
		// across concurrent requests (nested maps/slices require a full deep copy).
		params, err := deepCopyParams(spec.Parameters.Raw)
		if err != nil {
			span.End()
			return nil, fmt.Errorf("failed to clone parameters for policy %s:%s: %w", spec.Name, spec.Version, err)
		}

		// Execute policy
		action := normalizeRequestAction(pol.OnRequest(ctx, params))
		executionTime := time.Since(policyStartTime)

		// Record policy execution metrics
		metrics.PolicyExecutionsTotal.WithLabelValues(spec.Name, spec.Version, api, route, "executed").Inc()
		metrics.PolicyDurationSeconds.WithLabelValues(spec.Name, spec.Version, api, route).Observe(executionTime.Seconds())

		// Add execution time attribute
		if span.IsRecording() {
			span.SetAttributes(attribute.Int64(constants.AttrPolicyExecutionTimeNS, executionTime.Nanoseconds()))
		}

		policyResult := RequestPolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        action,
			ExecutionTime: executionTime,
			Skipped:       false,
		}

		result.Results = append(result.Results, policyResult)

		// Apply action if present
		if action != nil {
			// Check for short-circuit (T047)
			if action.StopExecution() {
				if span.IsRecording() {
					span.SetAttributes(attribute.Bool(constants.AttrPolicyShortCircuit, true))
				}
				metrics.ShortCircuitsTotal.WithLabelValues("", spec.Name).Inc()
				result.ShortCircuited = true
				result.FinalAction = action
				span.End()
				break
			}

			// Apply modifications to context (T045)
			if mods, ok := action.(*policy.UpstreamRequestModifications); ok {
				applyRequestModifications(ctx, mods)
			}
		}

		span.End()
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// ExecuteResponsePolicies executes response policies with condition evaluation
// T044: Implements execution with condition evaluation
// hasExecutionConditions indicates if any policy in the chain has CEL conditions; when false, CEL evaluation is skipped entirely
func (c *ChainExecutor) ExecuteResponsePolicies(traceCtx context.Context, policyList []policy.Policy, ctx *policy.ResponseContext, specs []policy.PolicySpec, api, route string, hasExecutionConditions bool) (*ResponseExecutionResult, error) {
	startTime := time.Now()
	result := &ResponseExecutionResult{
		Results: make([]ResponsePolicyResult, 0, len(policyList)),
	}

	// Execute each policy in reverse order (last to first)
	// This allows policies to "unwrap" in the reverse order they "wrapped" the request
	for i := len(policyList) - 1; i >= 0; i-- {
		pol := policyList[i]
		spec := specs[i]
		policyStartTime := time.Now()

		// Create span for individual policy execution - NoOp if tracing disabled
		_, span := c.tracer.Start(traceCtx, fmt.Sprintf(constants.SpanPolicyResponseFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))

		// Add policy metadata attributes
		if span.IsRecording() {
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
		}

		// Check if policy is enabled
		if !spec.Enabled {
			if span.IsRecording() {
				span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
			}
			metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "disabled").Inc()
			span.End()
			result.Results = append(result.Results, ResponsePolicyResult{
				PolicyName:    spec.Name,
				PolicyVersion: spec.Version,
				Skipped:       true,
				ExecutionTime: time.Since(policyStartTime),
			})
			continue
		}

		// Evaluate execution condition if present and if chain has any CEL conditions
		// Skip this block entirely when no policies in the chain have CEL conditions
		if hasExecutionConditions && spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			if c.celEvaluator != nil {
				conditionMet, err := c.celEvaluator.EvaluateResponseCondition(*spec.ExecutionCondition, ctx)
				if err != nil {
					if span.IsRecording() {
						span.RecordError(err)
						span.SetStatus(codes.Error, "condition evaluation failed")
					}
					span.End()
					return nil, fmt.Errorf("condition evaluation failed for policy %s:%s: %w", spec.Name, spec.Version, err)
				}
				if !conditionMet {
					// Condition not met - skip policy
					if span.IsRecording() {
						span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
						span.SetAttributes(attribute.String(constants.AttrSkipReason, constants.AttrSkipReasonConditionNotMet))
					}
					metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "condition_not_met").Inc()
					span.End()
					result.Results = append(result.Results, ResponsePolicyResult{
						PolicyName:    spec.Name,
						PolicyVersion: spec.Version,
						Skipped:       true,
						ExecutionTime: time.Since(policyStartTime),
					})
					continue
				}
			}
		}

		// Deep-copy params to prevent a policy from mutating the shared spec map
		// across concurrent requests (nested maps/slices require a full deep copy).
		params, err := deepCopyParams(spec.Parameters.Raw)
		if err != nil {
			span.End()
			return nil, fmt.Errorf("failed to clone parameters for policy %s:%s: %w", spec.Name, spec.Version, err)
		}

		// Execute policy
		action := normalizeResponseAction(pol.OnResponse(ctx, params))
		executionTime := time.Since(policyStartTime)

		// Record policy execution metrics
		metrics.PolicyExecutionsTotal.WithLabelValues(spec.Name, spec.Version, api, route, "executed").Inc()
		metrics.PolicyDurationSeconds.WithLabelValues(spec.Name, spec.Version, api, route).Observe(executionTime.Seconds())

		// Add execution time attribute
		if span.IsRecording() {
			span.SetAttributes(attribute.Int64(constants.AttrPolicyExecutionTimeNS, executionTime.Nanoseconds()))
		}

		policyResult := ResponsePolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        action,
			ExecutionTime: executionTime,
			Skipped:       false,
		}

		result.Results = append(result.Results, policyResult)

		// Apply action if present (T046)
		if action != nil {
			// Check for short-circuit
			if action.StopExecution() {
				if span.IsRecording() {
					span.SetAttributes(attribute.Bool(constants.AttrPolicyShortCircuit, true))
				}
				metrics.ShortCircuitsTotal.WithLabelValues("", spec.Name).Inc()
				result.ShortCircuited = true
				result.FinalAction = action
				span.End()
				break
			}

			if mods, ok := action.(*policy.DownstreamResponseModifications); ok {
				applyResponseModifications(ctx, mods)
			}
		}

		span.End()
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// normalizeRequestAction converts legacy value-type actions (returned by older policies)
// to their pointer equivalents so that all downstream type assertions use the pointer form.
func normalizeRequestAction(action policy.RequestAction) policy.RequestAction {
	switch a := action.(type) {
	case policy.UpstreamRequestModifications:
		return &a
	case policy.ImmediateResponse:
		return &a
	}
	return action
}

// normalizeResponseAction converts legacy value-type actions to pointer form.
func normalizeResponseAction(action policy.ResponseAction) policy.ResponseAction {
	switch a := action.(type) {
	case policy.DownstreamResponseModifications:
		return &a
	case policy.ImmediateResponse:
		return &a
	}
	return action
}

// applyRequestModifications applies request modifications to context
// T045: Implements request context modification
func applyRequestModifications(ctx *policy.RequestContext, mods *policy.UpstreamRequestModifications) {
	// Get direct access to headers for mutation (kernel-only API)
	headers := ctx.Headers.UnsafeInternalValues()

	// Set headers (replace existing) — deprecated flat field
	if mods.SetHeaders != nil {
		for key, value := range mods.SetHeaders {
			headers[key] = []string{value}
		}
	}

	// Remove headers — deprecated flat field
	if mods.RemoveHeaders != nil {
		for _, key := range mods.RemoveHeaders {
			delete(headers, key)
		}
	}

	// Append headers — deprecated flat field
	if mods.AppendHeaders != nil {
		for key, values := range mods.AppendHeaders {
			existing := headers[key]
			headers[key] = append(existing, values...)
		}
	}

	// Update body (nil = no change, []byte{} = clear)
	if mods.Body != nil {
		ctx.Body = &policy.Body{
			Content:     mods.Body,
			EndOfStream: true, // Modifications are always complete
			Present:     true,
		}
	}

	// Update path
	if mods.Path != nil {
		ctx.Path = *mods.Path
	}

	// Add query parameters
	if mods.AddQueryParameters != nil {
		ctx.Path = utils.AddQueryParametersToPath(ctx.Path, mods.AddQueryParameters)
	}

	// Remove query parameters
	if mods.RemoveQueryParameters != nil {
		ctx.Path = utils.RemoveQueryParametersFromPath(ctx.Path, mods.RemoveQueryParameters)
	}
	
	// Update method
	if mods.Method != nil {
		ctx.Method = *mods.Method
	}
}

// applyResponseModifications applies response modifications to context
// T046: Implements response context modification
func applyResponseModifications(ctx *policy.ResponseContext, mods *policy.DownstreamResponseModifications) {
	// Get direct access to response headers for mutation (kernel-only API)
	headers := ctx.ResponseHeaders.UnsafeInternalValues()

	// Set headers (replace existing) — deprecated flat field
	if mods.SetHeaders != nil {
		for key, value := range mods.SetHeaders {
			headers[key] = []string{value}
		}
	}

	// Remove headers — deprecated flat field
	if mods.RemoveHeaders != nil {
		for _, key := range mods.RemoveHeaders {
			delete(headers, key)
		}
	}

	// Append headers — deprecated flat field
	if mods.AppendHeaders != nil {
		for key, values := range mods.AppendHeaders {
			existing := headers[key]
			headers[key] = append(existing, values...)
		}
	}

	// Update body (nil = no change, []byte{} = clear)
	if mods.Body != nil {
		ctx.ResponseBody = &policy.Body{
			Content:     mods.Body,
			EndOfStream: true, // Modifications are always complete
			Present:     true,
		}
	}

	// Update status code
	if mods.StatusCode != nil {
		ctx.ResponseStatus = *mods.StatusCode
	}
}

// ChainExecutor represents the policy chain execution engine
// T048: Added CEL evaluator for condition evaluation and metrics collection
type ChainExecutor struct {
	registry     *registry.PolicyRegistry
	celEvaluator CELEvaluator
	tracer       trace.Tracer
}

// CELEvaluator interface for condition evaluation
type CELEvaluator interface {
	EvaluateRequestCondition(expression string, ctx *policy.RequestContext) (bool, error)
	EvaluateResponseCondition(expression string, ctx *policy.ResponseContext) (bool, error)
}

// NewChainExecutor creates a new ChainExecutor execution engine
func NewChainExecutor(reg *registry.PolicyRegistry, celEvaluator CELEvaluator, tracer trace.Tracer) *ChainExecutor {
	return &ChainExecutor{
		registry:     reg,
		celEvaluator: celEvaluator,
		tracer:       tracer,
	}
}

// deepCopyParams returns a deep copy of a map[string]interface{} via a JSON round-trip.
//
// A shallow copy (e.g. maps.Clone) is not sufficient because params can contain nested
// maps or slices — if a policy mutates a nested value, the change would bleed into
// subsequent concurrent requests that share the same PolicySpec.Parameters.Raw map.
//
// Trade-off: the JSON marshal/unmarshal adds a small per-request allocation cost.
// This is acceptable given that params are typically small config objects. If profiling
// shows this is a hot path, consider a lazy copy-on-write approach at the policy level.
func deepCopyParams(src map[string]interface{}) (map[string]interface{}, error) {
	if len(src) == 0 {
		return make(map[string]interface{}), nil
	}
	b, err := json.Marshal(src)
	if err != nil {
		return nil, err
	}
	var dst map[string]interface{}
	if err := json.Unmarshal(b, &dst); err != nil {
		return nil, err
	}
	return dst, nil
}

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
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ─── Request header phase ─────────────────────────────────────────────────────

// RequestHeaderPolicyResult is the result of executing a single RequestHeaderPolicy
type RequestHeaderPolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        policy.RequestHeaderAction
	ExecutionTime time.Duration
	Skipped       bool // true if condition evaluated to false
}

// RequestHeaderExecutionResult aggregates per-policy results for the request-headers phase
type RequestHeaderExecutionResult struct {
	Results            []RequestHeaderPolicyResult
	ShortCircuited     bool                       // true if chain stopped early due to ImmediateResponse
	FinalAction        policy.RequestHeaderAction // Final action to apply
	TotalExecutionTime time.Duration
}

// ExecuteRequestHeaderPolicies invokes each RequestHeaderPolicy in the chain.
// Policies that do not implement RequestHeaderPolicy are skipped silently.
func (c *ChainExecutor) ExecuteRequestHeaderPolicies(
	ctx context.Context,
	policyList []policy.Policy,
	reqCtx *policy.RequestHeaderContext,
	specs []policy.PolicySpec,
	api, route string,
	hasExecutionConditions bool,
) (*RequestHeaderExecutionResult, error) {
	startTime := time.Now()
	result := &RequestHeaderExecutionResult{
		Results:        make([]RequestHeaderPolicyResult, 0, len(policyList)),
		ShortCircuited: false,
	}

	for i, pol := range policyList {
		spec := specs[i]
		policyStartTime := time.Now()

		_, span := c.tracer.Start(ctx, fmt.Sprintf(constants.SpanPolicyRequestFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))
		if span.IsRecording() {
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
		}

		headerPol, ok := pol.(policy.RequestHeaderPolicy)
		if !ok {
			span.End()
			continue
		}
		if pol.Mode().RequestHeaderMode != policy.HeaderModeProcess {
			span.End()
			continue
		}

		if !spec.Enabled {
			if span.IsRecording() {
				span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
			}
			metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "disabled").Inc()
			span.End()
			result.Results = append(result.Results, RequestHeaderPolicyResult{
				PolicyName:    spec.Name,
				PolicyVersion: spec.Version,
				Skipped:       true,
				ExecutionTime: time.Since(policyStartTime),
			})
			continue
		}

		// Evaluate execution condition if present and if chain has any CEL conditions
		if hasExecutionConditions && spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			if c.celEvaluator != nil {
				conditionMet, err := c.celEvaluator.EvaluateRequestHeaderCondition(*spec.ExecutionCondition, reqCtx)
				if err != nil {
					if span.IsRecording() {
						span.RecordError(err)
						span.SetStatus(codes.Error, "condition evaluation failed")
					}
					span.End()
					return nil, fmt.Errorf("condition evaluation failed for policy %s:%s: %w", spec.Name, spec.Version, err)
				}
				if !conditionMet {
					if span.IsRecording() {
						span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
						span.SetAttributes(attribute.String(constants.AttrSkipReason, constants.AttrSkipReasonConditionNotMet))
					}
					metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "condition_not_met").Inc()
					span.End()
					result.Results = append(result.Results, RequestHeaderPolicyResult{
						PolicyName:    spec.Name,
						PolicyVersion: spec.Version,
						Skipped:       true,
						ExecutionTime: time.Since(policyStartTime),
					})
					continue
				}
			}
		}

		params, err := deepCopyParams(spec.Parameters.Raw)
		if err != nil {
			span.End()
			return nil, fmt.Errorf("failed to clone parameters for policy %s:%s: %w", spec.Name, spec.Version, err)
		}

		action := headerPol.OnRequestHeaders(ctx, reqCtx, params)
		executionTime := time.Since(policyStartTime)

		// Apply header mutations to reqCtx so subsequent policies and CEL conditions see the mutated state
		if mod, ok := action.(policy.UpstreamRequestHeaderModifications); ok {
			internalHeaders := reqCtx.Headers.UnsafeInternalValues()
			for k, v := range mod.HeadersToSet {
				internalHeaders[strings.ToLower(k)] = []string{v}
			}
			for _, k := range mod.HeadersToRemove {
				delete(internalHeaders, strings.ToLower(k))
			}
			if mod.Path != nil {
				reqCtx.Path = *mod.Path
			}
			if mod.Method != nil {
				reqCtx.Method = *mod.Method
			}
		}

		metrics.PolicyExecutionsTotal.WithLabelValues(spec.Name, spec.Version, api, route, "executed").Inc()
		metrics.PolicyDurationSeconds.WithLabelValues(spec.Name, spec.Version, api, route).Observe(executionTime.Seconds())

		if span.IsRecording() {
			span.SetAttributes(attribute.Int64(constants.AttrPolicyExecutionTimeNS, executionTime.Nanoseconds()))
		}

		result.Results = append(result.Results, RequestHeaderPolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        action,
			ExecutionTime: executionTime,
		})

		if _, ok := action.(policy.ImmediateResponse); ok {
			if span.IsRecording() {
				span.SetAttributes(attribute.Bool(constants.AttrPolicyShortCircuit, true))
			}
			metrics.ShortCircuitsTotal.WithLabelValues("", spec.Name).Inc()
			result.ShortCircuited = true
			result.FinalAction = action
			span.End()
			break
		}

		result.FinalAction = action
		span.End()
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// ─── Request body phase ───────────────────────────────────────────────────────

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

// ExecuteRequestPolicies executes request policies with condition evaluation
// hasExecutionConditions indicates if any policy in the chain has CEL conditions; when false, CEL evaluation is skipped entirely
func (c *ChainExecutor) ExecuteRequestPolicies(ctx context.Context, policyList []policy.Policy, reqCtx *policy.RequestContext, specs []policy.PolicySpec, api, route string, hasExecutionConditions bool) (*RequestExecutionResult, error) {
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
		_, span := c.tracer.Start(ctx, fmt.Sprintf(constants.SpanPolicyRequestFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))

		// Add policy metadata attributes
		if span.IsRecording() {
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
		}

		// Execute policy via RequestPolicy sub-interface
		rp, ok := pol.(policy.RequestPolicy)
		if !ok {
			span.End()
			continue
		}

		// Skip if the policy's mode says to skip request body processing
		if pol.Mode().RequestBodyMode == policy.BodyModeSkip {
			span.End()
			continue
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
				conditionMet, err := c.celEvaluator.EvaluateRequestBodyCondition(*spec.ExecutionCondition, reqCtx)
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

		slog.Debug("[body] calling OnRequestBody", "policy", spec.Name, "version", spec.Version, "route", route)
		action := rp.OnRequestBody(ctx, reqCtx, params)
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

			// Apply modifications to context
			if mods, ok := action.(policy.UpstreamRequestModifications); ok {
				applyRequestModifications(reqCtx, &mods)
			}
		}

		span.End()
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// ─── Response header phase ────────────────────────────────────────────────────

// ResponseHeaderPolicyResult is the result of executing a single ResponseHeaderPolicy
type ResponseHeaderPolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        policy.ResponseHeaderAction
	ExecutionTime time.Duration
	Skipped       bool
}

// ResponseHeaderExecutionResult aggregates per-policy results for the response-headers phase
type ResponseHeaderExecutionResult struct {
	Results            []ResponseHeaderPolicyResult
	ShortCircuited     bool                        // true if chain stopped early due to ImmediateResponse
	FinalAction        policy.ResponseHeaderAction // Final action to apply
	TotalExecutionTime time.Duration
}

// ExecuteResponseHeaderPolicies invokes each ResponseHeaderPolicy in the chain (reverse order).
// Policies that do not implement ResponseHeaderPolicy are skipped silently.
func (c *ChainExecutor) ExecuteResponseHeaderPolicies(
	ctx context.Context,
	policyList []policy.Policy,
	respCtx *policy.ResponseHeaderContext,
	specs []policy.PolicySpec,
	api, route string,
	hasExecutionConditions bool,
) (*ResponseHeaderExecutionResult, error) {
	startTime := time.Now()
	result := &ResponseHeaderExecutionResult{
		Results: make([]ResponseHeaderPolicyResult, 0, len(policyList)),
	}

	for i := len(policyList) - 1; i >= 0; i-- {
		pol := policyList[i]
		spec := specs[i]
		policyStartTime := time.Now()

		_, span := c.tracer.Start(ctx, fmt.Sprintf(constants.SpanPolicyResponseFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))
		if span.IsRecording() {
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
		}

		headerPol, ok := pol.(policy.ResponseHeaderPolicy)
		if !ok {
			span.End()
			continue
		}
		if pol.Mode().ResponseHeaderMode != policy.HeaderModeProcess {
			span.End()
			continue
		}

		if !spec.Enabled {
			if span.IsRecording() {
				span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
			}
			metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "disabled").Inc()
			span.End()
			result.Results = append(result.Results, ResponseHeaderPolicyResult{
				PolicyName: spec.Name, PolicyVersion: spec.Version,
				Skipped: true, ExecutionTime: time.Since(policyStartTime),
			})
			continue
		}

		if hasExecutionConditions && spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			if c.celEvaluator != nil {
				conditionMet, err := c.celEvaluator.EvaluateResponseHeaderCondition(*spec.ExecutionCondition, respCtx)
				if err != nil {
					span.End()
					return nil, fmt.Errorf("condition evaluation failed for policy %s:%s: %w", spec.Name, spec.Version, err)
				}
				if !conditionMet {
					if span.IsRecording() {
						span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
						span.SetAttributes(attribute.String(constants.AttrSkipReason, constants.AttrSkipReasonConditionNotMet))
					}
					metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "condition_not_met").Inc()
					span.End()
					result.Results = append(result.Results, ResponseHeaderPolicyResult{
						PolicyName:    spec.Name,
						PolicyVersion: spec.Version,
						Skipped:       true,
						ExecutionTime: time.Since(policyStartTime),
					})
					continue
				}
			}
		}

		params, err := deepCopyParams(spec.Parameters.Raw)
		if err != nil {
			span.End()
			return nil, fmt.Errorf("failed to clone parameters for policy %s:%s: %w", spec.Name, spec.Version, err)
		}

		action := headerPol.OnResponseHeaders(ctx, respCtx, params)
		executionTime := time.Since(policyStartTime)

		// Apply header mutations to respCtx so subsequent policies and CEL conditions see the mutated state
		if mod, ok := action.(policy.DownstreamResponseHeaderModifications); ok {
			internalHeaders := respCtx.ResponseHeaders.UnsafeInternalValues()
			for k, v := range mod.HeadersToSet {
				internalHeaders[strings.ToLower(k)] = []string{v}
			}
			for _, k := range mod.HeadersToRemove {
				delete(internalHeaders, strings.ToLower(k))
			}
		}

		metrics.PolicyExecutionsTotal.WithLabelValues(spec.Name, spec.Version, api, route, "executed").Inc()
		metrics.PolicyDurationSeconds.WithLabelValues(spec.Name, spec.Version, api, route).Observe(executionTime.Seconds())

		if span.IsRecording() {
			span.SetAttributes(attribute.Int64(constants.AttrPolicyExecutionTimeNS, executionTime.Nanoseconds()))
		}

		result.Results = append(result.Results, ResponseHeaderPolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        action,
			ExecutionTime: executionTime,
		})

		if _, ok := action.(policy.ImmediateResponse); ok {
			if span.IsRecording() {
				span.SetAttributes(attribute.Bool(constants.AttrPolicyShortCircuit, true))
			}
			metrics.ShortCircuitsTotal.WithLabelValues("", spec.Name).Inc()
			result.ShortCircuited = true
			result.FinalAction = action
			span.End()
			break
		}

		result.FinalAction = action
		span.End()
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// ─── Response body phase ──────────────────────────────────────────────────────

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

// ExecuteResponsePolicies executes response policies with condition evaluation
// hasExecutionConditions indicates if any policy in the chain has CEL conditions; when false, CEL evaluation is skipped entirely
func (c *ChainExecutor) ExecuteResponsePolicies(ctx context.Context, policyList []policy.Policy, respCtx *policy.ResponseContext, specs []policy.PolicySpec, api, route string, hasExecutionConditions bool) (*ResponseExecutionResult, error) {
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
		_, span := c.tracer.Start(ctx, fmt.Sprintf(constants.SpanPolicyResponseFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))

		// Add policy metadata attributes
		if span.IsRecording() {
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
		}

		// Execute policy via ResponsePolicy sub-interface
		rp, ok := pol.(policy.ResponsePolicy)
		if !ok {
			span.End()
			continue
		}

		// Skip if the policy's mode says to skip response body processing
		if pol.Mode().ResponseBodyMode == policy.BodyModeSkip {
			span.End()
			continue
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
				conditionMet, err := c.celEvaluator.EvaluateResponseBodyCondition(*spec.ExecutionCondition, respCtx)
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

		slog.Debug("[body] calling OnResponseBody", "policy", spec.Name, "version", spec.Version, "route", route)
		action := rp.OnResponseBody(ctx, respCtx, params)
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

		// Apply action if present
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

			if mods, ok := action.(policy.DownstreamResponseModifications); ok {
				applyResponseModifications(respCtx, &mods)
			}
		}

		span.End()
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// ─── Streaming request body phase ────────────────────────────────────────────

// StreamingRequestPolicyResult represents the result of executing a single streaming request policy
type StreamingRequestPolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        policy.StreamingRequestAction
	ExecutionTime time.Duration
	Skipped       bool
}

// StreamingRequestExecutionResult represents the result of executing all streaming request policies
type StreamingRequestExecutionResult struct {
	Results            []StreamingRequestPolicyResult
	FinalAction        policy.StreamingRequestAction
	FinalChunk         *policy.StreamBody
	TotalExecutionTime time.Duration
}

// ExecuteStreamingRequestPolicies executes streaming request policies chunk-by-chunk.
// Policies are executed in forward order (first to last). Each policy sees the
// (possibly mutated) chunk from the previous policy.
func (c *ChainExecutor) ExecuteStreamingRequestPolicies(
	ctx context.Context,
	policyList []policy.Policy,
	reqCtx *policy.RequestStreamContext,
	chunk *policy.StreamBody,
	specs []policy.PolicySpec,
	api, route string,
	hasExecutionConditions bool,
) (*StreamingRequestExecutionResult, error) {
	startTime := time.Now()
	result := &StreamingRequestExecutionResult{
		Results: make([]StreamingRequestPolicyResult, 0, len(policyList)),
	}

	currentChunk := chunk

	for i := 0; i < len(policyList); i++ {
		pol := policyList[i]
		spec := specs[i]
		policyStartTime := time.Now()

		_, span := c.tracer.Start(ctx, fmt.Sprintf(constants.SpanPolicyRequestFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))
		if span.IsRecording() {
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
		}

		streamingPol, ok := pol.(policy.StreamingRequestPolicy)
		if !ok {
			span.End()
			continue
		}

		// Mode()-first: only execute streaming callbacks for policies that explicitly opt into STREAM.
		if pol.Mode().RequestBodyMode != policy.BodyModeStream {
			span.End()
			continue
		}

		if !spec.Enabled {
			metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "disabled").Inc()
			span.End()
			result.Results = append(result.Results, StreamingRequestPolicyResult{
				PolicyName: spec.Name, PolicyVersion: spec.Version,
				Skipped: true, ExecutionTime: time.Since(policyStartTime),
			})
			continue
		}

		if hasExecutionConditions && spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			if c.celEvaluator != nil {
				conditionMet, err := c.celEvaluator.EvaluateStreamingRequestCondition(*spec.ExecutionCondition, reqCtx)
				if err != nil {
					if span.IsRecording() {
						span.RecordError(err)
						span.SetStatus(codes.Error, err.Error())
					}
					span.End()
					return nil, fmt.Errorf("condition evaluation failed for policy %s:%s: %w", spec.Name, spec.Version, err)
				}
				if !conditionMet {
					if span.IsRecording() {
						span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
						span.SetAttributes(attribute.String(constants.AttrSkipReason, constants.AttrSkipReasonConditionNotMet))
					}
					metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "condition_not_met").Inc()
					span.End()
					result.Results = append(result.Results, StreamingRequestPolicyResult{
						PolicyName: spec.Name, PolicyVersion: spec.Version,
						Skipped: true, ExecutionTime: time.Since(policyStartTime),
					})
					continue
				}
			}
		}

		params, err := deepCopyParams(spec.Parameters.Raw)
		if err != nil {
			span.End()
			return nil, fmt.Errorf("failed to clone parameters for policy %s:%s: %w", spec.Name, spec.Version, err)
		}

		slog.Debug("[streaming] calling OnRequestBodyChunk", "policy", spec.Name, "version", spec.Version, "route", route, "end_of_stream", currentChunk.EndOfStream)
		action := streamingPol.OnRequestBodyChunk(ctx, reqCtx, currentChunk, params)
		executionTime := time.Since(policyStartTime)

		metrics.PolicyExecutionsTotal.WithLabelValues(spec.Name, spec.Version, api, route, "executed").Inc()
		metrics.PolicyDurationSeconds.WithLabelValues(spec.Name, spec.Version, api, route).Observe(executionTime.Seconds())

		if span.IsRecording() {
			span.SetAttributes(attribute.Int64(constants.AttrPolicyExecutionTimeNS, executionTime.Nanoseconds()))
		}

		result.Results = append(result.Results, StreamingRequestPolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        action,
			ExecutionTime: executionTime,
		})

		// Chain the chunk: if a policy mutates the body, downstream policies see the mutated bytes.
		if fwd, ok := action.(policy.ForwardRequestChunk); ok && fwd.Body != nil {
			currentChunk = &policy.StreamBody{
				Chunk:       fwd.Body,
				EndOfStream: currentChunk.EndOfStream,
			}
		}

		result.FinalAction = action
		span.End()
	}

	result.FinalChunk = currentChunk
	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// ─── Streaming response body phase ───────────────────────────────────────────

// StreamingResponsePolicyResult represents the result of executing a single streaming response policy
type StreamingResponsePolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        policy.StreamingResponseAction
	ExecutionTime time.Duration
	Skipped       bool
}

// StreamingResponseExecutionResult represents the result of executing all streaming response policies
type StreamingResponseExecutionResult struct {
	Results            []StreamingResponsePolicyResult
	StreamTerminated   bool // true if a policy returned TerminateResponseChunk and the chain was stopped early
	FinalAction        policy.StreamingResponseAction
	FinalChunk         *policy.StreamBody
	TotalExecutionTime time.Duration
}

// ExecuteStreamingResponsePolicies executes streaming response policies chunk-by-chunk.
// Policies are executed in reverse order (last to first), mirroring the buffered response path.
func (c *ChainExecutor) ExecuteStreamingResponsePolicies(
	ctx context.Context,
	policyList []policy.Policy,
	respCtx *policy.ResponseStreamContext,
	chunk *policy.StreamBody,
	specs []policy.PolicySpec,
	api, route string,
	hasExecutionConditions bool,
) (*StreamingResponseExecutionResult, error) {
	startTime := time.Now()
	result := &StreamingResponseExecutionResult{
		Results: make([]StreamingResponsePolicyResult, 0, len(policyList)),
	}

	currentChunk := chunk

	for i := len(policyList) - 1; i >= 0; i-- {
		pol := policyList[i]
		spec := specs[i]
		policyStartTime := time.Now()

		_, span := c.tracer.Start(ctx, fmt.Sprintf(constants.SpanPolicyResponseFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))
		if span.IsRecording() {
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
		}

		streamingPol, ok := pol.(policy.StreamingResponsePolicy)
		if !ok {
			span.End()
			continue
		}

		// Mode()-first: only execute streaming callbacks for policies that explicitly opt into STREAM.
		if pol.Mode().ResponseBodyMode != policy.BodyModeStream {
			span.End()
			continue
		}

		if !spec.Enabled {
			if span.IsRecording() {
				span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
			}
			metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "disabled").Inc()
			span.End()
			result.Results = append(result.Results, StreamingResponsePolicyResult{
				PolicyName: spec.Name, PolicyVersion: spec.Version,
				Skipped: true, ExecutionTime: time.Since(policyStartTime),
			})
			continue
		}

		if hasExecutionConditions && spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			if c.celEvaluator != nil {
				conditionMet, err := c.celEvaluator.EvaluateStreamingResponseCondition(*spec.ExecutionCondition, respCtx)
				if err != nil {
					if span.IsRecording() {
						span.RecordError(err)
						span.SetStatus(codes.Error, "condition evaluation failed")
					}
					span.End()
					return nil, fmt.Errorf("condition evaluation failed for policy %s:%s: %w", spec.Name, spec.Version, err)
				}
				if !conditionMet {
					if span.IsRecording() {
						span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
						span.SetAttributes(attribute.String(constants.AttrSkipReason, constants.AttrSkipReasonConditionNotMet))
					}
					metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "condition_not_met").Inc()
					span.End()
					result.Results = append(result.Results, StreamingResponsePolicyResult{
						PolicyName:    spec.Name,
						PolicyVersion: spec.Version,
						Skipped:       true,
						ExecutionTime: time.Since(policyStartTime),
					})
					continue
				}
			}
		}

		params, err := deepCopyParams(spec.Parameters.Raw)
		if err != nil {
			span.End()
			return nil, fmt.Errorf("failed to clone parameters for policy %s:%s: %w", spec.Name, spec.Version, err)
		}

		slog.Debug("[streaming] calling OnResponseBodyChunk", "policy", spec.Name, "version", spec.Version, "route", route, "end_of_stream", currentChunk.EndOfStream)
		action := streamingPol.OnResponseBodyChunk(ctx, respCtx, currentChunk, params)
		executionTime := time.Since(policyStartTime)

		metrics.PolicyExecutionsTotal.WithLabelValues(spec.Name, spec.Version, api, route, "executed").Inc()
		metrics.PolicyDurationSeconds.WithLabelValues(spec.Name, spec.Version, api, route).Observe(executionTime.Seconds())

		if span.IsRecording() {
			span.SetAttributes(attribute.Int64(constants.AttrPolicyExecutionTimeNS, executionTime.Nanoseconds()))
		}

		result.Results = append(result.Results, StreamingResponsePolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        action,
			ExecutionTime: executionTime,
		})

		// Propagate modified bytes to the next policy in the chain.
		switch a := action.(type) {
		case policy.ForwardResponseChunk:
			if a.Body != nil {
				currentChunk = &policy.StreamBody{Chunk: a.Body, EndOfStream: currentChunk.EndOfStream}
			}
		case policy.TerminateResponseChunk:
			if a.Body != nil {
				currentChunk = &policy.StreamBody{Chunk: a.Body, EndOfStream: true}
			}
		}

		result.FinalAction = action
		span.End()

		// Short-circuit: a policy returned TerminateResponseChunk (e.g. guardrail intervened).
		// Stop executing remaining policies and signal the kernel to close the stream after
		// delivering the current chunk.
		if action.TerminateStream() {
			slog.Info("[streaming] policy requested stream termination; stopping chain",
				"policy", spec.Name,
				"version", spec.Version,
				"route", route,
			)
			result.StreamTerminated = true
			break
		}
	}

	result.FinalChunk = currentChunk
	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// applyRequestModifications applies request modifications to context
func applyRequestModifications(ctx *policy.RequestContext, mods *policy.UpstreamRequestModifications) {
	headers := ctx.Headers.UnsafeInternalValues()

	if mods.HeadersToSet != nil {
		for key, value := range mods.HeadersToSet {
			headers[key] = []string{value}
		}
	}

	if mods.HeadersToRemove != nil {
		for _, key := range mods.HeadersToRemove {
			delete(headers, key)
		}
	}

	if mods.Body != nil {
		ctx.Body = &policy.Body{
			Content:     mods.Body,
			EndOfStream: true,
			Present:     true,
		}
	}

	if mods.Path != nil {
		ctx.Path = *mods.Path
	}

	if mods.QueryParametersToAdd != nil {
		ctx.Path = utils.AddQueryParametersToPath(ctx.Path, mods.QueryParametersToAdd)
	}

	if mods.QueryParametersToRemove != nil {
		ctx.Path = utils.RemoveQueryParametersFromPath(ctx.Path, mods.QueryParametersToRemove)
	}

	if mods.Method != nil {
		ctx.Method = *mods.Method
	}
}

// applyResponseModifications applies response modifications to context
func applyResponseModifications(ctx *policy.ResponseContext, mods *policy.DownstreamResponseModifications) {
	headers := ctx.ResponseHeaders.UnsafeInternalValues()

	if mods.HeadersToSet != nil {
		for key, value := range mods.HeadersToSet {
			headers[key] = []string{value}
		}
	}

	if mods.HeadersToRemove != nil {
		for _, key := range mods.HeadersToRemove {
			delete(headers, key)
		}
	}

	if mods.Body != nil {
		ctx.ResponseBody = &policy.Body{
			Content:     mods.Body,
			EndOfStream: true,
			Present:     true,
		}
	}

	if mods.StatusCode != nil {
		ctx.ResponseStatus = *mods.StatusCode
	}
}

// deepCopyParams returns a deep copy of a map[string]interface{} via a JSON round-trip.
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

// ─── ChainExecutor ────────────────────────────────────────────────────────────

// ChainExecutor represents the policy chain execution engine
type ChainExecutor struct {
	registry     *registry.PolicyRegistry
	celEvaluator CELEvaluator
	tracer       trace.Tracer
}

// CELEvaluator interface for condition evaluation
type CELEvaluator interface {
	EvaluateRequestHeaderCondition(expression string, ctx *policy.RequestHeaderContext) (bool, error)
	EvaluateRequestBodyCondition(expression string, ctx *policy.RequestContext) (bool, error)
	EvaluateResponseHeaderCondition(expression string, ctx *policy.ResponseHeaderContext) (bool, error)
	EvaluateResponseBodyCondition(expression string, ctx *policy.ResponseContext) (bool, error)
	EvaluateStreamingRequestCondition(expression string, ctx *policy.RequestStreamContext) (bool, error)
	EvaluateStreamingResponseCondition(expression string, ctx *policy.ResponseStreamContext) (bool, error)
}

// NewChainExecutor creates a new ChainExecutor execution engine
func NewChainExecutor(reg *registry.PolicyRegistry, celEvaluator CELEvaluator, tracer trace.Tracer) *ChainExecutor {
	return &ChainExecutor{
		registry:     reg,
		celEvaluator: celEvaluator,
		tracer:       tracer,
	}
}

// GetCELEvaluator returns the CEL evaluator used for condition evaluation.
func (c *ChainExecutor) GetCELEvaluator() CELEvaluator {
	return c.celEvaluator
}

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
	"fmt"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ─── Request header phase ─────────────────────────────────────────────────────

// RequestHeaderPolicyResult is the result of executing a single RequestHeaderPolicy
type RequestHeaderPolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        *policy.RequestHeaderAction
	ExecutionTime time.Duration
	Skipped       bool // true if condition evaluated to false
}

// RequestHeaderExecutionResult aggregates per-policy results for the request-headers phase
type RequestHeaderExecutionResult struct {
	Results            []RequestHeaderPolicyResult
	ShortCircuited     bool                        // true if chain stopped early due to ImmediateResponse
	FinalAction        *policy.RequestHeaderAction // Final action to apply
	TotalExecutionTime time.Duration
}

// ExecuteRequestHeaderPolicies invokes each RequestHeaderPolicy in the chain.
// Policies that do not implement RequestHeaderPolicy are skipped silently.
func (c *ChainExecutor) ExecuteRequestHeaderPolicies(
	traceCtx context.Context,
	policyList []policy.Policy,
	ctx *policy.RequestHeaderContext,
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

		_, span := c.tracer.Start(traceCtx, fmt.Sprintf(constants.SpanPolicyRequestFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))
		if span.IsRecording() {

			// Add policy metadata attributes
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
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
		// Skip this block entirely when no policies in the chain have CEL conditions
		if hasExecutionConditions && spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			if c.celEvaluator != nil {
				conditionMet, err := c.celEvaluator.EvaluateRequestHeaderCondition(*spec.ExecutionCondition, ctx)
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

		headerPol, ok := pol.(policy.RequestHeaderPolicy)
		if !ok {
			span.End()
			continue
		}

		action := headerPol.OnRequestHeaders(ctx)
		executionTime := time.Since(policyStartTime)

		metrics.PolicyExecutionsTotal.WithLabelValues(spec.Name, spec.Version, api, route, "executed").Inc()
		metrics.PolicyDurationSeconds.WithLabelValues(spec.Name, spec.Version, api, route).Observe(executionTime.Seconds())

		if span.IsRecording() {
			span.SetAttributes(attribute.Int64(constants.AttrPolicyExecutionTimeNS, executionTime.Nanoseconds()))
		}

		result.Results = append(result.Results, RequestHeaderPolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        &action,
			ExecutionTime: executionTime,
		})

		if action.ImmediateResponse != nil {
			if span.IsRecording() {
				span.SetAttributes(attribute.Bool(constants.AttrPolicyShortCircuit, true))
			}
			metrics.ShortCircuitsTotal.WithLabelValues("", spec.Name).Inc()
			result.ShortCircuited = true
			result.FinalAction = &action
			span.End()
			break
		}

		result.FinalAction = &action
		span.End()
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// ─── Request body phase ───────────────────────────────────────────────────────

// RequestBodyPolicyResult is the result of executing a single RequestBodyPolicy
type RequestBodyPolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        *policy.RequestAction
	ExecutionTime time.Duration
	Skipped       bool
}

// RequestBodyExecutionResult aggregates per-policy results for the request-body phase
type RequestBodyExecutionResult struct {
	Results            []RequestBodyPolicyResult
	ShortCircuited     bool
	FinalAction        *policy.RequestAction
	TotalExecutionTime time.Duration
}

// ExecuteRequestBodyPolicies invokes each RequestBodyPolicy in the chain.
// Policies that do not implement RequestBodyPolicy are skipped silently.
func (c *ChainExecutor) ExecuteRequestBodyPolicies(
	traceCtx context.Context,
	policyList []policy.Policy,
	ctx *policy.RequestContext,
	specs []policy.PolicySpec,
	api, route string,
	hasExecutionConditions bool,
) (*RequestBodyExecutionResult, error) {
	startTime := time.Now()
	result := &RequestBodyExecutionResult{
		Results:        make([]RequestBodyPolicyResult, 0, len(policyList)),
		ShortCircuited: false,
	}

	for i, pol := range policyList {
		spec := specs[i]
		policyStartTime := time.Now()

		_, span := c.tracer.Start(traceCtx, fmt.Sprintf(constants.SpanPolicyRequestFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))
		if span.IsRecording() {
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
		}

		if !spec.Enabled {
			if span.IsRecording() {
				span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
			}
			metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "disabled").Inc()
			span.End()
			result.Results = append(result.Results, RequestBodyPolicyResult{
				PolicyName:    spec.Name,
				PolicyVersion: spec.Version,
				Skipped:       true,
				ExecutionTime: time.Since(policyStartTime),
			})
			continue
		}

		if hasExecutionConditions && spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			if c.celEvaluator != nil {
				conditionMet, err := c.celEvaluator.EvaluateRequestBodyCondition(*spec.ExecutionCondition, ctx)
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
					result.Results = append(result.Results, RequestBodyPolicyResult{
						PolicyName:    spec.Name,
						PolicyVersion: spec.Version,
						Skipped:       true,
						ExecutionTime: time.Since(policyStartTime),
					})
					continue
				}
			}
		}

		bodyPol, ok := pol.(policy.RequestPolicy)
		if !ok {
			span.End()
			continue
		}

		action := bodyPol.OnRequestBody(ctx)
		executionTime := time.Since(policyStartTime)

		metrics.PolicyExecutionsTotal.WithLabelValues(spec.Name, spec.Version, api, route, "executed").Inc()
		metrics.PolicyDurationSeconds.WithLabelValues(spec.Name, spec.Version, api, route).Observe(executionTime.Seconds())

		if span.IsRecording() {
			span.SetAttributes(attribute.Int64(constants.AttrPolicyExecutionTimeNS, executionTime.Nanoseconds()))
		}

		result.Results = append(result.Results, RequestBodyPolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        &action,
			ExecutionTime: executionTime,
		})

		if action.ImmediateResponse != nil {
			if span.IsRecording() {
				span.SetAttributes(attribute.Bool(constants.AttrPolicyShortCircuit, true))
			}
			metrics.ShortCircuitsTotal.WithLabelValues("", spec.Name).Inc()
			result.ShortCircuited = true
			result.FinalAction = &action
			span.End()
			break
		}

		// Apply body modifications so the next policy sees the updated context
		if action.BodyMutation != nil {
			ctx.Body = &policy.Body{
				Content:     action.BodyMutation,
				EndOfStream: true,
				Present:     true,
			}
		}
		if action.PathMutation != nil {
			ctx.Path = *action.PathMutation
		}
		if action.MethodMutation != nil {
			ctx.Method = *action.MethodMutation
		}

		result.FinalAction = &action
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
	Action        *policy.ResponseHeaderAction
	ExecutionTime time.Duration
	Skipped       bool
}

// ResponseHeaderExecutionResult aggregates per-policy results for the response-headers phase
type ResponseHeaderExecutionResult struct {
	Results            []ResponseHeaderPolicyResult
	TotalExecutionTime time.Duration
}

// ExecuteResponseHeaderPolicies invokes each ResponseHeaderPolicy in the chain (reverse order).
// Policies that do not implement ResponseHeaderPolicy are skipped silently.
func (c *ChainExecutor) ExecuteResponseHeaderPolicies(
	traceCtx context.Context,
	policyList []policy.Policy,
	ctx *policy.ResponseHeaderContext,
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

		_, span := c.tracer.Start(traceCtx, fmt.Sprintf(constants.SpanPolicyResponseFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))
		if span.IsRecording() {
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
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
				conditionMet, err := c.celEvaluator.EvaluateResponseHeaderCondition(*spec.ExecutionCondition, ctx)
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

		headerPol, ok := pol.(policy.ResponseHeaderPolicy)
		if !ok {
			span.End()
			continue
		}

		action := headerPol.OnResponseHeaders(ctx)
		executionTime := time.Since(policyStartTime)

		metrics.PolicyExecutionsTotal.WithLabelValues(spec.Name, spec.Version, api, route, "executed").Inc()
		metrics.PolicyDurationSeconds.WithLabelValues(spec.Name, spec.Version, api, route).Observe(executionTime.Seconds())

		if span.IsRecording() {
			span.SetAttributes(attribute.Int64(constants.AttrPolicyExecutionTimeNS, executionTime.Nanoseconds()))
		}

		result.Results = append(result.Results, ResponseHeaderPolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        &action,
			ExecutionTime: executionTime,
		})
		span.End()
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// ─── Response body phase ──────────────────────────────────────────────────────

// ResponseBodyPolicyResult is the result of executing a single ResponseBodyPolicy
type ResponseBodyPolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        *policy.ResponseAction
	ExecutionTime time.Duration
	Skipped       bool
}

// ResponseBodyExecutionResult aggregates per-policy results for the response-body phase
type ResponseBodyExecutionResult struct {
	Results            []ResponseBodyPolicyResult
	ShortCircuited     bool
	FinalAction        *policy.ResponseAction
	TotalExecutionTime time.Duration
}

// ExecuteResponseBodyPolicies invokes each ResponseBodyPolicy in the chain (reverse order).
// Policies that do not implement ResponseBodyPolicy are skipped silently.
func (c *ChainExecutor) ExecuteResponseBodyPolicies(
	traceCtx context.Context,
	policyList []policy.Policy,
	ctx *policy.ResponseContext,
	specs []policy.PolicySpec,
	api, route string,
	hasExecutionConditions bool,
) (*ResponseBodyExecutionResult, error) {
	startTime := time.Now()
	result := &ResponseBodyExecutionResult{
		Results: make([]ResponseBodyPolicyResult, 0, len(policyList)),
	}

	for i := len(policyList) - 1; i >= 0; i-- {
		pol := policyList[i]
		spec := specs[i]
		policyStartTime := time.Now()

		_, span := c.tracer.Start(traceCtx, fmt.Sprintf(constants.SpanPolicyResponseFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))
		if span.IsRecording() {
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
		}

		if !spec.Enabled {
			if span.IsRecording() {
				span.SetAttributes(attribute.Bool(constants.AttrPolicySkipped, true))
			}
			metrics.PolicySkippedTotal.WithLabelValues(spec.Name, "", "", "disabled").Inc()
			span.End()
			result.Results = append(result.Results, ResponseBodyPolicyResult{
				PolicyName: spec.Name, PolicyVersion: spec.Version,
				Skipped: true, ExecutionTime: time.Since(policyStartTime),
			})
			continue
		}

		if hasExecutionConditions && spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			if c.celEvaluator != nil {
				conditionMet, err := c.celEvaluator.EvaluateResponseBodyCondition(*spec.ExecutionCondition, ctx)
				if err != nil {
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
					result.Results = append(result.Results, ResponseBodyPolicyResult{
						PolicyName:    spec.Name,
						PolicyVersion: spec.Version,
						Skipped:       true,
						ExecutionTime: time.Since(policyStartTime),
					})
					continue
				}
			}
		}

		bodyPol, ok := pol.(policy.ResponsePolicy)
		if !ok {
			span.End()
			continue
		}

		action := bodyPol.OnResponseBody(ctx)
		executionTime := time.Since(policyStartTime)

		metrics.PolicyExecutionsTotal.WithLabelValues(spec.Name, spec.Version, api, route, "executed").Inc()
		metrics.PolicyDurationSeconds.WithLabelValues(spec.Name, spec.Version, api, route).Observe(executionTime.Seconds())

		if span.IsRecording() {
			span.SetAttributes(attribute.Int64(constants.AttrPolicyExecutionTimeNS, executionTime.Nanoseconds()))
		}

		result.Results = append(result.Results, ResponseBodyPolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        &action,
			ExecutionTime: executionTime,
		})

		if action.ImmediateResponse != nil {
			if span.IsRecording() {
				span.SetAttributes(attribute.Bool(constants.AttrPolicyShortCircuit, true))
			}
			metrics.ShortCircuitsTotal.WithLabelValues("", spec.Name).Inc()
			result.ShortCircuited = true
			result.FinalAction = &action
			span.End()
			break
		}

		// Propagate body modification to the next policy in the chain
		if action.BodyMutation != nil {
			ctx.ResponseBody = &policy.Body{
				Content:     action.BodyMutation,
				EndOfStream: true,
				Present:     true,
			}
		}

		result.FinalAction = &action
		span.End()
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// ─── Streaming request body phase ────────────────────────────────────────────

// StreamingRequestPolicyResult holds the outcome of invoking a single
// StreamingRequestPolicy on one chunk.
type StreamingRequestPolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        *policy.RequestChunkAction
	ExecutionTime time.Duration
	Skipped       bool
}

// StreamingRequestExecutionResult aggregates per-policy results from a single
// streaming request body chunk invocation.
type StreamingRequestExecutionResult struct {
	Results            []StreamingRequestPolicyResult
	FinalAction        *policy.RequestChunkAction
	TotalExecutionTime time.Duration
}

// ExecuteStreamingRequestPolicies invokes each StreamingRequestPolicy in the
// chain (forward order) for a single body chunk. Policies that do not implement
// StreamingRequestPolicy are skipped silently — chain compatibility is enforced
// at chain-build time.
func (c *ChainExecutor) ExecuteStreamingRequestPolicies(
	traceCtx context.Context,
	policyList []policy.Policy,
	ctx *policy.RequestStreamContext,
	chunk *policy.StreamBody,
	specs []policy.PolicySpec,
	api, route string,
	hasExecutionConditions bool,
) (*StreamingRequestExecutionResult, error) {
	startTime := time.Now()
	result := &StreamingRequestExecutionResult{
		Results: make([]StreamingRequestPolicyResult, 0, len(policyList)),
	}

	// Track the current chunk body so each policy sees the previous policy's output.
	currentChunk := chunk

	for i := 0; i < len(policyList); i++ {
		pol := policyList[i]
		spec := specs[i]
		policyStartTime := time.Now()

		_, span := c.tracer.Start(traceCtx, fmt.Sprintf(constants.SpanPolicyRequestFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))
		if span.IsRecording() {
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
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
				conditionMet, err := c.celEvaluator.EvaluateStreamingRequestCondition(*spec.ExecutionCondition, ctx)
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

		streamingPol, ok := pol.(policy.StreamingRequestPolicy)
		if !ok {
			span.End()
			continue
		}

		action := streamingPol.OnRequestBodyChunk(ctx, currentChunk)
		executionTime := time.Since(policyStartTime)

		metrics.PolicyExecutionsTotal.WithLabelValues(spec.Name, spec.Version, api, route, "executed").Inc()
		metrics.PolicyDurationSeconds.WithLabelValues(spec.Name, spec.Version, api, route).Observe(executionTime.Seconds())

		if span.IsRecording() {
			span.SetAttributes(attribute.Int64(constants.AttrPolicyExecutionTimeNS, executionTime.Nanoseconds()))
		}

		result.Results = append(result.Results, StreamingRequestPolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        &action,
			ExecutionTime: executionTime,
		})

		// Chain the chunk: if a policy mutates the body, downstream policies see the mutated bytes.
		if action.BodyMutation != nil {
			currentChunk = &policy.StreamBody{
				Chunk:       action.BodyMutation,
				EndOfStream: currentChunk.EndOfStream,
			}
		}

		result.FinalAction = &action
		span.End()
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// ─── Streaming response body phase ───────────────────────────────────────────

// StreamingResponsePolicyResult holds the outcome of invoking a single
// StreamingResponseBodyPolicy on one chunk.
type StreamingResponsePolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        *policy.ResponseChunkAction
	ExecutionTime time.Duration
	Skipped       bool
}

// StreamingResponseExecutionResult aggregates per-policy results from a single
// streaming response body chunk invocation.
type StreamingResponseExecutionResult struct {
	Results            []StreamingResponsePolicyResult
	FinalAction        *policy.ResponseChunkAction
	TotalExecutionTime time.Duration
}

// ExecuteStreamingResponsePolicies invokes each StreamingResponseBodyPolicy in the
// chain (reverse order) for a single body chunk. Policies that do not implement
// StreamingResponseBodyPolicy are skipped silently — chain compatibility is enforced
// at chain-build time.
func (c *ChainExecutor) ExecuteStreamingResponsePolicies(
	traceCtx context.Context,
	policyList []policy.Policy,
	ctx *policy.ResponseStreamContext,
	chunk *policy.StreamBody,
	specs []policy.PolicySpec,
	api, route string,
	hasExecutionConditions bool,
) (*StreamingResponseExecutionResult, error) {
	startTime := time.Now()
	result := &StreamingResponseExecutionResult{
		Results: make([]StreamingResponsePolicyResult, 0, len(policyList)),
	}

	// Track the current chunk body so each policy sees the previous policy's output.
	currentChunk := chunk

	for i := len(policyList) - 1; i >= 0; i-- {
		pol := policyList[i]
		spec := specs[i]
		policyStartTime := time.Now()

		_, span := c.tracer.Start(traceCtx, fmt.Sprintf(constants.SpanPolicyResponseFormat, spec.Name),
			trace.WithSpanKind(trace.SpanKindInternal))
		if span.IsRecording() {
			span.SetAttributes(
				attribute.String(constants.AttrPolicyName, spec.Name),
				attribute.String(constants.AttrPolicyVersion, spec.Version),
				attribute.Bool(constants.AttrPolicyEnabled, spec.Enabled),
			)
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
				conditionMet, err := c.celEvaluator.EvaluateStreamingResponseCondition(*spec.ExecutionCondition, ctx)
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

		streamingPol, ok := pol.(policy.StreamingResponsePolicy)
		if !ok {
			span.End()
			continue
		}

		action := streamingPol.OnResponseBodyChunk(ctx, currentChunk)
		executionTime := time.Since(policyStartTime)

		metrics.PolicyExecutionsTotal.WithLabelValues(spec.Name, spec.Version, api, route, "executed").Inc()
		metrics.PolicyDurationSeconds.WithLabelValues(spec.Name, spec.Version, api, route).Observe(executionTime.Seconds())

		if span.IsRecording() {
			span.SetAttributes(attribute.Int64(constants.AttrPolicyExecutionTimeNS, executionTime.Nanoseconds()))
		}

		result.Results = append(result.Results, StreamingResponsePolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        &action,
			ExecutionTime: executionTime,
		})

		// Propagate modified bytes to the next policy in the chain
		if action.BodyMutation != nil {
			currentChunk = &policy.StreamBody{
				Chunk:       action.BodyMutation,
				EndOfStream: chunk.EndOfStream,
			}
		}

		result.FinalAction = &action
		span.End()
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
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

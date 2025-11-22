package core

import (
	"fmt"
	"time"

	"github.com/policy-engine/sdk/policies"
)

// RequestPolicyResult represents the result of executing a single request policy
type RequestPolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        *policies.RequestPolicyAction
	Error         error
	ExecutionTime time.Duration
	Skipped       bool // true if condition evaluated to false
}

// RequestExecutionResult represents the result of executing all request policies in a chain
type RequestExecutionResult struct {
	Results            []RequestPolicyResult
	ShortCircuited     bool                          // true if chain stopped early due to ImmediateResponse
	FinalAction        *policies.RequestPolicyAction // Final action to apply
	TotalExecutionTime time.Duration
}

// ResponsePolicyResult represents the result of executing a single response policy
type ResponsePolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        *policies.ResponsePolicyAction
	Error         error
	ExecutionTime time.Duration
	Skipped       bool // true if condition evaluated to false
}

// ResponseExecutionResult represents the result of executing all response policies in a chain
type ResponseExecutionResult struct {
	Results            []ResponsePolicyResult
	FinalAction        *policies.ResponsePolicyAction // Final action to apply
	TotalExecutionTime time.Duration
}

// ExecuteRequestPolicies executes request policies with condition evaluation
// T043: Implements execution with condition evaluation and short-circuit logic
func (c *Core) ExecuteRequestPolicies(policyList []policies.RequestPolicy, ctx *policies.RequestContext, specs []policies.PolicySpec) (*RequestExecutionResult, error) {
	startTime := time.Now()
	result := &RequestExecutionResult{
		Results:        make([]RequestPolicyResult, 0, len(policyList)),
		ShortCircuited: false,
	}

	// Execute each policy in order
	for i, policy := range policyList {
		policyStartTime := time.Now()
		spec := specs[i]

		// Check if policy is enabled
		if !spec.Enabled {
			result.Results = append(result.Results, RequestPolicyResult{
				PolicyName:    spec.Name,
				PolicyVersion: spec.Version,
				Skipped:       true,
				ExecutionTime: time.Since(policyStartTime),
			})
			continue
		}

		// Evaluate execution condition if present
		if spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			if c.celEvaluator != nil {
				conditionMet, err := c.celEvaluator.EvaluateRequestCondition(*spec.ExecutionCondition, ctx)
				if err != nil {
					return nil, fmt.Errorf("condition evaluation failed for policy %s:%s: %w", spec.Name, spec.Version, err)
				}
				if !conditionMet {
					// Condition not met - skip policy
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

		// Execute policy
		action := policy.ExecuteRequest(ctx, spec.Parameters.Raw)
		executionTime := time.Since(policyStartTime)

		policyResult := RequestPolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        action,
			ExecutionTime: executionTime,
			Skipped:       false,
		}

		result.Results = append(result.Results, policyResult)

		// Apply action if present
		if action != nil && action.Action != nil {
			// Check for short-circuit (T047)
			if action.Action.StopExecution() {
				result.ShortCircuited = true
				result.FinalAction = action
				break
			}

			// Apply modifications to context (T045)
			if mods, ok := action.Action.(policies.UpstreamRequestModifications); ok {
				applyRequestModifications(ctx, &mods)
			}
		}
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// ExecuteResponsePolicies executes response policies with condition evaluation
// T044: Implements execution with condition evaluation
func (c *Core) ExecuteResponsePolicies(policyList []policies.ResponsePolicy, ctx *policies.ResponseContext, specs []policies.PolicySpec) (*ResponseExecutionResult, error) {
	startTime := time.Now()
	result := &ResponseExecutionResult{
		Results: make([]ResponsePolicyResult, 0, len(policyList)),
	}

	// Execute each policy in order
	for i, policy := range policyList {
		policyStartTime := time.Now()
		spec := specs[i]

		// Check if policy is enabled
		if !spec.Enabled {
			result.Results = append(result.Results, ResponsePolicyResult{
				PolicyName:    spec.Name,
				PolicyVersion: spec.Version,
				Skipped:       true,
				ExecutionTime: time.Since(policyStartTime),
			})
			continue
		}

		// Evaluate execution condition if present
		if spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			if c.celEvaluator != nil {
				conditionMet, err := c.celEvaluator.EvaluateResponseCondition(*spec.ExecutionCondition, ctx)
				if err != nil {
					return nil, fmt.Errorf("condition evaluation failed for policy %s:%s: %w", spec.Name, spec.Version, err)
				}
				if !conditionMet {
					// Condition not met - skip policy
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

		// Execute policy
		action := policy.ExecuteResponse(ctx, spec.Parameters.Raw)
		executionTime := time.Since(policyStartTime)

		policyResult := ResponsePolicyResult{
			PolicyName:    spec.Name,
			PolicyVersion: spec.Version,
			Action:        action,
			ExecutionTime: executionTime,
			Skipped:       false,
		}

		result.Results = append(result.Results, policyResult)

		// Apply action if present (T046)
		if action != nil && action.Action != nil {
			if mods, ok := action.Action.(policies.UpstreamResponseModifications); ok {
				applyResponseModifications(ctx, &mods)
			}
		}
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// applyRequestModifications applies request modifications to context
// T045: Implements request context modification
func applyRequestModifications(ctx *policies.RequestContext, mods *policies.UpstreamRequestModifications) {
	// Set headers (replace existing)
	if mods.SetHeaders != nil {
		for key, value := range mods.SetHeaders {
			ctx.Headers[key] = []string{value}
		}
	}

	// Remove headers
	if mods.RemoveHeaders != nil {
		for _, key := range mods.RemoveHeaders {
			delete(ctx.Headers, key)
		}
	}

	// Append headers
	if mods.AppendHeaders != nil {
		for key, values := range mods.AppendHeaders {
			existing := ctx.Headers[key]
			ctx.Headers[key] = append(existing, values...)
		}
	}

	// Update body (nil = no change, []byte{} = clear)
	if mods.Body != nil {
		ctx.Body = &policies.Body{
			Content:     mods.Body,
			EndOfStream: true, // Modifications are always complete
			Present:     true,
		}
	}

	// Update path
	if mods.Path != nil {
		ctx.Path = *mods.Path
	}

	// Update method
	if mods.Method != nil {
		ctx.Method = *mods.Method
	}
}

// applyResponseModifications applies response modifications to context
// T046: Implements response context modification
func applyResponseModifications(ctx *policies.ResponseContext, mods *policies.UpstreamResponseModifications) {
	// Set headers (replace existing)
	if mods.SetHeaders != nil {
		for key, value := range mods.SetHeaders {
			ctx.ResponseHeaders[key] = []string{value}
		}
	}

	// Remove headers
	if mods.RemoveHeaders != nil {
		for _, key := range mods.RemoveHeaders {
			delete(ctx.ResponseHeaders, key)
		}
	}

	// Append headers
	if mods.AppendHeaders != nil {
		for key, values := range mods.AppendHeaders {
			existing := ctx.ResponseHeaders[key]
			ctx.ResponseHeaders[key] = append(existing, values...)
		}
	}

	// Update body (nil = no change, []byte{} = clear)
	if mods.Body != nil {
		ctx.ResponseBody = &policies.Body{
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

// Core represents the policy execution engine
// T048: Added CEL evaluator for condition evaluation and metrics collection
type Core struct {
	registry     *PolicyRegistry
	celEvaluator CELEvaluator
}

// CELEvaluator interface for condition evaluation
type CELEvaluator interface {
	EvaluateRequestCondition(expression string, ctx *policies.RequestContext) (bool, error)
	EvaluateResponseCondition(expression string, ctx *policies.ResponseContext) (bool, error)
}

// NewCore creates a new Core execution engine
func NewCore(registry *PolicyRegistry, celEvaluator CELEvaluator) *Core {
	return &Core{
		registry:     registry,
		celEvaluator: celEvaluator,
	}
}

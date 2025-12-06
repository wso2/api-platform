package executor

import (
	"fmt"
	"time"

	"github.com/policy-engine/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// RequestPolicyResult represents the result of executing a single request policy
type RequestPolicyResult struct {
	PolicyName    string
	PolicyVersion string
	Action        policy.RequestAction
	Error         error
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
	FinalAction        policy.ResponseAction // Final action to apply
	TotalExecutionTime time.Duration
}

// ExecuteRequestPolicies executes request policies with condition evaluation
// T043: Implements execution with condition evaluation and short-circuit logic
func (c *ChainExecutor) ExecuteRequestPolicies(policyList []policy.Policy, ctx *policy.RequestContext, specs []policy.PolicySpec) (*RequestExecutionResult, error) {
	startTime := time.Now()
	result := &RequestExecutionResult{
		Results:        make([]RequestPolicyResult, 0, len(policyList)),
		ShortCircuited: false,
	}

	// Execute each policy in order
	for i, pol := range policyList {
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
		action := pol.OnRequest(ctx, spec.Parameters.Raw)
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
		if action != nil {
			// Check for short-circuit (T047)
			if action.StopExecution() {
				result.ShortCircuited = true
				result.FinalAction = action
				break
			}

			// Apply modifications to context (T045)
			if mods, ok := action.(policy.UpstreamRequestModifications); ok {
				applyRequestModifications(ctx, &mods)
			}
		}
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// ExecuteResponsePolicies executes response policies with condition evaluation
// T044: Implements execution with condition evaluation
func (c *ChainExecutor) ExecuteResponsePolicies(policyList []policy.Policy, ctx *policy.ResponseContext, specs []policy.PolicySpec) (*ResponseExecutionResult, error) {
	startTime := time.Now()
	result := &ResponseExecutionResult{
		Results: make([]ResponsePolicyResult, 0, len(policyList)),
	}

	// Execute each policy in reverse order (last to first)
	// This allows policies to "unwrap" in the reverse order they "wrapped" the request
	for i := len(policyList) - 1; i >= 0; i-- {
		pol := policyList[i]
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
		action := pol.OnResponse(ctx, spec.Parameters.Raw)
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
		if action != nil {
			if mods, ok := action.(policy.UpstreamResponseModifications); ok {
				applyResponseModifications(ctx, &mods)
			}
		}
	}

	result.TotalExecutionTime = time.Since(startTime)
	return result, nil
}

// applyRequestModifications applies request modifications to context
// T045: Implements request context modification
func applyRequestModifications(ctx *policy.RequestContext, mods *policy.UpstreamRequestModifications) {
	// Get direct access to headers for mutation (kernel-only API)
	headers := ctx.Headers.UnsafeInternalValues()

	// Set headers (replace existing)
	if mods.SetHeaders != nil {
		for key, value := range mods.SetHeaders {
			headers[key] = []string{value}
		}
	}

	// Remove headers
	if mods.RemoveHeaders != nil {
		for _, key := range mods.RemoveHeaders {
			delete(headers, key)
		}
	}

	// Append headers
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

	// Update method
	if mods.Method != nil {
		ctx.Method = *mods.Method
	}
}

// applyResponseModifications applies response modifications to context
// T046: Implements response context modification
func applyResponseModifications(ctx *policy.ResponseContext, mods *policy.UpstreamResponseModifications) {
	// Get direct access to response headers for mutation (kernel-only API)
	headers := ctx.ResponseHeaders.UnsafeInternalValues()

	// Set headers (replace existing)
	if mods.SetHeaders != nil {
		for key, value := range mods.SetHeaders {
			headers[key] = []string{value}
		}
	}

	// Remove headers
	if mods.RemoveHeaders != nil {
		for _, key := range mods.RemoveHeaders {
			delete(headers, key)
		}
	}

	// Append headers
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
}

// CELEvaluator interface for condition evaluation
type CELEvaluator interface {
	EvaluateRequestCondition(expression string, ctx *policy.RequestContext) (bool, error)
	EvaluateResponseCondition(expression string, ctx *policy.ResponseContext) (bool, error)
}

// NewChainExecutor creates a new ChainExecutor execution engine
func NewChainExecutor(reg *registry.PolicyRegistry, celEvaluator CELEvaluator) *ChainExecutor {
	return &ChainExecutor{
		registry:     reg,
		celEvaluator: celEvaluator,
	}
}

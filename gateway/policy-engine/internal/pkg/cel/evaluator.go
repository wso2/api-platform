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

package cel

import (
	"fmt"
	"sync"

	"github.com/google/cel-go/cel"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// CELEvaluator provides CEL expression evaluation for RequestContext and ResponseContext
type CELEvaluator interface {
	// EvaluateRequestCondition evaluates a CEL expression against RequestContext
	// Returns true if condition passes, false if it fails
	EvaluateRequestCondition(expression string, ctx *policy.RequestContext) (bool, error)

	// EvaluateResponseCondition evaluates a CEL expression against ResponseContext
	// Returns true if condition passes, false if it fails
	EvaluateResponseCondition(expression string, ctx *policy.ResponseContext) (bool, error)
}

// celEvaluator implements CELEvaluator with caching
type celEvaluator struct {
	mu sync.RWMutex

	// Compiled CEL programs cache
	// Key: expression string, Value: compiled cel.Program
	programCache map[string]cel.Program

	// Unified CEL environment supporting both request and response contexts
	env *cel.Env
}

// NewCELEvaluator creates a new CEL evaluator with caching
func NewCELEvaluator() (CELEvaluator, error) {
	// Create unified CEL environment supporting both request and response contexts
	env, err := createCELEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return &celEvaluator{
		programCache: make(map[string]cel.Program),
		env:          env,
	}, nil
}

// createCELEnv creates a unified CEL environment supporting both request and response contexts
// This environment is used for both request and response phase evaluations, allowing
// policies that execute in both phases to use the same executionCondition expression
func createCELEnv() (*cel.Env, error) {
	return cel.NewEnv(
		// Processing phase indicator - enables phase-specific logic in CEL expressions
		// Values: "request", "response" (future: "request_headers", "request_body", "response_headers", "response_body")
		cel.Variable("processing.phase", cel.StringType),
		// RequestContext variables
		cel.Variable("request", cel.ObjectType("RequestContext")),
		cel.Variable("request.Headers", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable("request.Body", cel.MapType(cel.StringType, cel.DynType)), // Body struct with Content, EndOfStream, Present
		cel.Variable("request.Path", cel.StringType),
		cel.Variable("request.Method", cel.StringType),
		cel.Variable("request.RequestID", cel.StringType),
		cel.Variable("request.Metadata", cel.MapType(cel.StringType, cel.DynType)),
		// ResponseContext variables
		cel.Variable("response", cel.ObjectType("ResponseContext")),
		cel.Variable("response.RequestHeaders", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable("response.RequestBody", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("response.RequestPath", cel.StringType),
		cel.Variable("response.RequestMethod", cel.StringType),
		cel.Variable("response.ResponseHeaders", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable("response.ResponseBody", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("response.ResponseStatus", cel.IntType),
		cel.Variable("response.RequestID", cel.StringType),
		cel.Variable("response.Metadata", cel.MapType(cel.StringType, cel.DynType)),
	)
}

// EvaluateRequestCondition evaluates a CEL expression against RequestContext
func (e *celEvaluator) EvaluateRequestCondition(expression string, ctx *policy.RequestContext) (bool, error) {
	// Get or compile program
	program, err := e.getOrCompileProgram(expression)
	if err != nil {
		return false, fmt.Errorf("failed to compile CEL expression: %w", err)
	}

	// Build body representation for CEL
	var bodyForCEL interface{}
	if ctx.Body != nil && ctx.Body.Present {
		bodyForCEL = map[string]interface{}{
			"Content":     ctx.Body.Content,
			"EndOfStream": ctx.Body.EndOfStream,
			"Present":     ctx.Body.Present,
		}
	} else {
		bodyForCEL = nil
	}

	// Build evaluation context - flatten to match CEL variable declarations
	evalCtx := map[string]interface{}{
		// Processing phase indicator
		"processing.phase": "request",
		"request": map[string]interface{}{
			"Headers":   ctx.Headers,
			"Body":      bodyForCEL,
			"Path":      ctx.Path,
			"Method":    ctx.Method,
			"RequestID": ctx.RequestID,
			"Metadata":  ctx.Metadata,
		},
		"request.Headers":   ctx.Headers,
		"request.Body":      bodyForCEL,
		"request.Path":      ctx.Path,
		"request.Method":    ctx.Method,
		"request.RequestID": ctx.RequestID,
		"request.Metadata":  ctx.Metadata,
		// Response variables (empty during request phase, for dual-phase policies)
		"response": map[string]interface{}{
			"RequestHeaders":  ctx.Headers,
			"RequestBody":     bodyForCEL,
			"RequestPath":     ctx.Path,
			"RequestMethod":   ctx.Method,
			"ResponseHeaders": map[string][]string{},
			"ResponseBody":    nil,
			"ResponseStatus":  0,
			"RequestID":       ctx.RequestID,
			"Metadata":        ctx.Metadata,
		},
		"response.RequestHeaders":  ctx.Headers,
		"response.RequestBody":     bodyForCEL,
		"response.RequestPath":     ctx.Path,
		"response.RequestMethod":   ctx.Method,
		"response.ResponseHeaders": map[string][]string{},
		"response.ResponseBody":    nil,
		"response.ResponseStatus":  0,
		"response.RequestID":       ctx.RequestID,
		"response.Metadata":        ctx.Metadata,
	}

	// Evaluate
	result, _, err := program.Eval(evalCtx)
	if err != nil {
		return false, fmt.Errorf("CEL evaluation failed: %w", err)
	}

	// Convert to boolean
	boolResult, ok := result.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression must return boolean, got %T", result.Value())
	}

	return boolResult, nil
}

// EvaluateResponseCondition evaluates a CEL expression against ResponseContext
func (e *celEvaluator) EvaluateResponseCondition(expression string, ctx *policy.ResponseContext) (bool, error) {
	// Get or compile program
	program, err := e.getOrCompileProgram(expression)
	if err != nil {
		return false, fmt.Errorf("failed to compile CEL expression: %w", err)
	}

	// Build body representations for CEL
	var requestBodyForCEL interface{}
	if ctx.RequestBody != nil && ctx.RequestBody.Present {
		requestBodyForCEL = map[string]interface{}{
			"Content":     ctx.RequestBody.Content,
			"EndOfStream": ctx.RequestBody.EndOfStream,
			"Present":     ctx.RequestBody.Present,
		}
	} else {
		requestBodyForCEL = nil
	}

	var responseBodyForCEL interface{}
	if ctx.ResponseBody != nil && ctx.ResponseBody.Present {
		responseBodyForCEL = map[string]interface{}{
			"Content":     ctx.ResponseBody.Content,
			"EndOfStream": ctx.ResponseBody.EndOfStream,
			"Present":     ctx.ResponseBody.Present,
		}
	} else {
		responseBodyForCEL = nil
	}

	// Build evaluation context - flatten to match CEL variable declarations
	evalCtx := map[string]interface{}{
		// Processing phase indicator
		"processing.phase": "response",
		"response": map[string]interface{}{
			"RequestHeaders":  ctx.RequestHeaders,
			"RequestBody":     requestBodyForCEL,
			"RequestPath":     ctx.RequestPath,
			"RequestMethod":   ctx.RequestMethod,
			"ResponseHeaders": ctx.ResponseHeaders,
			"ResponseBody":    responseBodyForCEL,
			"ResponseStatus":  ctx.ResponseStatus,
			"RequestID":       ctx.RequestID,
			"Metadata":        ctx.Metadata,
		},
		"response.RequestHeaders":  ctx.RequestHeaders,
		"response.RequestBody":     requestBodyForCEL,
		"response.RequestPath":     ctx.RequestPath,
		"response.RequestMethod":   ctx.RequestMethod,
		"response.ResponseHeaders": ctx.ResponseHeaders,
		"response.ResponseBody":    responseBodyForCEL,
		"response.ResponseStatus":  ctx.ResponseStatus,
		"response.RequestID":       ctx.RequestID,
		"response.Metadata":        ctx.Metadata,
		// Request variables (aliases to request data for consistency across phases)
		"request": map[string]interface{}{
			"Headers":   ctx.RequestHeaders,
			"Body":      requestBodyForCEL,
			"Path":      ctx.RequestPath,
			"Method":    ctx.RequestMethod,
			"RequestID": ctx.RequestID,
			"Metadata":  ctx.Metadata,
		},
		"request.Headers":   ctx.RequestHeaders,
		"request.Body":      requestBodyForCEL,
		"request.Path":      ctx.RequestPath,
		"request.Method":    ctx.RequestMethod,
		"request.RequestID": ctx.RequestID,
		"request.Metadata":  ctx.Metadata,
	}

	// Evaluate
	result, _, err := program.Eval(evalCtx)
	if err != nil {
		return false, fmt.Errorf("CEL evaluation failed: %w", err)
	}

	// Convert to boolean
	boolResult, ok := result.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression must return boolean, got %T", result.Value())
	}

	return boolResult, nil
}

// getOrCompileProgram gets cached program or compiles new one
// Uses a unified environment supporting both request and response contexts
func (e *celEvaluator) getOrCompileProgram(expression string) (cel.Program, error) {
	// Check cache first (read lock)
	e.mu.RLock()
	if program, ok := e.programCache[expression]; ok {
		e.mu.RUnlock()
		return program, nil
	}
	e.mu.RUnlock()

	// Compile (write lock)
	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check after acquiring write lock
	if program, ok := e.programCache[expression]; ok {
		return program, nil
	}

	// Compile expression
	ast, issues := e.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compilation failed: %w", issues.Err())
	}

	// Create program
	program, err := e.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program creation failed: %w", err)
	}

	// Cache and return
	e.programCache[expression] = program
	return program, nil
}

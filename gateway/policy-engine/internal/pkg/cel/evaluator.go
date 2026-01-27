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

// evalCtxPool is a sync.Pool for reusing evaluation context maps
// This significantly reduces memory allocations during CEL evaluation
var evalCtxPool = sync.Pool{
	New: func() interface{} {
		return make(map[string]interface{}, 24) // Pre-allocate with expected capacity
	},
}

// requestCtxPool is a sync.Pool for reusing request context maps
var requestCtxPool = sync.Pool{
	New: func() interface{} {
		return make(map[string]interface{}, 8)
	},
}

// responseCtxPool is a sync.Pool for reusing response context maps
var responseCtxPool = sync.Pool{
	New: func() interface{} {
		return make(map[string]interface{}, 10)
	},
}

// emptyHeadersMap is a pre-allocated empty headers map to avoid allocations
var emptyHeadersMap = map[string][]string{}

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

	// Build body representation for CEL (only allocate if body is present)
	var bodyForCEL interface{}
	if ctx.Body != nil && ctx.Body.Present {
		bodyForCEL = map[string]interface{}{
			"Content":     ctx.Body.Content,
			"EndOfStream": ctx.Body.EndOfStream,
			"Present":     ctx.Body.Present,
		}
	}

	// Get pooled maps for evaluation context
	evalCtx := evalCtxPool.Get().(map[string]interface{})
	requestMap := requestCtxPool.Get().(map[string]interface{})
	responseMap := responseCtxPool.Get().(map[string]interface{})

	// Populate request map
	requestMap["Headers"] = ctx.Headers
	requestMap["Body"] = bodyForCEL
	requestMap["Path"] = ctx.Path
	requestMap["Method"] = ctx.Method
	requestMap["RequestID"] = ctx.RequestID
	requestMap["Metadata"] = ctx.Metadata

	// Populate response map (empty during request phase, for dual-phase policies)
	responseMap["RequestHeaders"] = ctx.Headers
	responseMap["RequestBody"] = bodyForCEL
	responseMap["RequestPath"] = ctx.Path
	responseMap["RequestMethod"] = ctx.Method
	responseMap["ResponseHeaders"] = emptyHeadersMap
	responseMap["ResponseBody"] = nil
	responseMap["ResponseStatus"] = 0
	responseMap["RequestID"] = ctx.RequestID
	responseMap["Metadata"] = ctx.Metadata

	// Populate evaluation context
	evalCtx["processing.phase"] = "request"
	evalCtx["request"] = requestMap
	evalCtx["request.Headers"] = ctx.Headers
	evalCtx["request.Body"] = bodyForCEL
	evalCtx["request.Path"] = ctx.Path
	evalCtx["request.Method"] = ctx.Method
	evalCtx["request.RequestID"] = ctx.RequestID
	evalCtx["request.Metadata"] = ctx.Metadata
	evalCtx["response"] = responseMap
	evalCtx["response.RequestHeaders"] = ctx.Headers
	evalCtx["response.RequestBody"] = bodyForCEL
	evalCtx["response.RequestPath"] = ctx.Path
	evalCtx["response.RequestMethod"] = ctx.Method
	evalCtx["response.ResponseHeaders"] = emptyHeadersMap
	evalCtx["response.ResponseBody"] = nil
	evalCtx["response.ResponseStatus"] = 0
	evalCtx["response.RequestID"] = ctx.RequestID
	evalCtx["response.Metadata"] = ctx.Metadata

	// Evaluate
	result, _, err := program.Eval(evalCtx)

	// Clear and return maps to pool (must clear to avoid memory leaks from retained references)
	for k := range requestMap {
		delete(requestMap, k)
	}
	requestCtxPool.Put(requestMap)

	for k := range responseMap {
		delete(responseMap, k)
	}
	responseCtxPool.Put(responseMap)

	for k := range evalCtx {
		delete(evalCtx, k)
	}
	evalCtxPool.Put(evalCtx)

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

	// Build body representations for CEL (only allocate if body is present)
	var requestBodyForCEL interface{}
	if ctx.RequestBody != nil && ctx.RequestBody.Present {
		requestBodyForCEL = map[string]interface{}{
			"Content":     ctx.RequestBody.Content,
			"EndOfStream": ctx.RequestBody.EndOfStream,
			"Present":     ctx.RequestBody.Present,
		}
	}

	var responseBodyForCEL interface{}
	if ctx.ResponseBody != nil && ctx.ResponseBody.Present {
		responseBodyForCEL = map[string]interface{}{
			"Content":     ctx.ResponseBody.Content,
			"EndOfStream": ctx.ResponseBody.EndOfStream,
			"Present":     ctx.ResponseBody.Present,
		}
	}

	// Get pooled maps for evaluation context
	evalCtx := evalCtxPool.Get().(map[string]interface{})
	requestMap := requestCtxPool.Get().(map[string]interface{})
	responseMap := responseCtxPool.Get().(map[string]interface{})

	// Populate response map
	responseMap["RequestHeaders"] = ctx.RequestHeaders
	responseMap["RequestBody"] = requestBodyForCEL
	responseMap["RequestPath"] = ctx.RequestPath
	responseMap["RequestMethod"] = ctx.RequestMethod
	responseMap["ResponseHeaders"] = ctx.ResponseHeaders
	responseMap["ResponseBody"] = responseBodyForCEL
	responseMap["ResponseStatus"] = ctx.ResponseStatus
	responseMap["RequestID"] = ctx.RequestID
	responseMap["Metadata"] = ctx.Metadata

	// Populate request map (aliases to request data for consistency across phases)
	requestMap["Headers"] = ctx.RequestHeaders
	requestMap["Body"] = requestBodyForCEL
	requestMap["Path"] = ctx.RequestPath
	requestMap["Method"] = ctx.RequestMethod
	requestMap["RequestID"] = ctx.RequestID
	requestMap["Metadata"] = ctx.Metadata

	// Populate evaluation context
	evalCtx["processing.phase"] = "response"
	evalCtx["response"] = responseMap
	evalCtx["response.RequestHeaders"] = ctx.RequestHeaders
	evalCtx["response.RequestBody"] = requestBodyForCEL
	evalCtx["response.RequestPath"] = ctx.RequestPath
	evalCtx["response.RequestMethod"] = ctx.RequestMethod
	evalCtx["response.ResponseHeaders"] = ctx.ResponseHeaders
	evalCtx["response.ResponseBody"] = responseBodyForCEL
	evalCtx["response.ResponseStatus"] = ctx.ResponseStatus
	evalCtx["response.RequestID"] = ctx.RequestID
	evalCtx["response.Metadata"] = ctx.Metadata
	evalCtx["request"] = requestMap
	evalCtx["request.Headers"] = ctx.RequestHeaders
	evalCtx["request.Body"] = requestBodyForCEL
	evalCtx["request.Path"] = ctx.RequestPath
	evalCtx["request.Method"] = ctx.RequestMethod
	evalCtx["request.RequestID"] = ctx.RequestID
	evalCtx["request.Metadata"] = ctx.Metadata

	// Evaluate
	result, _, err := program.Eval(evalCtx)

	// Clear and return maps to pool (must clear to avoid memory leaks from retained references)
	for k := range requestMap {
		delete(requestMap, k)
	}
	requestCtxPool.Put(requestMap)

	for k := range responseMap {
		delete(responseMap, k)
	}
	responseCtxPool.Put(responseMap)

	for k := range evalCtx {
		delete(evalCtx, k)
	}
	evalCtxPool.Put(evalCtx)

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

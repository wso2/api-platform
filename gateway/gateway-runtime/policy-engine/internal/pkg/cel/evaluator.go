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

// CELEvaluator provides CEL expression evaluation for each processing phase context
type CELEvaluator interface {
	EvaluateRequestHeaderCondition(expression string, ctx *policy.RequestHeaderContext) (bool, error)
	EvaluateRequestBodyCondition(expression string, ctx *policy.RequestContext) (bool, error)
	EvaluateResponseHeaderCondition(expression string, ctx *policy.ResponseHeaderContext) (bool, error)
	EvaluateResponseBodyCondition(expression string, ctx *policy.ResponseContext) (bool, error)
	EvaluateStreamingRequestCondition(expression string, ctx *policy.RequestStreamContext) (bool, error)
	EvaluateStreamingResponseCondition(expression string, ctx *policy.ResponseStreamContext) (bool, error)
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

// createCELEnv creates a unified CEL environment supporting both request and response contexts.
// This environment is used for all phase evaluations, allowing policies to use the same
// executionCondition expression regardless of which phase they execute in.
func createCELEnv() (*cel.Env, error) {
	return cel.NewEnv(
		// Processing phase indicator — enables phase-specific logic in CEL expressions
		// Values: "request_headers", "request_body", "response_headers", "response_body"
		cel.Variable("processing.phase", cel.StringType),
		// RequestContext variables
		cel.Variable("request", cel.ObjectType("RequestContext")),
		cel.Variable("request.Headers", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable("request.Body", cel.MapType(cel.StringType, cel.DynType)),
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

// EvaluateRequestHeaderCondition evaluates a CEL expression against a RequestHeaderContext
func (e *celEvaluator) EvaluateRequestHeaderCondition(expression string, ctx *policy.RequestHeaderContext) (bool, error) {
	program, err := e.getOrCompileProgram(expression)
	if err != nil {
		return false, fmt.Errorf("failed to compile CEL expression: %w", err)
	}

	evalCtx := buildRequestHeaderEvalCtx(ctx, "request_headers")
	return e.eval(program, evalCtx)
}

// EvaluateRequestBodyCondition evaluates a CEL expression against a RequestBodyContext
func (e *celEvaluator) EvaluateRequestBodyCondition(expression string, ctx *policy.RequestContext) (bool, error) {
	program, err := e.getOrCompileProgram(expression)
	if err != nil {
		return false, fmt.Errorf("failed to compile CEL expression: %w", err)
	}
	return e.eval(program, buildRequestBodyEvalCtx(ctx, "request_body"))
}

// EvaluateResponseHeaderCondition evaluates a CEL expression against a ResponseHeaderContext
func (e *celEvaluator) EvaluateResponseHeaderCondition(expression string, ctx *policy.ResponseHeaderContext) (bool, error) {
	program, err := e.getOrCompileProgram(expression)
	if err != nil {
		return false, fmt.Errorf("failed to compile CEL expression: %w", err)
	}
	return e.eval(program, buildResponseHeaderEvalCtx(ctx, "response_headers"))
}

// EvaluateResponseBodyCondition evaluates a CEL expression against a ResponseBodyContext
func (e *celEvaluator) EvaluateResponseBodyCondition(expression string, ctx *policy.ResponseContext) (bool, error) {
	program, err := e.getOrCompileProgram(expression)
	if err != nil {
		return false, fmt.Errorf("failed to compile CEL expression: %w", err)
	}
	return e.eval(program, buildResponseBodyEvalCtx(ctx, "response_body"))
}

// EvaluateStreamingRequestCondition evaluates a CEL expression against a RequestStreamContext.
// The phase is set to "request_body" so conditions are consistent with buffered request body processing.
func (e *celEvaluator) EvaluateStreamingRequestCondition(expression string, ctx *policy.RequestStreamContext) (bool, error) {
	program, err := e.getOrCompileProgram(expression)
	if err != nil {
		return false, fmt.Errorf("failed to compile CEL expression: %w", err)
	}
	return e.eval(program, buildStreamingRequestEvalCtx(ctx))
}

// EvaluateStreamingResponseCondition evaluates a CEL expression against a ResponseStreamContext.
// The phase is set to "response_body" so conditions are consistent with buffered response body processing.
func (e *celEvaluator) EvaluateStreamingResponseCondition(expression string, ctx *policy.ResponseStreamContext) (bool, error) {
	program, err := e.getOrCompileProgram(expression)
	if err != nil {
		return false, fmt.Errorf("failed to compile CEL expression: %w", err)
	}
	return e.eval(program, buildStreamingResponseEvalCtx(ctx))
}

// bodyToCEL converts a *policy.Body to the map representation expected by CEL.
// Returns nil when the body is absent or not yet present.
func bodyToCEL(body *policy.Body) interface{} {
	if body == nil || !body.Present {
		return nil
	}
	return map[string]interface{}{
		"Content":     body.Content,
		"EndOfStream": body.EndOfStream,
		"Present":     body.Present,
	}
}

// buildRequestHeaderEvalCtx builds a CEL evaluation context from a RequestHeaderContext
func buildRequestHeaderEvalCtx(ctx *policy.RequestHeaderContext, phase string) map[string]interface{} {
	headers := ctx.Headers.GetAll()
	return map[string]interface{}{
		"processing.phase": phase,
		"request": map[string]interface{}{
			"Headers":   headers,
			"Body":      nil,
			"Path":      ctx.Path,
			"Method":    ctx.Method,
			"RequestID": ctx.RequestID,
			"Metadata":  ctx.Metadata,
		},
		"request.Headers":   headers,
		"request.Body":      nil,
		"request.Path":      ctx.Path,
		"request.Method":    ctx.Method,
		"request.RequestID": ctx.RequestID,
		"request.Metadata":  ctx.Metadata,
		"response": map[string]interface{}{
			"RequestHeaders":  headers,
			"RequestBody":     nil,
			"RequestPath":     ctx.Path,
			"RequestMethod":   ctx.Method,
			"ResponseHeaders": map[string][]string{},
			"ResponseBody":    nil,
			"ResponseStatus":  0,
			"RequestID":       ctx.RequestID,
			"Metadata":        ctx.Metadata,
		},
		"response.RequestHeaders":  headers,
		"response.RequestBody":     nil,
		"response.RequestPath":     ctx.Path,
		"response.RequestMethod":   ctx.Method,
		"response.ResponseHeaders": map[string][]string{},
		"response.ResponseBody":    nil,
		"response.ResponseStatus":  0,
		"response.RequestID":       ctx.RequestID,
		"response.Metadata":        ctx.Metadata,
	}
}

// buildRequestBodyEvalCtx builds a CEL evaluation context from a RequestContext
func buildRequestBodyEvalCtx(ctx *policy.RequestContext, phase string) map[string]interface{} {
	headers := ctx.Headers.GetAll()
	body := bodyToCEL(ctx.Body)
	return map[string]interface{}{
		"processing.phase": phase,
		"request": map[string]interface{}{
			"Headers":   headers,
			"Body":      body,
			"Path":      ctx.Path,
			"Method":    ctx.Method,
			"RequestID": ctx.RequestID,
			"Metadata":  ctx.Metadata,
		},
		"request.Headers":   headers,
		"request.Body":      body,
		"request.Path":      ctx.Path,
		"request.Method":    ctx.Method,
		"request.RequestID": ctx.RequestID,
		"request.Metadata":  ctx.Metadata,
		"response": map[string]interface{}{
			"RequestHeaders":  headers,
			"RequestBody":     body,
			"RequestPath":     ctx.Path,
			"RequestMethod":   ctx.Method,
			"ResponseHeaders": map[string][]string{},
			"ResponseBody":    nil,
			"ResponseStatus":  0,
			"RequestID":       ctx.RequestID,
			"Metadata":        ctx.Metadata,
		},
		"response.RequestHeaders":  headers,
		"response.RequestBody":     body,
		"response.RequestPath":     ctx.Path,
		"response.RequestMethod":   ctx.Method,
		"response.ResponseHeaders": map[string][]string{},
		"response.ResponseBody":    nil,
		"response.ResponseStatus":  0,
		"response.RequestID":       ctx.RequestID,
		"response.Metadata":        ctx.Metadata,
	}
}

// buildResponseHeaderEvalCtx builds a CEL evaluation context from a ResponseHeaderContext
func buildResponseHeaderEvalCtx(ctx *policy.ResponseHeaderContext, phase string) map[string]interface{} {
	requestHeaders := ctx.RequestHeaders.GetAll()
	requestBody := bodyToCEL(ctx.RequestBody)
	responseHeaders := ctx.ResponseHeaders.GetAll()
	return map[string]interface{}{
		"processing.phase": phase,
		"request": map[string]interface{}{
			"Headers":   requestHeaders,
			"Body":      requestBody,
			"Path":      ctx.RequestPath,
			"Method":    ctx.RequestMethod,
			"RequestID": ctx.RequestID,
			"Metadata":  ctx.Metadata,
		},
		"request.Headers":   requestHeaders,
		"request.Body":      requestBody,
		"request.Path":      ctx.RequestPath,
		"request.Method":    ctx.RequestMethod,
		"request.RequestID": ctx.RequestID,
		"request.Metadata":  ctx.Metadata,
		"response": map[string]interface{}{
			"RequestHeaders":  requestHeaders,
			"RequestBody":     requestBody,
			"RequestPath":     ctx.RequestPath,
			"RequestMethod":   ctx.RequestMethod,
			"ResponseHeaders": responseHeaders,
			"ResponseBody":    nil,
			"ResponseStatus":  ctx.ResponseStatus,
			"RequestID":       ctx.RequestID,
			"Metadata":        ctx.Metadata,
		},
		"response.RequestHeaders":  requestHeaders,
		"response.RequestBody":     requestBody,
		"response.RequestPath":     ctx.RequestPath,
		"response.RequestMethod":   ctx.RequestMethod,
		"response.ResponseHeaders": responseHeaders,
		"response.ResponseBody":    nil,
		"response.ResponseStatus":  ctx.ResponseStatus,
		"response.RequestID":       ctx.RequestID,
		"response.Metadata":        ctx.Metadata,
	}
}

// buildResponseBodyEvalCtx builds a CEL evaluation context from a ResponseContext
func buildResponseBodyEvalCtx(ctx *policy.ResponseContext, phase string) map[string]interface{} {
	requestHeaders := ctx.RequestHeaders.GetAll()
	requestBody := bodyToCEL(ctx.RequestBody)
	responseHeaders := ctx.ResponseHeaders.GetAll()
	responseBody := bodyToCEL(ctx.ResponseBody)
	return map[string]interface{}{
		"processing.phase": phase,
		"request": map[string]interface{}{
			"Headers":   requestHeaders,
			"Body":      requestBody,
			"Path":      ctx.RequestPath,
			"Method":    ctx.RequestMethod,
			"RequestID": ctx.RequestID,
			"Metadata":  ctx.Metadata,
		},
		"request.Headers":   requestHeaders,
		"request.Body":      requestBody,
		"request.Path":      ctx.RequestPath,
		"request.Method":    ctx.RequestMethod,
		"request.RequestID": ctx.RequestID,
		"request.Metadata":  ctx.Metadata,
		"response": map[string]interface{}{
			"RequestHeaders":  requestHeaders,
			"RequestBody":     requestBody,
			"RequestPath":     ctx.RequestPath,
			"RequestMethod":   ctx.RequestMethod,
			"ResponseHeaders": responseHeaders,
			"ResponseBody":    responseBody,
			"ResponseStatus":  ctx.ResponseStatus,
			"RequestID":       ctx.RequestID,
			"Metadata":        ctx.Metadata,
		},
		"response.RequestHeaders":  requestHeaders,
		"response.RequestBody":     requestBody,
		"response.RequestPath":     ctx.RequestPath,
		"response.RequestMethod":   ctx.RequestMethod,
		"response.ResponseHeaders": responseHeaders,
		"response.ResponseBody":    responseBody,
		"response.ResponseStatus":  ctx.ResponseStatus,
		"response.RequestID":       ctx.RequestID,
		"response.Metadata":        ctx.Metadata,
	}
}

// buildStreamingRequestEvalCtx builds a CEL evaluation context from a RequestStreamContext.
// Uses phase "request_body" — consistent with buffered request body processing.
func buildStreamingRequestEvalCtx(ctx *policy.RequestStreamContext) map[string]interface{} {
	headers := ctx.Headers.GetAll()
	return map[string]interface{}{
		"processing.phase": "request_body",
		"request": map[string]interface{}{
			"Headers":   headers,
			"Body":      nil,
			"Path":      ctx.Path,
			"Method":    ctx.Method,
			"RequestID": ctx.RequestID,
			"Metadata":  ctx.Metadata,
		},
		"request.Headers":   headers,
		"request.Body":      nil,
		"request.Path":      ctx.Path,
		"request.Method":    ctx.Method,
		"request.RequestID": ctx.RequestID,
		"request.Metadata":  ctx.Metadata,
		"response": map[string]interface{}{
			"RequestHeaders":  headers,
			"RequestBody":     nil,
			"RequestPath":     ctx.Path,
			"RequestMethod":   ctx.Method,
			"ResponseHeaders": map[string][]string{},
			"ResponseBody":    nil,
			"ResponseStatus":  0,
			"RequestID":       ctx.RequestID,
			"Metadata":        ctx.Metadata,
		},
		"response.RequestHeaders":  headers,
		"response.RequestBody":     nil,
		"response.RequestPath":     ctx.Path,
		"response.RequestMethod":   ctx.Method,
		"response.ResponseHeaders": map[string][]string{},
		"response.ResponseBody":    nil,
		"response.ResponseStatus":  0,
		"response.RequestID":       ctx.RequestID,
		"response.Metadata":        ctx.Metadata,
	}
}

// buildStreamingResponseEvalCtx builds a CEL evaluation context from a ResponseStreamContext.
// Uses phase "response_body" — consistent with buffered response body processing.
func buildStreamingResponseEvalCtx(ctx *policy.ResponseStreamContext) map[string]interface{} {
	requestHeaders := ctx.RequestHeaders.GetAll()
	requestBody := bodyToCEL(ctx.RequestBody)
	responseHeaders := ctx.ResponseHeaders.GetAll()
	return map[string]interface{}{
		"processing.phase": "response_body",
		"request": map[string]interface{}{
			"Headers":   requestHeaders,
			"Body":      requestBody,
			"Path":      ctx.RequestPath,
			"Method":    ctx.RequestMethod,
			"RequestID": ctx.RequestID,
			"Metadata":  ctx.Metadata,
		},
		"request.Headers":   requestHeaders,
		"request.Body":      requestBody,
		"request.Path":      ctx.RequestPath,
		"request.Method":    ctx.RequestMethod,
		"request.RequestID": ctx.RequestID,
		"request.Metadata":  ctx.Metadata,
		"response": map[string]interface{}{
			"RequestHeaders":  requestHeaders,
			"RequestBody":     requestBody,
			"RequestPath":     ctx.RequestPath,
			"RequestMethod":   ctx.RequestMethod,
			"ResponseHeaders": responseHeaders,
			"ResponseBody":    nil,
			"ResponseStatus":  ctx.ResponseStatus,
			"RequestID":       ctx.RequestID,
			"Metadata":        ctx.Metadata,
		},
		"response.RequestHeaders":  requestHeaders,
		"response.RequestBody":     requestBody,
		"response.RequestPath":     ctx.RequestPath,
		"response.RequestMethod":   ctx.RequestMethod,
		"response.ResponseHeaders": responseHeaders,
		"response.ResponseBody":    nil,
		"response.ResponseStatus":  ctx.ResponseStatus,
		"response.RequestID":       ctx.RequestID,
		"response.Metadata":        ctx.Metadata,
	}
}

// eval evaluates a compiled program against an evaluation context
func (e *celEvaluator) eval(program cel.Program, evalCtx map[string]interface{}) (bool, error) {
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

	e.mu.Lock()
	defer e.mu.Unlock()

	if program, ok := e.programCache[expression]; ok {
		return program, nil
	}

	ast, issues := e.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compilation failed: %w", issues.Err())
	}

	program, err := e.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program creation failed: %w", err)
	}

	e.programCache[expression] = program
	return program, nil
}

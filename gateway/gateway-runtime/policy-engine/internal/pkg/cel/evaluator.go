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

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// defaultProgramCacheSize is the maximum number of compiled CEL programs kept
// in memory. Least-recently-used entries are evicted when the limit is reached.
const defaultProgramCacheSize = 1024

// CELEvaluator provides CEL expression evaluation for each processing phase context
type CELEvaluator interface {
	EvaluateRequestHeaderCondition(expression string, ctx *policy.RequestHeaderContext) (bool, error)
	EvaluateRequestBodyCondition(expression string, ctx *policy.RequestContext) (bool, error)
	EvaluateResponseHeaderCondition(expression string, ctx *policy.ResponseHeaderContext) (bool, error)
	EvaluateResponseBodyCondition(expression string, ctx *policy.ResponseContext) (bool, error)
	EvaluateFaultCondition(expression string, ctx *policy.FaultContext) (bool, error)
	EvaluateStreamingRequestCondition(expression string, ctx *policy.RequestStreamContext) (bool, error)
	EvaluateStreamingResponseCondition(expression string, ctx *policy.ResponseStreamContext) (bool, error)
}

// celEvaluator implements CELEvaluator with caching
type celEvaluator struct {
	mu sync.Mutex

	// Bounded LRU cache of compiled CEL programs keyed by expression string.
	// LRU get promotes the entry to most-recently-used, so a plain Mutex is
	// required instead of an RWMutex.
	programCache *programLRUCache

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
		programCache: newProgramLRUCache(defaultProgramCacheSize),
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
		// FaultContext variables — the failure itself, for a fault policy's
		// executionCondition. See faultEvalVars for why these are declared for every phase
		// rather than only the fault one.
		cel.Variable("fault", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("fault.Source", cel.StringType),
		cel.Variable("fault.Code", cel.StringType),
		cel.Variable("fault.Type", cel.StringType),
		cel.Variable("fault.Direction", cel.StringType),
		cel.Variable("fault.Message", cel.StringType),
		cel.Variable("fault.Policy", cel.StringType),
		cel.Variable("fault.PolicyVersion", cel.StringType),
		cel.Variable("fault.PolicyPhase", cel.StringType),
		cel.Variable("fault.RouteKey", cel.StringType),
		cel.Variable("fault.Status", cel.IntType),
		cel.Variable("fault.OriginalStatus", cel.IntType),
		cel.Variable("fault.ResponseCommitted", cel.BoolType),
		cel.Variable("fault.Guardrail", cel.MapType(cel.StringType, cel.DynType)),
	)
}

// faultEvalVars flattens the failure into the activation keys the error.* variables read.
//
// Every activation gets these, not just the fault one: a variable declared in the environment
// but absent from the activation is an EVALUATION error rather than a false, so a condition
// mentioning fault.Type on a response policy would fail at request time. Zero values make it
// read false, which is what it means.
//
// Fault.Description is deliberately absent: it carries the content a guardrail blocked. A
// condition that needs to know WHICH guardrail acted reads fault.Guardrail.InterveningGuardrail.
func faultEvalVars(ctx *policy.FaultContext) map[string]interface{} {
	var (
		src, code, faultType, direction, message string
		policyName, policyVersion, policyPhase   string
		routeKey                                 string
		status, originalStatus                   int
		committed                                bool
		guardrail                                map[string]interface{}
	)
	if ctx != nil {
		src, routeKey = ctx.Source, ctx.RouteKey
		policyName, policyVersion, policyPhase = ctx.Policy, ctx.PolicyVersion, ctx.PolicyPhase
		status, originalStatus, committed = ctx.ResponseStatus, ctx.OriginalStatus, ctx.ResponseCommitted
		if e := ctx.Fault; e != nil {
			code, faultType, direction, message = e.Code, e.Type, e.Direction, e.Message
			if g := e.Guardrail; g != nil {
				guardrail = map[string]interface{}{
					"InterveningGuardrail": g.InterveningGuardrail,
					"Action":               g.Action,
					"ActionReason":         g.ActionReason,
				}
			}
		}
	}
	// Always a map, never nil: a nil would make fault.Guardrail.InterveningGuardrail an
	// evaluation error on every non-guardrail failure, and the natural way to write that
	// condition is to test the field directly.
	if guardrail == nil {
		guardrail = map[string]interface{}{
			"InterveningGuardrail": "", "Action": "", "ActionReason": "",
		}
	}

	flat := map[string]interface{}{
		"Source":            src,
		"Code":              code,
		"Type":              faultType,
		"Direction":         direction,
		"Message":           message,
		"Policy":            policyName,
		"PolicyVersion":     policyVersion,
		"PolicyPhase":       policyPhase,
		"RouteKey":          routeKey,
		"Status":            status,
		"OriginalStatus":    originalStatus,
		"ResponseCommitted": committed,
		"Guardrail":         guardrail,
	}
	vars := map[string]interface{}{"fault": flat}
	for k, v := range flat {
		vars["fault."+k] = v
	}
	return vars
}

// withFaultVars merges the fault.* activation keys into an existing activation.
func withFaultVars(activation map[string]interface{}, ctx *policy.FaultContext) map[string]interface{} {
	for k, v := range faultEvalVars(ctx) {
		activation[k] = v
	}
	return activation
}

// EvaluateRequestHeaderCondition evaluates a CEL expression against a RequestHeaderContext
func (e *celEvaluator) EvaluateRequestHeaderCondition(expression string, ctx *policy.RequestHeaderContext) (bool, error) {
	program, err := e.getOrCompileProgram(expression)
	if err != nil {
		return false, fmt.Errorf("failed to compile CEL expression: %w", err)
	}
	return e.eval(program, buildRequestHeaderEvalCtx(ctx, "request_headers"))
}

// EvaluateRequestBodyCondition evaluates a CEL expression against a RequestContext
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

// EvaluateResponseBodyCondition evaluates a CEL expression against a ResponseContext
func (e *celEvaluator) EvaluateResponseBodyCondition(expression string, ctx *policy.ResponseContext) (bool, error) {
	program, err := e.getOrCompileProgram(expression)
	if err != nil {
		return false, fmt.Errorf("failed to compile CEL expression: %w", err)
	}
	return e.eval(program, buildResponseBodyEvalCtx(ctx, "response_body"))
}

// EvaluateFaultCondition evaluates a CEL expression against an FaultContext, for a fault
// policy's executionCondition.
//
// The request/response half of the activation is identical to the response-body one, phase
// included, so a condition means the same thing on a response policy or a fault policy.
//
// On top of that it supplies the fault.* variables, which let a condition narrow on the
// failure itself — `fault.Source == "gateway"` to skip the backend's own errors. Every other
// phase supplies the same variables with zero values; see faultEvalVars.
func (e *celEvaluator) EvaluateFaultCondition(expression string, ctx *policy.FaultContext) (bool, error) {
	program, err := e.getOrCompileProgram(expression)
	if err != nil {
		return false, fmt.Errorf("failed to compile CEL expression: %w", err)
	}
	return e.eval(program, buildErrorEvalCtx(ctx))
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
	return withFaultVars(map[string]interface{}{
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
	}, nil)
}

// buildRequestBodyEvalCtx builds a CEL evaluation context from a RequestContext
func buildRequestBodyEvalCtx(ctx *policy.RequestContext, phase string) map[string]interface{} {
	headers := ctx.Headers.GetAll()
	body := bodyToCEL(ctx.Body)
	return withFaultVars(map[string]interface{}{
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
	}, nil)
}

// buildResponseHeaderEvalCtx builds a CEL evaluation context from a ResponseHeaderContext
func buildResponseHeaderEvalCtx(ctx *policy.ResponseHeaderContext, phase string) map[string]interface{} {
	requestHeaders := ctx.RequestHeaders.GetAll()
	requestBody := bodyToCEL(ctx.RequestBody)
	responseHeaders := ctx.ResponseHeaders.GetAll()
	return withFaultVars(map[string]interface{}{
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
	}, nil)
}

// buildResponseBodyEvalCtx builds a CEL evaluation context from a ResponseContext
func buildResponseBodyEvalCtx(ctx *policy.ResponseContext, phase string) map[string]interface{} {
	requestHeaders := ctx.RequestHeaders.GetAll()
	requestBody := bodyToCEL(ctx.RequestBody)
	responseHeaders := ctx.ResponseHeaders.GetAll()
	responseBody := bodyToCEL(ctx.ResponseBody)
	return withFaultVars(map[string]interface{}{
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
	}, nil)
}

// buildErrorEvalCtx builds a CEL evaluation context from an FaultContext.
//
// Emits every key buildResponseBodyEvalCtx does, from the same-named fields, so a condition
// written for a response policy means the same thing here. Kept as its own function rather
// than a conversion into ResponseContext: converting would allocate a throwaway view on every
// conditional fault entry, which is the cost this whole change removes.
//
// It additionally emits the error.* keys with the failure's real values — this is the one
// activation where they are not zero.
func buildErrorEvalCtx(ctx *policy.FaultContext) map[string]interface{} {
	requestHeaders := ctx.RequestHeaders.GetAll()
	requestBody := bodyToCEL(ctx.RequestBody)
	responseHeaders := ctx.ResponseHeaders.GetAll()
	responseBody := bodyToCEL(ctx.ResponseBody)

	// Metadata and RequestID live on the EMBEDDED *SharedContext, so reading them through
	// ctx dereferences a pointer the type system lets be nil — and the kernel treats it as
	// nil-able too (executeFaultPolicies guards ec.sharedCtx before reading APIName).
	//
	// Guarded rather than assumed non-nil because of where this runs: a panic here is a
	// panic inside the fault flow, on a request that is ALREADY failing. The one thing the
	// fault flow must never do is turn a served error into a dropped one, and a nil
	// dereference in a condition evaluation would do exactly that — for every request on
	// the route, since the condition is attached to the entry rather than to the failure.
	var metadata map[string]interface{}
	var requestID string
	if ctx.SharedContext != nil {
		metadata = ctx.Metadata
		requestID = ctx.RequestID
	}
	return withFaultVars(map[string]interface{}{
		"processing.phase": "response_body",
		"request": map[string]interface{}{
			"Headers":   requestHeaders,
			"Body":      requestBody,
			"Path":      ctx.RequestPath,
			"Method":    ctx.RequestMethod,
			"RequestID": requestID,
			"Metadata":  metadata,
		},
		"request.Headers":   requestHeaders,
		"request.Body":      requestBody,
		"request.Path":      ctx.RequestPath,
		"request.Method":    ctx.RequestMethod,
		"request.RequestID": requestID,
		"request.Metadata":  metadata,
		"response": map[string]interface{}{
			"RequestHeaders":  requestHeaders,
			"RequestBody":     requestBody,
			"RequestPath":     ctx.RequestPath,
			"RequestMethod":   ctx.RequestMethod,
			"ResponseHeaders": responseHeaders,
			"ResponseBody":    responseBody,
			"ResponseStatus":  ctx.ResponseStatus,
			"RequestID":       requestID,
			"Metadata":        metadata,
		},
		"response.RequestHeaders":  requestHeaders,
		"response.RequestBody":     requestBody,
		"response.RequestPath":     ctx.RequestPath,
		"response.RequestMethod":   ctx.RequestMethod,
		"response.ResponseHeaders": responseHeaders,
		"response.ResponseBody":    responseBody,
		"response.ResponseStatus":  ctx.ResponseStatus,
		"response.RequestID":       requestID,
		"response.Metadata":        metadata,
	}, ctx)
}

// buildStreamingRequestEvalCtx builds a CEL evaluation context from a RequestStreamContext.
// Uses phase "request_body" — consistent with buffered request body processing.
func buildStreamingRequestEvalCtx(ctx *policy.RequestStreamContext) map[string]interface{} {
	headers := ctx.Headers.GetAll()
	return withFaultVars(map[string]interface{}{
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
	}, nil)
}

// buildStreamingResponseEvalCtx builds a CEL evaluation context from a ResponseStreamContext.
// Uses phase "response_body" — consistent with buffered response body processing.
func buildStreamingResponseEvalCtx(ctx *policy.ResponseStreamContext) map[string]interface{} {
	requestHeaders := ctx.RequestHeaders.GetAll()
	requestBody := bodyToCEL(ctx.RequestBody)
	responseHeaders := ctx.ResponseHeaders.GetAll()
	return withFaultVars(map[string]interface{}{
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
	}, nil)
}

// eval evaluates a compiled program against an evaluation context
func (e *celEvaluator) eval(program cel.Program, evalCtx map[string]interface{}) (bool, error) {
	result, _, err := program.Eval(evalCtx)
	if err != nil {
		return false, fmt.Errorf("CEL evaluation failed: %w", err)
	}
	boolResult, ok := result.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression must return boolean, got %T", result.Value())
	}
	return boolResult, nil
}

// getOrCompileProgram returns a cached compiled program, or compiles and caches it.
// A plain Mutex (not RWMutex) is required because an LRU hit promotes the entry,
// which mutates the cache even on a read path.
func (e *celEvaluator) getOrCompileProgram(expression string) (cel.Program, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if program, ok := e.programCache.get(expression); ok {
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

	e.programCache.put(expression, program)
	return program, nil
}

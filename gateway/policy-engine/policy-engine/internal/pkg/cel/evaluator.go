package cel

import (
	"fmt"
	"sync"

	"github.com/google/cel-go/cel"

	"github.com/policy-engine/sdk/policies"
)

// CELEvaluator provides CEL expression evaluation for RequestContext and ResponseContext
type CELEvaluator interface {
	// EvaluateRequestCondition evaluates a CEL expression against RequestContext
	// Returns true if condition passes, false if it fails
	EvaluateRequestCondition(expression string, ctx *policies.RequestContext) (bool, error)

	// EvaluateResponseCondition evaluates a CEL expression against ResponseContext
	// Returns true if condition passes, false if it fails
	EvaluateResponseCondition(expression string, ctx *policies.ResponseContext) (bool, error)
}

// celEvaluator implements CELEvaluator with caching
type celEvaluator struct {
	mu sync.RWMutex

	// Compiled CEL programs cache
	// Key: expression string, Value: compiled cel.Program
	requestProgramCache  map[string]cel.Program
	responseProgramCache map[string]cel.Program

	// CEL environments for request and response contexts
	requestEnv  *cel.Env
	responseEnv *cel.Env
}

// NewCELEvaluator creates a new CEL evaluator with caching
func NewCELEvaluator() (CELEvaluator, error) {
	// Create request context environment
	requestEnv, err := createRequestEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to create request CEL environment: %w", err)
	}

	// Create response context environment
	responseEnv, err := createResponseEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to create response CEL environment: %w", err)
	}

	return &celEvaluator{
		requestProgramCache:  make(map[string]cel.Program),
		responseProgramCache: make(map[string]cel.Program),
		requestEnv:           requestEnv,
		responseEnv:          responseEnv,
	}, nil
}

// createRequestEnv creates a CEL environment for RequestContext evaluation
func createRequestEnv() (*cel.Env, error) {
	return cel.NewEnv(
		// RequestContext variables
		cel.Variable("request", cel.ObjectType("RequestContext")),
		cel.Variable("request.Headers", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable("request.Body", cel.MapType(cel.StringType, cel.DynType)), // Body struct with Content, EndOfStream, Present
		cel.Variable("request.Path", cel.StringType),
		cel.Variable("request.Method", cel.StringType),
		cel.Variable("request.RequestID", cel.StringType),
		cel.Variable("request.Metadata", cel.MapType(cel.StringType, cel.DynType)),
	)
}

// createResponseEnv creates a CEL environment for ResponseContext evaluation
func createResponseEnv() (*cel.Env, error) {
	return cel.NewEnv(
		// ResponseContext variables
		cel.Variable("response", cel.ObjectType("ResponseContext")),
		cel.Variable("response.RequestHeaders", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable("response.RequestBody", cel.MapType(cel.StringType, cel.DynType)), // Body struct with Content, EndOfStream, Present
		cel.Variable("response.RequestPath", cel.StringType),
		cel.Variable("response.RequestMethod", cel.StringType),
		cel.Variable("response.ResponseHeaders", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable("response.ResponseBody", cel.MapType(cel.StringType, cel.DynType)), // Body struct with Content, EndOfStream, Present
		cel.Variable("response.ResponseStatus", cel.IntType),
		cel.Variable("response.RequestID", cel.StringType),
		cel.Variable("response.Metadata", cel.MapType(cel.StringType, cel.DynType)),
	)
}

// EvaluateRequestCondition evaluates a CEL expression against RequestContext
func (e *celEvaluator) EvaluateRequestCondition(expression string, ctx *policies.RequestContext) (bool, error) {
	// Get or compile program
	program, err := e.getOrCompileRequestProgram(expression)
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

	// Build evaluation context
	evalCtx := map[string]interface{}{
		"request": map[string]interface{}{
			"Headers":   ctx.Headers,
			"Body":      bodyForCEL,
			"Path":      ctx.Path,
			"Method":    ctx.Method,
			"RequestID": ctx.RequestID,
			"Metadata":  ctx.Metadata,
		},
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
func (e *celEvaluator) EvaluateResponseCondition(expression string, ctx *policies.ResponseContext) (bool, error) {
	// Get or compile program
	program, err := e.getOrCompileResponseProgram(expression)
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

	// Build evaluation context
	evalCtx := map[string]interface{}{
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

// getOrCompileRequestProgram gets cached program or compiles new one for request context
func (e *celEvaluator) getOrCompileRequestProgram(expression string) (cel.Program, error) {
	// Check cache first (read lock)
	e.mu.RLock()
	if program, ok := e.requestProgramCache[expression]; ok {
		e.mu.RUnlock()
		return program, nil
	}
	e.mu.RUnlock()

	// Compile (write lock)
	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check after acquiring write lock
	if program, ok := e.requestProgramCache[expression]; ok {
		return program, nil
	}

	// Compile expression
	ast, issues := e.requestEnv.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compilation failed: %w", issues.Err())
	}

	// Create program
	program, err := e.requestEnv.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program creation failed: %w", err)
	}

	// Cache and return
	e.requestProgramCache[expression] = program
	return program, nil
}

// getOrCompileResponseProgram gets cached program or compiles new one for response context
func (e *celEvaluator) getOrCompileResponseProgram(expression string) (cel.Program, error) {
	// Check cache first (read lock)
	e.mu.RLock()
	if program, ok := e.responseProgramCache[expression]; ok {
		e.mu.RUnlock()
		return program, nil
	}
	e.mu.RUnlock()

	// Compile (write lock)
	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check after acquiring write lock
	if program, ok := e.responseProgramCache[expression]; ok {
		return program, nil
	}

	// Compile expression
	ast, issues := e.responseEnv.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compilation failed: %w", issues.Err())
	}

	// Create program
	program, err := e.responseEnv.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program creation failed: %w", err)
	}

	// Cache and return
	e.responseProgramCache[expression] = program
	return program, nil
}

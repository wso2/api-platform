package ratelimit

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/cel-go/cel"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// CELEvaluator provides CEL expression evaluation for rate limit key and cost extraction
type CELEvaluator struct {
	mu sync.RWMutex

	// Compiled CEL programs cache
	// Key: expression string, Value: compiled cel.Program
	programCache map[string]cel.Program

	// CEL environment for key extraction (returns string)
	keyEnv *cel.Env

	// CEL environment for cost extraction (returns numeric)
	costEnv *cel.Env
}

// globalCELEvaluator is a singleton CEL evaluator instance
var (
	globalCELEvaluator *CELEvaluator
	celEvaluatorOnce   sync.Once
	celInitErr         error
)

// GetCELEvaluator returns the singleton CEL evaluator instance
func GetCELEvaluator() (*CELEvaluator, error) {
	celEvaluatorOnce.Do(func() {
		evaluator, err := newCELEvaluator()
		if err != nil {
			celInitErr = err
			return
		}
		globalCELEvaluator = evaluator
	})
	if celInitErr != nil {
		return nil, celInitErr
	}
	return globalCELEvaluator, nil
}

// newCELEvaluator creates a new CEL evaluator with environments for both key and cost extraction
func newCELEvaluator() (*CELEvaluator, error) {
	keyEnv, err := createKeyExtractionEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to create key extraction CEL environment: %w", err)
	}

	costEnv, err := createCostExtractionEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to create cost extraction CEL environment: %w", err)
	}

	return &CELEvaluator{
		programCache: make(map[string]cel.Program),
		keyEnv:       keyEnv,
		costEnv:      costEnv,
	}, nil
}

// createKeyExtractionEnv creates a CEL environment for key extraction expressions
// Key extraction expressions must return a string
func createKeyExtractionEnv() (*cel.Env, error) {
	return cel.NewEnv(
		// Request context variables
		cel.Variable("request.Headers", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable("request.Path", cel.StringType),
		cel.Variable("request.Method", cel.StringType),
		cel.Variable("request.Metadata", cel.MapType(cel.StringType, cel.DynType)),
		// API context variables
		cel.Variable("api.Name", cel.StringType),
		cel.Variable("api.Version", cel.StringType),
		cel.Variable("api.Context", cel.StringType),
		cel.Variable("api.Id", cel.StringType),
		// Route info
		cel.Variable("route.Name", cel.StringType),
	)
}

// createCostExtractionEnv creates a CEL environment for cost extraction expressions
// Cost extraction expressions must return a numeric value (int or double)
func createCostExtractionEnv() (*cel.Env, error) {
	return cel.NewEnv(
		// Request context variables (for request_cel type)
		cel.Variable("request.Headers", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable("request.Body", cel.BytesType),
		cel.Variable("request.BodyString", cel.StringType),
		cel.Variable("request.Path", cel.StringType),
		cel.Variable("request.Method", cel.StringType),
		cel.Variable("request.Metadata", cel.MapType(cel.StringType, cel.DynType)),
		// Response context variables (for response_cel type)
		cel.Variable("response.Headers", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable("response.Body", cel.BytesType),
		cel.Variable("response.BodyString", cel.StringType),
		cel.Variable("response.Status", cel.IntType),
		// API context variables
		cel.Variable("api.Name", cel.StringType),
		cel.Variable("api.Version", cel.StringType),
		cel.Variable("api.Context", cel.StringType),
		cel.Variable("api.Id", cel.StringType),
	)
}

// EvaluateKeyExpression evaluates a CEL expression for key extraction from request context
// Returns the extracted key string or an error
func (e *CELEvaluator) EvaluateKeyExpression(expression string, ctx *policy.RequestContext, routeName string) (string, error) {
	program, err := e.getOrCompileKeyProgram(expression)
	if err != nil {
		return "", fmt.Errorf("failed to compile CEL expression: %w", err)
	}

	// Build evaluation context
	evalCtx := buildKeyEvalContext(ctx, routeName)

	// Evaluate
	result, _, err := program.Eval(evalCtx)
	if err != nil {
		slog.Debug("CEL key extraction evaluation failed", "expression", expression, "error", err)
		return "", fmt.Errorf("CEL evaluation failed: %w", err)
	}

	// Convert to string
	strResult, ok := result.Value().(string)
	if !ok {
		return "", fmt.Errorf("CEL expression must return string, got %T", result.Value())
	}

	return strResult, nil
}

// EvaluateRequestCostExpression evaluates a CEL expression for cost extraction from request context
// Returns the extracted cost value or an error
func (e *CELEvaluator) EvaluateRequestCostExpression(expression string, ctx *policy.RequestContext) (float64, error) {
	program, err := e.getOrCompileCostProgram(expression)
	if err != nil {
		return 0, fmt.Errorf("failed to compile CEL expression: %w", err)
	}

	// Build evaluation context for request phase
	evalCtx := buildRequestCostEvalContext(ctx)

	// Evaluate
	result, _, err := program.Eval(evalCtx)
	if err != nil {
		slog.Debug("CEL request cost extraction evaluation failed", "expression", expression, "error", err)
		return 0, fmt.Errorf("CEL evaluation failed: %w", err)
	}

	// Convert to float64
	return toFloat64(result.Value())
}

// EvaluateResponseCostExpression evaluates a CEL expression for cost extraction from response context
// Returns the extracted cost value or an error
func (e *CELEvaluator) EvaluateResponseCostExpression(expression string, ctx *policy.ResponseContext) (float64, error) {
	program, err := e.getOrCompileCostProgram(expression)
	if err != nil {
		return 0, fmt.Errorf("failed to compile CEL expression: %w", err)
	}

	// Build evaluation context for response phase
	evalCtx := buildResponseCostEvalContext(ctx)

	// Evaluate
	result, _, err := program.Eval(evalCtx)
	if err != nil {
		slog.Debug("CEL response cost extraction evaluation failed", "expression", expression, "error", err)
		return 0, fmt.Errorf("CEL evaluation failed: %w", err)
	}

	// Convert to float64
	return toFloat64(result.Value())
}

// buildKeyEvalContext builds the CEL evaluation context for key extraction
func buildKeyEvalContext(ctx *policy.RequestContext, routeName string) map[string]interface{} {
	// Convert headers to map[string][]string for CEL
	headers := make(map[string][]string)
	if ctx.Headers != nil {
		ctx.Headers.Iterate(func(key string, values []string) {
			headers[key] = values
		})
	}

	// Build metadata map
	metadata := make(map[string]interface{})
	if ctx.Metadata != nil {
		for k, v := range ctx.Metadata {
			metadata[k] = v
		}
	}

	return map[string]interface{}{
		"request.Headers":  headers,
		"request.Path":     ctx.Path,
		"request.Method":   ctx.Method,
		"request.Metadata": metadata,
		"api.Name":         ctx.APIName,
		"api.Version":      ctx.APIVersion,
		"api.Context":      ctx.APIContext,
		"api.Id":           ctx.APIId,
		"route.Name":       routeName,
	}
}

// buildRequestCostEvalContext builds the CEL evaluation context for request-phase cost extraction
func buildRequestCostEvalContext(ctx *policy.RequestContext) map[string]interface{} {
	// Convert headers to map[string][]string for CEL
	headers := make(map[string][]string)
	if ctx.Headers != nil {
		ctx.Headers.Iterate(func(key string, values []string) {
			headers[key] = values
		})
	}

	// Build metadata map
	metadata := make(map[string]interface{})
	if ctx.Metadata != nil {
		for k, v := range ctx.Metadata {
			metadata[k] = v
		}
	}

	// Get body content
	var bodyBytes []byte
	var bodyString string
	if ctx.Body != nil && ctx.Body.Present && ctx.Body.Content != nil {
		bodyBytes = ctx.Body.Content
		bodyString = string(bodyBytes)
	}

	return map[string]interface{}{
		"request.Headers":    headers,
		"request.Body":       bodyBytes,
		"request.BodyString": bodyString,
		"request.Path":       ctx.Path,
		"request.Method":     ctx.Method,
		"request.Metadata":   metadata,
		// Response variables are empty during request phase
		"response.Headers":    map[string][]string{},
		"response.Body":       []byte{},
		"response.BodyString": "",
		"response.Status":     int64(0),
		"api.Name":            ctx.APIName,
		"api.Version":         ctx.APIVersion,
		"api.Context":         ctx.APIContext,
		"api.Id":              ctx.APIId,
	}
}

// buildResponseCostEvalContext builds the CEL evaluation context for response-phase cost extraction
func buildResponseCostEvalContext(ctx *policy.ResponseContext) map[string]interface{} {
	// Convert request headers to map[string][]string for CEL
	requestHeaders := make(map[string][]string)
	if ctx.RequestHeaders != nil {
		ctx.RequestHeaders.Iterate(func(key string, values []string) {
			requestHeaders[key] = values
		})
	}

	// Convert response headers to map[string][]string for CEL
	responseHeaders := make(map[string][]string)
	if ctx.ResponseHeaders != nil {
		ctx.ResponseHeaders.Iterate(func(key string, values []string) {
			responseHeaders[key] = values
		})
	}

	// Build metadata map
	metadata := make(map[string]interface{})
	if ctx.Metadata != nil {
		for k, v := range ctx.Metadata {
			metadata[k] = v
		}
	}

	// Get request body content
	var requestBodyBytes []byte
	var requestBodyString string
	if ctx.RequestBody != nil && ctx.RequestBody.Present && ctx.RequestBody.Content != nil {
		requestBodyBytes = ctx.RequestBody.Content
		requestBodyString = string(requestBodyBytes)
	}

	// Get response body content
	var responseBodyBytes []byte
	var responseBodyString string
	if ctx.ResponseBody != nil && ctx.ResponseBody.Present && ctx.ResponseBody.Content != nil {
		responseBodyBytes = ctx.ResponseBody.Content
		responseBodyString = string(responseBodyBytes)
	}

	return map[string]interface{}{
		"request.Headers":     requestHeaders,
		"request.Body":        requestBodyBytes,
		"request.BodyString":  requestBodyString,
		"request.Path":        ctx.RequestPath,
		"request.Method":      ctx.RequestMethod,
		"request.Metadata":    metadata,
		"response.Headers":    responseHeaders,
		"response.Body":       responseBodyBytes,
		"response.BodyString": responseBodyString,
		"response.Status":     int64(ctx.ResponseStatus),
		"api.Name":            ctx.APIName,
		"api.Version":         ctx.APIVersion,
		"api.Context":         ctx.APIContext,
		"api.Id":              ctx.APIId,
	}
}

// toFloat64 converts a CEL result value to float64
func toFloat64(val interface{}) (float64, error) {
	switch v := val.(type) {
	case int64:
		return float64(v), nil
	case int:
		return float64(v), nil
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("CEL expression must return numeric value, got %T", val)
	}
}

// getOrCompileKeyProgram gets a cached program or compiles a new one for key extraction
func (e *CELEvaluator) getOrCompileKeyProgram(expression string) (cel.Program, error) {
	cacheKey := "key:" + expression

	// Check cache first (read lock)
	e.mu.RLock()
	if program, ok := e.programCache[cacheKey]; ok {
		e.mu.RUnlock()
		return program, nil
	}
	e.mu.RUnlock()

	// Compile (write lock)
	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check after acquiring write lock
	if program, ok := e.programCache[cacheKey]; ok {
		return program, nil
	}

	// Compile expression
	ast, issues := e.keyEnv.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compilation failed: %w", issues.Err())
	}

	// Create program
	program, err := e.keyEnv.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program creation failed: %w", err)
	}

	// Cache and return
	e.programCache[cacheKey] = program
	return program, nil
}

// getOrCompileCostProgram gets a cached program or compiles a new one for cost extraction
func (e *CELEvaluator) getOrCompileCostProgram(expression string) (cel.Program, error) {
	cacheKey := "cost:" + expression

	// Check cache first (read lock)
	e.mu.RLock()
	if program, ok := e.programCache[cacheKey]; ok {
		e.mu.RUnlock()
		return program, nil
	}
	e.mu.RUnlock()

	// Compile (write lock)
	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check after acquiring write lock
	if program, ok := e.programCache[cacheKey]; ok {
		return program, nil
	}

	// Compile expression
	ast, issues := e.costEnv.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compilation failed: %w", issues.Err())
	}

	// Create program
	program, err := e.costEnv.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program creation failed: %w", err)
	}

	// Cache and return
	e.programCache[cacheKey] = program
	return program, nil
}

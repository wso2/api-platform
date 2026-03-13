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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/testutils"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// =============================================================================
// NewCELEvaluator Tests
// =============================================================================

func TestNewCELEvaluator(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)
	assert.NotNil(t, evaluator)
}

// =============================================================================
// EvaluateRequestHeaderCondition Tests
// =============================================================================

func TestEvaluateRequestHeaderCondition_SimpleExpressions(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestHeaderContext()

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "True literal",
			expression: `true`,
			expected:   true,
		},
		{
			name:       "False literal",
			expression: `false`,
			expected:   false,
		},
		{
			name:       "Method equals GET",
			expression: `request.Method == "GET"`,
			expected:   true,
		},
		{
			name:       "Method equals POST - false",
			expression: `request.Method == "POST"`,
			expected:   false,
		},
		{
			name:       "Path starts with /api",
			expression: `request.Path.startsWith("/api")`,
			expected:   true,
		},
		{
			name:       "Path starts with /other - false",
			expression: `request.Path.startsWith("/other")`,
			expected:   false,
		},
		{
			name:       "Processing phase is request_headers",
			expression: `processing.phase == "request_headers"`,
			expected:   true,
		},
		{
			name:       "Processing phase is not response_headers",
			expression: `processing.phase == "response_headers"`,
			expected:   false,
		},
		{
			name:       "Path contains v1",
			expression: `request.Path.contains("v1")`,
			expected:   true,
		},
		{
			name:       "Request ID not empty",
			expression: `request.RequestID != ""`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateRequestHeaderCondition(tt.expression, reqCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateRequestHeaderCondition_ComplexExpressions(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestHeaderContext()

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "AND expression - method and path",
			expression: `request.Method == "GET" && request.Path.startsWith("/api")`,
			expected:   true,
		},
		{
			name:       "AND expression - one false",
			expression: `request.Method == "GET" && request.Path.startsWith("/other")`,
			expected:   false,
		},
		{
			name:       "OR expression - first true",
			expression: `request.Method == "GET" || request.Method == "POST"`,
			expected:   true,
		},
		{
			name:       "OR expression - second true",
			expression: `request.Method == "POST" || request.Method == "GET"`,
			expected:   true,
		},
		{
			name:       "OR expression - both false",
			expression: `request.Method == "POST" || request.Method == "PUT"`,
			expected:   false,
		},
		{
			name:       "Nested expression",
			expression: `(request.Method == "GET" || request.Method == "POST") && request.Path.startsWith("/api")`,
			expected:   true,
		},
		{
			name:       "Three conditions with AND",
			expression: `request.Method == "GET" && request.Path.startsWith("/api") && processing.phase == "request_headers"`,
			expected:   true,
		},
		{
			name:       "Five conditions with OR",
			expression: `request.Method == "GET" || request.Method == "POST" || request.Method == "PUT" || request.Method == "DELETE" || request.Method == "PATCH"`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateRequestHeaderCondition(tt.expression, reqCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateRequestHeaderCondition_InvalidExpression(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestHeaderContext()

	tests := []struct {
		name       string
		expression string
	}{
		{
			name:       "Syntax error - incomplete expression",
			expression: `request.Method ==`,
		},
		{
			name:       "Unknown variable",
			expression: `unknownVar == "test"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evaluator.EvaluateRequestHeaderCondition(tt.expression, reqCtx)
			assert.Error(t, err)
		})
	}
}

func TestEvaluateRequestHeaderCondition_NonBooleanResult(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestHeaderContext()

	// Expression that returns string instead of boolean
	_, err = evaluator.EvaluateRequestHeaderCondition(`request.Method`, reqCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must return boolean")
}

// =============================================================================
// EvaluateResponseHeaderCondition Tests
// =============================================================================

func TestEvaluateResponseHeaderCondition_SimpleExpressions(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	respCtx := testutils.NewTestResponseHeaderContext()

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "Status equals 200",
			expression: `response.ResponseStatus == 200`,
			expected:   true,
		},
		{
			name:       "Status equals 404 - false",
			expression: `response.ResponseStatus == 404`,
			expected:   false,
		},
		{
			name:       "Status >= 200",
			expression: `response.ResponseStatus >= 200`,
			expected:   true,
		},
		{
			name:       "Status < 300",
			expression: `response.ResponseStatus < 300`,
			expected:   true,
		},
		{
			name:       "Success status range",
			expression: `response.ResponseStatus >= 200 && response.ResponseStatus < 300`,
			expected:   true,
		},
		{
			name:       "Error status - false",
			expression: `response.ResponseStatus >= 400`,
			expected:   false,
		},
		{
			name:       "Processing phase is response_headers",
			expression: `processing.phase == "response_headers"`,
			expected:   true,
		},
		{
			name:       "Processing phase is not request_headers",
			expression: `processing.phase == "request_headers"`,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateResponseHeaderCondition(tt.expression, respCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateResponseHeaderCondition_CrossPhaseAccess(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	respCtx := testutils.NewTestResponseHeaderContext()

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "Access request method in response phase",
			expression: `response.RequestMethod == "GET"`,
			expected:   true,
		},
		{
			name:       "Access request path in response phase",
			expression: `response.RequestPath.startsWith("/api")`,
			expected:   true,
		},
		{
			name:       "Cross-phase combined condition",
			expression: `response.RequestMethod == "GET" && response.ResponseStatus == 200`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateResponseHeaderCondition(tt.expression, respCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateResponseHeaderCondition_StatusRanges(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	tests := []struct {
		name       string
		status     int
		expression string
		expected   bool
	}{
		{
			name:       "2xx check with 200",
			status:     200,
			expression: `response.ResponseStatus >= 200 && response.ResponseStatus < 300`,
			expected:   true,
		},
		{
			name:       "2xx check with 201",
			status:     201,
			expression: `response.ResponseStatus >= 200 && response.ResponseStatus < 300`,
			expected:   true,
		},
		{
			name:       "4xx check with 400",
			status:     400,
			expression: `response.ResponseStatus >= 400 && response.ResponseStatus < 500`,
			expected:   true,
		},
		{
			name:       "4xx check with 404",
			status:     404,
			expression: `response.ResponseStatus >= 400 && response.ResponseStatus < 500`,
			expected:   true,
		},
		{
			name:       "5xx check with 500",
			status:     500,
			expression: `response.ResponseStatus >= 500 && response.ResponseStatus < 600`,
			expected:   true,
		},
		{
			name:       "5xx check with 503",
			status:     503,
			expression: `response.ResponseStatus >= 500 && response.ResponseStatus < 600`,
			expected:   true,
		},
		{
			name:       "Not error (< 400)",
			status:     200,
			expression: `response.ResponseStatus < 400`,
			expected:   true,
		},
		{
			name:       "Is error (>= 400)",
			status:     500,
			expression: `response.ResponseStatus >= 400`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respCtx := testutils.NewTestResponseHeaderContext()
			respCtx.ResponseStatus = tt.status
			result, err := evaluator.EvaluateResponseHeaderCondition(tt.expression, respCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateResponseHeaderCondition_InvalidExpression(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	respCtx := testutils.NewTestResponseHeaderContext()

	_, err = evaluator.EvaluateResponseHeaderCondition(`response.ResponseStatus ==`, respCtx)
	assert.Error(t, err)
}

func TestEvaluateResponseHeaderCondition_NonBooleanResult(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	respCtx := testutils.NewTestResponseHeaderContext()

	_, err = evaluator.EvaluateResponseHeaderCondition(`response.ResponseStatus`, respCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must return boolean")
}

// =============================================================================
// Caching Tests
// =============================================================================

func TestCELEvaluator_ProgramCaching(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestHeaderContext()
	expression := `request.Method == "GET"`

	// First evaluation - compiles the program
	result1, err := evaluator.EvaluateRequestHeaderCondition(expression, reqCtx)
	require.NoError(t, err)
	assert.True(t, result1)

	// Second evaluation - should use cached program
	result2, err := evaluator.EvaluateRequestHeaderCondition(expression, reqCtx)
	require.NoError(t, err)
	assert.True(t, result2)

	// Both results should be the same
	assert.Equal(t, result1, result2)
}

func TestCELEvaluator_DifferentExpressions(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestHeaderContext()

	expressions := []string{
		`request.Method == "GET"`,
		`request.Method == "POST"`,
		`request.Path.startsWith("/api")`,
		`processing.phase == "request_headers"`,
	}

	// Evaluate each expression
	for _, expr := range expressions {
		_, err := evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		require.NoError(t, err, "Expression should evaluate without error: %s", expr)
	}
}

func TestCELEvaluator_ConcurrentAccess(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	// Use expressions that don't involve Headers (which requires conversion)
	expressions := []string{
		`request.Method == "GET"`,
		`request.Method == "POST"`,
		`request.Path.startsWith("/api")`,
		`processing.phase == "request_headers"`,
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 10; i++ {
		for _, expr := range expressions {
			wg.Add(1)
			go func(e string) {
				defer wg.Done()
				ctx := testutils.NewTestRequestHeaderContext()
				_, err := evaluator.EvaluateRequestHeaderCondition(e, ctx)
				if err != nil {
					errors <- err
				}
			}(expr)
		}
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent evaluation error: %v", err)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestEvaluateResponseHeaderCondition_ZeroStatus(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	respCtx := testutils.NewTestResponseHeaderContext()
	respCtx.ResponseStatus = 0

	result, err := evaluator.EvaluateResponseHeaderCondition(`response.ResponseStatus == 0`, respCtx)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluateRequestHeaderCondition_EmptyPath(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestHeaderContext()
	reqCtx.Path = ""

	result, err := evaluator.EvaluateRequestHeaderCondition(`request.Path == ""`, reqCtx)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluateRequestHeaderCondition_PathEndsWith(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestHeaderContext()

	result, err := evaluator.EvaluateRequestHeaderCondition(`request.Path.endsWith("123")`, reqCtx)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluateRequestHeaderCondition_PathSize(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestHeaderContext()

	result, err := evaluator.EvaluateRequestHeaderCondition(`size(request.Path) > 0`, reqCtx)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluateRequestHeaderCondition_MethodComparison(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			reqCtx := testutils.NewTestRequestHeaderContext()
			reqCtx.Method = method

			result, err := evaluator.EvaluateRequestHeaderCondition(`request.Method == "`+method+`"`, reqCtx)
			require.NoError(t, err)
			assert.True(t, result)
		})
	}
}

// =============================================================================
// Header Operation Tests
// =============================================================================

func TestEvaluateRequestHeaderCondition_HeaderOperations(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	// Create context with headers
	ctx := testutils.NewTestRequestHeaderContextWithHeaders(map[string][]string{
		"authorization": {"Bearer token123"},
		"content-type":  {"application/json"},
		"x-api-key":     {"sk-test-key"},
		"user-agent":    {"test-agent/1.0"},
	})
	ctx.Method = "POST"
	ctx.Path = "/api/v1/test"

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "header exists - authorization present",
			expression: `"authorization" in request.Headers`,
			expected:   true,
		},
		{
			name:       "header exists - x-api-key present",
			expression: `"x-api-key" in request.Headers`,
			expected:   true,
		},
		{
			name:       "header exists - missing header",
			expression: `"x-missing-header" in request.Headers`,
			expected:   false,
		},
		{
			name:       "header value check - exact match",
			expression: `request.Headers["content-type"][0] == "application/json"`,
			expected:   true,
		},
		{
			name:       "header value check - wrong value",
			expression: `request.Headers["content-type"][0] == "text/html"`,
			expected:   false,
		},
		{
			name:       "header value prefix check",
			expression: `request.Headers["authorization"][0].startsWith("Bearer")`,
			expected:   true,
		},
		{
			name:       "header value contains",
			expression: `request.Headers["user-agent"][0].contains("test-agent")`,
			expected:   true,
		},
		{
			name:       "multiple header conditions - AND",
			expression: `"authorization" in request.Headers && "x-api-key" in request.Headers`,
			expected:   true,
		},
		{
			name:       "multiple header conditions - OR",
			expression: `"authorization" in request.Headers || "x-missing" in request.Headers`,
			expected:   true,
		},
		{
			name:       "header and method condition",
			expression: `"x-api-key" in request.Headers && request.Method == "POST"`,
			expected:   true,
		},
		{
			name:       "header value starts with specific prefix",
			expression: `request.Headers["x-api-key"][0].startsWith("sk-")`,
			expected:   true,
		},
		{
			name:       "combined header value and path check",
			expression: `request.Headers["content-type"][0] == "application/json" && request.Path.startsWith("/api")`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateRequestHeaderCondition(tt.expression, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateRequestHeaderCondition_HeaderOperations_EmptyHeaders(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	// Create context with no headers
	ctx := testutils.NewTestRequestHeaderContextWithHeaders(map[string][]string{})
	ctx.Method = "GET"
	ctx.Path = "/api/test"

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "check for missing header in empty headers",
			expression: `"authorization" in request.Headers`,
			expected:   false,
		},
		{
			name:       "check for any header in empty headers",
			expression: `"content-type" in request.Headers`,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateRequestHeaderCondition(tt.expression, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateResponseHeaderCondition_HeaderOperations(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	// Create response context with request and response headers
	ctx := testutils.NewTestResponseHeaderContext()
	ctx.RequestHeaders = policy.NewHeaders(map[string][]string{
		"authorization": {"Bearer token123"},
		"content-type":  {"application/json"},
	})
	ctx.ResponseHeaders = policy.NewHeaders(map[string][]string{
		"content-type":   {"application/json; charset=utf-8"},
		"cache-control":  {"no-cache"},
		"x-request-id":   {"req-123"},
		"content-length": {"1024"},
	})
	ctx.ResponseStatus = 200

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "response header exists",
			expression: `"content-type" in response.ResponseHeaders`,
			expected:   true,
		},
		{
			name:       "response header missing",
			expression: `"x-missing" in response.ResponseHeaders`,
			expected:   false,
		},
		{
			name:       "response header value check",
			expression: `response.ResponseHeaders["cache-control"][0] == "no-cache"`,
			expected:   true,
		},
		{
			name:       "response header value contains",
			expression: `response.ResponseHeaders["content-type"][0].contains("application/json")`,
			expected:   true,
		},
		{
			name:       "request header exists in response phase",
			expression: `"authorization" in response.RequestHeaders`,
			expected:   true,
		},
		{
			name:       "request header value in response phase",
			expression: `response.RequestHeaders["authorization"][0].startsWith("Bearer")`,
			expected:   true,
		},
		{
			name:       "combined request and response headers",
			expression: `"authorization" in response.RequestHeaders && "content-type" in response.ResponseHeaders`,
			expected:   true,
		},
		{
			name:       "response header and status check",
			expression: `"x-request-id" in response.ResponseHeaders && response.ResponseStatus == 200`,
			expected:   true,
		},
		{
			name:       "check both request and response content-type",
			expression: `response.RequestHeaders["content-type"][0] == "application/json" && response.ResponseHeaders["content-type"][0].contains("application/json")`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateResponseHeaderCondition(tt.expression, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateResponseHeaderCondition_HeaderOperations_EmptyHeaders(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	// Create response context with empty headers
	ctx := testutils.NewTestResponseHeaderContext()
	ctx.RequestHeaders = policy.NewHeaders(map[string][]string{})
	ctx.ResponseHeaders = policy.NewHeaders(map[string][]string{})
	ctx.ResponseStatus = 200

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "check for missing request header",
			expression: `"authorization" in response.RequestHeaders`,
			expected:   false,
		},
		{
			name:       "check for missing response header",
			expression: `"content-type" in response.ResponseHeaders`,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateResponseHeaderCondition(tt.expression, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Real World Expression Tests
// =============================================================================

func TestCELEvaluator_RealWorldExpressions(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	tests := []struct {
		name       string
		setupReq   func() *policy.RequestHeaderContext
		setupResp  func() *policy.ResponseHeaderContext
		expression string
		phase      string // "request" or "response"
		expected   bool
	}{
		{
			name: "Skip POST requests",
			setupReq: func() *policy.RequestHeaderContext {
				ctx := testutils.NewTestRequestHeaderContext()
				ctx.Method = "GET"
				return ctx
			},
			expression: `request.Method != "POST"`,
			phase:      "request",
			expected:   true,
		},
		{
			name: "Only POST requests",
			setupReq: func() *policy.RequestHeaderContext {
				ctx := testutils.NewTestRequestHeaderContext()
				ctx.Method = "POST"
				return ctx
			},
			expression: `request.Method == "POST"`,
			phase:      "request",
			expected:   true,
		},
		{
			name: "Error response only",
			setupResp: func() *policy.ResponseHeaderContext {
				ctx := testutils.NewTestResponseHeaderContext()
				ctx.ResponseStatus = 500
				return ctx
			},
			expression: `response.ResponseStatus >= 400`,
			phase:      "response",
			expected:   true,
		},
		{
			name: "Success response only",
			setupResp: func() *policy.ResponseHeaderContext {
				ctx := testutils.NewTestResponseHeaderContext()
				ctx.ResponseStatus = 200
				return ctx
			},
			expression: `response.ResponseStatus >= 200 && response.ResponseStatus < 300`,
			phase:      "response",
			expected:   true,
		},
		{
			name: "Specific path prefix",
			setupReq: func() *policy.RequestHeaderContext {
				ctx := testutils.NewTestRequestHeaderContext()
				ctx.Path = "/admin/users"
				return ctx
			},
			expression: `request.Path.startsWith("/admin")`,
			phase:      "request",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result bool
			var err error

			if tt.phase == "request" {
				result, err = evaluator.EvaluateRequestHeaderCondition(tt.expression, tt.setupReq())
			} else {
				result, err = evaluator.EvaluateResponseHeaderCondition(tt.expression, tt.setupResp())
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

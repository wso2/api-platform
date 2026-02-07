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
// EvaluateRequestCondition Tests
// =============================================================================

func TestEvaluateRequestCondition_SimpleExpressions(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestContext()

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
			name:       "Processing phase is request",
			expression: `processing.phase == "request"`,
			expected:   true,
		},
		{
			name:       "Processing phase is not response",
			expression: `processing.phase == "response"`,
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
			result, err := evaluator.EvaluateRequestCondition(tt.expression, reqCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateRequestCondition_ComplexExpressions(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestContext()

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
			expression: `request.Method == "GET" && request.Path.startsWith("/api") && processing.phase == "request"`,
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
			result, err := evaluator.EvaluateRequestCondition(tt.expression, reqCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateRequestCondition_InvalidExpression(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestContext()

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
			_, err := evaluator.EvaluateRequestCondition(tt.expression, reqCtx)
			assert.Error(t, err)
		})
	}
}

func TestEvaluateRequestCondition_NonBooleanResult(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestContext()

	// Expression that returns string instead of boolean
	_, err = evaluator.EvaluateRequestCondition(`request.Method`, reqCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must return boolean")
}

// =============================================================================
// EvaluateResponseCondition Tests
// =============================================================================

func TestEvaluateResponseCondition_SimpleExpressions(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	respCtx := testutils.NewTestResponseContext()

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
			name:       "Processing phase is response",
			expression: `processing.phase == "response"`,
			expected:   true,
		},
		{
			name:       "Processing phase is not request",
			expression: `processing.phase == "request"`,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.EvaluateResponseCondition(tt.expression, respCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateResponseCondition_CrossPhaseAccess(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	respCtx := testutils.NewTestResponseContext()

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
			result, err := evaluator.EvaluateResponseCondition(tt.expression, respCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateResponseCondition_StatusRanges(t *testing.T) {
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
			respCtx := testutils.NewTestResponseContext()
			respCtx.ResponseStatus = tt.status
			result, err := evaluator.EvaluateResponseCondition(tt.expression, respCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateResponseCondition_InvalidExpression(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	respCtx := testutils.NewTestResponseContext()

	_, err = evaluator.EvaluateResponseCondition(`response.ResponseStatus ==`, respCtx)
	assert.Error(t, err)
}

func TestEvaluateResponseCondition_NonBooleanResult(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	respCtx := testutils.NewTestResponseContext()

	_, err = evaluator.EvaluateResponseCondition(`response.ResponseStatus`, respCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must return boolean")
}

// =============================================================================
// Caching Tests
// =============================================================================

func TestCELEvaluator_ProgramCaching(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestContext()
	expression := `request.Method == "GET"`

	// First evaluation - compiles the program
	result1, err := evaluator.EvaluateRequestCondition(expression, reqCtx)
	require.NoError(t, err)
	assert.True(t, result1)

	// Second evaluation - should use cached program
	result2, err := evaluator.EvaluateRequestCondition(expression, reqCtx)
	require.NoError(t, err)
	assert.True(t, result2)

	// Both results should be the same
	assert.Equal(t, result1, result2)
}

func TestCELEvaluator_DifferentExpressions(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestContext()

	expressions := []string{
		`request.Method == "GET"`,
		`request.Method == "POST"`,
		`request.Path.startsWith("/api")`,
		`processing.phase == "request"`,
	}

	// Evaluate each expression
	for _, expr := range expressions {
		_, err := evaluator.EvaluateRequestCondition(expr, reqCtx)
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
		`processing.phase == "request"`,
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 10; i++ {
		for _, expr := range expressions {
			wg.Add(1)
			go func(e string) {
				defer wg.Done()
				ctx := testutils.NewTestRequestContext()
				_, err := evaluator.EvaluateRequestCondition(e, ctx)
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

func TestEvaluateResponseCondition_ZeroStatus(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	respCtx := testutils.NewTestResponseContext()
	respCtx.ResponseStatus = 0

	result, err := evaluator.EvaluateResponseCondition(`response.ResponseStatus == 0`, respCtx)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluateRequestCondition_EmptyPath(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestContext()
	reqCtx.Path = ""

	result, err := evaluator.EvaluateRequestCondition(`request.Path == ""`, reqCtx)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluateRequestCondition_PathEndsWith(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestContext()

	result, err := evaluator.EvaluateRequestCondition(`request.Path.endsWith("123")`, reqCtx)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluateRequestCondition_PathSize(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	reqCtx := testutils.NewTestRequestContext()

	result, err := evaluator.EvaluateRequestCondition(`size(request.Path) > 0`, reqCtx)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluateRequestCondition_MethodComparison(t *testing.T) {
	evaluator, err := NewCELEvaluator()
	require.NoError(t, err)

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			reqCtx := testutils.NewTestRequestContext()
			reqCtx.Method = method

			result, err := evaluator.EvaluateRequestCondition(`request.Method == "`+method+`"`, reqCtx)
			require.NoError(t, err)
			assert.True(t, result)
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
		setupReq   func() *policy.RequestContext
		setupResp  func() *policy.ResponseContext
		expression string
		phase      string // "request" or "response"
		expected   bool
	}{
		{
			name: "Skip POST requests",
			setupReq: func() *policy.RequestContext {
				ctx := testutils.NewTestRequestContext()
				ctx.Method = "GET"
				return ctx
			},
			expression: `request.Method != "POST"`,
			phase:      "request",
			expected:   true,
		},
		{
			name: "Only POST requests",
			setupReq: func() *policy.RequestContext {
				ctx := testutils.NewTestRequestContext()
				ctx.Method = "POST"
				return ctx
			},
			expression: `request.Method == "POST"`,
			phase:      "request",
			expected:   true,
		},
		{
			name: "Error response only",
			setupResp: func() *policy.ResponseContext {
				ctx := testutils.NewTestResponseContext()
				ctx.ResponseStatus = 500
				return ctx
			},
			expression: `response.ResponseStatus >= 400`,
			phase:      "response",
			expected:   true,
		},
		{
			name: "Success response only",
			setupResp: func() *policy.ResponseContext {
				ctx := testutils.NewTestResponseContext()
				ctx.ResponseStatus = 200
				return ctx
			},
			expression: `response.ResponseStatus >= 200 && response.ResponseStatus < 300`,
			phase:      "response",
			expected:   true,
		},
		{
			name: "Specific path prefix",
			setupReq: func() *policy.RequestContext {
				ctx := testutils.NewTestRequestContext()
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
				result, err = evaluator.EvaluateRequestCondition(tt.expression, tt.setupReq())
			} else {
				result, err = evaluator.EvaluateResponseCondition(tt.expression, tt.setupResp())
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

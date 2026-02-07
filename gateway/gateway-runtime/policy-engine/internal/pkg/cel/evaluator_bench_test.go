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
	"testing"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// =============================================================================
// Helper Functions
// =============================================================================

// buildBenchRequestContext creates a RequestContext for CEL benchmarks.
func buildBenchRequestContext() *policy.RequestContext {
	return &policy.RequestContext{
		SharedContext: &policy.SharedContext{
			RequestID:     "bench-id",
			APIName:       "PetStore",
			APIVersion:    "v1.0",
			APIContext:    "/petstore",
			OperationPath: "/pets/{id}",
			Metadata:      map[string]interface{}{"custom_key": "custom_value"},
		},
		Headers:   policy.NewHeaders(map[string][]string{"content-type": {"application/json"}, "authorization": {"Bearer token123"}, "x-api-key": {"abc123"}}),
		Path:      "/petstore/v1.0/pets/123",
		Method:    "GET",
		Authority: "api.example.com",
		Scheme:    "https",
	}
}

// buildBenchResponseContext creates a ResponseContext for CEL benchmarks.
func buildBenchResponseContext() *policy.ResponseContext {
	reqCtx := buildBenchRequestContext()
	return &policy.ResponseContext{
		SharedContext:   reqCtx.SharedContext,
		RequestHeaders:  reqCtx.Headers,
		RequestBody:     reqCtx.Body,
		RequestPath:     reqCtx.Path,
		RequestMethod:   reqCtx.Method,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}, "x-custom": {"value"}}),
		ResponseStatus:  200,
		ResponseBody: &policy.Body{
			Content:     []byte(`{"data": "test"}`),
			EndOfStream: true,
			Present:     true,
		},
	}
}

// =============================================================================
// CEL Evaluator Creation Benchmark
// =============================================================================

// BenchmarkNewCELEvaluator benchmarks CEL evaluator creation.
// This measures the one-time cost of setting up the CEL environment.
func BenchmarkNewCELEvaluator(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewCELEvaluator()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// =============================================================================
// Request Condition Evaluation Benchmarks
// =============================================================================

// BenchmarkCELEvaluateRequestCondition benchmarks request condition evaluation.
func BenchmarkCELEvaluateRequestCondition(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	reqCtx := buildBenchRequestContext()

	b.Run("SimpleMethodCheck", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(`request.Method == "GET"`, reqCtx)
		}
	})

	b.Run("SimplePathCheck", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(`request.Path.startsWith("/petstore")`, reqCtx)
		}
	})

	b.Run("HeaderExists", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(`"authorization" in request.Headers`, reqCtx)
		}
	})

	b.Run("ComplexLogicalExpression", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(
				`request.Method == "GET" && request.Path.startsWith("/petstore") && request.Scheme == "https"`,
				reqCtx)
		}
	})

	b.Run("MetadataAccess", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(`"custom_key" in request.Metadata`, reqCtx)
		}
	})

	b.Run("ProcessingPhaseCheck", func(b *testing.B) {
		expr := `processing.phase == "request"`
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx)
		}
	})
}

// BenchmarkCELEvaluateRequestCondition_CacheHit benchmarks cached program execution.
func BenchmarkCELEvaluateRequestCondition_CacheHit(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	reqCtx := buildBenchRequestContext()
	expr := `request.Method == "GET"`

	// Warm up cache
	_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx)
	}
}

// BenchmarkCELEvaluateRequestCondition_CacheMiss benchmarks compilation + execution.
func BenchmarkCELEvaluateRequestCondition_CacheMiss(b *testing.B) {
	reqCtx := buildBenchRequestContext()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create fresh evaluator each iteration to force cache miss
		evaluator, _ := NewCELEvaluator()
		_, _ = evaluator.EvaluateRequestCondition(`request.Method == "GET"`, reqCtx)
	}
}

// =============================================================================
// Response Condition Evaluation Benchmarks
// =============================================================================

// BenchmarkCELEvaluateResponseCondition benchmarks response condition evaluation.
func BenchmarkCELEvaluateResponseCondition(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	respCtx := buildBenchResponseContext()

	b.Run("SimpleStatusCheck", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseCondition(`response.ResponseStatus == 200`, respCtx)
		}
	})

	b.Run("StatusRangeCheck", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseCondition(
				`response.ResponseStatus >= 200 && response.ResponseStatus < 300`,
				respCtx)
		}
	})

	b.Run("ResponseHeaderExists", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseCondition(`"content-type" in response.ResponseHeaders`, respCtx)
		}
	})

	b.Run("CrossPhaseAccess", func(b *testing.B) {
		// Access request data during response phase
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseCondition(
				`response.RequestMethod == "GET" && response.ResponseStatus == 200`,
				respCtx)
		}
	})

	b.Run("ProcessingPhaseCheck", func(b *testing.B) {
		expr := `processing.phase == "response"`
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseCondition(expr, respCtx)
		}
	})
}

// =============================================================================
// Parallel Execution Benchmarks
// =============================================================================

// BenchmarkCELEvaluateRequestCondition_Parallel benchmarks concurrent evaluation.
func BenchmarkCELEvaluateRequestCondition_Parallel(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	// Warm up cache
	reqCtx := buildBenchRequestContext()
	_, _ = evaluator.EvaluateRequestCondition(`request.Method == "GET"`, reqCtx)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine uses its own context to avoid data races
		ctx := buildBenchRequestContext()
		for pb.Next() {
			_, _ = evaluator.EvaluateRequestCondition(`request.Method == "GET"`, ctx)
		}
	})
}

// BenchmarkCELEvaluateRequestCondition_ParallelDifferentExprs benchmarks concurrent
// evaluation with different expressions to measure cache contention.
func BenchmarkCELEvaluateRequestCondition_ParallelDifferentExprs(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	expressions := []string{
		`request.Method == "GET"`,
		`request.Method == "POST"`,
		`request.Path.startsWith("/petstore")`,
		`request.Scheme == "https"`,
		`"authorization" in request.Headers`,
	}

	// Warm up cache
	reqCtx := buildBenchRequestContext()
	for _, expr := range expressions {
		_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := buildBenchRequestContext()
		i := 0
		for pb.Next() {
			expr := expressions[i%len(expressions)]
			_, _ = evaluator.EvaluateRequestCondition(expr, ctx)
			i++
		}
	})
}

// =============================================================================
// Expression Complexity Benchmarks
// =============================================================================

// BenchmarkCELExpressionComplexity measures impact of expression complexity.
func BenchmarkCELExpressionComplexity(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	reqCtx := buildBenchRequestContext()

	b.Run("Trivial", func(b *testing.B) {
		expr := `true`
		_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx) // warm cache
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx)
		}
	})

	b.Run("SingleField", func(b *testing.B) {
		expr := `request.Method == "GET"`
		_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx) // warm cache
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx)
		}
	})

	b.Run("TwoConditionsAnd", func(b *testing.B) {
		expr := `request.Method == "GET" && request.Scheme == "https"`
		_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx) // warm cache
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx)
		}
	})

	b.Run("ThreeConditionsAnd", func(b *testing.B) {
		expr := `request.Method == "GET" && request.Scheme == "https" && request.Path.startsWith("/petstore")`
		_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx) // warm cache
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx)
		}
	})

	b.Run("FiveConditionsOr", func(b *testing.B) {
		expr := `request.Method == "GET" || request.Method == "POST" || request.Method == "PUT" || request.Method == "DELETE" || request.Method == "PATCH"`
		_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx) // warm cache
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx)
		}
	})

	b.Run("NestedLogic", func(b *testing.B) {
		expr := `(request.Method == "GET" || request.Method == "HEAD") && (request.Scheme == "https" || request.Scheme == "http") && request.Path.startsWith("/petstore")`
		_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx) // warm cache
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx)
		}
	})
}

// =============================================================================
// Real-World Expression Benchmarks
// =============================================================================

// BenchmarkCELRealWorldExpressions benchmarks expressions typical in production.
func BenchmarkCELRealWorldExpressions(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	reqCtx := buildBenchRequestContext()
	respCtx := buildBenchResponseContext()

	b.Run("AuthHeaderRequired", func(b *testing.B) {
		expr := `"authorization" in request.Headers`
		_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx) // warm cache
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx)
		}
	})

	b.Run("MethodAndPath", func(b *testing.B) {
		expr := `request.Method == "POST" && request.Path.startsWith("/api/v1/")`
		_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx) // warm cache
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestCondition(expr, reqCtx)
		}
	})

	b.Run("ErrorResponseOnly", func(b *testing.B) {
		expr := `response.ResponseStatus >= 400`
		_, _ = evaluator.EvaluateResponseCondition(expr, respCtx) // warm cache
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseCondition(expr, respCtx)
		}
	})

	b.Run("SuccessResponseOnly", func(b *testing.B) {
		expr := `response.ResponseStatus >= 200 && response.ResponseStatus < 300`
		_, _ = evaluator.EvaluateResponseCondition(expr, respCtx) // warm cache
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseCondition(expr, respCtx)
		}
	})

	b.Run("DualPhaseExpression", func(b *testing.B) {
		// Expression that works in both phases using processing.phase
		reqExpr := `processing.phase == "request" && request.Method == "GET"`
		respExpr := `processing.phase == "response" && response.ResponseStatus == 200`

		_, _ = evaluator.EvaluateRequestCondition(reqExpr, reqCtx)    // warm cache
		_, _ = evaluator.EvaluateResponseCondition(respExpr, respCtx) // warm cache

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if i%2 == 0 {
				_, _ = evaluator.EvaluateRequestCondition(reqExpr, reqCtx)
			} else {
				_, _ = evaluator.EvaluateResponseCondition(respExpr, respCtx)
			}
		}
	})
}

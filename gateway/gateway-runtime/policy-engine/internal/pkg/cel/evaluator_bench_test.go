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

func buildBenchRequestHeaderContext() *policy.RequestHeaderContext {
	return &policy.RequestHeaderContext{
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

func buildBenchResponseHeaderContext() *policy.ResponseHeaderContext {
	reqCtx := buildBenchRequestHeaderContext()
	return &policy.ResponseHeaderContext{
		SharedContext:   reqCtx.SharedContext,
		RequestHeaders:  reqCtx.Headers,
		RequestPath:     reqCtx.Path,
		RequestMethod:   reqCtx.Method,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}, "x-custom": {"value"}}),
		ResponseStatus:  200,
	}
}

// =============================================================================
// CEL Evaluator Creation Benchmark
// =============================================================================

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
// Request Header Condition Evaluation Benchmarks
// =============================================================================

func BenchmarkCELEvaluateRequestHeaderCondition(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	reqCtx := buildBenchRequestHeaderContext()

	b.Run("SimpleMethodCheck", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(`request.Method == "GET"`, reqCtx)
		}
	})

	b.Run("SimplePathCheck", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(`request.Path.startsWith("/petstore")`, reqCtx)
		}
	})

	b.Run("HeaderExists", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(`"authorization" in request.Headers`, reqCtx)
		}
	})

	b.Run("ComplexLogicalExpression", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(
				`request.Method == "GET" && request.Path.startsWith("/petstore") && request.Scheme == "https"`,
				reqCtx)
		}
	})

	b.Run("MetadataAccess", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(`"custom_key" in request.Metadata`, reqCtx)
		}
	})

	b.Run("ProcessingPhaseCheck", func(b *testing.B) {
		expr := `processing.phase == "request"`
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		}
	})
}

func BenchmarkCELEvaluateRequestHeaderCondition_CacheHit(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	reqCtx := buildBenchRequestHeaderContext()
	expr := `request.Method == "GET"`

	// Warm up cache
	_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
	}
}

func BenchmarkCELEvaluateRequestHeaderCondition_CacheMiss(b *testing.B) {
	reqCtx := buildBenchRequestHeaderContext()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		evaluator, _ := NewCELEvaluator()
		_, _ = evaluator.EvaluateRequestHeaderCondition(`request.Method == "GET"`, reqCtx)
	}
}

// =============================================================================
// Response Header Condition Evaluation Benchmarks
// =============================================================================

func BenchmarkCELEvaluateResponseHeaderCondition(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	respCtx := buildBenchResponseHeaderContext()

	b.Run("SimpleStatusCheck", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseHeaderCondition(`response.ResponseStatus == 200`, respCtx)
		}
	})

	b.Run("StatusRangeCheck", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseHeaderCondition(
				`response.ResponseStatus >= 200 && response.ResponseStatus < 300`,
				respCtx)
		}
	})

	b.Run("ResponseHeaderExists", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseHeaderCondition(`"content-type" in response.ResponseHeaders`, respCtx)
		}
	})

	b.Run("CrossPhaseAccess", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseHeaderCondition(
				`response.RequestMethod == "GET" && response.ResponseStatus == 200`,
				respCtx)
		}
	})

	b.Run("ProcessingPhaseCheck", func(b *testing.B) {
		expr := `processing.phase == "response"`
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseHeaderCondition(expr, respCtx)
		}
	})
}

// =============================================================================
// Parallel Execution Benchmarks
// =============================================================================

func BenchmarkCELEvaluateRequestHeaderCondition_Parallel(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	reqCtx := buildBenchRequestHeaderContext()
	_, _ = evaluator.EvaluateRequestHeaderCondition(`request.Method == "GET"`, reqCtx)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := buildBenchRequestHeaderContext()
		for pb.Next() {
			_, _ = evaluator.EvaluateRequestHeaderCondition(`request.Method == "GET"`, ctx)
		}
	})
}

func BenchmarkCELEvaluateRequestHeaderCondition_ParallelDifferentExprs(b *testing.B) {
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

	reqCtx := buildBenchRequestHeaderContext()
	for _, expr := range expressions {
		_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := buildBenchRequestHeaderContext()
		i := 0
		for pb.Next() {
			expr := expressions[i%len(expressions)]
			_, _ = evaluator.EvaluateRequestHeaderCondition(expr, ctx)
			i++
		}
	})
}

// =============================================================================
// Expression Complexity Benchmarks
// =============================================================================

func BenchmarkCELExpressionComplexity(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	reqCtx := buildBenchRequestHeaderContext()

	b.Run("Trivial", func(b *testing.B) {
		expr := `true`
		_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		}
	})

	b.Run("SingleField", func(b *testing.B) {
		expr := `request.Method == "GET"`
		_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		}
	})

	b.Run("TwoConditionsAnd", func(b *testing.B) {
		expr := `request.Method == "GET" && request.Scheme == "https"`
		_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		}
	})

	b.Run("ThreeConditionsAnd", func(b *testing.B) {
		expr := `request.Method == "GET" && request.Scheme == "https" && request.Path.startsWith("/petstore")`
		_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		}
	})

	b.Run("FiveConditionsOr", func(b *testing.B) {
		expr := `request.Method == "GET" || request.Method == "POST" || request.Method == "PUT" || request.Method == "DELETE" || request.Method == "PATCH"`
		_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		}
	})

	b.Run("NestedLogic", func(b *testing.B) {
		expr := `(request.Method == "GET" || request.Method == "HEAD") && (request.Scheme == "https" || request.Scheme == "http") && request.Path.startsWith("/petstore")`
		_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		}
	})
}

// =============================================================================
// Real-World Expression Benchmarks
// =============================================================================

func BenchmarkCELRealWorldExpressions(b *testing.B) {
	evaluator, err := NewCELEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	reqCtx := buildBenchRequestHeaderContext()
	respCtx := buildBenchResponseHeaderContext()

	b.Run("AuthHeaderRequired", func(b *testing.B) {
		expr := `"authorization" in request.Headers`
		_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		}
	})

	b.Run("MethodAndPath", func(b *testing.B) {
		expr := `request.Method == "POST" && request.Path.startsWith("/api/v1/")`
		_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateRequestHeaderCondition(expr, reqCtx)
		}
	})

	b.Run("ErrorResponseOnly", func(b *testing.B) {
		expr := `response.ResponseStatus >= 400`
		_, _ = evaluator.EvaluateResponseHeaderCondition(expr, respCtx)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseHeaderCondition(expr, respCtx)
		}
	})

	b.Run("SuccessResponseOnly", func(b *testing.B) {
		expr := `response.ResponseStatus >= 200 && response.ResponseStatus < 300`
		_, _ = evaluator.EvaluateResponseHeaderCondition(expr, respCtx)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = evaluator.EvaluateResponseHeaderCondition(expr, respCtx)
		}
	})

	b.Run("DualPhaseExpression", func(b *testing.B) {
		reqExpr := `processing.phase == "request" && request.Method == "GET"`
		respExpr := `processing.phase == "response" && response.ResponseStatus == 200`

		_, _ = evaluator.EvaluateRequestHeaderCondition(reqExpr, reqCtx)
		_, _ = evaluator.EvaluateResponseHeaderCondition(respExpr, respCtx)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if i%2 == 0 {
				_, _ = evaluator.EvaluateRequestHeaderCondition(reqExpr, reqCtx)
			} else {
				_, _ = evaluator.EvaluateResponseHeaderCondition(respExpr, respCtx)
			}
		}
	})
}

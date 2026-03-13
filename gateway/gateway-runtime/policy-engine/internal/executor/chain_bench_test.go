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

package executor

import (
	"context"
	"fmt"
	"os"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// =============================================================================
// Test Setup
// =============================================================================

func TestMain(m *testing.M) {
	// Disable metrics for clean benchmark measurements.
	metrics.SetEnabled(false)
	metrics.Init()

	os.Exit(m.Run())
}

// =============================================================================
// Mock CEL Evaluators
// =============================================================================

// alwaysTrueCELEvaluator implements CELEvaluator returning true for all conditions.
type alwaysTrueCELEvaluator struct{}

func (e *alwaysTrueCELEvaluator) EvaluateRequestHeaderCondition(_ string, _ *policy.RequestHeaderContext) (bool, error) {
	return true, nil
}

func (e *alwaysTrueCELEvaluator) EvaluateRequestBodyCondition(_ string, _ *policy.RequestContext) (bool, error) {
	return true, nil
}

func (e *alwaysTrueCELEvaluator) EvaluateResponseHeaderCondition(_ string, _ *policy.ResponseHeaderContext) (bool, error) {
	return true, nil
}

func (e *alwaysTrueCELEvaluator) EvaluateResponseBodyCondition(_ string, _ *policy.ResponseContext) (bool, error) {
	return true, nil
}

func (e *alwaysTrueCELEvaluator) EvaluateStreamingRequestCondition(_ string, _ *policy.RequestStreamContext) (bool, error) {
	return true, nil
}

func (e *alwaysTrueCELEvaluator) EvaluateStreamingResponseCondition(_ string, _ *policy.ResponseStreamContext) (bool, error) {
	return true, nil
}

// sometimesFalseCELEvaluator skips every third policy for realistic testing.
type sometimesFalseCELEvaluator struct {
	counter int
}

func (e *sometimesFalseCELEvaluator) EvaluateRequestHeaderCondition(_ string, _ *policy.RequestHeaderContext) (bool, error) {
	e.counter++
	return e.counter%3 != 0, nil
}

func (e *sometimesFalseCELEvaluator) EvaluateRequestBodyCondition(_ string, _ *policy.RequestContext) (bool, error) {
	e.counter++
	return e.counter%3 != 0, nil
}

func (e *sometimesFalseCELEvaluator) EvaluateResponseHeaderCondition(_ string, _ *policy.ResponseHeaderContext) (bool, error) {
	e.counter++
	return e.counter%3 != 0, nil
}

func (e *sometimesFalseCELEvaluator) EvaluateResponseBodyCondition(_ string, _ *policy.ResponseContext) (bool, error) {
	e.counter++
	return e.counter%3 != 0, nil
}

func (e *sometimesFalseCELEvaluator) EvaluateStreamingRequestCondition(_ string, _ *policy.RequestStreamContext) (bool, error) {
	e.counter++
	return e.counter%3 != 0, nil
}

func (e *sometimesFalseCELEvaluator) EvaluateStreamingResponseCondition(_ string, _ *policy.ResponseStreamContext) (bool, error) {
	e.counter++
	return e.counter%3 != 0, nil
}

// =============================================================================
// Mock Policies
// =============================================================================

// passthroughRequestPolicy implements RequestHeaderPolicy with no modifications.
type passthroughRequestPolicy struct{}

func (p *passthroughRequestPolicy) OnRequestHeaders(_ *policy.RequestHeaderContext) policy.RequestHeaderAction {
	return policy.UpstreamRequestHeaderModifications{}
}

// passthroughResponsePolicy implements ResponseHeaderPolicy with no modifications.
type passthroughResponsePolicy struct{}

func (p *passthroughResponsePolicy) OnResponseHeaders(_ *policy.ResponseHeaderContext) policy.ResponseHeaderAction {
	return policy.DownstreamResponseHeaderModifications{}
}

// headerModifyRequestPolicy adds headers to the request.
type headerModifyRequestPolicy struct{}

func (p *headerModifyRequestPolicy) OnRequestHeaders(_ *policy.RequestHeaderContext) policy.RequestHeaderAction {
	return policy.UpstreamRequestHeaderModifications{
		Set:    map[string]string{"x-bench-header": "bench-value"},
		Append: map[string][]string{"x-multi": {"v1", "v2"}},
	}
}

// headerModifyResponsePolicy adds headers to the response.
type headerModifyResponsePolicy struct{}

func (p *headerModifyResponsePolicy) OnResponseHeaders(_ *policy.ResponseHeaderContext) policy.ResponseHeaderAction {
	return policy.DownstreamResponseHeaderModifications{
		Set: map[string]string{"x-bench-resp": "bench-resp-value"},
	}
}

// shortCircuitRequestPolicy short-circuits with ImmediateResponse.
type shortCircuitRequestPolicy struct{}

func (p *shortCircuitRequestPolicy) OnRequestHeaders(_ *policy.RequestHeaderContext) policy.RequestHeaderAction {
	return policy.ImmediateResponse{
		StatusCode: 401,
		Headers:    map[string]string{"content-type": "application/json"},
		Body:       []byte(`{"error":"unauthorized"}`),
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func buildPolicySpec(name, version string, condition *string) policy.PolicySpec {
	return policy.PolicySpec{
		Name:               name,
		Version:            version,
		Enabled:            true,
		Parameters:         policy.PolicyParameters{Raw: map[string]interface{}{}},
		ExecutionCondition: condition,
	}
}

func buildDisabledPolicySpec(name, version string) policy.PolicySpec {
	return policy.PolicySpec{
		Name:       name,
		Version:    version,
		Enabled:    false,
		Parameters: policy.PolicyParameters{Raw: map[string]interface{}{}},
	}
}

func buildTestRequestHeaderContext() *policy.RequestHeaderContext {
	return &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{
			RequestID:     "bench-req-id",
			APIName:       "PetStore",
			APIVersion:    "v1.0",
			APIContext:    "/petstore",
			OperationPath: "/pets/{id}",
			Metadata:      map[string]interface{}{},
		},
		Headers:   policy.NewHeaders(map[string][]string{"content-type": {"application/json"}, "authorization": {"Bearer token123"}}),
		Path:      "/petstore/v1.0/pets/123",
		Method:    "GET",
		Authority: "api.example.com",
		Scheme:    "https",
	}
}

func buildTestResponseHeaderContext() *policy.ResponseHeaderContext {
	reqCtx := buildTestRequestHeaderContext()
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
// Request Header Policy Execution Benchmarks
// =============================================================================

func BenchmarkExecuteRequestHeaderPolicies(b *testing.B) {
	scenarios := []struct {
		name        string
		numPolicies int
		withCEL     bool
	}{
		{"1Policy_NoCEL", 1, false},
		{"3Policies_NoCEL", 3, false},
		{"5Policies_NoCEL", 5, false},
		{"1Policy_WithCEL", 1, true},
		{"3Policies_WithCEL", 3, true},
		{"5Policies_WithCEL", 5, true},
	}

	for _, sc := range scenarios {
		b.Run(sc.name, func(b *testing.B) {
			var celEval CELEvaluator
			if sc.withCEL {
				celEval = &alwaysTrueCELEvaluator{}
			}

			tracer := trace.NewNoopTracerProvider().Tracer("bench")
			exec := NewChainExecutor(nil, celEval, tracer)

			var policies []policy.Policy
			var specs []policy.PolicySpec
			cond := `request.Method == "GET"`

			for i := 0; i < sc.numPolicies; i++ {
				policies = append(policies, &passthroughRequestPolicy{})
				var condPtr *string
				if sc.withCEL {
					condPtr = &cond
				}
				specs = append(specs, buildPolicySpec(fmt.Sprintf("p%d", i), "v1.0", condPtr))
			}

			reqCtx := buildTestRequestHeaderContext()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = exec.ExecuteRequestHeaderPolicies(
					context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", sc.withCEL)
			}
		})
	}
}

func BenchmarkExecuteRequestHeaderPolicies_WithModifications(b *testing.B) {
	tracer := trace.NewNoopTracerProvider().Tracer("bench")
	exec := NewChainExecutor(nil, nil, tracer)

	policies := []policy.Policy{
		&headerModifyRequestPolicy{},
		&headerModifyRequestPolicy{},
		&headerModifyRequestPolicy{},
	}
	specs := []policy.PolicySpec{
		buildPolicySpec("mod-1", "v1.0", nil),
		buildPolicySpec("mod-2", "v1.0", nil),
		buildPolicySpec("mod-3", "v1.0", nil),
	}

	reqCtx := buildTestRequestHeaderContext()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = exec.ExecuteRequestHeaderPolicies(
			context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", false)
	}
}

func BenchmarkExecuteRequestHeaderPolicies_ShortCircuit(b *testing.B) {
	tracer := trace.NewNoopTracerProvider().Tracer("bench")
	exec := NewChainExecutor(nil, nil, tracer)

	policies := []policy.Policy{
		&shortCircuitRequestPolicy{},
		&passthroughRequestPolicy{},
		&passthroughRequestPolicy{},
	}
	specs := []policy.PolicySpec{
		buildPolicySpec("auth", "v1.0", nil),
		buildPolicySpec("should-skip-1", "v1.0", nil),
		buildPolicySpec("should-skip-2", "v1.0", nil),
	}

	reqCtx := buildTestRequestHeaderContext()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = exec.ExecuteRequestHeaderPolicies(
			context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", false)
	}
}

func BenchmarkExecuteRequestHeaderPolicies_WithDisabled(b *testing.B) {
	tracer := trace.NewNoopTracerProvider().Tracer("bench")
	exec := NewChainExecutor(nil, nil, tracer)

	policies := []policy.Policy{
		&passthroughRequestPolicy{},
		&passthroughRequestPolicy{}, // disabled
		&passthroughRequestPolicy{},
		&passthroughRequestPolicy{}, // disabled
		&passthroughRequestPolicy{},
	}
	specs := []policy.PolicySpec{
		buildPolicySpec("p1", "v1.0", nil),
		buildDisabledPolicySpec("p2", "v1.0"),
		buildPolicySpec("p3", "v1.0", nil),
		buildDisabledPolicySpec("p4", "v1.0"),
		buildPolicySpec("p5", "v1.0", nil),
	}

	reqCtx := buildTestRequestHeaderContext()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = exec.ExecuteRequestHeaderPolicies(
			context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", false)
	}
}

func BenchmarkExecuteRequestHeaderPolicies_CELSkipping(b *testing.B) {
	celEval := &sometimesFalseCELEvaluator{}
	tracer := trace.NewNoopTracerProvider().Tracer("bench")
	exec := NewChainExecutor(nil, celEval, tracer)

	cond := `request.Method == "GET"`
	policies := []policy.Policy{
		&passthroughRequestPolicy{},
		&passthroughRequestPolicy{},
		&passthroughRequestPolicy{},
		&passthroughRequestPolicy{},
		&passthroughRequestPolicy{},
	}
	specs := []policy.PolicySpec{
		buildPolicySpec("p1", "v1.0", &cond),
		buildPolicySpec("p2", "v1.0", &cond),
		buildPolicySpec("p3", "v1.0", &cond),
		buildPolicySpec("p4", "v1.0", &cond),
		buildPolicySpec("p5", "v1.0", &cond),
	}

	reqCtx := buildTestRequestHeaderContext()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		celEval.counter = 0
		_, _ = exec.ExecuteRequestHeaderPolicies(
			context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", true)
	}
}

// =============================================================================
// Response Header Policy Execution Benchmarks
// =============================================================================

func BenchmarkExecuteResponseHeaderPolicies(b *testing.B) {
	scenarios := []struct {
		name        string
		numPolicies int
		withCEL     bool
	}{
		{"1Policy_NoCEL", 1, false},
		{"3Policies_NoCEL", 3, false},
		{"5Policies_NoCEL", 5, false},
		{"3Policies_WithCEL", 3, true},
	}

	for _, sc := range scenarios {
		b.Run(sc.name, func(b *testing.B) {
			var celEval CELEvaluator
			if sc.withCEL {
				celEval = &alwaysTrueCELEvaluator{}
			}

			tracer := trace.NewNoopTracerProvider().Tracer("bench")
			exec := NewChainExecutor(nil, celEval, tracer)

			var policies []policy.Policy
			var specs []policy.PolicySpec
			cond := `response.ResponseStatus == 200`

			for i := 0; i < sc.numPolicies; i++ {
				policies = append(policies, &headerModifyResponsePolicy{})
				var condPtr *string
				if sc.withCEL {
					condPtr = &cond
				}
				specs = append(specs, buildPolicySpec(fmt.Sprintf("p%d", i), "v1.0", condPtr))
			}

			respCtx := buildTestResponseHeaderContext()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = exec.ExecuteResponseHeaderPolicies(
					context.Background(), policies, respCtx, specs, "PetStore", "bench-route", sc.withCEL)
			}
		})
	}
}

// =============================================================================
// Parallel Execution Benchmarks
// =============================================================================

func BenchmarkExecuteRequestHeaderPolicies_Parallel(b *testing.B) {
	tracer := trace.NewNoopTracerProvider().Tracer("bench")
	exec := NewChainExecutor(nil, nil, tracer)

	policies := []policy.Policy{
		&passthroughRequestPolicy{},
		&headerModifyRequestPolicy{},
		&passthroughRequestPolicy{},
	}
	specs := []policy.PolicySpec{
		buildPolicySpec("p1", "v1.0", nil),
		buildPolicySpec("p2", "v1.0", nil),
		buildPolicySpec("p3", "v1.0", nil),
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			reqCtx := buildTestRequestHeaderContext()
			_, _ = exec.ExecuteRequestHeaderPolicies(
				context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", false)
		}
	})
}

// =============================================================================
// Realistic Workload Benchmark
// =============================================================================

func BenchmarkExecuteRequestHeaderPolicies_RealisticWorkload(b *testing.B) {
	tracer := trace.NewNoopTracerProvider().Tracer("bench")
	exec := NewChainExecutor(nil, nil, tracer)

	policies := []policy.Policy{
		&passthroughRequestPolicy{},  // Auth (simulated as passthrough)
		&passthroughRequestPolicy{},  // Rate limit (simulated as passthrough)
		&headerModifyRequestPolicy{}, // Add correlation headers
	}
	specs := []policy.PolicySpec{
		buildPolicySpec("jwt-auth", "v1.0", nil),
		buildPolicySpec("rate-limit", "v1.0", nil),
		buildPolicySpec("correlation", "v1.0", nil),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reqCtx := buildTestRequestHeaderContext()
		_, _ = exec.ExecuteRequestHeaderPolicies(
			context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", false)
	}
}

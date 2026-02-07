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
// Mock CEL Evaluator
// =============================================================================

// alwaysTrueCELEvaluator implements CELEvaluator with no real CEL overhead.
type alwaysTrueCELEvaluator struct{}

func (e *alwaysTrueCELEvaluator) EvaluateRequestCondition(string, *policy.RequestContext) (bool, error) {
	return true, nil
}

func (e *alwaysTrueCELEvaluator) EvaluateResponseCondition(string, *policy.ResponseContext) (bool, error) {
	return true, nil
}

// sometimesFalseCELEvaluator skips policies based on index for realistic testing.
type sometimesFalseCELEvaluator struct {
	counter int
}

func (e *sometimesFalseCELEvaluator) EvaluateRequestCondition(string, *policy.RequestContext) (bool, error) {
	e.counter++
	// Every third policy is skipped
	return e.counter%3 != 0, nil
}

func (e *sometimesFalseCELEvaluator) EvaluateResponseCondition(string, *policy.ResponseContext) (bool, error) {
	e.counter++
	return e.counter%3 != 0, nil
}

// =============================================================================
// Mock Policies
// =============================================================================

// passthroughPolicy implements policy.Policy with no modifications.
type passthroughPolicy struct{}

func (p *passthroughPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

func (p *passthroughPolicy) OnRequest(*policy.RequestContext, map[string]interface{}) policy.RequestAction {
	return nil
}

func (p *passthroughPolicy) OnResponse(*policy.ResponseContext, map[string]interface{}) policy.ResponseAction {
	return nil
}

// headerModifyPolicy adds headers to measure modification overhead.
type headerModifyPolicy struct{}

func (p *headerModifyPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

func (p *headerModifyPolicy) OnRequest(*policy.RequestContext, map[string]interface{}) policy.RequestAction {
	return policy.UpstreamRequestModifications{
		SetHeaders:    map[string]string{"x-bench-header": "bench-value"},
		AppendHeaders: map[string][]string{"x-multi": {"v1", "v2"}},
	}
}

func (p *headerModifyPolicy) OnResponse(*policy.ResponseContext, map[string]interface{}) policy.ResponseAction {
	return policy.UpstreamResponseModifications{
		SetHeaders: map[string]string{"x-bench-resp": "bench-resp-value"},
	}
}

// shortCircuitPolicy returns ImmediateResponse.
type shortCircuitPolicy struct{}

func (p *shortCircuitPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

func (p *shortCircuitPolicy) OnRequest(*policy.RequestContext, map[string]interface{}) policy.RequestAction {
	return policy.ImmediateResponse{
		StatusCode: 401,
		Headers:    map[string]string{"content-type": "application/json"},
		Body:       []byte(`{"error":"unauthorized"}`),
	}
}

func (p *shortCircuitPolicy) OnResponse(*policy.ResponseContext, map[string]interface{}) policy.ResponseAction {
	return nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// buildPolicySpec creates a PolicySpec for benchmarks.
func buildPolicySpec(name, version string, condition *string) policy.PolicySpec {
	return policy.PolicySpec{
		Name:               name,
		Version:            version,
		Enabled:            true,
		Parameters:         policy.PolicyParameters{Raw: map[string]interface{}{}},
		ExecutionCondition: condition,
	}
}

// buildDisabledPolicySpec creates a disabled PolicySpec.
func buildDisabledPolicySpec(name, version string) policy.PolicySpec {
	return policy.PolicySpec{
		Name:       name,
		Version:    version,
		Enabled:    false,
		Parameters: policy.PolicyParameters{Raw: map[string]interface{}{}},
	}
}

// buildTestRequestContext creates a realistic RequestContext for benchmarks.
func buildTestRequestContext() *policy.RequestContext {
	return &policy.RequestContext{
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

// buildTestResponseContext creates a realistic ResponseContext for benchmarks.
func buildTestResponseContext() *policy.ResponseContext {
	reqCtx := buildTestRequestContext()
	return &policy.ResponseContext{
		SharedContext:   reqCtx.SharedContext,
		RequestHeaders:  reqCtx.Headers,
		RequestBody:     reqCtx.Body,
		RequestPath:     reqCtx.Path,
		RequestMethod:   reqCtx.Method,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"content-type": {"application/json"}, "x-custom": {"value"}}),
		ResponseStatus:  200,
	}
}

// =============================================================================
// Request Policy Execution Benchmarks
// =============================================================================

// BenchmarkExecuteRequestPolicies benchmarks request policy chain execution.
func BenchmarkExecuteRequestPolicies(b *testing.B) {
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
				policies = append(policies, &passthroughPolicy{})
				var condPtr *string
				if sc.withCEL {
					condPtr = &cond
				}
				specs = append(specs, buildPolicySpec(
					fmt.Sprintf("p%d", i), "v1.0", condPtr))
			}

			reqCtx := buildTestRequestContext()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = exec.ExecuteRequestPolicies(
					context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", sc.withCEL)
			}
		})
	}
}

// BenchmarkExecuteRequestPolicies_WithModifications benchmarks with header modifications.
func BenchmarkExecuteRequestPolicies_WithModifications(b *testing.B) {
	tracer := trace.NewNoopTracerProvider().Tracer("bench")
	exec := NewChainExecutor(nil, nil, tracer)

	policies := []policy.Policy{
		&headerModifyPolicy{},
		&headerModifyPolicy{},
		&headerModifyPolicy{},
	}
	specs := []policy.PolicySpec{
		buildPolicySpec("mod-1", "v1.0", nil),
		buildPolicySpec("mod-2", "v1.0", nil),
		buildPolicySpec("mod-3", "v1.0", nil),
	}

	reqCtx := buildTestRequestContext()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = exec.ExecuteRequestPolicies(
			context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", false)
	}
}

// BenchmarkExecuteRequestPolicies_ShortCircuit benchmarks short-circuit behavior.
func BenchmarkExecuteRequestPolicies_ShortCircuit(b *testing.B) {
	tracer := trace.NewNoopTracerProvider().Tracer("bench")
	exec := NewChainExecutor(nil, nil, tracer)

	// Policy that short-circuits followed by policies that won't execute
	policies := []policy.Policy{
		&shortCircuitPolicy{},
		&passthroughPolicy{},
		&passthroughPolicy{},
	}
	specs := []policy.PolicySpec{
		buildPolicySpec("auth", "v1.0", nil),
		buildPolicySpec("should-skip-1", "v1.0", nil),
		buildPolicySpec("should-skip-2", "v1.0", nil),
	}

	reqCtx := buildTestRequestContext()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = exec.ExecuteRequestPolicies(
			context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", false)
	}
}

// BenchmarkExecuteRequestPolicies_WithDisabled benchmarks with disabled policies.
func BenchmarkExecuteRequestPolicies_WithDisabled(b *testing.B) {
	tracer := trace.NewNoopTracerProvider().Tracer("bench")
	exec := NewChainExecutor(nil, nil, tracer)

	policies := []policy.Policy{
		&passthroughPolicy{},
		&passthroughPolicy{}, // disabled
		&passthroughPolicy{},
		&passthroughPolicy{}, // disabled
		&passthroughPolicy{},
	}
	specs := []policy.PolicySpec{
		buildPolicySpec("p1", "v1.0", nil),
		buildDisabledPolicySpec("p2", "v1.0"),
		buildPolicySpec("p3", "v1.0", nil),
		buildDisabledPolicySpec("p4", "v1.0"),
		buildPolicySpec("p5", "v1.0", nil),
	}

	reqCtx := buildTestRequestContext()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = exec.ExecuteRequestPolicies(
			context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", false)
	}
}

// BenchmarkExecuteRequestPolicies_CELSkipping benchmarks CEL condition-based skipping.
func BenchmarkExecuteRequestPolicies_CELSkipping(b *testing.B) {
	celEval := &sometimesFalseCELEvaluator{}
	tracer := trace.NewNoopTracerProvider().Tracer("bench")
	exec := NewChainExecutor(nil, celEval, tracer)

	cond := `request.Method == "GET"`
	policies := []policy.Policy{
		&passthroughPolicy{},
		&passthroughPolicy{},
		&passthroughPolicy{},
		&passthroughPolicy{},
		&passthroughPolicy{},
	}
	specs := []policy.PolicySpec{
		buildPolicySpec("p1", "v1.0", &cond),
		buildPolicySpec("p2", "v1.0", &cond),
		buildPolicySpec("p3", "v1.0", &cond),
		buildPolicySpec("p4", "v1.0", &cond),
		buildPolicySpec("p5", "v1.0", &cond),
	}

	reqCtx := buildTestRequestContext()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		celEval.counter = 0 // Reset counter for consistent behavior
		_, _ = exec.ExecuteRequestPolicies(
			context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", true)
	}
}

// =============================================================================
// Response Policy Execution Benchmarks
// =============================================================================

// BenchmarkExecuteResponsePolicies benchmarks response policy chain execution.
func BenchmarkExecuteResponsePolicies(b *testing.B) {
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
				policies = append(policies, &headerModifyPolicy{})
				var condPtr *string
				if sc.withCEL {
					condPtr = &cond
				}
				specs = append(specs, buildPolicySpec(
					fmt.Sprintf("p%d", i), "v1.0", condPtr))
			}

			respCtx := buildTestResponseContext()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = exec.ExecuteResponsePolicies(
					context.Background(), policies, respCtx, specs, "PetStore", "bench-route", sc.withCEL)
			}
		})
	}
}

// =============================================================================
// Parallel Execution Benchmarks
// =============================================================================

// BenchmarkExecuteRequestPolicies_Parallel benchmarks parallel execution.
func BenchmarkExecuteRequestPolicies_Parallel(b *testing.B) {
	tracer := trace.NewNoopTracerProvider().Tracer("bench")
	exec := NewChainExecutor(nil, nil, tracer)

	policies := []policy.Policy{
		&passthroughPolicy{},
		&headerModifyPolicy{},
		&passthroughPolicy{},
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
			// Create fresh context per iteration to avoid data races
			reqCtx := buildTestRequestContext()
			_, _ = exec.ExecuteRequestPolicies(
				context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", false)
		}
	})
}

// =============================================================================
// Latency Distribution Benchmark
// =============================================================================

// BenchmarkExecuteRequestPolicies_RealisticWorkload simulates realistic API gateway workload.
func BenchmarkExecuteRequestPolicies_RealisticWorkload(b *testing.B) {
	// Realistic scenario: auth check + rate limit + header manipulation
	tracer := trace.NewNoopTracerProvider().Tracer("bench")
	exec := NewChainExecutor(nil, nil, tracer)

	policies := []policy.Policy{
		&passthroughPolicy{},  // Auth (simulated as passthrough)
		&passthroughPolicy{},  // Rate limit (simulated as passthrough)
		&headerModifyPolicy{}, // Add correlation headers
	}
	specs := []policy.PolicySpec{
		buildPolicySpec("jwt-auth", "v1.0", nil),
		buildPolicySpec("rate-limit", "v1.0", nil),
		buildPolicySpec("correlation", "v1.0", nil),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reqCtx := buildTestRequestContext()
		_, _ = exec.ExecuteRequestPolicies(
			context.Background(), policies, reqCtx, specs, "PetStore", "bench-route", false)
	}
}

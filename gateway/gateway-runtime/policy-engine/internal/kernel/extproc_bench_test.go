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

package kernel

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// =============================================================================
// Test Setup
// =============================================================================

func TestMain(m *testing.M) {
	// Suppress logging during benchmarks to prevent noise in output
	// and avoid logging overhead affecting benchmark measurements.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})))

	// Disable metrics for clean benchmark measurements.
	// All metrics.*.Inc()/.Observe() calls become zero-cost noops.
	metrics.SetEnabled(false)
	metrics.Init()

	os.Exit(m.Run())
}

// =============================================================================
// Mock Types
// =============================================================================

// mockProcessStream implements extprocv3.ExternalProcessor_ProcessServer
// for benchmarking without actual gRPC transport overhead.
type mockProcessStream struct {
	grpc.ServerStream
	ctx      context.Context
	requests []*extprocv3.ProcessingRequest
	idx      int
	mu       sync.Mutex
}

func (m *mockProcessStream) Context() context.Context {
	return m.ctx
}

func (m *mockProcessStream) Recv() (*extprocv3.ProcessingRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.idx >= len(m.requests) {
		return nil, io.EOF
	}
	req := m.requests[m.idx]
	m.idx++
	return req, nil
}

func (m *mockProcessStream) Send(*extprocv3.ProcessingResponse) error {
	// Discard response for benchmark (no allocation)
	return nil
}

// grpc.ServerStream interface methods (all no-ops)
func (m *mockProcessStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockProcessStream) SendHeader(metadata.MD) error { return nil }
func (m *mockProcessStream) SetTrailer(metadata.MD)       {}
func (m *mockProcessStream) SendMsg(interface{}) error    { return nil }
func (m *mockProcessStream) RecvMsg(interface{}) error    { return nil }

// =============================================================================
// Mock Policy Implementations
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
	return nil // passthrough - no modifications
}

func (p *passthroughPolicy) OnResponse(*policy.ResponseContext, map[string]interface{}) policy.ResponseAction {
	return nil // passthrough - no modifications
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
		SetHeaders: map[string]string{"x-bench-header": "bench-value"},
	}
}

func (p *headerModifyPolicy) OnResponse(*policy.ResponseContext, map[string]interface{}) policy.ResponseAction {
	return policy.UpstreamResponseModifications{
		SetHeaders: map[string]string{"x-bench-resp": "bench-resp-value"},
	}
}

// shortCircuitPolicy returns ImmediateResponse (simulates auth reject).
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

// newBenchServer creates an ExternalProcessorServer for benchmarks.
func newBenchServer(routes map[string]*registry.PolicyChain) *ExternalProcessorServer {
	// Create kernel with route mappings
	kernel := NewKernel()
	kernel.ApplyWholeRoutes(routes)

	// Create executor with no CEL evaluator and noop tracer.
	// nil CELEvaluator means no condition evaluation overhead.
	// noop tracer means no tracing overhead.
	chainExecutor := executor.NewChainExecutor(
		nil, // registry - not used in benchmarks
		nil, // celEvaluator - benchmarks will control this
		trace.NewNoopTracerProvider().Tracer("bench"),
	)

	// Create server with tracing disabled
	return NewExternalProcessorServer(
		kernel,
		chainExecutor,
		config.TracingConfig{Enabled: false},
		"bench-policy-engine",
	)
}

// buildPolicyChain creates a PolicyChain from policies and specs.
func buildPolicyChain(policies []policy.Policy, specs []policy.PolicySpec) *registry.PolicyChain {
	// Compute HasExecutionConditions based on specs
	hasExecutionConditions := false
	for _, spec := range specs {
		if spec.ExecutionCondition != nil && *spec.ExecutionCondition != "" {
			hasExecutionConditions = true
			break
		}
	}
	return &registry.PolicyChain{
		Policies:             policies,
		PolicySpecs:          specs,
		RequiresRequestBody:  false,
		RequiresResponseBody: false,
		HasExecutionConditions:     hasExecutionConditions,
	}
}

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

// buildRequestHeadersProcessingRequest creates a realistic request headers ProcessingRequest.
func buildRequestHeadersProcessingRequest(routeName string) *extprocv3.ProcessingRequest {
	// Build route metadata in protobuf text format (as Envoy sends it)
	metadataStr := `filter_metadata { key: "wso2.route" value { fields { key: "api_name" value { string_value: "PetStore" } } fields { key: "api_version" value { string_value: "v1.0" } } fields { key: "api_context" value { string_value: "/petstore" } } fields { key: "path" value { string_value: "/pets/{id}" } } fields { key: "api_id" value { string_value: "abc-123" } } fields { key: "api_kind" value { string_value: "rest" } } } }`

	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{
					Headers: []*corev3.HeaderValue{
						{Key: ":method", RawValue: []byte("GET")},
						{Key: ":path", RawValue: []byte("/petstore/v1.0/pets/123")},
						{Key: ":authority", RawValue: []byte("api.example.com")},
						{Key: ":scheme", RawValue: []byte("https")},
						{Key: "content-type", RawValue: []byte("application/json")},
						{Key: "x-request-id", RawValue: []byte("bench-req-id-12345")},
						{Key: "authorization", RawValue: []byte("Bearer token123")},
						{Key: "accept", RawValue: []byte("application/json")},
						{Key: "user-agent", RawValue: []byte("bench-client/1.0")},
					},
				},
				EndOfStream: true,
			},
		},
		Attributes: map[string]*structpb.Struct{
			constants.ExtProcFilter: {
				Fields: map[string]*structpb.Value{
					"xds.route_name":     structpb.NewStringValue(routeName),
					"xds.route_metadata": structpb.NewStringValue(metadataStr),
				},
			},
		},
	}
}

// buildResponseHeadersProcessingRequest creates a response headers ProcessingRequest.
func buildResponseHeadersProcessingRequest() *extprocv3.ProcessingRequest {
	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_ResponseHeaders{
			ResponseHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{
					Headers: []*corev3.HeaderValue{
						{Key: ":status", RawValue: []byte("200")},
						{Key: "content-type", RawValue: []byte("application/json")},
						{Key: "x-custom-header", RawValue: []byte("value")},
					},
				},
				EndOfStream: true,
			},
		},
	}
}

// newBenchStream creates a mockProcessStream with request and response phases.
func newBenchStream(routeName string) *mockProcessStream {
	return &mockProcessStream{
		ctx: context.Background(),
		requests: []*extprocv3.ProcessingRequest{
			buildRequestHeadersProcessingRequest(routeName),
			buildResponseHeadersProcessingRequest(),
		},
	}
}

// =============================================================================
// Full Process Benchmarks
// =============================================================================

// BenchmarkProcess benchmarks the full Process() lifecycle with different policy counts.
func BenchmarkProcess(b *testing.B) {
	scenarios := []struct {
		name        string
		numPolicies int
	}{
		{"NoPolicies", 0},
		{"1Policy", 1},
		{"3Policies", 3},
		{"5Policies", 5},
	}

	for _, sc := range scenarios {
		b.Run(sc.name, func(b *testing.B) {
			routeName := "bench-route"

			// Build policy chain
			var policies []policy.Policy
			var specs []policy.PolicySpec
			for i := 0; i < sc.numPolicies; i++ {
				policies = append(policies, &passthroughPolicy{})
				specs = append(specs, buildPolicySpec(
					fmt.Sprintf("passthrough-%d", i), "v1.0", nil))
			}

			// Create server with routes
			routes := make(map[string]*registry.PolicyChain)
			if sc.numPolicies > 0 {
				routes[routeName] = buildPolicyChain(policies, specs)
			}
			server := newBenchServer(routes)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				stream := newBenchStream(routeName)
				_ = server.Process(stream)
			}
		})
	}
}

// BenchmarkProcessParallel benchmarks Process() under parallel load.
func BenchmarkProcessParallel(b *testing.B) {
	routeName := "bench-route-parallel"

	// Create server with one header-modifying policy
	policies := []policy.Policy{&headerModifyPolicy{}}
	specs := []policy.PolicySpec{buildPolicySpec("header-mod", "v1.0", nil)}
	routes := map[string]*registry.PolicyChain{
		routeName: buildPolicyChain(policies, specs),
	}
	server := newBenchServer(routes)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stream := newBenchStream(routeName)
			_ = server.Process(stream)
		}
	})
}

// BenchmarkProcess_ShortCircuit benchmarks short-circuit behavior (auth reject scenario).
func BenchmarkProcess_ShortCircuit(b *testing.B) {
	routeName := "bench-route-sc"

	// Short-circuit policy (returns ImmediateResponse)
	policies := []policy.Policy{&shortCircuitPolicy{}}
	specs := []policy.PolicySpec{buildPolicySpec("auth-reject", "v1.0", nil)}
	routes := map[string]*registry.PolicyChain{
		routeName: buildPolicyChain(policies, specs),
	}
	server := newBenchServer(routes)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Short-circuit: only request headers phase, then EOF
		stream := &mockProcessStream{
			ctx: context.Background(),
			requests: []*extprocv3.ProcessingRequest{
				buildRequestHeadersProcessingRequest(routeName),
			},
		}
		_ = server.Process(stream)
	}
}

// BenchmarkProcess_HeaderModification benchmarks with header modification overhead.
func BenchmarkProcess_HeaderModification(b *testing.B) {
	routeName := "bench-route-header-mod"

	// Multiple header-modifying policies
	policies := []policy.Policy{
		&headerModifyPolicy{},
		&headerModifyPolicy{},
		&headerModifyPolicy{},
	}
	specs := []policy.PolicySpec{
		buildPolicySpec("header-mod-1", "v1.0", nil),
		buildPolicySpec("header-mod-2", "v1.0", nil),
		buildPolicySpec("header-mod-3", "v1.0", nil),
	}
	routes := map[string]*registry.PolicyChain{
		routeName: buildPolicyChain(policies, specs),
	}
	server := newBenchServer(routes)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream := newBenchStream(routeName)
		_ = server.Process(stream)
	}
}

// =============================================================================
// Component Benchmarks
// =============================================================================

// BenchmarkExtractRouteMetadata benchmarks protobuf text parsing for route metadata.
func BenchmarkExtractRouteMetadata(b *testing.B) {
	server := &ExternalProcessorServer{}
	req := buildRequestHeadersProcessingRequest("bench-route")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = server.extractRouteMetadata(req)
	}
}

// BenchmarkBuildRequestContext benchmarks RequestContext construction.
func BenchmarkBuildRequestContext(b *testing.B) {
	b.Run("WithRequestID", func(b *testing.B) {
		server := newBenchServer(nil)
		chain := buildPolicyChain(nil, nil)
		req := buildRequestHeadersProcessingRequest("bench-route")
		routeMetadata := RouteMetadata{
			RouteName:     "bench-route",
			APIName:       "PetStore",
			APIVersion:    "v1.0",
			Context:       "/petstore",
			OperationPath: "/pets/{id}",
			APIId:         "abc-123",
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ec := newPolicyExecutionContext(server, "bench-route", chain)
			ec.buildRequestContext(req.GetRequestHeaders(), routeMetadata)
		}
	})

	b.Run("WithoutRequestID_UUIDGeneration", func(b *testing.B) {
		server := newBenchServer(nil)
		chain := buildPolicyChain(nil, nil)

		// Build request without x-request-id header to force UUID generation
		reqNoID := buildRequestHeadersProcessingRequest("bench-route")
		headers := reqNoID.GetRequestHeaders().Headers.Headers
		filtered := make([]*corev3.HeaderValue, 0, len(headers))
		for _, h := range headers {
			if h.Key != "x-request-id" {
				filtered = append(filtered, h)
			}
		}
		reqNoID.GetRequestHeaders().Headers.Headers = filtered

		routeMetadata := RouteMetadata{RouteName: "bench-route"}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ec := newPolicyExecutionContext(server, "bench-route", chain)
			ec.buildRequestContext(reqNoID.GetRequestHeaders(), routeMetadata)
		}
	})
}

// BenchmarkBuildResponseContext benchmarks ResponseContext construction.
func BenchmarkBuildResponseContext(b *testing.B) {
	server := newBenchServer(nil)
	chain := buildPolicyChain(nil, nil)
	req := buildRequestHeadersProcessingRequest("bench-route")
	routeMetadata := RouteMetadata{RouteName: "bench-route", APIName: "PetStore"}
	respHeaders := buildResponseHeadersProcessingRequest().GetResponseHeaders()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ec := newPolicyExecutionContext(server, "bench-route", chain)
		ec.buildRequestContext(req.GetRequestHeaders(), routeMetadata)
		ec.buildResponseContext(respHeaders)
	}
}

// BenchmarkGetModeOverride benchmarks ProcessingMode override computation.
func BenchmarkGetModeOverride(b *testing.B) {
	chain := &registry.PolicyChain{
		Policies: []policy.Policy{&passthroughPolicy{}, &headerModifyPolicy{}},
		PolicySpecs: []policy.PolicySpec{
			buildPolicySpec("p1", "v1.0", nil),
			buildPolicySpec("p2", "v1.0", nil),
		},
		RequiresRequestBody:  false,
		RequiresResponseBody: false,
		HasExecutionConditions:     false,
	}

	server := newBenchServer(nil)
	ec := newPolicyExecutionContext(server, "bench-route", chain)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ec.getModeOverride()
	}
}

// BenchmarkTranslateRequestHeadersActions benchmarks protobuf response building.
func BenchmarkTranslateRequestHeadersActions(b *testing.B) {
	scenarios := []struct {
		name    string
		results []executor.RequestPolicyResult
	}{
		{
			"NoModifications",
			[]executor.RequestPolicyResult{
				{PolicyName: "p1", PolicyVersion: "v1.0", Action: nil, Skipped: false},
			},
		},
		{
			"WithHeaderMods",
			[]executor.RequestPolicyResult{
				{PolicyName: "p1", PolicyVersion: "v1.0", Action: policy.UpstreamRequestModifications{
					SetHeaders:    map[string]string{"x-added": "val1", "x-added2": "val2"},
					RemoveHeaders: []string{"x-remove-me"},
				}, Skipped: false},
			},
		},
		{
			"MultiplePolicesWithMods",
			[]executor.RequestPolicyResult{
				{PolicyName: "p1", PolicyVersion: "v1.0", Action: policy.UpstreamRequestModifications{
					SetHeaders: map[string]string{"x-p1": "v1"},
				}, Skipped: false},
				{PolicyName: "p2", PolicyVersion: "v1.0", Action: policy.UpstreamRequestModifications{
					SetHeaders:    map[string]string{"x-p2": "v2"},
					RemoveHeaders: []string{"x-p1"},
				}, Skipped: false},
				{PolicyName: "p3", PolicyVersion: "v1.0", Action: nil, Skipped: true},
			},
		},
	}

	for _, sc := range scenarios {
		b.Run(sc.name, func(b *testing.B) {
			chain := buildPolicyChain(
				[]policy.Policy{&passthroughPolicy{}},
				[]policy.PolicySpec{buildPolicySpec("p1", "v1.0", nil)},
			)
			server := newBenchServer(nil)
			ec := newPolicyExecutionContext(server, "bench-route", chain)
			req := buildRequestHeadersProcessingRequest("bench-route")
			routeMetadata := RouteMetadata{RouteName: "bench-route", APIName: "PetStore"}
			ec.buildRequestContext(req.GetRequestHeaders(), routeMetadata)

			execResult := &executor.RequestExecutionResult{
				Results:        sc.results,
				ShortCircuited: false,
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = TranslateRequestHeadersActions(execResult, chain, ec)
			}
		})
	}
}

// BenchmarkGetPolicyChainForKey benchmarks kernel route lookup (RWMutex contention).
func BenchmarkGetPolicyChainForKey(b *testing.B) {
	kernel := NewKernel()
	chain := buildPolicyChain(nil, nil)

	// Register 100 routes to simulate realistic workload
	for i := 0; i < 100; i++ {
		kernel.RegisterRoute(fmt.Sprintf("route-%d", i), chain)
	}

	b.Run("Serial", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = kernel.GetPolicyChainForKey("route-50")
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = kernel.GetPolicyChainForKey("route-50")
			}
		})
	})

	b.Run("ParallelWithWrites", func(b *testing.B) {
		// Simulates scenario with occasional route updates
		var writeCounter int64
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// Every 1000th operation is a write
				if writeCounter%1000 == 0 {
					kernel.RegisterRoute("dynamic-route", chain)
				}
				_ = kernel.GetPolicyChainForKey("route-50")
				writeCounter++
			}
		})
	})
}

// BenchmarkSkipAllProcessing benchmarks the skip-all path (no policy chain found).
func BenchmarkSkipAllProcessing(b *testing.B) {
	server := &ExternalProcessorServer{}
	routeMetadata := RouteMetadata{
		RouteName:     "bench-route",
		APIName:       "PetStore",
		APIVersion:    "v1.0",
		Context:       "/petstore",
		OperationPath: "/pets/{id}",
		APIKind:       "rest",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = server.skipAllProcessing(routeMetadata)
	}
}

// BenchmarkNewPolicyExecutionContext benchmarks context allocation.
func BenchmarkNewPolicyExecutionContext(b *testing.B) {
	server := newBenchServer(nil)
	chain := buildPolicyChain(
		[]policy.Policy{&passthroughPolicy{}},
		[]policy.PolicySpec{buildPolicySpec("p1", "v1.0", nil)},
	)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = newPolicyExecutionContext(server, "bench-route", chain)
	}
}

// =============================================================================
// Throughput Benchmark
// =============================================================================

// BenchmarkThroughput_Target3000RPS measures if we can achieve target throughput.
// This benchmark helps validate optimization progress toward 3000 req/sec target.
func BenchmarkThroughput_Target3000RPS(b *testing.B) {
	routeName := "throughput-test"

	// Realistic scenario: 2 policies (e.g., auth + rate-limit)
	policies := []policy.Policy{&passthroughPolicy{}, &headerModifyPolicy{}}
	specs := []policy.PolicySpec{
		buildPolicySpec("auth", "v1.0", nil),
		buildPolicySpec("ratelimit", "v1.0", nil),
	}
	routes := map[string]*registry.PolicyChain{
		routeName: buildPolicyChain(policies, specs),
	}
	server := newBenchServer(routes)

	b.ReportAllocs()
	b.ResetTimer()

	// Run with parallelism matching typical server workload
	b.SetParallelism(100) // 100 concurrent workers
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stream := newBenchStream(routeName)
			_ = server.Process(stream)
		}
	})

	// Note: After running, check ns/op. For 3000 RPS target:
	// Target latency = 1,000,000,000 ns / 3000 = ~333,333 ns/op (~333µs)
	// With parallelism, actual throughput = (parallelism * 1e9) / (ns/op)
}

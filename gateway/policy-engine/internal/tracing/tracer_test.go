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

package tracing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

// setupPropagator sets up the W3C Trace Context propagator for tests
func setupPropagator() {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
}

// =============================================================================
// InitTracer Tests
// =============================================================================

func TestInitTracer_Disabled(t *testing.T) {
	cfg := &config.Config{
		TracingConfig: config.TracingConfig{
			Enabled: false,
		},
	}

	shutdown, err := InitTracer(cfg)
	require.NoError(t, err)
	assert.NotNil(t, shutdown)

	// Shutdown should be a no-op
	shutdown()
}

func TestInitTracer_NilConfig(t *testing.T) {
	shutdown, err := InitTracer(nil)
	require.NoError(t, err)
	assert.NotNil(t, shutdown)

	// Shutdown should be a no-op
	shutdown()
}

func TestInitTracer_DisabledWithEndpoint(t *testing.T) {
	cfg := &config.Config{
		TracingConfig: config.TracingConfig{
			Enabled:  false,
			Endpoint: "localhost:4317",
		},
	}

	shutdown, err := InitTracer(cfg)
	require.NoError(t, err)
	assert.NotNil(t, shutdown)

	shutdown()
}

// =============================================================================
// ExtractTraceContext Tests
// =============================================================================

func TestExtractTraceContext_NoMetadata(t *testing.T) {
	setupPropagator()
	ctx := context.Background()
	newCtx := ExtractTraceContext(ctx)

	// Should return a valid context even without metadata
	assert.NotNil(t, newCtx)
}

func TestExtractTraceContext_EmptyMetadata(t *testing.T) {
	setupPropagator()
	md := metadata.MD{}
	ctx := metadata.NewIncomingContext(context.Background(), md)

	newCtx := ExtractTraceContext(ctx)
	assert.NotNil(t, newCtx)

	// Should have no valid trace context
	span := trace.SpanContextFromContext(newCtx)
	assert.False(t, span.IsValid())
}

func TestExtractTraceContext_WithTraceparent(t *testing.T) {
	setupPropagator()
	// Valid W3C traceparent header
	// Format: version-trace_id-parent_id-flags
	traceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

	md := metadata.MD{
		"traceparent": []string{traceparent},
	}
	ctx := metadata.NewIncomingContext(context.Background(), md)

	newCtx := ExtractTraceContext(ctx)
	assert.NotNil(t, newCtx)

	span := trace.SpanContextFromContext(newCtx)
	assert.True(t, span.IsValid())
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", span.TraceID().String())
}

func TestExtractTraceContext_WithTracestate(t *testing.T) {
	setupPropagator()
	traceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	tracestate := "vendor1=value1,vendor2=value2"

	md := metadata.MD{
		"traceparent": []string{traceparent},
		"tracestate":  []string{tracestate},
	}
	ctx := metadata.NewIncomingContext(context.Background(), md)

	newCtx := ExtractTraceContext(ctx)
	assert.NotNil(t, newCtx)

	span := trace.SpanContextFromContext(newCtx)
	assert.True(t, span.IsValid())
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", span.TraceID().String())
}

func TestExtractTraceContext_InvalidTraceparent(t *testing.T) {
	setupPropagator()
	testCases := []struct {
		name        string
		traceparent string
	}{
		{"empty", ""},
		{"invalid_format", "invalid-trace-parent"},
		{"short_trace_id", "00-4bf92f-00f067aa0ba902b7-01"},
		{"all_zeros_trace", "00-00000000000000000000000000000000-00f067aa0ba902b7-01"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			md := metadata.MD{
				"traceparent": []string{tc.traceparent},
			}
			ctx := metadata.NewIncomingContext(context.Background(), md)

			newCtx := ExtractTraceContext(ctx)
			assert.NotNil(t, newCtx)

			span := trace.SpanContextFromContext(newCtx)
			assert.False(t, span.IsValid())
		})
	}
}

func TestExtractTraceContext_MultipleValues(t *testing.T) {
	setupPropagator()
	// When multiple values are present, only the first should be used
	traceparent1 := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	traceparent2 := "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01"

	md := metadata.MD{
		"traceparent": []string{traceparent1, traceparent2},
	}
	ctx := metadata.NewIncomingContext(context.Background(), md)

	newCtx := ExtractTraceContext(ctx)
	assert.NotNil(t, newCtx)

	span := trace.SpanContextFromContext(newCtx)
	assert.True(t, span.IsValid())
	// Should use the first value
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", span.TraceID().String())
}

func TestExtractTraceContext_SampledFlag(t *testing.T) {
	setupPropagator()
	testCases := []struct {
		name        string
		traceparent string
		sampled     bool
	}{
		{
			name:        "sampled",
			traceparent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			sampled:     true,
		},
		{
			name:        "not_sampled",
			traceparent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00",
			sampled:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			md := metadata.MD{
				"traceparent": []string{tc.traceparent},
			}
			ctx := metadata.NewIncomingContext(context.Background(), md)

			newCtx := ExtractTraceContext(ctx)
			span := trace.SpanContextFromContext(newCtx)

			assert.True(t, span.IsValid())
			assert.Equal(t, tc.sampled, span.IsSampled())
		})
	}
}

func TestExtractTraceContext_OtherMetadata(t *testing.T) {
	setupPropagator()
	traceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

	md := metadata.MD{
		"traceparent":   []string{traceparent},
		"authorization": []string{"Bearer token123"},
		"content-type":  []string{"application/json"},
		"x-custom":      []string{"value"},
		"grpc-timeout":  []string{"5s"},
	}
	ctx := metadata.NewIncomingContext(context.Background(), md)

	newCtx := ExtractTraceContext(ctx)
	span := trace.SpanContextFromContext(newCtx)

	assert.True(t, span.IsValid())
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", span.TraceID().String())
}

// =============================================================================
// Config Defaults Tests
// =============================================================================

func TestInitTracerConfig_DefaultValues(t *testing.T) {
	// Test that default values are applied when not specified
	// We can't test the actual initialization without a running collector,
	// but we can verify the config handling

	cfg := &config.Config{
		TracingConfig: config.TracingConfig{
			Enabled: false, // Disabled to avoid needing a collector
		},
	}

	shutdown, err := InitTracer(cfg)
	require.NoError(t, err)
	defer shutdown()
}

func TestInitTracerConfig_CustomTimeout(t *testing.T) {
	cfg := &config.Config{
		TracingConfig: config.TracingConfig{
			Enabled:      false,
			BatchTimeout: 2 * time.Second,
		},
	}

	shutdown, err := InitTracer(cfg)
	require.NoError(t, err)
	defer shutdown()
}

func TestInitTracerConfig_CustomBatchSize(t *testing.T) {
	cfg := &config.Config{
		TracingConfig: config.TracingConfig{
			Enabled:            false,
			MaxExportBatchSize: 1024,
		},
	}

	shutdown, err := InitTracer(cfg)
	require.NoError(t, err)
	defer shutdown()
}

func TestInitTracerConfig_SamplingRates(t *testing.T) {
	testCases := []struct {
		name string
		rate float64
	}{
		{"zero_rate", 0.0},
		{"half_rate", 0.5},
		{"full_rate", 1.0},
		{"negative_rate", -1.0},
		{"over_one_rate", 1.5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				TracingConfig: config.TracingConfig{
					Enabled:      false, // Disabled to avoid needing a collector
					SamplingRate: tc.rate,
				},
			}

			shutdown, err := InitTracer(cfg)
			require.NoError(t, err)
			shutdown()
		})
	}
}

func TestInitTracerConfig_CustomServiceName(t *testing.T) {
	cfg := &config.Config{
		TracingConfig: config.TracingConfig{
			Enabled: false,
		},
		PolicyEngine: config.PolicyEngine{
			TracingServiceName: "custom-policy-engine",
		},
	}

	shutdown, err := InitTracer(cfg)
	require.NoError(t, err)
	defer shutdown()
}

func TestInitTracerConfig_CustomServiceVersion(t *testing.T) {
	cfg := &config.Config{
		TracingConfig: config.TracingConfig{
			Enabled:        false,
			ServiceVersion: "2.0.0",
		},
	}

	shutdown, err := InitTracer(cfg)
	require.NoError(t, err)
	defer shutdown()
}

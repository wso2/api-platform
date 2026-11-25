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
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc/metadata"
)

// InitTracer initializes the OpenTelemetry tracer and returns a shutdown function
// InitTracer initializes the OpenTelemetry tracer using values from cfg.
// If tracing is disabled in the configuration, this is a no-op and a
// no-op shutdown function is returned.
func InitTracer(cfg *config.Config) (func(), error) {
	// If tracing not enabled, return no-op
	ctx := context.Background()
	if cfg == nil || !cfg.TracingConfig.Enabled {
		slog.InfoContext(ctx, "Tracing is disabled by configuration")
		return func() {}, nil
	}

	endpoint := cfg.TracingConfig.Endpoint
	if endpoint == "" {
		endpoint = "otel-collector:4317"
	}

	slog.InfoContext(ctx, "Initializing OTLP exporter", "endpoint", endpoint)

	// Create OTLP exporter with configured options
	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(endpoint)}
	if cfg.TracingConfig.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// Ensure exporter is cleaned up on error paths
	success := false
	defer func() {
		if !success {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := exporter.Shutdown(shutdownCtx); err != nil {
				slog.ErrorContext(shutdownCtx, "Error shutting down exporter on init failure", "error", err)
			}
		}
	}()

	serviceName := cfg.PolicyEngine.TracingServiceName
	if serviceName == "" {
		serviceName = "policy-engine"
	}
	serviceVersion := cfg.TracingConfig.ServiceVersion
	if serviceVersion == "" {
		serviceVersion = "1.0.0"
	}

	// Create resource with service information and the configured resource
	// attributes. OTEL_RESOURCE_ATTRIBUTES is not read here: sdktrace.WithResource
	// below merges this resource over resource.Environment(), so environment
	// attributes are picked up automatically and anything set explicitly here wins
	// on a key collision. The gateway-controller gives the same precedence to the
	// router's Envoy resource detectors, so both components in the gateway-runtime
	// container report the same resource attributes.
	attrs := make([]attribute.KeyValue, 0, len(cfg.TracingConfig.ResourceAttributes)+2)
	for k, v := range cfg.TracingConfig.ResourceAttributes {
		attrs = append(attrs, attribute.String(k, v))
	}
	attrs = append(attrs,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
	)

	res, err := resource.New(ctx, resource.WithAttributes(attrs...))
	if err != nil {
		return nil, err
	}

	// Determine batch options
	batchTimeout := cfg.TracingConfig.BatchTimeout
	if batchTimeout <= 0 {
		batchTimeout = time.Second
	}
	maxBatch := cfg.TracingConfig.MaxExportBatchSize
	if maxBatch <= 0 {
		maxBatch = 512
	}

	// Determine sampler based on sampling rate
	samplingRate := cfg.TracingConfig.SamplingRate
	if samplingRate <= 0.0 {
		samplingRate = 1.0 // Default to sampling all requests
	}

	sampler := NewSampler(samplingRate)

	slog.InfoContext(ctx, "Using trace sampler", "sampling_rate", samplingRate)

	// Create trace provider with batch span processor
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(batchTimeout),
			sdktrace.WithMaxExportBatchSize(maxBatch),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	// Set global propagator to W3C Trace Context
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	slog.InfoContext(ctx, "OpenTelemetry tracer initialized successfully")

	// Mark initialization as successful to prevent cleanup of exporter
	success = true

	// Return shutdown function
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			slog.ErrorContext(ctx, "Error shutting down tracer provider", "error", err)
		}
	}, nil
}

// NewSampler returns the policy-engine sampler for the given rate (> 0).
//
// It is parent-based: when the root span has a parent (Envoy's ext_proc span
// from the stream metadata, or a client's traceparent), the parent's sampled
// flag decides, so the policy engine keeps or drops its spans together with the
// router. Envoy makes the decision per request: for a request with no incoming
// traceparent it samples randomly at the same rate, and for one with a
// traceparent it follows that header's sampled flag. Either way it always
// propagates its traceparent to the policy engine, including for requests it
// does not sample, so an independent decision here would export policy-engine
// spans whose router parent was dropped (orphans), and drop policy-engine spans
// from traces the router kept (gaps). The rate applies only to spans with no
// parent.
func NewSampler(samplingRate float64) sdktrace.Sampler {
	if samplingRate >= 1.0 {
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
	return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(samplingRate))
}

// ExtractTraceContext extracts the W3C Trace Context (traceparent/tracestate)
// from the given carrier and returns a context parented on the remote span.
//
// This is a defensive HTTP-header fallback: the carrier is built from the
// downstream request headers that Envoy delivers inside the RequestHeaders
// ProcessingRequest body. It is used only when the ext_proc stream metadata
// carries no valid span context (see ExtractTraceContextFromMetadata). The router
// and the policy engine normally read the same [tracing] configuration, so with
// tracing enabled the metadata is always present; the fallback matters only when
// the router's tracing configuration diverges from the policy engine's (e.g.
// router tracing off) and the client sends a traceparent.
//
// When the carrier holds no valid traceparent, the input context is returned
// unchanged and the caller will start a fresh root trace — an ordinary,
// non-error condition, so it is logged at debug.
func ExtractTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	propagator := otel.GetTextMapPropagator()
	newCtx := propagator.Extract(ctx, carrier)

	span := trace.SpanContextFromContext(newCtx)
	if span.IsValid() {
		slog.DebugContext(ctx, "Extracted trace context from request headers", "trace_id", span.TraceID().String())
	} else {
		slog.DebugContext(ctx, "No trace context in request headers; starting a new root trace")
	}

	return newCtx
}

// ExtractTraceContextFromMetadata extracts the W3C Trace Context from the
// incoming ext_proc gRPC stream metadata. Envoy's async gRPC client injects its
// ext_proc client span's traceparent (and tracestate) there, and one stream
// serves exactly one HTTP request, so this is the correct per-request parent.
// If the context has no metadata or no valid traceparent, ctx is returned with
// no remote span context added.
func ExtractTraceContextFromMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	carrier := propagation.MapCarrier{}
	if v := md.Get("traceparent"); len(v) > 0 {
		carrier["traceparent"] = v[0]
	}
	// tracestate may be split across several values; combine them in order,
	// skipping empty ones, the same way traceContextCarrier does for headers.
	var tracestate []string
	for _, v := range md.Get("tracestate") {
		if v != "" {
			tracestate = append(tracestate, v)
		}
	}
	if len(tracestate) > 0 {
		carrier["tracestate"] = strings.Join(tracestate, ",")
	}
	if len(carrier) == 0 {
		return ctx
	}
	newCtx := otel.GetTextMapPropagator().Extract(ctx, carrier)
	if sc := trace.SpanContextFromContext(newCtx); sc.IsValid() {
		slog.DebugContext(ctx, "Extracted trace context from ext_proc stream metadata", "trace_id", sc.TraceID().String())
	}
	return newCtx
}

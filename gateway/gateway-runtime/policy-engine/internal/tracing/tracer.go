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

	var sampler sdktrace.Sampler
	if samplingRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(samplingRate)
	}

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

// ExtractTraceContext extracts the W3C Trace Context (traceparent/tracestate)
// from the given carrier and returns a context parented on the remote span.
//
// The carrier must be built from the downstream HTTP request headers that Envoy
// delivers inside the RequestHeaders ProcessingRequest body — NOT from the
// ext_proc gRPC stream metadata. The request's traceparent travels in the
// message body, and the long-lived bidirectional ext_proc stream sets its gRPC
// metadata only once at stream establishment, so it never carries a per-request
// traceparent. Reading it from stream metadata always yielded an empty span
// context, producing a new (unparented) root trace for every request.
//
// When the carrier holds no valid traceparent, the input context is returned
// unchanged and the caller will start a fresh root trace — an ordinary,
// non-error condition (e.g. Envoy tracing disabled), so it is logged at debug.
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

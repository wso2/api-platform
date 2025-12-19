package tracing

import (
	"context"
	"log/slog"
	"time"

	"strings"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"

	"github.com/policy-engine/policy-engine/internal/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
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

	serviceName := cfg.PolicyEngine.TracingServiceName
	if serviceName == "" {
		serviceName = "policy-engine"
	}
	serviceVersion := cfg.TracingConfig.ServiceVersion
	if serviceVersion == "" {
		serviceVersion = "1.0.0"
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
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

	// Return shutdown function
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			slog.ErrorContext(ctx, "Error shutting down tracer provider", "error", err)
		}
	}, nil
}

// ExtractTraceContext extracts W3C Trace Context from gRPC metadata
func ExtractTraceContext(ctx context.Context) context.Context {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        slog.DebugContext(ctx, "No gRPC metadata in context")
        return ctx
    }

    // Create carrier from gRPC metadata
    carrier := propagation.MapCarrier{}

    for key, values := range md {
        lowerKey := strings.ToLower(key)
        // gRPC metadata is case-insensitive
        if lowerKey == "traceparent" || lowerKey == "tracestate" {
            if len(values) > 0 {
                carrier.Set(lowerKey, values[0])
                slog.DebugContext(ctx, "Extracted trace header", "header", lowerKey, "value", values[0])
            }
        }
    }

    // Extract using W3C Trace Context propagator
    propagator := otel.GetTextMapPropagator()
    newCtx := propagator.Extract(ctx, carrier)

    // Verify extraction
    span := trace.SpanContextFromContext(newCtx)
    if span.IsValid() {
        slog.DebugContext(ctx, "Successfully extracted trace", "trace_id", span.TraceID().String())
    } else {
        slog.WarnContext(ctx, "No valid trace context extracted")
    }

    return newCtx
}
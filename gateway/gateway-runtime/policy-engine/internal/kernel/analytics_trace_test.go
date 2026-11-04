/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"testing"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func spanWithContext(t *testing.T, flags trace.TraceFlags) trace.Span {
	t.Helper()
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	require.NoError(t, err)
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: flags})
	_, span := noop.NewTracerProvider().Tracer("t").Start(trace.ContextWithSpanContext(t.Context(), sc), "x")
	return span
}

// An analytics event carries the trace it belongs to, so it can be joined to it (#3738).
func TestBuildAnalyticsStruct_StampsSampledTraceContext(t *testing.T) {
	execCtx := &PolicyExecutionContext{sharedCtx: &policy.SharedContext{}, rootSpan: spanWithContext(t, trace.FlagsSampled)}

	result, err := buildAnalyticsStruct(map[string]any{}, execCtx)
	require.NoError(t, err)
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", result.Fields[analytics.TraceIDKey].GetStringValue())
	assert.Equal(t, "00f067aa0ba902b7", result.Fields[analytics.SpanIDKey].GetStringValue())
}

// An unsampled span is never exported, so its ids would point at a trace no backend has.
func TestBuildAnalyticsStruct_OmitsUnsampledTraceContext(t *testing.T) {
	for name, execCtx := range map[string]*PolicyExecutionContext{
		"unsampled": {rootSpan: spanWithContext(t, 0)},
		"no span":   {},
	} {
		t.Run(name, func(t *testing.T) {
			result, err := buildAnalyticsStruct(map[string]any{}, execCtx)
			require.NoError(t, err)
			assert.NotContains(t, result.Fields, analytics.TraceIDKey)
			assert.NotContains(t, result.Fields, analytics.SpanIDKey)
		})
	}
}

// A policy cannot point an event at another trace.
func TestBuildAnalyticsStruct_DropsPolicySuppliedTraceContext(t *testing.T) {
	result, err := buildAnalyticsStruct(map[string]any{
		analytics.TraceIDKey: "ffffffffffffffffffffffffffffffff",
		analytics.SpanIDKey:  "ffffffffffffffff",
	}, &PolicyExecutionContext{})
	require.NoError(t, err)
	assert.NotContains(t, result.Fields, analytics.TraceIDKey)
	assert.NotContains(t, result.Fields, analytics.SpanIDKey)
}

// End to end through Process: the metadata Envoy receives names the root span.
func TestProcess_AnalyticsMetadataCarriesRootSpanContext(t *testing.T) {
	server, k, sr := newSpanStatusServer(t)
	registerTestRoute(k, "pets", emptyChain())

	stream := newMockStream([]*extprocv3.ProcessingRequest{requestHeadersReq("pets", "GET", "/pets")})
	require.NoError(t, server.Process(stream))

	root := spanByName(sr.Ended(), constants.SpanExternalProcessingProcess)
	require.NotNil(t, root)
	require.NotEmpty(t, stream.responses)
	fields := analyticsFields(t, stream.responses[0])
	assert.Equal(t, root.SpanContext().TraceID().String(), fields[analytics.TraceIDKey])
	assert.Equal(t, root.SpanContext().SpanID().String(), fields[analytics.SpanIDKey])
}

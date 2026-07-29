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

package tracing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
)

// newRecordedSpan starts a span backed by a real SDK TracerProvider wired to sr,
// so RecordHTTPOutcome exercises the real IsRecording()/SetAttributes()/SetStatus()
// implementations rather than a noop stand-in.
func newRecordedSpan(t *testing.T) (trace.Span, *tracetest.SpanRecorder) {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	_, span := tp.Tracer("test").Start(context.Background(), "test-span")
	return span, sr
}

func endedSpan(t *testing.T, span trace.Span, sr *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()
	span.End()
	ended := sr.Ended()
	require.Len(t, ended, 1)
	return ended[0]
}

func attrValue(t *testing.T, span sdktrace.ReadOnlySpan, key attribute.Key) (attribute.Value, bool) {
	t.Helper()
	for _, kv := range span.Attributes() {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func TestRecordHTTPOutcome_Table(t *testing.T) {
	tests := []struct {
		name           string
		outcome        HTTPOutcome
		wantCode       codes.Code
		wantAttrCount  int // number of terminal attributes expected (status/reason/errorID)
		wantStatusCode int // -1 = not present
		wantReason     string
		wantErrorID    string
	}{
		{
			name:           "500 is Error with description",
			outcome:        HTTPOutcome{StatusCode: 500, Reason: constants.TerminalReasonPolicyError},
			wantCode:       codes.Error,
			wantStatusCode: 500,
			wantReason:     constants.TerminalReasonPolicyError,
		},
		{
			name:           "502 is Error",
			outcome:        HTTPOutcome{StatusCode: 502, Reason: constants.TerminalReasonUpstream},
			wantCode:       codes.Error,
			wantStatusCode: 502,
			wantReason:     constants.TerminalReasonUpstream,
		},
		{
			name:           "599 is Error",
			outcome:        HTTPOutcome{StatusCode: 599},
			wantCode:       codes.Error,
			wantStatusCode: 599,
		},
		{
			name:           "413 stays Unset (client fault, semconv-strict)",
			outcome:        HTTPOutcome{StatusCode: 413, Reason: constants.TerminalReasonPayloadTooLarge},
			wantCode:       codes.Unset,
			wantStatusCode: 413,
			wantReason:     constants.TerminalReasonPayloadTooLarge,
		},
		{
			name:           "401 stays Unset",
			outcome:        HTTPOutcome{StatusCode: 401, Reason: constants.TerminalReasonPolicyDenied},
			wantCode:       codes.Unset,
			wantStatusCode: 401,
			wantReason:     constants.TerminalReasonPolicyDenied,
		},
		{
			name:           "403 stays Unset",
			outcome:        HTTPOutcome{StatusCode: 403, Reason: constants.TerminalReasonPolicyDenied},
			wantCode:       codes.Unset,
			wantStatusCode: 403,
			wantReason:     constants.TerminalReasonPolicyDenied,
		},
		{
			name:           "429 stays Unset",
			outcome:        HTTPOutcome{StatusCode: 429, Reason: constants.TerminalReasonPolicyDenied},
			wantCode:       codes.Unset,
			wantStatusCode: 429,
			wantReason:     constants.TerminalReasonPolicyDenied,
		},
		{
			name:           "200 is Unset, never Ok",
			outcome:        HTTPOutcome{StatusCode: 200, Reason: constants.TerminalReasonUpstream},
			wantCode:       codes.Unset,
			wantStatusCode: 200,
			wantReason:     constants.TerminalReasonUpstream,
		},
		{
			name:           "204 is Unset",
			outcome:        HTTPOutcome{StatusCode: 204},
			wantCode:       codes.Unset,
			wantStatusCode: 204,
		},
		{
			name:           "304 is Unset",
			outcome:        HTTPOutcome{StatusCode: 304},
			wantCode:       codes.Unset,
			wantStatusCode: 304,
		},
		{
			name:           "zero value is a no-op",
			outcome:        HTTPOutcome{},
			wantCode:       codes.Unset,
			wantStatusCode: -1,
		},
		{
			name:           "sub-100 status code omits the attribute",
			outcome:        HTTPOutcome{StatusCode: 99, Reason: constants.TerminalReasonPolicyDenied},
			wantCode:       codes.Unset,
			wantStatusCode: -1,
			wantReason:     constants.TerminalReasonPolicyDenied,
		},
		{
			name:           "empty reason/errorID are omitted",
			outcome:        HTTPOutcome{StatusCode: 200},
			wantCode:       codes.Unset,
			wantStatusCode: 200,
		},
		{
			name:           "errorID recorded when set",
			outcome:        HTTPOutcome{StatusCode: 500, Reason: constants.TerminalReasonPolicyError, ErrorID: "abc-123"},
			wantCode:       codes.Error,
			wantStatusCode: 500,
			wantReason:     constants.TerminalReasonPolicyError,
			wantErrorID:    "abc-123",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			span, sr := newRecordedSpan(t)
			RecordHTTPOutcome(span, tc.outcome)
			ended := endedSpan(t, span, sr)

			assert.Equal(t, tc.wantCode, ended.Status().Code)
			assert.NotEqual(t, codes.Ok, ended.Status().Code, "RecordHTTPOutcome must never set codes.Ok")

			statusVal, ok := attrValue(t, ended, semconv.HTTPResponseStatusCodeKey)
			if tc.wantStatusCode == -1 {
				assert.False(t, ok, "did not expect http.response.status_code attribute")
			} else {
				require.True(t, ok, "expected http.response.status_code attribute")
				assert.Equal(t, int64(tc.wantStatusCode), statusVal.AsInt64())
			}

			reasonVal, ok := attrValue(t, ended, attribute.Key(constants.AttrTerminalReason))
			if tc.wantReason == "" {
				assert.False(t, ok, "did not expect terminal.reason attribute")
			} else {
				require.True(t, ok, "expected terminal.reason attribute")
				assert.Equal(t, tc.wantReason, reasonVal.AsString())
			}

			errIDVal, ok := attrValue(t, ended, attribute.Key(constants.AttrTerminalErrorID))
			if tc.wantErrorID == "" {
				assert.False(t, ok, "did not expect terminal.error_id attribute")
			} else {
				require.True(t, ok, "expected terminal.error_id attribute")
				assert.Equal(t, tc.wantErrorID, errIDVal.AsString())
			}

			if tc.wantCode == codes.Error && tc.wantStatusCode >= 500 {
				assert.NotEmpty(t, ended.Status().Description)
			}

			// RecordHTTPOutcome deliberately never calls RecordError.
			assert.Empty(t, ended.Events())
		})
	}
}

func TestRecordHTTPOutcome_500Description(t *testing.T) {
	span, sr := newRecordedSpan(t)
	RecordHTTPOutcome(span, HTTPOutcome{StatusCode: 500})
	ended := endedSpan(t, span, sr)
	assert.Equal(t, "HTTP 500 Internal Server Error", ended.Status().Description)
}

func TestRecordHTTPOutcome_NilSpan(t *testing.T) {
	assert.NotPanics(t, func() {
		RecordHTTPOutcome(nil, HTTPOutcome{StatusCode: 500})
	})
}

func TestRecordHTTPOutcome_NoopSpan(t *testing.T) {
	_, span := noop.NewTracerProvider().Tracer("test").Start(context.Background(), "noop-span")
	assert.NotPanics(t, func() {
		RecordHTTPOutcome(span, HTTPOutcome{StatusCode: 500})
	})
}

// TestRecordHTTPOutcome_NoDowngrade locks in the SDK's monotonic SetStatus
// guarantee: a Go error already recorded as Error must keep its status and
// description when a later sub-500 outcome is stamped on the same span.
func TestRecordHTTPOutcome_NoDowngrade(t *testing.T) {
	span, sr := newRecordedSpan(t)
	span.SetStatus(codes.Error, "boom")

	RecordHTTPOutcome(span, HTTPOutcome{StatusCode: 200, Reason: constants.TerminalReasonUpstream})
	ended := endedSpan(t, span, sr)

	assert.Equal(t, codes.Error, ended.Status().Code)
	assert.Equal(t, "boom", ended.Status().Description)

	statusVal, ok := attrValue(t, ended, semconv.HTTPResponseStatusCodeKey)
	require.True(t, ok)
	assert.Equal(t, int64(200), statusVal.AsInt64())
}

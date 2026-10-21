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
	"go.opentelemetry.io/otel/codes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// A policy's outbound call becomes a CLIENT span under the policy's span and carries its
// context to the service; a call with no trace is left alone.
func TestWrapTransport_ChildSpanAndTraceparentOnlyWithParent(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prevTP, prevProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
		_ = tp.Shutdown(context.Background())
	})

	var gotTraceparent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTraceparent = r.Header.Get("traceparent")
	}))
	defer srv.Close()
	client := &http.Client{Transport: WrapTransport(http.DefaultTransport)}

	call := func(ctx context.Context) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/moderate", nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
	}

	call(context.Background())
	assert.Empty(t, gotTraceparent, "a call with no trace must not start one")
	assert.Empty(t, sr.Ended())

	ctx, policySpan := tp.Tracer("test").Start(context.Background(), "policy.request.guardrail")
	call(ctx)
	policySpan.End()

	var client0 sdktrace.ReadOnlySpan
	for _, s := range sr.Ended() {
		if s.SpanKind() == trace.SpanKindClient {
			client0 = s
		}
	}
	require.NotNil(t, client0, "the outbound call must have a client span")
	assert.Equal(t, policySpan.SpanContext().SpanID(), client0.Parent().SpanID())
	assert.Contains(t, gotTraceparent, client0.SpanContext().SpanID().String(),
		"the service must receive the client span as its parent")
}

// The client span follows the HTTP client conventions, keeps credentials in the URL's
// user info and query out of the trace, fails on an error status, and leaves the
// caller's request untouched.
func TestWrapTransport_AttributesAndErrors(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prevTP, prevProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
		_ = tp.Shutdown(context.Background())
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	client := &http.Client{Transport: WrapTransport(http.DefaultTransport)}

	ctx, parent := tp.Tracer("test").Start(context.Background(), "policy")
	u := strings.Replace(srv.URL, "http://", "http://user:secret@", 1) + "/moderate?api_key=abc"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	parent.End()

	assert.Empty(t, req.Header.Get("traceparent"), "the caller's request must not be modified")
	var span sdktrace.ReadOnlySpan
	for _, s := range sr.Ended() {
		if s.SpanKind() == trace.SpanKindClient {
			span = s
		}
	}
	require.NotNil(t, span)
	assert.Equal(t, "POST", span.Name())
	attrs := map[string]string{}
	for _, kv := range span.Attributes() {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	assert.Equal(t, "POST", attrs["http.request.method"])
	assert.Equal(t, strings.TrimPrefix(srv.URL, "http://"), attrs["server.address"]+":"+attrs["server.port"])
	assert.Equal(t, srv.URL+"/moderate", attrs["url.full"], "no user info and no query")
	assert.Equal(t, "503", attrs["http.response.status_code"])
	assert.Equal(t, "503", attrs["error.type"])
	assert.Equal(t, codes.Error, span.Status().Code)
}

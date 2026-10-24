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
	"net/http"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// transportScope is the instrumentation scope of the outbound client spans.
const transportScope = "github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/tracing"

// WrapTransport instruments the engine's shared outbound HTTP client, the one policies
// call guardrail, moderation and embedding services with. A call made with a traced
// context gets a CLIENT span under the calling policy's span, following the
// OpenTelemetry HTTP client conventions, and the request carries that span's context
// to the service. A call made without one (at startup, or from a background refresh)
// is left untouched, so it starts no orphan trace.
func WrapTransport(rt http.RoundTripper) http.RoundTripper {
	return tracedTransport{next: rt}
}

type tracedTransport struct {
	next http.RoundTripper
}

// RoundTrip ends the span when the response headers arrive.
//
// shortcut: the span excludes the time the policy then spends reading the body; wrap
// the body to end it on Close if body-read time matters.
func (t tracedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if !trace.SpanContextFromContext(r.Context()).IsValid() {
		return t.next.RoundTrip(r)
	}
	ctx, span := otel.Tracer(transportScope).Start(r.Context(), r.Method,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(requestAttributes(r)...))
	defer span.End()

	// A RoundTripper must not modify the caller's request, so the headers go on a copy.
	r = r.Clone(ctx)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(r.Header))

	resp, err := t.next.RoundTrip(r)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(semconv.ErrorTypeOther)
		span.SetStatus(codes.Error, err.Error())
		return resp, err
	}
	span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(resp.StatusCode))
	if resp.StatusCode >= 400 {
		span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(resp.StatusCode)))
		span.SetStatus(codes.Error, "")
	}
	return resp, nil
}

// requestAttributes are the method, the server and the URL. The URL leaves out user
// info and the query, which may carry credentials such as an API key.
func requestAttributes(r *http.Request) []attribute.KeyValue {
	u := *r.URL
	u.User, u.RawQuery, u.ForceQuery, u.Fragment = nil, "", false, ""
	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(r.Method),
		semconv.URLFullKey.String(u.String()),
	}
	host, port := u.Hostname(), u.Port()
	if port == "" {
		port = map[string]string{"https": "443", "http": "80"}[u.Scheme]
	}
	if host != "" {
		attrs = append(attrs, semconv.ServerAddressKey.String(host))
		if n, err := strconv.Atoi(port); err == nil {
			attrs = append(attrs, semconv.ServerPortKey.Int(n))
		}
	}
	return attrs
}

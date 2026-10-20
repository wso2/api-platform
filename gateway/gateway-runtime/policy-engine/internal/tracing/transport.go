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

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

// WrapTransport instruments the engine's shared outbound HTTP client, the one policies
// call guardrail, moderation and embedding services with. A call made with a traced
// context gets a CLIENT span under the calling policy's span, and the request carries
// that span's context to the service. A call made without one (at startup, or from a
// background refresh) is left untouched, so it starts no orphan trace.
func WrapTransport(rt http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(rt, otelhttp.WithFilter(func(r *http.Request) bool {
		return trace.SpanContextFromContext(r.Context()).IsValid()
	}))
}

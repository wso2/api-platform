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
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
)

// HTTPOutcome is the terminal HTTP outcome of a request as far as the policy
// engine is concerned: the status code the downstream client will see, why the
// request ended that way, and the correlation id (if one was minted).
//
// The zero value means "no terminal status known yet" — RecordHTTPOutcome
// treats it as a no-op, which is the correct behaviour for a request phase that
// completed without terminating the request.
type HTTPOutcome struct {
	// StatusCode is the HTTP status the policy engine classifies this request as
	// having terminated with. 0 = unknown. For a normal completion (pass-through
	// or a headers-phase ImmediateResponse) this is what the downstream client
	// actually receives. It is NOT a wire guarantee in every case: a fault raised
	// mid-response-stream (after headers/chunks are already committed downstream)
	// records the engine's classification of the failure, but Envoy discards the
	// ImmediateResponse and resets the HTTP/2 stream instead of delivering this
	// status to the client. Treat this field as "how the engine explains the
	// outcome for observability", not as ground truth for what was on the wire.
	StatusCode int
	// Reason is one of the constants.TerminalReason* values. "" to omit.
	Reason string
	// ErrorID correlates the span with the x-error-id response header and the
	// error log line for engine-generated 5xx/413 responses. "" to omit.
	ErrorID string
}

// upstreamFaultReasons enumerates the terminal reasons whose >= 500 status did
// not originate in this process. The status code is still recorded so a backend
// fault stays queryable, but the span status stays Unset: this span covers the
// policy engine's own ext_proc stream, and a failing backend must not count
// against the policy engine's error rate — Envoy owns the span that represents
// the client-facing HTTP transaction.
//
// Deliberately a denylist, not an allowlist: a terminal reason added later
// defaults to being reported as an engine fault, so a new engine-side failure
// mode can never be silently swallowed.
var upstreamFaultReasons = map[string]bool{
	constants.TerminalReasonUpstream: true,
}

// RecordHTTPOutcome stamps a request's terminal HTTP outcome onto span.
//
// Semantics follow the OpenTelemetry HTTP semantic conventions:
//
//   - http.response.status_code is ALWAYS recorded for a valid status (>= 100),
//     including 4xx, so client-fault denials stay filterable by tag even though
//     they are not span errors.
//   - The span status is set to Error only for a >= 500 this process is
//     responsible for. A 4xx is a client fault, and an upstream 5xx
//     (terminal.reason=upstream_response) is the backend's, so both stay Unset —
//     that is what keeps auth/rate-limit denials and backend outages out of this
//     service's error rate. Find them by http.response.status_code +
//     terminal.reason instead.
//   - codes.Ok is NEVER set. Unset is the correct terminal status for a
//     successful server span, and because the SDK ignores any status change once
//     a higher code is set (Unset < Error < Ok), setting Ok would permanently
//     block later error reporting on the same span.
func RecordHTTPOutcome(span trace.Span, out HTTPOutcome) {
	if span == nil || !span.IsRecording() {
		return
	}

	attrs := make([]attribute.KeyValue, 0, 3)
	if out.StatusCode >= 100 {
		attrs = append(attrs, semconv.HTTPResponseStatusCode(out.StatusCode))
	}
	if out.Reason != "" {
		attrs = append(attrs, attribute.String(constants.AttrTerminalReason, out.Reason))
	}
	if out.ErrorID != "" {
		attrs = append(attrs, attribute.String(constants.AttrTerminalErrorID, out.ErrorID))
	}
	if len(attrs) == 0 {
		return
	}
	span.SetAttributes(attrs...)

	if out.StatusCode >= 500 && !upstreamFaultReasons[out.Reason] {
		if text := http.StatusText(out.StatusCode); text != "" {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d %s", out.StatusCode, text))
		} else {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", out.StatusCode))
		}
	}
}

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
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
)

// bodyStreamTrace is the tracing state of one streamed body: a single phase span over
// every body message of the stream, and one span per policy (executor.StreamSpans).
// Without it each message started its own phase span and its own policy spans, so a
// streamed LLM response of a few hundred chunks produced hundreds of spans.
type bodyStreamTrace struct {
	ctx      context.Context
	span     trace.Span
	policies *executor.StreamSpans
	chunks   int
	done     bool
}

// start returns the stream's phase span, starting it on the first message, and counts
// the message.
func (t *bodyStreamTrace) start(ctx context.Context, tracer trace.Tracer, name string) (context.Context, trace.Span) {
	if t.span == nil {
		t.ctx, t.span = tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))
		t.policies = executor.NewStreamSpans()
	}
	t.chunks++
	return t.ctx, t.span
}

// end ends the policy spans, then the phase span. Safe to call more than once, and on a
// stream that never started.
func (t *bodyStreamTrace) end() {
	if t.span == nil || t.done {
		return
	}
	t.done = true
	t.policies.End()
	if t.span.IsRecording() {
		t.span.SetAttributes(attribute.Int(constants.AttrStreamChunkCount, t.chunks))
	}
	t.span.End()
}

// policySpans returns the stream's policy spans, or nil when the stream is not traced
// as one, in which case the executor spans each call on its own.
func (t *bodyStreamTrace) policySpans() *executor.StreamSpans {
	if t.done {
		return nil
	}
	return t.policies
}

// active reports whether the next body message belongs to a stream traced as one.
func (t *bodyStreamTrace) active(streaming bool) bool {
	return streaming && !t.done
}

// startBodyPhaseSpan starts the span for one body message. A message of a streamed body
// (stream non-nil) gets the stream's shared span, which the caller ends with stream.end
// once the stream is over; the returned func ends a per-message span and is a no-op for
// a stream.
func (s *ExternalProcessorServer) startBodyPhaseSpan(ctx context.Context, name string, stream *bodyStreamTrace) (context.Context, trace.Span, func()) {
	if stream != nil {
		ctx, span := stream.start(ctx, s.tracer, name)
		return ctx, span, func() {}
	}
	ctx, span := s.tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))
	return ctx, span, func() { span.End() }
}

// endStreamTraces ends whichever body streams are still open, for a stream that closed
// before its body did: a cancel, a receive or send error, or an immediate response.
func (ec *PolicyExecutionContext) endStreamTraces() {
	ec.requestStream.end()
	ec.responseStream.end()
}

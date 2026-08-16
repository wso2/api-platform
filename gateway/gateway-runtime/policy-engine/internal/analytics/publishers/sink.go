/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package publishers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
)

// Sink is a destination for serialized traffic-log lines.
//
// Implementations must be safe for concurrent use: Write is called from the ALS
// ingest path, which serves multiple concurrent Envoy access-log streams.
//
// A traffic-log line can carry request and response bodies, so the choice of sink
// is a data-protection decision. Two rules follow from that and apply to every
// implementation:
//
//   - A sink that cannot deliver a line drops it and counts it. It must never fall
//     back to stdout, which would put the bodies into the container log — the exact
//     disclosure a non-stdout sink is configured to prevent.
//   - A sink failure must never surface to the client. Traffic logging is strictly
//     downstream of request handling.
type Sink interface {
	// Write emits one complete JSON line (without a trailing newline; the sink
	// adds its own). It must not block the caller for more than a bounded, short
	// interval: the caller runs on the access-log ingest path, and blocking there
	// backpressures Envoy's ALS stream and eventually Envoy itself.
	Write(line []byte)

	// Close flushes anything buffered and releases resources. It must be
	// idempotent and must respect ctx's deadline.
	Close(ctx context.Context) error

	// Name identifies the sink in log messages and metric labels. It matches the
	// corresponding config.TrafficLogSink* constant.
	Name() string
}

// errorLogInterval is the minimum gap between two error log lines from the same
// sink. A sink failure is usually persistent (a full disk, an unreachable
// receiver), so an unthrottled error-per-event would turn one fault into a log
// storm that fills whatever storage is still writable.
const errorLogInterval = 30 * time.Second

// errThrottle rate-limits repeated error logging from a sink. The zero value is
// ready to use and permits the first call.
type errThrottle struct {
	mu         sync.Mutex
	lastLogged time.Time
	suppressed int
}

// allow reports whether an error should be logged now, and returns the number of
// occurrences suppressed since the previous permitted log so the caller can say
// how much it is hiding.
func (t *errThrottle) allow(now time.Time) (bool, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.lastLogged.IsZero() && now.Sub(t.lastLogged) < errorLogInterval {
		t.suppressed++
		return false, 0
	}
	suppressed := t.suppressed
	t.suppressed = 0
	t.lastLogged = now
	return true, suppressed
}

// logError emits a rate-limited error for a sink failure. The line itself is never
// logged: it may contain request/response bodies, so echoing it into the
// application log would reintroduce the very exposure the sink exists to avoid.
func (t *errThrottle) logError(msg, sink string, err error) {
	if ok, suppressed := t.allow(time.Now()); ok {
		attrs := []any{"sink", sink, "error", err}
		if suppressed > 0 {
			attrs = append(attrs, "suppressedSinceLastLog", suppressed)
		}
		slog.Error(msg, attrs...)
	}
}

// writerSink writes each line to an io.Writer, serialized by a mutex so concurrent
// ALS streams cannot interleave partial lines. It backs the stdout sink and is used
// directly by tests to capture output.
type writerSink struct {
	mu   sync.Mutex
	w    io.Writer
	name string
	// closer is called by Close when the underlying writer owns a resource. Nil
	// for stdout, which this sink does not own and must not close.
	closer   io.Closer
	throttle errThrottle
}

// newWriterSink wraps an io.Writer as a Sink. The writer is not closed by Close
// unless it is also passed as closer.
func newWriterSink(w io.Writer, name string, closer io.Closer) *writerSink {
	return &writerSink{w: w, name: name, closer: closer}
}

// newStdoutSink returns the default sink: one JSON line per event on the process's
// stdout. This is the historical behavior and is byte-identical to it.
//
// Note that in the gateway-runtime container the entrypoint wraps the policy
// engine's stdout to prefix every line with "[pol] ", so lines from this sink
// arrive downstream prefixed and are not valid JSON on their own. The file and
// http sinks bypass that wrapper and emit the raw JSON.
func newStdoutSink() *writerSink {
	return newWriterSink(os.Stdout, sinkNameStdout, nil)
}

// Name returns the sink's identifier.
func (s *writerSink) Name() string { return s.name }

// Write appends the line and a newline to the underlying writer.
func (s *writerSink) Write(line []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := fmt.Fprintln(s.w, string(line)); err != nil {
		mDropped(s.name, dropReasonWriteFailed, 1)
		mWriteError(s.name, errCodeWrite, 1)
		s.throttle.logError("Failed to write traffic-log event", s.name, err)
		return
	}
	mWritten(s.name, 1)
}

// Close releases the underlying writer when this sink owns it. Writes go straight
// to the file descriptor with no userspace buffering, so there is nothing to flush.
func (s *writerSink) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closer == nil {
		return nil
	}
	closer := s.closer
	s.closer = nil // idempotent: a second Close is a no-op
	return closer.Close()
}

// Metric helpers.
//
// Every traffic-log metric goes through these rather than touching the package
// vars directly. The vars are nil until metrics.Init() runs — main() calls it
// long before any sink exists, but a sink constructor must not depend on that
// ordering, and guarding only in the constructor while the write path
// dereferences freely is worse than either choice made consistently: it turns a
// startup panic into a first-request panic.
//
// The nil check is a single interface comparison against a write path that
// already does a syscall, so the cost is not measurable.

func mWritten(sink string, n int) {
	if metrics.TrafficLogWrittenTotal != nil {
		metrics.TrafficLogWrittenTotal.WithLabelValues(sink).Add(float64(n))
	}
}

func mDropped(sink, reason string, n int) {
	if metrics.TrafficLogDroppedTotal != nil {
		metrics.TrafficLogDroppedTotal.WithLabelValues(sink, reason).Add(float64(n))
	}
}

func mWriteError(sink, code string, n int) {
	if metrics.TrafficLogWriteErrorsTotal != nil {
		metrics.TrafficLogWriteErrorsTotal.WithLabelValues(sink, code).Add(float64(n))
	}
}

func mQueueDepth(sink string, depth int) {
	if metrics.TrafficLogQueueDepth != nil {
		metrics.TrafficLogQueueDepth.WithLabelValues(sink).Set(float64(depth))
	}
}

func mQueueCapacity(sink string, capacity int) {
	if metrics.TrafficLogQueueCapacity != nil {
		metrics.TrafficLogQueueCapacity.WithLabelValues(sink).Set(float64(capacity))
	}
}

func mFlushDuration(sink string, seconds float64) {
	if metrics.TrafficLogFlushDurationSecond != nil {
		metrics.TrafficLogFlushDurationSecond.WithLabelValues(sink).Observe(seconds)
	}
}

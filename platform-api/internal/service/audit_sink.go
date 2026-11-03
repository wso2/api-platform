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

package service

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
)

// auditSink is a destination for audit lines besides the audit table.
//
// Implementations must be safe for concurrent use. A sink that cannot deliver a
// line drops it and logs; a sink failure never fails the request being audited.
type auditSink interface {
	// Write emits one JSON line, without a trailing newline. It runs on the
	// request path, so it must not block for more than a short, bounded time.
	Write(line []byte)

	// Close flushes anything buffered and releases resources. It must be
	// idempotent and respect ctx's deadline.
	Close(ctx context.Context) error

	// Name is the output name from audit.outputs.
	Name() string
}

// auditErrorLogInterval is the minimum gap between two error logs from one sink.
// A sink failure is usually persistent (a full disk, a receiver that is down), so
// logging every failed record would turn one fault into a log storm.
const auditErrorLogInterval = 30 * time.Second

// auditErrThrottle rate-limits repeated error logging from a sink. The zero value
// is ready to use and permits the first call.
type auditErrThrottle struct {
	mu         sync.Mutex
	lastLogged time.Time
	suppressed int
}

func (t *auditErrThrottle) logError(logger *slog.Logger, msg, sink string, err error) {
	t.mu.Lock()
	now := time.Now()
	if !t.lastLogged.IsZero() && now.Sub(t.lastLogged) < auditErrorLogInterval {
		t.suppressed++
		t.mu.Unlock()
		return
	}
	suppressed := t.suppressed
	t.suppressed = 0
	t.lastLogged = now
	t.mu.Unlock()

	attrs := []any{"sink", sink, "error", err}
	if suppressed > 0 {
		attrs = append(attrs, "suppressedSinceLastLog", suppressed)
	}
	logger.Error(msg, attrs...)
}

// auditWriterSink writes each line to an io.Writer it does not own, serialized by
// a mutex so concurrent records cannot interleave. It backs the stdout output.
type auditWriterSink struct {
	mu       sync.Mutex
	w        io.Writer
	name     string
	logger   *slog.Logger
	throttle auditErrThrottle
	// buf is reused across writes to append the newline, so each line is one Write.
	buf []byte
}

func newAuditStdoutSink(logger *slog.Logger) *auditWriterSink {
	return &auditWriterSink{w: os.Stdout, name: config.AuditOutputStdout, logger: logger}
}

func (s *auditWriterSink) Name() string { return s.name }

func (s *auditWriterSink) Write(line []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = append(append(s.buf[:0], line...), '\n')
	if _, err := s.w.Write(s.buf); err != nil {
		s.throttle.logError(s.logger, "Failed to write audit record; dropping it", s.name, err)
	}
}

// Close is a no-op: the writer is not owned by this sink and is unbuffered.
func (s *auditWriterSink) Close(context.Context) error { return nil }

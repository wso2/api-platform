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

package audit

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// sink must be safe for concurrent use. A sink that cannot deliver a line
// drops it and logs; it never fails the request.
type sink interface {
	// Write emits one JSON line, without a trailing newline. It runs on the
	// request path, so it must not block for more than a short, bounded time.
	Write(line []byte)

	Close(ctx context.Context) error

	Name() string
}

// Sink failures are usually persistent, so log at most one error per interval.
const errorLogInterval = 30 * time.Second

type errThrottle struct {
	mu         sync.Mutex
	lastLogged time.Time
	suppressed int
}

func (t *errThrottle) logError(logger *slog.Logger, msg, sinkName string, err error) {
	t.mu.Lock()
	now := time.Now()
	if !t.lastLogged.IsZero() && now.Sub(t.lastLogged) < errorLogInterval {
		t.suppressed++
		t.mu.Unlock()
		return
	}
	suppressed := t.suppressed
	t.suppressed = 0
	t.lastLogged = now
	t.mu.Unlock()

	attrs := []any{"sink", sinkName, "error", err}
	if suppressed > 0 {
		attrs = append(attrs, "suppressedSinceLastLog", suppressed)
	}
	logger.Error(msg, attrs...)
}

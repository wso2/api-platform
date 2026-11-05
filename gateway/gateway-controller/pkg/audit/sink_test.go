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
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// logBuffer collects what a sink logs; sinks log from their own goroutines.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newTestLogger() (*slog.Logger, *logBuffer) {
	logs := &logBuffer{}
	return slog.New(slog.NewTextHandler(logs, nil)), logs
}

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestErrThrottleLogsOncePerInterval(t *testing.T) {
	logger, logs := newTestLogger()
	var throttle errThrottle

	for range 3 {
		throttle.logError(logger, "write failed", "file", errors.New("disk full"))
	}
	if n := strings.Count(logs.String(), "write failed"); n != 1 {
		t.Fatalf("logged %d times within the interval, want 1:\n%s", n, logs)
	}

	throttle.lastLogged = time.Now().Add(-errorLogInterval)
	throttle.logError(logger, "write failed", "file", errors.New("disk full"))
	if n := strings.Count(logs.String(), "write failed"); n != 2 {
		t.Fatalf("logged %d times after the interval, want 2:\n%s", n, logs)
	}
	if !strings.Contains(logs.String(), "suppressedSinceLastLog=2") {
		t.Fatalf("the second log does not count the 2 suppressed errors:\n%s", logs)
	}
}

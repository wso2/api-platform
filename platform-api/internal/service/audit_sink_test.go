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
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// auditLogBuffer collects what a sink logs; sinks log from their own goroutines.
type auditLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *auditLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *auditLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newAuditTestLogger() (*slog.Logger, *auditLogBuffer) {
	logs := &auditLogBuffer{}
	return slog.New(slog.NewTextHandler(logs, nil)), logs
}

// waitFor polls cond until it holds or the deadline passes.
func waitForAudit(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestAuditErrThrottleLogsOncePerInterval(t *testing.T) {
	logger, logs := newAuditTestLogger()
	var throttle auditErrThrottle

	for range 3 {
		throttle.logError(logger, "write failed", "file", errors.New("disk full"))
	}
	if n := strings.Count(logs.String(), "write failed"); n != 1 {
		t.Fatalf("logged %d times within the interval, want 1:\n%s", n, logs)
	}

	throttle.lastLogged = time.Now().Add(-auditErrorLogInterval)
	throttle.logError(logger, "write failed", "file", errors.New("disk full"))
	if n := strings.Count(logs.String(), "write failed"); n != 2 {
		t.Fatalf("logged %d times after the interval, want 2:\n%s", n, logs)
	}
	if !strings.Contains(logs.String(), "suppressedSinceLastLog=2") {
		t.Fatalf("the second log does not count the 2 suppressed errors:\n%s", logs)
	}
}

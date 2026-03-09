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

package eventlistener

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
)

func TestProcessEvents_RecoversFromPanicAndContinues(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := make(chan eventhub.Event, 2)
	listener := &EventListener{
		logger:  logger,
		eventCh: eventCh,
		ctx:     ctx,
		cancel:  cancel,
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		listener.processEvents()
	}()

	// This panics because listener.store is nil when handling DELETE.
	eventCh <- eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "DELETE",
		EntityID:  "panic-api-id",
		EventID:   "corr-panic",
	}

	// If recovery works, the loop should continue and process this event too.
	eventCh <- eventhub.Event{
		EventType: eventhub.EventType("UNKNOWN"),
		Action:    "UPDATE",
		EntityID:  "safe-event-id",
		EventID:   "corr-safe",
	}

	close(eventCh)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for processEvents to exit")
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "Recovered from panic while processing event") {
		t.Fatalf("expected panic recovery log, got: %s", logs)
	}
	if !strings.Contains(logs, "Unknown event type received") {
		t.Fatalf("expected processing to continue after panic, got: %s", logs)
	}
}

func TestStart_RequiresSystemConfig(t *testing.T) {
	listener := &EventListener{}

	err := listener.Start()

	if err == nil {
		t.Fatal("expected start to fail without system config")
	}
	if !strings.Contains(err.Error(), "system configuration") {
		t.Fatalf("expected system configuration error, got: %v", err)
	}
}

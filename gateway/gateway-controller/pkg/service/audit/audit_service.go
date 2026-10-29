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

// Package audit provides fire-and-forget audit event recording for the
// gateway-controller management API. It mirrors the platform-api audit service
// design: a bounded semaphore caps in-flight goroutines, each sink is retried
// up to maxAttempts times before the event is dropped, and a cancelled HTTP
// request context never aborts a write (writes always use context.Background).
package audit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/middleware"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

const (
	maxInFlight = 100
	maxAttempts = 3
)

// AuditEvent is the write-path DTO. GatewayID, UUID, PerformedAt, and
// CorrelationID are populated by AuditService.Record — callers supply the
// remaining fields.
// JSON tags match the read API and platform-api's file sink, so FileSink lines use
// the same field names.
type AuditEvent struct {
	GatewayID     string    `json:"gatewayId"` // auto-filled from the service's own gatewayID
	UUID          string    `json:"id"`        // auto-filled (UUIDv7)
	PerformedAt   time.Time `json:"performedAt"`
	CorrelationID string    `json:"correlationId,omitempty"` // set by callers from params.CorrelationID, or auto-filled from ctx if empty

	Action       string `json:"action"`
	ResourceType string `json:"resourceType"`
	Outcome      string `json:"outcome"`
	PerformedBy  string `json:"performedBy,omitempty"`
	ResourceUUID string `json:"resourceId,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
	EventData    any    `json:"eventData,omitempty"` // marshalled to JSON; nil → NULL
}

// AuditSink is implemented by any backend that can persist an AuditEvent.
type AuditSink interface {
	Write(ctx context.Context, e *AuditEvent) error
}

// AuditService records audit events asynchronously.
type AuditService struct {
	enabled   bool
	gatewayID string
	sinks     []AuditSink
	sem       chan struct{}
	logger    *slog.Logger
}

// defaultLogDir holds audit.log when the file sink's path is not set.
const defaultLogDir = "./data/logs"

// NewAuditService builds the service and the sinks cfg asks for: the database
// always, plus the file and stdout sinks when enabled. A file sink that cannot
// be opened is skipped with a warning. When audit is disabled all Record calls
// are no-ops and no sinks are built, so no log file is created. gatewayID is
// injected into every event so callers do not need to repeat it.
func NewAuditService(cfg config.AuditConfig, gatewayID string, store storage.Storage, logger *slog.Logger) *AuditService {
	s := &AuditService{
		enabled:   cfg.Enabled,
		gatewayID: gatewayID,
		sem:       make(chan struct{}, maxInFlight),
		logger:    logger,
	}
	if !cfg.Enabled {
		return s
	}
	s.sinks = []AuditSink{NewDBSink(store)}
	if cfg.Sinks.File.Enabled {
		dir := cfg.Sinks.File.Path
		if dir == "" {
			dir = defaultLogDir
		}
		if fs, err := NewFileSink(dir); err == nil {
			s.sinks = append(s.sinks, fs)
		} else {
			logger.Warn("audit: file sink not started", slog.String("path", dir), slog.Any("error", err))
		}
	}
	if cfg.Sinks.Stdout.Enabled {
		s.sinks = append(s.sinks, NewStdoutSink())
	}
	return s
}

type withoutAuditKey struct{}

// WithoutAudit marks ctx as a machine-to-machine call (control-plane sync). Only
// operations a user performs through the management API are audited, so Record
// ignores events recorded under the returned context.
func WithoutAudit(ctx context.Context) context.Context {
	return context.WithValue(ctx, withoutAuditKey{}, true)
}

// AuditDiff compares two field snapshots built by the caller and returns the fields
// whose values differ, as {"from": old, "to": new}. Returns nil when nothing changed
// so EventData is omitted. Both snapshots must have the same keys.
func AuditDiff(before, after map[string]any) map[string]any {
	diff := map[string]any{}
	for k, to := range after {
		if from := before[k]; fmt.Sprint(from) != fmt.Sprint(to) {
			diff[k] = map[string]any{"from": from, "to": to}
		}
	}
	if len(diff) == 0 {
		return nil
	}
	return diff
}

// Record enqueues e for asynchronous writing to all configured sinks.
// It is fire-and-forget: it returns immediately and never returns an error.
// If the in-flight semaphore is full the event is dropped with a warning log.
func (s *AuditService) Record(ctx context.Context, e *AuditEvent) {
	if s == nil || !s.enabled || ctx.Value(withoutAuditKey{}) != nil {
		return
	}
	if e.Action == "" || e.ResourceType == "" || e.Outcome == "" {
		s.logger.Warn("audit: dropping event with missing required fields",
			slog.String("action", e.Action),
			slog.String("resource_type", e.ResourceType),
			slog.String("outcome", e.Outcome))
		return
	}

	// Fill fields the caller must not supply.
	e.GatewayID = s.gatewayID
	if e.UUID == "" {
		u, err := uuid.NewV7()
		if err != nil {
			s.logger.Warn("audit: failed to generate UUID, dropping event", slog.Any("error", err))
			return
		}
		e.UUID = u.String()
	}
	e.PerformedAt = time.Now().UTC()
	if e.CorrelationID == "" {
		e.CorrelationID = middleware.GetCorrelationIDFromContext(ctx)
	}

	select {
	case s.sem <- struct{}{}:
	default:
		s.logger.Warn("audit: semaphore full, dropping event",
			slog.String("resource_type", e.ResourceType),
			slog.String("action", e.Action))
		return
	}

	go func() {
		defer func() { <-s.sem }()
		writeCtx := context.Background()
		for _, sink := range s.sinks {
			s.writeWithRetry(writeCtx, sink, e)
		}
	}()
}

func (s *AuditService) writeWithRetry(ctx context.Context, sink AuditSink, e *AuditEvent) {
	backoff := 100 * time.Millisecond
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := sink.Write(ctx, e); err != nil {
			s.logger.Warn("audit: sink write failed",
				slog.Int("attempt", attempt),
				slog.Int("max_attempts", maxAttempts),
				slog.Any("error", err))
			if attempt < maxAttempts {
				time.Sleep(backoff)
				if next := backoff * 4; next < 400*time.Millisecond {
					backoff = next
				} else {
					backoff = 400 * time.Millisecond
				}
			}
			continue
		}
		return
	}
	s.logger.Error("audit: all write attempts failed, event dropped",
		slog.String("uuid", e.UUID),
		slog.String("resource_type", e.ResourceType),
		slog.String("action", e.Action))
}

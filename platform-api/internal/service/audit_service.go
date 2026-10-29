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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	httpmiddleware "github.com/wso2/api-platform/httpkit/middleware"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

type AuditEvent struct {
	OrganizationUUID string    `json:"organizationId"`
	Action           string    `json:"action"`
	ResourceType     string    `json:"resourceType"`
	Outcome          string    `json:"outcome"`
	UUID             string    `json:"id"`
	PerformedAt      time.Time `json:"performedAt"`

	PerformedBy   string `json:"performedBy,omitempty"`
	ResourceUUID  string `json:"resourceId,omitempty"`
	ResourceName  string `json:"resourceName,omitempty"`
	CorrelationID string `json:"correlationId,omitempty"`
	EventData     any    `json:"eventData,omitempty"`
}

// AuditSink is a write target. Implementations must be goroutine-safe.
// Sinks that need Close/flush implement io.Closer separately.
type AuditSink interface {
	Write(ctx context.Context, e *AuditEvent) error
}

// AuditService fans audit events to registered sinks. Record is non-blocking
// and never returns an error — events drop (logged as warn) when the
// semaphore is saturated, so the caller is never stalled by audit writes.
type AuditService struct {
	enabled bool
	sinks   []AuditSink
	sem     chan struct{}
	slogger *slog.Logger
}

const (
	maxInFlight = 100
	maxAttempts = 3
)

// defaultAuditLogDir holds audit.log when the file sink's path is not set.
const defaultAuditLogDir = "./data/logs"

// NewAuditService builds the service and the sinks cfg asks for: the database
// always, plus the file and stdout sinks when enabled. A file sink that cannot
// be opened is skipped with a warning. When audit is disabled no sinks are
// built, so no log file is created.
func NewAuditService(cfg config.Audit, repo repository.AuditLogRepository, slogger *slog.Logger) *AuditService {
	s := &AuditService{
		enabled: cfg.Enabled,
		sem:     make(chan struct{}, maxInFlight),
		slogger: slogger,
	}
	if !cfg.Enabled {
		return s
	}
	s.sinks = []AuditSink{NewDBSink(repo)}
	if cfg.Sinks.File.Enabled {
		dir := cfg.Sinks.File.Path
		if dir == "" {
			dir = defaultAuditLogDir
		}
		if fs, err := NewFileSink(dir); err == nil {
			s.sinks = append(s.sinks, fs)
		} else {
			slogger.Warn("audit_log: file sink not started", "path", dir, "error", err)
		}
	}
	if cfg.Sinks.Stdout.Enabled {
		s.sinks = append(s.sinks, NewStdoutSink())
	}
	return s
}

type withoutAuditKey struct{}

// WithoutAudit marks ctx as a machine-to-machine call (webhooks, gateway sync). Only
// operations a user performs through the public API are audited, so Record ignores
// events recorded under the returned context.
func WithoutAudit(ctx context.Context) context.Context {
	return context.WithValue(ctx, withoutAuditKey{}, true)
}

// Record enriches and fans out the event. Caller must set OrganizationUUID.
// A nil receiver is a no-op, so handlers that receive a nil AuditService (e.g. tests) are safe.
func (s *AuditService) Record(ctx context.Context, e *AuditEvent) {
	if s == nil || !s.enabled || ctx.Value(withoutAuditKey{}) != nil {
		return
	}

	if e.OrganizationUUID == "" {
		s.slogger.Warn("audit_log: dropping event — missing OrganizationUUID",
			"action", e.Action, "resource_type", e.ResourceType)
		return
	}

	// An AuditDiff with no changes is a nil map: store it as no event data.
	if m, ok := e.EventData.(map[string]any); ok && m == nil {
		e.EventData = nil
	}

	if e.UUID == "" {
		id, err := utils.GenerateUUID()
		if err != nil {
			s.slogger.Warn("audit_log: dropping event — uuid generation failed",
				"action", e.Action, "resource_type", e.ResourceType, "error", err)
			return
		}
		e.UUID = id
	}
	if e.PerformedAt.IsZero() {
		e.PerformedAt = time.Now().UTC()
	}
	e.CorrelationID = httpmiddleware.GetCorrelationIDFromContext(ctx)

	select {
	case s.sem <- struct{}{}:
		go func(e *AuditEvent) {
			defer func() { <-s.sem }()
			// context.Background: a canceled request ctx must not abort the write.
			s.fanOut(context.Background(), e)
		}(e)
	default:
		s.slogger.Warn("audit_log: queue saturated, dropping event",
			"uuid", e.UUID, "action", e.Action, "in_flight", len(s.sem))
	}
}

func (s *AuditService) fanOut(ctx context.Context, e *AuditEvent) {
	for _, sink := range s.sinks {
		s.writeWithRetry(ctx, sink, e)
	}
}

func (s *AuditService) writeWithRetry(ctx context.Context, sink AuditSink, e *AuditEvent) {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := sink.Write(ctx, e); err == nil {
			if attempt > 1 {
				s.slogger.Info("audit_log: sink write succeeded after retry",
					"sink", fmt.Sprintf("%T", sink), "uuid", e.UUID, "attempt", attempt)
			}
			return
		} else if attempt == maxAttempts {
			s.slogger.Error("audit_log: sink write failed, giving up",
				"sink", fmt.Sprintf("%T", sink), "uuid", e.UUID, "attempt", attempt, "error", err)
			return
		}
		time.Sleep(backoff(attempt))
	}
}

func backoff(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 100 * time.Millisecond
	default:
		return 400 * time.Millisecond
	}
}

// IsAuditableFailure returns true for errors that represent genuine operational
// failures (DB errors, unexpected internal errors) — the kind worth recording in
// the audit log. Returns false for user-input errors (4xx apperrors such as
// ValidationFailed, uniqueness violations) which are not audit-worthy.
func IsAuditableFailure(err error) bool {
	if err == nil {
		return false
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.HTTPStatus >= http.StatusInternalServerError
	}
	// Non-apperror errors (raw DB errors, etc.) are always auditable.
	return true
}

// AuditDiff compares two field snapshots built by the caller and returns the fields
// whose values differ, as {"from": old, "to": new}. Returns nil when nothing changed
// so EventData is omitted.
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

// Close waits for in-flight writes to finish, until ctx is done, and then closes
// the file sink. Events recorded after Close are dropped.
func (s *AuditService) Close(ctx context.Context) {
	if s == nil {
		return
	}
	for i := 0; i < cap(s.sem); i++ {
		select {
		case s.sem <- struct{}{}:
		case <-ctx.Done():
			s.slogger.Warn("audit_log: shutdown before in-flight writes finished", "in_flight", cap(s.sem)-i)
			return
		}
	}
	for _, sink := range s.sinks {
		if c, ok := sink.(io.Closer); ok {
			_ = c.Close()
		}
	}
}

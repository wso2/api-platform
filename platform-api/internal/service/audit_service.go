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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/platform-api/config"
)

type auditTable interface {
	Record(id string, performedAt time.Time, action, resourceUUID, resourceType, orgUUID, performedBy string) error
}

type AuditService struct {
	enabled bool
	repo    auditTable
	sinks   []auditSink
}

// NewAuditService builds the configured outputs, only when auditing is enabled.
// An output that cannot be built is an error, and the caller refuses to start.
func NewAuditService(repo auditTable, cfg config.Audit, slogger *slog.Logger) (*AuditService, error) {
	s := &AuditService{enabled: cfg.Enabled, repo: repo}
	if !cfg.Enabled {
		return s, nil
	}
	sinks, err := newAuditSinks(cfg, slogger)
	if err != nil {
		return nil, err
	}
	s.sinks = sinks
	return s, nil
}

func (s *AuditService) Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	if !s.enabled {
		return nil
	}
	id := uuid.New().String()
	// Microseconds: the finest precision every supported database stores, so
	// the row and the output line hold the same time.
	performedAt := time.Now().UTC().Truncate(time.Microsecond)

	err := s.repo.Record(id, performedAt, action, resourceUUID, resourceType, orgUUID, performedBy)
	if len(s.sinks) > 0 {
		rec := slog.NewRecord(performedAt, slog.LevelInfo, "audit", 0)
		rec.AddAttrs(
			slog.String("logType", "audit"),
			slog.String("id", id),
			slog.String("action", action),
			slog.String("resourceType", resourceType),
			slog.String("resourceId", resourceUUID),
			slog.String("organizationId", orgUUID),
			slog.String("performedBy", performedBy),
		)
		var buf bytes.Buffer
		_ = slog.NewJSONHandler(&buf, nil).Handle(context.Background(), rec)
		line := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
		for _, sink := range s.sinks {
			sink.Write(line)
		}
	}
	return err
}

// Close closes every output, flushing any that buffer. Errors are joined so one
// stuck output does not stop the others from closing.
func (s *AuditService) Close(ctx context.Context) error {
	var errs []error
	for _, sink := range s.sinks {
		if err := sink.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("closing %s audit output: %w", sink.Name(), err))
		}
	}
	return errors.Join(errs...)
}

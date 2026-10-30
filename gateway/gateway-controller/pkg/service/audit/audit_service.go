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
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

const auditLogFile = "./data/logs/audit.log"

type auditTable interface {
	SaveAudit(id string, performedAt time.Time, action, resourceUUID, resourceType, performedBy, correlationID string) error
}

type AuditService struct {
	enabled   bool
	store     auditTable
	gatewayID string
	out       slog.Handler
	logger    *slog.Logger
}

func NewAuditService(store auditTable, gatewayID string, cfg config.AuditConfig, logger *slog.Logger) *AuditService {
	s := &AuditService{enabled: cfg.Enabled, store: store, gatewayID: gatewayID, logger: logger}
	if !cfg.Enabled {
		return s
	}
	switch cfg.Output {
	case "stdout":
		s.out = slog.NewJSONHandler(os.Stdout, nil)
	case "file":
		f, err := openAuditLogFile()
		if err != nil {
			logger.Warn("Audit log file not opened; audit records go to the audit table only",
				slog.String("path", auditLogFile), slog.Any("error", err))
			break
		}
		s.out = slog.NewJSONHandler(f, nil)
	}
	return s
}

func openAuditLogFile() (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(auditLogFile), 0o750); err != nil {
		return nil, err
	}
	return os.OpenFile(auditLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
}

func (s *AuditService) Record(action, resourceUUID, resourceType, performedBy, correlationID string) {
	if s == nil || !s.enabled {
		return
	}
	id := uuid.New().String()
	// Microseconds: the finest precision every supported database stores, so
	// the row and the stdout line hold the same time.
	performedAt := time.Now().UTC().Truncate(time.Microsecond)

	if err := s.store.SaveAudit(id, performedAt, action, resourceUUID, resourceType, performedBy, correlationID); err != nil {
		s.logger.Warn("Failed to save audit record",
			slog.String("action", action), slog.String("resource_type", resourceType),
			slog.String("resource_uuid", resourceUUID), slog.Any("error", err))
	}
	if s.out != nil {
		line := slog.NewRecord(performedAt, slog.LevelInfo, "audit", 0)
		line.AddAttrs(
			slog.String("logType", "audit"),
			slog.String("id", id),
			slog.String("gatewayId", s.gatewayID),
			slog.String("correlationId", correlationID),
			slog.String("action", action),
			slog.String("resourceType", resourceType),
			slog.String("resourceId", resourceUUID),
			slog.String("performedBy", performedBy),
		)
		_ = s.out.Handle(context.Background(), line)
	}
}

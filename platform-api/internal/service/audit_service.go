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
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/platform-api/config"
)

const auditLogFile = "./data/logs/audit.log"

type auditTable interface {
	Record(id string, performedAt time.Time, action, resourceUUID, resourceType, orgUUID, performedBy string) error
}

type AuditService struct {
	repo auditTable
	out  slog.Handler
}

func NewAuditService(repo auditTable, cfg config.Audit, slogger *slog.Logger) *AuditService {
	s := &AuditService{repo: repo}
	switch cfg.Output {
	case "stdout":
		s.out = slog.NewJSONHandler(os.Stdout, nil)
	case "file":
		f, err := openAuditLogFile()
		if err != nil {
			slogger.Warn("Audit log file not opened; audit records go to the audit table only",
				"path", auditLogFile, "error", err)
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

func (s *AuditService) Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	id := uuid.New().String()
	// Microseconds: the finest precision every supported database stores, so
	// the row and the output line hold the same time.
	performedAt := time.Now().UTC().Truncate(time.Microsecond)

	err := s.repo.Record(id, performedAt, action, resourceUUID, resourceType, orgUUID, performedBy)
	if s.out != nil {
		line := slog.NewRecord(performedAt, slog.LevelInfo, "audit", 0)
		line.AddAttrs(
			slog.String("logType", "audit"),
			slog.String("id", id),
			slog.String("action", action),
			slog.String("resourceType", resourceType),
			slog.String("resourceId", resourceUUID),
			slog.String("organizationId", orgUUID),
			slog.String("performedBy", performedBy),
		)
		_ = s.out.Handle(context.Background(), line)
	}
	return err
}

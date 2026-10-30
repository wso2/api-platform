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
	"time"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/platform-api/config"
)

type auditTable interface {
	Record(id string, performedAt time.Time, action, resourceUUID, resourceType, orgUUID, performedBy string) error
}

type AuditService struct {
	repo   auditTable
	stdout slog.Handler
}

func NewAuditService(repo auditTable, cfg config.Audit) *AuditService {
	s := &AuditService{repo: repo}
	if cfg.Stdout {
		s.stdout = slog.NewJSONHandler(os.Stdout, nil)
	}
	return s
}

func (s *AuditService) Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	id := uuid.New().String()
	// Microseconds: the finest precision every supported database stores, so
	// the row and the stdout line hold the same time.
	performedAt := time.Now().UTC().Truncate(time.Microsecond)

	err := s.repo.Record(id, performedAt, action, resourceUUID, resourceType, orgUUID, performedBy)
	if s.stdout != nil {
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
		_ = s.stdout.Handle(context.Background(), line)
	}
	return err
}

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
	"time"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/audit"
)

type auditTable interface {
	Record(id string, performedAt time.Time, action, resourceUUID, resourceType, orgUUID, performedBy string) error
}

// AuditService records each audited change in the audit table and writes the
// same record to the outputs configured in [platform_api.audit].
type AuditService struct {
	enabled bool
	repo    auditTable
	out     *audit.Writer // nil when auditing is disabled
}

// NewAuditService builds the configured outputs when auditing is enabled. An
// output that cannot be built is an error, and the caller refuses to start.
func NewAuditService(repo auditTable, cfg config.Audit, slogger *slog.Logger) (*AuditService, error) {
	s := &AuditService{enabled: cfg.Enabled, repo: repo}
	if !cfg.Enabled {
		return s, nil
	}
	out, err := audit.NewWriter(cfg, slogger)
	if err != nil {
		return nil, err
	}
	s.out = out
	return s, nil
}

// Record writes the table row, then the output line with the same id and time.
// Only the table error is returned: an output failure never fails the request.
func (s *AuditService) Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	if !s.enabled {
		return nil
	}
	id := uuid.New().String()
	// Microseconds: the finest precision every supported database stores, so
	// the row and the output line hold the same time.
	performedAt := time.Now().UTC().Truncate(time.Microsecond)

	err := s.repo.Record(id, performedAt, action, resourceUUID, resourceType, orgUUID, performedBy)
	s.out.Write(audit.Record{
		ID:             id,
		Time:           performedAt,
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     resourceUUID,
		OrganizationID: orgUUID,
		PerformedBy:    performedBy,
	})
	return err
}

// Close flushes and closes the outputs.
func (s *AuditService) Close(ctx context.Context) error {
	if s.out == nil {
		return nil
	}
	return s.out.Close(ctx)
}

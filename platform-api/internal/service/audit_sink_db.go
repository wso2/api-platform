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
	"encoding/json"
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// DBSink persists audit events to the audit_log table via AuditLogRepository.
type DBSink struct {
	repo repository.AuditLogRepository
}

func NewDBSink(repo repository.AuditLogRepository) *DBSink {
	return &DBSink{repo: repo}
}

func (s *DBSink) Write(_ context.Context, e *AuditEvent) error {
	if err := s.repo.Create(toAuditLog(e)); err != nil {
		return fmt.Errorf("DBSink: %w", err)
	}
	return nil
}

func toAuditLog(e *AuditEvent) *model.AuditLog {
	return &model.AuditLog{
		UUID:             e.UUID,
		OrganizationUUID: e.OrganizationUUID,
		CorrelationID:    e.CorrelationID,
		Action:           e.Action,
		ResourceType:     e.ResourceType,
		ResourceUUID:     e.ResourceUUID,
		ResourceName:     e.ResourceName,
		PerformedBy:      e.PerformedBy,
		PerformedAt:      e.PerformedAt,
		Outcome:          e.Outcome,
		EventData:        marshalEventData(e.EventData),
	}
}

// marshalEventData returns "" on nil (including a nil map, e.g. an empty AuditDiff) or
// marshal failure — audit truth outweighs a lost payload.
func marshalEventData(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return ""
	}
	return string(b)
}

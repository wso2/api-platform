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

package repository

import (
	"time"

	"github.com/wso2/api-platform/platform-api/internal/database"

	"github.com/google/uuid"
)

// AuditRepo writes records to the audit table.
type AuditRepo struct {
	db *database.DB
}

// NewAuditRepo creates a new AuditRepo.
func NewAuditRepo(db *database.DB) AuditRepository {
	return &AuditRepo{db: db}
}

// Record inserts a single audit row. Failures are non-fatal — callers should ignore the returned error.
func (r *AuditRepo) Record(action, resourceUUID, resourceType, orgUUID, performedBy string) error {
	id := uuid.New().String()
	query := `INSERT INTO audit (uuid, action, resource_uuid, resource_type, organization_uuid, performed_by, performed_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(r.db.Rebind(query), id, action, resourceUUID, resourceType, orgUUID, performedBy, time.Now().UTC())
	return err
}

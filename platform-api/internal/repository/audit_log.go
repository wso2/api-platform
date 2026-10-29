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
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

type AuditLogRepo struct {
	db *database.DB
}

func NewAuditLogRepo(db *database.DB) AuditLogRepository {
	return &AuditLogRepo{db: db}
}

// AuditLogFilter narrows a List or Count. Zero-valued fields (other than
// OrganizationUUID, which is always applied) mean "no filter on this
// dimension". StartTime and EndTime are inclusive bounds. ResourceTypes
// matches a row of any listed type.
type AuditLogFilter struct {
	OrganizationUUID string
	StartTime        *time.Time
	EndTime          *time.Time
	Outcome          string
	PerformedBy      string
	Action           string
	ResourceTypes    []string
	ResourceUUID     string
}

const auditLogCols = `SELECT uuid, organization_uuid, correlation_id, action, resource_type, resource_uuid, resource_name, performed_by, performed_at, outcome, event_data FROM audit_log`

// nullString converts a Go string to a sql.NullString, treating empty as NULL.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// scanAuditLog reads one audit_log row from any scanning interface.
func scanAuditLog(row interface{ Scan(...interface{}) error }) (*model.AuditLog, error) {
	var (
		a             model.AuditLog
		correlationID sql.NullString
		resourceUUID  sql.NullString
		resourceName  sql.NullString
		performedBy   sql.NullString
		eventData     sql.NullString
	)
	err := row.Scan(
		&a.UUID,
		&a.OrganizationUUID,
		&correlationID,
		&a.Action,
		&a.ResourceType,
		&resourceUUID,
		&resourceName,
		&performedBy,
		&a.PerformedAt,
		&a.Outcome,
		&eventData,
	)
	if err != nil {
		return nil, err
	}
	a.CorrelationID = correlationID.String
	a.ResourceUUID = resourceUUID.String
	a.ResourceName = resourceName.String
	a.PerformedBy = performedBy.String
	a.EventData = eventData.String
	return &a, nil
}

// Create inserts a single audit_log row. UUID is generated (UUIDv7) and
// PerformedAt defaults to now if either is zero-valued. Empty string fields
// for nullable columns are written as SQL NULL.
func (r *AuditLogRepo) Create(row *model.AuditLog) error {
	if row.UUID == "" {
		id, err := utils.GenerateUUID()
		if err != nil {
			return fmt.Errorf("failed to generate audit log uuid: %w", err)
		}
		row.UUID = id
	}
	if row.PerformedAt.IsZero() {
		row.PerformedAt = time.Now().UTC()
	}

	query := r.db.Rebind(`
		INSERT INTO audit_log (
			uuid, organization_uuid, correlation_id, action, resource_type,
			resource_uuid, resource_name, performed_by, performed_at, outcome, event_data
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	_, err := r.db.Exec(query,
		row.UUID,
		row.OrganizationUUID,
		nullString(row.CorrelationID),
		row.Action,
		row.ResourceType,
		nullString(row.ResourceUUID),
		nullString(row.ResourceName),
		nullString(row.PerformedBy),
		row.PerformedAt,
		row.Outcome,
		nullString(row.EventData),
	)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}
	return nil
}

// buildAuditLogWhere turns a filter into a WHERE clause and bound args.
// Shared by List and Count so the two can't drift.
func buildAuditLogWhere(f AuditLogFilter) (where string, args []interface{}) {
	conditions := []string{"organization_uuid = ?"}
	args = append(args, f.OrganizationUUID)

	if f.StartTime != nil {
		conditions = append(conditions, "performed_at >= ?")
		args = append(args, *f.StartTime)
	}
	if f.EndTime != nil {
		conditions = append(conditions, "performed_at <= ?")
		args = append(args, *f.EndTime)
	}
	if f.Outcome != "" {
		conditions = append(conditions, "outcome = ?")
		args = append(args, f.Outcome)
	}
	if f.PerformedBy != "" {
		conditions = append(conditions, "performed_by = ?")
		args = append(args, f.PerformedBy)
	}
	if f.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, f.Action)
	}
	if len(f.ResourceTypes) > 0 {
		placeholders := make([]string, len(f.ResourceTypes))
		for i, t := range f.ResourceTypes {
			placeholders[i] = "?"
			args = append(args, t)
		}
		conditions = append(conditions, "resource_type IN ("+strings.Join(placeholders, ", ")+")")
	}
	if f.ResourceUUID != "" {
		conditions = append(conditions, "resource_uuid = ?")
		args = append(args, f.ResourceUUID)
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

// List returns audit_log rows matching f, ordered by performed_at DESC.
func (r *AuditLogRepo) List(f AuditLogFilter, limit, offset int) ([]*model.AuditLog, error) {
	where, args := buildAuditLogWhere(f)
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	query := r.db.Rebind(auditLogCols + where + " ORDER BY performed_at DESC " + pageClause)
	args = append(args, pageArgs...)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()

	var out []*model.AuditLog
	for rows.Next() {
		a, err := scanAuditLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Count returns the number of audit_log rows matching f, independent of pagination.
func (r *AuditLogRepo) Count(f AuditLogFilter) (int, error) {
	where, args := buildAuditLogWhere(f)
	query := r.db.Rebind("SELECT COUNT(*) FROM audit_log" + where)
	var n int
	if err := r.db.QueryRow(query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

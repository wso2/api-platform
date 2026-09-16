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
	"errors"
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// ApiPortalRepo implements ApiPortalRepository against the api_portals stub
// table (owned by another team — see schema.*.sql).
type ApiPortalRepo struct {
	db *database.DB
}

// NewApiPortalRepo creates a new API Portal repository.
func NewApiPortalRepo(db *database.DB) ApiPortalRepository {
	return &ApiPortalRepo{db: db}
}

// GetByHandleAndOrg retrieves an API Portal by handle and organization,
// regardless of workflow_status — only the GET /api-publications rollup
// (Slice 3) filters to "active"; draft/publication operations here just need
// the portal to exist. Returns (nil, nil) when no such portal exists.
func (r *ApiPortalRepo) GetByHandleAndOrg(handle, orgUUID string) (*model.APIPortal, error) {
	query := `
		SELECT uuid, organization_uuid, handle, display_name, description, url, workflow_status
		FROM api_portals
		WHERE handle = ? AND organization_uuid = ?
	`
	portal := &model.APIPortal{}
	var description, url sql.NullString
	err := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(
		&portal.UUID, &portal.OrganizationUUID, &portal.Handle, &portal.DisplayName,
		&description, &url, &portal.WorkflowStatus,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get API Portal by handle: %w", err)
	}
	portal.Description = description.String
	portal.URL = url.String
	return portal, nil
}

// ListActiveByOrg returns every api_portals row for orgUUID whose
// workflow_status is "active", ordered by registration time for a
// deterministic default order.
func (r *ApiPortalRepo) ListActiveByOrg(orgUUID string) ([]*model.APIPortal, error) {
	query := `
		SELECT uuid, organization_uuid, handle, display_name, description, url, workflow_status, created_at
		FROM api_portals
		WHERE organization_uuid = ? AND workflow_status = ?
		ORDER BY created_at, uuid
	`
	rows, err := r.db.Query(r.db.Rebind(query), orgUUID, model.APIPortalWorkflowStatusActive)
	if err != nil {
		return nil, fmt.Errorf("failed to list active API Portals: %w", err)
	}
	defer rows.Close()

	var portals []*model.APIPortal
	for rows.Next() {
		portal := &model.APIPortal{}
		var description, url sql.NullString
		if err := rows.Scan(
			&portal.UUID, &portal.OrganizationUUID, &portal.Handle, &portal.DisplayName,
			&description, &url, &portal.WorkflowStatus, &portal.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan API Portal row: %w", err)
		}
		portal.Description = description.String
		portal.URL = url.String
		portals = append(portals, portal)
	}
	return portals, rows.Err()
}

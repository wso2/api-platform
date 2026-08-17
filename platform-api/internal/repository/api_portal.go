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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// APIPortalRepo implements APIPortalRepository.
type APIPortalRepo struct {
	db *database.DB
}

// NewAPIPortalRepo creates a new API Portal repository.
func NewAPIPortalRepo(db *database.DB) APIPortalRepository {
	return &APIPortalRepo{db: db}
}

// apiPortalSelectColumns are the api_portals columns selected in every query, in scan order.
const apiPortalSelectColumns = `
	uuid, organization_uuid, handle, display_name, description, url,
	workflow_status, auth_type, auth_configuration, metadata,
	created_by, updated_by, created_at, updated_at
`

// scanAPIPortalRow scans one api_portals row using the column order in apiPortalSelectColumns.
func scanAPIPortalRow(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.APIPortal, error) {
	portal := &model.APIPortal{}
	var description, url, createdBy, updatedBy sql.NullString
	var authConfigBytes, metadataBytes []byte
	if err := scanner.Scan(
		&portal.ID, &portal.OrganizationID, &portal.Handle, &portal.Name, &description, &url,
		&portal.WorkflowStatus, &portal.AuthType, &authConfigBytes, &metadataBytes,
		&createdBy, &updatedBy, &portal.CreatedAt, &portal.UpdatedAt,
	); err != nil {
		return nil, err
	}
	portal.Description = description.String
	portal.URL = url.String
	portal.CreatedBy = createdBy.String
	portal.UpdatedBy = updatedBy.String
	authConfig, err := unmarshalAPIPortalBlob(authConfigBytes, "auth_configuration")
	if err != nil {
		return nil, err
	}
	portal.AuthConfig = authConfig
	metadata, err := unmarshalAPIPortalBlob(metadataBytes, "metadata")
	if err != nil {
		return nil, err
	}
	portal.Metadata = metadata
	return portal, nil
}

// marshalAPIPortalBlob serializes a JSON blob column value. A nil map becomes an
// empty JSON object so the NOT NULL BYTEA/BLOB/VARBINARY column always has
// valid content; readers (unmarshalAPIPortalBlob) mirror this by normalizing
// empty/{} back to an empty map so callers never nil-check.
func marshalAPIPortalBlob(m map[string]interface{}, field string) ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s: %w", field, err)
	}
	return b, nil
}

// unmarshalAPIPortalBlob deserializes a JSON blob and normalizes the result to
// a non-nil map.
func unmarshalAPIPortalBlob(b []byte, field string) (map[string]interface{}, error) {
	m := map[string]interface{}{}
	if len(b) > 0 {
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s: %w", field, err)
		}
	}
	if m == nil {
		m = map[string]interface{}{}
	}
	return m, nil
}

// Create inserts a new API Portal row.
func (r *APIPortalRepo) Create(portal *model.APIPortal) error {
	now := time.Now().UTC()
	portal.CreatedAt = now
	portal.UpdatedAt = now
	authConfigBytes, err := marshalAPIPortalBlob(portal.AuthConfig, "auth_configuration")
	if err != nil {
		return err
	}
	metadataBytes, err := marshalAPIPortalBlob(portal.Metadata, "metadata")
	if err != nil {
		return err
	}
	query := `
		INSERT INTO api_portals (uuid, organization_uuid, handle, display_name, description, url,
		                          workflow_status, auth_type, auth_configuration, metadata,
		                          created_by, updated_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = r.db.Exec(r.db.Rebind(query),
		portal.ID, portal.OrganizationID, portal.Handle, portal.Name, portal.Description, portal.URL,
		portal.WorkflowStatus, portal.AuthType, authConfigBytes, metadataBytes,
		portal.CreatedBy, portal.UpdatedBy, portal.CreatedAt, portal.UpdatedAt,
	)
	return err
}

// GetByUUID retrieves an API Portal by its internal UUID, scoped to the organization.
func (r *APIPortalRepo) GetByUUID(portalID, orgUUID string) (*model.APIPortal, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM api_portals
		WHERE uuid = ? AND organization_uuid = ?
	`, apiPortalSelectColumns)
	row := r.db.QueryRow(r.db.Rebind(query), portalID, orgUUID)
	portal, err := scanAPIPortalRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return portal, nil
}

// GetByHandleAndOrgID retrieves an API Portal by its handle within an organization.
func (r *APIPortalRepo) GetByHandleAndOrgID(handle, orgUUID string) (*model.APIPortal, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM api_portals
		WHERE handle = ? AND organization_uuid = ?
	`, apiPortalSelectColumns)
	row := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID)
	portal, err := scanAPIPortalRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return portal, nil
}

// ListPaginated returns a page of API Portals scoped to the organization,
// optionally filtered by workflow_status.
func (r *APIPortalRepo) ListPaginated(orgUUID string, workflowStatus *string, opts ListOptions) ([]*model.APIPortal, error) {
	var args []interface{}
	conditions := []string{`organization_uuid = ?`}
	args = append(args, orgUUID)
	if workflowStatus != nil {
		conditions = append(conditions, `workflow_status = ?`)
		args = append(args, *workflowStatus)
	}
	if searchClause, searchArgs := handleSearchClause(opts.Search); searchClause != "" {
		conditions = append(conditions, strings.TrimPrefix(searchClause, " AND "))
		args = append(args, searchArgs...)
	}
	col, dir := opts.resolveSort(listSortColumns, "created_at")
	pageClause, pageArgs := r.db.PaginationClause(opts.Limit, opts.Offset)
	args = append(args, pageArgs...)

	query := fmt.Sprintf(`
		SELECT %s FROM api_portals
		WHERE %s
		ORDER BY %s %s, handle ASC
		%s
	`, apiPortalSelectColumns, strings.Join(conditions, ` AND `), col, dir, pageClause)

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var portals []*model.APIPortal
	for rows.Next() {
		portal, err := scanAPIPortalRow(rows)
		if err != nil {
			return nil, err
		}
		portals = append(portals, portal)
	}
	return portals, rows.Err()
}

// Count returns the total number of API Portals matching the filter, independent of pagination.
func (r *APIPortalRepo) Count(orgUUID string, workflowStatus *string, search string) (int, error) {
	var args []interface{}
	conditions := []string{`organization_uuid = ?`}
	args = append(args, orgUUID)
	if workflowStatus != nil {
		conditions = append(conditions, `workflow_status = ?`)
		args = append(args, *workflowStatus)
	}
	if searchClause, searchArgs := handleSearchClause(search); searchClause != "" {
		conditions = append(conditions, strings.TrimPrefix(searchClause, " AND "))
		args = append(args, searchArgs...)
	}
	query := `SELECT COUNT(*) FROM api_portals WHERE ` + strings.Join(conditions, ` AND `)
	var total int
	if err := r.db.QueryRow(r.db.Rebind(query), args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// Update mutates only the whitelisted fields; immutable columns (uuid, organization_uuid,
// handle, data_version, created_by, created_at) are never touched. The caller is
// responsible for populating UpdatedBy before invoking.
func (r *APIPortalRepo) Update(portal *model.APIPortal) error {
	portal.UpdatedAt = time.Now().UTC()
	authConfigBytes, err := marshalAPIPortalBlob(portal.AuthConfig, "auth_configuration")
	if err != nil {
		return err
	}
	metadataBytes, err := marshalAPIPortalBlob(portal.Metadata, "metadata")
	if err != nil {
		return err
	}
	query := `
		UPDATE api_portals
		SET display_name = ?, description = ?, url = ?, workflow_status = ?,
		    auth_type = ?, auth_configuration = ?, metadata = ?,
		    updated_by = ?, updated_at = ?
		WHERE uuid = ? AND organization_uuid = ?
	`
	result, err := r.db.Exec(r.db.Rebind(query),
		portal.Name, portal.Description, portal.URL, portal.WorkflowStatus,
		portal.AuthType, authConfigBytes, metadataBytes,
		portal.UpdatedBy, portal.UpdatedAt,
		portal.ID, portal.OrganizationID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("api portal not found: uuid=%q organization_uuid=%q", portal.ID, portal.OrganizationID)
	}
	return nil
}

// Delete removes an API Portal row with organization isolation.
func (r *APIPortalRepo) Delete(portalID, orgUUID string) error {
	query := `DELETE FROM api_portals WHERE uuid = ? AND organization_uuid = ?`
	result, err := r.db.Exec(r.db.Rebind(query), portalID, orgUUID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("api portal not found: uuid=%q organization_uuid=%q", portalID, orgUUID)
	}
	return nil
}

// Exists reports whether an API Portal with the given handle exists in the organization.
func (r *APIPortalRepo) Exists(handle, orgUUID string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM api_portals WHERE handle = ? AND organization_uuid = ?`
	if err := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

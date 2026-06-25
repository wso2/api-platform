/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"
)

// WebBrokerAPIRepo handles database operations for WebBroker APIs
type WebBrokerAPIRepo struct {
	db           *database.DB
	artifactRepo *ArtifactRepo
}

// NewWebBrokerAPIRepo creates a new WebBrokerAPIRepo instance
func NewWebBrokerAPIRepo(db *database.DB) *WebBrokerAPIRepo {
	return &WebBrokerAPIRepo{db: db, artifactRepo: NewArtifactRepo(db)}
}

// Create creates a new WebBroker API in the database
func (r *WebBrokerAPIRepo) Create(a *model.WebBrokerAPI) error {
	uuidStr, err := utils.GenerateUUID()
	if err != nil {
		return fmt.Errorf("failed to generate WebBroker API ID: %w", err)
	}
	a.UUID = uuidStr
	now := time.Now().UTC()
	a.CreatedAt = now
	a.UpdatedAt = now

	configurationJSON, err := serializeWebBrokerAPIConfiguration(a.Configuration)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert into artifacts table first
	if err := r.artifactRepo.Create(tx, &model.Artifact{
		UUID:             a.UUID,
		Type:             constants.WebBrokerApi,
		OrganizationUUID: a.OrganizationUUID,
	}); err != nil {
		return fmt.Errorf("failed to create artifact: %w", err)
	}

	// Insert into webbroker_apis table
	query := `
		INSERT INTO webbroker_apis (
			uuid, organization_uuid, handle, name, version, project_uuid, description, created_by, lifecycle_status, configuration, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(r.db.Rebind(query),
		a.UUID, a.OrganizationUUID, a.Handle, a.Name, a.Version, a.ProjectUUID, a.Description, a.CreatedBy, a.LifeCycleStatus,
		configurationJSON, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create WebBroker API: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// GetByHandle retrieves a WebBroker API by its handle and organization UUID
func (r *WebBrokerAPIRepo) GetByHandle(handle, orgUUID string) (*model.WebBrokerAPI, error) {
	query := `
		SELECT
			uuid, handle, name, version, organization_uuid, created_at, updated_at,
			project_uuid, description, created_by, updated_by, lifecycle_status, configuration
		FROM webbroker_apis
		WHERE handle = ? AND organization_uuid = ?`
	row := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID)
	return r.scanWebBrokerAPI(row)
}

// GetByUUID retrieves a WebBroker API by its UUID and organization UUID
func (r *WebBrokerAPIRepo) GetByUUID(uuid, orgUUID string) (*model.WebBrokerAPI, error) {
	query := `
		SELECT
			uuid, handle, name, version, organization_uuid, created_at, updated_at,
			project_uuid, description, created_by, updated_by, lifecycle_status, configuration
		FROM webbroker_apis
		WHERE uuid = ? AND organization_uuid = ?`
	row := r.db.QueryRow(r.db.Rebind(query), uuid, orgUUID)
	return r.scanWebBrokerAPI(row)
}

// List retrieves all WebBroker APIs for an organization, optionally filtered by project
func (r *WebBrokerAPIRepo) List(orgUUID, projectUUID string, limit, offset int) ([]*model.WebBrokerAPI, error) {
	var query string
	var args []interface{}
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)

	if projectUUID != "" {
		query = `
			SELECT
				uuid, handle, name, version, organization_uuid, created_at, updated_at,
				project_uuid, description, created_by, updated_by, lifecycle_status, configuration
			FROM webbroker_apis
			WHERE organization_uuid = ? AND project_uuid = ?
			ORDER BY created_at DESC
			` + pageClause
		args = append([]interface{}{orgUUID, projectUUID}, pageArgs...)
	} else {
		query = `
			SELECT
				uuid, handle, name, version, organization_uuid, created_at, updated_at,
				project_uuid, description, created_by, updated_by, lifecycle_status, configuration
			FROM webbroker_apis
			WHERE organization_uuid = ?
			ORDER BY created_at DESC
			` + pageClause
		args = append([]interface{}{orgUUID}, pageArgs...)
	}

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.WebBrokerAPI
	for rows.Next() {
		a, err := r.scanWebBrokerAPIFromRows(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, rows.Err()
}

// Count returns the total number of WebBroker APIs for an organization
func (r *WebBrokerAPIRepo) Count(orgUUID string) (int, error) {
	return r.artifactRepo.CountByKindAndOrg(constants.WebBrokerApi, orgUUID)
}

// CountByProject returns the total number of WebBroker APIs for a specific project
func (r *WebBrokerAPIRepo) CountByProject(orgUUID, projectUUID string) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM webbroker_apis
		WHERE organization_uuid = ? AND project_uuid = ?`
	if err := r.db.QueryRow(r.db.Rebind(query), orgUUID, projectUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Update updates an existing WebBroker API
func (r *WebBrokerAPIRepo) Update(a *model.WebBrokerAPI) error {
	now := time.Now().UTC()
	a.UpdatedAt = now

	configurationJSON, err := serializeWebBrokerAPIConfiguration(a.Configuration)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the UUID from handle
	var apiUUID string
	query := `
		SELECT uuid FROM webbroker_apis
		WHERE handle = ? AND organization_uuid = ?`
	err = tx.QueryRow(r.db.Rebind(query), a.Handle, a.OrganizationUUID).Scan(&apiUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	// Update webbroker_apis table (name/version/updated_at now live here)
	query = `
		UPDATE webbroker_apis
		SET name = ?, version = ?, description = ?, lifecycle_status = ?, configuration = ?, updated_by = ?, updated_at = ?
		WHERE uuid = ?`
	result, err := tx.Exec(r.db.Rebind(query),
		a.Name, a.Version, a.Description, a.LifeCycleStatus, configurationJSON, a.UpdatedBy, now,
		apiUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to update WebBroker API: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// Delete deletes a WebBroker API by its handle and organization UUID
func (r *WebBrokerAPIRepo) Delete(handle, orgUUID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the UUID from handle
	var apiUUID string
	query := `
		SELECT uuid FROM webbroker_apis
		WHERE handle = ? AND organization_uuid = ?`
	err = tx.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(&apiUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	// Delete from webbroker_apis first, then artifacts
	_, err = tx.Exec(r.db.Rebind(`DELETE FROM webbroker_apis WHERE uuid = ?`), apiUUID)
	if err != nil {
		return err
	}

	if err := r.artifactRepo.Delete(tx, apiUUID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// Exists checks if a WebBroker API exists by its handle and organization UUID
func (r *WebBrokerAPIRepo) Exists(handle, orgUUID string) (bool, error) {
	return r.artifactRepo.Exists(constants.WebBrokerApi, handle, orgUUID)
}

// scanWebBrokerAPI scans a single Row into a WebBrokerAPI
func (r *WebBrokerAPIRepo) scanWebBrokerAPI(row *sql.Row) (*model.WebBrokerAPI, error) {
	var a model.WebBrokerAPI
	var configurationJSON sql.NullString
	if err := row.Scan(
		&a.UUID, &a.Handle, &a.Name, &a.Version, &a.OrganizationUUID, &a.CreatedAt, &a.UpdatedAt,
		&a.ProjectUUID, &a.Description, &a.CreatedBy, &a.UpdatedBy, &a.LifeCycleStatus, &configurationJSON,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if configurationJSON.Valid && configurationJSON.String != "" {
		if config, err := deserializeWebBrokerAPIConfiguration(configurationJSON); err != nil {
			return nil, fmt.Errorf("unmarshal configuration for WebBroker API %s: %w", a.Handle, err)
		} else if config != nil {
			a.Configuration = *config
		}
	}
	return &a, nil
}

// scanWebBrokerAPIFromRows scans a Rows row into a WebBrokerAPI
func (r *WebBrokerAPIRepo) scanWebBrokerAPIFromRows(rows *sql.Rows) (*model.WebBrokerAPI, error) {
	var a model.WebBrokerAPI
	var configurationJSON sql.NullString
	if err := rows.Scan(
		&a.UUID, &a.Handle, &a.Name, &a.Version, &a.OrganizationUUID, &a.CreatedAt, &a.UpdatedAt,
		&a.ProjectUUID, &a.Description, &a.CreatedBy, &a.UpdatedBy, &a.LifeCycleStatus, &configurationJSON,
	); err != nil {
		return nil, err
	}
	if configurationJSON.Valid && configurationJSON.String != "" {
		if config, err := deserializeWebBrokerAPIConfiguration(configurationJSON); err != nil {
			return nil, fmt.Errorf("unmarshal configuration for WebBroker API %s: %w", a.Handle, err)
		} else if config != nil {
			a.Configuration = *config
		}
	}
	return &a, nil
}

func serializeWebBrokerAPIConfiguration(config model.WebBrokerAPIConfiguration) (string, error) {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(configJSON), nil
}

func deserializeWebBrokerAPIConfiguration(configJSON sql.NullString) (*model.WebBrokerAPIConfiguration, error) {
	if !configJSON.Valid || configJSON.String == "" {
		return nil, fmt.Errorf("null configuration")
	}
	var config model.WebBrokerAPIConfiguration
	if err := json.Unmarshal([]byte(configJSON.String), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

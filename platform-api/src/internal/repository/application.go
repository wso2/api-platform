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
	"time"

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
)

type ApplicationRepo struct {
	db *database.DB
}

func NewApplicationRepo(db *database.DB) ApplicationRepository {
	return &ApplicationRepo{db: db}
}

func (r *ApplicationRepo) CreateApplication(app *model.Application) error {
	now := time.Now()
	app.CreatedAt = now
	app.UpdatedAt = now

	var projectUUID interface{}
	if app.ProjectUUID == "" {
		projectUUID = nil
	} else {
		projectUUID = app.ProjectUUID
	}

	query := `
		INSERT INTO applications (
			uuid, handle, project_uuid, organization_uuid, created_by,
			name, description, type, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Exec(
		r.db.Rebind(query),
		app.UUID, app.Handle, projectUUID, app.OrganizationUUID, app.CreatedBy,
		app.Name, app.Description, app.Type, app.CreatedAt, app.UpdatedAt,
	)
	return err
}

func (r *ApplicationRepo) GetApplicationByUUID(appID string) (*model.Application, error) {
	row := r.db.QueryRow(r.db.Rebind(`
		SELECT uuid, handle, project_uuid, organization_uuid, created_by, name, description, type, created_at, updated_at
		FROM applications
		WHERE uuid = ?
	`), appID)

	app, err := scanApplication(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return app, err
}

func (r *ApplicationRepo) GetApplicationByIDOrHandle(appIDOrHandle, orgID string) (*model.Application, error) {
	row := r.db.QueryRow(r.db.Rebind(`
		SELECT uuid, handle, project_uuid, organization_uuid, created_by, name, description, type, created_at, updated_at
		FROM applications
		WHERE organization_uuid = ? AND (uuid = ? OR handle = ?)
		ORDER BY CASE WHEN uuid = ? THEN 0 ELSE 1 END
		LIMIT 1
	`), orgID, appIDOrHandle, appIDOrHandle, appIDOrHandle)

	app, err := scanApplication(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return app, err
}

func (r *ApplicationRepo) GetArtifactByUUID(artifactUUID, orgID string) (*model.Artifact, error) {
	row := r.db.QueryRow(r.db.Rebind(`
		SELECT uuid, handle, name, version, kind, organization_uuid, created_at, updated_at
		FROM artifacts
		WHERE uuid = ? AND organization_uuid = ?
	`), artifactUUID, orgID)

	artifact := &model.Artifact{}
	err := row.Scan(
		&artifact.UUID,
		&artifact.Handle,
		&artifact.Name,
		&artifact.Version,
		&artifact.Kind,
		&artifact.OrganizationUUID,
		&artifact.CreatedAt,
		&artifact.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return artifact, nil
}

func (r *ApplicationRepo) GetApplicationsByProjectID(projectID, orgID string) ([]*model.Application, error) {
	rows, err := r.db.Query(r.db.Rebind(`
		SELECT uuid, handle, project_uuid, organization_uuid, created_by, name, description, type, created_at, updated_at
		FROM applications
		WHERE project_uuid = ? AND organization_uuid = ?
		ORDER BY created_at DESC, name ASC
	`), projectID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanApplications(rows)
}

func (r *ApplicationRepo) GetApplicationsByOrganizationID(orgID string) ([]*model.Application, error) {
	rows, err := r.db.Query(r.db.Rebind(`
		SELECT uuid, handle, project_uuid, organization_uuid, created_by, name, description, type, created_at, updated_at
		FROM applications
		WHERE organization_uuid = ?
		ORDER BY created_at DESC, name ASC
	`), orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanApplications(rows)
}

func (r *ApplicationRepo) GetApplicationByNameInProject(name, projectID, orgID string) (*model.Application, error) {
	var row *sql.Row
	if projectID == "" {
		row = r.db.QueryRow(r.db.Rebind(`
			SELECT uuid, handle, project_uuid, organization_uuid, created_by, name, description, type, created_at, updated_at
			FROM applications
			WHERE name = ? AND project_uuid IS NULL AND organization_uuid = ?
		`), name, orgID)
	} else {
		row = r.db.QueryRow(r.db.Rebind(`
			SELECT uuid, handle, project_uuid, organization_uuid, created_by, name, description, type, created_at, updated_at
			FROM applications
			WHERE name = ? AND project_uuid = ? AND organization_uuid = ?
		`), name, projectID, orgID)
	}

	app, err := scanApplication(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return app, err
}

func (r *ApplicationRepo) CheckApplicationHandleExists(handle, orgID string) (bool, error) {
	var count int
	err := r.db.QueryRow(r.db.Rebind(`
		SELECT COUNT(*)
		FROM applications
		WHERE handle = ? AND organization_uuid = ?
	`), handle, orgID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ApplicationRepo) UpdateApplication(app *model.Application) error {
	app.UpdatedAt = time.Now()

	_, err := r.db.Exec(r.db.Rebind(`
		UPDATE applications
		SET name = ?, description = ?, type = ?, updated_at = ?
		WHERE uuid = ? AND organization_uuid = ?
	`), app.Name, app.Description, app.Type, app.UpdatedAt, app.UUID, app.OrganizationUUID)
	return err
}

func (r *ApplicationRepo) DeleteApplication(appID, orgID string) error {
	_, err := r.db.Exec(r.db.Rebind(`DELETE FROM applications WHERE uuid = ? AND organization_uuid = ?`), appID, orgID)
	return err
}

func (r *ApplicationRepo) GetAPIKeyByNameAndArtifactHandle(keyName, artifactHandle, orgID string) (*model.ApplicationAPIKey, error) {
	row := r.db.QueryRow(r.db.Rebind(`
		SELECT ak.uuid, ak.name, ak.artifact_uuid, art.handle, art.kind, ak.status, ak.created_by, ak.created_at, ak.updated_at, ak.expires_at
		FROM api_keys ak
		INNER JOIN artifacts art ON art.uuid = ak.artifact_uuid
		WHERE art.organization_uuid = ? AND ak.name = ? AND art.handle = ?
	`), orgID, keyName, artifactHandle)

	key, err := scanApplicationAPIKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return key, err
}

func (r *ApplicationRepo) GetDeployedGatewayIDsByArtifactUUID(artifactUUID, orgID string) ([]string, error) {
	rows, err := r.db.Query(r.db.Rebind(`
		SELECT gateway_uuid
		FROM deployment_status
		WHERE artifact_uuid = ? AND organization_uuid = ? AND status = ?
	`), artifactUUID, orgID, string(model.DeploymentStatusDeployed))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var gatewayID string
		if err := rows.Scan(&gatewayID); err != nil {
			return nil, err
		}
		ids = append(ids, gatewayID)
	}

	return ids, rows.Err()
}

func (r *ApplicationRepo) ListMappedAPIKeys(applicationUUID string) ([]*model.ApplicationAPIKey, error) {
	rows, err := r.db.Query(r.db.Rebind(`
		SELECT ak.uuid, ak.name, ak.artifact_uuid, art.handle, art.kind, ak.status, ak.created_by, ak.created_at, ak.updated_at, ak.expires_at
		FROM application_api_keys aak
		INNER JOIN api_keys ak ON ak.uuid = aak.api_key_id
		INNER JOIN artifacts art ON art.uuid = ak.artifact_uuid
		WHERE aak.application_uuid = ?
		ORDER BY aak.created_at DESC, ak.name ASC, ak.uuid ASC
	`), applicationUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*model.ApplicationAPIKey
	for rows.Next() {
		key, err := scanApplicationAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
}

func (r *ApplicationRepo) ReplaceApplicationAPIKeys(applicationUUID string, apiKeyIDs []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.Exec(r.db.Rebind(`DELETE FROM application_api_keys WHERE application_uuid = ?`), applicationUUID); err != nil {
		return err
	}

	for _, apiKeyID := range uniqueStrings(apiKeyIDs) {
		now := time.Now()
		if _, err = tx.Exec(r.db.Rebind(`
			INSERT INTO application_api_keys (application_uuid, api_key_id, created_at, updated_at)
			VALUES (?, ?, ?, ?)
		`), applicationUUID, apiKeyID, now, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *ApplicationRepo) AddApplicationAPIKeys(applicationUUID string, apiKeyIDs []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingRows, err := tx.Query(r.db.Rebind(`
		SELECT api_key_id
		FROM application_api_keys
		WHERE application_uuid = ?
	`), applicationUUID)
	if err != nil {
		return err
	}

	existing := make(map[string]struct{})
	for existingRows.Next() {
		var apiKeyID string
		if err := existingRows.Scan(&apiKeyID); err != nil {
			_ = existingRows.Close()
			return err
		}
		existing[apiKeyID] = struct{}{}
	}
	if err := existingRows.Err(); err != nil {
		_ = existingRows.Close()
		return err
	}
	if err := existingRows.Close(); err != nil {
		return err
	}

	for _, apiKeyID := range uniqueStrings(apiKeyIDs) {
		if _, ok := existing[apiKeyID]; ok {
			continue
		}
		now := time.Now()
		if _, err = tx.Exec(r.db.Rebind(`
			INSERT INTO application_api_keys (application_uuid, api_key_id, created_at, updated_at)
			VALUES (?, ?, ?, ?)
		`), applicationUUID, apiKeyID, now, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *ApplicationRepo) RemoveApplicationAPIKey(applicationUUID, apiKeyID string) error {
	_, err := r.db.Exec(r.db.Rebind(`
		DELETE FROM application_api_keys
		WHERE application_uuid = ? AND api_key_id = ?
	`), applicationUUID, apiKeyID)
	return err
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanApplication(scanner rowScanner) (*model.Application, error) {
	var app model.Application
	var projectUUID sql.NullString
	var createdBy sql.NullString
	var description sql.NullString

	err := scanner.Scan(
		&app.UUID,
		&app.Handle,
		&projectUUID,
		&app.OrganizationUUID,
		&createdBy,
		&app.Name,
		&description,
		&app.Type,
		&app.CreatedAt,
		&app.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if projectUUID.Valid {
		app.ProjectUUID = projectUUID.String
	}

	if createdBy.Valid {
		app.CreatedBy = createdBy.String
	}
	if description.Valid {
		app.Description = description.String
	}

	return &app, nil
}

func scanApplications(rows *sql.Rows) ([]*model.Application, error) {
	var apps []*model.Application
	for rows.Next() {
		app, err := scanApplication(rows)
		if err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, rows.Err()
}

func scanApplicationAPIKey(scanner rowScanner) (*model.ApplicationAPIKey, error) {
	var key model.ApplicationAPIKey
	var apiKeyUUID string
	var status sql.NullString
	var createdBy sql.NullString
	var expiresAt sql.NullTime

	err := scanner.Scan(
		&apiKeyUUID,
		&key.Name,
		&key.ArtifactID,
		&key.ArtifactHandle,
		&key.ArtifactKind,
		&status,
		&createdBy,
		&key.CreatedAt,
		&key.UpdatedAt,
		&expiresAt,
	)
	if err != nil {
		return nil, err
	}

	// API response ID is derived from the API key UUID selected as ak.uuid.
	key.ID = apiKeyUUID
	key.APIKeyUUID = apiKeyUUID

	if status.Valid {
		key.Status = status.String
	}
	if createdBy.Valid {
		key.CreatedBy = createdBy.String
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		key.ExpiresAt = &t
	}

	return &key, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

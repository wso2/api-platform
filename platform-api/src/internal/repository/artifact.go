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

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
	"time"
)

type ArtifactRepo struct {
	db *database.DB
}

func NewArtifactRepo(db *database.DB) *ArtifactRepo {
	return &ArtifactRepo{db: db}
}

func (r *ArtifactRepo) Create(tx *sql.Tx, artifact *model.Artifact) error {
	now := time.Now().UTC()
	query := `
		INSERT INTO artifacts (uuid, handle, name, version, kind, organization_uuid, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(r.db.Rebind(query), artifact.UUID, artifact.Handle, artifact.Name, artifact.Version, artifact.Kind, artifact.OrganizationUUID, now, now)
	return err
}

func (r *ArtifactRepo) Delete(tx *sql.Tx, uuid string) error {
	query := `DELETE FROM artifacts WHERE uuid = ?`
	result, err := tx.Exec(r.db.Rebind(query), uuid)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *ArtifactRepo) Update(tx *sql.Tx, artifact *model.Artifact) error {
	query := `
		UPDATE artifacts SET name = ?, version = ?, updated_at = ?
		WHERE uuid = ? AND organization_uuid = ?
	`
	_, err := tx.Exec(r.db.Rebind(query), artifact.Name, artifact.Version, artifact.UpdatedAt, artifact.UUID, artifact.OrganizationUUID)
	return err
}

func (r *ArtifactRepo) Exists(kind, handle, orgUUID string) (bool, error) {
	query := `SELECT COUNT(*) FROM artifacts WHERE kind = ? AND handle = ? AND organization_uuid = ?`
	var count int
	err := r.db.QueryRow(r.db.Rebind(query), kind, handle, orgUUID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ArtifactRepo) GetByHandle(handle, orgUUID string) (*model.Artifact, error) {
	artifact := &model.Artifact{}
	query := `SELECT uuid, handle, name, version, kind, organization_uuid, created_at, updated_at FROM artifacts WHERE handle = ? AND organization_uuid = ?`
	err := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(
		&artifact.UUID, &artifact.Handle, &artifact.Name, &artifact.Version,
		&artifact.Kind, &artifact.OrganizationUUID, &artifact.CreatedAt, &artifact.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return artifact, nil
}

func (r *ArtifactRepo) CountByKindAndOrg(kind, orgUUID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM artifacts WHERE kind = ? AND organization_uuid = ?`
	err := r.db.QueryRow(r.db.Rebind(query), kind, orgUUID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ExistsByUUIDs returns the subset of provided UUIDs that exist in the artifacts table for the given org.
func (r *ArtifactRepo) ExistsByUUIDs(uuids []string, orgUUID string) ([]string, error) {
	if len(uuids) == 0 {
		return nil, nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(uuids))
	args := make([]interface{}, 0, len(uuids)+1)
	for i, uuid := range uuids {
		placeholders[i] = "?"
		args = append(args, uuid)
	}
	args = append(args, orgUUID)

	query := fmt.Sprintf(
		`SELECT uuid FROM artifacts WHERE uuid IN (%s) AND organization_uuid = ?`,
		strings.Join(placeholders, ", "),
	)

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to check artifact existence: %w", err)
	}
	defer rows.Close()

	var existing []string
	for rows.Next() {
		var uuid string
		if err := rows.Scan(&uuid); err != nil {
			return nil, err
		}
		existing = append(existing, uuid)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating artifact UUIDs: %w", err)
	}

	return existing, nil
}

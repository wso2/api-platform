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
	"strings"

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
)

type ArtifactRepo struct {
	db  *database.DB
	reg *ArtifactTableRegistry
}

// NewArtifactRepo creates an ArtifactRepo. When reg is provided it is used for
// dynamic UNION queries and kind validation; when omitted the core-only default
// registry (rest_apis, llm_providers, llm_proxies, mcp_proxies) is used.
func NewArtifactRepo(db *database.DB, reg ...*ArtifactTableRegistry) *ArtifactRepo {
	r := NewArtifactTableRegistry()
	if len(reg) > 0 && reg[0] != nil {
		r = reg[0]
	}
	return &ArtifactRepo{db: db, reg: r}
}

func (r *ArtifactRepo) Create(tx *sql.Tx, artifact *model.Artifact) error {
	if !r.reg.IsValidKindAlias(artifact.Type) {
		return fmt.Errorf("invalid artifact type: %q", artifact.Type)
	}
	query := `
		INSERT INTO artifacts (uuid, type, organization_uuid)
		VALUES (?, ?, ?)
	`
	_, err := tx.Exec(r.db.Rebind(query), artifact.UUID, artifact.Type, artifact.OrganizationUUID)
	return err
}

// Update is a no-op: artifact rows no longer store mutable fields (name/version moved to type-specific tables).
func (r *ArtifactRepo) Update(_ *sql.Tx, _ *model.Artifact) error {
	return nil
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

// Exists checks whether an artifact with the given type+handle exists for the org.
// Since handle moved to the type-specific tables, we query them directly.
func (r *ArtifactRepo) Exists(kind, handle, orgUUID string) (bool, error) {
	entry, ok := r.reg.TableByKindKey(kind)
	if !ok {
		return false, nil
	}
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE handle = ? AND organization_uuid = ?`, entry.Table)
	var count int
	err := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetAPIMetadataByHandle retrieves minimal API metadata by handle across all registered artifact tables.
func (r *ArtifactRepo) GetAPIMetadataByHandle(handle, orgUUID string) (*model.APIMetadata, error) {
	entries := r.reg.Entries()
	parts := make([]string, len(entries))
	args := make([]interface{}, 0, len(entries)*2)
	for i, e := range entries {
		parts[i] = fmt.Sprintf(
			"SELECT uuid, handle, display_name, version, '%s' AS type, organization_uuid FROM %s WHERE handle = ? AND organization_uuid = ?",
			e.KindAlias, e.Table,
		)
		args = append(args, handle, orgUUID)
	}
	query := "SELECT uuid, handle, display_name, version, type, organization_uuid FROM (\n\t\t\t" +
		strings.Join(parts, "\n\t\t\tUNION ALL\n\t\t\t") +
		"\n\t\t) combined"

	metadata := &model.APIMetadata{}
	err := r.db.QueryRow(r.db.Rebind(query), args...).Scan(
		&metadata.ID, &metadata.Handle, &metadata.Name, &metadata.Version, &metadata.Kind, &metadata.OrganizationID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return metadata, nil
}

// GetAPIMetadataByHandleAndKind retrieves minimal API metadata by handle from the
// table backing the given kind. Unlike GetAPIMetadataByHandle it does not union across
// every table, so the caller resolves the handle against exactly one artifact kind.
// Returns (nil, nil) when no matching artifact exists.
func (r *ArtifactRepo) GetAPIMetadataByHandleAndKind(handle, kind, orgUUID string) (*model.APIMetadata, error) {
	entry, ok := r.reg.TableByKindKey(kind)
	if !ok {
		return nil, fmt.Errorf("invalid artifact kind: %q", kind)
	}
	query := fmt.Sprintf(
		"SELECT uuid, handle, display_name, version, '%s' AS type, organization_uuid FROM %s WHERE handle = ? AND organization_uuid = ?",
		entry.KindAlias, entry.Table,
	)

	metadata := &model.APIMetadata{}
	err := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(
		&metadata.ID, &metadata.Handle, &metadata.Name, &metadata.Version, &metadata.Kind, &metadata.OrganizationID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return metadata, nil
}

// GetMetadataByUUIDs returns a map of artifact UUID to minimal metadata (handle, kind, ...)
// for the given UUIDs, resolved across all registered artifact tables. Missing UUIDs are
// simply absent from the returned map. Used for bulk handle/kind resolution (avoids N+1 queries).
func (r *ArtifactRepo) GetMetadataByUUIDs(uuids []string, orgUUID string) (map[string]*model.APIMetadata, error) {
	result := make(map[string]*model.APIMetadata)
	if len(uuids) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(uuids))
	for i := range uuids {
		placeholders[i] = "?"
	}
	inClause := strings.Join(placeholders, ", ")

	entries := r.reg.Entries()
	parts := make([]string, len(entries))
	args := make([]interface{}, 0, len(entries)*(len(uuids)+1))
	for i, e := range entries {
		parts[i] = fmt.Sprintf(
			"SELECT uuid, handle, display_name, version, '%s' AS type, organization_uuid FROM %s WHERE uuid IN (%s) AND organization_uuid = ?",
			e.KindAlias, e.Table, inClause,
		)
		for _, u := range uuids {
			args = append(args, u)
		}
		args = append(args, orgUUID)
	}
	query := "SELECT uuid, handle, display_name, version, type, organization_uuid FROM (\n\t\t\t" +
		strings.Join(parts, "\n\t\t\tUNION ALL\n\t\t\t") +
		"\n\t\t) combined"

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		m := &model.APIMetadata{}
		if err := rows.Scan(&m.ID, &m.Handle, &m.Name, &m.Version, &m.Kind, &m.OrganizationID); err != nil {
			return nil, err
		}
		result[m.ID] = m
	}
	return result, rows.Err()
}

// GetByHandle finds an artifact by handle across all registered artifact tables.
// Returns the artifact with its supplemental fields derived from the matching table.
func (r *ArtifactRepo) GetByHandle(handle, orgUUID string) (*model.Artifact, error) {
	entries := r.reg.Entries()
	parts := make([]string, len(entries))
	args := make([]interface{}, 0, len(entries)*2)
	for i, e := range entries {
		parts[i] = fmt.Sprintf(
			"SELECT uuid, handle, display_name, version, '%s' AS type, organization_uuid, origin FROM %s WHERE handle = ? AND organization_uuid = ?",
			e.KindAlias, e.Table,
		)
		args = append(args, handle, orgUUID)
	}
	query := "SELECT uuid, handle, display_name, version, type, organization_uuid, origin FROM (\n\t\t\t" +
		strings.Join(parts, "\n\t\t\tUNION ALL\n\t\t\t") +
		"\n\t\t) combined ORDER BY (SELECT NULL) " + r.db.FetchFirstClause(1)

	artifact := &model.Artifact{}
	err := r.db.QueryRow(r.db.Rebind(query), args...).Scan(
		&artifact.UUID, &artifact.Handle, &artifact.Name, &artifact.Version,
		&artifact.Type, &artifact.OrganizationUUID, &artifact.Origin,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return artifact, nil
}

// GetByUUID finds an artifact by UUID across all registered artifact tables.
// Returns the artifact with its supplemental fields derived from the matching table.
func (r *ArtifactRepo) GetByUUID(uuid, orgUUID string) (*model.Artifact, error) {
	entries := r.reg.Entries()
	parts := make([]string, len(entries))
	args := make([]interface{}, 0, len(entries)*2)
	for i, e := range entries {
		parts[i] = fmt.Sprintf(
			"SELECT uuid, handle, display_name, version, '%s' AS type, organization_uuid, origin FROM %s WHERE uuid = ? AND organization_uuid = ?",
			e.KindAlias, e.Table,
		)
		args = append(args, uuid, orgUUID)
	}
	query := "SELECT uuid, handle, display_name, version, type, organization_uuid, origin FROM (\n\t\t\t" +
		strings.Join(parts, "\n\t\t\tUNION ALL\n\t\t\t") +
		"\n\t\t) combined ORDER BY (SELECT NULL)" + r.db.FetchFirstClause(1)

	artifact := &model.Artifact{}
	err := r.db.QueryRow(r.db.Rebind(query), args...).Scan(
		&artifact.UUID, &artifact.Handle, &artifact.Name, &artifact.Version,
		&artifact.Type, &artifact.OrganizationUUID, &artifact.Origin,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return artifact, nil
}

func (r *ArtifactRepo) CountByKindAndOrg(kind, orgUUID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM artifacts WHERE type = ? AND organization_uuid = ?`
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

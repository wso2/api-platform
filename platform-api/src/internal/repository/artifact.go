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

	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
)

// originJoin derives an artifact's origin from its kind-specific child table. The
// origin column lives on each artifact-backed table (rest_apis, llm_providers,
// llm_proxies, mcp_proxies, websub_apis, webbroker_apis) rather than on artifacts,
// so reads that need it LEFT JOIN every child table and COALESCE the first non-null
// value, defaulting to control_plane for any artifact without a child row.
const originJoin = `
	LEFT JOIN rest_apis ra ON a.uuid = ra.uuid
	LEFT JOIN llm_providers lpr ON a.uuid = lpr.uuid
	LEFT JOIN llm_proxies lpx ON a.uuid = lpx.uuid
	LEFT JOIN mcp_proxies mp ON a.uuid = mp.uuid
	LEFT JOIN websub_apis ws ON a.uuid = ws.uuid
	LEFT JOIN webbroker_apis wb ON a.uuid = wb.uuid`

const originCoalesce = `COALESCE(ra.origin, lpr.origin, lpx.origin, mp.origin, ws.origin, wb.origin, 'control_plane')`

type ArtifactRepo struct {
	db *database.DB
}

func NewArtifactRepo(db *database.DB) *ArtifactRepo {
	return &ArtifactRepo{db: db}
}

func (r *ArtifactRepo) Create(tx *sql.Tx, artifact *model.Artifact) error {
	if !constants.ValidArtifactKinds[artifact.Type] {
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
	var table string
	switch kind {
	case "rest-api", "RestApi":
		table = "rest_apis"
	case "websub-api", "WebSubApi":
		table = "websub_apis"
	case "webbroker-api", "WebBrokerApi":
		table = "webbroker_apis"
	case "llm-provider", "LlmProvider":
		table = "llm_providers"
	case "llm-proxy", "LlmProxy":
		table = "llm_proxies"
	case "mcp-proxy", "MCPProxy", "Mcp":
		table = "mcp_proxies"
	default:
		return false, nil
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE handle = ? AND organization_uuid = ?`, table)
	var count int
	err := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetAPIMetadataByHandle retrieves minimal API metadata by handle across all API type tables.
func (r *ArtifactRepo) GetAPIMetadataByHandle(handle, orgUUID string) (*model.APIMetadata, error) {
	query := `
		SELECT uuid, handle, display_name, version, type, organization_uuid
		FROM (
			SELECT uuid, handle, display_name, version, 'RestApi'      AS type, organization_uuid FROM rest_apis      WHERE handle = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'LlmProxy'     AS type, organization_uuid FROM llm_proxies    WHERE handle = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'Mcp'          AS type, organization_uuid FROM mcp_proxies    WHERE handle = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'WebSubApi'    AS type, organization_uuid FROM websub_apis    WHERE handle = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'WebBrokerApi' AS type, organization_uuid FROM webbroker_apis WHERE handle = ? AND organization_uuid = ?
		) combined
	`
	metadata := &model.APIMetadata{}
	err := r.db.QueryRow(r.db.Rebind(query),
		handle, orgUUID, handle, orgUUID, handle, orgUUID, handle, orgUUID, handle, orgUUID,
	).Scan(&metadata.ID, &metadata.Handle, &metadata.Name, &metadata.Version, &metadata.Kind, &metadata.OrganizationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return metadata, nil
}

// GetByHandle finds an artifact by handle across all type-specific tables.
// Returns the artifact with its supplemental fields (handle, name, version, type, origin)
// derived from the matching kind-specific table.
func (r *ArtifactRepo) GetByHandle(handle, orgUUID string) (*model.Artifact, error) {
	query := `
		SELECT uuid, handle, display_name, version, type, organization_uuid, origin FROM (
			SELECT uuid, handle, display_name, version, 'RestApi'      AS type, organization_uuid, origin FROM rest_apis      WHERE handle = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'WebSubApi'    AS type, organization_uuid, origin FROM websub_apis    WHERE handle = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'WebBrokerApi' AS type, organization_uuid, origin FROM webbroker_apis WHERE handle = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'LlmProvider'  AS type, organization_uuid, origin FROM llm_providers  WHERE handle = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'LlmProxy'     AS type, organization_uuid, origin FROM llm_proxies    WHERE handle = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'Mcp'          AS type, organization_uuid, origin FROM mcp_proxies    WHERE handle = ? AND organization_uuid = ?
		) combined
		ORDER BY (SELECT NULL)
		` + r.db.FetchFirstClause(1)
	artifact := &model.Artifact{}
	err := r.db.QueryRow(r.db.Rebind(query),
		handle, orgUUID, handle, orgUUID, handle, orgUUID, handle, orgUUID, handle, orgUUID, handle, orgUUID,
	).Scan(&artifact.UUID, &artifact.Handle, &artifact.Name, &artifact.Version, &artifact.Type, &artifact.OrganizationUUID, &artifact.Origin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return artifact, nil
}

// GetByUUID finds an artifact by uuid across all type-specific tables.
// Returns the artifact with its supplemental fields (handle, name, version, type, origin)
// derived from the matching kind-specific table.
func (r *ArtifactRepo) GetByUUID(uuid, orgUUID string) (*model.Artifact, error) {
	query := `
		SELECT uuid, handle, display_name, version, type, organization_uuid, origin FROM (
			SELECT uuid, handle, display_name, version, 'RestApi'      AS type, organization_uuid, origin FROM rest_apis      WHERE uuid = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'WebSubApi'    AS type, organization_uuid, origin FROM websub_apis    WHERE uuid = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'WebBrokerApi' AS type, organization_uuid, origin FROM webbroker_apis WHERE uuid = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'LlmProvider'  AS type, organization_uuid, origin FROM llm_providers  WHERE uuid = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'LlmProxy'     AS type, organization_uuid, origin FROM llm_proxies    WHERE uuid = ? AND organization_uuid = ?
			UNION ALL
			SELECT uuid, handle, display_name, version, 'Mcp'          AS type, organization_uuid, origin FROM mcp_proxies    WHERE uuid = ? AND organization_uuid = ?
		) combined
		ORDER BY (SELECT NULL)
		` + r.db.FetchFirstClause(1)
	artifact := &model.Artifact{}
	err := r.db.QueryRow(r.db.Rebind(query),
		uuid, orgUUID, uuid, orgUUID, uuid, orgUUID, uuid, orgUUID, uuid, orgUUID, uuid, orgUUID,
	).Scan(&artifact.UUID, &artifact.Handle, &artifact.Name, &artifact.Version, &artifact.Type, &artifact.OrganizationUUID, &artifact.Origin)
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

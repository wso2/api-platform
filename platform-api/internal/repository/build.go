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
	"time"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// Build persistence lives on DeploymentRepo: a build is the deploy path's own
// input, and keeping it here avoids a second repository for one table.

// CreateBuild stores a rendered snapshot of an API's definition. Builds are
// immutable, so there is no update — preparing again creates another build.
func (r *DeploymentRepo) CreateBuild(build *model.Build) error {
	if build.BuildID == "" {
		buildID, err := utils.GenerateUUID()
		if err != nil {
			return fmt.Errorf("failed to generate build ID: %w", err)
		}
		build.BuildID = buildID
	}
	if build.CreatedAt.IsZero() {
		build.CreatedAt = time.Now().UTC()
	} else {
		build.CreatedAt = build.CreatedAt.UTC()
	}

	const query = `
		INSERT INTO builds (uuid, artifact_uuid, organization_uuid, content, data_version, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query),
		build.BuildID, build.ArtifactID, build.OrganizationID,
		build.Content, build.DataVersion, build.CreatedBy, build.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create build: %w", err)
	}
	return nil
}

// GetBuild returns one build of an API, including its content. Scoping by
// artifact and organization is what keeps a build id from another API — or
// another organization — resolving here.
func (r *DeploymentRepo) GetBuild(buildID, artifactUUID, orgUUID string) (*model.Build, error) {
	const query = `
		SELECT uuid, artifact_uuid, organization_uuid, content, data_version, created_by, created_at
		FROM builds
		WHERE uuid = ? AND artifact_uuid = ? AND organization_uuid = ?
	`
	var build model.Build
	var createdBy sql.NullString
	err := r.db.QueryRow(r.db.Rebind(query), buildID, artifactUUID, orgUUID).Scan(
		&build.BuildID, &build.ArtifactID, &build.OrganizationID,
		&build.Content, &build.DataVersion, &createdBy, &build.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get build: %w", err)
	}
	build.CreatedBy = createdBy.String
	return &build, nil
}

// GetBuilds lists an API's builds newest first, without their content — a
// listing is for choosing a build, and the artifacts are large.
func (r *DeploymentRepo) GetBuilds(artifactUUID, orgUUID string, limit int) ([]*model.Build, error) {
	if limit <= 0 {
		limit = 50
	}
	const query = `
		SELECT uuid, artifact_uuid, organization_uuid, data_version, created_by, created_at
		FROM builds
		WHERE artifact_uuid = ? AND organization_uuid = ?
		ORDER BY created_at DESC, uuid DESC
		LIMIT ?
	`
	rows, err := r.db.Query(r.db.Rebind(query), artifactUUID, orgUUID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list builds: %w", err)
	}
	defer rows.Close()

	builds := make([]*model.Build, 0)
	for rows.Next() {
		var build model.Build
		var createdBy sql.NullString
		if err := rows.Scan(
			&build.BuildID, &build.ArtifactID, &build.OrganizationID,
			&build.DataVersion, &createdBy, &build.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan build: %w", err)
		}
		build.CreatedBy = createdBy.String
		builds = append(builds, &build)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read builds: %w", err)
	}
	return builds, nil
}

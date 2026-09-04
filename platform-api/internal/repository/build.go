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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// Build persistence lives on DeploymentRepo: a build is the deploy path's own
// input, and keeping it here avoids a second repository for one table.

// buildIDAttempts bounds the retries when deriving a build id. Two prepares of the
// same API on the same day compete for the same index, and the primary key is what
// settles it; a handful of attempts is far more than a real race needs.
const buildIDAttempts = 5

// CreateBuildWithLimitEnforcement stores a rendered snapshot of an API's
// definition, first pruning that API's older builds back within hardLimit. Builds
// are immutable, so there is no update — preparing again creates another build.
//
// A build id is readable rather than random: the date and that day's index for the
// API, e.g. 2026-01-31-1 then 2026-01-31-2. It is an id people name in a support
// ticket or a log line, which a UUID is not. It is unique per API, so the artifact
// is always part of resolving one.
func (r *DeploymentRepo) CreateBuildWithLimitEnforcement(build *model.Build, hardLimit int) error {
	if err := r.pruneBuilds(build.ArtifactID, build.OrganizationID, hardLimit); err != nil {
		return err
	}
	if build.UUID == "" {
		buildUUID, err := utils.GenerateUUID()
		if err != nil {
			return fmt.Errorf("failed to generate build UUID: %w", err)
		}
		build.UUID = buildUUID
	}
	if build.CreatedAt.IsZero() {
		build.CreatedAt = time.Now().UTC()
	} else {
		build.CreatedAt = build.CreatedAt.UTC()
	}
	if build.BuildID != "" {
		return r.insertBuild(build)
	}

	var insertErr error
	for attempt := 0; attempt < buildIDAttempts; attempt++ {
		next, err := r.nextBuildID(build.ArtifactID, build.OrganizationID, build.CreatedAt)
		if err != nil {
			return err
		}
		if attempt > 0 && next == build.BuildID {
			// The id we just tried is still free, so the insert failed on something
			// other than a concurrent prepare and retrying cannot help.
			return insertErr
		}
		build.BuildID = next
		if insertErr = r.insertBuild(build); insertErr == nil {
			return nil
		}
	}
	return insertErr
}

// nextBuildID returns the next unused id for an API on the given day. Reading the
// day's ids and taking the highest index — rather than counting rows — keeps the
// sequence correct even after an API's builds are pruned.
func (r *DeploymentRepo) nextBuildID(artifactUUID, orgUUID string, day time.Time) (string, error) {
	prefix := day.UTC().Format("2006-01-02") + "-"
	const query = `
		SELECT build_id
		FROM builds
		WHERE artifact_uuid = ? AND organization_uuid = ? AND build_id LIKE ?
	`
	rows, err := r.db.Query(r.db.Rebind(query), artifactUUID, orgUUID, prefix+"%")
	if err != nil {
		return "", fmt.Errorf("failed to read build ids: %w", err)
	}
	defer rows.Close()

	highest := 0
	for rows.Next() {
		var buildID string
		if err := rows.Scan(&buildID); err != nil {
			return "", fmt.Errorf("failed to scan build id: %w", err)
		}
		index, err := strconv.Atoi(strings.TrimPrefix(buildID, prefix))
		if err != nil {
			continue
		}
		if index > highest {
			highest = index
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to read build ids: %w", err)
	}
	return prefix + strconv.Itoa(highest+1), nil
}

// insertBuild writes one build row.
func (r *DeploymentRepo) insertBuild(build *model.Build) error {
	var propertyBytes []byte
	if len(build.Properties) > 0 {
		var err error
		propertyBytes, err = json.Marshal(build.Properties)
		if err != nil {
			return fmt.Errorf("failed to marshal build properties: %w", err)
		}
	}

	const query = `
		INSERT INTO builds (uuid, build_id, artifact_uuid, organization_uuid, content, data_version, properties, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query),
		build.UUID, build.BuildID, build.ArtifactID, build.OrganizationID,
		build.Content, build.DataVersion, propertyBytes, build.CreatedBy, build.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create build: %w", err)
	}
	return nil
}

// applyBuildProperties decodes the stored property bag onto the model.
func applyBuildProperties(build *model.Build, propertyBytes []byte) error {
	if len(propertyBytes) == 0 {
		return nil
	}
	var properties map[string]any
	if err := json.Unmarshal(propertyBytes, &properties); err != nil {
		return fmt.Errorf("failed to unmarshal build properties: %w", err)
	}
	build.Properties = properties
	return nil
}

// GetBuild returns one build of an API, including its content. Scoping by
// artifact and organization is what keeps a build id from another API — or
// another organization — resolving here.
func (r *DeploymentRepo) GetBuild(buildID, artifactUUID, orgUUID string) (*model.Build, error) {
	const query = `
		SELECT uuid, build_id, artifact_uuid, organization_uuid, content, data_version, properties, created_by, created_at
		FROM builds
		WHERE build_id = ? AND artifact_uuid = ? AND organization_uuid = ?
	`
	var build model.Build
	var createdBy sql.NullString
	var propertyBytes []byte
	err := r.db.QueryRow(r.db.Rebind(query), buildID, artifactUUID, orgUUID).Scan(
		&build.UUID, &build.BuildID, &build.ArtifactID, &build.OrganizationID,
		&build.Content, &build.DataVersion, &propertyBytes, &createdBy, &build.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get build: %w", err)
	}
	if err := applyBuildProperties(&build, propertyBytes); err != nil {
		return nil, err
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
		SELECT uuid, build_id, artifact_uuid, organization_uuid, data_version, properties, created_by, created_at
		FROM builds
		WHERE artifact_uuid = ? AND organization_uuid = ?
		ORDER BY created_at DESC, build_id DESC
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
		var propertyBytes []byte
		if err := rows.Scan(
			&build.UUID, &build.BuildID, &build.ArtifactID, &build.OrganizationID,
			&build.DataVersion, &propertyBytes, &createdBy, &build.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan build: %w", err)
		}
		if err := applyBuildProperties(&build, propertyBytes); err != nil {
			return nil, err
		}
		build.CreatedBy = createdBy.String
		builds = append(builds, &build)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read builds: %w", err)
	}
	return builds, nil
}

// pruneBuilds keeps an API's stored builds within hardLimit before another one is
// added. The budget is per API — one API's history cannot be crowded out by
// another's, and unlike deployments a build belongs to no gateway, so there is
// nothing narrower to count by.
//
// Age alone does not decide what goes. A build is deleted only when no gateway's
// CURRENT deployment came from it: an old build that something is still serving is
// exactly the one that must survive, because it is what a promotion out of that
// environment carries and what a redeploy of that gateway sends. Age only orders
// the builds that are free to go.
//
// It is deliberately best-effort. It removes at most a batch, and if every old
// build is still in use the API simply keeps more than the limit rather than
// failing the prepare or deleting something that is running. It is also not
// transactional: two prepares racing here can each delete a batch, which is
// harmless because both only ever delete builds nothing is serving.
func (r *DeploymentRepo) pruneBuilds(artifactUUID, orgUUID string, hardLimit int) error {
	// A limit of zero or less means keep everything.
	if hardLimit <= 0 {
		return nil
	}

	const countQuery = `
		SELECT COUNT(*)
		FROM builds
		WHERE artifact_uuid = ? AND organization_uuid = ?
	`
	var count int
	if err := r.db.QueryRow(r.db.Rebind(countQuery), artifactUUID, orgUUID).Scan(&count); err != nil {
		return fmt.Errorf("failed to count builds: %w", err)
	}
	if count < hardLimit {
		return nil
	}

	inUse, err := r.buildsInUse(artifactUUID, orgUUID)
	if err != nil {
		return err
	}

	const oldestQuery = `
		SELECT uuid
		FROM builds
		WHERE artifact_uuid = ? AND organization_uuid = ?
		ORDER BY created_at ASC, build_id ASC
	`
	rows, err := r.db.Query(r.db.Rebind(oldestQuery), artifactUUID, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to list builds for cleanup: %w", err)
	}
	var expendable []string
	for rows.Next() {
		var buildUUID string
		if err := rows.Scan(&buildUUID); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan build for cleanup: %w", err)
		}
		if inUse[buildUUID] {
			continue
		}
		expendable = append(expendable, buildUUID)
		if len(expendable) == constants.BuildCleanupBatch {
			break
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to read builds for cleanup: %w", err)
	}

	// The reference is cleared before the row goes: deployments outlive the build
	// they came from, and their readable build id in metadata is what keeps the
	// origin legible once the snapshot itself is gone.
	const clearQuery = `UPDATE deployments SET build_uuid = NULL WHERE build_uuid = ?`
	const deleteQuery = `DELETE FROM builds WHERE uuid = ?`
	for _, buildUUID := range expendable {
		if _, err := r.db.Exec(r.db.Rebind(clearQuery), buildUUID); err != nil {
			return fmt.Errorf("failed to clear references to build %s: %w", buildUUID, err)
		}
		if _, err := r.db.Exec(r.db.Rebind(deleteQuery), buildUUID); err != nil {
			return fmt.Errorf("failed to delete build %s: %w", buildUUID, err)
		}
	}
	return nil
}

// buildsInUse returns the builds an API's gateways are currently deployed from,
// by uuid.
//
// One deployment per gateway is current — the one deployment_status names — and its
// build_uuid says which build it came from. Only those rows count: an archived
// deployment carries its own rendered content and never needs its build back, so it
// is not a reason to keep one.
func (r *DeploymentRepo) buildsInUse(artifactUUID, orgUUID string) (map[string]bool, error) {
	const query = `
		SELECT DISTINCT d.build_uuid
		FROM deployments d
		JOIN deployment_status s ON d.uuid = s.deployment_uuid
			AND d.artifact_uuid = s.artifact_uuid
			AND d.organization_uuid = s.organization_uuid
			AND d.gateway_uuid = s.gateway_uuid
		WHERE d.artifact_uuid = ? AND d.organization_uuid = ? AND d.build_uuid IS NOT NULL
	`
	rows, err := r.db.Query(r.db.Rebind(query), artifactUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to read deployed builds: %w", err)
	}
	defer rows.Close()

	inUse := map[string]bool{}
	for rows.Next() {
		var buildUUID string
		if err := rows.Scan(&buildUUID); err != nil {
			return nil, fmt.Errorf("failed to scan deployed build: %w", err)
		}
		inUse[buildUUID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read deployed builds: %w", err)
	}
	return inUse, nil
}

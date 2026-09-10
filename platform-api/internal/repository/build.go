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
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// Build persistence lives on DeploymentRepo: a build is the deploy path's own
// input, and keeping it here avoids a second repository for one table.

// ErrBuildLimitReached is returned when an API is at its build limit and every
// stored build is held by a deployment, so preparing another would have to remove
// one that is still needed. The remedy is the caller's to choose — which
// deployment to give up — so this surfaces rather than being resolved here.
var ErrBuildLimitReached = errors.New("build limit reached and no build is free to remove")

// ErrBuildInUse is returned when a build a caller asked to delete is held by a
// deployment.
var ErrBuildInUse = errors.New("build is in use by a deployment")

// ErrBuildNotFound is returned when the build named for deletion is not one of
// the API's builds.
var ErrBuildNotFound = errors.New("build not found")

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
	if err := initBuild(build); err != nil {
		return err
	}
	return createWithDerivedBuildID(build, func() error {
		return r.createBuild(build, hardLimit)
	})
}

// initBuild fills in the identity and the timestamp a build is stored with.
func initBuild(build *model.Build) error {
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
	return nil
}

// createWithDerivedBuildID runs one attempt at a time until the id derived for the
// build sticks: the loser of a race for the same index simply derives the next one
// and tries again. An id the caller chose is used as given — there is no index to
// re-derive, so a failure with one is final.
func createWithDerivedBuildID(build *model.Build, attempt func() error) error {
	if build.BuildID != "" {
		return attempt()
	}
	var err error
	for i := 0; i < buildIDAttempts; i++ {
		derived := build.BuildID
		// Cleared so the attempt derives the next free index rather than reusing an
		// id that has just been taken.
		build.BuildID = ""
		if err = attempt(); err == nil {
			return nil
		}
		if errors.Is(err, ErrBuildLimitReached) {
			// Not a race for an id — the attempt never got as far as deriving one.
			// Retrying re-runs the same prune against the same builds and refuses
			// again, so this is final.
			return err
		}
		if i > 0 && build.BuildID == derived {
			// The index this attempt derived is the one the last attempt already
			// tried, so nothing took it in between: the failure is not a concurrent
			// prepare and retrying cannot help.
			return err
		}
	}
	return err
}

// createBuild is one attempt at storing a build on a transaction of its own. A
// failed attempt rolls all of it back, which is what leaves the caller free to
// retry.
func (r *DeploymentRepo) createBuild(build *model.Build, hardLimit int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := r.storeBuild(tx, build, hardLimit); err != nil {
		return err
	}
	return tx.Commit()
}

// storeBuild prunes, derives the id when the build has none, and inserts — the
// whole of adding a build, on a transaction the caller owns. Deciding a build is
// expendable and referencing one are the same judgement about what is still
// needed, so they are settled together; a deploy from the API's definition runs
// this on the transaction that records the deployment, so the build and the
// deployment that names it commit as one.
func (r *DeploymentRepo) storeBuild(tx *sql.Tx, build *model.Build, hardLimit int) error {
	if err := r.pruneBuilds(tx, build.ArtifactID, build.OrganizationID, hardLimit); err != nil {
		return err
	}
	if build.BuildID == "" {
		buildID, err := r.nextBuildID(tx, build.ArtifactID, build.OrganizationID, build.CreatedAt)
		if err != nil {
			return err
		}
		build.BuildID = buildID
	}
	return r.insertBuild(tx, build)
}

// nextBuildID returns the next unused id for an API on the given day. Reading the
// day's ids and taking the highest index — rather than counting rows — keeps the
// sequence correct even after an API's builds are pruned.
func (r *DeploymentRepo) nextBuildID(tx *sql.Tx, artifactUUID, orgUUID string, day time.Time) (string, error) {
	prefix := day.UTC().Format("2006-01-02") + "-"
	const query = `
		SELECT build_id
		FROM builds
		WHERE artifact_uuid = ? AND organization_uuid = ? AND build_id LIKE ?
	`
	rows, err := tx.Query(r.db.Rebind(query), artifactUUID, orgUUID, prefix+"%")
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
func (r *DeploymentRepo) insertBuild(tx *sql.Tx, build *model.Build) error {
	var metadataBytes []byte
	if len(build.Metadata) > 0 {
		var err error
		metadataBytes, err = json.Marshal(build.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal build metadata: %w", err)
		}
	}

	const query = `
		INSERT INTO builds (uuid, build_id, artifact_uuid, organization_uuid, description, content, data_version, metadata, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(r.db.Rebind(query),
		build.UUID, build.BuildID, build.ArtifactID, build.OrganizationID,
		build.Description, build.Content, build.DataVersion, metadataBytes, build.CreatedBy, build.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create build: %w", err)
	}
	return nil
}

// applyBuildMetadata decodes the stored metadata bag onto the model.
func applyBuildMetadata(build *model.Build, metadataBytes []byte) error {
	if len(metadataBytes) == 0 {
		return nil
	}
	var metadata map[string]any
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return fmt.Errorf("failed to unmarshal build metadata: %w", err)
	}
	build.Metadata = metadata
	return nil
}

// GetBuild returns one build of an API, including its content. Scoping by
// artifact and organization is what keeps a build id from another API — or
// another organization — resolving here.
func (r *DeploymentRepo) GetBuild(buildID, artifactUUID, orgUUID string) (*model.Build, error) {
	const query = `
		SELECT uuid, build_id, artifact_uuid, organization_uuid, description, content, data_version, metadata, created_by, created_at
		FROM builds
		WHERE build_id = ? AND artifact_uuid = ? AND organization_uuid = ?
	`
	var build model.Build
	var createdBy, description sql.NullString
	var metadataBytes []byte
	err := r.db.QueryRow(r.db.Rebind(query), buildID, artifactUUID, orgUUID).Scan(
		&build.UUID, &build.BuildID, &build.ArtifactID, &build.OrganizationID,
		&description, &build.Content, &build.DataVersion, &metadataBytes, &createdBy, &build.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get build: %w", err)
	}
	if err := applyBuildMetadata(&build, metadataBytes); err != nil {
		return nil, err
	}
	build.CreatedBy = createdBy.String
	build.Description = description.String
	return &build, nil
}

// GetBuilds lists an API's builds newest first, without their content — a
// listing is for choosing a build, and the artifacts are large.
func (r *DeploymentRepo) GetBuilds(artifactUUID, orgUUID string, limit int) ([]*model.Build, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT uuid, build_id, artifact_uuid, organization_uuid, description, data_version, metadata, created_by, created_at
		FROM builds
		WHERE artifact_uuid = ? AND organization_uuid = ?
		ORDER BY created_at DESC, build_id DESC
	`
	pageClause, pageArgs := r.db.PaginationClause(limit, 0)
	query += " " + pageClause
	args := append([]any{artifactUUID, orgUUID}, pageArgs...)

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list builds: %w", err)
	}
	defer rows.Close()

	builds := make([]*model.Build, 0)
	for rows.Next() {
		var build model.Build
		var createdBy, description sql.NullString
		var metadataBytes []byte
		if err := rows.Scan(
			&build.UUID, &build.BuildID, &build.ArtifactID, &build.OrganizationID,
			&description, &build.DataVersion, &metadataBytes, &createdBy, &build.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan build: %w", err)
		}
		if err := applyBuildMetadata(&build, metadataBytes); err != nil {
			return nil, err
		}
		build.CreatedBy = createdBy.String
		build.Description = description.String
		builds = append(builds, &build)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read builds: %w", err)
	}
	return builds, nil
}

// DeleteBuild removes one of an API's builds by its readable id.
//
// A build held by a deployment is NOT deleted (ErrBuildInUse). Deleting it would
// leave a running deployment — or a suspended one that can still be restored —
// with no snapshot to trace back to or promote onward, and there is no way to
// re-render the definition as it stood. Which deployment to give up is the
// caller's decision, so this reports the conflict instead of resolving it.
//
// Resolving the build, testing it and deleting it happen on one transaction, so a
// deploy cannot claim the build between the test and the delete.
func (r *DeploymentRepo) DeleteBuild(buildID, artifactUUID, orgUUID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const findQuery = `
		SELECT uuid
		FROM builds
		WHERE build_id = ? AND artifact_uuid = ? AND organization_uuid = ?
	`
	var buildUUID string
	if err := tx.QueryRow(r.db.Rebind(findQuery), buildID, artifactUUID, orgUUID).Scan(&buildUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrBuildNotFound
		}
		return fmt.Errorf("failed to find build %s: %w", buildID, err)
	}

	inUse, err := r.buildsInUse(tx, artifactUUID, orgUUID)
	if err != nil {
		return err
	}
	if inUse[buildUUID] {
		return ErrBuildInUse
	}

	// releaseBuild's delete is conditional on nothing referencing the build, so a
	// deploy that claimed it since the test above leaves the row in place — which is
	// the same conflict, reported the same way rather than passed off as a success.
	removed, err := r.releaseBuild(tx, buildUUID)
	if err != nil {
		return err
	}
	if !removed {
		return ErrBuildInUse
	}
	return tx.Commit()
}

// pruneBuilds makes room for one more build within hardLimit. The budget is per
// API — one API's history cannot be crowded out by another's, and unlike
// deployments a build belongs to no gateway, so there is nothing narrower to count
// by.
//
// Age alone does not decide what goes. A build is removed only when no deployment
// holds it: a build something is still running, or still suspended and restorable
// from, is exactly the one that must survive, because it is what a promotion out of
// that environment carries and what restoring that deployment sends. Age only
// orders the builds that are free to go.
//
// When nothing is free the prepare is REFUSED (ErrBuildLimitReached) rather than
// quietly letting the API keep more than its budget: the limit is what an
// organization is entitled to store, so exceeding it has to be someone's decision.
// The caller is told to free a build, which is the one thing that can be done about
// it — the alternative is deleting a build a gateway can still be restored from.
//
// It removes as many free builds as the limit demands, not a fixed batch, so a
// limit that has been lowered converges on the first prepare instead of drifting
// down one build at a time.
//
// It runs on the caller's transaction, alongside the insert it makes room for, so
// what it reads about a build being in use still holds when it deletes.
func (r *DeploymentRepo) pruneBuilds(tx *sql.Tx, artifactUUID, orgUUID string, hardLimit int) error {
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
	if err := tx.QueryRow(r.db.Rebind(countQuery), artifactUUID, orgUUID).Scan(&count); err != nil {
		return fmt.Errorf("failed to count builds: %w", err)
	}
	if count < hardLimit {
		return nil
	}
	// One slot for the build being added, plus whatever the API is over by — a
	// limit lowered since the last prepare leaves it over by more than one.
	needed := count - hardLimit + 1

	inUse, err := r.buildsInUse(tx, artifactUUID, orgUUID)
	if err != nil {
		return err
	}

	const oldestQuery = `
		SELECT uuid
		FROM builds
		WHERE artifact_uuid = ? AND organization_uuid = ?
		ORDER BY created_at ASC, build_id ASC
	`
	rows, err := tx.Query(r.db.Rebind(oldestQuery), artifactUUID, orgUUID)
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
		if len(expendable) == needed {
			break
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to read builds for cleanup: %w", err)
	}
	if len(expendable) < needed {
		return ErrBuildLimitReached
	}

	// The reference is cleared before the row goes: deployments outlive the build
	// they came from, so an archived one keeps its content and simply stops naming a
	// build it can no longer resolve.
	//
	// Both statements re-test what buildsInUse read a moment ago, because a database
	// that reads committed rows per statement lets a deploy land in between. Scoping
	// the clear to archived deployments means one that has just become current never
	// has its origin taken away, and a delete conditional on nothing referencing the
	// build means one that has just been claimed simply stays — which is why the
	// rows actually deleted are counted rather than assumed, and the prepare refused
	// if the race left the API at its limit after all.
	freed := 0
	for _, buildUUID := range expendable {
		removed, err := r.releaseBuild(tx, buildUUID)
		if err != nil {
			return err
		}
		if removed {
			freed++
		}
	}
	if count-freed >= hardLimit {
		return ErrBuildLimitReached
	}
	return nil
}

// releaseBuild clears the archived deployments that name a build and then deletes
// it, reporting whether the row actually went. Both statements are conditional on
// nothing current referencing the build, so a build claimed by a deploy since it was
// picked stays and the caller learns it was not freed.
func (r *DeploymentRepo) releaseBuild(tx *sql.Tx, buildUUID string) (bool, error) {
	const clearQuery = `
		UPDATE deployments SET build_uuid = NULL
		WHERE build_uuid = ?
			AND NOT EXISTS (
				SELECT 1 FROM deployment_status s
				WHERE s.deployment_uuid = deployments.uuid
					AND s.artifact_uuid = deployments.artifact_uuid
					AND s.organization_uuid = deployments.organization_uuid
					AND s.gateway_uuid = deployments.gateway_uuid
			)
	`
	const deleteQuery = `
		DELETE FROM builds
		WHERE uuid = ?
			AND NOT EXISTS (SELECT 1 FROM deployments d WHERE d.build_uuid = builds.uuid)
	`
	if _, err := tx.Exec(r.db.Rebind(clearQuery), buildUUID); err != nil {
		return false, fmt.Errorf("failed to clear references to build %s: %w", buildUUID, err)
	}
	res, err := tx.Exec(r.db.Rebind(deleteQuery), buildUUID)
	if err != nil {
		return false, fmt.Errorf("failed to delete build %s: %w", buildUUID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		// A driver that cannot report the count cannot be asked again; treating the
		// delete as a no-op keeps the caller's accounting conservative, so the worst
		// case is refusing a prepare that would have fit.
		return false, nil
	}
	return affected > 0, nil
}

// buildsInUse returns the builds an API's gateways are currently deployed from,
// by uuid.
//
// One deployment per gateway is current — the one deployment_status names — and its
// build_uuid says which build it came from. Only those rows count: an archived
// deployment carries its own rendered content and never needs its build back, so it
// is not a reason to keep one.
func (r *DeploymentRepo) buildsInUse(tx *sql.Tx, artifactUUID, orgUUID string) (map[string]bool, error) {
	const query = `
		SELECT DISTINCT d.build_uuid
		FROM deployments d
		JOIN deployment_status s ON d.uuid = s.deployment_uuid
			AND d.artifact_uuid = s.artifact_uuid
			AND d.organization_uuid = s.organization_uuid
			AND d.gateway_uuid = s.gateway_uuid
		WHERE d.artifact_uuid = ? AND d.organization_uuid = ? AND d.build_uuid IS NOT NULL
	`
	rows, err := tx.Query(r.db.Rebind(query), artifactUUID, orgUUID)
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

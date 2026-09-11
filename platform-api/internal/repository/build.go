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

// buildHoldRule says which deployments count as HOLDING a build, and so whose
// references keep it alive. The two callers differ on purpose.
type buildHoldRule int

const (
	// heldByAnyCurrent — a build is held while any deployment the status table
	// still names references it, whatever that status is. Automatic pruning uses
	// this. An ARCHIVED deployment (no status row) does NOT hold its build, which is
	// what keeps repeated deploys to one gateway working: each supersedes the last,
	// the superseded ones stop holding anything, and their builds become reclaimable
	// without anyone being asked. A pipeline deploying to a single gateway would
	// otherwise wedge on the first deploy past the limit.
	heldByAnyCurrent buildHoldRule = iota
	// heldByGateway — only a deployment that is on its gateway, or moving on or off
	// it, holds the build. Deleting a build uses this, so a user can also reclaim
	// the build behind a SUSPENDED or FAILED deployment — ones pruning deliberately
	// leaves alone, because suspending something is not the same as being done with
	// it.
	heldByGateway
)

// gatewayStatusFilter narrows a deployment_status join to the deployments a gateway
// is involved with right now. UNDEPLOYED and FAILED are absent on purpose: neither
// is on a gateway, so neither blocks a delete — though both still block pruning,
// which does not apply this filter.
const gatewayStatusFilter = ` AND s.status IN ('DEPLOYED', 'DEPLOYING', 'UNDEPLOYING')`

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
// A build a gateway is involved with is not deleted (ErrBuildInUse): taking the
// snapshot out from under a DEPLOYED, DEPLOYING or UNDEPLOYING deployment would
// leave it with nothing to trace back to or promote onward, and the definition as
// it stood cannot be rendered again.
//
// Everything else releases the build — SUSPENDED, FAILED, and ARCHIVED deployments
// alike. This is where the limit is actually reclaimed, and it is a request rather
// than a cleanup because of what it costs: those deployments each keep the rendered
// artifact they were created with, so they stay REDEPLOYABLE without their build,
// but they stop naming one, and so stop being something a later environment can be
// promoted from. Giving that up is the caller's call, which is why automatic
// pruning never makes it.
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

	live, err := r.buildsInUse(tx, artifactUUID, orgUUID, heldByGateway)
	if err != nil {
		return err
	}
	if live[buildUUID] {
		return ErrBuildInUse
	}

	// releaseBuild's delete is conditional on nothing referencing the build, so a
	// deploy that claimed it since the test above leaves the row in place — which is
	// the same conflict, reported the same way rather than passed off as a success.
	removed, err := r.releaseBuild(tx, buildUUID, heldByGateway)
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
// Age alone does not decide what goes. A build is removed only when no CURRENT
// deployment names it: a build something is still running, still suspended and
// redeployable, or still retryable after a failure, is one the status table points
// at, and the cleanup will not cut that link. Age only orders the builds that are
// free to go.
//
// An ARCHIVED deployment does not hold its build. That is what keeps a pipeline
// working: deploying repeatedly to one gateway supersedes the previous deployment
// each time, so the builds behind those deployments become reclaimable on their own
// and the limit is never reached by ordinary redeployment. The archived deployment
// keeps its own rendered artifact and stays redeployable; it simply stops naming a
// build, so it can no longer be promoted onward.
//
// When nothing is free the prepare is REFUSED (ErrBuildLimitReached) rather than
// quietly letting the API keep more than its budget: the limit is what an
// organization is entitled to store, so exceeding it has to be someone's decision.
// That happens when the API's builds are spread across gateways that are each
// running or holding one, and the remedy is to delete a build (DeleteBuild), which
// can also reclaim the ones behind suspended and failed deployments that pruning
// leaves alone.
//
// It removes as many free builds as the limit demands, not a fixed batch, so a
// limit that has been lowered converges on the first prepare.
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

	inUse, err := r.buildsInUse(tx, artifactUUID, orgUUID, heldByAnyCurrent)
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
		removed, err := r.releaseBuild(tx, buildUUID, heldByAnyCurrent)
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

// releaseBuild deletes a build, first clearing the references held by deployments
// that do not stand in its way.
//
// Deployments outlive the build they came from: an archived one — and, for a
// delete, a suspended or failed one — keeps the rendered artifact it was created
// with, so it stays redeployable and simply stops naming a build it can no longer
// resolve. The scope of what gets cleared is exactly the complement of the caller's
// hold rule, so pruning never takes a build from a deployment the status table
// still names.
//
// The DELETE re-tests that nothing references the build, because a database that
// reads committed rows per statement lets a deploy land between the caller's check
// and this one. A build claimed in that window simply stays, and the caller is told
// it was not freed rather than having the claim silently broken.
func (r *DeploymentRepo) releaseBuild(tx *sql.Tx, buildUUID string, rule buildHoldRule) (bool, error) {
	clearQuery := `
		UPDATE deployments SET build_uuid = NULL
		WHERE build_uuid = ?
			AND NOT EXISTS (
				SELECT 1 FROM deployment_status s
				WHERE s.deployment_uuid = deployments.uuid
					AND s.artifact_uuid = deployments.artifact_uuid
					AND s.organization_uuid = deployments.organization_uuid
					AND s.gateway_uuid = deployments.gateway_uuid`
	if rule == heldByGateway {
		clearQuery += gatewayStatusFilter
	}
	clearQuery += `
			)
	`
	if _, err := tx.Exec(r.db.Rebind(clearQuery), buildUUID); err != nil {
		return false, fmt.Errorf("failed to clear references to build %s: %w", buildUUID, err)
	}

	const deleteQuery = `
		DELETE FROM builds
		WHERE uuid = ?
			AND NOT EXISTS (SELECT 1 FROM deployments d WHERE d.build_uuid = builds.uuid)
	`
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

// buildsInUse returns the builds of an API that are held under the given rule, by
// uuid.
//
// Both rules join the status table, so an archived deployment never holds a build
// either way. heldByGateway narrows further to the deployments a gateway is
// involved with, which is what lets a delete reclaim a suspended or failed
// deployment's build while pruning leaves it alone.
func (r *DeploymentRepo) buildsInUse(tx *sql.Tx, artifactUUID, orgUUID string,
	rule buildHoldRule) (map[string]bool, error) {
	query := `
		SELECT DISTINCT d.build_uuid
		FROM deployments d
		JOIN deployment_status s ON d.uuid = s.deployment_uuid
			AND d.artifact_uuid = s.artifact_uuid
			AND d.organization_uuid = s.organization_uuid
			AND d.gateway_uuid = s.gateway_uuid
		WHERE d.artifact_uuid = ? AND d.organization_uuid = ? AND d.build_uuid IS NOT NULL
	`
	if rule == heldByGateway {
		query += gatewayStatusFilter
	}
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

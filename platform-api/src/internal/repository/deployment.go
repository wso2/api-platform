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

	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"
)

// DeploymentRepo implements DeploymentRepository
type DeploymentRepo struct {
	db *database.DB
}

// NewDeploymentRepo creates a new deployment repository
func NewDeploymentRepo(db *database.DB) DeploymentRepository {
	return &DeploymentRepo{
		db: db,
	}
}

// CreateWithLimitEnforcement atomically creates a deployment with hard limit enforcement
// If deployment count >= hardLimit, deletes oldest 5 ARCHIVED deployments before inserting new one
// This entire operation is wrapped in a single transaction to ensure atomicity
// and to leverage row-level locks during deletion to reduce race conditions.
func (r *DeploymentRepo) CreateWithLimitEnforcement(deployment *model.Deployment, hardLimit int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Generate UUID for deployment if not already set
	if deployment.DeploymentID == "" {
		deploymentID, err := utils.GenerateUUID()
		if err != nil {
			return fmt.Errorf("failed to generate deployment ID: %w", err)
		}
		deployment.DeploymentID = deploymentID
	}
	deployment.CreatedAt = time.Now()

	// Status must be provided and should be DEPLOYED for new deployments
	if deployment.Status == nil {
		deployed := model.DeploymentStatusDeployed
		deployment.Status = &deployed
	}

	updatedAt := time.Now()
	deployment.UpdatedAt = &updatedAt

	// 1. Count total deployments for this artifact+Gateway
	var count int
	countQuery := `
		SELECT COUNT(*)
		FROM deployments
		WHERE artifact_uuid = ? AND gateway_uuid = ? AND organization_uuid = ?
	`
	err = tx.QueryRow(r.db.Rebind(countQuery), deployment.ArtifactID, deployment.GatewayID, deployment.OrganizationID).Scan(&count)
	if err != nil {
		return err
	}

	// 2. If at/over hard limit, delete oldest 5 ARCHIVED deployments
	if count >= hardLimit {
		// Get oldest 5 ARCHIVED deployment IDs (LEFT JOIN WHERE status IS NULL)
		getOldestQuery := `
			SELECT d.uuid
			FROM deployments d
			LEFT JOIN deployment_status s ON d.uuid = s.deployment_uuid
				AND d.artifact_uuid = s.artifact_uuid
				AND d.organization_uuid = s.organization_uuid
				AND d.gateway_uuid = s.gateway_uuid
			WHERE d.artifact_uuid = ? AND d.gateway_uuid = ? AND d.organization_uuid = ?
				AND s.deployment_uuid IS NULL
			ORDER BY d.created_at ASC
			` + r.db.FetchFirstClause(5) + `
		`

		rows, err := tx.Query(r.db.Rebind(getOldestQuery), deployment.ArtifactID, deployment.GatewayID, deployment.OrganizationID)
		if err != nil {
			return err
		}

		var idsToDelete []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			idsToDelete = append(idsToDelete, id)
		}
		rows.Close()

		// Check for iteration errors
		if err := rows.Err(); err != nil {
			return err
		}

		// Delete one-by-one to use row-level locks (prevents over-deletion in concurrent scenarios)
		deleteQuery := `DELETE FROM deployments WHERE uuid = ?`
		for _, id := range idsToDelete {
			_, err := tx.Exec(r.db.Rebind(deleteQuery), id)
			if err != nil {
				return err
			}
		}
	}

	// 3. Insert new deployment artifact
	deploymentQuery := `
		INSERT INTO deployments (uuid, name, artifact_uuid, organization_uuid, gateway_uuid, base_deployment_uuid, content, metadata, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var baseDeploymentID interface{}
	if deployment.BaseDeploymentID != nil {
		baseDeploymentID = *deployment.BaseDeploymentID
	}

	var metadataBytes []byte
	if len(deployment.Metadata) > 0 {
		var err error
		metadataBytes, err = json.Marshal(deployment.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal deployment metadata: %w", err)
		}
	}

	_, err = tx.Exec(r.db.Rebind(deploymentQuery), deployment.DeploymentID, deployment.Name, deployment.ArtifactID, deployment.OrganizationID,
		deployment.GatewayID, baseDeploymentID, deployment.Content, metadataBytes, deployment.CreatedBy, deployment.CreatedAt)
	if err != nil {
		return err
	}

	// 4. Insert or update deployment status (UPSERT)
	statusQuery := r.db.BuildUpsertQuery(
		"deployment_status",
		[]string{"artifact_uuid", "organization_uuid", "gateway_uuid", "deployment_uuid", "status", "status_desired", "performed_at", "status_reason", "updated_at"},
		[]string{"artifact_uuid", "organization_uuid", "gateway_uuid"},
		[]string{"deployment_uuid", "status", "status_desired", "performed_at", "status_reason=NULL", "updated_at"},
	)

	// Status and UpdatedAt are guaranteed to be non-nil by initialization at function start
	_, err = tx.Exec(r.db.Rebind(statusQuery),
		deployment.ArtifactID,
		deployment.OrganizationID,
		deployment.GatewayID,
		deployment.DeploymentID,
		*deployment.Status,
		string(*deployment.Status),
		*deployment.UpdatedAt,
		nil,
		*deployment.UpdatedAt,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// applyDeploymentBase populates the nullable base fields shared by all deployment scan paths.
func applyDeploymentBase(d *model.Deployment, baseID sql.NullString, createdBy sql.NullString, metadataBytes []byte) error {
	if baseID.Valid {
		d.BaseDeploymentID = &baseID.String
	}
	if createdBy.Valid {
		d.CreatedBy = createdBy.String
	}
	if len(metadataBytes) > 0 {
		var metadata map[string]interface{}
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return fmt.Errorf("failed to unmarshal deployment metadata: %w", err)
		}
		d.Metadata = metadata
	}
	return nil
}

// applyDeploymentStatus maps the LEFT-JOIN status columns onto the deployment model.
// A NULL statusStr means the deployment is ARCHIVED (no active status row).
func applyDeploymentStatus(d *model.Deployment, statusStr sql.NullString, updatedAt sql.NullTime, statusReason sql.NullString) {
	if statusStr.Valid {
		st := model.DeploymentStatus(statusStr.String)
		d.Status = &st
		if updatedAt.Valid {
			d.UpdatedAt = &updatedAt.Time
		}
		if statusReason.Valid && statusReason.String != "" {
			d.StatusReason = &statusReason.String
		}
	} else {
		archived := model.DeploymentStatusArchived
		d.Status = &archived
	}
}

// GetWithContent retrieves a deployment including its content (for rollback/base deployment scenarios)
func (r *DeploymentRepo) GetWithContent(deploymentID, artifactUUID, orgUUID string) (*model.Deployment, error) {
	deployment := &model.Deployment{}

	query := `
		SELECT uuid, name, artifact_uuid, organization_uuid, gateway_uuid, base_deployment_uuid, content, metadata, created_by, created_at
		FROM deployments
		WHERE uuid = ? AND artifact_uuid = ? AND organization_uuid = ?
	`

	var baseDeploymentID sql.NullString
	var metadataBytes []byte
	var createdBy sql.NullString

	err := r.db.QueryRow(r.db.Rebind(query), deploymentID, artifactUUID, orgUUID).Scan(
		&deployment.DeploymentID, &deployment.Name, &deployment.ArtifactID, &deployment.OrganizationID,
		&deployment.GatewayID, &baseDeploymentID, &deployment.Content, &metadataBytes, &createdBy, &deployment.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrDeploymentNotFound
		}
		return nil, err
	}

	if err := applyDeploymentBase(deployment, baseDeploymentID, createdBy, metadataBytes); err != nil {
		return nil, err
	}
	return deployment, nil
}

// Delete deletes a deployment record
func (r *DeploymentRepo) Delete(deploymentID, artifactUUID, orgUUID string) error {
	query := `DELETE FROM deployments WHERE uuid = ? AND artifact_uuid = ? AND organization_uuid = ?`

	result, err := r.db.Exec(r.db.Rebind(query), deploymentID, artifactUUID, orgUUID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return constants.ErrDeploymentNotFound
	}

	return nil
}

// GetCurrentByGateway retrieves the currently DEPLOYED deployment for an artifact on a gateway
// Returns only deployments with DEPLOYED status (filters out UNDEPLOYED/suspended deployments)
func (r *DeploymentRepo) GetCurrentByGateway(artifactUUID, gatewayID, orgUUID string) (*model.Deployment, error) {
	deployment := &model.Deployment{}

	query := `
		SELECT
			d.uuid, d.name, d.artifact_uuid, d.organization_uuid, d.gateway_uuid,
			d.base_deployment_uuid, d.content, d.metadata, d.created_by, d.created_at,
			s.status, s.updated_at AS status_updated_at
		FROM deployments d
		INNER JOIN deployment_status s
			ON d.uuid = s.deployment_uuid
			AND d.artifact_uuid = s.artifact_uuid
			AND d.organization_uuid = s.organization_uuid
			AND d.gateway_uuid = s.gateway_uuid
		WHERE d.artifact_uuid = ? AND d.gateway_uuid = ? AND d.organization_uuid = ?
			AND s.status_desired = 'DEPLOYED'
		ORDER BY d.created_at DESC
		` + r.db.FetchFirstClause(1) + `
	`

	var baseDeploymentID sql.NullString
	var metadataBytes []byte
	var createdBy sql.NullString
	var statusStr string
	var updatedAt time.Time

	err := r.db.QueryRow(r.db.Rebind(query), artifactUUID, gatewayID, orgUUID).Scan(
		&deployment.DeploymentID, &deployment.Name, &deployment.ArtifactID, &deployment.OrganizationID,
		&deployment.GatewayID, &baseDeploymentID, &deployment.Content, &metadataBytes, &createdBy, &deployment.CreatedAt,
		&statusStr, &updatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if err := applyDeploymentBase(deployment, baseDeploymentID, createdBy, metadataBytes); err != nil {
		return nil, err
	}
	status := model.DeploymentStatus(statusStr)
	deployment.Status = &status
	deployment.UpdatedAt = &updatedAt

	return deployment, nil
}

// SetCurrent inserts or updates the deployment status record to set the current deployment for an artifact on a gateway
func (r *DeploymentRepo) SetCurrent(artifactUUID, orgUUID, gatewayID, deploymentID string, status model.DeploymentStatus) (time.Time, error) {
	return r.SetCurrentWithDetails(artifactUUID, orgUUID, gatewayID, deploymentID, status, "", nil, "")
}

// SetCurrentWithDetails inserts or updates the deployment status record with full lifecycle fields.
// statusDesired is the user's intended final state (DEPLOYED/UNDEPLOYED).
// performedAt, if non-nil, is used as the concurrency token; otherwise defaults to now.
// statusReason is an optional error code (cleared on new deployments).
// Also maintains artifact_secret_refs (gateway_id rows): inserts refs on DEPLOYED, deletes them otherwise.
func (r *DeploymentRepo) SetCurrentWithDetails(artifactUUID, orgUUID, gatewayID, deploymentID string, status model.DeploymentStatus, statusDesired string, performedAt *time.Time, statusReason string) (time.Time, error) {
	updatedAt := time.Now()
	var pat time.Time
	if performedAt != nil {
		pat = *performedAt
	} else {
		pat = updatedAt
	}

	if statusDesired == "" {
		statusDesired = string(status)
	}

	var reasonVal interface{}
	if statusReason != "" {
		reasonVal = statusReason
	}

	tx, err := r.db.Begin()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := r.db.BuildUpsertQuery(
		"deployment_status",
		[]string{"artifact_uuid", "organization_uuid", "gateway_uuid", "deployment_uuid", "status", "status_desired", "performed_at", "status_reason", "updated_at"},
		[]string{"artifact_uuid", "organization_uuid", "gateway_uuid"},
		[]string{"deployment_uuid", "status", "status_desired", "performed_at", "status_reason", "updated_at"},
	)
	_, err = tx.Exec(r.db.Rebind(query),
		artifactUUID, orgUUID, gatewayID, deploymentID, status, statusDesired, pat, reasonVal, updatedAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to upsert deployment status: %w", err)
	}

	// Maintain gateway-specific secret refs from the deployment snapshot.
	// On DEPLOYED: derive handles from snapshot and insert; on UNDEPLOYED/ARCHIVED: clear only.
	var content []byte
	if status == model.DeploymentStatusDeployed {
		err = tx.QueryRow(r.db.Rebind(`SELECT content FROM deployments WHERE deployment_id = ?`), deploymentID).Scan(&content)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, fmt.Errorf("failed to fetch deployment content for secret refs: %w", err)
		}
	}
	if err := upsertDeploymentSecretRefs(tx, r.db, orgUUID, artifactUUID, gatewayID, content); err != nil {
		return time.Time{}, fmt.Errorf("failed to upsert deployment secret refs: %w", err)
	}

	return updatedAt, tx.Commit()
}

// GetStatus retrieves the current deployment status for an artifact on a gateway (lightweight - no content)
func (r *DeploymentRepo) GetStatus(artifactUUID, orgUUID, gatewayID string) (string, model.DeploymentStatus, *time.Time, error) {
	query := `
		SELECT deployment_uuid, status, updated_at
		FROM deployment_status
		WHERE artifact_uuid = ? AND organization_uuid = ? AND gateway_uuid = ?
	`

	var deploymentID string
	var statusStr string
	var updatedAt time.Time

	err := r.db.QueryRow(r.db.Rebind(query), artifactUUID, orgUUID, gatewayID).Scan(
		&deploymentID, &statusStr, &updatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No status row means no active deployment (all ARCHIVED)
			return "", "", nil, nil
		}
		return "", "", nil, err
	}

	return deploymentID, model.DeploymentStatus(statusStr), &updatedAt, nil
}

// GetStatusFull retrieves the full deployment status including performed_at and status_reason
func (r *DeploymentRepo) GetStatusFull(artifactUUID, orgUUID, gatewayID string) (deploymentID string, status model.DeploymentStatus, performedAt *time.Time, statusReason string, err error) {
	query := `
		SELECT deployment_uuid, status, performed_at, COALESCE(status_reason, '')
		FROM deployment_status
		WHERE artifact_uuid = ? AND organization_uuid = ? AND gateway_uuid = ?
	`

	var patNull sql.NullTime
	err = r.db.QueryRow(r.db.Rebind(query), artifactUUID, orgUUID, gatewayID).Scan(
		&deploymentID, &status, &patNull, &statusReason)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", nil, "", nil
		}
		return "", "", nil, "", err
	}

	var performedAtPtr *time.Time
	if patNull.Valid {
		t := patNull.Time
		performedAtPtr = &t
	}
	return deploymentID, status, performedAtPtr, statusReason, nil
}

// UpdateStatusWithPerformedAtGuard conditionally updates the deployment status only if performed_at matches.
// Returns the number of rows affected (0 means stale ack was discarded).
func (r *DeploymentRepo) UpdateStatusWithPerformedAtGuard(artifactUUID, orgUUID, gatewayID string, newStatus model.DeploymentStatus, statusReason string, performedAt time.Time, requireCurrentStatus []model.DeploymentStatus) (int64, error) {
	var reasonVal interface{}
	if statusReason != "" {
		reasonVal = statusReason
	}

	updatedAt := time.Now()

	if len(requireCurrentStatus) > 0 {
		placeholders := make([]string, len(requireCurrentStatus))
		args := []interface{}{newStatus, reasonVal, updatedAt}
		for i, s := range requireCurrentStatus {
			placeholders[i] = "?"
			args = append(args, s)
		}
		args = append(args, artifactUUID, orgUUID, gatewayID, performedAt)

		query := fmt.Sprintf(`
			UPDATE deployment_status
			SET status = ?, status_reason = ?, updated_at = ?
			WHERE status IN (%s)
			  AND artifact_uuid = ? AND organization_uuid = ? AND gateway_uuid = ?
			  AND performed_at = ?
		`, strings.Join(placeholders, ", "))

		result, err := r.db.Exec(r.db.Rebind(query), args...)
		if err != nil {
			return 0, err
		}
		return result.RowsAffected()
	}

	// No status filter — used for failure acks that can overwrite any status
	query := `
		UPDATE deployment_status
		SET status = ?, status_reason = ?, updated_at = ?
		WHERE artifact_uuid = ? AND organization_uuid = ? AND gateway_uuid = ?
		  AND performed_at = ?
	`
	result, err := r.db.Exec(r.db.Rebind(query), newStatus, reasonVal, updatedAt, artifactUUID, orgUUID, gatewayID, performedAt)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetStaleTransitionalStatuses finds deployment_status rows stuck in DEPLOYING/UNDEPLOYING
// for longer than the given timeout duration.
func (r *DeploymentRepo) GetStaleTransitionalStatuses(timeout time.Duration) ([]StaleDeploymentStatus, error) {
	cutoff := time.Now().Add(-timeout)
	query := `
		SELECT artifact_uuid, organization_uuid, gateway_uuid, deployment_uuid, status, status_desired, performed_at
		FROM deployment_status
		WHERE status IN ('DEPLOYING', 'UNDEPLOYING')
		  AND performed_at < ?
	`
	rows, err := r.db.Query(r.db.Rebind(query), cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []StaleDeploymentStatus
	for rows.Next() {
		var s StaleDeploymentStatus
		var statusDesired sql.NullString
		if err := rows.Scan(&s.ArtifactUUID, &s.OrganizationUUID, &s.GatewayUUID, &s.DeploymentID, &s.Status, &statusDesired, &s.PerformedAt); err != nil {
			return nil, err
		}
		s.StatusDesired = statusDesired.String
		results = append(results, s)
	}
	return results, rows.Err()
}

// StaleDeploymentStatus represents a deployment status row that has been in a transitional state too long
type StaleDeploymentStatus struct {
	ArtifactUUID     string
	OrganizationUUID string
	GatewayUUID      string
	DeploymentID     string
	Status           model.DeploymentStatus
	StatusDesired    string
	PerformedAt      time.Time
}

// GetArtifactUUIDByDeploymentID resolves the artifact UUID for a given deployment ID.
// Used to normalise ack handling where the gateway may send a string handle instead of a UUID.
func (r *DeploymentRepo) GetArtifactUUIDByDeploymentID(deploymentID, orgUUID string) (string, error) {
	var artifactUUID string
	err := r.db.QueryRow(r.db.Rebind(`
		SELECT artifact_uuid FROM deployments
		WHERE uuid = ? AND organization_uuid = ?
	`), deploymentID, orgUUID).Scan(&artifactUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return artifactUUID, nil
}

// DeleteStatus deletes the status entry for an artifact on a gateway
func (r *DeploymentRepo) DeleteStatus(artifactUUID, orgUUID, gatewayID string) error {
	query := `
		DELETE FROM deployment_status
		WHERE artifact_uuid = ? AND organization_uuid = ? AND gateway_uuid = ?
	`

	_, err := r.db.Exec(r.db.Rebind(query), artifactUUID, orgUUID, gatewayID)
	return err
}

// GetWithState retrieves a deployment with its lifecycle state populated (without content - lightweight)
func (r *DeploymentRepo) GetWithState(deploymentID, artifactUUID, orgUUID string) (*model.Deployment, error) {
	deployment := &model.Deployment{}

	query := `
		SELECT
			d.uuid, d.name, d.artifact_uuid, d.organization_uuid, d.gateway_uuid,
			d.base_deployment_uuid, d.metadata, d.created_by, d.created_at,
			s.status, s.updated_at AS status_updated_at, s.status_reason
		FROM deployments d
		LEFT JOIN deployment_status s
			ON d.uuid = s.deployment_uuid
			AND d.artifact_uuid = s.artifact_uuid
			AND d.organization_uuid = s.organization_uuid
			AND d.gateway_uuid = s.gateway_uuid
		WHERE d.uuid = ? AND d.artifact_uuid = ? AND d.organization_uuid = ?
	`

	var baseDeploymentID sql.NullString
	var metadataBytes []byte
	var createdBy sql.NullString
	var statusStr sql.NullString
	var updatedAtVal sql.NullTime
	var statusReasonStr sql.NullString

	err := r.db.QueryRow(r.db.Rebind(query), deploymentID, artifactUUID, orgUUID).Scan(
		&deployment.DeploymentID, &deployment.Name, &deployment.ArtifactID, &deployment.OrganizationID, &deployment.GatewayID,
		&baseDeploymentID, &metadataBytes, &createdBy, &deployment.CreatedAt,
		&statusStr, &updatedAtVal, &statusReasonStr)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrDeploymentNotFound
		}
		return nil, err
	}

	if err := applyDeploymentBase(deployment, baseDeploymentID, createdBy, metadataBytes); err != nil {
		return nil, err
	}
	applyDeploymentStatus(deployment, statusStr, updatedAtVal, statusReasonStr)
	return deployment, nil
}

// GetDeploymentsWithState retrieves deployments with their lifecycle states.
// It enforces a soft limit of N records per Gateway, ensuring that the
// currently DEPLOYED or UNDEPLOYED record is always included regardless of its age.
func (r *DeploymentRepo) GetDeploymentsWithState(artifactUUID, orgUUID string, gatewayID *string, status *string, maxPerAPIGW int) ([]*model.Deployment, error) {

	// 1. Validation Logic
	if status != nil {
		validStatuses := map[string]bool{
			string(model.DeploymentStatusDeployed):    true,
			string(model.DeploymentStatusUndeployed):  true,
			string(model.DeploymentStatusDeploying):   true,
			string(model.DeploymentStatusUndeploying): true,
			string(model.DeploymentStatusFailed):      true,
			string(model.DeploymentStatusArchived):    true,
		}
		if !validStatuses[*status] {
			return nil, fmt.Errorf("invalid deployment status: %s", *status)
		}
	}

	var args []interface{}

	// 2. Build the CTE (Common Table Expression)
	// We rank within the CTE to ensure each Gateway gets its own "Top N" bucket.
	// Order Priority:
	//   1. Records with an active status (Deployed/Undeployed)
	//   2. Creation date (Newest first)
	query := `
        WITH AnnotatedDeployments AS (
            SELECT
				d.uuid, d.name, d.artifact_uuid, d.organization_uuid, d.gateway_uuid,
                d.base_deployment_uuid, d.metadata, d.created_by, d.created_at,
                s.status as current_status,
                s.updated_at as status_updated_at,
                s.status_reason,
                ROW_NUMBER() OVER (
                    PARTITION BY d.gateway_uuid
                    ORDER BY
                        (CASE WHEN s.status IS NOT NULL THEN 0 ELSE 1 END) ASC,
                        d.created_at DESC
                ) as rank_idx
			FROM deployments d
			LEFT JOIN deployment_status s
                ON d.uuid = s.deployment_uuid
                AND d.gateway_uuid = s.gateway_uuid
				AND d.artifact_uuid = s.artifact_uuid
				AND d.organization_uuid = s.organization_uuid
			WHERE d.artifact_uuid = ? AND d.organization_uuid = ?
    `
	args = append(args, artifactUUID, orgUUID)

	if gatewayID != nil {
		query += " AND d.gateway_uuid = ?"
		args = append(args, *gatewayID)
	}

	// 3. Close CTE and start Outer Selection
	query += `
        )
        SELECT
			uuid, name, artifact_uuid, organization_uuid, gateway_uuid,
            base_deployment_uuid, metadata, created_by, created_at,
            current_status, status_updated_at, status_reason
        FROM AnnotatedDeployments
        WHERE rank_idx <= ?
    `
	args = append(args, maxPerAPIGW)

	// 4. Apply Status Filters on the Ranked Set
	if status != nil {
		if *status == string(model.DeploymentStatusArchived) {
			// ARCHIVED means no entry exists in the status table for this artifact
			query += " AND current_status IS NULL"
		} else {
			// DEPLOYED or UNDEPLOYED must match the status column exactly
			query += " AND current_status = ?"
			args = append(args, *status)
		}
	}

	// Final sorting for the application layer
	query += " ORDER BY gateway_uuid ASC, rank_idx ASC"

	// 5. Execution
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []*model.Deployment
	for rows.Next() {
		deployment := &model.Deployment{}
		var baseDeploymentID sql.NullString
		var metadataBytes []byte
		var createdBy sql.NullString
		var statusStr sql.NullString
		var updatedAtVal sql.NullTime
		var statusReasonStr sql.NullString

		if err := rows.Scan(
			&deployment.DeploymentID, &deployment.Name, &deployment.ArtifactID,
			&deployment.OrganizationID, &deployment.GatewayID,
			&baseDeploymentID, &metadataBytes, &createdBy, &deployment.CreatedAt,
			&statusStr, &updatedAtVal, &statusReasonStr); err != nil {
			return nil, err
		}

		if err := applyDeploymentBase(deployment, baseDeploymentID, createdBy, metadataBytes); err != nil {
			return nil, err
		}
		applyDeploymentStatus(deployment, statusStr, updatedAtVal, statusReasonStr)
		deployments = append(deployments, deployment)
	}

	// Check if the loop stopped because of an error rather than reaching the end
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during deployment rows iteration: %w", err)
	}

	return deployments, nil
}

// GetDeployedGatewayIDs returns the gateway IDs that have an active deployment status
// (DEPLOYED or UNDEPLOYED) for the given artifact. Since the deployment_status table
// only holds rows for those two states, a plain SELECT is sufficient.
func (r *DeploymentRepo) GetDeployedGatewayIDs(artifactUUID, orgUUID string) ([]string, error) {
	query := `SELECT gateway_uuid FROM deployment_status WHERE artifact_uuid = ? AND organization_uuid = ?`

	rows, err := r.db.Query(r.db.Rebind(query), artifactUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gatewayIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		gatewayIDs = append(gatewayIDs, id)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating gateway IDs: %w", err)
	}

	return gatewayIDs, nil
}

// GetLiveGatewayIDs returns the gateways on which the artifact is currently live, i.e.
// deployed or in a transitional state where it is still present on the gateway. Fully
// undeployed/archived/failed gateways are excluded.
func (r *DeploymentRepo) GetLiveGatewayIDs(artifactUUID, orgUUID string) ([]string, error) {
	query := `
		SELECT gateway_uuid FROM deployment_status
		WHERE artifact_uuid = ? AND organization_uuid = ? AND status IN (?, ?, ?)`

	rows, err := r.db.Query(r.db.Rebind(query), artifactUUID, orgUUID,
		string(model.DeploymentStatusDeployed),
		string(model.DeploymentStatusDeploying),
		string(model.DeploymentStatusUndeploying),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gatewayIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		gatewayIDs = append(gatewayIDs, id)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating live gateway IDs: %w", err)
	}

	return gatewayIDs, nil
}

// GetAllDeploymentsByGateway retrieves all deployments for a specific gateway
// Returns lightweight DeploymentInfo for listing deployments
// Only returns deployments that have an active status (DEPLOYED or UNDEPLOYED)
// Results are ordered by kind (RestApi -> LlmProvider -> LlmProxy -> Mcp) to ensure
// dependencies are processed in correct order (LLM Proxies depend on LLM Providers)
// If since is provided, only returns deployments updated after that timestamp
func (r *DeploymentRepo) GetAllDeploymentsByGateway(gatewayID, orgUUID string, since *time.Time) ([]*model.DeploymentInfo, error) {
	query := `
		SELECT
			s.deployment_uuid,
			s.artifact_uuid,
			src.handle,
			a.type,
			s.status,
			s.performed_at
		FROM deployment_status s
		INNER JOIN artifacts a ON s.artifact_uuid = a.uuid
		INNER JOIN (
			SELECT uuid, handle FROM rest_apis
			UNION ALL SELECT uuid, handle FROM websub_apis
			UNION ALL SELECT uuid, handle FROM webbroker_apis
			UNION ALL SELECT uuid, handle FROM llm_providers
			UNION ALL SELECT uuid, handle FROM llm_proxies
			UNION ALL SELECT uuid, handle FROM mcp_proxies
		) src ON src.uuid = s.artifact_uuid
		WHERE s.gateway_uuid = ? AND s.organization_uuid = ?`
	args := []interface{}{gatewayID, orgUUID}

	if since != nil {
		query += " AND s.performed_at > ?"
		args = append(args, *since)
	}

	query += `
		ORDER BY
			CASE a.type
				WHEN 'RestApi' THEN 1
				WHEN 'LlmProvider' THEN 2
				WHEN 'LlmProxy' THEN 3
				WHEN 'Mcp' THEN 4
				ELSE 5
			END,
			s.performed_at DESC`

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []*model.DeploymentInfo
	for rows.Next() {
		dep := &model.DeploymentInfo{}
		var statusStr string

		err := rows.Scan(
			&dep.DeploymentID,
			&dep.ArtifactID,
			&dep.Handle,
			&dep.Type,
			&statusStr,
			&dep.PerformedAt,
		)
		if err != nil {
			return nil, err
		}

		dep.Status = model.DeploymentStatus(statusStr)
		deployments = append(deployments, dep)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during deployment info iteration: %w", err)
	}

	return deployments, nil
}

// GetDeploymentContentByIDs retrieves deployment content for a batch of deployment IDs.
// Returns a map of deploymentID -> DeploymentContent.
// This is the only query that joins with the artifacts table to fetch Kind,
// keeping that concern isolated to the batch sync use case.
func (r *DeploymentRepo) GetDeploymentContentByIDs(deploymentIDs []string, orgUUID string, gatewayUUID string) (map[string]*model.DeploymentContent, error) {
	if len(deploymentIDs) == 0 {
		return map[string]*model.DeploymentContent{}, nil
	}

	placeholders := make([]string, len(deploymentIDs))
	args := make([]interface{}, len(deploymentIDs)+2)
	for i, id := range deploymentIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	args[len(deploymentIDs)] = orgUUID
	args[len(deploymentIDs)+1] = gatewayUUID

	query := fmt.Sprintf(`
		SELECT d.uuid, d.artifact_uuid, a.type, d.content
		FROM deployments d
		INNER JOIN artifacts a ON d.artifact_uuid = a.uuid
		WHERE d.uuid IN (%s) AND d.organization_uuid = ? AND d.gateway_uuid = ?
	`, strings.Join(placeholders, ","))

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]*model.DeploymentContent)
	for rows.Next() {
		dc := &model.DeploymentContent{}
		if err := rows.Scan(&dc.DeploymentID, &dc.ArtifactID, &dc.Type, &dc.Content); err != nil {
			return nil, err
		}
		result[dc.DeploymentID] = dc
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during deployment content iteration: %w", err)
	}

	return result, nil
}

// GetSecretHandlesByGateway returns the distinct secret handles referenced by all
// artifacts currently deployed on the gateway. Sourced from artifact_secret_refs
// where gateway_id matches — maintained at deploy/undeploy time.
func (r *DeploymentRepo) GetSecretHandlesByGateway(gatewayID, orgUUID string) ([]string, error) {
	query := r.db.Rebind(`
		SELECT DISTINCT secret_handle
		FROM artifact_secret_refs
		WHERE organization_uuid = ? AND gateway_id = ?
	`)

	rows, err := r.db.Query(query, orgUUID, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to query secret handles for gateway %s: %w", gatewayID, err)
	}
	defer rows.Close()

	var handles []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, fmt.Errorf("failed to scan secret handle row: %w", err)
		}
		handles = append(handles, h)
	}
	return handles, rows.Err()
}

// joinStrings joins strings with a separator (helper for building IN clauses)
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

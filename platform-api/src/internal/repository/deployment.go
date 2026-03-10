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
			SELECT d.deployment_id
			FROM deployments d
			LEFT JOIN deployment_status s ON d.deployment_id = s.deployment_id
				AND d.artifact_uuid = s.artifact_uuid
				AND d.organization_uuid = s.organization_uuid
				AND d.gateway_uuid = s.gateway_uuid
			WHERE d.artifact_uuid = ? AND d.gateway_uuid = ? AND d.organization_uuid = ?
				AND s.deployment_id IS NULL
			ORDER BY d.created_at ASC
			LIMIT 5
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
		deleteQuery := `DELETE FROM deployments WHERE deployment_id = ?`
		for _, id := range idsToDelete {
			_, err := tx.Exec(r.db.Rebind(deleteQuery), id)
			if err != nil {
				return err
			}
		}
	}

	// 3. Insert new deployment artifact
	deploymentQuery := `
		INSERT INTO deployments (deployment_id, name, artifact_uuid, organization_uuid, gateway_uuid, base_deployment_id, content, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var baseDeploymentID interface{}
	if deployment.BaseDeploymentID != nil {
		baseDeploymentID = *deployment.BaseDeploymentID
	}

	var metadataJSON string
	if len(deployment.Metadata) > 0 {
		metadataBytes, err := json.Marshal(deployment.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal deployment metadata: %w", err)
		}
		metadataJSON = string(metadataBytes)
	}

	_, err = tx.Exec(r.db.Rebind(deploymentQuery), deployment.DeploymentID, deployment.Name, deployment.ArtifactID, deployment.OrganizationID,
		deployment.GatewayID, baseDeploymentID, deployment.Content, metadataJSON, deployment.CreatedAt)
	if err != nil {
		return err
	}

	// 4. Insert or update deployment status (UPSERT)
	var statusQuery string
	if r.db.Driver() == "postgres" || r.db.Driver() == "postgresql" {
		statusQuery = `
			INSERT INTO deployment_status (artifact_uuid, organization_uuid, gateway_uuid, deployment_id, status, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (artifact_uuid, organization_uuid, gateway_uuid)
			DO UPDATE SET deployment_id = EXCLUDED.deployment_id, status = EXCLUDED.status, updated_at = EXCLUDED.updated_at
		`
	} else {
		statusQuery = `
			REPLACE INTO deployment_status (artifact_uuid, organization_uuid, gateway_uuid, deployment_id, status, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`
	}

	// Status and UpdatedAt are guaranteed to be non-nil by initialization at function start
	_, err = tx.Exec(r.db.Rebind(statusQuery),
		deployment.ArtifactID,
		deployment.OrganizationID,
		deployment.GatewayID,
		deployment.DeploymentID,
		*deployment.Status,
		*deployment.UpdatedAt,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetWithContent retrieves a deployment including its content (for rollback/base deployment scenarios)
func (r *DeploymentRepo) GetWithContent(deploymentID, artifactUUID, orgUUID string) (*model.Deployment, error) {
	deployment := &model.Deployment{}

	query := `
		SELECT deployment_id, name, artifact_uuid, organization_uuid, gateway_uuid, base_deployment_id, content, metadata, created_at
		FROM deployments
		WHERE deployment_id = ? AND artifact_uuid = ? AND organization_uuid = ?
	`

	var baseDeploymentID sql.NullString
	var metadataJSON string

	err := r.db.QueryRow(r.db.Rebind(query), deploymentID, artifactUUID, orgUUID).Scan(
		&deployment.DeploymentID, &deployment.Name, &deployment.ArtifactID, &deployment.OrganizationID,
		&deployment.GatewayID, &baseDeploymentID, &deployment.Content, &metadataJSON, &deployment.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrDeploymentNotFound
		}
		return nil, err
	}

	if baseDeploymentID.Valid {
		deployment.BaseDeploymentID = &baseDeploymentID.String
	}

	if metadataJSON != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err == nil {
			deployment.Metadata = metadata
		} else {
			return nil, fmt.Errorf("failed to unmarshal deployment metadata: %w", err)
		}
	}

	return deployment, nil
}

// Delete deletes a deployment record
func (r *DeploymentRepo) Delete(deploymentID, artifactUUID, orgUUID string) error {
	query := `DELETE FROM deployments WHERE deployment_id = ? AND artifact_uuid = ? AND organization_uuid = ?`

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
			d.deployment_id, d.name, d.artifact_uuid, d.organization_uuid, d.gateway_uuid,
			d.base_deployment_id, d.content, d.metadata, d.created_at,
			s.status, s.updated_at AS status_updated_at
		FROM deployments d
		INNER JOIN deployment_status s
			ON d.deployment_id = s.deployment_id
			AND d.artifact_uuid = s.artifact_uuid
			AND d.organization_uuid = s.organization_uuid
			AND d.gateway_uuid = s.gateway_uuid
		WHERE d.artifact_uuid = ? AND d.gateway_uuid = ? AND d.organization_uuid = ?
			AND s.status = ?
		ORDER BY d.created_at DESC
		LIMIT 1
	`

	var baseDeploymentID sql.NullString
	var metadataJSON string
	var statusStr string
	var updatedAt time.Time

	err := r.db.QueryRow(r.db.Rebind(query), artifactUUID, gatewayID, orgUUID, string(model.DeploymentStatusDeployed)).Scan(
		&deployment.DeploymentID, &deployment.Name, &deployment.ArtifactID, &deployment.OrganizationID,
		&deployment.GatewayID, &baseDeploymentID, &deployment.Content, &metadataJSON, &deployment.CreatedAt,
		&statusStr, &updatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if baseDeploymentID.Valid {
		deployment.BaseDeploymentID = &baseDeploymentID.String
	}

	if metadataJSON != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err == nil {
			deployment.Metadata = metadata
		} else {
			return nil, fmt.Errorf("failed to unmarshal deployment metadata: %w", err)
		}
	}

	// Populate status fields
	status := model.DeploymentStatus(statusStr)
	deployment.Status = &status
	deployment.UpdatedAt = &updatedAt

	return deployment, nil
}

// SetCurrent inserts or updates the deployment status record to set the current deployment for an artifact on a gateway
func (r *DeploymentRepo) SetCurrent(artifactUUID, orgUUID, gatewayID, deploymentID string, status model.DeploymentStatus) (time.Time, error) {
	updatedAt := time.Now()

	if r.db.Driver() == "postgres" || r.db.Driver() == "postgresql" {
		// PostgreSQL: Use ON CONFLICT
		query := `
			INSERT INTO deployment_status (artifact_uuid, organization_uuid, gateway_uuid, deployment_id, status, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (artifact_uuid, organization_uuid, gateway_uuid)
			DO UPDATE SET deployment_id = ?, status = ?, updated_at = ?
		`
		_, err := r.db.Exec(r.db.Rebind(query),
			artifactUUID, orgUUID, gatewayID, deploymentID, status, updatedAt,
			deploymentID, status, updatedAt)
		return updatedAt, err
	} else {
		// SQLite: Use REPLACE
		query := `
			REPLACE INTO deployment_status (artifact_uuid, organization_uuid, gateway_uuid, deployment_id, status, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`
		_, err := r.db.Exec(r.db.Rebind(query),
			artifactUUID, orgUUID, gatewayID, deploymentID, status, updatedAt)
		return updatedAt, err
	}
}

// GetStatus retrieves the current deployment status for an artifact on a gateway (lightweight - no content)
func (r *DeploymentRepo) GetStatus(artifactUUID, orgUUID, gatewayID string) (string, model.DeploymentStatus, *time.Time, error) {
	query := `
		SELECT deployment_id, status, updated_at
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
			d.deployment_id, d.name, d.artifact_uuid, d.organization_uuid, d.gateway_uuid,
			d.base_deployment_id, d.metadata, d.created_at,
			s.status, s.updated_at AS status_updated_at
		FROM deployments d
		LEFT JOIN deployment_status s
			ON d.deployment_id = s.deployment_id
			AND d.artifact_uuid = s.artifact_uuid
			AND d.organization_uuid = s.organization_uuid
			AND d.gateway_uuid = s.gateway_uuid
		WHERE d.deployment_id = ? AND d.artifact_uuid = ? AND d.organization_uuid = ?
	`

	var baseDeploymentID sql.NullString
	var metadataJSON string
	var statusStr sql.NullString
	var updatedAtVal sql.NullTime

	err := r.db.QueryRow(r.db.Rebind(query), deploymentID, artifactUUID, orgUUID).Scan(
		&deployment.DeploymentID, &deployment.Name, &deployment.ArtifactID, &deployment.OrganizationID, &deployment.GatewayID,
		&baseDeploymentID, &metadataJSON, &deployment.CreatedAt,
		&statusStr, &updatedAtVal)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrDeploymentNotFound
		}
		return nil, err
	}

	// Set nullable fields
	if baseDeploymentID.Valid {
		deployment.BaseDeploymentID = &baseDeploymentID.String
	}

	if metadataJSON != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err == nil {
			deployment.Metadata = metadata
		} else {
			return nil, fmt.Errorf("failed to unmarshal deployment metadata: %w", err)
		}
	}

	// Populate status fields from JOIN (nil if ARCHIVED)
	if statusStr.Valid {
		st := model.DeploymentStatus(statusStr.String)
		deployment.Status = &st
		if updatedAtVal.Valid {
			deployment.UpdatedAt = &updatedAtVal.Time
		}
	} else {
		// ARCHIVED state - Status and UpdatedAt remain nil
		archived := model.DeploymentStatusArchived
		deployment.Status = &archived
	}

	return deployment, nil
}

// GetDeploymentsWithState retrieves deployments with their lifecycle states.
// It enforces a soft limit of N records per Gateway, ensuring that the
// currently DEPLOYED or UNDEPLOYED record is always included regardless of its age.
func (r *DeploymentRepo) GetDeploymentsWithState(artifactUUID, orgUUID string, gatewayID *string, status *string, maxPerAPIGW int) ([]*model.Deployment, error) {

	// 1. Validation Logic
	if status != nil {
		validStatuses := map[string]bool{
			string(model.DeploymentStatusDeployed):   true,
			string(model.DeploymentStatusUndeployed): true,
			string(model.DeploymentStatusArchived):   true,
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
				d.deployment_id, d.name, d.artifact_uuid, d.organization_uuid, d.gateway_uuid,
                d.base_deployment_id, d.metadata, d.created_at,
                s.status as current_status,
                s.updated_at as status_updated_at,
                ROW_NUMBER() OVER (
                    PARTITION BY d.gateway_uuid
                    ORDER BY
                        (CASE WHEN s.status IS NOT NULL THEN 0 ELSE 1 END) ASC,
                        d.created_at DESC
                ) as rank_idx
			FROM deployments d
			LEFT JOIN deployment_status s
                ON d.deployment_id = s.deployment_id
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
			deployment_id, name, artifact_uuid, organization_uuid, gateway_uuid,
            base_deployment_id, metadata, created_at,
            current_status, status_updated_at
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
		var metadataJSON string
		var statusStr sql.NullString
		var updatedAtVal sql.NullTime

		err := rows.Scan(
			&deployment.DeploymentID, &deployment.Name, &deployment.ArtifactID,
			&deployment.OrganizationID, &deployment.GatewayID,
			&baseDeploymentID, &metadataJSON, &deployment.CreatedAt,
			&statusStr, &updatedAtVal)

		if err != nil {
			return nil, err
		}

		// Handle Nullable BaseDeploymentID
		if baseDeploymentID.Valid {
			deployment.BaseDeploymentID = &baseDeploymentID.String
		}

		// Handle Metadata
		if metadataJSON != "" {
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(metadataJSON), &metadata); err == nil {
				deployment.Metadata = metadata
			} else {
				return nil, fmt.Errorf("failed to unmarshal deployment metadata: %w", err)
			}
		}

		// Map Database Status to Model Status
		if statusStr.Valid {
			st := model.DeploymentStatus(statusStr.String)
			deployment.Status = &st
			if updatedAtVal.Valid {
				deployment.UpdatedAt = &updatedAtVal.Time
			}
		} else {
			// If the JOIN resulted in NULL, the record is ARCHIVED
			archived := model.DeploymentStatusArchived
			deployment.Status = &archived
			// For Archived, UpdatedAt usually defaults to nil
		}

		deployments = append(deployments, deployment)
	}

	// Check if the loop stopped because of an error rather than reaching the end
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during deployment rows iteration: %w", err)
	}

	return deployments, nil
}

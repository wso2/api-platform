/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
	"time"
)

// APIPublicationRepo implements the APIPublicationRepository interface
type APIPublicationRepo struct {
	db *database.DB
}

// NewAPIPublicationRepository creates a new API publication repository
func NewAPIPublicationRepository(db *database.DB) APIPublicationRepository {
	return &APIPublicationRepo{db: db}
}

// UpsertPublication creates or updates a publication record in a single query
func (r *APIPublicationRepo) UpsertPublication(publication *model.APIPublication) error {
	publication.UpdatedAt = time.Now()
	if publication.CreatedAt.IsZero() {
		publication.CreatedAt = time.Now()
	}

	// Validate the publication
	if err := publication.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	// Start transaction
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if the record exists within the transaction
	var exists bool
	checkQuery := `SELECT 1 FROM api_publications WHERE api_uuid = ? AND devportal_uuid = ? AND organization_uuid = ?`
	err = tx.QueryRow(checkQuery, publication.APIUUID, publication.DevPortalUUID, publication.OrganizationUUID).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existence: %w", err)
	}
	exists = (err != sql.ErrNoRows)

	if !exists {
		// Record does not exist, insert it
		insertQuery := `
			INSERT INTO api_publications (
				api_uuid, devportal_uuid, organization_uuid,
				status, api_version, devportal_ref_id,
				sandbox_endpoint_url, production_endpoint_url,
				created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		_, err = tx.Exec(insertQuery,
			publication.APIUUID, publication.DevPortalUUID, publication.OrganizationUUID,
			publication.Status, publication.APIVersion, publication.DevPortalRefID,
			publication.SandboxEndpointURL, publication.ProductionEndpointURL,
			publication.CreatedAt, publication.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to create API publication: %w", err)
		}
	} else {
		// Record exists, update it
		updateQuery := `
			UPDATE api_publications 
			SET status = ?, api_version = ?, devportal_ref_id = ?, 
			    sandbox_endpoint_url = ?, production_endpoint_url = ?, updated_at = ?
			WHERE api_uuid = ? AND devportal_uuid = ? AND organization_uuid = ?`
		result, err := tx.Exec(updateQuery,
			publication.Status, publication.APIVersion, publication.DevPortalRefID,
			publication.SandboxEndpointURL, publication.ProductionEndpointURL, publication.UpdatedAt,
			publication.APIUUID, publication.DevPortalUUID, publication.OrganizationUUID,
		)
		if err != nil {
			return fmt.Errorf("failed to update API publication: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get affected rows: %w", err)
		}
		if rowsAffected == 0 {
			// Verify if the row still exists (RowsAffected can be 0 for no-op updates)
			var stillExists bool
			err = tx.QueryRow(checkQuery, publication.APIUUID, publication.DevPortalUUID, publication.OrganizationUUID).Scan(&stillExists)
			if err == sql.ErrNoRows {
				return constants.ErrAPIPublicationNotFound
			}
			if err != nil {
				return fmt.Errorf("failed to verify API publication existence: %w", err)
			}
			// Row exists but no changes were made - this is OK (idempotent update)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Create creates a new API publication record
func (r *APIPublicationRepo) Create(publication *model.APIPublication) error {
	publication.CreatedAt = time.Now()
	publication.UpdatedAt = time.Now()

	// Validate the publication
	if err := publication.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		INSERT INTO api_publications (
			api_uuid, devportal_uuid, organization_uuid,
			status, api_version, devportal_ref_id,
			sandbox_endpoint_url, production_endpoint_url,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.Exec(query,
		publication.APIUUID, publication.DevPortalUUID, publication.OrganizationUUID,
		publication.Status, publication.APIVersion, publication.DevPortalRefID,
		publication.SandboxEndpointURL, publication.ProductionEndpointURL,
		publication.CreatedAt, publication.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create API publication: %w", err)
	}

	return nil
}

// GetByAPIAndDevPortal retrieves an API publication by API and DevPortal UUIDs
func (r *APIPublicationRepo) GetByAPIAndDevPortal(apiUUID, devPortalUUID, orgUUID string) (*model.APIPublication, error) {
	query := `
		SELECT api_uuid, devportal_uuid, organization_uuid,
			   status, api_version, devportal_ref_id,
			   sandbox_endpoint_url, production_endpoint_url,
			   created_at, updated_at
		FROM api_publications
		WHERE api_uuid = ? AND devportal_uuid = ? AND organization_uuid = ?`

	row := r.db.QueryRow(query, apiUUID, devPortalUUID, orgUUID)

	publication := &model.APIPublication{}
	err := row.Scan(
		&publication.APIUUID, &publication.DevPortalUUID, &publication.OrganizationUUID,
		&publication.Status, &publication.APIVersion, &publication.DevPortalRefID,
		&publication.SandboxEndpointURL, &publication.ProductionEndpointURL,
		&publication.CreatedAt, &publication.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, constants.ErrAPIPublicationNotFound
		}
		return nil, fmt.Errorf("failed to get API publication: %w", err)
	}

	return publication, nil
}

// GetByAPIUUID retrieves all API publications for a specific API and organization
func (r *APIPublicationRepo) GetByAPIUUID(apiUUID, orgUUID string) ([]*model.APIPublication, error) {
	query := `
		SELECT api_uuid, devportal_uuid, organization_uuid,
			   status, api_version, devportal_ref_id,
			   sandbox_endpoint_url, production_endpoint_url,
			   created_at, updated_at
		FROM api_publications
		WHERE api_uuid = ? AND organization_uuid = ?
		ORDER BY created_at DESC`

	rows, err := r.db.Query(query, apiUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to query API publications: %w", err)
	}
	defer rows.Close()

	var publications []*model.APIPublication
	for rows.Next() {
		publication := &model.APIPublication{}
		err := rows.Scan(
			&publication.APIUUID, &publication.DevPortalUUID, &publication.OrganizationUUID,
			&publication.Status, &publication.APIVersion, &publication.DevPortalRefID,
			&publication.SandboxEndpointURL, &publication.ProductionEndpointURL,
			&publication.CreatedAt, &publication.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API publication: %w", err)
		}
		publications = append(publications, publication)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over API publications: %w", err)
	}

	return publications, nil
}

// Update updates an existing API publication
func (r *APIPublicationRepo) Update(publication *model.APIPublication) error {
	publication.UpdatedAt = time.Now()

	// Validate the publication
	if err := publication.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		UPDATE api_publications 
		SET status = ?, api_version = ?, devportal_ref_id = ?, 
		    sandbox_endpoint_url = ?, production_endpoint_url = ?, updated_at = ?
		WHERE api_uuid = ? AND devportal_uuid = ? AND organization_uuid = ?`

	result, err := r.db.Exec(query,
		publication.Status, publication.APIVersion, publication.DevPortalRefID,
		publication.SandboxEndpointURL, publication.ProductionEndpointURL, publication.UpdatedAt,
		publication.APIUUID, publication.DevPortalUUID, publication.OrganizationUUID,
	)

	if err != nil {
		return fmt.Errorf("failed to update API publication: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		// Verify if the row still exists (RowsAffected can be 0 for no-op updates)
		var stillExists bool
		err = r.db.QueryRow(`
			SELECT 1 FROM api_publications 
			WHERE api_uuid = ? AND devportal_uuid = ? AND organization_uuid = ?`,
			publication.APIUUID, publication.DevPortalUUID, publication.OrganizationUUID,
		).Scan(&stillExists)
		if err == sql.ErrNoRows {
			return constants.ErrAPIPublicationNotFound
		}
		if err != nil {
			return fmt.Errorf("failed to verify API publication existence: %w", err)
		}
		// Row exists but no changes were made - this is OK (idempotent update)
	}

	return nil
}

// Delete removes a publication record
func (r *APIPublicationRepo) Delete(apiUUID, devPortalUUID, orgUUID string) error {
	query := `
		DELETE FROM api_publications 
		WHERE api_uuid = ? AND devportal_uuid = ? AND organization_uuid = ?`

	result, err := r.db.Exec(query, apiUUID, devPortalUUID, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to delete API publication: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return constants.ErrAPIPublicationNotFound
	}

	return nil
}

// GetAPIDevPortalsWithDetails retrieves all DevPortals associated with an API including publication details
// This mirrors the GetAPIGatewaysWithDetails pattern for consistency
func (r *APIPublicationRepo) GetAPIDevPortalsWithDetails(apiUUID, orgUUID string) ([]*model.APIDevPortalWithDetails, error) {
	query := `
		SELECT 
			-- DevPortal information
			d.uuid,
			d.organization_uuid,
			d.name,
			d.identifier,
			d.api_url,
			d.hostname,
			d.is_active,
			d.is_enabled,
			d.is_default,
			d.visibility,
			d.description,
			d.created_at,
			d.updated_at,
			
			-- Association information
			aa.created_at as associated_at,
			aa.updated_at as association_updated_at,
			
			-- Publication information (NULL if not published)
			CASE WHEN ap.api_uuid IS NOT NULL THEN 1 ELSE 0 END as is_published,
			ap.status as publication_status,
			ap.api_version,
			ap.devportal_ref_id,
			ap.sandbox_endpoint_url,
			ap.production_endpoint_url,
			ap.created_at as published_at,
			ap.updated_at as publication_updated_at
			
		FROM api_associations aa
		INNER JOIN devportals d 
			ON aa.resource_uuid = d.uuid
		LEFT JOIN api_publications ap 
			ON ap.api_uuid = aa.api_uuid 
			AND ap.devportal_uuid = aa.resource_uuid 
			AND ap.organization_uuid = aa.organization_uuid
			
		WHERE aa.api_uuid = ? 
		  AND aa.organization_uuid = ?
		  AND aa.association_type = 'dev_portal'
		ORDER BY aa.created_at DESC`

	rows, err := r.db.Query(query, apiUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to query API-DevPortal associations: %w", err)
	}
	defer rows.Close()

	var devPortals []*model.APIDevPortalWithDetails
	for rows.Next() {
		var dp model.APIDevPortalWithDetails
		err := rows.Scan(
			// DevPortal information
			&dp.UUID,
			&dp.OrganizationUUID,
			&dp.Name,
			&dp.Identifier,
			&dp.APIUrl,
			&dp.Hostname,
			&dp.IsActive,
			&dp.IsEnabled,
			&dp.IsDefault,
			&dp.Visibility,
			&dp.Description,
			&dp.CreatedAt,
			&dp.UpdatedAt,
			// Association information
			&dp.AssociatedAt,
			&dp.AssociationUpdatedAt,
			// Publication information (nullable)
			&dp.IsPublished,
			&dp.PublicationStatus,
			&dp.APIVersion,
			&dp.DevPortalRefID,
			&dp.SandboxEndpointURL,
			&dp.ProductionEndpointURL,
			&dp.PublishedAt,
			&dp.PublicationUpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan DevPortal row: %w", err)
		}
		devPortals = append(devPortals, &dp)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating DevPortal rows: %w", err)
	}

	return devPortals, nil
}

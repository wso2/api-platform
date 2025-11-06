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

	"github.com/google/uuid"
)

// APIPublicationRepository interface defines operations for API publication tracking
type APIPublicationRepository interface {
	// Basic CRUD operations - only the ones used by DevPortal manager
	Create(publication *model.APIPublication) error
	GetByAPIAndDevPortal(apiUUID, devPortalUUID, orgUUID string) (*model.APIPublication, error)
	Update(publication *model.APIPublication) error
}

// APIPublicationRepo implements the APIPublicationRepository interface
type APIPublicationRepo struct {
	db *database.DB
}

// NewAPIPublicationRepository creates a new API publication repository
func NewAPIPublicationRepository(db *database.DB) APIPublicationRepository {
	return &APIPublicationRepo{db: db}
}

// Create creates a new API publication record
func (r *APIPublicationRepo) Create(publication *model.APIPublication) error {
	if publication.UUID == "" {
		publication.UUID = uuid.New().String()
	}

	publication.CreatedAt = time.Now()
	publication.UpdatedAt = time.Now()

	// Validate the publication
	if err := publication.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		INSERT INTO api_publications (
			uuid, api_uuid, devportal_uuid, organization_uuid,
			status, api_version, devportal_ref_id,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.Exec(query,
		publication.UUID, publication.APIUUID, publication.DevPortalUUID, publication.OrganizationUUID,
		publication.Status, publication.APIVersion, publication.DevPortalRefID,
		publication.CreatedAt, publication.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create API publication: %w", err)
	}

	return nil
}

// GetByUUID retrieves an API publication by its UUID
func (r *APIPublicationRepo) GetByUUID(uuid string) (*model.APIPublication, error) {
	query := `
		SELECT uuid, api_uuid, devportal_uuid, organization_uuid,
			   status, api_version, devportal_ref_id,
			   created_at, updated_at
		FROM api_publications
		WHERE uuid = ?`

	row := r.db.QueryRow(query, uuid)

	publication := &model.APIPublication{}
	err := row.Scan(
		&publication.UUID, &publication.APIUUID, &publication.DevPortalUUID, &publication.OrganizationUUID,
		&publication.Status, &publication.APIVersion, &publication.DevPortalRefID,
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

// GetByAPIAndDevPortal retrieves an API publication by API and DevPortal UUIDs
func (r *APIPublicationRepo) GetByAPIAndDevPortal(apiUUID, devPortalUUID, orgUUID string) (*model.APIPublication, error) {
	query := `
		SELECT uuid, api_uuid, devportal_uuid, organization_uuid,
			   status, api_version, devportal_ref_id,
			   created_at, updated_at
		FROM api_publications
		WHERE api_uuid = ? AND devportal_uuid = ? AND organization_uuid = ?`

	row := r.db.QueryRow(query, apiUUID, devPortalUUID, orgUUID)

	publication := &model.APIPublication{}
	err := row.Scan(
		&publication.UUID, &publication.APIUUID, &publication.DevPortalUUID, &publication.OrganizationUUID,
		&publication.Status, &publication.APIVersion, &publication.DevPortalRefID,
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

// Update updates an existing API publication
func (r *APIPublicationRepo) Update(publication *model.APIPublication) error {
	publication.UpdatedAt = time.Now()

	// Validate the publication
	if err := publication.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	query := `
		UPDATE api_publications 
		SET status = ?, api_version = ?, devportal_ref_id = ?, updated_at = ?
		WHERE uuid = ? AND organization_uuid = ?`

	result, err := r.db.Exec(query,
		publication.Status, publication.APIVersion, publication.DevPortalRefID, publication.UpdatedAt,
		publication.UUID, publication.OrganizationUUID,
	)

	if err != nil {
		return fmt.Errorf("failed to update API publication: %w", err)
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

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

// publication_mappings and association_mappings tables removed — all functions in this file are disabled.

import (
	// "database/sql"   // publication_mappings table removed
	// "fmt"            // publication_mappings table removed
	// "platform-api/src/internal/constants" // publication_mappings table removed
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
	// "time"           // publication_mappings table removed
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
	// publication_mappings table removed — disabled
	// publication.UpdatedAt = time.Now()
	// if publication.CreatedAt.IsZero() { publication.CreatedAt = time.Now() }
	// if err := publication.Validate(); err != nil { return fmt.Errorf("validation error: %w", err) }
	// tx, err := r.db.Begin()
	// ...
	return nil
}

// Create creates a new API publication record
func (r *APIPublicationRepo) Create(publication *model.APIPublication) error {
	// publication_mappings table removed — disabled
	// publication.CreatedAt = time.Now()
	// publication.UpdatedAt = time.Now()
	// if err := publication.Validate(); err != nil { return fmt.Errorf("validation error: %w", err) }
	// query := `INSERT INTO publication_mappings (...) VALUES (...)`
	// _, err := r.db.Exec(...)
	// ...
	return nil
}

// GetByAPIAndDevPortal retrieves an API publication by API and DevPortal UUIDs
func (r *APIPublicationRepo) GetByAPIAndDevPortal(apiUUID, devPortalUUID, orgUUID string) (*model.APIPublication, error) {
	// publication_mappings table removed — disabled
	// query := `SELECT ... FROM publication_mappings WHERE api_uuid = ? AND devportal_uuid = ? AND organization_uuid = ?`
	// row := r.db.QueryRow(...)
	// ...
	return nil, nil
}

// GetByAPIUUID retrieves all API publications for a specific API and organization
func (r *APIPublicationRepo) GetByAPIUUID(apiUUID, orgUUID string) ([]*model.APIPublication, error) {
	// publication_mappings table removed — disabled
	// query := `SELECT ... FROM publication_mappings WHERE api_uuid = ? AND organization_uuid = ?`
	// rows, err := r.db.Query(...)
	// ...
	return nil, nil
}

// Update updates an existing API publication
func (r *APIPublicationRepo) Update(publication *model.APIPublication) error {
	// publication_mappings table removed — disabled
	// publication.UpdatedAt = time.Now()
	// if err := publication.Validate(); err != nil { return fmt.Errorf("validation error: %w", err) }
	// query := `UPDATE publication_mappings SET ... WHERE api_uuid = ? AND devportal_uuid = ? AND organization_uuid = ?`
	// result, err := r.db.Exec(...)
	// ...
	return nil
}

// Delete removes a publication record
func (r *APIPublicationRepo) Delete(apiUUID, devPortalUUID, orgUUID string) error {
	// publication_mappings table removed — disabled
	// query := `DELETE FROM publication_mappings WHERE api_uuid = ? AND devportal_uuid = ? AND organization_uuid = ?`
	// result, err := r.db.Exec(...)
	// ...
	return nil
}

// GetAPIDevPortalsWithDetails retrieves all DevPortals associated with an API including publication details
func (r *APIPublicationRepo) GetAPIDevPortalsWithDetails(apiUUID, orgUUID string) ([]*model.APIDevPortalWithDetails, error) {
	// publication_mappings and association_mappings tables removed — disabled
	// query := `SELECT ... FROM association_mappings aa INNER JOIN devportals d ... LEFT JOIN publication_mappings ap ...`
	// rows, err := r.db.Query(...)
	// ...
	return nil, nil
}

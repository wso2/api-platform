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
	"log"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
	"strings"
	"time"

	"github.com/google/uuid"
)

type devPortalRepository struct {
	db *database.DB
}

// NewDevPortalRepository creates a new instance of DevPortalRepository
func NewDevPortalRepository(db *database.DB) DevPortalRepository {
	return &devPortalRepository{db: db}
}

// Create creates a new DevPortal in the database
func (r *devPortalRepository) Create(devPortal *model.DevPortal) error {
	// Generate UUID if not provided
	if devPortal.UUID == "" {
		devPortal.UUID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	devPortal.CreatedAt = now
	devPortal.UpdatedAt = now

	// Validate before insertion
	if err := devPortal.Validate(); err != nil {
		return err
	}

	// Attempt to insert - let database constraints handle uniqueness
	query := `INSERT INTO devportals (
		uuid, organization_uuid, name, identifier, api_url, 
		hostname, is_active, is_enabled, api_key, header_key_name, is_default, visibility, description, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.Exec(query,
		devPortal.UUID, devPortal.OrganizationUUID, devPortal.Name, devPortal.Identifier,
		devPortal.APIUrl, devPortal.Hostname,
		devPortal.IsActive, devPortal.IsEnabled, devPortal.APIKey, devPortal.HeaderKeyName,
		devPortal.IsDefault, devPortal.Visibility, devPortal.Description, devPortal.CreatedAt, devPortal.UpdatedAt)

	if err != nil {
		// Handle unique constraint violations
		errStr := err.Error()
		if strings.Contains(errStr, "UNIQUE constraint failed") || strings.Contains(errStr, "constraint failed") {
			if strings.Contains(errStr, "organization_uuid, api_url") {
				return constants.ErrDevPortalAPIUrlExists
			}
			if strings.Contains(errStr, "identifier, api_url") {
				return constants.ErrDevPortalAPIUrlExists // identifier + api_url constraint
			}
			if strings.Contains(errStr, "organization_uuid, hostname") {
				return constants.ErrDevPortalHostnameExists
			}
			if strings.Contains(errStr, "idx_devportals_default_per_org") {
				return constants.ErrDevPortalDefaultAlreadyExists
			}
			return constants.ErrDevPortalAlreadyExists
		}
		log.Printf("[DevPortalRepository] Failed to create DevPortal %s: %v", devPortal.Name, err)
		return err
	}

	log.Printf("[DevPortalRepository] Created DevPortal %s (UUID: %s) for organization %s",
		devPortal.Name, devPortal.UUID, devPortal.OrganizationUUID)
	return nil
}

// GetByUUID retrieves a DevPortal by its UUID
func (r *devPortalRepository) GetByUUID(uuid, orgUUID string) (*model.DevPortal, error) {
	var devPortal model.DevPortal
	query := `SELECT uuid, organization_uuid, name, identifier, api_url, 
		hostname, is_active, is_enabled, api_key, header_key_name, is_default, visibility, description, created_at, updated_at 
		FROM devportals WHERE uuid = ? AND organization_uuid = ?`

	err := r.db.QueryRow(query, uuid, orgUUID).Scan(
		&devPortal.UUID, &devPortal.OrganizationUUID, &devPortal.Name, &devPortal.Identifier,
		&devPortal.APIUrl, &devPortal.Hostname,
		&devPortal.IsActive, &devPortal.IsEnabled, &devPortal.APIKey, &devPortal.HeaderKeyName,
		&devPortal.IsDefault, &devPortal.Visibility, &devPortal.Description, &devPortal.CreatedAt, &devPortal.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, constants.ErrDevPortalNotFound
		}
		log.Printf("[DevPortalRepository] Failed to get DevPortal by UUID %s for org %s: %v", uuid, orgUUID, err)
		return nil, err
	}

	return &devPortal, nil
}

// GetByOrganizationUUID retrieves DevPortals for an organization with optional filters
func (r *devPortalRepository) GetByOrganizationUUID(orgUUID string, isDefault, isActive *bool, limit, offset int) ([]*model.DevPortal, error) {
	query := `
		SELECT uuid, organization_uuid, name, identifier, api_url, 
			hostname, is_active, is_enabled, api_key, header_key_name, is_default, visibility, description, created_at, updated_at 
		FROM devportals 
		WHERE organization_uuid = ?`
	args := []interface{}{orgUUID}

	// Add filters if provided
	if isDefault != nil {
		query += " AND is_default = ?"
		args = append(args, *isDefault)
	}
	if isActive != nil {
		query += " AND is_active = ?"
		args = append(args, *isActive)
	}

	query += " ORDER BY is_default DESC, created_at ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		log.Printf("[DevPortalRepository] Failed to get filtered DevPortals for organization %s: %v", orgUUID, err)
		return nil, err
	}
	defer rows.Close()

	var devPortals []*model.DevPortal
	for rows.Next() {
		devPortal := &model.DevPortal{}
		err := rows.Scan(
			&devPortal.UUID, &devPortal.OrganizationUUID, &devPortal.Name, &devPortal.Identifier,
			&devPortal.APIUrl, &devPortal.Hostname,
			&devPortal.IsActive, &devPortal.IsEnabled, &devPortal.APIKey, &devPortal.HeaderKeyName,
			&devPortal.IsDefault, &devPortal.Visibility, &devPortal.Description, &devPortal.CreatedAt, &devPortal.UpdatedAt)
		if err != nil {
			return nil, err
		}
		devPortals = append(devPortals, devPortal)
	}

	return devPortals, rows.Err()
}

// Update updates an existing DevPortal
func (r *devPortalRepository) Update(devPortal *model.DevPortal, orgUUID string) error {
	// Update timestamp
	devPortal.UpdatedAt = time.Now()

	// Validate before update
	if err := devPortal.Validate(); err != nil {
		return err
	}

	query := `
		UPDATE devportals SET 
			name = ?, api_url = ?, hostname = ?,
			api_key = ?, header_key_name = ?, is_active = ?, is_enabled = ?,
			visibility = ?, description = ?, updated_at = ?
		WHERE uuid = ? AND organization_uuid = ?`

	result, err := r.db.Exec(query,
		devPortal.Name, devPortal.APIUrl, devPortal.Hostname,
		devPortal.APIKey, devPortal.HeaderKeyName,
		devPortal.IsActive, devPortal.IsEnabled, devPortal.Visibility, devPortal.Description, devPortal.UpdatedAt, devPortal.UUID, orgUUID)
	if err != nil {
		log.Printf("[DevPortalRepository] Failed to update DevPortal %s: %v", devPortal.UUID, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return constants.ErrDevPortalNotFound
	}

	log.Printf("[DevPortalRepository] Updated DevPortal %s (UUID: %s)", devPortal.Name, devPortal.UUID)
	return nil
}

// Delete deletes a DevPortal by UUID
func (r *devPortalRepository) Delete(uuid, orgUUID string) error {
	query := `DELETE FROM devportals WHERE uuid = ? AND organization_uuid = ?`

	result, err := r.db.Exec(query, uuid, orgUUID)
	if err != nil {
		log.Printf("[DevPortalRepository] Failed to delete DevPortal %s for org %s: %v", uuid, orgUUID, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return constants.ErrDevPortalNotFound
	}

	log.Printf("[DevPortalRepository] Deleted DevPortal %s", uuid)
	return nil
}

// GetDefaultByOrganizationUUID retrieves the default DevPortal for an organization
func (r *devPortalRepository) GetDefaultByOrganizationUUID(orgUUID string) (*model.DevPortal, error) {
	var devPortal model.DevPortal
	query := `SELECT uuid, organization_uuid, name, identifier, api_url, 
		hostname, is_active, is_enabled, api_key, header_key_name, is_default, visibility, description, created_at, updated_at 
		FROM devportals WHERE organization_uuid = ? AND is_default = 1`

	err := r.db.QueryRow(query, orgUUID).Scan(
		&devPortal.UUID, &devPortal.OrganizationUUID, &devPortal.Name, &devPortal.Identifier,
		&devPortal.APIUrl, &devPortal.Hostname,
		&devPortal.IsActive, &devPortal.IsEnabled, &devPortal.APIKey, &devPortal.HeaderKeyName,
		&devPortal.IsDefault, &devPortal.Visibility, &devPortal.Description, &devPortal.CreatedAt, &devPortal.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, constants.ErrDevPortalNotFound
		}
		log.Printf("[DevPortalRepository] Failed to get default DevPortal for organization %s: %v", orgUUID, err)
		return nil, err
	}

	return &devPortal, nil
}

// CountByOrganizationUUID counts DevPortals for an organization with optional filters
func (r *devPortalRepository) CountByOrganizationUUID(orgUUID string, isDefault, isActive *bool) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM devportals WHERE organization_uuid = ?`
	args := []interface{}{orgUUID}

	// Add filters if provided
	if isDefault != nil {
		query += " AND is_default = ?"
		args = append(args, *isDefault)
	}
	if isActive != nil {
		query += " AND is_active = ?"
		args = append(args, *isActive)
	}

	err := r.db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		log.Printf("[DevPortalRepository] Failed to count filtered DevPortals for organization %s: %v", orgUUID, err)
		return 0, err
	}

	return count, nil
}

// UpdateEnabledStatus updates the enabled status of a DevPortal
func (r *devPortalRepository) UpdateEnabledStatus(uuid, orgUUID string, isEnabled bool) error {
	query := `UPDATE devportals SET is_enabled = ?, updated_at = ? WHERE uuid = ? AND organization_uuid = ?`

	result, err := r.db.Exec(query, isEnabled, time.Now(), uuid, orgUUID)
	if err != nil {
		log.Printf("[DevPortalRepository] Failed to update enabled status for DevPortal %s (org %s): %v", uuid, orgUUID, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return constants.ErrDevPortalNotFound
	}

	log.Printf("[DevPortalRepository] Updated enabled status for DevPortal %s to %v", uuid, isEnabled)
	return nil
}

// SetAsDefault sets a DevPortal as the default for its organization
func (r *devPortalRepository) SetAsDefault(uuid, orgUUID string) error {
	// Get the DevPortal to find its organization
	devPortal, err := r.GetByUUID(uuid, orgUUID)
	if err != nil {
		return err
	}

	// Start transaction
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE devportals SET is_default = 0 WHERE organization_uuid = ? AND is_default = 1`,
		devPortal.OrganizationUUID)
	if err != nil {
		return err
	}

	// Set the new default
	result, err := tx.Exec(`UPDATE devportals SET is_default = 1, updated_at = ? WHERE uuid = ? AND organization_uuid = ?`,
		time.Now(), uuid, orgUUID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return constants.ErrDevPortalNotFound
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("[DevPortalRepository] Set DevPortal %s as default for organization %s", uuid, devPortal.OrganizationUUID)
	return nil
}

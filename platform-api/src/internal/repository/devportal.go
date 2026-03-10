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
	"log"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"
	"time"
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
		uuidStr, err := utils.GenerateUUID()
		if err != nil {
			return fmt.Errorf("failed to generate DevPortal ID: %w", err)
		}
		devPortal.UUID = uuidStr
	}

	// Set timestamps
	now := time.Now()
	devPortal.CreatedAt = now
	devPortal.UpdatedAt = now

	// Validate before insertion
	if err := devPortal.Validate(); err != nil {
		return err
	}

	// Start transaction to ensure atomicity of check and insert
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction for devportal creation: %w", err)
	}
	defer tx.Rollback()

	// Check for conflicts within the transaction
	if err := r.checkForConflictsTx(tx, devPortal); err != nil {
		return err
	}

	// Attempt to insert within the same transaction
	query := `INSERT INTO devportals (
		uuid, organization_uuid, name, identifier, api_url, 
		hostname, is_active, is_enabled, api_key, header_key_name, is_default, visibility, description, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.Exec(r.db.Rebind(query),
		devPortal.UUID, devPortal.OrganizationUUID, devPortal.Name, devPortal.Identifier,
		devPortal.APIUrl, devPortal.Hostname,
		devPortal.IsActive, devPortal.IsEnabled, devPortal.APIKey, devPortal.HeaderKeyName,
		devPortal.IsDefault, devPortal.Visibility, devPortal.Description, devPortal.CreatedAt, devPortal.UpdatedAt)

	if err != nil {
		log.Printf("[DevPortalRepository] Failed to create DevPortal %s: %v", devPortal.Name, err)
		return fmt.Errorf("failed to create devportal %s in organization %s: %w", devPortal.Name, devPortal.OrganizationUUID, err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit devportal creation transaction: %w", err)
	}

	log.Printf("[DevPortalRepository] Created DevPortal %s (UUID: %s) for organization %s",
		devPortal.Name, devPortal.UUID, devPortal.OrganizationUUID)
	return nil
}

// checkForConflictsTx checks for conflicts within a transaction
func (r *devPortalRepository) checkForConflictsTx(tx *sql.Tx, devPortal *model.DevPortal) error {
	// Check for existing DevPortal with same API URL in the same organization
	var count int
	err := tx.QueryRow(r.db.Rebind(`
		SELECT COUNT(*) FROM devportals
		WHERE organization_uuid = ? AND api_url = ?`),
		devPortal.OrganizationUUID, devPortal.APIUrl).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for existing API URL: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("devportal with API URL %s already exists in organization %s: %w",
			devPortal.APIUrl, devPortal.OrganizationUUID, constants.ErrDevPortalAPIUrlExists)
	}

	// Check for existing DevPortal with same hostname in the same organization
	err = tx.QueryRow(r.db.Rebind(`
		SELECT COUNT(*) FROM devportals
		WHERE organization_uuid = ? AND hostname = ?`),
		devPortal.OrganizationUUID, devPortal.Hostname).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for existing hostname: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("devportal with hostname %s already exists in organization %s: %w",
			devPortal.Hostname, devPortal.OrganizationUUID, constants.ErrDevPortalHostnameExists)
	}

	// Check for existing default DevPortal if this one is set as default
	if devPortal.IsDefault {
		err = tx.QueryRow(r.db.Rebind(`
			SELECT COUNT(*) FROM devportals
			WHERE organization_uuid = ? AND is_default = TRUE`),
			devPortal.OrganizationUUID).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check for existing default devportal: %w", err)
		}
		if count > 0 {
			return fmt.Errorf("default devportal already exists for organization %s: %w",
				devPortal.OrganizationUUID, constants.ErrDevPortalDefaultAlreadyExists)
		}
	}

	return nil
}

// checkForUpdateConflictsTx checks for update conflicts within a transaction
func (r *devPortalRepository) checkForUpdateConflictsTx(tx *sql.Tx, devPortal *model.DevPortal, orgUUID string) error {
	// Check for existing DevPortal with same API URL in the same organization (excluding this one)
	var count int
	err := tx.QueryRow(r.db.Rebind(`
		SELECT COUNT(*) FROM devportals
		WHERE organization_uuid = ? AND api_url = ? AND uuid != ?`),
		orgUUID, devPortal.APIUrl, devPortal.UUID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for existing API URL during update: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("devportal with API URL %s already exists in organization %s: %w",
			devPortal.APIUrl, orgUUID, constants.ErrDevPortalAPIUrlExists)
	}

	// Check for existing DevPortal with same hostname in the same organization (excluding this one)
	err = tx.QueryRow(r.db.Rebind(`
		SELECT COUNT(*) FROM devportals
		WHERE organization_uuid = ? AND hostname = ? AND uuid != ?`),
		orgUUID, devPortal.Hostname, devPortal.UUID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for existing hostname during update: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("devportal with hostname %s already exists in organization %s: %w",
			devPortal.Hostname, orgUUID, constants.ErrDevPortalHostnameExists)
	}

	return nil
}

// GetByUUID retrieves a DevPortal by its UUID
func (r *devPortalRepository) GetByUUID(uuid, orgUUID string) (*model.DevPortal, error) {
	var devPortal model.DevPortal
	query := `SELECT uuid, organization_uuid, name, identifier, api_url, 
		hostname, is_active, is_enabled, api_key, header_key_name, is_default, visibility, description, created_at, updated_at 
		FROM devportals WHERE uuid = ? AND organization_uuid = ?`

	err := r.db.QueryRow(r.db.Rebind(query), uuid, orgUUID).Scan(
		&devPortal.UUID, &devPortal.OrganizationUUID, &devPortal.Name, &devPortal.Identifier,
		&devPortal.APIUrl, &devPortal.Hostname,
		&devPortal.IsActive, &devPortal.IsEnabled, &devPortal.APIKey, &devPortal.HeaderKeyName,
		&devPortal.IsDefault, &devPortal.Visibility, &devPortal.Description, &devPortal.CreatedAt, &devPortal.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("devportal with UUID %s not found in organization %s: %w", uuid, orgUUID, constants.ErrDevPortalNotFound)
		}
		log.Printf("[DevPortalRepository] Failed to get DevPortal by UUID %s for org %s: %v", uuid, orgUUID, err)
		return nil, fmt.Errorf("failed to get devportal with UUID %s for organization %s: %w", uuid, orgUUID, err)
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

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		log.Printf("[DevPortalRepository] Failed to get filtered DevPortals for organization %s: %v", orgUUID, err)
		return nil, fmt.Errorf("failed to query devportals for organization %s: %w", orgUUID, err)
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
			return nil, fmt.Errorf("failed to scan devportal row for organization %s: %w", orgUUID, err)
		}
		devPortals = append(devPortals, devPortal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating devportal rows for organization %s: %w", orgUUID, err)
	}

	return devPortals, nil
}

// Update updates an existing DevPortal
func (r *devPortalRepository) Update(devPortal *model.DevPortal, orgUUID string) error {
	// Update timestamp
	devPortal.UpdatedAt = time.Now()

	// Validate before update
	if err := devPortal.Validate(); err != nil {
		return err
	}

	// Start transaction for atomic check and update
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction for devportal update: %w", err)
	}
	defer tx.Rollback()

	// Check for conflicts within the transaction
	if err := r.checkForUpdateConflictsTx(tx, devPortal, orgUUID); err != nil {
		return err
	}

	query := `
		UPDATE devportals SET 
			name = ?, api_url = ?, hostname = ?,
			api_key = ?, header_key_name = ?, is_active = ?, is_enabled = ?,
			visibility = ?, description = ?, updated_at = ?
		WHERE uuid = ? AND organization_uuid = ?`

	result, err := tx.Exec(r.db.Rebind(query),
		devPortal.Name, devPortal.APIUrl, devPortal.Hostname,
		devPortal.APIKey, devPortal.HeaderKeyName,
		devPortal.IsActive, devPortal.IsEnabled, devPortal.Visibility, devPortal.Description, devPortal.UpdatedAt, devPortal.UUID, orgUUID)
	if err != nil {
		log.Printf("[DevPortalRepository] Failed to update DevPortal %s: %v", devPortal.UUID, err)
		return fmt.Errorf("failed to update devportal %s in organization %s: %w", devPortal.UUID, orgUUID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for devportal update %s: %w", devPortal.UUID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("devportal with UUID %s not found in organization %s: %w", devPortal.UUID, orgUUID, constants.ErrDevPortalNotFound)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit devportal update transaction: %w", err)
	}

	log.Printf("[DevPortalRepository] Updated DevPortal %s (UUID: %s)", devPortal.Name, devPortal.UUID)
	return nil
}

// Delete deletes a DevPortal by UUID
func (r *devPortalRepository) Delete(uuid, orgUUID string) error {
	query := `DELETE FROM devportals WHERE uuid = ? AND organization_uuid = ?`

	result, err := r.db.Exec(r.db.Rebind(query), uuid, orgUUID)
	if err != nil {
		log.Printf("[DevPortalRepository] Failed to delete DevPortal %s for org %s: %v", uuid, orgUUID, err)
		return fmt.Errorf("failed to delete devportal %s from organization %s: %w", uuid, orgUUID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for devportal delete %s: %w", uuid, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("devportal with UUID %s not found in organization %s: %w", uuid, orgUUID, constants.ErrDevPortalNotFound)
	}

	log.Printf("[DevPortalRepository] Deleted DevPortal %s", uuid)
	return nil
}

// GetDefaultByOrganizationUUID retrieves the default DevPortal for an organization
func (r *devPortalRepository) GetDefaultByOrganizationUUID(orgUUID string) (*model.DevPortal, error) {
	var devPortal model.DevPortal
	query := `SELECT uuid, organization_uuid, name, identifier, api_url, 
		hostname, is_active, is_enabled, api_key, header_key_name, is_default, visibility, description, created_at, updated_at 
		FROM devportals WHERE organization_uuid = ? AND is_default = TRUE`

	err := r.db.QueryRow(r.db.Rebind(query), orgUUID).Scan(
		&devPortal.UUID, &devPortal.OrganizationUUID, &devPortal.Name, &devPortal.Identifier,
		&devPortal.APIUrl, &devPortal.Hostname,
		&devPortal.IsActive, &devPortal.IsEnabled, &devPortal.APIKey, &devPortal.HeaderKeyName,
		&devPortal.IsDefault, &devPortal.Visibility, &devPortal.Description, &devPortal.CreatedAt, &devPortal.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no default devportal found for organization %s: %w", orgUUID, constants.ErrDevPortalNotFound)
		}
		log.Printf("[DevPortalRepository] Failed to get default DevPortal for organization %s: %v", orgUUID, err)
		return nil, fmt.Errorf("failed to get default devportal for organization %s: %w", orgUUID, err)
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

	err := r.db.QueryRow(r.db.Rebind(query), args...).Scan(&count)
	if err != nil {
		log.Printf("[DevPortalRepository] Failed to count filtered DevPortals for organization %s: %v", orgUUID, err)
		return 0, fmt.Errorf("failed to count devportals for organization %s: %w", orgUUID, err)
	}

	return count, nil
}

// UpdateEnabledStatus updates the enabled status of a DevPortal
func (r *devPortalRepository) UpdateEnabledStatus(uuid, orgUUID string, isEnabled bool) error {
	query := `UPDATE devportals SET is_enabled = ?, updated_at = ? WHERE uuid = ? AND organization_uuid = ?`

	result, err := r.db.Exec(r.db.Rebind(query), isEnabled, time.Now(), uuid, orgUUID)
	if err != nil {
		log.Printf("[DevPortalRepository] Failed to update enabled status for DevPortal %s (org %s): %v", uuid, orgUUID, err)
		return fmt.Errorf("failed to update enabled status for devportal %s in organization %s: %w", uuid, orgUUID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for devportal enabled status update %s: %w", uuid, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("devportal with UUID %s not found in organization %s: %w", uuid, orgUUID, constants.ErrDevPortalNotFound)
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
		return fmt.Errorf("failed to start transaction for setting devportal %s as default: %w", uuid, err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(r.db.Rebind(`UPDATE devportals SET is_default = FALSE WHERE organization_uuid = ? AND is_default = TRUE`),
		devPortal.OrganizationUUID)
	if err != nil {
		return fmt.Errorf("failed to unset previous default devportal for organization %s: %w", devPortal.OrganizationUUID, err)
	}

	// Set the new default
	result, err := tx.Exec(r.db.Rebind(`UPDATE devportals SET is_default = TRUE, updated_at = ? WHERE uuid = ? AND organization_uuid = ?`),
		time.Now(), uuid, orgUUID)
	if err != nil {
		return fmt.Errorf("failed to set devportal %s as default for organization %s: %w", uuid, orgUUID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for setting devportal %s as default: %w", uuid, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("devportal with UUID %s not found in organization %s: %w", uuid, orgUUID, constants.ErrDevPortalNotFound)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction for setting devportal %s as default: %w", uuid, err)
	}

	log.Printf("[DevPortalRepository] Set DevPortal %s as default for organization %s", uuid, devPortal.OrganizationUUID)
	return nil
}

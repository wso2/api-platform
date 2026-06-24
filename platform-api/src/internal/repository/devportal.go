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

// devportals table removed — all functions in this file are disabled.

import (
	// "database/sql"   // devportals table removed
	// "fmt"            // devportals table removed
	// "log"            // devportals table removed
	// "platform-api/src/internal/constants" // devportals table removed
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
	// "platform-api/src/internal/utils"    // devportals table removed
	// "time"           // devportals table removed
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
	// devportals table removed — disabled
	// if devPortal.UUID == "" {
	// 	uuidStr, err := utils.GenerateUUID()
	// 	if err != nil {
	// 		return fmt.Errorf("failed to generate DevPortal ID: %w", err)
	// 	}
	// 	devPortal.UUID = uuidStr
	// }
	// now := time.Now()
	// devPortal.CreatedAt = now
	// devPortal.UpdatedAt = now
	// if err := devPortal.Validate(); err != nil { return err }
	// tx, err := r.db.Begin()
	// if err != nil { return fmt.Errorf("failed to start transaction for devportal creation: %w", err) }
	// defer tx.Rollback()
	// if err := r.checkForConflictsTx(tx, devPortal); err != nil { return err }
	// query := `INSERT INTO devportals (...) VALUES (...)`
	// _, err = tx.Exec(...)
	// if err != nil { return fmt.Errorf(...) }
	// if err := tx.Commit(); err != nil { return fmt.Errorf(...) }
	return nil
}

// checkForConflictsTx checks for conflicts within a transaction
func (r *devPortalRepository) checkForConflictsTx(tx interface{}, devPortal *model.DevPortal) error {
	// devportals table removed — disabled
	// var count int
	// err := tx.QueryRow(...).Scan(&count)
	// ...
	return nil
}

// checkForUpdateConflictsTx checks for update conflicts within a transaction
func (r *devPortalRepository) checkForUpdateConflictsTx(tx interface{}, devPortal *model.DevPortal, orgUUID string) error {
	// devportals table removed — disabled
	// var count int
	// err := tx.QueryRow(...).Scan(&count)
	// ...
	return nil
}

// GetByUUID retrieves a DevPortal by its UUID
func (r *devPortalRepository) GetByUUID(uuid, orgUUID string) (*model.DevPortal, error) {
	// devportals table removed — disabled
	// var devPortal model.DevPortal
	// query := `SELECT ... FROM devportals WHERE uuid = ? AND organization_uuid = ?`
	// err := r.db.QueryRow(...).Scan(...)
	// if err != nil { ... }
	// return &devPortal, nil
	return nil, nil
}

// GetByOrganizationUUID retrieves DevPortals for an organization with optional filters
func (r *devPortalRepository) GetByOrganizationUUID(orgUUID string, isDefault, isActive *bool, limit, offset int) ([]*model.DevPortal, error) {
	// devportals table removed — disabled
	// query := `SELECT ... FROM devportals WHERE organization_uuid = ?`
	// ...
	return nil, nil
}

// Update updates an existing DevPortal
func (r *devPortalRepository) Update(devPortal *model.DevPortal, orgUUID string) error {
	// devportals table removed — disabled
	// devPortal.UpdatedAt = time.Now()
	// if err := devPortal.Validate(); err != nil { return err }
	// tx, err := r.db.Begin()
	// ...
	return nil
}

// Delete deletes a DevPortal by UUID
func (r *devPortalRepository) Delete(uuid, orgUUID string) error {
	// devportals table removed — disabled
	// query := `DELETE FROM devportals WHERE uuid = ? AND organization_uuid = ?`
	// result, err := r.db.Exec(...)
	// ...
	return nil
}

// GetDefaultByOrganizationUUID retrieves the default DevPortal for an organization
func (r *devPortalRepository) GetDefaultByOrganizationUUID(orgUUID string) (*model.DevPortal, error) {
	// devportals table removed — disabled
	// var devPortal model.DevPortal
	// query := `SELECT ... FROM devportals WHERE organization_uuid = ? AND is_default = TRUE`
	// err := r.db.QueryRow(...).Scan(...)
	// ...
	return nil, nil
}

// CountByOrganizationUUID counts DevPortals for an organization with optional filters
func (r *devPortalRepository) CountByOrganizationUUID(orgUUID string, isDefault, isActive *bool) (int, error) {
	// devportals table removed — disabled
	// var count int
	// query := `SELECT COUNT(*) FROM devportals WHERE organization_uuid = ?`
	// ...
	return 0, nil
}

// UpdateEnabledStatus updates the enabled status of a DevPortal
func (r *devPortalRepository) UpdateEnabledStatus(uuid, orgUUID string, isEnabled bool) error {
	// devportals table removed — disabled
	// query := `UPDATE devportals SET is_enabled = ?, updated_at = ? WHERE uuid = ? AND organization_uuid = ?`
	// result, err := r.db.Exec(...)
	// ...
	return nil
}

// SetAsDefault sets a DevPortal as the default for its organization
func (r *devPortalRepository) SetAsDefault(uuid, orgUUID string) error {
	// devportals table removed — disabled
	// devPortal, err := r.GetByUUID(uuid, orgUUID)
	// if err != nil { return err }
	// tx, err := r.db.Begin()
	// ...
	return nil
}

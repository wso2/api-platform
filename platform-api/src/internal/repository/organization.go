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
	"errors"
	"time"

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
)

// OrganizationRepo implements OrganizationRepository
type OrganizationRepo struct {
	db *database.DB
}

// NewOrganizationRepo creates a new organization repository
func NewOrganizationRepo(db *database.DB) OrganizationRepository {
	return &OrganizationRepo{db: db}
}

// CreateOrganization inserts a new organization
func (r *OrganizationRepo) CreateOrganization(org *model.Organization) error {
	org.CreatedAt = time.Now()
	org.UpdatedAt = time.Now()

	query := `
		INSERT INTO organizations (uuid, handle, name, region, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query), org.ID, org.Handle, org.Name, org.Region, org.CreatedAt, org.UpdatedAt)

	return err
}

// GetOrganizationByIdOrHandle retrieves an organization by id or handle
func (r *OrganizationRepo) GetOrganizationByIdOrHandle(id, handle string) (*model.Organization, error) {
	org := &model.Organization{}
	query := `
		SELECT uuid, handle, name, region, created_at, updated_at
		FROM organizations
		WHERE uuid = ? OR handle = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), id, handle).Scan(
		&org.ID, &org.Handle, &org.Name, &org.Region, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return org, nil
}

// GetOrganizationByUUID retrieves an organization by ID
func (r *OrganizationRepo) GetOrganizationByUUID(orgId string) (*model.Organization, error) {
	org := &model.Organization{}
	query := `
		SELECT uuid, handle, name, region, created_at, updated_at
		FROM organizations
		WHERE uuid = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), orgId).Scan(
		&org.ID, &org.Handle, &org.Name, &org.Region, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return org, nil
}

// GetOrganizationByHandle retrieves an organization by handle
func (r *OrganizationRepo) GetOrganizationByHandle(handle string) (*model.Organization, error) {
	org := &model.Organization{}
	query := `
		SELECT uuid, handle, name, region, created_at, updated_at
		FROM organizations
		WHERE handle = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), handle).Scan(
		&org.ID, &org.Handle, &org.Name, &org.Region, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return org, nil
}

// UpdateOrganization modifies an existing organization
func (r *OrganizationRepo) UpdateOrganization(org *model.Organization) error {
	org.UpdatedAt = time.Now()
	query := `
		UPDATE organizations
		SET handle = ?, name = ?, region = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(r.db.Rebind(query), org.Handle, org.Name, org.Region, org.UpdatedAt, org.ID)

	return err
}

// DeleteOrganization removes an organization
func (r *OrganizationRepo) DeleteOrganization(orgId string) error {
	query := `DELETE FROM organizations WHERE uuid = ?`
	_, err := r.db.Exec(r.db.Rebind(query), orgId)
	return err
}

// ListOrganizations retrieves organizations with pagination
func (r *OrganizationRepo) ListOrganizations(limit, offset int) ([]*model.Organization, error) {
	query := `
		SELECT uuid, handle, name, region, created_at, updated_at
		FROM organizations
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(r.db.Rebind(query), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var organizations []*model.Organization
	for rows.Next() {
		org := &model.Organization{}
		err := rows.Scan(&org.ID, &org.Handle, &org.Name, &org.Region, &org.CreatedAt, &org.UpdatedAt)
		if err != nil {
			return nil, err
		}
		organizations = append(organizations, org)
	}

	return organizations, rows.Err()
}

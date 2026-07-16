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

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// OrganizationRepo implements OrganizationRepository
type OrganizationRepo struct {
	db                 *database.DB
	userOrgMappingRepo UserOrganizationMappingRepository
}

// NewOrganizationRepo creates a new organization repository
func NewOrganizationRepo(db *database.DB) OrganizationRepository {
	return &OrganizationRepo{db: db, userOrgMappingRepo: NewUserOrganizationMappingRepo(db)}
}

// CreateOrganization inserts a new organization
func (r *OrganizationRepo) CreateOrganization(org *model.Organization) error {
	org.CreatedAt = time.Now().UTC()
	org.UpdatedAt = time.Now().UTC()

	query := `
		INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_by, updated_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query),
		org.ID, org.Handle, org.Name, org.Region, org.IdpOrganizationRefUUID, org.CreatedBy, org.UpdatedBy, org.CreatedAt, org.UpdatedAt)
	return err
}

func (r *OrganizationRepo) GetOrganizationByIdOrHandle(id, handle string) (*model.Organization, error) {
	org := &model.Organization{}
	var createdBy, updatedBy sql.NullString
	query := `
		SELECT uuid, handle, display_name, region, idp_organization_ref_uuid, created_by, updated_by, created_at, updated_at
		FROM organizations
		WHERE uuid = ? OR handle = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), id, handle).Scan(
		&org.ID, &org.Handle, &org.Name, &org.Region, &org.IdpOrganizationRefUUID, &createdBy, &updatedBy, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	org.CreatedBy = createdBy.String
	org.UpdatedBy = updatedBy.String
	return org, nil
}

func (r *OrganizationRepo) GetOrganizationByUUID(orgId string) (*model.Organization, error) {
	org := &model.Organization{}
	var createdBy, updatedBy sql.NullString
	query := `
		SELECT uuid, handle, display_name, region, idp_organization_ref_uuid, created_by, updated_by, created_at, updated_at
		FROM organizations
		WHERE uuid = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), orgId).Scan(
		&org.ID, &org.Handle, &org.Name, &org.Region, &org.IdpOrganizationRefUUID, &createdBy, &updatedBy, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	org.CreatedBy = createdBy.String
	org.UpdatedBy = updatedBy.String
	return org, nil
}

func (r *OrganizationRepo) GetOrganizationByHandle(handle string) (*model.Organization, error) {
	org := &model.Organization{}
	var createdBy, updatedBy sql.NullString
	query := `
		SELECT uuid, handle, display_name, region, idp_organization_ref_uuid, created_by, updated_by, created_at, updated_at
		FROM organizations
		WHERE handle = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), handle).Scan(
		&org.ID, &org.Handle, &org.Name, &org.Region, &org.IdpOrganizationRefUUID, &createdBy, &updatedBy, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	org.CreatedBy = createdBy.String
	org.UpdatedBy = updatedBy.String
	return org, nil
}

// GetOrganizationByIdpOrgRefUUID looks up an organization by the identity
// provider's organization UUID (the value stored from the token's org claim).
// The empty string is never matched, so file-based organizations (which have
// no IDP reference) are not returned.
func (r *OrganizationRepo) GetOrganizationByIdpOrgRefUUID(idpOrgRefUUID string) (*model.Organization, error) {
	if idpOrgRefUUID == "" {
		return nil, nil
	}
	org := &model.Organization{}
	var createdBy, updatedBy sql.NullString
	query := `
		SELECT uuid, handle, display_name, region, idp_organization_ref_uuid, created_by, updated_by, created_at, updated_at
		FROM organizations
		WHERE idp_organization_ref_uuid = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), idpOrgRefUUID).Scan(
		&org.ID, &org.Handle, &org.Name, &org.Region, &org.IdpOrganizationRefUUID, &createdBy, &updatedBy, &org.CreatedAt, &org.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	org.CreatedBy = createdBy.String
	org.UpdatedBy = updatedBy.String
	return org, nil
}

func (r *OrganizationRepo) UpdateOrganization(org *model.Organization) error {
	org.UpdatedAt = time.Now().UTC()
	query := `
		UPDATE organizations
		SET handle = ?, display_name = ?, region = ?, updated_by = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(r.db.Rebind(query), org.Handle, org.Name, org.Region, org.UpdatedBy, org.UpdatedAt, org.ID)
	return err
}

func (r *OrganizationRepo) DeleteOrganization(orgId string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// The FK on org_uuid is declared ON DELETE CASCADE, so this delete is
	// normally redundant; it exists as defense-in-depth for pooled SQLite
	// connections that may not enforce foreign keys.
	if err := r.userOrgMappingRepo.DeleteByOrg(tx, orgId); err != nil {
		return err
	}

	if _, err := tx.Exec(r.db.Rebind(`DELETE FROM organizations WHERE uuid = ?`), orgId); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *OrganizationRepo) ListOrganizations(limit, offset int) ([]*model.Organization, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	query := `
		SELECT uuid, handle, display_name, region, idp_organization_ref_uuid, created_by, updated_by, created_at, updated_at
		FROM organizations
		ORDER BY created_at DESC
		` + pageClause
	rows, err := r.db.Query(r.db.Rebind(query), pageArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []*model.Organization
	for rows.Next() {
		org := &model.Organization{}
		var createdBy, updatedBy sql.NullString
		if err := rows.Scan(&org.ID, &org.Handle, &org.Name, &org.Region, &org.IdpOrganizationRefUUID, &createdBy, &updatedBy, &org.CreatedAt, &org.UpdatedAt); err != nil {
			return nil, err
		}
		org.CreatedBy = createdBy.String
		org.UpdatedBy = updatedBy.String
		orgs = append(orgs, org)
	}
	return orgs, rows.Err()
}

// CountOrganizations returns the total number of organizations, independent of
// any pagination applied by ListOrganizations.
func (r *OrganizationRepo) CountOrganizations() (int, error) {
	var total int
	query := `SELECT COUNT(*) FROM organizations`
	if err := r.db.QueryRow(r.db.Rebind(query)).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// ListOrganizationsForUser returns the organizations userUUID is a member of,
// per user_organization_mappings, newest first.
func (r *OrganizationRepo) ListOrganizationsForUser(userUUID string, limit, offset int) ([]*model.Organization, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	query := `
		SELECT o.uuid, o.handle, o.display_name, o.region, o.idp_organization_ref_uuid, o.created_by, o.updated_by, o.created_at, o.updated_at
		FROM organizations o
		JOIN user_organization_mappings m ON m.org_uuid = o.uuid
		WHERE m.user_uuid = ?
		ORDER BY o.created_at DESC
		` + pageClause
	args := append([]any{userUUID}, pageArgs...)
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []*model.Organization
	for rows.Next() {
		org := &model.Organization{}
		var createdBy, updatedBy sql.NullString
		if err := rows.Scan(&org.ID, &org.Handle, &org.Name, &org.Region, &org.IdpOrganizationRefUUID, &createdBy, &updatedBy, &org.CreatedAt, &org.UpdatedAt); err != nil {
			return nil, err
		}
		org.CreatedBy = createdBy.String
		org.UpdatedBy = updatedBy.String
		orgs = append(orgs, org)
	}
	return orgs, rows.Err()
}

// CountOrganizationsForUser returns the total membership count for userUUID,
// independent of any pagination applied by ListOrganizationsForUser.
func (r *OrganizationRepo) CountOrganizationsForUser(userUUID string) (int, error) {
	var total int
	query := `SELECT COUNT(*) FROM user_organization_mappings WHERE user_uuid = ?`
	if err := r.db.QueryRow(r.db.Rebind(query), userUUID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

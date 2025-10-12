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
		INSERT INTO organizations (uuid, handle, name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query, org.UUID, org.Handle, org.Name, org.CreatedAt, org.UpdatedAt)
	return err
}

// GetOrganizationByUUID retrieves an organization by UUID
func (r *OrganizationRepo) GetOrganizationByUUID(uuid string) (*model.Organization, error) {
	org := &model.Organization{}
	query := `
		SELECT uuid, handle, name, created_at, updated_at
		FROM organizations
		WHERE uuid = ?
	`
	err := r.db.QueryRow(query, uuid).Scan(
		&org.UUID, &org.Handle, &org.Name, &org.CreatedAt, &org.UpdatedAt,
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
		SELECT uuid, handle, name, created_at, updated_at
		FROM organizations
		WHERE handle = ?
	`
	err := r.db.QueryRow(query, handle).Scan(
		&org.UUID, &org.Handle, &org.Name, &org.CreatedAt, &org.UpdatedAt,
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
		SET handle = ?, name = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(query, org.Handle, org.Name, org.UpdatedAt, org.UUID)
	return err
}

// DeleteOrganization removes an organization
func (r *OrganizationRepo) DeleteOrganization(uuid string) error {
	query := `DELETE FROM organizations WHERE uuid = ?`
	_, err := r.db.Exec(query, uuid)
	return err
}

// ListOrganizations retrieves organizations with pagination
func (r *OrganizationRepo) ListOrganizations(limit, offset int) ([]*model.Organization, error) {
	query := `
		SELECT uuid, handle, name, created_at, updated_at
		FROM organizations
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var organizations []*model.Organization
	for rows.Next() {
		org := &model.Organization{}
		err := rows.Scan(&org.UUID, &org.Handle, &org.Name, &org.CreatedAt, &org.UpdatedAt)
		if err != nil {
			return nil, err
		}
		organizations = append(organizations, org)
	}

	return organizations, rows.Err()
}

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

// Create inserts a new organization
func (r *OrganizationRepo) Create(org *model.Organization) error {
	org.CreatedAt = time.Now()
	org.UpdatedAt = time.Now()

	query := `
		INSERT INTO organizations (uuid, handle, name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query, org.UUID, org.Handle, org.Name, org.CreatedAt, org.UpdatedAt)
	return err
}

// GetByUUID retrieves an organization by UUID
func (r *OrganizationRepo) GetByUUID(uuid string) (*model.Organization, error) {
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

// GetByHandle retrieves an organization by handle
func (r *OrganizationRepo) GetByHandle(handle string) (*model.Organization, error) {
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

// Update modifies an existing organization
func (r *OrganizationRepo) Update(org *model.Organization) error {
	org.UpdatedAt = time.Now()
	query := `
		UPDATE organizations
		SET handle = ?, name = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(query, org.Handle, org.Name, org.UpdatedAt, org.UUID)
	return err
}

// Delete removes an organization
func (r *OrganizationRepo) Delete(uuid string) error {
	query := `DELETE FROM organizations WHERE uuid = ?`
	_, err := r.db.Exec(query, uuid)
	return err
}

// List retrieves organizations with pagination
func (r *OrganizationRepo) List(limit, offset int) ([]*model.Organization, error) {
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

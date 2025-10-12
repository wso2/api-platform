package repository

import (
	"database/sql"
	"errors"
	"time"

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
)

// ProjectRepo implements ProjectRepository
type ProjectRepo struct {
	db *database.DB
}

// NewProjectRepo creates a new project repository
func NewProjectRepo(db *database.DB) ProjectRepository {
	return &ProjectRepo{db: db}
}

// Create inserts a new project
func (r *ProjectRepo) Create(project *model.Project) error {
	project.CreatedAt = time.Now()
	project.UpdatedAt = time.Now()

	query := `
		INSERT INTO projects (uuid, name, organization_id, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query, project.UUID, project.Name, project.OrganizationID, project.IsDefault, project.CreatedAt, project.UpdatedAt)
	if err != nil {
		return err
	}

	return nil
}

// GetByUUID retrieves a project by ID
func (r *ProjectRepo) GetByUUID(uuid string) (*model.Project, error) {
	project := &model.Project{}
	query := `
		SELECT uuid, name, organization_id, is_default, created_at, updated_at
		FROM projects
		WHERE uuid = ?
	`
	err := r.db.QueryRow(query, uuid).Scan(
		&project.UUID, &project.Name, &project.OrganizationID, &project.IsDefault, &project.CreatedAt, &project.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return project, nil
}

// GetByOrganizationID retrieves all projects for an organization
func (r *ProjectRepo) GetByOrganizationID(orgID string) ([]*model.Project, error) {
	query := `
		SELECT id, name, organization_id, is_default, created_at, updated_at
		FROM projects
		WHERE organization_id = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*model.Project
	for rows.Next() {
		project := &model.Project{}
		err := rows.Scan(&project.UUID, &project.Name, &project.OrganizationID, &project.IsDefault, &project.CreatedAt, &project.UpdatedAt)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	return projects, rows.Err()
}

// GetDefaultByOrganizationID retrieves the default project for an organization
func (r *ProjectRepo) GetDefaultByOrganizationID(orgID string) (*model.Project, error) {
	project := &model.Project{}
	query := `
		SELECT id, name, organization_id, is_default, created_at, updated_at
		FROM projects
		WHERE organization_id = ? AND is_default = true
		LIMIT 1
	`
	err := r.db.QueryRow(query, orgID).Scan(
		&project.UUID, &project.Name, &project.OrganizationID, &project.IsDefault, &project.CreatedAt, &project.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return project, nil
}

// Update modifies an existing project
func (r *ProjectRepo) Update(project *model.Project) error {
	project.UpdatedAt = time.Now()
	query := `
		UPDATE projects
		SET name = ?, organization_id = ?, is_default = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(query, project.Name, project.OrganizationID, project.IsDefault, project.UpdatedAt, project.UUID)
	return err
}

// Delete removes a project
func (r *ProjectRepo) Delete(uuid string) error {
	query := `DELETE FROM projects WHERE uuid = ?`
	_, err := r.db.Exec(query, uuid)
	return err
}

// List retrieves projects with pagination
func (r *ProjectRepo) List(limit, offset int) ([]*model.Project, error) {
	query := `
		SELECT uuid, name, organization_id, is_default, created_at, updated_at
		FROM projects
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*model.Project
	for rows.Next() {
		project := &model.Project{}
		err := rows.Scan(&project.UUID, &project.Name, &project.OrganizationID, &project.IsDefault, &project.CreatedAt, &project.UpdatedAt)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	return projects, rows.Err()
}

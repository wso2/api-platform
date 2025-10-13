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

// CreateProject inserts a new project
func (r *ProjectRepo) CreateProject(project *model.Project) error {
	project.CreatedAt = time.Now()
	project.UpdatedAt = time.Now()

	query := `
		INSERT INTO projects (uuid, name, organization_id, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query, project.UUID, project.Name, project.OrganizationID, project.IsDefault, project.CreatedAt, project.UpdatedAt)
	if err != nil {
		return err
	}

	return nil
}

// GetProjectByUUID retrieves a project by ID
func (r *ProjectRepo) GetProjectByUUID(uuid string) (*model.Project, error) {
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

// GetProjectByOrganizationID retrieves all projects for an organization
func (r *ProjectRepo) GetProjectByOrganizationID(orgID string) ([]*model.Project, error) {
	query := `
		SELECT uuid, name, organization_id, is_default, created_at, updated_at
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

// GetDefaultProjectByOrganizationID retrieves the default project for an organization
func (r *ProjectRepo) GetDefaultProjectByOrganizationID(orgID string) (*model.Project, error) {
	project := &model.Project{}
	query := `
		SELECT uuid, name, organization_id, is_default, created_at, updated_at
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

// UpdateProject modifies an existing project
func (r *ProjectRepo) UpdateProject(project *model.Project) error {
	project.UpdatedAt = time.Now()
	query := `
		UPDATE projects
		SET name = ?, organization_id = ?, is_default = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(query, project.Name, project.OrganizationID, project.IsDefault, project.UpdatedAt, project.UUID)
	return err
}

// DeleteProject removes a project
func (r *ProjectRepo) DeleteProject(uuid string) error {
	query := `DELETE FROM projects WHERE uuid = ?`
	_, err := r.db.Exec(query, uuid)
	return err
}

// ListProjects retrieves projects with pagination
func (r *ProjectRepo) ListProjects(orgID string, limit, offset int) ([]*model.Project, error) {
	query := `
		SELECT uuid, name, organization_id, is_default, created_at, updated_at
		FROM projects
		WHERE organization_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, orgID, limit, offset)
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

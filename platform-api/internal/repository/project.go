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
		INSERT INTO projects (uuid, handle, display_name, organization_uuid, description, created_by, created_at, updated_by, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query), project.ID, project.Handle, project.Name, project.OrganizationID, project.Description,
		project.CreatedBy, project.CreatedAt, project.UpdatedBy, project.UpdatedAt)
	if err != nil {
		return err
	}

	return nil
}

// GetProjectByUUID retrieves a project by ID
func (r *ProjectRepo) GetProjectByUUID(projectId string) (*model.Project, error) {
	project := &model.Project{}
	query := `
		SELECT uuid, handle, display_name, organization_uuid, description, created_by, created_at, updated_by, updated_at
		FROM projects
		WHERE uuid = ?
	`
	var createdBy, updatedBy sql.NullString
	err := r.db.QueryRow(r.db.Rebind(query), projectId).Scan(
		&project.ID, &project.Handle, &project.Name, &project.OrganizationID, &project.Description,
		&createdBy, &project.CreatedAt, &updatedBy, &project.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	project.CreatedBy = createdBy.String
	project.UpdatedBy = updatedBy.String
	return project, nil
}

// GetProjectByNameAndOrgID retrieves a project by name within an organization
func (r *ProjectRepo) GetProjectByNameAndOrgID(name, orgID string) (*model.Project, error) {
	project := &model.Project{}
	query := `
		SELECT uuid, handle, display_name, organization_uuid, description, created_by, created_at, updated_by, updated_at
		FROM projects
		WHERE display_name = ? AND organization_uuid = ?
	`
	var createdBy, updatedBy sql.NullString
	err := r.db.QueryRow(r.db.Rebind(query), name, orgID).Scan(
		&project.ID, &project.Handle, &project.Name, &project.OrganizationID, &project.Description,
		&createdBy, &project.CreatedAt, &updatedBy, &project.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	project.CreatedBy = createdBy.String
	project.UpdatedBy = updatedBy.String
	return project, nil
}

// GetProjectByHandleAndOrgID retrieves a project by handle within an organization
func (r *ProjectRepo) GetProjectByHandleAndOrgID(handle, orgID string) (*model.Project, error) {
	project := &model.Project{}
	query := `
		SELECT uuid, handle, display_name, organization_uuid, description, created_by, created_at, updated_by, updated_at
		FROM projects
		WHERE handle = ? AND organization_uuid = ?
	`
	var createdBy, updatedBy sql.NullString
	err := r.db.QueryRow(r.db.Rebind(query), handle, orgID).Scan(
		&project.ID, &project.Handle, &project.Name, &project.OrganizationID, &project.Description,
		&createdBy, &project.CreatedAt, &updatedBy, &project.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	project.CreatedBy = createdBy.String
	project.UpdatedBy = updatedBy.String
	return project, nil
}

// GetProjectsByOrganizationID retrieves all projects for an organization
func (r *ProjectRepo) GetProjectsByOrganizationID(orgID string) ([]*model.Project, error) {
	query := `
		SELECT uuid, handle, display_name, organization_uuid, description, created_by, created_at, updated_by, updated_at
		FROM projects
		WHERE organization_uuid = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(r.db.Rebind(query), orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*model.Project
	for rows.Next() {
		project := &model.Project{}
		var createdBy, updatedBy sql.NullString
		err := rows.Scan(&project.ID, &project.Handle, &project.Name, &project.OrganizationID, &project.Description,
			&createdBy, &project.CreatedAt, &updatedBy, &project.UpdatedAt)
		if err != nil {
			return nil, err
		}
		project.CreatedBy = createdBy.String
		project.UpdatedBy = updatedBy.String
		projects = append(projects, project)
	}

	return projects, rows.Err()
}

// UpdateProject modifies an existing project
func (r *ProjectRepo) UpdateProject(project *model.Project) error {
	project.UpdatedAt = time.Now()
	query := `
		UPDATE projects
		SET display_name = ?, description = ?, updated_by = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(r.db.Rebind(query), project.Name, project.Description, project.UpdatedBy, project.UpdatedAt, project.ID)
	return err
}

// DeleteProject removes a project
func (r *ProjectRepo) DeleteProject(projectId string) error {
	query := `DELETE FROM projects WHERE uuid = ?`
	_, err := r.db.Exec(r.db.Rebind(query), projectId)
	return err
}

// ListProjects retrieves projects with pagination
func (r *ProjectRepo) ListProjects(orgID string, opts ListOptions) ([]*model.Project, error) {
	query := `
		SELECT uuid, handle, display_name, organization_uuid, description, created_at, updated_at
		FROM projects
		WHERE organization_uuid = ?`
	args := []any{orgID}
	if searchClause, searchArgs := handleSearchClause(opts.Search); searchClause != "" {
		query += searchClause
		args = append(args, searchArgs...)
	}
	col, dir := opts.resolveSort(listSortColumns, "created_at")
	pageClause, pageArgs := r.db.PaginationClause(opts.Limit, opts.Offset)
	query += " ORDER BY " + col + " " + dir + ", handle ASC " + pageClause
	args = append(args, pageArgs...)
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*model.Project
	for rows.Next() {
		project := &model.Project{}
		err := rows.Scan(&project.ID, &project.Handle, &project.Name, &project.OrganizationID, &project.Description,
			&project.CreatedAt, &project.UpdatedAt)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	return projects, rows.Err()
}

// CountProjects returns the total number of projects in an organization,
// independent of any pagination applied by ListProjects.
func (r *ProjectRepo) CountProjects(orgID, search string) (int, error) {
	var total int
	query := `SELECT COUNT(*) FROM projects WHERE organization_uuid = ?`
	args := []any{orgID}
	if searchClause, searchArgs := handleSearchClause(search); searchClause != "" {
		query += searchClause
		args = append(args, searchArgs...)
	}
	if err := r.db.QueryRow(r.db.Rebind(query), args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

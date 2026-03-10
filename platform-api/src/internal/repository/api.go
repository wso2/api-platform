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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"
)

// APIRepo implements APIRepository
type APIRepo struct {
	db           *database.DB
	artifactRepo ArtifactRepository
}

// NewAPIRepo creates a new API repository
func NewAPIRepo(db *database.DB) APIRepository {
	return &APIRepo{
		db:           db,
		artifactRepo: NewArtifactRepo(db),
	}
}

// CreateAPI inserts a new API with all its configurations
func (r *APIRepo) CreateAPI(api *model.API) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Always generate a new UUID for the API
	apiID, err := utils.GenerateUUID()
	if err != nil {
		return fmt.Errorf("failed to generate API ID: %w", err)
	}
	api.ID = apiID
	api.CreatedAt = time.Now()
	api.UpdatedAt = time.Now()

	// Convert transport slice to JSON
	transportJSON, err := json.Marshal(api.Transport)
	if err != nil {
		return fmt.Errorf("failed to marshal transport: %w", err)
	}

	configurationJSON, err := serializeAPIConfigurations(api.Configuration)
	if err != nil {
		return err
	}

	kind := constants.RestApi
	if api.Kind == constants.WebSubApi {
		kind = constants.WebSubApi
	}

	if err := r.artifactRepo.Create(tx, &model.Artifact{
		UUID:             api.ID,
		Handle:           api.Handle,
		Name:             api.Name,
		Version:          api.Version,
		Kind:             kind,
		OrganizationUUID: api.OrganizationID,
		CreatedAt:        api.CreatedAt,
		UpdatedAt:        api.UpdatedAt,
	}); err != nil {
		return err
	}

	apiQuery := `
		INSERT INTO rest_apis (uuid, description, created_by, project_uuid, lifecycle_status, transport, configuration)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = tx.Exec(r.db.Rebind(apiQuery), api.ID, api.Description,
		api.CreatedBy, api.ProjectID, api.LifeCycleStatus,
		string(transportJSON), configurationJSON)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetAPIByUUID retrieves an API by UUID with all its configurations
func (r *APIRepo) GetAPIByUUID(apiUUID, orgUUID string) (*model.API, error) {
	api := &model.API{}

	query := `
		SELECT art.uuid, art.handle, art.name, art.kind, a.description, art.version, a.created_by,
			a.project_uuid, art.organization_uuid, a.lifecycle_status,
			a.transport, a.configuration, art.created_at, art.updated_at
		FROM rest_apis a INNER JOIN artifacts art
		ON a.uuid = art.uuid
		WHERE a.uuid = ? AND art.organization_uuid = ?
	`

	var transportJSON string
	var configJSON sql.NullString
	err := r.db.QueryRow(r.db.Rebind(query), apiUUID, orgUUID).Scan(
		&api.ID, &api.Handle, &api.Name, &api.Kind, &api.Description,
		&api.Version, &api.CreatedBy, &api.ProjectID, &api.OrganizationID, &api.LifeCycleStatus,
		&transportJSON, &configJSON,
		&api.CreatedAt, &api.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Parse transport JSON
	if transportJSON != "" {
		json.Unmarshal([]byte(transportJSON), &api.Transport)
	}
	if config, err := deserializeAPIConfigurations(configJSON); err != nil {
		return nil, err
	} else if config != nil {
		api.Configuration = *config
	}

	return api, nil
}

// GetAPIMetadataByHandle retrieves minimal API information by handle and organization ID
func (r *APIRepo) GetAPIMetadataByHandle(handle, orgUUID string) (*model.APIMetadata, error) {
	metadata := &model.APIMetadata{}

	query := `SELECT uuid, handle, name, version, kind, organization_uuid FROM artifacts WHERE handle = ? AND organization_uuid = ?`

	err := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(
		&metadata.ID, &metadata.Handle, &metadata.Name, &metadata.Version, &metadata.Kind, &metadata.OrganizationID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return metadata, nil
}

// GetAPIsByProjectUUID retrieves all APIs for a project
func (r *APIRepo) GetAPIsByProjectUUID(projectUUID, orgUUID string) ([]*model.API, error) {
	query := `
		SELECT art.uuid, art.handle, art.name, art.kind, a.description, art.version, a.created_by,
			a.project_uuid, art.organization_uuid, a.lifecycle_status,
			a.transport, a.configuration, art.created_at, art.updated_at
		FROM rest_apis a INNER JOIN artifacts art
		ON a.uuid = art.uuid
		WHERE a.project_uuid = ? AND art.organization_uuid = ?
		ORDER BY art.created_at DESC
	`

	rows, err := r.db.Query(r.db.Rebind(query), projectUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api := &model.API{}
		var transportJSON string
		var configJSON sql.NullString
		err := rows.Scan(&api.ID, &api.Handle, &api.Name, &api.Kind, &api.Description,
			&api.Version, &api.CreatedBy, &api.ProjectID, &api.OrganizationID,
			&api.LifeCycleStatus,
			&transportJSON, &configJSON, &api.CreatedAt, &api.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Parse transport JSON
		if transportJSON != "" {
			json.Unmarshal([]byte(transportJSON), &api.Transport)
		}
		if config, err := deserializeAPIConfigurations(configJSON); err != nil {
			return nil, err
		} else if config != nil {
			api.Configuration = *config
		}

		apis = append(apis, api)
	}

	return apis, rows.Err()
}

// GetAPIsByOrganizationUUID retrieves all APIs for an organization with optional project filter
func (r *APIRepo) GetAPIsByOrganizationUUID(orgUUID string, projectUUID string) ([]*model.API, error) {
	var query string
	var args []interface{}

	if projectUUID != "" {
		// Filter by specific project within the organization
		query = `
			SELECT art.uuid, art.handle, art.name, art.kind, a.description, art.version, a.created_by,
				a.project_uuid, art.organization_uuid, a.lifecycle_status,
				a.transport, a.configuration, art.created_at, art.updated_at
			FROM rest_apis a INNER JOIN artifacts art
			ON a.uuid = art.uuid
			WHERE art.organization_uuid = ? AND a.project_uuid = ?
			ORDER BY art.created_at DESC
		`
		args = []interface{}{orgUUID, projectUUID}
	} else {
		// Get all APIs for the organization
		query = `
			SELECT art.uuid, art.handle, art.name, art.kind, a.description, art.version, a.created_by,
				a.project_uuid, art.organization_uuid, a.lifecycle_status,
				a.transport, a.configuration, art.created_at, art.updated_at
			FROM rest_apis a INNER JOIN artifacts art
			ON a.uuid = art.uuid
			WHERE art.organization_uuid = ?
			ORDER BY art.created_at DESC
		`
		args = []interface{}{orgUUID}
	}

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api := &model.API{}
		var transportJSON string
		var configJSON sql.NullString
		err := rows.Scan(&api.ID, &api.Handle, &api.Name, &api.Kind, &api.Description,
			&api.Version, &api.CreatedBy, &api.ProjectID, &api.OrganizationID,
			&api.LifeCycleStatus,
			&transportJSON, &configJSON, &api.CreatedAt, &api.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Parse transport JSON
		if transportJSON != "" {
			json.Unmarshal([]byte(transportJSON), &api.Transport)
		}
		if config, err := deserializeAPIConfigurations(configJSON); err != nil {
			return nil, err
		} else if config != nil {
			api.Configuration = *config
		}

		apis = append(apis, api)
	}

	return apis, rows.Err()
}

// GetDeployedAPIsByGatewayUUID retrieves all APIs deployed to a specific gateway
func (r *APIRepo) GetDeployedAPIsByGatewayUUID(gatewayUUID, orgUUID string) ([]*model.API, error) {
	query := `
		SELECT a.uuid, art.name, a.description, art.version, a.created_by,
		       a.project_uuid, art.organization_uuid, art.kind, art.created_at, art.updated_at
		FROM rest_apis a INNER JOIN artifacts art ON a.uuid = art.uuid
		INNER JOIN deployment_status ad ON art.uuid = ad.artifact_uuid
		WHERE ad.gateway_uuid = ? AND art.organization_uuid = ? AND ad.status = ?
		ORDER BY art.created_at DESC
	`

	rows, err := r.db.Query(r.db.Rebind(query), gatewayUUID, orgUUID, string(model.DeploymentStatusDeployed))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api := &model.API{}
		err := rows.Scan(&api.ID, &api.Name, &api.Description,
			&api.Version, &api.CreatedBy, &api.ProjectID, &api.OrganizationID,
			&api.Kind, &api.CreatedAt, &api.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API row: %w", err)
		}
		apis = append(apis, api)
	}

	return apis, nil
}

// GetAPIsByGatewayUUID retrieves all APIs associated with a specific gateway
func (r *APIRepo) GetAPIsByGatewayUUID(gatewayUUID, orgUUID string) ([]*model.API, error) {
	query := `
		SELECT a.uuid, art.name, a.description, art.version, a.created_by,
			a.project_uuid, art.organization_uuid, art.kind, art.created_at, art.updated_at
		FROM rest_apis a
		INNER JOIN artifacts art ON a.uuid = art.uuid
		INNER JOIN association_mappings aa ON a.uuid = aa.artifact_uuid
		WHERE aa.resource_uuid = ? AND aa.association_type = 'gateway' AND art.organization_uuid = ?
		ORDER BY art.created_at DESC
	`

	rows, err := r.db.Query(r.db.Rebind(query), gatewayUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to query APIs associated with gateway: %w", err)
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api := &model.API{}
		err := rows.Scan(&api.ID, &api.Name, &api.Description,
			&api.Version, &api.CreatedBy, &api.ProjectID, &api.OrganizationID,
			&api.Kind, &api.CreatedAt, &api.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API row: %w", err)
		}
		apis = append(apis, api)
	}

	return apis, nil
}

// UpdateAPI modifies an existing API
func (r *APIRepo) UpdateAPI(api *model.API) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	api.UpdatedAt = time.Now()

	// Convert transport slice to JSON
	transportJSON, err := json.Marshal(api.Transport)
	if err != nil {
		return fmt.Errorf("failed to marshal transport: %w", err)
	}

	configurationJSON, err := serializeAPIConfigurations(api.Configuration)
	if err != nil {
		return err
	}
	// Update artifact record
	if err := r.artifactRepo.Update(tx, &model.Artifact{
		UUID:             api.ID,
		Name:             api.Name,
		Version:          api.Version,
		OrganizationUUID: api.OrganizationID,
		UpdatedAt:        api.UpdatedAt,
	}); err != nil {
		return err
	}
	// Update main API record
	query := `
		UPDATE rest_apis SET description = ?,
			created_by = ?, lifecycle_status = ?,
			transport = ?, configuration = ?
		WHERE uuid = ?
	`
	_, err = tx.Exec(r.db.Rebind(query), api.Description,
		api.CreatedBy, api.LifeCycleStatus,
		string(transportJSON), configurationJSON,
		api.ID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteAPI removes an API and all its configurations
func (r *APIRepo) DeleteAPI(apiUUID, orgUUID string) error {
	// Start transaction for atomicity
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete in order of dependencies (children first, parent last)
	deleteQueries := []string{
		// Delete API associations first
		`DELETE FROM association_mappings WHERE artifact_uuid = ? AND organization_uuid = ?`,
		// Delete API publications
		`DELETE FROM publication_mappings WHERE api_uuid = ? AND organization_uuid = ?`,
		// Delete API deployments
		`DELETE FROM deployments WHERE artifact_uuid = ? AND organization_uuid = ?`,
		// Delete from rest_apis table first, then artifacts
		`DELETE FROM rest_apis WHERE uuid = ?`,
	}

	// Execute all delete statements
	for i, query := range deleteQueries {
		switch i {
		case 0, 1, 2:
			if _, err := tx.Exec(r.db.Rebind(query), apiUUID, orgUUID); err != nil {
				return err
			}
		default:
			if _, err := tx.Exec(r.db.Rebind(query), apiUUID); err != nil {
				return err
			}
		}
	}

	// Delete from artifacts table using artifactRepo
	if err := r.artifactRepo.Delete(tx, apiUUID); err != nil {
		return err
	}

	return tx.Commit()
}

// CheckAPIExistsByHandleInOrganization checks if an API with the given handle exists within a specific organization
func (r *APIRepo) CheckAPIExistsByHandleInOrganization(handle, orgUUID string) (bool, error) {
	return r.artifactRepo.Exists(constants.RestApi, handle, orgUUID)
}

func serializePolicies(policies []model.Policy) (any, error) {
	if policies == nil {
		policies = []model.Policy{}
	}
	policiesJSON, err := json.Marshal(policies)
	if err != nil {
		return nil, err
	}

	return string(policiesJSON), nil
}

func deserializePolicies(policiesJSON sql.NullString) ([]model.Policy, error) {
	if !policiesJSON.Valid || policiesJSON.String == "" {
		return []model.Policy{}, nil
	}

	var policies []model.Policy
	if err := json.Unmarshal([]byte(policiesJSON.String), &policies); err != nil {
		return nil, err
	}

	return policies, nil
}

func serializeAPIConfigurations(config model.RestAPIConfig) (any, error) {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	return string(configJSON), nil
}

func deserializeAPIConfigurations(configJSON sql.NullString) (*model.RestAPIConfig, error) {
	if !configJSON.Valid || configJSON.String == "" {
		return nil, fmt.Errorf("Null configuration")
	}

	var config model.RestAPIConfig
	if err := json.Unmarshal([]byte(configJSON.String), &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// CheckAPIExistsByNameAndVersionInOrganization checks if an API with the given name and version exists within a specific organization
// excludeHandle: if provided, excludes the API with this handle from the check (useful for updates)
func (r *APIRepo) CheckAPIExistsByNameAndVersionInOrganization(name, version, orgUUID, excludeHandle string) (bool, error) {
	var query string
	var args []interface{}

	if excludeHandle != "" {
		query = `
			SELECT COUNT(*) FROM artifacts
			WHERE name = ? AND version = ? AND organization_uuid = ? AND handle != ?
		`
		args = []interface{}{name, version, orgUUID, excludeHandle}
	} else {
		query = `
			SELECT COUNT(*) FROM artifacts
			WHERE name = ? AND version = ? AND organization_uuid = ?
		`
		args = []interface{}{name, version, orgUUID}
	}

	var count int
	err := r.db.QueryRow(r.db.Rebind(query), args...).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// CreateAPIAssociation creates an association between an API and resource (e.g., gateway or dev portal)
func (r *APIRepo) CreateAPIAssociation(association *model.APIAssociation) error {
	if r.db.Driver() == "postgres" || r.db.Driver() == "postgresql" {
		// PostgreSQL: use RETURNING to get the generated ID
		query := `
			INSERT INTO association_mappings (artifact_uuid, organization_uuid, resource_uuid, association_type, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			RETURNING id
		`
		if err := r.db.QueryRow(r.db.Rebind(query), association.ArtifactID, association.OrganizationID, association.ResourceID,
			association.AssociationType, association.CreatedAt, association.UpdatedAt).Scan(&association.ID); err != nil {
			return err
		}
	} else {
		// SQLite: use LastInsertId
		query := `
			INSERT INTO association_mappings (artifact_uuid, organization_uuid, resource_uuid, association_type, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`
		result, err := r.db.Exec(r.db.Rebind(query), association.ArtifactID, association.OrganizationID, association.ResourceID,
			association.AssociationType, association.CreatedAt, association.UpdatedAt)
		if err != nil {
			return err
		}

		lastID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		association.ID = lastID
	}

	return nil
}

// UpdateAPIAssociation updates the updated_at timestamp for an existing API resource association
func (r *APIRepo) UpdateAPIAssociation(apiUUID, resourceId, associationType, orgUUID string) error {
	query := `
		UPDATE association_mappings
		SET updated_at = ?
		WHERE artifact_uuid = ? AND resource_uuid = ? AND association_type = ? AND organization_uuid = ?
	`
	_, err := r.db.Exec(r.db.Rebind(query), time.Now(), apiUUID, resourceId, associationType, orgUUID)
	return err
}

// GetAPIAssociations retrieves all resource associations for an API of a specific type
func (r *APIRepo) GetAPIAssociations(apiUUID, associationType, orgUUID string) ([]*model.APIAssociation, error) {
	query := `
		SELECT id, artifact_uuid, organization_uuid, resource_uuid, association_type, created_at, updated_at
		FROM association_mappings
		WHERE artifact_uuid = ? AND association_type = ? AND organization_uuid = ?
	`
	rows, err := r.db.Query(r.db.Rebind(query), apiUUID, associationType, orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var associations []*model.APIAssociation
	for rows.Next() {
		var association model.APIAssociation
		err := rows.Scan(&association.ID, &association.ArtifactID, &association.OrganizationID,
			&association.ResourceID, &association.AssociationType, &association.CreatedAt, &association.UpdatedAt)
		if err != nil {
			return nil, err
		}
		associations = append(associations, &association)
	}

	return associations, rows.Err()
}

// GetAPIGatewaysWithDetails retrieves all gateways associated with an API including deployment details
func (r *APIRepo) GetAPIGatewaysWithDetails(apiUUID, orgUUID string) ([]*model.APIGatewayWithDetails, error) {
	query := `
		SELECT
			g.uuid as id,
			g.organization_uuid as organization_id,
			g.name,
			g.display_name,
			g.description,
			g.properties,
			g.vhost,
			g.is_critical,
			g.gateway_functionality_type as functionality_type,
			g.is_active,
			g.created_at,
			g.updated_at,
			aa.created_at as associated_at,
			aa.updated_at as association_updated_at,
			CASE WHEN ad.deployment_id IS NOT NULL THEN 1 ELSE 0 END as is_deployed,
			ad.deployment_id,
			ad.updated_at as deployed_at
		FROM gateways g
		INNER JOIN association_mappings aa ON g.uuid = aa.resource_uuid AND aa.association_type = 'gateway'
		LEFT JOIN deployment_status ad ON g.uuid = ad.gateway_uuid AND ad.artifact_uuid = ? AND ad.status = ?
		WHERE aa.artifact_uuid = ? AND g.organization_uuid = ?
		ORDER BY aa.created_at DESC
	`

	rows, err := r.db.Query(r.db.Rebind(query), apiUUID, string(model.DeploymentStatusDeployed), apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gateways []*model.APIGatewayWithDetails
	for rows.Next() {
		gateway := &model.APIGatewayWithDetails{}
		var propertiesJSON string
		var deployedAt sql.NullTime
		var deploymentId sql.NullString

		err := rows.Scan(
			&gateway.ID,
			&gateway.OrganizationID,
			&gateway.Name,
			&gateway.DisplayName,
			&gateway.Description,
			&propertiesJSON,
			&gateway.Vhost,
			&gateway.IsCritical,
			&gateway.FunctionalityType,
			&gateway.IsActive,
			&gateway.CreatedAt,
			&gateway.UpdatedAt,
			&gateway.AssociatedAt,
			&gateway.AssociationUpdatedAt,
			&gateway.IsDeployed,
			&deploymentId,
			&deployedAt,
		)
		if err != nil {
			return nil, err
		}

		if propertiesJSON != "" && propertiesJSON != "{}" {
			if err := json.Unmarshal([]byte(propertiesJSON), &gateway.Properties); err != nil {
				return nil, fmt.Errorf("failed to unmarshal gateway properties: %w", err)
			}
		}

		if deploymentId.Valid {
			gateway.DeploymentID = &deploymentId.String
		}
		if deployedAt.Valid {
			gateway.DeployedAt = &deployedAt.Time
		}
		gateways = append(gateways, gateway)
	}

	return gateways, rows.Err()
}

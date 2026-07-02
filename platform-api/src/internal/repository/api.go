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
	"strings"
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

	configurationJSON, err := serializeAPIConfigurations(api.Configuration)
	if err != nil {
		return err
	}

	if err := r.artifactRepo.Create(tx, &model.Artifact{
		UUID:             api.ID,
		Type:             constants.RestApi,
		OrganizationUUID: api.OrganizationID,
	}); err != nil {
		return err
	}

	origin := api.Origin
	if origin == "" {
		origin = constants.OriginCP
	}

	apiQuery := `
		INSERT INTO rest_apis (uuid, organization_uuid, handle, display_name, version, description, created_by, project_uuid, lifecycle_status, configuration, origin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = tx.Exec(r.db.Rebind(apiQuery), api.ID, api.OrganizationID, api.Handle, api.Name, api.Version,
		api.Description, api.CreatedBy, api.ProjectID, api.LifeCycleStatus,
		[]byte(configurationJSON), origin, api.CreatedAt, api.UpdatedAt)
	if err != nil {
		return err
	}

	if err := upsertArtifactSecretRefs(tx, r.db, api.OrganizationID, api.ID, []byte(configurationJSON)); err != nil {
		return fmt.Errorf("failed to upsert artifact secret refs: %w", err)
	}

	return tx.Commit()
}

// GetAPIByUUID retrieves an API by UUID with all its configurations
func (r *APIRepo) GetAPIByUUID(apiUUID, orgUUID string) (*model.API, error) {
	api := &model.API{}

	query := `
		SELECT uuid, handle, display_name, description, version, created_by, updated_by,
			project_uuid, organization_uuid, lifecycle_status, configuration, origin, created_at, updated_at
		FROM rest_apis
		WHERE uuid = ? AND organization_uuid = ?
	`

	var configJSON sql.NullString
	var createdBy, updatedBy sql.NullString
	err := r.db.QueryRow(r.db.Rebind(query), apiUUID, orgUUID).Scan(
		&api.ID, &api.Handle, &api.Name, &api.Description,
		&api.Version, &createdBy, &updatedBy, &api.ProjectID, &api.OrganizationID, &api.LifeCycleStatus,
		&configJSON, &api.Origin, &api.CreatedAt, &api.UpdatedAt)
	api.Kind = constants.RestApi
	api.CreatedBy = createdBy.String
	if updatedBy.Valid {
		api.UpdatedBy = updatedBy.String
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if config, err := deserializeAPIConfigurations(configJSON); err != nil {
		return nil, err
	} else if config != nil {
		api.Configuration = *config
	}

	return api, nil
}

// GetAPIsByUUIDs returns a map of API UUID to handle for the given UUIDs in the organization.
// Used for bulk lookup to avoid N+1 queries. Returns empty map for empty input.
func (r *APIRepo) GetAPIsByUUIDs(uuids []string, orgUUID string) (map[string]string, error) {
	if len(uuids) == 0 {
		return map[string]string{}, nil
	}
	placeholders := make([]string, len(uuids))
	args := make([]interface{}, 0, len(uuids)+1)
	for i, u := range uuids {
		placeholders[i] = "?"
		args = append(args, u)
	}
	args = append(args, orgUUID)
	query := fmt.Sprintf(`
		SELECT uuid, handle FROM rest_apis
		WHERE uuid IN (%s) AND organization_uuid = ?
	`, strings.Join(placeholders, ","))
	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var uuid, handle string
		if err := rows.Scan(&uuid, &handle); err != nil {
			return nil, err
		}
		m[uuid] = handle
	}
	return m, rows.Err()
}

// GetAPIMetadataByHandle retrieves minimal API information by handle and organization ID
func (r *APIRepo) GetAPIMetadataByHandle(handle, orgUUID string) (*model.APIMetadata, error) {
	metadata := &model.APIMetadata{}

	query := `
		SELECT uuid, handle, display_name, version, organization_uuid
		FROM rest_apis
		WHERE handle = ? AND organization_uuid = ?
	`

	err := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(
		&metadata.ID, &metadata.Handle, &metadata.Name, &metadata.Version, &metadata.OrganizationID)
	metadata.Kind = constants.RestApi

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
		SELECT uuid, handle, display_name, description, version, created_by, updated_by,
			project_uuid, organization_uuid, lifecycle_status, configuration, origin, created_at, updated_at
		FROM rest_apis
		WHERE project_uuid = ? AND organization_uuid = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(r.db.Rebind(query), projectUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api, err := r.scanAPI(rows)
		if err != nil {
			return nil, err
		}
		apis = append(apis, api)
	}

	return apis, rows.Err()
}

// scanAPI scans a single Rows row into a model.API (no content column).
func (r *APIRepo) scanAPI(rows *sql.Rows) (*model.API, error) {
	api := &model.API{Kind: constants.RestApi}
	var configJSON sql.NullString
	var createdBy, updatedBy sql.NullString
	if err := rows.Scan(
		&api.ID, &api.Handle, &api.Name, &api.Description,
		&api.Version, &createdBy, &updatedBy, &api.ProjectID, &api.OrganizationID,
		&api.LifeCycleStatus, &configJSON, &api.Origin, &api.CreatedAt, &api.UpdatedAt,
	); err != nil {
		return nil, err
	}
	api.CreatedBy = createdBy.String
	if updatedBy.Valid {
		api.UpdatedBy = updatedBy.String
	}
	if config, err := deserializeAPIConfigurations(configJSON); err != nil {
		return nil, err
	} else if config != nil {
		api.Configuration = *config
	}
	return api, nil
}

// GetAPIsByOrganizationUUID retrieves all APIs for an organization with optional project filter
func (r *APIRepo) GetAPIsByOrganizationUUID(orgUUID string, projectUUID string) ([]*model.API, error) {
	var query string
	var args []interface{}

	query = `
		SELECT uuid, handle, display_name, description, version, created_by, updated_by,
			project_uuid, organization_uuid, lifecycle_status, configuration, origin, created_at, updated_at
		FROM rest_apis
		WHERE organization_uuid = ?`
	args = []interface{}{orgUUID}

	if projectUUID != "" {
		query += " AND project_uuid = ?"
		args = append(args, projectUUID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api, err := r.scanAPI(rows)
		if err != nil {
			return nil, err
		}
		apis = append(apis, api)
	}

	return apis, rows.Err()
}

// GetAPIsByGatewayUUID retrieves all APIs associated with a specific gateway
func (r *APIRepo) GetAPIsByGatewayUUID(gatewayUUID, orgUUID string) ([]*model.API, error) {
	query := `
		SELECT a.uuid, a.display_name, a.description, a.version, a.created_by,
			a.project_uuid, a.organization_uuid, a.origin, a.created_at, a.updated_at
		FROM rest_apis a
		INNER JOIN artifact_gateway_mappings aa ON a.uuid = aa.artifact_uuid
		WHERE aa.gateway_uuid = ? AND a.organization_uuid = ?
		ORDER BY a.created_at DESC
	`

	rows, err := r.db.Query(r.db.Rebind(query), gatewayUUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to query APIs associated with gateway: %w", err)
	}
	defer rows.Close()

	var apis []*model.API
	for rows.Next() {
		api := &model.API{Kind: constants.RestApi}
		var createdBy sql.NullString
		if err := rows.Scan(&api.ID, &api.Name, &api.Description,
			&api.Version, &createdBy, &api.ProjectID, &api.OrganizationID,
			&api.Origin, &api.CreatedAt, &api.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan API row: %w", err)
		}
		api.CreatedBy = createdBy.String
		apis = append(apis, api)
	}

	return apis, rows.Err()
}

// UpdateAPI modifies an existing API
func (r *APIRepo) UpdateAPI(api *model.API) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	api.UpdatedAt = time.Now()

	configurationJSON, err := serializeAPIConfigurations(api.Configuration)
	if err != nil {
		return err
	}

	// Update main API record (name and version now live in rest_apis)
	query := `
		UPDATE rest_apis SET display_name = ?, version = ?, description = ?,
			updated_by = ?, lifecycle_status = ?,
			configuration = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err = tx.Exec(r.db.Rebind(query), api.Name, api.Version, api.Description,
		api.UpdatedBy, api.LifeCycleStatus,
		[]byte(configurationJSON), api.UpdatedAt,
		api.ID)
	if err != nil {
		return err
	}

	if err := upsertArtifactSecretRefs(tx, r.db, api.OrganizationID, api.ID, []byte(configurationJSON)); err != nil {
		return fmt.Errorf("failed to upsert artifact secret refs: %w", err)
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

	// Delete gateway associations
	if _, err := tx.Exec(r.db.Rebind(`DELETE FROM artifact_gateway_mappings WHERE artifact_uuid = ? AND organization_uuid = ?`), apiUUID, orgUUID); err != nil {
		return err
	}

	// Delete in order of dependencies (children first, parent last)
	deleteQueries := []string{
		// Delete API deployments
		`DELETE FROM deployments WHERE artifact_uuid = ? AND organization_uuid = ?`,
		// Delete from rest_apis table first, then artifacts
		`DELETE FROM rest_apis WHERE uuid = ?`,
	}

	// Execute all delete statements
	for i, query := range deleteQueries {
		switch i {
		case 0:
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
	var count int
	err := r.db.QueryRow(r.db.Rebind(
		`SELECT COUNT(*) FROM rest_apis WHERE handle = ? AND organization_uuid = ?`),
		handle, orgUUID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
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

func serializeAPIConfigurations(config model.RestAPIConfig) (string, error) {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", err
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
		query = `SELECT COUNT(*) FROM rest_apis WHERE display_name = ? AND version = ? AND organization_uuid = ? AND handle != ?`
		args = []interface{}{name, version, orgUUID, excludeHandle}
	} else {
		query = `SELECT COUNT(*) FROM rest_apis WHERE display_name = ? AND version = ? AND organization_uuid = ?`
		args = []interface{}{name, version, orgUUID}
	}

	var count int
	err := r.db.QueryRow(r.db.Rebind(query), args...).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// CreateAPIAssociation creates a gateway-API association in artifact_gateway_mappings.
func (r *APIRepo) CreateAPIAssociation(association *model.APIAssociation) error {
	query := `
		INSERT INTO artifact_gateway_mappings (artifact_uuid, organization_uuid, gateway_uuid, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query),
		association.ArtifactID, association.OrganizationID, association.GatewayID,
		association.CreatedAt, association.UpdatedAt)
	return err
}

// UpdateAPIAssociation updates the updated_at timestamp for a gateway-API association.
func (r *APIRepo) UpdateAPIAssociation(apiUUID, resourceId, associationType, orgUUID string) error {
	query := `
		UPDATE artifact_gateway_mappings
		SET updated_at = ?
		WHERE artifact_uuid = ? AND gateway_uuid = ? AND organization_uuid = ?
	`
	_, err := r.db.Exec(r.db.Rebind(query), time.Now(), apiUUID, resourceId, orgUUID)
	return err
}

// GetAPIAssociations retrieves all gateway associations for an API.
// associationType is accepted for interface compatibility but only 'gateway' associations are stored.
func (r *APIRepo) GetAPIAssociations(apiUUID, associationType, orgUUID string) ([]*model.APIAssociation, error) {
	query := `
		SELECT artifact_uuid, organization_uuid, gateway_uuid, created_at, updated_at
		FROM artifact_gateway_mappings
		WHERE artifact_uuid = ? AND organization_uuid = ?
	`
	rows, err := r.db.Query(r.db.Rebind(query), apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var associations []*model.APIAssociation
	for rows.Next() {
		assoc := &model.APIAssociation{}
		err := rows.Scan(&assoc.ArtifactID, &assoc.OrganizationID, &assoc.GatewayID, &assoc.CreatedAt, &assoc.UpdatedAt)
		if err != nil {
			return nil, err
		}
		associations = append(associations, assoc)
	}

	return associations, rows.Err()
}

// GetAPIGatewaysWithDetails retrieves all gateways associated with an API including deployment details.
func (r *APIRepo) GetAPIGatewaysWithDetails(apiUUID, orgUUID string) ([]*model.APIGatewayWithDetails, error) {
	query := `
		SELECT
			g.uuid as id,
			g.organization_uuid as organization_id,
			g.display_name,
			g.handle,
			g.description,
			g.properties,
			g.is_critical,
			g.gateway_functionality_type as functionality_type,
			g.is_active,
			g.created_at,
			g.updated_at,
			aa.created_at as associated_at,
			aa.updated_at as association_updated_at,
			CASE WHEN ad.deployment_uuid IS NOT NULL THEN 1 ELSE 0 END as is_deployed,
			ad.deployment_uuid,
			ad.updated_at as deployed_at,
			ge.url
		FROM gateways g
		INNER JOIN artifact_gateway_mappings aa ON g.uuid = aa.gateway_uuid
		LEFT JOIN deployment_status ad ON g.uuid = ad.gateway_uuid AND ad.artifact_uuid = ? AND ad.status = ?
		LEFT JOIN gateway_endpoints ge ON g.uuid = ge.gateway_uuid
		WHERE aa.artifact_uuid = ? AND g.organization_uuid = ?
		ORDER BY aa.created_at DESC, ge.id ASC
	`

	rows, err := r.db.Query(r.db.Rebind(query), apiUUID, string(model.DeploymentStatusDeployed), apiUUID, orgUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gateways []*model.APIGatewayWithDetails
	byID := make(map[string]*model.APIGatewayWithDetails)
	for rows.Next() {
		var propertiesBytes []byte
		var isCritical, isActive int
		var deployedAt sql.NullTime
		var deploymentId, endpointURL sql.NullString
		var id, organizationID, name, handle, description, functionalityType string
		var createdAt, updatedAt, associatedAt, associationUpdatedAt time.Time
		var isDeployed bool

		err := rows.Scan(
			&id,
			&organizationID,
			&name,
			&handle,
			&description,
			&propertiesBytes,
			&isCritical,
			&functionalityType,
			&isActive,
			&createdAt,
			&updatedAt,
			&associatedAt,
			&associationUpdatedAt,
			&isDeployed,
			&deploymentId,
			&deployedAt,
			&endpointURL,
		)
		if err != nil {
			return nil, err
		}

		gateway, ok := byID[id]
		if !ok {
			gateway = &model.APIGatewayWithDetails{
				ID:                   id,
				OrganizationID:       organizationID,
				Name:                 name,
				Handle:               handle,
				Description:          description,
				IsCritical:           isCritical != 0,
				FunctionalityType:    functionalityType,
				IsActive:             isActive != 0,
				CreatedAt:            createdAt,
				UpdatedAt:            updatedAt,
				AssociatedAt:         associatedAt,
				AssociationUpdatedAt: associationUpdatedAt,
				IsDeployed:           isDeployed,
			}
			if len(propertiesBytes) > 0 && string(propertiesBytes) != "{}" {
				if err := json.Unmarshal(propertiesBytes, &gateway.Properties); err != nil {
					return nil, fmt.Errorf("failed to unmarshal gateway properties: %w", err)
				}
			}
			if deploymentId.Valid {
				gateway.DeploymentID = &deploymentId.String
			}
			if deployedAt.Valid {
				gateway.DeployedAt = &deployedAt.Time
			}
			byID[id] = gateway
			gateways = append(gateways, gateway)
		}
		if endpointURL.Valid {
			gateway.Endpoints = append(gateway.Endpoints, endpointURL.String)
		}
	}

	return gateways, rows.Err()
}

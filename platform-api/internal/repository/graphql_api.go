/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// GraphQLAPIRepo handles database operations for GraphQL APIs. GraphQL is a
// core artifact kind (like RestApi/LlmProvider/LlmProxy/Mcp), so this repo
// lives directly alongside api.go/mcp.go rather than in a plugin package.
type GraphQLAPIRepo struct {
	db           *database.DB
	artifactRepo *ArtifactRepo
}

// NewGraphQLAPIRepo creates a new GraphQLAPIRepo instance.
func NewGraphQLAPIRepo(db *database.DB, reg *ArtifactTableRegistry) *GraphQLAPIRepo {
	return &GraphQLAPIRepo{db: db, artifactRepo: NewArtifactRepo(db, reg)}
}

// Create creates a new GraphQL API in the database.
func (r *GraphQLAPIRepo) Create(a *model.GraphQLAPI) error {
	uuidStr, err := utils.GenerateUUID()
	if err != nil {
		return fmt.Errorf("failed to generate GraphQL API ID: %w", err)
	}
	a.ID = uuidStr
	now := time.Now().UTC()
	a.CreatedAt = now
	a.UpdatedAt = now

	configurationJSON, err := serializeGraphQLAPIConfiguration(a.Configuration)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert into artifacts table first.
	if err := r.artifactRepo.Create(tx, &model.Artifact{
		UUID:             a.ID,
		Type:             constants.GraphQLApi,
		OrganizationUUID: a.OrganizationID,
	}); err != nil {
		return fmt.Errorf("failed to create artifact: %w", err)
	}

	origin := a.Origin
	if origin == "" {
		origin = constants.OriginCP
	}

	if a.DataVersion == "" {
		a.DataVersion = string(gatewaytranslator.ComputeDataVersion(constants.GraphQLApi, constants.GatewayApiVersion))
	}

	query := `
		INSERT INTO graphql_apis (
			uuid, organization_uuid, handle, display_name, version, project_uuid, description, created_by, updated_by, configuration, origin, data_version, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(r.db.Rebind(query),
		a.ID, a.OrganizationID, a.Handle, a.Name, a.Version, a.ProjectID, a.Description, a.CreatedBy, a.UpdatedBy,
		configurationJSON, origin, a.DataVersion, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create GraphQL API: %w", err)
	}

	if err := upsertArtifactSecretRefs(tx, r.db, a.OrganizationID, a.ID, configurationJSON); err != nil {
		return fmt.Errorf("failed to upsert artifact secret refs: %w", err)
	}

	return tx.Commit()
}

// GetByHandle retrieves a GraphQL API by its handle and organization UUID.
func (r *GraphQLAPIRepo) GetByHandle(handle, orgUUID string) (*model.GraphQLAPI, error) {
	query := `
		SELECT
			uuid, handle, display_name, version, organization_uuid, origin, created_at, updated_at,
			project_uuid, description, created_by, updated_by, configuration, data_version
		FROM graphql_apis
		WHERE handle = ? AND organization_uuid = ?`
	row := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID)
	return r.scanGraphQLAPI(row)
}

// GetByUUID retrieves a GraphQL API by its UUID and organization UUID.
func (r *GraphQLAPIRepo) GetByUUID(uuid, orgUUID string) (*model.GraphQLAPI, error) {
	query := `
		SELECT
			uuid, handle, display_name, version, organization_uuid, origin, created_at, updated_at,
			project_uuid, description, created_by, updated_by, configuration, data_version
		FROM graphql_apis
		WHERE uuid = ? AND organization_uuid = ?`
	row := r.db.QueryRow(r.db.Rebind(query), uuid, orgUUID)
	return r.scanGraphQLAPI(row)
}

// List retrieves all GraphQL APIs for an organization, optionally filtered by project.
func (r *GraphQLAPIRepo) List(orgUUID, projectUUID string, limit, offset int) ([]*model.GraphQLAPI, error) {
	var query string
	var args []interface{}
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)

	if projectUUID != "" {
		query = `
			SELECT
				uuid, handle, display_name, version, organization_uuid, origin, created_at, updated_at,
				project_uuid, description, created_by, updated_by, configuration, data_version
			FROM graphql_apis
			WHERE organization_uuid = ? AND project_uuid = ?
			ORDER BY created_at DESC
			` + pageClause
		args = append([]interface{}{orgUUID, projectUUID}, pageArgs...)
	} else {
		query = `
			SELECT
				uuid, handle, display_name, version, organization_uuid, origin, created_at, updated_at,
				project_uuid, description, created_by, updated_by, configuration, data_version
			FROM graphql_apis
			WHERE organization_uuid = ?
			ORDER BY created_at DESC
			` + pageClause
		args = append([]interface{}{orgUUID}, pageArgs...)
	}

	rows, err := r.db.Query(r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.GraphQLAPI
	for rows.Next() {
		a, err := r.scanGraphQLAPIFromRows(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, rows.Err()
}

// Count returns the total number of GraphQL APIs for an organization.
func (r *GraphQLAPIRepo) Count(orgUUID string) (int, error) {
	return r.artifactRepo.CountByKindAndOrg(constants.GraphQLApi, orgUUID)
}

// CountByProject returns the total number of GraphQL APIs for a specific project.
func (r *GraphQLAPIRepo) CountByProject(orgUUID, projectUUID string) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM graphql_apis
		WHERE organization_uuid = ? AND project_uuid = ?`
	if err := r.db.QueryRow(r.db.Rebind(query), orgUUID, projectUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Update updates an existing GraphQL API.
func (r *GraphQLAPIRepo) Update(a *model.GraphQLAPI) error {
	now := time.Now().UTC()
	a.UpdatedAt = now

	configurationJSON, err := serializeGraphQLAPIConfiguration(a.Configuration)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var apiUUID string
	query := `
		SELECT uuid FROM graphql_apis
		WHERE handle = ? AND organization_uuid = ?`
	err = tx.QueryRow(r.db.Rebind(query), a.Handle, a.OrganizationID).Scan(&apiUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	if a.DataVersion == "" {
		a.DataVersion = string(gatewaytranslator.ComputeDataVersion(constants.GraphQLApi, constants.GatewayApiVersion))
	}

	query = `
		UPDATE graphql_apis
		SET display_name = ?, version = ?, description = ?, configuration = ?, updated_by = ?, data_version = ?, updated_at = ?
		WHERE uuid = ?`
	result, err := tx.Exec(r.db.Rebind(query),
		a.Name, a.Version, a.Description, configurationJSON, a.UpdatedBy, a.DataVersion, now,
		apiUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to update GraphQL API: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	if err := upsertArtifactSecretRefs(tx, r.db, a.OrganizationID, apiUUID, configurationJSON); err != nil {
		return fmt.Errorf("failed to upsert artifact secret refs: %w", err)
	}

	return tx.Commit()
}

// Delete deletes a GraphQL API by its handle and organization UUID.
func (r *GraphQLAPIRepo) Delete(handle, orgUUID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var apiUUID string
	query := `
		SELECT uuid FROM graphql_apis
		WHERE handle = ? AND organization_uuid = ?`
	err = tx.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(&apiUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	_, err = tx.Exec(r.db.Rebind(`DELETE FROM graphql_apis WHERE uuid = ?`), apiUUID)
	if err != nil {
		return err
	}

	if err := r.artifactRepo.Delete(tx, apiUUID); err != nil {
		return err
	}

	return tx.Commit()
}

// Exists checks if a GraphQL API exists by its handle and organization UUID.
func (r *GraphQLAPIRepo) Exists(handle, orgUUID string) (bool, error) {
	return r.artifactRepo.Exists(constants.GraphQLApi, handle, orgUUID)
}

// scanGraphQLAPI scans a single Row into a GraphQLAPI.
func (r *GraphQLAPIRepo) scanGraphQLAPI(row *sql.Row) (*model.GraphQLAPI, error) {
	var a model.GraphQLAPI
	var createdBy, updatedBy sql.NullString
	var configurationJSON []byte
	if err := row.Scan(
		&a.ID, &a.Handle, &a.Name, &a.Version, &a.OrganizationID, &a.Origin, &a.CreatedAt, &a.UpdatedAt,
		&a.ProjectID, &a.Description, &createdBy, &updatedBy, &configurationJSON, &a.DataVersion,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	a.Kind = constants.GraphQLApi
	a.CreatedBy = createdBy.String
	a.UpdatedBy = updatedBy.String
	if len(configurationJSON) > 0 {
		if config, err := deserializeGraphQLAPIConfiguration(configurationJSON); err != nil {
			return nil, fmt.Errorf("unmarshal configuration for GraphQL API %s: %w", a.Handle, err)
		} else if config != nil {
			a.Configuration = *config
		}
	}
	return &a, nil
}

// scanGraphQLAPIFromRows scans a Rows row into a GraphQLAPI.
func (r *GraphQLAPIRepo) scanGraphQLAPIFromRows(rows *sql.Rows) (*model.GraphQLAPI, error) {
	var a model.GraphQLAPI
	var createdBy, updatedBy sql.NullString
	var configurationJSON []byte
	if err := rows.Scan(
		&a.ID, &a.Handle, &a.Name, &a.Version, &a.OrganizationID, &a.Origin, &a.CreatedAt, &a.UpdatedAt,
		&a.ProjectID, &a.Description, &createdBy, &updatedBy, &configurationJSON, &a.DataVersion,
	); err != nil {
		return nil, err
	}
	a.Kind = constants.GraphQLApi
	a.CreatedBy = createdBy.String
	a.UpdatedBy = updatedBy.String
	if len(configurationJSON) > 0 {
		if config, err := deserializeGraphQLAPIConfiguration(configurationJSON); err != nil {
			return nil, fmt.Errorf("unmarshal configuration for GraphQL API %s: %w", a.Handle, err)
		} else if config != nil {
			a.Configuration = *config
		}
	}
	return &a, nil
}

func serializeGraphQLAPIConfiguration(config model.GraphQLAPIConfig) ([]byte, error) {
	return json.Marshal(config)
}

func deserializeGraphQLAPIConfiguration(configJSON []byte) (*model.GraphQLAPIConfig, error) {
	if len(configJSON) == 0 {
		return nil, fmt.Errorf("null configuration")
	}
	var config model.GraphQLAPIConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetAPIGatewaysWithDetails retrieves all gateways associated with this GraphQL
// API, including deployment details. Delegates to the same kind-agnostic helper
// APIRepo uses — see createArtifactGatewayAssociation's doc comment in
// repository/api.go for why this is shared rather than duplicated SQL.
func (r *GraphQLAPIRepo) GetAPIGatewaysWithDetails(apiUUID, orgUUID string) ([]*model.APIGatewayWithDetails, error) {
	return getArtifactGatewaysWithDetails(r.db, apiUUID, orgUUID)
}

// CreateAPIAssociation creates a gateway-API association for this GraphQL API.
func (r *GraphQLAPIRepo) CreateAPIAssociation(association *model.APIAssociation) error {
	return createArtifactGatewayAssociation(r.db, association)
}

// GetAPIAssociations retrieves all gateway associations for this GraphQL API.
// associationType is accepted for interface compatibility but only 'gateway'
// associations are stored.
func (r *GraphQLAPIRepo) GetAPIAssociations(apiUUID, associationType, orgUUID string) ([]*model.APIAssociation, error) {
	return getArtifactGatewayAssociations(r.db, apiUUID, orgUUID)
}

// UpdateAPIAssociation updates the updated_at timestamp and updated_by actor for a
// gateway-API association.
func (r *GraphQLAPIRepo) UpdateAPIAssociation(apiUUID, resourceId, associationType, orgUUID, updatedBy string) error {
	return updateArtifactGatewayAssociation(r.db, apiUUID, resourceId, orgUUID, updatedBy)
}

// EnsureGatewayAssociation creates a gateway association for the API if one does not
// already exist and resolves the metadata to use for the deployment. See
// ensureArtifactGatewayAssociation (repository/llm.go) for the full semantics —
// LLMProviderRepo/LLMProxyRepo delegate to the exact same helper.
func (r *GraphQLAPIRepo) EnsureGatewayAssociation(apiUUID, gatewayUUID, orgUUID, createdBy, deployMetadata string, metadataProvided bool) (string, error) {
	return ensureArtifactGatewayAssociation(r.db, apiUUID, gatewayUUID, orgUUID, createdBy, deployMetadata, metadataProvided)
}

// Compile-time assertion that GraphQLAPIRepo satisfies GraphQLAPIRepository.
var _ GraphQLAPIRepository = (*GraphQLAPIRepo)(nil)

/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
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

// MCPProxyRepo handles database operations for MCP proxies
type MCPProxyRepo struct {
	db           *database.DB
	artifactRepo *ArtifactRepo
}

// NewMCPProxyRepo creates a new MCPProxyRepo instance
func NewMCPProxyRepo(db *database.DB) *MCPProxyRepo {
	return &MCPProxyRepo{db: db, artifactRepo: NewArtifactRepo(db)}
}

// Create creates a new MCP proxy in the database
func (r *MCPProxyRepo) Create(p *model.MCPProxy) error {
	uuidStr, err := utils.GenerateUUID()
	if err != nil {
		return fmt.Errorf("failed to generate MCP proxy ID: %w", err)
	}
	p.UUID = uuidStr
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now

	configurationJSON, err := serializeMCPProxyConfiguration(p.Configuration)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert into artifacts table first using artifactRepo
	if err := r.artifactRepo.Create(tx, &model.Artifact{
		UUID:             p.UUID,
		Type:             constants.MCPProxy,
		OrganizationUUID: p.OrganizationUUID,
	}); err != nil {
		return fmt.Errorf("failed to create artifact: %w", err)
	}

	origin := p.Origin
	if origin == "" {
		origin = constants.OriginCP
	}

	if p.DataVersion == "" {
		p.DataVersion = string(gatewaytranslator.ComputeDataVersion(constants.MCPProxy, constants.GatewayApiVersion))
	}

	// Insert into mcp_proxies table
	query := `
		INSERT INTO mcp_proxies (
			uuid, handle, display_name, version, project_uuid, description, created_by, updated_by, configuration, origin, data_version, created_at, updated_at, organization_uuid
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(r.db.Rebind(query),
		p.UUID, p.Handle, p.Name, p.Version, p.ProjectUUID, p.Description, p.CreatedBy, p.UpdatedBy, configurationJSON, origin, p.DataVersion, p.CreatedAt, p.UpdatedAt,
		p.OrganizationUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to create MCP proxy: %w", err)
	}

	if err := upsertArtifactSecretRefs(tx, r.db, p.OrganizationUUID, p.UUID, []byte(configurationJSON)); err != nil {
		return fmt.Errorf("failed to upsert artifact secret refs: %w", err)
	}

	// Persist gateway associations (if any) within the same transaction.
	if err := insertArtifactGatewayAssociations(tx, r.db, p.UUID, p.OrganizationUUID, p.AssociatedGateways, now); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// GetByHandle retrieves an MCP proxy by its handle and organization UUID
func (r *MCPProxyRepo) GetByHandle(handle, orgUUID string) (*model.MCPProxy, error) {
	query := `
		SELECT
			uuid, handle, display_name, version, organization_uuid, origin, data_version, created_at, updated_at,
			project_uuid, description, created_by, updated_by, configuration
		FROM mcp_proxies
		WHERE handle = ? AND organization_uuid = ?`
	row := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID)

	var p model.MCPProxy
	var createdBy, updatedBy sql.NullString
	var configurationJSON []byte
	if err := row.Scan(
		&p.UUID, &p.Handle, &p.Name, &p.Version, &p.OrganizationUUID, &p.Origin, &p.DataVersion, &p.CreatedAt, &p.UpdatedAt,
		&p.ProjectUUID, &p.Description, &createdBy, &updatedBy, &configurationJSON,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	p.CreatedBy = createdBy.String
	p.UpdatedBy = updatedBy.String

	if len(configurationJSON) > 0 {
		if config, err := deserializeMCPProxyConfiguration(configurationJSON); err != nil {
			return nil, fmt.Errorf("unmarshal configuration for MCP proxy %s: %w", p.Handle, err)
		} else if config != nil {
			p.Configuration = *config
		}
	}

	associations, err := loadArtifactGatewayAssociations(r.db, p.UUID, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load gateway associations for MCP proxy %s: %w", p.Handle, err)
	}
	p.AssociatedGateways = associations

	return &p, nil
}

// GetByUUID retrieves an MCP proxy by its UUID and organization UUID
func (r *MCPProxyRepo) GetByUUID(uuid, orgUUID string) (*model.MCPProxy, error) {
	query := `
		SELECT
			uuid, handle, display_name, version, organization_uuid, origin, data_version, created_at, updated_at,
			project_uuid, description, created_by, updated_by, configuration
		FROM mcp_proxies
		WHERE uuid = ? AND organization_uuid = ?`
	row := r.db.QueryRow(r.db.Rebind(query), uuid, orgUUID)

	var p model.MCPProxy
	var createdBy, updatedBy sql.NullString
	var configurationJSON []byte
	if err := row.Scan(
		&p.UUID, &p.Handle, &p.Name, &p.Version, &p.OrganizationUUID, &p.Origin, &p.DataVersion, &p.CreatedAt, &p.UpdatedAt,
		&p.ProjectUUID, &p.Description, &createdBy, &updatedBy, &configurationJSON,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	p.CreatedBy = createdBy.String
	p.UpdatedBy = updatedBy.String

	if len(configurationJSON) > 0 {
		if config, err := deserializeMCPProxyConfiguration(configurationJSON); err != nil {
			return nil, fmt.Errorf("unmarshal configuration for MCP proxy %s: %w", p.Handle, err)
		} else if config != nil {
			p.Configuration = *config
		}
	}

	return &p, nil
}

// List retrieves all MCP proxies for an organization
func (r *MCPProxyRepo) List(orgUUID string, limit, offset int) ([]*model.MCPProxy, error) {
	pageClause, pageArgs := r.db.PaginationClause(limit, offset)
	query := `
		SELECT
			uuid, handle, display_name, version, organization_uuid, origin, data_version, created_at, updated_at,
			project_uuid, description, created_by, updated_by, configuration
		FROM mcp_proxies
		WHERE organization_uuid = ?
		ORDER BY created_at DESC
		` + pageClause
	rows, err := r.db.Query(r.db.Rebind(query), append([]any{orgUUID}, pageArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.MCPProxy
	for rows.Next() {
		var p model.MCPProxy
		var createdBy, updatedBy sql.NullString
		var configurationJSON []byte
		err := rows.Scan(
			&p.UUID, &p.Handle, &p.Name, &p.Version, &p.OrganizationUUID, &p.Origin, &p.DataVersion, &p.CreatedAt, &p.UpdatedAt,
			&p.ProjectUUID, &p.Description, &createdBy, &updatedBy, &configurationJSON,
		)
		if err != nil {
			return nil, err
		}
		p.CreatedBy = createdBy.String
		p.UpdatedBy = updatedBy.String
		if len(configurationJSON) > 0 {
			if config, err := deserializeMCPProxyConfiguration(configurationJSON); err != nil {
				return nil, fmt.Errorf("unmarshal configuration for MCP proxy %s: %w", p.Handle, err)
			} else if config != nil {
				p.Configuration = *config
			}
		}
		res = append(res, &p)
	}
	return res, rows.Err()
}

// Count returns the total number of MCP proxies for an organization
func (r *MCPProxyRepo) Count(orgUUID string) (int, error) {
	return r.artifactRepo.CountByKindAndOrg(constants.MCPProxy, orgUUID)
}

// ListByProject retrieves all MCP proxies for a specific project
func (r *MCPProxyRepo) ListByProject(orgUUID, projectUUID string) ([]*model.MCPProxy, error) {
	query := `
		SELECT
			uuid, handle, display_name, version, organization_uuid, origin, data_version, created_at, updated_at,
			project_uuid, description, created_by, updated_by, configuration
		FROM mcp_proxies
		WHERE organization_uuid = ? AND project_uuid = ?
		ORDER BY created_at DESC
		`
	rows, err := r.db.Query(r.db.Rebind(query), orgUUID, projectUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.MCPProxy
	for rows.Next() {
		var p model.MCPProxy
		var createdBy, updatedBy sql.NullString
		var configurationJSON []byte
		err := rows.Scan(
			&p.UUID, &p.Handle, &p.Name, &p.Version, &p.OrganizationUUID, &p.Origin, &p.DataVersion, &p.CreatedAt, &p.UpdatedAt,
			&p.ProjectUUID, &p.Description, &createdBy, &updatedBy, &configurationJSON,
		)
		if err != nil {
			return nil, err
		}
		p.CreatedBy = createdBy.String
		p.UpdatedBy = updatedBy.String
		if len(configurationJSON) > 0 {
			if config, err := deserializeMCPProxyConfiguration(configurationJSON); err != nil {
				return nil, fmt.Errorf("unmarshal configuration for MCP proxy %s: %w", p.Handle, err)
			} else if config != nil {
				p.Configuration = *config
			}
		}
		res = append(res, &p)
	}
	return res, rows.Err()
}

// CountByProject returns the total number of MCP proxies for a specific project
func (r *MCPProxyRepo) CountByProject(orgUUID, projectUUID string) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM mcp_proxies
		WHERE organization_uuid = ? AND project_uuid = ?`
	if err := r.db.QueryRow(r.db.Rebind(query), orgUUID, projectUUID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Update updates an existing MCP proxy
func (r *MCPProxyRepo) Update(p *model.MCPProxy) error {
	now := time.Now().UTC()
	p.UpdatedAt = now

	configurationJSON, err := serializeMCPProxyConfiguration(p.Configuration)
	if err != nil {
		return fmt.Errorf("failed to serialize configuration: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the proxy UUID from handle (and its current data_version, so an
	// unrelated edit that doesn't carry DataVersion forward on the incoming
	// model preserves the stored value instead of blindly recomputing it).
	var proxyUUID, existingDataVersion string
	query := `
		SELECT uuid, data_version FROM mcp_proxies
		WHERE handle = ? AND organization_uuid = ?`
	err = tx.QueryRow(r.db.Rebind(query), p.Handle, p.OrganizationUUID).Scan(&proxyUUID, &existingDataVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	if p.DataVersion == "" {
		if existingDataVersion != "" {
			p.DataVersion = existingDataVersion
		} else {
			p.DataVersion = string(gatewaytranslator.ComputeDataVersion(constants.MCPProxy, constants.GatewayApiVersion))
		}
	}

	// Update mcp_proxies table (name/version/updated_at now live here)
	query = `
		UPDATE mcp_proxies
		SET display_name = ?, version = ?, description = ?, configuration = ?, updated_by = ?, data_version = ?, updated_at = ?
		WHERE uuid = ?`
	result, err := tx.Exec(r.db.Rebind(query),
		p.Name, p.Version, p.Description, configurationJSON, p.UpdatedBy, p.DataVersion, now,
		proxyUUID,
	)
	if err != nil {
		return fmt.Errorf("failed to update MCP proxy: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	if err := upsertArtifactSecretRefs(tx, r.db, p.OrganizationUUID, proxyUUID, []byte(configurationJSON)); err != nil {
		return fmt.Errorf("failed to upsert artifact secret refs: %w", err)
	}

	// Replace the full set of gateway associations within the same transaction when the
	// caller manages associations. Deployments are intentionally left untouched.
	if p.ReplaceAssociatedGateways {
		if err := replaceArtifactGatewayAssociations(tx, r.db, proxyUUID, p.OrganizationUUID, p.AssociatedGateways, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// EnsureGatewayAssociation creates a gateway association for the MCP proxy if one does not
// already exist and resolves the metadata to use for the deployment. See
// ensureArtifactGatewayAssociation for the full semantics.
func (r *MCPProxyRepo) EnsureGatewayAssociation(proxyUUID, gatewayUUID, orgUUID, deployMetadata string, metadataProvided bool) (string, error) {
	return ensureArtifactGatewayAssociation(r.db, proxyUUID, gatewayUUID, orgUUID, deployMetadata, metadataProvided)
}

// Delete deletes an MCP proxy by its handle and organization UUID
func (r *MCPProxyRepo) Delete(handle, orgUUID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the proxy UUID from handle
	var proxyUUID string
	query := `
		SELECT uuid FROM mcp_proxies
		WHERE handle = ? AND organization_uuid = ?`
	err = tx.QueryRow(r.db.Rebind(query), handle, orgUUID).Scan(&proxyUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	// Delete from mcp_proxies first, then artifacts using artifactRepo
	_, err = tx.Exec(r.db.Rebind(`DELETE FROM mcp_proxies WHERE uuid = ?`), proxyUUID)
	if err != nil {
		return err
	}

	if err := r.artifactRepo.Delete(tx, proxyUUID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// Exists checks if an MCP proxy exists by its handle and organization UUID
func (r *MCPProxyRepo) Exists(handle, orgUUID string) (bool, error) {
	return r.artifactRepo.Exists(constants.MCPProxy, handle, orgUUID)
}

func serializeMCPProxyConfiguration(config model.MCPProxyConfiguration) ([]byte, error) {
	return json.Marshal(config)
}

func deserializeMCPProxyConfiguration(configJSON []byte) (*model.MCPProxyConfiguration, error) {
	if len(configJSON) == 0 {
		return nil, fmt.Errorf("null configuration")
	}
	var config model.MCPProxyConfiguration
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

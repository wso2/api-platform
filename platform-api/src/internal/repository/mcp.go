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

	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"
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
	now := time.Now()
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
		Handle:           p.Handle,
		Name:             p.Name,
		Version:          p.Version,
		Kind:             constants.MCPProxy,
		OrganizationUUID: p.OrganizationUUID,
	}); err != nil {
		return fmt.Errorf("failed to create artifact: %w", err)
	}

	// Insert into mcp_proxies table
	query := `
		INSERT INTO mcp_proxies (
			uuid, project_uuid, description, created_by, status, configuration
		)
		VALUES (?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(r.db.Rebind(query),
		p.UUID, p.ProjectUUID, p.Description, p.CreatedBy, p.Status, configurationJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to create MCP proxy: %w", err)
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
			a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
			p.project_uuid, p.description, p.created_by, p.status, p.configuration
		FROM artifacts a
		JOIN mcp_proxies p ON a.uuid = p.uuid
		WHERE a.handle = ? AND a.organization_uuid = ? AND a.kind = ?`
	row := r.db.QueryRow(r.db.Rebind(query), handle, orgUUID, constants.MCPProxy)

	var p model.MCPProxy
	var configurationJSON sql.NullString
	if err := row.Scan(
		&p.UUID, &p.Handle, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
		&p.ProjectUUID, &p.Description, &p.CreatedBy, &p.Status, &configurationJSON,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if configurationJSON.Valid && configurationJSON.String != "" {
		if config, err := deserializeMCPProxyConfiguration(configurationJSON); err != nil {
			return nil, fmt.Errorf("unmarshal configuration for MCP proxy %s: %w", p.Handle, err)
		} else if config != nil {
			p.Configuration = *config
		}
	}

	return &p, nil
}

// GetByUUID retrieves an MCP proxy by its UUID and organization UUID
func (r *MCPProxyRepo) GetByUUID(uuid, orgUUID string) (*model.MCPProxy, error) {
	query := `
		SELECT
			a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
			p.project_uuid, p.description, p.created_by, p.status, p.configuration
		FROM artifacts a
		JOIN mcp_proxies p ON a.uuid = p.uuid
		WHERE a.uuid = ? AND a.organization_uuid = ? AND a.kind = ?`
	row := r.db.QueryRow(r.db.Rebind(query), uuid, orgUUID, constants.MCPProxy)

	var p model.MCPProxy
	var configurationJSON sql.NullString
	if err := row.Scan(
		&p.UUID, &p.Handle, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
		&p.ProjectUUID, &p.Description, &p.CreatedBy, &p.Status, &configurationJSON,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if configurationJSON.Valid && configurationJSON.String != "" {
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
	query := `
		SELECT
			a.uuid, a.handle, a.name, a.version, a.organization_uuid, a.created_at, a.updated_at,
			p.project_uuid, p.description, p.created_by, p.status, p.configuration
		FROM artifacts a
		JOIN mcp_proxies p ON a.uuid = p.uuid
		WHERE a.organization_uuid = ? AND a.kind = ?
		ORDER BY a.created_at DESC
		LIMIT ? OFFSET ?`
	rows, err := r.db.Query(r.db.Rebind(query), orgUUID, constants.MCPProxy, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.MCPProxy
	for rows.Next() {
		var p model.MCPProxy
		var configurationJSON sql.NullString
		err := rows.Scan(
			&p.UUID, &p.Handle, &p.Name, &p.Version, &p.OrganizationUUID, &p.CreatedAt, &p.UpdatedAt,
			&p.ProjectUUID, &p.Description, &p.CreatedBy, &p.Status, &configurationJSON,
		)
		if err != nil {
			return nil, err
		}
		if configurationJSON.Valid && configurationJSON.String != "" {
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
	var count int
	query := `
		SELECT COUNT(*) FROM artifacts a
		JOIN mcp_proxies p ON a.uuid = p.uuid
		WHERE a.organization_uuid = ? AND a.kind = ?`
	if err := r.db.QueryRow(r.db.Rebind(query), orgUUID, constants.MCPProxy).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Update updates an existing MCP proxy
func (r *MCPProxyRepo) Update(p *model.MCPProxy) error {
	now := time.Now()
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

	// Get the proxy UUID from handle
	var proxyUUID string
	query := `
		SELECT uuid FROM artifacts
		WHERE handle = ? AND organization_uuid = ? AND kind = ?`
	err = tx.QueryRow(r.db.Rebind(query), p.Handle, p.OrganizationUUID, constants.MCPProxy).Scan(&proxyUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	// Update artifacts table
	if err := r.artifactRepo.Update(tx, &model.Artifact{
		UUID:             proxyUUID,
		Name:             p.Name,
		Version:          p.Version,
		OrganizationUUID: p.OrganizationUUID,
		UpdatedAt:        now,
	}); err != nil {
		return fmt.Errorf("failed to update artifact: %w", err)
	}

	// Update mcp_proxies table
	query = `
		UPDATE mcp_proxies
		SET description = ?, configuration = ?, status = ?
		WHERE uuid = ?`
	result, err := tx.Exec(r.db.Rebind(query),
		p.Description, configurationJSON, p.Status,
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
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
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
		SELECT uuid FROM artifacts
		WHERE handle = ? AND organization_uuid = ? AND kind = ?`
	err = tx.QueryRow(r.db.Rebind(query), handle, orgUUID, constants.MCPProxy).Scan(&proxyUUID)
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

// serializeMCPProxyConfiguration serializes the MCP proxy configuration to JSON string
func serializeMCPProxyConfiguration(config model.MCPProxyConfiguration) (string, error) {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(configJSON), nil
}

// deserializeMCPProxyConfiguration deserializes the JSON string to MCP proxy configuration
func deserializeMCPProxyConfiguration(configJSON sql.NullString) (*model.MCPProxyConfiguration, error) {
	if !configJSON.Valid || configJSON.String == "" {
		return nil, fmt.Errorf("null configuration")
	}
	var config model.MCPProxyConfiguration
	if err := json.Unmarshal([]byte(configJSON.String), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

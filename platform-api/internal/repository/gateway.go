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

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// GatewayRepo implements GatewayRepository
type GatewayRepo struct {
	db *database.DB
}

// NewGatewayRepo creates a new gateway repository
func NewGatewayRepo(db *database.DB) GatewayRepository {
	return &GatewayRepo{db: db}
}

// Create inserts a new gateway
func (r *GatewayRepo) Create(gateway *model.Gateway) error {
	gateway.CreatedAt = time.Now()
	gateway.UpdatedAt = time.Now()
	gateway.IsActive = false // Set default value to false at registration

	// Serialize properties to JSON bytes (BYTEA/BLOB column)
	var propertiesBytes []byte
	if gateway.Properties != nil {
		var err error
		propertiesBytes, err = json.Marshal(gateway.Properties)
		if err != nil {
			return fmt.Errorf("failed to marshal properties: %w", err)
		}
	} else {
		propertiesBytes = []byte("{}")
	}

	// Convert bools to ints for SMALLINT/INTEGER columns
	isCriticalInt := 0
	if gateway.IsCritical {
		isCriticalInt = 1
	}
	isActiveInt := 0
	if gateway.IsActive {
		isActiveInt = 1
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO gateways (uuid, organization_uuid, handle, display_name, description, properties, is_critical,
		                      gateway_functionality_type, version, is_active, created_by, updated_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := tx.Exec(r.db.Rebind(query), gateway.ID, gateway.OrganizationID, gateway.Handle, gateway.Name,
		gateway.Description, propertiesBytes, isCriticalInt, gateway.FunctionalityType, gateway.Version,
		isActiveInt, gateway.CreatedBy, gateway.UpdatedBy, gateway.CreatedAt, gateway.UpdatedAt); err != nil {
		return err
	}

	endpointQuery := `INSERT INTO gateway_endpoints (gateway_uuid, url) VALUES (?, ?)`
	for _, endpoint := range gateway.Endpoints {
		if _, err := tx.Exec(r.db.Rebind(endpointQuery), gateway.ID, endpoint); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// gatewaySelectColumns are the gateways columns selected by the join queries below, in scan order.
const gatewaySelectColumns = `
	g.uuid, g.organization_uuid, g.handle, g.display_name, g.description, g.properties,
	g.is_critical, g.gateway_functionality_type, g.version, g.is_active,
	g.created_by, g.updated_by, g.created_at, g.updated_at, ge.url
`

// scanGatewayJoinRow scans one row of a gateways LEFT JOIN gateway_endpoints result.
// The endpoint url is nullable since a gateway row may have no matching endpoint rows.
func scanGatewayJoinRow(rows *sql.Rows) (*model.Gateway, sql.NullString, error) {
	gateway := &model.Gateway{}
	var propertiesBytes []byte
	var isCritical, isActive int
	var createdBy, updatedBy, url sql.NullString
	if err := rows.Scan(
		&gateway.ID, &gateway.OrganizationID, &gateway.Handle, &gateway.Name, &gateway.Description, &propertiesBytes,
		&isCritical, &gateway.FunctionalityType, &gateway.Version, &isActive, &createdBy, &updatedBy,
		&gateway.CreatedAt, &gateway.UpdatedAt, &url,
	); err != nil {
		return nil, sql.NullString{}, err
	}
	gateway.IsCritical = isCritical != 0
	gateway.IsActive = isActive != 0
	gateway.CreatedBy = createdBy.String
	gateway.UpdatedBy = updatedBy.String
	if len(propertiesBytes) > 0 && string(propertiesBytes) != "{}" {
		if err := json.Unmarshal(propertiesBytes, &gateway.Properties); err != nil {
			return nil, sql.NullString{}, fmt.Errorf("failed to unmarshal properties: %w", err)
		}
	}
	return gateway, url, nil
}

// aggregateGatewayJoinRows folds a gateways LEFT JOIN gateway_endpoints result set (one row per
// endpoint, or a single row with a NULL url if a gateway has none) into one *model.Gateway per
// distinct gateway, preserving the order gateways first appear in and collecting their endpoints.
func aggregateGatewayJoinRows(rows *sql.Rows) ([]*model.Gateway, error) {
	var gateways []*model.Gateway
	byID := make(map[string]*model.Gateway)
	for rows.Next() {
		gateway, url, err := scanGatewayJoinRow(rows)
		if err != nil {
			return nil, err
		}
		existing, ok := byID[gateway.ID]
		if !ok {
			existing = gateway
			byID[gateway.ID] = existing
			gateways = append(gateways, existing)
		}
		if url.Valid {
			existing.Endpoints = append(existing.Endpoints, url.String)
		}
	}
	return gateways, rows.Err()
}

// GetByUUID retrieves a gateway by ID
func (r *GatewayRepo) GetByUUID(gatewayId string) (*model.Gateway, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM gateways g
		LEFT JOIN gateway_endpoints ge ON ge.gateway_uuid = g.uuid
		WHERE g.uuid = ?
		ORDER BY ge.id ASC
	`, gatewaySelectColumns)
	rows, err := r.db.Query(r.db.Rebind(query), gatewayId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	gateways, err := aggregateGatewayJoinRows(rows)
	if err != nil {
		return nil, err
	}
	if len(gateways) == 0 {
		return nil, nil
	}
	return gateways[0], nil
}

// GetByOrganizationID retrieves all gateways for an organization
func (r *GatewayRepo) GetByOrganizationID(orgID string) ([]*model.Gateway, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM gateways g
		LEFT JOIN gateway_endpoints ge ON ge.gateway_uuid = g.uuid
		WHERE g.organization_uuid = ?
		ORDER BY g.created_at DESC, ge.id ASC
	`, gatewaySelectColumns)
	rows, err := r.db.Query(r.db.Rebind(query), orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return aggregateGatewayJoinRows(rows)
}

// GetByHandleAndOrgID checks if a gateway with the given handle exists within an organization
func (r *GatewayRepo) GetByHandleAndOrgID(handle, orgID string) (*model.Gateway, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM gateways g
		LEFT JOIN gateway_endpoints ge ON ge.gateway_uuid = g.uuid
		WHERE g.handle = ? AND g.organization_uuid = ?
		ORDER BY ge.id ASC
	`, gatewaySelectColumns)
	rows, err := r.db.Query(r.db.Rebind(query), handle, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	gateways, err := aggregateGatewayJoinRows(rows)
	if err != nil {
		return nil, err
	}
	if len(gateways) == 0 {
		return nil, nil
	}
	return gateways[0], nil
}

// List retrieves all gateways
func (r *GatewayRepo) List() ([]*model.Gateway, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM gateways g
		LEFT JOIN gateway_endpoints ge ON ge.gateway_uuid = g.uuid
		ORDER BY g.created_at DESC, ge.id ASC
	`, gatewaySelectColumns)
	rows, err := r.db.Query(r.db.Rebind(query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return aggregateGatewayJoinRows(rows)
}

// Delete removes a gateway with organization isolation and cleans up all associations
func (r *GatewayRepo) Delete(gatewayID, organizationID string) error {
	// Start transaction for atomicity
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	deleteAssocQuery := `DELETE FROM artifact_gateway_mappings WHERE gateway_uuid = ? AND organization_uuid = ?`
	_, err = tx.Exec(r.db.Rebind(deleteAssocQuery), gatewayID, organizationID)
	if err != nil {
		return err
	}

	// Delete gateway with organization isolation (gateway_tokens and deployments will be cascade deleted via FK)
	deleteGatewayQuery := `DELETE FROM gateways WHERE uuid = ? AND organization_uuid = ?`
	result, err := tx.Exec(r.db.Rebind(deleteGatewayQuery), gatewayID, organizationID)
	if err != nil {
		return err
	}

	// Check if gateway was actually deleted
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		// Gateway not found (or belongs to different organization)
		return errors.New("gateway not found")
	}

	// Commit transaction
	return tx.Commit()
}

// UpdateGateway updates gateway details
func (r *GatewayRepo) UpdateGateway(gateway *model.Gateway) error {
	var propertiesBytes []byte
	if gateway.Properties != nil {
		var err error
		propertiesBytes, err = json.Marshal(gateway.Properties)
		if err != nil {
			return fmt.Errorf("failed to marshal properties: %w", err)
		}
	} else {
		propertiesBytes = []byte("{}")
	}

	isCriticalInt := 0
	if gateway.IsCritical {
		isCriticalInt = 1
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		UPDATE gateways
		SET display_name = ?, description = ?, is_critical = ?, properties = ?, updated_by = ?, updated_at = ?
		WHERE uuid = ?
	`
	if _, err := tx.Exec(r.db.Rebind(query), gateway.Name, gateway.Description, isCriticalInt, propertiesBytes, gateway.UpdatedBy, gateway.UpdatedAt, gateway.ID); err != nil {
		return err
	}

	if _, err := tx.Exec(r.db.Rebind(`DELETE FROM gateway_endpoints WHERE gateway_uuid = ?`), gateway.ID); err != nil {
		return err
	}
	endpointQuery := `INSERT INTO gateway_endpoints (gateway_uuid, url) VALUES (?, ?)`
	for _, endpoint := range gateway.Endpoints {
		if _, err := tx.Exec(r.db.Rebind(endpointQuery), gateway.ID, endpoint); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateActiveStatus updates the is_active status of a gateway
func (r *GatewayRepo) UpdateActiveStatus(gatewayId string, isActive bool) error {
	isActiveInt := 0
	if isActive {
		isActiveInt = 1
	}
	query := `
		UPDATE gateways
		SET is_active = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(r.db.Rebind(query), isActiveInt, time.Now(), gatewayId)
	return err
}

// CreateToken inserts a new token
func (r *GatewayRepo) CreateToken(token *model.GatewayToken) error {
	token.CreatedAt = time.Now()

	query := `
		INSERT INTO gateway_tokens (uuid, gateway_uuid, token_hash, salt, status, created_by, created_at, revoked_by, revoked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query), token.ID, token.GatewayID, token.TokenHash, token.Salt, token.Status,
		token.CreatedBy, token.CreatedAt, token.RevokedBy, token.RevokedAt)
	return err
}

// GetActiveTokensByGatewayUUID retrieves all active tokens for a gateway
func (r *GatewayRepo) GetActiveTokensByGatewayUUID(gatewayId string) ([]*model.GatewayToken, error) {
	query := `
		SELECT uuid, gateway_uuid, token_hash, salt, status, created_by, created_at, revoked_by, revoked_at
		FROM gateway_tokens
		WHERE gateway_uuid = ? AND status = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(r.db.Rebind(query), gatewayId, constants.GatewayTokenStatusActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*model.GatewayToken
	for rows.Next() {
		token := &model.GatewayToken{}
		var createdBy sql.NullString
		var revokedBy sql.NullString
		err := rows.Scan(
			&token.ID, &token.GatewayID, &token.TokenHash, &token.Salt, &token.Status,
			&createdBy, &token.CreatedAt, &revokedBy, &token.RevokedAt,
		)
		if err != nil {
			return nil, err
		}
		if createdBy.Valid {
			token.CreatedBy = createdBy.String
		}
		if revokedBy.Valid {
			token.RevokedBy = &revokedBy.String
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

// GetActiveTokenByHash retrieves an active token by its hash
func (r *GatewayRepo) GetActiveTokenByHash(tokenHash string) (*model.GatewayToken, error) {
	token := &model.GatewayToken{}
	var createdBy sql.NullString
	var revokedBy sql.NullString
	query := `
		SELECT uuid, gateway_uuid, token_hash, salt, status, created_by, created_at, revoked_by, revoked_at
		FROM gateway_tokens
		WHERE token_hash = ? AND status = ?
		ORDER BY (SELECT NULL)
		` + r.db.FetchFirstClause(1)
	err := r.db.QueryRow(r.db.Rebind(query), tokenHash, constants.GatewayTokenStatusActive).Scan(
		&token.ID, &token.GatewayID, &token.TokenHash, &token.Salt, &token.Status,
		&createdBy, &token.CreatedAt, &revokedBy, &token.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if createdBy.Valid {
		token.CreatedBy = createdBy.String
	}
	if revokedBy.Valid {
		token.RevokedBy = &revokedBy.String
	}
	return token, nil
}

// GetTokenByUUID retrieves a specific token by UUID
func (r *GatewayRepo) GetTokenByUUID(tokenId string) (*model.GatewayToken, error) {
	token := &model.GatewayToken{}
	var createdBy sql.NullString
	var revokedBy sql.NullString
	query := `
		SELECT uuid, gateway_uuid, token_hash, salt, status, created_by, created_at, revoked_by, revoked_at
		FROM gateway_tokens
		WHERE uuid = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), tokenId).Scan(
		&token.ID, &token.GatewayID, &token.TokenHash, &token.Salt, &token.Status,
		&createdBy, &token.CreatedAt, &revokedBy, &token.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if createdBy.Valid {
		token.CreatedBy = createdBy.String
	}
	if revokedBy.Valid {
		token.RevokedBy = &revokedBy.String
	}
	return token, nil
}

// RevokeToken updates token status to revoked
func (r *GatewayRepo) RevokeToken(tokenId, revokedBy string) error {
	now := time.Now()
	var revokedByVal interface{}
	if revokedBy != "" {
		revokedByVal = revokedBy
	}
	query := `
		UPDATE gateway_tokens
		SET status = ?, revoked_by = ?, revoked_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(r.db.Rebind(query), constants.GatewayTokenStatusRevoked, revokedByVal, now, tokenId)
	return err
}

// CountActiveTokens counts the number of active tokens for a gateway
func (r *GatewayRepo) CountActiveTokens(gatewayId string) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM gateway_tokens
		WHERE gateway_uuid = ? AND status = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), gatewayId, constants.GatewayTokenStatusActive).Scan(&count)
	return count, err
}

// HasGatewayDeployments checks if a gateway has any deployments
func (r *GatewayRepo) HasGatewayDeployments(gatewayID, organizationID string) (bool, error) {
	var deploymentCount int
	deploymentQuery := `SELECT COUNT(*)
		FROM deployment_status s
		WHERE s.gateway_uuid = ? AND s.organization_uuid = ? AND s.status = ?`
	err := r.db.QueryRow(r.db.Rebind(deploymentQuery), gatewayID, organizationID, string(model.DeploymentStatusDeployed)).Scan(&deploymentCount)
	if err != nil {
		return false, err
	}

	return deploymentCount > 0, nil
}

// HasGatewayAssociations checks if a gateway has any associations
func (r *GatewayRepo) HasGatewayAssociations(gatewayID, organizationID string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM artifact_gateway_mappings WHERE gateway_uuid = ? AND organization_uuid = ?`
	err := r.db.QueryRow(r.db.Rebind(query), gatewayID, organizationID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// HasGatewayAssociationsOrDeployments checks if a gateway has any associations (deployments or associations)
func (r *GatewayRepo) HasGatewayAssociationsOrDeployments(gatewayID, organizationID string) (bool, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM (
			SELECT 1 FROM deployment_status
			WHERE gateway_uuid = ? AND organization_uuid = ? AND status = ?
			UNION ALL
			SELECT 1 FROM artifact_gateway_mappings
			WHERE gateway_uuid = ? AND organization_uuid = ?
		) combined`
	err := r.db.QueryRow(r.db.Rebind(query), gatewayID, organizationID, string(model.DeploymentStatusDeployed), gatewayID, organizationID).Scan(&count)
	return count > 0, err
}

// UpdateGatewayManifest persists the gateway manifest JSON to the gateway row.
func (r *GatewayRepo) UpdateGatewayManifest(gatewayID string, manifest []byte) error {
	query := `UPDATE gateways SET manifest = ? WHERE uuid = ?`
	_, err := r.db.Exec(r.db.Rebind(query), manifest, gatewayID)
	return err
}

// UpdateGatewayVersion persists the version string reported by the gateway controller on manifest push.
func (r *GatewayRepo) UpdateGatewayVersion(gatewayID, version string) error {
	query := `UPDATE gateways SET version = ?, updated_at = ? WHERE uuid = ?`
	_, err := r.db.Exec(r.db.Rebind(query), version, time.Now(), gatewayID)
	return err
}

// GetGatewayManifest returns the raw manifest JSON stored for the gateway.
// Returns nil data (no error) if the gateway exists but has no manifest yet.
// Returns an error if the gateway row does not exist.
func (r *GatewayRepo) GetGatewayManifest(gatewayID string) ([]byte, error) {
	query := `SELECT manifest FROM gateways WHERE uuid = ?`
	var raw []byte
	err := r.db.QueryRow(r.db.Rebind(query), gatewayID).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("gateway not found: %s", gatewayID)
		}
		return nil, err
	}
	return raw, nil
}

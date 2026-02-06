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

	"platform-api/src/internal/database"
	"platform-api/src/internal/model"
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

	// Serialize properties to JSON
	var propertiesJSON string
	if gateway.Properties != nil {
		jsonBytes, err := json.Marshal(gateway.Properties)
		if err != nil {
			return fmt.Errorf("failed to marshal properties: %w", err)
		}
		propertiesJSON = string(jsonBytes)
	} else {
		propertiesJSON = "{}"
	}

	query := `
		INSERT INTO gateways (uuid, organization_uuid, name, display_name, description, properties, vhost, is_critical,
		                      gateway_functionality_type, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query), gateway.ID, gateway.OrganizationID, gateway.Name, gateway.DisplayName,
		gateway.Description, propertiesJSON, gateway.Vhost, gateway.IsCritical, gateway.FunctionalityType, gateway.IsActive,
		gateway.CreatedAt, gateway.UpdatedAt)
	return err
}

// GetByUUID retrieves a gateway by ID
func (r *GatewayRepo) GetByUUID(gatewayId string) (*model.Gateway, error) {
	gateway := &model.Gateway{}
	var propertiesJSON string
	query := `
		SELECT uuid, organization_uuid, name, display_name, description, properties, vhost, is_critical, gateway_functionality_type, is_active,
		       created_at, updated_at
		FROM gateways
		WHERE uuid = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), gatewayId).Scan(
		&gateway.ID, &gateway.OrganizationID, &gateway.Name, &gateway.DisplayName, &gateway.Description, &propertiesJSON, &gateway.Vhost,
		&gateway.IsCritical, &gateway.FunctionalityType, &gateway.IsActive, &gateway.CreatedAt, &gateway.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Deserialize properties from JSON
	if propertiesJSON != "" && propertiesJSON != "{}" {
		if err := json.Unmarshal([]byte(propertiesJSON), &gateway.Properties); err != nil {
			return nil, fmt.Errorf("failed to unmarshal properties: %w", err)
		}
	}

	return gateway, nil
}

// GetByOrganizationID retrieves all gateways for an organization
func (r *GatewayRepo) GetByOrganizationID(orgID string) ([]*model.Gateway, error) {
	query := `
		SELECT uuid, organization_uuid, name, display_name, description, properties, vhost, is_critical, gateway_functionality_type, is_active,
		       created_at, updated_at
		FROM gateways
		WHERE organization_uuid = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(r.db.Rebind(query), orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gateways []*model.Gateway
	for rows.Next() {
		gateway := &model.Gateway{}
		var propertiesJSON string
		err := rows.Scan(
			&gateway.ID, &gateway.OrganizationID, &gateway.Name, &gateway.DisplayName, &gateway.Description, &propertiesJSON, &gateway.Vhost,
			&gateway.IsCritical, &gateway.FunctionalityType, &gateway.IsActive, &gateway.CreatedAt, &gateway.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Deserialize properties from JSON
		if propertiesJSON != "" && propertiesJSON != "{}" {
			if err := json.Unmarshal([]byte(propertiesJSON), &gateway.Properties); err != nil {
				return nil, fmt.Errorf("failed to unmarshal properties: %w", err)
			}
		}

		gateways = append(gateways, gateway)
	}
	return gateways, nil
}

// GetByNameAndOrgID checks if a gateway with the given name exists within an organization
func (r *GatewayRepo) GetByNameAndOrgID(name, orgID string) (*model.Gateway, error) {
	gateway := &model.Gateway{}
	var propertiesJSON string
	query := `
		SELECT uuid, organization_uuid, name, display_name, description, properties, vhost, is_critical, gateway_functionality_type, is_active,
		       created_at, updated_at
		FROM gateways
		WHERE name = ? AND organization_uuid = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), name, orgID).Scan(
		&gateway.ID, &gateway.OrganizationID, &gateway.Name, &gateway.DisplayName, &gateway.Description, &propertiesJSON, &gateway.Vhost,
		&gateway.IsCritical, &gateway.FunctionalityType, &gateway.IsActive, &gateway.CreatedAt, &gateway.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Deserialize properties from JSON
	if propertiesJSON != "" && propertiesJSON != "{}" {
		if err := json.Unmarshal([]byte(propertiesJSON), &gateway.Properties); err != nil {
			return nil, fmt.Errorf("failed to unmarshal properties: %w", err)
		}
	}

	return gateway, nil
}

// List retrieves all gateways
func (r *GatewayRepo) List() ([]*model.Gateway, error) {
	query := `
		SELECT uuid, organization_uuid, name, display_name, description, properties, vhost, is_critical, gateway_functionality_type, is_active,
		       created_at, updated_at
		FROM gateways
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(r.db.Rebind(query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gateways []*model.Gateway
	for rows.Next() {
		gateway := &model.Gateway{}
		var propertiesJSON string
		err := rows.Scan(
			&gateway.ID, &gateway.OrganizationID, &gateway.Name, &gateway.DisplayName, &gateway.Description, &propertiesJSON, &gateway.Vhost,
			&gateway.IsCritical, &gateway.FunctionalityType, &gateway.IsActive, &gateway.CreatedAt, &gateway.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Deserialize properties from JSON
		if propertiesJSON != "" && propertiesJSON != "{}" {
			if err := json.Unmarshal([]byte(propertiesJSON), &gateway.Properties); err != nil {
				return nil, fmt.Errorf("failed to unmarshal properties: %w", err)
			}
		}

		gateways = append(gateways, gateway)
	}
	return gateways, nil
}

// Delete removes a gateway with organization isolation and cleans up all associations
func (r *GatewayRepo) Delete(gatewayID, organizationID string) error {
	// Start transaction for atomicity
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete API associations for this gateway
	deleteAssocQuery := `DELETE FROM api_associations 
	                     WHERE resource_uuid = ? AND association_type = 'gateway' AND organization_uuid = ?`
	_, err = tx.Exec(r.db.Rebind(deleteAssocQuery), gatewayID, organizationID)
	if err != nil {
		return err
	}

	// Delete gateway with organization isolation (gateway_tokens and api_deployments will be cascade deleted via FK)
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
	// Serialize properties to JSON
	var propertiesJSON string
	if gateway.Properties != nil {
		jsonBytes, err := json.Marshal(gateway.Properties)
		if err != nil {
			return fmt.Errorf("failed to marshal properties: %w", err)
		}
		propertiesJSON = string(jsonBytes)
	} else {
		propertiesJSON = "{}"
	}

	query := `
		UPDATE gateways
		SET display_name = ?, description = ?, is_critical = ?, properties = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(r.db.Rebind(query), gateway.DisplayName, gateway.Description, gateway.IsCritical, propertiesJSON, gateway.UpdatedAt, gateway.ID)
	return err
}

// UpdateActiveStatus updates the is_active status of a gateway
func (r *GatewayRepo) UpdateActiveStatus(gatewayId string, isActive bool) error {
	query := `
		UPDATE gateways
		SET is_active = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(r.db.Rebind(query), isActive, time.Now(), gatewayId)
	return err
}

// CreateToken inserts a new token
func (r *GatewayRepo) CreateToken(token *model.GatewayToken) error {
	token.CreatedAt = time.Now()

	query := `
		INSERT INTO gateway_tokens (uuid, gateway_uuid, token_hash, salt, status, created_at, revoked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(r.db.Rebind(query), token.ID, token.GatewayID, token.TokenHash, token.Salt, token.Status, token.CreatedAt,
		token.RevokedAt)
	return err
}

// GetActiveTokensByGatewayUUID retrieves all active tokens for a gateway
func (r *GatewayRepo) GetActiveTokensByGatewayUUID(gatewayId string) ([]*model.GatewayToken, error) {
	query := `
		SELECT uuid, gateway_uuid, token_hash, salt, status, created_at, revoked_at
		FROM gateway_tokens
		WHERE gateway_uuid = ? AND status = 'active'
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(r.db.Rebind(query), gatewayId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*model.GatewayToken
	for rows.Next() {
		token := &model.GatewayToken{}
		err := rows.Scan(
			&token.ID, &token.GatewayID, &token.TokenHash, &token.Salt, &token.Status, &token.CreatedAt, &token.RevokedAt,
		)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

// GetTokenByUUID retrieves a specific token by UUID
func (r *GatewayRepo) GetTokenByUUID(tokenId string) (*model.GatewayToken, error) {
	token := &model.GatewayToken{}
	query := `
		SELECT uuid, gateway_uuid, token_hash, salt, status, created_at, revoked_at
		FROM gateway_tokens
		WHERE uuid = ?
	`
	err := r.db.QueryRow(r.db.Rebind(query), tokenId).Scan(
		&token.ID, &token.GatewayID, &token.TokenHash, &token.Salt, &token.Status, &token.CreatedAt, &token.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return token, nil
}

// RevokeToken updates token status to revoked
func (r *GatewayRepo) RevokeToken(tokenId string) error {
	now := time.Now()
	query := `
		UPDATE gateway_tokens
		SET status = 'revoked', revoked_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(r.db.Rebind(query), now, tokenId)
	return err
}

// CountActiveTokens counts the number of active tokens for a gateway
func (r *GatewayRepo) CountActiveTokens(gatewayId string) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM gateway_tokens
		WHERE gateway_uuid = ? AND status = 'active'
	`
	err := r.db.QueryRow(r.db.Rebind(query), gatewayId).Scan(&count)
	return count, err
}

// HasGatewayAPIDeployments checks if a gateway has any API deployments
func (r *GatewayRepo) HasGatewayAPIDeployments(gatewayID, organizationID string) (bool, error) {
	var deploymentCount int
	deploymentQuery := `SELECT COUNT(*) FROM api_deployments WHERE gateway_uuid = ? AND organization_uuid = ?`
	err := r.db.QueryRow(r.db.Rebind(deploymentQuery), gatewayID, organizationID).Scan(&deploymentCount)
	if err != nil {
		return false, err
	}

	return deploymentCount > 0, nil
}

// HasGatewayAPIAssociations checks if a gateway has any API associations
func (r *GatewayRepo) HasGatewayAPIAssociations(gatewayID, organizationID string) (bool, error) {
	var associationCount int
	associationQuery := `SELECT COUNT(*) FROM api_associations WHERE resource_uuid = ? AND association_type = 'gateway' AND organization_uuid = ?`
	err := r.db.QueryRow(r.db.Rebind(associationQuery), gatewayID, organizationID).Scan(&associationCount)
	if err != nil {
		return false, err
	}

	return associationCount > 0, nil
}

// HasGatewayAssociations checks if a gateway has any API associations (deployments or associations)
func (r *GatewayRepo) HasGatewayAssociations(gatewayID, organizationID string) (bool, error) {
	// Check deployments first
	hasDeployments, err := r.HasGatewayAPIDeployments(gatewayID, organizationID)
	if err != nil {
		return false, err
	}

	if hasDeployments {
		return true, nil
	}

	// Check associations
	return r.HasGatewayAPIAssociations(gatewayID, organizationID)
}

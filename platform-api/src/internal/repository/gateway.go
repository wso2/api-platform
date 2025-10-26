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

	query := `
		INSERT INTO gateways (uuid, organization_uuid, name, display_name, description, vhost, is_critical, is_ai_gateway, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query, gateway.ID, gateway.OrganizationID, gateway.Name, gateway.DisplayName,
		gateway.Description, gateway.Vhost, gateway.IsCritical, gateway.IsAIGateway, gateway.CreatedAt, gateway.UpdatedAt)
	return err
}

// GetByUUID retrieves a gateway by ID
func (r *GatewayRepo) GetByUUID(gatewayId string) (*model.Gateway, error) {
	gateway := &model.Gateway{}
	query := `
		SELECT uuid, organization_uuid, name, display_name, description, vhost, is_critical, is_ai_gateway, created_at, updated_at
		FROM gateways
		WHERE uuid = ?
	`
	err := r.db.QueryRow(query, gatewayId).Scan(
		&gateway.ID, &gateway.OrganizationID, &gateway.Name, &gateway.DisplayName, &gateway.Description, &gateway.Vhost,
		&gateway.IsCritical, &gateway.IsAIGateway, &gateway.CreatedAt, &gateway.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return gateway, nil
}

// GetByOrganizationID retrieves all gateways for an organization
func (r *GatewayRepo) GetByOrganizationID(orgID string) ([]*model.Gateway, error) {
	query := `
		SELECT uuid, organization_uuid, name, display_name, description, vhost, is_critical, is_ai_gateway, created_at, updated_at
		FROM gateways
		WHERE organization_uuid = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gateways []*model.Gateway
	for rows.Next() {
		gateway := &model.Gateway{}
		err := rows.Scan(
			&gateway.ID, &gateway.OrganizationID, &gateway.Name, &gateway.DisplayName, &gateway.Description, &gateway.Vhost,
			&gateway.IsCritical, &gateway.IsAIGateway, &gateway.CreatedAt, &gateway.UpdatedAt)
		if err != nil {
			return nil, err
		}
		gateways = append(gateways, gateway)
	}
	return gateways, nil
}

// GetByNameAndOrgID checks if a gateway with the given name exists within an organization
func (r *GatewayRepo) GetByNameAndOrgID(name, orgID string) (*model.Gateway, error) {
	gateway := &model.Gateway{}
	query := `
		SELECT uuid, organization_uuid, name, display_name, description, vhost, is_critical, is_ai_gateway, created_at, updated_at
		FROM gateways
		WHERE name = ? AND organization_uuid = ?
	`
	err := r.db.QueryRow(query, name, orgID).Scan(
		&gateway.ID, &gateway.OrganizationID, &gateway.Name, &gateway.DisplayName, &gateway.Description, &gateway.Vhost,
		&gateway.IsCritical, &gateway.IsAIGateway, &gateway.CreatedAt, &gateway.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return gateway, nil
}

// List retrieves all gateways
func (r *GatewayRepo) List() ([]*model.Gateway, error) {
	query := `
		SELECT uuid, organization_uuid, name, display_name, description, vhost, is_critical, is_ai_gateway, created_at, updated_at
		FROM gateways
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gateways []*model.Gateway
	for rows.Next() {
		gateway := &model.Gateway{}
		err := rows.Scan(
			&gateway.ID, &gateway.OrganizationID, &gateway.Name, &gateway.DisplayName, &gateway.Description, &gateway.Vhost,
			&gateway.IsCritical, &gateway.IsAIGateway, &gateway.CreatedAt, &gateway.UpdatedAt)
		if err != nil {
			return nil, err
		}
		gateways = append(gateways, gateway)
	}
	return gateways, nil
}

// Delete removes a gateway with organization isolation (cascade deletes tokens via FK)
func (r *GatewayRepo) Delete(gatewayID, organizationID string) error {
	// Start transaction for atomicity
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete gateway with organization isolation
	query := `DELETE FROM gateways WHERE uuid = ? AND organization_uuid = ?`
	result, err := tx.Exec(query, gatewayID, organizationID)
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
	query := `
		UPDATE gateways
		SET display_name = ?, description = ?, is_critical = ?, updated_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(query, gateway.DisplayName, gateway.Description, gateway.IsCritical, gateway.UpdatedAt, gateway.ID)
	return err
}

// CreateToken inserts a new token
func (r *GatewayRepo) CreateToken(token *model.GatewayToken) error {
	token.CreatedAt = time.Now()

	query := `
		INSERT INTO gateway_tokens (uuid, gateway_uuid, token_hash, salt, status, created_at, revoked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query, token.ID, token.GatewayID, token.TokenHash, token.Salt, token.Status, token.CreatedAt, token.RevokedAt)
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
	rows, err := r.db.Query(query, gatewayId)
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
	err := r.db.QueryRow(query, tokenId).Scan(
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
	_, err := r.db.Exec(query, now, tokenId)
	return err
}

// CountActiveTokens counts the number of active tokens for a gateway
func (r *GatewayRepo) CountActiveTokens(gatewayId string) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM gateway_tokens
		WHERE gateway_uuid = ? AND status = 'active'
	`
	err := r.db.QueryRow(query, gatewayId).Scan(&count)
	return count, err
}

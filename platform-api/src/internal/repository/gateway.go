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
		INSERT INTO gateways (uuid, organization_id, name, display_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query, gateway.ID, gateway.OrganizationID, gateway.Name, gateway.DisplayName, gateway.CreatedAt, gateway.UpdatedAt)
	return err
}

// GetByUUID retrieves a gateway by UUID
func (r *GatewayRepo) GetByUUID(uuid string) (*model.Gateway, error) {
	gateway := &model.Gateway{}
	query := `
		SELECT uuid, organization_id, name, display_name, created_at, updated_at
		FROM gateways
		WHERE uuid = ?
	`
	err := r.db.QueryRow(query, uuid).Scan(
		&gateway.ID, &gateway.OrganizationID, &gateway.Name, &gateway.DisplayName, &gateway.CreatedAt, &gateway.UpdatedAt,
	)
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
		SELECT uuid, organization_id, name, display_name, created_at, updated_at
		FROM gateways
		WHERE organization_id = ?
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
			&gateway.ID, &gateway.OrganizationID, &gateway.Name, &gateway.DisplayName, &gateway.CreatedAt, &gateway.UpdatedAt,
		)
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
		SELECT uuid, organization_id, name, display_name, created_at, updated_at
		FROM gateways
		WHERE name = ? AND organization_id = ?
	`
	err := r.db.QueryRow(query, name, orgID).Scan(
		&gateway.ID, &gateway.OrganizationID, &gateway.Name, &gateway.DisplayName, &gateway.CreatedAt, &gateway.UpdatedAt,
	)
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
		SELECT uuid, organization_id, name, display_name, created_at, updated_at
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
			&gateway.ID, &gateway.OrganizationID, &gateway.Name, &gateway.DisplayName, &gateway.CreatedAt, &gateway.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		gateways = append(gateways, gateway)
	}
	return gateways, nil
}

// Delete removes a gateway (cascade deletes tokens via FK)
func (r *GatewayRepo) Delete(uuid string) error {
	query := `DELETE FROM gateways WHERE uuid = ?`
	_, err := r.db.Exec(query, uuid)
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
func (r *GatewayRepo) GetActiveTokensByGatewayUUID(gatewayUUID string) ([]*model.GatewayToken, error) {
	query := `
		SELECT uuid, gateway_uuid, token_hash, salt, status, created_at, revoked_at
		FROM gateway_tokens
		WHERE gateway_uuid = ? AND status = 'active'
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(query, gatewayUUID)
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
func (r *GatewayRepo) GetTokenByUUID(tokenUUID string) (*model.GatewayToken, error) {
	token := &model.GatewayToken{}
	query := `
		SELECT uuid, gateway_uuid, token_hash, salt, status, created_at, revoked_at
		FROM gateway_tokens
		WHERE uuid = ?
	`
	err := r.db.QueryRow(query, tokenUUID).Scan(
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
func (r *GatewayRepo) RevokeToken(tokenUUID string) error {
	now := time.Now()
	query := `
		UPDATE gateway_tokens
		SET status = 'revoked', revoked_at = ?
		WHERE uuid = ?
	`
	_, err := r.db.Exec(query, now, tokenUUID)
	return err
}

// CountActiveTokens counts the number of active tokens for a gateway
func (r *GatewayRepo) CountActiveTokens(gatewayUUID string) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM gateway_tokens
		WHERE gateway_uuid = ? AND status = 'active'
	`
	err := r.db.QueryRow(query, gatewayUUID).Scan(&count)
	return count, err
}
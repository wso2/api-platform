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

package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// GatewayService handles gateway business logic
type GatewayService struct {
	gatewayRepo repository.GatewayRepository
	orgRepo     repository.OrganizationRepository
}

// NewGatewayService creates a new gateway service
func NewGatewayService(gatewayRepo repository.GatewayRepository, orgRepo repository.OrganizationRepository) *GatewayService {
	return &GatewayService{
		gatewayRepo: gatewayRepo,
		orgRepo:     orgRepo,
	}
}

// RegisterGateway registers a new gateway with organization validation
func (s *GatewayService) RegisterGateway(orgID, name, displayName string) (*dto.GatewayResponse, error) {
	// 1. Validate inputs
	if err := s.validateGatewayInput(orgID, name, displayName); err != nil {
		return nil, err
	}

	// 2. Validate organization exists
	org, err := s.orgRepo.GetOrganizationByUUID(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query organization: %w", err)
	}
	if org == nil {
		return nil, errors.New("organization not found")
	}

	// 3. Check gateway name uniqueness within organization
	existing, err := s.gatewayRepo.GetByNameAndOrgID(name, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to check gateway name uniqueness: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("gateway with name '%s' already exists in this organization", name)
	}

	// 4. Generate UUID for gateway
	gatewayUUID := uuid.New().String()

	// 5. Create Gateway model
	gateway := &model.Gateway{
		ID:           gatewayUUID,
		OrganizationID: orgID,
		Name:           name,
		DisplayName:    displayName,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// 6. Generate plain-text token and salt
	plainToken, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	saltBytes, err := generateSalt()
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// 7. Hash token with salt
	tokenHash := hashToken(plainToken, saltBytes)
	saltHex := hex.EncodeToString(saltBytes)

	// 8. Create GatewayToken model
	tokenUUID := uuid.New().String()
	gatewayToken := &model.GatewayToken{
		ID:        tokenUUID,
		GatewayID: gatewayUUID,
		TokenHash:   tokenHash,
		Salt:        saltHex,
		Status:      "active",
		CreatedAt:   time.Now(),
		RevokedAt:   nil,
	}

	// 9. Insert gateway and token (in sequence - repository handles this)
	if err := s.gatewayRepo.Create(gateway); err != nil {
		return nil, fmt.Errorf("failed to create gateway: %w", err)
	}

	if err := s.gatewayRepo.CreateToken(gatewayToken); err != nil {
		// Note: In production, this should be wrapped in a transaction
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	// 10. Return GatewayResponse with gateway details
	response := &dto.GatewayResponse{
		ID:           gateway.ID,
		OrganizationID: gateway.OrganizationID,
		Name:           gateway.Name,
		DisplayName:    gateway.DisplayName,
		CreatedAt:      gateway.CreatedAt,
		UpdatedAt:      gateway.UpdatedAt,
	}

	return response, nil
}

// ListGateways retrieves all gateways with constitution-compliant envelope structure
func (s *GatewayService) ListGateways(orgID *string) (*dto.GatewayListResponse, error) {
	var gateways []*model.Gateway
	var err error

	// If orgID provided and non-empty, filter by organization
	if orgID != nil && *orgID != "" {
		gateways, err = s.gatewayRepo.GetByOrganizationID(*orgID)
	} else {
		gateways, err = s.gatewayRepo.List()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list gateways: %w", err)
	}

	// Convert to DTOs
	responses := make([]dto.GatewayResponse, 0, len(gateways))
	for _, gw := range gateways {
		responses = append(responses, dto.GatewayResponse{
			ID:           gw.ID,
			OrganizationID: gw.OrganizationID,
			Name:           gw.Name,
			DisplayName:    gw.DisplayName,
			CreatedAt:      gw.CreatedAt,
			UpdatedAt:      gw.UpdatedAt,
		})
	}

	// Build constitution-compliant list response with pagination metadata
	listResponse := &dto.GatewayListResponse{
		Count: len(responses),
		List:  responses,
		Pagination: dto.PaginationInfo{
			Total:  len(responses), // For now, total equals count (no pagination yet)
			Offset: 0,              // Starting from first item
			Limit:  len(responses), // Returning all items
		},
	}

	return listResponse, nil
}

// GetGateway retrieves a gateway by UUID
func (s *GatewayService) GetGateway(gatewayUUID string) (*dto.GatewayResponse, error) {
	// Validate UUID format
	if _, err := uuid.Parse(gatewayUUID); err != nil {
		return nil, errors.New("invalid UUID format")
	}

	gateway, err := s.gatewayRepo.GetByUUID(gatewayUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}

	if gateway == nil {
		return nil, errors.New("gateway not found")
	}

	response := &dto.GatewayResponse{
		ID:           gateway.ID,
		OrganizationID: gateway.OrganizationID,
		Name:           gateway.Name,
		DisplayName:    gateway.DisplayName,
		CreatedAt:      gateway.CreatedAt,
		UpdatedAt:      gateway.UpdatedAt,
	}

	return response, nil
}

// VerifyToken verifies a plain-text token and returns the associated gateway
func (s *GatewayService) VerifyToken(plainToken string) (*model.Gateway, error) {
	if plainToken == "" {
		return nil, errors.New("token is required")
	}

	// Get all gateways to check their active tokens
	gateways, err := s.gatewayRepo.List()
	if err != nil {
		return nil, fmt.Errorf("failed to query gateways: %w", err)
	}

	// For each gateway, check if the token matches any active token
	for _, gateway := range gateways {
		activeTokens, err := s.gatewayRepo.GetActiveTokensByGatewayUUID(gateway.ID)
		if err != nil {
			continue // Skip this gateway on error
		}

		for _, token := range activeTokens {
			if verifyToken(plainToken, token.TokenHash, token.Salt) {
				// Token matches - return gateway
				return gateway, nil
			}
		}
	}

	return nil, errors.New("invalid token")
}

// RotateToken generates a new token for a gateway (max 2 active tokens)
func (s *GatewayService) RotateToken(gatewayUUID string) (*dto.TokenRotationResponse, error) {
	// 1. Validate gateway exists
	gateway, err := s.gatewayRepo.GetByUUID(gatewayUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to query gateway: %w", err)
	}
	if gateway == nil {
		return nil, errors.New("gateway not found")
	}

	// 2. Count active tokens
	activeCount, err := s.gatewayRepo.CountActiveTokens(gatewayUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to count active tokens: %w", err)
	}

	// 3. Check max 2 active tokens limit
	if activeCount >= 2 {
		return nil, errors.New("maximum 2 active tokens allowed. Revoke old tokens before rotating")
	}

	// 4. Generate new plain-text token and salt
	plainToken, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	saltBytes, err := generateSalt()
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// 5. Hash new token
	tokenHash := hashToken(plainToken, saltBytes)
	saltHex := hex.EncodeToString(saltBytes)

	// 6. Create new GatewayToken model with status='active'
	tokenUUID := uuid.New().String()
	gatewayToken := &model.GatewayToken{
		ID:        tokenUUID,
		GatewayID: gatewayUUID,
		TokenHash:   tokenHash,
		Salt:        saltHex,
		Status:      "active",
		CreatedAt:   time.Now(),
		RevokedAt:   nil,
	}

	// 7. Insert token using repository
	if err := s.gatewayRepo.CreateToken(gatewayToken); err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	// 8. Return TokenRotationResponse with token UUID, plain-text token, timestamp, and message
	response := &dto.TokenRotationResponse{
		TokenID: tokenUUID,
		Token:     plainToken,
		CreatedAt: gatewayToken.CreatedAt,
		Message:   "New token generated successfully. Old token remains active until revoked.",
	}

	return response, nil
}

// validateGatewayInput validates gateway registration inputs
func (s *GatewayService) validateGatewayInput(orgID, name, displayName string) error {
	// Organization ID validation
	if strings.TrimSpace(orgID) == "" {
		return errors.New("organization ID is required")
	}
	if _, err := uuid.Parse(orgID); err != nil {
		return errors.New("invalid organization ID format")
	}

	// Gateway name validation
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("gateway name is required")
	}
	if len(name) < 3 {
		return errors.New("gateway name must be at least 3 characters")
	}
	if len(name) > 64 {
		return errors.New("gateway name must not exceed 64 characters")
	}

	// Check pattern: ^[a-z0-9-]+$
	namePattern := regexp.MustCompile(`^[a-z0-9-]+$`)
	if !namePattern.MatchString(name) {
		return errors.New("gateway name must contain only lowercase letters, numbers, and hyphens")
	}

	// No leading/trailing hyphens
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		return errors.New("gateway name cannot start or end with a hyphen")
	}

	// Display name validation
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return errors.New("display name is required")
	}
	if len(displayName) > 128 {
		return errors.New("display name must not exceed 128 characters")
	}

	return nil
}

// Token Generation and Hashing Utilities

// generateToken generates a cryptographically secure 32-byte random token, base64-encoded
func generateToken() (string, error) {
	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		return "", errors.New("failed to generate secure random token")
	}
	token := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(tokenBytes)
	return token, nil
}

// generateSalt generates a cryptographically secure 32-byte random salt
func generateSalt() ([]byte, error) {
	salt := make([]byte, 32)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, errors.New("failed to generate secure random salt")
	}
	return salt, nil
}

// hashToken computes SHA-256 hash of (token + salt) and returns hex-encoded string
func hashToken(plainToken string, salt []byte) string {
	h := sha256.New()
	h.Write([]byte(plainToken))
	h.Write(salt)
	tokenHash := h.Sum(nil)
	return hex.EncodeToString(tokenHash)
}

// verifyToken performs constant-time comparison of plain token against stored hash+salt
func verifyToken(plainToken string, storedHashHex string, storedSaltHex string) bool {
	storedSalt, err := hex.DecodeString(storedSaltHex)
	if err != nil {
		return false
	}
	storedHash, err := hex.DecodeString(storedHashHex)
	if err != nil {
		return false
	}
	h := sha256.New()
	h.Write([]byte(plainToken))
	h.Write(storedSalt)
	computedHash := h.Sum(nil)
	return subtle.ConstantTimeCompare(computedHash, storedHash) == 1
}

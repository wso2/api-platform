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
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GatewayService handles gateway business logic
type GatewayService struct {
	gatewayRepo repository.GatewayRepository
	orgRepo     repository.OrganizationRepository
	apiRepo     repository.APIRepository
}

// NewGatewayService creates a new gateway service
func NewGatewayService(gatewayRepo repository.GatewayRepository, orgRepo repository.OrganizationRepository,
	apiRepo repository.APIRepository) *GatewayService {
	return &GatewayService{
		gatewayRepo: gatewayRepo,
		orgRepo:     orgRepo,
		apiRepo:     apiRepo,
	}
}

// RegisterGateway registers a new gateway with organization validation
func (s *GatewayService) RegisterGateway(orgID, name, displayName, description, vhost string, isCritical bool,
	functionalityType string) (*dto.GatewayResponse, error) {
	// 1. Validate inputs
	if err := s.validateGatewayInput(orgID, name, displayName, vhost, functionalityType); err != nil {
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
	gatewayId := uuid.New().String()

	// 5. Create Gateway model
	gateway := &model.Gateway{
		ID:                gatewayId,
		OrganizationID:    orgID,
		Name:              name,
		DisplayName:       displayName,
		Description:       description,
		Vhost:             vhost,
		IsCritical:        isCritical,
		FunctionalityType: functionalityType,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
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
	tokenId := uuid.New().String()
	gatewayToken := &model.GatewayToken{
		ID:        tokenId,
		GatewayID: gatewayId,
		TokenHash: tokenHash,
		Salt:      saltHex,
		Status:    "active",
		CreatedAt: time.Now(),
		RevokedAt: nil,
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
		ID:                gateway.ID,
		OrganizationID:    gateway.OrganizationID,
		Name:              gateway.Name,
		DisplayName:       gateway.DisplayName,
		Description:       gateway.Description,
		Vhost:             gateway.Vhost,
		IsCritical:        gateway.IsCritical,
		FunctionalityType: gateway.FunctionalityType,
		IsActive:          gateway.IsActive,
		CreatedAt:         gateway.CreatedAt,
		UpdatedAt:         gateway.UpdatedAt,
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
			ID:                gw.ID,
			OrganizationID:    gw.OrganizationID,
			Name:              gw.Name,
			DisplayName:       gw.DisplayName,
			Description:       gw.Description,
			Vhost:             gw.Vhost,
			IsCritical:        gw.IsCritical,
			FunctionalityType: gw.FunctionalityType,
			IsActive:          gw.IsActive,
			CreatedAt:         gw.CreatedAt,
			UpdatedAt:         gw.UpdatedAt,
		})
	}

	// Build constitution-compliant list response with pagination metadata
	listResponse := &dto.GatewayListResponse{
		Count: len(responses),
		List:  responses,
		Pagination: dto.Pagination{
			Total:  len(responses), // For now, total equals count (no pagination yet)
			Offset: 0,              // Starting from first item
			Limit:  len(responses), // Returning all items
		},
	}

	return listResponse, nil
}

// GetGateway retrieves a gateway by ID
func (s *GatewayService) GetGateway(gatewayId, orgId string) (*dto.GatewayResponse, error) {
	// Validate UUID format
	if _, err := uuid.Parse(gatewayId); err != nil {
		return nil, errors.New("invalid UUID format")
	}

	gateway, err := s.gatewayRepo.GetByUUID(gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}

	if gateway == nil {
		return nil, errors.New("gateway not found")
	}

	if gateway.OrganizationID != orgId {
		return nil, errors.New("gateway not found")
	}

	response := &dto.GatewayResponse{
		ID:                gateway.ID,
		OrganizationID:    gateway.OrganizationID,
		Name:              gateway.Name,
		DisplayName:       gateway.DisplayName,
		Description:       gateway.Description,
		Vhost:             gateway.Vhost,
		IsCritical:        gateway.IsCritical,
		FunctionalityType: gateway.FunctionalityType,
		IsActive:          gateway.IsActive,
		CreatedAt:         gateway.CreatedAt,
		UpdatedAt:         gateway.UpdatedAt,
	}

	return response, nil
}

// UpdateGateway updates gateway details
func (s *GatewayService) UpdateGateway(gatewayId, orgId string, description, displayName *string,
	isCritical *bool) (*dto.GatewayResponse, error) {
	// Get existing gateway
	gateway, err := s.gatewayRepo.GetByUUID(gatewayId)
	if err != nil {
		return nil, err
	}
	if gateway == nil {
		return nil, constants.ErrGatewayNotFound
	}
	if gateway.OrganizationID != orgId {
		return nil, constants.ErrGatewayNotFound
	}

	if description != nil {
		gateway.Description = *description
	}
	if displayName != nil {
		gateway.DisplayName = *displayName
	}
	if isCritical != nil {
		gateway.IsCritical = *isCritical
	}
	gateway.UpdatedAt = time.Now()

	err = s.gatewayRepo.UpdateGateway(gateway)
	if err != nil {
		return nil, err
	}

	updatedGateway := &dto.GatewayResponse{
		ID:                gateway.ID,
		OrganizationID:    gateway.OrganizationID,
		Name:              gateway.Name,
		DisplayName:       gateway.DisplayName,
		Description:       gateway.Description,
		Vhost:             gateway.Vhost,
		IsCritical:        gateway.IsCritical,
		FunctionalityType: gateway.FunctionalityType,
		IsActive:          gateway.IsActive,
		CreatedAt:         gateway.CreatedAt,
		UpdatedAt:         gateway.UpdatedAt,
	}
	return updatedGateway, nil
}

// DeleteGateway deletes a gateway and all associated tokens (CASCADE)
func (s *GatewayService) DeleteGateway(gatewayID, orgID string) error {
	// Validate UUID format
	if _, err := uuid.Parse(gatewayID); err != nil {
		return errors.New("invalid UUID format")
	}

	// Verify gateway exists and belongs to organization
	gateway, err := s.gatewayRepo.GetByUUID(gatewayID)
	if err != nil {
		return err
	}
	if gateway == nil {
		return constants.ErrGatewayNotFound
	}
	if gateway.OrganizationID != orgID {
		// Return same error for both "not found" and "wrong organization" (security through obscurity)
		return constants.ErrGatewayNotFound
	}

	// Check if there are any API associations with this gateway
	hasAssociations, err := s.gatewayRepo.HasGatewayAssociations(gatewayID, orgID)
	if err != nil {
		return fmt.Errorf("failed to check gateway associations: %w", err)
	}

	if hasAssociations {
		return constants.ErrGatewayHasAssociatedAPIs
	}

	// Delete gateway (CASCADE will remove tokens automatically, api_associations cleanup handled by repository)
	err = s.gatewayRepo.Delete(gatewayID, orgID)
	if err != nil {
		return err
	}

	return nil
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
func (s *GatewayService) RotateToken(gatewayId, orgId string) (*dto.TokenRotationResponse, error) {
	// 1. Validate gateway exists
	gateway, err := s.gatewayRepo.GetByUUID(gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to query gateway: %w", err)
	}
	if gateway == nil {
		return nil, errors.New("gateway not found")
	}
	if gateway.OrganizationID != orgId {
		return nil, errors.New("gateway not found")
	}

	// 2. Count active tokens
	activeCount, err := s.gatewayRepo.CountActiveTokens(gatewayId)
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
	tokenId := uuid.New().String()
	gatewayToken := &model.GatewayToken{
		ID:        tokenId,
		GatewayID: gatewayId,
		TokenHash: tokenHash,
		Salt:      saltHex,
		Status:    "active",
		CreatedAt: time.Now(),
		RevokedAt: nil,
	}

	// 7. Insert token using repository
	if err := s.gatewayRepo.CreateToken(gatewayToken); err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	// 8. Return TokenRotationResponse with token UUID, plain-text token, timestamp, and message
	response := &dto.TokenRotationResponse{
		ID:        tokenId,
		Token:     plainToken,
		CreatedAt: gatewayToken.CreatedAt,
		Message:   "New token generated successfully. Old token remains active until revoked.",
	}

	return response, nil
}

// GetGatewayStatus retrieves gateway status information for polling
func (s *GatewayService) GetGatewayStatus(orgID string, gatewayId *string) (*dto.GatewayStatusListResponse, error) {
	// Validate organizationId is provided and valid
	if strings.TrimSpace(orgID) == "" {
		return nil, errors.New("organization ID is required")
	}

	var gateways []*model.Gateway
	var err error

	// If gatewayId is provided, get specific gateway
	if gatewayId != nil && *gatewayId != "" {
		gateway, err := s.gatewayRepo.GetByUUID(*gatewayId)
		if err != nil {
			return nil, fmt.Errorf("failed to get gateway: %w", err)
		}
		if gateway == nil {
			return nil, errors.New("gateway not found")
		}
		// Check organization access
		if gateway.OrganizationID != orgID {
			return nil, errors.New("gateway not found")
		}
		gateways = []*model.Gateway{gateway}
	} else {
		// Get all gateways for organization
		gateways, err = s.gatewayRepo.GetByOrganizationID(orgID)
		if err != nil {
			return nil, fmt.Errorf("failed to list gateways: %w", err)
		}
	}

	// Convert to lightweight status DTOs
	statusResponses := make([]dto.GatewayStatusResponse, 0, len(gateways))
	for _, gw := range gateways {
		statusResponses = append(statusResponses, dto.GatewayStatusResponse{
			ID:         gw.ID,
			Name:       gw.Name,
			IsActive:   gw.IsActive,
			IsCritical: gw.IsCritical,
		})
	}

	// Build constitution-compliant list response
	listResponse := &dto.GatewayStatusListResponse{
		Count: len(statusResponses),
		List:  statusResponses,
		Pagination: dto.Pagination{
			Total:  len(statusResponses),
			Offset: 0,
			Limit:  len(statusResponses),
		},
	}

	return listResponse, nil
}

// UpdateGatewayActiveStatus updates the active status of a gateway
func (s *GatewayService) UpdateGatewayActiveStatus(gatewayId string, isActive bool) error {
	return s.gatewayRepo.UpdateActiveStatus(gatewayId, isActive)
}

// GetGatewayArtifacts retrieves all artifacts (APIs) deployed to a specific gateway with pagination and optional type filtering
func (s *GatewayService) GetGatewayArtifacts(gatewayID, orgID, artifactType string) (*dto.GatewayArtifactListResponse, error) {
	// First validate that the gateway exists and belongs to the organization
	gateway, err := s.gatewayRepo.GetByUUID(gatewayID)
	if err != nil {
		return nil, err
	}
	if gateway == nil {
		return nil, constants.ErrGatewayNotFound
	}
	if gateway.OrganizationID != orgID {
		return nil, constants.ErrGatewayNotFound
	}

	// Get all APIs deployed to this gateway
	apis, err := s.apiRepo.GetDeployedAPIsByGatewayID(gatewayID, orgID)
	if err != nil {
		return nil, err
	}

	// Convert APIs to GatewayArtifact DTOs and apply type filtering
	allArtifacts := make([]dto.GatewayArtifact, 0)
	for _, api := range apis {
		// Skip if artifactType filter is specified and doesn't match "API"
		if artifactType != "" && artifactType != "API" {
			continue
		}

		// Determine API subtype based on the type field using APIUtil
		apiUtil := &utils.APIUtil{}
		subType := apiUtil.GetAPISubType(api.Type)

		artifact := dto.GatewayArtifact{
			ID:          api.ID,
			Name:        api.Name,
			DisplayName: api.DisplayName,
			Type:        "API",
			SubType:     subType,
			CreatedAt:   api.CreatedAt,
			UpdatedAt:   api.UpdatedAt,
		}
		allArtifacts = append(allArtifacts, artifact)
	}

	// If filtering by MCP or API_PRODUCT, return empty list for now (future implementation)
	if artifactType != "" && (artifactType == constants.ArtifactTypeMCP || artifactType == constants.ArtifactTypeAPIProduct) {
		// For future implementation when MCP and API_PRODUCT are supported
		allArtifacts = []dto.GatewayArtifact{}
	}

	listResponse := &dto.GatewayArtifactListResponse{
		Count: len(allArtifacts),
		List:  allArtifacts,
		Pagination: dto.Pagination{
			Total:  len(allArtifacts),
			Offset: 0,
			Limit:  len(allArtifacts),
		},
	}

	return listResponse, nil
}

// validateGatewayInput validates gateway registration inputs
func (s *GatewayService) validateGatewayInput(orgID, name, displayName, vhost, functionalityType string) error {
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

	// VHost validation
	vhost = strings.TrimSpace(vhost)
	if vhost == "" {
		return errors.New("vhost is required")
	}

	// Gateway type validation
	functionalityType = strings.TrimSpace(functionalityType)
	if functionalityType == "" {
		return errors.New("gateway functionality type is required")
	}
	if !constants.ValidGatewayFunctionalityType[functionalityType] {
		return fmt.Errorf("gateway type must be one of: %s, %s, %s",
			constants.GatewayFunctionalityTypeRegular, constants.GatewayFunctionalityTypeAI, constants.GatewayFunctionalityTypeEvent)
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

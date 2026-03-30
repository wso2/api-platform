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
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GatewayPolicyInput is the raw policy data received from the gateway controller.
// ManagedBy is used only for filtering and is not stored or returned.
type GatewayPolicyInput struct {
	Name             string                 `json:"name"`
	Version          string                 `json:"version"`
	DisplayName      string                 `json:"displayName,omitempty"`
	Description      *string                `json:"description,omitempty"`
	Parameters       map[string]interface{} `json:"parameters,omitempty"`
	SystemParameters map[string]interface{} `json:"systemParameters,omitempty"`
	ManagedBy        string                 `json:"managedBy"`
}

// GatewayPolicyDefinition is the cleaned policy data stored in memory and returned to APIM.
// PolicyDefinition holds the full policy schema (parameters + systemParameters) as received from the gateway controller.
type GatewayPolicyDefinition struct {
	Name             string                 `json:"name"`
	Version          string                 `json:"version"`
	DisplayName      string                 `json:"displayName,omitempty"`
	Description      *string                `json:"description,omitempty"`
	ManagedBy        string                 `json:"managedBy"`
	PolicyDefinition map[string]interface{} `json:"policyDefinition,omitempty"`
}

// Manifest holds the gateway manifest returned to APIM.
type Manifest struct {
	Policies json.RawMessage
}

// GatewayService handles gateway business logic
type GatewayService struct {
	gatewayRepo          repository.GatewayRepository
	orgRepo              repository.OrganizationRepository
	apiRepo              repository.APIRepository
	customPolicyRepo     repository.CustomPolicyRepository
	gatewayEventsService *GatewayEventsService
	slogger              *slog.Logger
}

// NewGatewayService creates a new gateway service
func NewGatewayService(gatewayRepo repository.GatewayRepository, orgRepo repository.OrganizationRepository,
	apiRepo repository.APIRepository, customPolicyRepo repository.CustomPolicyRepository,
	gatewayEventsService *GatewayEventsService, slogger *slog.Logger) *GatewayService {
	return &GatewayService{
		gatewayRepo:          gatewayRepo,
		orgRepo:              orgRepo,
		apiRepo:              apiRepo,
		customPolicyRepo:     customPolicyRepo,
		gatewayEventsService: gatewayEventsService,
		slogger:              slogger,
	}
}

// GetStoredManifest returns the latest gateway manifest. Called by APIM to retrieve
// the manifest that was pushed by the gateway controller on connect.
func (s *GatewayService) GetStoredManifest(gatewayID, orgID string) (*Manifest, error) {
	gateway, err := s.gatewayRepo.GetByUUID(gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil {
		return nil, constants.ErrGatewayNotFound
	}
	if gateway.OrganizationID != orgID {
		return nil, constants.ErrGatewayNotFound
	}

	raw, err := s.gatewayRepo.GetGatewayManifest(gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway manifest: %w", err)
	}

	return &Manifest{Policies: json.RawMessage(raw)}, nil
}

// ReceiveGatewayManifest stores the manifest posted by the gateway controller on connect.
// All policies are stored with name and version; customer-managed policies include policy_definition.
func (s *GatewayService) ReceiveGatewayManifest(orgID, gatewayID string, policies []GatewayPolicyInput) error {
	entries := make([]GatewayPolicyDefinition, 0, len(policies))
	for _, p := range policies {
		entry := GatewayPolicyDefinition{
			Name:        strings.ToLower(p.Name),
			Version:     p.Version,
			DisplayName: p.DisplayName,
			Description: p.Description,
			ManagedBy:   p.ManagedBy,
		}
		if p.ManagedBy == constants.PolicyManagedByCustomer {
			policyDef := map[string]interface{}{}
			if p.Parameters != nil {
				policyDef["parameters"] = p.Parameters
			}
			if p.SystemParameters != nil {
				policyDef["systemParameters"] = p.SystemParameters
			}
			entry.PolicyDefinition = policyDef
		}
		entries = append(entries, entry)
	}

	raw, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("failed to marshal gateway manifest: %w", err)
	}
	if err := s.gatewayRepo.UpdateGatewayManifest(gatewayID, raw); err != nil {
		return fmt.Errorf("failed to store gateway manifest: %w", err)
	}

	customerCount := 0
	for _, p := range policies {
		if p.ManagedBy == constants.PolicyManagedByCustomer {
			customerCount++
		}
	}
	s.slogger.Info("Gateway manifest received and stored",
		slog.String("org_id", orgID),
		slog.String("gateway_id", gatewayID),
		slog.Int("total_policy_count", len(entries)),
		slog.Int("customer_policy_count", customerCount),
	)
	return nil
}

// parsedVersion holds the numeric components of a semver string (e.g. "v1.2.3" or "1.2.3").
type parsedVersion struct {
	Major int
	Minor int
	Patch int
}

// parseVersion parses a version string of the form [v]MAJOR.MINOR.PATCH.
func parseVersion(v string) (parsedVersion, error) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return parsedVersion{}, fmt.Errorf("invalid version format '%s': expected MAJOR.MINOR.PATCH", v)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return parsedVersion{}, fmt.Errorf("invalid major version in '%s'", v)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return parsedVersion{}, fmt.Errorf("invalid minor version in '%s'", v)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return parsedVersion{}, fmt.Errorf("invalid patch version in '%s'", v)
	}
	return parsedVersion{Major: major, Minor: minor, Patch: patch}, nil
}

// SyncCustomPolicy upserts a custom policy from the gateway manifest into the gateway_custom_policies table.
// The gateway must belong to the caller's organization. The policy must exist in the manifest and it should be a custom policy.
func (s *GatewayService) SyncCustomPolicy(gatewayID, orgID, policyName, version string) (*model.CustomPolicy, error) {
	policyName = strings.ToLower(policyName)

	gateway, err := s.gatewayRepo.GetByUUID(gatewayID)
	if err != nil || gateway == nil {
		return nil, errors.New("gateway not found")
	}
	if gateway.OrganizationID != orgID {
		return nil, errors.New("gateway not found")
	}

	raw, err := s.gatewayRepo.GetGatewayManifest(gatewayID)
	if err != nil {
		s.slogger.Error("failed to read gateway manifest", slog.String("gateway_id", gatewayID), slog.String("org_id", orgID))
		return nil, fmt.Errorf("failed to read gateway manifest: %w", err)
	}
	if len(raw) == 0 {
		s.slogger.Error("gateway manifest is not available", slog.String("gateway_id", gatewayID), slog.String("org_id", orgID))
		return nil, errors.New("gateway manifest is not available")
	}

	var policies []GatewayPolicyDefinition
	if err := json.Unmarshal(raw, &policies); err != nil {
		return nil, fmt.Errorf("failed to parse gateway manifest: %w", err)
	}

	var found *GatewayPolicyDefinition
	for i := range policies {
		if policies[i].Name == policyName && policies[i].Version == version {
			found = &policies[i]
			break
		}
	}
	if found == nil {
		s.slogger.Error("policy not found in gateway manifest", slog.String("gateway_id", gatewayID), slog.String("org_id", orgID), slog.String("policy_name", policyName), slog.String("version", version))
		return nil, fmt.Errorf("policy '%s' version '%s' not found in gateway manifest", policyName, version)
	}
	if found.ManagedBy != constants.PolicyManagedByCustomer {
		s.slogger.Error("policy is not a custom policy", slog.String("gateway_id", gatewayID), slog.String("org_id", orgID), slog.String("policy_name", policyName), slog.String("version", version))
		return nil, fmt.Errorf("policy '%s' version '%s' is not a custom policy", policyName, version)
	}

	var policyDefJSON json.RawMessage
	if found.PolicyDefinition != nil {
		b, err := json.Marshal(found.PolicyDefinition)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize policy definition: %w", err)
		}
		policyDefJSON = json.RawMessage(b)
	}

	incomingVer, err := parseVersion(version)
	if err != nil {
		s.slogger.Error("invalid version format", slog.String("org_id", orgID), slog.String("policy_name", policyName), slog.String("version", version))
		return nil, fmt.Errorf("invalid version '%s': %w", version, err)
	}

	existingPolicies, err := s.customPolicyRepo.GetCustomPoliciesByName(orgID, policyName)
	if err != nil {
		s.slogger.Error("failed to check existing custom policies", slog.String("org_id", orgID), slog.String("policy_name", policyName), slog.String("version", version))
		return nil, fmt.Errorf("failed to check existing custom policies: %w", err)
	}

	// Find an existing policy with the same major version.
	var sameMajorVersionedPolicy *model.CustomPolicy
	for _, p := range existingPolicies {
		pv, err := parseVersion(p.Version)
		if err != nil {
			continue
		}
		if pv.Major == incomingVer.Major {
			sameMajorVersionedPolicy = p
			break
		}
	}

	policy := &model.CustomPolicy{
		OrganizationUUID: orgID,
		Name:             policyName,
		DisplayName:      utils.StringPtrIfNotEmpty(found.DisplayName),
		Version:          version,
		Description:      found.Description,
		PolicyDefinition: policyDefJSON,
	}

	if sameMajorVersionedPolicy != nil {
		existingVer, err := parseVersion(sameMajorVersionedPolicy.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to parse existing version '%s': %w", sameMajorVersionedPolicy.Version, err)
		}
		if existingVer.Minor == incomingVer.Minor && existingVer.Patch == incomingVer.Patch {
			// Exact same version — already exists.
			return nil, fmt.Errorf("custom policy '%s' version '%s' already exists", policyName, version)
		}
		if existingVer.Minor == incomingVer.Minor && existingVer.Patch != incomingVer.Patch {
			// Same major.minor, different patch — patch update, not allowed.
			return nil, fmt.Errorf("patch version updates are not allowed for policy '%s': existing '%s', incoming '%s'",
				policyName, sameMajorVersionedPolicy.Version, version)
		}
		if incomingVer.Minor < existingVer.Minor {
			return nil, fmt.Errorf("cannot downgrade policy '%s' from '%s' to '%s'",
				policyName, sameMajorVersionedPolicy.Version, version)
		}
		// New minor version — update the existing record.
		policy.UUID = sameMajorVersionedPolicy.UUID
		if err := s.customPolicyRepo.UpdateCustomPolicy(policy, sameMajorVersionedPolicy.Version); err != nil {
			s.slogger.Error("failed to update custom policy", slog.String("org_id", orgID), slog.String("policy_name", policyName), slog.String("old_version", sameMajorVersionedPolicy.Version), slog.String("new_version", version))
			return nil, fmt.Errorf("failed to update custom policy: %w", err)
		}
		s.slogger.Info("Custom policy updated (minor version bump)",
			slog.String("gateway_id", gatewayID),
			slog.String("org_id", orgID),
			slog.String("policy_name", policyName),
			slog.String("old_version", sameMajorVersionedPolicy.Version),
			slog.String("new_version", version),
		)
	} else {
		// No existing policy with this major version — insert as new.
		policyID, err := utils.GenerateUUID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate policy UUID: %w", err)
		}
		policy.UUID = policyID
		if err := s.customPolicyRepo.InsertCustomPolicy(policy); err != nil {
			return nil, fmt.Errorf("failed to insert custom policy: %w", err)
		}
		s.slogger.Info("Custom policy created",
			slog.String("gateway_id", gatewayID),
			slog.String("org_id", orgID),
			slog.String("policy_name", policyName),
			slog.String("version", version),
		)
	}

	persisted, err := s.customPolicyRepo.GetCustomPolicyByNameAndVersion(orgID, policyName, version)
	if err != nil || persisted == nil {
		return policy, nil
	}
	return persisted, nil
}

// ListCustomPolicies returns all custom policies synced for the given organization.
func (s *GatewayService) ListCustomPolicies(orgID string) ([]*model.CustomPolicy, error) {
	return s.customPolicyRepo.ListCustomPolicyByOrganization(orgID)
}

// GetCustomPolicyByUUID returns a custom policy by UUID, verifying org ownership and version.
func (s *GatewayService) GetCustomPolicyByUUID(orgID, policyUUID, version string) (*model.CustomPolicy, error) {
	policy, err := s.customPolicyRepo.GetCustomPolicyByUUID(orgID, policyUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve custom policy (org_id=%s, policy_uuid=%s): %w", orgID, policyUUID, err)
	}
	if policy == nil {
		return nil, constants.ErrCustomPolicyNotFound
	}
	if policy.Version != version {
		return nil, constants.ErrCustomPolicyVersionMismatch
	}
	return policy, nil
}

// DeleteCustomPolicyByUUID deletes a custom policy by UUID, verifying org ownership, version, and no active usages.
func (s *GatewayService) DeleteCustomPolicyByUUID(orgID, policyUUID, version string) error {
	policy, err := s.customPolicyRepo.GetCustomPolicyByUUID(orgID, policyUUID)
	if err != nil {
		return fmt.Errorf("failed to retrieve custom policy (org_id=%s, policy_uuid=%s): %w", orgID, policyUUID, err)
	}
	if policy == nil {
		return constants.ErrCustomPolicyNotFound
	}
	if policy.Version != version {
		return constants.ErrCustomPolicyVersionMismatch
	}

	return s.customPolicyRepo.DeleteCustomPolicyIfUnused(orgID, policyUUID)
}

// RegisterGateway registers a new gateway with organization validation
func (s *GatewayService) RegisterGateway(orgID, name, displayName, description, vhost string, isCritical bool,
	functionalityType string, properties map[string]interface{}) (*api.GatewayResponse, error) {
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
	gatewayId, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate gateway ID: %w", err)
	}

	// 5. Create Gateway model
	gateway := &model.Gateway{
		ID:                gatewayId,
		OrganizationID:    orgID,
		Name:              name,
		DisplayName:       displayName,
		Description:       description,
		Properties:        properties,
		Vhost:             vhost,
		IsCritical:        isCritical,
		FunctionalityType: functionalityType,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// 6. Insert gateway
	if err := s.gatewayRepo.Create(gateway); err != nil {
		return nil, fmt.Errorf("failed to create gateway: %w", err)
	}

	// 7. Return GatewayResponse
	return gatewayModelToAPI(gateway), nil
}

// ListGateways retrieves all gateways with constitution-compliant envelope structure
func (s *GatewayService) ListGateways(orgID *string) (*api.GatewayListResponse, error) {
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

	// Convert to API types
	responses := make([]api.GatewayResponse, 0, len(gateways))
	for _, gw := range gateways {
		if resp := gatewayModelToAPI(gw); resp != nil {
			responses = append(responses, *resp)
		}
	}

	// Build constitution-compliant list response with pagination metadata
	return &api.GatewayListResponse{
		Count: len(responses),
		List:  responses,
		Pagination: api.Pagination{
			Total:  len(responses),
			Offset: 0,
			Limit:  len(responses),
		},
	}, nil
}

// GetGateway retrieves a gateway by ID
func (s *GatewayService) GetGateway(gatewayId, orgId string) (*api.GatewayResponse, error) {
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

	return gatewayModelToAPI(gateway), nil
}

// UpdateGateway updates gateway details
func (s *GatewayService) UpdateGateway(gatewayId, orgId string, description, displayName *string,
	isCritical *bool, properties *map[string]interface{}) (*api.GatewayResponse, error) {
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
	if properties != nil {
		gateway.Properties = *properties
	}
	gateway.UpdatedAt = time.Now()

	err = s.gatewayRepo.UpdateGateway(gateway)
	if err != nil {
		return nil, err
	}

	return gatewayModelToAPI(gateway), nil
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

	// Delete gateway (FK CASCADE will automatically remove tokens and deployments; association_mappings cleanup is handled by the repository)
	err = s.gatewayRepo.Delete(gatewayID, orgID)
	if err != nil {
		return err
	}

	return nil
}

// VerifyToken verifies a plain-text token and returns the associated gateway
func (s *GatewayService) VerifyToken(plainToken string) (*model.Gateway, error) {
	if plainToken == "" {
		return nil, constants.ErrMissingAPIKey
	}

	// Hash the token and look it up directly in the database
	tokenHash := hashToken(plainToken)
	token, err := s.gatewayRepo.GetActiveTokenByHash(tokenHash)
	if err != nil {
		return nil, fmt.Errorf("failed to query token: %w", err)
	}
	if token == nil {
		return nil, constants.ErrInvalidAPIToken
	}

	// Fetch the associated gateway
	gateway, err := s.gatewayRepo.GetByUUID(token.GatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to query gateway: %w", err)
	}
	if gateway == nil {
		return nil, constants.ErrInvalidAPIToken
	}

	return gateway, nil
}

// ListTokens retrieves all active tokens for a gateway
func (s *GatewayService) ListTokens(gatewayId, orgId string) ([]api.TokenInfoResponse, error) {
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

	activeTokens, err := s.gatewayRepo.GetActiveTokensByGatewayUUID(gatewayId)
	if err != nil {
		return nil, fmt.Errorf("failed to get tokens: %w", err)
	}

	tokens := make([]api.TokenInfoResponse, 0, len(activeTokens))
	for _, t := range activeTokens {
		tokenUUID, err := uuid.Parse(t.ID)
		if err != nil {
			// Skip invalid UUIDs (should never happen for persisted tokens)
			continue
		}

		status := api.TokenInfoResponseStatus(t.Status)
		tokens = append(tokens, api.TokenInfoResponse{
			Id:        &tokenUUID,
			Status:    &status,
			CreatedAt: &t.CreatedAt,
			RevokedAt: t.RevokedAt,
		})
	}

	return tokens, nil
}

// RotateToken generates a new token for a gateway (max 2 active tokens)
func (s *GatewayService) RotateToken(gatewayId, orgId string) (*api.TokenRotationResponse, error) {
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

	// 4. Generate new plain-text token
	plainToken, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// 5. Hash new token
	tokenHash := hashToken(plainToken)

	// 6. Create new GatewayToken model with status='active'
	tokenId, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate gateway token ID: %w", err)
	}
	gatewayToken := &model.GatewayToken{
		ID:        tokenId,
		GatewayID: gatewayId,
		TokenHash: tokenHash,
		Salt:      "",
		Status:    "active",
		CreatedAt: time.Now(),
		RevokedAt: nil,
	}

	// 7. Insert token using repository
	if err := s.gatewayRepo.CreateToken(gatewayToken); err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	// 8. Return TokenRotationResponse
	return tokenRotationModelToAPI(tokenId, plainToken, gatewayToken.CreatedAt), nil
}

// RevokeToken revokes a specific token for a gateway
func (s *GatewayService) RevokeToken(gatewayId, tokenId, orgId string) error {
	gateway, err := s.gatewayRepo.GetByUUID(gatewayId)
	if err != nil {
		return fmt.Errorf("failed to query gateway: %w", err)
	}
	if gateway == nil {
		return errors.New("gateway not found")
	}
	if gateway.OrganizationID != orgId {
		return errors.New("gateway not found")
	}

	token, err := s.gatewayRepo.GetTokenByUUID(tokenId)
	if err != nil {
		return fmt.Errorf("failed to query token: %w", err)
	}
	if token == nil {
		return errors.New("token not found")
	}
	if token.GatewayID != gatewayId {
		return errors.New("token not found")
	}

	if err := s.gatewayRepo.RevokeToken(tokenId); err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	return nil
}

// GetGatewayStatus retrieves gateway status information for polling
func (s *GatewayService) GetGatewayStatus(orgID string, gatewayId *string) (*api.GatewayStatusListResponse, error) {
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

	// Convert to API types
	responses := make([]api.GatewayStatusResponse, 0, len(gateways))
	for _, gw := range gateways {
		if resp := gatewayStatusModelToAPI(gw); resp != nil {
			responses = append(responses, *resp)
		}
	}

	// Build constitution-compliant list response
	return &api.GatewayStatusListResponse{
		Count: len(responses),
		List:  responses,
		Pagination: api.Pagination{
			Total:  len(responses),
			Offset: 0,
			Limit:  len(responses),
		},
	}, nil
}

// UpdateGatewayActiveStatus updates the active status of a gateway
func (s *GatewayService) UpdateGatewayActiveStatus(gatewayId string, isActive bool) error {
	return s.gatewayRepo.UpdateActiveStatus(gatewayId, isActive)
}

// GetGatewayArtifacts retrieves all artifacts (APIs) deployed to a specific gateway with pagination and optional type filtering
func (s *GatewayService) GetGatewayArtifacts(gatewayID, orgID, artifactType string) (*api.GatewayArtifactListResponse, error) {
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
	// TODO(RakhithaRR): In future, when MCP and API_PRODUCT are supported, this method should be updated to query those artifacts as well,
	//  and apply type filtering at the database level for efficiency
	apis, err := s.apiRepo.GetDeployedAPIsByGatewayUUID(gatewayID, orgID)
	if err != nil {
		return nil, err
	}

	// Convert APIs to GatewayArtifact API types and apply type filtering
	artifacts := make([]api.GatewayArtifact, 0)
	var subType *api.GatewayArtifactSubType

	for _, apiModel := range apis {
		// Skip if artifactType filter is specified and doesn't match "API"
		if artifactType != "" && artifactType != constants.ArtifactTypeAPI {
			continue
		}

		sub := api.GatewayArtifactSubType(constants.APISubTypeHTTP)
		subType = &sub
		artifactTypeEnum := api.GatewayArtifactType(constants.ArtifactTypeAPI)

		if artifact := gatewayArtifactModelToAPI(apiModel, artifactTypeEnum, subType); artifact != nil {
			artifacts = append(artifacts, *artifact)
		}
	}

	// If filtering by MCP or API_PRODUCT, return empty list for now (future implementation)
	if artifactType != "" && (artifactType == constants.ArtifactTypeMCP || artifactType == constants.ArtifactTypeAPIProduct) {
		// For future implementation when MCP and API_PRODUCT are supported
		artifacts = []api.GatewayArtifact{}
	}

	return &api.GatewayArtifactListResponse{
		Count: len(artifacts),
		List:  artifacts,
		Pagination: api.Pagination{
			Total:  len(artifacts),
			Offset: 0,
			Limit:  len(artifacts),
		},
	}, nil
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

// hashToken computes SHA-256 hash of the token and returns hex-encoded string
func hashToken(plainToken string) string {
	h := sha256.New()
	h.Write([]byte(plainToken))
	tokenHash := h.Sum(nil)
	return hex.EncodeToString(tokenHash)
}

// Mapping functions

// gatewayModelToAPI converts a Gateway model to GatewayResponse API type
func gatewayModelToAPI(gateway *model.Gateway) *api.GatewayResponse {
	if gateway == nil {
		return nil
	}

	gatewayID, err := uuid.Parse(gateway.ID)
	if err != nil {
		return nil
	}
	orgID, err := uuid.Parse(gateway.OrganizationID)
	if err != nil {
		return nil
	}
	functionalityType := api.GatewayResponseFunctionalityType(gateway.FunctionalityType)

	return &api.GatewayResponse{
		Id:                &gatewayID,
		OrganizationId:    &orgID,
		Name:              &gateway.Name,
		DisplayName:       &gateway.DisplayName,
		Description:       utils.StringPtrIfNotEmpty(gateway.Description),
		Properties:        utils.MapPtrIfNotEmpty(gateway.Properties),
		Vhost:             &gateway.Vhost,
		IsCritical:        &gateway.IsCritical,
		FunctionalityType: &functionalityType,
		IsActive:          &gateway.IsActive,
		CreatedAt:         &gateway.CreatedAt,
		UpdatedAt:         &gateway.UpdatedAt,
	}
}

// gatewayStatusModelToAPI converts a Gateway model to GatewayStatusResponse API type
func gatewayStatusModelToAPI(gateway *model.Gateway) *api.GatewayStatusResponse {
	if gateway == nil {
		return nil
	}

	gatewayID, err := uuid.Parse(gateway.ID)
	if err != nil {
		return nil
	}

	return &api.GatewayStatusResponse{
		Id:         &gatewayID,
		Name:       &gateway.Name,
		IsActive:   &gateway.IsActive,
		IsCritical: &gateway.IsCritical,
	}
}

// tokenRotationModelToAPI creates a TokenRotationResponse API type
func tokenRotationModelToAPI(tokenID string, token string, createdAt time.Time) *api.TokenRotationResponse {
	id, err := uuid.Parse(tokenID)
	if err != nil {
		return nil
	}
	message := "New token generated successfully. Old token remains active until revoked."

	return &api.TokenRotationResponse{
		Id:        &id,
		Token:     &token,
		CreatedAt: &createdAt,
		Message:   &message,
	}
}

// gatewayArtifactModelToAPI converts an API model to GatewayArtifact API type
func gatewayArtifactModelToAPI(apiModel *model.API, artifactType api.GatewayArtifactType, subType *api.GatewayArtifactSubType) *api.GatewayArtifact {
	if apiModel == nil {
		return nil
	}

	return &api.GatewayArtifact{
		Id:        apiModel.Handle,
		Name:      apiModel.Name,
		Type:      artifactType,
		SubType:   subType,
		CreatedAt: apiModel.CreatedAt,
		UpdatedAt: apiModel.UpdatedAt,
	}
}

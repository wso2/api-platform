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
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
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
	gatewayRepo                         repository.GatewayRepository
	orgRepo                             repository.OrganizationRepository
	apiRepo                             repository.APIRepository
	customPolicyRepo                    repository.CustomPolicyRepository
	gatewayEventsService                *GatewayEventsService
	slogger                             *slog.Logger
	enableVersionVerification           bool
	enableFunctionalityTypeVerification bool
	auditRepo                           repository.AuditRepository
	identity                            *IdentityService
}

// NewGatewayService creates a new gateway service
func NewGatewayService(gatewayRepo repository.GatewayRepository, orgRepo repository.OrganizationRepository,
	apiRepo repository.APIRepository, customPolicyRepo repository.CustomPolicyRepository,
	gatewayEventsService *GatewayEventsService, slogger *slog.Logger,
	enableVersionVerification bool, enableFunctionalityTypeVerification bool,
	auditRepo repository.AuditRepository, identity *IdentityService) *GatewayService {
	return &GatewayService{
		gatewayRepo:                         gatewayRepo,
		orgRepo:                             orgRepo,
		apiRepo:                             apiRepo,
		customPolicyRepo:                    customPolicyRepo,
		gatewayEventsService:                gatewayEventsService,
		slogger:                             slogger,
		enableVersionVerification:           enableVersionVerification,
		enableFunctionalityTypeVerification: enableFunctionalityTypeVerification,
		auditRepo:                           auditRepo,
		identity:                            identity,
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

// legacyGatewayVersion is reported when the gateway controller does not include a
// "version" field in the manifest payload. v1.0.0 was the only such release.
const legacyGatewayVersion = "1.0.0"

// extractMajorMinor parses a version string and returns its canonical
// `major.minor` form (numeric, no leading zeros). Pre-release / build metadata
// (after `-` or `+`) is stripped. If only a major is present (e.g. "2"), the
// minor defaults to 0. Returns an empty string if the input is empty or the
// numeric segments cannot be parsed.
func extractMajorMinor(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return ""
	}
	minor := 0
	if len(parts) >= 2 && parts[1] != "" {
		m, err := strconv.Atoi(parts[1])
		if err != nil {
			return ""
		}
		minor = m
	}
	return strconv.Itoa(major) + "." + strconv.Itoa(minor)
}

// legacyFunctionalityType is reported when the gateway controller does not include a
// "functionalityType" field in the manifest payload (pre-1.1.0 builds).
const legacyFunctionalityType = "regular"

// functionalityTypeCompatible reports whether a controller-reported functionality
// type is acceptable for the registered gateway type.
//
// AI and regular gateways are compiled into the same binary, which reports
// itself as "regular". A gateway registered as either "ai" or "regular"
// therefore accepts a controller reporting "regular". The event gateway has
// its own binary and must match exactly.
func functionalityTypeCompatible(registered, reported string) bool {
	if registered == reported {
		return true
	}
	if registered == constants.GatewayFunctionalityTypeAI && reported == constants.GatewayFunctionalityTypeRegular {
		return true
	}
	return false
}

// ReceiveGatewayManifest stores the manifest posted by the gateway controller on connect.
// All policies are stored with name and version; customer-managed policies include policy_definition.
// gatewayVersion is the controller's reported build version; an empty string means the controller
// is on a legacy build (pre-1.1.0) that does not send a version — assumed to be "1.0.0".
// functionalityType is the controller's flavor ("regular", "event", ...); empty is treated as "regular".
// The reported major.minor and functionality type must match the registered values, otherwise the manifest is rejected.
func (s *GatewayService) ReceiveGatewayManifest(orgID, gatewayID, gatewayVersion, functionalityType string, policies []GatewayPolicyInput) error {
	gateway, err := s.gatewayRepo.GetByUUID(gatewayID)
	if err != nil {
		return fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgID {
		return constants.ErrGatewayNotFound
	}

	reported := strings.TrimSpace(gatewayVersion)
	if reported == "" {
		reported = legacyGatewayVersion
	}
	reportedMinor := extractMajorMinor(reported)
	registeredMinor := extractMajorMinor(gateway.Version)
	if registeredMinor == "" {
		registeredMinor = defaultGatewayVersion
	}
	if reportedMinor != registeredMinor {
		if s.enableVersionVerification {
			return fmt.Errorf("%w: registered=%s, reported=%s", constants.ErrGatewayVersionMismatch, registeredMinor, reportedMinor)
		}
		s.slogger.Warn("Gateway version mismatch ignored (verification disabled)",
			slog.String("org_id", orgID),
			slog.String("gateway_id", gatewayID),
			slog.String("registered", registeredMinor),
			slog.String("reported", reportedMinor),
		)
	}

	reportedType := strings.TrimSpace(functionalityType)
	if reportedType == "" {
		reportedType = legacyFunctionalityType
	}
	if !functionalityTypeCompatible(gateway.FunctionalityType, reportedType) {
		if s.enableFunctionalityTypeVerification {
			return fmt.Errorf("%w: registered=%s, reported=%s", constants.ErrGatewayFunctionalityTypeMismatch, gateway.FunctionalityType, reportedType)
		}
		s.slogger.Warn("Gateway functionality type mismatch ignored (verification disabled)",
			slog.String("org_id", orgID),
			slog.String("gateway_id", gatewayID),
			slog.String("registered", gateway.FunctionalityType),
			slog.String("reported", reportedType),
		)
	}

	entries := make([]GatewayPolicyDefinition, 0, len(policies))
	for _, p := range policies {
		if !constants.ValidPolicyManagedBy[p.ManagedBy] {
			s.slogger.Warn("Skipping policy with unknown managed_by value",
				slog.String("gateway_id", gatewayID),
				slog.String("policy_name", p.Name),
				slog.String("managed_by", p.ManagedBy),
			)
			continue
		}
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

	// Persist the version the controller reported so the deploy transform uses the
	// live gateway version rather than the stale value stored at registration time.
	if err := s.gatewayRepo.UpdateGatewayVersion(gatewayID, reported); err != nil {
		return fmt.Errorf("failed to update gateway version: %w", err)
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
		slog.String("gateway_version", reported),
		slog.String("functionality_type", reportedType),
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
		CreatedBy:        gatewayID,
		UpdatedBy:        gatewayID,
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
		if s.auditRepo != nil {
			_ = s.auditRepo.Record("CREATE", policy.UUID, "custom_policy", orgID, "")
		}
		return policy, nil
	}
	if s.auditRepo != nil {
		_ = s.auditRepo.Record("CREATE", persisted.UUID, "custom_policy", orgID, "")
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

	if err := s.customPolicyRepo.DeleteCustomPolicyIfUnused(orgID, policyUUID); err != nil {
		return err
	}
	if s.auditRepo != nil {
		_ = s.auditRepo.Record("DELETE", policyUUID, "custom_policy", orgID, "")
	}
	return nil
}

// defaultGatewayVersion is the value stored when a client registers a gateway without a version.
const defaultGatewayVersion = "1.0"

// RegisterGateway registers a new gateway with organization validation
func (s *GatewayService) RegisterGateway(orgID string, id *string, displayName, description string, endpoints []string, isCritical bool,
	functionalityType, version, createdBy string, properties map[string]interface{}) (*api.GatewayResponse, error) {
	// Determine handle: use provided id or auto-generate from displayName
	var name string
	if id != nil && strings.TrimSpace(*id) != "" {
		name = strings.TrimSpace(*id)
	} else {
		var err error
		name, err = utils.GenerateHandle(displayName, func(h string) bool {
			existing, _ := s.gatewayRepo.GetByHandleAndOrgID(h, orgID)
			return existing != nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate gateway handle: %w", err)
		}
	}

	// 1. Validate inputs
	if err := s.validateGatewayInput(orgID, name, displayName, endpoints, functionalityType); err != nil {
		return nil, err
	}

	normalizedEndpoints := make([]string, len(endpoints))
	for i, endpoint := range endpoints {
		normalizedEndpoints[i] = strings.TrimSpace(endpoint)
	}

	version = strings.TrimSpace(version)
	if version == "" {
		version = defaultGatewayVersion
	}
	// CalVer versions (e.g. "2026.05.13") are persisted verbatim so the exact
	// build is preserved. Two-segment `major.minor` versions are canonicalized
	// so equality checks against controller-reported versions (also normalized
	// via extractMajorMinor) cannot diverge over leading zeros or other
	// lexical variants.
	if strings.Count(version, ".") == 1 {
		if canonical := extractMajorMinor(version); canonical != "" {
			version = canonical
		}
	}

	// 2. Validate organization exists
	org, err := s.orgRepo.GetOrganizationByUUID(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query organization: %w", err)
	}
	if org == nil {
		return nil, errors.New("organization not found")
	}

	// 3. Check gateway handle uniqueness within organization
	existing, err := s.gatewayRepo.GetByHandleAndOrgID(name, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to check gateway handle uniqueness: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("gateway with handle '%s' already exists in this organization", name)
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
		Handle:            name,
		Name:              displayName,
		Description:       description,
		Properties:        properties,
		Endpoints:         normalizedEndpoints,
		IsCritical:        isCritical,
		FunctionalityType: functionalityType,
		Version:           version,
		CreatedBy:         createdBy,
		UpdatedBy:         createdBy,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// 6. Insert gateway
	if err := s.gatewayRepo.Create(gateway); err != nil {
		return nil, fmt.Errorf("failed to create gateway: %w", err)
	}

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("CREATE", gateway.ID, "gateway", orgID, createdBy)
	}

	// 7. Return GatewayResponse
	return s.gatewayModelToAPI(gateway)
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
		resp, err := s.gatewayModelToAPI(gw)
		if err != nil {
			return nil, err
		}
		if resp != nil {
			// updatedBy is detail-only; omit it from list responses.
			resp.UpdatedBy = nil
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
	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayId, orgId)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}

	if gateway == nil {
		return nil, errors.New("gateway not found")
	}

	return s.gatewayModelToAPI(gateway)
}

// UpdateGateway updates gateway details
func (s *GatewayService) UpdateGateway(gatewayId, orgId, updatedBy string, req *api.GatewayResponse) (*api.GatewayResponse, error) {
	// Get existing gateway by handle
	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayId, orgId)
	if err != nil {
		return nil, err
	}
	if gateway == nil {
		return nil, constants.ErrGatewayNotFound
	}

	gateway.Name = req.DisplayName
	if req.Description != nil {
		gateway.Description = *req.Description
	}
	if req.Endpoints != nil {
		gateway.Endpoints = *req.Endpoints
	}
	if req.IsCritical != nil {
		gateway.IsCritical = *req.IsCritical
	}
	if req.Properties != nil {
		gateway.Properties = *req.Properties
	}
	gateway.UpdatedBy = updatedBy
	gateway.UpdatedAt = time.Now()

	err = s.gatewayRepo.UpdateGateway(gateway)
	if err != nil {
		return nil, err
	}

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("UPDATE", gateway.ID, "gateway", orgId, updatedBy)
	}

	return s.gatewayModelToAPI(gateway)
}

// DeleteGateway deletes a gateway and all associated tokens (CASCADE)
func (s *GatewayService) DeleteGateway(gatewayID, orgID, deletedBy string) error {
	// Verify gateway exists and belongs to organization (gatewayID is now the handle)
	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayID, orgID)
	if err != nil {
		return err
	}
	if gateway == nil {
		return constants.ErrGatewayNotFound
	}

	// Delete gateway by UUID (FK CASCADE will automatically remove tokens and deployments; association_mappings cleanup is handled by the repository)
	err = s.gatewayRepo.Delete(gateway.ID, orgID)
	if err != nil {
		return err
	}

	if s.auditRepo != nil {
		_ = s.auditRepo.Record("DELETE", gateway.ID, "gateway", orgID, deletedBy)
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
	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayId, orgId)
	if err != nil {
		return nil, fmt.Errorf("failed to query gateway: %w", err)
	}
	if gateway == nil {
		return nil, errors.New("gateway not found")
	}

	activeTokens, err := s.gatewayRepo.GetActiveTokensByGatewayUUID(gateway.ID)
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
func (s *GatewayService) RotateToken(gatewayId, orgId, createdBy string) (*api.TokenRotationResponse, error) {
	// 1. Validate gateway exists
	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayId, orgId)
	if err != nil {
		return nil, fmt.Errorf("failed to query gateway: %w", err)
	}
	if gateway == nil {
		return nil, errors.New("gateway not found")
	}

	// 2. Count active tokens
	activeCount, err := s.gatewayRepo.CountActiveTokens(gateway.ID)
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
		GatewayID: gateway.ID,
		TokenHash: tokenHash,
		Salt:      "",
		Status:    constants.GatewayTokenStatusActive,
		CreatedBy: createdBy,
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
func (s *GatewayService) RevokeToken(gatewayId, tokenId, orgId, revokedBy string) error {
	gateway, err := s.gatewayRepo.GetByHandleAndOrgID(gatewayId, orgId)
	if err != nil {
		return fmt.Errorf("failed to query gateway: %w", err)
	}
	if gateway == nil {
		return errors.New("gateway not found")
	}

	token, err := s.gatewayRepo.GetTokenByUUID(tokenId)
	if err != nil {
		return fmt.Errorf("failed to query token: %w", err)
	}
	if token == nil {
		return errors.New("token not found")
	}
	if token.GatewayID != gateway.ID {
		return errors.New("token not found")
	}

	if err := s.gatewayRepo.RevokeToken(tokenId, revokedBy); err != nil {
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

	// If gatewayId is provided, get specific gateway by handle
	if gatewayId != nil && *gatewayId != "" {
		gateway, err := s.gatewayRepo.GetByHandleAndOrgID(*gatewayId, orgID)
		if err != nil {
			return nil, fmt.Errorf("failed to get gateway: %w", err)
		}
		if gateway == nil {
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

// validateGatewayInput validates gateway registration inputs
func (s *GatewayService) validateGatewayInput(orgID, name, displayName string, endpoints []string, functionalityType string) error {
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

	// Endpoints validation
	if len(endpoints) == 0 {
		return errors.New("at least one endpoint is required")
	}
	for _, endpoint := range endpoints {
		endpoint = strings.TrimSpace(endpoint)
		if endpoint == "" {
			return errors.New("endpoint must not be empty")
		}
		if len(endpoint) > 255 {
			return errors.New("endpoint must not exceed 255 characters")
		}
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
func (s *GatewayService) gatewayModelToAPI(gateway *model.Gateway) (*api.GatewayResponse, error) {
	if gateway == nil {
		return nil, nil
	}

	orgHandle := ""
	if org, err := s.orgRepo.GetOrganizationByUUID(gateway.OrganizationID); err == nil && org != nil {
		orgHandle = org.Handle
	}
	functionalityType := api.GatewayResponseFunctionalityType(gateway.FunctionalityType)

	resp := &api.GatewayResponse{
		Id:                &gateway.Handle,
		OrganizationId:    &orgHandle,
		DisplayName:       gateway.Name,
		Description:       utils.StringPtrIfNotEmpty(gateway.Description),
		Properties:        utils.MapPtrIfNotEmpty(gateway.Properties),
		Endpoints:         &gateway.Endpoints,
		IsCritical:        &gateway.IsCritical,
		FunctionalityType: &functionalityType,
		Version:           &gateway.Version,
		IsActive:          &gateway.IsActive,
		CreatedBy:         utils.StringPtrIfNotEmpty(gateway.CreatedBy),
		UpdatedBy:         utils.StringPtrIfNotEmpty(gateway.UpdatedBy),
		CreatedAt:         &gateway.CreatedAt,
		UpdatedAt:         &gateway.UpdatedAt,
	}
	if err := s.identity.ResolveIdentityField(&resp.CreatedBy); err != nil {
		return nil, err
	}
	if err := s.identity.ResolveIdentityField(&resp.UpdatedBy); err != nil {
		return nil, err
	}
	return resp, nil
}

// gatewayStatusModelToAPI converts a Gateway model to GatewayStatusResponse API type
func gatewayStatusModelToAPI(gateway *model.Gateway) *api.GatewayStatusResponse {
	if gateway == nil {
		return nil
	}

	return &api.GatewayStatusResponse{
		Id:         &gateway.Handle,
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

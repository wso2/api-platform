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
	"fmt"
	"log/slog"
	"platform-api/src/api"
	"platform-api/src/config"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// IDPClaimUpdater syncs org membership claims back to the IDP so that JWTs
// issued on subsequent logins carry the up-to-date claim value.
type IDPClaimUpdater interface {
	// UpdateOrgClaims patches /scim2/Me using the caller's own bearer token.
	// Used when the calling user registers a new org for themselves.
	UpdateOrgClaims(bearerToken string, orgIDs []string) error
	// UpdateUserOrgClaims patches /scim2/Users/{targetUserID} using an admin
	// bearer token. Used when an admin adds another user to an org.
	UpdateUserOrgClaims(adminBearerToken, targetUserID string, orgIDs []string) error
}

type OrganizationService struct {
	orgRepo           repository.OrganizationRepository
	membershipRepo    repository.UserOrgMembershipRepository
	idpClaimUpdater   IDPClaimUpdater // optional; nil disables IDP sync
	projectRepo       repository.ProjectRepository
	applicationRepo   repository.ApplicationRepository
	apiRepo           repository.APIRepository
	gatewayRepo       repository.GatewayRepository
	llmProviderRepo   repository.LLMProviderRepository
	llmProxyRepo      repository.LLMProxyRepository
	mcpProxyRepo      repository.MCPProxyRepository
	websubAPIRepo     repository.WebSubAPIRepository
	devPortalService  *DevPortalService
	llmTemplateSeeder *LLMTemplateSeeder
	config            *config.Server
	slogger           *slog.Logger
}

func NewOrganizationService(orgRepo repository.OrganizationRepository,
	membershipRepo repository.UserOrgMembershipRepository,
	idpClaimUpdater IDPClaimUpdater,
	projectRepo repository.ProjectRepository,
	applicationRepo repository.ApplicationRepository,
	apiRepo repository.APIRepository,
	gatewayRepo repository.GatewayRepository,
	llmProviderRepo repository.LLMProviderRepository,
	llmProxyRepo repository.LLMProxyRepository,
	mcpProxyRepo repository.MCPProxyRepository,
	websubAPIRepo repository.WebSubAPIRepository,
	devPortalService *DevPortalService,
	llmTemplateSeeder *LLMTemplateSeeder,
	cfg *config.Server,
	slogger *slog.Logger,
) *OrganizationService {
	return &OrganizationService{
		orgRepo:           orgRepo,
		membershipRepo:    membershipRepo,
		idpClaimUpdater:   idpClaimUpdater,
		projectRepo:       projectRepo,
		applicationRepo:   applicationRepo,
		apiRepo:           apiRepo,
		gatewayRepo:       gatewayRepo,
		llmProviderRepo:   llmProviderRepo,
		llmProxyRepo:      llmProxyRepo,
		mcpProxyRepo:      mcpProxyRepo,
		websubAPIRepo:     websubAPIRepo,
		devPortalService:  devPortalService,
		llmTemplateSeeder: llmTemplateSeeder,
		config:            cfg,
		slogger:           slogger,
	}
}

func (s *OrganizationService) GetOrganizationSubscription(orgID string) (*api.OrganizationSubscription, error) {
	if _, err := s.GetOrganizationByUUID(orgID); err != nil {
		return nil, err
	}

	llmProvidersCount, err := s.llmProviderRepo.Count(orgID)
	if err != nil {
		return nil, err
	}

	llmProxiesCount, err := s.llmProxyRepo.Count(orgID)
	if err != nil {
		return nil, err
	}

	applicationsCount, err := s.applicationRepo.CountApplicationsByOrganizationID(orgID)
	if err != nil {
		return nil, err
	}

	mcpProxiesCount, err := s.mcpProxyRepo.Count(orgID)
	if err != nil {
		return nil, err
	}

	websubAPICount, err := s.websubAPIRepo.Count(orgID)
	if err != nil {
		return nil, err
	}

	gateways, err := s.gatewayRepo.GetByOrganizationID(orgID)
	if err != nil {
		return nil, err
	}

	apis, err := s.apiRepo.GetAPIsByOrganizationUUID(orgID, "")
	if err != nil {
		return nil, err
	}

	llmProvidersLimit := constants.MaxLLMProvidersPerOrganization
	llmProvidersRemaining := max(llmProvidersLimit-llmProvidersCount, 0)

	llmProxiesLimit := constants.MaxLLMProxiesPerOrganization
	llmProxiesRemaining := max(llmProxiesLimit-llmProxiesCount, 0)

	mcpProxiesLimit := constants.MaxMCPProxiesPerOrganization
	mcpProxiesRemaining := max(mcpProxiesLimit-mcpProxiesCount, 0)

	websubAPIsLimit := constants.MaxWebSubAPIsPerOrganization
	websubAPIsRemaining := max(websubAPIsLimit-websubAPICount, 0)

	res := &api.OrganizationSubscription{
		Plan: "free",
		Quotas: api.OrganizationSubscriptionQuotas{
			LlmProviders: api.OrganizationQuota{
				Used:      llmProvidersCount,
				Limit:     intPtr(llmProvidersLimit),
				Remaining: intPtr(llmProvidersRemaining),
			},
			LlmProxies: api.OrganizationQuota{
				Used:      llmProxiesCount,
				Limit:     intPtr(llmProxiesLimit),
				Remaining: intPtr(llmProxiesRemaining),
			},
			Applications: api.OrganizationQuota{
				Used: applicationsCount,
			},
			McpProxies: api.OrganizationQuota{
				Used:      mcpProxiesCount,
				Limit:     intPtr(mcpProxiesLimit),
				Remaining: intPtr(mcpProxiesRemaining),
			},
			Gateways: api.OrganizationQuota{
				Used: len(gateways),
			},
			Apis: api.OrganizationQuota{
				Used: len(apis),
			},
			WebsubApis: &api.OrganizationQuota{
				Used:      websubAPICount,
				Limit:     intPtr(websubAPIsLimit),
				Remaining: intPtr(websubAPIsRemaining),
			},
		},
	}

	return res, nil
}

// GetUserOrganizations returns all organizations the user belongs to.
//
// Resolution order:
//  1. JWT `organizations` claim — fast path, seeds DB as a side-effect.
//  2. JWT `organization` claim  — backward-compat for old single-org tokens.
//  3. DB membership table       — authoritative fallback when claims are absent
//     (covers the gap between org creation and the user's next JWT refresh).
func (s *OrganizationService) GetUserOrganizations(userID string, jwtOrgIDs []string, jwtOrgID string) ([]*api.Organization, error) {
	// Deduplicate and filter empty strings from JWT claim.
	seen := make(map[string]struct{})
	var ids []string
	for _, id := range jwtOrgIDs {
		if id != "" {
			if _, dup := seen[id]; !dup {
				ids = append(ids, id)
				seen[id] = struct{}{}
			}
		}
	}
	if len(ids) == 0 && jwtOrgID != "" {
		ids = []string{jwtOrgID}
	}

	if len(ids) > 0 {
		result := make([]*api.Organization, 0, len(ids))
		for _, id := range ids {
			org, err := s.orgRepo.GetOrganizationByUUID(id)
			if err != nil || org == nil {
				continue
			}
			// Keep DB in sync with IDP claims (idempotent).
			_ = s.membershipRepo.CreateMembership(userID, id, "owner")
			apiOrg, convErr := s.modelToAPI(org)
			if convErr != nil {
				s.slogger.Warn("Failed to convert org to API model", "orgID", id, "error", convErr)
				continue
			}
			result = append(result, apiOrg)
		}
		if len(result) > 0 {
			return result, nil
		}
	}

	// DB fallback: JWT claims not yet populated.
	orgs, err := s.membershipRepo.GetOrganizationsByUserID(userID)
	if err != nil {
		s.slogger.Warn("Failed to query user org memberships", "userID", userID, "error", err)
		return []*api.Organization{}, nil
	}
	result := make([]*api.Organization, 0, len(orgs))
	for _, o := range orgs {
		apiOrg, convErr := s.modelToAPI(o)
		if convErr != nil {
			s.slogger.Warn("Failed to convert org to API model", "orgID", o.ID, "error", convErr)
			continue
		}
		result = append(result, apiOrg)
	}
	return result, nil
}

// AddUserToOrganization grants membership in an existing org, persists it to the
// DB, and syncs the updated org list to the IDP (best-effort).
// adminBearerToken is the caller's access token forwarded to SCIM2 /Users/{id}.
func (s *OrganizationService) AddUserToOrganization(adminBearerToken, userID, orgID, role string) error {
	org, err := s.orgRepo.GetOrganizationByUUID(orgID)
	if err != nil || org == nil {
		return constants.ErrOrganizationNotFound
	}

	if err := s.membershipRepo.CreateMembership(userID, orgID, role); err != nil {
		return err
	}

	if s.idpClaimUpdater != nil && adminBearerToken != "" {
		orgs, listErr := s.membershipRepo.GetOrganizationsByUserID(userID)
		if listErr != nil {
			s.slogger.Warn("Failed to fetch user orgs for IDP sync after member add", "userID", userID, "error", listErr)
			return nil
		}
		orgIDs := make([]string, 0, len(orgs))
		for _, o := range orgs {
			orgIDs = append(orgIDs, o.ID)
		}
		if syncErr := s.idpClaimUpdater.UpdateUserOrgClaims(adminBearerToken, userID, orgIDs); syncErr != nil {
			s.slogger.Warn("Failed to sync org claim to IDP for added member", "targetUserID", userID, "error", syncErr)
		}
	}
	return nil
}

func (s *OrganizationService) RegisterOrganization(id string, handle string, name string, region string, userID string) (*api.Organization, error) {
	// Auto-generate handle from name if not provided
	if handle == "" {
		generated, genErr := utils.GenerateHandle(name, func(h string) bool {
			existing, _ := s.orgRepo.GetOrganizationByIdOrHandle("", h)
			return existing != nil
		})
		if genErr != nil {
			return nil, fmt.Errorf("failed to generate organization handle: %w", genErr)
		}
		handle = generated
	}

	// Check if id or handle already exists
	existingOrg, err := s.orgRepo.GetOrganizationByIdOrHandle(id, handle)
	if err != nil {
		return nil, err
	}
	if existingOrg != nil {
		if existingOrg.ID == id {
			return nil, constants.ErrOrganizationExists
		}
		return nil, constants.ErrHandleExists
	}

	if name == "" {
		name = handle // Default name to handle if not provided
	}

	// Generate default project ID upfront before persisting any data
	projectID, err := utils.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate default project ID: %w", err)
	}

	// Create organization in platform-api database first
	org := &api.Organization{
		Id:        &openapi_types.UUID{},
		Handle:    handle,
		Name:      name,
		Region:    region,
		CreatedAt: utils.TimePtrIfNotZero(time.Now()),
	}

	orgModel := s.apiToModel(org, id)
	err = s.orgRepo.CreateOrganization(orgModel)
	if err != nil {
		return nil, err
	}

	// Seed default LLM provider templates for the new organization (best-effort)
	if s.llmTemplateSeeder != nil {
		if seedErr := s.llmTemplateSeeder.SeedForOrg(id); seedErr != nil {
			s.slogger.Warn("Failed to seed default LLM templates for organization", "organization", name, "error", seedErr)
		}
	}

	// Create default DevPortal if enabled
	if s.devPortalService != nil && s.config != nil && s.config.DefaultDevPortal.Enabled {
		defaultDevPortal, devPortalErr := s.devPortalService.CreateDefaultDevPortal(id)
		if devPortalErr != nil {
			s.slogger.Warn("Failed to create default DevPortal for organization", "organization", name, "error", devPortalErr)
			// Don't fail organization creation, but log the error
		} else if defaultDevPortal != nil {
			s.slogger.Info("Created default DevPortal for organization", "devPortal", defaultDevPortal.Name, "organization", name)
		}
		// No organization sync during creation - sync happens during DevPortal activation
	}

	// Create default project for the organization
	defaultProject := &model.Project{
		ID:             projectID,
		Name:           "default",
		OrganizationID: id,
		Description:    "Default project",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err = s.projectRepo.CreateProject(defaultProject)
	orgResponse, convErr := s.modelToAPI(orgModel)
	if convErr != nil {
		return nil, convErr
	}
	if err != nil {
		// If project creation fails, return the organization anyway
		// (we don't rollback organization creation)
		return orgResponse, err
	}

	if userID != "" {
		if membershipErr := s.membershipRepo.CreateMembership(userID, id, "owner"); membershipErr != nil {
			s.slogger.Warn("Failed to create org membership for user", "userID", userID, "orgID", id, "error", membershipErr)
		}
	}

	return orgResponse, nil
}

// RegisterOrganizationForUser creates an org and syncs the updated org list back
// to the IDP so the next JWT includes it.
// bearerToken is the user's own access token forwarded to SCIM2 /Me.
// The org list synced to the IDP is fetched from the DB (authoritative) so it
// always reflects every org the user belongs to, not just what was in the JWT.
func (s *OrganizationService) RegisterOrganizationForUser(userID, bearerToken, id, handle, name, region string, _ []string) (*api.Organization, error) {
	org, err := s.RegisterOrganization(id, handle, name, region, userID)
	if err != nil {
		return nil, err
	}

	if s.idpClaimUpdater != nil && bearerToken != "" {
		// Fetch the full membership list from DB (includes the just-created org).
		dbOrgs, listErr := s.membershipRepo.GetOrganizationsByUserID(userID)
		if listErr != nil {
			s.slogger.Warn("Failed to fetch user orgs for IDP sync after org creation", "userID", userID, "error", listErr)
		} else {
			allOrgIDs := make([]string, 0, len(dbOrgs))
			for _, o := range dbOrgs {
				allOrgIDs = append(allOrgIDs, o.ID)
			}
			if syncErr := s.syncOrgClaimToIDP(bearerToken, allOrgIDs); syncErr != nil {
				s.slogger.Warn("Failed to sync org claim to IDP", "userID", userID, "error", syncErr)
			}
		}
	}

	return org, nil
}

// syncOrgClaimToIDP writes orgIDs to the IDP via SCIM2 /Me.
func (s *OrganizationService) syncOrgClaimToIDP(bearerToken string, orgIDs []string) error {
	return s.idpClaimUpdater.UpdateOrgClaims(bearerToken, orgIDs)
}

func containsStr(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func (s *OrganizationService) GetOrganizationByUUID(orgId string) (*api.Organization, error) {
	orgModel, err := s.orgRepo.GetOrganizationByUUID(orgId)
	if err != nil {
		return nil, err
	}

	if orgModel == nil {
		return nil, constants.ErrOrganizationNotFound
	}

	org, convErr := s.modelToAPI(orgModel)
	if convErr != nil {
		return nil, convErr
	}

	return org, nil
}

// Mapping functions
func (s *OrganizationService) apiToModel(org *api.Organization, id string) *model.Organization {
	if org == nil {
		return nil
	}

	createdAt := time.Now()
	if org.CreatedAt != nil {
		createdAt = *org.CreatedAt
	}

	return &model.Organization{
		ID:        id,
		Handle:    org.Handle,
		Name:      org.Name,
		Region:    org.Region,
		CreatedAt: createdAt,
		UpdatedAt: time.Now(),
	}
}

func (s *OrganizationService) modelToAPI(orgModel *model.Organization) (*api.Organization, error) {
	if orgModel == nil {
		return nil, nil
	}

	orgID, err := utils.ParseOpenAPIUUID(orgModel.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse organization ID as UUID: %w", err)
	}

	return &api.Organization{
		Id:        orgID,
		Handle:    orgModel.Handle,
		Name:      orgModel.Name,
		Region:    orgModel.Region,
		CreatedAt: utils.TimePtrIfNotZero(orgModel.CreatedAt),
		UpdatedAt: utils.TimePtrIfNotZero(orgModel.UpdatedAt),
	}, nil
}

func intPtr(value int) *int {
	return &value
}

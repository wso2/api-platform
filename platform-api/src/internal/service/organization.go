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
	"log"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"regexp"
	"strings"
	"time"

	"platform-api/src/internal/client/apiportal"
	apiportalDto "platform-api/src/internal/client/apiportal/dto"
)

type OrganizationService struct {
	orgRepo         repository.OrganizationRepository
	projectRepo     repository.ProjectRepository
	apiPortalClient *apiportal.ApiPortalClient
}

func NewOrganizationService(orgRepo repository.OrganizationRepository,
	projectRepo repository.ProjectRepository, apiPortalClient *apiportal.ApiPortalClient) *OrganizationService {
	return &OrganizationService{
		orgRepo:         orgRepo,
		projectRepo:     projectRepo,
		apiPortalClient: apiPortalClient,
	}
}

func (s *OrganizationService) RegisterOrganization(id string, handle string, name string, region string) (*dto.Organization, error) {
	// Validate handle is URL friendly
	if !s.isURLFriendly(handle) {
		return nil, constants.ErrInvalidHandle
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

	// Synchronize with api portal if enabled
	if s.apiPortalClient != nil && s.apiPortalClient.IsEnabled() {
		log.Printf("[OrganizationService] api portal enabled, synchronizing organization: %s", name)

		// Create organization in api portal
		// Map platform-api organization to apiportal format
		orgReq := &apiportalDto.OrganizationCreateRequest{
			OrgID:                  id,                    // Platform-api organization UUID
			OrgName:                name,                  // Organization display name
			OrgHandle:              handle,                // URL-friendly handle
			OrganizationIdentifier: handle,                // Use handle as identifier
			RoleClaimName:          "roles",               // Default JWT claim for roles
			GroupsClaimName:        "groups",              // Default JWT claim for groups
			OrganizationClaimName:  "organizationID",      // Default JWT claim for organization
			AdminRole:              "admin",               // Default admin role
			SubscriberRole:         "Internal/subscriber", // Default subscriber role
			SuperAdminRole:         "superAdmin",          // Default super admin role
		}

		orgResp, err := s.apiPortalClient.CreateOrganization(orgReq)
		if err != nil {
			log.Printf("[OrganizationService] Failed to create organization in api portal: %v", err)
			return nil, constants.ErrApiPortalSync
		}

		log.Printf("[OrganizationService] Organization synced to api portal: %s (ID: %s)", orgResp.OrgName, orgResp.OrgID)

		// Create default "unlimited" subscription policy
		policyReq := s.apiPortalClient.CreateDefaultSubscriptionPolicy()
		policyResp, err := s.apiPortalClient.CreateSubscriptionPolicy(id, policyReq)
		if err != nil {
			log.Printf("[OrganizationService] Failed to create subscription policy in api portal: %v", err)
			// Note: Organization already created in apiportal, but policy creation failed
			return nil, constants.ErrApiPortalSync
		}

		log.Printf("[OrganizationService] Default subscription policy created: %s (ID: %s)", policyResp.PolicyName, policyResp.ID)
	} else {
		log.Printf("[OrganizationService] api portal disabled, skipping synchronization")
	}

	// CreateOrganization organization in platform-api
	org := &dto.Organization{
		ID:        id,
		Handle:    handle,
		Name:      name,
		Region:    region,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	orgModel := s.dtoToModel(org)
	err = s.orgRepo.CreateOrganization(orgModel)
	if err != nil {
		return nil, err
	}

	// Create default project for the organization
	defaultProject := &model.Project{
		ID:             "default" + "-" + handle,
		Name:           "Default",
		OrganizationID: org.ID,
		Description:    "Default project",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err = s.projectRepo.CreateProject(defaultProject)
	if err != nil {
		// If project creation fails, roll back the organization creation
		return org, err
	}

	return org, nil
}

func (s *OrganizationService) GetOrganizationByUUID(orgId string) (*dto.Organization, error) {
	orgModel, err := s.orgRepo.GetOrganizationByUUID(orgId)
	if err != nil {
		return nil, err
	}

	if orgModel == nil {
		return nil, constants.ErrOrganizationNotFound
	}

	org := s.modelToDTO(orgModel)
	return org, nil
}

func (s *OrganizationService) isURLFriendly(handle string) bool {
	// URL friendly: lowercase letters, numbers, hyphens, underscores
	// Must start with letter, no consecutive special chars
	pattern := `^[a-z][a-z0-9_-]*[a-z0-9]$|^[a-z]$`
	matched, _ := regexp.MatchString(pattern, strings.ToLower(handle))
	return matched && handle == strings.ToLower(handle)
}

// Mapping functions
func (s *OrganizationService) dtoToModel(dto *dto.Organization) *model.Organization {
	if dto == nil {
		return nil
	}

	return &model.Organization{
		ID:        dto.ID,
		Handle:    dto.Handle,
		Name:      dto.Name,
		Region:    dto.Region,
		CreatedAt: dto.CreatedAt,
		UpdatedAt: dto.UpdatedAt,
	}
}

func (s *OrganizationService) modelToDTO(model *model.Organization) *dto.Organization {
	if model == nil {
		return nil
	}

	return &dto.Organization{
		ID:        model.ID,
		Handle:    model.Handle,
		Name:      model.Name,
		Region:    model.Region,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}

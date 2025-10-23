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
	"platform-api/src/internal/constants"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
)

// GatewayInternalAPIService handles internal gateway API operations
type GatewayInternalAPIService struct {
	apiRepo     repository.APIRepository
	gatewayRepo repository.GatewayRepository
	orgRepo     repository.OrganizationRepository
	apiUtil     *utils.APIUtil
}

// NewGatewayInternalAPIService creates a new gateway internal API service
func NewGatewayInternalAPIService(apiRepo repository.APIRepository, gatewayRepo repository.GatewayRepository,
	orgRepo repository.OrganizationRepository) *GatewayInternalAPIService {
	return &GatewayInternalAPIService{
		apiRepo:     apiRepo,
		gatewayRepo: gatewayRepo,
		orgRepo:     orgRepo,
		apiUtil:     &utils.APIUtil{},
	}
}

// GetAPIsByOrganization retrieves all APIs for a specific organization (used by gateways)
func (s *GatewayInternalAPIService) GetAPIsByOrganization(orgID string) (map[string]string, error) {
	// Get all APIs for the organization
	apis, err := s.apiRepo.GetAPIsByOrganizationID(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve APIs: %w", err)
	}

	apiYamlMap := make(map[string]string)
	for _, api := range apis {
		apiDTO := s.apiUtil.ModelToDTO(api)
		apiYaml, err := s.apiUtil.GenerateAPIDeploymentYAML(apiDTO)
		if err != nil {
			return nil, fmt.Errorf("failed to generate API YAML: %w", err)
		}
		apiYamlMap[api.ID] = apiYaml
	}
	return apiYamlMap, nil
}

// GetAPIByUUID retrieves an API by its ID
func (s *GatewayInternalAPIService) GetAPIByUUID(apiId, orgId string) (map[string]string, error) {
	apiModel, err := s.apiRepo.GetAPIByUUID(apiId)
	if err != nil {
		return nil, fmt.Errorf("failed to get api: %w", err)
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}
	if apiModel.OrganizationID != orgId {
		return nil, constants.ErrAPINotFound
	}

	apiDTO := s.apiUtil.ModelToDTO(apiModel)
	apiYaml, err := s.apiUtil.GenerateAPIDeploymentYAML(apiDTO)
	if err != nil {
		return nil, fmt.Errorf("failed to generate API YAML: %w", err)
	}
	apiYamlMap := map[string]string{
		apiDTO.ID: apiYaml,
	}
	return apiYamlMap, nil
}

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
	"log"

	"github.com/google/uuid"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
)

// UpstreamService handles backend service operations
type UpstreamService struct {
	backendServiceRepo repository.BackendServiceRepository
	apiUtil            *utils.APIUtil
}

// NewUpstreamService creates a new upstream service
func NewUpstreamService(backendServiceRepo repository.BackendServiceRepository) *UpstreamService {
	return &UpstreamService{
		backendServiceRepo: backendServiceRepo,
		apiUtil:            &utils.APIUtil{},
	}
}

// UpsertBackendService checks if a backend service exists by name in the organization
// If it exists, updates it with new configuration. If not, creates a new one.
// Returns the UUID of the backend service.
func (s *UpstreamService) UpsertBackendService(backendServiceDTO *dto.BackendService, orgId string) (string, error) {
	// ideally should throw an error if name is empty
	if backendServiceDTO.Name == "" {
		// generate uuid name if name is empty
		backendServiceName := uuid.New().String()
		backendServiceDTO.Name = backendServiceName
		log.Printf("[WARN] Backend service name is empty, generated UUID name: %s", backendServiceName)
		//return "", errors.New("backend service name is required")
	}

	// Check if backend service with the same name exists in the organization
	existingService, err := s.backendServiceRepo.GetBackendServiceByNameAndOrgID(backendServiceDTO.Name, orgId)
	if err != nil {
		return "", fmt.Errorf("failed to check existing backend service: %w", err)
	}

	if existingService != nil {
		// Backend service exists, update it with new configuration
		existingService.Description = backendServiceDTO.Description
		existingService.Retries = backendServiceDTO.Retries
		existingService.Timeout = s.apiUtil.TimeoutDTOToModel(backendServiceDTO.Timeout)
		existingService.LoadBalance = s.apiUtil.LoadBalanceDTOToModel(backendServiceDTO.LoadBalance)
		existingService.CircuitBreaker = s.apiUtil.CircuitBreakerDTOToModel(backendServiceDTO.CircuitBreaker)
		existingService.Endpoints = s.apiUtil.BackendEndpointsDTOToModel(backendServiceDTO.Endpoints)

		if err := s.backendServiceRepo.UpdateBackendService(existingService); err != nil {
			return "", fmt.Errorf("failed to update backend service: %w", err)
		}

		return existingService.ID, nil
	} else {
		// Backend service doesn't exist, create a new one
		newServiceUUID := uuid.New().String()
		newService := &model.BackendService{
			ID:             newServiceUUID,
			OrganizationID: orgId,
			Name:           backendServiceDTO.Name,
			Description:    backendServiceDTO.Description,
			Retries:        backendServiceDTO.Retries,
			Timeout:        s.apiUtil.TimeoutDTOToModel(backendServiceDTO.Timeout),
			LoadBalance:    s.apiUtil.LoadBalanceDTOToModel(backendServiceDTO.LoadBalance),
			CircuitBreaker: s.apiUtil.CircuitBreakerDTOToModel(backendServiceDTO.CircuitBreaker),
			Endpoints:      s.apiUtil.BackendEndpointsDTOToModel(backendServiceDTO.Endpoints),
		}

		if err := s.backendServiceRepo.CreateBackendService(newService); err != nil {
			return "", fmt.Errorf("failed to create backend service: %w", err)
		}

		return newServiceUUID, nil
	}
}

// AssociateBackendServiceWithAPI creates an association between an API and a backend service
func (s *UpstreamService) AssociateBackendServiceWithAPI(apiId, backendServiceId string, isDefault bool) error {
	return s.backendServiceRepo.AssociateBackendServiceWithAPI(apiId, backendServiceId, isDefault)
}

// GetBackendServicesByAPIID retrieves all backend services associated with an API
func (s *UpstreamService) GetBackendServicesByAPIID(apiId string) ([]*model.BackendService, error) {
	return s.backendServiceRepo.GetBackendServicesByAPIID(apiId)
}

// DisassociateBackendServiceFromAPI removes the association between an API and a backend service
func (s *UpstreamService) DisassociateBackendServiceFromAPI(apiId, backendServiceId string) error {
	return s.backendServiceRepo.DisassociateBackendServiceFromAPI(apiId, backendServiceId)
}

// CreateBackendServiceFromUpstream creates a backend service from upstream configuration
func (s *UpstreamService) CreateBackendServiceFromUpstream(upstream dto.Upstream, orgId string) (string, error) {
	backendService := &dto.BackendService{
		Name: uuid.New().String(), // Generate a random name since Upstream doesn't have a name field
		Endpoints: []dto.BackendEndpoint{
			{
				URL:    upstream.URL,
				Weight: 100,
			},
		},
	}

	return s.UpsertBackendService(backendService, orgId)
}

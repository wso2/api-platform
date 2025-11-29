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

package devportal_client

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	dto "platform-api/src/internal/client/devportal_client/dto"

	"github.com/go-playground/validator/v10"
)

// OrganizationsService defines organization-related operations supported by the DevPortal DevPortalClient.
type OrganizationsService interface {
	Create(req dto.OrganizationCreateRequest) (*dto.OrganizationResponse, error)
	Get(orgID string) (*dto.OrganizationResponse, error)
	List() (dto.OrganizationListResponse, error)
	Update(orgID string, req dto.OrganizationUpdateRequest) (*dto.OrganizationResponse, error)
	Delete(orgID string) error
}

// organizationsService is the concrete implementation of OrganizationsService.
type organizationsService struct {
	DevPortalClient *DevPortalClient
	validator       *validator.Validate
}

// Create posts a new organization to the DevPortal.
// Assumes endpoint POST {baseURL}/devportal/organizations
func (s *organizationsService) Create(req dto.OrganizationCreateRequest) (*dto.OrganizationResponse, error) {
	if err := s.validator.Struct(req); err != nil {
		return nil, fmt.Errorf("organization creation validation failed for orgID=%s: %w", req.OrgID, err)
	}
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath)

	httpReq, err := s.DevPortalClient.NewRequest(http.MethodPost, url).
		WithJSONBody(req).
		WithPreflightCheck().
		Build()
	if err != nil {
		return nil, err
	}
	var out dto.OrganizationResponse
	if err := s.DevPortalClient.doAndDecode(httpReq, []int{200, 201}, &out); err != nil {
		if de, ok := err.(*DevPortalError); ok && de.Code == 409 {
			// Organization might already exist - verify if it's the same one
			existing, fetchErr := s.Get(req.OrgID)
			if fetchErr != nil {
				// Distinguish between different failure scenarios
				if errors.Is(fetchErr, ErrOrganizationNotFound) {
					log.Println("Organization conflict detected but GET returned not found for orgID=", req.OrgID)
					// Race condition: org was deleted between 409 and GET
					// This is unusual but possible - treat as create failure with context
					return nil, ErrOrganizationCreationFailed
				}
				// Other errors (network, auth, etc.) - preserve context
				log.Printf("Failed to fetch existing organization during conflict resolution for orgID=%s: %v", req.OrgID, fetchErr)
				return nil, ErrOrganizationCreationFailed
			}
			// Compare key fields
			if existing.IsSameAs(req) {
				return existing, nil // Matches - treat as success
			} else {
				return nil, ErrOrganizationAlreadyExists // Mismatch - true conflict
			}
		}
		return nil, err
	}
	return &out, nil
}

// Get retrieves an organization by ID. Assumes GET {baseURL}/devportal/organizations/{orgID}
func (s *organizationsService) Get(orgID string) (*dto.OrganizationResponse, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID)
	httpReq, err := s.DevPortalClient.NewRequest(http.MethodGet, url).
		Build()
	if err != nil {
		return nil, err
	}
	var out dto.OrganizationResponse
	if err := s.DevPortalClient.doAndDecode(httpReq, []int{200}, &out); err != nil {
		// convert not-found into friendly message like before
		if de, ok := err.(*DevPortalError); ok && de.Code == 404 {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}
	return &out, nil
}

// List returns all devportal/organizations. Assumes GET {baseURL}/devportal/organizations
func (s *organizationsService) List() (dto.OrganizationListResponse, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath)
	httpReq, err := s.DevPortalClient.NewRequest(http.MethodGet, url).Build()
	if err != nil {
		return nil, err
	}
	var out dto.OrganizationListResponse
	if err := s.DevPortalClient.doAndDecode(httpReq, []int{200}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Update sends an update for an organization. Assumes PUT {baseURL}/devportal/organizations/{orgID}
func (s *organizationsService) Update(orgID string, req dto.OrganizationUpdateRequest) (*dto.OrganizationResponse, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID)
	httpReq, err := s.DevPortalClient.NewRequest(http.MethodPut, url).
		WithJSONBody(req).
		Build()
	if err != nil {
		return nil, err
	}
	var out dto.OrganizationResponse
	if err := s.DevPortalClient.doAndDecode(httpReq, []int{200}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes an organization. Assumes DELETE {baseURL}/devportal/organizations/{orgID}
func (s *organizationsService) Delete(orgID string) error {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID)
	httpReq, err := s.DevPortalClient.NewRequest(http.MethodDelete, url).
		Build()
	if err != nil {
		return err
	}
	return s.DevPortalClient.doNoContent(httpReq, []int{200, 204})
}

// Organizations returns a service for organization-related operations.
func (c *DevPortalClient) Organizations() OrganizationsService {
	return &organizationsService{
		DevPortalClient: c,
		validator:       c.validator,
	}
}

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
	dto "platform-api/src/internal/client/devportal_client/dto"
)

// ViewsService defines operations for DevPortal views.
type ViewsService interface {
	Create(orgID string, req dto.ViewRequest) (*dto.ViewResponse, error)
	Get(orgID, name string) (*dto.ViewResponse, error)
	List(orgID string) ([]dto.ViewResponse, error)
	Update(orgID, name string, req dto.ViewUpdateRequest) (*dto.ViewResponse, error)
	Delete(orgID, name string) error
}

type viewsService struct {
	DevPortalClient *DevPortalClient
}

func (s *viewsService) baseURL(orgID string) string {
	// endpoints are under /organizations/{orgId}/views
	return s.DevPortalClient.buildURL(organizationsPath, orgID, viewsPath)
}

func (s *viewsService) Create(orgID string, req dto.ViewRequest) (*dto.ViewResponse, error) {
	url := s.baseURL(orgID)
	httpReq, err := s.DevPortalClient.newJSONRequest("POST", url, req)
	if err != nil {
		return nil, err
	}
	var out dto.ViewResponse
	if err := s.DevPortalClient.doAndDecode(httpReq, []int{200, 201}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *viewsService) Get(orgID, name string) (*dto.ViewResponse, error) {
	url := s.DevPortalClient.buildURL(organizationsPath, orgID, viewsPath, name)
	httpReq, err := s.DevPortalClient.newJSONRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	var out dto.ViewResponse
	if err := s.DevPortalClient.doAndDecode(httpReq, []int{200}, &out); err != nil {
		if de, ok := err.(*DevPortalError); ok && de.Code == 404 {
			return nil, ErrViewNotFound
		}
		return nil, err
	}
	return &out, nil
}

func (s *viewsService) List(orgID string) ([]dto.ViewResponse, error) {
	url := s.baseURL(orgID)
	httpReq, err := s.DevPortalClient.newJSONRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	var out []dto.ViewResponse
	if err := s.DevPortalClient.doAndDecode(httpReq, []int{200}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *viewsService) Update(orgID, name string, req dto.ViewUpdateRequest) (*dto.ViewResponse, error) {
	url := s.DevPortalClient.buildURL(organizationsPath, orgID, viewsPath, name)
	httpReq, err := s.DevPortalClient.newJSONRequest("PUT", url, req)
	if err != nil {
		return nil, err
	}
	var out dto.ViewResponse
	if err := s.DevPortalClient.doAndDecode(httpReq, []int{200}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *viewsService) Delete(orgID, name string) error {
	url := s.DevPortalClient.buildURL(organizationsPath, orgID, viewsPath, name)
	httpReq, err := s.DevPortalClient.newJSONRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	return s.DevPortalClient.doNoContent(httpReq, []int{200, 204})
}

// Views returns a service for view-related operations.
func (c *DevPortalClient) Views() ViewsService { return &viewsService{DevPortalClient: c} }

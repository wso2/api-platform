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

import dto "platform-api/src/internal/client/devportal_client/dto"

// LabelsService manages label operations for an organization.
type LabelsService interface {
	Create(orgID string, labels []dto.Label) ([]dto.Label, error)
	// Update(orgID string, label dto.Label) (*dto.Label, error)
	List(orgID string) ([]dto.Label, error)
	// Delete(orgID string, names []string) error
}

type labelsService struct {
	DevPortalClient *DevPortalClient
}

func (s *labelsService) Create(orgID string, labels []dto.Label) ([]dto.Label, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, labelsPath)
	req, err := s.DevPortalClient.newJSONRequest("POST", url, labels)
	if err != nil {
		return nil, err
	}
	var out []dto.Label
	if err := s.DevPortalClient.doAndDecode(req, []int{200, 201}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// func (s *labelsService) Update(orgID string, label dto.Label) (*dto.Label, error) {
// 	url := s.DevPortalClient.buildURL("devportal/organizations", orgID, "labels")
// 	req, err := s.DevPortalClient.newJSONRequest("PUT", url, label)
// 	if err != nil {
// 		return nil, err
// 	}
// 	var out dto.Label
// 	if err := s.DevPortalClient.doAndDecode(req, []int{200}, &out); err != nil {
// 		return nil, err
// 	}
// 	return &out, nil
// }

func (s *labelsService) List(orgID string) ([]dto.Label, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, labelsPath)
	req, err := s.DevPortalClient.newJSONRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	var out []dto.Label
	if err := s.DevPortalClient.doAndDecode(req, []int{200}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// func (s *labelsService) Delete(orgID string, names []string) error {
// 	url := s.DevPortalClient.buildURL("devportal/organizations", orgID, "labels")
// 	req, err := s.DevPortalClient.newJSONRequest("DELETE", url, names)
// 	if err != nil {
// 		return err
// 	}
// 	return s.DevPortalClient.doNoContent(req, []int{200, 204})
// }

// Expose via DevPortalClient
func (c *DevPortalClient) Labels() LabelsService { return &labelsService{DevPortalClient: c} }

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

// SubscriptionPoliciesService manages subscription policies for an organization.
type SubscriptionPoliciesService interface {
	Create(orgID string, policies []dto.SubscriptionPolicy) ([]dto.SubscriptionPolicy, error)
	Update(orgID string, policies []dto.SubscriptionPolicy) error
	Get(orgID, policyID string) (*dto.SubscriptionPolicy, error)
	Delete(orgID, policyName string) error
}

type subscriptionPoliciesService struct {
	DevPortalClient *DevPortalClient
}

func (s *subscriptionPoliciesService) Create(orgID string, policies []dto.SubscriptionPolicy) ([]dto.SubscriptionPolicy, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, subscriptionPoliciesPath)
	req, err := s.DevPortalClient.newJSONRequest("POST", url, policies)
	if err != nil {
		return nil, err
	}
	var out []dto.SubscriptionPolicy
	if err := s.DevPortalClient.doAndDecode(req, []int{200, 201}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *subscriptionPoliciesService) Update(orgID string, policies []dto.SubscriptionPolicy) error {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, subscriptionPoliciesPath)
	req, err := s.DevPortalClient.newJSONRequest("PUT", url, policies)
	if err != nil {
		return err
	}
	if err := s.DevPortalClient.doNoContent(req, []int{200, 201}); err != nil {
		return err
	}
	return nil
}

func (s *subscriptionPoliciesService) Get(orgID, policyID string) (*dto.SubscriptionPolicy, error) {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, subscriptionPoliciesPath, policyID)
	req, err := s.DevPortalClient.newJSONRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	var out dto.SubscriptionPolicy
	if err := s.DevPortalClient.doAndDecode(req, []int{200}, &out); err != nil {
		if de, ok := err.(*DevPortalError); ok && de.Code == 404 {
			return nil, ErrSubscriptionPolicyNotFound
		}
		return nil, err
	}
	return &out, nil
}

func (s *subscriptionPoliciesService) Delete(orgID, policyName string) error {
	url := s.DevPortalClient.buildURL(devportalOrganizationsPath, orgID, subscriptionPoliciesPath, policyName)
	req, err := s.DevPortalClient.newJSONRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	return s.DevPortalClient.doNoContent(req, []int{200, 204})
}

// Expose via DevPortalClient
func (c *DevPortalClient) SubscriptionPolicies() SubscriptionPoliciesService {
	return &subscriptionPoliciesService{DevPortalClient: c}
}

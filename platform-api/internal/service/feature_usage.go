/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"errors"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/featureusage"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// featureUsageAttributor maps the platform organization onto the key WSO2
// Cloud's other Moesif publishers use, so one organization is one Moesif
// record whichever service reported it.
type featureUsageAttributor struct {
	orgRepo repository.OrganizationRepository
}

// NewFeatureUsageAttributor returns the featureusage.Attributor backed by the
// organization repository.
func NewFeatureUsageAttributor(orgRepo repository.OrganizationRepository) featureusage.Attributor {
	return &featureUsageAttributor{orgRepo: orgRepo}
}

// CompanyID returns the identity provider's organization UUID — the WSO2 Cloud
// organization UUID every other publisher keys on — and falls back to the
// platform UUID only for an organization registered without one (file-based or
// internal-token auth, where there is no other publisher to agree with).
func (a *featureUsageAttributor) CompanyID(orgUUID string) (string, error) {
	org, err := a.orgRepo.GetOrganizationByUUID(orgUUID)
	if err != nil {
		return "", err
	}
	if org == nil {
		return "", errors.New("organization not found")
	}
	if ref := strings.TrimSpace(org.IdpOrganizationRefUUID); ref != "" {
		return ref, nil
	}
	return org.ID, nil
}

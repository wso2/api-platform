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

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/featureusage"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// featureUsageAttributor maps platform identifiers onto the keys WSO2 Cloud's
// other Moesif publishers use, so one organization or user is one Moesif
// record whichever service reported it.
type featureUsageAttributor struct {
	orgRepo  repository.OrganizationRepository
	identity *IdentityService
}

// NewFeatureUsageAttributor returns the featureusage.Attributor backed by the
// organization repository and the identity mapping.
func NewFeatureUsageAttributor(orgRepo repository.OrganizationRepository, identity *IdentityService) featureusage.Attributor {
	return &featureUsageAttributor{orgRepo: orgRepo, identity: identity}
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

// UserID returns the acting user's identity-provider subject (in WSO2 Cloud,
// the Thunder user ID). A user with no mapping returns "". The reporter sends
// the value only if it is a UUID, so a service account ("sa:..." subject), an
// email-shaped subject or a federated numeric subject is never sent.
func (a *featureUsageAttributor) UserID(actorUUID string) (string, error) {
	sub, err := a.identity.SubForUUID(actorUUID)
	if err != nil {
		return "", err
	}
	if sub == constants.DeletedUser {
		return "", nil
	}
	return sub, nil
}

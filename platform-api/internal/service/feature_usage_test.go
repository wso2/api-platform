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
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

type featureUsageOrgRepo struct {
	repository.OrganizationRepository
	org *model.Organization
	err error
}

func (r featureUsageOrgRepo) GetOrganizationByUUID(string) (*model.Organization, error) {
	return r.org, r.err
}

// unmappedIdentityRepo knows no user, like a write made without an identity.
type unmappedIdentityRepo struct{ passthroughIdentityRepo }

func (unmappedIdentityRepo) GetSubByUUID(string) (string, bool, error) { return "", false, nil }

func TestFeatureUsageAttributorCompanyID(t *testing.T) {
	const platformUUID = "11111111-2222-4333-8444-555555555555"
	const idpUUID = "b3e1c2d4-7f6a-4e5b-9c8d-0a1b2c3d4e5f"
	tests := map[string]struct {
		repo    featureUsageOrgRepo
		want    string
		wantErr bool
	}{
		"prefers the identity provider's organization UUID": {
			repo: featureUsageOrgRepo{org: &model.Organization{ID: platformUUID, IdpOrganizationRefUUID: idpUUID}},
			want: idpUUID,
		},
		"falls back to the platform UUID without one": {
			repo: featureUsageOrgRepo{org: &model.Organization{ID: platformUUID}},
			want: platformUUID,
		},
		"an unknown organization is an error": {repo: featureUsageOrgRepo{}, wantErr: true},
		"a lookup failure is an error":        {repo: featureUsageOrgRepo{err: errors.New("db down")}, wantErr: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := NewFeatureUsageAttributor(tc.repo, newTestIdentityService()).CompanyID(platformUUID)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Errorf("CompanyID = %q, %v; want %q (error %v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestFeatureUsageAttributorUserID(t *testing.T) {
	mapped := NewFeatureUsageAttributor(featureUsageOrgRepo{}, newTestIdentityService())
	if got, err := mapped.UserID("0193e7a2-4c5d-7e6f-8a9b-0c1d2e3f4a5b"); err != nil || got != "0193e7a2-4c5d-7e6f-8a9b-0c1d2e3f4a5b" {
		t.Errorf("UserID = %q, %v; want the subject", got, err)
	}
	unmapped := NewFeatureUsageAttributor(featureUsageOrgRepo{}, NewIdentityService(unmappedIdentityRepo{}))
	if got, err := unmapped.UserID("ghost"); err != nil || got != "" {
		t.Errorf("UserID for an unmapped user = %q, %v; want no user", got, err)
	}
}

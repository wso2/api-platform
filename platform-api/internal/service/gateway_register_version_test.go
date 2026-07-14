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
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// TestRegisterGatewayVersionStored guards the fix for the e2e 404 regression:
// a version-less registration must be stored as "" (unknown), NOT a fabricated
// "1.0". The deploy transform maps "" to the latest gateway data version (v1);
// stamping "1.0" here would wrongly down-convert every artifact to v1alpha1 for
// a current gateway that then refuses to route it. A positively supplied
// version is still stored (major.minor canonicalized).
func TestRegisterGatewayVersionStored(t *testing.T) {
	orgID := "123e4567-e89b-12d3-a456-426614174000"

	tests := []struct {
		name     string
		supplied string
		want     string
	}{
		{"no version is stored as empty (assume latest, not 1.0)", "", ""},
		{"blank version is stored as empty", "   ", ""},
		{"supplied major.minor is canonicalized and stored", "1.1", "1.1"},
		{"supplied full semver is stored verbatim", "1.2.0", "1.2.0"},
		{"CalVer is stored verbatim", "2026.05.13", "2026.05.13"},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGatewayRepo := &mockGatewayRepository{}
			service := &GatewayService{
				gatewayRepo: mockGatewayRepo,
				orgRepo:     &mockOrganizationRepository{org: &model.Organization{ID: orgID}},
				auditRepo:   &noopAuditRepo{},
				identity:    newTestIdentityService(),
			}

			// Unique handle per case so uniqueness checks never collide.
			gatewayID := "gw-version-case-" + string(rune('a'+i))
			_, err := service.RegisterGateway(
				orgID,
				&gatewayID,
				"Version Case Gateway",
				"",
				[]string{"https://api.example.com"},
				false,
				constants.GatewayFunctionalityTypeRegular,
				tt.supplied,
				"test-user",
				nil,
			)
			if err != nil {
				t.Fatalf("RegisterGateway() error = %v", err)
			}
			if mockGatewayRepo.createdGateway == nil {
				t.Fatalf("Create() was not called")
			}
			if got := mockGatewayRepo.createdGateway.Version; got != tt.want {
				t.Errorf("stored gateway version = %q, want %q", got, tt.want)
			}
		})
	}
}

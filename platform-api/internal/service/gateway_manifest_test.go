/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

type gatewayManifestRepository struct {
	repository.GatewayRepository
	gateway            *model.Gateway
	manifest           []byte
	lookupHandle       string
	lookupOrganization string
	manifestGatewayID  string
}

func (r *gatewayManifestRepository) GetByHandleAndOrgID(handle, orgID string) (*model.Gateway, error) {
	r.lookupHandle = handle
	r.lookupOrganization = orgID
	return r.gateway, nil
}

func (r *gatewayManifestRepository) GetGatewayManifest(gatewayID string) ([]byte, error) {
	r.manifestGatewayID = gatewayID
	return r.manifest, nil
}

func TestGetStoredManifestResolvesGatewayHandleToUUID(t *testing.T) {
	const (
		gatewayHandle = "test"
		gatewayUUID   = "019f6a6f-8db4-797d-a894-15d8577c4b44"
		orgID         = "019f6a6e-774d-717d-875c-bb11413aa38e"
	)
	wantManifest := []byte(`[{"name":"rate-limit"}]`)
	repo := &gatewayManifestRepository{
		gateway: &model.Gateway{
			ID:             gatewayUUID,
			Handle:         gatewayHandle,
			OrganizationID: orgID,
		},
		manifest: wantManifest,
	}
	service := &GatewayService{gatewayRepo: repo}

	manifest, err := service.GetStoredManifest(gatewayHandle, orgID)
	if err != nil {
		t.Fatalf("GetStoredManifest() error = %v", err)
	}
	if repo.lookupHandle != gatewayHandle || repo.lookupOrganization != orgID {
		t.Errorf("gateway lookup = (%q, %q), want (%q, %q)",
			repo.lookupHandle, repo.lookupOrganization, gatewayHandle, orgID)
	}
	if repo.manifestGatewayID != gatewayUUID {
		t.Errorf("manifest lookup ID = %q, want gateway UUID %q", repo.manifestGatewayID, gatewayUUID)
	}
	if string(manifest.Policies) != string(wantManifest) {
		t.Errorf("manifest policies = %s, want %s", manifest.Policies, wantManifest)
	}
}

func TestGetStoredManifestReturnsNotFoundForUnknownHandle(t *testing.T) {
	service := &GatewayService{gatewayRepo: &gatewayManifestRepository{}}

	_, err := service.GetStoredManifest("unknown", "org-id")
	if err == nil {
		t.Fatal("GetStoredManifest() expected gateway-not-found error, got nil")
	}
}

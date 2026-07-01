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
	"reflect"
	"strings"
	"testing"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
)

// TestRegisterGatewayEndpoints covers endpoint validation and persistence on CREATE:
//  1. endpoints nil              — must fail: at least one endpoint required
//  2. endpoints empty slice      — must fail: at least one endpoint required
//  3. endpoint with empty string — must fail: endpoint must not be empty
//  4. endpoint too long          — must fail: endpoint must not exceed 255 characters
//  5. single endpoint            — valid, stored and returned
//  6. multiple endpoints         — valid, order preserved
func TestRegisterGatewayEndpoints(t *testing.T) {
	const orgID = "123e4567-e89b-12d3-a456-426614174000"

	newService := func() (*GatewayService, *mockGatewayRepository) {
		repo := &mockGatewayRepository{}
		svc := &GatewayService{
			gatewayRepo: repo,
			orgRepo:     &mockOrganizationRepository{org: &model.Organization{ID: orgID}},
			auditRepo:   &noopAuditRepo{},
		}
		return svc, repo
	}

	register := func(svc *GatewayService, handle string, endpoints []string) (*api.GatewayResponse, error) {
		return svc.RegisterGateway(
			orgID, &handle, "Production Gateway", "", endpoints,
			false, constants.GatewayFunctionalityTypeRegular, "1.0", "test-user", nil,
		)
	}

	t.Run("endpoints nil — rejected", func(t *testing.T) {
		svc, _ := newService()
		_, err := register(svc, "gw-no-endpoints", nil)
		if err == nil {
			t.Fatal("RegisterGateway() expected error for nil endpoints, got nil")
		}
		if err.Error() != "at least one endpoint is required" {
			t.Errorf("error = %q, want %q", err.Error(), "at least one endpoint is required")
		}
	})

	t.Run("endpoints empty slice — rejected", func(t *testing.T) {
		svc, _ := newService()
		_, err := register(svc, "gw-empty-endpoints", []string{})
		if err == nil {
			t.Fatal("RegisterGateway() expected error for empty endpoints, got nil")
		}
		if err.Error() != "at least one endpoint is required" {
			t.Errorf("error = %q, want %q", err.Error(), "at least one endpoint is required")
		}
	})

	t.Run("endpoint with empty string — rejected", func(t *testing.T) {
		svc, _ := newService()
		_, err := register(svc, "gw-empty-url", []string{""})
		if err == nil {
			t.Fatal("RegisterGateway() expected error for empty endpoint, got nil")
		}
		if !strings.Contains(err.Error(), "endpoint must not be empty") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "endpoint must not be empty")
		}
	})

	t.Run("endpoint too long — rejected", func(t *testing.T) {
		svc, _ := newService()
		longEndpoint := "https://" + strings.Repeat("a", 250) + ".example.com"
		_, err := register(svc, "gw-long-url", []string{longEndpoint})
		if err == nil {
			t.Fatal("RegisterGateway() expected error for endpoint exceeding max length, got nil")
		}
		if !strings.Contains(err.Error(), "endpoint must not exceed 255 characters") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "endpoint must not exceed 255 characters")
		}
	})

	t.Run("single endpoint", func(t *testing.T) {
		svc, repo := newService()
		endpoints := []string{"https://api.example.com:8443"}
		response, err := register(svc, "gw-single-endpoint", endpoints)
		if err != nil {
			t.Fatalf("RegisterGateway() error = %v", err)
		}
		if repo.createdGateway == nil {
			t.Fatal("Create() was not called")
		}
		if !reflect.DeepEqual(repo.createdGateway.Endpoints, endpoints) {
			t.Errorf("stored endpoints = %v, want %v", repo.createdGateway.Endpoints, endpoints)
		}
		if response.Endpoints == nil || !reflect.DeepEqual(*response.Endpoints, endpoints) {
			t.Errorf("response endpoints = %v, want %v", response.Endpoints, endpoints)
		}
	})

	t.Run("multiple endpoints — order preserved", func(t *testing.T) {
		svc, repo := newService()
		endpoints := []string{
			"https://api.example.com:8443",
			"wss://events.example.com:8444",
		}
		response, err := register(svc, "gw-multi-endpoint", endpoints)
		if err != nil {
			t.Fatalf("RegisterGateway() error = %v", err)
		}
		if repo.createdGateway == nil {
			t.Fatal("Create() was not called")
		}
		if !reflect.DeepEqual(repo.createdGateway.Endpoints, endpoints) {
			t.Errorf("stored endpoints = %v, want %v", repo.createdGateway.Endpoints, endpoints)
		}
		if response.Endpoints == nil || !reflect.DeepEqual(*response.Endpoints, endpoints) {
			t.Errorf("response endpoints = %v, want %v", response.Endpoints, endpoints)
		}
	})
}

// TestUpdateGatewayPreservesEndpoints locks in that endpoints are immutable after creation —
// UpdateGateway never reads api.GatewayResponse.Endpoints, so whatever the gateway already has
// (as loaded from the repository) must survive an update untouched, same as vhost was immutable.
func TestUpdateGatewayPreservesEndpoints(t *testing.T) {
	const orgID = "123e4567-e89b-12d3-a456-426614174001"
	const gatewayID = "123e4567-e89b-12d3-a456-426614174002"

	existingEndpoints := []string{"https://old.example.com:8080"}

	baseGateway := &model.Gateway{
		ID:             gatewayID,
		OrganizationID: orgID,
		Handle:         "my-gateway",
		Endpoints:      existingEndpoints,
	}

	repo := &mockGatewayRepository{getByNameResult: baseGateway}
	svc := &GatewayService{
		gatewayRepo: repo,
		orgRepo:     &mockOrganizationRepository{org: &model.Organization{ID: orgID, Handle: "test-org"}},
		auditRepo:   &noopAuditRepo{},
	}

	newDescription := "Updated description"
	attemptedEndpoints := []string{"https://attempted-override.example.com"}
	response, err := svc.UpdateGateway(gatewayID, orgID, "test-user", &api.GatewayResponse{
		DisplayName: baseGateway.Name,
		Description: &newDescription,
		Endpoints:   &attemptedEndpoints,
	})
	if err != nil {
		t.Fatalf("UpdateGateway() error = %v", err)
	}

	if repo.updatedGateway == nil {
		t.Fatal("UpdateGateway() did not call the repository's UpdateGateway method")
	}
	if !reflect.DeepEqual(repo.updatedGateway.Endpoints, existingEndpoints) {
		t.Errorf("stored endpoints = %v, want existing endpoints %v to be preserved", repo.updatedGateway.Endpoints, existingEndpoints)
	}
	if response.Endpoints == nil || !reflect.DeepEqual(*response.Endpoints, existingEndpoints) {
		t.Errorf("response endpoints = %v, want existing endpoints %v to be preserved", response.Endpoints, existingEndpoints)
	}
}

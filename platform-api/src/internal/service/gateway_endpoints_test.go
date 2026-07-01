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
	"testing"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
)

// TestRegisterGatewayEndpoints covers endpoint validation on CREATE:
//  1. endpoints nil              — must fail: at least one endpoint required
//  2. endpoints empty slice      — must fail: at least one endpoint required
//  3. endpoint with empty string — must fail: url is required
//  4. endpoint with invalid URL  — must fail: not a valid URL
//  5. single endpoint            — valid
//  6. multiple endpoints         — valid
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
			orgID, handle, "Production Gateway", "",
			false, constants.GatewayFunctionalityTypeRegular, "1.0", "test-user",
			nil, endpoints,
		)
	}

	t.Run("endpoint with empty string — rejected", func(t *testing.T) {
		svc, _ := newService()
		_, err := register(svc, "gw-empty-url", []string{""})
		if err == nil {
			t.Fatal("RegisterGateway() expected error for empty url, got nil")
		}
		if !contains(err.Error(), "url is required") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "url is required")
		}
	})

	t.Run("endpoint with invalid URL — rejected", func(t *testing.T) {
		svc, _ := newService()
		_, err := register(svc, "gw-bad-url", []string{"not-a-url"})
		if err == nil {
			t.Fatal("RegisterGateway() expected error for invalid url, got nil")
		}
		if !contains(err.Error(), "not a valid URL") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "not a valid URL")
		}
	})

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

	t.Run("multiple endpoints", func(t *testing.T) {
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

// TestUpdateGatewayEndpoints covers endpoint validation on UPDATE:
//  1. nil endpoints pointer       — not an update; existing endpoints preserved
//  2. empty endpoints slice       — must fail: at least one endpoint required
//  3. endpoint with empty string  — must fail: url is required
//  4. endpoint with invalid URL   — must fail: not a valid URL
//  5. valid replacement           — stored and returned correctly
func TestUpdateGatewayEndpoints(t *testing.T) {
	const orgID = "123e4567-e89b-12d3-a456-426614174000"
	const gatewayID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	existingEndpoints := []string{"https://old.example.com:8080"}

	newService := func() (*GatewayService, *mockGatewayRepository) {
		eps := make([]string, len(existingEndpoints))
		copy(eps, existingEndpoints)
		gw := &model.Gateway{
			ID:             gatewayID,
			OrganizationID: orgID,
			Name:           "my-gateway",
			Endpoints:      eps,
		}
		repo := &mockGatewayRepository{getByUUIDResult: gw}
		svc := &GatewayService{
			gatewayRepo: repo,
			auditRepo:   &noopAuditRepo{},
		}
		return svc, repo
	}

	update := func(svc *GatewayService, endpoints *[]string) (*api.GatewayResponse, error) {
		return svc.UpdateGateway(gatewayID, orgID, "test-user", nil, nil, nil, nil, endpoints)
	}

	t.Run("nil endpoints — existing preserved", func(t *testing.T) {
		svc, repo := newService()
		_, err := update(svc, nil)
		if err != nil {
			t.Fatalf("UpdateGateway() unexpected error = %v", err)
		}
		if !reflect.DeepEqual(repo.updatedGateway.Endpoints, existingEndpoints) {
			t.Errorf("endpoints = %v, want existing %v", repo.updatedGateway.Endpoints, existingEndpoints)
		}
	})

	t.Run("empty endpoints slice — rejected", func(t *testing.T) {
		svc, _ := newService()
		eps := []string{}
		_, err := update(svc, &eps)
		if err == nil {
			t.Fatal("UpdateGateway() expected error for empty endpoints, got nil")
		}
		if !contains(err.Error(), "at least one endpoint is required") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "at least one endpoint is required")
		}
	})

	t.Run("endpoint with empty string — rejected", func(t *testing.T) {
		svc, _ := newService()
		eps := []string{""}
		_, err := update(svc, &eps)
		if err == nil {
			t.Fatal("UpdateGateway() expected error for empty url, got nil")
		}
		if !contains(err.Error(), "url is required") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "url is required")
		}
	})

	t.Run("endpoint with invalid URL — rejected", func(t *testing.T) {
		svc, _ := newService()
		eps := []string{"not-a-url"}
		_, err := update(svc, &eps)
		if err == nil {
			t.Fatal("UpdateGateway() expected error for invalid url, got nil")
		}
		if !contains(err.Error(), "not a valid URL") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "not a valid URL")
		}
	})

	t.Run("valid replacement endpoints — stored and returned", func(t *testing.T) {
		svc, repo := newService()
		newEps := []string{"wss://new.example.com:9000"}
		resp, err := update(svc, &newEps)
		if err != nil {
			t.Fatalf("UpdateGateway() error = %v", err)
		}
		if !reflect.DeepEqual(repo.updatedGateway.Endpoints, newEps) {
			t.Errorf("stored endpoints = %v, want %v", repo.updatedGateway.Endpoints, newEps)
		}
		if resp == nil || resp.Endpoints == nil || len(*resp.Endpoints) != 1 {
			t.Fatalf("response endpoints = %v, want 1 entry", resp.Endpoints)
		}
		if (*resp.Endpoints)[0] != "wss://new.example.com:9000" {
			t.Errorf("response endpoint = %q, want %q", (*resp.Endpoints)[0], "wss://new.example.com:9000")
		}
	})
}

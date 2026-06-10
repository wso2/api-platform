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
//  1. endpoints omitted (nil)       — must fail: at least one endpoint required
//  2. endpoints empty slice         — must fail: at least one endpoint required
//  3. single endpoint               — valid
//  4. multiple endpoints            — valid
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

	register := func(svc *GatewayService, handle string, endpoints []model.GatewayEndpoint) (*api.GatewayResponse, error) {
		return svc.RegisterGateway(
			orgID, handle, "Production Gateway", "",
			false, constants.GatewayFunctionalityTypeRegular, "1.0", "test-user",
			nil, endpoints,
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

		_, err := register(svc, "gw-empty-endpoints", []model.GatewayEndpoint{})
		if err == nil {
			t.Fatal("RegisterGateway() expected error for empty endpoints, got nil")
		}
		if err.Error() != "at least one endpoint is required" {
			t.Errorf("error = %q, want %q", err.Error(), "at least one endpoint is required")
		}
	})

	t.Run("single endpoint", func(t *testing.T) {
		svc, repo := newService()

		endpoints := []model.GatewayEndpoint{
			{Host: "api.example.com", Protocol: "https", Port: 8443},
		}

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

		emptyCtx := ""
		want := []api.GatewayEndpoint{
			{Host: "api.example.com", Protocol: "https", Port: 8443, Context: &emptyCtx},
		}
		if response.Endpoints == nil || !reflect.DeepEqual(*response.Endpoints, want) {
			t.Errorf("response endpoints = %v, want %v", response.Endpoints, want)
		}
	})

	t.Run("multiple endpoints", func(t *testing.T) {
		svc, repo := newService()

		endpoints := []model.GatewayEndpoint{
			{Host: "api.example.com", Protocol: "https", Port: 8443},
			{Host: "events.example.com", Protocol: "wss", Port: 8444},
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

		emptyCtx := ""
		want := []api.GatewayEndpoint{
			{Host: "api.example.com", Protocol: "https", Port: 8443, Context: &emptyCtx},
			{Host: "events.example.com", Protocol: "wss", Port: 8444, Context: &emptyCtx},
		}
		if response.Endpoints == nil || !reflect.DeepEqual(*response.Endpoints, want) {
			t.Errorf("response endpoints = %v, want %v", response.Endpoints, want)
		}
	})
}

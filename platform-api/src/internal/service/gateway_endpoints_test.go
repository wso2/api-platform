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

	t.Run("endpoint with empty host — rejected", func(t *testing.T) {
		svc, _ := newService()
		_, err := register(svc, "gw-bad-host", []model.GatewayEndpoint{
			{Host: "", Protocol: "https", Port: 443},
		})
		if err == nil {
			t.Fatal("RegisterGateway() expected error for empty host, got nil")
		}
		if !contains(err.Error(), "host is required") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "host is required")
		}
	})

	t.Run("endpoint with empty protocol — rejected", func(t *testing.T) {
		svc, _ := newService()
		_, err := register(svc, "gw-bad-proto", []model.GatewayEndpoint{
			{Host: "api.example.com", Protocol: "", Port: 443},
		})
		if err == nil {
			t.Fatal("RegisterGateway() expected error for empty protocol, got nil")
		}
		if !contains(err.Error(), "protocol is required") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "protocol is required")
		}
	})

	t.Run("endpoint with port out of range — rejected", func(t *testing.T) {
		svc, _ := newService()
		_, err := register(svc, "gw-bad-port", []model.GatewayEndpoint{
			{Host: "api.example.com", Protocol: "https", Port: 99999},
		})
		if err == nil {
			t.Fatal("RegisterGateway() expected error for out-of-range port, got nil")
		}
		if !contains(err.Error(), "port must be between") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "port must be between")
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

// TestUpdateGatewayEndpoints covers endpoint validation on UPDATE:
//  1. nil endpoints pointer    — not an update; existing endpoints preserved
//  2. empty endpoints slice    — must fail: at least one endpoint required
//  3. endpoint with empty host — must fail: host is required
//  4. endpoint with bad port   — must fail: port must be between 1 and 65535
//  5. valid replacement        — stored and returned correctly
func TestUpdateGatewayEndpoints(t *testing.T) {
	const orgID = "123e4567-e89b-12d3-a456-426614174000"
	const gatewayID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	existingEndpoints := []model.GatewayEndpoint{
		{Host: "old.example.com", Protocol: "https", Port: 8080},
	}

	newService := func() (*GatewayService, *mockGatewayRepository) {
		eps := make([]model.GatewayEndpoint, len(existingEndpoints))
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

	update := func(svc *GatewayService, endpoints *[]model.GatewayEndpoint) (*api.GatewayResponse, error) {
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
		eps := []model.GatewayEndpoint{}
		_, err := update(svc, &eps)
		if err == nil {
			t.Fatal("UpdateGateway() expected error for empty endpoints, got nil")
		}
		if !contains(err.Error(), "at least one endpoint is required") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "at least one endpoint is required")
		}
	})

	t.Run("endpoint with empty host — rejected", func(t *testing.T) {
		svc, _ := newService()
		eps := []model.GatewayEndpoint{{Host: "", Protocol: "https", Port: 443}}
		_, err := update(svc, &eps)
		if err == nil {
			t.Fatal("UpdateGateway() expected error for empty host, got nil")
		}
		if !contains(err.Error(), "host is required") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "host is required")
		}
	})

	t.Run("endpoint with invalid port — rejected", func(t *testing.T) {
		svc, _ := newService()
		eps := []model.GatewayEndpoint{{Host: "api.example.com", Protocol: "https", Port: 65536}}
		_, err := update(svc, &eps)
		if err == nil {
			t.Fatal("UpdateGateway() expected error for out-of-range port, got nil")
		}
		if !contains(err.Error(), "port must be between") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "port must be between")
		}
	})

	t.Run("valid replacement endpoints — stored and returned", func(t *testing.T) {
		svc, repo := newService()
		newEps := []model.GatewayEndpoint{
			{Host: "new.example.com", Protocol: "wss", Port: 9000},
		}
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
		if string((*resp.Endpoints)[0].Protocol) != "wss" {
			t.Errorf("response protocol = %q, want %q", (*resp.Endpoints)[0].Protocol, "wss")
		}
	})
}

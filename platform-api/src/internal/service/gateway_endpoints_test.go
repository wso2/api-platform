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

	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
)

// TestRegisterGatewayEndpoints covers the two supported CREATE combinations:
//  1. vhost only (endpoints omitted)  — endpoints is optional, vhost is required
//  2. both vhost and endpoints present — all values stored and returned
//
// A "single endpoint" sub-case is included alongside the multi-endpoint case.
func TestRegisterGatewayEndpoints(t *testing.T) {
	const orgID = "123e4567-e89b-12d3-a456-426614174000"

	newService := func() (*GatewayService, *mockGatewayRepository) {
		repo := &mockGatewayRepository{}
		svc := &GatewayService{
			gatewayRepo: repo,
			orgRepo:     &mockOrganizationRepository{org: &model.Organization{ID: orgID}},
		}
		return svc, repo
	}

	t.Run("vhost only, no endpoints", func(t *testing.T) {
		svc, repo := newService()

		response, err := svc.RegisterGateway(
			orgID, "gw-vhost-only", "Vhost Only Gateway", "", "api.example.com",
			false, constants.GatewayFunctionalityTypeRegular, "1.0", nil, nil,
		)
		if err != nil {
			t.Fatalf("RegisterGateway() unexpected error: %v", err)
		}
		if response == nil {
			t.Fatal("RegisterGateway() returned nil response")
		}

		// vhost stored correctly
		if repo.createdGateway.Vhost != "api.example.com" {
			t.Errorf("createdGateway.Vhost = %q, want %q", repo.createdGateway.Vhost, "api.example.com")
		}

		// endpoints stored as nil in model (nothing was provided)
		if repo.createdGateway.Endpoints != nil {
			t.Errorf("createdGateway.Endpoints = %v, want nil", repo.createdGateway.Endpoints)
		}

		// response reflects the vhost
		if response.Vhost == nil || *response.Vhost != "api.example.com" {
			t.Errorf("response.Vhost = %v, want \"api.example.com\"", response.Vhost)
		}

		// response endpoints is a pointer to an empty slice (not nil) so it serialises as []
		if response.Endpoints == nil {
			t.Error("response.Endpoints is nil, want pointer to empty slice")
		} else if len(*response.Endpoints) != 0 {
			t.Errorf("response.Endpoints length = %d, want 0", len(*response.Endpoints))
		}
	})

	t.Run("both vhost and single endpoint", func(t *testing.T) {
		svc, repo := newService()

		input := []model.GatewayEndpoint{
			{Host: "api.example.com", Protocol: "https", Port: 443},
		}

		response, err := svc.RegisterGateway(
			orgID, "gw-single-ep", "Single Endpoint Gateway", "", "api.example.com",
			false, constants.GatewayFunctionalityTypeRegular, "1.0", nil, input,
		)
		if err != nil {
			t.Fatalf("RegisterGateway() unexpected error: %v", err)
		}
		if response == nil {
			t.Fatal("RegisterGateway() returned nil response")
		}

		// vhost and endpoint stored correctly
		if repo.createdGateway.Vhost != "api.example.com" {
			t.Errorf("createdGateway.Vhost = %q, want %q", repo.createdGateway.Vhost, "api.example.com")
		}
		if !reflect.DeepEqual(repo.createdGateway.Endpoints, input) {
			t.Errorf("createdGateway.Endpoints = %v, want %v", repo.createdGateway.Endpoints, input)
		}

		// response reflects both
		if response.Vhost == nil || *response.Vhost != "api.example.com" {
			t.Errorf("response.Vhost = %v, want \"api.example.com\"", response.Vhost)
		}
		if response.Endpoints == nil {
			t.Fatal("response.Endpoints is nil")
		}
		if len(*response.Endpoints) != 1 {
			t.Fatalf("response.Endpoints length = %d, want 1", len(*response.Endpoints))
		}
		ep := (*response.Endpoints)[0]
		if ep.Host != "api.example.com" || string(ep.Protocol) != "https" || int(ep.Port) != 443 {
			t.Errorf("response endpoint = {host:%s protocol:%s port:%d}, want {api.example.com https 443}",
				ep.Host, ep.Protocol, ep.Port)
		}
	})

	t.Run("both vhost and multiple endpoints", func(t *testing.T) {
		svc, repo := newService()

		input := []model.GatewayEndpoint{
			{Host: "api.example.com", Protocol: "https", Port: 8443},
			{Host: "events.example.com", Protocol: "wss", Port: 8444},
			{Host: "events.example.com", Protocol: "sse", Port: 8445},
		}

		response, err := svc.RegisterGateway(
			orgID, "gw-multi-ep", "Multi Endpoint Gateway", "", "api.example.com",
			false, constants.GatewayFunctionalityTypeRegular, "1.0", nil, input,
		)
		if err != nil {
			t.Fatalf("RegisterGateway() unexpected error: %v", err)
		}
		if response == nil {
			t.Fatal("RegisterGateway() returned nil response")
		}

		if !reflect.DeepEqual(repo.createdGateway.Endpoints, input) {
			t.Errorf("createdGateway.Endpoints = %v, want %v", repo.createdGateway.Endpoints, input)
		}

		if response.Endpoints == nil {
			t.Fatal("response.Endpoints is nil")
		}
		if len(*response.Endpoints) != len(input) {
			t.Fatalf("response.Endpoints length = %d, want %d", len(*response.Endpoints), len(input))
		}
		for i, ep := range *response.Endpoints {
			if ep.Host != input[i].Host || string(ep.Protocol) != input[i].Protocol || int(ep.Port) != input[i].Port {
				t.Errorf("response endpoint[%d] = {host:%s protocol:%s port:%d}, want {%s %s %d}",
					i, ep.Host, ep.Protocol, ep.Port, input[i].Host, input[i].Protocol, input[i].Port)
			}
		}
	})
}

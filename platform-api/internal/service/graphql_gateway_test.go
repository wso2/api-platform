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

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// TestGraphQLAddGatewaysToAPI_CreatesAssociationAndReturnsList verifies
// AddGatewaysToAPI resolves the handle, validates the gateway, creates a new
// association (via the shared artifact_gateway_mappings helpers — see
// GraphQLAPIRepository's doc comment), and returns the up-to-date gateway
// list, mirroring APIService.AddGatewaysToAPI's behavior for REST.
func TestGraphQLAddGatewaysToAPI_CreatesAssociationAndReturnsList(t *testing.T) {
	stored := &model.GraphQLAPI{
		ID:             "gql-uuid-1",
		Handle:         "countries-graphql-api",
		OrganizationID: "org-1",
		Configuration:  model.GraphQLAPIConfig{SDL: validCountriesGraphQLSDL},
	}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
			if handle == stored.Handle && orgUUID == stored.OrganizationID {
				return stored, nil
			}
			return nil, nil
		},
		gatewayDetails: []*model.APIGatewayWithDetails{
			{ID: "gw-uuid-1", Handle: "prod-gateway", Name: "Prod Gateway"},
		},
	}
	gatewayRepo := &mockGatewayRepository{
		getByNameResult: &model.Gateway{ID: "gw-uuid-1", Handle: "prod-gateway", Name: "Prod Gateway", OrganizationID: "org-1"},
	}
	orgRepo := &mockOrganizationRepo{org: &model.Organization{ID: "org-1", Handle: "acme"}}

	svc := newGraphQLTestServiceWithGateways(repo, nil, gatewayRepo, orgRepo)

	resp, err := svc.AddGatewaysToAPI("countries-graphql-api", []string{"prod-gateway"}, "org-1", "creator-uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected a response, got nil")
	}
	if len(repo.createdAssociations) != 1 {
		t.Fatalf("expected exactly one association to be created, got %d", len(repo.createdAssociations))
	}
	assoc := repo.createdAssociations[0]
	if assoc.ArtifactID != stored.ID {
		t.Errorf("expected association ArtifactID %q, got %q", stored.ID, assoc.ArtifactID)
	}
	if assoc.GatewayID != "gw-uuid-1" {
		t.Errorf("expected association GatewayID %q, got %q", "gw-uuid-1", assoc.GatewayID)
	}
	if len(resp.List) != 1 || resp.List[0].Id == nil || *resp.List[0].Id != "prod-gateway" {
		t.Errorf("expected the returned gateway list to include prod-gateway, got: %+v", resp.List)
	}
}

// TestGraphQLAddGatewaysToAPI_UnknownGateway_NotFound verifies a gateway handle
// that doesn't resolve within the org is rejected before any association is
// written.
func TestGraphQLAddGatewaysToAPI_UnknownGateway_NotFound(t *testing.T) {
	stored := &model.GraphQLAPI{ID: "gql-uuid-1", Handle: "countries-graphql-api", OrganizationID: "org-1"}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
	}
	gatewayRepo := &mockGatewayRepository{getByNameResult: nil}
	orgRepo := &mockOrganizationRepo{}

	svc := newGraphQLTestServiceWithGateways(repo, nil, gatewayRepo, orgRepo)

	_, err := svc.AddGatewaysToAPI("countries-graphql-api", []string{"does-not-exist"}, "org-1", "creator-uuid")
	if err == nil {
		t.Fatal("expected an error for an unknown gateway handle")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGatewayNotFound {
		t.Errorf("expected %s, got %s", apperror.CodeGatewayNotFound, code)
	}
	if len(repo.createdAssociations) != 0 {
		t.Error("expected no association to be created for an unknown gateway")
	}
}

// TestGraphQLGetAPIGateways_ReturnsAssociatedGateways verifies GetAPIGateways
// resolves the handle and returns the paginated gateway list for the artifact.
func TestGraphQLGetAPIGateways_ReturnsAssociatedGateways(t *testing.T) {
	stored := &model.GraphQLAPI{ID: "gql-uuid-1", Handle: "countries-graphql-api", OrganizationID: "org-1"}
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) { return stored, nil },
		gatewayDetails: []*model.APIGatewayWithDetails{
			{ID: "gw-uuid-1", Handle: "prod-gateway", Name: "Prod Gateway"},
			{ID: "gw-uuid-2", Handle: "staging-gateway", Name: "Staging Gateway"},
		},
	}
	orgRepo := &mockOrganizationRepo{org: &model.Organization{ID: "org-1", Handle: "acme"}}
	svc := newGraphQLTestServiceWithGateways(repo, nil, &mockGatewayRepository{}, orgRepo)

	resp, err := svc.GetAPIGateways("countries-graphql-api", "org-1", 25, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || len(resp.List) != 2 {
		t.Fatalf("expected 2 associated gateways, got: %+v", resp)
	}
	if resp.Pagination.Total != 2 {
		t.Errorf("expected pagination total 2, got %d", resp.Pagination.Total)
	}
}

// TestGraphQLGetAPIGateways_NotFound verifies a nonexistent GraphQL API handle
// returns GRAPHQL_API_NOT_FOUND rather than an empty gateway list.
func TestGraphQLGetAPIGateways_NotFound(t *testing.T) {
	repo := &mockGraphQLAPIRepo{
		getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) { return nil, nil },
	}
	svc := newGraphQLTestServiceWithGateways(repo, nil, &mockGatewayRepository{}, &mockOrganizationRepo{})

	_, err := svc.GetAPIGateways("does-not-exist", "org-1", 25, 0)
	if err == nil {
		t.Fatal("expected an error for a nonexistent GraphQL API")
	}
	if code := graphQLCatalogCode(t, err); code != apperror.CodeGraphQLAPINotFound {
		t.Errorf("expected %s, got %s", apperror.CodeGraphQLAPINotFound, code)
	}
}

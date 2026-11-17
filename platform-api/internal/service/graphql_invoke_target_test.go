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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// deployedGraphQLArtifact is a GraphQL API deployment as stored.
func deployedGraphQLArtifact(context string) string {
	return "apiVersion: gateway.api-platform.wso2.com/v1alpha1\nkind: GraphQLApi\nmetadata:\n  name: countries\n" +
		"spec:\n  displayName: Countries\n  version: v1\n  context: " + context + "\n"
}

// newGraphQLInvokeTargetService serves the GraphQL API countries (invokeTargetAPI)
// of invokeTargetOrg on gateways, with deployments by gateway ID.
func newGraphQLInvokeTargetService(deployments map[string]string, gateways ...*model.APIGatewayWithDetails) *GraphQLAPIService {
	apiContext := "/countries/v1/graphql"
	stored := &model.GraphQLAPI{
		ID:             invokeTargetAPI,
		Handle:         "countries",
		Version:        "v1",
		OrganizationID: invokeTargetOrg,
		Configuration:  model.GraphQLAPIConfig{Context: &apiContext},
	}
	return &GraphQLAPIService{
		repo: &mockGraphQLAPIRepo{
			getByHandleFunc: func(handle, orgUUID string) (*model.GraphQLAPI, error) {
				if handle == stored.Handle && orgUUID == stored.OrganizationID {
					return stored, nil
				}
				return nil, nil
			},
			gatewayDetails: gateways,
		},
		deploymentRepo: &invokeTargetDeploymentRepo{content: deployments},
	}
}

func TestGetGraphQLAPIInvokeTargetReturnsTheDeployedContext(t *testing.T) {
	svc := newGraphQLInvokeTargetService(map[string]string{"gw-prod": deployedGraphQLArtifact("/countries/v0/graphql")},
		&model.APIGatewayWithDetails{ID: "gw-dev", Handle: "dev-gw", Endpoints: []string{"https://dev.example.com"}, IsDeployed: true},
		&model.APIGatewayWithDetails{ID: "gw-prod", Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: true},
	)

	target, err := svc.GetAPIInvokeTarget("countries", "prod-gw", invokeTargetOrg)

	require.NoError(t, err)
	assert.Equal(t, "/countries/v0/graphql", target.Context)
	assert.Empty(t, target.Vhost)
	assert.Equal(t, []string{"https://prod.example.com"}, target.Endpoints)
}

func TestGetGraphQLAPIInvokeTargetResolvesTheVersionInTheContext(t *testing.T) {
	svc := newGraphQLInvokeTargetService(map[string]string{"gw-prod": deployedGraphQLArtifact("/countries/$version/graphql")},
		&model.APIGatewayWithDetails{ID: "gw-prod", Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: true},
	)

	target, err := svc.GetAPIInvokeTarget("countries", "prod-gw", invokeTargetOrg)

	require.NoError(t, err)
	assert.Equal(t, "/countries/v1/graphql", target.Context)
}

func TestGetGraphQLAPIInvokeTargetRefusesAGatewayItIsNotDeployedTo(t *testing.T) {
	for name, svc := range map[string]*GraphQLAPIService{
		"associated only": newGraphQLInvokeTargetService(map[string]string{},
			&model.APIGatewayWithDetails{ID: "gw-prod", Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: false}),
		"no live deployment": newGraphQLInvokeTargetService(map[string]string{},
			&model.APIGatewayWithDetails{ID: "gw-prod", Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: true}),
		"an unknown gateway": newGraphQLInvokeTargetService(map[string]string{"gw-dev": deployedGraphQLArtifact("/countries")},
			&model.APIGatewayWithDetails{ID: "gw-dev", Handle: "dev-gw", Endpoints: []string{"https://dev.example.com"}, IsDeployed: true}),
	} {
		_, err := svc.GetAPIInvokeTarget("countries", "prod-gw", invokeTargetOrg)
		assert.True(t, apperror.GraphQLAPIInvokeGatewayNotDeployed.Is(err), "%s: got %v", name, err)
	}
}

func TestGetGraphQLAPIInvokeTargetOnlyFindsTheCallersOwnAPI(t *testing.T) {
	svc := newGraphQLInvokeTargetService(map[string]string{"gw-prod": deployedGraphQLArtifact("/countries")},
		&model.APIGatewayWithDetails{ID: "gw-prod", Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: true},
	)

	for _, tc := range []struct{ name, api, org string }{
		{"another organization", "countries", "org-uuid-2"},
		{"an unknown GraphQL API", "cities", invokeTargetOrg},
	} {
		_, err := svc.GetAPIInvokeTarget(tc.api, "prod-gw", tc.org)
		assert.True(t, apperror.GraphQLAPINotFound.Is(err), "%s: got %v", tc.name, err)
	}
}

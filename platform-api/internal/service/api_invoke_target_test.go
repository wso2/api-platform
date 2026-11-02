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
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

const (
	invokeTargetOrg = "org-uuid-1"
	invokeTargetAPI = "api-uuid-1"
)

// invokeTargetAPIRepo holds one REST API of invokeTargetOrg and its gateways.
type invokeTargetAPIRepo struct {
	repository.APIRepository
	api      *model.API
	gateways []*model.APIGatewayWithDetails
}

func (r *invokeTargetAPIRepo) GetAPIMetadataByHandle(handle, orgUUID string) (*model.APIMetadata, error) {
	if r.api == nil || handle != r.api.Handle || orgUUID != r.api.OrganizationID {
		return nil, nil
	}
	return &model.APIMetadata{ID: r.api.ID, Handle: r.api.Handle, OrganizationID: r.api.OrganizationID}, nil
}

func (r *invokeTargetAPIRepo) GetAPIByUUID(apiUUID, orgUUID string) (*model.API, error) {
	if r.api == nil || apiUUID != r.api.ID || orgUUID != r.api.OrganizationID {
		return nil, nil
	}
	return r.api, nil
}

func (r *invokeTargetAPIRepo) GetAPIGatewaysWithDetails(apiUUID, orgUUID string) ([]*model.APIGatewayWithDetails, error) {
	return r.gateways, nil
}

func newInvokeTargetService(gateways ...*model.APIGatewayWithDetails) *APIService {
	apiContext := "/pizza/v1"
	return &APIService{apiRepo: &invokeTargetAPIRepo{
		api: &model.API{
			ID:             invokeTargetAPI,
			Handle:         "pizza-api",
			OrganizationID: invokeTargetOrg,
			Configuration:  model.RestAPIConfig{Context: &apiContext},
		},
		gateways: gateways,
	}}
}

func TestGetAPIInvokeTargetReturnsTheDeployedGateway(t *testing.T) {
	svc := newInvokeTargetService(
		&model.APIGatewayWithDetails{Handle: "dev-gw", Endpoints: []string{"https://dev.example.com"}, IsDeployed: true},
		&model.APIGatewayWithDetails{Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: true},
	)

	apiContext, endpoints, err := svc.GetAPIInvokeTarget("pizza-api", "prod-gw", invokeTargetOrg)

	require.NoError(t, err)
	assert.Equal(t, "/pizza/v1", apiContext)
	assert.Equal(t, []string{"https://prod.example.com"}, endpoints)
}

func TestGetAPIInvokeTargetRefusesAGatewayTheAPIIsNotDeployedTo(t *testing.T) {
	// Associated but not deployed: the console never offers it, and accepting it
	// would let a caller aim a request at any gateway merely linked to the API.
	svc := newInvokeTargetService(
		&model.APIGatewayWithDetails{Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: false},
	)

	_, _, err := svc.GetAPIInvokeTarget("pizza-api", "prod-gw", invokeTargetOrg)

	assert.True(t, apperror.RESTAPIInvokeGatewayNotDeployed.Is(err), "got %v", err)
}

func TestGetAPIInvokeTargetRefusesAnUnknownGateway(t *testing.T) {
	svc := newInvokeTargetService(
		&model.APIGatewayWithDetails{Handle: "dev-gw", Endpoints: []string{"https://dev.example.com"}, IsDeployed: true},
	)

	_, _, err := svc.GetAPIInvokeTarget("pizza-api", "prod-gw", invokeTargetOrg)

	assert.True(t, apperror.RESTAPIInvokeGatewayNotDeployed.Is(err), "got %v", err)
}

func TestGetAPIInvokeTargetOnlyFindsTheCallersOwnAPI(t *testing.T) {
	svc := newInvokeTargetService(
		&model.APIGatewayWithDetails{Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: true},
	)

	for _, tc := range []struct{ name, api, org string }{
		{"another organization", "pizza-api", "org-uuid-2"},
		{"an unknown REST API", "burger-api", invokeTargetOrg},
	} {
		_, _, err := svc.GetAPIInvokeTarget(tc.api, "prod-gw", tc.org)
		assert.True(t, apperror.RESTAPINotFound.Is(err), "%s: got %v", tc.name, err)
	}
}

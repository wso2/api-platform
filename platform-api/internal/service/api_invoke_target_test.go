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

// invokeTargetDeploymentRepo holds the live deployment of invokeTargetAPI on each
// gateway, by gateway ID, as the artifact YAML the gateway runs.
type invokeTargetDeploymentRepo struct {
	repository.DeploymentRepository
	content map[string]string
}

func (r *invokeTargetDeploymentRepo) GetCurrentByGateway(artifactUUID, gatewayID, orgUUID string) (*model.Deployment, error) {
	c, ok := r.content[gatewayID]
	if !ok || artifactUUID != invokeTargetAPI || orgUUID != invokeTargetOrg {
		return nil, nil
	}
	return &model.Deployment{DeploymentID: "dep-" + gatewayID, Content: []byte(c)}, nil
}

// deployedArtifact is a REST API deployment as stored, with a main vhost when vhost is set.
func deployedArtifact(context, vhost string) string {
	a := "apiVersion: gateway.api-platform.wso2.com/v1alpha1\nkind: RestApi\nmetadata:\n  name: pizza-api\n" +
		"spec:\n  displayName: Pizza\n  version: v1\n  context: " + context + "\n"
	if vhost != "" {
		a += "  vhosts:\n    main: " + vhost + "\n"
	}
	return a
}

// newInvokeTargetService serves pizza-api, context /pizza/v1, on gateways; each
// deployed gateway runs that same definition.
func newInvokeTargetService(gateways ...*model.APIGatewayWithDetails) *APIService {
	deployments := map[string]string{}
	for _, gw := range gateways {
		if gw.IsDeployed {
			deployments[gw.ID] = deployedArtifact("/pizza/v1", "")
		}
	}
	return newInvokeTargetServiceWith(deployments, gateways...)
}

// newInvokeTargetServiceWith is newInvokeTargetService with the deployments given
// by gateway ID, for a gateway that runs something other than the current definition.
func newInvokeTargetServiceWith(deployments map[string]string, gateways ...*model.APIGatewayWithDetails) *APIService {
	apiContext := "/pizza/v1"
	return &APIService{
		apiRepo: &invokeTargetAPIRepo{
			api: &model.API{
				ID:             invokeTargetAPI,
				Handle:         "pizza-api",
				OrganizationID: invokeTargetOrg,
				Configuration:  model.RestAPIConfig{Context: &apiContext},
			},
			gateways: gateways,
		},
		deploymentRepo: &invokeTargetDeploymentRepo{content: deployments},
	}
}

func TestGetAPIInvokeTargetReturnsTheDeployedGateway(t *testing.T) {
	svc := newInvokeTargetService(
		&model.APIGatewayWithDetails{ID: "gw-dev", Handle: "dev-gw", Endpoints: []string{"https://dev.example.com"}, IsDeployed: true},
		&model.APIGatewayWithDetails{ID: "gw-prod", Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: true},
	)

	target, err := svc.GetAPIInvokeTarget("pizza-api", "prod-gw", invokeTargetOrg)

	require.NoError(t, err)
	assert.Equal(t, "/pizza/v1", target.Context)
	assert.Empty(t, target.Vhost)
	assert.Equal(t, []string{"https://prod.example.com"}, target.Endpoints)
}

// The gateway serves what was deployed: a context changed since, and not yet
// redeployed, is not where the API is.
func TestGetAPIInvokeTargetUsesTheDeployedContext(t *testing.T) {
	svc := newInvokeTargetServiceWith(map[string]string{"gw-prod": deployedArtifact("/pizza/v0", "")},
		&model.APIGatewayWithDetails{ID: "gw-prod", Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: true},
	)

	target, err := svc.GetAPIInvokeTarget("pizza-api", "prod-gw", invokeTargetOrg)

	require.NoError(t, err)
	assert.Equal(t, "/pizza/v0", target.Context)
}

// The gateway resolves $version in the context, so the relay must call the resolved path.
func TestGetAPIInvokeTargetResolvesTheVersionInTheContext(t *testing.T) {
	svc := newInvokeTargetServiceWith(map[string]string{"gw-prod": deployedArtifact("/pizza/$version", "")},
		&model.APIGatewayWithDetails{ID: "gw-prod", Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: true},
	)

	target, err := svc.GetAPIInvokeTarget("pizza-api", "prod-gw", invokeTargetOrg)

	require.NoError(t, err)
	assert.Equal(t, "/pizza/v1", target.Context)
}

func TestGetAPIInvokeTargetReturnsTheDeployedMainVhost(t *testing.T) {
	for name, tc := range map[string]struct{ vhost, want string }{
		"a vhost of its own":         {"pizza.example.com", "pizza.example.com"},
		"the gateway's default":      {"_gateway_default_", ""},
		"no vhost in the deployment": {"", ""},
	} {
		t.Run(name, func(t *testing.T) {
			svc := newInvokeTargetServiceWith(map[string]string{"gw-prod": deployedArtifact("/pizza/v1", tc.vhost)},
				&model.APIGatewayWithDetails{ID: "gw-prod", Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: true},
			)

			target, err := svc.GetAPIInvokeTarget("pizza-api", "prod-gw", invokeTargetOrg)

			require.NoError(t, err)
			assert.Equal(t, tc.want, target.Vhost)
		})
	}
}

func TestGetAPIInvokeTargetRefusesAGatewayWithNoLiveDeployment(t *testing.T) {
	svc := newInvokeTargetServiceWith(map[string]string{},
		&model.APIGatewayWithDetails{ID: "gw-prod", Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: true},
	)

	_, err := svc.GetAPIInvokeTarget("pizza-api", "prod-gw", invokeTargetOrg)

	assert.True(t, apperror.RESTAPIInvokeGatewayNotDeployed.Is(err), "got %v", err)
}

func TestGetAPIInvokeTargetRefusesAGatewayTheAPIIsNotDeployedTo(t *testing.T) {
	// Associated but not deployed: the console never offers it, and accepting it
	// would let a caller aim a request at any gateway merely linked to the API.
	svc := newInvokeTargetService(
		&model.APIGatewayWithDetails{ID: "gw-prod", Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: false},
	)

	_, err := svc.GetAPIInvokeTarget("pizza-api", "prod-gw", invokeTargetOrg)

	assert.True(t, apperror.RESTAPIInvokeGatewayNotDeployed.Is(err), "got %v", err)
}

func TestGetAPIInvokeTargetRefusesAnUnknownGateway(t *testing.T) {
	svc := newInvokeTargetService(
		&model.APIGatewayWithDetails{ID: "gw-dev", Handle: "dev-gw", Endpoints: []string{"https://dev.example.com"}, IsDeployed: true},
	)

	_, err := svc.GetAPIInvokeTarget("pizza-api", "prod-gw", invokeTargetOrg)

	assert.True(t, apperror.RESTAPIInvokeGatewayNotDeployed.Is(err), "got %v", err)
}

func TestGetAPIInvokeTargetOnlyFindsTheCallersOwnAPI(t *testing.T) {
	svc := newInvokeTargetService(
		&model.APIGatewayWithDetails{ID: "gw-prod", Handle: "prod-gw", Endpoints: []string{"https://prod.example.com"}, IsDeployed: true},
	)

	for _, tc := range []struct{ name, api, org string }{
		{"another organization", "pizza-api", "org-uuid-2"},
		{"an unknown REST API", "burger-api", invokeTargetOrg},
	} {
		_, err := svc.GetAPIInvokeTarget(tc.api, "prod-gw", tc.org)
		assert.True(t, apperror.RESTAPINotFound.Is(err), "%s: got %v", tc.name, err)
	}
}

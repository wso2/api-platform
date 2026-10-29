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
package mcpserver

import (
	"context"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/api"
)

func str1(s string) *string { return &s }

func (f *fake) createProject(_ context.Context, _ caller, req api.CreateProjectRequest) (*api.Project, error) {
	now := time.Date(2026, 10, 19, 1, 2, 3, 0, time.UTC)
	return &api.Project{Id: str1("p1"), DisplayName: req.DisplayName, Description: req.Description,
		OrganizationId: str1("acme"), CreatedBy: str1("u1"), UpdatedBy: str1("u1"), CreatedAt: &now, UpdatedAt: &now}, nil
}

func (f *fake) listGateways(context.Context, caller, page) ([]api.GatewayResponse, int, error) {
	g, _ := f.getGateway(context.Background(), caller{}, "gw1")
	return []api.GatewayResponse{*g}, 1, nil
}

func (f *fake) getGateway(context.Context, caller, string) (*api.GatewayResponse, error) {
	now := time.Date(2026, 10, 19, 1, 2, 3, 0, time.UTC)
	active, critical := true, false
	kind := api.GatewayResponseFunctionalityType("regular")
	return &api.GatewayResponse{Id: str1("gw1"), DisplayName: "GW", Description: str1("d"), Endpoints: &[]string{"https://gw"},
		FunctionalityType: &kind, IsActive: &active, IsCritical: &critical, OrganizationId: str1("acme"),
		Properties: &map[string]interface{}{"k": "v"}, CreatedBy: str1("u1"), UpdatedBy: str1("u1"),
		CreatedAt: &now, UpdatedAt: &now, Version: str1("1.2.0")}, nil
}

func (f *fake) deploy(_ context.Context, _ caller, _ *apiType, _ string, req api.DeployRequest) (*api.DeploymentResponse, error) {
	return &api.DeploymentResponse{DeploymentId: openapi_types.UUID(uuidOf("aaaaaaaa-0000-0000-0000-000000000009")), Name: req.Name,
		GatewayId: req.GatewayId, Status: api.DeploymentResponseStatusFAILED, StatusReason: str1("DEPLOYMENT_TIMEOUT")}, nil
}

// TestPlatformTools_ReturnTheRawObjectsThroughTheSDKOutputCheck calls each tool
// through a real client, so the SDK checks the raw types against its schema.
func TestPlatformTools_ReturnTheRawObjectsThroughTheSDKOutputCheck(t *testing.T) {
	s, _ := newTestServer(t)
	cs := connect(t, s, who(testOrg, "ap:project:manage", "ap:gateway:manage", "ap:rest_api:manage"))

	for _, tc := range []struct {
		tool string
		args map[string]any
		want string
	}{
		{"list_projects", nil, `"displayName":"Default"`},
		{"create_project", map[string]any{"name": "p1", "description": "d"}, `"organizationId":"acme"`},
		{"list_gateways", nil, `"functionalityType":"regular"`},
		{"get_gateway", map[string]any{"gatewayId": "gw1"}, `"isActive":true`},
		{"deploy_api", map[string]any{"apiType": "rest-api", "apiId": "a1", "gatewayId": "gw1"}, `"statusReason":"DEPLOYMENT_TIMEOUT"`},
	} {
		res, text := call(t, cs, tc.tool, tc.args)
		require.False(t, res.IsError, "%s: %s", tc.tool, text)
		assert.Contains(t, text, tc.want, tc.tool)
	}
}

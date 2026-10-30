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

	"github.com/wso2/api-platform/platform-api/api"
)

// object is an API as the REST API sends it: a JSON object.
type object = map[string]any

// backend is what the tools call to do their work: direct, which calls the
// services, or a test's fake. Each method stands for one REST route.
type backend interface {
	listAPIs(ctx context.Context, c caller, t *apiType, projectID string, p page) ([]object, int, error)
	getAPI(ctx context.Context, c caller, t *apiType, id string) (object, error)
	createAPI(ctx context.Context, c caller, t *apiType, payload object) (object, error)
	updateAPI(ctx context.Context, c caller, t *apiType, id string, whole object) (object, error)
	deleteAPI(ctx context.Context, c caller, t *apiType, id string) error
	listDeployments(ctx context.Context, c caller, t *apiType, id, gatewayID, status string, p page) ([]api.DeploymentResponse, int, error)
	deploy(ctx context.Context, c caller, t *apiType, id string, req api.DeployRequest) (*api.DeploymentResponse, error)
	undeploy(ctx context.Context, c caller, t *apiType, id, deploymentID, gatewayID string) (*api.DeploymentResponse, error)

	listProjects(ctx context.Context, c caller, p page) ([]api.Project, int, error)
	createProject(ctx context.Context, c caller, req api.CreateProjectRequest) (*api.Project, error)
	deleteProject(ctx context.Context, c caller, id string) error
	listGateways(ctx context.Context, c caller, p page) ([]api.GatewayResponse, int, error)
	getGateway(ctx context.Context, c caller, id string) (*api.GatewayResponse, error)
}

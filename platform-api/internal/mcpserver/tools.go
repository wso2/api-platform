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

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Risk levels. The console agent asks before a write, and always before a
// destructive call. "repeat" means calling twice gives the same result.
var (
	read              = &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}
	write             = &mcp.ToolAnnotations{DestructiveHint: ptr(false)}
	writeRepeat       = &mcp.ToolAnnotations{DestructiveHint: ptr(false), IdempotentHint: true}
	destructiveRepeat = &mcp.ToolAnnotations{DestructiveHint: ptr(true), IdempotentHint: true}
)

func ptr[T any](v T) *T { return &v }

// addTools is the full list: one add call per tool, with the REST routes it
// stands for. {c} is the API type's collection.
func (s *Server) addTools(srv *mcp.Server) {
	// API tools.
	add(s, srv, &mcp.Tool{
		Name: "list_apis", Title: "List APIs", Annotations: read,
		Description: "List the APIs of one type. Pass a projectId from list_projects, except for llm-provider, which belongs to the organization.",
	}, []string{"GET {c}"}, s.listAPIs)

	add(s, srv, &mcp.Tool{
		Name: "get_api", Title: "Get API", Annotations: read,
		Description: "Get one API, with its full configuration. Credentials are never returned. llm-provider and llm-proxy results leave out the OpenAPI document.",
	}, []string{"GET {c}/{id}"}, s.getAPI)

	add(s, srv, &mcp.Tool{
		Name: "get_api_schema", Title: "Get API Schema", Annotations: read,
		Description: "Show the JSON payload create_api takes for an API type: its JSON Schema, with the required fields listed first. update_api takes the same fields, none of them required. Read it before writing a payload.",
	}, nil, s.getAPISchema)

	add(s, srv, &mcp.Tool{
		Name: "list_api_deployments", Title: "List API Deployments", Annotations: read,
		Description: "Show where an API is deployed and the status of each deployment. Poll it after deploy_api: DEPLOYING becomes DEPLOYED, or FAILED.",
	}, []string{"GET {c}/{id}/deployments"}, s.listDeployments)

	add(s, srv, &mcp.Tool{
		Name: "create_api", Title: "Create API", Annotations: write,
		Description: "Create an API from a JSON payload, in the Platform API's format for that type. Call get_api_schema first for the format. A credential can be a {{ secret \"name\" }} reference to a secret stored in the console.",
	}, []string{"POST {c}"}, s.createAPI)

	add(s, srv, &mcp.Tool{
		Name: "update_api", Title: "Update API", Annotations: writeRepeat,
		Description: "Change some fields of an API. Fields left out stay as they are; an array replaces the old one, so read it with get_api first; null removes a field, but a rest-api keeps its description: send an empty string to clear it. To empty a list, send []. Running deployments do not change until the API is deployed again. The result shows what was saved; some fields can't change after create.",
	}, []string{"GET {c}/{id}", "PUT {c}/{id}"}, s.updateAPI)

	add(s, srv, &mcp.Tool{
		Name: "deploy_api", Title: "Deploy API", Annotations: writeRepeat,
		Description: "Deploy an API, as it is now, to a gateway from list_gateways. It finishes later: poll list_api_deployments.",
	}, []string{"POST {c}/{id}/deployments"}, s.deployAPI)

	add(s, srv, &mcp.Tool{
		Name: "undeploy_api", Title: "Undeploy API", Annotations: destructiveRepeat,
		Description: "Take an API off a gateway. The API itself stays; use delete_api to remove it.",
	}, []string{"GET {c}/{id}/deployments", "POST {c}/{id}/deployments/{d}/undeploy"}, s.undeployAPI)

	add(s, srv, &mcp.Tool{
		Name: "delete_api", Title: "Delete API", Annotations: destructiveRepeat,
		Description: "Delete an API. Every gateway stops serving it at once.",
	}, []string{"DELETE {c}/{id}"}, s.deleteAPI)

	// Platform tools.
	add(s, srv, &mcp.Tool{
		Name: "list_projects", Title: "List Projects", Annotations: read,
		Description: "List the projects in the organization. APIs live in projects.",
	}, []string{"GET /projects"}, s.listProjects)

	add(s, srv, &mcp.Tool{
		Name: "create_project", Title: "Create Project", Annotations: write,
		Description: "Create a project to hold APIs.",
	}, []string{"POST /projects"}, s.createProject)

	add(s, srv, &mcp.Tool{
		Name: "delete_project", Title: "Delete Project", Annotations: destructiveRepeat,
		Description: "Delete a project. The Platform API refuses while it still has APIs, or when it is the organization's last project.",
	}, []string{"DELETE /projects/{id}"}, s.deleteProject)

	add(s, srv, &mcp.Tool{
		Name: "list_gateways", Title: "List Gateways", Annotations: read,
		Description: "List the gateways APIs can be deployed to, with their functionality type and whether each is active.",
	}, []string{"GET /gateways"}, s.listGateways)

	add(s, srv, &mcp.Tool{
		Name: "get_gateway", Title: "Get Gateway", Annotations: read,
		Description: "Get one gateway, with its functionality type, whether it is active, and its endpoints.",
	}, []string{"GET /gateways/{id}"}, s.getGateway)
}

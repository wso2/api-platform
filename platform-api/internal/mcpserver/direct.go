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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/pagination"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// direct calls the same service methods the REST handlers call, and repeats
// the handler checks the services do not make.
type direct struct {
	projects *service.ProjectService
	gateways *service.GatewayService
	apis     map[string]directAPIs // by API type
}

// directAPIs is one API type's services. It is the only per-type code.
type directAPIs interface {
	list(org, projectID string, opts repository.ListOptions) ([]any, int, error)
	get(org, id string) (any, error)
	create(org, user string, payload []byte) (any, error)
	update(org, user, id string, whole []byte) (any, error)
	delete(org, user, id string) error
	deployments(org, id, gatewayID, status string) ([]api.DeploymentResponse, error)
	deploy(org, user, id string, req *api.DeployRequest) (*api.DeploymentResponse, error)
	undeploy(org, user, id, deploymentID, gatewayID string) (*api.DeploymentResponse, error)
}

// newDirect fails when an API type has no adapter, so no call has to check.
func newDirect(d Deps) (direct, error) {
	dir := direct{
		projects: d.Projects,
		gateways: d.Gateways,
		apis: map[string]directAPIs{
			"rest-api":     restAPIs{apis: d.APIs, deploys: d.Deployments},
			"llm-provider": llmProviders{providers: d.LLMProviders, deploys: d.LLMProviderDeployments},
			"llm-proxy":    llmProxies{proxies: d.LLMProxies, deploys: d.LLMProxyDeployments},
			"mcp-proxy":    mcpProxies{proxies: d.MCPProxies, deploys: d.MCPProxyDeployments},
			"agent-proxy":  agentProxies{proxies: d.AgentProxies, deploys: d.AgentProxyDeployments},
		},
	}
	for _, t := range apiTypes {
		if dir.apis[t.name] == nil {
			return direct{}, fmt.Errorf("mcp server: API type %s has no direct adapter", t.name)
		}
	}
	return dir, nil
}

// decode reads a payload into an API type's request struct.
func decode[T any](payload []byte, apiType string) (*T, error) {
	var req T
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fail("the payload is not a valid %s: %v", apiType, err)
	}
	return &req, nil
}

func listOptions(p page) repository.ListOptions {
	return repository.ListOptions{Limit: p.limit, Offset: p.offset, SortOrder: "desc"}
}

func (d direct) listAPIs(_ context.Context, c caller, t *apiType, projectID string, p page) ([]object, int, error) {
	items, total, err := d.apis[t.name].list(c.org, projectID, listOptions(p))
	if err != nil {
		return nil, 0, err
	}
	out := make([]object, 0, len(items))
	for _, it := range items {
		o, err := toObject(it, nil)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, o)
	}
	return out, total, nil
}

func (d direct) getAPI(_ context.Context, c caller, t *apiType, id string) (object, error) {
	return toObject(d.apis[t.name].get(c.org, id))
}

func (d direct) createAPI(_ context.Context, c caller, t *apiType, payload object) (object, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return toObject(d.apis[t.name].create(c.org, c.user, body))
}

func (d direct) updateAPI(_ context.Context, c caller, t *apiType, id string, whole object) (object, error) {
	body, err := json.Marshal(whole)
	if err != nil {
		return nil, err
	}
	return toObject(d.apis[t.name].update(c.org, c.user, id, body))
}

func (d direct) deleteAPI(_ context.Context, c caller, t *apiType, id string) error {
	return d.apis[t.name].delete(c.org, c.user, id)
}

func (d direct) listDeployments(_ context.Context, c caller, t *apiType, id, gatewayID, status string, p page) ([]api.DeploymentResponse, int, error) {
	all, err := d.apis[t.name].deployments(c.org, id, gatewayID, status)
	if err != nil {
		return nil, 0, err
	}
	return pagination.Window(all, p.limit, p.offset), len(all), nil
}

func (d direct) deploy(_ context.Context, c caller, t *apiType, id string, req api.DeployRequest) (*api.DeploymentResponse, error) {
	return d.apis[t.name].deploy(c.org, c.user, id, &req)
}

func (d direct) undeploy(_ context.Context, c caller, t *apiType, id, deploymentID, gatewayID string) (*api.DeploymentResponse, error) {
	return d.apis[t.name].undeploy(c.org, c.user, id, deploymentID, gatewayID)
}

func (d direct) listProjects(_ context.Context, c caller, p page) ([]api.Project, int, error) {
	return d.projects.GetProjectsByOrganization(c.org, listOptions(p))
}

func (d direct) createProject(_ context.Context, c caller, req api.CreateProjectRequest) (*api.Project, error) {
	return d.projects.CreateProject(&req, c.org, c.user)
}

func (d direct) deleteProject(_ context.Context, c caller, id string) error {
	return d.projects.DeleteProject(id, c.org, c.user)
}

func (d direct) listGateways(_ context.Context, c caller, p page) ([]api.GatewayResponse, int, error) {
	res, err := d.gateways.ListGateways(&c.org, listOptions(p))
	if err != nil {
		return nil, 0, err
	}
	return res.List, res.Pagination.Total, nil
}

func (d direct) getGateway(_ context.Context, c caller, id string) (*api.GatewayResponse, error) {
	return d.gateways.GetGateway(id, c.org)
}

// restAPIs is the rest-api type: the APIService and the DeploymentService.
type restAPIs struct {
	apis    *service.APIService
	deploys *service.DeploymentService
}

func (r restAPIs) list(org, projectID string, opts repository.ListOptions) ([]any, int, error) {
	items, total, err := r.apis.GetAPIsByOrganization(org, projectID, opts)
	return anyList(items), total, err
}

func (r restAPIs) get(org, id string) (any, error) { return r.apis.GetAPIByHandle(id, org) }

func (r restAPIs) create(org, user string, payload []byte) (any, error) {
	req, err := decode[api.CreateRESTAPIRequest](payload, "rest-api")
	if err != nil {
		return nil, err
	}
	if err := checkUpstream(req.Upstream); err != nil {
		return nil, err
	}
	created, _, err := r.apis.CreateAPI(req, org, user)
	return created, err
}

func (r restAPIs) update(org, user, id string, whole []byte) (any, error) {
	req, err := decode[api.RESTAPI](whole, "rest-api")
	if err != nil {
		return nil, err
	}
	if err := checkUpstream(req.Upstream); err != nil {
		return nil, err
	}
	return r.apis.UpdateAPIByHandle(id, req, org, user)
}

func (r restAPIs) delete(org, user, id string) error { return r.apis.DeleteAPIByHandle(id, org, user) }

func (r restAPIs) deployments(org, id, gatewayID, status string) ([]api.DeploymentResponse, error) {
	res, err := r.deploys.GetDeploymentsByHandle(id, gatewayID, status, org)
	if err != nil {
		return nil, err
	}
	return res.List, nil
}

func (r restAPIs) deploy(org, user, id string, req *api.DeployRequest) (*api.DeploymentResponse, error) {
	return r.deploys.DeployAPIByHandle(id, req, org, user)
}

func (r restAPIs) undeploy(org, user, id, deploymentID, gatewayID string) (*api.DeploymentResponse, error) {
	return r.deploys.UndeployDeploymentByHandle(id, deploymentID, gatewayID, org, user)
}

// llmProviders is the llm-provider type. A provider belongs to the
// organization, so it has no project.
type llmProviders struct {
	providers *service.LLMProviderService
	deploys   *service.LLMProviderDeploymentService
}

func (l llmProviders) list(org, _ string, opts repository.ListOptions) ([]any, int, error) {
	res, err := l.providers.List(org, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	return anyList(res.List), res.Pagination.Total, nil
}

func (l llmProviders) get(org, id string) (any, error) { return l.providers.Get(org, id) }

func (l llmProviders) create(org, user string, payload []byte) (any, error) {
	req, err := decode[api.LLMProvider](payload, "llm-provider")
	if err != nil {
		return nil, err
	}
	return l.providers.Create(org, user, req)
}

func (l llmProviders) update(org, user, id string, whole []byte) (any, error) {
	req, err := decode[api.LLMProvider](whole, "llm-provider")
	if err != nil {
		return nil, err
	}
	return l.providers.Update(org, id, user, req)
}

func (l llmProviders) delete(org, user, id string) error { return l.providers.Delete(org, id, user) }

func (l llmProviders) deployments(org, id, gatewayID, status string) ([]api.DeploymentResponse, error) {
	res, err := l.deploys.GetLLMProviderDeployments(id, org, optional(gatewayID), optional(status))
	if err != nil {
		return nil, err
	}
	return res.List, nil
}

func (l llmProviders) deploy(org, user, id string, req *api.DeployRequest) (*api.DeploymentResponse, error) {
	return l.deploys.DeployLLMProvider(id, req, org, user)
}

func (l llmProviders) undeploy(org, user, id, deploymentID, gatewayID string) (*api.DeploymentResponse, error) {
	return l.deploys.UndeployLLMProviderDeployment(id, deploymentID, gatewayID, org, user)
}

// llmProxies is the llm-proxy type.
type llmProxies struct {
	proxies *service.LLMProxyService
	deploys *service.LLMProxyDeploymentService
}

func (l llmProxies) list(org, projectID string, opts repository.ListOptions) ([]any, int, error) {
	res, err := l.proxies.List(org, &projectID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	return anyList(res.List), res.Pagination.Total, nil
}

func (l llmProxies) get(org, id string) (any, error) { return l.proxies.Get(org, id) }

func (l llmProxies) create(org, user string, payload []byte) (any, error) {
	req, err := decode[api.LLMProxy](payload, "llm-proxy")
	if err != nil {
		return nil, err
	}
	return l.proxies.Create(org, user, req)
}

func (l llmProxies) update(org, user, id string, whole []byte) (any, error) {
	req, err := decode[api.LLMProxy](whole, "llm-proxy")
	if err != nil {
		return nil, err
	}
	return l.proxies.Update(org, id, user, req)
}

func (l llmProxies) delete(org, user, id string) error { return l.proxies.Delete(org, id, user) }

func (l llmProxies) deployments(org, id, gatewayID, status string) ([]api.DeploymentResponse, error) {
	res, err := l.deploys.GetLLMProxyDeployments(id, org, optional(gatewayID), optional(status))
	if err != nil {
		return nil, err
	}
	return res.List, nil
}

func (l llmProxies) deploy(org, user, id string, req *api.DeployRequest) (*api.DeploymentResponse, error) {
	return l.deploys.DeployLLMProxy(id, req, org, user)
}

func (l llmProxies) undeploy(org, user, id, deploymentID, gatewayID string) (*api.DeploymentResponse, error) {
	return l.deploys.UndeployLLMProxyDeployment(id, deploymentID, gatewayID, org, user)
}

// mcpProxies is the mcp-proxy type.
type mcpProxies struct {
	proxies *service.MCPProxyService
	deploys *service.MCPDeploymentService
}

func (m mcpProxies) list(org, projectID string, opts repository.ListOptions) ([]any, int, error) {
	res, err := m.proxies.ListByProject(org, projectID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	return anyList(res.List), res.Pagination.Total, nil
}

func (m mcpProxies) get(org, id string) (any, error) { return m.proxies.Get(org, id) }

func (m mcpProxies) create(org, user string, payload []byte) (any, error) {
	req, err := decode[api.MCPProxy](payload, "mcp-proxy")
	if err != nil {
		return nil, err
	}
	return m.proxies.Create(org, user, req)
}

func (m mcpProxies) update(org, user, id string, whole []byte) (any, error) {
	req, err := decode[api.MCPProxy](whole, "mcp-proxy")
	if err != nil {
		return nil, err
	}
	return m.proxies.Update(org, id, user, req)
}

func (m mcpProxies) delete(org, user, id string) error { return m.proxies.Delete(org, id, user) }

func (m mcpProxies) deployments(org, id, gatewayID, status string) ([]api.DeploymentResponse, error) {
	res, err := m.deploys.GetDeploymentsByHandle(id, gatewayID, status, org)
	if err != nil {
		return nil, err
	}
	return res.List, nil
}

func (m mcpProxies) deploy(org, user, id string, req *api.DeployRequest) (*api.DeploymentResponse, error) {
	return m.deploys.DeployMCPProxyByHandle(id, req, org, user)
}

func (m mcpProxies) undeploy(org, user, id, deploymentID, gatewayID string) (*api.DeploymentResponse, error) {
	return m.deploys.UndeployDeploymentByHandle(id, deploymentID, gatewayID, org, user)
}

// agentProxies is the agent-proxy type. Its body is a union over protocols,
// so it goes through the same strict decoder as the REST handler.
type agentProxies struct {
	proxies *service.AgentProxyService
	deploys *service.AgentDeploymentService
}

func (a agentProxies) list(org, projectID string, opts repository.ListOptions) ([]any, int, error) {
	res, err := a.proxies.List(org, nil, optional(projectID), opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	return anyList(res.List), res.Pagination.Total, nil
}

func (a agentProxies) get(org, id string) (any, error) { return a.proxies.Get(org, id) }

func (a agentProxies) create(org, user string, payload []byte) (any, error) {
	req, err := dto.DecodeAgentProxyRequest(payload)
	if err != nil {
		return nil, fail("%v", err)
	}
	return a.proxies.Create(org, user, req)
}

func (a agentProxies) update(org, user, id string, whole []byte) (any, error) {
	req, err := dto.DecodeAgentProxyRequest(whole)
	if err != nil {
		return nil, fail("%v", err)
	}
	return a.proxies.Update(org, id, user, req)
}

func (a agentProxies) delete(org, user, id string) error { return a.proxies.Delete(org, id, user) }

func (a agentProxies) deployments(org, id, gatewayID, status string) ([]api.DeploymentResponse, error) {
	res, err := a.deploys.ListByHandle(id, gatewayID, status, org)
	if err != nil {
		return nil, err
	}
	return res.List, nil
}

func (a agentProxies) deploy(org, user, id string, req *api.DeployRequest) (*api.DeploymentResponse, error) {
	return a.deploys.DeployByHandle(id, req, org, user)
}

func (a agentProxies) undeploy(org, user, id, deploymentID, gatewayID string) (*api.DeploymentResponse, error) {
	return a.deploys.UndeployByHandle(id, deploymentID, gatewayID, org, user)
}

// anyList turns a typed list into the []any the adapters return.
func anyList[T any](items []T) []any {
	out := make([]any, len(items))
	for i := range items {
		out[i] = items[i]
	}
	return out
}

// optional is nil for "", as the services' optional filters expect.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// checkUpstream repeats the REST handler's upstream checks (handler/api.go).
func checkUpstream(u api.Upstream) error {
	set := func(d api.UpstreamDefinition) (hasURL, hasRef bool) {
		return d.Url != nil && strings.TrimSpace(*d.Url) != "", d.Ref != nil && strings.TrimSpace(*d.Ref) != ""
	}
	mainURL, mainRef := set(u.Main)
	var sbURL, sbRef bool
	if u.Sandbox != nil {
		sbURL, sbRef = set(*u.Sandbox)
	}
	switch {
	case !mainURL && !mainRef && !sbURL && !sbRef:
		return fail("At least one upstream endpoint (main or sandbox) is required")
	case mainURL && mainRef:
		return fail("The upstream main must specify either a url or a ref, not both.")
	case sbURL && sbRef:
		return fail("The upstream sandbox must specify either a url or a ref, not both.")
	}
	return nil
}

// toObject turns a service result into the JSON object the REST API would send.
func toObject(v any, err error) (object, error) {
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var o object
	return o, json.Unmarshal(b, &o)
}

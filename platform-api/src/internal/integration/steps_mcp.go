//go:build integration

/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package integration

import (
	"fmt"

	"github.com/cucumber/godog"

	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// registerMCPSteps wires the MCP control-plane (proxy) repository scenarios.
// Like the LLM steps these run purely at the store layer — no real MCP server
// is contacted and the upstream API key is a dummy string.
func registerMCPSteps(ctx *godog.ScenarioContext, w *world) {
	ctx.Step(`^I create an MCP proxy "([^"]*)" with upstream key "([^"]*)"$`, w.createMCPProxy)
	ctx.Step(`^reading the MCP proxy back returns upstream key "([^"]*)"$`, w.mcpProxyUpstreamKeyIs)
	ctx.Step(`^listing MCP proxies by organization returns (\d+)$`, w.listMCPProxiesByOrg)
	ctx.Step(`^listing MCP proxies by project returns (\d+)$`, w.listMCPProxiesByProject)
	ctx.Step(`^I update the MCP proxy description to "([^"]*)"$`, w.updateMCPProxyDescription)
	ctx.Step(`^reading the MCP proxy back shows description "([^"]*)"$`, w.mcpProxyDescriptionIs)
	ctx.Step(`^I delete the MCP proxy$`, w.deleteMCPProxy)
}

func (w *world) createMCPProxy(handle, upstreamKey string) error {
	repo := repository.NewMCPProxyRepo(w.it.db)
	projID := w.projID
	proxy := &model.MCPProxy{
		OrganizationUUID: w.orgID,
		Handle:           handle,
		Name:             "MCP " + handle,
		ProjectUUID:      &projID,
		Version:          "v1.0",
		CreatedBy:        "it-user",
		Configuration: model.MCPProxyConfiguration{
			Name:        "MCP " + handle,
			Version:     "v1.0",
			SpecVersion: "2025-06-18",
			// The upstream API key is a dummy string — the store never calls the
			// MCP server, so no real credential is required.
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL: "http://mock-mcp-server:8080/mcp",
					Auth: &model.UpstreamAuth{
						Type:   "api-key",
						Header: "Authorization",
						Value:  upstreamKey,
					},
				},
			},
		},
	}
	if err := repo.Create(proxy); err != nil {
		return fmt.Errorf("[%s] create MCP proxy failed: %w", w.it.driver, err)
	}
	w.mcpHandle = handle
	return nil
}

func (w *world) getMCPProxy() (*model.MCPProxy, error) {
	repo := repository.NewMCPProxyRepo(w.it.db)
	proxy, err := repo.GetByHandle(w.mcpHandle, w.orgID)
	if err != nil {
		return nil, fmt.Errorf("[%s] GetByHandle(%s) failed: %w", w.it.driver, w.mcpHandle, err)
	}
	if proxy == nil {
		return nil, fmt.Errorf("[%s] GetByHandle(%s): want the MCP proxy, got nil", w.it.driver, w.mcpHandle)
	}
	return proxy, nil
}

func (w *world) mcpProxyUpstreamKeyIs(want string) error {
	proxy, err := w.getMCPProxy()
	if err != nil {
		return err
	}
	if proxy.Configuration.Upstream.Main == nil || proxy.Configuration.Upstream.Main.Auth == nil {
		return fmt.Errorf("[%s] MCP proxy upstream auth did not round-trip: %+v", w.it.driver, proxy.Configuration.Upstream)
	}
	if got := proxy.Configuration.Upstream.Main.Auth.Value; got != want {
		return fmt.Errorf("[%s] MCP proxy upstream key: want %q, got %q", w.it.driver, want, got)
	}
	return nil
}

func (w *world) listMCPProxiesByOrg(want int) error {
	repo := repository.NewMCPProxyRepo(w.it.db)
	list, err := repo.List(w.orgID, 100, 0)
	if err != nil {
		return fmt.Errorf("[%s] List MCP proxies failed: %w", w.it.driver, err)
	}
	if len(list) != want {
		return fmt.Errorf("[%s] List MCP proxies: want %d, got %d", w.it.driver, want, len(list))
	}
	count, err := repo.Count(w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] Count MCP proxies failed: %w", w.it.driver, err)
	}
	if count != want {
		return fmt.Errorf("[%s] Count MCP proxies: want %d, got %d", w.it.driver, want, count)
	}
	return nil
}

func (w *world) listMCPProxiesByProject(want int) error {
	repo := repository.NewMCPProxyRepo(w.it.db)
	list, err := repo.ListByProject(w.orgID, w.projID)
	if err != nil {
		return fmt.Errorf("[%s] ListByProject MCP proxies failed: %w", w.it.driver, err)
	}
	if len(list) != want {
		return fmt.Errorf("[%s] ListByProject MCP proxies: want %d, got %d", w.it.driver, want, len(list))
	}
	count, err := repo.CountByProject(w.orgID, w.projID)
	if err != nil {
		return fmt.Errorf("[%s] CountByProject MCP proxies failed: %w", w.it.driver, err)
	}
	if count != want {
		return fmt.Errorf("[%s] CountByProject MCP proxies: want %d, got %d", w.it.driver, want, count)
	}
	return nil
}

func (w *world) updateMCPProxyDescription(desc string) error {
	repo := repository.NewMCPProxyRepo(w.it.db)
	proxy, err := w.getMCPProxy()
	if err != nil {
		return err
	}
	proxy.Description = desc
	proxy.UpdatedBy = "it-user"
	if err := repo.Update(proxy); err != nil {
		return fmt.Errorf("[%s] Update MCP proxy failed: %w", w.it.driver, err)
	}
	return nil
}

func (w *world) mcpProxyDescriptionIs(want string) error {
	proxy, err := w.getMCPProxy()
	if err != nil {
		return err
	}
	if proxy.Description != want {
		return fmt.Errorf("[%s] MCP proxy description: want %q, got %q", w.it.driver, want, proxy.Description)
	}
	return nil
}

func (w *world) deleteMCPProxy() error {
	repo := repository.NewMCPProxyRepo(w.it.db)
	if err := repo.Delete(w.mcpHandle, w.orgID); err != nil {
		return fmt.Errorf("[%s] Delete MCP proxy failed: %w", w.it.driver, err)
	}
	return nil
}

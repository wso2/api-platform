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
	"fmt"
	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
)

// MCPServerInfoFetcher fetches an MCP server's capabilities (tools/prompts/resources)
// from its upstream. It is satisfied by *MCPProxyService; the importer depends on the
// interface so the network fetch can be substituted in tests.
type MCPServerInfoFetcher interface {
	FetchServerInfo(orgUUID string, req *api.MCPServerInfoFetchRequest) (*api.MCPServerInfoFetchResponse, error)
}

// mcpProxyImporter imports MCP Proxy artifacts (project-scoped).
type mcpProxyImporter struct {
	mcpProxyRepo repository.MCPProxyRepository
	artifactRepo repository.ArtifactRepository
	serverInfo   MCPServerInfoFetcher
}

func newMCPProxyImporter(mcpProxyRepo repository.MCPProxyRepository, artifactRepo repository.ArtifactRepository,
	serverInfo MCPServerInfoFetcher) *mcpProxyImporter {
	return &mcpProxyImporter{mcpProxyRepo: mcpProxyRepo, artifactRepo: artifactRepo, serverInfo: serverInfo}
}

func (i *mcpProxyImporter) Kind() string          { return constants.MCPProxy }
func (i *mcpProxyImporter) RequiresProject() bool { return true }

func (i *mcpProxyImporter) Import(ctx *ImportContext) (*ImportResult, error) {
	version := utils.ImportVersion(ctx.Configuration)

	// The gateway pushes the artifact spec in the same shape the control plane emits when
	// generating a deployment (model.MCPProxyDeploymentSpec). Decode into that shape and
	// reverse-map it into the stored model.MCPProxyConfiguration — the inverse of
	// BuildMCPDeploymentYAML in utils/mcp.go.
	var spec model.MCPProxyDeploymentSpec
	if err := utils.DecodeSpec(ctx.Configuration.Spec, &spec); err != nil {
		return nil, err
	}
	cfg := mapMCPProxySpecToConfig(spec)

	if ctx.Existing == nil {
		// The gateway does not push the MCP server's capabilities; pull them from the
		// upstream so the AI Workspace can render tools/prompts/resources, mirroring the
		// mcp-proxies/fetch-server-info flow used at CP-native create time.
		cfg.Capabilities = i.fetchCapabilities(ctx.OrgID, cfg)
		projectID := ctx.ProjectID
		proxy := &model.MCPProxy{
			UUID:             ctx.ID,
			Handle:           utils.ImportHandle(ctx.Configuration),
			OrganizationUUID: ctx.OrgID,
			ProjectUUID:      &projectID,
			Name:             utils.ImportDisplayName(ctx.Configuration),
			Version:          version,
			Origin:           constants.OriginDP,
			Configuration:    cfg,
		}
		if err := i.mcpProxyRepo.Create(proxy); err != nil {
			return nil, fmt.Errorf("failed to create MCP proxy from gateway import: %w", err)
		}
		return &ImportResult{ID: proxy.UUID, DeployedVersion: version, Deployable: true}, nil
	}

	existing, err := i.mcpProxyRepo.GetByUUID(ctx.ID, ctx.OrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing MCP proxy: %w", err)
	}
	if existing == nil {
		return &ImportResult{ID: ctx.ID, DeployedVersion: version, Deployable: true}, nil
	}

	switch ctx.MetadataMode {
	case utils.SkipWorkingCopy:
		// Stale, out-of-order push: a newer deployment already defines the working copy.
		return &ImportResult{ID: ctx.ID, DeployedVersion: version, Deployable: true}, nil
	case utils.WriteFullMetadata:
		existing.Name = utils.ImportDisplayName(ctx.Configuration)
		existing.Version = version
		projectID := ctx.ProjectID
		existing.ProjectUUID = &projectID
		// Refresh capabilities from the upstream alongside the rest of the configuration.
		cfg.Capabilities = i.fetchCapabilities(ctx.OrgID, cfg)
		existing.Configuration = cfg
	case utils.WriteGatewaySpecificOnly:
		// CP-owned: No gateway specific metadata is written to the working copy
	}
	if err := i.mcpProxyRepo.Update(existing); err != nil {
		return nil, fmt.Errorf("failed to update MCP proxy from gateway import: %w", err)
	}
	return &ImportResult{ID: ctx.ID, DeployedVersion: version, Deployable: true}, nil
}

// mapMCPProxySpecToConfig reverse-maps a gateway-pushed MCP proxy deployment spec into
// the control plane's stored model.MCPProxyConfiguration. It is the inverse of
// BuildMCPDeploymentYAML: the single flat upstream ({url, auth}) is un-flattened into a
// main endpoint. Capabilities are not carried on the deployment spec (they are fetched
// out-of-band), so they are left unset, mirroring the forward mapping.
func mapMCPProxySpecToConfig(spec model.MCPProxyDeploymentSpec) model.MCPProxyConfiguration {
	cfg := model.MCPProxyConfiguration{
		Name:        spec.DisplayName,
		Version:     spec.Version,
		Vhost:       spec.Vhost,
		SpecVersion: spec.SpecVersion,
		Policies:    spec.Policies,
		Upstream:    mapMCPUpstreamToModel(spec.Upstream),
	}
	if spec.Context != "" {
		context := spec.Context
		cfg.Context = &context
	}
	return cfg
}

// mapMCPUpstreamToModel converts the gateway's single flat MCP upstream ({url, auth}) into
// the control plane's main/sandbox UpstreamConfig, mapping the single endpoint to the main
// endpoint (the gateway does not support a sandbox endpoint for MCP proxies).
func mapMCPUpstreamToModel(in model.MCPProxyUpstream) model.UpstreamConfig {
	if in.URL == "" && in.Auth == nil {
		return model.UpstreamConfig{}
	}
	return model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: in.URL, Auth: in.Auth}}
}

// fetchCapabilities pulls the MCP server's tools/prompts/resources from the proxy's
// upstream by reusing MCPProxyService.FetchServerInfo. The request carries the upstream
// URL and auth directly (no proxyId — the proxy is not yet stored at import time), which
// drives FetchServerInfo's initial-fetch branch (URL validation + reachability check +
// server-info fetch). It is best-effort: an unreachable or misbehaving server is logged
// and the import proceeds without capabilities rather than failing the whole push.
func (i *mcpProxyImporter) fetchCapabilities(orgID string, cfg model.MCPProxyConfiguration) *model.MCPProxyCapabilities {
	if cfg.Upstream.Main == nil || cfg.Upstream.Main.URL == "" {
		return nil
	}

	url := cfg.Upstream.Main.URL
	req := &api.MCPServerInfoFetchRequest{Url: &url}
	if cfg.Upstream.Main.Auth != nil {
		req.Auth = mapModelAuthToAPI(cfg.Upstream.Main.Auth)
	}

	info, err := i.serverInfo.FetchServerInfo(orgID, req)
	if err != nil {
		return nil
	}
	return &model.MCPProxyCapabilities{
		Prompts:   info.Prompts,
		Resources: info.Resources,
		Tools:     info.Tools,
	}
}

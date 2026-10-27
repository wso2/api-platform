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
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

const directTestOrg = "org-direct-it"

// realDirect is direct over the env services and a SQLite database, with one
// organization, its LLM templates and one project.
type realDirect struct {
	direct
	caller    caller
	projectID string
}

// newRealDirect wires the services as server.go does. Collaborators the
// create, get, update, delete and list paths never touch are left nil.
func newRealDirect(t *testing.T) realDirect {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := database.NewConnection(&config.Database{Driver: "sqlite3", Path: filepath.Join(t.TempDir(), "mcp.db")}, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.InitSchema("../database/schema.sqlite.sql", logger))
	_, err = db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, 'direct-it-org', 'Direct IT Org', 'default', 'idp-ref', datetime('now'), datetime('now'))`, directTestOrg)
	require.NoError(t, err)

	cfg := &config.Server{Deployments: config.Deployments{MaxPerAPIGateway: 20, MaxBuildsPerAPI: 5}}
	registry := repository.NewArtifactTableRegistry()
	orgRepo := repository.NewOrganizationRepo(db)
	projectRepo := repository.NewProjectRepo(db)
	apiRepo := repository.NewAPIRepo(db)
	appRepo := repository.NewApplicationRepo(db, registry)
	gatewayRepo := repository.NewGatewayRepo(db)
	customPolicyRepo := repository.NewCustomPolicyRepo(db)
	artifactRepo := repository.NewArtifactRepo(db, registry)
	deploymentRepo := repository.NewDeploymentRepo(db, registry)
	subscriptionPlanRepo := repository.NewSubscriptionPlanRepo(db)
	templateRepo := repository.NewLLMProviderTemplateRepo(db)
	providerRepo := repository.NewLLMProviderRepo(db)
	proxyRepo := repository.NewLLMProxyRepo(db)
	mcpProxyRepo := repository.NewMCPProxyRepo(db)
	agentProxyRepo := repository.NewAgentProxyRepo(db)
	apiKeyRepo := repository.NewAPIKeyRepo(db, registry)
	auditRepo := repository.NewAuditRepo(db)
	graphqlAPIRepo := repository.NewGraphQLAPIRepo(db, registry)

	templates, err := utils.LoadLLMProviderTemplatesFromDirectory("../../resources/default-llm-provider-templates")
	require.NoError(t, err)
	seeder := service.NewLLMTemplateSeeder(templateRepo, templates)
	require.NoError(t, seeder.SeedForOrg(directTestOrg))

	apiUtil := &utils.APIUtil{}
	identity := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	events := service.NewGatewayEventsService(nil, identity, logger)
	definitions := service.NewArtifactDefinitions(
		service.NewRestAPIDefinition(apiRepo, apiUtil),
		service.NewMCPProxyDefinition(mcpProxyRepo, &utils.MCPUtils{}),
		service.NewLLMProxyDefinition(proxyRepo, providerRepo, templateRepo),
		service.NewLLMProviderDefinition(providerRepo, templateRepo),
		service.NewAgentProxyDefinition(agentProxyRepo, &utils.AgentProxyUtils{}),
		service.NewGraphQLAPIDefinition(graphqlAPIRepo, projectRepo),
	)

	providers := service.NewLLMProviderService(providerRepo, templateRepo, orgRepo, seeder, deploymentRepo, gatewayRepo, events, logger, auditRepo, cfg, identity)
	providers.SetProxyRepository(proxyRepo)
	proxies := service.NewLLMProxyService(proxyRepo, providerRepo, projectRepo, deploymentRepo, gatewayRepo, events, logger, auditRepo, cfg, identity)
	proxies.SetTemplateRepository(templateRepo)

	dir, err := newDirect(Deps{
		Projects: service.NewProjectService(projectRepo, orgRepo, apiRepo, mcpProxyRepo, agentProxyRepo, appRepo, auditRepo, identity, logger),
		APIs: service.NewAPIService(apiRepo, projectRepo, orgRepo, gatewayRepo, deploymentRepo,
			subscriptionPlanRepo, customPolicyRepo, events, apiUtil, logger, auditRepo, identity),
		Deployments: service.NewDeploymentService(apiRepo, artifactRepo, deploymentRepo, gatewayRepo, orgRepo, apiKeyRepo, events, auditRepo, apiUtil, definitions, identity, cfg, logger),
		Gateways:    service.NewGatewayService(gatewayRepo, orgRepo, apiRepo, customPolicyRepo, events, logger, false, false, auditRepo, identity),

		LLMProviders:           providers,
		LLMProviderDeployments: service.NewLLMProviderDeploymentService(providerRepo, templateRepo, deploymentRepo, gatewayRepo, orgRepo, apiKeyRepo, events, artifactRepo, definitions, identity, cfg, logger),
		LLMProxies:             proxies,
		LLMProxyDeployments:    service.NewLLMProxyDeploymentService(proxyRepo, deploymentRepo, gatewayRepo, orgRepo, apiKeyRepo, events, artifactRepo, definitions, identity, cfg, logger),
		MCPProxies:             service.NewMCPProxyService(mcpProxyRepo, projectRepo, deploymentRepo, gatewayRepo, events, logger, auditRepo, cfg, identity),
		MCPProxyDeployments:    service.NewMCPDeploymentService(mcpProxyRepo, deploymentRepo, gatewayRepo, orgRepo, artifactRepo, apiKeyRepo, events, definitions, identity, cfg, logger),
		AgentProxies:           service.NewAgentProxyService(agentProxyRepo, projectRepo, deploymentRepo, gatewayRepo, events, logger, auditRepo, cfg, identity),
		AgentProxyDeployments:  service.NewAgentDeploymentService(agentProxyRepo, deploymentRepo, gatewayRepo, artifactRepo, apiKeyRepo, events, definitions, identity, cfg, logger),
	})
	require.NoError(t, err)

	env := realDirect{direct: dir, caller: caller{org: directTestOrg, user: "direct-it-user"}}
	project, err := env.createProject(context.Background(), env.caller, api.CreateProjectRequest{DisplayName: "Direct IT"})
	require.NoError(t, err)
	require.NotNil(t, project.Id)
	env.projectID = *project.Id
	return env
}

// idOf is the id the Platform API chose, which can differ from the name asked for.
func idOf(t *testing.T, created object) string {
	t.Helper()
	for _, key := range []string{"id", "handle"} {
		if id, ok := created[key].(string); ok && id != "" {
			return id
		}
	}
	require.Fail(t, "the created API has no id", "%v", created)
	return ""
}

// Each API type's payload as create_api takes it. An llm-proxy names the provider
// by id, so the provider type must be created first.
func directPayload(apiTypeName, projectID, providerID string) object {
	switch apiTypeName {
	case "rest-api":
		return object{"displayName": "Orders", "context": "/orders", "version": "v1", "projectId": projectID,
			"upstream": object{"main": object{"url": "https://orders.example.com"}}}
	case "llm-provider":
		return object{"displayName": "OpenAI Test", "version": "v1.0", "template": "openai",
			"upstream":      object{"main": object{"url": "https://api.openai.com/v1", "auth": object{"type": "api-key", "header": "Authorization", "value": "sk-test"}}},
			"accessControl": object{"mode": "allow_all"}}
	case "llm-proxy":
		return object{"displayName": "Chat Proxy", "version": "v1.0", "projectId": projectID, "provider": object{"id": providerID}}
	case "mcp-proxy":
		return object{"displayName": "Docs MCP", "version": "v1.0", "projectId": projectID,
			"upstream": object{"main": object{"url": "https://mcp.example.com/mcp"}}}
	default:
		return object{"displayName": "Helper Agent", "version": "v1.0", "projectId": projectID, "protocol": "a2a",
			"upstream": object{"main": object{"url": "https://agent.example.com"}},
			"a2a":      object{"protocolVersion": "1.0", "operationConfigs": object{"transports": []any{object{"protocolBinding": "JSONRPC"}}}}}
	}
}

// TestDirect_APILifecycle runs create, get, list, update, list deployments and
// delete for every API type, through the env services.
func TestDirect_APILifecycle(t *testing.T) {
	ctx := context.Background()
	env := newRealDirect(t)

	// The provider is created first and deleted last: the proxy uses it.
	provider, err := apiTypeNamed("llm-provider")
	require.NoError(t, err)
	createdProvider, err := env.createAPI(ctx, env.caller, provider, directPayload("llm-provider", env.projectID, ""))
	require.NoError(t, err)
	providerID := idOf(t, createdProvider)

	for _, name := range []string{"rest-api", "mcp-proxy", "agent-proxy", "llm-proxy", "llm-provider"} {
		t.Run(name, func(t *testing.T) {
			apiKind, err := apiTypeNamed(name)
			require.NoError(t, err)
			projectID := env.projectID
			if apiKind.project == projectNone {
				projectID = ""
			}

			id := providerID
			if name != "llm-provider" {
				created, err := env.createAPI(ctx, env.caller, apiKind, directPayload(name, env.projectID, providerID))
				require.NoError(t, err)
				id = idOf(t, created)
			}

			got, err := env.getAPI(ctx, env.caller, apiKind, id)
			require.NoError(t, err)
			assert.Equal(t, id, got["id"])

			listed, total, err := env.listAPIs(ctx, env.caller, apiKind, projectID, page{limit: 20})
			require.NoError(t, err)
			assert.GreaterOrEqual(t, total, 1)
			var found bool
			for _, item := range listed {
				found = found || item["id"] == id
			}
			assert.True(t, found, "list has %s: %v", id, listed)

			// update takes the whole object, as updateAPI's caller has merged it.
			got["description"] = "changed by MCP"
			updated, err := env.updateAPI(ctx, env.caller, apiKind, id, got)
			require.NoError(t, err)
			assert.Equal(t, "changed by MCP", updated["description"])

			deployments, total, err := env.listDeployments(ctx, env.caller, apiKind, id, "", "", page{limit: 20})
			require.NoError(t, err)
			assert.Empty(t, deployments)
			assert.Zero(t, total)

			if name == "llm-provider" {
				return // the proxy is deleted by now; the provider goes last
			}
			require.NoError(t, env.deleteAPI(ctx, env.caller, apiKind, id))
			_, err = env.getAPI(ctx, env.caller, apiKind, id)
			assert.Error(t, err, "a deleted API is gone")
		})
	}

	require.NoError(t, env.deleteAPI(ctx, env.caller, provider, providerID))
	_, err = env.getAPI(ctx, env.caller, provider, providerID)
	assert.Error(t, err)
}

func TestDirect_ProjectsAndGateways(t *testing.T) {
	ctx := context.Background()
	env := newRealDirect(t)

	spare, err := env.createProject(ctx, env.caller, api.CreateProjectRequest{DisplayName: "Spare"})
	require.NoError(t, err)
	require.NotNil(t, spare.Id)

	projects, total, err := env.listProjects(ctx, env.caller, page{limit: 20})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, projects, 2)

	require.NoError(t, env.deleteProject(ctx, env.caller, *spare.Id))
	_, total, err = env.listProjects(ctx, env.caller, page{limit: 20})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Error(t, env.deleteProject(ctx, env.caller, *spare.Id), "a second delete finds nothing")

	gateways, total, err := env.listGateways(ctx, env.caller, page{limit: 20})
	require.NoError(t, err)
	assert.Empty(t, gateways)
	assert.Zero(t, total)

	gateway, err := env.getGateway(ctx, env.caller, "no-such-gateway")
	assert.Nil(t, gateway)
	assert.Error(t, err)
}

func TestDirect_AdapterErrors(t *testing.T) {
	ctx := context.Background()
	env := newRealDirect(t)

	t.Run("rest-api without an upstream", func(t *testing.T) {
		restAPI, _ := apiTypeNamed("rest-api")
		_, err := env.createAPI(ctx, env.caller, restAPI, object{"displayName": "Orders", "context": "/orders", "version": "v1", "projectId": env.projectID})
		assert.ErrorContains(t, err, "At least one upstream endpoint")

		created, err := env.createAPI(ctx, env.caller, restAPI, directPayload("rest-api", env.projectID, ""))
		require.NoError(t, err)
		whole, err := env.getAPI(ctx, env.caller, restAPI, idOf(t, created))
		require.NoError(t, err)
		delete(whole, "upstream")
		_, err = env.updateAPI(ctx, env.caller, restAPI, idOf(t, created), whole)
		assert.ErrorContains(t, err, "At least one upstream endpoint", "update checks it too")
	})

	// A field of the wrong type fails in the decoder, before any service runs.
	for _, name := range []string{"rest-api", "llm-provider", "llm-proxy", "mcp-proxy"} {
		t.Run(name+" with a field of the wrong type", func(t *testing.T) {
			apiKind, _ := apiTypeNamed(name)
			bad := object{"displayName": 42}
			_, err := env.createAPI(ctx, env.caller, apiKind, bad)
			assert.ErrorContains(t, err, "is not a valid "+name)
			_, err = env.updateAPI(ctx, env.caller, apiKind, "anything", bad)
			assert.ErrorContains(t, err, "is not a valid "+name)
		})
	}

	t.Run("agent-proxy with a bad body", func(t *testing.T) {
		agent, _ := apiTypeNamed("agent-proxy")
		bad := object{"displayName": "Helper Agent", "protocol": "no-such-protocol"}
		_, err := env.createAPI(ctx, env.caller, agent, bad)
		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "is not a valid", "the strict decoder speaks, not decode")
		_, err = env.updateAPI(ctx, env.caller, agent, "anything", bad)
		assert.Error(t, err)
	})

	t.Run("an unknown id is not found", func(t *testing.T) {
		for _, apiKind := range apiTypes {
			_, err := env.getAPI(ctx, env.caller, &apiKind, "no-such-api")
			assert.Error(t, err, apiKind.name)
			assert.Error(t, env.deleteAPI(ctx, env.caller, &apiKind, "no-such-api"), apiKind.name)
			_, _, err = env.listDeployments(ctx, env.caller, &apiKind, "no-such-api", "", "", page{limit: 20})
			assert.Error(t, err, apiKind.name)
		}
	})
}

// TestDirect_DeployRefusesAnUnknownAPI reaches deploy and undeploy of every
// type without a gateway: the API is looked up first.
func TestDirect_DeployRefusesAnUnknownAPI(t *testing.T) {
	ctx := context.Background()
	env := newRealDirect(t)
	for _, apiKind := range apiTypes {
		_, err := env.deploy(ctx, env.caller, &apiKind, "no-such-api", api.DeployRequest{Base: "current", GatewayId: "no-such-gateway"})
		assert.Error(t, err, apiKind.name)
		_, err = env.undeploy(ctx, env.caller, &apiKind, "no-such-api", "aaaaaaaa-0000-0000-0000-000000000001", "no-such-gateway")
		assert.Error(t, err, apiKind.name)
	}
}

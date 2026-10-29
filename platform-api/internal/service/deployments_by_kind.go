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
	"context"
	"fmt"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// ArtifactDeployments is the deployment and build lifecycle of ONE artifact kind,
// under names that do not vary between kinds.
//
// The build half needs no adapting: every kind's service already exposes those four
// identically, because they all delegate to the shared build store. The deployment
// half does — the kinds spell the same operations differently (DeployAPIByHandle,
// DeployMCPProxyByHandle, DeployLLMProxy) and only REST threads an actor through —
// so the adapters below normalise that and nothing else.
type ArtifactDeployments interface {
	CreateBuildByHandle(handle, orgID, actor, description string,
		metadata map[string]interface{}) (*api.BuildResponse, error)
	GetBuildByHandle(handle, buildID, orgID string) (*api.BuildResponse, error)
	GetBuildsByHandle(handle, orgID string, limit int) (*api.BuildListResponse, error)
	DeleteBuildByHandle(handle, buildID, orgID string) error

	Deploy(ctx context.Context, handle string, req *api.DeployRequest, orgID, actor string) (*api.DeploymentResponse, error)
	Undeploy(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error)
	Restore(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error)
	GetDeployment(handle, deploymentID, orgID string) (*api.DeploymentResponse, error)
	ListDeployments(handle, gatewayID, status, orgID string) (*api.DeploymentListResponse, error)
}

// DeploymentsByKind routes an operation to the service for an artifact kind.
//
// The kind is always given by the caller rather than inferred from the handle:
// handles are unique only WITHIN a kind, so inferring one would let a request reach
// an artifact it did not name. Each kind's service validates that the artifact it
// resolves really is of its kind, so the check is made once, where the artifact is
// actually read.
type DeploymentsByKind map[string]ArtifactDeployments

// For returns the services for an artifact kind, or a validation error naming it.
// An unknown kind is the caller's mistake — it named something the platform does
// not deploy — so it is a bad request rather than an internal error.
func (d DeploymentsByKind) For(kind string) (ArtifactDeployments, error) {
	deployments, ok := d[kind]
	if !ok {
		return nil, apperror.ValidationFailed.New(
			fmt.Sprintf("%q is not an artifact kind that can be deployed.", kind))
	}
	return deployments, nil
}

// NewDeploymentsByKind indexes each kind's services under the kind the artifact row
// carries, which is the same key ArtifactDefinitions uses.
func NewDeploymentsByKind(
	rest *DeploymentService,
	mcp *MCPDeploymentService,
	llmProxy *LLMProxyDeploymentService,
	llmProvider *LLMProviderDeploymentService,
	agent *AgentDeploymentService,
	graphql *GraphQLAPIDeploymentService,
) DeploymentsByKind {
	return DeploymentsByKind{
		constants.RestApi:     restDeployments{rest},
		constants.MCPProxy:    mcpDeployments{mcp},
		constants.LLMProxy:    llmProxyDeployments{llmProxy},
		constants.LLMProvider: llmProviderDeployments{llmProvider},
		constants.AgentProxy:  agentDeployments{agent},
		constants.GraphQLApi:  graphqlDeployments{graphql},
	}
}

// restDeployments adapts the REST API service. Only Deploy is renamed; the rest
// already match, including the actor.
type restDeployments struct{ *DeploymentService }

func (a restDeployments) Deploy(ctx context.Context, handle string, req *api.DeployRequest, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.DeployAPIByHandle(ctx, handle, req, orgID, actor)
}

func (a restDeployments) Undeploy(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.UndeployDeploymentByHandle(ctx, handle, deploymentID, gatewayHandle, orgID, actor)
}

func (a restDeployments) Restore(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.RestoreDeploymentByHandle(ctx, handle, deploymentID, gatewayHandle, orgID, actor)
}

func (a restDeployments) GetDeployment(handle, deploymentID, orgID string) (*api.DeploymentResponse, error) {
	return a.GetDeploymentByHandle(handle, deploymentID, orgID)
}

func (a restDeployments) ListDeployments(handle, gatewayID, status, orgID string) (*api.DeploymentListResponse, error) {
	return a.GetDeploymentsByHandle(handle, gatewayID, status, orgID)
}

// mcpDeployments adapts the MCP proxy service, which records no actor of its own.
type mcpDeployments struct{ *MCPDeploymentService }

func (a mcpDeployments) Deploy(ctx context.Context, handle string, req *api.DeployRequest, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.DeployMCPProxyByHandle(ctx, handle, req, orgID, actor)
}

func (a mcpDeployments) Undeploy(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.UndeployDeploymentByHandle(ctx, handle, deploymentID, gatewayHandle, orgID, actor)
}

func (a mcpDeployments) Restore(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.RestoreMCPDeploymentByHandle(ctx, handle, deploymentID, gatewayHandle, orgID, actor)
}

func (a mcpDeployments) GetDeployment(handle, deploymentID, orgID string) (*api.DeploymentResponse, error) {
	return a.MCPDeploymentService.GetDeploymentByHandle(handle, deploymentID, orgID)
}

func (a mcpDeployments) ListDeployments(handle, gatewayID, status, orgID string) (*api.DeploymentListResponse, error) {
	return a.MCPDeploymentService.GetDeploymentsByHandle(handle, gatewayID, status, orgID)
}

// llmProxyDeployments adapts the LLM proxy service, whose identifier IS the handle
// and whose listing takes optional filters as pointers.
type llmProxyDeployments struct{ *LLMProxyDeploymentService }

func (a llmProxyDeployments) Deploy(ctx context.Context, handle string, req *api.DeployRequest, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.DeployLLMProxy(ctx, handle, req, orgID, actor)
}

func (a llmProxyDeployments) Undeploy(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.UndeployLLMProxyDeployment(ctx, handle, deploymentID, gatewayHandle, orgID, actor)
}

func (a llmProxyDeployments) Restore(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.RestoreLLMProxyDeployment(ctx, handle, deploymentID, gatewayHandle, orgID, actor)
}

func (a llmProxyDeployments) GetDeployment(handle, deploymentID, orgID string) (*api.DeploymentResponse, error) {
	return a.GetLLMProxyDeployment(handle, deploymentID, orgID)
}

func (a llmProxyDeployments) ListDeployments(handle, gatewayID, status, orgID string) (*api.DeploymentListResponse, error) {
	return a.GetLLMProxyDeployments(handle, orgID, optionalFilter(gatewayID), optionalFilter(status))
}

// llmProviderDeployments adapts the LLM provider service.
type llmProviderDeployments struct{ *LLMProviderDeploymentService }

func (a llmProviderDeployments) Deploy(ctx context.Context, handle string, req *api.DeployRequest, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.DeployLLMProvider(ctx, handle, req, orgID, actor)
}

func (a llmProviderDeployments) Undeploy(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.UndeployLLMProviderDeployment(ctx, handle, deploymentID, gatewayHandle, orgID, actor)
}

func (a llmProviderDeployments) Restore(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.RestoreLLMProviderDeployment(ctx, handle, deploymentID, gatewayHandle, orgID, actor)
}

func (a llmProviderDeployments) GetDeployment(handle, deploymentID, orgID string) (*api.DeploymentResponse, error) {
	return a.GetLLMProviderDeployment(handle, deploymentID, orgID)
}

func (a llmProviderDeployments) ListDeployments(handle, gatewayID, status, orgID string) (*api.DeploymentListResponse, error) {
	return a.GetLLMProviderDeployments(handle, orgID, optionalFilter(gatewayID), optionalFilter(status))
}

// agentDeployments adapts the Agent proxy service, which records no actor on
// undeploy or restore.
type agentDeployments struct{ *AgentDeploymentService }

func (a agentDeployments) Deploy(ctx context.Context, handle string, req *api.DeployRequest, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.DeployByHandle(ctx, handle, req, orgID, actor)
}

func (a agentDeployments) Undeploy(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, _ string) (*api.DeploymentResponse, error) {
	return a.UndeployByHandle(ctx, handle, deploymentID, gatewayHandle, orgID)
}

func (a agentDeployments) Restore(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, _ string) (*api.DeploymentResponse, error) {
	return a.RestoreByHandle(ctx, handle, deploymentID, gatewayHandle, orgID)
}

func (a agentDeployments) GetDeployment(handle, deploymentID, orgID string) (*api.DeploymentResponse, error) {
	return a.GetByHandle(handle, deploymentID, orgID)
}

func (a agentDeployments) ListDeployments(handle, gatewayID, status, orgID string) (*api.DeploymentListResponse, error) {
	return a.ListByHandle(handle, gatewayID, status, orgID)
}

// graphqlDeployments adapts the GraphQL API service, whose identifier IS the
// handle and whose listing takes optional filters as pointers, same as LLM.
type graphqlDeployments struct{ *GraphQLAPIDeploymentService }

func (a graphqlDeployments) Deploy(ctx context.Context, handle string, req *api.DeployRequest, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.DeployGraphQLAPI(ctx, handle, req, orgID, actor)
}

func (a graphqlDeployments) Undeploy(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, _ string) (*api.DeploymentResponse, error) {
	return a.UndeployGraphQLAPIDeployment(ctx, handle, deploymentID, gatewayHandle, orgID)
}

func (a graphqlDeployments) Restore(ctx context.Context, handle, deploymentID, gatewayHandle, orgID, _ string) (*api.DeploymentResponse, error) {
	return a.RestoreGraphQLAPIDeployment(ctx, handle, deploymentID, gatewayHandle, orgID)
}

func (a graphqlDeployments) GetDeployment(handle, deploymentID, orgID string) (*api.DeploymentResponse, error) {
	return a.GraphQLAPIDeploymentService.GetGraphQLAPIDeployment(handle, deploymentID, orgID)
}

func (a graphqlDeployments) ListDeployments(handle, gatewayID, status, orgID string) (*api.DeploymentListResponse, error) {
	return a.GraphQLAPIDeploymentService.GetGraphQLAPIDeployments(handle, orgID, optionalFilter(gatewayID), optionalFilter(status))
}

// optionalFilter turns an empty filter into "not given", which is how the LLM
// services spell an absent gateway or status.
func optionalFilter(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// The methods below give DeploymentsByKind the shape plugins use: the same
// operations, each taking the artifact kind alongside the handle. They resolve the
// kind once and delegate, so the routing lives here rather than in every plugin.

// CreateBuildByHandle prepares a build of an artifact of the named kind.
func (d DeploymentsByKind) CreateBuildByHandle(handle, kind, orgID, actor, description string,
	metadata map[string]interface{}) (*api.BuildResponse, error) {
	deployments, err := d.For(kind)
	if err != nil {
		return nil, err
	}
	return deployments.CreateBuildByHandle(handle, orgID, actor, description, metadata)
}

// GetBuildByHandle returns one of an artifact's builds.
func (d DeploymentsByKind) GetBuildByHandle(handle, kind, buildID, orgID string) (*api.BuildResponse, error) {
	deployments, err := d.For(kind)
	if err != nil {
		return nil, err
	}
	return deployments.GetBuildByHandle(handle, buildID, orgID)
}

// GetBuildsByHandle lists an artifact's builds, newest first.
func (d DeploymentsByKind) GetBuildsByHandle(handle, kind, orgID string, limit int) (*api.BuildListResponse, error) {
	deployments, err := d.For(kind)
	if err != nil {
		return nil, err
	}
	return deployments.GetBuildsByHandle(handle, orgID, limit)
}

// DeleteBuildByHandle removes one of an artifact's builds.
func (d DeploymentsByKind) DeleteBuildByHandle(handle, kind, buildID, orgID string) error {
	deployments, err := d.For(kind)
	if err != nil {
		return err
	}
	return deployments.DeleteBuildByHandle(handle, buildID, orgID)
}

// DeployByHandle deploys an artifact of the named kind onto one gateway.
func (d DeploymentsByKind) DeployByHandle(handle, kind string, req *api.DeployRequest,
	orgID, actor string) (*api.DeploymentResponse, error) {
	deployments, err := d.For(kind)
	if err != nil {
		return nil, err
	}
	return deployments.Deploy(context.Background(), handle, req, orgID, actor)
}

// GetDeploymentsByHandle lists an artifact's deployments.
func (d DeploymentsByKind) GetDeploymentsByHandle(handle, kind, gatewayID, status, orgID string) (*api.DeploymentListResponse, error) {
	deployments, err := d.For(kind)
	if err != nil {
		return nil, err
	}
	return deployments.ListDeployments(handle, gatewayID, status, orgID)
}

// GetDeploymentByHandle returns a single deployment of an artifact.
func (d DeploymentsByKind) GetDeploymentByHandle(handle, kind, deploymentID, orgID string) (*api.DeploymentResponse, error) {
	deployments, err := d.For(kind)
	if err != nil {
		return nil, err
	}
	return deployments.GetDeployment(handle, deploymentID, orgID)
}

// UndeployDeploymentByHandle takes a deployment off its gateway.
func (d DeploymentsByKind) UndeployDeploymentByHandle(handle, kind, deploymentID, gatewayHandle,
	orgID, actor string) (*api.DeploymentResponse, error) {
	deployments, err := d.For(kind)
	if err != nil {
		return nil, err
	}
	return deployments.Undeploy(context.Background(), handle, deploymentID, gatewayHandle, orgID, actor)
}

// RestoreDeploymentByHandle puts a suspended or superseded deployment back on its
// gateway, serving the artifact it already holds.
func (d DeploymentsByKind) RestoreDeploymentByHandle(handle, kind, deploymentID, gatewayHandle,
	orgID, actor string) (*api.DeploymentResponse, error) {
	deployments, err := d.For(kind)
	if err != nil {
		return nil, err
	}
	return deployments.Restore(context.Background(), handle, deploymentID, gatewayHandle, orgID, actor)
}

// ArtifactProjects resolves the project an artifact belongs to. Satisfied by
// repository.ArtifactRepo.
type ArtifactProjects interface {
	GetProjectHandle(kind, handle, orgUUID string) (projectHandle string, found bool, err error)
}

// PluginDeployments is the deployments capability handed to plugins: every
// kind-routed operation, plus the project an artifact belongs to, which a plugin
// that scopes deployments by project checks before acting on an artifact.
type PluginDeployments struct {
	DeploymentsByKind
	Projects ArtifactProjects
}

// GetArtifactProjectByHandle returns the handle of the project an artifact of the
// named kind belongs to, or an empty handle when it belongs to none.
func (p PluginDeployments) GetArtifactProjectByHandle(handle, kind, orgID string) (string, error) {
	if _, err := p.For(kind); err != nil {
		return "", err
	}
	project, found, err := p.Projects.GetProjectHandle(kind, handle, orgID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", apperror.ArtifactNotFound.New()
	}
	return project, nil
}

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

	Deploy(handle string, req *api.DeployRequest, orgID, actor string) (*api.DeploymentResponse, error)
	Undeploy(handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error)
	Restore(handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error)
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
) DeploymentsByKind {
	return DeploymentsByKind{
		constants.RestApi:     restDeployments{rest},
		constants.MCPProxy:    mcpDeployments{mcp},
		constants.LLMProxy:    llmProxyDeployments{llmProxy},
		constants.LLMProvider: llmProviderDeployments{llmProvider},
	}
}

// restDeployments adapts the REST API service. Only Deploy is renamed; the rest
// already match, including the actor.
type restDeployments struct{ *DeploymentService }

func (a restDeployments) Deploy(handle string, req *api.DeployRequest, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.DeployAPIByHandle(handle, req, orgID, actor)
}

func (a restDeployments) Undeploy(handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.UndeployDeploymentByHandle(handle, deploymentID, gatewayHandle, orgID, actor)
}

func (a restDeployments) Restore(handle, deploymentID, gatewayHandle, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.RestoreDeploymentByHandle(handle, deploymentID, gatewayHandle, orgID, actor)
}

func (a restDeployments) GetDeployment(handle, deploymentID, orgID string) (*api.DeploymentResponse, error) {
	return a.GetDeploymentByHandle(handle, deploymentID, orgID)
}

func (a restDeployments) ListDeployments(handle, gatewayID, status, orgID string) (*api.DeploymentListResponse, error) {
	return a.GetDeploymentsByHandle(handle, gatewayID, status, orgID)
}

// mcpDeployments adapts the MCP proxy service, which records no actor of its own.
type mcpDeployments struct{ *MCPDeploymentService }

func (a mcpDeployments) Deploy(handle string, req *api.DeployRequest, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.DeployMCPProxyByHandle(handle, req, orgID, actor)
}

func (a mcpDeployments) Undeploy(handle, deploymentID, gatewayHandle, orgID, _ string) (*api.DeploymentResponse, error) {
	return a.UndeployDeploymentByHandle(handle, deploymentID, gatewayHandle, orgID)
}

func (a mcpDeployments) Restore(handle, deploymentID, gatewayHandle, orgID, _ string) (*api.DeploymentResponse, error) {
	return a.RestoreMCPDeploymentByHandle(handle, deploymentID, gatewayHandle, orgID)
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

func (a llmProxyDeployments) Deploy(handle string, req *api.DeployRequest, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.DeployLLMProxy(handle, req, orgID, actor)
}

func (a llmProxyDeployments) Undeploy(handle, deploymentID, gatewayHandle, orgID, _ string) (*api.DeploymentResponse, error) {
	return a.UndeployLLMProxyDeployment(handle, deploymentID, gatewayHandle, orgID)
}

func (a llmProxyDeployments) Restore(handle, deploymentID, gatewayHandle, orgID, _ string) (*api.DeploymentResponse, error) {
	return a.RestoreLLMProxyDeployment(handle, deploymentID, gatewayHandle, orgID)
}

func (a llmProxyDeployments) GetDeployment(handle, deploymentID, orgID string) (*api.DeploymentResponse, error) {
	return a.GetLLMProxyDeployment(handle, deploymentID, orgID)
}

func (a llmProxyDeployments) ListDeployments(handle, gatewayID, status, orgID string) (*api.DeploymentListResponse, error) {
	return a.GetLLMProxyDeployments(handle, orgID, optionalFilter(gatewayID), optionalFilter(status))
}

// llmProviderDeployments adapts the LLM provider service.
type llmProviderDeployments struct{ *LLMProviderDeploymentService }

func (a llmProviderDeployments) Deploy(handle string, req *api.DeployRequest, orgID, actor string) (*api.DeploymentResponse, error) {
	return a.DeployLLMProvider(handle, req, orgID, actor)
}

func (a llmProviderDeployments) Undeploy(handle, deploymentID, gatewayHandle, orgID, _ string) (*api.DeploymentResponse, error) {
	return a.UndeployLLMProviderDeployment(handle, deploymentID, gatewayHandle, orgID)
}

func (a llmProviderDeployments) Restore(handle, deploymentID, gatewayHandle, orgID, _ string) (*api.DeploymentResponse, error) {
	return a.RestoreLLMProviderDeployment(handle, deploymentID, gatewayHandle, orgID)
}

func (a llmProviderDeployments) GetDeployment(handle, deploymentID, orgID string) (*api.DeploymentResponse, error) {
	return a.GetLLMProviderDeployment(handle, deploymentID, orgID)
}

func (a llmProviderDeployments) ListDeployments(handle, gatewayID, status, orgID string) (*api.DeploymentListResponse, error) {
	return a.GetLLMProviderDeployments(handle, orgID, optionalFilter(gatewayID), optionalFilter(status))
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
	return deployments.Deploy(handle, req, orgID, actor)
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
	return deployments.Undeploy(handle, deploymentID, gatewayHandle, orgID, actor)
}

// RestoreDeploymentByHandle puts a suspended or superseded deployment back on its
// gateway, serving the artifact it already holds.
func (d DeploymentsByKind) RestoreDeploymentByHandle(handle, kind, deploymentID, gatewayHandle,
	orgID, actor string) (*api.DeploymentResponse, error) {
	deployments, err := d.For(kind)
	if err != nil {
		return nil, err
	}
	return deployments.Restore(handle, deploymentID, gatewayHandle, orgID, actor)
}

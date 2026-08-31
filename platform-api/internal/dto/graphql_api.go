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

package dto

import "github.com/wso2/api-platform/platform-api/internal/model"

// GraphQLAPIDeploymentYAML represents the GraphQL API deployment YAML structure
// pushed to the gateway-controller. Mirrors APIDeploymentYAML (api.go) in shape,
// substituting GraphQLAPIYAMLData for the spec section.
type GraphQLAPIDeploymentYAML struct {
	ApiVersion string             `yaml:"apiVersion" binding:"required"`
	Kind       string             `yaml:"kind" binding:"required"`
	Metadata   DeploymentMetadata `yaml:"metadata" binding:"required"`
	Spec       GraphQLAPIYAMLData `yaml:"spec" binding:"required"`
}

// GetApiVersion returns the artifact's CRD apiVersion.
func (d *GraphQLAPIDeploymentYAML) GetApiVersion() string { return d.ApiVersion }

// SetApiVersion sets the artifact's CRD apiVersion.
func (d *GraphQLAPIDeploymentYAML) SetApiVersion(v string) { d.ApiVersion = v }

// GraphQLAPIYAMLData represents the spec section of the GraphQL API deployment
// YAML. Deliberately absent compared to APIYAMLData: Operations/Channels — a
// GraphQL API has exactly one logical endpoint, not a per-resource/per-verb
// operation list. The schema itself is never sent to the gateway: it plays no
// role in routing (GraphQLAPITransformer always builds exactly one POST route,
// regardless of what queries/mutations exist), so platform-api keeps SDL as a
// CP-side onboarding/documentation concern (model.GraphQLAPIConfig.SDL, used by
// dev-portal's schema viewer) and never forwards it into this deployment shape.
type GraphQLAPIYAMLData struct {
	DisplayName       string           `yaml:"displayName"`
	Version           string           `yaml:"version"`
	Context           string           `yaml:"context"`
	SubscriptionPlans []string         `yaml:"subscriptionPlans,omitempty"`
	Upstream          *GraphQLUpstream `yaml:"upstream,omitempty"`
	Policies          []Policy         `yaml:"policies,omitempty"`
}

// GraphQLUpstream represents the upstream configuration for the GraphQL API
// deployment YAML. Unlike RestAPI's per-operation upstream shape, a GraphQL
// API has exactly one logical endpoint per environment, but — like REST — it
// still supports an optional sandbox split alongside the main upstream (see
// GraphQLAPIConfigData.Upstream.Sandbox in the gateway's OpenAPI spec and
// GraphQLAPITransformer's sandbox route handling).
type GraphQLUpstream struct {
	Main    *GraphQLUpstreamTarget `yaml:"main,omitempty"`
	Sandbox *GraphQLUpstreamTarget `yaml:"sandbox,omitempty"`
}

// GraphQLUpstreamTarget represents the GraphQL upstream endpoint (url or ref),
// including auth. Unlike REST's UpstreamTarget (which has no Auth field, so
// upstream credentials are silently dropped from the deployment YAML), this
// type carries Auth from day one, matching MCP Proxy's
// BuildMCPDeploymentYAML (internal/utils/mcp.go). Auth is the raw model type
// (not the redacted api.UpstreamAuth used in read responses) because this YAML
// is what the gateway actually uses to authenticate to the upstream.
type GraphQLUpstreamTarget struct {
	URL  string              `yaml:"url,omitempty"`
	Ref  string              `yaml:"ref,omitempty"`
	Auth *model.UpstreamAuth `yaml:"auth,omitempty"`
}

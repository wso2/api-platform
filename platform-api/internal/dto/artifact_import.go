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

import "time"

// ImportGatewayArtifactRequest is the generic request body used by a data-plane
// gateway to push (create/update) an artifact to the control plane. The flow is
// kind-agnostic; the artifact type is identified by Configuration.Kind and routed
// to the matching importer in the control plane's artifact-type registry.
type ImportGatewayArtifactRequest struct {
	// DPID is the data-plane (gateway) artifact UUID. The control plane does NOT reuse it
	// as the CP artifact UUID — it mints its own and returns it in the response. It is the
	// key under which this artifact's result is returned in the bulk response; artifacts are
	// matched in the CP by handle.
	DPID string `json:"dpid" binding:"required"`
	// Configuration is the artifact descriptor (apiVersion/kind/metadata/spec).
	Configuration ArtifactImportConfig `json:"configuration" binding:"required"`
	// Status is the artifact's status on the gateway: deployed|pending|failed|undeployed.
	Status     string     `json:"status" binding:"required"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	DeployedAt *time.Time `json:"deployedAt,omitempty"`
	// Properties is an open-ended bag for future extensions.
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// ArtifactImportConfig is the gateway artifact custom resource (CR) exactly as it is
// deployed to the gateway: a k8s-shaped descriptor with apiVersion/kind/metadata/spec.
// The concrete shape of Spec is determined by Kind (RestApi, LlmProvider,
// LlmProviderTemplate, LlmProxy, Mcp, ... — extensible to future kinds); the matching
// importer in the registry interprets it. It is kept generic at this layer so the
// endpoint stays decoupled from each kind's spec schema.
type ArtifactImportConfig struct {
	APIVersion string                 `json:"apiVersion" yaml:"apiVersion"`
	Kind       string                 `json:"kind" yaml:"kind" binding:"required"`
	Metadata   ArtifactImportMetadata `json:"metadata" yaml:"metadata" binding:"required"`
	Spec       map[string]interface{} `json:"spec" yaml:"spec"`
}

// ArtifactImportMetadata carries the artifact's identity in a k8s-shaped form
// (name + labels/annotations). The project is conveyed via labels/annotations
// (the "project-id" label or the "gateway.api-platform.wso2.com/project-id"
// annotation) so the descriptor maps cleanly onto a Kubernetes custom resource.
// The project is required for project-scoped kinds (REST API, LLM Proxy, MCP Proxy)
// and ignored for organization-level kinds (LLM Provider, LLM Provider Template).
type ArtifactImportMetadata struct {
	Name        string            `json:"name" yaml:"name" binding:"required"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

// ImportGatewayArtifactsResponse is the response to a bulk gateway-artifact import. Artifacts
// maps each pushed artifact's data-plane UUID (dpid) to its per-artifact result; Total/Success/
// Failed are the aggregate counts. A failed artifact's result carries a non-empty Error.
type ImportGatewayArtifactsResponse struct {
	Total     int                                      `json:"total"`
	Success   int                                      `json:"success"`
	Failed    int                                      `json:"failed"`
	Artifacts map[string]ImportGatewayArtifactResponse `json:"artifacts"`
}

// ImportGatewayArtifactResponse is the generic per-artifact result for an imported artifact.
type ImportGatewayArtifactResponse struct {
	// ID is the control-plane artifact UUID (minted by the CP, reused on subsequent
	// pushes matched by handle). The gateway stores this as its cp_artifact_id.
	ID              string     `json:"id,omitempty"`
	Status          string     `json:"status"`
	Origin          string     `json:"origin"` // always "gateway_api" for imported artifacts
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	DeployedAt      *time.Time `json:"deployedAt,omitempty"`
	DeployedVersion string     `json:"deployedVersion,omitempty"`
	// Error is the failure reason when this artifact could not be imported; empty on success.
	Error string `json:"error,omitempty"`
}

/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package admin

import "time"

// ConfigDumpResponse is the top-level response structure for the config_dump endpoint
type ConfigDumpResponse struct {
	Timestamp      time.Time          `json:"timestamp"`
	PolicyRegistry PolicyRegistryDump `json:"policy_registry"`
	PolicyChains   PolicyChainsDump   `json:"policy_chains"`
	RouteMetadata  RouteMetadataDump  `json:"route_metadata"`
	LazyResources  LazyResourcesDump  `json:"lazy_resources"`
	XDSSync        XDSSyncInfo        `json:"xds_sync"`
}

// XDSSyncInfo contains policy xDS sync version details.
type XDSSyncInfo struct {
	PolicyChainVersion string `json:"policy_chain_version"`
}

// XDSSyncStatusResponse is the response payload for GET /xds_sync_status.
type XDSSyncStatusResponse struct {
	Component          string    `json:"component"`
	Timestamp          time.Time `json:"timestamp"`
	PolicyChainVersion string    `json:"policy_chain_version"`
}

// HealthResponse is the response payload for GET /health.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// LazyResourcesDump contains information about all lazy resources
type LazyResourcesDump struct {
	TotalResources  int                           `json:"total_resources"`
	ResourcesByType map[string][]LazyResourceInfo `json:"resources_by_type"`
}

// LazyResourceInfo contains information about a single lazy resource
type LazyResourceInfo struct {
	ID           string                 `json:"id"`
	ResourceType string                 `json:"resource_type"`
	Resource     map[string]interface{} `json:"resource"`
}

// PolicyRegistryDump contains information about all registered policies
type PolicyRegistryDump struct {
	TotalPolicies int          `json:"total_policies"`
	Policies      []PolicyInfo `json:"policies"`
}

// PolicyInfo contains information about a single policy
type PolicyInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// PolicyChainsDump contains information about all configured policy chains
type PolicyChainsDump struct {
	TotalPolicyChains int                `json:"total_policy_chains"`
	PolicyChains      []PolicyChainEntry `json:"policy_chains"`
}

// PolicyChainEntry contains the policy chain configuration for a single route
type PolicyChainEntry struct {
	RouteKey             string       `json:"route_key"`
	RequiresRequestBody  bool         `json:"requires_request_body"`
	RequiresResponseBody bool         `json:"requires_response_body"`
	TotalPolicies        int          `json:"total_policies"`
	Policies             []PolicySpec `json:"policies"`
}

// RouteMetadataDump contains route metadata for all configured routes
type RouteMetadataDump struct {
	TotalRoutes int                  `json:"total_routes"`
	Routes      []RouteMetadataEntry `json:"routes"`
}

// RouteMetadataEntry contains metadata for a single route
type RouteMetadataEntry struct {
	RouteKey                string            `json:"route_key"`
	APIId                   string            `json:"api_id"`
	APIName                 string            `json:"api_name"`
	APIVersion              string            `json:"api_version"`
	Context                 string            `json:"context"`
	OperationPath           string            `json:"operation_path"`
	Vhost                   string            `json:"vhost"`
	APIKind                 string            `json:"api_kind"`
	TemplateHandle          string            `json:"template_handle,omitempty"`
	ProviderName            string            `json:"provider_name,omitempty"`
	ProjectID               string            `json:"project_id,omitempty"`
	DefaultUpstreamCluster  string            `json:"default_upstream_cluster"`
	UpstreamBasePath        string            `json:"upstream_base_path"`
	UpstreamDefinitionPaths map[string]string `json:"upstream_definition_paths"`
}

// PolicySpec contains specification for a policy instance
type PolicySpec struct {
	Name               string                 `json:"name"`
	Version            string                 `json:"version"`
	Enabled            bool                   `json:"enabled"`
	ExecutionCondition *string                `json:"execution_condition"`
	Parameters         map[string]interface{} `json:"parameters"`
}

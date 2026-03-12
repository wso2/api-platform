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
	Routes         RoutesDump         `json:"routes"`
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
	Status         string                `json:"status"`
	Timestamp      string                `json:"timestamp"`
	PythonExecutor *PythonExecutorHealth `json:"python_executor,omitempty"`
}

// PythonExecutorHealth holds the health status of the Python executor.
type PythonExecutorHealth struct {
	Status         string `json:"status"`
	LoadedPolicies int32  `json:"loaded_policies"`
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

// RoutesDump contains information about all configured routes
type RoutesDump struct {
	TotalRoutes  int           `json:"total_routes"`
	RouteConfigs []RouteConfig `json:"route_configs"`
}

// RouteConfig contains configuration for a single route
type RouteConfig struct {
	RouteKey             string       `json:"route_key"`
	RequiresRequestBody  bool         `json:"requires_request_body"`
	RequiresResponseBody bool         `json:"requires_response_body"`
	TotalPolicies        int          `json:"total_policies"`
	Policies             []PolicySpec `json:"policies"`
}

// PolicySpec contains specification for a policy instance
type PolicySpec struct {
	Name               string                 `json:"name"`
	Version            string                 `json:"version"`
	Enabled            bool                   `json:"enabled"`
	ExecutionCondition *string                `json:"execution_condition"`
	Parameters         map[string]interface{} `json:"parameters"`
}

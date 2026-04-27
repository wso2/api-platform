/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package models

import "time"

// RuntimeDeployConfig is the kind-agnostic intermediate representation produced by
// each transformer (RestAPI, LLM Provider, LLM Proxy). Both the Envoy xDS translator
// and the policy xDS translator consume this struct.
type RuntimeDeployConfig struct {
	Metadata            Metadata
	Context             string // API base path (e.g. "/weather/$version"); "" for kinds with no context
	PolicyChainResolver string // name of resolver registered in PE (e.g. "route-key", "mcp-tool")
	Routes              map[string]*Route
	PolicyChains        map[string]*PolicyChain
	UpstreamClusters    map[string]*UpstreamCluster
	SensitiveValues     []string // resolved secret plaintext values for redaction; populated from StoredConfig.SensitiveValues
}

// Metadata contains identity information for the deployed API.
type Metadata struct {
	UUID        string
	Kind        string
	Handle      string
	Version     string
	DisplayName string
	ProjectID   string
	LLM         *LLMMetadata // nil for non-LLM kinds
}

// LLMMetadata carries LLM-specific metadata for provider/proxy scenarios.
type LLMMetadata struct {
	TemplateHandle string
	ProviderName   string
}

// Route represents a single Envoy route derived from an API operation.
type Route struct {
	Method          string
	Path            string // full path including context prefix (set by transformer)
	OperationPath   string // original operation path without context prefix
	Vhost           string // "" = default vhost
	AutoHostRewrite bool
	Timeout         *RouteTimeout
	Upstream        RouteUpstream
}

// RouteTimeout holds parsed timeout values for a route.
type RouteTimeout struct {
	Connect *time.Duration
}

// RouteUpstream links a route to its upstream cluster.
type RouteUpstream struct {
	ClusterKey       string // key into UpstreamClusters map
	UseClusterHeader bool   // if true, policy selects upstream dynamically
	DefaultCluster   string // default cluster name when UseClusterHeader is true
}

// PolicyChain is an ordered list of policies for a route.
type PolicyChain struct {
	Policies []Policy
}

// Policy represents a single policy instance within a chain.
type Policy struct {
	Name               string
	Version            string
	Params             map[string]interface{}
	ExecutionCondition *string
}

// UpstreamCluster represents an Envoy cluster with its endpoints.
type UpstreamCluster struct {
	BasePath  string
	Endpoints []Endpoint
	TLS       *UpstreamTLS
}

// Endpoint is a single upstream host:port target.
type Endpoint struct {
	Host string
	Port int
}

// UpstreamTLS holds TLS configuration for an upstream cluster.
type UpstreamTLS struct {
	Enabled bool
}

// ConfigTransformer transforms a StoredConfig into a RuntimeDeployConfig.
type ConfigTransformer interface {
	Transform(cfg *StoredConfig) (*RuntimeDeployConfig, error)
}

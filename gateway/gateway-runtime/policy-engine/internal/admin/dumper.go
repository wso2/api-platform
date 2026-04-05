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

import (
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// DumpConfig dumps the current policy engine configuration
func DumpConfig(k *kernel.Kernel, reg *registry.PolicyRegistry, policyChainVersion string) *ConfigDumpResponse {
	return &ConfigDumpResponse{
		Timestamp:      time.Now(),
		PolicyRegistry: dumpPolicyRegistry(reg),
		PolicyChains:   dumpPolicyChains(k),
		RouteMetadata:  dumpRouteMetadata(k),
		LazyResources:  dumpLazyResources(),
		XDSSync: XDSSyncInfo{
			PolicyChainVersion: policyChainVersion,
		},
	}
}

// dumpPolicyRegistry creates a dump of the policy registry
func dumpPolicyRegistry(reg *registry.PolicyRegistry) PolicyRegistryDump {
	policies := reg.DumpPolicies()

	policyInfos := make([]PolicyInfo, 0, len(policies))
	for _, def := range policies {
		policyInfos = append(policyInfos, PolicyInfo{
			Name:    def.Name,
			Version: def.Version,
		})
	}

	return PolicyRegistryDump{
		TotalPolicies: len(policyInfos),
		Policies:      policyInfos,
	}
}

// dumpPolicyChains creates a dump of all policy chain configurations
func dumpPolicyChains(k *kernel.Kernel) PolicyChainsDump {
	routes := k.DumpRoutes()
	// TODO: (renuka) Redact sensitive info from parameters if any

	entries := make([]PolicyChainEntry, 0, len(routes))
	for routeKey, chain := range routes {
		entries = append(entries, PolicyChainEntry{
			RouteKey:             routeKey,
			RequiresRequestBody:  chain.RequiresRequestBody,
			RequiresResponseBody: chain.RequiresResponseBody,
			TotalPolicies:        len(chain.PolicySpecs),
			Policies:             dumpPolicySpecs(chain.PolicySpecs),
		})
	}

	return PolicyChainsDump{
		TotalPolicyChains: len(entries),
		PolicyChains:      entries,
	}
}

// dumpRouteMetadata creates a dump of all route metadata
func dumpRouteMetadata(k *kernel.Kernel) RouteMetadataDump {
	configs := k.DumpRouteConfigs()

	entries := make([]RouteMetadataEntry, 0, len(configs))
	for routeKey, cfg := range configs {
		entries = append(entries, RouteMetadataEntry{
			RouteKey:                routeKey,
			APIId:                   cfg.Metadata.APIId,
			APIName:                 cfg.Metadata.APIName,
			APIVersion:              cfg.Metadata.APIVersion,
			Context:                 cfg.Metadata.Context,
			OperationPath:           cfg.Metadata.OperationPath,
			Vhost:                   cfg.Metadata.Vhost,
			APIKind:                 cfg.Metadata.APIKind,
			TemplateHandle:          cfg.Metadata.TemplateHandle,
			ProviderName:            cfg.Metadata.ProviderName,
			ProjectID:               cfg.Metadata.ProjectID,
			DefaultUpstreamCluster:  cfg.Metadata.DefaultUpstreamCluster,
			UpstreamBasePath:        cfg.Metadata.UpstreamBasePath,
			UpstreamDefinitionPaths: cfg.Metadata.UpstreamDefinitionPaths,
		})
	}

	return RouteMetadataDump{
		TotalRoutes: len(entries),
		Routes:      entries,
	}
}

// dumpPolicySpecs converts SDK PolicySpecs to admin PolicySpecs
func dumpPolicySpecs(specs []policy.PolicySpec) []PolicySpec {
	result := make([]PolicySpec, 0, len(specs))
	for _, spec := range specs {
		result = append(result, PolicySpec{
			Name:               spec.Name,
			Version:            spec.Version,
			Enabled:            spec.Enabled,
			ExecutionCondition: spec.ExecutionCondition,
			Parameters:         spec.Parameters.Raw,
		})
	}
	return result
}

// dumpLazyResources creates a dump of all lazy resources
func dumpLazyResources() LazyResourcesDump {
	// Get the singleton lazy resource store
	lazyResourceStore := policy.GetLazyResourceStoreInstance()

	// Get all resources
	allResources := lazyResourceStore.GetAllResources()

	// Group resources by type
	resourcesByType := make(map[string][]LazyResourceInfo)
	for _, resource := range allResources {
		info := LazyResourceInfo{
			ID:           resource.ID,
			ResourceType: resource.ResourceType,
			Resource:     resource.Resource,
		}
		resourcesByType[resource.ResourceType] = append(resourcesByType[resource.ResourceType], info)
	}

	return LazyResourcesDump{
		TotalResources:  len(allResources),
		ResourcesByType: resourcesByType,
	}
}

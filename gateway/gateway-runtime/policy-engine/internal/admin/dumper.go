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
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// DumpConfig dumps the current policy engine configuration
func DumpConfig(k *kernel.Kernel, reg *registry.PolicyRegistry) *ConfigDumpResponse {
	return &ConfigDumpResponse{
		Timestamp:      time.Now(),
		PolicyRegistry: dumpPolicyRegistry(reg),
		Routes:         dumpRoutes(k),
		LazyResources:  dumpLazyResources(),
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

// dumpRoutes creates a dump of all route configurations
func dumpRoutes(k *kernel.Kernel) RoutesDump {
	routes := k.DumpRoutes()
	// TODO: (renuka) Redact sensitive info from parameters if any

	routeConfigs := make([]RouteConfig, 0, len(routes))
	for routeKey, chain := range routes {
		routeConfigs = append(routeConfigs, RouteConfig{
			RouteKey:             routeKey,
			RequiresRequestBody:  chain.RequiresRequestBody,
			RequiresResponseBody: chain.RequiresResponseBody,
			TotalPolicies:        len(chain.PolicySpecs),
			Policies:             dumpPolicySpecs(chain.PolicySpecs),
		})
	}

	return RoutesDump{
		TotalRoutes:  len(routeConfigs),
		RouteConfigs: routeConfigs,
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

package admin

import (
	"time"

	"github.com/policy-engine/policy-engine/internal/kernel"
	"github.com/policy-engine/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// DumpConfig dumps the current policy engine configuration
func DumpConfig(k *kernel.Kernel, reg *registry.PolicyRegistry) *ConfigDumpResponse {
	return &ConfigDumpResponse{
		Timestamp:      time.Now(),
		PolicyRegistry: dumpPolicyRegistry(reg),
		Routes:         dumpRoutes(k),
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

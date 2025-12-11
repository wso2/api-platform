package kernel

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/policy-engine/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

// ConfigLoader loads policy chain configurations
// T077: Implement file-based configuration loader as fallback
type ConfigLoader struct {
	kernel   *Kernel
	registry *registry.PolicyRegistry
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader(kernel *Kernel, reg *registry.PolicyRegistry) *ConfigLoader {
	return &ConfigLoader{
		kernel:   kernel,
		registry: reg,
	}
}

// LoadFromFile loads policy chain configurations from a YAML file
// T077: File-based configuration loader implementation
func (cl *ConfigLoader) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var configs []policyenginev1.PolicyChain
	if err := yaml.Unmarshal(data, &configs); err != nil {
		return fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// T075: Implement configuration validation before applying
	for _, config := range configs {
		if err := cl.validateConfig(&config); err != nil {
			return fmt.Errorf("invalid configuration for route %s: %w", config.RouteKey, err)
		}
	}

	// T076: Implement atomic PolicyChain replacement in Routes map
	// Build all chains first, then replace atomically
	chains := make(map[string]*registry.PolicyChain)
	for _, config := range configs {
		chain, err := cl.buildPolicyChain(config.RouteKey, &config)
		if err != nil {
			return fmt.Errorf("failed to build policy chain for route %s: %w", config.RouteKey, err)
		}
		chains[config.RouteKey] = chain
	}

	// Replace routes atomically
	cl.kernel.mu.Lock()
	defer cl.kernel.mu.Unlock()

	ctx := context.Background()
	for routeKey, chain := range chains {
		cl.kernel.Routes[routeKey] = chain
		slog.InfoContext(ctx, "Loaded policy chain for route",
			"route", routeKey,
			"policies", len(chain.Policies))
	}

	return nil
}

// validateConfig validates a policy chain configuration
// T075: Configuration validation implementation
func (cl *ConfigLoader) validateConfig(config *policyenginev1.PolicyChain) error {
	if config.RouteKey == "" {
		return fmt.Errorf("route_key is required")
	}

	for i, policyConfig := range config.Policies {
		if policyConfig.Name == "" {
			return fmt.Errorf("policy[%d]: name is required", i)
		}
		if policyConfig.Version == "" {
			return fmt.Errorf("policy[%d]: version is required", i)
		}

		// Check if policy exists in registry
		_, err := cl.registry.GetDefinition(policyConfig.Name, policyConfig.Version)
		if err != nil {
			return fmt.Errorf("policy[%d]: %w", i, err)
		}

		_, err = cl.registry.GetFactory(policyConfig.Name, policyConfig.Version)
		if err != nil {
			return fmt.Errorf("policy[%d]: %w", i, err)
		}
	}

	return nil
}

// buildPolicyChain builds a PolicyChain from configuration
func (cl *ConfigLoader) buildPolicyChain(routeKey string, config *policyenginev1.PolicyChain) (*registry.PolicyChain, error) {
	var policyList []policy.Policy
	var policySpecs []policy.PolicySpec

	requiresRequestBody := false
	requiresResponseBody := false

	for _, policyConfig := range config.Policies {
		// Create metadata with route information
		metadata := policy.PolicyMetadata{
			RouteName: routeKey,
		}

		// Create instance using factory with metadata and params
		impl, err := cl.registry.CreateInstance(policyConfig.Name, policyConfig.Version, metadata, policyConfig.Parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to create policy instance %s:%s for route %s: %w",
				policyConfig.Name, policyConfig.Version, routeKey, err)
		}

		// Build PolicySpec
		spec := policy.PolicySpec{
			Name:               policyConfig.Name,
			Version:            policyConfig.Version,
			Enabled:            policyConfig.Enabled,
			ExecutionCondition: policyConfig.ExecutionCondition,
			Parameters: policy.PolicyParameters{
				Raw: policyConfig.Parameters,
			},
		}

		// Add to policy list
		policyList = append(policyList, impl)
		policySpecs = append(policySpecs, spec)

		// Get policy mode and update body requirements
		mode := impl.Mode()

		// Update request body requirement (if any policy needs buffering)
		if mode.RequestBodyMode == policy.BodyModeBuffer || mode.RequestBodyMode == policy.BodyModeStream {
			requiresRequestBody = true
		}

		// Update response body requirement (if any policy needs buffering)
		if mode.ResponseBodyMode == policy.BodyModeBuffer || mode.ResponseBodyMode == policy.BodyModeStream {
			requiresResponseBody = true
		}
	}

	chain := &registry.PolicyChain{
		Policies:             policyList,
		PolicySpecs:          policySpecs,
		RequiresRequestBody:  requiresRequestBody,
		RequiresResponseBody: requiresResponseBody,
	}

	return chain, nil
}

// PolicyDiscoveryService would implement full xDS protocol
// T071-T074, T076: xDS service (stub for future implementation)
// For MVP, we use file-based configuration via ConfigLoader above
type PolicyDiscoveryService struct {
	// Future: implement xDS streaming service
	// This would handle StreamPolicyMappings with versioning
	// For now, rely on ConfigLoader for static configuration
}

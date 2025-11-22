package kernel

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/policy-engine/sdk/core"
	"github.com/policy-engine/sdk/policies"
)

// PolicyChainConfig represents the configuration for a policy chain on a route
// T072: PolicyChainConfig structure for xDS or file-based config
type PolicyChainConfig struct {
	RouteKey string                 `yaml:"route_key"`
	Policies []PolicyInstanceConfig `yaml:"policies"`
}

// PolicyInstanceConfig represents a single policy instance in a chain
type PolicyInstanceConfig struct {
	Name               string                 `yaml:"name"`
	Version            string                 `yaml:"version"`
	Enabled            bool                   `yaml:"enabled"`
	ExecutionCondition *string                `yaml:"executionCondition,omitempty"`
	Parameters         map[string]interface{} `yaml:"parameters"`
}

// ConfigLoader loads policy chain configurations
// T077: Implement file-based configuration loader as fallback
type ConfigLoader struct {
	kernel   *Kernel
	registry *core.PolicyRegistry
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader(kernel *Kernel, registry *core.PolicyRegistry) *ConfigLoader {
	return &ConfigLoader{
		kernel:   kernel,
		registry: registry,
	}
}

// LoadFromFile loads policy chain configurations from a YAML file
// T077: File-based configuration loader implementation
func (cl *ConfigLoader) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var configs []PolicyChainConfig
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
	chains := make(map[string]*core.PolicyChain)
	for _, config := range configs {
		chain, err := cl.buildPolicyChain(&config)
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
			"request_policies", len(chain.RequestPolicies),
			"response_policies", len(chain.ResponsePolicies))
	}

	return nil
}

// validateConfig validates a policy chain configuration
// T075: Configuration validation implementation
func (cl *ConfigLoader) validateConfig(config *PolicyChainConfig) error {
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

		_, err = cl.registry.GetImplementation(policyConfig.Name, policyConfig.Version)
		if err != nil {
			return fmt.Errorf("policy[%d]: %w", i, err)
		}
	}

	return nil
}

// buildPolicyChain builds a PolicyChain from configuration
func (cl *ConfigLoader) buildPolicyChain(config *PolicyChainConfig) (*core.PolicyChain, error) {
	var requestPolicies []policies.RequestPolicy
	var responsePolicies []policies.ResponsePolicy
	var requestSpecs []policies.PolicySpec
	var responseSpecs []policies.PolicySpec

	requiresRequestBody := false
	requiresResponseBody := false

	for _, policyConfig := range config.Policies {
		// Get policy definition and implementation
		def, err := cl.registry.GetDefinition(policyConfig.Name, policyConfig.Version)
		if err != nil {
			return nil, err
		}

		impl, err := cl.registry.GetImplementation(policyConfig.Name, policyConfig.Version)
		if err != nil {
			return nil, err
		}

		// Build PolicySpec
		spec := policies.PolicySpec{
			Name:               policyConfig.Name,
			Version:            policyConfig.Version,
			Enabled:            policyConfig.Enabled,
			ExecutionCondition: policyConfig.ExecutionCondition,
			Parameters: policies.PolicyParameters{
				Raw: policyConfig.Parameters,
			},
		}

		// Add to appropriate list based on policy type
		if reqPolicy, ok := impl.(policies.RequestPolicy); ok {
			requestPolicies = append(requestPolicies, reqPolicy)
			requestSpecs = append(requestSpecs, spec)
			if def.RequiresRequestBody {
				requiresRequestBody = true
			}
		}

		if respPolicy, ok := impl.(policies.ResponsePolicy); ok {
			responsePolicies = append(responsePolicies, respPolicy)
			responseSpecs = append(responseSpecs, spec)
			if def.RequiresResponseBody {
				requiresResponseBody = true
			}
		}
	}

	chain := &core.PolicyChain{
		RequestPolicies:      requestPolicies,
		ResponsePolicies:     responsePolicies,
		RequestPolicySpecs:   requestSpecs,
		ResponsePolicySpecs:  responseSpecs,
		Metadata:             make(map[string]interface{}),
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

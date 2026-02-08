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

package kernel

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
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
		// File-based config doesn't have API metadata, pass empty values
		chain, err := cl.buildPolicyChain(config.RouteKey, &config, policyenginev1.Metadata{})
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
func (cl *ConfigLoader) buildPolicyChain(routeKey string, config *policyenginev1.PolicyChain, apiMetadata policyenginev1.Metadata) (*registry.PolicyChain, error) {
	var policyList []policy.Policy
	var policySpecs []policy.PolicySpec

	requiresRequestBody := false
	requiresResponseBody := false
	hasExecutionConditions := false

	for _, policyConfig := range config.Policies {
		// Create metadata with route and API information
		metadata := policy.PolicyMetadata{
			RouteName:  routeKey,
			APIId:      apiMetadata.APIId,
			APIName:    apiMetadata.APIName,
			APIVersion: apiMetadata.Version,
		}

		// Create instance using factory with metadata and params
		// CreateInstance returns the policy and merged params (initParams + runtime params)
		impl, mergedParams, err := cl.registry.CreateInstance(policyConfig.Name, policyConfig.Version, metadata, policyConfig.Parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to create policy instance %s:%s for route %s: %w",
				policyConfig.Name, policyConfig.Version, routeKey, err)
		}

		// Build PolicySpec with merged params so OnRequest/OnResponse receive merged values
		spec := policy.PolicySpec{
			Name:               policyConfig.Name,
			Version:            policyConfig.Version,
			Enabled:            policyConfig.Enabled,
			ExecutionCondition: policyConfig.ExecutionCondition,
			Parameters: policy.PolicyParameters{
				Raw: mergedParams,
			},
		}

		// Check if policy has CEL execution condition
		if policyConfig.ExecutionCondition != nil && *policyConfig.ExecutionCondition != "" {
			hasExecutionConditions = true
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
		HasExecutionConditions:     hasExecutionConditions,
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

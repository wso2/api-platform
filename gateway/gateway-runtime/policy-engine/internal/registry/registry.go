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

package registry

import (
	"fmt"
	"sync"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// PolicyRegistry provides centralized policy lookup
// THREAD-SAFETY: This registry is initialized during program startup (via init() functions)
// before any concurrent access begins. All Register() calls must complete before the gRPC
// server starts serving requests. After initialization, the maps are read-only and safe for
// concurrent access without synchronization.
type PolicyRegistry struct {
	// Policy definitions indexed by "name:version" composite key
	// Example key: "jwtValidation:v1.0.0"
	Definitions map[string]*policy.PolicyDefinition

	// Policy factory functions indexed by "name:version" composite key
	// Factory creates policy instances with metadata, initParams, and params
	Factories map[string]policy.PolicyFactory

	// ConfigResolver resolves ${config} CEL expressions in systemParameters
	ConfigResolver *ConfigResolver
}

// Global singleton registry
var globalRegistry *PolicyRegistry
var registryOnce sync.Once

// GetRegistry returns the global policy registry singleton
func GetRegistry() *PolicyRegistry {
	registryOnce.Do(func() {
		globalRegistry = &PolicyRegistry{
			Definitions: make(map[string]*policy.PolicyDefinition),
			Factories:   make(map[string]policy.PolicyFactory),
		}
	})
	return globalRegistry
}

// mergeParams merges initParams (resolved) with runtime params
// Runtime params override init params when keys conflict
func mergeParams(initParams, params map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(initParams)+len(params))

	// Copy initParams first
	for k, v := range initParams {
		merged[k] = v
	}

	// Override with runtime params
	for k, v := range params {
		merged[k] = v
	}

	return merged
}

// GetDefinition retrieves a policy definition by name and version
func (r *PolicyRegistry) GetDefinition(name, version string) (*policy.PolicyDefinition, error) {
	key := compositeKey(name, version)
	def, ok := r.Definitions[key]
	if !ok {
		return nil, fmt.Errorf("policy definition not found: %s", key)
	}
	return def, nil
}

// CreateInstance creates a new policy instance for a specific route
// This method is called during BuildPolicyChain for each route-policy combination
// Returns the policy instance and the merged parameters (initParams + params)
func (r *PolicyRegistry) CreateInstance(
	name, version string,
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, map[string]interface{}, error) {
	key := compositeKey(name, version)

	factory, ok := r.Factories[key]
	if !ok {
		return nil, nil, fmt.Errorf("policy factory not found: %s", key)
	}

	def, ok := r.Definitions[key]
	if !ok {
		return nil, nil, fmt.Errorf("policy definition not found: %s", key)
	}

	// Extract initParams from PolicyDefinition
	initParams := def.SystemParameters
	if initParams == nil {
		initParams = make(map[string]interface{})
	}

	// Resolve ${config} references in initParams
	if r.ConfigResolver == nil {
		return nil, nil, fmt.Errorf("policy %s: ConfigResolver is not initialized", key)
	}
	var err error
	initParams, err = r.ConfigResolver.ResolveMap(initParams)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve config for policy %s: %w", key, err)
	}

	// Merge resolved initParams with runtime params (params override initParams)
	mergedParams := mergeParams(initParams, params)

	// Call factory to create instance with merged params
	instance, err := factory(metadata, mergedParams)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create policy instance %s: %w", key, err)
	}

	return instance, mergedParams, nil
}

// GetFactory retrieves a policy factory by name and version
// Useful for validation without creating instances
func (r *PolicyRegistry) GetFactory(name, version string) (policy.PolicyFactory, error) {
	key := compositeKey(name, version)
	factory, ok := r.Factories[key]
	if !ok {
		return nil, fmt.Errorf("policy factory not found: %s", key)
	}
	return factory, nil
}

// Register registers a policy definition and factory function
// This method is ONLY called during init() before any concurrent access begins. Hence no need for synchronization.
func (r *PolicyRegistry) Register(def *policy.PolicyDefinition, factory policy.PolicyFactory) error {
	key := compositeKey(def.Name, def.Version)

	// Check for duplicates
	if _, exists := r.Definitions[key]; exists {
		return fmt.Errorf("policy already registered: %s", key)
	}

	r.Definitions[key] = def
	r.Factories[key] = factory
	return nil
}

// SetConfig sets the configuration for resolving ${config} references in systemParameters
// This should be called during startup after loading the config file
func (r *PolicyRegistry) SetConfig(config map[string]interface{}) error {
	resolver, err := NewConfigResolver(config)
	if err != nil {
		return fmt.Errorf("failed to create config resolver: %w", err)
	}
	r.ConfigResolver = resolver
	return nil
}

// compositeKey creates a composite key from name and version
func compositeKey(name, version string) string {
	return fmt.Sprintf("%s:%s", name, version)
}

// DumpPolicies returns all registered policy definitions for debugging
// Returns a copy of the definitions map
func (r *PolicyRegistry) DumpPolicies() map[string]*policy.PolicyDefinition {
	// Create a copy of the definitions map
	dump := make(map[string]*policy.PolicyDefinition, len(r.Definitions))
	for key, def := range r.Definitions {
		dump[key] = def
	}
	return dump
}

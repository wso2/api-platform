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

	"github.com/wso2/api-platform/common/version"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// PolicyEntry holds a policy definition and its factory function together
type PolicyEntry struct {
	Definition *policy.PolicyDefinition
	Factory    policy.PolicyFactory
}

// PolicyRegistry provides centralized policy lookup
// THREAD-SAFETY: This registry is initialized during program startup (via init() functions)
// before any concurrent access begins. All Register() calls must complete before the gRPC
// server starts serving requests. After initialization, the map is read-only and safe for
// concurrent access without synchronization.
type PolicyRegistry struct {
	// Policies indexed by "name:vN" composite key (major version only)
	// Example key: "jwtValidation:v1"
	Policies map[string]*PolicyEntry

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
			Policies: make(map[string]*PolicyEntry),
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

// GetInstance returns a policy instance for a specific route-policy combination.
// The factory may return a new instance or a cached one — this is up to the policy implementation.
// Returns the policy instance and the merged parameters (initParams + params)
func (r *PolicyRegistry) GetInstance(
	name, version string,
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, map[string]interface{}, error) {
	key := compositeKey(name, version)

	entry, ok := r.Policies[key]
	if !ok {
		return nil, nil, fmt.Errorf("policy not found: %s", key)
	}

	// Extract initParams from PolicyDefinition
	initParams := entry.Definition.SystemParameters
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
	instance, err := entry.Factory(metadata, mergedParams)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create policy instance %s: %w", key, err)
	}

	return instance, mergedParams, nil
}

// PolicyExists checks if a policy with the given name and version is registered
// (both definition and factory must exist)
func (r *PolicyRegistry) PolicyExists(name, version string) error {
	key := compositeKey(name, version)
	if _, ok := r.Policies[key]; !ok {
		return fmt.Errorf("policy not found: %s", key)
	}
	return nil
}

// Register registers a policy definition and factory function.
// The registry key uses the major version extracted from def.Version (e.g., "jwt-auth:v1").
// Only one version per major version can be registered.
// This method is ONLY called during init() before any concurrent access begins. Hence no need for synchronization.
func (r *PolicyRegistry) Register(def *policy.PolicyDefinition, factory policy.PolicyFactory) error {
	majorVer := version.MajorVersion(def.Version)
	key := compositeKey(def.Name, majorVer)

	// Check for duplicates
	if existingEntry, exists := r.Policies[key]; exists {
		return fmt.Errorf("duplicate policies for major version %s: attempting to register (name: %s, version: %s) but already registered (name: %s, version: %s)",
			majorVer,
			def.Name, def.Version,
			existingEntry.Definition.Name, existingEntry.Definition.Version)
	}

	r.Policies[key] = &PolicyEntry{
		Definition: def,
		Factory:    factory,
	}
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
	dump := make(map[string]*policy.PolicyDefinition, len(r.Policies))
	for key, entry := range r.Policies {
		dump[key] = entry.Definition
	}
	return dump
}

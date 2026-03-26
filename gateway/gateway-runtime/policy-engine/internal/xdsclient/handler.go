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

package xdsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/common/apikey"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	policyenginev1 "github.com/wso2/api-platform/sdk/gateway/policyengine/v1"
)

// StoredPolicyConfig represents stored policy configuration from gateway-controller
// Uses SDK types for routes, adds gateway-specific metadata wrapper
type StoredPolicyConfig struct {
	ID            string                       `json:"id"`
	Configuration policyenginev1.Configuration `json:"configuration"`
	Version       int64                        `json:"version"`
}

// ResourceHandler handles xDS resource updates
type ResourceHandler struct {
	kernel              *kernel.Kernel
	registry            *registry.PolicyRegistry
	configLoader        *kernel.ConfigLoader
	apiKeyHandler       *APIKeyOperationHandler
	lazyResourceHandler *LazyResourceHandler
	subscriptionStore   *policyenginev1.SubscriptionStore
	subscriptionHandler *SubscriptionStateHandler
}

// NewResourceHandler creates a new ResourceHandler
func NewResourceHandler(k *kernel.Kernel, reg *registry.PolicyRegistry) *ResourceHandler {
	apiKeyStore := apikey.GetAPIkeyStoreInstance()
	lazyResourceStore := policy.GetLazyResourceStoreInstance()
	subStore := policyenginev1.GetSubscriptionStoreInstance()
	return &ResourceHandler{
		kernel:              k,
		registry:            reg,
		configLoader:        kernel.NewConfigLoader(k, reg),
		apiKeyHandler:       NewAPIKeyOperationHandler(apiKeyStore, slog.Default()),
		lazyResourceHandler: NewLazyResourceHandler(lazyResourceStore, slog.Default()),
		subscriptionStore:   subStore,
		subscriptionHandler: NewSubscriptionStateHandler(subStore, slog.Default()),
	}
}

// policyChainWithMetadata pairs a PolicyChain config with its API metadata
type policyChainWithMetadata struct {
	config   *policyenginev1.PolicyChain
	metadata policyenginev1.Metadata
}

// HandlePolicyChainUpdate processes custom PolicyChainConfig resources from ADS response
func (h *ResourceHandler) HandlePolicyChainUpdate(ctx context.Context, resources []*anypb.Any, version string) error {
	slog.InfoContext(ctx, "Handling policy chain update via ADS",
		"version", version,
		"num_resources", len(resources))

	// Parse all resources first (validation phase)
	// Each resource is a StoredPolicyConfig containing multiple routes with shared API metadata
	configsWithMetadata := make([]policyChainWithMetadata, 0)

	for i, resource := range resources {
		if resource.TypeUrl != PolicyChainTypeURL {
			slog.WarnContext(ctx, "Skipping resource with unexpected type",
				"expected", PolicyChainTypeURL,
				"actual", resource.TypeUrl,
				"index", i)
			continue
		}

		// Unmarshal google.protobuf.Struct from the Any
		// The xDS server double-wraps: res.Value contains serialized Any,
		// which in turn contains the serialized Struct
		innerAny := &anypb.Any{}
		if err := proto.Unmarshal(resource.Value, innerAny); err != nil {
			return fmt.Errorf("failed to unmarshal inner Any from resource: %w", err)
		}

		// Now unmarshal the Struct from the inner Any's Value
		policyStruct := &structpb.Struct{}
		if err := proto.Unmarshal(innerAny.Value, policyStruct); err != nil {
			return fmt.Errorf("failed to unmarshal policy struct from inner Any: %w", err)
		}

		// Convert Struct to JSON then to StoredPolicyConfig
		jsonBytes, err := protojson.Marshal(policyStruct)
		if err != nil {
			return fmt.Errorf("failed to marshal policy struct to JSON: %w", err)
		}

		var storedConfig StoredPolicyConfig
		if err := json.Unmarshal(jsonBytes, &storedConfig); err != nil {
			return fmt.Errorf("failed to unmarshal stored policy config from JSON: %w", err)
		}

		slog.InfoContext(ctx, "Parsed StoredPolicyConfig",
			"id", storedConfig.ID,
			"api_name", storedConfig.Configuration.Metadata.APIName,
			"routes", len(storedConfig.Configuration.Routes))

		// Extract API metadata (shared by all routes in this StoredPolicyConfig)
		apiMetadata := storedConfig.Configuration.Metadata

		// Extract PolicyChain configurations (already in SDK format)
		routeConfigs := h.convertStoredConfigToPolicyChains(&storedConfig)

		// Validate each route configuration and pair with metadata
		// Note: We log errors but continue to avoid NACK loops when policies are missing
		for _, config := range routeConfigs {
			if err := h.validatePolicyChainConfig(config); err != nil {
				slog.WarnContext(ctx, "Skipping invalid route configuration",
					"route", config.RouteKey,
					"error", err)
				continue // Skip this route but process others
			}
			configsWithMetadata = append(configsWithMetadata, policyChainWithMetadata{
				config:   config,
				metadata: apiMetadata,
			})
		}
	}

	// Build all policy chains (can fail if policy not found or validation fails)
	chains := make(map[string]*registry.PolicyChain)
	for _, cwm := range configsWithMetadata {
		chain, err := h.buildPolicyChain(cwm.config.RouteKey, cwm.config, cwm.metadata)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to build policy chain for route, skipping",
				"route", cwm.config.RouteKey,
				"error", err)
			continue // Skip this route but process others
		}
		chains[cwm.config.RouteKey] = chain
	}

	// Apply changes atomically
	// This replaces ALL routes with the new set from xDS (State of the World)
	h.kernel.ApplyWholeRoutes(chains)

	// Record metrics for policy chains loaded
	metrics.PolicyChainsLoaded.WithLabelValues("ads").Set(float64(len(chains)))

	// Calculate and record policies per chain
	for routeKey, chain := range chains {
		policyCount := float64(len(chain.Policies))
		// Extract API name from route key (format: "api-name::route-name" or just routeKey)
		apiName := routeKey
		if strings.Contains(routeKey, "::") {
			parts := strings.SplitN(routeKey, "::", 2)
			if len(parts) == 2 {
				apiName = parts[0]
			}
		}
		metrics.PoliciesPerChain.WithLabelValues(routeKey, apiName).Set(policyCount)
	}

	slog.InfoContext(ctx, "Policy chain update completed successfully",
		"version", version,
		"total_routes", len(chains))

	return nil
}

// HandleRouteConfigUpdate processes RouteConfig resources from ADS response.
// These contain metadata, resolver name, and upstream path info for each route.
func (h *ResourceHandler) HandleRouteConfigUpdate(ctx context.Context, resources []*anypb.Any, version string) error {
	slog.InfoContext(ctx, "Handling route config update via ADS",
		"version", version,
		"num_resources", len(resources))

	routeConfigs := make(map[string]*kernel.RouteConfig)

	for i, resource := range resources {
		if resource.TypeUrl != RouteConfigTypeURL {
			slog.WarnContext(ctx, "Skipping resource with unexpected type",
				"expected", RouteConfigTypeURL,
				"actual", resource.TypeUrl,
				"index", i)
			continue
		}

		// Unmarshal google.protobuf.Struct from the Any
		innerAny := &anypb.Any{}
		if err := proto.Unmarshal(resource.Value, innerAny); err != nil {
			return fmt.Errorf("failed to unmarshal inner Any from route config resource: %w", err)
		}

		routeStruct := &structpb.Struct{}
		if err := proto.Unmarshal(innerAny.Value, routeStruct); err != nil {
			return fmt.Errorf("failed to unmarshal route config struct from inner Any: %w", err)
		}

		// Convert Struct to JSON then to a map
		jsonBytes, err := protojson.Marshal(routeStruct)
		if err != nil {
			return fmt.Errorf("failed to marshal route config struct to JSON: %w", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &data); err != nil {
			return fmt.Errorf("failed to unmarshal route config JSON: %w", err)
		}

		routeKey, _ := data["route_key"].(string)
		if routeKey == "" {
			slog.WarnContext(ctx, "Skipping route config with empty route_key", "index", i)
			continue
		}

		rc := &kernel.RouteConfig{}

		// Parse metadata
		if metaMap, ok := data["metadata"].(map[string]interface{}); ok {
			rc.Metadata = kernel.RouteMetadata{
				RouteName:      routeKey,
				APIName:        getStringFromMap(metaMap, "display_name"),
				APIVersion:     getStringFromMap(metaMap, "version"),
				Context:        getStringFromMap(metaMap, "api_context"),
				Vhost:          getStringFromMap(metaMap, "vhost"),
				APIKind:        getStringFromMap(metaMap, "kind"),
				TemplateHandle: getStringFromMap(metaMap, "template_handle"),
				ProviderName:   getStringFromMap(metaMap, "provider_name"),
				ProjectID:      getStringFromMap(metaMap, "project_id"),
				OperationPath:  getStringFromMap(metaMap, "path"),
				APIId:          getStringFromMap(metaMap, "uuid"),
			}
		}

		rc.Metadata.DefaultUpstreamCluster = getStringFromMap(data, "default_upstream_cluster")
		rc.Metadata.UpstreamBasePath = getStringFromMap(data, "upstream_base_path")

		if pathsRaw, ok := data["upstream_definition_paths"].(map[string]interface{}); ok {
			paths := make(map[string]string, len(pathsRaw))
			for k, v := range pathsRaw {
				if s, ok := v.(string); ok {
					paths[k] = s
				}
			}
			rc.Metadata.UpstreamDefinitionPaths = paths
		}

		routeConfigs[routeKey] = rc
	}

	// Apply atomically
	h.kernel.ApplyWholeRouteConfigs(routeConfigs)

	slog.InfoContext(ctx, "Route config update completed successfully",
		"version", version,
		"total_routes", len(routeConfigs))

	return nil
}

// getStringFromMap safely extracts a string value from a map.
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// convertStoredConfigToPolicyChains extracts PolicyChain configurations from StoredPolicyConfig
// With SDK types, the routes are already in the correct format
func (h *ResourceHandler) convertStoredConfigToPolicyChains(stored *StoredPolicyConfig) []*policyenginev1.PolicyChain {
	// Routes are already in PolicyChain format from the SDK
	// Just convert from slice values to slice of pointers
	configs := make([]*policyenginev1.PolicyChain, 0, len(stored.Configuration.Routes))
	for i := range stored.Configuration.Routes {
		configs = append(configs, &stored.Configuration.Routes[i])
	}
	return configs
}

// validatePolicyChainConfig validates a PolicyChain configuration.
// Only checks structural requirements (non-empty route key, name, version).
// Policy existence is handled gracefully in buildPolicyChain.
func (h *ResourceHandler) validatePolicyChainConfig(config *policyenginev1.PolicyChain) error {
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
		if err := h.registry.PolicyExists(policyConfig.Name, policyConfig.Version); err != nil {
			return fmt.Errorf("policy[%d]: %w", i, err)
		}
	}

	return nil
}

// getAllRouteKeys returns all registered route keys
// Note: This is a workaround since we can't access kernel.Routes directly
// In a production system, you might want to add a GetAllRoutes() method to Kernel
func (h *ResourceHandler) getAllRouteKeys() []string {
	// Since we can't access kernel.Routes directly, we'll track routes ourselves
	// or implement GetAllRoutes() in the Kernel
	// For now, we'll just return empty slice as xDS State of the World
	// will send all routes anyway, so unregistering is optional
	return []string{}
}

// buildPolicyChain builds a PolicyChain from configuration
// This is a copy of kernel.ConfigLoader.buildPolicyChain logic
func (h *ResourceHandler) buildPolicyChain(routeKey string, config *policyenginev1.PolicyChain, apiMetadata policyenginev1.Metadata) (*registry.PolicyChain, error) {
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

		// Check if attachedTo is present in parameters and set it in metadata
		if val, ok := policyConfig.Parameters["attachedTo"]; ok {
			if attachedTo, ok := val.(string); ok {
				metadata.AttachedTo = policy.Level(attachedTo)
			}
		}

		// Get instance using factory with metadata and params
		// GetInstance returns the policy and merged params (initParams + runtime params)
		impl, mergedParams, err := h.registry.GetInstance(policyConfig.Name, policyConfig.Version, metadata, policyConfig.Parameters)
		if err != nil {
			// Fail the entire chain rather than silently omitting a policy.
			// A security policy (e.g. api-key-auth) that fails to instantiate must not
			// be silently skipped — doing so would let traffic pass without that policy.
			return nil, fmt.Errorf("failed to create policy instance %s:%s: %w", policyConfig.Name, policyConfig.Version, err)
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
		if mode.RequestBodyMode == policy.BodyModeBuffer || mode.RequestBodyMode == policy.BodyModeStream {
			requiresRequestBody = true
		}
		if mode.ResponseBodyMode == policy.BodyModeBuffer || mode.ResponseBodyMode == policy.BodyModeStream {
			requiresResponseBody = true
		}
	}

	chain := &registry.PolicyChain{
		Policies:               policyList,
		PolicySpecs:            policySpecs,
		RequiresRequestBody:    requiresRequestBody,
		RequiresResponseBody:   requiresResponseBody,
		HasExecutionConditions: hasExecutionConditions,
	}

	return chain, nil
}

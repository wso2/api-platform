package xdsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/policy-engine/policy-engine/internal/kernel"
	"github.com/policy-engine/policy-engine/internal/registry"
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
	kernel       *kernel.Kernel
	registry     *registry.PolicyRegistry
	configLoader *kernel.ConfigLoader
}

// NewResourceHandler creates a new ResourceHandler
func NewResourceHandler(k *kernel.Kernel, reg *registry.PolicyRegistry) *ResourceHandler {
	return &ResourceHandler{
		kernel:       k,
		registry:     reg,
		configLoader: kernel.NewConfigLoader(k, reg),
	}
}

// HandlePolicyChainUpdate processes custom PolicyChainConfig resources from ADS response
func (h *ResourceHandler) HandlePolicyChainUpdate(ctx context.Context, resources []*anypb.Any, version string) error {
	slog.InfoContext(ctx, "Handling policy chain update via ADS",
		"version", version,
		"num_resources", len(resources))

	// Parse all resources first (validation phase)
	// Each resource is a StoredPolicyConfig containing multiple routes
	configs := make([]*policyenginev1.PolicyChain, 0)

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

		// Extract PolicyChain configurations (already in SDK format)
		routeConfigs := h.convertStoredConfigToPolicyChains(&storedConfig)

		// Validate each route configuration
		// Note: We log errors but continue to avoid NACK loops when policies are missing
		for _, config := range routeConfigs {
			if err := h.validatePolicyChainConfig(config); err != nil {
				slog.WarnContext(ctx, "Skipping invalid route configuration",
					"route", config.RouteKey,
					"error", err)
				continue // Skip this route but process others
			}
			configs = append(configs, config) // Only add valid configs
		}
	}

	// Build all policy chains (can fail if policy not found or validation fails)
	chains := make(map[string]*registry.PolicyChain)
	for _, config := range configs {
		chain, err := h.buildPolicyChain(config.RouteKey, config)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to build policy chain for route, skipping",
				"route", config.RouteKey,
				"error", err)
			continue // Skip this route but process others
		}
		chains[config.RouteKey] = chain
	}

	// Apply changes atomically
	// This replaces ALL routes with the new set from xDS (State of the World)
	h.kernel.ApplyWholeRoutes(chains)

	slog.InfoContext(ctx, "Policy chain update completed successfully",
		"version", version,
		"total_routes", len(chains))

	return nil
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

// validatePolicyChainConfig validates a PolicyChain configuration
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
		_, err := h.registry.GetDefinition(policyConfig.Name, policyConfig.Version)
		if err != nil {
			return fmt.Errorf("policy[%d]: %w", i, err)
		}

		_, err = h.registry.GetFactory(policyConfig.Name, policyConfig.Version)
		if err != nil {
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
func (h *ResourceHandler) buildPolicyChain(routeKey string, config *policyenginev1.PolicyChain) (*registry.PolicyChain, error) {
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
		impl, err := h.registry.CreateInstance(policyConfig.Name, policyConfig.Version, metadata, policyConfig.Parameters)
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

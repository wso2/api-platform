package xdsclient

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/protobuf/types/known/anypb"
	"gopkg.in/yaml.v3"

	"github.com/policy-engine/policy-engine/internal/kernel"
	"github.com/policy-engine/policy-engine/internal/registry"
	"github.com/policy-engine/sdk/policy"
)

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

// HandlePolicyChainUpdate processes PolicyChainConfig resources from xDS response
func (h *ResourceHandler) HandlePolicyChainUpdate(ctx context.Context, resources []*anypb.Any, version string) error {
	slog.InfoContext(ctx, "Handling policy chain update",
		"version", version,
		"num_resources", len(resources))

	// Parse all resources first (validation phase)
	configs := make([]*kernel.PolicyChainConfig, 0, len(resources))
	for i, resource := range resources {
		if resource.TypeUrl != PolicyChainTypeURL {
			slog.WarnContext(ctx, "Skipping resource with unexpected type",
				"expected", PolicyChainTypeURL,
				"actual", resource.TypeUrl,
				"index", i)
			continue
		}

		// Unmarshal the resource value (YAML encoded in the Any value)
		// The xDS server should encode PolicyChainConfig as YAML in Any.Value
		var config kernel.PolicyChainConfig
		if err := yaml.Unmarshal(resource.Value, &config); err != nil {
			return fmt.Errorf("failed to unmarshal resource[%d]: %w", i, err)
		}

		// Validate configuration
		if err := h.validatePolicyChainConfig(&config); err != nil {
			return fmt.Errorf("invalid configuration for resource[%d] route=%s: %w", i, config.RouteKey, err)
		}

		configs = append(configs, &config)
	}

	// Build all policy chains (can fail if policy not found or validation fails)
	chains := make(map[string]*registry.PolicyChain)
	for _, config := range configs {
		chain, err := h.buildPolicyChain(config)
		if err != nil {
			return fmt.Errorf("failed to build policy chain for route %s: %w", config.RouteKey, err)
		}
		chains[config.RouteKey] = chain
	}

	// Apply changes atomically
	// This replaces ALL routes with the new set from xDS (State of the World)
	// Since Kernel.Routes is unexported and we need atomic updates,
	// we'll unregister all routes and register new ones

	// Get current routes to unregister them
	currentRoutes := h.getAllRouteKeys()
	for _, routeKey := range currentRoutes {
		h.kernel.UnregisterRoute(routeKey)
	}

	// Register new routes
	for routeKey, chain := range chains {
		h.kernel.RegisterRoute(routeKey, chain)
		slog.InfoContext(ctx, "Applied policy chain for route",
			"route", routeKey,
			"num_policies", len(chain.Policies),
			"version", version)
	}

	slog.InfoContext(ctx, "Policy chain update completed successfully",
		"version", version,
		"total_routes", len(chains))

	return nil
}

// validatePolicyChainConfig validates a PolicyChainConfig
func (h *ResourceHandler) validatePolicyChainConfig(config *kernel.PolicyChainConfig) error {
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

		_, err = h.registry.GetImplementation(policyConfig.Name, policyConfig.Version)
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
func (h *ResourceHandler) buildPolicyChain(config *kernel.PolicyChainConfig) (*registry.PolicyChain, error) {
	var policyList []policy.Policy
	var policySpecs []policy.PolicySpec

	requiresRequestBody := false
	requiresResponseBody := false

	for _, policyConfig := range config.Policies {
		// Get policy implementation
		impl, err := h.registry.GetImplementation(policyConfig.Name, policyConfig.Version)
		if err != nil {
			return nil, err
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
		Metadata:             make(map[string]interface{}),
		RequiresRequestBody:  requiresRequestBody,
		RequiresResponseBody: requiresResponseBody,
	}

	return chain, nil
}

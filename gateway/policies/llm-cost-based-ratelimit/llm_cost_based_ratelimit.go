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

package llmcostratelimit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	ratelimit "github.com/Tharsanan1/gateway-controllers/policies/advanced-ratelimit"
)

const (
	ResourceTypeLlmProviderTemplate     = "LlmProviderTemplate"
	ResourceTypeProviderTemplateMapping = "ProviderTemplateMapping"
	MetadataKeyProviderName             = "provider_name"
	MetadataKeyDelegate                 = "llm_cost_delegate"
	MetadataKeyCostScaleFactor          = "llm_cost_scale_factor"

	// DefaultCostScaleFactor is the default scaling factor for dollar amounts.
	// Converts dollars to nano-dollars to preserve precision in int64 counters.
	// $1.00 = 1,000,000,000 nano-dollars
	// Can be overridden via systemParameters.costScaleFactor
	DefaultCostScaleFactor = 1_000_000_000
)

// delegateEntry holds a delegate and its cache key for atomic storage
type delegateEntry struct {
	cacheKey string
	delegate policy.Policy
}

// LLMCostRateLimitPolicy delegates LLM cost-based rate limiting to advanced-ratelimit
// by dynamically resolving cost extraction paths from provider templates and applying
// user-defined monetary budgets.
type LLMCostRateLimitPolicy struct {
	metadata  policy.PolicyMetadata
	delegates sync.Map // map[string]*delegateEntry (providerName -> delegate entry)
}

// GetPolicy creates and initializes the LLM cost-based rate limit policy.
func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return &LLMCostRateLimitPolicy{
		metadata: metadata,
	}, nil
}

// Mode returns the processing mode for this policy.
func (p *LLMCostRateLimitPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// OnRequest processes the request phase by delegating to a provider-specific ratelimit instance.
func (p *LLMCostRateLimitPolicy) OnRequest(
	ctx *policy.RequestContext,
	params map[string]interface{},
) policy.RequestAction {
	slog.Debug("OnRequest: processing LLM cost-based rate limit",
		"route", p.metadata.RouteName,
		"params", params)

	providerName, ok := ctx.SharedContext.Metadata[MetadataKeyProviderName].(string)
	if !ok || providerName == "" {
		slog.Debug("OnRequest: provider name not found in metadata; skipping LLM cost rate limit",
			"route", p.metadata.RouteName)
		return nil
	}

	slog.Debug("OnRequest: resolved provider",
		"route", p.metadata.RouteName,
		"provider", providerName)

	delegate, err := p.resolveDelegate(providerName, params)
	if err != nil {
		slog.Warn("OnRequest: failed to resolve rate limit delegate for provider",
			"route", p.metadata.RouteName,
			"provider", providerName,
			"error", err)
		return nil
	}

	if delegate == nil {
		slog.Debug("OnRequest: no delegate available for provider; skipping",
			"route", p.metadata.RouteName,
			"provider", providerName)
		return nil
	}

	// Pin the delegate to the request context for use in OnResponse
	ctx.SharedContext.Metadata[MetadataKeyDelegate] = delegate

	// Store the cost scale factor for use in OnResponse header transformation
	costScaleFactor := extractCostScaleFactor(params)
	ctx.SharedContext.Metadata[MetadataKeyCostScaleFactor] = costScaleFactor

	slog.Debug("OnRequest: delegating to advanced-ratelimit",
		"route", p.metadata.RouteName,
		"provider", providerName,
		"costScaleFactor", costScaleFactor)

	return delegate.OnRequest(ctx, params)
}

// OnResponse processes the response phase by delegating to the same provider-specific instance.
// Uses the delegate pinned during OnRequest to ensure consistency even if template changes.
// After delegation, it adds custom headers that show cost values in dollars.
func (p *LLMCostRateLimitPolicy) OnResponse(
	ctx *policy.ResponseContext,
	params map[string]interface{},
) policy.ResponseAction {
	slog.Debug("OnResponse: processing LLM cost-based rate limit",
		"route", p.metadata.RouteName)

	var delegateAction policy.ResponseAction

	// First, try to use the delegate pinned during OnRequest (ensures consistency)
	if delegate, ok := ctx.SharedContext.Metadata[MetadataKeyDelegate].(policy.Policy); ok {
		slog.Debug("OnResponse: using pinned delegate from request phase",
			"route", p.metadata.RouteName)
		delegateAction = delegate.OnResponse(ctx, params)
	} else {
		// Fallback: look up by provider name (for cases where OnRequest didn't run)
		providerName, ok := ctx.SharedContext.Metadata[MetadataKeyProviderName].(string)
		if !ok || providerName == "" {
			slog.Debug("OnResponse: provider name not found in metadata; skipping",
				"route", p.metadata.RouteName)
			return nil
		}

		slog.Debug("OnResponse: looking up delegate by provider (fallback)",
			"route", p.metadata.RouteName,
			"provider", providerName)

		if entry, ok := p.delegates.Load(providerName); ok {
			if de, ok := entry.(*delegateEntry); ok && de.delegate != nil {
				slog.Debug("OnResponse: delegating to advanced-ratelimit",
					"route", p.metadata.RouteName,
					"provider", providerName)
				delegateAction = de.delegate.OnResponse(ctx, params)
			}
		}

		if delegateAction == nil {
			slog.Debug("OnResponse: no delegate found for provider",
				"route", p.metadata.RouteName,
				"provider", providerName)
			return nil
		}
	}

	// Retrieve the cost scale factor (stored during OnRequest, or use default)
	costScaleFactor := DefaultCostScaleFactor
	if scaleFactor, ok := ctx.SharedContext.Metadata[MetadataKeyCostScaleFactor].(int); ok && scaleFactor > 0 {
		costScaleFactor = scaleFactor
	}

	// Add custom dollar-denominated headers by transforming the delegate's response
	return p.addDollarHeaders(delegateAction, costScaleFactor)
}

// addDollarHeaders transforms the delegate's response action to include
// human-readable dollar-denominated headers alongside the scaled values.
func (p *LLMCostRateLimitPolicy) addDollarHeaders(action policy.ResponseAction, costScaleFactor int) policy.ResponseAction {
	if action == nil {
		return nil
	}

	// Only handle UpstreamResponseModifications
	modifications, ok := action.(policy.UpstreamResponseModifications)
	if !ok {
		return action
	}

	if modifications.SetHeaders == nil {
		return action
	}

	// Create a copy of headers to avoid modifying the original
	newHeaders := make(map[string]string, len(modifications.SetHeaders)+4)
	for k, v := range modifications.SetHeaders {
		newHeaders[k] = v
	}

	// Convert scaled headers to dollar headers
	// Look for both IETF (ratelimit-*) and legacy (x-ratelimit-*) headers
	addScaledHeader(newHeaders, "ratelimit-limit", "x-ratelimit-cost-limit-dollars", costScaleFactor)
	addScaledHeader(newHeaders, "ratelimit-remaining", "x-ratelimit-cost-remaining-dollars", costScaleFactor)
	addScaledHeader(newHeaders, "x-ratelimit-limit", "x-ratelimit-cost-limit-dollars", costScaleFactor)
	addScaledHeader(newHeaders, "x-ratelimit-remaining", "x-ratelimit-cost-remaining-dollars", costScaleFactor)

	modifications.SetHeaders = newHeaders
	return modifications
}

// addScaledHeader reads a scaled value from sourceKey and adds a dollar-formatted
// header at targetKey. If targetKey already exists, it won't be overwritten.
func addScaledHeader(headers map[string]string, sourceKey, targetKey string, costScaleFactor int) {
	// Skip if target already exists
	if _, exists := headers[targetKey]; exists {
		return
	}

	sourceValue, ok := headers[sourceKey]
	if !ok || sourceValue == "" {
		return
	}

	// Parse the scaled value
	scaledValue, err := strconv.ParseInt(sourceValue, 10, 64)
	if err != nil {
		slog.Debug("addScaledHeader: failed to parse source value",
			"sourceKey", sourceKey,
			"sourceValue", sourceValue,
			"error", err)
		return
	}

	// Convert to dollars using the configured scale factor
	dollars := float64(scaledValue) / float64(costScaleFactor)
	headers[targetKey] = fmt.Sprintf("%.6f", dollars)

	slog.Debug("addScaledHeader: added dollar header",
		"sourceKey", sourceKey,
		"scaledValue", scaledValue,
		"costScaleFactor", costScaleFactor,
		"targetKey", targetKey,
		"dollars", dollars)
}

// resolveDelegate ensures an advanced-ratelimit instance exists for the given provider.
// This method is thread-safe using sync.Map with atomic delegateEntry storage.
func (p *LLMCostRateLimitPolicy) resolveDelegate(providerName string, params map[string]interface{}) (policy.Policy, error) {
	slog.Debug("resolveDelegate: checking for existing delegate",
		"route", p.metadata.RouteName,
		"provider", providerName)

	// Get the template to compute the cache key
	store := policy.GetLazyResourceStoreInstance()

	// 1. Get Provider-to-Template Mapping
	mappingResource, err := store.GetResourceByIDAndType(providerName, ResourceTypeProviderTemplateMapping)
	if err != nil {
		slog.Error("resolveDelegate: failed to get provider template mapping",
			"route", p.metadata.RouteName,
			"provider", providerName,
			"error", err)
		return nil, err
	}

	if mappingResource == nil {
		slog.Warn("resolveDelegate: provider template mapping not found",
			"route", p.metadata.RouteName,
			"provider", providerName)
		return nil, nil
	}

	templateHandle, ok := mappingResource.Resource["template_handle"].(string)
	if !ok || templateHandle == "" {
		slog.Warn("resolveDelegate: template_handle not found or empty in mapping",
			"route", p.metadata.RouteName,
			"provider", providerName)
		return nil, nil
	}

	// 2. Get the Actual Template
	templateResource, err := store.GetResourceByIDAndType(templateHandle, ResourceTypeLlmProviderTemplate)
	if err != nil {
		slog.Error("resolveDelegate: failed to get LLM provider template",
			"route", p.metadata.RouteName,
			"provider", providerName,
			"templateHandle", templateHandle,
			"error", err)
		return nil, err
	}

	if templateResource == nil {
		slog.Warn("resolveDelegate: LLM provider template not found",
			"route", p.metadata.RouteName,
			"provider", providerName,
			"templateHandle", templateHandle)
		return nil, nil
	}

	// 3. Compute a hash of the template to use in the cache key
	templateHash := computeResourceHash(templateResource.Resource)
	cacheKey := providerName + ":" + templateHash

	slog.Debug("resolveDelegate: computed cache key",
		"route", p.metadata.RouteName,
		"provider", providerName,
		"templateHandle", templateHandle,
		"templateHash", truncateHash(templateHash),
		"cacheKey", cacheKey)

	// Fast path: check if delegate already exists for this provider with same template
	if existing, ok := p.delegates.Load(providerName); ok {
		if entry, ok := existing.(*delegateEntry); ok && entry.cacheKey == cacheKey {
			slog.Debug("resolveDelegate: found existing delegate for current template (fast path)",
				"route", p.metadata.RouteName,
				"provider", providerName,
				"templateHash", truncateHash(templateHash))
			return entry.delegate, nil
		}
		// Template changed - continue to create new delegate
		if entry, ok := existing.(*delegateEntry); ok {
			slog.Debug("resolveDelegate: template changed, creating new delegate",
				"route", p.metadata.RouteName,
				"provider", providerName,
				"oldCacheKey", truncateHash(entry.cacheKey),
				"newCacheKey", truncateHash(cacheKey))
		}
	}

	slog.Debug("resolveDelegate: creating new delegate (slow path)",
		"route", p.metadata.RouteName,
		"provider", providerName,
		"templateHash", truncateHash(templateHash))

	// Slow path: create the delegate
	delegate, err := p.createDelegateWithTemplate(providerName, params, templateResource.Resource)
	if err != nil {
		slog.Error("resolveDelegate: failed to create delegate",
			"route", p.metadata.RouteName,
			"provider", providerName,
			"error", err)
		return nil, err
	}

	// Store the delegate entry atomically (single map, single entry)
	entry := &delegateEntry{
		cacheKey: cacheKey,
		delegate: delegate,
	}
	p.delegates.Store(providerName, entry)

	slog.Debug("resolveDelegate: successfully created and stored new delegate",
		"route", p.metadata.RouteName,
		"provider", providerName,
		"templateHash", truncateHash(templateHash))

	return delegate, nil
}

// truncateHash safely truncates a hash string for logging purposes
func truncateHash(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

// createDelegateWithTemplate creates a delegate using the provided template (already fetched).
// This avoids double-fetching the template when called from resolveDelegate.
func (p *LLMCostRateLimitPolicy) createDelegateWithTemplate(providerName string, params map[string]interface{}, template map[string]interface{}) (policy.Policy, error) {
	// Transform LLM cost limits into advanced-ratelimit parameters
	rlParams := transformToRatelimitParams(params, template)

	slog.Debug("createDelegateWithTemplate: parameters transformed",
		"route", p.metadata.RouteName,
		"provider", providerName,
		"quotasCount", len(rlParams["quotas"].([]interface{})))

	// Create the delegate instance
	delegate, err := ratelimit.GetPolicy(p.metadata, rlParams)
	if err != nil {
		slog.Error("createDelegateWithTemplate: failed to create advanced-ratelimit policy",
			"route", p.metadata.RouteName,
			"provider", providerName,
			"error", err)
		return nil, err
	}

	slog.Debug("createDelegateWithTemplate: successfully created delegate",
		"route", p.metadata.RouteName,
		"provider", providerName)

	return delegate, nil
}

// computeResourceHash computes a SHA256 hash of the resource map for cache key generation.
func computeResourceHash(resource map[string]interface{}) string {
	// Serialize the resource to JSON
	data, err := json.Marshal(resource)
	if err != nil {
		// If marshaling fails, return a random string to force a cache miss
		return "error-" + randomString(8)
	}

	// Compute SHA256 hash
	hash := sha256.Sum256(data)
	// Return first 16 characters of hex encoding (sufficient for collision resistance)
	return hex.EncodeToString(hash[:])[:16]
}

// randomString generates a random string of the given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// transformToRatelimitParams converts LLM cost-specific parameters to the advanced-ratelimit structure.
// Costs are configured as "cost per N tokens" (default N=1,000,000) and converted to per-token costs
// for the advanced-ratelimit engine by dividing by N.
//
// Budget limits (in dollars) are scaled by costScaleFactor (configurable via systemParameters.costScaleFactor)
// to preserve precision when the underlying rate limiter uses int64 counters.
// The cost multipliers are also scaled so that the final deduction is in the scaled unit.
func transformToRatelimitParams(params map[string]interface{}, template map[string]interface{}) map[string]interface{} {
	slog.Debug("transformToRatelimitParams: starting parameter transformation",
		"params", params,
		"template", template)

	// Get the budget limits from user parameters
	budgetLimits := params["budgetLimits"]
	if budgetLimits == nil {
		slog.Debug("transformToRatelimitParams: no budgetLimits found")
		return map[string]interface{}{"quotas": []interface{}{}}
	}

	// Get token costs from system parameters (adjusted by costPerNTokens)
	tokenCosts, costPerNTokens := extractTokenCosts(params)

	// Get the cost scale factor from system parameters
	costScaleFactor := extractCostScaleFactor(params)

	// Validate costPerNTokens to prevent division by zero
	if costPerNTokens < 1 {
		slog.Warn("transformToRatelimitParams: costPerNTokens is less than 1, using default 1000000",
			"provided", costPerNTokens)
		costPerNTokens = 1000000
	}

	slog.Debug("transformToRatelimitParams: extracted token costs",
		"costPerNTokens", costPerNTokens,
		"costScaleFactor", costScaleFactor,
		"hasPromptCost", tokenCosts["promptTokenCost"] != nil,
		"hasCompletionCost", tokenCosts["completionTokenCost"] != nil,
		"hasTotalCost", tokenCosts["totalTokenCost"] != nil)

	// Build a single quota with multiple limits for different time windows
	// Scale the dollar amounts using the configured scale factor for precision
	var limits []interface{}
	budgetItems, ok := budgetLimits.([]interface{})
	if !ok {
		slog.Debug("transformToRatelimitParams: budgetLimits is not an array")
		return map[string]interface{}{"quotas": []interface{}{}}
	}

	for _, item := range budgetItems {
		if m, ok := item.(map[string]interface{}); ok {
			// Scale the amount from dollars to scaled units (e.g., micro-dollars)
			// Use float64 since the advanced-ratelimit policy expects float64 (from JSON parsing)
			var scaledAmount float64
			switch v := m["amount"].(type) {
			case float64:
				scaledAmount = v * float64(costScaleFactor)
			case int:
				scaledAmount = float64(v) * float64(costScaleFactor)
			case int64:
				scaledAmount = float64(v) * float64(costScaleFactor)
			default:
				slog.Warn("transformToRatelimitParams: unsupported amount type",
					"type", fmt.Sprintf("%T", m["amount"]))
				continue
			}

			limit := map[string]interface{}{
				"limit":    scaledAmount,
				"duration": m["duration"],
			}
			limits = append(limits, limit)

			slog.Debug("transformToRatelimitParams: scaled limit",
				"originalAmount", m["amount"],
				"scaledAmount", scaledAmount,
				"costScaleFactor", costScaleFactor,
				"duration", m["duration"])
		}
	}

	if len(limits) == 0 {
		slog.Debug("transformToRatelimitParams: no valid limits found")
		return map[string]interface{}{"quotas": []interface{}{}}
	}

	// Build cost extraction sources with multipliers for each token type.
	// The multiplier converts token count to scaled units:
	//   scaledCost = tokens × (costPerNTokens / N) × costScaleFactor
	// where costPerNTokens is the dollar cost for N tokens.
	var sources []interface{}

	// Add prompt token cost source with multiplier (scaled)
	if promptCost, ok := tokenCosts["promptTokenCost"].(float64); ok && promptCost > 0 {
		// costPerToken in dollars, then scale to configured unit
		costPerTokenDollars := promptCost / float64(costPerNTokens)
		costPerTokenScaled := costPerTokenDollars * float64(costScaleFactor)
		if source := buildCostSource(template, "promptTokens", costPerTokenScaled); source != nil {
			sources = append(sources, source)
			slog.Debug("transformToRatelimitParams: added prompt cost source",
				"promptCost", promptCost,
				"costPerNTokens", costPerNTokens,
				"costPerTokenDollars", costPerTokenDollars,
				"costPerTokenScaled", costPerTokenScaled,
				"costScaleFactor", costScaleFactor)
		}
	}

	// Add completion token cost source with multiplier (scaled)
	if completionCost, ok := tokenCosts["completionTokenCost"].(float64); ok && completionCost > 0 {
		costPerTokenDollars := completionCost / float64(costPerNTokens)
		costPerTokenScaled := costPerTokenDollars * float64(costScaleFactor)
		if source := buildCostSource(template, "completionTokens", costPerTokenScaled); source != nil {
			sources = append(sources, source)
			slog.Debug("transformToRatelimitParams: added completion cost source",
				"completionCost", completionCost,
				"costPerNTokens", costPerNTokens,
				"costPerTokenDollars", costPerTokenDollars,
				"costPerTokenScaled", costPerTokenScaled,
				"costScaleFactor", costScaleFactor)
		}
	}

	// Add total token cost source with multiplier (if individual costs not specified)
	if len(sources) == 0 {
		if totalCost, ok := tokenCosts["totalTokenCost"].(float64); ok && totalCost > 0 {
			costPerTokenDollars := totalCost / float64(costPerNTokens)
			costPerTokenScaled := costPerTokenDollars * float64(costScaleFactor)
			if source := buildCostSource(template, "totalTokens", costPerTokenScaled); source != nil {
				sources = append(sources, source)
				slog.Debug("transformToRatelimitParams: added total cost source",
					"totalCost", totalCost,
					"costPerNTokens", costPerNTokens,
					"costPerTokenDollars", costPerTokenDollars,
					"costPerTokenScaled", costPerTokenScaled,
					"costScaleFactor", costScaleFactor)
			}
		}
	}

	// Build the quota
	quota := map[string]interface{}{
		"name":   "llm_cost_quota",
		"limits": limits,
		"keyExtraction": []interface{}{
			map[string]interface{}{"type": "routename"},
		},
	}

	// Add cost extraction configuration if we have sources
	if len(sources) > 0 {
		quota["costExtraction"] = map[string]interface{}{
			"enabled": true,
			"sources": sources,
			"default": 0, // Default to 0 cost if extraction fails
		}
	}

	quotas := []interface{}{quota}

	rlParams := map[string]interface{}{
		"quotas": quotas,
	}

	// Copy through system parameters
	for _, key := range []string{"algorithm", "backend", "redis", "memory"} {
		if val, ok := params[key]; ok {
			rlParams[key] = val
		}
	}

	slog.Debug("transformToRatelimitParams: completed transformation",
		"quotasCount", len(quotas),
		"hasAlgorithm", rlParams["algorithm"] != nil,
		"hasBackend", rlParams["backend"] != nil,
		"quotas", quotas)

	return rlParams
}

// extractCostScaleFactor extracts the cost scale factor from system parameters.
// The scale factor determines how dollar amounts are scaled for precision in int64 counters.
// Default: 1,000,000 (micro-dollars)
func extractCostScaleFactor(params map[string]interface{}) int {
	// Check for system parameters at root level (injected by gateway)
	if systemParams, ok := params["systemParameters"].(map[string]interface{}); ok {
		if scaleFactor := extractIntValue(systemParams["costScaleFactor"], 0); scaleFactor > 0 {
			return scaleFactor
		}
	}

	// Also check at root level for directly embedded system parameters
	if scaleFactor := extractIntValue(params["costScaleFactor"], 0); scaleFactor > 0 {
		return scaleFactor
	}

	return DefaultCostScaleFactor
}

// extractTokenCosts extracts token cost configuration and costPerNTokens from system parameters.
// System parameters define the cost per N tokens set by the gateway admin.
// Returns the token costs map and the costPerNTokens value (default: 1000000).
func extractTokenCosts(params map[string]interface{}) (map[string]interface{}, int) {
	costs := make(map[string]interface{})
	costPerNTokens := 1000000 // Default: cost applies to 1,000,000 tokens

	// Check for system parameters at root level (injected by gateway)
	if systemParams, ok := params["systemParameters"].(map[string]interface{}); ok {
		// Extract costPerNTokens first
		costPerNTokens = extractIntValue(systemParams["costPerNTokens"], costPerNTokens)

		if promptCost, ok := getFloatValue(systemParams["promptTokenCost"]); ok {
			costs["promptTokenCost"] = promptCost
		}
		if completionCost, ok := getFloatValue(systemParams["completionTokenCost"]); ok {
			costs["completionTokenCost"] = completionCost
		}
		if totalCost, ok := getFloatValue(systemParams["totalTokenCost"]); ok {
			costs["totalTokenCost"] = totalCost
		}
		return costs, costPerNTokens
	}

	// Also check at root level for directly embedded system parameters
	costPerNTokens = extractIntValue(params["costPerNTokens"], costPerNTokens)

	if promptCost, ok := getFloatValue(params["promptTokenCost"]); ok {
		costs["promptTokenCost"] = promptCost
	}
	if completionCost, ok := getFloatValue(params["completionTokenCost"]); ok {
		costs["completionTokenCost"] = completionCost
	}
	if totalCost, ok := getFloatValue(params["totalTokenCost"]); ok {
		costs["totalTokenCost"] = totalCost
	}

	return costs, costPerNTokens
}

// extractIntValue extracts an int from various numeric types with a default value
func extractIntValue(v interface{}, defaultValue int) int {
	if v == nil {
		return defaultValue
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case int32:
		return int(val)
	case float64:
		return int(val)
	case float32:
		return int(val)
	}
	return defaultValue
}

// getFloatValue extracts a float64 from various numeric types
func getFloatValue(v interface{}) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	}
	return 0, false
}

// buildCostSource builds a cost extraction source from template configuration.
// The multiplier represents the cost per token.
func buildCostSource(template map[string]interface{}, templateKey string, costPerToken float64) map[string]interface{} {
	if template == nil {
		return nil
	}

	// The template structure may have spec directly or nested in configuration
	spec := getTemplateSpec(template)
	if spec == nil {
		return nil
	}

	usage, ok := spec[templateKey].(map[string]interface{})
	if !ok {
		return nil
	}

	path, ok := usage["identifier"].(string)
	if !ok || path == "" {
		return nil
	}

	location, _ := usage["location"].(string)

	// Map location to source type
	switch location {
	case "header":
		return map[string]interface{}{
			"type":       "response_header",
			"key":        path,
			"multiplier": costPerToken,
		}
	case "metadata":
		return map[string]interface{}{
			"type":       "response_metadata",
			"key":        path,
			"multiplier": costPerToken,
		}
	case "payload", "":
		// Default to response body with jsonPath
		return map[string]interface{}{
			"type":       "response_body",
			"jsonPath":   path,
			"multiplier": costPerToken,
		}
	default:
		// For any other location, assume payload/response_body
		return map[string]interface{}{
			"type":       "response_body",
			"jsonPath":   path,
			"multiplier": costPerToken,
		}
	}
}

// getTemplateSpec extracts the spec from a template which may be nested
func getTemplateSpec(template map[string]interface{}) map[string]interface{} {
	if template == nil {
		return nil
	}

	// Try direct spec
	if spec, ok := template["spec"].(map[string]interface{}); ok {
		return spec
	}

	// Try nested in configuration
	if config, ok := template["configuration"].(map[string]interface{}); ok {
		if spec, ok := config["spec"].(map[string]interface{}); ok {
			return spec
		}
	}

	return nil
}

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

package tokenbasedratelimit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	ratelimit "github.com/wso2/gateway-controllers/policies/advanced-ratelimit"
)

const (
	ResourceTypeLlmProviderTemplate     = "LlmProviderTemplate"
	ResourceTypeProviderTemplateMapping = "ProviderTemplateMapping"
	MetadataKeyProviderName             = "provider_name"
)

// TokenBasedRateLimitPolicy delegates LLM token-based rate limiting to advanced-ratelimit
// by dynamically resolving cost extraction paths from provider templates.
type TokenBasedRateLimitPolicy struct {
	metadata          policy.PolicyMetadata
	delegates         sync.Map // map[string]policy.Policy (providerName -> advanced-ratelimit instance)
	delegateCacheKeys sync.Map // map[string]string (providerName -> cacheKey for template change detection)
	sf                singleflight.Group // prevents duplicate delegate creation
}

// GetPolicy creates and initializes the token-based rate limit policy.
func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return &TokenBasedRateLimitPolicy{
		metadata: metadata,
	}, nil
}

// Mode returns the processing mode for this policy.
func (p *TokenBasedRateLimitPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// OnRequest processes the request phase by delegating to a provider-specific ratelimit instance.
func (p *TokenBasedRateLimitPolicy) OnRequest(
	ctx *policy.RequestContext,
	params map[string]interface{},
) policy.RequestAction {
	slog.Debug("OnRequest: processing token-based rate limit",
		"route", p.metadata.RouteName,
		"params", params)

	providerName, ok := ctx.SharedContext.Metadata[MetadataKeyProviderName].(string)
	if !ok || providerName == "" {
		slog.Debug("OnRequest: provider name not found in metadata; skipping token-based rate limit",
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
		slog.Warn("OnRequest: delegate is nil for provider",
			"route", p.metadata.RouteName,
			"provider", providerName)
		return nil
	}

	slog.Debug("OnRequest: delegating to advanced-ratelimit",
		"route", p.metadata.RouteName,
		"provider", providerName)

	return delegate.OnRequest(ctx, params)
}

// OnResponse processes the response phase by delegating to the same provider-specific instance.
func (p *TokenBasedRateLimitPolicy) OnResponse(
	ctx *policy.ResponseContext,
	params map[string]interface{},
) policy.ResponseAction {
	slog.Debug("OnResponse: processing token-based rate limit",
		"route", p.metadata.RouteName)

	providerName, ok := ctx.SharedContext.Metadata[MetadataKeyProviderName].(string)
	if !ok || providerName == "" {
		slog.Debug("OnResponse: provider name not found in metadata; skipping",
			"route", p.metadata.RouteName)
		return nil
	}

	slog.Debug("OnResponse: looking up delegate",
		"route", p.metadata.RouteName,
		"provider", providerName)

	if delegate, ok := p.delegates.Load(providerName); ok {
		slog.Debug("OnResponse: delegating to advanced-ratelimit",
			"route", p.metadata.RouteName,
			"provider", providerName)
		return delegate.(policy.Policy).OnResponse(ctx, params)
	}

	slog.Debug("OnResponse: no delegate found for provider",
		"route", p.metadata.RouteName,
		"provider", providerName)

	return nil
}

// resolveDelegate ensures an advanced-ratelimit instance exists for the given provider.
// This method is thread-safe and uses singleflight to prevent duplicate delegate creation
// when multiple goroutines attempt to create a delegate for the same provider/template
// simultaneously. Only one goroutine performs the expensive creation, and others wait
// for the result. The delegate is cached with a key that includes a hash of the template,
// so when the template changes, a new delegate is created automatically.
func (p *TokenBasedRateLimitPolicy) resolveDelegate(providerName string, params map[string]interface{}) (policy.Policy, error) {
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
		slog.Error("resolveDelegate: provider template mapping not found",
			"route", p.metadata.RouteName,
			"provider", providerName)
		return nil, nil
	}

	templateHandle, ok := mappingResource.Resource["template_handle"].(string)
	if !ok || templateHandle == "" {
		slog.Error("resolveDelegate: template_handle not found or empty in mapping",
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
		slog.Error("resolveDelegate: LLM provider template not found",
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
		"templateHash", templateHash[:8],
		"cacheKey", cacheKey)

	// Fast path: check if delegate already exists for this provider
	// We check using providerName as the key since that's what OnResponse uses
	if existingDelegate, ok := p.delegates.Load(providerName); ok {
		// Check if the existing delegate is for the current template version
		// by comparing the stored cacheKey
		if storedCacheKey, hasKey := p.delegateCacheKeys.Load(providerName); hasKey {
			if storedCacheKey.(string) == cacheKey {
				slog.Debug("resolveDelegate: found existing delegate for current template (fast path)",
					"route", p.metadata.RouteName,
					"provider", providerName,
					"templateHash", templateHash[:8])
				return existingDelegate.(policy.Policy), nil
			}
			// Template changed - continue to create new delegate
			slog.Debug("resolveDelegate: template changed, creating new delegate",
				"route", p.metadata.RouteName,
				"provider", providerName,
				"oldTemplateHash", storedCacheKey.(string)[:8],
				"newTemplateHash", templateHash[:8])
		}
	}

	// Slow path: use singleflight to prevent duplicate delegate creation
	// Only one goroutine will execute the creation for a given cacheKey
	result, err, _ := p.sf.Do(cacheKey, func() (interface{}, error) {
		// Double-check: another goroutine may have created it while we were waiting
		if existingDelegate, ok := p.delegates.Load(providerName); ok {
			if storedCacheKey, hasKey := p.delegateCacheKeys.Load(providerName); hasKey {
				if storedCacheKey.(string) == cacheKey {
					slog.Debug("resolveDelegate: delegate created by another goroutine",
						"route", p.metadata.RouteName,
						"provider", providerName,
						"templateHash", templateHash[:8])
					return existingDelegate.(policy.Policy), nil
				}
			}
		}

		slog.Debug("resolveDelegate: creating new delegate (singleflight)",
			"route", p.metadata.RouteName,
			"provider", providerName,
			"templateHash", templateHash[:8])

		// Create the delegate (expensive operation)
		delegate, err := p.createDelegateWithTemplate(providerName, params, templateResource.Resource)
		if err != nil {
			slog.Error("resolveDelegate: failed to create delegate",
				"route", p.metadata.RouteName,
				"provider", providerName,
				"error", err)
			return nil, err
		}

		// Store the delegate with providerName as key (for OnResponse lookup)
		// and store the cacheKey separately for template change detection
		p.delegates.Store(providerName, delegate)
		p.delegateCacheKeys.Store(providerName, cacheKey)

		slog.Debug("resolveDelegate: successfully created and stored new delegate",
			"route", p.metadata.RouteName,
			"provider", providerName,
			"templateHash", templateHash[:8])

		return delegate, nil
	})

	if err != nil {
		return nil, err
	}
	return result.(policy.Policy), nil
}

// createDelegate creates a new advanced-ratelimit delegate for the given provider.
// This involves fetching resources from the store and transforming parameters.
func (p *TokenBasedRateLimitPolicy) createDelegate(providerName string, params map[string]interface{}) (policy.Policy, error) {
	slog.Debug("createDelegate: starting delegate creation",
		"route", p.metadata.RouteName,
		"provider", providerName)

	store := policy.GetLazyResourceStoreInstance()

	// 1. Get Provider-to-Template Mapping
	slog.Debug("createDelegate: fetching provider template mapping",
		"route", p.metadata.RouteName,
		"provider", providerName,
		"resourceType", ResourceTypeProviderTemplateMapping)

	mappingResource, err := store.GetResourceByIDAndType(providerName, ResourceTypeProviderTemplateMapping)
	if err != nil {
		slog.Error("createDelegate: failed to get provider template mapping",
			"route", p.metadata.RouteName,
			"provider", providerName,
			"error", err)
		return nil, err
	}

	if mappingResource == nil {
		slog.Error("createDelegate: provider template mapping not found",
			"route", p.metadata.RouteName,
			"provider", providerName)
		return nil, nil
	}

	templateHandle, ok := mappingResource.Resource["template_handle"].(string)
	if !ok || templateHandle == "" {
		slog.Error("createDelegate: template_handle not found or empty in mapping",
			"route", p.metadata.RouteName,
			"provider", providerName,
			"hasTemplateHandle", ok)
		return nil, nil
	}

	// 2. Get the Actual Template
	templateResource, err := store.GetResourceByIDAndType(templateHandle, ResourceTypeLlmProviderTemplate)
	if err != nil {
		slog.Error("createDelegate: failed to get LLM provider template",
			"route", p.metadata.RouteName,
			"provider", providerName,
			"templateHandle", templateHandle,
			"error", err)
		return nil, err
	}

	if templateResource == nil {
		slog.Error("createDelegate: LLM provider template not found",
			"route", p.metadata.RouteName,
			"provider", providerName,
			"templateHandle", templateHandle)
		return nil, nil
	}

	return p.createDelegateWithTemplate(providerName, params, templateResource.Resource)
}

// createDelegateWithTemplate creates a delegate using the provided template (already fetched).
// This avoids double-fetching the template when called from resolveDelegate.
func (p *TokenBasedRateLimitPolicy) createDelegateWithTemplate(providerName string, params map[string]interface{}, template map[string]interface{}) (policy.Policy, error) {
	// Transform LLM limits into advanced-ratelimit parameters
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
// This allows detecting when the template has changed.
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

// transformToRatelimitParams converts LLM-specific parameters to the advanced-ratelimit structure.
func transformToRatelimitParams(params map[string]interface{}, template map[string]interface{}) map[string]interface{} {
	slog.Debug("transformToRatelimitParams: starting parameter transformation",
		"params", params,
		"template", template)

	var quotas []interface{}

	addQuota := func(name string, limitsKey string, templateKey string) {
		limits := params[limitsKey]
		if limits == nil {
			slog.Debug("addQuota: no limits found, skipping",
				"name", name,
				"limitsKey", limitsKey)
			return
		}

		quota := map[string]interface{}{
			"name":   name,
			"limits": convertLimits(limits),
			"keyExtraction": []interface{}{
				map[string]interface{}{"type": "routename"},
			},
		}

		if template != nil {
			// The template structure has spec directly: template["spec"]
			if spec, ok := template["spec"].(map[string]interface{}); ok {
				if usage, ok := spec[templateKey].(map[string]interface{}); ok {
					if path, ok := usage["identifier"].(string); ok && path != "" {
						// Map template location to cost extraction type
						location, _ := usage["location"].(string)
						sourceType := "response_body" // default
						sourceConfig := map[string]interface{}{
							"type": sourceType,
						}

						switch location {
						case "header":
							sourceType = "request_header"
							sourceConfig["type"] = sourceType
							sourceConfig["key"] = path
						case "metadata":
							sourceType = "metadata"
							sourceConfig["type"] = sourceType
							sourceConfig["key"] = path
						case "payload":
							// payload location uses response_body type with jsonPath
							sourceConfig["jsonPath"] = path
						default:
							// For any other location, assume payload/response_body
							sourceConfig["jsonPath"] = path
						}

						slog.Debug("addQuota: configured cost extraction",
							"name", name,
							"location", location,
							"sourceType", sourceType,
							"path", path)

						quota["costExtraction"] = map[string]interface{}{
							"enabled": true,
							"sources": []interface{}{sourceConfig},
						}
					}
				}
			}
		}
		quotas = append(quotas, quota)
	}

	addQuota("prompt_tokens", "promptTokenLimits", "promptTokens")
	addQuota("completion_tokens", "completionTokenLimits", "completionTokens")
	addQuota("total_tokens", "totalTokenLimits", "totalTokens")

	rlParams := map[string]interface{}{
		"quotas": quotas,
	}

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

func convertLimits(rawLimits interface{}) []interface{} {
	items, ok := rawLimits.([]interface{})
	if !ok {
		return nil
	}
	var converted []interface{}
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			converted = append(converted, map[string]interface{}{
				"limit":    m["count"],
				"duration": m["duration"],
			})
		}
	}
	return converted
}

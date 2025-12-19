/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package semanticcache

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
	embeddingproviders "github.com/wso2/api-platform/sdk/utils/embeddingproviders"
	vectordbproviders "github.com/wso2/api-platform/sdk/utils/vectordbproviders"
)

const (
	// MetadataKeyEmbedding is the key used to store embedding in metadata between request and response phases
	MetadataKeyEmbedding = "semantic_cache_embedding"
	// MetadataKeyAPIID is the key used to store API ID in metadata
	MetadataKeyAPIID = "semantic_cache_api_id"
)

var (
	// Map of policy instance hash to its providers
	embeddingProviders   = make(map[string]embeddingproviders.EmbeddingProvider)
	vectorStoreProviders = make(map[string]vectordbproviders.VectorDBProvider)

	// Mutex to protect access to global providers
	providerMutex sync.RWMutex

	// Map of policy instance hash to its configurations (to detect changes)
	embeddingConfigs   = make(map[string]embeddingproviders.EmbeddingProviderConfig)
	vectorStoreConfigs = make(map[string]vectordbproviders.VectorDBProviderConfig)

	// Map to track if index has been created for each policy instance
	indexCreated = make(map[string]bool)
	indexMutex   sync.RWMutex
)

// SemanticCachePolicy implements semantic caching for LLM responses
type SemanticCachePolicy struct {
	embeddingConfig     embeddingproviders.EmbeddingProviderConfig
	vectorStoreConfig   vectordbproviders.VectorDBProviderConfig
	embeddingProvider   embeddingproviders.EmbeddingProvider
	vectorStoreProvider vectordbproviders.VectorDBProvider
	jsonPath            string
	threshold           float64
	policyInstanceHash  string
}

// GetPolicy creates a new instance of the semantic cache policy
func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	p := &SemanticCachePolicy{}

	// Parse and validate parameters
	if err := parseParams(params, p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// Generate unique policy instance hash based on route and policy metadata
	p.policyInstanceHash = generatePolicyInstanceHash(metadata)

	// Initialize or reuse embedding provider
	if err := p.initializeEmbeddingProvider(); err != nil {
		return nil, fmt.Errorf("failed to initialize embedding provider: %w", err)
	}

	// Initialize or reuse vector store provider
	if err := p.initializeVectorStoreProvider(); err != nil {
		return nil, fmt.Errorf("failed to initialize vector store provider: %w", err)
	}

	if p.embeddingProvider == nil || p.vectorStoreProvider == nil {
		return nil, fmt.Errorf("failed to initialize providers")
	}

	return p, nil
}

// parseParams parses and validates parameters from the params map
func parseParams(params map[string]interface{}, p *SemanticCachePolicy) error {
	// Required parameters
	embeddingProvider, ok := params["embeddingProvider"].(string)
	if !ok || embeddingProvider == "" {
		return fmt.Errorf("'embeddingProvider' parameter is required")
	}

	vectorStoreProvider, ok := params["vectorStoreProvider"].(string)
	if !ok || vectorStoreProvider == "" {
		return fmt.Errorf("'vectorStoreProvider' parameter is required")
	}

	thresholdRaw, ok := params["threshold"]
	if !ok {
		return fmt.Errorf("'threshold' parameter is required")
	}
	threshold, err := extractFloat64(thresholdRaw)
	if err != nil {
		return fmt.Errorf("'threshold' must be a number: %w", err)
	}
	if threshold < 0.0 || threshold > 1.0 {
		return fmt.Errorf("'threshold' must be between 0.0 and 1.0")
	}

	// Parse embedding provider config
	p.embeddingConfig = embeddingproviders.EmbeddingProviderConfig{
		EmbeddingProvider: embeddingProvider,
	}

	// Required for OPENAI, MISTRAL, AZURE_OPENAI
	if endpoint, ok := params["embeddingEndpoint"].(string); ok && endpoint != "" {
		p.embeddingConfig.EmbeddingEndpoint = endpoint
	} else {
		return fmt.Errorf("'embeddingEndpoint' is required for %s provider", embeddingProvider)
	}

	if model, ok := params["embeddingModel"].(string); ok && model != "" {
		p.embeddingConfig.EmbeddingModel = model
	} else {
		return fmt.Errorf("'embeddingModel' is required for %s provider", embeddingProvider)
	}

	if apiKey, ok := params["apiKey"].(string); ok && apiKey != "" {
		p.embeddingConfig.APIKey = apiKey
	} else {
		return fmt.Errorf("'apiKey' is required for %s provider", embeddingProvider)
	}

	if headerName, ok := params["headerName"].(string); ok && headerName != "" {
		p.embeddingConfig.AuthHeaderName = headerName
	} else {
		p.embeddingConfig.AuthHeaderName = "Authorization"
	}

	// Parse vector store provider config
	p.vectorStoreConfig = vectordbproviders.VectorDBProviderConfig{
		VectorStoreProvider: vectorStoreProvider,
		Threshold:           fmt.Sprintf("%.2f", threshold),
	}

	if dbHost, ok := params["dbHost"].(string); ok && dbHost != "" {
		p.vectorStoreConfig.DBHost = dbHost
	} else {
		return fmt.Errorf("'dbHost' is required")
	}

	if dbPortRaw, ok := params["dbPort"]; ok {
		dbPort, err := extractInt(dbPortRaw)
		if err != nil {
			return fmt.Errorf("'dbPort' must be a number: %w", err)
		}
		p.vectorStoreConfig.DBPort = dbPort
	} else {
		return fmt.Errorf("'dbPort' is required")
	}

	if embeddingDim, ok := params["embeddingDimension"]; ok {
		dim, err := extractInt(embeddingDim)
		if err != nil {
			return fmt.Errorf("'embeddingDimension' must be a number: %w", err)
		}
		p.vectorStoreConfig.EmbeddingDimension = strconv.Itoa(dim)
	} else {
		return fmt.Errorf("'embeddingDimension' is required")
	}

	if username, ok := params["username"].(string); ok {
		p.vectorStoreConfig.Username = username
	}

	if password, ok := params["password"].(string); ok {
		p.vectorStoreConfig.Password = password
	}

	if database, ok := params["database"].(string); ok {
		p.vectorStoreConfig.DatabaseName = database
	}

	if ttlRaw, ok := params["ttl"]; ok {
		ttl, err := extractInt(ttlRaw)
		if err != nil {
			return fmt.Errorf("'ttl' must be a number: %w", err)
		}
		p.vectorStoreConfig.TTL = strconv.Itoa(ttl)
	}

	// Optional JSONPath for extracting text from request body
	if jsonPath, ok := params["jsonPath"].(string); ok {
		p.jsonPath = jsonPath
	}

	p.threshold = threshold

	return nil
}

// extractFloat64 safely extracts a float64 from various types
func extractFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot convert %q to float64: %w", v, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

// extractInt safely extracts an integer from various types
func extractInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		if v != float64(int(v)) {
			return 0, fmt.Errorf("expected an integer but got %v", v)
		}
		return int(v), nil
	case string:
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("cannot convert %q to int: %w", v, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

// generatePolicyInstanceHash generates a unique hash for this policy instance
func generatePolicyInstanceHash(metadata policy.PolicyMetadata) string {
	// Use route name as the unique identifier for this policy instance
	// This ensures each route gets its own provider instances
	return metadata.RouteName
}

// initializeEmbeddingProvider initializes or reuses the embedding provider
func (p *SemanticCachePolicy) initializeEmbeddingProvider() error {
	providerMutex.RLock()
	configChanged := !reflect.DeepEqual(p.embeddingConfig, embeddingConfigs[p.policyInstanceHash])
	existingProvider := embeddingProviders[p.policyInstanceHash]
	providerMutex.RUnlock()

	if !configChanged && existingProvider != nil {
		p.embeddingProvider = existingProvider
		return nil
	}

	providerMutex.Lock()
	defer providerMutex.Unlock()

	// Check again after acquiring lock
	if !reflect.DeepEqual(p.embeddingConfig, embeddingConfigs[p.policyInstanceHash]) || embeddingProviders[p.policyInstanceHash] == nil {
		provider, err := createEmbeddingProvider(p.embeddingConfig)
		if err != nil {
			return fmt.Errorf("failed to create embedding provider: %w", err)
		}

		embeddingProviders[p.policyInstanceHash] = provider
		embeddingConfigs[p.policyInstanceHash] = p.embeddingConfig
		p.embeddingProvider = provider
	} else {
		p.embeddingProvider = embeddingProviders[p.policyInstanceHash]
	}

	return nil
}

// initializeVectorStoreProvider initializes or reuses the vector store provider
func (p *SemanticCachePolicy) initializeVectorStoreProvider() error {
	providerMutex.RLock()
	configChanged := !reflect.DeepEqual(p.vectorStoreConfig, vectorStoreConfigs[p.policyInstanceHash])
	existingProvider := vectorStoreProviders[p.policyInstanceHash]
	providerMutex.RUnlock()

	if !configChanged && existingProvider != nil {
		p.vectorStoreProvider = existingProvider
		return nil
	}

	providerMutex.Lock()
	defer providerMutex.Unlock()

	// Check again after acquiring lock
	if !reflect.DeepEqual(p.vectorStoreConfig, vectorStoreConfigs[p.policyInstanceHash]) || vectorStoreProviders[p.policyInstanceHash] == nil {
		provider, err := createVectorDBProvider(p.vectorStoreConfig)
		if err != nil {
			return fmt.Errorf("failed to create vector store provider: %w", err)
		}

		vectorStoreProviders[p.policyInstanceHash] = provider
		vectorStoreConfigs[p.policyInstanceHash] = p.vectorStoreConfig
		p.vectorStoreProvider = provider
	} else {
		p.vectorStoreProvider = vectorStoreProviders[p.policyInstanceHash]
	}

	return nil
}

// createEmbeddingProvider creates a new embedding provider based on the config
func createEmbeddingProvider(config embeddingproviders.EmbeddingProviderConfig) (embeddingproviders.EmbeddingProvider, error) {
	var provider embeddingproviders.EmbeddingProvider

	switch config.EmbeddingProvider {
	case "OPENAI":
		provider = &embeddingproviders.OpenAIEmbeddingProvider{}
	case "MISTRAL":
		provider = &embeddingproviders.MistralEmbeddingProvider{}
	case "AZURE_OPENAI":
		provider = &embeddingproviders.AzureOpenAIEmbeddingProvider{}
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", config.EmbeddingProvider)
	}

	if err := provider.Init(config); err != nil {
		return nil, fmt.Errorf("failed to initialize embedding provider: %w", err)
	}

	return provider, nil
}

// createVectorDBProvider creates a new vector DB provider based on the config
func createVectorDBProvider(config vectordbproviders.VectorDBProviderConfig) (vectordbproviders.VectorDBProvider, error) {
	var provider vectordbproviders.VectorDBProvider

	switch config.VectorStoreProvider {
	case "REDIS":
		provider = &vectordbproviders.RedisVectorDBProvider{}
	case "MILVUS":
		provider = &vectordbproviders.MilvusVectorDBProvider{}
	default:
		return nil, fmt.Errorf("unsupported vector store provider: %s", config.VectorStoreProvider)
	}

	if err := provider.Init(config); err != nil {
		return nil, fmt.Errorf("failed to initialize vector store provider: %w", err)
	}

	// Create index immediately after provider initialization
	if err := provider.CreateIndex(); err != nil {
		return nil, fmt.Errorf("failed to create vector store index: %w", err)
	}

	return provider, nil
}

// ensureIndexCreated ensures the vector store index is created
func (p *SemanticCachePolicy) ensureIndexCreated() error {
	indexMutex.RLock()
	created := indexCreated[p.policyInstanceHash]
	indexMutex.RUnlock()

	if created {
		return nil
	}

	indexMutex.Lock()
	defer indexMutex.Unlock()

	// Check again after acquiring lock (double-check pattern)
	if indexCreated[p.policyInstanceHash] {
		return nil
	}

	// Create index
	if err := p.vectorStoreProvider.CreateIndex(); err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	indexCreated[p.policyInstanceHash] = true
	return nil
}

// Mode returns the processing mode for this policy
func (p *SemanticCachePolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// Name returns the policy name
func (p *SemanticCachePolicy) Name() string {
	return "SemanticCache"
}

// Version returns the policy version
func (p *SemanticCachePolicy) Version() string {
	return "v1.0.0"
}

// OnRequest handles request body processing for semantic caching
func (p *SemanticCachePolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	if ctx.Body == nil || ctx.Body.Content == nil || len(ctx.Body.Content) == 0 {
		return policy.UpstreamRequestModifications{}
	}

	// Extract text from request body using JSONPath if specified
	textToEmbed := string(ctx.Body.Content)
	if p.jsonPath != "" {
		extracted, err := utils.ExtractStringValueFromJsonpath(ctx.Body.Content, p.jsonPath)
		if err != nil {
			// If JSONPath extraction fails, use entire body
			textToEmbed = string(ctx.Body.Content)
		} else {
			textToEmbed = extracted
		}
	}

	// Generate embedding
	embedding, err := p.embeddingProvider.GetEmbedding(textToEmbed)
	if err != nil {
		// Log error but don't block request
		return policy.UpstreamRequestModifications{}
	}

	// Store embedding in metadata for response phase
	if ctx.Metadata == nil {
		ctx.Metadata = make(map[string]interface{})
	}
	embeddingBytes, err := json.Marshal(embedding)
	if err == nil {
		ctx.Metadata[MetadataKeyEmbedding] = string(embeddingBytes)
	}

	// Ensure index is created (lazy initialization)
	if err := p.ensureIndexCreated(); err != nil {
		// If index creation fails, continue to upstream (don't block request)
		return policy.UpstreamRequestModifications{}
	}

	// Get API ID from context (use APIName and APIVersion to create unique ID)
	apiID := fmt.Sprintf("%s:%s", ctx.APIName, ctx.APIVersion)
	if apiID == ":" {
		// Fallback to route name if API info not available
		apiID = ctx.RequestID
	}

	// Check cache for similar response
	// Threshold needs to be a string for the vector DB provider
	cacheFilter := map[string]interface{}{
		"threshold": fmt.Sprintf("%.2f", p.threshold),
		"api_id":    apiID,
		"ctx":       context.Background(), // Vector DB providers need context
	}

	cacheResponse, err := p.vectorStoreProvider.Retrieve(embedding, cacheFilter)
	if err != nil {
		// Cache miss or error - continue to upstream
		return policy.UpstreamRequestModifications{}
	}

	// Check if we got a valid cache response
	// Retrieve returns empty CacheResponse on no match or threshold not met
	if cacheResponse.ResponsePayload == nil || len(cacheResponse.ResponsePayload) == 0 {
		// Cache miss - continue to upstream
		return policy.UpstreamRequestModifications{}
	}

	// Cache hit - return cached response immediately
	responseBytes, err := json.Marshal(cacheResponse.ResponsePayload)
	if err != nil {
		return policy.UpstreamRequestModifications{}
	}

	return policy.ImmediateResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type":   "application/json",
			"X-Cache-Status": "HIT",
		},
		Body: responseBytes,
	}
}

// OnResponse handles response body processing for semantic caching
func (p *SemanticCachePolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// Only cache successful responses (2xx status codes)
	if ctx.ResponseStatus < 200 || ctx.ResponseStatus >= 300 {
		return policy.UpstreamResponseModifications{}
	}

	if ctx.ResponseBody == nil || ctx.ResponseBody.Content == nil {
		return policy.UpstreamResponseModifications{}
	}

	// Retrieve embedding from metadata (stored in request phase)
	embeddingStr, ok := ctx.Metadata[MetadataKeyEmbedding].(string)
	if !ok || embeddingStr == "" {
		return policy.UpstreamResponseModifications{}
	}

	// Deserialize embedding
	var embedding []float32
	if err := json.Unmarshal([]byte(embeddingStr), &embedding); err != nil {
		return policy.UpstreamResponseModifications{}
	}

	// Parse response body
	var responseData map[string]interface{}
	if err := json.Unmarshal(ctx.ResponseBody.Content, &responseData); err != nil {
		return policy.UpstreamResponseModifications{}
	}

	// Get API ID from context (use APIName and APIVersion to create unique ID)
	apiID := fmt.Sprintf("%s:%s", ctx.APIName, ctx.APIVersion)
	if apiID == ":" {
		// Fallback to route name if API info not available
		apiID = ctx.RequestID
	}

	// Store in cache
	cacheResponse := vectordbproviders.CacheResponse{
		ResponsePayload:     responseData,
		RequestHash:         uuid.New().String(),
		ResponseFetchedTime: time.Now(),
	}

	cacheFilter := map[string]interface{}{
		"api_id": apiID,
		"ctx":    context.Background(), // Vector DB providers need context
	}

	if err := p.vectorStoreProvider.Store(embedding, cacheResponse, cacheFilter); err != nil {
		// Log error but don't modify response
		return policy.UpstreamResponseModifications{}
	}

	return policy.UpstreamResponseModifications{}
}

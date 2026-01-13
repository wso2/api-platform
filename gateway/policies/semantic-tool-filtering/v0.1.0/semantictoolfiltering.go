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

package semantictoolfiltering

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
	embeddingproviders "github.com/wso2/api-platform/sdk/utils/embeddingproviders"
)

const (
	// Selection modes
	SelectionModeTopK      = "TOP_K"
	SelectionModeThreshold = "THRESHOLD"

	// Internal timeout for embedding provider (not exposed in policy definition)
	DefaultTimeoutMs = 5000
)

// ToolWithScore represents a tool with its similarity score
type ToolWithScore struct {
	Tool  map[string]interface{}
	Score float64
}

// SemanticToolFilteringPolicy implements semantic filtering for tool selection
type SemanticToolFilteringPolicy struct {
	embeddingConfig   embeddingproviders.EmbeddingProviderConfig
	embeddingProvider embeddingproviders.EmbeddingProvider
	selectionMode     string
	topK              int
	threshold         float64
	queryJSONPath     string
	toolsJSONPath     string
}

// GetPolicy creates a new instance of the semantic tool filtering policy
func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	p := &SemanticToolFilteringPolicy{}

	// Parse and validate embedding provider configuration (from systemParameters)
	if err := parseEmbeddingConfig(params, p); err != nil {
		return nil, fmt.Errorf("invalid embedding config: %w", err)
	}

	// Initialize embedding provider
	embeddingProvider, err := createEmbeddingProvider(p.embeddingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding provider: %w", err)
	}
	p.embeddingProvider = embeddingProvider

	// Parse policy parameters (runtime parameters)
	if err := parseParams(params, p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	slog.Debug("SemanticToolFiltering: Policy initialized",
		"embeddingProvider", p.embeddingConfig.EmbeddingProvider,
		"selectionMode", p.selectionMode,
		"topK", p.topK,
		"threshold", p.threshold)

	return p, nil
}

// parseEmbeddingConfig parses and validates embedding provider configuration
func parseEmbeddingConfig(params map[string]interface{}, p *SemanticToolFilteringPolicy) error {
	provider, ok := params["embeddingProvider"].(string)
	if !ok || provider == "" {
		return fmt.Errorf("'embeddingProvider' is required")
	}

	embeddingEndpoint, ok := params["embeddingEndpoint"].(string)
	if !ok || embeddingEndpoint == "" {
		return fmt.Errorf("'embeddingEndpoint' is required")
	}

	// embeddingModel is required for OPENAI and MISTRAL, but not for AZURE_OPENAI
	embeddingModel, ok := params["embeddingModel"].(string)
	if !ok || embeddingModel == "" {
		providerUpper := strings.ToUpper(provider)
		if providerUpper == "OPENAI" || providerUpper == "MISTRAL" {
			return fmt.Errorf("'embeddingModel' is required for %s provider", provider)
		}
		// For AZURE_OPENAI, embeddingModel is optional (deployment name is in endpoint)
		embeddingModel = ""
	}

	apiKey, ok := params["apiKey"].(string)
	if !ok || apiKey == "" {
		return fmt.Errorf("'apiKey' is required")
	}

	// Set header name based on provider type
	// Azure OpenAI uses "api-key", others use "Authorization"
	authHeaderName := "Authorization"
	if strings.ToUpper(provider) == "AZURE_OPENAI" {
		authHeaderName = "api-key"
	}

	p.embeddingConfig = embeddingproviders.EmbeddingProviderConfig{
		EmbeddingProvider: strings.ToUpper(provider),
		EmbeddingEndpoint: embeddingEndpoint,
		APIKey:            apiKey,
		AuthHeaderName:    authHeaderName,
		EmbeddingModel:    embeddingModel,
		TimeOut:           strconv.Itoa(DefaultTimeoutMs),
	}

	return nil
}

// parseParams parses and validates runtime parameters from the params map
func parseParams(params map[string]interface{}, p *SemanticToolFilteringPolicy) error {
	fmt.Println("Parsing params:", params)
	// Optional: selectionMode (default TOP_K)
	selectionMode, ok := params["selectionMode"].(string)
	if !ok || selectionMode == "" {
		selectionMode = SelectionModeTopK
	}
	if selectionMode != SelectionModeTopK && selectionMode != SelectionModeThreshold {
		return fmt.Errorf("'selectionMode' must be TOP_K or THRESHOLD")
	}
	p.selectionMode = selectionMode

	// Optional: topK (default 5 as per policy-definition.yaml)
	if topKRaw, ok := params["topK"]; ok {
		topK, err := extractInt(topKRaw)
		if err != nil {
			return fmt.Errorf("'topK' must be a number: %w", err)
		}
		if topK < 0 || topK > 20 {
			return fmt.Errorf("'topK' must be between 0 and 20")
		}
		p.topK = topK
	} else {
		p.topK = 5 // default from policy-definition.yaml
	}

	// Optional: similarityThreshold (default 0.7 as per policy-definition.yaml)
	if thresholdRaw, ok := params["similarityThreshold"]; ok {
		threshold, err := extractFloat64(thresholdRaw)
		if err != nil {
			return fmt.Errorf("'similarityThreshold' must be a number: %w", err)
		}
		if threshold < 0.0 || threshold > 1.0 {
			return fmt.Errorf("'similarityThreshold' must be between 0.0 and 1.0")
		}
		p.threshold = threshold
	} else {
		p.threshold = 0.7 // default from policy-definition.yaml
	}

	// Optional: jsonPath (default "$.messages[-1].content" as per policy-definition.yaml)
	if jsonPathRaw, ok := params["jsonPath"]; ok {
		if jsonPath, ok := jsonPathRaw.(string); ok {
			if jsonPath != "" {
				p.queryJSONPath = jsonPath
			} else {
				p.queryJSONPath = "$.messages[-1].content" // default from policy-definition.yaml
			}
		} else {
			return fmt.Errorf("'jsonPath' must be a string")
		}
	} else {
		p.queryJSONPath = "$.messages[-1].content" // default from policy-definition.yaml
	}

	// Optional: toolsPath (default "$.tools" as per policy-definition.yaml)
	if toolsPathRaw, ok := params["toolsPath"]; ok {
		if toolsPath, ok := toolsPathRaw.(string); ok {
			if toolsPath != "" {
				p.toolsJSONPath = toolsPath
			} else {
				p.toolsJSONPath = "$.tools" // default from policy-definition.yaml
			}
		} else {
			return fmt.Errorf("'toolsPath' must be a string")
		}
	} else {
		p.toolsJSONPath = "$.tools" // default from policy-definition.yaml
	}

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

// Mode returns the processing mode for this policy
func (p *SemanticToolFilteringPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer, // Need to read and modify request body
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnRequest handles request body processing for semantic tool filtering
func (p *SemanticToolFilteringPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	var content []byte
	if ctx.Body != nil {
		content = ctx.Body.Content
	}

	if len(content) == 0 {
		slog.Debug("SemanticToolFiltering: Empty request body")
		return policy.UpstreamRequestModifications{}
	}

	// Parse request body as JSON
	var requestBody map[string]interface{}
	if err := json.Unmarshal(content, &requestBody); err != nil {
		return p.buildErrorResponse("Invalid JSON in request body", err)
	}

	// Extract user query using JSONPath
	userQuery, err := utils.ExtractStringValueFromJsonpath(content, p.queryJSONPath)
	if err != nil {
		return p.buildErrorResponse("Error extracting user query from JSONPath", err)
	}

	if userQuery == "" {
		slog.Debug("SemanticToolFiltering: Empty user query")
		return policy.UpstreamRequestModifications{}
	}

	// Extract tools array using JSONPath
	toolsJSON, err := utils.ExtractValueFromJsonpath(requestBody, p.toolsJSONPath)
	if err != nil {
		return p.buildErrorResponse("Error extracting tools from JSONPath", err)
	}

	// Parse tools array
	var tools []interface{}
	var toolsBytes []byte
	switch v := toolsJSON.(type) {
	case []byte:
		toolsBytes = v
	case string:
		toolsBytes = []byte(v)
	default:
		var err error
		toolsBytes, err = json.Marshal(v)
		if err != nil {
			return p.buildErrorResponse("Invalid tools format in request", err)
		}
	}
	if err := json.Unmarshal(toolsBytes, &tools); err != nil {
		return p.buildErrorResponse("Invalid tools format in request", err)
	}

	if len(tools) == 0 {
		slog.Debug("SemanticToolFiltering: No tools to filter")
		return policy.UpstreamRequestModifications{}
	}

	// Generate embedding for user query
	queryEmbedding, err := p.embeddingProvider.GetEmbedding(userQuery)
	if err != nil {
		slog.Error("SemanticToolFiltering: Error generating query embedding", "error", err)
		return p.buildErrorResponse("Error generating query embedding", err)
	}

	// Calculate similarity scores for each tool
	toolsWithScores := make([]ToolWithScore, 0, len(tools))
	for _, toolRaw := range tools {
		toolMap, ok := toolRaw.(map[string]interface{})
		if !ok {
			slog.Warn("SemanticToolFiltering: Invalid tool format, skipping")
			continue
		}

		// Extract tool description (try common fields)
		toolDesc := extractToolDescription(toolMap)
		if toolDesc == "" {
			slog.Warn("SemanticToolFiltering: No description found for tool, skipping",
				"toolName", toolMap["name"])
			continue
		}

		// Generate embedding for tool description
		toolEmbedding, err := p.embeddingProvider.GetEmbedding(toolDesc)
		if err != nil {
			slog.Warn("SemanticToolFiltering: Error generating tool embedding, skipping",
				"error", err, "toolName", toolMap["name"])
			continue
		}

		// Calculate cosine similarity
		similarity, err := cosineSimilarity(queryEmbedding, toolEmbedding)
		if err != nil {
			slog.Warn("SemanticToolFiltering: Error calculating similarity, skipping",
				"error", err, "toolName", toolMap["name"])
			continue
		}

		toolsWithScores = append(toolsWithScores, ToolWithScore{
			Tool:  toolMap,
			Score: similarity,
		})
	}

	if len(toolsWithScores) == 0 {
		slog.Debug("SemanticToolFiltering: No valid tools after embedding generation")
		return policy.UpstreamRequestModifications{}
	}

	// Filter tools based on selection mode
	filteredTools := p.filterTools(toolsWithScores)

	slog.Debug("SemanticToolFiltering: Filtered tools",
		"originalCount", len(tools),
		"filteredCount", len(filteredTools),
		"selectionMode", p.selectionMode)

	// Update request body with filtered tools
	if err := updateToolsInRequestBody(&requestBody, p.toolsJSONPath, filteredTools); err != nil {
		return p.buildErrorResponse("Error updating request body with filtered tools", err)
	}

	// Marshal modified request body
	modifiedBody, err := json.Marshal(requestBody)
	if err != nil {
		return p.buildErrorResponse("Error marshaling modified request body", err)
	}

	return policy.UpstreamRequestModifications{
		Body: modifiedBody,
	}
}

// OnResponse is a no-op for this policy (only modifies requests)
func (p *SemanticToolFilteringPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	return policy.UpstreamResponseModifications{}
}

// extractToolDescription extracts description text from a tool definition
func extractToolDescription(tool map[string]interface{}) string {
	// Try common fields for tool description
	fields := []string{"description", "desc", "summary", "info"}

	for _, field := range fields {
		if desc, ok := tool[field].(string); ok && desc != "" {
			return desc
		}
	}

	// If no description field, try to use name + function description
	name, _ := tool["name"].(string)

	// Check for function/parameters structure (OpenAI format)
	if function, ok := tool["function"].(map[string]interface{}); ok {
		if desc, ok := function["description"].(string); ok && desc != "" {
			if name != "" {
				return fmt.Sprintf("%s: %s", name, desc)
			}
			return desc
		}
	}

	// Fallback to just name if available
	if name != "" {
		return name
	}

	return ""
}

// cosineSimilarity calculates cosine similarity between two embeddings
func cosineSimilarity(a, b []float32) (float64, error) {
	if len(a) == 0 || len(b) == 0 {
		return 0, fmt.Errorf("embedding vectors cannot be empty")
	}

	if len(a) != len(b) {
		return 0, fmt.Errorf("embedding dimensions do not match: %d vs %d", len(a), len(b))
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0, fmt.Errorf("embedding vector norm is zero")
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB)), nil
}

// filterTools filters tools based on selection mode and criteria
func (p *SemanticToolFilteringPolicy) filterTools(toolsWithScores []ToolWithScore) []map[string]interface{} {
	// Sort by score in descending order
	sort.Slice(toolsWithScores, func(i, j int) bool {
		return toolsWithScores[i].Score > toolsWithScores[j].Score
	})

	var filtered []map[string]interface{}

	switch p.selectionMode {
	case SelectionModeTopK:
		// Select top K tools
		limit := p.topK
		if limit > len(toolsWithScores) {
			limit = len(toolsWithScores)
		}
		for i := 0; i < limit; i++ {
			filtered = append(filtered, toolsWithScores[i].Tool)
		}

	case SelectionModeThreshold:
		// Select all tools above threshold
		for _, item := range toolsWithScores {
			if item.Score >= p.threshold {
				filtered = append(filtered, item.Tool)
			}
		}
	}

	return filtered
}

// updateToolsInRequestBody updates the tools array in the request body
func updateToolsInRequestBody(requestBody *map[string]interface{}, toolsPath string, tools []map[string]interface{}) error {
	// For simplicity, assume toolsPath is "$.tools" which means top-level "tools" key
	// More complex JSONPath would require a library for setting values
	if toolsPath == "$.tools" {
		(*requestBody)["tools"] = tools
		return nil
	}

	// For other paths, we'd need a more sophisticated JSONPath setter
	// For now, return error for unsupported paths
	return fmt.Errorf("unsupported toolsPath: %s (only $.tools is currently supported)", toolsPath)
}

// buildErrorResponse builds an error response
func (p *SemanticToolFilteringPolicy) buildErrorResponse(message string, err error) policy.RequestAction {
	errorMsg := message
	if err != nil {
		errorMsg = fmt.Sprintf("%s: %v", message, err)
	}

	slog.Error("SemanticToolFiltering: " + errorMsg)

	responseBody := map[string]interface{}{
		"error": map[string]interface{}{
			"type":    "SEMANTIC_TOOL_FILTERING",
			"message": errorMsg,
		},
	}

	bodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		bodyBytes = []byte(`{"error":{"type":"SEMANTIC_TOOL_FILTERING","message":"Internal error"}}`)
	}

	return policy.ImmediateResponse{
		StatusCode: 400,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: bodyBytes,
	}
}

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
	SelectionModeTopK      = "By Rank"
	SelectionModeThreshold = "By Threshold"

	// Internal timeout for embedding provider (not exposed in policy definition)
	DefaultTimeoutMs = 5000
)

// ToolWithScore represents a tool with its similarity score
type ToolWithScore struct {
	Tool  map[string]interface{}
	Score float64
}

// TextTool represents a tool parsed from text format
type TextTool struct {
	Name        string
	Description string
	StartPos    int // Start position in original text
	EndPos      int // End position in original text (after </tooldescription>)
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
	userQueryIsJson   bool
	toolsIsJson       bool
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
		return fmt.Errorf("'selectionMode' must be By Rank or By Threshold")
	}
	p.selectionMode = selectionMode

	// Optional: Limit (default 5 as per policy-definition.yaml)
	if limitRaw, ok := params["Limit"]; ok {
		limit, err := extractInt(limitRaw)
		if err != nil {
			return fmt.Errorf("'Limit' must be a number: %w", err)
		}
		if limit < 0 || limit > 20 {
			return fmt.Errorf("'Limit' must be between 0 and 20")
		}
		p.topK = limit
	} else {
		p.topK = 5 // default from policy-definition.yaml
	}

	// Optional: similarityThreshold (default 0.7 as per policy-definition.yaml)
	if thresholdRaw, ok := params["Threshold"]; ok {
		threshold, err := extractFloat64(thresholdRaw)
		if err != nil {
			return fmt.Errorf("'Threshold' must be a number: %w", err)
		}
		if threshold < 0.0 || threshold > 1.0 {
			return fmt.Errorf("'Threshold' must be between 0.0 and 1.0")
		}
		p.threshold = threshold
	} else {
		p.threshold = 0.7 // default from policy-definition.yaml
	}

	// Optional: jsonPath (default "$.messages[-1].content" as per policy-definition.yaml)
	if jsonPathRaw, ok := params["queryJSONPath"]; ok {
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
	if toolsPathRaw, ok := params["toolsJSONPath"]; ok {
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

	// Optional: userQueryIsJson (default true - JSON format)
	if userQueryIsJsonRaw, ok := params["userQueryIsJson"]; ok {
		userQueryIsJson, err := extractBool(userQueryIsJsonRaw)
		if err != nil {
			return fmt.Errorf("'userQueryIsJson' must be a boolean: %w", err)
		}
		p.userQueryIsJson = userQueryIsJson
	} else {
		p.userQueryIsJson = true // default to JSON format
	}

	// Optional: toolsIsJson (default true - JSON format)
	if toolsIsJsonRaw, ok := params["toolsIsJson"]; ok {
		toolsIsJson, err := extractBool(toolsIsJsonRaw)
		if err != nil {
			return fmt.Errorf("'toolsIsJson' must be a boolean: %w", err)
		}
		p.toolsIsJson = toolsIsJson
	} else {
		p.toolsIsJson = true // default to JSON format
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

// extractBool safely extracts a boolean from various types
func extractBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		lower := strings.ToLower(v)
		if lower == "true" || lower == "1" || lower == "yes" {
			return true, nil
		}
		if lower == "false" || lower == "0" || lower == "no" {
			return false, nil
		}
		return false, fmt.Errorf("cannot convert %q to bool", v)
	case int:
		return v != 0, nil
	case float64:
		return v != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", value)
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

// extractUserQueryFromText extracts user query from text content using <userq> tags
func extractUserQueryFromText(content string) (string, error) {
	startTag := "<userq>"
	endTag := "</userq>"

	startIdx := strings.Index(content, startTag)
	if startIdx == -1 {
		return "", fmt.Errorf("user query start tag <userq> not found")
	}

	endIdx := strings.Index(content, endTag)
	if endIdx == -1 {
		return "", fmt.Errorf("user query end tag </userq> not found")
	}

	if endIdx <= startIdx {
		return "", fmt.Errorf("invalid user query tag positions")
	}

	query := content[startIdx+len(startTag) : endIdx]
	return strings.TrimSpace(query), nil
}

// extractToolsFromText extracts tools from text content using <toolname> and <tooldescription> tags
func extractToolsFromText(content string) ([]TextTool, error) {
	var tools []TextTool

	toolNameStartTag := "<toolname>"
	toolNameEndTag := "</toolname>"
	toolDescStartTag := "<tooldescription>"
	toolDescEndTag := "</tooldescription>"

	// Find all tool definitions in the content
	searchStart := 0
	for {
		// Find tool name
		nameStartIdx := strings.Index(content[searchStart:], toolNameStartTag)
		if nameStartIdx == -1 {
			break
		}
		nameStartIdx += searchStart

		nameEndIdx := strings.Index(content[nameStartIdx:], toolNameEndTag)
		if nameEndIdx == -1 {
			return nil, fmt.Errorf("tool name end tag </toolname> not found for tool starting at position %d", nameStartIdx)
		}
		nameEndIdx += nameStartIdx

		toolName := strings.TrimSpace(content[nameStartIdx+len(toolNameStartTag) : nameEndIdx])

		// Find tool description after the name
		descSearchStart := nameEndIdx + len(toolNameEndTag)
		descStartIdx := strings.Index(content[descSearchStart:], toolDescStartTag)
		if descStartIdx == -1 {
			return nil, fmt.Errorf("tool description start tag <tooldescription> not found for tool '%s'", toolName)
		}
		descStartIdx += descSearchStart

		descEndIdx := strings.Index(content[descStartIdx:], toolDescEndTag)
		if descEndIdx == -1 {
			return nil, fmt.Errorf("tool description end tag </tooldescription> not found for tool '%s'", toolName)
		}
		descEndIdx += descStartIdx

		toolDesc := strings.TrimSpace(content[descStartIdx+len(toolDescStartTag) : descEndIdx])

		tools = append(tools, TextTool{
			Name:        toolName,
			Description: toolDesc,
			StartPos:    nameStartIdx,
			EndPos:      descEndIdx + len(toolDescEndTag),
		})

		// Move search start past this tool
		searchStart = descEndIdx + len(toolDescEndTag)
	}

	return tools, nil
}

// rebuildTextWithFilteredTools rebuilds the text content keeping only filtered tools
func rebuildTextWithFilteredTools(originalContent string, allTools []TextTool, filteredToolNames map[string]bool) string {
	if len(allTools) == 0 {
		return originalContent
	}

	// Sort tools by start position in reverse order to process from end to start
	// This ensures position calculations remain valid as we remove content
	sortedTools := make([]TextTool, len(allTools))
	copy(sortedTools, allTools)
	sort.Slice(sortedTools, func(i, j int) bool {
		return sortedTools[i].StartPos > sortedTools[j].StartPos
	})

	result := originalContent

	// Remove tools that are not in the filtered list
	for _, tool := range sortedTools {
		if !filteredToolNames[tool.Name] {
			// Remove this tool from the content
			result = result[:tool.StartPos] + result[tool.EndPos:]
		}
	}

	// Clean up any extra whitespace/newlines left after removal
	result = cleanupWhitespace(result)

	return result
}

// cleanupWhitespace removes excessive whitespace while preserving structure
func cleanupWhitespace(content string) string {
	// Replace multiple consecutive newlines with double newline
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}
	// Replace multiple consecutive spaces with single space
	for strings.Contains(content, "  ") {
		content = strings.ReplaceAll(content, "  ", " ")
	}
	return strings.TrimSpace(content)
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
	slog.Debug("Onrequest")
	// Handle based on format type (JSON or Text)
	if p.userQueryIsJson && p.toolsIsJson {
		// Pure JSON mode
		return p.handleJSONRequest(ctx, content)
	} else if !p.userQueryIsJson && !p.toolsIsJson {
		// Pure Text mode
		return p.handleTextRequest(ctx, content)
	} else {
		// Mixed mode
		return p.handleMixedRequest(ctx, content)
	}
}

// handleJSONRequest handles requests where both user query and tools are in JSON format
func (p *SemanticToolFilteringPolicy) handleJSONRequest(ctx *policy.RequestContext, content []byte) policy.RequestAction {
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

	// Get embedding cache instance
	embeddingCache := policy.GetEmbeddingCacheStoreInstance()
	apiId := ctx.APIId

	// Ensure API cache exists
	if !embeddingCache.HasAPI(apiId) {
		embeddingCache.AddAPICache(apiId)
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

		// Get tool name for cache entry
		toolName, _ := toolMap["name"].(string)

		// Hash the tool description for cache lookup
		descHash := policy.HashDescription(toolDesc)

		// Try to get embedding from cache
		var toolEmbedding []float32
		cachedEntry := embeddingCache.GetEntry(apiId, descHash)
		if cachedEntry != nil {
			// Cache hit - use cached embedding
			toolEmbedding = cachedEntry.Embedding
			slog.Debug("SemanticToolFiltering: Cache hit for tool embedding",
				"toolName", toolName)
		} else {
			// Cache miss - generate embedding and store in cache
			var err error
			toolEmbedding, err = p.embeddingProvider.GetEmbedding(toolDesc)
			if err != nil {
				slog.Warn("SemanticToolFiltering: Error generating tool embedding, skipping",
					"error", err, "toolName", toolName)
				continue
			}

			// Store in cache
			embeddingCache.AddEntry(apiId, descHash, toolName, toolEmbedding)
			slog.Debug("SemanticToolFiltering: Cached new tool embedding",
				"toolName", toolName)
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

// handleTextRequest handles requests where both user query and tools are in text format with tags
func (p *SemanticToolFilteringPolicy) handleTextRequest(ctx *policy.RequestContext, content []byte) policy.RequestAction {
	contentStr := string(content)

	// Extract user query from <userq> tags
	userQuery, err := extractUserQueryFromText(contentStr)
	if err != nil {
		return p.buildErrorResponse("Error extracting user query from text", err)
	}

	if userQuery == "" {
		slog.Debug("SemanticToolFiltering: Empty user query")
		return policy.UpstreamRequestModifications{}
	}

	// Extract tools from <toolname> and <tooldescription> tags
	textTools, err := extractToolsFromText(contentStr)
	if err != nil {
		return p.buildErrorResponse("Error extracting tools from text", err)
	}

	if len(textTools) == 0 {
		slog.Debug("SemanticToolFiltering: No tools to filter")
		return policy.UpstreamRequestModifications{}
	}

	// Generate embedding for user query
	queryEmbedding, err := p.embeddingProvider.GetEmbedding(userQuery)
	if err != nil {
		slog.Error("SemanticToolFiltering: Error generating query embedding", "error", err)
		return p.buildErrorResponse("Error generating query embedding", err)
	}

	// Get embedding cache instance
	embeddingCache := policy.GetEmbeddingCacheStoreInstance()
	apiId := ctx.APIId

	// Ensure API cache exists
	if !embeddingCache.HasAPI(apiId) {
		embeddingCache.AddAPICache(apiId)
	}

	// Calculate similarity scores for each tool
	type TextToolWithScore struct {
		Tool  TextTool
		Score float64
	}
	toolsWithScores := make([]TextToolWithScore, 0, len(textTools))

	for _, tool := range textTools {
		// Use name + description for better semantic matching
		toolText := fmt.Sprintf("%s: %s", tool.Name, tool.Description)

		// Hash the tool text for cache lookup
		textHash := policy.HashDescription(toolText)

		// Try to get embedding from cache
		var toolEmbedding []float32
		cachedEntry := embeddingCache.GetEntry(apiId, textHash)
		if cachedEntry != nil {
			// Cache hit - use cached embedding
			toolEmbedding = cachedEntry.Embedding
			slog.Debug("SemanticToolFiltering: Cache hit for tool embedding",
				"toolName", tool.Name)
		} else {
			// Cache miss - generate embedding and store in cache
			var err error
			toolEmbedding, err = p.embeddingProvider.GetEmbedding(toolText)
			if err != nil {
				slog.Warn("SemanticToolFiltering: Error generating tool embedding, skipping",
					"error", err, "toolName", tool.Name)
				continue
			}

			// Store in cache
			embeddingCache.AddEntry(apiId, textHash, tool.Name, toolEmbedding)
			slog.Debug("SemanticToolFiltering: Cached new tool embedding",
				"toolName", tool.Name)
		}

		// Calculate cosine similarity
		similarity, err := cosineSimilarity(queryEmbedding, toolEmbedding)
		if err != nil {
			slog.Warn("SemanticToolFiltering: Error calculating similarity, skipping",
				"error", err, "toolName", tool.Name)
			continue
		}

		toolsWithScores = append(toolsWithScores, TextToolWithScore{
			Tool:  tool,
			Score: similarity,
		})
	}

	if len(toolsWithScores) == 0 {
		slog.Debug("SemanticToolFiltering: No valid tools after embedding generation")
		return policy.UpstreamRequestModifications{}
	}

	// Sort by score in descending order
	sort.Slice(toolsWithScores, func(i, j int) bool {
		return toolsWithScores[i].Score > toolsWithScores[j].Score
	})

	// Filter based on selection mode
	filteredToolNames := make(map[string]bool)
	switch p.selectionMode {
	case SelectionModeTopK:
		limit := p.topK
		if limit > len(toolsWithScores) {
			limit = len(toolsWithScores)
		}
		for i := 0; i < limit; i++ {
			filteredToolNames[toolsWithScores[i].Tool.Name] = true
		}
	case SelectionModeThreshold:
		for _, item := range toolsWithScores {
			if item.Score >= p.threshold {
				filteredToolNames[item.Tool.Name] = true
			}
		}
	}

	slog.Debug("SemanticToolFiltering: Filtered tools (text mode)",
		"originalCount", len(textTools),
		"filteredCount", len(filteredToolNames),
		"selectionMode", p.selectionMode)

	// Rebuild text content with only filtered tools
	modifiedContent := rebuildTextWithFilteredTools(contentStr, textTools, filteredToolNames)

	return policy.UpstreamRequestModifications{
		Body: []byte(modifiedContent),
	}
}

// handleMixedRequest handles requests where user query and tools have different formats
func (p *SemanticToolFilteringPolicy) handleMixedRequest(ctx *policy.RequestContext, content []byte) policy.RequestAction {
	contentStr := string(content)
	var userQuery string
	var err error

	// Extract user query based on format
	if p.userQueryIsJson {
		// Try to parse as JSON and extract using JSONPath
		userQuery, err = utils.ExtractStringValueFromJsonpath(content, p.queryJSONPath)
		if err != nil {
			return p.buildErrorResponse("Error extracting user query from JSONPath", err)
		}
	} else {
		// Extract from text tags
		userQuery, err = extractUserQueryFromText(contentStr)
		if err != nil {
			return p.buildErrorResponse("Error extracting user query from text", err)
		}
	}

	if userQuery == "" {
		slog.Debug("SemanticToolFiltering: Empty user query")
		return policy.UpstreamRequestModifications{}
	}

	// Generate embedding for user query
	queryEmbedding, err := p.embeddingProvider.GetEmbedding(userQuery)
	if err != nil {
		slog.Error("SemanticToolFiltering: Error generating query embedding", "error", err)
		return p.buildErrorResponse("Error generating query embedding", err)
	}

	// Get embedding cache instance
	embeddingCache := policy.GetEmbeddingCacheStoreInstance()
	apiId := ctx.APIId

	// Ensure API cache exists
	if !embeddingCache.HasAPI(apiId) {
		embeddingCache.AddAPICache(apiId)
	}

	// Handle tools based on format
	if p.toolsIsJson {
		// Parse as JSON and handle tools
		var requestBody map[string]interface{}
		if err := json.Unmarshal(content, &requestBody); err != nil {
			return p.buildErrorResponse("Invalid JSON in request body", err)
		}

		toolsJSON, err := utils.ExtractValueFromJsonpath(requestBody, p.toolsJSONPath)
		if err != nil {
			return p.buildErrorResponse("Error extracting tools from JSONPath", err)
		}

		var tools []interface{}
		var toolsBytes []byte
		switch v := toolsJSON.(type) {
		case []byte:
			toolsBytes = v
		case string:
			toolsBytes = []byte(v)
		default:
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

		toolsWithScores := make([]ToolWithScore, 0, len(tools))
		for _, toolRaw := range tools {
			toolMap, ok := toolRaw.(map[string]interface{})
			if !ok {
				continue
			}

			toolDesc := extractToolDescription(toolMap)
			if toolDesc == "" {
				continue
			}

			// Get tool name for cache entry
			toolName, _ := toolMap["name"].(string)

			// Hash the tool description for cache lookup
			descHash := policy.HashDescription(toolDesc)

			// Try to get embedding from cache
			var toolEmbedding []float32
			cachedEntry := embeddingCache.GetEntry(apiId, descHash)
			if cachedEntry != nil {
				// Cache hit - use cached embedding
				toolEmbedding = cachedEntry.Embedding
				slog.Debug("SemanticToolFiltering: Cache hit for tool embedding",
					"toolName", toolName)
			} else {
				// Cache miss - generate embedding and store in cache
				var err error
				toolEmbedding, err = p.embeddingProvider.GetEmbedding(toolDesc)
				if err != nil {
					continue
				}

				// Store in cache
				embeddingCache.AddEntry(apiId, descHash, toolName, toolEmbedding)
				slog.Debug("SemanticToolFiltering: Cached new tool embedding",
					"toolName", toolName)
			}

			similarity, err := cosineSimilarity(queryEmbedding, toolEmbedding)
			if err != nil {
				continue
			}

			toolsWithScores = append(toolsWithScores, ToolWithScore{
				Tool:  toolMap,
				Score: similarity,
			})
		}

		if len(toolsWithScores) == 0 {
			return policy.UpstreamRequestModifications{}
		}

		filteredTools := p.filterTools(toolsWithScores)
		if err := updateToolsInRequestBody(&requestBody, p.toolsJSONPath, filteredTools); err != nil {
			return p.buildErrorResponse("Error updating request body with filtered tools", err)
		}

		modifiedBody, err := json.Marshal(requestBody)
		if err != nil {
			return p.buildErrorResponse("Error marshaling modified request body", err)
		}

		return policy.UpstreamRequestModifications{
			Body: modifiedBody,
		}
	} else {
		// Tools in text format
		textTools, err := extractToolsFromText(contentStr)
		if err != nil {
			return p.buildErrorResponse("Error extracting tools from text", err)
		}

		if len(textTools) == 0 {
			return policy.UpstreamRequestModifications{}
		}

		type TextToolWithScore struct {
			Tool  TextTool
			Score float64
		}
		toolsWithScores := make([]TextToolWithScore, 0, len(textTools))

		for _, tool := range textTools {
			toolText := fmt.Sprintf("%s: %s", tool.Name, tool.Description)

			// Hash the tool text for cache lookup
			textHash := policy.HashDescription(toolText)

			// Try to get embedding from cache
			var toolEmbedding []float32
			cachedEntry := embeddingCache.GetEntry(apiId, textHash)
			if cachedEntry != nil {
				// Cache hit - use cached embedding
				toolEmbedding = cachedEntry.Embedding
				slog.Debug("SemanticToolFiltering: Cache hit for tool embedding",
					"toolName", tool.Name)
			} else {
				// Cache miss - generate embedding and store in cache
				var err error
				toolEmbedding, err = p.embeddingProvider.GetEmbedding(toolText)
				if err != nil {
					continue
				}

				// Store in cache
				embeddingCache.AddEntry(apiId, textHash, tool.Name, toolEmbedding)
				slog.Debug("SemanticToolFiltering: Cached new tool embedding",
					"toolName", tool.Name)
			}

			similarity, err := cosineSimilarity(queryEmbedding, toolEmbedding)
			if err != nil {
				continue
			}

			toolsWithScores = append(toolsWithScores, TextToolWithScore{
				Tool:  tool,
				Score: similarity,
			})
		}

		if len(toolsWithScores) == 0 {
			return policy.UpstreamRequestModifications{}
		}

		sort.Slice(toolsWithScores, func(i, j int) bool {
			return toolsWithScores[i].Score > toolsWithScores[j].Score
		})

		filteredToolNames := make(map[string]bool)
		switch p.selectionMode {
		case SelectionModeTopK:
			limit := p.topK
			if limit > len(toolsWithScores) {
				limit = len(toolsWithScores)
			}
			for i := 0; i < limit; i++ {
				filteredToolNames[toolsWithScores[i].Tool.Name] = true
			}
		case SelectionModeThreshold:
			for _, item := range toolsWithScores {
				if item.Score >= p.threshold {
					filteredToolNames[item.Tool.Name] = true
				}
			}
		}

		modifiedContent := rebuildTextWithFilteredTools(contentStr, textTools, filteredToolNames)

		return policy.UpstreamRequestModifications{
			Body: []byte(modifiedContent),
		}
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
	// Remove leading "$." if present
	path := strings.TrimPrefix(toolsPath, "$.")
	parts := strings.Split(path, ".")

	if len(parts) == 0 {
		return fmt.Errorf("invalid toolsPath: %s", toolsPath)
	}

	// Handle array index in path, e.g., "tools[0]"
	curr := *requestBody
	for idx, part := range parts {
		// Check if part contains array index, e.g., "tools[0]"
		if openIdx := strings.Index(part, "["); openIdx != -1 && strings.HasSuffix(part, "]") {
			field := part[:openIdx]
			indexStr := part[openIdx+1 : len(part)-1]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return fmt.Errorf("invalid array index in path: %s", part)
			}

			// If this is the last part, set the value at the array index
			if idx == len(parts)-1 {
				// Ensure the array exists
				arr, ok := curr[field].([]interface{})
				if !ok {
					// Create array if not present
					arr = make([]interface{}, index+1)
				} else if len(arr) <= index {
					// Extend array if needed
					newArr := make([]interface{}, index+1)
					copy(newArr, arr)
					arr = newArr
				}
				arr[index] = tools
				curr[field] = arr
				return nil
			}

			// Not last part, descend into the array element
			arr, ok := curr[field].([]interface{})
			if !ok {
				// Create array if not present
				arr = make([]interface{}, index+1)
				curr[field] = arr
			} else if len(arr) <= index {
				// Extend array if needed
				newArr := make([]interface{}, index+1)
				copy(newArr, arr)
				arr = newArr
				curr[field] = arr
			}
			// If element is nil, create map
			if arr[index] == nil {
				arr[index] = make(map[string]interface{})
			}
			nextMap, ok := arr[index].(map[string]interface{})
			if !ok {
				return fmt.Errorf("expected map at array index %d in field %s", index, field)
			}
			curr = nextMap
			continue
		}

		// If this is the last part, set the value
		if idx == len(parts)-1 {
			curr[part] = tools
			return nil
		}

		// If the next level doesn't exist, create it as a map
		next, ok := curr[part]
		if !ok {
			newMap := make(map[string]interface{})
			curr[part] = newMap
			curr = newMap
			continue
		}

		// If the next level is a map, descend into it
		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected map at path %s but found %T", part, next)
		}
		curr = nextMap
	}
	for i, part := range parts {
		// If this is the last part, set the value
		if i == len(parts)-1 {
			curr[part] = tools
			return nil
		}

		// If the next level doesn't exist, create it as a map
		next, ok := curr[part]
		if !ok {
			newMap := make(map[string]interface{})
			curr[part] = newMap
			curr = newMap
			continue
		}

		// If the next level is a map, descend into it
		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected map at path %s but found %T", part, next)
		}
		curr = nextMap
	}

	return nil
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

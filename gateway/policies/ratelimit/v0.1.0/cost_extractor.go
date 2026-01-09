package ratelimit

import (
	"log/slog"
	"strconv"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
)

// CostSourceType defines the type of source for cost extraction
type CostSourceType string

const (
	CostSourceResponseHeader CostSourceType = "response_header"
	CostSourceMetadata       CostSourceType = "metadata"
	CostSourceResponseBody   CostSourceType = "response_body"
)

// CostSource represents a single source for extracting cost
type CostSource struct {
	Type     CostSourceType // "response_header", "metadata", "response_body"
	Key      string         // Header name or metadata key
	JSONPath string         // For response_body type
}

// CostExtractionConfig holds the configuration for cost extraction
type CostExtractionConfig struct {
	Enabled bool
	Sources []CostSource
	Default int64
}

// CostExtractor handles extracting cost from response data
type CostExtractor struct {
	config CostExtractionConfig
}

// NewCostExtractor creates a new CostExtractor with the given configuration
func NewCostExtractor(config CostExtractionConfig) *CostExtractor {
	return &CostExtractor{config: config}
}

// ExtractCost tries to extract cost from the response using configured sources
// Returns (cost, extracted) where extracted indicates if a value was found
func (e *CostExtractor) ExtractCost(ctx *policy.ResponseContext) (int64, bool) {
	if !e.config.Enabled {
		return e.config.Default, false
	}

	// Try sources in order until one succeeds
	for _, source := range e.config.Sources {
		cost, ok := e.extractFromSource(ctx, source)
		if ok {
			slog.Debug("Cost extracted successfully",
				"type", source.Type,
				"key", source.Key,
				"jsonPath", source.JSONPath,
				"cost", cost)
			return cost, true
		}
	}

	// All sources failed, use default
	slog.Debug("All cost extraction sources failed, using default",
		"default", e.config.Default)
	return e.config.Default, false
}

// extractFromSource extracts cost from a single source
func (e *CostExtractor) extractFromSource(ctx *policy.ResponseContext, source CostSource) (int64, bool) {
	switch source.Type {
	case CostSourceResponseHeader:
		return e.extractFromHeader(ctx, source.Key)
	case CostSourceMetadata:
		return e.extractFromMetadata(ctx, source.Key)
	case CostSourceResponseBody:
		return e.extractFromBody(ctx, source.JSONPath)
	default:
		slog.Warn("Unknown cost source type", "type", source.Type)
		return 0, false
	}
}

// extractFromHeader extracts cost from a response header
func (e *CostExtractor) extractFromHeader(ctx *policy.ResponseContext, headerName string) (int64, bool) {
	if ctx.ResponseHeaders == nil {
		return 0, false
	}

	// Headers are case-insensitive
	values := ctx.ResponseHeaders.Get(strings.ToLower(headerName))
	if len(values) == 0 || values[0] == "" {
		return 0, false
	}

	cost, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		slog.Warn("Failed to parse cost from header",
			"header", headerName,
			"value", values[0],
			"error", err)
		return 0, false
	}

	if cost < 0 {
		slog.Warn("Negative cost value from header, treating as extraction failure",
			"header", headerName,
			"value", cost)
		return 0, false
	}

	return cost, true
}

// extractFromMetadata extracts cost from shared metadata
func (e *CostExtractor) extractFromMetadata(ctx *policy.ResponseContext, key string) (int64, bool) {
	val, ok := ctx.Metadata[key]
	if !ok {
		return 0, false
	}

	// Handle various numeric types that might be stored in metadata
	switch v := val.(type) {
	case int64:
		if v >= 0 {
			return v, true
		}
	case int:
		if v >= 0 {
			return int64(v), true
		}
	case float64:
		if v >= 0 {
			return int64(v), true
		}
	case string:
		cost, err := strconv.ParseInt(v, 10, 64)
		if err == nil && cost >= 0 {
			return cost, true
		}
		slog.Warn("Failed to parse cost from metadata string",
			"key", key,
			"value", v,
			"error", err)
	default:
		slog.Warn("Unsupported type for cost in metadata",
			"key", key,
			"type", "%T", v)
	}

	return 0, false
}

// extractFromBody extracts cost from response body using JSONPath
func (e *CostExtractor) extractFromBody(ctx *policy.ResponseContext, jsonPath string) (int64, bool) {
	if ctx.ResponseBody == nil || !ctx.ResponseBody.Present {
		return 0, false
	}

	bodyBytes := ctx.ResponseBody.Content
	if len(bodyBytes) == 0 {
		return 0, false
	}

	// Use the existing utils function for JSONPath extraction
	valueStr, err := utils.ExtractStringValueFromJsonpath(bodyBytes, jsonPath)
	if err != nil {
		slog.Debug("Failed to extract cost from response body",
			"jsonPath", jsonPath,
			"error", err)
		return 0, false
	}

	cost, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		slog.Warn("Failed to parse cost from response body",
			"jsonPath", jsonPath,
			"value", valueStr,
			"error", err)
		return 0, false
	}

	if cost < 0 {
		slog.Warn("Negative cost value from response body, treating as extraction failure",
			"jsonPath", jsonPath,
			"value", cost)
		return 0, false
	}

	return cost, true
}

// RequiresResponseBody returns true if any source requires response body access
func (e *CostExtractor) RequiresResponseBody() bool {
	if !e.config.Enabled {
		return false
	}
	for _, source := range e.config.Sources {
		if source.Type == CostSourceResponseBody {
			return true
		}
	}
	return false
}

// parseCostExtractionConfig parses the costExtraction configuration from parameters
func parseCostExtractionConfig(params map[string]interface{}) (*CostExtractionConfig, error) {
	costExtractionRaw, ok := params["costExtraction"]
	if !ok {
		return nil, nil // Not configured
	}

	costExtractionMap, ok := costExtractionRaw.(map[string]interface{})
	if !ok {
		return nil, nil // Invalid format, treat as not configured
	}

	config := &CostExtractionConfig{
		Enabled: false,
		Default: 1,
	}

	// Parse enabled
	if enabled, ok := costExtractionMap["enabled"].(bool); ok {
		config.Enabled = enabled
	}

	if !config.Enabled {
		return config, nil // Not enabled, no need to parse further
	}

	// Parse default
	if defaultVal, ok := costExtractionMap["default"].(float64); ok {
		config.Default = int64(defaultVal)
		if config.Default < 1 {
			config.Default = 1
		}
	}

	// Parse sources
	sourcesRaw, ok := costExtractionMap["sources"].([]interface{})
	if !ok || len(sourcesRaw) == 0 {
		// No sources configured but enabled - disable it
		config.Enabled = false
		return config, nil
	}

	config.Sources = make([]CostSource, 0, len(sourcesRaw))
	for _, sourceRaw := range sourcesRaw {
		sourceMap, ok := sourceRaw.(map[string]interface{})
		if !ok {
			continue
		}

		sourceType, ok := sourceMap["type"].(string)
		if !ok {
			continue
		}

		source := CostSource{
			Type: CostSourceType(sourceType),
		}

		if key, ok := sourceMap["key"].(string); ok {
			source.Key = key
		}

		if jsonPath, ok := sourceMap["jsonPath"].(string); ok {
			source.JSONPath = jsonPath
		}

		config.Sources = append(config.Sources, source)
	}

	if len(config.Sources) == 0 {
		config.Enabled = false
	}

	return config, nil
}

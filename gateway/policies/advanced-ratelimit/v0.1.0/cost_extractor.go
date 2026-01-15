package ratelimit

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
)

// CostSourceType defines the type of source for cost extraction
type CostSourceType string

const (
	// Request phase sources
	CostSourceRequestHeader   CostSourceType = "request_header"
	CostSourceRequestMetadata CostSourceType = "request_metadata"
	CostSourceRequestBody     CostSourceType = "request_body"

	// Response phase sources
	CostSourceResponseHeader   CostSourceType = "response_header"
	CostSourceResponseMetadata CostSourceType = "response_metadata"
	CostSourceResponseBody     CostSourceType = "response_body"
)

// CostSource represents a single source for extracting cost
type CostSource struct {
	Type       CostSourceType // source type
	Key        string         // Header name or metadata key
	JSONPath   string         // For body types: JSONPath expression
	Multiplier float64        // Multiplier for extracted value (default: 1.0)
}

// CostExtractionConfig holds the configuration for cost extraction
type CostExtractionConfig struct {
	Enabled bool
	Sources []CostSource
	Default float64 // Default cost if all sources fail
}

// CostExtractor handles extracting cost from request/response data
type CostExtractor struct {
	config CostExtractionConfig
}

// NewCostExtractor creates a new CostExtractor with the given configuration
func NewCostExtractor(config CostExtractionConfig) *CostExtractor {
	return &CostExtractor{config: config}
}

// GetConfig returns the cost extraction configuration
func (e *CostExtractor) GetConfig() CostExtractionConfig {
	return e.config
}

// ExtractRequestCost extracts cost from request-phase sources only.
// Returns (cost, extracted) where extracted indicates if any value was found.
// When multiple sources succeed, their values are summed (with multipliers applied).
func (e *CostExtractor) ExtractRequestCost(ctx *policy.RequestContext) (float64, bool) {
	if !e.config.Enabled {
		return e.config.Default, false
	}

	var total float64
	var found bool

	for _, source := range e.config.Sources {
		if !isRequestPhaseSource(source.Type) {
			continue
		}

		val, ok := e.extractFromRequestSource(ctx, source)
		if ok {
			found = true
			total += val * source.Multiplier
			slog.Debug("Request cost extracted from source",
				"type", source.Type,
				"key", source.Key,
				"jsonPath", source.JSONPath,
				"rawValue", val,
				"multiplier", source.Multiplier,
				"contribution", val*source.Multiplier)
		}
	}

	if !found {
		slog.Debug("All request cost extraction sources failed, using default",
			"default", e.config.Default)
		return e.config.Default, false
	}

	if total < 0 {
		slog.Warn("Total cost from request sources is negative; clamping to zero", "cost", total)
		total = 0
	}

	slog.Debug("Request cost extracted successfully", "totalCost", total)
	return total, true
}

// ExtractResponseCost extracts cost from response-phase sources only.
// Returns (cost, extracted) where extracted indicates if any value was found.
// When multiple sources succeed, their values are summed (with multipliers applied).
func (e *CostExtractor) ExtractResponseCost(ctx *policy.ResponseContext) (float64, bool) {
	if !e.config.Enabled {
		return e.config.Default, false
	}

	var total float64
	var found bool

	for _, source := range e.config.Sources {
		if !isResponsePhaseSource(source.Type) {
			continue
		}

		val, ok := e.extractFromResponseSource(ctx, source)
		if ok {
			found = true
			total += val * source.Multiplier
			slog.Debug("Response cost extracted from source",
				"type", source.Type,
				"key", source.Key,
				"jsonPath", source.JSONPath,
				"rawValue", val,
				"multiplier", source.Multiplier,
				"contribution", val*source.Multiplier)
		}
	}

	if !found {
		slog.Debug("All response cost extraction sources failed, using default",
			"default", e.config.Default)
		return e.config.Default, false
	}

	if total < 0 {
		slog.Warn("Total cost from response sources is negative; clamping to zero", "cost", total)
		total = 0
	}

	slog.Debug("Response cost extracted successfully", "totalCost", total)
	return total, true
}

// isRequestPhaseSource returns true if the source type is available during request phase
func isRequestPhaseSource(t CostSourceType) bool {
	switch t {
	case CostSourceRequestHeader, CostSourceRequestMetadata, CostSourceRequestBody:
		return true
	default:
		return false
	}
}

// isResponsePhaseSource returns true if the source type is available during response phase
func isResponsePhaseSource(t CostSourceType) bool {
	switch t {
	case CostSourceResponseHeader, CostSourceResponseMetadata, CostSourceResponseBody:
		return true
	default:
		return false
	}
}

// extractFromRequestSource extracts cost from a single request-phase source
func (e *CostExtractor) extractFromRequestSource(ctx *policy.RequestContext, source CostSource) (float64, bool) {
	switch source.Type {
	case CostSourceRequestHeader:
		return e.extractFromRequestHeader(ctx, source.Key)
	case CostSourceRequestMetadata:
		return e.extractFromRequestMetadata(ctx, source.Key)
	case CostSourceRequestBody:
		return e.extractFromRequestBody(ctx, source.JSONPath)
	default:
		return 0, false
	}
}

// extractFromResponseSource extracts cost from a single response-phase source
func (e *CostExtractor) extractFromResponseSource(ctx *policy.ResponseContext, source CostSource) (float64, bool) {
	switch source.Type {
	case CostSourceResponseHeader:
		return e.extractFromResponseHeader(ctx, source.Key)
	case CostSourceResponseMetadata:
		return e.extractFromResponseMetadata(ctx, source.Key)
	case CostSourceResponseBody:
		return e.extractFromResponseBody(ctx, source.JSONPath)
	default:
		return 0, false
	}
}

// extractFromRequestHeader extracts cost from a request header
func (e *CostExtractor) extractFromRequestHeader(ctx *policy.RequestContext, headerName string) (float64, bool) {
	if ctx.Headers == nil {
		return 0, false
	}

	values := ctx.Headers.Get(strings.ToLower(headerName))
	if len(values) == 0 || values[0] == "" {
		return 0, false
	}

	cost, err := strconv.ParseFloat(values[0], 64)
	if err != nil {
		slog.Warn("Failed to parse cost from request header",
			"header", headerName,
			"value", values[0],
			"error", err)
		return 0, false
	}

	return cost, true
}

// extractFromRequestMetadata extracts cost from request metadata
func (e *CostExtractor) extractFromRequestMetadata(ctx *policy.RequestContext, key string) (float64, bool) {
	return extractFromMetadataMap(ctx.Metadata, key)
}

// extractFromRequestBody extracts cost from request body using JSONPath
func (e *CostExtractor) extractFromRequestBody(ctx *policy.RequestContext, jsonPath string) (float64, bool) {
	if ctx.Body == nil || !ctx.Body.Present {
		return 0, false
	}

	return extractFromBodyBytes(ctx.Body.Content, jsonPath)
}

// extractFromResponseHeader extracts cost from a response header
func (e *CostExtractor) extractFromResponseHeader(ctx *policy.ResponseContext, headerName string) (float64, bool) {
	if ctx.ResponseHeaders == nil {
		return 0, false
	}

	values := ctx.ResponseHeaders.Get(strings.ToLower(headerName))
	if len(values) == 0 || values[0] == "" {
		return 0, false
	}

	cost, err := strconv.ParseFloat(values[0], 64)
	if err != nil {
		slog.Warn("Failed to parse cost from response header",
			"header", headerName,
			"value", values[0],
			"error", err)
		return 0, false
	}

	return cost, true
}

// extractFromResponseMetadata extracts cost from response metadata
func (e *CostExtractor) extractFromResponseMetadata(ctx *policy.ResponseContext, key string) (float64, bool) {
	return extractFromMetadataMap(ctx.Metadata, key)
}

// extractFromResponseBody extracts cost from response body using JSONPath
func (e *CostExtractor) extractFromResponseBody(ctx *policy.ResponseContext, jsonPath string) (float64, bool) {
	if ctx.ResponseBody == nil || !ctx.ResponseBody.Present {
		return 0, false
	}

	return extractFromBodyBytes(ctx.ResponseBody.Content, jsonPath)
}

// extractFromMetadataMap is a helper to extract cost from a metadata map
func extractFromMetadataMap(metadata map[string]interface{}, key string) (float64, bool) {
	val, ok := metadata[key]
	if !ok {
		return 0, false
	}

	switch v := val.(type) {
	case int64:
		return float64(v), true
	case int:
		return float64(v), true
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case string:
		cost, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return cost, true
		}
		slog.Warn("Failed to parse cost from metadata string",
			"key", key,
			"value", v,
			"error", err)
	default:
		slog.Warn("Unsupported type for cost in metadata",
			"key", key,
			"type", fmt.Sprintf("%T", val))
	}

	return 0, false
}

// extractFromBodyBytes is a helper to extract cost from body bytes using JSONPath
func extractFromBodyBytes(bodyBytes []byte, jsonPath string) (float64, bool) {
	if len(bodyBytes) == 0 {
		return 0, false
	}

	valueStr, err := utils.ExtractStringValueFromJsonpath(bodyBytes, jsonPath)
	if err != nil {
		slog.Debug("Failed to extract cost from body",
			"jsonPath", jsonPath,
			"error", err)
		return 0, false
	}

	cost, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		slog.Warn("Failed to parse cost from body",
			"jsonPath", jsonPath,
			"value", valueStr,
			"error", err)
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

// RequiresRequestBody returns true if any source requires request body access
func (e *CostExtractor) RequiresRequestBody() bool {
	if !e.config.Enabled {
		return false
	}
	for _, source := range e.config.Sources {
		if source.Type == CostSourceRequestBody {
			return true
		}
	}
	return false
}

// HasRequestPhaseSources returns true if any source is available during request phase
func (e *CostExtractor) HasRequestPhaseSources() bool {
	if !e.config.Enabled {
		return false
	}
	for _, source := range e.config.Sources {
		if isRequestPhaseSource(source.Type) {
			return true
		}
	}
	return false
}

// HasResponsePhaseSources returns true if any source is available during response phase
func (e *CostExtractor) HasResponsePhaseSources() bool {
	if !e.config.Enabled {
		return false
	}
	for _, source := range e.config.Sources {
		if isResponsePhaseSource(source.Type) {
			return true
		}
	}
	return false
}

// parseCostExtractionConfig parses the costExtraction configuration from a raw value
// which should be a map[string]interface{} from either quota["costExtraction"] or legacy params["costExtraction"].
func parseCostExtractionConfig(raw interface{}) (*CostExtractionConfig, error) {
	if raw == nil {
		return nil, nil
	}

	costExtractionMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, nil // invalid format, treat as not configured
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
		config.Default = defaultVal
		if config.Default < 0 {
			config.Default = 0
		}
	} else if defaultVal, ok := costExtractionMap["default"].(int); ok {
		config.Default = float64(defaultVal)
		if config.Default < 0 {
			config.Default = 0
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
	for i, sourceRaw := range sourcesRaw {
		sourceMap, ok := sourceRaw.(map[string]interface{})
		if !ok {
			continue
		}

		sourceType, ok := sourceMap["type"].(string)
		if !ok {
			continue
		}

		source := CostSource{
			Type:       CostSourceType(sourceType),
			Multiplier: 1.0, // default multiplier
		}

		if key, ok := sourceMap["key"].(string); ok {
			source.Key = key
		}

		if jsonPath, ok := sourceMap["jsonPath"].(string); ok {
			source.JSONPath = jsonPath
		}

		// Parse multiplier
		if mult, ok := sourceMap["multiplier"].(float64); ok {
			if mult < 0 {
				return nil, fmt.Errorf("sources[%d].multiplier must be non-negative, got %v", i, mult)
			}
			source.Multiplier = mult
		} else if mult, ok := sourceMap["multiplier"].(int); ok {
			if mult < 0 {
				return nil, fmt.Errorf("sources[%d].multiplier must be non-negative, got %v", i, mult)
			}
			source.Multiplier = float64(mult)
		}

		config.Sources = append(config.Sources, source)
	}

	if len(config.Sources) == 0 {
		config.Enabled = false
	}

	return config, nil
}

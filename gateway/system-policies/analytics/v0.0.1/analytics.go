package analytics

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	"github.com/wso2/api-platform/sdk/utils"
)

const (
	KindAsyncsse       = "async/sse"
	KindAsyncwebsocket = "async/websocket"
	KindAsyncwebsub    = "async/websub"
	KindRestApi        = "RestApi"
	KindLlmProvider    = "LlmProvider"
	KindMCP            = "Mcp"

	// Analytics metadata keys for LLM token information
	// These match the keys defined in policy-engine/internal/analytics/analytics.go
	PromptTokenCountMetadataKey      = "aitoken:prompttokencount"
	CompletionTokenCountMetadataKey  = "aitoken:completiontokencount"
	TotalTokenCountMetadataKey       = "aitoken:totaltokencount"
	ModelIDMetadataKey               = "aitoken:modelid"
	AIProviderNameMetadataKey        = "ai:providername"
	AIProviderDisplayNameMetadataKey = "ai:providerdisplayname"
)

var (
	// JSON Path expressions to extract MCP analytics properties from response body
	JsonRpcMethodJsonPath     = "$.method"
	McpCapabilityNameJsonPath = "$.params.name"
	McpResourceUriJsonPath    = "$.params.uri"
	ProtocolVersionJsonPath   = "$.params.protocolVersion"
	ClientNameJsonPath        = "$.params.clientInfo.name"
	ClientVersionJsonPath     = "$.params.clientInfo.version"
	JsonRpcErrorCodeJsonPath  = "$.error.code"
)

// AnalyticsPolicy implements the default analytics data collection process.
type AnalyticsPolicy struct{}

// LLMTokenInfo holds extracted token-related information from LLM provider responses
type LLMProviderAnalyticsInfo struct {
	ProviderName        *string // Provider name
	ProviderDisplayName *string // Provider display name
	PromptTokens        *int64  // Number of prompt tokens
	CompletionTokens    *int64  // Number of completion tokens
	TotalTokens         *int64  // Total number of tokens
	RemainingTokens     *int64  // Remaining tokens from rate limit headers
	RequestModel        *string // Model name from request
	ResponseModel       *string // Model name from response
}

var ins = &AnalyticsPolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// Mode returns the processing mode for this policy
func (a *AnalyticsPolicy) Mode() policy.ProcessingMode {
	// For now analytics will go through all the headers and body to collect the analytics data.
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// OnRequest performs Analytics collection process during the request phase
func (a *AnalyticsPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	slog.Info("Analytics system policy: OnRequest called")
	// Extract common analytics data from the request
	// Based on the API kind, collect the analytics data
	apiKind := ctx.SharedContext.APIKind
	switch apiKind {
	case KindRestApi:
		// Collect analytics data for REST API scenario
	case KindLlmProvider:
		// Collect analytics data for AI API(LLM Provider) specific scenario
		// Based on the json paths provided the the template, extract the token count data
	case KindMCP:
		// Collect analytics data specific for MCP scenario from request
		// Currently no data is collected
	default:
		slog.Error("Invalid API kind")
	}
	return nil
}

// OnRequest performs Analytics collection process during the response phase
func (p *AnalyticsPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	slog.Info("Analytics system policy: OnResponse called")
	// Store tokenInfo in analytics metadata for publishing
	analyticsMetadata := make(map[string]any)

	// Based on the API kind, collect the analytics data
	apiKind := ctx.SharedContext.APIKind
	slog.Debug("API kind: ", "apiKind", apiKind)
	switch apiKind {
	case KindRestApi:
		// Collect analytics data for REST API spcific scenario
	case KindLlmProvider:
		// Collect the analytics data for the AI API(LLM Provider) specific scenario
		providerTemplate := params["providerTemplate"]
		slog.Debug("Provider template param from policy: ", "providerTemplate", providerTemplate)
		if providerTemplate != nil {
			tokenInfo, err := extractLLMProviderAnalyticsInfo(providerTemplate, ctx)
			if err != nil {
				slog.Warn("Failed to extract LLM token info", "error", err)
			} else if tokenInfo != nil {
				slog.Info("Extracted LLM token info",
					"promptTokens", tokenInfo.PromptTokens,
					"completionTokens", tokenInfo.CompletionTokens,
					"totalTokens", tokenInfo.TotalTokens,
					"remainingTokens", tokenInfo.RemainingTokens,
					"requestModel", tokenInfo.RequestModel,
					"responseModel", tokenInfo.ResponseModel,
					"providerName", tokenInfo.ProviderName,
					"providerDisplayName", tokenInfo.ProviderDisplayName,
				)

				// Token-related metadata
				if tokenInfo.PromptTokens != nil {
					analyticsMetadata[PromptTokenCountMetadataKey] = strconv.FormatInt(*tokenInfo.PromptTokens, 10)
				}
				if tokenInfo.CompletionTokens != nil {
					analyticsMetadata[CompletionTokenCountMetadataKey] = strconv.FormatInt(*tokenInfo.CompletionTokens, 10)
				}
				if tokenInfo.TotalTokens != nil {
					analyticsMetadata[TotalTokenCountMetadataKey] = strconv.FormatInt(*tokenInfo.TotalTokens, 10)
				}
				if tokenInfo.ResponseModel != nil {
					analyticsMetadata[ModelIDMetadataKey] = *tokenInfo.ResponseModel
				} else if tokenInfo.RequestModel != nil {
					// Fallback to request model if response model is not available
					analyticsMetadata[ModelIDMetadataKey] = *tokenInfo.RequestModel
				}
				if tokenInfo.ProviderName != nil {
					analyticsMetadata[AIProviderNameMetadataKey] = *tokenInfo.ProviderName
				}
				if tokenInfo.ProviderDisplayName != nil {
					analyticsMetadata[AIProviderDisplayNameMetadataKey] = *tokenInfo.ProviderDisplayName
				}

				// Return modifications with analytics metadata
				if len(analyticsMetadata) > 0 {
					return policy.UpstreamResponseModifications{
						AnalyticsMetadata: analyticsMetadata,
					}
				}
			}
		}
	case KindMCP:
		// Collect the analytics data specific for MCP specific scenario
		// Currently no data is collected
	default:
		slog.Error("Invalid API kind")
	}

	// Return modifications with analytics metadata (including headers if available)
	if len(analyticsMetadata) > 0 {
		return policy.UpstreamResponseModifications{
			AnalyticsMetadata: analyticsMetadata,
		}
	}

	return nil
}

// extractLLMTokenInfo extracts the LLM token information from the response and request bodies
func extractLLMProviderAnalyticsInfo(template interface{}, ctx *policy.ResponseContext) (*LLMProviderAnalyticsInfo, error) {
	templateMap, ok := template.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("template is not a map")
	}

	spec, ok := templateMap["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("template spec is not a map")
	}

	// Parse the response and request bodies
	var responseJSON map[string]interface{}
	if ctx.ResponseBody != nil && ctx.ResponseBody.Content != nil {
		_ = json.Unmarshal(ctx.ResponseBody.Content, &responseJSON)
	}

	var requestJSON map[string]interface{}
	if ctx.RequestBody != nil && ctx.RequestBody.Content != nil {
		_ = json.Unmarshal(ctx.RequestBody.Content, &requestJSON)
	}

	info := &LLMProviderAnalyticsInfo{}

	// Helper closure
	extract := func(field string, fromRequest bool) (interface{}, error) {
		fieldCfg, ok := spec[field].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("field %s not found", field)
		}

		location, _ := fieldCfg["location"].(string)
		identifier, _ := fieldCfg["identifier"].(string)
		if identifier == "" {
			return nil, fmt.Errorf("identifier missing for %s", field)
		}

		switch strings.ToLower(location) {
		case "payload":
			var src map[string]interface{}
			if fromRequest {
				src = requestJSON
			} else {
				src = responseJSON
			}
			if src == nil {
				return nil, fmt.Errorf("payload not available for %s", field)
			}
			return utils.ExtractValueFromJsonpath(src, identifier)
		case "header":
			if fromRequest {
				if ctx.RequestHeaders == nil {
					return nil, fmt.Errorf("request headers missing")
				}
				if v := ctx.RequestHeaders.Get(identifier); len(v) > 0 {
					return v[0], nil
				}
			} else {
				if ctx.ResponseHeaders == nil {
					return nil, fmt.Errorf("response headers missing")
				}
				if v := ctx.ResponseHeaders.Get(identifier); len(v) > 0 {
					return v[0], nil
				}
			}
			return nil, fmt.Errorf("header %s not found", identifier)
		default:
			return nil, fmt.Errorf("unsupported location %s", location)
		}
	}

	// Extract numeric fields
	if v, err := extract("promptTokens", false); err == nil {
		if i, err := convertToInt64(v); err == nil {
			info.PromptTokens = &i
		}
	}
	if v, err := extract("completionTokens", false); err == nil {
		if i, err := convertToInt64(v); err == nil {
			info.CompletionTokens = &i
		}
	}
	if v, err := extract("totalTokens", false); err == nil {
		if i, err := convertToInt64(v); err == nil {
			info.TotalTokens = &i
		}
	}
	if v, err := extract("remainingTokens", false); err == nil {
		if i, err := convertToInt64(v); err == nil {
			info.RemainingTokens = &i
		}
	}
	// Extract model fields
	if v, err := extract("requestModel", true); err == nil {
		if s, ok := v.(string); ok {
			info.RequestModel = &s
		}
	}
	if v, err := extract("responseModel", false); err == nil {
		if s, ok := v.(string); ok {
			info.ResponseModel = &s
		}
	}

	if md, ok := templateMap["metadata"].(map[string]interface{}); ok {
		if nameVal, ok := md["name"].(string); ok && strings.TrimSpace(nameVal) != "" {
			info.ProviderName = &nameVal
		}
	}

	if displayName, ok := spec["displayName"].(string); ok && strings.TrimSpace(displayName) != "" {
		// If ai:providername not set yet, fall back to displayName
		if info.ProviderName == nil {
			info.ProviderName = &displayName
		}
		// Also expose display name separately for potential consumers
		info.ProviderDisplayName = &displayName
	}

	return info, nil
}

// convertToInt64 converts various numeric types to int64
func convertToInt64(value interface{}) (int64, error) {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(v.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return int64(v.Float()), nil
	case reflect.String:
		s := v.String()
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i, nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f), nil
		}
		return 0, fmt.Errorf("cannot convert string %q to int64", s)
	default:
		return 0, fmt.Errorf("cannot convert type %T to int64", value)
	}
}

package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	"github.com/wso2/api-platform/sdk/core/utils"
)

const (
	// API Kinds
	KindAsyncsse       = "async/sse"
	KindAsyncwebsocket = "async/websocket"
	KindAsyncwebsub    = "async/websub"
	KindRestApi        = "RestApi"
	KindLlmProvider    = "LlmProvider"
	KindLlmProxy       = "LlmProxy"
	KindMCP            = "Mcp"

	// Analytics metadata keys for LLM token information
	// These match the keys defined in policy-engine/internal/analytics/analytics.go
	PromptTokenCountMetadataKey      = "aitoken:prompttokencount"
	CompletionTokenCountMetadataKey  = "aitoken:completiontokencount"
	TotalTokenCountMetadataKey       = "aitoken:totaltokencount"
	ModelIDMetadataKey               = "aitoken:modelid"
	AIProviderNameMetadataKey        = "ai:providername"
	AIProviderDisplayNameMetadataKey = "ai:providerdisplayname"
	ApplicationIDMetadataKey         = "x-wso2-application-id"
	ApplicationNameMetadataKey       = "x-wso2-application-name"

	// Lazy resource type for LLM provider templates
	lazyResourceTypeLLMProviderTemplate = "LlmProviderTemplate"
	// Lazy resource type for provider-to-template mapping
	lazyResourceTypeProviderTemplateMapping = "ProviderTemplateMapping"

	// SharedContext.Metadata key used to accumulate streaming response body chunks.
	// Deleted after EndOfStream processing to avoid memory leaks.
	analyticsStreamAccKey = "__analytics_stream_acc"
)

var (
	// JSON Path expressions to extract MCP analytics properties from response body
	JsonRpcMethodJsonPath     = "$.method"
	McpCapabilityNameJsonPath = "$.params.name"
	McpResourceUriJsonPath    = "$.params.uri"
	ProtocolVersionJsonPath   = "$.params.protocolVersion"
	ClientNameJsonPath        = "$.params.clientInfo.name"
	ClientVersionJsonPath     = "$.params.clientInfo.version"

	ServerProtocolVersionJsonPath = "$.result.protocolVersion"
	ServerInfoNameJsonPath        = "$.result.serverInfo.name"
	ServerInfoVersionJsonPath     = "$.result.serverInfo.version"
	IsErrorJsonPath               = "$.result.IsError"
	JsonRpcErrorCodeJsonPath      = "$.error.code"
)

// AnalyticsPolicy implements the default analytics data collection process.
type AnalyticsPolicy struct{}

type McpRequestAnalyticsProperties struct {
	JsonRpcMethod  string         `json:"jsonRpcMethod,omitempty"`
	Capability     string         `json:"capability,omitempty"`
	CapabilityName string         `json:"capabilityName,omitempty"`
	ClientInfo     *McpClientInfo `json:"clientInfo,omitempty"`
}

type McpClientInfo struct {
	RequestedProtocolVersion string `json:"requestedProtocolVersion"`
	Name                     string `json:"name"`
	Version                  string `json:"version"`
}

type McpServerInfo struct {
	ProtocolVersion string `json:"protocolVersion,omitempty"`
	Name            string `json:"name,omitempty"`
	Version         string `json:"version,omitempty"`
}

type McpResponseAnalyticsProperties struct {
	IsError    *bool          `json:"isError,omitempty"`
	ErrorCode  *int           `json:"errorCode,omitempty"`
	ServerInfo *McpServerInfo `json:"serverInfo,omitempty"`
}

// LLMProviderAnalyticsInfo holds extracted token-related information from LLM provider responses
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

// GetPolicyV2 is an alias for GetPolicy, provided for compatibility with the
// Builder-generated plugin registry which calls GetPolicyV2 on all plugins.
func GetPolicyV2(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return GetPolicy(metadata, params)
}

// Mode returns the processing mode for this policy.
// ResponseBodyMode is BodyModeStream so the kernel keeps streaming enabled when
// all other policies in the chain also support streaming. The buffered fallback
// (OnResponseBody) is still called when the chain cannot stream.
func (a *AnalyticsPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeStream,
	}
}

// OnRequestBody performs Analytics collection process during the request phase (buffered).
func (a *AnalyticsPolicy) OnRequestBody(_ context.Context, ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	slog.Debug("Analytics system policy: OnRequestBody called")
	allowPayloads := getAllowPayloadsFlag(params)
	analyticsMetadata := make(map[string]any)

	// When allow_payloads is enabled, capture the raw request body into analytics metadata.
	if allowPayloads && ctx != nil && ctx.Body != nil && len(ctx.Body.Content) > 0 {
		slog.Debug("Capturing request payload for analytics")
		analyticsMetadata["request_payload"] = string(ctx.Body.Content)
	}

	apiKind := ctx.SharedContext.APIKind
	switch apiKind {
	case KindRestApi:
		// Collect analytics data for REST API scenario
	case KindLlmProvider:
		// Collect analytics data for AI API(LLM Provider) specific scenario
	case KindLlmProxy:
		// Collect analytics data for LLM Proxy specific scenario
	case KindMCP:
		// Collect analytics data specific for MCP scenario from request
		if ctx.Headers != nil && len(ctx.Headers.GetAll()) > 0 {
			sessionIDs := ctx.Headers.Get("mcp-session-id")
			if len(sessionIDs) > 0 {
				analyticsMetadata["mcp_session_id"] = sessionIDs[0]
			}
		}

		if ctx != nil && ctx.Body != nil && len(ctx.Body.Content) > 0 {
			var mcpPayload map[string]interface{}
			if err := json.Unmarshal(ctx.Body.Content, &mcpPayload); err != nil {
				slog.Error("Failed to unmarshal MCP request body for analytics", "error", err)
				break
			}

			props := McpRequestAnalyticsProperties{}

			extractString := func(path string) string {
				val, err := utils.ExtractValueFromJsonpath(mcpPayload, path)
				if err != nil || val == nil {
					return ""
				}
				if s, ok := val.(string); ok {
					return s
				}
				return ""
			}

			props.JsonRpcMethod = extractString(JsonRpcMethodJsonPath)
			props.CapabilityName = extractString(McpCapabilityNameJsonPath)
			props.Capability = extractString(McpResourceUriJsonPath)

			clientInfo := McpClientInfo{
				RequestedProtocolVersion: extractStringFromJsonpath(mcpPayload, ProtocolVersionJsonPath),
				Name:                     extractStringFromJsonpath(mcpPayload, ClientNameJsonPath),
				Version:                  extractStringFromJsonpath(mcpPayload, ClientVersionJsonPath),
			}
			if clientInfo.RequestedProtocolVersion != "" || clientInfo.Name != "" || clientInfo.Version != "" {
				props.ClientInfo = &clientInfo
			}

			if data, err := json.Marshal(props); err != nil {
				slog.Error("Failed to marshal MCP request analytics properties", "error", err)
			} else {
				analyticsMetadata["mcp_request_properties"] = string(data)
			}
		}
	default:
		slog.Error("Invalid API kind")
	}

	if len(analyticsMetadata) > 0 {
		return policy.UpstreamRequestModifications{
			AnalyticsMetadata: analyticsMetadata,
		}
	}
	return nil
}

// OnResponseBody performs Analytics collection during the response phase (buffered fallback).
// Called when the chain is in buffered mode (e.g. another policy does not support streaming).
func (a *AnalyticsPolicy) OnResponseBody(_ context.Context, ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	slog.Debug("Analytics system policy: OnResponseBody called")
	allowPayloads := getAllowPayloadsFlag(params)

	analyticsMetadata := make(map[string]any)

	for authCtx := ctx.SharedContext.AuthContext; authCtx != nil; authCtx = authCtx.Previous {
		if authCtx.Authenticated && authCtx.Subject != "" {
			analyticsMetadata["x-wso2-user-id"] = authCtx.Subject
			slog.Debug("Analytics system policy: User ID extracted from AuthContext",
				"subject", authCtx.Subject,
				"authType", authCtx.AuthType,
			)
			break
		}
	}

	apiKind := ctx.SharedContext.APIKind
	slog.Debug("API kind: ", "apiKind", apiKind)
	switch apiKind {
	case KindRestApi:
		// Collect analytics data for REST API specific scenario
	case KindLlmProvider, KindLlmProxy:
		templateHandle, ok := ctx.SharedContext.Metadata["template_handle"].(string)
		slog.Info("Template handle(extracted from route metadata): ", "templateHandle", templateHandle)
		if !ok || templateHandle == "" {
			slog.Debug("No template handle found in route metadata for LLM API")
		} else {
			template, err := getTemplateByHandle(templateHandle)
			if err != nil {
				slog.Warn("Failed to get template from lazy resource cache", "templateHandle", templateHandle, "error", err)
			} else {
				tokenInfo, err := extractLLMProviderAnalyticsInfo(template, ctx)
				if err != nil {
					slog.Warn("Failed to extract LLM token info", "error", err)
				} else if tokenInfo != nil {
					slog.Debug("Extracted LLM token info",
						"promptTokens", tokenInfo.PromptTokens,
						"completionTokens", tokenInfo.CompletionTokens,
						"totalTokens", tokenInfo.TotalTokens,
						"remainingTokens", tokenInfo.RemainingTokens,
						"requestModel", tokenInfo.RequestModel,
						"responseModel", tokenInfo.ResponseModel,
						"providerName", tokenInfo.ProviderName,
						"providerDisplayName", tokenInfo.ProviderDisplayName,
					)
					populateTokenAnalyticsMetadata(analyticsMetadata, tokenInfo)
				}
			}
		}
	case KindMCP:
		if ctx.ResponseHeaders != nil && len(ctx.ResponseHeaders.GetAll()) > 0 {
			if analyticsMetadata["mcp_session_id"] == nil {
				sessionIDs := ctx.ResponseHeaders.Get("mcp-session-id")
				if len(sessionIDs) > 0 {
					analyticsMetadata["mcp_session_id"] = sessionIDs[0]
				}
			}
		}

		if ctx != nil && ctx.ResponseBody != nil && len(ctx.ResponseBody.Content) > 0 {
			responseContent := ctx.ResponseBody.Content

			isSSE := isSSEContent(ctx.ResponseHeaders, responseContent)
			if isSSE {
				jsonData, err := parseSSEFirstDataEvent(responseContent)
				if err != nil {
					slog.Error("Failed to parse SSE response", "error", err)
				} else {
					responseContent = jsonData
				}
			}

			trimmed := bytes.TrimSpace(responseContent)
			if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
				var mcpResponsePayload map[string]interface{}
				if err := json.Unmarshal(trimmed, &mcpResponsePayload); err != nil {
					slog.Warn("Failed to unmarshal MCP response body for server info analytics", "error", err)
				} else {
					props := extractMCPResponseAnalyticsProps(mcpResponsePayload)
					if props != nil {
						if data, err := json.Marshal(props); err != nil {
							slog.Error("Failed to marshal MCP response analytics properties", "error", err)
						} else {
							analyticsMetadata["mcp_response_properties"] = string(data)
						}
					}
				}
			}
		}
	default:
		slog.Error("Invalid API kind")
	}

	if allowPayloads {
		if ctx != nil && ctx.ResponseBody != nil && len(ctx.ResponseBody.Content) > 0 {
			slog.Debug("Capturing response payload for analytics")
			analyticsMetadata["response_payload"] = string(ctx.ResponseBody.Content)
		}
	}

	if len(analyticsMetadata) > 0 {
		return policy.DownstreamResponseModifications{
			AnalyticsMetadata: analyticsMetadata,
		}
	}
	return nil
}

// OnResponseBodyChunk handles streaming response body chunks.
// Chunks are accumulated in SharedContext.Metadata. On EndOfStream the accumulated
// bytes are parsed and analytics metadata is emitted on the final ResponseChunkAction.
func (a *AnalyticsPolicy) OnResponseBodyChunk(_ context.Context, ctx *policy.ResponseStreamContext, chunk *policy.StreamBody, params map[string]interface{}) policy.ResponseChunkAction {
	slog.Debug("Analytics system policy: OnResponseBodyChunk called")
	if ctx.SharedContext.Metadata == nil {
		ctx.SharedContext.Metadata = make(map[string]interface{})
	}

	if len(chunk.Chunk) > 0 {
		acc, _ := ctx.SharedContext.Metadata[analyticsStreamAccKey].([]byte)
		ctx.SharedContext.Metadata[analyticsStreamAccKey] = append(acc, chunk.Chunk...)
	}

	if !chunk.EndOfStream {
		return policy.ResponseChunkAction{}
	}

	// EndOfStream: consume accumulated bytes and emit analytics.
	accumulated, _ := ctx.SharedContext.Metadata[analyticsStreamAccKey].([]byte)
	delete(ctx.SharedContext.Metadata, analyticsStreamAccKey)

	analyticsMetadata := make(map[string]any)

	for authCtx := ctx.SharedContext.AuthContext; authCtx != nil; authCtx = authCtx.Previous {
		if authCtx.Authenticated && authCtx.Subject != "" {
			analyticsMetadata["x-wso2-user-id"] = authCtx.Subject
			break
		}
	}

	apiKind := ctx.SharedContext.APIKind
	switch apiKind {
	case KindRestApi:
		// No body analytics for REST API
	case KindLlmProvider, KindLlmProxy:
		templateHandle, ok := ctx.SharedContext.Metadata["template_handle"].(string)
		if ok && templateHandle != "" {
			template, err := getTemplateByHandle(templateHandle)
			if err != nil {
				slog.Warn("Failed to get template from lazy resource cache (streaming)", "templateHandle", templateHandle, "error", err)
			} else {
				var requestBodyBytes []byte
				if ctx.RequestBody != nil {
					requestBodyBytes = ctx.RequestBody.Content
				}
				// Streaming responses are SSE; the last data event carries usage fields.
				tokenInfo, err := extractLLMProviderAnalyticsInfoFromBytes(
					template, ctx.RequestHeaders, ctx.ResponseHeaders,
					requestBodyBytes, accumulated,
				)
				if err != nil {
					slog.Warn("Failed to extract LLM token info from streaming response", "error", err)
				} else if tokenInfo != nil {
					populateTokenAnalyticsMetadata(analyticsMetadata, tokenInfo)
				}
			}
		}
	case KindMCP:
		if ctx.ResponseHeaders != nil {
			sessionIDs := ctx.ResponseHeaders.Get("mcp-session-id")
			if len(sessionIDs) > 0 {
				analyticsMetadata["mcp_session_id"] = sessionIDs[0]
			}
		}

		if len(accumulated) > 0 {
			mcpPayload := extractMCPPayloadFromAccumulated(accumulated, ctx.ResponseHeaders)
			if mcpPayload != nil {
				props := extractMCPResponseAnalyticsProps(mcpPayload)
				if props != nil {
					if data, err := json.Marshal(props); err != nil {
						slog.Error("Failed to marshal MCP response analytics properties (streaming)", "error", err)
					} else {
						analyticsMetadata["mcp_response_properties"] = string(data)
					}
				}
			}
		}
	default:
		slog.Debug("Analytics streaming: unhandled API kind", "kind", apiKind)
	}

	if getAllowPayloadsFlag(params) && len(accumulated) > 0 {
		analyticsMetadata["response_payload"] = string(accumulated)
	}

	if len(analyticsMetadata) == 0 {
		return policy.ResponseChunkAction{}
	}
	return policy.ResponseChunkAction{AnalyticsMetadata: analyticsMetadata}
}

// NeedsMoreResponseData always returns false: each chunk is processed immediately
// and analytics data is accumulated internally in SharedContext.Metadata.
func (a *AnalyticsPolicy) NeedsMoreResponseData(accumulated []byte) bool {
	return false
}

// getTemplateByHandle retrieves a template from the lazy resource cache by its handle
func getTemplateByHandle(templateHandle string) (map[string]interface{}, error) {
	if templateHandle == "" {
		return nil, fmt.Errorf("template handle is empty")
	}

	store := policy.GetLazyResourceStoreInstance()
	if store == nil {
		return nil, fmt.Errorf("lazy resource store is not available")
	}

	resource, err := store.GetResourceByIDAndType(templateHandle, lazyResourceTypeLLMProviderTemplate)
	if err != nil {
		return nil, fmt.Errorf(
			"template with handle '%s' and type '%s' not found in cache: %w",
			templateHandle,
			lazyResourceTypeLLMProviderTemplate,
			err,
		)
	}

	if resource.Resource == nil {
		return nil, fmt.Errorf("template resource data is nil for handle '%s'", templateHandle)
	}

	return resource.Resource, nil
}

// extractLLMProviderAnalyticsInfo extracts LLM analytics from a buffered ResponseContext.
// For SSE content (buffered from a non-streaming path) the last data event is used.
func extractLLMProviderAnalyticsInfo(template map[string]interface{}, ctx *policy.ResponseContext) (*LLMProviderAnalyticsInfo, error) {
	var responseBodyBytes []byte
	if ctx.ResponseBody != nil {
		responseBodyBytes = ctx.ResponseBody.Content
	}

	var requestBodyBytes []byte
	if ctx.RequestBody != nil {
		requestBodyBytes = ctx.RequestBody.Content
	}

	return extractLLMProviderAnalyticsInfoFromBytes(
		template, ctx.RequestHeaders, ctx.ResponseHeaders,
		requestBodyBytes, responseBodyBytes,
	)
}

// extractLLMProviderAnalyticsInfoFromBytes extracts LLM analytics from raw byte slices.
// responseBodyBytes may be a full JSON body or accumulated SSE chunks; SSE is detected
// automatically and the last data event is used for token extraction.
func extractLLMProviderAnalyticsInfoFromBytes(
	template map[string]interface{},
	requestHeaders, responseHeaders *policy.Headers,
	requestBodyBytes, responseBodyBytes []byte,
) (*LLMProviderAnalyticsInfo, error) {
	var responseJSON map[string]interface{}
	if len(responseBodyBytes) > 0 {
		// SSE responses: find the last data event which carries usage fields.
		if jsonData, err := parseSSELastDataEvent(responseBodyBytes); err == nil {
			_ = json.Unmarshal(jsonData, &responseJSON)
		} else {
			// Plain JSON response.
			_ = json.Unmarshal(responseBodyBytes, &responseJSON)
		}
	}

	var requestJSON map[string]interface{}
	if len(requestBodyBytes) > 0 {
		_ = json.Unmarshal(requestBodyBytes, &requestJSON)
	}

	return extractLLMAnalyticsFromJSON(template, requestHeaders, responseHeaders, requestJSON, responseJSON)
}

// extractLLMAnalyticsFromJSON is the core extraction logic operating on pre-parsed JSON maps.
func extractLLMAnalyticsFromJSON(
	template map[string]interface{},
	requestHeaders, responseHeaders *policy.Headers,
	requestJSON, responseJSON map[string]interface{},
) (*LLMProviderAnalyticsInfo, error) {
	if template == nil {
		return nil, fmt.Errorf("template is nil")
	}

	spec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("template spec is not a map")
	}

	info := &LLMProviderAnalyticsInfo{}

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
				if requestHeaders == nil {
					return nil, fmt.Errorf("request headers missing")
				}
				if v := requestHeaders.Get(identifier); len(v) > 0 {
					return v[0], nil
				}
			} else {
				if responseHeaders == nil {
					return nil, fmt.Errorf("response headers missing")
				}
				if v := responseHeaders.Get(identifier); len(v) > 0 {
					return v[0], nil
				}
			}
			return nil, fmt.Errorf("header %s not found", identifier)
		default:
			return nil, fmt.Errorf("unsupported location %s", location)
		}
	}

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

	if md, ok := template["metadata"].(map[string]interface{}); ok {
		if nameVal, ok := md["name"].(string); ok && strings.TrimSpace(nameVal) != "" {
			info.ProviderName = &nameVal
		}
	}

	if displayName, ok := spec["displayName"].(string); ok && strings.TrimSpace(displayName) != "" {
		if info.ProviderName == nil {
			info.ProviderName = &displayName
		}
		info.ProviderDisplayName = &displayName
	}

	return info, nil
}

// populateTokenAnalyticsMetadata copies LLM token fields into an analytics metadata map.
func populateTokenAnalyticsMetadata(analyticsMetadata map[string]any, tokenInfo *LLMProviderAnalyticsInfo) {
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
		analyticsMetadata[ModelIDMetadataKey] = *tokenInfo.RequestModel
	}
	if tokenInfo.ProviderName != nil {
		analyticsMetadata[AIProviderNameMetadataKey] = *tokenInfo.ProviderName
	}
	if tokenInfo.ProviderDisplayName != nil {
		analyticsMetadata[AIProviderDisplayNameMetadataKey] = *tokenInfo.ProviderDisplayName
	}
}

// extractMCPPayloadFromAccumulated finds the first MCP JSON-RPC result or error event
// from accumulated SSE bytes, or parses the bytes directly as JSON.
func extractMCPPayloadFromAccumulated(accumulated []byte, responseHeaders *policy.Headers) map[string]interface{} {
	if isSSEContent(responseHeaders, accumulated) {
		lines := strings.Split(string(accumulated), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				continue
			}
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(data), &obj); err != nil {
				continue
			}
			// Prefer events that carry a result or error field.
			if _, hasResult := obj["result"]; hasResult {
				return obj
			}
			if _, hasError := obj["error"]; hasError {
				return obj
			}
		}
		return nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(accumulated, &obj); err != nil {
		return nil
	}
	return obj
}

// extractMCPResponseAnalyticsProps builds MCP analytics properties from a parsed JSON-RPC payload.
// Returns nil when there is no relevant data to report.
func extractMCPResponseAnalyticsProps(payload map[string]interface{}) *McpResponseAnalyticsProperties {
	props := McpResponseAnalyticsProperties{}

	serverInfo := McpServerInfo{
		ProtocolVersion: extractStringFromJsonpath(payload, ServerProtocolVersionJsonPath),
		Name:            extractStringFromJsonpath(payload, ServerInfoNameJsonPath),
		Version:         extractStringFromJsonpath(payload, ServerInfoVersionJsonPath),
	}

	if serverInfo.Name != "" || serverInfo.Version != "" {
		props.ServerInfo = &serverInfo
	}

	isError, err := extractBoolFromJsonpath(payload, IsErrorJsonPath)
	if err == nil {
		props.IsError = &isError
	}

	errorCode, err := extractIntFromJsonpath(payload, JsonRpcErrorCodeJsonPath)
	if err == nil {
		props.ErrorCode = &errorCode
	}

	if props.IsError != nil || props.ErrorCode != nil || props.ServerInfo != nil {
		return &props
	}
	return nil
}

// isSSEContent returns true when content is Server-Sent Events format, detected via
// Content-Type header or content structure.
func isSSEContent(headers *policy.Headers, content []byte) bool {
	if headers != nil {
		contentTypes := headers.Get("content-type")
		if len(contentTypes) > 0 && strings.Contains(strings.ToLower(contentTypes[0]), "text/event-stream") {
			return true
		}
	}
	s := string(content)
	return strings.HasPrefix(s, "event:") || strings.Contains(s, "\ndata:")
}

// parseSSEFirstDataEvent returns the JSON bytes from the first "data:" line in SSE content.
// Used for MCP responses where the relevant event is typically the first one.
func parseSSEFirstDataEvent(sseContent []byte) ([]byte, error) {
	lines := strings.Split(string(sseContent), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			return []byte(strings.TrimPrefix(line, "data: ")), nil
		}
	}
	return nil, fmt.Errorf("no data field found in SSE response")
}

// parseSSELastDataEvent returns the JSON bytes from the last non-[DONE] "data:" line.
// Used for LLM streaming responses where the final event carries token usage information.
func parseSSELastDataEvent(sseContent []byte) ([]byte, error) {
	lines := strings.Split(string(sseContent), "\n")
	var lastData []byte
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			continue
		}
		lastData = []byte(data)
	}
	if lastData == nil {
		return nil, fmt.Errorf("no valid data event found in SSE response")
	}
	return lastData, nil
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

// getAllowPayloadsFlag safely extracts the allow_payloads boolean from policy parameters.
func getAllowPayloadsFlag(params map[string]interface{}) bool {
	if params == nil {
		return false
	}
	raw, ok := params["allow_payloads"]
	if !ok {
		return false
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		return lower == "true" || lower == "1" || lower == "yes"
	default:
		return false
	}
}

// Helper to extract string values via JSONPath
func extractStringFromJsonpath(payload map[string]interface{}, path string) string {
	val, err := utils.ExtractValueFromJsonpath(payload, path)
	if err != nil || val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// Helper to extract int values via JSONPath
func extractIntFromJsonpath(payload map[string]interface{}, path string) (int, error) {
	val, err := utils.ExtractValueFromJsonpath(payload, path)
	if err != nil || val == nil {
		return 0, fmt.Errorf("value not found at path %s", path)
	}
	switch v := val.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("unexpected type %T at path %s", val, path)
	}
}

// Helper to extract bool values via JSONPath
func extractBoolFromJsonpath(payload map[string]interface{}, path string) (bool, error) {
	val, err := utils.ExtractValueFromJsonpath(payload, path)
	if err != nil || val == nil {
		return false, fmt.Errorf("value not found at path %s", path)
	}
	switch v := val.(type) {
	case bool:
		return v, nil
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		return lower == "true" || lower == "1" || lower == "yes", nil
	default:
		return false, fmt.Errorf("unexpected type %T at path %s", val, path)
	}
}

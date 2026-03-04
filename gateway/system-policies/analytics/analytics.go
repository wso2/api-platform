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

	// AuthContext key for user ID (used for analytics)
	AuthContextKeyUserID = "x-wso2-user-id"

	// Lazy resource type for LLM provider templates
	lazyResourceTypeLLMProviderTemplate = "LlmProviderTemplate"
	// Lazy resource type for provider-to-template mapping
	lazyResourceTypeProviderTemplateMapping = "ProviderTemplateMapping"
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

	ServerProtocolVersionJsonPath = "$.result.protocolVersion"
	ServerInfoNameJsonPath        = "$.result.serverInfo.name"
	ServerInfoVersionJsonPath     = "$.result.serverInfo.version"
)

// AnalyticsPolicy implements the default analytics data collection process.
type AnalyticsPolicy struct{}

type McpRequestAnalyticsProperties struct {
	JsonRpcMethod  string         `json:"jsonRpcMethod,omitempty"`
	Capability     string         `json:"capability,omitempty"`
	CapabilityName string         `json:"capabilityName,omitempty"`
	ClientInfo     *McpClientInfo `json:"clientInfo,omitempty"`
	ServerInfo     *McpServerInfo `json:"serverInfo,omitempty"`
}

type McpClientInfo struct {
	RequestedProtocolVersion string `json:"requestedProtocolVersion"`
	Name                     string `json:"name"`
	Version                  string `json:"version"`
}

type McpServerInfo struct {
	ProtocolVersion string                `json:"protocolVersion,omitempty"`
	ServerInfo      *McpServerInfoDetails `json:"serverInfo,omitempty"`
}

type McpServerInfoDetails struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type McpResponseAnalyticsProperties struct {
	IsError   bool `json:"isError,omitempty"`
	ErrorCode int  `json:"errorCode,omitempty"`
}

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
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// OnRequest performs Analytics collection process during the request phase
func (a *AnalyticsPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	slog.Debug("Analytics system policy: OnRequest called")
	allowPayloads := getAllowPayloadsFlag(params)
	// Store tokenInfo in analytics metadata for publishing
	analyticsMetadata := make(map[string]any)

	// When allow_payloads is enabled, capture the raw request body into analytics metadata.
	if allowPayloads && ctx != nil && ctx.Body != nil && len(ctx.Body.Content) > 0 {
		slog.Debug("Capturing request payload for analytics")
		analyticsMetadata["request_payload"] = string(ctx.Body.Content)
	}

	// Extract common analytics data from the request
	// Based on the API kind, collect the analytics data
	apiKind := ctx.SharedContext.APIKind
	switch apiKind {
	case KindRestApi:
		// Collect analytics data for REST API scenario
	case KindLlmProvider:
		// Collect analytics data for AI API(LLM Provider) specific scenario
		// Based on the json paths provided the the template, extract the token count data
	case KindLlmProxy:
		// Collect analytics data for LLM Proxy specific scenario
		// Currently no data is collected
	case KindMCP:
		// Collect analytics data specific for MCP scenario from request
		if ctx.Headers != nil && len(ctx.Headers.GetAll()) > 0 {
			// Need to get the mcp-session-id from headers
			sessionIDs := ctx.Headers.Get("mcp-session-id")
			if len(sessionIDs) > 0 {
				analyticsMetadata["mcp_session_id"] = sessionIDs[0]
			}
		}

		if ctx != nil && ctx.Body != nil && len(ctx.Body.Content) > 0 {
			// slog.Debug("MCP Request Body:", "body", string(ctx.Body.Content))
			var mcpPayload map[string]interface{}
			if err := json.Unmarshal(ctx.Body.Content, &mcpPayload); err != nil {
				slog.Error("Failed to unmarshal MCP request body for analytics", "error", err)
				break
			}

			props := McpRequestAnalyticsProperties{}

			// Helper to extract string values via JSONPath
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

			// Populate top-level MCP request analytics properties
			props.JsonRpcMethod = extractString(JsonRpcMethodJsonPath)
			props.CapabilityName = extractString(McpCapabilityNameJsonPath)
			props.Capability = extractString(McpResourceUriJsonPath)

			// Populate client info
			clientInfo := McpClientInfo{
				RequestedProtocolVersion: extractStringFromJsonpath(mcpPayload, ProtocolVersionJsonPath),
				Name:                     extractStringFromJsonpath(mcpPayload, ClientNameJsonPath),
				Version:                  extractStringFromJsonpath(mcpPayload, ClientVersionJsonPath),
			}
			// Only set ClientInfo pointer if at least one field is non-empty so that omitempty can exclude it from JSON
			if clientInfo.RequestedProtocolVersion != "" || clientInfo.Name != "" || clientInfo.Version != "" {
				props.ClientInfo = &clientInfo
			}

			// Marshal to JSON string for transmission through metadata
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

// getTemplateByHandle retrieves a template from the lazy resource cache by its handle
func getTemplateByHandle(templateHandle string) (map[string]interface{}, error) {
	if templateHandle == "" {
		return nil, fmt.Errorf("template handle is empty")
	}

	store := policy.GetLazyResourceStoreInstance()
	if store == nil {
		return nil, fmt.Errorf("lazy resource store is not available")
	}

	// Use ID + resource type lookup to avoid ambiguous matches when the same ID
	// exists under different lazy resource types (e.g., ProviderTemplateMapping).
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

// OnRequest performs Analytics collection process during the response phase
func (p *AnalyticsPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	slog.Debug("Analytics system policy: OnResponse called")
	allowPayloads := getAllowPayloadsFlag(params)

	// Store tokenInfo in analytics metadata for publishing
	analyticsMetadata := make(map[string]any)

	// Extract user ID from AuthContext if available (set by jwt-auth policy)
	if ctx.SharedContext.AuthContext != nil {
		if userID, ok := ctx.SharedContext.AuthContext[AuthContextKeyUserID]; ok && userID != "" {
			analyticsMetadata[AuthContextKeyUserID] = userID
			slog.Debug("Analytics system policy: User ID extracted from AuthContext",
				"userID", userID,
			)
		}
	}

	// Based on the API kind, collect the analytics data
	apiKind := ctx.SharedContext.APIKind
	slog.Debug("API kind: ", "apiKind", apiKind)
	switch apiKind {
	case KindRestApi:
		// Collect analytics data for REST API spcific scenario
	case KindLlmProvider, KindLlmProxy:
		// Collect the analytics data for the AI API(LLM Provider/Proxy) specific scenario
		// Get template handle from SharedContext metadata
		templateHandle, ok := ctx.SharedContext.Metadata["template_handle"].(string)
		slog.Info("Template handle(extracted from route metadata): ", "templateHandle", templateHandle)
		if !ok || templateHandle == "" {
			slog.Debug("No template handle found in route metadata for LLM API")
		} else {
			// Fetch template from lazy resource cache
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

				}
			}
		}
	case KindMCP:
		// Collect the analytics data specific for MCP specific scenario
		if ctx.ResponseHeaders != nil && len(ctx.ResponseHeaders.GetAll()) > 0 {
			if analyticsMetadata["mcp_session_id"] == nil {
				sessionIDs := ctx.ResponseHeaders.Get("mcp-session-id")
				if len(sessionIDs) > 0 {
					analyticsMetadata["mcp_session_id"] = sessionIDs[0]
				}
			}
		}

		// Extract server info from response body
		if ctx != nil && ctx.ResponseBody != nil && len(ctx.ResponseBody.Content) > 0 {
			var mcpResponsePayload map[string]interface{}
			responseContent := ctx.ResponseBody.Content

			// Check if response is in SSE format by inspecting content-type or content structure
			isSSE := false
			if ctx.ResponseHeaders != nil {
				contentTypes := ctx.ResponseHeaders.Get("content-type")
				if len(contentTypes) > 0 && strings.Contains(strings.ToLower(contentTypes[0]), "text/event-stream") {
					isSSE = true
				}
			}

			// Also check content structure if header check didn't confirm SSE
			if !isSSE && (strings.HasPrefix(string(responseContent), "event:") || strings.Contains(string(responseContent), "\ndata:")) {
				isSSE = true
			}

			// Parse SSE format if detected
			if isSSE {
				jsonData, err := parseSSEResponse(responseContent)
				if err != nil {
					slog.Error("Failed to parse SSE response", "error", err)
				} else {
					responseContent = jsonData
				}
			}

			// Unmarshal the JSON (either from SSE data field or direct response)
			if err := json.Unmarshal(responseContent, &mcpResponsePayload); err != nil {
				slog.Error("Failed to unmarshal MCP response body for server info analytics", "error", err)
			} else {
				// Populate server info details
				serverInfoDetails := McpServerInfoDetails{
					Name:    extractStringFromJsonpath(mcpResponsePayload, ServerInfoNameJsonPath),
					Version: extractStringFromJsonpath(mcpResponsePayload, ServerInfoVersionJsonPath),
				}

				// Populate server info
				serverInfo := McpServerInfo{
					ProtocolVersion: extractStringFromJsonpath(mcpResponsePayload, ServerProtocolVersionJsonPath),
				}

				// Only set ServerInfo pointer if at least one field is non-empty
				if serverInfoDetails.Name != "" || serverInfoDetails.Version != "" {
					serverInfo.ServerInfo = &serverInfoDetails
				}

				// Only attach server info if at least one field is non-empty
				if serverInfo.ProtocolVersion != "" || serverInfo.ServerInfo != nil {
					if data, err := json.Marshal(serverInfo); err != nil {
						slog.Error("Failed to marshal MCP server info", "error", err)
					} else {
						analyticsMetadata["mcp_server_info"] = string(data)
					}
				}
			}
		}
	default:
		slog.Error("Invalid API kind")
	}

	// Optionally capture request and response payloads when enabled.
	if allowPayloads {
		if ctx != nil && ctx.ResponseBody != nil && len(ctx.ResponseBody.Content) > 0 {
			slog.Debug("Capturing response payload for analytics")
			analyticsMetadata["response_payload"] = string(ctx.ResponseBody.Content)
		}
	}

	// Return modifications with analytics metadata (including headers if available)
	if len(analyticsMetadata) > 0 {
		return policy.UpstreamResponseModifications{
			AnalyticsMetadata: analyticsMetadata,
		}
	}

	return nil
}

// parseSSEResponse parses Server-Sent Events format and extracts the JSON from the data field
func parseSSEResponse(sseContent []byte) ([]byte, error) {
	lines := strings.Split(string(sseContent), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			jsonData := strings.TrimPrefix(line, "data: ")
			return []byte(jsonData), nil
		}
	}
	return nil, fmt.Errorf("no data field found in SSE response")
}

// extractLLMTokenInfo extracts the LLM token information from the response and request bodies
// template is expected to be a map[string]interface{} from the lazy resource cache
func extractLLMProviderAnalyticsInfo(template map[string]interface{}, ctx *policy.ResponseContext) (*LLMProviderAnalyticsInfo, error) {
	if template == nil {
		return nil, fmt.Errorf("template is nil")
	}

	spec, ok := template["spec"].(map[string]interface{})
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

	if md, ok := template["metadata"].(map[string]interface{}); ok {
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

// getAllowPayloadsFlag safely extracts the allow_payloads boolean from policy parameters.
// Falls back to false when the parameter is missing or of an unexpected type.
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
func extractStringFromJsonpath(mcpResponsePayload map[string]interface{}, path string) string {
	val, err := utils.ExtractValueFromJsonpath(mcpResponsePayload, path)
	if err != nil || val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

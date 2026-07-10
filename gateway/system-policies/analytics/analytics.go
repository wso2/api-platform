package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strconv"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	"github.com/wso2/api-platform/sdk/core/utils"
)

const (
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

	// Auth-context metadata keys. Populated generically (auth-type-agnostic, via
	// SharedContext.AuthContext) by populateAuthAnalyticsMetadata below, so they work
	// uniformly for jwt-auth, opaque-token-auth, api-key-auth, mcp-auth, or any future
	// auth policy without that policy needing to export anything itself. Consumed by the
	// policy-engine's prepareAnalyticEvent (internal/analytics/analytics.go), which mirrors
	// these key names, and from there exposed to the stdout traffic-logging publisher's
	// global "$ctx:auth.*" properties.
	AuthUserIDMetadataKey       = "x-wso2-user-id"
	AuthTypeMetadataKey         = "x-wso2-auth-type"
	AuthIssuerMetadataKey       = "x-wso2-auth-issuer"
	AuthCredentialIDMetadataKey = "x-wso2-auth-credential-id"
	AuthTokenIDMetadataKey      = "x-wso2-auth-token-id"
	AuthAudienceMetadataKey     = "x-wso2-auth-audience"
	AuthScopesMetadataKey       = "x-wso2-auth-scopes"
	AuthPropertiesMetadataKey   = "x-wso2-auth-properties"
	AuthAuthorizedMetadataKey   = "x-wso2-auth-authorized"

	// Subscription metadata keys for subscription and monetization information.
	BillingCustomerIDMetadataKey     = "x-wso2-billing-customer-id"
	BillingSubscriptionIDMetadataKey = "x-wso2-billing-subscription-id"
	SubscriptionStatusMetadataKey    = "x-wso2-subscription-status"
	SubscriptionPlanNameMetadataKey  = "x-wso2-subscription-plan-name"

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
	IsErrorJsonPath               = "$.result.isError"
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

// OnRequestHeaders collects analytics data available at the request-headers phase.
// OnRequestBody is called inline for bodyless requests when RequiresRequestBody is true,
// but OnRequestHeaders acts as a safety net for chains where that condition does not hold,
// and provides early capture of header-sourced analytics (application ID, MCP session ID).
func (a *AnalyticsPolicy) OnRequestHeaders(_ context.Context, reqCtx *policy.RequestHeaderContext, params map[string]interface{}) policy.RequestHeaderAction {
	slog.Debug("Analytics system policy: OnRequestHeaders called")
	analyticsMetadata := make(map[string]any)

	if reqCtx.Headers != nil {
		if appIDs := reqCtx.Headers.Get("x-wso2-application-id"); len(appIDs) > 0 {
			analyticsMetadata[ApplicationIDMetadataKey] = appIDs[0]
		}
		if appNames := reqCtx.Headers.Get("x-wso2-application-name"); len(appNames) > 0 {
			analyticsMetadata[ApplicationNameMetadataKey] = appNames[0]
		}
	}

	if reqCtx.SharedContext.APIKind == policy.APIKindMCP && reqCtx.Headers != nil {
		if sessionIDs := reqCtx.Headers.Get("mcp-session-id"); len(sessionIDs) > 0 {
			analyticsMetadata["mcp_session_id"] = sessionIDs[0]
		}
	}

	// Capture all request headers when enabled, so they flow into analytics events
	// (and the stdout/log publisher) without attaching a per-API header policy.
	if sendReqHeaders, _ := getHeaderFlags(params); sendReqHeaders && reqCtx.Headers != nil {
		if headers := serializeHeaders(reqCtx.Headers); headers != "" {
			analyticsMetadata["request_headers"] = headers
		}
	}

	if len(analyticsMetadata) > 0 {
		return policy.UpstreamRequestHeaderModifications{AnalyticsMetadata: analyticsMetadata}
	}
	return policy.UpstreamRequestHeaderModifications{}
}

// populateAuthAnalyticsMetadata walks authChain (handling layered/multi-auth via
// AuthContext.Previous) and, for the first layer that is both authenticated and has a
// non-empty subject, stamps auth-derived analytics metadata into analyticsMetadata —
// auth-type-agnostic, so it works uniformly for jwt-auth, opaque-token-auth, api-key-auth,
// mcp-auth, or any future auth policy without that policy needing to export anything
// itself; it only relies on the common AuthContext type any auth policy already populates.
// Scopes are sorted and space-joined and audience is comma-joined, matching the
// log-message policy's own $ctx:auth.* serialization choices for consistency across
// per-API and global traffic-log properties. Custom claims (Properties) are JSON-encoded,
// mirroring how captured headers are already carried as a JSON string elsewhere in this
// file. Fields are omitted (not empty-stringed) when absent, consistent with every other
// optional key in this policy.
func populateAuthAnalyticsMetadata(analyticsMetadata map[string]any, authChain *policy.AuthContext) {
	for authCtx := authChain; authCtx != nil; authCtx = authCtx.Previous {
		if !authCtx.Authenticated || authCtx.Subject == "" {
			continue
		}

		analyticsMetadata[AuthUserIDMetadataKey] = authCtx.Subject
		if authCtx.AuthType != "" {
			analyticsMetadata[AuthTypeMetadataKey] = authCtx.AuthType
		}
		if authCtx.Issuer != "" {
			analyticsMetadata[AuthIssuerMetadataKey] = authCtx.Issuer
		}
		if authCtx.CredentialID != "" {
			analyticsMetadata[AuthCredentialIDMetadataKey] = authCtx.CredentialID
		}
		if authCtx.TokenId != "" {
			analyticsMetadata[AuthTokenIDMetadataKey] = authCtx.TokenId
		}
		if len(authCtx.Audience) > 0 {
			analyticsMetadata[AuthAudienceMetadataKey] = strings.Join(authCtx.Audience, ",")
		}
		if len(authCtx.Scopes) > 0 {
			scopes := make([]string, 0, len(authCtx.Scopes))
			for name := range authCtx.Scopes {
				scopes = append(scopes, name)
			}
			sort.Strings(scopes)
			analyticsMetadata[AuthScopesMetadataKey] = strings.Join(scopes, " ")
		}
		if len(authCtx.Properties) > 0 {
			if data, err := json.Marshal(authCtx.Properties); err == nil {
				analyticsMetadata[AuthPropertiesMetadataKey] = string(data)
			} else {
				slog.Warn("Analytics system policy: failed to marshal auth properties", "error", err)
			}
		}
		// Authorized is distinct from Authenticated (which this block already gates on):
		// it reflects a separate authorization check (e.g. mcp-authz) and can genuinely be
		// false even for an authenticated request, so it's always stamped rather than
		// omitted-when-zero like the optional fields above.
		analyticsMetadata[AuthAuthorizedMetadataKey] = strconv.FormatBool(authCtx.Authorized)

		slog.Debug("Analytics system policy: auth-context metadata extracted",
			"subject", authCtx.Subject, "authType", authCtx.AuthType)
		return
	}
}

// OnResponseHeaders collects analytics data available at the response-headers phase.
// Auth context and response headers are already populated here, so we emit them early
// rather than waiting for the body phase (which may not be reached for header-only responses).
func (a *AnalyticsPolicy) OnResponseHeaders(_ context.Context, respCtx *policy.ResponseHeaderContext, params map[string]interface{}) policy.ResponseHeaderAction {
	slog.Debug("Analytics system policy: OnResponseHeaders called")
	analyticsMetadata := make(map[string]any)

	populateAuthAnalyticsMetadata(analyticsMetadata, respCtx.SharedContext.AuthContext)

	// Subscription and monetization fields are written to SharedContext.Metadata by subscription-validation policy
	if md := respCtx.SharedContext.Metadata; md != nil {
		if v, ok := md[BillingCustomerIDMetadataKey].(string); ok && v != "" {
			analyticsMetadata[BillingCustomerIDMetadataKey] = v
		}
		if v, ok := md[BillingSubscriptionIDMetadataKey].(string); ok && v != "" {
			analyticsMetadata[BillingSubscriptionIDMetadataKey] = v
		}
		if v, ok := md[SubscriptionStatusMetadataKey].(string); ok && v != "" {
			analyticsMetadata[SubscriptionStatusMetadataKey] = v
		}
		if v, ok := md[SubscriptionPlanNameMetadataKey].(string); ok && v != "" {
			analyticsMetadata[SubscriptionPlanNameMetadataKey] = v
		}
	}

	// Capture the response content type for all API kinds. The Envoy access log does
	// not carry response headers (no additional_response_headers_to_log is configured,
	// to avoid an uncontrolled header source)
	if respCtx.ResponseHeaders != nil {
		if contentTypes := respCtx.ResponseHeaders.Get("content-type"); len(contentTypes) > 0 {
			analyticsMetadata["response_content_type"] = contentTypes[0]
		}
	}

	if respCtx.SharedContext.APIKind == policy.APIKindMCP && respCtx.ResponseHeaders != nil {
		if sessionIDs := respCtx.ResponseHeaders.Get("mcp-session-id"); len(sessionIDs) > 0 {
			analyticsMetadata["mcp_session_id"] = sessionIDs[0]
		}
	}

	// Capture all response headers when enabled.
	if _, sendRespHeaders := getHeaderFlags(params); sendRespHeaders && respCtx.ResponseHeaders != nil {
		if headers := serializeHeaders(respCtx.ResponseHeaders); headers != "" {
			analyticsMetadata["response_headers"] = headers
		}
	}

	if len(analyticsMetadata) > 0 {
		return policy.DownstreamResponseHeaderModifications{AnalyticsMetadata: analyticsMetadata}
	}
	return policy.DownstreamResponseHeaderModifications{}
}

// OnRequestBody performs Analytics collection process during the request phase (buffered).
func (a *AnalyticsPolicy) OnRequestBody(_ context.Context, ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	slog.Debug("Analytics system policy: OnRequestBody called")
	sendReqBody, _ := getPayloadFlags(params)
	analyticsMetadata := make(map[string]any)

	// When request payload capture is enabled, capture the raw request body into analytics metadata.
	if sendReqBody && ctx != nil && ctx.Body != nil && len(ctx.Body.Content) > 0 {
		slog.Debug("Capturing request payload for analytics")
		analyticsMetadata["request_payload"] = string(ctx.Body.Content)
	}

	apiKind := ctx.SharedContext.APIKind
	switch apiKind {
	case policy.APIKindRestApi:
		// Collect analytics data for REST API scenario
	case policy.APIKindLlmProvider:
		// Collect analytics data for AI API(LLM Provider) specific scenario
	case policy.APIKindLlmProxy:
		// Collect analytics data for LLM Proxy specific scenario
	case policy.APIKindMCP:
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
			props.Capability = deriveMCPCapability(props.JsonRpcMethod)

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
	_, sendRespBody := getPayloadFlags(params)

	analyticsMetadata := make(map[string]any)

	populateAuthAnalyticsMetadata(analyticsMetadata, ctx.SharedContext.AuthContext)

	apiKind := ctx.SharedContext.APIKind
	slog.Debug("API kind: ", "apiKind", apiKind)
	switch apiKind {
	case policy.APIKindRestApi:
		// Collect analytics data for REST API specific scenario
	case policy.APIKindLlmProvider, policy.APIKindLlmProxy:
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
	case policy.APIKindMCP:
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

	if sendRespBody {
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
func (a *AnalyticsPolicy) OnResponseBodyChunk(_ context.Context, ctx *policy.ResponseStreamContext, chunk *policy.StreamBody, params map[string]interface{}) policy.StreamingResponseAction {
	slog.Debug("Analytics system policy: OnResponseBodyChunk called")
	if ctx.SharedContext.Metadata == nil {
		ctx.SharedContext.Metadata = make(map[string]interface{})
	}

	if len(chunk.Chunk) > 0 {
		acc, _ := ctx.SharedContext.Metadata[analyticsStreamAccKey].([]byte)
		ctx.SharedContext.Metadata[analyticsStreamAccKey] = append(acc, chunk.Chunk...)
	}

	if !chunk.EndOfStream {
		return policy.ForwardResponseChunk{}
	}

	// EndOfStream: consume accumulated bytes and emit analytics.
	accumulated, _ := ctx.SharedContext.Metadata[analyticsStreamAccKey].([]byte)
	delete(ctx.SharedContext.Metadata, analyticsStreamAccKey)

	analyticsMetadata := make(map[string]any)

	populateAuthAnalyticsMetadata(analyticsMetadata, ctx.SharedContext.AuthContext)

	apiKind := ctx.SharedContext.APIKind
	switch apiKind {
	case policy.APIKindRestApi:
		// No body analytics for REST API
	case policy.APIKindLlmProvider, policy.APIKindLlmProxy:
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
	case policy.APIKindMCP:
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

	_, sendRespBody := getPayloadFlags(params)
	if sendRespBody && len(accumulated) > 0 {
		analyticsMetadata["response_payload"] = string(accumulated)
	}

	if len(analyticsMetadata) == 0 {
		return policy.ForwardResponseChunk{}
	}
	return policy.ForwardResponseChunk{AnalyticsMetadata: analyticsMetadata}
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
// automatically. For SSE, all data events are merged (later events win on top-level key
// conflicts) so that providers like Anthropic — which send the model name in the first
// event (message_start) and final token counts in a middle event (message_delta) — are
// handled correctly alongside providers that put everything in the last event.
func extractLLMProviderAnalyticsInfoFromBytes(
	template map[string]interface{},
	requestHeaders, responseHeaders *policy.Headers,
	requestBodyBytes, responseBodyBytes []byte,
) (*LLMProviderAnalyticsInfo, error) {
	var responseJSON map[string]interface{}
	if len(responseBodyBytes) > 0 {
		// SSE responses: merge all data events so fields spread across events are visible.
		if merged, err := parseSSEMergedDataEvents(responseBodyBytes); err == nil {
			responseJSON = merged
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

	// isError denotes whether the JSON-RPC response represents an error. It is true when a
	// protocol-level error object is present ($.error) or when a tool result explicitly sets
	// result.isError=true; false otherwise. Always emitted so consumers get a definitive flag.
	isError := false
	if errVal, hasError := payload["error"]; hasError && errVal != nil {
		isError = true
	} else if resultIsError, err := extractBoolFromJsonpath(payload, IsErrorJsonPath); err == nil {
		isError = resultIsError
	}
	props.IsError = &isError

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

// parseSSEMergedDataEvents merges all non-[DONE] data events from SSE content into a single
// JSON map. Keys from later events overwrite keys from earlier events at the top level, so
// providers like Anthropic that spread data across multiple events are handled correctly:
//
//   - message_start → contributes "message" (contains "model") and nested "usage"
//   - message_delta → contributes top-level "usage" with final input/output token counts
//   - message_stop  → contributes only "type"; no useful fields
//
// After the merge the caller can resolve e.g. "$.message.model" and "$.usage.output_tokens"
// from a single map instead of having to scan individual events.
func parseSSEMergedDataEvents(sseContent []byte) (map[string]interface{}, error) {
	lines := strings.Split(string(sseContent), "\n")
	merged := make(map[string]interface{})
	found := false
	var currentData []string

	flushEvent := func() {
		if len(currentData) == 0 {
			return
		}
		payload := strings.Join(currentData, "\n")
		currentData = currentData[:0]
		if payload == "[DONE]" {
			return
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &obj); err != nil {
			return
		}
		for k, v := range obj {
			merged[k] = v
		}
		found = true
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			currentData = append(currentData, strings.TrimPrefix(line, "data: "))
		} else if line == "" {
			flushEvent()
		}
	}
	flushEvent()

	if !found {
		return nil, fmt.Errorf("no valid data events found in SSE response")
	}
	return merged, nil
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

// getPayloadFlags derives per-direction payload capture flags from policy parameters.
// New parameters request_body and response_body take precedence. When neither
// is provided, the deprecated allow_payloads flag is used as a fallback, mapping to
// both directions for backward compatibility.
func getPayloadFlags(params map[string]interface{}) (sendRequestBody, sendResponseBody bool) {
	if params == nil {
		return false, false
	}

	hasReq, hasResp := false, false

	if raw, ok := params["request_body"]; ok {
		sendRequestBody = parseBoolLike(raw)
		hasReq = true
	}
	if raw, ok := params["response_body"]; ok {
		sendResponseBody = parseBoolLike(raw)
		hasResp = true
	}

	// If either of the new flags has been explicitly configured, do not consult
	// the deprecated allow_payloads flag.
	if hasReq || hasResp {
		return sendRequestBody, sendResponseBody
	}

	// Backward compatibility: fall back to allow_payloads when new flags are absent.
	if raw, ok := params["allow_payloads"]; ok {
		if parseBoolLike(raw) {
			return true, true
		}
	}

	return false, false
}

// parseBoolLike interprets bool and common string ("true"/"1"/"yes") representations
// of a boolean policy parameter.
func parseBoolLike(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		lower := strings.ToLower(strings.TrimSpace(val))
		return lower == "true" || lower == "1" || lower == "yes"
	default:
		return false
	}
}

// getHeaderFlags derives per-direction header capture flags from policy parameters.
func getHeaderFlags(params map[string]interface{}) (sendRequestHeaders, sendResponseHeaders bool) {
	if params == nil {
		return false, false
	}
	if raw, ok := params["request_headers"]; ok {
		sendRequestHeaders = parseBoolLike(raw)
	}
	if raw, ok := params["response_headers"]; ok {
		sendResponseHeaders = parseBoolLike(raw)
	}
	return sendRequestHeaders, sendResponseHeaders
}

// serializeHeaders renders all headers as a JSON object string ({"name":"v1, v2"}),
// matching the request_headers/response_headers format the analytics engine reads.
// Returns "" when there are no headers. Sensitive values are not masked here; the
// stdout/log publisher applies masked_headers on output.
func serializeHeaders(headers *policy.Headers) string {
	all := headers.GetAll()
	if len(all) == 0 {
		return ""
	}
	flat := make(map[string]string, len(all))
	for name, values := range all {
		flat[name] = strings.Join(values, ", ")
	}
	data, err := json.Marshal(flat)
	if err != nil {
		slog.Error("Failed to marshal headers for analytics", "error", err)
		return ""
	}
	return string(data)
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

// deriveMCPCapability maps an MCP JSON-RPC method to a capability type for analytics.
// It returns "" when the method has no mapping, in which case the caller omits the
// field (mirrors carbon-apimgt's SynapseAnalyticsDataProvider behavior).
func deriveMCPCapability(method string) string {
	switch {
	case strings.HasPrefix(method, "tools/"):
		return "TOOL"
	case strings.HasPrefix(method, "resources/"):
		return "RESOURCE"
	case strings.HasPrefix(method, "prompts/"):
		return "PROMPT"
	default:
		return ""
	}
}
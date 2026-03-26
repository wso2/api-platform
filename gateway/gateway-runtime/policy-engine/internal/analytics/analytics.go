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

package analytics

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"strconv"
	"time"

	v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	analytics_publisher "github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/publishers"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
)

const lazyResourceTypeLLMProviderTemplate = "LlmProviderTemplate"
const lazyResourceTypeProviderTemplateMapping = "ProviderTemplateMapping"

// EventCategory represents the category of an event.
type EventCategory string

// FaultCategory represents the category of a fault.
type FaultCategory string

// RFC3339Millis represents the RFC3339 date format with milliseconds.
const RFC3339Millis = "2006-01-02T15:04:05.000Z07:00"

const (
	// EventCategorySuccess represents a successful event.
	EventCategorySuccess EventCategory = "SUCCESS"
	// EventCategoryFault represents a fault event.
	EventCategoryFault EventCategory = "FAULT"
	// EventCategoryInvalid represents an invalid event.
	EventCategoryInvalid EventCategory = "INVALID"
	// FaultCategoryTargetConnectivity represents a target connectivity fault.
	FaultCategoryTargetConnectivity FaultCategory = "TARGET_CONNECTIVITY"
	// FaultCategoryOther represents other faults.
	FaultCategoryOther FaultCategory = "OTHER"
	// DefaultAnalyticsPublisher represents the default analytics publisher.
	DefaultAnalyticsPublisher = "default"
	// MoesifAnalyticsPublisher represents the Moesif analytics publisher.
	MoesifAnalyticsPublisher = "moesif"

	// HeaderKeys represents the header keys.
	RequestHeadersKey  = "request_headers"
	ResponseHeadersKey = "response_headers"

	// PromptTokenCountMetadataKey represents the prompt token count metadata key.
	PromptTokenCountMetadataKey string = "aitoken:prompttokencount"
	// CompletionTokenCountMetadataKey represents the completion token count metadata key.
	CompletionTokenCountMetadataKey string = "aitoken:completiontokencount"
	// TotalTokenCountMetadataKey represents the total token count metadata key.
	TotalTokenCountMetadataKey string = "aitoken:totaltokencount"
	// ModelIDMetadataKey represents the model name metadata key.
	ModelIDMetadataKey string = "aitoken:modelid"

	// AIProviderNameMetadataKey represents the AI provider metadata key.
	AIProviderNameMetadataKey string = "ai:providername"
	// AIProviderAPIVersionMetadataKey represents the AI provider API version metadata key.
	AIProviderAPIVersionMetadataKey string = "ai:providerversion"

	// UserIDMetadataKey represents the user ID metadata key for analytics.
	UserIDMetadataKey string = "x-wso2-user-id"
)

// Analytics represents analytics collector service.
type Analytics struct {
	// cfg represents the server configuration.
	cfg *config.Config
	// publishers represents the publishers.
	publishers []analytics_publisher.Publisher
}

// NewAnalytics creates a new instance of Analytics.
func NewAnalytics(cfg *config.Config) *Analytics {
	analyticsCfg := cfg.Analytics
	publishers := make([]analytics_publisher.Publisher, 0)
	if analyticsCfg.Enabled {
		for _, publisherName := range analyticsCfg.EnabledPublishers {
			switch publisherName {
			case "moesif":
				publisher := analytics_publisher.NewMoesif(&analyticsCfg.Publishers.Moesif)
				if publisher != nil {
					publishers = append(publishers, publisher)
					slog.Info("Moesif publisher added")
				}
			default:
				slog.Warn("Unknown publisher type", "type", publisherName)
			}
		}
	}

	if len(publishers) == 0 {
		slog.Debug("No analytics publishers found. Analytics will not be published.")
	}
	return &Analytics{
		cfg:        cfg,
		publishers: publishers,
	}
}

// Process processes event and publishes the data
func (c *Analytics) Process(event *v3.HTTPAccessLogEntry) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic occurred",
				"error", r,
				"context", "Recovered from panic in Process method",
			)
		}
	}()
	if c.isInvalid(event) {
		slog.Error("Invalid event received from the access log service")
		return
	}

	// Add logic to publish the event
	analyticEvent := c.prepareAnalyticEvent(event)
	for _, publisher := range c.publishers {
		publisher.Publish(analyticEvent)
	}

}

// isInvalid checks if the log entry is invalid.
func (c *Analytics) isInvalid(logEntry *v3.HTTPAccessLogEntry) bool {
	return logEntry.GetResponse() == nil
}

// GetFaultType returns the fault type.
func (c *Analytics) GetFaultType() FaultCategory {
	return FaultCategoryOther
}

func (c *Analytics) prepareAnalyticEvent(logEntry *v3.HTTPAccessLogEntry) *dto.Event {
	keyValuePairsFromMetadata := make(map[string]string)
	typedValuePairsFromMetadata := make(map[string]interface{})
	slog.Debug("Log entry: ", "logEntry", logEntry)
	if logEntry.CommonProperties != nil && logEntry.CommonProperties.Metadata != nil && logEntry.CommonProperties.Metadata.FilterMetadata != nil {
		slog.Debug("Proceeding to filtering metadata")
		if sv, exists := logEntry.CommonProperties.Metadata.FilterMetadata[constants.ExtProcFilterName]; exists {
			if sv.Fields != nil {
				slog.Debug(fmt.Sprintf("Filter metadata: %+v", sv))
				for key, value := range sv.Fields {
					if value != nil {
						if key == "analytics_data" {
							// Handle the analytics_data struct
							if analyticsStruct := value.GetStructValue(); analyticsStruct != nil {
								for analyticsKey, analyticsValue := range analyticsStruct.Fields {
									if analyticsValue != nil {
										metadataValue := analyticsValue.AsInterface()
										typedValuePairsFromMetadata[analyticsKey] = metadataValue
										if stringValue, ok := metadataValue.(string); ok {
											keyValuePairsFromMetadata[analyticsKey] = stringValue
										} else {
											keyValuePairsFromMetadata[analyticsKey] = fmt.Sprintf("%v", metadataValue)
										}
									}
								}
							}
						} else {
							// Handle regular string values
							metadataValue := value.AsInterface()
							typedValuePairsFromMetadata[key] = metadataValue
							if stringValue, ok := metadataValue.(string); ok {
								keyValuePairsFromMetadata[key] = stringValue
							} else {
								keyValuePairsFromMetadata[key] = fmt.Sprintf("%v", metadataValue)
							}
						}
					}
				}
			}
		}
	}

	event := &dto.Event{}
	for key, value := range keyValuePairsFromMetadata {
		slog.Debug(fmt.Sprintf("Metadata key: %v -> value: %+v", key, value))
	}
	// Prepare extended API
	extendedAPI := dto.ExtendedAPI{}
	extendedAPI.APIType = keyValuePairsFromMetadata[APITypeKey]
	extendedAPI.APIID = keyValuePairsFromMetadata[APIIDKey]
	extendedAPI.APICreator = keyValuePairsFromMetadata[APICreatorKey]
	extendedAPI.APIName = keyValuePairsFromMetadata[APINameKey]
	extendedAPI.APIVersion = keyValuePairsFromMetadata[APIVersionKey]
	extendedAPI.APICreatorTenantDomain = keyValuePairsFromMetadata[APICreatorTenantDomainKey]
	extendedAPI.OrganizationID = keyValuePairsFromMetadata[APIOrganizationIDKey]
	extendedAPI.APIContext = keyValuePairsFromMetadata[APIContextKey]
	extendedAPI.EnvironmentID = keyValuePairsFromMetadata[APIEnvironmentKey]
	extendedAPI.ProjectID = keyValuePairsFromMetadata[ProjectIDKey]

	request := logEntry.GetRequest()
	response := logEntry.GetResponse()

	// Prepare operation
	operation := dto.Operation{}
	// operation.APIResourceTemplate = keyValuePairsFromMetadata[APIResourceTemplateKey]
	if request != nil {
		operation.APIResourceTemplate = logEntry.GetRequest().GetOriginalPath()
		operation.APIMethod = logEntry.Request.GetRequestMethod().String()
	}

	// Prepare target
	target := dto.Target{}
	target.ResponseCacheHit = false
	if response != nil {
		target.TargetResponseCode = int(logEntry.GetResponse().GetResponseCode().Value)
		// target.Destination = keyValuePairsFromMetadata[DestinationKey]
		target.Destination = logEntry.GetRequest().GetAuthority() + logEntry.GetRequest().GetPath()
		target.ResponseCodeDetail = logEntry.GetResponse().GetResponseCodeDetails()
	}

	// Prepare Application
	application := &dto.Application{}
	if keyValuePairsFromMetadata[AppIDKey] == Unknown {
		application = c.getAnonymousApp()
	} else {
		application.ApplicationID = keyValuePairsFromMetadata[AppIDKey]
		application.KeyType = keyValuePairsFromMetadata[AppKeyTypeKey]
		application.ApplicationName = keyValuePairsFromMetadata[AppNameKey]
		application.ApplicationOwner = keyValuePairsFromMetadata[AppOwnerKey]
	}

	properties := logEntry.GetCommonProperties()
	if properties != nil && properties.TimeToLastRxByte != nil &&
		properties.TimeToFirstUpstreamTxByte != nil && properties.TimeToFirstUpstreamRxByte != nil &&
		properties.TimeToLastUpstreamRxByte != nil && properties.TimeToLastDownstreamTxByte != nil {

		lastRx :=
			(properties.TimeToLastRxByte.Seconds * 1000) +
				(int64(properties.TimeToLastRxByte.Nanos) / 1_000_000)

		firstUpTx :=
			(properties.TimeToFirstUpstreamTxByte.Seconds * 1000) +
				(int64(properties.TimeToFirstUpstreamTxByte.Nanos) / 1_000_000)

		firstUpRx :=
			(properties.TimeToFirstUpstreamRxByte.Seconds * 1000) +
				(int64(properties.TimeToFirstUpstreamRxByte.Nanos) / 1_000_000)

		lastUpRx :=
			(properties.TimeToLastUpstreamRxByte.Seconds * 1000) +
				(int64(properties.TimeToLastUpstreamRxByte.Nanos) / 1_000_000)

		lastDownTx :=
			(properties.TimeToLastDownstreamTxByte.Seconds * 1000) +
				(int64(properties.TimeToLastDownstreamTxByte.Nanos) / 1_000_000)

		latencies := dto.Latencies{
			BackendLatency:           lastUpRx - firstUpTx,
			RequestMediationLatency:  firstUpTx - lastRx,
			ResponseLatency:          lastDownTx - firstUpRx,
			ResponseMediationLatency: lastDownTx - lastUpRx,
		}

		event.Latencies = &latencies
	}

	// prepare metaInfo
	metaInfo := dto.MetaInfo{}
	if logEntry.GetCommonProperties().GetStreamId() != "" {
		metaInfo.CorrelationID = logEntry.GetCommonProperties().GetStreamId()
	} else {
		metaInfo.CorrelationID = logEntry.GetRequest().RequestId
	}
	metaInfo.RegionID = keyValuePairsFromMetadata[RegionKey]

	userAgent := logEntry.GetRequest().GetUserAgent()
	userName := keyValuePairsFromMetadata[APIUserNameKey]
	userIP := logEntry.GetCommonProperties().GetDownstreamRemoteAddress().GetSocketAddress().GetAddress()
	if userIP == "" {
		userIP = Unknown
	}
	if userAgent == "" {
		userAgent = Unknown
	}

	event.MetaInfo = &metaInfo
	event.API = &extendedAPI
	event.Operation = &operation
	event.Target = &target
	event.Application = application
	event.UserAgentHeader = userAgent
	event.UserName = userName
	event.UserIP = userIP
	event.ProxyResponseCode = int(logEntry.GetResponse().GetResponseCode().Value)
	event.RequestTimestamp = logEntry.GetCommonProperties().GetStartTime().AsTime()
	event.Properties = make(map[string]interface{}, 0)

	// Set user ID from metadata if available (for analytics/Moesif integration)
	if userID, exists := keyValuePairsFromMetadata[UserIDMetadataKey]; exists && userID != "" {
		event.Properties[UserIDMetadataKey] = userID
		slog.Debug("Analytics: User ID set from metadata", "userID", userID)
	}

	// Forward guardrail metadata when available in analytics_data.
	if guardrailHitRaw, exists := typedValuePairsFromMetadata[constants.GuardrailHitMetadataKey]; exists {
		switch guardrailHit := guardrailHitRaw.(type) {
		case bool:
			event.Properties[constants.GuardrailHitMetadataKey] = guardrailHit
		case string:
			if parsed, err := strconv.ParseBool(guardrailHit); err == nil {
				event.Properties[constants.GuardrailHitMetadataKey] = parsed
			}
		}
	}
	if guardrailName, exists := keyValuePairsFromMetadata[constants.GuardrailNameMetadataKey]; exists && guardrailName != "" {
		event.Properties[constants.GuardrailNameMetadataKey] = guardrailName
	}

	var parsedLLMCost interface{}

	// Set LLM cost from metadata when available.
	if rawCost, exists := keyValuePairsFromMetadata[constants.LLMCostMetadataKey]; exists && rawCost != "" {

		slog.Debug("Proceeding to process LLM cost metadata")
		if llmCost, err := strconv.ParseFloat(rawCost, 64); err == nil {
			parsedLLMCost = llmCost
		} else {
			parsedLLMCost = rawCost
		}
	}
	if parsedLLMCost != nil {
		event.Properties[constants.LLMCostPropertyKey] = parsedLLMCost
	}

	// Process AI related metadata only if all the required metadata are present
	if keyValuePairsFromMetadata[AIProviderNameMetadataKey] != "" ||
		keyValuePairsFromMetadata[AIProviderAPIVersionMetadataKey] != "" ||
		keyValuePairsFromMetadata[ModelIDMetadataKey] != "" {
		slog.Debug("Proceeding to process AI related metadata")
		aiMetadata := dto.AIMetadata{}
		aiMetadata.VendorName = keyValuePairsFromMetadata[AIProviderNameMetadataKey]
		aiMetadata.VendorVersion = keyValuePairsFromMetadata[APIVersionKey]
		aiMetadata.Model = keyValuePairsFromMetadata[ModelIDMetadataKey]
		if parsedLLMCost != nil {
			aiMetadata.LLMCost = parsedLLMCost
		}
		event.Properties["aiMetadata"] = aiMetadata

		aiTokenUsage := dto.AITokenUsage{}
		// Prompt tokens
		if raw, ok := keyValuePairsFromMetadata[PromptTokenCountMetadataKey]; !ok {
			slog.Debug(
				"Prompt token count not found in response",
				"metadataKey", PromptTokenCountMetadataKey,
			)
		} else if promptToken, err := strconv.Atoi(raw); err == nil {
			aiTokenUsage.PromptToken = promptToken
		} else {
			slog.Error("Error converting PromptTokenCountMetadataKey to integer", "error", err)
		}
		// Completion tokens
		if raw, ok := keyValuePairsFromMetadata[CompletionTokenCountMetadataKey]; !ok {
			slog.Debug(
				"Completion token count not found in response",
				"metadataKey", CompletionTokenCountMetadataKey,
			)
		} else if completionToken, err := strconv.Atoi(raw); err == nil {
			aiTokenUsage.CompletionToken = completionToken
		} else {
			slog.Error("Error converting CompletionTokenCountMetadataKey to integer", "error", err)
		}
		// Total tokens
		if raw, ok := keyValuePairsFromMetadata[TotalTokenCountMetadataKey]; !ok {
			slog.Debug(
				"Total token count not found in response",
				"metadataKey", TotalTokenCountMetadataKey,
			)
		} else if totalToken, err := strconv.Atoi(raw); err == nil {
			aiTokenUsage.TotalToken = totalToken
		} else {
			slog.Error("Error converting TotalTokenCountMetadataKey to integer", "error", err)
		}

		hour := time.Now().Hour()
		aiTokenUsage.Hour = &hour
		event.Properties["aiTokenUsage"] = aiTokenUsage

		if aiMetadata.VendorName != "" {
			event.Properties["isEgress"] = true
			event.Properties["subtype"] = "AIAPI"
		}
	}

	if userName == "" {
		userName = Unknown
	}
	event.Properties["userName"] = userName
	event.Properties["commonName"] = "N/A"
	event.Properties["apiContext"] = extendedAPI.APIContext
	if logEntry.Response != nil {
		if contentTypeHeader := logEntry.Response.GetResponseHeaders()["content-type"]; contentTypeHeader != "" {
			event.Properties["responseContentType"] = contentTypeHeader
		} else {
			event.Properties["responseContentType"] = Unknown
		}
		event.Properties["responseSize"] = logEntry.Response.ResponseBodyBytes
	} else {
		event.Properties["responseContentType"] = Unknown
	}

	//Adding request and response headers for the analytics event
	if requestHeaders, exists := keyValuePairsFromMetadata[RequestHeadersKey]; exists {
		event.Properties["requestHeaders"] = requestHeaders
	}
	if responseHeaders, exists := keyValuePairsFromMetadata[ResponseHeadersKey]; exists {
		event.Properties["responseHeaders"] = responseHeaders
	}

	// Optionally attach request and response payloads when enabled via configuration.
	if c.cfg.Analytics.AllowPayloads {
		if requestPayload, ok := keyValuePairsFromMetadata["request_payload"]; ok && requestPayload != "" {
			event.Properties["request_payload"] = requestPayload
			slog.Debug("Analytics request payload captured", "size_bytes", len(requestPayload))
		}
		if responsePayload, ok := keyValuePairsFromMetadata["response_payload"]; ok && responsePayload != "" {
			event.Properties["response_payload"] = responsePayload
			slog.Debug("Analytics response payload captured", "size_bytes", len(responsePayload))
		}
	}

	if keyValuePairsFromMetadata[APITypeKey] != "" && keyValuePairsFromMetadata[APITypeKey] == "Mcp" {
		mcpAnalytics := make(map[string]interface{})
		if mcpSessionID, ok := keyValuePairsFromMetadata["mcp_session_id"]; ok && mcpSessionID != "" {
			mcpAnalytics["mcp_session_id"] = mcpSessionID
		}
		if mcpRequestProps, ok := keyValuePairsFromMetadata["mcp_request_properties"]; ok && mcpRequestProps != "" {
			// Parse the JSON string into a map
			var propsMap map[string]interface{}
			if err := json.Unmarshal([]byte(mcpRequestProps), &propsMap); err == nil {
				maps.Copy(mcpAnalytics, propsMap)
			} else {
				slog.Debug("Failed to unmarshal MCP request properties", "error", err)
				// Fallback to raw string if parsing fails
				mcpAnalytics["mcp_request_properties"] = mcpRequestProps
			}
		}
		if mcpResponseProps, ok := keyValuePairsFromMetadata["mcp_response_properties"]; ok && mcpResponseProps != "" {
			// Parse the JSON string into a map
			var responsePropsMap map[string]interface{}
			if err := json.Unmarshal([]byte(mcpResponseProps), &responsePropsMap); err == nil {
				maps.Copy(mcpAnalytics, responsePropsMap)
			} else {
				slog.Debug("Failed to unmarshal MCP response properties", "error", err)
				// Fallback to raw string if parsing fails
				mcpAnalytics["mcp_response_properties"] = mcpResponseProps
			}
		}
		event.Properties["mcpAnalytics"] = mcpAnalytics
	}

	return event
}

func (c *Analytics) getAnonymousApp() *dto.Application {
	application := &dto.Application{}
	application.ApplicationID = anonymousValue
	application.ApplicationName = anonymousValue
	application.KeyType = anonymousValue
	application.ApplicationOwner = anonymousValue
	return application
}

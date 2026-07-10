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

// NewAnalytics creates a new instance of Analytics. Publishers are assembled from
// each independently-configured consumer of the collected data: the analytics
// consumer ([analytics], e.g. Moesif) and the traffic-logging consumer
// ([traffic_logging], stdout JSON). Both rely on the collector being enabled to
// receive any events.
func NewAnalytics(cfg *config.Config) *Analytics {
	analyticsCfg := cfg.Analytics
	publishers := make([]analytics_publisher.Publisher, 0)
	if analyticsCfg.Enabled {
		for _, publisherName := range analyticsCfg.EnabledPublishers {
			switch publisherName {
			case MoesifAnalyticsPublisher:
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

	// Traffic logging is a standalone consumer, independent of analytics.
	if cfg.TrafficLogging.Enabled {
		publishers = append(publishers, analytics_publisher.NewLog(&cfg.TrafficLogging))
		slog.Info("Traffic logging (stdout) publisher added")
	}

	if len(publishers) == 0 {
		slog.Debug("No analytics publishers found. Collected events will not be published.")
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

		toMs := func(secs int64, nanos int32) int64 {
			return (secs * 1000) + int64(nanos)/1_000_000
		}
		toUs := func(secs int64, nanos int32) int64 {
			return (secs * 1_000_000) + int64(nanos)/1000
		}

		// Moesif-oriented latencies (milliseconds).
		lastRx := toMs(properties.TimeToLastRxByte.Seconds, properties.TimeToLastRxByte.Nanos)
		firstUpTx := toMs(properties.TimeToFirstUpstreamTxByte.Seconds, properties.TimeToFirstUpstreamTxByte.Nanos)
		firstUpRx := toMs(properties.TimeToFirstUpstreamRxByte.Seconds, properties.TimeToFirstUpstreamRxByte.Nanos)
		lastUpRx := toMs(properties.TimeToLastUpstreamRxByte.Seconds, properties.TimeToLastUpstreamRxByte.Nanos)
		lastDownTx := toMs(properties.TimeToLastDownstreamTxByte.Seconds, properties.TimeToLastDownstreamTxByte.Nanos)

		event.Latencies = &dto.Latencies{
			BackendLatency:           lastUpRx - firstUpTx,
			RequestMediationLatency:  firstUpTx - lastRx,
			ResponseLatency:          lastDownTx - firstUpRx,
			ResponseMediationLatency: lastDownTx - lastUpRx,
		}

		// Traffic-log latencies (microseconds), derived from the same timepoints
		// at full precision. Kept separate from the millisecond Latencies above so
		// Moesif's units are unaffected.
		lastRxUs := toUs(properties.TimeToLastRxByte.Seconds, properties.TimeToLastRxByte.Nanos)
		firstUpTxUs := toUs(properties.TimeToFirstUpstreamTxByte.Seconds, properties.TimeToFirstUpstreamTxByte.Nanos)
		firstUpRxUs := toUs(properties.TimeToFirstUpstreamRxByte.Seconds, properties.TimeToFirstUpstreamRxByte.Nanos)
		lastDownTxUs := toUs(properties.TimeToLastDownstreamTxByte.Seconds, properties.TimeToLastDownstreamTxByte.Nanos)

		trafficLatencies := dto.TrafficLogLatencies{
			DurationUs:                lastDownTxUs,           // DS_RX_BEG → DS_TX_END
			RequestMediationLatencyUs: firstUpTxUs - lastRxUs, // DS_RX_END → US_TX_BEG
		}

		// US_TX_END → US_RX_BEG: time the backend spent before sending the first response byte (TTFB).
		if properties.TimeToLastUpstreamTxByte != nil {
			lastUpTxUs := toUs(properties.TimeToLastUpstreamTxByte.Seconds, properties.TimeToLastUpstreamTxByte.Nanos)
			trafficLatencies.BackendLatencyUs = firstUpRxUs - lastUpTxUs
		}

		// US_RX_BEG → DS_TX_BEG: gateway overhead processing the first response byte before writing downstream.
		if properties.TimeToFirstDownstreamTxByte != nil {
			firstDownTxUs := toUs(properties.TimeToFirstDownstreamTxByte.Seconds, properties.TimeToFirstDownstreamTxByte.Nanos)
			trafficLatencies.ResponseMediationLatencyUs = firstDownTxUs - firstUpRxUs
		}

		event.TrafficLogLatencies = &trafficLatencies
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

	// Auth-context metadata (type, issuer, credential/token IDs, audience, scopes, custom
	// claims), stamped generically by the collector system policy for any authenticated
	// request regardless of auth type; plus PropKeyMetadata, the JSON-encoded raw
	// SharedContext.Metadata bag (see dto.PropKeyMetadata doc comment). Key names match
	// the raw metadata 1:1 (see dto.PropKeyAuth* doc comment), so no case translation is
	// needed here.
	for _, key := range []string{
		dto.PropKeyAuthType, dto.PropKeyAuthIssuer, dto.PropKeyAuthCredentialID,
		dto.PropKeyAuthTokenID, dto.PropKeyAuthAudience, dto.PropKeyAuthScopes, dto.PropKeyAuthProperties,
		dto.PropKeyAuthAuthorized, dto.PropKeyMetadata,
	} {
		if v, exists := keyValuePairsFromMetadata[key]; exists && v != "" {
			event.Properties[key] = v
		}
	}

	// Prepare Subscription
	subscription := &dto.Subscription{}
	subscription.BillingCustomerID = keyValuePairsFromMetadata[BillingCustomerIDKey]
	subscription.BillingSubscriptionID = keyValuePairsFromMetadata[BillingSubscriptionIDKey]
	subscription.Status = keyValuePairsFromMetadata[SubscriptionStatusKey]
	subscription.PlanName = keyValuePairsFromMetadata[SubscriptionPlanNameKey]
	event.Subscription = subscription

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
	// Resolve responseContentType for all API kinds. The analytics system policy captures
	// it from the response headers into analytics metadata (response_content_type) because
	// the Envoy access log does not carry response headers. Prefer that value; fall back to
	// the access-log header if present, then to Unknown.
	responseContentType := Unknown
	if ct, ok := keyValuePairsFromMetadata["response_content_type"]; ok && ct != "" {
		responseContentType = ct
	} else if logEntry.Response != nil {
		if contentTypeHeader := logEntry.Response.GetResponseHeaders()["content-type"]; contentTypeHeader != "" {
			responseContentType = contentTypeHeader
		}
	}
	event.Properties["responseContentType"] = responseContentType
	if logEntry.Response != nil {
		event.Properties["responseSize"] = logEntry.Response.ResponseBodyBytes
	}

	// requestSize is common to all API kinds; mirror responseSize using the Envoy access-log byte count.
	if request != nil {
		event.Properties["requestSize"] = request.GetRequestBodyBytes()
	}

	//Adding request and response headers for the analytics event
	if requestHeaders, exists := keyValuePairsFromMetadata[RequestHeadersKey]; exists {
		event.Properties[dto.PropKeyRequestHeaders] = requestHeaders
	}
	if responseHeaders, exists := keyValuePairsFromMetadata[ResponseHeadersKey]; exists {
		event.Properties[dto.PropKeyResponseHeaders] = responseHeaders
	}

	// Optionally attach request and response payloads when enabled via the collector.
	if c.cfg.Collector.RequestBody {
		if requestPayload, ok := keyValuePairsFromMetadata[dto.PropKeyRequestPayload]; ok && requestPayload != "" {
			event.Properties[dto.PropKeyRequestPayload] = requestPayload
			slog.Debug("Analytics request payload captured", "size_bytes", len(requestPayload))
		}
	}
	if c.cfg.Collector.ResponseBody {
		if responsePayload, ok := keyValuePairsFromMetadata[dto.PropKeyResponsePayload]; ok && responsePayload != "" {
			event.Properties[dto.PropKeyResponsePayload] = responsePayload
			slog.Debug("Analytics response payload captured", "size_bytes", len(responsePayload))
		}
	}

	if keyValuePairsFromMetadata[APITypeKey] != "" && keyValuePairsFromMetadata[APITypeKey] == "Mcp" {
		mcpAnalytics := make(map[string]interface{})
		if mcpSessionID, ok := keyValuePairsFromMetadata["mcp_session_id"]; ok && mcpSessionID != "" {
			mcpAnalytics["sessionId"] = mcpSessionID
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
		// Additionally, if there's an error code in the response properties from policies, add it to the response props
		if mcpErrorCode, ok := keyValuePairsFromMetadata["mcpErrorCode"]; ok && mcpErrorCode != "" {
			if _, exists := mcpAnalytics["errorCode"]; !exists {
				if code, err := strconv.Atoi(mcpErrorCode); err == nil {
					mcpAnalytics["errorCode"] = code
				} else {
					slog.Debug("Invalid MCP error code format; storing raw value", "mcpErrorCode", mcpErrorCode, "error", err)
					mcpAnalytics["errorCode"] = mcpErrorCode
				}
			} else {
				slog.Debug("MCP error code already exists in mcpAnalytics, skipping adding it again", "mcpErrorCode", mcpErrorCode)
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

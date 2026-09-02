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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"strconv"
	"sync"
	"time"

	v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	analytics_publisher "github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/publishers"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
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

	// InternalLoopbackMetadataKey is the analytics metadata key for the marker added to the proxy's
	// internal loopback request to the provider.
	InternalLoopbackMetadataKey string = "x-wso2-internal-loopback"

	// PropInternalLoopbackProvider marks the provider-side loopback hop of a proxy call
	// so Process can drop the duplicate event before publisher fan-out.
	PropInternalLoopbackProvider string = "isInternalLoopbackProvider"
)

// Analytics represents analytics collector service.
type Analytics struct {
	// cfg represents the server configuration.
	cfg *config.Config
	// publishers represents the publishers.
	publishers []analytics_publisher.Publisher
	// missingDirectPeerWarn limits the "direct remote address unavailable" warning to one
	// line per process which otherwise repeating it once per request would flood the logs
	missingDirectPeerWarn sync.Once
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
		logPublisher, err := analytics_publisher.NewLog(&cfg.TrafficLogging)
		if err != nil {
			// Fail closed. Continuing without the configured sinks would leave
			// traffic logging on stdout or on nothing at all, and the stdout path
			// writes request and response bodies into the container log — the
			// exact disclosure a file or http sink is configured to prevent. This
			// is a startup-time condition verified locally, so refusing to run is
			// the correct outcome; config.Validate has already proven every sink
			// is constructible, which makes this branch defensive rather than
			// reachable in practice.
			slog.Error("Failed to initialize traffic-logging sinks; refusing to start", "error", err)
			panic(fmt.Sprintf("traffic logging configuration is unusable: %v", err))
		}
		publishers = append(publishers, logPublisher)
		slog.Info("Traffic logging publisher added", "outputs", cfg.TrafficLogging.Outputs)
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

	// Suppress the internal loopback provider hop of an LLM proxy call so a single client
	// call is counted once, detecting using the marker header set by the proxy
	// when carrying on its loopback forward
	if v, ok := analyticEvent.Properties[PropInternalLoopbackProvider].(bool); ok && v {
		correlationID := ""
		if analyticEvent.MetaInfo != nil {
			correlationID = analyticEvent.MetaInfo.CorrelationID
		}
		apiType := ""
		if analyticEvent.API != nil {
			apiType = analyticEvent.API.APIType
		}
		slog.Debug("Suppressing internal loopback provider analytics event",
			"apiType", apiType,
			"correlationId", correlationID,
		)
		return
	}

	for _, publisher := range c.publishers {
		publisher.Publish(analyticEvent)
	}

}

// Close shuts down every publisher that holds resources or buffers events,
// bounded by ctx. Publishers that do not implement Closer are skipped.
//
// Call this only after the ALS server has stopped accepting events, so the flush
// does not race new arrivals. Without it, a buffering publisher (the traffic-log
// HTTP sink, Moesif) loses its in-flight batch on every pod restart, rolling update
// and scale-down.
func (c *Analytics) Close(ctx context.Context) error {
	var errs []error
	for _, publisher := range c.publishers {
		closer, ok := publisher.(analytics_publisher.Closer)
		if !ok {
			continue
		}
		if err := closer.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// isInternalLoopbackHop identifies the provider-side hop of an LLM proxy loopback call by requiring
// both the proxy marker and the unforgeable direct TCP peer; if the peer is unavailable, it fails open, warns once, and allows duplicates.
func (c *Analytics) isInternalLoopbackHop(apiType, marker, directRemoteIP, downstreamListener, correlationID string) bool {
	if apiType != "LlmProvider" || marker != "true" {
		return false
	}
	if directRemoteIP == "" {
		// Warn once so the resulting duplicates are visible at the default (info) log level
		c.missingDirectPeerWarn.Do(func() {
			slog.Warn("Internal loopback marker present but downstream direct remote address is unavailable; not suppressing event (logged once per process)",
				"apiType", apiType,
				"correlationId", correlationID,
			)
		})
		// Enabling debug shows whether this affects all traffic or only some ingress paths.
		slog.Debug("Internal loopback marker present but downstream direct remote address is unavailable",
			"apiType", apiType,
			"downstreamListener", downstreamListener,
			"correlationId", correlationID,
		)
		return false
	}
	return isLoopbackAddress(directRemoteIP)
}

// isLoopbackAddress reports whether ip is an IPv4/IPv6 loopback address.
func isLoopbackAddress(ip string) bool {
	if ip == "127.0.0.1" || ip == "::1" {
		return true
	}
	if parsed := net.ParseIP(ip); parsed != nil {
		return parsed.IsLoopback()
	}
	return false
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
	// subType currently mirrors apiType; populated here so it flows through the analytics event.
	extendedAPI.SubType = keyValuePairsFromMetadata[APITypeKey]
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
			// Total request duration: downstream request received → downstream
			// response sent (DS_RX_BEG → DS_TX_END). None of the other four is a
			// substitute — ResponseLatency starts at the first upstream response byte,
			// so it omits everything the request spent getting there, and
			// BackendLatency covers only the upstream leg. This is the same span the
			// traffic-log path already computes as DurationUs, in milliseconds to match
			// its siblings here.
			Duration: lastDownTx,
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

	// Physical socket peer of the downstream connection. Empty when Envoy did not report it.
	directRemoteIP := logEntry.GetCommonProperties().GetDownstreamDirectRemoteAddress().GetSocketAddress().GetAddress()
	// Address of the listener that accepted the connection. Only used to diagnose a missing directRemoteIP
	downstreamListener := logEntry.GetCommonProperties().GetDownstreamLocalAddress().GetSocketAddress().GetAddress()

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

	// Flag the internal loopback provider hop of an LLM proxy call — the duplicate of the
	// proxy's own event — so a single client call is counted once.
	if c.isInternalLoopbackHop(extendedAPI.APIType, keyValuePairsFromMetadata[InternalLoopbackMetadataKey],
		directRemoteIP, downstreamListener, metaInfo.CorrelationID) {
		event.Properties[PropInternalLoopbackProvider] = true
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

	if keyValuePairsFromMetadata[APITypeKey] == string(policy.APIKindAgent) {
		event.Properties[AgentAnalyticsProperty] = buildAgentAnalytics(
			keyValuePairsFromMetadata, logEntry, operation.APIMethod, event.ProxyResponseCode)
	}

	return event
}

// AgentAnalyticsProperty is the event property the Agent analytics envelope is
// assembled under.
//
// It is an envelope keyed by domain rather than a flat block named after one protocol,
// so a later Agent analytics domain can be added as a sibling of `a2a` without its
// fields having to be told apart from A2A's by name. It replaces the earlier flat
// `a2aAnalytics` property; nothing publishes that key any more, and a publisher test
// asserts its absence, because a consumer reading both would silently see two
// different shapes of the same event depending on the gateway version.
const AgentAnalyticsProperty = "agentAnalytics"

// buildAgentAnalytics assembles the published Agent analytics envelope for one event.
//
// Every dimension a downstream A2A dashboard needs that is not already a first-class
// field on the event lives here. Latency stays on event.Latencies, which the ALS
// timepoints already give for every kind; consumer identity stays on the auth-context
// properties, which are populated generically for any authenticated request. Only the
// facts specific to A2A — which operation, over which transport, and whether the agent
// actually succeeded — are assembled here.
//
// The result is typed rather than a map. What this function produces is an external
// contract, and a map is a contract only by convention: a renamed key or a value that
// quietly changes type stays invisible until a dashboard goes blank.
//
// Nothing in here is aggregated. Counts, distinct-consumer rollups and success rates
// are computed downstream; this function's whole contract is that the dimensions each
// of those needs are present, bounded where they must be, and derived correctly.
func buildAgentAnalytics(
	metadata map[string]string,
	logEntry *v3.HTTPAccessLogEntry,
	requestMethod string,
	statusCode int,
) *dto.AgentAnalytics {
	a2a := &dto.A2AAnalytics{}

	// The canonical operation, stamped by the kernel from the bound chain key rather
	// than reported by anything that parsed the request — so it names the operation
	// whose policies actually ran.
	operation := metadata[ResolvedOperationKey]
	terminalReason := metadata[TerminalReasonKey]

	a2a.RequestType = a2aRequestType(operation, terminalReason, requestMethod)

	if a2a.RequestType != A2ARequestTypeOperation {
		// A card fetch or a preflight. It gets no operation, no outcome and no
		// transport: it is reported so the traffic is visible, and deliberately not
		// shaped like an invocation so nothing downstream can roll it in with one.
		return &dto.AgentAnalytics{A2A: a2a}
	}

	if operation == "" {
		// An invocation whose operation could not be determined — the request named an
		// operation this protocol version does not define, or was not a well-formed
		// envelope. It is grouped rather than dropped: these are failures a success
		// rate has to count, and grouping keeps the operation dimension bounded when
		// the value that failed to resolve was caller-supplied.
		operation = A2AOperationUnknown
	}
	a2a.Operation = operation

	// Transport, protocol version and the request identifiers, extracted once by the
	// a2a resolver and passed through the analytics system policy. Transport is a
	// separate dimension from operation on purpose: the two A2A bindings of one
	// operation resolve to the same chain and must aggregate to the same operation,
	// while still being distinguishable.
	request := decodeA2ARequestBlock(metadata[A2ARequestPropertiesKey])
	a2a.Transport = request.Transport
	a2a.ProtocolVersion = request.ProtocolVersion
	if !request.A2ARequestAnalytics.IsEmpty() {
		a2a.Request = &request.A2ARequestAnalytics
	}

	if response := decodeA2AResponseBlock(metadata[A2AResponsePropertiesKey]); !response.IsEmpty() {
		a2a.Response = response
	}

	applyA2AResolverAttributes(a2a, metadata)

	outcome, origin := a2aOutcome(a2a, terminalReason, statusCode,
		logEntry.GetCommonProperties().GetUpstreamRemoteAddress() != nil)
	a2a.Outcome = outcome
	a2a.FailureOrigin = origin

	return &dto.AgentAnalytics{A2A: a2a}
}

// a2aRequestBlock is the wire shape of the analytics system policy's request block.
//
// It is the published request object plus the two protocol facts, which the policy
// assembles alongside the identifiers because they come from the same resolver output —
// but which belong at the A2A level of the published model, not inside `request`: they
// describe the invocation, not what the caller asked the agent to do. Embedding keeps
// the published type the single definition of the fields it owns, rather than a second
// struct listing them again.
type a2aRequestBlock struct {
	dto.A2ARequestAnalytics
	Transport       string `json:"transport,omitempty"`
	ProtocolVersion string `json:"protocolVersion,omitempty"`
}

// decodeA2ARequestBlock and decodeA2AResponseBlock unmarshal the blocks the analytics
// system policy serialized at the Envoy metadata boundary.
//
// A block that will not parse yields an empty one and a debug line, rather than being
// carried onto the event as a raw string the way the earlier flat map did. On a typed
// envelope there is nowhere honest to put it: a consumer reading a named field would
// see it as absent either way, and a stray string where an object belongs is the kind
// of shape change the typing exists to prevent.
func decodeA2ARequestBlock(encoded string) a2aRequestBlock {
	var block a2aRequestBlock
	if encoded == "" {
		return block
	}
	if err := json.Unmarshal([]byte(encoded), &block); err != nil {
		slog.Debug("Failed to unmarshal A2A request analytics properties",
			"key", A2ARequestPropertiesKey, "error", err)
		return a2aRequestBlock{}
	}
	return block
}

func decodeA2AResponseBlock(encoded string) *dto.A2AResponseAnalytics {
	if encoded == "" {
		return nil
	}
	var block dto.A2AResponseAnalytics
	if err := json.Unmarshal([]byte(encoded), &block); err != nil {
		slog.Debug("Failed to unmarshal A2A response analytics properties",
			"key", A2AResponsePropertiesKey, "error", err)
		return nil
	}
	return &block
}

// applyA2AResolverAttributes fills in the two bounded protocol facts from the
// resolver's own attributes, for a request the engine refused before any policy ran.
//
// Fallback only, and never an override: on a request that resolved, the analytics
// system policy already put both into the request-properties block, and that block
// is the authority. This adds them for a rejection, where that policy never ran —
// so a version-refused event still says which binding and which configured version
// it was aimed at, instead of only which API.
//
// Both values are fixed at route ingest and drawn from closed sets (a two-valued
// transport enum, a registered protocol version), which is what makes them safe to
// put on an event a dashboard groups by. The version the *caller* stated is not here
// and must not be: it is unbounded and attacker-chosen.
func applyA2AResolverAttributes(a2a *dto.A2AAnalytics, metadata map[string]string) {
	if a2a.Transport == "" {
		a2a.Transport = metadata[A2ATransportAttributeKey]
	}
	if a2a.ProtocolVersion == "" {
		a2a.ProtocolVersion = metadata[A2AProtocolVersionAttributeKey]
	}
}

// a2aRequestType classifies a request on an Agent's routes.
//
// An Agent serves three shapes of traffic on one context: its A2A operations, its
// public Agent Card, and the CORS preflights for both. Only the first is an
// invocation. A resolved operation is the discriminator rather than a path match,
// because the card's gateway-facing path is author-configurable while the a2a resolver
// is attached to operation routes and nothing else.
//
// A request the resolver rejected has no operation either, and it must not be
// misfiled: it was aimed at an operation route and is an attempted invocation that
// failed, not a card fetch. That is what the terminal reason distinguishes — either
// of the two pre-chain reasons, since a request refused for the protocol version it
// stated was every bit as much an attempted invocation as one whose payload named no
// known operation.
func a2aRequestType(operation, terminalReason, requestMethod string) string {
	switch {
	case operation != "" || isA2APreChainRefusal(terminalReason):
		return A2ARequestTypeOperation
	case requestMethod == "OPTIONS":
		return A2ARequestTypePreflight
	default:
		return A2ARequestTypeAgentCard
	}
}

// isA2APreChainRefusal reports whether the engine refused this request before any
// policy chain was bound. Both reasons mean the same thing for classification — an
// attempted invocation that never reached the agent — and they are kept apart only
// so a consumer can tell a protocol-version problem from a payload one.
func isA2APreChainRefusal(terminalReason string) bool {
	return terminalReason == constants.TerminalReasonResolutionFailed ||
		terminalReason == constants.TerminalReasonA2AVersionRejected
}

// a2aOutcome derives whether an Agent invocation succeeded and, if it did not, which
// component is answerable for that.
//
// The HTTP status is not the answer on its own. A JSON-RPC error rides a 200, so a
// status-only reading reports a failed invocation as a success; and a policy denial
// and an agent's own rejection can arrive as the same status, so a status-only reading
// blames the agent for the gateway's refusals. The rules are ordered from the most
// specific fact available to the least:
//
//  1. The engine said what ended the request — a policy short-circuit, or a payload it
//     could not resolve to an operation. Authoritative about *who produced the
//     response*: the engine is the only component that knows, and it knows for certain.
//     Whether that response is a failure is a separate question, and for a
//     short-circuit the status answers it — a policy can answer as well as refuse.
//  2. A 5xx: the agent's if the request reached it, otherwise the gateway's.
//  3. A 4xx: the agent's if the request reached it; otherwise the gateway refused it
//     for something the caller controls, which is a client fault.
//  4. A success status whose body was read: the A2A result decides. An error object
//     inside a 200 is the agent's failure — the case the whole function exists for.
//  5. A success status whose body was *not* read: undetermined, unless the transport
//     itself makes the status authoritative. See a2aSuccessStatusOutcome.
//
// Known gap: a response-phase policy denial is not covered by rule 1 — only
// request-phase short-circuits stamp the terminal reason — so it lands in rule 2 or 3
// and is attributed to the upstream the request did reach.
func a2aOutcome(a2a *dto.A2AAnalytics, terminalReason string, statusCode int, upstreamContacted bool) (string, string) {
	switch terminalReason {
	case constants.TerminalReasonPolicyDenied:
		// A policy produced the response instead of the agent. Refusing is the
		// common case and the one this attribution exists for: an auth denial and
		// an upstream's own 401 are otherwise indistinguishable, and a success-rate
		// dashboard that cannot separate them blames the agent for the gateway's
		// rejections.
		//
		// But a policy can also *answer*. A managed protected Agent Card is served
		// by the gateway's own A2A policy with a 200, and the request stopping at
		// the gateway is the feature rather than a fault. Treating every
		// short-circuit as a failure would move every locally served card into the
		// failure bucket and make the Agent look broken in proportion to how often
		// its card is fetched. So the status decides, and a successful one falls
		// through to the ordinary derivation below.
		if statusCode >= 400 {
			return A2AOutcomeFailure, A2AFailureOriginPolicy
		}
	case constants.TerminalReasonResolutionFailed, constants.TerminalReasonA2AVersionRejected:
		// The payload named no operation this protocol version defines, was not a
		// well-formed envelope at all, or stated a protocol version this Agent does
		// not expose. The caller's, not the agent's — the agent never saw it.
		return A2AOutcomeFailure, A2AFailureOriginClient
	}
	if statusCode >= 500 {
		if upstreamContacted {
			return A2AOutcomeFailure, A2AFailureOriginUpstream
		}
		return A2AOutcomeFailure, A2AFailureOriginGateway
	}
	if statusCode >= 400 {
		if upstreamContacted {
			return A2AOutcomeFailure, A2AFailureOriginUpstream
		}
		return A2AOutcomeFailure, A2AFailureOriginClient
	}
	if a2a.Response != nil && a2a.Response.IsError != nil {
		if *a2a.Response.IsError {
			return A2AOutcomeFailure, A2AFailureOriginUpstream
		}
		return A2AOutcomeSuccess, ""
	}
	return a2aSuccessStatusOutcome(a2a), ""
}

// a2aSuccessStatusOutcome decides what a 2xx means when the response body yielded no
// A2A result — it was empty, unreadable, or the response policy never saw it.
//
// The answer depends on the transport, because the two A2A bindings put the outcome in
// different places:
//
//   - JSON-RPC multiplexes everything onto one endpoint and answers 200 whether the
//     call succeeded or failed. The status carries no outcome information whatsoever,
//     so with no readable body there is nothing to conclude. Calling it a success would
//     manufacture the exact false-success this derivation exists to prevent.
//   - HTTP+JSON is REST-shaped: an error is a real error status, not a 200. A 2xx is
//     therefore itself the agent's statement of success, and a bodiless one (a 204 from
//     DeleteTaskPushNotificationConfig, say) is a determined success rather than a
//     missing observation.
//
// An unrecognised or absent transport is treated as undetermined rather than assumed
// REST-shaped: this is the fail-honest direction, and it is not reachable in practice —
// every operation route's resolver stamps the transport at ingest.
func a2aSuccessStatusOutcome(a2a *dto.A2AAnalytics) string {
	if a2a.Transport == a2aTransportHTTPJSON {
		return A2AOutcomeSuccess
	}
	return A2AOutcomeUnknown
}

func (c *Analytics) getAnonymousApp() *dto.Application {
	application := &dto.Application{}
	application.ApplicationID = anonymousValue
	application.ApplicationName = anonymousValue
	application.KeyType = anonymousValue
	application.ApplicationOwner = anonymousValue
	return application
}

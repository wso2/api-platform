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

package publishers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/moesif/moesifapi-go"
	"github.com/moesif/moesifapi-go/models"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
)

const (
	anonymous         = "anonymous"
	userIDPropertyKey = "x-wso2-user-id"
)

// Moesif represents a Moesif publisher.
type Moesif struct {
	cfg       *config.MoesifPublisherConfig
	api       moesifapi.API
	events    []*models.EventModel
	mu        sync.Mutex
	done      chan struct{}
	closeOnce sync.Once
	// closeErr is the result of the one-and-only shutdown flush, so every caller
	// of Close sees the same outcome rather than the second one getting a
	// misleading nil.
	closeErr error
}

// MoesifConfig holds the configs specific for the Moesif publisher.
// Deprecated: Use config.MoesifPublisherConfig directly instead.
type MoesifConfig struct {
	ApplicationID      string `mapstructure:"application_id"`
	BaseURL            string `mapstructure:"moesif_base_url"`
	PublishInterval    int    `mapstructure:"publish_interval"`
	EventQueueSize     int    `mapstructure:"event_queue_size"`
	BatchSize          int    `mapstructure:"batch_size"`
	TimerWakeupSeconds int    `mapstructure:"timer_wakeup_seconds"`
}

// NewMoesif creates a new Moesif publisher.
func NewMoesif(moesifCfg *config.MoesifPublisherConfig) *Moesif {
	if moesifCfg == nil {
		slog.Error("Moesif config is nil")
		return nil
	}

	// Read moesifApplicationId from environment variable first, fallback to config
	moesifApplicationId := os.Getenv("MOESIF_KEY")
	if moesifApplicationId == "" {
		moesifApplicationId = moesifCfg.ApplicationID
	}

	// Moesif Client Configs
	eventQueueSize, batchSize, timerWakeupSeconds :=
		moesifCfg.EventQueueSize,
		moesifCfg.BatchSize,
		moesifCfg.TimerWakeupSeconds

	var apiEndpoint *string
	if moesifCfg.BaseURL != "" {
		apiEndpoint = &moesifCfg.BaseURL
	}
	apiClient := moesifapi.NewAPI(moesifApplicationId, apiEndpoint, eventQueueSize, batchSize, timerWakeupSeconds)
	moesif := &Moesif{
		cfg:    moesifCfg,
		events: []*models.EventModel{},
		api:    apiClient,
		mu:     sync.Mutex{},
		done:   make(chan struct{}),
	}
	go func() {
		ticker := time.NewTicker(time.Duration(moesifCfg.PublishInterval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-moesif.done:
				return
			case <-ticker.C:
				moesif.mu.Lock()
				if len(moesif.events) > 0 {
					slog.Debug(fmt.Sprintf("Publishing %d events to Moesif", len(moesif.events)))
					err := moesif.api.QueueEvents(moesif.events)
					if err != nil {
						slog.Error("Error publishing events to Moesif", "error", err)
					}
					moesif.events = []*models.EventModel{}
				}
				moesif.mu.Unlock()
			}
		}
	}()
	return moesif
}

// Close stops the background publishing goroutine and flushes whatever it had
// not yet published, satisfying Closer so Analytics.Close reaches it during
// graceful shutdown. Safe to call multiple times.
//
// Stopping the ticker alone is not enough: events accumulate in m.events between
// ticks, so a shutdown landing mid-interval would abandon up to one full
// interval's worth. They are handed to the client and then flushed, because
// QueueEvents only moves them into the client's own queue — which has its own
// timer and would otherwise be discarded when the process exits.
//
// The flush is bounded by ctx. moesifapi-go v1.1.5 exposes no context or
// configurable timeout on QueueEvents/Flush — Flush is synchronous and subject
// only to the SDK's own HTTP defaults — so it is run on its own goroutine and
// abandoned if ctx expires first. That goroutine may briefly outlive Close; the
// process is exiting, and the alternative is letting an unreachable Moesif
// endpoint hold shutdown open past the operator's budget until the kubelet
// SIGKILLs the pod, which would lose the traffic-log sinks' flush too.
func (m *Moesif) Close(ctx context.Context) error {
	m.closeOnce.Do(func() {
		if m.done != nil {
			close(m.done)
		}

		// Serialised with the ticker's publish path by the same mutex, so a tick
		// already in flight completes first and this then finds the buffer empty
		// — each event is queued exactly once.
		m.mu.Lock()
		pending := m.events
		m.events = nil
		m.mu.Unlock()

		if m.api == nil {
			return
		}

		// Buffered so the goroutine can always finish and exit even after ctx
		// expired and nobody is left reading.
		flushed := make(chan error, 1)
		go func() {
			var err error
			if len(pending) > 0 {
				slog.Info("Flushing buffered Moesif events on shutdown", "count", len(pending))
				if qErr := m.api.QueueEvents(pending); qErr != nil {
					err = fmt.Errorf("queueing %d buffered event(s) on shutdown: %w", len(pending), qErr)
				}
			}
			m.api.Flush()
			flushed <- err
		}()

		select {
		case err := <-flushed:
			m.closeErr = err
		case <-ctx.Done():
			m.closeErr = fmt.Errorf("moesif shutdown flush did not complete within the "+
				"shutdown budget; %d buffered event(s) may be lost: %w", len(pending), ctx.Err())
		}
		if m.closeErr != nil {
			slog.Error("Moesif shutdown flush did not complete cleanly", "error", m.closeErr)
		}
	})
	return m.closeErr
}

// Publish publishes an event to Moesif.
func (m *Moesif) Publish(event *dto.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	slog.Debug("Preparing event to be published to Moesif")
	uri := event.API.APIContext + event.Operation.APIResourceTemplate
	if event.Operation.APIResourceTemplate != "" {
		uri = event.Operation.APIResourceTemplate
	}

	// Headers are only published when the analytics-header-filter policy is configured
	// and has emitted the (already allow/deny filtered) header set into event metadata.
	// When it is not configured, no headers are sent at all.
	headers := map[string]interface{}{}
	if rawReqHeaders, ok := event.Properties["requestHeaders"]; ok && rawReqHeaders != nil {
		slog.Debug("Request headers (PUBLISHER): ", "requestHeaders", rawReqHeaders)
		if jsonStr, ok := rawReqHeaders.(string); ok {
			var hMap map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &hMap); err == nil && len(hMap) > 0 {
				slog.Debug("Unmarshalled hMap (PUBLISHER): ", "requestHeaders", hMap)
				headers = hMap
			} else if err != nil {
				slog.Warn("Failed to unmarshal request headers", "error", err)
			}
		}
	}

	rspHeaders := map[string]interface{}{}
	if rawRspHeaders, ok := event.Properties["responseHeaders"]; ok && rawRspHeaders != nil {
		slog.Debug("Response headers (PUBLISHER): ", "responseHeaders", rawRspHeaders)
		if jsonStr, ok := rawRspHeaders.(string); ok {
			var hMap map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &hMap); err == nil && len(hMap) > 0 {
				slog.Debug("Unmarshalled hMap (PUBLISHER): ", "responseHeaders", hMap)
				rspHeaders = hMap
			} else if err != nil {
				slog.Warn("Failed to unmarshal response headers", "error", err)
			}
		}
	}

	req := models.EventRequestModel{
		Time:       &event.RequestTimestamp,
		Uri:        uri,
		Verb:       event.Operation.APIMethod,
		ApiVersion: &event.API.APIVersion,
		IpAddress:  &event.UserIP,
		Headers:    headers,
		Body:       nil,
	}
	respTime := event.RequestTimestamp
	if event.Latencies != nil {
		respTime = event.RequestTimestamp.Add(time.Duration(event.Latencies.ResponseLatency) * time.Millisecond)
	}

	rsp := models.EventResponseModel{
		Time:    &respTime,
		Status:  event.ProxyResponseCode,
		Headers: rspHeaders,
	}

	// Medatadata Map for the event
	metadataMap := make(map[string]interface{})
	// API Metadata
	metadataMap["correlationId"] = event.MetaInfo.CorrelationID
	metadataMap["apiContext"] = event.API.APIContext
	metadataMap["apiName"] = event.API.APIName
	metadataMap["apiVersion"] = event.API.APIVersion
	metadataMap["apiType"] = event.API.APIType
	// subType currently mirrors apiType (populated from the same source on the event).
	metadataMap["subType"] = event.API.SubType
	metadataMap["apiId"] = event.API.APIID
	metadataMap["projectId"] = event.API.ProjectID

	// AI Metadata.
	// Only include aiMetadata and aiTokenUsage for LlmProvider and LlmProxy events.
	if event.API.APIType == "LlmProvider" || event.API.APIType == "LlmProxy" {
		// Safely extract aiMetadata with nil check
		if aiMetadataVal, exists := event.Properties["aiMetadata"]; exists && aiMetadataVal != nil {
			if aiMetadata, ok := aiMetadataVal.(dto.AIMetadata); ok {
				slog.Debug("aiMetadata from publisher", "aiMetadata", aiMetadata)
				//[Required Format] key:aiMetadata -> dto.AIMetadata object
				metadataMap["aiMetadata"] = aiMetadata
			} else {
				slog.Warn("AI Metadata property cannot be converted to the required format")
			}
		} else {
			slog.Warn("AI Metadata property cannot be found in the event properties")
		}

		// Safely extract aiTokenUsage with nil check
		if aiTokenUsageVal, exists := event.Properties["aiTokenUsage"]; exists && aiTokenUsageVal != nil {
			if aiTokenUsage, ok := aiTokenUsageVal.(dto.AITokenUsage); ok {
				slog.Debug("tokenUsage from publisher", "tokenUsage", aiTokenUsage)
				//[Required Format] key:aiTokenUsage ->  dto.AITokenUsage object
				metadataMap["aiTokenUsage"] = aiTokenUsage
			} else {
				slog.Warn("Token usage property cannot be converted to the required format")
			}
		} else {
			slog.Warn("AI Token Usage data cannot be found in the event properties")
		}
	}

	// MCP Analytics
	if event.API.APIType == "Mcp" {
		if mcpAnalytics, ok := event.Properties["mcpAnalytics"]; ok && mcpAnalytics != nil {
			metadataMap["mcpAnalytics"] = mcpAnalytics
		}
	}

	// A2A Analytics. Gated on the API kind for the same reason the MCP block above
	// is: an Agent's dimensions are meaningless on any other kind, and forwarding
	// the key unconditionally would put an empty object on every event.
	if event.API.APIType == "Agent" {
		if a2aAnalytics, ok := event.Properties["a2aAnalytics"]; ok && a2aAnalytics != nil {
			metadataMap["a2aAnalytics"] = a2aAnalytics
		}
	}

	// Attach request/response payloads to metadata when present in event properties.
	if requestPayload, ok := event.Properties["request_payload"]; ok && requestPayload != nil {
		metadataMap["request_payload"] = requestPayload
	}
	if responsePayload, ok := event.Properties["response_payload"]; ok && responsePayload != nil {
		metadataMap["response_payload"] = responsePayload
	}

	// responseContentType
	if responseContentType, ok := event.Properties["responseContentType"]; ok && responseContentType != nil {
		metadataMap["responseContentType"] = responseContentType
	}

	// responseSize
	if responseSize, ok := event.Properties["responseSize"]; ok && responseSize != nil {
		metadataMap["responseSize"] = responseSize
	}

	// requestSize
	if requestSize, ok := event.Properties["requestSize"]; ok && requestSize != nil {
		metadataMap["requestSize"] = requestSize
	}

	// Advanced latency info
	if event.Latencies != nil {
		metadataMap["backendLatency"] = event.Latencies.BackendLatency
		metadataMap["requestMediationLatency"] = event.Latencies.RequestMediationLatency
		metadataMap["responseLatency"] = event.Latencies.ResponseLatency
		metadataMap["responseMediationLatency"] = event.Latencies.ResponseMediationLatency
		// Total request duration. The other four are all partial spans, so a
		// latency-distribution percentile computed from them measures a phase rather
		// than what the caller waited.
		metadataMap["duration"] = event.Latencies.Duration
	}

	// commonName
	if commonName, ok := event.Properties["commonName"]; ok && commonName != nil {
		metadataMap["commonName"] = commonName
	}

	// guardrail metadata
	if llmCost, ok := event.Properties[constants.LLMCostPropertyKey]; ok && llmCost != nil {
		metadataMap[constants.LLMCostPropertyKey] = llmCost
	}
	if isGuardrailHit, ok := event.Properties[constants.GuardrailHitMetadataKey]; ok && isGuardrailHit != nil {
		metadataMap[constants.GuardrailHitMetadataKey] = isGuardrailHit
	}
	if guardrailName, ok := event.Properties[constants.GuardrailNameMetadataKey]; ok && guardrailName != nil {
		metadataMap[constants.GuardrailNameMetadataKey] = guardrailName
	}

	// application metadata
	if event.Application != nil {
		if event.Application.ApplicationID != "" {
			metadataMap["applicationId"] = event.Application.ApplicationID
		}
		if event.Application.ApplicationName != "" {
			metadataMap["applicationName"] = event.Application.ApplicationName
		}
	}

	// subscription metadata
	if event.Subscription != nil {
		if event.Subscription.BillingCustomerID != "" {
			metadataMap["billingCustomerId"] = event.Subscription.BillingCustomerID
		}
		if event.Subscription.BillingSubscriptionID != "" {
			metadataMap["billingSubscriptionId"] = event.Subscription.BillingSubscriptionID
		}
		if event.Subscription.Status != "" {
			metadataMap["subscriptionStatus"] = event.Subscription.Status
		}
		if event.Subscription.PlanName != "" {
			metadataMap["subscriptionPlanName"] = event.Subscription.PlanName
		}
	}

	// isEgress
	if isEgress, ok := event.Properties["isEgress"]; ok && isEgress != nil {
		metadataMap["isEgress"] = isEgress
	}

	// Determine user ID - use from event properties if available, otherwise anonymous
	userID := anonymous
	if userIDVal, ok := event.Properties[userIDPropertyKey]; ok {
		if uid, ok := userIDVal.(string); ok && uid != "" {
			userID = uid
			slog.Debug("Moesif: Using authenticated user ID", "userID", userID)
		}
	}

	eventModel := &models.EventModel{
		Request:  req,
		Response: rsp,
		UserId:   &userID,
		Metadata: metadataMap,
	}
	m.events = append(m.events, eventModel)
	slog.Debug(fmt.Sprintf("Event added to the queue. Queue size: %d", len(m.events)))
	slog.Debug("Events", "events", m.events)
}

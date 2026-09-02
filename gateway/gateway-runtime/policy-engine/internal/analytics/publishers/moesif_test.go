/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	moesifapi "github.com/moesif/moesifapi-go"
	"github.com/moesif/moesifapi-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
)

// createTestMoesifWithoutAPI creates a Moesif publisher without a real API for testing Publish method.
// The api field is nil, but Publish doesn't call api methods - it only queues events.
func createTestMoesifWithoutAPI() *Moesif {
	return &Moesif{
		cfg: &config.MoesifPublisherConfig{
			ApplicationID:      "test-app-id",
			PublishInterval:    5,
			EventQueueSize:     100,
			BatchSize:          10,
			TimerWakeupSeconds: 1,
		},
		api:    nil, // Publish method doesn't call api methods
		events: []*models.EventModel{},
		mu:     sync.Mutex{},
	}
}

// getMetadata extracts the metadata map from an EventModel.
func getMetadata(event *models.EventModel) map[string]interface{} {
	if event.Metadata == nil {
		return nil
	}
	return event.Metadata.(map[string]interface{})
}

// createBaseEvent creates a basic event for testing.
func createBaseEvent() *dto.Event {
	now := time.Now()
	return &dto.Event{
		RequestTimestamp:  now,
		ProxyResponseCode: 200,
		UserAgentHeader:   "test-agent",
		UserIP:            "192.168.1.1",
		Properties:        make(map[string]interface{}),
		API: &dto.ExtendedAPI{
			API: dto.API{
				APIID:      "api-123",
				APIName:    "test-api",
				APIVersion: "v1.0",
				APIType:    "Rest",
				SubType:    "Rest",
			},
			APIContext: "/test",
			ProjectID:  "project-123",
		},
		Operation: &dto.Operation{
			APIMethod:           "GET",
			APIResourceTemplate: "/resource",
		},
		MetaInfo: &dto.MetaInfo{
			CorrelationID: "corr-123",
		},
		Latencies: &dto.Latencies{
			ResponseLatency: 100,
		},
		TrafficLogLatencies: &dto.TrafficLogLatencies{
			DurationUs:                 250000,
			RequestMediationLatencyUs:  50000,
			ResponseMediationLatencyUs: 30000,
			BackendLatencyUs:           40000,
		},
	}
}

func TestNewMoesif_NilConfig(t *testing.T) {
	result := NewMoesif(nil)
	assert.Nil(t, result, "NewMoesif should return nil when config is nil")
}

func TestNewMoesif_DefaultBaseURL(t *testing.T) {
	// Clear MOESIF_KEY env var to ensure we use config
	originalKey := os.Getenv("MOESIF_KEY")
	os.Unsetenv("MOESIF_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("MOESIF_KEY", originalKey)
		}
	}()

	pubCfg := &config.MoesifPublisherConfig{
		ApplicationID:      "test-app-id",
		PublishInterval:    1,
		EventQueueSize:     100,
		BatchSize:          10,
		TimerWakeupSeconds: 1,
		// No BaseURL - should use default
	}

	result := NewMoesif(pubCfg)
	require.NotNil(t, result, "NewMoesif should return a valid publisher")
	t.Cleanup(func() {
		_ = result.Close(context.Background())
	})
}

func TestNewMoesif_EnvVarOverridesConfig(t *testing.T) {
	// Set MOESIF_KEY env var
	os.Setenv("MOESIF_KEY", "env-app-id")
	defer os.Unsetenv("MOESIF_KEY")

	pubCfg := &config.MoesifPublisherConfig{
		ApplicationID:      "config-app-id",
		BaseURL:            "http://test.moesif.com",
		PublishInterval:    1,
		EventQueueSize:     100,
		BatchSize:          10,
		TimerWakeupSeconds: 1,
	}

	result := NewMoesif(pubCfg)
	require.NotNil(t, result, "NewMoesif should return a valid publisher")
	t.Cleanup(func() {
		_ = result.Close(context.Background())
	})
}

func TestPublish_BasicEvent(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	moesif.Publish(event)

	assert.Len(t, moesif.events, 1, "Should have one event queued")
	assert.Equal(t, "/resource", moesif.events[0].Request.Uri)
	assert.Equal(t, "GET", moesif.events[0].Request.Verb)
	assert.Equal(t, 200, moesif.events[0].Response.Status)
}

func TestPublish_WithRequestHeaders(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"Content-Type":"application/json","X-Custom":"value"}`

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	headers := moesif.events[0].Request.Headers.(map[string]interface{})
	assert.Equal(t, "application/json", headers["Content-Type"])
	assert.Equal(t, "value", headers["X-Custom"])
}

func TestPublish_WithInvalidRequestHeaders(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Properties["requestHeaders"] = `invalid json`

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	// Invalid JSON: no headers configured -> no headers published (no defaults).
	headers := moesif.events[0].Request.Headers.(map[string]interface{})
	assert.Empty(t, headers)
}

func TestPublish_WithEmptyRequestHeaders(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{}`

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	// Empty JSON object -> no headers published (no defaults).
	headers := moesif.events[0].Request.Headers.(map[string]interface{})
	assert.Empty(t, headers)
}

func TestPublish_WithResponseHeaders(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Properties["responseHeaders"] = `{"Content-Type":"text/html","X-Response":"value"}`

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	headers := moesif.events[0].Response.Headers.(map[string]interface{})
	assert.Equal(t, "text/html", headers["Content-Type"])
	assert.Equal(t, "value", headers["X-Response"])
}

func TestPublish_WithInvalidResponseHeaders(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Properties["responseHeaders"] = `not valid json`

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	// Invalid JSON: no headers configured -> no headers published (no defaults).
	headers := moesif.events[0].Response.Headers.(map[string]interface{})
	assert.Empty(t, headers)
}

func TestPublish_NoHeadersConfigured(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	// No requestHeaders/responseHeaders in event properties (analytics-header-filter
	// policy not configured) -> both header sets must be empty.

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	reqHeaders := moesif.events[0].Request.Headers.(map[string]interface{})
	rspHeaders := moesif.events[0].Response.Headers.(map[string]interface{})
	assert.Empty(t, reqHeaders)
	assert.Empty(t, rspHeaders)
}

func TestPublish_LlmProviderWithAIMetadata(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.API.APIType = "LlmProvider"
	event.Properties["aiMetadata"] = dto.AIMetadata{
		Model:      "gpt-4",
		VendorName: "openai",
	}
	event.Properties["aiTokenUsage"] = dto.AITokenUsage{
		PromptToken:     100,
		CompletionToken: 50,
		TotalToken:      150,
	}

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	metadata := getMetadata(moesif.events[0])
	assert.NotNil(t, metadata["aiMetadata"])
	assert.NotNil(t, metadata["aiTokenUsage"])

	aiMeta := metadata["aiMetadata"].(dto.AIMetadata)
	assert.Equal(t, "gpt-4", aiMeta.Model)
	assert.Equal(t, "openai", aiMeta.VendorName)

	tokenUsage := metadata["aiTokenUsage"].(dto.AITokenUsage)
	assert.Equal(t, 100, tokenUsage.PromptToken)
	assert.Equal(t, 50, tokenUsage.CompletionToken)
}

func TestPublish_LlmProxyWithAIMetadata(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.API.APIType = "LlmProxy"
	event.Properties["aiMetadata"] = dto.AIMetadata{
		Model:      "gpt-4",
		VendorName: "openai",
	}
	event.Properties["aiTokenUsage"] = dto.AITokenUsage{
		PromptToken:     100,
		CompletionToken: 50,
		TotalToken:      150,
	}

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	metadata := getMetadata(moesif.events[0])
	assert.NotNil(t, metadata["aiMetadata"])
	assert.NotNil(t, metadata["aiTokenUsage"])

	aiMeta := metadata["aiMetadata"].(dto.AIMetadata)
	assert.Equal(t, "gpt-4", aiMeta.Model)
	assert.Equal(t, "openai", aiMeta.VendorName)

	tokenUsage := metadata["aiTokenUsage"].(dto.AITokenUsage)
	assert.Equal(t, 150, tokenUsage.TotalToken)
}

func TestPublish_LlmProviderMissingAIMetadata(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.API.APIType = "LlmProvider"
	// No aiMetadata or aiTokenUsage in properties

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	metadata := getMetadata(moesif.events[0])
	assert.Nil(t, metadata["aiMetadata"])
	assert.Nil(t, metadata["aiTokenUsage"])
}

func TestPublish_LlmProviderWrongTypeAIMetadata(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.API.APIType = "LlmProvider"
	event.Properties["aiMetadata"] = "wrong type"
	event.Properties["aiTokenUsage"] = map[string]int{"wrong": 123}

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	metadata := getMetadata(moesif.events[0])
	// Should not be present since type assertion fails
	assert.Nil(t, metadata["aiMetadata"])
	assert.Nil(t, metadata["aiTokenUsage"])
}

func TestPublish_McpAPIType(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.API.APIType = "Mcp"
	event.Properties["mcpAnalytics"] = map[string]interface{}{
		"toolName": "search",
		"duration": 150,
	}

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	metadata := getMetadata(moesif.events[0])
	assert.NotNil(t, metadata["mcpAnalytics"])
	mcpAnalytics := metadata["mcpAnalytics"].(map[string]interface{})
	assert.Equal(t, "search", mcpAnalytics["toolName"])
}

// Total latency has to reach the publisher, not just the event: the other four
// latencies are all partial spans, so a percentile computed from them measures a phase
// rather than what the caller waited.
func TestPublish_ForwardsTotalDuration(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Latencies = &dto.Latencies{
		Duration:                 250,
		BackendLatency:           100,
		ResponseLatency:          100,
		RequestMediationLatency:  50,
		ResponseMediationLatency: 50,
	}

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	metadata := getMetadata(moesif.events[0])
	assert.Equal(t, int64(250), metadata["duration"],
		"total request duration must be forwarded, not only its component spans")
	assert.Equal(t, int64(100), metadata["backendLatency"])
}

// The published Agent contract, asserted at its actual boundary: what a downstream
// consumer reads is the serialized metadata, so the whole envelope is checked as JSON
// rather than field by field through Go accessors.
func TestPublish_AgentAnalyticsEnvelopeNesting(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	partCount := 2
	returnImmediately, isError := false, false
	event := createBaseEvent()
	event.API.APIType = "Agent"
	event.Properties[agentAnalyticsProperty] = &dto.AgentAnalytics{
		A2A: &dto.A2AAnalytics{
			RequestType:     "operation",
			Operation:       "SendMessage",
			Transport:       "JSONRPC",
			ProtocolVersion: "1.0",
			Request: &dto.A2ARequestAnalytics{
				MessageID:         "msg-1",
				InputPartCount:    &partCount,
				ReturnImmediately: &returnImmediately,
			},
			Response: &dto.A2AResponseAnalytics{
				IsError:     &isError,
				PayloadType: "task",
				TaskID:      "task-9",
				ContextID:   "ctx-9",
				TaskState:   "TASK_STATE_COMPLETED",
			},
			Outcome: "SUCCESS",
		},
	}

	moesif.Publish(event)

	require.Len(t, moesif.events, 1)
	metadata := getMetadata(moesif.events[0])
	require.NotNil(t, metadata[agentAnalyticsProperty])

	encoded, err := json.Marshal(metadata[agentAnalyticsProperty])
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"a2a": {
			"requestType": "operation",
			"operation": "SendMessage",
			"transport": "JSONRPC",
			"protocolVersion": "1.0",
			"request": {"messageId": "msg-1", "inputPartCount": 2, "returnImmediately": false},
			"response": {
				"isError": false, "payloadType": "task",
				"taskId": "task-9", "contextId": "ctx-9", "taskState": "TASK_STATE_COMPLETED"
			},
			"outcome": "SUCCESS"
		}
	}`, string(encoded))
}

// The flat a2aAnalytics key this envelope replaced must not be published alongside it.
// A consumer that found both would be reading two shapes of the same event depending
// on which gateway version produced it, and whichever it picked would go stale
// silently when the other stopped being written.
func TestPublish_LegacyFlatA2AAnalyticsKeyIsNotPublished(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.API.APIType = "Agent"
	event.Properties[agentAnalyticsProperty] = &dto.AgentAnalytics{
		A2A: &dto.A2AAnalytics{RequestType: "operation", Operation: "SendMessage"},
	}

	moesif.Publish(event)

	require.Len(t, moesif.events, 1)
	metadata := getMetadata(moesif.events[0])
	assert.NotContains(t, metadata, "a2aAnalytics",
		"the legacy flat key must not be published, not even alongside the envelope")
	assert.Contains(t, metadata, agentAnalyticsProperty)
}

// A card fetch carries only requestType, so it stays distinguishable from an
// invocation all the way out to the consumer rather than only inside the gateway.
func TestPublish_AgentCardEventCarriesOnlyRequestType(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.API.APIType = "Agent"
	event.Properties[agentAnalyticsProperty] = &dto.AgentAnalytics{
		A2A: &dto.A2AAnalytics{RequestType: "agentCard"},
	}

	moesif.Publish(event)

	require.Len(t, moesif.events, 1)
	encoded, err := json.Marshal(getMetadata(moesif.events[0])[agentAnalyticsProperty])
	require.NoError(t, err)
	assert.JSONEq(t, `{"a2a": {"requestType": "agentCard"}}`, string(encoded))
}

// The block is gated on the API kind, the way the MCP block is: an Agent's dimensions
// are meaningless on any other kind, and forwarding the key unconditionally would put
// an empty object on every event the gateway publishes.
func TestPublish_AgentAnalyticsNotForwardedForOtherKinds(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.API.APIType = "RestApi"
	event.Properties[agentAnalyticsProperty] = &dto.AgentAnalytics{
		A2A: &dto.A2AAnalytics{Operation: "SendMessage"},
	}

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	assert.Nil(t, getMetadata(moesif.events[0])[agentAnalyticsProperty])
}

func TestPublish_WithPayloads(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Properties["request_payload"] = `{"query": "test"}`
	event.Properties["response_payload"] = `{"result": "success"}`

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	metadata := getMetadata(moesif.events[0])
	assert.Equal(t, `{"query": "test"}`, metadata["request_payload"])
	assert.Equal(t, `{"result": "success"}`, metadata["response_payload"])
}

func TestPublish_WithLLMCost(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Properties[constants.LLMCostPropertyKey] = 0.00004231

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	metadata := getMetadata(moesif.events[0])
	assert.Equal(t, 0.00004231, metadata[constants.LLMCostPropertyKey])
}

func TestPublish_WithGuardrailMetadata(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Properties[constants.GuardrailHitMetadataKey] = true
	event.Properties[constants.GuardrailNameMetadataKey] = "word-count-guardrail"

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	metadata := getMetadata(moesif.events[0])
	assert.Equal(t, true, metadata[constants.GuardrailHitMetadataKey])
	assert.Equal(t, "word-count-guardrail", metadata[constants.GuardrailNameMetadataKey])
}

func TestPublish_WithApplicationMetadata(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Application = &dto.Application{
		ApplicationID:   "app-123",
		ApplicationName: "gold-plan-app",
	}

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	metadata := getMetadata(moesif.events[0])
	assert.Equal(t, "app-123", metadata["applicationId"])
	assert.Equal(t, "gold-plan-app", metadata["applicationName"])
}

func TestPublish_WithUserID(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Properties["x-wso2-user-id"] = "user-123"

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	assert.Equal(t, "user-123", *moesif.events[0].UserId)
}

func TestPublish_WithEmptyUserID(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Properties["x-wso2-user-id"] = ""

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	assert.Equal(t, "anonymous", *moesif.events[0].UserId)
}

func TestPublish_WithoutLatencies(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Latencies = nil

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	// Response time should equal request timestamp when no latencies
	assert.Equal(t, event.RequestTimestamp, *moesif.events[0].Response.Time)
}

func TestPublish_WithEmptyResourceTemplate(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Operation.APIResourceTemplate = ""

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	// URI should be constructed from context + template
	assert.Equal(t, "/test", moesif.events[0].Request.Uri)
}

func TestPublish_RequestHeadersNonString(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	// Set requestHeaders to a non-string value
	event.Properties["requestHeaders"] = 12345

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	// Non-string value -> no headers configured -> no headers published (no defaults).
	headers := moesif.events[0].Request.Headers.(map[string]interface{})
	assert.Empty(t, headers)
}

func TestPublish_ResponseHeadersNonString(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	// Set responseHeaders to a non-string value
	event.Properties["responseHeaders"] = []int{1, 2, 3}

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	// Non-string value -> no headers configured -> no headers published (no defaults).
	headers := moesif.events[0].Response.Headers.(map[string]interface{})
	assert.Empty(t, headers)
}

func TestPublish_MetadataContainsAPIInfo(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	metadata := getMetadata(moesif.events[0])
	assert.Equal(t, "corr-123", metadata["correlationId"])
	assert.Equal(t, "/test", metadata["apiContext"])
	assert.Equal(t, "test-api", metadata["apiName"])
	assert.Equal(t, "v1.0", metadata["apiVersion"])
	assert.Equal(t, "Rest", metadata["apiType"])
	assert.Equal(t, "Rest", metadata["subType"])
	assert.Equal(t, "api-123", metadata["apiId"])
	assert.Equal(t, "project-123", metadata["projectId"])
}

// Test that the subType in metadata mirrors the APIType for various API types.
func TestPublish_SubTypeMirrorsAPIType(t *testing.T) {
	for _, apiType := range []string{"Rest", "LlmProvider", "LlmProxy", "Mcp"} {
		t.Run(apiType, func(t *testing.T) {
			moesif := createTestMoesifWithoutAPI()

			event := createBaseEvent()
			// Production populates APIType and SubType from the same metadata source
			// (see analytics.go); mirror that here so the fixture matches the real event.
			event.API.APIType = apiType
			event.API.SubType = apiType
			moesif.Publish(event)

			assert.Len(t, moesif.events, 1)
			metadata := getMetadata(moesif.events[0])
			assert.Equal(t, apiType, metadata["subType"])
			assert.Equal(t, metadata["apiType"], metadata["subType"])
		})
	}
}

// fakeMoesifAPI records what Close hands to the client. Only the two methods
// Close exercises do anything; the rest satisfy the interface.
type fakeMoesifAPI struct {
	mu      sync.Mutex
	queued  [][]*models.EventModel
	flushes int
}

func (f *fakeMoesifAPI) QueueEvents(e []*models.EventModel) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queued = append(f.queued, e)
	return nil
}
func (f *fakeMoesifAPI) Flush() { f.mu.Lock(); f.flushes++; f.mu.Unlock() }
func (f *fakeMoesifAPI) snapshot() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, b := range f.queued {
		n += len(b)
	}
	return n, f.flushes
}

func (f *fakeMoesifAPI) QueueEvent(*models.EventModel) error                  { return nil }
func (f *fakeMoesifAPI) QueueUser(*models.UserModel) error                    { return nil }
func (f *fakeMoesifAPI) QueueUsers([]*models.UserModel) error                 { return nil }
func (f *fakeMoesifAPI) QueueCompany(*models.CompanyModel) error              { return nil }
func (f *fakeMoesifAPI) QueueCompanies([]*models.CompanyModel) error          { return nil }
func (f *fakeMoesifAPI) QueueSubscription(*models.SubscriptionModel) error    { return nil }
func (f *fakeMoesifAPI) QueueSubscriptions([]*models.SubscriptionModel) error { return nil }
func (f *fakeMoesifAPI) CreateEvent(*models.EventModel) (http.Header, error)  { return nil, nil }
func (f *fakeMoesifAPI) CreateEventsBatch([]*models.EventModel) (http.Header, error) {
	return nil, nil
}
func (f *fakeMoesifAPI) UpdateUser(*models.UserModel) error                         { return nil }
func (f *fakeMoesifAPI) UpdateUsersBatch([]*models.UserModel) error                 { return nil }
func (f *fakeMoesifAPI) GetAppConfig() (*http.Response, error)                      { return nil, nil }
func (f *fakeMoesifAPI) UpdateCompany(*models.CompanyModel) error                   { return nil }
func (f *fakeMoesifAPI) UpdateCompaniesBatch([]*models.CompanyModel) error          { return nil }
func (f *fakeMoesifAPI) UpdateSubscription(*models.SubscriptionModel) error         { return nil }
func (f *fakeMoesifAPI) UpdateSubscriptionsBatch([]*models.SubscriptionModel) error { return nil }
func (f *fakeMoesifAPI) GetGovernanceRules() (moesifapi.GovernanceRulesResponse, error) {
	return moesifapi.GovernanceRulesResponse{}, nil
}
func (f *fakeMoesifAPI) SetEventsHeaderCallback(string, func(string)) {}
func (f *fakeMoesifAPI) Close()                                       {}

// TestMoesif_CloseFlushesBufferedEvents pins that shutdown does not abandon the
// events accumulated between publish ticks. Close previously only stopped the
// ticker goroutine, so up to one full publish_interval of events was silently
// lost on every restart, rolling update and scale-down.
func TestMoesif_CloseFlushesBufferedEvents(t *testing.T) {
	fake := &fakeMoesifAPI{}
	m := &Moesif{
		api:    fake,
		done:   make(chan struct{}),
		events: []*models.EventModel{{}, {}, {}},
	}

	require.NoError(t, m.Close(context.Background()))

	queued, flushes := fake.snapshot()
	assert.Equal(t, 3, queued, "buffered events must be handed to the client on shutdown")
	assert.Equal(t, 1, flushes, "the client queue must be flushed, not left to its own timer")

	// Idempotent: a second Close must not re-queue the same events.
	require.NoError(t, m.Close(context.Background()))
	queued2, _ := fake.snapshot()
	assert.Equal(t, 3, queued2, "Close must be idempotent and never double-publish")
}

// slowMoesifAPI blocks in Flush until released, standing in for an unreachable
// Moesif endpoint. moesifapi-go's Flush is synchronous with no context, so this
// is the only way the sink can be held.
type slowMoesifAPI struct {
	fakeMoesifAPI
	release chan struct{}
}

func (s *slowMoesifAPI) Flush() {
	<-s.release
	s.fakeMoesifAPI.Flush()
}

// TestMoesif_CloseHonoursShutdownDeadline pins CodeRabbit #2: a hung Moesif
// endpoint must not hold shutdown open past the operator's budget. Doing so
// would also cost the traffic-log sinks their own flush, since Analytics.Close
// runs them in sequence.
func TestMoesif_CloseHonoursShutdownDeadline(t *testing.T) {
	api := &slowMoesifAPI{release: make(chan struct{})}
	t.Cleanup(func() { close(api.release) }) // let the goroutine finish after the test
	m := &Moesif{api: api, done: make(chan struct{}), events: []*models.EventModel{{}}}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := m.Close(ctx)
	elapsed := time.Since(start)

	require.Error(t, err, "an unfinished flush must be reported, not swallowed")
	assert.Contains(t, err.Error(), "shutdown budget")
	assert.Less(t, elapsed, time.Second, "Close must return on the deadline, not block on Flush")

	// The error is remembered, so a second Close reports the same outcome rather
	// than a misleading nil.
	assert.Equal(t, err, m.Close(context.Background()))
}

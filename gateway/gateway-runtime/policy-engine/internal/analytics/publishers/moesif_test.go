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
	"os"
	"sync"
	"testing"
	"time"

	"github.com/moesif/moesifapi-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// createTestMoesifWithoutAPI creates a Moesif publisher without a real API for testing Publish method.
// The api field is nil, but Publish doesn't call api methods - it only queues events.
func createTestMoesifWithoutAPI() *Moesif {
	return &Moesif{
		cfg:    &config.PublisherConfig{Type: "moesif", Enabled: true},
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
	}
}

func TestNewMoesif_ConfigDecodeError(t *testing.T) {
	// Settings with incompatible types should cause decode error
	pubCfg := &config.PublisherConfig{
		Type:    "moesif",
		Enabled: true,
		Settings: map[string]interface{}{
			// publish_interval expects int, passing a struct should cause an error
			"publish_interval": struct{ invalid string }{"not-a-number"},
		},
	}

	result := NewMoesif(pubCfg)
	assert.Nil(t, result, "NewMoesif should return nil when config decode fails")
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

	pubCfg := &config.PublisherConfig{
		Type:    "moesif",
		Enabled: true,
		Settings: map[string]interface{}{
			"application_id":      "test-app-id",
			"publish_interval":    1,
			"event_queue_size":    100,
			"batch_size":          10,
			"timer_wakeup_seconds": 1,
			// No moesif_base_url - should use default
		},
	}

	result := NewMoesif(pubCfg)
	require.NotNil(t, result, "NewMoesif should return a valid publisher")
	t.Cleanup(func() {
		result.Close()
	})
}

func TestNewMoesif_EnvVarOverridesConfig(t *testing.T) {
	// Set MOESIF_KEY env var
	os.Setenv("MOESIF_KEY", "env-app-id")
	defer os.Unsetenv("MOESIF_KEY")

	pubCfg := &config.PublisherConfig{
		Type:    "moesif",
		Enabled: true,
		Settings: map[string]interface{}{
			"application_id":      "config-app-id",
			"moesif_base_url":     "http://test.moesif.com",
			"publish_interval":    1,
			"event_queue_size":    100,
			"batch_size":          10,
			"timer_wakeup_seconds": 1,
		},
	}

	result := NewMoesif(pubCfg)
	require.NotNil(t, result, "NewMoesif should return a valid publisher")
	t.Cleanup(func() {
		result.Close()
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
	// Should fall back to default headers
	headers := moesif.events[0].Request.Headers.(map[string]interface{})
	assert.Equal(t, "test-agent", headers["User-Agent"])
}

func TestPublish_WithEmptyRequestHeaders(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{}`

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	// Empty JSON object should fall back to default headers
	headers := moesif.events[0].Request.Headers.(map[string]interface{})
	assert.Equal(t, "test-agent", headers["User-Agent"])
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
	// Should fall back to default headers
	headers := moesif.events[0].Response.Headers.(map[string]interface{})
	assert.Equal(t, "no-cache", headers["Cache-Control"])
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
	// Should use default headers
	headers := moesif.events[0].Request.Headers.(map[string]interface{})
	assert.Equal(t, "test-agent", headers["User-Agent"])
}

func TestPublish_ResponseHeadersNonString(t *testing.T) {
	moesif := createTestMoesifWithoutAPI()

	event := createBaseEvent()
	// Set responseHeaders to a non-string value
	event.Properties["responseHeaders"] = []int{1, 2, 3}

	moesif.Publish(event)

	assert.Len(t, moesif.events, 1)
	// Should use default headers
	headers := moesif.events[0].Response.Headers.(map[string]interface{})
	assert.Equal(t, "no-cache", headers["Cache-Control"])
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
	assert.Equal(t, "api-123", metadata["apiId"])
	assert.Equal(t, "project-123", metadata["projectId"])
}

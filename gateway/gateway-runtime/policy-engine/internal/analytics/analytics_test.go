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

package analytics

import (
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// mockPublisher is a test publisher that records whether Publish was called
type mockPublisher struct {
	called bool
	event  *dto.Event
}

func (m *mockPublisher) Publish(event *dto.Event) {
	m.called = true
	m.event = event
}

// =============================================================================
// NewAnalytics Tests
// =============================================================================

func TestNewAnalytics_Disabled(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled: false,
		},
	}

	analytics := NewAnalytics(cfg)

	require.NotNil(t, analytics)
	assert.Empty(t, analytics.publishers)
}

func TestNewAnalytics_EnabledNoPublishers(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled:    true,
			Publishers: []config.PublisherConfig{},
		},
	}

	analytics := NewAnalytics(cfg)

	require.NotNil(t, analytics)
	assert.Empty(t, analytics.publishers)
}

func TestNewAnalytics_EnabledWithDisabledPublisher(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled: true,
			Publishers: []config.PublisherConfig{
				{
					Enabled: false,
					Type:    "moesif",
				},
			},
		},
	}

	analytics := NewAnalytics(cfg)

	require.NotNil(t, analytics)
	assert.Empty(t, analytics.publishers)
}

func TestNewAnalytics_EnabledWithUnknownPublisherType(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled: true,
			Publishers: []config.PublisherConfig{
				{
					Enabled: true,
					Type:    "unknown-type",
				},
			},
		},
	}

	analytics := NewAnalytics(cfg)

	require.NotNil(t, analytics)
	assert.Empty(t, analytics.publishers) // Unknown type should not be added
}

// =============================================================================
// isInvalid Tests
// =============================================================================

func TestIsInvalid_NilResponse(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	logEntry := &v3.HTTPAccessLogEntry{
		Response: nil,
	}

	assert.True(t, analytics.isInvalid(logEntry))
}

func TestIsInvalid_ValidResponse(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	logEntry := &v3.HTTPAccessLogEntry{
		Response: &v3.HTTPResponseProperties{
			ResponseCode: wrapperspb.UInt32(200),
		},
	}

	assert.False(t, analytics.isInvalid(logEntry))
}

// =============================================================================
// GetFaultType Tests
// =============================================================================

func TestGetFaultType(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	faultType := analytics.GetFaultType()

	assert.Equal(t, FaultCategoryOther, faultType)
}

// =============================================================================
// getAnonymousApp Tests
// =============================================================================

func TestGetAnonymousApp(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	app := analytics.getAnonymousApp()

	require.NotNil(t, app)
	assert.Equal(t, anonymousValue, app.ApplicationID)
	assert.Equal(t, anonymousValue, app.ApplicationName)
	assert.Equal(t, anonymousValue, app.KeyType)
	assert.Equal(t, anonymousValue, app.ApplicationOwner)
}

// =============================================================================
// Process Tests
// =============================================================================

func TestProcess_NilResponse(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	logEntry := &v3.HTTPAccessLogEntry{
		Response: nil,
	}

	// Should not panic, should handle gracefully
	analytics.Process(logEntry)
}

func TestProcess_NoPublishers(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			Enabled: false,
		},
	}
	analytics := NewAnalytics(cfg)

	logEntry := &v3.HTTPAccessLogEntry{
		Response: &v3.HTTPResponseProperties{
			ResponseCode: wrapperspb.UInt32(200),
		},
		Request: &v3.HTTPRequestProperties{
			RequestMethod: corev3.RequestMethod_GET,
		},
	}

	// Should not panic
	analytics.Process(logEntry)
}

func TestProcess_WithMockPublisher(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	// Inject a mock publisher
	mockPub := &mockPublisher{}
	analytics.publishers = append(analytics.publishers, mockPub)

	logEntry := createLogEntryWithMetadata(map[string]string{
		APINameKey: "TestAPI",
	})

	analytics.Process(logEntry)

	// Verify publisher was called
	assert.True(t, mockPub.called)
	require.NotNil(t, mockPub.event)
	assert.Equal(t, "TestAPI", mockPub.event.API.APIName)
}

func TestProcess_PanicRecovery(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	// Inject a panicking publisher
	panicPub := &panicPublisher{}
	analytics.publishers = append(analytics.publishers, panicPub)

	logEntry := createLogEntryWithMetadata(map[string]string{
		APINameKey: "TestAPI",
	})

	// Should not panic - the defer/recover should catch it
	assert.NotPanics(t, func() {
		analytics.Process(logEntry)
	})
}

// panicPublisher is a test publisher that panics when Publish is called
type panicPublisher struct{}

func (p *panicPublisher) Publish(event *dto.Event) {
	panic("simulated panic in publisher")
}

// =============================================================================
// Constants Tests
// =============================================================================

func TestConstants(t *testing.T) {
	// Test that constants are defined correctly
	assert.Equal(t, "SUCCESS", string(EventCategorySuccess))
	assert.Equal(t, "FAULT", string(EventCategoryFault))
	assert.Equal(t, "INVALID", string(EventCategoryInvalid))
	assert.Equal(t, "TARGET_CONNECTIVITY", string(FaultCategoryTargetConnectivity))
	assert.Equal(t, "OTHER", string(FaultCategoryOther))
	assert.Equal(t, "default", DefaultAnalyticsPublisher)
	assert.Equal(t, "moesif", MoesifAnalyticsPublisher)
}

func TestMetadataKeys(t *testing.T) {
	// Test metadata key constants
	assert.Equal(t, "x-wso2-api-id", APIIDKey)
	assert.Equal(t, "x-wso2-api-name", APINameKey)
	assert.Equal(t, "x-wso2-api-version", APIVersionKey)
	assert.Equal(t, "x-wso2-application-id", AppIDKey)
	assert.Equal(t, "x-wso2-application-name", AppNameKey)
	assert.Equal(t, "x-wso2-correlation-id", CorrelationIDKey)
	assert.Equal(t, "UNKNOWN", Unknown)
}

func TestAIMetadataKeys(t *testing.T) {
	// Test AI-related metadata key constants
	assert.Equal(t, "aitoken:prompttokencount", PromptTokenCountMetadataKey)
	assert.Equal(t, "aitoken:completiontokencount", CompletionTokenCountMetadataKey)
	assert.Equal(t, "aitoken:totaltokencount", TotalTokenCountMetadataKey)
	assert.Equal(t, "aitoken:modelid", ModelIDMetadataKey)
	assert.Equal(t, "ai:providername", AIProviderNameMetadataKey)
	assert.Equal(t, "ai:providerversion", AIProviderAPIVersionMetadataKey)
}

func TestResponseDetailConstants(t *testing.T) {
	assert.Equal(t, "via_upstream", UpstreamSuccessResponseDetail)
	assert.Equal(t, "ext_authz_denied", ExtAuthDeniedResponseDetail)
	assert.Equal(t, "ext_authz_error", ExtAuthErrorResponseDetail)
	assert.Equal(t, "route_not_found", RouteNotFoundResponseDetail)
}

func TestErrorCodeConstants(t *testing.T) {
	assert.Equal(t, 900800, APIThrottleOutErrorCode)
	assert.Equal(t, 900801, HardLimitExceededErrorCode)
	assert.Equal(t, 900802, ResourceThrottleOutErrorCode)
	assert.Equal(t, 900803, ApplicationThrottleOutErrorCode)
	assert.Equal(t, 900804, SubscriptionThrottleOutErrorCode)
	assert.Equal(t, 900805, BlockedErrorCode)
	assert.Equal(t, 900806, CustomPolicyThrottleOutErrorCode)
}

func TestHeaderKeyConstants(t *testing.T) {
	assert.Equal(t, "request_headers", RequestHeadersKey)
	assert.Equal(t, "response_headers", ResponseHeadersKey)
}

func TestRFC3339MillisFormat(t *testing.T) {
	assert.Equal(t, "2006-01-02T15:04:05.000Z07:00", RFC3339Millis)
}

// =============================================================================
// prepareAnalyticEvent Tests
// =============================================================================

func TestPrepareAnalyticEvent_WithFilterMetadata(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	// Create log entry with filter metadata containing analytics_data
	logEntry := createLogEntryWithMetadata(map[string]string{
		APIIDKey:      "api-123",
		APINameKey:    "TestAPI",
		APIVersionKey: "v1.0.0",
		APIContextKey: "/test",
		AppIDKey:      "app-456",
		AppNameKey:    "TestApp",
		AppOwnerKey:   "owner",
		AppKeyTypeKey: "PRODUCTION",
	})

	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	assert.Equal(t, "api-123", event.API.APIID)
	assert.Equal(t, "TestAPI", event.API.APIName)
	assert.Equal(t, "v1.0.0", event.API.APIVersion)
	assert.Equal(t, "/test", event.API.APIContext)
	assert.Equal(t, "app-456", event.Application.ApplicationID)
	assert.Equal(t, "TestApp", event.Application.ApplicationName)
}

func TestPrepareAnalyticEvent_WithAnonymousApp(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	// Create log entry with UNKNOWN app ID
	logEntry := createLogEntryWithMetadata(map[string]string{
		AppIDKey: Unknown,
	})

	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	assert.Equal(t, anonymousValue, event.Application.ApplicationID)
	assert.Equal(t, anonymousValue, event.Application.ApplicationName)
}

func TestPrepareAnalyticEvent_WithAIMetadata(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	// Create log entry with AI metadata
	logEntry := createLogEntryWithMetadata(map[string]string{
		AIProviderNameMetadataKey:       "openai",
		AIProviderAPIVersionMetadataKey: "v1",
		ModelIDMetadataKey:              "gpt-4",
		PromptTokenCountMetadataKey:     "100",
		CompletionTokenCountMetadataKey: "50",
		TotalTokenCountMetadataKey:      "150",
	})

	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	// Check AI metadata is in properties
	aiMetadata, ok := event.Properties["aiMetadata"]
	require.True(t, ok)
	require.NotNil(t, aiMetadata)

	// Check token usage
	aiTokenUsage, ok := event.Properties["aiTokenUsage"]
	require.True(t, ok)
	require.NotNil(t, aiTokenUsage)
}

func TestPrepareAnalyticEvent_WithAIMetadataInvalidTokens(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	// Create log entry with invalid token counts (non-numeric)
	logEntry := createLogEntryWithMetadata(map[string]string{
		AIProviderNameMetadataKey:       "openai",
		PromptTokenCountMetadataKey:     "invalid",
		CompletionTokenCountMetadataKey: "not-a-number",
		TotalTokenCountMetadataKey:      "abc",
	})

	// Should not panic, should handle gracefully
	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
}

func TestPrepareAnalyticEvent_WithLatencies(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	logEntry := createLogEntryWithLatencies()

	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	require.NotNil(t, event.Latencies)
	assert.True(t, event.Latencies.BackendLatency >= 0)
}

func TestPrepareAnalyticEvent_WithUserID(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	logEntry := createLogEntryWithMetadata(map[string]string{
		UserIDMetadataKey: "user-123",
	})

	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	userID, ok := event.Properties[UserIDMetadataKey]
	require.True(t, ok)
	assert.Equal(t, "user-123", userID)
}

func TestPrepareAnalyticEvent_WithRequestResponseHeaders(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	logEntry := createLogEntryWithMetadata(map[string]string{
		RequestHeadersKey:  `{"Content-Type": "application/json"}`,
		ResponseHeadersKey: `{"X-Custom": "value"}`,
	})

	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	reqHeaders, ok := event.Properties["requestHeaders"]
	require.True(t, ok)
	assert.Contains(t, reqHeaders, "Content-Type")
}

func TestPrepareAnalyticEvent_WithPayloadsEnabled(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			AllowPayloads: true,
		},
	}
	analytics := NewAnalytics(cfg)

	logEntry := createLogEntryWithMetadata(map[string]string{
		"request_payload":  `{"key": "value"}`,
		"response_payload": `{"result": "ok"}`,
	})

	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	reqPayload, ok := event.Properties["request_payload"]
	require.True(t, ok)
	assert.Equal(t, `{"key": "value"}`, reqPayload)

	respPayload, ok := event.Properties["response_payload"]
	require.True(t, ok)
	assert.Equal(t, `{"result": "ok"}`, respPayload)
}

func TestPrepareAnalyticEvent_WithPayloadsDisabled(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{
			AllowPayloads: false,
		},
	}
	analytics := NewAnalytics(cfg)

	logEntry := createLogEntryWithMetadata(map[string]string{
		"request_payload":  `{"key": "value"}`,
		"response_payload": `{"result": "ok"}`,
	})

	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	// Payloads should not be included when disabled
	_, ok := event.Properties["request_payload"]
	assert.False(t, ok)
}

func TestPrepareAnalyticEvent_WithMCPAnalytics(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	logEntry := createLogEntryWithMetadata(map[string]string{
		APITypeKey:               "Mcp",
		"mcp_session_id":         "session-123",
		"mcp_request_properties": `{"tool": "calculator", "action": "add"}`,
		"mcp_server_info":        `{"name": "math-server", "version": "1.0"}`,
	})

	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	mcpAnalytics, ok := event.Properties["mcpAnalytics"]
	require.True(t, ok)
	require.NotNil(t, mcpAnalytics)

	mcpMap, ok := mcpAnalytics.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "session-123", mcpMap["mcp_session_id"])
}

func TestPrepareAnalyticEvent_WithMCPAnalyticsInvalidJSON(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	// Create log entry with invalid JSON for MCP properties
	logEntry := createLogEntryWithMetadata(map[string]string{
		APITypeKey:               "Mcp",
		"mcp_session_id":         "session-123",
		"mcp_request_properties": `{invalid json`,
		"mcp_server_info":        `{also invalid`,
	})

	// Should not panic, should fallback to raw string
	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	mcpAnalytics, ok := event.Properties["mcpAnalytics"]
	require.True(t, ok)
	require.NotNil(t, mcpAnalytics)
}

func TestPrepareAnalyticEvent_WithEmptyUserName(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	logEntry := createLogEntryWithMetadata(map[string]string{
		APIUserNameKey: "",
	})

	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	// Empty username should be set to Unknown
	assert.Equal(t, Unknown, event.Properties["userName"])
}

func TestPrepareAnalyticEvent_WithResponseContentType(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	logEntry := createLogEntryWithResponseHeaders(map[string]string{
		"content-type": "application/json",
	})

	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	assert.Equal(t, "application/json", event.Properties["responseContentType"])
}

func TestPrepareAnalyticEvent_WithCorrelationID(t *testing.T) {
	cfg := &config.Config{}
	analytics := NewAnalytics(cfg)

	logEntry := createLogEntryWithStreamID("stream-correlation-123")

	event := analytics.prepareAnalyticEvent(logEntry)

	require.NotNil(t, event)
	assert.Equal(t, "stream-correlation-123", event.MetaInfo.CorrelationID)
}

// =============================================================================
// Helper Functions for Creating Test Log Entries
// =============================================================================

func createLogEntryWithMetadata(metadata map[string]string) *v3.HTTPAccessLogEntry {
	fields := make(map[string]*structpb.Value)
	for key, value := range metadata {
		fields[key] = structpb.NewStringValue(value)
	}

	return &v3.HTTPAccessLogEntry{
		CommonProperties: &v3.AccessLogCommon{
			Metadata: &corev3.Metadata{
				FilterMetadata: map[string]*structpb.Struct{
					constants.ExtProcFilterName: {
						Fields: map[string]*structpb.Value{
							"analytics_data": structpb.NewStructValue(&structpb.Struct{
								Fields: fields,
							}),
						},
					},
				},
			},
			DownstreamRemoteAddress: &corev3.Address{
				Address: &corev3.Address_SocketAddress{
					SocketAddress: &corev3.SocketAddress{
						Address: "192.168.1.1",
					},
				},
			},
		},
		Request: &v3.HTTPRequestProperties{
			RequestMethod: corev3.RequestMethod_GET,
			Authority:     "api.example.com",
			Path:          "/test/resource",
			UserAgent:     "test-agent",
		},
		Response: &v3.HTTPResponseProperties{
			ResponseCode:    wrapperspb.UInt32(200),
			ResponseHeaders: map[string]string{},
		},
	}
}

func createLogEntryWithLatencies() *v3.HTTPAccessLogEntry {
	return &v3.HTTPAccessLogEntry{
		CommonProperties: &v3.AccessLogCommon{
			TimeToFirstUpstreamTxByte: &durationpb.Duration{Seconds: 0, Nanos: 100000000},  // 100ms
			TimeToLastUpstreamRxByte:  &durationpb.Duration{Seconds: 0, Nanos: 200000000},  // 200ms
			TimeToLastDownstreamTxByte: &durationpb.Duration{Seconds: 0, Nanos: 250000000}, // 250ms
			DownstreamRemoteAddress: &corev3.Address{
				Address: &corev3.Address_SocketAddress{
					SocketAddress: &corev3.SocketAddress{
						Address: "192.168.1.1",
					},
				},
			},
		},
		Request: &v3.HTTPRequestProperties{
			RequestMethod: corev3.RequestMethod_GET,
		},
		Response: &v3.HTTPResponseProperties{
			ResponseCode: wrapperspb.UInt32(200),
		},
	}
}

func createLogEntryWithResponseHeaders(headers map[string]string) *v3.HTTPAccessLogEntry {
	return &v3.HTTPAccessLogEntry{
		CommonProperties: &v3.AccessLogCommon{
			DownstreamRemoteAddress: &corev3.Address{
				Address: &corev3.Address_SocketAddress{
					SocketAddress: &corev3.SocketAddress{
						Address: "192.168.1.1",
					},
				},
			},
		},
		Request: &v3.HTTPRequestProperties{
			RequestMethod: corev3.RequestMethod_GET,
		},
		Response: &v3.HTTPResponseProperties{
			ResponseCode:    wrapperspb.UInt32(200),
			ResponseHeaders: headers,
		},
	}
}

func createLogEntryWithStreamID(streamID string) *v3.HTTPAccessLogEntry {
	return &v3.HTTPAccessLogEntry{
		CommonProperties: &v3.AccessLogCommon{
			StreamId: streamID,
			DownstreamRemoteAddress: &corev3.Address{
				Address: &corev3.Address_SocketAddress{
					SocketAddress: &corev3.SocketAddress{
						Address: "192.168.1.1",
					},
				},
			},
		},
		Request: &v3.HTTPRequestProperties{
			RequestMethod: corev3.RequestMethod_GET,
			RequestId:     "fallback-request-id",
		},
		Response: &v3.HTTPResponseProperties{
			ResponseCode: wrapperspb.UInt32(200),
		},
	}
}

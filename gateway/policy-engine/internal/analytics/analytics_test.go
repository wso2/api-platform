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
	"testing"

	v3 "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/policy-engine/internal/config"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

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

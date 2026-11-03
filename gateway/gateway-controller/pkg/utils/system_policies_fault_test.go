/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// The collector is the one system policy that belongs on the fault path, and it must be
// present there whenever it is present on the normal path — an API whose failures are not
// recorded is the case this whole mechanism exists to prevent.
func TestFaultSystemPolicies_FollowsTheCollector(t *testing.T) {
	t.Run("nil config yields nothing", func(t *testing.T) {
		assert.Nil(t, FaultSystemPolicies(nil, nil),
			"a caller appends this unconditionally, so nil must be safe")
	})

	t.Run("collector disabled yields nothing", func(t *testing.T) {
		assert.Empty(t, FaultSystemPolicies(&config.Config{}, nil),
			"no collector means no fault-path collector either")
	})

	t.Run("analytics enabled yields the collector", func(t *testing.T) {
		cfg := &config.Config{Analytics: config.AnalyticsConfig{Enabled: true}}
		got := FaultSystemPolicies(cfg, nil)
		assert.Len(t, got, 1)
		assert.Equal(t, constants.ANALYTICS_SYSTEM_POLICY_NAME, got[0].Name)
		assert.Equal(t, constants.ANALYTICS_SYSTEM_POLICY_VERSION, got[0].Version)
		assert.True(t, got[0].Enabled)
	})

	// Traffic logging alone also activates the collector, so the fault path has to follow
	// the same predicate rather than testing Analytics.Enabled on its own.
	t.Run("traffic logging alone is enough", func(t *testing.T) {
		cfg := &config.Config{TrafficLogging: config.TrafficLoggingConfig{Enabled: true}}
		assert.Len(t, FaultSystemPolicies(cfg, nil), 1,
			"the fault path must use IsCollectorEnabled, not the analytics flag alone")
	})
}

// The capture flags must be resolved identically on both paths. A collector that captured
// payloads on the success path but not the failure path would be the kind of asymmetry
// nobody notices until they need the failing request.
func TestFaultSystemPolicies_ResolvesTheSameParametersAsTheNormalChain(t *testing.T) {
	cfg := &config.Config{
		Analytics: config.AnalyticsConfig{Enabled: true},
		Collector: config.CollectorConfig{
			RequestBody:     true,
			ResponseBody:    true,
			RequestHeaders:  true,
			ResponseHeaders: false,
		},
	}

	normal := InjectSystemPolicies(nil, cfg, nil)
	fault := FaultSystemPolicies(cfg, nil)

	assert.Len(t, normal, 1)
	assert.Len(t, fault, 1)
	assert.Equal(t, normal[0].Parameters, fault[0].Parameters,
		"one resolver, so the two placements cannot drift")
	assert.Equal(t, true, fault[0].Parameters["request_body"])
	assert.Equal(t, false, fault[0].Parameters["response_headers"])
}

// Policy-specific and shared additionalProps overrides have to reach the fault placement
// too, since they are how a deployment tunes the collector.
func TestFaultSystemPolicies_HonoursAdditionalProps(t *testing.T) {
	cfg := &config.Config{Analytics: config.AnalyticsConfig{Enabled: true}}
	props := map[string]any{
		constants.ANALYTICS_SYSTEM_POLICY_NAME: map[string]interface{}{"custom": "value"},
		SharedParamsKey:                        map[string]interface{}{"shared": "yes"},
	}

	got := FaultSystemPolicies(cfg, props)

	assert.Len(t, got, 1)
	assert.Equal(t, "value", got[0].Parameters["custom"])
	assert.Equal(t, "yes", got[0].Parameters["shared"])
}

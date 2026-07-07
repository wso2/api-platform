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

package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsEnabled(t *testing.T) {
	assert.False(t, IsEnabled(false, false))
	assert.True(t, IsEnabled(true, false))
	assert.True(t, IsEnabled(false, true))
	assert.True(t, IsEnabled(true, true))
}

func TestMigrateDeprecatedCapture_SkippedWhenAnalyticsDisabled(t *testing.T) {
	collectorReq, collectorResp := false, false
	MigrateDeprecatedCapture(false, CaptureFlags{SendRequestBody: true, SendResponseBody: true, AllowPayloads: true}, &collectorReq, &collectorResp)
	assert.False(t, collectorReq)
	assert.False(t, collectorResp)
}

func TestMigrateDeprecatedCapture_DirectionalFlags(t *testing.T) {
	collectorReq, collectorResp := false, false
	MigrateDeprecatedCapture(true, CaptureFlags{SendRequestBody: true, SendResponseBody: false}, &collectorReq, &collectorResp)
	assert.True(t, collectorReq)
	assert.False(t, collectorResp)
}

func TestMigrateDeprecatedCapture_AllowPayloadsFillsInWhenNoDirectionalFlags(t *testing.T) {
	collectorReq, collectorResp := false, false
	MigrateDeprecatedCapture(true, CaptureFlags{AllowPayloads: true}, &collectorReq, &collectorResp)
	assert.True(t, collectorReq)
	assert.True(t, collectorResp)
}

func TestMigrateDeprecatedCapture_DirectionalFlagsWinOverAllowPayloads(t *testing.T) {
	collectorReq, collectorResp := false, true
	MigrateDeprecatedCapture(true, CaptureFlags{SendRequestBody: true, AllowPayloads: true}, &collectorReq, &collectorResp)
	assert.True(t, collectorReq)
	assert.True(t, collectorResp, "already true; allow_payloads must not be re-applied since a directional flag is set")
}

type testTransportConfig struct {
	Port int
	Mode string
}

func TestMigrateDeprecatedTransport_SkippedWhenAnalyticsDisabled(t *testing.T) {
	def := testTransportConfig{Port: 18090, Mode: "uds"}
	collectorCfg := def
	deprecated := testTransportConfig{Port: 9999, Mode: "tcp"}

	MigrateDeprecatedTransport(false, deprecated, &collectorCfg, def, "analytics.grpc_event_server")

	assert.Equal(t, def, collectorCfg, "collector transport must stay at default when analytics is disabled")
}

func TestMigrateDeprecatedTransport_SkippedWhenDeprecatedAtDefault(t *testing.T) {
	def := testTransportConfig{Port: 18090, Mode: "uds"}
	collectorCfg := def

	MigrateDeprecatedTransport(true, def, &collectorCfg, def, "analytics.grpc_event_server")

	assert.Equal(t, def, collectorCfg)
}

func TestMigrateDeprecatedTransport_MigratesWhenCollectorAtDefault(t *testing.T) {
	def := testTransportConfig{Port: 18090, Mode: "uds"}
	collectorCfg := def
	deprecated := testTransportConfig{Port: 9999, Mode: "tcp"}

	MigrateDeprecatedTransport(true, deprecated, &collectorCfg, def, "analytics.grpc_event_server")

	assert.Equal(t, deprecated, collectorCfg)
}

func TestMigrateDeprecatedTransport_DoesNotClobberExplicitCollectorConfig(t *testing.T) {
	def := testTransportConfig{Port: 18090, Mode: "uds"}
	explicit := testTransportConfig{Port: 7000, Mode: "tcp"}
	collectorCfg := explicit
	deprecated := testTransportConfig{Port: 9999, Mode: "tcp"}

	MigrateDeprecatedTransport(true, deprecated, &collectorCfg, def, "analytics.grpc_event_server")

	assert.Equal(t, explicit, collectorCfg, "an explicit [collector.als] override must never be clobbered by the deprecated alias")
}

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

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The correlation store lives under [collector.correlation_store]: it serves every
// collector consumer (analytics and traffic logging), not analytics alone. These
// tests load real TOML so a mistyped koanf tag cannot silently fall back to defaults.

func TestLoad_CorrelationStore_EveryKeyBindsFromTOML(t *testing.T) {
	path := writeOTelTOML(t, `
[traffic_logging]
enabled = true

[collector.correlation_store]
mode = "off"
capacity = 50000
ttl = "45s"
shards = 64
max_payload_bytes = 131072
max_body_bytes = 67108864
max_entry_age = "10m"
`)
	cfg, err := Load(path)
	require.NoError(t, err)

	c := cfg.Collector.CorrelationStore
	assert.Equal(t, "off", c.Mode, "mode")
	assert.Equal(t, 50000, c.Capacity, "capacity")
	assert.Equal(t, 45*time.Second, c.TTL, "ttl")
	assert.Equal(t, 64, c.Shards, "shards")
	assert.Equal(t, 131072, c.MaxPayloadBytes, "max_payload_bytes")
	assert.Equal(t, int64(67108864), c.MaxBodyBytes, "max_body_bytes")
	assert.Equal(t, 10*time.Minute, c.MaxEntryAge, "max_entry_age")
}

func TestLoad_CorrelationStore_OmittedSectionKeepsDefaults(t *testing.T) {
	path := writeOTelTOML(t, `
[traffic_logging]
enabled = true
`)
	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, defaultCorrelationStoreConfig(), cfg.Collector.CorrelationStore)
}

func TestLoad_CorrelationStore_InvalidValueFailsWithNewKeyName(t *testing.T) {
	path := writeOTelTOML(t, `
[traffic_logging]
enabled = true

[collector.correlation_store]
capacity = 0
`)
	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector.correlation_store.capacity")
}

func TestValidate_CorrelationStoreBounds(t *testing.T) {
	cases := map[string]struct {
		toml string
		want string
	}{
		"too many shards":         {"shards = 2048\ncapacity = 50000", "collector.correlation_store.shards must not exceed 1024"},
		"capacity over the limit": {"capacity = 20000000", "collector.correlation_store.capacity must not exceed 10000000"},
		"capacity below shards":   {"capacity = 8\nshards = 64", "collector.correlation_store.capacity (8) must be at least shards (64)"},
		"unknown mode":            {`mode = "sometimes"`, `collector.correlation_store.mode must be "auto", "on" or "off", got "sometimes"`},
		"negative max_entry_age":  {`max_entry_age = "-1s"`, "collector.correlation_store.max_entry_age must not be negative"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeOTelTOML(t, "[traffic_logging]\nenabled = true\n\n[collector.correlation_store]\n"+tc.toml+"\n")
			_, err := Load(path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestLoad_CollectorIgnorePathPrefixesBindsFromTOML(t *testing.T) {
	path := writeOTelTOML(t, `
[traffic_logging]
enabled = true

[collector]
ignore_path_prefixes = ["/_gateway-health", "/metrics"]
`)
	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"/_gateway-health", "/metrics"}, cfg.Collector.IgnorePathPrefixes)
}

// A TTL not longer than Envoy's access-log flush interval never fails startup: while
// the store is in use it is raised to twice the flush interval; with the store off
// it is left alone. The flush interval also honors the deprecated
// [analytics.grpc_event_server] setting the gateway-controller still applies.
func TestCorrelationStoreTTLAgainstFlushInterval(t *testing.T) {
	cases := map[string]struct {
		toml string
		want time.Duration
	}{
		"default ttl, default flush": {"", 30 * time.Second},
		"ttl at the flush interval":  {"[collector.correlation_store]\nttl = \"1s\"\n", 2 * time.Second},
		"long flush interval": {
			"[collector.server]\nbuffer_flush_interval = 60000000000\n", 2 * time.Minute},
		"long flush, store off": {
			"[collector.server]\nbuffer_flush_interval = 60000000000\n\n[collector.correlation_store]\nmode = \"off\"\n", 30 * time.Second},
		"long flush, TCP (auto mode)": {
			"[collector.server]\nmode = \"tcp\"\nbuffer_flush_interval = 60000000000\n", 30 * time.Second},
		"deprecated flush setting": {
			"[analytics]\nenabled = true\n\n[analytics.publishers.moesif]\napplication_id = \"placeholder\"\n\n[analytics.grpc_event_server]\nbuffer_flush_interval = 45000000000\n", 90 * time.Second},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, err := Load(writeOTelTOML(t, "[traffic_logging]\nenabled = true\n\n"+tc.toml))
			require.NoError(t, err, "startup must not fail")
			assert.Equal(t, tc.want, cfg.Collector.CorrelationStore.TTL)
		})
	}
}

// The store needs the ext_proc and ALS streams of a request to reach this process,
// which only UDS guarantees; "auto" turns it off in TCP mode.
func TestCorrelationStoreEnabled(t *testing.T) {
	cases := []struct {
		name, mode, extProcMode, alsMode string
		collector                        bool
		want                             bool
	}{
		{"auto, both uds", "auto", "uds", "uds", true, true},
		{"auto, defaults", "", "", "", true, true},
		{"auto, ext_proc over tcp", "auto", "tcp", "uds", true, false},
		{"auto, ALS over tcp", "auto", "uds", "tcp", true, false},
		{"on, tcp", "on", "tcp", "tcp", true, true},
		{"off, uds", "off", "uds", "uds", true, false},
		{"collector disabled", "on", "uds", "uds", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.TrafficLogging.Enabled = tc.collector
			cfg.Analytics.Enabled = false
			cfg.Collector.CorrelationStore.Mode = tc.mode
			cfg.PolicyEngine.Server.Mode = tc.extProcMode
			cfg.Collector.Server.Mode = tc.alsMode
			assert.Equal(t, tc.want, cfg.CorrelationStoreEnabled())
		})
	}
}

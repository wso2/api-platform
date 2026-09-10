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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The struct-level validation tests build OTelPublisherConfig values in Go, which
// proves the rules but never exercises the koanf tags. A mistyped tag fails
// silently — the key is ignored and the default is used — so an operator sets
// on_queue_full or queue_capacity, sees no error, and gets default behaviour.
// These tests load real TOML through Load() and assert every field.

func writeOTelTOML(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

// Every OTel publisher key set to a non-default value, so a field that failed to
// bind shows up as its default rather than what the TOML asked for.
func TestLoad_OTelPublisher_EveryKeyBindsFromTOML(t *testing.T) {
	path := writeOTelTOML(t, `
[analytics]
enabled = true
enabled_publishers = ["otel"]

[analytics.publishers.otel]
endpoint = "https://collector.example.com:4318/v1/logs"
service_name = "plumbing-test"
service_version = "9.9.9"
batch_size = 7
flush_interval = "3s"
queue_capacity = 42
on_queue_full = "drop_oldest"
timeout = "11s"
max_retries = 5
retry_backoff = "250ms"
retry_abort_queue_ratio = 0.25
compression = "gzip"

[analytics.publishers.otel.headers]
"x-api-key" = "secret-value"

[analytics.publishers.otel.resource_attributes]
"deployment.environment" = "staging"
"service.namespace" = "gw"
`)
	cfg, err := Load(path)
	require.NoError(t, err)

	o := cfg.Analytics.Publishers.OTel
	assert.Equal(t, "https://collector.example.com:4318/v1/logs", o.Endpoint, "endpoint")
	assert.Equal(t, "plumbing-test", o.ServiceName, "service_name")
	assert.Equal(t, "9.9.9", o.ServiceVersion, "service_version")
	assert.Equal(t, 7, o.BatchSize, "batch_size")
	assert.Equal(t, 3*time.Second, o.FlushInterval, "flush_interval")
	assert.Equal(t, 42, o.QueueCapacity, "queue_capacity")
	assert.Equal(t, QueueDropOldest, o.OnQueueFull, "on_queue_full")
	assert.Equal(t, 11*time.Second, o.Timeout, "timeout")
	assert.Equal(t, 5, o.MaxRetries, "max_retries")
	assert.Equal(t, 250*time.Millisecond, o.RetryBackoff, "retry_backoff")
	assert.InDelta(t, 0.25, o.RetryAbortQueueRatio, 1e-9, "retry_abort_queue_ratio")
	assert.Equal(t, OTelCompressionGzip, o.Compression, "compression")
	assert.Equal(t, map[string]string{"x-api-key": "secret-value"}, o.Headers, "headers")
	assert.Equal(t, map[string]string{
		"deployment.environment": "staging",
		"service.namespace":      "gw",
	}, o.ResourceAttributes, "resource_attributes")

	// The abort depth an operator actually gets from these two keys together.
	assert.Equal(t, 10, o.EffectiveRetryAbortDepth(), "42 * 0.25 truncates to 10")
}

// The TLS sub-table is its own koanf level, so it binds separately from the
// publisher keys above.
func TestLoad_OTelPublisher_TLSBindsFromTOML(t *testing.T) {
	// writeSelfSignedPair (otel_publisher_test.go) emits a self-signed CA cert and
	// its key; the same cert serves as ca_file and as the mTLS client cert here,
	// which is all the validation needs — it checks the material parses, not that
	// it chains to anything.
	certPath, keyPath := writeSelfSignedPair(t)
	caPath := certPath

	path := writeOTelTOML(t, `
[analytics]
enabled = true
enabled_publishers = ["otel"]

[analytics.publishers.otel]
endpoint = "https://collector.example.com:4318/v1/logs"

[analytics.publishers.otel.tls]
ca_file = "`+caPath+`"
cert_file = "`+certPath+`"
key_file = "`+keyPath+`"
insecure_skip_verify = true
`)
	cfg, err := Load(path)
	require.NoError(t, err)

	tlsCfg := cfg.Analytics.Publishers.OTel.TLS
	assert.Equal(t, caPath, tlsCfg.CAFile, "ca_file")
	assert.Equal(t, certPath, tlsCfg.CertFile, "cert_file")
	assert.Equal(t, keyPath, tlsCfg.KeyFile, "key_file")
	assert.True(t, tlsCfg.InsecureSkipVerify, "insecure_skip_verify")
}

// Keys left out of the TOML must fall back to the shipped defaults rather than
// zero values, which would fail validation or silently disable batching.
func TestLoad_OTelPublisher_OmittedKeysKeepDefaults(t *testing.T) {
	path := writeOTelTOML(t, `
[analytics]
enabled = true
enabled_publishers = ["otel"]

[analytics.publishers.otel]
endpoint = "http://otel-collector:4318/v1/logs"
`)
	cfg, err := Load(path)
	require.NoError(t, err)

	o := cfg.Analytics.Publishers.OTel
	assert.Equal(t, "gateway-runtime", o.ServiceName, "default service_name")
	assert.Equal(t, 100, o.BatchSize, "default batch_size")
	assert.Equal(t, 5*time.Second, o.FlushInterval, "default flush_interval")
	assert.Equal(t, 10000, o.QueueCapacity, "default queue_capacity")
	assert.Equal(t, QueueDropNew, o.OnQueueFull, "default on_queue_full")
	assert.Equal(t, 10*time.Second, o.Timeout, "default timeout")
	assert.Equal(t, 3, o.MaxRetries, "default max_retries")
	assert.Equal(t, time.Second, o.RetryBackoff, "default retry_backoff")
	assert.InDelta(t, DefaultOTelRetryAbortQueueRatio, o.RetryAbortQueueRatio, 1e-9, "default abort ratio")
	assert.Equal(t, OTelCompressionNone, o.Compression, "default compression")
}

// A bad value in the TOML must fail the load, not be silently coerced. This is
// what makes the process refuse to start rather than run with defaults.
func TestLoad_OTelPublisher_InvalidTOMLValuesFailClosed(t *testing.T) {
	cases := []struct {
		name string
		body string
		// wantErr is a substring the rejection must contain, so a case cannot
		// pass on an unrelated error (a duplicate TOML key, say).
		wantErr string
	}{
		{"unknown drop policy", `on_queue_full = "sometimes"`, "on_queue_full must be"},
		{"queue smaller than batch", "queue_capacity = 5\nbatch_size = 50", "must be >= batch_size"},
		{"zero queue capacity", `queue_capacity = 0`, "queue_capacity must be > 0"},
		{"unknown compression", `compression = "snappy"`, "compression must be"},
		{"abort ratio above one", `retry_abort_queue_ratio = 1.5`, "retry_abort_queue_ratio must be between"},
		{"negative retries", `max_retries = -1`, "max_retries must be >= 0"},
		{"zero flush interval", `flush_interval = "0s"`, "flush_interval must be > 0"},
		{"zero timeout", `timeout = "0s"`, "timeout must be > 0"},
		{"retries without backoff", "max_retries = 2\nretry_backoff = \"0s\"", "retry_backoff must be positive"},
		{"non-http scheme", `endpoint = "ftp://collector:4318/v1/logs"`, "scheme must be http or https"},
		{"missing ca file", `[analytics.publishers.otel.tls]` + "\n" + `ca_file = "/nonexistent/ca.pem"`, "cannot read ca_file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := "endpoint = \"http://otel-collector:4318/v1/logs\"\n"
			if strings.Contains(tc.body, "endpoint =") {
				base = "" // the case sets its own; a second one is a TOML parse error
			}
			path := writeOTelTOML(t, `
[analytics]
enabled = true
enabled_publishers = ["otel"]

[analytics.publishers.otel]
`+base+tc.body+"\n")
			_, err := Load(path)
			require.Error(t, err, "invalid TOML value must fail the load")
			assert.Contains(t, err.Error(), tc.wantErr, "must be rejected for the stated reason")
			t.Logf("rejected with: %v", err)
		})
	}
}

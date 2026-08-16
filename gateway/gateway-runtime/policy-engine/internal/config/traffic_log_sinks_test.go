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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// trafficLogConfig returns a Config with traffic logging enabled and the given
// sink settings applied, ready for Validate.
func trafficLogConfig(mutate func(*TrafficLoggingConfig)) *Config {
	cfg := defaultConfig()
	cfg.TrafficLogging.Enabled = true
	mutate(&cfg.TrafficLogging)
	return cfg
}

func TestNormalizeTrafficLogOutputs(t *testing.T) {
	t.Run("unset defaults to stdout", func(t *testing.T) {
		got, err := NormalizeTrafficLogOutputs(nil)
		require.NoError(t, err)
		assert.Equal(t, []string{TrafficLogSinkStdout}, got)
	})

	t.Run("explicitly empty falls back to stdout", func(t *testing.T) {
		got, err := NormalizeTrafficLogOutputs([]string{})
		require.NoError(t, err)
		assert.Equal(t, []string{TrafficLogSinkStdout}, got)
	})

	t.Run("case and whitespace are normalized", func(t *testing.T) {
		got, err := NormalizeTrafficLogOutputs([]string{" File ", "HTTP"})
		require.NoError(t, err)
		assert.Equal(t, []string{TrafficLogSinkFile, TrafficLogSinkHTTP}, got)
	})

	// A typo must fail loudly: silently ignoring it would leave an operator who
	// asked for a file sink writing request bodies to the container log.
	t.Run("unknown name is an error", func(t *testing.T) {
		_, err := NormalizeTrafficLogOutputs([]string{"flie"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "flie")
	})

	t.Run("duplicate is an error", func(t *testing.T) {
		_, err := NormalizeTrafficLogOutputs([]string{"file", "file"})
		assert.Error(t, err)
	})
}

func TestValidate_TrafficLogFileSink(t *testing.T) {
	t.Run("valid path passes and creates the file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nested", "traffic.log")
		cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
			tl.Outputs = []string{TrafficLogSinkFile}
			tl.File = TrafficLogFileConfig{Path: path, MaxSizeMB: 10}
		})
		require.NoError(t, cfg.Validate())

		// Validation proves the sink works by actually opening it, so a
		// permissions or mount problem surfaces at startup rather than at the
		// first request, once PII is already flowing.
		fi, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
	})

	t.Run("missing path fails closed", func(t *testing.T) {
		cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
			tl.Outputs = []string{TrafficLogSinkFile}
		})
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "traffic_logging.file")
	})

	t.Run("relative path fails closed", func(t *testing.T) {
		cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
			tl.Outputs = []string{TrafficLogSinkFile}
			tl.File = TrafficLogFileConfig{Path: "relative/traffic.log"}
		})
		assert.Error(t, cfg.Validate())
	})

	t.Run("unwritable directory fails closed rather than degrading to stdout", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("running as root: permission checks do not apply")
		}
		dir := t.TempDir()
		require.NoError(t, os.Chmod(dir, 0o500)) // read+execute, no write
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

		cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
			tl.Outputs = []string{TrafficLogSinkFile}
			tl.File = TrafficLogFileConfig{Path: filepath.Join(dir, "sub", "traffic.log")}
		})
		assert.Error(t, cfg.Validate())
	})

	t.Run("negative max size is an error", func(t *testing.T) {
		cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
			tl.Outputs = []string{TrafficLogSinkFile}
			tl.File = TrafficLogFileConfig{Path: filepath.Join(t.TempDir(), "t.log"), MaxSizeMB: -1}
		})
		assert.Error(t, cfg.Validate())
	})
}

func TestValidate_TrafficLogHTTPSink(t *testing.T) {
	base := func(tl *TrafficLoggingConfig) {
		tl.Outputs = []string{TrafficLogSinkHTTP}
		tl.HTTP = defaultTrafficLogHTTPConfig()
		tl.HTTP.Endpoint = "https://splunk.example.com:8088/services/collector/raw"
	}

	t.Run("https endpoint passes", func(t *testing.T) {
		assert.NoError(t, trafficLogConfig(base).Validate())
	})

	t.Run("missing endpoint fails closed", func(t *testing.T) {
		cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
			base(tl)
			tl.HTTP.Endpoint = ""
		})
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "traffic_logging.http")
	})

	// The traffic log carries request and response bodies, so plaintext must be a
	// deliberate opt-in rather than something a typo can produce.
	t.Run("plaintext http requires explicit opt-in", func(t *testing.T) {
		cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
			base(tl)
			tl.HTTP.Endpoint = "http://collector.internal:8088/ingest"
		})
		assert.Error(t, cfg.Validate())

		cfg = trafficLogConfig(func(tl *TrafficLoggingConfig) {
			base(tl)
			tl.HTTP.Endpoint = "http://collector.internal:8088/ingest"
			tl.HTTP.AllowInsecureTransport = true
		})
		assert.NoError(t, cfg.Validate())
	})

	t.Run("non-http scheme is rejected", func(t *testing.T) {
		for _, endpoint := range []string{"file:///etc/passwd", "gopher://x/1", "ftp://x/y"} {
			cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
				base(tl)
				tl.HTTP.Endpoint = endpoint
			})
			assert.Error(t, cfg.Validate(), "endpoint %q must be rejected", endpoint)
		}
	})

	// An unbounded queue in front of a bounded sender is deferred unbounded memory
	// growth, and these lines carry bodies.
	t.Run("queue capacity must be bounded and positive", func(t *testing.T) {
		for _, capacity := range []int{0, -1} {
			cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
				base(tl)
				tl.HTTP.QueueCapacity = capacity
			})
			assert.Error(t, cfg.Validate())
		}
	})

	t.Run("batch and timeout bounds must be positive", func(t *testing.T) {
		mutators := map[string]func(*TrafficLoggingConfig){
			"batch_max_events": func(tl *TrafficLoggingConfig) { tl.HTTP.BatchMaxEvents = 0 },
			"batch_max_bytes":  func(tl *TrafficLoggingConfig) { tl.HTTP.BatchMaxBytes = 0 },
			"flush_interval":   func(tl *TrafficLoggingConfig) { tl.HTTP.FlushInterval = 0 },
			"request_timeout":  func(tl *TrafficLoggingConfig) { tl.HTTP.RequestTimeout = 0 },
			"max_retries":      func(tl *TrafficLoggingConfig) { tl.HTTP.MaxRetries = -1 },
		}
		for name, mutate := range mutators {
			t.Run(name, func(t *testing.T) {
				cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
					base(tl)
					mutate(tl)
				})
				assert.Error(t, cfg.Validate())
			})
		}
	})

	t.Run("on_queue_full must be a known policy", func(t *testing.T) {
		cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
			base(tl)
			tl.HTTP.OnQueueFull = "block"
		})
		assert.Error(t, cfg.Validate())
	})

	t.Run("auth type validation", func(t *testing.T) {
		cases := map[string]struct {
			auth    TrafficLogHTTPAuthConfig
			wantErr bool
		}{
			"none": {TrafficLogHTTPAuthConfig{Type: TrafficLogAuthNone}, false},
			"bearer with token": {TrafficLogHTTPAuthConfig{Type: TrafficLogAuthBearer,
				Bearer: TrafficLogHTTPAuthBearerConfig{Token: "t"}}, false},
			"bearer no token": {TrafficLogHTTPAuthConfig{Type: TrafficLogAuthBearer}, true},
			"basic complete": {TrafficLogHTTPAuthConfig{Type: TrafficLogAuthBasic,
				Basic: TrafficLogHTTPAuthBasicConfig{Username: "u", Password: "p"}}, false},
			"basic missing pass": {TrafficLogHTTPAuthConfig{Type: TrafficLogAuthBasic,
				Basic: TrafficLogHTTPAuthBasicConfig{Username: "u"}}, true},
			"header complete": {TrafficLogHTTPAuthConfig{Type: TrafficLogAuthHeader,
				Header: TrafficLogHTTPAuthHeaderConfig{Name: "Authorization", Value: "Splunk x"}}, false},
			"header missing name": {TrafficLogHTTPAuthConfig{Type: TrafficLogAuthHeader,
				Header: TrafficLogHTTPAuthHeaderConfig{Value: "Splunk x"}}, true},
			"unknown type": {TrafficLogHTTPAuthConfig{Type: "oauth2"}, true},

			// The sub-table shape makes this expressible, and it must still fail:
			// fields under a type that was not selected are never consulted.
			"bearer token supplied under the wrong sub-table": {TrafficLogHTTPAuthConfig{
				Type:  TrafficLogAuthBearer,
				Basic: TrafficLogHTTPAuthBasicConfig{Username: "u", Password: "p"},
			}, true},
		}
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
					base(tl)
					tl.HTTP.Auth = tc.auth
				})
				if tc.wantErr {
					assert.Error(t, cfg.Validate())
				} else {
					assert.NoError(t, cfg.Validate())
				}
			})
		}
	})

	t.Run("tls material must exist and be usable", func(t *testing.T) {
		cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
			base(tl)
			tl.HTTP.TLS.CAFile = "/nonexistent/ca.pem"
		})
		assert.Error(t, cfg.Validate())

		garbage := filepath.Join(t.TempDir(), "ca.pem")
		require.NoError(t, os.WriteFile(garbage, []byte("not a certificate"), 0o600))
		cfg = trafficLogConfig(func(tl *TrafficLoggingConfig) {
			base(tl)
			tl.HTTP.TLS.CAFile = garbage
		})
		assert.Error(t, cfg.Validate())
	})

	t.Run("mTLS cert and key must be set together", func(t *testing.T) {
		cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
			base(tl)
			tl.HTTP.TLS.CertFile = "/some/cert.pem"
		})
		assert.Error(t, cfg.Validate())
	})
}

func TestValidate_TrafficLogOutputsInteraction(t *testing.T) {
	t.Run("unknown sink name fails startup", func(t *testing.T) {
		cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
			tl.Outputs = []string{"flie"}
		})
		assert.Error(t, cfg.Validate())
	})

	// Selecting stdout alongside file must validate both, but a broken file sink
	// still fails: a partially usable set is not "good enough".
	t.Run("broken file sink fails even when stdout is also selected", func(t *testing.T) {
		cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
			tl.Outputs = []string{TrafficLogSinkStdout, TrafficLogSinkFile}
			tl.File = TrafficLogFileConfig{Path: "not-absolute.log"}
		})
		assert.Error(t, cfg.Validate())
	})

	// Sink config is only read for sinks that are actually selected, so a stale
	// [traffic_logging.http] block left behind by a rollback cannot fail startup.
	t.Run("unselected sink config is not validated", func(t *testing.T) {
		cfg := trafficLogConfig(func(tl *TrafficLoggingConfig) {
			tl.Outputs = []string{TrafficLogSinkStdout}
			tl.HTTP.Endpoint = "not a url at all"
			tl.File.Path = "also-not-absolute"
		})
		assert.NoError(t, cfg.Validate())
	})

	t.Run("disabled traffic logging skips sink validation entirely", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.TrafficLogging.Enabled = false
		cfg.TrafficLogging.Outputs = []string{TrafficLogSinkFile}
		cfg.TrafficLogging.File = TrafficLogFileConfig{Path: "relative.log"}
		assert.NoError(t, cfg.Validate())
	})
}

func TestEffectiveShutdownTimeout(t *testing.T) {
	assert.Equal(t, DefaultTrafficLogShutdownTimeout,
		TrafficLoggingConfig{}.EffectiveShutdownTimeout(),
		"an unset timeout must fall back to the default rather than skipping the flush")
	assert.Equal(t, 9*time.Second,
		TrafficLoggingConfig{ShutdownTimeout: 9 * time.Second}.EffectiveShutdownTimeout())
}

// TestShippedTemplateMatchesTrafficLogDefaults loads the config-template.toml we
// actually ship and asserts its [traffic_logging] sink values equal the code
// defaults. The template is the reference an operator reads before writing their
// own config, so a value that drifts from the code does not just go stale — it
// documents a number the gateway will not use.
func TestShippedTemplateMatchesTrafficLogDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "..", "..", "configs", "config-template.toml"))
	require.NoError(t, err, "the shipped config-template.toml must load and validate")

	tl := cfg.TrafficLogging
	assert.Equal(t, []string{TrafficLogSinkStdout}, tl.Outputs,
		"the template must ship the historical stdout-only behavior")
	assert.Equal(t, DefaultTrafficLogShutdownTimeout, tl.ShutdownTimeout)

	// Path and Endpoint are deliberately empty in both: each is required only
	// when its sink is selected, so the template must not quietly supply one.
	assert.Equal(t, defaultTrafficLogFileConfig(), tl.File)
	assert.Equal(t, defaultTrafficLogHTTPConfig(), tl.HTTP)
}

func TestDefaultConfigTrafficLoggingPreservesStdout(t *testing.T) {
	cfg := defaultConfig()
	assert.Equal(t, []string{TrafficLogSinkStdout}, cfg.TrafficLogging.Outputs,
		"an existing deployment that upgrades without touching its config must keep stdout")
	assert.False(t, cfg.TrafficLogging.Enabled)
	assert.Equal(t, DefaultTrafficLogShutdownTimeout, cfg.TrafficLogging.ShutdownTimeout)
}

// TestResolveTrafficLogFilePath_RejectsTraversal pins that a ".." segment is
// rejected rather than silently resolved. filepath.Clean would turn
// "/var/log/wso2/../../etc/x.log" into "/etc/x.log" — a file written outside the
// volume the operator mounted, and past the chart's mount-containment check.
func TestResolveTrafficLogFilePath_RejectsTraversal(t *testing.T) {
	for _, p := range []string{
		"/var/log/wso2/../../etc/traffic.log",
		"/var/log/wso2/traffic/../../../traffic.log",
		"/../traffic.log",
	} {
		_, err := ResolveTrafficLogFilePath(p)
		assert.Error(t, err, "path %q escapes its directory and must be rejected", p)
	}
	// A clean absolute path is still accepted, and a single dot is harmless.
	for _, p := range []string{"/var/log/wso2/traffic/traffic.log", "/var/log/wso2/./traffic.log"} {
		got, err := ResolveTrafficLogFilePath(p)
		require.NoError(t, err, "path %q is legitimate", p)
		assert.True(t, filepath.IsAbs(got))
	}
}

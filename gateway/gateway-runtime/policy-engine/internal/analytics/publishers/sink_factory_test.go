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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
)

func sinkNames(sinks []Sink) []string {
	out := make([]string, 0, len(sinks))
	for _, s := range sinks {
		out = append(out, s.Name())
	}
	return out
}

func closeAll(t *testing.T, sinks []Sink) {
	t.Helper()
	for _, s := range sinks {
		_ = s.Close(context.Background())
	}
}

// Unset outputs must resolve to stdout, so an existing deployment that upgrades
// without touching its config keeps exactly the behavior it had.
func TestNewSinks_DefaultsToStdout(t *testing.T) {
	sinks, err := newSinks(&config.TrafficLoggingConfig{Enabled: true})
	require.NoError(t, err)
	t.Cleanup(func() { closeAll(t, sinks) })
	assert.Equal(t, []string{config.TrafficLogSinkStdout}, sinkNames(sinks))
}

func TestNewSinks_FileOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.log")
	sinks, err := newSinks(&config.TrafficLoggingConfig{
		Enabled: true,
		Outputs: []string{config.TrafficLogSinkFile},
		File:    config.TrafficLogFileConfig{Path: path, MaxSizeMB: 10},
	})
	require.NoError(t, err)
	t.Cleanup(func() { closeAll(t, sinks) })

	assert.Equal(t, []string{config.TrafficLogSinkFile}, sinkNames(sinks))
	assert.NotContains(t, sinkNames(sinks), config.TrafficLogSinkStdout,
		"selecting the file sink must take stdout out of the picture entirely")
}

func TestNewSinks_StdoutAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.log")
	sinks, err := newSinks(&config.TrafficLoggingConfig{
		Enabled: true,
		Outputs: []string{config.TrafficLogSinkStdout, config.TrafficLogSinkFile},
		File:    config.TrafficLogFileConfig{Path: path, MaxSizeMB: 10},
	})
	require.NoError(t, err)
	t.Cleanup(func() { closeAll(t, sinks) })
	assert.Equal(t, []string{config.TrafficLogSinkStdout, config.TrafficLogSinkFile}, sinkNames(sinks))
}

// A typo'd sink name must fail loudly. Silently ignoring it would leave an operator
// who asked for a file sink writing request bodies to the container log.
func TestNewSinks_RejectsUnknownName(t *testing.T) {
	_, err := newSinks(&config.TrafficLoggingConfig{
		Enabled: true,
		Outputs: []string{"flie"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flie")
}

func TestNewSinks_RejectsDuplicate(t *testing.T) {
	_, err := newSinks(&config.TrafficLoggingConfig{
		Enabled: true,
		Outputs: []string{config.TrafficLogSinkStdout, config.TrafficLogSinkStdout},
	})
	assert.Error(t, err)
}

// An unusable file sink must fail construction, never silently degrade to stdout.
func TestNewSinks_UnusableFileSinkFailsClosed(t *testing.T) {
	_, err := newSinks(&config.TrafficLoggingConfig{
		Enabled: true,
		Outputs: []string{config.TrafficLogSinkFile},
		File:    config.TrafficLogFileConfig{Path: "relative/traffic.log"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "traffic_logging.file")
}

// A failure part way through the list must not leave earlier sinks open: the file
// created for the first sink has to be closed before the error propagates.
func TestNewSinks_ClosesAlreadyBuiltSinksOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.log")
	_, err := newSinks(&config.TrafficLoggingConfig{
		Enabled: true,
		Outputs: []string{config.TrafficLogSinkFile, config.TrafficLogSinkHTTP},
		File:    config.TrafficLogFileConfig{Path: path, MaxSizeMB: 10},
		HTTP: config.TrafficLogHTTPConfig{
			// Missing token for the bearer type -> construction fails.
			Endpoint:       "https://example.invalid/ingest",
			QueueCapacity:  10,
			BatchMaxEvents: 1,
			Auth:           config.TrafficLogHTTPAuthConfig{Type: config.TrafficLogAuthBearer},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "traffic_logging.http")

	// The file sink was built first, so the file exists; it must have been closed.
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr)
}

// Traffic logging disabled means no sink is opened at all: no file is created on
// disk and no sender goroutine is started for a feature nobody enabled.
func TestNewLog_DisabledBuildsNoSinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.log")
	l, err := NewLog(&config.TrafficLoggingConfig{
		Enabled: false,
		Outputs: []string{config.TrafficLogSinkFile},
		File:    config.TrafficLogFileConfig{Path: path, MaxSizeMB: 10},
	})
	require.NoError(t, err)
	assert.Empty(t, l.sinks)

	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "no file may be created when traffic logging is off")
}

// NewLog must surface a sink failure so the caller can refuse to start.
func TestNewLog_PropagatesSinkFailure(t *testing.T) {
	_, err := NewLog(&config.TrafficLoggingConfig{
		Enabled: true,
		Outputs: []string{config.TrafficLogSinkFile},
		File:    config.TrafficLogFileConfig{Path: ""},
	})
	assert.Error(t, err)
}

func TestLog_CloseClosesEverySink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.log")
	l, err := NewLog(&config.TrafficLoggingConfig{
		Enabled: true,
		Outputs: []string{config.TrafficLogSinkStdout, config.TrafficLogSinkFile},
		File:    config.TrafficLogFileConfig{Path: path, MaxSizeMB: 10},
	})
	require.NoError(t, err)
	require.NoError(t, l.Close(context.Background()))
	require.NoError(t, l.Close(context.Background()), "Close must be idempotent")
}

// TestNewSinks_MaterializesFailureCountersAtZero guards the ambiguity described
// on initSinkMetrics: without this, traffic_log_dropped_total is absent from the
// scrape on a healthy gateway, so a dashboard cannot distinguish "no drops" from
// "the metrics path is broken".
func TestNewSinks_MaterializesFailureCountersAtZero(t *testing.T) {
	reg := metrics.GetRegistry() // TestMain enabled metrics before Init

	dir := t.TempDir()
	cfg := &config.TrafficLoggingConfig{
		Outputs: []string{config.TrafficLogSinkStdout, config.TrafficLogSinkFile},
		File:    config.TrafficLogFileConfig{Path: filepath.Join(dir, "t.log"), MaxSizeMB: 1},
	}
	sinks, err := newSinks(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, s := range sinks {
			_ = s.Close(context.Background())
		}
	})

	families, err := reg.Gather()
	require.NoError(t, err)
	// Key on name=value pairs, not positional values: Gather sorts labels by
	// label NAME, so a positional key silently reorders (reason before sink).
	got := map[string]bool{}
	for _, f := range families {
		for _, m := range f.GetMetric() {
			key := f.GetName()
			for _, l := range m.GetLabel() {
				key += "|" + l.GetName() + "=" + l.GetValue()
			}
			got[key] = true
		}
	}

	// Present at zero for every configured sink, before anything is written.
	for _, want := range []string{
		"policy_engine_traffic_log_dropped_total|reason=write_failed|sink=stdout",
		"policy_engine_traffic_log_dropped_total|reason=write_failed|sink=file",
		"policy_engine_traffic_log_dropped_total|reason=rotate_failed|sink=file",
		"policy_engine_traffic_log_write_errors_total|code=rotate|sink=file",
		"policy_engine_traffic_log_write_errors_total|code=write|sink=stdout",
		"policy_engine_traffic_log_written_total|sink=file",
		"policy_engine_traffic_log_written_total|sink=stdout",
	} {
		assert.True(t, got[want], "series %q must exist at zero on a healthy gateway", want)
	}
	// Capacity must be published so the backlog alert can be a ratio.
	assert.False(t, got["policy_engine_traffic_log_queue_capacity|sink=http"],
		"no http sink was configured, so no capacity should be published")

	// A sink that was not configured must NOT appear — an operator reading the
	// scrape should see only the sinks actually in use.
	assert.False(t, got["policy_engine_traffic_log_dropped_total|reason=queue_full|sink=http"],
		"an unconfigured sink must not be advertised")
}

// TestNewSinks_PublishesQueueCapacity pins the metric the TrafficLogQueueBacklog
// alert divides by. A fixed depth threshold cannot work across deployments: 1000
// is 10% of the default 10000 queue but unreachable on a queue configured
// smaller, so the alert has to compare depth against this.
func TestNewSinks_PublishesQueueCapacity(t *testing.T) {
	srv := httptest.NewServer(&receiver{status: http.StatusOK})
	t.Cleanup(srv.Close)

	cfg := &config.TrafficLoggingConfig{
		Outputs: []string{config.TrafficLogSinkHTTP},
		HTTP:    httpSinkCfg(srv.URL),
	}
	cfg.HTTP.QueueCapacity = 4242
	sinks, err := newSinks(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, s := range sinks {
			_ = s.Close(context.Background())
		}
	})

	families, err := metrics.GetRegistry().Gather()
	require.NoError(t, err)
	var got float64
	for _, f := range families {
		if f.GetName() != "policy_engine_traffic_log_queue_capacity" {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "sink" && l.GetValue() == config.TrafficLogSinkHTTP {
					got = m.GetGauge().GetValue()
				}
			}
		}
	}
	assert.Equal(t, float64(4242), got,
		"the configured queue_capacity must be published as a gauge")
}

// TestNewSinks_CleanupIsBounded pins CodeRabbit #3: the rollback close when a
// later sink fails to build uses a bounded context, so a wedged sink cannot hang
// startup and hide the construction error the operator needs to see.
func TestNewSinks_CleanupIsBounded(t *testing.T) {
	srv := httptest.NewServer(&receiver{status: http.StatusOK})
	t.Cleanup(srv.Close)

	// http builds, file then fails on a relative path -> closeAll runs.
	cfg := &config.TrafficLoggingConfig{
		Outputs: []string{config.TrafficLogSinkHTTP, config.TrafficLogSinkFile},
		HTTP:    httpSinkCfg(srv.URL),
		File:    config.TrafficLogFileConfig{Path: "relative/traffic.log"},
	}
	done := make(chan error, 1)
	go func() { _, err := newSinks(cfg); done <- err }()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "traffic_logging.file")
	case <-time.After(sinkCleanupTimeout + 5*time.Second):
		t.Fatal("newSinks did not return: the cleanup close is not bounded")
	}
}

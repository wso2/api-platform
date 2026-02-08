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

package metrics

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// =============================================================================
// NewServer Tests
// =============================================================================

func TestNewServer(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Port:    9100,
	}

	server := NewServer(cfg)

	require.NotNil(t, server)
	assert.Equal(t, cfg, server.cfg)
	require.NotNil(t, server.httpServer)
	assert.Equal(t, ":9100", server.httpServer.Addr)
}

func TestNewServer_DifferentPort(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Port:    9200,
	}

	server := NewServer(cfg)

	require.NotNil(t, server)
	assert.Equal(t, ":9200", server.httpServer.Addr)
}

// =============================================================================
// Server Start/Stop Tests
// =============================================================================

func TestServer_StartStop(t *testing.T) {
	cfg := &config.MetricsConfig{
		Enabled: true,
		Port:    9101,
	}

	server := NewServer(cfg)

	// Start in goroutine
	startCtx := context.Background()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(startCtx)
	}()

	// Wait for server to be ready with retries
	var resp *http.Response
	var err error
	for i := 0; i < 10; i++ {
		time.Sleep(50 * time.Millisecond)
		resp, err = http.Get("http://localhost:9101/health")
		if err == nil {
			resp.Body.Close()
			break
		}
	}
	require.NoError(t, err, "server should be reachable after startup")

	// Test metrics endpoint
	resp, err = http.Get("http://localhost:9101/metrics")
	require.NoError(t, err, "metrics endpoint should be reachable")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Test health endpoint
	resp, err = http.Get("http://localhost:9101/health")
	require.NoError(t, err, "health endpoint should be reachable")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Stop the server
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Stop(stopCtx)
	assert.NoError(t, err)

	// Check that Start returned without error (it should return nil on graceful shutdown)
	select {
	case startErr := <-errCh:
		// Either nil or http.ErrServerClosed is acceptable
		if startErr != nil && startErr != http.ErrServerClosed {
			t.Errorf("unexpected error from Start: %v", startErr)
		}
	case <-time.After(2 * time.Second):
		t.Error("Start did not return after Stop")
	}
}

// =============================================================================
// StartMemoryMetricsUpdater Tests
// =============================================================================

func TestStartMemoryMetricsUpdater(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start the updater with a short interval
	StartMemoryMetricsUpdater(ctx, 50*time.Millisecond)

	// Wait for at least one update cycle
	time.Sleep(100 * time.Millisecond)

	// The function should return when context is cancelled
	<-ctx.Done()
}

func TestStartMemoryMetricsUpdater_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Start the updater
	StartMemoryMetricsUpdater(ctx, 1*time.Second)

	// Cancel immediately
	cancel()

	// The goroutine should exit cleanly (no way to verify without race detector,
	// but this at least ensures no panic)
	time.Sleep(50 * time.Millisecond)
}

// =============================================================================
// UpdateMemoryMetrics Tests
// =============================================================================

func TestUpdateMemoryMetrics(t *testing.T) {
	// This should not panic
	UpdateMemoryMetrics()
}

// =============================================================================
// Init Tests
// =============================================================================

func TestInit(t *testing.T) {
	registry := Init()

	require.NotNil(t, registry)
}

// =============================================================================
// Noop Wrapper Tests
// =============================================================================

func TestNoopCounter_Inc(t *testing.T) {
	// Create noop counter when metrics disabled
	origEnabled := Enabled
	Enabled = false
	defer func() { Enabled = origEnabled }()

	counter := newCounter(prometheus.CounterOpts{
		Name: "test_counter",
		Help: "test help",
	})
	// These should not panic even when metrics are disabled
	counter.Inc()
	counter.Add(1.5)
}

func TestNoopCounterVec_Operations(t *testing.T) {
	origEnabled := Enabled
	Enabled = false
	defer func() { Enabled = origEnabled }()

	counterVec := newCounterVec(prometheus.CounterOpts{
		Name: "test_counter_vec",
		Help: "test help",
	}, []string{"label"})
	counterVec.WithLabelValues("value").Inc()
	counterVec.WithLabelValues("value").Add(2.0)
}

func TestNoopHistogram_Observe(t *testing.T) {
	origEnabled := Enabled
	Enabled = false
	defer func() { Enabled = origEnabled }()

	histogram := newHistogram(prometheus.HistogramOpts{
		Name: "test_histogram",
		Help: "test help",
	})
	// This should not panic
	histogram.Observe(1.5)
}

func TestNoopHistogramVec_Observe(t *testing.T) {
	origEnabled := Enabled
	Enabled = false
	defer func() { Enabled = origEnabled }()

	histogramVec := newHistogramVec(prometheus.HistogramOpts{
		Name: "test_histogram_vec",
		Help: "test help",
	}, []string{"label"})
	histogramVec.WithLabelValues("value").Observe(1.5)
}

func TestNoopGauge_Operations(t *testing.T) {
	origEnabled := Enabled
	Enabled = false
	defer func() { Enabled = origEnabled }()

	gauge := newGauge(prometheus.GaugeOpts{
		Name: "test_gauge",
		Help: "test help",
	})
	// These should not panic
	gauge.Set(5.0)
	gauge.Inc()
	gauge.Dec()
	gauge.Add(1.0)
	gauge.Sub(0.5)
}

func TestNoopGaugeVec_Operations(t *testing.T) {
	origEnabled := Enabled
	Enabled = false
	defer func() { Enabled = origEnabled }()

	gaugeVec := newGaugeVec(prometheus.GaugeOpts{
		Name: "test_gauge_vec",
		Help: "test help",
	}, []string{"label"})
	gaugeVec.WithLabelValues("value").Set(10.0)
	gaugeVec.WithLabelValues("value").Inc()
	gaugeVec.WithLabelValues("value").Dec()
	gaugeVec.WithLabelValues("value").Add(1.0)
	gaugeVec.WithLabelValues("value").Sub(0.5)
}

func TestNoopGaugeFunc(t *testing.T) {
	origEnabled := Enabled
	Enabled = false
	defer func() { Enabled = origEnabled }()

	// When disabled, returns nil (by design - registration will skip it)
	gaugeFunc := newGaugeFunc(prometheus.GaugeOpts{
		Name: "test_gauge_func",
		Help: "test help",
	}, func() float64 {
		return 42.0
	})
	// GaugeFunc returns nil when disabled - this is expected behavior
	assert.Nil(t, gaugeFunc)
}

// =============================================================================
// Enabled Wrapper Tests (with metrics enabled)
// =============================================================================

func TestEnabledCounter_Inc(t *testing.T) {
	origEnabled := Enabled
	Enabled = true
	defer func() { Enabled = origEnabled }()

	counter := newCounter(prometheus.CounterOpts{
		Name: "test_enabled_counter_inc",
		Help: "test help",
	})
	counter.Inc()
	counter.Add(1.5)
}

func TestEnabledHistogram_Observe(t *testing.T) {
	origEnabled := Enabled
	Enabled = true
	defer func() { Enabled = origEnabled }()

	histogram := newHistogram(prometheus.HistogramOpts{
		Name: "test_enabled_histogram_observe",
		Help: "test help",
	})
	histogram.Observe(1.5)
}

func TestEnabledGauge_Operations(t *testing.T) {
	origEnabled := Enabled
	Enabled = true
	defer func() { Enabled = origEnabled }()

	gauge := newGauge(prometheus.GaugeOpts{
		Name: "test_enabled_gauge_ops",
		Help: "test help",
	})
	gauge.Set(5.0)
	gauge.Inc()
	gauge.Dec()
	gauge.Add(1.0)
	gauge.Sub(0.5)
}

func TestEnabledGaugeFunc(t *testing.T) {
	origEnabled := Enabled
	Enabled = true
	defer func() { Enabled = origEnabled }()

	gaugeFunc := newGaugeFunc(prometheus.GaugeOpts{
		Name: "test_enabled_gauge_func_call",
		Help: "test help",
	}, func() float64 {
		return 42.0
	})
	require.NotNil(t, gaugeFunc)
}

// =============================================================================
// GetRegistry Test
// =============================================================================

func TestGetRegistry(t *testing.T) {
	// Initialize first
	Init()
	registry := GetRegistry()
	require.NotNil(t, registry)
}

// =============================================================================
// SetEnabled Tests
// =============================================================================

func TestSetEnabled(t *testing.T) {
	origEnabled := Enabled
	defer func() { Enabled = origEnabled }()

	SetEnabled(true)
	assert.True(t, IsEnabled())

	SetEnabled(false)
	assert.False(t, IsEnabled())
}

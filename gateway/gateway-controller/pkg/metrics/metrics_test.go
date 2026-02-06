/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"sync"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"log/slog"
)

func TestInit(t *testing.T) {
	// Reset state for clean test
	once = resetOnce()
	registry = nil
	Enabled = false

	// Test disabled metrics
	reg := Init()
	if reg == nil {
		t.Error("Init() returned nil even when metrics disabled")
	}

	// Verify that metrics are noop when disabled
	// These should not panic even though registry is minimal
	APIOperationsTotal.WithLabelValues("create", "success", "rest").Inc()
	APIsTotal.WithLabelValues("rest", "deployed").Set(1)
}

func TestInitEnabled(t *testing.T) {
	// Reset state for clean test
	once = resetOnce()
	registry = nil
	Enabled = true

	reg := Init()
	if reg == nil {
		t.Error("Init() returned nil when metrics enabled")
	}

	// Verify that real metrics work
	APIOperationsTotal.WithLabelValues("create", "success", "rest").Inc()
	APIsTotal.WithLabelValues("rest", "deployed").Set(5)
}

func TestGetRegistry(t *testing.T) {
	// Reset state
	once = resetOnce()
	registry = nil
	Enabled = true

	// GetRegistry should initialize if not already done
	reg := GetRegistry()
	if reg == nil {
		t.Error("GetRegistry() returned nil")
	}

	// Second call should return same registry
	reg2 := GetRegistry()
	if reg != reg2 {
		t.Error("GetRegistry() returned different registry on second call")
	}
}

func TestUpdateMemoryMetrics(t *testing.T) {
	// Reset state
	once = resetOnce()
	registry = nil
	Enabled = true
	Init()

	// Should not panic
	UpdateMemoryMetrics()
}

func TestUpdateMemoryMetricsDisabled(t *testing.T) {
	// Reset state
	once = resetOnce()
	registry = nil
	Enabled = false
	Init()

	// Should not panic even when disabled
	UpdateMemoryMetrics()
}

func TestNoopMetrics(t *testing.T) {
	// Reset state
	once = resetOnce()
	registry = nil
	Enabled = false
	Init()

	// Test that all noop metrics work without panic
	t.Run("CounterVec noop", func(t *testing.T) {
		APIOperationsTotal.WithLabelValues("test", "test", "test").Inc()
		APIOperationsTotal.WithLabelValues("test", "test", "test").Add(5)
	})

	t.Run("GaugeVec noop", func(t *testing.T) {
		APIsTotal.WithLabelValues("test", "test").Set(10)
		APIsTotal.WithLabelValues("test", "test").Inc()
		APIsTotal.WithLabelValues("test", "test").Dec()
		APIsTotal.WithLabelValues("test", "test").Add(1)
		APIsTotal.WithLabelValues("test", "test").Sub(1)
	})

	t.Run("HistogramVec noop", func(t *testing.T) {
		APIOperationDurationSeconds.WithLabelValues("test", "test").Observe(0.5)
	})

	t.Run("Histogram noop", func(t *testing.T) {
		DeploymentLatencySeconds.Observe(1.0)
	})

	t.Run("Gauge noop", func(t *testing.T) {
		Up.Set(1)
		Up.Inc()
		Up.Dec()
		Up.Add(1)
		Up.Sub(1)
	})

	t.Run("Counter noop", func(t *testing.T) {
		ControlPlaneReconnectionsTotal.Inc()
		ControlPlaneReconnectionsTotal.Add(5)
	})
}

func TestRealMetrics(t *testing.T) {
	// Reset state
	once = resetOnce()
	registry = nil
	Enabled = true
	Init()

	// Test that all real metrics work without panic
	t.Run("CounterVec real", func(t *testing.T) {
		APIOperationsTotal.WithLabelValues("create", "success", "rest").Inc()
		APIOperationsTotal.WithLabelValues("delete", "error", "rest").Add(3)
	})

	t.Run("GaugeVec real", func(t *testing.T) {
		APIsTotal.WithLabelValues("rest", "deployed").Set(10)
		APIsTotal.WithLabelValues("rest", "deployed").Inc()
		APIsTotal.WithLabelValues("rest", "deployed").Dec()
	})

	t.Run("HistogramVec real", func(t *testing.T) {
		APIOperationDurationSeconds.WithLabelValues("create", "rest").Observe(0.123)
	})

	t.Run("Histogram real", func(t *testing.T) {
		DeploymentLatencySeconds.Observe(2.5)
	})

	t.Run("Gauge real", func(t *testing.T) {
		Up.Set(1)
		ConcurrentRequests.Inc()
		ConcurrentRequests.Dec()
	})

	t.Run("Counter real", func(t *testing.T) {
		ControlPlaneReconnectionsTotal.Inc()
		ControlPlaneReconnectionsTotal.Add(2)
	})
}

// resetOnce returns a new sync.Once to reset the initialization state
func resetOnce() (o sync.Once) {
	return
}

func TestIsEnabled(t *testing.T) {
	// Reset state
	once = resetOnce()
	registry = nil
	Enabled = false

	if IsEnabled() != false {
		t.Error("IsEnabled() should return false when metrics disabled")
	}

	Enabled = true
	if IsEnabled() != true {
		t.Error("IsEnabled() should return true when metrics enabled")
	}
}

func TestSetEnabled(t *testing.T) {
	// Reset state
	once = resetOnce()
	registry = nil

	SetEnabled(false)
	if Enabled != false {
		t.Error("SetEnabled(false) did not set Enabled to false")
	}

	SetEnabled(true)
	if Enabled != true {
		t.Error("SetEnabled(true) did not set Enabled to true")
	}
}

func TestNewServer(t *testing.T) {
	// Reset state
	once = resetOnce()
	registry = nil
	Enabled = true
	Init()

	cfg := &config.MetricsConfig{Port: 9090}
	logger := slog.Default()

	server := NewServer(cfg, logger)
	if server == nil {
		t.Error("NewServer() returned nil")
	}

	if server.cfg.Port != 9090 {
		t.Errorf("NewServer port = %d, want 9090", server.cfg.Port)
	}

	if server.httpServer == nil {
		t.Error("NewServer did not initialize HTTP server")
	}
}

func TestServer_Stop(t *testing.T) {
	// Reset state
	once = resetOnce()
	registry = nil
	Enabled = true
	Init()

	cfg := &config.MetricsConfig{Port: 0}
	logger := slog.Default()
	server := NewServer(cfg, logger)

	// Stop should not panic even if server wasn't started
	ctx := context.Background()
	err := server.Stop(ctx)
	// Stopping a server that never started returns no error
	if err != nil {
		t.Logf("Stop returned error (acceptable): %v", err)
	}
}

func TestStartMemoryMetricsUpdater(t *testing.T) {
	// Reset state
	once = resetOnce()
	registry = nil
	Enabled = true
	Init()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the updater in background
	go StartMemoryMetricsUpdater(ctx, 100*time.Millisecond)

	// Wait a bit to let it run
	time.Sleep(250 * time.Millisecond)

	// Cancel context to stop it
	cancel()

	// Wait a bit for cleanup
	time.Sleep(50 * time.Millisecond)
}

func TestServer_Start(t *testing.T) {
	// Reset state
	once = resetOnce()
	registry = nil
	Enabled = true
	Init()

	// Use port 0 to get any available port
	cfg := &config.MetricsConfig{Port: 0}
	logger := slog.Default()
	server := NewServer(cfg, logger)

	// Start should begin listening (but fail on port 0 bind issues are OK)
	err := server.Start()
	if err != nil {
		t.Logf("Start returned error (may be acceptable): %v", err)
	}

	// Clean up
	ctx := context.Background()
	server.Stop(ctx)
}

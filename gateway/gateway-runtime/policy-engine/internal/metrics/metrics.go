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
	"runtime"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

const (
	namespace = "policy_engine"
)

var (
	once     sync.Once
	registry *prometheus.Registry

	RequestsTotal          CounterVec
	RequestDurationSeconds HistogramVec
	RequestErrorsTotal     CounterVec
	ShortCircuitsTotal     CounterVec

	PolicyExecutionsTotal CounterVec
	PolicyDurationSeconds HistogramVec
	PolicySkippedTotal    CounterVec
	PoliciesPerChain      GaugeVec

	PolicyChainsLoaded GaugeVec
	XDSUpdatesTotal    CounterVec
	XDSConnectionState GaugeVec
	SnapshotSize       GaugeVec

	ActiveStreams               Gauge
	BodyBytesProcessed          CounterVec
	ContextBuildDurationSeconds HistogramVec

	Up                    Gauge
	GRPCConnectionsActive GaugeVec
	Goroutines            GaugeFunc
	MemoryBytes           GaugeVec

	PolicyErrorsTotal        CounterVec
	StreamErrorsTotal        CounterVec
	RouteLookupFailuresTotal Counter
	PanicRecoveriesTotal     CounterVec
)

// initMetrics initializes all metric variables.
// This must be called after SetEnabled() to ensure proper noop behavior when disabled.
func initMetrics() {
	RequestsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "requests_total",
			Help:      "Total number of requests processed by the policy engine",
		},
		[]string{"phase", "route", "api_name", "api_version"},
	)

	RequestDurationSeconds = newHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "request_duration_seconds",
			Help:      "Duration of request processing in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5},
		},
		[]string{"phase", "route"},
	)

	RequestErrorsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "request_errors_total",
			Help:      "Total number of request processing errors",
		},
		[]string{"phase", "error_type", "route"},
	)

	ShortCircuitsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "short_circuits_total",
			Help:      "Total number of short-circuited requests (e.g., auth failures, rate limits)",
		},
		[]string{"route", "policy_name"},
	)

	PolicyExecutionsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "policy_executions_total",
			Help:      "Total number of policy executions",
		},
		[]string{"policy_name", "policy_version", "api", "route", "status"},
	)

	PolicyDurationSeconds = newHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "policy_duration_seconds",
			Help:      "Duration of individual policy execution in seconds",
			Buckets:   []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		},
		[]string{"policy_name", "policy_version", "api", "route"},
	)

	PolicySkippedTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "policy_skipped_total",
			Help:      "Total number of skipped policies",
		},
		[]string{"policy_name", "api", "route", "reason"},
	)

	PoliciesPerChain = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "policies_per_chain",
			Help:      "Current number of policies in each policy chain",
		},
		[]string{"route", "api"},
	)

	PolicyChainsLoaded = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "policy_chains_loaded",
			Help:      "Number of policy chains currently loaded",
		},
		[]string{"mode"},
	)

	XDSUpdatesTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "xds_updates_total",
			Help:      "Total number of xDS configuration updates",
		},
		[]string{"status", "type"},
	)

	XDSConnectionState = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "xds_connection_state",
			Help:      "Current xDS connection state (1=connected, 0=disconnected)",
		},
		[]string{"state"},
	)

	SnapshotSize = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "snapshot_size",
			Help:      "Size of received xDS snapshot resources",
		},
		[]string{"resource_type"},
	)

	ActiveStreams = newGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_streams",
			Help:      "Number of active ext_proc streams",
		},
	)

	BodyBytesProcessed = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "body_bytes_processed",
			Help:      "Total bytes of request/response body processed",
		},
		[]string{"phase", "operation"},
	)

	ContextBuildDurationSeconds = newHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "context_build_duration_seconds",
			Help:      "Duration of building execution context in seconds",
			Buckets:   []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05},
		},
		[]string{"type"},
	)

	Up = newGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Policy engine liveness indicator (1=up, 0=down)",
		},
	)

	GRPCConnectionsActive = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "grpc_connections_active",
			Help:      "Number of active gRPC connections",
		},
		[]string{"type"},
	)

	Goroutines = newGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "goroutines",
			Help:      "Current number of goroutines",
		},
		func() float64 {
			return float64(runtime.NumGoroutine())
		},
	)

	MemoryBytes = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "memory_bytes",
			Help:      "Memory usage in bytes",
		},
		[]string{"type"},
	)

	PolicyErrorsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "policy_errors_total",
			Help:      "Total number of policy-specific errors",
		},
		[]string{"policy_name", "error_type"},
	)

	StreamErrorsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "stream_errors_total",
			Help:      "Total number of gRPC stream errors",
		},
		[]string{"error_type"},
	)

	RouteLookupFailuresTotal = newCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "route_lookup_failures_total",
			Help:      "Total number of route lookup failures",
		},
	)

	PanicRecoveriesTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "panic_recoveries_total",
			Help:      "Total number of panic recoveries",
		},
		[]string{"component"},
	)
}

func registerCounterVec(v CounterVec) {
	if !Enabled {
		return
	}
	if wrapper, ok := v.(*counterVecWrapper); ok {
		if err := registry.Register(wrapper.CounterVec); err != nil {
			// Already registered or other error - ignore
		}
	}
}

func registerHistogramVec(v HistogramVec) {
	if !Enabled {
		return
	}
	if wrapper, ok := v.(*histogramVecWrapper); ok {
		if err := registry.Register(wrapper.HistogramVec); err != nil {
			// Already registered or other error - ignore
		}
	}
}

func registerGaugeVec(v GaugeVec) {
	if !Enabled {
		return
	}
	if wrapper, ok := v.(*gaugeVecWrapper); ok {
		if err := registry.Register(wrapper.GaugeVec); err != nil {
			// Already registered or other error - ignore
		}
	}
}

func registerGauge(v Gauge) {
	if !Enabled {
		return
	}
	if g, ok := v.(prometheus.Gauge); ok {
		if err := registry.Register(g); err != nil {
			// Already registered or other error - ignore
		}
	}
}

func registerCounter(v Counter) {
	if !Enabled {
		return
	}
	if c, ok := v.(prometheus.Counter); ok {
		if err := registry.Register(c); err != nil {
			// Already registered or other error - ignore
		}
	}
}

func registerGaugeFunc(v GaugeFunc) {
	if !Enabled || v == nil {
		return
	}
	if err := registry.Register(v); err != nil {
		// Already registered or other error - ignore
	}
}

func initRegistry() {
	registry = prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	registerCounterVec(RequestsTotal)
	registerHistogramVec(RequestDurationSeconds)
	registerCounterVec(RequestErrorsTotal)
	registerCounterVec(ShortCircuitsTotal)

	registerCounterVec(PolicyExecutionsTotal)
	registerHistogramVec(PolicyDurationSeconds)
	registerCounterVec(PolicySkippedTotal)
	registerGaugeVec(PoliciesPerChain)

	registerGaugeVec(PolicyChainsLoaded)
	registerCounterVec(XDSUpdatesTotal)
	registerGaugeVec(XDSConnectionState)
	registerGaugeVec(SnapshotSize)

	registerGauge(ActiveStreams)
	registerCounterVec(BodyBytesProcessed)
	registerHistogramVec(ContextBuildDurationSeconds)

	registerGauge(Up)
	registerGaugeVec(GRPCConnectionsActive)
	registerGaugeFunc(Goroutines)
	registerGaugeVec(MemoryBytes)

	registerCounterVec(PolicyErrorsTotal)
	registerCounterVec(StreamErrorsTotal)
	registerCounter(RouteLookupFailuresTotal)
	registerCounterVec(PanicRecoveriesTotal)

	Up.Set(1)
}

// Init initializes the metrics registry with all collectors.
// This must be called after SetEnabled() has been called.
func Init() *prometheus.Registry {
	once.Do(func() {
		// Initialize all metric variables first
		initMetrics()

		if !Enabled {
			registry = prometheus.NewRegistry()
			return
		}
		initRegistry()
	})

	return registry
}

// GetRegistry returns the prometheus registry
func GetRegistry() *prometheus.Registry {
	if registry == nil {
		return Init()
	}
	return registry
}

// UpdateMemoryMetrics updates memory-related metrics
func UpdateMemoryMetrics() {
	if !Enabled {
		return
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	MemoryBytes.WithLabelValues("heap_alloc").Set(float64(m.HeapAlloc))
	MemoryBytes.WithLabelValues("heap_sys").Set(float64(m.HeapSys))
	MemoryBytes.WithLabelValues("stack").Set(float64(m.StackInuse))
}

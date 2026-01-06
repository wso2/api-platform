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
	namespace = "gateway_controller"
)

var (
	once     sync.Once
	registry *prometheus.Registry

	APIOperationsTotal          CounterVec
	APIOperationDurationSeconds HistogramVec
	APIsTotal                   GaugeVec
	ValidationErrorsTotal       CounterVec
	DeploymentLatencySeconds    Histogram

	SnapshotGenerationDurationSeconds HistogramVec
	SnapshotGenerationTotal           CounterVec
	SnapshotSize                      GaugeVec
	PolicyEngineSnapshotSize          GaugeVec
	TranslationErrorsTotal            CounterVec
	RoutesPerAPI                      Histogram

	XDSClientsConnected      GaugeVec
	XDSStreamRequestsTotal   CounterVec
	XDSSnapshotAckTotal      CounterVec
	XDSStreamDurationSeconds HistogramVec

	DatabaseOperationsTotal          CounterVec
	DatabaseOperationDurationSeconds HistogramVec
	DatabaseSizeBytes                GaugeVec
	ConfigStoreSize                  GaugeVec
	StorageErrorsTotal               CounterVec

	CertificatesTotal          GaugeVec
	CertificateOperationsTotal CounterVec
	CertificateExpirySeconds   GaugeVec
	SDSUpdatesTotal            CounterVec

	PoliciesTotal               GaugeVec
	PolicyChainLength           HistogramVec
	PolicySnapshotUpdatesTotal  CounterVec
	PolicyValidationErrorsTotal CounterVec

	ControlPlaneConnectionState       GaugeVec
	ControlPlaneReconnectionsTotal    Counter
	ControlPlaneEventsSentTotal       CounterVec
	ControlPlaneMessageLatencySeconds Histogram

	HTTPRequestsTotal          CounterVec
	HTTPRequestDurationSeconds HistogramVec
	HTTPRequestSizeBytes       HistogramVec
	HTTPResponseSizeBytes      HistogramVec
	ConcurrentRequests         Gauge

	LLMProvidersTotal         GaugeVec
	LLMProviderTemplatesTotal Gauge
	MCPProxiesTotal           GaugeVec
	LLMOperationsTotal        CounterVec

	Up                     Gauge
	Info                   GaugeVec
	Goroutines             GaugeFunc
	MemoryBytes            GaugeVec
	SnapshotCacheSizeBytes Gauge
	ConfigReloadTimestamp  Gauge

	ValidationDurationSeconds HistogramVec
	ErrorsTotal               CounterVec
	PanicRecoveriesTotal      CounterVec
)

// initMetrics initializes all metric variables.
// This must be called after SetEnabled() to ensure proper noop behavior when disabled.
func initMetrics() {
	APIOperationsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "api_operations_total",
			Help:      "Total number of API operations",
		},
		[]string{"operation", "status", "api_type"},
	)

	APIOperationDurationSeconds = newHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "api_operation_duration_seconds",
			Help:      "Duration of API operations in seconds",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		},
		[]string{"operation", "api_type"},
	)

	APIsTotal = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "apis_total",
			Help:      "Total number of deployed APIs",
		},
		[]string{"api_type", "status"},
	)

	ValidationErrorsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "validation_errors_total",
			Help:      "Total number of validation errors",
		},
		[]string{"operation", "error_type"},
	)

	DeploymentLatencySeconds = newHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "deployment_latency_seconds",
			Help:      "End-to-end deployment latency in seconds",
			Buckets:   []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0},
		},
	)

	SnapshotGenerationDurationSeconds = newHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "snapshot_generation_duration_seconds",
			Help:      "Duration of snapshot generation in seconds",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"type"},
	)

	SnapshotGenerationTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "snapshot_generation_total",
			Help:      "Total number of snapshot generations",
		},
		[]string{"type", "status", "trigger"},
	)

	SnapshotSize = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "snapshot_size",
			Help:      "Size of xDS snapshot resources",
		},
		[]string{"resource_type"},
	)

	PolicyEngineSnapshotSize = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "policy_engine_snapshot_size",
			Help:      "Size of xDS snapshot resources sent to policy engine",
		},
		[]string{"resource_type"},
	)

	TranslationErrorsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "translation_errors_total",
			Help:      "Total number of configuration translation errors",
		},
		[]string{"error_type"},
	)

	RoutesPerAPI = newHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "routes_per_api",
			Help:      "Number of routes per API",
			Buckets:   []float64{1, 5, 10, 25, 50, 100, 250},
		},
	)

	XDSClientsConnected = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "xds_clients_connected",
			Help:      "Number of connected xDS clients",
		},
		[]string{"server", "node_id"},
	)

	XDSStreamRequestsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "xds_stream_requests_total",
			Help:      "Total number of xDS stream requests",
		},
		[]string{"server", "type_url", "operation"},
	)

	XDSSnapshotAckTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "xds_snapshot_ack_total",
			Help:      "Total number of xDS snapshot ACK/NACK",
		},
		[]string{"server", "node_id", "status"},
	)

	XDSStreamDurationSeconds = newHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "xds_stream_duration_seconds",
			Help:      "Duration of xDS streams in seconds",
			Buckets:   []float64{1, 5, 30, 60, 300, 600, 1800, 3600},
		},
		[]string{"server", "node_id"},
	)

	DatabaseOperationsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "database_operations_total",
			Help:      "Total number of database operations",
		},
		[]string{"operation", "table", "status"},
	)

	DatabaseOperationDurationSeconds = newHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "database_operation_duration_seconds",
			Help:      "Duration of database operations in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		},
		[]string{"operation", "table"},
	)

	DatabaseSizeBytes = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "database_size_bytes",
			Help:      "Size of database in bytes",
		},
		[]string{"database"},
	)

	ConfigStoreSize = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "config_store_size",
			Help:      "Number of items in config store",
		},
		[]string{"type"},
	)

	StorageErrorsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "storage_errors_total",
			Help:      "Total number of storage errors",
		},
		[]string{"operation", "error_type"},
	)

	CertificatesTotal = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "certificates_total",
			Help:      "Total number of certificates",
		},
		[]string{"type"},
	)

	CertificateOperationsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "certificate_operations_total",
			Help:      "Total number of certificate operations",
		},
		[]string{"operation", "status"},
	)

	CertificateExpirySeconds = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "certificate_expiry_seconds",
			Help:      "Certificate expiry time in seconds since epoch",
		},
		[]string{"cert_id", "cert_name"},
	)

	SDSUpdatesTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "sds_updates_total",
			Help:      "Total number of SDS updates",
		},
		[]string{"status"},
	)

	PoliciesTotal = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "policies_total",
			Help:      "Total number of policies",
		},
		[]string{"api_id", "route"},
	)

	PolicyChainLength = newHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "policy_chain_length",
			Help:      "Length of policy chains",
			Buckets:   []float64{0, 1, 2, 5, 10, 20, 50},
		},
		[]string{"api_id", "route"},
	)

	PolicySnapshotUpdatesTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "policy_snapshot_updates_total",
			Help:      "Total number of policy snapshot updates",
		},
		[]string{"status"},
	)

	PolicyValidationErrorsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "policy_validation_errors_total",
			Help:      "Total number of policy validation errors",
		},
		[]string{"error_type"},
	)

	ControlPlaneConnectionState = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "control_plane_connection_state",
			Help:      "Control plane connection state (1=connected, 0=disconnected)",
		},
		[]string{"state"},
	)

	ControlPlaneReconnectionsTotal = newCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "control_plane_reconnections_total",
			Help:      "Total number of control plane reconnections",
		},
	)

	ControlPlaneEventsSentTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "control_plane_events_sent_total",
			Help:      "Total number of control plane events sent",
		},
		[]string{"event_type", "status"},
	)

	ControlPlaneMessageLatencySeconds = newHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "control_plane_message_latency_seconds",
			Help:      "Control plane message latency in seconds",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		},
	)

	HTTPRequestsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	HTTPRequestDurationSeconds = newHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		},
		[]string{"method", "endpoint"},
	)

	HTTPRequestSizeBytes = newHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_size_bytes",
			Help:      "HTTP request size in bytes",
			Buckets:   []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"endpoint"},
	)

	HTTPResponseSizeBytes = newHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"endpoint"},
	)

	ConcurrentRequests = newGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "concurrent_requests",
			Help:      "Number of concurrent HTTP requests",
		},
	)

	LLMProvidersTotal = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "llm_providers_total",
			Help:      "Total number of LLM providers",
		},
		[]string{"status"},
	)

	LLMProviderTemplatesTotal = newGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "llm_provider_templates_total",
			Help:      "Total number of LLM provider templates",
		},
	)

	MCPProxiesTotal = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "mcp_proxies_total",
			Help:      "Total number of MCP proxies",
		},
		[]string{"type", "status"},
	)

	LLMOperationsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "llm_operations_total",
			Help:      "Total number of LLM operations",
		},
		[]string{"operation", "status"},
	)

	Up = newGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Gateway controller liveness indicator (1=up, 0=down)",
		},
	)

	Info = newGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "info",
			Help:      "Gateway controller build information",
		},
		[]string{"version", "storage_type", "build_date"},
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

	SnapshotCacheSizeBytes = newGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "snapshot_cache_size_bytes",
			Help:      "Size of xDS snapshot cache in bytes",
		},
	)

	ConfigReloadTimestamp = newGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "config_reload_timestamp",
			Help:      "Timestamp of last configuration reload",
		},
	)

	ValidationDurationSeconds = newHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "validation_duration_seconds",
			Help:      "Duration of validation operations in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1},
		},
		[]string{"validator_type"},
	)

	ErrorsTotal = newCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "errors_total",
			Help:      "Total number of errors by component",
		},
		[]string{"component", "error_type"},
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

func registerHistogram(v Histogram) {
	if !Enabled {
		return
	}
	if h, ok := v.(prometheus.Histogram); ok {
		if err := registry.Register(h); err != nil {
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

	registerCounterVec(APIOperationsTotal)
	registerHistogramVec(APIOperationDurationSeconds)
	registerGaugeVec(APIsTotal)
	registerCounterVec(ValidationErrorsTotal)
	registerHistogram(DeploymentLatencySeconds)

	registerHistogramVec(SnapshotGenerationDurationSeconds)
	registerCounterVec(SnapshotGenerationTotal)
	registerGaugeVec(SnapshotSize)
	registerGaugeVec(PolicyEngineSnapshotSize)
	registerCounterVec(TranslationErrorsTotal)
	registerHistogram(RoutesPerAPI)

	registerGaugeVec(XDSClientsConnected)
	registerCounterVec(XDSStreamRequestsTotal)
	registerCounterVec(XDSSnapshotAckTotal)
	registerHistogramVec(XDSStreamDurationSeconds)

	registerCounterVec(DatabaseOperationsTotal)
	registerHistogramVec(DatabaseOperationDurationSeconds)
	registerGaugeVec(DatabaseSizeBytes)
	registerGaugeVec(ConfigStoreSize)
	registerCounterVec(StorageErrorsTotal)

	registerGaugeVec(CertificatesTotal)
	registerCounterVec(CertificateOperationsTotal)
	registerGaugeVec(CertificateExpirySeconds)
	registerCounterVec(SDSUpdatesTotal)

	registerGaugeVec(PoliciesTotal)
	registerHistogramVec(PolicyChainLength)
	registerCounterVec(PolicySnapshotUpdatesTotal)
	registerCounterVec(PolicyValidationErrorsTotal)

	registerGaugeVec(ControlPlaneConnectionState)
	registerCounter(ControlPlaneReconnectionsTotal)
	registerCounterVec(ControlPlaneEventsSentTotal)
	registerHistogram(ControlPlaneMessageLatencySeconds)

	registerCounterVec(HTTPRequestsTotal)
	registerHistogramVec(HTTPRequestDurationSeconds)
	registerHistogramVec(HTTPRequestSizeBytes)
	registerHistogramVec(HTTPResponseSizeBytes)
	registerGauge(ConcurrentRequests)

	registerGaugeVec(LLMProvidersTotal)
	registerGauge(LLMProviderTemplatesTotal)
	registerGaugeVec(MCPProxiesTotal)
	registerCounterVec(LLMOperationsTotal)

	registerGauge(Up)
	registerGaugeVec(Info)
	registerGaugeFunc(Goroutines)
	registerGaugeVec(MemoryBytes)
	registerGauge(SnapshotCacheSizeBytes)
	registerGauge(ConfigReloadTimestamp)

	registerHistogramVec(ValidationDurationSeconds)
	registerCounterVec(ErrorsTotal)
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
	MemoryBytes.WithLabelValues("stack_inuse").Set(float64(m.StackInuse))
}

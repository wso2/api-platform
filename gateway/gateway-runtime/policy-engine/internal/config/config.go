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

package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	toml "github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/wso2/api-platform/common/collector"
	"github.com/wso2/api-platform/common/configinterpolate"
)

const (
	// DefaultMaxDecompressedBytes is the default cap on decompressed bytes buffered
	// from a Content-Encoded body — the whole body when buffered, each chunk when
	// streaming. Applied when max_decompressed_bytes is unset for a direction.
	DefaultMaxDecompressedBytes int64 = 10 * 1024 * 1024 // 10 MiB

	// ExtProcMessageOverheadBytes is the headroom an ext_proc message needs above the
	// body it carries: request/response headers, Envoy attributes, dynamic metadata and
	// protobuf framing all travel in the same message. The gRPC message limits are
	// validated against the body ceilings plus this, because a limit sized to the body
	// alone fails mid-request with ResourceExhausted on any request whose headers are
	// large — a failure that looks like a gateway fault rather than a misconfiguration.
	ExtProcMessageOverheadBytes int64 = 1 * 1024 * 1024 // 1 MiB

	// maxConfigurableDecompressedBytes is the largest body ceiling that can still have
	// ExtProcMessageOverheadBytes added to it without overflowing int64. Validate rejects
	// anything above it, which is what lets RequiredExtProcMessageBytes add without
	// checking. Far beyond any real deployment — the point is that the arithmetic is
	// total, not that the number is reachable.
	maxConfigurableDecompressedBytes int64 = math.MaxInt64 - ExtProcMessageOverheadBytes

	// DefaultMaxConcurrentStreams bounds in-flight ext_proc calls on the Envoy
	// connection, one stream per request being processed. gRPC's own default is
	// effectively unlimited, so an explicit value is what makes the stream budget a
	// bounded resource rather than whatever the peer asks for.
	//
	// Deliberately generous. This is not a load-shedding control: Envoy does not
	// degrade gracefully when it runs out of streams, it stalls, so a value below a
	// pod's peak concurrent in-flight requests costs availability rather than
	// protecting anything. Raise it if a single runtime instance legitimately carries
	// more concurrency than this.
	DefaultMaxConcurrentStreams uint32 = 10000
)

// defaultFileSourceAllowlist is the policy-engine's default set of directories that
// a {{ file "..." }} config-interpolation token may read from. It uses the
// operator-visible container name (gateway-runtime), and can be overridden via the
// shared APIP_CONFIG_FILE_SOURCE_ALLOWLIST env var
// (see configinterpolate.ResolveAllowlist).
var defaultFileSourceAllowlist = []string{
	"/etc/gateway-runtime",
	"/secrets/gateway-runtime",
}

type Config struct {
	PolicyEngine         PolicyEngine           `koanf:"policy_engine"`
	GatewayController    map[string]interface{} `koanf:"gateway_controller"`
	PolicyConfigurations map[string]interface{} `koanf:"policy_configurations"`
	Collector            CollectorConfig        `koanf:"collector"`
	Analytics            AnalyticsConfig        `koanf:"analytics"`
	TrafficLogging       TrafficLoggingConfig   `koanf:"traffic_logging"`
	TracingConfig        TracingConfig          `koanf:"tracing"`
}

// CollectorConfig holds the data-collection ("collector") configuration. The
// collector is the shared capture pipeline that gathers request/response headers
// and bodies and ships them to the policy-engine over ALS. It underpins every
// consumer of that data (analytics and traffic logging) and is implicitly active
// whenever a consumer is enabled — see Config.IsCollectorEnabled. This section
// tunes capture and transport; it has no on/off flag of its own.
type CollectorConfig struct {
	// RequestBody / ResponseBody attach captured request/response bodies
	// onto the collected event.
	RequestBody  bool `koanf:"request_body"`
	ResponseBody bool `koanf:"response_body"`
	// Server tunes the policy-engine ALS receiver (the gRPC server that ingests
	// collected access logs). It is part of the collector transport and is
	// configured under the shared [collector.server] section (the controller
	// reads the same section to configure Envoy's sender side).
	Server AccessLogsServiceConfig `koanf:"server"`
}

// AnalyticsConfig holds analytics configuration
type AnalyticsConfig struct {
	Enabled            bool                      `koanf:"enabled"`
	EnabledPublishers  []string                  `koanf:"enabled_publishers"`
	Publishers         AnalyticsPublishersConfig `koanf:"publishers"`
	GRPCEventServerCfg map[string]interface{}    `koanf:"grpc_event_server"`
	// AccessLogsServiceCfg is a deprecated alias. ALS receiver tuning moved to
	// [collector.server]; when set here it is migrated onto the collector during
	// validation (with a warning). Prefer [collector.server].
	AccessLogsServiceCfg AccessLogsServiceConfig `koanf:"access_logs_service"`
	// AllowPayloads, SendRequestBody and SendResponseBody are deprecated aliases.
	// Body capture now lives under [collector]. When set, these are mapped onto
	// collector.request_body / collector.response_body during validation
	// (with a warning). Prefer the [collector] fields directly.
	AllowPayloads    bool `koanf:"allow_payloads"`
	SendRequestBody  bool `koanf:"send_request_body"`
	SendResponseBody bool `koanf:"send_response_body"`
}

// AnalyticsPublishersConfig holds configuration for all analytics publishers
type AnalyticsPublishersConfig struct {
	Moesif MoesifPublisherConfig `koanf:"moesif"`
}

// Traffic-log sink names accepted in traffic_logging.outputs.
const (
	// TrafficLogSinkStdout writes each line to the process's stdout. This is the
	// default and the historical behavior.
	TrafficLogSinkStdout = "stdout"
	// TrafficLogSinkFile appends each line to a local file, rotating at a size
	// ceiling. Keeps request/response bodies out of the container log.
	TrafficLogSinkFile = "file"
	// TrafficLogSinkHTTP batches lines and POSTs them to an operator-named
	// endpoint. Requires no co-located log collector.
	TrafficLogSinkHTTP = "http"
)

// Traffic-log HTTP auth types accepted in traffic_logging.http.auth.type.
const (
	// TrafficLogAuthNone sends no authentication material.
	TrafficLogAuthNone = "none"
	// TrafficLogAuthBearer sends "Authorization: Bearer <token>".
	TrafficLogAuthBearer = "bearer"
	// TrafficLogAuthBasic sends "Authorization: Basic <base64(user:pass)>".
	TrafficLogAuthBasic = "basic"
	// TrafficLogAuthHeader sends an arbitrary header. Required for receivers
	// whose scheme is not Bearer — Splunk HEC uses "Authorization: Splunk <token>".
	TrafficLogAuthHeader = "header"
)

// Behavior when the HTTP sink's queue is full (traffic_logging.http.on_queue_full).
const (
	// TrafficLogQueueDropNew discards the incoming line, preserving older ones.
	TrafficLogQueueDropNew = "drop_new"
	// TrafficLogQueueDropOldest evicts the oldest queued line to make room.
	TrafficLogQueueDropOldest = "drop_oldest"
)

// TrafficLoggingConfig holds configuration for the traffic-logging feature, which
// writes each collected event as a JSON line to one or more sinks. It is a consumer
// of the collector; enabling it implicitly activates the collector. There is a single
// mode: when Enabled, a line is emitted for every request to every API, with no
// policy required — including requests denied by an auth policy short-circuit.
//
// The line contains request/response bodies when the corresponding capture flags are
// on, so the choice of sink is a data-protection decision, not just a routing one:
// the default stdout sink puts those bodies in the container log, and therefore on
// the node's disk and into any node-level log collector.
type TrafficLoggingConfig struct {
	// Enabled turns JSON traffic logging on.
	Enabled bool `koanf:"enabled"`
	// Outputs names the sinks each line is written to. Valid entries are
	// "stdout", "file" and "http" (see the TrafficLogSink* constants); order is
	// irrelevant and duplicates are rejected. Unset or empty resolves to
	// ["stdout"] with a warning, preserving the historical behavior exactly; an
	// unknown name is rejected at startup. Only a typo can be mistaken for an
	// intent that was not honoured, so only a typo fails — an empty list
	// expresses no intent to keep bodies out of the container log.
	Outputs []string `koanf:"outputs"`
	// File configures the "file" sink. Only read when Outputs contains "file".
	File TrafficLogFileConfig `koanf:"file"`
	// HTTP configures the "http" sink. Only read when Outputs contains "http".
	HTTP TrafficLogHTTPConfig `koanf:"http"`
	// ShutdownTimeout bounds the flush of buffering sinks on SIGTERM. Only the
	// HTTP sink buffers; the stdout and file sinks write straight to their fd and
	// have nothing to flush.
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
	// MaskedHeaders lists header names (case-insensitive) whose values are
	// redacted in the logged requestHeaders/responseHeaders.
	MaskedHeaders []string `koanf:"masked_headers"`
	// MaxPayloadSize caps the number of bytes of request/response payload written
	// to the log line (0 = no limit). Truncation is applied at the publisher, so
	// the collector still captures the full body and other consumers (e.g. Moesif)
	// are unaffected.
	MaxPayloadSize int `koanf:"max_payload_size"`
	// RequestHeaders / RequestBody / ResponseHeaders / ResponseBody select which
	// captured fields are attached to the log line. Each is a no-op if the
	// corresponding [collector] capture flag is off — this directive can only
	// select among what the collector already captured.
	RequestHeaders  bool `koanf:"request_headers"`
	RequestBody     bool `koanf:"request_body"`
	ResponseHeaders bool `koanf:"response_headers"`
	ResponseBody    bool `koanf:"response_body"`
	// ExcludeFields drops the named fields from the emitted line and keeps
	// everything else, layered on top of the flow selection above. Names are
	// top-level keys (e.g. "latencies", "requestHeaders") or dotted paths of
	// arbitrary depth into nested JSON objects (e.g. "requestHeaders.authorization",
	// or "properties.claims.internal_debug" to reach into a nested object produced
	// by a Properties $ctx: expression that evaluates to a CEL map wholesale).
	ExcludeFields []string `koanf:"exclude_fields"`
	// Properties adds extra key->value pairs to the emitted line's top-level
	// "properties" object. A value prefixed "$ctx:" is evaluated as a CEL
	// expression against a request-context surface built from the collected
	// event (see publishers.globalPropertyEvaluator), including a real auth.*
	// namespace backed by analytics metadata the collector system policy stamps
	// generically for any authenticated request; other values are emitted as
	// literal strings.
	Properties map[string]string `koanf:"properties"`
}

// TrafficLogFileConfig configures the "file" traffic-log sink
// ([traffic_logging.file]).
//
// The file holds request/response bodies, so it is created 0600 inside a 0700
// directory, with both modes established at creation time rather than chmod'd
// afterwards. There is deliberately no file_mode knob: 0600 is the only defensible
// value for this content, and making it configurable only creates a way to get it
// wrong.
type TrafficLogFileConfig struct {
	// Path is the absolute path of the live log file. Required when the "file"
	// sink is selected. A relative path is rejected rather than resolved against
	// the process's working directory, which is almost never what was meant.
	Path string `koanf:"path"`
	// MaxSizeMB is the size at which the live file is rotated. Rotation renames
	// the live file to <path>.1 (clobbering any previous backup) and reopens, so
	// the worst-case on-disk total is 2 x MaxSizeMB.
	//
	// 0 disables rotation entirely. That is permitted — an operator mounting a
	// dedicated, sized volume may want the volume to be the bound — but it is not
	// the default and it logs a warning at startup, because stdout is bounded by
	// the kubelet today and an unrotated file is not bounded by anything.
	MaxSizeMB int `koanf:"max_size_mb"`
}

// TrafficLogHTTPConfig configures the "http" traffic-log sink
// ([traffic_logging.http]): lines are queued, batched, and POSTed to an
// operator-named endpoint. This removes the need for any co-located log collector
// (sidecar or node-level agent) and keeps the bodies off the node's disk entirely.
//
// The endpoint is operator configuration, at the same trust tier as the rest of
// config.toml, so the dial-time private-IP guard that ssrf-prevention.md requires
// for request-derived URLs does not apply here — this is a destination the operator
// chose deliberately, and internal ones are the normal case.
type TrafficLogHTTPConfig struct {
	// Endpoint is the absolute URL each batch is POSTed to. Required when the
	// "http" sink is selected. Must be https:// unless AllowInsecureTransport is
	// explicitly set.
	Endpoint string `koanf:"endpoint"`
	// ContentType is the Content-Type header sent with each batch. Defaults to
	// application/x-ndjson, which matches the newline-delimited body this sink
	// produces and is accepted by Splunk HEC's /raw endpoint, Elasticsearch and
	// OpenSearch _bulk, Fluent Bit's http input, and the OTel collector.
	ContentType string `koanf:"content_type"`
	// AllowInsecureTransport permits a plaintext http:// endpoint. Off by default
	// and intended only for a local collector on the pod network; the traffic log
	// carries request/response bodies, so plaintext is a real disclosure.
	AllowInsecureTransport bool `koanf:"allow_insecure_transport"`

	// BatchMaxEvents / BatchMaxBytes / FlushInterval close a batch on whichever
	// bound is reached first.
	BatchMaxEvents int           `koanf:"batch_max_events"`
	BatchMaxBytes  int           `koanf:"batch_max_bytes"`
	FlushInterval  time.Duration `koanf:"flush_interval"`

	// QueueCapacity bounds the in-memory queue between the access-log ingest path
	// and the sending goroutine. It must be bounded: an unbounded queue in front
	// of a bounded sender is just deferred unbounded memory growth, and these
	// events carry bodies, so the growth is fast.
	QueueCapacity int `koanf:"queue_capacity"`
	// OnQueueFull selects what happens when the queue is full — "drop_new"
	// (default, preserves older lines) or "drop_oldest" (preserves recency).
	// Either way a line is dropped and counted; the sink never blocks the caller
	// and never falls back to stdout.
	OnQueueFull string `koanf:"on_queue_full"`

	// RequestTimeout bounds a single POST attempt.
	RequestTimeout time.Duration `koanf:"request_timeout"`
	// MaxRetries is the number of retry attempts after the initial one. Retries
	// apply to transport errors, 5xx and 429 only; a 4xx means the receiver
	// rejected the batch's shape and retrying would just amplify it.
	MaxRetries int `koanf:"max_retries"`
	// RetryBackoff is the base delay for exponential backoff. Jitter is applied
	// per attempt so replicas retrying after a shared outage do not synchronize.
	RetryBackoff time.Duration `koanf:"retry_backoff"`
	// RetryAbortQueueRatio is the fraction of QueueCapacity at which a retrying
	// batch abandons its remaining attempts so the single sender can resume
	// draining.
	//
	// There is one sender and it retries synchronously, so nothing drains the
	// queue while a batch waits. Retrying is free while the queue is shallow and
	// expensive once it is filling, which is exactly what this ratio expresses.
	//
	// The value is used literally, never remapped: 0 means retries are always
	// abandoned (one attempt per batch), and 1 means never abort early — the
	// behaviour before this existed, where a hung receiver holds the sender for
	// the whole retry budget. Omitting the key leaves the shipped 0.5 default,
	// which is distinct from writing 0. See EffectiveRetryAbortDepth.
	RetryAbortQueueRatio float64 `koanf:"retry_abort_queue_ratio"`

	// Auth configures per-request authentication material.
	Auth TrafficLogHTTPAuthConfig `koanf:"auth"`
	// TLS configures the transport's trust and client-certificate material.
	TLS TrafficLogHTTPTLSConfig `koanf:"tls"`
}

// TrafficLogHTTPAuthConfig configures authentication for the HTTP sink
// ([traffic_logging.http.auth]).
//
// Secret values should be supplied via the config interpolation tokens rather than
// inlined, e.g. token = '{{ file "/secrets/gateway-runtime/hec-token" }}' or
// '{{ env "TRAFFIC_LOG_TOKEN" }}'. Nothing in this struct is ever logged, including
// on a transport error.
// Type selects the scheme and each scheme's fields live in its own sub-table, the
// same discriminator-plus-named-section shape [analytics] uses for
// enabled_publishers / [analytics.publishers.moesif]. Keeping the fields separated
// means a field can never be read under a type it does not belong to, and adding a
// scheme later touches only its own struct.
type TrafficLogHTTPAuthConfig struct {
	// Type selects the scheme: "none" (default), "bearer", "basic" or "header".
	// Only the matching sub-table below is read; the others are ignored entirely.
	Type string `koanf:"type"`

	// Bearer is read only when Type is "bearer" ([traffic_logging.http.auth.bearer]).
	Bearer TrafficLogHTTPAuthBearerConfig `koanf:"bearer"`
	// Basic is read only when Type is "basic" ([traffic_logging.http.auth.basic]).
	Basic TrafficLogHTTPAuthBasicConfig `koanf:"basic"`
	// Header is read only when Type is "header" ([traffic_logging.http.auth.header]).
	Header TrafficLogHTTPAuthHeaderConfig `koanf:"header"`
}

// TrafficLogHTTPAuthBearerConfig configures the "bearer" scheme
// ([traffic_logging.http.auth.bearer]), sending "Authorization: Bearer <token>".
type TrafficLogHTTPAuthBearerConfig struct {
	// Token is required when the "bearer" type is selected.
	Token string `koanf:"token"`
}

// TrafficLogHTTPAuthBasicConfig configures the "basic" scheme
// ([traffic_logging.http.auth.basic]), sending
// "Authorization: Basic <base64(username:password)>".
type TrafficLogHTTPAuthBasicConfig struct {
	// Username and Password are both required when the "basic" type is selected.
	Username string `koanf:"username"`
	Password string `koanf:"password"`
}

// TrafficLogHTTPAuthHeaderConfig configures the "header" scheme
// ([traffic_logging.http.auth.header]), sending one literal header verbatim.
//
// This exists because not every receiver uses the Bearer scheme: Splunk HEC
// expects "Authorization: Splunk <token>", which "bearer" cannot express. It also
// covers receivers authenticating on a non-Authorization header entirely.
type TrafficLogHTTPAuthHeaderConfig struct {
	// Name and Value are both required when the "header" type is selected.
	Name  string `koanf:"name"`
	Value string `koanf:"value"`
}

// TrafficLogHTTPTLSConfig configures TLS for the HTTP sink
// ([traffic_logging.http.tls]).
type TrafficLogHTTPTLSConfig struct {
	// CAFile is a PEM bundle used to verify the receiver's certificate. Empty
	// means the system trust store, which is correct for a public SaaS receiver
	// and usually wrong for an internal collector with a private CA.
	CAFile string `koanf:"ca_file"`
	// CertFile / KeyFile enable mTLS. Both must be set, or neither.
	CertFile string `koanf:"cert_file"`
	KeyFile  string `koanf:"key_file"`
	// InsecureSkipVerify disables receiver certificate verification. Off by
	// default; when on, startup logs a warning naming the endpoint, because this
	// exposes request/response bodies to anyone who can intercept the connection.
	InsecureSkipVerify bool `koanf:"insecure_skip_verify"`
}

// MoesifPublisherConfig holds Moesif-specific configuration
type MoesifPublisherConfig struct {
	ApplicationID      string `koanf:"application_id"`
	BaseURL            string `koanf:"moesif_base_url"`
	PublishInterval    int    `koanf:"publish_interval"`
	EventQueueSize     int    `koanf:"event_queue_size"`
	BatchSize          int    `koanf:"batch_size"`
	TimerWakeupSeconds int    `koanf:"timer_wakeup_seconds"`
}

// Config represents the complete policy engine configuration
type PolicyEngine struct {
	Server         ServerConfig         `koanf:"server"`
	Admin          AdminConfig          `koanf:"admin"`
	Metrics        MetricsConfig        `koanf:"metrics"`
	ConfigMode     ConfigModeConfig     `koanf:"config_mode"`
	XDS            XDSConfig            `koanf:"xds"`
	FileConfig     FileConfigConfig     `koanf:"file_config"`
	Logging        LoggingConfig        `koanf:"logging"`
	PythonExecutor PythonExecutorConfig `koanf:"python_executor"`
	// HTTPClient configures the single shared outbound *http.Client built once at
	// startup (see cmd/policy-engine/main.go) and injected into every policy
	// instance via PolicyMetadata.SharedHTTPClient — see HTTPClientConfig's doc
	// comment for the full rationale.
	HTTPClient HTTPClientConfig `koanf:"http_client"`
	// Tracing holds OpenTelemetry exporter configuration
	TracingServiceName string `koanf:"tracing_service_name"`

	// RequestBody and ResponseBody hold body-processing limits per direction, so
	// request bodies can be bounded differently from response bodies.
	RequestBody  BodyConfig `koanf:"request_body"`
	ResponseBody BodyConfig `koanf:"response_body"`

	// RawConfig holds the complete raw configuration map including custom fields
	// This is used for resolving ${config} CEL expressions in policy systemParameters
	// Note: No struct tag - populated manually via k.Raw()
	RawConfig map[string]interface{}
}

// BodyConfig holds body-processing limits for one direction
// ([policy_engine.request_body] or [policy_engine.response_body]).
type BodyConfig struct {
	// MaxDecompressedBytes caps decompressed bytes buffered per body (buffered mode)
	// or per chunk (streaming) — not cumulative, so long-lived streams such as SSE
	// are unaffected. Defaults to DefaultMaxDecompressedBytes when unset.
	MaxDecompressedBytes int64 `koanf:"max_decompressed_bytes"`
}

// HTTPClientConfig configures the single shared outbound *http.Client that the
// policy-engine builds once at startup and injects into every policy instance
// via PolicyMetadata.SharedHTTPClient (see internal/registry.PolicyRegistry).
// It mirrors github.com/wso2/api-platform/httpkit/httpclient.Config field-for-field (see
// that package's own doc comments for full semantics) so every knob the
// library exposes that has a natural TOML shape is operator-configurable here
// under [policy_engine.http_client], rather than left for each policy to
// reinvent. The few fields httpclient.Config exposes that CANNOT be expressed
// in TOML — Go callback hooks (GetClientCertificate, VerifyPeerCertificate,
// VerifyConnection, ConnectHeader) and a pre-built *x509.CertPool — are not
// represented here.
//
// Timeouts.Overall is only a generous safety-net budget: a policy issuing a
// call with its own tighter per-operation deadline should use
// context.WithTimeout, since http.Client.Do honors a request's context
// deadline independent of the client-level Timeout.
//
// SSRF guarding is off by default (see HTTPClientSSRFConfig) since not every
// policy dials a tenant/user-supplied URL. A policy whose outbound target
// comes from request-derived or tenant-configured data — as opposed to a
// fixed, operator-configured backend — should have its operator enable SSRF
// guarding here; see ssrf-prevention.md.
type HTTPClientConfig struct {
	Pooling  HTTPClientPoolingConfig  `koanf:"pooling"`
	Timeouts HTTPClientTimeoutsConfig `koanf:"timeouts"`
	TLS      HTTPClientTLSConfig      `koanf:"tls"`
	Proxy    HTTPClientProxyConfig    `koanf:"proxy"`
	SSRF     HTTPClientSSRFConfig     `koanf:"ssrf"`
}

// HTTPClientPoolingConfig mirrors httpclient.PoolingConfig.
type HTTPClientPoolingConfig struct {
	MaxIdleConns        int           `koanf:"max_idle_conns"`
	MaxIdleConnsPerHost int           `koanf:"max_idle_conns_per_host"`
	MaxConnsPerHost     int           `koanf:"max_conns_per_host"`
	IdleConnTimeout     time.Duration `koanf:"idle_conn_timeout"`
	KeepAlive           time.Duration `koanf:"keep_alive"`
	DisableKeepAlives   bool          `koanf:"disable_keep_alives"`
	// EnableHTTP2 opts into HTTP/2. See httpclient.PoolingConfig.EnableHTTP2's doc comment
	// on the HTTP/2 connection-coalescing caveat before enabling.
	EnableHTTP2 bool `koanf:"enable_http2"`
}

// HTTPClientTimeoutsConfig mirrors httpclient.TimeoutsConfig.
type HTTPClientTimeoutsConfig struct {
	Overall          time.Duration `koanf:"overall"` // safety-net only; see HTTPClientConfig's doc comment
	Dial             time.Duration `koanf:"dial"`
	TLSHandshake     time.Duration `koanf:"tls_handshake"`
	ResponseHeader   time.Duration `koanf:"response_header"`
	ExpectContinue   time.Duration `koanf:"expect_continue"`
	MaxResponseBytes int64         `koanf:"max_response_bytes"` // 0 = package default (10MiB); negative disables the bound
}

// HTTPClientTLSConfig mirrors the TOML-expressible subset of httpclient.TLSConfig.
type HTTPClientTLSConfig struct {
	MinVersion       string `koanf:"min_version"`       // one of "TLS1_0".."TLS1_3"
	MaxVersion       string `koanf:"max_version"`       // one of "TLS1_0".."TLS1_3"
	CipherSuites     string `koanf:"cipher_suites"`     // comma-separated Go crypto/tls cipher suite names; TLS 1.2 and below only
	CurvePreferences string `koanf:"curve_preferences"` // comma-separated, e.g. "X25519MLKEM768,X25519,P-256"
	RootCAFile       string `koanf:"root_ca_file"`      // PEM CA bundle; empty uses the system root pool
	ClientCertFile   string `koanf:"client_cert_file"`  // mTLS to the origin; both cert and key must be set together
	ClientKeyFile    string `koanf:"client_key_file"`
}

// HTTPClientProxyConfig mirrors the TOML-expressible subset of httpclient.ProxyConfig.
type HTTPClientProxyConfig struct {
	// Mode selects how the proxy is determined: "none" (default), "environment"
	// (HTTP_PROXY/HTTPS_PROXY/NO_PROXY), or "url" (URL/Username/Password/NoProxy below).
	Mode     string   `koanf:"mode"`
	URL      string   `koanf:"url"`
	Username string   `koanf:"username"`
	Password string   `koanf:"password"`
	NoProxy  []string `koanf:"no_proxy"` // exact host, ".suffix", or CIDR entries; only used when mode == "url"

	TLS HTTPClientProxyTLSConfig `koanf:"tls"`

	// Egress states how origin-destination SSRF risk is handled when a forward proxy is
	// also configured: "delegated" (trust the proxy's own egress controls) or
	// "manual_connect" (validate the origin locally before ever issuing CONNECT). Must be
	// set explicitly whenever Mode != "none" and SSRF.Enabled — httpclient.New fails
	// closed at startup otherwise rather than silently choosing one.
	Egress string `koanf:"egress"`
}

// HTTPClientProxyTLSConfig mirrors httpclient.ProxyTLSConfig (the proxy's own TLS
// handshake, fully decoupled from the origin TLS handshake in HTTPClientTLSConfig).
type HTTPClientProxyTLSConfig struct {
	RootCAFile         string `koanf:"root_ca_file"`
	ClientCertFile     string `koanf:"client_cert_file"`
	ClientKeyFile      string `koanf:"client_key_file"`
	InsecureSkipVerify bool   `koanf:"insecure_skip_verify"`
}

// HTTPClientSSRFConfig mirrors the TOML-expressible subset of httpclient.SSRFConfig. Off by
// default — see HTTPClientConfig's doc comment on when a policy's operator should enable it.
type HTTPClientSSRFConfig struct {
	Enabled bool `koanf:"enabled"`
	// Preset selects a built-in netguard policy: "permit_private_block_metadata" (a
	// backend that is normally private — a ClusterIP, a service-DNS name, localhost —
	// stays reachable; only link-local/metadata/unspecified/multicast are refused) or
	// "public_only" (stricter: every private/loopback/link-local/CGNAT address is
	// refused, for a URL expected to point at the public internet). Required when Enabled
	// is true. Custom CIDR allow/deny lists have no natural TOML shape and are not
	// exposed here — use the httpclient/netguard packages directly in code for that.
	Preset         string   `koanf:"preset"`
	MaxRedirects   int      `koanf:"max_redirects"`
	AllowedSchemes []string `koanf:"allowed_schemes"` // empty defaults to {"https"}
}

// MetricsConfig holds Prometheus metrics server configuration
type MetricsConfig struct {
	// Enabled indicates whether the metrics server should be started
	Enabled bool `koanf:"enabled"`

	// Port is the port for the metrics HTTP server
	Port int `koanf:"port"`
}

// TracingConfig holds OpenTelemetry tracing configuration
type TracingConfig struct {
	// Enabled toggles tracing on/off
	Enabled bool `koanf:"enabled"`

	// Endpoint is the OTLP gRPC endpoint (host:port)
	Endpoint string `koanf:"endpoint"`

	// Insecure indicates whether to use an insecure connection (no TLS)
	Insecure bool `koanf:"insecure"`

	// ServiceVersion is the service version reported to the tracing backend
	ServiceVersion string `koanf:"service_version"`

	// BatchTimeout is the export batch timeout
	BatchTimeout time.Duration `koanf:"batch_timeout"`

	// MaxExportBatchSize is the maximum batch size for exports
	MaxExportBatchSize int `koanf:"max_export_batch_size"`

	// SamplingRate is the ratio of requests to sample (0.0 to 1.0)
	// 1.0 = sample all requests, 0.1 = sample 10% of requests
	// If set to 0 or not specified, defaults to 1.0 (sample all)
	SamplingRate float64 `koanf:"sampling_rate"`

	// ResourceAttributes are OpenTelemetry resource attributes attached to every
	// exported span, e.g. {"deployment.environment": "prod"}. The same block is
	// read by the gateway-controller to configure the router's (Envoy's) resource
	// detector, so one setting covers both components. Attributes discovered from
	// the environment (OTEL_RESOURCE_ATTRIBUTES) are still honoured, but these
	// take precedence.
	ResourceAttributes map[string]string `koanf:"resource_attributes"`
}

// ServerConfig holds ext_proc server configuration
type ServerConfig struct {
	// Mode is the connection mode: "uds" (default) or "tcp"
	// In UDS mode, the socket path is a constant (not configurable)
	Mode string `koanf:"mode"`

	// ExtProcPort is the port for the ext_proc gRPC server (TCP mode only)
	ExtProcPort int `koanf:"extproc_port"`

	// MaxRecvMsgBytes and MaxSendMsgBytes bound one ext_proc message in each
	// direction. Both must accommodate the larger of the two body decompression
	// ceilings plus ExtProcMessageOverheadBytes, because both directions carry both
	// kinds of body: the engine receives a request body and may return a mutated one,
	// then receives a response body and may return a mutated one.
	//
	// They exist because gRPC's defaults are not this service's threat model — the
	// receive default is 4 MiB regardless of how the body ceilings are configured, and
	// the send default is unbounded.
	MaxRecvMsgBytes int64 `koanf:"max_recv_msg_bytes"`
	MaxSendMsgBytes int64 `koanf:"max_send_msg_bytes"`

	// MaxConcurrentStreams bounds concurrent in-flight ext_proc calls. See
	// DefaultMaxConcurrentStreams for why this is a generous bound rather than a
	// load-shedding knob.
	MaxConcurrentStreams uint32 `koanf:"max_concurrent_streams"`
}

// RequiredExtProcMessageBytes is the smallest message limit coherent with the
// configured body ceilings. Both directions are sized off the larger ceiling, since
// each carries request and response bodies alike.
//
// The addition cannot overflow: Validate rejects a ceiling above
// maxConfigurableDecompressedBytes before reaching here. That ordering matters — an
// overflowed sum would be *negative*, and a negative requirement compares below every
// configured message limit, so the coherence checks that follow would pass an absurd
// ceiling instead of refusing it.
func (p PolicyEngine) RequiredExtProcMessageBytes() int64 {
	ceiling := p.RequestBody.MaxDecompressedBytes
	if p.ResponseBody.MaxDecompressedBytes > ceiling {
		ceiling = p.ResponseBody.MaxDecompressedBytes
	}
	return ceiling + ExtProcMessageOverheadBytes
}

// PythonExecutorConfig holds configuration for the Python executor bridge.
// The Policy Engine uses this to connect to the Python executor process.
type PythonExecutorConfig struct {
	Server  PythonExecutorServerConfig `koanf:"server"`
	Timeout time.Duration              `koanf:"timeout"`
}

// PythonExecutorServerConfig holds Python executor connection configuration
type PythonExecutorServerConfig struct {
	// Mode is the connection mode: "uds" (default) or "tcp"
	Mode string `koanf:"mode"`

	// Port is the TCP port for the Python executor gRPC server (TCP mode only)
	Port int `koanf:"port"`

	// Host is the TCP host for the Python executor (TCP mode only, default: "localhost")
	Host string `koanf:"host"`

	// Path is the Unix Domain Socket path (UDS mode only)
	Path string `koanf:"path"`
}

// AdminConfig holds admin HTTP server configuration
type AdminConfig struct {
	// Enabled indicates whether the admin server should be started
	Enabled bool `koanf:"enabled"`

	// Port is the port for the admin HTTP server
	Port int `koanf:"port"`

	// AllowedIPs is a list of IP addresses allowed to access the admin API
	// Defaults to localhost only (127.0.0.1 and ::1)
	AllowedIPs []string `koanf:"allowed_ips"`

	// Pprof gates the Go runtime profiling endpoints served on this admin server.
	Pprof PprofConfig `koanf:"pprof"`

	// ConfigDump gates the /config_dump endpoint served on this admin server.
	ConfigDump ConfigDumpConfig `koanf:"config_dump"`
}

// ConfigDumpConfig gates the /config_dump endpoint served on the admin HTTP
// server. Disabled by default — /health and other admin routes are unaffected
// by this flag; when disabled, /config_dump returns 404 rather than a payload.
type ConfigDumpConfig struct {
	Enabled bool `koanf:"enabled"`
}

// PprofConfig gates the Go runtime profiling endpoints (net/http/pprof) served on
// the admin HTTP server. Disabled by default; when disabled the /debug/pprof/*
// routes are not registered at all (they return 404, not 403).
type PprofConfig struct {
	// Enabled registers the /debug/pprof/* handlers on the admin server.
	Enabled bool `koanf:"enabled"`

	// BlockProfileRate is passed to runtime.SetBlockProfileRate (0 = block profiling off).
	BlockProfileRate int `koanf:"block_profile_rate"`

	// MutexProfileFraction is passed to runtime.SetMutexProfileFraction (0 = mutex profiling off).
	MutexProfileFraction int `koanf:"mutex_profile_fraction"`
}

// ConfigModeConfig specifies how policy chains are configured
type ConfigModeConfig struct {
	// Mode can be "file" or "xds"
	Mode string `koanf:"mode"`
}

// XDSConfig holds xDS client configuration
type XDSConfig struct {
	// ConnectTimeout is the timeout for establishing initial connection
	ConnectTimeout time.Duration `koanf:"connect_timeout"`

	// RequestTimeout is the timeout for individual xDS requests
	RequestTimeout time.Duration `koanf:"request_timeout"`

	// InitialReconnectDelay is the initial delay before reconnecting
	InitialReconnectDelay time.Duration `koanf:"initial_reconnect_delay"`

	// MaxReconnectDelay is the maximum delay between reconnection attempts
	MaxReconnectDelay time.Duration `koanf:"max_reconnect_delay"`

	// TLS configuration
	TLS XDSTLSConfig `koanf:"tls"`
}

// XDSTLSConfig holds TLS configuration for xDS connection
type XDSTLSConfig struct {
	// Enabled indicates whether to use TLS
	Enabled bool `koanf:"enabled"`

	// CertPath is the path to the TLS certificate file
	CertPath string `koanf:"cert_path"`

	// KeyPath is the path to the TLS private key file
	KeyPath string `koanf:"key_path"`

	// CAPath is the path to the CA certificate for server verification
	CAPath string `koanf:"ca_path"`
}

// FileConfigConfig holds file-based configuration settings
type FileConfigConfig struct {
	// Path is the path to the policy chains YAML file
	Path string `koanf:"path"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	// Level can be "debug", "info", "warn", "error"
	Level string `koanf:"level"`

	// Format can be "json" or "text"
	Format string `koanf:"format"`
}

// AccessLogsServiceConfig holds access logs service configuration
type AccessLogsServiceConfig struct {
	Mode string `koanf:"mode"` // Connection mode: "uds" (default) or "tcp"
	// ServerPort overrides the fixed ALS listener port (collector.ServerPort, 18090),
	// used only in "tcp" mode. Deprecated: no longer defaulted here or documented in
	// config-template.toml/Helm charts, so new deployments have no way to discover or
	// set it. Kept solely so a config that already sets it explicitly keeps working;
	// leave unset (0) to use the fixed port. Must match the gateway-controller's
	// collector.server.port override, or the two sides will fail to connect.
	ServerPort            int           `koanf:"server_port"`
	ShutdownTimeout       time.Duration `koanf:"shutdown_timeout"`
	PublicKeyPath         string        `koanf:"public_key_path"`
	PrivateKeyPath        string        `koanf:"private_key_path"`
	ALSPlainText          bool          `koanf:"als_plain_text"`
	ExtProcMaxMessageSize int           `koanf:"max_message_size"`
	ExtProcMaxHeaderLimit int           `koanf:"max_header_limit"`
}

// Load loads configuration from one or more files layered over built-in defaults.
// Priority: Config files > Defaults.
//
// Files are merged in the order given with last-wins precedence: a key set in a
// later file overrides the same key from an earlier file. Merge semantics follow
// koanf — nested tables (maps) deep-merge, while list/array values are replaced
// wholesale, not appended. A field may be overridden across files with a different
// representation — e.g. a numeric value in the base and an {{ env }} token (a string)
// in an overlay — and still resolve, because types are only checked after
// interpolation by the weakly-typed unmarshal (a non-coercible value still fails there).
//
// At least one config file path is required; each must exist and parse. The loader
// fails closed rather than silently running on built-in defaults when no file is
// supplied. {{ env }} / {{ file }} interpolation runs once, after all files are
// merged, so a token declared in an earlier file can be resolved by a later overlay.
//
// The configuration supports Go-style duration strings (e.g., "10s", "5m", "1h")
// for all duration fields. The DecodeHook automatically converts string durations
// to time.Duration values before assignment.
func Load(configPaths ...string) (*Config, error) {
	cfg := defaultConfig()

	if len(configPaths) == 0 {
		return nil, fmt.Errorf("at least one config file path is required")
	}

	// Deliberately NOT koanf StrictMerge: strict merging compares the raw parsed
	// types across files, but an {{ env }} / {{ file }} interpolation token is a
	// string until it is resolved after the merge — so strict merging would reject a
	// numeric/bool field that one file sets natively and another overrides with a
	// token. Cross-file type errors are instead caught downstream by the weakly-typed
	// unmarshal and Validate.
	k := koanf.New(".")

	if err := k.Load(confmap.Provider(defaultResolvableConfig(), "."), nil); err != nil {
		return nil, fmt.Errorf("failed to seed resolvable config defaults: %w", err)
	}

	// Load each config file in order. Successive loads deep-merge maps and replace
	// arrays, giving last-wins precedence for keys set in more than one file.
	for _, configPath := range configPaths {
		if err := k.Load(file.Provider(configPath), toml.Parser()); err != nil {
			return nil, fmt.Errorf("failed to load config file %q: %w", configPath, err)
		}
	}

	// Resolve Go template tokens ({{ env }} / {{ file }}) in string leaves of the
	// file-loaded config before unmarshalling; fails closed on a missing required
	// value or a disallowed/oversize file. A token-free config is a no-op. Must run
	// before the RawConfig capture below so policies see resolved values in ${config}
	// expressions.
	k, err := interpolate(k)
	if err != nil {
		return nil, err
	}

	// Unmarshal into pre-populated config struct with defaults
	// Koanf will merge: fields from file/env overwrite defaults, unset fields keep defaults
	if err := k.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{
		DecoderConfig: &mapstructure.DecoderConfig{
			TagName:          "koanf",
			WeaklyTypedInput: true,
			Result:           cfg,
			DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Capture complete raw config for CEL ${config} expression resolution.
	// Uses the interpolated instance so ${config} expressions resolve to the
	// materialized values, not the literal {{ ... }} tokens.
	cfg.PolicyEngine.RawConfig = k.Raw()

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// interpolate resolves Go template tokens ({{ env }} / {{ file }}) in the merged
// config and returns a fresh koanf instance holding the expanded values. It uses a
// new instance (rather than reloading into k) so no un-expanded leaves survive. The
// file-source allowlist is the policy-engine default, overridable via the shared
// APIP_CONFIG_FILE_SOURCE_ALLOWLIST env var. Resolved values are never logged; only
// reference counts are emitted at info level.
func interpolate(k *koanf.Koanf) (*koanf.Koanf, error) {
	opts := configinterpolate.Options{
		FileAllowlist: configinterpolate.ResolveAllowlist(defaultFileSourceAllowlist),
	}
	expanded, stats, err := configinterpolate.Expand(k.Raw(), opts)
	if err != nil {
		return nil, fmt.Errorf("config interpolation failed: %w", err)
	}

	out := koanf.New(".")
	if err := out.Load(confmap.Provider(expanded, "."), nil); err != nil {
		return nil, fmt.Errorf("failed to reload interpolated config: %w", err)
	}
	if stats.Fields > 0 {
		slog.Info("config interpolation complete",
			slog.Int("env_refs", stats.EnvRefs),
			slog.Int("file_refs", stats.FileRefs),
			slog.Int("fields", stats.Fields))
	}
	return out, nil
}

// DefaultLLMCostPricingFile is the model-pricing file the llm-cost policy fallback
const DefaultLLMCostPricingFile = "/etc/policy-engine/llm-pricing/model_prices.json"

// defaultResolvableConfig returns defaults for config keys that policy definitions
// reference via ${config...} system-parameter markers
func defaultResolvableConfig() map[string]interface{} {
	return map[string]interface{}{
		"policy_configurations.llm_cost_v1.pricing_file": DefaultLLMCostPricingFile,
	}
}

// defaultMaskedHeaders returns the header names whose values traffic logging redacts
func defaultMaskedHeaders() []string {
	return []string{"authorization", "x-api-key", "x-jwt-assertion"}
}

// defaultTrafficLogFileConfig returns the defaults for the "file" traffic-log sink.
// Path is intentionally empty: it is required only when the sink is selected, and a
// default path would create a file nobody asked for.
func defaultTrafficLogFileConfig() TrafficLogFileConfig {
	return TrafficLogFileConfig{
		Path: "",
		// 100 MiB live + 100 MiB backup = 200 MiB worst case, comfortably under a
		// modest emptyDir sizeLimit while still holding a useful window of traffic.
		MaxSizeMB: 100,
	}
}

// defaultTrafficLogHTTPConfig returns the defaults for the "http" traffic-log sink.
// Endpoint is intentionally empty: it is required only when the sink is selected.
func defaultTrafficLogHTTPConfig() TrafficLogHTTPConfig {
	return TrafficLogHTTPConfig{
		Endpoint:    "",
		ContentType: "application/x-ndjson",
		// 100 events / 1 MiB / 5s: small enough that a low-traffic gateway still
		// delivers promptly, large enough that a busy one is not doing a POST per
		// request.
		BatchMaxEvents: 100,
		BatchMaxBytes:  1 << 20,
		FlushInterval:  5 * time.Second,
		// 10k lines is roughly 100 MiB at the 10 KiB/line worst case — enough to
		// ride out a short receiver blip without letting a long outage grow the
		// heap without bound.
		QueueCapacity:  10000,
		OnQueueFull:    TrafficLogQueueDropNew,
		RequestTimeout: 10 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   time.Second,
		// Half: retry freely while the queue is shallow, stop once it is filling.
		RetryAbortQueueRatio: DefaultTrafficLogRetryAbortQueueRatio,
		Auth:                 TrafficLogHTTPAuthConfig{Type: TrafficLogAuthNone},
		TLS:                  TrafficLogHTTPTLSConfig{},
	}
}

// defaultAccessLogsServiceConfig returns the default policy-engine ALS receiver tuning.
// Shared by the collector (canonical) and the deprecated [analytics].access_logs_service
// alias so a partial alias override migrates cleanly.
func defaultAccessLogsServiceConfig() AccessLogsServiceConfig {
	return AccessLogsServiceConfig{
		Mode:                  "",
		ShutdownTimeout:       600 * time.Second,
		PublicKeyPath:         "",
		PrivateKeyPath:        "",
		ALSPlainText:          true,
		ExtProcMaxMessageSize: 1000000000,
		ExtProcMaxHeaderLimit: 8192,
	}
}

// defaultConfig returns a Config struct with default configuration values
func defaultConfig() *Config {
	return &Config{
		PolicyEngine: PolicyEngine{
			Server: ServerConfig{
				Mode:        "",
				ExtProcPort: 9001,
				// MaxRecvMsgBytes and MaxSendMsgBytes are deliberately left zero: Validate
				// derives them from the effective body ceilings, and a default here would
				// pre-empt that derivation. Since Load starts from this config, a non-zero
				// default is indistinguishable from an operator's explicit choice — so
				// raising request_body.max_decompressed_bytes would fail startup demanding
				// the message limits be restated, instead of following the ceiling up.
				MaxConcurrentStreams: DefaultMaxConcurrentStreams,
			},
			Admin: AdminConfig{
				Enabled:    true,
				Port:       9002,
				AllowedIPs: []string{"*"},
				Pprof: PprofConfig{
					Enabled:              false,
					BlockProfileRate:     0,
					MutexProfileFraction: 0,
				},
				ConfigDump: ConfigDumpConfig{
					Enabled: false,
				},
			},
			Metrics: MetricsConfig{
				Enabled: false,
				Port:    9003,
			},
			ConfigMode: ConfigModeConfig{
				Mode: "xds",
			},
			XDS: XDSConfig{
				ConnectTimeout:        10 * time.Second,
				RequestTimeout:        5 * time.Second,
				InitialReconnectDelay: 1 * time.Second,
				MaxReconnectDelay:     60 * time.Second,
				TLS: XDSTLSConfig{
					Enabled: false,
				},
			},
			FileConfig: FileConfigConfig{
				Path: "",
			},
			Logging: LoggingConfig{
				Level:  "info",
				Format: "text",
			},
			PythonExecutor: PythonExecutorConfig{
				Server: PythonExecutorServerConfig{
					Mode: "",
					Port: 9010,
					Host: "localhost",
				},
				Timeout: 30 * time.Second,
			},
			HTTPClient: HTTPClientConfig{
				Pooling: HTTPClientPoolingConfig{
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 10,
					MaxConnsPerHost:     100,
					IdleConnTimeout:     90 * time.Second,
					KeepAlive:           30 * time.Second,
				},
				Timeouts: HTTPClientTimeoutsConfig{
					Overall:        30 * time.Second, // safety-net only; see HTTPClientConfig's doc comment
					Dial:           10 * time.Second,
					TLSHandshake:   10 * time.Second,
					ResponseHeader: 10 * time.Second,
					ExpectContinue: 1 * time.Second,
				},
				TLS: HTTPClientTLSConfig{
					MinVersion:       "TLS1_2",
					MaxVersion:       "TLS1_3",
					CurvePreferences: "",
				},
				Proxy: HTTPClientProxyConfig{
					Mode: "none",
				},
				SSRF: HTTPClientSSRFConfig{
					Enabled: false,
				},
			},
			TracingServiceName: "policy-engine",
			RequestBody: BodyConfig{
				MaxDecompressedBytes: DefaultMaxDecompressedBytes,
			},
			ResponseBody: BodyConfig{
				MaxDecompressedBytes: DefaultMaxDecompressedBytes,
			},
		},
		Collector: CollectorConfig{
			RequestBody:  false,
			ResponseBody: false,
			Server:       defaultAccessLogsServiceConfig(),
		},
		TrafficLogging: TrafficLoggingConfig{
			Enabled: false,
			// Default to stdout so an existing deployment that upgrades without
			// touching its config keeps byte-identical behavior.
			Outputs:         []string{TrafficLogSinkStdout},
			File:            defaultTrafficLogFileConfig(),
			HTTP:            defaultTrafficLogHTTPConfig(),
			ShutdownTimeout: DefaultTrafficLogShutdownTimeout,
			MaskedHeaders:   defaultMaskedHeaders(),
			MaxPayloadSize:  0,
			RequestHeaders:  false,
			RequestBody:     false,
			ResponseHeaders: false,
			ResponseBody:    false,
			ExcludeFields:   []string{},
			Properties:      map[string]string{},
		},
		Analytics: AnalyticsConfig{
			Enabled:           false,
			EnabledPublishers: []string{"moesif"},
			Publishers: AnalyticsPublishersConfig{
				Moesif: MoesifPublisherConfig{
					ApplicationID:      "",
					BaseURL:            "https://api.moesif.net",
					PublishInterval:    5,
					EventQueueSize:     10000,
					BatchSize:          50,
					TimerWakeupSeconds: 3,
				},
			},
			GRPCEventServerCfg: map[string]interface{}{
				"server_port":           18090,
				"buffer_flush_interval": 1000000000,
				"buffer_size_bytes":     16384,
				"grpc_request_timeout":  20000000000,
			},
			// Deprecated alias: default mirrors the collector so a partial
			// [analytics.access_logs_service] override migrates cleanly.
			AccessLogsServiceCfg: defaultAccessLogsServiceConfig(),
			AllowPayloads:        false,
			SendRequestBody:      false,
			SendResponseBody:     false,
		},
		TracingConfig: TracingConfig{
			Enabled:            false,
			Endpoint:           "otel-collector:4317",
			Insecure:           true,
			ServiceVersion:     "1.0.0",
			BatchTimeout:       1 * time.Second,
			MaxExportBatchSize: 512,
			SamplingRate:       1.0,
		},
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate policy engine connection mode
	switch c.PolicyEngine.Server.Mode {
	case "uds", "":
	case "tcp":
		if c.PolicyEngine.Server.ExtProcPort <= 0 || c.PolicyEngine.Server.ExtProcPort > 65535 {
			return fmt.Errorf("invalid extproc_port: %d (must be 1-65535)", c.PolicyEngine.Server.ExtProcPort)
		}
	default:
		return fmt.Errorf("server.mode must be 'uds' or 'tcp', got: %s", c.PolicyEngine.Server.Mode)
	}

	// Validate python executor config
	switch c.PolicyEngine.PythonExecutor.Server.Mode {
	case "uds", "":
	case "tcp":
		if c.PolicyEngine.PythonExecutor.Server.Host == "" {
			return fmt.Errorf("invalid policy_engine.python_executor.server.host: must be non-empty when mode = 'tcp'")
		}
		if c.PolicyEngine.PythonExecutor.Server.Port <= 0 || c.PolicyEngine.PythonExecutor.Server.Port > 65535 {
			return fmt.Errorf("invalid policy_engine.python_executor.server.port: %d (must be 1-65535)", c.PolicyEngine.PythonExecutor.Server.Port)
		}
	default:
		return fmt.Errorf("policy_engine.python_executor.server.mode must be 'uds' or 'tcp', got: %s", c.PolicyEngine.PythonExecutor.Server.Mode)
	}
	if c.PolicyEngine.PythonExecutor.Timeout <= 0 {
		return fmt.Errorf("policy_engine.python_executor.timeout must be positive")
	}

	// Validate admin config
	if c.PolicyEngine.Admin.Enabled {
		if c.PolicyEngine.Admin.Port <= 0 || c.PolicyEngine.Admin.Port > 65535 {
			return fmt.Errorf("invalid admin.port: %d (must be 1-65535)", c.PolicyEngine.Admin.Port)
		}
		// Only check port conflict if using TCP mode
		if c.PolicyEngine.Server.Mode == "tcp" && c.PolicyEngine.Admin.Port == c.PolicyEngine.Server.ExtProcPort {
			return fmt.Errorf("admin.port cannot be same as server.extproc_port")
		}
		if len(c.PolicyEngine.Admin.AllowedIPs) == 0 {
			return fmt.Errorf("admin.allowed_ips cannot be empty when admin is enabled")
		}
	}

	// Validate metrics config
	if c.PolicyEngine.Metrics.Enabled {
		if c.PolicyEngine.Metrics.Port <= 0 || c.PolicyEngine.Metrics.Port > 65535 {
			return fmt.Errorf("invalid metrics.port: %d (must be 1-65535)", c.PolicyEngine.Metrics.Port)
		}
		// Only check port conflict if using TCP mode
		if c.PolicyEngine.Server.Mode == "tcp" && c.PolicyEngine.Metrics.Port == c.PolicyEngine.Server.ExtProcPort {
			return fmt.Errorf("metrics.port cannot be same as server.extproc_port")
		}
		if c.PolicyEngine.Metrics.Port == c.PolicyEngine.Admin.Port {
			return fmt.Errorf("metrics.port cannot be same as admin.port")
		}
	}

	if c.PolicyEngine.RequestBody.MaxDecompressedBytes <= 0 {
		return fmt.Errorf("policy_engine.request_body.max_decompressed_bytes must be positive, got %d", c.PolicyEngine.RequestBody.MaxDecompressedBytes)
	}
	if c.PolicyEngine.ResponseBody.MaxDecompressedBytes <= 0 {
		return fmt.Errorf("policy_engine.response_body.max_decompressed_bytes must be positive, got %d", c.PolicyEngine.ResponseBody.MaxDecompressedBytes)
	}

	// Both ceilings feed RequiredExtProcMessageBytes, which adds
	// ExtProcMessageOverheadBytes to the larger of them. Bounding them here, before that
	// addition happens, is what keeps it total.
	for name, v := range map[string]int64{
		"request_body":  c.PolicyEngine.RequestBody.MaxDecompressedBytes,
		"response_body": c.PolicyEngine.ResponseBody.MaxDecompressedBytes,
	} {
		if v > maxConfigurableDecompressedBytes {
			return fmt.Errorf(
				"policy_engine.%s.max_decompressed_bytes is %d, which exceeds the maximum %d — "+
					"a larger ceiling cannot have the %d of ext_proc message overhead added to it "+
					"without overflowing",
				name, v, maxConfigurableDecompressedBytes, ExtProcMessageOverheadBytes)
		}
	}

	// ext_proc gRPC message and stream limits.
	//
	// Unset means "derive from the body ceilings" rather than "reject", so a Config built
	// in code (tests, embedders) stays usable and an operator who raises a body ceiling
	// does not also have to restate the message limits. Load() starts from
	// defaultConfig(), so a file-sourced config already carries values; this covers the
	// rest. Same normalise-then-validate shape as the router's
	// per_connection_buffer_limit_bytes.
	required := c.PolicyEngine.RequiredExtProcMessageBytes()
	if c.PolicyEngine.Server.MaxRecvMsgBytes == 0 {
		c.PolicyEngine.Server.MaxRecvMsgBytes = required
	}
	if c.PolicyEngine.Server.MaxSendMsgBytes == 0 {
		c.PolicyEngine.Server.MaxSendMsgBytes = required
	}
	if c.PolicyEngine.Server.MaxConcurrentStreams == 0 {
		c.PolicyEngine.Server.MaxConcurrentStreams = DefaultMaxConcurrentStreams
	}

	// An *explicit* value below the ceiling is still refused: a message limit under the
	// body a policy is allowed to buffer fails mid-request with ResourceExhausted, which
	// surfaces as a gateway fault on live traffic instead of a startup error naming the
	// two settings that disagree. Refusing to start is the cheaper failure.
	if c.PolicyEngine.Server.MaxRecvMsgBytes < required {
		return fmt.Errorf(
			"policy_engine.server.max_recv_msg_bytes is %d, which is below the %d required by the configured "+
				"body decompression ceilings plus %d of ext_proc message overhead",
			c.PolicyEngine.Server.MaxRecvMsgBytes, required, ExtProcMessageOverheadBytes)
	}
	if c.PolicyEngine.Server.MaxSendMsgBytes < required {
		return fmt.Errorf(
			"policy_engine.server.max_send_msg_bytes is %d, which is below the %d required by the configured "+
				"body decompression ceilings plus %d of ext_proc message overhead",
			c.PolicyEngine.Server.MaxSendMsgBytes, required, ExtProcMessageOverheadBytes)
	}
	// grpc.MaxRecvMsgSize/MaxSendMsgSize take an int, so a value that does not survive
	// the conversion would silently become a different limit than the one configured.
	// int is 64-bit on every platform this ships on, making this unreachable there —
	// which is the point of asserting it here rather than at the conversion.
	for name, v := range map[string]int64{
		"max_recv_msg_bytes": c.PolicyEngine.Server.MaxRecvMsgBytes,
		"max_send_msg_bytes": c.PolicyEngine.Server.MaxSendMsgBytes,
	} {
		if int64(int(v)) != v {
			return fmt.Errorf("policy_engine.server.%s is %d, which does not fit this platform's int", name, v)
		}
	}

	// Validate config mode
	if c.PolicyEngine.ConfigMode.Mode != "file" && c.PolicyEngine.ConfigMode.Mode != "xds" {
		return fmt.Errorf("invalid config_mode.mode: %s (must be 'file' or 'xds')", c.PolicyEngine.ConfigMode.Mode)
	}

	// Validate based on config mode
	if c.PolicyEngine.ConfigMode.Mode == "xds" {
		if err := c.validateXDSConfig(); err != nil {
			return err
		}
	} else if c.PolicyEngine.ConfigMode.Mode == "file" {
		if c.PolicyEngine.FileConfig.Path == "" {
			return fmt.Errorf("file_config.path is required when config_mode.mode is 'file'")
		}
	}

	// Validate logging
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.PolicyEngine.Logging.Level] {
		return fmt.Errorf("invalid logging.level: %s (must be debug, info, warn, or error)", c.PolicyEngine.Logging.Level)
	}

	validFormats := map[string]bool{"json": true, "text": true}
	if !validFormats[c.PolicyEngine.Logging.Format] {
		return fmt.Errorf("invalid logging.format: %s (must be json or text)", c.PolicyEngine.Logging.Format)
	}

	if err := c.validateCollectorConfig(); err != nil {
		return err
	}
	if c.TrafficLogging.MaxPayloadSize < 0 {
		return fmt.Errorf("traffic_logging.max_payload_size must be >= 0, got %d", c.TrafficLogging.MaxPayloadSize)
	}
	if err := c.validateTrafficLoggingConfig(); err != nil {
		return err
	}
	if c.Analytics.Enabled {
		if err := c.validateAnalyticsConfig(); err != nil {
			return fmt.Errorf("analytics configuration validation failed: %v", err)
		}
	}
	if c.TracingConfig.Enabled {
		if c.TracingConfig.Endpoint == "" {
			return fmt.Errorf("tracing.endpoint is required when tracing is enabled")
		}
		if c.TracingConfig.BatchTimeout <= 0 {
			return fmt.Errorf("tracing.batch_timeout must be positive")
		}
		if c.TracingConfig.MaxExportBatchSize <= 0 {
			return fmt.Errorf("tracing.max_export_batch_size must be positive")
		}
		if c.TracingConfig.SamplingRate <= 0.0 || c.TracingConfig.SamplingRate > 1.0 {
			return fmt.Errorf("tracing.sampling_rate must be > 0.0 and <= 1.0, got %f", c.TracingConfig.SamplingRate)
		}
	}

	return nil
}

// validateXDSConfig validates xDS configuration
func (c *Config) validateXDSConfig() error {
	if c.PolicyEngine.XDS.ConnectTimeout <= 0 {
		return fmt.Errorf("xds.connect_timeout must be positive")
	}

	if c.PolicyEngine.XDS.RequestTimeout <= 0 {
		return fmt.Errorf("xds.request_timeout must be positive")
	}

	if c.PolicyEngine.XDS.InitialReconnectDelay <= 0 {
		return fmt.Errorf("xds.initial_reconnect_delay must be positive")
	}

	if c.PolicyEngine.XDS.MaxReconnectDelay <= 0 {
		return fmt.Errorf("xds.max_reconnect_delay must be positive")
	}

	if c.PolicyEngine.XDS.TLS.Enabled {
		if c.PolicyEngine.XDS.TLS.CertPath == "" {
			return fmt.Errorf("xds.tls.cert_path is required when TLS is enabled")
		}
		if c.PolicyEngine.XDS.TLS.KeyPath == "" {
			return fmt.Errorf("xds.tls.key_path is required when TLS is enabled")
		}
		if c.PolicyEngine.XDS.TLS.CAPath == "" {
			return fmt.Errorf("xds.tls.ca_path is required when TLS is enabled")
		}
	}

	return nil
}

// validateCollectorConfig migrates deprecated analytics capture aliases onto the
// collector and enforces the collector prerequisite: a consumer (analytics or
// traffic logging) requires the collector that feeds it. The collector has no
// on/off flag of its own: it is implicitly active whenever a consumer is enabled
// (see IsCollectorEnabled), so its transport is validated only in that case.
func (c *Config) validateCollectorConfig() error {
	c.migrateDeprecatedAnalyticsCapture()
	c.migrateDeprecatedAnalyticsTransport()

	if c.IsCollectorEnabled() {
		if err := validateAccessLogsServiceConfig(c.Collector.Server); err != nil {
			return err
		}
	}
	return nil
}

// IsCollectorEnabled reports whether the collector should run. The collector is
// implicit: it is active whenever any consumer of the collected data is enabled
// (analytics or stdout traffic logging), and off otherwise.
func (c *Config) IsCollectorEnabled() bool {
	return collector.IsEnabled(c.Analytics.Enabled, c.TrafficLogging.Enabled)
}

// migrateDeprecatedAnalyticsTransport maps a deprecated [analytics].access_logs_service
// override onto the collector when the collector's receiver tuning is still at its
// default, so existing configs keep working after the transport moved to [collector].
// See collector.MigrateDeprecatedTransport for the shared (with the gateway-controller)
// migration logic and its guarding-while-analytics-enabled rationale.
func (c *Config) migrateDeprecatedAnalyticsTransport() {
	collector.MigrateDeprecatedTransport(
		c.Analytics.Enabled,
		c.Analytics.AccessLogsServiceCfg,
		&c.Collector.Server,
		defaultAccessLogsServiceConfig(),
		"analytics.access_logs_service",
	)
}

// validateAccessLogsServiceConfig validates the policy-engine ALS receiver tuning.
// The transport port is normally the fixed, non-configurable collector.ServerPort
// constant (see collector.ServerPort); als.ServerPort is a deprecated override honored
// only for backward compatibility with configs that already set it (see its doc comment).
func validateAccessLogsServiceConfig(als AccessLogsServiceConfig) error {
	switch als.Mode {
	case "uds", "tcp", "":
	default:
		return fmt.Errorf("collector.server.mode must be 'uds' or 'tcp', got: %s", als.Mode)
	}
	if als.ServerPort != 0 {
		slog.Warn("collector.server.server_port is deprecated and no longer documented; the ALS port is fixed at " +
			strconv.Itoa(collector.ServerPort) + " by default. Honoring the configured override for backward " +
			"compatibility — ensure the gateway-controller's collector.server.port matches, or the two sides will fail to connect.")
		if als.ServerPort < 0 || als.ServerPort > 65535 {
			return fmt.Errorf("collector.server.server_port must be between 1 and 65535, got %d", als.ServerPort)
		}
	}
	if als.ShutdownTimeout <= 0 {
		return fmt.Errorf("collector.server.shutdown_timeout must be positive, got %s", als.ShutdownTimeout)
	}
	if als.ExtProcMaxMessageSize <= 0 {
		return fmt.Errorf("collector.server.max_message_size must be positive, got %d", als.ExtProcMaxMessageSize)
	}
	if als.ExtProcMaxHeaderLimit <= 0 {
		return fmt.Errorf("collector.server.max_header_limit must be positive, got %d", als.ExtProcMaxHeaderLimit)
	}
	if als.ExtProcMaxHeaderLimit > math.MaxUint32 {
		return fmt.Errorf("collector.server.max_header_limit must be <= %d, got %d", uint64(math.MaxUint32), als.ExtProcMaxHeaderLimit)
	}
	return nil
}

// migrateDeprecatedAnalyticsCapture maps the deprecated analytics.allow_payloads /
// analytics.send_request_body / analytics.send_response_body onto the collector's
// body-capture flags, so existing configs keep working after capture settings
// moved under [collector]. See collector.MigrateDeprecatedCapture for the shared
// (with the gateway-controller) migration logic and its guarding-while-analytics-
// enabled rationale.
func (c *Config) migrateDeprecatedAnalyticsCapture() {
	collector.MigrateDeprecatedCapture(
		c.Analytics.Enabled,
		collector.CaptureFlags{
			SendRequestBody:  c.Analytics.SendRequestBody,
			SendResponseBody: c.Analytics.SendResponseBody,
			AllowPayloads:    c.Analytics.AllowPayloads,
		},
		&c.Collector.RequestBody,
		&c.Collector.ResponseBody,
	)
}

// validateAnalyticsConfig validates the analytics consumer configuration (publishers).
// ALS transport validation lives in validateCollectorConfig.
func (c *Config) validateAnalyticsConfig() error {
	if c.Analytics.Enabled {
		// Validate enabled publishers
		for _, publisherName := range c.Analytics.EnabledPublishers {
			switch publisherName {
			case "moesif":
				moesifCfg := c.Analytics.Publishers.Moesif
				if moesifCfg.ApplicationID == "" {
					return fmt.Errorf("analytics.publishers.moesif.application_id is required when moesif is enabled")
				}
				if moesifCfg.PublishInterval <= 0 {
					return fmt.Errorf("analytics.publishers.moesif.publish_interval must be > 0 seconds, got %d", moesifCfg.PublishInterval)
				}
				if moesifCfg.BaseURL != "" {
					if u, err := url.Parse(moesifCfg.BaseURL); err != nil || u.Scheme == "" || u.Host == "" {
						return fmt.Errorf("analytics.publishers.moesif.moesif_base_url must be a valid URL (e.g. https://api.moesif.net), got %q", moesifCfg.BaseURL)
					}
				}
			default:
				return fmt.Errorf("unknown publisher type in enabled_publishers: %s", publisherName)
			}
		}
	}
	return nil
}

// validateTrafficLoggingConfig validates the traffic-logging config and warns
// about settings that have no effect.
func (c *Config) validateTrafficLoggingConfig() error {
	tl := c.TrafficLogging

	if !tl.Enabled {
		if len(tl.Properties) > 0 {
			slog.Warn("traffic_logging.properties is set but traffic_logging.enabled is false; it has no effect")
		}
		return nil
	}

	if tl.RequestBody && !c.Collector.RequestBody {
		slog.Warn("traffic_logging.request_body is true but collector.request_body is false; " +
			"traffic logging can only select among what the collector captured, so no request body will be logged")
	}
	if tl.ResponseBody && !c.Collector.ResponseBody {
		slog.Warn("traffic_logging.response_body is true but collector.response_body is false; " +
			"traffic logging can only select among what the collector captured, so no response body will be logged")
	}

	// A non-positive shutdown timeout is not a misconfiguration worth refusing to
	// start over — it only means "do not wait for buffered sinks to flush", which
	// is degraded but not a disclosure. It is clamped to the default at the point
	// of use (see DefaultTrafficLogShutdownTimeout). Negative is still nonsense.
	if tl.ShutdownTimeout < 0 {
		return fmt.Errorf("traffic_logging.shutdown_timeout must not be negative, got %s", tl.ShutdownTimeout)
	}

	// Validate the effective sink set, not each section in isolation: a sink that
	// cannot be built must fail startup rather than silently leaving the operator
	// on stdout, which would put request/response bodies back into the container
	// log — the exact disclosure a file/http sink is configured to prevent.
	outputs, err := NormalizeTrafficLogOutputs(tl.Outputs)
	if err != nil {
		return err
	}
	for _, out := range outputs {
		switch out {
		case TrafficLogSinkStdout:
			// Nothing to validate; always available.
		case TrafficLogSinkFile:
			if err := validateTrafficLogFileConfig(tl.File); err != nil {
				return fmt.Errorf("traffic_logging.file: %w", err)
			}
		case TrafficLogSinkHTTP:
			if err := validateTrafficLogHTTPConfig(tl.HTTP); err != nil {
				return fmt.Errorf("traffic_logging.http: %w", err)
			}
		}
	}

	return nil
}

// NormalizeTrafficLogOutputs lower-cases and trims the configured sink names,
// rejecting an unknown name or a duplicate. It is exported so the publisher layer
// builds its sinks from exactly the same normalized list that was validated here,
// rather than re-parsing the raw strings and possibly disagreeing.
//
// An unset or empty list resolves to ["stdout"], the historical behavior, with a
// warning. It is deliberately not an error: the case that must fail loudly is a
// name that was meant to select a sink and does not (a typo such as "flie"), which
// would otherwise leave an operator who asked for a file sink writing bodies to the
// container log. An empty list expresses no such intent and cannot be mistaken for
// one.
func NormalizeTrafficLogOutputs(outputs []string) ([]string, error) {
	normalized := make([]string, 0, len(outputs))
	seen := make(map[string]bool, len(outputs))
	for _, raw := range outputs {
		out := strings.ToLower(strings.TrimSpace(raw))
		if out == "" {
			continue
		}
		switch out {
		case TrafficLogSinkStdout, TrafficLogSinkFile, TrafficLogSinkHTTP:
		default:
			return nil, fmt.Errorf("unknown traffic_logging.outputs entry %q (valid: %s, %s, %s)",
				raw, TrafficLogSinkStdout, TrafficLogSinkFile, TrafficLogSinkHTTP)
		}
		if seen[out] {
			return nil, fmt.Errorf("traffic_logging.outputs lists %q more than once", out)
		}
		seen[out] = true
		normalized = append(normalized, out)
	}
	if len(normalized) == 0 {
		if len(outputs) > 0 {
			// The operator wrote something that reduced to nothing (e.g. outputs =
			// [""]); say so rather than silently substituting a default.
			slog.Warn("traffic_logging.outputs contains no usable sink name; falling back to " +
				TrafficLogSinkStdout)
		}
		return []string{TrafficLogSinkStdout}, nil
	}
	return normalized, nil
}

// EffectiveRetryAbortDepth returns the queue depth at or above which a retrying
// batch gives up. The configured ratio is used literally — it is NOT remapped, so
// what an operator writes is what runs:
//
//	0            depth 0, which every queue length satisfies: retries are always
//	             abandoned, i.e. one delivery attempt per batch.
//	0 < r <= 1   depth = queue_capacity * r.
//
// An omitted key keeps the shipped default (see defaultTrafficLogHTTPConfig),
// because config.Load unmarshals over a pre-populated struct: absent leaves 0.5
// in place, while an explicit 0 overwrites it. That is what makes "0 means 0"
// safe here — the two cases are genuinely distinguishable.
//
// The floor applies only to a NON-zero ratio, where a depth of 0 could only come
// from rounding a small ratio against a small queue and would silently mean
// something the operator did not ask for.
func (c TrafficLogHTTPConfig) EffectiveRetryAbortDepth() int {
	depth := int(float64(c.QueueCapacity) * c.RetryAbortQueueRatio)
	if depth < 1 && c.RetryAbortQueueRatio > 0 {
		depth = 1
	}
	return depth
}

// DefaultTrafficLogRetryAbortQueueRatio is the fraction of the HTTP sink's queue
// at which a retrying batch gives up. Half is a deliberate midpoint: high enough
// that an ordinary blip still gets its full retry budget, low enough that a hung
// receiver cannot consume the whole queue before the sender resumes draining.
const DefaultTrafficLogRetryAbortQueueRatio = 0.5

// DefaultTrafficLogShutdownTimeout bounds the flush of buffering traffic-log sinks
// when traffic_logging.shutdown_timeout is unset or non-positive.
const DefaultTrafficLogShutdownTimeout = 5 * time.Second

// EffectiveShutdownTimeout returns the shutdown timeout to actually use, applying
// the default when the configured value is unset or non-positive. This keeps a
// hand-built Config (tests, embedding callers) from ending up with a zero timeout
// that would skip the flush entirely.
func (t TrafficLoggingConfig) EffectiveShutdownTimeout() time.Duration {
	if t.ShutdownTimeout <= 0 {
		return DefaultTrafficLogShutdownTimeout
	}
	return t.ShutdownTimeout
}

// validateTrafficLogFileConfig checks the file sink can actually be used, and
// proves it by creating the parent directory and opening the file. Doing the real
// I/O here — rather than deferring it to first write — means a permissions or
// mount problem surfaces as a clean startup failure instead of silently dropping
// traffic logs at runtime, once PII is already flowing.
func validateTrafficLogFileConfig(cfg TrafficLogFileConfig) error {
	path, err := ResolveTrafficLogFilePath(cfg.Path)
	if err != nil {
		return err
	}
	if cfg.MaxSizeMB < 0 {
		return fmt.Errorf("max_size_mb must be >= 0, got %d", cfg.MaxSizeMB)
	}
	if cfg.MaxSizeMB == 0 {
		slog.Warn("traffic_logging.file.max_size_mb is 0: rotation is disabled and the traffic log "+
			"will grow until the volume is full. Set a size ceiling, or make sure the mounted volume "+
			"bounds it.", "path", path)
	}

	// 0700 on the directory and 0600 on the file, established at creation. umask
	// can only clear permission bits, never add them, so these modes hold under
	// any umask; a post-hoc chmod would leave a window where the file is readable.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("cannot create directory for %q: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("cannot open %q for append: %w", path, err)
	}
	permErr := VerifyTrafficLogPerms(f, path)
	if err := f.Close(); err != nil {
		return fmt.Errorf("cannot close %q: %w", path, err)
	}
	return permErr
}

// TrafficLogPermMask is the set of permission bits that must be clear on the
// traffic-log file and its directory: anything readable by group or other.
const TrafficLogPermMask os.FileMode = 0o077

// VerifyTrafficLogPerms fails when the traffic-log file or its directory is more
// permissive than the modes this sink creates.
//
// MkdirAll and O_CREATE apply their mode only when they actually create; a path
// left behind by an earlier run, restored from a backup, or pre-created on a
// mounted volume silently keeps its old permissions. That file holds request and
// response bodies, so a world-readable one left over from before defeats the
// entire reason for choosing the file sink.
//
// The file is a hard failure, per GO-AUTH-018: it is the confidentiality
// boundary, and it fails rather than chmod'ing because the restrictive mode must
// be established at creation time — a chmod after the descriptor is open leaves a
// window in which the file is both populated and readable.
//
// The containing directory only warns. A 0600 file is unreadable regardless of
// its directory's mode, so a permissive directory is an integrity and
// defence-in-depth concern rather than a disclosure. It cannot be an error
// because the directory is frequently not ours: under Kubernetes an emptyDir
// mount point is created 0777 by the kubelet, so an operator who points
// file.path straight at the mount root would otherwise be unable to start at all.
func VerifyTrafficLogPerms(f *os.File, path string) error {
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat %q: %w", path, err)
	}
	if perm := fi.Mode().Perm(); perm&TrafficLogPermMask != 0 {
		return fmt.Errorf("%q has permissions %#o, which allow group/other access to logged "+
			"request and response bodies; it already existed so it kept its previous mode. "+
			"Fix it with `chmod 600 %s` (or remove the file) and restart", path, perm, path)
	}

	dir := filepath.Dir(path)
	if di, err := os.Stat(dir); err == nil {
		if perm := di.Mode().Perm(); perm&TrafficLogPermMask != 0 {
			slog.Warn("traffic-log directory is group/other accessible; the log file itself is "+
				"0600 so its contents are not exposed, but consider placing the file in a "+
				"dedicated 0700 subdirectory of the mount rather than at its root",
				"dir", dir, "mode", fmt.Sprintf("%#o", perm))
		}
	}
	return nil
}

// ResolveTrafficLogFilePath validates and cleans a traffic-log file path. Exported
// so the file sink resolves the path identically to the way it was validated.
func ResolveTrafficLogFilePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required when the %q sink is selected", TrafficLogSinkFile)
	}
	if strings.ContainsRune(path, 0) {
		return "", fmt.Errorf("path must not contain a null byte")
	}
	if !filepath.IsAbs(path) {
		// A relative path resolves against the process's working directory, which
		// is an implementation detail of the image and almost never intended.
		return "", fmt.Errorf("path must be absolute, got %q", path)
	}
	// filepath.Clean RESOLVES ".." rather than rejecting it, so
	// "/var/log/wso2/../../etc/traffic.log" would silently become
	// "/etc/traffic.log" — outside whatever volume the operator mounted for it,
	// and past the chart's mount-containment check, which compares the
	// un-normalized string. Reject the traversal instead of quietly relocating
	// the file (file-access.md directive 1).
	for _, segment := range strings.Split(path, string(filepath.Separator)) {
		if segment == ".." {
			return "", fmt.Errorf("path must not contain %q segments, got %q", "..", path)
		}
	}
	return filepath.Clean(path), nil
}

// validateTrafficLogHTTPConfig checks the HTTP sink's endpoint, bounds, auth and
// TLS material. Any secret material referenced here has already been resolved by
// the {{ file }} / {{ env }} interpolation pass, so a missing secret file fails
// before this point.
func validateTrafficLogHTTPConfig(cfg TrafficLogHTTPConfig) error {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return fmt.Errorf("endpoint is required when the %q sink is selected", TrafficLogSinkHTTP)
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is not a valid URL: %w", err)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !cfg.AllowInsecureTransport {
			return fmt.Errorf("endpoint uses plaintext http:// but allow_insecure_transport is false; " +
				"the traffic log carries request and response bodies, so set allow_insecure_transport = true " +
				"only for a trusted local collector")
		}
		slog.Warn("traffic_logging.http endpoint is plaintext http://; request and response bodies "+
			"are transmitted unencrypted", "host", u.Host)
	default:
		return fmt.Errorf("endpoint scheme must be https (or http with allow_insecure_transport), got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("endpoint must include a host, got %q", cfg.Endpoint)
	}

	if cfg.BatchMaxEvents <= 0 {
		return fmt.Errorf("batch_max_events must be positive, got %d", cfg.BatchMaxEvents)
	}
	if cfg.BatchMaxBytes <= 0 {
		return fmt.Errorf("batch_max_bytes must be positive, got %d", cfg.BatchMaxBytes)
	}
	if cfg.FlushInterval <= 0 {
		return fmt.Errorf("flush_interval must be positive, got %s", cfg.FlushInterval)
	}
	if cfg.QueueCapacity <= 0 {
		return fmt.Errorf("queue_capacity must be positive, got %d; an unbounded queue in front of a "+
			"bounded sender is deferred unbounded memory growth", cfg.QueueCapacity)
	}
	switch strings.ToLower(strings.TrimSpace(cfg.OnQueueFull)) {
	case TrafficLogQueueDropNew, TrafficLogQueueDropOldest:
	default:
		return fmt.Errorf("on_queue_full must be %q or %q, got %q",
			TrafficLogQueueDropNew, TrafficLogQueueDropOldest, cfg.OnQueueFull)
	}
	if cfg.RequestTimeout <= 0 {
		return fmt.Errorf("request_timeout must be positive, got %s", cfg.RequestTimeout)
	}
	if cfg.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be >= 0, got %d", cfg.MaxRetries)
	}
	if cfg.MaxRetries > 0 && cfg.RetryBackoff <= 0 {
		return fmt.Errorf("retry_backoff must be positive when max_retries > 0, got %s", cfg.RetryBackoff)
	}
	if cfg.RetryAbortQueueRatio < 0 || cfg.RetryAbortQueueRatio > 1 {
		return fmt.Errorf("retry_abort_queue_ratio must be between 0 and 1 "+
			"(0 = always abandon retries, 1 = never abort early), got %v",
			cfg.RetryAbortQueueRatio)
	}

	if err := validateTrafficLogHTTPAuth(cfg.Auth); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if err := validateTrafficLogHTTPTLS(cfg.TLS, u.Host); err != nil {
		return fmt.Errorf("tls: %w", err)
	}
	return nil
}

// validateTrafficLogHTTPAuth checks the auth type and that its required fields are
// present. Error messages never include the secret value itself.
func validateTrafficLogHTTPAuth(cfg TrafficLogHTTPAuthConfig) error {
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "", TrafficLogAuthNone:
		return nil
	case TrafficLogAuthBearer:
		if cfg.Bearer.Token == "" {
			return fmt.Errorf("bearer.token is required when type is %q", TrafficLogAuthBearer)
		}
	case TrafficLogAuthBasic:
		if cfg.Basic.Username == "" || cfg.Basic.Password == "" {
			return fmt.Errorf("basic.username and basic.password are both required when type is %q",
				TrafficLogAuthBasic)
		}
	case TrafficLogAuthHeader:
		if cfg.Header.Name == "" || cfg.Header.Value == "" {
			return fmt.Errorf("header.name and header.value are both required when type is %q",
				TrafficLogAuthHeader)
		}
	default:
		return fmt.Errorf("unknown type %q (valid: %s, %s, %s, %s)", cfg.Type,
			TrafficLogAuthNone, TrafficLogAuthBearer, TrafficLogAuthBasic, TrafficLogAuthHeader)
	}
	return nil
}

// validateTrafficLogHTTPTLS checks that any referenced TLS material exists and
// parses, so a bad path fails at startup rather than on the first batch.
func validateTrafficLogHTTPTLS(cfg TrafficLogHTTPTLSConfig, host string) error {
	if cfg.InsecureSkipVerify {
		slog.Warn("traffic_logging.http.tls.insecure_skip_verify is true: the receiver's certificate "+
			"is not verified, so request and response bodies are exposed to anyone able to intercept "+
			"this connection", "host", host)
	}
	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return fmt.Errorf("cannot read ca_file %q: %w", cfg.CAFile, err)
		}
		if !x509.NewCertPool().AppendCertsFromPEM(pem) {
			return fmt.Errorf("ca_file %q contains no usable PEM certificate", cfg.CAFile)
		}
	}
	if (cfg.CertFile == "") != (cfg.KeyFile == "") {
		return fmt.Errorf("cert_file and key_file must be set together for mTLS (one is set, the other is not)")
	}
	if cfg.CertFile != "" {
		if _, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile); err != nil {
			return fmt.Errorf("cannot load client certificate/key pair: %w", err)
		}
	}
	return nil
}

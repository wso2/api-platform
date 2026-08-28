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

package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"

	"github.com/wso2/api-platform/httpkit/tlsconfig"
)

// Config is the top-level runtime configuration for the event gateway.
type Config struct {
	Server       ServerConfig       `koanf:"server"`
	Kafka        KafkaConfig        `koanf:"kafka"`
	WebSub       WebSubConfig       `koanf:"websub"`
	PolicyEngine PolicyEngineConfig `koanf:"policy_engine"`
	ControlPlane ControlPlaneConfig `koanf:"controlplane"`
	Logging      LoggingConfig      `koanf:"logging"`
	HTTPClient   HTTPClientConfig   `koanf:"http_client"`
	RuntimeID    string             `koanf:"runtime_id"`
}

// ServerConfig holds HTTP/WS server settings.
type ServerConfig struct {
	WebSubEnabled                bool   `koanf:"websub_enabled"`
	WebSubHTTPPort               int    `koanf:"websub_http_port"`
	WebSubHTTPSPort              int    `koanf:"websub_https_port"`
	WebSubTLSEnabled             bool   `koanf:"websub_tls_enabled"`
	WebSubTLSCertFile            string `koanf:"websub_tls_cert_file"`
	WebSubTLSKeyFile             string `koanf:"websub_tls_key_file"`
	WebSubTLSMinVersion          string `koanf:"websub_tls_min_version"`
	WebSubTLSMaxVersion          string `koanf:"websub_tls_max_version"`
	WebSubTLSCipherSuites        string `koanf:"websub_tls_cipher_suites"`
	WebSubTLSCurvePreferences    string `koanf:"websub_tls_curve_preferences"`
	WebSocketPort                int    `koanf:"websocket_port"`
	WebSocketHTTPSPort           int    `koanf:"websocket_https_port"`
	WebSocketTLSEnabled          bool   `koanf:"websocket_tls_enabled"`
	WebSocketTLSCertFile         string `koanf:"websocket_tls_cert_file"`
	WebSocketTLSKeyFile          string `koanf:"websocket_tls_key_file"`
	WebSocketTLSMinVersion       string `koanf:"websocket_tls_min_version"`
	WebSocketTLSMaxVersion       string `koanf:"websocket_tls_max_version"`
	WebSocketTLSCipherSuites     string `koanf:"websocket_tls_cipher_suites"`
	WebSocketTLSCurvePreferences string `koanf:"websocket_tls_curve_preferences"`
	AdminPort                    int    `koanf:"admin_port"`
	MetricsPort                  int    `koanf:"metrics_port"`

	// ReadTimeout, WriteTimeout, and IdleTimeout bound the WebSub/WebSocket
	// managed HTTP(S) servers (see newManagedServer) so a slow or malicious
	// client can't hold a connection open indefinitely (Slowloris-style
	// resource exhaustion). MaxHeaderBytes bounds header size the same way.
	// All four must be non-zero — DefaultConfig supplies safe defaults.
	ReadTimeout    time.Duration `koanf:"read_timeout"`
	WriteTimeout   time.Duration `koanf:"write_timeout"`
	IdleTimeout    time.Duration `koanf:"idle_timeout"`
	MaxHeaderBytes int           `koanf:"max_header_bytes"`
}

// KafkaConfig holds Kafka connection settings.
type KafkaConfig struct {
	Brokers             []string `koanf:"brokers"`
	ConsumerGroupPrefix string   `koanf:"consumer_group_prefix"`
	TLS                 bool     `koanf:"tls"`
	TLSCAFile           string   `koanf:"tls_ca_file"`
	TLSCertFile         string   `koanf:"tls_cert_file"`
	TLSKeyFile          string   `koanf:"tls_key_file"`
	TLSServerName       string   `koanf:"tls_server_name"`
	SASLMechanism       string   `koanf:"sasl_mechanism"`
	SASLUsername        string   `koanf:"sasl_username"`
	SASLPassword        string   `koanf:"sasl_password"`
}

// WebSubConfig holds WebSub-specific settings.
type WebSubConfig struct {
	VerificationTimeoutSeconds int    `koanf:"verification_timeout_seconds"`
	DeliveryMaxRetries         int    `koanf:"delivery_max_retries"`
	DeliveryInitialDelayMs     int    `koanf:"delivery_initial_delay_ms"`
	DeliveryMaxDelayMs         int    `koanf:"delivery_max_delay_ms"`
	DeliveryConcurrency        int    `koanf:"delivery_concurrency"`
	DefaultLeaseSeconds        int    `koanf:"default_lease_seconds"`
	SubscriptionsTopicName     string `koanf:"subscriptions_topic_name"`
}

// PolicyEngineConfig points to the policy engine configuration.
type PolicyEngineConfig struct {
	ConfigFile string `koanf:"config_file"`
	ChainsFile string `koanf:"chains_file"`
}

// ControlPlaneConfig configures the xDS-based control plane connection.
type ControlPlaneConfig struct {
	Enabled    bool   `koanf:"enabled"`
	XDSAddress string `koanf:"xds_address"`
	NodeID     string `koanf:"node_id"`
}

// LoggingConfig controls the runtime's structured logger.
type LoggingConfig struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}

// HTTPClientConfig configures the shared outbound *http.Client used by WebSub's
// subscribe/unsubscribe intent verification (Verifier, see
// internal/connectors/receiver/websub/verification.go) and message delivery
// (Deliverer, see .../delivery.go). Both call sites dial a tenant/subscriber-supplied
// CallbackURL and share the identical pooling/TLS/proxy/SSRF posture, so a single
// [http_client] TOML section configures both; each call site still keeps its own
// existing timeout parameter/field (Verifier's `timeout` argument, sourced from
// websub.verification_timeout_seconds; Deliverer's own delivery timeout), which always
// overrides Timeouts.Overall below for that specific client — see BuildHTTPClientConfig.
//
// This mirrors the TOML-expressible subset of
// github.com/wso2/api-platform/httpkit/httpclient.Config (see gateway-controller's
// pkg/config.HTTPClientConfig for the reference shape this intentionally mirrors
// field-for-field, aside from SSRF below). Go-only callback hooks
// (GetClientCertificate, VerifyPeerCertificate, VerifyConnection, ConnectHeader) and a
// pre-built *x509.CertPool have no TOML shape and are not represented here.
//
// Unlike gateway-controller's HTTPClientConfig, SSRF protection is NOT configurable
// off: every CallbackURL dialed by either client is tenant/subscriber-supplied —
// exactly the scenario ssrf-prevention.md targets — so SSRF.Enabled=true and
// netguard.PublicOnly() are hardcoded in BuildHTTPClientConfig rather than sourced
// from this struct. PublicOnly (not PermitPrivateBlockMetadata) is deliberate: a
// tenant-supplied CallbackURL must never be usable to reach an operator's own
// private/loopback network, only the public internet. Only the redirect/scheme
// knobs netguard exposes are configurable, via HTTPClientSSRFConfig.
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

// HTTPClientTimeoutsConfig mirrors httpclient.TimeoutsConfig. Overall is only a common
// default shared by both call sites — see HTTPClientConfig's doc comment for why each
// call site's own existing timeout always takes precedence over it.
type HTTPClientTimeoutsConfig struct {
	Overall        time.Duration `koanf:"overall"`
	Dial           time.Duration `koanf:"dial"`
	TLSHandshake   time.Duration `koanf:"tls_handshake"`
	ResponseHeader time.Duration `koanf:"response_header"`
	ExpectContinue time.Duration `koanf:"expect_continue"`
	// MaxResponseBytes bounds a callback response body. 0 = package default
	// (10MiB). Unlike httpclient.TimeoutsConfig.MaxResponseBytes, a negative
	// value here is rejected by BuildHTTPClientConfig rather than disabling
	// the bound: every response body this client reads comes from a
	// tenant/subscriber-supplied CallbackURL, so an unbounded read is never
	// an acceptable opt-in for this client (see file-access.md directive 5).
	MaxResponseBytes int64 `koanf:"max_response_bytes"`
}

// HTTPClientTLSConfig mirrors the TOML-expressible subset of httpclient.TLSConfig.
type HTTPClientTLSConfig struct {
	MinVersion       string `koanf:"min_version"`       // one of "TLS1_0".."TLS1_3"
	MaxVersion       string `koanf:"max_version"`       // one of "TLS1_0".."TLS1_3"
	CipherSuites     string `koanf:"cipher_suites"`     // comma-separated Go crypto/tls cipher suite names; TLS 1.2 and below only
	CurvePreferences string `koanf:"curve_preferences"` // comma-separated, e.g. "X25519MLKEM768,X25519,P-256"
	RootCAFile       string `koanf:"root_ca_file"`      // PEM CA bundle; empty uses the system root pool
	ClientCertFile   string `koanf:"client_cert_file"`  // mTLS to the callback endpoint; both cert and key must be set together
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
	// set explicitly whenever Mode != "none" — SSRF guarding is always enabled for this
	// client (see HTTPClientConfig's doc comment), unlike gateway-controller where this
	// is only required when SSRF is ALSO enabled. BuildHTTPClientConfig fails closed at
	// config-build time otherwise rather than silently choosing one.
	Egress string `koanf:"egress"`
}

// HTTPClientProxyTLSConfig mirrors httpclient.ProxyTLSConfig (the proxy's own TLS
// handshake, fully decoupled from the origin TLS handshake in HTTPClientTLSConfig).
type HTTPClientProxyTLSConfig struct {
	RootCAFile     string `koanf:"root_ca_file"`
	ClientCertFile string `koanf:"client_cert_file"`
	ClientKeyFile  string `koanf:"client_key_file"`
	// InsecureSkipVerify and InsecureSkipVerifyAcknowledged are deliberately
	// separate fields: httpkit's own acknowledgement gate
	// (httpclient.ProxyTLSConfig, see tls.go) requires an operator to opt
	// into disabling verification twice, once per field, so a single
	// "insecure_skip_verify = true" in TOML can't silently satisfy its own
	// gate. Both must be explicitly set to true for InsecureSkipVerify to
	// take effect.
	InsecureSkipVerify             bool `koanf:"insecure_skip_verify"`
	InsecureSkipVerifyAcknowledged bool `koanf:"insecure_skip_verify_acknowledged"`
}

// HTTPClientSSRFConfig mirrors the TOML-expressible redirect/scheme knobs of
// httpclient.SSRFConfig. It deliberately has NO Enabled/Preset field: SSRF guarding for
// this client is always on with netguard.PublicOnly() (see HTTPClientConfig's doc
// comment) — there is no supported way to disable it, or to permit private/loopback
// destinations, via config.
type HTTPClientSSRFConfig struct {
	MaxRedirects   int      `koanf:"max_redirects"`   // 0 uses netguard's own default (5)
	AllowedSchemes []string `koanf:"allowed_schemes"` // empty defaults to {"https"}
}

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			WebSubEnabled:      true,
			WebSubHTTPPort:     8080,
			WebSubHTTPSPort:    8443,
			WebSocketPort:      8081,
			WebSocketHTTPSPort: 8444,
			AdminPort:          9002,
			MetricsPort:        9003,
			ReadTimeout:        30 * time.Second,
			WriteTimeout:       60 * time.Second,
			IdleTimeout:        120 * time.Second,
			MaxHeaderBytes:     1 << 20, // 1 MiB
		},
		Kafka: KafkaConfig{
			Brokers:             []string{"localhost:9092"},
			ConsumerGroupPrefix: "event-gateway",
		},
		WebSub: WebSubConfig{
			VerificationTimeoutSeconds: 10,
			DeliveryMaxRetries:         5,
			DeliveryInitialDelayMs:     1000,
			DeliveryMaxDelayMs:         60000,
			DeliveryConcurrency:        64,
			DefaultLeaseSeconds:        0,
			SubscriptionsTopicName:     "__subscriptions",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
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
				Overall:        30 * time.Second, // common default only; each call site's own timeout overrides this
				Dial:           10 * time.Second,
				TLSHandshake:   10 * time.Second,
				ResponseHeader: 10 * time.Second,
				ExpectContinue: 1 * time.Second,
			},
			TLS: HTTPClientTLSConfig{
				MinVersion:       "TLS1_2",
				MaxVersion:       "TLS1_3",
				CurvePreferences: "X25519MLKEM768,X25519,P-256",
			},
			Proxy: HTTPClientProxyConfig{
				Mode: "none",
			},
		},
	}
}

// Load loads configuration from a TOML file and environment variables.
// Environment variables use the prefix APIP_EGW_ and map top-level sections to
// names such as APIP_EGW_SERVER_WEBSUB_PORT and APIP_EGW_CONTROLPLANE_XDS_ADDRESS.
func Load(path string) (*Config, map[string]interface{}, error) {
	k := koanf.New(".")
	cfg := DefaultConfig()

	if path != "" {
		if err := k.Load(file.Provider(path), toml.Parser()); err != nil {
			return nil, nil, fmt.Errorf("failed to load config file %s: %w", path, err)
		}
	}

	// Load environment variable overrides. Single underscores separate the
	// top-level section from the field name, while field-name underscores are
	// preserved (for example SERVER_WEBSUB_PORT -> server.websub_port).
	if err := k.Load(env.ProviderWithValue("APIP_EGW_", ".", func(key, value string) (string, interface{}) {
		mapped := mapEnvKey(key)
		if mapped == "" {
			return "", nil
		}
		return mapped, mapEnvValue(mapped, value)
	}), nil); err != nil {
		return nil, nil, fmt.Errorf("failed to load env vars: %w", err)
	}

	if err := k.Unmarshal("", cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := validate(cfg); err != nil {
		return nil, nil, err
	}

	// Extract the raw map for policy_configurations (used for ${config} resolution).
	// k.Raw() returns a nested map[string]interface{} which is what the policy engine's
	// SetConfig/config resolver expects. k.All() returns a flat dot-separated map and
	// would cause ${config.policy_configurations.*} references to fail silently.
	rawConfig := k.Raw()

	slog.Info("Configuration loaded",
		"websub_enabled", cfg.Server.WebSubEnabled,
		"websub_http_port", cfg.Server.WebSubHTTPPort,
		"websub_https_port", cfg.Server.WebSubHTTPSPort,
		"websub_tls_enabled", cfg.Server.WebSubTLSEnabled,
		"websocket_port", cfg.Server.WebSocketPort,
		"websocket_tls_enabled", cfg.Server.WebSocketTLSEnabled,
		"admin_port", cfg.Server.AdminPort,
		"kafka_brokers", cfg.Kafka.Brokers,
		"log_level", cfg.Logging.Level,
		"log_format", cfg.Logging.Format,
	)

	return cfg, rawConfig, nil
}

func mapEnvKey(key string) string {
	name := strings.ToLower(strings.TrimPrefix(key, "APIP_EGW_"))

	switch {
	case name == "runtime_id":
		return "runtime_id"
	case strings.HasPrefix(name, "server_"):
		return "server." + strings.TrimPrefix(name, "server_")
	case strings.HasPrefix(name, "kafka_"):
		return "kafka." + strings.TrimPrefix(name, "kafka_")
	case strings.HasPrefix(name, "websub_"):
		return "websub." + strings.TrimPrefix(name, "websub_")
	case strings.HasPrefix(name, "policy_engine_"):
		return "policy_engine." + strings.TrimPrefix(name, "policy_engine_")
	case strings.HasPrefix(name, "controlplane_"):
		return "controlplane." + strings.TrimPrefix(name, "controlplane_")
	case strings.HasPrefix(name, "logging_"):
		return "logging." + strings.TrimPrefix(name, "logging_")
	default:
		// Support generic nested keys using "__" for literal underscores.
		name = strings.ReplaceAll(name, "__", "%UNDERSCORE%")
		name = strings.ReplaceAll(name, "_", ".")
		name = strings.ReplaceAll(name, "%UNDERSCORE%", "_")
		return name
	}
}

func mapEnvValue(path, value string) interface{} {
	value = strings.TrimSpace(value)

	switch path {
	case "kafka.brokers":
		return splitCSV(value)
	case "server.websub_http_port",
		"server.websub_https_port",
		"server.websocket_port",
		"server.websocket_https_port",
		"server.admin_port",
		"server.metrics_port",
		"websub.verification_timeout_seconds",
		"websub.delivery_max_retries",
		"websub.delivery_initial_delay_ms",
		"websub.delivery_max_delay_ms",
		"websub.delivery_concurrency",
		"websub.default_lease_seconds":
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	case "kafka.tls", "controlplane.enabled", "server.websub_enabled", "server.websub_tls_enabled", "server.websocket_tls_enabled":
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}

	return value
}

func splitCSV(value string) []string {
	if value == "" {
		return []string{}
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func validate(cfg *Config) error {
	if err := validateServerPorts(cfg.Server); err != nil {
		return err
	}

	if err := validateServerTimeouts(cfg.Server); err != nil {
		return err
	}

	if cfg.Server.WebSubTLSEnabled {
		if err := validateReadableFile(cfg.Server.WebSubTLSCertFile, "server.websub_tls_cert_file", "server.websub_tls_enabled"); err != nil {
			return err
		}
		if err := validateReadableFile(cfg.Server.WebSubTLSKeyFile, "server.websub_tls_key_file", "server.websub_tls_enabled"); err != nil {
			return err
		}
		if err := validateListenerTLSTuning("server.websub_tls", cfg.Server.WebSubTLSMinVersion, cfg.Server.WebSubTLSMaxVersion, cfg.Server.WebSubTLSCipherSuites, cfg.Server.WebSubTLSCurvePreferences); err != nil {
			return err
		}
	}

	if cfg.Server.WebSocketTLSEnabled {
		if err := validateReadableFile(cfg.Server.WebSocketTLSCertFile, "server.websocket_tls_cert_file", "server.websocket_tls_enabled"); err != nil {
			return err
		}
		if err := validateReadableFile(cfg.Server.WebSocketTLSKeyFile, "server.websocket_tls_key_file", "server.websocket_tls_enabled"); err != nil {
			return err
		}
		if err := validateListenerTLSTuning("server.websocket_tls", cfg.Server.WebSocketTLSMinVersion, cfg.Server.WebSocketTLSMaxVersion, cfg.Server.WebSocketTLSCipherSuites, cfg.Server.WebSocketTLSCurvePreferences); err != nil {
			return err
		}
	}

	switch cfg.Logging.Level {
	case "", "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("logging.level must be one of debug, info, warn, error")
	}

	switch cfg.Logging.Format {
	case "", "text", "json":
	default:
		return fmt.Errorf("logging.format must be one of text, json")
	}

	if err := validateKafkaConfig(cfg.Kafka); err != nil {
		return err
	}

	return nil
}

// validateListenerTLSTuning validates the optional min/max TLS version,
// cipher suite, and curve preference tuning for one of the inbound HTTPS
// listeners (WebSub or WebSocket). All four fields are optional — leaving
// them empty defers to Go's own crypto/tls defaults — but if set, they must
// name a value tlsconfig recognizes. fieldPrefix is used to qualify the
// returned error (e.g. "server.websub_tls").
func validateListenerTLSTuning(fieldPrefix, minVersion, maxVersion, cipherSuites, curvePreferences string) error {
	if err := tlsconfig.ValidateVersionRange(minVersion, maxVersion); err != nil {
		return fmt.Errorf("%s_min_version/%s_max_version: %w", fieldPrefix, fieldPrefix, err)
	}
	if _, err := tlsconfig.ParseCipherSuites(cipherSuites); err != nil {
		return fmt.Errorf("%s_cipher_suites: %w", fieldPrefix, err)
	}
	if _, err := tlsconfig.ParseCurvePreferences(curvePreferences); err != nil {
		return fmt.Errorf("%s_curve_preferences: %w", fieldPrefix, err)
	}
	return nil
}

func validateKafkaConfig(kafkaCfg KafkaConfig) error {
	if len(kafkaCfg.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers must contain at least one broker")
	}

	if kafkaCfg.TLS {
		if strings.TrimSpace(kafkaCfg.TLSCAFile) != "" {
			if err := validateReadableFile(kafkaCfg.TLSCAFile, "kafka.tls_ca_file", "kafka.tls"); err != nil {
				return err
			}
		}
		certFile := strings.TrimSpace(kafkaCfg.TLSCertFile)
		keyFile := strings.TrimSpace(kafkaCfg.TLSKeyFile)
		if certFile != "" || keyFile != "" {
			if certFile == "" || keyFile == "" {
				return fmt.Errorf("kafka.tls_cert_file and kafka.tls_key_file must be configured together")
			}
			if err := validateReadableFile(certFile, "kafka.tls_cert_file", "kafka.tls"); err != nil {
				return err
			}
			if err := validateReadableFile(keyFile, "kafka.tls_key_file", "kafka.tls"); err != nil {
				return err
			}
		}
	} else if strings.TrimSpace(kafkaCfg.TLSCAFile) != "" || strings.TrimSpace(kafkaCfg.TLSCertFile) != "" || strings.TrimSpace(kafkaCfg.TLSKeyFile) != "" || strings.TrimSpace(kafkaCfg.TLSServerName) != "" {
		return fmt.Errorf("kafka TLS files or server name require kafka.tls=true")
	}

	mechanism := strings.ToLower(strings.TrimSpace(kafkaCfg.SASLMechanism))
	switch mechanism {
	case "", "plain", "scram-sha-256", "scram-sha-512":
	default:
		return fmt.Errorf("kafka.sasl_mechanism must be one of plain, scram-sha-256, scram-sha-512")
	}

	if mechanism != "" {
		if kafkaCfg.SASLUsername == "" {
			return fmt.Errorf("kafka.sasl_username is required when kafka.sasl_mechanism is set")
		}
		if kafkaCfg.SASLPassword == "" {
			return fmt.Errorf("kafka.sasl_password is required when kafka.sasl_mechanism is set")
		}
	}

	return nil
}

func validateServerPorts(serverCfg ServerConfig) error {
	ports := []struct {
		name  string
		value int
	}{
		{name: "server.websub_http_port", value: serverCfg.WebSubHTTPPort},
		{name: "server.websub_https_port", value: serverCfg.WebSubHTTPSPort},
		{name: "server.websocket_port", value: serverCfg.WebSocketPort},
		{name: "server.websocket_https_port", value: serverCfg.WebSocketHTTPSPort},
		{name: "server.admin_port", value: serverCfg.AdminPort},
		{name: "server.metrics_port", value: serverCfg.MetricsPort},
	}

	seen := make(map[int]string, len(ports))
	for _, port := range ports {
		if port.value <= 0 {
			return fmt.Errorf("%s must be a positive integer, got %d", port.name, port.value)
		}
		if previous, exists := seen[port.value]; exists {
			return fmt.Errorf("%s port %d conflicts with %s", port.name, port.value, previous)
		}
		seen[port.value] = port.name
	}

	return nil
}

// validateServerTimeouts rejects a zero/negative read/write/idle timeout or
// max-header-byte ceiling on the managed WebSub/WebSocket HTTP(S) servers —
// the zero value for http.Server leaves these unbounded, which is exactly
// the Slowloris-style exposure this configuration exists to close (see
// go-network-service-hardening.md directive 1).
func validateServerTimeouts(serverCfg ServerConfig) error {
	if serverCfg.ReadTimeout <= 0 {
		return fmt.Errorf("server.read_timeout must be positive, got %s", serverCfg.ReadTimeout)
	}
	if serverCfg.WriteTimeout <= 0 {
		return fmt.Errorf("server.write_timeout must be positive, got %s", serverCfg.WriteTimeout)
	}
	if serverCfg.IdleTimeout <= 0 {
		return fmt.Errorf("server.idle_timeout must be positive, got %s", serverCfg.IdleTimeout)
	}
	if serverCfg.MaxHeaderBytes <= 0 {
		return fmt.Errorf("server.max_header_bytes must be positive, got %d", serverCfg.MaxHeaderBytes)
	}
	return nil
}

func validateReadableFile(filePath, fieldName, enabledFieldName string) error {
	trimmedPath := strings.TrimSpace(filePath)
	if trimmedPath == "" {
		return fmt.Errorf("%s is required when %s is true", fieldName, enabledFieldName)
	}

	info, err := os.Stat(trimmedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s file %q does not exist", fieldName, trimmedPath)
		}
		return fmt.Errorf("failed to access %s file %q: %w", fieldName, trimmedPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s path %q must be a file, not a directory", fieldName, trimmedPath)
	}

	fileHandle, err := os.Open(trimmedPath)
	if err != nil {
		return fmt.Errorf("%s file %q is not readable: %w", fieldName, trimmedPath, err)
	}
	if err := fileHandle.Close(); err != nil {
		return fmt.Errorf("failed to close %s file %q after validation: %w", fieldName, trimmedPath, err)
	}

	return nil
}

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

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config is the top-level runtime configuration for the event gateway.
type Config struct {
	Server       ServerConfig       `koanf:"server"`
	Kafka        KafkaConfig        `koanf:"kafka"`
	WebSub       WebSubConfig       `koanf:"websub"`
	PolicyEngine PolicyEngineConfig `koanf:"policy_engine"`
	ControlPlane ControlPlaneConfig `koanf:"controlplane"`
	Logging      LoggingConfig      `koanf:"logging"`
	RuntimeID    string             `koanf:"runtime_id"`
}

// ServerConfig holds HTTP/WS server settings.
type ServerConfig struct {
	WebSubEnabled     bool   `koanf:"websub_enabled"`
	WebSubHTTPPort    int    `koanf:"websub_http_port"`
	WebSubHTTPSPort   int    `koanf:"websub_https_port"`
	WebSubTLSEnabled  bool   `koanf:"websub_tls_enabled"`
	WebSubTLSCertFile string `koanf:"websub_tls_cert_file"`
	WebSubTLSKeyFile  string `koanf:"websub_tls_key_file"`
	WebSocketPort     int    `koanf:"websocket_port"`
	AdminPort         int    `koanf:"admin_port"`
	MetricsPort       int    `koanf:"metrics_port"`
}

// KafkaConfig holds Kafka connection settings.
type KafkaConfig struct {
	Brokers             []string `koanf:"brokers"`
	ConsumerGroupPrefix string   `koanf:"consumer_group_prefix"`
	TLS                 bool     `koanf:"tls"`
	SASLMechanism       string   `koanf:"sasl_mechanism"`
	SASLUsername        string   `koanf:"sasl_username"`
	SASLPassword        string   `koanf:"sasl_password"`
}

// WebSubConfig holds WebSub-specific settings.
type WebSubConfig struct {
	VerificationTimeoutSeconds int `koanf:"verification_timeout_seconds"`
	DeliveryMaxRetries         int `koanf:"delivery_max_retries"`
	DeliveryInitialDelayMs     int `koanf:"delivery_initial_delay_ms"`
	DeliveryMaxDelayMs         int `koanf:"delivery_max_delay_ms"`
	DeliveryConcurrency        int `koanf:"delivery_concurrency"`
	DefaultLeaseSeconds        int `koanf:"default_lease_seconds"`
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

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			WebSubEnabled:   true,
			WebSubHTTPPort:  8080,
			WebSubHTTPSPort: 8443,
			WebSocketPort:   8081,
			AdminPort:       9002,
			MetricsPort:     9003,
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
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
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

	// Extract the raw map for policy_configurations (used for ${config} resolution)
	rawConfig := k.All()

	slog.Info("Configuration loaded",
		"websub_enabled", cfg.Server.WebSubEnabled,
		"websub_http_port", cfg.Server.WebSubHTTPPort,
		"websub_https_port", cfg.Server.WebSubHTTPSPort,
		"websub_tls_enabled", cfg.Server.WebSubTLSEnabled,
		"websocket_port", cfg.Server.WebSocketPort,
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
	case "kafka.tls", "controlplane.enabled", "server.websub_enabled", "server.websub_tls_enabled":
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

	if cfg.Server.WebSubTLSEnabled {
		if err := validateReadableFile(cfg.Server.WebSubTLSCertFile, "server.websub_tls_cert_file"); err != nil {
			return err
		}
		if err := validateReadableFile(cfg.Server.WebSubTLSKeyFile, "server.websub_tls_key_file"); err != nil {
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

func validateReadableFile(filePath, fieldName string) error {
	trimmedPath := strings.TrimSpace(filePath)
	if trimmedPath == "" {
		return fmt.Errorf("%s is required when server.websub_tls_enabled is true", fieldName)
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

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
	RuntimeID    string             `koanf:"runtime_id"`
}

// ServerConfig holds HTTP/WS server settings.
type ServerConfig struct {
	WebSubPort    int `koanf:"websub_port"`
	WebSocketPort int `koanf:"websocket_port"`
	AdminPort     int `koanf:"admin_port"`
	MetricsPort   int `koanf:"metrics_port"`
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

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			WebSubPort:    8080,
			WebSocketPort: 8081,
			AdminPort:     9002,
			MetricsPort:   9003,
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
	}
}

// Load loads configuration from a TOML file and environment variables.
// Environment variables use the prefix APIP_EGW_ and replace dots with underscores.
func Load(path string) (*Config, map[string]interface{}, error) {
	k := koanf.New(".")
	cfg := DefaultConfig()

	if path != "" {
		if err := k.Load(file.Provider(path), toml.Parser()); err != nil {
			return nil, nil, fmt.Errorf("failed to load config file %s: %w", path, err)
		}
	}

	// Load environment variable overrides
	if err := k.Load(env.Provider("APIP_EGW_", ".", func(s string) string {
		return strings.Replace(
			strings.ToLower(strings.TrimPrefix(s, "APIP_EGW_")),
			"_", ".", -1)
	}), nil); err != nil {
		return nil, nil, fmt.Errorf("failed to load env vars: %w", err)
	}

	if err := k.Unmarshal("", cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Extract the raw map for policy_configurations (used for ${config} resolution)
	rawConfig := k.All()

	slog.Info("Configuration loaded",
		"websub_port", cfg.Server.WebSubPort,
		"websocket_port", cfg.Server.WebSocketPort,
		"admin_port", cfg.Server.AdminPort,
		"kafka_brokers", cfg.Kafka.Brokers,
	)

	return cfg, rawConfig, nil
}

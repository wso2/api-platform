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

package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"github.com/wso2/api-platform/event-gateway/gateway-runtime/internal/config"
)

// ConnectionConfig holds the Kafka connection settings used by the driver.
type ConnectionConfig struct {
	Brokers                       []string
	CompactTopicPartitions        int
	CompactTopicReplicationFactor int
	TLS                           bool
	TLSCAFile                     string
	TLSCertFile                   string
	TLSKeyFile                    string
	TLSServerName                 string
	SASLMechanism                 string
	SASLUsername                  string
	SASLPassword                  string
}

// ResolveConnectionConfig merges global runtime config with per-binding overrides.
func ResolveConnectionConfig(global config.KafkaConfig, overrides map[string]interface{}) (ConnectionConfig, error) {
	cfg := ConnectionConfig{
		Brokers:                       append([]string(nil), global.Brokers...),
		CompactTopicPartitions:        global.CompactTopicPartitions,
		CompactTopicReplicationFactor: global.CompactTopicReplicationFactor,
		TLS:                           global.TLS,
		TLSCAFile:                     global.TLSCAFile,
		TLSCertFile:                   global.TLSCertFile,
		TLSKeyFile:                    global.TLSKeyFile,
		TLSServerName:                 global.TLSServerName,
		SASLMechanism:                 global.SASLMechanism,
		SASLUsername:                  global.SASLUsername,
		SASLPassword:                  global.SASLPassword,
	}
	if cfg.CompactTopicPartitions <= 0 {
		return ConnectionConfig{}, fmt.Errorf("kafka.compact_topic_partitions must be a positive integer, got %d", cfg.CompactTopicPartitions)
	}
	if cfg.CompactTopicReplicationFactor <= 0 {
		return ConnectionConfig{}, fmt.Errorf("kafka.compact_topic_replication_factor must be a positive integer, got %d", cfg.CompactTopicReplicationFactor)
	}

	if overrides != nil {
		if brokers, ok, err := stringSliceOverride(overrides["brokers"]); err != nil {
			return ConnectionConfig{}, err
		} else if ok {
			cfg.Brokers = brokers
		}
		if v, ok, err := intOverride(overrides["compact_topic_partitions"]); err != nil {
			return ConnectionConfig{}, err
		} else if ok {
			cfg.CompactTopicPartitions = v
		}
		if v, ok, err := intOverride(overrides["compact_topic_replication_factor"]); err != nil {
			return ConnectionConfig{}, err
		} else if ok {
			cfg.CompactTopicReplicationFactor = v
		}
		if v, ok, err := boolOverride(overrides["tls"]); err != nil {
			return ConnectionConfig{}, err
		} else if ok {
			cfg.TLS = v
		}
		if v, ok, err := stringOverride(overrides["tls_ca_file"]); err != nil {
			return ConnectionConfig{}, err
		} else if ok {
			cfg.TLSCAFile = v
		}
		if v, ok, err := stringOverride(overrides["tls_cert_file"]); err != nil {
			return ConnectionConfig{}, err
		} else if ok {
			cfg.TLSCertFile = v
		}
		if v, ok, err := stringOverride(overrides["tls_key_file"]); err != nil {
			return ConnectionConfig{}, err
		} else if ok {
			cfg.TLSKeyFile = v
		}
		if v, ok, err := stringOverride(overrides["tls_server_name"]); err != nil {
			return ConnectionConfig{}, err
		} else if ok {
			cfg.TLSServerName = v
		}
		if v, ok, err := stringOverride(overrides["sasl_mechanism"]); err != nil {
			return ConnectionConfig{}, err
		} else if ok {
			cfg.SASLMechanism = v
		}
		if v, ok, err := stringOverride(overrides["sasl_username"]); err != nil {
			return ConnectionConfig{}, err
		} else if ok {
			cfg.SASLUsername = v
		}
		if v, ok, err := stringOverride(overrides["sasl_password"]); err != nil {
			return ConnectionConfig{}, err
		} else if ok {
			cfg.SASLPassword = v
		}
	}

	normalizeConnectionConfig(&cfg)
	if err := validateConnectionConfig(cfg); err != nil {
		return ConnectionConfig{}, err
	}
	return cfg, nil
}

func normalizeConnectionConfig(cfg *ConnectionConfig) {
	normalizedBrokers := make([]string, 0, len(cfg.Brokers))
	for _, broker := range cfg.Brokers {
		trimmed := strings.TrimSpace(broker)
		if trimmed == "" {
			continue
		}
		normalizedBrokers = append(normalizedBrokers, trimmed)
	}
	cfg.Brokers = normalizedBrokers
	cfg.SASLMechanism = strings.ToLower(strings.TrimSpace(cfg.SASLMechanism))
	cfg.TLSCAFile = strings.TrimSpace(cfg.TLSCAFile)
	cfg.TLSCertFile = strings.TrimSpace(cfg.TLSCertFile)
	cfg.TLSKeyFile = strings.TrimSpace(cfg.TLSKeyFile)
	cfg.TLSServerName = strings.TrimSpace(cfg.TLSServerName)
}

func validateConnectionConfig(cfg ConnectionConfig) error {
	if len(cfg.Brokers) == 0 {
		return fmt.Errorf("kafka brokers must not be empty")
	}
	if cfg.CompactTopicPartitions <= 0 {
		return fmt.Errorf("kafka.compact_topic_partitions must be a positive integer, got %d", cfg.CompactTopicPartitions)
	}
	if cfg.CompactTopicPartitions > math.MaxInt32 {
		return fmt.Errorf("kafka.compact_topic_partitions must be <= %d, got %d", math.MaxInt32, cfg.CompactTopicPartitions)
	}
	if cfg.CompactTopicReplicationFactor <= 0 {
		return fmt.Errorf("kafka.compact_topic_replication_factor must be a positive integer, got %d", cfg.CompactTopicReplicationFactor)
	}
	if cfg.CompactTopicReplicationFactor > math.MaxInt16 {
		return fmt.Errorf("kafka.compact_topic_replication_factor must be <= %d, got %d", math.MaxInt16, cfg.CompactTopicReplicationFactor)
	}

	if !cfg.TLS {
		if cfg.TLSCAFile != "" || cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" || cfg.TLSServerName != "" {
			return fmt.Errorf("kafka TLS files or server name require kafka.tls=true")
		}
	}

	if cfg.TLS {
		if cfg.TLSCAFile != "" {
			if err := validateReadableFile(cfg.TLSCAFile, "kafka.tls_ca_file"); err != nil {
				return err
			}
		}
		if cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
			if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
				return fmt.Errorf("kafka.tls_cert_file and kafka.tls_key_file must be configured together")
			}
			if err := validateReadableFile(cfg.TLSCertFile, "kafka.tls_cert_file"); err != nil {
				return err
			}
			if err := validateReadableFile(cfg.TLSKeyFile, "kafka.tls_key_file"); err != nil {
				return err
			}
		}
	}

	switch cfg.SASLMechanism {
	case "", "plain", "scram-sha-256", "scram-sha-512":
	default:
		return fmt.Errorf("unsupported kafka sasl mechanism %q", cfg.SASLMechanism)
	}

	if cfg.SASLMechanism != "" {
		if cfg.SASLUsername == "" {
			return fmt.Errorf("kafka.sasl_username is required when kafka.sasl_mechanism is set")
		}
		if cfg.SASLPassword == "" {
			return fmt.Errorf("kafka.sasl_password is required when kafka.sasl_mechanism is set")
		}
	}

	return nil
}

// BuildClientOptions returns franz-go client options for the Kafka connection.
func BuildClientOptions(cfg ConnectionConfig, extraOpts ...kgo.Opt) ([]kgo.Opt, error) {
	opts := []kgo.Opt{kgo.SeedBrokers(cfg.Brokers...)}

	if cfg.TLS {
		tlsCfg, err := buildTLSConfig(cfg)
		if err != nil {
			return nil, err
		}
		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}

	if cfg.SASLMechanism != "" {
		mechanism, err := buildSASLMechanism(cfg)
		if err != nil {
			return nil, err
		}
		opts = append(opts, kgo.SASL(mechanism))
	}

	return append(opts, extraOpts...), nil
}

func buildTLSConfig(cfg ConnectionConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: cfg.TLSServerName,
	}

	if cfg.TLSCAFile != "" {
		caPEM, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read kafka TLS CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("failed to parse kafka TLS CA file %q", cfg.TLSCAFile)
		}
		tlsCfg.RootCAs = pool
	}

	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load kafka client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}

func buildSASLMechanism(cfg ConnectionConfig) (sasl.Mechanism, error) {
	switch cfg.SASLMechanism {
	case "plain":
		return plain.Auth{
			User: cfg.SASLUsername,
			Pass: cfg.SASLPassword,
		}.AsMechanism(), nil
	case "scram-sha-256":
		return scram.Auth{
			User: cfg.SASLUsername,
			Pass: cfg.SASLPassword,
		}.AsSha256Mechanism(), nil
	case "scram-sha-512":
		return scram.Auth{
			User: cfg.SASLUsername,
			Pass: cfg.SASLPassword,
		}.AsSha512Mechanism(), nil
	default:
		return nil, fmt.Errorf("unsupported kafka sasl mechanism %q", cfg.SASLMechanism)
	}
}

func boolOverride(value interface{}) (bool, bool, error) {
	if value == nil {
		return false, false, nil
	}
	v, ok := value.(bool)
	if !ok {
		return false, false, fmt.Errorf("expected boolean Kafka config override, got %T", value)
	}
	return v, true, nil
}

func intOverride(value interface{}) (int, bool, error) {
	if value == nil {
		return 0, false, nil
	}
	switch v := value.(type) {
	case int:
		return v, true, nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false, fmt.Errorf("expected integer Kafka config override, got non-finite float64 %v", v)
		}
		if v != math.Trunc(v) {
			return 0, false, fmt.Errorf("expected integer Kafka config override, got non-integer float64 %v", v)
		}
		if v < math.MinInt32 || v > math.MaxInt32 {
			return 0, false, fmt.Errorf("expected integer Kafka config override within [%d, %d], got float64 %v", math.MinInt32, math.MaxInt32, v)
		}
		return int(v), true, nil
	default:
		return 0, false, fmt.Errorf("expected int override, got %T", value)
	}
}

func stringOverride(value interface{}) (string, bool, error) {
	if value == nil {
		return "", false, nil
	}
	v, ok := value.(string)
	if !ok {
		return "", false, fmt.Errorf("expected string Kafka config override, got %T", value)
	}
	return v, true, nil
}

func stringSliceOverride(value interface{}) ([]string, bool, error) {
	if value == nil {
		return nil, false, nil
	}

	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...), true, nil
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, false, fmt.Errorf("expected string broker entry, got %T", item)
			}
			out = append(out, str)
		}
		return out, true, nil
	default:
		return nil, false, fmt.Errorf("expected string slice Kafka config override, got %T", value)
	}
}

func validateReadableFile(filePath, fieldName string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s file %q does not exist", fieldName, filePath)
		}
		return fmt.Errorf("failed to access %s file %q: %w", fieldName, filePath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s path %q must be a file, not a directory", fieldName, filePath)
	}
	fileHandle, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("%s file %q is not readable: %w", fieldName, filePath, err)
	}
	return fileHandle.Close()
}

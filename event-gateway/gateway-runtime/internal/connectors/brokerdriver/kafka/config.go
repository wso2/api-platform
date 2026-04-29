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
	"fmt"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

const (
	SASLMechanismPlain       = "plain"
	SASLMechanismSCRAMSHA256 = "scram-sha-256"
	SASLMechanismSCRAMSHA512 = "scram-sha-512"
)

// ConnectionConfig defines the Kafka connection settings used by the
// broker-driver runtime.
type ConnectionConfig struct {
	Brokers       []string
	TLS           bool
	SASLMechanism string
	SASLUsername  string
	SASLPassword  string
}

// ResolveConnectionConfig merges broker-driver-specific overrides onto runtime
// Kafka defaults, then validates the merged result.
func ResolveConnectionConfig(defaults ConnectionConfig, overrides map[string]interface{}) (ConnectionConfig, error) {
	cfg := normalizeConnectionConfig(defaults)

	if overrides != nil {
		for key, value := range overrides {
			switch key {
			case "brokers":
				brokers, err := parseBrokerList(value)
				if err != nil {
					return ConnectionConfig{}, fmt.Errorf("invalid kafka broker-driver config %q: %w", key, err)
				}
				cfg.Brokers = brokers
			case "tls":
				enabled, ok := value.(bool)
				if !ok {
					return ConnectionConfig{}, fmt.Errorf("invalid kafka broker-driver config %q: expected bool", key)
				}
				cfg.TLS = enabled
			case "sasl_mechanism":
				mechanism, ok := value.(string)
				if !ok {
					return ConnectionConfig{}, fmt.Errorf("invalid kafka broker-driver config %q: expected string", key)
				}
				cfg.SASLMechanism = mechanism
			case "sasl_username":
				username, ok := value.(string)
				if !ok {
					return ConnectionConfig{}, fmt.Errorf("invalid kafka broker-driver config %q: expected string", key)
				}
				cfg.SASLUsername = username
			case "sasl_password":
				password, ok := value.(string)
				if !ok {
					return ConnectionConfig{}, fmt.Errorf("invalid kafka broker-driver config %q: expected string", key)
				}
				cfg.SASLPassword = password
			default:
				return ConnectionConfig{}, fmt.Errorf("unsupported kafka broker-driver config %q", key)
			}
		}
	}

	cfg = normalizeConnectionConfig(cfg)
	if err := ValidateConnectionConfig(cfg); err != nil {
		return ConnectionConfig{}, err
	}
	return cfg, nil
}

// ValidateConnectionConfig validates Kafka connection settings before client
// creation.
func ValidateConnectionConfig(cfg ConnectionConfig) error {
	if len(cfg.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers must contain at least one broker")
	}

	switch cfg.SASLMechanism {
	case "":
		if cfg.SASLUsername != "" || cfg.SASLPassword != "" {
			return fmt.Errorf("kafka SASL credentials require sasl_mechanism to be set")
		}
	case SASLMechanismPlain, SASLMechanismSCRAMSHA256, SASLMechanismSCRAMSHA512:
		if cfg.SASLUsername == "" || cfg.SASLPassword == "" {
			return fmt.Errorf("kafka sasl_mechanism %q requires both sasl_username and sasl_password", cfg.SASLMechanism)
		}
	default:
		return fmt.Errorf("kafka sasl_mechanism must be one of %q, %q, %q", SASLMechanismPlain, SASLMechanismSCRAMSHA256, SASLMechanismSCRAMSHA512)
	}

	return nil
}

// BuildClientOptions converts the typed Kafka connection config into franz-go
// options shared by all Kafka client creation paths in the broker-driver.
func BuildClientOptions(cfg ConnectionConfig, extraOpts ...kgo.Opt) ([]kgo.Opt, error) {
	cfg = normalizeConnectionConfig(cfg)
	if err := ValidateConnectionConfig(cfg); err != nil {
		return nil, err
	}

	opts := make([]kgo.Opt, 0, 3+len(extraOpts))
	opts = append(opts, kgo.SeedBrokers(cfg.Brokers...))

	if cfg.TLS {
		opts = append(opts, kgo.DialTLSConfig(new(tls.Config)))
	}

	if cfg.SASLMechanism != "" {
		auth := scram.Auth{
			User: cfg.SASLUsername,
			Pass: cfg.SASLPassword,
		}
		switch cfg.SASLMechanism {
		case SASLMechanismPlain:
			opts = append(opts, kgo.SASL(plain.Auth{
				User: cfg.SASLUsername,
				Pass: cfg.SASLPassword,
			}.AsMechanism()))
		case SASLMechanismSCRAMSHA256:
			opts = append(opts, kgo.SASL(auth.AsSha256Mechanism()))
		case SASLMechanismSCRAMSHA512:
			opts = append(opts, kgo.SASL(auth.AsSha512Mechanism()))
		}
	}

	opts = append(opts, extraOpts...)
	return opts, nil
}

func normalizeConnectionConfig(cfg ConnectionConfig) ConnectionConfig {
	brokers := make([]string, 0, len(cfg.Brokers))
	for _, broker := range cfg.Brokers {
		broker = strings.TrimSpace(broker)
		if broker == "" {
			continue
		}
		brokers = append(brokers, broker)
	}

	cfg.Brokers = brokers
	cfg.SASLMechanism = strings.ToLower(strings.TrimSpace(cfg.SASLMechanism))
	return cfg
}

func parseBrokerList(value interface{}) ([]string, error) {
	switch v := value.(type) {
	case []string:
		return normalizeConnectionConfig(ConnectionConfig{Brokers: v}).Brokers, nil
	case []interface{}:
		brokers := make([]string, 0, len(v))
		for _, item := range v {
			broker, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("expected brokers to contain only strings")
			}
			brokers = append(brokers, broker)
		}
		return normalizeConnectionConfig(ConnectionConfig{Brokers: brokers}).Brokers, nil
	default:
		return nil, fmt.Errorf("expected []string")
	}
}

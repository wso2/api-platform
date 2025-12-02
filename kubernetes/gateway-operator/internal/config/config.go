/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// OperatorConfig holds the runtime configuration for the operator
type OperatorConfig struct {
	// Gateway configuration
	Gateway GatewayConfig `koanf:"gateway"`

	// Reconciliation configuration
	Reconciliation ReconciliationConfig `koanf:"reconciliation"`

	// Logging configuration
	Logging LoggingConfig `koanf:"logging"`
}

// GatewayConfig holds configuration for gateway deployments
type GatewayConfig struct {
	// Helm-only deployment is supported. Manifest/template fields are removed.

	// HelmChartPath is the path to the Helm chart directory
	HelmChartPath string `koanf:"helm_chart_path"`

	// HelmChartName is the name of the Helm chart (for remote charts)
	HelmChartName string `koanf:"helm_chart_name"`

	// HelmChartVersion is the version of the Helm chart
	HelmChartVersion string `koanf:"helm_chart_version"`

	// HelmValuesFilePath is the path to a custom values.yaml file (optional)
	// If not set, the chart's default values.yaml will be used
	HelmValuesFilePath string `koanf:"helm_values_file_path"`
}

// ReconciliationConfig holds reconciliation loop configuration
type ReconciliationConfig struct {
	// SyncPeriod is the minimum frequency at which watched resources are reconciled
	SyncPeriod time.Duration `koanf:"sync_period"`

	// MaxConcurrentReconciles is the maximum number of concurrent reconciles
	MaxConcurrentReconciles int `koanf:"max_concurrent_reconciles"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	// Level is the log level (debug, info, warn, error)
	Level string `koanf:"level"`

	// Development enables development mode logging
	Development bool `koanf:"development"`
}

// getDefaults returns default configuration as a map
func getDefaults() map[string]interface{} {
	return map[string]interface{}{
		"gateway": map[string]interface{}{
			"helm_chart_name":        "api-platform-gateway",
			"helm_chart_version":     "0.1.0",
			"helm_values_file_path":  "",
		},
		"reconciliation": map[string]interface{}{
			"sync_period":               "10m",
			"max_concurrent_reconciles": 1,
		},
		"logging": map[string]interface{}{
			"level":       "info",
			"development": true,
		},
	}
}

// LoadConfig loads configuration from file, environment variables, and defaults
// Priority: Environment variables > Config file > Defaults
func LoadConfig(configPath string) (*OperatorConfig, error) {
	k := koanf.New(".")

	// Load defaults
	defaults := getDefaults()
	if err := k.Load(confmap.Provider(defaults, "."), nil); err != nil {
		return nil, fmt.Errorf("failed to load defaults: %w", err)
	}

	// Load config file if path is provided and exists
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("failed to load config file %s: %w", configPath, err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to access config file %s: %w", configPath, err)
		}
		// If file doesn't exist, just continue with defaults
	}

	// Load environment variables with prefix "GATEWAY_"
	// Example: GATEWAY_USE_HELM=true -> gateway.use_helm
	//          GATEWAY_HELM_CHART_PATH=/path -> gateway.helm_chart_path
	if err := k.Load(env.Provider("GATEWAY_", ".", func(s string) string {
		// Remove GATEWAY_ prefix
		s = strings.TrimPrefix(s, "GATEWAY_")
		// Convert to lowercase
		s = strings.ToLower(s)

		// Map specific environment variables to config keys
		switch s {
		case "helm_chart_name":
			return "gateway.helm_chart_name"
		case "helm_chart_version":
			return "gateway.helm_chart_version"
		case "helm_values_file_path":
			return "gateway.helm_values_file_path"
		default:
			// For other vars, use standard mapping (underscore to dot)
			s = strings.ReplaceAll(s, "_", ".")
			return s
		}
	}), nil); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Unmarshal into OperatorConfig struct
	var cfg OperatorConfig
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Validate validates the configuration
func (c *OperatorConfig) Validate() error {
	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}

	// Validate reconciliation config
	if c.Reconciliation.MaxConcurrentReconciles < 1 {
		return fmt.Errorf("max concurrent reconciles must be at least 1")
	}

	return nil
}

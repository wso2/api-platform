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
	"path/filepath"
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
	Gateway GatewayConfig

	// Reconciliation configuration
	Reconciliation ReconciliationConfig

	// Logging configuration
	Logging LoggingConfig
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

	// ControlPlaneHost is the gateway control plane host
	ControlPlaneHost string `koanf:"controlplane_host"`

	// ControlPlaneToken is the authentication token for control plane
	ControlPlaneToken string `koanf:"controlplane_token"`

	// StorageType defines the storage backend (sqlite, postgres, etc.)
	StorageType string `koanf:"storage_type"`

	// StorageSQLitePath is the path for SQLite database
	StorageSQLitePath string `koanf:"storage_sqlite_path"`

	// DefaultImage is the default gateway controller image
	DefaultImage string `koanf:"default_image"`

	// DefaultRouterImage is the default router image
	DefaultRouterImage string `koanf:"default_router_image"`

	// DefaultReplicas is the default number of replicas
	DefaultReplicas int32 `koanf:"default_replicas"`
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

// NewDefaultConfig returns a new OperatorConfig with default values
func NewDefaultConfig() *OperatorConfig {
	return &OperatorConfig{
		Gateway: GatewayConfig{
			HelmChartPath:      "../helm/gateway-helm-chart",
			HelmChartName:      "api-platform-gateway",
			HelmChartVersion:   "0.1.0",
			HelmValuesFilePath: "", // Empty means use chart's default values.yaml
			ControlPlaneHost:   "host.docker.internal:8443",
			ControlPlaneToken:  "",
			StorageType:        "sqlite",
			StorageSQLitePath:  "./data/gateway.db",
			DefaultImage:       "wso2/gateway-controller:latest",
			DefaultRouterImage: "wso2/gateway-router:latest",
			DefaultReplicas:    1,
		},
		Reconciliation: ReconciliationConfig{
			SyncPeriod:              10 * time.Minute,
			MaxConcurrentReconciles: 1,
		},
		Logging: LoggingConfig{
			Level:       "info",
			Development: true,
		},
	}
}

// getDefaults returns default configuration as a map
func getDefaults() map[string]interface{} {
	return map[string]interface{}{
		"gateway": map[string]interface{}{
			"manifest_path":          "internal/controller/resources/api-platform-gateway-k8s-manifests.yaml",
			"manifest_template_path": "internal/controller/resources/api-platform-gateway-k8s-manifests.yaml.tmpl",
			"use_helm":               true,
			"helm_chart_path":        "../helm/gateway-helm-chart",
			"helm_chart_name":        "api-platform-gateway",
			"helm_chart_version":     "0.1.0",
			"helm_values_file_path":  "",
			"controlplane_host":      "host.docker.internal:8443",
			"controlplane_token":     "",
			"storage_type":           "sqlite",
			"storage_sqlite_path":    "./data/gateway.db",
			"default_image":          "wso2/gateway-controller:latest",
			"default_router_image":   "wso2/gateway-router:latest",
			"default_replicas":       1,
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
		case "manifest_path":
			return "gateway.manifest_path"
		case "manifest_template_path":
			return "gateway.manifest_template_path"
		case "use_helm":
			return "gateway.use_helm"
		case "helm_chart_path":
			return "gateway.helm_chart_path"
		case "helm_chart_name":
			return "gateway.helm_chart_name"
		case "helm_chart_version":
			return "gateway.helm_chart_version"
		case "helm_values_file_path":
			return "gateway.helm_values_file_path"
		case "controlplane_host":
			return "gateway.controlplane_host"
		case "controlplane_token":
			return "gateway.controlplane_token"
		case "storage_type":
			return "gateway.storage_type"
		case "storage_sqlite_path":
			return "gateway.storage_sqlite_path"
		case "default_image":
			return "gateway.default_image"
		case "router_image":
			return "gateway.default_router_image"
		case "log_level":
			return "logging.level"
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
	// Validate manifest path exists
	if c.Gateway.ManifestPath != "" {
		absPath, err := filepath.Abs(c.Gateway.ManifestPath)
		if err != nil {
			return fmt.Errorf("invalid manifest path: %w", err)
		}

		if _, err := os.Stat(absPath); err != nil {
			return fmt.Errorf("manifest file not found at %s: %w", absPath, err)
		}

		// Update to absolute path
		c.Gateway.ManifestPath = absPath
	}

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

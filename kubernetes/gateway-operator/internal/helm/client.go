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

package helm

import (
	"context"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Client provides methods to install, upgrade, and uninstall Helm charts
type Client struct {
	settings       *cli.EnvSettings
	registryClient *registry.Client
}

// NewClient creates a new Helm client
func NewClient() (*Client, error) {
	return NewClientWithOptions(false)
}

// NewClientWithOptions creates a new Helm client with custom options
func NewClientWithOptions(plainHTTP bool) (*Client, error) {
	settings := cli.New()

	// Build registry client options
	opts := []registry.ClientOption{
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptEnableCache(true),
		registry.ClientOptWriter(os.Stderr),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	}

	// Add PlainHTTP option if requested
	if plainHTTP {
		opts = append(opts, registry.ClientOptPlainHTTP())
	}

	// Create registry client for OCI support
	registryClient, err := registry.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	return &Client{
		settings:       settings,
		registryClient: registryClient,
	}, nil
}

// InstallOrUpgradeOptions contains options for installing or upgrading a Helm chart
type InstallOrUpgradeOptions struct {
	// ReleaseName is the name of the Helm release
	ReleaseName string

	// Namespace is the Kubernetes namespace to install the chart into
	Namespace string

	// ChartName is the name of the remote chart (e.g., "bitnami/nginx" or "oci://registry/chart")
	ChartName string

	// RepoURL is the Helm repository URL (optional if repo is already added)
	RepoURL string

	// Version is the chart version (optional, uses latest if not specified)
	Version string

	// ValuesYAML is the values.yaml content as a string
	ValuesYAML string

	// ValuesFilePath is the path to a values.yaml file
	ValuesFilePath string

	// CreateNamespace creates the namespace if it doesn't exist
	CreateNamespace bool

	// Wait waits for all resources to be ready
	Wait bool

	// Timeout for the operation in seconds
	Timeout int64

	// Username for registry authentication (optional)
	Username string

	// Password for registry authentication (optional)
	Password string

	// Insecure allows insecure registry connections (skips TLS verification, still uses HTTPS)
	Insecure bool

	// PlainHTTP forces plain HTTP instead of HTTPS for OCI registries
	PlainHTTP bool
}

// UninstallOptions contains options for uninstalling a Helm release
type UninstallOptions struct {
	// ReleaseName is the name of the Helm release to uninstall
	ReleaseName string

	// Namespace is the Kubernetes namespace of the release
	Namespace string

	// Wait waits for all resources to be deleted
	Wait bool

	// Timeout for the operation in seconds
	Timeout int64
}

// InstallOrUpgrade installs a new Helm chart or upgrades an existing release
func (c *Client) InstallOrUpgrade(ctx context.Context, opts InstallOrUpgradeOptions) error {
	log := log.FromContext(ctx)

	log.Info("Installing or upgrading Helm chart",
		"release", opts.ReleaseName,
		"namespace", opts.Namespace,
		"chart", opts.ChartName)

	// Create action configuration
	actionConfig, err := c.newActionConfig(opts.Namespace)
	if err != nil {
		return fmt.Errorf("failed to create action configuration: %w", err)
	}

	// Set registry client in action configuration
	actionConfig.RegistryClient = c.registryClient

	// Check if release exists
	histClient := action.NewHistory(actionConfig)
	histClient.Max = 1
	_, err = histClient.Run(opts.ReleaseName)
	releaseExists := err == nil

	if releaseExists {
		log.Info("Release exists, performing upgrade", "release", opts.ReleaseName)
		return c.upgrade(ctx, actionConfig, opts)
	}

	log.Info("Release does not exist, performing install", "release", opts.ReleaseName)
	return c.install(ctx, actionConfig, opts)
}

// install performs a Helm install
func (c *Client) install(ctx context.Context, actionConfig *action.Configuration, opts InstallOrUpgradeOptions) error {
	log := log.FromContext(ctx)

	client := action.NewInstall(actionConfig)
	client.ReleaseName = opts.ReleaseName
	client.Namespace = opts.Namespace
	client.CreateNamespace = opts.CreateNamespace
	client.Wait = opts.Wait
	client.Version = opts.Version

	if opts.Timeout > 0 {
		client.Timeout = time.Duration(opts.Timeout) * time.Second
	}

	// Handle registry authentication if provided
	if opts.Username != "" && opts.Password != "" {
		registryHost, err := extractRegistryHost(opts.ChartName)
		if err != nil {
			return fmt.Errorf("failed to extract registry host: %w", err)
		}

		if err := c.registryClient.Login(
			registryHost,
			registry.LoginOptBasicAuth(opts.Username, opts.Password),
			registry.LoginOptInsecure(opts.Insecure),
		); err != nil {
			return fmt.Errorf("failed to login to registry: %w", err)
		}
	}

	// Locate and load chart from remote repository or OCI registry
	chartPath, err := client.ChartPathOptions.LocateChart(opts.ChartName, c.settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart %s: %w", opts.ChartName, err)
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	// Parse values
	values, err := c.parseValues(opts)
	if err != nil {
		return fmt.Errorf("failed to parse values: %w", err)
	}

	// Install the chart
	release, err := client.Run(chart, values)
	if err != nil {
		return fmt.Errorf("failed to install chart: %w", err)
	}

	log.Info("Successfully installed chart",
		"release", release.Name,
		"version", release.Chart.Metadata.Version,
		"status", release.Info.Status)

	return nil
}

// upgrade performs a Helm upgrade
func (c *Client) upgrade(ctx context.Context, actionConfig *action.Configuration, opts InstallOrUpgradeOptions) error {
	log := log.FromContext(ctx)

	client := action.NewUpgrade(actionConfig)
	client.Namespace = opts.Namespace
	client.Wait = opts.Wait
	client.Version = opts.Version

	if opts.Timeout > 0 {
		client.Timeout = time.Duration(opts.Timeout) * time.Second
	}

	// Handle registry authentication if provided
	if opts.Username != "" && opts.Password != "" {
		registryHost, err := extractRegistryHost(opts.ChartName)
		if err != nil {
			return fmt.Errorf("failed to extract registry host: %w", err)
		}

		if err := c.registryClient.Login(
			registryHost,
			registry.LoginOptBasicAuth(opts.Username, opts.Password),
			registry.LoginOptInsecure(opts.Insecure),
		); err != nil {
			return fmt.Errorf("failed to login to registry: %w", err)
		}
	}

	// Locate and load chart from remote repository or OCI registry
	chartPath, err := client.ChartPathOptions.LocateChart(opts.ChartName, c.settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart %s: %w", opts.ChartName, err)
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	// Parse values
	values, err := c.parseValues(opts)
	if err != nil {
		return fmt.Errorf("failed to parse values: %w", err)
	}

	// Upgrade the chart
	release, err := client.Run(opts.ReleaseName, chart, values)
	if err != nil {
		return fmt.Errorf("failed to upgrade chart: %w", err)
	}

	log.Info("Successfully upgraded chart",
		"release", release.Name,
		"version", release.Chart.Metadata.Version,
		"status", release.Info.Status)

	return nil
}

// Uninstall uninstalls a Helm release
func (c *Client) Uninstall(ctx context.Context, opts UninstallOptions) error {
	log := log.FromContext(ctx)

	log.Info("Uninstalling Helm release",
		"release", opts.ReleaseName,
		"namespace", opts.Namespace)

	// Create action configuration
	actionConfig, err := c.newActionConfig(opts.Namespace)
	if err != nil {
		return fmt.Errorf("failed to create action configuration: %w", err)
	}

	// Check if release exists
	histClient := action.NewHistory(actionConfig)
	histClient.Max = 1
	_, err = histClient.Run(opts.ReleaseName)
	if err == driver.ErrReleaseNotFound {
		log.Info("Release not found, nothing to uninstall", "release", opts.ReleaseName)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check release existence: %w", err)
	}

	// Uninstall the release
	client := action.NewUninstall(actionConfig)
	client.Wait = opts.Wait

	if opts.Timeout > 0 {
		client.Timeout = time.Duration(opts.Timeout) * time.Second
	}

	response, err := client.Run(opts.ReleaseName)
	if err != nil {
		return fmt.Errorf("failed to uninstall release: %w", err)
	}

	log.Info("Successfully uninstalled release",
		"release", opts.ReleaseName,
		"info", response.Info)

	return nil
}

// newActionConfig creates a new Helm action configuration
func (c *Client) newActionConfig(namespace string) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)

	// Create a config flags instance
	configFlags := genericclioptions.NewConfigFlags(true)
	configFlags.Namespace = &namespace

	// Initialize the action configuration
	if err := actionConfig.Init(configFlags, namespace, "secret", func(format string, v ...interface{}) {
		// Use structured logging instead of printf
		log.Log.Info(fmt.Sprintf(format, v...))
	}); err != nil {
		return nil, err
	}

	return actionConfig, nil
}

// GetReleaseName generates a release name from a gateway name
func GetReleaseName(gatewayName string) string {
	// Helm release names must be DNS-compliant
	// Gateway name is already validated as a Kubernetes resource name
	return fmt.Sprintf("%s-gateway", gatewayName)
}

// parseValues parses values from YAML string or file
func (c *Client) parseValues(opts InstallOrUpgradeOptions) (map[string]interface{}, error) {
	values := make(map[string]interface{})

	// Load from file if specified
	if opts.ValuesFilePath != "" {
		data, err := os.ReadFile(opts.ValuesFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read values file: %w", err)
		}

		var rawValues interface{}
		if err := yaml.Unmarshal(data, &rawValues); err != nil {
			return nil, fmt.Errorf("failed to parse values file: %w", err)
		}

		// Convert to map[string]interface{} for JSON compatibility
		converted, err := convertToStringMap(rawValues)
		if err != nil {
			return nil, fmt.Errorf("failed to convert values from file: %w", err)
		}

		if convertedMap, ok := converted.(map[string]interface{}); ok {
			values = convertedMap
		}
	}

	// Override with inline YAML if specified
	if opts.ValuesYAML != "" {
		var rawValues interface{}
		if err := yaml.Unmarshal([]byte(opts.ValuesYAML), &rawValues); err != nil {
			return nil, fmt.Errorf("failed to parse values YAML: %w", err)
		}

		// Convert to map[string]interface{} for JSON compatibility
		converted, err := convertToStringMap(rawValues)
		if err != nil {
			return nil, fmt.Errorf("failed to convert inline values: %w", err)
		}

		// Merge inline values (they take precedence)
		if inlineValues, ok := converted.(map[string]interface{}); ok {
			for k, v := range inlineValues {
				values[k] = v
			}
		}
	}

	return values, nil
}

// convertToStringMap recursively converts map[interface{}]interface{} to map[string]interface{}
// This is necessary because yaml.v2 creates map[interface{}]interface{} which is not JSON-serializable
func convertToStringMap(input interface{}) (interface{}, error) {
	switch value := input.(type) {
	case map[interface{}]interface{}:
		// Convert map[interface{}]interface{} to map[string]interface{}
		result := make(map[string]interface{})
		for k, v := range value {
			keyStr, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("non-string key found: %v", k)
			}
			converted, err := convertToStringMap(v)
			if err != nil {
				return nil, err
			}
			result[keyStr] = converted
		}
		return result, nil

	case map[string]interface{}:
		// Already the correct type, but recurse into values
		result := make(map[string]interface{})
		for k, v := range value {
			converted, err := convertToStringMap(v)
			if err != nil {
				return nil, err
			}
			result[k] = converted
		}
		return result, nil

	case []interface{}:
		// Handle slices recursively
		result := make([]interface{}, len(value))
		for i, v := range value {
			converted, err := convertToStringMap(v)
			if err != nil {
				return nil, err
			}
			result[i] = converted
		}
		return result, nil

	default:
		// Primitive types (string, int, bool, etc.) are returned as-is
		return value, nil
	}
}

// extractRegistryHost extracts the registry hostname from an OCI chart reference
// Example: "oci://registry-1.docker.io/tharsanan/api-platform-gateway" -> "registry-1.docker.io"
func extractRegistryHost(chartRef string) (string, error) {
	// Remove "oci://" prefix if present
	if len(chartRef) > 6 && chartRef[:6] == "oci://" {
		chartRef = chartRef[6:]
	}

	// Find the first slash to get the hostname
	for i, c := range chartRef {
		if c == '/' {
			return chartRef[:i], nil
		}
	}

	// If no slash found, the entire string is the host
	if chartRef != "" {
		return chartRef, nil
	}

	return "", fmt.Errorf("invalid chart reference: %s", chartRef)
}

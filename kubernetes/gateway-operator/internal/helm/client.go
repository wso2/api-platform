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
	"path/filepath"

	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Client provides methods to install, upgrade, and uninstall Helm charts
type Client struct {
	settings *cli.EnvSettings
}

// NewClient creates a new Helm client
func NewClient() *Client {
	return &Client{
		settings: cli.New(),
	}
}

// InstallOrUpgradeOptions contains options for installing or upgrading a Helm chart
type InstallOrUpgradeOptions struct {
	// ReleaseName is the name of the Helm release
	ReleaseName string

	// Namespace is the Kubernetes namespace to install the chart into
	Namespace string

	// ChartPath is the path to the Helm chart (can be local path or chart name)
	ChartPath string

	// ValuesYAML is the values.yaml content as a string
	ValuesYAML string

	// ValuesFilePath is the path to a values.yaml file
	ValuesFilePath string

	// Version is the chart version (optional, for remote charts)
	Version string

	// CreateNamespace creates the namespace if it doesn't exist
	CreateNamespace bool

	// Wait waits for all resources to be ready
	Wait bool

	// Timeout for the operation
	Timeout int64
}

// UninstallOptions contains options for uninstalling a Helm release
type UninstallOptions struct {
	// ReleaseName is the name of the Helm release to uninstall
	ReleaseName string

	// Namespace is the Kubernetes namespace of the release
	Namespace string

	// Wait waits for all resources to be deleted
	Wait bool

	// Timeout for the operation
	Timeout int64
}

// InstallOrUpgrade installs a new Helm chart or upgrades an existing release
func (c *Client) InstallOrUpgrade(ctx context.Context, opts InstallOrUpgradeOptions) error {
	log := log.FromContext(ctx)

	log.Info("Installing or upgrading Helm chart",
		"release", opts.ReleaseName,
		"namespace", opts.Namespace,
		"chart", opts.ChartPath)

	// Create action configuration
	actionConfig, err := c.newActionConfig(opts.Namespace)
	if err != nil {
		return fmt.Errorf("failed to create action configuration: %w", err)
	}

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

	if opts.Timeout > 0 {
		client.Timeout = 0 // Will use default
	}

	// Load chart
	chartPath, err := client.ChartPathOptions.LocateChart(opts.ChartPath, c.settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart: %w", err)
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

	if opts.Timeout > 0 {
		client.Timeout = 0 // Will use default
	}

	// Load chart
	chartPath, err := client.ChartPathOptions.LocateChart(opts.ChartPath, c.settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart: %w", err)
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
		client.Timeout = 0 // Will use default
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

// parseValues parses values from YAML string or file
func (c *Client) parseValues(opts InstallOrUpgradeOptions) (map[string]interface{}, error) {
	values := make(map[string]interface{})

	// Load from file if specified
	if opts.ValuesFilePath != "" {
		data, err := os.ReadFile(opts.ValuesFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read values file: %w", err)
		}

		if err := yaml.Unmarshal(data, &values); err != nil {
			return nil, fmt.Errorf("failed to parse values file: %w", err)
		}
	}

	// Override with inline YAML if specified
	if opts.ValuesYAML != "" {
		inlineValues := make(map[string]interface{})
		if err := yaml.Unmarshal([]byte(opts.ValuesYAML), &inlineValues); err != nil {
			return nil, fmt.Errorf("failed to parse values YAML: %w", err)
		}

		// Merge inline values (they take precedence)
		for k, v := range inlineValues {
			values[k] = v
		}
	}

	return values, nil
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

// WriteValuesFile writes values to a temporary file
func WriteValuesFile(values string, gatewayName string) (string, error) {
	// Create a temporary directory for values files
	tmpDir := filepath.Join(os.TempDir(), "gateway-operator-helm")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Write values to file
	valuesFile := filepath.Join(tmpDir, fmt.Sprintf("%s-values.yaml", gatewayName))
	if err := os.WriteFile(valuesFile, []byte(values), 0644); err != nil {
		return "", fmt.Errorf("failed to write values file: %w", err)
	}

	return valuesFile, nil
}

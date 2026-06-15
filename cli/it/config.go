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

package it

import (
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Suite names used to scope a run to a subset of the test groups.
const (
	SuiteGateway   = "gateway"
	SuiteDevPortal = "devportal"
)

// EnvSuites is the env var that scopes which suites run (comma-separated, e.g.
// "devportal" or "gateway,devportal"). When unset/empty, all suites run.
const EnvSuites = "IT_SUITES"

// SelectedSuites returns the set of suites to run, parsed from IT_SUITES. An
// unset/empty value selects all suites.
func SelectedSuites() map[string]bool {
	raw := strings.TrimSpace(os.Getenv(EnvSuites))
	selected := map[string]bool{}
	if raw == "" {
		selected[SuiteGateway] = true
		selected[SuiteDevPortal] = true
		return selected
	}
	for _, part := range strings.Split(raw, ",") {
		if name := strings.ToLower(strings.TrimSpace(part)); name != "" {
			selected[name] = true
		}
	}
	return selected
}

// FeaturePaths returns the gherkin feature directories for the selected suites.
func FeaturePaths() []string {
	selected := SelectedSuites()
	var paths []string
	if selected[SuiteGateway] {
		paths = append(paths, "features/gateway")
	}
	if selected[SuiteDevPortal] {
		paths = append(paths, "features/devportal")
	}
	if len(paths) == 0 {
		paths = []string{"features"}
	}
	return paths
}

// TestConfig represents the test configuration file structure
type TestConfig struct {
	Infrastructure InfrastructureConfig `yaml:"infrastructure"`
	Tests          TestsConfig          `yaml:"tests"`
}

// InfrastructureConfig holds infrastructure-related configuration
type InfrastructureConfig struct {
	ComposeFile         string `yaml:"compose_file"`
	StartupTimeout      string `yaml:"startup_timeout"`
	HealthCheckInterval string `yaml:"health_check_interval"`
	DockerRegistry      string `yaml:"docker_registry"`
	ImageTag            string `yaml:"image_tag"`
}

// TestsConfig holds all test group configurations
type TestsConfig struct {
	Gateway   GatewayTestsConfig   `yaml:"gateway"`
	DevPortal DevPortalTestsConfig `yaml:"devportal"`
}

// GatewayTestsConfig holds gateway-related test configurations
type GatewayTestsConfig struct {
	Manage []TestDefinition `yaml:"manage"`
	Apply  []TestDefinition `yaml:"apply"`
	API    []TestDefinition `yaml:"api"`
	MCP    []TestDefinition `yaml:"mcp"`
	Build  []TestDefinition `yaml:"build"`
}

// DevPortalTestsConfig holds developer-portal-related test configurations
type DevPortalTestsConfig struct {
	Manage       []TestDefinition `yaml:"manage"`
	Organization []TestDefinition `yaml:"organization"`
	API          []TestDefinition `yaml:"api"`
	Subscription []TestDefinition `yaml:"subscription"`
	APIKey       []TestDefinition `yaml:"apikey"`
	Application  []TestDefinition `yaml:"application"`
	E2E          []TestDefinition `yaml:"e2e"`
}

// TestDefinition represents a single test definition
type TestDefinition struct {
	ID       string   `yaml:"id"`
	Name     string   `yaml:"name"`
	Enabled  bool     `yaml:"enabled"`
	Requires []string `yaml:"requires"`
}

// InfrastructureID represents infrastructure component identifiers
type InfrastructureID string

const (
	// InfraCLI represents the CLI binary
	InfraCLI InfrastructureID = "CLI"
	// InfraGateway represents the gateway stack (running containers)
	InfraGateway InfrastructureID = "GATEWAY"
	// InfraMCPServer represents the MCP server
	InfraMCPServer InfrastructureID = "MCP_SERVER"
	// InfraDevPortal represents the developer portal stack (postgres + devportal)
	InfraDevPortal InfrastructureID = "DEVPORTAL"
)

// LoadTestConfig loads the test configuration from a YAML file
func LoadTestConfig(path string) (*TestConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config TestConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// GetEnabledTests returns all enabled test definitions for the selected suites.
func (c *TestConfig) GetEnabledTests() []TestDefinition {
	selected := SelectedSuites()
	var enabled []TestDefinition

	if selected[SuiteGateway] {
		for _, group := range c.gatewayGroups() {
			for _, t := range group {
				if t.Enabled {
					enabled = append(enabled, t)
				}
			}
		}
	}
	if selected[SuiteDevPortal] {
		for _, group := range c.devPortalGroups() {
			for _, t := range group {
				if t.Enabled {
					enabled = append(enabled, t)
				}
			}
		}
	}

	return enabled
}

// gatewayGroups returns all gateway test groups for iteration.
func (c *TestConfig) gatewayGroups() [][]TestDefinition {
	return [][]TestDefinition{
		c.Tests.Gateway.Manage,
		c.Tests.Gateway.Apply,
		c.Tests.Gateway.API,
		c.Tests.Gateway.MCP,
		c.Tests.Gateway.Build,
	}
}

// devPortalGroups returns all developer-portal test groups for iteration.
func (c *TestConfig) devPortalGroups() [][]TestDefinition {
	return [][]TestDefinition{
		c.Tests.DevPortal.Manage,
		c.Tests.DevPortal.Organization,
		c.Tests.DevPortal.API,
		c.Tests.DevPortal.Subscription,
		c.Tests.DevPortal.APIKey,
		c.Tests.DevPortal.Application,
		c.Tests.DevPortal.E2E,
	}
}

// GetAllTests returns all test definitions regardless of enabled status
func (c *TestConfig) GetAllTests() []TestDefinition {
	var all []TestDefinition

	all = append(all, c.Tests.Gateway.Manage...)
	all = append(all, c.Tests.Gateway.Apply...)
	all = append(all, c.Tests.Gateway.API...)
	all = append(all, c.Tests.Gateway.MCP...)
	all = append(all, c.Tests.Gateway.Build...)
	for _, group := range c.devPortalGroups() {
		all = append(all, group...)
	}

	return all
}

// GetRequiredInfrastructure returns unique infrastructure IDs required by enabled tests
func (c *TestConfig) GetRequiredInfrastructure() []InfrastructureID {
	required := make(map[InfrastructureID]bool)

	for _, test := range c.GetEnabledTests() {
		for _, req := range test.Requires {
			required[InfrastructureID(req)] = true
		}
	}

	order := []InfrastructureID{
		InfraCLI,
		InfraGateway,
		InfraMCPServer,
		InfraDevPortal,
	}

	var result []InfrastructureID
	for _, id := range order {
		if required[id] {
			result = append(result, id)
			delete(required, id)
		}
	}

	// Append any remaining IDs deterministically (sorted by string).
	if len(required) > 0 {
		var others []InfrastructureID
		for id := range required {
			others = append(others, id)
		}
		sort.Slice(others, func(i, j int) bool { return string(others[i]) < string(others[j]) })
		result = append(result, others...)
	}

	return result
}

// IsTestEnabled checks if a test with the given ID is enabled
func (c *TestConfig) IsTestEnabled(testID string) bool {
	for _, test := range c.GetAllTests() {
		if test.ID == testID {
			return test.Enabled
		}
	}
	return false
}

// GetTestByID returns the test definition for a given ID
func (c *TestConfig) GetTestByID(testID string) *TestDefinition {
	for _, test := range c.GetAllTests() {
		if test.ID == testID {
			return &test
		}
	}
	return nil
}

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

	"gopkg.in/yaml.v3"
)

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
	Gateway GatewayTestsConfig `yaml:"gateway"`
}

// GatewayTestsConfig holds gateway-related test configurations
type GatewayTestsConfig struct {
	Manage []TestDefinition `yaml:"manage"`
	Apply  []TestDefinition `yaml:"apply"`
	API    []TestDefinition `yaml:"api"`
	MCP    []TestDefinition `yaml:"mcp"`
	Build  []TestDefinition `yaml:"build"`
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

// GetEnabledTests returns all enabled test definitions
func (c *TestConfig) GetEnabledTests() []TestDefinition {
	var enabled []TestDefinition

	// Collect from all test groups
	for _, t := range c.Tests.Gateway.Manage {
		if t.Enabled {
			enabled = append(enabled, t)
		}
	}
	for _, t := range c.Tests.Gateway.Apply {
		if t.Enabled {
			enabled = append(enabled, t)
		}
	}
	for _, t := range c.Tests.Gateway.API {
		if t.Enabled {
			enabled = append(enabled, t)
		}
	}
	for _, t := range c.Tests.Gateway.MCP {
		if t.Enabled {
			enabled = append(enabled, t)
		}
	}
	for _, t := range c.Tests.Gateway.Build {
		if t.Enabled {
			enabled = append(enabled, t)
		}
	}

	return enabled
}

// GetAllTests returns all test definitions regardless of enabled status
func (c *TestConfig) GetAllTests() []TestDefinition {
	var all []TestDefinition

	all = append(all, c.Tests.Gateway.Manage...)
	all = append(all, c.Tests.Gateway.Apply...)
	all = append(all, c.Tests.Gateway.API...)
	all = append(all, c.Tests.Gateway.MCP...)
	all = append(all, c.Tests.Gateway.Build...)

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

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

// Package project holds the shared model for an api-platform project on disk:
// the .api-platform/config.yaml schema, the per-portal config layout, and the
// helpers used to scaffold and load them. Keeping these types here lets the
// project, devportal and (future) ai-workspace commands share a single
// source of truth instead of redeclaring the structs in each command package.
package project

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Default version stamped into a freshly scaffolded project config.
const DefaultConfigVersion = "1.0.0"

// Default project-level file paths (relative to the project root). The project
// is used for both APIs and non-API artifacts (MCP proxies, LLM providers),
// so the keys deliberately avoid the "api" prefix.
const (
	DefaultDeploymentArtifact = "./runtime.yaml"
	DefaultMetadataFile       = "./metadata.yaml"
	DefaultDefinition         = "./definition.yaml"
	DefaultDocs               = "./docs"
	DefaultTests              = "./tests"
)

// FilePaths describes where the core project artifacts live, relative to the
// project root.
type FilePaths struct {
	DeploymentArtifact string `yaml:"deploymentArtifact,omitempty"`
	MetadataFile       string `yaml:"metadataFile,omitempty"`
	Definition         string `yaml:"definition,omitempty"`
	Docs               string `yaml:"docs,omitempty"`
	Tests              string `yaml:"tests,omitempty"`
}

// PortalFilePaths describes a devportal's artifact layout, relative to its
// portalRoot.
type PortalFilePaths struct {
	MetadataFile string `yaml:"metadataFile,omitempty"`
	Definition   string `yaml:"definition,omitempty"`
	Docs         string `yaml:"docs,omitempty"`
	Content      string `yaml:"content,omitempty"`
}

// PortalConfig is the layout for a devportal section in the project config.
type PortalConfig struct {
	Name       string          `yaml:"name,omitempty"`
	PortalRoot string          `yaml:"portalRoot,omitempty"`
	FilePaths  PortalFilePaths `yaml:"filePaths,omitempty"`
}

// Default file paths for an ai-workspace portal config, relative to its
// portalRoot.
const (
	DefaultAIWorkspaceMetadata   = "./metadata.yaml"
	DefaultAIWorkspaceRuntime    = "./runtime.yaml"
	DefaultAIWorkspaceDefinition = "./definition.yaml"
	DefaultAIWorkspaceDocs       = "./docs"
)

// AIWorkspaceFilePaths describes an ai-workspace portal's artifact layout,
// relative to its portalRoot. Unlike a devportal it carries a runtime artifact.
// Definition is the optional OpenAPI spec folded into the generated llm-proxy
// payload.
type AIWorkspaceFilePaths struct {
	Metadata   string `yaml:"metadata,omitempty"`
	Runtime    string `yaml:"runtime,omitempty"`
	Definition string `yaml:"definition,omitempty"`
}

// AIWorkspaceConfig is one ai-workspace portal configuration in the project
// config.
type AIWorkspaceConfig struct {
	Name       string               `yaml:"name,omitempty"`
	PortalRoot string               `yaml:"portalRoot,omitempty"`
	FilePaths  AIWorkspaceFilePaths `yaml:"filePaths,omitempty"`
}

// Config is the on-disk .api-platform/config.yaml for an api-platform project.
type Config struct {
	Version            string                 `yaml:"version,omitempty"`
	FilePaths          FilePaths              `yaml:"filePaths,omitempty"`
	GovernanceRulesets []string               `yaml:"governanceRulesets"`
	AutoSync           map[string]interface{} `yaml:"autoSync,omitempty"`
	DevPortals         []PortalConfig         `yaml:"devportals,omitempty"`
	AIWorkspaces       []AIWorkspaceConfig    `yaml:"ai-workspaces,omitempty"`
}

// DefaultFilePaths returns the project-level file paths used when scaffolding a
// new project.
func DefaultFilePaths() FilePaths {
	return FilePaths{
		DeploymentArtifact: DefaultDeploymentArtifact,
		MetadataFile:       DefaultMetadataFile,
		Definition:         DefaultDefinition,
		Docs:               DefaultDocs,
		Tests:              DefaultTests,
	}
}

// Normalize fills any empty project-level file path with its default so callers
// can rely on every path being populated after a load.
func (c *Config) Normalize() {
	defaults := DefaultFilePaths()
	if strings.TrimSpace(c.FilePaths.DeploymentArtifact) == "" {
		c.FilePaths.DeploymentArtifact = defaults.DeploymentArtifact
	}
	if strings.TrimSpace(c.FilePaths.MetadataFile) == "" {
		c.FilePaths.MetadataFile = defaults.MetadataFile
	}
	if strings.TrimSpace(c.FilePaths.Definition) == "" {
		c.FilePaths.Definition = defaults.Definition
	}
	if strings.TrimSpace(c.FilePaths.Docs) == "" {
		c.FilePaths.Docs = defaults.Docs
	}
	if strings.TrimSpace(c.FilePaths.Tests) == "" {
		c.FilePaths.Tests = defaults.Tests
	}
}

// Load reads and parses a project config from configPath, normalizing the file
// paths before returning.
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read project config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse project config: %w", err)
	}

	config.Normalize()
	return &config, nil
}

// Save marshals config back to configPath.
func Save(configPath string, config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save project config: %w", err)
	}

	return nil
}

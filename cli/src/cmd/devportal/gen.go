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
package devportal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/project"
	"github.com/wso2/api-platform/cli/utils"
	"gopkg.in/yaml.v3"
)

const (
	GenCmdLiteral = "gen"
	GenCmdExample = `# Generate the default devportal artifact in the current project
ap devportal gen

# Generate the default devportal artifact for a project in a specified directory
ap devportal gen -f /path/to/project`
)

var genProjectDir string

var genCmd = &cobra.Command{
	Use:     GenCmdLiteral,
	Short:   "Generate the default devportal artifact for the project",
	Long:    "Generate a default devportal artifact (devportal directory with devportal.yaml, definition.yaml, docs and content) inside the project located in the specified directory (or current directory if not specified).",
	Example: GenCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGenCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(genCmd, utils.FlagFile, &genProjectDir, "", "Path to the project directory (defaults to current directory)")
}

// genMetadataResource is the subset of the project metadata.yaml the generated
// devportal manifest is derived from.
type genMetadataResource struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		DisplayName string `yaml:"displayName"`
		Version     string `yaml:"version"`
	} `yaml:"spec"`
}

func runGenCommand() error {
	if genProjectDir == "" {
		genProjectDir = "."
	}

	projectRoot, err := filepath.Abs(genProjectDir)
	if err != nil {
		return fmt.Errorf("failed to resolve project directory: %w", err)
	}

	projectConfigPath, err := resolveProjectConfigPath(projectRoot)
	if err != nil {
		return err
	}

	projectConfig, err := project.Load(projectConfigPath)
	if err != nil {
		return err
	}

	// The default devportal artifact lives in a fixed "devportal" directory at
	// the project root. Refuse to overwrite an existing one so a hand-edited
	// artifact is never clobbered by a regeneration.
	portalRoot := filepath.Join(projectRoot, "devportal")
	if _, err := os.Stat(portalRoot); err == nil {
		return fmt.Errorf("devportal directory already exists: %s", portalRoot)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect devportal directory: %w", err)
	}

	manifest, err := buildGeneratedDevPortalManifest(projectRoot, projectConfig)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(portalRoot, 0755); err != nil {
		return fmt.Errorf("failed to create devportal directory: %w", err)
	}

	// definition.yaml is copied from the project's home definition so the
	// devportal artifact carries the same API definition.
	definitionSource := resolveProjectPath(projectRoot, projectConfig.FilePaths.Definition)
	if _, err := os.Stat(definitionSource); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("unable to find project definition file at %s", definitionSource)
		}
		return fmt.Errorf("failed to inspect project definition file: %w", err)
	}
	if err := copyFile(definitionSource, filepath.Join(portalRoot, "definition.yaml")); err != nil {
		return err
	}

	for _, dir := range []string{"docs", "content"} {
		if err := os.MkdirAll(filepath.Join(portalRoot, dir), 0755); err != nil {
			return fmt.Errorf("failed to create devportal %s directory: %w", dir, err)
		}
	}

	if err := os.WriteFile(filepath.Join(portalRoot, "devportal.yaml"), []byte(manifest), 0644); err != nil {
		return fmt.Errorf("failed to write devportal manifest: %w", err)
	}

	// Register the generated artifact in the project config so `ap devportal
	// build` archives it from the config instead of having to re-detect the
	// folder. Skip if a config entry already points at this folder.
	portalConfig := defaultDevPortalConfig()
	if !hasDevPortalConfig(projectConfig, portalConfig.PortalRoot) {
		projectConfig.DevPortals = append(projectConfig.DevPortals, portalConfig)
		if err := project.Save(projectConfigPath, projectConfig); err != nil {
			return err
		}
	}

	fmt.Printf("DevPortal artifact generated at %s\n", portalRoot)
	return nil
}

// hasDevPortalConfig reports whether config already has a devportal entry whose
// portalRoot resolves to the same relative folder as portalRoot.
func hasDevPortalConfig(config *project.Config, portalRoot string) bool {
	target := normalizePortalRoot(portalRoot)
	for i := range config.DevPortals {
		if normalizePortalRoot(config.DevPortals[i].PortalRoot) == target {
			return true
		}
	}
	return false
}

func normalizePortalRoot(portalRoot string) string {
	trimmed := strings.TrimSpace(portalRoot)
	trimmed = strings.TrimPrefix(trimmed, "./")
	return filepath.Clean(trimmed)
}

// resolveProjectConfigPath validates that projectRoot is an api-platform
// project (it has a .api-platform/config.yaml) and returns the config path.
func resolveProjectConfigPath(projectRoot string) (string, error) {
	projectConfigDir := filepath.Join(projectRoot, ".api-platform")
	if _, err := os.Stat(projectConfigDir); os.IsNotExist(err) {
		return "", fmt.Errorf("unable to find project directory, please execute this command inside a project")
	} else if err != nil {
		return "", fmt.Errorf("failed to inspect project directory: %w", err)
	}

	projectConfigPath := filepath.Join(projectConfigDir, "config.yaml")
	if _, err := os.Stat(projectConfigPath); os.IsNotExist(err) {
		return "", fmt.Errorf("unable to find project directory, please execute this command inside a project")
	} else if err != nil {
		return "", fmt.Errorf("failed to inspect project config: %w", err)
	}

	return projectConfigPath, nil
}

// buildGeneratedDevPortalManifest renders the default devportal.yaml, sourcing
// metadata.name, spec.displayName and spec.version from the project metadata.yaml.
func buildGeneratedDevPortalManifest(projectRoot string, projectConfig *project.Config) (string, error) {
	metadataPath := resolveProjectPath(projectRoot, projectConfig.FilePaths.MetadataFile)
	metadataData, err := os.ReadFile(metadataPath)
	if err != nil {
		return "", fmt.Errorf("failed to read project metadata: %w", err)
	}

	var metadata genMetadataResource
	if err := yaml.Unmarshal(metadataData, &metadata); err != nil {
		return "", fmt.Errorf("failed to parse project metadata: %w", err)
	}

	kind := strings.TrimSpace(metadata.Kind)
	if kind == "" {
		kind = "RestApi"
	}

	return renderGeneratedDevPortalManifest(
		kind,
		strings.TrimSpace(metadata.Metadata.Name),
		strings.TrimSpace(metadata.Spec.DisplayName),
		strings.TrimSpace(metadata.Spec.Version),
	), nil
}

// generatedDevPortalTemplate is the default devportal artifact. The dynamic
// values (kind, metadata.name, spec.displayName, spec.version) come from the
// project metadata.yaml; the remaining fields are sample defaults the user is
// expected to edit.
const generatedDevPortalTemplate = `apiVersion: devportal.api-platform.wso2.com/v1
kind: %s

metadata:
  name: %s

spec:
  type: REST
  displayName: %s
  version: %s
  description:
  provider: WSO2
  gatewayType: wso2/api-platform
  referenceID:

  tags: []

  labels:
    - default

  subscriptionPolicies: []

  visibility: PUBLIC
  visibleGroups: []

  businessInformation:
    businessOwner: Platform Owner
    businessOwnerEmail: support@example.com
    technicalOwner: API Team
    technicalOwnerEmail: architecture@example.com

  endpoints:
    sandboxUrl: http://localhost:8080/booking
    productionUrl: http://localhost:8080/booking
`

func renderGeneratedDevPortalManifest(kind, name, displayName, version string) string {
	return fmt.Sprintf(generatedDevPortalTemplate, kind, name, displayName, version)
}

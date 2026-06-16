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
package apiproject

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	InitCmdLiteral = "init"
	InitCmdExample = `# Initialize a new API project
ap apiproject init --display-name foo-api --type rest

# Add a API project fully interactively cobra
ap apiproject init`
)

var displayName string
var projectType string
var addNoInteractive bool

var initCmd = &cobra.Command{
	Use:     InitCmdLiteral,
	Short:   "Initialize a new API project",
	Long:    "Initialize a new API project with the specified parameters.",
	Example: InitCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runInitCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(initCmd, utils.FlagName, &displayName, "", "Display name of the artifact")
	utils.AddStringFlag(initCmd, utils.FlagType, &projectType, "", "Type of the artifact")
	utils.AddBoolFlag(initCmd, utils.FlagNoInteractive, &addNoInteractive, false, "Skip interactive prompts")
}

func runInitCommand() error {
	var err error
	if !addNoInteractive {
		if strings.TrimSpace(displayName) == "" {
			displayName, err = utils.PromptInput("Enter API Project name: ")
			if err != nil {
				return fmt.Errorf("Failed to read display name: %w", err)
			}
		}
		if strings.TrimSpace(projectType) == "" {
			projectType, err = utils.PromptInput(fmt.Sprintf("Enter artifact type (%s): ", strings.Join(supportedArtifactTypes(), ", ")))
			if err != nil {
				return fmt.Errorf("Failed to read artifact type: %w", err)
			}
		}
	}

	displayName = strings.TrimSpace(displayName)
	projectType = strings.ToLower(strings.TrimSpace(projectType))

	if displayName == "" {
		return fmt.Errorf("display name is required")
	}
	if projectType == "" {
		return fmt.Errorf("artifact type is required")
	}

	if !isValidArtifactType(projectType) {
		return fmt.Errorf("unsupported artifact type: %s (supported types: %s)", projectType, strings.Join(supportedArtifactTypes(), ", "))
	}

	if err := buildDirectoryStructure(displayName, projectType); err != nil {
		return err
	}

	fmt.Printf("Artifact project created at .%c%s\n", os.PathSeparator, displayName)
	return nil
}

func buildDirectoryStructure(name, artifactType string) error {
	if !isValidArtifactType(artifactType) {
		return fmt.Errorf("unsupported artifact type: %s (supported types: %s)", artifactType, strings.Join(supportedArtifactTypes(), ", "))
	}

	projectDirName, err := validateProjectDirectoryName(name)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine current working directory: %w", err)
	}

	projectRoot := filepath.Join(cwd, projectDirName)
	if _, err := os.Stat(projectRoot); err == nil {
		return fmt.Errorf("project directory already exists: %s", projectRoot)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect project directory: %w", err)
	}

	directories := []string{
		filepath.Join(projectRoot, ".api-platform"),
		filepath.Join(projectRoot, "docs"),
		filepath.Join(projectRoot, "tests"),
	}
	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	resourceName := buildResourceName(name)
	files := map[string]string{
		filepath.Join(projectRoot, ".api-platform", "config.yaml"): buildConfigYAML(),
		filepath.Join(projectRoot, "metadata.yaml"):                buildMetadataYAML(resourceName, artifactType),
		filepath.Join(projectRoot, "runtime.yaml"):                 buildRuntimeYAML(resourceName, name, artifactType),
		filepath.Join(projectRoot, "definition.yaml"):              buildDefinitionYAML(name),
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
	}

	return nil
}

func validateProjectDirectoryName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("display name is required")
	}

	if name == "." || name == ".." {
		return "", fmt.Errorf("display name cannot be %q", name)
	}

	if strings.ContainsRune(name, os.PathSeparator) {
		return "", fmt.Errorf("display name cannot contain path separators")
	}

	if os.PathSeparator != '/' && strings.ContainsRune(name, '/') {
		return "", fmt.Errorf("display name cannot contain path separators")
	}

	return name, nil
}

func buildResourceName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = strings.ReplaceAll(normalized, " ", "-")

	invalidChars := regexp.MustCompile(`[^a-z0-9.-]+`)
	repeatedHyphens := regexp.MustCompile(`-+`)

	normalized = invalidChars.ReplaceAllString(normalized, "-")
	normalized = repeatedHyphens.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-.")

	if normalized == "" {
		normalized = "api"
	}

	return fmt.Sprintf("%s", normalized)
}

func buildConfigYAML() string {
	return `version: 1.0.0

# Default file paths (can be customized)
filePaths:
  deploymentArtifact: ./runtime.yaml
  metadataFile: ./metadata.yaml
  apiDefinition: ./definition.yaml
  docs: ./docs
  tests: ./tests

# Governance rulesets for design-time validation
governanceRulesets: []

# Auto-sync configuration for vscode plugin
autoSync:
  gatewayArtifactFromDefinition: true  # Auto-generate gateway.yaml when definition.yaml changes
`
}

func buildMetadataYAML(resourceName, artifactType string) string {
	apiVersion := getApiVersion(artifactType)
	kind := getArtifactKind(artifactType)
	return fmt.Sprintf(`apiVersion: %s
kind: %s
metadata:
  name: %q
spec:
  description: ""
  gatewayType: wso2/api-platform
  status: PUBLISHED
  referenceID: ""
  tags: []
  labels: []
  businessInformation:
    businessOwner: ""
    businessOwnerEmail: ""
    technicalOwner: ""
    technicalOwnerEmail: ""
  endpoints:
    sandboxUrl: ""
    productionUrl: ""
`, apiVersion, kind, resourceName)
}

func buildRuntimeYAML(resourceName, displayName, artifactType string) string {
	kind := getArtifactKind(artifactType)
	return fmt.Sprintf(`apiVersion: gateway.api-platform.wso2.com/v1
kind: %s
metadata:
  name: %q
spec:
  displayName: %q
  version:
  context:
  upstream:
    main:
      url: "http://sample-backend.org:9080"           # Change this to your backend URL
  operations:
    - path: /*
      method: GET
    - path: /*
      method: POST
    - path: /*
      method: PUT
    - path: /*
      method: DELETE
	- path: /*
	  method: OPTIONS
`, kind, resourceName, displayName)
}

func buildDefinitionYAML(displayName string) string {
	return fmt.Sprintf(`openapi: 3.0.3
info:
  title: %q
  version: v1.0
servers:
  - url: https://example.com
paths:
  "/*":
    get:
      responses:
        "200":
          description: OK
    post:
      responses:
        "200":
          description: OK
    put:
      responses:
        "200":
          description: OK
    delete:
      responses:
        "200":
          description: OK
    options:
      responses:
        "200":
          description: OK
`, displayName)
}

func supportedArtifactTypes() []string {
	return []string{
		utils.TypeREST,
		utils.TypeLLMProxy,
		utils.TypeLLMProvider,
		utils.TypeLLMProviderTemplate,
		utils.TypeMCPProxy,
	}
}

func isValidArtifactType(artifactType string) bool {
	for _, t := range supportedArtifactTypes() {
		if artifactType == t {
			return true
		}
	}
	return false
}

func getArtifactKind(artifactType string) string {
	kindMap := map[string]string{
		utils.TypeREST:                    "RestApi",
		utils.TypeLLMProxy:                "LlmProxy",
		utils.TypeLLMProvider:             "LlmProvider",
		utils.TypeLLMProviderTemplate:     "LlmProviderTemplate",
		utils.TypeMCPProxy:                "McpProxy",
	}
	if kind, exists := kindMap[artifactType]; exists {
		return kind
	}
	return "RestApi"
}

func getApiVersion(artifactType string) string {
	switch artifactType {
	case utils.TypeREST:
		return "management.api-platform.wso2.com/v1"
	case utils.TypeLLMProxy, utils.TypeLLMProvider, utils.TypeLLMProviderTemplate, utils.TypeMCPProxy:
		return "ai-workspace.api-platform.wso2.com/v1"
	default:
		return "management.api-platform.wso2.com/v1"
	}
}
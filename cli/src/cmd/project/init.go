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
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/project"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	InitCmdLiteral = "init"
	InitCmdExample = `# Initialize a new project
ap project init --display-name foo-api --type rest

# Initialize a new project interactively
ap project init`
)

var displayName string
var projectType string
var addNoInteractive bool

var initCmd = &cobra.Command{
	Use:     InitCmdLiteral,
	Short:   "Initialize a new project",
	Long:    "Initialize a new project with the specified parameters.",
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
			displayName, err = utils.PromptInput("Enter project name: ")
			if err != nil {
				return fmt.Errorf("Failed to read display name: %w", err)
			}
		}
		if strings.TrimSpace(projectType) == "" {
			projectType, err = utils.PromptSelect("Select artifact type:", project.SupportedArtifactTypes())
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

	if !project.IsValidArtifactType(projectType) {
		return fmt.Errorf("unsupported artifact type: %s (supported types: %s)", projectType, strings.Join(project.SupportedArtifactTypes(), ", "))
	}

	if err := buildDirectoryStructure(displayName, projectType); err != nil {
		return err
	}

	fmt.Printf("Project created at .%c%s\n", os.PathSeparator, displayName)
	return nil
}

func buildDirectoryStructure(name, artifactType string) error {
	if !project.IsValidArtifactType(artifactType) {
		return fmt.Errorf("unsupported artifact type: %s (supported types: %s)", artifactType, strings.Join(project.SupportedArtifactTypes(), ", "))
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

	configYAML, err := project.BuildConfigYAML()
	if err != nil {
		return err
	}
	metadataYAML, err := project.BuildMetadataYAML(artifactType, resourceName, name)
	if err != nil {
		return err
	}
	runtimeYAML, err := project.BuildRuntimeYAML(artifactType, resourceName, name)
	if err != nil {
		return err
	}

	files := map[string]string{
		filepath.Join(projectRoot, ".api-platform", "config.yaml"): configYAML,
		filepath.Join(projectRoot, "metadata.yaml"):                metadataYAML,
		filepath.Join(projectRoot, "runtime.yaml"):                 runtimeYAML,
		filepath.Join(projectRoot, "definition.yaml"):              project.BuildDefinitionYAML(name),
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

	return normalized
}

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

package aiws

import (
	"encoding/json"
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
	BuildCmdLiteral = "build"
	BuildCmdExample = `# Build the AI workspace artifact in the current directory
ap ai-ws build

# Build from a specific project directory
ap ai-ws build -f /path/to/project

# Write the generated payload to a specific directory
ap ai-ws build -o build/

# Write the generated payload to a specific file
ap ai-ws build -o build/openai.json

# Build and fold the OpenAPI spec (definition.yaml) into the payload
ap ai-ws build --use-spec`
)

var (
	buildProjectDir string
	buildOutputDir  string
	buildUseSpec    bool
)

var buildCmd = &cobra.Command{
	Use:   BuildCmdLiteral,
	Short: "Build the project for AI workspace",
	Long: "Build the AI workspace artifact for the project located in the specified directory " +
		"(or current directory if not specified). For each ai-workspace configuration in " +
		".api-platform/config.yaml, the command reads its metadata.yaml and runtime.yaml and generates " +
		"an llm-proxy creation payload as a JSON file. The openapi field is left empty by default; pass " +
		"--use-spec to fold in the OpenAPI spec from definition.yaml when it exists.",
	Example: BuildCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runBuildCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(buildCmd, utils.FlagFile, &buildProjectDir, "", "Path to the project directory (defaults to current directory)")
	utils.AddStringFlag(buildCmd, utils.FlagOutput, &buildOutputDir, "", "Output path: a .json file to write the payload to, or a directory (defaults to the project build directory)")
	utils.AddBoolFlag(buildCmd, utils.FlagUseSpec, &buildUseSpec, false, "Fold the OpenAPI spec (definition.yaml) into the generated payload when it exists")
}

// failedWorkspace records an ai-workspace config that could not be built so the
// others can still be generated and the failures reported together.
type failedWorkspace struct {
	name string
	err  error
}

func runBuildCommand() error {
	if buildProjectDir == "" {
		buildProjectDir = "."
	}

	projectRoot, err := filepath.Abs(buildProjectDir)
	if err != nil {
		return fmt.Errorf("failed to resolve project directory: %w", err)
	}

	projectConfigDir := filepath.Join(projectRoot, ".api-platform")
	if _, err := os.Stat(projectConfigDir); os.IsNotExist(err) {
		return fmt.Errorf("unable to find project directory, please execute this command inside a project")
	} else if err != nil {
		return fmt.Errorf("failed to inspect project directory: %w", err)
	}

	projectConfigPath := filepath.Join(projectConfigDir, "config.yaml")
	if _, err := os.Stat(projectConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("unable to find project directory, please execute this command inside a project")
	} else if err != nil {
		return fmt.Errorf("failed to inspect project config: %w", err)
	}

	projectConfig, err := project.Load(projectConfigPath)
	if err != nil {
		return err
	}

	// Create a default ai-workspace config if none exists and persist it so the
	// project records the configuration that was built.
	if len(projectConfig.AIWorkspaces) == 0 {
		projectConfig.AIWorkspaces = append(projectConfig.AIWorkspaces, project.AIWorkspaceConfig{
			Name:       "default",
			PortalRoot: ".",
		})
		if err := project.Save(projectConfigPath, projectConfig); err != nil {
			return err
		}
	}

	for i := range projectConfig.AIWorkspaces {
		normalizeAIWorkspaceProjectConfig(&projectConfig.AIWorkspaces[i])
	}

	// Resolve -o into either an explicit output file (when it ends in .json) or
	// an output directory. With no -o, payloads land in the project build dir.
	outputDir := filepath.Join(projectRoot, "build")
	outputFile := ""
	if trimmed := strings.TrimSpace(buildOutputDir); trimmed != "" {
		resolved, err := filepath.Abs(trimmed)
		if err != nil {
			return fmt.Errorf("failed to resolve output path: %w", err)
		}
		if strings.EqualFold(filepath.Ext(resolved), ".json") {
			outputFile = resolved
			outputDir = filepath.Dir(resolved)
		} else {
			outputDir = resolved
		}
	}

	// An explicit output file can only hold a single payload.
	if outputFile != "" && len(projectConfig.AIWorkspaces) > 1 {
		return fmt.Errorf("output path %q is a file, but %d ai-workspace configurations are defined; use a directory instead",
			buildOutputDir, len(projectConfig.AIWorkspaces))
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outputs, failures := generateAIWorkspaceBuildArtifacts(projectRoot, outputDir, outputFile, buildUseSpec, projectConfig.AIWorkspaces)

	for _, output := range outputs {
		fmt.Printf("AI workspace payload generated at %s\n", output)
	}

	if len(failures) > 0 {
		messages := make([]string, 0, len(failures))
		for _, failure := range failures {
			fmt.Fprintf(os.Stderr, "AI workspace build failed for %q: %v\n", failure.name, failure.err)
			messages = append(messages, failure.err.Error())
		}
		return fmt.Errorf("failed to build %d of %d ai-workspace configuration(s): %s",
			len(failures), len(projectConfig.AIWorkspaces), strings.Join(messages, "; "))
	}

	return nil
}

func normalizeAIWorkspaceProjectConfig(config *project.AIWorkspaceConfig) {
	if strings.TrimSpace(config.Name) == "" {
		config.Name = "default"
	}
	if strings.TrimSpace(config.PortalRoot) == "" {
		config.PortalRoot = "."
	}
	if strings.TrimSpace(config.FilePaths.Metadata) == "" {
		config.FilePaths.Metadata = project.DefaultAIWorkspaceMetadata
	}
	if strings.TrimSpace(config.FilePaths.Runtime) == "" {
		config.FilePaths.Runtime = project.DefaultAIWorkspaceRuntime
	}
	if strings.TrimSpace(config.FilePaths.Definition) == "" {
		config.FilePaths.Definition = project.DefaultAIWorkspaceDefinition
	}
}

func generateAIWorkspaceBuildArtifacts(projectRoot, outputDir, outputFile string, useSpec bool, configs []project.AIWorkspaceConfig) ([]string, []failedWorkspace) {
	outputs := make([]string, 0, len(configs))
	failures := make([]failedWorkspace, 0)
	seen := make(map[string]string, len(configs)) // payload filename -> config name

	for i := range configs {
		outputPath, err := buildSingleAIWorkspacePayload(projectRoot, outputDir, outputFile, useSpec, seen, &configs[i])
		if err != nil {
			failures = append(failures, failedWorkspace{name: configs[i].Name, err: err})
			continue
		}
		outputs = append(outputs, outputPath)
	}

	return outputs, failures
}

// buildSingleAIWorkspacePayload reads the metadata.yaml and runtime.yaml for one
// ai-workspace config, derives the llm-proxy creation payload, optionally folds
// in the OpenAPI spec, and writes it as JSON. When outputFile is set it is
// written there; otherwise it lands at outputDir/<proxy-name>.json. Any existing
// file is overwritten. Returning an error drops only this config.
func buildSingleAIWorkspacePayload(projectRoot, outputDir, outputFile string, useSpec bool, seen map[string]string, config *project.AIWorkspaceConfig) (string, error) {
	baseDir := resolveProjectPath(projectRoot, config.PortalRoot)
	if err := ensureWithinProjectRoot(projectRoot, baseDir, config.Name, "portalRoot"); err != nil {
		return "", err
	}
	if err := ensurePathExists(baseDir, true, config.Name, "portalRoot"); err != nil {
		return "", err
	}

	metadataPath := resolveProjectPath(baseDir, config.FilePaths.Metadata)
	runtimePath := resolveProjectPath(baseDir, config.FilePaths.Runtime)

	// metadata.yaml and runtime.yaml are the required inputs for the payload.
	for _, required := range []struct {
		label string
		path  string
	}{
		{label: "metadata", path: metadataPath},
		{label: "runtime", path: runtimePath},
	} {
		if err := ensureWithinProjectRoot(projectRoot, required.path, config.Name, required.label); err != nil {
			return "", err
		}
		if err := ensurePathExists(required.path, false, config.Name, required.label); err != nil {
			return "", err
		}
	}

	var metadata aiWorkspaceMetadata
	if err := readYAMLFile(metadataPath, &metadata); err != nil {
		return "", fmt.Errorf("ai-workspace config %q: failed to read metadata: %w", config.Name, err)
	}
	var runtime aiWorkspaceRuntime
	if err := readYAMLFile(runtimePath, &runtime); err != nil {
		return "", fmt.Errorf("ai-workspace config %q: failed to read runtime: %w", config.Name, err)
	}

	proxyName := strings.TrimSpace(metadata.Metadata.Name)
	if proxyName == "" {
		return "", fmt.Errorf("ai-workspace config %q is invalid: metadata.metadata.name is required", config.Name)
	}

	// The openapi field is left empty by default. It is populated only when the
	// user opts in with --use-spec and the configured definition.yaml exists.
	openapi := ""
	if useSpec {
		definitionPath := resolveProjectPath(baseDir, config.FilePaths.Definition)
		if err := ensureWithinProjectRoot(projectRoot, definitionPath, config.Name, "definition"); err != nil {
			return "", err
		}
		if info, err := os.Stat(definitionPath); err == nil && !info.IsDir() {
			data, err := os.ReadFile(definitionPath)
			if err != nil {
				return "", fmt.Errorf("ai-workspace config %q: failed to read definition: %w", config.Name, err)
			}
			openapi = string(data)
		} else if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("ai-workspace config %q: failed to inspect definition: %w", config.Name, err)
		}
	}

	payload := buildLLMProxyPayload(proxyName, metadata, runtime, openapi)

	// An explicit -o file path wins; otherwise the artifact is named after the
	// ai-workspace config name (not metadata.name) under the output directory,
	// guarding against collisions.
	outputPath := outputFile
	if outputPath == "" {
		fileName := payloadFileName(config.Name)
		if existing, ok := seen[fileName]; ok {
			return "", fmt.Errorf("payload file %q is already produced by config %q; rename one of the ai-workspace configurations to avoid overwriting the artifact", fileName, existing)
		}
		seen[fileName] = config.Name
		outputPath = filepath.Join(outputDir, fileName)
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("ai-workspace config %q: failed to marshal payload: %w", config.Name, err)
	}
	if err := os.WriteFile(outputPath, append(data, '\n'), 0644); err != nil {
		return "", fmt.Errorf("ai-workspace config %q: failed to write payload: %w", config.Name, err)
	}

	return outputPath, nil
}

// buildLLMProxyPayload assembles the createLLMProxy request body from the
// project's metadata.yaml (name/version) and runtime.yaml (context, provider,
// policies). projectId is intentionally omitted and vhost is left empty for the
// caller to fill in at publish time.
func buildLLMProxyPayload(proxyName string, metadata aiWorkspaceMetadata, runtime aiWorkspaceRuntime, openapi string) llmProxyPayload {
	payload := llmProxyPayload{
		Name:     proxyName,
		Version:  strings.TrimSpace(metadata.Spec.Version),
		Context:  strings.TrimSpace(runtime.Spec.Context),
		Vhost:    "",
		OpenAPI:  openapi,
		Provider: llmProxyProvider{ID: strings.TrimSpace(runtime.Spec.Provider.ID)},
		Policies: []llmPolicy{},
	}

	if auth := runtime.Spec.Provider.Auth; auth != nil {
		payload.Provider.Auth = &llmUpstreamAuth{
			Type:   auth.Type,
			Header: auth.Header,
			Value:  auth.Value,
		}
	}

	for _, policy := range runtime.Spec.Policies {
		mapped := llmPolicy{
			Name:    policy.Name,
			Version: policy.Version,
			Paths:   make([]llmPolicyPath, 0, len(policy.Paths)),
		}
		for _, path := range policy.Paths {
			mapped.Paths = append(mapped.Paths, llmPolicyPath{
				Path:    path.Path,
				Methods: path.Methods,
				Params:  path.Params,
			})
		}
		payload.Policies = append(payload.Policies, mapped)
	}

	return payload
}

func payloadFileName(name string) string {
	sanitized := strings.TrimSpace(name)
	sanitized = strings.ReplaceAll(sanitized, string(os.PathSeparator), "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	if sanitized == "" {
		sanitized = "ai-workspace"
	}
	return sanitized + ".json"
}

func readYAMLFile(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("failed to parse %s: %w", filepath.Base(path), err)
	}
	return nil
}

// --- metadata.yaml / runtime.yaml input shapes (only the fields used here) ---

type aiWorkspaceMetadata struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		DisplayName string `yaml:"displayName"`
		Version     string `yaml:"version"`
	} `yaml:"spec"`
}

type aiWorkspaceRuntime struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		DisplayName string                  `yaml:"displayName"`
		Version     string                  `yaml:"version"`
		Context     string                  `yaml:"context"`
		Provider    runtimeProvider         `yaml:"provider"`
		Policies    []runtimeProviderPolicy `yaml:"policies"`
	} `yaml:"spec"`
}

type runtimeProvider struct {
	ID   string               `yaml:"id"`
	Auth *runtimeProviderAuth `yaml:"auth"`
}

type runtimeProviderAuth struct {
	Type   string `yaml:"type"`
	Header string `yaml:"header"`
	Value  string `yaml:"value"`
}

type runtimeProviderPolicy struct {
	Name    string              `yaml:"name"`
	Version string              `yaml:"version"`
	Paths   []runtimePolicyPath `yaml:"paths"`
}

type runtimePolicyPath struct {
	Path    string                 `yaml:"path"`
	Methods []string               `yaml:"methods"`
	Params  map[string]interface{} `yaml:"params"`
}

// --- createLLMProxy request body (subset; see openapi.yaml LLMProxy schema) ---

type llmProxyPayload struct {
	Name     string           `json:"name"`
	Version  string           `json:"version"`
	Context  string           `json:"context,omitempty"`
	Vhost    string           `json:"vhost"`
	Provider llmProxyProvider `json:"provider"`
	OpenAPI  string           `json:"openapi"`
	Policies []llmPolicy      `json:"policies"`
}

type llmProxyProvider struct {
	ID   string           `json:"id"`
	Auth *llmUpstreamAuth `json:"auth,omitempty"`
}

type llmUpstreamAuth struct {
	Type   string `json:"type,omitempty"`
	Header string `json:"header,omitempty"`
	Value  string `json:"value,omitempty"`
}

type llmPolicy struct {
	Name    string          `json:"name"`
	Version string          `json:"version"`
	Paths   []llmPolicyPath `json:"paths"`
}

type llmPolicyPath struct {
	Path    string                 `json:"path"`
	Methods []string               `json:"methods"`
	Params  map[string]interface{} `json:"params"`
}

// --- path helpers ---

func resolveProjectPath(root, pathValue string) string {
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return root
	}

	trimmed = strings.TrimPrefix(trimmed, "./")
	return filepath.Join(root, filepath.Clean(trimmed))
}

// ensureWithinProjectRoot rejects resolved paths that escape the project root
// (e.g. via ".." segments or symlinks in a config value), keeping build inputs
// bounded to the project directory.
func ensureWithinProjectRoot(projectRoot, path, configName, fieldName string) error {
	canonicalRoot, err := canonicalizePath(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve project root for ai-workspace config %q: %w", configName, err)
	}
	canonicalTarget, err := canonicalizePath(path)
	if err != nil {
		return fmt.Errorf("failed to resolve %s for ai-workspace config %q: %w", fieldName, configName, err)
	}

	rel, err := filepath.Rel(canonicalRoot, canonicalTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("ai-workspace config %q is invalid: %s path resolves outside the project root: %s", configName, fieldName, path)
	}

	return nil
}

// canonicalizePath returns an absolute, symlink-resolved form of path so that
// containment checks are reliable across differing path forms. When the path
// does not yet exist, it resolves symlinks on the nearest existing ancestor and
// re-appends the remaining segments rather than failing.
func canonicalizePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	remainder := ""
	current := abs
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			if remainder == "" {
				return resolved, nil
			}
			return filepath.Join(resolved, remainder), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return abs, nil
		}
		remainder = filepath.Join(filepath.Base(current), remainder)
		current = parent
	}
}

func ensurePathExists(path string, wantDir bool, configName, fieldName string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("ai-workspace config %q is invalid: %s path does not exist: %s", configName, fieldName, path)
		}
		return fmt.Errorf("failed to inspect %s for ai-workspace config %q: %w", fieldName, configName, err)
	}

	if wantDir && !info.IsDir() {
		return fmt.Errorf("ai-workspace config %q is invalid: %s must be a directory: %s", configName, fieldName, path)
	}
	if !wantDir && info.IsDir() {
		return fmt.Errorf("ai-workspace config %q is invalid: %s must be a file: %s", configName, fieldName, path)
	}

	return nil
}

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
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/aiworkspace"
	"github.com/wso2/api-platform/cli/internal/project"
	"github.com/wso2/api-platform/cli/utils"
	"gopkg.in/yaml.v3"
)

const (
	BuildCmdLiteral = "build"
	BuildCmdExample = `# Validate the AI workspace artifact in the current directory
ap ai-workspace build

# Validate from a specific project directory
ap ai-workspace build -f /path/to/project`
)

var buildProjectDir string

var buildCmd = &cobra.Command{
	Use:   BuildCmdLiteral,
	Short: "Validate the project's AI workspace artifact",
	Long: "Validate the AI workspace artifact for the project located in the specified directory " +
		"(or the current directory if not specified). The command reads the ai-workspace " +
		"configuration in .api-platform/config.yaml and checks that its metadata.yaml, runtime.yaml " +
		"and definition.yaml are present, that the metadata and runtime kinds align, and that the " +
		"resource name matches. It does not generate or send any artifact — use `ap ai-workspace apply` " +
		"to generate the creation payload and create the artifact.",
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
}

func runBuildCommand() error {
	projectRoot, wsConfig, err := resolveProjectAIWorkspace(buildProjectDir)
	if err != nil {
		return err
	}

	artifact, err := loadAIWorkspaceArtifact(projectRoot, wsConfig)
	if err != nil {
		return fmt.Errorf("AI workspace validation failed for %q: %w", wsConfig.Name, err)
	}

	fmt.Printf("AI workspace artifact %q (kind: %s) built successfully\n", artifact.ResourceName, artifact.BaseKind)
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

// aiWorkspaceArtifact holds the validated inputs for the project's ai-workspace
// artifact, loaded from metadata.yaml, runtime.yaml and definition.yaml. It is
// produced by loadAIWorkspaceArtifact (which validates but does not generate a
// payload) and consumed by buildPayload (which generates the creation payload).
// The split lets `build` validate only, while `apply`/`edit` validate then
// generate and send.
type aiWorkspaceArtifact struct {
	ConfigName   string
	BaseKind     string // LlmProvider / LlmProxy / Mcp ("Metadata" suffix stripped)
	ResourceName string // metadata.name; becomes the payload id
	metadata     aiWorkspaceMetadata
	runtime      aiWorkspaceRuntime
	openapi      string        // definition.yaml content (folded into provider/proxy payloads)
	mcpDef       mcpDefinition // parsed definition, populated for the Mcp kind
}

// resolveProjectAIWorkspace resolves the project root from projectDir, loads the
// project config, ensures a single ai-workspace configuration exists (creating a
// default and persisting it when absent), normalizes it, and returns the project
// root and the ai-workspace config entry. Shared by build, apply and edit.
func resolveProjectAIWorkspace(projectDir string) (string, *project.AIWorkspaceConfig, error) {
	if strings.TrimSpace(projectDir) == "" {
		projectDir = "."
	}
	projectRoot, err := filepath.Abs(projectDir)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve project directory: %w", err)
	}

	projectConfigDir := filepath.Join(projectRoot, ".api-platform")
	if _, err := os.Stat(projectConfigDir); os.IsNotExist(err) {
		return "", nil, fmt.Errorf("unable to find project directory, please execute this command inside a project")
	} else if err != nil {
		return "", nil, fmt.Errorf("failed to inspect project directory: %w", err)
	}

	projectConfigPath := filepath.Join(projectConfigDir, "config.yaml")
	if _, err := os.Stat(projectConfigPath); os.IsNotExist(err) {
		return "", nil, fmt.Errorf("unable to find project directory, please execute this command inside a project")
	} else if err != nil {
		return "", nil, fmt.Errorf("failed to inspect project config: %w", err)
	}

	projectConfig, err := project.Load(projectConfigPath)
	if err != nil {
		return "", nil, err
	}

	// A project has at most one ai-workspace config. Create a default one if it
	// is absent and persist it so the project records the configuration used.
	if projectConfig.AIWorkspace == nil {
		projectConfig.AIWorkspace = &project.AIWorkspaceConfig{
			Name:       "default",
			PortalRoot: ".",
		}
		if err := project.Save(projectConfigPath, projectConfig); err != nil {
			return "", nil, err
		}
	}

	normalizeAIWorkspaceProjectConfig(projectConfig.AIWorkspace)
	return projectRoot, projectConfig.AIWorkspace, nil
}

// loadAIWorkspaceArtifact reads and validates the ai-workspace artifact for the
// given config: it checks that metadata.yaml, runtime.yaml and definition.yaml
// are present and within the project, that the metadata/runtime kinds align
// (after stripping the metadata "Metadata" suffix), that metadata.name is set
// and matches between the two files, and (for the Mcp kind) that the definition
// parses. It does NOT generate the creation payload — call buildPayload for that.
func loadAIWorkspaceArtifact(projectRoot string, config *project.AIWorkspaceConfig) (*aiWorkspaceArtifact, error) {
	baseDir := resolveProjectPath(projectRoot, config.PortalRoot)
	if err := ensureWithinProjectRoot(projectRoot, baseDir, config.Name, "portalRoot"); err != nil {
		return nil, err
	}
	if err := ensurePathExists(baseDir, true, config.Name, "portalRoot"); err != nil {
		return nil, err
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
			return nil, err
		}
		if err := ensurePathExists(required.path, false, config.Name, required.label); err != nil {
			return nil, err
		}
	}

	var metadata aiWorkspaceMetadata
	if err := readYAMLFile(metadataPath, &metadata); err != nil {
		return nil, fmt.Errorf("ai-workspace config %q: failed to read metadata: %w", config.Name, err)
	}
	var runtime aiWorkspaceRuntime
	if err := readYAMLFile(runtimePath, &runtime); err != nil {
		return nil, fmt.Errorf("ai-workspace config %q: failed to read runtime: %w", config.Name, err)
	}

	// The kind declared in metadata.yaml carries a "Metadata" suffix
	// (e.g. LlmProxyMetadata) while runtime.yaml uses the base kind (LlmProxy).
	// Normalize both to the base kind before comparing and dispatching.
	rawMetadataKind := strings.TrimSpace(metadata.Kind)
	rawRuntimeKind := strings.TrimSpace(runtime.Kind)
	metadataKind := strings.TrimSuffix(rawMetadataKind, metadataKindSuffix)
	runtimeKind := strings.TrimSuffix(rawRuntimeKind, metadataKindSuffix)
	if metadataKind != runtimeKind {
		return nil, fmt.Errorf("ai-workspace config %q: kind mismatch: metadata.yaml has kind %q but runtime.yaml has kind %q", config.Name, rawMetadataKind, rawRuntimeKind)
	}

	resourceName := strings.TrimSpace(metadata.Metadata.Name)
	if resourceName == "" {
		return nil, fmt.Errorf("ai-workspace config %q is invalid: metadata.metadata.name is required", config.Name)
	}
	if runtimeName := strings.TrimSpace(runtime.Metadata.Name); runtimeName != resourceName {
		return nil, fmt.Errorf("ai-workspace config %q: name mismatch: metadata.yaml has metadata.name %q but runtime.yaml has metadata.name %q", config.Name, resourceName, runtimeName)
	}

	artifact := &aiWorkspaceArtifact{
		ConfigName:   config.Name,
		BaseKind:     metadataKind,
		ResourceName: resourceName,
		metadata:     metadata,
		runtime:      runtime,
	}

	// The OpenAPI spec (definition.yaml) is required for every kind.
	spec, err := loadAIWorkspaceSpec(projectRoot, baseDir, config, true)
	if err != nil {
		return nil, err
	}
	artifact.openapi = spec

	switch metadataKind {
	case kindLLMProxy, kindLLMProvider:
		// Folded into the payload verbatim; no further parsing at build/validate.
	case kindMCP:
		// An MCP proxy's capabilities live in the definition, so it must parse.
		var definition mcpDefinition
		if err := yaml.Unmarshal([]byte(spec), &definition); err != nil {
			return nil, fmt.Errorf("ai-workspace config %q: failed to parse definition: %w", config.Name, err)
		}
		artifact.mcpDef = definition
	default:
		return nil, fmt.Errorf("ai-workspace config %q: unsupported kind %q (supported: %s, %s, %s)", config.Name, metadataKind, kindLLMProxy, kindLLMProvider, kindMCP)
	}

	return artifact, nil
}

// buildPayload generates the creation payload for the validated artifact. The
// payload shape is driven by the artifact kind. Returns nil for an unsupported
// kind (loadAIWorkspaceArtifact already rejects those, so callers can treat nil
// as a programming error).
func (a *aiWorkspaceArtifact) buildPayload() interface{} {
	switch a.BaseKind {
	case kindLLMProxy:
		return buildLLMProxyPayload(a.ResourceName, a.metadata, a.runtime, a.openapi)
	case kindLLMProvider:
		return buildLLMProviderPayload(a.ResourceName, a.metadata, a.runtime, a.openapi)
	case kindMCP:
		return buildMCPProxyPayload(a.ResourceName, a.metadata, a.runtime, a.mcpDef)
	default:
		return nil
	}
}

// marshalAIWorkspacePayload generates the creation payload from the validated
// artifact and returns it as JSON. LlmProxy and Mcp artifacts are project-scoped
// and require a projectId (from --project-id) injected into the body; providers
// are not project-scoped. Returns the JSON body and the projectId used (empty
// for providers).
func marshalAIWorkspacePayload(artifact *aiWorkspaceArtifact, projectIDFlag string) ([]byte, string, error) {
	payload := artifact.buildPayload()
	if payload == nil {
		return nil, "", fmt.Errorf("unsupported kind %q", artifact.BaseKind)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode payload: %w", err)
	}

	// Providers are not project-scoped; send the payload as generated.
	if artifact.BaseKind == kindLLMProvider {
		return raw, "", nil
	}

	// Proxies and MCP proxies require a projectId. Inject it without dropping any
	// generated fields.
	projectID := strings.TrimSpace(projectIDFlag)
	if projectID == "" {
		return nil, "", fmt.Errorf("project ID is required for %s artifacts (use --project-id)", artifact.BaseKind)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, "", fmt.Errorf("failed to encode payload: %w", err)
	}
	m["projectId"] = projectID
	body, err := json.Marshal(m)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode payload: %w", err)
	}
	return body, projectID, nil
}

// cliEnvPlaceholderPattern matches ENV_CLI_* placeholders carried from
// metadata.yaml/runtime.yaml into the generated payload. Both `${ENV_CLI_X}`
// and `$ENV_CLI_X` forms are supported, as well as a bare `ENV_CLI_X` token.
// The bare form requires a leading word boundary so a placeholder embedded in a
// larger identifier (e.g. MY_ENV_CLI_X) is not partially substituted.
var cliEnvPlaceholderPattern = regexp.MustCompile(`\$\{ENV_CLI_[A-Za-z0-9_]+\}|\$ENV_CLI_[A-Za-z0-9_]+|\bENV_CLI_[A-Za-z0-9_]+`)

// cliEnvPlaceholderName strips the `${...}`/`$` wrapping from a matched
// placeholder, returning the bare variable name (ENV_CLI_X).
func cliEnvPlaceholderName(placeholder string) string {
	name := strings.TrimPrefix(placeholder, "${")
	name = strings.TrimSuffix(name, "}")
	return strings.TrimPrefix(name, "$")
}

// resolveEnvPlaceholders substitutes ENV_CLI_* placeholders in the generated
// payload before it is sent to the AI workspace. Values are looked up in an
// env file first — the file given via --env-file when provided (it must
// exist), otherwise the project root's .env when present — falling back to the
// process environment for names the file does not define. When the payload has
// no placeholders it is returned unchanged. Any placeholder that cannot be
// resolved fails the command with the full list of missing variables.
func resolveEnvPlaceholders(body []byte, projectRoot, envFileFlag string) ([]byte, error) {
	content := string(body)
	matches := cliEnvPlaceholderPattern.FindAllString(content, -1)
	if len(matches) == 0 {
		return body, nil
	}

	fileValues, err := loadEnvValues(projectRoot, envFileFlag)
	if err != nil {
		return nil, err
	}

	missing := map[string]bool{}
	resolved := cliEnvPlaceholderPattern.ReplaceAllStringFunc(content, func(placeholder string) string {
		name := cliEnvPlaceholderName(placeholder)
		value, ok := fileValues[name]
		if !ok {
			value, ok = os.LookupEnv(name)
		}
		if !ok {
			missing[name] = true
			return placeholder
		}
		// The replacement happens inside JSON string values, so the value must be
		// JSON-escaped (sans the surrounding quotes json.Marshal adds).
		escaped, _ := json.Marshal(value)
		return string(escaped[1 : len(escaped)-1])
	})

	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for name := range missing {
			names = append(names, name)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("unresolved environment variable(s) in the generated artifact: %s (define them in an env file passed via --%s, a .env file in the project root, or the environment)",
			strings.Join(names, ", "), utils.FlagEnvFile)
	}

	return []byte(resolved), nil
}

// loadEnvValues returns the KEY=VALUE pairs from the env file selected for
// placeholder resolution: the --env-file path when given (missing file is an
// error), else the project root's .env when it exists, else an empty map (the
// process environment is consulted by the caller as the fallback).
func loadEnvValues(projectRoot, envFileFlag string) (map[string]string, error) {
	if path := strings.TrimSpace(envFileFlag); path != "" {
		values, err := parseEnvFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read env file %q: %w", path, err)
		}
		return values, nil
	}

	defaultPath := filepath.Join(projectRoot, ".env")
	if _, err := os.Stat(defaultPath); err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("failed to inspect .env file: %w", err)
	}
	values, err := parseEnvFile(defaultPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .env file %q: %w", defaultPath, err)
	}
	return values, nil
}

// parseEnvFile reads a dotenv-style file: one KEY=VALUE per line, blank lines
// and #-comments ignored, an optional `export ` prefix allowed, and single or
// double quotes around the value stripped.
func parseEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
				(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
		}
		values[key] = value
	}
	return values, nil
}

// aiWorkspaceCreatePath returns the collection endpoint to POST to for the kind.
func aiWorkspaceCreatePath(baseKind string) string {
	switch baseKind {
	case kindLLMProvider:
		return aiworkspace.ProviderPath()
	case kindLLMProxy:
		return aiworkspace.ProxyPath()
	case kindMCP:
		return aiworkspace.MCPProxyPath()
	default:
		return ""
	}
}

// aiWorkspaceUpdatePath returns the by-id endpoint to PUT to for the kind.
func aiWorkspaceUpdatePath(baseKind, id string) string {
	switch baseKind {
	case kindLLMProvider:
		return aiworkspace.ProviderByIDPath(id)
	case kindLLMProxy:
		return aiworkspace.ProxyByIDPath(id)
	case kindMCP:
		return aiworkspace.MCPProxyByIDPath(id)
	default:
		return ""
	}
}

// Supported artifact kinds. These match the `kind` declared in metadata.yaml
// and runtime.yaml.
const (
	kindLLMProxy    = "LlmProxy"
	kindLLMProvider = "LlmProvider"
	kindMCP         = "Mcp"
	// metadataKindSuffix is appended to the metadata.yaml kind for ai-workspace
	// artifacts (e.g. LlmProxyMetadata) to distinguish it from the runtime kind
	// (LlmProxy). It is stripped before comparing/dispatching on the kind.
	metadataKindSuffix = "Metadata"
)

// loadAIWorkspaceSpec reads the configured definition.yaml relative to baseDir
// and returns its content. When required is true a missing definition is an
// error; otherwise a missing definition yields an empty spec.
func loadAIWorkspaceSpec(projectRoot, baseDir string, config *project.AIWorkspaceConfig, required bool) (string, error) {
	definitionPath := resolveProjectPath(baseDir, config.FilePaths.Definition)
	if err := ensureWithinProjectRoot(projectRoot, definitionPath, config.Name, "definition"); err != nil {
		return "", err
	}

	info, err := os.Stat(definitionPath)
	if err != nil {
		if os.IsNotExist(err) {
			if required {
				return "", fmt.Errorf("ai-workspace config %q is invalid: definition path does not exist: %s", config.Name, definitionPath)
			}
			return "", nil
		}
		return "", fmt.Errorf("ai-workspace config %q: failed to inspect definition: %w", config.Name, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("ai-workspace config %q is invalid: definition must be a file: %s", config.Name, definitionPath)
	}

	data, err := os.ReadFile(definitionPath)
	if err != nil {
		return "", fmt.Errorf("ai-workspace config %q: failed to read definition: %w", config.Name, err)
	}
	return string(data), nil
}

// templateModelIDs maps an LLM provider template to the model IDs it exposes.
// When a provider's template matches a key here, the build emits a single
// modelProviders entry (keyed by the template) carrying these models.
var templateModelIDs = map[string][]string{
	"meta": {
		"us.meta.llama3-3-70b-instruct-v1:0",
		"us.meta.llama4-maverick-17b-instruct-v1:0",
	},
	"openai":        {"gpt-4o-mini", "gpt-4.1-mini", "o4-mini"},
	"anthropic":     {"claude-3.5-sonnet", "claude-3-opus"},
	"google-vertex": {"gemini-1.5-pro", "gemini-1.5-flash"},
	"aws-bedrock":   {"amazon.titan-text-premier", "anthropic.claude-v2"},
	"mistralai": {
		"mistral-large-latest",
		"mistral-small-latest",
		"open-mixtral-8x22b",
	},
}

// modelProvidersForTemplate returns the modelProviders block for a provider
// template. It returns a single model provider (keyed by the template name)
// carrying the template's models, or nil for an unknown template so the payload
// omits the field.
func modelProvidersForTemplate(template string) []llmModelProvider {
	template = strings.TrimSpace(template)
	modelIDs, ok := templateModelIDs[template]
	if !ok || len(modelIDs) == 0 {
		return nil
	}

	provider := llmModelProvider{ID: template, DisplayName: template}
	for _, modelID := range modelIDs {
		provider.Models = append(provider.Models, llmModel{ID: modelID, DisplayName: modelID})
	}
	return []llmModelProvider{provider}
}

// buildLLMProviderPayload assembles the createLLMProvider request body from the
// project's metadata.yaml (name/version) and runtime.yaml (context, template,
// upstream, accessControl, policies). The api-key-auth policy is mapped to the
// security block, and modelProviders is derived from the template (see
// modelProvidersForTemplate).
func buildLLMProviderPayload(name string, metadata aiWorkspaceMetadata, runtime aiWorkspaceRuntime, openapi string) llmProviderPayload {
	template := strings.TrimSpace(runtime.Spec.Template)
	payload := llmProviderPayload{
		ID:                 name,
		DisplayName:        strings.TrimSpace(metadata.Spec.DisplayName),
		Version:            strings.TrimSpace(metadata.Spec.Version),
		Context:            strings.TrimSpace(runtime.Spec.Context),
		Template:           template,
		OpenAPI:            openapi,
		ModelProviders:     modelProvidersForTemplate(template),
		AssociatedGateways: normalizeAssociatedGateways(metadata.Spec.AssociatedGateways),
	}

	if up := runtime.Spec.Upstream; up != nil {
		target := llmUpstreamTarget{URL: strings.TrimSpace(up.URL)}
		if up.Auth != nil {
			target.Auth = &llmUpstreamAuth{Type: up.Auth.Type, Header: up.Auth.Header, Value: up.Auth.Value}
		}
		payload.Upstream = &llmUpstream{Main: target}
	}

	if ac := runtime.Spec.AccessControl; ac != nil {
		mapped := &llmAccessControl{Mode: ac.Mode}
		for _, exception := range ac.Exceptions {
			mapped.Exceptions = append(mapped.Exceptions, routeException{Methods: exception.Methods, Path: exception.Path})
		}
		payload.AccessControl = mapped
	}

	payload.RateLimiting = buildRateLimitingFromPolicies(runtime.Spec.Policies)
	payload.Security = buildSecurityFromPolicies(runtime.Spec.Policies)

	// Any policy that is not mapped to security (api-key-auth) or rateLimiting
	// (*-ratelimit) passes through into the policies array unchanged.
	for _, policy := range runtime.Spec.Policies {
		if policy.Name == "api-key-auth" || strings.HasSuffix(policy.Name, "-ratelimit") {
			continue
		}
		payload.Policies = append(payload.Policies, mapPolicy(policy))
	}

	return payload
}

// mapPolicy converts a runtime.yaml policy into the payload policy shape.
func mapPolicy(policy runtimeProviderPolicy) llmPolicy {
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
	return mapped
}

// buildMCPProxyPayload assembles the MCP proxy creation payload from the
// project's metadata.yaml (name/version), runtime.yaml (context, mcpSpecVersion,
// upstream, policies) and definition.yaml (capabilities). projectId is left out
// here and injected at publish time.
func buildMCPProxyPayload(name string, metadata aiWorkspaceMetadata, runtime aiWorkspaceRuntime, definition mcpDefinition) mcpProxyPayload {
	description := strings.TrimSpace(runtime.Spec.Description)
	if description == "" {
		description = defaultProxyDescription
	}

	payload := mcpProxyPayload{
		ID:             name,
		DisplayName:    strings.TrimSpace(metadata.Spec.DisplayName),
		Version:        strings.TrimSpace(metadata.Spec.Version),
		Context:        strings.TrimSpace(runtime.Spec.Context),
		Description:    description,
		MCPSpecVersion: strings.TrimSpace(runtime.Spec.SpecVersion),
		Capabilities: &mcpCapabilities{
			Prompts:   definition.Prompts,
			Resources: mcpResources(definition.Resources),
			Tools:     definition.Tools,
		},
		AssociatedGateways: normalizeAssociatedGateways(metadata.Spec.AssociatedGateways),
	}

	if up := runtime.Spec.Upstream; up != nil {
		target := llmUpstreamTarget{URL: strings.TrimSpace(up.URL)}
		if up.Auth != nil {
			target.Auth = &llmUpstreamAuth{Type: up.Auth.Type, Header: up.Auth.Header, Value: up.Auth.Value}
		}
		payload.Upstream = &llmUpstream{Main: target}
	}

	for _, policy := range runtime.Spec.Policies {
		payload.Policies = append(payload.Policies, mcpPolicy{
			Name:    policy.Name,
			Version: policy.Version,
			Params:  policy.Params,
		})
	}

	return payload
}

// mcpResources strips each definition resource down to the fields the
// capabilities block carries (uri, name, mimeType), dropping inline text/blob
// content.
func mcpResources(resources []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(resources))
	for _, resource := range resources {
		trimmed := map[string]interface{}{}
		for _, key := range []string{"uri", "name", "mimeType"} {
			if value, ok := resource[key]; ok {
				trimmed[key] = value
			}
		}
		out = append(out, trimmed)
	}
	return out
}

// normalizeAssociatedGateways trims the associatedGateways read from
// metadata.yaml: gateway ids are trimmed and entries without an id are
// dropped. It returns nil when nothing remains so the payload omits the field
// (matching the optional schema in openapi.yaml).
func normalizeAssociatedGateways(gateways []associatedGateway) []associatedGateway {
	out := make([]associatedGateway, 0, len(gateways))
	for _, gateway := range gateways {
		id := strings.TrimSpace(gateway.ID)
		if id == "" {
			continue
		}
		out = append(out, associatedGateway{ID: id, Configurations: gateway.Configurations})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildRateLimitingFromPolicies maps the *-ratelimit policies in runtime.yaml
// into the provider's rateLimiting block. The policy name selects the dimension
// (advanced-* -> request, token-based-* -> token, llm-cost-based-* -> cost) and
// the scope is consumer-level when the policy is flagged consumerBased (or, for
// advanced quotas, when the quota name carries a "consumer" prefix); otherwise
// it is provider-level.
//
// Each limit is applied globally when its path is "/*" and resource-wise (keyed
// by the path) otherwise. A scope that has any resource-wise limit is emitted as
// resourceWise, with any "/*" limits folded into its default.
func buildRateLimitingFromPolicies(policies []runtimeProviderPolicy) *llmRateLimiting {
	provider := &scopeAccumulator{}
	consumer := &scopeAccumulator{}

	for _, policy := range policies {
		if !strings.HasSuffix(policy.Name, "-ratelimit") {
			continue
		}
		for _, path := range policy.Paths {
			params := path.Params
			if params == nil {
				continue
			}
			consumerBased := asBool(params["consumerBased"])

			switch {
			case strings.HasPrefix(policy.Name, "advanced"):
				dimension, quotaConsumer := advancedRequestDimension(params)
				if dimension == nil {
					continue
				}
				scope := provider
				if consumerBased || quotaConsumer {
					scope = consumer
				}
				scope.configFor(path.Path).Request = dimension
			case strings.Contains(policy.Name, "token"):
				dimension := tokenDimension(params)
				if dimension == nil {
					continue
				}
				scope := provider
				if consumerBased {
					scope = consumer
				}
				scope.configFor(path.Path).Token = dimension
			case strings.Contains(policy.Name, "cost"):
				dimension := costDimension(params)
				if dimension == nil {
					continue
				}
				scope := provider
				if consumerBased {
					scope = consumer
				}
				scope.configFor(path.Path).Cost = dimension
			}
		}
	}

	providerScope := provider.build()
	consumerScope := consumer.build()
	if providerScope == nil && consumerScope == nil {
		return nil
	}
	return &llmRateLimiting{ProviderLevel: providerScope, ConsumerLevel: consumerScope}
}

// scopeAccumulator collects rate-limit dimensions for one scope (provider or
// consumer), separating global ("/*") limits from per-path (resource-wise) ones.
type scopeAccumulator struct {
	global    *rateLimitConfig
	resources map[string]*rateLimitConfig
	order     []string // preserves resource insertion order
}

// configFor returns the limit config a dimension on path should be written to,
// creating it on first use.
func (a *scopeAccumulator) configFor(path string) *rateLimitConfig {
	if path == "" || path == "/*" {
		if a.global == nil {
			a.global = &rateLimitConfig{}
		}
		return a.global
	}
	if a.resources == nil {
		a.resources = map[string]*rateLimitConfig{}
	}
	if config, ok := a.resources[path]; ok {
		return config
	}
	config := &rateLimitConfig{}
	a.resources[path] = config
	a.order = append(a.order, path)
	return config
}

// build renders the accumulator into a scope: global when only "/*" limits were
// seen, resourceWise (with "/*" limits as the default) when any path-specific
// limit was seen, or nil when empty.
func (a *scopeAccumulator) build() *rateLimitScope {
	if len(a.order) == 0 {
		if a.global == nil {
			return nil
		}
		return &rateLimitScope{Global: a.global}
	}

	defaultConfig := a.global
	if defaultConfig == nil {
		defaultConfig = &rateLimitConfig{}
	}
	resourceWise := &resourceWiseConfig{Default: defaultConfig}
	for _, path := range a.order {
		resourceWise.Resources = append(resourceWise.Resources, resourceLimit{Resource: path, Limit: a.resources[path]})
	}
	return &rateLimitScope{ResourceWise: resourceWise}
}

// advancedRequestDimension reads the first quota's first limit into a request
// dimension and reports whether the quota name marks it consumer-scoped.
func advancedRequestDimension(params map[string]interface{}) (*rateLimitDimension, bool) {
	quota := firstMap(params["quotas"])
	if quota == nil {
		return nil, false
	}
	isConsumer := strings.HasPrefix(asString(quota["name"]), "consumer")
	limit := firstMap(quota["limits"])
	if limit == nil {
		return nil, isConsumer
	}
	count, _ := asInt(limit["limit"])
	return &rateLimitDimension{
		Enabled: true,
		Count:   count,
		Reset:   parseResetWindow(asString(limit["duration"])),
	}, isConsumer
}

func tokenDimension(params map[string]interface{}) *rateLimitDimension {
	limit := firstMap(params["totalTokenLimits"])
	if limit == nil {
		return nil
	}
	count, _ := asInt(limit["count"])
	return &rateLimitDimension{
		Enabled: true,
		Count:   count,
		Reset:   parseResetWindow(asString(limit["duration"])),
	}
}

func costDimension(params map[string]interface{}) *rateLimitCostDimension {
	limit := firstMap(params["budgetLimits"])
	if limit == nil {
		return nil
	}
	amount, _ := asFloat(limit["amount"])
	return &rateLimitCostDimension{
		Enabled: true,
		Amount:  amount,
		Reset:   parseResetWindow(asString(limit["duration"])),
	}
}

// parseResetWindow turns a duration like "1h" or "3h" into a {duration, unit}
// reset window. Unknown unit suffixes are passed through unchanged.
func parseResetWindow(value string) *rateLimitReset {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	i := 0
	for i < len(value) && value[i] >= '0' && value[i] <= '9' {
		i++
	}
	if i == 0 {
		return nil
	}
	duration, err := strconv.Atoi(value[:i])
	if err != nil {
		return nil
	}
	unit := strings.ToLower(strings.TrimSpace(value[i:]))
	switch unit {
	case "m", "min", "minute", "minutes":
		unit = "minute"
	case "h", "hr", "hour", "hours":
		unit = "hour"
	case "d", "day", "days":
		unit = "day"
	case "w", "week", "weeks":
		unit = "week"
	case "mo", "month", "months":
		unit = "month"
	}
	return &rateLimitReset{Duration: duration, Unit: unit}
}

// --- free-form params accessors (runtime policy params are open JSON) ---

func firstMap(value interface{}) map[string]interface{} {
	slice, ok := value.([]interface{})
	if !ok || len(slice) == 0 {
		return nil
	}
	m, _ := slice[0].(map[string]interface{})
	return m
}

func asString(value interface{}) string {
	s, _ := value.(string)
	return s
}

func asBool(value interface{}) bool {
	b, _ := value.(bool)
	return b
}

func asInt(value interface{}) (int, bool) {
	switch n := value.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}

func asFloat(value interface{}) (float64, bool) {
	switch n := value.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// buildSecurityFromPolicies maps the api-key-auth policy (if present) to the
// provider's security block.
func buildSecurityFromPolicies(policies []runtimeProviderPolicy) *securityConfig {
	for _, policy := range policies {
		if policy.Name != "api-key-auth" {
			continue
		}
		apiKey := &apiKeySecurity{Enabled: true}
		for _, path := range policy.Paths {
			if v, ok := path.Params["key"].(string); ok && apiKey.Key == "" {
				apiKey.Key = v
			}
			if v, ok := path.Params["in"].(string); ok && apiKey.In == "" {
				apiKey.In = v
			}
		}
		return &securityConfig{Enabled: true, APIKey: apiKey}
	}
	return nil
}

// defaultProxyDescription is used when runtime.yaml carries no spec.description.
const defaultProxyDescription = "No description provided for this proxy."

// buildLLMProxyPayload assembles the createLLMProxy request body from the
// project's metadata.yaml (name/version/displayName) and runtime.yaml (context,
// provider, description, policies). Policies come from runtime.yaml's split
// globalPolicies / operationPolicies sections: the api-key-auth global policy is
// mapped to the security block, every other global policy passes through into
// globalPolicies, and operationPolicies pass through with their per-path params.
// Each policy's params are policy-specific (no common schema) and are copied
// verbatim. projectId is intentionally omitted for the caller to inject at
// publish time.
func buildLLMProxyPayload(proxyName string, metadata aiWorkspaceMetadata, runtime aiWorkspaceRuntime, openapi string) llmProxyPayload {
	description := strings.TrimSpace(runtime.Spec.Description)
	if description == "" {
		description = defaultProxyDescription
	}

	payload := llmProxyPayload{
		ID:                 proxyName,
		DisplayName:        strings.TrimSpace(metadata.Spec.DisplayName),
		Version:            strings.TrimSpace(metadata.Spec.Version),
		Context:            strings.TrimSpace(runtime.Spec.Context),
		Description:        description,
		OpenAPI:            openapi,
		ReadOnly:           false,
		Provider:           llmProxyProvider{ID: strings.TrimSpace(runtime.Spec.Provider.ID)},
		AssociatedGateways: normalizeAssociatedGateways(metadata.Spec.AssociatedGateways),
	}

	// The proxy references its provider by id; the provider owns the credential
	// value, so only the auth type/header are carried here (never the secret).
	if auth := runtime.Spec.Provider.Auth; auth != nil {
		payload.Provider.Auth = &llmUpstreamAuth{
			Type:   auth.Type,
			Header: auth.Header,
		}
	}

	// api-key-auth is expressed as the security block; all other global policies
	// pass through with their policy-specific params.
	payload.Security = buildSecurityFromGlobalPolicies(runtime.Spec.GlobalPolicies)
	for _, policy := range runtime.Spec.GlobalPolicies {
		if policy.Name == "api-key-auth" {
			continue
		}
		payload.GlobalPolicies = append(payload.GlobalPolicies, llmGlobalPolicy{
			Name:    policy.Name,
			Version: policy.Version,
			Params:  policy.Params,
		})
	}

	for _, policy := range runtime.Spec.OperationPolicies {
		payload.OperationPolicies = append(payload.OperationPolicies, mapPolicy(policy))
	}

	return payload
}

// buildSecurityFromGlobalPolicies maps the api-key-auth global policy (if
// present) to the proxy's security block. Its params sit at the policy level
// (in, key), unlike the provider's paths-based api-key-auth policy.
func buildSecurityFromGlobalPolicies(policies []runtimeProviderPolicy) *securityConfig {
	for _, policy := range policies {
		if policy.Name != "api-key-auth" {
			continue
		}
		apiKey := &apiKeySecurity{Enabled: true}
		if v, ok := policy.Params["key"].(string); ok {
			apiKey.Key = v
		}
		if v, ok := policy.Params["in"].(string); ok {
			apiKey.In = v
		}
		return &securityConfig{Enabled: true, APIKey: apiKey}
	}
	return nil
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
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		DisplayName string `yaml:"displayName"`
		Version     string `yaml:"version"`
		// AssociatedGateways lives under spec in metadata.yaml; the build extracts
		// it from there and folds it into the generated payload.
		AssociatedGateways []associatedGateway `yaml:"associatedGateways"`
	} `yaml:"spec"`
}

// associatedGateway mirrors the AssociatedGateway schema (openapi.yaml): the
// gateway id plus a free-form per-gateway configuration override. The same
// shape is used to parse metadata.yaml and to emit the build payload.
type associatedGateway struct {
	ID             string                 `json:"id" yaml:"id"`
	Configurations map[string]interface{} `json:"configurations,omitempty" yaml:"configurations"`
}

type aiWorkspaceRuntime struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		DisplayName   string                `yaml:"displayName"`
		Version       string                `yaml:"version"`
		Context       string                `yaml:"context"`
		Description   string                `yaml:"description"`
		Template      string                `yaml:"template"`
		SpecVersion   string                `yaml:"specVersion"`
		Provider      runtimeProvider       `yaml:"provider"`
		Upstream      *runtimeUpstream      `yaml:"upstream"`
		AccessControl *runtimeAccessControl `yaml:"accessControl"`
		// Policies is the legacy flat list still used by the LLM provider and MCP
		// proxy builders. LLM proxies use the split globalPolicies /
		// operationPolicies below.
		Policies          []runtimeProviderPolicy `yaml:"policies"`
		GlobalPolicies    []runtimeProviderPolicy `yaml:"globalPolicies"`
		OperationPolicies []runtimeProviderPolicy `yaml:"operationPolicies"`
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

type runtimeUpstream struct {
	URL  string               `yaml:"url"`
	Auth *runtimeProviderAuth `yaml:"auth"`
}

type runtimeAccessControl struct {
	Mode       string                  `yaml:"mode"`
	Exceptions []runtimeRouteException `yaml:"exceptions"`
}

type runtimeRouteException struct {
	Methods []string `yaml:"methods"`
	Path    string   `yaml:"path"`
}

type runtimeProviderPolicy struct {
	Name    string              `yaml:"name"`
	Version string              `yaml:"version"`
	Paths   []runtimePolicyPath `yaml:"paths"`
	// Params holds policy-level params used by MCP policies (LLM proxy/provider
	// policies carry their params under paths[].params instead).
	Params map[string]interface{} `yaml:"params"`
}

type runtimePolicyPath struct {
	Path    string                 `yaml:"path"`
	Methods []string               `yaml:"methods"`
	Params  map[string]interface{} `yaml:"params"`
}

// --- createLLMProxy request body (subset; see openapi.yaml LLMProxy schema) ---

type llmProxyPayload struct {
	ID                 string              `json:"id"`
	DisplayName        string              `json:"displayName"`
	Version            string              `json:"version"`
	Context            string              `json:"context,omitempty"`
	Description        string              `json:"description"`
	Provider           llmProxyProvider    `json:"provider"`
	OpenAPI            string              `json:"openapi"`
	ReadOnly           bool                `json:"readOnly"`
	Security           *securityConfig     `json:"security,omitempty"`
	GlobalPolicies     []llmGlobalPolicy   `json:"globalPolicies,omitempty"`
	OperationPolicies  []llmPolicy         `json:"operationPolicies,omitempty"`
	AssociatedGateways []associatedGateway `json:"associatedGateways,omitempty"`
}

// llmGlobalPolicy is an api-level policy applied across all operations. Unlike
// operation policies it has no paths; its params are policy-specific and passed
// through verbatim.
type llmGlobalPolicy struct {
	Name    string                 `json:"name"`
	Version string                 `json:"version"`
	Params  map[string]interface{} `json:"params,omitempty"`
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

// --- MCP definition input (definition.yaml) and MCP proxy request body ---

// mcpDefinition is the parsed definition.yaml for an MCP proxy. Prompts and
// tools pass through verbatim; resources are trimmed before being emitted.
type mcpDefinition struct {
	Prompts   []map[string]interface{} `yaml:"prompts"`
	Resources []map[string]interface{} `yaml:"resources"`
	Tools     []map[string]interface{} `yaml:"tools"`
}

type mcpProxyPayload struct {
	ID                 string              `json:"id"`
	DisplayName        string              `json:"displayName"`
	Version            string              `json:"version"`
	Context            string              `json:"context,omitempty"`
	Description        string              `json:"description"`
	MCPSpecVersion     string              `json:"mcpSpecVersion,omitempty"`
	Upstream           *llmUpstream        `json:"upstream,omitempty"`
	Capabilities       *mcpCapabilities    `json:"capabilities,omitempty"`
	Policies           []mcpPolicy         `json:"policies,omitempty"`
	AssociatedGateways []associatedGateway `json:"associatedGateways,omitempty"`
}

type mcpCapabilities struct {
	Prompts   []map[string]interface{} `json:"prompts"`
	Resources []map[string]interface{} `json:"resources"`
	Tools     []map[string]interface{} `json:"tools"`
}

type mcpPolicy struct {
	Name    string                 `json:"name"`
	Version string                 `json:"version"`
	Params  map[string]interface{} `json:"params"`
}

// --- createLLMProvider request body (subset; see openapi.yaml LLMProvider schema) ---

type llmProviderPayload struct {
	ID                 string              `json:"id"`
	DisplayName        string              `json:"displayName"`
	Version            string              `json:"version"`
	Context            string              `json:"context,omitempty"`
	Template           string              `json:"template"`
	ModelProviders     []llmModelProvider  `json:"modelProviders,omitempty"`
	Upstream           *llmUpstream        `json:"upstream,omitempty"`
	AccessControl      *llmAccessControl   `json:"accessControl,omitempty"`
	OpenAPI            string              `json:"openapi"`
	RateLimiting       *llmRateLimiting    `json:"rateLimiting,omitempty"`
	Security           *securityConfig     `json:"security,omitempty"`
	Policies           []llmPolicy         `json:"policies,omitempty"`
	AssociatedGateways []associatedGateway `json:"associatedGateways,omitempty"`
}

// llmModelProvider / llmModel mirror the LLMModelProvider / LLMModel schemas
// (openapi.yaml). The build derives them from the provider template.
type llmModelProvider struct {
	ID          string     `json:"id,omitempty"`
	DisplayName string     `json:"displayName"`
	Models      []llmModel `json:"models,omitempty"`
}

type llmModel struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
}

type llmRateLimiting struct {
	ProviderLevel *rateLimitScope `json:"providerLevel,omitempty"`
	ConsumerLevel *rateLimitScope `json:"consumerLevel,omitempty"`
}

type rateLimitScope struct {
	Global       *rateLimitConfig    `json:"global,omitempty"`
	ResourceWise *resourceWiseConfig `json:"resourceWise,omitempty"`
}

type resourceWiseConfig struct {
	Default   *rateLimitConfig `json:"default,omitempty"`
	Resources []resourceLimit  `json:"resources"`
}

type resourceLimit struct {
	Resource string           `json:"resource"`
	Limit    *rateLimitConfig `json:"limit,omitempty"`
}

type rateLimitConfig struct {
	Request *rateLimitDimension     `json:"request,omitempty"`
	Token   *rateLimitDimension     `json:"token,omitempty"`
	Cost    *rateLimitCostDimension `json:"cost,omitempty"`
}

type rateLimitDimension struct {
	Enabled bool            `json:"enabled"`
	Count   int             `json:"count"`
	Reset   *rateLimitReset `json:"reset,omitempty"`
}

type rateLimitCostDimension struct {
	Enabled bool            `json:"enabled"`
	Amount  float64         `json:"amount"`
	Reset   *rateLimitReset `json:"reset,omitempty"`
}

type rateLimitReset struct {
	Duration int    `json:"duration"`
	Unit     string `json:"unit"`
}

type llmUpstream struct {
	Main llmUpstreamTarget `json:"main"`
}

type llmUpstreamTarget struct {
	URL  string           `json:"url,omitempty"`
	Auth *llmUpstreamAuth `json:"auth,omitempty"`
}

type llmAccessControl struct {
	Mode       string           `json:"mode"`
	Exceptions []routeException `json:"exceptions,omitempty"`
}

type routeException struct {
	Methods []string `json:"methods"`
	Path    string   `json:"path"`
}

type securityConfig struct {
	Enabled bool            `json:"enabled"`
	APIKey  *apiKeySecurity `json:"apiKey,omitempty"`
}

type apiKeySecurity struct {
	Enabled bool   `json:"enabled"`
	Key     string `json:"key,omitempty"`
	In      string `json:"in,omitempty"`
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

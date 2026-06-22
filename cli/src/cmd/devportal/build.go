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
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/project"
	"github.com/wso2/api-platform/cli/utils"
	"gopkg.in/yaml.v3"
)

// Standard names every devportal artifact takes inside the archive, regardless
// of the source paths configured in .api-platform/config.yaml. Standardizing
// here gives the published zip a predictable layout for the devportal to
// unpack, while letting authors keep arbitrary on-disk filenames.
const (
	archiveMetadataFileName   = "devportal.yaml"
	archiveDefinitionFileName = "definition.yaml"
	archiveDocsDirName        = "docs"
	archiveContentDirName     = "content"
)

const (
	BuildCmdLiteral = "build"
	BuildCmdExample = `# Build the project in the current directory for devportal
ap devportal build

# Build a project in a specified directory
ap devportal build -f /path/to/project

# Build and stamp a reference ID into each devportal manifest (declarative mode)
ap devportal build -f /path/to/project --reference-id 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575 --gateway-type wso2/api-platform`
)

var (
	buildProjectDir  string
	buildReferenceID string
	buildGatewayType string
)

var buildCmd = &cobra.Command{
	Use:     BuildCmdLiteral,
	Short:   "Build the project for devportal",
	Long:    "Build the project located in the specified directory (or current directory if not specified) for deployment to the devportal.",
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
	utils.AddStringFlag(buildCmd, utils.FlagReferenceID, &buildReferenceID, "", "Reference ID to set under spec.referenceID in every devportal manifest")
	utils.AddStringFlag(buildCmd, utils.FlagGatewayType, &buildGatewayType, "", "Gateway type to set under spec.gatewayType in every devportal manifest")
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

	// Build only zips devportal folders; it never generates their contents
	// (that is `ap devportal gen`'s job, which also registers the config). As a
	// fallback, if the project config carries no devportal configs but the
	// default ./devportal folder exists, register it here and persist it so the
	// build can proceed.
	if len(projectConfig.DevPortals) == 0 {
		portalConfig, err := registerDefaultDevPortalConfig(projectRoot)
		if err != nil {
			return err
		}
		projectConfig.DevPortals = append(projectConfig.DevPortals, *portalConfig)
		if err := project.Save(projectConfigPath, projectConfig); err != nil {
			return err
		}
	}

	for i := range projectConfig.DevPortals {
		normalizeDevPortalProjectConfig(&projectConfig.DevPortals[i])
	}

	buildDir := filepath.Join(projectRoot, "build")
	if err := os.RemoveAll(buildDir); err != nil {
		return fmt.Errorf("failed to clean build directory: %w", err)
	}
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to recreate build directory: %w", err)
	}

	zipPaths, failures, err := buildDevPortalArchives(projectRoot, buildDir, projectConfig.DevPortals)
	if err != nil {
		return err
	}

	for _, zipPath := range zipPaths {
		fmt.Printf("DevPortal build created at %s\n", zipPath)
	}

	if len(failures) > 0 {
		messages := make([]string, 0, len(failures))
		for _, failure := range failures {
			fmt.Fprintf(os.Stderr, "DevPortal build failed for %q: %v\n", failure.name, failure.err)
			messages = append(messages, failure.err.Error())
		}
		return fmt.Errorf("failed to build %d of %d devportal configuration(s): %s",
			len(failures), len(projectConfig.DevPortals), strings.Join(messages, "; "))
	}

	return nil
}

// failedPortal records a devportal config that could not be built so the others
// can still be archived and the failures reported together.
type failedPortal struct {
	name string
	err  error
}

// defaultDevPortalConfig returns the portal config describing a project's
// default ./devportal folder. Both `gen` (when generating the folder) and
// `build` (as a fallback when no config is registered) use it so the layout
// stays in one place.
func defaultDevPortalConfig() project.PortalConfig {
	return project.PortalConfig{
		Name:       "default",
		PortalRoot: "./devportal",
		FilePaths: project.PortalFilePaths{
			MetadataFile: "./devportal.yaml",
			Definition:   "./definition.yaml",
			Docs:         "./docs",
			Content:      "./content",
		},
	}
}

// registerDefaultDevPortalConfig is build's fallback path: when the project
// config carries no devportal configs but the default ./devportal folder
// exists on disk, it returns the config describing that folder so build can
// archive it. The folder's contents are produced by `ap devportal gen`; build
// never generates them. It errors if the folder is missing.
func registerDefaultDevPortalConfig(projectRoot string) (*project.PortalConfig, error) {
	portalConfig := defaultDevPortalConfig()

	portalRoot := resolveProjectPath(projectRoot, portalConfig.PortalRoot)
	info, err := os.Stat(portalRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no devportal artifact found at %s; run 'ap devportal gen' to generate one", portalRoot)
		}
		return nil, fmt.Errorf("failed to inspect devportal directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("expected devportal artifact directory but found a file at %s", portalRoot)
	}

	return &portalConfig, nil
}

func resolveProjectPath(projectRoot, pathValue string) string {
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return projectRoot
	}

	trimmed = strings.TrimPrefix(trimmed, "./")
	return filepath.Join(projectRoot, filepath.Clean(trimmed))
}

func normalizeDevPortalProjectConfig(config *project.PortalConfig) {
	if strings.TrimSpace(config.Name) == "" {
		config.Name = "default"
	}
	if strings.TrimSpace(config.PortalRoot) == "" {
		config.PortalRoot = "./devportal"
	}
	if strings.TrimSpace(config.FilePaths.MetadataFile) == "" {
		config.FilePaths.MetadataFile = "./devportal.yaml"
	}
	if strings.TrimSpace(config.FilePaths.Definition) == "" {
		config.FilePaths.Definition = "./definition.yaml"
	}
	if strings.TrimSpace(config.FilePaths.Docs) == "" {
		config.FilePaths.Docs = "./docs"
	}
	if strings.TrimSpace(config.FilePaths.Content) == "" {
		config.FilePaths.Content = "./content"
	}
}

func buildDevPortalArchives(projectRoot, buildDir string, portalConfigs []project.PortalConfig) ([]string, []failedPortal, error) {
	if err := ensureUniqueDevPortalZipNames(portalConfigs); err != nil {
		return nil, nil, err
	}

	zipPaths := make([]string, 0, len(portalConfigs))
	failures := make([]failedPortal, 0)

	for i := range portalConfigs {
		zipPath, err := buildSingleDevPortalArchive(projectRoot, buildDir, &portalConfigs[i])
		if err != nil {
			failures = append(failures, failedPortal{name: portalConfigs[i].Name, err: err})
			continue
		}
		zipPaths = append(zipPaths, zipPath)
	}

	return zipPaths, failures, nil
}

// buildSingleDevPortalArchive validates one devportal config, stamps any
// --reference-id / --gateway-type overrides into its manifest, and zips it.
// Returning an error here drops only this config; the caller keeps building the
// rest.
func buildSingleDevPortalArchive(projectRoot, buildDir string, portalConfig *project.PortalConfig) (string, error) {
	if err := validateDevPortalConfig(projectRoot, portalConfig); err != nil {
		return "", err
	}

	if err := applyManifestOverrides(projectRoot, portalConfig); err != nil {
		return "", err
	}

	stagingDir, err := createDevPortalArchiveStagingDir(projectRoot, buildDir, portalConfig)
	if err != nil {
		return "", err
	}

	zipPath := filepath.Join(buildDir, buildDevPortalZipFileName(portalConfig.Name))
	if err := utils.ZipDirectory(stagingDir, zipPath); err != nil {
		_ = os.RemoveAll(stagingDir)
		return "", fmt.Errorf("failed to build devportal archive for %s: %w", portalConfig.Name, err)
	}
	if err := os.RemoveAll(stagingDir); err != nil {
		return "", fmt.Errorf("failed to clean staging directory for devportal config %q: %w", portalConfig.Name, err)
	}

	return zipPath, nil
}

// applyManifestOverrides stamps the build-time --reference-id / --gateway-type
// flags into the devportal manifest under spec.referenceID / spec.gatewayType.
// The manifest itself carries no reference ID by default; supplying one at
// build time lets the same artifact be published to different devportals (each
// wired to a different gateway). Existing manifest fields and ordering are
// preserved.
func applyManifestOverrides(projectRoot string, portalConfig *project.PortalConfig) error {
	referenceID := strings.TrimSpace(buildReferenceID)
	gatewayType := strings.TrimSpace(buildGatewayType)
	if referenceID == "" && gatewayType == "" {
		return nil
	}

	manifestPath := resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.MetadataFile)
	if err := ensureWithinProjectRoot(projectRoot, manifestPath, portalConfig.Name, "metadataFile"); err != nil {
		return err
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read devportal manifest for config %q: %w", portalConfig.Name, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("failed to parse devportal manifest for config %q: %w", portalConfig.Name, err)
	}

	root := &doc
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("devportal manifest for config %q is not a mapping", portalConfig.Name)
	}

	spec := mappingValueNode(root, "spec")
	if spec == nil {
		spec = &yaml.Node{Kind: yaml.MappingNode}
		setMappingChild(root, "spec", spec)
	}
	if referenceID != "" {
		setMappingScalar(spec, "referenceID", referenceID)
	}
	if gatewayType != "" {
		setMappingScalar(spec, "gatewayType", gatewayType)
	}

	out, err := marshalNode(&doc)
	if err != nil {
		return fmt.Errorf("failed to marshal devportal manifest for config %q: %w", portalConfig.Name, err)
	}
	if err := os.WriteFile(manifestPath, out, 0644); err != nil {
		return fmt.Errorf("failed to write devportal manifest for config %q: %w", portalConfig.Name, err)
	}

	return nil
}

func validateDevPortalConfig(projectRoot string, portalConfig *project.PortalConfig) error {
	portalRoot := resolveProjectPath(projectRoot, portalConfig.PortalRoot)
	if err := ensureWithinProjectRoot(projectRoot, portalRoot, portalConfig.Name, "portalRoot"); err != nil {
		return err
	}
	if err := ensurePathExists(portalRoot, true, portalConfig.Name, "portalRoot"); err != nil {
		return err
	}

	requiredPaths := []struct {
		label string
		path  string
		isDir bool
	}{
		{label: "metadataFile", path: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.MetadataFile), isDir: false},
		{label: "definition", path: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.Definition), isDir: false},
		{label: "docs", path: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.Docs), isDir: true},
		{label: "content", path: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.Content), isDir: true},
	}

	for _, requiredPath := range requiredPaths {
		if err := ensureWithinProjectRoot(projectRoot, requiredPath.path, portalConfig.Name, requiredPath.label); err != nil {
			return err
		}
		if err := ensurePathExists(requiredPath.path, requiredPath.isDir, portalConfig.Name, requiredPath.label); err != nil {
			return err
		}
	}

	return nil
}

// ensureWithinProjectRoot rejects resolved paths that escape the project root
// (e.g. via ".." segments or symlinks in a config value), keeping build inputs
// bounded to the project directory before anything is copied into the archive.
func ensureWithinProjectRoot(projectRoot, path, portalName, fieldName string) error {
	canonicalRoot, err := canonicalizePath(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve project root for devportal config %q: %w", portalName, err)
	}
	canonicalTarget, err := canonicalizePath(path)
	if err != nil {
		return fmt.Errorf("failed to resolve %s for devportal config %q: %w", fieldName, portalName, err)
	}

	rel, err := filepath.Rel(canonicalRoot, canonicalTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("devportal config %q is invalid: %s path resolves outside the project root: %s", portalName, fieldName, path)
	}

	return nil
}

// canonicalizePath returns an absolute, symlink-resolved form of path so that
// containment checks are reliable across differing path forms. When the path
// does not yet exist (it may be validated before creation), it resolves
// symlinks on the nearest existing ancestor and re-appends the remaining
// segments rather than failing.
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
			// Reached the filesystem root without an existing ancestor; fall
			// back to the lexically cleaned absolute path.
			return abs, nil
		}
		remainder = filepath.Join(filepath.Base(current), remainder)
		current = parent
	}
}

func ensurePathExists(path string, wantDir bool, portalName, fieldName string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("devportal config %q is invalid: %s path does not exist: %s", portalName, fieldName, path)
		}
		return fmt.Errorf("failed to inspect %s for devportal config %q: %w", fieldName, portalName, err)
	}

	if wantDir && !info.IsDir() {
		return fmt.Errorf("devportal config %q is invalid: %s must be a directory: %s", portalName, fieldName, path)
	}
	if !wantDir && info.IsDir() {
		return fmt.Errorf("devportal config %q is invalid: %s must be a file: %s", portalName, fieldName, path)
	}

	return nil
}

func createDevPortalArchiveStagingDir(projectRoot, buildDir string, portalConfig *project.PortalConfig) (string, error) {
	stagingRoot, err := os.MkdirTemp(buildDir, "devportal-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create staging directory for devportal config %q: %w", portalConfig.Name, err)
	}
	cleanupOnError := true
	defer func() {
		if cleanupOnError {
			_ = os.RemoveAll(stagingRoot)
		}
	}()

	copyOperations := []struct {
		source string
		target string
		isDir  bool
	}{
		{
			source: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.MetadataFile),
			target: filepath.Join(stagingRoot, archiveMetadataFileName),
			isDir:  false,
		},
		{
			source: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.Definition),
			target: filepath.Join(stagingRoot, archiveDefinitionFileName),
			isDir:  false,
		},
		{
			source: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.Docs),
			target: filepath.Join(stagingRoot, archiveDocsDirName),
			isDir:  true,
		},
		{
			source: resolvePortalConfigPath(projectRoot, portalConfig, portalConfig.FilePaths.Content),
			target: filepath.Join(stagingRoot, archiveContentDirName),
			isDir:  true,
		},
	}

	for _, copyOperation := range copyOperations {
		if copyOperation.isDir {
			if err := copyDirectory(copyOperation.source, copyOperation.target); err != nil {
				return "", err
			}
			continue
		}

		if err := copyFile(copyOperation.source, copyOperation.target); err != nil {
			return "", err
		}
	}

	cleanupOnError = false
	return stagingRoot, nil
}

func resolvePortalConfigPath(projectRoot string, portalConfig *project.PortalConfig, pathValue string) string {
	portalRoot := resolveProjectPath(projectRoot, portalConfig.PortalRoot)
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return portalRoot
	}

	return filepath.Clean(filepath.Join(portalRoot, trimmed))
}

// ensureUniqueDevPortalZipNames fails fast when two devportal configs sanitize
// to the same archive filename, which would otherwise silently overwrite an
// earlier artifact during the build loop.
func ensureUniqueDevPortalZipNames(portalConfigs []project.PortalConfig) error {
	seen := make(map[string]string, len(portalConfigs))
	for i := range portalConfigs {
		displayName := strings.TrimSpace(portalConfigs[i].Name)
		if displayName == "" {
			displayName = "default"
		}

		zipName := buildDevPortalZipFileName(portalConfigs[i].Name)
		if existing, ok := seen[zipName]; ok {
			return fmt.Errorf("devportal configs %q and %q both map to archive %q; rename one of them to avoid overwriting the artifact", existing, displayName, zipName)
		}
		seen[zipName] = displayName
	}

	return nil
}

func buildDevPortalZipFileName(portalName string) string {
	trimmedName := strings.TrimSpace(portalName)
	if trimmedName == "" || trimmedName == "default" {
		return "devportal.zip"
	}

	return fmt.Sprintf("devportal_%s.zip", sanitizeArchiveName(trimmedName))
}

func sanitizeArchiveName(value string) string {
	sanitized := strings.ToLower(strings.TrimSpace(value))
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	sanitized = strings.ReplaceAll(sanitized, "_", "-")
	sanitized = strings.Trim(sanitized, "-.")
	if sanitized == "" {
		return "default"
	}

	return sanitized
}

func copyFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", sourcePath, err)
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create target directory for %s: %w", targetPath, err)
	}

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target file %s: %w", targetPath, err)
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w", sourcePath, targetPath, err)
	}

	return nil
}

func copyDirectory(sourceDir, targetDir string) error {
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("failed to reset target directory %s: %w", targetDir, err)
	}

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to resolve relative path for %s: %w", path, err)
		}

		targetPath := filepath.Join(targetDir, relPath)
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		return copyFile(path, targetPath)
	})
}

// marshalManifest renders a generated manifest with the 2-space indentation
// used across the project's YAML artifacts.
func marshalManifest(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// marshalNode renders an already-parsed YAML node, preserving its structure and
// comments while applying the project's 2-space indentation.
func marshalNode(node *yaml.Node) ([]byte, error) {
	return marshalManifest(node)
}

// mappingValueNode returns the value node for key in a mapping node, or nil.
func mappingValueNode(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// setMappingChild sets (or replaces) the value node for key in a mapping node.
func setMappingChild(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value)
}

// setMappingScalar sets a string-valued key in a mapping node, updating it in
// place if present so surrounding fields and ordering are preserved.
func setMappingScalar(mapping *yaml.Node, key, value string) {
	if mapping.Kind != yaml.MappingNode {
		mapping.Kind = yaml.MappingNode
	}
	if existing := mappingValueNode(mapping, key); existing != nil {
		existing.Kind = yaml.ScalarNode
		existing.Tag = "!!str"
		existing.Value = value
		existing.Style = 0
		return
	}
	setMappingChild(mapping, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}

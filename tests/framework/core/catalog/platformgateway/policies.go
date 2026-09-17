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
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package platformgateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/tests/framework/core/builder"
)

const (
	policyWorkspaceBase = ".wso2-apip-gateway-build"
	policyWorkspaceDir  = "policies"
	policyTargetDir     = "target"
	policyOutputDir     = "output"
	policyExportDir     = "controller-policies"
)

// DerivedImages contains the controller and runtime images produced by a custom gateway build.
type DerivedImages struct {
	Controller string
	Runtime    string
}

// BuildSourceWithPolicies builds the current platform-gateway source with a complete
// local policy tree staged into the gateway build context.
func BuildSourceWithPolicies(
	ctx context.Context, repoRoot, version, source string, runner builder.Runner, coverage bool,
) (DerivedImages, error) {
	if ctx == nil {
		return DerivedImages{}, fmt.Errorf("platform-gateway: build context is required")
	}
	if runner == nil {
		return DerivedImages{}, fmt.Errorf("platform-gateway: build runner is required")
	}
	if strings.TrimSpace(version) == "" {
		return DerivedImages{}, fmt.Errorf("platform-gateway: gateway version is required for a source policy build")
	}
	version = strings.TrimSpace(version)
	root, err := absoluteRepoRoot(repoRoot)
	if err != nil {
		return DerivedImages{}, err
	}
	workspace, err := stagePolicyWorkspace(root, source)
	if err != nil {
		return DerivedImages{}, err
	}
	defer workspace.close()

	images := derivedImages(version, workspace.Digest)
	spec, err := sourcePolicyBuildSpec(version, workspace, images)
	if err != nil {
		return DerivedImages{}, err
	}
	if err := builder.Build(ctx, spec, builder.Request{
		RepoRoot: root,
		Version:  version,
		Coverage: coverage,
		Runner:   runner,
	}); err != nil {
		return DerivedImages{}, err
	}
	return images, nil
}

// BuildVersionedWithPolicies derives runtime and controller images from a versioned
// gateway using the matching gateway-builder and base gateway images.
func BuildVersionedWithPolicies(
	ctx context.Context, repoRoot, version, source, controllerBase, runtimeBase string, runner builder.Runner,
) (DerivedImages, error) {
	if ctx == nil {
		return DerivedImages{}, fmt.Errorf("platform-gateway: build context is required")
	}
	if runner == nil {
		return DerivedImages{}, fmt.Errorf("platform-gateway: build runner is required")
	}
	if strings.TrimSpace(version) == "" {
		return DerivedImages{}, fmt.Errorf("platform-gateway: gateway version is required for a versioned policy build")
	}
	version = strings.TrimSpace(version)
	if strings.TrimSpace(controllerBase) == "" || strings.TrimSpace(runtimeBase) == "" {
		return DerivedImages{}, fmt.Errorf("platform-gateway: versioned policy build requires controller and runtime base images")
	}
	controllerBase = strings.TrimSpace(controllerBase)
	runtimeBase = strings.TrimSpace(runtimeBase)
	root, err := absoluteRepoRoot(repoRoot)
	if err != nil {
		return DerivedImages{}, err
	}

	workspace, err := stagePolicyWorkspace(root, source)
	if err != nil {
		return DerivedImages{}, err
	}
	defer workspace.close()

	images := derivedImages(version, workspace.Digest)
	commands := versionedPolicyBuildCommands(root, version, workspace, controllerBase, runtimeBase, images)
	for i, command := range commands {
		if err := runner.Run(ctx, command); err != nil {
			return DerivedImages{}, fmt.Errorf("platform-gateway: versioned policy build command %d: %w", i+1, err)
		}
	}
	return images, nil
}

func absoluteRepoRoot(repoRoot string) (string, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return "", fmt.Errorf("platform-gateway: repository root is required")
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", fmt.Errorf("platform-gateway: resolving repository root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("platform-gateway: reading repository root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("platform-gateway: repository root %q is not a directory", repoRoot)
	}
	return root, nil
}

// policyWorkspace is a Docker-accessible, self-contained gateway build workspace.
type policyWorkspace struct {
	Root               string
	BuildFile          string
	Policies           string
	Target             string
	ControllerPolicies string
	Digest             string
}

// close removes the framework-owned policy workspace.
func (w policyWorkspace) close() {
	if w.Root != "" {
		_ = os.RemoveAll(w.Root)
	}
}

type policyBuildFile struct {
	Version  string              `yaml:"version"`
	Gateway  gatewayBuildOptions `yaml:"gateway"`
	Policies []policyBuildEntry  `yaml:"policies"`
}

type gatewayBuildOptions struct {
	Version string `yaml:"version"`
}

type policyBuildEntry struct {
	Name     string `yaml:"name"`
	FilePath string `yaml:"filePath"`
}

type policyDefinition struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

func stagePolicyWorkspace(repoRoot, source string) (policyWorkspace, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: repository root is required")
	}
	if strings.TrimSpace(source) == "" {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: policy source is required")
	}
	if filepath.IsAbs(source) {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: policy source must be relative: %q", source)
	}

	root, err := absoluteRepoRoot(repoRoot)
	if err != nil {
		return policyWorkspace{}, err
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: canonicalizing repository root: %w", err)
	}
	sourcePath, err := filepath.Abs(filepath.Join(root, source))
	if err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: resolving policy source: %w", err)
	}
	sourcePath, err = filepath.EvalSymlinks(sourcePath)
	if err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: resolving policy source %q: %w", source, err)
	}
	if !pathWithin(canonicalRoot, sourcePath) {
		approvedSibling := filepath.Join(root, "..", "gateway-controllers", "policies")
		canonicalSibling, siblingErr := filepath.EvalSymlinks(approvedSibling)
		if siblingErr != nil || !pathWithin(canonicalSibling, sourcePath) {
			return policyWorkspace{}, fmt.Errorf(
				"platform-gateway: policy source %q must resolve within repository root or ../gateway-controllers/policies",
				source,
			)
		}
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: reading policy source %q: %w", source, err)
	}
	if !info.IsDir() {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: policy source %q is not a directory", source)
	}

	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: reading policy source %q: %w", source, err)
	}
	if len(entries) == 0 {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: policy source %q is empty", source)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: locating home directory: %w", err)
	}
	base := filepath.Join(home, policyWorkspaceBase)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: creating workspace base: %w", err)
	}
	workspaceRoot, err := os.MkdirTemp(base, "build-")
	if err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: creating policy workspace: %w", err)
	}
	workspace := policyWorkspace{
		Root:               workspaceRoot,
		BuildFile:          filepath.Join(workspaceRoot, "build.yaml"),
		Policies:           filepath.Join(workspaceRoot, policyWorkspaceDir),
		Target:             filepath.Join(workspaceRoot, policyTargetDir),
		ControllerPolicies: filepath.Join(workspaceRoot, policyExportDir),
	}
	keep := false
	defer func() {
		if !keep {
			workspace.close()
		}
	}()

	if err := os.MkdirAll(workspace.Policies, 0o755); err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: creating staged policy directory: %w", err)
	}
	if err := os.MkdirAll(workspace.ControllerPolicies, 0o755); err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: creating controller policy directory: %w", err)
	}
	if err := prepareSourceBuildTarget(root, workspace.Target); err != nil {
		return policyWorkspace{}, err
	}

	manifest := policyBuildFile{
		Version:  "v1",
		Gateway:  gatewayBuildOptions{Version: ""},
		Policies: make([]policyBuildEntry, 0, len(entries)),
	}
	seen := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return policyWorkspace{}, fmt.Errorf("platform-gateway: policy source entry %q is a symlink", entry.Name())
		}
		if !entry.IsDir() {
			return policyWorkspace{}, fmt.Errorf("platform-gateway: policy source entry %q is not a directory", entry.Name())
		}

		src := filepath.Join(sourcePath, entry.Name())
		definition, err := readPolicyDefinition(src)
		if err != nil {
			return policyWorkspace{}, fmt.Errorf("platform-gateway: policy %q: %w", entry.Name(), err)
		}
		if previous, exists := seen[definition.Name]; exists {
			return policyWorkspace{}, fmt.Errorf("platform-gateway: duplicate policy name %q in %q and %q",
				definition.Name, previous, entry.Name())
		}
		seen[definition.Name] = entry.Name()

		dst := filepath.Join(workspace.Policies, entry.Name())
		if err := copyTree(src, dst); err != nil {
			return policyWorkspace{}, fmt.Errorf("platform-gateway: staging policy %q: %w", entry.Name(), err)
		}
		manifest.Policies = append(manifest.Policies, policyBuildEntry{
			Name:     definition.Name,
			FilePath: filepath.ToSlash(filepath.Join(policyWorkspaceDir, entry.Name())),
		})
	}

	data, err := yaml.Marshal(&manifest)
	if err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: encoding policy build manifest: %w", err)
	}
	if err := os.WriteFile(workspace.BuildFile, data, 0o644); err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: writing policy build manifest: %w", err)
	}
	sourceManifest := manifest
	for i := range sourceManifest.Policies {
		sourceManifest.Policies[i].FilePath = strings.Replace(
			sourceManifest.Policies[i].FilePath, policyWorkspaceDir+"/", "dev-policies/", 1,
		)
	}
	sourceData, err := yaml.Marshal(&sourceManifest)
	if err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: encoding source policy build manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(workspace.Target, "build.yaml"), sourceData, 0o644); err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: writing source build manifest: %w", err)
	}

	digest, err := directoryDigest(workspace.Root)
	if err != nil {
		return policyWorkspace{}, fmt.Errorf("platform-gateway: hashing policy workspace: %w", err)
	}
	workspace.Digest = digest
	keep = true
	return workspace, nil
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func readPolicyDefinition(policyDir string) (policyDefinition, error) {
	path := filepath.Join(policyDir, "policy-definition.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return policyDefinition{}, fmt.Errorf("reading policy-definition.yaml: %w", err)
	}
	var definition policyDefinition
	if err := yaml.Unmarshal(data, &definition); err != nil {
		return policyDefinition{}, fmt.Errorf("parsing policy-definition.yaml: %w", err)
	}
	if strings.TrimSpace(definition.Name) == "" {
		return policyDefinition{}, fmt.Errorf("policy-definition.yaml has no name")
	}
	if strings.TrimSpace(definition.Version) == "" {
		return policyDefinition{}, fmt.Errorf("policy-definition.yaml has no version")
	}
	return definition, nil
}

func prepareSourceBuildTarget(repoRoot, target string) error {
	if err := os.MkdirAll(filepath.Join(target, "configs", "llm-pricing"), 0o755); err != nil {
		return fmt.Errorf("platform-gateway: creating source build target: %w", err)
	}
	if err := copyFile(filepath.Join(repoRoot, "LICENSE"), filepath.Join(target, "LICENSE")); err != nil {
		return fmt.Errorf("platform-gateway: staging LICENSE: %w", err)
	}
	if err := copyTree(
		filepath.Join(repoRoot, "gateway", "configs", "llm-pricing"),
		filepath.Join(target, "configs", "llm-pricing"),
	); err != nil {
		return fmt.Errorf("platform-gateway: staging LLM pricing configuration: %w", err)
	}
	return nil
}

func sourcePolicyBuildSpec(version string, workspace policyWorkspace, images DerivedImages) (builder.Spec, error) {
	base, err := BuildSpec(version)
	if err != nil {
		return builder.Spec{}, err
	}

	return builder.Spec{
		Component: base.Component,
		SourceDir: base.SourceDir,
		Images:    base.Images,
		Coverage:  base.Coverage,
		Plan: func(root, v string, coverage builder.CoverageSpec) ([]builder.Command, error) {
			return sourcePolicyBuildCommands(root, v, workspace, images.Controller, images.Runtime, coverage), nil
		},
	}, nil
}

func sourcePolicyBuildCommands(
	repoRoot, version string, workspace policyWorkspace, controller, runtime string, coverage builder.CoverageSpec,
) []builder.Command {
	root := filepath.Clean(repoRoot)
	runtimeDockerfile := filepath.Join(root, "gateway", "gateway-runtime", "Dockerfile")
	controllerDockerfile := filepath.Join(root, "gateway", "gateway-controller", "Dockerfile")

	buildBase := []string{
		"docker", "buildx", "build", "-f", runtimeDockerfile,
		"--build-context", "sdk=" + filepath.Join(root, "sdk"),
		"--build-context", "sdk-python=" + filepath.Join(root, "sdk-python"),
		"--build-context", "sdk-core=" + filepath.Join(root, "sdk", "core"),
		"--build-context", "common=" + filepath.Join(root, "common"),
		"--build-context", "gateway-common=" + filepath.Join(root, "gateway", "common"),
		"--build-context", "httpkit=" + filepath.Join(root, "httpkit"),
		"--build-context", "gateway-builder=" + filepath.Join(root, "gateway", "gateway-builder"),
		"--build-context", "system-policies=" + filepath.Join(root, "gateway", "system-policies"),
		"--build-context", "dev-policies=" + workspace.Policies,
		"--build-context", "target=" + workspace.Target,
		"--build-arg", "VERSION=" + version,
		"--build-arg", "GIT_COMMIT=framework",
	}
	runtimeBuild := append([]string(nil), buildBase...)
	runtimeBuild = append(runtimeBuild, builder.CoverageBuildArgs(coverage)...)
	runtimeBuild = append(runtimeBuild, "--target", "production", "--tag", runtime, "--load", filepath.Join(root, "gateway", "gateway-runtime"))

	policyExport := append([]string(nil), buildBase...)
	policyExport = append(policyExport, "--target", "policy-export", "--output", "type=local,dest="+workspace.ControllerPolicies,
		filepath.Join(root, "gateway", "gateway-runtime"))

	controllerBuild := []string{
		"docker", "buildx", "build", "-f", controllerDockerfile,
		"--build-context", "sdk=" + filepath.Join(root, "sdk"),
		"--build-context", "sdk-core=" + filepath.Join(root, "sdk", "core"),
		"--build-context", "common=" + filepath.Join(root, "common"),
		"--build-context", "gateway-common=" + filepath.Join(root, "gateway", "common"),
		"--build-context", "httpkit=" + filepath.Join(root, "httpkit"),
		"--build-context", "build-manifest=" + workspace.Root,
		"--build-context", "policies=" + workspace.ControllerPolicies,
		"--build-context", "target=" + workspace.Target,
		"--build-arg", "VERSION=" + version,
		"--build-arg", "GIT_COMMIT=framework",
	}
	controllerBuild = append(controllerBuild, builder.CoverageBuildArgs(coverage)...)
	controllerBuild = append(controllerBuild, "--target", "production", "--tag", controller, "--load", filepath.Join(root, "gateway", "gateway-controller"))

	manifest := filepath.Join(workspace.Root, "build-manifest.yaml")
	extractManifest := []string{
		"sh", "-c", "docker run --rm --entrypoint cat \"$1\" /app/build-manifest.yaml > \"$2\"",
		"sh", runtime, manifest,
	}
	return []builder.Command{
		{Directory: root, Args: runtimeBuild},
		{Directory: root, Args: policyExport},
		{Directory: root, Args: extractManifest},
		{Directory: root, Args: controllerBuild},
	}
}

func versionedPolicyBuildCommands(
	repoRoot, version string, workspace policyWorkspace, controllerBase, runtimeBase string, images DerivedImages,
) []builder.Command {
	builderImage := "ghcr.io/wso2/api-platform/gateway-builder:" + version
	return []builder.Command{
		{
			Directory: repoRoot,
			Args: []string{
				"docker", "run", "--rm", "-v", workspace.Root + ":/workspace", builderImage,
				"-gateway-controller-base-image", controllerBase,
				"-gateway-runtime-base-image", runtimeBase,
				"-build-file", "/workspace/build.yaml",
				"-out-dir", "/workspace/output",
			},
		},
		{
			Directory: repoRoot,
			Args: []string{
				"docker", "buildx", "build", "-f", filepath.Join(workspace.Root, policyOutputDir, "gateway-runtime", "Dockerfile"),
				"--tag", images.Runtime, "--load", filepath.Join(workspace.Root, policyOutputDir, "gateway-runtime"),
			},
		},
		{
			Directory: repoRoot,
			Args: []string{
				"docker", "buildx", "build", "-f", filepath.Join(workspace.Root, policyOutputDir, "gateway-controller", "Dockerfile"),
				"--tag", images.Controller, "--load", filepath.Join(workspace.Root, policyOutputDir, "gateway-controller"),
			},
		},
	}
}

func derivedImages(version, digest string) DerivedImages {
	version = safeTagPart(version)
	if len(digest) > 16 {
		digest = digest[:16]
	}
	tag := "framework-" + version + "-policies-" + digest
	return DerivedImages{
		Controller: "local/apip-gateway-controller:" + tag,
		Runtime:    "local/apip-gateway-runtime:" + tag,
	}
}

func safeTagPart(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	value = strings.Trim(b.String(), "-._")
	if value == "" {
		return "version"
	}
	return value
}

func directoryDigest(root string) (string, error) {
	hash := sha256.New()
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink %q is not allowed", path)
		}
		if entry.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		if _, err := io.WriteString(hash, filepath.ToSlash(rel)+"\x00"); err != nil {
			return "", err
		}
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func copyTree(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlink %q is not allowed", source)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", source)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		src := filepath.Join(source, entry.Name())
		dst := filepath.Join(destination, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink %q is not allowed", src)
		}
		if entry.IsDir() {
			if err := copyTree(src, dst); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(source, destination string) (err error) {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := input.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

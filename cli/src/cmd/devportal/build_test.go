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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/cli/internal/project"
)

func TestEnsureUniqueDevPortalZipNames_DetectsCollision(t *testing.T) {
	configs := []project.PortalConfig{
		{Name: "My Portal"},
		{Name: "my_portal"},
	}

	err := ensureUniqueDevPortalZipNames(configs)
	if err == nil {
		t.Fatalf("expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), "devportal_my-portal.zip") {
		t.Fatalf("expected error to name the conflicting archive, got %v", err)
	}
}

func TestEnsureUniqueDevPortalZipNames_AllowsDistinctNames(t *testing.T) {
	configs := []project.PortalConfig{
		{Name: ""},
		{Name: "eu"},
		{Name: "us"},
	}

	if err := ensureUniqueDevPortalZipNames(configs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureWithinProjectRoot_RejectsEscapingPath(t *testing.T) {
	projectRoot := t.TempDir()
	escaping := filepath.Join(projectRoot, "..", "outside")

	err := ensureWithinProjectRoot(projectRoot, escaping, "default", "portalRoot")
	if err == nil {
		t.Fatalf("expected error for escaping path, got nil")
	}
	if !strings.Contains(err.Error(), "resolves outside the project root") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestEnsureWithinProjectRoot_AllowsContainedPaths(t *testing.T) {
	projectRoot := t.TempDir()

	cases := []string{
		projectRoot,                                              // the root itself
		filepath.Join(projectRoot, "devportal"),                 // nested dir
		filepath.Join(projectRoot, "devportal", "metadata.yaml"), // nested file
	}
	for _, path := range cases {
		if err := ensureWithinProjectRoot(projectRoot, path, "default", "portalRoot"); err != nil {
			t.Fatalf("unexpected error for contained path %q: %v", path, err)
		}
	}
}

func TestEnsureWithinProjectRoot_RejectsSymlinkEscape(t *testing.T) {
	projectRoot := t.TempDir()
	outside := t.TempDir()

	// A symlink inside the project that points outside it must be rejected even
	// though its lexical path is under the root.
	link := filepath.Join(projectRoot, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}

	err := ensureWithinProjectRoot(projectRoot, filepath.Join(link, "secret.yaml"), "default", "metadataFile")
	if err == nil || !strings.Contains(err.Error(), "resolves outside the project root") {
		t.Fatalf("expected symlink escape to be rejected, got %v", err)
	}
}

func TestValidateDevPortalConfig_RejectsEscapingPortalRoot(t *testing.T) {
	projectRoot := t.TempDir()
	portalConfig := &project.PortalConfig{Name: "default", PortalRoot: "../../etc"}

	err := validateDevPortalConfig(projectRoot, portalConfig)
	if err == nil {
		t.Fatalf("expected error for escaping portalRoot, got nil")
	}
	if !strings.Contains(err.Error(), "resolves outside the project root") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRegisterDefaultDevPortalConfig_RequiresExistingFolder(t *testing.T) {
	projectRoot := t.TempDir()

	_, err := registerDefaultDevPortalConfig(projectRoot)
	if err == nil || !strings.Contains(err.Error(), "no devportal artifact found") {
		t.Fatalf("expected missing-folder error, got %v", err)
	}
}

func TestRegisterDefaultDevPortalConfig_ReturnsConfigForExistingFolder(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, "devportal"), 0755); err != nil {
		t.Fatalf("failed to create devportal dir: %v", err)
	}

	portalConfig, err := registerDefaultDevPortalConfig(projectRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if portalConfig.Name != "default" || portalConfig.PortalRoot != "./devportal" {
		t.Fatalf("unexpected portal config: %+v", portalConfig)
	}
}

func TestRunBuildCommand_RequiresProjectDirectory(t *testing.T) {
	workDir := t.TempDir()
	buildProjectDir = workDir

	err := runBuildCommand()
	if err == nil || err.Error() != "unable to find project directory, please execute this command inside a project" {
		t.Fatalf("expected project directory error, got %v", err)
	}
}

func TestRunBuildCommand_RequiresDevPortalFolderWhenNoConfig(t *testing.T) {
	projectRoot := createProjectFixture(t)
	buildProjectDir = projectRoot

	err := runBuildCommand()
	if err == nil || !strings.Contains(err.Error(), "no devportal artifact found") {
		t.Fatalf("expected missing-folder error, got %v", err)
	}
}

func TestRunBuildCommand_RegistersExistingFolderAndArchives(t *testing.T) {
	projectRoot := createProjectFixture(t)
	createDevPortalFolderFixture(t, projectRoot, "devportal")
	buildProjectDir = projectRoot

	if err := runBuildCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectRoot, "build", "devportal.zip")); err != nil {
		t.Fatalf("expected devportal.zip to be created: %v", err)
	}

	projectConfig, err := project.Load(filepath.Join(projectRoot, ".api-platform", "config.yaml"))
	if err != nil {
		t.Fatalf("failed to load updated project config: %v", err)
	}
	if len(projectConfig.DevPortals) != 1 || projectConfig.DevPortals[0].Name != "default" {
		t.Fatalf("expected default devportal config to be registered, got %+v", projectConfig.DevPortals)
	}
}

func TestRunBuildCommand_StampsReferenceIDAndGatewayType(t *testing.T) {
	projectRoot := createProjectFixture(t)
	createDevPortalFolderFixture(t, projectRoot, "devportal")
	buildProjectDir = projectRoot

	buildReferenceID = "1ba42a09-45c0-40f8-a1bf-e4aa7cde1575"
	buildGatewayType = "wso2/api-platform"
	t.Cleanup(func() {
		buildReferenceID = ""
		buildGatewayType = ""
	})

	if err := runBuildCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	manifestData, readErr := os.ReadFile(filepath.Join(projectRoot, "devportal", "devportal.yaml"))
	if readErr != nil {
		t.Fatalf("failed to read manifest: %v", readErr)
	}
	for _, want := range []string{
		"referenceID: 1ba42a09-45c0-40f8-a1bf-e4aa7cde1575",
		"gatewayType: wso2/api-platform",
	} {
		if !strings.Contains(string(manifestData), want) {
			t.Fatalf("expected stamped manifest to contain %q, got %q", want, string(manifestData))
		}
	}
}

func TestRunBuildCommand_BuildsMultipleConfiguredDevPortalsAndCleansBuildDir(t *testing.T) {
	projectRoot := createProjectFixture(t)

	defaultPortalRoot := filepath.Join(projectRoot, "devportal")
	petstorePortalRoot := filepath.Join(projectRoot, "petstore")
	for _, dir := range []string{
		filepath.Join(defaultPortalRoot, "docs"),
		filepath.Join(defaultPortalRoot, "content"),
		filepath.Join(petstorePortalRoot, "docs"),
		filepath.Join(petstorePortalRoot, "content"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	for _, path := range []string{
		filepath.Join(defaultPortalRoot, "devportal.yaml"),
		filepath.Join(petstorePortalRoot, "devportal.yaml"),
	} {
		if err := os.WriteFile(path, []byte("apiVersion: devportal.api-platform.wso2.com/v1\nkind: RestApi\n"), 0644); err != nil {
			t.Fatalf("failed to write portal manifest: %v", err)
		}
	}

	if err := project.Save(filepath.Join(projectRoot, ".api-platform", "config.yaml"), &project.Config{
		Version:   "1.0.0",
		FilePaths: project.DefaultFilePaths(),
		DevPortals: []project.PortalConfig{
			{
				Name:       "default",
				PortalRoot: "./devportal",
				FilePaths: project.PortalFilePaths{
					MetadataFile: "./devportal.yaml",
					Definition:   "./../definition.yaml",
					Docs:         "./docs",
					Content:      "./content",
				},
			},
			{
				Name:       "petstore",
				PortalRoot: "./petstore",
				FilePaths: project.PortalFilePaths{
					MetadataFile: "./devportal.yaml",
					Definition:   "./../definition.yaml",
					Docs:         "./docs",
					Content:      "./content",
				},
			},
		},
	}); err != nil {
		t.Fatalf("failed to save project config: %v", err)
	}

	buildDir := filepath.Join(projectRoot, "build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "stale.txt"), []byte("stale"), 0644); err != nil {
		t.Fatalf("failed to write stale file: %v", err)
	}

	buildProjectDir = projectRoot
	if err := runBuildCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, path := range []string{
		filepath.Join(buildDir, "devportal.zip"),
		filepath.Join(buildDir, "devportal_petstore.zip"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected zip artifact to exist: %s (%v)", path, err)
		}
	}

	if _, err := os.Stat(filepath.Join(buildDir, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected stale build artifact to be removed, got err=%v", err)
	}
}

func TestRunBuildCommand_PartialSuccessReportsFailureAndBuildsRest(t *testing.T) {
	projectRoot := createProjectFixture(t)

	// "default" is fully set up; "broken" is missing its required content dir.
	defaultPortalRoot := filepath.Join(projectRoot, "devportal")
	brokenPortalRoot := filepath.Join(projectRoot, "broken")
	for _, dir := range []string{
		filepath.Join(defaultPortalRoot, "docs"),
		filepath.Join(defaultPortalRoot, "content"),
		filepath.Join(brokenPortalRoot, "docs"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}
	for _, path := range []string{
		filepath.Join(defaultPortalRoot, "devportal.yaml"),
		filepath.Join(brokenPortalRoot, "devportal.yaml"),
	} {
		if err := os.WriteFile(path, []byte("apiVersion: devportal.api-platform.wso2.com/v1\nkind: RestApi\n"), 0644); err != nil {
			t.Fatalf("failed to write portal manifest: %v", err)
		}
	}

	if err := project.Save(filepath.Join(projectRoot, ".api-platform", "config.yaml"), &project.Config{
		FilePaths: project.DefaultFilePaths(),
		DevPortals: []project.PortalConfig{
			{
				Name:       "default",
				PortalRoot: "./devportal",
				FilePaths: project.PortalFilePaths{
					MetadataFile: "./devportal.yaml",
					Definition:   "./../definition.yaml",
					Docs:         "./docs",
					Content:      "./content",
				},
			},
			{
				Name:       "broken",
				PortalRoot: "./broken",
				FilePaths: project.PortalFilePaths{
					MetadataFile: "./devportal.yaml",
					Definition:   "./../definition.yaml",
					Docs:         "./docs",
					Content:      "./content",
				},
			},
		},
	}); err != nil {
		t.Fatalf("failed to save project config: %v", err)
	}

	buildProjectDir = projectRoot
	err := runBuildCommand()
	if err == nil || !strings.Contains(err.Error(), `devportal config "broken" is invalid: content path does not exist`) {
		t.Fatalf("expected failure for the broken config, got %v", err)
	}

	// The healthy config must still have produced its archive.
	if _, statErr := os.Stat(filepath.Join(projectRoot, "build", "devportal.zip")); statErr != nil {
		t.Fatalf("expected the healthy config to still build its archive: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(projectRoot, "build", "devportal_broken.zip")); !os.IsNotExist(statErr) {
		t.Fatalf("did not expect an archive for the broken config, got err=%v", statErr)
	}
}

func TestRunBuildCommand_ValidatesRequiredContentPath(t *testing.T) {
	projectRoot := createProjectFixture(t)

	portalRoot := filepath.Join(projectRoot, "devportal")
	if err := os.MkdirAll(filepath.Join(portalRoot, "docs"), 0755); err != nil {
		t.Fatalf("failed to create docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(portalRoot, "devportal.yaml"), []byte("kind: RestApi\n"), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	if err := project.Save(filepath.Join(projectRoot, ".api-platform", "config.yaml"), &project.Config{
		FilePaths: project.DefaultFilePaths(),
		DevPortals: []project.PortalConfig{
			{
				Name:       "default",
				PortalRoot: "./devportal",
				FilePaths: project.PortalFilePaths{
					MetadataFile: "./devportal.yaml",
					Definition:   "./../definition.yaml",
					Docs:         "./docs",
					Content:      "./content",
				},
			},
		},
	}); err != nil {
		t.Fatalf("failed to save project config: %v", err)
	}

	buildProjectDir = projectRoot
	err := runBuildCommand()
	if err == nil || !strings.Contains(err.Error(), `devportal config "default" is invalid: content path does not exist`) {
		t.Fatalf("expected content validation error, got %v", err)
	}
}

func createProjectFixture(t *testing.T) string {
	t.Helper()

	projectRoot := t.TempDir()
	for _, dir := range []string{
		filepath.Join(projectRoot, ".api-platform"),
		filepath.Join(projectRoot, "docs"),
		filepath.Join(projectRoot, "tests"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create fixture dir %s: %v", dir, err)
		}
	}

	files := map[string]string{
		filepath.Join(projectRoot, ".api-platform", "config.yaml"): "version: 1.0.0\nfilePaths:\n  deploymentArtifact: ./runtime.yaml\n  metadataFile: ./metadata.yaml\n  definition: ./definition.yaml\n  docs: ./docs\n  tests: ./tests\n",
		filepath.Join(projectRoot, "metadata.yaml"):                "apiVersion: management.api-platform.wso2.com/v1\nkind: RestApi\nmetadata:\n  name: foo-1.0.0\nspec:\n  referenceID: \"\"\n",
		filepath.Join(projectRoot, "runtime.yaml"):                 "apiVersion: gateway.api-platform.wso2.com/v1\nkind: RestApi\nspec:\n  displayName: Petstore API\n  version: 1.0.0\n  subscriptionPlans:\n    - gold\n    - silver\n",
		filepath.Join(projectRoot, "definition.yaml"):              "openapi: 3.0.0\ninfo:\n  title: Petstore API\n",
		filepath.Join(projectRoot, "docs", "README.md"):            "# Docs\n",
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write fixture file %s: %v", path, err)
		}
	}

	return projectRoot
}

// createDevPortalFolderFixture creates a complete devportal artifact folder
// (as `ap devportal gen` would) under projectRoot/portalDir so build has
// something to archive.
func createDevPortalFolderFixture(t *testing.T, projectRoot, portalDir string) {
	t.Helper()

	portalRoot := filepath.Join(projectRoot, portalDir)
	for _, dir := range []string{
		filepath.Join(portalRoot, "docs"),
		filepath.Join(portalRoot, "content"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create devportal fixture dir %s: %v", dir, err)
		}
	}

	files := map[string]string{
		filepath.Join(portalRoot, "devportal.yaml"):  "apiVersion: devportal.api-platform.wso2.com/v1\nkind: RestApi\nmetadata:\n  name: foo-1.0.0\nspec:\n  type: REST\n  displayName: Petstore API\n",
		filepath.Join(portalRoot, "definition.yaml"): "openapi: 3.0.0\ninfo:\n  title: Petstore API\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write devportal fixture file %s: %v", path, err)
		}
	}
}

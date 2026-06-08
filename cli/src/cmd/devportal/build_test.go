package devportal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureUniqueDevPortalZipNames_DetectsCollision(t *testing.T) {
	configs := []devPortalProjectConfig{
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
	configs := []devPortalProjectConfig{
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
		projectRoot,                                         // the root itself
		filepath.Join(projectRoot, "devportal"),             // nested dir
		filepath.Join(projectRoot, "devportal", "api.yaml"), // nested file
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

	err := ensureWithinProjectRoot(projectRoot, filepath.Join(link, "secret.yaml"), "default", "apiMetadata")
	if err == nil || !strings.Contains(err.Error(), "resolves outside the project root") {
		t.Fatalf("expected symlink escape to be rejected, got %v", err)
	}
}

func TestValidateDevPortalConfig_RejectsEscapingPortalRoot(t *testing.T) {
	projectRoot := t.TempDir()
	portalConfig := &devPortalProjectConfig{Name: "default", PortalRoot: "../../etc"}

	err := validateDevPortalConfig(projectRoot, portalConfig)
	if err == nil {
		t.Fatalf("expected error for escaping portalRoot, got nil")
	}
	if !strings.Contains(err.Error(), "resolves outside the project root") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestBuildDefaultDevPortalManifest_RejectsEscapingSourcePath(t *testing.T) {
	projectRoot := t.TempDir()
	projectConfig := &apiProjectConfig{
		FilePaths: apiProjectFilePaths{
			APIMetadata:        "../../../etc/passwd",
			DeploymentArtifact: "./gateway.yaml",
		},
	}

	_, err := buildDefaultDevPortalManifest(projectRoot, projectConfig)
	if err == nil || !strings.Contains(err.Error(), "resolves outside the project root") {
		t.Fatalf("expected containment error, got %v", err)
	}
}

func TestCreateDefaultDevPortalConfig_RejectsEscapingSourcePath(t *testing.T) {
	projectRoot := t.TempDir()
	projectConfig := &apiProjectConfig{
		FilePaths: apiProjectFilePaths{
			APIDefinition: "../../outside/definition.yaml",
			Docs:          "./docs",
		},
	}

	_, err := projectConfig.createDefaultDevPortalConfig(projectRoot)
	if err == nil || !strings.Contains(err.Error(), "resolves outside the project root") {
		t.Fatalf("expected containment error, got %v", err)
	}
}

func TestRunBuildCommand_RequiresAPIProjectDirectory(t *testing.T) {
	workDir := t.TempDir()
	buildProjectDir = workDir

	err := runBuildCommand()
	if err == nil || err.Error() != "unable to find api project directory, please execute this command inside api project" {
		t.Fatalf("expected api project directory error, got %v", err)
	}
}

func TestRunBuildCommand_CreatesDefaultDevPortalArtifacts(t *testing.T) {
	projectRoot := createAPIProjectFixture(t)
	buildProjectDir = projectRoot

	if err := runBuildCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, path := range []string{
		filepath.Join(projectRoot, "devportal"),
		filepath.Join(projectRoot, "devportal", "devportal.yaml"),
		filepath.Join(projectRoot, "devportal", "definition.yaml"),
		filepath.Join(projectRoot, "devportal", "docs"),
		filepath.Join(projectRoot, "devportal", "content"),
		filepath.Join(projectRoot, "build", "devportal.zip"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected path to exist: %s (%v)", path, err)
		}
	}

	projectConfig, err := loadProjectConfig(filepath.Join(projectRoot, ".api-platform", "config.yaml"))
	if err != nil {
		t.Fatalf("failed to load updated project config: %v", err)
	}
	if len(projectConfig.DevPortals) != 1 || projectConfig.DevPortals[0].Name != "default" {
		t.Fatalf("expected default devportal config to be added, got %+v", projectConfig.DevPortals)
	}

	manifestData, err := os.ReadFile(filepath.Join(projectRoot, "devportal", "devportal.yaml"))
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	if !strings.Contains(string(manifestData), "displayName: Petstore API") {
		t.Fatalf("expected generated manifest to contain displayName, got %q", string(manifestData))
	}
}

func TestRunBuildCommand_BuildsMultipleConfiguredDevPortalsAndCleansBuildDir(t *testing.T) {
	projectRoot := createAPIProjectFixture(t)

	defaultPortalRoot := filepath.Join(projectRoot, "devportal")
	petstorePortalRoot := filepath.Join(projectRoot, "petstore")
	if err := os.MkdirAll(filepath.Join(defaultPortalRoot, "docs"), 0755); err != nil {
		t.Fatalf("failed to create docs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(defaultPortalRoot, "content"), 0755); err != nil {
		t.Fatalf("failed to create content: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(petstorePortalRoot, "docs"), 0755); err != nil {
		t.Fatalf("failed to create docs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(petstorePortalRoot, "content"), 0755); err != nil {
		t.Fatalf("failed to create content: %v", err)
	}

	for _, path := range []string{
		filepath.Join(defaultPortalRoot, "devportal.yaml"),
		filepath.Join(petstorePortalRoot, "devportal.yaml"),
	} {
		if err := os.WriteFile(path, []byte("apiVersion: devportal.api-platform.wso2.com/v1\nkind: RestApi\n"), 0644); err != nil {
			t.Fatalf("failed to write portal manifest: %v", err)
		}
	}

	if err := saveProjectConfig(filepath.Join(projectRoot, ".api-platform", "config.yaml"), &apiProjectConfig{
		Version: "1.0.0",
		FilePaths: apiProjectFilePaths{
			DeploymentArtifact: "./gateway.yaml",
			APIMetadata:        "./api.yaml",
			APIDefinition:      "./definition.yaml",
			Docs:               "./docs",
			Tests:              "./tests",
		},
		DevPortals: []devPortalProjectConfig{
			{
				Name:       "default",
				PortalRoot: "./devportal",
				FilePaths: devPortalProjectPaths{
					APIMetadata:   "./devportal.yaml",
					APIDefinition: "./../definition.yaml",
					Docs:          "./docs",
					Content:       "./content",
				},
			},
			{
				Name:       "petstore",
				PortalRoot: "./petstore",
				FilePaths: devPortalProjectPaths{
					APIMetadata:   "./devportal.yaml",
					APIDefinition: "./../definition.yaml",
					Docs:          "./docs",
					Content:       "./content",
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

func TestRunBuildCommand_ValidatesRequiredContentPath(t *testing.T) {
	projectRoot := createAPIProjectFixture(t)

	portalRoot := filepath.Join(projectRoot, "devportal")
	if err := os.MkdirAll(filepath.Join(portalRoot, "docs"), 0755); err != nil {
		t.Fatalf("failed to create docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(portalRoot, "devportal.yaml"), []byte("kind: RestApi\n"), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	if err := saveProjectConfig(filepath.Join(projectRoot, ".api-platform", "config.yaml"), &apiProjectConfig{
		FilePaths: apiProjectFilePaths{
			DeploymentArtifact: "./gateway.yaml",
			APIMetadata:        "./api.yaml",
			APIDefinition:      "./definition.yaml",
			Docs:               "./docs",
			Tests:              "./tests",
		},
		DevPortals: []devPortalProjectConfig{
			{
				Name:       "default",
				PortalRoot: "./devportal",
				FilePaths: devPortalProjectPaths{
					APIMetadata:   "./devportal.yaml",
					APIDefinition: "./../definition.yaml",
					Docs:          "./docs",
					Content:       "./content",
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

func createAPIProjectFixture(t *testing.T) string {
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
		filepath.Join(projectRoot, ".api-platform", "config.yaml"): "version: 1.0.0\nfilePaths:\n  deploymentArtifact: ./gateway.yaml\n  apiMetadata: ./api.yaml\n  apiDefinition: ./definition.yaml\n  docs: ./docs\n  tests: ./tests\n",
		filepath.Join(projectRoot, "api.yaml"):                     "apiVersion: management.api-platform.wso2.com/v1\nkind: Api\nmetadata:\n  name: foo-1.0.0\n",
		filepath.Join(projectRoot, "gateway.yaml"):                 "apiVersion: gateway.api-platform.wso2.com/v1\nkind: RestApi\nspec:\n  displayName: Petstore API\n  version: 1.0.0\n  subscriptionPlans:\n    - gold\n    - silver\n",
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

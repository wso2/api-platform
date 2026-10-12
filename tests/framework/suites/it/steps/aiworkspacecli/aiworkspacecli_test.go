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

package aiworkspacecli

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// testContext carries the runner scope and cleanup registry the steps rely on.
func testContext(t *testing.T) (context.Context, *cleanup.Registry) {
	t.Helper()
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	reg := cleanup.NewRegistry(slog.Default())
	require.NoError(t, cleanup.Install(ctx, reg))
	t.Cleanup(func() { _ = reg.Sweep(ctx) })
	return ctx, reg
}

// fakeCLI writes an executable stand-in for ap that prints its arguments, HOME and token,
// and exits with the status in FAKE_AP_EXIT.
func fakeCLI(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ap")
	script := "#!/bin/sh\necho \"args=$*\"\necho \"home=$HOME\"\necho \"token=$WSO2AP_AIWORKSPACE_TOKEN\"\necho \"oops\" >&2\nexit ${FAKE_AP_EXIT:-0}\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

// fakeCLIInWorkingDirectory is fakeCLI placed under the test's working directory, so a
// relative path to it only resolves from that directory.
func fakeCLIInWorkingDirectory(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(mustGetwd(t), ".fake-ap-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })
	path := filepath.Join(dir, "ap")
	require.NoError(t, os.Rename(fakeCLI(t), path))
	return path
}

func TestLookup(t *testing.T) {
	tests := []struct {
		key     string
		want    artifactSpec
		wantErr bool
	}{
		{key: "llm-provider", want: artifactSpec{initType: "App-LLM-Provider", resourceDir: "llm-provider",
			group: "llm-provider", artifactKind: "LlmProvider"}},
		{key: "llm-proxy", want: artifactSpec{initType: "LLM-Proxy", resourceDir: "llm-proxy",
			group: "app-llm-proxy", artifactKind: "LlmProxy", projectScoped: true}},
		{key: "mcp-proxy", want: artifactSpec{initType: "MCP-Proxy", resourceDir: "mcp",
			group: "mcp-proxy", artifactKind: "Mcp", projectScoped: true}},
		{key: "mcp", wantErr: true},
		{key: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, err := lookup(tt.key)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestAssertContainsAll(t *testing.T) {
	ok := cliResult{stdout: "Status: success\napplied successfully", exit: 0}
	require.NoError(t, assertContainsAll(ok, successMarker, createdMarker))
	require.NoError(t, assertContainsAll(ok))

	require.ErrorContains(t, assertContainsAll(ok, updatedMarker), "updated successfully")
	require.ErrorContains(t, assertContainsAll(cliResult{stdout: "Status: success", exit: 1}, successMarker), "exit 0")
	require.ErrorContains(t, assertContainsAll(cliResult{}, successMarker), "Status: success")

	split := cliResult{stdout: "Status: success", stderr: "applied successfully"}
	require.NoError(t, assertContainsAll(split, successMarker, createdMarker), "stdout and stderr are searched together")
}

func TestResolveBinary(t *testing.T) {
	binary := fakeCLIInWorkingDirectory(t)

	t.Setenv(envCLIBinary, binary)
	got, err := resolveBinary()
	require.NoError(t, err)
	require.Equal(t, binary, got)

	relative := relativeToWorkingDirectory(t, binary)
	require.False(t, filepath.IsAbs(relative))
	t.Setenv(envCLIBinary, relative)
	got, err = resolveBinary()
	require.NoError(t, err)
	require.True(t, filepath.IsAbs(got), "resolved path %q must be absolute", got)
	require.Equal(t, binary, got)

	t.Setenv(envCLIBinary, filepath.Join(relative, "..", "missing"))
	_, err = resolveBinary()
	require.ErrorContains(t, err, "make -C tests/framework ap-cli")
	require.ErrorContains(t, err, string(filepath.Separator), "the error names the resolved path")

	t.Setenv(envCLIBinary, filepath.Join(t.TempDir(), "missing"))
	_, err = resolveBinary()
	require.ErrorContains(t, err, "make -C tests/framework ap-cli")

	t.Setenv(envCLIBinary, t.TempDir())
	_, err = resolveBinary()
	require.ErrorContains(t, err, "is a directory")
}

func TestCLIRunIsolatesHomeAndReportsExitStatus(t *testing.T) {
	ctx, reg := testContext(t)
	t.Setenv(envCLIBinary, relativeToWorkingDirectory(t, fakeCLIInWorkingDirectory(t)))
	bin, err := resolveBinary()
	require.NoError(t, err)
	c, err := newCLI(ctx, bin)
	require.NoError(t, err)
	require.NotEqual(t, mustGetwd(t), c.workspace, "the CLI runs from its own scratch workspace")
	require.Len(t, reg.Pending(), 2, "the HOME and workspace are both registered for cleanup")

	res, err := c.run(ctx, []string{"WSO2AP_AIWORKSPACE_TOKEN=tok"}, "ai-workspace", "build", "-f", "demo")
	require.NoError(t, err)
	require.Equal(t, 0, res.exit)
	require.Contains(t, res.stdout, "args=ai-workspace build -f demo")
	require.Contains(t, res.stdout, "home="+c.home)
	require.Contains(t, res.stdout, "token=tok")
	require.Equal(t, "oops\n", res.stderr)

	t.Setenv("FAKE_AP_EXIT", "3")
	res, err = c.run(ctx, nil, "version")
	require.NoError(t, err, "a non-zero exit is a result, not an error")
	require.Equal(t, 3, res.exit)
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	return dir
}

func relativeToWorkingDirectory(t *testing.T, path string) string {
	t.Helper()
	relative, err := filepath.Rel(mustGetwd(t), path)
	require.NoError(t, err)
	return relative
}

func TestCLIRunFailsWhenTheBinaryCannotRun(t *testing.T) {
	ctx, _ := testContext(t)
	c, err := newCLI(ctx, filepath.Join(t.TempDir(), "absent"))
	require.NoError(t, err)

	res, err := c.run(ctx, nil, "version")
	require.Error(t, err)
	require.Equal(t, -1, res.exit)
}

func TestScratchDirectoriesAreRemovedByCleanup(t *testing.T) {
	ctx, reg := testContext(t)
	c, err := newCLI(ctx, fakeCLI(t))
	require.NoError(t, err)
	require.DirExists(t, c.home)
	require.DirExists(t, c.workspace)

	require.NoError(t, reg.Sweep(ctx))
	require.NoDirExists(t, c.home)
	require.NoDirExists(t, c.workspace)
	require.Empty(t, reg.Pending())
}

// fixtureSuite builds a feature root holding one fixture set and a session whose workspace
// already contains the scaffolded project directory, as `ap project init` would leave it.
func fixtureSuite(t *testing.T) (context.Context, *Steps, string) {
	t.Helper()
	ctx, _ := testContext(t)
	root := t.TempDir()
	dir := filepath.Join(root, fixtureRoot, "llm-proxy")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "create"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "edit"), 0o755))
	write := func(rel, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, rel), []byte(content), 0o644))
	}
	write("definition.yaml", "openapi: 3.0.0\n# ${CTX:artifactId} stays literal in definitions\n")
	write("create/metadata.yaml", "metadata:\n  name: ${CTX:artifactId}\n")
	write("create/runtime.yaml", "spec:\n  context: ${CTX:artifactContext}\n  provider:\n    id: ${CTX:providerId}\n")
	write("edit/metadata.yaml", "spec:\n  associatedGateways:\n    - id: ${CTX:gatewayHandleA}\n")
	write("edit/runtime.yaml", "spec:\n  context: ${CTX:artifactContext}\n  changed: true\n")

	for key, value := range map[string]string{
		keyArtifactID: "demo-proxy", "artifactContext": "/demo-proxy-1",
		"providerId": "demo-provider", "gatewayHandleA": "gw-a",
	} {
		require.NoError(t, tcontext.Set(ctx, key, value))
	}

	s := &Steps{featureRoot: root, registered: map[string]bool{}}
	t.Setenv(envCLIBinary, fakeCLI(t))
	c, err := s.session(ctx)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(c.workspace, "demo-proxy"), 0o755))
	return ctx, s, filepath.Join(c.workspace, "demo-proxy")
}

func TestStageCreateExpandsMetadataAndRuntimeButCopiesDefinitionVerbatim(t *testing.T) {
	ctx, s, project := fixtureSuite(t)
	spec, err := lookup("llm-proxy")
	require.NoError(t, err)

	require.NoError(t, s.stage(ctx, spec, "demo-proxy", "create"))

	metadata, err := os.ReadFile(filepath.Join(project, "metadata.yaml"))
	require.NoError(t, err)
	require.Equal(t, "metadata:\n  name: demo-proxy\n", string(metadata))

	runtime, err := os.ReadFile(filepath.Join(project, "runtime.yaml"))
	require.NoError(t, err)
	require.Equal(t, "spec:\n  context: /demo-proxy-1\n  provider:\n    id: demo-provider\n", string(runtime))

	definition, err := os.ReadFile(filepath.Join(project, "definition.yaml"))
	require.NoError(t, err)
	require.Equal(t, "openapi: 3.0.0\n# ${CTX:artifactId} stays literal in definitions\n", string(definition))
}

func TestStageEditOverlaysMetadataAndRuntimeAndKeepsDefinition(t *testing.T) {
	ctx, s, project := fixtureSuite(t)
	spec, err := lookup("llm-proxy")
	require.NoError(t, err)
	require.NoError(t, s.stage(ctx, spec, "demo-proxy", "create"))

	require.NoError(t, os.WriteFile(filepath.Join(project, "definition.yaml"), []byte("kept\n"), 0o644))
	require.NoError(t, s.stage(ctx, spec, "demo-proxy", "edit"))

	metadata, err := os.ReadFile(filepath.Join(project, "metadata.yaml"))
	require.NoError(t, err)
	require.Equal(t, "spec:\n  associatedGateways:\n    - id: gw-a\n", string(metadata))

	runtime, err := os.ReadFile(filepath.Join(project, "runtime.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(runtime), "changed: true")

	definition, err := os.ReadFile(filepath.Join(project, "definition.yaml"))
	require.NoError(t, err)
	require.Equal(t, "kept\n", string(definition), "edit does not restage definition.yaml")
}

func TestStageFailsOnAnUnresolvedPlaceholderOrMissingFixture(t *testing.T) {
	ctx, s, _ := fixtureSuite(t)
	spec, err := lookup("llm-proxy")
	require.NoError(t, err)

	tcontext.Remove(ctx, "providerId")
	require.ErrorContains(t, s.stage(ctx, spec, "demo-proxy", "create"), "providerId")

	missing, err := lookup("mcp-proxy")
	require.NoError(t, err)
	require.Error(t, s.stage(ctx, missing, "demo-proxy", "create"))
}

func TestReadFixtureRefusesPathsOutsideTheFeatureRoot(t *testing.T) {
	s := &Steps{featureRoot: t.TempDir()}
	_, err := s.readFixture("../outside.yaml")
	require.Error(t, err)
	_, err = s.readFixture("/etc/passwd")
	require.Error(t, err)
}

func TestExecuteClearsTheStaleResultBeforeEachCall(t *testing.T) {
	ctx, _ := testContext(t)
	t.Setenv(envCLIBinary, fakeCLI(t))
	s := &Steps{registered: map[string]bool{}}

	_, err := s.execute(ctx, nil, "version")
	require.NoError(t, err)
	require.Contains(t, s.last.stdout, "args=version")

	t.Setenv(envCLIBinary, filepath.Join(t.TempDir(), "gone"))
	s.cli = nil
	_, err = s.execute(ctx, nil, "version")
	require.Error(t, err)
	require.Equal(t, cliResult{}, s.last, "a failed invocation must not leave the previous result to assert on")
}

func TestReportedAppliedChecksTheLastResult(t *testing.T) {
	s := &Steps{last: cliResult{stdout: "Status: success\nupdated successfully"}}
	require.NoError(t, s.reportedApplied(context.Background(), "llm-provider", "updated"))
	require.Error(t, s.reportedApplied(context.Background(), "llm-provider", "created"))
}

func TestAuthEnvCarriesTheBearerToken(t *testing.T) {
	s := &Steps{token: "jwt"}
	require.Equal(t, []string{"WSO2AP_AIWORKSPACE_TOKEN=jwt"}, s.authEnv())
}

func TestConfigureWorkspaceRejectsAnUnknownActor(t *testing.T) {
	s := &Steps{}
	require.ErrorContains(t, s.configureWorkspace(context.Background(), "consumer"), "only \"admin\"")
}

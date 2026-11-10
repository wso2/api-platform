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
	"github.com/wso2/api-platform/tests/framework/suites/it/steps/apcli"
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
	ok := apcli.Result{Stdout: "Status: success\napplied successfully", Exit: 0}
	require.NoError(t, assertContainsAll(ok, successMarker, createdMarker))
	require.NoError(t, assertContainsAll(ok))

	require.ErrorContains(t, assertContainsAll(ok, updatedMarker), "updated successfully")
	require.ErrorContains(t, assertContainsAll(apcli.Result{Stdout: "Status: success", Exit: 1}, successMarker), "exit 0")
	require.ErrorContains(t, assertContainsAll(apcli.Result{}, successMarker), "Status: success")

	split := apcli.Result{Stdout: "Status: success", Stderr: "applied successfully"}
	require.NoError(t, assertContainsAll(split, successMarker, createdMarker), "stdout and stderr are searched together")
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
	t.Setenv(apcli.EnvBinary, fakeCLI(t))
	c, err := s.session(ctx)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(c.Workspace(), "demo-proxy"), 0o755))
	return ctx, s, filepath.Join(c.Workspace(), "demo-proxy")
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
	t.Setenv(apcli.EnvBinary, fakeCLI(t))
	s := &Steps{registered: map[string]bool{}}

	_, err := s.execute(ctx, nil, "version")
	require.NoError(t, err)
	require.Contains(t, s.last.Stdout, "args=version")

	t.Setenv(apcli.EnvBinary, filepath.Join(t.TempDir(), "gone"))
	s.cli = nil
	_, err = s.execute(ctx, nil, "version")
	require.Error(t, err)
	require.Equal(t, apcli.Result{}, s.last, "a failed invocation must not leave the previous result to assert on")
}

func TestReportedAppliedChecksTheLastResult(t *testing.T) {
	s := &Steps{last: apcli.Result{Stdout: "Status: success\nupdated successfully"}}
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

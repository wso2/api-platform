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

package platformgateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	frameworkbuilder "github.com/wso2/api-platform/tests/framework/core/builder"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
)

type policyRecordingRunner struct {
	commands []frameworkbuilder.Command
	failAt   int
}

func (r *policyRecordingRunner) Run(_ context.Context, command frameworkbuilder.Command) error {
	r.commands = append(r.commands, command)
	if r.failAt > 0 && len(r.commands) == r.failAt {
		return fmt.Errorf("synthetic command failure")
	}
	return nil
}

func unitRepoRoot(t *testing.T) string {
	t.Helper()
	root, ok := shared.RepoRootFromCallerFile()
	require.True(t, ok, "repository root not found")
	return root
}

func gatewayControllersPolicySource(t *testing.T) string {
	t.Helper()
	root := unitRepoRoot(t)
	source := filepath.Join(root, "..", "gateway-controllers", "policies")
	info, err := os.Stat(source)
	if os.IsNotExist(err) {
		t.Skipf("gateway-controllers policy checkout is not present at %s", source)
	}
	require.NoError(t, err, "gateway-controllers policy checkout is required at %s", source)
	require.True(t, info.IsDir(), "gateway-controllers policy checkout is not a directory: %s", source)
	return filepath.Join("..", "gateway-controllers", "policies")
}

// composeFile is the gateway stack the suite actually runs.
func composeFile(t *testing.T) map[string]any {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	path := filepath.Join(filepath.Dir(thisFile), "docker-compose.yaml")
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "cannot read %s", path)

	rendered := strings.ReplaceAll(string(raw), "${INSTANCE:-}", "")
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &doc), "%s is not valid YAML", path)
	return doc
}

// composeServiceEnv returns a service's `environment:` list as a map.
//
// Only the KEY=VALUE list form is understood, which is the form this file uses. A mapping-form
// environment block would silently return nothing, so the caller asserts non-emptiness rather
// than trusting a zero result.
func composeServiceEnv(t *testing.T, service string) map[string]string {
	t.Helper()

	doc := composeFile(t)
	services, ok := doc["services"].(map[string]any)
	require.True(t, ok, "compose file has no services map")

	svc, ok := services[service].(map[string]any)
	require.True(t, ok, "compose file has no %q service", service)

	env := map[string]string{}
	entries, ok := svc["environment"].([]any)
	if !ok {
		return env
	}
	for _, e := range entries {
		s, ok := e.(string)
		if !ok {
			continue
		}
		if k, v, found := strings.Cut(s, "="); found {
			env[k] = v
		}
	}
	return env
}

// TestComposeCarriesTheControllerITVars verifies parity between the service definition and
// the Compose stack used by integration tests.
func TestComposeCarriesTheControllerITVars(t *testing.T) {
	composeEnv := composeServiceEnv(t, "gateway-controller")
	require.NotEmpty(t, composeEnv,
		"the gateway-controller service has no parseable environment: list — if it was "+
			"converted to the mapping form, composeServiceEnv needs updating, otherwise this "+
			"test silently guards nothing")

	definitionEnv := GatewayController().Env

	for key, want := range definitionEnv {
		if !strings.HasPrefix(key, "IT_") {
			continue
		}
		got, present := composeEnv[key]
		require.True(t, present,
			"%s is missing from docker-compose.yaml", key)
		require.Equal(t, want, got,
			"%s disagrees between GatewayController() (%q) and the compose file (%q)",
			key, want, got)
	}
}

func TestPlatformGatewayDefinition(t *testing.T) {
	definition := PlatformGateway()
	require.Equal(t, "platform-gateway", definition.Name)
	require.True(t, definition.IsCompose())
	require.Equal(t, "gateway-runtime", definition.Compose.PrimaryService)
	require.ElementsMatch(t, []string{"gateway-controller", "gateway-runtime"}, definition.Compose.Services)
	for _, endpoint := range []string{"http", "https", "rest", "admin"} {
		_, ok := definition.Endpoint(endpoint)
		require.True(t, ok, endpoint)
	}
}

func TestPlatformGatewayVersionUpdatesBothServices(t *testing.T) {
	t.Setenv(shared.EnvCoverageMode, "false")
	definition := PlatformGateway().WithImageVersion("legacy")

	require.Equal(t, "ghcr.io/wso2/api-platform/gateway-controller:legacy",
		definition.Compose.Env["PG_CONTROLLER_IMAGE"])
	require.Equal(t, "ghcr.io/wso2/api-platform/gateway-runtime:legacy",
		definition.Compose.Env["PG_RUNTIME_IMAGE"])
}

func TestPlatformGatewayReleasedConfigProfilesUseOfficialBases(t *testing.T) {
	definition := PlatformGateway()
	for version, want := range map[string]string{
		"1.1.0": "tests/framework/core/catalog/platformgateway/resources/1.1.0/config.toml",
		"1.2.0": "tests/framework/core/catalog/platformgateway/resources/1.2.0/config.toml",
	} {
		selected, err := definition.WithReleaseVersion(version)
		require.NoError(t, err)
		require.Equal(t, want, selected.Config.BaseConfigPath)
		require.Equal(t,
			"tests/framework/core/catalog/platformgateway/resources/"+version+"/gateway-controller-storage.toml",
			selected.Config.SharedOverlayPath)
	}

	_, err := definition.WithReleaseVersion("1.3.0")
	require.ErrorContains(t, err, `no profile for version "1.3.0"`)

	runtimeDefinition := GatewayRuntime()
	runtimeConfig, err := runtimeDefinition.WithReleaseVersion("1.1.0")
	require.NoError(t, err)
	require.Empty(t, runtimeConfig.Config.SharedOverlayPath)

	legacy, err := definition.WithReleaseVersion("1.1.0")
	require.NoError(t, err)
	require.Equal(t, "/api/admin/v0.9/health", legacy.Health.Path)
	current, err := definition.WithReleaseVersion("1.2.0")
	require.NoError(t, err)
	require.Equal(t, "/api/admin/v1/health", current.Health.Path)
}

func TestPlatformGatewayReleasedConfigProfilesAssembleVersionSpecificLayers(t *testing.T) {
	root := unitRepoRoot(t)
	for version, wantLiteralEnv := range map[string]bool{"1.1.0": false, "1.2.0": true} {
		t.Run(version, func(t *testing.T) {
			definition, err := PlatformGateway().WithConfigVersion(version)
			require.NoError(t, err)
			content, err := components.Assemble(definition.Config, root, "", components.Vars{components.VarBlock: "gateway"})
			require.NoError(t, err)

			config := string(content)
			require.Contains(t, config, "username = 'consumer'")
			require.Contains(t, config, "enabled = true")
			require.Equal(t, wantLiteralEnv, strings.Contains(config, "{{ env"))
		})
	}
}

// TestComposeRuntimeBoundsItsShutdownDrain verifies the configured graceful shutdown period.
//
// Asserted on the compose file rather than trusted to a comment because the failure is silent —
// tests still pass, teardown just costs a forced 10s per container and no process exits cleanly.
func TestComposeRuntimeBoundsItsShutdownDrain(t *testing.T) {
	env := composeServiceEnv(t, "gateway-runtime")

	drain, ok := env["ROUTER_DRAIN_TIME_SECONDS"]
	require.True(t, ok,
		"gateway-runtime does not set ROUTER_DRAIN_TIME_SECONDS. The entrypoint defaults it to "+
			"15s, which does not fit docker's 10s stop timeout, so every teardown is a SIGKILL")
	require.Equal(t, "2", drain,
		"ROUTER_DRAIN_TIME_SECONDS is %q; it must stay well inside docker's stop timeout", drain)

	doc := composeFile(t)
	services := doc["services"].(map[string]any)
	runtime := services["gateway-runtime"].(map[string]any)
	require.Equal(t, "30s", runtime["stop_grace_period"],
		"gateway-runtime must raise stop_grace_period above the drain, so a later increase to "+
			"ROUTER_DRAIN_TIME_SECONDS cannot silently reintroduce the mid-drain SIGKILL")
}

func TestPlatformGatewayCoverageEnvironmentFollowsRunMode(t *testing.T) {
	t.Setenv(shared.EnvCoverageMode, "false")
	require.NotContains(t, PlatformGateway().Compose.Env, "GOCOVERDIR")

	t.Setenv(shared.EnvCoverageMode, "true")
	require.Equal(t, "/coverage", PlatformGateway().Compose.Env["GOCOVERDIR"])
}

func TestBuildSourceWithPoliciesStagesCompletePolicyTree(t *testing.T) {
	root := unitRepoRoot(t)
	source := gatewayControllersPolicySource(t)
	runner := &policyRecordingRunner{}

	images, err := BuildSourceWithPolicies(context.Background(), root, "1.2.0-SNAPSHOT", source, runner, false)
	require.NoError(t, err)
	require.Contains(t, images.Controller, "local/apip-gateway-controller:framework-1.2.0-snapshot-policies-")
	require.Contains(t, images.Runtime, "local/apip-gateway-runtime:framework-1.2.0-snapshot-policies-")
	require.Len(t, runner.commands, 4)
	require.Contains(t, strings.Join(runner.commands[0].Args, " "), "--build-context")
	require.Contains(t, strings.Join(runner.commands[0].Args, " "), "dev-policies=")
	require.Contains(t, strings.Join(runner.commands[0].Args, " "), images.Runtime)
	require.Contains(t, strings.Join(runner.commands[3].Args, " "), images.Controller)
	require.Equal(t, root, runner.commands[0].Directory)
}

func TestStagePolicyWorkspaceGeneratesDeterministicManifest(t *testing.T) {
	root := unitRepoRoot(t)
	source, err := os.MkdirTemp(root, ".framework-policy-source-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(source)) })
	for _, policy := range []struct {
		name    string
		version string
	}{{"z-policy", "v1.0.0"}, {"a-policy", "v2.0.0"}} {
		dir := filepath.Join(source, policy.name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "policy-definition.yaml"), []byte(
			"name: "+policy.name+"\nversion: "+policy.version+"\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "policy.go"), []byte("package policy\n"), 0o644))
	}

	relative, err := filepath.Rel(root, source)
	require.NoError(t, err)
	workspace, err := stagePolicyWorkspace(root, relative)
	require.NoError(t, err)
	data, err := os.ReadFile(workspace.BuildFile)
	require.NoError(t, err)
	var manifest policyBuildFile
	require.NoError(t, yaml.Unmarshal(data, &manifest))
	require.Equal(t, "v1", manifest.Version)
	require.Equal(t, []policyBuildEntry{
		{Name: "a-policy", FilePath: "policies/a-policy"},
		{Name: "z-policy", FilePath: "policies/z-policy"},
	}, manifest.Policies)
	sourceData, err := os.ReadFile(filepath.Join(workspace.Target, "build.yaml"))
	require.NoError(t, err)
	var sourceManifest policyBuildFile
	require.NoError(t, yaml.Unmarshal(sourceData, &sourceManifest))
	require.Equal(t, "dev-policies/a-policy", sourceManifest.Policies[0].FilePath)
	require.NotEmpty(t, workspace.Digest)

	workspace.close()
	_, err = os.Stat(workspace.Root)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestBuildVersionedWithPoliciesUsesGatewayBuilderAndDerivedImages(t *testing.T) {
	root := unitRepoRoot(t)
	source := gatewayControllersPolicySource(t)
	runner := &policyRecordingRunner{}

	images, err := BuildVersionedWithPolicies(context.Background(), root, "1.2.0-SNAPSHOT", source,
		"ghcr.io/wso2/api-platform/gateway-controller:1.2.0-SNAPSHOT",
		"ghcr.io/wso2/api-platform/gateway-runtime:1.2.0-SNAPSHOT", runner)
	require.NoError(t, err)
	require.Contains(t, images.Controller, "local/apip-gateway-controller:framework-1.2.0-snapshot-policies-")
	require.Contains(t, images.Runtime, "local/apip-gateway-runtime:framework-1.2.0-snapshot-policies-")
	require.Len(t, runner.commands, 3)
	require.Equal(t, "docker", runner.commands[0].Args[0])
	require.Contains(t, strings.Join(runner.commands[0].Args, " "), "gateway-builder:1.2.0-SNAPSHOT")
	require.Contains(t, strings.Join(runner.commands[0].Args, " "), "-gateway-controller-base-image")
	require.Contains(t, strings.Join(runner.commands[1].Args, " "), "gateway-runtime/Dockerfile")
	require.Contains(t, strings.Join(runner.commands[2].Args, " "), "gateway-controller/Dockerfile")
}

func TestPolicyWorkspaceRejectsNonPolicyEntries(t *testing.T) {
	root := unitRepoRoot(t)
	source, err := os.MkdirTemp(root, ".framework-policy-source-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(source)) })
	require.NoError(t, os.MkdirAll(source, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(source, "README.md"), []byte("not a policy"), 0o644))

	relative, err := filepath.Rel(root, source)
	require.NoError(t, err)
	_, err = BuildSourceWithPolicies(context.Background(), root, "1.2.0-SNAPSHOT", relative, &policyRecordingRunner{}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a directory")
}

func TestPolicyWorkspaceRejectsSourceOutsideApprovedRoots(t *testing.T) {
	root := unitRepoRoot(t)
	outside := t.TempDir()
	source, err := filepath.Rel(root, outside)
	require.NoError(t, err)

	_, err = stagePolicyWorkspace(root, source)
	require.ErrorContains(t, err, "must resolve within repository root or ../gateway-controllers/policies")
}

func TestPolicyWorkspaceRejectsDuplicatePolicyNames(t *testing.T) {
	root := unitRepoRoot(t)
	source, err := os.MkdirTemp(root, ".framework-policy-duplicate-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(source)) })
	for _, name := range []string{"one", "two"} {
		dir := filepath.Join(source, name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "policy-definition.yaml"), []byte(
			"name: same-policy\nversion: v1.0.0\n"), 0o644))
	}

	relative, err := filepath.Rel(root, source)
	require.NoError(t, err)
	_, err = BuildSourceWithPolicies(context.Background(), root, "1.2.0-SNAPSHOT", relative, &policyRecordingRunner{}, false)
	require.ErrorContains(t, err, "duplicate policy name")
}

func TestPolicyBuildValidatesInputsBeforeStaging(t *testing.T) {
	root := unitRepoRoot(t)
	runner := &policyRecordingRunner{}
	_, err := BuildSourceWithPolicies(context.Background(), root, "", "missing", runner, false)
	require.ErrorContains(t, err, "gateway version is required")
	_, err = BuildSourceWithPolicies(context.Background(), root, "1.2.0-SNAPSHOT", "missing", nil, false)
	require.ErrorContains(t, err, "build runner is required")
	_, err = BuildVersionedWithPolicies(context.Background(), root, "1.2.0-SNAPSHOT", "missing", "controller", "runtime", runner)
	require.ErrorContains(t, err, "resolving policy source")
}

func TestVersionedPolicyBuildDoesNotReturnImagesAfterCommandFailure(t *testing.T) {
	root := unitRepoRoot(t)
	runner := &policyRecordingRunner{failAt: 2}

	images, err := BuildVersionedWithPolicies(context.Background(), root, "1.2.0-SNAPSHOT",
		gatewayControllersPolicySource(t),
		"ghcr.io/wso2/api-platform/gateway-controller:1.2.0-SNAPSHOT",
		"ghcr.io/wso2/api-platform/gateway-runtime:1.2.0-SNAPSHOT", runner)
	require.ErrorContains(t, err, "versioned policy build command 2")
	require.Empty(t, images.Controller)
	require.Empty(t, images.Runtime)
}

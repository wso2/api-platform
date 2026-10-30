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

package apcli

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cucumber/godog"
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

// fakeCLI writes an executable stand-in for ap. It prints its arguments, HOME, working
// directory, gateway username and FAKE_AP_OUT, appends its arguments to $HOME/calls.log, writes
// "oops" to stderr and exits with the status in FAKE_AP_EXIT.
func fakeCLI(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ap")
	script := "#!/bin/sh\n" +
		"echo \"args=$*\"\necho \"home=$HOME\"\necho \"pwd=$(pwd)\"\necho \"user=$WSO2AP_GW_USERNAME\"\n" +
		"echo \"$*\" >> \"$HOME/calls.log\"\n[ -n \"$FAKE_AP_OUT\" ] && printf '%s\\n' \"$FAKE_AP_OUT\"\n" +
		"echo \"oops\" >&2\nexit ${FAKE_AP_EXIT:-0}\n"
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

// calls returns the ap invocations a session's fake CLI recorded, one per line.
func calls(t *testing.T, s *Steps) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.session.Home(), "calls.log"))
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func TestResolveBinary(t *testing.T) {
	binary := fakeCLIInWorkingDirectory(t)

	t.Setenv(EnvBinary, binary)
	got, err := ResolveBinary()
	require.NoError(t, err)
	require.Equal(t, binary, got)

	relative := relativeToWorkingDirectory(t, binary)
	require.False(t, filepath.IsAbs(relative))
	t.Setenv(EnvBinary, relative)
	got, err = ResolveBinary()
	require.NoError(t, err)
	require.True(t, filepath.IsAbs(got), "resolved path %q must be absolute", got)
	require.Equal(t, binary, got)

	t.Setenv(EnvBinary, filepath.Join(relative, "..", "missing"))
	_, err = ResolveBinary()
	require.ErrorContains(t, err, "make -C tests/framework ap-cli")
	require.ErrorContains(t, err, string(filepath.Separator), "the error names the resolved path")

	t.Setenv(EnvBinary, filepath.Join(t.TempDir(), "missing"))
	_, err = ResolveBinary()
	require.ErrorContains(t, err, "make -C tests/framework ap-cli")

	t.Setenv(EnvBinary, t.TempDir())
	_, err = ResolveBinary()
	require.ErrorContains(t, err, "is a directory")
}

func TestSessionRunIsolatesHomeAndReportsExitStatus(t *testing.T) {
	ctx, reg := testContext(t)
	t.Setenv(EnvBinary, relativeToWorkingDirectory(t, fakeCLIInWorkingDirectory(t)))
	bin, err := ResolveBinary()
	require.NoError(t, err)
	s, err := NewSession(ctx, bin)
	require.NoError(t, err)
	require.NotEqual(t, mustGetwd(t), s.Workspace(), "the CLI runs from its own scratch workspace")
	require.Len(t, reg.Pending(), 2, "the HOME and workspace are both registered for cleanup")

	res, err := s.Run(ctx, []string{"WSO2AP_GW_USERNAME=admin"}, "gateway", "list")
	require.NoError(t, err)
	require.Equal(t, 0, res.Exit)
	require.Contains(t, res.Stdout, "args=gateway list")
	require.Contains(t, res.Stdout, "home="+s.Home())
	require.Contains(t, res.Stdout, "user=admin")
	require.Equal(t, "oops\n", res.Stderr)

	t.Setenv("FAKE_AP_EXIT", "3")
	res, err = s.Run(ctx, nil, "version")
	require.NoError(t, err, "a non-zero exit is a result, not an error")
	require.Equal(t, 3, res.Exit)
}

func TestSessionRunFailsWhenTheBinaryCannotRun(t *testing.T) {
	ctx, _ := testContext(t)
	s, err := NewSession(ctx, filepath.Join(t.TempDir(), "absent"))
	require.NoError(t, err)

	res, err := s.Run(ctx, nil, "version")
	require.Error(t, err)
	require.Equal(t, -1, res.Exit)
}

func TestScratchDirectoriesAreRemovedByCleanup(t *testing.T) {
	ctx, reg := testContext(t)
	s, err := NewSession(ctx, fakeCLI(t))
	require.NoError(t, err)
	require.DirExists(t, s.Home())
	require.DirExists(t, s.Workspace())

	require.NoError(t, reg.Sweep(ctx))
	require.NoDirExists(t, s.Home())
	require.NoDirExists(t, s.Workspace())
	require.Empty(t, reg.Pending())
}

func TestNewSessionFailsWithoutACleanupRegistry(t *testing.T) {
	_, err := NewSession(context.Background(), fakeCLI(t))
	require.Error(t, err)
}

func TestConcurrentSessionsAreIsolated(t *testing.T) {
	ctx, reg := testContext(t)
	bin := fakeCLI(t)
	const n = 8
	homes := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s, err := NewSession(ctx, bin)
			if err != nil {
				t.Error(err)
				return
			}
			homes[i] = s.Home()
		}(i)
	}
	wg.Wait()
	seen := map[string]bool{}
	for _, home := range homes {
		require.NotEmpty(t, home)
		require.False(t, seen[home], "sessions must not share a HOME")
		seen[home] = true
	}
	require.Len(t, reg.Pending(), 2*n)
}

func TestResultCombined(t *testing.T) {
	require.Equal(t, "outerr", Result{Stdout: "out", Stderr: "err"}.Combined())
	require.Empty(t, Result{}.Combined())
}

func TestArgumentsExpandValuesAndResolveResources(t *testing.T) {
	ctx, _ := testContext(t)
	root := t.TempDir()
	fixture := filepath.Join(root, "resources", "fixtures", "malformed.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(fixture), 0o755))
	require.NoError(t, os.WriteFile(fixture, []byte("[\n"), 0o644))
	require.NoError(t, tcontext.Set(ctx, "apiName", "orders-api"))
	s := &Steps{featureRoot: root}

	args, err := s.arguments(ctx, "gateway apply -f resources/fixtures/malformed.yaml --id ${CTX:apiName}")
	require.NoError(t, err)
	resolved, err := filepath.EvalSymlinks(fixture)
	require.NoError(t, err)
	gotPath, err := filepath.EvalSymlinks(args[3])
	require.NoError(t, err)
	require.Equal(t, resolved, gotPath)
	require.Equal(t, []string{"gateway", "apply", "-f", args[3], "--id", "orders-api"}, args)

	args, err = s.arguments(ctx, "gateway apply -f missing-file.yaml")
	require.NoError(t, err)
	require.Equal(t, "missing-file.yaml", args[3], "other relative paths resolve against the CLI workspace")

	_, err = s.arguments(ctx, "   ")
	require.Error(t, err)
	_, err = s.arguments(ctx, "gateway rest-api get --id ${CTX:unknown}")
	require.Error(t, err)
	_, err = s.arguments(ctx, "gateway apply -f resources/../../outside.yaml")
	require.Error(t, err)
}

func TestUseGatewayAddsAGatewayByURLAndMakesItCurrent(t *testing.T) {
	ctx, _ := testContext(t)
	t.Setenv(EnvBinary, fakeCLI(t))
	require.NoError(t, tcontext.Set(ctx, "server", "http://localhost:19999"))
	s := &Steps{deployed: map[string]cleanup.Kind{}}

	require.NoError(t, s.useGateway(ctx, `a gateway at "${CTX:server}"`, "unreachable-gw"))
	require.Equal(t, []string{
		"gateway add --display-name unreachable-gw --server http://localhost:19999",
		"gateway use --display-name unreachable-gw",
	}, calls(t, s))
	require.Empty(t, s.env, "a gateway added by URL carries no credentials")
}

func TestUseGatewayRejectsInvalidTargets(t *testing.T) {
	ctx, _ := testContext(t)
	t.Setenv(EnvBinary, fakeCLI(t))
	s := &Steps{deployed: map[string]cleanup.Kind{}}

	require.ErrorContains(t, s.useGateway(ctx, "a mystery gateway", "gw"), "unknown CLI gateway target")
	require.ErrorContains(t, s.useGateway(ctx, `a gateway at ""`, "gw"), "server URL")
	require.ErrorContains(t, s.useGateway(ctx, `a gateway at "http://localhost:1"`, " "), "display name")
	require.ErrorContains(t, s.useGateway(ctx, platformGatewayTarget, "gw"), "not part of this topology")
	require.Nil(t, s.session, "no command runs for an invalid target")
}

func TestUseGatewayFailsWhenTheCLIRejectsTheGateway(t *testing.T) {
	ctx, _ := testContext(t)
	t.Setenv(EnvBinary, fakeCLI(t))
	t.Setenv("FAKE_AP_EXIT", "1")
	s := &Steps{deployed: map[string]cleanup.Kind{}}

	err := s.useGateway(ctx, `a gateway at "http://localhost:1"`, "gw")
	require.ErrorContains(t, err, "gateway add")
	require.ErrorContains(t, err, "exit 1")
}

func TestRunWithArgumentsPublishesTheResult(t *testing.T) {
	ctx, _ := testContext(t)
	t.Setenv(EnvBinary, fakeCLI(t))
	s := &Steps{deployed: map[string]cleanup.Kind{}}

	require.NoError(t, s.runWithArguments(ctx, "gateway list"))
	require.NoError(t, s.commandOutcome(ctx, "succeed"))
	require.NoError(t, s.outputContains(ctx, "output", "contain", "args=gateway list"))
	require.Contains(t, s.last.Stdout, "pwd=")
	require.NoError(t, s.outputContains(ctx, "output", "contain", "pwd="))

	t.Setenv("FAKE_AP_EXIT", "2")
	require.NoError(t, s.runWithArguments(ctx, "gateway current"), "a failing command is a result to assert on")
	require.NoError(t, s.commandOutcome(ctx, "fail"))
	require.ErrorContains(t, s.commandOutcome(ctx, "succeed"), "exited 2")
}

func TestExecuteClearsTheStaleResultBeforeEachCall(t *testing.T) {
	ctx, _ := testContext(t)
	t.Setenv(EnvBinary, fakeCLI(t))
	s := &Steps{deployed: map[string]cleanup.Kind{}}

	_, err := s.execute(ctx, "version")
	require.NoError(t, err)
	require.True(t, s.ran)

	t.Setenv(EnvBinary, filepath.Join(t.TempDir(), "gone"))
	s.session = nil
	_, err = s.execute(ctx, "version")
	require.Error(t, err)
	require.Equal(t, Result{}, s.last, "a failed invocation must not leave the previous result to assert on")
	require.False(t, s.ran)
	require.ErrorContains(t, s.commandOutcome(ctx, "fail"), "no ap command has run")
}

func TestCommandOutcome(t *testing.T) {
	s := &Steps{}
	require.ErrorContains(t, s.commandOutcome(context.Background(), "succeed"), "no ap command has run")

	s.last, s.ran = Result{Exit: 0, Stdout: "ok"}, true
	require.NoError(t, s.commandOutcome(context.Background(), "succeed"))
	require.ErrorContains(t, s.commandOutcome(context.Background(), "fail"), "exited 0")

	s.last = Result{Exit: 1, Stderr: "boom"}
	require.NoError(t, s.commandOutcome(context.Background(), "fail"))
	require.ErrorContains(t, s.commandOutcome(context.Background(), "succeed"), "boom")
}

func TestOutputContains(t *testing.T) {
	ctx, _ := testContext(t)
	require.NoError(t, tcontext.Set(ctx, "apiName", "orders-api"))
	s := &Steps{}
	require.ErrorContains(t, s.outputContains(ctx, "output", "contain", "x"), "no ap command has run")

	s.last, s.ran = Result{Stdout: "deployed orders-api\n", Stderr: "Error: required flag\n"}, true
	require.NoError(t, s.outputContains(ctx, "output", "contain", "${CTX:apiName}"))
	require.NoError(t, s.outputContains(ctx, "output", "contain", "required flag"), "output covers stderr too")
	require.NoError(t, s.outputContains(ctx, "error output", "contain", "required flag"))
	require.Error(t, s.outputContains(ctx, "error output", "contain", "orders-api"), "error output is stderr alone")
	require.NoError(t, s.outputContains(ctx, "output", "not contain", "petstore"))
	require.ErrorContains(t, s.outputContains(ctx, "output", "not contain", "orders-api"), "not to contain")
	require.Error(t, s.outputContains(ctx, "output", "contain", "${CTX:unknown}"))
}

func TestForgetDeletedDeregistersOnlyResourcesTheScenarioApplied(t *testing.T) {
	ctx, reg := testContext(t)
	reg.RegisterDeleter(cleanup.KindAPI, func(context.Context, cleanup.Resource) error { return nil })
	require.NoError(t, reg.Register(cleanup.Resource{Kind: cleanup.KindAPI, ID: "orders-api", Actor: "admin"}))
	s := &Steps{deployed: map[string]cleanup.Kind{"rest-api/orders-api": cleanup.KindAPI}}

	s.forgetDeleted(ctx, []string{"gateway", "rest-api", "delete", "--id", "other-api"})
	s.forgetDeleted(ctx, []string{"gateway", "mcp", "delete", "--id", "orders-api"})
	s.forgetDeleted(ctx, []string{"gateway", "rest-api", "get", "--id", "orders-api"})
	s.forgetDeleted(ctx, []string{"gateway", "rest-api", "delete"})
	s.forgetDeleted(ctx, []string{"version"})
	require.Len(t, reg.Pending(), 1, "only a delete of an applied resource deregisters it")

	s.forgetDeleted(ctx, []string{"gateway", "rest-api", "delete", "--id", "orders-api"})
	require.Empty(t, reg.Pending())
	require.Empty(t, s.deployed)
}

func TestRunWithArgumentsDeregistersADeletedResourceOnlyOnSuccess(t *testing.T) {
	ctx, reg := testContext(t)
	t.Setenv(EnvBinary, fakeCLI(t))
	reg.RegisterDeleter(cleanup.KindMCPProxy, func(context.Context, cleanup.Resource) error { return nil })
	require.NoError(t, reg.Register(cleanup.Resource{Kind: cleanup.KindMCPProxy, ID: "tools-mcp", Actor: "admin"}))
	s := &Steps{deployed: map[string]cleanup.Kind{"mcp/tools-mcp": cleanup.KindMCPProxy}}

	t.Setenv("FAKE_AP_EXIT", "1")
	require.NoError(t, s.runWithArguments(ctx, "gateway mcp delete --id tools-mcp"))
	require.Len(t, reg.Pending(), 3, "a failed delete keeps the resource registered beside the two scratch directories")

	t.Setenv("FAKE_AP_EXIT", "0")
	require.NoError(t, s.runWithArguments(ctx, "gateway mcp delete --id tools-mcp"))
	for _, pending := range reg.Pending() {
		require.NotEqual(t, cleanup.KindMCPProxy, pending.Kind)
	}
}

func TestFlagValue(t *testing.T) {
	require.Equal(t, "a", flagValue([]string{"x", "--id", "a"}, "--id"))
	require.Empty(t, flagValue([]string{"x", "--id"}, "--id"))
	require.Empty(t, flagValue([]string{"x"}, "--id"))
	require.Empty(t, flagValue(nil, "--id"))
}

func TestResourceName(t *testing.T) {
	name, err := resourceName("apiVersion: v1\nkind: RestApi\nmetadata:\n  name: orders-api\nspec: {}\n")
	require.NoError(t, err)
	require.Equal(t, "orders-api", name)

	_, err = resourceName("kind: RestApi\nspec: {}\n")
	require.ErrorContains(t, err, "no metadata.name")
	_, err = resourceName("metadata: [\n")
	require.Error(t, err)
	_, err = resourceName("metadata:\n  name: ../escape\n")
	require.ErrorContains(t, err, "not a valid resource name")
	_, err = resourceName("")
	require.Error(t, err)
}

func TestDeployFromTemplateRejectsUnsupportedKindsAndVerbs(t *testing.T) {
	ctx, _ := testContext(t)
	s := &Steps{deployed: map[string]cleanup.Kind{}}
	err := s.deployFromTemplate(ctx, "apply", "LLM provider", "resources/templates/llm-provider.yaml", &godog.Table{})
	require.ErrorContains(t, err, "cannot deploy")
	err = s.deployFromTemplate(ctx, "create", "API", "resources/templates/rest-api.yaml", &godog.Table{})
	require.ErrorContains(t, err, `with "apply", not "create"`)
	err = s.deployFromTemplate(ctx, "apply", "subscription", "resources/templates/subscription.yaml", &godog.Table{})
	require.ErrorContains(t, err, `with "create", not "apply"`)
	require.Nil(t, s.session, "nothing runs for an unsupported deployment")
}

// valueTable builds a two-column values table from key/value pairs.
func valueTable(t *testing.T, pairs ...string) *godog.Table {
	t.Helper()
	require.Zero(t, len(pairs)%2)
	type cell struct {
		Value string `json:"value"`
	}
	type row struct {
		Cells []cell `json:"cells"`
	}
	payload := struct {
		Rows []row `json:"rows"`
	}{}
	for i := 0; i < len(pairs); i += 2 {
		payload.Rows = append(payload.Rows, row{Cells: []cell{{Value: pairs[i]}, {Value: pairs[i+1]}}})
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	table := &godog.Table{}
	require.NoError(t, json.Unmarshal(data, table))
	return table
}

func TestDeployFromTemplateCreatesASubscriptionAndRegistersItsID(t *testing.T) {
	ctx, reg := testContext(t)
	t.Setenv(EnvBinary, fakeCLI(t))
	t.Setenv("FAKE_AP_OUT", "Subscription \"orders-sub\" created successfully.\n{\"id\": \"ref-01\", \"apiId\": \"orders\"}")
	reg.RegisterDeleter(cleanup.KindSubscription, func(context.Context, cleanup.Resource) error { return nil })
	s := &Steps{featureRoot: t.TempDir(), deployed: map[string]cleanup.Kind{}}

	require.NoError(t, s.deployFromTemplate(ctx, "create", "subscription", "resources/templates/subscription.yaml",
		valueTable(t, "apiVersion", "gateway.api-platform.wso2.com/v1", "name", "orders-sub",
			"spec.externalRefId", "ref-01", "spec.apiId", "orders")))
	invocations := calls(t, s)
	require.Len(t, invocations, 1)
	require.Contains(t, invocations[0], "gateway subscription create -f ")
	written, err := os.ReadFile(filepath.Join(s.session.Workspace(), "orders-sub.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(written), "kind: Subscription")
	require.Contains(t, string(written), "externalRefId: ref-01")
	require.NoError(t, s.commandOutcome(ctx, "succeed"))

	var registered []string
	for _, pending := range reg.Pending() {
		if pending.Kind == cleanup.KindSubscription {
			registered = append(registered, pending.ID)
		}
	}
	require.Equal(t, []string{"ref-01"}, registered, "the id the gateway returned is registered for cleanup")
	require.Equal(t, cleanup.KindSubscription, s.deployed["subscription/ref-01"])
}

func TestDeployFromTemplatePublishesARejectedCreateWithoutRegisteringIt(t *testing.T) {
	ctx, reg := testContext(t)
	t.Setenv(EnvBinary, fakeCLI(t))
	t.Setenv("FAKE_AP_EXIT", "1")
	s := &Steps{featureRoot: t.TempDir(), deployed: map[string]cleanup.Kind{}}

	require.NoError(t, s.deployFromTemplate(ctx, "create", "subscription", "resources/templates/subscription.yaml",
		valueTable(t, "apiVersion", "gateway.api-platform.wso2.com/v1", "name", "orders-sub")),
		"a rejected create is a result to assert on, not a step failure")
	require.NoError(t, s.commandOutcome(ctx, "fail"))
	for _, pending := range reg.Pending() {
		require.NotEqual(t, cleanup.KindSubscription, pending.Kind)
	}
	require.Empty(t, s.deployed)
}

func TestCreatedID(t *testing.T) {
	id, err := createdID("Subscription \"a\" created successfully.\n{\n  \"id\": \"sub-1\"\n}\n")
	require.NoError(t, err)
	require.Equal(t, "sub-1", id)

	_, err = createdID("created")
	require.ErrorContains(t, err, "no JSON response")
	_, err = createdID("{\"apiId\": \"x\"}")
	require.ErrorContains(t, err, "no id")
	_, err = createdID("{not json")
	require.Error(t, err)
	_, err = createdID("")
	require.Error(t, err)
}

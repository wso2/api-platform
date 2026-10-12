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

// Package aiworkspacecli holds the steps that drive the ap CLI's ai-workspace commands
// against platform-api, the control plane the CLI treats as its AI Workspace server. A
// scenario scaffolds a CLI project per artifact kind (LLM provider, LLM proxy, MCP proxy),
// builds and applies it, and reads the artifact back through the CLI itself.
//
// Each scenario owns an isolated CLI HOME and scratch workspace, created on first use and
// removed by the cleanup registry. Artifacts the CLI creates on platform-api are registered
// for cleanup as soon as the CLI reports the create.
package aiworkspacecli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/actor"
	controlplane "github.com/wso2/api-platform/tests/framework/core/catalog/platformapi"
	"github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
	"github.com/wso2/api-platform/tests/framework/suites/it/steps/platformapi"
)

const (
	// workspaceName is the CLI-side name the platform-api connection is registered under.
	workspaceName = "it-ws"

	// fixtureRoot holds the demo project files, relative to the suite's feature root.
	fixtureRoot = "resources/fixtures/ai-workspace"

	// Scenario values the feature stores before the CLI steps run.
	keyArtifactID = "artifactId"
	keyProjectID  = "projectHandle"

	createdMarker = "applied successfully"
	updatedMarker = "updated successfully"
	successMarker = "Status: success"
)

// artifactSpec describes one AI Workspace artifact kind: how to scaffold it (initType), which
// fixture directory backs it (resourceDir), the CLI get/list command group, the gateway
// artifact kind used for cleanup, and whether it is project-scoped (proxies and MCP proxies
// require --project-id).
type artifactSpec struct {
	initType      string
	resourceDir   string
	group         string
	artifactKind  string
	projectScoped bool
}

// lookup resolves the artifact name used in feature files.
func lookup(key string) (artifactSpec, error) {
	switch key {
	case "llm-provider":
		return artifactSpec{initType: "App-LLM-Provider", resourceDir: "llm-provider",
			group: "llm-provider", artifactKind: "LlmProvider"}, nil
	case "llm-proxy":
		return artifactSpec{initType: "LLM-Proxy", resourceDir: "llm-proxy",
			group: "app-llm-proxy", artifactKind: "LlmProxy", projectScoped: true}, nil
	case "mcp-proxy":
		return artifactSpec{initType: "MCP-Proxy", resourceDir: "mcp",
			group: "mcp-proxy", artifactKind: "Mcp", projectScoped: true}, nil
	default:
		return artifactSpec{}, fmt.Errorf("unknown AI Workspace artifact %q (want llm-provider, llm-proxy or mcp-proxy)", key)
	}
}

// Steps holds one scenario's CLI session. Register builds a fresh instance for every
// scenario, so nothing here is shared between scenarios or runners.
type Steps struct {
	topo        *runtime.Topology
	funnel      *httpx.Funnel
	featureRoot string

	cli        *cli
	token      string
	last       cliResult
	registered map[string]bool
}

// Register binds the AI Workspace CLI steps.
func Register(sc *godog.ScenarioContext, topo *runtime.Topology, funnel *httpx.Funnel, featureRoot string) {
	s := &Steps{topo: topo, funnel: funnel, featureRoot: featureRoot, registered: map[string]bool{}}
	sc.Step(`^the "ap" CLI is available$`, s.cliAvailable)
	sc.Step(`^the CLI is configured for the AI Workspace as "([^"]*)"$`, s.configureWorkspace)
	sc.Step(`^the "([^"]*)" project artifact is initialized$`, s.initArtifact)
	sc.Step(`^I edit the "([^"]*)" artifact$`, s.editArtifact)
	sc.Step(`^I build the "([^"]*)" artifact$`, s.buildArtifact)
	sc.Step(`^I (?:re-)?apply the "([^"]*)" artifact$`, s.applyArtifact)
	sc.Step(`^the CLI reports the "([^"]*)" artifact was (created|updated)$`, s.reportedApplied)
	sc.Step(`^the "([^"]*)" artifact is retrievable from the AI Workspace$`, s.artifactRetrievable)
	sc.Step(`^the "([^"]*)" artifact is listed in the AI Workspace$`, s.artifactListed)
	sc.Step(`^the "([^"]*)" artifact is associated with gateway "([^"]*)"$`, s.artifactAssociatedWithGateway)
}

// session returns the scenario's CLI session, failing with the build hint when the binary is
// missing. A missing binary fails the scenario: a skipped test would leave the build green.
func (s *Steps) session(ctx context.Context) (*cli, error) {
	if s.cli != nil {
		return s.cli, nil
	}
	bin, err := resolveBinary()
	if err != nil {
		return nil, err
	}
	c, err := newCLI(ctx, bin)
	if err != nil {
		return nil, err
	}
	s.cli = c
	return c, nil
}

// execute runs one CLI command, publishing its result for the assertions that follow. The
// previous result is cleared first so a failure to run cannot leave a stale one to assert on.
func (s *Steps) execute(ctx context.Context, extraEnv []string, args ...string) (cliResult, error) {
	s.last = cliResult{}
	c, err := s.session(ctx)
	if err != nil {
		return cliResult{}, err
	}
	res, err := c.run(ctx, extraEnv, args...)
	if err != nil {
		return cliResult{}, err
	}
	s.last = res
	return res, nil
}

func (s *Steps) authEnv() []string {
	return []string{"WSO2AP_AIWORKSPACE_TOKEN=" + s.token}
}

// cliAvailable verifies the binary runs, as the legacy suite's smoke test did.
func (s *Steps) cliAvailable(ctx context.Context) error {
	res, err := s.execute(ctx, nil, "version")
	if err != nil {
		return err
	}
	if res.exit != 0 {
		return fmt.Errorf("ap version failed (exit %d):\n%s", res.exit, res.combined())
	}
	return nil
}

// configureWorkspace registers platform-api as the CLI's AI Workspace using oauth (bearer)
// authentication. No token is stored on disk; every command receives it through
// WSO2AP_AIWORKSPACE_TOKEN.
func (s *Steps) configureWorkspace(ctx context.Context, who string) error {
	if who != "admin" {
		return fmt.Errorf("unsupported AI Workspace actor %q: only \"admin\" is available", who)
	}
	base, err := platformapi.BaseURL(s.topo)
	if err != nil {
		return err
	}
	admin := actor.Administrator()
	token, err := controlplane.ControlPlaneLogin(ctx, base, admin.Username, admin.Password)
	if err != nil {
		return fmt.Errorf("authenticating to the control plane: %w", err)
	}
	s.token = token

	res, err := s.execute(ctx, nil, "ai-workspace", "add",
		"--display-name", workspaceName,
		"--server", base,
		"--auth", "oauth",
		"--no-interactive",
	)
	if err != nil {
		return err
	}
	if res.exit != 0 {
		return fmt.Errorf("ap ai-workspace add failed (exit %d):\n%s", res.exit, res.combined())
	}
	res, err = s.execute(ctx, nil, "ai-workspace", "use", "--display-name", workspaceName)
	if err != nil {
		return err
	}
	if res.exit != 0 {
		return fmt.Errorf("ap ai-workspace use failed (exit %d):\n%s", res.exit, res.combined())
	}
	return nil
}

// initArtifact scaffolds the project with `ap project init` and then replaces the generated
// files with the known-good "create" demo content.
func (s *Steps) initArtifact(ctx context.Context, key string) error {
	spec, id, err := s.artifact(ctx, key)
	if err != nil {
		return err
	}
	res, err := s.execute(ctx, nil, "project", "init",
		"--display-name", id,
		"--type", spec.initType,
		"--no-interactive",
	)
	if err != nil {
		return err
	}
	if res.exit != 0 {
		return fmt.Errorf("ap project init (%s) failed (exit %d):\n%s", spec.initType, res.exit, res.combined())
	}
	return s.stage(ctx, spec, id, "create")
}

// editArtifact overlays the "edit" demo content (changed runtime, and metadata that adds
// spec.associatedGateways) so the re-apply exercises the update path with modified content.
func (s *Steps) editArtifact(ctx context.Context, key string) error {
	spec, id, err := s.artifact(ctx, key)
	if err != nil {
		return err
	}
	return s.stage(ctx, spec, id, "edit")
}

// stage copies the demo files for a variant ("create" or "edit") into the scaffolded project.
// metadata.yaml and runtime.yaml have their ${CTX:...} placeholders expanded; definition.yaml
// is shared by both variants, staged only on create, and copied byte for byte.
func (s *Steps) stage(ctx context.Context, spec artifactSpec, id, variant string) error {
	c, err := s.session(ctx)
	if err != nil {
		return err
	}
	projectDir := filepath.Join(c.workspace, id)
	files := []string{"metadata.yaml", "runtime.yaml"}
	if variant == "create" {
		files = append(files, "definition.yaml")
	}
	for _, name := range files {
		relative := filepath.Join(fixtureRoot, spec.resourceDir, variant, name)
		if name == "definition.yaml" {
			relative = filepath.Join(fixtureRoot, spec.resourceDir, name)
		}
		content, err := s.readFixture(relative)
		if err != nil {
			return fmt.Errorf("stage %s (%s) for %q: %w", name, variant, id, err)
		}
		if name != "definition.yaml" {
			expanded, err := stepscommon.Expand(ctx, string(content))
			if err != nil {
				return fmt.Errorf("stage %s (%s) for %q: %w", name, variant, id, err)
			}
			content = []byte(expanded)
		}
		if err := os.WriteFile(filepath.Join(projectDir, name), content, 0o644); err != nil {
			return fmt.Errorf("stage %s (%s) for %q: %w", name, variant, id, err)
		}
	}
	return nil
}

func (s *Steps) readFixture(relative string) ([]byte, error) {
	path, err := stepscommon.ResourceTemplatePath(s.featureRoot, relative)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (s *Steps) buildArtifact(ctx context.Context, key string) error {
	_, id, err := s.artifact(ctx, key)
	if err != nil {
		return err
	}
	res, err := s.execute(ctx, nil, "ai-workspace", "build", "-f", id)
	if err != nil {
		return err
	}
	if res.exit != 0 {
		return fmt.Errorf("ap ai-workspace build (%s) failed (exit %d):\n%s", key, res.exit, res.combined())
	}
	return nil
}

// applyArtifact runs `ap ai-workspace apply`, passing --project-id for project-scoped kinds.
// The first successful apply creates the artifact, so it is registered for cleanup here,
// before any assertion can fail.
func (s *Steps) applyArtifact(ctx context.Context, key string) error {
	spec, id, err := s.artifact(ctx, key)
	if err != nil {
		return err
	}
	args := []string{"ai-workspace", "apply", "-f", id, "--display-name", workspaceName, "--insecure"}
	if spec.projectScoped {
		project, err := tcontext.ResolveString(ctx, keyProjectID)
		if err != nil {
			return err
		}
		args = append(args, "--project-id", project)
	}
	res, err := s.execute(ctx, s.authEnv(), args...)
	if err != nil {
		return err
	}
	if res.exit != 0 {
		return fmt.Errorf("ap ai-workspace apply (%s) failed (exit %d):\n%s", key, res.exit, res.combined())
	}
	if strings.Contains(res.combined(), createdMarker) && !s.registered[id] {
		if err := platformapi.RegisterCreatedArtifact(ctx, s.topo, s.funnel, spec.artifactKind, id); err != nil {
			return fmt.Errorf("registering cleanup for %s %q: %w", key, id, err)
		}
		s.registered[id] = true
	}
	return nil
}

func (s *Steps) reportedApplied(_ context.Context, _ string, outcome string) error {
	marker := createdMarker
	if outcome == "updated" {
		marker = updatedMarker
	}
	return assertContainsAll(s.last, successMarker, marker)
}

// artifactRetrievable confirms persistence by reading the artifact back through the CLI's own
// get-by-id command.
func (s *Steps) artifactRetrievable(ctx context.Context, key string) error {
	spec, id, err := s.artifact(ctx, key)
	if err != nil {
		return err
	}
	res, err := s.execute(ctx, s.authEnv(),
		"ai-workspace", spec.group, "get", "--id", id, "--display-name", workspaceName, "--insecure")
	if err != nil {
		return err
	}
	if res.exit != 0 {
		return fmt.Errorf("ap ai-workspace %s get (%s) failed (exit %d):\n%s", spec.group, key, res.exit, res.combined())
	}
	if !strings.Contains(res.combined(), id) {
		return fmt.Errorf("get output for %q did not contain id %q:\n%s", key, id, res.combined())
	}
	return nil
}

// artifactListed confirms the applied artifact appears in the CLI's list command. Provider
// lists are organization-scoped; proxy and MCP lists require --project-id.
func (s *Steps) artifactListed(ctx context.Context, key string) error {
	spec, id, err := s.artifact(ctx, key)
	if err != nil {
		return err
	}
	args := []string{"ai-workspace", spec.group, "list", "--display-name", workspaceName, "--insecure"}
	if spec.projectScoped {
		project, err := tcontext.ResolveString(ctx, keyProjectID)
		if err != nil {
			return err
		}
		args = append(args, "--project-id", project)
	}
	res, err := s.execute(ctx, s.authEnv(), args...)
	if err != nil {
		return err
	}
	if res.exit != 0 {
		return fmt.Errorf("ap ai-workspace %s list (%s) failed (exit %d):\n%s", spec.group, key, res.exit, res.combined())
	}
	if !strings.Contains(res.combined(), id) {
		return fmt.Errorf("list output for %q did not contain id %q:\n%s", key, id, res.combined())
	}
	return nil
}

// artifactAssociatedWithGateway confirms the update persisted spec.associatedGateways by
// reading the artifact back and checking the gateway handle appears in the server response.
func (s *Steps) artifactAssociatedWithGateway(ctx context.Context, key, gateway string) error {
	spec, id, err := s.artifact(ctx, key)
	if err != nil {
		return err
	}
	gatewayID, err := stepscommon.Expand(ctx, gateway)
	if err != nil {
		return err
	}
	res, err := s.execute(ctx, s.authEnv(),
		"ai-workspace", spec.group, "get", "--id", id, "--display-name", workspaceName, "--insecure")
	if err != nil {
		return err
	}
	if res.exit != 0 {
		return fmt.Errorf("ap ai-workspace %s get (%s) failed (exit %d):\n%s", spec.group, key, res.exit, res.combined())
	}
	if !strings.Contains(res.combined(), gatewayID) {
		return fmt.Errorf("get output for %q did not show associated gateway %q:\n%s", key, gatewayID, res.combined())
	}
	return nil
}

// artifact resolves the artifact kind and the scenario's artifact id.
func (s *Steps) artifact(ctx context.Context, key string) (artifactSpec, string, error) {
	spec, err := lookup(key)
	if err != nil {
		return artifactSpec{}, "", err
	}
	id, err := tcontext.ResolveString(ctx, keyArtifactID)
	if err != nil {
		return artifactSpec{}, "", err
	}
	return spec, id, nil
}

// assertContainsAll requires a zero exit and every substring in the combined output.
func assertContainsAll(res cliResult, substrs ...string) error {
	out := res.combined()
	if res.exit != 0 {
		return fmt.Errorf("expected exit 0, got %d:\n%s", res.exit, out)
	}
	for _, want := range substrs {
		if !strings.Contains(out, want) {
			return fmt.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	return nil
}

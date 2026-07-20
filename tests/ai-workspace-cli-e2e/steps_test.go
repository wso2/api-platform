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

package awcli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

// artifactSpec describes one AI Workspace artifact kind end-to-end: how to
// scaffold it (initType), which resource files back it (resourceDir), the
// directory / resource id it maps to (dir), the CLI get/list sub-command group
// (group), and whether it is project-scoped (proxies/MCP require --project-id).
type artifactSpec struct {
	initType      string
	dir           string
	resourceDir   string
	group         string
	projectScoped bool
}

// artifacts is keyed by the friendly name used in the feature file. dir is both
// the `ap project init` display name and the resource id (metadata.name), so it
// matches the name inside the staged demo files.
var artifacts = map[string]artifactSpec{
	"llm-provider": {initType: "App-LLM-Provider", dir: "claude-provider", resourceDir: "llm-provider", group: "llm-provider", projectScoped: false},
	"llm-proxy":    {initType: "LLM-Proxy", dir: "claude-proxy", resourceDir: "llm-proxy", group: "app-llm-proxy", projectScoped: true},
	"mcp-proxy":    {initType: "MCP-Proxy", dir: "mcp-proxy", resourceDir: "mcp", group: "mcp-proxy", projectScoped: true},
}

// world holds per-scenario state: the last CLI invocation result.
type world struct {
	last cliResult
}

func initializeScenario(ctx *godog.ScenarioContext) {
	w := &world{}

	ctx.Step(`^the platform-api AI Workspace backend is running$`, w.backendRunning)
	ctx.Step(`^I am authenticated to the AI Workspace$`, w.authenticated)
	ctx.Step(`^the "([^"]*)" project artifact is initialized$`, w.initArtifact)
	ctx.Step(`^I edit the "([^"]*)" artifact$`, w.editArtifact)
	ctx.Step(`^I build the "([^"]*)" artifact$`, w.buildArtifact)
	ctx.Step(`^I apply the "([^"]*)" artifact$`, w.applyArtifact)
	ctx.Step(`^I re-apply the "([^"]*)" artifact$`, w.applyArtifact)
	ctx.Step(`^the CLI reports the "([^"]*)" artifact was created$`, w.reportedCreated)
	ctx.Step(`^the CLI reports the "([^"]*)" artifact was updated$`, w.reportedUpdated)
	ctx.Step(`^the "([^"]*)" artifact is retrievable from the AI Workspace$`, w.artifactRetrievable)
	ctx.Step(`^the "([^"]*)" artifact is listed in the AI Workspace$`, w.artifactListed)
	ctx.Step(`^the "([^"]*)" artifact is associated with gateway "([^"]*)"$`, w.artifactAssociatedWithGateway)
}

func (w *world) backendRunning() error {
	if suite.token == "" || suite.projectID == "" {
		return fmt.Errorf("platform-api AI Workspace backend was not bootstrapped (token/project missing)")
	}
	return nil
}

func (w *world) authenticated() error {
	if suite.token == "" {
		return fmt.Errorf("no AI Workspace bearer token available")
	}
	return nil
}

// initArtifact scaffolds the project with `ap project init` and then replaces the
// generated files with the suite's known-good "create" demo resources.
func (w *world) initArtifact(key string) error {
	spec, err := lookup(key)
	if err != nil {
		return err
	}

	res, err := runAP(nil, "project", "init",
		"--display-name", spec.dir,
		"--type", spec.initType,
		"--no-interactive",
	)
	if err != nil {
		return err
	}
	if res.exit != 0 {
		return fmt.Errorf("ap project init (%s) failed (exit %d):\n%s", spec.initType, res.exit, res.combined())
	}

	return stage(spec, "create")
}

// editArtifact overlays the "edit" demo variant (changed runtime + metadata that
// adds spec.associatedGateways) so the re-apply exercises the update path with
// genuinely modified content.
func (w *world) editArtifact(key string) error {
	spec, err := lookup(key)
	if err != nil {
		return err
	}
	return stage(spec, "edit")
}

// stage copies the demo resource files for the given variant ("create" or "edit")
// into the scaffolded project directory. definition.yaml is shared across
// variants and only staged on create.
func stage(spec artifactSpec, variant string) error {
	projectDir := filepath.Join(suite.workspaceRoot, spec.dir)
	files := []string{"metadata.yaml", "runtime.yaml"}
	if variant == "create" {
		files = append(files, "definition.yaml")
	}
	for _, name := range files {
		src := filepath.Join(suite.resourcesRoot, spec.resourceDir, variant, name)
		if name == "definition.yaml" {
			// definition.yaml lives at the resource-dir root, shared by both variants.
			src = filepath.Join(suite.resourcesRoot, spec.resourceDir, name)
		}
		dst := filepath.Join(projectDir, name)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("stage %s (%s) for %q: %w", name, variant, spec.dir, err)
		}
	}
	return nil
}

func (w *world) buildArtifact(key string) error {
	spec, err := lookup(key)
	if err != nil {
		return err
	}
	res, err := runAP(nil, "ai-workspace", "build", "-f", spec.dir)
	if err != nil {
		return err
	}
	w.last = res
	if res.exit != 0 {
		return fmt.Errorf("ap ai-workspace build (%s) failed (exit %d):\n%s", key, res.exit, res.combined())
	}
	return nil
}

// applyArtifact runs `ap ai-workspace apply`, supplying the bearer token via the
// environment and --project-id for project-scoped kinds. It records the result so
// the create/update assertions can inspect it.
func (w *world) applyArtifact(key string) error {
	spec, err := lookup(key)
	if err != nil {
		return err
	}
	args := []string{"ai-workspace", "apply", "-f", spec.dir, "--display-name", wsName, "--insecure"}
	if spec.projectScoped {
		args = append(args, "--project-id", suite.projectID)
	}
	res, err := runAP([]string{"WSO2AP_AIWORKSPACE_TOKEN=" + suite.token}, args...)
	if err != nil {
		return err
	}
	w.last = res
	if res.exit != 0 {
		return fmt.Errorf("ap ai-workspace apply (%s) failed (exit %d):\n%s", key, res.exit, res.combined())
	}
	return nil
}

func (w *world) reportedCreated(key string) error {
	return assertContainsAll(w.last, "Status: success", "applied successfully")
}

func (w *world) reportedUpdated(key string) error {
	return assertContainsAll(w.last, "Status: success", "updated successfully")
}

// artifactRetrievable confirms persistence by reading the artifact back through
// the CLI's own get-by-id command.
func (w *world) artifactRetrievable(key string) error {
	spec, err := lookup(key)
	if err != nil {
		return err
	}
	res, err := runAP([]string{"WSO2AP_AIWORKSPACE_TOKEN=" + suite.token},
		"ai-workspace", spec.group, "get", "--id", spec.dir, "--display-name", wsName, "--insecure")
	if err != nil {
		return err
	}
	w.last = res
	if res.exit != 0 {
		return fmt.Errorf("ap ai-workspace %s get (%s) failed (exit %d):\n%s", spec.group, key, res.exit, res.combined())
	}
	if !strings.Contains(res.combined(), spec.dir) {
		return fmt.Errorf("get output for %q did not contain id %q:\n%s", key, spec.dir, res.combined())
	}
	return nil
}

// artifactListed confirms the applied artifact appears in the CLI's list command.
// Provider lists are org-scoped; proxy/MCP lists require --project-id.
func (w *world) artifactListed(key string) error {
	spec, err := lookup(key)
	if err != nil {
		return err
	}
	args := []string{"ai-workspace", spec.group, "list", "--display-name", wsName, "--insecure"}
	if spec.projectScoped {
		args = append(args, "--project-id", suite.projectID)
	}
	res, err := runAP([]string{"WSO2AP_AIWORKSPACE_TOKEN=" + suite.token}, args...)
	if err != nil {
		return err
	}
	w.last = res
	if res.exit != 0 {
		return fmt.Errorf("ap ai-workspace %s list (%s) failed (exit %d):\n%s", spec.group, key, res.exit, res.combined())
	}
	if !strings.Contains(res.combined(), spec.dir) {
		return fmt.Errorf("list output for %q did not contain id %q:\n%s", key, spec.dir, res.combined())
	}
	return nil
}

// artifactAssociatedWithGateway confirms the update persisted spec.associatedGateways
// by reading the artifact back and checking the gateway handle appears in the
// server response.
func (w *world) artifactAssociatedWithGateway(key, gatewayID string) error {
	spec, err := lookup(key)
	if err != nil {
		return err
	}
	res, err := runAP([]string{"WSO2AP_AIWORKSPACE_TOKEN=" + suite.token},
		"ai-workspace", spec.group, "get", "--id", spec.dir, "--display-name", wsName, "--insecure")
	if err != nil {
		return err
	}
	w.last = res
	if res.exit != 0 {
		return fmt.Errorf("ap ai-workspace %s get (%s) failed (exit %d):\n%s", spec.group, key, res.exit, res.combined())
	}
	if !strings.Contains(res.combined(), gatewayID) {
		return fmt.Errorf("get output for %q did not show associated gateway %q:\n%s", key, gatewayID, res.combined())
	}
	return nil
}

func lookup(key string) (artifactSpec, error) {
	spec, ok := artifacts[key]
	if !ok {
		return artifactSpec{}, fmt.Errorf("unknown artifact key %q", key)
	}
	return spec, nil
}

func assertContainsAll(res cliResult, substrs ...string) error {
	out := res.combined()
	if res.exit != 0 {
		return fmt.Errorf("expected exit 0, got %d:\n%s", res.exit, out)
	}
	for _, s := range substrs {
		if !strings.Contains(out, s) {
			return fmt.Errorf("expected output to contain %q, got:\n%s", s, out)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

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

package steps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wso2/api-platform/cli/it/resources"
)

// DevPortalSteps provides developer-portal-specific step definitions. Most
// devportal commands are exercised through the generic `I run ap with arguments`
// step; these helpers cover the setup steps (configuring and selecting a
// devportal) and the end-to-end publish flow (build → publish → key → plan →
// subscribe) that needs to thread the generated API ID between commands.
type DevPortalSteps struct {
	state TestState

	// E2E flow state, captured between steps within a scenario.
	projectDir string
	zipPath    string
	orgID      string
	apiID      string
}

// NewDevPortalSteps creates a new DevPortalSteps instance.
func NewDevPortalSteps(state TestState) *DevPortalSteps {
	return &DevPortalSteps{state: state}
}

// EnsureDevPortalConfigured adds a devportal config pointing at the test
// deployment (api-key auth) and makes it the active devportal so subsequent
// commands that omit --display-name resolve to it.
func (s *DevPortalSteps) EnsureDevPortalConfigured(name string) error {
	err := s.state.ExecuteCLI(
		"devportal", "add",
		"--display-name", name,
		"--server", resources.DevPortalURL,
		"--auth", resources.DevPortalAuthType,
		"--api-key", resources.DevPortalAPIKey,
		"--no-interactive",
	)
	if err != nil {
		return err
	}
	if s.state.GetExitCode() != 0 {
		return fmt.Errorf("failed to add devportal %q: %s", name, s.state.GetCombinedOutput())
	}

	return s.SetCurrentDevPortal(name)
}

// SetCurrentDevPortal sets the active devportal.
func (s *DevPortalSteps) SetCurrentDevPortal(name string) error {
	if err := s.state.ExecuteCLI("devportal", "use", "--display-name", name); err != nil {
		return err
	}
	if s.state.GetExitCode() != 0 {
		return fmt.Errorf("failed to set current devportal %q: %s", name, s.state.GetCombinedOutput())
	}
	return nil
}

// runOK runs a CLI command and returns an error if it exits non-zero, so the
// calling step fails with the combined output for easy diagnosis.
func (s *DevPortalSteps) runOK(action string, args ...string) error {
	if err := s.state.ExecuteCLI(args...); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	if s.state.GetExitCode() != 0 {
		return fmt.Errorf("%s failed (exit %d):\n%s", action, s.state.GetExitCode(), s.state.GetCombinedOutput())
	}
	return nil
}

// BuildEchoAPIProject scaffolds an API project with `ap apiproject init`, stages
// the provided echo devportal artifact (devportal.yaml + definition.yaml) into a
// `devportal/` portal root, points the project config at it, and runs
// `ap devportal build` to produce build/devportal.zip.
func (s *DevPortalSteps) BuildEchoAPIProject() error {
	workDir := s.state.GetWorkingDir()
	if workDir == "" {
		return fmt.Errorf("working directory not set")
	}

	// 1. Scaffold the API project (created under workDir using the display name).
	if err := s.runOK("apiproject init",
		"apiproject", "init",
		"--display-name", "echo-api",
		"--type", "rest",
		"--version", "v2.0",
		"--context", "/ping",
		"--no-interactive",
	); err != nil {
		return err
	}
	s.projectDir = filepath.Join(workDir, "echo-api")

	// 2. Stage the devportal portal root with the provided artifact files.
	portalRoot := filepath.Join(s.projectDir, "devportal")
	for _, dir := range []string{portalRoot, filepath.Join(portalRoot, "docs"), filepath.Join(portalRoot, "content")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}
	if err := copyResourceFile(resources.GetDevPortalResourcePath("echo-devportal.yaml"), filepath.Join(portalRoot, "devportal.yaml")); err != nil {
		return err
	}
	if err := copyResourceFile(resources.GetDevPortalResourcePath("echo-definition.yaml"), filepath.Join(portalRoot, "definition.yaml")); err != nil {
		return err
	}

	// 3. Point the project config at the staged portal root so `build` bundles
	//    the provided artifacts as-is (instead of auto-generating a manifest).
	if err := os.WriteFile(filepath.Join(s.projectDir, ".api-platform", "config.yaml"), []byte(echoProjectConfig), 0644); err != nil {
		return fmt.Errorf("failed to write project config: %w", err)
	}

	// 4. Build the devportal artifact zip.
	if err := s.runOK("devportal build", "devportal", "build", "-f", s.projectDir); err != nil {
		return err
	}
	s.zipPath = filepath.Join(s.projectDir, "build", "devportal.zip")
	if _, err := os.Stat(s.zipPath); err != nil {
		return fmt.Errorf("expected built artifact at %s: %w", s.zipPath, err)
	}
	return nil
}

// TargetOrganization records the organization id used by the publish, plan,
// api-key, and subscription steps of the end-to-end flow.
func (s *DevPortalSteps) TargetOrganization(orgID string) error {
	s.orgID = strings.TrimSpace(orgID)
	if s.orgID == "" {
		return fmt.Errorf("organization id must not be empty")
	}
	return nil
}

// PublishBuiltAPI publishes the built artifact to the targeted organization and
// captures the generated Developer Portal API ID from the response.
func (s *DevPortalSteps) PublishBuiltAPI() error {
	if s.orgID == "" {
		return fmt.Errorf("no target organization; run the target organization step first")
	}
	if s.zipPath == "" {
		return fmt.Errorf("no built artifact; run the build step first")
	}
	if err := s.runOK("rest-api publish",
		"devportal", "rest-api", "publish", "-f", s.zipPath, "--org", s.orgID,
	); err != nil {
		return err
	}

	apiID, err := extractAPIID(s.state.GetStdout())
	if err != nil {
		return fmt.Errorf("could not capture API ID from publish response: %w\n%s", err, s.state.GetStdout())
	}
	s.apiID = apiID
	return nil
}

// GetPublishedAPI retrieves the published API by its captured ID. The CLI treats
// a 404 as a non-error (exit 0), so this also asserts the response echoes the
// captured API ID to confirm the API really is retrievable.
func (s *DevPortalSteps) GetPublishedAPI() error {
	if s.apiID == "" {
		return fmt.Errorf("no published API ID captured; run the publish step first")
	}
	if err := s.runOK("rest-api get",
		"devportal", "rest-api", "get", "--org", s.orgID, "--api-id", s.apiID,
	); err != nil {
		return err
	}
	if !strings.Contains(s.state.GetStdout(), s.apiID) {
		return fmt.Errorf("rest-api get did not return the published API %q:\n%s", s.apiID, s.state.GetCombinedOutput())
	}
	return nil
}

// GenerateAPIKeyForPublishedAPI generates an API key bound to the published API.
func (s *DevPortalSteps) GenerateAPIKeyForPublishedAPI(keyName string) error {
	if s.apiID == "" {
		return fmt.Errorf("no published API ID captured; run the publish step first")
	}
	return s.runOK("api-key generate",
		"devportal", "api-key", "generate",
		"--org", s.orgID, "--api-id", s.apiID, "--name", keyName, "--no-interactive",
	)
}

// PublishSubscriptionPlan uploads a subscription plan CR to the targeted org.
// Run before publishing the API so the API can reference the new plan (the
// publish flow fails if a listed policy name does not exist in the org).
func (s *DevPortalSteps) PublishSubscriptionPlan(resourcePath string) error {
	if s.orgID == "" {
		return fmt.Errorf("no target organization; run the target organization step first")
	}
	planPath := resources.GetDevPortalResourcePath(filepath.Base(resourcePath))
	return s.runOK("sub-plan publish",
		"devportal", "sub-plan", "publish", "-f", planPath, "--org", s.orgID,
	)
}

// CreateSubscriptionForPublishedAPI subscribes to the published API with a plan.
func (s *DevPortalSteps) CreateSubscriptionForPublishedAPI(plan string) error {
	if s.apiID == "" {
		return fmt.Errorf("no published API ID captured; run the publish step first")
	}
	return s.runOK("subscription create",
		"devportal", "subscription", "create",
		"--org", s.orgID, "--api-id", s.apiID, "--subscription-plan", plan,
	)
}

// echoProjectConfig is the .api-platform/config.yaml written for the staged echo
// project. The devportals entry makes `ap devportal build` bundle the staged
// portal root verbatim.
const echoProjectConfig = `version: 1.0.0
filePaths:
  deploymentArtifact: ./gateway.yaml
  apiMetadata: ./api.yaml
  apiDefinition: ./definition.yaml
  docs: ./docs
  tests: ./tests
governanceRulesets: []
autoSync:
  gatewayArtifactFromDefinition: true
devportals:
  - name: default
    portalRoot: ./devportal
    filePaths:
      apiMetadata: ./devportal.yaml
      apiDefinition: ./definition.yaml
      docs: ./docs
      content: ./content
`

// copyResourceFile copies a source file to dst, creating parent directories.
func copyResourceFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read resource %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create dir for %s: %w", dst, err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", dst, err)
	}
	return nil
}

// extractAPIID pulls the top-level "apiID" field from the publish command's
// stdout, which prints a header line followed by the pretty-printed JSON body.
func extractAPIID(stdout string) (string, error) {
	start := strings.Index(stdout, "{")
	if start < 0 {
		return "", fmt.Errorf("no JSON object found in output")
	}
	var body struct {
		APIID string `json:"apiID"`
	}
	if err := json.Unmarshal([]byte(stdout[start:]), &body); err != nil {
		return "", fmt.Errorf("failed to parse response JSON: %w", err)
	}
	if strings.TrimSpace(body.APIID) == "" {
		return "", fmt.Errorf("response did not contain a non-empty apiID")
	}
	return body.APIID, nil
}

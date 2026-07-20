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

// Package awcli holds the AI Workspace CLI end-to-end suite. It boots a real
// platform-api control plane (the AI Workspace backend) over docker compose,
// then drives the real `ap` CLI binary against it: register the workspace, scaffold
// projects with `ap project init`, and publish the three AI Workspace artifact
// kinds (LLM provider, LLM proxy, MCP proxy) with `ap ai-workspace build`/`apply`,
// asserting the CLI persists each artifact and can read it back.
package awcli

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
)

const (
	// composeProject namespaces the docker compose stack for this suite.
	composeProject = "aiwscli-e2e"
	// composeFile is the standalone platform-api compose in this directory.
	composeFile = "docker-compose.yaml"

	// portalAuthBase is where the file-based login endpoint lives.
	portalAuthBase = "/api/portal/v0.9"
	// apiBase is the AI Workspace / project REST base path.
	apiBase = "/api/v0.9"

	// wsName is the CLI-side name we register the platform-api under.
	wsName = "it-ws"

	// pollTimeout bounds platform-api readiness polling.
	pollTimeout = 180 * time.Second
)

// platformAPI is the host-side base URL of the platform-api HTTPS listener.
var platformAPI = "https://localhost:" + envOr("PA_HOST_PORT", "9243")

// demoGateways are the gateway handles referenced by the edit artifacts'
// spec.associatedGateways. They are registered up front so the server can
// resolve the associations (resolveAssociatedGateways rejects unknown handles).
var demoGateways = []string{"prod-eu-01", "prod-eu-02"}

// httpClient talks to the self-signed platform-api directly (login + project
// bootstrap only — every artifact operation goes through the `ap` CLI).
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 — local self-signed cert
	},
}

// awSuite is the shared, suite-scoped state populated in BeforeSuite and read by
// the step definitions.
type awSuite struct {
	token         string // admin JWT from /auth/login, reused as the AI Workspace bearer token
	projectID     string // server-side project handle for project-scoped kinds (proxies/MCP)
	cliBin        string // absolute path to the built `ap` binary
	home          string // isolated HOME so the CLI writes its own ~/.wso2ap/config.yaml
	workspaceRoot string // scratch dir where `ap project init` scaffolds projects
	resourcesRoot string // absolute path to this suite's resources/ dir
}

var suite awSuite

// TestFeatures is the suite entry point. Tags can be narrowed via AW_TAGS.
func TestFeatures(t *testing.T) {
	status := godog.TestSuite{
		Name:                 "ai-workspace-cli-e2e",
		TestSuiteInitializer: initializeSuite,
		ScenarioInitializer:  initializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			Tags:     os.Getenv("AW_TAGS"),
			Strict:   true,
			TestingT: t,
		},
	}.Run()
	if status != 0 {
		t.Fatalf("godog suite failed with status %d", status)
	}
}

func initializeSuite(ctx *godog.TestSuiteContext) {
	ctx.BeforeSuite(func() {
		if err := bringUp(); err != nil {
			tearDown()
			panic(fmt.Sprintf("failed to bring up AI Workspace CLI e2e stack: %v", err))
		}
	})
	ctx.AfterSuite(func() {
		if os.Getenv("AW_KEEP") == "" {
			tearDown()
		}
	})
}

// bringUp starts platform-api, waits for health, mints an admin token, creates a
// server-side project, verifies the `ap` binary, and registers the workspace in
// an isolated CLI config.
func bringUp() error {
	if err := compose("up", "-d"); err != nil {
		return fmt.Errorf("start platform-api: %w", err)
	}
	if err := waitHealthy(); err != nil {
		return err
	}
	if err := login(); err != nil {
		return err
	}
	if err := createProject(); err != nil {
		return err
	}
	for _, gw := range demoGateways {
		if err := createGateway(gw); err != nil {
			return err
		}
	}
	if err := resolveCLI(); err != nil {
		return err
	}
	if err := setupCLIEnv(); err != nil {
		return err
	}
	return registerWorkspace()
}

func tearDown() {
	_ = compose("down", "-v", "--remove-orphans")
	if suite.home != "" {
		_ = os.RemoveAll(suite.home)
	}
	if suite.workspaceRoot != "" {
		_ = os.RemoveAll(suite.workspaceRoot)
	}
}

// compose runs `docker compose` for this suite's stack. PLATFORM_API_IMAGE and
// PA_HOST_PORT flow through from the process environment.
func compose(args ...string) error {
	full := append([]string{"compose", "-p", composeProject, "-f", composeFile}, args...)
	cmd := exec.Command("docker", full...)
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker compose %v: %w\n%s", args, err, out)
	}
	return nil
}

func waitHealthy() error {
	deadline := time.Now().Add(pollTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := httpClient.Get(platformAPI + "/health")
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK && bytes.Contains(body, []byte("ok")) {
				return nil
			}
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, body)
		} else {
			lastErr = err
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("platform-api did not become healthy at %s: %v", platformAPI, lastErr)
}

// login exchanges admin/admin for a JWT via the file-based login endpoint.
func login() error {
	form := url.Values{"username": {"admin"}, "password": {"admin"}}
	resp, err := httpClient.PostForm(platformAPI+portalAuthBase+"/auth/login", form)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed (%d): %s", resp.StatusCode, body)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("decode login response: %w (%s)", err, body)
	}
	if out.Token == "" {
		return fmt.Errorf("login response had no token: %s", body)
	}
	suite.token = out.Token
	return nil
}

// createProject creates a server-side project and records its handle, which the
// project-scoped kinds (LLM proxy, MCP proxy) pass to `apply --project-id`.
func createProject() error {
	reqBody, _ := json.Marshal(map[string]string{"displayName": "IT AI Workspace Project"})
	req, err := http.NewRequest(http.MethodPost, platformAPI+apiBase+"/projects", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+suite.token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create project request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("create project failed (%d): %s", resp.StatusCode, body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("decode project response: %w (%s)", err, body)
	}
	if out.ID == "" {
		return fmt.Errorf("project response had no id: %s", body)
	}
	suite.projectID = out.ID
	return nil
}

// createGateway registers a control-plane gateway by handle so artifacts can
// associate with it. associatedGateways references are validated server-side
// (unknown handles are rejected), so the demo handles must exist before apply.
func createGateway(handle string) error {
	reqBody, _ := json.Marshal(map[string]any{
		"id":                handle,
		"displayName":       handle,
		"endpoints":         []string{"http://" + handle + ".example.com"},
		"functionalityType": "regular",
	})
	req, err := http.NewRequest(http.MethodPost, platformAPI+apiBase+"/gateways", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+suite.token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create gateway %q request: %w", handle, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// Treat an already-existing gateway as success so reruns against a kept stack
	// (AW_KEEP) don't fail.
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("create gateway %q failed (%d): %s", handle, resp.StatusCode, body)
	}
	return nil
}

// resolveCLI locates and smoke-tests the `ap` binary (override with AP_CLI_BINARY).
func resolveCLI() error {
	bin := envOr("AP_CLI_BINARY", "")
	if bin == "" {
		abs, err := filepath.Abs(filepath.Join("..", "..", "cli", "src", "build", "ap"))
		if err != nil {
			return fmt.Errorf("resolve CLI path: %w", err)
		}
		bin = abs
	}
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("ap binary not found at %s (build it with 'make -C cli/src build-skip-tests'): %w", bin, err)
	}
	suite.cliBin = bin
	if res, err := runAP(nil, "version"); err != nil || res.exit != 0 {
		return fmt.Errorf("ap version failed (exit %d): %v\n%s%s", res.exit, err, res.stdout, res.stderr)
	}
	return nil
}

// setupCLIEnv creates the isolated HOME + scratch workspace and resolves the
// resources dir.
func setupCLIEnv() error {
	home, err := os.MkdirTemp("", "aiwscli-home-*")
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "aiwscli-work-*")
	if err != nil {
		return err
	}
	res, err := filepath.Abs("resources")
	if err != nil {
		return err
	}
	suite.home = home
	suite.workspaceRoot = work
	suite.resourcesRoot = res
	return nil
}

// registerWorkspace points the CLI at platform-api using oauth (bearer) auth. No
// token is stored on disk; it is supplied per-call via WSO2AP_AIWORKSPACE_TOKEN.
func registerWorkspace() error {
	res, err := runAP(nil, "ai-workspace", "add",
		"--display-name", wsName,
		"--server", platformAPI,
		"--auth", "oauth",
		"--no-interactive",
	)
	if err != nil {
		return err
	}
	if res.exit != 0 {
		return fmt.Errorf("ai-workspace add failed (exit %d):\n%s%s", res.exit, res.stdout, res.stderr)
	}
	res, err = runAP(nil, "ai-workspace", "use", "--display-name", wsName)
	if err != nil {
		return err
	}
	if res.exit != 0 {
		return fmt.Errorf("ai-workspace use failed (exit %d):\n%s%s", res.exit, res.stdout, res.stderr)
	}
	return nil
}

// cliResult captures a single `ap` invocation.
type cliResult struct {
	stdout string
	stderr string
	exit   int
}

func (r cliResult) combined() string { return r.stdout + r.stderr }

// runAP executes the `ap` binary with an isolated HOME (so its config never
// touches the developer's real ~/.wso2ap) and the given extra environment.
// A non-zero exit is returned in the result (not as a Go error); a Go error
// means the process could not run at all.
func runAP(extraEnv []string, args ...string) (cliResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, suite.cliBin, args...)
	cmd.Dir = suite.workspaceRoot
	cmd.Env = append(os.Environ(), "AP_NO_COLOR=true", "HOME="+suite.home)
	cmd.Env = append(cmd.Env, extraEnv...)

	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()

	res := cliResult{stdout: out.String(), stderr: errb.String()}
	if ee, ok := err.(*exec.ExitError); ok {
		res.exit = ee.ExitCode()
		return res, nil
	}
	if err != nil {
		res.exit = -1
		return res, fmt.Errorf("run ap %s: %w", strings.Join(args, " "), err)
	}
	return res, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

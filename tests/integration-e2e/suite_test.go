/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

// Package e2e is a godog (Cucumber) suite that drives the real platform-api
// control plane and the real gateway data plane end to end, on a database
// engine selected by E2E_DB (postgres | sqlite | sqlserver).
package e2e

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/cucumber/godog"
)

const (
	composeProject = "apip-e2e-bdd"
	ingressHost    = "localhost"
	pollTimeout    = 90 * time.Second
)

// Host-side endpoints. Ports are overridable so the suite can run alongside
// other local stacks; container-internal wiring (controller -> platform-api:9243)
// is unaffected. Defaults match the compose files and CI.
var (
	platformAPI = "https://localhost:" + envOr("PA_HOST_PORT", "9243")
	ingressGw1  = "http://localhost:" + envOr("GW_HTTP_PORT", "18080")
	ingressGw2  = "http://localhost:" + envOr("GW2_HTTP_PORT", "18081")
)

// suite holds state established once for the whole run (BeforeSuite): the chosen
// database, the running stack, the admin token and the pre-registered gateways.
var suite struct {
	db          string // postgres | sqlite | sqlserver
	composeFile string
	multi       bool // second gateway available (postgres stack only)
	token       string
	projectID   string
	gw1ID       string
	gw2ID       string
}

var httpClient = &http.Client{
	Timeout:   25 * time.Second,
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
}

// TestFeatures is the go test entry point that runs the godog suite.
func TestFeatures(t *testing.T) {
	tags := os.Getenv("E2E_TAGS")
	if tags == "" && os.Getenv("E2E_DB") != "" && os.Getenv("E2E_DB") != "postgres" {
		tags = "~@multigateway" // second gateway is only wired on the postgres stack
	}
	status := godog.TestSuite{
		Name:                 "platform-api-gateway-e2e",
		TestSuiteInitializer: initializeSuite,
		ScenarioInitializer:  initializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			Tags:     tags,
			Strict:   true,
			TestingT: t,
		},
	}.Run()
	if status != 0 {
		t.Fatalf("godog suite failed with status %d", status)
	}
}

// initializeSuite brings the whole stack up before any scenario and tears it
// down afterwards. The registration-token bootstrap (create gateway -> mint
// token -> start its controller) is done here so scenarios can simply deploy.
func initializeSuite(ctx *godog.TestSuiteContext) {
	ctx.BeforeSuite(func() {
		if err := bringUpStack(); err != nil {
			tearDownStack()
			panic(fmt.Sprintf("e2e setup failed: %v", err))
		}
	})
	ctx.AfterSuite(func() {
		if os.Getenv("E2E_KEEP") == "" {
			tearDownStack()
		}
	})
}

func bringUpStack() error {
	suite.db = envOr("E2E_DB", "postgres")
	switch suite.db {
	case "postgres":
		suite.composeFile, suite.multi = "docker-compose.yaml", true
	case "sqlite":
		suite.composeFile = "docker-compose.sqlite.yaml"
	case "sqlserver":
		suite.composeFile = "docker-compose.sqlserver.yaml"
	default:
		return fmt.Errorf("unsupported E2E_DB %q", suite.db)
	}
	fmt.Printf("E2E database backend: %s (%s)\n", suite.db, suite.composeFile)

	// Phase 1: control plane + backend.
	phase1 := []string{"platform-api", "sample-backend"}
	if suite.db != "sqlite" {
		phase1 = append([]string{dbService()}, phase1...)
	}
	if err := compose(nil, append([]string{"up", "-d"}, phase1...)...); err != nil {
		return fmt.Errorf("start control plane: %w", err)
	}
	if err := waitHealthy(); err != nil {
		return err
	}

	// Bootstrap: authenticate, project, gateways + registration tokens.
	var err error
	if suite.token, err = login(); err != nil {
		return err
	}
	if suite.projectID, err = createProject(); err != nil {
		return err
	}
	gw1Token, gw2Token := "", ""
	if suite.gw1ID, gw1Token, err = createGatewayAndToken("e2e-gw"); err != nil {
		return err
	}
	env := map[string]string{"GATEWAY_REGISTRATION_TOKEN": gw1Token}
	dataPlane := []string{"gateway-controller", "gateway-runtime"}
	if suite.multi {
		if suite.gw2ID, gw2Token, err = createGatewayAndToken("e2e-gw2"); err != nil {
			return err
		}
		env["GATEWAY_REGISTRATION_TOKEN_2"] = gw2Token
		dataPlane = append(dataPlane, "gateway-controller-2", "gateway-runtime-2")
	}

	// Phase 2: data plane with the minted tokens.
	if err := compose(env, append([]string{"up", "-d"}, dataPlane...)...); err != nil {
		return fmt.Errorf("start data plane: %w", err)
	}
	return nil
}

func tearDownStack() {
	_ = compose(map[string]string{"GATEWAY_REGISTRATION_TOKEN": "x"}, "down", "-v", "--remove-orphans")
}

func dbService() string {
	if suite.db == "sqlserver" {
		return "sqlserver"
	}
	return "postgres"
}

// compose runs `docker compose -p <project> -f <file> <args...>` with optional
// extra environment (for the registration tokens).
func compose(extraEnv map[string]string, args ...string) error {
	full := append([]string{"compose", "-p", composeProject, "-f", suite.composeFile}, args...)
	cmd := exec.Command("docker", full...)
	cmd.Env = os.Environ()
	for k, v := range extraEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker %v: %w\n%s", args, err, out)
	}
	return nil
}

func waitHealthy() error {
	deadline := time.Now().Add(pollTimeout * 2)
	for time.Now().Before(deadline) {
		resp, err := httpClient.Get(platformAPI + "/health")
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 && bytes.Contains(body, []byte("ok")) {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("platform-api did not become healthy")
}

// --- platform-api REST helpers --------------------------------------------

func apiCall(method, path, token string, body any) (int, []byte, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, platformAPI+path, rdr)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out, nil
}

func login() (string, error) {
	form := url.Values{"username": {"admin"}, "password": {"admin"}}
	req, err := http.NewRequest(http.MethodPost, platformAPI+"/api/portal/v0.9/auth/login",
		bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(body, &r)
	if resp.StatusCode != 200 || r.Token == "" {
		return "", fmt.Errorf("login failed (%d): %s", resp.StatusCode, body)
	}
	return r.Token, nil
}

func createProject() (string, error) {
	st, body, err := apiCall(http.MethodPost, "/api/v0.9/projects", suite.token,
		map[string]string{"id": "e2e-proj", "displayName": "e2e-proj", "description": "e2e"})
	if err != nil {
		return "", err
	}
	id := jsonField(body, "id", "uuid")
	if st >= 300 || id == "" {
		return "", fmt.Errorf("create project failed (%d): %s", st, body)
	}
	return id, nil
}

func createGatewayAndToken(name string) (gatewayID, token string, err error) {
	st, body, err := apiCall(http.MethodPost, "/api/v0.9/gateways", suite.token, map[string]any{
		"id": name, "displayName": name, "vhost": ingressHost, "functionalityType": "regular",
	})
	if err != nil {
		return "", "", err
	}
	gatewayID = jsonField(body, "id", "uuid")
	if st >= 300 || gatewayID == "" {
		return "", "", fmt.Errorf("create gateway failed (%d): %s", st, body)
	}
	st, body, err = apiCall(http.MethodPost, "/api/v0.9/gateways/"+gatewayID+"/tokens", suite.token, map[string]any{})
	if err != nil {
		return "", "", err
	}
	token = jsonField(body, "token")
	if st >= 300 || token == "" {
		return "", "", fmt.Errorf("rotate token failed (%d): %s", st, body)
	}
	return gatewayID, token, nil
}

// --- small helpers ---------------------------------------------------------

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// jsonField returns the first non-empty string value among the given keys.
func jsonField(body []byte, keys ...string) string {
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return ""
	}
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

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
	"mime/multipart"
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
	if tags == "" {
		// @wip scenarios are quarantined (a known platform-api bug — see
		// features/llm-provider-secret.feature); run them explicitly with
		// E2E_TAGS=@llm to reproduce.
		tags = "~@wip"
		if db := os.Getenv("E2E_DB"); db != "" && db != "postgres" {
			// The second gateway is only wired on the postgres stack; the
			// restart/recovery, secret and lifecycle scenarios are validated
			// postgres-only. Only the base api-deployment feature runs cross-DB.
			tags += " && ~@multigateway && ~@restart && ~@secret && ~@lifecycle && ~@apikey && ~@mcp"
		}
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
		"id": name, "displayName": name, "endpoints": []string{"http://" + ingressHost}, "functionalityType": "regular",
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

// createSecret stores an organization-scoped secret in platform-api. The endpoint
// is multipart/form-data (not JSON); id is the handle used in {{ secret "id" }}
// placeholders, value is the plaintext (encrypted at rest, never returned).
func createSecret(id, value string) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fields := []struct{ k, v string }{
		{"id", id}, {"displayName", id}, {"value", value}, {"type", "GENERIC"},
	}
	for _, f := range fields {
		if err := mw.WriteField(f.k, f.v); err != nil {
			return err
		}
	}
	if err := mw.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, platformAPI+"/api/v0.9/secrets", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+suite.token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("create secret failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}

// restAPIWithHeaderBody builds a REST API create/update body routed to the sample
// backend with one GET operation carrying a set-headers policy that injects
// headerName=headerValue into the upstream request. In platform-api,
// method/path/policies nest under operations[].request. The sample backend echoes
// the header back so a test can assert it reached the upstream.
func restAPIWithHeaderBody(displayName, context, headerName, headerValue string) map[string]any {
	return map[string]any{
		"displayName": displayName,
		"context":     context,
		"version":     "v1",
		"projectId":   suite.projectID,
		"upstream":    map[string]any{"main": map[string]any{"url": "http://sample-backend:9080"}},
		"operations": []map[string]any{{
			"name": "getRoot",
			"request": map[string]any{
				"method": "GET",
				"path":   "/",
				"policies": []map[string]any{{
					"name":    "set-headers",
					"version": "v1",
					"params": map[string]any{
						"request": map[string]any{
							"headers": []map[string]any{{"name": headerName, "value": headerValue}},
						},
					},
				}},
			},
		}},
	}
}

// createRestAPIWithSecretHeader creates a REST API whose set-headers policy injects
// `<headerName>: Bearer {{ secret "<secretID>" }}` — the gateway resolves the
// placeholder at runtime.
func createRestAPIWithSecretHeader(secretID, headerName string) (id, context string, err error) {
	suffix := randHex()
	context = "/e2e-" + suffix
	body := restAPIWithHeaderBody("e2e-secret-api-"+suffix, context, headerName,
		`Bearer {{ secret "`+secretID+`" }}`)
	st, resp, err := apiCall(http.MethodPost, "/api/v0.9/rest-apis", suite.token, body)
	if err != nil {
		return "", "", err
	}
	id = jsonField(resp, "id", "handle", "uuid")
	if st >= 300 || id == "" {
		return "", "", fmt.Errorf("create secret API failed (%d): %s", st, resp)
	}
	return id, context, nil
}

// createRestAPIWithHeader creates a REST API whose set-headers policy injects a
// literal headerName=headerValue. Returns the API id, context and displayName
// (the displayName is needed to PUT-update the API later).
func createRestAPIWithHeader(headerName, headerValue string) (id, context, displayName string, err error) {
	suffix := randHex()
	context = "/e2e-" + suffix
	displayName = "e2e-hdr-api-" + suffix
	body := restAPIWithHeaderBody(displayName, context, headerName, headerValue)
	st, resp, err := apiCall(http.MethodPost, "/api/v0.9/rest-apis", suite.token, body)
	if err != nil {
		return "", "", "", err
	}
	id = jsonField(resp, "id", "handle", "uuid")
	if st >= 300 || id == "" {
		return "", "", "", fmt.Errorf("create API failed (%d): %s", st, resp)
	}
	return id, context, displayName, nil
}

// updateRestAPIHeader PUT-updates the API (full-object replace) to inject a new
// header value, so a subsequent redeploy carries the mutated spec.
func updateRestAPIHeader(id, displayName, context, headerName, headerValue string) error {
	body := restAPIWithHeaderBody(displayName, context, headerName, headerValue)
	st, resp, err := apiCall(http.MethodPut, "/api/v0.9/rest-apis/"+id, suite.token, body)
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("update API failed (%d): %s", st, resp)
	}
	return nil
}

// deleteRestAPI deletes the API from the control plane; the gateway receives an
// api.deleted event and stops serving it.
func deleteRestAPI(id string) error {
	st, resp, err := apiCall(http.MethodDelete, "/api/v0.9/rest-apis/"+id, suite.token, nil)
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("delete API failed (%d): %s", st, resp)
	}
	return nil
}

// restoreDeployment restores a previously UNDEPLOYED deployment on its gateway.
func restoreDeployment(apiID, deploymentID, gatewayID string) error {
	st, resp, err := apiCall(http.MethodPost,
		"/api/v0.9/rest-apis/"+apiID+"/deployments/"+deploymentID+"/restore?gatewayId="+gatewayID,
		suite.token, nil)
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("restore deployment failed (%d): %s", st, resp)
	}
	return nil
}

// createRestAPIRequiringKey creates a REST API routed to the sample backend whose
// GET operation carries an api-key-auth policy (the key is read from keyHeader).
// Once deployed, the gateway rejects requests without a valid key. (API-key auth
// is a policy on the operation, not a top-level security field.)
func createRestAPIRequiringKey(keyHeader string) (id, context string, err error) {
	suffix := randHex()
	context = "/e2e-" + suffix
	body := map[string]any{
		"displayName": "e2e-key-api-" + suffix,
		"context":     context,
		"version":     "v1",
		"projectId":   suite.projectID,
		"upstream":    map[string]any{"main": map[string]any{"url": "http://sample-backend:9080"}},
		"operations": []map[string]any{{
			"name": "getRoot",
			"request": map[string]any{
				"method": "GET",
				"path":   "/",
				"policies": []map[string]any{{
					"name":    "api-key-auth",
					"version": "v1",
					"params":  map[string]any{"key": keyHeader, "in": "header"},
				}},
			},
		}},
	}
	st, resp, err := apiCall(http.MethodPost, "/api/v0.9/rest-apis", suite.token, body)
	if err != nil {
		return "", "", err
	}
	id = jsonField(resp, "id", "handle", "uuid")
	if st >= 300 || id == "" {
		return "", "", fmt.Errorf("create key-protected API failed (%d): %s", st, resp)
	}
	return id, context, nil
}

// createAPIKeyForAPI registers a caller-supplied plaintext API key for the API.
// platform-api hashes it and broadcasts it to the gateways where the API is
// deployed (the apikey.created event), so the gateway then accepts that key.
func createAPIKeyForAPI(apiID, keyValue string) error {
	st, resp, err := apiCall(http.MethodPost, "/api/v0.9/rest-apis/"+apiID+"/api-keys", suite.token,
		map[string]any{"displayName": "e2e-key", "apiKey": keyValue})
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("create API key failed (%d): %s", st, resp)
	}
	return nil
}

// createMCPProxy creates an MCP proxy whose upstream is the real MCP server
// (mcp-backend). Returns the proxy handle (used in the deployment path) and its
// context. The gateway serves it at <context>/mcp.
func createMCPProxy() (handle, context string, err error) {
	suffix := randHex()
	handle = "mcp-" + suffix
	context = "/mcp-" + suffix
	body := map[string]any{
		"id":             handle,
		"displayName":    "e2e-mcp-" + suffix,
		"version":        "v1.0",
		"context":        context,
		"mcpSpecVersion": "2025-06-18",
		"upstream":       map[string]any{"main": map[string]any{"url": "http://mcp-backend:3001"}},
	}
	st, resp, err := apiCall(http.MethodPost, "/api/v0.9/mcp-proxies", suite.token, body)
	if err != nil {
		return "", "", err
	}
	if id := jsonField(resp, "id", "handle"); id != "" {
		handle = id
	}
	if st >= 300 {
		return "", "", fmt.Errorf("create MCP proxy failed (%d): %s", st, resp)
	}
	return handle, context, nil
}

// deployMCPProxy deploys the MCP proxy (by handle) to the gateway; the deploy call
// creates the gateway association (no separate attach). Returns the deployment id.
func deployMCPProxy(handle, gatewayID string) (string, error) {
	st, resp, err := apiCall(http.MethodPost, "/api/v0.9/mcp-proxies/"+handle+"/deployments", suite.token,
		map[string]any{"base": "current", "gatewayId": gatewayID, "name": "dep-" + randHex()})
	if err != nil {
		return "", err
	}
	id := jsonField(resp, "deploymentId")
	if st >= 300 || id == "" {
		return "", fmt.Errorf("deploy MCP proxy failed (%d): %s", st, resp)
	}
	return id, nil
}

// createLLMProxy creates an LLM proxy over an existing (deployed) LLM provider,
// referenced by handle. Returns the proxy handle and context. NOTE: deploying the
// proxy to a gateway requires the referenced provider (and its template) to be
// deployed there first — which currently hits the provider template-apiVersion
// bug (see features/llm-provider-secret.feature), so the proxy e2e is @wip.
func createLLMProxy(providerHandle string) (handle, context string, err error) {
	suffix := randHex()
	handle = "lp-" + suffix
	context = "/lp-" + suffix
	body := map[string]any{
		"id":          handle,
		"displayName": "e2e-llmproxy-" + suffix,
		"version":     "v1.0",
		"projectId":   suite.projectID,
		"context":     context,
		"provider":    map[string]any{"id": providerHandle},
	}
	st, resp, err := apiCall(http.MethodPost, "/api/v0.9/llm-proxies", suite.token, body)
	if err != nil {
		return "", "", err
	}
	if id := jsonField(resp, "id", "handle"); id != "" {
		handle = id
	}
	if st >= 300 {
		return "", "", fmt.Errorf("create LLM proxy failed (%d): %s", st, resp)
	}
	return handle, context, nil
}

// deployLLMProxy deploys the LLM proxy (by handle) to the gateway. Returns the
// deployment id.
func deployLLMProxy(handle, gatewayID string) (string, error) {
	st, resp, err := apiCall(http.MethodPost, "/api/v0.9/llm-proxies/"+handle+"/deployments", suite.token,
		map[string]any{"base": "current", "gatewayId": gatewayID, "name": "dep-" + randHex()})
	if err != nil {
		return "", err
	}
	id := jsonField(resp, "deploymentId")
	if st >= 300 || id == "" {
		return "", fmt.Errorf("deploy LLM proxy failed (%d): %s", st, resp)
	}
	return id, nil
}

// createLLMProviderWithSecretAuth creates an openai-template LLM provider routed
// to the sample backend, whose upstream carries an Authorization header built
// from the secret (upstream.main.auth.value = 'Bearer {{ secret "<secretID>" }}').
// The gateway resolves the placeholder and injects it upstream at runtime. Unlike
// a REST API, an LLM provider has no projectId and needs no separate gateway
// attach — the deployment call creates the association. Returns the provider
// handle (used in the deployment path) and its context.
func createLLMProviderWithSecretAuth(secretID string) (handle, context string, err error) {
	suffix := randHex()
	handle = "llm-" + suffix   // handle used in the deployment path
	context = "/llm-" + suffix // context is <= 20 chars, no trailing slash
	st, body, err := apiCall(http.MethodPost, "/api/v0.9/llm-providers", suite.token, map[string]any{
		"id":          handle,
		"displayName": "e2e-llm-" + suffix,
		"version":     "v1.0",
		"context":     context,
		"template":    "openai",
		"upstream": map[string]any{
			"main": map[string]any{
				"url": "http://sample-backend:9080",
				"auth": map[string]any{
					"type":   "api-key",
					"header": "Authorization",
					"value":  `Bearer {{ secret "` + secretID + `" }}`,
				},
			},
		},
		"accessControl": map[string]any{"mode": "allow_all"},
	})
	if err != nil {
		return "", "", err
	}
	if id := jsonField(body, "id", "handle"); id != "" {
		handle = id
	}
	if st >= 300 {
		return "", "", fmt.Errorf("create LLM provider failed (%d): %s", st, body)
	}
	return handle, context, nil
}

// deployLLMProvider deploys the provider (by handle) to the gateway. The deploy
// body is the same shape as a REST API deployment, and creates the gateway
// association itself — no prior attach call. Returns the deployment id.
func deployLLMProvider(handle, gatewayID string) (string, error) {
	st, body, err := apiCall(http.MethodPost, "/api/v0.9/llm-providers/"+handle+"/deployments", suite.token,
		map[string]any{"base": "current", "gatewayId": gatewayID, "name": "dep-" + randHex()})
	if err != nil {
		return "", err
	}
	id := jsonField(body, "deploymentId")
	if st >= 300 || id == "" {
		return "", fmt.Errorf("deploy LLM provider failed (%d): %s", st, body)
	}
	return id, nil
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

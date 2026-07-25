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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
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
	composeProject = "apip-e2e-bdd"
	ingressHost    = "localhost"
	// Ingress/readiness poll budget. Generous headroom so the full multi-scenario
	// run (postgres + two gateways + devportal, with controller restarts) tolerates
	// slower Envoy config propagation under load on constrained hosts.
	pollTimeout = 120 * time.Second

	// Webhook HMAC secret shared with the developer portal (matches WEBHOOK_SECRET in
	// docker-compose.yaml). The RSA key pair used for the encrypted key/token fields
	// is generated fresh per run by prepareWebhookKey (not read from the repo — the
	// private key is intentionally gitignored and absent in CI).
	webhookSecret = "5bd108b058ac9b318faf771c82a3f88bf6d3be5cc51c221e7ee213dabdbdee22"

	// Admin user injected via AUTH_FILE_BASED_USERS on the @devportal stack. It
	// carries both the platform-api ap:* scopes and the dp:*_manage scopes the
	// developer portal requires, so the same admin JWT authorizes both products.
	// (A mounted config's users are ignored — the built-in default admin wins —
	// but the AUTH_FILE_BASED_USERS env var does override it.)
	fileBasedAdminUsers = `[{"username":"admin","password_hash":"$2y$10$U2yKMwGamGwDoMu0hRPT7u8nCuP8z/qxHFOKV6dhIxkJN9NJ0eVQ.","scopes":"ap:organization:manage ap:gateway:manage ap:gateway_custom_policy:manage ap:rest_api:manage ap:llm_provider:manage ap:llm_proxy:manage ap:mcp_proxy:manage ap:webbroker_api:manage ap:websub_api:manage ap:application:manage ap:subscription:manage ap:subscription_plan:manage ap:project:manage ap:llm_template:manage ap:devportal:manage ap:api_key:read ap:secret:manage dp:org_manage dp:api_manage dp:sub_plan_manage dp:app_manage dp:subscription_manage dp:api_key_manage dp:webhook_subscriber_manage"}]`
)

// Host-side endpoints. Ports are overridable so the suite can run alongside
// other local stacks; container-internal wiring (controller -> platform-api:9243)
// is unaffected. Defaults match the compose files and CI.
var (
	platformAPI  = "https://localhost:" + envOr("PA_HOST_PORT", "9243")
	ingressGw1   = "http://localhost:" + envOr("GW_HTTP_PORT", "18080")
	ingressGw2   = "http://localhost:" + envOr("GW2_HTTP_PORT", "18081")
	devportalAPI = "http://localhost:" + envOr("DP_HOST_PORT", "9543")
	// gwMgmtAPI is the gateway-controller management REST API (port 9090 in the
	// e2e compose, overridable via GW_MGMT_PORT). Used to verify that resources
	// deployed via platform-api are visible on the data plane.
	gwMgmtAPI = "http://localhost:" + envOr("GW_MGMT_PORT", "9090")
)

// REST API base paths, defined in one place so each product's API version/prefix can
// be reconfigured centrally — and independently, since platform-api and the developer
// portal version on separate release cadences and may diverge. Overridable via env
// (PA_API_BASE / DP_API_BASE) so a version bump needs no code change. platformAPIBase
// is prepended by the apiCall helper; devportalBase by dpDo — so their callers name
// only the resource path (e.g. "/rest-apis").
var (
	platformAPIBase = envOr("PA_API_BASE", "/api/v0.9")
	devportalBase   = envOr("DP_API_BASE", "/api/v0.9")
)

// Additional platform-api path prefixes, distinct from the resource API version above
// (they carry their own version segment). Overridable for the same forward-compat reason.
var (
	portalAuthPath = envOr("PA_PORTAL_BASE", "/api/portal/v0.9") // username/password login
	// webhookReceiverPath is the platform-api webhook receiver, addressed by the
	// devportal at the container-internal host (see webhookReceiverURL).
	webhookReceiverPath = envOr("PA_WEBHOOK_BASE", "/api/internal/v0.9") + "/webhook/events"
)

// suite holds state established once for the whole run (BeforeSuite): the chosen
// database, the running stack, the admin token and the pre-registered gateways.
var suite struct {
	db          string // postgres | sqlite | sqlserver
	composeFile string
	multi       bool // second gateway available (postgres stack only)
	devportal   bool // developer portal + webhook wired (postgres stack only)
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
		// The second gateway and the developer portal are only wired on the
		// postgres stack, so their scenarios are skipped elsewhere.
		tags = "~@multigateway && ~@devportal"
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
		// The postgres stack is the one wired with the second gateway and the
		// developer portal (+ webhook), so @multigateway and @devportal run here.
		// The devportal service needs its own image, so only bring it up when the
		// @devportal scenario is actually selected (a tag subset may exclude it).
		suite.composeFile, suite.multi = "docker-compose.yaml", true
		suite.devportal = devportalSelected()
	case "sqlite":
		suite.composeFile = "docker-compose.sqlite.yaml"
	case "sqlserver":
		suite.composeFile = "docker-compose.sqlserver.yaml"
	default:
		return fmt.Errorf("unsupported E2E_DB %q", suite.db)
	}
	fmt.Printf("E2E database backend: %s (%s)\n", suite.db, suite.composeFile)

	// The postgres platform-api has the webhook receiver enabled and mounts the
	// private key at startup (phase 1), so make a container-readable copy and
	// export PA_WEBHOOK_KEY before bringing it up — regardless of whether the
	// @devportal scenario runs this time.
	if suite.db == "postgres" {
		if err := prepareWebhookKey(); err != nil {
			return err
		}
		// Give the admin JWT the dp:* scopes the developer portal enforces.
		if err := os.Setenv("AUTH_FILE_BASED_USERS", fileBasedAdminUsers); err != nil {
			return err
		}
	}

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
	if suite.devportal {
		dataPlane = append(dataPlane, "devportal")
	}

	// Phase 2: data plane (+ devportal) with the minted tokens.
	if err := compose(env, append([]string{"up", "-d"}, dataPlane...)...); err != nil {
		return fmt.Errorf("start data plane: %w", err)
	}

	// Bootstrap the developer portal so it can fire webhooks that platform-api
	// accepts: link its org to the control-plane org handle and register the
	// platform-api webhook subscriber.
	if suite.devportal {
		if err := bootstrapDevportal(); err != nil {
			return fmt.Errorf("bootstrap devportal: %w", err)
		}
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

// apiCall issues a JSON request to the platform-api resource API. path is the
// resource path relative to platformAPIBase (e.g. "/rest-apis"), which is prepended.
func apiCall(method, path, token string, body any) (int, []byte, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, platformAPI+platformAPIBase+path, rdr)
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
	req, err := http.NewRequest(http.MethodPost, platformAPI+portalAuthPath+"/auth/login",
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
	st, body, err := apiCall(http.MethodPost, "/projects", suite.token,
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
	st, body, err := apiCall(http.MethodPost, "/gateways", suite.token, map[string]any{
		"id": name, "displayName": name, "endpoints": []string{"http://" + ingressHost}, "functionalityType": "regular",
	})
	if err != nil {
		return "", "", err
	}
	gatewayID = jsonField(body, "id", "uuid")
	if st >= 300 || gatewayID == "" {
		return "", "", fmt.Errorf("create gateway failed (%d): %s", st, body)
	}
	st, body, err = apiCall(http.MethodPost, "/gateways/"+gatewayID+"/tokens", suite.token, map[string]any{})
	if err != nil {
		return "", "", err
	}
	token = jsonField(body, "token")
	if st >= 300 || token == "" {
		return "", "", fmt.Errorf("rotate token failed (%d): %s", st, body)
	}
	return gatewayID, token, nil
}

// devportalSelected reports whether the @devportal scenario will run, so the
// devportal service (which needs its own image) is only brought up when needed.
// A default run (no E2E_TAGS) includes it; an explicit tag subset includes it
// only when it selects @devportal and does not negate it.
func devportalSelected() bool {
	tags := os.Getenv("E2E_TAGS")
	if tags == "" {
		return true
	}
	return strings.Contains(tags, "@devportal") && !strings.Contains(tags, "~@devportal")
}

// prepareWebhookKey copies the repo's devportal webhook private key (mode 0600,
// owned by the host user) to a world-readable copy and points PA_WEBHOOK_KEY at
// it, so the platform-api container (uid 10001) can read the mounted key. The
// key is a non-secret dev fixture, so a 0644 copy is fine.
//
// The copy is written into the working directory (the compose-file dir, under
// the user's home), NOT os.TempDir(): the container runtime (e.g. colima) only
// shares the home tree into its VM, so a /tmp source would fail to bind-mount
// (docker would create an empty directory at the target instead).
// webhookPublicKeyPEM is the SPKI PEM of the RSA key pair generated by
// prepareWebhookKey; registerWebhookSubscriber gives it to the developer portal so it
// can encrypt key/token fields that platform-api decrypts with the matching private key.
var webhookPublicKeyPEM string

// prepareWebhookKey generates the RSA key pair used for the devportal↔platform-api
// hybrid encryption of webhook secret fields, writes the private key where the
// platform-api container mounts it (PA_WEBHOOK_KEY), and stores the public key for the
// subscriber registration. The pair is generated per run rather than read from the
// repo, since the private key is gitignored (and absent in CI); any matched pair works.
//
// The private key is written under the compose directory (not os.TempDir) because the
// container runtime only shares the working tree into its VM, and 0644 so the
// container user (uid 10001) can read the read-only mount.
func prepareWebhookKey() error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate webhook key pair: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	dst := filepath.Join(cwd, ".webhook-key.it.pem")
	if err := os.WriteFile(dst, privPEM, 0o644); err != nil {
		return fmt.Errorf("write webhook private key: %w", err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return fmt.Errorf("marshal webhook public key: %w", err)
	}
	webhookPublicKeyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))

	return os.Setenv("PA_WEBHOOK_KEY", dst)
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


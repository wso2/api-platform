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

package e2e

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// world holds the per-scenario state: the API created for the scenario and the
// deployment ids returned when it is deployed to each gateway. Some restart
// scenarios use a second API to prove a deployment made while the gateway is
// down is picked up on the next controller start.
type world struct {
	apiID      string
	apiContext string // e.g. /e2e-ab12cd34
	depGw1     string
	depGw2     string

	api2ID      string // optional second API (deploy-while-down scenario)
	api2Context string
	dep2Gw1     string

	secretID    string // optional secret (secret-resolution scenarios)
	secretValue string

	llmHandle  string // optional LLM provider (llm-provider secret scenarios)
	llmContext string

	apiDisplayName string // tracked so lifecycle scenarios can PUT-update the API

	apiKeyValue string // caller-supplied API key (api-key auth scenario)

	mcpHandle  string // MCP proxy (mcp scenario)
	mcpContext string

	llmProxyHandle  string // LLM proxy (llm-proxy @wip scenario)
	llmProxyContext string
}

// lifecycleVersionHeader is the request header the lifecycle scenarios inject with
// a version marker (v1/v2) so update propagation is observable via the echo.
const lifecycleVersionHeader = "X-Api-Version"

// apiKeyHeaderName is the request header the API-key auth scenario configures and
// sends the key in.
const apiKeyHeaderName = "apikey"

// secretHeaderName is the request header the set-headers policy injects with the
// resolved secret value; the sample backend echoes it back for assertion.
const secretHeaderName = "X-Auth-Token"

// initializeScenario is invoked by godog for each scenario; it binds a fresh
// world so scenarios do not share API/deployment state.
func initializeScenario(sc *godog.ScenarioContext) {
	w := &world{}

	sc.Step(`^the platform-api control plane and gateway data plane are running$`, w.stackRunning)
	sc.Step(`^I am authenticated to platform-api$`, w.authenticated)
	sc.Step(`^a REST API routed to the sample backend$`, w.aRestAPI)

	sc.Step(`^I deploy the API to the gateway$`, w.deployToGateway)
	sc.Step(`^the API is deployed to the gateway and served$`, w.deployedAndServed)
	sc.Step(`^I undeploy the API from the gateway$`, w.undeployFromGateway)
	sc.Step(`^the gateway serves the API$`, w.gatewayServes)
	sc.Step(`^the gateway stops serving the API$`, w.gatewayStopsServing)
	sc.Step(`^the gateway still serves the API$`, w.gatewayStillServes)
	sc.Step(`^a request to a path outside the API context returns 404$`, w.unmappedPathReturns404)

	sc.Step(`^I deploy the API to the second gateway$`, w.deployToSecondGateway)
	sc.Step(`^the second gateway serves the API$`, w.secondGatewayServes)
	sc.Step(`^I undeploy the API from the second gateway$`, w.undeployFromSecondGateway)
	sc.Step(`^the second gateway stops serving the API$`, w.secondGatewayStopsServing)

	// Restart / recovery steps.
	sc.Step(`^I restart the gateway controller$`, w.restartController)
	sc.Step(`^I restart the gateway runtime$`, w.restartRuntime)
	sc.Step(`^I restart the whole gateway$`, w.restartWholeGateway)
	sc.Step(`^the gateway controller is stopped$`, w.stopController)
	sc.Step(`^the gateway controller is started$`, w.startController)
	sc.Step(`^the gateway store is wiped and the gateway controller is restarted$`, w.wipeStoreAndRestartController)

	sc.Step(`^a second REST API routed to the sample backend$`, w.aSecondRestAPI)
	sc.Step(`^I deploy the second API to the gateway while it is stopped$`, w.deploySecondWhileStopped)
	sc.Step(`^I undeploy the API from the gateway while it is stopped$`, w.undeployFromGateway)
	sc.Step(`^the gateway serves the second API$`, w.gatewayServesSecond)

	sc.Step(`^the API is deployed to the second gateway and served$`, w.deployedToSecondAndServed)
	sc.Step(`^the second gateway still serves the API$`, w.secondGatewayStillServes)

	// Secret-resolution steps.
	sc.Step(`^a secret in platform-api$`, w.aSecret)
	sc.Step(`^a REST API that injects the secret into an upstream request header$`, w.aRestAPIWithSecretHeader)
	sc.Step(`^the gateway injects the resolved secret value into the upstream request$`, w.secretResolvedUpstream)

	// LLM-provider secret steps.
	sc.Step(`^an LLM provider whose upstream authorization uses the secret$`, w.anLLMProviderWithSecret)
	sc.Step(`^I deploy the LLM provider to the gateway$`, w.deployLLMToGateway)
	sc.Step(`^invoking the LLM provider sends the resolved secret upstream$`, w.llmInvokeResolvedSecret)

	// Control-plane (platform-api) restart steps.
	sc.Step(`^I restart the platform-api control plane$`, w.restartPlatformAPI)
	sc.Step(`^the control plane accepts requests again$`, w.controlPlaneAcceptsRequests)
	sc.Step(`^I deploy the second API to the gateway$`, w.deploySecond)

	// Lifecycle steps (update-redeploy, delete, restore).
	sc.Step(`^a REST API that injects the version header "([^"]*)"$`, w.aRestAPIWithVersionHeader)
	sc.Step(`^the gateway injects the version header "([^"]*)"$`, w.gatewayInjectsVersionHeader)
	sc.Step(`^I update the API to inject the version header "([^"]*)" and redeploy$`, w.updateVersionHeaderAndRedeploy)
	sc.Step(`^I delete the API from platform-api$`, w.deleteAPI)
	sc.Step(`^I restore the deployment$`, w.restoreDeploymentStep)

	// API-key auth steps.
	sc.Step(`^a REST API that requires an API key$`, w.aRestAPIRequiringKey)
	sc.Step(`^a request without an API key is rejected$`, w.requestWithoutKeyRejected)
	sc.Step(`^I generate an API key for the API$`, w.generateAPIKey)
	sc.Step(`^a request with the API key is accepted$`, w.requestWithKeyAccepted)

	// MCP proxy steps.
	sc.Step(`^the MCP backend is running$`, w.mcpBackendRunning)
	sc.Step(`^an MCP proxy routed to the MCP backend$`, w.anMCPProxy)
	sc.Step(`^I deploy the MCP proxy to the gateway$`, w.deployMCPToGateway)
	sc.Step(`^the gateway serves the MCP proxy$`, w.gatewayServesMCP)

	// LLM-proxy steps (@wip — blocked by the provider template-apiVersion bug).
	sc.Step(`^an LLM proxy over that provider$`, w.anLLMProxyOverProvider)
	sc.Step(`^I deploy the LLM proxy to the gateway$`, w.deployLLMProxyToGateway)
	sc.Step(`^invoking the LLM proxy sends the resolved secret upstream$`, w.llmProxyInvokeResolvedSecret)
}

// --- Background steps ------------------------------------------------------

func (w *world) stackRunning() error {
	if suite.gw1ID == "" {
		return fmt.Errorf("gateway was not registered during suite setup")
	}
	return nil
}

func (w *world) authenticated() error {
	if suite.token == "" {
		return fmt.Errorf("not authenticated to platform-api")
	}
	return nil
}

// --- Given -----------------------------------------------------------------

// createRestAPI creates a REST API in platform-api routed to the sample backend
// and returns its id and context path. The name/context must be URL-friendly
// (no slash); the context doubles as the ingress path the data plane serves.
func createRestAPI() (id, context string, err error) {
	suffix := randHex()
	context = "/e2e-" + suffix
	st, body, err := apiCall(http.MethodPost, "/api/v0.9/rest-apis", suite.token, map[string]any{
		"displayName": "e2e-api-" + suffix,
		"context":     context,
		"version":     "v1",
		"projectId":   suite.projectID,
		"upstream":    map[string]any{"main": map[string]any{"url": "http://sample-backend:9080"}},
	})
	if err != nil {
		return "", "", err
	}
	id = jsonField(body, "id", "handle", "uuid")
	if st >= 300 || id == "" {
		return "", "", fmt.Errorf("create API failed (%d): %s", st, body)
	}
	return id, context, nil
}

func (w *world) aRestAPI() error {
	id, context, err := createRestAPI()
	if err != nil {
		return err
	}
	w.apiID, w.apiContext = id, context
	return nil
}

func (w *world) aSecondRestAPI() error {
	id, context, err := createRestAPI()
	if err != nil {
		return err
	}
	w.api2ID, w.api2Context = id, context
	return nil
}

func (w *world) aSecret() error {
	w.secretID = "e2e-sec-" + randHex()
	w.secretValue = "e2e-secret-value-" + randHex()
	return createSecret(w.secretID, w.secretValue)
}

func (w *world) aRestAPIWithSecretHeader() error {
	id, context, err := createRestAPIWithSecretHeader(w.secretID, secretHeaderName)
	if err != nil {
		return err
	}
	w.apiID, w.apiContext = id, context
	return nil
}

func (w *world) anLLMProviderWithSecret() error {
	handle, context, err := createLLMProviderWithSecretAuth(w.secretID)
	if err != nil {
		return err
	}
	w.llmHandle, w.llmContext = handle, context
	return nil
}

func (w *world) deployLLMToGateway() error {
	_, err := deployLLMProvider(w.llmHandle, suite.gw1ID)
	return err
}

func (w *world) llmInvokeResolvedSecret() error {
	// The LLM provider injects its credential into the Authorization header
	// (upstream.main.auth.header), not the set-headers X-Auth-Token used by the
	// REST scenario.
	return waitIngressPostEchoHeader(ingressGw1, w.llmContext+"/chat/completions",
		"Authorization", "Bearer "+w.secretValue)
}

func (w *world) deployedAndServed() error {
	if err := w.deployToGateway(); err != nil {
		return err
	}
	return w.gatewayServes()
}

// --- deploy / undeploy -----------------------------------------------------

// deployNoBounce attaches the gateway to the API and creates a deployment via
// the platform-api REST API, without touching the gateway controller. It is the
// pure control-plane half of a deployment. If the controller is connected it
// picks the deployment up from the live `api.deployed` event; if it is stopped it
// picks it up from the connect-time full sync on its next start. Returns the
// deployment id.
func deployNoBounce(apiID, gatewayID string) (string, error) {
	if st, body, err := apiCall(http.MethodPost, "/api/v0.9/rest-apis/"+apiID+"/gateways", suite.token,
		[]map[string]string{{"gatewayId": gatewayID}}); err != nil {
		return "", err
	} else if st >= 300 {
		return "", fmt.Errorf("attach gateway failed (%d): %s", st, body)
	}
	st, body, err := apiCall(http.MethodPost, "/api/v0.9/rest-apis/"+apiID+"/deployments", suite.token,
		map[string]any{"base": "current", "gatewayId": gatewayID, "name": "dep-" + randHex()})
	if err != nil {
		return "", err
	}
	id := jsonField(body, "deploymentId")
	if st >= 300 || id == "" {
		return "", fmt.Errorf("deploy failed (%d): %s", st, body)
	}
	return id, nil
}

// While the controller is connected, platform-api pushes an `api.deployed` event
// down the control-plane socket and the controller deploys that single API to the
// data plane live (handleAPIDeployedEvent in pkg/controlplane/client.go) — no
// restart needed. (The connect-time full sync, c.syncOnce, still runs only once;
// it is what the deploy-while-stopped and empty-store recovery scenarios rely on.)
// So deployToGateway just creates the deployment via deployNoBounce and lets the
// live event propagate it; the assertion step polls the ingress until it serves.

func undeploy(apiID, deploymentID, gatewayID string) error {
	st, body, err := apiCall(http.MethodPost,
		"/api/v0.9/rest-apis/"+apiID+"/deployments/"+deploymentID+"/undeploy?gatewayId="+gatewayID, suite.token, nil)
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("undeploy failed (%d): %s", st, body)
	}
	return nil
}

func (w *world) deployToGateway() error {
	id, err := deployNoBounce(w.apiID, suite.gw1ID)
	if err != nil {
		return err
	}
	w.depGw1 = id
	return nil
}

func (w *world) deployToSecondGateway() error {
	id, err := deployNoBounce(w.apiID, suite.gw2ID)
	if err != nil {
		return err
	}
	w.depGw2 = id
	return nil
}

func (w *world) undeployFromGateway() error       { return undeploy(w.apiID, w.depGw1, suite.gw1ID) }
func (w *world) undeployFromSecondGateway() error { return undeploy(w.apiID, w.depGw2, suite.gw2ID) }

func (w *world) deployedToSecondAndServed() error {
	if err := w.deployToSecondGateway(); err != nil {
		return err
	}
	return w.secondGatewayServes()
}

// deploySecondWhileStopped creates the second API's deployment against gw1 via
// the control plane only. The controller is stopped at this point, so it will
// observe the deployment on its next start (connect-time sync), which is exactly
// what the "deploy while the gateway is down" scenario verifies.
func (w *world) deploySecondWhileStopped() error {
	id, err := deployNoBounce(w.api2ID, suite.gw1ID)
	if err != nil {
		return err
	}
	w.dep2Gw1 = id
	return nil
}

// --- restart / recovery steps ----------------------------------------------

func (w *world) restartController() error { return compose(nil, "restart", "gateway-controller") }
func (w *world) restartRuntime() error    { return compose(nil, "restart", "gateway-runtime") }
func (w *world) stopController() error    { return compose(nil, "stop", "gateway-controller") }
func (w *world) startController() error   { return compose(nil, "start", "gateway-controller") }

func (w *world) restartWholeGateway() error {
	return compose(nil, "restart", "gateway-controller", "gateway-runtime")
}

// restartPlatformAPI bounces the control plane and waits for it to become healthy
// again. The gateway is untouched: its data plane must keep serving while the CP
// is down, and its controller must auto-reconnect the control-plane socket once
// the CP is back.
func (w *world) restartPlatformAPI() error {
	if err := compose(nil, "restart", "platform-api"); err != nil {
		return err
	}
	return waitHealthy()
}

// controlPlaneAcceptsRequests proves the restarted control plane is functional by
// re-authenticating against it.
func (w *world) controlPlaneAcceptsRequests() error {
	if _, err := login(); err != nil {
		return fmt.Errorf("control plane did not accept requests after restart: %w", err)
	}
	return nil
}

// deploySecond deploys the already-created second API to the first gateway via the
// control plane only (no controller restart), so a connected — or freshly
// reconnected — controller picks it up from the live api.deployed event.
func (w *world) deploySecond() error {
	id, err := deployNoBounce(w.api2ID, suite.gw1ID)
	if err != nil {
		return err
	}
	w.dep2Gw1 = id
	return nil
}

// --- lifecycle steps (update-redeploy, delete, restore) --------------------

func (w *world) aRestAPIWithVersionHeader(value string) error {
	id, context, displayName, err := createRestAPIWithHeader(lifecycleVersionHeader, value)
	if err != nil {
		return err
	}
	w.apiID, w.apiContext, w.apiDisplayName = id, context, displayName
	return nil
}

func (w *world) gatewayInjectsVersionHeader(value string) error {
	return waitIngressEchoHeader(ingressGw1, w.apiContext, lifecycleVersionHeader, value)
}

func (w *world) updateVersionHeaderAndRedeploy(value string) error {
	if err := updateRestAPIHeader(w.apiID, w.apiDisplayName, w.apiContext, lifecycleVersionHeader, value); err != nil {
		return err
	}
	id, err := deployNoBounce(w.apiID, suite.gw1ID)
	if err != nil {
		return err
	}
	w.depGw1 = id
	return nil
}

func (w *world) deleteAPI() error { return deleteRestAPI(w.apiID) }

func (w *world) restoreDeploymentStep() error {
	return restoreDeployment(w.apiID, w.depGw1, suite.gw1ID)
}

// --- API-key auth steps ----------------------------------------------------

func (w *world) aRestAPIRequiringKey() error {
	id, context, err := createRestAPIRequiringKey(apiKeyHeaderName)
	if err != nil {
		return err
	}
	w.apiID, w.apiContext = id, context
	return nil
}

// requestWithoutKeyRejected waits until the ingress serves the route but rejects
// an unauthenticated request (401/403). It tolerates transient 404s while the
// deployment propagates.
func (w *world) requestWithoutKeyRejected() error {
	deadline := time.Now().Add(pollTimeout)
	var last int
	for time.Now().Before(deadline) {
		if last = ingressStatusWithHeader(ingressGw1, w.apiContext, "", ""); last == 401 || last == 403 {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("request without API key: wanted 401/403, last observed %d", last)
}

func (w *world) generateAPIKey() error {
	w.apiKeyValue = "e2e-apikey-" + randHex()
	return createAPIKeyForAPI(w.apiID, w.apiKeyValue)
}

func (w *world) requestWithKeyAccepted() error {
	deadline := time.Now().Add(pollTimeout)
	var last int
	for time.Now().Before(deadline) {
		if last = ingressStatusWithHeader(ingressGw1, w.apiContext, apiKeyHeaderName, w.apiKeyValue); last == 200 {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("request with API key: wanted 200, last observed %d", last)
}

// --- MCP proxy steps -------------------------------------------------------

// mcpInitBody is a minimal MCP JSON-RPC initialize request.
const mcpInitBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":` +
	`{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"e2e","version":"1.0.0"}}}`

func (w *world) mcpBackendRunning() error {
	// Started on demand (not part of the default phase-1/2 bring-up).
	return compose(nil, "up", "-d", "mcp-backend")
}

func (w *world) anMCPProxy() error {
	handle, context, err := createMCPProxy()
	if err != nil {
		return err
	}
	w.mcpHandle, w.mcpContext = handle, context
	return nil
}

func (w *world) deployMCPToGateway() error {
	_, err := deployMCPProxy(w.mcpHandle, suite.gw1ID)
	return err
}

// gatewayServesMCP polls the MCP endpoint (<context>/mcp) with a JSON-RPC
// initialize until the gateway routes it to the MCP backend and returns a
// successful JSON-RPC result.
func (w *world) gatewayServesMCP() error {
	deadline := time.Now().Add(pollTimeout)
	var lastCode int
	var lastBody string
	for time.Now().Before(deadline) {
		lastCode, lastBody = mcpInitialize(ingressGw1, w.mcpContext)
		if lastCode == 200 && strings.Contains(lastBody, `"result"`) {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("MCP initialize %s%s/mcp: wanted 200 with a result, last %d: %.200s",
		ingressGw1, w.mcpContext, lastCode, lastBody)
}

// --- LLM-proxy steps (@wip — blocked by provider template-apiVersion bug) --

func (w *world) anLLMProxyOverProvider() error {
	handle, context, err := createLLMProxy(w.llmHandle)
	if err != nil {
		return err
	}
	w.llmProxyHandle, w.llmProxyContext = handle, context
	return nil
}

func (w *world) deployLLMProxyToGateway() error {
	_, err := deployLLMProxy(w.llmProxyHandle, suite.gw1ID)
	return err
}

func (w *world) llmProxyInvokeResolvedSecret() error {
	return waitIngressPostEchoHeader(ingressGw1, w.llmProxyContext+"/chat/completions",
		"Authorization", "Bearer "+w.secretValue)
}

// mcpInitialize POSTs an MCP initialize request through the ingress and returns
// the status and body (the MCP response may be JSON or an SSE data frame; both
// carry the JSON-RPC "result").
func mcpInitialize(base, context string) (int, string) {
	req, err := http.NewRequest(http.MethodPost, base+context+"/mcp", strings.NewReader(mcpInitBody))
	if err != nil {
		return -1, ""
	}
	req.Host = ingressHost
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// wipeStoreAndRestartController simulates a fresh/replaced gateway: the
// controller is stopped, its local artifact store is wiped, and it is started
// again. On start it re-runs its connect-time sync and must re-fetch every
// deployed artifact from the control plane, proving the control plane is the
// source of truth for recovery.
func (w *world) wipeStoreAndRestartController() error {
	if err := w.stopController(); err != nil {
		return err
	}
	if err := wipeGatewayStore(); err != nil {
		return err
	}
	return w.startController()
}

// --- Then (data-plane assertions) ------------------------------------------

func (w *world) gatewayServes() error       { return waitIngress(ingressGw1, w.apiContext, 200) }
func (w *world) gatewayStopsServing() error { return waitIngress(ingressGw1, w.apiContext, 404) }
func (w *world) gatewayServesSecond() error { return waitIngress(ingressGw1, w.api2Context, 200) }
func (w *world) secondGatewayServes() error { return waitIngress(ingressGw2, w.apiContext, 200) }
func (w *world) secondGatewayStopsServing() error {
	return waitIngress(ingressGw2, w.apiContext, 404)
}

func (w *world) gatewayStillServes() error {
	if code := ingressStatus(ingressGw1, w.apiContext); code != 200 {
		return fmt.Errorf("gateway 1 should still serve the API, got %d", code)
	}
	return nil
}

func (w *world) secondGatewayStillServes() error {
	if code := ingressStatus(ingressGw2, w.apiContext); code != 200 {
		return fmt.Errorf("gateway 2 should still serve the API, got %d", code)
	}
	return nil
}

func (w *world) unmappedPathReturns404() error {
	if code := ingressStatus(ingressGw1, "/no-such-"+randHex()); code != 404 {
		return fmt.Errorf("unmapped path should return 404, got %d", code)
	}
	return nil
}

// wipeGatewayStore empties the first gateway controller's local artifact store
// so a subsequent start must recover everything from the control plane. Only the
// postgres stack is exercised for restart scenarios, so this truncates every
// table in gw1's store database (gateway_test); gw2 uses a separate database
// (gateway_test2) and is left untouched. The controller must be stopped first so
// no rows are being written during the truncate.
func wipeGatewayStore() error {
	if suite.db != "postgres" {
		return fmt.Errorf("wipeGatewayStore only implemented for postgres, got %q", suite.db)
	}
	const truncateAll = `DO $$ DECLARE r RECORD; BEGIN ` +
		`FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP ` +
		`EXECUTE 'TRUNCATE TABLE public.' || quote_ident(r.tablename) || ' CASCADE'; ` +
		`END LOOP; END $$;`
	return compose(nil, "exec", "-T", "postgres",
		"psql", "-v", "ON_ERROR_STOP=1", "-U", "apip", "-d", "gateway_test", "-c", truncateAll)
}

func (w *world) secretResolvedUpstream() error {
	return waitIngressEchoHeader(ingressGw1, w.apiContext, secretHeaderName, "Bearer "+w.secretValue)
}

// --- ingress helpers -------------------------------------------------------

// llmChatBody is a minimal OpenAI chat/completions request the openai provider
// template accepts, used to drive an LLM provider through the gateway.
const llmChatBody = `{"model":"gpt-4","messages":[{"role":"user","content":"e2e"}]}`

// echoedHeaderFrom sends req through the ingress and returns the value the sample
// backend echoed back for the named request header (the backend reflects the
// request it received as JSON: {"headers": {"X-Auth-Token": ["..."]}, ...}).
// Returns "" if the request is not served (non-200), the body is not the echo
// JSON, or the header is absent.
func echoedHeaderFrom(req *http.Request, header string) string {
	req.Host = ingressHost // gateway routes by vhost
	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	var echo struct {
		Headers map[string][]string `json:"headers"`
	}
	if json.Unmarshal(body, &echo) != nil {
		return ""
	}
	return strings.Join(echo.Headers[header], ",")
}

// echoedHeader does a GET to base+context+"/" and returns the echoed header value.
func echoedHeader(base, context, header string) string {
	req, err := http.NewRequest(http.MethodGet, base+context+"/", nil)
	if err != nil {
		return ""
	}
	return echoedHeaderFrom(req, header)
}

// postEchoedHeader does a POST of the LLM chat body to base+path and returns the
// echoed header value.
func postEchoedHeader(base, path, header string) string {
	req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(llmChatBody))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	return echoedHeaderFrom(req, header)
}

// waitIngressEchoHeader polls the ingress (GET) until the sample backend echoes
// the named header containing want (proving the gateway injected and resolved
// it), or times out.
func waitIngressEchoHeader(base, context, header, want string) error {
	return pollEchoedHeader(func() string { return echoedHeader(base, context, header) },
		fmt.Sprintf("GET %s%s/", base, context), header, want)
}

// waitIngressPostEchoHeader polls the ingress (POST to path) until the echoed
// header contains want, or times out.
func waitIngressPostEchoHeader(base, path, header, want string) error {
	return pollEchoedHeader(func() string { return postEchoedHeader(base, path, header) },
		fmt.Sprintf("POST %s%s", base, path), header, want)
}

func pollEchoedHeader(fetch func() string, what, header, want string) error {
	deadline := time.Now().Add(pollTimeout)
	var last string
	for time.Now().Before(deadline) {
		if last = fetch(); strings.Contains(last, want) {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("ingress %s header %q: wanted to contain %q, last observed %q",
		what, header, want, last)
}

func ingressStatus(base, context string) int {
	return ingressStatusWithHeader(base, context, "", "")
}

// ingressStatusWithHeader does a GET base+context+"/" and returns the status,
// optionally setting a request header (used for API-key auth checks).
func ingressStatusWithHeader(base, context, header, value string) int {
	req, err := http.NewRequest(http.MethodGet, base+context+"/", nil)
	if err != nil {
		return -1
	}
	req.Host = ingressHost // gateway routes by vhost
	if header != "" {
		req.Header.Set(header, value)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}

// waitIngress polls the gateway ingress until it returns want (or times out).
func waitIngress(base, context string, want int) error {
	deadline := time.Now().Add(pollTimeout)
	var last int
	for time.Now().Before(deadline) {
		if last = ingressStatus(base, context); last == want {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("ingress %s%s: wanted %d, last observed %d", base, context, want, last)
}

func randHex() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

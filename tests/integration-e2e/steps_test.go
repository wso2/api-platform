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
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"
)

// world holds the per-scenario state: the API created for the scenario and the
// deployment ids returned when it is deployed to each gateway.
type world struct {
	apiID      string
	apiContext string // e.g. /e2e-ab12cd34
	depGw1     string
	depGw2     string
}

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

func (w *world) aRestAPI() error {
	// Name/displayName must be URL-friendly (no slash); the context is the path.
	suffix := randHex()
	w.apiContext = "/e2e-" + suffix
	st, body, err := apiCall(http.MethodPost, "/api/v1/rest-apis", suite.token, map[string]any{
		"name":      "e2e-api-" + suffix,
		"context":   w.apiContext,
		"version":   "v1",
		"projectId": suite.projectID,
		"upstream":  map[string]any{"main": map[string]any{"url": "http://sample-backend:9080"}},
	})
	if err != nil {
		return err
	}
	w.apiID = jsonField(body, "id", "handle", "uuid")
	if st >= 300 || w.apiID == "" {
		return fmt.Errorf("create API failed (%d): %s", st, body)
	}
	return nil
}

func (w *world) deployedAndServed() error {
	if err := w.deployToGateway(); err != nil {
		return err
	}
	return w.gatewayServes()
}

// --- deploy / undeploy -----------------------------------------------------

// deploy attaches the gateway to the API, creates a deployment and bounces the
// gateway's controller so it picks the deployment up.
//
// The controller runs a full deployment sync only once, on first connect
// (c.syncOnce in pkg/controlplane/client.go); deployments created while it is
// already connected are not full-synced. Restarting the controller process
// re-runs that one-time sync, which is the data-plane equivalent of "the
// controller noticed the new deployment". Returns the deployment id.
func deploy(apiID, gatewayID, controllerService string) (string, error) {
	if st, body, err := apiCall(http.MethodPost, "/api/v1/rest-apis/"+apiID+"/gateways", suite.token,
		[]map[string]string{{"gatewayId": gatewayID}}); err != nil {
		return "", err
	} else if st >= 300 {
		return "", fmt.Errorf("attach gateway failed (%d): %s", st, body)
	}
	st, body, err := apiCall(http.MethodPost, "/api/v1/rest-apis/"+apiID+"/deployments", suite.token,
		map[string]any{"base": "current", "gatewayId": gatewayID, "name": "dep-" + randHex()})
	if err != nil {
		return "", err
	}
	id := jsonField(body, "deploymentId")
	if st >= 300 || id == "" {
		return "", fmt.Errorf("deploy failed (%d): %s", st, body)
	}
	if err := compose(nil, "restart", controllerService); err != nil {
		return "", fmt.Errorf("restart %s: %w", controllerService, err)
	}
	return id, nil
}

func undeploy(apiID, deploymentID, gatewayID string) error {
	st, body, err := apiCall(http.MethodPost,
		"/api/v1/rest-apis/"+apiID+"/deployments/"+deploymentID+"/undeploy?gatewayId="+gatewayID, suite.token, nil)
	if err != nil {
		return err
	}
	if st >= 300 {
		return fmt.Errorf("undeploy failed (%d): %s", st, body)
	}
	return nil
}

func (w *world) deployToGateway() error {
	id, err := deploy(w.apiID, suite.gw1ID, "gateway-controller")
	if err != nil {
		return err
	}
	w.depGw1 = id
	return nil
}

func (w *world) deployToSecondGateway() error {
	id, err := deploy(w.apiID, suite.gw2ID, "gateway-controller-2")
	if err != nil {
		return err
	}
	w.depGw2 = id
	return nil
}

func (w *world) undeployFromGateway() error       { return undeploy(w.apiID, w.depGw1, suite.gw1ID) }
func (w *world) undeployFromSecondGateway() error { return undeploy(w.apiID, w.depGw2, suite.gw2ID) }

// --- Then (data-plane assertions) ------------------------------------------

func (w *world) gatewayServes() error          { return waitIngress(ingressGw1, w.apiContext, 200) }
func (w *world) gatewayStopsServing() error    { return waitIngress(ingressGw1, w.apiContext, 404) }
func (w *world) secondGatewayServes() error    { return waitIngress(ingressGw2, w.apiContext, 200) }
func (w *world) secondGatewayStopsServing() error {
	return waitIngress(ingressGw2, w.apiContext, 404)
}

func (w *world) gatewayStillServes() error {
	if code := ingressStatus(ingressGw1, w.apiContext); code != 200 {
		return fmt.Errorf("gateway 1 should still serve the API, got %d", code)
	}
	return nil
}

func (w *world) unmappedPathReturns404() error {
	if code := ingressStatus(ingressGw1, "/no-such-"+randHex()); code != 404 {
		return fmt.Errorf("unmapped path should return 404, got %d", code)
	}
	return nil
}

// --- ingress helpers -------------------------------------------------------

func ingressStatus(base, context string) int {
	req, err := http.NewRequest(http.MethodGet, base+context+"/", nil)
	if err != nil {
		return -1
	}
	req.Host = ingressHost // gateway routes by vhost
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

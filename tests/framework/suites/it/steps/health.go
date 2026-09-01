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

// Health steps: the three liveness surfaces a gateway exposes, addressed separately.
//
// Three processes, three endpoints, and they can disagree — a controller that answers while
// the router is not accepting traffic is precisely the state worth catching. The legacy suite
// reached them at fixed host ports; here each is resolved from the running topology, because
// the ports are published dynamically so blocks can run concurrently.

package steps

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// keyHealthResults carries the per-service outcome from the check step to the assertion step.
const keyHealthResults = "health.results"

func (g *Gateway) registerHealthSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I send a GET request to the gateway controller admin health endpoint$`,
		g.getControllerHealth)
	sc.Step(`^I send a GET request to the router ready endpoint$`, g.getRouterReady)
	sc.Step(`^I send a GET request to the policy engine health endpoint$`, g.getPolicyEngineHealth)
	sc.Step(`^the response should indicate healthy status$`, g.responseIndicatesHealthy)
	sc.Step(`^I check the health of all gateway services$`, g.checkAllHealth)
	sc.Step(`^all services should report healthy status$`, g.allServicesHealthy)
}

// healthTargets resolves the three endpoints against the running topology.
//
// Built per call rather than cached: a scenario may stop and start a service, and a URL
// captured earlier would point at a port docker has since reassigned.
func (g *Gateway) healthTargets() (map[string]string, error) {
	controller, err := g.adminURL("/health")
	if err != nil {
		return nil, err
	}
	// The ROUTER's readiness is Envoy's own /ready, not a product endpoint: it reports whether
	// the listeners are programmed and accepting traffic, which is the thing a caller depends
	// on and which the controller's health cannot speak for.
	envoy, err := g.topo.URL("platform-gateway", "envoy-admin")
	if err != nil {
		return nil, err
	}
	engine, err := g.topo.URL("platform-gateway", "policy-admin")
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"gateway-controller": controller,
		"router":             envoy + "/ready",
		"policy-engine":      engine + "/health",
	}, nil
}

// get performs a health request and publishes it for the shared assertion steps.
func (g *Gateway) getHealth(ctx context.Context, service string) error {
	targets, err := g.healthTargets()
	if err != nil {
		return err
	}
	resp, err := g.funnel.Get(ctx, targets[service], nil)
	if err != nil {
		return fmt.Errorf("requesting %s health: %w", service, err)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, resp)
}

func (g *Gateway) getControllerHealth(ctx context.Context) error {
	return g.getHealth(ctx, "gateway-controller")
}

func (g *Gateway) getRouterReady(ctx context.Context) error {
	return g.getHealth(ctx, "router")
}

func (g *Gateway) getPolicyEngineHealth(ctx context.Context) error {
	return g.getHealth(ctx, "policy-engine")
}

// responseIndicatesHealthy asserts the BODY says healthy, not merely that the status was 200.
//
// Deliberately separate from the status assertion the scenarios already make alongside it: a
// health endpoint that returns 200 with a body reporting a degraded component is a real state,
// and one this would catch while a status check alone would not.
//
// Both spellings are accepted because the three endpoints do not agree: Envoy's /ready answers
// with the plain word LIVE, while the product endpoints report JSON.
func (g *Gateway) responseIndicatesHealthy(ctx context.Context) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	body := strings.ToLower(resp.Text())
	for _, want := range []string{"ok", "healthy", "live"} {
		if strings.Contains(body, want) {
			return nil
		}
	}
	return fmt.Errorf("the response does not indicate healthy status: %s", resp.Describe())
}

// checkAllHealth probes every service and records the outcome per service.
//
// A transport error and a non-200 are recorded the same way, as unhealthy. The distinction
// matters when debugging but not to the claim under test, and the assertion step names which
// service failed either way.
func (g *Gateway) checkAllHealth(ctx context.Context) error {
	targets, err := g.healthTargets()
	if err != nil {
		return err
	}
	results := map[string]bool{}
	for name, url := range targets {
		resp, err := g.funnel.Get(ctx, url, nil)
		results[name] = err == nil && resp.Succeeded()
	}
	return tcontext.Set(ctx, keyHealthResults, results)
}

func (g *Gateway) allServicesHealthy(ctx context.Context) error {
	v, ok := tcontext.Get(ctx, keyHealthResults)
	if !ok {
		return fmt.Errorf("no health check has been performed in this scenario")
	}
	results, ok := v.(map[string]bool)
	if !ok {
		return fmt.Errorf("health results are stored as %T", v)
	}

	var unhealthy []string
	for name, healthy := range results {
		if !healthy {
			unhealthy = append(unhealthy, name)
		}
	}
	if len(unhealthy) == 0 {
		return nil
	}
	// Sorted so a failure message is stable across runs — map iteration order is random, and
	// an unstable message reads as a different failure each time.
	sort.Strings(unhealthy)
	return fmt.Errorf("these gateway services are not healthy: %s", strings.Join(unhealthy, ", "))
}

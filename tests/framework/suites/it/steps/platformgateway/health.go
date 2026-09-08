/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package platformgateway contains platform-gateway-specific integration steps.
package platformgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

const healthResultsKey = "health.results"

// Steps provides health checks for the platform gateway services.
type Steps struct {
	topo   *runtime.Topology
	funnel *httpx.Funnel
}

func (s *Steps) healthTargets() (map[string]string, error) {
	controller, err := s.topo.URL("platform-gateway", "admin")
	if err != nil {
		return nil, err
	}
	envoy, err := s.topo.URL("platform-gateway", "envoy-admin")
	if err != nil {
		return nil, err
	}
	engine, err := s.topo.URL("platform-gateway", "policy-admin")
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"gateway-controller": controller + "/api/admin/v1/health",
		"router":             envoy + "/ready",
		"policy-engine":      engine + "/health",
	}, nil
}

func (s *Steps) getHealth(ctx context.Context, service string) error {
	targets, err := s.healthTargets()
	if err != nil {
		return err
	}
	target, ok := targets[service]
	if !ok {
		return fmt.Errorf("unknown gateway health service %q", service)
	}
	resp, err := s.funnel.Get(ctx, target, nil)
	if err != nil {
		return fmt.Errorf("requesting %s health: %w", service, err)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, resp)
}

func (s *Steps) controllerHealth(ctx context.Context) error {
	return s.getHealth(ctx, "gateway-controller")
}

func (s *Steps) waitForController(ctx context.Context) error {
	targets, err := s.healthTargets()
	if err != nil {
		return err
	}
	target := targets["gateway-controller"]
	return stepscommon.AwaitResponse(ctx,
		func(ctx context.Context) (*httpx.Response, error) {
			resp, requestErr := s.funnel.Get(ctx, target, nil)
			if requestErr != nil {
				return nil, retry.Transient(requestErr)
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool {
			healthy, _ := responseIsHealthy("gateway-controller", resp)
			return healthy
		},
		"waiting for gateway controller health")
}

func (s *Steps) routerReady(ctx context.Context) error {
	return s.getHealth(ctx, "router")
}

func (s *Steps) routerReadyUntil(ctx context.Context, status int) error {
	targets, err := s.healthTargets()
	if err != nil {
		return err
	}
	target := targets["router"]
	var response *httpx.Response
	err = stepscommon.AwaitResponse(
		ctx,
		func(ctx context.Context) (*httpx.Response, error) {
			resp, requestErr := s.funnel.Get(ctx, target, nil)
			if requestErr != nil {
				return nil, retry.Transient(requestErr)
			}
			response = resp
			return resp, nil
		},
		func(resp *httpx.Response) bool {
			return resp != nil && resp.StatusCode == status
		},
		fmt.Sprintf("waiting for router readiness status %d", status),
	)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

func (s *Steps) policyEngineHealth(ctx context.Context) error {
	return s.getHealth(ctx, "policy-engine")
}

func (s *Steps) stopService(ctx context.Context, service string) error {
	stack, resolved, err := s.topo.ServiceControl(service)
	if err != nil {
		return err
	}
	if err := stack.StopService(ctx, resolved); err != nil {
		return fmt.Errorf("stopping gateway service %q: %w", service, err)
	}
	return nil
}

func (s *Steps) startService(ctx context.Context, service string) error {
	stack, resolved, err := s.topo.ServiceControl(service)
	if err != nil {
		return err
	}
	if err := stack.StartService(ctx, resolved); err != nil {
		return fmt.Errorf("starting gateway service %q: %w", service, err)
	}
	return nil
}

func (s *Steps) restartService(ctx context.Context, service string) error {
	stack, resolved, err := s.topo.ServiceControl(service)
	if err != nil {
		return err
	}
	if err := stack.RestartService(ctx, resolved); err != nil {
		return fmt.Errorf("restarting gateway service %q: %w", service, err)
	}
	return nil
}

func (s *Steps) responseIndicatesHealthy(ctx context.Context) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return fmt.Errorf("health response was not successful: %s", resp.Describe())
	}
	if healthyStatus(resp.Body) {
		return nil
	}
	return fmt.Errorf("the response does not report status healthy: %s", resp.Describe())
}

func (s *Steps) serviceUnhealthy(ctx context.Context, service string) error {
	v, ok := tcontext.Get(ctx, healthResultsKey)
	if !ok {
		return fmt.Errorf("no health check has been performed in this scenario")
	}
	results, ok := v.(map[string]healthResult)
	if !ok {
		return fmt.Errorf("health results are stored as %T", v)
	}
	result, ok := results[service]
	if !ok {
		return fmt.Errorf("health check did not include service %q", service)
	}
	if result.err == nil && result.healthy {
		return fmt.Errorf("service %q is healthy: %s", service, result.detail)
	}
	return nil
}

func healthyStatus(body []byte) bool {
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		return strings.EqualFold(strings.TrimSpace(payload.Status), "healthy")
	}
	return strings.EqualFold(strings.TrimSpace(string(body)), "healthy")
}

type healthResult struct {
	status  int
	healthy bool
	detail  string
	err     error
}

func responseIsHealthy(service string, resp *httpx.Response) (bool, string) {
	if resp == nil {
		return false, "no response"
	}
	if !resp.Succeeded() {
		return false, fmt.Sprintf("status %d", resp.StatusCode)
	}
	if service == "router" {
		return true, fmt.Sprintf("status %d", resp.StatusCode)
	}
	if !healthyStatus(resp.Body) {
		return false, "body does not report status healthy"
	}
	return true, "status healthy"
}

func (s *Steps) checkAllHealth(ctx context.Context) error {
	targets, err := s.healthTargets()
	if err != nil {
		return err
	}
	results := make(map[string]healthResult, len(targets))
	for name, url := range targets {
		resp, requestErr := s.funnel.Get(ctx, url, nil)
		result := healthResult{err: requestErr}
		if requestErr != nil {
			result.detail = requestErr.Error()
		} else {
			if resp != nil {
				result.status = resp.StatusCode
			}
			result.healthy, result.detail = responseIsHealthy(name, resp)
		}
		results[name] = result
	}
	return tcontext.Set(ctx, healthResultsKey, results)
}

func (s *Steps) allServicesHealthy(ctx context.Context) error {
	v, ok := tcontext.Get(ctx, healthResultsKey)
	if !ok {
		return fmt.Errorf("no health check has been performed in this scenario")
	}
	results, ok := v.(map[string]healthResult)
	if !ok {
		return fmt.Errorf("health results are stored as %T", v)
	}
	var unhealthy []string
	for name, result := range results {
		if result.err != nil || !result.healthy {
			if result.err != nil {
				unhealthy = append(unhealthy, fmt.Sprintf("%s (%v)", name, result.err))
			} else {
				unhealthy = append(unhealthy, fmt.Sprintf("%s (%s)", name, result.detail))
			}
		}
	}
	if len(unhealthy) == 0 {
		return nil
	}
	sort.Strings(unhealthy)
	return fmt.Errorf("these services are not healthy: %s", strings.Join(unhealthy, ", "))
}

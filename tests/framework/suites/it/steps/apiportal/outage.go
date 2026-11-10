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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package apiportal

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

// portalStoppedKind restores a portal a scenario stopped. Its order is lower than every other
// portal resource, so the portal is back before their cleanup needs it.
var portalStoppedKind = cleanup.Kind{Name: "api-portal-stopped", Order: 1}

// registerOutage binds the steps that take the API Portal down and bring it back.
func registerOutage(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^I stop the API Portal$`, s.stopPortal)
	sc.Step(`^I start the API Portal and wait until it is ready$`, s.startPortal)
}

// stopPortal stops the portal's container and arranges for it to be started again when the
// scenario ends, so a failing scenario cannot leave the block without a portal.
func (s *Steps) stopPortal(ctx context.Context) error {
	stack, service, err := s.topo.ServiceControl(portalService)
	if err != nil {
		return err
	}
	reg, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	if err := reg.RegisterDeleter(portalStoppedKind, func(ctx context.Context, _ cleanup.Resource) error {
		return s.bringPortalUp(ctx)
	}); err != nil {
		return err
	}
	if err := reg.Register(cleanup.Resource{
		Kind: portalStoppedKind, ID: portalService, Actor: "admin", Description: "API Portal stopped by the scenario",
	}); err != nil {
		return err
	}
	if err := stack.StopService(ctx, service); err != nil {
		return fmt.Errorf("stopping the API Portal: %w", err)
	}
	return nil
}

// startPortal starts the stopped portal and waits for its health endpoint.
func (s *Steps) startPortal(ctx context.Context) error {
	if err := s.bringPortalUp(ctx); err != nil {
		return err
	}
	reg, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	reg.Deregister(portalStoppedKind, portalService)
	return nil
}

// bringPortalUp starts the portal's container and waits until it reports healthy.
func (s *Steps) bringPortalUp(ctx context.Context) error {
	stack, service, err := s.topo.ServiceControl(portalService)
	if err != nil {
		return err
	}
	if err := stack.StartService(ctx, service); err != nil {
		return fmt.Errorf("starting the API Portal: %w", err)
	}
	instance, err := s.topo.Component(portalService)
	if err != nil {
		return err
	}
	if err := runtime.AwaitHealthy(ctx, instance, newPortalProber()); err != nil {
		return fmt.Errorf("waiting for the API Portal: %w", err)
	}
	return nil
}

// portalProber probes the portal's health endpoint the way the compose readiness wait does: it
// follows redirects, since the portal's root redirects to its sign-in page, and verifies the
// portal's own certificate. The framework's default prober trusts a different certificate for
// this component.
type portalProber struct {
	client *httpx.Client
}

func newPortalProber() portalProber {
	return portalProber{client: httpx.NewClient(httpx.Options{
		Timeout: 5 * time.Second, FollowRedirects: true, TLSClientConfig: shared.APIPortalClientTLS(),
	})}
}

// Probe implements runtime.Prober.
func (p portalProber) Probe(ctx context.Context, url string) (int, error) {
	resp, err := p.client.Do(ctx, httpx.Request{Method: http.MethodGet, URL: url}, 0, 0)
	if err != nil {
		return 0, err
	}
	return resp.StatusCode, nil
}

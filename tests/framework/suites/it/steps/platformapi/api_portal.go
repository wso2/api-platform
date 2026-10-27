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

package platformapi

import (
	"context"
	"net/http"
	"net/url"
	"regexp"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// apiPortalsPath is the publisher-API collection for the api-portal registration resource.
const apiPortalsPath = "/api-portals"

// apiPortalLocationPattern parses the Location value the control plane sets on a successful
// CreateApiPortal. The portal id equals its handle (see openapi.yaml ApiPortalResponse.id).
var apiPortalLocationPattern = regexp.MustCompile(`/api-portals/([^/?#]+)$`)

// RegisterAPIPortal binds the Gherkin steps that drive the API Portal registration surface on
// the publisher API. Requests go through the shared control-plane funnel so the response is
// visible to the generic response assertions, and a successful create registers the portal
// against the publication package's platformAPIPortalKind before the step returns.
func RegisterAPIPortal(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^I create an API Portal via the control plane from "([^"]*)"(?: as "([^"]*)")? with values:$`,
		s.createAPIPortalFromTemplate)
	sc.Step(`^I update the API Portal "([^"]*)" via the control plane from "([^"]*)"(?: as "([^"]*)")? with values:$`,
		s.updateAPIPortalFromTemplate)
	sc.Step(`^I delete the API Portal "([^"]*)" via the control plane(?: as "([^"]*)")?$`, s.deleteAPIPortalStep)
	sc.Step(`^I get the API Portal "([^"]*)" via the control plane(?: as "([^"]*)")?$`, s.getAPIPortalStep)
	sc.Step(`^I list API Portals via the control plane(?: as "([^"]*)")?$`, s.listAPIPortalsStep)
}

func (s *Steps) createAPIPortalFromTemplate(ctx context.Context, templateName, who string, table *godog.Table) error {
	payload, err := s.renderJSONTemplate(ctx, templateName, table)
	if err != nil {
		return err
	}
	resp, err := s.sendControlPlane(ctx, controlPlaneCall{
		method: http.MethodPost, path: apiPortalsPath, who: who, body: payload,
	})
	if err != nil {
		return err
	}
	return s.trackAPIPortalResource(ctx, http.MethodPost, "", resp)
}

func (s *Steps) updateAPIPortalFromTemplate(
	ctx context.Context, handle, templateName, who string, table *godog.Table,
) error {
	resolved, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	payload, err := s.renderJSONTemplate(ctx, templateName, table)
	if err != nil {
		return err
	}
	_, err = s.sendControlPlane(ctx, controlPlaneCall{
		method: http.MethodPut, path: apiPortalsPath + "/" + url.PathEscape(resolved), who: who, body: payload,
	})
	return err
}

func (s *Steps) deleteAPIPortalStep(ctx context.Context, handle, who string) error {
	resolved, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	resp, err := s.sendControlPlane(ctx, controlPlaneCall{
		method: http.MethodDelete, path: apiPortalsPath + "/" + url.PathEscape(resolved), who: who,
	})
	if err != nil {
		return err
	}
	return s.trackAPIPortalResource(ctx, http.MethodDelete, resolved, resp)
}

func (s *Steps) getAPIPortalStep(ctx context.Context, handle, who string) error {
	resolved, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	_, err = s.sendControlPlane(ctx, controlPlaneCall{
		method: http.MethodGet, path: apiPortalsPath + "/" + url.PathEscape(resolved), who: who,
	})
	return err
}

func (s *Steps) listAPIPortalsStep(ctx context.Context, who string) error {
	_, err := s.sendControlPlane(ctx, controlPlaneCall{
		method: http.MethodGet, path: apiPortalsPath, who: who,
	})
	return err
}

// trackAPIPortalResource keeps the cleanup registry in step with what the control plane just
// acknowledged. A 201 to the collection registers the created portal under the publication
// package's platformAPIPortalKind so it is removed by the shared pathDeleter at scenario end;
// a 204 on a single-portal path deregisters what the scenario removed itself.
func (s *Steps) trackAPIPortalResource(ctx context.Context, method, handleHint string, resp *httpx.Response) error {
	switch {
	case method == http.MethodPost && resp.StatusCode == http.StatusCreated:
		match := apiPortalLocationPattern.FindStringSubmatch(locationPath(resp.Headers.Get("Location")))
		if match == nil {
			return nil
		}
		handle := match[1]
		return s.registerTracked(ctx, platformAPIPortalKind, handle,
			func(ctx context.Context) error {
				base, bearer, err := s.authed(ctx)
				if err != nil {
					return err
				}
				return s.deleteResource(ctx, base, bearer, apiPortalsPath+"/"+url.PathEscape(handle))
			},
			func(reg *cleanup.Registry) error {
				return reg.RegisterDeleter(platformAPIPortalKind, s.pathDeleter(platformAPIPortalKind, apiPortalsPath))
			})
	case method == http.MethodDelete && resp.StatusCode == http.StatusNoContent && handleHint != "":
		reg, err := cleanup.Of(ctx)
		if err != nil {
			return err
		}
		reg.Deregister(platformAPIPortalKind, handleHint)
	}
	return nil
}

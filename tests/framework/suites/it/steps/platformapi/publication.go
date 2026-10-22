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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// publicationTypes maps a gateway artifact kind to the apiType segment of the publication
// endpoints.
var publicationTypes = map[string]string{"RestApi": "rest-api"}

var (
	platformPublicationKind = cleanup.Kind{Name: "platform-api-publication", Order: 45}
	platformAPIPortalKind   = cleanup.Kind{Name: "platform-api-api-portal", Order: 60}
)

// publicationIDSeparator joins a portal handle, an API type and an API handle into one cleanup
// resource id.
const publicationIDSeparator = "/"

// RegisterPublication binds the steps that register an API Portal with the control plane and
// publish an API to it.
func RegisterPublication(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^I register the API Portal "([^"]*)" with the control plane$`, s.registerAPIPortal)
	sc.Step(`^I save a publication draft of the "([^"]*)" "([^"]*)" on the API Portal "([^"]*)" with values:$`,
		s.savePublicationDraft)
	sc.Step(`^I save a minimal OpenAPI definition as the draft definition of the "([^"]*)" "([^"]*)" on the API Portal "([^"]*)"$`,
		s.saveDraftDefinition)
	sc.Step(`^I publish the "([^"]*)" "([^"]*)" to the API Portal "([^"]*)" through the control plane$`, s.publishToAPIPortal)
	sc.Step(`^the publication of the "([^"]*)" "([^"]*)" on the API Portal "([^"]*)" should have status "([^"]*)"$`,
		s.publicationStatusIs)
}

// publicationType returns the apiType segment for a gateway artifact kind.
func publicationType(kind string) (string, error) {
	apiType, ok := publicationTypes[kind]
	if !ok {
		return "", fmt.Errorf("publishing %q to an API Portal is not supported", kind)
	}
	return apiType, nil
}

// publicationPath returns the control-plane path of one API's publication resources on a portal.
func publicationPath(portalHandle, apiType, apiHandle string) string {
	return "/api-portals/" + url.PathEscape(portalHandle) + "/apis/" + apiType + "/" + url.PathEscape(apiHandle)
}

// splitPublicationID undoes the cleanup resource id built from a portal, an API type and an API
// handle.
func splitPublicationID(id string) (portalHandle, apiType, apiHandle string, err error) {
	parts := strings.Split(id, publicationIDSeparator)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("malformed publication id %q", id)
	}
	return parts[0], parts[1], parts[2], nil
}

// publicationTarget resolves the placeholders of a publication step into the portal handle, the
// API handle and the endpoint apiType.
func publicationTarget(ctx context.Context, kind, apiHandle, portalHandle string) (portal, api, apiType string, err error) {
	if apiType, err = publicationType(kind); err != nil {
		return "", "", "", err
	}
	if api, err = stepscommon.Expand(ctx, apiHandle); err != nil {
		return "", "", "", err
	}
	if portal, err = stepscommon.Expand(ctx, portalHandle); err != nil {
		return "", "", "", err
	}
	return portal, api, apiType, nil
}

// registerAPIPortal registers the block's API Portal under a handle, with the key platform-api
// publishes to it with.
func (s *Steps) registerAPIPortal(ctx context.Context, handle string) error {
	resolved, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	portal, err := s.topo.Component("api-portal")
	if err != nil {
		return err
	}
	portalURL, err := portal.InternalURL("https")
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}
	if err := s.postJSON(ctx, base, bearer, "/api-portals", map[string]any{
		"name": resolved, "handle": resolved, "url": portalURL, "sharedKey": shared.APIPortalSharedKey(),
	}, nil); err != nil {
		return err
	}
	return s.registerTracked(ctx, platformAPIPortalKind, resolved,
		func(ctx context.Context) error {
			return s.deleteResource(ctx, base, bearer, "/api-portals/"+url.PathEscape(resolved))
		},
		func(reg *cleanup.Registry) error {
			return reg.RegisterDeleter(platformAPIPortalKind, s.pathDeleter(platformAPIPortalKind, "/api-portals"))
		})
}

// post issues one authenticated POST without a body and returns the response whatever its status.
func (s *Steps) post(ctx context.Context, base, bearer, path string) (*httpx.Response, error) {
	resp, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodPost, URL: base + apiBase + path, Headers: map[string]string{"Authorization": "Bearer " + bearer},
	}, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", path, err)
	}
	return resp, nil
}

// deleteResource deletes one control-plane resource; a resource that is already gone is not an
// error.
func (s *Steps) deleteResource(ctx context.Context, base, bearer, path string) error {
	resp, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodDelete, URL: base + apiBase + path, Headers: map[string]string{"Authorization": "Bearer " + bearer},
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("DELETE %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusNotFound && !resp.Succeeded() {
		return fmt.Errorf("DELETE %s failed: %s", path, resp.Describe())
	}
	return nil
}

// draftFromTable builds a publication draft from a two-column values table. A value that is a
// valid JSON array or object, such as a plan list, is decoded; any other value, including text
// that merely starts with a bracket, is a string. Fields the table does not name stay absent.
func draftFromTable(ctx context.Context, table *godog.Table) ([]byte, error) {
	if table == nil || len(table.Rows) == 0 {
		return nil, fmt.Errorf("a publication draft needs a values table")
	}
	draft := make(map[string]any, len(table.Rows))
	for i, row := range table.Rows {
		if row == nil || len(row.Cells) != 2 {
			return nil, fmt.Errorf("draft values row %d must contain exactly two cells", i+1)
		}
		key := strings.TrimSpace(row.Cells[0].Value)
		if key == "" {
			return nil, fmt.Errorf("draft values row %d has an empty key", i+1)
		}
		if _, exists := draft[key]; exists {
			return nil, fmt.Errorf("duplicate draft value %q", key)
		}
		value, err := stepscommon.Expand(ctx, row.Cells[1].Value)
		if err != nil {
			return nil, err
		}
		if decoded, ok := decodeJSONValue(value); ok {
			draft[key] = decoded
			continue
		}
		draft[key] = value
	}
	return json.Marshal(draft)
}

// decodeJSONValue decodes value when it is a JSON array or object.
func decodeJSONValue(value string) (any, bool) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "{") {
		return nil, false
	}
	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil, false
	}
	return decoded, true
}

// savePublicationDraft saves the draft built from the values table.
func (s *Steps) savePublicationDraft(
	ctx context.Context, kind, apiHandle, portalHandle string, table *godog.Table,
) error {
	portal, api, apiType, err := publicationTarget(ctx, kind, apiHandle, portalHandle)
	if err != nil {
		return err
	}
	payload, err := draftFromTable(ctx, table)
	if err != nil {
		return err
	}
	resp, err := s.sendControlPlane(ctx, controlPlaneCall{
		method: http.MethodPut, path: publicationPath(portal, apiType, api) + "/draft", body: payload,
	})
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return fmt.Errorf("saving the publication draft failed: %s", resp.Describe())
	}
	return s.trackSelectedPlans(ctx, payload)
}

// selectedPlans returns the subscription plan handles a draft selects.
func selectedPlans(draft []byte) []string {
	var parsed struct {
		SubscriptionPlanIDs []string `json:"subscriptionPlanIds"`
	}
	if err := json.Unmarshal(draft, &parsed); err != nil {
		return nil
	}
	return parsed.SubscriptionPlanIDs
}

// trackSelectedPlans registers the removal from the API Portal of every plan the draft selects.
// Publishing creates the plans the portal does not have yet, and deleting a plan from the control
// plane does not remove its copy on the portal.
func (s *Steps) trackSelectedPlans(ctx context.Context, draft []byte) error {
	plans := selectedPlans(draft)
	if len(plans) > 0 && s.trackPortalPlan == nil {
		return fmt.Errorf("the draft selects plans, but no API Portal steps are registered to remove them")
	}
	for _, plan := range plans {
		if err := s.trackPortalPlan(ctx, plan); err != nil {
			return fmt.Errorf("registering plan %q for removal from the API Portal: %w", plan, err)
		}
	}
	return nil
}

// minimalOpenAPIDefinition returns the smallest OpenAPI document the control plane accepts as a
// REST API definition.
func minimalOpenAPIDefinition(title string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"openapi": "3.0.3",
		"info":    map[string]any{"title": title, "version": "1.0.0"},
		"paths": map[string]any{"/ping": map[string]any{"get": map[string]any{
			"responses": map[string]any{"200": map[string]any{"description": "ok"}},
		}}},
	})
}

// saveDraftDefinition stores a minimal OpenAPI definition on the saved draft, which publishing
// requires and never takes from the API itself.
func (s *Steps) saveDraftDefinition(ctx context.Context, kind, apiHandle, portalHandle string) error {
	portal, api, apiType, err := publicationTarget(ctx, kind, apiHandle, portalHandle)
	if err != nil {
		return err
	}
	definition, err := minimalOpenAPIDefinition(api)
	if err != nil {
		return err
	}
	resp, err := s.sendControlPlane(ctx, controlPlaneCall{
		method: http.MethodPut, path: publicationPath(portal, apiType, api) + "/draft/definition", body: definition,
	})
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return fmt.Errorf("saving the draft definition failed: %s", resp.Describe())
	}
	return nil
}

// publishToAPIPortal publishes the saved draft. The response is published for the scenario to
// assert. The unpublish that removes the listing is registered before the request is sent,
// because the portal keeps its listing after the control plane's records are deleted and a
// publish can fail after the portal accepted it; unpublishing what was never published is settled.
func (s *Steps) publishToAPIPortal(ctx context.Context, kind, apiHandle, portalHandle string) error {
	portal, api, apiType, err := publicationTarget(ctx, kind, apiHandle, portalHandle)
	if err != nil {
		return err
	}
	if err := s.registerPublication(ctx, portal, apiType, api); err != nil {
		return err
	}
	_, err = s.sendControlPlane(ctx, controlPlaneCall{
		method: http.MethodPost, path: publicationPath(portal, apiType, api) + "/publish",
	})
	return err
}

// publicationStateConflict is the error code of an unpublish that found nothing to unpublish.
const publicationStateConflict = "PUBLICATION_STATE_CONFLICT"

// unpublishSettled reports whether an unpublish response leaves nothing to clean up: it
// succeeded, the API is unknown, or the scenario had already unpublished it. Any other conflict,
// such as the portal still holding subscriptions or keys, means the listing is still there.
func unpublishSettled(resp *httpx.Response) bool {
	if resp.Succeeded() || resp.StatusCode == http.StatusNotFound {
		return true
	}
	if resp.StatusCode != http.StatusConflict {
		return false
	}
	var failure struct {
		Code string `json:"code"`
	}
	return json.Unmarshal(resp.Body, &failure) == nil && failure.Code == publicationStateConflict
}

// registerPublication registers the unpublish that removes a publication at cleanup.
func (s *Steps) registerPublication(ctx context.Context, portalHandle, apiType, apiHandle string) error {
	reg, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	if err := reg.RegisterDeleter(platformPublicationKind, func(ctx context.Context, res cleanup.Resource) error {
		portal, apiType, api, splitErr := splitPublicationID(res.ID)
		if splitErr != nil {
			return splitErr
		}
		base, bearer, authErr := s.authed(ctx)
		if authErr != nil {
			return authErr
		}
		resp, doErr := s.post(ctx, base, bearer, publicationPath(portal, apiType, api)+"/unpublish")
		if doErr != nil {
			return doErr
		}
		if !unpublishSettled(resp) {
			return fmt.Errorf("unpublishing %s from %s: %s", api, portal, resp.Describe())
		}
		return nil
	}); err != nil {
		return err
	}
	return reg.Register(cleanup.Resource{
		Kind: platformPublicationKind, ID: strings.Join([]string{portalHandle, apiType, apiHandle}, publicationIDSeparator),
		Actor: "admin", Description: "published via platform-api",
	})
}

// publicationStatusIs asserts the status the control plane reports for the live publication.
func (s *Steps) publicationStatusIs(ctx context.Context, kind, apiHandle, portalHandle, want string) error {
	portal, api, apiType, err := publicationTarget(ctx, kind, apiHandle, portalHandle)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}
	resp, err := s.get(ctx, base, bearer, publicationPath(portal, apiType, api)+"/publication")
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return fmt.Errorf("reading the publication of %s on %s failed: %s", api, portal, resp.Describe())
	}
	var publication struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Body, &publication); err != nil {
		return fmt.Errorf("decoding the publication of %s on %s: %w", api, portal, err)
	}
	if publication.Status != want {
		return fmt.Errorf("the publication of %s on %s has status %q, want %q", api, portal, publication.Status, want)
	}
	return nil
}

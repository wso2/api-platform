/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudconsole

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

const (
	// portalNameMaxLength and portalDescriptionMaxLength are the longest name and description the
	// platform accepts.
	portalNameMaxLength        = 255
	portalDescriptionMaxLength = 1023
)

// portalSecretMarkers are fragments of a field name that a managed API Portal's answer must never
// carry: the shared key between the control plane and the portal, and the credentials of the
// portal's identity-provider client. Only field names are checked, so an ordinary name or
// description such as "Secrets documentation" is not taken for a leak.
var portalSecretMarkers = []string{"sharedkey", "shared_key", "clientsecret", "client_secret", "password", "secret"}

// portalCreateBodyDefects turn a valid portal create body into one the platform must refuse before
// it reads any field of it. Each returns the raw body to send.
var portalCreateBodyDefects = map[string]func(valid map[string]any) ([]byte, error){
	"an unknown field": func(valid map[string]any) ([]byte, error) {
		valid["unexpected"] = true
		return json.Marshal(valid)
	},
	"an organization field": func(valid map[string]any) ([]byte, error) {
		valid["organization"] = "another-organization"
		return json.Marshal(valid)
	},
	"a name that is not text": func(valid map[string]any) ([]byte, error) {
		valid["name"] = 42
		return json.Marshal(valid)
	},
	"malformed JSON": func(valid map[string]any) ([]byte, error) {
		raw, err := json.Marshal(valid)
		if err != nil {
			return nil, err
		}
		return raw[:len(raw)-1], nil
	},
}

// portalUpdateDefects are update bodies the platform must refuse.
var portalUpdateDefects = map[string]map[string]any{
	"an empty name":              {"name": ""},
	"an empty login environment": {"loginEnvironment": ""},
	"a name longer than 255 characters": {
		"name": strings.Repeat("n", portalNameMaxLength+1),
	},
	"a description longer than 1023 characters": {
		"description": strings.Repeat("d", portalDescriptionMaxLength+1),
	},
	"a handle": {"handle": "another-handle"},
}

// portalUpdateFields are the fields of an update a values table may set, with the type each has.
var portalUpdateFields = map[string]string{
	"name": "text", "description": "text", "loginEnvironment": "text", "selfSignup": "boolean",
}

func portalNamesOf[V any](values map[string]V) string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// registerPortalManageSteps binds the steps that change a managed API Portal after it is created
// and the steps that check what its answers carry.
func (s *Steps) registerPortalManageSteps(sc stepRegistrar) {
	sc.Step(`^I send a managed API Portal create request with (an unknown field|an organization field|a name that is not text|malformed JSON)$`, s.sendDefectiveCreateBody)
	sc.Step(`^I update the synthetic managed API Portal with:$`, s.updateRunnerPortal)
	sc.Step(`^I try to update the synthetic managed API Portal with (an empty name|an empty login environment|a name longer than 255 characters|a description longer than 1023 characters|a handle)$`, s.tryDefectiveUpdate)
	sc.Step(`^I rotate the key of the synthetic managed API Portal$`, s.rotateRunnerPortalKey)
	sc.Step(`^I (update|rotate the key of) a managed API Portal that does not exist$`, s.changeAbsentPortal)
	sc.Step(`^the managed API Portal answer should carry its details$`, s.assertPortalDetails)
	sc.Step(`^the managed API Portal answer should expose no secret$`, s.assertPortalAnswerHasNoSecret)
	sc.Step(`^the managed API Portal answer should locate the synthetic managed API Portal$`, s.assertPortalLocation)
	sc.Step(`^I send a "([^"]*)" request (without a token|with a forged token) to the managed API Portal path "([^"]*)"$`, s.sendWithoutValidToken)
}

// sendDefectiveCreateBody sends a create body that breaks the request's shape and publishes the
// answer. The portal is registered for cleanup first and dropped again when the platform refuses
// the request, so a portal accepted against expectation is removed.
func (s *Steps) sendDefectiveCreateBody(ctx context.Context, defect string) error {
	corrupt, ok := portalCreateBodyDefects[defect]
	if !ok {
		return fmt.Errorf("unknown managed API Portal create body defect %q: the defects are %s", defect, portalNamesOf(portalCreateBodyDefects))
	}
	if err := generateResourceAndStore(ctx, "syn-portal", keyRefusedAPIPortalID); err != nil {
		return err
	}
	handle, err := requiredValue(ctx, keyRefusedAPIPortalID)
	if err != nil {
		return err
	}
	body, err := corrupt(portalRequestBody(handle, "Synthetic "+handle))
	if err != nil {
		return fmt.Errorf("building the %s create body: %w", defect, err)
	}
	if err := cleanup.Register(ctx, portalResource(handle)); err != nil {
		return fmt.Errorf("registering managed API Portal %q for cleanup: %w", handle, err)
	}
	headers, err := bearerHeaders(ctx)
	if err != nil {
		return err
	}
	headers["Content-Type"] = "application/json"
	resp, err := s.send(ctx, http.MethodPost, managedPortalsCollection, headers, body)
	if err != nil {
		return err
	}
	if refusedCreate(resp) {
		deregister(ctx, kindAPIPortal, handle)
	}
	return nil
}

// portalUpdateBody turns the cells of a values table into an update body.
func portalUpdateBody(values map[string]string) (map[string]any, error) {
	body := make(map[string]any, len(values))
	for field, raw := range values {
		kind, ok := portalUpdateFields[field]
		if !ok {
			return nil, fmt.Errorf("unsupported managed API Portal update field %q: the fields are %s", field, portalNamesOf(portalUpdateFields))
		}
		if kind == "boolean" {
			value, err := strconv.ParseBool(strings.TrimSpace(raw))
			if err != nil {
				return nil, fmt.Errorf("update field %q must be true or false: %w", field, err)
			}
			body[field] = value
			continue
		}
		body[field] = raw
	}
	return body, nil
}

// updateRunnerPortal updates the runner's portal with the fields of the table and publishes the
// answer without requiring success.
func (s *Steps) updateRunnerPortal(ctx context.Context, table *godog.Table) error {
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	for field, raw := range values {
		if values[field], err = expandCloudValue(ctx, raw); err != nil {
			return err
		}
	}
	body, err := portalUpdateBody(values)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodPut, portalPath(handle), body)
	return err
}

// tryDefectiveUpdate sends an update the platform must refuse to the runner's portal and publishes
// the answer.
func (s *Steps) tryDefectiveUpdate(ctx context.Context, defect string) error {
	body, ok := portalUpdateDefects[defect]
	if !ok {
		return fmt.Errorf("unknown managed API Portal update defect %q: the defects are %s", defect, portalNamesOf(portalUpdateDefects))
	}
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodPut, portalPath(handle), body)
	return err
}

// rotateRunnerPortalKey asks the platform to rotate the shared key of the runner's portal and
// publishes the answer.
func (s *Steps) rotateRunnerPortalKey(ctx context.Context) error {
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodPost, portalPath(handle)+"/rotate-key", nil)
	return err
}

// changeAbsentPortal updates a portal, or rotates its key, that nothing has created, and publishes
// the answer.
func (s *Steps) changeAbsentPortal(ctx context.Context, change string) error {
	if err := generateResourceAndStore(ctx, "syn-portal", keyAbsentAPIPortalID); err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyAbsentAPIPortalID)
	if err != nil {
		return err
	}
	if change == "update" {
		_, err = s.doAuthenticated(ctx, http.MethodPut, portalPath(id), map[string]any{"name": "Synthetic " + id})
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodPost, portalPath(id)+"/rotate-key", nil)
	return err
}

// portalDetailFields are the fields every answer about one managed API Portal carries.
// The handle is the `id`: the answer has no separate `handle` field.
var portalDetailFields = []string{"id", "name", "url", "status", "loginEnvironment", "createdAt", "updatedAt"}

// assertPortalDetails checks the published answer is one portal that carries its details.
func (s *Steps) assertPortalDetails(ctx context.Context) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var portal map[string]any
	if err := json.Unmarshal(resp.Body, &portal); err != nil {
		return fmt.Errorf("the managed API Portal answer is not a JSON object: %w (%s)", err, resp.Describe())
	}
	return requirePortalDetails(portal)
}

func requirePortalDetails(portal map[string]any) error {
	var missing []string
	for _, field := range portalDetailFields {
		if value, _ := portal[field].(string); strings.TrimSpace(value) == "" {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("the managed API Portal answer lacks %s", strings.Join(missing, ", "))
	}
	return nil
}

// assertPortalAnswerHasNoSecret checks no field name or value of the published answer carries a
// secret of the portal.
func (s *Steps) assertPortalAnswerHasNoSecret(ctx context.Context) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if !resp.HasBody() {
		return fmt.Errorf("the managed API Portal answer has no body to check: %s", resp.Describe())
	}
	return requireNoSecret(resp.Body)
}

// requireNoSecret fails when any field name of a JSON body, at any depth, carries one of the portal
// secret markers. The matching marker is named, never the body.
func requireNoSecret(body []byte) error {
	var document any
	if err := json.Unmarshal(body, &document); err != nil {
		return fmt.Errorf("the managed API Portal answer is not JSON, so it cannot be checked for secrets: %w", err)
	}
	if marker := secretMarkerIn(document); marker != "" {
		return fmt.Errorf("the managed API Portal answer has a field named after %q, which would reveal a secret of the portal", marker)
	}
	return nil
}

// secretMarkerIn returns the first portal secret marker found in a field name of the value, or "".
func secretMarkerIn(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for name, nested := range typed {
			lower := strings.ToLower(name)
			for _, marker := range portalSecretMarkers {
				if strings.Contains(lower, marker) {
					return marker
				}
			}
			if marker := secretMarkerIn(nested); marker != "" {
				return marker
			}
		}
	case []any:
		for _, nested := range typed {
			if marker := secretMarkerIn(nested); marker != "" {
				return marker
			}
		}
	}
	return ""
}

// assertPortalLocation checks the published answer's Location header names the runner's portal.
func (s *Steps) assertPortalLocation(ctx context.Context) error {
	handle, err := runnerPortal(ctx)
	if err != nil {
		return err
	}
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	location := strings.TrimSpace(resp.Headers.Get("Location"))
	if !strings.HasSuffix(location, managedPortalsCollection+"/"+handle) {
		return fmt.Errorf("the Location header is %q, want a URL ending in %s/%s", location, managedPortalsCollection, handle)
	}
	return nil
}

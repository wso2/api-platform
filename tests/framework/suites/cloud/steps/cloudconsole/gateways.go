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
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// gatewayStatuses are the statuses a managed gateway reports.
var gatewayStatuses = map[string]bool{"PROVISIONING": true, "FAILED": true, "ACTIVE": true, "DISCONNECTED": true}

// gatewayKinds are the functionality types a managed gateway can have.
var gatewayKinds = []string{regularGatewayKind, "ai", "event"}

// scenarioGatewayDescription is the description a suite run gives the gateways it creates for its
// scenarios when the scenario names none. It tells a run that is pointed at a gateway whether the
// gateway belongs to a suite run, whatever its name looks like.
const scenarioGatewayDescription = "Created by the APIP cloud integration suite for one of its runs and removed when the run ends."

// managedGatewayRequestKeys lists the values a managed gateway request table may carry.
var managedGatewayRequestKeys = map[string]bool{
	"displayName": true, "environment": true, "handle": true, "version": true,
	"functionalityType": true, "description": true, "isCritical": true, "properties": true, "isDefault": true,
}

// validateGatewayListing checks the fields every listed managed gateway carries and that the
// status agrees with the connection flag.
func validateGatewayListing(items []map[string]any) error {
	if len(items) == 0 {
		return errors.New("the organisation lists no managed gateway")
	}
	for _, item := range items {
		id, _ := item["id"].(string)
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("a listed managed gateway has no id: %v", item)
		}
		for _, field := range []string{"isActive", "isCritical", "isDefault"} {
			if _, ok := item[field].(bool); !ok {
				return fmt.Errorf("managed gateway %q: %s is missing or not a boolean", id, field)
			}
		}
		if environment, _ := item["environment"].(string); strings.TrimSpace(environment) == "" {
			return fmt.Errorf("managed gateway %q reports no environment", id)
		}
		status, _ := item["status"].(string)
		if !gatewayStatuses[status] {
			return fmt.Errorf("managed gateway %q reports status %q, want one of PROVISIONING, FAILED, ACTIVE, DISCONNECTED (the listing gives it as %v)", id, status, item)
		}
		reason, _ := item["statusReason"].(string)
		if status == "FAILED" && reason == "" {
			return fmt.Errorf("managed gateway %q is FAILED without a status reason", id)
		}
		if status != "FAILED" && reason != "" {
			return fmt.Errorf("managed gateway %q reports status reason %q while %s", id, reason, status)
		}
		active, _ := item["isActive"].(bool)
		if active && status != "ACTIVE" && status != "FAILED" {
			return fmt.Errorf("managed gateway %q is connected but reports %s", id, status)
		}
		if status == "ACTIVE" && !active {
			return fmt.Errorf("managed gateway %q reports ACTIVE but isActive is false", id)
		}
		if kind, _ := item["functionalityType"].(string); kind != "" && !slices.Contains(gatewayKinds, kind) {
			return fmt.Errorf("managed gateway %q has unknown functionality type %q", id, kind)
		}
		if endpoints, present := item["endpoints"]; present {
			list, ok := endpoints.([]any)
			if !ok {
				return fmt.Errorf("managed gateway %q: endpoints is not a list", id)
			}
			for _, endpoint := range list {
				if text, ok := endpoint.(string); !ok || strings.TrimSpace(text) == "" {
					return fmt.Errorf("managed gateway %q lists an endpoint that is not a string: %v", id, endpoint)
				}
			}
		}
	}
	return nil
}

// expectedGatewayID is the id the platform registers a gateway under: the environment and the
// handle, which is the display name folded to lower case when none is given.
func expectedGatewayID(environment, displayName, handle string) string {
	if handle = strings.TrimSpace(handle); handle == "" {
		handle = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(displayName), " ", "-"))
	}
	return environment + "-" + handle
}

// buildManagedGatewayRequest turns a values table into the body of a managed gateway request.
// The display name defaults to the scenario's chosen name; the environment is required.
func buildManagedGatewayRequest(ctx context.Context, values map[string]string) (map[string]any, error) {
	body := map[string]any{}
	for key, raw := range values {
		if !managedGatewayRequestKeys[key] {
			return nil, fmt.Errorf("unsupported managed gateway value %q", key)
		}
		value, err := expandCloudValue(ctx, raw)
		if err != nil {
			return nil, fmt.Errorf("expanding %q: %w", key, err)
		}
		parsed, err := typedGatewayValue(key, value)
		if err != nil {
			return nil, err
		}
		body[key] = parsed
	}
	if _, present := body["displayName"]; !present {
		name, err := requiredValue(ctx, keyManagedGatewayName)
		if err != nil {
			return nil, fmt.Errorf("no displayName given and no gateway name chosen: %w", err)
		}
		body["displayName"] = name
	}
	if environment, _ := body["environment"].(string); strings.TrimSpace(environment) == "" {
		return nil, errors.New("a managed gateway request needs an environment")
	}
	if _, present := body["description"]; !present {
		body["description"] = scenarioGatewayDescription
	}
	return body, nil
}

// typedGatewayValue converts a table cell to the type its field has on the wire.
func typedGatewayValue(key, value string) (any, error) {
	switch key {
	case "isCritical", "isDefault":
		flag, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("%s must be true or false, got %q", key, value)
		}
		return flag, nil
	case "properties":
		var properties map[string]any
		if err := json.Unmarshal([]byte(value), &properties); err != nil {
			return nil, fmt.Errorf("properties must be a JSON object: %w", err)
		}
		return properties, nil
	}
	return value, nil
}

// syntheticGatewayBase is the base of the names a suite run gives its managed gateways. The unique
// generator adds 11 characters to it, and the platform limits a handle to 37 minus twice the
// environment name's length, which leaves 15 characters in "development".
const syntheticGatewayBase = "sg"

// chooseGatewayName picks the unique name a managed gateway request is made under.
func (s *Steps) chooseGatewayName(ctx context.Context) error {
	return generateResourceAndStore(ctx, syntheticGatewayBase, keyManagedGatewayName)
}

// requestManagedGateway sends a managed gateway request built from a table and publishes the
// response without requiring success, so a scenario can assert a refusal as well as a creation.
// The gateway is registered for cleanup before the request, under the id the platform will give
// it, and the registration is dropped again when the platform refuses the request.
func (s *Steps) requestManagedGateway(ctx context.Context, table *godog.Table) error {
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	return s.requestManagedGatewayWith(ctx, values)
}

// requestManagedGatewayWith is requestManagedGateway for values already read from a table. The
// gateway is cleaned up when the scenario ends.
func (s *Steps) requestManagedGatewayWith(ctx context.Context, values map[string]string) error {
	return s.requestManagedGatewayScoped(ctx, values, cleanup.ScenarioScope)
}

// requestRunnerManagedGateway requests a gateway that later scenarios of the runner keep using, so
// it is cleaned up when the runner ends. A gateway takes minutes to build, which is why the
// scenarios that do not test its creation share one.
func (s *Steps) requestRunnerManagedGateway(ctx context.Context, table *godog.Table) error {
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	return s.requestManagedGatewayScoped(ctx, values, cleanup.RunnerScope)
}

// registerGateway records a gateway for cleanup in the given scope.
func registerGateway(ctx context.Context, scope cleanup.Scope, id string) error {
	resource := cleanup.Resource{
		Kind: cleanup.KindGateway, ID: id, Actor: resourceActor, Description: "managed gateway " + id,
	}
	if scope == cleanup.RunnerScope {
		return cleanup.RegisterRunner(ctx, resource)
	}
	return cleanup.Register(ctx, resource)
}

// gatewayRequestAccepted reports whether the platform took a gateway request: it answers 201 when
// the gateway is registered at once and 202 when its build was started.
func gatewayRequestAccepted(status int) bool {
	return status == http.StatusCreated || status == http.StatusAccepted
}

func (s *Steps) requestManagedGatewayScoped(ctx context.Context, values map[string]string, scope cleanup.Scope) error {
	body, err := buildManagedGatewayRequest(ctx, values)
	if err != nil {
		return err
	}
	environment, _ := body["environment"].(string)
	displayName, _ := body["displayName"].(string)
	handle, _ := body["handle"].(string)
	fallback := expectedGatewayID(environment, displayName, handle)
	known := registeredForCleanup(ctx, fallback)
	if !known {
		if err := registerGateway(ctx, scope, fallback); err != nil {
			return fmt.Errorf("registering managed gateway %q for cleanup: %w", fallback, err)
		}
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/managed-gateways", body)
	if err != nil {
		return err
	}
	if !gatewayRequestAccepted(resp.StatusCode) {
		if !known && refusedCreate(resp) {
			deregister(ctx, cleanup.KindGateway, fallback)
		}
		return nil
	}
	id, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating APIP managed gateway: %w", err)
	}
	if id != fallback && !known {
		deregister(ctx, cleanup.KindGateway, fallback)
	}
	// The platform may register the gateway under another id than the one expected, for example
	// when the name is taken, so the id it answered with is the one that must be cleaned up.
	if !registeredForCleanup(ctx, id) {
		if err := registerGateway(ctx, scope, id); err != nil {
			return fmt.Errorf("registering managed gateway %q for cleanup: %w", id, err)
		}
	}
	if err := recordSyntheticGateway(ctx, id, environment); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyManagedGatewayEnv, environment); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyManagedGatewayID, id)
}

// registeredForCleanup reports whether a gateway is waiting to be cleaned up.
func registeredForCleanup(ctx context.Context, id string) bool {
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return false
	}
	for _, resource := range registry.Pending() {
		if resource.Kind == cleanup.KindGateway && resource.ID == id {
			return true
		}
	}
	return false
}

// listManagedGateways lists the managed gateways and publishes the response.
func (s *Steps) listManagedGateways(ctx context.Context) error {
	_, err := s.doAuthenticated(ctx, http.MethodGet, "/managed-gateways", nil)
	return err
}

// publishedListing decodes the list the last response carried.
func publishedListing(ctx context.Context) ([]map[string]any, error) {
	response, err := httpx.Published(ctx)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, responseError(response)
	}
	items, err := listItems(response)
	if err != nil {
		return nil, fmt.Errorf("decoding managed gateways: %w (%s)", err, response.Describe())
	}
	return items, nil
}

func (s *Steps) assertListingIsValid(ctx context.Context) error {
	items, err := publishedListing(ctx)
	if err != nil {
		return err
	}
	return validateGatewayListing(items)
}

func (s *Steps) assertListingIncludesActiveGateway(ctx context.Context) error {
	items, err := publishedListing(ctx)
	if err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyGatewayID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item["id"] == id {
			if active, _ := item["isActive"].(bool); !active {
				return fmt.Errorf("the listing shows the active gateway %q as not active", id)
			}
			return nil
		}
	}
	return fmt.Errorf("the listing of %d managed gateways does not include the active gateway %q", len(items), id)
}

// assertSelectedEnvironmentListed checks the environment chosen to host a gateway is one the
// platform lists, named by its own id.
func (s *Steps) assertSelectedEnvironmentListed(ctx context.Context) error {
	name, err := requiredValue(ctx, keyEnvironmentName)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodGet, "/environments", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	items, err := listItems(resp)
	if err != nil {
		return fmt.Errorf("decoding APIP environments: %w", err)
	}
	for _, item := range items {
		if item["name"] != name {
			continue
		}
		if item["id"] != name {
			return fmt.Errorf("environment %q is listed with id %v, want its own name", name, item["id"])
		}
		if _, ok := item["isProduction"].(bool); !ok {
			return fmt.Errorf("environment %q does not report isProduction", name)
		}
		return nil
	}
	return fmt.Errorf("the selected environment %q is not in the platform's list of %d environments", name, len(items))
}

// sameValue compares a value read from JSON with the value a table cell stands for.
func sameValue(got, want any) bool {
	if reflect.DeepEqual(got, want) {
		return true
	}
	left, leftErr := json.Marshal(got)
	right, rightErr := json.Marshal(want)
	return leftErr == nil && rightErr == nil && string(left) == string(right)
}

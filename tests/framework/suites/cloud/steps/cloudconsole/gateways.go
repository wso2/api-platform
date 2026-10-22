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
	"net/url"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	// maxGatewayHandleLength is the width of the platform's gateway handle column; a managed
	// gateway is registered as "<environment>-<handle>", so the environment's name uses part of it.
	maxGatewayHandleLength = 40
	// regularGatewayKind is the kind of a gateway that records none.
	regularGatewayKind = "regular"
	// gatewayBuildTimeout bounds how long the platform may take to report a new gateway as built.
	gatewayBuildTimeout = 3 * time.Minute
)

// gatewayStatuses are the statuses a managed gateway reports.
var gatewayStatuses = map[string]bool{"PROVISIONING": true, "FAILED": true, "ACTIVE": true, "DISCONNECTED": true}

// gatewayKinds are the functionality types a managed gateway can have.
var gatewayKinds = []string{regularGatewayKind, "ai", "event"}

// managedGatewayRequestKeys lists the values a managed gateway request table may carry.
var managedGatewayRequestKeys = map[string]bool{
	"displayName": true, "environment": true, "handle": true, "version": true,
	"functionalityType": true, "description": true, "isCritical": true, "properties": true, "isDefault": true,
}

// managedGatewayUpdateKeys lists the values a managed gateway update table may carry.
var managedGatewayUpdateKeys = map[string]bool{
	"displayName": true, "description": true, "isCritical": true, "properties": true,
}

// gatewayIdentityFields are the fields of a managed gateway an update must leave as they are.
var gatewayIdentityFields = []string{"id", "environment", "host", "functionalityType", "version"}

// gatewayKind is the kind of a listed gateway; a gateway with none recorded is a regular one.
func gatewayKind(item map[string]any) string {
	if kind, _ := item["functionalityType"].(string); strings.TrimSpace(kind) != "" {
		return kind
	}
	return regularGatewayKind
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

// gatewayHandleBudget is how many characters a handle may use in an environment.
func gatewayHandleBudget(environment string) int {
	if budget := maxGatewayHandleLength - len(environment) - 1; budget > 1 {
		return budget
	}
	return 1
}

// handleOfLength builds a handle of exactly the given length from a unique name: it keeps the
// name's tail, which is what makes it unique, and pads short names with a letter.
func handleOfLength(name string, length int) string {
	if length < 1 {
		return ""
	}
	if len(name) >= length {
		name = name[len(name)-length:]
	} else {
		name += strings.Repeat("x", length-len(name))
	}
	if name[0] == '-' {
		name = "a" + name[1:]
	}
	return name
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

// chooseGatewayName starts a scenario's managed gateways: it picks a unique name and forgets the
// gateways an earlier scenario of the runner created, which are already deleted.
func (s *Steps) chooseGatewayName(ctx context.Context) error {
	if err := generateResourceAndStore(ctx, "syn-gw", keyManagedGatewayName); err != nil {
		return err
	}
	for _, key := range []string{keySyntheticGateways, keyManagedGatewayID, keyManagedGatewayEnv, keyGatewayIdentity, keyDefaultGatewayID} {
		if err := tcontext.Set(ctx, key, ""); err != nil {
			return err
		}
	}
	return nil
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

// requestManagedGatewayWith is requestManagedGateway for values already read from a table.
func (s *Steps) requestManagedGatewayWith(ctx context.Context, values map[string]string) error {
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
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: cleanup.KindGateway, ID: fallback, Actor: resourceActor, Description: "managed gateway " + fallback,
		}); err != nil {
			return fmt.Errorf("registering managed gateway %q for cleanup: %w", fallback, err)
		}
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, "/managed-gateways", body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		if !known && resp.StatusCode >= 400 && resp.StatusCode < 500 {
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
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: cleanup.KindGateway, ID: id, Actor: resourceActor, Description: "managed gateway " + id,
		}); err != nil {
			return fmt.Errorf("registering managed gateway %q for cleanup: %w", id, err)
		}
	}
	if err := recordSyntheticGateway(ctx, id); err != nil {
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

// recordSyntheticGateway adds a gateway to the scenario's list once.
func recordSyntheticGateway(ctx context.Context, id string) error {
	ids := syntheticGatewayIDs(ctx)
	if slices.Contains(ids, id) {
		return nil
	}
	return tcontext.Set(ctx, keySyntheticGateways, strings.Join(append(ids, id), ","))
}

// createGatewayBelowHandleLimit creates a synthetic gateway whose handle is the given number of
// characters short of the environment's limit.
func (s *Steps) createGatewayBelowHandleLimit(ctx context.Context, below int) error {
	environment, err := requiredValue(ctx, keyGatewayEnv)
	if err != nil {
		return fmt.Errorf("the active gateway does not report its environment: %w", err)
	}
	name, err := freshGatewayName(ctx)
	if err != nil {
		return err
	}
	length := gatewayHandleBudget(environment) - below
	handle := handleOfLength(name, length)
	if len(handle) != length || length < 1 {
		return fmt.Errorf("cannot build a handle %d characters short of the %d-character limit", below, gatewayHandleBudget(environment))
	}
	values := map[string]string{"displayName": name, "environment": environment, "handle": handle}
	if err := s.requestManagedGatewayWith(ctx, values); err != nil {
		return err
	}
	return s.requireCreated(ctx)
}

// freshGatewayName draws a unique gateway name without touching the scenario's chosen name.
func freshGatewayName(ctx context.Context) (string, error) {
	local, ok := tcontext.LocalOf(ctx)
	if !ok || local == nil {
		return "", errors.New("cannot draw a gateway name without runner context")
	}
	previous, _ := tcontext.ResolveString(ctx, keyManagedGatewayName)
	if err := generateResourceAndStore(ctx, "syn-gw", keyManagedGatewayName); err != nil {
		return "", err
	}
	name, err := requiredValue(ctx, keyManagedGatewayName)
	if err != nil {
		return "", err
	}
	if previous != "" {
		local.Set(keyManagedGatewayName, previous)
	}
	return name, nil
}

// requireCreated fails unless the last response was a creation.
func (s *Steps) requireCreated(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusCreated {
		return responseError(response)
	}
	return nil
}

// fetchGatewayList reads the managed gateways, narrowed to one kind when given.
func (s *Steps) fetchGatewayList(ctx context.Context, kind string) ([]map[string]any, error) {
	path := "/managed-gateways"
	if kind != "" {
		path += "?functionalityType=" + url.QueryEscape(kind)
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retry.Transient(responseError(resp))
	}
	items, err := listItems(resp)
	if err != nil {
		return nil, retry.Transient(fmt.Errorf("decoding managed gateways: %w", err))
	}
	return items, nil
}

// fetchManagedGateway reads one managed gateway.
func (s *Steps) fetchManagedGateway(ctx context.Context, id string) (map[string]any, error) {
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, "/managed-gateways/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retry.Transient(responseError(resp))
	}
	var document map[string]any
	if err := json.Unmarshal(resp.Body, &document); err != nil {
		return nil, retry.Transient(fmt.Errorf("decoding managed gateway %q: %w", id, err))
	}
	return document, nil
}

// listManagedGateways lists the managed gateways and publishes the response.
func (s *Steps) listManagedGateways(ctx context.Context) error {
	_, err := s.doAuthenticated(ctx, http.MethodGet, "/managed-gateways", nil)
	return err
}

// listManagedGatewaysOfType lists the managed gateways of one kind and publishes the response.
func (s *Steps) listManagedGatewaysOfType(ctx context.Context, kind string) error {
	_, err := s.doAuthenticated(ctx, http.MethodGet, "/managed-gateways?functionalityType="+url.QueryEscape(kind), nil)
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

func (s *Steps) assertListingOnlyOfType(ctx context.Context, kind string) error {
	items, err := publishedListing(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if got := gatewayKind(item); got != kind {
			return fmt.Errorf("the listing for type %q includes %v, a gateway of type %q", kind, item["id"], got)
		}
	}
	return nil
}

// gatewayIDs lists the ids of listed gateways.
func gatewayIDs(items []map[string]any) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if id, _ := item["id"].(string); id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// partitionByKind checks that the typed listings split the unfiltered one: every gateway is in
// exactly one of them, in the one of its own kind, and none lists a gateway the full list lacks.
func partitionByKind(all []map[string]any, byKind map[string][]map[string]any) error {
	owner := map[string]string{}
	for _, kind := range gatewayKinds {
		for _, item := range byKind[kind] {
			id, _ := item["id"].(string)
			if got := gatewayKind(item); got != kind {
				return fmt.Errorf("the listing for type %q includes %s, a gateway of type %q", kind, id, got)
			}
			if previous, taken := owner[id]; taken {
				return fmt.Errorf("gateway %s is listed for both type %q and type %q", id, previous, kind)
			}
			owner[id] = kind
		}
	}
	for _, id := range gatewayIDs(all) {
		if _, listed := owner[id]; !listed {
			return fmt.Errorf("gateway %s is in the unfiltered list but in no typed list", id)
		}
		delete(owner, id)
	}
	if len(owner) > 0 {
		stray := make([]string, 0, len(owner))
		for id := range owner {
			stray = append(stray, id)
		}
		sort.Strings(stray)
		return fmt.Errorf("typed lists include gateways the unfiltered list lacks: %s", strings.Join(stray, ", "))
	}
	return nil
}

// assertTypesPartitionTheListing reads the list unfiltered and once per kind, retrying because a
// gateway created by another runner between the reads would show as a mismatch.
func (s *Steps) assertTypesPartitionTheListing(ctx context.Context) error {
	return retry.Await(ctx, retry.Options{Timeout: time.Minute, Interval: 2 * time.Second},
		func(ctx context.Context) (string, error) {
			all, err := s.fetchGatewayList(ctx, "")
			if err != nil {
				return "", err
			}
			byKind := map[string][]map[string]any{}
			for _, kind := range gatewayKinds {
				if byKind[kind], err = s.fetchGatewayList(ctx, kind); err != nil {
					return "", err
				}
			}
			if mismatch := partitionByKind(all, byKind); mismatch != nil {
				return mismatch.Error(), nil
			}
			return "", nil
		},
		func(mismatch string) bool { return mismatch == "" },
		"waiting for the typed gateway listings to partition the unfiltered one")
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

// updateManagedGateway changes the mutable fields of the scenario's gateway and publishes the
// response. The gateway's identity is read first so a step can check the update left it alone.
func (s *Steps) updateManagedGateway(ctx context.Context, table *godog.Table) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	body := map[string]any{}
	for key, raw := range values {
		if !managedGatewayUpdateKeys[key] {
			return fmt.Errorf("unsupported managed gateway update value %q", key)
		}
		value, err := expandCloudValue(ctx, raw)
		if err != nil {
			return fmt.Errorf("expanding %q: %w", key, err)
		}
		if body[key], err = typedGatewayValue(key, value); err != nil {
			return err
		}
	}
	before, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return fmt.Errorf("reading managed gateway %q before the update: %w", id, err)
	}
	identity := map[string]any{}
	for _, field := range gatewayIdentityFields {
		identity[field] = before[field]
	}
	raw, err := json.Marshal(identity)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyGatewayIdentity, string(raw)); err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodPut, "/managed-gateways/"+url.PathEscape(id), body)
	return err
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

// gatewayFieldValue reads a field of a gateway document; "properties.<name>" reads one property.
func gatewayFieldValue(document map[string]any, field string) any {
	if name, nested := strings.CutPrefix(field, "properties."); nested {
		properties, _ := document["properties"].(map[string]any)
		return properties[name]
	}
	return document[field]
}

// assertManagedGatewayReports waits until the gateway reports every field the table names.
func (s *Steps) assertManagedGatewayReports(ctx context.Context, table *godog.Table) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	expected := map[string]any{}
	for field, raw := range values {
		value, err := expandCloudValue(ctx, raw)
		if err != nil {
			return fmt.Errorf("expanding %q: %w", field, err)
		}
		expected[field] = parseConfigCell(value)
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (string, error) {
			document, err := s.fetchManagedGateway(ctx, id)
			if err != nil {
				return "", err
			}
			var differing []string
			for field, want := range expected {
				if got := gatewayFieldValue(document, field); !sameValue(got, want) {
					differing = append(differing, fmt.Sprintf("%s is %v, want %v", field, got, want))
				}
			}
			sort.Strings(differing)
			return strings.Join(differing, "; "), nil
		},
		func(differing string) bool { return differing == "" },
		"waiting for managed gateway "+id+" to report the updated fields")
}

// assertIdentityUnchanged checks the update left the gateway's identity as it was.
func (s *Steps) assertIdentityUnchanged(ctx context.Context) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	var before map[string]any
	if err := json.Unmarshal([]byte(optionalValue(ctx, keyGatewayIdentity)), &before); err != nil {
		return fmt.Errorf("no identity was recorded before the update: %w", err)
	}
	document, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return err
	}
	for _, field := range gatewayIdentityFields {
		if !sameValue(document[field], before[field]) {
			return fmt.Errorf("the update changed %s from %v to %v", field, before[field], document[field])
		}
	}
	return nil
}

// assertBoundToEnvironment checks the gateway is bound to the environment it was created in: its
// own record and the listing both say so, with a host that carries no scheme.
func (s *Steps) assertBoundToEnvironment(ctx context.Context) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	environment, err := requiredValue(ctx, keyManagedGatewayEnv)
	if err != nil {
		return err
	}
	document, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return err
	}
	if got, _ := document["environment"].(string); got != environment {
		return fmt.Errorf("managed gateway %q reports environment %q, want %q", id, got, environment)
	}
	host, _ := document["host"].(string)
	if strings.TrimSpace(host) == "" || strings.Contains(host, "://") {
		return fmt.Errorf("managed gateway %q reports host %q, want a host without a scheme", id, host)
	}
	if _, ok := document["isDefault"].(bool); !ok {
		return fmt.Errorf("managed gateway %q does not report isDefault", id)
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (string, error) {
			current, err := s.fetchManagedGateway(ctx, id)
			if err != nil {
				return "", err
			}
			if endpoints, _ := current["endpoints"].([]any); len(endpoints) == 0 {
				return "the gateway lists no endpoints derived from its host", nil
			}
			items, err := s.fetchGatewayList(ctx, "")
			if err != nil {
				return "", err
			}
			for _, item := range items {
				if item["id"] == id {
					if item["environment"] != environment {
						return fmt.Sprintf("the listing shows environment %v", item["environment"]), nil
					}
					return "", nil
				}
			}
			return "the gateway is not in the listing", nil
		},
		func(problem string) bool { return problem == "" },
		"waiting for managed gateway "+id+" to be listed for environment "+environment+" with endpoints")
}

// assertEveryCreatedGatewayExists checks every gateway the scenario created can be read.
func (s *Steps) assertEveryCreatedGatewayExists(ctx context.Context) error {
	ids := syntheticGatewayIDs(ctx)
	if len(ids) == 0 {
		return errors.New("the scenario has created no managed gateway")
	}
	for _, id := range ids {
		if _, err := s.fetchManagedGateway(ctx, id); err != nil {
			return fmt.Errorf("managed gateway %q: %w", id, err)
		}
	}
	return nil
}

// assertGatewayListedOnce checks the environment holds exactly one gateway with the scenario's
// display name. Counting by name rather than by id is what shows a repeated request that was
// answered with a second gateway under another id.
func (s *Steps) assertGatewayListedOnce(ctx context.Context) error {
	name, err := requiredValue(ctx, keyManagedGatewayName)
	if err != nil {
		return err
	}
	environment, err := requiredValue(ctx, keyManagedGatewayEnv)
	if err != nil {
		return err
	}
	items, err := s.fetchGatewayList(ctx, "")
	if err != nil {
		return err
	}
	var ids []string
	for _, item := range items {
		if item["displayName"] == name && item["environment"] == environment {
			id, _ := item["id"].(string)
			ids = append(ids, id)
		}
	}
	if len(ids) != 1 {
		return fmt.Errorf("environment %q holds %d managed gateways named %q (%s), want exactly one",
			environment, len(ids), name, strings.Join(ids, ", "))
	}
	return nil
}

// assertGatewayHasItsNameAsID checks the gateway is registered under the id its name gives it,
// which is what a name that was free again produces.
func (s *Steps) assertGatewayHasItsNameAsID(ctx context.Context) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	name, err := requiredValue(ctx, keyManagedGatewayName)
	if err != nil {
		return err
	}
	environment, err := requiredValue(ctx, keyManagedGatewayEnv)
	if err != nil {
		return err
	}
	if want := expectedGatewayID(environment, name, ""); id != want {
		return fmt.Errorf("the gateway is registered as %q, want %q: its name was not free", id, want)
	}
	return nil
}

// defaultGatewayOf finds the gateway an environment resolves to for a kind. At most one gateway
// per environment and kind carries the flag.
func defaultGatewayOf(items []map[string]any, environment, kind string) (string, error) {
	var found []string
	for _, item := range items {
		if item["environment"] != environment || gatewayKind(item) != kind {
			continue
		}
		if isDefault, _ := item["isDefault"].(bool); isDefault {
			id, _ := item["id"].(string)
			found = append(found, id)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", fmt.Errorf("environment %q has no default %s gateway", environment, kind)
	}
	return "", fmt.Errorf("environment %q has %d default %s gateways (%s), want one", environment, len(found), kind, strings.Join(found, ", "))
}

// deleteDefaultGatewayOfSyntheticEnvironment asks for the default regular gateway of the
// scenario's synthetic environment to be deleted and publishes the response.
func (s *Steps) deleteDefaultGatewayOfSyntheticEnvironment(ctx context.Context) error {
	environment, err := requiredValue(ctx, keyEnvironmentName)
	if err != nil {
		return err
	}
	items, err := s.fetchGatewayList(ctx, "")
	if err != nil {
		return err
	}
	id, err := defaultGatewayOf(items, environment, regularGatewayKind)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyDefaultGatewayID, id); err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodDelete, "/managed-gateways/"+url.PathEscape(id), nil)
	return err
}

// assertRefusedDefaultStillExists checks the gateway whose deletion was refused is still there.
func (s *Steps) assertRefusedDefaultStillExists(ctx context.Context) error {
	id, err := requiredValue(ctx, keyDefaultGatewayID)
	if err != nil {
		return err
	}
	document, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return fmt.Errorf("the default gateway %q was refused deletion but cannot be read: %w", id, err)
	}
	if isDefault, _ := document["isDefault"].(bool); !isDefault {
		return fmt.Errorf("gateway %q is no longer its environment's default", id)
	}
	return nil
}

// firstReadStatus checks the status a gateway reports on the first read after it was created: a
// documented one that is neither a failure nor a connected controller.
func firstReadStatus(document map[string]any) error {
	id, _ := document["id"].(string)
	status, _ := document["status"].(string)
	if !gatewayStatuses[status] {
		return fmt.Errorf("managed gateway %q reports status %q on its first read, want a recorded provisioning state", id, status)
	}
	if status == "FAILED" {
		reason, _ := document["statusReason"].(string)
		return fmt.Errorf("managed gateway %q is FAILED on its first read: %s", id, reason)
	}
	return nil
}

// assertFirstReadHasStatus reads the gateway once, straight after it was created.
func (s *Steps) assertFirstReadHasStatus(ctx context.Context) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	document, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return err
	}
	return firstReadStatus(document)
}

// statusProgression records the statuses a gateway reported, in order, without repeats.
type statusProgression []string

// add records a status when it differs from the last one.
func (p *statusProgression) add(status string) {
	if n := len(*p); n == 0 || (*p)[n-1] != status {
		*p = append(*p, status)
	}
}

// check reports why a progression is not a gateway being built: it must never fail, and once
// built it must not go back to being built.
func (p statusProgression) check() error {
	built := false
	for _, status := range p {
		switch status {
		case "FAILED":
			return fmt.Errorf("the gateway reported FAILED (statuses seen: %s)", strings.Join(p, " -> "))
		case "PROVISIONING":
			if built {
				return fmt.Errorf("the gateway went back to PROVISIONING after it was built (statuses seen: %s)", strings.Join(p, " -> "))
			}
		default:
			built = true
		}
	}
	return nil
}

// assertStatusProgressesToBuilt reads the gateway until it stops being built and checks the
// statuses it passed through were those of a build that succeeds.
func (s *Steps) assertStatusProgressesToBuilt(ctx context.Context) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	var seen statusProgression
	err = retry.Await(ctx, retry.Options{Timeout: gatewayBuildTimeout, Interval: 2 * time.Second},
		func(ctx context.Context) (string, error) {
			document, err := s.fetchManagedGateway(ctx, id)
			if err != nil {
				return "", err
			}
			status, _ := document["status"].(string)
			if !gatewayStatuses[status] {
				return "", fmt.Errorf("managed gateway %q reports status %q", id, status)
			}
			seen.add(status)
			if err := seen.check(); err != nil {
				return "", err
			}
			return status, nil
		},
		func(status string) bool { return status != "PROVISIONING" },
		"waiting for managed gateway "+id+" to be built")
	if err != nil {
		return fmt.Errorf("%w (statuses seen: %s)", err, strings.Join(seen, " -> "))
	}
	return nil
}

// assertGatewayNotConnected checks registration alone did not mark the gateway connected.
func (s *Steps) assertGatewayNotConnected(ctx context.Context) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	document, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return err
	}
	if active, _ := document["isActive"].(bool); active {
		return fmt.Errorf("managed gateway %q reports its controller connected straight after registration", id)
	}
	if status, _ := document["status"].(string); status == "ACTIVE" {
		return fmt.Errorf("managed gateway %q reports ACTIVE straight after registration", id)
	}
	return nil
}

// assertGatewayConnected checks the gateway reports its controller connected.
func (s *Steps) assertGatewayConnected(ctx context.Context) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	document, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return err
	}
	if active, _ := document["isActive"].(bool); !active {
		return fmt.Errorf("managed gateway %q does not report its controller connected", id)
	}
	if status, _ := document["status"].(string); status != "ACTIVE" {
		return fmt.Errorf("managed gateway %q is connected but reports status %q", id, status)
	}
	return nil
}

// assertGatewayVersion checks whether the gateway reports a version: none until its controller
// has reported one, and one afterwards.
func (s *Steps) assertGatewayVersion(ctx context.Context, state string) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	document, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return err
	}
	version, _ := document["version"].(string)
	switch state {
	case "no":
		if strings.TrimSpace(version) != "" {
			return fmt.Errorf("managed gateway %q reports version %q before its controller reported one", id, version)
		}
	default:
		if strings.TrimSpace(version) == "" {
			return fmt.Errorf("managed gateway %q still reports no version after its controller connected", id)
		}
	}
	return nil
}

// assertGatewayEndpointAnswers checks the endpoint of every gateway the scenario created answers
// HTTP requests, which is what a gateway whose service is routable does whatever the path.
func (s *Steps) assertGatewayEndpointAnswers(ctx context.Context) error {
	ids := syntheticGatewayIDs(ctx)
	if len(ids) == 0 {
		return errors.New("the scenario has created no managed gateway")
	}
	for _, id := range ids {
		if err := s.awaitEndpointAnswers(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// awaitEndpointAnswers waits until the first approved endpoint of a gateway answers a request.
func (s *Steps) awaitEndpointAnswers(ctx context.Context, id string) error {
	document, err := s.fetchManagedGateway(ctx, id)
	if err != nil {
		return err
	}
	endpoints, _ := document["endpoints"].([]any)
	endpoint, ok := firstApprovedDataPlaneEndpoint(ctx, s, endpoints)
	if !ok {
		return fmt.Errorf("managed gateway %q lists no endpoint that passed the data-plane host check: %v", id, endpoints)
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 5 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := s.client.Do(ctx, httpx.Request{Method: http.MethodGet, URL: endpoint + "/"}, 0, 0)
			if err != nil {
				return nil, retry.Transient(err)
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool {
			return resp != nil && resp.StatusCode < http.StatusInternalServerError
		},
		"waiting for the endpoint of managed gateway "+id+" to answer")
}

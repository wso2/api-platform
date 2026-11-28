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
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// kindSelfHostedGateway is a kind of its own, not cleanup.KindGateway, because the registry keeps
// one deleter per kind name and the two are deleted through different collections
// (/self-hosted-gateways and /managed-gateways): sharing the kind would send one of them to the
// wrong collection, which answers 404 and is swept as already gone. It is deleted with the managed
// gateways.
var kindSelfHostedGateway = cleanup.Kind{Name: "self-hosted-gateway", Order: cleanup.KindGateway.Order}

const (
	// selfHostedGatewaysPath is the collection the cloud console's gateways page drives for
	// gateways the customer runs themselves (portals/cloud-plugins/apip-cloud-ui-gateways).
	selfHostedGatewaysPath = "/self-hosted-gateways"

	// pendingEndpoint is the placeholder a self-hosted gateway is registered with until its
	// own controller dials in and reports a real endpoint. It matches PENDING_ENDPOINT in
	// the console's selfHostedGatewaysApi.ts, which is what the console sends.
	pendingEndpoint = "pending://awaiting-connection"

	// selfHostedFunctionalityType is the plain API gateway kind ("regular", not "ai" or
	// "event").
	selfHostedFunctionalityType = "regular"

	// activeTokenStatus is the only token status the console counts as active; a token with
	// no status is active too, as the console treats it.
	activeTokenStatus = "active"

	// selfHostedEnvironmentProperty is the property the cloud sets to a self-hosted gateway's
	// bound environment, which the portals read it from.
	selfHostedEnvironmentProperty = "environment"
)

// registerSelfHostedGatewaySteps binds the steps that register a self-hosted gateway, manage its
// registration tokens and delete it.
func (s *Steps) registerSelfHostedGatewaySteps(sc stepRegistrar) {
	sc.Step(`^I create a synthetic self-hosted APIP gateway$`, s.createSelfHostedGateway)
	sc.Step(`^I create a synthetic self-hosted APIP gateway with:$`, s.createSelfHostedGatewayWith)
	sc.Step(`^the self-hosted APIP gateway should be retrievable and awaiting connection$`, s.getSelfHostedGatewayAwaitingConnection)
	sc.Step(`^the self-hosted APIP gateway should be retrievable and inactive, with the endpoints and properties it was created with$`,
		s.assertSelfHostedGatewayKeepsItsValues)
	sc.Step(`^the self-hosted APIP gateway should name the console user as its creator$`, s.assertSelfHostedGatewayCreator)
	sc.Step(`^the self-hosted APIP gateway should be listed$`, s.assertSelfHostedGatewayListed)
	sc.Step(`^I generate (?:a|another) registration token for the self-hosted APIP gateway$`, s.generateRegistrationToken)
	sc.Step(`^I reconfigure the registration token of the self-hosted APIP gateway$`, s.reconfigureRegistrationToken)
	sc.Step(`^I revoke the first registration token of the self-hosted APIP gateway(?: again)?$`, s.revokeFirstRegistrationToken)
	sc.Step(`^the self-hosted APIP gateway should have exactly (\d+) active registration tokens?$`, s.assertActiveRegistrationTokens)
	sc.Step(`^the revoked registration tokens should no longer be active$`, s.assertRevokedTokensInactive)
	sc.Step(`^the last registration token of the self-hosted APIP gateway should still be active$`, s.assertLastTokenActive)
	sc.Step(`^I delete the synthetic self-hosted APIP gateway$`, s.deleteSelfHostedGateway)
	sc.Step(`^the self-hosted APIP gateway should eventually be absent$`, s.verifySelfHostedGatewayDeleted)
	s.registerLocalGatewaySteps(sc)
}

func selfHostedGatewayPath(id string) string {
	return selfHostedGatewaysPath + "/" + url.PathEscape(id)
}

func selfHostedGatewayTokensPath(id string) string {
	return selfHostedGatewayPath(id) + "/tokens"
}

// createSelfHostedGateway registers a gateway in the selected environment the way the
// console does: no endpoint yet, only the pending placeholder.
func (s *Steps) createSelfHostedGateway(ctx context.Context) error {
	return s.registerSelfHostedGateway(ctx, []string{pendingEndpoint}, nil)
}

// createSelfHostedGatewayWith registers a gateway with the endpoints (a JSON array of URLs) and
// properties (a JSON object) the table gives, and keeps both for the assertions that follow.
func (s *Steps) createSelfHostedGatewayWith(ctx context.Context, table *godog.Table) error {
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	for key := range values {
		if key != "endpoints" && key != "properties" {
			return fmt.Errorf("unsupported self-hosted gateway value %q (want endpoints or properties)", key)
		}
	}
	var endpoints []string
	if err := json.Unmarshal([]byte(values["endpoints"]), &endpoints); err != nil || len(endpoints) == 0 {
		return fmt.Errorf("endpoints must be a non-empty JSON array of URLs, got %q", values["endpoints"])
	}
	properties := map[string]any{}
	if raw, present := values["properties"]; present {
		if err := json.Unmarshal([]byte(raw), &properties); err != nil {
			return fmt.Errorf("properties must be a JSON object, got %q", raw)
		}
	}
	if err := tcontext.Set(ctx, keySelfHostedGatewayEndpoints, values["endpoints"]); err != nil {
		return err
	}
	encoded, err := json.Marshal(properties)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keySelfHostedGatewayProperties, string(encoded)); err != nil {
		return err
	}
	return s.registerSelfHostedGateway(ctx, endpoints, properties)
}

// registerSelfHostedGateway registers a regular self-hosted gateway in the selected environment.
func (s *Steps) registerSelfHostedGateway(ctx context.Context, endpoints []string, properties map[string]any) error {
	environment, err := requiredValue(ctx, keyEnvironmentName)
	if err != nil {
		return err
	}
	// "shgw" keeps the handle short: Unique() adds 11 characters, and the gateway handle is
	// capped at 40 (platform-api's CreateGatewayRequest.id). If the handle is derived from the
	// environment and the display name, as the managed-gateway one is, a long base would not fit.
	if err := generateResourceAndStore(ctx, "shgw", keySelfHostedGatewayID); err != nil {
		return err
	}
	name, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	// Register a best-guess id before the call, as the managed-gateway step does, so a create
	// that succeeds with an undecodable body still has something to sweep. A wrong guess is
	// harmless: its delete answers 404, which the sweep treats as already gone.
	cleanupID := environment + "-" + name
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: kindSelfHostedGateway, ID: cleanupID, Actor: resourceActor,
		Description: "self-hosted gateway " + cleanupID,
	}); err != nil {
		return fmt.Errorf("registering self-hosted gateway %q for cleanup: %w", cleanupID, err)
	}
	body := map[string]any{
		"displayName":       name,
		"description":       "Created and deleted by the APIP cloud integration test. Safe to delete.",
		"environment":       environment,
		"functionalityType": selfHostedFunctionalityType,
		"endpoints":         endpoints,
	}
	if len(properties) > 0 {
		body["properties"] = properties
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, selfHostedGatewaysPath, body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	id, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating APIP self-hosted gateway: %w", err)
	}
	if id != cleanupID {
		// Register the id the platform answered with before dropping the guess, so the gateway
		// is never left without a registration. If that fails, nothing would clean it up, so
		// delete it now and fail the step.
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: kindSelfHostedGateway, ID: id, Actor: resourceActor,
			Description: "self-hosted gateway " + id,
		}); err != nil {
			err = fmt.Errorf("registering self-hosted gateway %q for cleanup: %w", id, err)
			return errors.Join(err, s.removeUnregisteredSelfHostedGateway(ctx, id))
		}
		deregister(ctx, kindSelfHostedGateway, cleanupID)
	}
	return tcontext.Set(ctx, keySelfHostedGatewayID, id)
}

// removeUnregisteredSelfHostedGateway is the best-effort delete of a gateway that could not be
// registered for cleanup. It returns nil when the gateway is gone, and an error naming the
// gateway when it may still exist.
func (s *Steps) removeUnregisteredSelfHostedGateway(ctx context.Context, id string) error {
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodDelete, selfHostedGatewayPath(id), nil)
	if err != nil {
		return fmt.Errorf("deleting unregistered self-hosted gateway %q: %w", id, err)
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("deleting unregistered self-hosted gateway %q: %w", id, responseError(resp))
	}
	return nil
}

// getSelfHostedGatewayAwaitingConnection reads the gateway back by id. It also proves the
// by-id route exists, which is what makes the later "eventually absent" 404 meaningful
// rather than the answer of a route that was never there.
func (s *Steps) getSelfHostedGatewayAwaitingConnection(ctx context.Context) error {
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	environment, err := requiredValue(ctx, keyEnvironmentName)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodGet, selfHostedGatewayPath(id), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	var document map[string]any
	if err := json.Unmarshal(resp.Body, &document); err != nil {
		return fmt.Errorf("decoding APIP self-hosted gateway: %w (%s)", err, resp.Describe())
	}
	return checkAwaitingConnection(document, id, environment)
}

// readSelfHostedGateway reads the scenario's self-hosted gateway by id.
func (s *Steps) readSelfHostedGateway(ctx context.Context) (map[string]any, string, error) {
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return nil, "", err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodGet, selfHostedGatewayPath(id), nil)
	if err != nil {
		return nil, id, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, id, responseError(resp)
	}
	var document map[string]any
	if err := json.Unmarshal(resp.Body, &document); err != nil {
		return nil, id, fmt.Errorf("decoding APIP self-hosted gateway: %w (%s)", err, resp.Describe())
	}
	return document, id, nil
}

// assertSelfHostedGatewayKeepsItsValues checks a gateway created with endpoints and properties
// reads back inactive with those endpoints in order and those properties, beside the
// properties.environment the cloud sets to the bound environment.
func (s *Steps) assertSelfHostedGatewayKeepsItsValues(ctx context.Context) error {
	document, id, err := s.readSelfHostedGateway(ctx)
	if err != nil {
		return err
	}
	environment, err := requiredValue(ctx, keyEnvironmentName)
	if err != nil {
		return err
	}
	var wantEndpoints []string
	rawEndpoints, err := requiredValue(ctx, keySelfHostedGatewayEndpoints)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(rawEndpoints), &wantEndpoints); err != nil {
		return err
	}
	wantProperties := map[string]any{}
	rawProperties, err := requiredValue(ctx, keySelfHostedGatewayProperties)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(rawProperties), &wantProperties); err != nil {
		return err
	}
	if got, _ := document["id"].(string); got != id {
		return fmt.Errorf("self-hosted gateway id = %q, want %q", got, id)
	}
	if got, _ := document["environment"].(string); got != environment {
		return fmt.Errorf("self-hosted gateway %q environment = %q, want %q", id, got, environment)
	}
	if active, ok := document["isActive"].(bool); !ok || active {
		return fmt.Errorf("self-hosted gateway %q isActive = %v, want false: no controller has connected", id, document["isActive"])
	}
	if got, _ := document["functionalityType"].(string); got != selfHostedFunctionalityType {
		return fmt.Errorf("self-hosted gateway %q functionalityType = %q, want %q", id, got, selfHostedFunctionalityType)
	}
	endpoints, _ := document["endpoints"].([]any)
	if len(endpoints) != len(wantEndpoints) {
		return fmt.Errorf("self-hosted gateway %q endpoints = %v, want %v", id, document["endpoints"], wantEndpoints)
	}
	for i, want := range wantEndpoints {
		if !reflect.DeepEqual(endpoints[i], want) {
			return fmt.Errorf("self-hosted gateway %q endpoints = %v, want %v", id, document["endpoints"], wantEndpoints)
		}
	}
	properties, _ := document["properties"].(map[string]any)
	for key, want := range wantProperties {
		// Structurally: a property may hold an object or an array, which != cannot compare.
		if got, present := properties[key]; !present || !reflect.DeepEqual(got, want) {
			return fmt.Errorf("self-hosted gateway %q property %q = %v, want %v", id, key, properties[key], want)
		}
	}
	if got, _ := properties[selfHostedEnvironmentProperty].(string); got != environment {
		return fmt.Errorf("self-hosted gateway %q property %q = %v, want its environment %q",
			id, selfHostedEnvironmentProperty, got, environment)
	}
	return nil
}

// assertSelfHostedGatewayCreator checks the gateway names the console user as its creator.
func (s *Steps) assertSelfHostedGatewayCreator(ctx context.Context) error {
	document, id, err := s.readSelfHostedGateway(ctx)
	if err != nil {
		return err
	}
	want, err := consoleUserDisplayName(ctx)
	if err != nil {
		return err
	}
	if got, _ := document["createdBy"].(string); got != want {
		return fmt.Errorf("self-hosted gateway %q createdBy = %q, want the console user %q", id, got, want)
	}
	return nil
}

// assertSelfHostedGatewayListed checks the self-hosted gateway list, which holds every
// self-hosted gateway of the organisation, names the scenario's gateway.
func (s *Steps) assertSelfHostedGatewayListed(ctx context.Context) error {
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodGet, selfHostedGatewaysPath, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	items, err := listItems(resp)
	if err != nil {
		return fmt.Errorf("decoding self-hosted gateways: %w", err)
	}
	for _, item := range items {
		if got, _ := item["id"].(string); got == id {
			return nil
		}
	}
	return fmt.Errorf("self-hosted gateway %q is not listed: %s", id, resp.Describe())
}

// checkAwaitingConnection asserts the state of a self-hosted gateway no controller has
// connected to yet: inactive, in its environment, with only the pending endpoint.
func checkAwaitingConnection(document map[string]any, id, environment string) error {
	if got, _ := document["id"].(string); got != id {
		return fmt.Errorf("self-hosted gateway id = %q, want %q", got, id)
	}
	if got, _ := document["environment"].(string); got != environment {
		return fmt.Errorf("self-hosted gateway %q environment = %q, want %q", id, got, environment)
	}
	active, ok := document["isActive"].(bool)
	if !ok {
		return fmt.Errorf("self-hosted gateway %q has no boolean isActive field", id)
	}
	if active {
		return fmt.Errorf("self-hosted gateway %q is active, but no controller has connected to it", id)
	}
	endpoints, ok := document["endpoints"].([]any)
	if !ok || len(endpoints) != 1 || endpoints[0] != pendingEndpoint {
		return fmt.Errorf("self-hosted gateway %q endpoints = %v, want [%s]", id, document["endpoints"], pendingEndpoint)
	}
	return nil
}

func (s *Steps) generateRegistrationToken(ctx context.Context) error {
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	return s.mintRegistrationToken(ctx, id)
}

// mintRegistrationToken mints a token and checks it came back, without the token's value
// ever leaving this function.
func (s *Steps) mintRegistrationToken(ctx context.Context, gatewayID string) error {
	_, err := s.mintRegistrationTokenValue(ctx, gatewayID)
	return err
}

// mintRegistrationTokenValue mints a token and returns its plain-text value, which the caller
// must hand only to the gateway it is for. The response is deliberately neither published to
// the funnel nor described in an error: the plain-text token appears only in this response.
func (s *Steps) mintRegistrationTokenValue(ctx context.Context, gatewayID string) (string, error) {
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodPost, selfHostedGatewayTokensPath(gatewayID), map[string]any{})
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusCreated {
		if resp.StatusCode >= http.StatusBadRequest {
			return "", responseError(resp) // an error body holds no minted token
		}
		return "", fmt.Errorf("minting a registration token: %s %s -> %d, want %d (body withheld: it may hold a token)",
			resp.Method, resp.URL, resp.StatusCode, http.StatusCreated)
	}
	tokenID, token, err := parseMintedToken(resp.Body)
	if err != nil {
		return "", fmt.Errorf("minting a registration token for self-hosted gateway %q: %w", gatewayID, err)
	}
	if err := recordIssuedToken(ctx, tokenID); err != nil {
		return "", err
	}
	return token, nil
}

// recordIssuedToken appends a minted token's id, never its value, to the scenario's issued ids.
func recordIssuedToken(ctx context.Context, tokenID string) error {
	issued := optionalValue(ctx, keyIssuedTokenIDs)
	if issued != "" {
		issued += ","
	}
	return tcontext.Set(ctx, keyIssuedTokenIDs, issued+tokenID)
}

// issuedTokenIDs returns the ids of the tokens the scenario issued, in issue order.
func issuedTokenIDs(ctx context.Context) ([]string, error) {
	issued := optionalValue(ctx, keyIssuedTokenIDs)
	if issued == "" {
		return nil, fmt.Errorf("no registration token was issued in this scenario")
	}
	return strings.Split(issued, ","), nil
}

// revokeFirstRegistrationToken revokes the first token the scenario issued, by id, and records it
// as revoked. Revoking it again is answered the same way.
func (s *Steps) revokeFirstRegistrationToken(ctx context.Context) error {
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	issued, err := issuedTokenIDs(ctx)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodDelete,
		selfHostedGatewayTokensPath(id)+"/"+url.PathEscape(issued[0]), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return responseError(resp)
	}
	return tcontext.Set(ctx, keyRevokedTokenIDs, issued[0])
}

// assertLastTokenActive checks the last token the scenario issued is still active.
func (s *Steps) assertLastTokenActive(ctx context.Context) error {
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	issued, err := issuedTokenIDs(ctx)
	if err != nil {
		return err
	}
	last := issued[len(issued)-1]
	active, err := s.activeRegistrationTokenIDs(ctx, id)
	if err != nil {
		return err
	}
	if !slices.Contains(active, last) {
		return fmt.Errorf("registration token %q of self-hosted gateway %q is not active; active: %v", last, id, active)
	}
	return nil
}

// mintedTokenID checks a mint response carries both a token and its id, and returns the id.
// Its errors never quote the body.
func mintedTokenID(body []byte) (string, error) {
	id, _, err := parseMintedToken(body)
	return id, err
}

// parseMintedToken returns the id and the plain-text value of a minted token. Its errors never
// quote the body.
func parseMintedToken(body []byte) (id, token string, err error) {
	var document struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return "", "", fmt.Errorf("the token response is not a JSON object")
	}
	if strings.TrimSpace(document.Token) == "" {
		return "", "", fmt.Errorf("the token response has no token")
	}
	if strings.TrimSpace(document.ID) == "" {
		return "", "", fmt.Errorf("the token response has no token id")
	}
	return document.ID, strings.TrimSpace(document.Token), nil
}

// reconfigureRegistrationToken does what the console's "reconfigure" does: revoke every
// active token, then mint a new one. It records the revoked ids for the next assertion.
func (s *Steps) reconfigureRegistrationToken(ctx context.Context) error {
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	active, err := s.activeRegistrationTokenIDs(ctx, id)
	if err != nil {
		return err
	}
	// With nothing to revoke, "the revoked tokens are no longer active" would pass vacuously.
	if len(active) == 0 {
		return fmt.Errorf("self-hosted gateway %q has no active registration token to reconfigure", id)
	}
	for _, tokenID := range active {
		resp, err := s.doAuthenticated(ctx, http.MethodDelete,
			selfHostedGatewayTokensPath(id)+"/"+url.PathEscape(tokenID), nil)
		if err != nil {
			return err
		}
		// 204, as apip-platform-api's /self-hosted-gateways contract documents it — not the
		// 200 with a message body that platform-api's own /gateways/{id}/tokens/{id} returns.
		if resp.StatusCode != http.StatusNoContent {
			return responseError(resp)
		}
	}
	if err := tcontext.Set(ctx, keyRevokedTokenIDs, strings.Join(active, ",")); err != nil {
		return err
	}
	return s.mintRegistrationToken(ctx, id)
}

func (s *Steps) assertActiveRegistrationTokens(ctx context.Context, want int) error {
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	active, err := s.activeRegistrationTokenIDs(ctx, id)
	if err != nil {
		return err
	}
	if len(active) != want {
		return fmt.Errorf("self-hosted gateway %q has %d active registration tokens %v, want %d", id, len(active), active, want)
	}
	return nil
}

func (s *Steps) assertRevokedTokensInactive(ctx context.Context) error {
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	joined, err := requiredValue(ctx, keyRevokedTokenIDs)
	if err != nil {
		return err
	}
	active, err := s.activeRegistrationTokenIDs(ctx, id)
	if err != nil {
		return err
	}
	if still := stillActive(strings.Split(joined, ","), active); len(still) > 0 {
		return fmt.Errorf("revoked registration tokens %v of self-hosted gateway %q are still active", still, id)
	}
	return nil
}

// stillActive returns the revoked ids that appear among the active ones.
func stillActive(revoked, active []string) []string {
	var still []string
	for _, tokenID := range revoked {
		if slices.Contains(active, tokenID) {
			still = append(still, tokenID)
		}
	}
	return still
}

func (s *Steps) activeRegistrationTokenIDs(ctx context.Context, gatewayID string) ([]string, error) {
	resp, err := s.doAuthenticated(ctx, http.MethodGet, selfHostedGatewayTokensPath(gatewayID), nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, responseError(resp)
	}
	items, err := listItems(resp)
	if err != nil {
		return nil, fmt.Errorf("decoding registration tokens of self-hosted gateway %q: %w", gatewayID, err)
	}
	return activeTokenIDs(items), nil
}

// activeTokenIDs applies the console's rule: a token is active when its status is "active"
// or absent.
func activeTokenIDs(items []map[string]any) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		id, _ := item["id"].(string)
		if strings.TrimSpace(id) == "" {
			continue
		}
		if status, present := item["status"]; present && status != activeTokenStatus {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func (s *Steps) deleteSelfHostedGateway(ctx context.Context) error {
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodDelete, selfHostedGatewayPath(id), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return responseError(resp)
	}
	if resp.StatusCode == http.StatusNotFound {
		deregister(ctx, kindSelfHostedGateway, id)
	}
	return nil
}

func (s *Steps) verifySelfHostedGatewayDeleted(ctx context.Context) error {
	id, err := requiredValue(ctx, keySelfHostedGatewayID)
	if err != nil {
		return err
	}
	if err := s.awaitNotFound(ctx, selfHostedGatewayPath(id), "self-hosted gateway "+id); err != nil {
		return err
	}
	deregister(ctx, kindSelfHostedGateway, id)
	return nil
}

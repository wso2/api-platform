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
	"net/url"
	"slices"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

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
)

func selfHostedGatewayPath(id string) string {
	return selfHostedGatewaysPath + "/" + url.PathEscape(id)
}

func selfHostedGatewayTokensPath(id string) string {
	return selfHostedGatewayPath(id) + "/tokens"
}

// createSelfHostedGateway registers a gateway in the selected environment the way the
// console does: no endpoint yet, only the pending placeholder.
func (s *Steps) createSelfHostedGateway(ctx context.Context) error {
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
		Kind: cleanup.KindSelfHostedGateway, ID: cleanupID, Actor: resourceActor,
		Description: "self-hosted gateway " + cleanupID,
	}); err != nil {
		return fmt.Errorf("registering self-hosted gateway %q for cleanup: %w", cleanupID, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, selfHostedGatewaysPath, map[string]any{
		"displayName":       name,
		"description":       "Created and deleted by the APIP cloud integration test. Safe to delete.",
		"environment":       environment,
		"functionalityType": selfHostedFunctionalityType,
		"endpoints":         []string{pendingEndpoint},
	})
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
		deregister(ctx, cleanup.KindSelfHostedGateway, cleanupID)
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: cleanup.KindSelfHostedGateway, ID: id, Actor: resourceActor,
			Description: "self-hosted gateway " + id,
		}); err != nil {
			return fmt.Errorf("registering self-hosted gateway %q for cleanup: %w", id, err)
		}
	}
	return tcontext.Set(ctx, keySelfHostedGatewayID, id)
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
// ever leaving this function. The response is deliberately neither published to the
// funnel nor described in an error: the plain-text token appears only in this response.
func (s *Steps) mintRegistrationToken(ctx context.Context, gatewayID string) error {
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodPost, selfHostedGatewayTokensPath(gatewayID), map[string]any{})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		if resp.StatusCode >= http.StatusBadRequest {
			return responseError(resp) // an error body holds no minted token
		}
		return fmt.Errorf("minting a registration token: %s %s -> %d, want %d (body withheld: it may hold a token)",
			resp.Method, resp.URL, resp.StatusCode, http.StatusCreated)
	}
	if _, err := mintedTokenID(resp.Body); err != nil {
		return fmt.Errorf("minting a registration token for self-hosted gateway %q: %w", gatewayID, err)
	}
	return nil
}

// mintedTokenID checks a mint response carries both a token and its id, and returns the id.
// Its errors never quote the body.
func mintedTokenID(body []byte) (string, error) {
	var document struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return "", fmt.Errorf("the token response is not a JSON object")
	}
	if strings.TrimSpace(document.Token) == "" {
		return "", fmt.Errorf("the token response has no token")
	}
	if strings.TrimSpace(document.ID) == "" {
		return "", fmt.Errorf("the token response has no token id")
	}
	return document.ID, nil
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
		deregister(ctx, cleanup.KindSelfHostedGateway, id)
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
	deregister(ctx, cleanup.KindSelfHostedGateway, id)
	return nil
}

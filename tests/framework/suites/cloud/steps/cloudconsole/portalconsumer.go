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

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	// portalAPIBasePath is where a portal serves its REST API.
	portalAPIBasePath = portalBasePath + "/api/v0.9"
	// The portal keeps a session's CSRF token in portalCSRFCookie, and a mutating call made with
	// the session repeats it in portalCSRFHeader.
	portalCSRFCookie = "XSRF-TOKEN"
	portalCSRFHeader = "X-CSRF-Token"

	// consumerActor creates what the runner's consumer owns on the portal.
	consumerActor = "portal-consumer"
	// consumerResourceDescription marks what the consumer creates as safe to delete.
	consumerResourceDescription = "Created and deleted by the APIP cloud integration test. Safe to delete."

	keyConsumerApplicationID     = "cloudPortalApplicationID"
	keyConsumerSubscriptionID    = "cloudPortalSubscriptionID"
	keyConsumerSubscriptionToken = "cloudPortalSubscriptionToken"
	// keyPreviousSubscriptionToken holds the token a subscription had before it was regenerated.
	keyPreviousSubscriptionToken = "cloudPreviousSubscriptionToken"
	keyConsumerAPIKeyID          = "cloudPortalAPIKeyID"
	// keyPreviousAPIKey holds the value an API key had before the consumer regenerated it.
	keyPreviousAPIKey = "cloudPreviousAPIKey"
	// The consumer's OAuth2 keys: the client's generated name, the portal's key id, the client
	// secret the portal shows once, and the access token issued with them.
	keyConsumerOAuth2ClientName = "cloudPortalOAuth2ClientName"
	keyConsumerOAuth2KeyID      = "cloudPortalOAuth2KeyID"
	keyConsumerOAuth2Secret     = "cloudPortalOAuth2Secret"
	keyAccessToken              = "cloudAccessToken"
)

// Cleanup kinds of what the consumer creates. Subscriptions and keys are registered per API before
// the call that creates them, and cleanup removes whatever the consumer has for that API, so one
// whose answer was lost is removed too. They go before the application a key can name, and all
// three before the publication: the portal does not withdraw an API that still has subscriptions
// or active keys. kindConsumerLeftovers is the runner's last sweep of the consumer's portal: it
// removes whatever a failed scenario cleanup left.
var (
	kindConsumerLeftovers     = cleanup.Kind{Name: "portal-consumer-leftovers", Order: 5}
	kindConsumerSubscriptions = cleanup.Kind{Name: "portal-subscriptions", Order: 10}
	kindConsumerAPIKeys       = cleanup.Kind{Name: "portal-api-keys", Order: 15}
	kindConsumerOAuth2Keys    = cleanup.Kind{Name: "portal-oauth2-keys", Order: 16}
	kindConsumerApplication   = cleanup.Kind{Name: "portal-application", Order: 20}
)

// errNoConsumerSession is a call made without a signed-in consumer's session.
var errNoConsumerSession = errors.New("the consumer's portal session holds no CSRF token; sign the consumer in first")

// listPageSize is how many items a cleanup sweep reads per page, the portal's largest page.
const listPageSize = 100

// registerPortalConsumerSteps binds the steps the signed-in consumer takes on the portal's REST API.
func (s *Steps) registerPortalConsumerSteps(sc stepRegistrar) {
	sc.Step(`^the consumer creates a synthetic application on the managed API Portal$`, s.createConsumerApplication)
	sc.Step(`^the consumer reads the synthetic application on the managed API Portal$`, s.readConsumerApplication)
	sc.Step(`^the consumer updates the synthetic application on the managed API Portal with:$`, s.updateConsumerApplication)
	sc.Step(`^the consumer deletes the synthetic application on the managed API Portal$`, s.deleteConsumerApplication)
	sc.Step(`^the consumer subscribes to the synthetic REST API on the managed API Portal with (the|the other) synthetic subscription plan$`, s.subscribeConsumer)
	sc.Step(`^the consumer reads the synthetic subscription on the managed API Portal$`, s.readConsumerSubscription)
	sc.Step(`^the consumer moves the synthetic subscription to (the|the other) synthetic subscription plan on the managed API Portal$`, s.changeConsumerPlan)
	sc.Step(`^the consumer sets the synthetic subscription (ACTIVE|INACTIVE) on the managed API Portal$`, s.setConsumerSubscriptionStatus)
	sc.Step(`^the consumer regenerates the token of the synthetic subscription on the managed API Portal$`, s.regenerateConsumerSubscriptionToken)
	sc.Step(`^the synthetic subscription's token should be (unchanged|replaced)$`, s.assertConsumerSubscriptionToken)
	sc.Step(`^the consumer unsubscribes from the synthetic REST API on the managed API Portal$`, s.unsubscribeConsumer)
	sc.Step(`^the consumer generates an API key for the synthetic REST API on the managed API Portal$`, s.generateConsumerAPIKey)
	sc.Step(`^the consumer regenerates the API key on the managed API Portal$`, s.regenerateConsumerAPIKey)
	sc.Step(`^the consumer revokes the API key on the managed API Portal$`, s.revokeConsumerAPIKey)
	sc.Step(`^the consumer (associates the API key with|dissociates the API key from) the synthetic application on the managed API Portal$`, s.changeConsumerAPIKeyApplication)
	sc.Step(`^the consumer creates OAuth2 keys on the managed API Portal$`, s.createConsumerOAuth2Keys)
	sc.Step(`^the consumer (associates the OAuth2 keys with|dissociates the OAuth2 keys from) the synthetic application on the managed API Portal$`, s.changeConsumerOAuth2KeysApplication)
	sc.Step(`^the consumer generates an access token with the OAuth2 keys on the managed API Portal$`, s.generateConsumerAccessToken)
	sc.Step(`^the consumer's (applications|subscriptions|API keys|API keys for the synthetic REST API|API keys of the synthetic application|OAuth2 keys|OAuth2 keys of the synthetic application) on the managed API Portal should (include|not include) the (synthetic application|synthetic subscription|API key|OAuth2 keys)(?: as (ACTIVE|REVOKED))?$`, s.assertConsumerListing)
}

// apiRequest calls the portal's REST API with the consumer's session. A call that changes
// something repeats the session's CSRF token, as the portal's own pages do.
func (b *portalBrowser) apiRequest(ctx context.Context, method, path string, body any) (*httpx.Response, error) {
	target, err := b.portal.Parse(portalAPIBasePath + path)
	if err != nil {
		return nil, fmt.Errorf("the portal API path %q does not parse: %w", path, err)
	}
	if !sameOrigin(target, b.portal) {
		return nil, fmt.Errorf("the portal API request would leave the portal for %s", target.Redacted())
	}
	headers := map[string]string{"Accept": "application/json"}
	if method != http.MethodGet {
		token := b.cookie(target, portalCSRFCookie)
		if token == "" {
			return nil, errNoConsumerSession
		}
		headers[portalCSRFHeader] = token
	}
	var raw []byte
	if body != nil {
		if raw, err = json.Marshal(body); err != nil {
			return nil, fmt.Errorf("encoding the request to %s: %w", path, err)
		}
	}
	return b.sendWith(ctx, method, target.String(), raw, headers)
}

// cookie returns the value of the named cookie the browser holds for target, or "".
func (b *portalBrowser) cookie(target *url.URL, name string) string {
	for _, cookie := range b.jar.Cookies(target) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

// consumerCall makes one call as the consumer and publishes the answer for the assertion steps.
func (s *Steps) consumerCall(ctx context.Context, method, path string, body any) (*httpx.Response, error) {
	browser, err := scenarioBrowser(ctx)
	if err != nil {
		return nil, err
	}
	httpx.ClearPublished(ctx)
	resp, err := browser.apiRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	return resp, s.publishPortalAnswer(ctx, resp)
}

// consumerSecretCall makes one call whose answer carries an API key. The answer is not published,
// so the key never reaches a step's response slot, and an earlier answer cannot stand in for it.
func consumerSecretCall(ctx context.Context, method, path string, body any) (*httpx.Response, error) {
	browser, err := scenarioBrowser(ctx)
	if err != nil {
		return nil, err
	}
	httpx.ClearPublished(ctx)
	return browser.apiRequest(ctx, method, path, body)
}

// registerConsumerResource registers what the consumer is about to create, once: a scenario that
// subscribes or generates a key twice for one API keeps one registration of the API's items.
func registerConsumerResource(ctx context.Context, resource cleanup.Resource) error {
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	for _, pending := range registry.Pending() {
		if pending.Kind.Name == resource.Kind.Name && pending.ID == resource.ID {
			return nil
		}
	}
	return registry.Register(resource)
}

func applicationPath(id string) string { return "/applications/" + url.PathEscape(id) }

func subscriptionPath(id string) string { return "/subscriptions/" + url.PathEscape(id) }

func apiKeysPath(api string) string { return "/apis/" + url.PathEscape(api) + "/api-keys" }

// applicationBody is an application create or update. A create names the handle; an update
// changes the fields the table names and keeps the handle.
func applicationBody(id string, create bool, overrides map[string]string) (map[string]any, error) {
	body := map[string]any{"displayName": publicationTitle + id, "description": consumerResourceDescription}
	if create {
		body["id"] = id
	}
	for key, value := range overrides {
		switch key {
		case "displayName", "description":
			body[key] = value
		default:
			return nil, fmt.Errorf("unsupported application field %q", key)
		}
	}
	return body, nil
}

func (s *Steps) createConsumerApplication(ctx context.Context) error {
	if _, err := scenarioBrowser(ctx); err != nil {
		return err
	}
	if err := generateResourceAndStore(ctx, "syn-app", keyConsumerApplicationID); err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyConsumerApplicationID)
	if err != nil {
		return err
	}
	body, err := applicationBody(id, true, nil)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: kindConsumerApplication, ID: id, Actor: consumerActor, Description: "portal application " + id,
	}); err != nil {
		return fmt.Errorf("registering portal application %q for cleanup: %w", id, err)
	}
	resp, err := s.consumerCall(ctx, http.MethodPost, "/applications", body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		if refusedCreate(resp) {
			deregister(ctx, kindConsumerApplication, id)
		}
		return responseError(resp)
	}
	return nil
}

func (s *Steps) readConsumerApplication(ctx context.Context) error {
	id, err := requiredValue(ctx, keyConsumerApplicationID)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodGet, applicationPath(id), nil)
	return err
}

func (s *Steps) updateConsumerApplication(ctx context.Context, table *godog.Table) error {
	id, err := requiredValue(ctx, keyConsumerApplicationID)
	if err != nil {
		return err
	}
	overrides, err := tableValues(table)
	if err != nil {
		return err
	}
	for key, value := range overrides {
		if overrides[key], err = expandCloudValue(ctx, value); err != nil {
			return err
		}
	}
	body, err := applicationBody(id, false, overrides)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodPut, applicationPath(id), body)
	return err
}

func (s *Steps) deleteConsumerApplication(ctx context.Context) error {
	id, err := requiredValue(ctx, keyConsumerApplicationID)
	if err != nil {
		return err
	}
	resp, err := s.consumerCall(ctx, http.MethodDelete, applicationPath(id), nil)
	if err != nil {
		return err
	}
	if resp.Succeeded() {
		deregister(ctx, kindConsumerApplication, id)
	}
	return nil
}

// planKeyFor resolves "the" or "the other" synthetic subscription plan to its state key.
func planKeyFor(which string) string {
	if which == "the other" {
		return keySecondPlanID
	}
	return keyPlanID
}

// subscribeConsumer subscribes the consumer to the synthetic REST API and leaves the answer for
// the status assertion. The consumer's subscriptions to the API are registered for cleanup first.
func (s *Steps) subscribeConsumer(ctx context.Context, which string) error {
	api, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	plan, err := requiredValue(ctx, planKeyFor(which))
	if err != nil {
		return err
	}
	if _, err := scenarioBrowser(ctx); err != nil {
		return err
	}
	if err := registerConsumerResource(ctx, cleanup.Resource{
		Kind: kindConsumerSubscriptions, ID: api, Actor: consumerActor, Description: "the consumer's subscriptions to " + api,
	}); err != nil {
		return fmt.Errorf("registering the consumer's subscriptions to %q for cleanup: %w", api, err)
	}
	resp, err := s.consumerCall(ctx, http.MethodPost, "/subscriptions",
		map[string]any{"artifactId": api, "subscriptionPlanId": plan})
	if err != nil || resp.StatusCode != http.StatusCreated {
		return err
	}
	id, err := stringField(resp, "subscriptionId")
	if err != nil {
		return fmt.Errorf("the portal created a subscription it does not identify: %w", err)
	}
	if err := tcontext.Set(ctx, keyConsumerSubscriptionID, id); err != nil {
		return err
	}
	token, err := stringField(resp, "subscriptionToken")
	if err != nil {
		return fmt.Errorf("the portal created a subscription without a token: %w", err)
	}
	return tcontext.Set(ctx, keyConsumerSubscriptionToken, token)
}

func (s *Steps) readConsumerSubscription(ctx context.Context) error {
	id, err := requiredValue(ctx, keyConsumerSubscriptionID)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodGet, subscriptionPath(id), nil)
	return err
}

func (s *Steps) changeConsumerPlan(ctx context.Context, which string) error {
	id, err := requiredValue(ctx, keyConsumerSubscriptionID)
	if err != nil {
		return err
	}
	plan, err := requiredValue(ctx, planKeyFor(which))
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodPost, subscriptionPath(id)+"/change-plan", map[string]any{"planId": plan})
	return err
}

func (s *Steps) setConsumerSubscriptionStatus(ctx context.Context, status string) error {
	id, err := requiredValue(ctx, keyConsumerSubscriptionID)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodPut, subscriptionPath(id), map[string]any{"status": status})
	return err
}

func (s *Steps) regenerateConsumerSubscriptionToken(ctx context.Context) error {
	id, err := requiredValue(ctx, keyConsumerSubscriptionID)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodPost, subscriptionPath(id)+"/regenerate-token", nil)
	return err
}

// assertConsumerSubscriptionToken compares the token in the last answer with the one the
// subscription had. A replaced token becomes the one later steps compare against and send, and
// the one it replaced the previous subscription token. Neither token is ever repeated in an error.
func (s *Steps) assertConsumerSubscriptionToken(ctx context.Context, how string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	token, err := stringField(resp, "subscriptionToken")
	if err != nil {
		return err
	}
	previous, err := requiredValue(ctx, keyConsumerSubscriptionToken)
	if err != nil {
		return err
	}
	switch {
	case how == "unchanged" && token != previous:
		return errors.New("the synthetic subscription's token changed")
	case how == "replaced" && token == previous:
		return errors.New("the synthetic subscription kept its token")
	case how == "replaced":
		if err := tcontext.Set(ctx, keyPreviousSubscriptionToken, previous); err != nil {
			return err
		}
		return tcontext.Set(ctx, keyConsumerSubscriptionToken, token)
	}
	return nil
}

// unsubscribeConsumer deletes the consumer's subscription. The API's registration stays: the
// scenario can subscribe again, and its cleanup removes only what is left.
func (s *Steps) unsubscribeConsumer(ctx context.Context) error {
	id, err := requiredValue(ctx, keyConsumerSubscriptionID)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodDelete, subscriptionPath(id), nil)
	return err
}

// generateConsumerAPIKey generates an API key for the synthetic REST API and keeps its value in
// scenario state, where the gateway steps read it.
func (s *Steps) generateConsumerAPIKey(ctx context.Context) error {
	api, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	if _, err := scenarioBrowser(ctx); err != nil {
		return err
	}
	if err := registerConsumerResource(ctx, cleanup.Resource{
		Kind: kindConsumerAPIKeys, ID: api, Actor: consumerActor, Description: "the consumer's API keys for " + api,
	}); err != nil {
		return fmt.Errorf("registering the consumer's API keys for %q for cleanup: %w", api, err)
	}
	resp, err := consumerSecretCall(ctx, http.MethodPost, apiKeysPath(api)+"/generate",
		map[string]any{"displayName": "Synthetic key for " + api})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	id, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("the portal generated an API key it does not identify: %w", err)
	}
	if err := tcontext.Set(ctx, keyConsumerAPIKeyID, id); err != nil {
		return err
	}
	key, err := stringField(resp, "key")
	if err != nil {
		return fmt.Errorf("the portal generated an API key without its value: %w", err)
	}
	return tcontext.Set(ctx, keyAPIKey, key)
}

// regenerateConsumerAPIKey gives the consumer's key a new value. The value it had is kept as the
// previous API key, which the gateway steps can send too.
func (s *Steps) regenerateConsumerAPIKey(ctx context.Context) error {
	api, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyConsumerAPIKeyID)
	if err != nil {
		return err
	}
	previous, err := requiredValue(ctx, keyAPIKey)
	if err != nil {
		return err
	}
	resp, err := consumerSecretCall(ctx, http.MethodPost, apiKeysPath(api)+"/regenerate", map[string]any{"keyId": id})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	key, err := stringField(resp, "key")
	if err != nil {
		return fmt.Errorf("the portal regenerated an API key without its new value: %w", err)
	}
	if key == previous {
		return errors.New("the portal regenerated the API key with the value it already had")
	}
	if err := tcontext.Set(ctx, keyPreviousAPIKey, previous); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyAPIKey, key)
}

func (s *Steps) revokeConsumerAPIKey(ctx context.Context) error {
	api, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyConsumerAPIKeyID)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodPost, apiKeysPath(api)+"/revoke", map[string]any{"keyId": id})
	return err
}

func (s *Steps) changeConsumerAPIKeyApplication(ctx context.Context, change string) error {
	api, err := requiredValue(ctx, keyAPIID)
	if err != nil {
		return err
	}
	id, err := requiredValue(ctx, keyConsumerAPIKeyID)
	if err != nil {
		return err
	}
	if change == "dissociates the API key from" {
		_, err = s.consumerCall(ctx, http.MethodPost, apiKeysPath(api)+"/dissociate", map[string]any{"keyId": id})
		return err
	}
	app, err := requiredValue(ctx, keyConsumerApplicationID)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodPost, apiKeysPath(api)+"/associate", map[string]any{"keyId": id, "appId": app})
	return err
}

// registeringKeyManager returns the id of a key manager the portal registers clients with.
func registeringKeyManager(items []map[string]any) (string, error) {
	for _, item := range items {
		id, _ := item["id"].(string)
		if creation, _ := item["keyCreation"].(string); creation == "register" && id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("none of the portal's %d key managers registers clients", len(items))
}

func oauth2KeyPath(id string) string { return "/oauth2-keys/" + url.PathEscape(id) }

// createConsumerOAuth2Keys registers a client_credentials client for the consumer with the
// portal's key manager. The client's name is registered for cleanup first, so a client whose
// answer is lost is still deleted. The answer is not published: it carries the client secret.
func (s *Steps) createConsumerOAuth2Keys(ctx context.Context) error {
	if _, err := scenarioBrowser(ctx); err != nil {
		return err
	}
	resp, err := s.consumerCall(ctx, http.MethodGet, "/key-managers/metadata", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	managers, err := listItems(resp)
	if err != nil {
		return fmt.Errorf("decoding the portal's key managers: %w", err)
	}
	manager, err := registeringKeyManager(managers)
	if err != nil {
		return err
	}
	if err := generateResourceAndStore(ctx, "syn-client", keyConsumerOAuth2ClientName); err != nil {
		return err
	}
	name, err := requiredValue(ctx, keyConsumerOAuth2ClientName)
	if err != nil {
		return err
	}
	if err := registerConsumerResource(ctx, cleanup.Resource{
		Kind: kindConsumerOAuth2Keys, ID: name, Actor: consumerActor, Description: "the consumer's OAuth2 client " + name,
	}); err != nil {
		return fmt.Errorf("registering OAuth2 client %q for cleanup: %w", name, err)
	}
	resp, err = consumerSecretCall(ctx, http.MethodPost, "/oauth2-keys", map[string]any{
		"keyManagerId": manager,
		"properties": map[string]any{
			"client_name": name, "grant_types": []string{"client_credentials"}, "token_endpoint_auth_method": "client_secret_basic",
		},
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return responseError(resp)
	}
	id, err := stringField(resp, "keyId")
	if err != nil {
		return fmt.Errorf("the portal created OAuth2 keys it does not identify: %w", err)
	}
	if err := tcontext.Set(ctx, keyConsumerOAuth2KeyID, id); err != nil {
		return err
	}
	secret, err := stringField(resp, "consumerSecret")
	if err != nil {
		return fmt.Errorf("the portal created OAuth2 keys without the client secret: %w", err)
	}
	return tcontext.Set(ctx, keyConsumerOAuth2Secret, secret)
}

func (s *Steps) changeConsumerOAuth2KeysApplication(ctx context.Context, change string) error {
	id, err := requiredValue(ctx, keyConsumerOAuth2KeyID)
	if err != nil {
		return err
	}
	if change == "dissociates the OAuth2 keys from" {
		_, err = s.consumerCall(ctx, http.MethodPost, oauth2KeyPath(id)+"/dissociate", nil)
		return err
	}
	app, err := requiredValue(ctx, keyConsumerApplicationID)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodPost, oauth2KeyPath(id)+"/associate", map[string]any{"applicationId": app})
	return err
}

// generateConsumerAccessToken asks the portal for an access token with the consumer's client
// secret and keeps the token in scenario state, where the gateway steps read it. The answer is
// not published: it carries the token.
func (s *Steps) generateConsumerAccessToken(ctx context.Context) error {
	id, err := requiredValue(ctx, keyConsumerOAuth2KeyID)
	if err != nil {
		return err
	}
	secret, err := requiredValue(ctx, keyConsumerOAuth2Secret)
	if err != nil {
		return err
	}
	resp, err := consumerSecretCall(ctx, http.MethodPost, oauth2KeyPath(id)+"/generate-token", map[string]any{"consumerSecret": secret})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	token, err := stringField(resp, "accessToken")
	if err != nil {
		return fmt.Errorf("the portal issued no access token: %w", err)
	}
	return tcontext.Set(ctx, keyAccessToken, token)
}

// consumerListing is one of the consumer's lists on the portal and how its items are identified.
type consumerListing struct {
	item    string
	idField string
	idKey   string
	path    func(ctx context.Context) (string, error)
}

// consumerListings maps the step's list phrase to the list it reads.
var consumerListings = map[string]consumerListing{
	"applications": {item: "synthetic application", idField: "id", idKey: keyConsumerApplicationID,
		path: func(context.Context) (string, error) { return "/applications", nil }},
	"subscriptions": {item: "synthetic subscription", idField: "subscriptionId", idKey: keyConsumerSubscriptionID,
		path: func(context.Context) (string, error) { return "/subscriptions", nil }},
	"API keys": {item: "API key", idField: "id", idKey: keyConsumerAPIKeyID,
		path: func(context.Context) (string, error) { return "/api-keys", nil }},
	"API keys for the synthetic REST API": {item: "API key", idField: "id", idKey: keyConsumerAPIKeyID,
		path: func(ctx context.Context) (string, error) {
			api, err := requiredValue(ctx, keyAPIID)
			return apiKeysPath(api), err
		}},
	"API keys of the synthetic application": {item: "API key", idField: "id", idKey: keyConsumerAPIKeyID,
		path: func(ctx context.Context) (string, error) {
			app, err := requiredValue(ctx, keyConsumerApplicationID)
			return applicationPath(app) + "/api-keys", err
		}},
	"OAuth2 keys": {item: "OAuth2 keys", idField: "keyId", idKey: keyConsumerOAuth2KeyID,
		path: func(context.Context) (string, error) { return "/oauth2-keys", nil }},
	"OAuth2 keys of the synthetic application": {item: "OAuth2 keys", idField: "keyId", idKey: keyConsumerOAuth2KeyID,
		path: func(ctx context.Context) (string, error) {
			app, err := requiredValue(ctx, keyConsumerApplicationID)
			return applicationPath(app) + "/oauth2-keys", err
		}},
}

// listingFor returns the list the step reads, refusing an item the list cannot hold and a status
// on anything but a key.
func listingFor(list, item, status string) (consumerListing, error) {
	listing, ok := consumerListings[list]
	if !ok {
		return consumerListing{}, fmt.Errorf("unsupported consumer list %q", list)
	}
	if listing.item != item {
		return consumerListing{}, fmt.Errorf("the consumer's %s hold no %s", list, item)
	}
	if status != "" && item != "API key" {
		return consumerListing{}, fmt.Errorf("only an API key is listed with a status, not a %s", item)
	}
	return listing, nil
}

// listedItem returns the item of items whose field is id.
func listedItem(items []map[string]any, field, id string) (map[string]any, bool) {
	for _, item := range items {
		if value, _ := item[field].(string); value == id {
			return item, true
		}
	}
	return nil, false
}

// assertConsumerListing reads one of the consumer's lists and checks whether it holds the
// scenario's item, and with which status when one is named.
func (s *Steps) assertConsumerListing(ctx context.Context, list, expectation, item, status string) error {
	listing, err := listingFor(list, item, status)
	if err != nil {
		return err
	}
	if expectation == "not include" && status != "" {
		return errors.New("a status is named only for an item the list should include")
	}
	id, err := requiredValue(ctx, listing.idKey)
	if err != nil {
		return err
	}
	path, err := listing.path(ctx)
	if err != nil {
		return err
	}
	resp, err := s.consumerCall(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	items, err := listItems(resp)
	if err != nil {
		return fmt.Errorf("decoding the consumer's %s: %w", list, err)
	}
	found, listed := listedItem(items, listing.idField, id)
	switch {
	case expectation == "not include" && listed:
		return fmt.Errorf("the consumer's %s still include the %s %s", list, item, id)
	case expectation == "include" && !listed:
		return fmt.Errorf("the consumer's %s do not include the %s %s among %d listed", list, item, id, len(items))
	case status != "":
		if got, _ := found["status"].(string); got != status {
			return fmt.Errorf("the %s %s is listed as %q, want %s", item, id, got, status)
		}
	}
	return nil
}

// consumerSession makes cleanup's calls as the consumer. A session that has ended, or a browser
// that never held one, is signed in again once with the runner's consumer.
type consumerSession struct {
	signIn func(ctx context.Context) error
}

func (c consumerSession) call(ctx context.Context, method, path string, body any) (*httpx.Response, error) {
	if browser, err := scenarioBrowser(ctx); err == nil {
		resp, err := browser.apiRequest(ctx, method, path, body)
		if err == nil && resp.StatusCode != http.StatusUnauthorized {
			return resp, nil
		}
		if err != nil && !errors.Is(err, errNoConsumerSession) {
			return nil, err
		}
	}
	if err := c.signIn(ctx); err != nil {
		return nil, fmt.Errorf("signing the consumer in to clean up: %w", err)
	}
	browser, err := scenarioBrowser(ctx)
	if err != nil {
		return nil, err
	}
	return browser.apiRequest(ctx, method, path, body)
}

// listAll reads every page of one of the consumer's lists. found is false when the portal does not
// have the list's parent, an API it no longer lists.
func (c consumerSession) listAll(ctx context.Context, path string, query url.Values) (items []map[string]any, found bool, err error) {
	for offset := 0; ; offset += listPageSize {
		page := url.Values{}
		for key, values := range query {
			page[key] = values
		}
		page.Set("limit", fmt.Sprint(listPageSize))
		page.Set("offset", fmt.Sprint(offset))
		resp, err := c.call(ctx, http.MethodGet, path+"?"+page.Encode(), nil)
		if err != nil {
			return nil, false, err
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, false, nil
		}
		if resp.StatusCode != http.StatusOK {
			return nil, false, responseError(resp)
		}
		listed, err := listItems(resp)
		if err != nil {
			return nil, false, fmt.Errorf("decoding %s: %w", path, err)
		}
		items = append(items, listed...)
		if len(listed) < listPageSize {
			return items, true, nil
		}
	}
}

// settled reports whether an answer leaves nothing behind: done, already gone, or (for a key)
// already revoked.
func settled(resp *httpx.Response, conflictSettles bool) bool {
	return resp.Succeeded() || resp.StatusCode == http.StatusNotFound ||
		(conflictSettles && resp.StatusCode == http.StatusConflict)
}

func (c consumerSession) deleteApplication(ctx context.Context, id string) error {
	resp, err := c.call(ctx, http.MethodDelete, applicationPath(id), nil)
	if err != nil {
		return err
	}
	if !settled(resp, false) {
		return responseError(resp)
	}
	return nil
}

func (c consumerSession) deleteSubscription(ctx context.Context, id string) error {
	resp, err := c.call(ctx, http.MethodDelete, subscriptionPath(id), nil)
	if err != nil {
		return err
	}
	if !settled(resp, false) {
		return responseError(resp)
	}
	return nil
}

func (c consumerSession) revokeAPIKey(ctx context.Context, api, id string) error {
	resp, err := c.call(ctx, http.MethodPost, apiKeysPath(api)+"/revoke", map[string]any{"keyId": id})
	if err != nil {
		return err
	}
	if !settled(resp, true) {
		return responseError(resp)
	}
	return nil
}

// removeSubscriptions deletes every subscription the consumer has, to one API or, with api empty,
// to any.
func (c consumerSession) removeSubscriptions(ctx context.Context, api string) error {
	query := url.Values{}
	if api != "" {
		query.Set("artifactId", api)
	}
	items, _, err := c.listAll(ctx, "/subscriptions", query)
	if err != nil {
		return err
	}
	var failed []error
	for _, item := range items {
		if id, _ := item["subscriptionId"].(string); id != "" {
			failed = append(failed, c.deleteSubscription(ctx, id))
		}
	}
	return errors.Join(failed...)
}

// removeAPIKeys revokes every active key the consumer has, for one API or, with api empty, for any.
func (c consumerSession) removeAPIKeys(ctx context.Context, api string) error {
	path := "/api-keys"
	if api != "" {
		path = apiKeysPath(api)
	}
	items, _, err := c.listAll(ctx, path, nil)
	if err != nil {
		return err
	}
	var failed []error
	for _, item := range items {
		id, _ := item["id"].(string)
		owner, _ := item["apiId"].(string)
		if owner == "" {
			owner = api
		}
		if status, _ := item["status"].(string); id == "" || owner == "" || status != "ACTIVE" {
			continue
		}
		failed = append(failed, c.revokeAPIKey(ctx, owner, id))
	}
	return errors.Join(failed...)
}

// removeOAuth2Keys deletes the consumer's OAuth2 keys, and with them their clients at the key
// manager: the one whose client has the given name or, with name empty, every one.
func (c consumerSession) removeOAuth2Keys(ctx context.Context, name string) error {
	items, _, err := c.listAll(ctx, "/oauth2-keys", nil)
	if err != nil {
		return err
	}
	var failed []error
	for _, item := range items {
		id, _ := item["keyId"].(string)
		if listed, _ := item["name"].(string); id == "" || (name != "" && listed != name) {
			continue
		}
		resp, err := c.call(ctx, http.MethodDelete, oauth2KeyPath(id), nil)
		switch {
		case err != nil:
			failed = append(failed, err)
		case !settled(resp, false):
			failed = append(failed, responseError(resp))
		}
	}
	return errors.Join(failed...)
}

func (c consumerSession) removeApplications(ctx context.Context) error {
	items, _, err := c.listAll(ctx, "/applications", nil)
	if err != nil {
		return err
	}
	var failed []error
	for _, item := range items {
		if id, _ := item["id"].(string); id != "" {
			failed = append(failed, c.deleteApplication(ctx, id))
		}
	}
	return errors.Join(failed...)
}

// removeLeftovers empties the runner's consumer's portal: its subscriptions, active keys, OAuth2
// keys and applications. Everything the consumer owns was created by this runner. It always signs the
// consumer in, because the runner's last scenario may have left a visitor's browser or a
// signed-out one.
func (c consumerSession) removeLeftovers(ctx context.Context) error {
	if err := c.signIn(ctx); err != nil {
		return fmt.Errorf("signing the consumer in to clean up: %w", err)
	}
	return errors.Join(c.removeSubscriptions(ctx, ""), c.removeAPIKeys(ctx, ""), c.removeOAuth2Keys(ctx, ""),
		c.removeApplications(ctx))
}

// registerConsumerDeleters installs the deleters of what the consumer creates on the portal.
// signIn signs the runner's consumer in again when cleanup finds no live session.
func registerConsumerDeleters(reg *cleanup.Registry, signIn func(ctx context.Context) error) error {
	session := consumerSession{signIn: signIn}
	deleters := map[cleanup.Kind]cleanup.Deleter{
		kindConsumerSubscriptions: func(ctx context.Context, resource cleanup.Resource) error {
			return session.removeSubscriptions(ctx, resource.ID)
		},
		kindConsumerAPIKeys: func(ctx context.Context, resource cleanup.Resource) error {
			return session.removeAPIKeys(ctx, resource.ID)
		},
		kindConsumerOAuth2Keys: func(ctx context.Context, resource cleanup.Resource) error {
			return session.removeOAuth2Keys(ctx, resource.ID)
		},
		kindConsumerApplication: func(ctx context.Context, resource cleanup.Resource) error {
			return session.deleteApplication(ctx, resource.ID)
		},
		kindConsumerLeftovers: func(ctx context.Context, _ cleanup.Resource) error {
			return session.removeLeftovers(ctx)
		},
	}
	for kind, deleter := range deleters {
		if err := reg.RegisterDeleter(kind, deleter); err != nil {
			return err
		}
	}
	return nil
}

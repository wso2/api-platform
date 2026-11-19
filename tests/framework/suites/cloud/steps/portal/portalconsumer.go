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

package portal

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"syscall"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

const (
	portalAPIBasePath = portalBasePath + "/api/v0.9"
	// A mutating call with a portal session repeats the portalCSRFCookie value in portalCSRFHeader.
	portalCSRFCookie            = "XSRF-TOKEN"
	portalCSRFHeader            = "X-CSRF-Token"
	consumerActor               = "portal-consumer"
	consumerResourceDescription = "Created and deleted by the APIP cloud integration test. Safe to delete."
	keyConsumerApplicationID    = "cloudPortalApplicationID"
	keyConsumerSubscriptionID   = "cloudPortalSubscriptionID"
	keyConsumerSubscriptionAPI  = "cloudPortalSubscriptionAPI"
	// keyAnsweredSubscriptionToken holds the answeredToken of the published consumer answer.
	keyAnsweredSubscriptionToken = "cloudAnsweredSubscriptionToken"
	keyConsumerAPIKeyID          = "cloudPortalAPIKeyID"
	keyConsumerOAuth2ClientName  = "cloudPortalOAuth2ClientName"
	keyConsumerOAuth2KeyID       = "cloudPortalOAuth2KeyID"
	keyConsumerOAuth2Secret      = "cloudPortalOAuth2Secret"
	subscriptionTokenField       = "subscriptionToken"
	redactedValue                = "[redacted]"
	// listPageSize is the portal's largest page.
	listPageSize = 100
)

var (
	kindConsumerLeftovers       = cleanup.Kind{Name: "portal-consumer-leftovers", Order: 5}
	kindConsumerSubscriptions   = cleanup.Kind{Name: "portal-subscriptions", Order: 10}
	kindConsumerAPIKeys         = cleanup.Kind{Name: "portal-api-keys", Order: 15}
	kindConsumerOAuth2Key       = cleanup.Kind{Name: "portal-oauth2-key", Order: 16}
	kindConsumerOAuth2KeysNamed = cleanup.Kind{Name: "portal-oauth2-keys", Order: 17}
	kindConsumerApplication     = cleanup.Kind{Name: "portal-application", Order: 20}
)

var errNoConsumerSession = errors.New("the consumer's portal session holds no CSRF token; sign the consumer in first")

// answeredToken is the subscription token of a published answer, whose copy carries it redacted.
type answeredToken struct {
	answer *httpx.Response
	token  string
}

// RegisterPortalConsumerSteps binds the steps the signed-in consumer takes on the portal's REST API.
func (s *Steps) RegisterPortalConsumerSteps(sc cloudbase.StepRegistrar) {
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

// apiRequest calls the portal's REST API with the browser's session.
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

// redactSubscriptionTokens returns a copy of resp whose subscription tokens are redacted, and the
// token of a single subscription answer, if any.
func redactSubscriptionTokens(resp *httpx.Response) (*httpx.Response, string) {
	if !bytes.Contains(resp.Body, []byte(`"`+subscriptionTokenField+`"`)) {
		return resp, ""
	}
	var document any
	if json.Unmarshal(resp.Body, &document) != nil {
		return resp, ""
	}
	var token string
	if object, ok := document.(map[string]any); ok {
		token, _ = object[subscriptionTokenField].(string)
		if object[subscriptionTokenField] != nil {
			object[subscriptionTokenField] = redactedValue
		}
		if list, ok := object["list"].([]any); ok {
			for _, item := range list {
				if entry, ok := item.(map[string]any); ok && entry[subscriptionTokenField] != nil {
					entry[subscriptionTokenField] = redactedValue
				}
			}
		}
	}
	redacted, err := json.Marshal(document)
	if err != nil {
		return resp, token
	}
	copied := *resp
	copied.Body = redacted
	return &copied, token
}

// consumerCall makes one call as the consumer and publishes the answer, with subscription tokens
// redacted, for the assertion steps.
func (s *Steps) consumerCall(ctx context.Context, method, path string, body any) (*httpx.Response, error) {
	browser, err := scenarioBrowser(ctx)
	if err != nil {
		return nil, err
	}
	httpx.ClearPublished(ctx)
	tcontext.Remove(ctx, keyAnsweredSubscriptionToken)
	resp, err := browser.apiRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	published, token := redactSubscriptionTokens(resp)
	if token != "" {
		if err := tcontext.Set(ctx, keyAnsweredSubscriptionToken, answeredToken{answer: published, token: token}); err != nil {
			return nil, err
		}
	}
	return resp, s.publishPortalAnswer(ctx, published)
}

// consumerSecretCall makes one call whose answer carries a credential; the answer is never published.
func consumerSecretCall(ctx context.Context, method, path string, body any) (*httpx.Response, error) {
	browser, err := scenarioBrowser(ctx)
	if err != nil {
		return nil, err
	}
	httpx.ClearPublished(ctx)
	return browser.apiRequest(ctx, method, path, body)
}

// answerError reports an unexpected answer without its body, which can carry a credential.
func answerError(resp *httpx.Response) error {
	return &cloudbase.StatusError{Status: resp.StatusCode,
		Message: fmt.Sprintf("cloud-console: unexpected response: %s %s -> %d", resp.Method, resp.URL, resp.StatusCode)}
}

// registerConsumerResource registers a resource for cleanup unless it is already registered.
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

// listPages reads every page of a portal list through call. found is false when the portal answers
// 404: the list's API or application is gone. On an error it returns the items read before it.
func listPages(
	ctx context.Context, call func(context.Context, string, string, any) (*httpx.Response, error), path string, query url.Values,
) (items []map[string]any, found bool, err error) {
	for offset := 0; ; offset += listPageSize {
		page := url.Values{}
		for key, values := range query {
			page[key] = values
		}
		page.Set("limit", fmt.Sprint(listPageSize))
		page.Set("offset", fmt.Sprint(offset))
		resp, err := call(ctx, http.MethodGet, path+"?"+page.Encode(), nil)
		if err != nil {
			return items, true, err
		}
		if resp.StatusCode == http.StatusNotFound && offset == 0 {
			return nil, false, nil
		}
		if resp.StatusCode != http.StatusOK {
			return items, true, answerError(resp)
		}
		listed, err := cloudbase.ListItems(resp)
		if err != nil {
			return items, true, fmt.Errorf("decoding %s: %w", path, err)
		}
		items = append(items, listed...)
		if len(listed) < listPageSize {
			return items, true, nil
		}
	}
}

func applicationPath(id string) string { return "/applications/" + url.PathEscape(id) }

func subscriptionPath(id string) string { return "/subscriptions/" + url.PathEscape(id) }

func apiKeysPath(api string) string { return "/apis/" + url.PathEscape(api) + "/api-keys" }

func oauth2KeyPath(id string) string { return "/oauth2-keys/" + url.PathEscape(id) }

// applicationBody is an application create, which names the handle, or update, which keeps it.
func applicationBody(id string, create bool, overrides map[string]string) (map[string]any, error) {
	body := map[string]any{"displayName": cloudbase.PublicationTitle + id, "description": consumerResourceDescription}
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
	if err := cloudbase.GenerateResourceAndStore(ctx, "syn-app", keyConsumerApplicationID); err != nil {
		return err
	}
	id, err := cloudbase.RequiredValue(ctx, keyConsumerApplicationID)
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
		if cloudbase.RefusedCreate(resp) {
			cloudbase.Deregister(ctx, kindConsumerApplication, id)
		}
		return cloudbase.ResponseError(resp)
	}
	return nil
}

func (s *Steps) readConsumerApplication(ctx context.Context) error {
	id, err := cloudbase.RequiredValue(ctx, keyConsumerApplicationID)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodGet, applicationPath(id), nil)
	return err
}

func (s *Steps) updateConsumerApplication(ctx context.Context, table *godog.Table) error {
	id, err := cloudbase.RequiredValue(ctx, keyConsumerApplicationID)
	if err != nil {
		return err
	}
	overrides, err := cloudbase.TableValues(table)
	if err != nil {
		return err
	}
	for key, value := range overrides {
		if overrides[key], err = cloudbase.ExpandCloudValue(ctx, value); err != nil {
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
	id, err := cloudbase.RequiredValue(ctx, keyConsumerApplicationID)
	if err != nil {
		return err
	}
	resp, err := s.consumerCall(ctx, http.MethodDelete, applicationPath(id), nil)
	if err != nil {
		return err
	}
	if resp.Succeeded() {
		cloudbase.Deregister(ctx, kindConsumerApplication, id)
	}
	return nil
}

func planKeyFor(which string) string {
	if which == "the other" {
		return keySecondPlanID
	}
	return keyPlanID
}

// subscribeConsumer subscribes the consumer and leaves the answer for the status assertion.
func (s *Steps) subscribeConsumer(ctx context.Context, which string) error {
	api, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIID)
	if err != nil {
		return err
	}
	plan, err := cloudbase.RequiredValue(ctx, planKeyFor(which))
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
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		if bound, _ := tcontext.ResolveString(ctx, keyConsumerSubscriptionAPI); bound != api {
			forgetConsumerSubscription(ctx)
		}
		return nil
	}
	id, err := cloudbase.StringField(resp, "subscriptionId")
	if err != nil {
		return fmt.Errorf("the portal created a subscription it does not identify: %w", err)
	}
	if err := tcontext.Set(ctx, keyConsumerSubscriptionID, id); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyConsumerSubscriptionAPI, api); err != nil {
		return err
	}
	token, err := cloudbase.StringField(resp, subscriptionTokenField)
	if err != nil {
		return fmt.Errorf("the portal created a subscription without a token: %w", err)
	}
	return tcontext.Set(ctx, cloudbase.KeyConsumerSubscriptionToken, token)
}

// forgetConsumerSubscription drops the subscription of an earlier scenario's API.
func forgetConsumerSubscription(ctx context.Context) {
	for _, key := range []string{keyConsumerSubscriptionID, keyConsumerSubscriptionAPI, cloudbase.KeyConsumerSubscriptionToken, cloudbase.KeyPreviousSubscriptionToken} {
		tcontext.Remove(ctx, key)
	}
}

// consumerSubscription returns the id of the consumer's subscription to the synthetic REST API.
func consumerSubscription(ctx context.Context) (string, error) {
	api, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIID)
	if err != nil {
		return "", err
	}
	if bound, _ := tcontext.ResolveString(ctx, keyConsumerSubscriptionAPI); bound != api {
		return "", fmt.Errorf("the consumer has no subscription to the synthetic REST API %s", api)
	}
	return cloudbase.RequiredValue(ctx, keyConsumerSubscriptionID)
}

func (s *Steps) readConsumerSubscription(ctx context.Context) error {
	id, err := consumerSubscription(ctx)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodGet, subscriptionPath(id), nil)
	return err
}

func (s *Steps) changeConsumerPlan(ctx context.Context, which string) error {
	id, err := consumerSubscription(ctx)
	if err != nil {
		return err
	}
	plan, err := cloudbase.RequiredValue(ctx, planKeyFor(which))
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodPost, subscriptionPath(id)+"/change-plan", map[string]any{"planId": plan})
	return err
}

func (s *Steps) setConsumerSubscriptionStatus(ctx context.Context, status string) error {
	id, err := consumerSubscription(ctx)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodPut, subscriptionPath(id), map[string]any{"status": status})
	return err
}

func (s *Steps) regenerateConsumerSubscriptionToken(ctx context.Context) error {
	id, err := consumerSubscription(ctx)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodPost, subscriptionPath(id)+"/regenerate-token", nil)
	return err
}

// assertConsumerSubscriptionToken compares the token in the last answer with the subscription's.
// A replaced token becomes the subscription token, and the old one the previous subscription token.
func (s *Steps) assertConsumerSubscriptionToken(ctx context.Context, how string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	value, _ := tcontext.Get(ctx, keyAnsweredSubscriptionToken)
	answered, ok := value.(answeredToken)
	if !ok || answered.answer != resp {
		return errors.New("the last answer is not a subscription answer from the portal")
	}
	if resp.StatusCode != http.StatusOK {
		return cloudbase.ResponseError(resp)
	}
	token := answered.token
	previous, err := cloudbase.RequiredValue(ctx, cloudbase.KeyConsumerSubscriptionToken)
	if err != nil {
		return err
	}
	switch {
	case how == "unchanged" && token != previous:
		return errors.New("the synthetic subscription's token changed")
	case how == "replaced" && token == previous:
		return errors.New("the synthetic subscription kept its token")
	case how == "replaced":
		if err := tcontext.Set(ctx, cloudbase.KeyPreviousSubscriptionToken, previous); err != nil {
			return err
		}
		return tcontext.Set(ctx, cloudbase.KeyConsumerSubscriptionToken, token)
	}
	return nil
}

// unsubscribeConsumer deletes the subscription and keeps the API's cleanup registration, since the
// scenario can subscribe again.
func (s *Steps) unsubscribeConsumer(ctx context.Context) error {
	id, err := consumerSubscription(ctx)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodDelete, subscriptionPath(id), nil)
	return err
}

func (s *Steps) generateConsumerAPIKey(ctx context.Context) error {
	api, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIID)
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
		return answerError(resp)
	}
	id, err := cloudbase.StringField(resp, "id")
	if err != nil {
		return fmt.Errorf("the portal generated an API key it does not identify: %w", err)
	}
	if err := tcontext.Set(ctx, keyConsumerAPIKeyID, id); err != nil {
		return err
	}
	key, err := cloudbase.StringField(resp, "key")
	if err != nil {
		return fmt.Errorf("the portal generated an API key without its value: %w", err)
	}
	return tcontext.Set(ctx, cloudbase.KeyAPIKey, key)
}

// regenerateConsumerAPIKey gives the key a new value and keeps the old one as the previous API key.
func (s *Steps) regenerateConsumerAPIKey(ctx context.Context) error {
	api, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIID)
	if err != nil {
		return err
	}
	id, err := cloudbase.RequiredValue(ctx, keyConsumerAPIKeyID)
	if err != nil {
		return err
	}
	previous, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIKey)
	if err != nil {
		return err
	}
	resp, err := consumerSecretCall(ctx, http.MethodPost, apiKeysPath(api)+"/regenerate", map[string]any{"keyId": id})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return answerError(resp)
	}
	key, err := cloudbase.StringField(resp, "key")
	if err != nil {
		return fmt.Errorf("the portal regenerated an API key without its new value: %w", err)
	}
	if key == previous {
		return errors.New("the portal regenerated the API key with the value it already had")
	}
	if err := tcontext.Set(ctx, cloudbase.KeyPreviousAPIKey, previous); err != nil {
		return err
	}
	return tcontext.Set(ctx, cloudbase.KeyAPIKey, key)
}

func (s *Steps) revokeConsumerAPIKey(ctx context.Context) error {
	api, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIID)
	if err != nil {
		return err
	}
	id, err := cloudbase.RequiredValue(ctx, keyConsumerAPIKeyID)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodPost, apiKeysPath(api)+"/revoke", map[string]any{"keyId": id})
	return err
}

func (s *Steps) changeConsumerAPIKeyApplication(ctx context.Context, change string) error {
	api, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIID)
	if err != nil {
		return err
	}
	id, err := cloudbase.RequiredValue(ctx, keyConsumerAPIKeyID)
	if err != nil {
		return err
	}
	if change == "dissociates the API key from" {
		_, err = s.consumerCall(ctx, http.MethodPost, apiKeysPath(api)+"/dissociate", map[string]any{"keyId": id})
		return err
	}
	app, err := cloudbase.RequiredValue(ctx, keyConsumerApplicationID)
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

// createConsumerOAuth2Keys registers a client_credentials client with the portal's key manager.
func (s *Steps) createConsumerOAuth2Keys(ctx context.Context) error {
	if _, err := scenarioBrowser(ctx); err != nil {
		return err
	}
	managers, found, err := listPages(ctx, s.consumerCall, "/key-managers/metadata", nil)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("the portal does not serve its key managers")
	}
	manager, err := registeringKeyManager(managers)
	if err != nil {
		return err
	}
	if err := cloudbase.GenerateResourceAndStore(ctx, "syn-client", keyConsumerOAuth2ClientName); err != nil {
		return err
	}
	name, err := cloudbase.RequiredValue(ctx, keyConsumerOAuth2ClientName)
	if err != nil {
		return err
	}
	if err := registerConsumerResource(ctx, cleanup.Resource{
		Kind: kindConsumerOAuth2KeysNamed, ID: name, Actor: consumerActor, Description: "the consumer's OAuth2 client " + name,
	}); err != nil {
		return fmt.Errorf("registering OAuth2 client %q for cleanup: %w", name, err)
	}
	resp, err := consumerSecretCall(ctx, http.MethodPost, "/oauth2-keys", map[string]any{
		"keyManagerId": manager,
		"properties": map[string]any{
			"client_name": name, "grant_types": []string{"client_credentials"}, "token_endpoint_auth_method": "client_secret_basic",
		},
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return answerError(resp)
	}
	id, err := cloudbase.StringField(resp, "keyId")
	if err != nil {
		return fmt.Errorf("the portal created OAuth2 keys it does not identify: %w", err)
	}
	if err := registerConsumerResource(ctx, cleanup.Resource{
		Kind: kindConsumerOAuth2Key, ID: id, Actor: consumerActor, Description: "the consumer's OAuth2 key " + id,
	}); err != nil {
		return fmt.Errorf("registering OAuth2 key %q for cleanup: %w", id, err)
	}
	if err := tcontext.Set(ctx, keyConsumerOAuth2KeyID, id); err != nil {
		return err
	}
	secret, err := cloudbase.StringField(resp, "consumerSecret")
	if err != nil {
		return fmt.Errorf("the portal created OAuth2 keys without the client secret: %w", err)
	}
	return tcontext.Set(ctx, keyConsumerOAuth2Secret, secret)
}

func (s *Steps) changeConsumerOAuth2KeysApplication(ctx context.Context, change string) error {
	id, err := cloudbase.RequiredValue(ctx, keyConsumerOAuth2KeyID)
	if err != nil {
		return err
	}
	if change == "dissociates the OAuth2 keys from" {
		_, err = s.consumerCall(ctx, http.MethodPost, oauth2KeyPath(id)+"/dissociate", nil)
		return err
	}
	app, err := cloudbase.RequiredValue(ctx, keyConsumerApplicationID)
	if err != nil {
		return err
	}
	_, err = s.consumerCall(ctx, http.MethodPost, oauth2KeyPath(id)+"/associate", map[string]any{"applicationId": app})
	return err
}

func (s *Steps) generateConsumerAccessToken(ctx context.Context) error {
	id, err := cloudbase.RequiredValue(ctx, keyConsumerOAuth2KeyID)
	if err != nil {
		return err
	}
	secret, err := cloudbase.RequiredValue(ctx, keyConsumerOAuth2Secret)
	if err != nil {
		return err
	}
	resp, err := consumerSecretCall(ctx, http.MethodPost, oauth2KeyPath(id)+"/generate-token", map[string]any{"consumerSecret": secret})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return answerError(resp)
	}
	token, err := cloudbase.StringField(resp, "accessToken")
	if err != nil {
		return fmt.Errorf("the portal issued no access token: %w", err)
	}
	return tcontext.Set(ctx, cloudbase.KeyAccessToken, token)
}

// consumerListing is one of the consumer's lists and how its items are identified.
type consumerListing struct {
	item    string
	idField string
	idKey   string
	path    func(ctx context.Context) (string, error)
}

func fixedPath(path string) func(context.Context) (string, error) {
	return func(context.Context) (string, error) { return path, nil }
}

func applicationListPath(list string) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		app, err := cloudbase.RequiredValue(ctx, keyConsumerApplicationID)
		return applicationPath(app) + list, err
	}
}

// consumerListings maps the step's list phrase to the list it reads.
var consumerListings = map[string]consumerListing{
	"applications": {item: "synthetic application", idField: "id", idKey: keyConsumerApplicationID, path: fixedPath("/applications")},
	"subscriptions": {item: "synthetic subscription", idField: "subscriptionId", idKey: keyConsumerSubscriptionID,
		path: fixedPath("/subscriptions")},
	"API keys": {item: "API key", idField: "id", idKey: keyConsumerAPIKeyID, path: fixedPath("/api-keys")},
	"API keys for the synthetic REST API": {item: "API key", idField: "id", idKey: keyConsumerAPIKeyID,
		path: func(ctx context.Context) (string, error) {
			api, err := cloudbase.RequiredValue(ctx, cloudbase.KeyAPIID)
			return apiKeysPath(api), err
		}},
	"API keys of the synthetic application": {item: "API key", idField: "id", idKey: keyConsumerAPIKeyID,
		path: applicationListPath("/api-keys")},
	"OAuth2 keys": {item: "OAuth2 keys", idField: "keyId", idKey: keyConsumerOAuth2KeyID, path: fixedPath("/oauth2-keys")},
	"OAuth2 keys of the synthetic application": {item: "OAuth2 keys", idField: "keyId", idKey: keyConsumerOAuth2KeyID,
		path: applicationListPath("/oauth2-keys")},
}

// listingFor returns the list a step reads, refusing an item the list cannot hold and a status on
// anything but an API key.
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

func listedItem(items []map[string]any, field, id string) (map[string]any, bool) {
	for _, item := range items {
		if value, _ := item[field].(string); value == id {
			return item, true
		}
	}
	return nil, false
}

// assertConsumerListing reads every page of one of the consumer's lists and checks whether it
// holds the scenario's item, with the named status if any.
func (s *Steps) assertConsumerListing(ctx context.Context, list, expectation, item, status string) error {
	listing, err := listingFor(list, item, status)
	if err != nil {
		return err
	}
	if expectation == "not include" && status != "" {
		return errors.New("a status is named only for an item the list should include")
	}
	id, err := cloudbase.RequiredValue(ctx, listing.idKey)
	if err != nil {
		return err
	}
	path, err := listing.path(ctx)
	if err != nil {
		return err
	}
	items, found, err := listPages(ctx, s.consumerCall, path, nil)
	if err != nil {
		return fmt.Errorf("reading the consumer's %s: %w", list, err)
	}
	if !found {
		return fmt.Errorf("the portal does not have the consumer's %s", list)
	}
	entry, listed := listedItem(items, listing.idField, id)
	switch {
	case expectation == "not include" && listed:
		return fmt.Errorf("the consumer's %s still include the %s %s", list, item, id)
	case expectation == "include" && !listed:
		return fmt.Errorf("the consumer's %s do not include the %s %s among %d listed", list, item, id, len(items))
	case status != "":
		if got, _ := entry["status"].(string); got != status {
			return fmt.Errorf("the %s %s is listed as %q, want %s", item, id, got, status)
		}
	}
	return nil
}

// consumerSession makes cleanup's calls as the consumer, signing the consumer in again once when
// the browser holds no live session or a stale CSRF token.
type consumerSession struct {
	signIn func(ctx context.Context) error
}

func (c consumerSession) call(ctx context.Context, method, path string, body any) (*httpx.Response, error) {
	if browser, err := scenarioBrowser(ctx); err == nil {
		resp, err := browser.apiRequest(ctx, method, path, body)
		if err == nil && resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
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

// settled reports whether an answer leaves nothing behind: done, already gone, or, when
// conflictSettles, already revoked.
func settled(resp *httpx.Response, conflictSettles bool) bool {
	return resp.Succeeded() || resp.StatusCode == http.StatusNotFound ||
		(conflictSettles && resp.StatusCode == http.StatusConflict)
}

// removalRetryWindow bounds how long one removal is repeated, so an item that stays unavailable
// leaves the rest of the sweep its time.
const removalRetryWindow = 20 * time.Second

// leftoverSweepAttempts is how many times the runner's sweep of a consumer's portal is run while
// it fails, each time listing what is still there.
const leftoverSweepAttempts = 3

// remove sends one cleanup call and fails unless the answer is settled. A call that got no full
// answer, a 5xx or a 429 is repeated for up to removalRetryWindow, since a removal that is missed
// leaves the item on the shared portal for good; any other answer is final.
func (c consumerSession) remove(ctx context.Context, method, path string, body any, conflictSettles bool) error {
	ctx, cancel := context.WithTimeout(ctx, removalRetryWindow)
	defer cancel()
	var failure, transient error
	err := retry.Await(ctx, retry.Options{Interval: 2 * time.Second},
		func(ctx context.Context) (bool, error) {
			resp, err := c.call(ctx, method, path, body)
			switch {
			case err != nil && failedInTransit(err):
				transient = err
				return false, retry.Transient(err)
			case err != nil:
				failure = err
			case resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests:
				transient = answerError(resp)
				return false, retry.Transient(transient)
			case !settled(resp, conflictSettles):
				failure = answerError(resp)
			}
			return true, nil
		},
		func(answered bool) bool { return answered }, method+" "+path)
	if err != nil {
		if transient != nil {
			return fmt.Errorf("%w (last attempt: %v)", err, transient)
		}
		return err
	}
	return failure
}

// failedInTransit reports whether err is a request that got no full answer: a dropped connection,
// or an answer whose body was cut off. A request that could not be built, a certificate the client
// refuses, or an answer over the size limit is not, since sending it again changes nothing.
func failedInTransit(err error) bool {
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr) && urlErr.Op != "parse" && !certificateRefused(err)
}

// certificateRefused reports whether err is the client refusing the server's certificate.
func certificateRefused(err error) bool {
	var unknownAuthority x509.UnknownAuthorityError
	var invalid x509.CertificateInvalidError
	var hostname x509.HostnameError
	var verification *tls.CertificateVerificationError
	return errors.As(err, &unknownAuthority) || errors.As(err, &invalid) ||
		errors.As(err, &hostname) || errors.As(err, &verification)
}

func (c consumerSession) deleteApplication(ctx context.Context, id string) error {
	return c.remove(ctx, http.MethodDelete, applicationPath(id), nil, false)
}

// removeSubscriptions deletes the consumer's subscriptions to api, or to every API when api is "".
func (c consumerSession) removeSubscriptions(ctx context.Context, api string) error {
	query := url.Values{}
	if api != "" {
		query.Set("artifactId", api)
	}
	items, _, err := listPages(ctx, c.call, "/subscriptions", query)
	failed := []error{err}
	for _, item := range items {
		if id, _ := item["subscriptionId"].(string); id != "" {
			failed = append(failed, c.remove(ctx, http.MethodDelete, subscriptionPath(id), nil, false))
		}
	}
	return errors.Join(failed...)
}

// removeAPIKeys revokes the consumer's active keys for api, or for every API when api is "".
func (c consumerSession) removeAPIKeys(ctx context.Context, api string) error {
	path := "/api-keys"
	if api != "" {
		path = apiKeysPath(api)
	}
	items, _, err := listPages(ctx, c.call, path, nil)
	failed := []error{err}
	for _, item := range items {
		id, _ := item["id"].(string)
		owner, _ := item["apiId"].(string)
		if owner == "" {
			owner = api
		}
		if status, _ := item["status"].(string); id == "" || owner == "" || status != "ACTIVE" {
			continue
		}
		failed = append(failed, c.remove(ctx, http.MethodPost, apiKeysPath(owner)+"/revoke", map[string]any{"keyId": id}, true))
	}
	return errors.Join(failed...)
}

func (c consumerSession) deleteOAuth2Key(ctx context.Context, id string) error {
	return c.remove(ctx, http.MethodDelete, oauth2KeyPath(id), nil, false)
}

// removeOAuth2Keys deletes the consumer's OAuth2 keys, and their clients, whose client is named
// name, or all of them when name is "".
func (c consumerSession) removeOAuth2Keys(ctx context.Context, name string) error {
	items, _, err := listPages(ctx, c.call, "/oauth2-keys", nil)
	failed := []error{err}
	for _, item := range items {
		id, _ := item["keyId"].(string)
		if listed, _ := item["name"].(string); id != "" && (name == "" || listed == name) {
			failed = append(failed, c.deleteOAuth2Key(ctx, id))
		}
	}
	return errors.Join(failed...)
}

func (c consumerSession) removeApplications(ctx context.Context) error {
	items, _, err := listPages(ctx, c.call, "/applications", nil)
	failed := []error{err}
	for _, item := range items {
		if id, _ := item["id"].(string); id != "" {
			failed = append(failed, c.deleteApplication(ctx, id))
		}
	}
	return errors.Join(failed...)
}

// removeLeftovers empties the portal of consumer, the runner's consumer. Each sweep signs in,
// because the last scenario may have left a visitor's or a signed-out browser, and a sweep that
// fails is run again, up to leftoverSweepAttempts times, before the failure is reported.
func (c consumerSession) removeLeftovers(ctx context.Context, consumer string) error {
	if current, _, err := consumerCredentials(ctx); err != nil || current != consumer {
		return fmt.Errorf("the portal items of consumer %q were not swept: the runner can sign in only as its latest consumer", consumer)
	}
	attempts := 0
	var failed error
	err := retry.Await(ctx, retry.Options{Interval: 2 * time.Second},
		func(ctx context.Context) (bool, error) {
			attempts++
			failed = c.sweepPortal(ctx)
			if failed == nil || attempts >= leftoverSweepAttempts {
				return true, nil
			}
			return false, retry.Transient(failed)
		},
		func(done bool) bool { return done }, "sweeping the portal of consumer "+consumer)
	if err != nil {
		return err
	}
	if failed != nil {
		return fmt.Errorf("the portal of consumer %q still holds items after %d sweeps: %w", consumer, attempts, failed)
	}
	return nil
}

// sweepPortal signs the consumer in and removes everything the consumer holds on the portal.
func (c consumerSession) sweepPortal(ctx context.Context) error {
	if err := c.signIn(ctx); err != nil {
		return fmt.Errorf("signing the consumer in to clean up: %w", err)
	}
	return errors.Join(c.removeSubscriptions(ctx, ""), c.removeAPIKeys(ctx, ""), c.removeOAuth2Keys(ctx, ""),
		c.removeApplications(ctx))
}

// RegisterConsumerDeleters installs the deleters of what the consumer creates. signIn signs the
// runner's consumer in.
func RegisterConsumerDeleters(reg *cleanup.Registry, signIn func(ctx context.Context) error) error {
	session := consumerSession{signIn: signIn}
	deleters := map[cleanup.Kind]cleanup.Deleter{
		kindConsumerSubscriptions: func(ctx context.Context, resource cleanup.Resource) error {
			return session.removeSubscriptions(ctx, resource.ID)
		},
		kindConsumerAPIKeys: func(ctx context.Context, resource cleanup.Resource) error {
			return session.removeAPIKeys(ctx, resource.ID)
		},
		kindConsumerOAuth2Key: func(ctx context.Context, resource cleanup.Resource) error {
			return session.deleteOAuth2Key(ctx, resource.ID)
		},
		kindConsumerOAuth2KeysNamed: func(ctx context.Context, resource cleanup.Resource) error {
			return session.removeOAuth2Keys(ctx, resource.ID)
		},
		kindConsumerApplication: func(ctx context.Context, resource cleanup.Resource) error {
			return session.deleteApplication(ctx, resource.ID)
		},
		kindConsumerLeftovers: func(ctx context.Context, resource cleanup.Resource) error {
			return session.removeLeftovers(ctx, resource.ID)
		},
	}
	for kind, deleter := range deleters {
		if err := reg.RegisterDeleter(kind, deleter); err != nil {
			return err
		}
	}
	return nil
}

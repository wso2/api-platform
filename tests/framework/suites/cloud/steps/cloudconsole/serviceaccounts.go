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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cucumber/godog"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	serviceAccountsCollection = "/service-accounts"
	// serviceAccountTokenPath is the token exchange. The CP gateway serves it without jwt-auth,
	// since a workload has no token yet; platform-api checks the client credentials.
	serviceAccountTokenPath = serviceAccountsCollection + "/token"
	clientCredentialsGrant  = "client_credentials"
	formContentType         = "application/x-www-form-urlencoded"
	accessTokenField        = "access_token"

	keyServiceAccountID = "cloudServiceAccountID"
	// keyServiceAccountClientID and keyServiceAccountClientSecret hold the account's credentials
	// for ${CTX:...} in the exchange tables. The secret is never published or logged.
	keyServiceAccountClientID     = "cloudServiceAccountClientID"
	keyServiceAccountClientSecret = "cloudServiceAccountClientSecret"
)

// kindServiceAccount is a service account a scenario created. Nothing references one, so its
// place in the deletion order does not matter.
var kindServiceAccount = cleanup.Kind{Name: "service-account", Order: 70}

// registerServiceAccountSteps binds the steps that create a service account and exchange its
// credentials.
func (s *Steps) registerServiceAccountSteps(sc stepRegistrar) {
	sc.Step(`^I create a synthetic service account with the role "([^"]*)"$`, s.createServiceAccount)
	sc.Step(`^I exchange the service account's credentials( using HTTP Basic as "([^"]*)" with secret "([^"]*)")? with:$`,
		s.exchangeServiceAccountCredentials)
	sc.Step(`^the service account's own credentials should be exchanged( using HTTP Basic)? for a token with the scope "([^"]*)"$`,
		s.assertCredentialsExchange)
	sc.Step(`^the cloud API answer should carry no access token$`, s.assertNoAccessToken)
}

// createServiceAccount creates a service account holding role and keeps its credentials for
// the exchange steps. The answer carries the account's secret, so it is never published.
func (s *Steps) createServiceAccount(ctx context.Context, role string) error {
	httpx.ClearPublished(ctx)
	if strings.TrimSpace(role) == "" {
		return fmt.Errorf("a service account needs a role")
	}
	if err := generateResourceAndStore(ctx, "syn-sa", keyServiceAccountID); err != nil {
		return err
	}
	name, err := requiredValue(ctx, keyServiceAccountID)
	if err != nil {
		return err
	}
	// The platform generates the account's id from its display name, so the name is registered
	// before the call: a create that succeeds with an undecodable answer still has something to
	// sweep. A wrong guess is harmless, since its delete answers 404.
	if err := registerServiceAccount(ctx, name); err != nil {
		return err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodPost, serviceAccountsCollection, map[string]any{
		"displayName": name,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
		"roles":       []string{role},
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		if refusedCreate(resp) {
			deregister(ctx, kindServiceAccount, name)
		}
		return responseError(resp)
	}
	// The answer is ServiceAccountCredentials: the account, with its id, next to the credentials.
	var answer struct {
		ServiceAccount struct {
			ID string `json:"id"`
		} `json:"serviceAccount"`
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}
	if err := json.Unmarshal(resp.Body, &answer); err != nil || answer.ServiceAccount.ID == "" {
		return fmt.Errorf("the service account %q was created, but its answer names no id", name)
	}
	created := struct{ ID, ClientID, ClientSecret string }{
		answer.ServiceAccount.ID, answer.ClientID, answer.ClientSecret,
	}
	if created.ID != name {
		// Register the id the platform answered with before dropping the guess, and before the
		// credentials are checked, so the account is never left without a registration. If that
		// fails, nothing would clean it up, so delete it now and fail the step.
		if err := registerServiceAccount(ctx, created.ID); err != nil {
			return errors.Join(err, s.removeUnregisteredServiceAccount(ctx, created.ID))
		}
		deregister(ctx, kindServiceAccount, name)
	}
	if created.ClientID == "" || created.ClientSecret == "" {
		return fmt.Errorf("the service account %q was created, but its answer names no client ID and secret", name)
	}
	for key, value := range map[string]string{
		keyServiceAccountID: created.ID, keyServiceAccountClientID: created.ClientID,
		keyServiceAccountClientSecret: created.ClientSecret,
	} {
		if err := tcontext.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func registerServiceAccount(ctx context.Context, id string) error {
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: kindServiceAccount, ID: id, Actor: resourceActor, Description: "service account " + id,
	}); err != nil {
		return fmt.Errorf("registering service account %q for cleanup: %w", id, err)
	}
	return nil
}

// removeUnregisteredServiceAccount is the best-effort delete of an account that could not be
// registered for cleanup. It returns nil when the account is gone.
func (s *Steps) removeUnregisteredServiceAccount(ctx context.Context, id string) error {
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodDelete, serviceAccountsCollection+"/"+url.PathEscape(id), nil)
	if err != nil {
		return fmt.Errorf("service account %q may still exist: %w", id, err)
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("service account %q may still exist: %s", id, resp.Describe())
	}
	return nil
}

// exchangeServiceAccountCredentials posts a token request to the exchange and publishes the
// answer, with any access token redacted, for the assertion steps. The table rows are the form,
// sent as given, empty values included; grant_type defaults to client_credentials. With HTTP
// Basic the table still supplies the form, so a scenario can send both client authentication
// methods at once.
func (s *Steps) exchangeServiceAccountCredentials(ctx context.Context, basic, clientID, clientSecret string, table *godog.Table) error {
	// A request that fails before it is sent must not leave an earlier answer to be asserted.
	httpx.ClearPublished(ctx)
	form, err := tokenForm(ctx, table)
	if err != nil {
		return err
	}
	authorization := ""
	if basic != "" {
		id, err := expandCloudValue(ctx, clientID)
		if err != nil {
			return err
		}
		secret, err := expandCloudValue(ctx, clientSecret)
		if err != nil {
			return err
		}
		authorization = basicClientCredentials(id, secret)
	}
	resp, err := s.sendTokenRequest(ctx, form, authorization)
	if err != nil {
		return err
	}
	if s.funnel == nil {
		return nil
	}
	return s.funnel.Publish(ctx, redactAccessToken(resp))
}

// assertCredentialsExchange exchanges the account's own credentials for scope and checks that a
// token is issued: the control for the refusals. The token is never published or reported.
func (s *Steps) assertCredentialsExchange(ctx context.Context, basic, scope string) error {
	id, err := requiredValue(ctx, keyServiceAccountClientID)
	if err != nil {
		return err
	}
	secret, err := requiredValue(ctx, keyServiceAccountClientSecret)
	if err != nil {
		return err
	}
	form, authorization := url.Values{
		"grant_type": {clientCredentialsGrant}, "client_id": {id}, "client_secret": {secret}, "scope": {scope},
	}, ""
	if basic != "" {
		// The same credentials in the Authorization header alone, as RFC 6749 section 2.3.1 sends them.
		form.Del("client_id")
		form.Del("client_secret")
		authorization = basicClientCredentials(id, secret)
	}
	resp, err := s.sendTokenRequest(ctx, form, authorization)
	if err != nil {
		return err
	}
	redacted := redactAccessToken(resp)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the exchange refused the account's own credentials: %s", redacted.Describe())
	}
	var issued struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(resp.Body, &issued); err != nil || issued.AccessToken == "" ||
		!strings.EqualFold(issued.TokenType, "Bearer") || issued.ExpiresIn <= 0 {
		return fmt.Errorf("the exchange answer is not a bearer token: %s", redacted.Describe())
	}
	if issued.Scope != scope {
		return fmt.Errorf("the token was issued for the scope %q, not %q", issued.Scope, scope)
	}
	return nil
}

// assertNoAccessToken checks that the published answer carries no access token, redacted or not.
func (s *Steps) assertNoAccessToken(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var answer map[string]any
	if err := json.Unmarshal(response.Body, &answer); err != nil {
		return fmt.Errorf("the answer is not a JSON object: %s", response.Describe())
	}
	if _, carried := answer[accessTokenField]; carried {
		return fmt.Errorf("the answer carries an access token: %s", response.Describe())
	}
	return nil
}

// sendTokenRequest posts form to the exchange without a bearer token. The request is not
// published: it carries the client secret, and its answer may carry a token.
func (s *Steps) sendTokenRequest(ctx context.Context, form url.Values, authorization string) (*httpx.Response, error) {
	base, err := s.topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{"Content-Type": formContentType}
	if authorization != "" {
		headers["Authorization"] = authorization
	}
	return s.client.Do(ctx, httpx.Request{
		Method: http.MethodPost, URL: cloudAPIURL(base, serviceAccountTokenPath),
		Headers: headers, Body: []byte(form.Encode()),
	}, 0, 0)
}

// tokenForm turns a two-column values table into a token request form.
func tokenForm(ctx context.Context, table *godog.Table) (url.Values, error) {
	if table == nil || len(table.Rows) == 0 {
		return nil, fmt.Errorf("the token request needs a values table")
	}
	form := url.Values{}
	for _, row := range table.Rows {
		if len(row.Cells) != 2 {
			return nil, fmt.Errorf("a token request row needs a key and a value, found %d cells", len(row.Cells))
		}
		key := strings.TrimSpace(row.Cells[0].Value)
		if key == "" {
			return nil, fmt.Errorf("a token request row needs a key")
		}
		if form.Has(key) {
			return nil, fmt.Errorf("the token request names %q twice", key)
		}
		value, err := expandCloudValue(ctx, row.Cells[1].Value)
		if err != nil {
			return nil, err
		}
		form.Set(key, value)
	}
	if !form.Has("grant_type") {
		form.Set("grant_type", clientCredentialsGrant)
	}
	return form, nil
}

// basicClientCredentials builds the Authorization header of RFC 6749 section 2.3.1, which
// form-encodes the id and the secret before joining them.
func basicClientCredentials(id, secret string) string {
	pair := url.QueryEscape(id) + ":" + url.QueryEscape(secret)
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(pair))
}

// redactAccessToken returns a copy of resp whose access token is replaced, so a token the
// exchange issues never reaches a published answer or an error message.
func redactAccessToken(resp *httpx.Response) *httpx.Response {
	if resp == nil || len(resp.Body) == 0 {
		return resp
	}
	copied := *resp
	var answer map[string]any
	if json.Unmarshal(resp.Body, &answer) != nil {
		// Not a JSON object. A form-encoded or malformed answer can still carry the token, so
		// one that names the field loses its whole body rather than risk it.
		if bytes.Contains(resp.Body, []byte(accessTokenField)) {
			copied.Body = []byte(redactedValue)
			return &copied
		}
		return resp
	}
	// The decoded key, so an escaped spelling of it (acc\u0065ss_token) is found too.
	if _, present := answer[accessTokenField]; !present {
		return resp
	}
	answer[accessTokenField] = redactedValue
	redacted, err := json.Marshal(answer)
	if err != nil {
		copied.Body = []byte(redactedValue)
		return &copied
	}
	copied.Body = redacted
	return &copied
}

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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package apiportal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	portalcatalog "github.com/wso2/api-platform/tests/framework/core/catalog/apiportal"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
	"github.com/wso2/api-platform/tests/framework/testbench/services/oidc"
)

// idpClientID is the client the multi-organization portals are registered as with the
// testbench identity provider (core/catalog/overlays/api-portal-multi-organization.toml), and
// so the default audience of the tokens minted here.
const idpClientID = "api-portal-it-client"

// portalSharedKey is the shared key the multi-organization portals accept for platform-api's
// publishing calls; its sha256 is internal_auth.hash in api-portal-multi-organization.toml.
const portalSharedKey = "api-portal-multi-organization-it-shared-key"

// maxBrowserHops bounds the redirects one browser navigation follows.
const maxBrowserHops = 8

// Context keys for one simulated browser, suffixed with the scenario's partition and the
// browser's name so every scenario starts with fresh browsers (see browserKey).
const (
	browserCookieKey    = "apiPortalBrowserCookie."
	browserSessionKey   = "apiPortalBrowserIDPSession."
	browserPathKey      = "apiPortalBrowserPath."
	browserAuthorizeKey = "apiPortalBrowserAuthorize."
)

// idpResourceOrder places IDP-created resources in the same teardown order as the platform-
// authenticated kinds in portal.go: subscribers and MCP servers with APIs, seeded rows last.
const (
	idpAPIKeyOrder     = 10
	idpSubscriberOrder = 60
	idpAPIOrder        = 40
	seededRowsOrder    = 90
)

// registerMultiOrganizationSteps binds the steps that drive the API Portal in IDP and
// multi-organization mode: tokens from the testbench identity provider, browser sign-in
// through it, shared-key publishing, and the database state those modes leave behind.
func (s *Steps) registerMultiOrganizationSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I mint an API Portal IDP token stored as "([^"]*)" with claims:$`, s.mintIDPToken)
	sc.Step(`^I send an API Portal "([^"]*)" request to "([^"]*)" using portal "([^"]*)" with token "([^"]*)"$`,
		s.sendWithToken)
	sc.Step(`^I send an API Portal "([^"]*)" request to "([^"]*)" using portal "([^"]*)" with token "([^"]*)" and JSON body:$`,
		s.sendWithTokenAndBody)
	sc.Step(`^(\d+) concurrent API Portal "([^"]*)" requests to "([^"]*)" using portal "([^"]*)" with token "([^"]*)" should all return status (\d+)$`,
		s.sendConcurrentlyWithToken)
	sc.Step(`^a unique API Portal REST API is created in portal "([^"]*)" with token "([^"]*)" and stored as "([^"]*)"$`,
		s.createAPIWithToken)
	sc.Step(`^a unique API Portal MCP server is created in portal "([^"]*)" with token "([^"]*)" and stored as "([^"]*)"$`,
		s.createMCPServerWithToken)
	sc.Step(`^an API Portal webhook subscriber for events "([^"]*)" delivering to sink "([^"]*)" is registered in portal "([^"]*)" with token "([^"]*)"$`,
		s.createWebhookSubscriberWithToken)
	sc.Step(`^I generate (\d+) API Portal API keys for API "([^"]*)" in portal "([^"]*)" with token "([^"]*)"$`,
		s.generateAPIKeysWithToken)
	sc.Step(`^the API Portal webhook sink "([^"]*)" should receive exactly (\d+) "([^"]*)" events for organization "([^"]*)"$`,
		s.assertSinkReceivesExactly)
	sc.Step(`^I publish a unique REST API to API Portal "([^"]*)" with shared key "([^"]*)" and store its id as "([^"]*)"$`,
		s.publishWithSharedKey)
	sc.Step(`^I publish a unique REST API to API Portal "([^"]*)" with the shared key and store its id as "([^"]*)"$`,
		s.publishWithTheSharedKey)
	sc.Step(`^I publish a unique REST API to API Portal "([^"]*)" with the shared key and header "([^"]*)" set to "([^"]*)" and store its id as "([^"]*)"$`,
		s.publishWithTheSharedKeyAndHeader)
	sc.Step(`^the API Portal browser "([^"]*)" has an IDP session with claims:$`, s.setBrowserIDPSession)
	sc.Step(`^the API Portal browser "([^"]*)" has no IDP session$`, s.clearBrowserIDPSession)
	sc.Step(`^I browse API Portal page "([^"]*)" using portal "([^"]*)" as browser "([^"]*)"$`, s.browse)
	sc.Step(`^I sign in to API Portal "([^"]*)" from "([^"]*)" as browser "([^"]*)"$`, s.signIn)
	sc.Step(`^I send an API Portal "([^"]*)" request to "([^"]*)" using portal "([^"]*)" as browser "([^"]*)"$`,
		s.sendAsBrowser)
	sc.Step(`^the API Portal browser "([^"]*)" should be on page "([^"]*)"$`, s.assertBrowserPage)
	sc.Step(`^the API Portal browser "([^"]*)" last IDP authorization parameter "([^"]*)" should be "([^"]*)"$`,
		s.assertAuthorizeParameter)
	sc.Step(`^the API Portal browser "([^"]*)" last IDP authorization parameter "([^"]*)" should be absent$`,
		s.assertAuthorizeParameterAbsent)
	sc.Step(`^the API Portal redirect parameter "([^"]*)" should be "([^"]*)"$`, s.assertRedirectParameter)
	sc.Step(`^the API Portal redirect parameter "([^"]*)" should be absent$`, s.assertRedirectParameterAbsent)
	sc.Step(`^the API Portal "([^"]*)" should have (\d+) organizations? with IDP reference "([^"]*)"$`,
		s.assertOrganizationCount)
	sc.Step(`^the API Portal "([^"]*)" organization with IDP reference "([^"]*)" should have handle "([^"]*)" and display name "([^"]*)"$`,
		s.assertOrganizationNames)
	sc.Step(`^the API Portal "([^"]*)" organization with IDP reference "([^"]*)" should have a handle matching "([^"]*)"$`,
		s.assertOrganizationHandlePattern)
	sc.Step(`^I store the API Portal "([^"]*)" organization id for IDP reference "([^"]*)" as "([^"]*)"$`,
		s.storeOrganizationID)
	sc.Step(`^API Portal rows for a portal_id no portal serves, with an organization whose IDP reference is "([^"]*)", are seeded in the database of "([^"]*)" and stored as "([^"]*)"$`,
		s.seedUnownedPortalRows)
	sc.Step(`^the seeded API Portal rows "([^"]*)" should stay untouched$`, s.assertSeededRowsUntouched)
}

// newIdentityProviderClient returns a client that verifies the testbench identity provider's
// certificate by the host name it is issued for, whatever address the test process dials.
func newIdentityProviderClient() (*httpx.Client, error) {
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(shared.IdentityProviderTLS().CertPEM) {
		return nil, errors.New("API Portal: the identity provider certificate is not valid PEM")
	}
	return httpx.NewClient(httpx.Options{TLSClientConfig: &tls.Config{
		RootCAs: roots, ServerName: shared.IdentityProviderHost, MinVersion: tls.VersionTLS12,
	}}), nil
}

// claimsFromTable reads a two-column claim table. A value that is a JSON array, object or
// quoted string is decoded, so a claim can be a list, a map, or a string Gherkin would
// otherwise trim; any other value is a string.
func claimsFromTable(ctx context.Context, table *godog.Table) (map[string]any, error) {
	if table == nil {
		return nil, errors.New("API Portal claims table is required")
	}
	claims := make(map[string]any, len(table.Rows))
	for i, row := range table.Rows {
		if len(row.Cells) != 2 {
			return nil, fmt.Errorf("API Portal claims row %d must have exactly two cells", i+1)
		}
		name := strings.TrimSpace(row.Cells[0].Value)
		if name == "" {
			return nil, fmt.Errorf("API Portal claims row %d has an empty claim name", i+1)
		}
		if _, duplicate := claims[name]; duplicate {
			return nil, fmt.Errorf("API Portal claim %q is given twice", name)
		}
		value, err := stepscommon.Expand(ctx, row.Cells[1].Value)
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(value)
		if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, `"`) {
			var decoded any
			if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
				return nil, fmt.Errorf("API Portal claim %q is not valid JSON: %w", name, err)
			}
			claims[name] = decoded
			continue
		}
		claims[name] = value
	}
	if sub, _ := claims["sub"].(string); sub == "" {
		return nil, errors.New("API Portal claims must include a sub")
	}
	return claims, nil
}

func (s *Steps) mintIDPToken(ctx context.Context, storeAs string, table *godog.Table) error {
	claims, err := claimsFromTable(ctx, table)
	if err != nil {
		return err
	}
	if _, ok := claims["aud"]; !ok {
		claims["aud"] = idpClientID
	}
	body, err := json.Marshal(claims)
	if err != nil {
		return fmt.Errorf("encoding API Portal IDP claims: %w", err)
	}
	base, err := s.topo.URL("testbench", "oidc")
	if err != nil {
		return err
	}
	response, err := s.idp.Do(ctx, httpx.Request{
		Method: http.MethodPost, URL: base + "/mint", Body: body, ContentType: "application/json",
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("minting an API Portal IDP token: %w", err)
	}
	if err := response.RequireSuccessWithBody("minting an API Portal IDP token"); err != nil {
		return err
	}
	var minted struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(response.Body, &minted); err != nil || minted.Token == "" {
		return fmt.Errorf("the identity provider returned no token: %s", response.Describe())
	}
	return tcontext.Set(ctx, storeAs, minted.Token)
}

// tokenRequest issues one bearer-authenticated request and publishes its response.
func (s *Steps) tokenRequest(ctx context.Context, method, path, portal, tokenKey string, body []byte) error {
	tcontext.Remove(ctx, httpx.ResponseKey)
	request, err := s.bearerRequest(ctx, method, path, portal, tokenKey, body)
	if err != nil {
		return err
	}
	response, err := s.client.Do(ctx, request, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal %s %s: %w", method, path, err)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

func (s *Steps) bearerRequest(ctx context.Context, method, path, portal, tokenKey string, body []byte) (httpx.Request, error) {
	base, err := s.portalBaseNamed(portal)
	if err != nil {
		return httpx.Request{}, err
	}
	path, err = stepscommon.Expand(ctx, path)
	if err != nil {
		return httpx.Request{}, err
	}
	token, err := tcontext.ResolveString(ctx, tokenKey)
	if err != nil {
		return httpx.Request{}, fmt.Errorf("API Portal token %q is unavailable: %w", tokenKey, err)
	}
	request := httpx.Request{
		Method: strings.ToUpper(method), URL: s.portalRequestURL(base, path),
		Headers: map[string]string{"Authorization": "Bearer " + token}, Body: body,
	}
	if len(body) > 0 {
		request.ContentType = "application/json"
	}
	return request, nil
}

func (s *Steps) sendWithToken(ctx context.Context, method, path, portal, tokenKey string) error {
	return s.tokenRequest(ctx, method, path, portal, tokenKey, nil)
}

func (s *Steps) sendWithTokenAndBody(ctx context.Context, method, path, portal, tokenKey string, body *godog.DocString) error {
	if body == nil {
		return errors.New("API Portal request body is required")
	}
	expanded, err := stepscommon.Expand(ctx, body.Content)
	if err != nil {
		return err
	}
	if !json.Valid([]byte(expanded)) {
		return errors.New("API Portal request body must be JSON")
	}
	return s.tokenRequest(ctx, method, path, portal, tokenKey, []byte(expanded))
}

func (s *Steps) sendConcurrentlyWithToken(ctx context.Context, count int, method, path, portal, tokenKey string, want int) error {
	if count <= 0 {
		return fmt.Errorf("API Portal concurrent request count must be positive, got %d", count)
	}
	tcontext.Remove(ctx, httpx.ResponseKey)
	request, err := s.bearerRequest(ctx, method, path, portal, tokenKey, nil)
	if err != nil {
		return err
	}
	responses := make([]*httpx.Response, count)
	errs := make([]error, count)
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			responses[i], errs[i] = s.client.Do(ctx, request, 0, 0)
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("API Portal concurrent %s %s: %w", method, path, err)
	}
	for i, response := range responses {
		if response.StatusCode != want {
			return fmt.Errorf("API Portal concurrent request %d of %d: want status %d, got %s", i+1, count, want, response.Describe())
		}
	}
	return tcontext.Set(ctx, httpx.ResponseKey, responses[count-1])
}

// idpRemoval is how a resource created with an IDP or shared-key credential is removed: a
// DELETE of its path, or for an API key a POST of its revocation.
type idpRemoval struct {
	method string
	path   string
	body   []byte
}

// idpCleanup prepares scenario cleanup for a resource removed with the credential that created
// it, and returns the registry and resource the caller registers. Each resource gets a kind of
// its own, because the deleter holds that resource's portal and credential rather than
// resolving an actor. The returned function removes the resource at once, for a caller whose
// registration failed.
func (s *Steps) idpCleanup(ctx context.Context, name string, order int, id, portal string, removal idpRemoval, headers map[string]string, actor string) (*cleanup.Registry, cleanup.Resource, func(context.Context) error, error) {
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return nil, cleanup.Resource{}, nil, err
	}
	kind := cleanup.Kind{Name: name + "/" + portal + "/" + id, Order: order}
	deleter := func(ctx context.Context, _ cleanup.Resource) error {
		base, err := s.portalBaseNamed(portal)
		if err != nil {
			return err
		}
		request := httpx.Request{Method: removal.method, URL: base + apiPrefix + removal.path, Headers: headers, Body: removal.body}
		if len(removal.body) > 0 {
			request.ContentType = "application/json"
		}
		response, err := s.client.Do(ctx, request, 0, 0)
		if err != nil {
			return err
		}
		// Already gone: deleted, or for a key already revoked, by the scenario itself.
		if response.Succeeded() || response.StatusCode == http.StatusNotFound ||
			(removal.method == http.MethodPost && response.StatusCode == http.StatusConflict) {
			return nil
		}
		return fmt.Errorf("deleting API Portal %s %q failed: %s", name, id, response.Describe())
	}
	deleteNow := func(ctx context.Context) error { return deleter(ctx, cleanup.Resource{}) }
	if err := registry.RegisterDeleter(kind, deleter); err != nil {
		return nil, cleanup.Resource{}, nil, errors.Join(err, deleteNow(ctx))
	}
	return registry, cleanup.Resource{Kind: kind, ID: id, Actor: actor, Description: "created in " + portal}, deleteNow, nil
}

func deletion(path string) idpRemoval { return idpRemoval{method: http.MethodDelete, path: path} }

// restAPIMetadata is the smallest published REST API the portal accepts, under id.
func restAPIMetadata(id string) string {
	metadata, _ := json.Marshal(map[string]any{
		"id": id, "name": "Multi-organization " + id, "version": "v1.0", "type": "REST", "status": "PUBLISHED",
		"endPoints": map[string]string{
			"productionURL": "https://backend.example.invalid/" + id,
			"sandboxURL":    "https://sandbox.example.invalid/" + id,
		},
	})
	return string(metadata)
}

// createAPI publishes a REST API with the given credential headers and registers it for
// cleanup with the same headers.
func (s *Steps) createAPI(ctx context.Context, portal string, headers map[string]string, actor, storeAs string) (*httpx.Response, error) {
	id, err := unique.Unique(ctx, "mt-api")
	if err != nil {
		return nil, err
	}
	id = strings.ToLower(strings.ReplaceAll(id, "_", "-"))
	if err := tcontext.Set(ctx, storeAs, id); err != nil {
		return nil, err
	}
	base, err := s.portalBaseNamed(portal)
	if err != nil {
		return nil, err
	}
	response, err := s.multipart(ctx, base, "/apis", headers, restAPIMetadata(id), portalRESTDefinition, "definition.json", "application/json")
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusCreated {
		return response, nil
	}
	registry, resource, deleteNow, err := s.idpCleanup(ctx, "api-portal-idp-api", idpAPIOrder, id, portal, deletion("/apis/"+url.PathEscape(id)), headers, actor)
	if err != nil {
		return nil, err
	}
	if err := registry.RegisterScoped(cleanup.ScenarioScope, resource); err != nil {
		return nil, errors.Join(err, deleteNow(ctx))
	}
	return response, nil
}

// multipart posts API metadata and a definition to the portal's REST API.
func (s *Steps) multipart(ctx context.Context, base, path string, headers map[string]string, metadata, definition, filename, contentType string) (*httpx.Response, error) {
	var body strings.Builder
	boundary := "api-portal-multi-organization-boundary"
	fmt.Fprintf(&body, "--%s\r\nContent-Disposition: form-data; name=\"metadata\"\r\n\r\n%s\r\n", boundary, metadata)
	fmt.Fprintf(&body, "--%s\r\nContent-Disposition: form-data; name=\"definition\"; filename=%q\r\nContent-Type: %s\r\n\r\n%s\r\n",
		boundary, filename, contentType, definition)
	fmt.Fprintf(&body, "--%s--\r\n", boundary)
	response, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodPost, URL: base + apiPrefix + path, Headers: headers,
		Body: []byte(body.String()), ContentType: "multipart/form-data; boundary=" + boundary,
	}, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("API Portal POST %s: %w", path, err)
	}
	return response, nil
}

func (s *Steps) bearerHeaders(ctx context.Context, tokenKey string) (map[string]string, error) {
	token, err := tcontext.ResolveString(ctx, tokenKey)
	if err != nil {
		return nil, fmt.Errorf("API Portal token %q is unavailable: %w", tokenKey, err)
	}
	return map[string]string{"Authorization": "Bearer " + token}, nil
}

func (s *Steps) createAPIWithToken(ctx context.Context, portal, tokenKey, storeAs string) error {
	headers, err := s.bearerHeaders(ctx, tokenKey)
	if err != nil {
		return err
	}
	response, err := s.createAPI(ctx, portal, headers, "token:"+tokenKey, storeAs)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("creating an API Portal REST API: %s", response.Describe())
	}
	return nil
}

func (s *Steps) createMCPServerWithToken(ctx context.Context, portal, tokenKey, storeAs string) error {
	headers, err := s.bearerHeaders(ctx, tokenKey)
	if err != nil {
		return err
	}
	id, err := unique.Unique(ctx, "mt-mcp")
	if err != nil {
		return err
	}
	id = strings.ToLower(strings.ReplaceAll(id, "_", "-"))
	metadata, _ := json.Marshal(map[string]any{
		"id": id, "name": id, "version": "1.0.0", "type": "MCP", "status": "PUBLISHED",
		"endPoints": map[string]string{"productionURL": "https://mcp.example.invalid/" + id},
	})
	base, err := s.portalBaseNamed(portal)
	if err != nil {
		return err
	}
	response, err := s.multipart(ctx, base, "/mcp-servers", headers, string(metadata), portalMCPDefinition, "definition.yaml", "application/yaml")
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("creating an API Portal MCP server: %s", response.Describe())
	}
	registry, resource, deleteNow, err := s.idpCleanup(ctx, "api-portal-idp-mcp-server", idpAPIOrder, id, portal, deletion("/mcp-servers/"+url.PathEscape(id)), headers, "token:"+tokenKey)
	if err != nil {
		return err
	}
	if err := registry.RegisterScoped(cleanup.ScenarioScope, resource); err != nil {
		return errors.Join(err, deleteNow(ctx))
	}
	return tcontext.Set(ctx, storeAs, id)
}

// sinkPartition is the webhook partition a named sink records into: the scenario's own
// partition, suffixed so several sinks in one scenario stay apart.
func (s *Steps) sinkPartition(ctx context.Context, sink string) (string, error) {
	partition, err := s.webhookPartition(ctx)
	if err != nil {
		return "", err
	}
	sink = strings.TrimSpace(sink)
	if sink == "" || strings.ContainsAny(sink, "/?#") {
		return "", fmt.Errorf("API Portal webhook sink name %q is not a path segment", sink)
	}
	return partition + "-" + sink, nil
}

func (s *Steps) createWebhookSubscriberWithToken(ctx context.Context, events, sink, portal, tokenKey string) error {
	headers, err := s.bearerHeaders(ctx, tokenKey)
	if err != nil {
		return err
	}
	partition, err := s.sinkPartition(ctx, sink)
	if err != nil {
		return err
	}
	id, err := unique.Unique(ctx, "mt-subscriber")
	if err != nil {
		return err
	}
	id = strings.ToLower(strings.ReplaceAll(id, "_", "-"))
	var eventList []string
	for _, event := range strings.Split(events, ",") {
		if event = strings.TrimSpace(event); event != "" {
			eventList = append(eventList, event)
		}
	}
	payload, _ := json.Marshal(map[string]any{
		"id": id, "displayName": "Sink " + sink, "events": eventList, "enabled": true,
		"targetUrl": "http://testbench:3012/" + partition + "/webhook",
	})
	base, err := s.portalBaseNamed(portal)
	if err != nil {
		return err
	}
	response, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodPost, URL: base + apiPrefix + "/webhook-subscribers", Headers: headers,
		Body: payload, ContentType: "application/json",
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("registering an API Portal webhook subscriber: %w", err)
	}
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("registering an API Portal webhook subscriber: %s", response.Describe())
	}
	registry, resource, deleteNow, err := s.idpCleanup(ctx, "api-portal-idp-webhook-subscriber", idpSubscriberOrder, id, portal,
		deletion("/webhook-subscribers/"+url.PathEscape(id)), headers, "token:"+tokenKey)
	if err != nil {
		return err
	}
	if err := registry.RegisterScoped(cleanup.ScenarioScope, resource); err != nil {
		return errors.Join(err, deleteNow(ctx))
	}
	return nil
}

// generateAPIKey generates one API key and registers its revocation, which must precede its
// API's deletion: the portal refuses to delete an API that still has active keys.
func (s *Steps) generateAPIKey(ctx context.Context, portal, apiID string, headers map[string]string) error {
	base, err := s.portalBaseNamed(portal)
	if err != nil {
		return err
	}
	name, err := unique.Unique(ctx, "mt-key")
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"id": strings.ToLower(strings.ReplaceAll(name, "_", "-"))})
	response, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodPost, URL: base + apiPrefix + "/apis/" + url.PathEscape(apiID) + "/api-keys/generate",
		Headers: headers, Body: payload, ContentType: "application/json",
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("generating an API Portal API key: %w", err)
	}
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("generating an API Portal API key in %s: %s", portal, response.Describe())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body, &created); err != nil || created.ID == "" {
		return fmt.Errorf("the API Portal API key response carried no id: %s", response.Describe())
	}
	revoke, _ := json.Marshal(map[string]string{"keyId": created.ID})
	registry, resource, revokeNow, err := s.idpCleanup(ctx, "api-portal-idp-api-key", idpAPIKeyOrder, created.ID, portal,
		idpRemoval{method: http.MethodPost, path: "/apis/" + url.PathEscape(apiID) + "/api-keys/revoke", body: revoke},
		headers, "bearer")
	if err != nil {
		return err
	}
	if err := registry.RegisterScoped(cleanup.ScenarioScope, resource); err != nil {
		return errors.Join(err, revokeNow(ctx))
	}
	return nil
}

// generateAPIKeysWithToken generates count keys, one after another, for the API stored under
// apiKey.
func (s *Steps) generateAPIKeysWithToken(ctx context.Context, count int, apiKey, portal, tokenKey string) error {
	if count <= 0 {
		return fmt.Errorf("API Portal API key count must be positive, got %d", count)
	}
	apiID, err := tcontext.ResolveString(ctx, apiKey)
	if err != nil {
		return err
	}
	headers, err := s.bearerHeaders(ctx, tokenKey)
	if err != nil {
		return err
	}
	for range count {
		if err := s.generateAPIKey(ctx, portal, apiID, headers); err != nil {
			return err
		}
	}
	return nil
}

// sinkDeliveries returns the bodies a named sink has received.
func (s *Steps) sinkDeliveries(ctx context.Context, sink string) ([]map[string]any, error) {
	partition, err := s.sinkPartition(ctx, sink)
	if err != nil {
		return nil, err
	}
	base, err := s.topo.URL("testbench", "webhook")
	if err != nil {
		return nil, err
	}
	response, err := s.client.Do(ctx, httpx.Request{Method: http.MethodGet, URL: base + "/" + partition + "/test/deliveries"}, 0, 0)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if !response.Succeeded() {
		return nil, retry.Transient(fmt.Errorf("listing webhook sink %q: %s", sink, response.Describe()))
	}
	var deliveries []struct {
		Body map[string]any `json:"body"`
	}
	if err := json.Unmarshal(response.Body, &deliveries); err != nil {
		return nil, fmt.Errorf("decoding webhook sink %q deliveries: %w", sink, err)
	}
	bodies := make([]map[string]any, 0, len(deliveries))
	for _, delivery := range deliveries {
		bodies = append(bodies, delivery.Body)
	}
	return bodies, nil
}

// assertSinkReceivesExactly waits until the sink's count of eventType deliveries settles and
// asserts it is want, each a distinct event of the given organization.
func (s *Steps) assertSinkReceivesExactly(ctx context.Context, sink string, want int, eventType, orgKey string) error {
	orgID, err := stepscommon.Expand(ctx, orgKey)
	if err != nil {
		return err
	}
	var matching []map[string]any
	settled, err := retry.SettledCount(ctx, retry.Options{Interval: time.Second}, 4*time.Second,
		func(ctx context.Context) (int, error) {
			bodies, err := s.sinkDeliveries(ctx, sink)
			if err != nil {
				return 0, err
			}
			matching = matching[:0]
			for _, body := range bodies {
				if body["event_type"] == eventType {
					matching = append(matching, body)
				}
			}
			return len(matching), nil
		})
	if err != nil {
		return fmt.Errorf("waiting for webhook sink %q: %w", sink, err)
	}
	if !settled.Quiet {
		return fmt.Errorf("webhook sink %q was still receiving %q events at the deadline (%d so far)", sink, eventType, settled.Value)
	}
	if settled.Value != want {
		return fmt.Errorf("webhook sink %q received %d %q events, want exactly %d", sink, settled.Value, eventType, want)
	}
	seen := make(map[string]bool, len(matching))
	for _, body := range matching {
		eventID, _ := body["event_id"].(string)
		if eventID == "" || seen[eventID] {
			return fmt.Errorf("webhook sink %q received event %q more than once or without an id", sink, eventID)
		}
		seen[eventID] = true
		org, _ := body["org"].(map[string]any)
		if ref, _ := org["ref_id"].(string); ref != orgID {
			return fmt.Errorf("webhook sink %q received an event of organization %q, want %q", sink, ref, orgID)
		}
	}
	return nil
}

func (s *Steps) publishWithSharedKey(ctx context.Context, portal, key, storeAs string) error {
	key, err := stepscommon.Expand(ctx, key)
	if err != nil {
		return err
	}
	return s.publishShared(ctx, portal, map[string]string{"Authorization": "SharedKey " + key}, storeAs)
}

func (s *Steps) publishWithTheSharedKey(ctx context.Context, portal, storeAs string) error {
	return s.publishShared(ctx, portal, map[string]string{"Authorization": "SharedKey " + portalSharedKey}, storeAs)
}

func (s *Steps) publishWithTheSharedKeyAndHeader(ctx context.Context, portal, header, value, storeAs string) error {
	value, err := stepscommon.Expand(ctx, value)
	if err != nil {
		return err
	}
	return s.publishShared(ctx, portal, map[string]string{
		"Authorization": "SharedKey " + portalSharedKey, strings.TrimSpace(header): value,
	}, storeAs)
}

// publishShared publishes a REST API the way platform-api does and publishes the response.
// Cleanup deletes it with the shared key, without the extra headers.
func (s *Steps) publishShared(ctx context.Context, portal string, headers map[string]string, storeAs string) error {
	tcontext.Remove(ctx, httpx.ResponseKey)
	response, err := s.createAPI(ctx, portal, headers, "shared-key", storeAs)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

func (s *Steps) setBrowserIDPSession(ctx context.Context, browser string, table *godog.Table) error {
	claims, err := claimsFromTable(ctx, table)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(claims)
	if err != nil {
		return fmt.Errorf("encoding the API Portal browser IDP session: %w", err)
	}
	return s.setBrowserValue(ctx, browserSessionKey, browser, base64.RawURLEncoding.EncodeToString(raw))
}

func (s *Steps) clearBrowserIDPSession(ctx context.Context, browser string) error {
	return s.setBrowserValue(ctx, browserSessionKey, browser, "")
}

// browserKey names one piece of a browser's state. Runner-local context outlives a scenario,
// so the key carries the scenario's own partition: a later scenario's browser of the same name
// starts without the earlier one's cookies or IDP session.
func (s *Steps) browserKey(ctx context.Context, prefix, browser string) (string, error) {
	partition, err := s.webhookPartition(ctx)
	if err != nil {
		return "", err
	}
	return prefix + partition + "." + browser, nil
}

func (s *Steps) setBrowserValue(ctx context.Context, prefix, browser, value string) error {
	key, err := s.browserKey(ctx, prefix, browser)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, key, value)
}

func (s *Steps) browserValue(ctx context.Context, prefix, browser string) string {
	key, err := s.browserKey(ctx, prefix, browser)
	if err != nil {
		return ""
	}
	value, err := tcontext.ResolveString(ctx, key)
	if err != nil {
		return ""
	}
	return value
}

// visit performs one browser request against the portal, sending and keeping its cookies.
func (s *Steps) visit(ctx context.Context, portal, browser, target string) (*httpx.Response, error) {
	base, err := s.portalBaseNamed(portal)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{}
	if cookie := s.browserValue(ctx, browserCookieKey, browser); cookie != "" {
		headers["Cookie"] = cookie
	}
	response, err := s.client.Do(ctx, httpx.Request{Method: http.MethodGet, URL: s.portalRequestURL(base, target), Headers: headers}, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("API Portal browser GET %s: %w", target, err)
	}
	if cookies := response.Headers.Values("Set-Cookie"); len(cookies) > 0 {
		merged := mergeCookieHeader(s.browserValue(ctx, browserCookieKey, browser), cookies)
		if err := s.setBrowserValue(ctx, browserCookieKey, browser, merged); err != nil {
			return nil, err
		}
	}
	return response, nil
}

// authorizeAtIDP sends the portal's authorization redirect to the identity provider, signed in
// as the browser's IDP session if it has one, and returns where the provider redirects to.
func (s *Steps) authorizeAtIDP(ctx context.Context, browser string, location *url.URL) (*url.URL, error) {
	if err := s.setBrowserValue(ctx, browserAuthorizeKey, browser, location.RawQuery); err != nil {
		return nil, err
	}
	base, err := s.topo.URL("testbench", "oidc")
	if err != nil {
		return nil, err
	}
	headers := map[string]string{}
	if session := s.browserValue(ctx, browserSessionKey, browser); session != "" {
		headers[oidc.LoginHeader] = session
	}
	response, err := s.idp.Do(ctx, httpx.Request{
		Method: http.MethodGet, URL: base + location.Path + "?" + location.RawQuery, Headers: headers,
	}, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("API Portal browser authorization at the identity provider: %w", err)
	}
	if response.StatusCode != http.StatusFound {
		return nil, fmt.Errorf("the identity provider did not redirect back: %s", response.Describe())
	}
	return url.Parse(response.Headers.Get("Location"))
}

// isIdentityProvider reports whether a redirect leaves the portal for the identity provider.
func isIdentityProvider(location *url.URL) bool {
	return location.IsAbs() && location.Hostname() == shared.IdentityProviderHost
}

// portalTarget is the path and query a portal redirect names, whatever host it carries: the
// portal addresses itself by its container alias, which only resolves on the block network.
func portalTarget(location *url.URL) string {
	target := location.EscapedPath()
	if location.RawQuery != "" {
		target += "?" + location.RawQuery
	}
	return target
}

// browse follows redirects the way a browser does, through the identity provider and back,
// and publishes the final response.
func (s *Steps) browse(ctx context.Context, page, portal, browser string) error {
	tcontext.Remove(ctx, httpx.ResponseKey)
	page, err := stepscommon.Expand(ctx, page)
	if err != nil {
		return err
	}
	target := page
	response, err := s.visit(ctx, portal, browser, target)
	if err != nil {
		return err
	}
	for hops := 0; isRedirect(response.StatusCode); hops++ {
		if hops == maxBrowserHops {
			return fmt.Errorf("API Portal browser %q exceeded %d redirects from %s", browser, maxBrowserHops, page)
		}
		location, err := url.Parse(response.Headers.Get("Location"))
		if err != nil {
			return fmt.Errorf("API Portal browser %q got an unparseable redirect: %w", browser, err)
		}
		if isIdentityProvider(location) {
			if location, err = s.authorizeAtIDP(ctx, browser, location); err != nil {
				return err
			}
		}
		target = portalTarget(location)
		if response, err = s.visit(ctx, portal, browser, target); err != nil {
			return err
		}
	}
	path, _, _ := strings.Cut(target, "?")
	if err := s.setBrowserValue(ctx, browserPathKey, browser, path); err != nil {
		return err
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

func isRedirect(status int) bool {
	return status == http.StatusFound || status == http.StatusSeeOther ||
		status == http.StatusMovedPermanently || status == http.StatusTemporaryRedirect
}

// signIn starts a login at a portal page, completes it at the identity provider as the
// browser's IDP session, and publishes the portal's response to the callback.
func (s *Steps) signIn(ctx context.Context, portal, page, browser string) error {
	tcontext.Remove(ctx, httpx.ResponseKey)
	page, err := stepscommon.Expand(ctx, page)
	if err != nil {
		return err
	}
	response, err := s.visit(ctx, portal, browser, page)
	if err != nil {
		return err
	}
	location, err := url.Parse(response.Headers.Get("Location"))
	if err != nil || !isRedirect(response.StatusCode) || !isIdentityProvider(location) {
		return fmt.Errorf("API Portal login from %s did not redirect to the identity provider: %s", page, response.Describe())
	}
	callback, err := s.authorizeAtIDP(ctx, browser, location)
	if err != nil {
		return err
	}
	if response, err = s.visit(ctx, portal, browser, portalTarget(callback)); err != nil {
		return err
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

func (s *Steps) sendAsBrowser(ctx context.Context, method, path, portal, browser string) error {
	if !strings.EqualFold(method, http.MethodGet) {
		return fmt.Errorf("API Portal browser requests are GETs, got %q", method)
	}
	tcontext.Remove(ctx, httpx.ResponseKey)
	path, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	response, err := s.visit(ctx, portal, browser, path)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

func (s *Steps) assertBrowserPage(ctx context.Context, browser, want string) error {
	want, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	if got := s.browserValue(ctx, browserPathKey, browser); got != want {
		return fmt.Errorf("API Portal browser %q is on %q, want %q", browser, got, want)
	}
	return nil
}

func (s *Steps) authorizeParameters(ctx context.Context, browser string) (url.Values, error) {
	key, err := s.browserKey(ctx, browserAuthorizeKey, browser)
	if err != nil {
		return nil, err
	}
	raw, err := tcontext.ResolveString(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("API Portal browser %q has not been to the identity provider: %w", browser, err)
	}
	return url.ParseQuery(raw)
}

func (s *Steps) assertAuthorizeParameter(ctx context.Context, browser, name, want string) error {
	want, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	values, err := s.authorizeParameters(ctx, browser)
	if err != nil {
		return err
	}
	if got, present := values[name]; !present || got[0] != want {
		return fmt.Errorf("the identity provider was sent %s=%v, want %q", name, got, want)
	}
	return nil
}

func (s *Steps) assertAuthorizeParameterAbsent(ctx context.Context, browser, name string) error {
	values, err := s.authorizeParameters(ctx, browser)
	if err != nil {
		return err
	}
	if got, present := values[name]; present {
		return fmt.Errorf("the identity provider was sent %s=%v, want no %s", name, got, name)
	}
	return nil
}

func (s *Steps) redirectParameters(ctx context.Context) (url.Values, error) {
	response, err := httpx.Published(ctx)
	if err != nil {
		return nil, err
	}
	location, err := url.Parse(response.Headers.Get("Location"))
	if err != nil || !isRedirect(response.StatusCode) {
		return nil, fmt.Errorf("the response is not a redirect: %s", response.Describe())
	}
	return location.Query(), nil
}

func (s *Steps) assertRedirectParameter(ctx context.Context, name, want string) error {
	want, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	values, err := s.redirectParameters(ctx)
	if err != nil {
		return err
	}
	if got, present := values[name]; !present || got[0] != want {
		return fmt.Errorf("the redirect carries %s=%v, want %q", name, got, want)
	}
	return nil
}

func (s *Steps) assertRedirectParameterAbsent(ctx context.Context, name string) error {
	values, err := s.redirectParameters(ctx)
	if err != nil {
		return err
	}
	if got, present := values[name]; present {
		return fmt.Errorf("the redirect carries %s=%v, want none", name, got)
	}
	return nil
}

// portalDB opens the database a multi-organization portal component uses and returns it with
// the portal_id that component serves.
func (s *Steps) portalDB(ctx context.Context, portal string) (*portalDatabase, error) {
	portalID, ok := portalcatalog.PortalID(portal)
	if !ok {
		return nil, fmt.Errorf("API Portal component %q has no known portal_id", portal)
	}
	store, ok := s.topo.Storage.Plan.StoreFor(portal, 0)
	if !ok {
		return nil, fmt.Errorf("API Portal component %q has no database", portal)
	}
	db, closeDB, err := s.topo.OpenComponentDB(ctx, portal, "")
	if err != nil {
		return nil, err
	}
	return &portalDatabase{db: db, close: closeDB, engine: store.Type, portalID: portalID}, nil
}

// portalDatabase is a direct connection to a portal's database.
type portalDatabase struct {
	db       *sql.DB
	close    func() error
	engine   components.DBType
	portalID string
}

// bind rewrites ?-placeholders into the engine's own form.
func (p *portalDatabase) bind(query string) string {
	var marker func(int) string
	switch p.engine {
	case components.Postgres:
		marker = func(n int) string { return "$" + strconv.Itoa(n) }
	case components.SQLServer:
		marker = func(n int) string { return "@p" + strconv.Itoa(n) }
	default:
		return query
	}
	var out strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			out.WriteString(marker(n))
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

// organizationRow is what the steps assert about a stored organization.
type organizationRow struct {
	uuid, handle, displayName, idpRefID string
}

// organizations returns the portal's organizations whose idp_ref_id is exactly idpRefID. The
// comparison is repeated here because SQL Server's default collation matches case- and
// trailing-space-insensitively.
func (p *portalDatabase) organizations(ctx context.Context, idpRefID string) ([]organizationRow, error) {
	rows, err := p.db.QueryContext(ctx, p.bind(
		"SELECT uuid, handle, display_name, idp_ref_id FROM organizations WHERE idp_ref_id = ? AND portal_id = ?"),
		idpRefID, p.portalID)
	if err != nil {
		return nil, fmt.Errorf("querying API Portal organizations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []organizationRow
	for rows.Next() {
		var row organizationRow
		if err := rows.Scan(&row.uuid, &row.handle, &row.displayName, &row.idpRefID); err != nil {
			return nil, err
		}
		if row.idpRefID == idpRefID {
			out = append(out, row)
		}
	}
	return out, rows.Err()
}

// withOrganizations runs fn with the portal's organizations for an IDP reference.
func (s *Steps) withOrganizations(ctx context.Context, portal, idpRefID string, fn func([]organizationRow) error) error {
	idpRefID, err := stepscommon.Expand(ctx, idpRefID)
	if err != nil {
		return err
	}
	database, err := s.portalDB(ctx, portal)
	if err != nil {
		return err
	}
	defer func() { _ = database.close() }()
	rows, err := database.organizations(ctx, idpRefID)
	if err != nil {
		return err
	}
	return fn(rows)
}

func (s *Steps) assertOrganizationCount(ctx context.Context, portal string, want int, idpRefID string) error {
	return s.withOrganizations(ctx, portal, idpRefID, func(rows []organizationRow) error {
		if len(rows) != want {
			return fmt.Errorf("API Portal %s has %d organizations with IDP reference %q, want %d", portal, len(rows), idpRefID, want)
		}
		return nil
	})
}

func singleOrganization(portal, idpRefID string, rows []organizationRow) (organizationRow, error) {
	if len(rows) != 1 {
		return organizationRow{}, fmt.Errorf("API Portal %s has %d organizations with IDP reference %q, want 1", portal, len(rows), idpRefID)
	}
	return rows[0], nil
}

func (s *Steps) assertOrganizationNames(ctx context.Context, portal, idpRefID, handle, displayName string) error {
	handle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	displayName, err = stepscommon.Expand(ctx, displayName)
	if err != nil {
		return err
	}
	return s.withOrganizations(ctx, portal, idpRefID, func(rows []organizationRow) error {
		row, err := singleOrganization(portal, idpRefID, rows)
		if err != nil {
			return err
		}
		if row.handle != handle || row.displayName != displayName {
			return fmt.Errorf("API Portal organization is %q / %q, want %q / %q", row.handle, row.displayName, handle, displayName)
		}
		return nil
	})
}

func (s *Steps) assertOrganizationHandlePattern(ctx context.Context, portal, idpRefID, pattern string) error {
	pattern, err := stepscommon.Expand(ctx, pattern)
	if err != nil {
		return err
	}
	matcher, err := regexp.Compile("^(?:" + pattern + ")$")
	if err != nil {
		return fmt.Errorf("API Portal handle pattern %q: %w", pattern, err)
	}
	return s.withOrganizations(ctx, portal, idpRefID, func(rows []organizationRow) error {
		row, err := singleOrganization(portal, idpRefID, rows)
		if err != nil {
			return err
		}
		if !matcher.MatchString(row.handle) {
			return fmt.Errorf("API Portal organization handle %q does not match %q", row.handle, pattern)
		}
		return nil
	})
}

func (s *Steps) storeOrganizationID(ctx context.Context, portal, idpRefID, storeAs string) error {
	return s.withOrganizations(ctx, portal, idpRefID, func(rows []organizationRow) error {
		row, err := singleOrganization(portal, idpRefID, rows)
		if err != nil {
			return err
		}
		return tcontext.Set(ctx, storeAs, row.uuid)
	})
}

// seededRows identifies the rows seedUnownedPortalRows wrote.
type seededRows struct {
	Portal          string `json:"portal"`
	PortalID        string `json:"portalId"`
	Event           string `json:"event"`
	PendingDelivery string `json:"pendingDelivery"`
	StaleDelivery   string `json:"staleDelivery"`
}

// seedUnownedPortalRows writes the rows another portal on the same database would have, under
// a portal_id no running portal serves: an organization whose handle, display name and
// idp_ref_id are all idpRefID, a pending event, a pending delivery, and a delivery stranded
// in flight past the stale threshold. Nothing may claim or resolve to them, so any change, and
// any credential that resolves to that organization, is a portal reaching across portals.
func (s *Steps) seedUnownedPortalRows(ctx context.Context, idpRefID, portal, storeAs string) error {
	idpRefID, err := stepscommon.Expand(ctx, idpRefID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(idpRefID) == "" {
		return errors.New("the seeded API Portal organization needs an IDP reference")
	}
	name, err := unique.Unique(ctx, "unowned-portal")
	if err != nil {
		return err
	}
	name = strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	ids := make([]string, 5)
	for i := range ids {
		if ids[i], err = unique.Unique(ctx, "unowned-row"); err != nil {
			return err
		}
	}
	rows := seededRows{Portal: portal, PortalID: name, Event: ids[1], PendingDelivery: ids[2], StaleDelivery: ids[3]}
	database, err := s.portalDB(ctx, portal)
	if err != nil {
		return err
	}
	defer func() { _ = database.close() }()

	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	kind := cleanup.Kind{Name: "api-portal-seeded-rows/" + name, Order: seededRowsOrder}
	deleter := func(ctx context.Context, _ cleanup.Resource) error {
		cleanupDB, err := s.portalDB(ctx, portal)
		if err != nil {
			return err
		}
		defer func() { _ = cleanupDB.close() }()
		for _, table := range []string{"event_deliveries", "events", "organizations"} {
			if _, err := cleanupDB.db.ExecContext(ctx, cleanupDB.bind("DELETE FROM "+table+" WHERE portal_id = ?"), name); err != nil {
				return fmt.Errorf("deleting seeded API Portal %s rows: %w", table, err)
			}
		}
		return nil
	}
	if err := registry.RegisterDeleter(kind, deleter); err != nil {
		return err
	}
	if err := registry.RegisterScoped(cleanup.ScenarioScope, cleanup.Resource{
		Kind: kind, ID: name, Actor: "database", Description: "rows under an unowned portal_id",
	}); err != nil {
		return err
	}

	statements := []struct {
		query string
		args  []any
	}{
		{"INSERT INTO organizations (uuid, portal_id, display_name, handle, idp_ref_id, configuration, created_by, updated_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			[]any{ids[0], name, idpRefID, idpRefID, idpRefID, "{}", "it", "it"}},
		{"INSERT INTO events (uuid, type, org_uuid, portal_id, aggregate_type, aggregate_uuid, payload, occurred_at, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			[]any{ids[1], "apikey.generated", ids[0], name, "api_key", ids[4], "{}", time.Now().UTC(), "PENDING"}},
		{"INSERT INTO event_deliveries (uuid, event_uuid, portal_id, subscriber_id, target_url, status) VALUES (?, ?, ?, ?, ?, ?)",
			[]any{ids[2], ids[1], name, "unowned-pending", "http://testbench:3012/" + name + "/webhook", "PENDING"}},
		{"INSERT INTO event_deliveries (uuid, event_uuid, portal_id, subscriber_id, target_url, status, last_attempt_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			[]any{ids[3], ids[1], name, "unowned-stale", "http://testbench:3012/" + name + "/webhook", "IN_FLIGHT", time.Now().UTC().Add(-10 * time.Minute)}},
	}
	for _, statement := range statements {
		if _, err := database.db.ExecContext(ctx, database.bind(statement.query), statement.args...); err != nil {
			return fmt.Errorf("seeding API Portal rows: %w", err)
		}
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, storeAs, string(encoded))
}

// seededStatuses returns the seeded event's and deliveries' statuses, keyed by row id.
func (s *Steps) seededStatuses(ctx context.Context, rows seededRows) (map[string]string, error) {
	database, err := s.portalDB(ctx, rows.Portal)
	if err != nil {
		return nil, err
	}
	defer func() { _ = database.close() }()
	statuses := map[string]string{}
	for _, table := range []string{"events", "event_deliveries"} {
		result, err := database.db.QueryContext(ctx, database.bind("SELECT uuid, status FROM "+table+" WHERE portal_id = ?"), rows.PortalID)
		if err != nil {
			return nil, retry.Transient(err)
		}
		for result.Next() {
			var id, status string
			if err := result.Scan(&id, &status); err != nil {
				_ = result.Close()
				return nil, err
			}
			statuses[id] = status
		}
		if err := errors.Join(result.Err(), result.Close()); err != nil {
			return nil, err
		}
	}
	return statuses, nil
}

// assertSeededRowsUntouched checks, over a window longer than several dispatcher and delivery
// worker cycles, that the seeded rows keep the statuses they were written with.
func (s *Steps) assertSeededRowsUntouched(ctx context.Context, storeAs string) error {
	raw, err := tcontext.ResolveString(ctx, storeAs)
	if err != nil {
		return err
	}
	var rows seededRows
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return fmt.Errorf("reading seeded API Portal rows %q: %w", storeAs, err)
	}
	want := map[string]string{rows.Event: "PENDING", rows.PendingDelivery: "PENDING", rows.StaleDelivery: "IN_FLIGHT"}
	_, err = retry.Never(ctx, retry.Options{Interval: time.Second}, 5*time.Second,
		func(ctx context.Context) (map[string]string, error) { return s.seededStatuses(ctx, rows) },
		func(got map[string]string) bool {
			if len(got) != len(want) {
				return true
			}
			for id, status := range want {
				if got[id] != status {
					return true
				}
			}
			return false
		})
	if err != nil {
		statuses, _ := s.seededStatuses(ctx, rows)
		return fmt.Errorf("a portal changed rows under a portal_id it does not serve (now %v, want %v): %w", statuses, want, err)
	}
	return nil
}

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

// Package apiportal holds steps that create real subscription, application, and API-key
// artifacts through api-portal's own REST API — the product that issues subscription tokens
// and API keys, which reach platform-api (and from there the gateway) as signed webhook
// events. See core/catalog/apiportal's own doc comment for why the portal, not platform-api,
// is the thing under test here.
//
// api-portal accepts the platform-api admin JWT directly as a bearer token: it verifies the
// RS256 signature against the shared keypair's public half (auth.local.public_key_path in
// portals/api-portal/configs/config.toml, wired to the same jwt_public.pem
// core/catalog/apiportal.portalCryptoFiles copies from the control plane's own key material)
// and takes the organization from the token's org_handle claim. Every call in this package
// reuses that same bearer token — no session cookie, no CSRF handshake.
package apiportal

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha3"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/tests/framework/core/actor"
	controlplane "github.com/wso2/api-platform/tests/framework/core/catalog/platformapi"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
	platformapisteps "github.com/wso2/api-platform/tests/framework/suites/it/steps/platformapi"
)

// orgHandle is the single organization every block shares with platform-api — see
// core/catalog/overlays/api-portal-storage.toml, which hardcodes both sides to "default"
// rather than linking them at runtime.
const orgHandle = "default"
const portalService = "api-portal"

// apiPrefix is api-portal's whole REST surface, mounted under the portal's own base path.
const apiPrefix = "/api-portal/api/v0.9"

// webhookSubscriberID is a fixed handle, not a generated one: the Background step that
// registers it runs once per SCENARIO (every scenario in the devportal-webhook runner shares
// one webhook target), so registration must be idempotent across repeats within a runner
// rather than accumulate a new subscriber per scenario.
const webhookSubscriberID = "platform-api"

var (
	portalAPIKeyKind                = cleanup.Kind{Name: "api-portal-api-key", Order: 10}
	portalSubscriptionKind          = cleanup.Kind{Name: "api-portal-subscription", Order: 20}
	portalApplicationKeyMappingKind = cleanup.Kind{Name: "api-portal-application-key-mapping", Order: 25}
	// An OAuth2 key is removed before the key manager it was issued on, which refuses deletion
	// while keys still reference it.
	portalOAuth2KeyKind   = cleanup.Kind{Name: "api-portal-oauth2-key", Order: 28}
	portalApplicationKind = cleanup.Kind{Name: "api-portal-application", Order: 30}
	portalAPIKind         = cleanup.Kind{Name: "api-portal-api", Order: 40}
	// MCP servers have their own kind: a kind keeps one deleter, and an MCP server is deleted
	// through /mcp-servers, which never resolves a REST API's handle.
	portalMCPServerKind = cleanup.Kind{Name: "api-portal-mcp-server", Order: 40}
	// An API workflow lives under its view, so it is removed before the view and needs its own
	// kind: the view's deleter must keep the /views path.
	portalWorkflowKind         = cleanup.Kind{Name: "api-portal-api-workflow", Order: 20}
	portalSubscriptionPlanKind = cleanup.Kind{Name: "api-portal-subscription-plan", Order: 50}
	portalMeterKind            = cleanup.Kind{Name: "api-portal-meter", Order: 55}
	portalWebhookKind          = cleanup.Kind{Name: "api-portal-webhook-subscriber", Order: 60}
	// The organization is never deleted; its settings are written back after everything the
	// scenario created under it is gone.
	portalOrganizationKind = cleanup.Kind{Name: "api-portal-organization-settings", Order: 90}
)

// Steps holds what every api-portal REST call needs.
type Steps struct {
	topo *runtime.Topology
	// client calls api-portal, verifying its certificate.
	client *httpx.Client
	// controlPlane calls platform-api, verifying its certificate.
	controlPlane *httpx.Client
	// idp calls the testbench identity provider, verifying its certificate.
	idp *httpx.Client
	// featureRoot is the suite root that resource templates resolve against.
	featureRoot string
}

// Register binds the steps that drive api-portal's own REST API — subscription plans,
// publishing an API, applications, subscriptions, and API keys. Portal requests go through a
// client that trusts the portal's certificate; the shared client every other suite request
// funnels through trusts only the control plane's, and serves the calls made to platform-api.
// It returns the function that removes a plan from the portal when the scenario ends, for the
// control-plane steps that publish plans to it. Resource templates resolve against featureRoot.
func Register(
	sc *godog.ScenarioContext, topo *runtime.Topology, controlPlane *httpx.Client, featureRoot string,
) func(ctx context.Context, handle string) error {
	s := &Steps{topo: topo, controlPlane: controlPlane, featureRoot: featureRoot}
	s.client = newPortalClient()
	idp, idpErr := newIdentityProviderClient()
	s.idp = idp
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		if idpErr != nil {
			return ctx, idpErr
		}
		_, ok := tcontext.SharedOf(ctx)
		if !ok {
			return ctx, fmt.Errorf("API Portal webhook partition requires shared context")
		}
		_, ok = tcontext.LocalOf(ctx)
		if !ok {
			return ctx, fmt.Errorf("API Portal webhook partition requires local context")
		}
		partition, err := unique.UniqueContext(ctx, "api-portal-testbench")
		if err != nil {
			return ctx, err
		}
		partition = strings.TrimPrefix(partition, "/")
		if err := tcontext.Set(ctx, "apiPortalRunnerPartition", partition); err != nil {
			return ctx, err
		}
		return ctx, nil
	})
	sc.Step(`^a REST API is created in the API Portal and stored as "([^"]*)"$`,
		s.createRESTAPIFixture)
	sc.Step(`^I send an unauthenticated API Portal "([^"]*)" request to "([^"]*)"$`,
		s.sendUnauthenticated)
	sc.Step(`^I send an unauthenticated API Portal "([^"]*)" request to "([^"]*)" using portal "([^"]*)"$`,
		s.sendUnauthenticatedAt)
	sc.Step(`^I send an unauthenticated API Portal public "([^"]*)" request to "([^"]*)"$`,
		s.sendPublic)
	sc.Step(`^I send an unauthenticated API Portal "([^"]*)" request to the root path "([^"]*)"$`,
		s.sendRootPath)
	sc.Step(`^I submit an API Portal file login for actor "([^"]*)"$`, s.submitFileLogin)
	sc.Step(`^I submit an API Portal file login for actor "([^"]*)" to organization "([^"]*)" using portal "([^"]*)"$`,
		s.submitFileLoginAt)
	sc.Step(`^I submit an API Portal file login for actor "([^"]*)" with an invalid password to organization "([^"]*)" using portal "([^"]*)"$`,
		s.submitFileLoginInvalidAt)
	sc.Step(`^I submit an API Portal file login for actor "([^"]*)" with an invalid password$`, s.submitFileLoginInvalidPassword)
	sc.Step(`^I submit an API Portal file login for a nonexistent user$`, s.submitFileLoginNonexistentUser)
	sc.Step(`^I submit an API Portal file login with a missing password$`, s.submitFileLoginMissingPassword)
	sc.Step(`^I send an API Portal session request to "([^"]*)"$`, s.sendSessionRequest)
	sc.Step(`^I send an API Portal session request to "([^"]*)" using portal "([^"]*)"$`,
		s.sendSessionRequestAt)
	sc.Step(`^the API Portal response should set a cookie named "([^"]*)"$`, s.responseSetsCookie)
	sc.Step(`^I verify API Portal view "([^"]*)" returns not found when AI discovery is disabled as "([^"]*)"$`,
		s.verifyAIDiscoveryDisabled)
	sc.Step(`^I store the API Portal response body as "([^"]*)"$`, s.storeResponseBody)
	sc.Step(`^the API Portal response body should equal stored "([^"]*)"$`, s.responseBodyEqualsStored)
	sc.Step(`^I store the API Portal response header "([^"]*)" as "([^"]*)"$`, s.storeResponseHeader)
	sc.Step(`^the API Portal response header "([^"]*)" should equal stored "([^"]*)"$`, s.responseHeaderEqualsStored)
	sc.Step(`^the API Portal IT role mappings should match except for role "([^"]*)"$`, s.assertITRoleMappingsMatch)
	sc.Step(`^the API Portal IT role "([^"]*)" should be read-only on the portal side$`, s.assertITRoleIsReadOnly)
	sc.Step(`^I wait for an API Portal webhook event "([^"]*)"$`, s.waitForWebhookEvent)
	sc.Step(`^I wait for an API Portal webhook event "([^"]*)" containing "([^"]*)"$`, s.waitForWebhookEventContaining)
	sc.Step(`^I wait for API Portal webhook event "([^"]*)" to reach status "([^"]*)"$`,
		s.waitForWebhookEventStatus)
	sc.Step(`^the API Portal delivery for subscriber "([^"]*)" and event "([^"]*)" should be "([^"]*)"$`,
		s.assertWebhookDeliveryStatus)
	sc.Step(`^the API Portal webhook delivery "([^"]*)" containing "([^"]*)" should have a valid signature using secret "([^"]*)"$`,
		s.assertWebhookSignature)
	sc.Step(`^the API Portal webhook deliveries containing "([^"]*)" should be (\d+), each carrying its event headers and envelope$`,
		s.assertWebhookDeliveryEnvelopes)
	sc.Step(`^the API Portal webhook encrypted field "([^"]*)" should decrypt to "([^"]*)" using secret "([^"]*)"$`,
		s.assertWebhookEncryptedField)
	sc.Step(`^I verify no API Portal webhook event "([^"]*)" is delivered$`, s.verifyNoWebhookEvent)
	sc.Step(`^I verify no API Portal webhook event "([^"]*)" containing "([^"]*)" is delivered$`,
		s.verifyNoWebhookEventContaining)
	sc.Step(`^I reset API Portal webhook events$`, s.resetWebhookEvents)
	sc.Step(`^I send an authenticated API Portal "([^"]*)" request to "([^"]*)" as "([^"]*)"$`,
		s.sendAuthenticated)
	sc.Step(`^I send an authenticated API Portal "([^"]*)" request to "([^"]*)" as "([^"]*)" until status (\d+)$`,
		s.sendAuthenticatedUntilStatus)
	sc.Step(`^I send an authenticated API Portal "([^"]*)" request to "([^"]*)" as "([^"]*)" until the response body contains "([^"]*)"$`,
		s.sendAuthenticatedUntilBodyContains)
	sc.Step(`^I send an authenticated API Portal "([^"]*)" request to "([^"]*)" as "([^"]*)" until the top-level JSON response array field "([^"]*)" has (\d+) items?$`,
		s.sendAuthenticatedUntilJSONArrayLength)
	sc.Step(`^I send an authenticated API Portal "([^"]*)" request to "([^"]*)" as "([^"]*)" with header "([^"]*)" set to "([^"]*)"$`,
		s.sendAuthenticatedWithHeader)
	sc.Step(`^I send an authenticated API Portal "([^"]*)" request to "([^"]*)" as "([^"]*)" with JSON body:$`,
		s.sendAuthenticatedWithBody)
	sc.Step(`^I send an authenticated API Portal "([^"]*)" request to "([^"]*)" as "([^"]*)" with JSON values:$`,
		s.sendAuthenticatedWithValues)
	sc.Step(`^I send an authenticated API Portal "([^"]*)" multipart request to "([^"]*)" as "([^"]*)" with metadata only:$`,
		s.sendAuthenticatedMultipartMetadataOnly)
	sc.Step(`^I send an authenticated API Portal "([^"]*)" multipart request to "([^"]*)" as "([^"]*)" with metadata and definition "([^"]*)":$`,
		s.sendAuthenticatedMultipartWithDefinition)
	sc.Step(`^I send an authenticated API Portal "([^"]*)" artifact upload "([^"]*)" request to "([^"]*)" as "([^"]*)"$`,
		s.sendArtifactUpload)
	sc.Step(`^I send an authenticated API Portal "([^"]*)" multipart request to "([^"]*)" as "([^"]*)" with file field "([^"]*)" from "([^"]*)" with values:$`,
		s.sendAuthenticatedMultipartTemplate)
	sc.Step(`^the API Portal organization "([^"]*)" is restored to its current settings after the scenario$`,
		s.restoreOrganizationAfterScenario)
	sc.Step(`^a unique API Portal multipart resource is created at "([^"]*)" as "([^"]*)" with metadata and definition "([^"]*)" stored as "([^"]*)":$`,
		s.createMultipartResource)
	sc.Step(`^a unique API Portal MCP server with label "([^"]*)" is created and stored as "([^"]*)"$`,
		s.createMCPServerWithLabel)
	sc.Step(`^a unique API Portal resource is created at "([^"]*)" as "([^"]*)" with body and stored as "([^"]*)":$`,
		s.createJSONResource)
	sc.Step(`^a unique API Portal resource is created at "([^"]*)" as "([^"]*)" with values and stored as "([^"]*)":$`,
		s.createJSONResourceWithValues)
	sc.Step(`^I register API Portal resource "([^"]*)" at "([^"]*)" for cleanup$`,
		s.registerResourceForCleanup)
	sc.Step(`^I register API Portal application key mapping "([^"]*)" for application "([^"]*)" for cleanup$`,
		s.registerApplicationKeyMappingForCleanup)
	sc.Step(`^I register API Portal API key "([^"]*)" of API "([^"]*)" owned by "([^"]*)" for cleanup$`,
		s.registerAPIKeyForCleanup)
	sc.Step(`^I upload default API Portal content for API "([^"]*)" as "([^"]*)"$`,
		s.uploadAPIContent)
	sc.Step(`^I upload API Portal content "([^"]*)" for API "([^"]*)" as "([^"]*)"$`,
		s.uploadAPIContentVariant)
	sc.Step(`^I (POST|PUT) API Portal content "([^"]*)" for MCP server "([^"]*)" as "([^"]*)"$`,
		s.uploadMCPContentMethodVariant)
	sc.Step(`^I send an API Portal session "([^"]*)" request to "([^"]*)" as "([^"]*)" with JSON values:$`,
		s.sendCSRFSessionRequestWithValues)
	sc.Step(`^I send an API Portal session "([^"]*)" request to "([^"]*)" as "([^"]*)"$`,
		s.sendCSRFSessionRequestWithoutBody)
	sc.Step(`^I (POST|PUT) API Portal content "([^"]*)" for API "([^"]*)" as "([^"]*)"$`,
		s.uploadAPIContentMethodVariant)
	sc.Step(`^the API Portal response body should equal hex "([^"]*)"$`,
		s.responseBodyEqualsHex)
	sc.Step(`^I upload API Portal theme "([^"]*)" for view "([^"]*)" as "([^"]*)"$`,
		s.uploadTheme)
	sc.Step(`^a developer subscription for API "([^"]*)" using plan "([^"]*)" is created and stored as "([^"]*)"$`,
		s.createSubscription)
	sc.Step(`^a publisher API key for API "([^"]*)" is generated and stored as "([^"]*)"$`,
		s.createAPIKey)
	sc.Step(`^a "([^"]*)" API key for API "([^"]*)" is generated and stored as "([^"]*)"$`,
		s.createAPIKeyAs)
	sc.Step(`^a publisher API key for MCP server "([^"]*)" is generated and stored as "([^"]*)"$`,
		s.createMCPKey)
	sc.Step(`^the subscription plan "([^"]*)" is synced to the API Portal$`, s.syncSubscriptionPlan)
	sc.Step(`^the "([^"]*)" API is published to the API Portal offering plan "([^"]*)"$`, s.publishAPI)
	registerPublication(sc, s)
	sc.Step(`^an application subscribed to API "([^"]*)" plan "([^"]*)" is created in the API Portal, the token is stored as "([^"]*)" and the subscription is stored as "([^"]*)"$`,
		s.subscribeToAPIAndStore)
	sc.Step(`^an API key for API "([^"]*)" is generated in the API Portal, the value is stored as "([^"]*)" and the handle is stored as "([^"]*)"$`,
		s.generateAPIKeyAndStore)
	sc.Step(`^a Portal API key for API "([^"]*)" is generated, the value is stored as "([^"]*)" and the handle is stored as "([^"]*)"$`,
		s.generatePortalAPIKeyAndStore)
	sc.Step(`^a webhook subscriber targeting the control plane is registered in the API Portal(?: for events "([^"]*)")?$`,
		s.registerWebhookSubscriber)

	// Credential-lifecycle steps: key expiry/revoke/regenerate and subscription
	// plan-switch/token-regenerate/pause/resume/remove.
	sc.Step(`^the API key "([^"]*)" for API "([^"]*)" is regenerated with expiry "([^"]*)" in the API Portal and stored as "([^"]*)"$`,
		s.regenerateKeyExpiryAndStore)
	sc.Step(`^the API key "([^"]*)" for API "([^"]*)" is revoked in the API Portal$`, s.revokeAPIKey)
	sc.Step(`^the subscription "([^"]*)" is switched to plan "([^"]*)" in the API Portal$`, s.switchSubscriptionPlan)
	sc.Step(`^the subscription "([^"]*)" token is regenerated in the API Portal, the previous token is stored as "([^"]*)" and the new token is stored as "([^"]*)"$`,
		s.regenerateSubscriptionToken)
	sc.Step(`^the subscription "([^"]*)" status is set to "([^"]*)" in the API Portal$`, s.setSubscriptionStatus)
	sc.Step(`^the subscription "([^"]*)" is removed in the API Portal$`, s.removeSubscription)
	s.registerAPITemplateSteps(sc)
	s.registerMultiOrganizationSteps(sc)
	s.registerBillingSteps(sc)
	s.registerSecuritySteps(sc)
	s.registerControlPlaneEventSteps(sc)
	return s.trackPublishedPlan
}

const portalRESTDefinition = `{
  "openapi": "3.0.3",
  "info": {"title": "Fixture API", "version": "1.0.0"},
  "paths": {"/ping": {"get": {"responses": {"200": {"description": "ok"}}}}}
}`

const portalMCPDefinition = `- type: TOOL
  name: ping
  description: Health check tool.
  inputSchema:
    type: object
    properties: {}
`

func (s *Steps) createRESTAPIFixture(ctx context.Context, storeAs string) error {
	id, err := unique.Unique(ctx, "portal-rest-api")
	if err != nil {
		return err
	}
	base, token, err := s.authedAs(ctx, "publisher")
	if err != nil {
		return err
	}
	metadata := fmt.Sprintf(`{
  "id": %q,
  "name": %q,
  "version": "v1",
  "type": "REST",
  "status": "PUBLISHED",
  "subscriptionPlans": [{"id": "Gold"}, {"id": "Silver"}],
  "endPoints": {
    "productionURL": "https://backend.example.invalid/%s",
    "sandboxURL": "https://sandbox.example.invalid/%s"
  }
}`, id, id, id, id)
	var created struct {
		ID string `json:"id"`
	}
	if err := s.postJSONMultipart(ctx, base, token, "/apis", metadata, portalRESTDefinition, &created); err != nil {
		return err
	}
	if created.ID == "" {
		created.ID = id
	}
	if err := s.registerPortalResource(ctx, portalAPIKind, created.ID, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: "/apis"}); err != nil {
		return err
	}
	return tcontext.Set(ctx, storeAs, created.ID)
}

func (s *Steps) sendUnauthenticated(ctx context.Context, method, path string) error {
	return s.sendPortal(ctx, method, path, "", nil)
}

func (s *Steps) sendUnauthenticatedAt(ctx context.Context, method, path, portal string) error {
	base, err := s.portalBaseNamed(portal)
	if err != nil {
		return err
	}
	path, err = stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	response, err := s.client.Do(ctx, httpx.Request{
		Method: strings.ToUpper(method), URL: s.portalRequestURL(base, path),
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal %s %s: %w", method, path, err)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

func (s *Steps) sendPublic(ctx context.Context, method, path string) error {
	base, err := s.portalBase()
	if err != nil {
		return err
	}
	path, err = stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	response, err := s.client.Do(ctx, httpx.Request{
		Method: strings.ToUpper(method), URL: s.portalRequestURL(base, path),
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal public %s %s: %w", method, path, err)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

// sendRootPath sends an unauthenticated request to the portal host exactly as written, with no
// Management API prefix, so a scenario can also address a path outside the /api-portal mount.
func (s *Steps) sendRootPath(ctx context.Context, method, path string) error {
	base, err := s.portalBase()
	if err != nil {
		return err
	}
	path, err = stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("API Portal root path %q must start with /", path)
	}
	response, err := s.client.Do(ctx, httpx.Request{
		Method: strings.ToUpper(method), URL: portalPublicURL(base, path),
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal root path %s %s: %w", method, path, err)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

func (s *Steps) submitFileLogin(ctx context.Context, role string) error {
	credentials, err := portalActor(role)
	if err != nil {
		return err
	}
	return s.submitFileLoginValues(ctx, credentials.Username, credentials.Password)
}

func (s *Steps) submitFileLoginAt(ctx context.Context, role, organization, portal string) error {
	credentials, err := portalActor(role)
	if err != nil {
		return err
	}
	return s.submitFileLoginValuesAt(ctx, credentials.Username, credentials.Password, organization, portal)
}

func (s *Steps) submitFileLoginInvalidAt(ctx context.Context, role, organization, portal string) error {
	credentials, err := portalActor(role)
	if err != nil {
		return err
	}
	return s.submitFileLoginValuesAt(ctx, credentials.Username, "invalid-password", organization, portal)
}

func (s *Steps) submitFileLoginInvalidPassword(ctx context.Context, role string) error {
	credentials, err := portalActor(role)
	if err != nil {
		return err
	}
	return s.submitFileLoginValues(ctx, credentials.Username, "invalid-password")
}

func (s *Steps) submitFileLoginNonexistentUser(ctx context.Context) error {
	return s.submitFileLoginValues(ctx, "no-such-user", "invalid-password")
}

func (s *Steps) submitFileLoginMissingPassword(ctx context.Context) error {
	return s.submitFileLoginValues(ctx, actor.Administrator().Username, "")
}

func (s *Steps) submitFileLoginValues(ctx context.Context, username, password string) error {
	return s.submitFileLoginValuesAt(ctx, username, password, orgHandle, portalService)
}

func (s *Steps) submitFileLoginValuesAt(ctx context.Context, username, password, organization, portal string) error {
	base, err := s.portalBaseNamed(portal)
	if err != nil {
		return err
	}
	organization, err = stepscommon.Expand(ctx, organization)
	if err != nil {
		return err
	}
	form := url.Values{"username": {username}}
	if password != "" {
		form.Set("password", password)
	}
	response, err := s.client.Do(ctx, httpx.Request{
		Method:      http.MethodPost,
		URL:         base + "/api-portal/" + organization + "/views/default/login",
		Body:        []byte(form.Encode()),
		ContentType: "application/x-www-form-urlencoded",
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal file login: %w", err)
	}
	if cookies := response.Headers.Values("Set-Cookie"); len(cookies) > 0 {
		pairs := make([]string, 0, len(cookies))
		for _, raw := range cookies {
			if pair := strings.TrimSpace(strings.SplitN(raw, ";", 2)[0]); pair != "" {
				pairs = append(pairs, pair)
			}
		}
		if len(pairs) > 0 {
			if err := tcontext.Set(ctx, "apiPortalSessionCookie", strings.Join(pairs, "; ")); err != nil {
				return err
			}
		}
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

func (s *Steps) storeResponseHeader(ctx context.Context, name, key string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, key, response.Headers.Get(name))
}

func (s *Steps) responseHeaderEqualsStored(ctx context.Context, name, key string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	want, err := tcontext.ResolveString(ctx, key)
	if err != nil {
		return err
	}
	if got := response.Headers.Get(name); got != want {
		return fmt.Errorf("API Portal response header %q did not match stored value", name)
	}
	return nil
}

func (s *Steps) assertITRoleMappingsMatch(_ context.Context, exception string) error {
	platform, portal, err := loadITRoleMappings()
	if err != nil {
		return err
	}
	if exception == "" {
		return fmt.Errorf("API Portal IT role mapping exception is required")
	}
	for role, scopes := range platform {
		if role == exception {
			continue
		}
		portalScopes, ok := portal[role]
		if !ok {
			return fmt.Errorf("API Portal IT role mapping is missing role %q", role)
		}
		if !equalStrings(scopes, portalScopes) {
			return fmt.Errorf("API Portal IT role mapping differs for role %q", role)
		}
	}
	for role := range portal {
		if role != exception {
			if _, ok := platform[role]; !ok {
				return fmt.Errorf("Platform API IT role mapping is missing role %q", role)
			}
		}
	}
	if _, ok := platform[exception]; !ok {
		return fmt.Errorf("Platform API IT role mapping is missing exception role %q", exception)
	}
	if _, ok := portal[exception]; !ok {
		return fmt.Errorf("API Portal IT role mapping is missing exception role %q", exception)
	}
	return nil
}

func (s *Steps) assertITRoleIsReadOnly(_ context.Context, role string) error {
	platform, portal, err := loadITRoleMappings()
	if err != nil {
		return err
	}
	platformScopes, ok := platform[role]
	if !ok {
		return fmt.Errorf("Platform API IT role mapping is missing role %q", role)
	}
	portalScopes, ok := portal[role]
	if !ok {
		return fmt.Errorf("API Portal IT role mapping is missing role %q", role)
	}
	if !containsString(platformScopes, "dp:application:create") {
		return fmt.Errorf("Platform API IT role %q does not grant application creation", role)
	}
	if containsString(portalScopes, "dp:application:create") {
		return fmt.Errorf("API Portal IT role %q grants application creation", role)
	}
	for _, scope := range portalScopes {
		if !strings.HasSuffix(scope, ":read") {
			return fmt.Errorf("API Portal IT role %q has non-read scope", role)
		}
	}
	return nil
}

func loadITRoleMappings() (map[string][]string, map[string][]string, error) {
	root, ok := shared.RepoRootFromCallerFile()
	if !ok {
		return nil, nil, fmt.Errorf("locate repository root for API Portal IT role mappings")
	}
	platform, err := loadRoleMapping(filepath.Join(root, "portals/api-portal/it/configs/roles-platform-api-it.yaml"))
	if err != nil {
		return nil, nil, fmt.Errorf("load Platform API IT role mapping: %w", err)
	}
	portal, err := loadRoleMapping(filepath.Join(root, "portals/api-portal/it/configs/portal-roles-it.yaml"))
	if err != nil {
		return nil, nil, fmt.Errorf("load API Portal IT role mapping: %w", err)
	}
	return platform, portal, nil
}

func loadRoleMapping(path string) (map[string][]string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document struct {
		Roles []struct {
			Name   string   `yaml:"name"`
			Scopes []string `yaml:"scopes"`
		} `yaml:"roles"`
	}
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return nil, err
	}
	roles := make(map[string][]string, len(document.Roles))
	for _, role := range document.Roles {
		if role.Name == "" || len(role.Scopes) == 0 {
			return nil, fmt.Errorf("invalid role mapping entry")
		}
		if _, exists := roles[role.Name]; exists {
			return nil, fmt.Errorf("duplicate role %q", role.Name)
		}
		scopes := append([]string(nil), role.Scopes...)
		sort.Strings(scopes)
		roles[role.Name] = scopes
	}
	if len(roles) == 0 {
		return nil, fmt.Errorf("role mapping is empty")
	}
	return roles, nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (s *Steps) responseSetsCookie(ctx context.Context, name string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	for _, raw := range response.Headers.Values("Set-Cookie") {
		if cookieName := strings.TrimSpace(strings.SplitN(raw, "=", 2)[0]); cookieName == name {
			return nil
		}
	}
	return fmt.Errorf("API Portal response did not set cookie %q", name)
}

func (s *Steps) storeResponseBody(ctx context.Context, key string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, key, string(response.Body))
}

func (s *Steps) responseBodyEqualsStored(ctx context.Context, key string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	stored, err := tcontext.ResolveString(ctx, key)
	if err != nil {
		return err
	}
	if string(response.Body) != stored {
		return fmt.Errorf("API Portal response body differs from stored response")
	}
	return nil
}

func (s *Steps) waitForWebhookEvent(ctx context.Context, eventType string) error {
	return s.waitForWebhookEventMatch(ctx, eventType, "")
}

// webhookPartition returns the scenario-scoped testbench partition used by API Portal
// fixtures. Keeping this separate from the framework's block partition preserves the
// existing isolation contract for every other testbench service.
func (s *Steps) webhookPartition(ctx context.Context) (string, error) {
	return tcontext.ResolveString(ctx, "apiPortalRunnerPartition")
}

func (s *Steps) waitForWebhookEventContaining(ctx context.Context, eventType, marker string) error {
	return s.waitForWebhookEventMatch(ctx, eventType, marker)
}

func (s *Steps) assertWebhookDeliveryStatus(ctx context.Context, subscriberID, eventType, wantStatus string) error {
	subscriberID, err := stepscommon.Expand(ctx, subscriberID)
	if err != nil {
		return err
	}
	eventType, err = stepscommon.Expand(ctx, eventType)
	if err != nil {
		return err
	}
	wantStatus, err = stepscommon.Expand(ctx, wantStatus)
	if err != nil {
		return err
	}
	if subscriberID == "" || eventType == "" || wantStatus == "" {
		return fmt.Errorf("API Portal webhook delivery assertion requires subscriber, event, and status")
	}
	result, err := retry.Until(ctx, retry.Options{Interval: time.Second}, func(ctx context.Context) (bool, error) {
		base, token, err := s.authedAs(ctx, "admin")
		if err != nil {
			return false, err
		}
		response, err := s.client.Do(ctx, httpx.Request{
			Method:  http.MethodGet,
			URL:     base + apiPrefix + "/webhook-subscribers/" + url.PathEscape(subscriberID) + "/deliveries",
			Headers: map[string]string{"Authorization": "Bearer " + token},
		}, 0, 0)
		if err != nil {
			return false, retry.Transient(err)
		}
		if !response.Succeeded() {
			return false, fmt.Errorf("API Portal webhook delivery lookup returned status %d", response.StatusCode)
		}
		var document struct {
			List []struct {
				EventType string `json:"eventType"`
				Status    string `json:"status"`
			} `json:"list"`
		}
		if err := json.Unmarshal(response.Body, &document); err != nil {
			return false, fmt.Errorf("decoding API Portal webhook deliveries: %w", err)
		}
		for _, delivery := range document.List {
			if delivery.EventType == eventType && delivery.Status == wantStatus {
				return true, nil
			}
		}
		return false, nil
	}, func(found bool) bool { return found })
	if err != nil {
		return fmt.Errorf("waiting for API Portal webhook delivery %q: %w", eventType, err)
	}
	if !result {
		return fmt.Errorf("API Portal webhook delivery %q did not reach the expected status", eventType)
	}
	return nil
}

func (s *Steps) waitForWebhookEventMatch(ctx context.Context, eventType, marker string) error {
	eventType, err := stepscommon.Expand(ctx, eventType)
	if err != nil {
		return err
	}
	marker, err = stepscommon.Expand(ctx, marker)
	if err != nil {
		return err
	}
	block, err := s.webhookPartition(ctx)
	if err != nil {
		return err
	}
	base, err := s.topo.URL("testbench", "webhook")
	if err != nil {
		return err
	}
	endpoint := base + "/" + block + "/test/event?eventType=" + url.QueryEscape(eventType)
	if marker != "" {
		endpoint += "&contains=" + url.QueryEscape(marker)
	}
	type pollResult struct {
		response *httpx.Response
		found    bool
	}
	result, err := retry.Until(ctx, retry.Options{Interval: time.Second}, func(ctx context.Context) (pollResult, error) {
		response, err := s.client.Do(ctx, httpx.Request{Method: http.MethodGet, URL: endpoint}, 0, 0)
		if err != nil {
			return pollResult{}, retry.Transient(err)
		}
		return pollResult{response: response, found: response.Succeeded() && response.HasBody()}, nil
	}, func(result pollResult) bool { return result.found })
	if err != nil {
		return fmt.Errorf("waiting for API Portal webhook event %q: %w", eventType, err)
	}
	if !result.found {
		return fmt.Errorf("API Portal webhook event %q was not delivered before the timeout", eventType)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, result.response)
}

func (s *Steps) waitForWebhookEventStatus(ctx context.Context, eventID, wantStatus string) error {
	eventID, err := stepscommon.Expand(ctx, eventID)
	if err != nil {
		return err
	}
	wantStatus, err = stepscommon.Expand(ctx, wantStatus)
	if err != nil {
		return err
	}
	if eventID == "" || wantStatus == "" {
		return fmt.Errorf("API Portal webhook event status assertion requires event ID and status")
	}

	var observed string
	result, err := retry.Until(ctx, retry.Options{Interval: time.Second}, func(ctx context.Context) (bool, error) {
		base, token, err := s.authedAs(ctx, "admin")
		if err != nil {
			return false, err
		}
		response, err := s.client.Do(ctx, httpx.Request{
			Method:  http.MethodGet,
			URL:     base + apiPrefix + "/webhook-events/" + url.PathEscape(eventID),
			Headers: map[string]string{"Authorization": "Bearer " + token},
		}, 0, 0)
		if err != nil {
			return false, retry.Transient(err)
		}
		if !response.Succeeded() {
			return false, nil
		}
		var event struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(response.Body, &event); err != nil {
			return false, fmt.Errorf("decoding API Portal webhook event: %w", err)
		}
		observed = event.Status
		if event.Status != wantStatus && webhookEventStatusIsTerminal(event.Status) {
			return false, fmt.Errorf(
				"API Portal webhook event %q settled on status %q, which %q can no longer follow: a failed delivery is never retried",
				eventID, event.Status, wantStatus)
		}
		return event.Status == wantStatus, nil
	}, func(reached bool) bool { return reached })
	if err != nil {
		return fmt.Errorf("waiting for API Portal webhook event %q to reach %q (last observed %q): %w",
			eventID, wantStatus, observedOrUnknown(observed), err)
	}
	if !result {
		return fmt.Errorf("API Portal webhook event %q did not reach status %q before the timeout (last observed %q)",
			eventID, wantStatus, observedOrUnknown(observed))
	}
	return nil
}

// webhookEventStatusIsTerminal reports whether the portal can still move an event off status.
// eventDao.reconcile only ever settles on ALL_DELIVERED or FAILED, and markFailed records a
// single attempt with no retry scheduling, so neither is reachable from the other.
func webhookEventStatusIsTerminal(status string) bool {
	return status == "ALL_DELIVERED" || status == "FAILED"
}

func observedOrUnknown(status string) string {
	if status == "" {
		return "none - the event was never read"
	}
	return status
}

// recordedWebhookDelivery is one delivery as the testbench webhook sink recorded it.
type recordedWebhookDelivery struct {
	Headers map[string][]string `json:"headers"`
	Body    json.RawMessage     `json:"body"`
}

// assertWebhookDeliveryEnvelopes waits until the sink holds want deliveries whose body contains the
// marker, then checks each one's headers and envelope (checkWebhookDeliveryEnvelopes).
func (s *Steps) assertWebhookDeliveryEnvelopes(ctx context.Context, marker string, want int) error {
	marker, err := stepscommon.Expand(ctx, marker)
	if err != nil {
		return err
	}
	if strings.TrimSpace(marker) == "" {
		return fmt.Errorf("API Portal webhook delivery envelope assertion requires a marker")
	}
	if want <= 0 {
		return fmt.Errorf("API Portal webhook delivery envelope assertion requires a positive count")
	}
	block, err := s.webhookPartition(ctx)
	if err != nil {
		return err
	}
	base, err := s.topo.URL("testbench", "webhook")
	if err != nil {
		return err
	}
	endpoint := base + "/" + block + "/test/deliveries"
	matched, err := retry.Until(ctx, retry.Options{Interval: time.Second}, func(ctx context.Context) ([]recordedWebhookDelivery, error) {
		response, err := s.client.Do(ctx, httpx.Request{Method: http.MethodGet, URL: endpoint}, 0, 0)
		if err != nil {
			return nil, retry.Transient(err)
		}
		if !response.Succeeded() {
			return nil, fmt.Errorf("API Portal webhook delivery lookup returned status %d", response.StatusCode)
		}
		var deliveries []recordedWebhookDelivery
		if err := json.Unmarshal(response.Body, &deliveries); err != nil {
			return nil, fmt.Errorf("decode API Portal webhook deliveries: %w", err)
		}
		return deliveriesContaining(deliveries, marker), nil
	}, func(matched []recordedWebhookDelivery) bool { return len(matched) >= want })
	if err != nil {
		return fmt.Errorf("waiting for %d API Portal webhook deliveries containing %q: %w", want, marker, err)
	}
	if len(matched) != want {
		return fmt.Errorf("API Portal webhook deliveries containing %q: got %d, want %d", marker, len(matched), want)
	}
	return checkWebhookDeliveryEnvelopes(matched)
}

// deliveriesContaining returns the deliveries whose body contains the marker.
func deliveriesContaining(deliveries []recordedWebhookDelivery, marker string) []recordedWebhookDelivery {
	matched := make([]recordedWebhookDelivery, 0, len(deliveries))
	for _, delivery := range deliveries {
		if bytes.Contains(delivery.Body, []byte(marker)) {
			matched = append(matched, delivery)
		}
	}
	return matched
}

// checkWebhookDeliveryEnvelopes checks the contract every API Portal delivery keeps: a JSON body
// whose event_type and event_id the X-Api-Portal-Event and X-Api-Portal-Event-Id headers repeat, a
// delivery id of its own, an RFC 3339 occurred_at, the organization reference, the list of encrypted
// fields and the event data. Event ids are unique too, since each event reaches a subscriber once.
func checkWebhookDeliveryEnvelopes(deliveries []recordedWebhookDelivery) error {
	if len(deliveries) == 0 {
		return fmt.Errorf("no API Portal webhook delivery to check")
	}
	deliveryIDs := map[string]bool{}
	eventIDs := map[string]bool{}
	for i, delivery := range deliveries {
		var envelope struct {
			EventID    string `json:"event_id"`
			EventType  string `json:"event_type"`
			OccurredAt string `json:"occurred_at"`
			Org        struct {
				RefID string `json:"ref_id"`
			} `json:"org"`
			EncryptedFields json.RawMessage `json:"encrypted_fields"`
			Data            json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(delivery.Body, &envelope); err != nil {
			return fmt.Errorf("delivery %d: body is not an event envelope: %w", i+1, err)
		}
		if envelope.EventID == "" || envelope.EventType == "" {
			return fmt.Errorf("delivery %d: event_id and event_type are required", i+1)
		}
		label := fmt.Sprintf("delivery %d (%s)", i+1, envelope.EventType)
		if contentType := headerValue(delivery.Headers, "Content-Type"); !strings.HasPrefix(contentType, "application/json") {
			return fmt.Errorf("%s: Content-Type is %q, want application/json", label, contentType)
		}
		if got := headerValue(delivery.Headers, "X-Api-Portal-Event"); got != envelope.EventType {
			return fmt.Errorf("%s: X-Api-Portal-Event is %q, want the event_type %q", label, got, envelope.EventType)
		}
		if got := headerValue(delivery.Headers, "X-Api-Portal-Event-Id"); got != envelope.EventID {
			return fmt.Errorf("%s: X-Api-Portal-Event-Id is %q, want the event_id %q", label, got, envelope.EventID)
		}
		deliveryID := headerValue(delivery.Headers, "X-Api-Portal-Delivery-Id")
		if deliveryID == "" {
			return fmt.Errorf("%s: X-Api-Portal-Delivery-Id is missing", label)
		}
		if deliveryIDs[deliveryID] {
			return fmt.Errorf("%s: X-Api-Portal-Delivery-Id %q repeats another delivery's", label, deliveryID)
		}
		deliveryIDs[deliveryID] = true
		if eventIDs[envelope.EventID] {
			return fmt.Errorf("%s: event_id %q repeats another delivery's", label, envelope.EventID)
		}
		eventIDs[envelope.EventID] = true
		if _, err := time.Parse(time.RFC3339, envelope.OccurredAt); err != nil {
			return fmt.Errorf("%s: occurred_at %q is not RFC 3339", label, envelope.OccurredAt)
		}
		if strings.TrimSpace(envelope.Org.RefID) == "" {
			return fmt.Errorf("%s: org.ref_id is missing", label)
		}
		var encrypted []string
		if err := json.Unmarshal(envelope.EncryptedFields, &encrypted); err != nil || encrypted == nil {
			return fmt.Errorf("%s: encrypted_fields is not a list", label)
		}
		var data map[string]any
		if err := json.Unmarshal(envelope.Data, &data); err != nil || data == nil {
			return fmt.Errorf("%s: data is not an object", label)
		}
	}
	return nil
}

func (s *Steps) assertWebhookSignature(ctx context.Context, eventType, marker, secret string) error {
	eventType, err := stepscommon.Expand(ctx, eventType)
	if err != nil {
		return err
	}
	marker, err = stepscommon.Expand(ctx, marker)
	if err != nil {
		return err
	}
	secret, err = stepscommon.Expand(ctx, secret)
	if err != nil {
		return err
	}
	if secret == "" {
		return fmt.Errorf("API Portal webhook signature secret must not be empty")
	}
	block, err := s.webhookPartition(ctx)
	if err != nil {
		return err
	}
	base, err := s.topo.URL("testbench", "webhook")
	if err != nil {
		return err
	}
	endpoint := base + "/" + block + "/test/deliveries?eventType=" + url.QueryEscape(eventType)
	result, err := retry.Until(ctx, retry.Options{Interval: time.Second}, func(ctx context.Context) (bool, error) {
		response, err := s.client.Do(ctx, httpx.Request{Method: http.MethodGet, URL: endpoint}, 0, 0)
		if err != nil {
			return false, retry.Transient(err)
		}
		if !response.Succeeded() {
			return false, fmt.Errorf("API Portal webhook delivery lookup returned status %d", response.StatusCode)
		}
		var deliveries []struct {
			Headers map[string][]string `json:"headers"`
			Body    json.RawMessage     `json:"body"`
		}
		if err := json.Unmarshal(response.Body, &deliveries); err != nil {
			return false, fmt.Errorf("decode API Portal webhook deliveries: %w", err)
		}
		for _, delivery := range deliveries {
			if marker != "" && !bytes.Contains(delivery.Body, []byte(marker)) {
				continue
			}
			signature := headerValue(delivery.Headers, "X-Api-Portal-Signature")
			if signature != "" && validWebhookSignature(secret, delivery.Body, signature) {
				return true, nil
			}
		}
		return false, nil
	}, func(found bool) bool { return found })
	if err != nil {
		return fmt.Errorf("waiting for a valid API Portal webhook signature: %w", err)
	}
	if !result {
		return fmt.Errorf("API Portal webhook delivery did not have a valid signature")
	}
	return nil
}

const webhookFieldKeyInfo = "api-portal-webhook-field-encryption-v1"

func (s *Steps) assertWebhookEncryptedField(ctx context.Context, field, expected, secret string) error {
	field, err := stepscommon.Expand(ctx, field)
	if err != nil {
		return err
	}
	expected, err = stepscommon.Expand(ctx, expected)
	if err != nil {
		return err
	}
	secret, err = stepscommon.Expand(ctx, secret)
	if err != nil {
		return err
	}
	if field == "" || secret == "" {
		return fmt.Errorf("API Portal encrypted webhook assertion requires a field and secret")
	}
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := json.Unmarshal(response.Body, &document); err != nil {
		return fmt.Errorf("decoding API Portal webhook response: %w", err)
	}
	value, present := traverseWebhookJSON(document, field)
	if !present {
		return fmt.Errorf("API Portal webhook encrypted field %q is absent", field)
	}
	envelope, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("API Portal webhook field %q is not an encrypted envelope", field)
	}
	plaintext, err := decryptWebhookField(secret, envelope)
	if err != nil {
		return fmt.Errorf("decrypting API Portal webhook field %q: %w", field, err)
	}
	if subtle.ConstantTimeCompare([]byte(plaintext), []byte(expected)) != 1 {
		return fmt.Errorf("decrypted API Portal webhook field %q did not match the expected value", field)
	}
	return nil
}

func traverseWebhookJSON(document map[string]any, field string) (any, bool) {
	var current any = document
	for _, part := range strings.Split(field, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func decryptWebhookField(secret string, envelope map[string]any) (string, error) {
	get := func(name string) ([]byte, error) {
		value, ok := envelope[name].(string)
		if !ok || value == "" {
			return nil, fmt.Errorf("encrypted envelope is missing %s", name)
		}
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("encrypted envelope has invalid %s", name)
		}
		return decoded, nil
	}
	iv, err := get("iv")
	if err != nil {
		return "", err
	}
	tag, err := get("tag")
	if err != nil {
		return "", err
	}
	ciphertext, err := get("ciphertext")
	if err != nil {
		return "", err
	}
	if len(iv) != 12 || len(tag) != 16 {
		return "", fmt.Errorf("encrypted envelope has invalid dimensions")
	}
	key, err := hkdf.Key(sha3.New256, []byte(secret), nil, webhookFieldKeyInfo, 32)
	if err != nil {
		return "", fmt.Errorf("deriving encrypted field key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating encrypted field cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating encrypted field GCM: %w", err)
	}
	plaintext, err := gcm.Open(nil, iv, append(ciphertext, tag...), nil)
	if err != nil {
		return "", fmt.Errorf("opening encrypted field: %w", err)
	}
	return string(plaintext), nil
}

func headerValue(headers map[string][]string, want string) string {
	for name, values := range headers {
		if strings.EqualFold(name, want) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func validWebhookSignature(secret string, body json.RawMessage, header string) bool {
	parts := strings.Split(header, ",")
	if len(parts) != 2 {
		return false
	}
	values := make(map[string]string, 2)
	for _, part := range parts {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || key == "" || value == "" {
			return false
		}
		values[key] = value
	}
	timestamp := values["t"]
	if _, err := strconv.ParseInt(timestamp, 10, 64); err != nil {
		return false
	}
	want := values["v1"]
	if len(want) != sha256.Size*2 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(body)
	got := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(got), []byte(want))
}

func (s *Steps) verifyNoWebhookEvent(ctx context.Context, eventType string) error {
	return s.verifyNoWebhookEventMatch(ctx, eventType, "")
}

func (s *Steps) verifyNoWebhookEventContaining(ctx context.Context, eventType, marker string) error {
	return s.verifyNoWebhookEventMatch(ctx, eventType, marker)
}

func (s *Steps) verifyNoWebhookEventMatch(ctx context.Context, eventType, marker string) error {
	eventType, err := stepscommon.Expand(ctx, eventType)
	if err != nil {
		return err
	}
	marker, err = stepscommon.Expand(ctx, marker)
	if err != nil {
		return err
	}
	block, err := s.webhookPartition(ctx)
	if err != nil {
		return err
	}
	base, err := s.topo.URL("testbench", "webhook")
	if err != nil {
		return err
	}
	endpoint := base + "/" + block + "/test/event?eventType=" + url.QueryEscape(eventType)
	if marker != "" {
		endpoint += "&contains=" + url.QueryEscape(marker)
	}
	_, err = retry.Never(ctx, retry.Options{Interval: time.Second}, 5*time.Second,
		func(ctx context.Context) (*httpx.Response, error) {
			response, err := s.client.Do(ctx, httpx.Request{Method: http.MethodGet, URL: endpoint}, 0, 0)
			if err != nil {
				return nil, retry.Transient(err)
			}
			return response, nil
		}, func(response *httpx.Response) bool {
			return response != nil && response.Succeeded() && response.HasBody()
		})
	if err != nil {
		return fmt.Errorf("checking that API Portal webhook event %q was not delivered: %w", eventType, err)
	}
	return nil
}

func (s *Steps) resetWebhookEvents(ctx context.Context) error {
	block, err := s.webhookPartition(ctx)
	if err != nil {
		return err
	}
	base, err := s.topo.URL("testbench", "webhook")
	if err != nil {
		return err
	}
	response, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodPost,
		URL:    base + "/" + block + "/test/reset",
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("resetting API Portal webhook events: %w", err)
	}
	if !response.Succeeded() {
		return fmt.Errorf("resetting API Portal webhook events failed: %s", response.Describe())
	}
	return nil
}

func (s *Steps) sendSessionRequest(ctx context.Context, path string) error {
	return s.sendSessionRequestAt(ctx, path, portalService)
}

func (s *Steps) sendSessionRequestAt(ctx context.Context, path, portal string) error {
	path, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	cookie, err := tcontext.ResolveString(ctx, "apiPortalSessionCookie")
	if err != nil {
		return fmt.Errorf("API Portal session cookie is unavailable: %w", err)
	}
	base, err := s.portalBaseNamed(portal)
	if err != nil {
		return err
	}
	response, err := s.client.Do(ctx, httpx.Request{
		Method:  http.MethodGet,
		URL:     s.portalRequestURL(base, path),
		Headers: map[string]string{"Cookie": cookie},
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal session request: %w", err)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

func (s *Steps) verifyAIDiscoveryDisabled(ctx context.Context, view, role string) (err error) {
	view, err = stepscommon.Expand(ctx, view)
	if err != nil {
		return err
	}
	if strings.Contains(view, "/") || view == "" {
		return fmt.Errorf("API Portal view must be a single non-empty path segment")
	}
	base, cookie, csrf, err := s.csrfSession(ctx, role)
	if err != nil {
		return err
	}
	configURL := base + "/api-portal/" + orgHandle + "/views/" + view + "/llms-config"
	setDiscovery := func(enabled bool) (*httpx.Response, error) {
		payload, marshalErr := json.Marshal(map[string]any{
			"aiEnabled":         enabled,
			"portalName":        "",
			"portalDescription": "",
		})
		if marshalErr != nil {
			return nil, marshalErr
		}
		response, requestErr := s.client.Do(ctx, httpx.Request{
			Method: http.MethodPut, URL: configURL,
			Headers: map[string]string{
				"Cookie":       cookie,
				"X-CSRF-Token": csrf,
			},
			Body:        payload,
			ContentType: "application/json",
		}, 0, 0)
		if requestErr != nil {
			return nil, requestErr
		}
		return response, nil
	}
	disabled, err := setDiscovery(false)
	if err != nil {
		return fmt.Errorf("disabling API Portal AI discovery: %w", err)
	}
	if !disabled.Succeeded() {
		return fmt.Errorf("disabling API Portal AI discovery failed: %s", disabled.Describe())
	}
	defer func() {
		restored, restoreErr := setDiscovery(true)
		if restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("restoring API Portal AI discovery: %w", restoreErr))
			return
		}
		if !restored.Succeeded() {
			err = errors.Join(err, fmt.Errorf("restoring API Portal AI discovery failed: %s", restored.Describe()))
		}
	}()
	response, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodGet,
		URL:    base + "/api-portal/" + orgHandle + "/views/" + view + "/llms.txt",
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("requesting API Portal AI discovery while disabled: %w", err)
	}
	if response.StatusCode != http.StatusNotFound {
		return fmt.Errorf("API Portal AI discovery returned status %d while disabled, want %d", response.StatusCode, http.StatusNotFound)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

// csrfSession signs the role in through the portal's file login and returns the portal base URL,
// the session cookie and the CSRF token a session-authenticated write must send. The session is
// refreshed once after login, as the browser client does, because local login regenerates the
// session after the first XSRF cookie is issued.
func (s *Steps) csrfSession(ctx context.Context, role string) (base, cookie, csrf string, err error) {
	if err = s.submitFileLogin(ctx, role); err != nil {
		return "", "", "", err
	}
	cookie, err = tcontext.ResolveString(ctx, "apiPortalSessionCookie")
	if err != nil {
		return "", "", "", fmt.Errorf("API Portal session cookie is unavailable: %w", err)
	}
	base, err = s.portalBase()
	if err != nil {
		return "", "", "", err
	}
	refresh, err := s.client.Do(ctx, httpx.Request{
		Method:  http.MethodGet,
		URL:     base + apiPrefix + "/organizations/" + orgHandle,
		Headers: map[string]string{"Cookie": cookie},
	}, 0, 0)
	if err != nil {
		return "", "", "", fmt.Errorf("refreshing API Portal session: %w", err)
	}
	cookie = mergeCookieHeader(cookie, refresh.Headers.Values("Set-Cookie"))
	if err := tcontext.Set(ctx, "apiPortalSessionCookie", cookie); err != nil {
		return "", "", "", err
	}
	csrf, err = csrfToken(cookie)
	if err != nil {
		return "", "", "", err
	}
	return base, cookie, csrf, nil
}

// csrfToken returns the decoded XSRF-TOKEN value from a Cookie header.
func csrfToken(cookie string) (string, error) {
	for _, pair := range strings.Split(cookie, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if ok && name == "XSRF-TOKEN" {
			token, err := url.QueryUnescape(value)
			if err != nil {
				return "", fmt.Errorf("decoding API Portal CSRF token: %w", err)
			}
			if token != "" {
				return token, nil
			}
		}
	}
	return "", fmt.Errorf("API Portal session did not provide a CSRF token")
}

// sendCSRFSessionRequest signs the role in and sends a session-authenticated request with its CSRF
// token to a portal page route written in full (for example /api-portal/default/views/v/llms-config).
// A values table becomes the JSON body. The response is published for the assertions that follow.
func (s *Steps) sendCSRFSessionRequest(ctx context.Context, method, path, role string, table *godog.Table) error {
	path, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(path, "/api-portal/") {
		return fmt.Errorf("API Portal session path %q must be a full /api-portal/... page path", path)
	}
	var payload []byte
	if table != nil {
		values, err := tableJSONValues(ctx, table)
		if err != nil {
			return err
		}
		if payload, err = json.Marshal(values); err != nil {
			return fmt.Errorf("encoding API Portal session request body: %w", err)
		}
	}
	base, cookie, csrf, err := s.csrfSession(ctx, role)
	if err != nil {
		return err
	}
	request := httpx.Request{
		Method:  strings.ToUpper(method),
		URL:     portalPublicURL(base, path),
		Headers: map[string]string{"Cookie": cookie, "X-CSRF-Token": csrf},
	}
	if payload != nil {
		request.Body = payload
		request.ContentType = "application/json"
	}
	response, err := s.client.Do(ctx, request, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal session %s %s: %w", method, path, err)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

func (s *Steps) sendCSRFSessionRequestWithValues(ctx context.Context, method, path, role string, table *godog.Table) error {
	return s.sendCSRFSessionRequest(ctx, method, path, role, table)
}

func (s *Steps) sendCSRFSessionRequestWithoutBody(ctx context.Context, method, path, role string) error {
	return s.sendCSRFSessionRequest(ctx, method, path, role, nil)
}

func mergeCookieHeader(existing string, setCookies []string) string {
	pairs := make(map[string]string)
	order := make([]string, 0)
	for _, raw := range strings.Split(existing, ";") {
		pair := strings.TrimSpace(strings.SplitN(raw, ";", 2)[0])
		name, _, ok := strings.Cut(pair, "=")
		if !ok || name == "" {
			continue
		}
		if _, seen := pairs[name]; !seen {
			order = append(order, name)
		}
		pairs[name] = pair
	}
	for _, raw := range setCookies {
		pair := strings.TrimSpace(strings.SplitN(raw, ";", 2)[0])
		name, _, ok := strings.Cut(pair, "=")
		if !ok || name == "" {
			continue
		}
		if _, seen := pairs[name]; !seen {
			order = append(order, name)
		}
		pairs[name] = pair
	}
	updated := make([]string, 0, len(order))
	for _, name := range order {
		updated = append(updated, pairs[name])
	}
	return strings.Join(updated, "; ")
}

func (s *Steps) sendAuthenticated(ctx context.Context, method, path, role string) error {
	return s.sendPortal(ctx, method, path, role, nil)
}

// sendAuthenticatedUntilStatus repeats a request until it answers with want, publishing the
// matching response for the assertions that follow. A read issued immediately after a write
// can observe the pre-write state, so a single-shot assertion on the settled status is a
// race; the wanted status is still required, only waited for.
func (s *Steps) sendAuthenticatedUntilStatus(ctx context.Context, method, path, role string, want int) error {
	var last int
	result, err := retry.Until(ctx, retry.Options{Interval: time.Second},
		func(ctx context.Context) (bool, error) {
			if err := s.sendPortal(ctx, method, path, role, nil); err != nil {
				return false, retry.Transient(err)
			}
			response, err := httpx.Published(ctx)
			if err != nil {
				return false, err
			}
			last = response.StatusCode
			return last == want, nil
		}, func(matched bool) bool { return matched })
	if err != nil {
		return fmt.Errorf("waiting for API Portal %s %s to answer %d (last %d): %w",
			method, path, want, last, err)
	}
	if !result {
		return fmt.Errorf("API Portal %s %s never answered %d (last %d)", method, path, want, last)
	}
	return nil
}

// sendAuthenticatedUntilBodyContains waits for a read to expose a value written
// by an earlier request. It deliberately accepts only GET: retrying a mutation
// would repeat its side effects rather than wait for its committed state to become
// observable.
func (s *Steps) sendAuthenticatedUntilBodyContains(ctx context.Context, method, path, role, want string) error {
	if !strings.EqualFold(strings.TrimSpace(method), http.MethodGet) {
		return fmt.Errorf("API Portal response-body readiness requires GET, got %q", method)
	}
	want, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	if want == "" {
		return fmt.Errorf("API Portal response-body readiness requires a non-empty value")
	}

	last := "no response"
	result, err := retry.Until(ctx, retry.Options{Interval: time.Second},
		func(ctx context.Context) (bool, error) {
			if err := s.sendPortal(ctx, http.MethodGet, path, role, nil); err != nil {
				last = err.Error()
				return false, retry.Transient(err)
			}
			response, err := httpx.Published(ctx)
			if err != nil {
				return false, err
			}
			last = response.Describe()
			return response.StatusCode == http.StatusOK && strings.Contains(string(response.Body), want), nil
		}, func(matched bool) bool { return matched })
	if err != nil {
		return fmt.Errorf("waiting for API Portal GET %s to expose %q (last %s): %w", path, want, last, err)
	}
	if !result {
		return fmt.Errorf("API Portal GET %s never exposed %q (last %s)", path, want, last)
	}
	return nil
}

// sendAuthenticatedUntilJSONArrayLength waits for an API Portal read to observe an exact
// top-level JSON array length. It deliberately accepts only GET: retrying a mutation would
// repeat its side effects rather than wait for its committed state to become observable.
func (s *Steps) sendAuthenticatedUntilJSONArrayLength(ctx context.Context, method, path, role, field string, want int) error {
	if !strings.EqualFold(strings.TrimSpace(method), http.MethodGet) {
		return fmt.Errorf("API Portal JSON array readiness requires GET, got %q", method)
	}
	field, err := stepscommon.Expand(ctx, field)
	if err != nil {
		return err
	}
	if strings.TrimSpace(field) == "" {
		return fmt.Errorf("API Portal JSON array readiness requires a field")
	}
	if strings.ContainsAny(field, ".[") {
		return fmt.Errorf("API Portal JSON array readiness supports only top-level fields, got %q", field)
	}

	last := "no response"
	result, err := retry.Until(ctx, retry.Options{Interval: time.Second},
		func(ctx context.Context) (bool, error) {
			if err := s.sendPortal(ctx, http.MethodGet, path, role, nil); err != nil {
				last = err.Error()
				return false, retry.Transient(err)
			}
			response, err := httpx.Published(ctx)
			if err != nil {
				return false, err
			}
			if response.StatusCode != http.StatusOK {
				last = response.Describe()
				return false, nil
			}
			got, err := topLevelJSONArrayLength(response.Body, field)
			if err != nil {
				return false, err
			}
			last = fmt.Sprintf("field %q had %d items", field, got)
			return got == want, nil
		}, func(matched bool) bool { return matched })
	if err != nil {
		return fmt.Errorf("waiting for API Portal GET %s to expose %d items in JSON array field %q (last %s): %w",
			path, want, field, last, err)
	}
	if !result {
		return fmt.Errorf("API Portal GET %s never exposed %d items in JSON array field %q (last %s)",
			path, want, field, last)
	}
	return nil
}

func topLevelJSONArrayLength(body []byte, field string) (int, error) {
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		return 0, fmt.Errorf("response is not a JSON object: %w", err)
	}
	value, ok := document[field]
	if !ok {
		return 0, fmt.Errorf("JSON array field %q is absent", field)
	}
	array, ok := value.([]any)
	if !ok {
		return 0, fmt.Errorf("JSON field %q is not an array", field)
	}
	return len(array), nil
}

func (s *Steps) sendAuthenticatedWithHeader(ctx context.Context, method, path, role, name, value string) error {
	return s.sendPortalWithHeaders(ctx, method, path, role, nil, map[string]string{name: value})
}

func (s *Steps) sendAuthenticatedWithBody(
	ctx context.Context, method, path, role string, body *godog.DocString,
) error {
	payload, err := stepscommon.Expand(ctx, body.Content)
	if err != nil {
		return err
	}
	if !json.Valid([]byte(payload)) {
		return fmt.Errorf("API Portal request body is not valid JSON")
	}
	return s.sendPortal(ctx, method, path, role, []byte(payload))
}

func (s *Steps) sendAuthenticatedWithValues(
	ctx context.Context, method, path, role string, table *godog.Table,
) error {
	values, err := tableJSONValues(ctx, table)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("encoding API Portal request values: %w", err)
	}
	return s.sendPortal(ctx, method, path, role, payload)
}

// tableJSONValues reads a two-column values table into a JSON object. Context and unique-name
// placeholders are expanded, and a cell whose trimmed value is a JSON array or object is sent as
// that structure; every other cell is sent as a string.
func tableJSONValues(ctx context.Context, table *godog.Table) (map[string]any, error) {
	if table == nil || len(table.Rows) == 0 {
		return nil, fmt.Errorf("API Portal values table is required")
	}
	values := make(map[string]any, len(table.Rows))
	for i, row := range table.Rows {
		if row == nil || len(row.Cells) != 2 {
			return nil, fmt.Errorf("API Portal values row %d must have exactly two cells", i+1)
		}
		key, err := stepscommon.Expand(ctx, strings.TrimSpace(row.Cells[0].Value))
		if err != nil {
			return nil, err
		}
		if key == "" {
			return nil, fmt.Errorf("API Portal values row %d has an empty key", i+1)
		}
		if _, duplicate := values[key]; duplicate {
			return nil, fmt.Errorf("API Portal values row %d repeats the key %q", i+1, key)
		}
		value, err := stepscommon.Expand(ctx, row.Cells[1].Value)
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(value)
		if trimmed != "" && strings.ContainsAny(trimmed[:1], "[{") && json.Valid([]byte(trimmed)) {
			var structured any
			if err := json.Unmarshal([]byte(trimmed), &structured); err != nil {
				return nil, fmt.Errorf("API Portal value %q is not valid JSON: %w", key, err)
			}
			values[key] = structured
			continue
		}
		values[key] = value
	}
	return values, nil
}

func (s *Steps) sendAuthenticatedMultipartMetadataOnly(
	ctx context.Context, method, path, role string, metadata *godog.DocString,
) error {
	metadataJSON, err := stepscommon.Expand(ctx, metadata.Content)
	if err != nil {
		return err
	}
	if !json.Valid([]byte(metadataJSON)) {
		return fmt.Errorf("API Portal metadata is not valid JSON")
	}
	return s.sendPortalMultipart(ctx, method, path, role, []byte(metadataJSON), nil, "", "")
}

func (s *Steps) sendAuthenticatedMultipartWithDefinition(
	ctx context.Context, method, path, role, definition string, metadata *godog.DocString,
) error {
	metadataJSON, err := stepscommon.Expand(ctx, metadata.Content)
	if err != nil {
		return err
	}
	if !json.Valid([]byte(metadataJSON)) {
		return fmt.Errorf("API Portal metadata is not valid JSON")
	}
	definition, err = stepscommon.Expand(ctx, definition)
	if err != nil {
		return err
	}
	definitionContent, filename, contentType, err := portalDefinition(definition)
	if err != nil {
		return err
	}
	return s.sendPortalMultipart(ctx, method, path, role, []byte(metadataJSON), definitionContent, filename, contentType)
}

func (s *Steps) sendArtifactUpload(ctx context.Context, method, kind, path, role string) error {
	method = strings.ToUpper(method)
	path, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	base, token, err := s.authedAs(ctx, role)
	if err != nil {
		return err
	}
	zipData, err := artifactZip(ctx, kind)
	if err != nil {
		return err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("artifact", "artifact.zip")
	if err != nil {
		return err
	}
	if _, err := part.Write(zipData); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	headers := map[string]string{"Authorization": "Bearer " + token}
	if strings.EqualFold(strings.TrimSpace(kind), "oversized") {
		headers["Expect"] = "100-continue"
	}
	response, err := s.client.Do(ctx, httpx.Request{
		Method: method, URL: s.portalRequestURL(base, path), Headers: headers,
		Body: body.Bytes(), ContentType: writer.FormDataContentType(),
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal artifact upload: %w", err)
	}
	if err := tcontext.Set(ctx, httpx.ResponseKey, response); err != nil {
		return err
	}
	if method != http.MethodPost || !response.Succeeded() {
		return nil
	}
	var created map[string]any
	if err := json.Unmarshal(response.Body, &created); err != nil {
		return fmt.Errorf("decoding API Portal artifact response: %w", err)
	}
	id := stringValue(created, "id")
	if id == "" {
		id = stringValue(created, "uuid")
	}
	if id == "" {
		return fmt.Errorf("API Portal artifact response contained no resource identifier")
	}
	return s.registerPortalResource(ctx, portalAPIKind, id, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: "/apis"})
}

func artifactZip(ctx context.Context, kind string) ([]byte, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	handle, _ := tcontext.Get(ctx, "apiId")
	apiID, _ := handle.(string)
	if apiID == "" {
		artifact, _ := tcontext.Get(ctx, "artifactId")
		apiID, _ = artifact.(string)
	}
	if apiID == "" {
		return nil, fmt.Errorf("API Portal artifact requires apiId in context")
	}
	labels := ""
	switch kind {
	case "labels-empty", "labels-update-empty":
		labels = "  labels: []\n"
	case "valid-with-label":
		labelID, err := tcontext.ResolveString(ctx, "labelId")
		if err != nil {
			return nil, err
		}
		labels = fmt.Sprintf("  labels: [%s]\n", labelID)
	}
	if kind == "not-a-zip" {
		// Named artifact.zip by the upload step, but not a ZIP archive.
		return []byte("this upload is not a ZIP archive"), nil
	}
	description := "artifact upload fixture"
	if kind == "stable-update" {
		description = "artifact upload updated"
	}
	apiType, status := "REST", "PUBLISHED"
	switch kind {
	case "mcp-type":
		apiType = "MCP"
	case "invalid-status":
		status = "BOGUS"
	}
	metadata := fmt.Sprintf(`metadata:
  name: %s
spec:
  displayName: "Artifact API %s"
  version: "v1.0"
  description: "%s"
  type: %s
  status: %s
%s
  endpoints:
    sandboxUrl: https://sandbox.example.invalid/%s
    productionUrl: https://backend.example.invalid/%s
`, apiID, apiID, description, apiType, status, labels, apiID, apiID)
	definition := `{"openapi":"3.0.3","info":{"title":"artifact","version":"1"},"paths":{}}`
	entries := make([]struct{ name, content string }, 0, 2)
	switch kind {
	case "missing-metadata":
		entries = append(entries, struct{ name, content string }{"definition.json", definition})
	case "missing-definition":
		entries = append(entries, struct{ name, content string }{"api.yaml", metadata})
	case "zip-slip":
		entries = append(entries,
			struct{ name, content string }{"api.yaml", metadata},
			struct{ name, content string }{"definition.json", definition},
			struct{ name, content string }{"../outside.txt", "x"})
	case "too-many-entries":
		entries = append(entries,
			struct{ name, content string }{"api.yaml", metadata},
			struct{ name, content string }{"definition.json", definition})
		for i := 0; i < 520; i++ {
			entries = append(entries, struct{ name, content string }{fmt.Sprintf("filler/%03d.txt", i), "x"})
		}
	case "oversized":
		entries = append(entries, struct{ name, content string }{"large.bin", strings.Repeat("A", 11*1024*1024)})
	case "absolute-path":
		entries = append(entries,
			struct{ name, content string }{"api.yaml", metadata},
			struct{ name, content string }{"definition.json", definition},
			struct{ name, content string }{"/tmp/apip-absolute-entry.txt", "x"})
	default:
		entries = append(entries,
			struct{ name, content string }{"api.yaml", metadata},
			struct{ name, content string }{"definition.json", definition})
	}
	var out bytes.Buffer
	archive := zip.NewWriter(&out)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		file, err := archive.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := file.Write([]byte(entry.content)); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (s *Steps) createMultipartResource(ctx context.Context, path, role, definition, storeAs string, metadata *godog.DocString) error {
	path, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	metadataJSON, err := stepscommon.Expand(ctx, metadata.Content)
	if err != nil {
		return err
	}
	if !json.Valid([]byte(metadataJSON)) {
		return fmt.Errorf("API Portal metadata is not valid JSON")
	}
	definitionContent, filename, contentType, err := portalDefinition(definition)
	if err != nil {
		return err
	}
	base, token, err := s.authedAs(ctx, role)
	if err != nil {
		return err
	}
	var created map[string]any
	if err := s.sendPortalMultipartAndDecode(ctx, base, token, http.MethodPost, path, []byte(metadataJSON), definitionContent, filename, contentType, &created); err != nil {
		return err
	}
	id := stringValue(created, "id")
	if id == "" {
		id = stringValue(created, "uuid")
	}
	if id == "" {
		id = stringValue(created, "subscriptionId")
	}
	if id == "" {
		return fmt.Errorf("POST %s succeeded without a resource identifier", path)
	}
	if err := s.registerPortalResource(ctx, portalResourceKind(path), id, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: path}); err != nil {
		return err
	}
	return tcontext.Set(ctx, storeAs, id)
}

func (s *Steps) createMCPServerWithLabel(ctx context.Context, label, storeAs string) error {
	label, err := stepscommon.Expand(ctx, label)
	if err != nil {
		return err
	}
	id, err := unique.Unique(ctx, "portal-view-mcp")
	if err != nil {
		return err
	}
	base, token, err := s.authedAs(ctx, "publisher")
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(map[string]any{
		"id": id, "name": "View Scoped MCP " + id, "version": "v1.0", "type": "MCP", "status": "PUBLISHED",
		"labels": []string{label},
		"endPoints": map[string]string{
			"productionURL": "https://mcp.example.invalid/" + id,
			"sandboxURL":    "https://mcp-sandbox.example.invalid/" + id,
		},
	})
	if err != nil {
		return fmt.Errorf("encoding API Portal MCP metadata: %w", err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := s.postYAMLMultipart(ctx, base, token, "/mcp-servers", string(metadata), portalMCPDefinition, &created, ""); err != nil {
		return err
	}
	if created.ID == "" {
		created.ID = id
	}
	if err := s.registerPortalResource(ctx, portalMCPServerKind, created.ID, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: "/mcp-servers"}); err != nil {
		return err
	}
	return tcontext.Set(ctx, storeAs, created.ID)
}

// restTitledDefinitionPrefix names the REST fixture with a scenario-chosen title, written
// "rest titled <title>", so a scenario can find an API by a word that only its definition holds.
const restTitledDefinitionPrefix = "rest titled "

func portalDefinition(kind string) ([]byte, string, string, error) {
	if trimmed := strings.TrimSpace(kind); strings.HasPrefix(strings.ToLower(trimmed+" "), restTitledDefinitionPrefix) {
		title := strings.TrimSpace(trimmed[len(restTitledDefinitionPrefix)-1:])
		if title == "" {
			return nil, "", "", fmt.Errorf("the titled REST definition needs a title")
		}
		document, err := json.Marshal(map[string]any{
			"openapi": "3.0.3",
			"info":    map[string]string{"title": title, "version": "1.0.0"},
			"paths":   map[string]any{"/ping": map[string]any{"get": map[string]any{"responses": map[string]any{"200": map[string]string{"description": "ok"}}}}},
		})
		if err != nil {
			return nil, "", "", fmt.Errorf("encoding the titled REST definition: %w", err)
		}
		return document, "definition.json", "application/json", nil
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "rest near-limit":
		return paddedRESTDefinition(uploadNearLimitBytes)
	case "rest oversized":
		return paddedRESTDefinition(uploadOversizedBytes)
	case "rest", "websocket", "websub":
		return []byte(portalRESTDefinition), "definition.json", "application/json", nil
	case "graphql":
		return []byte("type Query { hello: String }"), "schema.graphql", "application/octet-stream", nil
	case "soap":
		return []byte(`<?xml version="1.0"?><definitions xmlns="http://schemas.xmlsoap.org/wsdl/"></definitions>`), "definition.wsdl", "application/xml", nil
	case "mcp":
		return []byte(portalMCPDefinition), "definition.yaml", "application/yaml", nil
	default:
		// Keep the step useful for negative tests that intentionally provide malformed
		// definition content. Named fixtures use the cases above; any other value is the
		// definition payload itself, matching the original multipart step contract.
		return []byte(kind), "definition.json", "application/json", nil
	}
}

// uploadNearLimitBytes and uploadOversizedBytes sit just under and just over the portal's default
// per-file upload limit (uploads.max_bytes, 10 MiB).
const (
	uploadNearLimitBytes = 10<<20 - 256<<10
	uploadOversizedBytes = 10<<20 + 256<<10
)

// paddedRESTDefinition returns the REST fixture definition padded with a long description to
// exactly size bytes.
func paddedRESTDefinition(size int) ([]byte, string, string, error) {
	const prefix = `{"openapi":"3.0.3","info":{"title":"Fixture API","version":"1.0.0","description":"`
	const suffix = `"},"paths":{"/ping":{"get":{"responses":{"200":{"description":"ok"}}}}}}`
	padding := size - len(prefix) - len(suffix)
	if padding < 0 {
		return nil, "", "", fmt.Errorf("padded REST definition size %d is too small", size)
	}
	return []byte(prefix + strings.Repeat("a", padding) + suffix), "definition.json", "application/json", nil
}

func (s *Steps) createJSONResource(ctx context.Context, path, role, storeAs string, body *godog.DocString) error {
	path, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	payload, err := stepscommon.Expand(ctx, body.Content)
	if err != nil {
		return err
	}
	var request map[string]any
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		return fmt.Errorf("API Portal resource body is not valid JSON: %w", err)
	}
	if strings.HasPrefix(path, "/key-managers") {
		if id, ok := request["id"].(string); ok {
			request["id"] = strings.ReplaceAll(id, "_", "-")
		}
	}
	base, token, err := s.authedAs(ctx, role)
	if err != nil {
		return err
	}
	response, err := s.doJSON(ctx, base, token, http.MethodPost, path, request)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, httpx.ResponseKey, response); err != nil {
		return err
	}
	if !response.Succeeded() {
		return fmt.Errorf("POST %s failed: %s", path, response.Describe())
	}
	var created map[string]any
	if err := json.Unmarshal(response.Body, &created); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}
	id := stringValue(created, "id")
	if id == "" {
		id = stringValue(created, "uuid")
	}
	if id == "" {
		id = stringValue(created, "keyId")
	}
	if id == "" {
		id = stringValue(request, "id")
	}
	if id == "" {
		return fmt.Errorf("POST %s succeeded without a resource identifier", path)
	}
	if err := s.registerPortalResource(ctx, portalResourceKind(path), id, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: path, role: portalCleanupRole(path, role)}); err != nil {
		return err
	}
	return tcontext.Set(ctx, storeAs, id)
}

func (s *Steps) createJSONResourceWithValues(ctx context.Context, path, role, storeAs string, table *godog.Table) error {
	values, err := tableJSONValues(ctx, table)
	if err != nil {
		return err
	}
	body, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("encoding API Portal resource values: %w", err)
	}
	return s.createJSONResource(ctx, path, role, storeAs, &godog.DocString{Content: string(body)})
}

func (s *Steps) registerResourceForCleanup(ctx context.Context, id, path string) error {
	id, err := stepscommon.Expand(ctx, id)
	if err != nil {
		return err
	}
	path, err = stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	return s.registerPortalResource(ctx, portalResourceKind(path), id, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: path})
}

// sendAuthenticatedMultipartTemplate renders a resource template with the values table and sends
// the document as the one file part of a multipart request, under the named form field, as the
// named role. The file is named after the template.
func (s *Steps) sendAuthenticatedMultipartTemplate(
	ctx context.Context, method, path, role, field, template string, table *godog.Table,
) error {
	path, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	templatePath, err := stepscommon.ResourceTemplatePath(s.featureRoot, template)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("read resource template %q: %w", template, err)
	}
	document, err := stepscommon.RenderResourceTemplate(ctx, template, content, table)
	if err != nil {
		return fmt.Errorf("resource template %q: %w", template, err)
	}
	base, token, err := s.authedAs(ctx, role)
	if err != nil {
		return err
	}
	body, contentType, err := multipartFileBody(field, filepath.Base(templatePath), document)
	if err != nil {
		return err
	}
	response, err := s.client.Do(ctx, httpx.Request{
		Method: strings.ToUpper(method), URL: s.portalRequestURL(base, path),
		Headers: map[string]string{"Authorization": "Bearer " + token},
		Body:    body, ContentType: contentType,
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal %s %s: %w", method, path, err)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

// multipartFileBody builds a multipart body holding one file part. The part's type follows the
// file name: YAML for .yaml/.yml, otherwise plain text.
func multipartFileBody(field, filename, content string) ([]byte, string, error) {
	if strings.TrimSpace(field) == "" {
		return nil, "", fmt.Errorf("a multipart file needs a form field name")
	}
	contentType := "text/plain"
	if lower := strings.ToLower(filename); strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		contentType = "application/yaml"
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := addPart(writer, field, filename, contentType, content); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

// restoreOrganizationAfterScenario reads the organization's settings now and writes them back when
// the scenario ends, pass or fail. An organization update replaces every settable field, so a
// scenario that changes one must not leave it, or the fields it did not send, changed for the
// runners that share the organization.
func (s *Steps) restoreOrganizationAfterScenario(ctx context.Context, orgID string) error {
	orgID, err := stepscommon.Expand(ctx, orgID)
	if err != nil {
		return err
	}
	base, token, err := s.authed(ctx)
	if err != nil {
		return err
	}
	path := "/organizations/" + url.PathEscape(orgID)
	response, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodGet, URL: base + apiPrefix + path,
		Headers: map[string]string{"Authorization": "Bearer " + token},
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("reading API Portal organization %q: %w", orgID, err)
	}
	if !response.Succeeded() {
		return fmt.Errorf("reading API Portal organization %q failed: %s", orgID, response.Describe())
	}
	var current map[string]any
	if err := json.Unmarshal(response.Body, &current); err != nil {
		return fmt.Errorf("decoding API Portal organization %q: %w", orgID, err)
	}
	payload := organizationRestorePayload(current)
	return s.registerPortalResource(ctx, portalOrganizationKind, orgID, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodPut, path: path, payload: func(string) any { return payload }})
}

// organizationRestorePayload keeps the fields an organization update writes. A field the
// organization has no value for is left out, which the update stores as empty again.
func organizationRestorePayload(org map[string]any) map[string]any {
	payload := map[string]any{}
	for _, field := range []string{
		"id", "idpRefId", "displayName", "cpRefId",
		"businessOwner", "businessOwnerContact", "businessOwnerEmail",
	} {
		if value, ok := org[field].(string); ok && value != "" {
			payload[field] = value
		}
	}
	if configuration, ok := org["configuration"].(map[string]any); ok {
		payload["configuration"] = configuration
	}
	return payload
}

// registerAPIKeyForCleanup registers an API key that a scenario generated with a request step, so
// that its owner revokes it when the scenario ends. An API with an active key cannot be deleted.
func (s *Steps) registerAPIKeyForCleanup(ctx context.Context, keyID, apiID, role string) error {
	keyID, err := stepscommon.Expand(ctx, keyID)
	if err != nil {
		return err
	}
	apiID, err = stepscommon.Expand(ctx, apiID)
	if err != nil {
		return err
	}
	request, err := apiKeyRevokeCleanup(apiID, role)
	if err != nil {
		return err
	}
	return s.registerPortalResource(ctx, apiKeyKindOf(apiID), keyID, cleanup.ScenarioScope, request)
}

// apiKeyKindOf is the clean-up kind for the keys of one API. A kind keeps one deleter, and the
// revoke path names the API, so keys of different APIs need kinds of their own. It shares the
// order of the other API keys, so keys are revoked before their API is deleted.
func apiKeyKindOf(apiID string) cleanup.Kind {
	return cleanup.Kind{Name: portalAPIKeyKind.Name + ":" + apiID, Order: portalAPIKeyKind.Order}
}

// apiKeyRevokeCleanup is the clean-up request that revokes one key of an API as its owner.
func apiKeyRevokeCleanup(apiID, role string) (portalCleanupRequest, error) {
	if strings.TrimSpace(apiID) == "" {
		return portalCleanupRequest{}, errors.New("an API key registered for clean-up needs its API id")
	}
	if _, err := portalActor(role); err != nil {
		return portalCleanupRequest{}, err
	}
	return portalCleanupRequest{
		method:  http.MethodPost,
		path:    "/apis/" + url.PathEscape(apiID) + "/api-keys/revoke",
		role:    role,
		payload: func(id string) any { return map[string]any{"keyId": id} },
	}, nil
}

func (s *Steps) registerApplicationKeyMappingForCleanup(ctx context.Context, mappingID, applicationID string) error {
	mappingID, err := stepscommon.Expand(ctx, mappingID)
	if err != nil {
		return err
	}
	applicationID, err = stepscommon.Expand(ctx, applicationID)
	if err != nil {
		return err
	}
	return s.registerPortalResource(ctx, portalApplicationKeyMappingKind, mappingID, cleanup.ScenarioScope,
		portalCleanupRequest{
			method: http.MethodDelete,
			path:   "/applications/" + applicationID + "/oauth-keys",
			role:   "developer",
		})
}

func (s *Steps) uploadAPIContent(ctx context.Context, apiID, role string) error {
	return s.uploadAPIContentRequest(ctx, http.MethodPost, "default", "/apis", apiID, role)
}

func (s *Steps) uploadAPIContentVariant(ctx context.Context, variant, apiID, role string) error {
	return s.uploadAPIContentRequest(ctx, http.MethodPost, variant, "/apis", apiID, role)
}

func (s *Steps) uploadAPIContentMethodVariant(ctx context.Context, method, variant, apiID, role string) error {
	return s.uploadAPIContentRequest(ctx, strings.ToUpper(method), variant, "/apis", apiID, role)
}

func (s *Steps) uploadMCPContentMethodVariant(ctx context.Context, method, variant, mcpID, role string) error {
	return s.uploadAPIContentRequest(ctx, strings.ToUpper(method), variant, "/mcp-servers", mcpID, role)
}

// uploadAPIContentRequest sends a content archive to the assets endpoint of an API (/apis) or an
// MCP server (/mcp-servers) and publishes the response.
func (s *Steps) uploadAPIContentRequest(ctx context.Context, method, variant, collection, artifactID, role string) error {
	artifactID, err := stepscommon.Expand(ctx, artifactID)
	if err != nil {
		return err
	}
	archive, err := contentZip(strings.ToLower(strings.TrimSpace(variant)))
	if err != nil {
		return err
	}
	base, token, err := s.authedAs(ctx, role)
	if err != nil {
		return err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if strings.EqualFold(strings.TrimSpace(variant), "image-metadata") {
		// The multipart field is sent in addition to the content archive, matching the API
		// Portal contract that maps an image to its key.
		if err := writer.WriteField("imageMetadata", `{"api-icon":"api-icon.png"}`); err != nil {
			return err
		}
	}
	part, err := writer.CreateFormFile("content", "content.zip")
	if err != nil {
		return err
	}
	if _, err := part.Write(archive); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	headers := map[string]string{"Authorization": "Bearer " + token}
	if strings.EqualFold(strings.TrimSpace(variant), "oversized") {
		headers["Expect"] = "100-continue"
	}
	response, err := s.client.Do(ctx, httpx.Request{
		Method:  method,
		URL:     s.portalRequestURL(base, collection+"/"+artifactID+"/assets"),
		Headers: headers,
		Body:    body.Bytes(), ContentType: writer.FormDataContentType(),
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal content upload: %w", err)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

// Content fixture bytes. contentIconPNG is the default icon; contentAlternateIconPNG differs from it
// so a replacement can be told apart by its bytes.
const (
	contentIconPNG          = "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d49444154789c6360000002000100ffff03000006000557bfabd40000000049454e44ae426082"
	contentAlternateIconPNG = "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000a49444154789c6300010000050001aabbccdd0000000049454e44ae426082"
	contentLandingPage      = "<section>API Portal content</section>"
	contentHowToDoc         = "# Setup guide\n\nCall the ping operation with your subscription token.\n"
	contentOverviewDoc      = "# Overview\n\nThis API answers ping requests.\n"
	contentExtraDoc         = "# Extra guide\n\nAdded by a later upload.\n"
)

// contentZip builds the content archive for a variant. Every archive has one wrapper folder
// (bundle/), the layout the portal accepts:
//
//	default, image-metadata   web/api-content.hbs (landing page) and web/api-icon.png
//	alternate                 the same with a different icon
//	with-docs                 default plus docs/HowTo/setup.md and a top-level docs/overview.md
//	skips                     default plus a nested web/nested/deep.md and an unsupported web/run.sh
//	no-content-dirs           only bundle/readme.txt, with neither web/ nor docs/
//	replace-and-add           the alternate icon and docs/HowTo/extra.md, without the landing page
//	near-limit, oversized     default plus a stored web/large.png just under or just over the upload limit
func contentZip(variant string) ([]byte, error) {
	icon, err := hex.DecodeString(contentIconPNG)
	if err != nil {
		return nil, err
	}
	alternate, err := hex.DecodeString(contentAlternateIconPNG)
	if err != nil {
		return nil, err
	}
	base := []zipEntry{
		{name: "bundle/web/api-content.hbs", content: []byte(contentLandingPage)},
		{name: "bundle/web/api-icon.png", content: icon},
	}
	var entries []zipEntry
	switch variant {
	case "default", "image-metadata":
		entries = base
	case "alternate":
		entries = []zipEntry{base[0], {name: "bundle/web/api-icon.png", content: alternate}}
	case "with-docs":
		entries = append(base,
			zipEntry{name: "bundle/docs/HowTo/setup.md", content: []byte(contentHowToDoc)},
			zipEntry{name: "bundle/docs/overview.md", content: []byte(contentOverviewDoc)})
	case "skips":
		entries = append(base,
			zipEntry{name: "bundle/web/nested/deep.md", content: []byte("# Nested\n")},
			zipEntry{name: "bundle/web/run.sh", content: []byte("echo unsupported\n")})
	case "no-content-dirs":
		entries = []zipEntry{{name: "bundle/readme.txt", content: []byte("no web or docs folder")}}
	case "replace-and-add":
		entries = []zipEntry{
			{name: "bundle/web/api-icon.png", content: alternate},
			{name: "bundle/docs/HowTo/extra.md", content: []byte(contentExtraDoc)},
		}
	case "near-limit":
		entries = append(base, zipEntry{name: "bundle/web/large.png", content: make([]byte, uploadNearLimitBytes), stored: true})
	case "oversized":
		entries = append(base, zipEntry{name: "bundle/web/large.png", content: make([]byte, uploadOversizedBytes), stored: true})
	default:
		return nil, fmt.Errorf("unknown API Portal content variant %q", variant)
	}
	return buildZip(entries)
}

// zipEntry is one file of a generated archive. A stored entry is written uncompressed, so the
// archive is as large on the wire as the content it holds.
type zipEntry struct {
	name    string
	content []byte
	stored  bool
}

func buildZip(entries []zipEntry) ([]byte, error) {
	var out bytes.Buffer
	archive := zip.NewWriter(&out)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.stored {
			header.Method = zip.Store
		}
		file, err := archive.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := file.Write(entry.content); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (s *Steps) responseBodyEqualsHex(ctx context.Context, expected string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	want, err := hex.DecodeString(strings.TrimSpace(expected))
	if err != nil {
		return fmt.Errorf("decoding expected response bytes: %w", err)
	}
	if !bytes.Equal(response.Body, want) {
		return fmt.Errorf("API Portal response body did not match expected bytes")
	}
	return nil
}

func (s *Steps) uploadTheme(ctx context.Context, variant, viewID, role string) error {
	viewID, err := stepscommon.Expand(ctx, viewID)
	if err != nil {
		return err
	}
	base, token, err := s.authedAs(ctx, role)
	if err != nil {
		return err
	}
	archiveData, err := themeZip(strings.ToLower(strings.TrimSpace(variant)))
	if err != nil {
		return err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "theme.zip")
	if err != nil {
		return err
	}
	if _, err := part.Write(archiveData); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	headers := map[string]string{"Authorization": "Bearer " + token}
	if strings.EqualFold(strings.TrimSpace(variant), "oversized") {
		headers["Expect"] = "100-continue"
	}
	response, err := s.client.Do(ctx, httpx.Request{
		Method:  http.MethodPost,
		URL:     s.portalRequestURL(base, "/views/"+viewID+"/apply-theme"),
		Headers: headers,
		Body:    body.Bytes(), ContentType: writer.FormDataContentType(),
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal theme upload: %w", err)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

// themeOversizedBytes is past the portal's default upload limit (10 MiB).
const themeOversizedBytes = 11 << 20

// themeZip builds the theme archive for a variant. Every archive has the single wrapper folder
// (theme/) a theme needs:
//
//	default, favicon, new, old   styles/main.css and one image (brand-mark.png, favicon.ico, new.png, old.png)
//	unsupported                  only an .exe file, so the archive holds no theme file
//	scripts-only                 only scripts/app.js, so the archive holds no theme file
//	with-script                  the default theme plus scripts/app.js
//	script-in-template           the default theme plus a partial that embeds a <script> element
//	zip-slip                     the default theme plus an entry outside the extraction root
//	oversized                    the default theme plus an image past the upload limit
//	near-limit                   the default theme plus an image just under the upload limit
func themeZip(variant string) ([]byte, error) {
	const png = "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d49444154789c6360000002000100ffff03000006000557bfabd40000000049454e44ae426082"
	const ico = "00000100010010100000010020002801000016000000"
	const css = "body { color: #123456; }"
	image, err := hex.DecodeString(png)
	if err != nil {
		return nil, err
	}
	favicon, err := hex.DecodeString(ico)
	if err != nil {
		return nil, err
	}
	themed := func(imageName string, imageBytes []byte, extra ...zipEntry) []zipEntry {
		return append([]zipEntry{
			{name: "theme/styles/main.css", content: []byte(css)},
			{name: "theme/images/" + imageName, content: imageBytes},
		}, extra...)
	}
	const script = "window.themeScript = true;"
	var entries []zipEntry
	switch variant {
	case "default":
		entries = themed("brand-mark.png", image)
	case "favicon":
		entries = themed("favicon.ico", favicon)
	case "new":
		entries = themed("new.png", image)
	case "old":
		entries = themed("old.png", image)
	case "unsupported":
		entries = []zipEntry{{name: "theme/evil.exe", content: []byte("MZ")}}
	case "scripts-only":
		entries = []zipEntry{{name: "theme/scripts/app.js", content: []byte(script)}}
	case "with-script":
		entries = themed("brand-mark.png", image, zipEntry{name: "theme/scripts/app.js", content: []byte(script)})
	case "script-in-template":
		entries = themed("brand-mark.png", image,
			zipEntry{name: "theme/partials/header.hbs", content: []byte("<header><script>" + script + "</script></header>")})
	case "zip-slip":
		entries = themed("brand-mark.png", image, zipEntry{name: "../outside.css", content: []byte(css)})
	case "oversized":
		entries = themed("brand-mark.png", image, zipEntry{name: "theme/images/large.png", content: make([]byte, themeOversizedBytes), stored: true})
	case "near-limit":
		entries = themed("brand-mark.png", image, zipEntry{name: "theme/images/large.png", content: make([]byte, uploadNearLimitBytes), stored: true})
	default:
		return nil, fmt.Errorf("unknown API Portal theme variant %q", variant)
	}
	return buildZip(entries)
}

func (s *Steps) createSubscription(ctx context.Context, apiID, plan, storeAs string) error {
	apiID, err := stepscommon.Expand(ctx, apiID)
	if err != nil {
		return err
	}
	plan, err = stepscommon.Expand(ctx, plan)
	if err != nil {
		return err
	}
	base, token, err := s.authedAs(ctx, "developer")
	if err != nil {
		return err
	}
	var created struct {
		ID    string `json:"subscriptionId"`
		Token string `json:"subscriptionToken"`
	}
	response, err := s.doJSON(ctx, base, token, http.MethodPost, "/subscriptions", map[string]any{
		"artifactId": apiID, "subscriptionPlanId": plan,
	})
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, httpx.ResponseKey, response); err != nil {
		return err
	}
	if !response.Succeeded() {
		return fmt.Errorf("POST /subscriptions failed: %s", response.Describe())
	}
	if err := decodeJSON(response, "/subscriptions", &created); err != nil {
		return err
	}
	if created.ID == "" || created.Token == "" {
		return fmt.Errorf("API Portal subscription response was incomplete")
	}
	if err := s.registerPortalResource(ctx, portalSubscriptionKind, created.ID, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: "/subscriptions", role: "developer"}); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, storeAs, created.ID); err != nil {
		return err
	}
	return tcontext.Set(ctx, "subscriptionToken", created.Token)
}

func (s *Steps) createAPIKey(ctx context.Context, apiID, storeAs string) error {
	return s.createAPIKeyAs(ctx, "publisher", apiID, storeAs)
}

func (s *Steps) createAPIKeyAs(ctx context.Context, role, apiID, storeAs string) error {
	apiID, err := stepscommon.Expand(ctx, apiID)
	if err != nil {
		return err
	}
	keyID, err := unique.Unique(ctx, "portal-key")
	if err != nil {
		return err
	}
	base, token, err := s.authedAs(ctx, role)
	if err != nil {
		return err
	}
	var created struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	response, err := s.doJSON(ctx, base, token, http.MethodPost, "/apis/"+apiID+"/api-keys/generate", map[string]any{"id": keyID})
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, httpx.ResponseKey, response); err != nil {
		return err
	}
	if !response.Succeeded() {
		return fmt.Errorf("POST API key generation failed: %s", response.Describe())
	}
	if err := decodeJSON(response, "/apis/"+apiID+"/api-keys/generate", &created); err != nil {
		return err
	}
	if created.ID == "" || created.Key == "" {
		return fmt.Errorf("API Portal API-key response was incomplete")
	}
	if err := s.registerPortalResource(ctx, portalAPIKeyKind, created.ID, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodPost, path: "/apis/" + apiID + "/api-keys/revoke",
			role:    role,
			payload: func(id string) any { return map[string]any{"keyId": id} }}); err != nil {
		return err
	}
	return tcontext.Set(ctx, storeAs, created.ID)
}

func (s *Steps) createMCPKey(ctx context.Context, mcpID, storeAs string) error {
	mcpID, err := stepscommon.Expand(ctx, mcpID)
	if err != nil {
		return err
	}
	keyID, err := unique.Unique(ctx, "portal-mcp-key")
	if err != nil {
		return err
	}
	base, token, err := s.authedAs(ctx, "publisher")
	if err != nil {
		return err
	}
	path := "/mcp-servers/" + mcpID + "/api-keys/generate"
	response, err := s.doJSON(ctx, base, token, http.MethodPost, path, map[string]any{"id": keyID})
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, httpx.ResponseKey, response); err != nil {
		return err
	}
	if !response.Succeeded() {
		return fmt.Errorf("POST MCP API key generation failed: %s", response.Describe())
	}
	var created struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := decodeJSON(response, path, &created); err != nil {
		return err
	}
	if created.ID == "" || created.Key == "" {
		return fmt.Errorf("API Portal MCP API-key response was incomplete")
	}
	if err := s.registerPortalResource(ctx, portalAPIKeyKind, created.ID, cleanup.ScenarioScope,
		portalCleanupRequest{
			method: http.MethodPost,
			path:   "/mcp-servers/" + mcpID + "/api-keys/revoke",
			payload: func(id string) any {
				return map[string]any{"keyId": id}
			},
		}); err != nil {
		return err
	}
	return tcontext.Set(ctx, storeAs, created.ID)
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func portalResourceKind(path string) cleanup.Kind {
	switch {
	case strings.Contains(path, "/api-workflows"):
		return portalWorkflowKind
	case strings.HasPrefix(path, "/labels"):
		return cleanup.Kind{Name: "api-portal-label", Order: 15}
	case strings.HasPrefix(path, "/views"):
		return cleanup.Kind{Name: "api-portal-view", Order: 25}
	case strings.HasPrefix(path, "/applications"):
		return portalApplicationKind
	case strings.HasPrefix(path, "/oauth2-keys"):
		return portalOAuth2KeyKind
	case strings.HasPrefix(path, "/key-managers"):
		return cleanup.Kind{Name: "api-portal-key-manager", Order: 35}
	case strings.HasPrefix(path, "/subscription-plans"):
		return portalSubscriptionPlanKind
	case strings.HasPrefix(path, "/subscriptions"):
		return portalSubscriptionKind
	case strings.HasPrefix(path, "/meters"):
		return portalMeterKind
	case strings.HasPrefix(path, "/webhook-subscribers"):
		return portalWebhookKind
	case strings.HasPrefix(path, "/mcp-servers"):
		return portalMCPServerKind
	default:
		return cleanup.Kind{Name: "api-portal-resource", Order: 40}
	}
}

// portalCleanupRole is the role that removes a resource created at path by role. Applications
// and OAuth2 keys are visible only to the user who created them, so they are deleted as that
// user; every other collection is deleted as the administrator.
func portalCleanupRole(path, role string) string {
	if strings.HasPrefix(path, "/applications") || strings.HasPrefix(path, "/oauth2-keys") {
		return role
	}
	return ""
}

func (s *Steps) sendPortal(ctx context.Context, method, path, role string, body []byte) error {
	return s.sendPortalWithHeaders(ctx, method, path, role, body, nil)
}

func (s *Steps) sendPortalWithHeaders(ctx context.Context, method, path, role string, body []byte, extra map[string]string) error {
	base, err := s.portalBase()
	if err != nil {
		return err
	}
	path, err = stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	headers := map[string]string{}
	if role != "" {
		_, token, err := s.authedAs(ctx, role)
		if err != nil {
			return err
		}
		headers["Authorization"] = "Bearer " + token
	}
	for name, value := range extra {
		headers[name] = value
	}
	request := httpx.Request{Method: strings.ToUpper(method), URL: s.portalRequestURL(base, path), Headers: headers, Body: body}
	if len(body) > 0 {
		request.ContentType = "application/json"
	}
	response, err := s.client.Do(ctx, request, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal %s %s: %w", method, path, err)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

// sendPortalMultipart sends metadata, and a definition part when one is given, through the
// portal funnel and publishes the response for the assertions that follow.
func (s *Steps) sendPortalMultipart(ctx context.Context, method, path, role string, metadata, definition []byte, filename, contentType string) error {
	base, err := s.portalBase()
	if err != nil {
		return err
	}
	path, err = stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	_, token, err := s.authedAs(ctx, role)
	if err != nil {
		return err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("metadata", string(metadata)); err != nil {
		return err
	}
	if len(definition) > 0 {
		if err := addPart(writer, "definition", filename, contentType, string(definition)); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	response, err := s.client.Do(ctx, httpx.Request{
		Method: strings.ToUpper(method), URL: s.portalRequestURL(base, path),
		Headers: map[string]string{"Authorization": "Bearer " + token},
		Body:    body.Bytes(), ContentType: writer.FormDataContentType(),
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal %s %s: %w", method, path, err)
	}
	return tcontext.Set(ctx, httpx.ResponseKey, response)
}

func (s *Steps) sendPortalMultipartAndDecode(ctx context.Context, base, token, method, path string,
	metadata, definition []byte, filename, contentType string, out any) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("metadata", string(metadata)); err != nil {
		return err
	}
	if err := addPart(writer, "definition", filename, contentType, string(definition)); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	response, err := s.client.Do(ctx, httpx.Request{
		Method: strings.ToUpper(method), URL: s.portalRequestURL(base, path),
		Headers: map[string]string{"Authorization": "Bearer " + token},
		Body:    body.Bytes(), ContentType: writer.FormDataContentType(),
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("API Portal %s %s: %w", method, path, err)
	}
	if !response.Succeeded() {
		return fmt.Errorf("API Portal %s %s failed: %s", method, path, response.Describe())
	}
	return decodeJSON(response, path, out)
}

// portalPublicURL addresses a portal route exactly as written: unlike portalRequestURL it never
// adds the Management API prefix, so a scenario can also reach a path outside /api-portal.
func portalPublicURL(base, path string) string {
	return base + path
}

func (s *Steps) portalRequestURL(base, path string) string {
	if path == "/health" || strings.HasPrefix(path, "/health?") ||
		path == "/llms.txt" || strings.HasPrefix(path, "/llms.txt?") ||
		path == "/api-portal" || strings.HasPrefix(path, "/api-portal/") {
		return base + path
	}
	return base + apiPrefix + path
}

// portalBase resolves api-portal's HTTPS base URL for this block.
func (s *Steps) portalBase() (string, error) {
	return s.portalBaseNamed(portalService)
}

func (s *Steps) portalBaseNamed(portal string) (string, error) {
	return s.topo.URL(strings.TrimSpace(portal), "https")
}

// devportalAPIHandle is api-portal's OWN identifier for a published API, distinct from
// platform-api's handle. api-portal's metadata.id and platform-api's handle are two different
// systems' own resource identifiers, cross-linked only through referenceId (see publishAPI) —
// deriving this deterministically from the platform-api handle lets every later step recompute
// it without needing separate context storage.
func devportalAPIHandle(platformAPIHandle string) string {
	return "dp-" + platformAPIHandle
}

// authed resolves api-portal's base URL alongside a fresh platform-api admin bearer token,
// since every call below needs both. Logs in fresh each time — no cached session state — the
// same stateless-per-call convention platformapi.Steps.bearer uses for its own bearer tokens.
func (s *Steps) authed(ctx context.Context) (base, token string, err error) {
	return s.authedAs(ctx, "admin")
}

func (s *Steps) authedAs(ctx context.Context, role string) (base, token string, err error) {
	base, err = s.portalBase()
	if err != nil {
		return "", "", err
	}
	cpBase, err := platformapisteps.BaseURL(s.topo)
	if err != nil {
		return "", "", err
	}
	credentials, err := portalActor(role)
	if err != nil {
		return "", "", err
	}
	token, err = controlplane.ControlPlaneLogin(ctx, cpBase, credentials.Username, credentials.Password)
	if err != nil {
		return "", "", fmt.Errorf("authenticating to platform-api for the API Portal bearer token: %w", err)
	}
	return base, token, nil
}

func portalActor(role string) (actor.Credentials, error) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return actor.Administrator(), nil
	case "publisher":
		return actor.Publisher(), nil
	case "developer":
		return actor.Developer(), nil
	case "narrow":
		return actor.Narrow(), nil
	default:
		return actor.Credentials{}, fmt.Errorf("unsupported API Portal actor %q", role)
	}
}

// getJSON issues one authenticated GET against api-portal's REST API.
func (s *Steps) getJSON(ctx context.Context, base, token, path string) (*httpx.Response, error) {
	resp, err := s.client.Do(ctx, httpx.Request{
		Method:  http.MethodGet,
		URL:     base + apiPrefix + path,
		Headers: map[string]string{"Authorization": "Bearer " + token},
	}, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	return resp, nil
}

// doJSON issues one authenticated JSON request against api-portal's REST API without asserting
// success, so callers that need to branch on status (e.g. a 409 on an idempotent create) can.
func (s *Steps) doJSON(ctx context.Context, base, token, method, path string, payload any) (*httpx.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method:      method,
		URL:         base + apiPrefix + path,
		Headers:     map[string]string{"Authorization": "Bearer " + token},
		Body:        body,
		ContentType: "application/json",
	}, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	return resp, nil
}

type portalCleanupRequest struct {
	method string
	path   string
	role   string
	// target, when set, is the path a DELETE is sent to instead of path/<id>.
	target  func(string) string
	payload func(string) any
}

// registerPortalResource records an API Portal resource and installs its deleter. If either
// registration operation fails after the resource exists, the resource is removed immediately
// so a failed scenario cannot leave an untracked portal artifact behind.
func (s *Steps) registerPortalResource(ctx context.Context, kind cleanup.Kind, id string, scope cleanup.Scope, request portalCleanupRequest) error {
	registry, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	actor := request.role
	if actor == "" {
		actor = "admin"
	}
	deleter := func(ctx context.Context, resource cleanup.Resource) error {
		// A kind keeps one deleter, so the role comes from the resource: resources of one kind
		// can be owned by different actors.
		role := resource.Actor
		if role == "" {
			role = actor
		}
		base, token, err := s.authedAs(ctx, role)
		if err != nil {
			return err
		}
		var response *httpx.Response
		if request.method == http.MethodDelete {
			// Escaped so a handle a scenario expects to be refused (one with a space or a slash)
			// still produces a valid clean-up request.
			target := request.path + "/" + url.PathEscape(resource.ID)
			if request.target != nil {
				target = request.target(resource.ID)
			}
			response, err = s.client.Do(ctx, httpx.Request{
				Method:  request.method,
				URL:     base + apiPrefix + target,
				Headers: map[string]string{"Authorization": "Bearer " + token},
			}, 0, 0)
		} else {
			response, err = s.doJSON(ctx, base, token, request.method, request.path, request.payload(resource.ID))
		}
		if err != nil {
			return err
		}
		if response.StatusCode == http.StatusNotFound ||
			(request.method == http.MethodPost && strings.HasSuffix(request.path, "/api-keys/revoke") &&
				response.StatusCode == http.StatusConflict) {
			return nil
		}
		if !response.Succeeded() {
			return fmt.Errorf("deleting API Portal resource %q failed: %s", resource.ID, response.Describe())
		}
		return nil
	}
	if err := registry.RegisterDeleter(kind, deleter); err != nil {
		return errors.Join(err, deleter(ctx, cleanup.Resource{Kind: kind, ID: id, Actor: actor}))
	}
	resource := cleanup.Resource{Kind: kind, ID: id, Actor: actor, Description: "created via API Portal"}
	if err := registry.RegisterScoped(scope, resource); err != nil {
		return errors.Join(err, deleter(ctx, resource))
	}
	return nil
}

func decodeJSON(resp *httpx.Response, path string, out any) error {
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}
	return nil
}

// postJSON issues one authenticated JSON POST against api-portal's REST API and asserts success.
func (s *Steps) postJSON(ctx context.Context, base, token, path string, payload, out any) error {
	resp, err := s.doJSON(ctx, base, token, http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return fmt.Errorf("POST %s failed: %s", path, resp.Describe())
	}
	return decodeJSON(resp, path, out)
}

// putJSON issues one authenticated JSON PUT against api-portal's REST API and asserts success.
func (s *Steps) putJSON(ctx context.Context, base, token, path string, payload, out any) error {
	resp, err := s.doJSON(ctx, base, token, http.MethodPut, path, payload)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return fmt.Errorf("PUT %s failed: %s", path, resp.Describe())
	}
	return decodeJSON(resp, path, out)
}

// postYAMLMultipart issues an authenticated multipart POST with a definition YAML part. When
// metadataFilename is non-empty, metadata is uploaded as a YAML file as required by the
// artifact-style API import path; an empty filename keeps the JSON metadata-field behavior used
// by the MCP fixture.
func (s *Steps) postYAMLMultipart(ctx context.Context, base, token, path, metadataYAML, definitionYAML string, out any, metadataFilename string) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if metadataFilename == "" {
		if err := writer.WriteField("metadata", metadataYAML); err != nil {
			return fmt.Errorf("building multipart request to %s: %w", path, err)
		}
	} else if err := addYAMLPart(writer, "metadata", metadataFilename, metadataYAML); err != nil {
		return fmt.Errorf("building multipart request to %s: %w", path, err)
	}
	if err := addYAMLPart(writer, "definition", "definition.yaml", definitionYAML); err != nil {
		return fmt.Errorf("building multipart request to %s: %w", path, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("building multipart request to %s: %w", path, err)
	}

	resp, err := s.client.Do(ctx, httpx.Request{
		Method:      http.MethodPost,
		URL:         base + apiPrefix + path,
		Headers:     map[string]string{"Authorization": "Bearer " + token},
		Body:        body.Bytes(),
		ContentType: writer.FormDataContentType(),
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	if !resp.Succeeded() {
		return fmt.Errorf("POST %s failed: %s", path, resp.Describe())
	}
	return decodeJSON(resp, path, out)
}

func (s *Steps) postJSONMultipart(ctx context.Context, base, token, path, metadataJSON, definitionJSON string, out any) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("metadata", metadataJSON)
	if err := addPart(writer, "definition", "definition.json", "application/json", definitionJSON); err != nil {
		return fmt.Errorf("building multipart request to %s: %w", path, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("building multipart request to %s: %w", path, err)
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodPost, URL: base + apiPrefix + path,
		Headers: map[string]string{"Authorization": "Bearer " + token},
		Body:    body.Bytes(), ContentType: writer.FormDataContentType(),
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	if !resp.Succeeded() {
		return fmt.Errorf("POST %s failed: %s", path, resp.Describe())
	}
	return decodeJSON(resp, path, out)
}

// addYAMLPart writes a named YAML file part into a multipart writer.
func addYAMLPart(mw *multipart.Writer, field, filename, content string) error {
	return addPart(mw, field, filename, "application/yaml", content)
}

func addPart(mw *multipart.Writer, field, filename, contentType, content string) error {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename))
	h.Set("Content-Type", contentType)
	part, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = part.Write([]byte(content))
	return err
}

// syncSubscriptionPlan upserts a plan in api-portal whose refId equals the platform-api plan
// handle, so subscription.created carries a resolvable subscription_plan.ref_id. PUT is a bulk
// upsert (an array body, even for one plan) — api-portal's own subscription-plan endpoint has
// no single-object POST/PUT pair. The quota mirrors the "10000 requests per hour" plan every
// devportal-webhook scenario creates on platform-api: keeping both sides' literal in sync is
// what api-portal's own onboarding flow does, not just what enforcement requires.
func (s *Steps) syncSubscriptionPlan(ctx context.Context, handle string) error {
	resolvedHandle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	refID, err := platformapisteps.SubscriptionPlanUUID(ctx, s.topo, s.controlPlane, resolvedHandle)
	if err != nil {
		return fmt.Errorf("resolving platform-api subscription plan %q: %w", resolvedHandle, err)
	}
	base, token, err := s.authed(ctx)
	if err != nil {
		return err
	}
	if err := s.putJSON(ctx, base, token, "/subscription-plans", []map[string]any{
		{
			"id":          resolvedHandle,
			"displayName": resolvedHandle,
			"refId":       refID,
			"limits": []map[string]any{
				{"limitType": "REQUEST_COUNT", "limitCount": 10000, "timeUnit": "HOUR", "timeAmount": 1},
			},
		},
	}, nil); err != nil {
		return err
	}
	return s.registerPortalResource(ctx, portalSubscriptionPlanKind, resolvedHandle, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: "/subscription-plans"})
}

// publishAPI creates api-portal's own metadata record for an API already deployed through
// platform-api. Its own handle (devportalAPIHandle) is deliberately DIFFERENT from platform-api's
// handle: they are two systems' own resource identifiers, cross-linked only through referenceId,
// which api-portal echoes back as data.api.ref_id on outgoing subscription.created/apikey.generated
// webhook events — the field platform-api's receiver actually resolves the artifact by.
func (s *Steps) publishAPI(ctx context.Context, handle, planHandles string) error {
	resolvedHandle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	// A comma-separated list, not just one plan: the credential-lifecycle scenario switches a
	// live subscription to a second plan, which the API must already offer at publish time.
	var plansYAML strings.Builder
	for _, planHandle := range strings.Split(planHandles, ",") {
		resolvedPlan, err := stepscommon.Expand(ctx, strings.TrimSpace(planHandle))
		if err != nil {
			return err
		}
		plansYAML.WriteString("\n    - " + resolvedPlan)
	}
	base, token, err := s.authed(ctx)
	if err != nil {
		return err
	}
	name := devportalAPIHandle(resolvedHandle)
	metadataYAML := fmt.Sprintf(`apiVersion: api-portal.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: %s
spec:
  type: REST
  displayName: %s
  version: v1
  status: PUBLISHED
  referenceId: %s
  subscriptionPlans:%s
  endpoints:
    sandboxUrl: https://backend.invalid/%s
    productionUrl: https://backend.invalid/%s
`, name, name, resolvedHandle, plansYAML.String(), resolvedHandle, resolvedHandle)

	// A throwaway definition, required by POST /apis but otherwise irrelevant to what this
	// scenario tests: authorization enforcement happens on the real gateway, against the API
	// platform-api already deployed there, not against anything api-portal's own record
	// describes.
	definitionYAML := fmt.Sprintf(`openapi: 3.0.3
info:
  title: %s
  version: "1.0.0"
paths:
  /ping:
    get:
      responses:
        '200':
          description: ok
`, name)

	if err := s.postYAMLMultipart(ctx, base, token, "/apis", metadataYAML, definitionYAML, nil, "metadata.yaml"); err != nil {
		return err
	}
	return s.registerPortalResource(ctx, portalAPIKind, name, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: "/apis"})
}

// subscribeToAPIAndStore creates a developer-owned application and subscribes it to the API's
// plan — one combined operation, matching api-portal's own single-application-per-subscription
// flow. artifactId is api-portal's OWN API handle, not the referenceId: subscriptions reference
// the artifact api-portal itself manages. The returned subscriptionToken (plaintext) is the
// gateway Subscription-Key value; creating the subscription fires subscription.created. The
// subscription id is also stored (rather than only the token) so a later credential-lifecycle
// operation — change-plan, regenerate-token, pause/resume, remove — can address this exact
// subscription; the smoke scenario simply never reads it back.
func (s *Steps) subscribeToAPIAndStore(ctx context.Context, apiHandle, planHandle, tokenStoreAs, subscriptionStoreAs string) error {
	resolvedAPI, err := stepscommon.Expand(ctx, apiHandle)
	if err != nil {
		return err
	}
	resolvedPlan, err := stepscommon.Expand(ctx, planHandle)
	if err != nil {
		return err
	}
	subscriptionID, subscriptionToken, err := s.subscribeToArtifact(ctx, devportalAPIHandle(resolvedAPI), resolvedAPI+"-app", resolvedPlan)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, tokenStoreAs, subscriptionToken); err != nil {
		return err
	}
	return tcontext.Set(ctx, subscriptionStoreAs, subscriptionID)
}

// subscribeToArtifact creates an application and subscribes it to the portal artifact with the
// plan, registers both for cleanup, and returns the subscription's id and token.
func (s *Steps) subscribeToArtifact(ctx context.Context, artifactID, appHandle, planHandle string) (subscriptionID, subscriptionToken string, err error) {
	base, token, err := s.authed(ctx)
	if err != nil {
		return "", "", err
	}
	if err := s.postJSON(ctx, base, token, "/applications", map[string]any{
		"id":          appHandle,
		"displayName": appHandle,
		"description": "IT fixture application",
	}, nil); err != nil {
		return "", "", err
	}
	if err := s.registerPortalResource(ctx, portalApplicationKind, appHandle, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: "/applications"}); err != nil {
		return "", "", err
	}

	var created struct {
		SubscriptionID    string `json:"subscriptionId"`
		SubscriptionToken string `json:"subscriptionToken"`
	}
	if err := s.postJSON(ctx, base, token, "/subscriptions", map[string]any{
		"artifactId":         artifactID,
		"subscriptionPlanId": planHandle,
	}, &created); err != nil {
		return "", "", err
	}
	if created.SubscriptionToken == "" || created.SubscriptionID == "" {
		return "", "", fmt.Errorf("api-portal returned an incomplete subscription for %q: %+v", artifactID, created)
	}
	if err := s.registerPortalResource(ctx, portalSubscriptionKind, created.SubscriptionID, cleanup.ScenarioScope,
		portalCleanupRequest{method: http.MethodDelete, path: "/subscriptions"}); err != nil {
		return "", "", err
	}
	return created.SubscriptionID, created.SubscriptionToken, nil
}

// generateAPIKeyAndStore generates an API key in the portal for an API and stores its plaintext
// value — the value the api-key-auth policy checks, presented as the API-Key header. The
// plaintext is returned exactly once, by this call, and never persisted by api-portal. The key
// is created with an explicit, framework-generated id (rather than letting api-portal mint one),
// and that handle is also stored, so a later lifecycle operation (revoke/regenerate) can address
// this exact key; the smoke scenario simply never reads it back. Reused as-is for "a new API key
// is generated" after a revoke: each call mints an entirely new key and handle.
func (s *Steps) generateAPIKeyAndStore(ctx context.Context, apiHandle, valueStoreAs, handleStoreAs string) error {
	resolvedAPI, err := stepscommon.Expand(ctx, apiHandle)
	if err != nil {
		return err
	}
	keyHandle, err := unique.Unique(ctx, "devportal-key")
	if err != nil {
		return err
	}
	value, storedHandle, err := s.generateKeyForArtifact(ctx, devportalAPIHandle(resolvedAPI), keyHandle)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, valueStoreAs, value); err != nil {
		return err
	}
	return tcontext.Set(ctx, handleStoreAs, storedHandle)
}

// generateKeyForArtifact generates an API key with the given id for the portal artifact and
// registers it to be revoked at the end of the scenario. It returns the key's value, shown only
// once, and its handle.
func (s *Steps) generateKeyForArtifact(ctx context.Context, artifactID, keyID string) (value, handle string, err error) {
	base, token, err := s.authed(ctx)
	if err != nil {
		return "", "", err
	}
	var created struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	}
	keysPath := "/apis/" + url.PathEscape(artifactID) + "/api-keys"
	if err := s.postJSON(ctx, base, token, keysPath+"/generate", map[string]any{
		"id":          keyID,
		"displayName": "IT fixture key",
	}, &created); err != nil {
		return "", "", err
	}
	if created.Key == "" {
		return "", "", fmt.Errorf("api-portal returned no API key for %q", artifactID)
	}
	handle = created.ID
	if handle == "" {
		handle = keyID
	}
	if err := s.registerPortalResource(ctx, portalAPIKeyKind, handle, cleanup.ScenarioScope,
		portalCleanupRequest{
			method:  http.MethodPost,
			path:    keysPath + "/revoke",
			payload: func(id string) any { return map[string]any{"keyId": id} },
		}); err != nil {
		return "", "", err
	}
	return created.Key, handle, nil
}

// generatePortalAPIKeyAndStore generates a key for an API created directly in the
// API Portal. Unlike the devportal webhook flow above, the API identifier is already
// the portal's own handle and must not receive the platform-api-to-portal "dp-" mapping.
func (s *Steps) generatePortalAPIKeyAndStore(ctx context.Context, apiID, valueStoreAs, handleStoreAs string) error {
	resolvedAPI, err := stepscommon.Expand(ctx, apiID)
	if err != nil {
		return err
	}
	keyID, err := unique.Unique(ctx, "portal-key")
	if err != nil {
		return err
	}
	base, token, err := s.authedAs(ctx, "publisher")
	if err != nil {
		return err
	}
	var created struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	}
	path := "/apis/" + resolvedAPI + "/api-keys/generate"
	if err := s.postJSON(ctx, base, token, path, map[string]any{"id": keyID}, &created); err != nil {
		return err
	}
	if created.Key == "" {
		return fmt.Errorf("api-portal returned no API key for API %q", resolvedAPI)
	}
	storedHandle := created.ID
	if storedHandle == "" {
		storedHandle = keyID
	}
	if err := s.registerPortalResource(ctx, portalAPIKeyKind, storedHandle, cleanup.ScenarioScope,
		portalCleanupRequest{
			method: http.MethodPost,
			path:   "/apis/" + resolvedAPI + "/api-keys/revoke",
			role:   "publisher",
			payload: func(id string) any {
				return map[string]any{"keyId": id}
			},
		}); err != nil {
		return err
	}
	if err := tcontext.Set(ctx, valueStoreAs, created.Key); err != nil {
		return err
	}
	return tcontext.Set(ctx, handleStoreAs, storedHandle)
}

// regenerateKeyExpiryAndStore regenerates an API key with a new expiry. Regenerate mints a new
// plaintext secret too (the previous one becomes invalid), so the stored key value must be
// overwritten with the response — used both to expire a key (a past expiresAt, which the
// gateway must then reject) and to restore it (a future expiresAt).
func (s *Steps) regenerateKeyExpiryAndStore(ctx context.Context, keyHandle, apiHandle, expiresAt, storeAs string) error {
	resolvedKey, err := stepscommon.Expand(ctx, keyHandle)
	if err != nil {
		return err
	}
	resolvedAPI, err := stepscommon.Expand(ctx, apiHandle)
	if err != nil {
		return err
	}
	base, token, err := s.authed(ctx)
	if err != nil {
		return err
	}
	var created struct {
		Key string `json:"key"`
	}
	path := "/apis/" + devportalAPIHandle(resolvedAPI) + "/api-keys/regenerate"
	if err := s.postJSON(ctx, base, token, path, map[string]any{
		"keyId":     resolvedKey,
		"expiresAt": expiresAt,
	}, &created); err != nil {
		return err
	}
	if created.Key == "" {
		return fmt.Errorf("api-portal returned no API key from regenerate for key %q", resolvedKey)
	}
	return tcontext.Set(ctx, storeAs, created.Key)
}

// revokeAPIKey revokes an API key; the gateway must then reject it with 401 while the
// subscription it is paired with stays active, isolating the rejection to the key.
func (s *Steps) revokeAPIKey(ctx context.Context, keyHandle, apiHandle string) error {
	resolvedKey, err := stepscommon.Expand(ctx, keyHandle)
	if err != nil {
		return err
	}
	resolvedAPI, err := stepscommon.Expand(ctx, apiHandle)
	if err != nil {
		return err
	}
	return s.revokeKeyOfArtifact(ctx, devportalAPIHandle(resolvedAPI), resolvedKey)
}

// revokeKeyOfArtifact revokes one API key of the portal artifact.
func (s *Steps) revokeKeyOfArtifact(ctx context.Context, artifactID, keyHandle string) error {
	base, token, err := s.authed(ctx)
	if err != nil {
		return err
	}
	return s.postJSON(ctx, base, token, "/apis/"+url.PathEscape(artifactID)+"/api-keys/revoke", map[string]any{"keyId": keyHandle}, nil)
}

// switchSubscriptionPlan switches a live subscription to a different plan the API already
// offers. The devportal field is planId, not subscriptionPlanId — a different name from the
// one POST /subscriptions itself accepts.
func (s *Steps) switchSubscriptionPlan(ctx context.Context, subscriptionID, planHandle string) error {
	resolvedSub, err := stepscommon.Expand(ctx, subscriptionID)
	if err != nil {
		return err
	}
	resolvedPlan, err := stepscommon.Expand(ctx, planHandle)
	if err != nil {
		return err
	}
	base, token, err := s.authed(ctx)
	if err != nil {
		return err
	}
	path := "/subscriptions/" + resolvedSub + "/change-plan"
	return s.postJSON(ctx, base, token, path, map[string]any{"planId": resolvedPlan}, nil)
}

// regenerateSubscriptionToken mints a new subscription token, invalidating the previous one. The
// previous value is stored alongside the new one so a scenario can assert that it is rejected.
func (s *Steps) regenerateSubscriptionToken(ctx context.Context, subscriptionID, prevStoreAs, newStoreAs string) error {
	resolvedSub, err := stepscommon.Expand(ctx, subscriptionID)
	if err != nil {
		return err
	}
	previousToken, err := tcontext.ResolveString(ctx, newStoreAs)
	if err != nil {
		return fmt.Errorf("resolving the current subscription token before regenerating it: %w", err)
	}
	base, token, err := s.authed(ctx)
	if err != nil {
		return err
	}
	var created struct {
		SubscriptionToken string `json:"subscriptionToken"`
	}
	path := "/subscriptions/" + resolvedSub + "/regenerate-token"
	if err := s.postJSON(ctx, base, token, path, map[string]any{}, &created); err != nil {
		return err
	}
	if created.SubscriptionToken == "" || created.SubscriptionToken == previousToken {
		return fmt.Errorf("api-portal did not return a new subscription token for subscription %q", resolvedSub)
	}
	if err := tcontext.Set(ctx, prevStoreAs, previousToken); err != nil {
		return err
	}
	return tcontext.Set(ctx, newStoreAs, created.SubscriptionToken)
}

// setSubscriptionStatus pauses (INACTIVE) or resumes (ACTIVE) a subscription; the gateway must
// reject a paused subscription's credentials with 403 while the API key itself stays valid.
func (s *Steps) setSubscriptionStatus(ctx context.Context, subscriptionID, status string) error {
	resolvedSub, err := stepscommon.Expand(ctx, subscriptionID)
	if err != nil {
		return err
	}
	base, token, err := s.authed(ctx)
	if err != nil {
		return err
	}
	return s.putJSON(ctx, base, token, "/subscriptions/"+resolvedSub, map[string]any{"status": status}, nil)
}

// removeSubscription deletes a subscription outright — a terminal state the gateway must reject
// with 403, distinct from a mere pause.
func (s *Steps) removeSubscription(ctx context.Context, subscriptionID string) error {
	resolvedSub, err := stepscommon.Expand(ctx, subscriptionID)
	if err != nil {
		return err
	}
	base, token, err := s.authed(ctx)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method:  http.MethodDelete,
		URL:     base + apiPrefix + "/subscriptions/" + resolvedSub,
		Headers: map[string]string{"Authorization": "Bearer " + token},
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("DELETE /subscriptions/%s: %w", resolvedSub, err)
	}
	if !resp.Succeeded() {
		return fmt.Errorf("DELETE /subscriptions/%s failed: %s", resolvedSub, resp.Describe())
	}
	if err := s.registerPortalResource(ctx, portalWebhookKind, webhookSubscriberID, cleanup.RunnerScope,
		portalCleanupRequest{method: http.MethodDelete, path: "/webhook-subscribers"}); err != nil {
		return err
	}
	return nil
}

// linkOrganizationToControlPlane sets api-portal's organization record's cpRefId to
// platform-api's org handle — a real, explicit link, distinct from both sides merely being
// configured with the SAME handle string ("default"). Without it, api-portal's outgoing
// webhook envelopes carry its OWN internal org UUID as org.ref_id (deliveryWorker.js's
// documented fallback: "cp_ref_id || org_uuid"), which platform-api's receiver can never
// resolve via GetOrganizationByHandle — every event is rejected with OrganizationNotFound
// before it even reaches event-specific processing.
func (s *Steps) linkOrganizationToControlPlane(ctx context.Context) error {
	base, token, err := s.authed(ctx)
	if err != nil {
		return err
	}
	// OrganizationUpdateRequest is a full replace (displayName/id/idpRefId all required, per
	// api-portal's own OpenAPI spec) — read the current record first so this only actually
	// changes cpRefId, not the identity fields a partial-looking PUT would otherwise blank out.
	resp, err := s.getJSON(ctx, base, token, "/organizations/"+orgHandle)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return fmt.Errorf("GET /organizations/%s failed: %s", orgHandle, resp.Describe())
	}
	var current struct {
		DisplayName string `json:"displayName"`
		ID          string `json:"id"`
		IdpRefID    string `json:"idpRefId"`
	}
	if err := json.Unmarshal(resp.Body, &current); err != nil {
		return fmt.Errorf("decoding organization %q: %w", orgHandle, err)
	}
	displayName := current.DisplayName
	if displayName == "" {
		displayName = "Default"
	}
	return s.putJSON(ctx, base, token, "/organizations/"+orgHandle, map[string]any{
		"displayName": displayName,
		"id":          current.ID,
		"idpRefId":    current.IdpRefID,
		"cpRefId":     orgHandle,
	}, nil)
}

// registerWebhookSubscriber points api-portal's outgoing webhooks at platform-api's receiver,
// with the same secret platform-api's [platform_api.webhook] config expects — see
// core/catalog/overlays/platform-api-portal-publishing.toml. Without this, api-portal never tells
// platform-api about anything created above; the webhook receiver, though enabled, has nothing
// subscribed to deliver to it.
//
// The Background step that calls this runs once per scenario, and every scenario in this
// runner shares the SAME fixed subscriber id, so a repeat registration must be idempotent: a
// 409 Conflict from an earlier scenario's registration is resynced via PUT rather than treated
// as fatal, matching the same subscriber events allowlist and secret on every call.
//
// events is a comma-separated list of event patterns; when empty, the subscriber receives the
// API key and subscription events. A pattern ending in ".*" matches by prefix, so
// "subscription.*" does not match the subscription_plan events.
func (s *Steps) registerWebhookSubscriber(ctx context.Context, events string) error {
	patterns := []string{"apikey.*", "subscription.*"}
	if strings.TrimSpace(events) != "" {
		patterns = nil
		for _, pattern := range strings.Split(events, ",") {
			if pattern = strings.TrimSpace(pattern); pattern != "" {
				patterns = append(patterns, pattern)
			}
		}
	}
	if err := s.linkOrganizationToControlPlane(ctx); err != nil {
		return err
	}
	// api-portal's OWN webhook delivery worker dials this URL from INSIDE its own container,
	// on the block's docker network — not from the test process, which is why this needs
	// platform-api's internal (alias-based) address rather than the host-mapped one every
	// other call in this package uses.
	controlPlaneBase, err := platformapisteps.InternalBaseURL(s.topo)
	if err != nil {
		return err
	}
	base, token, err := s.authed(ctx)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"id":          webhookSubscriberID,
		"displayName": "Platform API",
		"targetUrl":   controlPlaneBase + "/api/internal/v0.9/webhook/events",
		"secret":      webhookSecret(),
		"events":      patterns,
		"enabled":     true,
	}
	resp, err := s.doJSON(ctx, base, token, http.MethodPost, "/webhook-subscribers", payload)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusConflict {
		resp, err = s.doJSON(ctx, base, token, http.MethodPut, "/webhook-subscribers/"+webhookSubscriberID, payload)
		if err != nil {
			return err
		}
	}
	if !resp.Succeeded() {
		return fmt.Errorf("register webhook subscriber failed: %s", resp.Describe())
	}
	return nil
}

// webhookSecret is the HMAC secret shared between platform-api's webhook receiver and this
// subscriber registration. A fixed test literal, not a generated one — it protects nothing
// beyond proving the product's own signature verification works with a correctly matched
// secret on both sides, exactly like api-portal's own hardcoded encryption_key/session_secret
// in core/catalog/overlays/api-portal-storage.toml.
var webhookSecret = controlplane.WebhookSecret

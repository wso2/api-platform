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

package platformapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// deployPaths maps a gateway artifact kind to its control-plane resource collection, for
// creation and deployment - the same collections artifactPath resolves for DP -> CP
// assertions, since both directions address the same platform-api resources.
var deployPaths = artifactPaths

// apiKeyHeader and subscriptionKeyHeader are the default header names the api-key-auth and
// subscription-validation policies check, matched exactly by createSecuredRestAPI's params.
const (
	apiKeyHeader          = "API-Key"
	subscriptionKeyHeader = "Subscription-Key"
)

// RegisterDeploy binds the steps that create and deploy an artifact THROUGH platform-api's
// own REST API, as opposed to control_plane.go's steps, which only ever assert against
// artifacts a gateway pushed up. Requests go through the same shared client every other
// suite request funnels through.
func RegisterDeploy(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^I create a secret "([^"]*)" via the control plane$`, s.createSecret)
	sc.Step(`^I create an LLM provider "([^"]*)" via the control plane referencing template "([^"]*)"$`,
		func(ctx context.Context, id, template string) error {
			return s.createLLMProvider(ctx, id, template, "")
		})
	sc.Step(`^I create an LLM provider "([^"]*)" via the control plane referencing template "([^"]*)" and secret "([^"]*)"$`,
		s.createLLMProvider)
	sc.Step(`^I create an LLM proxy "([^"]*)" via the control plane in project "([^"]*)" referencing provider "([^"]*)" and secret "([^"]*)"$`,
		s.createLLMProxy)
	sc.Step(`^I create an MCP proxy "([^"]*)" via the control plane referencing secret "([^"]*)"$`,
		s.createMCPProxy)
	sc.Step(`^I create a REST API "([^"]*)" via the control plane in project "([^"]*)" with context "([^"]*)" and an upstream auth secret "([^"]*)"$`,
		s.createRestAPIUpstreamSecret)
	sc.Step(`^I create a REST API "([^"]*)" via the control plane in project "([^"]*)" with context "([^"]*)" and a policy header secret "([^"]*)"$`,
		s.createRestAPIPolicySecret)
	sc.Step(`^I create a REST API "([^"]*)" via the control plane in project "([^"]*)" with context "([^"]*)"$`,
		s.createRestAPIPlain)
	sc.Step(`^I deploy the "([^"]*)" "([^"]*)" to the gateway via the control plane$`, s.deployArtifact)
	sc.Step(`^I deploy the "([^"]*)" "([^"]*)" to the gateway via the control plane and store the deployment id as "([^"]*)"$`,
		s.deployArtifactAndStore)
	sc.Step(`^I undeploy the "RestApi" "([^"]*)" deployment "([^"]*)" from the gateway via the control plane$`,
		s.undeployRestAPI)
	sc.Step(`^I create a subscription plan "([^"]*)" allowing (\d+) requests per (minute|hour|day|month) via the control plane$`,
		s.createSubscriptionPlan)
	sc.Step(`^I create a secured REST API "([^"]*)" via the control plane in project "([^"]*)" with context "([^"]*)" offering plan "([^"]*)"$`,
		s.createSecuredRestAPI)
	sc.Step(`^I create an application "([^"]*)" via the control plane in project "([^"]*)"$`,
		s.createApplication)
	sc.Step(`^I create a subscription for REST API "([^"]*)" application "([^"]*)" plan "([^"]*)" via the control plane and store the subscription token as "([^"]*)"$`,
		s.createSubscription)
	sc.Step(`^I issue an API key "([^"]*)" for REST API "([^"]*)" via the control plane$`,
		s.issueAPIKey)
}

// authed resolves the control plane's base URL and a fresh admin bearer token together,
// since every call below needs both.
func (s *Steps) authed(ctx context.Context) (base, bearer string, err error) {
	base, err = s.baseURL()
	if err != nil {
		return "", "", err
	}
	bearer, err = s.bearer(ctx, base)
	if err != nil {
		return "", "", fmt.Errorf("authenticating to the control plane: %w", err)
	}
	return base, bearer, nil
}

// postJSON issues one authenticated JSON POST against the control plane and decodes the
// response into out when the call succeeds. out may be nil when the response body is not
// needed.
func (s *Steps) postJSON(ctx context.Context, base, bearer, path string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method:      http.MethodPost,
		URL:         base + apiBase + path,
		Headers:     map[string]string{"Authorization": "Bearer " + bearer},
		Body:        body,
		ContentType: "application/json",
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	if !resp.Succeeded() {
		return fmt.Errorf("POST %s failed: %s", path, resp.Describe())
	}
	if out != nil {
		if err := json.Unmarshal(resp.Body, out); err != nil {
			return fmt.Errorf("decoding response from POST %s: %w", path, err)
		}
	}
	return nil
}

// createSecret creates a GENERIC secret in platform-api under the given handle. The value
// and display name are arbitrary - only the handle is asserted on downstream.
func (s *Steps) createSecret(ctx context.Context, handle string) error {
	resolvedHandle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}

	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	for _, kv := range [][2]string{
		{"id", resolvedHandle},
		{"displayName", resolvedHandle},
		{"value", "test-value-" + resolvedHandle},
		{"type", "GENERIC"},
	} {
		if err := mw.WriteField(kv[0], kv[1]); err != nil {
			return err
		}
	}
	if err := mw.Close(); err != nil {
		return err
	}

	resp, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodPost,
		URL:    base + apiBase + "/secrets",
		Headers: map[string]string{
			"Authorization": "Bearer " + bearer,
			"Content-Type":  mw.FormDataContentType(),
		},
		Body: buf.Bytes(),
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("creating control-plane secret %q: %w", resolvedHandle, err)
	}
	if !resp.Succeeded() {
		return fmt.Errorf("creating control-plane secret %q: %s", resolvedHandle, resp.Describe())
	}
	return nil
}

// secretPlaceholder is the template literal the gateway controller resolves on demand at
// deploy time by fetching the named secret's value.
func secretPlaceholder(handle string) string {
	return `{{ secret "` + handle + `" }}`
}

// createLLMProvider creates an LLM provider via platform-api. secretHandle may be empty, for
// a plain provider (e.g. the base provider an LLM proxy references).
func (s *Steps) createLLMProvider(ctx context.Context, id, template, secretHandle string) error {
	resolvedID, err := stepscommon.Expand(ctx, id)
	if err != nil {
		return err
	}
	resolvedTemplate, err := stepscommon.Expand(ctx, template)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}

	upstream := map[string]any{"url": "http://testbench:3000"}
	if secretHandle != "" {
		resolvedSecret, err := stepscommon.Expand(ctx, secretHandle)
		if err != nil {
			return err
		}
		upstream["auth"] = map[string]any{
			"type":   "api-key",
			"header": "Authorization",
			"value":  secretPlaceholder(resolvedSecret),
		}
	}

	return s.postJSON(ctx, base, bearer, "/llm-providers", map[string]any{
		"id":          resolvedID,
		"displayName": resolvedID,
		"version":     "v1.0",
		"template":    resolvedTemplate,
		"upstream":    map[string]any{"main": upstream},
		"accessControl": map[string]any{
			"mode": "allow_all",
		},
	}, nil)
}

// createLLMProxy creates an LLM proxy referencing an already-deployed provider by id, with
// its auth override set to a secret placeholder.
func (s *Steps) createLLMProxy(ctx context.Context, id, projectHandle, providerID, secretHandle string) error {
	resolvedID, err := stepscommon.Expand(ctx, id)
	if err != nil {
		return err
	}
	resolvedProject, err := stepscommon.Expand(ctx, projectHandle)
	if err != nil {
		return err
	}
	resolvedProvider, err := stepscommon.Expand(ctx, providerID)
	if err != nil {
		return err
	}
	resolvedSecret, err := stepscommon.Expand(ctx, secretHandle)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}

	return s.postJSON(ctx, base, bearer, "/llm-proxies", map[string]any{
		"id":          resolvedID,
		"displayName": resolvedID,
		"version":     "v1.0",
		"projectId":   resolvedProject,
		"provider": map[string]any{
			"id": resolvedProvider,
			"auth": map[string]any{
				"type":   "api-key",
				"header": "Authorization",
				"value":  secretPlaceholder(resolvedSecret),
			},
		},
	}, nil)
}

// createMCPProxy creates an MCP proxy whose upstream auth value embeds a secret placeholder.
func (s *Steps) createMCPProxy(ctx context.Context, id, secretHandle string) error {
	resolvedID, err := stepscommon.Expand(ctx, id)
	if err != nil {
		return err
	}
	resolvedSecret, err := stepscommon.Expand(ctx, secretHandle)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}

	return s.postJSON(ctx, base, bearer, "/mcp-proxies", map[string]any{
		"id":             resolvedID,
		"displayName":    resolvedID,
		"version":        "v1.0",
		"mcpSpecVersion": "2025-06-18",
		"upstream": map[string]any{
			"main": map[string]any{
				"url": "http://testbench:3009",
				"auth": map[string]any{
					"type":   "api-key",
					"header": "Authorization",
					"value":  secretPlaceholder(resolvedSecret),
				},
			},
		},
	}, nil)
}

// createRestAPIUpstreamSecret creates a REST API whose upstream auth value embeds a secret
// placeholder.
func (s *Steps) createRestAPIUpstreamSecret(ctx context.Context, id, projectHandle, apiContext, secretHandle string) error {
	resolvedSecret, err := stepscommon.Expand(ctx, secretHandle)
	if err != nil {
		return err
	}
	return s.createRestAPIWithSecret(ctx, id, projectHandle, apiContext, nil, map[string]any{
		"url": "http://testbench:3000",
		"auth": map[string]any{
			"type":   "api-key",
			"header": "Authorization",
			"value":  secretPlaceholder(resolvedSecret),
		},
	})
}

// createRestAPIPolicySecret creates a REST API with one operation carrying a set-headers
// policy whose header value embeds a secret placeholder, nested under
// operations[].request.policies[].params rather than the upstream auth block.
func (s *Steps) createRestAPIPolicySecret(ctx context.Context, id, projectHandle, apiContext, secretHandle string) error {
	resolvedSecret, err := stepscommon.Expand(ctx, secretHandle)
	if err != nil {
		return err
	}
	operations := []map[string]any{
		{
			"request": map[string]any{
				"method": "GET",
				"path":   "/",
				"policies": []map[string]any{
					{
						"name":    "set-headers",
						"version": "v1",
						"params": map[string]any{
							"request": map[string]any{
								"headers": []map[string]any{
									{"name": "X-Api-Token", "value": secretPlaceholder(resolvedSecret)},
								},
							},
						},
					},
				},
			},
		},
	}
	return s.createRestAPIWithSecret(ctx, id, projectHandle, apiContext, operations, map[string]any{
		"url": "http://testbench:3000",
	})
}

// createRestAPIPlain creates a plain REST API via platform-api, with one GET /health
// operation routed to testbench - no secret involved.
func (s *Steps) createRestAPIPlain(ctx context.Context, id, projectHandle, apiContext string) error {
	operations := []map[string]any{
		{"request": map[string]any{"method": "GET", "path": "/health"}},
	}
	return s.createRestAPIWithSecret(ctx, id, projectHandle, apiContext, operations, map[string]any{
		"url": "http://testbench:3000",
	})
}

// createRestAPIWithSecret creates a REST API via platform-api with the given upstream block
// and, optionally, operations. Despite the name, this is the general REST API creation path -
// createRestAPIPlain also uses it, with no secret placeholder anywhere in its payload.
func (s *Steps) createRestAPIWithSecret(
	ctx context.Context, id, projectHandle, apiContext string, operations []map[string]any, upstreamMain map[string]any,
) error {
	resolvedID, err := stepscommon.Expand(ctx, id)
	if err != nil {
		return err
	}
	resolvedProject, err := stepscommon.Expand(ctx, projectHandle)
	if err != nil {
		return err
	}
	resolvedContext, err := stepscommon.Expand(ctx, apiContext)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"displayName": resolvedID,
		"context":     resolvedContext,
		"version":     "v1",
		"projectId":   resolvedProject,
		"upstream":    map[string]any{"main": upstreamMain},
	}
	if operations != nil {
		payload["operations"] = operations
	}

	var created struct {
		ID     string `json:"id"`
		Handle string `json:"handle"`
		UUID   string `json:"uuid"`
	}
	if err := s.postJSON(ctx, base, bearer, "/rest-apis", payload, &created); err != nil {
		return err
	}
	handle := created.Handle
	if handle == "" {
		handle = created.ID
	}
	if handle == "" {
		handle = created.UUID
	}
	if handle != resolvedID && handle != "" {
		return fmt.Errorf("control plane assigned REST API handle %q, expected %q — a later deploy/verify step would address the wrong resource", handle, resolvedID)
	}
	return nil
}

// gatewayUUID discovers the block's single registered gateway's control-plane identifier -
// "id" in the list response (which, for this fixed-handle registration, is literally
// "it-gateway"; see core/catalog/platformapi's provisionGatewayRegistration), falling back to
// "uuid" in case a future control-plane version reports a distinct server-assigned one there
// instead.
func (s *Steps) gatewayUUID(ctx context.Context, base, bearer string) (string, error) {
	resp, err := s.get(ctx, base, bearer, "/gateways")
	if err != nil {
		return "", fmt.Errorf("listing control-plane gateways: %w", err)
	}
	if !resp.Succeeded() {
		return "", fmt.Errorf("listing control-plane gateways: %s", resp.Describe())
	}
	var doc struct {
		List []struct {
			ID   string `json:"id"`
			UUID string `json:"uuid"`
		} `json:"list"`
	}
	if err := json.Unmarshal(resp.Body, &doc); err != nil {
		return "", fmt.Errorf("decoding control-plane gateway list: %w", err)
	}
	if len(doc.List) == 0 {
		return "", fmt.Errorf("control plane reports no registered gateway: %s", resp.Text())
	}
	if doc.List[0].ID != "" {
		return doc.List[0].ID, nil
	}
	if doc.List[0].UUID != "" {
		return doc.List[0].UUID, nil
	}
	return "", fmt.Errorf("control plane's registered gateway has no id or uuid: %s", resp.Text())
}

// deployArtifact deploys an already-created artifact to the block's gateway, discarding the
// deployment id. A REST API must first be attached to the gateway via a separate call; every
// other kind deploys directly.
func (s *Steps) deployArtifact(ctx context.Context, kind, handle string) error {
	_, err := s.deployArtifactID(ctx, kind, handle)
	return err
}

// deployArtifactAndStore deploys an artifact and stores the resulting deployment id, for a
// later undeploy call to reference.
func (s *Steps) deployArtifactAndStore(ctx context.Context, kind, handle, storeAs string) error {
	id, err := s.deployArtifactID(ctx, kind, handle)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, storeAs, id)
}

func (s *Steps) deployArtifactID(ctx context.Context, kind, handle string) (string, error) {
	resolvedKind, err := stepscommon.Expand(ctx, kind)
	if err != nil {
		return "", err
	}
	resolvedHandle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return "", err
	}
	collection, ok := deployPaths[resolvedKind]
	if !ok {
		return "", fmt.Errorf("unknown artifact kind %q for control-plane deployment", resolvedKind)
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return "", err
	}
	gatewayID, err := s.gatewayUUID(ctx, base, bearer)
	if err != nil {
		return "", err
	}

	if resolvedKind == "RestApi" {
		// The attach endpoint takes a JSON array of gateway references, not a bare object. A
		// redeploy attaches the same gateway again - the control plane treats this as a no-op
		// rather than a conflict, since re-attaching what is already attached carries no new
		// information.
		if err := s.postJSON(ctx, base, bearer, "/"+collection+"/"+resolvedHandle+"/gateways",
			[]map[string]string{{"gatewayId": gatewayID}}, nil); err != nil {
			return "", fmt.Errorf("attaching gateway to REST API %q: %w", resolvedHandle, err)
		}
	}

	deploymentName, err := unique.Unique(ctx, "dep")
	if err != nil {
		return "", err
	}
	var deployed struct {
		DeploymentID string `json:"deploymentId"`
	}
	if err := s.postJSON(ctx, base, bearer, "/"+collection+"/"+resolvedHandle+"/deployments", map[string]any{
		"name":      deploymentName,
		"base":      "current",
		"gatewayId": gatewayID,
	}, &deployed); err != nil {
		return "", fmt.Errorf("deploying %s %q: %w", resolvedKind, resolvedHandle, err)
	}
	if deployed.DeploymentID == "" {
		return "", fmt.Errorf("deploying %s %q: control plane returned no deployment id", resolvedKind, resolvedHandle)
	}
	return deployed.DeploymentID, nil
}

// undeployRestAPI undeploys a REST API's deployment from the block's gateway.
func (s *Steps) undeployRestAPI(ctx context.Context, handle, deploymentID string) error {
	resolvedHandle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	resolvedDeploymentID, err := stepscommon.Expand(ctx, deploymentID)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}
	gatewayID, err := s.gatewayUUID(ctx, base, bearer)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(ctx, httpx.Request{
		Method:  http.MethodPost,
		URL:     base + apiBase + "/rest-apis/" + resolvedHandle + "/deployments/" + resolvedDeploymentID + "/undeploy?gatewayId=" + gatewayID,
		Headers: map[string]string{"Authorization": "Bearer " + bearer},
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("undeploying REST API %q: %w", resolvedHandle, err)
	}
	if !resp.Succeeded() {
		return fmt.Errorf("undeploying REST API %q: %s", resolvedHandle, resp.Describe())
	}
	return nil
}

// createSubscriptionPlan creates an ACTIVE, org-scoped subscription plan under the given
// handle, with one REQUEST_COUNT limit.
func (s *Steps) createSubscriptionPlan(ctx context.Context, handle string, count int, unit string) error {
	resolvedHandle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}
	return s.postJSON(ctx, base, bearer, "/subscription-plans", map[string]any{
		"id":          resolvedHandle,
		"displayName": resolvedHandle,
		"status":      "ACTIVE",
		"limits": []map[string]any{
			{"limitType": "REQUEST_COUNT", "timeUnit": strings.ToUpper(unit), "limitCount": count},
		},
	}, nil)
}

// createSecuredRestAPI creates a PUBLISHED REST API offering the given subscription plan,
// guarded by api-key-auth and subscription-validation - both attached at API level, so they
// apply to every path under the API's context regardless of any operation-level policy.
func (s *Steps) createSecuredRestAPI(ctx context.Context, id, projectHandle, apiContext, planHandle string) error {
	resolvedID, err := stepscommon.Expand(ctx, id)
	if err != nil {
		return err
	}
	resolvedProject, err := stepscommon.Expand(ctx, projectHandle)
	if err != nil {
		return err
	}
	resolvedContext, err := stepscommon.Expand(ctx, apiContext)
	if err != nil {
		return err
	}
	resolvedPlan, err := stepscommon.Expand(ctx, planHandle)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}

	return s.postJSON(ctx, base, bearer, "/rest-apis", map[string]any{
		"displayName":       resolvedID,
		"context":           resolvedContext,
		"version":           "v1",
		"projectId":         resolvedProject,
		"lifeCycleStatus":   "PUBLISHED",
		"subscriptionPlans": []string{resolvedPlan},
		"upstream":          map[string]any{"main": map[string]any{"url": "http://testbench:3000"}},
		"policies": []map[string]any{
			{"name": "api-key-auth", "version": "v1", "params": map[string]any{"key": apiKeyHeader, "in": "header"}},
			{"name": "subscription-validation", "version": "v1", "params": map[string]any{"subscriptionKeyHeader": subscriptionKeyHeader}},
		},
	}, nil)
}

// createApplication creates a GenAI application under the given project.
func (s *Steps) createApplication(ctx context.Context, id, projectHandle string) error {
	resolvedID, err := stepscommon.Expand(ctx, id)
	if err != nil {
		return err
	}
	resolvedProject, err := stepscommon.Expand(ctx, projectHandle)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}
	return s.postJSON(ctx, base, bearer, "/applications", map[string]any{
		"id":          resolvedID,
		"displayName": resolvedID,
		"projectId":   resolvedProject,
		"type":        "genai",
	}, nil)
}

// createSubscription subscribes an application to a REST API under a plan, storing the
// minted subscription token - the raw value the Subscription-Key header must carry, returned
// only on creation.
func (s *Steps) createSubscription(ctx context.Context, apiHandle, appHandle, planHandle, storeAs string) error {
	resolvedAPI, err := stepscommon.Expand(ctx, apiHandle)
	if err != nil {
		return err
	}
	resolvedApp, err := stepscommon.Expand(ctx, appHandle)
	if err != nil {
		return err
	}
	resolvedPlan, err := stepscommon.Expand(ctx, planHandle)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}

	var created struct {
		SubscriptionToken string `json:"subscriptionToken"`
	}
	if err := s.postJSON(ctx, base, bearer, "/subscriptions", map[string]any{
		"artifactId":         resolvedAPI,
		"kind":               "RestApi",
		"subscriberId":       "e2e-subscriber",
		"applicationId":      resolvedApp,
		"subscriptionPlanId": resolvedPlan,
	}, &created); err != nil {
		return err
	}
	if created.SubscriptionToken == "" {
		return fmt.Errorf("control plane returned no subscription token for API %q", resolvedAPI)
	}
	return tcontext.Set(ctx, storeAs, created.SubscriptionToken)
}

// issueAPIKey supplies the plaintext key value to platform-api, which hashes and broadcasts
// it to the gateways where the API is deployed. The response never carries the secret back,
// so the caller must retain the value it supplied - keyValue is expected to already be a
// generated, unique context value the invoking step reuses as the API-Key header.
func (s *Steps) issueAPIKey(ctx context.Context, keyValue, apiHandle string) error {
	resolvedKey, err := stepscommon.Expand(ctx, keyValue)
	if err != nil {
		return err
	}
	resolvedAPI, err := stepscommon.Expand(ctx, apiHandle)
	if err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}
	return s.postJSON(ctx, base, bearer, "/rest-apis/"+resolvedAPI+"/api-keys", map[string]any{
		"displayName": "key-" + resolvedAPI,
		"apiKey":      resolvedKey,
	}, nil)
}

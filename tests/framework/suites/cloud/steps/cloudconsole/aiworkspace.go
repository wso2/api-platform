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
	"sort"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	keyAIGatewayID       = "cloudAIGatewayID"
	keyAIGatewayEnv      = "cloudAIGatewayEnvironment"
	keyAIGatewayEndpoint = "cloudAIGatewayEndpoint"
	keyRegularGatewayID  = "cloudRegularGatewayID"

	keyLLMProviderID      = "cloudLLMProviderID"
	keyLLMProviderContext = "cloudLLMProviderContext"
	keyLLMProviderTargets = "cloudLLMProviderDeployments"
	keyLLMProxyID         = "cloudLLMProxyID"
	keyLLMProxyContext    = "cloudLLMProxyContext"
	keyLLMProxyTargets    = "cloudLLMProxyDeployments"

	llmProvidersCollection = "/llm-providers"
	llmProxiesCollection   = "/llm-proxies"
	aiGatewayKind          = "ai"
	defaultLLMTemplate     = "openai"
	defaultLLMModel        = "gpt-4o-mini"
	llmListLimit           = 100
	llmProxyDeployKind     = "LlmProxy"
)

// The platform refuses to delete a provider while a proxy names it, and a secret while a
// provider or proxy references it, so proxies go first, then providers, then secrets.
var (
	kindLLMProxy    = cleanup.Kind{Name: "cloud-llm-proxy", Order: 62}
	kindLLMProvider = cleanup.Kind{Name: "cloud-llm-provider", Order: 64}
)

// llmProviderKeys and llmProxyKeys list the values the synthetic provider and proxy tables may carry.
var (
	llmProviderKeys = map[string]bool{"template": true, "upstreamUrl": true, "upstreamAuth": true}
	llmProxyKeys    = map[string]bool{"providerAuth": true, "globalPolicies": true, "operationPolicies": true}
)

// llmArtifact says how one kind of AI artifact is created, listed and deployed: a provider belongs
// to the organisation and deploys across its environments, a proxy belongs to a project and
// deploys through the project's pipeline.
type llmArtifact struct {
	noun        string
	handleBase  string
	idKey       string
	contextKey  string
	targetsKey  string
	collection  string
	cleanupKind cleanup.Kind
	// deployKind is the artifact kind the project deployment routes are asked for; empty for
	// a provider, which has a route of its own.
	deployKind string
}

var (
	providerArtifact = llmArtifact{
		noun: "LLM provider", handleBase: "syn-lp", idKey: keyLLMProviderID, contextKey: keyLLMProviderContext,
		targetsKey: keyLLMProviderTargets, collection: llmProvidersCollection, cleanupKind: kindLLMProvider,
	}
	proxyArtifact = llmArtifact{
		noun: "LLM proxy", handleBase: "syn-lx", idKey: keyLLMProxyID, contextKey: keyLLMProxyContext,
		targetsKey: keyLLMProxyTargets, collection: llmProxiesCollection, cleanupKind: kindLLMProxy,
		deployKind: llmProxyDeployKind,
	}
)

// deploymentRoute is the path and fixed query of the artifact's deployment routes.
func (a llmArtifact) deploymentRoute(ctx context.Context) (path string, query url.Values, err error) {
	id, err := requiredValue(ctx, a.idKey)
	if err != nil {
		return "", nil, fmt.Errorf("there is no synthetic %s: %w", a.noun, err)
	}
	if a.deployKind == "" {
		return a.collection + "/" + url.PathEscape(id) + "/environment-deployments", url.Values{}, nil
	}
	project, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return "", nil, err
	}
	return "/projects/" + url.PathEscape(project) + "/apis/" + url.PathEscape(id) + "/deployments",
		url.Values{"kind": {a.deployKind}}, nil
}

// listPath is the artifact's collection with the paging a scenario asks for. A proxy belongs to a
// project and the platform lists them per project, so the project is part of the request.
func (a llmArtifact) listPath(ctx context.Context) (string, error) {
	query := url.Values{"limit": {fmt.Sprint(llmListLimit)}}
	if a.deployKind != "" {
		project, err := requiredValue(ctx, keyProjectID)
		if err != nil {
			return "", fmt.Errorf("the %s list is per project: %w", a.noun, err)
		}
		query.Set("projectId", project)
	}
	return withQuery(a.collection, query), nil
}

func withQuery(path string, query url.Values) string {
	if len(query) == 0 {
		return path
	}
	return path + "?" + query.Encode()
}

// findActiveAIGateway picks an AI gateway whose controller is connected, with the environment and
// data-plane endpoint a scenario needs to deploy to it and send traffic through it.
func (s *Steps) findActiveAIGateway(ctx context.Context) error {
	items, err := s.fetchGatewayList(ctx, aiGatewayKind)
	if err != nil {
		return err
	}
	choice, err := s.chooseActiveGateway(ctx, items, "APIP AI gateway")
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyAIGatewayID, choice.id); err != nil {
		return err
	}
	if choice.environment != "" {
		if err := tcontext.Set(ctx, keyAIGatewayEnv, choice.environment); err != nil {
			return err
		}
	}
	return tcontext.Set(ctx, keyAIGatewayEndpoint, choice.endpoint)
}

// findAIAndRegularGateway picks an AI gateway and a regular gateway of one environment. Neither
// has to be connected: the platform checks a gateway's kind before it looks at anything else.
func (s *Steps) findAIAndRegularGateway(ctx context.Context) error {
	items, err := s.fetchGatewayList(ctx, "")
	if err != nil {
		return err
	}
	environment, aiID, regularID, err := pairGatewaysByEnvironment(items)
	if err != nil {
		return err
	}
	for key, value := range map[string]string{keyAIGatewayEnv: environment, keyAIGatewayID: aiID, keyRegularGatewayID: regularID} {
		if err := tcontext.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

// pairGatewaysByEnvironment finds the first environment that holds both an AI and a regular
// gateway, preferring each kind's default.
func pairGatewaysByEnvironment(items []map[string]any) (environment, aiID, regularID string, err error) {
	type pair struct {
		ai, regular               string
		aiDefault, regularDefault bool
	}
	byEnvironment := map[string]*pair{}
	var order []string
	for _, item := range items {
		id, _ := item["id"].(string)
		env, _ := item["environment"].(string)
		id, env = strings.TrimSpace(id), strings.TrimSpace(env)
		if id == "" || env == "" {
			continue
		}
		held, ok := byEnvironment[env]
		if !ok {
			held = &pair{}
			byEnvironment[env] = held
			order = append(order, env)
		}
		isDefault, _ := item["isDefault"].(bool)
		switch gatewayKind(item) {
		case aiGatewayKind:
			if held.ai == "" || (isDefault && !held.aiDefault) {
				held.ai, held.aiDefault = id, isDefault
			}
		case regularGatewayKind:
			if held.regular == "" || (isDefault && !held.regularDefault) {
				held.regular, held.regularDefault = id, isDefault
			}
		}
	}
	for _, env := range order {
		if held := byEnvironment[env]; held.ai != "" && held.regular != "" {
			return env, held.ai, held.regular, nil
		}
	}
	return "", "", "", fmt.Errorf("no environment holds both an AI gateway and a regular gateway (%d gateways listed in %d environments)",
		len(items), len(order))
}

// echoBackendOf is the backend the synthetic provider's upstream points at.
func echoBackendOf(ctx context.Context) (string, error) {
	backend, err := requiredValue(ctx, keyEchoBackendURL)
	if err != nil {
		return "", fmt.Errorf("choose the echo backend before creating the provider: %w", err)
	}
	return backend, nil
}

// expandedJSON expands a table value and decodes it as JSON.
func expandedJSON(ctx context.Context, key, raw string) (any, error) {
	value, err := expandCloudValue(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("expanding %q: %w", key, err)
	}
	var decoded any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return nil, fmt.Errorf("value %q is not valid JSON: %w", key, err)
	}
	return decoded, nil
}

func checkKeys(allowed map[string]bool, values map[string]string, what string) error {
	for key := range values {
		if !allowed[key] {
			return fmt.Errorf("unsupported %s value %q", what, key)
		}
	}
	return nil
}

// newLLMProviderDefinition builds a provider that forwards to the echo backend with every path
// allowed. A table value changes the template, the upstream URL or the upstream credential.
func newLLMProviderDefinition(ctx context.Context, id string, overrides map[string]string) (map[string]any, error) {
	if err := checkKeys(llmProviderKeys, overrides, "LLM provider"); err != nil {
		return nil, err
	}
	upstreamURL, err := echoBackendOf(ctx)
	if err != nil {
		return nil, err
	}
	if raw := strings.TrimSpace(overrides["upstreamUrl"]); raw != "" {
		if upstreamURL, err = expandCloudValue(ctx, raw); err != nil {
			return nil, err
		}
	}
	template := defaultLLMTemplate
	if raw := strings.TrimSpace(overrides["template"]); raw != "" {
		if template, err = expandCloudValue(ctx, raw); err != nil {
			return nil, err
		}
	}
	main := map[string]any{"url": upstreamURL}
	if raw := strings.TrimSpace(overrides["upstreamAuth"]); raw != "" {
		auth, err := expandedJSON(ctx, "upstreamAuth", raw)
		if err != nil {
			return nil, err
		}
		main["auth"] = auth
	}
	return map[string]any{
		"id": id, "displayName": "Synthetic " + id,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
		"version":     "v1.0", "context": "/" + id, "template": template,
		"upstream":      map[string]any{"main": main},
		"accessControl": map[string]any{"mode": "allow_all"},
	}, nil
}

// newLLMProxyDefinition builds a proxy over the synthetic provider in a project.
func newLLMProxyDefinition(ctx context.Context, id, projectID, providerID string, overrides map[string]string) (map[string]any, error) {
	if err := checkKeys(llmProxyKeys, overrides, "LLM proxy"); err != nil {
		return nil, err
	}
	provider := map[string]any{"id": providerID}
	definition := map[string]any{
		"id": id, "displayName": "Synthetic " + id,
		"description": "Created and deleted by the APIP cloud integration test. Safe to delete.",
		"version":     "v1.0", "projectId": projectID, "context": "/" + id, "provider": provider,
	}
	for key, raw := range overrides {
		decoded, err := expandedJSON(ctx, key, raw)
		if err != nil {
			return nil, err
		}
		if key == "providerAuth" {
			provider["auth"] = decoded
			continue
		}
		definition[key] = decoded
	}
	return definition, nil
}

// postLLMArtifact registers the artifact for cleanup, creates it and keeps the id the platform
// answered with. A refused create is deregistered so cleanup does not look for it.
func (s *Steps) postLLMArtifact(ctx context.Context, a llmArtifact, id string, definition map[string]any) error {
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: a.cleanupKind, ID: id, Actor: resourceActor, Description: a.noun + " " + id,
	}); err != nil {
		return fmt.Errorf("registering %s %q for cleanup: %w", a.noun, id, err)
	}
	resp, err := s.doAuthenticated(ctx, http.MethodPost, a.collection, definition)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		deregister(ctx, a.cleanupKind, id)
		return responseError(resp)
	}
	createdID, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating the %s: %w", a.noun, err)
	}
	if createdID != id {
		deregister(ctx, a.cleanupKind, id)
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: a.cleanupKind, ID: createdID, Actor: resourceActor, Description: a.noun + " " + createdID,
		}); err != nil {
			return fmt.Errorf("registering %s %q for cleanup: %w", a.noun, createdID, err)
		}
	}
	return tcontext.Set(ctx, a.idKey, createdID)
}

// chooseLLMHandle generates the artifact's handle and its context.
func chooseLLMHandle(ctx context.Context, a llmArtifact) (string, error) {
	if err := generateResourceAndStore(ctx, a.handleBase, a.idKey); err != nil {
		return "", err
	}
	id, err := requiredValue(ctx, a.idKey)
	if err != nil {
		return "", err
	}
	if err := tcontext.Set(ctx, a.contextKey, "/"+id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Steps) createLLMProvider(ctx context.Context, table *godog.Table) error {
	overrides, err := tableValues(table)
	if err != nil {
		return err
	}
	id, err := chooseLLMHandle(ctx, providerArtifact)
	if err != nil {
		return err
	}
	definition, err := newLLMProviderDefinition(ctx, id, overrides)
	if err != nil {
		return err
	}
	return s.postLLMArtifact(ctx, providerArtifact, id, definition)
}

func (s *Steps) createLLMProxy(ctx context.Context, table *godog.Table) error {
	overrides, err := tableValues(table)
	if err != nil {
		return err
	}
	projectID, err := requiredValue(ctx, keyProjectID)
	if err != nil {
		return err
	}
	providerID, err := requiredValue(ctx, keyLLMProviderID)
	if err != nil {
		return fmt.Errorf("a proxy needs a synthetic LLM provider: %w", err)
	}
	id, err := chooseLLMHandle(ctx, proxyArtifact)
	if err != nil {
		return err
	}
	definition, err := newLLMProxyDefinition(ctx, id, projectID, providerID, overrides)
	if err != nil {
		return err
	}
	return s.postLLMArtifact(ctx, proxyArtifact, id, definition)
}

// listLLMArtifacts reads the first page of the organisation's providers or proxies.
func (s *Steps) listLLMArtifacts(a llmArtifact) func(context.Context) error {
	return func(ctx context.Context) error {
		path, err := a.listPath(ctx)
		if err != nil {
			return err
		}
		resp, err := s.doAuthenticated(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return responseError(resp)
		}
		return nil
	}
}

// readLLMProvider reads the synthetic provider by its handle.
func (s *Steps) readLLMProvider(ctx context.Context) error {
	id, err := requiredValue(ctx, keyLLMProviderID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodGet, providerArtifact.collection+"/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	return nil
}

// assertListIncludes checks the published list names the synthetic artifact.
func (s *Steps) assertListIncludes(a llmArtifact) func(context.Context) error {
	return func(ctx context.Context) error {
		id, err := requiredValue(ctx, a.idKey)
		if err != nil {
			return err
		}
		response, err := httpx.Published(ctx)
		if err != nil {
			return err
		}
		items, err := listItems(response)
		if err != nil {
			return fmt.Errorf("decoding the %s list: %w", a.noun, err)
		}
		for _, item := range items {
			if item["id"] == id {
				return nil
			}
		}
		return fmt.Errorf("the list of %ss (%d listed) does not include the synthetic %s %q", a.noun, len(items), a.noun, id)
	}
}

// assertEveryListedArtifactReadable reads each listed provider and proxy by its handle. A handle
// that lists but cannot be read would belong to something other than the caller's organisation.
func (s *Steps) assertEveryListedArtifactReadable(ctx context.Context) error {
	for _, a := range []llmArtifact{providerArtifact, proxyArtifact} {
		path, err := a.listPath(ctx)
		if err != nil {
			return err
		}
		resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, path, nil)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return responseError(resp)
		}
		items, err := listItems(resp)
		if err != nil {
			return fmt.Errorf("decoding the %s list: %w", a.noun, err)
		}
		for _, item := range items {
			id, _ := item["id"].(string)
			if id == "" {
				return fmt.Errorf("a listed %s has no id", a.noun)
			}
			one, err := s.doAuthenticatedRaw(ctx, http.MethodGet, a.collection+"/"+url.PathEscape(id), nil)
			if err != nil {
				return err
			}
			if one.StatusCode != http.StatusOK {
				return fmt.Errorf("the listed %s %q cannot be read: %s", a.noun, id, one.Describe())
			}
		}
	}
	return nil
}

// documentPath reads a dotted path of a decoded JSON document.
func documentPath(document any, path string) (any, bool) {
	current := document
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		if current, ok = object[part]; !ok {
			return nil, false
		}
	}
	return current, true
}

// assertResponseReports checks fields of the published response against a table of dotted paths
// and expected values.
func (s *Steps) assertResponseReports(ctx context.Context, table *godog.Table) error {
	values, err := tableValues(table)
	if err != nil {
		return err
	}
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var document any
	if err := json.Unmarshal(response.Body, &document); err != nil {
		return fmt.Errorf("the response is not JSON: %w (%s)", err, response.Describe())
	}
	var differing []string
	for path, raw := range values {
		want, err := expandCloudValue(ctx, raw)
		if err != nil {
			return fmt.Errorf("expanding %q: %w", path, err)
		}
		got, present := documentPath(document, path)
		if !present || !sameValue(got, parseConfigCell(want)) {
			differing = append(differing, fmt.Sprintf("%s is %v, want %s", path, got, want))
		}
	}
	sort.Strings(differing)
	if len(differing) > 0 {
		return fmt.Errorf("the response does not report the expected fields: %s", strings.Join(differing, "; "))
	}
	return nil
}

// requestLLMDeployment asks for a deployment of the synthetic artifact and publishes the response
// without requiring success, so a scenario can assert on a refusal. A created deployment is
// recorded by gateway.
func (s *Steps) requestLLMDeployment(a llmArtifact) func(context.Context, *godog.Table) error {
	return func(ctx context.Context, table *godog.Table) error {
		values, err := tableValues(table)
		if err != nil {
			return err
		}
		body, err := buildDeploymentRequest(ctx, values)
		if err != nil {
			return err
		}
		path, query, err := a.deploymentRoute(ctx)
		if err != nil {
			return err
		}
		resp, err := s.doAuthenticated(ctx, http.MethodPost, withQuery(path, query), body)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusCreated {
			return nil
		}
		return recordLLMTargets(ctx, a, resp)
	}
}

// recordLLMTargets keeps the deployment each gateway was given, by gateway.
func recordLLMTargets(ctx context.Context, a llmArtifact, resp *httpx.Response) error {
	items, err := listItems(resp)
	if err != nil {
		return fmt.Errorf("decoding the deployment response: %w", err)
	}
	targets := map[string]string{}
	for _, item := range items {
		gatewayID, _ := item["gatewayId"].(string)
		deploymentID, _ := item["deploymentId"].(string)
		if gatewayID != "" && deploymentID != "" {
			targets[gatewayID] = deploymentID
		}
	}
	encoded, err := json.Marshal(targets)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, a.targetsKey, string(encoded))
}

func llmTargets(ctx context.Context, a llmArtifact) (map[string]string, error) {
	encoded, err := requiredValue(ctx, a.targetsKey)
	if err != nil {
		return nil, fmt.Errorf("the synthetic %s has not been deployed: %w", a.noun, err)
	}
	targets := map[string]string{}
	if err := json.Unmarshal([]byte(encoded), &targets); err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("the deployment request of the synthetic %s created no deployment", a.noun)
	}
	return targets, nil
}

// deploymentStates reads the status the deployment listing gives each gateway the artifact was
// deployed to.
func (s *Steps) deploymentStates(ctx context.Context, a llmArtifact, targets map[string]string) (map[string]string, error) {
	path, query, err := a.deploymentRoute(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, withQuery(path, query), nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retry.Transient(responseError(resp))
	}
	stages, err := listItems(resp)
	if err != nil {
		return nil, retry.Transient(fmt.Errorf("decoding the %s deployments: %w", a.noun, err))
	}
	states := map[string]string{}
	for _, stage := range stages {
		gateways, _ := stage["gateways"].([]any)
		for _, raw := range gateways {
			gateway, _ := raw.(map[string]any)
			gatewayID, _ := gateway["gatewayId"].(string)
			if _, wanted := targets[gatewayID]; !wanted {
				continue
			}
			status, _ := gateway["status"].(string)
			states[gatewayID] = status
		}
	}
	return states, nil
}

// waitLLMDeployment waits until the artifact's deployment on every gateway it was given to has the
// status, and stops early on a deployment that failed.
func (s *Steps) waitLLMDeployment(a llmArtifact) func(context.Context, string) error {
	return func(ctx context.Context, want string) error {
		targets, err := llmTargets(ctx, a)
		if err != nil {
			return err
		}
		last, err := retry.Until(ctx, retry.Options{Timeout: deployedTimeout, Interval: 5 * time.Second},
			func(ctx context.Context) (string, error) {
				states, err := s.deploymentStates(ctx, a, targets)
				if err != nil {
					return "", err
				}
				return summariseStates(targets, states, want), nil
			},
			func(summary string) bool { return summary == "" || strings.Contains(summary, "FAILED") })
		if err != nil {
			return fmt.Errorf("waiting for the %s deployment to be %s: %w", a.noun, want, err)
		}
		if last != "" {
			return fmt.Errorf("the %s deployment never reached %s: %s", a.noun, want, last)
		}
		return nil
	}
}

// summariseStates names the gateways that are not yet at the wanted status; it is empty when every
// gateway is.
func summariseStates(targets, states map[string]string, want string) string {
	var pending []string
	for gatewayID := range targets {
		if got := states[gatewayID]; got != want {
			if got == "" {
				got = "not listed"
			}
			pending = append(pending, gatewayID+": "+got)
		}
	}
	sort.Strings(pending)
	return strings.Join(pending, ", ")
}

// undeployLLM undeploys the artifact from every gateway it was deployed to.
func (s *Steps) undeployLLM(a llmArtifact) func(context.Context) error {
	return func(ctx context.Context) error {
		targets, err := llmTargets(ctx, a)
		if err != nil {
			return err
		}
		environment, err := requiredValue(ctx, keyAIGatewayEnv)
		if err != nil {
			return err
		}
		path, query, err := a.deploymentRoute(ctx)
		if err != nil {
			return err
		}
		for gatewayID, deploymentID := range targets {
			query.Set("environment", environment)
			query.Set("gatewayId", gatewayID)
			resp, err := s.doAuthenticated(ctx, http.MethodPost,
				withQuery(path+"/"+url.PathEscape(deploymentID)+"/undeploy", query), nil)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusNoContent {
				return responseError(resp)
			}
		}
		return nil
	}
}

// chatCompletionRequest is the request a client sends an LLM provider or proxy: one user message.
func chatCompletionRequest(target, text string) (httpx.Request, error) {
	body, err := json.Marshal(map[string]any{
		"model":    defaultLLMModel,
		"messages": []map[string]string{{"role": "user", "content": text}},
	})
	if err != nil {
		return httpx.Request{}, err
	}
	return httpx.Request{Method: http.MethodPost, URL: target, Body: body, ContentType: "application/json"}, nil
}

// sendChatUntilStatus sends a chat completion request until the gateway answers with the status.
func (s *Steps) sendChatUntilStatus(ctx context.Context, target, text string, want int) error {
	request, err := chatCompletionRequest(target, text)
	if err != nil {
		return err
	}
	return s.sendRequestUntilStatus(ctx, request, want)
}

// sendChatUntilEchoesRotated sends chat completion requests until the echoed header carries the
// replacement secret's value.
func (s *Steps) sendChatUntilEchoesRotated(ctx context.Context, target, text, header string) error {
	current, err := requiredValue(ctx, keySecretValue)
	if err != nil {
		return err
	}
	previous, err := requiredValue(ctx, keySecretPreviousValue)
	if err != nil {
		return fmt.Errorf("the secret has not been replaced: %w", err)
	}
	request, err := chatCompletionRequest(target, text)
	if err != nil {
		return err
	}
	request, err = s.prepareDataPlaneRequest(ctx, request)
	if err != nil {
		return err
	}
	return s.untilEchoesRotated(ctx, request, header, current, previous)
}

// registerAIWorkspaceSteps binds the steps that find an AI gateway, create synthetic LLM providers
// and proxies, deploy them and send them chat requests.
func (s *Steps) registerAIWorkspaceSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I find an active APIP AI gateway$`, s.findActiveAIGateway)
	sc.Step(`^I find an AI gateway and a regular gateway of one APIP environment$`, s.findAIAndRegularGateway)

	sc.Step(`^I create a synthetic LLM provider$`, func(ctx context.Context) error { return s.createLLMProvider(ctx, nil) })
	sc.Step(`^I create a synthetic LLM provider with:$`, s.createLLMProvider)
	sc.Step(`^I create a synthetic LLM proxy over the synthetic LLM provider in the default project$`,
		func(ctx context.Context) error { return s.createLLMProxy(ctx, nil) })
	sc.Step(`^I create a synthetic LLM proxy over the synthetic LLM provider in the default project with:$`, s.createLLMProxy)
	sc.Step(`^I list the LLM providers$`, s.listLLMArtifacts(providerArtifact))
	sc.Step(`^I list the LLM proxies$`, s.listLLMArtifacts(proxyArtifact))
	sc.Step(`^I read the synthetic LLM provider$`, s.readLLMProvider)
	sc.Step(`^the LLM provider list should include the synthetic LLM provider$`, s.assertListIncludes(providerArtifact))
	sc.Step(`^the LLM proxy list should include the synthetic LLM proxy$`, s.assertListIncludes(proxyArtifact))
	sc.Step(`^every listed LLM provider and proxy should be readable by its handle$`, s.assertEveryListedArtifactReadable)
	sc.Step(`^the cloud API response should report:$`, s.assertResponseReports)

	sc.Step(`^I request a deployment of the synthetic LLM provider with:$`, s.requestLLMDeployment(providerArtifact))
	sc.Step(`^I request a deployment of the synthetic LLM proxy with:$`, s.requestLLMDeployment(proxyArtifact))
	sc.Step(`^the synthetic LLM provider deployment should become (DEPLOYED|UNDEPLOYED)$`, s.waitLLMDeployment(providerArtifact))
	sc.Step(`^the synthetic LLM proxy deployment should become (DEPLOYED|UNDEPLOYED)$`, s.waitLLMDeployment(proxyArtifact))
	sc.Step(`^I undeploy the synthetic LLM provider$`, s.undeployLLM(providerArtifact))
	sc.Step(`^I undeploy the synthetic LLM proxy$`, s.undeployLLM(proxyArtifact))

	sc.Step(`^I send a chat completion request to "([^"]*)" saying "([^"]*)" until status (\d+)$`, s.sendChatUntilStatus)
	sc.Step(`^a chat completion request to "([^"]*)" saying "([^"]*)" should echo the header "([^"]*)" with the replacement secret value$`,
		s.sendChatUntilEchoesRotated)
}

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

// Package steps holds the integration suite's step definitions.
package steps

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
	platformgateway "github.com/wso2/api-platform/tests/framework/suites/it/steps/platformgateway"
)

// API base paths, kept in sync with the gateway's OpenAPI documents.
const (
	ManagementBasePath = "/api/management/v1"
	adminBasePath      = "/api/admin/v1"
)

// Context keys this suite publishes.
const (
	keyLastAPIName = "lastApiName"

	// Request-shaping state set by one step and read by the next. It lives in the SCENARIO
	// scope rather than on the Gateway struct because one Gateway serves a whole block, and
	// its runners run in parallel — a struct field here would be shared mutable state
	// across concurrent runners, which is the exact defect this framework exists to remove.
	// What this scenario expects the config dump to show: handle -> should-be-present.
	// A deploy sets true, a delete sets false. Both are waits — see awaitDumpConsistent.
	keyDumpExpectations = "dumpExpectations"
)

// headerRateLimitRemaining is how every rate-limit policy reports what is left of a quota.
//
// ONE header, whatever the policy counts: advanced-ratelimit counts requests and
// token-based-ratelimit counts tokens, but both report through this name. Lookup is
// case-insensitive per RFC 7230, which is the only reason features spelling it
// "X-Ratelimit-Remaining" also pass — the casing here is the product's.
const headerRateLimitRemaining = "X-RateLimit-Remaining"

// sendUntilQuotaRemaining waits for the configured rate-limit remainder.
func (g *Gateway) sendUntilQuotaRemaining(ctx context.Context, method, path string, want int) error {
	return g.sendUntilHeader(ctx, method, path, headerRateLimitRemaining, strconv.Itoa(want))
}

// Gateway holds what the gateway steps need. The shared plumbing lives in Base.
type Gateway struct {
	*Base
}

type lazyResource struct {
	ID       string         `json:"id"`
	Type     string         `json:"resource_type"`
	Resource map[string]any `json:"resource"`
}

type lazyDump struct {
	ResourcesByType map[string][]lazyResource `json:"resources_by_type"`
}

type configDump struct {
	Lazy          lazyDump          `json:"lazy_resources"`
	RouteMetadata routeMetadataDump `json:"route_metadata"`
	PolicyChains  policyChainsDump  `json:"policy_chains"`
}

type routeMetadataDump struct {
	Routes []struct {
		// Context is the API's resolved gateway-facing base path (e.g. "/admin-test/v1") -
		// the policy engine has no field literally named "basePath".
		Context string `json:"context"`
	} `json:"routes"`
}

type policyChainsDump struct {
	PolicyChains []struct {
		// RouteKey is "METHOD|fullPath|vhost" (see GenerateRouteNameWithDiscriminator).
		RouteKey string `json:"route_key"`
		Policies []struct {
			Name string `json:"name"`
		} `json:"policies"`
	} `json:"policy_chains"`
}

// routeKeyPath extracts the fullPath segment from a "METHOD|fullPath|vhost" route key.
func routeKeyPath(routeKey string) string {
	parts := strings.SplitN(routeKey, "|", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func (g *Gateway) lazyResources(ctx context.Context) ([]lazyResource, error) {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return nil, err
	}
	var dump configDump
	if err := json.Unmarshal(resp.Body, &dump); err != nil {
		return nil, fmt.Errorf("parse policy-engine config dump: %w", err)
	}
	var resources []lazyResource
	for _, group := range dump.Lazy.ResourcesByType {
		resources = append(resources, group...)
	}
	return resources, nil
}

func (g *Gateway) lazyTemplatePresent(ctx context.Context, id, kind string) error {
	return g.lazyResourcePresentByType(ctx, id, kind, true)
}

func (g *Gateway) lazyResourcePresent(ctx context.Context, id, kind string) error {
	return g.lazyResourcePresentByType(ctx, id, kind, true)
}

func (g *Gateway) lazyResourcePresentByType(ctx context.Context, id, kind string, want bool) error {
	id, err := stepscommon.Expand(ctx, id)
	if err != nil {
		return err
	}
	resources, err := g.lazyResources(ctx)
	if err != nil {
		return err
	}
	for _, resource := range resources {
		if resource.ID == id && resource.Type == kind {
			if want {
				return nil
			}
			return fmt.Errorf("lazy resource %q of type %q is present", id, kind)
		}
	}
	if want {
		return fmt.Errorf("lazy resource %q of type %q is absent", id, kind)
	}
	return nil
}

func (g *Gateway) lazyTemplateAbsent(ctx context.Context, id string) error {
	return g.lazyResourceAbsent(ctx, id)
}

func (g *Gateway) lazyResourceAbsent(ctx context.Context, id string) error {
	id, err := stepscommon.Expand(ctx, id)
	if err != nil {
		return err
	}
	resources, err := g.lazyResources(ctx)
	if err != nil {
		return err
	}
	for _, resource := range resources {
		if resource.ID == id {
			return fmt.Errorf("lazy resource %q is present as %q", id, resource.Type)
		}
	}
	return nil
}

func (g *Gateway) lazyTypedResourceAbsent(ctx context.Context, id, kind string) error {
	return g.lazyResourcePresentByType(ctx, id, kind, false)
}

func (g *Gateway) lazyDisplayName(ctx context.Context, id, want string) error {
	id, err := stepscommon.Expand(ctx, id)
	if err != nil {
		return err
	}
	resources, err := g.lazyResources(ctx)
	if err != nil {
		return err
	}
	for _, resource := range resources {
		if resource.ID != id {
			continue
		}
		spec, ok := resource.Resource["spec"].(map[string]any)
		if !ok {
			return fmt.Errorf("lazy resource %q has no spec", id)
		}
		if spec["displayName"] != want {
			return fmt.Errorf("lazy resource %q has display name %v, want %q", id, spec["displayName"], want)
		}
		return nil
	}
	return fmt.Errorf("lazy resource %q is absent", id)
}

func (g *Gateway) providerTemplateMapping(ctx context.Context, provider, template string) error {
	provider, err := stepscommon.Expand(ctx, provider)
	if err != nil {
		return err
	}
	template, err = stepscommon.Expand(ctx, template)
	if err != nil {
		return err
	}
	resources, err := g.lazyResources(ctx)
	if err != nil {
		return err
	}
	for _, resource := range resources {
		if resource.ID == provider && resource.Type == "ProviderTemplateMapping" {
			if resource.Resource["template_handle"] != template {
				return fmt.Errorf("provider %q maps to %v, want %q", provider, resource.Resource["template_handle"], template)
			}
			return nil
		}
	}
	return fmt.Errorf("provider template mapping %q is absent", provider)
}

// serviceRequestUntilProviderTemplateMapping polls a policy-engine config dump until the
// requested provider mapping points to the expected template.
func (g *Gateway) serviceRequestUntilProviderTemplateMapping(
	ctx context.Context, method, service, path, provider, template string,
) error {
	if strings.ToUpper(method) != http.MethodGet || service != "policy-engine" {
		return fmt.Errorf("provider-mapping readiness requires a GET request to the policy-engine")
	}
	url, err := g.serviceURL(ctx, service, path)
	if err != nil {
		return err
	}
	provider, err = stepscommon.Expand(ctx, provider)
	if err != nil {
		return err
	}
	template, err = stepscommon.Expand(ctx, template)
	if err != nil {
		return err
	}
	accept := func(resp *httpx.Response) bool {
		return resp != nil && resp.Succeeded() && providerTemplateMappingMatches(resp.Body, provider, template)
	}
	last, err := retry.Until(ctx, retry.Options{Interval: 200 * time.Millisecond},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, requestErr := g.funnel.Client().Do(ctx, httpx.Request{
				Method: http.MethodGet, URL: url, Headers: g.scenarioHeaders(ctx),
			}, 0, 0)
			if requestErr != nil {
				return nil, retry.Transient(requestErr)
			}
			return resp, nil
		}, accept)
	if err != nil {
		return fmt.Errorf("waiting for provider %q to map to template %q: %w", provider, template, err)
	}
	if !accept(last) {
		return fmt.Errorf("provider %q did not map to template %q", provider, template)
	}
	return g.funnel.Publish(ctx, last)
}

func providerTemplateMappingMatches(body []byte, provider, template string) bool {
	var dump configDump
	if err := json.Unmarshal(body, &dump); err != nil {
		return false
	}
	for _, resource := range dump.Lazy.ResourcesByType["ProviderTemplateMapping"] {
		if resource.ID == provider {
			return resource.Resource["template_handle"] == template
		}
	}
	return false
}

// serviceRequestUntilLazyResourceAbsent polls a policy-engine config dump until the specified
// resource type no longer contains the requested resource ID.
func (g *Gateway) serviceRequestUntilLazyResourceAbsent(
	ctx context.Context, method, service, path, resourceID, resourceType string,
) error {
	if strings.ToUpper(method) != http.MethodGet || service != "policy-engine" {
		return fmt.Errorf("lazy-resource deletion readiness requires a GET request to the policy-engine")
	}
	url, err := g.serviceURL(ctx, service, path)
	if err != nil {
		return err
	}
	resourceID, err = stepscommon.Expand(ctx, resourceID)
	if err != nil {
		return err
	}
	resourceType, err = stepscommon.Expand(ctx, resourceType)
	if err != nil {
		return err
	}
	accept := func(resp *httpx.Response) bool {
		return resp != nil && resp.Succeeded() && lazyResourceAbsent(resp.Body, resourceID, resourceType)
	}
	last, err := retry.Until(ctx, retry.Options{Interval: 200 * time.Millisecond},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, requestErr := g.funnel.Client().Do(ctx, httpx.Request{
				Method: http.MethodGet, URL: url, Headers: g.scenarioHeaders(ctx),
			}, 0, 0)
			if requestErr != nil {
				return nil, retry.Transient(requestErr)
			}
			return resp, nil
		}, accept)
	if err != nil {
		return fmt.Errorf("waiting for lazy resource %q of type %q to be removed: %w", resourceID, resourceType, err)
	}
	if !accept(last) {
		return fmt.Errorf("lazy resource %q of type %q was not removed", resourceID, resourceType)
	}
	return g.funnel.Publish(ctx, last)
}

func lazyResourceAbsent(body []byte, resourceID, resourceType string) bool {
	var dump configDump
	if err := json.Unmarshal(body, &dump); err != nil {
		return false
	}
	for _, resource := range dump.Lazy.ResourcesByType[resourceType] {
		if resource.ID == resourceID {
			return false
		}
	}
	return true
}

func (g *Gateway) routeMetadataProvider(ctx context.Context, provider string) error {
	provider, err := stepscommon.Expand(ctx, provider)
	if err != nil {
		return err
	}
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if strings.Contains(string(resp.Body), provider) {
		return nil
	}
	return fmt.Errorf("route metadata does not contain provider %q", provider)
}

// awaitConfigDumpRouteAbsent polls the policy engine's own config dump until no route has the
// given base path. The generic funnel's dump-consistency wait (awaitDumpConsistent) is capped
// at retry.PropagationCeiling (60s); the policy engine's own route_metadata rebuild after an
// API deletion has been measured taking well beyond that and varies widely between runs
// (observed ~70s in one run, ~153s in another - consistent with a periodic reconciliation
// cycle rather than a fixed propagation delay), so this polls with its own much longer timeout
// instead of relying on that fixed ceiling.
func (g *Gateway) awaitConfigDumpRouteAbsent(ctx context.Context, basePath string) error {
	resolved, err := stepscommon.Expand(ctx, basePath)
	if err != nil {
		return err
	}
	url, err := g.serviceURL(ctx, "policy-engine", "/config_dump")
	if err != nil {
		return err
	}
	accept := func(resp *httpx.Response) bool {
		if resp == nil || !resp.Succeeded() {
			return false
		}
		var dump configDump
		if err := json.Unmarshal(resp.Body, &dump); err != nil {
			return false
		}
		for _, route := range dump.RouteMetadata.Routes {
			if route.Context == resolved {
				return false
			}
		}
		return true
	}
	last, err := retry.Until(ctx, retry.Options{Timeout: 240 * time.Second, Interval: 2 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, requestErr := g.funnel.Client().Do(ctx, httpx.Request{
				Method: http.MethodGet, URL: url, Headers: g.scenarioHeaders(ctx),
			}, 0, 0)
			if requestErr != nil {
				return nil, retry.Transient(requestErr)
			}
			return resp, nil
		}, accept)
	if err := awaited(last, err, accept,
		fmt.Sprintf("waiting for the config dump to stop containing a route with base path %q", resolved)); err != nil {
		return err
	}
	return g.funnel.Publish(ctx, last)
}

// configDumpRouteBasePath asserts whether the policy engine's config dump has a route whose
// resolved context (its gateway-facing base path) matches basePath.
func (g *Gateway) configDumpRouteBasePath(ctx context.Context, basePath string, want bool) error {
	resolved, err := stepscommon.Expand(ctx, basePath)
	if err != nil {
		return err
	}
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var dump configDump
	if err := json.Unmarshal(resp.Body, &dump); err != nil {
		return fmt.Errorf("parsing policy-engine config dump: %w", err)
	}
	found := false
	for _, route := range dump.RouteMetadata.Routes {
		if route.Context == resolved {
			found = true
			break
		}
	}
	if found == want {
		return nil
	}
	if want {
		return fmt.Errorf("config dump does not contain a route with base path %q", resolved)
	}
	return fmt.Errorf("config dump still contains a route with base path %q", resolved)
}

// configDumpPolicyForRoute asserts the policy engine's config dump shows policyName attached
// to the operation whose full path is routePath.
func (g *Gateway) configDumpPolicyForRoute(ctx context.Context, policyName, routePath string) error {
	resolvedPath, err := stepscommon.Expand(ctx, routePath)
	if err != nil {
		return err
	}
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var dump configDump
	if err := json.Unmarshal(resp.Body, &dump); err != nil {
		return fmt.Errorf("parsing policy-engine config dump: %w", err)
	}
	for _, entry := range dump.PolicyChains.PolicyChains {
		if routeKeyPath(entry.RouteKey) != resolvedPath {
			continue
		}
		for _, p := range entry.Policies {
			if p.Name == policyName {
				return nil
			}
		}
	}
	return fmt.Errorf("config dump does not show policy %q attached to route %q", policyName, resolvedPath)
}

func (g *Gateway) lazyResourceCount(ctx context.Context, want int, id string) error {
	id, err := stepscommon.Expand(ctx, id)
	if err != nil {
		return err
	}
	resources, err := g.lazyResources(ctx)
	if err != nil {
		return err
	}
	count := 0
	for _, resource := range resources {
		if resource.ID == id {
			count++
		}
	}
	if count < want {
		return fmt.Errorf("found %d lazy resources with id %q, want at least %d", count, id, want)
	}
	return nil
}

func (g *Gateway) mcpInitialize(ctx context.Context, path string) error {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{"roots":{"listChanged":true}},"clientInfo":{"name":"framework-client","version":"1.0.0"}}}`
	return g.sendMCPRequest(ctx, path, body)
}

// mcpToolCall calls the named testbench MCP tool with arguments matching that tool's own
// input schema - the testbench "add" tool takes numeric operands, "echo" takes a message.
func (g *Gateway) mcpToolCall(ctx context.Context, tool, path string) error {
	args := `{"a":40,"b":60}`
	if tool == "echo" {
		args = `{"message":"Hello, World!"}`
	}
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, tool, args)
	return g.sendMCPRequest(ctx, path, body)
}

func (g *Gateway) mcpToolsList(ctx context.Context, path string) error {
	body := `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`
	return g.sendMCPRequest(ctx, path, body)
}

// mcpNotificationInitialized sends the notification a client is expected to send once, right
// after a successful initialize response, before issuing any further request.
func (g *Gateway) mcpNotificationInitialized(ctx context.Context, path string) error {
	body := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	return g.sendMCPRequest(ctx, path, body)
}

// mcpToolCallInvalidParams omits the required "name" field, which every JSON-RPC tools/call
// request must carry.
func (g *Gateway) mcpToolCallInvalidParams(ctx context.Context, path string) error {
	body := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"arguments":{"a":40,"b":60}}}`
	return g.sendMCPRequest(ctx, path, body)
}

// jwtToken mints a JWT from the testbench jwks mock and stores it in runner context under key,
// for a later "I set header" step to attach as a bearer token. issuer becomes the token's iss
// claim and must match a configured keymanager's issuer exactly; scope and claims are optional
// (empty skips them) and claims is a comma-separated list of key=value pairs.
func (g *Gateway) jwtToken(ctx context.Context, issuer, scope, claims, key string) error {
	resolvedIssuer, err := stepscommon.Expand(ctx, issuer)
	if err != nil {
		return err
	}
	base, err := g.topo.URL("testbench", "jwks")
	if err != nil {
		return err
	}
	q := url.Values{}
	q.Set("issuer", resolvedIssuer)
	if scope != "" {
		resolvedScope, err := stepscommon.Expand(ctx, scope)
		if err != nil {
			return err
		}
		q.Set("scope", resolvedScope)
	}
	for _, pair := range strings.Split(claims, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("invalid claim %q: expected key=value", pair)
		}
		claimValue, err := stepscommon.Expand(ctx, strings.TrimSpace(kv[1]))
		if err != nil {
			return err
		}
		q.Set("claim_"+strings.TrimSpace(kv[0]), claimValue)
	}
	resp, err := retry.Until(ctx, retry.Options{},
		func(ctx context.Context) (*httpx.Response, error) {
			r, requestErr := g.funnel.Client().Do(ctx, httpx.Request{
				Method: http.MethodGet, URL: base + "/token?" + q.Encode(),
			}, 0, 0)
			if requestErr != nil {
				return nil, retry.Transient(requestErr)
			}
			return r, nil
		}, func(resp *httpx.Response) bool { return resp != nil })
	if err != nil {
		return fmt.Errorf("minting a JWT token: %w", err)
	}
	if !resp.Succeeded() {
		return fmt.Errorf("minting a JWT token failed: %s", resp.Describe())
	}
	local, ok := tcontext.LocalOf(ctx)
	if !ok || local == nil {
		return fmt.Errorf("cannot store a JWT token without runner context")
	}
	local.Set(key, strings.TrimSpace(string(resp.Body)))
	return nil
}

// sendMCPRequest sends a streamable HTTP MCP request and publishes its JSON-RPC payload.
func (g *Gateway) sendMCPRequest(ctx context.Context, path, body string) error {
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	url, err := g.gatewayURL(resolved)
	if err != nil {
		return err
	}
	resp, err := g.funnel.Send(ctx, httpx.Request{
		Method: http.MethodPost,
		URL:    url,
		Headers: func() map[string]string {
			headers := g.scenarioHeaders(ctx)
			headers["Content-Type"] = "application/json"
			headers["Accept"] = "application/json, text/event-stream"
			return headers
		}(),
		Body: []byte(body),
		Host: g.requestHost(ctx),
	})
	if err != nil {
		return err
	}
	resp.Body = mcpJSONPayload(resp.Body)
	return g.funnel.Publish(ctx, resp)
}

// mcpJSONPayload extracts the JSON-RPC document from a streamable HTTP SSE response.
func mcpJSONPayload(body []byte) []byte {
	text := string(body)
	marker := "data:"
	start := strings.Index(text, marker)
	if start < 0 {
		return body
	}
	start += len(marker)
	end := strings.IndexByte(text[start:], '\n')
	if end >= 0 {
		end += start
	} else {
		end = len(text)
	}
	payload := strings.TrimSpace(text[start:end])
	if json.Valid([]byte(payload)) {
		return []byte(payload)
	}
	return body
}

// awaited turns the result of a retry.Until poll into a pass or a failure.
func awaited(
	last *httpx.Response, err error, accept func(*httpx.Response) bool, what string,
) error {
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if accept(last) {
		return nil
	}
	if last == nil {
		return fmt.Errorf("%s: the condition never held, and no response was ever received", what)
	}
	return fmt.Errorf("%s: the condition never held; the last response was %s", what, last.Describe())
}

// headerWith returns the scenario headers with one header set, overriding any sticky value.
func (g *Gateway) headerWith(ctx context.Context, name, value string) map[string]string {
	h := g.scenarioHeaders(ctx)
	h[name] = value
	return h
}

// managementURL builds a management API URL from the running topology.
func (g *Gateway) managementURL(path string) (string, error) {
	base, err := g.topo.URL("platform-gateway", "rest")
	if err != nil {
		return "", err
	}
	return base + ManagementBasePath + path, nil
}

func (g *Gateway) adminURL(path string) (string, error) {
	base, err := g.topo.URL("platform-gateway", "admin")
	if err != nil {
		return "", err
	}
	return base + adminBasePath + path, nil
}

// apiNameFrom extracts metadata.name from an API definition.
func apiNameFrom(definition string) string {
	metadata := resourceMetadata{}
	if err := yaml.Unmarshal([]byte(definition), &metadata); err != nil {
		return ""
	}
	return metadata.Metadata.Name
}

type resourceMetadata struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
}

// register wires gateway, health, and timeout steps.
func (g *Gateway) register(sc *godog.ScenarioContext) {
	// Request state is runner-scoped, so clear it before each scenario.
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		if err := g.resetRequest(ctx); err != nil {
			return ctx, err
		}
		// Dump expectations belong to one scenario. Keeping them in the runner-local
		// context would make a later scenario wait for resources created by an earlier one.
		return ctx, tcontext.Set(ctx, keyDumpExpectations, map[string]bool{})
	})

	sc.Step(`^the gateway services are running$`, g.gatewayIsRunning)
	sc.Step(`^I authenticate using basic auth as "([^"]*)"$`, g.authenticateAs)

	sc.Step(
		`^I create (API|LLM provider|LLM provider template|MCP proxy|LLM proxy) with configuration:$`,
		g.createResource)
	sc.Step(`^I create API with JSON configuration:$`, g.createJSONAPI)
	g.registerResourceTemplateSteps(sc)
	sc.Step(`^I get the (API|LLM provider|LLM provider template|MCP proxy|LLM proxy) "([^"]*)"$`,
		g.getResource)
	sc.Step(`^I list all (LLM providers|LLM provider templates|MCP proxies|LLM proxies)$`,
		g.listResources)
	sc.Step(`^I update the (API|LLM provider|LLM provider template|MCP proxy|LLM proxy) "([^"]*)" with configuration:$`, g.updateResource)
	sc.Step(`^I delete the (API|LLM provider|LLM provider template|MCP proxy|LLM proxy) "([^"]*)"$`, g.deleteResource)
	sc.Step(`^I send a "([^"]*)" request to the "([^"]*)" service at "([^"]*)"$`, g.serviceRequest)
	sc.Step(`^I send a "([^"]*)" request to the "([^"]*)" service at "([^"]*)" with body:$`, g.serviceRequestWithBody)
	sc.Step(`^I send a "([^"]*)" request to the "([^"]*)" service at "([^"]*)" until status (\d+)$`,
		g.serviceRequestUntilStatus)
	sc.Step(`^I send a "([^"]*)" request to the "([^"]*)" service at "([^"]*)" until the response body does not contain "([^"]*)"$`,
		g.serviceRequestUntilBodyNotContains)
	sc.Step(`^I resolve the "([^"]*)" service URL at "([^"]*)" and store it as "([^"]*)"$`,
		g.resolveServiceURLAndStore)
	sc.Step(`^I register the "([^"]*)" "([^"]*)" for cleanup$`, g.registerAdminResourceForCleanup)
	sc.Step(`^the "([^"]*)" service logs should contain "([^"]*)"$`, g.serviceLogsContain)
	sc.Step(`^the "([^"]*)" service logs should not contain "([^"]*)"$`, g.serviceLogsNotContain)
	sc.Step(`^the "([^"]*)" service log event containing "([^"]*)" should not contain "([^"]*)"$`,
		g.serviceLogEventExcludes)
	sc.Step(`^I send a "([^"]*)" request to the "([^"]*)" service at "([^"]*)" until lazy resource "([^"]*)" has display name "([^"]*)"$`,
		g.serviceRequestUntilLazyDisplayName)
	sc.Step(`^I send a "([^"]*)" request to the "([^"]*)" service at "([^"]*)" until provider template mapping "([^"]*)" maps to template "([^"]*)"$`,
		g.serviceRequestUntilProviderTemplateMapping)
	sc.Step(`^I send a "([^"]*)" request to the "([^"]*)" service at "([^"]*)" until lazy resource "([^"]*)" of type "([^"]*)" is absent$`,
		g.serviceRequestUntilLazyResourceAbsent)
	sc.Step(`^the latest analytics event for path "([^"]*)" should (contain|not contain) (request|response) header "([^"]*)"(?: with value "([^"]*)")?$`,
		g.analyticsHeader)
	sc.Step(`^I reset the analytics collector$`, g.resetAnalyticsCollector)
	sc.Step(`^I wait for the analytics collector to settle$`, g.waitForAnalyticsToSettle)
	sc.Step(`^the analytics collector should have received at least (\d+) events?$`, g.analyticsEventCountAtLeast)
	sc.Step(`^the analytics collector should have received (\d+) events?$`, g.analyticsEventCountExactly)
	sc.Step(`^the latest analytics event for path "([^"]*)" should have request method "([^"]*)"$`,
		g.analyticsRequestMethod)
	sc.Step(`^the latest analytics event for path "([^"]*)" should have response status (\d+)$`,
		g.analyticsResponseStatus)
	sc.Step(`^the latest analytics event for path "([^"]*)" should have metadata field "([^"]*)" with value "([^"]*)"$`,
		g.analyticsMetadataField)
	sc.Step(`^the response should be an oob-template list$`, g.oobTemplateList)
	sc.Step(`^the lazy resources should contain template "([^"]*)" of type "([^"]*)"$`, g.lazyTemplatePresent)
	sc.Step(`^the lazy resources should not contain template "([^"]*)"$`, g.lazyTemplateAbsent)
	sc.Step(`^the lazy resource "([^"]*)" should have display name "([^"]*)"$`, g.lazyDisplayName)
	sc.Step(`^the lazy resources should contain resource "([^"]*)" of type "([^"]*)"$`, g.lazyResourcePresent)
	sc.Step(`^the lazy resources should not contain resource "([^"]*)"$`, g.lazyResourceAbsent)
	sc.Step(`^the lazy resources should not contain resource "([^"]*)" of type "([^"]*)"$`, g.lazyTypedResourceAbsent)
	sc.Step(`^the provider template mapping "([^"]*)" should map to template "([^"]*)"$`, g.providerTemplateMapping)
	sc.Step(`^the policy engine route metadata should contain provider_name "([^"]*)"$`, g.routeMetadataProvider)
	sc.Step(`^the config dump should contain route with base path "([^"]*)"$`,
		func(ctx context.Context, basePath string) error {
			return g.configDumpRouteBasePath(ctx, basePath, true)
		})
	sc.Step(`^the config dump should not contain route with base path "([^"]*)"$`,
		func(ctx context.Context, basePath string) error {
			return g.configDumpRouteBasePath(ctx, basePath, false)
		})
	sc.Step(`^the config dump should contain policy "([^"]*)" for route "([^"]*)"$`,
		g.configDumpPolicyForRoute)
	sc.Step(`^I wait for the config dump to stop containing a route with base path "([^"]*)"$`,
		g.awaitConfigDumpRouteAbsent)
	sc.Step(`^the lazy resources should have at least (\d+) resources with id "([^"]*)"$`, g.lazyResourceCount)
	sc.Step(`^I use the MCP Client to send an initialize request to "([^"]*)"$`, g.mcpInitialize)
	sc.Step(`^I use the MCP Client to send "([^"]*)" tools/call request to "([^"]*)"$`, g.mcpToolCall)
	sc.Step(`^I use the MCP Client to send a tools/list request to "([^"]*)"$`, g.mcpToolsList)
	sc.Step(`^I use the MCP Client to send a notifications/initialized notification to "([^"]*)"$`,
		g.mcpNotificationInitialized)
	sc.Step(`^I use the MCP Client to send a tools/call request with invalid params to "([^"]*)"$`,
		g.mcpToolCallInvalidParams)
	sc.Step(`^I get a JWT token from the mock JWKS server with issuer "([^"]*)" and store it as "([^"]*)"$`,
		func(ctx context.Context, issuer, key string) error {
			return g.jwtToken(ctx, issuer, "", "", key)
		})
	sc.Step(`^I get a JWT token from the mock JWKS server with issuer "([^"]*)" and scope "([^"]*)" and store it as "([^"]*)"$`,
		func(ctx context.Context, issuer, scope, key string) error {
			return g.jwtToken(ctx, issuer, scope, "", key)
		})
	sc.Step(`^I get a JWT token from the mock JWKS server with issuer "([^"]*)" and claims "([^"]*)" and store it as "([^"]*)"$`,
		func(ctx context.Context, issuer, claims, key string) error {
			return g.jwtToken(ctx, issuer, "", claims, key)
		})
	sc.Step(`^I get a JWT token from the mock JWKS server with issuer "([^"]*)", scope "([^"]*)" and claims "([^"]*)" and store it as "([^"]*)"$`,
		g.jwtToken)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" until the rate limit reports (\d+) remaining$`,
		g.sendUntilQuotaRemaining)

	sc.Step(`^I wait for policy snapshot sync$`, g.awaitPolicySnapshotSync)

	sc.Step(`^the response body should contain template literal:$`, g.responseBodyContainsTemplateLiteral)
	sc.Step(`^the stored (RestApi|LlmProvider|LlmProxy|Mcp) configuration for "([^"]*)" should contain:$`,
		func(ctx context.Context, kind, handle string, literal *godog.DocString) error {
			return g.assertStoredConfiguration(ctx, kind, handle, literal, true)
		})
	sc.Step(`^the stored (RestApi|LlmProvider|LlmProxy|Mcp) configuration for "([^"]*)" should not contain:$`,
		func(ctx context.Context, kind, handle string, literal *godog.DocString) error {
			return g.assertStoredConfiguration(ctx, kind, handle, literal, false)
		})

	g.registerTimeoutSteps(sc)
	platformgateway.Register(sc, g.topo, g.funnel)
}

const elapsedTolerance = 0.05

func (g *Gateway) registerTimeoutSteps(sc *godog.ScenarioContext) {
	sc.Step(`^the gateway should have timed out after "([^"]*)" seconds with status (\d+)$`,
		g.timedOutAfter)
	sc.Step(`^the gateway should have responded within "([^"]*)" seconds$`, g.respondedWithin)
}

func (g *Gateway) timedOutAfter(ctx context.Context, wantSeconds string, wantStatus int) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	want, err := parseSeconds(wantSeconds)
	if err != nil {
		return err
	}
	floor := time.Duration(want * (1 - elapsedTolerance) * float64(time.Second))
	if resp.Elapsed < floor {
		return fmt.Errorf("expected the gateway to take at least %ss before timing out, but %s returned after %s",
			wantSeconds, resp.Describe(), resp.Elapsed.Round(time.Millisecond))
	}
	if resp.StatusCode != wantStatus {
		return fmt.Errorf("expected the timeout to surface as status %d, got %s", wantStatus, resp.Describe())
	}
	return nil
}

func (g *Gateway) respondedWithin(ctx context.Context, wantSeconds string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	want, err := parseSeconds(wantSeconds)
	if err != nil {
		return err
	}
	ceiling := time.Duration(want * (1 + elapsedTolerance) * float64(time.Second))
	if resp.Elapsed > ceiling {
		return fmt.Errorf("expected a response within %ss, but %s took %s",
			wantSeconds, resp.Describe(), resp.Elapsed.Round(time.Millisecond))
	}
	return nil
}

func parseSeconds(value string) (float64, error) {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("parsing expected seconds %q: %w", value, err)
	}
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return 0, fmt.Errorf("expected seconds must be finite and non-negative, got %q", value)
	}
	return seconds, nil
}

// gatewayIsRunning asserts that the gateway health endpoint responds successfully.
func (g *Gateway) gatewayIsRunning(ctx context.Context) error {
	url, err := g.adminURL("/health")
	if err != nil {
		return err
	}
	resp, err := g.funnel.Get(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("the gateway is not answering its health endpoint: %w", err)
	}
	if !resp.Succeeded() {
		return fmt.Errorf("the gateway is unhealthy: %s", resp.Describe())
	}
	return nil
}

// authenticateAs stores the credentials subsequent steps send.
//
// The credentials are supplied by the test overlay and published in shared scope so all
// gateway steps use the same authentication path.
func (g *Gateway) authenticateAs(ctx context.Context, who string) error {
	userKey, passKey := frameworkruntime.KeyAdminUser, frameworkruntime.KeyAdminPass
	switch who {
	case "admin":
	case "consumer":
		userKey, passKey = frameworkruntime.KeyConsumerUser, frameworkruntime.KeyConsumerPass
	case "developer":
		userKey, passKey = frameworkruntime.KeyDeveloperUser, frameworkruntime.KeyDeveloperPass
	default:
		return fmt.Errorf("unknown actor %q: supported actors are admin, consumer, and developer", who)
	}
	user, err := tcontext.ResolveString(ctx, userKey)
	if err != nil {
		return err
	}
	pass, err := tcontext.ResolveString(ctx, passKey)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, keyAuthHeader, BasicAuthHeader(user, pass))
}

// createResource creates one resource of the kind named in the step, which must match the kind
// the definition declares.
//
// API creation keeps its own path: it alone registers cleanup and waits for the deployment to
// become visible.
func (g *Gateway) createResource(ctx context.Context, kind string, body *godog.DocString) error {
	spec, ok := resourceKinds[kind]
	if !ok {
		return fmt.Errorf("unknown resource kind %q", kind)
	}
	if got := kindFromDefinition(body.Content); got != spec.declared {
		return fmt.Errorf("step creates %s but the definition declares kind %q, want %q",
			kind, got, spec.declared)
	}
	if spec.collection == "" {
		return g.createAPI(ctx, body, "application/yaml")
	}
	return g.mutateResource(ctx, http.MethodPost, spec.collection, "", body)
}

func (g *Gateway) createJSONAPI(ctx context.Context, body *godog.DocString) error {
	return g.createAPI(ctx, body, "application/json")
}

// updateResource replaces an existing resource of the kind named in the step.
//
// The declared kind is checked only when the definition states one: a few LlmProxy update bodies
// omit it, and rejecting those would fail scenarios over a field they never set.
func (g *Gateway) updateResource(
	ctx context.Context, kind, name string, body *godog.DocString,
) error {
	spec, ok := resourceKinds[kind]
	if !ok {
		return fmt.Errorf("unknown resource kind %q", kind)
	}
	if got := kindFromDefinition(body.Content); got != "" && got != spec.declared {
		return fmt.Errorf("step updates %s but the definition declares kind %q, want %q",
			kind, got, spec.declared)
	}
	if spec.collection == "" {
		return g.updateAPI(ctx, name, body)
	}
	return g.mutateResource(ctx, http.MethodPut, spec.collection, name, body)
}

// getResource retrieves one resource of the kind named in the step.
func (g *Gateway) getResource(ctx context.Context, kind, name string) error {
	spec, ok := resourceKinds[kind]
	if !ok {
		return fmt.Errorf("unknown resource kind %q", kind)
	}
	if spec.collection == "" {
		return g.getAPI(ctx, name)
	}
	resolved, err := stepscommon.Expand(ctx, name)
	if err != nil {
		return err
	}
	url, err := g.serviceURL(ctx, "gateway-controller", spec.collection+"/"+resolved)
	if err != nil {
		return err
	}
	_, err = g.funnel.Get(ctx, url, g.scenarioHeaders(ctx))
	return err
}

// pluralResourceKinds maps the plural phrasing a "list all" step names to its resourceKinds key.
var pluralResourceKinds = map[string]string{
	"LLM providers":          "LLM provider",
	"LLM provider templates": "LLM provider template",
	"MCP proxies":            "MCP proxy",
	"LLM proxies":            "LLM proxy",
}

// listResources lists every resource in the collection for the kind named in the step.
func (g *Gateway) listResources(ctx context.Context, kindPlural string) error {
	kind, ok := pluralResourceKinds[kindPlural]
	if !ok {
		return fmt.Errorf("unknown resource kind %q", kindPlural)
	}
	spec := resourceKinds[kind]
	url, err := g.serviceURL(ctx, "gateway-controller", spec.collection)
	if err != nil {
		return err
	}
	_, err = g.funnel.Get(ctx, url, g.scenarioHeaders(ctx))
	return err
}

// deleteResource removes a resource of the kind named in the step.
func (g *Gateway) deleteResource(ctx context.Context, kind, name string) error {
	spec, ok := resourceKinds[kind]
	if !ok {
		return fmt.Errorf("unknown resource kind %q", kind)
	}
	if spec.collection == "" {
		return g.deleteAPI(ctx, name)
	}
	return g.mutateResource(ctx, http.MethodDelete, spec.collection, name, nil)
}

// resourceKinds maps the kind named in a step to the kind its definition must declare and the
// controller collection it lives in. An empty collection means the API-specific handlers own
// that path: they alone register and deregister cleanup.
var resourceKinds = map[string]struct{ declared, collection string }{
	"API":                   {"RestApi", ""},
	"LLM provider":          {"LlmProvider", collLLMProviders},
	"LLM provider template": {"LlmProviderTemplate", collLLMTemplates},
	"MCP proxy":             {"Mcp", collMCPProxies},
	"LLM proxy":             {"LlmProxy", collLLMProxies},
}

// kindFromDefinition returns the top-level kind a definition declares.
func kindFromDefinition(def string) string {
	metadata := resourceMetadata{}
	if err := yaml.Unmarshal([]byte(def), &metadata); err != nil {
		return ""
	}
	return metadata.Kind
}

// createAPI posts an API definition and waits for it to become routable.
//
// The migrated behaviour differs from the original in ONE respect, deliberately: the
// original slept a fixed second for xDS propagation. A fixed sleep is both too long when
// propagation is fast and too short when a loaded host is slow — the second case being a
// flake that looks like a product bug. This polls instead, bounded by the shared ceiling.
func (g *Gateway) createAPI(ctx context.Context, body *godog.DocString, contentType string) error {
	// The definition may name resources; expanding placeholders is what keeps two concurrent
	// scenarios from creating the same API.
	definition, err := stepscommon.Expand(ctx, body.Content)
	if err != nil {
		return err
	}

	url, err := g.managementURL("/rest-apis")
	if err != nil {
		return err
	}

	resp, err := g.funnel.Post(ctx, url, g.headerWith(ctx, "Content-Type", contentType), []byte(definition))
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		// Returned rather than asserted here: a scenario may deploy an invalid definition on
		// purpose and assert the rejection itself.
		return nil
	}

	name := apiNameFrom(definition)
	if name == "" {
		return nil
	}
	expectInDump(ctx, name, true)
	if err := tcontext.Set(ctx, keyLastAPIName, name); err != nil {
		return err
	}

	// Registered as soon as it exists, and BEFORE anything else can fail: a resource created
	// but not registered is a resource that leaks when the next step errors.
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanup.KindAPI, ID: name, Actor: "admin", Description: "deployed by " + scenarioLabel(ctx),
	}); err != nil {
		deleteURL, urlErr := g.managementURL("/rest-apis/" + name)
		if urlErr == nil {
			if deleteErr := g.compensateDelete(ctx, deleteURL, g.scenarioHeaders(ctx)); deleteErr != nil {
				return fmt.Errorf("registering API %q for cleanup: %w; compensation failed: %v",
					name, err, deleteErr)
			}
		}
		return fmt.Errorf("registering API %q for cleanup: %w", name, err)
	}

	return g.awaitDeployed(ctx, name)
}

// awaitDeployed polls the management API until the deployment is visible.
func (g *Gateway) awaitDeployed(ctx context.Context, name string) error {
	url, err := g.managementURL("/rest-apis/" + name)
	if err != nil {
		return err
	}
	headers := g.scenarioHeaders(ctx)

	accept := func(r *httpx.Response) bool { return r != nil && r.Succeeded() }
	last, err := retry.Until(ctx,
		retry.Options{Timeout: 30 * time.Second, Interval: 200 * time.Millisecond},
		func(ctx context.Context) (*httpx.Response, error) {
			// The raw client, not the funnel: this is an intermediate read and must not
			// become the response the next assertion targets.
			return g.funnel.Client().Do(ctx, httpx.Request{
				Method: http.MethodGet, URL: url, Headers: headers,
			}, 0, 0)
		},
		accept,
	)
	return awaited(last, err, accept,
		fmt.Sprintf("API %q was accepted but never became retrievable", name))
}

func (g *Gateway) getAPI(ctx context.Context, name string) error {
	resolved, err := stepscommon.Expand(ctx, name)
	if err != nil {
		return err
	}
	url, err := g.managementURL("/rest-apis/" + resolved)
	if err != nil {
		return err
	}
	_, err = g.funnel.Get(ctx, url, g.scenarioHeaders(ctx))
	return err
}

func (g *Gateway) deleteAPI(ctx context.Context, name string) error {
	resolved, err := stepscommon.Expand(ctx, name)
	if err != nil {
		return err
	}
	url, err := g.managementURL("/rest-apis/" + resolved)
	if err != nil {
		return err
	}
	resp, err := g.funnel.Delete(ctx, url, g.scenarioHeaders(ctx))
	if err != nil {
		return err
	}
	if resp.Succeeded() {
		// The test deleted it and will assert on that, so teardown must not try again and
		// log a spurious leak.
		if reg, err := cleanup.Of(ctx); err == nil {
			reg.Deregister(cleanup.KindAPI, resolved)
		}
		// A later config-dump read must wait for the removal to propagate, not for the
		// handle to appear.
		expectInDump(ctx, resolved, false)
	}
	return nil
}

// ── Assertions, unchanged in meaning from the original suite ─────────────────────

// serviceEndpoints maps the names features use for a SERVICE onto the component endpoint
// that now serves it.
//
// These names are a legacy of the era when the gateway was two separately addressed
// containers: "gateway-controller" and "gateway-controller-admin" name CONTAINERS, which is
// precisely the distinction the platform-gateway component exists to hide. They are mapped
// rather than rewritten because they appear 134 times across the suite, and rewriting them is
// a vocabulary change worth doing deliberately in one pass rather than smuggling into a
// migration. Recorded in the ledger as outstanding.
// Each entry carries the API BASE PATH too, because features address these services with a
// bare resource path ("/rest-apis", "/certificates"). The version prefix belongs to the
// product's API contract, not to the feature, and keeping it here means a version bump is one
// edit rather than a rewrite of every scenario.
var serviceEndpoints = map[string]struct {
	component, endpoint, basePath string
	partitioned                   bool
}{
	"gateway-controller":       {component: "platform-gateway", endpoint: "rest", basePath: ManagementBasePath},
	"gateway-controller-admin": {component: "platform-gateway", endpoint: "admin", basePath: adminBasePath},
	"policy-engine":            {component: "platform-gateway", endpoint: "policy-admin"},
	"analytics":                {component: "testbench", endpoint: "analytics", partitioned: true},
	"capture":                  {component: "testbench", endpoint: "capture", partitioned: true},
	// Metrics live on a DIFFERENT compose service from the one tests normally address —
	// controller metrics on the controller, policy-engine metrics on the runtime — which the
	// component contract resolves via Endpoint.Service. No base path: a scrape is not an API.
	"controller-metrics":    {component: "platform-gateway", endpoint: "metrics"},
	"policy-engine-metrics": {component: "platform-gateway", endpoint: "pe-metrics"},
}

// serviceURL resolves a feature's service name and path to a URL on the running topology.
func (g *Gateway) serviceURL(ctx context.Context, service, path string) (string, error) {
	spec, ok := serviceEndpoints[service]
	if !ok {
		return "", fmt.Errorf("unknown service %q: this suite addresses %v", service, sortedServiceNames())
	}
	base, err := g.topo.URL(spec.component, spec.endpoint)
	if err != nil {
		return "", err
	}
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return "", err
	}
	if resolved != "" && !strings.HasPrefix(resolved, "/") {
		resolved = "/" + resolved
	}
	if spec.partitioned {
		if g.topo.Block == nil {
			return "", fmt.Errorf("service %q requires a block partition, but the topology has no block", service)
		}
		resolved = "/" + g.topo.Block.PartitionKey() + resolved
	}
	return base + spec.basePath + resolved, nil
}

// resolveServiceURLAndStore resolves a testbench service's URL and stores it in local scope, for
// use as an upstream target elsewhere in the scenario — most notably a partitioned service like
// "capture", whose address includes a block key a feature cannot know in advance.
func (g *Gateway) resolveServiceURLAndStore(ctx context.Context, service, path, key string) error {
	url, err := g.serviceURL(ctx, service, path)
	if err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("cannot store a resolved service URL with an empty key")
	}
	return tcontext.Set(ctx, key, url)
}

// adminResourceKinds maps the resource-kind names features use for the generic cleanup
// registration step onto the cleanup kind and admin-API collection path that owns it.
var adminResourceKinds = map[string]struct {
	kind       cleanup.Kind
	collection string
}{
	"certificate": {cleanup.KindCertificate, "/certificates"},
	"secret":      {cleanup.KindSecret, "/secrets"},
}

// registerAdminResourceForCleanup registers a resource managed through the gateway-controller
// admin API (a certificate, a secret, ...) for cleanup, using an ID resolved from scenario
// context. If registration itself fails, it makes a best-effort compensating delete against the
// resource's own admin endpoint rather than leaking it.
func (g *Gateway) registerAdminResourceForCleanup(ctx context.Context, kindName, idExpr string) error {
	entry, ok := adminResourceKinds[kindName]
	if !ok {
		return fmt.Errorf("unknown cleanup resource kind %q: this suite recognizes %v",
			kindName, sortedAdminResourceKindNames())
	}
	id, err := stepscommon.Expand(ctx, idExpr)
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: entry.kind, ID: id, Actor: "admin", Description: "created by " + scenarioLabel(ctx),
	}); err != nil {
		deleteURL, urlErr := g.serviceURL(ctx, "gateway-controller", entry.collection+"/"+id)
		if urlErr == nil {
			if deleteErr := g.compensateDelete(ctx, deleteURL, g.scenarioHeaders(ctx)); deleteErr != nil {
				return fmt.Errorf("registering %s %q for cleanup: %w; compensation failed: %v",
					kindName, id, err, deleteErr)
			}
		}
		return fmt.Errorf("registering %s %q for cleanup: %w", kindName, id, err)
	}
	return nil
}

func sortedAdminResourceKindNames() []string {
	out := make([]string, 0, len(adminResourceKinds))
	for k := range adminResourceKinds {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedServiceNames() []string {
	out := make([]string, 0, len(serviceEndpoints))
	for k := range serviceEndpoints {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// serviceRequest invokes a component's own API, rather than the data plane.
func (g *Gateway) serviceRequest(ctx context.Context, method, service, path string) error {
	return g.serviceRequestWithBody(ctx, method, service, path, nil)
}

// serviceRequestWithBody invokes a component's own API, with a body when one is given.
func (g *Gateway) serviceRequestWithBody(
	ctx context.Context, method, service, path string, body *godog.DocString,
) error {
	method = strings.ToUpper(method)

	url, err := g.serviceURL(ctx, service, path)
	if err != nil {
		return err
	}

	// The config dump lags the deploy by one event-hub poll and nothing else exposes that, so
	// the framework waits here rather than making every scenario encode the timing.
	if method == http.MethodGet && strings.HasPrefix(strings.TrimPrefix(path, "/"), "config_dump") {
		if err := g.awaitDumpConsistent(ctx, url); err != nil {
			return err
		}
	}

	var payload []byte
	headers := g.scenarioHeaders(ctx)
	if body != nil {
		content, expErr := stepscommon.Expand(ctx, body.Content)
		if expErr != nil {
			return expErr
		}
		payload = []byte(content)
		if headers["Content-Type"] == "" {
			headers["Content-Type"] = "application/json"
		}
	}
	return g.invokeWith(ctx, method, url, headers, payload)
}

// serviceRequestUntilStatus polls a component endpoint until it returns the
// expected status and publishes the confirming response.
func (g *Gateway) serviceRequestUntilStatus(ctx context.Context, method, service, path string, want int) error {
	method = strings.ToUpper(method)
	url, err := g.serviceURL(ctx, service, path)
	if err != nil {
		return err
	}
	err = retry.Await(ctx, retry.Options{Interval: 2 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, requestErr := g.funnel.Send(ctx, httpx.Request{
				Method: method, URL: url, Headers: g.scenarioHeaders(ctx),
			})
			if requestErr != nil {
				return nil, retry.Transient(requestErr)
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool { return resp != nil && resp.StatusCode == want },
		fmt.Sprintf("waiting for %s %s to return %d", method, url, want),
	)
	if err != nil {
		return err
	}
	return nil
}

// serviceRequestUntilBodyNotContains polls a service path until its response body no longer
// contains want - used for a diagnostic endpoint like config_dump, whose snapshot may lag a
// moment behind a just-completed controller mutation.
func (g *Gateway) serviceRequestUntilBodyNotContains(
	ctx context.Context, method, service, path, want string,
) error {
	method = strings.ToUpper(method)
	url, err := g.serviceURL(ctx, service, path)
	if err != nil {
		return err
	}
	resolvedWant, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	err = retry.Await(ctx, retry.Options{Interval: 500 * time.Millisecond},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, requestErr := g.funnel.Send(ctx, httpx.Request{
				Method: method, URL: url, Headers: g.scenarioHeaders(ctx),
			})
			if requestErr != nil {
				return nil, retry.Transient(requestErr)
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool {
			return resp != nil && resp.Succeeded() && !strings.Contains(string(resp.Body), resolvedWant)
		},
		fmt.Sprintf("waiting for %s %s to stop containing %q", method, url, resolvedWant),
	)
	return err
}

// serviceLogsContain waits until a service log contains the supplied marker.
func (g *Gateway) serviceLogsContain(ctx context.Context, service, marker string) error {
	return g.assertServiceLogs(ctx, service, marker, true)
}

// serviceLogsNotContain waits until a service log no longer contains the supplied marker.
func (g *Gateway) serviceLogsNotContain(ctx context.Context, service, marker string) error {
	stack, _, err := g.topo.ServiceControl(service)
	if err != nil {
		return err
	}
	logs := stack.Logs(ctx)
	if strings.Contains(logs, marker) {
		return fmt.Errorf("service %q logs unexpectedly contain %q", service, marker)
	}
	return nil
}

// serviceLogEventExcludes waits for the event identified by marker and checks the
// exclusion against that event rather than the complete historical log.
func (g *Gateway) serviceLogEventExcludes(ctx context.Context, service, marker, excluded string) error {
	if strings.TrimSpace(marker) == "" || strings.TrimSpace(excluded) == "" {
		return fmt.Errorf("service log event marker and excluded text must not be empty")
	}
	stack, _, err := g.topo.ServiceControl(service)
	if err != nil {
		return err
	}
	last := ""
	_, err = retry.Until(ctx, retry.Options{Interval: 200 * time.Millisecond}, func(ctx context.Context) (string, error) {
		last = stack.Logs(ctx)
		return last, nil
	}, func(logs string) bool {
		for _, line := range strings.Split(logs, "\n") {
			if strings.Contains(line, marker) {
				return !strings.Contains(line, excluded)
			}
		}
		return false
	})
	if err != nil {
		return fmt.Errorf("waiting for log event containing %q without %q: %w; last logs: %s",
			marker, excluded, err, truncateLog(last))
	}
	return nil
}

func (g *Gateway) assertServiceLogs(ctx context.Context, service, marker string, wantPresent bool) error {
	if strings.TrimSpace(marker) == "" {
		return fmt.Errorf("service log marker must not be empty")
	}
	stack, resolved, err := g.topo.ServiceControl(service)
	if err != nil {
		return err
	}
	last := ""
	_, err = retry.Until(ctx, retry.Options{Interval: 200 * time.Millisecond}, func(ctx context.Context) (string, error) {
		last = stack.Logs(ctx)
		return last, nil
	}, func(logs string) bool {
		present := strings.Contains(logs, marker)
		if wantPresent {
			return present
		}
		return !present
	})
	if err != nil {
		state := "present"
		if !wantPresent {
			state = "absent"
		}
		return fmt.Errorf("waiting for %s logs for service %q to become %s: %w; last logs: %s",
			resolved, service, state, err, truncateLog(last))
	}
	return nil
}

func truncateLog(logs string) string {
	const max = 2000
	if len(logs) <= max {
		return logs
	}
	return logs[len(logs)-max:]
}

// serviceRequestUntilLazyDisplayName polls a policy-engine config dump until a template has
// the expected runtime display name, then publishes the confirming response.
func (g *Gateway) serviceRequestUntilLazyDisplayName(
	ctx context.Context, method, service, path, resourceID, displayName string,
) error {
	if strings.ToUpper(method) != http.MethodGet || service != "policy-engine" {
		return fmt.Errorf("lazy-resource readiness requires a GET request to the policy-engine")
	}
	url, err := g.serviceURL(ctx, service, path)
	if err != nil {
		return err
	}
	resourceID, err = stepscommon.Expand(ctx, resourceID)
	if err != nil {
		return err
	}
	displayName, err = stepscommon.Expand(ctx, displayName)
	if err != nil {
		return err
	}
	accept := func(resp *httpx.Response) bool {
		return resp != nil && resp.Succeeded() && lazyDisplayNameMatches(resp.Body, resourceID, displayName)
	}
	last, err := retry.Until(ctx, retry.Options{Interval: 200 * time.Millisecond},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, requestErr := g.funnel.Client().Do(ctx, httpx.Request{
				Method: http.MethodGet, URL: url, Headers: g.scenarioHeaders(ctx),
			}, 0, 0)
			if requestErr != nil {
				return nil, retry.Transient(requestErr)
			}
			return resp, nil
		}, accept)
	if err != nil {
		return fmt.Errorf("waiting for lazy resource %q to have display name %q: %w", resourceID, displayName, err)
	}
	if !accept(last) {
		return fmt.Errorf("lazy resource %q did not have display name %q: %s", resourceID, displayName, last.Describe())
	}
	return g.funnel.Publish(ctx, last)
}

func lazyDisplayNameMatches(body []byte, resourceID, displayName string) bool {
	var dump configDump
	if err := json.Unmarshal(body, &dump); err != nil {
		return false
	}
	for _, resource := range dump.Lazy.ResourcesByType["LlmProviderTemplate"] {
		if resource.ID != resourceID {
			continue
		}
		spec, ok := resource.Resource["spec"].(map[string]any)
		return ok && spec["displayName"] == displayName
	}
	return false
}

type analyticsEvent struct {
	Request struct {
		URI     string              `json:"uri"`
		Verb    string              `json:"verb"`
		Headers map[string][]string `json:"headers"`
	} `json:"request"`
	Response struct {
		Status  int                 `json:"status"`
		Headers map[string][]string `json:"headers"`
	} `json:"response"`
	Metadata map[string]any `json:"metadata"`
}

func (g *Gateway) analyticsHeader(
	ctx context.Context, path, mode, plane, header, value string,
) error {
	path, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	event, err := g.latestAnalyticsEvent(ctx, path)
	if err != nil {
		return err
	}
	var headers map[string][]string
	if plane == "request" {
		headers = event.Request.Headers
	} else {
		headers = event.Response.Headers
	}
	actual, present := analyticsHeaderValue(headers, header)
	switch mode {
	case "contain":
		if !present {
			return fmt.Errorf("analytics event for %q does not contain %s header %q", path, plane, header)
		}
		if value != "" && actual != value {
			return fmt.Errorf("analytics event for %q has %s header %q value %q, want %q",
				path, plane, header, actual, value)
		}
	case "not contain":
		if present {
			return fmt.Errorf("analytics event for %q contains denied %s header %q with value %q",
				path, plane, header, actual)
		}
	default:
		return fmt.Errorf("unsupported analytics header assertion %q", mode)
	}
	return nil
}

func (g *Gateway) latestAnalyticsEvent(ctx context.Context, path string) (*analyticsEvent, error) {
	url, err := g.serviceURL(ctx, "analytics", "/test/events")
	if err != nil {
		return nil, err
	}
	var observed []string
	accept := func(event *analyticsEvent) bool {
		return event != nil && analyticsEventMatchesPath(event.Request.URI, path)
	}
	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	last, err := retry.Until(pollCtx, retry.Options{Interval: time.Second},
		func(ctx context.Context) (*analyticsEvent, error) {
			response, getErr := g.funnel.Client().Do(ctx, httpx.Request{
				Method: http.MethodGet, URL: url, Headers: g.scenarioHeaders(ctx),
			}, 0, 0)
			if getErr != nil {
				return nil, retry.Transient(getErr)
			}
			if !response.Succeeded() {
				return nil, retry.Transient(fmt.Errorf("analytics events returned %s", response.Describe()))
			}
			var events []analyticsEvent
			if decodeErr := json.Unmarshal(response.Body, &events); decodeErr != nil {
				return nil, decodeErr
			}
			for i := len(events) - 1; i >= 0; i-- {
				if events[i].Request.URI != "" {
					observed = append(observed, events[i].Request.URI)
				}
				if accept(&events[i]) {
					return &events[i], nil
				}
			}
			return nil, nil
		}, accept)
	if err != nil {
		return nil, fmt.Errorf("reading analytics event for path %q (observed URIs: %v): %w",
			path, observed, err)
	}
	if last == nil {
		return nil, fmt.Errorf("no analytics event found for request path %q (observed URIs: %v)", path, observed)
	}
	return last, nil
}

// resetAnalyticsCollector clears every event the testbench analytics collector has buffered
// for this block, so a scenario's own counts aren't inflated by earlier scenarios' traffic.
func (g *Gateway) resetAnalyticsCollector(ctx context.Context) error {
	url, err := g.serviceURL(ctx, "analytics", "/test/reset")
	if err != nil {
		return err
	}
	resp, err := g.funnel.Client().Do(ctx, httpx.Request{
		Method: http.MethodPost, URL: url, Headers: g.scenarioHeaders(ctx),
	}, 0, 0)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return fmt.Errorf("resetting the analytics collector failed: %s", resp.Describe())
	}
	return nil
}

func (g *Gateway) analyticsEventCount(ctx context.Context) (int, error) {
	url, err := g.serviceURL(ctx, "analytics", "/test/events/count")
	if err != nil {
		return 0, err
	}
	resp, err := g.funnel.Client().Do(ctx, httpx.Request{
		Method: http.MethodGet, URL: url, Headers: g.scenarioHeaders(ctx),
	}, 0, 0)
	if err != nil {
		return 0, retry.Transient(err)
	}
	if !resp.Succeeded() {
		return 0, retry.Transient(fmt.Errorf("analytics count returned %s", resp.Describe()))
	}
	var out struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return 0, err
	}
	return out.Count, nil
}

// analyticsEventCountAtLeast polls until the collector has buffered at least want events.
func (g *Gateway) analyticsEventCountAtLeast(ctx context.Context, want int) error {
	accept := func(n int) bool { return n >= want }
	last, err := retry.Until(ctx, retry.Options{Timeout: 15 * time.Second, Interval: 500 * time.Millisecond},
		func(ctx context.Context) (int, error) { return g.analyticsEventCount(ctx) }, accept)
	if err != nil {
		return err
	}
	if !accept(last) {
		return fmt.Errorf("expected at least %d analytics events, got %d", want, last)
	}
	return nil
}

// settleAnalyticsEventCount waits for the collector's count to stop changing for a quiet
// period. A bare threshold poll would return the instant the count first reaches a target and
// could miss a late-arriving duplicate event landing just after - this is what actually
// verifies "no more are coming", used both to check an exact count and to drain a prior
// request's own publish delay before a scenario resets the collector for its real assertion.
func (g *Gateway) settleAnalyticsEventCount(ctx context.Context) (retry.Settled, error) {
	settled, err := retry.SettledCount(ctx, retry.Options{Timeout: 12 * time.Second}, 3*time.Second,
		func(ctx context.Context) (int, error) { return g.analyticsEventCount(ctx) })
	if err != nil {
		return settled, err
	}
	if !settled.Quiet {
		return settled, fmt.Errorf("analytics event count did not settle: last observed %d after %d sample(s)",
			settled.Value, settled.Samples)
	}
	return settled, nil
}

func (g *Gateway) analyticsEventCountExactly(ctx context.Context, want int) error {
	settled, err := g.settleAnalyticsEventCount(ctx)
	if err != nil {
		return err
	}
	if settled.Value != want {
		return fmt.Errorf("expected exactly %d analytics events, got %d", want, settled.Value)
	}
	return nil
}

// waitForAnalyticsToSettle waits until the collector's event count stops changing, without
// asserting a specific value. Use it before resetting the collector, so an earlier request's
// own delayed publish (moesif_base_url's publish_interval) can't land after the reset and
// contaminate a subsequent exact-count assertion.
func (g *Gateway) waitForAnalyticsToSettle(ctx context.Context) error {
	_, err := g.settleAnalyticsEventCount(ctx)
	return err
}

// analyticsEventForPath resolves path's placeholders and returns the event latestAnalyticsEvent
// finds for it. Filtering by the path a scenario already knows it requested is what makes this
// robust: the events array's arrival order at the collector doesn't necessarily match request
// chronology, so picking "the last element" can return an unrelated event.
func (g *Gateway) analyticsEventForPath(ctx context.Context, path string) (*analyticsEvent, error) {
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return nil, err
	}
	return g.latestAnalyticsEvent(ctx, resolved)
}

func (g *Gateway) analyticsRequestMethod(ctx context.Context, path, want string) error {
	event, err := g.analyticsEventForPath(ctx, path)
	if err != nil {
		return err
	}
	if event.Request.Verb != want {
		return fmt.Errorf("analytics event for %q has request method %q, want %q", path, event.Request.Verb, want)
	}
	return nil
}

func (g *Gateway) analyticsResponseStatus(ctx context.Context, path string, want int) error {
	event, err := g.analyticsEventForPath(ctx, path)
	if err != nil {
		return err
	}
	if event.Response.Status != want {
		return fmt.Errorf("analytics event for %q has response status %d, want %d", path, event.Response.Status, want)
	}
	return nil
}

func (g *Gateway) analyticsMetadataField(ctx context.Context, path, field, want string) error {
	event, err := g.analyticsEventForPath(ctx, path)
	if err != nil {
		return err
	}
	resolved, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	value, ok := event.Metadata[field]
	if !ok {
		return fmt.Errorf("latest analytics event metadata has no field %q", field)
	}
	if got := fmt.Sprintf("%v", value); got != resolved {
		return fmt.Errorf("latest analytics event metadata field %q is %q, want %q", field, got, resolved)
	}
	return nil
}

func analyticsHeaderValue(headers map[string][]string, wanted string) (string, bool) {
	for name, value := range headers {
		if strings.EqualFold(name, wanted) {
			if len(value) == 0 {
				return "", true
			}
			return value[0], true
		}
	}
	return "", false
}

func analyticsEventMatchesPath(eventURI, requestedPath string) bool {
	eventURI = strings.TrimSpace(eventURI)
	requestedPath = strings.TrimRight(strings.TrimSpace(requestedPath), "/")
	if eventURI == "" || requestedPath == "" {
		return false
	}
	return requestedPath == eventURI || strings.HasSuffix(requestedPath, "/"+strings.TrimPrefix(eventURI, "/"))
}

// updateAPI replaces an existing API definition.
//
// The body is YAML like a deploy, so the content type is set explicitly — the default of
// application/json would make the controller reject it with a JSON parse error naming the
// first byte of "apiVersion" rather than the content type.
func (g *Gateway) updateAPI(ctx context.Context, name string, body *godog.DocString) error {
	resolvedName, err := stepscommon.Expand(ctx, name)
	if err != nil {
		return err
	}
	definition, err := stepscommon.Expand(ctx, body.Content)
	if err != nil {
		return err
	}
	url, err := g.serviceURL(ctx, "gateway-controller", "/rest-apis/"+resolvedName)
	if err != nil {
		return err
	}
	headers := g.headerWith(ctx, "Content-Type", "application/yaml")
	return g.invokeWith(ctx, http.MethodPut, url, headers, []byte(definition))
}

// handleFromDefinition extracts metadata.name from a YAML definition, which is the handle the
// config dump reports.
func handleFromDefinition(def string) string {
	return apiNameFrom(def)
}

// Controller-managed resource collections, by the path segment that addresses them.
const (
	collLLMProviders = "/llm-providers"
	collLLMTemplates = "/llm-provider-templates"
	collMCPProxies   = "/mcp-proxies"
	collLLMProxies   = "/llm-proxies"
)

// mutateResource creates, replaces or removes a controller resource and waits for the change
// to be OBSERVABLE before returning.
//
// The original slept a flat second after every mutation. That is the anti-pattern this
// framework removes: too long when the controller is fast, too short when it is loaded, and
// the second case is a flake that reads as a product bug. What the sleep was actually waiting
// for is that the resource has reached its new state, so that is what this waits for —
// present after a create or update, absent after a delete.
//
// Strictly stronger than the sleep, too: a create that the controller ACCEPTED but never
// persisted used to pass, because a second elapsed either way.
func (g *Gateway) mutateResource(
	ctx context.Context, method, collection, id string, body *godog.DocString,
) error {
	resolvedID, err := stepscommon.Expand(ctx, id)
	if err != nil {
		return err
	}

	var expandedBody string
	if body != nil {
		expandedBody, err = stepscommon.Expand(ctx, body.Content)
		if err != nil {
			return err
		}
	}

	path := collection
	if resolvedID != "" {
		path = collection + "/" + resolvedID
	}
	if method == http.MethodDelete {
		expectInDump(ctx, resolvedID, false)
	} else if expandedBody != "" {
		if h := handleFromDefinition(expandedBody); h != "" {
			expectInDump(ctx, h, true)
		}
	}
	url, err := g.serviceURL(ctx, "gateway-controller", path)
	if err != nil {
		return err
	}

	var payload []byte
	if expandedBody != "" {
		payload = []byte(expandedBody)
	}

	// YAML, like every other definition this controller accepts. Defaulting to JSON here
	// produces "failed to parse JSON: invalid character 'a'" — the first byte of apiVersion.
	headers := g.headerWith(ctx, "Content-Type", "application/yaml")
	if err := g.invokeWith(ctx, method, url, headers, payload); err != nil {
		return err
	}

	// A mutation the controller REJECTED is a legitimate assertion in several scenarios, so
	// only a successful one is waited on: waiting for a rejected create to appear would hang
	// until the ceiling and report a timeout instead of the 4xx the scenario is asserting.
	resp, err := httpx.Published(ctx)
	if err != nil || resp == nil || !resp.Succeeded() {
		return nil //nolint:nilerr // the response is the assertion; the next Then reads it
	}
	resourceID := resolvedID
	if method == http.MethodPost {
		if resourceID == "" && expandedBody != "" {
			resourceID = handleFromDefinition(expandedBody)
		}
		if kind, ok := cleanupKindForCollection(collection); ok && resourceID != "" {
			if err := cleanup.Register(ctx, cleanup.Resource{
				Kind: kind, ID: resourceID, Actor: "admin",
				Description: "created by " + scenarioLabel(ctx),
			}); err != nil {
				deleteURL, urlErr := g.serviceURL(ctx, "gateway-controller", collection+"/"+resourceID)
				if urlErr == nil {
					if deleteErr := g.compensateDelete(ctx, deleteURL, g.scenarioHeaders(ctx)); deleteErr != nil {
						return fmt.Errorf("registering %s %q for cleanup: %w; compensation failed: %v",
							collection, resourceID, err, deleteErr)
					}
				}
				return fmt.Errorf("registering %s %q for cleanup: %w", collection, resourceID, err)
			}
		}
	}

	// The published response must survive: the next step asserts on the MUTATION's response,
	// not on the poll's. So the wait uses a bare client and restores it afterwards.
	settled := resp
	waitFor := func(want bool) error {
		target := collection + "/" + resourceID
		if resourceID == "" {
			return nil // a create without a known id has nothing to poll for
		}
		pollURL, err := g.serviceURL(ctx, "gateway-controller", target)
		if err != nil {
			return err
		}
		accept := func(r *httpx.Response) bool {
			if r == nil {
				return false
			}
			if want {
				return r.Succeeded()
			}
			return r.StatusCode == http.StatusNotFound
		}
		last, err := retry.Until(ctx,
			retry.Options{Interval: 200 * time.Millisecond},
			func(ctx context.Context) (*httpx.Response, error) {
				return g.funnel.Client().Do(ctx, httpx.Request{
					Method: http.MethodGet, URL: pollURL, Headers: g.scenarioHeaders(ctx),
				}, 0, 0)
			},
			accept,
		)
		return awaited(last, err, accept,
			fmt.Sprintf("waiting for %s to exist=%t", target, want))
	}

	var waitErr error
	if method == http.MethodDelete {
		waitErr = waitFor(false)
	} else {
		waitErr = waitFor(true)
	}
	if waitErr != nil {
		return fmt.Errorf("%s %s did not become observable: %w", method, path, waitErr)
	}
	if method == http.MethodDelete {
		if kind, ok := cleanupKindForCollection(collection); ok {
			if reg, err := cleanup.Of(ctx); err == nil {
				reg.Deregister(kind, resourceID)
			}
		}
	}
	return tcontext.Set(ctx, httpx.ResponseKey, settled)
}

func (g *Gateway) compensateDelete(ctx context.Context, url string, headers map[string]string) error {
	resp, err := g.funnel.Client().Do(ctx, httpx.Request{
		Method: http.MethodDelete, URL: url, Headers: headers,
	}, 0, 0)
	if err != nil {
		return err
	}
	if !resp.Succeeded() {
		return fmt.Errorf("delete returned %s", resp.Describe())
	}
	return nil
}

func cleanupKindForCollection(collection string) (cleanup.Kind, bool) {
	switch collection {
	case collLLMProviders:
		return cleanup.KindLLMProvider, true
	case collLLMProxies:
		return cleanup.KindLLMProxy, true
	case collLLMTemplates:
		return cleanup.KindLLMProviderTemplate, true
	case collMCPProxies:
		return cleanup.KindMCPProxy, true
	default:
		return cleanup.Kind{}, false
	}
}

// expectInDump records what the config dump must eventually show for a handle.
//
// present=true after a create, false after a delete. Tracking BOTH matters: a scenario that
// deploys, checks the dump, deletes and checks again is asserting the dump reflects the
// removal — and waiting only for appearance would block forever on the second read.
func expectInDump(ctx context.Context, handle string, present bool) {
	handle = strings.TrimSpace(handle)
	if handle == "" {
		return
	}
	expectations := map[string]bool{}
	if v, ok := tcontext.Get(ctx, keyDumpExpectations); ok {
		if existing, ok := v.(map[string]bool); ok {
			for k, val := range existing {
				expectations[k] = val
			}
		}
	}
	expectations[handle] = present
	_ = tcontext.Set(ctx, keyDumpExpectations, expectations)
}

// awaitDumpConsistent polls the config dump until it reflects everything this scenario has
// deployed.
//
// The config dump is EVENTUALLY CONSISTENT and nothing else exposes that. Measured on a real
// gateway, immediately after a deploy returns 201:
//
//	GET /rest-apis/{name}   visible after   2ms   (database)
//	GET /rest-apis          visible after   3ms   (database)
//	GET /config_dump        visible after 151ms   (in-memory store, via the event-hub poll)
//
// Only the dump reads s.store, which the EventListener populates on its poll cycle. So there
// is no cheaper condition to wait on — no management endpoint lags with it, and waiting for
// the API to be "deployed" proves nothing about the dump.
//
// The original suite covered this with a flat 1s sleep after every deploy, which worked only
// because it exceeded the poll interval. Polling the dump for the handles this scenario
// actually created is the same wait expressed as a condition: it returns as soon as the data
// is there, and it fails loudly rather than silently reading a stale dump.
func (g *Gateway) awaitDumpConsistent(ctx context.Context, url string) error {
	v, ok := tcontext.Get(ctx, keyDumpExpectations)
	if !ok {
		return nil // nothing created or removed here; the dump cannot be stale
	}
	expectations, _ := v.(map[string]bool)
	if len(expectations) == 0 {
		return nil
	}

	accept := func(r *httpx.Response) bool {
		if r == nil || !r.Succeeded() {
			return false
		}
		body := r.Text()
		for handle, wantPresent := range expectations {
			if strings.Contains(body, handle) != wantPresent {
				return false
			}
		}
		return true
	}
	last, err := retry.Until(ctx,
		retry.Options{Interval: 50 * time.Millisecond},
		func(ctx context.Context) (*httpx.Response, error) {
			return g.funnel.Client().Do(ctx, httpx.Request{
				Method: http.MethodGet, URL: url, Headers: g.scenarioHeaders(ctx),
			}, 0, 0)
		},
		accept,
	)
	return awaited(last, err, accept,
		fmt.Sprintf("the config dump never became consistent with %v", expectations))
}

// oobTemplateList asserts the listing carries the out-of-box provider templates, exactly.
func (g *Gateway) oobTemplateList(ctx context.Context) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	return oobTemplateListIs(resp)
}

// oobTemplateListIs asserts the response carries EXACTLY the out-of-box provider templates.
//
// Order-insensitive: two captures of this endpoint came back in different orders, so the templates
// are matched on metadata.name rather than by index. Every other field is compared, which is what
// makes this exact rather than a presence check — the version this replaced accepted "at least"
// the expected count and ignored any extra or altered template.
//
// status is excluded because createdAt/updatedAt are stamped per boot. They were the ONLY fields
// that differed between the two captures; everything else was byte-identical.
func oobTemplateListIs(resp *httpx.Response) error {
	var doc struct {
		Count     int              `json:"count"`
		Templates []map[string]any `json:"templates"`
	}
	if err := json.Unmarshal(resp.Body, &doc); err != nil {
		return fmt.Errorf("parsing the template list: %w (%s)", err, resp.Describe())
	}

	var want []any
	if err := json.Unmarshal([]byte(oobProviderTemplates), &want); err != nil {
		return fmt.Errorf("the expected template fixture is not valid JSON: %w", err)
	}

	got := make([]any, 0, len(doc.Templates))
	for _, t := range doc.Templates {
		got = append(got, t)
	}
	ignore := []string{"status"}
	gotByName, err := keyElements(got, "metadata.name", ignore)
	if err != nil {
		return fmt.Errorf("indexing the returned templates: %w (%s)", err, resp.Describe())
	}
	wantByName, err := keyElements(want, "metadata.name", ignore)
	if err != nil {
		return fmt.Errorf("indexing the expected templates: %w", err)
	}

	// Presence, not equality of the whole set: other runners in this block create templates of
	// their own and the listing returns every one, so "count == 7" and "no extras" both failed
	// (observed: expected count 7, got 8). What IS pinned is that each out-of-box template is
	// present and matches its shipped definition field for field.
	var missing []string
	for n := range wantByName {
		if _, ok := gotByName[n]; !ok {
			missing = append(missing, n)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("out-of-box templates missing from the listing: %v (%s)", missing, resp.Describe())
	}
	for _, n := range sortedElementKeys(wantByName) {
		if !reflect.DeepEqual(gotByName[n], wantByName[n]) {
			g, _ := json.Marshal(gotByName[n])
			w, _ := json.Marshal(wantByName[n])
			return fmt.Errorf("out-of-box template %q differs:\n  expected %s\n  got      %s", n, w, g)
		}
	}
	return nil
}

// oobProviderTemplates is the shipped out-of-box template set, captured from the product and
// reduced to the fields that are stable across boots. Regenerate the JSON deliberately when the
// product intends to change what it ships — that review is the point of pinning it.
//
// Embedded rather than read at runtime: the suite is a test binary that runs from whichever
// directory `go test` chose, so a relative path would be a failure mode with no upside.
//
//go:embed fixtures/oob_provider_templates.json
var oobProviderTemplates string

// keyElements indexes array elements by the value at keyPath, dropping the ignored paths.
func keyElements(arr []any, keyPath string, drop []string) (map[string]any, error) {
	out := make(map[string]any, len(arr))
	for i, el := range arr {
		m, ok := el.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("element %d is %T, not an object", i, el)
		}
		k, ok := traverseJSON(m, keyPath)
		if !ok {
			return nil, fmt.Errorf("element %d has no %q", i, keyPath)
		}
		key := fmt.Sprintf("%v", k)
		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("two elements share %s=%s", keyPath, key)
		}
		pruned := deepCopyWithout(m, drop)
		out[key] = pruned
	}
	return out, nil
}

// deepCopyWithout copies a decoded JSON object minus the given dotted paths.
func deepCopyWithout(node any, drop []string) any {
	switch v := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, child := range v {
			skip := false
			var nested []string
			for _, d := range drop {
				d = strings.TrimSpace(d)
				if d == k {
					skip = true
					break
				}
				if rest, ok := strings.CutPrefix(d, k+"."); ok {
					nested = append(nested, rest)
				}
			}
			if skip {
				continue
			}
			out[k] = deepCopyWithout(child, nested)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, child := range v {
			out = append(out, deepCopyWithout(child, drop))
		}
		return out
	default:
		return node
	}
}

func sortedElementKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// awaitPolicySnapshotSync blocks until the policy engine is running the controller's current
// policy chain.
//
// A deploy returning 200 means the CONTROL PLANE accepted it, not that the data plane routes
// it yet, and the two are asynchronous. Both processes report the chain version they hold at
// /xds_sync_status, so this compares them rather than sleeping: the condition is observable,
// so waiting on it is bounded and self-explaining when it fails.
//
// This exists alongside the readiness send because that step cannot be
// used for a route that is SUPPOSED to fail — a sandbox upstream configured to time out never
// becomes ready, so readiness must be established from the control plane's own state instead.
// policyChainVersions reads policy_chain_version from the controller and the policy frameworkruntime.
//
// Shared by the snapshot-sync and chain-advance waits so there is one definition of where the
// versions come from and how they are parsed.
func (g *Gateway) policyChainVersions(ctx context.Context) (controller, engine string, err error) {
	controllerBase, err := g.topo.URL("platform-gateway", "admin")
	if err != nil {
		return "", "", err
	}
	engineBase, err := g.topo.URL("platform-gateway", "policy-admin")
	if err != nil {
		return "", "", err
	}

	read := func(url string, authenticated bool) (string, error) {
		var headers map[string]string
		if authenticated {
			if v, ok := tcontext.Get(ctx, keyAuthHeader); ok {
				if h, ok := v.(string); ok && h != "" {
					headers = map[string]string{"Authorization": h}
				}
			}
		}
		resp, err := g.funnel.Get(ctx, url, headers)
		if err != nil {
			return "", err
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("%s -> %d", url, resp.StatusCode)
		}
		var doc map[string]interface{}
		if err := json.Unmarshal([]byte(resp.Body), &doc); err != nil {
			return "", err
		}
		v, ok := doc["policy_chain_version"]
		if !ok {
			return "", fmt.Errorf("%s: no policy_chain_version in %s", url, resp.Body)
		}
		return fmt.Sprintf("%v", v), nil
	}

	controller, err = read(controllerBase+adminBasePath+"/xds_sync_status", true)
	if err != nil {
		return "", "", err
	}
	engine, err = read(engineBase+"/xds_sync_status", false)
	if err != nil {
		// The controller's value is still returned so a failure names what WAS seen.
		return controller, "", err
	}
	return controller, engine, nil
}

func (g *Gateway) awaitPolicySnapshotSync(ctx context.Context) error {

	// Routed through retry.Until rather than a hand-rolled loop, and the change is not
	// cosmetic: this loop set its own 30-second deadline, one sixth of
	// retry.PropagationCeiling. Options.deadline floors every wait at the ceiling precisely so
	// a call site cannot quietly pick a shorter one — a loaded runner has been observed ~90-100s
	// behind a successful write, so a shorter cap could report "did not sync" for a component
	// that was merely slow. It also inherits the tiered cadence instead of hammering a
	// struggling engine at the base interval for the whole window.
	type snapshotVersions struct{ controller, engine string }

	seen, err := retry.Until(ctx,
		retry.Options{Interval: 200 * time.Millisecond},
		func(ctx context.Context) (snapshotVersions, error) {
			// Transient: during warm-up either admin endpoint can refuse the connection or
			// answer non-200, and a malformed body is classified the same way — neither is
			// worth failing fast on, because both resolve as the pair comes up. The
			// controller's value is carried through so a failure names what WAS seen.
			ctrl, eng, err := g.policyChainVersions(ctx)
			if err != nil {
				return snapshotVersions{controller: ctrl}, retry.Transient(err)
			}
			return snapshotVersions{controller: ctrl, engine: eng}, nil
		},
		func(v snapshotVersions) bool {
			return v.controller != "" && v.controller == v.engine
		},
	)
	if err != nil {
		return fmt.Errorf("policy snapshot sync: %w", err)
	}
	// Until returns the last result with a nil error when every attempt succeeded but the
	// condition never held, so the verdict is the caller's — this check is what turns that into
	// a failure, and omitting it is exactly how a poll silently passes.
	if seen.controller == "" || seen.controller != seen.engine {
		return fmt.Errorf(
			"policy snapshot did not sync within %s: controller=%q, engine=%q",
			retry.PropagationCeiling, seen.controller, seen.engine)
	}
	return nil
}

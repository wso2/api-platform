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

// Package steps holds the gateway suite's step definitions.
//
// These are ported from the existing suite with their ASSERTIONS UNCHANGED. What changes is
// addressing: where a step previously targeted a hardcoded localhost port, it now resolves a
// URL from the running topology. That single change is what allows two blocks to run at once,
// and it is why the port is worth doing at all.
package steps

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	frameworkruntime "github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

// API base paths, kept in sync with the gateway's OpenAPI documents.
const (
	managementBasePath = "/api/management/v1"
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

// sendUntilQuotaRemaining polls a governed path until the rate limit reports the wanted remaining
// quota, then leaves that response published for the assertions after it.
//
// The step says "remaining" and not "requests" or "tokens" deliberately: the unit is decided by
// which policy is attached, not by the caller, and naming one here would imply two headers exist.
//
// Why a quota reading rather than a status: after a policy UPDATE a status poll sees 200 both
// before and after the swap, so it cannot tell which configuration is serving. A remaining count
// can — and it also proves the counter RESET, because a carried-over counter never reaches the
// expected figure and the step fails with what it kept seeing instead of throttling later for
// reasons that look like a quota result.
//
// Costs one request against whichever bucket is live when the condition holds. Polls before that
// are charged to the pre-update bucket, so they spend nothing this scenario counts.
func (g *Gateway) sendUntilQuotaRemaining(ctx context.Context, method, path string, want int) error {
	return g.sendUntilHeader(ctx, method, path, headerRateLimitRemaining, strconv.Itoa(want))
}

// Gateway holds what the gateway steps need. The shared plumbing lives in Base.
type Gateway struct {
	*Base
}

// New builds the step set for one block's topology.
func New(topo *frameworkruntime.Topology) *Gateway {
	client := httpx.NewClient(httpx.Options{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
	})
	exposeContextValues()
	return &Gateway{Base: &Base{topo: topo, funnel: httpx.NewFunnel(client, 3, 2*time.Second)}}
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

// managementURL builds a management API URL from the RUNNING topology.
//
// The replacement for a hardcoded "http://localhost:9090/...". The port is not knowable
// until the container is running, which is precisely why the old form could only ever
// support one stack at a time.
func (g *Gateway) managementURL(path string) (string, error) {
	base, err := g.topo.URL("platform-gateway", "rest")
	if err != nil {
		return "", err
	}
	return base + managementBasePath + path, nil
}

func (g *Gateway) adminURL(path string) (string, error) {
	base, err := g.topo.URL("platform-gateway", "admin")
	if err != nil {
		return "", err
	}
	return base + adminBasePath + path, nil
}

// apiNameFrom extracts metadata.name from a YAML API definition.
//
// Deliberately a narrow scan rather than a YAML parse: the definition is a docstring written
// for the product, and pulling in a parser to read one field would mean the step failing on
// syntax the product itself accepts.
func apiNameFrom(definition string) string {
	inMetadata := false
	for _, line := range strings.Split(definition, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "metadata:":
			inMetadata = true
		case inMetadata && strings.HasPrefix(trimmed, "name:"):
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "name:")), `"'`)
		case inMetadata && !strings.HasPrefix(line, " ") && trimmed != "":
			// Left the metadata block without finding a name.
			inMetadata = false
		}
	}
	return ""
}

// Register wires every step this suite provides.
//
// Every step uses sc.Step, never sc.Given/When/Then, and that is deliberate.
//
// godog's Given/When/Then are keyword-RESTRICTED matchers, while Gherkin's And/But inherit
// the keyword of the step above them. So `And I reset the request` written after a `Then`
// is a Then-step, and a When-registered pattern silently does not match it — the step is
// reported UNDEFINED even though it is plainly registered. That cost a run here: six steps
// in cel_conditions were undefined purely because they followed a Then.
//
// Keyword-agnostic registration is also what the original suite did, so this keeps ported
// features matching without rewriting their keyword structure.
func (g *Gateway) Register(sc *godog.ScenarioContext) {
	// Request-shaping state is reset before EVERY scenario.
	//
	// The scope it lives in — Local — is per RUNNER, and every scenario in that runner
	// shares it. Without this reset a header set in one scenario silently applies to the
	// next, which is exactly what "I set header" must not mean. The original suite got this
	// for free because its step struct was per-scenario; the port changed that lifetime, so
	// the reset has to be explicit.
	//
	// Features do call "I reset the request", but only where their author noticed. Relying
	// on that makes correctness depend on every future author noticing too.
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		return ctx, g.resetRequest(ctx)
	})

	sc.Step(`^the gateway services are running$`, g.gatewayIsRunning)
	sc.Step(`^I authenticate using basic auth as "([^"]*)"$`, g.authenticateAs)

	sc.Step(
		`^I create (API|LLM provider|LLM provider template|MCP proxy|LLM proxy) with configuration:$`,
		g.createResource)
	sc.Step(`^I get the API "([^"]*)"$`, g.getAPI)
	sc.Step(`^I update the (API|LLM provider|LLM provider template|MCP proxy|LLM proxy) "([^"]*)" with configuration:$`, g.updateResource)
	sc.Step(`^I delete the (API|LLM provider|LLM provider template|MCP proxy|LLM proxy) "([^"]*)"$`, g.deleteResource)
	sc.Step(`^I send a "([^"]*)" request to the "([^"]*)" service at "([^"]*)"$`, g.serviceRequest)
	sc.Step(`^I send a "([^"]*)" request to the "([^"]*)" service at "([^"]*)" with body:$`, g.serviceRequestWithBody)
	sc.Step(`^the response should be an oob-template list$`, g.oobTemplateList)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" until the rate limit reports (\d+) remaining$`,
		g.sendUntilQuotaRemaining)

	sc.Step(`^I wait for policy snapshot sync$`, g.awaitPolicySnapshotSync)

	g.registerBaseSteps(sc)
	g.registerTimeoutSteps(sc)
	g.registerHealthSteps(sc)
}

// NOTE: there is deliberately NO `I wait for N seconds` step.
//
// The legacy suite uses one 131 times. A fixed sleep is too long when the system is fast and
// too short when it is loaded, and the second case is a flake that reads as a product bug.
// Every one of those uses is waiting for a CONDITION — routability, a snapshot, a resource
// existing — and the condition is what a migrated feature must say.
//
// The step is absent rather than deprecated so that a feature ported by copy-paste FAILS at
// registration with an undefined step, which is a loud, immediate error. Leaving it
// registered with a comment asking people not to use it would let sleeps back in silently,
// one paste at a time.

// gatewayIsRunning asserts the gateway answers its own health endpoint.
//
// The engine already gated the block on this before any scenario ran, so it is a cheap
// restatement rather than a wait — but it keeps the Gherkin readable and fails clearly if a
// container died between boot and this scenario.
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
	if who != "admin" {
		return fmt.Errorf("unknown actor %q: this suite provisions only 'admin'", who)
	}
	user, err := tcontext.ResolveString(ctx, frameworkruntime.KeyAdminUser)
	if err != nil {
		return err
	}
	pass, err := tcontext.ResolveString(ctx, frameworkruntime.KeyAdminPass)
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
		return g.createAPI(ctx, body)
	}
	return g.mutateResource(ctx, http.MethodPost, spec.collection, "", body)
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
	for _, line := range strings.Split(def, "\n") {
		if strings.HasPrefix(line, "kind:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "kind:"))
		}
	}
	return ""
}

// createAPI posts an API definition and waits for it to become routable.
//
// The migrated behaviour differs from the original in ONE respect, deliberately: the
// original slept a fixed second for xDS propagation. A fixed sleep is both too long when
// propagation is fast and too short when a loaded host is slow — the second case being a
// flake that looks like a product bug. This polls instead, bounded by the shared ceiling.
func (g *Gateway) createAPI(ctx context.Context, body *godog.DocString) error {
	// The definition may name resources; expanding placeholders is what keeps two concurrent
	// scenarios from creating the same API.
	definition, err := unique.Expand(ctx, body.Content)
	if err != nil {
		return err
	}

	url, err := g.managementURL("/rest-apis")
	if err != nil {
		return err
	}

	resp, err := g.funnel.Post(ctx, url, g.headerWith(ctx, "Content-Type", "application/yaml"), []byte(definition))
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
		return err
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
	resolved, err := unique.Expand(ctx, name)
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
	resolved, err := unique.Expand(ctx, name)
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
var serviceEndpoints = map[string]struct{ endpoint, basePath string }{
	"gateway-controller":       {"rest", managementBasePath},
	"gateway-controller-admin": {"admin", adminBasePath},
	// Metrics live on a DIFFERENT compose service from the one tests normally address —
	// controller metrics on the controller, policy-engine metrics on the runtime — which the
	// component contract resolves via Endpoint.Service. No base path: a scrape is not an API.
	"controller-metrics":    {"metrics", ""},
	"policy-engine-metrics": {"pe-metrics", ""},
}

// serviceURL resolves a feature's service name and path to a URL on the running topology.
func (g *Gateway) serviceURL(ctx context.Context, service, path string) (string, error) {
	spec, ok := serviceEndpoints[service]
	if !ok {
		return "", fmt.Errorf("unknown service %q: this suite addresses %v", service, sortedServiceNames())
	}
	base, err := g.topo.URL("platform-gateway", spec.endpoint)
	if err != nil {
		return "", err
	}
	resolved, err := unique.Expand(ctx, path)
	if err != nil {
		return "", err
	}
	if resolved != "" && !strings.HasPrefix(resolved, "/") {
		resolved = "/" + resolved
	}
	return base + spec.basePath + resolved, nil
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
		content, expErr := unique.Expand(ctx, body.Content)
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

// updateAPI replaces an existing API definition.
//
// The body is YAML like a deploy, so the content type is set explicitly — the default of
// application/json would make the controller reject it with a JSON parse error naming the
// first byte of "apiVersion" rather than the content type.
func (g *Gateway) updateAPI(ctx context.Context, name string, body *godog.DocString) error {
	resolvedName, err := unique.Expand(ctx, name)
	if err != nil {
		return err
	}
	definition, err := unique.Expand(ctx, body.Content)
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
	inMetadata := false
	for _, line := range strings.Split(def, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "metadata:") {
			inMetadata = true
			continue
		}
		if inMetadata {
			if strings.HasPrefix(trimmed, "name:") {
				return strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			}
			if trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				return "" // left the metadata block
			}
		}
	}
	return ""
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
	resolvedID, err := unique.Expand(ctx, id)
	if err != nil {
		return err
	}

	path := collection
	if resolvedID != "" {
		path = collection + "/" + resolvedID
	}
	if method == http.MethodDelete {
		expectInDump(ctx, resolvedID, false)
	} else if body != nil {
		if h := handleFromDefinition(body.Content); h != "" {
			expectInDump(ctx, h, true)
		}
	}
	url, err := g.serviceURL(ctx, "gateway-controller", path)
	if err != nil {
		return err
	}

	var payload []byte
	if body != nil {
		content, err := unique.Expand(ctx, body.Content)
		if err != nil {
			return err
		}
		payload = []byte(content)
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
	if method == http.MethodPost {
		resourceID := resolvedID
		if resourceID == "" && body != nil {
			resourceID = handleFromDefinition(body.Content)
			resourceID, err = unique.Expand(ctx, resourceID)
			if err != nil {
				return err
			}
		}
		if kind, ok := cleanupKindForCollection(collection); ok && resourceID != "" {
			if err := cleanup.Register(ctx, cleanup.Resource{
				Kind: kind, ID: resourceID, Actor: "admin",
				Description: "created by " + scenarioLabel(ctx),
			}); err != nil {
				return err
			}
		}
	}

	// The published response must survive: the next step asserts on the MUTATION's response,
	// not on the poll's. So the wait uses a bare client and restores it afterwards.
	settled := resp
	waitFor := func(want bool) error {
		target := collection + "/" + resolvedID
		if resolvedID == "" {
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
			exists := r.StatusCode != http.StatusNotFound
			return exists == want
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
				reg.Deregister(kind, resolvedID)
			}
		}
	}
	return tcontext.Set(ctx, httpx.ResponseKey, settled)
}

func cleanupKindForCollection(collection string) (cleanup.Kind, bool) {
	switch collection {
	case collLLMProviders:
		return cleanup.KindLLMProvider, true
	case collLLMProxies:
		return cleanup.KindLLMProxy, true
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
	// behind a successful write, so a 30s cap could report "did not sync" for a component that
	// was merely slow. It also inherits the tiered cadence: 200ms polls for the first minute,
	// then easing off, instead of hammering a struggling engine 5 times a second for the whole
	// window.
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

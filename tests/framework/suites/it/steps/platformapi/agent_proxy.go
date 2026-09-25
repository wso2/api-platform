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
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/tests/framework/core/actor"
	controlplane "github.com/wso2/api-platform/tests/framework/core/catalog/platformapi"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// internalAPIBase is the control plane's gateway-facing API, authenticated with a gateway key.
const internalAPIBase = "/api/internal/v1"

// gatewayKeyHeader carries a gateway's control-plane key on the internal API.
const gatewayKeyHeader = "api-key"

// Scenario-context keys owned by the Agent proxy steps.
const (
	keyGatewayToken   = "platformAPIGatewayToken"
	agentProxyRecordD = "\x00"
)

// agentCardFixtureEndpoint is the testbench endpoint serving the Agent Card fixture.
const agentCardFixtureEndpoint = "agentcard"

var (
	platformAgentAPIKeyKind     = cleanup.Kind{Name: "platform-api-agent-proxy-api-key", Order: 31}
	platformGatewayTokenKind    = cleanup.Kind{Name: "platform-api-gateway-token", Order: 35}
	platformAgentDeploymentKind = cleanup.Kind{Name: "platform-api-agent-proxy-deployment", Order: 41}
	platformAgentProxyKind      = cleanup.Kind{Name: "platform-api-agent-proxy", Order: 53}
)

// agentProxyLocationPattern parses the Location values the control plane sets for Agent proxy
// resources: the proxy itself, one of its deployments or one of its API keys.
var agentProxyLocationPattern = regexp.MustCompile(
	`/agent-proxies/([^/?#]+)(?:/(deployments|api-keys)/([^/?#]+))?$`)

// RegisterAgentProxy binds the steps that drive the Agent proxy control-plane surface: a
// response-publishing request funnel for the publisher API, canonical-template authoring,
// deployment, the gateway-internal API, the Agent Card fixture and deployment archives.
//
// Every control-plane request goes through one funnel that publishes its response for the
// shared assertions and records what it created: a 201 naming an Agent proxy, a deployment or
// an API key is registered for cleanup immediately, including one a negative scenario did not
// expect to succeed, and a 204 delete deregisters what it removed.
func RegisterAgentProxy(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^I send a "([^"]*)" request to the control plane at "([^"]*)"(?: as "([^"]*)")?(?: with header "([^"]*)" set to "([^"]*)")?$`,
		func(ctx context.Context, method, path, who, header, value string) error {
			return s.sendControlPlaneStep(ctx, method, path, who, header, value, nil)
		})
	sc.Step(`^I send a "([^"]*)" request to the control plane at "([^"]*)"(?: as "([^"]*)")?(?: with header "([^"]*)" set to "([^"]*)")? with body:$`,
		s.sendControlPlaneStep)
	sc.Step(`^I send a "GET" request to the control plane at "([^"]*)" until the JSON field "([^"]*)" is "([^"]*)"$`,
		s.pollControlPlaneJSONField)
	sc.Step(`^I send a "([^"]*)" request to the control plane at "([^"]*)" until the response header "([^"]*)" matches "([^"]*)" with body:$`,
		s.pollControlPlaneHeader)
	sc.Step(`^I create an Agent proxy via the control plane from "([^"]*)"(?: as "([^"]*)")? with values:$`,
		s.createAgentProxyFromTemplate)
	sc.Step(`^I update the Agent proxy "([^"]*)" via the control plane from "([^"]*)"(?: as "([^"]*)")? with values:$`,
		s.updateAgentProxyFromTemplate)
	sc.Step(`^I deploy the Agent proxy "([^"]*)" to the gateway via the control plane and store the deployment id as "([^"]*)"$`,
		s.deployAgentProxy)
	sc.Step(`^I store the registered gateway id as "([^"]*)"$`, s.storeGatewayID)
	sc.Step(`^I create a secret "([^"]*)" with value "([^"]*)" via the control plane$`, s.createSecretWithValue)
	sc.Step(`^I store the Agent Card fixture URL for scope "([^"]*)" in mode "([^"]*)" as "([^"]*)"$`,
		s.storeAgentCardFixtureURL)
	sc.Step(`^the Agent Card fixture should have received (\d+) requests? for scope "([^"]*)"$`,
		s.agentCardFixtureCountIs)
	sc.Step(`^I fetch the Agent Card of Agent proxy "([^"]*)" via the control plane until the Agent Card fixture has received (\d+) requests? for scope "([^"]*)"$`,
		s.fetchAgentCardUntilFixtureCount)
	sc.Step(`^I obtain an API key for the registered gateway via the control plane$`, s.obtainGatewayToken)
	sc.Step(`^I send a "([^"]*)" request to the gateway internal API at "([^"]*)"( without a gateway API key)?$`,
		s.sendGatewayInternal)
	sc.Step(`^I resolve the gateway internal artifact id of Agent proxy deployment "([^"]*)" and store it as "([^"]*)"$`,
		s.resolveInternalArtifactID)
	sc.Step(`^I fetch the gateway deployment batch for deployment "([^"]*)" via the gateway internal API$`,
		s.fetchDeploymentBatch)
	sc.Step(`^the response should be a (ZIP|gzip tar) archive containing only "([^"]*)"$`, s.archiveContainsOnly)
	sc.Step(`^the archived deployment YAML field "([^"]*)" should be "([^"]*)"$`, s.archivedFieldIs)
	sc.Step(`^the archived deployment YAML field "([^"]*)" should not exist$`, s.archivedFieldAbsent)
}

// actorCredentials maps a feature's actor name to the suite's control-plane users. The empty
// name is the administrator, so a step that names no actor authenticates as it always has.
func actorCredentials(who string) (actor.Credentials, error) {
	switch strings.TrimSpace(who) {
	case "", "admin":
		return actor.Administrator(), nil
	case "publisher":
		return actor.Publisher(), nil
	case "developer":
		return actor.Developer(), nil
	default:
		return actor.Credentials{}, fmt.Errorf("unknown control-plane actor %q: supported actors are admin, publisher, and developer", who)
	}
}

// controlPlaneCall is one publisher-API request made through the funnel.
type controlPlaneCall struct {
	method string
	path   string
	who    string
	header string
	value  string
	body   []byte
}

func (s *Steps) sendControlPlaneStep(
	ctx context.Context, method, path, who, header, value string, body *godog.DocString,
) error {
	call := controlPlaneCall{method: method, path: path, who: who, header: header, value: value}
	if body != nil {
		content, err := stepscommon.Expand(ctx, body.Content)
		if err != nil {
			return err
		}
		call.body = []byte(content)
	}
	_, err := s.sendControlPlane(ctx, call)
	return err
}

// sendControlPlane issues one publisher-API request, publishes the response and records any
// resource the response reports creating or deleting.
func (s *Steps) sendControlPlane(ctx context.Context, call controlPlaneCall) (*httpx.Response, error) {
	method := strings.ToUpper(strings.TrimSpace(call.method))
	if method == "" {
		return nil, fmt.Errorf("a control-plane request needs a method")
	}
	path, err := stepscommon.Expand(ctx, call.path)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("control-plane path %q must start with /", path)
	}
	credentials, err := actorCredentials(call.who)
	if err != nil {
		return nil, err
	}
	base, err := s.baseURL()
	if err != nil {
		return nil, err
	}
	bearer, err := controlplane.ControlPlaneLogin(ctx, base, credentials.Username, credentials.Password)
	if err != nil {
		return nil, fmt.Errorf("authenticating to the control plane as %q: %w", credentials.Username, err)
	}
	headers := map[string]string{"Authorization": "Bearer " + bearer}
	if call.body != nil {
		headers["Content-Type"] = "application/json"
	}
	if name := strings.TrimSpace(call.header); name != "" {
		value, expandErr := stepscommon.Expand(ctx, call.value)
		if expandErr != nil {
			return nil, expandErr
		}
		headers[name] = value
	}
	resp, err := s.funnel.Send(ctx, httpx.Request{
		Method: method, URL: base + apiBase + path, Headers: headers, Body: call.body,
	})
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	if err := s.trackAgentProxyResource(ctx, method, call.body, resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// trackAgentProxyResource registers what a successful create produced and deregisters what a
// successful delete removed, so cleanup follows the product's own answer rather than the
// scenario's expectation of it.
func (s *Steps) trackAgentProxyResource(ctx context.Context, method string, body []byte, resp *httpx.Response) error {
	switch {
	case method == http.MethodPost && resp.StatusCode == http.StatusCreated:
		match := agentProxyLocationPattern.FindStringSubmatch(locationPath(resp.Headers.Get("Location")))
		if match == nil {
			return nil
		}
		handle, sub, id := match[1], match[2], match[3]
		switch sub {
		case "":
			return s.registerTracked(ctx, platformAgentProxyKind, handle, func(ctx context.Context) error {
				return s.deleteAgentProxy(ctx, handle)
			}, s.registerAgentProxyDeleter)
		case "deployments":
			gatewayID, err := requestGatewayID(body)
			if err != nil {
				return fmt.Errorf("recording the created deployment %q for cleanup: %w", id, err)
			}
			record := strings.Join([]string{handle, id, gatewayID}, agentProxyRecordD)
			return s.registerTracked(ctx, platformAgentDeploymentKind, record, func(ctx context.Context) error {
				return s.undeployAgentDeployment(ctx, record)
			}, s.registerAgentDeploymentDeleter)
		case "api-keys":
			record := handle + agentProxyRecordD + id
			return s.registerTracked(ctx, platformAgentAPIKeyKind, record, func(ctx context.Context) error {
				return s.revokeAgentAPIKey(ctx, record)
			}, s.registerAgentAPIKeyDeleter)
		}
	case method == http.MethodDelete && resp.StatusCode == http.StatusNoContent:
		return deregisterDeleted(ctx, resp.URL)
	}
	return nil
}

// registerTracked registers a created resource for cleanup, deleting it on the spot when the
// registration itself fails so an unregistered resource can never outlive the scenario.
func (s *Steps) registerTracked(
	ctx context.Context, kind cleanup.Kind, id string, compensate func(context.Context) error,
	installDeleter func(*cleanup.Registry) error,
) error {
	reg, err := cleanup.Of(ctx)
	if err == nil {
		err = installDeleter(reg)
	}
	if err == nil {
		err = reg.Register(cleanup.Resource{Kind: kind, ID: id, Actor: "admin", Description: "created via platform-api"})
	}
	if err == nil {
		return nil
	}
	if compensateErr := compensate(ctx); compensateErr != nil {
		return fmt.Errorf("registering %s %q for cleanup: %w; compensating delete failed: %v", kind.Name, id, err, compensateErr)
	}
	return fmt.Errorf("registering %s %q for cleanup: %w", kind.Name, id, err)
}

// deregisterDeleted drops the cleanup record of an Agent proxy, deployment or API key the
// scenario deleted itself.
func deregisterDeleted(ctx context.Context, rawURL string) error {
	match := agentProxyLocationPattern.FindStringSubmatch(locationPath(rawURL))
	if match == nil {
		return nil
	}
	reg, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	handle, sub, id := match[1], match[2], match[3]
	switch sub {
	case "":
		reg.Deregister(platformAgentProxyKind, handle)
	case "deployments":
		prefix := handle + agentProxyRecordD + id + agentProxyRecordD
		for _, pending := range reg.Pending() {
			if pending.Kind.Name == platformAgentDeploymentKind.Name && strings.HasPrefix(pending.ID, prefix) {
				reg.Deregister(platformAgentDeploymentKind, pending.ID)
			}
		}
	case "api-keys":
		reg.Deregister(platformAgentAPIKeyKind, handle+agentProxyRecordD+id)
	}
	return nil
}

// locationPath returns the unescaped path of a Location header or request URL.
func locationPath(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return parsed.Path
}

func requestGatewayID(body []byte) (string, error) {
	var request struct {
		GatewayID string `json:"gatewayId"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return "", fmt.Errorf("the deployment request body is not JSON: %w", err)
	}
	if strings.TrimSpace(request.GatewayID) == "" {
		return "", fmt.Errorf("the deployment request names no gatewayId")
	}
	return request.GatewayID, nil
}

func (s *Steps) registerAgentProxyDeleter(reg *cleanup.Registry) error {
	return reg.RegisterDeleter(platformAgentProxyKind, func(ctx context.Context, res cleanup.Resource) error {
		return s.deleteAgentProxy(ctx, res.ID)
	})
}

func (s *Steps) registerAgentDeploymentDeleter(reg *cleanup.Registry) error {
	return reg.RegisterDeleter(platformAgentDeploymentKind, func(ctx context.Context, res cleanup.Resource) error {
		return s.undeployAgentDeployment(ctx, res.ID)
	})
}

func (s *Steps) registerAgentAPIKeyDeleter(reg *cleanup.Registry) error {
	return reg.RegisterDeleter(platformAgentAPIKeyKind, func(ctx context.Context, res cleanup.Resource) error {
		return s.revokeAgentAPIKey(ctx, res.ID)
	})
}

// deleteAgentProxy removes an Agent proxy; one that is already gone is not a failure.
func (s *Steps) deleteAgentProxy(ctx context.Context, handle string) error {
	resp, err := s.adminCall(ctx, http.MethodDelete, "/agent-proxies/"+url.PathEscape(handle))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNotFound && !resp.Succeeded() {
		return fmt.Errorf("deleting Agent proxy %q: %s", handle, resp.Describe())
	}
	return nil
}

// undeployAgentDeployment undeploys a recorded deployment. A deployment that is already gone,
// or no longer active because the scenario undeployed it, needs nothing more.
func (s *Steps) undeployAgentDeployment(ctx context.Context, record string) error {
	parts := strings.Split(record, agentProxyRecordD)
	if len(parts) != 3 {
		return fmt.Errorf("invalid Agent proxy deployment cleanup record")
	}
	path := "/agent-proxies/" + url.PathEscape(parts[0]) + "/deployments/" + url.PathEscape(parts[1]) +
		"/undeploy?gatewayId=" + url.QueryEscape(parts[2])
	resp, err := s.adminCall(ctx, http.MethodPost, path)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusConflict || resp.Succeeded() {
		return nil
	}
	return fmt.Errorf("undeploying Agent proxy %q deployment %q: %s", parts[0], parts[1], resp.Describe())
}

// revokeAgentAPIKey revokes a recorded API key. Revocation needs a gateway association, and a
// key on an Agent proxy that was never deployed has none; such a key is removed with its Agent
// proxy, whose own cleanup is always registered, so the documented 503 is not a leak.
func (s *Steps) revokeAgentAPIKey(ctx context.Context, record string) error {
	parts := strings.Split(record, agentProxyRecordD)
	if len(parts) != 2 {
		return fmt.Errorf("invalid Agent proxy API key cleanup record")
	}
	resp, err := s.adminCall(ctx, http.MethodDelete,
		"/agent-proxies/"+url.PathEscape(parts[0])+"/api-keys/"+url.PathEscape(parts[1]))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound || resp.Succeeded() ||
		(resp.StatusCode == http.StatusServiceUnavailable && errorCode(resp) == "GATEWAY_CONNECTION_UNAVAILABLE") {
		return nil
	}
	return fmt.Errorf("revoking Agent proxy %q API key %q: %s", parts[0], parts[1], resp.Describe())
}

// adminCall issues one unpublished administrator request, for cleanup and intermediate lookups
// whose responses no assertion reads.
func (s *Steps) adminCall(ctx context.Context, method, path string) (*httpx.Response, error) {
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return nil, err
	}
	return s.client.Do(ctx, httpx.Request{
		Method: method, URL: base + apiBase + path,
		Headers: map[string]string{"Authorization": "Bearer " + bearer},
	}, 0, 0)
}

func errorCode(resp *httpx.Response) string {
	var body struct {
		Code string `json:"code"`
	}
	if resp == nil || json.Unmarshal(resp.Body, &body) != nil {
		return ""
	}
	return body.Code
}

// pollControlPlaneJSONField repeats a read until a JSON field holds the wanted value, publishing
// the matching response. Deployment status is acknowledged asynchronously by the gateway, so a
// read taken straight after the write legitimately reports the transitional state.
func (s *Steps) pollControlPlaneJSONField(ctx context.Context, path, field, want string) error {
	expected, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	last := "no response"
	err = retry.Await(ctx, retry.Options{Interval: time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, sendErr := s.sendControlPlane(ctx, controlPlaneCall{method: http.MethodGet, path: path})
			if sendErr != nil {
				return nil, retry.Transient(sendErr)
			}
			last = resp.Describe()
			return resp, nil
		},
		func(resp *httpx.Response) bool { return jsonFieldEquals(resp, field, expected) },
		fmt.Sprintf("waiting for GET %s to report %s %q", path, field, expected))
	if err != nil {
		return fmt.Errorf("%w (last %s)", err, last)
	}
	return nil
}

// pollControlPlaneHeader repeats a side-effect-free request until a response header matches a
// pattern, publishing the matching response. It exists for freshness headers such as Age, whose
// value is a whole number of seconds and so is only positive once a second has passed.
func (s *Steps) pollControlPlaneHeader(
	ctx context.Context, method, path, header, pattern string, body *godog.DocString,
) error {
	resolved, err := stepscommon.Expand(ctx, pattern)
	if err != nil {
		return err
	}
	re, err := regexp.Compile(resolved)
	if err != nil {
		return fmt.Errorf("header pattern %q is invalid: %w", resolved, err)
	}
	content, err := stepscommon.Expand(ctx, body.Content)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Interval: 500 * time.Millisecond},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, sendErr := s.sendControlPlane(ctx, controlPlaneCall{method: method, path: path, body: []byte(content)})
			if sendErr != nil {
				return nil, retry.Transient(sendErr)
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool { return resp != nil && re.MatchString(resp.Headers.Get(header)) },
		fmt.Sprintf("waiting for %s %s to answer with header %s matching %q", method, path, header, resolved))
}

func jsonFieldEquals(resp *httpx.Response, field, want string) bool {
	if resp == nil || resp.StatusCode != http.StatusOK {
		return false
	}
	var doc any
	if json.Unmarshal(resp.Body, &doc) != nil {
		return false
	}
	got, ok := traverse(doc, field)
	return ok && fmt.Sprintf("%v", got) == want
}

func (s *Steps) createAgentProxyFromTemplate(ctx context.Context, templateName, who string, table *godog.Table) error {
	payload, err := s.renderJSONTemplate(ctx, templateName, table)
	if err != nil {
		return err
	}
	_, err = s.sendControlPlane(ctx, controlPlaneCall{method: http.MethodPost, path: "/agent-proxies", who: who, body: payload})
	return err
}

func (s *Steps) updateAgentProxyFromTemplate(ctx context.Context, handle, templateName, who string, table *godog.Table) error {
	resolved, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	payload, err := s.renderJSONTemplate(ctx, templateName, table)
	if err != nil {
		return err
	}
	_, err = s.sendControlPlane(ctx, controlPlaneCall{
		method: http.MethodPut, path: "/agent-proxies/" + url.PathEscape(resolved), who: who, body: payload,
	})
	return err
}

// renderJSONTemplate renders a canonical YAML resource template and encodes it as the JSON
// document the publisher API accepts.
func (s *Steps) renderJSONTemplate(ctx context.Context, templateName string, table *godog.Table) ([]byte, error) {
	path, err := stepscommon.ResourceTemplatePath(s.featureRoot, templateName)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read resource template %q: %w", templateName, err)
	}
	rendered, err := stepscommon.RenderResourceTemplate(ctx, templateName, content, table)
	if err != nil {
		return nil, fmt.Errorf("resource template %q: %w", templateName, err)
	}
	return yamlToJSON([]byte(rendered))
}

func yamlToJSON(document []byte) ([]byte, error) {
	var value any
	if err := yaml.Unmarshal(document, &value); err != nil {
		return nil, fmt.Errorf("parse rendered template: %w", err)
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, fmt.Errorf("rendered template is not a mapping")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode rendered template as JSON: %w", err)
	}
	return encoded, nil
}

// deployAgentProxy creates a deployment of the current Agent proxy configuration on the block's
// gateway. The 201 is published, so a scenario can assert its status, body and Location, and the
// deployment id is stored for the lifecycle actions that follow.
func (s *Steps) deployAgentProxy(ctx context.Context, handle, storeAs string) error {
	resolved, err := stepscommon.Expand(ctx, handle)
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
	name, err := unique.Unique(ctx, "agent-dep")
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"name": name, "base": "current", "gatewayId": gatewayID})
	if err != nil {
		return err
	}
	resp, err := s.sendControlPlane(ctx, controlPlaneCall{
		method: http.MethodPost, path: "/agent-proxies/" + url.PathEscape(resolved) + "/deployments", body: body,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("deploying Agent proxy %q: %s", resolved, resp.Describe())
	}
	var created struct {
		DeploymentID string `json:"deploymentId"`
	}
	if err := json.Unmarshal(resp.Body, &created); err != nil || created.DeploymentID == "" {
		return fmt.Errorf("deploying Agent proxy %q: the response names no deployment: %s", resolved, resp.Describe())
	}
	return tcontext.Set(ctx, storeAs, created.DeploymentID)
}

func (s *Steps) storeGatewayID(ctx context.Context, key string) error {
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}
	gatewayID, err := s.gatewayUUID(ctx, base, bearer)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, key, gatewayID)
}

// agentCardFixtureInternalURL is the fixture base the control plane dials on the block network.
func (s *Steps) agentCardFixtureInternalURL() (string, error) {
	inst, err := s.topo.Component("testbench")
	if err != nil {
		return "", err
	}
	base, err := inst.InternalURL(agentCardFixtureEndpoint)
	if err != nil {
		return "", err
	}
	if s.topo.Block == nil {
		return "", fmt.Errorf("the Agent Card fixture requires a block partition, but the topology has no block")
	}
	return base + "/" + s.topo.Block.PartitionKey(), nil
}

func (s *Steps) storeAgentCardFixtureURL(ctx context.Context, scope, mode, key string) error {
	resolvedScope, err := stepscommon.Expand(ctx, scope)
	if err != nil {
		return err
	}
	resolvedMode, err := stepscommon.Expand(ctx, mode)
	if err != nil {
		return err
	}
	base, err := s.agentCardFixtureInternalURL()
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, key, agentCardFixtureURL(base, resolvedScope, resolvedMode))
}

func agentCardFixtureURL(base, scope, mode string) string {
	return strings.TrimSuffix(base, "/") + "/" + scope + "/" + mode
}

// agentCardFixtureCount reads how many card requests the fixture received for a scope.
func (s *Steps) agentCardFixtureCount(ctx context.Context, scope string) (int, error) {
	base, err := s.topo.URL("testbench", agentCardFixtureEndpoint)
	if err != nil {
		return 0, err
	}
	if s.topo.Block == nil {
		return 0, fmt.Errorf("the Agent Card fixture requires a block partition, but the topology has no block")
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodGet,
		URL:    base + "/" + s.topo.Block.PartitionKey() + "/test/requests?scope=" + url.QueryEscape(scope),
	}, 0, 0)
	if err != nil {
		return 0, fmt.Errorf("reading the Agent Card fixture counter: %w", err)
	}
	return parseFixtureCount(resp, scope)
}

func parseFixtureCount(resp *httpx.Response, scope string) (int, error) {
	if resp == nil || resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("reading the Agent Card fixture counter for scope %q: %s", scope, resp.Describe())
	}
	var body struct {
		Scope string `json:"scope"`
		Count *int   `json:"count"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil || body.Count == nil || body.Scope != scope {
		return 0, fmt.Errorf("the Agent Card fixture counter for scope %q is malformed: %s", scope, resp.Describe())
	}
	return *body.Count, nil
}

func (s *Steps) agentCardFixtureCountIs(ctx context.Context, want int, scope string) error {
	resolved, err := stepscommon.Expand(ctx, scope)
	if err != nil {
		return err
	}
	got, err := s.agentCardFixtureCount(ctx, resolved)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("the Agent Card fixture received %d requests for scope %q, want %d", got, resolved, want)
	}
	return nil
}

// fetchAgentCardUntilFixtureCount repeats the stored-handle card fetch until the upstream has
// been contacted the wanted number of times, publishing the final fetch response. It is how a
// scenario observes a cache entry expiring without a fixed wait.
func (s *Steps) fetchAgentCardUntilFixtureCount(ctx context.Context, handle string, want int, scope string) error {
	resolvedHandle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	resolvedScope, err := stepscommon.Expand(ctx, scope)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"agentProxyId": resolvedHandle})
	if err != nil {
		return err
	}
	last := -1
	_, err = retry.Until(ctx, retry.Options{Interval: time.Second},
		func(ctx context.Context) (bool, error) {
			if _, sendErr := s.sendControlPlane(ctx, controlPlaneCall{
				method: http.MethodPost, path: "/agent-proxies/fetch-agent-card", body: body,
			}); sendErr != nil {
				return false, retry.Transient(sendErr)
			}
			count, countErr := s.agentCardFixtureCount(ctx, resolvedScope)
			if countErr != nil {
				return false, retry.Transient(countErr)
			}
			last = count
			if count > want {
				return false, fmt.Errorf("the Agent Card fixture received %d requests for scope %q, more than the %d awaited",
					count, resolvedScope, want)
			}
			return count == want, nil
		}, func(done bool) bool { return done })
	if err != nil {
		return fmt.Errorf("waiting for the Agent Card fixture to receive %d requests for scope %q (last %d): %w",
			want, resolvedScope, last, err)
	}
	return nil
}

// obtainGatewayToken mints an additional key for the block's gateway through the publisher API
// and revokes it at cleanup. A gateway holds at most two active keys and the running gateway owns
// one, so a key another runner holds is waited out rather than treated as a failure.
func (s *Steps) obtainGatewayToken(ctx context.Context) error {
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}
	gatewayID, err := s.gatewayUUID(ctx, base, bearer)
	if err != nil {
		return err
	}
	path := "/gateways/" + url.PathEscape(gatewayID) + "/tokens"
	var minted struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	err = retry.Await(ctx, retry.Options{Interval: 2 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, callErr := s.client.Do(ctx, httpx.Request{
				Method: http.MethodPost, URL: base + apiBase + path, Body: []byte(`{}`), ContentType: "application/json",
				Headers: map[string]string{"Authorization": "Bearer " + bearer},
			}, 0, 0)
			if callErr != nil {
				return nil, retry.Transient(callErr)
			}
			if resp.StatusCode == http.StatusConflict && errorCode(resp) == "GATEWAY_TOKEN_LIMIT_REACHED" {
				return resp, nil
			}
			if resp.StatusCode != http.StatusCreated {
				return nil, fmt.Errorf("minting a gateway key: %s", resp.Describe())
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool {
			return resp != nil && resp.StatusCode == http.StatusCreated && json.Unmarshal(resp.Body, &minted) == nil
		},
		"waiting for a free gateway key slot")
	if err != nil {
		return err
	}
	if minted.ID == "" || minted.Token == "" {
		return fmt.Errorf("the control plane minted a gateway key without an id or token")
	}
	revoke := func(ctx context.Context) error { return s.revokeGatewayToken(ctx, gatewayID, minted.ID) }
	if err := s.registerTracked(ctx, platformGatewayTokenKind, gatewayID+agentProxyRecordD+minted.ID, revoke,
		func(reg *cleanup.Registry) error {
			return reg.RegisterDeleter(platformGatewayTokenKind, func(ctx context.Context, res cleanup.Resource) error {
				parts := strings.Split(res.ID, agentProxyRecordD)
				if len(parts) != 2 {
					return fmt.Errorf("invalid gateway key cleanup record")
				}
				return s.revokeGatewayToken(ctx, parts[0], parts[1])
			})
		}); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyGatewayToken, minted.Token)
}

func (s *Steps) revokeGatewayToken(ctx context.Context, gatewayID, tokenID string) error {
	resp, err := s.adminCall(ctx, http.MethodDelete, "/gateways/"+url.PathEscape(gatewayID)+"/tokens/"+url.PathEscape(tokenID))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNotFound && !resp.Succeeded() {
		return fmt.Errorf("revoking gateway key %q: %s", tokenID, resp.Describe())
	}
	return nil
}

func gatewayToken(ctx context.Context) (string, error) {
	value, ok := tcontext.Get(ctx, keyGatewayToken)
	token, isString := value.(string)
	if !ok || !isString || token == "" {
		return "", fmt.Errorf("no gateway key has been obtained in this scenario; obtain one first")
	}
	return token, nil
}

// sendGatewayInternal issues one gateway-internal API request as the block's gateway, or with
// no key at all, and publishes the response.
func (s *Steps) sendGatewayInternal(ctx context.Context, method, path, withoutKey string) error {
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resolved, "/") {
		return fmt.Errorf("gateway internal path %q must start with /", resolved)
	}
	headers := map[string]string{}
	if withoutKey == "" {
		token, tokenErr := gatewayToken(ctx)
		if tokenErr != nil {
			return tokenErr
		}
		headers[gatewayKeyHeader] = token
	}
	base, err := s.baseURL()
	if err != nil {
		return err
	}
	_, err = s.funnel.Send(ctx, httpx.Request{
		Method: strings.ToUpper(method), URL: base + internalAPIBase + resolved, Headers: headers,
	})
	return err
}

// resolveInternalArtifactID finds a deployment in the gateway's own deployment listing and
// stores the artifact identifier the gateway addresses it by. The public API never exposes that
// identifier, which is the point: the internal routes key on it, the public ones on the handle.
func (s *Steps) resolveInternalArtifactID(ctx context.Context, deployment, key string) error {
	deploymentID, err := stepscommon.Expand(ctx, deployment)
	if err != nil {
		return err
	}
	token, err := gatewayToken(ctx)
	if err != nil {
		return err
	}
	base, err := s.baseURL()
	if err != nil {
		return err
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodGet, URL: base + internalAPIBase + "/deployments",
		Headers: map[string]string{gatewayKeyHeader: token},
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("listing the gateway's deployments: %w", err)
	}
	artifactID, err := internalArtifactID(resp, deploymentID)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, key, artifactID)
}

// internalArtifactID selects a deployment from the gateway deployment listing, requiring the
// gateway's vocabulary for an Agent proxy.
func internalArtifactID(resp *httpx.Response, deploymentID string) (string, error) {
	if resp == nil || resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("listing the gateway's deployments: %s", resp.Describe())
	}
	var body struct {
		Deployments []struct {
			ArtifactID   string `json:"artifactId"`
			DeploymentID string `json:"deploymentId"`
			Kind         string `json:"kind"`
		} `json:"deployments"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return "", fmt.Errorf("decoding the gateway's deployments: %w", err)
	}
	for _, d := range body.Deployments {
		if d.DeploymentID != deploymentID {
			continue
		}
		if d.Kind != "Agent" {
			return "", fmt.Errorf("the gateway lists deployment %q as kind %q, want Agent", deploymentID, d.Kind)
		}
		if d.ArtifactID == "" {
			return "", fmt.Errorf("the gateway lists deployment %q without an artifact id", deploymentID)
		}
		return d.ArtifactID, nil
	}
	return "", fmt.Errorf("the gateway's deployment listing does not include deployment %q", deploymentID)
}

func (s *Steps) fetchDeploymentBatch(ctx context.Context, deployment string) error {
	deploymentID, err := stepscommon.Expand(ctx, deployment)
	if err != nil {
		return err
	}
	token, err := gatewayToken(ctx)
	if err != nil {
		return err
	}
	base, err := s.baseURL()
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string][]string{"deploymentIds": {deploymentID}})
	if err != nil {
		return err
	}
	_, err = s.funnel.Send(ctx, httpx.Request{
		Method: http.MethodPost, URL: base + internalAPIBase + "/deployments/fetch-batch", Body: body,
		Headers: map[string]string{
			gatewayKeyHeader: token, "Content-Type": "application/json", "Accept": "application/x-tar+gzip",
		},
	})
	return err
}

// archiveEntries reads a published ZIP or gzip-compressed tar body into its regular entries.
func archiveEntries(format string, body []byte) (map[string][]byte, error) {
	entries := map[string][]byte{}
	switch format {
	case "ZIP":
		reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			return nil, fmt.Errorf("the response is not a ZIP archive: %w", err)
		}
		for _, file := range reader.File {
			if file.FileInfo().IsDir() {
				continue
			}
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("opening ZIP entry %q: %w", file.Name, err)
			}
			content, err := io.ReadAll(io.LimitReader(rc, int64(len(body))*64+1))
			_ = rc.Close()
			if err != nil {
				return nil, fmt.Errorf("reading ZIP entry %q: %w", file.Name, err)
			}
			entries[file.Name] = content
		}
	case "gzip tar":
		gz, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("the response is not gzip-compressed: %w", err)
		}
		defer func() { _ = gz.Close() }()
		reader := tar.NewReader(gz)
		for {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("reading the tar archive: %w", err)
			}
			if header.Typeflag != tar.TypeReg {
				continue
			}
			content, err := io.ReadAll(io.LimitReader(reader, int64(len(body))*64+1))
			if err != nil {
				return nil, fmt.Errorf("reading tar entry %q: %w", header.Name, err)
			}
			entries[header.Name] = content
		}
	default:
		return nil, fmt.Errorf("unsupported archive format %q", format)
	}
	return entries, nil
}

// publishedArchive returns the single deployment document in the published archive. ZIP is
// tried first because the artifact fetch serves ZIP; the batch fetch serves a gzip tar.
func publishedArchive(ctx context.Context) (string, []byte, error) {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return "", nil, err
	}
	format := "ZIP"
	if bytes.HasPrefix(resp.Body, []byte{0x1f, 0x8b}) {
		format = "gzip tar"
	}
	entries, err := archiveEntries(format, resp.Body)
	if err != nil {
		return "", nil, err
	}
	if len(entries) != 1 {
		return "", nil, fmt.Errorf("the archive holds %d entries %v, want exactly one deployment document", len(entries), sortedKeys(entries))
	}
	for name, content := range entries {
		return name, content, nil
	}
	return "", nil, fmt.Errorf("the archive is empty")
}

func (s *Steps) archiveContainsOnly(ctx context.Context, format, name string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	want, err := stepscommon.Expand(ctx, name)
	if err != nil {
		return err
	}
	entries, err := archiveEntries(format, resp.Body)
	if err != nil {
		return fmt.Errorf("%w (%s)", err, resp.Describe())
	}
	if len(entries) != 1 {
		return fmt.Errorf("the %s archive holds %v, want only %q", format, sortedKeys(entries), want)
	}
	if _, ok := entries[want]; !ok {
		return fmt.Errorf("the %s archive holds %v, want only %q", format, sortedKeys(entries), want)
	}
	return nil
}

func (s *Steps) archivedFieldIs(ctx context.Context, field, want string) error {
	expected, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	name, document, err := publishedArchiveDocument(ctx)
	if err != nil {
		return err
	}
	got, ok := traverse(document, field)
	if !ok {
		return fmt.Errorf("archived deployment %q has no field %q", name, field)
	}
	if text := renderValue(got); text != expected {
		return fmt.Errorf("archived deployment %q field %q: expected %q, got %q", name, field, expected, text)
	}
	return nil
}

func (s *Steps) archivedFieldAbsent(ctx context.Context, field string) error {
	name, document, err := publishedArchiveDocument(ctx)
	if err != nil {
		return err
	}
	if got, ok := traverse(document, field); ok {
		return fmt.Errorf("archived deployment %q field %q should be absent, but holds %v", name, field, got)
	}
	return nil
}

func publishedArchiveDocument(ctx context.Context) (string, any, error) {
	name, content, err := publishedArchive(ctx)
	if err != nil {
		return "", nil, err
	}
	var document any
	if err := yaml.Unmarshal(content, &document); err != nil {
		return "", nil, fmt.Errorf("archived deployment %q is not YAML: %w", name, err)
	}
	return name, document, nil
}

// renderValue renders a decoded scalar the way a feature writes it; composite values render as
// compact JSON so an array or object can be compared exactly.
func renderValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return v
	case map[string]any, []any:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(encoded)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// traverse walks a dotted path through decoded JSON or YAML, honouring [n] array indices.
func traverse(doc any, path string) (any, bool) {
	current := doc
	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			continue
		}
		name, indices, ok := splitSegment(segment)
		if !ok {
			return nil, false
		}
		if name != "" {
			object, isObject := current.(map[string]any)
			if !isObject {
				return nil, false
			}
			if current, isObject = object[name]; !isObject {
				return nil, false
			}
		}
		for _, index := range indices {
			array, isArray := current.([]any)
			if !isArray || index < 0 || index >= len(array) {
				return nil, false
			}
			current = array[index]
		}
	}
	return current, true
}

func splitSegment(segment string) (string, []int, bool) {
	open := strings.Index(segment, "[")
	if open < 0 {
		return segment, nil, true
	}
	name := segment[:open]
	var indices []int
	for rest := segment[open:]; rest != ""; {
		if !strings.HasPrefix(rest, "[") {
			return "", nil, false
		}
		end := strings.Index(rest, "]")
		if end < 0 {
			return "", nil, false
		}
		index, err := strconv.Atoi(rest[1:end])
		if err != nil {
			return "", nil, false
		}
		indices = append(indices, index)
		rest = rest[end+1:]
	}
	return name, indices, true
}

func sortedKeys(entries map[string][]byte) []string {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// createSecretWithValue creates a GENERIC secret holding a caller-chosen value, for a scenario
// whose upstream checks the credential the secret resolves to.
func (s *Steps) createSecretWithValue(ctx context.Context, handle, value string) error {
	resolvedValue, err := stepscommon.Expand(ctx, value)
	if err != nil {
		return err
	}
	return s.createSecretValued(ctx, handle, resolvedValue)
}

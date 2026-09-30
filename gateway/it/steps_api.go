/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package it

import (
	"encoding/base64"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
	"gopkg.in/yaml.v3"
)

// policyPropagationDelay is how long a gateway request waits after an
// accepted mutation for the router and the Policy Engine to apply it, unless
// a step has already polled the change into place.
const policyPropagationDelay = 1 * time.Second

// deployedAPINamesContextKey holds the API names this scenario deployed. It
// lives in TestState.Context so TestState.Reset clears it per scenario.
const deployedAPINamesContextKey = "deployedAPINames"

// apiConfigMetadata captures just enough of a RestApi or Agent
// configuration's YAML to recover its name, the route it serves and whether
// it carries mtls-auth; every other field is ignored.
type apiConfigMetadata struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Context    string `yaml:"context"`
		Version    string `yaml:"version"`
		Operations []struct {
			Method string `yaml:"method"`
			Path   string `yaml:"path"`
		} `yaml:"operations"`
		A2A struct {
			OperationConfigs struct {
				Transports []struct {
					PathPrefix string `yaml:"pathPrefix"`
				} `yaml:"transports"`
			} `yaml:"operationConfigs"`
		} `yaml:"a2a"`
	} `yaml:"spec"`
}

// deployedRoutesContextKey holds the route of every API and Agent this
// scenario deployed, keyed by the configuration's name, so cleanup can see
// each delete reach the router.
const deployedRoutesContextKey = "deployedRoutes"

// deployedRoute is the plain-listener route a deployed configuration serves.
type deployedRoute struct {
	name     string
	method   string
	path     string
	mtlsAuth bool
}

// deployedAgentNamesContextKey holds the Agent names this scenario deployed,
// on the same terms as deployedAPINamesContextKey.
const deployedAgentNamesContextKey = "deployedAgentNames"

// pathParamPattern matches a {param} segment of an operation path.
var pathParamPattern = regexp.MustCompile(`\{[^}/]*\}`)

// mtlsAuthPolicyPattern matches an mtls-auth policy entry in a configuration.
var mtlsAuthPolicyPattern = regexp.MustCompile(`(?m)name:\s*["']?mtls-auth["']?\s*$`)

// recordDeployedAPIName records the configuration's metadata.name and route
// for end-of-scenario cleanup. A body without a parseable name created
// nothing, so it is skipped.
func recordDeployedAPIName(state *TestState, body string) {
	recordDeployedName(state, deployedAPINamesContextKey, body)
}

// recordDeployedAgentName records an Agent configuration's metadata.name and
// route for end-of-scenario cleanup.
func recordDeployedAgentName(state *TestState, body string) {
	recordDeployedName(state, deployedAgentNamesContextKey, body)
}

func recordDeployedName(state *TestState, key, body string) {
	var cfg apiConfigMetadata
	if err := yaml.Unmarshal([]byte(body), &cfg); err != nil || cfg.Metadata.Name == "" {
		return
	}
	existing, _ := state.GetContextValue(key)
	names, _ := existing.([]string)
	names = append(names, cfg.Metadata.Name)
	state.SetContextValue(key, names)

	routes := deployedRoutes(state)
	routes[cfg.Metadata.Name] = routeOf(cfg, body)
	state.SetContextValue(deployedRoutesContextKey, routes)
}

// routeOf derives the route a configuration serves on the plain listener: an
// API's first operation, or an Agent's first transport, which takes POST.
func routeOf(cfg apiConfigMetadata, body string) deployedRoute {
	route := deployedRoute{
		name:     cfg.Metadata.Name,
		method:   http.MethodGet,
		path:     strings.ReplaceAll(cfg.Spec.Context, "$version", cfg.Spec.Version),
		mtlsAuth: mtlsAuthPolicyPattern.MatchString(body),
	}
	switch {
	case len(cfg.Spec.Operations) > 0:
		op := cfg.Spec.Operations[0]
		route.method = strings.ToUpper(op.Method)
		route.path += pathParamPattern.ReplaceAllString(op.Path, "x")
	case len(cfg.Spec.A2A.OperationConfigs.Transports) > 0:
		route.method = http.MethodPost
		route.path += strings.TrimSuffix(cfg.Spec.A2A.OperationConfigs.Transports[0].PathPrefix, "/")
	}
	return route
}

// deployedRoutes returns a copy of the routes this scenario recorded.
func deployedRoutes(state *TestState) map[string]deployedRoute {
	raw, _ := state.GetContextValue(deployedRoutesContextKey)
	recorded, _ := raw.(map[string]deployedRoute)
	routes := make(map[string]deployedRoute, len(recorded)+1)
	for name, route := range recorded {
		routes[name] = route
	}
	return routes
}

// deployedConfigCarriesMTLSAuth reports whether the named configuration, as
// this scenario last deployed it, carries mtls-auth.
func deployedConfigCarriesMTLSAuth(state *TestState, name string) bool {
	route, ok := deployedRoutes(state)[name]
	return ok && route.mtlsAuth
}

// recordUpdatedRoute replaces the recorded route of an API this scenario
// deployed with the one its update serves.
func recordUpdatedRoute(state *TestState, name, body string) {
	routes := deployedRoutes(state)
	if _, ok := routes[name]; !ok {
		return
	}
	var cfg apiConfigMetadata
	if err := yaml.Unmarshal([]byte(body), &cfg); err != nil {
		return
	}
	cfg.Metadata.Name = name
	routes[name] = routeOf(cfg, body)
	state.SetContextValue(deployedRoutesContextKey, routes)
}

// deployAPIConfiguration POSTs a RestApi configuration to the gateway
// controller, records its name for cleanup, and marks policy propagation
// pending if the controller accepted it.
func deployAPIConfiguration(state *TestState, httpSteps *steps.HTTPSteps, body string) error {
	recordDeployedAPIName(state, body)
	httpSteps.SetHeader("Content-Type", "application/yaml")
	if err := httpSteps.SendPOSTToService("gateway-controller", "/rest-apis", &godog.DocString{Content: body}); err != nil {
		return err
	}
	markPropagationPendingIfAccepted(state, httpSteps, mtlsAuthPolicyPattern.MatchString(body))
	return nil
}

// markPropagationPendingIfAccepted marks policy propagation pending only
// after a mutation the controller accepted (2xx). A refused mutation leaves
// the gateway untouched, so nothing needs to wait for it. The wait itself is
// paid by the next gateway request, unless an endpoint-wait step polls the
// change into place first.
func markPropagationPendingIfAccepted(state *TestState, httpSteps *steps.HTTPSteps, changesListener bool) {
	if resp := httpSteps.LastResponse(); resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return
	}
	markPropagationPending(state, changesListener)
}

// updateAPIConfiguration PUTs a RestApi configuration to the gateway
// controller under the given API name and marks policy propagation pending.
func updateAPIConfiguration(state *TestState, httpSteps *steps.HTTPSteps, apiName, body string) error {
	httpSteps.SetHeader("Content-Type", "application/yaml")
	changesListener := deployedConfigCarriesMTLSAuth(state, apiName) || mtlsAuthPolicyPattern.MatchString(body)
	if err := httpSteps.SendPUTToService("gateway-controller", "/rest-apis/"+apiName, &godog.DocString{Content: body}); err != nil {
		return err
	}
	markPropagationPendingIfAccepted(state, httpSteps, changesListener)
	recordUpdatedRoute(state, apiName, body)
	return nil
}

// cleanupDeployedAPIs deletes, as admin, every API this scenario recorded.
// It is best-effort: an API already deleted or never created just gets a 404.
func cleanupDeployedAPIs(state *TestState, httpSteps *steps.HTTPSteps) {
	cleanupDeployed(state, httpSteps, deployedAPINamesContextKey, "/rest-apis/")
}

// cleanupDeployedAgents deletes, as admin, every Agent this scenario
// recorded, on the same best-effort terms as cleanupDeployedAPIs.
func cleanupDeployedAgents(state *TestState, httpSteps *steps.HTTPSteps) {
	cleanupDeployed(state, httpSteps, deployedAgentNamesContextKey, "/agents/")
}

func cleanupDeployed(state *TestState, httpSteps *steps.HTTPSteps, key, pathPrefix string) {
	raw, ok := state.GetContextValue(key)
	if !ok {
		return
	}
	names, ok := raw.([]string)
	if !ok || len(names) == 0 {
		return
	}

	admin, ok := state.Config.Users["admin"]
	if !ok {
		return
	}
	creds := base64.StdEncoding.EncodeToString([]byte(admin.Username + ":" + admin.Password))
	httpSteps.SetHeader("Authorization", "Basic "+creds)

	for _, name := range names {
		_ = httpSteps.SendDELETEToService("gateway-controller", pathPrefix+name)
	}
}

// RegisterAPISteps registers all API deployment step definitions
func RegisterAPISteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	// Single deploy function used by multiple step patterns
	deployAPI := func(body *godog.DocString) error {
		return deployAPIConfiguration(state, httpSteps, body.Content)
	}

	// Single delete function used by multiple step patterns
	deleteAPI := func(name string) error {
		err := httpSteps.SendDELETEToService("gateway-controller", "/rest-apis/"+name)
		if err != nil {
			return err
		}
		markPropagationPendingIfAccepted(state, httpSteps, deployedConfigCarriesMTLSAuth(state, name))
		return nil
	}

	// Register multiple step patterns for deploy
	ctx.Step(`^I deploy this API configuration:$`, deployAPI)
	ctx.Step(`^I deploy an API with the following configuration:$`, deployAPI)
	ctx.Step(`^I deploy a test API with the following configuration:$`, deployAPI)

	// Register multiple step patterns for delete
	ctx.Step(`^I delete the API "([^"]*)"$`, deleteAPI)
	// Note: Version parameter is semantically meaningful in tests but not used by the API endpoint.
	// The API deletes by name only - version is embedded in the API YAML, not in the DELETE path.
	ctx.Step(`^I delete the API "([^"]*)" version "([^"]*)"$`, func(name, version string) error {
		return deleteAPI(name)
	})

	ctx.Step(`^I update the API "([^"]*)" with this configuration:$`, func(apiName string, body *godog.DocString) error {
		return updateAPIConfiguration(state, httpSteps, apiName, body.Content)
	})

	ctx.Step(`^I get the API "([^"]*)"$`, func(name string) error {
		return httpSteps.SendGETToService("gateway-controller", "/rest-apis/"+name)
	})
}

// scenarioHasTag reports whether the scenario, or the feature it belongs to,
// carries the given tag (godog copies feature-level tags onto each scenario).
func scenarioHasTag(sc *godog.Scenario, tag string) bool {
	for _, t := range sc.Tags {
		if t.Name == tag {
			return true
		}
	}
	return false
}

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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/wso2/api-platform/gateway/it/steps"
)

// PolicyEngineSteps wraps TestState and HTTPSteps for policy-engine specific step definitions
type PolicyEngineSteps struct {
	state     *TestState
	httpSteps *steps.HTTPSteps
}

// RegisterPolicyEngineSteps registers all policy-engine specific step definitions
func RegisterPolicyEngineSteps(ctx *godog.ScenarioContext, state *TestState, httpSteps *steps.HTTPSteps) {
	p := &PolicyEngineSteps{state: state, httpSteps: httpSteps}

	// Config dump endpoint steps
	ctx.Step(`^I send a GET request to the policy-engine config dump endpoint$`, p.iSendGETRequestToConfigDumpEndpoint)
	ctx.Step(`^I send a POST request to the policy-engine config dump endpoint$`, p.iSendPOSTRequestToConfigDumpEndpoint)

	// JSON structure validation steps
	ctx.Step(`^the response JSON should have key "([^"]*)"$`, p.theResponseJSONShouldHaveKey)
	ctx.Step(`^the response JSON at "([^"]*)" should have key "([^"]*)"$`, p.theResponseJSONAtPathShouldHaveKey)
	ctx.Step(`^the response JSON at "([^"]*)" should be greater than (\d+)$`, p.theResponseJSONAtPathShouldBeGreaterThan)

	// Route configuration validation steps
	ctx.Step(`^the config dump should contain route with basePath "([^"]*)"$`, p.theConfigDumpShouldContainRouteWithBasePath)
	ctx.Step(`^the config dump should not contain route with basePath "([^"]*)"$`, p.theConfigDumpShouldNotContainRouteWithBasePath)
	ctx.Step(`^the config dump should contain policy "([^"]*)" for route "([^"]*)"$`, p.theConfigDumpShouldContainPolicyForRoute)
}

// iSendGETRequestToConfigDumpEndpoint sends a GET request to the policy-engine config dump endpoint
func (p *PolicyEngineSteps) iSendGETRequestToConfigDumpEndpoint() error {
	url := fmt.Sprintf("%s/config_dump", p.state.Config.PolicyEngineURL)
	return p.httpSteps.ISendGETRequest(url)
}

// iSendPOSTRequestToConfigDumpEndpoint sends a POST request to the policy-engine config dump endpoint
func (p *PolicyEngineSteps) iSendPOSTRequestToConfigDumpEndpoint() error {
	url := fmt.Sprintf("%s/config_dump", p.state.Config.PolicyEngineURL)
	return p.httpSteps.ISendPOSTRequest(url)
}

// theResponseJSONShouldHaveKey validates that the response JSON has a specific top-level key
func (p *PolicyEngineSteps) theResponseJSONShouldHaveKey(key string) error {
	body := p.httpSteps.LastBody()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("failed to parse response as JSON: %w", err)
	}

	if _, exists := data[key]; !exists {
		return fmt.Errorf("response JSON does not have key '%s'. Available keys: %v", key, getKeys(data))
	}
	return nil
}

// theResponseJSONAtPathShouldHaveKey validates that a nested JSON path has a specific key
func (p *PolicyEngineSteps) theResponseJSONAtPathShouldHaveKey(path, key string) error {
	body := p.httpSteps.LastBody()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("failed to parse response as JSON: %w", err)
	}

	// Navigate to the path
	value, err := navigateJSONPath(data, path)
	if err != nil {
		return err
	}

	// Check if the value is a map and has the key
	nested, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("value at path '%s' is not an object, it is %T", path, value)
	}

	if _, exists := nested[key]; !exists {
		return fmt.Errorf("JSON at path '%s' does not have key '%s'. Available keys: %v", path, key, getKeys(nested))
	}
	return nil
}

// theResponseJSONAtPathShouldBeGreaterThan validates that a numeric value at a path is greater than expected
func (p *PolicyEngineSteps) theResponseJSONAtPathShouldBeGreaterThan(path string, expected int) error {
	body := p.httpSteps.LastBody()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("failed to parse response as JSON: %w", err)
	}

	value, err := navigateJSONPath(data, path)
	if err != nil {
		return err
	}

	// Convert to number
	var num float64
	switch v := value.(type) {
	case float64:
		num = v
	case int:
		num = float64(v)
	case int64:
		num = float64(v)
	default:
		return fmt.Errorf("value at path '%s' is not a number, it is %T", path, value)
	}

	if int(num) <= expected {
		return fmt.Errorf("value at path '%s' is %d, expected greater than %d", path, int(num), expected)
	}
	return nil
}

// theConfigDumpShouldContainRouteWithBasePath validates that the config dump contains a route with the given base path
func (p *PolicyEngineSteps) theConfigDumpShouldContainRouteWithBasePath(basePath string) error {
	body := p.httpSteps.LastBody()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("failed to parse response as JSON: %w", err)
	}

	routes, ok := data["routes"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("routes section not found in config dump")
	}

	routeConfigs, ok := routes["route_configs"].([]interface{})
	if !ok {
		return fmt.Errorf("route_configs not found in routes section")
	}

	for _, rc := range routeConfigs {
		routeConfig, ok := rc.(map[string]interface{})
		if !ok {
			continue
		}
		routeKey, _ := routeConfig["route_key"].(string)
		// RouteKey format is typically "method_basepath_path" or similar
		if strings.Contains(routeKey, basePath) {
			return nil
		}
	}

	return fmt.Errorf("no route found with basePath '%s' in config dump. Total routes: %d", basePath, len(routeConfigs))
}

// theConfigDumpShouldNotContainRouteWithBasePath validates that the config dump does NOT contain a route with the given base path
func (p *PolicyEngineSteps) theConfigDumpShouldNotContainRouteWithBasePath(basePath string) error {
	body := p.httpSteps.LastBody()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("failed to parse response as JSON: %w", err)
	}

	routes, ok := data["routes"].(map[string]interface{})
	if !ok {
		// No routes section means no routes, which is what we want
		return nil
	}

	routeConfigs, ok := routes["route_configs"].([]interface{})
	if !ok || len(routeConfigs) == 0 {
		// No route configs is fine
		return nil
	}

	for _, rc := range routeConfigs {
		routeConfig, ok := rc.(map[string]interface{})
		if !ok {
			continue
		}
		routeKey, _ := routeConfig["route_key"].(string)
		if strings.Contains(routeKey, basePath) {
			return fmt.Errorf("route with basePath '%s' still exists in config dump (route_key: %s)", basePath, routeKey)
		}
	}

	return nil
}

// theConfigDumpShouldContainPolicyForRoute validates that a specific policy exists for a route
func (p *PolicyEngineSteps) theConfigDumpShouldContainPolicyForRoute(policyName, routeBasePath string) error {
	body := p.httpSteps.LastBody()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("failed to parse response as JSON: %w", err)
	}

	routes, ok := data["routes"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("routes section not found in config dump")
	}

	routeConfigs, ok := routes["route_configs"].([]interface{})
	if !ok {
		return fmt.Errorf("route_configs not found in routes section")
	}

	for _, rc := range routeConfigs {
		routeConfig, ok := rc.(map[string]interface{})
		if !ok {
			continue
		}
		routeKey, _ := routeConfig["route_key"].(string)
		if !strings.Contains(routeKey, routeBasePath) {
			continue
		}

		// Found the route, now check for the policy
		policies, ok := routeConfig["policies"].([]interface{})
		if !ok {
			return fmt.Errorf("policies not found for route '%s'", routeKey)
		}

		for _, pol := range policies {
			policy, ok := pol.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := policy["name"].(string)
			if name == policyName {
				return nil
			}
		}

		return fmt.Errorf("policy '%s' not found for route '%s'. Available policies: %v", policyName, routeKey, policies)
	}

	return fmt.Errorf("no route found with basePath '%s' in config dump", routeBasePath)
}

// navigateJSONPath navigates a dot-separated path in a JSON object
func navigateJSONPath(data map[string]interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			var exists bool
			current, exists = v[part]
			if !exists {
				return nil, fmt.Errorf("path '%s' not found (missing key '%s')", path, part)
			}
		default:
			return nil, fmt.Errorf("cannot navigate path '%s': value at '%s' is not an object", path, part)
		}
	}

	return current, nil
}

// getKeys returns the keys of a map as a slice of strings
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

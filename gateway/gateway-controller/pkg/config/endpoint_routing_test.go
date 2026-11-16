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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// routingPolicyDefinition is a policy definition shaped like dynamic-endpoint
// v1.1: targetUpstream is the v1.0 parameter and carries no annotation.
func routingPolicyDefinition(name, version string) models.PolicyDefinition {
	return models.PolicyDefinition{
		Name:    name,
		Version: version,
		Parameters: &map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"targetUpstream": map[string]interface{}{"type": "string"},
				"targetEndpoint": map[string]interface{}{"type": "string", EndpointRoutingAnnotation: EndpointRoutingEndpointName},
				"endpointHeader": map[string]interface{}{"type": "string", EndpointRoutingAnnotation: EndpointRoutingHeaderName},
				"endpointSelector": map[string]interface{}{
					"type": "object",
				},
			},
		},
	}
}

func routingDefinitions() map[string]models.PolicyDefinition {
	return map[string]models.PolicyDefinition{
		"route-policy|v1.1.0": routingPolicyDefinition("route-policy", "v1.1.0"),
		"plain-policy|v1.0.0": {
			Name: "plain-policy", Version: "v1.0.0",
			Parameters: &map[string]interface{}{"properties": map[string]interface{}{
				"targetEndpoint": map[string]interface{}{"type": "string"},
			}},
		},
		"no-params|v1.0.0": {Name: "no-params", Version: "v1.0.0"},
		"odd-annotation|v1.0.0": {
			Name: "odd-annotation", Version: "v1.0.0",
			Parameters: &map[string]interface{}{"properties": map[string]interface{}{
				"target": map[string]interface{}{"type": "string", EndpointRoutingAnnotation: "somethingElse"},
				"flag":   map[string]interface{}{"type": "string", EndpointRoutingAnnotation: true},
			}},
		},
	}
}

func TestEndpointRouting_InstanceRoutes(t *testing.T) {
	routing := NewEndpointRouting(routingDefinitions())

	tests := []struct {
		name    string
		policy  string
		version string
		params  map[string]interface{}
		want    bool
	}{
		{"fixed target", "route-policy", "v1.1.0", map[string]interface{}{"targetEndpoint": "tenant-a"}, true},
		{"header mode", "route-policy", "v1.1.0", map[string]interface{}{"endpointHeader": "x-endpoint"}, true},
		{"v1.0 parameters only", "route-policy", "v1.1.0", map[string]interface{}{"targetUpstream": "local"}, false},
		{"annotated parameter left empty", "route-policy", "v1.1.0", map[string]interface{}{"targetEndpoint": "  "}, false},
		{"annotated parameter of the wrong type", "route-policy", "v1.1.0", map[string]interface{}{"targetEndpoint": 7}, false},
		{"no parameters", "route-policy", "v1.1.0", nil, false},
		{"another version of the policy", "route-policy", "v1.0.1", map[string]interface{}{"targetEndpoint": "tenant-a"}, false},
		{"major-only version, as a policy chain carries it", "route-policy", "v1", map[string]interface{}{"targetEndpoint": "tenant-a"}, true},
		{"another major", "route-policy", "v2", map[string]interface{}{"targetEndpoint": "tenant-a"}, false},
		{"same parameter name without the annotation", "plain-policy", "v1.0.0", map[string]interface{}{"targetEndpoint": "tenant-a"}, false},
		{"definition without parameters", "no-params", "v1.0.0", map[string]interface{}{"targetEndpoint": "tenant-a"}, false},
		{"unrecognised annotation values", "odd-annotation", "v1.0.0", map[string]interface{}{"target": "tenant-a", "flag": "x"}, false},
		{"unknown policy", "missing", "v1.0.0", map[string]interface{}{"targetEndpoint": "tenant-a"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, routing.InstanceRoutes(tc.policy, tc.version, tc.params))
		})
	}
}

// A major with two loaded versions does not resolve — an API attaching it is
// refused at deploy — so it must not be treated as routing either.
func TestEndpointRouting_AmbiguousMajorDoesNotRoute(t *testing.T) {
	defs := routingDefinitions()
	defs["route-policy|v1.2.0"] = routingPolicyDefinition("route-policy", "v1.2.0")
	routing := NewEndpointRouting(defs)

	params := map[string]interface{}{"targetEndpoint": "tenant-a"}
	assert.False(t, routing.InstanceRoutes("route-policy", "v1", params))
	assert.True(t, routing.InstanceRoutes("route-policy", "v1.1.0", params))
	assert.True(t, routing.InstanceRoutes("route-policy", "v1.2.0", params))
}

// An attachment with no version means the latest loaded one.
func TestEndpointRouting_StaticTargetsWithNoVersionUsesTheLatest(t *testing.T) {
	routing := NewEndpointRouting(routingDefinitions())
	policies := []api.Policy{{Name: "route-policy", Params: &map[string]interface{}{"targetEndpoint": "tenant-a"}}}
	assert.Equal(t, []string{"tenant-a"}, routing.StaticTargets(&policies))
}

func TestEndpointRouting_NilIsInert(t *testing.T) {
	var routing *EndpointRouting
	assert.False(t, routing.InstanceRoutes("route-policy", "v1.1.0", map[string]interface{}{"targetEndpoint": "tenant-a"}))
	assert.Nil(t, routing.StaticTargets(&[]api.Policy{{Name: "route-policy", Version: "v1"}}))
}

func TestEndpointRouting_StaticTargets(t *testing.T) {
	routing := NewEndpointRouting(routingDefinitions())
	params := func(m map[string]interface{}) *map[string]interface{} { return &m }

	policies := []api.Policy{
		// Fixed target, attached by major version as an API does.
		{Name: "route-policy", Version: "v1", Params: params(map[string]interface{}{"targetEndpoint": "tenant-default"})},
		// Header mode names no fixed Endpoint.
		{Name: "route-policy", Version: "v1", Params: params(map[string]interface{}{"endpointHeader": "x-endpoint"})},
		// Header mode with a fixed fallback names one.
		{Name: "route-policy", Version: "v1", Params: params(map[string]interface{}{
			"endpointHeader": "x-endpoint", "targetEndpoint": "tenant-fallback",
		})},
		// Not annotated, unresolvable, or without parameters: nothing.
		{Name: "plain-policy", Version: "v1", Params: params(map[string]interface{}{"targetEndpoint": "ignored"})},
		{Name: "route-policy", Version: "v9", Params: params(map[string]interface{}{"targetEndpoint": "ignored"})},
		{Name: "route-policy", Version: "v1"},
	}

	assert.ElementsMatch(t, []string{"tenant-default", "tenant-fallback"}, routing.StaticTargets(&policies))
	assert.Empty(t, routing.StaticTargets(nil))
}

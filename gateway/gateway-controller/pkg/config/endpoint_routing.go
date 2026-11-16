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
	"strings"

	versionutil "github.com/wso2/api-platform/common/version"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// EndpointRoutingAnnotation is the parameter-level annotation a policy
// definition puts on a parameter that routes requests to a gateway-wide
// Endpoint. The controller reads it from the definitions it already validates
// parameters against, so it never needs to know a routing policy by name.
const EndpointRoutingAnnotation = "x-wso2-endpoint-routing"

// Values of EndpointRoutingAnnotation.
const (
	// EndpointRoutingEndpointName marks a parameter whose value is the name of
	// one fixed Endpoint: a static reference from the API to that Endpoint.
	EndpointRoutingEndpointName = "endpointName"
	// EndpointRoutingHeaderName marks a parameter whose value is the request
	// header that carries the Endpoint name, chosen per request.
	EndpointRoutingHeaderName = "headerName"
)

// EndpointRouting answers, for a policy instance, whether it routes requests
// to gateway-wide Endpoints and which Endpoints it names as fixed targets. It
// is built once from the loaded policy definitions.
type EndpointRouting struct {
	latestVersions map[string]string
	// annotated maps "name|version" to that definition's annotated parameters
	// (parameter name -> annotation value). Each definition is indexed under
	// its full version and, when it is the only one of its major, under the
	// major-only version too: an API attaches a policy by major version, and
	// the policy chain carries it that way.
	annotated map[string]map[string]string
}

// NewEndpointRouting indexes the endpoint-routing parameters of definitions.
func NewEndpointRouting(definitions map[string]models.PolicyDefinition) *EndpointRouting {
	routing := &EndpointRouting{
		latestVersions: BuildLatestVersionIndex(definitions),
		annotated:      make(map[string]map[string]string),
	}

	// A major-only version resolves only when exactly one full version of that
	// major is loaded (ResolvePolicyVersion refuses an ambiguous one), so the
	// alias is added on the same condition.
	perMajor := make(map[string]int)
	for _, def := range definitions {
		if fullSemverPattern.MatchString(def.Version) {
			perMajor[endpointRoutingKey(def.Name, versionutil.MajorVersion(def.Version))]++
		}
	}
	for _, def := range definitions {
		params := endpointRoutingParameters(def)
		if len(params) == 0 {
			continue
		}
		routing.annotated[endpointRoutingKey(def.Name, def.Version)] = params
		if !fullSemverPattern.MatchString(def.Version) {
			continue
		}
		if majorKey := endpointRoutingKey(def.Name, versionutil.MajorVersion(def.Version)); perMajor[majorKey] == 1 {
			routing.annotated[majorKey] = params
		}
	}
	return routing
}

func endpointRoutingKey(name, version string) string {
	return name + "|" + version
}

// endpointRoutingParameters returns the top-level parameters of def that carry
// the endpoint-routing annotation with a recognised value.
func endpointRoutingParameters(def models.PolicyDefinition) map[string]string {
	if def.Parameters == nil {
		return nil
	}
	properties, ok := (*def.Parameters)["properties"].(map[string]interface{})
	if !ok {
		return nil
	}
	var annotated map[string]string
	for name, raw := range properties {
		schema, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := schema[EndpointRoutingAnnotation].(string)
		if role != EndpointRoutingEndpointName && role != EndpointRoutingHeaderName {
			continue
		}
		if annotated == nil {
			annotated = make(map[string]string)
		}
		annotated[name] = role
	}
	return annotated
}

// routes reports whether an instance of the policy at the given version (full
// or major-only) sets an endpoint-routing parameter, and the fixed Endpoints it
// names.
func (r *EndpointRouting) routes(name, version string, params map[string]interface{}) (bool, []string) {
	if r == nil || len(params) == 0 {
		return false, nil
	}
	annotated := r.annotated[endpointRoutingKey(name, strings.TrimSpace(version))]
	if len(annotated) == 0 {
		return false, nil
	}
	routes := false
	var targets []string
	for param, role := range annotated {
		value, _ := params[param].(string)
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		routes = true
		if role == EndpointRoutingEndpointName {
			targets = append(targets, value)
		}
	}
	return routes, targets
}

// InstanceRoutes reports whether a policy instance with the given parameters
// can route a request to a gateway-wide Endpoint. version is the instance's
// version as the policy chain carries it: major-only, or full. A route whose
// chain holds such an instance must let the policy engine choose its cluster.
func (r *EndpointRouting) InstanceRoutes(name, version string, params map[string]interface{}) bool {
	routes, _ := r.routes(name, version, params)
	return routes
}

// StaticTargets returns the Endpoints the attached policies name as fixed
// targets. A policy whose version does not resolve contributes nothing: it is
// not deployed either.
func (r *EndpointRouting) StaticTargets(policies *[]api.Policy) []string {
	if r == nil || policies == nil {
		return nil
	}
	var targets []string
	for _, policy := range *policies {
		if policy.Params == nil {
			continue
		}
		version := strings.TrimSpace(policy.Version)
		if version == "" {
			// No version means the latest loaded one.
			version = r.latestVersions[policy.Name]
		}
		_, instanceTargets := r.routes(policy.Name, version, *policy.Params)
		targets = append(targets, instanceTargets...)
	}
	return targets
}

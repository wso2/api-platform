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
	"sort"
	"strings"

	versionutil "github.com/wso2/api-platform/common/version"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// BackendRoutingAnnotation marks a policy parameter that makes a route select its cluster per
// request among the gateway's global Backends. The controller reads it from the policy
// definitions, so no policy name is hard-coded: a route is divertible when its chain sets such a
// parameter.
const BackendRoutingAnnotation = "x-choreolabs-backend-routing"

const (
	// BackendRoutingName annotates a parameter whose value names a fixed target: the API's
	// upstream definition of that name when the API has one, otherwise a Backend (e.g.
	// dynamic-endpoint's targetUpstream).
	BackendRoutingName = "name"
	// BackendRoutingExpression annotates a parameter holding an expression evaluated per request
	// to name the Backend (e.g. dynamic-endpoint's backendExpression).
	BackendRoutingExpression = "expression"
)

// BackendRoutingIndex maps a policy (name and major version) to its annotated parameters and
// their annotation values. Built once from the loaded policy definitions.
type BackendRoutingIndex map[string]map[string]string

func backendRoutingKey(name, majorVersion string) string {
	return name + "|" + majorVersion
}

// BuildBackendRoutingIndex collects every parameter annotated BackendRoutingAnnotation from the
// policy definitions' parameter schemas.
func BuildBackendRoutingIndex(definitions map[string]models.PolicyDefinition) BackendRoutingIndex {
	index := BackendRoutingIndex{}
	for _, def := range definitions {
		if def.Parameters == nil {
			continue
		}
		properties, ok := (*def.Parameters)["properties"].(map[string]interface{})
		if !ok {
			continue
		}
		for param, raw := range properties {
			schema, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			value, ok := schema[BackendRoutingAnnotation].(string)
			if !ok || value == "" {
				continue
			}
			key := backendRoutingKey(def.Name, versionutil.MajorVersion(def.Version))
			if index[key] == nil {
				index[key] = map[string]string{}
			}
			index[key][param] = value
		}
	}
	return index
}

// annotated returns the annotated parameters of a policy version, which may be major-only ("v1")
// or full ("v1.1.0").
func (idx BackendRoutingIndex) annotated(name, version string) map[string]string {
	if len(idx) == 0 {
		return nil
	}
	return idx[backendRoutingKey(name, versionutil.MajorVersion(strings.TrimSpace(version)))]
}

// RoutesToBackends reports whether a policy instance with these parameters may route a request to
// a global Backend: it sets at least one annotated parameter to a non-empty value.
func (idx BackendRoutingIndex) RoutesToBackends(name, version string, params map[string]interface{}) bool {
	for param := range idx.annotated(name, version) {
		if isSetParam(params[param]) {
			return true
		}
	}
	return false
}

// StaticBackendRef is one parameter of a policy instance that names a fixed target, which is a
// Backend unless the API has an upstream definition of the same name.
type StaticBackendRef struct {
	// Param is the parameter name, e.g. targetUpstream.
	Param string
	// Backend is the name it holds.
	Backend string
}

// StaticBackendRefs returns the parameters of a policy instance annotated BackendRoutingName
// that are set, ordered by parameter name. The caller excludes names that resolve to one of the
// API's own upstream definitions.
func (idx BackendRoutingIndex) StaticBackendRefs(name, version string, params map[string]interface{}) []StaticBackendRef {
	var refs []StaticBackendRef
	for param, annotation := range idx.annotated(name, version) {
		if annotation != BackendRoutingName {
			continue
		}
		if value, ok := params[param].(string); ok && strings.TrimSpace(value) != "" {
			refs = append(refs, StaticBackendRef{Param: param, Backend: strings.TrimSpace(value)})
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Param < refs[j].Param })
	return refs
}

func isSetParam(value interface{}) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(v) != ""
	default:
		return true
	}
}

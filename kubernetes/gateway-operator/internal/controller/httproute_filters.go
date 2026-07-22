/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package controller

import (
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

// dynamicEndpointPolicyName is the name of the dynamic-endpoint routing policy.
const dynamicEndpointPolicyName = "dynamic-endpoint"

func policyFromParams(name, version string, params map[string]interface{}) (apiv1.Policy, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return apiv1.Policy{}, err
	}
	return apiv1.Policy{
		Name:    name,
		Version: version,
		Params:  &runtime.RawExtension{Raw: raw},
	}, nil
}

func policiesFromHTTPRouteFilters(filters []gatewayv1.HTTPRouteFilter) ([]apiv1.Policy, bool, error) {
	var policies []apiv1.Policy
	var hasRedirect bool

	for _, f := range filters {
		switch f.Type {
		case gatewayv1.HTTPRouteFilterRequestHeaderModifier:
			if f.RequestHeaderModifier == nil {
				continue
			}
			mod := f.RequestHeaderModifier
			if len(mod.Set) > 0 {
				headers := make([]map[string]string, 0, len(mod.Set))
				for _, h := range mod.Set {
					headers = append(headers, map[string]string{
						"name":  string(h.Name),
						"value": h.Value,
					})
				}
				p, err := policyFromParams("set-headers", "v1", map[string]interface{}{
					"request": map[string]interface{}{"headers": headers},
				})
				if err != nil {
					return nil, false, err
				}
				policies = append(policies, p)
			}
			if len(mod.Add) > 0 {
				// Gateway-API "add" APPENDS to any existing header value, unlike "set"
				// which overwrites. The set-headers policy selects this via the top-level
				// "mode": "append" parameter (-> HeadersToAppend -> APPEND_IF_EXISTS_OR_ADD),
				// so a pre-existing client value is preserved. Headers go under the same
				// request.headers section as "set"; only the mode differs. Because mode is a
				// single per-policy setting, Add is emitted as its own set-headers instance,
				// separate from the Set instance above.
				headers := make([]map[string]string, 0, len(mod.Add))
				for _, h := range mod.Add {
					headers = append(headers, map[string]string{
						"name":  string(h.Name),
						"value": h.Value,
					})
				}
				p, err := policyFromParams("set-headers", "v1", map[string]interface{}{
					"mode":    "append",
					"request": map[string]interface{}{"headers": headers},
				})
				if err != nil {
					return nil, false, err
				}
				policies = append(policies, p)
			}
			if len(mod.Remove) > 0 {
				// The remove-headers policy schema expects request.headers to be a list of
				// objects ({name: <header>}), the same item shape as set-headers — not bare
				// strings. Sending strings fails controller config validation with
				// "Expected: object, given: string".
				headers := make([]map[string]string, 0, len(mod.Remove))
				for _, h := range mod.Remove {
					headers = append(headers, map[string]string{"name": string(h)})
				}
				p, err := policyFromParams("remove-headers", "v1", map[string]interface{}{
					"request": map[string]interface{}{"headers": headers},
				})
				if err != nil {
					return nil, false, err
				}
				policies = append(policies, p)
			}
		case gatewayv1.HTTPRouteFilterRequestRedirect:
			if f.RequestRedirect == nil {
				continue
			}
			// A RequestRedirect is realized as the redirect policy: it builds the Location
			// (preserving any component the filter leaves unset) and short-circuits the
			// request. hasRedirect tells the caller this rule terminates at the gateway and
			// legitimately has no backends.
			p, err := redirectPolicyFromFilter(f.RequestRedirect)
			if err != nil {
				return nil, false, err
			}
			policies = append(policies, p)
			hasRedirect = true
		default:
			// ExtensionRef and unsupported filters are handled elsewhere or ignored.
		}
	}
	return policies, hasRedirect, nil
}

// mapHTTPHeaderMatches converts Gateway-API header matchers into the operation's
// MatchHeaders field, which the gateway-controller renders as Envoy route header matchers
// for route selection and precedence.
func mapHTTPHeaderMatches(headers []gatewayv1.HTTPHeaderMatch) []apiv1.OperationHeaderMatch {
	if len(headers) == 0 {
		return nil
	}
	out := make([]apiv1.OperationHeaderMatch, 0, len(headers))
	for _, h := range headers {
		matchType := "Exact"
		if h.Type != nil && *h.Type == gatewayv1.HeaderMatchRegularExpression {
			matchType = "RegularExpression"
		}
		out = append(out, apiv1.OperationHeaderMatch{
			Name:  string(h.Name),
			Value: h.Value,
			Type:  matchType,
		})
	}
	return out
}

// findDynamicEndpoint returns the operation's dynamic-endpoint policy if present. Retained as
// a shared helper (used by redirect/direct-response handling and tests) after header-based
// routing was reverted to native matchHeaders.
func findDynamicEndpoint(op apiv1.Operation) (apiv1.Policy, bool) {
	for _, p := range op.Policies {
		if p.Name == dynamicEndpointPolicyName {
			return p, true
		}
	}
	return apiv1.Policy{}, false
}

func pathMatchTypeFromHTTPRoute(m *gatewayv1.HTTPPathMatch) apiv1.OperationPathMatchType {
	if m == nil || m.Type == nil {
		return apiv1.OperationPathMatchExact
	}
	switch *m.Type {
	case gatewayv1.PathMatchPathPrefix:
		return apiv1.OperationPathMatchPathPrefix
	default:
		return apiv1.OperationPathMatchExact
	}
}

func operationPathFromMatch(m gatewayv1.HTTPRouteMatch) string {
	pathVal := "/"
	if m.Path != nil && m.Path.Value != nil {
		p := strings.TrimSpace(*m.Path.Value)
		if p != "" {
			pathVal = p
			if !strings.HasPrefix(pathVal, "/") {
				pathVal = "/" + pathVal
			}
		}
	}
	if pathMatchTypeFromHTTPRoute(m.Path) == apiv1.OperationPathMatchPathPrefix {
		if pathVal != "/" && !strings.HasSuffix(pathVal, "/*") {
			if strings.HasSuffix(pathVal, "/") {
				pathVal = strings.TrimSuffix(pathVal, "/") + "/*"
			} else {
				pathVal = pathVal + "/*"
			}
		} else if pathVal == "/" {
			pathVal = "/*"
		}
	}
	return pathVal
}

func dynamicEndpointPolicy(defName string) (apiv1.Policy, error) {
	return policyFromParams("dynamic-endpoint", "v1", map[string]interface{}{
		"targetUpstream": defName,
	})
}

func upstreamDefinitionNameForService(svcNS, svcName string, port int32) string {
	return sanitizeUpstreamDefName(fmt.Sprintf("%s-%s-%d", svcNS, svcName, port))
}

func sanitizeUpstreamDefName(name string) string {
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.ReplaceAll(name, "/", "-")
	return name
}

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
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

// This file maps a Gateway-API RequestRedirect filter into the redirect policy. It is
// self-contained so it can be moved/removed easily. Header matching, redirects, and
// direct responses are all realized as policies; the operator no longer emits redirect
// as an operation field.

const (
	redirectPolicyName = "redirect"
	// redirectPolicyVersion must match the major version the redirect policy is built
	// at in gateway/build.yaml. The policy is currently under review at v0.9.0
	redirectPolicyVersion = "v0"

	// defaultRedirectStatus is the Gateway-API default when a RequestRedirect filter
	// omits statusCode.
	defaultRedirectStatus = 302

	pathModeFull   = "full"
	pathModePrefix = "prefix"
)

// redirectPolicyFromFilter builds the redirect policy attachment from a Gateway-API
// RequestRedirect filter. Only the components the filter specifies are emitted; anything
// left unset is preserved from the request at runtime by the policy.
func redirectPolicyFromFilter(f *gatewayv1.HTTPRequestRedirectFilter) (apiv1.Policy, error) {
	statusCode := defaultRedirectStatus
	if f.StatusCode != nil {
		statusCode = *f.StatusCode
	}
	params := map[string]interface{}{"statusCode": statusCode}
	if f.Scheme != nil && *f.Scheme != "" {
		params["scheme"] = *f.Scheme
	}
	if f.Hostname != nil {
		params["hostname"] = string(*f.Hostname)
	}
	if f.Port != nil {
		params["port"] = int(*f.Port)
	}
	if f.Path != nil {
		if p := redirectPathParams(f.Path); p != nil {
			params["path"] = p
		}
	}
	return policyFromParams(redirectPolicyName, redirectPolicyVersion, params)
}

// operationHasRedirectPolicy reports whether the operation already carries a redirect
// policy (a terminal action), used to keep such ops out of header-routing collapse.
func operationHasRedirectPolicy(op apiv1.Operation) bool {
	for _, p := range op.Policies {
		if p.Name == redirectPolicyName {
			return true
		}
	}
	return false
}

// redirectPathParams maps the filter's path modifier into the policy's {mode, value}
// shape. Returns nil when the modifier is incomplete.
func redirectPathParams(p *gatewayv1.HTTPPathModifier) map[string]interface{} {
	switch p.Type {
	case gatewayv1.FullPathHTTPPathModifier:
		if p.ReplaceFullPath != nil {
			return map[string]interface{}{"mode": pathModeFull, "value": *p.ReplaceFullPath}
		}
	case gatewayv1.PrefixMatchHTTPPathModifier:
		if p.ReplacePrefixMatch != nil {
			return map[string]interface{}{"mode": pathModePrefix, "value": *p.ReplacePrefixMatch}
		}
	}
	return nil
}

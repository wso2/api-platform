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
	"testing"

	"github.com/stretchr/testify/require"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// TestPoliciesFromHTTPRouteFilters_RemoveHeadersAreObjects guards HTTPRouteRequestHeaderModifier:
// the remove-headers policy schema requires request.headers items to be objects ({name: ...}),
// not bare strings. Sending strings makes the controller reject the config with HTTP 400
// ("Invalid type. Expected: object, given: string").
func TestPoliciesFromHTTPRouteFilters_RemoveHeadersAreObjects(t *testing.T) {
	filters := []gatewayv1.HTTPRouteFilter{{
		Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
		RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
			Remove: []string{"X-Remove-1", "X-Remove-2"},
		},
	}}

	policies, _, err := policiesFromHTTPRouteFilters(filters)
	require.NoError(t, err)
	require.Len(t, policies, 1)
	require.Equal(t, "remove-headers", policies[0].Name)
	require.NotNil(t, policies[0].Params)

	var params struct {
		Request struct {
			Headers []map[string]any `json:"headers"`
		} `json:"request"`
	}
	require.NoError(t, json.Unmarshal(policies[0].Params.Raw, &params))
	require.Len(t, params.Request.Headers, 2)
	// Each item must be an object carrying a "name" field (not a bare string).
	require.Equal(t, "X-Remove-1", params.Request.Headers[0]["name"])
	require.Equal(t, "X-Remove-2", params.Request.Headers[1]["name"])
}

// TestPoliciesFromHTTPRouteFilters_SetHeadersAreObjects confirms set/add headers keep the
// {name, value} object shape the schema expects.
func TestPoliciesFromHTTPRouteFilters_SetHeadersAreObjects(t *testing.T) {
	filters := []gatewayv1.HTTPRouteFilter{{
		Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
		RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
			Set: []gatewayv1.HTTPHeader{{Name: "X-Set", Value: "v"}},
		},
	}}
	policies, _, err := policiesFromHTTPRouteFilters(filters)
	require.NoError(t, err)
	require.Len(t, policies, 1)
	require.Equal(t, "set-headers", policies[0].Name)

	var params struct {
		Request struct {
			Headers []map[string]any `json:"headers"`
		} `json:"request"`
	}
	require.NoError(t, json.Unmarshal(policies[0].Params.Raw, &params))
	require.Len(t, params.Request.Headers, 1)
	require.Equal(t, "X-Set", params.Request.Headers[0]["name"])
	require.Equal(t, "v", params.Request.Headers[0]["value"])
}

// TestPoliciesFromHTTPRouteFilters_AddHeadersUseAppend guards Gateway-API "add" semantics:
// an Add filter must emit set-headers with top-level mode "append" (-> HeadersToAppend ->
// APPEND_IF_EXISTS_OR_ADD, preserving the existing value), with the headers under the normal
// request.headers section. "set" mode (overwrite) would fail HTTPRouteRequestHeaderModifier.
func TestPoliciesFromHTTPRouteFilters_AddHeadersUseAppend(t *testing.T) {
	filters := []gatewayv1.HTTPRouteFilter{{
		Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
		RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
			Add: []gatewayv1.HTTPHeader{{Name: "X-Header-Add", Value: "add-appends-values"}},
		},
	}}
	policies, _, err := policiesFromHTTPRouteFilters(filters)
	require.NoError(t, err)
	require.Len(t, policies, 1)
	require.Equal(t, "set-headers", policies[0].Name)

	var params struct {
		Mode    string `json:"mode"`
		Request struct {
			Headers []map[string]any `json:"headers"`
		} `json:"request"`
	}
	require.NoError(t, json.Unmarshal(policies[0].Params.Raw, &params))
	require.Equal(t, "append", params.Mode, "Add must select append mode, not the default set (overwrite)")
	require.Len(t, params.Request.Headers, 1)
	require.Equal(t, "X-Header-Add", params.Request.Headers[0]["name"])
	require.Equal(t, "add-appends-values", params.Request.Headers[0]["value"])
}

// TestPoliciesFromHTTPRouteFilters_Redirect verifies a RequestRedirect filter is realized
// as the redirect policy (not a direct response), and signals hasRedirect so the caller
// skips the no-backends 500 fallback. The detailed filter->params mapping is covered by
// TestRedirectPolicyFromFilter.
func TestPoliciesFromHTTPRouteFilters_Redirect(t *testing.T) {
	host := gatewayv1.PreciseHostname("example.org")
	status := 307
	filters := []gatewayv1.HTTPRouteFilter{{
		Type: gatewayv1.HTTPRouteFilterRequestRedirect,
		RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
			StatusCode: &status,
			Hostname:   &host,
		},
	}}

	policies, hasRedirect, err := policiesFromHTTPRouteFilters(filters)
	require.NoError(t, err)
	require.True(t, hasRedirect, "a RequestRedirect must signal hasRedirect")

	var redirectPolicies int
	for _, p := range policies {
		if p.Name == redirectPolicyName {
			redirectPolicies++
		}
	}
	require.Equal(t, 1, redirectPolicies, "exactly one redirect policy must be emitted")
}

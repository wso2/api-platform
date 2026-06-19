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

	policies, _, _, err := policiesFromHTTPRouteFilters(filters)
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
	policies, _, _, err := policiesFromHTTPRouteFilters(filters)
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
	policies, _, _, err := policiesFromHTTPRouteFilters(filters)
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

// TestPoliciesFromHTTPRouteFilters_RedirectStructured guards faithful Gateway-API RequestRedirect:
// the filter is mapped to a structured OperationRedirect where every component the filter omits stays
// nil (preserved from the original request), the omitted status defaults to 302, and a redirect never
// produces an OperationDirectResponse nor any Location header / localhost / forced-http fallback.
func TestPoliciesFromHTTPRouteFilters_RedirectStructured(t *testing.T) {
	strptr := func(s string) *string { return &s }
	redirectFilter := func(r *gatewayv1.HTTPRequestRedirectFilter) []gatewayv1.HTTPRouteFilter {
		return []gatewayv1.HTTPRouteFilter{{Type: gatewayv1.HTTPRouteFilterRequestRedirect, RequestRedirect: r}}
	}

	t.Run("status only preserves everything else", func(t *testing.T) {
		status := 301
		_, direct, redirect, err := policiesFromHTTPRouteFilters(redirectFilter(
			&gatewayv1.HTTPRequestRedirectFilter{StatusCode: &status}))
		require.NoError(t, err)
		require.Nil(t, direct, "a redirect must not produce a direct response")
		require.NotNil(t, redirect)
		require.Equal(t, 301, redirect.StatusCode)
		require.Nil(t, redirect.Scheme, "omitted scheme stays nil (preserve request scheme)")
		require.Nil(t, redirect.Hostname, "omitted hostname stays nil (no localhost)")
		require.Nil(t, redirect.Port)
		require.Nil(t, redirect.Path)
	})

	t.Run("omitted status defaults to 302", func(t *testing.T) {
		_, _, redirect, err := policiesFromHTTPRouteFilters(redirectFilter(&gatewayv1.HTTPRequestRedirectFilter{}))
		require.NoError(t, err)
		require.NotNil(t, redirect)
		require.Equal(t, 302, redirect.StatusCode)
	})

	t.Run("scheme only", func(t *testing.T) {
		_, _, redirect, err := policiesFromHTTPRouteFilters(redirectFilter(
			&gatewayv1.HTTPRequestRedirectFilter{Scheme: strptr("https")}))
		require.NoError(t, err)
		require.NotNil(t, redirect.Scheme)
		require.Equal(t, "https", *redirect.Scheme)
		require.Nil(t, redirect.Hostname)
		require.Nil(t, redirect.Port)
	})

	t.Run("hostname only", func(t *testing.T) {
		host := gatewayv1.PreciseHostname("example.org")
		_, _, redirect, err := policiesFromHTTPRouteFilters(redirectFilter(
			&gatewayv1.HTTPRequestRedirectFilter{Hostname: &host}))
		require.NoError(t, err)
		require.NotNil(t, redirect.Hostname)
		require.Equal(t, "example.org", *redirect.Hostname)
		require.Nil(t, redirect.Scheme)
		require.Nil(t, redirect.Port)
	})

	t.Run("port only", func(t *testing.T) {
		port := gatewayv1.PortNumber(8443)
		_, _, redirect, err := policiesFromHTTPRouteFilters(redirectFilter(
			&gatewayv1.HTTPRequestRedirectFilter{Port: &port}))
		require.NoError(t, err)
		require.NotNil(t, redirect.Port)
		require.Equal(t, 8443, *redirect.Port)
	})

	t.Run("path ReplaceFullPath", func(t *testing.T) {
		_, _, redirect, err := policiesFromHTTPRouteFilters(redirectFilter(
			&gatewayv1.HTTPRequestRedirectFilter{Path: &gatewayv1.HTTPPathModifier{
				Type: gatewayv1.FullPathHTTPPathModifier, ReplaceFullPath: strptr("/new"),
			}}))
		require.NoError(t, err)
		require.NotNil(t, redirect.Path)
		require.Equal(t, "ReplaceFullPath", redirect.Path.Type)
		require.NotNil(t, redirect.Path.ReplaceFullPath)
		require.Equal(t, "/new", *redirect.Path.ReplaceFullPath)
		require.Nil(t, redirect.Path.ReplacePrefixMatch)
	})

	t.Run("path ReplacePrefixMatch", func(t *testing.T) {
		_, _, redirect, err := policiesFromHTTPRouteFilters(redirectFilter(
			&gatewayv1.HTTPRequestRedirectFilter{Path: &gatewayv1.HTTPPathModifier{
				Type: gatewayv1.PrefixMatchHTTPPathModifier, ReplacePrefixMatch: strptr("/p"),
			}}))
		require.NoError(t, err)
		require.NotNil(t, redirect.Path)
		require.Equal(t, "ReplacePrefixMatch", redirect.Path.Type)
		require.NotNil(t, redirect.Path.ReplacePrefixMatch)
		require.Equal(t, "/p", *redirect.Path.ReplacePrefixMatch)
		require.Nil(t, redirect.Path.ReplaceFullPath)
	})

	t.Run("combined hostname and status, no direct response", func(t *testing.T) {
		status := 307
		host := gatewayv1.PreciseHostname("h.example.org")
		_, direct, redirect, err := policiesFromHTTPRouteFilters(redirectFilter(
			&gatewayv1.HTTPRequestRedirectFilter{StatusCode: &status, Hostname: &host}))
		require.NoError(t, err)
		require.Nil(t, direct)
		require.NotNil(t, redirect)
		require.Equal(t, 307, redirect.StatusCode)
		require.Equal(t, "h.example.org", *redirect.Hostname)
		require.Nil(t, redirect.Scheme)
		require.Nil(t, redirect.Port)
		require.Nil(t, redirect.Path)
	})
}

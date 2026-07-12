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
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

type redirectParams struct {
	StatusCode int     `json:"statusCode"`
	Scheme     *string `json:"scheme"`
	Hostname   *string `json:"hostname"`
	Port       *int    `json:"port"`
	Path       *struct {
		Mode  string `json:"mode"`
		Value string `json:"value"`
	} `json:"path"`
}

func paramsOf(t *testing.T, p apiv1.Policy) redirectParams {
	t.Helper()
	require.NotNil(t, p.Params)
	var out redirectParams
	require.NoError(t, json.Unmarshal(p.Params.Raw, &out))
	return out
}

func findRedirectParams(t *testing.T, op apiv1.Operation) redirectParams {
	t.Helper()
	for _, p := range op.Policies {
		if p.Name == redirectPolicyName {
			return paramsOf(t, p)
		}
	}
	t.Fatalf("no redirect policy on operation")
	return redirectParams{}
}

// pointer helpers for Gateway-API redirect filter fields (strptr is defined elsewhere in
// the package test suite and reused here for *string fields).
func statusPtr(i int) *int                        { return &i }
func hostPtr(s string) *gatewayv1.PreciseHostname { h := gatewayv1.PreciseHostname(s); return &h }
func portPtr(i int32) *gatewayv1.PortNumber       { p := gatewayv1.PortNumber(i); return &p }

// TestRedirectPolicyFromFilter checks the RequestRedirect-filter -> policy-params mapping,
// including the path modifier translation to {mode, value} and unset-omission.
func TestRedirectPolicyFromFilter(t *testing.T) {
	t.Run("all components with prefix path", func(t *testing.T) {
		f := &gatewayv1.HTTPRequestRedirectFilter{
			StatusCode: statusPtr(308),
			Scheme:     strPtr("https"),
			Hostname:   hostPtr("example.org"),
			Port:       portPtr(8443),
			Path: &gatewayv1.HTTPPathModifier{
				Type:               gatewayv1.PrefixMatchHTTPPathModifier,
				ReplacePrefixMatch: strPtr("/replacement-prefix"),
			},
		}
		p, err := redirectPolicyFromFilter(f)
		require.NoError(t, err)
		require.Equal(t, redirectPolicyName, p.Name)
		got := paramsOf(t, p)
		require.Equal(t, 308, got.StatusCode)
		require.Equal(t, "https", *got.Scheme)
		require.Equal(t, "example.org", *got.Hostname)
		require.Equal(t, 8443, *got.Port)
		require.NotNil(t, got.Path)
		require.Equal(t, "prefix", got.Path.Mode)
		require.Equal(t, "/replacement-prefix", got.Path.Value)
	})

	t.Run("ReplaceFullPath -> full", func(t *testing.T) {
		f := &gatewayv1.HTTPRequestRedirectFilter{
			Path: &gatewayv1.HTTPPathModifier{
				Type:            gatewayv1.FullPathHTTPPathModifier,
				ReplaceFullPath: strPtr("/replacement-full"),
			},
		}
		p, err := redirectPolicyFromFilter(f)
		require.NoError(t, err)
		got := paramsOf(t, p)
		require.Equal(t, 302, got.StatusCode, "status defaults to 302 when the filter omits it")
		require.Equal(t, "full", got.Path.Mode)
		require.Equal(t, "/replacement-full", got.Path.Value)
	})

	t.Run("host only preserves the rest (unset omitted)", func(t *testing.T) {
		f := &gatewayv1.HTTPRequestRedirectFilter{
			StatusCode: statusPtr(301),
			Hostname:   hostPtr("example.org"),
		}
		p, err := redirectPolicyFromFilter(f)
		require.NoError(t, err)
		got := paramsOf(t, p)
		require.Equal(t, 301, got.StatusCode)
		require.Equal(t, "example.org", *got.Hostname)
		require.Nil(t, got.Scheme, "unset scheme must be omitted")
		require.Nil(t, got.Port, "unset port must be omitted")
		require.Nil(t, got.Path, "unset path must be omitted")
	})
}

func redirectRouteScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))
	return scheme
}

// TestBuildAPIConfig_Redirect: a RequestRedirect filter compiles to a redirect policy on
// every operation (no backend routing), replacing the old OperationRedirect field.
func TestBuildAPIConfig_Redirect(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(redirectRouteScheme(t)).Build()

	exact := gatewayv1.PathMatchExact
	path := "/host-redirect"
	get := gatewayv1.HTTPMethodGet
	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "redirect-route", Namespace: "default"},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{{
				Matches: []gatewayv1.HTTPRouteMatch{{
					Path:   &gatewayv1.HTTPPathMatch{Type: &exact, Value: &path},
					Method: &get,
				}},
				Filters: []gatewayv1.HTTPRouteFilter{{
					Type: gatewayv1.HTTPRouteFilterRequestRedirect,
					RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
						Hostname:   hostPtr("example.org"),
						StatusCode: statusPtr(302),
					},
				}},
			}},
		},
	}

	spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
	require.NoError(t, err)
	require.NotEmpty(t, spec.Operations)

	for _, op := range spec.Operations {
		require.True(t, operationHasRedirectPolicy(op), "every op must carry the redirect policy")
		// No dynamic-endpoint routing on a redirect operation.
		_, hasDyn := findDynamicEndpoint(op)
		require.False(t, hasDyn, "redirect op must not have backend routing")
	}
	params := findRedirectParams(t, spec.Operations[0])
	require.Equal(t, 302, params.StatusCode)
	require.Equal(t, "example.org", *params.Hostname)
}

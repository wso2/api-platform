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
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

// hbrParams mirrors the header-based-routing policy parameter schema for assertions.
type hbrParams struct {
	Rules []struct {
		Destination string `json:"destination"`
		Matches     []struct {
			MatchHeaders []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
				Type  string `json:"type"`
			} `json:"matchHeaders"`
		} `json:"matches"`
	} `json:"rules"`
	NoMatchStatusCode *int `json:"noMatchStatusCode"`
}

func hdrMatch(name, value string) headerMatch {
	return headerMatch{Name: name, Value: value, Type: "Exact"}
}

// headerOp builds a staged operation with a dynamic-endpoint routing policy and the given
// header matchers (mirroring what the compile loop stages before collapsing).
func headerOp(method, path, dest string, hs ...headerMatch) stagedOperation {
	dep, _ := dynamicEndpointPolicy(dest)
	return stagedOperation{
		op: apiv1.Operation{
			Method:        apiv1.OperationMethod(method),
			Path:          path,
			PathMatchType: apiv1.OperationPathMatchExact,
			Policies:      []apiv1.Policy{dep},
		},
		headers: hs,
	}
}

func findHBRPolicy(t *testing.T, op apiv1.Operation) hbrParams {
	t.Helper()
	for _, p := range op.Policies {
		if p.Name == headerBasedRoutingPolicyName {
			require.NotNil(t, p.Params)
			var out hbrParams
			require.NoError(t, json.Unmarshal(p.Params.Raw, &out))
			return out
		}
	}
	t.Fatalf("no header-based-routing policy on operation")
	return hbrParams{}
}

// TestCollapseHeaderMatches_Conformance mirrors the HTTPRouteHeaderMatching test at the
// operation level: the compile path stages one operation per match (each with its header
// matchers and a dynamic-endpoint policy). Collapsing them yields a single operation
// carrying one header-based-routing policy with all rules.
func TestCollapseHeaderMatches_Conformance(t *testing.T) {
	staged := []stagedOperation{
		headerOp("GET", "/", "infra-backend-v1", hdrMatch("version", "one")),
		headerOp("GET", "/", "infra-backend-v2", hdrMatch("version", "two")),
		headerOp("GET", "/", "infra-backend-v1", hdrMatch("version", "two"), hdrMatch("color", "orange")),
		headerOp("GET", "/", "infra-backend-v1", hdrMatch("color", "blue")),
		headerOp("GET", "/", "infra-backend-v1", hdrMatch("color", "green")),
		headerOp("GET", "/", "infra-backend-v2", hdrMatch("color", "red")),
		headerOp("GET", "/", "infra-backend-v2", hdrMatch("color", "yellow")),
	}

	out := collapseHeaderMatchesToPolicy(staged, "", zap.NewNop())
	require.Len(t, out, 1, "seven header-differentiated ops collapse to one")

	params := findHBRPolicy(t, out[0])
	require.Len(t, params.Rules, 7, "one policy rule per source op, in order")
	require.NotNil(t, params.NoMatchStatusCode, "no header-less default => 404 on no match")
	require.Equal(t, 404, *params.NoMatchStatusCode)

	// Order preserved (earlier-rule-wins tie-break relies on it).
	require.Equal(t, "infra-backend-v1", params.Rules[0].Destination)
	require.Equal(t, "one", params.Rules[0].Matches[0].MatchHeaders[0].Value)
	// The 2-header AND rule is carried faithfully.
	require.Len(t, params.Rules[2].Matches[0].MatchHeaders, 2)
	require.Equal(t, "infra-backend-v1", params.Rules[2].Destination)

	// No dynamic-endpoint policy remains — the header-based-routing policy owns routing.
	for _, p := range out[0].Policies {
		require.NotEqual(t, dynamicEndpointPolicyName, p.Name)
	}
}

// TestCollapseHeaderMatches_WithDefault covers a group that also has a header-less
// catch-all: the default's dynamic-endpoint is retained (runs first) and noMatchStatusCode
// is left unset so non-matching requests pass through to it.
func TestCollapseHeaderMatches_WithDefault(t *testing.T) {
	defaultOp := headerOp("GET", "/", "infra-backend-v1") // no headers => default
	staged := []stagedOperation{
		headerOp("GET", "/", "infra-backend-v2", hdrMatch("version", "two")),
		defaultOp,
	}

	out := collapseHeaderMatchesToPolicy(staged, "", zap.NewNop())
	require.Len(t, out, 1)

	params := findHBRPolicy(t, out[0])
	require.Len(t, params.Rules, 1)
	require.Equal(t, "infra-backend-v2", params.Rules[0].Destination)
	require.Nil(t, params.NoMatchStatusCode, "default present => passthrough, no short-circuit")

	// The default dynamic-endpoint must be present and ordered before header-based-routing.
	var depIdx, hbrIdx = -1, -1
	for i, p := range out[0].Policies {
		switch p.Name {
		case dynamicEndpointPolicyName:
			depIdx = i
		case headerBasedRoutingPolicyName:
			hbrIdx = i
		}
	}
	require.NotEqual(t, -1, depIdx, "default dynamic-endpoint retained")
	require.NotEqual(t, -1, hbrIdx)
	require.Less(t, depIdx, hbrIdx, "default routing runs before header-based-routing")
}

// TestCollapseHeaderMatches_NonHeaderGroupUntouched verifies groups without header
// matching are passed through unchanged.
func TestCollapseHeaderMatches_NonHeaderGroupUntouched(t *testing.T) {
	plain := headerOp("GET", "/plain", "backend-a") // no headers, not a header group
	out := collapseHeaderMatchesToPolicy([]stagedOperation{plain}, "", zap.NewNop())
	require.Len(t, out, 1)
	require.Equal(t, plain.op.Path, out[0].Path)
	// dynamic-endpoint still there; no header-based-routing added.
	_, ok := findDynamicEndpoint(out[0])
	require.True(t, ok)
	for _, p := range out[0].Policies {
		require.NotEqual(t, headerBasedRoutingPolicyName, p.Name)
	}
}

// TestCollapseHeaderMatches_RedirectFallsBackToNative verifies a header-matched op that
// also redirects cannot be collapsed and is left as its original operation.
func TestCollapseHeaderMatches_RedirectFallsBackToNative(t *testing.T) {
	op := headerOp("GET", "/", "infra-backend-v1", hdrMatch("version", "one"))
	op.op.Redirect = &apiv1.OperationRedirect{StatusCode: 302}
	out := collapseHeaderMatchesToPolicy([]stagedOperation{op}, "", zap.NewNop())
	require.Len(t, out, 1)
	require.NotNil(t, out[0].Redirect, "kept as the original redirect operation")
	for _, p := range out[0].Policies {
		require.NotEqual(t, headerBasedRoutingPolicyName, p.Name)
	}
}

// TestBuildAPIConfigFromHTTPRoute_HeaderRoutingPolicy is the end-to-end path: a
// header-differentiated HTTPRoute compiles to a single operation carrying the
// header-based-routing policy, with an upstream definition per backend service.
func TestBuildAPIConfigFromHTTPRoute_HeaderRoutingPolicy(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))

	v1 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "infra-backend-v1", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}
	v2 := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "infra-backend-v2", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(v1, v2).Build()

	exact := gatewayv1.PathMatchExact
	root := "/"
	get := gatewayv1.HTTPMethodGet
	mkRule := func(headerName, headerValue, svc string) gatewayv1.HTTPRouteRule {
		return gatewayv1.HTTPRouteRule{
			Matches: []gatewayv1.HTTPRouteMatch{{
				Path:    &gatewayv1.HTTPPathMatch{Type: &exact, Value: &root},
				Method:  &get,
				Headers: []gatewayv1.HTTPHeaderMatch{{Name: gatewayv1.HTTPHeaderName(headerName), Value: headerValue}},
			}},
			BackendRefs: []gatewayv1.HTTPBackendRef{{
				BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Name: gatewayv1.ObjectName(svc),
						Port: ptrPort(8080),
					},
				},
			}},
		}
	}

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "header-matching",
			Namespace: "default",
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				mkRule("version", "one", "infra-backend-v1"),
				mkRule("version", "two", "infra-backend-v2"),
			},
		},
	}

	spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
	require.NoError(t, err)

	require.Len(t, spec.Operations, 1, "header rules collapse to a single operation")
	op := spec.Operations[0]

	params := findHBRPolicy(t, op)
	require.Len(t, params.Rules, 2)
	require.Equal(t, 404, *params.NoMatchStatusCode)

	// Both backend services became upstream definitions the policy can target.
	names := map[string]bool{}
	for _, d := range spec.UpstreamDefinitions {
		names[d.Name] = true
	}
	require.True(t, names[params.Rules[0].Destination], "destination has a matching upstream definition")
	require.True(t, names[params.Rules[1].Destination], "destination has a matching upstream definition")
}

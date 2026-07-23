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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

// TestBuildAPIConfigFromHTTPRoute_EmitsMatchForm verifies the operator compiles a Gateway-API
// HTTPRoute (with a PathPrefix match AND a header match) into the new `match` structure:
// method + path{value,type} + headers all live under Operation.Match, and the old top-level
// pathMatchType/matchHeaders fields are gone.
func TestBuildAPIConfigFromHTTPRoute_EmitsMatchForm(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}
	cl := fake.NewClientBuilder().WithScheme(redirectRouteScheme(t)).WithObjects(svc).Build()

	prefix := gatewayv1.PathMatchPathPrefix
	pathVal := "/foo"
	get := gatewayv1.HTTPMethodGet
	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "hdr-route", Namespace: "default"},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{{
				Matches: []gatewayv1.HTTPRouteMatch{{
					Path:    &gatewayv1.HTTPPathMatch{Type: &prefix, Value: &pathVal},
					Method:  &get,
					Headers: []gatewayv1.HTTPHeaderMatch{{Name: gatewayv1.HTTPHeaderName("x-variant"), Value: "alpha"}},
				}},
				BackendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{Name: "backend", Port: portPtr(8080)},
				}}},
			}},
		},
	}

	spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
	require.NoError(t, err)
	require.NotEmpty(t, spec.Operations)

	op := spec.Operations[0]
	require.NotNil(t, op.Match, "operator must emit the match form (not top-level fields)")
	assert.Empty(t, op.Method, "top-level method must be empty; identity lives under match")
	assert.Empty(t, op.Path, "top-level path must be empty; identity lives under match")

	assert.Equal(t, apiv1.OperationMethodGET, op.Match.Method)
	assert.Equal(t, "/foo/*", op.Match.Path.Value, "PathPrefix path is emitted with the /* wildcard suffix")
	assert.Equal(t, apiv1.OperationPathMatchPathPrefix, op.Match.Path.Type)
	require.Len(t, op.Match.Headers, 1, "header match must be emitted under match.headers")
	assert.Equal(t, "x-variant", op.Match.Headers[0].Name)
	assert.Equal(t, "alpha", op.Match.Headers[0].Value)
	assert.Equal(t, "Exact", op.Match.Headers[0].Type)
}

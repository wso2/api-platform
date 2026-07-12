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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
)

type respondParams struct {
	StatusCode int `json:"statusCode"`
}

func respondParamsOf(t *testing.T, op apiv1.Operation) respondParams {
	t.Helper()
	for _, p := range op.Policies {
		if p.Name == respondPolicyName {
			require.NotNil(t, p.Params)
			var out respondParams
			require.NoError(t, json.Unmarshal(p.Params.Raw, &out))
			return out
		}
	}
	t.Fatalf("no respond policy on operation")
	return respondParams{}
}

// TestRespondPolicyFromStatus checks the status -> respond policy-params mapping.
func TestRespondPolicyFromStatus(t *testing.T) {
	p, err := respondPolicyFromStatus(500)
	require.NoError(t, err)
	require.Equal(t, respondPolicyName, p.Name)
	require.Equal(t, respondPolicyVersion, p.Version)
	require.NotNil(t, p.Params)
	var got respondParams
	require.NoError(t, json.Unmarshal(p.Params.Raw, &got))
	require.Equal(t, 500, got.StatusCode)
}

// TestBuildAPIConfig_NoBackendRespondPolicy: a rule with no backendRefs terminates at the
// gateway via a respond policy (statusCode 500) with no backend routing.
func TestBuildAPIConfig_NoBackendRespondPolicy(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(redirectRouteScheme(t)).Build()

	exact := gatewayv1.PathMatchExact
	path := "/no-backend"
	get := gatewayv1.HTTPMethodGet
	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "dr-route", Namespace: "default"},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{{
				Matches: []gatewayv1.HTTPRouteMatch{{
					Path:   &gatewayv1.HTTPPathMatch{Type: &exact, Value: &path},
					Method: &get,
				}},
			}},
		},
	}

	spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
	require.NoError(t, err)
	require.NotEmpty(t, spec.Operations)

	for _, op := range spec.Operations {
		require.True(t, operationHasRespondPolicy(op), "every op must carry the respond policy")
		_, hasDyn := findDynamicEndpoint(op)
		require.False(t, hasDyn, "respond op must not have backend routing")
	}
	require.Equal(t, 500, respondParamsOf(t, spec.Operations[0]).StatusCode)
}

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

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
)

// TestConfigHashStableAcrossBookkeepingAnnotations guards against the self-referential
// redeploy hot-loop: the operator writes its deploy hash back onto the HTTPRoute as the
// AnnHTTPRouteLastDeployedConfigHash annotation, and the payload metadata is built from the
// route's annotations. If those bookkeeping annotations fed into the hashed payload, the
// hash would change every reconcile, the dedup guard would never trip, and the reconciler
// would redeploy ~50x/s (observed as HTTPRouteWeight storming the gateway-controller and
// starving every other conformance test into 404s).
//
// The hash must be identical before and after the bookkeeping annotations are present.
func TestConfigHashStableAcrossBookkeepingAnnotations(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(redirectRouteScheme(t)).WithObjects(
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "infra-backend-v1", Namespace: "default"},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "infra-backend-v2", Namespace: "default"},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}}},
	).Build()

	pathPrefix := gatewayv1.PathMatchPathPrefix
	slash := "/"
	w1, w2 := int32(1), int32(1)
	mkRoute := func(ann map[string]string) *gatewayv1.HTTPRoute {
		return &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "weighted-backends", Namespace: "default", Annotations: ann},
			Spec: gatewayv1.HTTPRouteSpec{
				Rules: []gatewayv1.HTTPRouteRule{{
					Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Type: &pathPrefix, Value: &slash}}},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "infra-backend-v1", Port: portPtr(8080)}, Weight: &w1}},
						{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: "infra-backend-v2", Port: portPtr(8080)}, Weight: &w2}},
					},
				}},
			},
		}
	}

	handle := "default-weighted-backends"
	hashFor := func(route *gatewayv1.HTTPRoute) string {
		spec, err := buildAPIConfigFromHTTPRouteForTest(context.Background(), cl, route, "cluster.local")
		require.NoError(t, err)
		y, err := gatewayclient.BuildRestAPIYAML(apiv1.GroupVersion.String(), "RestApi",
			payloadMetadataForHTTPRoute(route, handle), *spec)
		require.NoError(t, err)
		return hashRestAPIPayload(y)
	}

	// Reconcile 1: only a user annotation present.
	base := map[string]string{AnnHTTPRouteContext: "/weighted/$version"}
	h1 := hashFor(mkRoute(base))

	// Reconcile 2: the operator has written its bookkeeping annotations back onto the route.
	withBookkeeping := map[string]string{
		AnnHTTPRouteContext:                   "/weighted/$version",
		AnnHTTPRouteLastDeployedConfigHash:    h1,
		AnnHTTPRouteLastDeployedParentGateway: "gateway-conformance-infra/same-namespace",
	}
	h2 := hashFor(mkRoute(withBookkeeping))

	require.Equal(t, h1, h2,
		"config hash must not change when the operator's own bookkeeping annotations are present; "+
			"otherwise the dedup guard never trips and the reconciler redeploys in a hot loop")

	// And a user annotation change MUST still change the hash (dedup stays effective).
	changed := map[string]string{AnnHTTPRouteContext: "/weighted-changed/$version", AnnHTTPRouteLastDeployedConfigHash: h1}
	require.NotEqual(t, h1, hashFor(mkRoute(changed)), "a real config change must still flip the hash")
}

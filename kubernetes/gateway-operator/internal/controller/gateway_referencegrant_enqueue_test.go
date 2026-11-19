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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
)

// The GatewaySecretInvalidReferenceGrant conformance case: a Gateway first resolved while another
// grant allowed it must be re-evaluated once that grant is deleted.
func TestEnqueueK8sGatewaysForReferenceGrant(t *testing.T) {
	https := func(ns, name, class, secretNS string, group ...string) *gatewayv1.Gateway {
		ref := gatewayv1.SecretObjectReference{Name: "certificate"}
		if secretNS != "" {
			ref.Namespace = goldenStr[gatewayv1.Namespace](secretNS)
		}
		if len(group) > 0 {
			ref.Group = goldenStr[gatewayv1.Group](group[0])
		}
		return &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
			Spec: gatewayv1.GatewaySpec{
				GatewayClassName: gatewayv1.ObjectName(class),
				Listeners: []gatewayv1.Listener{{
					Name: "https", Port: 443, Protocol: gatewayv1.HTTPSProtocolType,
					TLS: &gatewayv1.ListenerTLSConfig{CertificateRefs: []gatewayv1.SecretObjectReference{ref}},
				}},
			},
		}
	}
	cl := fake.NewClientBuilder().WithScheme(redirectRouteScheme(t)).WithObjects(
		https("infra", "cross-ns", "wso2", "web-backend"),
		// Listener validation accepts "core" for the core group, so the watch must too.
		https("infra", "cross-ns-core-group", "wso2", "web-backend", "core"),
		https("infra", "cross-ns-other-group", "wso2", "web-backend", "example.com"),
		https("infra", "same-ns", "wso2", ""),
		https("infra", "other-ns", "wso2", "elsewhere"),
		https("infra", "unmanaged", "istio", "web-backend"),
		https("web-backend", "local", "wso2", "web-backend"),
	).Build()
	cfg := &config.OperatorConfig{}
	cfg.GatewayAPI.GatewayClassNames = []string{"wso2"}
	r := &K8sGatewayReconciler{Client: cl, Config: cfg}

	grant := &gatewayv1beta1.ReferenceGrant{ObjectMeta: metav1.ObjectMeta{Namespace: "web-backend", Name: "g"}}
	got := r.enqueueK8sGatewaysForReferenceGrant(context.Background(), grant)
	require.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: types.NamespacedName{Namespace: "infra", Name: "cross-ns"}},
		{NamespacedName: types.NamespacedName{Namespace: "infra", Name: "cross-ns-core-group"}},
	}, got)
}

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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
)

// TestGatewayHelmExpectedResourcesPresent verifies the Helm self-heal check. In particular it guards
// the regression where a gateway-runtime Service deleted out-of-band left resourcesOK (and therefore
// skipHelm) true, so Helm never re-ran to recreate it and reconcile looped forever returning
// pending/error. The runtime Service must be folded into the expected-resource check alongside the
// Deployments and the controller Service.
func TestGatewayHelmExpectedResourcesPresent(t *testing.T) {
	const (
		gatewayName = "gw1"
		namespace   = "infra"
	)
	release := helm.GetReleaseName(gatewayName)

	deployment := func() client.Object {
		return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name:      release + "-controller",
			Namespace: namespace,
			Labels:    map[string]string{"app.kubernetes.io/instance": release},
		}}
	}
	controllerSvc := func() client.Object {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      release + "-controller",
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/instance":  release,
					"app.kubernetes.io/component": "controller",
				},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "rest", Port: 9090}}},
		}
	}
	runtimeSvc := func() client.Object {
		return &corev1.Service{ObjectMeta: metav1.ObjectMeta{
			Name:      release + "-gateway-runtime",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/instance":  release,
				"app.kubernetes.io/component": "gateway-runtime",
			},
		}}
	}

	cases := []struct {
		name           string
		objects        []client.Object
		wantOK         bool
		detailContains string
	}{
		{
			name:    "all resources present",
			objects: []client.Object{deployment(), controllerSvc(), runtimeSvc()},
			wantOK:  true,
		},
		{
			name:           "runtime Service missing triggers re-install",
			objects:        []client.Object{deployment(), controllerSvc()},
			wantOK:         false,
			detailContains: "gateway runtime Service not found",
		},
		{
			name:           "controller Service missing triggers re-install",
			objects:        []client.Object{deployment(), runtimeSvc()},
			wantOK:         false,
			detailContains: "gateway controller Service not found",
		},
		{
			name:           "no deployments triggers re-install",
			objects:        []client.Object{controllerSvc(), runtimeSvc()},
			wantOK:         false,
			detailContains: "no Deployments labeled for this Helm release",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, appsv1.AddToScheme(scheme))
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.objects...).Build()

			ok, detail, err := gatewayHelmExpectedResourcesPresent(context.Background(), cl, gatewayName, namespace)
			require.NoError(t, err)
			require.Equal(t, tc.wantOK, ok)
			if tc.detailContains != "" {
				require.True(t, strings.Contains(detail, tc.detailContains),
					"detail %q should contain %q", detail, tc.detailContains)
			}
		})
	}
}

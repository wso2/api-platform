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
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

const waitRestAPIYAML = `
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: wait-api
  namespace: wait-ns
spec:
  displayName: wait-api
  version: v1.0
  context: /wait
  upstream:
    main:
      url: https://backend.example.com
  operations:
    - method: GET
      path: /info
`

func newWaitRestAPI(t *testing.T) *apiv1.RestApi {
	t.Helper()
	api := &apiv1.RestApi{}
	require.NoError(t, yaml.Unmarshal([]byte(waitRestAPIYAML), api))
	api.Generation = 1 // as the API server sets on create; the fake client leaves it at 0
	return api
}

// registerUnverifiedGateway registers a gateway whose controller Service cannot be verified:
// the entry carries no verified identity and the Service does not exist.
func registerUnverifiedGateway(t *testing.T, namespace string) {
	t.Helper()
	reg := registry.GetGatewayRegistry()
	reg.Register(&registry.GatewayInfo{
		Name: "gw-wait", Namespace: namespace,
		APISelector: &apiv1.APISelector{Scope: apiv1.NamespacedScope, Namespaces: []string{namespace}},
		ServiceName: "gwc-wait", ServicePort: 9090,
	})
	t.Cleanup(func() { reg.Unregister(namespace, "gw-wait") })
}

func newWaitRestAPIReconciler(t *testing.T, objs ...client.Object) (*RestApiReconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&apiv1.RestApi{}).WithObjects(objs...).Build()
	cfg := &config.OperatorConfig{}
	cfg.Reconciliation.MaxRetryAttempts = 2
	return NewRestApiReconciler(cl, scheme, cfg, slog.New(slog.NewTextHandler(io.Discard, nil))), cl
}

// TestRestApiWaitsForUnverifiedControllerService checks that a refused controller Service
// is a wait, not a failed attempt: with a retry budget of two, many reconciles still end in
// GatewayNotReady with a fixed requeue, never the terminal DeploymentFailed state.
func TestRestApiWaitsForUnverifiedControllerService(t *testing.T) {
	api := newWaitRestAPI(t)
	r, cl := newWaitRestAPIReconciler(t, api)
	registerUnverifiedGateway(t, api.Namespace)

	ctx := context.Background()
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(api)}
	for i := 0; i < 6; i++ {
		res, err := r.Reconcile(ctx, req)
		require.NoError(t, err)
		if i == 0 {
			continue // adds the finalizer
		}
		require.Equal(t, 15*time.Second, res.RequeueAfter, "reconcile %d", i)
	}

	got := &apiv1.RestApi{}
	require.NoError(t, cl.Get(ctx, req.NamespacedName, got))
	cond := meta.FindStatusCondition(got.Status.Conditions, apiv1.APIConditionProgrammed)
	require.NotNil(t, cond)
	require.Equal(t, apiv1.APIProgrammedReasonGatewayNotReady, cond.Reason)
	require.Contains(t, cond.Message, "controller Service")

	entry, ok := r.apiTracker.Get(req.String())
	require.True(t, ok)
	require.Zero(t, entry.RetryCount, "waiting for the Service must not use the retry budget")
}

// TestRestApiDeletionKeepsFinalizerWhenCleanupFails checks that a RestApi whose removal from a
// matched gateway fails keeps its finalizer, so deletion is retried instead of the CR
// disappearing while the API is still deployed.
func TestRestApiDeletionKeepsFinalizerWhenCleanupFails(t *testing.T) {
	api := newWaitRestAPI(t)
	api.Finalizers = []string{apiFinalizerName}
	now := metav1.Now()
	api.DeletionTimestamp = &now
	r, cl := newWaitRestAPIReconciler(t, api)
	registerUnverifiedGateway(t, api.Namespace)

	ctx := context.Background()
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(api)}
	_, err := r.Reconcile(ctx, req)
	require.ErrorIs(t, err, errControllerServiceUnverified)

	got := &apiv1.RestApi{}
	require.NoError(t, cl.Get(ctx, req.NamespacedName, got))
	require.Contains(t, got.Finalizers, apiFinalizerName)
}

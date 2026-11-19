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
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

// TestRestAPIDeletionKeepsFinalizerWhenGatewayCleanupFails covers a RestApi deleted while the
// API cannot be removed from a matching gateway (here: its controller credential Secret is not
// readable). The finalizer and tracker entry must stay so the deletion is retried, instead of
// forgetting an API that is still deployed on the gateway.
func TestRestAPIDeletionKeepsFinalizerWhenGatewayCleanupFails(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))

	const apiNamespace = "cleanup-team"
	reg := registry.GetGatewayRegistry()
	gw := &registry.GatewayInfo{
		Name: "cleanup-gw", Namespace: "cleanup-gw-ns", ServiceName: "cleanup-gw-controller", ServicePort: 9090,
		APISelector: &apiv1.APISelector{Scope: apiv1.NamespacedScope, Namespaces: []string{apiNamespace}},
	}
	reg.Register(gw)
	t.Cleanup(func() { reg.Unregister(gw.Namespace, gw.Name) })

	now := metav1.Now()
	api := &apiv1.RestApi{ObjectMeta: metav1.ObjectMeta{
		Name: "orders", Namespace: apiNamespace,
		Finalizers:        []string{apiFinalizerName},
		DeletionTimestamp: &now,
	}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(api).Build()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	r := NewRestApiReconciler(c, scheme, &config.OperatorConfig{}, logger)

	key := types.NamespacedName{Namespace: apiNamespace, Name: api.Name}
	r.apiTracker.Set(key.String(), &APITrackingEntry{Generation: 1})

	current := &apiv1.RestApi{}
	require.NoError(t, c.Get(ctx, key, current))
	_, err := r.reconcileAPIDeletion(ctx, current)
	require.Error(t, err, "a failed gateway cleanup must be returned so the deletion is retried")
	require.Contains(t, err.Error(), "cleanup-gw-ns/cleanup-gw")

	after := &apiv1.RestApi{}
	require.NoError(t, c.Get(ctx, key, after))
	require.True(t, controllerutil.ContainsFinalizer(after, apiFinalizerName), "finalizer must stay while the API may still be deployed")
	_, tracked := r.apiTracker.Get(key.String())
	require.True(t, tracked, "tracker entry must stay while the API may still be deployed")
}

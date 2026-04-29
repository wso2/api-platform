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
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
)

func testZapLogger(t *testing.T) *zap.Logger {
	t.Helper()
	return zaptest.NewLogger(t)
}

func testOperatorConfigForGatewayClass(t *testing.T, classNames ...string) *config.OperatorConfig {
	t.Helper()
	cfg := &config.OperatorConfig{
		GatewayAPI: config.GatewayAPIConfig{GatewayClassNames: classNames},
		Reconciliation: config.ReconciliationConfig{
			MaxConcurrentReconciles: 1,
			MaxRetryAttempts:        1,
			InitialBackoff:          time.Second,
			MaxBackoffDuration:      time.Minute,
		},
		Logging: config.LoggingConfig{Level: "info", Format: "text"},
	}
	require.NoError(t, cfg.Validate())
	return cfg
}

func TestK8sGatewayClassReconciler_AcceptedWhenManaged(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	gc := &gatewayv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{Name: "wso2-api-platform", Generation: 1},
		Spec: gatewayv1.GatewayClassSpec{
			ControllerName: PlatformGatewayControllerName,
		},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gc).
		WithStatusSubresource(gc).
		Build()
	r := NewK8sGatewayClassReconciler(cl, scheme, testOperatorConfigForGatewayClass(t, "wso2-api-platform"), testZapLogger(t))
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: gc.Name}})
	require.NoError(t, err)

	updated := &gatewayv1.GatewayClass{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: gc.Name}, updated))
	cond := meta.FindStatusCondition(updated.Status.Conditions, string(gatewayv1.GatewayClassConditionStatusAccepted))
	require.NotNil(t, cond)
	require.Equal(t, metav1.ConditionTrue, cond.Status)
	require.Equal(t, string(gatewayv1.GatewayClassReasonAccepted), cond.Reason)
	require.Equal(t, int64(1), cond.ObservedGeneration)
}

func TestK8sGatewayClassReconciler_NotAcceptedWhenNotInAllowlist(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	gc := &gatewayv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{Name: "other-class", Generation: 2},
		Spec: gatewayv1.GatewayClassSpec{
			ControllerName: PlatformGatewayControllerName,
		},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gc).
		WithStatusSubresource(gc).
		Build()
	r := NewK8sGatewayClassReconciler(cl, scheme, testOperatorConfigForGatewayClass(t, "wso2-api-platform"), testZapLogger(t))
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: gc.Name}})
	require.NoError(t, err)

	updated := &gatewayv1.GatewayClass{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: gc.Name}, updated))
	cond := meta.FindStatusCondition(updated.Status.Conditions, string(gatewayv1.GatewayClassConditionStatusAccepted))
	require.NotNil(t, cond)
	require.Equal(t, metav1.ConditionFalse, cond.Status)
	require.Equal(t, string(gatewayv1.GatewayClassReasonUnsupported), cond.Reason)
}

func TestK8sGatewayClassReconciler_NoOpDifferentController(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(gatewayv1.AddToScheme(scheme))
	gc := &gatewayv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{Name: "other-class", Generation: 1},
		Spec: gatewayv1.GatewayClassSpec{
			ControllerName: "example.com/not-us",
		},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gc).
		WithStatusSubresource(gc).
		Build()
	r := NewK8sGatewayClassReconciler(cl, scheme, testOperatorConfigForGatewayClass(t, "other-class"), testZapLogger(t))
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: gc.Name}})
	require.NoError(t, err)

	updated := &gatewayv1.GatewayClass{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: gc.Name}, updated))
	require.Empty(t, updated.Status.Conditions)
}

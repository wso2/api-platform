package controller

import (
	"context"
	"testing"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
)

func TestEnqueueGatewaysForConfigMap_UsesIndexedLookup(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := apiv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add APIGateway scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}

	indexedClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&apiv1.APIGateway{}, apigatewayConfigRefIndex, func(rawObj client.Object) []string {
			gateway, ok := rawObj.(*apiv1.APIGateway)
			if !ok || gateway.Spec.ConfigRef == nil || gateway.Spec.ConfigRef.Name == "" {
				return nil
			}
			return []string{gateway.Spec.ConfigRef.Name}
		}).
		WithObjects(
			&apiv1.APIGateway{
				ObjectMeta: metav1.ObjectMeta{Name: "gw-a", Namespace: "ns-a"},
				Spec: apiv1.GatewaySpec{
					ConfigRef: &corev1.LocalObjectReference{Name: "shared-values"},
				},
			},
			&apiv1.APIGateway{
				ObjectMeta: metav1.ObjectMeta{Name: "gw-b", Namespace: "ns-a"},
				Spec: apiv1.GatewaySpec{
					ConfigRef: &corev1.LocalObjectReference{Name: "other-values"},
				},
			},
			&apiv1.APIGateway{
				ObjectMeta: metav1.ObjectMeta{Name: "gw-c", Namespace: "ns-b"},
				Spec: apiv1.GatewaySpec{
					ConfigRef: &corev1.LocalObjectReference{Name: "shared-values"},
				},
			},
		).
		Build()

	reconciler := &GatewayReconciler{
		Client: indexedClient,
		Config: &config.OperatorConfig{},
		Logger: zap.NewNop(),
	}

	requests := reconciler.enqueueGatewaysForConfigMap(context.Background(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-values", Namespace: "ns-a"},
	})

	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}

	want := reconcile.Request{NamespacedName: client.ObjectKey{Namespace: "ns-a", Name: "gw-a"}}
	if requests[0] != want {
		t.Fatalf("unexpected request: got %#v want %#v", requests[0], want)
	}
}

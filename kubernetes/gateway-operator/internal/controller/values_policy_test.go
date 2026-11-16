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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/auth"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/valuespolicy"
)

const hostPathValues = `gateway:
  gatewayRuntime:
    deployment:
      extraVolumes:
        - name: root
          hostPath:
            path: /
`

func valuesPolicyTestConfig() *config.OperatorConfig {
	return &config.OperatorConfig{
		Gateway: config.GatewayConfig{
			ValuesPolicy: config.ValuesPolicyConfig{
				Enabled:                true,
				AllowedImageRegistries: valuespolicy.DefaultAllowedImageRegistries,
			},
		},
		Reconciliation: config.ReconciliationConfig{
			MaxConcurrentReconciles: 1,
			MaxRetryAttempts:        3,
			InitialBackoff:          time.Second,
			MaxBackoffDuration:      time.Minute,
		},
	}
}

func drainEvents(rec *events.FakeRecorder) []string {
	var out []string
	for {
		select {
		case e := <-rec.Events:
			out = append(out, e)
		default:
			return out
		}
	}
}

// A ConfigMap the policy refuses must not reach Helm: the reconcile settles on
// Programmed=False/Invalid for this generation and config, records a Warning
// Event naming the key, and does not retry until the values change.
func TestAPIGatewayValuesPolicyViolation(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))

	const (
		name      = "gw1"
		namespace = "default"
	)
	key := types.NamespacedName{Namespace: namespace, Name: name}

	gw := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: 1,
			Finalizers: []string{apigatewayFinalizerName},
		},
		Spec: apiv1.GatewaySpec{
			ConfigRef: &corev1.LocalObjectReference{Name: name + "-config"},
		},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-config", Namespace: namespace},
		Data:       map[string]string{"values.yaml": hostPathValues},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&apiv1.APIGateway{}).
		WithObjects(gw, cm).
		Build()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	r := NewGatewayReconciler(c, scheme, valuesPolicyTestConfig(), logger)
	rec := events.NewFakeRecorder(10)
	r.Recorder = rec

	// First reconcile: refused before Helm. Had Helm been reached it would have
	// failed without a cluster and left a Retrying condition and a requeue.
	result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	require.NoError(t, err)
	require.False(t, result.Requeue)
	require.Zero(t, result.RequeueAfter, "a policy refusal must not be retried")

	updated := &apiv1.APIGateway{}
	require.NoError(t, c.Get(ctx, key, updated))

	const wantKey = "gateway.gatewayRuntime.deployment.extraVolumes[0].hostPath"
	programmed := meta.FindStatusCondition(updated.Status.Conditions, apiv1.GatewayConditionProgrammed)
	require.NotNil(t, programmed)
	require.Equal(t, metav1.ConditionFalse, programmed.Status)
	require.Equal(t, apiv1.GatewayProgrammedReasonInvalid, programmed.Reason)
	require.Equal(t, int64(1), programmed.ObservedGeneration)
	require.Contains(t, programmed.Message, wantKey)

	accepted := meta.FindStatusCondition(updated.Status.Conditions, apiv1.GatewayConditionAccepted)
	require.NotNil(t, accepted)
	require.Equal(t, metav1.ConditionFalse, accepted.Status)
	require.Equal(t, apiv1.GatewayAcceptedReasonInvalidConfiguration, accepted.Reason)

	require.Equal(t, auth.CalculateConfigHash(hostPathValues), updated.Status.ConfigHash,
		"the refused config hash is recorded so only a change to it triggers a new attempt")

	evts := drainEvents(rec)
	require.Len(t, evts, 1)
	require.True(t, strings.HasPrefix(evts[0], corev1.EventTypeWarning+" "+EventReasonValuesPolicyViolation+" "), evts[0])
	require.Contains(t, evts[0], wantKey)

	// Second reconcile with the same values: nothing to do.
	result, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	require.NoError(t, err)
	require.False(t, result.Requeue)
	require.Zero(t, result.RequeueAfter)
	require.Empty(t, drainEvents(rec))

	// Fixing the ConfigMap triggers a new deployment attempt.
	latestCM := &corev1.ConfigMap{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(cm), latestCM))
	latestCM.Data["values.yaml"] = "gateway:\n  gatewayRuntime:\n    deployment:\n      replicaCount: 2\n"
	require.NoError(t, c.Update(ctx, latestCM))

	result, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	require.NoError(t, err)
	require.True(t, result.Requeue, "a changed ConfigMap must schedule a redeployment")
	require.NoError(t, c.Get(ctx, key, updated))
	programmed = meta.FindStatusCondition(updated.Status.Conditions, apiv1.GatewayConditionProgrammed)
	require.NotNil(t, programmed)
	require.Equal(t, "ConfigChanged", programmed.Reason)
}

// A disabled policy lets the same values through to the deployment step.
func TestAPIGatewayValuesPolicyDisabled(t *testing.T) {
	cfg := valuesPolicyTestConfig()
	cfg.Gateway.ValuesPolicy.Enabled = false
	require.NoError(t, checkGatewayValuesPolicy(cfg, hostPathValues))
	require.NoError(t, checkGatewayValuesPolicy(nil, hostPathValues))

	cfg.Gateway.ValuesPolicy.Enabled = true
	require.Error(t, checkGatewayValuesPolicy(cfg, hostPathValues))
}

// The Kubernetes Gateway API path applies the same policy to the values
// ConfigMap named by the helm-values-configmap annotation.
func TestK8sGatewayValuesPolicyViolation(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	require.NoError(t, gatewayv1.Install(scheme))

	const (
		name      = "gw1"
		namespace = "default"
	)

	gw := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Generation:  1,
			Annotations: map[string]string{AnnK8sGatewayHelmValuesConfigMap: name + "-values"},
		},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: "wso2-api-platform",
			Listeners: []gatewayv1.Listener{{
				Name:     "http",
				Port:     8080,
				Protocol: gatewayv1.HTTPProtocolType,
			}},
		},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-values", Namespace: namespace},
		Data:       map[string]string{"values.yaml": hostPathValues},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&gatewayv1.Gateway{}).
		WithObjects(gw, cm).
		Build()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	r := NewK8sGatewayReconciler(c, scheme, valuesPolicyTestConfig(), logger)
	rec := events.NewFakeRecorder(10)
	r.Recorder = rec

	require.NoError(t, r.syncGateway(ctx, gw, logger))

	updated := &gatewayv1.Gateway{}
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(gw), updated))
	programmed := meta.FindStatusCondition(updated.Status.Conditions, string(gatewayv1.GatewayConditionProgrammed))
	require.NotNil(t, programmed)
	require.Equal(t, metav1.ConditionFalse, programmed.Status)
	require.Equal(t, string(gatewayv1.GatewayReasonInvalid), programmed.Reason)
	require.Contains(t, programmed.Message, "extraVolumes[0].hostPath")
	_, hashed := updated.Annotations[AnnK8sGatewayHelmValuesHash]
	require.False(t, hashed, "no Helm values hash may be recorded for refused values")

	evts := drainEvents(rec)
	require.Len(t, evts, 1)
	require.Contains(t, evts[0], EventReasonValuesPolicyViolation)
}

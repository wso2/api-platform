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
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/auth"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
)

// buildPermanentlyFailedGateway constructs an APIGateway that already exhausted its
// retries: Programmed=False/DeploymentFailed with ObservedGeneration pinned to the
// current generation, mirroring exactly what handleGatewayDeploymentError writes.
func buildPermanentlyFailedGateway(name, namespace, oldConfigHash string) *apiv1.APIGateway {
	return &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: 1,
			Finalizers: []string{apigatewayFinalizerName},
		},
		Spec: apiv1.GatewaySpec{
			ConfigRef: &corev1.LocalObjectReference{Name: name + "-config"},
		},
		Status: apiv1.GatewayStatus{
			ConfigHash: oldConfigHash,
			Conditions: []metav1.Condition{
				{
					Type:               apiv1.GatewayConditionProgrammed,
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 1,
					Reason:             apiv1.GatewayProgrammedReasonDeploymentFailed,
					Message:            "Max retries (10) exceeded. Last error: context deadline exceeded",
					LastTransitionTime: metav1.Now(),
				},
			},
		},
	}
}

// Once an APIGateway exhausts MaxRetryAttempts, handleGatewayDeploymentError sets Programmed=False/DeploymentFailed
// AND ObservedGeneration == the CR's current generation. A later legitimate ConfigMap
// change (which doesn't bump the CR's generation) must still trigger a fresh deployment
// attempt - previously it was silently swallowed by "Default: nothing to do" because the
// config-changed redeploy path was gated on Programmed==True, and Case 2 (generation-based)
// never re-fires once ObservedGeneration already equals the current generation.

func TestGatewayConfigChangeUnsticksPermanentlyFailedGateway(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))

	const (
		name      = "gw1"
		namespace = "default"
	)

	oldHash := auth.CalculateConfigHash("replicaCount: 1\n")
	newValues := "replicaCount: 2\n" // the "fix" the operator applies after the outage clears
	require.NotEqual(t, oldHash, auth.CalculateConfigHash(newValues))

	gw := buildPermanentlyFailedGateway(name, namespace, oldHash)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-config", Namespace: namespace},
		Data:       map[string]string{"values.yaml": newValues},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&apiv1.APIGateway{}).
		WithObjects(gw, cm).
		Build()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	r := NewGatewayReconciler(c, scheme, &config.OperatorConfig{}, logger)

	trackingKey := types.NamespacedName{Namespace: namespace, Name: name}.String()
	// Mirror the exhausted-retries tracker state left behind by handleGatewayDeploymentError.
	r.gatewayTracker.Set(trackingKey, &GatewayTrackingEntry{
		Generation: 1,
		Status:     GatewayTrackingStatusDeployed,
		RetryCount: 10,
	})

	result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}})
	require.NoError(t, err)
	require.True(t, result.Requeue, "config change on a permanently-failed gateway must requeue for redeployment")

	updated := &apiv1.APIGateway{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, updated))

	cond := meta.FindStatusCondition(updated.Status.Conditions, apiv1.GatewayConditionProgrammed)
	require.NotNil(t, cond)
	require.Equal(t, metav1.ConditionFalse, cond.Status)
	require.Equal(t, "ConfigChanged", cond.Reason, "must pivot out of the terminal DeploymentFailed reason once config changes")

	entry, ok := r.gatewayTracker.Get(trackingKey)
	require.True(t, ok)
	require.Equal(t, GatewayTrackingStatusConfigChanged, entry.Status)
	require.Equal(t, 0, entry.RetryCount, "retry budget must reset for the new deployment attempt")
}

// TestGatewayNoConfigChangeStaysPermanentlyFailed ensures the fix doesn't over-correct:
// a permanently-failed gateway with NO config change must remain untouched until the
// config or CR spec actually changes.
func TestGatewayNoConfigChangeStaysPermanentlyFailed(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))

	const (
		name      = "gw1"
		namespace = "default"
	)

	values := "replicaCount: 1\n"
	hash := auth.CalculateConfigHash(values)

	gw := buildPermanentlyFailedGateway(name, namespace, hash)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-config", Namespace: namespace},
		Data:       map[string]string{"values.yaml": values},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&apiv1.APIGateway{}).
		WithObjects(gw, cm).
		Build()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	r := NewGatewayReconciler(c, scheme, &config.OperatorConfig{}, logger)

	trackingKey := types.NamespacedName{Namespace: namespace, Name: name}.String()
	r.gatewayTracker.Set(trackingKey, &GatewayTrackingEntry{
		Generation: 1,
		Status:     GatewayTrackingStatusDeployed,
		RetryCount: 10,
	})

	result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}})
	require.NoError(t, err)
	require.False(t, result.Requeue)
	require.Zero(t, result.RequeueAfter)

	updated := &apiv1.APIGateway{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, updated))

	cond := meta.FindStatusCondition(updated.Status.Conditions, apiv1.GatewayConditionProgrammed)
	require.NotNil(t, cond)
	require.Equal(t, apiv1.GatewayProgrammedReasonDeploymentFailed, cond.Reason)
}

// TestTerminalFailureRecordsAttemptedConfigHash locks in the counterpart to the fix above.
// Status.ConfigHash is otherwise only written on a successful deploy, so a config that can
// never deploy would leave the hash empty (or stale), keeping configChanged permanently true.
// Case 1 would then reset RetryCount to 0 on every reconcile and restart the retry budget
// forever, making MaxRetryAttempts non-terminating. Recording the attempted hash on terminal
// failure closes that loop - asserted here end to end.
func TestTerminalFailureRecordsAttemptedConfigHash(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))

	const (
		name       = "gw1"
		namespace  = "default"
		maxRetries = 3
	)

	values := "replicaCount: 1\n" // a config that never deploys successfully
	hash := auth.CalculateConfigHash(values)

	// Never successfully deployed: ConfigHash is empty, mid-retry rather than terminal.
	gw := buildPermanentlyFailedGateway(name, namespace, "")
	meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:               apiv1.GatewayConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: 0,
		Reason:             apiv1.GatewayProgrammedReasonRetrying,
		Message:            "Deployment failed, retrying (attempt 2/3): boom",
		LastTransitionTime: metav1.Now(),
	})
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-config", Namespace: namespace},
		Data:       map[string]string{"values.yaml": values},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&apiv1.APIGateway{}).
		WithObjects(gw, cm).
		Build()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := &config.OperatorConfig{}
	cfg.Reconciliation.MaxRetryAttempts = maxRetries
	r := NewGatewayReconciler(c, scheme, cfg, logger)

	trackingKey := types.NamespacedName{Namespace: namespace, Name: name}.String()
	entry := &GatewayTrackingEntry{
		Generation: 1,
		Status:     GatewayTrackingStatusRetrying,
		RetryCount: maxRetries - 1, // the next failure exhausts the budget
	}
	r.gatewayTracker.Set(trackingKey, entry)

	// Drive the final failing attempt.
	_, err := r.handleGatewayDeploymentError(ctx, gw, trackingKey, entry,
		errors.New("context deadline exceeded"), 0, hash)
	require.NoError(t, err)

	failed := &apiv1.APIGateway{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, failed))
	require.Equal(t, hash, failed.Status.ConfigHash,
		"terminal failure must record the attempted config hash, not leave it empty")

	cond := meta.FindStatusCondition(failed.Status.Conditions, apiv1.GatewayConditionProgrammed)
	require.NotNil(t, cond)
	require.Equal(t, apiv1.GatewayProgrammedReasonDeploymentFailed, cond.Reason)

	// The status write above re-enqueues a reconcile. With the hash recorded, that reconcile
	// must be a no-op instead of resetting the retry budget and redeploying.
	result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}})
	require.NoError(t, err)
	require.False(t, result.Requeue, "a terminally failed gateway must not requeue without a real config change")
	require.Zero(t, result.RequeueAfter)

	after, ok := r.gatewayTracker.Get(trackingKey)
	require.True(t, ok)
	require.Equal(t, maxRetries, after.RetryCount,
		"retry budget must stay exhausted - resetting it here is the infinite-redeploy loop")
}

// TestConfigRefRemovalDoesNotLoop covers a gateway whose spec.configRef was removed after a
// config had been recorded. The current hash is then "", so both the terminal-failure and the
// success path must overwrite the stale stored hash with "". Otherwise configChanged stays true
// forever and Case 1 resets the retry budget and redeploys on every reconcile.
func TestConfigRefRemovalDoesNotLoop(t *testing.T) {
	const (
		name       = "gw1"
		namespace  = "default"
		maxRetries = 3
	)
	staleHash := auth.CalculateConfigHash("replicaCount: 1\n")

	setup := func(t *testing.T) (*GatewayReconciler, *apiv1.APIGateway, string, *GatewayTrackingEntry) {
		scheme := runtime.NewScheme()
		require.NoError(t, clientgoscheme.AddToScheme(scheme))
		require.NoError(t, apiv1.AddToScheme(scheme))

		// configRef removed (generation bumped to 2), hash from the old ConfigMap still in status.
		gw := buildPermanentlyFailedGateway(name, namespace, staleHash)
		gw.Generation = 2
		gw.Spec.ConfigRef = nil
		meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
			Type:               apiv1.GatewayConditionProgrammed,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: 0,
			Reason:             apiv1.GatewayProgrammedReasonRetrying,
			Message:            "Deployment failed, retrying (attempt 2/3): boom",
			LastTransitionTime: metav1.Now(),
		})

		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&apiv1.APIGateway{}).
			WithObjects(gw).
			Build()

		cfg := &config.OperatorConfig{}
		cfg.Reconciliation.MaxRetryAttempts = maxRetries
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		r := NewGatewayReconciler(c, scheme, cfg, logger)

		trackingKey := types.NamespacedName{Namespace: namespace, Name: name}.String()
		entry := &GatewayTrackingEntry{
			Generation: 2,
			Status:     GatewayTrackingStatusRetrying,
			RetryCount: maxRetries - 1,
		}
		r.gatewayTracker.Set(trackingKey, entry)
		return r, gw, trackingKey, entry
	}

	t.Run("terminal failure", func(t *testing.T) {
		ctx := context.Background()
		r, gw, trackingKey, entry := setup(t)

		_, err := r.handleGatewayDeploymentError(ctx, gw, trackingKey, entry,
			errors.New("context deadline exceeded"), 0, "")
		require.NoError(t, err)

		failed := &apiv1.APIGateway{}
		require.NoError(t, r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, failed))
		require.Empty(t, failed.Status.ConfigHash, "stale hash must be cleared when configRef is removed")

		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}})
		require.NoError(t, err)
		require.False(t, result.Requeue, "must not treat the cleared configRef as a config change")
		require.Zero(t, result.RequeueAfter)

		after, ok := r.gatewayTracker.Get(trackingKey)
		require.True(t, ok)
		require.Equal(t, maxRetries, after.RetryCount, "retry budget must stay exhausted")
	})

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		r, gw, trackingKey, entry := setup(t)

		_, err := r.handleGatewayDeploymentSuccess(ctx, gw, trackingKey, entry, 0, "ready", "")
		require.NoError(t, err)

		deployed := &apiv1.APIGateway{}
		require.NoError(t, r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, deployed))
		require.Empty(t, deployed.Status.ConfigHash, "stale hash must be cleared when configRef is removed")

		cond := meta.FindStatusCondition(deployed.Status.Conditions, apiv1.GatewayConditionProgrammed)
		require.NotNil(t, cond)
		require.Equal(t, metav1.ConditionTrue, cond.Status)

		// The "already deployed" branch re-registers the gateway, which fails here because the
		// fake cluster has no gateway-controller Service; that error is unrelated to the loop.
		// The loop would instead take the config-changed branch: requeue, flip Programmed to
		// ConfigChanged and reset the tracker - assert none of that happened.
		result, _ := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}})
		require.False(t, result.Requeue, "a deployed gateway without configRef must not redeploy")

		latest := &apiv1.APIGateway{}
		require.NoError(t, r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, latest))
		cond = meta.FindStatusCondition(latest.Status.Conditions, apiv1.GatewayConditionProgrammed)
		require.NotNil(t, cond)
		require.Equal(t, metav1.ConditionTrue, cond.Status, "must stay Programmed, not flip to ConfigChanged")

		after, ok := r.gatewayTracker.Get(trackingKey)
		require.True(t, ok)
		require.Equal(t, GatewayTrackingStatusDeployed, after.Status)
	})
}

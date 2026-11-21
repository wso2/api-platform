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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
)

const (
	deletionTestName      = "gw-delete"
	deletionTestNamespace = "default"
)

var deletionTestKey = types.NamespacedName{Namespace: deletionTestNamespace, Name: deletionTestName}

// deletedGateway builds an APIGateway that was deleted deletedAgo ago and still
// carries the operator's finalizer, as the API server leaves it.
func deletedGateway(deletedAgo time.Duration, annotations map[string]string) *apiv1.APIGateway {
	ts := metav1.NewTime(time.Now().Add(-deletedAgo))
	return &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:              deletionTestName,
			Namespace:         deletionTestNamespace,
			Generation:        2,
			Annotations:       annotations,
			Finalizers:        []string{apigatewayFinalizerName},
			DeletionTimestamp: &ts,
		},
		Status: apiv1.GatewayStatus{
			Phase: apiv1.GatewayPhaseReady,
			Conditions: []metav1.Condition{{
				Type:               apiv1.GatewayConditionProgrammed,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 1,
				Reason:             apiv1.GatewayProgrammedReasonProgrammed,
				LastTransitionTime: metav1.Now(),
			}},
		},
	}
}

// deletionTestReconciler returns a reconciler over gw whose uninstall step only
// counts its calls, so the tests never reach Helm.
func deletionTestReconciler(t *testing.T, grace time.Duration, objs ...client.Object) (*GatewayReconciler, client.Client, *int) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&apiv1.APIGateway{}).
		WithObjects(objs...).
		Build()

	cfg := &config.OperatorConfig{}
	cfg.Gateway.DeletionGracePeriod = grace
	r := NewGatewayReconciler(c, scheme, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))

	uninstalls := 0
	r.uninstallGateway = func(_ context.Context, owner *apiv1.APIGateway) error {
		require.Equal(t, deletionTestName, owner.Name)
		uninstalls++
		return nil
	}
	return r, c, &uninstalls
}

func reconcileDeletionTestGateway(t *testing.T, r *GatewayReconciler) ctrl.Result {
	t.Helper()
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: deletionTestKey})
	require.NoError(t, err)
	return result
}

func requireGatewayRemoved(t *testing.T, c client.Client) {
	t.Helper()
	err := c.Get(context.Background(), deletionTestKey, &apiv1.APIGateway{})
	require.True(t, apierrors.IsNotFound(err), "APIGateway should be gone once the finalizer is removed, got %v", err)
}

func TestGatewayDeletionWithinGracePeriodKeepsRelease(t *testing.T) {
	r, c, uninstalls := deletionTestReconciler(t, time.Hour, deletedGateway(time.Minute, nil))

	result := reconcileDeletionTestGateway(t, r)

	require.Zero(t, *uninstalls, "release must not be uninstalled inside the grace period")
	require.Greater(t, result.RequeueAfter, 58*time.Minute)
	require.LessOrEqual(t, result.RequeueAfter, time.Hour)

	got := &apiv1.APIGateway{}
	require.NoError(t, c.Get(context.Background(), deletionTestKey, got))
	require.True(t, controllerutil.ContainsFinalizer(got, apigatewayFinalizerName), "finalizer must stay while the release is kept")
	require.Equal(t, apiv1.GatewayPhaseDeleting, got.Status.Phase)

	pending := meta.FindStatusCondition(got.Status.Conditions, apiv1.GatewayConditionDeletionPending)
	require.NotNil(t, pending)
	require.Equal(t, metav1.ConditionTrue, pending.Status)
	require.Equal(t, apiv1.GatewayDeletionPendingReasonGracePeriod, pending.Reason)
	proceedAt := got.DeletionTimestamp.Add(time.Hour).UTC().Format(time.RFC3339)
	require.Contains(t, pending.Message, proceedAt)
	require.Contains(t, pending.Message, AnnAPIGatewayDeleteImmediately)

	programmed := meta.FindStatusCondition(got.Status.Conditions, apiv1.GatewayConditionProgrammed)
	require.NotNil(t, programmed)
	require.Equal(t, metav1.ConditionTrue, programmed.Status, "the gateway is still serving while deletion is pending")
}

func TestGatewayDeletionHoldDoesNotRewriteStatus(t *testing.T) {
	r, c, uninstalls := deletionTestReconciler(t, time.Hour, deletedGateway(time.Minute, nil))

	reconcileDeletionTestGateway(t, r)
	first := &apiv1.APIGateway{}
	require.NoError(t, c.Get(context.Background(), deletionTestKey, first))

	reconcileDeletionTestGateway(t, r)
	second := &apiv1.APIGateway{}
	require.NoError(t, c.Get(context.Background(), deletionTestKey, second))

	require.Zero(t, *uninstalls)
	require.Equal(t, first.ResourceVersion, second.ResourceVersion,
		"a repeated hold must not patch status again, or each patch would trigger another reconcile")
}

func TestGatewayDeletionAfterGracePeriodUninstalls(t *testing.T) {
	r, c, uninstalls := deletionTestReconciler(t, time.Hour, deletedGateway(2*time.Hour, nil))

	result := reconcileDeletionTestGateway(t, r)

	require.Equal(t, 1, *uninstalls)
	require.Zero(t, result.RequeueAfter)
	requireGatewayRemoved(t, c)
}

func TestGatewayDeletionDeleteImmediatelyAnnotationSkipsGracePeriod(t *testing.T) {
	gw := deletedGateway(time.Minute, map[string]string{AnnAPIGatewayDeleteImmediately: "true"})
	r, c, uninstalls := deletionTestReconciler(t, time.Hour, gw)

	reconcileDeletionTestGateway(t, r)

	require.Equal(t, 1, *uninstalls)
	requireGatewayRemoved(t, c)
}

func TestGatewayDeletionDeleteImmediatelyAnnotationOtherValueWaits(t *testing.T) {
	gw := deletedGateway(time.Minute, map[string]string{AnnAPIGatewayDeleteImmediately: "false"})
	r, _, uninstalls := deletionTestReconciler(t, time.Hour, gw)

	result := reconcileDeletionTestGateway(t, r)

	require.Zero(t, *uninstalls)
	require.Positive(t, result.RequeueAfter)
}

func TestGatewayDeletionWithoutGracePeriodUninstallsImmediately(t *testing.T) {
	r, c, uninstalls := deletionTestReconciler(t, 0, deletedGateway(0, nil))

	reconcileDeletionTestGateway(t, r)

	require.Equal(t, 1, *uninstalls, "a zero grace period keeps the previous immediate uninstall")
	requireGatewayRemoved(t, c)
}

// Removing the finalizer by hand during the grace period lets the APIGateway go
// while its release keeps running. An APIGateway re-created under the same name
// must then deploy (adopting the release) instead of being matched against what
// was tracked for the old object.
func TestGatewayRecreatedAfterFinalizerRemovalStartsFresh(t *testing.T) {
	r, c, _ := deletionTestReconciler(t, time.Hour)
	trackingKey := deletionTestKey.String()
	staleEntry := &GatewayTrackingEntry{Generation: 1, Status: GatewayTrackingStatusDeployed}

	// The old object is gone: the NotFound reconcile forgets it.
	r.gatewayTracker.Set(trackingKey, staleEntry)
	reconcileDeletionTestGateway(t, r)
	_, tracked := r.gatewayTracker.Get(trackingKey)
	require.False(t, tracked, "tracking must be dropped once the APIGateway is gone")

	// Even if that NotFound was never observed, a new object (no finalizer yet)
	// clears the stale entry before it is processed.
	r.gatewayTracker.Set(trackingKey, staleEntry)
	require.NoError(t, c.Create(context.Background(), &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{Name: deletionTestName, Namespace: deletionTestNamespace, Generation: 1},
	}))
	result := reconcileDeletionTestGateway(t, r)
	require.True(t, result.Requeue)
	_, tracked = r.gatewayTracker.Get(trackingKey)
	require.False(t, tracked, "a re-created APIGateway must not inherit the old object's tracking entry")

	got := &apiv1.APIGateway{}
	require.NoError(t, c.Get(context.Background(), deletionTestKey, got))
	require.True(t, controllerutil.ContainsFinalizer(got, apigatewayFinalizerName))
}

func TestGatewayDeletionWait(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	deletedAt := metav1.NewTime(now.Add(-10 * time.Minute))

	cases := []struct {
		name        string
		grace       time.Duration
		annotations map[string]string
		wantWait    time.Duration
	}{
		{name: "no grace period", grace: 0, wantWait: 0},
		{name: "inside grace period", grace: time.Hour, wantWait: 50 * time.Minute},
		{name: "grace period passed", grace: 5 * time.Minute, wantWait: 0},
		{name: "annotation true", grace: time.Hour, annotations: map[string]string{AnnAPIGatewayDeleteImmediately: "true"}, wantWait: 0},
		{name: "annotation case and spaces", grace: time.Hour, annotations: map[string]string{AnnAPIGatewayDeleteImmediately: " TRUE "}, wantWait: 0},
		{name: "annotation empty", grace: time.Hour, annotations: map[string]string{AnnAPIGatewayDeleteImmediately: ""}, wantWait: 50 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.OperatorConfig{}
			cfg.Gateway.DeletionGracePeriod = tc.grace
			r := &GatewayReconciler{Config: cfg}
			gw := &apiv1.APIGateway{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &deletedAt, Annotations: tc.annotations}}

			wait, _ := r.gatewayDeletionWait(gw, now)
			require.Equal(t, tc.wantWait, wait)
		})
	}
}

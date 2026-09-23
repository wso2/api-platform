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
	"io"
	"log/slog"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

// failingStatusWriter fails the first n status patches, then delegates. It reproduces a
// transient status-subresource write failure without touching production code.
type failingStatusWriter struct {
	client.SubResourceWriter
	failures *int
}

func (w *failingStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	if *w.failures > 0 {
		*w.failures--
		return errors.New("simulated status patch failure")
	}
	return w.SubResourceWriter.Patch(ctx, obj, patch, opts...)
}

func (w *failingStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if *w.failures > 0 {
		*w.failures--
		return errors.New("simulated status update failure")
	}
	return w.SubResourceWriter.Update(ctx, obj, opts...)
}

type failingStatusClient struct {
	client.Client
	failures *int
}

func (c *failingStatusClient) Status() client.SubResourceWriter {
	return &failingStatusWriter{SubResourceWriter: c.Client.Status(), failures: c.failures}
}

// newStatusFailureReconciler builds a reconciler whose first statusFailures status writes fail.
// deployGateway is left as the real processGatewayDeployment so the actual state transitions
// run; the Helm step is replaced by deployResult.
func newStatusFailureReconciler(t *testing.T, gw *apiv1.APIGateway, statusFailures int) (*GatewayReconciler, *int) {
	t.Helper()
	scheme := recoveryScheme(t)
	base := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&apiv1.APIGateway{}).
		WithRuntimeObjects(gw).Build()
	remaining := statusFailures
	r := &GatewayReconciler{
		Client:         &failingStatusClient{Client: base, failures: &remaining},
		Scheme:         scheme,
		Config:         recoveryConfig(),
		gatewayTracker: NewGatewayTracker(),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return r, &remaining
}

// TestProcessGatewayDeployment_initialStatusPatchFailureStaysRecoverable covers the first
// wedge: processGatewayDeployment records Processing, then the initial-condition patch fails
// and the reconcile returns an error. Because controller-runtime requeues that error, the
// next reconcile must be able to retry — if the tracker still said Processing it would skip
// and return nil, ending the requeue chain and stranding the gateway.
func TestProcessGatewayDeployment_initialStatusPatchFailureStaysRecoverable(t *testing.T) {
	gw := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: testGateway, Namespace: testNamespace, Generation: 1,
			Finalizers: []string{apigatewayFinalizerName},
		},
	}
	r, _ := newStatusFailureReconciler(t, gw, 1)

	// Attempt 1: the real code path, with the initial status patch failing.
	_, err := r.processGatewayDeployment(context.Background(), gw, testTrackKey, 1, "", "hash-1")
	if err == nil {
		t.Fatal("expected the initial status patch failure to be returned")
	}

	if entry, ok := r.gatewayTracker.Get(testTrackKey); ok {
		t.Fatalf("tracker still holds %q after a failed initial status patch; the next reconcile would skip", entry.Status)
	}

	// Attempt 2 is the requeue. Drive the real decision path and prove it deploys rather
	// than skipping.
	deployCalls := 0
	r.deployGateway = func(_ context.Context, _ *apiv1.APIGateway, _ string, _ int64, _, _ string) (ctrl.Result, error) {
		deployCalls++
		return ctrl.Result{}, nil
	}
	if _, err := callDecide(t, r, gw); err != nil {
		t.Fatalf("second reconcile returned an error: %v", err)
	}
	if deployCalls != 1 {
		t.Errorf("deploy attempts on the requeued reconcile = %d, want 1", deployCalls)
	}
}

// TestHandleGatewayDeploymentSuccess_statusPatchFailureStaysRecoverable covers the second
// wedge: the deployment succeeded, the tracker moved to Deployed, then Programmed=True failed
// to persist. The CR still reports failure, and a Deployed tracker made every later reconcile
// skip on "status not yet propagated", so the gateway stayed false forever.
func TestHandleGatewayDeploymentSuccess_statusPatchFailureStaysRecoverable(t *testing.T) {
	gw := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: testGateway, Namespace: testNamespace, Generation: 1,
			Finalizers: []string{apigatewayFinalizerName},
		},
	}
	r, _ := newStatusFailureReconciler(t, gw, 1)

	entry := &GatewayTrackingEntry{Generation: 1, Status: GatewayTrackingStatusProcessing, InputHash: "hash-1"}
	r.gatewayTracker.Set(testTrackKey, entry)

	_, err := r.handleGatewayDeploymentSuccess(context.Background(), gw, testTrackKey, entry, 0, "ready", "")
	if err == nil {
		t.Fatal("expected the success status patch failure to be returned")
	}

	if stored, ok := r.gatewayTracker.Get(testTrackKey); ok {
		t.Fatalf("tracker still holds %q after a failed success status patch; every later reconcile would skip", stored.Status)
	}

	// Confirm the persisted condition really is still not programmed, so the wedge would
	// have been permanent rather than cosmetic.
	stored := &apiv1.APIGateway{}
	if err := r.Get(context.Background(), typesName(), stored); err != nil {
		t.Fatalf("get gateway: %v", err)
	}
	if cond := findProgrammed(stored); cond != nil && cond.Status == metav1.ConditionTrue {
		t.Fatal("expected Programmed to remain not-True after the failed patch")
	}

	// The requeued reconcile must re-drive the deployment.
	deployCalls := 0
	r.deployGateway = func(_ context.Context, _ *apiv1.APIGateway, _ string, _ int64, _, _ string) (ctrl.Result, error) {
		deployCalls++
		return ctrl.Result{}, nil
	}
	if _, err := callDecide(t, r, stored); err != nil {
		t.Fatalf("second reconcile returned an error: %v", err)
	}
	if deployCalls != 1 {
		t.Errorf("deploy attempts on the requeued reconcile = %d, want 1", deployCalls)
	}
}

// TestHandleGatewayDeploymentError_exhaustionStatusPatchFailureStaysRecoverable drives the real
// exhaustion path with the status patch failing, rather than hand-building a Failed entry.
func TestHandleGatewayDeploymentError_exhaustionStatusPatchFailureStaysRecoverable(t *testing.T) {
	gw := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: testGateway, Namespace: testNamespace, Generation: 1,
			Finalizers: []string{apigatewayFinalizerName},
		},
	}
	r, _ := newStatusFailureReconciler(t, gw, 1)

	// Record the failure against the gateway's real deployment inputs, so the follow-up
	// reconciles exercise the unchanged-input path rather than an accidental hash mismatch.
	_, inputHash, err := r.deploymentInputs(context.Background(), gw)
	if err != nil {
		t.Fatalf("deploymentInputs: %v", err)
	}
	entry := &GatewayTrackingEntry{
		Generation: 1, Status: GatewayTrackingStatusProcessing, RetryCount: 2, InputHash: inputHash,
	}
	r.gatewayTracker.Set(testTrackKey, entry)

	if _, err := r.handleGatewayDeploymentError(context.Background(), gw, testTrackKey, entry, errors.New("boom"), 0); err == nil {
		t.Fatal("expected the failure status patch error to be returned")
	}

	// Unlike Processing/Deployed, a Failed entry carries its own bounded retry, so keeping it
	// is what makes the gateway recoverable even though the failure was never persisted.
	stored, ok := r.gatewayTracker.Get(testTrackKey)
	if !ok {
		t.Fatal("expected the Failed entry to be retained so the bounded retry survives")
	}
	if stored.Status != GatewayTrackingStatusFailed {
		t.Errorf("status = %q, want %q", stored.Status, GatewayTrackingStatusFailed)
	}
	if stored.NextRetryTime.IsZero() {
		t.Fatal("expected a bounded next-retry time despite the failed status write")
	}

	deployCalls := 0
	r.deployGateway = func(_ context.Context, _ *apiv1.APIGateway, _ string, _ int64, _, _ string) (ctrl.Result, error) {
		deployCalls++
		return ctrl.Result{}, nil
	}

	// Second reconcile, inside the retry window: the failure was never persisted, so the CR
	// still carries no Programmed condition and this runs the real decision path.
	res, err := callDecide(t, r, gw)
	if err != nil {
		t.Fatalf("reconcile inside the retry window returned an error: %v", err)
	}
	if deployCalls != 0 {
		t.Errorf("deploy attempts inside the retry window = %d, want 0", deployCalls)
	}
	if res.RequeueAfter <= 0 {
		t.Errorf("RequeueAfter = %v, want a bounded retry despite the failed status write", res.RequeueAfter)
	}

	// Third reconcile, once the window has elapsed: exactly one bounded attempt.
	stored.NextRetryTime = time.Now().Add(-time.Second)
	r.gatewayTracker.Set(testTrackKey, stored)

	if _, err := callDecide(t, r, gw); err != nil {
		t.Fatalf("reconcile after the retry window returned an error: %v", err)
	}
	if deployCalls != 1 {
		t.Errorf("deploy attempts after the retry window = %d, want exactly 1", deployCalls)
	}
}

func findProgrammed(gw *apiv1.APIGateway) *metav1.Condition {
	for i := range gw.Status.Conditions {
		if gw.Status.Conditions[i].Type == apiv1.GatewayConditionProgrammed {
			return &gw.Status.Conditions[i]
		}
	}
	return nil
}

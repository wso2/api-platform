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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
)

const (
	testSyncPeriod = 10 * time.Minute
	testNamespace  = "gw-ns"
	testGateway    = "gw"
	testTrackKey   = testNamespace + "/" + testGateway
)

func recoveryScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 to scheme: %v", err)
	}
	if err := apiv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apiv1 to scheme: %v", err)
	}
	return scheme
}

func recoveryConfig() *config.OperatorConfig {
	cfg := &config.OperatorConfig{}
	cfg.Reconciliation.MaxRetryAttempts = 3
	cfg.Reconciliation.InitialBackoff = time.Second
	cfg.Reconciliation.MaxBackoffDuration = time.Minute
	cfg.Reconciliation.SyncPeriod = testSyncPeriod
	return cfg
}

// failedGateway builds an APIGateway whose persisted status reports the exhausted failure:
// Programmed=False with DeploymentFailed at the current generation. This is the exact shape
// that used to be terminal.
func failedGateway(generation int64) *apiv1.APIGateway {
	gw := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testGateway,
			Namespace:  testNamespace,
			Generation: generation,
			Finalizers: []string{apigatewayFinalizerName},
		},
	}
	meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:               apiv1.GatewayConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: generation,
		Reason:             apiv1.GatewayProgrammedReasonDeploymentFailed,
		Message:            "Max retries (3) exceeded. Last error: boom",
		LastTransitionTime: metav1.Now(),
	})
	return gw
}

// newRecoveryReconciler wires a reconciler whose deployments are recorded instead of
// executed, so a reconciliation decision can be asserted without a cluster or Helm.
func newRecoveryReconciler(t *testing.T, objs ...runtime.Object) (*GatewayReconciler, *int) {
	t.Helper()
	scheme := recoveryScheme(t)
	builder := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&apiv1.APIGateway{})
	for _, o := range objs {
		builder = builder.WithRuntimeObjects(o)
	}
	deployCalls := 0
	r := &GatewayReconciler{
		Client:         builder.Build(),
		Scheme:         scheme,
		Config:         recoveryConfig(),
		gatewayTracker: NewGatewayTracker(),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	r.deployGateway = func(_ context.Context, _ *apiv1.APIGateway, trackingKey string, generation int64, _, inputHash string) (ctrl.Result, error) {
		deployCalls++
		r.gatewayTracker.Set(trackingKey, &GatewayTrackingEntry{
			Generation: generation,
			Status:     GatewayTrackingStatusProcessing,
			RetryCount: retryCountFor(mustEntry(r, trackingKey), true, generation, inputHash),
			InputHash:  inputHash,
		})
		return ctrl.Result{}, nil
	}
	return r, &deployCalls
}

func mustEntry(r *GatewayReconciler, key string) *GatewayTrackingEntry {
	entry, _ := r.gatewayTracker.Get(key)
	return entry
}

// typesName is the namespaced name of the gateway under test.
func typesName() types.NamespacedName {
	return types.NamespacedName{Namespace: testNamespace, Name: testGateway}
}

// callDecide runs the reconciliation decision for a gateway using its persisted status.
func callDecide(t *testing.T, r *GatewayReconciler, gw *apiv1.APIGateway) (ctrl.Result, error) {
	t.Helper()
	cond := meta.FindStatusCondition(gw.Status.Conditions, apiv1.GatewayConditionProgrammed)
	observed := int64(0)
	if cond != nil {
		observed = cond.ObservedGeneration
	}
	entry, has := r.gatewayTracker.Get(testTrackKey)
	return r.decideAndProcess(context.Background(), gw, testTrackKey, gw.Generation, observed, cond, entry, has)
}

// --- exhaustion bookkeeping -------------------------------------------------------------

func TestHandleGatewayDeploymentError_exhaustionRecordsFailedNotDeployed(t *testing.T) {
	gw := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{Name: testGateway, Namespace: testNamespace, Generation: 1},
	}
	r, _ := newRecoveryReconciler(t, gw)

	// One attempt short of the limit, so this call exhausts the budget.
	entry := &GatewayTrackingEntry{Generation: 1, Status: GatewayTrackingStatusProcessing, RetryCount: 2, InputHash: "h1"}
	r.gatewayTracker.Set(testTrackKey, entry)

	before := time.Now()
	res, err := r.handleGatewayDeploymentError(context.Background(), gw, testTrackKey, entry, errors.New("boom"), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored, ok := r.gatewayTracker.Get(testTrackKey)
	if !ok {
		t.Fatal("expected a tracking entry after exhaustion")
	}
	// The regression: exhaustion used to be recorded as Deployed, which made every recovery
	// path treat the gateway as finished.
	if stored.Status != GatewayTrackingStatusFailed {
		t.Errorf("status = %q, want %q", stored.Status, GatewayTrackingStatusFailed)
	}
	if stored.Status == GatewayTrackingStatusDeployed {
		t.Error("an exhausted deployment must never be recorded as deployed")
	}
	if !stored.NextRetryTime.After(before) {
		t.Errorf("NextRetryTime = %v, want a bounded future retry", stored.NextRetryTime)
	}
	if res.RequeueAfter != testSyncPeriod {
		t.Errorf("RequeueAfter = %v, want %v", res.RequeueAfter, testSyncPeriod)
	}
}

func TestHandleGatewayDeploymentError_exhaustionWritesDeploymentFailed(t *testing.T) {
	gw := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{Name: testGateway, Namespace: testNamespace, Generation: 1},
	}
	r, _ := newRecoveryReconciler(t, gw)
	entry := &GatewayTrackingEntry{Generation: 1, Status: GatewayTrackingStatusProcessing, RetryCount: 2}
	r.gatewayTracker.Set(testTrackKey, entry)

	if _, err := r.handleGatewayDeploymentError(context.Background(), gw, testTrackKey, entry, errors.New("boom"), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored := &apiv1.APIGateway{}
	if err := r.Get(context.Background(), typesName(), stored); err != nil {
		t.Fatalf("get gateway: %v", err)
	}
	cond := meta.FindStatusCondition(stored.Status.Conditions, apiv1.GatewayConditionProgrammed)
	if cond == nil {
		t.Fatal("expected a Programmed condition")
	}
	if cond.Status != metav1.ConditionFalse {
		t.Errorf("condition status = %v, want False", cond.Status)
	}
	if cond.Reason != apiv1.GatewayProgrammedReasonDeploymentFailed {
		t.Errorf("reason = %q, want %q", cond.Reason, apiv1.GatewayProgrammedReasonDeploymentFailed)
	}
	if cond.ObservedGeneration != 1 {
		t.Errorf("observedGeneration = %d, want 1", cond.ObservedGeneration)
	}
}

func TestHandleGatewayDeploymentError_belowLimitStillRetries(t *testing.T) {
	gw := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{Name: testGateway, Namespace: testNamespace, Generation: 1},
	}
	r, _ := newRecoveryReconciler(t, gw)
	entry := &GatewayTrackingEntry{Generation: 1, Status: GatewayTrackingStatusProcessing, RetryCount: 0}
	r.gatewayTracker.Set(testTrackKey, entry)

	res, err := r.handleGatewayDeploymentError(context.Background(), gw, testTrackKey, entry, errors.New("boom"), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stored, _ := r.gatewayTracker.Get(testTrackKey)
	if stored.Status != GatewayTrackingStatusRetrying {
		t.Errorf("status = %q, want %q (unchanged behaviour below the limit)", stored.Status, GatewayTrackingStatusRetrying)
	}
	if res.RequeueAfter <= 0 {
		t.Error("expected a backoff requeue while retries remain")
	}
}

// --- recovery decisions ------------------------------------------------------------------

func TestDecideAndProcess_restartWithPersistedFailureRecovers(t *testing.T) {
	// Controller restarted: the failure is persisted but no tracker entry exists. This used
	// to fall through to "nothing to do" and wedge the gateway permanently.
	gw := failedGateway(1)
	r, calls := newRecoveryReconciler(t, gw)

	if _, err := callDecide(t, r, gw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("deploy attempts = %d, want 1 after a restart with a persisted failure", *calls)
	}
}

func TestDecideAndProcess_unchangedFailureBeforeWindowDoesNotDeploy(t *testing.T) {
	gw := failedGateway(1)
	r, calls := newRecoveryReconciler(t, gw)

	_, inputHash, err := r.deploymentInputs(context.Background(), gw)
	if err != nil {
		t.Fatalf("deploymentInputs: %v", err)
	}
	r.gatewayTracker.Set(testTrackKey, &GatewayTrackingEntry{
		Generation:    1,
		Status:        GatewayTrackingStatusFailed,
		RetryCount:    3,
		InputHash:     inputHash,
		NextRetryTime: time.Now().Add(testSyncPeriod),
	})

	res, err := callDecide(t, r, gw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 0 {
		t.Errorf("deploy attempts = %d, want 0 while inside the retry window", *calls)
	}
	if res.RequeueAfter <= 0 || res.RequeueAfter > testSyncPeriod {
		t.Errorf("RequeueAfter = %v, want a bounded wait within the retry window", res.RequeueAfter)
	}
}

func TestDecideAndProcess_statusOnlyReconcilesDoNotHotLoop(t *testing.T) {
	// Repeated status-driven reconciles of an unchanged failure must not deploy and must not
	// write status, otherwise each write triggers the next reconcile.
	gw := failedGateway(1)
	r, calls := newRecoveryReconciler(t, gw)

	_, inputHash, err := r.deploymentInputs(context.Background(), gw)
	if err != nil {
		t.Fatalf("deploymentInputs: %v", err)
	}
	r.gatewayTracker.Set(testTrackKey, &GatewayTrackingEntry{
		Generation:    1,
		Status:        GatewayTrackingStatusFailed,
		RetryCount:    3,
		InputHash:     inputHash,
		NextRetryTime: time.Now().Add(testSyncPeriod),
	})

	for i := 0; i < 5; i++ {
		res, err := callDecide(t, r, gw)
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if res.RequeueAfter <= 0 {
			t.Fatalf("iteration %d: expected a timed requeue, got %v", i, res)
		}
	}
	if *calls != 0 {
		t.Errorf("deploy attempts = %d, want 0 across repeated status reconciles", *calls)
	}

	// The condition must be untouched, so no status write re-triggers reconciliation.
	stored := &apiv1.APIGateway{}
	if err := r.Get(context.Background(), typesName(), stored); err != nil {
		t.Fatalf("get gateway: %v", err)
	}
	cond := meta.FindStatusCondition(stored.Status.Conditions, apiv1.GatewayConditionProgrammed)
	if cond == nil || cond.Reason != apiv1.GatewayProgrammedReasonDeploymentFailed {
		t.Errorf("condition = %v, want the failure left untouched", cond)
	}
}

func TestDecideAndProcess_retryWindowElapsedDeploysOnce(t *testing.T) {
	gw := failedGateway(1)
	r, calls := newRecoveryReconciler(t, gw)

	_, inputHash, err := r.deploymentInputs(context.Background(), gw)
	if err != nil {
		t.Fatalf("deploymentInputs: %v", err)
	}
	r.gatewayTracker.Set(testTrackKey, &GatewayTrackingEntry{
		Generation:    1,
		Status:        GatewayTrackingStatusFailed,
		RetryCount:    3,
		InputHash:     inputHash,
		NextRetryTime: time.Now().Add(-time.Second), // window already elapsed
	})

	if _, err := callDecide(t, r, gw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 1 {
		t.Errorf("deploy attempts = %d, want exactly 1 once the retry window elapsed", *calls)
	}
}

func TestDecideAndProcess_configMapChangeRecoversImmediately(t *testing.T) {
	gw := failedGateway(1)
	gw.Spec.ConfigRef = &corev1.LocalObjectReference{Name: "gw-values"}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-values", Namespace: testNamespace},
		Data:       map[string]string{"values.yaml": "replicaCount: 2\n"},
	}
	r, calls := newRecoveryReconciler(t, gw, cm)

	// The failure was recorded against different ConfigMap content.
	r.gatewayTracker.Set(testTrackKey, &GatewayTrackingEntry{
		Generation:    1,
		Status:        GatewayTrackingStatusFailed,
		RetryCount:    3,
		InputHash:     "stale-hash",
		NextRetryTime: time.Now().Add(testSyncPeriod), // window has NOT elapsed
	})

	if _, err := callDecide(t, r, gw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 1 {
		t.Errorf("deploy attempts = %d, want 1 immediately after a ConfigMap change", *calls)
	}
}

// TestDecideAndProcess_specInfrastructureAnnotationChangeRecoversImmediately covers a change
// to spec.infrastructure.annotations, which is a deployment input: it becomes commonAnnotations
// in the rendered values. Note this is a *spec* change, so on a real cluster it also bumps
// metadata.generation; it is not the `kubectl annotate` route from the issue, which is covered
// by TestDecideAndProcess_metadataAnnotationOnlyChangeWaitsForRetryWindow below.
func TestDecideAndProcess_specInfrastructureAnnotationChangeRecoversImmediately(t *testing.T) {
	gw := failedGateway(1)
	r, calls := newRecoveryReconciler(t, gw)

	// Record the failure against the gateway with no infrastructure annotations.
	_, baseHash, err := r.deploymentInputs(context.Background(), gw)
	if err != nil {
		t.Fatalf("deploymentInputs: %v", err)
	}
	r.gatewayTracker.Set(testTrackKey, &GatewayTrackingEntry{
		Generation:    1,
		Status:        GatewayTrackingStatusFailed,
		RetryCount:    3,
		InputHash:     baseHash,
		NextRetryTime: time.Now().Add(testSyncPeriod),
	})

	// A deployment-affecting annotation changes what would be rendered.
	gw.Spec.Infrastructure = &apiv1.GatewayInfrastructure{
		Annotations: map[string]string{"example.com/rollout": "2"},
	}

	if _, err := callDecide(t, r, gw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 1 {
		t.Errorf("deploy attempts = %d, want 1 immediately after a spec.infrastructure annotation change", *calls)
	}
}

// TestDecideAndProcess_metadataAnnotationOnlyChangeWaitsForRetryWindow pins down the exact
// route the issue tried: `kubectl annotate apigateway ...`, which changes ObjectMeta.Annotations
// without bumping metadata.generation.
//
// This is NOT an immediate retry trigger, and that is deliberate. Metadata annotations are not
// deployment inputs — only spec.infrastructure labels/annotations reach the rendered values via
// commonAnnotations — so fingerprinting them would mean retrying on changes that cannot alter
// the outcome. It would also be a hot-loop risk: this codebase already writes operator-managed
// annotations onto resources it reconciles (see httproute_controller.go), so any such annotation
// added to APIGateway later would retrigger deployments on its own writes. Making
// re-annotation a supported reset would mean introducing a documented reset annotation, which
// is a public API decision for the maintainers.
//
// The gateway is still not wedged: it recovers on the bounded retry window, on an operator
// restart, and on a ConfigMap correction. This test asserts the honest behaviour so the
// limitation cannot be mistaken for a fix.
func TestDecideAndProcess_metadataAnnotationOnlyChangeWaitsForRetryWindow(t *testing.T) {
	gw := failedGateway(1)
	r, calls := newRecoveryReconciler(t, gw)

	_, baseHash, err := r.deploymentInputs(context.Background(), gw)
	if err != nil {
		t.Fatalf("deploymentInputs: %v", err)
	}
	r.gatewayTracker.Set(testTrackKey, &GatewayTrackingEntry{
		Generation:    1,
		Status:        GatewayTrackingStatusFailed,
		RetryCount:    3,
		InputHash:     baseHash,
		NextRetryTime: time.Now().Add(testSyncPeriod),
	})

	// `kubectl annotate` — metadata only, generation unchanged.
	generationBefore := gw.Generation
	gw.ObjectMeta.Annotations = map[string]string{"ops.example.com/nudge": "1"}
	if gw.Generation != generationBefore {
		t.Fatalf("test setup changed the generation (%d -> %d); it must stay equal to observedGeneration",
			generationBefore, gw.Generation)
	}

	res, err := callDecide(t, r, gw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 0 {
		t.Errorf("deploy attempts = %d, want 0: a metadata annotation is not a deployment input", *calls)
	}
	// Crucially it is not wedged either — a bounded retry is still scheduled.
	if res.RequeueAfter <= 0 {
		t.Errorf("RequeueAfter = %v, want a bounded retry so the gateway still recovers", res.RequeueAfter)
	}

	// And the fingerprint must be unchanged by metadata annotations, so the retry budget is
	// not silently reset by an unrelated edit.
	_, afterHash, err := r.deploymentInputs(context.Background(), gw)
	if err != nil {
		t.Fatalf("deploymentInputs: %v", err)
	}
	if afterHash != baseHash {
		t.Error("metadata annotations must not affect the deployment input fingerprint")
	}
}

func TestDeploymentInputs_infrastructureAnnotationOrderIsStable(t *testing.T) {
	// The overlay is marshalled YAML with sorted keys, so a differently-ordered literal must
	// produce the same fingerprint; otherwise map iteration order would reset retry budgets.
	gwA := failedGateway(1)
	gwA.Spec.Infrastructure = &apiv1.GatewayInfrastructure{
		Annotations: map[string]string{"a": "1", "b": "2", "c": "3"},
	}
	gwB := failedGateway(1)
	gwB.Spec.Infrastructure = &apiv1.GatewayInfrastructure{
		Annotations: map[string]string{"c": "3", "b": "2", "a": "1"},
	}

	r, _ := newRecoveryReconciler(t, gwA)
	_, hashA, err := r.deploymentInputs(context.Background(), gwA)
	if err != nil {
		t.Fatalf("deploymentInputs A: %v", err)
	}
	// Repeat many times: a single comparison could pass by luck with random map ordering.
	for i := 0; i < 50; i++ {
		_, hashB, err := r.deploymentInputs(context.Background(), gwB)
		if err != nil {
			t.Fatalf("deploymentInputs B: %v", err)
		}
		if hashA != hashB {
			t.Fatalf("iteration %d: fingerprint depends on map ordering (%s vs %s)", i, hashA, hashB)
		}
	}
}

func TestDecideAndProcess_failedStatusNotYetObservedStillRecovers(t *testing.T) {
	// The failure was recorded in the tracker but the status write never became visible, so
	// the condition still shows the earlier retry with observedGeneration 0.
	gw := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: testGateway, Namespace: testNamespace, Generation: 1,
			Finalizers: []string{apigatewayFinalizerName},
		},
	}
	meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:               apiv1.GatewayConditionProgrammed,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: 0,
		Reason:             apiv1.GatewayProgrammedReasonRetrying,
		LastTransitionTime: metav1.Now(),
	})
	r, calls := newRecoveryReconciler(t, gw)

	r.gatewayTracker.Set(testTrackKey, &GatewayTrackingEntry{
		Generation:    1,
		Status:        GatewayTrackingStatusFailed,
		RetryCount:    3,
		InputHash:     "stale-hash", // inputs since corrected
		NextRetryTime: time.Now().Add(testSyncPeriod),
	})

	if _, err := callDecide(t, r, gw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 1 {
		t.Errorf("deploy attempts = %d, want 1 even when the failure status is not yet observable", *calls)
	}
}

func TestDecideAndProcess_programmedGatewayIsUntouched(t *testing.T) {
	// Existing success behaviour must not change.
	gw := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: testGateway, Namespace: testNamespace, Generation: 2,
			Finalizers: []string{apigatewayFinalizerName},
		},
	}
	meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:               apiv1.GatewayConditionProgrammed,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: 2,
		Reason:             apiv1.GatewayProgrammedReasonProgrammed,
		LastTransitionTime: metav1.Now(),
	})
	r, calls := newRecoveryReconciler(t, gw)

	// The already-programmed path also re-registers the gateway, which needs the gateway's
	// Services to exist. That registration is unrelated to recovery, so its error is ignored
	// here; what matters is that no deployment is triggered and the tracker records Deployed.
	_, _ = callDecide(t, r, gw)

	if *calls != 0 {
		t.Errorf("deploy attempts = %d, want 0 for an already programmed gateway", *calls)
	}
	stored, ok := r.gatewayTracker.Get(testTrackKey)
	if !ok || stored.Status != GatewayTrackingStatusDeployed {
		t.Errorf("tracker = %v, want a Deployed entry", stored)
	}
}

func TestDecideAndProcess_newerGenerationStillDeploys(t *testing.T) {
	// A newer generation over a persisted failure must deploy, as before.
	gw := failedGateway(1)
	gw.Generation = 2
	r, calls := newRecoveryReconciler(t, gw)

	if _, err := callDecide(t, r, gw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 1 {
		t.Errorf("deploy attempts = %d, want 1 for a newer generation", *calls)
	}
}

func TestDecideAndProcess_processingGenerationIsNotRedeployed(t *testing.T) {
	gw := failedGateway(1)
	r, calls := newRecoveryReconciler(t, gw)
	r.gatewayTracker.Set(testTrackKey, &GatewayTrackingEntry{
		Generation: 1,
		Status:     GatewayTrackingStatusProcessing,
	})

	if _, err := callDecide(t, r, gw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 0 {
		t.Errorf("deploy attempts = %d, want 0 while a deployment is in progress", *calls)
	}
}

// --- pure helpers ------------------------------------------------------------------------

func TestDecideFailedRecovery(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		entry         *GatewayTrackingEntry
		inputHash     string
		wantDeploy    bool
		wantRequeueGT time.Duration
	}{
		{
			name:       "changed inputs deploy immediately",
			entry:      &GatewayTrackingEntry{InputHash: "old", NextRetryTime: now.Add(time.Hour)},
			inputHash:  "new",
			wantDeploy: true,
		},
		{
			name:       "window elapsed deploys",
			entry:      &GatewayTrackingEntry{InputHash: "same", NextRetryTime: now.Add(-time.Second)},
			inputHash:  "same",
			wantDeploy: true,
		},
		{
			name:       "exact boundary deploys",
			entry:      &GatewayTrackingEntry{InputHash: "same", NextRetryTime: now},
			inputHash:  "same",
			wantDeploy: true,
		},
		{
			name:       "zero next retry time deploys rather than waiting forever",
			entry:      &GatewayTrackingEntry{InputHash: "same"},
			inputHash:  "same",
			wantDeploy: true,
		},
		{
			name:          "inside the window waits",
			entry:         &GatewayTrackingEntry{InputHash: "same", NextRetryTime: now.Add(5 * time.Minute)},
			inputHash:     "same",
			wantDeploy:    false,
			wantRequeueGT: 4 * time.Minute,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &GatewayReconciler{Config: recoveryConfig()}
			deployNow, requeueAfter, reason := r.decideFailedRecovery(tc.entry, tc.inputHash, now)
			if deployNow != tc.wantDeploy {
				t.Errorf("deployNow = %v, want %v", deployNow, tc.wantDeploy)
			}
			if !tc.wantDeploy && requeueAfter <= tc.wantRequeueGT {
				t.Errorf("requeueAfter = %v, want > %v", requeueAfter, tc.wantRequeueGT)
			}
			if reason == "" {
				t.Error("expected a reason for the decision")
			}
		})
	}
}

func TestRetryCountFor_resetsWhenInputsChange(t *testing.T) {
	exhausted := &GatewayTrackingEntry{Generation: 1, RetryCount: 3, InputHash: "old"}

	if got := retryCountFor(exhausted, true, 1, "old"); got != 3 {
		t.Errorf("same generation and inputs: retryCount = %d, want 3 (budget preserved)", got)
	}
	if got := retryCountFor(exhausted, true, 1, "new"); got != 0 {
		t.Errorf("changed inputs: retryCount = %d, want 0 (budget reset)", got)
	}
	if got := retryCountFor(exhausted, true, 2, "old"); got != 0 {
		t.Errorf("new generation: retryCount = %d, want 0", got)
	}
	if got := retryCountFor(nil, false, 1, "old"); got != 0 {
		t.Errorf("no entry: retryCount = %d, want 0", got)
	}
	// Nil entry with hasExisting set must not panic.
	if got := retryCountFor(nil, true, 1, "old"); got != 0 {
		t.Errorf("nil entry: retryCount = %d, want 0", got)
	}
}

func TestDeploymentInputHash(t *testing.T) {
	base := deploymentInputHash("a: 1\n", "commonAnnotations:\n  x: y\n")

	if base != deploymentInputHash("a: 1\n", "commonAnnotations:\n  x: y\n") {
		t.Error("hash must be stable for identical inputs")
	}
	if base == deploymentInputHash("a: 2\n", "commonAnnotations:\n  x: y\n") {
		t.Error("ConfigMap content must affect the hash")
	}
	if base == deploymentInputHash("a: 1\n", "commonAnnotations:\n  x: z\n") {
		t.Error("deployment-affecting annotations must affect the hash")
	}
	// The separator must stop different splits from colliding.
	if deploymentInputHash("ab", "c") == deploymentInputHash("a", "bc") {
		t.Error("field boundaries must be unambiguous")
	}
}

func TestRetryWindow_fallsBackWhenUnset(t *testing.T) {
	r := &GatewayReconciler{Config: &config.OperatorConfig{}}
	if got := r.retryWindow(); got <= 0 {
		t.Errorf("retryWindow = %v, want a positive fallback", got)
	}
	// A nil config must not panic; recovery still needs a bounded window.
	rNil := &GatewayReconciler{}
	if got := rNil.retryWindow(); got <= 0 {
		t.Errorf("retryWindow with nil config = %v, want a positive fallback", got)
	}
	if got := r.retryWindow(); got != 10*time.Minute {
		t.Errorf("retryWindow fallback = %v, want 10m", got)
	}
	rSet := &GatewayReconciler{Config: recoveryConfig()}
	if got := rSet.retryWindow(); got != testSyncPeriod {
		t.Errorf("retryWindow = %v, want the configured sync period %v", got, testSyncPeriod)
	}
}

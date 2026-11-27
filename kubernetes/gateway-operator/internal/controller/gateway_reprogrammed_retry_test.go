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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	apiv2alpha1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v2alpha1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

const retryTestNS = "gw-retry"

func retryTestClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	require.NoError(t, gatewayv1.Install(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).
		WithStatusSubresource(&apiv1.RestApi{}, &apiv1.APIGateway{}).Build()
}

// programmedGateway returns an APIGateway whose Programmed condition became True at
// programmedAt. With adminCredentials pointing at a Secret that is never created, any
// call to it fails in the auth step, so no network request is attempted.
func programmedGateway(name string, programmedAt time.Time) *apiv1.APIGateway {
	gw := &apiv1.APIGateway{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: retryTestNS},
		Spec: apiv1.GatewaySpec{
			APISelector: apiv1.APISelector{Scope: apiv1.LabelSelectorScope, MatchLabels: map[string]string{"target": name}},
		},
		Status: apiv1.GatewayStatus{Conditions: []metav1.Condition{{
			Type: apiv1.GatewayConditionProgrammed, Status: metav1.ConditionTrue, Reason: "Programmed",
			LastTransitionTime: metav1.NewTime(programmedAt),
		}}},
	}
	apiv2alpha1.SetAdminCredentials(gw, &apiv2alpha1.GatewayAdminCredentials{
		Username: "admin",
		PasswordSecretRef: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name + "-admin-missing"},
			Key:                  "password",
		},
	})
	return gw
}

func registerRetryGateway(t *testing.T, gw *apiv1.APIGateway) *registry.GatewayInfo {
	t.Helper()
	info := &registry.GatewayInfo{
		Name: gw.Name, Namespace: gw.Namespace, APISelector: &gw.Spec.APISelector,
		ServiceName: gw.Name + "-controller", ServicePort: 9090,
	}
	registry.GetGatewayRegistry().Register(info)
	t.Cleanup(func() { registry.GetGatewayRegistry().Unregister(gw.Namespace, gw.Name) })
	return info
}

func TestGatewayReprogrammedSinceFailure(t *testing.T) {
	ctx := context.Background()
	failedAt := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))

	after := programmedGateway("after", failedAt.Add(30*time.Second))
	before := programmedGateway("before", failedAt.Add(-30*time.Second))
	notProgrammed := programmedGateway("down", failedAt.Add(30*time.Second))
	notProgrammed.Status.Conditions[0].Status = metav1.ConditionFalse
	k8sGW := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "k8s-gw", Namespace: retryTestNS},
		Status: gatewayv1.GatewayStatus{Conditions: []metav1.Condition{{
			Type: string(gatewayv1.GatewayConditionProgrammed), Status: metav1.ConditionTrue, Reason: "Programmed",
			LastTransitionTime: metav1.NewTime(failedAt.Add(30 * time.Second)),
		}}},
	}
	c := retryTestClient(t, after, before, notProgrammed, k8sGW)
	gwInfo := func(name string, fromGatewayAPI bool) *registry.GatewayInfo {
		return &registry.GatewayInfo{Name: name, Namespace: retryTestNS, FromGatewayAPI: fromGatewayAPI}
	}

	// Without an observation (tracker cleared) the failure time is the reference.
	none := gatewayObservation{}
	assert.True(t, gatewayReprogrammedSinceFailure(ctx, c, gwInfo("after", false), none, &failedAt), "gateway programmed after the failure")
	assert.False(t, gatewayReprogrammedSinceFailure(ctx, c, gwInfo("before", false), none, &failedAt), "gateway programmed before the failure")
	assert.False(t, gatewayReprogrammedSinceFailure(ctx, c, gwInfo("down", false), none, &failedAt), "gateway not programmed")
	assert.False(t, gatewayReprogrammedSinceFailure(ctx, c, gwInfo("missing", false), none, &failedAt), "gateway not found")
	assert.False(t, gatewayReprogrammedSinceFailure(ctx, c, gwInfo("after", false), none, nil), "no failure time")
	assert.True(t, gatewayReprogrammedSinceFailure(ctx, c, gwInfo("k8s-gw", true), none, &failedAt), "Gateway API gateway programmed after the failure")

	// With an observation, the transition the attempt saw is the reference, whatever
	// the failure time.
	seenBefore := gatewayObservation{Recorded: true, ProgrammedAt: failedAt.Add(-30 * time.Second)}
	seenNotProgrammed := gatewayObservation{Recorded: true}
	assert.True(t, gatewayReprogrammedSinceFailure(ctx, c, gwInfo("after", false), seenBefore, &failedAt), "gateway re-programmed since the attempt")
	assert.True(t, gatewayReprogrammedSinceFailure(ctx, c, gwInfo("after", false), seenNotProgrammed, &failedAt), "gateway was not programmed during the attempt")
	assert.False(t, gatewayReprogrammedSinceFailure(ctx, c, gwInfo("before", false), seenBefore, &failedAt), "same transition the attempt saw")
	assert.False(t, gatewayReprogrammedSinceFailure(ctx, c, gwInfo("down", false), seenNotProgrammed, &failedAt), "gateway still not programmed")
	recoveredBeforeStatusWrite := gatewayObservation{Recorded: true, ProgrammedAt: failedAt.Add(-90 * time.Second)}
	assert.True(t, gatewayReprogrammedSinceFailure(ctx, c, gwInfo("before", false), recoveredBeforeStatusWrite, &failedAt),
		"gateway recovered after the attempt started but before the failure status was written")

	assert.Equal(t, gatewayObservation{Recorded: true, ProgrammedAt: failedAt.Add(30 * time.Second)}, observeGateway(ctx, c, gwInfo("after", false)))
	assert.Equal(t, gatewayObservation{Recorded: true}, observeGateway(ctx, c, gwInfo("down", false)))
}

// terminallyFailedRestApi is a RestApi whose deployment ended in DeploymentFailed at its
// current generation, with status.lastUpdateTime = failedAt.
func terminallyFailedRestApi(target string, failedAt metav1.Time) *apiv1.RestApi {
	return &apiv1.RestApi{
		ObjectMeta: metav1.ObjectMeta{
			Name: "api-retry", Namespace: retryTestNS, Generation: 2,
			Labels: map[string]string{"target": target},
		},
		Spec: apiv1.APIConfigData{DisplayName: "api-retry", Version: "v1", Context: "/retry"},
		Status: apiv1.RestApiStatus{
			Conditions: []metav1.Condition{{
				Type: apiv1.APIConditionProgrammed, Status: metav1.ConditionFalse, ObservedGeneration: 2,
				Reason: apiv1.APIProgrammedReasonDeploymentFailed, Message: "Deployment failed: 401",
				LastTransitionTime: failedAt,
			}},
			LastUpdateTime: &failedAt,
		},
	}
}

func newRetryRestApiReconciler(c client.Client) *RestApiReconciler {
	return &RestApiReconciler{
		Client:     c,
		Config:     &config.OperatorConfig{},
		apiTracker: NewAPITracker(),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func decideRestApi(t *testing.T, r *RestApiReconciler, name string) *apiv1.RestApi {
	t.Helper()
	ctx := context.Background()
	api := &apiv1.RestApi{}
	require.NoError(t, r.Get(ctx, types.NamespacedName{Namespace: retryTestNS, Name: name}, api))
	cond := meta.FindStatusCondition(api.Status.Conditions, apiv1.APIConditionProgrammed)
	trackingKey := types.NamespacedName{Namespace: api.Namespace, Name: api.Name}.String()
	entry, has := r.apiTracker.Get(trackingKey)
	_, err := r.decideAndProcess(ctx, api, trackingKey, api.Generation, cond.ObservedGeneration, cond, entry, has)
	require.NoError(t, err)
	after := &apiv1.RestApi{}
	require.NoError(t, r.Get(ctx, types.NamespacedName{Namespace: retryTestNS, Name: name}, after))
	return after
}

// A RestApi that failed terminally is deployed again once its gateway is programmed
// after the failure (here the attempt fails again in the auth step, which moves it out
// of DeploymentFailed into Retrying and proves a new attempt ran).
func TestRestApiRetriesTerminalFailureAfterGatewayReprogrammed(t *testing.T) {
	failedAt := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))
	gw := programmedGateway("gw-after", failedAt.Add(30*time.Second))
	registerRetryGateway(t, gw)
	r := newRetryRestApiReconciler(retryTestClient(t, gw, terminallyFailedRestApi("gw-after", failedAt)))

	after := decideRestApi(t, r, "api-retry")
	cond := meta.FindStatusCondition(after.Status.Conditions, apiv1.APIConditionProgrammed)
	require.NotNil(t, cond)
	assert.Equal(t, apiv1.APIProgrammedReasonRetrying, cond.Reason, "a new deployment attempt must have run")
	require.NotNil(t, after.Status.LastUpdateTime)
	assert.True(t, after.Status.LastUpdateTime.After(failedAt.Time), "the attempt records a newer failure time, so the same gateway transition cannot retrigger it")
}

// Positive control's counterpart: when the gateway was programmed before the failure,
// nothing has changed since, so the terminal state is left alone.
func TestRestApiKeepsTerminalFailureWhenGatewayNotReprogrammed(t *testing.T) {
	failedAt := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))
	gw := programmedGateway("gw-before", failedAt.Add(-30*time.Second))
	registerRetryGateway(t, gw)
	r := newRetryRestApiReconciler(retryTestClient(t, gw, terminallyFailedRestApi("gw-before", failedAt)))

	after := decideRestApi(t, r, "api-retry")
	cond := meta.FindStatusCondition(after.Status.Conditions, apiv1.APIConditionProgrammed)
	require.NotNil(t, cond)
	assert.Equal(t, apiv1.APIProgrammedReasonDeploymentFailed, cond.Reason)
	assert.True(t, after.Status.LastUpdateTime.Equal(&failedAt))
}

// The gateway recovered after the failed attempt started but before its failure status
// was written, so its transition is older than status.lastUpdateTime. The attempt's
// own observation still shows it as re-programmed, so the deployment is retried, once.
func TestRestApiRetriesWhenGatewayRecoveredBeforeFailureWasWritten(t *testing.T) {
	failedAt := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))
	gw := programmedGateway("gw-race", failedAt.Add(-time.Second))
	registerRetryGateway(t, gw)
	r := newRetryRestApiReconciler(retryTestClient(t, gw, terminallyFailedRestApi("gw-race", failedAt)))
	trackingKey := types.NamespacedName{Namespace: retryTestNS, Name: "api-retry"}.String()
	r.apiTracker.Set(trackingKey, &APITrackingEntry{
		Generation: 2, Status: TrackingStatusDeployed,
		GatewayObserved: gatewayObservation{Recorded: true}, // gateway was not Programmed when the attempt started
	})

	after := decideRestApi(t, r, "api-retry")
	cond := meta.FindStatusCondition(after.Status.Conditions, apiv1.APIConditionProgrammed)
	require.NotNil(t, cond)
	assert.Equal(t, apiv1.APIProgrammedReasonRetrying, cond.Reason, "a new deployment attempt must have run")
	entry, ok := r.apiTracker.Get(trackingKey)
	require.True(t, ok)
	assert.Equal(t, gatewayObservation{Recorded: true, ProgrammedAt: failedAt.Add(-time.Second)}, entry.GatewayObserved,
		"the retried attempt records the transition it saw, so the same transition cannot retrigger it")
}

// Positive control's counterpart: the attempt already saw the gateway's current
// transition, so even though that transition is newer than the failure time nothing has
// changed and the terminal state is left alone.
func TestRestApiKeepsTerminalFailureWhenAttemptSawCurrentTransition(t *testing.T) {
	failedAt := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))
	programmedAt := failedAt.Add(30 * time.Second)
	gw := programmedGateway("gw-seen", programmedAt)
	registerRetryGateway(t, gw)
	r := newRetryRestApiReconciler(retryTestClient(t, gw, terminallyFailedRestApi("gw-seen", failedAt)))
	trackingKey := types.NamespacedName{Namespace: retryTestNS, Name: "api-retry"}.String()
	r.apiTracker.Set(trackingKey, &APITrackingEntry{
		Generation: 2, Status: TrackingStatusDeployed,
		GatewayObserved: gatewayObservation{Recorded: true, ProgrammedAt: programmedAt},
	})

	after := decideRestApi(t, r, "api-retry")
	cond := meta.FindStatusCondition(after.Status.Conditions, apiv1.APIConditionProgrammed)
	require.NotNil(t, cond)
	assert.Equal(t, apiv1.APIProgrammedReasonDeploymentFailed, cond.Reason)
	assert.True(t, after.Status.LastUpdateTime.Equal(&failedAt))
}

// A failed delete on a matched gateway is reported, so the finalizer stays and the
// deletion is retried; with no matched gateway there is nothing to clean up.
func TestCleanupAPIDeploymentsReportsGatewayFailures(t *testing.T) {
	gw := programmedGateway("gw-cleanup", time.Now())
	registerRetryGateway(t, gw)
	r := newRetryRestApiReconciler(retryTestClient(t, gw))

	api := terminallyFailedRestApi("gw-cleanup", metav1.Now())
	err := r.cleanupAPIDeployments(context.Background(), api)
	require.Error(t, err, "the admin Secret is missing, so the delete cannot be sent")
	assert.Contains(t, err.Error(), retryTestNS+"/gw-cleanup")

	unmatched := terminallyFailedRestApi("no-such-gateway", metav1.Now())
	require.NoError(t, r.cleanupAPIDeployments(context.Background(), unmatched))
}

func terminallyFailedMcp(target string, failedAt metav1.Time) *apiv1.Mcp {
	return &apiv1.Mcp{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mcp-retry", Namespace: retryTestNS, Generation: 3,
			Labels: map[string]string{"target": target},
		},
		Status: apiv1.ResourceStatus{
			Conditions: []metav1.Condition{
				{Type: apiv1.ConditionAccepted, Status: metav1.ConditionTrue, ObservedGeneration: 3, Reason: apiv1.ReasonAccepted, LastTransitionTime: failedAt},
				{Type: apiv1.ConditionProgrammed, Status: metav1.ConditionFalse, ObservedGeneration: 3, Reason: apiv1.ReasonDeploymentFailed,
					Message: "Deployment failed: 401", LastTransitionTime: failedAt},
			},
			LastUpdateTime: &failedAt,
		},
	}
}

func decideMcp(t *testing.T, r *McpReconciler) *apiv1.Mcp {
	t.Helper()
	ctx := context.Background()
	obj := &apiv1.Mcp{}
	key := types.NamespacedName{Namespace: retryTestNS, Name: "mcp-retry"}
	require.NoError(t, r.Get(ctx, key, obj))
	cond := meta.FindStatusCondition(obj.Status.Conditions, apiv1.ConditionProgrammed)
	trackingKey := key.String()
	entry, has := r.Tracker.Get(trackingKey)
	_, err := r.decideAndProcess(ctx, obj, trackingKey, obj.Generation, cond.ObservedGeneration, cond, entry, has)
	require.NoError(t, err)
	after := &apiv1.Mcp{}
	require.NoError(t, r.Get(ctx, key, after))
	return after
}

func newRetryMcpReconciler(t *testing.T, objs ...client.Object) *McpReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).
		WithStatusSubresource(&apiv1.Mcp{}, &apiv1.APIGateway{}).Build()
	return NewMcpReconciler(c, &config.OperatorConfig{}, slog.New(slog.NewTextHandler(io.Discard, nil)), NewResourceTracker())
}

// The generic reconciler (every kind other than RestApi) retries a terminal failure
// once its gateway is programmed after it: the attempt resets and rewrites the status,
// so lastUpdateTime moves past the failure.
func TestGenericRetriesTerminalFailureAfterGatewayReprogrammed(t *testing.T) {
	failedAt := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))
	gw := programmedGateway("gw-generic-after", failedAt.Add(30*time.Second))
	registerRetryGateway(t, gw)
	r := newRetryMcpReconciler(t, gw, terminallyFailedMcp("gw-generic-after", failedAt))

	after := decideMcp(t, r)
	require.NotNil(t, after.Status.LastUpdateTime)
	assert.True(t, after.Status.LastUpdateTime.After(failedAt.Time), "a new deployment attempt must have run")
	cond := meta.FindStatusCondition(after.Status.Conditions, apiv1.ConditionProgrammed)
	require.NotNil(t, cond)
	assert.NotEqual(t, "Deployment failed: 401", cond.Message)
}

func TestGenericKeepsTerminalFailureWhenGatewayNotReprogrammed(t *testing.T) {
	failedAt := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))
	gw := programmedGateway("gw-generic-before", failedAt.Add(-30*time.Second))
	registerRetryGateway(t, gw)
	r := newRetryMcpReconciler(t, gw, terminallyFailedMcp("gw-generic-before", failedAt))

	after := decideMcp(t, r)
	assert.True(t, after.Status.LastUpdateTime.Equal(&failedAt))
	cond := meta.FindStatusCondition(after.Status.Conditions, apiv1.ConditionProgrammed)
	require.NotNil(t, cond)
	assert.Equal(t, apiv1.ReasonDeploymentFailed, cond.Reason)
	assert.Equal(t, "Deployment failed: 401", cond.Message)
}

// Same race as TestRestApiRetriesWhenGatewayRecoveredBeforeFailureWasWritten, through the
// generic reconciler, with its positive control: once the retry has recorded the
// gateway's current transition, a later reconcile does not retry again.
func TestGenericRetriesWhenGatewayRecoveredBeforeFailureWasWritten(t *testing.T) {
	failedAt := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))
	gw := programmedGateway("gw-generic-race", failedAt.Add(-time.Second))
	registerRetryGateway(t, gw)
	r := newRetryMcpReconciler(t, gw, terminallyFailedMcp("gw-generic-race", failedAt))
	trackingKey := types.NamespacedName{Namespace: retryTestNS, Name: "mcp-retry"}.String()
	r.Tracker.Set(trackingKey, &ResourceTrackingEntry{
		Generation: 3, Status: ResourceStatusDeployed,
		GatewayObserved: gatewayObservation{Recorded: true},
	})

	after := decideMcp(t, r)
	require.NotNil(t, after.Status.LastUpdateTime)
	assert.True(t, after.Status.LastUpdateTime.After(failedAt.Time), "a new deployment attempt must have run")
	entry, ok := r.Tracker.Get(trackingKey)
	require.True(t, ok)
	assert.Equal(t, gatewayObservation{Recorded: true, ProgrammedAt: failedAt.Add(-time.Second)}, entry.GatewayObserved)

	// Make the resource terminally failed again at the retried attempt's observation:
	// with no new gateway transition, it stays failed.
	failedAgain := terminallyFailedMcp("gw-generic-race", metav1.NewTime(failedAt.Add(-2*time.Second)))
	current := &apiv1.Mcp{}
	require.NoError(t, r.Get(context.Background(), types.NamespacedName{Namespace: retryTestNS, Name: "mcp-retry"}, current))
	current.Status = failedAgain.Status
	require.NoError(t, r.Status().Update(context.Background(), current))
	entry.Status = ResourceStatusDeployed
	entry.NextRetryTime = time.Time{}
	r.Tracker.Set(trackingKey, entry)

	again := decideMcp(t, r)
	cond := meta.FindStatusCondition(again.Status.Conditions, apiv1.ConditionProgrammed)
	require.NotNil(t, cond)
	assert.Equal(t, apiv1.ReasonDeploymentFailed, cond.Reason, "the same gateway transition must not retrigger a retry")
	assert.Equal(t, "Deployment failed: 401", cond.Message)
}

// failFirstStatusWrite makes the first status write (patch or update) fail with a
// transient error, the way a conflicting or unavailable API server would.
func failFirstStatusWrite() interceptor.Funcs {
	failed := false
	fail := func() error {
		if failed {
			return nil
		}
		failed = true
		return apierrors.NewServiceUnavailable("transient: status write failed")
	}
	return interceptor.Funcs{
		SubResourcePatch: func(ctx context.Context, c client.Client, sub string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			if err := fail(); err != nil {
				return err
			}
			return c.SubResource(sub).Patch(ctx, obj, patch, opts...)
		},
		SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			if err := fail(); err != nil {
				return err
			}
			return c.SubResource(sub).Update(ctx, obj, opts...)
		},
	}
}

// A recovery retry whose initial status write fails has not called the gateway, so it
// must not consume the gateway transition: the next reconcile retries again.
func TestRestApiRecoveryRetrySurvivesFailedInitialStatusWrite(t *testing.T) {
	failedAt := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))
	gw := programmedGateway("gw-status-fail", failedAt.Add(30*time.Second))
	registerRetryGateway(t, gw)
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(gw, terminallyFailedRestApi("gw-status-fail", failedAt)).
		WithStatusSubresource(&apiv1.RestApi{}, &apiv1.APIGateway{}).
		WithInterceptorFuncs(failFirstStatusWrite()).Build()
	r := newRetryRestApiReconciler(c)
	ctx := context.Background()
	key := types.NamespacedName{Namespace: retryTestNS, Name: "api-retry"}
	trackingKey := key.String()

	api := &apiv1.RestApi{}
	require.NoError(t, r.Get(ctx, key, api))
	cond := meta.FindStatusCondition(api.Status.Conditions, apiv1.APIConditionProgrammed)
	_, err := r.decideAndProcess(ctx, api, trackingKey, api.Generation, cond.ObservedGeneration, cond, nil, false)
	require.Error(t, err, "the initial status write fails")
	entry, ok := r.apiTracker.Get(trackingKey)
	require.True(t, ok)
	assert.False(t, entry.GatewayObserved.Recorded, "an attempt that never called the gateway records no observation")

	after := decideRestApi(t, r, "api-retry")
	cond = meta.FindStatusCondition(after.Status.Conditions, apiv1.APIConditionProgrammed)
	require.NotNil(t, cond)
	assert.Equal(t, apiv1.APIProgrammedReasonRetrying, cond.Reason, "the retry must run once the status write succeeds")
}

// Same as TestRestApiRecoveryRetrySurvivesFailedInitialStatusWrite, through the generic
// reconciler.
func TestGenericRecoveryRetrySurvivesFailedInitialStatusWrite(t *testing.T) {
	failedAt := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))
	gw := programmedGateway("gw-generic-status-fail", failedAt.Add(30*time.Second))
	registerRetryGateway(t, gw)
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(gw, terminallyFailedMcp("gw-generic-status-fail", failedAt)).
		WithStatusSubresource(&apiv1.Mcp{}, &apiv1.APIGateway{}).
		WithInterceptorFuncs(failFirstStatusWrite()).Build()
	r := NewMcpReconciler(c, &config.OperatorConfig{}, slog.New(slog.NewTextHandler(io.Discard, nil)), NewResourceTracker())
	ctx := context.Background()
	key := types.NamespacedName{Namespace: retryTestNS, Name: "mcp-retry"}
	trackingKey := key.String()

	obj := &apiv1.Mcp{}
	require.NoError(t, r.Get(ctx, key, obj))
	cond := meta.FindStatusCondition(obj.Status.Conditions, apiv1.ConditionProgrammed)
	_, err := r.decideAndProcess(ctx, obj, trackingKey, obj.Generation, cond.ObservedGeneration, cond, nil, false)
	require.Error(t, err, "the initial status write fails")
	entry, ok := r.Tracker.Get(trackingKey)
	require.True(t, ok)
	assert.False(t, entry.GatewayObserved.Recorded, "an attempt that never called the gateway records no observation")

	after := decideMcp(t, r)
	require.NotNil(t, after.Status.LastUpdateTime)
	assert.True(t, after.Status.LastUpdateTime.After(failedAt.Time), "the retry must run once the status write succeeds")
	cond = meta.FindStatusCondition(after.Status.Conditions, apiv1.ConditionProgrammed)
	require.NotNil(t, cond)
	assert.NotEqual(t, "Deployment failed: 401", cond.Message)
}

// The gateway recovered during the failed attempt, before its failure status was written,
// so its transition is older than status.lastUpdateTime and only the attempt's own
// observation shows it. A recovery retry whose initial status write fails must keep that
// observation, or the next reconcile falls back to status.lastUpdateTime and never retries.
func TestRestApiRecoveryRetryKeepsObservationAfterFailedInitialStatusWrite(t *testing.T) {
	failedAt := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))
	gw := programmedGateway("gw-status-fail-race", failedAt.Add(-time.Second))
	registerRetryGateway(t, gw)
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(gw, terminallyFailedRestApi("gw-status-fail-race", failedAt)).
		WithStatusSubresource(&apiv1.RestApi{}, &apiv1.APIGateway{}).
		WithInterceptorFuncs(failFirstStatusWrite()).Build()
	r := newRetryRestApiReconciler(c)
	ctx := context.Background()
	key := types.NamespacedName{Namespace: retryTestNS, Name: "api-retry"}
	trackingKey := key.String()
	failedAttempt := gatewayObservation{Recorded: true} // gateway was not Programmed when the failed attempt called it
	r.apiTracker.Set(trackingKey, &APITrackingEntry{Generation: 2, Status: TrackingStatusDeployed, GatewayObserved: failedAttempt})

	api := &apiv1.RestApi{}
	require.NoError(t, r.Get(ctx, key, api))
	cond := meta.FindStatusCondition(api.Status.Conditions, apiv1.APIConditionProgrammed)
	seeded, has := r.apiTracker.Get(trackingKey)
	_, err := r.decideAndProcess(ctx, api, trackingKey, api.Generation, cond.ObservedGeneration, cond, seeded, has)
	require.Error(t, err, "the initial status write fails")
	entry, ok := r.apiTracker.Get(trackingKey)
	require.True(t, ok)
	assert.Equal(t, failedAttempt, entry.GatewayObserved, "the retry keeps the failed attempt's observation until it calls the gateway")

	after := decideRestApi(t, r, "api-retry")
	cond = meta.FindStatusCondition(after.Status.Conditions, apiv1.APIConditionProgrammed)
	require.NotNil(t, cond)
	assert.Equal(t, apiv1.APIProgrammedReasonRetrying, cond.Reason, "the retry must run once the status write succeeds")
	entry, ok = r.apiTracker.Get(trackingKey)
	require.True(t, ok)
	assert.Equal(t, gatewayObservation{Recorded: true, ProgrammedAt: failedAt.Add(-time.Second)}, entry.GatewayObserved,
		"the attempt that called the gateway records the transition it saw")
}

// Same as TestRestApiRecoveryRetryKeepsObservationAfterFailedInitialStatusWrite, through
// the generic reconciler.
func TestGenericRecoveryRetryKeepsObservationAfterFailedInitialStatusWrite(t *testing.T) {
	failedAt := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))
	gw := programmedGateway("gw-generic-status-fail-race", failedAt.Add(-time.Second))
	registerRetryGateway(t, gw)
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(gw, terminallyFailedMcp("gw-generic-status-fail-race", failedAt)).
		WithStatusSubresource(&apiv1.Mcp{}, &apiv1.APIGateway{}).
		WithInterceptorFuncs(failFirstStatusWrite()).Build()
	r := NewMcpReconciler(c, &config.OperatorConfig{}, slog.New(slog.NewTextHandler(io.Discard, nil)), NewResourceTracker())
	ctx := context.Background()
	key := types.NamespacedName{Namespace: retryTestNS, Name: "mcp-retry"}
	trackingKey := key.String()
	failedAttempt := gatewayObservation{Recorded: true}
	r.Tracker.Set(trackingKey, &ResourceTrackingEntry{Generation: 3, Status: ResourceStatusDeployed, GatewayObserved: failedAttempt})

	obj := &apiv1.Mcp{}
	require.NoError(t, r.Get(ctx, key, obj))
	cond := meta.FindStatusCondition(obj.Status.Conditions, apiv1.ConditionProgrammed)
	seeded, has := r.Tracker.Get(trackingKey)
	_, err := r.decideAndProcess(ctx, obj, trackingKey, obj.Generation, cond.ObservedGeneration, cond, seeded, has)
	require.Error(t, err, "the initial status write fails")
	entry, ok := r.Tracker.Get(trackingKey)
	require.True(t, ok)
	assert.Equal(t, failedAttempt, entry.GatewayObserved, "the retry keeps the failed attempt's observation until it calls the gateway")

	after := decideMcp(t, r)
	require.NotNil(t, after.Status.LastUpdateTime)
	assert.True(t, after.Status.LastUpdateTime.After(failedAt.Time), "the retry must run once the status write succeeds")
	entry, ok = r.Tracker.Get(trackingKey)
	require.True(t, ok)
	assert.Equal(t, gatewayObservation{Recorded: true, ProgrammedAt: failedAt.Add(-time.Second)}, entry.GatewayObserved)
}

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
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

const recordTestNS = "deployed-record-test"

func recordTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(s))
	utilruntime.Must(apiv1.AddToScheme(s))
	utilruntime.Must(gatewayv1.AddToScheme(s))
	return s
}

func recordTestClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().WithScheme(recordTestScheme(t)).
		WithStatusSubresource(&apiv1.RestApi{}, &apiv1.ManagedSecret{}).
		WithObjects(objs...).Build()
}

func recordTestConfig() *config.OperatorConfig {
	cfg := &config.OperatorConfig{}
	cfg.GatewayAPI.GatewayClassNames = []string{"platform-gw"}
	return cfg
}

// registerRecordTestGateway registers a gateway the way the gateway controllers do once its
// runtime is up, and returns its endpoint.
func registerRecordTestGateway(t *testing.T, name string, selector *apiv1.APISelector, fromGatewayAPI bool) string {
	t.Helper()
	info := &registry.GatewayInfo{
		Name: name, Namespace: recordTestNS, ServiceName: name + "-controller", ServicePort: 9090,
		APISelector: selector, FromGatewayAPI: fromGatewayAPI,
	}
	registry.GetGatewayRegistry().Register(info)
	t.Cleanup(func() { registry.GetGatewayRegistry().Unregister(recordTestNS, name) })
	return info.GetGatewayServiceEndpoint()
}

func recordTestAPIGateway(name string) *apiv1.APIGateway {
	return &apiv1.APIGateway{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: recordTestNS}}
}

func recordAnnotation(entries map[string]string) map[string]string {
	raw, _ := json.Marshal(entries)
	return map[string]string{AnnDeployedToGateways: string(raw)}
}

func deletingRestAPI(name string, ann map[string]string) *apiv1.RestApi {
	now := metav1.Now()
	return &apiv1.RestApi{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: recordTestNS,
			Finalizers:        []string{apiFinalizerName},
			DeletionTimestamp: &now,
			Annotations:       ann,
		},
		Spec: apiv1.APIConfigData{Context: "/" + name, DisplayName: name, Version: "v1"},
	}
}

func newRecordTestRestApiReconciler(cl client.Client) (*RestApiReconciler, *fakeGatewayREST) {
	r := NewRestApiReconciler(cl, cl.Scheme(), recordTestConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	rest := newFakeGatewayREST()
	r.restAPI = rest
	return r, rest
}

func reconcileRestAPI(t *testing.T, r *RestApiReconciler, name string) (reconcile.Result, error) {
	t.Helper()
	return r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKey{Namespace: recordTestNS, Name: name}})
}

func restAPIGone(t *testing.T, cl client.Client, name string) bool {
	t.Helper()
	err := cl.Get(context.Background(), client.ObjectKey{Namespace: recordTestNS, Name: name}, &apiv1.RestApi{})
	if apierrors.IsNotFound(err) {
		return true
	}
	require.NoError(t, err)
	return false
}

// A RestApi deleted after an operator restart, before its gateway has registered again, keeps its
// finalizer until the gateway registers, and is then removed from it.
func TestRestApiDeletion_AfterRestart_WaitsForGatewayRegistration(t *testing.T) {
	const gw = "gw-restart-restapi"
	api := deletingRestAPI("orders", recordAnnotation(map[string]string{recordTestNS + "/" + gw: gatewayKindAPIGateway}))
	cl := recordTestClient(t, api, recordTestAPIGateway(gw))
	r, rest := newRecordTestRestApiReconciler(cl)

	// The registry is empty, as right after a restart.
	_, registered := registry.GetGatewayRegistry().Get(recordTestNS, gw)
	require.False(t, registered)

	res, err := reconcileRestAPI(t, r, "orders")
	require.NoError(t, err)
	require.Equal(t, requeueGatewayNotRegistered, res.RequeueAfter)
	require.False(t, restAPIGone(t, cl, "orders"), "finalizer must stay while the gateway is unregistered")
	require.Empty(t, rest.calls)

	// Still waiting on the next pass.
	res, err = reconcileRestAPI(t, r, "orders")
	require.NoError(t, err)
	require.Equal(t, requeueGatewayNotRegistered, res.RequeueAfter)
	require.False(t, restAPIGone(t, cl, "orders"))

	ep := registerRecordTestGateway(t, gw, nil, false)
	res, err = reconcileRestAPI(t, r, "orders")
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.Equal(t, 1, rest.count("delete", ep))
	require.True(t, restAPIGone(t, cl, "orders"))
}

func TestRestApiDeletion_CleanupErrorKeepsFinalizer(t *testing.T) {
	const gw = "gw-error-restapi"
	api := deletingRestAPI("payments", recordAnnotation(map[string]string{recordTestNS + "/" + gw: gatewayKindAPIGateway}))
	cl := recordTestClient(t, api, recordTestAPIGateway(gw))
	r, rest := newRecordTestRestApiReconciler(cl)
	ep := registerRecordTestGateway(t, gw, nil, false)
	rest.failDelete[ep] = errors.New("gateway returned status 503")

	_, err := reconcileRestAPI(t, r, "payments")
	require.Error(t, err)
	require.False(t, restAPIGone(t, cl, "payments"))

	delete(rest.failDelete, ep)
	_, err = reconcileRestAPI(t, r, "payments")
	require.NoError(t, err)
	require.True(t, restAPIGone(t, cl, "payments"))
}

func TestRestApiDeletion_NoRecordDeletesImmediately(t *testing.T) {
	cl := recordTestClient(t, deletingRestAPI("legacy", nil))
	r, rest := newRecordTestRestApiReconciler(cl)

	res, err := reconcileRestAPI(t, r, "legacy")
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.Empty(t, rest.calls)
	require.True(t, restAPIGone(t, cl, "legacy"))
}

func TestRestApiDeletion_RecordedGatewayGoneDeletes(t *testing.T) {
	api := deletingRestAPI("orphan", recordAnnotation(map[string]string{
		recordTestNS + "/gw-removed-apigw": gatewayKindAPIGateway,
		recordTestNS + "/gw-removed-k8s":   gatewayKindGatewayAPI,
	}))
	cl := recordTestClient(t, api)
	r, rest := newRecordTestRestApiReconciler(cl)

	res, err := reconcileRestAPI(t, r, "orphan")
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.Empty(t, rest.calls)
	require.True(t, restAPIGone(t, cl, "orphan"))
}

// A Gateway API Gateway of a managed class registers once its runtime is up, so deletion waits for
// it; one of another class never registers, so deletion does not wait.
func TestRestApiDeletion_GatewayAPIGatewayWaitsOnlyForManagedClass(t *testing.T) {
	managed := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-managed-k8s", Namespace: recordTestNS},
		Spec:       gatewayv1.GatewaySpec{GatewayClassName: "platform-gw"},
	}
	other := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-other-k8s", Namespace: recordTestNS},
		Spec:       gatewayv1.GatewaySpec{GatewayClassName: "someone-else"},
	}
	cl := recordTestClient(t, managed, other,
		deletingRestAPI("on-managed", recordAnnotation(map[string]string{recordTestNS + "/gw-managed-k8s": gatewayKindGatewayAPI})),
		deletingRestAPI("on-other", recordAnnotation(map[string]string{recordTestNS + "/gw-other-k8s": gatewayKindGatewayAPI})),
	)
	r, _ := newRecordTestRestApiReconciler(cl)

	res, err := reconcileRestAPI(t, r, "on-managed")
	require.NoError(t, err)
	require.Equal(t, requeueGatewayNotRegistered, res.RequeueAfter)
	require.False(t, restAPIGone(t, cl, "on-managed"))

	res, err = reconcileRestAPI(t, r, "on-other")
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.True(t, restAPIGone(t, cl, "on-other"))
}

// A deploy records its gateway on the RestApi, and the record is not forwarded to the gateway.
func TestRestApiDeploy_RecordsGateway(t *testing.T) {
	const gw = "gw-deploy-restapi"
	ns := recordTestNS
	api := &apiv1.RestApi{
		ObjectMeta: metav1.ObjectMeta{Name: "catalog", Namespace: ns, Finalizers: []string{apiFinalizerName}, Generation: 1},
		Spec:       apiv1.APIConfigData{Context: "/catalog", DisplayName: "catalog", Version: "v1"},
	}
	cl := recordTestClient(t, api, recordTestAPIGateway(gw))
	r, rest := newRecordTestRestApiReconciler(cl)
	ep := registerRecordTestGateway(t, gw, &apiv1.APISelector{Scope: apiv1.NamespacedScope, Namespaces: []string{ns}}, false)

	_, err := reconcileRestAPI(t, r, "catalog")
	require.NoError(t, err)
	require.True(t, rest.has(ep, "catalog"))

	got := &apiv1.RestApi{}
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: "catalog"}, got))
	require.Equal(t, gatewayRecord{ns + "/" + gw: gatewayKindAPIGateway}, readGatewayRecord(got))
	require.NotContains(t, string(rest.apis[ep]["catalog"]), AnnDeployedToGateways)
}

// stubSecretAdapter is the ManagedSecret adapter with the gateway calls replaced.
type stubSecretAdapter struct {
	managedSecretAdapter
	deployed  []string
	deleted   []string
	deleteErr error
}

func (a *stubSecretAdapter) Deploy(_ context.Context, _ client.Client, ep string, _ client.Object, _ gatewayclient.AuthHeaderFunc) (DeployResult, error) {
	a.deployed = append(a.deployed, ep)
	return DeployResult{}, nil
}

func (a *stubSecretAdapter) Delete(_ context.Context, ep string, _ client.Object, _ gatewayclient.AuthHeaderFunc) error {
	a.deleted = append(a.deleted, ep)
	return a.deleteErr
}

func newRecordTestGenericReconciler(cl client.Client) (*GenericReconciler, *stubSecretAdapter) {
	adapter := &stubSecretAdapter{}
	return &GenericReconciler{
		Client:  cl,
		Adapter: adapter,
		Config:  recordTestConfig(),
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Tracker: NewResourceTracker(),
	}, adapter
}

func deletingManagedSecret(name string, ann map[string]string) *apiv1.ManagedSecret {
	now := metav1.Now()
	return &apiv1.ManagedSecret{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: recordTestNS,
		Finalizers:        []string{managedSecretFinalizer},
		DeletionTimestamp: &now,
		Annotations:       ann,
	}}
}

func reconcileGeneric(t *testing.T, r *GenericReconciler, name string) (reconcile.Result, error) {
	t.Helper()
	return r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKey{Namespace: recordTestNS, Name: name}})
}

func managedSecretGone(t *testing.T, cl client.Client, name string) bool {
	t.Helper()
	err := cl.Get(context.Background(), client.ObjectKey{Namespace: recordTestNS, Name: name}, &apiv1.ManagedSecret{})
	if apierrors.IsNotFound(err) {
		return true
	}
	require.NoError(t, err)
	return false
}

func TestGenericDeletion_AfterRestart_WaitsForGatewayRegistration(t *testing.T) {
	const gw = "gw-restart-generic"
	cl := recordTestClient(t, recordTestAPIGateway(gw),
		deletingManagedSecret("db-password", recordAnnotation(map[string]string{recordTestNS + "/" + gw: gatewayKindAPIGateway})))
	r, adapter := newRecordTestGenericReconciler(cl)

	res, err := reconcileGeneric(t, r, "db-password")
	require.NoError(t, err)
	require.Equal(t, requeueGatewayNotRegistered, res.RequeueAfter)
	require.False(t, managedSecretGone(t, cl, "db-password"))
	require.Empty(t, adapter.deleted)

	ep := registerRecordTestGateway(t, gw, nil, false)
	adapter.deleteErr = errors.New("gateway returned status 500")
	_, err = reconcileGeneric(t, r, "db-password")
	require.Error(t, err)
	require.False(t, managedSecretGone(t, cl, "db-password"), "a cleanup error must keep the finalizer")

	adapter.deleteErr = nil
	res, err = reconcileGeneric(t, r, "db-password")
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.Equal(t, []string{ep, ep}, adapter.deleted)
	require.True(t, managedSecretGone(t, cl, "db-password"))
}

func TestGenericDeletion_NoRecordDeletesImmediately(t *testing.T) {
	cl := recordTestClient(t, deletingManagedSecret("plain", nil))
	r, adapter := newRecordTestGenericReconciler(cl)

	res, err := reconcileGeneric(t, r, "plain")
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.Empty(t, adapter.deleted)
	require.True(t, managedSecretGone(t, cl, "plain"))
}

func TestGenericDeletion_RecordedGatewayGoneDeletes(t *testing.T) {
	cl := recordTestClient(t,
		deletingManagedSecret("orphan", recordAnnotation(map[string]string{recordTestNS + "/gw-removed-generic": gatewayKindAPIGateway})))
	r, adapter := newRecordTestGenericReconciler(cl)

	res, err := reconcileGeneric(t, r, "orphan")
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.Empty(t, adapter.deleted)
	require.True(t, managedSecretGone(t, cl, "orphan"))
}

func TestGenericDeploy_RecordsGateway(t *testing.T) {
	const gw = "gw-deploy-generic"
	ns := recordTestNS
	secret := &apiv1.ManagedSecret{ObjectMeta: metav1.ObjectMeta{
		Name: "api-token", Namespace: ns, Finalizers: []string{managedSecretFinalizer}, Generation: 1,
	}}
	cl := recordTestClient(t, secret, recordTestAPIGateway(gw))
	r, adapter := newRecordTestGenericReconciler(cl)
	ep := registerRecordTestGateway(t, gw, &apiv1.APISelector{Scope: apiv1.NamespacedScope, Namespaces: []string{ns}}, false)

	_, err := reconcileGeneric(t, r, "api-token")
	require.NoError(t, err)
	require.Equal(t, []string{ep}, adapter.deployed)

	got := &apiv1.ManagedSecret{}
	require.NoError(t, cl.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: "api-token"}, got))
	require.Equal(t, gatewayRecord{ns + "/" + gw: gatewayKindAPIGateway}, readGatewayRecord(got))
}

func TestPayloadAnnotations_DropsDeploymentRecord(t *testing.T) {
	in := map[string]string{AnnDeployedToGateways: `{"ns/gw":"APIGateway"}`, AnnProjectID: "p1"}
	require.Equal(t, map[string]string{AnnProjectID: "p1"}, payloadAnnotations(in))
	require.Len(t, in, 2, "input must not be modified")
	plain := map[string]string{AnnProjectID: "p1"}
	require.Equal(t, plain, payloadAnnotations(plain))
	require.Nil(t, payloadAnnotations(nil))
}

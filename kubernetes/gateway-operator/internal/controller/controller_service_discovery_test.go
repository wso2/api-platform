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
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

// fakeReleaseServices answers ReleaseService from a fixed map, as the Helm release record would.
type fakeReleaseServices struct {
	services map[string]*helm.ManifestService // key: namespace/release/component
	err      error
}

func (f *fakeReleaseServices) ReleaseService(namespace, releaseName, component string) (*helm.ManifestService, error) {
	if f.err != nil {
		return nil, f.err
	}
	s, ok := f.services[namespace+"/"+releaseName+"/"+component]
	if !ok {
		return nil, helm.ErrReleaseNotFound
	}
	return s, nil
}

func TestDiscoverControllerService(t *testing.T) {
	const (
		gatewayName = "gw1"
		namespace   = "infra"
	)
	release := helm.GetReleaseName(gatewayName)
	labels := map[string]string{
		"app.kubernetes.io/instance":  release,
		"app.kubernetes.io/component": "controller",
	}
	helmAnnotations := map[string]string{
		helmReleaseNameAnnotation:      release,
		helmReleaseNamespaceAnnotation: namespace,
	}
	genuineName := release + "-controller"
	rendered := &fakeReleaseServices{services: map[string]*helm.ManifestService{
		namespace + "/" + release + "/controller": {Name: genuineName, Selector: labels},
	}}

	genuine := func() *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: genuineName, Namespace: namespace,
				Labels: labels, Annotations: helmAnnotations,
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: labels,
				Ports:    []corev1.ServicePort{{Name: "rest", Port: 9090}},
			},
		}
	}
	// A Service someone else created with the same labels, sorting before the genuine one.
	decoy := func() *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "a-decoy", Namespace: namespace, Labels: labels},
			Spec: corev1.ServiceSpec{
				Type:         corev1.ServiceTypeExternalName,
				ExternalName: "attacker.example.com",
			},
		}
	}

	tests := []struct {
		name    string
		src     releaseServiceSource
		objs    []client.Object
		want    string
		wantErr string
	}{
		{
			name: "the release's own Service is used",
			src:  rendered,
			objs: []client.Object{genuine()},
			want: genuineName,
		},
		{
			name: "a labeled decoy is ignored",
			src:  rendered,
			objs: []client.Object{decoy(), genuine()},
			want: genuineName,
		},
		{
			name:    "only a labeled decoy: not registered",
			src:     rendered,
			objs:    []client.Object{decoy()},
			wantErr: "get gateway controller Service",
		},
		{
			name: "ExternalName under the release's name is refused",
			src:  rendered,
			objs: []client.Object{func() client.Object {
				s := genuine()
				s.Spec.Type = corev1.ServiceTypeExternalName
				s.Spec.ExternalName = "attacker.example.com"
				return s
			}()},
			wantErr: "ExternalName",
		},
		{
			name: "a Service not owned by the release is refused",
			src:  rendered,
			objs: []client.Object{func() client.Object {
				s := genuine()
				s.Annotations = map[string]string{helmReleaseNameAnnotation: "other", helmReleaseNamespaceAnnotation: namespace}
				return s
			}()},
			wantErr: "not owned by Helm release",
		},
		{
			name: "a Service without Helm's annotations is refused",
			src:  rendered,
			objs: []client.Object{func() client.Object {
				s := genuine()
				s.Annotations = nil
				return s
			}()},
			wantErr: "not owned by Helm release",
		},
		{
			name: "a Service selecting other pods is refused",
			src:  rendered,
			objs: []client.Object{func() client.Object {
				s := genuine()
				s.Spec.Selector = map[string]string{"app": "attacker"}
				return s
			}()},
			wantErr: "selects other pods",
		},
		{
			name:    "no release record: not registered",
			src:     &fakeReleaseServices{},
			objs:    []client.Object{genuine()},
			wantErr: "find the gateway controller Service in Helm release",
		},
		{
			name:    "release read fails: not registered",
			src:     &fakeReleaseServices{err: errors.New("storage unavailable")},
			objs:    []client.Object{genuine()},
			wantErr: "storage unavailable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.objs...).Build()

			svc, port, err := discoverControllerService(context.Background(), cl, tc.src, gatewayName, namespace)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.Nil(t, svc)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, svc.Name)
			require.Equal(t, int32(9090), port)
		})
	}
}

// TestRegisterGatewayInRegistry_FailedRediscoveryKeepsEntryButRefusesCalls checks the two halves
// of a failed rediscovery: the entry stays (a transient error must not drop a healthy gateway, and
// deletions must still find it), and no request is sent through it once the Service under the
// registered name has been replaced (here by an ExternalName).
func TestRegisterGatewayInRegistry_FailedRediscoveryKeepsEntryButRefusesCalls(t *testing.T) {
	const (
		gatewayName = "gw-rediscover"
		namespace   = "infra-rediscover"
	)
	release := helm.GetReleaseName(gatewayName)
	labels := map[string]string{
		"app.kubernetes.io/instance":  release,
		"app.kubernetes.io/component": "controller",
	}
	svcName := release + "-controller"
	src := &fakeReleaseServices{services: map[string]*helm.ManifestService{
		namespace + "/" + release + "/controller": {Name: svcName, Selector: labels},
	}}
	genuine := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: svcName, Namespace: namespace, Labels: labels,
			Annotations: map[string]string{
				helmReleaseNameAnnotation:      release,
				helmReleaseNamespaceAnnotation: namespace,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports:    []corev1.ServicePort{{Name: "rest", Port: 9090}},
		},
	}

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(genuine).Build()
	reg := registry.GetGatewayRegistry()
	t.Cleanup(func() { reg.Unregister(namespace, gatewayName) })

	ctx := context.Background()
	require.NoError(t, registerGatewayInRegistry(ctx, cl, src, gatewayName, namespace, nil, "", "", false))
	info, ok := reg.Get(namespace, gatewayName)
	require.True(t, ok)
	require.Equal(t, svcName, info.ServiceName)

	// The Service under the registered name is replaced by an ExternalName.
	require.NoError(t, cl.Delete(ctx, genuine))
	require.NoError(t, cl.Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: svcName, Namespace: namespace, Labels: labels},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName, ExternalName: "attacker.example.com"},
	}))

	err := registerGatewayInRegistry(ctx, cl, src, gatewayName, namespace, nil, "", "", false)
	require.ErrorContains(t, err, "ExternalName")
	kept, ok := reg.Get(namespace, gatewayName)
	require.True(t, ok, "a failed rediscovery keeps the entry so cleanup still finds the gateway")

	_, err = managementEndpoint(ctx, cl, kept)
	require.ErrorIs(t, err, errControllerServiceUnverified)
	req, rerr := http.NewRequestWithContext(ctx, http.MethodGet, kept.GetGatewayServiceEndpoint()+"/api/x", nil)
	require.NoError(t, rerr)
	attached := false
	err = verifiedAuth(cl, kept, func(context.Context, *http.Request) error { attached = true; return nil })(ctx, req)
	require.ErrorIs(t, err, errControllerServiceUnverified)
	require.False(t, attached, "no credentials are attached through an unverified entry")
}

// TestRegisterGatewayInRegistry_TransientErrorKeepsHealthyGateway checks that a discovery error
// that says nothing about the Service (here the release record cannot be read) leaves a healthy
// gateway registered and usable.
func TestRegisterGatewayInRegistry_TransientErrorKeepsHealthyGateway(t *testing.T) {
	const (
		gatewayName = "gw-transient"
		namespace   = "infra-transient"
	)
	release := helm.GetReleaseName(gatewayName)
	labels := map[string]string{"app.kubernetes.io/instance": release, "app.kubernetes.io/component": "controller"}
	svcName := release + "-controller"
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: svcName, Namespace: namespace, Labels: labels, UID: "uid-transient",
			Annotations: map[string]string{helmReleaseNameAnnotation: release, helmReleaseNamespaceAnnotation: namespace},
		},
		Spec: corev1.ServiceSpec{Selector: labels, Ports: []corev1.ServicePort{{Name: "rest", Port: 9090}}},
	}).Build()
	reg := registry.GetGatewayRegistry()
	t.Cleanup(func() { reg.Unregister(namespace, gatewayName) })
	ctx := context.Background()

	ok := &fakeReleaseServices{services: map[string]*helm.ManifestService{
		namespace + "/" + release + "/controller": {Name: svcName, Selector: labels},
	}}
	require.NoError(t, registerGatewayInRegistry(ctx, cl, ok, gatewayName, namespace, nil, "", "", false))

	failing := &fakeReleaseServices{err: errors.New("helm storage unavailable")}
	require.Error(t, registerGatewayInRegistry(ctx, cl, failing, gatewayName, namespace, nil, "", "", false))

	info, found := reg.Get(namespace, gatewayName)
	require.True(t, found)
	ep, err := managementEndpoint(ctx, cl, info)
	require.NoError(t, err, "the unchanged Service is still verified and usable")
	require.Equal(t, info.GetGatewayServiceEndpoint(), ep)
}

// TestVerifiedAuth_RejectsOtherHosts checks that credentials are attached only to requests that
// target the gateway's verified controller endpoint.
func TestVerifiedAuth_RejectsOtherHosts(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	sel := map[string]string{"app": "gwc"}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "gwc", Namespace: "ns", UID: "uid-1"},
		Spec:       corev1.ServiceSpec{Selector: sel},
	}).Build()
	gw := &registry.GatewayInfo{Name: "gw", Namespace: "ns", ServiceName: "gwc", ServicePort: 9090, ServiceUID: "uid-1", ServiceSelector: sel}
	ctx := context.Background()
	authFn := verifiedAuth(cl, gw, func(_ context.Context, r *http.Request) error { r.Header.Set("Authorization", "Basic x"); return nil })

	good, err := http.NewRequestWithContext(ctx, http.MethodGet, gw.GetGatewayServiceEndpoint()+"/api/management/v0.9/rest-apis", nil)
	require.NoError(t, err)
	require.NoError(t, authFn(ctx, good))
	require.Equal(t, "Basic x", good.Header.Get("Authorization"))

	other, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://attacker.example.com/api", nil)
	require.NoError(t, err)
	require.ErrorIs(t, authFn(ctx, other), errControllerServiceUnverified)
	require.Empty(t, other.Header.Get("Authorization"))
}

// TestManagementEndpoint covers the check every management call makes against the live Service.
// Registration runs only while the gateway is ready, so after a successful registration the
// gateway can go not ready and its entry is never refreshed; these cases change the Service
// without re-registering, which is exactly that state.
func TestManagementEndpoint(t *testing.T) {
	const (
		gatewayName = "gw-mgmt"
		namespace   = "infra-mgmt"
	)
	release := helm.GetReleaseName(gatewayName)
	labels := map[string]string{
		"app.kubernetes.io/instance":  release,
		"app.kubernetes.io/component": "controller",
	}
	svcName := release + "-controller"
	src := &fakeReleaseServices{services: map[string]*helm.ManifestService{
		namespace + "/" + release + "/controller": {Name: svcName, Selector: labels},
	}}
	genuine := func() *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: svcName, Namespace: namespace, Labels: labels, UID: "uid-genuine",
				Annotations: map[string]string{
					helmReleaseNameAnnotation:      release,
					helmReleaseNamespaceAnnotation: namespace,
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: labels,
				Ports:    []corev1.ServicePort{{Name: "rest", Port: 9090}},
			},
		}
	}

	tests := []struct {
		name    string
		change  func(t *testing.T, ctx context.Context, cl client.Client)
		wantErr string
	}{
		{
			name:   "the verified Service is used",
			change: func(*testing.T, context.Context, client.Client) {},
		},
		{
			name: "deleted and recreated under the same name (new UID)",
			change: func(t *testing.T, ctx context.Context, cl client.Client) {
				require.NoError(t, cl.Delete(ctx, genuine()))
				s := genuine()
				s.UID = "uid-replacement"
				s.ResourceVersion = ""
				require.NoError(t, cl.Create(ctx, s))
			},
			wantErr: "changed since it was verified",
		},
		{
			name: "replaced by an ExternalName",
			change: func(t *testing.T, ctx context.Context, cl client.Client) {
				require.NoError(t, cl.Delete(ctx, genuine()))
				require.NoError(t, cl.Create(ctx, &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: svcName, Namespace: namespace, UID: "uid-external"},
					Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName, ExternalName: "attacker.example.com"},
				}))
			},
			wantErr: "changed since it was verified",
		},
		{
			name: "switched to ExternalName in place (same UID)",
			change: func(t *testing.T, ctx context.Context, cl client.Client) {
				s := &corev1.Service{}
				require.NoError(t, cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: svcName}, s))
				s.Spec.Type = corev1.ServiceTypeExternalName
				s.Spec.ExternalName = "attacker.example.com"
				require.NoError(t, cl.Update(ctx, s))
			},
			wantErr: "changed since it was verified",
		},
		{
			name: "selector retargeted in place (same UID)",
			change: func(t *testing.T, ctx context.Context, cl client.Client) {
				s := &corev1.Service{}
				require.NoError(t, cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: svcName}, s))
				s.Spec.Selector = map[string]string{"app": "attacker"}
				require.NoError(t, cl.Update(ctx, s))
			},
			wantErr: "changed since it was verified",
		},
		{
			name: "deleted",
			change: func(t *testing.T, ctx context.Context, cl client.Client) {
				require.NoError(t, cl.Delete(ctx, genuine()))
			},
			wantErr: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(genuine()).Build()
			reg := registry.GetGatewayRegistry()
			t.Cleanup(func() { reg.Unregister(namespace, gatewayName) })
			ctx := context.Background()

			require.NoError(t, registerGatewayInRegistry(ctx, cl, src, gatewayName, namespace, nil, "", "", false))
			info, ok := reg.Get(namespace, gatewayName)
			require.True(t, ok)
			require.Equal(t, "uid-genuine", info.ServiceUID)

			tc.change(t, ctx, cl)

			ep, err := managementEndpoint(ctx, cl, info)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.Empty(t, ep)
				return
			}
			require.NoError(t, err)
			require.Equal(t, info.GetGatewayServiceEndpoint(), ep)
		})
	}
}

func TestManagementEndpoint_UnverifiedEntryRefused(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "gwc", Namespace: "ns", UID: "uid-1"},
	}).Build()
	_, err := managementEndpoint(context.Background(), cl, &registry.GatewayInfo{Name: "gw", Namespace: "ns", ServiceName: "gwc", ServicePort: 9090})
	require.ErrorContains(t, err, "changed since it was verified")
	require.ErrorIs(t, err, errControllerServiceUnverified)
}

// TestManagementEndpoint_ReadsTheLiveService checks that the Service is read through the
// uncached reader, not the caller's (cached) client: a Service retargeted on the API server
// is refused even while the cache still shows the verified version.
func TestManagementEndpoint_ReadsTheLiveService(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	selector := map[string]string{"app.kubernetes.io/component": "controller"}
	svc := func(sel map[string]string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "gwc", Namespace: "ns", UID: "uid-1"},
			Spec:       corev1.ServiceSpec{Selector: sel},
		}
	}
	cached := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc(selector)).Build()
	live := fake.NewClientBuilder().WithScheme(scheme).WithObjects(svc(map[string]string{"app": "attacker"})).Build()
	gw := &registry.GatewayInfo{
		Name: "gw", Namespace: "ns", ServiceName: "gwc", ServicePort: 9090,
		ServiceUID: "uid-1", ServiceSelector: selector,
	}

	ep, err := managementEndpoint(context.Background(), cached, gw)
	require.NoError(t, err, "without a live reader the caller's client is used")
	require.NotEmpty(t, ep)

	SetServiceReader(live)
	t.Cleanup(func() { SetServiceReader(nil) })
	_, err = managementEndpoint(context.Background(), cached, gw)
	require.ErrorIs(t, err, errControllerServiceUnverified)
}

func TestControllerRESTPort(t *testing.T) {
	svc := &corev1.Service{Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
		{Name: "metrics", Port: 9091},
		{Name: "rest", Port: 19090},
	}}}
	port, err := controllerRESTPort(svc)
	require.NoError(t, err)
	require.Equal(t, int32(19090), port)

	port, err = controllerRESTPort(&corev1.Service{})
	require.NoError(t, err)
	require.Equal(t, int32(9090), port)

	// With controller TLS on the chart exposes only rest-tls; the operator cannot use it yet.
	_, err = controllerRESTPort(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "gwc", Namespace: "ns"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "rest-tls", Port: 9093}, {Name: "xds", Port: 18000}}},
	})
	require.ErrorContains(t, err, "only over TLS")

	// rest-tls numbered 9090 is still TLS: not accepted by the numeric fallback.
	_, err = controllerRESTPort(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "gwc", Namespace: "ns"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "rest-tls", Port: 9090}}},
	})
	require.ErrorContains(t, err, "only over TLS")

	// An unnamed plain port 9090 is still accepted.
	port, err = controllerRESTPort(&corev1.Service{Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 9090}}}})
	require.NoError(t, err)
	require.Equal(t, int32(9090), port)
}

// TestVerifiedAuth_ServiceChangedBetweenRequests covers a Service retargeted between two management
// requests of one operation: the first request is verified and sent, the second is refused and gets
// no credentials, because the check runs in each request's auth step.
func TestVerifiedAuth_ServiceChangedBetweenRequests(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	sel := map[string]string{"app": "gwc"}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "gwc", Namespace: "ns", UID: "uid-1"},
		Spec:       corev1.ServiceSpec{Selector: sel},
	}).Build()
	gw := &registry.GatewayInfo{Name: "gw", Namespace: "ns", ServiceName: "gwc", ServicePort: 9090, ServiceUID: "uid-1", ServiceSelector: sel}
	ctx := context.Background()
	authFn := verifiedAuth(cl, gw, func(_ context.Context, r *http.Request) error { r.Header.Set("Authorization", "Basic x"); return nil })
	newReq := func() *http.Request {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, gw.GetGatewayServiceEndpoint()+"/api/management/v0.9/api-keys", nil)
		require.NoError(t, err)
		return req
	}

	first := newReq()
	require.NoError(t, authFn(ctx, first))
	require.Equal(t, "Basic x", first.Header.Get("Authorization"))

	// Between the two requests the Service is retargeted to other pods.
	svc := &corev1.Service{}
	require.NoError(t, cl.Get(ctx, client.ObjectKey{Namespace: "ns", Name: "gwc"}, svc))
	svc.Spec.Selector = map[string]string{"app": "attacker"}
	require.NoError(t, cl.Update(ctx, svc))

	second := newReq()
	require.ErrorIs(t, authFn(ctx, second), errControllerServiceUnverified)
	require.Empty(t, second.Header.Get("Authorization"))
}

// TestSubscriptionStatusFallbackReportsUnverifiedService checks that when the status-only fallback
// cannot be sent because the controller Service is unverified, the wait is reported (so the
// reconciler retries and the status, for example a revocation, still applies) instead of the
// original update error.
func TestSubscriptionStatusFallbackReportsUnverifiedService(t *testing.T) {
	status := "REVOKED"
	cr := &apiv1.Subscription{Spec: apiv1.SubscriptionSpec{Status: &status}}
	cause := &gatewayclient.NonRetryableError{Err: errors.New("plan rejected"), StatusCode: http.StatusBadRequest}
	refuse := func(context.Context, *http.Request) error {
		return errControllerServiceUnverified
	}
	err := updateSubscriptionStatusOnFailure(context.Background(), "http://gwc.ns.svc.cluster.local:9090", "sub-1", cr, cause, refuse)
	require.ErrorIs(t, err, errControllerServiceUnverified)
}

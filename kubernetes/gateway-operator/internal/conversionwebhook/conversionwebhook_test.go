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

package conversionwebhook

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	apiv1alpha1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	apiv2alpha1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v2alpha1"
)

const (
	testNS      = "gw-op"
	testService = "gw-op-webhook"
	testSecret  = "gw-op-webhook-cert"
)

var (
	testNow      = time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	testValidity = validity{ca: 10 * 365 * 24 * time.Hour, serving: 365 * 24 * time.Hour, renewBefore: 90 * 24 * time.Hour, caOverlap: 15 * time.Minute}
	testLogger   = slog.New(slog.NewTextHandler(io.Discard, nil))
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(s))
	require.NoError(t, apiextensionsv1.AddToScheme(s))
	require.NoError(t, apiv1.AddToScheme(s))
	require.NoError(t, apiv1alpha1.AddToScheme(s))
	require.NoError(t, apiv2alpha1.AddToScheme(s))
	return s
}

func freshBundle(t *testing.T) certBundle {
	t.Helper()
	b, err := renew(certBundle{}, renewAll, serviceDNSNames(testService, testNS), testNow, testValidity)
	require.NoError(t, err)
	return b
}

func TestPlanRenewal(t *testing.T) {
	names := serviceDNSNames(testService, testNS)
	b := freshBundle(t)

	assert.Equal(t, renewNothing, planRenewal(b, names, testNow, testValidity.renewBefore), "fresh bundle")
	assert.Equal(t, renewNothing, planRenewal(b, names, testNow.Add(200*24*time.Hour), testValidity.renewBefore),
		"serving certificate with more than 90 days left")
	assert.Equal(t, renewServing, planRenewal(b, names, testNow.Add(300*24*time.Hour), testValidity.renewBefore),
		"serving certificate within 90 days of expiry")
	assert.Equal(t, renewServing, planRenewal(b, serviceDNSNames("renamed", testNS), testNow, testValidity.renewBefore),
		"Service renamed")
	assert.Equal(t, renewAll, planRenewal(b, names, testNow.Add(10*365*24*time.Hour), testValidity.renewBefore),
		"CA within 90 days of expiry")
	assert.Equal(t, renewAll, planRenewal(certBundle{}, names, testNow, testValidity.renewBefore), "empty Secret")

	other := freshBundle(t)
	mixed := b
	mixed.tlsCert, mixed.tlsKey = other.tlsCert, other.tlsKey
	assert.Equal(t, renewServing, planRenewal(mixed, names, testNow, testValidity.renewBefore),
		"serving certificate from another CA")

	mismatched := b
	mismatched.tlsKey = other.tlsKey
	assert.Equal(t, renewServing, planRenewal(mismatched, names, testNow, testValidity.renewBefore),
		"serving key does not match the certificate")
}

// A renewed serving certificate keeps the CA, so the CRD's caBundle stays valid.
func TestRenewServingKeepsCA(t *testing.T) {
	b := freshBundle(t)
	later := testNow.Add(300 * 24 * time.Hour)
	renewed, err := renew(b, renewServing, serviceDNSNames(testService, testNS), later, testValidity)
	require.NoError(t, err)
	assert.Equal(t, b.caCert, renewed.caCert)
	assert.Equal(t, b.caKey, renewed.caKey)
	assert.NotEqual(t, b.tlsCert, renewed.tlsCert)
	assert.Equal(t, renewNothing, planRenewal(renewed, serviceDNSNames(testService, testNS), later, testValidity.renewBefore))

	leaf, err := parseCertPEM(renewed.tlsCert)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"gw-op-webhook", "gw-op-webhook.gw-op", "gw-op-webhook.gw-op.svc", "gw-op-webhook.gw-op.svc.cluster.local",
	}, leaf.DNSNames)
	assert.Equal(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, leaf.ExtKeyUsage)
}

func newSecretManager(c client.Client, now time.Time) *secretManager {
	return &secretManager{
		reader: c, writer: c, namespace: testNS, name: testSecret,
		dnsNames: serviceDNSNames(testService, testNS), validity: testValidity,
		now: func() time.Time { return now },
	}
}

func TestSecretManagerEnsure(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()
	ctx := context.Background()

	first, err := newSecretManager(c, testNow).ensure(ctx)
	require.NoError(t, err)
	secret := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: testNS, Name: testSecret}, secret))
	assert.Equal(t, first.data(), secret.Data)
	version := secret.ResourceVersion

	// Nothing to renew: no write.
	again, err := newSecretManager(c, testNow.Add(24*time.Hour)).ensure(ctx)
	require.NoError(t, err)
	assert.Equal(t, first, again)
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: testNS, Name: testSecret}, secret))
	assert.Equal(t, version, secret.ResourceVersion)

	// Close to expiry: the serving certificate is re-issued under the same CA.
	renewed, err := newSecretManager(c, testNow.Add(300*24*time.Hour)).ensure(ctx)
	require.NoError(t, err)
	assert.Equal(t, first.caCert, renewed.caCert)
	assert.NotEqual(t, first.tlsCert, renewed.tlsCert)
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: testNS, Name: testSecret}, secret))
	assert.Equal(t, renewed.data(), secret.Data)
}

func placeholderCRD() *apiextensionsv1.CustomResourceDefinition {
	path := ConvertPath
	port := int32(443)
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: APIGatewayCRDName},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group:    "gateway.api-platform.wso2.com",
			Versions: crdVersions("v1", "v1alpha1", "v2alpha1"),
			Conversion: &apiextensionsv1.CustomResourceConversion{
				Strategy: apiextensionsv1.WebhookConverter,
				Webhook: &apiextensionsv1.WebhookConversion{
					ConversionReviewVersions: []string{"v1"},
					ClientConfig: &apiextensionsv1.WebhookClientConfig{Service: &apiextensionsv1.ServiceReference{
						Namespace: "system", Name: "webhook", Path: &path, Port: &port,
					}},
				},
			},
		},
	}
}

func TestCRDReconcilerPointsCRDAtWebhook(t *testing.T) {
	ctx := context.Background()
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(placeholderCRD()).Build()
	secrets := newSecretManager(c, testNow)
	r := &crdReconciler{conversion: newConversion(c, testNow), secrets: secrets, crdName: APIGatewayCRDName, resync: time.Minute}

	// No certificate yet: wait for the rotator.
	res, err := r.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, res.RequeueAfter)

	bundle, err := secrets.ensure(ctx)
	require.NoError(t, err)
	res, err = r.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	assert.Equal(t, time.Minute, res.RequeueAfter)

	crd := &apiextensionsv1.CustomResourceDefinition{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: APIGatewayCRDName}, crd))
	svc := crd.Spec.Conversion.Webhook.ClientConfig.Service
	assert.Equal(t, apiextensionsv1.WebhookConverter, crd.Spec.Conversion.Strategy)
	assert.Equal(t, testNS, svc.Namespace)
	assert.Equal(t, testService, svc.Name)
	assert.Equal(t, "/convert", *svc.Path)
	assert.Equal(t, int32(443), *svc.Port)
	assert.Equal(t, bundle.caCert, crd.Spec.Conversion.Webhook.ClientConfig.CABundle)
	version := crd.ResourceVersion

	// Already correct: no write.
	_, err = r.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: APIGatewayCRDName}, crd))
	assert.Equal(t, version, crd.ResourceVersion)
}

// conversionReview posts a ConversionReview for objs to the handler and returns the
// converted objects.
func conversionReview(t *testing.T, url, desired string, objs ...runtime.Object) []map[string]interface{} {
	t.Helper()
	review := apiextensionsv1.ConversionReview{
		TypeMeta: metav1.TypeMeta{APIVersion: "apiextensions.k8s.io/v1", Kind: "ConversionReview"},
		Request:  &apiextensionsv1.ConversionRequest{UID: "req-1", DesiredAPIVersion: desired},
	}
	for _, o := range objs {
		raw, err := json.Marshal(o)
		require.NoError(t, err)
		review.Request.Objects = append(review.Request.Objects, runtime.RawExtension{Raw: raw})
	}
	body, err := json.Marshal(review)
	require.NoError(t, err)
	resp, err := http.Post(url+ConvertPath, "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	out := apiextensionsv1.ConversionReview{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotNil(t, out.Response)
	require.Equal(t, metav1.StatusSuccess, out.Response.Result.Status, out.Response.Result.Message)
	var converted []map[string]interface{}
	for _, o := range out.Response.ConvertedObjects {
		m := map[string]interface{}{}
		require.NoError(t, json.Unmarshal(o.Raw, &m))
		converted = append(converted, m)
	}
	return converted
}

func newConversionServer(t *testing.T, maxBytes int64) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(ConvertPath, limitBody(maxBytes, conversion.NewWebhookHandler(testScheme(t), conversion.NewRegistry())))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// The API server stores v2alpha1 writes as v1 and serves v1 objects as v2alpha1:
// spec.adminCredentials goes into the annotation and comes back out of it.
func TestConversionReviewRoundTrip(t *testing.T) {
	srv := newConversionServer(t, DefaultMaxRequestBytes)
	gw := &apiv2alpha1.APIGateway{
		TypeMeta:   metav1.TypeMeta{APIVersion: "gateway.api-platform.wso2.com/v2alpha1", Kind: "APIGateway"},
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-1", Namespace: "gw1"},
		Spec: apiv2alpha1.GatewaySpec{
			APISelector: apiv2alpha1.APISelector{Scope: apiv2alpha1.ClusterScope},
			AdminCredentials: &apiv2alpha1.GatewayAdminCredentials{
				Username: "gw1-admin",
				PasswordSecretRef: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "gateway-1-admin"}, Key: "password",
				},
			},
		},
	}

	stored := conversionReview(t, srv.URL, "gateway.api-platform.wso2.com/v1", gw)
	require.Len(t, stored, 1)
	assert.Equal(t, "gateway.api-platform.wso2.com/v1", stored[0]["apiVersion"])
	assert.NotContains(t, stored[0]["spec"], "adminCredentials")
	annotations := stored[0]["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
	assert.Contains(t, annotations[apiv2alpha1.ConversionDataAnnotation], `"passwordSecretRef":{"name":"gateway-1-admin","key":"password"}`)

	v1obj := &apiv1.APIGateway{}
	raw, err := json.Marshal(stored[0])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, v1obj))

	served := conversionReview(t, srv.URL, "gateway.api-platform.wso2.com/v2alpha1", v1obj)
	require.Len(t, served, 1)
	spec := served[0]["spec"].(map[string]interface{})
	creds := spec["adminCredentials"].(map[string]interface{})
	assert.Equal(t, "gw1-admin", creds["username"])
	assert.NotContains(t, served[0]["metadata"], "annotations")

	// v1 -> v1alpha1 keeps the annotation, so a v1alpha1 update preserves it.
	alpha := conversionReview(t, srv.URL, "gateway.api-platform.wso2.com/v1alpha1", v1obj)
	assert.Contains(t, alpha[0]["metadata"].(map[string]interface{})["annotations"], apiv2alpha1.ConversionDataAnnotation)
}

func TestLimitBody(t *testing.T) {
	srv := newConversionServer(t, 64)

	resp, err := http.Post(srv.URL+ConvertPath, "application/json", strings.NewReader(strings.Repeat("x", 65)))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	assert.NotContains(t, string(body), "64", "the response must not reveal the configured limit")

	// Chunked (no Content-Length) bodies are capped too.
	req, err := http.NewRequest(http.MethodPost, srv.URL+ConvertPath, io.MultiReader(strings.NewReader(strings.Repeat("x", 100))))
	require.NoError(t, err)
	req.ContentLength = -1
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)

	resp, err = http.Get(srv.URL + ConvertPath)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestCurvePreferences(t *testing.T) {
	assert.Equal(t, []tls.CurveID{tls.X25519MLKEM768, tls.X25519, tls.CurveP256}, curvePreferences(true))
	assert.Equal(t, []tls.CurveID{tls.X25519, tls.CurveP256}, curvePreferences(false))
}

// The webhook's TLS serves the in-memory certificate for the Service name, negotiates
// the hybrid key exchange with a client that supports it, and still connects a
// classical-only client.
func TestTLSHandshake(t *testing.T) {
	bundle := freshBundle(t)
	// The test certificate is valid from testNow; serve it on a real clock by issuing
	// one for now.
	bundle, err := renew(bundle, renewServing, serviceDNSNames(testService, testNS), time.Now(), testValidity)
	require.NoError(t, err)
	store := &certStore{}
	require.NoError(t, store.set(bundle))

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	srv.TLS = tlsConfig(store, true, false, testLogger)
	srv.StartTLS()
	t.Cleanup(srv.Close)

	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(bundle.caCert))
	dial := func(curves []tls.CurveID) tls.ConnectionState {
		t.Helper()
		conn, err := tls.Dial("tcp", srv.Listener.Addr().String(), &tls.Config{
			RootCAs: roots, ServerName: testService + "." + testNS + ".svc", CurvePreferences: curves,
			MinVersion: tls.VersionTLS13,
		})
		require.NoError(t, err)
		defer conn.Close()
		return conn.ConnectionState()
	}
	assert.Equal(t, tls.X25519MLKEM768, dial(nil).CurveID, "hybrid key exchange preferred")
	assert.Equal(t, tls.X25519, dial([]tls.CurveID{tls.X25519}).CurveID, "classical fallback")
	assert.Equal(t, []string{"http/1.1"}, srv.TLS.NextProtos, "HTTP/2 off by default")
}

func TestCertStoreNotReady(t *testing.T) {
	store := &certStore{}
	assert.False(t, store.ready())
	_, err := store.GetCertificate(nil)
	require.Error(t, err)
}

func TestOptionsValidate(t *testing.T) {
	ok := Options{Namespace: testNS, ServiceName: testService, SecretName: testSecret, Port: 9443, MaxRequestBytes: 1}
	require.NoError(t, ok.validate())
	bad := ok
	bad.Namespace = ""
	require.ErrorContains(t, bad.validate(), "POD_NAMESPACE")
	bad = ok
	bad.Port = 70000
	require.Error(t, bad.validate())
}

// Every replica runs the rotator. A replica that loses the race to create the Secret
// (its read saw no Secret, but another replica created one before its create) serves
// the winner's certificates instead of failing or overwriting them.
func TestSecretManagerEnsureLosesCreateRace(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	backing := fake.NewClientBuilder().WithScheme(scheme).Build()

	winner, err := newSecretManager(backing, testNow).ensure(ctx)
	require.NoError(t, err)

	// The loser's first read happened before the winner's create.
	staleReads := 1
	reader := interceptor.NewClient(backing.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, isSecret := obj.(*corev1.Secret); isSecret && staleReads > 0 {
				staleReads--
				return apierrors.NewNotFound(corev1.Resource("secrets"), key.Name)
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})
	loser := newSecretManager(backing, testNow)
	loser.reader = reader

	got, err := loser.ensure(ctx)
	require.NoError(t, err)
	assert.Equal(t, winner, got, "the loser serves the winner's certificates")

	secret := &corev1.Secret{}
	require.NoError(t, backing.Get(ctx, types.NamespacedName{Namespace: testNS, Name: testSecret}, secret))
	assert.Equal(t, winner.data(), secret.Data, "the winner's Secret is not overwritten")
}

// Nothing in the webhook waits for the leader lease: during a rolling upgrade the old
// operator version holds it, and new pods must still get a certificate, serve, point
// the CRD at the webhook and turn ready, or the rollout never replaces the old pods.
func TestWebhookDoesNotNeedLeaderElection(t *testing.T) {
	assert.False(t, (&server{}).NeedLeaderElection(), "server")
	assert.False(t, (&loader{}).NeedLeaderElection(), "loader")
	assert.False(t, (&rotator{}).NeedLeaderElection(), "rotator")
	opts := crdControllerOptions()
	require.NotNil(t, opts.NeedLeaderElection, "CRD reconciler")
	assert.False(t, *opts.NeedLeaderElection, "CRD reconciler")
}

func newConversion(c client.Client, now time.Time) *crdConversion {
	return &crdConversion{reader: c, writer: c, crdName: APIGatewayCRDName, namespace: testNS,
		service: testService, now: func() time.Time { return now }, logger: testLogger}
}

func crdCABundle(t *testing.T, c client.Client) []byte {
	t.Helper()
	crd := &apiextensionsv1.CustomResourceDefinition{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: APIGatewayCRDName}, crd))
	return crd.Spec.Conversion.Webhook.ClientConfig.CABundle
}

func pemCount(b []byte) int { return strings.Count(string(b), "BEGIN CERTIFICATE") }

// When the CA nears expiry, the new CA is created with the old one kept as previous
// for the overlap, and the CRD trusts both until the overlap ends.
func TestCARenewalKeepsPreviousCAForOverlap(t *testing.T) {
	ctx := context.Background()
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(placeholderCRD()).Build()
	first, err := newSecretManager(c, testNow).ensure(ctx)
	require.NoError(t, err)

	nearCAExpiry := testNow.Add(10*365*24*time.Hour - 30*24*time.Hour)
	renewed, err := newSecretManager(c, nearCAExpiry).ensure(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, first.caCert, renewed.caCert, "a new CA")
	assert.Equal(t, first.caCert, renewed.previousCA, "the old CA kept as previous")
	assert.Equal(t, nearCAExpiry.Add(15*time.Minute), renewed.previousUntil)

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: testNS, Name: testSecret}, secret))
	assert.Equal(t, first.caCert, secret.Data[SecretKeyPreviousCA])
	assert.Equal(t, nearCAExpiry.Add(15*time.Minute).UTC().Format(time.RFC3339), secret.Annotations[PreviousCAUntilAnnotation])

	// During the overlap the CRD trusts both; the reconciler comes back when it ends.
	r := &crdReconciler{conversion: newConversion(c, nearCAExpiry), secrets: newSecretManager(c, nearCAExpiry),
		crdName: APIGatewayCRDName, resync: 30 * time.Minute}
	res, err := r.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	assert.Equal(t, 2, pemCount(crdCABundle(t, c)))
	assert.Equal(t, 15*time.Minute+time.Second, res.RequeueAfter)

	// After the overlap only the new CA is trusted.
	r.conversion = newConversion(c, nearCAExpiry.Add(16*time.Minute))
	_, err = r.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	assert.Equal(t, renewed.caCert, crdCABundle(t, c))
}

// When the Secret was deleted, the new CA takes the CA the CRD trusts today as previous,
// so replicas still serving the old certificate are not rejected.
func TestDeletedSecretTakesPreviousCAFromCRD(t *testing.T) {
	ctx := context.Background()
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(placeholderCRD()).Build()
	old, err := newSecretManager(c, testNow).ensure(ctx)
	require.NoError(t, err)
	require.NoError(t, newConversion(c, testNow).trust(ctx, old))
	require.NoError(t, c.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: testNS, Name: testSecret}}))

	m := newSecretManager(c, testNow)
	m.currentCA = newConversion(c, testNow).caBundle
	fresh, err := m.ensure(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, old.caCert, fresh.caCert)
	assert.Equal(t, old.caCert, fresh.previousCA)
	assert.Equal(t, 2, pemCount(trustedCABundle(fresh, testNow)))
	assert.Equal(t, fresh.caCert, trustedCABundle(fresh, testNow.Add(15*time.Minute)), "overlap over")
}

func TestPreviousCAsFilters(t *testing.T) {
	a, b := freshBundle(t), freshBundle(t)
	both := append(append([]byte{}, a.caCert...), b.caCert...)
	assert.Equal(t, a.caCert, previousCAs(both, b.caCert, testNow), "the current CA is dropped")
	assert.Empty(t, previousCAs(both, b.caCert, testNow.Add(11*365*24*time.Hour)), "expired CAs are dropped")
	assert.Empty(t, previousCAs(a.tlsCert, b.caCert, testNow), "non-CA certificates are dropped")
	assert.Empty(t, previousCAs([]byte("garbage"), b.caCert, testNow))
}

// A certificate is served only after the CRD trusts it: if the CRD cannot be patched,
// the replica keeps serving what it had.
func TestServeTrustsBeforeServing(t *testing.T) {
	ctx := context.Background()
	backing := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(placeholderCRD()).Build()
	b := freshBundle(t)
	failing := interceptor.NewClient(backing.(client.WithWatch), interceptor.Funcs{
		Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
			return apierrors.NewServiceUnavailable("patch refused")
		},
	})
	st := &servingState{store: &certStore{}, conversion: newConversion(failing, testNow)}
	require.Error(t, st.serve(ctx, b))
	assert.False(t, st.store.ready(), "nothing served while the CRD does not trust the certificate")

	st.conversion = newConversion(backing, testNow)
	require.NoError(t, st.serve(ctx, b))
	assert.True(t, st.store.serving(b))
	assert.Equal(t, b.caCert, crdCABundle(t, backing))
}

// The loader recreates a deleted Secret instead of waiting for the hourly rotator.
func TestLoaderRecreatesMissingSecret(t *testing.T) {
	ctx := context.Background()
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(placeholderCRD()).Build()
	l := &loader{servingState: &servingState{secrets: newSecretManager(c, testNow), conversion: newConversion(c, testNow),
		store: &certStore{}}, interval: time.Minute, logger: testLogger}
	l.tick(ctx)
	secret := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: testNS, Name: testSecret}, secret))
	assert.True(t, l.store.ready())
	assert.Equal(t, secret.Data[SecretKeyCACert], crdCABundle(t, c))
}

// sync always serves what the Secret holds when it runs: a certificate served earlier
// from an older read is replaced, and one already served is not reloaded.
func TestSyncServesTheSecretsCurrentCertificate(t *testing.T) {
	ctx := context.Background()
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(placeholderCRD()).Build()
	m := newSecretManager(c, testNow)
	current, err := m.ensure(ctx)
	require.NoError(t, err)
	st := &servingState{secrets: m, conversion: newConversion(c, testNow), store: &certStore{}}

	older := freshBundle(t) // stands in for a certificate read before a renewal
	require.NoError(t, st.store.set(older))
	loaded, err := st.sync(ctx)
	require.NoError(t, err)
	assert.True(t, loaded)
	assert.True(t, st.store.serving(current))

	loaded, err = st.sync(ctx)
	require.NoError(t, err)
	assert.False(t, loaded, "nothing to reload")
}

// A CA key that does not belong to the CA certificate makes the bundle unusable for
// signing, so the whole bundle is replaced rather than retrying serving renewals.
func TestMismatchedCAKeyRenewsEverything(t *testing.T) {
	a, b := freshBundle(t), freshBundle(t)
	mixed := a
	mixed.caKey = b.caKey
	_, _, err := parseCA(mixed.caCert, mixed.caKey)
	require.ErrorContains(t, err, "does not match")
	assert.Equal(t, renewAll, planRenewal(mixed, serviceDNSNames(testService, testNS), testNow, testValidity.renewBefore))
	_, _, err = parseCA(a.caCert, a.caKey)
	require.NoError(t, err)
}

func crdVersions(names ...string) []apiextensionsv1.CustomResourceDefinitionVersion {
	var out []apiextensionsv1.CustomResourceDefinitionVersion
	for i, n := range names {
		out = append(out, apiextensionsv1.CustomResourceDefinitionVersion{Name: n, Served: true, Storage: i == 0})
	}
	return out
}

// oldCRD is an older chart's APIGateway CRD: v1 and v1alpha1 only, identical schemas.
func oldCRD(conv *apiextensionsv1.CustomResourceConversion) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: APIGatewayCRDName},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "gateway.api-platform.wso2.com", Versions: crdVersions("v1", "v1alpha1"), Conversion: conv,
		},
	}
}

func liveConversion(t *testing.T, c client.Client) *apiextensionsv1.CustomResourceConversion {
	t.Helper()
	crd := &apiextensionsv1.CustomResourceDefinition{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: APIGatewayCRDName}, crd))
	return crd.Spec.Conversion
}

// An older chart's CRD (after a rollback, or before the new CRD is applied) serves no
// version that needs the webhook: the operator does not point it at the webhook, so a
// rolled-back install is never left converting through a webhook that is gone.
func TestTrustLeavesCRDWithoutV2alpha1OnNone(t *testing.T) {
	ctx := context.Background()
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).
		WithObjects(oldCRD(&apiextensionsv1.CustomResourceConversion{Strategy: apiextensionsv1.NoneConverter})).Build()
	require.NoError(t, newConversion(c, testNow).trust(ctx, freshBundle(t)))
	assert.Equal(t, apiextensionsv1.NoneConverter, liveConversion(t, c).Strategy)
}

// An older CRD left pointing at this operator's webhook (Helm does not touch CRDs on a
// rollback) is set back to None, and serving still proceeds.
func TestTrustReleasesOwnWebhookOnCRDWithoutV2alpha1(t *testing.T) {
	ctx := context.Background()
	ours := desiredConversion(testNS, testService, []byte("ca"))
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(oldCRD(ours)).Build()
	st := &servingState{store: &certStore{}, conversion: newConversion(c, testNow)}
	b := freshBundle(t)
	require.NoError(t, st.serve(ctx, b))
	assert.True(t, st.store.serving(b), "the webhook still serves")
	got := liveConversion(t, c)
	assert.Equal(t, apiextensionsv1.NoneConverter, got.Strategy)
	assert.Nil(t, got.Webhook)
}

// A webhook conversion that is not this operator's (another install, or the shipped
// placeholder) on a CRD without v2alpha1 is left alone.
func TestTrustLeavesForeignWebhookAlone(t *testing.T) {
	ctx := context.Background()
	foreign := desiredConversion("other-ns", "other-webhook", []byte("ca"))
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(oldCRD(foreign)).Build()
	require.NoError(t, newConversion(c, testNow).trust(ctx, freshBundle(t)))
	assert.True(t, pointsAt(liveConversion(t, c), "other-ns", "other-webhook"))
}

// Once the new CRD (serving v2alpha1) is applied, the operator points it at the webhook.
func TestTrustPointsCRDServingV2alpha1AtWebhook(t *testing.T) {
	ctx := context.Background()
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(placeholderCRD()).Build()
	require.NoError(t, newConversion(c, testNow).trust(ctx, freshBundle(t)))
	assert.True(t, pointsAt(liveConversion(t, c), testNS, testService))
}

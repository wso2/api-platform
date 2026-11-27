/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	choreolabsv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/choreolabs/v1"
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
)

// fakeBackendGateway is a gateway-controller management API holding Backends by name. It
// records every request, and answers DELETE with deleteStatus when it is set.
type fakeBackendGateway struct {
	mu           sync.Mutex
	stored       map[string]bool
	requests     []string
	lastBody     string
	deleteStatus int
}

func (g *fakeBackendGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.requests = append(g.requests, r.Method+" "+r.URL.Path)
	name := r.URL.Path[len(gatewayclient.BackendsPath()):]
	if len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}
	switch r.Method {
	case http.MethodGet:
		if g.stored[name] {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	case http.MethodPost, http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		g.lastBody = string(body)
		var envelope struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}
		_ = yaml.Unmarshal(body, &envelope)
		g.stored[envelope.Metadata.Name] = true
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_, _ = w.Write([]byte("{}"))
	case http.MethodDelete:
		if g.deleteStatus != 0 {
			w.WriteHeader(g.deleteStatus)
			_, _ = w.Write([]byte(`{"status":"error","message":"in use"}`))
			return
		}
		if !g.stored[name] {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		delete(g.stored, name)
		w.WriteHeader(http.StatusOK)
	}
}

func backendCR() *choreolabsv1.Backend {
	basePath := "/v1"
	weight := 70
	verify := true
	connect := "2s"
	return &choreolabsv1.Backend{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-eu", Namespace: "default", Labels: map[string]string{"team": "orders"}},
		Spec: choreolabsv1.BackendSpec{
			DisplayName: "Orders (EU)",
			BasePath:    &basePath,
			Endpoints: []choreolabsv1.BackendEndpoint{
				{URL: "https://orders-eu-1:8443", Weight: &weight},
				{URL: "https://orders-eu-2:8443"},
			},
			Timeout: &apiv1.UpstreamTimeout{Connect: &connect},
			TLS:     &choreolabsv1.BackendTLS{TrustedCAs: []string{"orders-ca"}, VerifyHostName: &verify},
		},
	}
}

// A Backend CR becomes a Backend on the gateway: created with POST, then updated with PUT, in the
// gateway's own shape and with the apiVersion the gateway requires for the kind.
func TestBackendAdapter_DeploysTheBackendToTheGateway(t *testing.T) {
	gw := &fakeBackendGateway{stored: map[string]bool{}}
	srv := httptest.NewServer(gw)
	defer srv.Close()
	adapter := &backendAdapter{}
	cr := backendCR()

	_, err := adapter.Deploy(context.Background(), nil, srv.URL, cr, nil)
	require.NoError(t, err)
	_, err = adapter.Deploy(context.Background(), nil, srv.URL, cr, nil)
	require.NoError(t, err)

	path := gatewayclient.BackendsPath()
	require.Equal(t, []string{
		"GET " + path + "/orders-eu", "POST " + path,
		"GET " + path + "/orders-eu", "PUT " + path + "/orders-eu",
	}, gw.requests)

	var sent map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(gw.lastBody), &sent))
	require.Equal(t, "gateway.choreolabs.com/v1", sent["apiVersion"])
	require.Equal(t, "Backend", sent["kind"])
	metadata := sent["metadata"].(map[string]interface{})
	require.Equal(t, "orders-eu", metadata["name"])
	require.Equal(t, map[string]interface{}{"team": "orders"}, metadata["labels"])
	spec := sent["spec"].(map[string]interface{})
	require.Equal(t, "/v1", spec["basePath"])
	require.Equal(t, map[string]interface{}{"connect": "2s"}, spec["timeout"])
	require.Equal(t, []interface{}{
		map[string]interface{}{"url": "https://orders-eu-1:8443", "weight": float64(70)},
		map[string]interface{}{"url": "https://orders-eu-2:8443"},
	}, spec["endpoints"])
	require.Equal(t, map[string]interface{}{"trustedCAs": []interface{}{"orders-ca"}, "verifyHostName": true}, spec["tls"])
}

// Deleting a Backend that a deployed API still pins is refused by the gateway (409). The adapter
// reports the failure, so the finalizer stays and the delete is retried; a Backend already gone
// counts as deleted.
func TestBackendAdapter_DeleteFailsWhileTheGatewayRefusesIt(t *testing.T) {
	gw := &fakeBackendGateway{stored: map[string]bool{"orders-eu": true}, deleteStatus: http.StatusConflict}
	srv := httptest.NewServer(gw)
	defer srv.Close()
	adapter := &backendAdapter{}

	require.Error(t, adapter.Delete(context.Background(), srv.URL, backendCR(), nil))

	gw.deleteStatus = 0
	require.NoError(t, adapter.Delete(context.Background(), srv.URL, backendCR(), nil))
	require.NoError(t, adapter.Delete(context.Background(), srv.URL, backendCR(), nil), "a Backend already gone counts as deleted")
}

func TestBackendAdapter_Identity(t *testing.T) {
	adapter := &backendAdapter{}
	require.Equal(t, "Backend", adapter.Kind())
	require.Equal(t, "gateway.choreolabs.com/backend-finalizer", adapter.FinalizerName())
	require.False(t, adapter.IsUUIDKeyed())
	cr := backendCR()
	require.Equal(t, "orders-eu", adapter.Handle(cr))
	ns, labels := adapter.GatewaySelectionKey(cr)
	require.Equal(t, "default", ns)
	require.Equal(t, map[string]string{"team": "orders"}, labels)
}

// A label change does not bump metadata.generation, so the reconciler would skip it as already
// deployed. The labels choose the Backend's gateways and are what a backendSelector matches on
// the gateway, so the adapter reports a label change as drift until the new labels are deployed.
func TestBackendAdapter_ALabelOnlyChangeRedeploys(t *testing.T) {
	gw := &fakeBackendGateway{stored: map[string]bool{}}
	srv := httptest.NewServer(gw)
	defer srv.Close()
	scheme := runtime.NewScheme()
	require.NoError(t, choreolabsv1.AddToScheme(scheme))
	cr := backendCR()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr).Build()
	adapter := &backendAdapter{}
	ctx := context.Background()
	key := types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name}
	latest := func() *choreolabsv1.Backend {
		var got choreolabsv1.Backend
		require.NoError(t, k8sClient.Get(ctx, key, &got))
		return &got
	}
	deploy := func() {
		obj := latest()
		result, err := adapter.Deploy(ctx, k8sClient, srv.URL, obj, nil)
		require.NoError(t, err)
		require.NoError(t, adapter.onExternalDepsApplied(ctx, k8sClient, obj, result.Fingerprint))
	}

	need, err := adapter.needsRedeployForExternalDeps(ctx, k8sClient, latest())
	require.NoError(t, err)
	require.True(t, need, "labels never deployed")

	deploy()
	need, err = adapter.needsRedeployForExternalDeps(ctx, k8sClient, latest())
	require.NoError(t, err)
	require.False(t, need, "recording the fingerprint is not itself a change")

	relabelled := latest()
	relabelled.Labels = map[string]string{"team": "payments"}
	require.NoError(t, k8sClient.Update(ctx, relabelled))
	need, err = adapter.needsRedeployForExternalDeps(ctx, k8sClient, latest())
	require.NoError(t, err)
	require.True(t, need, "a label-only change redeploys")

	deploy()
	var sent map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(gw.lastBody), &sent))
	require.Equal(t, map[string]interface{}{"team": "payments"}, sent["metadata"].(map[string]interface{})["labels"])
	need, err = adapter.needsRedeployForExternalDeps(ctx, k8sClient, latest())
	require.NoError(t, err)
	require.False(t, need)
}

func TestBackendLabelsFingerprint(t *testing.T) {
	require.Equal(t, backendLabelsFingerprint(map[string]string{"a": "1", "b": "2"}),
		backendLabelsFingerprint(map[string]string{"b": "2", "a": "1"}))
	require.NotEqual(t, backendLabelsFingerprint(map[string]string{"a": "1b"}),
		backendLabelsFingerprint(map[string]string{"a1": "b"}))
	require.NotEmpty(t, backendLabelsFingerprint(nil), "a Backend without labels still records its deploy")
	require.Equal(t, backendLabelsFingerprint(nil), backendLabelsFingerprint(map[string]string{}))
}

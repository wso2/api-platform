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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

const (
	mgClass      = "wso2-mg"
	mgOtherClass = "istio"
)

// fakeGatewayREST stands in for every gateway-controller, keyed by endpoint.
type fakeGatewayREST struct {
	mu         sync.Mutex
	apis       map[string]map[string][]byte
	calls      []restCall
	failExists map[string]error
	failDeploy map[string]error
	failDelete map[string]error
	delay      map[string]time.Duration
}

type restCall struct {
	op, endpoint, handle string
	at                   time.Time
}

func newFakeGatewayREST() *fakeGatewayREST {
	return &fakeGatewayREST{
		apis:       map[string]map[string][]byte{},
		failExists: map[string]error{},
		failDeploy: map[string]error{},
		failDelete: map[string]error{},
		delay:      map[string]time.Duration{},
	}
}

func (f *fakeGatewayREST) record(op, ep, handle string) {
	f.calls = append(f.calls, restCall{op: op, endpoint: ep, handle: handle, at: time.Now()})
}

func (f *fakeGatewayREST) Exists(_ context.Context, ep, handle string, _ gatewayclient.AuthHeaderFunc) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("exists", ep, handle)
	if err := f.failExists[ep]; err != nil {
		return false, err
	}
	_, ok := f.apis[ep][handle]
	return ok, nil
}

func (f *fakeGatewayREST) Deploy(_ context.Context, ep, handle string, apiYAML []byte, _ bool, _ gatewayclient.AuthHeaderFunc) error {
	f.mu.Lock()
	d := f.delay[ep]
	f.mu.Unlock()
	time.Sleep(d)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("deploy", ep, handle)
	if err := f.failDeploy[ep]; err != nil {
		return err
	}
	if f.apis[ep] == nil {
		f.apis[ep] = map[string][]byte{}
	}
	f.apis[ep][handle] = append([]byte(nil), apiYAML...)
	return nil
}

func (f *fakeGatewayREST) Delete(_ context.Context, ep, handle string, _ gatewayclient.AuthHeaderFunc) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("delete", ep, handle)
	if err := f.failDelete[ep]; err != nil {
		return err
	}
	delete(f.apis[ep], handle)
	return nil
}

func (f *fakeGatewayREST) count(op, ep string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c.op == op && c.endpoint == ep {
			n++
		}
	}
	return n
}

func (f *fakeGatewayREST) has(ep, handle string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.apis[ep][handle]
	return ok
}

func (f *fakeGatewayREST) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
}

type routeHarness struct {
	t    *testing.T
	ns   string
	cl   client.Client
	r    *HTTPRouteReconciler
	rest *fakeGatewayREST
}

func newRouteHarness(t *testing.T, ns string, funcs *interceptor.Funcs, objs ...client.Object) *routeHarness {
	t.Helper()
	b := fake.NewClientBuilder().WithScheme(redirectRouteScheme(t)).
		WithStatusSubresource(&gatewayv1.HTTPRoute{}).WithObjects(objs...)
	if funcs != nil {
		b = b.WithInterceptorFuncs(*funcs)
	}
	cl := b.Build()
	cfg := &config.OperatorConfig{}
	cfg.GatewayAPI.GatewayClassNames = []string{mgClass, goldenGatewayClass}
	cfg.GatewayAPI.ClusterDomain = "cluster.local"
	rest := newFakeGatewayREST()
	r := NewHTTPRouteReconciler(cl, cl.Scheme(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.restAPI = rest
	return &routeHarness{t: t, ns: ns, cl: cl, r: r, rest: rest}
}

// register makes a Gateway known to the operator, as the Gateway controller does once its runtime is up:
// the controller Service exists and the registry entry records the identity discovery verified.
func (h *routeHarness) register(gw *gatewayv1.Gateway) string {
	h.t.Helper()
	svcName := "gwc-" + gw.Name
	selector := map[string]string{"app.kubernetes.io/component": "controller", "gateway": gw.Name}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: svcName, Namespace: gw.Namespace, UID: types.UID("uid-" + gw.Namespace + "-" + svcName)},
		Spec:       corev1.ServiceSpec{Selector: selector, Ports: []corev1.ServicePort{{Name: "rest", Port: 9090}}},
	}
	if err := h.cl.Create(context.Background(), svc); err != nil && !apierrors.IsAlreadyExists(err) {
		require.NoError(h.t, err)
	}
	info := &registry.GatewayInfo{
		Name: gw.Name, Namespace: gw.Namespace, ServiceName: svcName, ServicePort: 9090, FromGatewayAPI: true,
		ServiceUID: string(svc.UID), ServiceSelector: selector,
	}
	registry.GetGatewayRegistry().Register(info)
	h.t.Cleanup(func() { registry.GetGatewayRegistry().Unregister(gw.Namespace, gw.Name) })
	return info.GetGatewayServiceEndpoint()
}

func (h *routeHarness) endpoint(gwNamespace, gwName string) string {
	return (&registry.GatewayInfo{Namespace: gwNamespace, ServiceName: "gwc-" + gwName, ServicePort: 9090}).GetGatewayServiceEndpoint()
}

func (h *routeHarness) reconcile(route *gatewayv1.HTTPRoute) ctrl.Result {
	h.t.Helper()
	res, err := h.r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(route)})
	require.NoError(h.t, err)
	return res
}

// settle reconciles until the route stops asking for an immediate requeue (the finalizer step).
func (h *routeHarness) settle(route *gatewayv1.HTTPRoute) ctrl.Result {
	h.t.Helper()
	var res ctrl.Result
	for i := 0; i < 3; i++ {
		res = h.reconcile(route)
		if !res.Requeue {
			return res
		}
	}
	return res
}

func (h *routeHarness) get(route *gatewayv1.HTTPRoute) *gatewayv1.HTTPRoute {
	h.t.Helper()
	got := &gatewayv1.HTTPRoute{}
	require.NoError(h.t, h.cl.Get(context.Background(), client.ObjectKeyFromObject(route), got))
	return got
}

func (h *routeHarness) update(route *gatewayv1.HTTPRoute, mutate func(*gatewayv1.HTTPRoute)) {
	h.t.Helper()
	latest := h.get(route)
	mutate(latest)
	latest.Generation++
	require.NoError(h.t, h.cl.Update(context.Background(), latest))
}

func mgGateway(ns, name, class string, from gatewayv1.FromNamespaces) *gatewayv1.Gateway {
	return &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: gatewayv1.ObjectName(class),
			Listeners: []gatewayv1.Listener{{
				Name:          "http",
				Port:          80,
				Protocol:      gatewayv1.HTTPProtocolType,
				AllowedRoutes: &gatewayv1.AllowedRoutes{Namespaces: &gatewayv1.RouteNamespaces{From: &from}},
			}},
		},
	}
}

func mgRef(ns, name string) gatewayv1.ParentReference {
	return gatewayv1.ParentReference{Name: gatewayv1.ObjectName(name), Namespace: goldenStr[gatewayv1.Namespace](ns)}
}

func mgRoute(ns, name, path string, refs ...gatewayv1.ParentReference) *gatewayv1.HTTPRoute {
	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Generation: 1},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: refs},
			Rules: []gatewayv1.HTTPRouteRule{{
				Matches:     []gatewayv1.HTTPRouteMatch{goldenPrefixMatch(path)},
				BackendRefs: []gatewayv1.HTTPBackendRef{goldenBackend("echo-v1", 1)},
			}},
		},
	}
}

func mgObjects(ns string) []client.Object {
	return []client.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}},
		mgService(ns, "echo-v1"),
		mgService(ns, "echo-v2"),
	}
}

func mgService(ns, name string) client.Object {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
	}
}

// ourParentStatus returns our status entry for a Gateway, or nil.
func ourParentStatus(route *gatewayv1.HTTPRoute, gwNamespace, gwName string) *gatewayv1.RouteParentStatus {
	for i := range route.Status.Parents {
		ps := &route.Status.Parents[i]
		if ps.ControllerName != PlatformGatewayControllerName {
			continue
		}
		if string(ps.ParentRef.Name) == gwName && normalizeParentRefNamespace(ps.ParentRef, route.Namespace) == gwNamespace {
			return ps
		}
	}
	return nil
}

func requireCondition(t *testing.T, ps *gatewayv1.RouteParentStatus, condType string, status metav1.ConditionStatus, reason string) {
	t.Helper()
	require.NotNil(t, ps, "missing parent status entry")
	c := meta.FindStatusCondition(ps.Conditions, condType)
	require.NotNil(t, c, "missing %s condition", condType)
	require.Equal(t, status, c.Status, "%s status (reason %s: %s)", condType, c.Reason, c.Message)
	if reason != "" {
		require.Equal(t, reason, c.Reason, "%s reason", condType)
	}
}

// recordOf returns the route's effective deployment record (multi-Gateway annotation plus the
// legacy pair). A malformed multi-Gateway annotation fails the test.
func recordOf(t *testing.T, route *gatewayv1.HTTPRoute) deployedGateways {
	t.Helper()
	if raw := route.Annotations[AnnHTTPRouteDeployedGateways]; raw != "" {
		var rec deployedGateways
		require.NoError(t, json.Unmarshal([]byte(raw), &rec))
	}
	return deployedGatewaysFromRoute(route)
}

// --- Single-parent routes behave exactly as before ---

func TestHTTPRouteGoldenPayloadsUnchanged(t *testing.T) {
	for _, fx := range goldenFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			wantYAML, err := os.ReadFile(filepath.Join("testdata", "httproute_golden", fx.name+".yaml"))
			require.NoError(t, err)
			wantAnn, err := os.ReadFile(filepath.Join("testdata", "httproute_golden", fx.name+".annotations"))
			require.NoError(t, err)
			wantParent, wantHash, _ := strings.Cut(strings.TrimSpace(string(wantAnn)), "\n")

			objs := append(goldenBaseObjects(), fx.gateway, fx.route.DeepCopy())
			h := newRouteHarness(t, goldenRouteNamespace, nil, objs...)
			ep := h.register(fx.gateway)
			h.settle(fx.route)

			handle := DefaultHTTPRouteAPIHandle(fx.route)
			got, ok := h.rest.apis[ep][handle]
			require.True(t, ok, "route not deployed")
			require.Equal(t, string(wantYAML), string(got), "deployed payload must be byte-identical to the released operator's")

			route := h.get(fx.route)
			require.Equal(t, wantParent, route.Annotations[AnnHTTPRouteLastDeployedParentGateway])
			require.Equal(t, wantHash, route.Annotations[AnnHTTPRouteLastDeployedConfigHash])
			require.Equal(t, deployedGateways{wantParent: {Hash: wantHash, Handle: handle}}, recordOf(t, route))
			require.NotContains(t, route.Annotations, AnnHTTPRouteDeployedGateways,
				"a single-Gateway route keeps only the legacy pair, as earlier releases wrote")
		})
	}
}

func TestHTTPRouteUpgradeFromLegacyAnnotationsDoesNotRedeploy(t *testing.T) {
	for _, fx := range goldenFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			wantYAML, err := os.ReadFile(filepath.Join("testdata", "httproute_golden", fx.name+".yaml"))
			require.NoError(t, err)
			wantAnn, err := os.ReadFile(filepath.Join("testdata", "httproute_golden", fx.name+".annotations"))
			require.NoError(t, err)
			wantParent, wantHash, _ := strings.Cut(strings.TrimSpace(string(wantAnn)), "\n")

			// The route as the released operator left it: finalizer and the legacy pair only.
			route := fx.route.DeepCopy()
			route.Finalizers = []string{httprouteFinalizer}
			if route.Annotations == nil {
				route.Annotations = map[string]string{}
			}
			route.Annotations[AnnHTTPRouteLastDeployedParentGateway] = wantParent
			route.Annotations[AnnHTTPRouteLastDeployedConfigHash] = wantHash

			h := newRouteHarness(t, goldenRouteNamespace, nil, append(goldenBaseObjects(), fx.gateway, route)...)
			ep := h.register(fx.gateway)
			handle := DefaultHTTPRouteAPIHandle(route)
			h.rest.apis[ep] = map[string][]byte{handle: wantYAML}

			h.settle(route)
			h.settle(route)
			require.Zero(t, h.rest.count("deploy", ep), "an upgrade must not redeploy unchanged routes")
			require.Zero(t, h.rest.count("delete", ep))

			got := h.get(route)
			require.Equal(t, route.Annotations, got.Annotations, "an upgrade must not touch a single-Gateway route's annotations")
			require.Equal(t, deployedGateways{wantParent: {Hash: wantHash, Handle: handle}}, recordOf(t, got))
			requireCondition(t, ourParentStatus(got, goldenGatewayNamespace, fx.gateway.Name), "Accepted", metav1.ConditionTrue, "Accepted")
		})
	}
}

func TestHTTPRoutePayloadIgnoresDeploymentRecord(t *testing.T) {
	fx := goldenFixtures()[0]
	h := newRouteHarness(t, goldenRouteNamespace, nil, append(goldenBaseObjects(), fx.gateway, fx.route.DeepCopy())...)
	ep := h.register(fx.gateway)
	h.settle(fx.route)
	require.Equal(t, 1, h.rest.count("deploy", ep))

	// The record written by the first deploy must not change the payload, or the route would
	// redeploy on every reconcile.
	h.rest.reset()
	for i := 0; i < 3; i++ {
		h.reconcile(fx.route)
	}
	require.Zero(t, h.rest.count("deploy", ep))
}

// --- Multiple parent Gateways ---

// The HTTPRouteMultipleGateways conformance shape: one shared route on two Gateways, and a
// dedicated route on each.
func TestHTTPRouteMultipleGatewaysDeploysToEach(t *testing.T) {
	ns := "mg-each"
	gwSame := mgGateway(ns, "same-namespace", mgClass, gatewayv1.NamespacesFromSame)
	gwAll := mgGateway(ns, "all-namespaces", mgClass, gatewayv1.NamespacesFromAll)
	shared := mgRoute(ns, "shared", "/shared", mgRef(ns, "same-namespace"), mgRef(ns, "all-namespaces"))
	dedicatedSame := mgRoute(ns, "dedicated-same", "/", mgRef(ns, "same-namespace"))
	dedicatedAll := mgRoute(ns, "dedicated-all", "/", mgRef(ns, "all-namespaces"))

	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), gwSame, gwAll, shared, dedicatedSame, dedicatedAll)...)
	epSame, epAll := h.register(gwSame), h.register(gwAll)
	for _, r := range []*gatewayv1.HTTPRoute{shared, dedicatedSame, dedicatedAll} {
		require.Zero(t, h.settle(r).RequeueAfter)
	}

	require.True(t, h.rest.has(epSame, ns+"-shared"))
	require.True(t, h.rest.has(epAll, ns+"-shared"))
	require.True(t, h.rest.has(epSame, ns+"-dedicated-same"))
	require.False(t, h.rest.has(epAll, ns+"-dedicated-same"), "a dedicated route must stay on its own Gateway")
	require.True(t, h.rest.has(epAll, ns+"-dedicated-all"))
	require.False(t, h.rest.has(epSame, ns+"-dedicated-all"))

	got := h.get(shared)
	for _, gw := range []string{"same-namespace", "all-namespaces"} {
		ps := ourParentStatus(got, ns, gw)
		requireCondition(t, ps, "Accepted", metav1.ConditionTrue, "Accepted")
		requireCondition(t, ps, "ResolvedRefs", metav1.ConditionTrue, "ResolvedRefs")
	}
	rec := recordOf(t, got)
	require.Len(t, rec, 2)
	require.Equal(t, ns+"-shared", rec[ns+"/same-namespace"].Handle)
	require.Empty(t, got.Annotations[AnnHTTPRouteLastDeployedParentGateway], "multi-Gateway routes get no legacy pair")
	require.Empty(t, got.Annotations[AnnHTTPRouteLastDeployedConfigHash])

	// Steady state: nothing is redeployed.
	h.rest.reset()
	h.reconcile(shared)
	require.Zero(t, h.rest.count("deploy", epSame)+h.rest.count("deploy", epAll))
}

func TestHTTPRouteAddAndRemoveParentGateway(t *testing.T) {
	ns := "mg-addremove"
	gw1 := mgGateway(ns, "gw1", mgClass, gatewayv1.NamespacesFromAll)
	gw2 := mgGateway(ns, "gw2", mgClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "gw1"))
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), gw1, gw2, route)...)
	ep1, ep2 := h.register(gw1), h.register(gw2)
	h.settle(route)
	require.True(t, h.rest.has(ep1, ns+"-r"))

	// Adding a second Gateway deploys there and leaves the first untouched.
	h.rest.reset()
	h.update(route, func(r *gatewayv1.HTTPRoute) {
		r.Spec.ParentRefs = append(r.Spec.ParentRefs, mgRef(ns, "gw2"))
	})
	h.settle(route)
	require.Equal(t, 1, h.rest.count("deploy", ep2))
	require.Zero(t, h.rest.count("deploy", ep1), "the first Gateway's config didn't change")
	require.Zero(t, h.rest.count("delete", ep1))
	require.Len(t, recordOf(t, h.get(route)), 2)

	// Removing it deletes the API there only, prunes its status entry and restores the legacy pair.
	h.rest.reset()
	h.update(route, func(r *gatewayv1.HTTPRoute) { r.Spec.ParentRefs = r.Spec.ParentRefs[:1] })
	h.settle(route)
	require.Equal(t, 1, h.rest.count("delete", ep2))
	require.False(t, h.rest.has(ep2, ns+"-r"))
	require.True(t, h.rest.has(ep1, ns+"-r"))
	require.Zero(t, h.rest.count("delete", ep1)+h.rest.count("deploy", ep1))

	got := h.get(route)
	require.Nil(t, ourParentStatus(got, ns, "gw2"), "status entry for a removed parent is pruned")
	requireCondition(t, ourParentStatus(got, ns, "gw1"), "Accepted", metav1.ConditionTrue, "Accepted")
	require.Len(t, recordOf(t, got), 1)
	require.Equal(t, ns+"/gw1", got.Annotations[AnnHTTPRouteLastDeployedParentGateway])
	require.Equal(t, recordOf(t, got)[ns+"/gw1"].Hash, got.Annotations[AnnHTTPRouteLastDeployedConfigHash])
	require.NotContains(t, got.Annotations, AnnHTTPRouteDeployedGateways, "back on one Gateway: legacy pair only")
}

// A downgrade without reducing a multi-Gateway route first leaves copies behind. When the route is
// cut to one parent and the operator is upgraded again, the stale record still leads cleanup to
// the orphaned copy.
func TestHTTPRouteUpgradeAfterDowngradeCleansOrphans(t *testing.T) {
	ns := "mg-downgrade"
	gw1 := mgGateway(ns, "gw1", mgClass, gatewayv1.NamespacesFromAll)
	gw2 := mgGateway(ns, "gw2", mgClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "gw1"), mgRef(ns, "gw2"))
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), gw1, gw2, route)...)
	ep1, ep2 := h.register(gw1), h.register(gw2)
	h.settle(route)
	stale := h.get(route).Annotations[AnnHTTPRouteDeployedGateways]
	require.NotEmpty(t, stale)

	// The older operator handled the route after it was cut to gw1: it wrote its legacy pair and
	// never touched the multi-Gateway annotation.
	h.update(route, func(r *gatewayv1.HTTPRoute) {
		r.Spec.ParentRefs = r.Spec.ParentRefs[:1]
		r.Annotations[AnnHTTPRouteLastDeployedParentGateway] = ns + "/gw1"
		r.Annotations[AnnHTTPRouteLastDeployedConfigHash] = "hash-from-older-operator"
	})
	h.rest.reset()
	h.settle(route)
	require.False(t, h.rest.has(ep2, ns+"-r"), "the orphaned copy is removed")
	require.True(t, h.rest.has(ep1, ns+"-r"))
	got := h.get(route)
	require.NotContains(t, got.Annotations, AnnHTTPRouteDeployedGateways)
	require.Equal(t, ns+"/gw1", got.Annotations[AnnHTTPRouteLastDeployedParentGateway])
}

// Moving a route to a Gateway that isn't ready keeps it serving on the old one until the new
// deploy succeeds, as before.
func TestHTTPRouteMoveToPendingGatewayKeepsOldServing(t *testing.T) {
	ns := "mg-move"
	gw1 := mgGateway(ns, "gw1", mgClass, gatewayv1.NamespacesFromAll)
	gw2 := mgGateway(ns, "gw2", mgClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "gw1"))
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), gw1, gw2, route)...)
	ep1 := h.register(gw1)
	h.settle(route)

	h.update(route, func(r *gatewayv1.HTTPRoute) { r.Spec.ParentRefs = []gatewayv1.ParentReference{mgRef(ns, "gw2")} })
	res := h.settle(route)
	require.Equal(t, requeueGatewayNotRegistered, res.RequeueAfter)
	require.True(t, h.rest.has(ep1, ns+"-r"), "old Gateway keeps serving while the new one is pending")
	require.Zero(t, h.rest.count("delete", ep1))
	got := h.get(route)
	require.Contains(t, recordOf(t, got), ns+"/gw1")
	require.NotNil(t, ourParentStatus(got, ns, "gw1"), "kept as a cleanup backup while cleanup is pending")

	ep2 := h.register(gw2)
	require.Zero(t, h.settle(route).RequeueAfter)
	require.True(t, h.rest.has(ep2, ns+"-r"))
	require.False(t, h.rest.has(ep1, ns+"-r"))
	got = h.get(route)
	require.Equal(t, deployedGateways{ns + "/gw2": recordOf(t, got)[ns+"/gw2"]}, recordOf(t, got))
	require.Nil(t, ourParentStatus(got, ns, "gw1"))
	require.Equal(t, ns+"/gw2", got.Annotations[AnnHTTPRouteLastDeployedParentGateway])
}

func TestHTTPRouteFailingGatewayDoesNotAffectOthers(t *testing.T) {
	ns := "mg-fail"
	gw1 := mgGateway(ns, "gw1", mgClass, gatewayv1.NamespacesFromAll)
	gw2 := mgGateway(ns, "gw2", mgClass, gatewayv1.NamespacesFromAll)
	gw3 := mgGateway(ns, "gw3", mgClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "gw1"), mgRef(ns, "gw2"), mgRef(ns, "gw3"))
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), gw1, gw2, gw3, route)...)
	ep1, ep2, ep3 := h.register(gw1), h.register(gw2), h.register(gw3)
	h.rest.failDeploy[ep2] = &gatewayclient.RetryableError{Err: errors.New("connection refused"), StatusCode: 503}
	h.rest.failDeploy[ep3] = &gatewayclient.NonRetryableError{Err: errors.New("invalid api"), StatusCode: 400}

	res := h.settle(route)
	require.Equal(t, requeueGatewayRESTRetry, res.RequeueAfter)
	require.True(t, h.rest.has(ep1, ns+"-r"))

	got := h.get(route)
	requireCondition(t, ourParentStatus(got, ns, "gw1"), "Accepted", metav1.ConditionTrue, "Accepted")
	requireCondition(t, ourParentStatus(got, ns, "gw2"), "Accepted", metav1.ConditionFalse, "Retrying")
	requireCondition(t, ourParentStatus(got, ns, "gw3"), "Accepted", metav1.ConditionFalse, "DeploymentFailed")
	require.Equal(t, []string{ns + "/gw1"}, keysOf(recordOf(t, got)))

	// gw2 recovers on retry; gw1 is not redeployed.
	delete(h.rest.failDeploy, ep2)
	h.rest.reset()
	h.reconcile(route)
	require.True(t, h.rest.has(ep2, ns+"-r"))
	require.Zero(t, h.rest.count("deploy", ep1))
	requireCondition(t, ourParentStatus(h.get(route), ns, "gw2"), "Accepted", metav1.ConditionTrue, "Accepted")
}

func keysOf(rec deployedGateways) []string {
	return sortedRecordKeys(rec)
}

func TestHTTPRouteSlowGatewayDoesNotDelayOthers(t *testing.T) {
	ns := "mg-slow"
	var gws []client.Object
	var refs []gatewayv1.ParentReference
	for _, n := range []string{"gw1", "gw2", "gw3", "gw4"} {
		gws = append(gws, mgGateway(ns, n, mgClass, gatewayv1.NamespacesFromAll))
		refs = append(refs, mgRef(ns, n))
	}
	route := mgRoute(ns, "r", "/", refs...)
	h := newRouteHarness(t, ns, nil, append(append(mgObjects(ns), gws...), route)...)
	for _, gw := range gws {
		ep := h.register(gw.(*gatewayv1.Gateway))
		h.rest.delay[ep] = 300 * time.Millisecond
	}
	h.reconcile(route) // finalizer
	start := time.Now()
	h.reconcile(route)
	require.Less(t, time.Since(start), 900*time.Millisecond, "Gateways must be deployed in parallel")
	require.Len(t, recordOf(t, h.get(route)), 4)
}

func TestForEachBoundedLimitsConcurrency(t *testing.T) {
	var mu sync.Mutex
	running, peak := 0, 0
	forEachBounded(10, httpRouteGatewayConcurrency, func(int) {
		mu.Lock()
		running++
		if running > peak {
			peak = running
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		running--
		mu.Unlock()
	})
	require.Equal(t, httpRouteGatewayConcurrency, peak)
}

// --- Gateways of other classes ---

func TestHTTPRouteOtherClassParentGetsNoStatusFromUs(t *testing.T) {
	ns := "mg-mixed"
	ours := mgGateway(ns, "ours", mgClass, gatewayv1.NamespacesFromAll)
	theirs := mgGateway(ns, "theirs", mgOtherClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "ours"), mgRef(ns, "theirs"))
	// What the released operator left behind: a rejection written under our name on both parents,
	// next to the other controller's own entry.
	invalid := metav1.Condition{Type: "ResolvedRefs", Status: metav1.ConditionFalse, Reason: "Invalid",
		Message: "HTTPRoute with multiple distinct Gateway parentRefs is not supported", LastTransitionTime: metav1.Now()}
	route.Status.Parents = []gatewayv1.RouteParentStatus{
		{ParentRef: mgRef(ns, "ours"), ControllerName: PlatformGatewayControllerName, Conditions: []metav1.Condition{invalid}},
		{ParentRef: mgRef(ns, "theirs"), ControllerName: PlatformGatewayControllerName, Conditions: []metav1.Condition{invalid}},
		{ParentRef: mgRef(ns, "theirs"), ControllerName: "istio.io/gateway-controller", Conditions: []metav1.Condition{
			{Type: "Accepted", Status: metav1.ConditionTrue, Reason: "Accepted", LastTransitionTime: metav1.Now()}}},
	}
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), ours, theirs, route)...)
	ep := h.register(ours)
	h.settle(route)

	require.True(t, h.rest.has(ep, ns+"-r"))
	got := h.get(route)
	requireCondition(t, ourParentStatus(got, ns, "ours"), "Accepted", metav1.ConditionTrue, "Accepted")
	requireCondition(t, ourParentStatus(got, ns, "ours"), "ResolvedRefs", metav1.ConditionTrue, "ResolvedRefs")
	require.Nil(t, ourParentStatus(got, ns, "theirs"), "no status from us on another class's Gateway")
	require.Len(t, got.Status.Parents, 2)
	theirsKept := false
	for _, ps := range got.Status.Parents {
		if ps.ControllerName == "istio.io/gateway-controller" {
			theirsKept = true
		}
	}
	require.True(t, theirsKept, "the other controller's entry is untouched")
}

func TestHTTPRouteOnlyOtherClassParentsIsLeftAlone(t *testing.T) {
	ns := "mg-foreign"
	theirs := mgGateway(ns, "theirs", mgOtherClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "theirs"))
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), theirs, route)...)
	before := h.get(route)
	require.Zero(t, h.settle(route).RequeueAfter)
	after := h.get(route)
	require.Equal(t, before.ResourceVersion, after.ResourceVersion, "a route that isn't ours must not be written to")
	require.Empty(t, after.Finalizers)
}

func TestHTTPRouteMovedToOtherClassGatewayIsCleanedUp(t *testing.T) {
	ns := "mg-toforeign"
	ours := mgGateway(ns, "ours", mgClass, gatewayv1.NamespacesFromAll)
	theirs := mgGateway(ns, "theirs", mgOtherClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "ours"))
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), ours, theirs, route)...)
	ep := h.register(ours)
	h.settle(route)

	h.update(route, func(r *gatewayv1.HTTPRoute) { r.Spec.ParentRefs = []gatewayv1.ParentReference{mgRef(ns, "theirs")} })
	h.settle(route)
	require.False(t, h.rest.has(ep, ns+"-r"))
	got := h.get(route)
	require.Empty(t, got.Annotations[AnnHTTPRouteDeployedGateways])
	require.Empty(t, got.Annotations[AnnHTTPRouteLastDeployedParentGateway])
	require.Nil(t, ourParentStatus(got, ns, "ours"))
	require.Nil(t, ourParentStatus(got, ns, "theirs"))
}

// --- Deletion ---

// Reproduces the stuck deletion: once a route moved to another class's Gateway, deleting it
// waited forever for that Gateway to register with us.
func TestHTTPRouteDeletionAfterMoveToOtherClassGateway(t *testing.T) {
	ns := "mg-delforeign"
	theirs := mgGateway(ns, "theirs", mgOtherClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "theirs"))
	route.Finalizers = []string{httprouteFinalizer}
	now := metav1.Now()
	route.DeletionTimestamp = &now
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), theirs, route)...)

	res := h.reconcile(route)
	require.Zero(t, res.RequeueAfter)
	err := h.cl.Get(context.Background(), client.ObjectKeyFromObject(route), &gatewayv1.HTTPRoute{})
	require.True(t, apierrors.IsNotFound(err), "finalizer must be removed so the route goes away, got %v", err)
}

func TestHTTPRouteDeletionRemovesFromEveryGateway(t *testing.T) {
	ns := "mg-delall"
	gw1 := mgGateway(ns, "gw1", mgClass, gatewayv1.NamespacesFromAll)
	gw2 := mgGateway(ns, "gw2", mgClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "gw1"), mgRef(ns, "gw2"))
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), gw1, gw2, route)...)
	ep1, ep2 := h.register(gw1), h.register(gw2)
	h.settle(route)

	// One gateway-controller fails the delete: the finalizer stays until it succeeds.
	h.rest.failDelete[ep2] = errors.New("connection refused")
	require.NoError(t, h.cl.Delete(context.Background(), h.get(route)))
	_, err := h.r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(route)})
	require.Error(t, err)
	require.Contains(t, h.get(route).Finalizers, httprouteFinalizer)
	require.False(t, h.rest.has(ep1, ns+"-r"))

	delete(h.rest.failDelete, ep2)
	h.reconcile(route)
	require.False(t, h.rest.has(ep2, ns+"-r"))
	err = h.cl.Get(context.Background(), client.ObjectKeyFromObject(route), &gatewayv1.HTTPRoute{})
	require.True(t, apierrors.IsNotFound(err))
}

func TestHTTPRouteDeletionSkipsGatewayBeingDeleted(t *testing.T) {
	ns := "mg-delgw"
	gw1 := mgGateway(ns, "gw1", mgClass, gatewayv1.NamespacesFromAll)
	gw2 := mgGateway(ns, "gw2", mgClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "gw1"), mgRef(ns, "gw2"))
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), gw1, gw2, route)...)
	ep1 := h.register(gw1)
	h.register(gw2)
	h.settle(route)

	// gw2 is being torn down: the Gateway controller has already unregistered it.
	gw := &gatewayv1.Gateway{}
	require.NoError(t, h.cl.Get(context.Background(), client.ObjectKeyFromObject(gw2), gw))
	gw.Finalizers = []string{"gateway.api-platform.wso2.com/test"}
	require.NoError(t, h.cl.Update(context.Background(), gw))
	require.NoError(t, h.cl.Delete(context.Background(), gw))
	registry.GetGatewayRegistry().Unregister(ns, "gw2")

	require.NoError(t, h.cl.Delete(context.Background(), h.get(route)))
	require.Zero(t, h.reconcile(route).RequeueAfter)
	require.False(t, h.rest.has(ep1, ns+"-r"))
	err := h.cl.Get(context.Background(), client.ObjectKeyFromObject(route), &gatewayv1.HTTPRoute{})
	require.True(t, apierrors.IsNotFound(err))
}

func TestHTTPRouteDeletionWaitsForOwnGatewayToRegister(t *testing.T) {
	ns := "mg-delwait"
	gw1 := mgGateway(ns, "gw1", mgClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "gw1"))
	route.Finalizers = []string{httprouteFinalizer}
	route.Annotations = map[string]string{AnnHTTPRouteLastDeployedParentGateway: ns + "/gw1"}
	now := metav1.Now()
	route.DeletionTimestamp = &now
	// e.g. right after an operator restart, before the Gateway controller re-registers gw1.
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), gw1, route)...)

	require.Equal(t, requeueGatewayNotRegistered, h.reconcile(route).RequeueAfter)
	require.Contains(t, h.get(route).Finalizers, httprouteFinalizer)

	ep := h.register(gw1)
	h.reconcile(route)
	require.Equal(t, 1, h.rest.count("delete", ep))
	err := h.cl.Get(context.Background(), client.ObjectKeyFromObject(route), &gatewayv1.HTTPRoute{})
	require.True(t, apierrors.IsNotFound(err))
}

// --- Bookkeeping edge cases ---

func TestHTTPRouteHandleChangeRemovesOldHandle(t *testing.T) {
	ns := "mg-handle"
	gw1 := mgGateway(ns, "gw1", mgClass, gatewayv1.NamespacesFromAll)
	gw2 := mgGateway(ns, "gw2", mgClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "gw1"), mgRef(ns, "gw2"))
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), gw1, gw2, route)...)
	ep1, ep2 := h.register(gw1), h.register(gw2)
	h.settle(route)

	h.update(route, func(r *gatewayv1.HTTPRoute) { r.Annotations[AnnHTTPRouteAPIHandle] = "orders-v2" })
	h.settle(route)
	for _, ep := range []string{ep1, ep2} {
		require.True(t, h.rest.has(ep, "orders-v2"))
		require.False(t, h.rest.has(ep, ns+"-r"), "old handle must be removed")
	}
	require.Equal(t, "orders-v2", recordOf(t, h.get(route))[ns+"/gw1"].Handle)
	require.Equal(t, 1, h.rest.count("delete", ep1), "old handle removed once")

	// Deleting the route removes the current handle.
	require.NoError(t, h.cl.Delete(context.Background(), h.get(route)))
	h.reconcile(route)
	require.False(t, h.rest.has(ep1, "orders-v2"))
	require.False(t, h.rest.has(ep2, "orders-v2"))
}

func TestHTTPRouteLostRecordFallsBackToStatus(t *testing.T) {
	ns := "mg-lost"
	gw1 := mgGateway(ns, "gw1", mgClass, gatewayv1.NamespacesFromAll)
	gw2 := mgGateway(ns, "gw2", mgClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "gw1"), mgRef(ns, "gw2"))
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), gw1, gw2, route)...)
	_, ep2 := h.register(gw1), h.register(gw2)
	h.settle(route)

	// A GitOps sync wipes the operator's annotations and drops gw2 from the route.
	h.update(route, func(r *gatewayv1.HTTPRoute) {
		r.Annotations = nil
		r.Spec.ParentRefs = r.Spec.ParentRefs[:1]
	})
	h.settle(route)
	require.False(t, h.rest.has(ep2, ns+"-r"), "status entry must still lead cleanup to gw2")
	require.Nil(t, ourParentStatus(h.get(route), ns, "gw2"))
}

func TestHTTPRouteParentGatewayNotFound(t *testing.T) {
	ns := "mg-missing"
	gw1 := mgGateway(ns, "gw1", mgClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "gw1"), mgRef(ns, "ghost"))
	h := newRouteHarness(t, ns, nil, append(mgObjects(ns), gw1, route)...)
	ep1 := h.register(gw1)
	res := h.settle(route)
	require.Equal(t, requeueParentGatewayMissing, res.RequeueAfter)
	require.True(t, h.rest.has(ep1, ns+"-r"), "a missing parent doesn't block the others")
	require.Nil(t, ourParentStatus(h.get(route), ns, "ghost"), "no status for a Gateway whose class is unknown")
}

func TestHTTPRouteNotAllowedByListenerIsNotDeployed(t *testing.T) {
	ns := "mg-notallowed"
	gwNs := "mg-notallowed-infra"
	gw := mgGateway(gwNs, "gw", mgClass, gatewayv1.NamespacesFromSame)
	route := mgRoute(ns, "r", "/", mgRef(gwNs, "gw"))
	h := newRouteHarness(t, ns, nil, append(append(mgObjects(ns), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gwNs}}), gw, route)...)
	ep := h.register(gw)
	require.Zero(t, h.settle(route).RequeueAfter)
	require.Zero(t, h.rest.count("deploy", ep))
	requireCondition(t, ourParentStatus(h.get(route), gwNs, "gw"), "Accepted", metav1.ConditionFalse, string(gatewayv1.RouteReasonNotAllowedByListeners))
}

// A Gateway whose listeners stop accepting the route is cleaned up at once, even while another
// parent is still pending or failed; only moves wait for the other parents to settle.
func TestHTTPRouteRevokedGatewayIsCleanedUpWithoutWaiting(t *testing.T) {
	for _, tc := range []struct {
		name  string
		other string // "", "pending" or "failed"
	}{
		{name: "other parent pending", other: "pending"},
		{name: "other parent failed", other: "failed"},
		{name: "only parent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ns := "mg-revoke"
			gwNs := "mg-revoke-infra"
			gwA := mgGateway(gwNs, "gw-a", mgClass, gatewayv1.NamespacesFromAll)
			gwB := mgGateway(gwNs, "gw-b", mgClass, gatewayv1.NamespacesFromAll)
			refs := []gatewayv1.ParentReference{mgRef(gwNs, "gw-a")}
			if tc.other != "" {
				refs = append(refs, mgRef(gwNs, "gw-b"))
			}
			route := mgRoute(ns, "r", "/", refs...)
			objs := append(mgObjects(ns), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gwNs}}, gwA, gwB, route)
			h := newRouteHarness(t, ns, nil, objs...)
			epA := h.register(gwA)
			if tc.other == "failed" {
				epB := h.register(gwB)
				h.rest.failDeploy[epB] = &gatewayclient.NonRetryableError{Err: errors.New("configuration already exists"), StatusCode: 409}
			}
			h.settle(route)
			require.True(t, h.rest.has(epA, ns+"-r"))

			// gw-a's owner stops allowing routes from other namespaces.
			gw := &gatewayv1.Gateway{}
			require.NoError(t, h.cl.Get(context.Background(), client.ObjectKeyFromObject(gwA), gw))
			same := gatewayv1.NamespacesFromSame
			gw.Spec.Listeners[0].AllowedRoutes.Namespaces.From = &same
			require.NoError(t, h.cl.Update(context.Background(), gw))

			h.reconcile(route)
			require.False(t, h.rest.has(epA, ns+"-r"), "a Gateway that rejects the route must stop serving it")
			got := h.get(route)
			require.NotContains(t, recordOf(t, got), gwNs+"/gw-a")
			requireCondition(t, ourParentStatus(got, gwNs, "gw-a"), "Accepted", metav1.ConditionFalse, string(gatewayv1.RouteReasonNotAllowedByListeners))
		})
	}
}

// --- Status writes ---

func TestHTTPRouteStatusConflictKeepsOtherControllersEntry(t *testing.T) {
	ns := "mg-conflict"
	gw1 := mgGateway(ns, "gw1", mgClass, gatewayv1.NamespacesFromAll)
	route := mgRoute(ns, "r", "/", mgRef(ns, "gw1"))
	route.Finalizers = []string{httprouteFinalizer}
	injected := false
	funcs := &interceptor.Funcs{
		SubResourcePatch: func(ctx context.Context, c client.Client, sub string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			if !injected {
				// Another controller writes its own entry between our read and our patch.
				injected = true
				other := &gatewayv1.HTTPRoute{}
				if err := c.Get(ctx, client.ObjectKeyFromObject(obj), other); err != nil {
					return err
				}
				other.Status.Parents = append(other.Status.Parents, gatewayv1.RouteParentStatus{
					ParentRef: mgRef(ns, "elsewhere"), ControllerName: "example.com/other",
					Conditions: []metav1.Condition{{Type: "Accepted", Status: metav1.ConditionTrue, Reason: "Accepted", LastTransitionTime: metav1.Now()}},
				})
				if err := c.Status().Update(ctx, other); err != nil {
					return err
				}
			}
			return c.SubResource(sub).Patch(ctx, obj, patch, opts...)
		},
	}
	h := newRouteHarness(t, ns, funcs, append(mgObjects(ns), gw1, route)...)
	h.register(gw1)
	h.settle(route)

	got := h.get(route)
	require.True(t, injected)
	requireCondition(t, ourParentStatus(got, ns, "gw1"), "Accepted", metav1.ConditionTrue, "Accepted")
	found := false
	for _, ps := range got.Status.Parents {
		if ps.ControllerName == "example.com/other" {
			found = true
		}
	}
	require.True(t, found, "the other controller's entry must survive our status write")
}

// --- Units ---

func TestGroupParentTargetsByGateway(t *testing.T) {
	route := &gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Namespace: "apps"}}
	sec := func(s string) *gatewayv1.SectionName { v := gatewayv1.SectionName(s); return &v }
	route.Spec.ParentRefs = []gatewayv1.ParentReference{
		{Name: "b", SectionName: sec("http")},
		{Name: "a", Namespace: goldenStr[gatewayv1.Namespace]("infra")},
		{Name: "b", SectionName: sec("https")},
		{Name: "b", Namespace: goldenStr[gatewayv1.Namespace]("infra")},
		{Name: "svc", Kind: goldenStr[gatewayv1.Kind]("Service")},
	}
	groups := groupParentTargetsByGateway(parentGatewayRefs(route))
	require.Len(t, groups, 3)
	require.Equal(t, client.ObjectKey{Namespace: "apps", Name: "b"}, groups[0].key)
	require.Len(t, groups[0].targets, 2, "two sections of one Gateway share a group")
	require.Equal(t, client.ObjectKey{Namespace: "infra", Name: "a"}, groups[1].key)
	require.Equal(t, client.ObjectKey{Namespace: "infra", Name: "b"}, groups[2].key, "same name in another namespace is another Gateway")
}

func TestDeployedGatewaysFromRoute(t *testing.T) {
	mk := func(ann map[string]string) *gatewayv1.HTTPRoute {
		return &gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "r", Annotations: ann}}
	}
	require.Empty(t, deployedGatewaysFromRoute(mk(nil)))

	legacy := map[string]string{AnnHTTPRouteLastDeployedParentGateway: "infra/gw", AnnHTTPRouteLastDeployedConfigHash: "abc"}
	require.Equal(t, deployedGateways{"infra/gw": {Hash: "abc", Handle: "apps-r"}}, deployedGatewaysFromRoute(mk(legacy)))

	withRecord := map[string]string{
		AnnHTTPRouteDeployedGateways:          `{"infra/a":{"hash":"1","handle":"h"},"bad-key":{"hash":"2"},"x/y/z":{}}`,
		AnnHTTPRouteLastDeployedParentGateway: "infra/gw",
		AnnHTTPRouteLastDeployedConfigHash:    "abc",
	}
	require.Equal(t, deployedGateways{"infra/a": {Hash: "1", Handle: "h"}, "infra/gw": {Hash: "abc", Handle: "apps-r"}},
		deployedGatewaysFromRoute(mk(withRecord)), "both are merged, and unreadable keys are ignored")

	sameGateway := map[string]string{
		AnnHTTPRouteDeployedGateways:          `{"infra/gw":{"hash":"1","handle":"old"}}`,
		AnnHTTPRouteLastDeployedParentGateway: "infra/gw",
		AnnHTTPRouteLastDeployedConfigHash:    "abc",
	}
	require.Equal(t, deployedGateways{"infra/gw": {Hash: "1", Handle: "old"}}, deployedGatewaysFromRoute(mk(sameGateway)),
		"for the same Gateway the multi-Gateway entry wins, since it carries the handle")

	broken := map[string]string{AnnHTTPRouteDeployedGateways: `{not json`}
	for k, v := range legacy {
		broken[k] = v
	}
	require.Equal(t, deployedGateways{"infra/gw": {Hash: "abc", Handle: "apps-r"}}, deployedGatewaysFromRoute(mk(broken)),
		"invalid JSON is ignored")
}

func TestDeployedGatewaysAnnotations(t *testing.T) {
	ptr := func(m map[string]*string, k string) (string, bool, bool) {
		v, ok := m[k]
		if !ok {
			return "", false, false
		}
		if v == nil {
			return "", true, true
		}
		return *v, true, false
	}

	none := deployedGatewaysAnnotations(deployedGateways{}, "h")
	_, set, removed := ptr(none, AnnHTTPRouteDeployedGateways)
	require.True(t, set && removed)
	_, set, removed = ptr(none, AnnHTTPRouteLastDeployedParentGateway)
	require.True(t, set && removed)
	_, set, _ = ptr(none, AnnHTTPRouteLastDeployedConfigHash)
	require.False(t, set, "the legacy hash is left alone, as before")

	one := deployedGatewaysAnnotations(deployedGateways{"infra/gw": {Hash: "abc", Handle: "h"}}, "h")
	_, _, removed = ptr(one, AnnHTTPRouteDeployedGateways)
	require.True(t, removed, "one Gateway: the legacy pair is the whole record")
	v, _, _ := ptr(one, AnnHTTPRouteLastDeployedParentGateway)
	require.Equal(t, "infra/gw", v)
	v, _, _ = ptr(one, AnnHTTPRouteLastDeployedConfigHash)
	require.Equal(t, "abc", v)

	pending := deployedGatewaysAnnotations(deployedGateways{"infra/gw": {Hash: "abc", Handle: "old"}}, "new")
	v, _, _ = ptr(pending, AnnHTTPRouteDeployedGateways)
	require.JSONEq(t, `{"infra/gw":{"hash":"abc","handle":"old"}}`, v, "an old handle still to be removed must be remembered")
	v, _, _ = ptr(pending, AnnHTTPRouteLastDeployedParentGateway)
	require.Equal(t, "infra/gw", v)

	two := deployedGatewaysAnnotations(deployedGateways{"infra/b": {Hash: "2"}, "infra/a": {Hash: "1"}}, "h")
	v, _, _ = ptr(two, AnnHTTPRouteDeployedGateways)
	require.Equal(t, `{"infra/a":{"hash":"1"},"infra/b":{"hash":"2"}}`, v, "stable key order")
	_, _, removed = ptr(two, AnnHTTPRouteLastDeployedParentGateway)
	require.True(t, removed)
	_, _, removed = ptr(two, AnnHTTPRouteLastDeployedConfigHash)
	require.True(t, removed)
}

func TestIsOperatorBookkeepingAnnotationCoversRecord(t *testing.T) {
	require.True(t, isOperatorBookkeepingAnnotation(AnnHTTPRouteDeployedGateways))
	md := payloadMetadataForHTTPRoute(&gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		AnnHTTPRouteDeployedGateways: `{"a/b":{}}`, "example.com/x": "y",
	}}}, "h")
	require.Equal(t, map[string]string{"example.com/x": "y"}, md.Annotations)
}

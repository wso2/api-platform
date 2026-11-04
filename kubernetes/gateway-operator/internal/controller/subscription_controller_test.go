/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the Apache License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/registry"
)

// subscriptionListServer serves GET /subscriptions?apiId=... with the subscriptions each API holds.
func subscriptionListServer(t *testing.T, byAPI map[string][]map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, gatewayclient.ManagementAPIBasePath+"/subscriptions", r.URL.Path)
		items := byAPI[r.URL.Query().Get("apiId")]
		if items == nil {
			items = []map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"subscriptions": items, "count": len(items)})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func subscriptionCR(apiID string, externalRefID *string) *apiv1.Subscription {
	cr := &apiv1.Subscription{}
	cr.Spec.ApiId = apiID
	cr.Spec.ExternalRefId = externalRefID
	return cr
}

func TestRecoverSubscriptionIDAfterConflict_AdoptsExternalRefIdOnTheSameAPI(t *testing.T) {
	srv := subscriptionListServer(t, map[string][]map[string]any{
		"orders-api": {{"id": "other-sub", "externalRefId": "other-sub"}, {"id": "acme-sub-01", "externalRefId": "acme-sub-01"}},
	})
	ref := "acme-sub-01"

	id, err := recoverSubscriptionIDAfterConflict(context.Background(), srv.URL, subscriptionCR("orders-api", &ref), nil)
	require.NoError(t, err)
	require.Equal(t, "acme-sub-01", id)
}

// A subscription with the same ID on another API must never be adopted: the follow-up update would
// overwrite that subscription's plan and token. The error must not say whether the ID or the token
// collided, so it never confirms that a token is in use.
func TestRecoverSubscriptionIDAfterConflict_RejectsExternalRefIdNotOnTheAPI(t *testing.T) {
	srv := subscriptionListServer(t, map[string][]map[string]any{
		"orders-api":   {{"id": "other-sub", "externalRefId": "other-sub"}},
		"payments-api": {{"id": "acme-sub-01", "externalRefId": "acme-sub-01"}},
	})
	ref := "acme-sub-01"

	id, err := recoverSubscriptionIDAfterConflict(context.Background(), srv.URL, subscriptionCR("orders-api", &ref), nil)
	require.Empty(t, id)
	var nonRetryable *gatewayclient.NonRetryableError
	require.ErrorAs(t, err, &nonRetryable)
	require.Equal(t, http.StatusConflict, nonRetryable.StatusCode)
	require.Contains(t, err.Error(), "conflicts with an existing subscription")
	require.NotContains(t, err.Error(), "already used")
	require.NotContains(t, err.Error(), "payments-api")
}

func TestRecoverSubscriptionIDAfterConflict_WithoutExternalRefIdUsesTheAPIAndApplication(t *testing.T) {
	srv := subscriptionListServer(t, map[string][]map[string]any{
		"orders-api": {{"id": "generated-uuid"}},
	})

	id, err := recoverSubscriptionIDAfterConflict(context.Background(), srv.URL, subscriptionCR("orders-api", nil), nil)
	require.NoError(t, err)
	require.Equal(t, "generated-uuid", id)

	empty := ""
	id, err = recoverSubscriptionIDAfterConflict(context.Background(), srv.URL, subscriptionCR("orders-api", &empty), nil)
	require.NoError(t, err)
	require.Equal(t, "generated-uuid", id, "an empty externalRefId behaves as if it were not set")
}

// An externalRefId subscription on the same API but for another application belongs to another CR, so it
// must not be adopted and overwritten.
func TestRecoverSubscriptionIDAfterConflict_RequiresTheCRApplication(t *testing.T) {
	srv := subscriptionListServer(t, map[string][]map[string]any{
		"orders-api": {{"id": "acme-sub-01", "externalRefId": "acme-sub-01", "applicationId": "app-x"}},
	})
	ref, app := "acme-sub-01", "app-1"

	cr := subscriptionCR("orders-api", &ref)
	cr.Spec.ApplicationId = &app
	id, err := recoverSubscriptionIDAfterConflict(context.Background(), srv.URL, cr, nil)
	require.Empty(t, id)
	var nonRetryable *gatewayclient.NonRetryableError
	require.ErrorAs(t, err, &nonRetryable)
	require.Equal(t, http.StatusConflict, nonRetryable.StatusCode)

	app = "app-x"
	id, err = recoverSubscriptionIDAfterConflict(context.Background(), srv.URL, cr, nil)
	require.NoError(t, err)
	require.Equal(t, "acme-sub-01", id)
}

// Without externalRefId, the only subscription on the API belonging to another application yields no id,
// so Deploy takes its retryable conflict path instead of updating that subscription.
func TestRecoverSubscriptionIDAfterConflict_DoesNotAdoptAnotherApplicationsSubscription(t *testing.T) {
	srv := subscriptionListServer(t, map[string][]map[string]any{
		"orders-api": {{"id": "sub-x", "applicationId": "app-x"}},
	})

	id, err := recoverSubscriptionIDAfterConflict(context.Background(), srv.URL, subscriptionCR("orders-api", nil), nil)
	require.NoError(t, err)
	require.Empty(t, id)
}

// Gateways that predate externalRefId ignore it, assign their own ids and leave externalRefId out of their
// responses, so the subscription is recovered by API and application, as it was before externalRefId.
func TestRecoverSubscriptionIDAfterConflict_FallsBackOnGatewaysWithoutExternalRefId(t *testing.T) {
	srv := subscriptionListServer(t, map[string][]map[string]any{
		"orders-api": {{"id": "gateway-assigned-uuid"}},
	})
	ref := "acme-sub-01"

	id, err := recoverSubscriptionIDAfterConflict(context.Background(), srv.URL, subscriptionCR("orders-api", &ref), nil)
	require.NoError(t, err)
	require.Equal(t, "gateway-assigned-uuid", id)
}

// With no subscription on the API there is nothing to recover, whichever gateway version answered.
func TestRecoverSubscriptionIDAfterConflict_RejectsExternalRefIdWhenTheAPIHasNoSubscriptions(t *testing.T) {
	srv := subscriptionListServer(t, map[string][]map[string]any{})
	ref := "acme-sub-01"

	id, err := recoverSubscriptionIDAfterConflict(context.Background(), srv.URL, subscriptionCR("orders-api", &ref), nil)
	require.Empty(t, id)
	var nonRetryable *gatewayclient.NonRetryableError
	require.ErrorAs(t, err, &nonRetryable)
	require.Equal(t, http.StatusConflict, nonRetryable.StatusCode)
}

func TestSubscriptionIDOwnedByAnotherCR(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))

	reg := registry.GetGatewayRegistry()
	shared := &registry.GatewayInfo{
		Name: "owner-check-shared", Namespace: "gw", ServiceName: "shared", ServicePort: 9090,
		APISelector: &apiv1.APISelector{Scope: apiv1.NamespacedScope, Namespaces: []string{"owner-team-a", "owner-team-b"}},
	}
	other := &registry.GatewayInfo{
		Name: "owner-check-other", Namespace: "gw", ServiceName: "other", ServicePort: 9090,
		APISelector: &apiv1.APISelector{Scope: apiv1.NamespacedScope, Namespaces: []string{"owner-team-c"}},
	}
	reg.Register(shared)
	reg.Register(other)
	t.Cleanup(func() {
		reg.Unregister(shared.Namespace, shared.Name)
		reg.Unregister(other.Namespace, other.Name)
	})

	deployed := func(ns, name, id string) *apiv1.Subscription {
		cr := &apiv1.Subscription{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}}
		cr.Status.Id = id
		return cr
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&apiv1.Subscription{}).WithObjects(
		deployed("owner-team-a", "orders", "acme-sub-01"),
		deployed("owner-team-c", "orders", "acme-sub-02"),
	).Build()
	endpoint := shared.GetGatewayServiceEndpoint()

	owned, err := subscriptionIDOwnedByAnotherCR(ctx, c, endpoint, deployed("owner-team-b", "orders", ""), "acme-sub-01")
	require.NoError(t, err)
	require.True(t, owned, "a CR in another namespace on the same gateway already manages the subscription")

	owned, err = subscriptionIDOwnedByAnotherCR(ctx, c, endpoint, deployed("owner-team-a", "orders", ""), "acme-sub-01")
	require.NoError(t, err)
	require.False(t, owned, "the CR's own status.id is not another CR's")

	owned, err = subscriptionIDOwnedByAnotherCR(ctx, c, endpoint, deployed("owner-team-b", "orders", ""), "acme-sub-02")
	require.NoError(t, err)
	require.False(t, owned, "the same id on a different gateway is a different subscription")
}

// subscriptionGatewayServer serves the subscription list, answers every create with 409 and records every
// PUT body. putStatuses gives the status of successive PUTs; once exhausted, PUTs succeed and report the
// plan they were sent, as a gateway with in-place plan updates does.
func subscriptionGatewayServer(t *testing.T, byAPI map[string][]map[string]any, putStatuses ...int) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var puts []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items := byAPI[r.URL.Query().Get("apiId")]
			if items == nil {
				items = []map[string]any{}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"subscriptions": items, "count": len(items)})
		case http.MethodPut:
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			puts = append(puts, body)
			if len(puts) <= len(putStatuses) && putStatuses[len(puts)-1] != http.StatusOK {
				w.WriteHeader(putStatuses[len(puts)-1])
				_, _ = w.Write([]byte(`{"status":"error","message":"rejected"}`))
				return
			}
			id := path.Base(r.URL.Path)
			resp := map[string]any{"id": id, "externalRefId": id}
			if plan, ok := body["subscriptionPlanId"]; ok && plan != nil {
				resp["subscriptionPlanId"] = plan
			}
			_ = json.NewEncoder(w).Encode(resp)
		case http.MethodPost:
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"status":"error","message":"conflict"}`))
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &puts
}

// deployedSubscription is a CR already on the gateway as acme-sub-01. With baseline set, it carries the
// immutable-field fingerprint an earlier reconcile stored.
func deployedSubscription(t *testing.T, baseline bool) *apiv1.Subscription {
	t.Helper()
	token, status, plan := "token-1", "REVOKED", "gold-plan-uuid"
	cr := &apiv1.Subscription{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "orders"}}
	cr.Spec.ApiId = "orders-api"
	cr.Spec.SubscriptionToken.Value = &token
	cr.Spec.Status = &status
	cr.Spec.SubscriptionPlanId = &plan
	cr.Status.Id = "acme-sub-01"
	if baseline {
		fp, err := subscriptionImmutableFingerprint(cr)
		require.NoError(t, err)
		cr.Annotations = map[string]string{annSubscriptionLastAppliedImmutableFingerprint: fp}
	}
	return cr
}

func subscriptionTestClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func TestSubscriptionDeploy_UpdatesStatusPlanAndTokenInOneRequest(t *testing.T) {
	cr := deployedSubscription(t, true)
	srv, puts := subscriptionGatewayServer(t, nil)

	res, err := (&subscriptionAdapter{}).Deploy(context.Background(), subscriptionTestClient(t, cr), srv.URL, cr, nil)
	require.NoError(t, err)
	require.Equal(t, "acme-sub-01", res.Id)
	require.Equal(t, []map[string]any{{
		"status": "REVOKED", "subscriptionPlanId": "gold-plan-uuid", "subscriptionToken": "token-1",
	}}, *puts)
}

// A plan or token the gateway rejects must not stop the status, so a subscription can still be revoked.
func TestSubscriptionDeploy_AppliesTheStatusWhenThePlanOrTokenIsRejected(t *testing.T) {
	for _, code := range []int{http.StatusBadRequest, http.StatusConflict} {
		cr := deployedSubscription(t, true)
		srv, puts := subscriptionGatewayServer(t, nil, code)

		_, err := (&subscriptionAdapter{}).Deploy(context.Background(), subscriptionTestClient(t, cr), srv.URL, cr, nil)
		var nonRetryable *gatewayclient.NonRetryableError
		require.ErrorAs(t, err, &nonRetryable)
		require.Equal(t, code, nonRetryable.StatusCode, "the CR still reports why the full update failed")
		require.Len(t, *puts, 2)
		require.Equal(t, map[string]any{"status": "REVOKED"}, (*puts)[1], "only the status is sent again")
	}
}

// A missing subscription is not a plan or token problem, so no status-only update follows.
func TestSubscriptionDeploy_DoesNotRetryTheStatusWhenTheSubscriptionIsMissing(t *testing.T) {
	cr := deployedSubscription(t, true)
	srv, puts := subscriptionGatewayServer(t, nil, http.StatusNotFound)

	_, err := (&subscriptionAdapter{}).Deploy(context.Background(), subscriptionTestClient(t, cr), srv.URL, cr, nil)
	require.Error(t, err)
	require.Len(t, *puts, 1)
}

func TestSubscriptionDeploy_AppliesTheStatusWhenTheTokenSecretIsMissing(t *testing.T) {
	cr := deployedSubscription(t, true)
	cr.Spec.SubscriptionToken = apiv1.SecretValueSource{ValueFrom: &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "missing-secret"}, Key: "token",
	}}
	fp, err := subscriptionImmutableFingerprint(cr)
	require.NoError(t, err)
	cr.Annotations[annSubscriptionLastAppliedImmutableFingerprint] = fp
	srv, puts := subscriptionGatewayServer(t, nil)

	_, err = (&subscriptionAdapter{}).Deploy(context.Background(), subscriptionTestClient(t, cr), srv.URL, cr, nil)
	require.Error(t, err)
	require.Equal(t, []map[string]any{{"status": "REVOKED"}}, *puts)
}

// A CR without a stored fingerprint, such as one from an earlier operator version, takes its spec as the
// baseline once the gateway subscription matches it.
func TestSubscriptionDeploy_SetsTheBaselineWhenTheGatewaySubscriptionMatches(t *testing.T) {
	cr := deployedSubscription(t, false)
	cr.Annotations = map[string]string{annSubscriptionLegacyCreateFingerprint: "legacy"}
	srv, puts := subscriptionGatewayServer(t, map[string][]map[string]any{
		"orders-api": {{"id": "acme-sub-01"}},
	})
	c := subscriptionTestClient(t, cr)

	_, err := (&subscriptionAdapter{}).Deploy(context.Background(), c, srv.URL, cr, nil)
	require.NoError(t, err)
	require.Len(t, *puts, 1)

	var stored apiv1.Subscription
	require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(cr), &stored))
	require.NotEmpty(t, stored.Annotations[annSubscriptionLastAppliedImmutableFingerprint])
	require.NotContains(t, stored.Annotations, annSubscriptionLegacyCreateFingerprint)
}

// An edit an earlier operator version rejected, here a changed apiId, must still be rejected: the
// subscription is not on the API the spec now names, so nothing is updated and no baseline is stored.
func TestSubscriptionDeploy_RejectsASpecThatNoLongerMatchesTheGatewaySubscription(t *testing.T) {
	app := "app-1"
	cases := map[string]struct {
		byAPI  map[string][]map[string]any
		mutate func(*apiv1.Subscription)
	}{
		"apiId changed": {
			byAPI: map[string][]map[string]any{"payments-api": {{"id": "acme-sub-01"}}},
		},
		"billingCustomerId changed": {
			byAPI: map[string][]map[string]any{"orders-api": {{"id": "acme-sub-01", "billingCustomerId": "cus_1"}}},
			mutate: func(cr *apiv1.Subscription) {
				customer := "cus_2"
				cr.Spec.BillingCustomerId = &customer
			},
		},
		"billingSubscriptionId added": {
			byAPI: map[string][]map[string]any{"orders-api": {{"id": "acme-sub-01"}}},
			mutate: func(cr *apiv1.Subscription) {
				billing := "sub_1"
				cr.Spec.BillingSubscriptionId = &billing
			},
		},
		"applicationId changed": {
			byAPI:  map[string][]map[string]any{"orders-api": {{"id": "acme-sub-01"}}},
			mutate: func(cr *apiv1.Subscription) { cr.Spec.ApplicationId = &app },
		},
		"externalRefId added": {
			byAPI: map[string][]map[string]any{"orders-api": {{"id": "acme-sub-01", "externalRefId": "acme-sub-01"}}},
			mutate: func(cr *apiv1.Subscription) {
				ref := "acme-sub-02"
				cr.Spec.ExternalRefId = &ref
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cr := deployedSubscription(t, false)
			if tc.mutate != nil {
				tc.mutate(cr)
			}
			srv, puts := subscriptionGatewayServer(t, tc.byAPI)
			c := subscriptionTestClient(t, cr)

			_, err := (&subscriptionAdapter{}).Deploy(context.Background(), c, srv.URL, cr, nil)
			var nonRetryable *gatewayclient.NonRetryableError
			require.ErrorAs(t, err, &nonRetryable)
			require.Contains(t, err.Error(), "immutable")
			require.Empty(t, *puts)

			var stored apiv1.Subscription
			require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(cr), &stored))
			require.NotContains(t, stored.Annotations, annSubscriptionLastAppliedImmutableFingerprint)
		})
	}
}

// Gateways that predate externalRefId never report it, so a CR they deployed under a gateway-assigned id
// still matches its spec.externalRefId.
func TestSubscriptionDeploy_AcceptsAGatewayAssignedIdOnGatewaysWithoutExternalRefId(t *testing.T) {
	cr := deployedSubscription(t, false)
	ref := "acme-sub-01-ref"
	cr.Spec.ExternalRefId = &ref
	srv, puts := subscriptionGatewayServer(t, map[string][]map[string]any{
		"orders-api": {{"id": "acme-sub-01"}},
	})

	_, err := (&subscriptionAdapter{}).Deploy(context.Background(), subscriptionTestClient(t, cr), srv.URL, cr, nil)
	require.NoError(t, err)
	require.Len(t, *puts, 1)
}

// A plan that is still being deployed is waited for, as before, and the status is then sent with it.
func TestSubscriptionDeploy_WaitsForAPlanBeingDeployedBeforeSendingTheStatus(t *testing.T) {
	cr := deployedSubscription(t, true)
	plan := &apiv1.SubscriptionPlan{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "gold-plan-uuid"}}
	srv, puts := subscriptionGatewayServer(t, nil)

	_, err := (&subscriptionAdapter{}).Deploy(context.Background(), subscriptionTestClient(t, cr, plan), srv.URL, cr, nil)
	var retryable *gatewayclient.RetryableError
	require.ErrorAs(t, err, &retryable)
	require.Empty(t, *puts)
}

// Once a fingerprint is stored, an immutable-field edit is rejected from it, without asking the gateway.
func TestSubscriptionDeploy_RejectsAnImmutableFieldEditAfterTheBaseline(t *testing.T) {
	cr := deployedSubscription(t, true)
	cr.Spec.ApiId = "payments-api"
	srv, puts := subscriptionGatewayServer(t, nil)

	_, err := (&subscriptionAdapter{}).Deploy(context.Background(), subscriptionTestClient(t, cr), srv.URL, cr, nil)
	var nonRetryable *gatewayclient.NonRetryableError
	require.ErrorAs(t, err, &nonRetryable)
	require.Contains(t, err.Error(), "immutable")
	require.Empty(t, *puts)
}

// A CR with matching billing ids on the gateway gets its baseline as usual.
func TestSubscriptionDeploy_SetsTheBaselineWhenTheBillingIdsMatch(t *testing.T) {
	cr := deployedSubscription(t, false)
	customer, billing := "cus_1", "sub_1"
	cr.Spec.BillingCustomerId = &customer
	cr.Spec.BillingSubscriptionId = &billing
	srv, puts := subscriptionGatewayServer(t, map[string][]map[string]any{
		"orders-api": {{"id": "acme-sub-01", "billingCustomerId": "cus_1", "billingSubscriptionId": "sub_1"}},
	})

	_, err := (&subscriptionAdapter{}).Deploy(context.Background(), subscriptionTestClient(t, cr), srv.URL, cr, nil)
	require.NoError(t, err)
	require.Len(t, *puts, 1)
}

// When the status-only fallback fails transiently, the error is retryable so the status is applied later,
// and it still says why the full update failed.
func TestSubscriptionDeploy_RetriesWhenTheStatusFallbackFailsTransiently(t *testing.T) {
	cr := deployedSubscription(t, true)
	srv, puts := subscriptionGatewayServer(t, nil, http.StatusBadRequest, http.StatusServiceUnavailable)

	_, err := (&subscriptionAdapter{}).Deploy(context.Background(), subscriptionTestClient(t, cr), srv.URL, cr, nil)
	var retryable *gatewayclient.RetryableError
	require.ErrorAs(t, err, &retryable)
	require.Contains(t, err.Error(), "400")
	require.Len(t, *puts, 2)
}

// A subscription adopted after a create conflict also gets its status when its plan or token is rejected.
func TestSubscriptionDeploy_AppliesTheStatusToARecoveredSubscriptionWhenThePlanOrTokenIsRejected(t *testing.T) {
	cr := deployedSubscription(t, false)
	cr.Status.Id = ""
	ref := "acme-sub-01"
	cr.Spec.ExternalRefId = &ref
	srv, puts := subscriptionGatewayServer(t, map[string][]map[string]any{
		"orders-api": {{"id": "acme-sub-01", "externalRefId": "acme-sub-01"}},
	}, http.StatusConflict)

	_, err := (&subscriptionAdapter{}).Deploy(context.Background(), subscriptionTestClient(t, cr), srv.URL, cr, nil)
	var nonRetryable *gatewayclient.NonRetryableError
	require.ErrorAs(t, err, &nonRetryable)
	require.Equal(t, http.StatusConflict, nonRetryable.StatusCode)
	require.Len(t, *puts, 2)
	require.Equal(t, map[string]any{"status": "REVOKED"}, (*puts)[1])
}

// A gateway that accepts the update but keeps the old plan has not applied it, so the update fails instead
// of being reported as programmed.
func TestSubscriptionDeploy_FailsWhenTheGatewayKeepsTheOldPlan(t *testing.T) {
	cr := deployedSubscription(t, true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "acme-sub-01", "subscriptionPlanId": "silver-plan-uuid"})
	}))
	t.Cleanup(srv.Close)

	_, err := (&subscriptionAdapter{}).Deploy(context.Background(), subscriptionTestClient(t, cr), srv.URL, cr, nil)
	var nonRetryable *gatewayclient.NonRetryableError
	require.ErrorAs(t, err, &nonRetryable)
	require.Contains(t, err.Error(), "did not apply spec.subscriptionPlanId")
}

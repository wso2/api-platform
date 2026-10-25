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
	"testing"

	"github.com/stretchr/testify/require"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/gatewayclient"
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
		"orders-api": {{"id": "other-sub"}, {"id": "acme-sub-01", "externalRefId": "acme-sub-01"}},
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
		"orders-api":   {{"id": "other-sub"}},
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

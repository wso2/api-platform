package gatewayclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindSubscriptionIDByAPIAndApplication_SingleResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, ManagementAPIBasePath+"/subscriptions", r.URL.Path)
		require.Equal(t, "hello-sub-api", r.URL.Query().Get("apiId"))
		require.Equal(t, "", r.URL.Query().Get("applicationId"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"subscriptions": []map[string]any{{
				"id":    "sub-1",
				"apiId": "hello-sub-api",
			}},
		})
	}))
	defer srv.Close()

	auth := AuthHeaderFunc(func(_ context.Context, _ *http.Request) error { return nil })
	id, err := FindSubscriptionIDByAPIAndApplication(context.Background(), srv.URL, "hello-sub-api", nil, auth)
	require.NoError(t, err)
	require.Equal(t, "sub-1", id)
}

func TestFindSubscriptionIDByAPIAndApplication_FilterByApplicationID(t *testing.T) {
	app := "app-1"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, ManagementAPIBasePath+"/subscriptions", r.URL.Path)
		require.Equal(t, "hello-sub-api", r.URL.Query().Get("apiId"))
		require.Equal(t, "app-1", r.URL.Query().Get("applicationId"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"subscriptions": []map[string]any{{
				"id":            "sub-1",
				"apiId":         "hello-sub-api",
				"applicationId": "app-1",
			}},
		})
	}))
	defer srv.Close()

	auth := AuthHeaderFunc(func(_ context.Context, _ *http.Request) error { return nil })
	id, err := FindSubscriptionIDByAPIAndApplication(context.Background(), srv.URL, "hello-sub-api", &app, auth)
	require.NoError(t, err)
	require.Equal(t, "sub-1", id)
}

func TestFindSubscriptionIDByAPIAndApplication_AmbiguousResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, ManagementAPIBasePath+"/subscriptions", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"subscriptions": []map[string]any{
				{"id": "sub-1", "apiId": "hello-sub-api"},
				{"id": "sub-2", "apiId": "hello-sub-api"},
			},
		})
	}))
	defer srv.Close()

	auth := AuthHeaderFunc(func(_ context.Context, _ *http.Request) error { return nil })
	id, err := FindSubscriptionIDByAPIAndApplication(context.Background(), srv.URL, "hello-sub-api", nil, auth)
	require.NoError(t, err)
	require.Empty(t, id)
}

func TestFindSubscriptionIDByAPIAndApplication_ConflictStatusClassification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"status":"error","message":"conflict"}`, http.StatusConflict)
	}))
	defer srv.Close()

	auth := AuthHeaderFunc(func(_ context.Context, _ *http.Request) error { return nil })
	_, err := FindSubscriptionIDByAPIAndApplication(context.Background(), srv.URL, "hello-sub-api", nil, auth)
	require.Error(t, err)
	var nr *NonRetryableError
	require.ErrorAs(t, err, &nr)
	require.Equal(t, http.StatusConflict, nr.StatusCode)
}

func TestListSubscriptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, ManagementAPIBasePath+"/subscriptions", r.URL.Path)
		switch r.URL.Query().Get("apiId") {
		case "hello-sub-api":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"subscriptions": []map[string]any{
					{"id": "ext-ref-1", "externalRefId": "ext-ref-1", "apiId": "hello-sub-api"},
					{"id": "sub-2", "apiId": "hello-sub-api"},
				},
				"count": 2,
			})
		case "flaky-api":
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"subscriptions": []map[string]any{}, "count": 0})
		}
	}))
	defer srv.Close()

	auth := AuthHeaderFunc(func(_ context.Context, _ *http.Request) error { return nil })
	got, err := ListSubscriptions(context.Background(), srv.URL, "hello-sub-api", nil, auth)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "ext-ref-1", got[0].Id)
	require.NotNil(t, got[0].ExternalRefId)
	require.Equal(t, "ext-ref-1", *got[0].ExternalRefId)

	empty, err := ListSubscriptions(context.Background(), srv.URL, "other-api", nil, auth)
	require.NoError(t, err)
	require.Empty(t, empty)

	_, err = ListSubscriptions(context.Background(), srv.URL, "flaky-api", nil, auth)
	var retryable *RetryableError
	require.ErrorAs(t, err, &retryable)

	_, err = ListSubscriptions(context.Background(), srv.URL, "", nil, auth)
	require.Error(t, err)
}

func TestUpdateSubscription_PayloadShape(t *testing.T) {
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, ManagementAPIBasePath+"/subscriptions/sub-1", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		bodies = append(bodies, body)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "sub-1"})
	}))
	defer srv.Close()

	auth := AuthHeaderFunc(func(_ context.Context, _ *http.Request) error { return nil })
	plan, token, status := "gold", "new-token", "ACTIVE"
	_, err := UpdateSubscription(context.Background(), srv.URL, "sub-1", SubscriptionUpdatePayload{
		Status: &status, SubscriptionPlanId: &plan, SubscriptionToken: &token,
	}, auth)
	require.NoError(t, err)
	// A nil plan must be sent as an explicit null so the gateway removes it.
	_, err = UpdateSubscription(context.Background(), srv.URL, "sub-1", SubscriptionUpdatePayload{}, auth)
	require.NoError(t, err)

	require.Len(t, bodies, 2)
	require.Equal(t, map[string]any{"status": "ACTIVE", "subscriptionPlanId": "gold", "subscriptionToken": "new-token"}, bodies[0])
	planValue, present := bodies[1]["subscriptionPlanId"]
	require.True(t, present)
	require.Nil(t, planValue)
	require.NotContains(t, bodies[1], "status")
	require.NotContains(t, bodies[1], "subscriptionToken")
}

// A lookup must never return another application's subscription, even when it is the only one on the API.
func TestFindSubscriptionIDByAPIAndApplication_MatchesNormalizedApplicationOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var items []map[string]any
		switch r.URL.Query().Get("apiId") {
		case "single-other-app":
			items = []map[string]any{{"id": "sub-x", "applicationId": "app-x"}}
		case "padded-app":
			items = []map[string]any{{"id": "sub-1", "applicationId": " app-1 "}}
		case "empty-app":
			items = []map[string]any{{"id": "sub-1", "applicationId": ""}}
		case "two-same-app":
			items = []map[string]any{{"id": "sub-1", "applicationId": "app-1"}, {"id": "sub-2", "applicationId": "app-1"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"subscriptions": items})
	}))
	defer srv.Close()
	auth := AuthHeaderFunc(func(_ context.Context, _ *http.Request) error { return nil })
	app1, blank := "app-1", "   "

	id, err := FindSubscriptionIDByAPIAndApplication(context.Background(), srv.URL, "single-other-app", nil, auth)
	require.NoError(t, err)
	require.Empty(t, id, "the only subscription belongs to another application")

	id, err = FindSubscriptionIDByAPIAndApplication(context.Background(), srv.URL, "single-other-app", &app1, auth)
	require.NoError(t, err)
	require.Empty(t, id, "an application filter the gateway did not apply must not match")

	id, err = FindSubscriptionIDByAPIAndApplication(context.Background(), srv.URL, "padded-app", &app1, auth)
	require.NoError(t, err)
	require.Equal(t, "sub-1", id)

	id, err = FindSubscriptionIDByAPIAndApplication(context.Background(), srv.URL, "empty-app", &blank, auth)
	require.NoError(t, err)
	require.Equal(t, "sub-1", id, "an empty and a blank application both mean no application")

	id, err = FindSubscriptionIDByAPIAndApplication(context.Background(), srv.URL, "two-same-app", &app1, auth)
	require.NoError(t, err)
	require.Empty(t, id, "two matches are ambiguous")
}

func TestSameApplication(t *testing.T) {
	s := func(v string) *string { return &v }
	require.True(t, SameApplication(nil, nil))
	require.True(t, SameApplication(nil, s("")))
	require.True(t, SameApplication(s("  "), nil))
	require.True(t, SameApplication(s(" app-1"), s("app-1 ")))
	require.False(t, SameApplication(s("app-1"), nil))
	require.False(t, SameApplication(s("app-1"), s("app-2")))
}

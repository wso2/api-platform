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

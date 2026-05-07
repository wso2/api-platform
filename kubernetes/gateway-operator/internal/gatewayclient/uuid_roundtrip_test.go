package gatewayclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSubscriptionPlan_UUIDRoundTrip exercises the gateway-issued UUID flow:
// the first POST returns a fresh id; subsequent updates and deletes are
// addressed via /subscription-plans/{id}.
func TestSubscriptionPlan_UUIDRoundTrip(t *testing.T) {
	const issuedID = "550e8400-e29b-41d4-a716-446655440000"

	gotPaths := make([]string, 0, 3)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.Method+" "+r.URL.Path)

		switch {
		case r.Method == http.MethodPost && r.URL.Path == ManagementAPIBasePath+"/subscription-plans":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(SubscriptionPlanResponse{Id: issuedID, PlanName: "Bronze"})
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, ManagementAPIBasePath+"/subscription-plans/"):
			require.Equal(t, ManagementAPIBasePath+"/subscription-plans/"+issuedID, r.URL.Path)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(SubscriptionPlanResponse{Id: issuedID, PlanName: "Bronze"})
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, ManagementAPIBasePath+"/subscription-plans/"):
			require.Equal(t, ManagementAPIBasePath+"/subscription-plans/"+issuedID, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	auth := AuthHeaderFunc(func(_ context.Context, _ *http.Request) error { return nil })

	createResp, err := CreateSubscriptionPlan(context.Background(), srv.URL, SubscriptionPlanCreatePayload{PlanName: "Bronze"}, auth)
	require.NoError(t, err)
	require.Equal(t, issuedID, createResp.Id)

	_, err = UpdateSubscriptionPlan(context.Background(), srv.URL, createResp.Id, SubscriptionPlanUpdatePayload{}, auth)
	require.NoError(t, err)

	require.NoError(t, DeleteSubscriptionPlan(context.Background(), srv.URL, createResp.Id, auth))

	require.Equal(t, []string{
		"POST " + ManagementAPIBasePath + "/subscription-plans",
		"PUT " + ManagementAPIBasePath + "/subscription-plans/" + issuedID,
		"DELETE " + ManagementAPIBasePath + "/subscription-plans/" + issuedID,
	}, gotPaths)
}

func TestFindSubscriptionPlanIDByPlanName(t *testing.T) {
	const wantID = "11111111-1111-1111-1111-111111111111"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, ManagementAPIBasePath+"/subscription-plans", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"subscriptionPlans": []map[string]string{
				{"id": "other", "planName": "other-plan"},
				{"planId": wantID, "planName": "demo-plan"},
			},
			"count": 2,
		})
	}))
	defer srv.Close()
	auth := AuthHeaderFunc(func(_ context.Context, _ *http.Request) error { return nil })
	id, err := FindSubscriptionPlanIDByPlanName(context.Background(), srv.URL, "demo-plan", auth)
	require.NoError(t, err)
	require.Equal(t, wantID, id)
}

// TestCertificate_UUIDRoundTrip mirrors the SubscriptionPlan flow for the
// /certificates endpoint (which has no PUT variant).
func TestCertificate_UUIDRoundTrip(t *testing.T) {
	const issuedID = "11111111-2222-3333-4444-555555555555"

	gotPaths := make([]string, 0, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == ManagementAPIBasePath+"/certificates":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(CertificateCreateResponse{Id: issuedID, Name: "ca"})
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, ManagementAPIBasePath+"/certificates/"):
			require.Equal(t, ManagementAPIBasePath+"/certificates/"+issuedID, r.URL.Path)
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	auth := AuthHeaderFunc(func(_ context.Context, _ *http.Request) error { return nil })

	resp, err := UploadCertificate(context.Background(), srv.URL, CertificateUploadPayload{
		Name:        "ca",
		Certificate: "-----BEGIN CERTIFICATE-----\nhello\n-----END CERTIFICATE-----",
	}, auth)
	require.NoError(t, err)
	require.Equal(t, issuedID, resp.Id)

	require.NoError(t, DeleteCertificate(context.Background(), srv.URL, resp.Id, auth))

	require.Equal(t, []string{
		"POST " + ManagementAPIBasePath + "/certificates",
		"DELETE " + ManagementAPIBasePath + "/certificates/" + issuedID,
	}, gotPaths)
}

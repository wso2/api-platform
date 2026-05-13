package gatewayclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyExists_UsesListEndpoint(t *testing.T) {
	const parent = "hello-apikey-api"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != ManagementAPIBasePath+"/rest-apis/"+parent+"/api-keys" {
			http.NotFound(w, r)
			return
		}
		n := "demo-restapi-apikey"
		_ = json.NewEncoder(w).Encode(map[string]any{
			"apiKeys": []map[string]any{{"name": n}},
		})
	}))
	defer srv.Close()

	auth := AuthHeaderFunc(func(_ context.Context, _ *http.Request) error { return nil })
	ok, err := APIKeyExists(context.Background(), srv.URL, ApiKeyParentKindRestApi, parent, "demo-restapi-apikey", auth)
	require.NoError(t, err)
	require.True(t, ok)

	ok2, err := APIKeyExists(context.Background(), srv.URL, ApiKeyParentKindRestApi, parent, "other", auth)
	require.NoError(t, err)
	require.False(t, ok2)
}

func TestDeployAPIKey_FallbackToPutOnCreateConflict(t *testing.T) {
	const (
		parent = "demo-llm-provider-apikey"
		key    = "demo-llmprovider-apikey"
	)

	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == ManagementAPIBasePath+"/llm-providers/"+parent+"/api-keys":
			http.Error(w, `{"message":"configuration already exists: API key value already exists","status":"error"}`, http.StatusConflict)
		case r.Method == http.MethodPut && r.URL.Path == ManagementAPIBasePath+"/llm-providers/"+parent+"/api-keys/"+key:
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	auth := AuthHeaderFunc(func(_ context.Context, _ *http.Request) error { return nil })
	err := DeployAPIKey(context.Background(), srv.URL, ApiKeyParentKindLlmProvider, parent, key, APIKeyCreatePayload{
		Name:   key,
		ApiKey: "demo-llmprovider-apikey-value-1234567890-abcdef",
	}, false, auth)
	require.NoError(t, err)
	require.Equal(t, []string{
		"POST " + ManagementAPIBasePath + "/llm-providers/" + parent + "/api-keys",
		"PUT " + ManagementAPIBasePath + "/llm-providers/" + parent + "/api-keys/" + key,
	}, got)
}

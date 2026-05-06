package subapikey

import (
	"io"
	"net/http"
	"testing"

	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/test/testutil"
	"github.com/wso2/api-platform/cli/utils"
)

func TestRunGenerateCommand_SendsPayload(t *testing.T) {
	testutil.WithTempHome(t)

	var gotBody string
	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/platform-api-keys/generate" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"apiKeyId":"key-1"}`))
	})

	writeSubKeyConfig(t, server.URL)

	generateOrgID = "org-1"
	generateAPIID = "api-1"
	generateKeyName = "mobile-app-key"
	generateExpiresAt = "2026-12-31T23:59:59Z"
	generateName = ""
	generatePlatform = ""
	generateInsecure = false

	if err := runGenerateCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != `{"apiId":"api-1","name":"mobile-app-key","expiresAt":"2026-12-31T23:59:59Z"}` {
		t.Fatalf("unexpected request body %q", gotBody)
	}
}

func TestRunGetCommand_ListsPlatformAPIKeys(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/platform-api-keys" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"apiKeyId":"key-1"}]`))
	})

	writeSubKeyConfig(t, server.URL)

	getOrgID = "org-1"
	getName = ""
	getPlatform = ""
	getInsecure = false

	if err := runGetCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRegenerateCommand_SendsPayload(t *testing.T) {
	testutil.WithTempHome(t)

	var gotBody string
	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/platform-api-keys/key-1/regenerate" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiKeyId":"key-1"}`))
	})

	writeSubKeyConfig(t, server.URL)

	regenerateOrgID = "org-1"
	regenerateAPIKeyID = "key-1"
	regenerateAPIID = "api-1"
	regenerateKeyName = "mobile-app-key"
	regenerateExpiresAt = ""
	regenerateName = ""
	regeneratePlatform = ""
	regenerateInsecure = false

	if err := runRegenerateCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != `{"apiId":"api-1","name":"mobile-app-key"}` {
		t.Fatalf("unexpected request body %q", gotBody)
	}
}

func TestRunRevokeCommand_SendsQueryParameter(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/platform-api-keys/key-1/revoke" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("apiId"); got != "api-1" {
			t.Fatalf("expected apiId query, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"revoked":true}`))
	})

	writeSubKeyConfig(t, server.URL)

	revokeOrgID = "org-1"
	revokeAPIKeyID = "key-1"
	revokeAPIID = "api-1"
	revokeName = ""
	revokePlatform = ""
	revokeInsecure = false

	if err := runRevokeCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildPlatformAPIKeyPayload_MissingAPIID(t *testing.T) {
	_, err := buildPlatformAPIKeyPayload("", "mobile-app-key", "")
	if err == nil || err.Error() != "api ID is required" {
		t.Fatalf("expected api ID validation error, got %v", err)
	}
}

func writeSubKeyConfig(t *testing.T, serverURL string) {
	t.Helper()
	testutil.WriteCLIConfig(t, &config.Config{
		CurrentPlatform: "default",
		Platforms: map[string]*config.Platform{
			"default": {
				DevPortals: map[string]*config.DevPortal{
					"portal": {
						URL: serverURL,
						Auth: config.AuthConfig{
							Type:   utils.AuthTypeAPIKey,
							APIKey: "api-key",
						},
					},
				},
				ActiveDevPortal: "portal",
			},
		},
	})
}

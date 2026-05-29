package subscription

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/test/testutil"
	"github.com/wso2/api-platform/cli/utils"
)

func TestRunCreateCommand_SendsJSONPayload(t *testing.T) {
	testutil.WithTempHome(t)

	var gotBody string
	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/api-platform-subscriptions" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"subscriptionId":"sub-1"}`))
	})

	writeSubscriptionConfig(t, server.URL)

	createOrgID = "org-1"
	createAPIID = "api-1"
	createSubscriptionPlan = "gold"
	createName = ""
	createPlatform = ""
	createInsecure = false

	if err := runCreateCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != `{"apiId":"api-1","subscriptionPlanName":"gold"}` {
		t.Fatalf("unexpected request body %q", gotBody)
	}
}

func TestRunEditCommand_SendsJSONPayload(t *testing.T) {
	testutil.WithTempHome(t)

	var gotBody string
	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut {
			t.Fatalf("expected PUT request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/api-platform-subscriptions/sub-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ACTIVE"}`))
	})

	writeSubscriptionConfig(t, server.URL)

	editOrgID = "org-1"
	editSubscription = "sub-1"
	editStatus = "ACTIVE"
	editName = ""
	editPlatform = ""
	editInsecure = false

	if err := runEditCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != `{"status":"ACTIVE"}` {
		t.Fatalf("unexpected request body %q", gotBody)
	}
}

func TestRunGetCommand_ListAllAndSingle(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/devportal/organizations/org-1/api-platform-subscriptions":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET request, got %s", req.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"subscriptionId":"sub-1"}]`))
		case "/devportal/organizations/org-1/api-platform-subscriptions/sub-1":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET request, got %s", req.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"subscriptionId":"sub-1","status":"ACTIVE"}`))
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
	})

	writeSubscriptionConfig(t, server.URL)

	getOrgID = "org-1"
	getSubscription = ""
	getName = ""
	getPlatform = ""
	getInsecure = false

	out := testutil.CaptureStdout(t, func() {
		if err := runGetCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"subscriptionId": "sub-1"`) {
		t.Fatalf("expected list output, got %q", out)
	}

	getSubscription = "sub-1"
	out = testutil.CaptureStdout(t, func() {
		if err := runGetCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"status": "ACTIVE"`) {
		t.Fatalf("expected single output, got %q", out)
	}
}

func TestRunDeleteCommand_SendsDelete(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/api-platform-subscriptions/sub-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deleted":true}`))
	})

	writeSubscriptionConfig(t, server.URL)

	deleteOrgID = "org-1"
	deleteSubscription = "sub-1"
	deleteName = ""
	deletePlatform = ""
	deleteInsecure = false

	out := testutil.CaptureStdout(t, func() {
		if err := runDeleteCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"deleted": true`) {
		t.Fatalf("expected delete output, got %q", out)
	}
}

func TestRunCreateCommand_OmitsSubscriptionPlanWhenEmpty(t *testing.T) {
	testutil.WithTempHome(t)

	var gotBody string
	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"subscriptionId":"sub-1"}`))
	})

	writeSubscriptionConfig(t, server.URL)

	createOrgID = "org-1"
	createAPIID = "api-1"
	createSubscriptionPlan = ""
	createName = ""
	createPlatform = ""
	createInsecure = false

	if err := runCreateCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != `{"apiId":"api-1"}` {
		t.Fatalf("unexpected request body %q", gotBody)
	}
}

func TestRunCreateCommand_MissingRequiredFlags(t *testing.T) {
	testutil.WithTempHome(t)
	writeSubscriptionConfig(t, "http://example.com")

	createOrgID = "org-1"
	createAPIID = ""
	createSubscriptionPlan = "gold"
	createName = ""
	createPlatform = ""
	createInsecure = false

	err := runCreateCommand()
	if err == nil || !strings.Contains(err.Error(), "api ID is required") {
		t.Fatalf("expected api ID validation error, got %v", err)
	}
}

func TestRunEditCommand_MissingStatus(t *testing.T) {
	testutil.WithTempHome(t)
	writeSubscriptionConfig(t, "http://example.com")

	editOrgID = "org-1"
	editSubscription = "sub-1"
	editStatus = ""
	editName = ""
	editPlatform = ""
	editInsecure = false

	err := runEditCommand()
	if err == nil || !strings.Contains(err.Error(), "status is required") {
		t.Fatalf("expected status validation error, got %v", err)
	}
}

func writeSubscriptionConfig(t *testing.T, serverURL string) {
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

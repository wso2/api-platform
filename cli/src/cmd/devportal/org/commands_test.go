package org

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/test/testutil"
	"github.com/wso2/api-platform/cli/utils"
)

func TestRunListCommand_PrintsOrganizationTable(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"orgId":"org-1","orgName":"Acme","businessOwner":"Jane","organizationIdentifier":"acme"}]`))
	})

	writeOrgConfig(t, server.URL)

	listName = ""
	listPlatform = ""
	listInsecure = false

	out := testutil.CaptureStdout(t, func() {
		if err := runListCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, value := range []string{"ORG_ID", "ORG_NAME", "BUSINESS_OWNER", "ORGANIZATION_IDENTIFIER", "org-1", "Acme", "Jane", "acme"} {
		if !strings.Contains(out, value) {
			t.Fatalf("expected output to contain %q, got %q", value, out)
		}
	}
}

func TestRunGetCommand_PrintsJSON(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"orgId":"org-1","orgName":"Acme"}`))
	})

	writeOrgConfig(t, server.URL)

	getOrgID = "org-1"
	getName = ""
	getPlatform = ""
	getInsecure = false

	out := testutil.CaptureStdout(t, func() {
		if err := runGetCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"orgId": "org-1"`) {
		t.Fatalf("expected json output, got %q", out)
	}
}

func TestRunAddCommand_SendsOrganizationYAMLMultipart(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		assertMultipartOrganization(t, req, "org.yaml", "apiVersion: devportal.api-platform.wso2.com/v1\nkind: Organization\n")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"orgId":"org-1"}`))
	})

	writeOrgConfig(t, server.URL)

	workDir := t.TempDir()
	addFilePath = filepath.Join(workDir, "org.yaml")
	if err := os.WriteFile(addFilePath, []byte("apiVersion: devportal.api-platform.wso2.com/v1\nkind: Organization\n"), 0644); err != nil {
		t.Fatalf("failed to write organization fixture: %v", err)
	}
	addName = ""
	addPlatform = ""
	addInsecure = false

	if err := runAddCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertMultipartOrganization(t *testing.T, req *http.Request, wantFile, wantPayload string) {
	t.Helper()

	if got := req.Header.Get("Content-Type"); !strings.HasPrefix(got, "multipart/form-data") {
		t.Fatalf("expected multipart/form-data content type, got %q", got)
	}
	reader, err := req.MultipartReader()
	if err != nil {
		t.Fatalf("failed to create multipart reader: %v", err)
	}
	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("failed to read multipart part: %v", err)
	}
	defer func() { _ = part.Close() }()

	if part.FormName() != "organization" {
		t.Fatalf("expected multipart field organization, got %q", part.FormName())
	}
	if part.FileName() != wantFile {
		t.Fatalf("expected multipart filename %q, got %q", wantFile, part.FileName())
	}
	body, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("failed to read multipart payload: %v", err)
	}
	if string(body) != wantPayload {
		t.Fatalf("unexpected multipart payload %q", string(body))
	}
	if part, err := reader.NextPart(); err != io.EOF {
		if part != nil {
			_ = part.Close()
		}
		t.Fatalf("expected one multipart part, got next part err=%v", err)
	}
}

func TestRunEditCommand_SendsJSONPayload(t *testing.T) {
	testutil.WithTempHome(t)

	var gotBody string
	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut {
			t.Fatalf("expected PUT request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"updated":true}`))
	})

	writeOrgConfig(t, server.URL)

	workDir := t.TempDir()
	editOrgID = "org-1"
	editFilePath = testutil.WriteJSONFixture(t, workDir, "organization.json", []byte(`{"orgName":"Updated"}`))
	editName = ""
	editPlatform = ""
	editInsecure = false

	if err := runEditCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != `{"orgName":"Updated"}` {
		t.Fatalf("unexpected request body %q", gotBody)
	}
}

func TestRunDeleteCommand_SendsDelete(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deleted":true}`))
	})

	writeOrgConfig(t, server.URL)

	deleteOrgID = "org-1"
	deleteName = ""
	deletePlatform = ""
	deleteInsecure = false

	out := testutil.CaptureStdout(t, func() {
		if err := runDeleteCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"deleted": true`) {
		t.Fatalf("expected delete response output, got %q", out)
	}
}

func TestRunAddCommand_MissingYAMLFile(t *testing.T) {
	testutil.WithTempHome(t)
	writeOrgConfig(t, "http://example.com")

	addFilePath = filepath.Join(t.TempDir(), "missing.yaml")
	addName = ""
	addPlatform = ""
	addInsecure = false

	err := runAddCommand()
	if err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("expected missing file error, got %v", err)
	}
}

func TestRunEditCommand_JSONDirectoryPath(t *testing.T) {
	testutil.WithTempHome(t)
	writeOrgConfig(t, "http://example.com")

	dirPath := filepath.Join(t.TempDir(), "payload")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("failed to create fixture directory: %v", err)
	}

	editOrgID = "org-1"
	editFilePath = dirPath
	editName = ""
	editPlatform = ""
	editInsecure = false

	err := runEditCommand()
	if err == nil || !strings.Contains(err.Error(), "got directory") {
		t.Fatalf("expected directory path error, got %v", err)
	}
}

func writeOrgConfig(t *testing.T, serverURL string) {
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

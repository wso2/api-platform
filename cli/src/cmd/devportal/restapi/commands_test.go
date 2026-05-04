package restapi

import (
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/test/testutil"
	"github.com/wso2/api-platform/cli/utils"
)

func TestRunListCommand_PrintsTable(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/apis" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		if req.URL.RawQuery != "tags=default" {
			t.Fatalf("unexpected query %q", req.URL.RawQuery)
		}
		if got := req.Header.Get(utils.DevPortalAPIHeader); got != "api-key" {
			t.Fatalf("expected api key header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"apiID":"api-1","apiHandle":"foo-1.0.0","apiInfo":{"apiName":"foo","apiVersion":"1.0.0"}}]`))
	})

	writeRestAPIConfig(t, &config.Config{
		CurrentPlatform: "eu",
		Platforms: map[string]*config.Platform{
			"eu": {
				DevPortals: map[string]*config.DevPortal{
					"portal-eu": {
						URL: server.URL,
						Auth: config.AuthConfig{
							Type:   utils.AuthTypeAPIKey,
							APIKey: "api-key",
						},
					},
				},
				ActiveDevPortal: "portal-eu",
			},
		},
	})

	listOrgID = "org-1"
	listName = ""
	listPlatform = ""
	listInsecure = false

	out := testutil.CaptureStdout(t, func() {
		if err := runListCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, value := range []string{"API_ID", "API_HANDLE", "API_NAME", "API_VERSION", "api-1", "foo-1.0.0", "foo", "1.0.0"} {
		if !strings.Contains(out, value) {
			t.Fatalf("expected output to contain %q, got %q", value, out)
		}
	}
}

func TestRunListCommand_UsesNamedPortalInDefaultPlatform(t *testing.T) {
	testutil.WithTempHome(t)

	defaultHit := false
	defaultServer := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		defaultHit = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	euServer := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		t.Fatalf("did not expect eu server to be called")
	})

	writeRestAPIConfig(t, &config.Config{
		CurrentPlatform: "eu",
		Platforms: map[string]*config.Platform{
			"default": {
				DevPortals: map[string]*config.DevPortal{
					"shared": {URL: defaultServer.URL, Auth: config.AuthConfig{Type: utils.AuthTypeAPIKey, APIKey: "key"}},
				},
				ActiveDevPortal: "shared",
			},
			"eu": {
				DevPortals: map[string]*config.DevPortal{
					"shared": {URL: euServer.URL, Auth: config.AuthConfig{Type: utils.AuthTypeAPIKey, APIKey: "key"}},
				},
				ActiveDevPortal: "shared",
			},
		},
	})

	listOrgID = "org-1"
	listName = "shared"
	listPlatform = ""
	listInsecure = false

	if err := runListCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !defaultHit {
		t.Fatalf("expected named portal without platform to resolve in default platform")
	}
}

func TestRunGetCommand_PrintsJSON(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/apis/api-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiID":"api-1","apiInfo":{"apiName":"foo"}}`))
	})

	writeSingleActivePortalConfig(t, server.URL)

	getOrgID = "org-1"
	getAPIID = "api-1"
	getName = ""
	getPlatform = ""
	getInsecure = false

	out := testutil.CaptureStdout(t, func() {
		if err := runGetCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"apiID": "api-1"`) {
		t.Fatalf("expected json output, got %q", out)
	}
}

func TestRunPublishCommand_DefaultArtifactAndMultipartUpload(t *testing.T) {
	testutil.WithTempHome(t)

	workDir := t.TempDir()
	testutil.WithWorkingDir(t, workDir)
	testutil.WriteZipFixture(t, workDir, "devportal.zip")

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/apis" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		assertMultipartArtifact(t, req, "devportal.zip")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiID":"api-1"}`))
	})

	writeSingleActivePortalConfig(t, server.URL)

	publishFilePath = ""
	publishOrgID = "org-1"
	publishName = ""
	publishPlatform = ""
	publishInsecure = false

	out := testutil.CaptureStdout(t, func() {
		if err := runPublishCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "API artifact published") {
		t.Fatalf("expected publish message, got %q", out)
	}
}

func TestRunPublishCommand_MissingDefaultArtifact(t *testing.T) {
	testutil.WithTempHome(t)
	workDir := t.TempDir()
	testutil.WithWorkingDir(t, workDir)
	writeSingleActivePortalConfig(t, "http://example.com")

	publishFilePath = ""
	publishOrgID = "org-1"
	publishName = ""
	publishPlatform = ""
	publishInsecure = false

	err := runPublishCommand()
	if err == nil || !strings.Contains(err.Error(), "Provide --file or place devportal.zip in the current directory") {
		t.Fatalf("expected missing artifact guidance error, got %v", err)
	}
}

func TestRunEditCommand_UploadsArtifact(t *testing.T) {
	testutil.WithTempHome(t)

	workDir := t.TempDir()
	testutil.WriteZipFixture(t, workDir, "artifact.zip")
	writeSingleActivePortalConfig(t, testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut {
			t.Fatalf("expected PUT request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/apis/api-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		assertMultipartArtifact(t, req, "artifact.zip")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"updated":true}`))
	}).URL)

	editFilePath = filepath.Join(workDir, "artifact.zip")
	editOrgID = "org-1"
	editAPIID = "api-1"
	editName = ""
	editPlatform = ""
	editInsecure = false

	if err := runEditCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDeleteCommand_SendsDelete(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/apis/api-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deleted":true}`))
	})

	writeSingleActivePortalConfig(t, server.URL)

	deleteOrgID = "org-1"
	deleteAPIID = "api-1"
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

func writeSingleActivePortalConfig(t *testing.T, serverURL string) {
	t.Helper()

	writeRestAPIConfig(t, &config.Config{
		CurrentPlatform: "eu",
		Platforms: map[string]*config.Platform{
			"eu": {
				DevPortals: map[string]*config.DevPortal{
					"portal-eu": {
						URL: serverURL,
						Auth: config.AuthConfig{
							Type:   utils.AuthTypeAPIKey,
							APIKey: "api-key",
						},
					},
				},
				ActiveDevPortal: "portal-eu",
			},
		},
	})
}

func writeRestAPIConfig(t *testing.T, cfg *config.Config) {
	t.Helper()
	testutil.WriteCLIConfig(t, cfg)
}

func assertMultipartArtifact(t *testing.T, req *http.Request, wantFile string) {
	t.Helper()

	reader, err := req.MultipartReader()
	if err != nil {
		t.Fatalf("failed to create multipart reader: %v", err)
	}
	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("failed to read multipart part: %v", err)
	}
	defer func() { _ = part.Close() }()

	if part.FormName() != "artifact" {
		t.Fatalf("expected multipart field artifact, got %q", part.FormName())
	}
	if part.FileName() != wantFile {
		t.Fatalf("expected multipart filename %q, got %q", wantFile, part.FileName())
	}
	body, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("failed to read multipart payload: %v", err)
	}
	if string(body) != "zip-fixture" {
		t.Fatalf("unexpected multipart payload %q", string(body))
	}
}

func TestRestAPIImports(t *testing.T) {
	_ = multipart.ErrMessageTooLarge
	_ = os.ErrNotExist
}


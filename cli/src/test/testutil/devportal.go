package testutil

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wso2/api-platform/cli/internal/config"
)

func WithTempHome(t *testing.T) string {
	t.Helper()

	homeDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", homeDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Setenv("HOME", originalHome); err != nil {
			t.Fatalf("failed to restore HOME: %v", err)
		}
	})

	return homeDir
}

func WithWorkingDir(t *testing.T, dir string) {
	t.Helper()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	})
}

func WriteCLIConfig(t *testing.T, cfg *config.Config) string {
	t.Helper()

	if err := config.SaveConfig(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	configPath, err := config.GetConfigPath()
	if err != nil {
		t.Fatalf("failed to resolve config path: %v", err)
	}
	return configPath
}

func CaptureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = w

	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = r.Close()
	})

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close stdout writer: %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to capture stdout: %v", err)
	}
	return buf.String()
}

func NewDevPortalServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func WriteZipFixture(t *testing.T, dir, name string) string {
	t.Helper()

	zipPath := filepath.Join(dir, name)
	if err := os.WriteFile(zipPath, []byte("zip-fixture"), 0644); err != nil {
		t.Fatalf("failed to write zip fixture: %v", err)
	}
	return zipPath
}

func WriteJSONFixture(t *testing.T, dir, name string, body []byte) string {
	t.Helper()

	jsonPath := filepath.Join(dir, name)
	if err := os.WriteFile(jsonPath, body, 0644); err != nil {
		t.Fatalf("failed to write json fixture: %v", err)
	}
	return jsonPath
}


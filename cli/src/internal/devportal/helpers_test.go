/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
package devportal

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/test/testutil"
)

func TestResolveDevPortal(t *testing.T) {
	cfg := &config.Config{
		CurrentPlatform: "eu",
		Platforms: map[string]*config.Platform{
			"default": {
				DevPortals: map[string]*config.DevPortal{
					"default-portal": {URL: "http://default.example.com"},
				},
				ActiveDevPortal: "default-portal",
			},
			"eu": {
				DevPortals: map[string]*config.DevPortal{
					"portal-eu": {URL: "http://eu.example.com"},
				},
				ActiveDevPortal: "portal-eu",
			},
		},
	}

	tests := []struct {
		name             string
		selectedName     string
		selectedPlatform string
		wantPortal       string
		wantPlatform     string
		wantErr          string
	}{
		{
			name:         "named portal defaults to default platform",
			selectedName: "default-portal",
			wantPortal:   "default-portal",
			wantPlatform: "default",
		},
		{
			name:             "named portal resolves in explicit platform",
			selectedName:     "portal-eu",
			selectedPlatform: "eu",
			wantPortal:       "portal-eu",
			wantPlatform:     "eu",
		},
		{
			name:         "active portal resolves from current platform",
			wantPortal:   "portal-eu",
			wantPlatform: "eu",
		},
		{
			name:             "missing active portal on explicit platform",
			selectedPlatform: "us",
			wantErr:          "no active devportal set for platform 'us'",
		},
		{
			name:         "missing named portal returns platform specific error",
			selectedName: "missing",
			wantErr:      "devportal 'missing' not found in platform 'default'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portal, platform, err := ResolveDevPortal(cfg, tt.selectedName, tt.selectedPlatform)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if platform != tt.wantPlatform {
				t.Fatalf("expected platform %q, got %q", tt.wantPlatform, platform)
			}
			if portal == nil || portal.Name != tt.wantPortal {
				t.Fatalf("expected portal %q, got %+v", tt.wantPortal, portal)
			}
		})
	}
}

func TestResolveArtifactPath(t *testing.T) {
	workDir := t.TempDir()
	testutil.WithWorkingDir(t, workDir)

	zipPath := testutil.WriteZipFixture(t, workDir, "artifact.zip")
	dirPath := filepath.Join(workDir, "directory")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("failed to create directory fixture: %v", err)
	}
	txtPath := filepath.Join(workDir, "artifact.txt")
	if err := os.WriteFile(txtPath, []byte("text"), 0644); err != nil {
		t.Fatalf("failed to write text fixture: %v", err)
	}

	t.Run("explicit valid zip path", func(t *testing.T) {
		got, err := ResolveArtifactPath(zipPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != zipPath {
			t.Fatalf("expected absolute path %q, got %q", zipPath, got)
		}
	})

	t.Run("falls back to local devportal zip", func(t *testing.T) {
		defaultZip := testutil.WriteZipFixture(t, workDir, "devportal.zip")
		expectedPath, err := filepath.EvalSymlinks(defaultZip)
		if err != nil {
			expectedPath, err = filepath.Abs(defaultZip)
			if err != nil {
				t.Fatalf("failed to resolve expected path: %v", err)
			}
		}
		got, err := ResolveArtifactPath("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resolvedGot, err := filepath.EvalSymlinks(got)
		if err != nil {
			resolvedGot = got
		}
		if resolvedGot != expectedPath {
			t.Fatalf("expected fallback path %q, got %q", expectedPath, resolvedGot)
		}
	})

	t.Run("missing default zip returns guidance", func(t *testing.T) {
		defaultZip := filepath.Join(workDir, "devportal.zip")
		_ = os.Remove(defaultZip)
		_, err := ResolveArtifactPath("")
		if err == nil || !strings.Contains(err.Error(), "Provide --file or place devportal.zip in the current directory") {
			t.Fatalf("expected guidance error, got %v", err)
		}
	})

	t.Run("directory path errors", func(t *testing.T) {
		_, err := ResolveArtifactPath(dirPath)
		if err == nil || !strings.Contains(err.Error(), "got directory") {
			t.Fatalf("expected directory error, got %v", err)
		}
	})

	t.Run("non zip extension errors", func(t *testing.T) {
		_, err := ResolveArtifactPath(txtPath)
		if err == nil || !strings.Contains(err.Error(), "must be a .zip file") {
			t.Fatalf("expected extension error, got %v", err)
		}
	})
}

func TestReadJSONFile(t *testing.T) {
	workDir := t.TempDir()
	jsonPath := testutil.WriteJSONFixture(t, workDir, "payload.json", []byte(`{"name":"foo"}`))
	dirPath := filepath.Join(workDir, "payload-dir")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("failed to create directory fixture: %v", err)
	}

	content, err := ReadJSONFile(jsonPath)
	if err != nil {
		t.Fatalf("unexpected error reading valid json file: %v", err)
	}
	if string(content) != `{"name":"foo"}` {
		t.Fatalf("unexpected json content %q", string(content))
	}

	_, err = ReadJSONFile(filepath.Join(workDir, "missing.json"))
	if err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("expected missing file error, got %v", err)
	}

	_, err = ReadJSONFile(dirPath)
	if err == nil || !strings.Contains(err.Error(), "got directory") {
		t.Fatalf("expected directory error, got %v", err)
	}
}

func TestWrapRequestErrorSuggestsInsecure(t *testing.T) {
	tlsErr := errors.New(`Post "https://localhost": tls: failed to verify certificate: x509: "localhost" certificate is not standards compliant`)

	if !ShouldSuggestInsecure(tlsErr) {
		t.Fatalf("expected TLS error to suggest --insecure")
	}

	got := WrapRequestError("publish api artifact", tlsErr, false)
	if !strings.Contains(got.Error(), "--insecure") {
		t.Fatalf("expected insecure hint, got %v", got)
	}

	got = WrapRequestError("publish api artifact", tlsErr, true)
	if strings.Contains(got.Error(), "--insecure") {
		t.Fatalf("did not expect insecure hint when already insecure: %v", got)
	}

	plainErr := errors.New("connection refused")
	if ShouldSuggestInsecure(plainErr) {
		t.Fatalf("did not expect insecure hint for non TLS error")
	}
}

func TestPrintJSONResponse(t *testing.T) {
	t.Run("pretty prints json", func(t *testing.T) {
		resp := &http.Response{Body: ioNopCloser(`{"name":"foo","version":"1.0.0"}`)}
		out := testutil.CaptureStdout(t, func() {
			if err := PrintJSONResponse(resp); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if !strings.Contains(out, "\"name\": \"foo\"") {
			t.Fatalf("expected pretty json output, got %q", out)
		}
	})

	t.Run("prints raw text for non json", func(t *testing.T) {
		resp := &http.Response{Body: ioNopCloser("plain text")}
		out := testutil.CaptureStdout(t, func() {
			if err := PrintJSONResponse(resp); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if strings.TrimSpace(out) != "plain text" {
			t.Fatalf("expected raw output, got %q", out)
		}
	})

	t.Run("prints nothing for empty body", func(t *testing.T) {
		resp := &http.Response{Body: ioNopCloser("")}
		out := testutil.CaptureStdout(t, func() {
			if err := PrintJSONResponse(resp); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		if out != "" {
			t.Fatalf("expected empty output, got %q", out)
		}
	})
}

type stringReadCloser struct {
	*strings.Reader
}

func (s stringReadCloser) Close() error {
	return nil
}

func ioNopCloser(value string) stringReadCloser {
	return stringReadCloser{Reader: strings.NewReader(value)}
}

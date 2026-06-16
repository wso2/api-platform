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
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/test/testutil"
	"github.com/wso2/api-platform/cli/utils"
)

func TestClientDoAuthHeaders(t *testing.T) {
	tests := []struct {
		name           string
		auth           config.AuthConfig
		env            map[string]string
		wantHeaderName string
		wantHeaderFunc func(*testing.T, *http.Request)
	}{
		{
			name: "basic auth from config",
			auth: config.AuthConfig{Type: utils.AuthTypeBasic, Username: "admin", Password: "secret"},
			wantHeaderFunc: func(t *testing.T, req *http.Request) {
				t.Helper()
				want := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret"))
				if got := req.Header.Get("Authorization"); got != want {
					t.Fatalf("expected authorization header %q, got %q", want, got)
				}
			},
		},
		{
			name: "oauth auth from config",
			auth: config.AuthConfig{Type: utils.AuthTypeOAuth, Token: "token-123"},
			wantHeaderFunc: func(t *testing.T, req *http.Request) {
				t.Helper()
				if got := req.Header.Get("Authorization"); got != "Bearer token-123" {
					t.Fatalf("expected bearer auth header, got %q", got)
				}
			},
		},
		{
			name: "api key auth from config",
			auth: config.AuthConfig{Type: utils.AuthTypeAPIKey, APIKey: "key-123"},
			wantHeaderFunc: func(t *testing.T, req *http.Request) {
				t.Helper()
				if got := req.Header.Get(utils.DevPortalAPIHeader); got != "key-123" {
					t.Fatalf("expected api key header, got %q", got)
				}
			},
		},
		{
			name: "env overrides config",
			auth: config.AuthConfig{Type: utils.AuthTypeAPIKey, APIKey: "config-key"},
			env: map[string]string{
				utils.EnvDevPortalAPIKey: "env-key",
			},
			wantHeaderFunc: func(t *testing.T, req *http.Request) {
				t.Helper()
				if got := req.Header.Get(utils.DevPortalAPIHeader); got != "env-key" {
					t.Fatalf("expected env api key header, got %q", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.env {
				original := os.Getenv(key)
				if err := os.Setenv(key, value); err != nil {
					t.Fatalf("failed to set env: %v", err)
				}
				t.Cleanup(func() {
					_ = os.Setenv(key, original)
				})
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				tt.wantHeaderFunc(t, req)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()

			client := NewClient(&config.DevPortal{
				Name: "portal",
				URL:  server.URL,
				Auth: tt.auth,
			})

			resp, err := client.Get("/health")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer resp.Body.Close()
		})
	}
}

func TestClientDoUnauthorizedMessages(t *testing.T) {
	tests := []struct {
		name       string
		auth       config.AuthConfig
		env        map[string]string
		wantSubstr string
	}{
		{
			name: "basic auth env credentials",
			auth: config.AuthConfig{Type: utils.AuthTypeBasic, Username: "cfg", Password: "cfg"},
			env: map[string]string{
				utils.EnvDevPortalUsername: "env-user",
				utils.EnvDevPortalPassword: "env-pass",
			},
			wantSubstr: "Credentials were sourced from environment variables.",
		},
		{
			name:       "oauth config credentials",
			auth:       config.AuthConfig{Type: utils.AuthTypeOAuth, Token: "cfg-token"},
			wantSubstr: "Credentials were sourced from the configuration file.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.env {
				original := os.Getenv(key)
				if err := os.Setenv(key, value); err != nil {
					t.Fatalf("failed to set env: %v", err)
				}
				t.Cleanup(func() {
					_ = os.Setenv(key, original)
				})
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			}))
			defer server.Close()

			client := NewClient(&config.DevPortal{Name: "portal", URL: server.URL, Auth: tt.auth})
			_, err := client.Get("/health")
			if err == nil || !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Fatalf("expected unauthorized error containing %q, got %v", tt.wantSubstr, err)
			}
		})
	}
}

func TestClientPostMultipartFile(t *testing.T) {
	workDir := t.TempDir()
	artifactPath := testutil.WriteZipFixture(t, workDir, "artifact.zip")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", req.Method)
		}

		reader, err := req.MultipartReader()
		if err != nil {
			t.Fatalf("failed to create multipart reader: %v", err)
		}

		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("failed to read multipart part: %v", err)
		}
		if part.FormName() != "artifact" {
			t.Fatalf("expected multipart field artifact, got %q", part.FormName())
		}
		if part.FileName() != "artifact.zip" {
			t.Fatalf("expected filename artifact.zip, got %q", part.FileName())
		}
		payload, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("failed to read multipart body: %v", err)
		}
		if string(payload) != "zip-fixture" {
			t.Fatalf("unexpected multipart payload %q", string(payload))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewClient(&config.DevPortal{
		Name: "portal",
		URL:  server.URL,
		Auth: config.AuthConfig{Type: utils.AuthTypeAPIKey, APIKey: "key"},
	})

	resp, err := client.PostMultipartFile("/devportal/organizations/org/apis", "artifact", artifactPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
}

func TestClientPostJSON(t *testing.T) {
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", req.Method)
		}
		if got := req.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
			t.Fatalf("expected application/json content type, got %q", got)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewClient(&config.DevPortal{
		Name: "portal",
		URL:  server.URL,
		Auth: config.AuthConfig{Type: utils.AuthTypeOAuth, Token: "token"},
	})

	resp, err := client.PostJSON("/devportal/organizations", []byte(`{"orgName":"Acme"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if gotBody != `{"orgName":"Acme"}` {
		t.Fatalf("unexpected json body %q", gotBody)
	}
}

func TestMultipartHelperImportGuard(t *testing.T) {
	_ = multipart.ErrMessageTooLarge
}


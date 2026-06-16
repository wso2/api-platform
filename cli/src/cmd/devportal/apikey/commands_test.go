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

package apikey

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
		if req.URL.Path != "/devportal/organizations/org-1/api-keys/generate" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"keyId":"key-1","secret":"plaintext"}`))
	})

	writeAPIKeyConfig(t, server.URL)

	generateOrgID = "org-1"
	generateAPIID = "api-1"
	generateName = "weather_prod_key"
	generateExpiresAt = "2026-12-31T23:59:59Z"
	generateDisplayName = ""
	generatePlatform = ""
	generateInsecure = false
	generateNoInteractive = true

	if err := runGenerateCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != `{"apiId":"api-1","name":"weather_prod_key","expiresAt":"2026-12-31T23:59:59Z"}` {
		t.Fatalf("unexpected request body %q", gotBody)
	}
}

func TestRunGetCommand_ListsAPIKeys(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/api-keys" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("apiId"); got != "api-1" {
			t.Fatalf("expected apiId query, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"keyId":"key-1"}]`))
	})

	writeAPIKeyConfig(t, server.URL)

	getOrgID = "org-1"
	getAPIID = "api-1"
	getDisplayName = ""
	getPlatform = ""
	getInsecure = false

	if err := runGetCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRegenerateCommand_PostsToRegenerate(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/api-keys/key-1/regenerate" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keyId":"key-1","secret":"plaintext"}`))
	})

	writeAPIKeyConfig(t, server.URL)

	regenerateOrgID = "org-1"
	regenerateAPIKeyID = "key-1"
	regenerateDisplayName = ""
	regeneratePlatform = ""
	regenerateInsecure = false

	if err := runRegenerateCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRevokeCommand_PostsToRevoke(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", req.Method)
		}
		if req.URL.Path != "/devportal/organizations/org-1/api-keys/key-1/revoke" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	writeAPIKeyConfig(t, server.URL)

	revokeOrgID = "org-1"
	revokeAPIKeyID = "key-1"
	revokeDisplayName = ""
	revokePlatform = ""
	revokeInsecure = false

	if err := runRevokeCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildAPIKeyPayload_MissingAPIID(t *testing.T) {
	_, err := buildAPIKeyPayload("", "weather_prod_key", "")
	if err == nil || err.Error() != "api ID is required" {
		t.Fatalf("expected api ID validation error, got %v", err)
	}
}

func TestBuildAPIKeyPayload_InvalidName(t *testing.T) {
	_, err := buildAPIKeyPayload("api-1", "Invalid Name", "")
	if err == nil {
		t.Fatalf("expected name validation error, got nil")
	}
}

func TestBuildAPIKeyPayload_OmitsEmptyExpiry(t *testing.T) {
	data, err := buildAPIKeyPayload("api-1", "weather_prod_key", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"apiId":"api-1","name":"weather_prod_key"}` {
		t.Fatalf("unexpected payload %q", string(data))
	}
}

func writeAPIKeyConfig(t *testing.T, serverURL string) {
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

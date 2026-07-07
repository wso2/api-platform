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

package application

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
		if req.URL.Path != "/o/org-1/devportal/v1/applications" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"applicationId":"app-1"}`))
	})

	writeApplicationConfig(t, server.URL)

	createOrgID = "org-1"
	createAppName = "Weather App"
	createType = "WEB"
	createDescription = "Calls the Weather APIs"
	createName = ""
	createPlatform = ""
	createInsecure = false

	if err := runCreateCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != `{"name":"Weather App","type":"WEB","description":"Calls the Weather APIs"}` {
		t.Fatalf("unexpected request body %q", gotBody)
	}
}

func TestRunCreateCommand_OmitsDescriptionWhenEmpty(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"applicationId":"app-1"}`))
	})

	writeApplicationConfig(t, server.URL)

	createOrgID = "org-1"
	createAppName = "Weather App"
	createType = "WEB"
	createDescription = ""
	createName = ""
	createPlatform = ""
	createInsecure = false

	if err := runCreateCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != `{"name":"Weather App","type":"WEB"}` {
		t.Fatalf("unexpected request body %q", gotBody)
	}
}

func TestRunUpdateCommand_SendsJSONPayload(t *testing.T) {
	testutil.WithTempHome(t)

	var gotBody string
	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut {
			t.Fatalf("expected PUT request, got %s", req.Method)
		}
		if req.URL.Path != "/o/org-1/devportal/v1/applications/app-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"applicationId":"app-1"}`))
	})

	writeApplicationConfig(t, server.URL)

	updateOrgID = "org-1"
	updateAppID = "app-1"
	updateAppName = "Weather App"
	updateType = "WEB"
	updateDescription = ""
	updateName = ""
	updatePlatform = ""
	updateInsecure = false

	if err := runUpdateCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != `{"name":"Weather App","type":"WEB"}` {
		t.Fatalf("unexpected request body %q", gotBody)
	}
}

func TestRunGetCommand_ListAllAndSingle(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/o/org-1/devportal/v1/applications":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET request, got %s", req.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"applicationId":"app-1"}]`))
		case "/o/org-1/devportal/v1/applications/app-1":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET request, got %s", req.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"applicationId":"app-1","name":"Weather App"}`))
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
	})

	writeApplicationConfig(t, server.URL)

	getOrgID = "org-1"
	getAppID = ""
	getName = ""
	getPlatform = ""
	getInsecure = false

	out := testutil.CaptureStdout(t, func() {
		if err := runGetCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"applicationId": "app-1"`) {
		t.Fatalf("expected list output, got %q", out)
	}

	getAppID = "app-1"
	out = testutil.CaptureStdout(t, func() {
		if err := runGetCommand(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"name": "Weather App"`) {
		t.Fatalf("expected single output, got %q", out)
	}
}

func TestRunDeleteCommand_SendsDelete(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE request, got %s", req.Method)
		}
		if req.URL.Path != "/o/org-1/devportal/v1/applications/app-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"deleted":true}`))
	})

	writeApplicationConfig(t, server.URL)

	deleteOrgID = "org-1"
	deleteAppID = "app-1"
	deleteName = ""
	deletePlatform = ""
	deleteInsecure = false

	if err := runDeleteCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildApplicationPayload_MissingName(t *testing.T) {
	_, err := buildApplicationPayload("", "WEB", "")
	if err == nil || !strings.Contains(err.Error(), "application name is required") {
		t.Fatalf("expected name validation error, got %v", err)
	}
}

func TestBuildApplicationPayload_MissingType(t *testing.T) {
	_, err := buildApplicationPayload("Weather App", "", "")
	if err == nil || !strings.Contains(err.Error(), "application type is required") {
		t.Fatalf("expected type validation error, got %v", err)
	}
}

func writeApplicationConfig(t *testing.T, serverURL string) {
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

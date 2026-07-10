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

package subplan

import (
	"net/http"
	"strings"
	"testing"

	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/test/testutil"
	"github.com/wso2/api-platform/cli/utils"
)

// Subscription plan create/publish moved to the unified `ap devportal apply`
// command; its kind detection and CR validation are covered in the devportal
// package's apply tests. Get/list/delete remain under this group.

func TestRunGetCommand_GetsPlanByPolicyID(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", req.Method)
		}
		if req.URL.Path != "/api/v0.9/subscription-plans/plan-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"policyId":"plan-1","policyName":"Gold"}`))
	})

	writeSubPlanConfig(t, server.URL)

	getPolicyID = "plan-1"
	getName = ""
	getPlatform = ""
	getInsecure = false

	if err := runGetCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGetCommand_MissingPolicyID(t *testing.T) {
	testutil.WithTempHome(t)

	getPolicyID = ""

	if err := runGetCommand(); err == nil || !strings.Contains(err.Error(), "policy ID is required") {
		t.Fatalf("expected policy ID validation error, got %v", err)
	}
}

func TestRunListCommand_ListsPlans(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", req.Method)
		}
		if req.URL.Path != "/api/v0.9/subscription-plans" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"policyId":"plan-1","policyName":"Gold"}]`))
	})

	writeSubPlanConfig(t, server.URL)

	listName = ""
	listPlatform = ""
	listInsecure = false

	if err := runListCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDeleteCommand_DeletesPlanByPolicyID(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE request, got %s", req.Method)
		}
		if req.URL.Path != "/api/v0.9/subscription-plans/plan-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	writeSubPlanConfig(t, server.URL)

	deletePolicyID = "plan-1"
	deleteName = ""
	deletePlatform = ""
	deleteInsecure = false

	if err := runDeleteCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDeleteCommand_MissingPolicyID(t *testing.T) {
	testutil.WithTempHome(t)

	deletePolicyID = ""

	if err := runDeleteCommand(); err == nil || !strings.Contains(err.Error(), "policy ID is required") {
		t.Fatalf("expected policy ID validation error, got %v", err)
	}
}

func writeSubPlanConfig(t *testing.T, serverURL string) {
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

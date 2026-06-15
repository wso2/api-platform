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

const singlePlanYAML = `apiVersion: devportal.api-platform.wso2.com/v1
kind: SubscriptionPolicy
metadata:
  name: Gold
spec:
  displayName: Gold Plan
  billingPlan: FREE
  type: requestcount
  requestCount: 5000
  description: Allows 5000 requests per minute
  refId: cp-plan-gold
`

const planListYAML = `apiVersion: devportal.api-platform.wso2.com/v1
kind: SubscriptionPolicyList
items:
  - metadata:
      name: Gold
    spec:
      displayName: Gold Plan
      billingPlan: FREE
      type: requestcount
      requestCount: 5000
  - metadata:
      name: Unlimited
    spec:
      displayName: Unlimited
      billingPlan: FREE
      type: requestcount
      requestCount: -1
`

func TestRunPublishCommand_UploadsSinglePlanMultipart(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", req.Method)
		}
		if req.URL.Path != "/o/org-1/devportal/v1/subscription-policies" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		assertMultipartPlan(t, req, "plan.yaml", singlePlanYAML)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"policyName":"Gold"}`))
	})

	writeSubPlanConfig(t, server.URL)

	workDir := t.TempDir()
	publishFilePath = filepath.Join(workDir, "plan.yaml")
	if err := os.WriteFile(publishFilePath, []byte(singlePlanYAML), 0644); err != nil {
		t.Fatalf("failed to write plan fixture: %v", err)
	}
	publishOrgID = "org-1"
	publishName = ""
	publishPlatform = ""
	publishInsecure = false

	if err := runPublishCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPublishCommand_UploadsPlanListMultipart(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/o/org-1/devportal/v1/subscription-policies" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		assertMultipartPlan(t, req, "plans.yaml", planListYAML)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`[{"policyName":"Gold"},{"policyName":"Unlimited"}]`))
	})

	writeSubPlanConfig(t, server.URL)

	workDir := t.TempDir()
	publishFilePath = filepath.Join(workDir, "plans.yaml")
	if err := os.WriteFile(publishFilePath, []byte(planListYAML), 0644); err != nil {
		t.Fatalf("failed to write plan list fixture: %v", err)
	}
	publishOrgID = "org-1"
	publishName = ""
	publishPlatform = ""
	publishInsecure = false

	if err := runPublishCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPublishCommand_MissingFile(t *testing.T) {
	testutil.WithTempHome(t)

	publishFilePath = filepath.Join(t.TempDir(), "missing.yaml")
	publishOrgID = "org-1"

	if err := runPublishCommand(); err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
}

func TestValidateSubscriptionPlanCR_AcceptsSinglePlan(t *testing.T) {
	if err := validateSubscriptionPlanCR([]byte(singlePlanYAML)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSubscriptionPlanCR_AcceptsPlanList(t *testing.T) {
	if err := validateSubscriptionPlanCR([]byte(planListYAML)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSubscriptionPlanCR_RejectsUnknownKind(t *testing.T) {
	err := validateSubscriptionPlanCR([]byte("kind: Banana\nmetadata:\n  name: x\n"))
	if err == nil || !strings.Contains(err.Error(), "unsupported kind") {
		t.Fatalf("expected unsupported kind error, got %v", err)
	}
}

func TestValidateSubscriptionPlanCR_RejectsEmptyList(t *testing.T) {
	err := validateSubscriptionPlanCR([]byte("kind: SubscriptionPolicyList\nitems: []\n"))
	if err == nil || !strings.Contains(err.Error(), "at least one plan") {
		t.Fatalf("expected empty list error, got %v", err)
	}
}

func TestValidateSubscriptionPlanCR_RejectsMissingName(t *testing.T) {
	err := validateSubscriptionPlanCR([]byte("kind: SubscriptionPolicy\nspec:\n  displayName: Gold Plan\n"))
	if err == nil || !strings.Contains(err.Error(), "metadata.name is required") {
		t.Fatalf("expected missing name error, got %v", err)
	}
}

func TestRunGetCommand_GetsPlanByPolicyID(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", req.Method)
		}
		if req.URL.Path != "/o/org-1/devportal/v1/subscription-policies/plan-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"policyId":"plan-1","policyName":"Gold"}`))
	})

	writeSubPlanConfig(t, server.URL)

	getPolicyID = "plan-1"
	getOrgID = "org-1"
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
	getOrgID = "org-1"

	if err := runGetCommand(); err == nil || !strings.Contains(err.Error(), "policy ID is required") {
		t.Fatalf("expected policy ID validation error, got %v", err)
	}
}

func TestRunDeleteCommand_DeletesPlanByPolicyID(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewDevPortalServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE request, got %s", req.Method)
		}
		if req.URL.Path != "/o/org-1/devportal/v1/subscription-policies/plan-1" {
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	writeSubPlanConfig(t, server.URL)

	deletePolicyID = "plan-1"
	deleteOrgID = "org-1"
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
	deleteOrgID = "org-1"

	if err := runDeleteCommand(); err == nil || !strings.Contains(err.Error(), "policy ID is required") {
		t.Fatalf("expected policy ID validation error, got %v", err)
	}
}

func assertMultipartPlan(t *testing.T, req *http.Request, wantFile, wantPayload string) {
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

	if part.FormName() != "subscriptionPolicy" {
		t.Fatalf("expected multipart field subscriptionPolicy, got %q", part.FormName())
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

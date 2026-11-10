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

package subscription

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/test/testutil"
	"github.com/wso2/api-platform/cli/utils"
)

func TestBuildUpdatePayload(t *testing.T) {
	tests := []struct {
		name    string
		opts    subscriptionUpdateOptions
		want    map[string]interface{}
		wantErr string
	}{
		{
			name: "status only",
			opts: subscriptionUpdateOptions{status: " REVOKED ", statusSet: true},
			want: map[string]interface{}{"status": "REVOKED"},
		},
		{
			name: "move to another plan",
			opts: subscriptionUpdateOptions{planID: "gold-plan-id", planSet: true},
			want: map[string]interface{}{"subscriptionPlanId": "gold-plan-id"},
		},
		{
			name: "clear plan sends explicit null",
			opts: subscriptionUpdateOptions{clearPlan: true},
			want: map[string]interface{}{"subscriptionPlanId": nil},
		},
		{
			name: "rotate token with status",
			opts: subscriptionUpdateOptions{status: "ACTIVE", statusSet: true, token: "new-token"},
			want: map[string]interface{}{"status": "ACTIVE", "subscriptionToken": "new-token"},
		},
		{
			name:    "plan and clear plan together",
			opts:    subscriptionUpdateOptions{planID: "gold-plan-id", planSet: true, clearPlan: true},
			wantErr: "cannot be used together",
		},
		{
			name:    "blank plan and clear plan together",
			opts:    subscriptionUpdateOptions{planID: "  ", planSet: true, clearPlan: true},
			wantErr: "cannot be used together",
		},
		{
			name:    "blank status is rejected, not dropped",
			opts:    subscriptionUpdateOptions{status: "  ", statusSet: true, planID: "gold-plan-id", planSet: true},
			wantErr: "--status must not be empty",
		},
		{
			name:    "blank plan is rejected, not dropped",
			opts:    subscriptionUpdateOptions{status: "ACTIVE", statusSet: true, planID: " ", planSet: true},
			wantErr: "--subscription-plan-id must not be empty",
		},
		{
			name:    "blank status alone is rejected, not reported as nothing to update",
			opts:    subscriptionUpdateOptions{status: "", statusSet: true},
			wantErr: "--status must not be empty",
		},
		{
			name:    "blank plan alone is rejected, not reported as nothing to update",
			opts:    subscriptionUpdateOptions{planID: "\t", planSet: true},
			wantErr: "--subscription-plan-id must not be empty",
		},
		{
			name:    "blank status beside a token change is rejected",
			opts:    subscriptionUpdateOptions{status: " ", statusSet: true, token: "new-token"},
			wantErr: "--status must not be empty",
		},
		{
			name:    "nothing to update",
			opts:    subscriptionUpdateOptions{},
			wantErr: "nothing to update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildUpdatePayload(tt.opts)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected an error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected payload: got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// newUpdateTestCommand registers the selection flags plus the flags whose presence
// runUpdateCommand inspects via cmd.Flags().Changed.
func newUpdateTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	gateway.AddSelectionFlags(cmd)
	var token, status, planID string
	utils.AddStringFlag(cmd, utils.FlagSubscriptionToken, &token, "", "")
	utils.AddStringFlag(cmd, utils.FlagStatus, &status, "", "")
	utils.AddStringFlag(cmd, utils.FlagSubscriptionPlanID, &planID, "", "")
	return cmd
}

func writeGatewayConfig(t *testing.T, serverURL string) {
	t.Helper()
	testutil.WriteCLIConfig(t, &config.Config{
		CurrentPlatform: "default",
		Platforms: map[string]*config.Platform{
			"default": {
				Gateways: map[string]*config.Gateway{
					"test-gateway": {
						Server: serverURL,
						Auth:   config.AuthConfig{Type: "none"},
					},
				},
				ActiveGateway: "test-gateway",
			},
		},
	})
}

// setUpdateFlags sets the package-level flag variables for one test and
// restores them afterwards.
func setUpdateFlags(t *testing.T, id, status, planID string, clearPlan bool, token string, rotate bool) {
	t.Helper()
	updateSubscriptionID, updateStatus, updatePlanID = id, status, planID
	updateClearPlan, updateSubscriptionToken, updateRotateToken = clearPlan, token, rotate
	t.Cleanup(func() {
		updateSubscriptionID, updateStatus, updatePlanID = "", "", ""
		updateClearPlan, updateSubscriptionToken, updateRotateToken = false, "", false
	})
}

func TestRunUpdate_ClearPlanSendsNullInPut(t *testing.T) {
	testutil.WithTempHome(t)

	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		raw, _ := io.ReadAll(req.Body)
		if err := json.Unmarshal(raw, &gotBody); err != nil {
			t.Fatalf("request body is not JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sub-1","apiId":"petstore-api-v1.0","status":"ACTIVE"}`))
	})
	writeGatewayConfig(t, server.URL)
	setUpdateFlags(t, "sub-1", "", "", true, "", false)

	if err := runUpdateCommand(newUpdateTestCommand()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("expected PUT request, got %s", gotMethod)
	}
	if gotPath != "/subscriptions/sub-1" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
	want := map[string]interface{}{"subscriptionPlanId": nil}
	if !reflect.DeepEqual(gotBody, want) {
		t.Fatalf("unexpected request body: got %#v, want %#v", gotBody, want)
	}
}

func TestRunUpdate_SurfacesGatewayErrorMessage(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"status":"error","message":"The subscription conflicts with an existing subscription"}`))
	})
	writeGatewayConfig(t, server.URL)
	setUpdateFlags(t, "sub-1", "", "", false, "new-token", false)

	err := runUpdateCommand(newUpdateTestCommand())
	if err == nil || !strings.Contains(err.Error(), "409") || !strings.Contains(err.Error(), "conflicts with an existing subscription") {
		t.Fatalf("expected an error with the 409 status and gateway message, got %v", err)
	}
}

func TestRunUpdate_RejectsRotateWithInlineToken(t *testing.T) {
	testutil.WithTempHome(t)
	setUpdateFlags(t, "sub-1", "", "", false, "new-token", true)

	err := runUpdateCommand(newUpdateTestCommand())
	if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("expected a flag conflict error, got %v", err)
	}
}

func TestRunUpdate_RequiresID(t *testing.T) {
	testutil.WithTempHome(t)
	setUpdateFlags(t, "", "REVOKED", "", false, "", false)

	if err := runUpdateCommand(newUpdateTestCommand()); err == nil {
		t.Fatal("expected an --id validation error, got nil")
	}
}

func TestRunUpdate_RejectsSuppliedButBlankFlags(t *testing.T) {
	testutil.WithTempHome(t)

	setUpdateFlags(t, "sub-1", "  ", "gold-plan-id", false, "", false)
	cmd := newUpdateTestCommand()
	if err := cmd.Flags().Set(utils.FlagStatus, "  "); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set(utils.FlagSubscriptionPlanID, "gold-plan-id"); err != nil {
		t.Fatal(err)
	}
	if err := runUpdateCommand(cmd); err == nil || !strings.Contains(err.Error(), "--status must not be empty") {
		t.Fatalf("expected a blank --status to be rejected, got %v", err)
	}

	setUpdateFlags(t, "sub-1", "", " ", true, "", false)
	cmd = newUpdateTestCommand()
	if err := cmd.Flags().Set(utils.FlagSubscriptionPlanID, " "); err != nil {
		t.Fatal(err)
	}
	if err := runUpdateCommand(cmd); err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("expected a blank --subscription-plan-id with --clear-subscription-plan to conflict, got %v", err)
	}
}

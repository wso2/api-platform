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
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/test/testutil"
)

// newTestCommand mirrors graphqlapi's own helper: a bare *cobra.Command with
// the --platform/--gateway selection flags registered, which
// gateway.NewClientFromCommand reads to resolve the active gateway.
func newTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	gateway.AddSelectionFlags(cmd)
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

func TestRunCreateCommand_PostsToAPIKeysEndpoint(t *testing.T) {
	testutil.WithTempHome(t)

	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		if err := json.NewDecoder(req.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"success","message":"API key generated successfully","apiKey":{"name":"smoke-key-1","apiKey":"apip_abc123"}}`))
	})
	writeGatewayConfig(t, server.URL)

	createAPIID = "countries-graphql-api"
	createName = "smoke-key-1"
	createExpiresInDuration = 0
	createExpiresInUnit = ""

	if err := runCreateCommand(newTestCommand()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST request, got %s", gotMethod)
	}
	if gotPath != "/graphql-apis/countries-graphql-api/api-keys" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
	if gotBody["name"] != "smoke-key-1" {
		t.Fatalf("expected request body name to be the --name flag value, got %v", gotBody["name"])
	}
	if _, present := gotBody["expiresIn"]; present {
		t.Fatalf("expected no expiresIn field when duration/unit are unset, got %v", gotBody)
	}
}

func TestRunCreateCommand_NameOmitted_NotSentInBody(t *testing.T) {
	testutil.WithTempHome(t)

	var gotBody map[string]interface{}
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		_ = json.NewDecoder(req.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"success","apiKey":{"name":"auto-generated-name"}}`))
	})
	writeGatewayConfig(t, server.URL)

	createAPIID = "countries-graphql-api"
	createName = ""
	createExpiresInDuration = 0
	createExpiresInUnit = ""

	if err := runCreateCommand(newTestCommand()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, present := gotBody["name"]; present {
		t.Fatalf("expected no 'name' field in the request body when --name is omitted, letting the server auto-generate one, got %v", gotBody)
	}
}

func TestRunCreateCommand_WithExpiresIn_SendsDurationAndUnit(t *testing.T) {
	testutil.WithTempHome(t)

	var gotBody map[string]interface{}
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		_ = json.NewDecoder(req.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"success","apiKey":{"name":"smoke-key-1"}}`))
	})
	writeGatewayConfig(t, server.URL)

	createAPIID = "countries-graphql-api"
	createName = "smoke-key-1"
	createExpiresInDuration = 30
	createExpiresInUnit = "days"

	if err := runCreateCommand(newTestCommand()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expiresIn, ok := gotBody["expiresIn"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected request body to contain an expiresIn object, got %v", gotBody)
	}
	if expiresIn["duration"] != float64(30) || expiresIn["unit"] != "days" {
		t.Fatalf("expected expiresIn {duration: 30, unit: days}, got %v", expiresIn)
	}
}

func TestRunCreateCommand_RequiresID(t *testing.T) {
	testutil.WithTempHome(t)

	createAPIID = ""
	createName = ""
	createExpiresInDuration = 0
	createExpiresInUnit = ""

	err := runCreateCommand(newTestCommand())
	if err == nil {
		t.Fatal("expected an --id validation error, got nil")
	}
}

func TestRunCreateCommand_ExpiresInDurationWithoutUnit_Errors(t *testing.T) {
	testutil.WithTempHome(t)

	createAPIID = "countries-graphql-api"
	createName = ""
	createExpiresInDuration = 30
	createExpiresInUnit = ""

	err := runCreateCommand(newTestCommand())
	if err == nil || !strings.Contains(err.Error(), "expires-in-unit") {
		t.Fatalf("expected an error about --expires-in-unit being required alongside --expires-in-duration, got %v", err)
	}
}

func TestRunCreateCommand_InvalidExpiresInUnit_Errors(t *testing.T) {
	testutil.WithTempHome(t)

	createAPIID = "countries-graphql-api"
	createName = ""
	createExpiresInDuration = 30
	createExpiresInUnit = "fortnights"

	err := runCreateCommand(newTestCommand())
	if err == nil || !strings.Contains(err.Error(), "fortnights") {
		t.Fatalf("expected an invalid-unit error mentioning the bad value, got %v", err)
	}
}

func TestRunListCommand_CallsAPIKeysEndpoint(t *testing.T) {
	testutil.WithTempHome(t)

	var gotPath string
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", req.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","totalCount":1,"apiKeys":[{"name":"smoke-key-1","apiId":"countries-graphql-api","status":"active"}]}`))
	})
	writeGatewayConfig(t, server.URL)

	listAPIID = "countries-graphql-api"

	if err := runListCommand(newTestCommand()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/graphql-apis/countries-graphql-api/api-keys" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
}

func TestRunListCommand_RequiresID(t *testing.T) {
	testutil.WithTempHome(t)

	listAPIID = ""

	err := runListCommand(newTestCommand())
	if err == nil {
		t.Fatal("expected an --id validation error, got nil")
	}
}

func TestRunListCommand_NotFound(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	writeGatewayConfig(t, server.URL)

	listAPIID = "nonexistent"

	err := runListCommand(newTestCommand())
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected a not-found error, got %v", err)
	}
}

func TestRunRegenerateCommand_PostsToRegenerateEndpoint(t *testing.T) {
	testutil.WithTempHome(t)

	var gotMethod, gotPath string
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","apiKey":{"name":"smoke-key-1","apiKey":"apip_newvalue"}}`))
	})
	writeGatewayConfig(t, server.URL)

	regenerateAPIID = "countries-graphql-api"
	regenerateKeyName = "smoke-key-1"

	if err := runRegenerateCommand(newTestCommand()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST request, got %s", gotMethod)
	}
	if gotPath != "/graphql-apis/countries-graphql-api/api-keys/smoke-key-1/regenerate" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
}

func TestRunRegenerateCommand_RequiresIDAndKeyName(t *testing.T) {
	testutil.WithTempHome(t)

	regenerateAPIID = ""
	regenerateKeyName = ""

	if err := runRegenerateCommand(newTestCommand()); err == nil {
		t.Fatal("expected an --id validation error, got nil")
	}

	regenerateAPIID = "countries-graphql-api"
	regenerateKeyName = ""
	if err := runRegenerateCommand(newTestCommand()); err == nil {
		t.Fatal("expected a --key-name validation error, got nil")
	}
}

func TestRunRegenerateCommand_NotFound(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	writeGatewayConfig(t, server.URL)

	regenerateAPIID = "nonexistent"
	regenerateKeyName = "smoke-key-1"

	err := runRegenerateCommand(newTestCommand())
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected an error mentioning the 404 status, got %v", err)
	}
}

func TestRunUpdateCommand_PutsToAPIKeyEndpoint(t *testing.T) {
	testutil.WithTempHome(t)

	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		_ = json.NewDecoder(req.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","apiKey":{"name":"smoke-key-1"}}`))
	})
	writeGatewayConfig(t, server.URL)

	updateAPIID = "countries-graphql-api"
	updateKeyName = "smoke-key-1"
	updateNewAPIKey = "external-key-value-that-is-at-least-36-characters-long"

	if err := runUpdateCommand(newTestCommand()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("expected PUT request, got %s", gotMethod)
	}
	if gotPath != "/graphql-apis/countries-graphql-api/api-keys/smoke-key-1" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
	// The request body field must be "apiKey" - the server's
	// APIKeyCreationRequest schema has no "name" field for this endpoint;
	// sending "name" here would silently no-op server-side.
	if gotBody["apiKey"] != updateNewAPIKey {
		t.Fatalf(`expected request body {"apiKey": ...}, got %v`, gotBody)
	}
	if _, present := gotBody["name"]; present {
		t.Fatalf("request body must not contain a 'name' field, got %v", gotBody)
	}
}

func TestRunUpdateCommand_RequiresAllFlags(t *testing.T) {
	testutil.WithTempHome(t)

	updateAPIID, updateKeyName, updateNewAPIKey = "", "smoke-key-1", "value-value-value-value-value-value"
	if err := runUpdateCommand(newTestCommand()); err == nil {
		t.Fatal("expected an --id validation error, got nil")
	}

	updateAPIID, updateKeyName, updateNewAPIKey = "countries-graphql-api", "", "value-value-value-value-value-value"
	if err := runUpdateCommand(newTestCommand()); err == nil {
		t.Fatal("expected a --key-name validation error, got nil")
	}

	updateAPIID, updateKeyName, updateNewAPIKey = "countries-graphql-api", "smoke-key-1", ""
	if err := runUpdateCommand(newTestCommand()); err == nil {
		t.Fatal("expected an --api-key validation error, got nil")
	}
}

// TestRunUpdateCommand_RejectsLocallyGeneratedKey guards the real business rule
// surfaced during manual verification of this feature: the gateway rejects
// updating a locally-generated key (only regenerate is allowed for those) with
// a 400, which the CLI must surface as an error, not silently succeed.
func TestRunUpdateCommand_RejectsLocallyGeneratedKey(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":"error","message":"operation not allowed: updates are only allowed for externally generated API keys"}`))
	})
	writeGatewayConfig(t, server.URL)

	updateAPIID = "countries-graphql-api"
	updateKeyName = "smoke-key-1"
	updateNewAPIKey = "external-key-value-that-is-at-least-36-characters-long"

	err := runUpdateCommand(newTestCommand())
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected an error mentioning the 400 status, got %v", err)
	}
}

func TestRunRevokeCommand_DeletesAPIKeyEndpoint(t *testing.T) {
	testutil.WithTempHome(t)

	var gotMethod, gotPath string
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	})
	writeGatewayConfig(t, server.URL)

	revokeAPIID = "countries-graphql-api"
	revokeKeyName = "smoke-key-1"

	if err := runRevokeCommand(newTestCommand()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("expected DELETE request, got %s", gotMethod)
	}
	if gotPath != "/graphql-apis/countries-graphql-api/api-keys/smoke-key-1" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
}

func TestRunRevokeCommand_RequiresIDAndKeyName(t *testing.T) {
	testutil.WithTempHome(t)

	revokeAPIID, revokeKeyName = "", "smoke-key-1"
	if err := runRevokeCommand(newTestCommand()); err == nil {
		t.Fatal("expected an --id validation error, got nil")
	}

	revokeAPIID, revokeKeyName = "countries-graphql-api", ""
	if err := runRevokeCommand(newTestCommand()); err == nil {
		t.Fatal("expected a --key-name validation error, got nil")
	}
}

func TestRunRevokeCommand_NotFound(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	writeGatewayConfig(t, server.URL)

	revokeAPIID = "countries-graphql-api"
	revokeKeyName = "nonexistent"

	err := runRevokeCommand(newTestCommand())
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected an error mentioning the 404 status, got %v", err)
	}
}

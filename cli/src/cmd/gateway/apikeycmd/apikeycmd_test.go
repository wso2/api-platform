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

package apikeycmd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/test/testutil"
)

// testConfig mirrors the real GraphQL API config (cmd/gateway/graphqlapi/apikey)
// so these tests double as a regression check on that production Config value,
// while exercising the shared RunList/RunRegenerate/RunRevoke logic once for
// every API kind that reuses this package.
var testConfig = Config{
	KindLabel:       "GraphQL API",
	KindPathSegment: "graphql-api",
	ExampleAPIID:    "countries-graphql-api",
	KeysPath:        "/graphql-apis/%s/api-keys",
	KeyByNamePath:   "/graphql-apis/%s/api-keys/%s",
	RegeneratePath:  "/graphql-apis/%s/api-keys/%s/regenerate",
}

// newTestCommand mirrors gateway command packages' own helper: a bare
// *cobra.Command with the --platform/--gateway selection flags registered,
// which gateway.NewClientFromCommand reads to resolve the active gateway.
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

func TestRunList_CallsAPIKeysEndpoint(t *testing.T) {
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

	if err := RunList(newTestCommand(), testConfig, "countries-graphql-api"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/graphql-apis/countries-graphql-api/api-keys" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
}

func TestRunList_RequiresID(t *testing.T) {
	testutil.WithTempHome(t)

	err := RunList(newTestCommand(), testConfig, "")
	if err == nil {
		t.Fatal("expected an --id validation error, got nil")
	}
}

func TestRunList_NotFound(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	writeGatewayConfig(t, server.URL)

	err := RunList(newTestCommand(), testConfig, "nonexistent")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected a not-found error, got %v", err)
	}
}

func TestRunRegenerate_PostsToRegenerateEndpoint(t *testing.T) {
	testutil.WithTempHome(t)

	var gotMethod, gotPath string
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","apiKey":{"name":"smoke-key-1","apiKey":"apip_newvalue"}}`))
	})
	writeGatewayConfig(t, server.URL)

	if err := RunRegenerate(newTestCommand(), testConfig, "countries-graphql-api", "smoke-key-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST request, got %s", gotMethod)
	}
	if gotPath != "/graphql-apis/countries-graphql-api/api-keys/smoke-key-1/regenerate" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
}

func TestRunRegenerate_RequiresIDAndKeyName(t *testing.T) {
	testutil.WithTempHome(t)

	if err := RunRegenerate(newTestCommand(), testConfig, "", ""); err == nil {
		t.Fatal("expected an --id validation error, got nil")
	}
	if err := RunRegenerate(newTestCommand(), testConfig, "countries-graphql-api", ""); err == nil {
		t.Fatal("expected a --key-name validation error, got nil")
	}
}

func TestRunRegenerate_NotFound(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	writeGatewayConfig(t, server.URL)

	err := RunRegenerate(newTestCommand(), testConfig, "nonexistent", "smoke-key-1")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected an error mentioning the 404 status, got %v", err)
	}
}

func TestRunRevoke_DeletesAPIKeyEndpoint(t *testing.T) {
	testutil.WithTempHome(t)

	var gotMethod, gotPath string
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	})
	writeGatewayConfig(t, server.URL)

	if err := RunRevoke(newTestCommand(), testConfig, "countries-graphql-api", "smoke-key-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("expected DELETE request, got %s", gotMethod)
	}
	if gotPath != "/graphql-apis/countries-graphql-api/api-keys/smoke-key-1" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
}

func TestRunRevoke_RequiresIDAndKeyName(t *testing.T) {
	testutil.WithTempHome(t)

	if err := RunRevoke(newTestCommand(), testConfig, "", "smoke-key-1"); err == nil {
		t.Fatal("expected an --id validation error, got nil")
	}
	if err := RunRevoke(newTestCommand(), testConfig, "countries-graphql-api", ""); err == nil {
		t.Fatal("expected a --key-name validation error, got nil")
	}
}

func TestRunRevoke_NotFound(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	writeGatewayConfig(t, server.URL)

	err := RunRevoke(newTestCommand(), testConfig, "countries-graphql-api", "nonexistent")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected an error mentioning the 404 status, got %v", err)
	}
}

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

package graphqlapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/config"
	"github.com/wso2/api-platform/cli/internal/gateway"
	"github.com/wso2/api-platform/cli/test/testutil"
)

// newTestCommand builds a bare *cobra.Command with the --platform/--gateway
// selection flags registered, matching what every real graphql-api subcommand
// gets via gateway.AddSelectionFlags in its own init(). NewClientFromCommand
// reads those flags, so a command missing them would resolve against whatever
// is "active" in config regardless of intent - tests leave them unset to
// exercise the same active-gateway fallback real usage relies on.
func newTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	gateway.AddSelectionFlags(cmd)
	return cmd
}

// writeGatewayConfig points the active gateway (platform "default") at the
// given test server URL with no authentication, and returns the config path.
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

func TestRunListCommand_CallsGraphQLAPIsEndpoint(t *testing.T) {
	testutil.WithTempHome(t)

	var gotPath string
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", req.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","count":0,"graphqlApis":[]}`))
	})
	writeGatewayConfig(t, server.URL)

	if err := runListCommand(newTestCommand()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/graphql-apis" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
}

func TestRunListCommand_NotFoundTreatedAsEmpty(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	writeGatewayConfig(t, server.URL)

	if err := runListCommand(newTestCommand()); err != nil {
		t.Fatalf("expected 404 to be treated as an empty list, got error: %v", err)
	}
}

func TestRunGetCommand_ByID(t *testing.T) {
	testutil.WithTempHome(t)

	var gotPath string
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"gateway.api-platform.wso2.com/v1","kind":"GraphQLApi","metadata":{"name":"countries-graphql-api"},"spec":{"displayName":"Countries","version":"v1","context":"/countries"},"status":{"id":"countries-graphql-api"}}`))
	})
	writeGatewayConfig(t, server.URL)

	getAPIID = "countries-graphql-api"
	getAPIName = ""
	getAPIVersion = ""
	getAPIFormat = "json"

	if err := runGetCommand(newTestCommand()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/graphql-apis/countries-graphql-api" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
}

func TestRunGetCommand_ByDisplayNameAndVersion(t *testing.T) {
	testutil.WithTempHome(t)

	var gotPaths []string
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		gotPaths = append(gotPaths, req.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		if req.URL.Path == "/graphql-apis" {
			// The list-by-filter lookup must query displayName, not "name" -
			// the server only ever supported a displayName filter (confirmed
			// against the generated ListGraphQLAPIsParams struct); a "name"
			// query param would silently return everything unfiltered.
			if got := req.URL.Query().Get("displayName"); got != "Countries GraphQL API" {
				t.Fatalf("expected displayName query param, got query %q", req.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"status":"success","count":1,"graphqlApis":[{"metadata":{"name":"countries-graphql-api"},"spec":{},"status":{"id":"countries-graphql-api"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"apiVersion":"gateway.api-platform.wso2.com/v1","kind":"GraphQLApi","metadata":{"name":"countries-graphql-api"},"spec":{},"status":{"id":"countries-graphql-api"}}`))
	})
	writeGatewayConfig(t, server.URL)

	getAPIID = ""
	getAPIName = "Countries GraphQL API"
	getAPIVersion = "v1"
	getAPIFormat = "json"

	if err := runGetCommand(newTestCommand()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotPaths) != 2 {
		t.Fatalf("expected a list lookup followed by a get-by-id call, got %v", gotPaths)
	}
}

func TestRunGetCommand_RequiresIDOrName(t *testing.T) {
	testutil.WithTempHome(t)

	getAPIID = ""
	getAPIName = ""
	getAPIVersion = ""
	getAPIFormat = "json"

	err := runGetCommand(newTestCommand())
	if err == nil || err.Error() != "either --id or --display-name (with --version) must be specified" {
		t.Fatalf("expected id/name validation error, got %v", err)
	}
}

func TestRunGetCommand_RejectsInvalidFormat(t *testing.T) {
	testutil.WithTempHome(t)

	getAPIID = "countries-graphql-api"
	getAPIName = ""
	getAPIVersion = ""
	getAPIFormat = "xml"

	err := runGetCommand(newTestCommand())
	if err == nil {
		t.Fatal("expected an invalid-format error, got nil")
	}
}

func TestRunDeleteCommand_CallsDeleteByID(t *testing.T) {
	testutil.WithTempHome(t)

	var gotMethod, gotPath string
	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})
	writeGatewayConfig(t, server.URL)

	deleteAPIID = "countries-graphql-api"

	if err := runDeleteCommand(newTestCommand()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("expected DELETE request, got %s", gotMethod)
	}
	if gotPath != "/graphql-apis/countries-graphql-api" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
}

// TestRunDeleteCommand_NotFound guards Client.Delete's actual contract: any
// non-2xx status (including 404) comes back as a non-nil error with resp==nil,
// so the error text is whatever formatHTTPError produces, not a bespoke
// "not found" message built from a status-code check on resp (which would be
// unreachable dead code once err is non-nil).
func TestRunDeleteCommand_NotFound(t *testing.T) {
	testutil.WithTempHome(t)

	server := testutil.NewGatewayServer(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	writeGatewayConfig(t, server.URL)

	deleteAPIID = "nonexistent"

	err := runDeleteCommand(newTestCommand())
	if err == nil {
		t.Fatal("expected an error for a 404 response, got nil")
	}
	if !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "nonexistent") {
		t.Fatalf("expected error to mention the 404 status and the API ID, got %v", err)
	}
}

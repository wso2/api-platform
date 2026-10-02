/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package utils

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"

	"gopkg.in/yaml.v3"
)

func TestBuildMCPDeploymentYAML(t *testing.T) {
	util := &MCPUtils{}

	ctx := "/mcp-test"
	projectID := "proj-456"
	proxy := &model.MCPProxy{
		Handle:      "test-mcp-proxy",
		Name:        "Test MCP Proxy",
		Version:     "v1.0",
		ProjectUUID: &projectID,
		Configuration: model.MCPProxyConfiguration{
			Context:     &ctx,
			SpecVersion: "2025-06-18",
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL: "http://mcp-backend:8080/mcp",
				},
			},
		},
	}

	// Build struct
	deploymentStruct, err := util.BuildMCPDeploymentYAML(proxy)
	if err != nil {
		t.Fatalf("BuildMCPDeploymentYAML() error = %v", err)
	}

	// Marshal the struct
	structBytes, err := yaml.Marshal(deploymentStruct)
	if err != nil {
		t.Fatalf("failed to marshal struct: %v", err)
	}

	// Generate via the wrapper
	yamlString, err := util.GenerateMCPDeploymentYAML(proxy)
	if err != nil {
		t.Fatalf("GenerateMCPDeploymentYAML() error = %v", err)
	}

	// Compare: both should produce identical YAML
	if string(structBytes) != yamlString {
		t.Errorf("BuildMCPDeploymentYAML + Marshal differs from GenerateMCPDeploymentYAML.\nBuild:\n%s\nGenerate:\n%s", string(structBytes), yamlString)
	}

	// Verify key struct fields
	if deploymentStruct.ApiVersion != constants.GatewayApiVersion {
		t.Errorf("ApiVersion = %q", deploymentStruct.ApiVersion)
	}
	if deploymentStruct.Kind != constants.MCPProxy {
		t.Errorf("Kind = %q", deploymentStruct.Kind)
	}
	if deploymentStruct.Metadata.Name != "test-mcp-proxy" {
		t.Errorf("Metadata.Name = %q", deploymentStruct.Metadata.Name)
	}
	if deploymentStruct.Metadata.Labels["projectId"] != "proj-456" {
		t.Errorf("Metadata.Labels[projectId] = %q", deploymentStruct.Metadata.Labels["projectId"])
	}
	if deploymentStruct.Spec.Upstream.URL != "http://mcp-backend:8080/mcp" {
		t.Errorf("Upstream.URL = %q", deploymentStruct.Spec.Upstream.URL)
	}
	if deploymentStruct.Spec.Context != "/mcp-test" {
		t.Errorf("Context = %q", deploymentStruct.Spec.Context)
	}
	if got := deploymentStruct.Spec.SpecVersions; len(got) != 1 || got[0] != "2025-06-18" {
		t.Errorf("SpecVersions = %v", got)
	}
	if deploymentStruct.Spec.SpecVersion != "" {
		t.Errorf("SpecVersion = %q, want empty: the deprecated field is never emitted", deploymentStruct.Spec.SpecVersion)
	}
	// The struct assertions above cannot catch a key rename, so pin the emitted text.
	if !strings.Contains(yamlString, "specVersions:") {
		t.Errorf("emitted YAML has no specVersions key:\n%s", yamlString)
	}
	if strings.Contains(yamlString, "specVersion:") {
		t.Errorf("emitted YAML still carries a specVersion key, which the gateway reads as present:\n%s", yamlString)
	}
}

// TestFetchMCPServerInfoUpstream401IsNotOurUnauthorized pins the status-code
// separation between "the remote MCP server rejected our credentials" and "the
// caller's own session is invalid". Clients (the AI Workspace) react to a 401
// from this API by tearing down the session and redirecting to /login, so
// relaying an upstream's 401 verbatim let any auth-requiring MCP server sign
// the user out. The condition must surface as MCP_PROXY_UPSTREAM_UNAUTHORIZED
// with a non-401 status instead.
func TestFetchMCPServerInfoUpstream401IsNotOurUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := FetchMCPServerInfo(srv.URL, "", "")
	if err == nil {
		t.Fatal("expected an error when the MCP server rejects the initialize request")
	}

	// errors.As, not a type assertion: FetchMCPServerInfo wraps the failure.
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected a catalog error, got %T: %v", err, err)
	}
	if appErr.Code != apperror.CodeMCPProxyUpstreamUnauthorized {
		t.Errorf("Code = %q, want %q", appErr.Code, apperror.CodeMCPProxyUpstreamUnauthorized)
	}
	if appErr.HTTPStatus != http.StatusBadRequest {
		t.Errorf("HTTPStatus = %d, want %d", appErr.HTTPStatus, http.StatusBadRequest)
	}
}

// The snapshot of what an upstream reported is control-plane-only. Nothing in the deployment
// spec carries it, which is why adding the field needed no data-version bump: a gateway cannot
// observe it, so no gateway has to understand it. MCPProxyDeploymentSpec is an allow-list
// struct, so this breaks the moment someone adds the field to it.
func TestBuildMCPDeploymentYAMLOmitsUpstreamSpecVersions(t *testing.T) {
	util := &MCPUtils{}
	ctx := "/mcp-test"
	proxy := &model.MCPProxy{
		Handle:  "test-mcp-proxy",
		Name:    "Test MCP Proxy",
		Version: "v1.0",
		Configuration: model.MCPProxyConfiguration{
			Context:              &ctx,
			SpecVersions:         []string{"2025-06-18", "2026-07-28"},
			UpstreamSpecVersions: []string{"2024-11-05", "2099-01-01"},
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{URL: "http://mcp-backend:8080/mcp"},
			},
		},
	}

	yamlString, err := util.GenerateMCPDeploymentYAML(proxy)
	if err != nil {
		t.Fatalf("GenerateMCPDeploymentYAML() error = %v", err)
	}

	if !strings.Contains(yamlString, "2025-06-18") || !strings.Contains(yamlString, "2026-07-28") {
		t.Errorf("declared versions missing from the deployment YAML:\n%s", yamlString)
	}
	for _, reported := range []string{"upstreamSpecVersions", "2024-11-05", "2099-01-01"} {
		if strings.Contains(yamlString, reported) {
			t.Errorf("deployment YAML carries %q, which no gateway understands:\n%s", reported, yamlString)
		}
	}
}

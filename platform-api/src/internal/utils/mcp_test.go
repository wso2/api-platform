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
	"testing"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"

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
	if deploymentStruct.Spec.SpecVersion != "2025-06-18" {
		t.Errorf("SpecVersion = %q", deploymentStruct.Spec.SpecVersion)
	}
}

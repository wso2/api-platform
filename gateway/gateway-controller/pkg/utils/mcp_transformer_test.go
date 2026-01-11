/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package utils

import (
	"testing"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

func TestProtocolVersionComparator(t *testing.T) {
	cases := []struct {
		base    string
		current string
		expect  bool
	}{
		{constants.SPEC_VERSION_2025_JUNE, constants.SPEC_VERSION_2025_JUNE, true},
		{"2025-01-01", constants.SPEC_VERSION_2025_JUNE, true},
		{"2026-01-01", constants.SPEC_VERSION_2025_JUNE, false},
		{"2024-12-31", "2025-12-31", true},
	}
	for _, c := range cases {
		got := protocolVersionComparator(c.base, c.current)
		if got != c.expect {
			t.Fatalf("protocolVersionComparator(%s,%s)=%v, want %v", c.base, c.current, got, c.expect)
		}
	}
}

func TestAddMCPSpecificOperations_DefaultVersion(t *testing.T) {
	// SpecVersion nil should use LATEST_SUPPORTED_MCP_SPEC_VERSION
	cfg := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			SpecVersion: nil,
		},
	}
	ops := addMCPSpecificOperations(cfg, false)
	// baseline operations count: GET, POST, DELETE on MCP_RESOURCE_PATH
	// if latest >= 2025-06-01, metadata path GET should be present
	wantBase := 3
	want := wantBase
	if protocolVersionComparator(constants.SPEC_VERSION_2025_JUNE, LATEST_SUPPORTED_MCP_SPEC_VERSION) {
		want = wantBase + 1
	}
	if len(ops) != want {
		t.Fatalf("expected %d operations, got %d", want, len(ops))
	}
	// verify paths/methods contain the MCP base ops
	baseMethods := map[api.OperationMethod]bool{api.OperationMethod(api.GET): true,
		api.OperationMethod(api.POST): true, api.OperationMethod(api.DELETE): true}
	basePath := constants.MCP_RESOURCE_PATH
	foundBase := 0
	foundPRM := false
	for _, op := range ops {
		if op.Path == basePath && baseMethods[op.Method] {
			foundBase++
		}
		if op.Path == constants.MCP_PRM_RESOURCE_PATH && op.Method == api.OperationMethod(api.GET) {
			foundPRM = true
		}
	}
	if foundBase != wantBase {
		t.Fatalf("expected %d base ops on %s, found %d", wantBase, basePath, foundBase)
	}
	if protocolVersionComparator(constants.SPEC_VERSION_2025_JUNE, LATEST_SUPPORTED_MCP_SPEC_VERSION) && !foundPRM {
		t.Fatalf("expected protected resources metadata GET operation to be present")
	}
}

func TestAddMCPSpecificOperations_SpecifiedVersion(t *testing.T) {
	v := constants.SPEC_VERSION_2025_JUNE
	cfg := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			SpecVersion: &v,
		},
	}
	ops := addMCPSpecificOperations(cfg, false)
	// Expect 4 operations including metadata GET
	if len(ops) != 4 {
		t.Fatalf("expected 4 operations for spec %s, got %d", v, len(ops))
	}
	foundPRM := false
	for _, op := range ops {
		if op.Path == constants.MCP_PRM_RESOURCE_PATH && op.Method == api.OperationMethod(api.GET) {
			foundPRM = true
		}
	}
	if !foundPRM {
		t.Fatalf("expected protected resources metadata GET operation for spec %s", v)
	}
}

func TestMCPTransformer_Transform(t *testing.T) {
	name := "petstore"
	version := "1.0.0"
	context := "/petstore"
	url := "http://backend:8080"
	upstream := api.MCPProxyConfigData_Upstream{
		Url: &url,
	}
	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	in := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			DisplayName: name,
			Version:     version,
			Context:     &context,
			Upstream:    upstream,
			SpecVersion: &latest,
		},
	}
	var out api.APIConfiguration
	tr := &MCPTransformer{}
	res, err := tr.Transform(in, &out)
	if err != nil {
		t.Fatalf("Transform returned an error: %v", err)
	}

	apiData, err := res.Spec.AsAPIConfigData()
	if err != nil {
		t.Fatalf("Transform produced invalid API config data: %v", err)
	}

	if apiData.DisplayName != name || apiData.Version != version || apiData.Context != context {
		t.Fatalf("Transform did not copy basic fields correctly: got %+v", res.Spec)
	}
	if apiData.Upstream.Main.Url == nil {
		t.Fatalf("Transform did not set apiData.Upstream.Main.Url")
	}
	if *apiData.Upstream.Main.Url != "http://backend:8080" {
		t.Fatalf("Transform did not copy upstreams correctly: got %+v", *apiData.Upstream.Main.Url)
	}
	// Should include MCP operations
	ops := apiData.Operations
	if len(ops) < 3 {
		t.Fatalf("expected at least 3 MCP operations, got %d", len(ops))
	}
	// Ensure kind and version set
	if res.Kind != api.RestApi {
		t.Fatalf("expected Kind Httprest, got %s", res.Kind)
	}
	if res.ApiVersion != api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1 {
		t.Fatalf("expected Version ApiPlatformWso2Comv1, got %s", res.ApiVersion)
	}
}

func TestMCPTransformer_Transform_WithPoliciesAndUpstreamAuth(t *testing.T) {
	name := "petstore"
	version := "1.0.0"
	context := "/petstore"
	url := "http://backend:8080"
	authHeader := "Authorization"
	authValue := "Bearer token-xyz"
	authType := api.MCPProxyConfigDataUpstreamAuthType("bearer")

	upstream := api.MCPProxyConfigData_Upstream{
		Url: &url,
		Auth: &struct {
			Header *string                                `json:"header,omitempty" yaml:"header,omitempty"`
			Type   api.MCPProxyConfigDataUpstreamAuthType `json:"type" yaml:"type"`
			Value  *string                                `json:"value,omitempty" yaml:"value,omitempty"`
		}{
			Header: &authHeader,
			Type:   authType,
			Value:  &authValue,
		},
	}

	existingPolicy := api.Policy{
		Name:    "SomePolicy",
		Version: "v1",
	}
	policies := []api.Policy{existingPolicy}

	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	in := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			DisplayName: name,
			Version:     version,
			Context:     &context,
			Upstream:    upstream,
			SpecVersion: &latest,
			Policies:    &policies,
		},
	}

	var out api.APIConfiguration
	tr := &MCPTransformer{}
	res, err := tr.Transform(in, &out)
	if err != nil {
		t.Fatalf("Transform returned an error: %v", err)
	}

	apiData, err := res.Spec.AsAPIConfigData()
	if err != nil {
		t.Fatalf("Transform produced invalid API config data: %v", err)
	}

	if apiData.Policies == nil {
		t.Fatalf("Expected policies to be present")
	}

	resPolicies := *apiData.Policies
	if len(resPolicies) != 2 {
		t.Fatalf("Expected 2 policies, got %d", len(resPolicies))
	}

	// Check first policy is the existing one
	if resPolicies[0].Name != existingPolicy.Name {
		t.Errorf("Expected first policy to be %s, got %s", existingPolicy.Name, resPolicies[0].Name)
	}

	// Check second policy is the modify headers policy
	if resPolicies[1].Name != constants.MODIFY_HEADERS_POLICY_NAME {
		t.Errorf("Expected last policy to be %s, got %s", constants.MODIFY_HEADERS_POLICY_NAME, resPolicies[1].Name)
	}
}

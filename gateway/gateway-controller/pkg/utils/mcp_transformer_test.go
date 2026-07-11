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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
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
	baseMethods := map[api.OperationMethod]bool{api.OperationMethodGET: true,
		api.OperationMethodPOST: true, api.OperationMethodDELETE: true}
	basePath := constants.MCP_RESOURCE_PATH
	foundBase := 0
	foundPRM := false
	for _, op := range ops {
		if op.Path == basePath && baseMethods[op.Method] {
			foundBase++
		}
		if op.Path == constants.MCP_PRM_RESOURCE_PATH && op.Method == api.OperationMethodGET {
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
		if op.Path == constants.MCP_PRM_RESOURCE_PATH && op.Method == api.OperationMethodGET {
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
	var out api.RestAPI
	tr := &MCPTransformer{}
	res, err := tr.Transform(in, &out)
	if err != nil {
		t.Fatalf("Transform returned an error: %v", err)
	}

	apiData := res.Spec

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
	if res.Kind != api.RestAPIKindRestApi {
		t.Fatalf("expected Kind Httprest, got %s", res.Kind)
	}
	if res.ApiVersion != api.RestAPIApiVersionGatewayApiPlatformWso2Comv1 {
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

	var out api.RestAPI
	tr := &MCPTransformer{}
	res, err := tr.Transform(in, &out)
	if err != nil {
		t.Fatalf("Transform returned an error: %v", err)
	}

	apiData := res.Spec

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

	// Check second policy is the set headers policy
	if resPolicies[1].Name != constants.SET_HEADERS_POLICY_NAME {
		t.Errorf("Expected last policy to be %s, got %s", constants.SET_HEADERS_POLICY_NAME, resPolicies[1].Name)
	}
}

func TestNewMCPTransformer(t *testing.T) {
	tr := NewMCPTransformer()
	if tr == nil {
		t.Fatal("Expected non-nil MCPTransformer")
	}
}

func TestMCPTransformer_Transform_InvalidInput(t *testing.T) {
	tr := NewMCPTransformer()
	var out api.RestAPI

	// Test with nil input
	_, err := tr.Transform(nil, &out)
	if err == nil {
		t.Error("Expected error for nil input")
	}

	// Test with wrong type
	_, err = tr.Transform("not a valid type", &out)
	if err == nil {
		t.Error("Expected error for invalid type")
	}
}

func TestMCPTransformer_Transform_WithVhost(t *testing.T) {
	name := "vhost-test"
	version := "1.0.0"
	url := "http://backend:8080"
	vhost := "api.example.com"

	upstream := api.MCPProxyConfigData_Upstream{
		Url: &url,
	}

	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	in := &api.MCPProxyConfiguration{
		Metadata: api.Metadata{Name: "vhost-test-proxy"},
		Spec: api.MCPProxyConfigData{
			DisplayName: name,
			Version:     version,
			Upstream:    upstream,
			SpecVersion: &latest,
			Vhost:       &vhost,
		},
	}

	var out api.RestAPI
	tr := &MCPTransformer{}
	res, err := tr.Transform(in, &out)
	if err != nil {
		t.Fatalf("Transform returned an error: %v", err)
	}

	apiData := res.Spec

	if apiData.Vhosts == nil {
		t.Fatal("Expected Vhosts to be set")
	}
	if apiData.Vhosts.Main != vhost {
		t.Errorf("Expected vhost %s, got %s", vhost, apiData.Vhosts.Main)
	}
}

func TestMCPTransformer_Transform_WithCORSPolicy(t *testing.T) {
	name := "cors-test"
	version := "1.0.0"
	url := "http://backend:8080"

	upstream := api.MCPProxyConfigData_Upstream{
		Url: &url,
	}

	corsPolicy := api.Policy{
		Name:    "CORS",
		Version: "v1.0.0",
	}
	policies := []api.Policy{corsPolicy}

	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	in := &api.MCPProxyConfiguration{
		Metadata: api.Metadata{Name: "cors-test-proxy"},
		Spec: api.MCPProxyConfigData{
			DisplayName: name,
			Version:     version,
			Upstream:    upstream,
			SpecVersion: &latest,
			Policies:    &policies,
		},
	}

	var out api.RestAPI
	tr := &MCPTransformer{}
	res, err := tr.Transform(in, &out)
	if err != nil {
		t.Fatalf("Transform returned an error: %v", err)
	}

	apiData := res.Spec

	// With CORS policy, OPTIONS operations should be included
	foundOptions := false
	for _, op := range apiData.Operations {
		if op.Method == api.OperationMethodOPTIONS {
			foundOptions = true
			break
		}
	}

	if !foundOptions {
		t.Error("Expected OPTIONS operations to be present when CORS policy is enabled")
	}
}

func TestMCPTransformer_Transform_WithoutContext(t *testing.T) {
	name := "no-context-test"
	version := "1.0.0"
	url := "http://backend:8080"

	upstream := api.MCPProxyConfigData_Upstream{
		Url: &url,
	}

	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	in := &api.MCPProxyConfiguration{
		Metadata: api.Metadata{Name: "no-context-proxy"},
		Spec: api.MCPProxyConfigData{
			DisplayName: name,
			Version:     version,
			Upstream:    upstream,
			SpecVersion: &latest,
			Context:     nil, // No context
		},
	}

	var out api.RestAPI
	tr := &MCPTransformer{}
	res, err := tr.Transform(in, &out)
	if err != nil {
		t.Fatalf("Transform returned an error: %v", err)
	}

	apiData := res.Spec

	// Context should be empty when not provided
	if apiData.Context != "" {
		t.Errorf("Expected empty context, got %s", apiData.Context)
	}
}

func TestMCPTransformer_Transform_WithEmptySpecVersion(t *testing.T) {
	name := "empty-version-test"
	version := "1.0.0"
	url := "http://backend:8080"

	upstream := api.MCPProxyConfigData_Upstream{
		Url: &url,
	}

	emptyVersion := ""
	in := &api.MCPProxyConfiguration{
		Metadata: api.Metadata{Name: "empty-version-proxy"},
		Spec: api.MCPProxyConfigData{
			DisplayName: name,
			Version:     version,
			Upstream:    upstream,
			SpecVersion: &emptyVersion, // Empty version string
		},
	}

	var out api.RestAPI
	tr := &MCPTransformer{}
	res, err := tr.Transform(in, &out)
	if err != nil {
		t.Fatalf("Transform returned an error: %v", err)
	}

	// Should use default/latest version
	apiData := res.Spec

	// Should have operations
	if len(apiData.Operations) == 0 {
		t.Error("Expected operations to be present")
	}
}

func TestAddMCPSpecificOperations_WithOptions(t *testing.T) {
	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	cfg := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			SpecVersion: &latest,
		},
	}

	// Test with options required (CORS enabled)
	ops := addMCPSpecificOperations(cfg, true)

	// Should have OPTIONS operations
	foundOptions := 0
	for _, op := range ops {
		if op.Method == api.OperationMethodOPTIONS {
			foundOptions++
		}
	}

	if foundOptions == 0 {
		t.Error("Expected OPTIONS operations when optionsRequired is true")
	}
}

func TestAddMCPSpecificOperations_OlderVersion(t *testing.T) {
	// Test with a version before 2025-06-18
	olderVersion := "2024-01-01"
	cfg := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			SpecVersion: &olderVersion,
		},
	}

	ops := addMCPSpecificOperations(cfg, false)

	// Should only have base operations (GET, POST, DELETE on /mcp)
	if len(ops) != 3 {
		t.Errorf("Expected 3 operations for older spec version, got %d", len(ops))
	}

	// Should not have PRM resource path
	for _, op := range ops {
		if op.Path == constants.MCP_PRM_RESOURCE_PATH {
			t.Error("Should not have PRM resource path for older spec version")
		}
	}
}

func TestGetParamsOfPolicy_MCP(t *testing.T) {
	params, err := GetParamsOfPolicy(constants.SET_HEADERS_POLICY_PARAMS, "X-Custom-Header", "custom-value")
	if err != nil {
		t.Fatalf("GetParamsOfPolicy returned error: %v", err)
	}

	if params == nil {
		t.Fatal("Expected non-nil params")
	}

	// Check that request is present
	request, ok := params["request"].(map[string]any)
	if !ok {
		t.Fatal("Expected request object in params")
	}
	if _, ok := request["headers"]; !ok {
		t.Error("Expected request.headers in params")
	}
}

// ============================================================================
// Resilience route-mapping tests
// ============================================================================

// mcpOpResilience returns the resilience block attached to the operation matching method+path,
// or nil if the operation is absent or carries no resilience.
func mcpOpResilience(ops []api.Operation, method api.OperationMethod, path string) *api.Resilience {
	for _, op := range ops {
		if op.Method == method && op.Path == path {
			return op.Resilience
		}
	}
	return nil
}

// When no resilience is set, the MCP forwarding routes (GET/POST/DELETE /mcp) default the route
// timeout to "0s" (disabled) — MCP is streaming, so a finite total cap would sever a live stream.
// The local-response routes (OPTIONS, PRM) must carry no resilience. See mcp-timeout-divergence.md.
func TestMCPTransform_Resilience_DefaultsRouteTimeoutDisabled(t *testing.T) {
	context := "/everything"
	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	in := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			DisplayName: "everything",
			Version:     "v1.0",
			Context:     &context,
			SpecVersion: &latest,
			Upstream:    api.MCPProxyConfigData_Upstream{Url: stringPtr("http://backend:8080")},
			// A cors policy makes the transformer emit the OPTIONS routes so we can assert they are skipped.
			Policies: &[]api.Policy{{Name: "cors", Version: "v1"}},
		},
	}

	res, err := (&MCPTransformer{}).Transform(in, &api.RestAPI{})
	require.NoError(t, err)
	ops := res.Spec.Operations

	for _, m := range []api.OperationMethod{api.OperationMethodGET, api.OperationMethodPOST, api.OperationMethodDELETE} {
		r := mcpOpResilience(ops, m, constants.MCP_RESOURCE_PATH)
		require.NotNil(t, r, "forwarding %s %s should carry resilience", m, constants.MCP_RESOURCE_PATH)
		require.NotNil(t, r.Timeout)
		assert.Equal(t, "0s", *r.Timeout, "route timeout should default to disabled for MCP")
		assert.Nil(t, r.IdleTimeout, "idle timeout should be left to the global default when unset")
	}

	// Local-response routes must not carry a route timeout.
	assert.Nil(t, mcpOpResilience(ops, api.OperationMethodOPTIONS, constants.MCP_RESOURCE_PATH), "OPTIONS /mcp is a local reply")
	assert.Nil(t, mcpOpResilience(ops, api.OperationMethodGET, constants.MCP_PRM_RESOURCE_PATH), "PRM route is a local reply")
	assert.Nil(t, mcpOpResilience(ops, api.OperationMethodOPTIONS, constants.MCP_PRM_RESOURCE_PATH), "OPTIONS PRM is a local reply")
}

// An explicit resilience block overrides the disabled default and its idleTimeout is carried through,
// on the forwarding routes only.
func TestMCPTransform_Resilience_UserOverride(t *testing.T) {
	context := "/everything"
	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	in := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			DisplayName: "everything",
			Version:     "v1.0",
			Context:     &context,
			SpecVersion: &latest,
			Upstream:    api.MCPProxyConfigData_Upstream{Url: stringPtr("http://backend:8080")},
			Resilience:  &api.Resilience{Timeout: stringPtr("30s"), IdleTimeout: stringPtr("120s")},
		},
	}

	res, err := (&MCPTransformer{}).Transform(in, &api.RestAPI{})
	require.NoError(t, err)
	ops := res.Spec.Operations

	r := mcpOpResilience(ops, api.OperationMethodGET, constants.MCP_RESOURCE_PATH)
	require.NotNil(t, r)
	require.NotNil(t, r.Timeout)
	assert.Equal(t, "30s", *r.Timeout)
	require.NotNil(t, r.IdleTimeout)
	assert.Equal(t, "120s", *r.IdleTimeout)

	// The PRM route (present for spec >= 2025-06-18) is still skipped.
	assert.Nil(t, mcpOpResilience(ops, api.OperationMethodGET, constants.MCP_PRM_RESOURCE_PATH))
}

// A user timeout with no idle override: timeout is the user value, idle stays unset (global default).
func TestMCPTransform_Resilience_TimeoutOnly(t *testing.T) {
	context := "/everything"
	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	in := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			DisplayName: "everything",
			Version:     "v1.0",
			Context:     &context,
			SpecVersion: &latest,
			Upstream:    api.MCPProxyConfigData_Upstream{Url: stringPtr("http://backend:8080")},
			Resilience:  &api.Resilience{Timeout: stringPtr("45s")},
		},
	}

	res, err := (&MCPTransformer{}).Transform(in, &api.RestAPI{})
	require.NoError(t, err)

	r := mcpOpResilience(res.Spec.Operations, api.OperationMethodPOST, constants.MCP_RESOURCE_PATH)
	require.NotNil(t, r)
	require.NotNil(t, r.Timeout)
	assert.Equal(t, "45s", *r.Timeout)
	assert.Nil(t, r.IdleTimeout)
}

// ============================================================================
// Upstream ref / connect-timeout threading tests
// ============================================================================

// The MCP converter maps an upstream `ref` onto the derived RestAPI and threads the
// upstreamDefinitions through, so the per-upstream connect timeout resolves like it does for RestApi.
func TestMCPTransform_UpstreamRef_ThreadsDefinitions(t *testing.T) {
	context := "/everything"
	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	defs := []api.UpstreamDefinition{{
		Name:    "mcp-backend",
		Timeout: &api.UpstreamTimeout{Connect: stringPtr("6s")},
	}}
	in := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			DisplayName:         "everything",
			Version:             "v1.0",
			Context:             &context,
			SpecVersion:         &latest,
			UpstreamDefinitions: &defs,
			Upstream:            api.MCPProxyConfigData_Upstream{Ref: stringPtr("mcp-backend")},
		},
	}

	var out api.RestAPI
	res, err := (&MCPTransformer{}).Transform(in, &out)
	require.NoError(t, err)

	require.NotNil(t, res.Spec.Upstream.Main.Ref)
	assert.Equal(t, "mcp-backend", *res.Spec.Upstream.Main.Ref)
	assert.Nil(t, res.Spec.Upstream.Main.Url)

	require.NotNil(t, res.Spec.UpstreamDefinitions)
	require.Len(t, *res.Spec.UpstreamDefinitions, 1)
	require.NotNil(t, (*res.Spec.UpstreamDefinitions)[0].Timeout)
	assert.Equal(t, "6s", *(*res.Spec.UpstreamDefinitions)[0].Timeout.Connect)
}

func TestMCPTransform_UpstreamUrl_Unchanged(t *testing.T) {
	context := "/everything"
	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	in := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			DisplayName: "everything",
			Version:     "v1.0",
			Context:     &context,
			SpecVersion: &latest,
			Upstream:    api.MCPProxyConfigData_Upstream{Url: stringPtr("http://backend:8080")},
		},
	}

	var out api.RestAPI
	res, err := (&MCPTransformer{}).Transform(in, &out)
	require.NoError(t, err)

	require.NotNil(t, res.Spec.Upstream.Main.Url)
	assert.Equal(t, "http://backend:8080", *res.Spec.Upstream.Main.Url)
	assert.Nil(t, res.Spec.Upstream.Main.Ref)
	assert.Nil(t, res.Spec.UpstreamDefinitions)
}

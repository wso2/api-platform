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

package service

// Each artifact service applies network access control to its create and update
// requests. These tests drive the services themselves, so a hook that is missing, or
// that runs after the request is stored, fails here. Every refusal is paired with the
// same call on a permitted URL.

import (
	"context"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// refusedClusterURL is denied by clusterDenyRule; permittedURL is not.
const (
	refusedClusterURL = "http://backend.default.svc.cluster.local:8080"
	permittedURL      = "https://backend.example.com"
)

func refusedOverride() *[]api.AssociatedGateway {
	return endpointOverride("gateway", refusedClusterURL)
}

func TestAgentProxyService_ChecksRequestURLs(t *testing.T) {
	useRequestAccessControl(t, clusterDenyRule)

	req := validAgentProxyRequest()
	req.Upstream.Main.Url = strPointer(refusedClusterURL)
	_, err := newAgentProxyTestService(&mockAgentProxyRepository{}).Create(agentTestOrg, "alice", req)
	assertRefusedField(t, err, "upstream.main.url")

	stored := storedAgentProxy(constants.OriginCP)
	repo := &mockAgentProxyRepository{stored: stored}
	svc := newAgentProxyTestService(repo)
	_, err = svc.Update(agentTestOrg, stored.Handle, "alice", req)
	assertRefusedField(t, err, "upstream.main.url")
	if repo.stored.Configuration.Upstream.Main.URL != "http://weather-agent:9000" {
		t.Fatalf("a refused update stored upstream %q", repo.stored.Configuration.Upstream.Main.URL)
	}

	req.Upstream.Main.Url = strPointer(permittedURL)
	if _, err := svc.Update(agentTestOrg, stored.Handle, "alice", req); err != nil {
		t.Fatalf("Update with a permitted upstream: %v", err)
	}
}

// A DP-originated proxy keeps its stored upstream, so the request's copy is not
// checked; its per-gateway overrides are stored by the control plane, so they are.
func TestAgentProxyService_DPOriginUpdateChecksOnlyGatewayOverrides(t *testing.T) {
	useRequestAccessControl(t, clusterDenyRule)
	stored := storedAgentProxy(constants.OriginDP)
	stored.Configuration.Upstream.Main.URL = refusedClusterURL // accepted by its gateway
	repo := &mockAgentProxyRepository{stored: stored}
	svc := newAgentProxyTestService(repo)

	req := validAgentProxyRequest()
	req.Upstream.Main.Url = strPointer(refusedClusterURL) // echoed back from GET
	description := "Only the description changes"
	req.Description = &description
	if _, err := svc.Update(agentTestOrg, stored.Handle, "alice", req); err != nil {
		t.Fatalf("a description edit of a DP-originated proxy: %v", err)
	}

	req.AssociatedGateways = refusedOverride()
	_, err := svc.Update(agentTestOrg, stored.Handle, "alice", req)
	assertRefusedField(t, err, "associatedGateways[0].configurations.endpointUrl")
}

func newProviderServiceForImportTest(d *importTestDeps) *LLMProviderService {
	return NewLLMProviderService(
		repository.NewLLMProviderRepo(d.db), d.templateRepo, repository.NewOrganizationRepo(d.db), nil, d.deployment,
		repository.NewGatewayRepo(d.db), nil, newTestLogger(), &noopAuditRepo{}, &config.Server{},
		NewIdentityService(repository.NewUserIdentityMappingRepo(d.db)),
	)
}

func providerRequest(id, templateHandle, upstreamURL string) *api.LLMProvider {
	return &api.LLMProvider{
		Id: strPointer(id), DisplayName: "Provider " + id, Version: "v1.0", Template: templateHandle,
		Upstream:      upstreamOf(upstreamURL, ""),
		AccessControl: api.LLMAccessControl{Mode: api.LLMAccessControlMode("deny_all")},
	}
}

func TestLLMProviderService_ChecksRequestURLs(t *testing.T) {
	d := setupImportTest(t)
	const templateHandle = "netac-template"
	importDPTemplate(t, d, "dpt-netac-0000-0000-000000000001", templateHandle)
	importDPProvider(t, d, "dpp-netac-0000-0000-000000000001", "dp-netac-provider", templateHandle)
	svc := newProviderServiceForImportTest(d)
	useRequestAccessControl(t, clusterDenyRule)

	_, err := svc.Create(importTestOrgID, "alice", providerRequest("cp-refused", templateHandle, refusedClusterURL))
	assertRefusedField(t, err, "upstream.main.url")

	created, err := svc.Create(importTestOrgID, "alice", providerRequest("cp-allowed", templateHandle, permittedURL))
	if err != nil {
		t.Fatalf("Create with a permitted upstream: %v", err)
	}
	_, err = svc.Update(importTestOrgID, *created.Id, "alice", providerRequest("cp-allowed", templateHandle, refusedClusterURL))
	assertRefusedField(t, err, "upstream.main.url")

	// DP-originated: the request's upstream is ignored and not checked; an override is.
	dpReq := providerRequest("dp-netac-provider", templateHandle, refusedClusterURL)
	if _, err := svc.Update(importTestOrgID, "dp-netac-provider", "alice", dpReq); err != nil {
		t.Fatalf("an update of a DP-originated provider: %v", err)
	}
	dpReq.AssociatedGateways = refusedOverride()
	_, err = svc.Update(importTestOrgID, "dp-netac-provider", "alice", dpReq)
	assertRefusedField(t, err, "associatedGateways[0].configurations.endpointUrl")
}

func TestLLMProxyService_ChecksRequestURLs(t *testing.T) {
	d := setupImportTest(t)
	const templateHandle = "netac-proxy-template"
	const providerHandle = "netac-proxy-provider"
	importDPTemplate(t, d, "dpt-netac-0000-0000-000000000002", templateHandle)
	importDPProvider(t, d, "dpp-netac-0000-0000-000000000002", providerHandle, templateHandle)
	svc := NewLLMProxyService(
		repository.NewLLMProxyRepo(d.db), repository.NewLLMProviderRepo(d.db), repository.NewProjectRepo(d.db),
		d.deployment, repository.NewGatewayRepo(d.db), nil, newTestLogger(), &noopAuditRepo{}, &config.Server{},
		NewIdentityService(repository.NewUserIdentityMappingRepo(d.db)),
	)
	useRequestAccessControl(t, clusterDenyRule)

	proxy := func(id string) *api.LLMProxy {
		return &api.LLMProxy{Id: strPointer(id), DisplayName: "Proxy " + id, Version: "v1.0", ProjectId: "default",
			Provider: &api.LLMProxyProvider{Id: providerHandle}}
	}
	refused := proxy("netac-proxy-refused")
	refused.AssociatedGateways = refusedOverride()
	_, err := svc.Create(importTestOrgID, "alice", refused)
	assertRefusedField(t, err, "associatedGateways[0].configurations.endpointUrl")

	created, err := svc.Create(importTestOrgID, "alice", proxy("netac-proxy-allowed"))
	if err != nil {
		t.Fatalf("Create without an override: %v", err)
	}
	update := proxy("netac-proxy-allowed")
	update.AssociatedGateways = refusedOverride()
	_, err = svc.Update(importTestOrgID, *created.Id, "alice", update)
	assertRefusedField(t, err, "associatedGateways[0].configurations.endpointUrl")
}

func TestMCPProxyService_ChecksRequestURLs(t *testing.T) {
	d := setupImportTest(t)
	mustImport(t, d, dpMCPReq("dp-netac-mcp-0001", "dp-netac-mcp", "DP NETAC MCP"))
	svc := NewMCPProxyService(
		repository.NewMCPProxyRepo(d.db), repository.NewProjectRepo(d.db), d.deployment, repository.NewGatewayRepo(d.db),
		nil, newTestLogger(), &noopAuditRepo{}, &config.Server{},
		NewIdentityService(repository.NewUserIdentityMappingRepo(d.db)),
	)
	useRequestAccessControl(t, clusterDenyRule)

	mcp := func(id, upstreamURL string) *api.MCPProxy {
		return &api.MCPProxy{Id: strPointer(id), DisplayName: "MCP " + id, Version: "v1.0", ProjectId: strPointer("default"),
			Upstream: upstreamOf(upstreamURL, "")}
	}
	_, err := svc.Create(importTestOrgID, "alice", mcp("netac-mcp-refused", refusedClusterURL+"/mcp"))
	assertRefusedField(t, err, "upstream.main.url")

	created, err := svc.Create(importTestOrgID, "alice", mcp("netac-mcp-allowed", permittedURL+"/mcp"))
	if err != nil {
		t.Fatalf("Create with a permitted upstream: %v", err)
	}
	_, err = svc.Update(importTestOrgID, *created.Id, "alice", mcp("netac-mcp-allowed", refusedClusterURL+"/mcp"))
	assertRefusedField(t, err, "upstream.main.url")

	// DP-originated: the request's upstream is ignored and not checked; an override is.
	dpReq := mcp("dp-netac-mcp", refusedClusterURL+"/mcp")
	if _, err := svc.Update(importTestOrgID, "dp-netac-mcp", "alice", dpReq); err != nil {
		t.Fatalf("an update of a DP-originated MCP proxy: %v", err)
	}
	dpReq.AssociatedGateways = refusedOverride()
	_, err = svc.Update(importTestOrgID, "dp-netac-mcp", "alice", dpReq)
	assertRefusedField(t, err, "associatedGateways[0].configurations.endpointUrl")
}

func TestGraphQLAPIService_ChecksRequestURLs(t *testing.T) {
	useRequestAccessControl(t, clusterDenyRule)
	svc := newGraphQLTestService(&mockGraphQLAPIRepo{}, &model.Project{ID: "proj-1", Handle: "default"})

	_, err := svc.Create("org-1", "alice", &api.CreateGraphQLAPIRequest{
		DisplayName: "Refused", Version: "v1", ProjectId: "default", Upstream: upstreamOf(refusedClusterURL+"/graphql", ""),
	})
	assertRefusedField(t, err, "upstream.main.url")

	_, err = svc.Update("org-1", "netac-gql", "alice", &api.GraphQLAPI{
		DisplayName: "Refused", Version: "v1", Upstream: upstreamOf(permittedURL+"/graphql", ""),
		SdlUrl: strPointer(refusedClusterURL + "/schema.graphql"),
	})
	assertRefusedField(t, err, "sdlUrl")
}

// Template URLs stay editable on DP-originated templates, so every path checks them,
// and a refused spec URL names metadata.openapiSpecUrl.
func TestLLMProviderTemplateService_ChecksRequestURLs(t *testing.T) {
	useRequestAccessControl(t, clusterDenyRule)
	refusedSpec := refusedClusterURL + "/openapi.yaml"
	repo := &mockLLMProviderTemplateCRUDRepo{}
	svc := NewLLMProviderTemplateService(repo, &noopAuditRepo{}, newTestIdentityService())

	req := validTemplateRequest("NETAC Template")
	req.Metadata.OpenapiSpecUrl = &refusedSpec
	_, err := svc.Create("org-1", "alice", req)
	assertRefusedField(t, err, "metadata.openapiSpecUrl")
	if repo.created != nil {
		t.Fatal("a refused create stored the template")
	}

	allowed := validTemplateRequest("NETAC Template")
	if _, err := svc.Create("org-1", "alice", allowed); err != nil {
		t.Fatalf("Create with permitted URLs: %v", err)
	}

	repo.getByIDFunc = func(id, _ string) (*model.LLMProviderTemplate, error) {
		return &model.LLMProviderTemplate{ID: id, Name: "NETAC Template", Version: "v1.0", ManagedBy: "organization"}, nil
	}
	_, err = svc.Update("org-1", "netac-template", "alice", req)
	assertRefusedField(t, err, "metadata.openapiSpecUrl")

	versionReq := &api.CreateLLMProviderTemplateVersionRequest{
		DisplayName: strPointer("NETAC Template"), Version: "v1.1",
		Metadata: &api.LLMProviderTemplateMetadata{EndpointUrl: strPointer(refusedClusterURL)},
	}
	_, err = svc.CreateVersion("org-1", "netac-template", "alice", versionReq)
	assertRefusedField(t, err, "metadata.endpointUrl")
	_, err = svc.CopyVersion("org-1", "netac-template", "", "v1.1", "alice", versionReq)
	assertRefusedField(t, err, "metadata.endpointUrl")
}

func TestDeploymentEndpointOverrides_AreChecked(t *testing.T) {
	useRequestAccessControl(t, clusterDenyRule)

	// REST: a refused override creates no deployment; a permitted one deploys.
	depRepo := &buildTestDeploymentRepo{}
	restSvc := newBuildTestService(&buildTestAPIRepo{apiModel: buildTestAPI()}, depRepo)
	deploy := func(endpoint string) error {
		_, err := restSvc.DeployAPI(buildTestAPIUUID, &api.DeployRequest{
			Name: "netac", Base: "current", GatewayId: "test-gateway",
			Metadata: &map[string]interface{}{constants.MetadataKeyEndpointUrl: endpoint},
		}, buildTestOrgUUID, "tester")
		return err
	}
	assertRefusedField(t, deploy(refusedClusterURL), "metadata.endpointUrl")
	if depRepo.created != nil {
		t.Fatal("a refused deploy created a deployment")
	}
	if err := deploy(permittedURL); err != nil {
		t.Fatalf("a deploy with a permitted override: %v", err)
	}
	if depRepo.created == nil {
		t.Fatal("a permitted deploy created no deployment")
	}

	// LLM provider: a refused override leaves the rendered upstream as it was.
	llmSvc := &LLMProviderDeploymentService{}
	rendered := providerWithUpstreamAuth("api-key", "")
	rendered.Spec.Upstream.URL = "https://api.openai.com"
	err := llmSvc.applyUpstreamOverrides(rendered, map[string]interface{}{constants.MetadataKeyEndpointUrl: refusedClusterURL}, "org-1")
	assertRefusedField(t, err, "metadata.endpointUrl")
	if rendered.Spec.Upstream.URL != "https://api.openai.com" {
		t.Fatalf("a refused override changed the upstream to %q", rendered.Spec.Upstream.URL)
	}
	if err := llmSvc.applyUpstreamOverrides(rendered, map[string]interface{}{constants.MetadataKeyEndpointUrl: permittedURL}, "org-1"); err != nil || rendered.Spec.Upstream.URL != permittedURL {
		t.Fatalf("a permitted override: upstream %q, err %v", rendered.Spec.Upstream.URL, err)
	}

	_, err = extractEndpointURLOverride(map[string]interface{}{constants.MetadataKeyEndpointUrl: refusedClusterURL + "/mcp"})
	assertRefusedField(t, err, "metadata.endpointUrl")
	got, err := extractEndpointURLOverride(map[string]interface{}{constants.MetadataKeyEndpointUrl: permittedURL + "/mcp"})
	if err != nil || got == nil || *got != permittedURL+"/mcp" {
		t.Fatalf("a permitted MCP override = %v, %v", got, err)
	}
}

// The request's JSON shape decides what is checked; anything that is not an object
// carries no URL fields.
func TestCheckRequestURLs_InputsWithoutURLFields(t *testing.T) {
	useRequestAccessControl(t, clusterDenyRule)
	for name, payload := range map[string]any{
		"a list":                 []string{refusedClusterURL},
		"a non-string upstream":  map[string]any{"upstream": map[string]any{"main": map[string]any{"url": 42}}},
		"a gateway without conf": map[string]any{"associatedGateways": []any{"gw-1", map[string]any{"id": "gw-2"}}},
	} {
		if err := CheckRequestURLs(context.Background(), "org-1", payload); err != nil {
			t.Errorf("%s: err = %v, want nil", name, err)
		}
	}
	if err := CheckRequestURLs(context.Background(), "org-1", nil); err != nil {
		t.Errorf("a nil payload: err = %v, want nil", err)
	}
	if got := hostOf("http://[::1"); got != "" {
		t.Errorf("hostOf of a URL that does not parse = %q, want empty", got)
	}
	if err := CheckRequestURLs(context.Background(), "org-1", map[string]any{"upstream": make(chan int)}); err == nil {
		t.Error("a payload that does not marshal: err = nil, want an error")
	}
}

func TestDestinationOf(t *testing.T) {
	cases := []struct {
		raw, want string
		checked   bool
	}{
		{"https://api.example.com/v1", "https://api.example.com/v1", true},
		{"", "", false},
		// A placeholder only the gateway can resolve: the whole URL, the scheme or the host.
		{`{{ secret "backend-url" }}`, "", false},
		{`{{ env "SCHEME" }}://api.example.com`, "", false},
		{`https://{{ env "HOST" }}/v1`, "", false},
		{`https://api.{{ env "DOMAIN" }}:8443`, "", false},
		// A placeholder anywhere else leaves the literal host to check.
		{`http://orders.default.svc.cluster.local/{{"x"}}`, "http://orders.default.svc.cluster.local", true},
		{`https://api.example.com:8443/v1?key={{ secret "k" }}#frag`, "https://api.example.com:8443", true},
		{`https://{{ secret "user" }}@orders.default.svc.cluster.local/v1`, "https://orders.default.svc.cluster.local", true},
		{`https://{{ secret "user" }}@/v1`, "", false},
	}
	for _, tc := range cases {
		got, checked := destinationOf(tc.raw)
		if got != tc.want || checked != tc.checked {
			t.Errorf("destinationOf(%q) = (%q, %v), want (%q, %v)", tc.raw, got, checked, tc.want, tc.checked)
		}
	}
}

// A placeholder in the path must not hide a refused host, while one in the host is the
// gateway's to check.
func TestCheckRequestURLs_PlaceholdersOutsideTheHostDoNotExemptIt(t *testing.T) {
	useRequestAccessControl(t, clusterDenyRule)
	hidden := &api.CreateRESTAPIRequest{Upstream: upstreamOf(`http://orders.default.svc.cluster.local/{{"x"}}`, "")}
	assertRefusedField(t, CheckRequestURLs(context.Background(), "org-1", hidden), "upstream.main.url")

	templatedHost := &api.CreateRESTAPIRequest{Upstream: upstreamOf(`https://{{ env "BACKEND_HOST" }}/v1`, "")}
	if err := CheckRequestURLs(context.Background(), "org-1", templatedHost); err != nil {
		t.Errorf("a templated host: err = %v, want nil", err)
	}
}

// CheckRequestURLsForUpdate checks everything for a control-plane artifact and only the
// per-gateway overrides for a DP-originated one.
func TestCheckRequestURLsForUpdate(t *testing.T) {
	useRequestAccessControl(t, clusterDenyRule)
	upstreamOnly := &api.MCPProxy{Upstream: upstreamOf(refusedClusterURL, "")}
	withOverride := &api.MCPProxy{Upstream: upstreamOf(refusedClusterURL, ""), AssociatedGateways: refusedOverride()}

	assertRefusedField(t, CheckRequestURLsForUpdate(context.Background(), "org-1", constants.OriginCP, upstreamOnly), "upstream.main.url")
	if err := CheckRequestURLsForUpdate(context.Background(), "org-1", constants.OriginDP, upstreamOnly); err != nil {
		t.Errorf("a DP-originated upstream: err = %v, want nil", err)
	}
	assertRefusedField(t, CheckRequestURLsForUpdate(context.Background(), "org-1", constants.OriginDP, withOverride),
		"associatedGateways[0].configurations.endpointUrl")
}

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

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/repository"
)

func strPointer(s string) *string { return &s }

// TestArtifactImport_GeneratesCPUUIDAcrossKinds verifies the DP->CP flow mints a fresh
// control-plane UUID for every artifact kind (the data-plane UUID sent in the request is
// never reused as the CP UUID), returns it in the response, and that cross-artifact
// references — which are expressed as handles, not UUIDs — resolve correctly
// (proxy->provider, provider->template).
func TestArtifactImport_GeneratesCPUUIDAcrossKinds(t *testing.T) {
	d := setupImportTest(t)
	now := time.Now()

	mkReq := func(id, kind, name, project string, spec map[string]interface{}) dto.ImportGatewayArtifactRequest {
		md := dto.ArtifactImportMetadata{Name: name}
		if project != "" {
			md.Annotations = projectAnnotations(project)
		}
		return dto.ImportGatewayArtifactRequest{
			DPID:   id,
			Status: "deployed",
			Configuration: dto.ArtifactImportConfig{
				APIVersion: "api-platform.wso2.com/v1",
				Kind:       kind,
				Metadata:   md,
				Spec:       spec,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	// Data-plane UUIDs sent in the requests; the CP must NOT reuse any of these.
	const (
		restDPID  = "uuid-rest-0000-0000-000000000001"
		tmplDPID  = "uuid-tmpl-0000-0000-000000000002"
		provDPID  = "uuid-prov-0000-0000-000000000003"
		proxyDPID = "uuid-prxy-0000-0000-000000000004"
		mcpDPID   = "uuid-mcp0-0000-0000-000000000005"
	)

	// REST API (artifacts table).
	restResp, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		mkReq(restDPID, constants.RestApi, "dp-rest", "default", map[string]interface{}{"context": "/r"}))
	if err != nil {
		t.Fatalf("import REST: %v", err)
	}
	// LLM Provider Template (own table) + Provider referencing it by handle.
	importDPTemplate(t, d, tmplDPID, "dp-tmpl")
	importDPProvider(t, d, provDPID, "dp-prov", "dp-tmpl")
	// LLM Proxy referencing the provider by its handle (not a UUID).
	proxyResp, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		mkReq(proxyDPID, constants.LLMProxy, "dp-proxy", "default", map[string]interface{}{"provider": map[string]interface{}{"id": "dp-prov"}}))
	if err != nil {
		t.Fatalf("import LLM proxy: %v", err)
	}
	// MCP Proxy.
	mcpResp, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		mkReq(mcpDPID, constants.MCPProxy, "dp-mcp", "default", map[string]interface{}{"context": "/m"}))
	if err != nil {
		t.Fatalf("import MCP proxy: %v", err)
	}

	// Artifacts-table kinds: looked up by their stable handle, the CP UUID is freshly
	// generated (not the DP UUID), the response echoes it, and the DP UUID resolves to
	// nothing in the CP.
	for _, tc := range []struct{ dpID, respID, handle, kind string }{
		{restDPID, restResp.ID, "dp-rest", constants.RestApi},
		{provDPID, "", "dp-prov", constants.LLMProvider},
		{proxyDPID, proxyResp.ID, "dp-proxy", constants.LLMProxy},
		{mcpDPID, mcpResp.ID, "dp-mcp", constants.MCPProxy},
	} {
		art, err := d.artifactRepo.GetByHandle(tc.handle, importTestOrgID)
		if err != nil {
			t.Fatalf("GetByHandle(%s): %v", tc.handle, err)
		}
		if art == nil {
			t.Errorf("%s artifact not found by handle %s", tc.kind, tc.handle)
			continue
		}
		if art.Type != tc.kind {
			t.Errorf("%s: got kind=%s, want %s", tc.handle, art.Type, tc.kind)
		}
		if art.UUID == tc.dpID {
			t.Errorf("%s: CP reused the DP UUID %s; it must mint its own", tc.handle, tc.dpID)
		}
		if tc.respID != "" && tc.respID != art.UUID {
			t.Errorf("%s: response ID %q != stored CP UUID %q", tc.handle, tc.respID, art.UUID)
		}
		if byDP, _ := d.artifactRepo.GetByUUID(tc.dpID, importTestOrgID); byDP != nil {
			t.Errorf("%s: artifact resolvable by DP UUID %s; the CP owns its own UUID", tc.handle, tc.dpID)
		}
	}

	// The proxy's provider reference (a handle) must be resolved to the provider's CP UUID.
	provArt, _ := d.artifactRepo.GetByHandle("dp-prov", importTestOrgID)
	proxy, err := repository.NewLLMProxyRepo(d.db).GetByID("dp-proxy", importTestOrgID)
	if err != nil || proxy == nil {
		t.Fatalf("load proxy: (%v, %v)", proxy, err)
	}
	if provArt == nil || proxy.ProviderUUID != provArt.UUID {
		t.Errorf("proxy.ProviderUUID = %q, want resolved provider CP UUID %v", proxy.ProviderUUID, provArt)
	}

	// Template lives in its own table; its CP UUID is generated, not the DP UUID.
	tmpl, err := d.templateRepo.GetByID("dp-tmpl", importTestOrgID)
	if err != nil {
		t.Fatalf("template GetByID: %v", err)
	}
	if tmpl == nil || tmpl.UUID == tmplDPID {
		t.Errorf("template CP UUID not generated (got %v, DP UUID was %s)", tmpl, tmplDPID)
	}
}

func newTestLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// importDPTemplate imports a data-plane LLM Provider Template and returns its handle.
func importDPTemplate(t *testing.T, d *importTestDeps, id, handle string) {
	t.Helper()
	req := dto.ImportGatewayArtifactRequest{
		DPID:   id,
		Status: "deployed",
		Configuration: dto.ArtifactImportConfig{
			APIVersion: "api-platform.wso2.com/v1",
			Kind:       constants.LLMProviderTemplate,
			Metadata:   dto.ArtifactImportMetadata{Name: handle},
			Spec:       map[string]interface{}{"displayName": "DP " + handle},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if _, err := d.svc.Import(importTestOrgID, importTestGatewayID, req); err != nil {
		t.Fatalf("import DP template: %v", err)
	}
}

// importDPProvider imports a data-plane LLM Provider referencing the given template handle.
func importDPProvider(t *testing.T, d *importTestDeps, id, handle, templateHandle string) {
	t.Helper()
	req := dto.ImportGatewayArtifactRequest{
		DPID:   id,
		Status: "deployed",
		Configuration: dto.ArtifactImportConfig{
			APIVersion: "api-platform.wso2.com/v1",
			Kind:       constants.LLMProvider,
			Metadata:   dto.ArtifactImportMetadata{Name: handle},
			Spec: map[string]interface{}{
				"displayName": "DP " + handle,
				"version":     "v1.0",
				"template":    templateHandle,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if _, err := d.svc.Import(importTestOrgID, importTestGatewayID, req); err != nil {
		t.Fatalf("import DP provider: %v", err)
	}
}

// TestArtifactImport_LLMProviderLiftsSecurityAndRateLimit verifies that when a gateway
// pushes an LLM provider whose security/rate-limiting are carried as policies, the
// import lifts them into the first-class Security/RateLimiting fields and persists them.
func TestArtifactImport_LLMProviderLiftsSecurityAndRateLimit(t *testing.T) {
	d := setupImportTest(t)

	const templateHandle = "dp-rl-template"
	importDPTemplate(t, d, "dpt-rl-0000-0000-000000000001", templateHandle)

	const id = "dpp-rl-0000-0000-000000000001"
	req := dto.ImportGatewayArtifactRequest{
		DPID:   id,
		Status: "deployed",
		Configuration: dto.ArtifactImportConfig{
			APIVersion: "api-platform.wso2.com/v1",
			Kind:       constants.LLMProvider,
			Metadata:   dto.ArtifactImportMetadata{Name: "dp-rl-provider"},
			Spec: map[string]interface{}{
				"displayName": "DP RL Provider",
				"version":     "v1.0",
				"template":    templateHandle,
				"policies": []map[string]interface{}{
					{
						"name":  "api-key-auth",
						"paths": []map[string]interface{}{{"path": "/*", "methods": []string{"*"}, "params": map[string]interface{}{"key": "sk-123", "in": "header"}}},
					},
					{
						"name":  "token-based-ratelimit",
						"paths": []map[string]interface{}{{"path": "/*", "methods": []string{"*"}, "params": map[string]interface{}{"totalTokenLimits": []map[string]interface{}{{"count": 1000, "duration": "1h"}}}}},
					},
					{
						"name":  "custom-guardrail",
						"paths": []map[string]interface{}{{"path": "/*", "methods": []string{"*"}}},
					},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if _, err := d.svc.Import(importTestOrgID, importTestGatewayID, req); err != nil {
		t.Fatalf("import provider: %v", err)
	}

	stored, err := repository.NewLLMProviderRepo(d.db).GetByID("dp-rl-provider", importTestOrgID)
	if err != nil || stored == nil {
		t.Fatalf("GetByID = (%v, %v)", stored, err)
	}
	if stored.Configuration.Security == nil || stored.Configuration.Security.APIKey == nil ||
		stored.Configuration.Security.APIKey.Key != "sk-123" {
		t.Errorf("security not lifted/persisted: %+v", stored.Configuration.Security)
	}
	if stored.Configuration.RateLimiting == nil || stored.Configuration.RateLimiting.ProviderLevel == nil ||
		stored.Configuration.RateLimiting.ProviderLevel.Global == nil ||
		stored.Configuration.RateLimiting.ProviderLevel.Global.Token == nil ||
		stored.Configuration.RateLimiting.ProviderLevel.Global.Token.Count != 1000 {
		t.Errorf("rate limiting not lifted/persisted: %+v", stored.Configuration.RateLimiting)
	}
	// The system policies must not remain in the generic policy list; the custom one must.
	if len(stored.Configuration.Policies) != 1 || stored.Configuration.Policies[0].Name != "custom-guardrail" {
		t.Errorf("policies = %+v, want only [custom-guardrail]", stored.Configuration.Policies)
	}
}

// TestCPProviderFromDPTemplate verifies a control-plane LLM Provider can be created
// referencing a data-plane-originated (DP) LLM Provider Template.
func TestCPProviderFromDPTemplate(t *testing.T) {
	d := setupImportTest(t)

	const templateHandle = "dp-openai-template"
	importDPTemplate(t, d, "dpt-0000-0000-0000-000000000001", templateHandle)

	providerSvc := NewLLMProviderService(
		repository.NewLLMProviderRepo(d.db),
		d.templateRepo,
		repository.NewOrganizationRepo(d.db),
		nil, // templateSeeder not needed: the DP template already exists
		d.deployment,
		repository.NewGatewayRepo(d.db),
		nil, // gatewayEventsService unused on create
		newTestLogger(),
		&noopAuditRepo{},
	)

	created, err := providerSvc.Create(importTestOrgID, "tester", &api.LLMProvider{
		Id:            strPointer("cp-provider"),
		DisplayName:          "CP Provider",
		Version:       "v1.0",
		Template:      templateHandle, // references the DP template
		Upstream:      api.Upstream{Main: api.UpstreamDefinition{Url: strPointer("https://api.openai.com")}},
		AccessControl: api.LLMAccessControl{Mode: api.LLMAccessControlMode("deny_all")},
	})
	if err != nil {
		t.Fatalf("create CP provider from DP template: %v", err)
	}
	if created == nil || created.Id == nil || *created.Id != "cp-provider" {
		t.Fatalf("unexpected created provider: %#v", created)
	}
	// The new provider is control-plane originated, hence not read-only.
	if created.ReadOnly == nil || *created.ReadOnly {
		t.Errorf("CP provider readOnly = %v, want false", created.ReadOnly)
	}
}

// TestCPProxyFromDPProvider verifies a control-plane LLM Proxy can be created
// referencing a data-plane-originated (DP) LLM Provider.
func TestCPProxyFromDPProvider(t *testing.T) {
	d := setupImportTest(t)

	const templateHandle = "dp-openai-template"
	const providerHandle = "dp-provider"
	importDPTemplate(t, d, "dpt-0000-0000-0000-000000000002", templateHandle)
	importDPProvider(t, d, "dpp-0000-0000-0000-000000000002", providerHandle, templateHandle)

	proxySvc := NewLLMProxyService(
		repository.NewLLMProxyRepo(d.db),
		repository.NewLLMProviderRepo(d.db),
		repository.NewProjectRepo(d.db),
		d.deployment,
		repository.NewGatewayRepo(d.db),
		nil, // gatewayEventsService unused on create
		newTestLogger(),
		&noopAuditRepo{},
	)

	created, err := proxySvc.Create(importTestOrgID, "tester", &api.LLMProxy{
		Id:        strPointer("cp-proxy"),
		DisplayName:      "CP Proxy",
		Version:   "v1.0",
		ProjectId: "default", // project handle (setupImportTest inserts handle "default")
		Provider:  api.LLMProxyProvider{Id: providerHandle}, // references the DP provider
	})
	if err != nil {
		t.Fatalf("create CP proxy from DP provider: %v", err)
	}
	if created == nil || created.Id == nil || *created.Id != "cp-proxy" {
		t.Fatalf("unexpected created proxy: %#v", created)
	}
	if created.ReadOnly == nil || *created.ReadOnly {
		t.Errorf("CP proxy readOnly = %v, want false", created.ReadOnly)
	}
}

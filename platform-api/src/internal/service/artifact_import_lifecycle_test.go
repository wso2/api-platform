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
	"errors"
	"testing"
	"time"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
)

// ---------------------------------------------------------------------------
// Request builders for each gateway-pushed (DP) artifact kind. Each carries a
// data-plane UUID (dpID) that the control plane must NOT reuse, the handle
// (metadata.name) used to match an existing artifact, and a display name used
// to detect whether metadata was (re)written.
// ---------------------------------------------------------------------------

func dpTemplateReq(dpID, handle, displayName string) dto.ImportGatewayArtifactRequest {
	return dto.ImportGatewayArtifactRequest{
		DPID:   dpID,
		Status: "deployed",
		Configuration: dto.ArtifactImportConfig{
			APIVersion: "api-platform.wso2.com/v1",
			Kind:       constants.LLMProviderTemplate,
			// Org-level kind: no project annotation.
			Metadata: dto.ArtifactImportMetadata{Name: handle},
			Spec:     map[string]interface{}{"displayName": displayName},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func dpProviderReq(dpID, handle, displayName, templateHandle string) dto.ImportGatewayArtifactRequest {
	return dto.ImportGatewayArtifactRequest{
		DPID:   dpID,
		Status: "deployed",
		Configuration: dto.ArtifactImportConfig{
			APIVersion: "api-platform.wso2.com/v1",
			Kind:       constants.LLMProvider,
			// Org-level kind: no project annotation; references the template by handle.
			Metadata: dto.ArtifactImportMetadata{Name: handle},
			Spec: map[string]interface{}{
				"displayName": displayName,
				"version":     "v1.0",
				"template":    templateHandle,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func dpProxyReq(dpID, handle, displayName, providerHandle string) dto.ImportGatewayArtifactRequest {
	return dto.ImportGatewayArtifactRequest{
		DPID:   dpID,
		Status: "deployed",
		Configuration: dto.ArtifactImportConfig{
			APIVersion: "api-platform.wso2.com/v1",
			Kind:       constants.LLMProxy,
			// Project-scoped kind: project supplied via the project-id annotation.
			Metadata: dto.ArtifactImportMetadata{Name: handle, Annotations: projectAnnotations("default")},
			Spec: map[string]interface{}{
				"displayName": displayName,
				"version":     "v1.0",
				// The proxy CR references its provider by handle, encoded as an object.
				"provider": map[string]interface{}{"id": providerHandle},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func dpMCPReq(dpID, handle, displayName string) dto.ImportGatewayArtifactRequest {
	return dto.ImportGatewayArtifactRequest{
		DPID:   dpID,
		Status: "deployed",
		Configuration: dto.ArtifactImportConfig{
			APIVersion: "api-platform.wso2.com/v1",
			Kind:       constants.MCPProxy,
			Metadata:   dto.ArtifactImportMetadata{Name: handle, Annotations: projectAnnotations("default")},
			Spec: map[string]interface{}{
				"displayName": displayName,
				"version":     "v1.0",
				"context":     "/mcp",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// undeployed returns a copy of req with an "undeployed" status (the push a gateway
// sends when the artifact is deleted from it).
func undeployed(req dto.ImportGatewayArtifactRequest) dto.ImportGatewayArtifactRequest {
	req.Status = "undeployed"
	return req
}

// ---------------------------------------------------------------------------
// Assertion helpers
// ---------------------------------------------------------------------------

func mustImport(t *testing.T, d *importTestDeps, req dto.ImportGatewayArtifactRequest) *dto.ImportGatewayArtifactResponse {
	t.Helper()
	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID, req)
	if err != nil {
		t.Fatalf("Import(kind=%s, handle=%s) error: %v", req.Configuration.Kind, req.Configuration.Metadata.Name, err)
	}
	return resp
}

// artifactByHandle returns the artifacts-table row (origin/kind/uuid/name) for a handle.
func artifactByHandle(t *testing.T, d *importTestDeps, handle string) *model.Artifact {
	t.Helper()
	art, err := d.artifactRepo.GetByHandle(handle, importTestOrgID)
	if err != nil {
		t.Fatalf("GetByHandle(%s): %v", handle, err)
	}
	return art
}

// mcpServerInfoStub drives the fake MCP server-info fetcher injected into the import
// service in tests. It defaults to an empty (no-capability) success so imports never
// make a real network call.
var mcpServerInfoStub = func(*api.MCPServerInfoFetchRequest) (*api.MCPServerInfoFetchResponse, error) {
	return &api.MCPServerInfoFetchResponse{}, nil
}

// fakeMCPServerInfoFetcher is the MCPServerInfoFetcher injected into the import service in
// tests; it delegates to the swappable mcpServerInfoStub.
type fakeMCPServerInfoFetcher struct{}

func (fakeMCPServerInfoFetcher) FetchServerInfo(orgUUID string, req *api.MCPServerInfoFetchRequest) (*api.MCPServerInfoFetchResponse, error) {
	return mcpServerInfoStub(req)
}

// stubMCPServerInfo sets the capability payload (or error) the fake fetcher returns for
// the duration of a test, restoring the default on cleanup.
func stubMCPServerInfo(t *testing.T, resp *api.MCPServerInfoFetchResponse, err error) {
	t.Helper()
	prev := mcpServerInfoStub
	mcpServerInfoStub = func(*api.MCPServerInfoFetchRequest) (*api.MCPServerInfoFetchResponse, error) {
		return resp, err
	}
	t.Cleanup(func() { mcpServerInfoStub = prev })
}

// depStatus returns the current deployment ID and status for an artifact on the gateway.
func depStatus(t *testing.T, d *importTestDeps, artifactUUID string) (string, model.DeploymentStatus) {
	t.Helper()
	depID, status, _, err := d.deployment.GetStatus(artifactUUID, importTestOrgID, importTestGatewayID)
	if err != nil {
		t.Fatalf("GetStatus(%s): %v", artifactUUID, err)
	}
	return depID, status
}

// ===========================================================================
// utils.DecideMetadataWrite: the origin x deployment-time (last-in-wins) decision
// matrix (unit test). This is the core rule that governs every kind's
// metadata-write behaviour in the DP->CP import flow.
// ===========================================================================

func TestDecideMetadataWrite_OriginAndDeploymentTime(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	older := t0.Add(-time.Hour)
	newer := t0.Add(time.Hour)
	tp := func(t time.Time) *time.Time { return &t }

	cases := []struct {
		name               string
		isNew              bool
		origin             string
		currentDeployedAt  *time.Time
		incomingDeployedAt *time.Time
		want               utils.MetadataWriteMode
	}{
		{"new artifact -> full metadata", true, constants.OriginDP, nil, tp(t0), utils.WriteFullMetadata},
		{"existing CP origin -> gateway-specific only", false, constants.OriginCP, tp(t0), tp(newer), utils.WriteGatewaySpecificOnly},
		{"existing DP, newer incoming -> full metadata (this push wins)", false, constants.OriginDP, tp(t0), tp(newer), utils.WriteFullMetadata},
		{"existing DP, older incoming -> skip (stale)", false, constants.OriginDP, tp(t0), tp(older), utils.SkipWorkingCopy},
		{"existing DP, equal incoming -> skip (not strictly newer)", false, constants.OriginDP, tp(t0), tp(t0), utils.SkipWorkingCopy},
		{"existing DP, nil incoming -> skip", false, constants.OriginDP, tp(t0), nil, utils.SkipWorkingCopy},
		{"existing DP, nil current with non-nil incoming -> full metadata", false, constants.OriginDP, nil, tp(t0), utils.WriteFullMetadata},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := utils.DecideMetadataWrite(tc.isNew, tc.origin, tc.currentDeployedAt, tc.incomingDeployedAt); got != tc.want {
				t.Errorf("utils.DecideMetadataWrite(isNew=%v, origin=%q, current=%v, incoming=%v) = %v, want %v",
					tc.isNew, tc.origin, tc.currentDeployedAt, tc.incomingDeployedAt, got, tc.want)
			}
		})
	}
}

// ===========================================================================
// REST API lifecycle
// ===========================================================================

func TestImport_RESTAPI_Lifecycle_LastInWins(t *testing.T) {
	d := setupImportTest(t)

	// 1. Create: CP mints its own UUID (not the DP UUID), origin DP, deployment recorded.
	resp := mustImport(t, d, withDeployedAt(restImportRequest("dp-rest-1", "weather", "Weather v1"), baseDeployedAt))
	if resp.Origin != constants.OriginDP {
		t.Errorf("origin = %q, want DP", resp.Origin)
	}
	if resp.ID == "" || resp.ID == "dp-rest-1" {
		t.Errorf("response ID = %q, want a fresh CP UUID (not the DP UUID)", resp.ID)
	}
	cpID := resp.ID
	if art := artifactByHandle(t, d, "weather"); art == nil || art.Origin != constants.OriginDP || art.UUID != cpID || art.Type != constants.RestApi {
		t.Fatalf("artifact = %+v, want origin DP, kind RestApi, uuid %s", art, cpID)
	}
	if depID, st := depStatus(t, d, cpID); depID == "" || st != model.DeploymentStatusDeployed {
		t.Errorf("after create: (%q,%q), want non-empty DEPLOYED", depID, st)
	}

	// 2. Re-push (new DP UUID, same handle) with a newer DeployedAt -> metadata updated, CP UUID reused.
	resp2 := mustImport(t, d, withDeployedAt(restImportRequest("dp-rest-2", "weather", "Weather v2"), newerDeployedAt))
	if resp2.ID != cpID {
		t.Errorf("re-push ID = %q, want reuse of %q (matched by handle)", resp2.ID, cpID)
	}
	if name := artifactByHandle(t, d, "weather").Name; name != "Weather v2" {
		t.Errorf("name = %q, want updated to 'Weather v2' (newer DeployedAt wins)", name)
	}

	// 3. Undeploy (gateway delete): stays, marked undeployed, no new deployment.
	depBefore, _ := depStatus(t, d, cpID)
	mustImport(t, d, undeployed(restImportRequest("dp-rest-3", "weather", "Weather v2")))
	depAfter, st := depStatus(t, d, cpID)
	if st != model.DeploymentStatusUndeployed {
		t.Errorf("after undeploy status = %q, want UNDEPLOYED", st)
	}
	if depAfter != depBefore {
		t.Errorf("undeploy must not create a new deployment (before=%q after=%q)", depBefore, depAfter)
	}
	if artifactByHandle(t, d, "weather") == nil {
		t.Error("artifact removed on undeploy; it must remain")
	}

	// 4. Re-deploy with a newer DeployedAt: a NEW deployment is created and metadata updated.
	mustImport(t, d, withDeployedAt(restImportRequest("dp-rest-4", "weather", "Weather v3"), evenNewerDeployedAt))
	depRedeploy, st := depStatus(t, d, cpID)
	if st != model.DeploymentStatusDeployed {
		t.Errorf("after redeploy status = %q, want DEPLOYED", st)
	}
	if depRedeploy == "" || depRedeploy == depAfter {
		t.Errorf("redeploy must create a new deployment (undeploy=%q redeploy=%q)", depAfter, depRedeploy)
	}
	if name := artifactByHandle(t, d, "weather").Name; name != "Weather v3" {
		t.Errorf("name = %q, want 'Weather v3' after redeploy", name)
	}
}

func TestImport_RESTAPI_StalePushPreservesMetadata(t *testing.T) {
	d := setupImportTest(t)

	mustImport(t, d, withDeployedAt(restImportRequest("dp-rest-1", "weather", "First Name"), baseDeployedAt))
	// Stale re-push (older DeployedAt) must not overwrite the working-copy metadata.
	mustImport(t, d, withDeployedAt(restImportRequest("dp-rest-2", "weather", "Second Name"), olderDeployedAt))

	if name := artifactByHandle(t, d, "weather").Name; name != "First Name" {
		t.Errorf("name = %q, want unchanged 'First Name' (stale older-DeployedAt push)", name)
	}
}

// ===========================================================================
// LLM Provider Template lifecycle (organization-level, no deployment)
// ===========================================================================

func TestImport_LLMProviderTemplate_Lifecycle(t *testing.T) {
	d := setupImportTest(t)

	// Create: org-level, origin DP, NO deployment row, CP mints its own UUID.
	resp := mustImport(t, d, withDeployedAt(dpTemplateReq("dp-tmpl-1", "openai-tmpl", "OpenAI v1"), baseDeployedAt))
	if resp.Origin != constants.OriginDP {
		t.Errorf("origin = %q, want DP", resp.Origin)
	}
	if resp.ID == "" || resp.ID == "dp-tmpl-1" {
		t.Errorf("response ID = %q, want a fresh CP UUID", resp.ID)
	}
	cpID := resp.ID

	tmpl, err := d.templateRepo.GetByID("openai-tmpl", importTestOrgID)
	if err != nil || tmpl == nil {
		t.Fatalf("GetByID: (%v, %v)", tmpl, err)
	}
	if tmpl.Origin != constants.OriginDP || tmpl.UUID != cpID || tmpl.Name != "OpenAI v1" {
		t.Errorf("template = %+v, want origin DP, uuid %s, name 'OpenAI v1'", tmpl, cpID)
	}
	if depID, _ := depStatus(t, d, cpID); depID != "" {
		t.Errorf("template must not have a deployment row, got %q", depID)
	}

	// Re-push with a newer DeployedAt -> metadata updated, same CP UUID.
	resp2 := mustImport(t, d, withDeployedAt(dpTemplateReq("dp-tmpl-2", "openai-tmpl", "OpenAI v2"), newerDeployedAt))
	if resp2.ID != cpID {
		t.Errorf("re-push ID = %q, want reuse of %q", resp2.ID, cpID)
	}
	tmpl2, _ := d.templateRepo.GetByID("openai-tmpl", importTestOrgID)
	if tmpl2.Name != "OpenAI v2" {
		t.Errorf("template name = %q, want updated 'OpenAI v2'", tmpl2.Name)
	}
}

func TestImport_LLMProviderTemplate_StalePushPreservesMetadata(t *testing.T) {
	d := setupImportTest(t)

	mustImport(t, d, withDeployedAt(dpTemplateReq("dp-tmpl-1", "openai-tmpl", "First"), baseDeployedAt))
	mustImport(t, d, withDeployedAt(dpTemplateReq("dp-tmpl-2", "openai-tmpl", "Second"), olderDeployedAt))

	tmpl, _ := d.templateRepo.GetByID("openai-tmpl", importTestOrgID)
	if tmpl == nil || tmpl.Name != "First" {
		t.Errorf("template name = %v, want unchanged 'First' (stale older-DeployedAt push)", tmpl)
	}
}

func TestImport_LLMProviderTemplate_CPOriginProtected(t *testing.T) {
	d := setupImportTest(t)

	// Pre-create a CP-origin template (created in the control plane).
	if err := d.templateRepo.Create(&model.LLMProviderTemplate{
		OrganizationUUID: importTestOrgID,
		ID:               "shared-tmpl",
		Name:             "Original CP Name",
		Origin:           constants.OriginCP,
	}); err != nil {
		t.Fatalf("seed CP template: %v", err)
	}

	// A gateway pushes a same-handle template; metadata must NOT be overwritten.
	mustImport(t, d, dpTemplateReq("dp-tmpl-x", "shared-tmpl", "Hacked Name"))

	tmpl, _ := d.templateRepo.GetByID("shared-tmpl", importTestOrgID)
	if tmpl == nil {
		t.Fatal("template missing after import")
	}
	if tmpl.Name != "Original CP Name" {
		t.Errorf("name = %q; CP-origin template metadata must not change", tmpl.Name)
	}
	if tmpl.Origin != constants.OriginCP {
		t.Errorf("origin = %q, want CP", tmpl.Origin)
	}
}

// ===========================================================================
// LLM Provider lifecycle (organization-level, deployment-backed,
// references its template by handle)
// ===========================================================================

func TestImport_LLMProvider_Lifecycle_LastInWins(t *testing.T) {
	d := setupImportTest(t)
	mustImport(t, d, dpTemplateReq("dp-t", "prov-tmpl", "T")) // prerequisite template

	// Create.
	resp := mustImport(t, d, withDeployedAt(dpProviderReq("dp-prov-1", "openai", "OpenAI v1", "prov-tmpl"), baseDeployedAt))
	if resp.Origin != constants.OriginDP || resp.ID == "" || resp.ID == "dp-prov-1" {
		t.Fatalf("create resp = %+v, want DP origin + fresh CP UUID", resp)
	}
	cpID := resp.ID
	if art := artifactByHandle(t, d, "openai"); art.Origin != constants.OriginDP || art.Type != constants.LLMProvider || art.UUID != cpID {
		t.Fatalf("artifact = %+v, want origin DP, kind LlmProvider, uuid %s", art, cpID)
	}
	// The provider's template reference (a handle) must resolve to the template's CP UUID.
	tmplArt, _ := d.templateRepo.GetByID("prov-tmpl", importTestOrgID)
	prov, err := repository.NewLLMProviderRepo(d.db).GetByID("openai", importTestOrgID)
	if err != nil || prov == nil {
		t.Fatalf("load provider: (%v, %v)", prov, err)
	}
	if tmplArt == nil || prov.TemplateUUID != tmplArt.UUID {
		t.Errorf("provider.TemplateUUID = %q, want resolved template CP UUID %v", prov.TemplateUUID, tmplArt)
	}
	if depID, st := depStatus(t, d, cpID); depID == "" || st != model.DeploymentStatusDeployed {
		t.Errorf("after create: (%q,%q), want DEPLOYED", depID, st)
	}

	// Newer-DeployedAt update wins.
	resp2 := mustImport(t, d, withDeployedAt(dpProviderReq("dp-prov-2", "openai", "OpenAI v2", "prov-tmpl"), newerDeployedAt))
	if resp2.ID != cpID {
		t.Errorf("re-push ID = %q, want %q", resp2.ID, cpID)
	}
	if name := artifactByHandle(t, d, "openai").Name; name != "OpenAI v2" {
		t.Errorf("name = %q, want 'OpenAI v2'", name)
	}

	// Undeploy then redeploy.
	depBefore, _ := depStatus(t, d, cpID)
	mustImport(t, d, undeployed(dpProviderReq("dp-prov-3", "openai", "x", "prov-tmpl")))
	if dep, st := depStatus(t, d, cpID); st != model.DeploymentStatusUndeployed || dep != depBefore {
		t.Errorf("after undeploy: (%q,%q), want UNDEPLOYED with no new deployment (%q)", dep, st, depBefore)
	}
	mustImport(t, d, withDeployedAt(dpProviderReq("dp-prov-4", "openai", "OpenAI v3", "prov-tmpl"), evenNewerDeployedAt))
	if dep, st := depStatus(t, d, cpID); st != model.DeploymentStatusDeployed || dep == depBefore {
		t.Errorf("after redeploy: (%q,%q), want a new DEPLOYED deployment", dep, st)
	}
}

// TestImport_LLMProvider_MapsFlatUpstream verifies the gateway's flat upstream
// ({url, ref, auth}) is mapped into the control plane's main/sandbox UpstreamConfig
// during import, rather than being silently dropped by the generic spec decode.
func TestImport_LLMProvider_MapsFlatUpstream(t *testing.T) {
	d := setupImportTest(t)
	mustImport(t, d, dpTemplateReq("dp-t", "prov-tmpl", "T"))

	req := dpProviderReq("dp-prov-1", "openai", "OpenAI v1", "prov-tmpl")
	req.Configuration.Spec["upstream"] = map[string]interface{}{
		"url": "https://httpbin.org/anything/v1",
		"auth": map[string]interface{}{
			"type":   "api-key",
			"header": "Authorization",
			"value":  "api_key_abc123",
		},
	}
	req.Configuration.Spec["accessControl"] = map[string]interface{}{
		"mode": "deny_all",
		"exceptions": []map[string]interface{}{
			{"path": "/chat/completions", "methods": []string{"POST"}},
		},
	}
	mustImport(t, d, req)

	prov, err := repository.NewLLMProviderRepo(d.db).GetByID("openai", importTestOrgID)
	if err != nil || prov == nil {
		t.Fatalf("load provider: (%v, %v)", prov, err)
	}
	up := prov.Configuration.Upstream
	if up == nil || up.Main == nil {
		t.Fatalf("Configuration.Upstream.Main = nil, want gateway upstream mapped to main endpoint (got %+v)", up)
	}
	if up.Main.URL != "https://httpbin.org/anything/v1" {
		t.Errorf("Main.URL = %q, want the gateway upstream URL", up.Main.URL)
	}
	if up.Main.Auth == nil || up.Main.Auth.Type != "api-key" || up.Main.Auth.Header != "Authorization" || up.Main.Auth.Value != "api_key_abc123" {
		t.Errorf("Main.Auth = %+v, want the gateway upstream auth mapped through", up.Main.Auth)
	}
	ac := prov.Configuration.AccessControl
	if ac == nil || ac.Mode != "deny_all" || len(ac.Exceptions) != 1 || ac.Exceptions[0].Path != "/chat/completions" {
		t.Errorf("AccessControl = %+v, want deny_all with one /chat/completions exception", ac)
	}
}

func TestImport_LLMProvider_StalePushPreservesMetadata(t *testing.T) {
	d := setupImportTest(t)
	mustImport(t, d, dpTemplateReq("dp-t", "prov-tmpl", "T"))

	mustImport(t, d, withDeployedAt(dpProviderReq("dp-prov-1", "openai", "First", "prov-tmpl"), baseDeployedAt))
	mustImport(t, d, withDeployedAt(dpProviderReq("dp-prov-2", "openai", "Second", "prov-tmpl"), olderDeployedAt))

	if name := artifactByHandle(t, d, "openai").Name; name != "First" {
		t.Errorf("name = %q, want unchanged 'First' (stale older-DeployedAt push)", name)
	}
}

func TestImport_LLMProvider_MissingTemplate(t *testing.T) {
	d := setupImportTest(t)
	_, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		dpProviderReq("dp-prov-1", "openai", "OpenAI", "does-not-exist"))
	if !errors.Is(err, constants.ErrInvalidInput) {
		t.Fatalf("Import() error = %v, want ErrInvalidInput for a missing template reference", err)
	}
}

// ===========================================================================
// LLM Proxy lifecycle (project-scoped, deployment-backed,
// references its provider by handle)
// ===========================================================================

func TestImport_LLMProxy_Lifecycle_LastInWins(t *testing.T) {
	d := setupImportTest(t)
	// Prerequisites: a template and a provider (referenced by handle).
	mustImport(t, d, dpTemplateReq("dp-t", "prx-tmpl", "T"))
	mustImport(t, d, dpProviderReq("dp-p", "prx-prov", "Prov", "prx-tmpl"))

	resp := mustImport(t, d, withDeployedAt(dpProxyReq("dp-proxy-1", "chat-proxy", "Chat v1", "prx-prov"), baseDeployedAt))
	if resp.Origin != constants.OriginDP || resp.ID == "" || resp.ID == "dp-proxy-1" {
		t.Fatalf("create resp = %+v, want DP origin + fresh CP UUID", resp)
	}
	cpID := resp.ID
	if art := artifactByHandle(t, d, "chat-proxy"); art.Origin != constants.OriginDP || art.Type != constants.LLMProxy || art.UUID != cpID {
		t.Fatalf("artifact = %+v, want origin DP, kind LlmProxy, uuid %s", art, cpID)
	}
	// The proxy's provider reference (handle) must resolve to the provider's CP UUID.
	provArt, _ := d.artifactRepo.GetByHandle("prx-prov", importTestOrgID)
	proxy, err := repository.NewLLMProxyRepo(d.db).GetByID("chat-proxy", importTestOrgID)
	if err != nil || proxy == nil {
		t.Fatalf("load proxy: (%v, %v)", proxy, err)
	}
	if provArt == nil || proxy.ProviderUUID != provArt.UUID {
		t.Errorf("proxy.ProviderUUID = %q, want resolved provider CP UUID %v", proxy.ProviderUUID, provArt)
	}
	if depID, st := depStatus(t, d, cpID); depID == "" || st != model.DeploymentStatusDeployed {
		t.Errorf("after create: (%q,%q), want DEPLOYED", depID, st)
	}

	// Newer-DeployedAt update wins.
	resp2 := mustImport(t, d, withDeployedAt(dpProxyReq("dp-proxy-2", "chat-proxy", "Chat v2", "prx-prov"), newerDeployedAt))
	if resp2.ID != cpID {
		t.Errorf("re-push ID = %q, want %q", resp2.ID, cpID)
	}
	if name := artifactByHandle(t, d, "chat-proxy").Name; name != "Chat v2" {
		t.Errorf("name = %q, want 'Chat v2'", name)
	}

	// Undeploy then redeploy.
	depBefore, _ := depStatus(t, d, cpID)
	mustImport(t, d, undeployed(dpProxyReq("dp-proxy-3", "chat-proxy", "x", "prx-prov")))
	if dep, st := depStatus(t, d, cpID); st != model.DeploymentStatusUndeployed || dep != depBefore {
		t.Errorf("after undeploy: (%q,%q), want UNDEPLOYED with no new deployment", dep, st)
	}
	mustImport(t, d, withDeployedAt(dpProxyReq("dp-proxy-4", "chat-proxy", "Chat v3", "prx-prov"), evenNewerDeployedAt))
	if dep, st := depStatus(t, d, cpID); st != model.DeploymentStatusDeployed || dep == depBefore {
		t.Errorf("after redeploy: (%q,%q), want a new DEPLOYED deployment", dep, st)
	}
}

// TestImport_LLMProxy_MapsProviderAuth verifies the proxy CR's provider object
// ({id, auth}) is reverse-mapped so the provider handle lands in Provider and the
// gateway-specific upstream auth lands in UpstreamAuth (rather than being dropped).
func TestImport_LLMProxy_MapsProviderAuth(t *testing.T) {
	d := setupImportTest(t)
	mustImport(t, d, dpTemplateReq("dp-t", "prx-tmpl", "T"))
	mustImport(t, d, dpProviderReq("dp-p", "prx-prov", "Prov", "prx-tmpl"))

	req := dpProxyReq("dp-proxy-1", "chat-proxy", "Chat v1", "prx-prov")
	req.Configuration.Spec["provider"] = map[string]interface{}{
		"id": "prx-prov",
		"auth": map[string]interface{}{
			"type":   "api-key",
			"header": "Authorization",
			"value":  "proxy_key_xyz",
		},
	}
	mustImport(t, d, req)

	proxy, err := repository.NewLLMProxyRepo(d.db).GetByID("chat-proxy", importTestOrgID)
	if err != nil || proxy == nil {
		t.Fatalf("load proxy: (%v, %v)", proxy, err)
	}
	if proxy.Configuration.Provider != "prx-prov" {
		t.Errorf("Configuration.Provider = %q, want 'prx-prov'", proxy.Configuration.Provider)
	}
	auth := proxy.Configuration.UpstreamAuth
	if auth == nil {
		t.Fatalf("Configuration.UpstreamAuth = nil, want the provider auth mapped through")
	}
	if auth.Type != "api-key" || auth.Header != "Authorization" || auth.Value != "proxy_key_xyz" {
		t.Errorf("UpstreamAuth = %+v, want gateway provider auth", auth)
	}
}

func TestImport_LLMProxy_StalePushPreservesMetadata(t *testing.T) {
	d := setupImportTest(t)
	mustImport(t, d, dpTemplateReq("dp-t", "prx-tmpl", "T"))
	mustImport(t, d, dpProviderReq("dp-p", "prx-prov", "Prov", "prx-tmpl"))

	mustImport(t, d, withDeployedAt(dpProxyReq("dp-proxy-1", "chat-proxy", "First", "prx-prov"), baseDeployedAt))
	mustImport(t, d, withDeployedAt(dpProxyReq("dp-proxy-2", "chat-proxy", "Second", "prx-prov"), olderDeployedAt))

	if name := artifactByHandle(t, d, "chat-proxy").Name; name != "First" {
		t.Errorf("name = %q, want unchanged 'First' (stale older-DeployedAt push)", name)
	}
}

func TestImport_LLMProxy_MissingProvider(t *testing.T) {
	d := setupImportTest(t)
	_, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		dpProxyReq("dp-proxy-1", "chat-proxy", "Chat", "no-such-provider"))
	if !errors.Is(err, constants.ErrInvalidInput) {
		t.Fatalf("Import() error = %v, want ErrInvalidInput for a missing provider reference", err)
	}
}

// ===========================================================================
// MCP Proxy lifecycle (project-scoped, deployment-backed)
// ===========================================================================

func TestImport_MCPProxy_Lifecycle_LastInWins(t *testing.T) {
	d := setupImportTest(t)

	resp := mustImport(t, d, withDeployedAt(dpMCPReq("dp-mcp-1", "weather-mcp", "Weather MCP v1"), baseDeployedAt))
	if resp.Origin != constants.OriginDP || resp.ID == "" || resp.ID == "dp-mcp-1" {
		t.Fatalf("create resp = %+v, want DP origin + fresh CP UUID", resp)
	}
	cpID := resp.ID
	if art := artifactByHandle(t, d, "weather-mcp"); art.Origin != constants.OriginDP || art.Type != constants.MCPProxy || art.UUID != cpID {
		t.Fatalf("artifact = %+v, want origin DP, kind Mcp, uuid %s", art, cpID)
	}
	if depID, st := depStatus(t, d, cpID); depID == "" || st != model.DeploymentStatusDeployed {
		t.Errorf("after create: (%q,%q), want DEPLOYED", depID, st)
	}

	// Newer-DeployedAt update wins.
	resp2 := mustImport(t, d, withDeployedAt(dpMCPReq("dp-mcp-2", "weather-mcp", "Weather MCP v2"), newerDeployedAt))
	if resp2.ID != cpID {
		t.Errorf("re-push ID = %q, want %q", resp2.ID, cpID)
	}
	if name := artifactByHandle(t, d, "weather-mcp").Name; name != "Weather MCP v2" {
		t.Errorf("name = %q, want 'Weather MCP v2'", name)
	}

	// Undeploy then redeploy.
	depBefore, _ := depStatus(t, d, cpID)
	mustImport(t, d, undeployed(dpMCPReq("dp-mcp-3", "weather-mcp", "x")))
	if dep, st := depStatus(t, d, cpID); st != model.DeploymentStatusUndeployed || dep != depBefore {
		t.Errorf("after undeploy: (%q,%q), want UNDEPLOYED with no new deployment", dep, st)
	}
	mustImport(t, d, withDeployedAt(dpMCPReq("dp-mcp-4", "weather-mcp", "Weather MCP v3"), evenNewerDeployedAt))
	if dep, st := depStatus(t, d, cpID); st != model.DeploymentStatusDeployed || dep == depBefore {
		t.Errorf("after redeploy: (%q,%q), want a new DEPLOYED deployment", dep, st)
	}
}

// TestImport_MCPProxy_MapsFlatUpstream verifies the gateway's flat MCP upstream
// ({url, auth}) plus specVersion are reverse-mapped into the stored configuration
// (the single endpoint becomes the main endpoint), rather than being dropped.
func TestImport_MCPProxy_MapsFlatUpstream(t *testing.T) {
	d := setupImportTest(t)

	req := dpMCPReq("dp-mcp-1", "weather-mcp", "Weather MCP v1")
	req.Configuration.Spec["specVersion"] = "2025-06-18"
	req.Configuration.Spec["upstream"] = map[string]interface{}{
		"url": "https://mcp.example.com/sse",
		"auth": map[string]interface{}{
			"type":   "api-key",
			"header": "Authorization",
			"value":  "mcp_key_123",
		},
	}
	mustImport(t, d, req)

	proxy, err := repository.NewMCPProxyRepo(d.db).GetByHandle("weather-mcp", importTestOrgID)
	if err != nil || proxy == nil {
		t.Fatalf("load MCP proxy: (%v, %v)", proxy, err)
	}
	if proxy.Configuration.SpecVersion != "2025-06-18" {
		t.Errorf("SpecVersion = %q, want '2025-06-18'", proxy.Configuration.SpecVersion)
	}
	main := proxy.Configuration.Upstream.Main
	if main == nil {
		t.Fatalf("Configuration.Upstream.Main = nil, want gateway upstream mapped to main endpoint")
	}
	if main.URL != "https://mcp.example.com/sse" {
		t.Errorf("Main.URL = %q, want the gateway upstream URL", main.URL)
	}
	if main.Auth == nil || main.Auth.Type != "api-key" || main.Auth.Header != "Authorization" || main.Auth.Value != "mcp_key_123" {
		t.Errorf("Main.Auth = %+v, want the gateway upstream auth mapped through", main.Auth)
	}
}

// TestImport_MCPProxy_FetchesCapabilities verifies the importer pulls the MCP server's
// tools/prompts/resources from the upstream URL and stores them in the configuration.
func TestImport_MCPProxy_FetchesCapabilities(t *testing.T) {
	d := setupImportTest(t)
	tools := []map[string]interface{}{{"name": "get_weather"}}
	prompts := []map[string]interface{}{{"name": "forecast"}}
	resources := []map[string]interface{}{{"uri": "weather://today"}}
	stubMCPServerInfo(t, &api.MCPServerInfoFetchResponse{
		Tools:     &tools,
		Prompts:   &prompts,
		Resources: &resources,
	}, nil)

	req := dpMCPReq("dp-mcp-1", "weather-mcp", "Weather MCP")
	req.Configuration.Spec["upstream"] = map[string]interface{}{"url": "https://mcp.example.com/sse"}
	mustImport(t, d, req)

	proxy, err := repository.NewMCPProxyRepo(d.db).GetByHandle("weather-mcp", importTestOrgID)
	if err != nil || proxy == nil {
		t.Fatalf("load MCP proxy: (%v, %v)", proxy, err)
	}
	caps := proxy.Configuration.Capabilities
	if caps == nil || caps.Tools == nil || len(*caps.Tools) != 1 || (*caps.Tools)[0]["name"] != "get_weather" {
		t.Errorf("Capabilities.Tools = %+v, want the fetched tools", caps)
	}
	if caps == nil || caps.Prompts == nil || len(*caps.Prompts) != 1 {
		t.Errorf("Capabilities.Prompts = %+v, want the fetched prompts", caps)
	}
	if caps == nil || caps.Resources == nil || len(*caps.Resources) != 1 {
		t.Errorf("Capabilities.Resources = %+v, want the fetched resources", caps)
	}
}

// TestImport_MCPProxy_CapabilityFetchFailureIsBestEffort verifies a failed upstream fetch
// does not fail the import — the proxy is still created, just without capabilities.
func TestImport_MCPProxy_CapabilityFetchFailureIsBestEffort(t *testing.T) {
	d := setupImportTest(t)
	// The capability fetch fails — the import must still succeed, just without capabilities.
	stubMCPServerInfo(t, nil, errors.New("server-info fetch failed"))

	req := dpMCPReq("dp-mcp-1", "weather-mcp", "Weather MCP")
	req.Configuration.Spec["upstream"] = map[string]interface{}{"url": "https://mcp.example.com/sse"}
	mustImport(t, d, req)

	proxy, err := repository.NewMCPProxyRepo(d.db).GetByHandle("weather-mcp", importTestOrgID)
	if err != nil || proxy == nil {
		t.Fatalf("load MCP proxy: (%v, %v)", proxy, err)
	}
	if proxy.Configuration.Capabilities != nil {
		t.Errorf("Capabilities = %+v, want nil after a failed fetch", proxy.Configuration.Capabilities)
	}
}

// TestImport_GeneratesDeploymentIDWhenAbsent verifies the control plane falls back to
// generating its own deployment ID when the gateway does not supply one.
func TestImport_GeneratesDeploymentIDWhenAbsent(t *testing.T) {
	d := setupImportTest(t)

	resp := mustImport(t, d, dpMCPReq("dp-mcp-1", "weather-mcp", "Weather MCP"))

	depID, _ := depStatus(t, d, resp.ID)
	if depID == "" {
		t.Errorf("deployment ID = empty, want a control-plane-generated ID")
	}
}

func TestImport_MCPProxy_StalePushPreservesMetadata(t *testing.T) {
	d := setupImportTest(t)

	mustImport(t, d, withDeployedAt(dpMCPReq("dp-mcp-1", "weather-mcp", "First"), baseDeployedAt))
	mustImport(t, d, withDeployedAt(dpMCPReq("dp-mcp-2", "weather-mcp", "Second"), olderDeployedAt))

	if name := artifactByHandle(t, d, "weather-mcp").Name; name != "First" {
		t.Errorf("name = %q, want unchanged 'First' (stale older-DeployedAt push)", name)
	}
}

// TestImport_MCPProxy_CPOriginProtected covers the cross-origin same-handle scenario for
// a deployment-backed kind: a CP-created MCP proxy (not yet deployed) and a gateway-created
// MCP proxy share a handle. The push must preserve the CP artifact's metadata (even with a
// newer DeployedAt; the origin guard wins) and only add a new deployment entry.
func TestImport_MCPProxy_CPOriginProtected(t *testing.T) {
	d := setupImportTest(t)

	mcpRepo := repository.NewMCPProxyRepo(d.db)
	proj := importTestProjectID
	if err := mcpRepo.Create(&model.MCPProxy{
		Handle:           "shared-mcp",
		OrganizationUUID: importTestOrgID,
		ProjectUUID:      &proj,
		Name:             "Original CP Name",
		Version:          "v1.0",
		Origin:           constants.OriginCP,
		Configuration:    model.MCPProxyConfiguration{Name: "Original CP Name", Version: "v1.0"},
	}); err != nil {
		t.Fatalf("seed CP MCP proxy: %v", err)
	}
	cpArt := artifactByHandle(t, d, "shared-mcp")
	if cpArt == nil {
		t.Fatal("seeded CP MCP artifact not found")
	}
	cpID := cpArt.UUID

	// Gateway pushes a same-handle MCP proxy with a different DP UUID and a newer DeployedAt;
	// the CP-origin guard must still protect the metadata.
	resp := mustImport(t, d, withDeployedAt(dpMCPReq("dp-shared-mcp", "shared-mcp", "Hacked Name"), newerDeployedAt))
	if resp.ID != cpID {
		t.Errorf("response ID = %q, want existing CP UUID %q", resp.ID, cpID)
	}

	art := artifactByHandle(t, d, "shared-mcp")
	if art == nil {
		t.Fatal("artifact missing after import")
	}
	if art.Name != "Original CP Name" {
		t.Errorf("name = %q; CP-origin metadata must not change even for a newer-DeployedAt push", art.Name)
	}
	if art.Origin != constants.OriginCP {
		t.Errorf("origin = %q, want CP", art.Origin)
	}
	// A new deployment entry must have been added for the CP artifact.
	if depID, st := depStatus(t, d, cpID); depID == "" || st != model.DeploymentStatusDeployed {
		t.Errorf("deployment = (%q,%q), want a new DEPLOYED entry for the CP artifact", depID, st)
	}
}

// TestImport_MCPProxy_ReadOnlyInGetAndList exercises the readOnly flag end-to-end through
// the MCP proxy service: a DP-origin artifact (imported from a gateway) is read-only in
// both Get and List, while a CP-origin artifact (created via the service) is not. The GET
// readOnly mapping for all kinds is covered by TestReadOnlyReflectsOrigin; this adds the
// "listing" path end-to-end.
func TestImport_MCPProxy_ReadOnlyInGetAndList(t *testing.T) {
	d := setupImportTest(t)
	svc := NewMCPProxyService(
		repository.NewMCPProxyRepo(d.db),
		repository.NewProjectRepo(d.db),
		d.deployment,
		repository.NewGatewayRepo(d.db),
		nil, // gatewayEventsService unused on create/get/list
		newTestLogger(),
		&noopAuditRepo{},
	)

	// DP-origin MCP proxy via the gateway import flow.
	mustImport(t, d, dpMCPReq("dp-mcp-ro", "dp-mcp", "DP MCP"))

	// CP-origin MCP proxy via the service. ProjectId is the project handle
	// ("default" for the seeded project), resolved to the UUID on create.
	proj := "default"
	if _, err := svc.Create(importTestOrgID, "tester", &api.MCPProxy{
		Id:        strPointer("cp-mcp"),
		DisplayName:      "CP MCP",
		Version:   "v1.0",
		ProjectId: &proj,
		Upstream:  api.Upstream{Main: api.UpstreamDefinition{Url: strPointer("https://api.example.com")}},
	}); err != nil {
		t.Fatalf("create CP MCP proxy: %v", err)
	}

	// Get: DP read-only, CP mutable.
	dpGet, err := svc.Get(importTestOrgID, "dp-mcp")
	if err != nil {
		t.Fatalf("Get(dp-mcp): %v", err)
	}
	if dpGet.ReadOnly == nil || !*dpGet.ReadOnly {
		t.Errorf("Get(dp-mcp).ReadOnly = %v, want true", dpGet.ReadOnly)
	}
	cpGet, err := svc.Get(importTestOrgID, "cp-mcp")
	if err != nil {
		t.Fatalf("Get(cp-mcp): %v", err)
	}
	if cpGet.ReadOnly == nil || *cpGet.ReadOnly {
		t.Errorf("Get(cp-mcp).ReadOnly = %v, want false", cpGet.ReadOnly)
	}

	// List: each item carries the correct readOnly per origin.
	list, err := svc.List(importTestOrgID, 100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	seen := map[string]bool{}
	for i := range list.List {
		item := list.List[i]
		if item.Id == nil {
			continue
		}
		seen[*item.Id] = true
		switch *item.Id {
		case "dp-mcp":
			if item.ReadOnly == nil || !*item.ReadOnly {
				t.Errorf("list item dp-mcp ReadOnly = %v, want true", item.ReadOnly)
			}
		case "cp-mcp":
			if item.ReadOnly == nil || *item.ReadOnly {
				t.Errorf("list item cp-mcp ReadOnly = %v, want false", item.ReadOnly)
			}
		}
	}
	if !seen["dp-mcp"] || !seen["cp-mcp"] {
		t.Errorf("list missing expected items; saw %v", seen)
	}
}

// TestCPSideGuard_UpdateBlockedForDPOrigin verifies that the control-plane CRUD services
// reject updates to DP-originated (gateway-pushed) artifacts with ErrArtifactReadOnly,
// for every kind. The DP artifacts are seeded through the import flow. (REST is covered by
// TestArtifactImport_Enforcement_ReadOnlyAndDeletion; deploy/undeploy/restore guards are
// covered by the deployment-guard tests.)
func TestCPSideGuard_UpdateBlockedForDPOrigin(t *testing.T) {
	logger := newTestLogger()

	t.Run("LLMProviderTemplate", func(t *testing.T) {
		d := setupImportTest(t)
		svc := NewLLMProviderTemplateService(d.templateRepo, &noopAuditRepo{})
		mustImport(t, d, dpTemplateReq("dp-t", "blk-tmpl", "T"))
		if _, err := svc.Update(importTestOrgID, "blk-tmpl", "tester", &api.LLMProviderTemplate{DisplayName: "Hacked"}); !errors.Is(err, constants.ErrArtifactReadOnly) {
			t.Errorf("Template Update(DP) = %v, want ErrArtifactReadOnly", err)
		}
	})

	t.Run("LLMProvider", func(t *testing.T) {
		d := setupImportTest(t)
		svc := NewLLMProviderService(repository.NewLLMProviderRepo(d.db), d.templateRepo,
			repository.NewOrganizationRepo(d.db), nil, d.deployment, repository.NewGatewayRepo(d.db), nil, logger, &noopAuditRepo{})
		mustImport(t, d, dpTemplateReq("dp-t", "p-tmpl", "T"))
		mustImport(t, d, dpProviderReq("dp-p", "blk-prov", "P", "p-tmpl"))
		if _, err := svc.Update(importTestOrgID, "blk-prov", "tester", &api.LLMProvider{DisplayName: "Hacked"}); !errors.Is(err, constants.ErrArtifactReadOnly) {
			t.Errorf("Provider Update(DP) = %v, want ErrArtifactReadOnly", err)
		}
	})

	t.Run("LLMProxy", func(t *testing.T) {
		d := setupImportTest(t)
		svc := NewLLMProxyService(repository.NewLLMProxyRepo(d.db), repository.NewLLMProviderRepo(d.db),
			repository.NewProjectRepo(d.db), d.deployment, repository.NewGatewayRepo(d.db), nil, logger, &noopAuditRepo{})
		mustImport(t, d, dpTemplateReq("dp-t", "px-tmpl", "T"))
		mustImport(t, d, dpProviderReq("dp-p", "px-prov", "P", "px-tmpl"))
		mustImport(t, d, dpProxyReq("dp-x", "blk-proxy", "X", "px-prov"))
		if _, err := svc.Update(importTestOrgID, "blk-proxy", "tester", &api.LLMProxy{
			DisplayName: "Hacked", Version: "v2", Provider: api.LLMProxyProvider{Id: "px-prov"},
		}); !errors.Is(err, constants.ErrArtifactReadOnly) {
			t.Errorf("Proxy Update(DP) = %v, want ErrArtifactReadOnly", err)
		}
	})

	t.Run("MCPProxy", func(t *testing.T) {
		d := setupImportTest(t)
		svc := NewMCPProxyService(repository.NewMCPProxyRepo(d.db), repository.NewProjectRepo(d.db),
			d.deployment, repository.NewGatewayRepo(d.db), nil, logger, &noopAuditRepo{})
		mustImport(t, d, dpMCPReq("dp-m", "blk-mcp", "M"))
		if _, err := svc.Update(importTestOrgID, "blk-mcp", "tester", &api.MCPProxy{
			Id: strPointer("blk-mcp"), DisplayName: "Hacked", Version: "v2",
			Upstream: api.Upstream{Main: api.UpstreamDefinition{Url: strPointer("https://api.example.com")}},
		}); !errors.Is(err, constants.ErrArtifactReadOnly) {
			t.Errorf("MCP Update(DP) = %v, want ErrArtifactReadOnly", err)
		}
	})
}

// TestLLMProviderTemplate_DeleteOriginGuard verifies that the control plane refuses to
// delete a DP-originated (gateway-pushed) template (it is read-only), while a CP-created
// template can be deleted. Templates have no per-gateway deployment, so the read-only
// guard applies directly.
func TestLLMProviderTemplate_DeleteOriginGuard(t *testing.T) {
	d := setupImportTest(t)
	svc := NewLLMProviderTemplateService(d.templateRepo, &noopAuditRepo{})

	// DP-origin template (imported from a gateway) cannot be deleted from the CP.
	mustImport(t, d, dpTemplateReq("dp-t", "dp-tmpl", "DP Template"))
	if err := svc.Delete(importTestOrgID, "dp-tmpl", "tester"); !errors.Is(err, constants.ErrArtifactReadOnly) {
		t.Errorf("Delete(DP template) = %v, want ErrArtifactReadOnly", err)
	}
	// It must still exist after the rejected delete.
	if tmpl, _ := d.templateRepo.GetByID("dp-tmpl", importTestOrgID); tmpl == nil {
		t.Error("DP template was deleted despite being read-only")
	}

	// CP-origin template can be deleted.
	if err := d.templateRepo.Create(&model.LLMProviderTemplate{
		OrganizationUUID: importTestOrgID,
		ID:               "cp-tmpl",
		Name:             "CP Template",
		Origin:           constants.OriginCP,
	}); err != nil {
		t.Fatalf("seed CP template: %v", err)
	}
	if err := svc.Delete(importTestOrgID, "cp-tmpl", "tester"); err != nil {
		t.Errorf("Delete(CP template) = %v, want nil", err)
	}
	if tmpl, _ := d.templateRepo.GetByID("cp-tmpl", importTestOrgID); tmpl != nil {
		t.Error("CP template was not deleted")
	}

	// Deleting a non-existent template returns not-found (guard does not mask it).
	if err := svc.Delete(importTestOrgID, "no-such-tmpl", "tester"); !errors.Is(err, constants.ErrLLMProviderTemplateNotFound) {
		t.Errorf("Delete(missing) = %v, want ErrLLMProviderTemplateNotFound", err)
	}
}

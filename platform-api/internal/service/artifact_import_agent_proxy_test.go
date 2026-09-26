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
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"gopkg.in/yaml.v3"
)

// Section 15 — bottom-up (DP->CP) Agent import.
//
// These tests drive the real import orchestrator over a real SQLite schema: the
// gateway pushes kind Agent with spec.a2a, the control plane stores kind
// AgentProxy with protocol a2a, and the stored Agent proxy is read-only.

// recordingCardInvalidator records every display-cache invalidation the importer
// asks for.
type recordingCardInvalidator struct {
	mu    sync.Mutex
	calls []agentCardCacheKey
}

func (r *recordingCardInvalidator) InvalidateAgentCard(orgUUID, proxyUUID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, agentCardCacheKey{orgUUID: orgUUID, proxyUUID: proxyUUID})
}

// agentImportDeps extends the shared import fixture with the Agent-specific
// repositories and a service wired to a recording card-cache invalidator.
type agentImportDeps struct {
	*importTestDeps
	svc         *ArtifactImportService
	agentRepo   repository.AgentProxyRepository
	apiKeyRepo  repository.APIKeyRepository
	gatewayRepo repository.GatewayRepository
	projectRepo repository.ProjectRepository
	invalidator *recordingCardInvalidator
	logs        *bytes.Buffer
}

func setupAgentImportTest(t *testing.T) *agentImportDeps {
	t.Helper()
	base := setupImportTest(t)
	db := base.db

	reg := repository.NewArtifactTableRegistry()
	agentRepo := repository.NewAgentProxyRepo(db)
	gatewayRepo := repository.NewGatewayRepo(db)
	projectRepo := repository.NewProjectRepo(db)
	cfg := &config.Server{}
	cfg.Deployments.MaxPerAPIGateway = 10
	invalidator := &recordingCardInvalidator{}
	logs := &bytes.Buffer{}

	svc := NewArtifactImportService(base.apiRepo, repository.NewLLMProviderRepo(db), base.templateRepo,
		repository.NewLLMProxyRepo(db), repository.NewMCPProxyRepo(db), agentRepo,
		base.artifactRepo, base.deployment, gatewayRepo, projectRepo, cfg,
		slog.New(slog.NewTextHandler(logs, nil)), fakeMCPServerInfoFetcher{}, invalidator)

	return &agentImportDeps{
		importTestDeps: base,
		svc:            svc,
		agentRepo:      agentRepo,
		apiKeyRepo:     repository.NewAPIKeyRepo(db, reg),
		gatewayRepo:    gatewayRepo,
		projectRepo:    projectRepo,
		invalidator:    invalidator,
		logs:           logs,
	}
}

// minimalGatewayAgentSpec is the gateway form of the Section 1 minimal request:
// one transport, no operation configuration beyond it, no card block.
func minimalGatewayAgentSpec() map[string]interface{} {
	return map[string]interface{}{
		"displayName": "Weather Agent",
		"version":     "v1.0",
		"context":     "/weather",
		"upstream": map[string]interface{}{
			"url": "http://weather-agent:9000",
		},
		"a2a": map[string]interface{}{
			"protocolVersion": "1.0",
			"operationConfigs": map[string]interface{}{
				"transports": []interface{}{
					map[string]interface{}{"protocolBinding": "JSONRPC", "pathPrefix": "/rpc"},
				},
			},
		},
	}
}

// fullGatewayAgentSpec is the gateway form of the Section 1 fuller request: both
// transports, auth, common and per-operation policies, a managed public card with
// a custom path and card policies, and a passthrough protected card with an
// explicit rewriteUrls: false.
func fullGatewayAgentSpec() map[string]interface{} {
	return map[string]interface{}{
		"displayName": "Weather Agent",
		"version":     "v1.0",
		"context":     "/weather",
		"vhost":       "agents.gw.com",
		"upstream": map[string]interface{}{
			"url": "http://weather-agent:9000",
			"auth": map[string]interface{}{
				"type":   "api-key",
				"header": "X-API-Key",
				"value":  `{{ secret "weather-upstream" }}`,
			},
		},
		"resilience": map[string]interface{}{"idleTimeout": "5s"},
		"a2a": map[string]interface{}{
			"protocolVersion": "1.0",
			"operationConfigs": map[string]interface{}{
				"transports": []interface{}{
					map[string]interface{}{"protocolBinding": "JSONRPC", "pathPrefix": "/rpc"},
					map[string]interface{}{"protocolBinding": "HTTP+JSON", "pathPrefix": "/rest"},
				},
				"policies": []interface{}{
					map[string]interface{}{
						"name":    "jwt-auth",
						"version": "v1",
						"params": map[string]interface{}{
							"issuer":         "https://idp.example.com",
							"requiredScopes": []interface{}{"a2a.invoke"},
						},
					},
				},
				"operations": []interface{}{
					map[string]interface{}{
						"name": "SendMessage",
						"policies": []interface{}{
							map[string]interface{}{"name": "advanced-ratelimit", "version": "v1",
								"executionCondition": "request.method == 'POST'"},
						},
						"resilience": map[string]interface{}{"timeout": "30s"},
					},
				},
			},
			"agentCard": map[string]interface{}{
				"public": map[string]interface{}{
					"mode":     "managed",
					"path":     "/cards/weather.json",
					"policies": []interface{}{map[string]interface{}{"name": "cors", "version": "v1"}},
					"content":  managedCardContent(),
				},
				"protected": map[string]interface{}{
					"mode":        "passthrough",
					"rewriteUrls": false,
				},
			},
		},
	}
}

// managedCardContent is a complete Agent Card carrying an extension field, so a
// round trip can prove free-form content is preserved rather than normalized.
func managedCardContent() map[string]interface{} {
	return map[string]interface{}{
		"name":        "Weather Agent",
		"description": "Provides forecasts and severe-weather alerts",
		"version":     "1.0.0",
		"supportedInterfaces": []interface{}{
			map[string]interface{}{"protocolBinding": "JSONRPC", "url": "https://gw.example.com/weather/rpc", "protocolVersion": "1.0"},
		},
		"capabilities":       map[string]interface{}{"streaming": true, "extendedAgentCard": true},
		"defaultInputModes":  []interface{}{"text/plain"},
		"defaultOutputModes": []interface{}{"text/plain"},
		"skills": []interface{}{
			map[string]interface{}{"id": "forecast", "name": "Forecast", "description": "Multi-day forecast", "tags": []interface{}{"weather"}},
		},
		"x-vendor-extension": map[string]interface{}{"tier": "gold"},
	}
}

func agentImportRequest(dpid, handle string, spec map[string]interface{}) dto.ImportGatewayArtifactRequest {
	return dto.ImportGatewayArtifactRequest{
		DPID:   dpid,
		Status: utils.ImportStatusDeployed,
		Configuration: dto.ArtifactImportConfig{
			APIVersion: constants.GatewayApiVersion,
			Kind:       constants.GatewayKindAgent,
			Metadata:   dto.ArtifactImportMetadata{Name: handle, Annotations: projectAnnotations("default")},
			Spec:       spec,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// jsonNormalize round-trips a value through JSON so documents built from Go maps,
// YAML decoding and JSON decoding compare equal regardless of their concrete
// number and map types.
func jsonNormalize(t *testing.T, v interface{}) interface{} {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func deepCopySpec(t *testing.T, spec map[string]interface{}) map[string]interface{} {
	t.Helper()
	return jsonNormalize(t, spec).(map[string]interface{})
}

func TestAgentImport_CreatesReadOnlyAgentProxy(t *testing.T) {
	d := setupAgentImportTest(t)

	const dpid = "a1111111-1111-1111-1111-111111111111"
	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID, agentImportRequest(dpid, "weather-agent", minimalGatewayAgentSpec()))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if resp.ID == "" || resp.ID == dpid {
		t.Fatalf("response ID = %q, want a control-plane UUID distinct from the DP UUID", resp.ID)
	}
	if resp.Origin != constants.OriginDP {
		t.Errorf("response Origin = %q, want %q", resp.Origin, constants.OriginDP)
	}
	if resp.DeployedVersion != "v1.0" {
		t.Errorf("response DeployedVersion = %q, want v1.0", resp.DeployedVersion)
	}

	// Stored under the control-plane kind — never the gateway's Agent.
	art, err := d.artifactRepo.GetByUUID(resp.ID, importTestOrgID)
	if err != nil || art == nil {
		t.Fatalf("GetByUUID = (%v, %v)", art, err)
	}
	if art.Type != constants.AgentProxy {
		t.Errorf("artifacts.type = %q, want %q", art.Type, constants.AgentProxy)
	}
	if art.Origin != constants.OriginDP {
		t.Errorf("artifact origin = %q, want %q", art.Origin, constants.OriginDP)
	}

	proxy, err := d.agentRepo.GetByHandle("weather-agent", importTestOrgID)
	if err != nil || proxy == nil {
		t.Fatalf("GetByHandle = (%v, %v)", proxy, err)
	}
	if proxy.UUID != resp.ID {
		t.Errorf("agent proxy UUID = %q, want the response ID %q", proxy.UUID, resp.ID)
	}
	if proxy.Protocol != model.AgentProxyProtocolA2A {
		t.Errorf("protocol = %q, want %q", proxy.Protocol, model.AgentProxyProtocolA2A)
	}
	if !proxy.IsReadOnly() {
		t.Error("imported Agent proxy is not read-only")
	}
	if proxy.ProjectUUID != importTestProjectID {
		t.Errorf("project = %q, want %q", proxy.ProjectUUID, importTestProjectID)
	}
	if proxy.Name != "Weather Agent" || proxy.Version != "v1.0" {
		t.Errorf("name/version = %q/%q, want Weather Agent/v1.0", proxy.Name, proxy.Version)
	}

	a2a := proxy.Configuration.A2A
	if a2a == nil {
		t.Fatal("stored configuration has no a2a block")
	}
	if a2a.ProtocolVersion != "1.0" {
		t.Errorf("a2a.protocolVersion = %q, want 1.0", a2a.ProtocolVersion)
	}
	if len(a2a.Transports) != 1 || a2a.Transports[0].ProtocolBinding != "JSONRPC" ||
		a2a.Transports[0].PathPrefix == nil || *a2a.Transports[0].PathPrefix != "/rpc" {
		t.Errorf("a2a.transports = %+v, want the one JSONRPC transport at /rpc moved out of operationConfigs", a2a.Transports)
	}
	// The gateway's operationConfigs only carried the transports, so the control
	// plane's optional block stays omitted — as does the card block.
	if a2a.OperationConfigs != nil {
		t.Errorf("a2a.operationConfigs = %+v, want omitted", a2a.OperationConfigs)
	}
	if a2a.AgentCard != nil {
		t.Errorf("a2a.agentCard = %+v, want omitted", a2a.AgentCard)
	}

	// The deployment status and the gateway association exist, so the Agent proxy
	// is listed against this gateway and key update/revoke has a target.
	depID, status, _, err := d.deployment.GetStatus(resp.ID, importTestOrgID, importTestGatewayID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if depID == "" || status != model.DeploymentStatusDeployed {
		t.Errorf("deployment status = (%q, %q), want a DEPLOYED record", depID, status)
	}
	assocs, err := d.apiRepo.GetAPIAssociations(resp.ID, constants.AssociationTypeGateway, importTestOrgID)
	if err != nil {
		t.Fatalf("GetAPIAssociations: %v", err)
	}
	if len(assocs) != 1 || assocs[0].GatewayID != importTestGatewayID {
		t.Errorf("gateway associations = %+v, want exactly the pushing gateway", assocs)
	}
}

// The persisted row follows the Section 2 boundary: protocol in its column only,
// no column-backed metadata and no gateway spec wrapper in the document, and a
// data version resolved through the gateway→control-plane kind mapping.
func TestAgentImport_PersistsColumnAndDocumentBoundary(t *testing.T) {
	d := setupAgentImportTest(t)

	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		agentImportRequest("a2222222-2222-2222-2222-222222222222", "weather-agent", fullGatewayAgentSpec()))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	var protocol, dataVersion, origin string
	var configuration []byte
	if err := d.db.QueryRow(`SELECT protocol, data_version, origin, configuration FROM agent_proxies WHERE uuid = ?`, resp.ID).
		Scan(&protocol, &dataVersion, &origin, &configuration); err != nil {
		t.Fatalf("select agent_proxies row: %v", err)
	}
	if protocol != string(model.AgentProxyProtocolA2A) {
		t.Errorf("agent_proxies.protocol = %q, want a2a", protocol)
	}
	if origin != constants.OriginDP {
		t.Errorf("agent_proxies.origin = %q, want %q", origin, constants.OriginDP)
	}
	wantDataVersion := string(gatewaytranslator.ComputeDataVersion(constants.AgentProxy, constants.GatewayApiVersion))
	if dataVersion != wantDataVersion {
		t.Errorf("agent_proxies.data_version = %q, want %q (the AgentProxy entry)", dataVersion, wantDataVersion)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(configuration, &doc); err != nil {
		t.Fatalf("configuration is not JSON: %v", err)
	}
	for _, key := range []string{"protocol", "spec", "specVersion", "displayName", "name", "version", "kind", "apiVersion", "metadata"} {
		if _, ok := doc[key]; ok {
			t.Errorf("configuration carries %q; it belongs in a column or nowhere", key)
		}
	}
	a2a, ok := doc["a2a"].(map[string]interface{})
	if !ok {
		t.Fatalf("configuration has no a2a object: %v", doc)
	}
	if _, ok := a2a["protocol"]; ok {
		t.Error("configuration.a2a duplicates the protocol discriminator")
	}
	if _, ok := a2a["transports"]; !ok {
		t.Error("configuration.a2a.transports is missing")
	}
	if opCfgs, ok := a2a["operationConfigs"].(map[string]interface{}); ok {
		if _, ok := opCfgs["transports"]; ok {
			t.Error("transports were left in the gateway's operationConfigs position")
		}
	}

	// The secret placeholder is recorded as a reference of the imported artifact.
	var refs int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM artifact_secret_refs WHERE artifact_uuid = ? AND secret_handle = ?`,
		resp.ID, "weather-upstream").Scan(&refs); err != nil {
		t.Fatalf("count secret refs: %v", err)
	}
	if refs != 1 {
		t.Errorf("artifact_secret_refs rows for weather-upstream = %d, want 1", refs)
	}
}

// The import is the inverse of the Section 7 builder: building a gateway artifact
// from the imported Agent proxy reproduces the spec the gateway pushed, for both
// the minimal and the fuller shape — including the moved transports, omitted card
// blocks, the explicit rewriteUrls: false and free-form card extensions.
func TestAgentImport_InverseOfDeploymentBuilder(t *testing.T) {
	cases := map[string]func() map[string]interface{}{
		"minimal": minimalGatewayAgentSpec,
		"full":    fullGatewayAgentSpec,
	}
	for name, specFn := range cases {
		t.Run(name, func(t *testing.T) {
			d := setupAgentImportTest(t)
			pushed := specFn()
			if _, err := d.svc.Import(importTestOrgID, importTestGatewayID,
				agentImportRequest("a3333333-3333-3333-3333-333333333333", "weather-agent", deepCopySpec(t, pushed))); err != nil {
				t.Fatalf("Import() error = %v", err)
			}
			proxy, err := d.agentRepo.GetByHandle("weather-agent", importTestOrgID)
			if err != nil || proxy == nil {
				t.Fatalf("GetByHandle = (%v, %v)", proxy, err)
			}

			built, err := (&utils.AgentProxyUtils{}).BuildAgentProxyDeploymentYAML(proxy)
			if err != nil {
				t.Fatalf("BuildAgentProxyDeploymentYAML: %v", err)
			}
			if built.Kind != constants.GatewayKindAgent {
				t.Errorf("built kind = %q, want %q", built.Kind, constants.GatewayKindAgent)
			}
			specYAML, err := yaml.Marshal(built.Spec)
			if err != nil {
				t.Fatalf("marshal built spec: %v", err)
			}
			var rebuilt map[string]interface{}
			if err := yaml.Unmarshal(specYAML, &rebuilt); err != nil {
				t.Fatalf("unmarshal built spec: %v", err)
			}

			got := jsonNormalize(t, rebuilt)
			want := jsonNormalize(t, pushed)
			if !reflect.DeepEqual(got, want) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				wantJSON, _ := json.MarshalIndent(want, "", "  ")
				t.Errorf("rebuilt gateway spec differs from the pushed spec\n got: %s\nwant: %s", gotJSON, wantJSON)
			}
		})
	}
}

// The public read of an imported Agent proxy is the ordinary redacted A2A shape:
// kind AgentProxy, protocol a2a at the root, the typed a2a block, readOnly: true,
// and no upstream credential. Every public mutation is then refused with 403.
func TestAgentImport_PublicResponseIsReadOnlyAndMutationsAreRefused(t *testing.T) {
	d := setupAgentImportTest(t)

	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		withDeployedAt(agentImportRequest("a4444444-4444-4444-4444-444444444444", "weather-agent", fullGatewayAgentSpec()), baseDeployedAt))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	cfg := &config.Server{}
	cfg.Deployments.MaxPerAPIGateway = 10
	proxySvc := NewAgentProxyService(d.agentRepo, d.projectRepo, d.deployment, d.gatewayRepo, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)), repository.NewAuditRepo(d.db), cfg,
		NewIdentityService(repository.NewUserIdentityMappingRepo(d.db)))

	got, err := proxySvc.Get(importTestOrgID, "weather-agent")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Kind == nil || *got.Kind != api.A2AAgentProxyKindAgentProxy {
		t.Errorf("kind = %v, want AgentProxy", got.Kind)
	}
	if got.Id == nil || *got.Id != "weather-agent" {
		t.Errorf("id = %v, want the public handle weather-agent (never the UUID)", got.Id)
	}
	if got.ProjectId != "default" {
		t.Errorf("projectId = %q, want the project handle", got.ProjectId)
	}
	if got.Protocol != api.A2AAgentProxyProtocol(model.AgentProxyProtocolA2A) {
		t.Errorf("protocol = %q, want a2a", got.Protocol)
	}
	if got.ReadOnly == nil || !*got.ReadOnly {
		t.Errorf("readOnly = %v, want true", got.ReadOnly)
	}
	if got.A2a.ProtocolVersion != "1.0" || len(got.A2a.Transports) != 2 {
		t.Errorf("a2a = %+v, want protocolVersion 1.0 and both transports", got.A2a)
	}
	if got.A2a.AgentCard == nil || got.A2a.AgentCard.Public == nil || got.A2a.AgentCard.Public.Content == nil {
		t.Fatalf("a2a.agentCard.public.content missing from the response: %+v", got.A2a.AgentCard)
	}
	if _, ok := (*got.A2a.AgentCard.Public.Content)["x-vendor-extension"]; !ok {
		t.Error("the card's extension field was not preserved")
	}
	if prot := got.A2a.AgentCard.Protected; prot == nil || prot.RewriteUrls == nil || *prot.RewriteUrls {
		t.Errorf("protected card = %+v, want passthrough with an explicit rewriteUrls: false", prot)
	}
	if auth := got.Upstream.Main.Auth; auth == nil || auth.Value != nil {
		t.Errorf("upstream.main.auth = %+v, want the auth block with its credential redacted", auth)
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "weather-upstream") {
		t.Error("the response echoes the upstream secret handle")
	}
	if strings.Contains(string(encoded), resp.ID) {
		t.Error("the response exposes the internal artifact UUID")
	}

	// PUT with the body the caller just read back is refused as read-only, not as
	// a validation failure.
	if _, err := proxySvc.Update(importTestOrgID, "weather-agent", "someone", got); !apperror.ArtifactReadOnly.Is(err) {
		t.Errorf("Update() error = %v, want ARTIFACT_READ_ONLY", err)
	}
	// Delete is refused while the gateway still runs it.
	if err := proxySvc.Delete(importTestOrgID, "weather-agent", "someone"); !apperror.ArtifactDeployed.Is(err) {
		t.Errorf("Delete() error = %v, want ARTIFACT_DEPLOYED while deployed", err)
	}

	// No public redeploy of a gateway-origin Agent proxy.
	deploySvc := NewAgentDeploymentService(d.agentRepo, d.deployment, d.gatewayRepo, d.artifactRepo, d.apiKeyRepo,
		nil, nil, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err = deploySvc.DeployByHandle("weather-agent",
		&api.DeployRequest{Name: "redeploy", Base: "current", GatewayId: "gw1"}, importTestOrgID, "someone")
	if !apperror.ArtifactReadOnly.Is(err) {
		t.Errorf("DeployByHandle() error = %v, want ARTIFACT_READ_ONLY", err)
	}

	// The refusals left the working copy exactly as imported.
	after, err := d.agentRepo.GetByHandle("weather-agent", importTestOrgID)
	if err != nil || after == nil {
		t.Fatalf("GetByHandle = (%v, %v)", after, err)
	}
	if after.Configuration.Upstream.Main == nil || after.Configuration.Upstream.Main.Auth == nil ||
		after.Configuration.Upstream.Main.Auth.Value != `{{ secret "weather-upstream" }}` {
		t.Errorf("stored upstream credential changed after refused mutations: %+v", after.Configuration.Upstream.Main)
	}
}

// Keys minted for an imported Agent proxy are associated with the control-plane
// UUID, and the gateway's Agent backfill (kind AgentProxy) returns them for the
// pushing gateway — the gateway then remaps that UUID to its local artifact via
// cp_artifact_id (CP-O5).
func TestAgentImport_APIKeysAssociateWithControlPlaneUUIDAndBackfill(t *testing.T) {
	d := setupAgentImportTest(t)

	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		agentImportRequest("a5555555-5555-5555-5555-555555555555", "weather-agent", minimalGatewayAgentSpec()))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	key := &model.APIKey{
		UUID:           "k5555555-5555-5555-5555-555555555555",
		ArtifactUUID:   resp.ID,
		Name:           "consumer-key",
		DisplayName:    "Consumer Key",
		MaskedAPIKey:   "****abcd",
		APIKeyHashes:   `{"sha256":"deadbeef"}`,
		Status:         "active",
		CreatedBy:      "user-1",
		AllowedTargets: "ALL",
	}
	if err := d.apiKeyRepo.Create(key); err != nil {
		t.Fatalf("create API key: %v", err)
	}

	keys, err := d.apiKeyRepo.ListByGatewayAndKind(importTestGatewayID, importTestOrgID, constants.AgentProxy, "")
	if err != nil {
		t.Fatalf("ListByGatewayAndKind: %v", err)
	}
	if len(keys) != 1 || keys[0].ArtifactUUID != resp.ID {
		t.Fatalf("backfill keys = %+v, want the one key against the control-plane UUID %q", keys, resp.ID)
	}
	// The gateway's kind name selects nothing: the stored kind is AgentProxy.
	if gwKeys, err := d.apiKeyRepo.ListByGatewayAndKind(importTestGatewayID, importTestOrgID, constants.GatewayKindAgent, ""); err != nil || len(gwKeys) != 0 {
		t.Errorf("ListByGatewayAndKind(Agent) = (%v, %v), want none", gwKeys, err)
	}
	// A gateway the Agent proxy was never pushed from gets nothing.
	if other, err := d.apiKeyRepo.ListByGatewayAndKind("gw-unrelated", importTestOrgID, constants.AgentProxy, ""); err != nil || len(other) != 0 {
		t.Errorf("ListByGatewayAndKind(other gateway) = (%v, %v), want none", other, err)
	}
}

// An import is refused only when there is nothing the control plane can store —
// and then nothing is stored at all: no artifact, no Agent proxy row, no deployment.
func TestAgentImport_RefusesOnlyWhatCannotBeStored(t *testing.T) {
	cases := []struct {
		name   string
		handle string
		mutate func(req *dto.ImportGatewayArtifactRequest)
		reason string
	}{
		{name: "empty spec", reason: "no spec", mutate: func(r *dto.ImportGatewayArtifactRequest) {
			r.Configuration.Spec = nil
		}},
		{name: "spec that is not an Agent", reason: "decode spec", mutate: func(r *dto.ImportGatewayArtifactRequest) {
			r.Configuration.Spec["a2a"] = "not-an-object"
		}},
		{name: "missing a2a block", reason: "spec.a2a is required", mutate: func(r *dto.ImportGatewayArtifactRequest) {
			delete(r.Configuration.Spec, "a2a")
		}},
		{name: "unregistered protocolVersion", reason: `protocolVersion "2.0" is not supported`, mutate: func(r *dto.ImportGatewayArtifactRequest) {
			importSpecA2A(r)["protocolVersion"] = "2.0"
		}},
		{name: "handle wider than the column", handle: strings.Repeat("a", 41), reason: "at most 40"},
		{name: "reserved handle", handle: "fetch-agent-card", reason: `"fetch-agent-card" is reserved`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := setupAgentImportTest(t)
			handle := tc.handle
			if handle == "" {
				handle = "weather-agent"
			}
			req := agentImportRequest("a6666666-6666-6666-6666-666666666666", handle, deepCopySpec(t, minimalGatewayAgentSpec()))
			if tc.mutate != nil {
				tc.mutate(&req)
			}

			_, err := d.svc.Import(importTestOrgID, importTestGatewayID, req)
			if !apperror.ValidationFailed.Is(err) {
				t.Fatalf("Import() error = %v, want VALIDATION_FAILED", err)
			}
			if !strings.Contains(err.Error(), handle) || !strings.Contains(err.Error(), tc.reason) {
				t.Errorf("error %q does not name the Agent and the reason %q", err.Error(), tc.reason)
			}

			if art, err := d.artifactRepo.GetByHandle(handle, importTestOrgID); err != nil || art != nil {
				t.Errorf("artifact after refused import = (%v, %v), want none", art, err)
			}
			var rows int
			if err := d.db.QueryRow(`SELECT COUNT(*) FROM agent_proxies`).Scan(&rows); err != nil {
				t.Fatalf("count agent_proxies: %v", err)
			}
			if rows != 0 {
				t.Errorf("agent_proxies rows after refused import = %d, want 0", rows)
			}
			if err := d.db.QueryRow(`SELECT COUNT(*) FROM deployments`).Scan(&rows); err != nil {
				t.Fatalf("count deployments: %v", err)
			}
			if rows != 0 {
				t.Errorf("deployments after refused import = %d, want 0", rows)
			}
		})
	}
}

// Everything else imports, as it does for every other kind: what maps is stored,
// known-but-unrepresentable fields are dropped and named in a warning, unknown
// fields are ignored, and the authoring contract is not applied to a gateway-owned
// Agent. The deployment record keeps the pushed configuration verbatim, so the
// dropped settings are still on record.
func TestAgentImport_StoresWhatMapsAndDropsTheRest(t *testing.T) {
	cases := []struct {
		name    string
		handle  string
		mutate  func(req *dto.ImportGatewayArtifactRequest)
		dropped []string
		check   func(t *testing.T, p *model.AgentProxy)
	}{
		{name: "unknown sibling protocol block is ignored", mutate: func(r *dto.ImportGatewayArtifactRequest) {
			r.Configuration.Spec["mcp"] = map[string]interface{}{"specVersion": "2025-06-18"}
		}},
		{name: "unknown field inside a2a is ignored", mutate: func(r *dto.ImportGatewayArtifactRequest) {
			importSpecA2A(r)["streaming"] = true
		}},
		{name: "top-level policies are ignored", mutate: func(r *dto.ImportGatewayArtifactRequest) {
			r.Configuration.Spec["policies"] = []interface{}{map[string]interface{}{"name": "cors", "version": "v1"}}
		}},
		{name: "unknown apiVersion is stored", mutate: func(r *dto.ImportGatewayArtifactRequest) {
			r.Configuration.APIVersion = "gateway.api-platform.wso2.com/v1alpha1"
		}},
		{name: "ref is kept and upstreamDefinitions dropped",
			dropped: []string{"spec.upstreamDefinitions"},
			mutate: func(r *dto.ImportGatewayArtifactRequest) {
				r.Configuration.Spec["upstream"] = map[string]interface{}{"ref": "primary"}
				r.Configuration.Spec["upstreamDefinitions"] = []interface{}{
					map[string]interface{}{"name": "primary", "upstreams": []interface{}{map[string]interface{}{"url": "http://a:9000"}}},
				}
			},
			check: func(t *testing.T, p *model.AgentProxy) {
				if m := p.Configuration.Upstream.Main; m == nil || m.Ref != "primary" || m.URL != "" {
					t.Errorf("upstream main = %+v, want ref primary and no url", m)
				}
			}},
		{name: "manual host rewrite is dropped", dropped: []string{"spec.upstream.hostRewrite"},
			mutate: func(r *dto.ImportGatewayArtifactRequest) { importSpecUpstream(r)["hostRewrite"] = "manual" }},
		{name: "oauth2 auth keeps its type and drops its policy params",
			dropped: []string{"spec.upstream.auth.policyParams"},
			mutate: func(r *dto.ImportGatewayArtifactRequest) {
				importSpecUpstream(r)["auth"] = map[string]interface{}{"type": "oauth2",
					"policyParams": map[string]interface{}{"tokenEndpoint": "https://idp/token", "clientSecret": "s3cr3t"}}
			},
			check: func(t *testing.T, p *model.AgentProxy) {
				if a := p.Configuration.Upstream.Main.Auth; a == nil || a.Type != "oauth2" || a.Value != "" {
					t.Errorf("upstream auth = %+v, want type oauth2 with nothing else", a)
				}
			}},
		{name: "policy-backed api-key auth drops every policy field",
			dropped: []string{"spec.upstream.auth.policyName", "spec.upstream.auth.policyParams", "spec.upstream.auth.policyVersion"},
			mutate: func(r *dto.ImportGatewayArtifactRequest) {
				importSpecUpstream(r)["auth"] = map[string]interface{}{"type": "api-key", "policyName": "set-headers",
					"policyVersion": "v1", "policyParams": map[string]interface{}{"request": map[string]interface{}{}}}
			}},
		{name: "api-key auth without a credential is stored", mutate: func(r *dto.ImportGatewayArtifactRequest) {
			importSpecUpstream(r)["auth"] = map[string]interface{}{"type": "api-key", "header": "X-API-Key"}
		}},
		{name: "enabled card signing is dropped and the card kept",
			dropped: []string{"spec.a2a.agentCard.public.signing"},
			mutate: func(r *dto.ImportGatewayArtifactRequest) {
				importSpecA2A(r)["agentCard"] = map[string]interface{}{"public": map[string]interface{}{
					"mode": "managed", "content": managedCardContent(), "signing": map[string]interface{}{"enabled": true}}}
			},
			check: func(t *testing.T, p *model.AgentProxy) {
				c := p.Configuration.A2A.AgentCard
				if c == nil || c.Public == nil || c.Public.Content == nil {
					t.Errorf("public card = %+v, want the managed card kept", c)
				}
			}},
		{name: "unknown operation name is stored", mutate: func(r *dto.ImportGatewayArtifactRequest) {
			importSpecOpCfgs(r)["operations"] = []interface{}{map[string]interface{}{"name": "Teleport"}}
		}, check: func(t *testing.T, p *model.AgentProxy) {
			ops := p.Configuration.A2A.OperationConfigs
			if ops == nil || len(ops.Operations) != 1 || ops.Operations[0].Name != "Teleport" {
				t.Errorf("operationConfigs = %+v, want the operation stored as sent", ops)
			}
		}},
		{name: "managed card missing required keys is stored as sent, not completed",
			mutate: func(r *dto.ImportGatewayArtifactRequest) {
				importSpecA2A(r)["agentCard"] = map[string]interface{}{"public": map[string]interface{}{
					"mode": "managed", "content": map[string]interface{}{"name": "Weather Agent"}}}
			},
			check: func(t *testing.T, p *model.AgentProxy) {
				content := p.Configuration.A2A.AgentCard.Public.Content
				if len(content) != 1 || content["name"] != "Weather Agent" {
					t.Errorf("card content = %v, want exactly what was pushed", content)
				}
			}},
		{name: "missing operationConfigs stores no transports", mutate: func(r *dto.ImportGatewayArtifactRequest) {
			delete(importSpecA2A(r), "operationConfigs")
		}, check: func(t *testing.T, p *model.AgentProxy) {
			if len(p.Configuration.A2A.Transports) != 0 || p.Configuration.A2A.OperationConfigs != nil {
				t.Errorf("a2a = %+v, want no transports and no operationConfigs", p.Configuration.A2A)
			}
		}},
		{name: "protected card without a mode is stored", mutate: func(r *dto.ImportGatewayArtifactRequest) {
			importSpecA2A(r)["agentCard"] = map[string]interface{}{"protected": map[string]interface{}{"rewriteUrls": true}}
		}},
		{name: "handle outside the authoring pattern is stored", handle: "Weather.Agent"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := setupAgentImportTest(t)
			handle := tc.handle
			if handle == "" {
				handle = "weather-agent"
			}
			req := agentImportRequest("a6666666-6666-6666-6666-666666666666", handle, deepCopySpec(t, minimalGatewayAgentSpec()))
			if tc.mutate != nil {
				tc.mutate(&req)
			}

			resp, err := d.svc.Import(importTestOrgID, importTestGatewayID, req)
			if err != nil {
				t.Fatalf("Import() error = %v, want the Agent imported", err)
			}
			proxy, err := d.agentRepo.GetByUUID(resp.ID, importTestOrgID)
			if err != nil || proxy == nil {
				t.Fatalf("GetByUUID = (%v, %v)", proxy, err)
			}
			if proxy.Protocol != model.AgentProxyProtocolA2A || !proxy.IsReadOnly() {
				t.Errorf("protocol/origin = %q/%q, want a2a/%s", proxy.Protocol, proxy.Origin, constants.OriginDP)
			}
			if tc.check != nil {
				tc.check(t, proxy)
			}

			logged := d.logs.String()
			for _, field := range tc.dropped {
				if !strings.Contains(logged, field) {
					t.Errorf("warning log does not name dropped field %s:\n%s", field, logged)
				}
			}
			if len(tc.dropped) == 0 && strings.Contains(logged, "cannot represent") {
				t.Errorf("a drop was reported for a push that lost nothing known:\n%s", logged)
			}
			if strings.Contains(logged, "s3cr3t") {
				t.Error("the warning log carries a dropped credential value")
			}

			// The deployment record still holds the configuration as pushed.
			var content []byte
			if err := d.db.QueryRow(`SELECT content FROM deployments WHERE artifact_uuid = ?`, resp.ID).Scan(&content); err != nil {
				t.Fatalf("select deployment content: %v", err)
			}
			var pushed map[string]interface{}
			if err := yaml.Unmarshal(content, &pushed); err != nil {
				t.Fatalf("deployment content is not YAML: %v", err)
			}
			if got, want := jsonNormalize(t, pushed["spec"]), jsonNormalize(t, req.Configuration.Spec); !reflect.DeepEqual(got, want) {
				t.Errorf("deployment content spec = %v, want the pushed spec %v", got, want)
			}
		})
	}
}

// Values indistinguishable from their absence are accepted and not stored; gateway
// lifecycle state is accepted and left to the push status.
func TestAgentImport_AcceptsValuesEquivalentToAbsence(t *testing.T) {
	d := setupAgentImportTest(t)

	spec := minimalGatewayAgentSpec()
	spec["deploymentState"] = "deployed"
	spec["upstream"].(map[string]interface{})["hostRewrite"] = "auto"
	spec["upstream"].(map[string]interface{})["auth"] = map[string]interface{}{"type": "none"}
	spec["a2a"].(map[string]interface{})["agentCard"] = map[string]interface{}{
		"public": map[string]interface{}{
			"mode":    "managed",
			"content": managedCardContent(),
			"signing": map[string]interface{}{"enabled": false},
		},
	}

	if _, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		agentImportRequest("a7777777-7777-7777-7777-777777777777", "weather-agent", spec)); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	proxy, err := d.agentRepo.GetByHandle("weather-agent", importTestOrgID)
	if err != nil || proxy == nil {
		t.Fatalf("GetByHandle = (%v, %v)", proxy, err)
	}
	if auth := proxy.Configuration.Upstream.Main.Auth; auth == nil || auth.Type != "none" {
		t.Errorf("upstream auth = %+v, want the explicit none", auth)
	}
	card := proxy.Configuration.A2A.AgentCard
	if card == nil || card.Public == nil || card.Public.Mode == nil || *card.Public.Mode != model.AgentCardModeManaged {
		t.Fatalf("public card = %+v, want managed", card)
	}
	if card.Protected != nil {
		t.Errorf("protected card = %+v, want omitted (absence is never materialized)", card.Protected)
	}
	if strings.Contains(d.logs.String(), "cannot represent") {
		t.Errorf("values equivalent to absence were reported as dropped:\n%s", d.logs.String())
	}
}

// Last-in-wins, as for every other kind: a newer push rewrites a gateway-origin
// working copy (and drops its cached display fetch), a stale one does not, and
// the project never moves.
func TestAgentImport_LastInWinsWorkingCopy(t *testing.T) {
	d := setupAgentImportTest(t)
	const otherProjectID = "project-import-002"
	if _, err := d.db.Exec(`INSERT INTO projects (uuid, handle, display_name, organization_uuid, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		otherProjectID, "other-project", "Other", importTestOrgID, ""); err != nil {
		t.Fatalf("seed second project: %v", err)
	}

	const dpid = "a8888888-8888-8888-8888-888888888888"
	first, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		withDeployedAt(agentImportRequest(dpid, "weather-agent", minimalGatewayAgentSpec()), baseDeployedAt))
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if len(d.invalidator.calls) != 0 {
		t.Errorf("a create invalidated the card cache: %+v", d.invalidator.calls)
	}

	// Newer push: new display name, version, upstream and a managed card — and a
	// different project annotation, which is resolved but not applied.
	newer := fullGatewayAgentSpec()
	newer["displayName"] = "Weather Agent Two"
	newer["version"] = "v2.0"
	newerReq := withDeployedAt(agentImportRequest(dpid, "weather-agent", newer), newerDeployedAt)
	newerReq.Configuration.Metadata.Annotations = projectAnnotations("other-project")
	second, err := d.svc.Import(importTestOrgID, importTestGatewayID, newerReq)
	if err != nil {
		t.Fatalf("newer import: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("newer push ID = %q, want the existing control-plane UUID %q", second.ID, first.ID)
	}
	proxy, err := d.agentRepo.GetByHandle("weather-agent", importTestOrgID)
	if err != nil || proxy == nil {
		t.Fatalf("GetByHandle = (%v, %v)", proxy, err)
	}
	if proxy.Name != "Weather Agent Two" || proxy.Version != "v2.0" {
		t.Errorf("name/version = %q/%q, want the newer push's", proxy.Name, proxy.Version)
	}
	if proxy.Configuration.A2A.AgentCard == nil || len(proxy.Configuration.A2A.Transports) != 2 {
		t.Errorf("configuration was not replaced by the newer push: %+v", proxy.Configuration.A2A)
	}
	if proxy.ProjectUUID != importTestProjectID {
		t.Errorf("project = %q, want it to stay in %q", proxy.ProjectUUID, importTestProjectID)
	}
	if proxy.Protocol != model.AgentProxyProtocolA2A || !proxy.IsReadOnly() {
		t.Errorf("protocol/origin changed: %q/%q", proxy.Protocol, proxy.Origin)
	}
	wantCall := agentCardCacheKey{orgUUID: importTestOrgID, proxyUUID: first.ID}
	if len(d.invalidator.calls) != 1 || d.invalidator.calls[0] != wantCall {
		t.Errorf("card cache invalidations = %+v, want exactly %+v", d.invalidator.calls, wantCall)
	}

	// Stale push: older than the watermark, so the working copy is untouched and the
	// cache is left alone.
	stale := minimalGatewayAgentSpec()
	stale["displayName"] = "Stale Name"
	if _, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		withDeployedAt(agentImportRequest(dpid, "weather-agent", stale), olderDeployedAt)); err != nil {
		t.Fatalf("stale import: %v", err)
	}
	proxy, _ = d.agentRepo.GetByHandle("weather-agent", importTestOrgID)
	if proxy == nil || proxy.Name != "Weather Agent Two" {
		t.Errorf("stale push rewrote the working copy: %+v", proxy)
	}
	if len(d.invalidator.calls) != 1 {
		t.Errorf("a stale push invalidated the card cache: %+v", d.invalidator.calls)
	}
}

// A gateway Agent that lands on the handle of a control-plane-authored Agent proxy
// records a deployment against it and never touches its metadata.
func TestAgentImport_ControlPlaneOwnedAgentProxyIsNotOverwritten(t *testing.T) {
	d := setupAgentImportTest(t)

	path := "/rpc"
	cpProxy := &model.AgentProxy{
		Handle:           "weather-agent",
		OrganizationUUID: importTestOrgID,
		ProjectUUID:      importTestProjectID,
		Name:             "CP Authored",
		Protocol:         model.AgentProxyProtocolA2A,
		Version:          "v1.0",
		Origin:           constants.OriginCP,
		Configuration: model.AgentProxyConfiguration{
			Upstream: model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "http://cp-agent:9000"}},
			A2A: &model.A2AProtocolConfig{
				ProtocolVersion: "1.0",
				Transports:      []model.A2ATransport{{ProtocolBinding: "JSONRPC", PathPrefix: &path}},
			},
		},
	}
	if err := d.agentRepo.Create(cpProxy); err != nil {
		t.Fatalf("seed CP agent proxy: %v", err)
	}

	pushed := fullGatewayAgentSpec()
	pushed["displayName"] = "Gateway Name"
	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		withDeployedAt(agentImportRequest("a9999999-9999-9999-9999-999999999999", "weather-agent", pushed), newerDeployedAt))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if resp.ID != cpProxy.UUID {
		t.Errorf("response ID = %q, want the control-plane Agent proxy %q", resp.ID, cpProxy.UUID)
	}
	after, err := d.agentRepo.GetByUUID(cpProxy.UUID, importTestOrgID)
	if err != nil || after == nil {
		t.Fatalf("GetByUUID = (%v, %v)", after, err)
	}
	if after.Name != "CP Authored" || after.Origin != constants.OriginCP ||
		after.Configuration.Upstream.Main.URL != "http://cp-agent:9000" || after.Configuration.A2A.AgentCard != nil {
		t.Errorf("control-plane-owned Agent proxy was modified by the import: %+v", after)
	}
	if len(d.invalidator.calls) != 0 {
		t.Errorf("an import that wrote nothing invalidated the card cache: %+v", d.invalidator.calls)
	}
	depID, status, _, err := d.deployment.GetStatus(cpProxy.UUID, importTestOrgID, importTestGatewayID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if depID == "" || status != model.DeploymentStatusDeployed {
		t.Errorf("deployment status = (%q, %q), want a DEPLOYED record for the gateway", depID, status)
	}
}

// The handle-reuse guard compares the stored kind: a gateway Agent on a REST API's
// handle is a conflict, and a gateway Agent on an AgentProxy's handle is not.
func TestAgentImport_HandleOwnedByAnotherKindIsAConflict(t *testing.T) {
	d := setupAgentImportTest(t)
	if _, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		restImportRequest("b1111111-1111-1111-1111-111111111111", "shared-handle", "Shared")); err != nil {
		t.Fatalf("seed REST API import: %v", err)
	}

	_, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		agentImportRequest("b2222222-2222-2222-2222-222222222222", "shared-handle", minimalGatewayAgentSpec()))
	if !apperror.ArtifactExists.Is(err) {
		t.Fatalf("Import() error = %v, want ARTIFACT_EXISTS", err)
	}
	if proxy, err := d.agentRepo.GetByHandle("shared-handle", importTestOrgID); err != nil || proxy != nil {
		t.Errorf("agent proxy after conflict = (%v, %v), want none", proxy, err)
	}
}

// An undeployed push marks the imported Agent proxy undeployed on that gateway and
// keeps it; an undeployed Agent push naming another kind's handle is ignored.
func TestAgentImport_UndeployPush(t *testing.T) {
	d := setupAgentImportTest(t)

	const dpid = "b3333333-3333-3333-3333-333333333333"
	created, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		agentImportRequest(dpid, "weather-agent", minimalGatewayAgentSpec()))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	undeploy := agentImportRequest(dpid, "weather-agent", minimalGatewayAgentSpec())
	undeploy.Status = utils.ImportStatusUndeployed
	resp, err := d.svc.Import(importTestOrgID, importTestGatewayID, undeploy)
	if err != nil {
		t.Fatalf("undeploy Import() error = %v", err)
	}
	if resp.ID != created.ID {
		t.Errorf("undeploy response ID = %q, want %q", resp.ID, created.ID)
	}
	_, status, _, err := d.deployment.GetStatus(created.ID, importTestOrgID, importTestGatewayID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status != model.DeploymentStatusUndeployed {
		t.Errorf("status after undeploy push = %q, want UNDEPLOYED", status)
	}
	if proxy, _ := d.agentRepo.GetByUUID(created.ID, importTestOrgID); proxy == nil {
		t.Error("undeploy push deleted the Agent proxy; it must be kept")
	}

	// Kind guard: an Agent undeploy that names a REST API's handle changes nothing.
	rest, err := d.svc.Import(importTestOrgID, importTestGatewayID,
		restImportRequest("b4444444-4444-4444-4444-444444444444", "rest-only", "Rest Only"))
	if err != nil {
		t.Fatalf("seed REST API import: %v", err)
	}
	wrongKind := agentImportRequest("b5555555-5555-5555-5555-555555555555", "rest-only", minimalGatewayAgentSpec())
	wrongKind.Status = utils.ImportStatusUndeployed
	ignored, err := d.svc.Import(importTestOrgID, importTestGatewayID, wrongKind)
	if err != nil {
		t.Fatalf("wrong-kind undeploy Import() error = %v", err)
	}
	if ignored.ID != "" {
		t.Errorf("wrong-kind undeploy echoed artifact %q; it must not resolve another kind", ignored.ID)
	}
	_, restStatus, _, err := d.deployment.GetStatus(rest.ID, importTestOrgID, importTestGatewayID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if restStatus != model.DeploymentStatusDeployed {
		t.Errorf("REST API status after an Agent undeploy push = %q, want DEPLOYED", restStatus)
	}
}

// In a bulk push an Agent imports alongside other kinds, and a refused Agent fails
// only its own entry.
func TestAgentImport_BulkPushIsContinueOnError(t *testing.T) {
	d := setupAgentImportTest(t)

	bad := agentImportRequest("c2222222-2222-2222-2222-222222222222", "bad-agent", minimalGatewayAgentSpec())
	delete(bad.Configuration.Spec, "a2a")
	reqs := []dto.ImportGatewayArtifactRequest{
		agentImportRequest("c1111111-1111-1111-1111-111111111111", "good-agent", minimalGatewayAgentSpec()),
		bad,
		restImportRequest("c3333333-3333-3333-3333-333333333333", "a-rest-api", "A REST API"),
	}
	resp := d.svc.ImportArtifacts(importTestOrgID, importTestGatewayID, reqs)
	if resp.Total != 3 || resp.Success != 2 || resp.Failed != 1 {
		t.Fatalf("bulk result total/success/failed = %d/%d/%d, want 3/2/1", resp.Total, resp.Success, resp.Failed)
	}
	if r := resp.Artifacts["c1111111-1111-1111-1111-111111111111"]; r.Error != "" || r.ID == "" {
		t.Errorf("good Agent result = %+v, want success with a control-plane ID", r)
	}
	if r := resp.Artifacts["c2222222-2222-2222-2222-222222222222"]; r.Error == "" {
		t.Errorf("bad Agent result = %+v, want a failure reason", r)
	}
	if r := resp.Artifacts["c3333333-3333-3333-3333-333333333333"]; r.Error != "" {
		t.Errorf("REST API result = %+v, want success", r)
	}
}

func importSpecA2A(r *dto.ImportGatewayArtifactRequest) map[string]interface{} {
	return r.Configuration.Spec["a2a"].(map[string]interface{})
}

func importSpecOpCfgs(r *dto.ImportGatewayArtifactRequest) map[string]interface{} {
	return importSpecA2A(r)["operationConfigs"].(map[string]interface{})
}

func importSpecUpstream(r *dto.ImportGatewayArtifactRequest) map[string]interface{} {
	return r.Configuration.Spec["upstream"].(map[string]interface{})
}

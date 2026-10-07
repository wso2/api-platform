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
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/gwversion"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// AgentDeploymentService over the real repositories and a real SQLite schema.
// The import fixture seeds the org, the project and gateway "gw1"; these tests
// add a second gateway in the same org, a gateway in another org, and the Agent
// proxies under test.

const (
	depTOtherOrg       = "org-import-002"
	depTOtherGateway   = "gw-import-other-org"
	depTSecondGateway  = "gw-import-002"
	depTSecondGWHandle = "gw2"
	depTProxyHandle    = "weather-agent"
)

type depTEnv struct {
	*agentImportDeps
	svc   *AgentDeploymentService
	hub   *capturingEventHub
	cfg   *config.Server
	proxy string // UUID of the CP-origin Agent proxy depTProxyHandle
}

func depTSetup(t *testing.T) *depTEnv {
	t.Helper()
	d := setupAgentImportTest(t)

	if _, err := d.db.Exec(`INSERT INTO gateways (uuid, organization_uuid, handle, display_name, description, properties, created_at, updated_at)
		VALUES (?, ?, ?, ?, '', '{}', datetime('now'), datetime('now'))`,
		depTSecondGateway, importTestOrgID, depTSecondGWHandle, "Gateway 2"); err != nil {
		t.Fatalf("seed second gateway: %v", err)
	}
	// Both gateways are new enough to have the Agent kind.
	if _, err := d.db.Exec(`UPDATE gateways SET version = ? WHERE organization_uuid = ?`,
		gwversion.MinAgentKindGatewayVersion, importTestOrgID); err != nil {
		t.Fatalf("set gateway versions: %v", err)
	}
	if _, err := d.db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, ?, 'Other Org', 'default', 'idp-other', datetime('now'), datetime('now'))`,
		depTOtherOrg, "h-"+depTOtherOrg); err != nil {
		t.Fatalf("seed other org: %v", err)
	}
	// Same handle as the caller's gateway, but in another organization.
	if _, err := d.db.Exec(`INSERT INTO gateways (uuid, organization_uuid, handle, display_name, description, properties, created_at, updated_at)
		VALUES (?, ?, 'gw-foreign', 'Foreign', '', '{}', datetime('now'), datetime('now'))`,
		depTOtherGateway, depTOtherOrg); err != nil {
		t.Fatalf("seed other-org gateway: %v", err)
	}

	cfg := &config.Server{}
	cfg.Deployments.MaxPerAPIGateway = 5
	hub := &capturingEventHub{}
	events := NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger())
	svc := NewAgentDeploymentService(d.agentRepo, d.deployment, d.gatewayRepo, d.artifactRepo, d.apiKeyRepo,
		events, NewArtifactDefinitions(NewAgentProxyDefinition(d.agentRepo, &utils.AgentProxyUtils{})),
		cfg, newTestLogger())

	env := &depTEnv{agentImportDeps: d, svc: svc, hub: hub, cfg: cfg}
	env.proxy = env.createProxy(t, depTProxyHandle, constants.OriginCP)
	return env
}

// createProxy stores an A2A Agent proxy with the given origin and returns its UUID.
func (e *depTEnv) createProxy(t *testing.T, handle, origin string) string {
	t.Helper()
	p := &model.AgentProxy{
		Handle:           handle,
		OrganizationUUID: importTestOrgID,
		ProjectUUID:      importTestProjectID,
		Name:             "Weather Agent",
		Version:          "v1.0",
		Protocol:         model.AgentProxyProtocolA2A,
		Origin:           origin,
		CreatedBy:        "creator",
		Configuration: model.AgentProxyConfiguration{
			Context:  strPtr("/weather"),
			Upstream: model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "http://weather-agent:9000"}},
			A2A: &model.A2AProtocolConfig{
				ProtocolVersion: "1.0",
				OperationConfigs: model.A2AOperationConfigs{
					Transports: []model.A2ATransport{{ProtocolBinding: "JSONRPC"}},
				},
			},
		},
	}
	if err := e.agentRepo.Create(p); err != nil {
		t.Fatalf("create Agent proxy %s: %v", handle, err)
	}
	return p.UUID
}

func depTDeployReq(name, gateway string) *api.DeployRequest {
	return &api.DeployRequest{Name: name, Base: "current", GatewayId: gateway}
}

// deploy creates a deployment of depTProxyHandle on the gateway and returns its id.
func (e *depTEnv) deploy(t *testing.T, gateway string) string {
	t.Helper()
	resp, err := e.svc.DeployByHandle(depTProxyHandle, depTDeployReq("dep", gateway), importTestOrgID, "deployer")
	if err != nil {
		t.Fatalf("DeployByHandle(%s): %v", gateway, err)
	}
	return resp.DeploymentId.String()
}

// ack simulates the gateway acknowledging a deployment into a terminal state.
func (e *depTEnv) ack(t *testing.T, deploymentID string, status model.DeploymentStatus, reason string) {
	t.Helper()
	if _, err := e.db.Exec(`UPDATE deployment_status SET status = ?, status_reason = ? WHERE deployment_uuid = ?`,
		string(status), reason, deploymentID); err != nil {
		t.Fatalf("simulate ack: %v", err)
	}
}

// eventTypes returns the type of every event broadcast so far, in order.
func (e *depTEnv) eventTypes(t *testing.T) []string {
	t.Helper()
	types := make([]string, 0, len(e.hub.published))
	for _, ev := range e.hub.published {
		var envelope dto.GatewayEventDTO
		if err := json.Unmarshal([]byte(ev.EventData), &envelope); err != nil {
			t.Fatalf("decode event envelope: %v", err)
		}
		types = append(types, envelope.Type)
	}
	return types
}

func depTWantErr(t *testing.T, op string, err error, want interface{ Is(error) bool }) {
	t.Helper()
	if err == nil || !want.Is(err) {
		t.Fatalf("%s error = %v, want %v", op, err, want)
	}
}

func TestAgentDeployment_DeployValidatesTheRequestBeforeAnyLookup(t *testing.T) {
	env := depTSetup(t)
	buildID := "some-build"
	cases := []struct {
		name string
		req  *api.DeployRequest
	}{
		{name: "nil body", req: nil},
		{name: "blank name", req: &api.DeployRequest{Name: "  ", Base: "current", GatewayId: "gw1"}},
		{name: "missing base", req: &api.DeployRequest{Name: "dep", GatewayId: "gw1"}},
		{name: "buildId with base current", req: &api.DeployRequest{Name: "dep", Base: "current", BuildId: &buildID, GatewayId: "gw1"}},
		{name: "blank gatewayId", req: &api.DeployRequest{Name: "dep", Base: "current", GatewayId: " "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The handle does not exist: a validation failure proves the request is
			// checked before the Agent proxy is resolved.
			_, err := env.svc.DeployByHandle("no-such-agent", tc.req, importTestOrgID, "deployer")
			depTWantErr(t, "DeployByHandle", err, apperror.AgentProxyDeploymentValidationFailed)
		})
	}
	if n := len(env.hub.published); n != 0 {
		t.Fatalf("%d events broadcast for rejected deploys, want none", n)
	}
}

func TestAgentDeployment_DeployUnknownOrForeignAgentProxyIsNotFound(t *testing.T) {
	env := depTSetup(t)
	for _, tc := range []struct{ name, handle, org string }{
		{name: "unknown handle", handle: "no-such-agent", org: importTestOrgID},
		{name: "blank handle", handle: "  ", org: importTestOrgID},
		{name: "another organization's caller", handle: depTProxyHandle, org: depTOtherOrg},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := env.svc.DeployByHandle(tc.handle, depTDeployReq("dep", "gw1"), tc.org, "deployer")
			depTWantErr(t, "DeployByHandle", err, apperror.AgentProxyNotFound)
		})
	}
}

func TestAgentDeployment_DeployToUnknownOrForeignGatewayIsGatewayNotFound(t *testing.T) {
	env := depTSetup(t)
	for _, gw := range []string{"no-such-gateway", "gw-foreign"} {
		_, err := env.svc.DeployByHandle(depTProxyHandle, depTDeployReq("dep", gw), importTestOrgID, "deployer")
		depTWantErr(t, "DeployByHandle("+gw+")", err, apperror.GatewayNotFound)
	}
	var n int
	if err := env.db.QueryRow(`SELECT COUNT(*) FROM deployments WHERE artifact_uuid = ?`, env.proxy).Scan(&n); err != nil {
		t.Fatalf("count deployments: %v", err)
	}
	if n != 0 {
		t.Fatalf("%d deployments recorded for refused deploys, want none", n)
	}
}

func TestAgentDeployment_DeployCreatesADeployingRecordAndNotifiesTheGateway(t *testing.T) {
	env := depTSetup(t)
	meta := map[string]interface{}{"team": "weather"}
	req := depTDeployReq("first", "gw1")
	req.Metadata = &meta

	resp, err := env.svc.DeployByHandle(depTProxyHandle, req, importTestOrgID, "deployer")
	if err != nil {
		t.Fatalf("DeployByHandle: %v", err)
	}
	if resp.Status != api.DeploymentResponseStatus(model.DeploymentStatusDeploying) {
		t.Fatalf("status = %q, want DEPLOYING", resp.Status)
	}
	if resp.GatewayId != "gw1" || resp.Name != "first" {
		t.Fatalf("response = %+v, want name first on gateway handle gw1", resp)
	}
	if resp.BuildId == nil || *resp.BuildId == "" {
		t.Fatal("a deploy from current names no build")
	}
	if resp.Metadata == nil || (*resp.Metadata)["team"] != "weather" {
		t.Fatalf("metadata = %v, want the request's metadata seeding the association", resp.Metadata)
	}

	depID, status, _, err := env.deployment.GetStatus(env.proxy, importTestOrgID, importTestGatewayID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if depID != resp.DeploymentId.String() || status != model.DeploymentStatusDeploying {
		t.Fatalf("current = (%s, %s), want (%s, DEPLOYING)", depID, status, resp.DeploymentId)
	}

	var content []byte
	if err := env.db.QueryRow(`SELECT content FROM deployments WHERE uuid = ?`, depID).Scan(&content); err != nil {
		t.Fatalf("read content: %v", err)
	}
	var artifact map[string]any
	if err := yaml.Unmarshal(content, &artifact); err != nil {
		t.Fatalf("stored content is not YAML: %v", err)
	}
	if artifact["kind"] != constants.GatewayKindAgent || artifact["apiVersion"] != constants.GatewayApiVersion {
		t.Fatalf("stored kind/apiVersion = %v/%v, want %s/%s",
			artifact["kind"], artifact["apiVersion"], constants.GatewayKindAgent, constants.GatewayApiVersion)
	}

	if types := env.eventTypes(t); len(types) != 1 || types[0] != EventTypeAgentDeployed {
		t.Fatalf("events = %v, want one %s", types, EventTypeAgentDeployed)
	}
}

func TestAgentDeployment_DeployKeepsAnExistingAssociationsMetadata(t *testing.T) {
	env := depTSetup(t)
	first := map[string]interface{}{"team": "weather"}
	req := depTDeployReq("first", "gw1")
	req.Metadata = &first
	if _, err := env.svc.DeployByHandle(depTProxyHandle, req, importTestOrgID, "deployer"); err != nil {
		t.Fatalf("first deploy: %v", err)
	}

	// No metadata on the redeploy: the association's stored metadata applies.
	resp, err := env.svc.DeployByHandle(depTProxyHandle, depTDeployReq("second", "gw1"), importTestOrgID, "deployer")
	if err != nil {
		t.Fatalf("second deploy: %v", err)
	}
	if resp.Metadata == nil || (*resp.Metadata)["team"] != "weather" {
		t.Fatalf("metadata = %v, want the association's stored metadata", resp.Metadata)
	}
}

func TestAgentDeployment_DeployRefusesAMisconfiguredDeploymentLimit(t *testing.T) {
	env := depTSetup(t)
	env.cfg.Deployments.MaxPerAPIGateway = 0
	if _, err := env.svc.DeployByHandle(depTProxyHandle, depTDeployReq("dep", "gw1"), importTestOrgID, "deployer"); err == nil {
		t.Fatal("DeployByHandle succeeded with MaxPerAPIGateway = 0, want an error")
	}
	if n := len(env.hub.published); n != 0 {
		t.Fatalf("%d events broadcast for a refused deploy, want none", n)
	}
}

func TestAgentDeployment_DeployFromAnUnknownBuildIsBuildNotFound(t *testing.T) {
	env := depTSetup(t)
	missing := "no-such-build"
	req := &api.DeployRequest{Name: "dep", Base: "build", BuildId: &missing, GatewayId: "gw1"}
	_, err := env.svc.DeployByHandle(depTProxyHandle, req, importTestOrgID, "deployer")
	depTWantErr(t, "DeployByHandle", err, apperror.BuildNotFound)
}

func TestAgentDeployment_DataPlaneOriginIsReadOnlyForEveryLifecycleOperation(t *testing.T) {
	env := depTSetup(t)
	env.createProxy(t, "dp-agent", constants.OriginDP)

	_, err := env.svc.DeployByHandle("dp-agent", depTDeployReq("dep", "gw1"), importTestOrgID, "deployer")
	depTWantErr(t, "DeployByHandle", err, apperror.ArtifactReadOnly)
	_, err = env.svc.UndeployByHandle("dp-agent", "any", "gw1", importTestOrgID)
	depTWantErr(t, "UndeployByHandle", err, apperror.ArtifactReadOnly)
	_, err = env.svc.RestoreByHandle("dp-agent", "any", "gw1", importTestOrgID)
	depTWantErr(t, "RestoreByHandle", err, apperror.ArtifactReadOnly)
	err = env.svc.DeleteByHandle("dp-agent", "any", importTestOrgID)
	depTWantErr(t, "DeleteByHandle", err, apperror.ArtifactReadOnly)
}

func TestAgentDeployment_BuildsLifecycleByHandle(t *testing.T) {
	env := depTSetup(t)

	build, err := env.svc.CreateBuildByHandle(depTProxyHandle, importTestOrgID, "builder", "first build",
		map[string]interface{}{"ticket": "T-1"})
	if err != nil {
		t.Fatalf("CreateBuildByHandle: %v", err)
	}
	if build.BuildId == "" || build.Description == nil || *build.Description != "first build" {
		t.Fatalf("build = %+v, want an id and the given description", build)
	}

	got, err := env.svc.GetBuildByHandle(depTProxyHandle, build.BuildId, importTestOrgID)
	if err != nil || got.BuildId != build.BuildId {
		t.Fatalf("GetBuildByHandle = (%+v, %v), want build %s", got, err, build.BuildId)
	}

	list, err := env.svc.GetBuildsByHandle(depTProxyHandle, importTestOrgID, 10)
	if err != nil || list.Count != 1 || list.List[0].BuildId != build.BuildId {
		t.Fatalf("GetBuildsByHandle = (%+v, %v), want the one build", list, err)
	}

	// The prepared build deploys as-is.
	id := build.BuildId
	resp, err := env.svc.DeployByHandle(depTProxyHandle,
		&api.DeployRequest{Name: "from-build", Base: "build", BuildId: &id, GatewayId: "gw1"}, importTestOrgID, "deployer")
	if err != nil {
		t.Fatalf("DeployByHandle(base=build): %v", err)
	}
	if resp.BuildId == nil || *resp.BuildId != build.BuildId {
		t.Fatalf("deployment buildId = %v, want %s", resp.BuildId, build.BuildId)
	}

	// A build a deployment runs cannot be removed; once the deployment record is
	// gone, it can.
	if err := env.svc.DeleteBuildByHandle(depTProxyHandle, build.BuildId, importTestOrgID); err == nil {
		t.Fatal("DeleteBuildByHandle removed a build held by a deployment")
	}
	env.ack(t, resp.DeploymentId.String(), model.DeploymentStatusUndeployed, "")
	if err := env.svc.DeleteByHandle(depTProxyHandle, resp.DeploymentId.String(), importTestOrgID); err != nil {
		t.Fatalf("DeleteByHandle: %v", err)
	}
	if err := env.svc.DeleteBuildByHandle(depTProxyHandle, build.BuildId, importTestOrgID); err != nil {
		t.Fatalf("DeleteBuildByHandle: %v", err)
	}
	_, err = env.svc.GetBuildByHandle(depTProxyHandle, build.BuildId, importTestOrgID)
	depTWantErr(t, "GetBuildByHandle after delete", err, apperror.BuildNotFound)
}

func TestAgentDeployment_BuildOperationsOnAnUnknownOrForeignAgentProxyAreNotFound(t *testing.T) {
	env := depTSetup(t)
	for _, tc := range []struct{ name, handle, org string }{
		{name: "unknown handle", handle: "no-such-agent", org: importTestOrgID},
		{name: "another organization", handle: depTProxyHandle, org: depTOtherOrg},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := env.svc.CreateBuildByHandle(tc.handle, tc.org, "builder", "", nil)
			depTWantErr(t, "CreateBuildByHandle", err, apperror.AgentProxyNotFound)
			_, err = env.svc.GetBuildByHandle(tc.handle, "b", tc.org)
			depTWantErr(t, "GetBuildByHandle", err, apperror.AgentProxyNotFound)
			_, err = env.svc.GetBuildsByHandle(tc.handle, tc.org, 10)
			depTWantErr(t, "GetBuildsByHandle", err, apperror.AgentProxyNotFound)
			err = env.svc.DeleteBuildByHandle(tc.handle, "b", tc.org)
			depTWantErr(t, "DeleteBuildByHandle", err, apperror.AgentProxyNotFound)
		})
	}
}

func TestAgentDeployment_UndeployMovesAnActiveDeploymentToUndeploying(t *testing.T) {
	env := depTSetup(t)
	depID := env.deploy(t, "gw1")
	env.ack(t, depID, model.DeploymentStatusDeployed, "")
	env.hub.published = nil

	resp, err := env.svc.UndeployByHandle(depTProxyHandle, depID, "gw1", importTestOrgID)
	if err != nil {
		t.Fatalf("UndeployByHandle: %v", err)
	}
	if resp.Status != api.DeploymentResponseStatus(model.DeploymentStatusUndeploying) || resp.UpdatedAt == nil {
		t.Fatalf("response = %+v, want UNDEPLOYING with an updatedAt", resp)
	}
	if _, status, _, _ := env.deployment.GetStatus(env.proxy, importTestOrgID, importTestGatewayID); status != model.DeploymentStatusUndeploying {
		t.Fatalf("stored status = %s, want UNDEPLOYING", status)
	}
	if types := env.eventTypes(t); len(types) != 1 || types[0] != EventTypeAgentUndeployed {
		t.Fatalf("events = %v, want one %s", types, EventTypeAgentUndeployed)
	}
}

func TestAgentDeployment_UndeployRefusals(t *testing.T) {
	env := depTSetup(t)
	depID := env.deploy(t, "gw1")

	_, err := env.svc.UndeployByHandle("no-such-agent", depID, "gw1", importTestOrgID)
	depTWantErr(t, "unknown Agent proxy", err, apperror.AgentProxyNotFound)
	_, err = env.svc.UndeployByHandle(depTProxyHandle, depID, " ", importTestOrgID)
	depTWantErr(t, "blank gateway", err, apperror.AgentProxyDeploymentValidationFailed)
	_, err = env.svc.UndeployByHandle(depTProxyHandle, depID, "gw-foreign", importTestOrgID)
	depTWantErr(t, "foreign gateway", err, apperror.GatewayNotFound)
	_, err = env.svc.UndeployByHandle(depTProxyHandle, "00000000-0000-0000-0000-000000000000", "gw1", importTestOrgID)
	depTWantErr(t, "unknown deployment", err, apperror.DeploymentNotFound)
	_, err = env.svc.UndeployByHandle(depTProxyHandle, depID, depTSecondGWHandle, importTestOrgID)
	depTWantErr(t, "other gateway of the org", err, apperror.DeploymentGatewayMismatch)

	env.ack(t, depID, model.DeploymentStatusUndeployed, "")
	_, err = env.svc.UndeployByHandle(depTProxyHandle, depID, "gw1", importTestOrgID)
	depTWantErr(t, "already undeployed", err, apperror.DeploymentNotActive)
}

func TestAgentDeployment_RestoreRedeploysAPreviousDeployment(t *testing.T) {
	env := depTSetup(t)
	first := env.deploy(t, "gw1")
	env.ack(t, first, model.DeploymentStatusDeployed, "")
	second := env.deploy(t, "gw1")
	env.ack(t, second, model.DeploymentStatusDeployed, "")
	env.hub.published = nil

	resp, err := env.svc.RestoreByHandle(depTProxyHandle, first, "gw1", importTestOrgID)
	if err != nil {
		t.Fatalf("RestoreByHandle: %v", err)
	}
	if resp.DeploymentId.String() != first || resp.Status != api.DeploymentResponseStatus(model.DeploymentStatusDeploying) {
		t.Fatalf("response = %+v, want %s DEPLOYING", resp, first)
	}
	current, status, _, err := env.deployment.GetStatus(env.proxy, importTestOrgID, importTestGatewayID)
	if err != nil || current != first || status != model.DeploymentStatusDeploying {
		t.Fatalf("current = (%s, %s, %v), want (%s, DEPLOYING)", current, status, err, first)
	}
	if types := env.eventTypes(t); len(types) != 1 || types[0] != EventTypeAgentDeployed {
		t.Fatalf("events = %v, want one %s", types, EventTypeAgentDeployed)
	}
}

func TestAgentDeployment_RestoreRefusals(t *testing.T) {
	env := depTSetup(t)
	depID := env.deploy(t, "gw1")
	env.ack(t, depID, model.DeploymentStatusDeployed, "")

	_, err := env.svc.RestoreByHandle(depTProxyHandle, depID, "gw1", importTestOrgID)
	depTWantErr(t, "restore the current deployment", err, apperror.DeploymentRestoreConflict)
	_, err = env.svc.RestoreByHandle(depTProxyHandle, "00000000-0000-0000-0000-000000000000", "gw1", importTestOrgID)
	depTWantErr(t, "unknown deployment", err, apperror.DeploymentNotFound)
	_, err = env.svc.RestoreByHandle(depTProxyHandle, depID, depTSecondGWHandle, importTestOrgID)
	depTWantErr(t, "other gateway of the org", err, apperror.DeploymentGatewayMismatch)
	_, err = env.svc.RestoreByHandle(depTProxyHandle, depID, "", importTestOrgID)
	depTWantErr(t, "blank gateway", err, apperror.AgentProxyDeploymentValidationFailed)
	_, err = env.svc.RestoreByHandle(depTProxyHandle, depID, "no-such-gateway", importTestOrgID)
	depTWantErr(t, "unknown gateway", err, apperror.GatewayNotFound)
	_, err = env.svc.RestoreByHandle(depTProxyHandle, depID, "gw1", depTOtherOrg)
	depTWantErr(t, "another organization", err, apperror.AgentProxyNotFound)

	// Once undeployed, the same deployment is restorable.
	env.ack(t, depID, model.DeploymentStatusUndeployed, "")
	if _, err := env.svc.RestoreByHandle(depTProxyHandle, depID, "gw1", importTestOrgID); err != nil {
		t.Fatalf("RestoreByHandle after undeploy: %v", err)
	}
}

func TestAgentDeployment_DeleteOnlyRemovesUndeployedRecords(t *testing.T) {
	env := depTSetup(t)
	superseded := env.deploy(t, "gw1")
	env.ack(t, superseded, model.DeploymentStatusDeployed, "")
	current := env.deploy(t, "gw1")
	env.ack(t, current, model.DeploymentStatusDeployed, "")

	err := env.svc.DeleteByHandle(depTProxyHandle, current, importTestOrgID)
	depTWantErr(t, "delete deployed", err, apperror.DeploymentActive)
	err = env.svc.DeleteByHandle(depTProxyHandle, superseded, importTestOrgID)
	depTWantErr(t, "delete archived", err, apperror.AgentProxyDeploymentNotUndeployed)
	err = env.svc.DeleteByHandle(depTProxyHandle, "00000000-0000-0000-0000-000000000000", importTestOrgID)
	depTWantErr(t, "delete unknown", err, apperror.DeploymentNotFound)
	err = env.svc.DeleteByHandle(depTProxyHandle, current, depTOtherOrg)
	depTWantErr(t, "delete from another organization", err, apperror.AgentProxyNotFound)

	env.ack(t, current, model.DeploymentStatusUndeployed, "")
	env.hub.published = nil
	if err := env.svc.DeleteByHandle(depTProxyHandle, current, importTestOrgID); err != nil {
		t.Fatalf("DeleteByHandle(undeployed): %v", err)
	}
	if n := len(env.hub.published); n != 0 {
		t.Fatalf("%d events broadcast for a record deletion, want none", n)
	}
	_, err = env.svc.GetByHandle(depTProxyHandle, current, importTestOrgID)
	depTWantErr(t, "GetByHandle after delete", err, apperror.DeploymentNotFound)
}

func TestEnsureAgentDeploymentDeletable(t *testing.T) {
	status := func(s model.DeploymentStatus) *model.DeploymentStatus { return &s }
	cases := []struct {
		name   string
		status *model.DeploymentStatus
		want   interface{ Is(error) bool }
	}{
		{name: "undeployed", status: status(model.DeploymentStatusUndeployed)},
		{name: "deployed", status: status(model.DeploymentStatusDeployed), want: apperror.DeploymentActive},
		{name: "deploying", status: status(model.DeploymentStatusDeploying), want: apperror.DeploymentActive},
		{name: "undeploying", status: status(model.DeploymentStatusUndeploying), want: apperror.AgentProxyDeploymentNotUndeployed},
		{name: "failed", status: status(model.DeploymentStatusFailed), want: apperror.AgentProxyDeploymentNotUndeployed},
		{name: "archived", status: status(model.DeploymentStatusArchived), want: apperror.AgentProxyDeploymentNotUndeployed},
		{name: "nil (superseded)", status: nil, want: apperror.AgentProxyDeploymentNotUndeployed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ensureAgentDeploymentDeletable(tc.status)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("error = %v, want deletable", err)
				}
				return
			}
			depTWantErr(t, "ensureAgentDeploymentDeletable", err, tc.want)
		})
	}
}

func TestAgentDeployment_GetReturnsStateAndFailureReason(t *testing.T) {
	env := depTSetup(t)
	depID := env.deploy(t, "gw1")
	env.ack(t, depID, model.DeploymentStatusFailed, "upstream unreachable")

	got, err := env.svc.GetByHandle(depTProxyHandle, depID, importTestOrgID)
	if err != nil {
		t.Fatalf("GetByHandle: %v", err)
	}
	if got.Status != api.DeploymentResponseStatus(model.DeploymentStatusFailed) {
		t.Fatalf("status = %s, want FAILED", got.Status)
	}
	if got.StatusReason == nil || *got.StatusReason != "upstream unreachable" {
		t.Fatalf("statusReason = %v, want the gateway's reason", got.StatusReason)
	}
	if got.BuildId == nil || *got.BuildId == "" {
		t.Fatal("GetByHandle names no build")
	}

	_, err = env.svc.GetByHandle(depTProxyHandle, "00000000-0000-0000-0000-000000000000", importTestOrgID)
	depTWantErr(t, "unknown deployment", err, apperror.DeploymentNotFound)
	_, err = env.svc.GetByHandle(depTProxyHandle, depID, depTOtherOrg)
	depTWantErr(t, "another organization", err, apperror.AgentProxyNotFound)

	// A deployment of another Agent proxy is not reachable through this handle.
	env.createProxy(t, "other-agent", constants.OriginCP)
	_, err = env.svc.GetByHandle("other-agent", depID, importTestOrgID)
	depTWantErr(t, "another Agent proxy's deployment", err, apperror.DeploymentNotFound)
}

func TestAgentDeployment_ListFiltersByGatewayAndStatus(t *testing.T) {
	env := depTSetup(t)
	onFirst := env.deploy(t, "gw1")
	env.ack(t, onFirst, model.DeploymentStatusDeployed, "")
	onSecond := env.deploy(t, depTSecondGWHandle)

	all, err := env.svc.ListByHandle(depTProxyHandle, "", "", importTestOrgID)
	if err != nil || all.Count != 2 {
		t.Fatalf("ListByHandle(all) = (%+v, %v), want 2 deployments", all, err)
	}
	for _, d := range all.List {
		if d.BuildId == nil || *d.BuildId == "" {
			t.Fatalf("listed deployment %s names no build", d.DeploymentId)
		}
	}

	byGateway, err := env.svc.ListByHandle(depTProxyHandle, depTSecondGWHandle, "", importTestOrgID)
	if err != nil || byGateway.Count != 1 || byGateway.List[0].DeploymentId.String() != onSecond {
		t.Fatalf("ListByHandle(gw2) = (%+v, %v), want only %s", byGateway, err, onSecond)
	}

	byStatus, err := env.svc.ListByHandle(depTProxyHandle, "", string(model.DeploymentStatusDeployed), importTestOrgID)
	if err != nil || byStatus.Count != 1 || byStatus.List[0].DeploymentId.String() != onFirst {
		t.Fatalf("ListByHandle(DEPLOYED) = (%+v, %v), want only %s", byStatus, err, onFirst)
	}

	// A gateway filter naming no gateway of this org is an empty page, not an error.
	for _, gw := range []string{"no-such-gateway", "gw-foreign"} {
		none, err := env.svc.ListByHandle(depTProxyHandle, gw, "", importTestOrgID)
		if err != nil || none.Count != 0 || none.List == nil || len(none.List) != 0 {
			t.Fatalf("ListByHandle(%s) = (%+v, %v), want an empty list", gw, none, err)
		}
	}
}

func TestAgentDeployment_ListRefusals(t *testing.T) {
	env := depTSetup(t)

	_, err := env.svc.ListByHandle(depTProxyHandle, "", "deployed-ish", importTestOrgID)
	depTWantErr(t, "invalid status filter", err, apperror.DeploymentInvalidStatus)
	_, err = env.svc.ListByHandle("no-such-agent", "", "", importTestOrgID)
	depTWantErr(t, "unknown Agent proxy", err, apperror.AgentProxyNotFound)
	_, err = env.svc.ListByHandle(depTProxyHandle, "", "", depTOtherOrg)
	depTWantErr(t, "another organization", err, apperror.AgentProxyNotFound)

	env.cfg.Deployments.MaxPerAPIGateway = 0
	if _, err := env.svc.ListByHandle(depTProxyHandle, "", "", importTestOrgID); err == nil {
		t.Fatal("ListByHandle succeeded with MaxPerAPIGateway = 0, want an error")
	}
}

func TestIsValidDeploymentStatusFilter(t *testing.T) {
	for _, s := range []model.DeploymentStatus{
		model.DeploymentStatusDeployed, model.DeploymentStatusUndeployed, model.DeploymentStatusArchived,
		model.DeploymentStatusDeploying, model.DeploymentStatusUndeploying, model.DeploymentStatusFailed,
	} {
		if !isValidDeploymentStatusFilter(string(s)) {
			t.Errorf("isValidDeploymentStatusFilter(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "deployed", "ACTIVE", "DEPLOYED "} {
		if isValidDeploymentStatusFilter(s) {
			t.Errorf("isValidDeploymentStatusFilter(%q) = true, want false", s)
		}
	}
}

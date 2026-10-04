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

// Gateway acknowledgements for Agent proxy deployments, end to end: a real
// deploy/undeploy over HTTP records the transitional state, the ack is applied
// through DeploymentService.HandleDeploymentAck — exactly what the gateway
// WebSocket handler calls for a deployment.ack message — and the public
// deployment resources report the result.
//
// Agent acks take the same path as every other kind's: the gateway acks with
// resourceType Agent and the AgentProxy's artifact UUID, and the shared handler
// applies it by artifact, gateway and performedAt.

package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// agentAckEnv is a deployable Agent proxy plus the service that applies acks.
type agentAckEnv struct {
	*agentDeployEnv
	acks           *service.DeploymentService
	deploymentRepo repository.DeploymentRepository
	proxyUUID      string
}

func setupAgentAckEnv(t *testing.T) *agentAckEnv {
	t.Helper()
	env := setupAgentDeployEnv(t)
	registry := repository.NewArtifactTableRegistry()
	deploymentRepo := repository.NewDeploymentRepo(env.db, registry)
	acks := service.NewDeploymentService(nil, repository.NewArtifactRepo(env.db, registry), deploymentRepo,
		repository.NewGatewayRepo(env.db), nil, nil, nil, nil, nil,
		service.NewArtifactDefinitions(), agentDeployConfig(), slog.Default())

	var proxyUUID string
	if err := env.db.QueryRow(`SELECT uuid FROM agent_proxies WHERE handle = ? AND organization_uuid = ?`,
		env.proxy, agentProxyOrg).Scan(&proxyUUID); err != nil {
		t.Fatalf("resolve Agent proxy UUID: %v", err)
	}
	return &agentAckEnv{agentDeployEnv: env, acks: acks, deploymentRepo: deploymentRepo, proxyUUID: proxyUUID}
}

// performedAt is the concurrency token the control plane recorded for the
// gateway's current deployment — what the event carried and the ack echoes.
func (e *agentAckEnv) performedAt(t *testing.T, gatewayUUID string) time.Time {
	t.Helper()
	_, _, performedAt, _, err := e.deploymentRepo.GetStatusFull(e.proxyUUID, agentProxyOrg, gatewayUUID)
	if err != nil || performedAt == nil {
		t.Fatalf("read performedAt: %v (performedAt=%v)", err, performedAt)
	}
	return *performedAt
}

// gatewayAck builds the ack the gateway sends for the Agent proxy.
func (e *agentAckEnv) gatewayAck(deploymentID, action, status, errorCode string, performedAt time.Time) *model.DeploymentAckPayload {
	return &model.DeploymentAckPayload{
		DeploymentID: deploymentID,
		ArtifactID:   e.proxyUUID,
		ResourceType: constants.GatewayKindAgent,
		Action:       action,
		Status:       status,
		PerformedAt:  performedAt,
		ErrorCode:    errorCode,
	}
}

func (e *agentAckEnv) apply(t *testing.T, ack *model.DeploymentAckPayload) {
	t.Helper()
	if err := e.acks.HandleDeploymentAck(e.gatewayUUID, agentProxyOrg, ack); err != nil {
		t.Fatalf("HandleDeploymentAck: %v", err)
	}
}

// deployment reads the public deployment resource.
func (e *agentAckEnv) deployment(t *testing.T, deploymentID string) map[string]any {
	t.Helper()
	return decodeAgentProxyJSON(t, callAgentProxy(t, e.handler, http.MethodGet,
		e.deploymentsPath()+"/"+deploymentID, ""), http.StatusOK)
}

func (e *agentAckEnv) assertState(t *testing.T, deploymentID, wantStatus string, wantReason any) {
	t.Helper()
	got := e.deployment(t, deploymentID)
	if got["status"] != wantStatus || got["statusReason"] != wantReason {
		t.Fatalf("deployment = status %v, statusReason %v; want %s, %v", got["status"], got["statusReason"], wantStatus, wantReason)
	}
}

func TestAgentProxyDeploymentAck_SuccessMovesDeployingToDeployed(t *testing.T) {
	env := setupAgentAckEnv(t)
	deploymentID := env.deploy(t, env.gateway)
	env.assertState(t, deploymentID, "DEPLOYING", nil)

	env.apply(t, env.gatewayAck(deploymentID, "deploy", "success", "", env.performedAt(t, env.gatewayUUID)))

	env.assertState(t, deploymentID, "DEPLOYED", nil)
	list := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet,
		env.deploymentsPath()+"?status=DEPLOYED", ""), http.StatusOK)
	if list["count"].(float64) != 1 {
		t.Fatalf("DEPLOYED filter count = %v, want 1 — list must report the same result as GET", list["count"])
	}
}

// A definition the gateway's own validation rejects surfaces as a failed
// deployment carrying the gateway's reason, on both GET and list.
func TestAgentProxyDeploymentAck_GatewayValidationFailureIsSurfaced(t *testing.T) {
	for _, code := range []string{
		model.DeploymentErrorAgentValidationFailed,
		model.DeploymentErrorAgentRenderFailed,
		model.DeploymentErrorAgentArtifactFetchFailed,
		model.DeploymentErrorAgentConflict,
		model.DeploymentErrorGatewayFailure,
	} {
		t.Run(code, func(t *testing.T) {
			env := setupAgentAckEnv(t)
			deploymentID := env.deploy(t, env.gateway)

			env.apply(t, env.gatewayAck(deploymentID, "deploy", "failed", code, env.performedAt(t, env.gatewayUUID)))

			env.assertState(t, deploymentID, "FAILED", code)
			list := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodGet,
				env.deploymentsPath()+"?status=FAILED", ""), http.StatusOK)
			items := list["list"].([]any)
			if len(items) != 1 || items[0].(map[string]any)["statusReason"] != code {
				t.Fatalf("FAILED list = %v, want the one deployment with statusReason %s", items, code)
			}
		})
	}
}

// A gateway's reason is stored as a code or not at all: free text or a value
// too long for the column is recorded as GATEWAY_PROCESSING_ERROR, and a
// failure with no code has no statusReason.
func TestAgentProxyDeploymentAck_ReasonIsSanitized(t *testing.T) {
	for name, tc := range map[string]struct {
		raw  string
		want any
	}{
		"free text":  {"upstream auth failed for https://admin:hunter2@agent.internal", model.DeploymentErrorGatewayFailure},
		"too long":   {strings.Repeat("A", 80), model.DeploymentErrorGatewayFailure},
		"lower case": {"agent_validation_failed", model.DeploymentErrorGatewayFailure},
		"empty":      {"", nil},
	} {
		t.Run(name, func(t *testing.T) {
			env := setupAgentAckEnv(t)
			deploymentID := env.deploy(t, env.gateway)

			env.apply(t, env.gatewayAck(deploymentID, "deploy", "failed", tc.raw, env.performedAt(t, env.gatewayUUID)))

			env.assertState(t, deploymentID, "FAILED", tc.want)
		})
	}
}

// An ack carrying an older performedAt than the current one is discarded: it
// cannot overwrite the newer state, success or failure.
func TestAgentProxyDeploymentAck_StaleAckDoesNotOverwriteNewerState(t *testing.T) {
	env := setupAgentAckEnv(t)
	deploymentID := env.deploy(t, env.gateway)
	current := env.performedAt(t, env.gatewayUUID)
	older := current.Add(-time.Minute)

	env.apply(t, env.gatewayAck(deploymentID, "deploy", "failed", model.DeploymentErrorAgentValidationFailed, older))
	env.assertState(t, deploymentID, "DEPLOYING", nil)
	env.apply(t, env.gatewayAck(deploymentID, "deploy", "success", "", older))
	env.assertState(t, deploymentID, "DEPLOYING", nil)

	env.apply(t, env.gatewayAck(deploymentID, "deploy", "success", "", current))
	env.assertState(t, deploymentID, "DEPLOYED", nil)

	// A redeploy of the same deployment record (restore after undeploy) starts a
	// new performedAt; the old deploy's token is now stale in turn.
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost,
		env.deploymentsPath()+"/"+deploymentID+"/undeploy?gatewayId="+env.gateway, ""), http.StatusAccepted)
	env.apply(t, env.gatewayAck(deploymentID, "undeploy", "failed", model.DeploymentErrorIDMismatch, current))
	env.assertState(t, deploymentID, "UNDEPLOYING", nil)
}

// A success that arrives after the timeout job gave up does not resurrect the
// deployment: the status resource keeps reporting the timeout.
func TestAgentProxyDeploymentAck_LateAckAfterTimeoutIsDiscarded(t *testing.T) {
	env := setupAgentAckEnv(t)
	deploymentID := env.deploy(t, env.gateway)
	token := env.performedAt(t, env.gatewayUUID)
	if _, err := env.deploymentRepo.UpdateStatusWithPerformedAtGuard(env.proxyUUID, agentProxyOrg, env.gatewayUUID,
		model.DeploymentStatusFailed, model.DeploymentErrorTimeout, token,
		[]model.DeploymentStatus{model.DeploymentStatusDeploying}); err != nil {
		t.Fatalf("simulate timeout: %v", err)
	}

	env.apply(t, env.gatewayAck(deploymentID, "deploy", "success", "", token))
	env.assertState(t, deploymentID, "FAILED", model.DeploymentErrorTimeout)
}

func TestAgentProxyDeploymentAck_UndeployTransitions(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		env := setupAgentAckEnv(t)
		deploymentID := env.deploy(t, env.gateway)
		env.apply(t, env.gatewayAck(deploymentID, "deploy", "success", "", env.performedAt(t, env.gatewayUUID)))
		decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost,
			env.deploymentsPath()+"/"+deploymentID+"/undeploy?gatewayId="+env.gateway, ""), http.StatusAccepted)

		// A deploy-action ack cannot complete an undeploy.
		env.apply(t, env.gatewayAck(deploymentID, "deploy", "success", "", env.performedAt(t, env.gatewayUUID)))
		env.assertState(t, deploymentID, "UNDEPLOYING", nil)

		env.apply(t, env.gatewayAck(deploymentID, "undeploy", "success", "", env.performedAt(t, env.gatewayUUID)))
		env.assertState(t, deploymentID, "UNDEPLOYED", nil)
	})
	t.Run("failure", func(t *testing.T) {
		env := setupAgentAckEnv(t)
		deploymentID := env.deploy(t, env.gateway)
		env.apply(t, env.gatewayAck(deploymentID, "deploy", "success", "", env.performedAt(t, env.gatewayUUID)))
		decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost,
			env.deploymentsPath()+"/"+deploymentID+"/undeploy?gatewayId="+env.gateway, ""), http.StatusAccepted)

		env.apply(t, env.gatewayAck(deploymentID, "undeploy", "failed", model.DeploymentErrorIDMismatch, env.performedAt(t, env.gatewayUUID)))
		env.assertState(t, deploymentID, "FAILED", model.DeploymentErrorIDMismatch)
	})
	t.Run("restore then ack", func(t *testing.T) {
		env := setupAgentAckEnv(t)
		deploymentID := env.deploy(t, env.gateway)
		env.apply(t, env.gatewayAck(deploymentID, "deploy", "success", "", env.performedAt(t, env.gatewayUUID)))
		decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost,
			env.deploymentsPath()+"/"+deploymentID+"/undeploy?gatewayId="+env.gateway, ""), http.StatusAccepted)
		env.apply(t, env.gatewayAck(deploymentID, "undeploy", "success", "", env.performedAt(t, env.gatewayUUID)))
		decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost,
			env.deploymentsPath()+"/"+deploymentID+"/restore?gatewayId="+env.gateway, ""), http.StatusAccepted)

		env.apply(t, env.gatewayAck(deploymentID, "deploy", "success", "", env.performedAt(t, env.gatewayUUID)))
		env.assertState(t, deploymentID, "DEPLOYED", nil)
	})
}

// Acks are scoped to the acknowledging gateway and its organization: another
// gateway's ack or another organization's connection changes nothing.
func TestAgentProxyDeploymentAck_ScopedToGatewayAndOrganization(t *testing.T) {
	env := setupAgentAckEnv(t)
	otherGateway := seedAgentGateway(t, env.db, agentProxyOrg, "ai-gw-2", "1.2.0")
	deploymentID := env.deploy(t, env.gateway)
	ack := env.gatewayAck(deploymentID, "deploy", "success", "", env.performedAt(t, env.gatewayUUID))

	if err := env.acks.HandleDeploymentAck(otherGateway, agentProxyOrg, ack); err != nil {
		t.Fatalf("ack from another gateway: %v", err)
	}
	if err := env.acks.HandleDeploymentAck(env.gatewayUUID, agentProxyOtherOrg, ack); err != nil {
		t.Fatalf("ack from another organization: %v", err)
	}
	env.assertState(t, deploymentID, "DEPLOYING", nil)
}

// An ack that arrives after the Agent proxy was deleted is discarded quietly.
func TestAgentProxyDeploymentAck_AfterAgentDeletionIsDiscarded(t *testing.T) {
	env := setupAgentAckEnv(t)
	deploymentID := env.deploy(t, env.gateway)
	token := env.performedAt(t, env.gatewayUUID)
	env.ack(t, deploymentID, "UNDEPLOYED", "")
	if rec := callAgentProxy(t, env.handler, http.MethodDelete, agentProxyBase+"/"+env.proxy, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete Agent proxy = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	env.apply(t, env.gatewayAck(deploymentID, "deploy", "success", "", token))
	if n := env.countRows(t, `SELECT COUNT(*) FROM deployment_status WHERE artifact_uuid = ?`, env.proxyUUID); n != 0 {
		t.Fatalf("%d deployment_status rows after a post-deletion ack, want 0", n)
	}
}

func TestAgentProxyDeploymentAck_RejectsUnknownActionAndStatus(t *testing.T) {
	env := setupAgentAckEnv(t)
	deploymentID := env.deploy(t, env.gateway)
	token := env.performedAt(t, env.gatewayUUID)

	for _, ack := range []*model.DeploymentAckPayload{
		env.gatewayAck(deploymentID, "delete", "success", "", token),
		env.gatewayAck(deploymentID, "deploy", "partial", "", token),
	} {
		if err := env.acks.HandleDeploymentAck(env.gatewayUUID, agentProxyOrg, ack); err == nil {
			t.Fatalf("ack %+v accepted, want an error", ack)
		}
	}
	env.assertState(t, deploymentID, "DEPLOYING", nil)
}

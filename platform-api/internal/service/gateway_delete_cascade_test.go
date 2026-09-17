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
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

const (
	cascadeGatewayHandle = "prod-gw"
	cascadeGatewayUUID   = "gw-uuid-1"
	cascadeOrgUUID       = "org-uuid-1"
)

// cascadeGatewayRepo serves one gateway and records that Delete ran.
type cascadeGatewayRepo struct {
	repository.GatewayRepository
	deleted   bool
	deleteErr error
}

func (r *cascadeGatewayRepo) GetByHandleAndOrgID(handle, orgID string) (*model.Gateway, error) {
	if handle != cascadeGatewayHandle || orgID != cascadeOrgUUID {
		return nil, nil
	}
	return &model.Gateway{ID: cascadeGatewayUUID, Handle: cascadeGatewayHandle}, nil
}

func (r *cascadeGatewayRepo) Delete(gatewayID, orgID string) error {
	r.deleted = true
	return r.deleteErr
}

// cascadeDeploymentRepo models the gateway's deployments through the batched drain:
// one bulk transition, then a count that reports what is still awaiting acknowledgement.
type cascadeDeploymentRepo struct {
	repository.DeploymentRepository
	deployments []*model.DeploymentInfo
	err         error
	// acksAfter: after this many count calls, report nothing pending (gateway acked).
	acksAfter   int
	countCalls  int
	marked      int64
	performedAt []time.Time
}

func (r *cascadeDeploymentRepo) GetControlPlaneDeploymentsByGateway(
	gatewayID, orgUUID string, since *time.Time) ([]*model.DeploymentInfo, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.deployments, nil
}

// MarkGatewayDeploymentsUndeploying records the single bulk transition the drain makes
// before publishing, with the performed_at token the events must carry.
func (r *cascadeDeploymentRepo) MarkGatewayDeploymentsUndeploying(gatewayUUID, orgUUID string,
	performedAt time.Time) (int64, error) {
	r.marked++
	r.performedAt = append(r.performedAt, performedAt)
	return int64(len(r.deployments)), nil
}

func (r *cascadeDeploymentRepo) CountGatewayDeploymentsAwaitingUndeployAck(
	gatewayUUID, orgUUID string) (int, error) {
	r.countCalls++
	if r.acksAfter > 0 && r.countCalls > r.acksAfter {
		return 0, nil
	}
	return len(r.deployments), nil
}

func deployedOn(artifactUUID, deploymentID, kind string, status model.DeploymentStatus) *model.DeploymentInfo {
	return &model.DeploymentInfo{
		DeploymentID: deploymentID,
		ArtifactID:   artifactUUID,
		Type:         kind,
		Status:       status,
	}
}

func newCascadeService(gwRepo *cascadeGatewayRepo, depRepo *cascadeDeploymentRepo,
	hub eventhub.EventHub) *GatewayService {

	return &GatewayService{
		gatewayRepo:          gwRepo,
		deploymentRepo:       depRepo,
		gatewayEventsService: NewGatewayEventsService(hub, nil, slog.Default()),
		slogger:              slog.Default(),
	}
}

// Deleting a gateway must tell it to drop everything it is serving. Without this the
// records go and the gateway keeps serving artifacts nothing refers to any more.
func TestDeleteGateway_UndeploysEveryKindItIsRunning(t *testing.T) {
	gwRepo := &cascadeGatewayRepo{}
	depRepo := &cascadeDeploymentRepo{acksAfter: 1, deployments: []*model.DeploymentInfo{
		deployedOn("rest-1", "dep-1", constants.RestApi, model.DeploymentStatusDeployed),
		deployedOn("prov-1", "dep-2", constants.LLMProvider, model.DeploymentStatusDeployed),
		deployedOn("proxy-1", "dep-3", constants.LLMProxy, model.DeploymentStatusDeployed),
		deployedOn("mcp-1", "dep-4", constants.MCPProxy, model.DeploymentStatusDeployed),
	}}
	hub := &capturingEventHub{}
	svc := newCascadeService(gwRepo, depRepo, hub)

	if err := svc.DeleteGateway(cascadeGatewayHandle, cascadeOrgUUID, "tester"); err != nil {
		t.Fatalf("DeleteGateway: %v", err)
	}
	if len(hub.published) != 4 {
		t.Fatalf("published %d undeployment events, want one per deployment", len(hub.published))
	}
	// Each kind must get ITS OWN event: an MCP proxy told to undeploy through the REST
	// event would not be matched by the gateway, and the artifact would stay serving.
	want := map[string]string{
		"dep-1": EventTypeAPIUndeployed,
		"dep-2": EventTypeLLMProviderUndeployed,
		"dep-3": EventTypeLLMProxyUndeployed,
		"dep-4": EventTypeMCPProxyUndeployed,
	}
	for _, published := range hub.published {
		var envelope struct {
			Type    string `json:"type"`
			Payload struct {
				DeploymentID string `json:"deploymentId"`
			} `json:"payload"`
		}
		if err := json.Unmarshal([]byte(published.EventData), &envelope); err != nil {
			t.Fatalf("event data is not the expected envelope: %v", err)
		}
		expected, known := want[envelope.Payload.DeploymentID]
		if !known {
			t.Errorf("an event was published for unknown deployment %q", envelope.Payload.DeploymentID)
			continue
		}
		if envelope.Type != expected {
			t.Errorf("deployment %s was undeployed as %q, want %q",
				envelope.Payload.DeploymentID, envelope.Type, expected)
		}
		delete(want, envelope.Payload.DeploymentID)
	}
	if len(want) != 0 {
		t.Errorf("no undeployment event for %v", want)
	}
	if !gwRepo.deleted {
		t.Error("the gateway was not deleted")
	}
}

// A deployment the gateway is not serving needs no event: sending one would ask it to
// drop something it does not have.
func TestDeleteGateway_SkipsWhatIsNotOnTheGateway(t *testing.T) {
	gwRepo := &cascadeGatewayRepo{}
	depRepo := &cascadeDeploymentRepo{acksAfter: 1, deployments: []*model.DeploymentInfo{
		deployedOn("rest-1", "dep-1", constants.RestApi, model.DeploymentStatusDeployed),
		deployedOn("rest-2", "dep-2", constants.RestApi, model.DeploymentStatusUndeployed),
	}}
	hub := &capturingEventHub{}
	svc := newCascadeService(gwRepo, depRepo, hub)

	if err := svc.DeleteGateway(cascadeGatewayHandle, cascadeOrgUUID, "tester"); err != nil {
		t.Fatalf("DeleteGateway: %v", err)
	}
	if len(hub.published) != 1 {
		t.Errorf("published %d events, want only the deployed one", len(hub.published))
	}
}

// An event store that refuses the write is NOT the same as an offline gateway: an offline
// gateway's event is queued fine and waits. If the undeployment cannot even be queued,
// deleting the records would leave the gateway serving an artifact nothing refers to —
// the original bug. The delete must be refused, and the deployment put back so a retry
// picks it up again (runningOnGateway only selects deployed/deploying).
func TestDeleteGateway_RefusesWhenTheUndeploymentCannotBeQueued(t *testing.T) {
	gwRepo := &cascadeGatewayRepo{}
	depRepo := &cascadeDeploymentRepo{acksAfter: 1, deployments: []*model.DeploymentInfo{
		deployedOn("rest-1", "dep-1", constants.RestApi, model.DeploymentStatusDeployed),
	}}
	hub := &failingEventHub{}
	svc := newCascadeService(gwRepo, depRepo, hub)

	err := svc.DeleteGateway(cascadeGatewayHandle, cascadeOrgUUID, "tester")
	if err == nil {
		t.Fatal("the gateway was deleted even though its undeployment could not be queued")
	}
	if gwRepo.deleted {
		t.Error("the records were removed while the gateway still serves the artifact")
	}
	// The deployments are left UNDEPLOYING, which toUndeployOnGateway re-selects, so a
	// retry undeploys them again rather than skipping them forever.
	if depRepo.marked != 1 {
		t.Errorf("bulk transition ran %d time(s), want exactly one before the failed publish",
			depRepo.marked)
	}
}

// Likewise if the deployments cannot even be listed: the delete still proceeds.
func TestDeleteGateway_ProceedsWhenTheDeploymentsCannotBeListed(t *testing.T) {
	gwRepo := &cascadeGatewayRepo{}
	depRepo := &cascadeDeploymentRepo{err: errors.New("database unavailable")}
	hub := &capturingEventHub{}
	svc := newCascadeService(gwRepo, depRepo, hub)

	if err := svc.DeleteGateway(cascadeGatewayHandle, cascadeOrgUUID, "tester"); err != nil {
		t.Fatalf("DeleteGateway: %v", err)
	}
	if !gwRepo.deleted {
		t.Error("the gateway was not deleted")
	}
}

// failingEventHub stands in for a gateway that cannot be reached.
type failingEventHub struct{ capturingEventHub }

func (h *failingEventHub) PublishEvent(string, eventhub.Event) error {
	return errors.New("gateway not connected")
}

// A delete that fails AFTER the undeployment events were published must surface the
// error, because that is what lets the caller retry. The records still read DEPLOYED, so
// a retry republishes (a repeat is a no-op on the gateway, matched by deployment id and
// timestamp) and removes them. Swallowing the error here would strand the gateway
// undeployed with its records intact and nothing to prompt another attempt.
func TestDeleteGateway_SurfacesADeleteFailureSoItCanBeRetried(t *testing.T) {
	gwRepo := &cascadeGatewayRepo{deleteErr: errors.New("database unavailable")}
	depRepo := &cascadeDeploymentRepo{acksAfter: 1, deployments: []*model.DeploymentInfo{
		deployedOn("rest-1", "dep-1", constants.RestApi, model.DeploymentStatusDeployed),
	}}
	hub := &capturingEventHub{}
	svc := newCascadeService(gwRepo, depRepo, hub)

	err := svc.DeleteGateway(cascadeGatewayHandle, cascadeOrgUUID, "tester")
	if err == nil {
		t.Fatal("a failed delete reported success; the caller would never retry")
	}
	if len(hub.published) != 1 {
		t.Errorf("published %d events, want the undeployment to have been attempted", len(hub.published))
	}
}

// The gateway matches its acknowledgement against the stored performed_at, so the drain
// must record the UNDEPLOYING transition with the SAME token it puts in the event.
// Publishing a timestamp the row does not carry has the acknowledgement discarded as
// stale, and the artifact silently stays deployed.
func TestDeleteGateway_RecordsTheTokenItPublishes(t *testing.T) {
	gwRepo := &cascadeGatewayRepo{}
	depRepo := &cascadeDeploymentRepo{acksAfter: 1, deployments: []*model.DeploymentInfo{
		deployedOn("rest-1", "dep-1", constants.RestApi, model.DeploymentStatusDeployed),
	}}
	hub := &capturingEventHub{}
	svc := newCascadeService(gwRepo, depRepo, hub)

	if err := svc.DeleteGateway(cascadeGatewayHandle, cascadeOrgUUID, "tester"); err != nil {
		t.Fatalf("DeleteGateway: %v", err)
	}
	if len(depRepo.performedAt) != 1 {
		t.Fatal("no performed_at token was stored; the gateway's ack would be discarded as stale")
	}
	var envelope struct {
		Payload struct {
			PerformedAt time.Time `json:"performedAt"`
		} `json:"payload"`
	}
	if err := json.Unmarshal([]byte(hub.published[0].EventData), &envelope); err != nil {
		t.Fatalf("event data: %v", err)
	}
	if !envelope.Payload.PerformedAt.Equal(depRepo.performedAt[0]) {
		t.Errorf("published token %v does not match the stored token %v; the ack would be discarded",
			envelope.Payload.PerformedAt, depRepo.performedAt[0])
	}
}

// The delete must not remove the gateway until the artifacts are actually gone. The
// events live in hub rows keyed to the gateway, so deleting it cascades them away —
// publishing and deleting in one breath destroys the undeployment before any replica can
// deliver it. The drain therefore re-reads until nothing is left deployed.
func TestDeleteGateway_WaitsForTheGatewayToConfirm(t *testing.T) {
	gwRepo := &cascadeGatewayRepo{}
	// Still deployed on the first three reads; gone from the fourth.
	depRepo := &cascadeDeploymentRepo{acksAfter: 3, deployments: []*model.DeploymentInfo{
		deployedOn("rest-1", "dep-1", constants.RestApi, model.DeploymentStatusDeployed),
	}}
	hub := &capturingEventHub{}
	svc := newCascadeService(gwRepo, depRepo, hub)

	if err := svc.DeleteGateway(cascadeGatewayHandle, cascadeOrgUUID, "tester"); err != nil {
		t.Fatalf("DeleteGateway: %v", err)
	}
	if depRepo.countCalls < 3 {
		t.Errorf("the pending count was read %d time(s); the delete did not wait for confirmation",
			depRepo.countCalls)
	}
	if !gwRepo.deleted {
		t.Error("the gateway was never deleted after the artifacts drained")
	}
}

// A deployment left UNDEPLOYING with nothing queued to move it on — a publish failed and
// the restore failed too — must still be picked up by a later delete. Selecting only
// deployed/deploying made it invisible to every retry, so the records would be removed
// while the gateway carried on serving the artifact.
func TestDeleteGateway_RetriesADeploymentStuckUndeploying(t *testing.T) {
	gwRepo := &cascadeGatewayRepo{}
	depRepo := &cascadeDeploymentRepo{acksAfter: 1, deployments: []*model.DeploymentInfo{
		deployedOn("rest-1", "dep-1", constants.RestApi, model.DeploymentStatusUndeploying),
	}}
	hub := &capturingEventHub{}
	svc := newCascadeService(gwRepo, depRepo, hub)

	if err := svc.DeleteGateway(cascadeGatewayHandle, cascadeOrgUUID, "tester"); err != nil {
		t.Fatalf("DeleteGateway: %v", err)
	}
	if len(hub.published) != 1 {
		t.Fatalf("published %d events; a deployment stuck UNDEPLOYING was never re-undeployed",
			len(hub.published))
	}
	if !gwRepo.deleted {
		t.Error("the gateway was not deleted")
	}
}

// batchingEventHub records how many publish CALLS were made, not just how many events,
// so a test can tell one batch from many individual publishes.
type batchingEventHub struct {
	capturingEventHub
	batchCalls  int
	singleCalls int
}

func (h *batchingEventHub) PublishEvent(gatewayID string, e eventhub.Event) error {
	h.singleCalls++
	return h.capturingEventHub.PublishEvent(gatewayID, e)
}

func (h *batchingEventHub) PublishEventBatch(gatewayID string, events []eventhub.Event) error {
	h.batchCalls++
	for i := range events {
		h.published = append(h.published, events[i])
	}
	return nil
}

// A gateway can hold thousands of artifacts. Undeploying them one at a time would be a
// database transaction each, putting the delete far beyond any caller's timeout — and a
// timeout here deletes the gateway with events still queued, which is the very bug this
// change exists to fix. The whole set must go out as one bulk transition and one publish.
func TestDeleteGateway_UndeploysManyArtifactsInBulk(t *testing.T) {
	const artifacts = 500
	deployments := make([]*model.DeploymentInfo, 0, artifacts)
	for i := 0; i < artifacts; i++ {
		deployments = append(deployments, deployedOn(
			fmt.Sprintf("artifact-%d", i), fmt.Sprintf("dep-%d", i),
			constants.RestApi, model.DeploymentStatusDeployed))
	}
	gwRepo := &cascadeGatewayRepo{}
	depRepo := &cascadeDeploymentRepo{acksAfter: 1, deployments: deployments}
	hub := &batchingEventHub{}
	svc := newCascadeService(gwRepo, depRepo, hub)

	if err := svc.DeleteGateway(cascadeGatewayHandle, cascadeOrgUUID, "tester"); err != nil {
		t.Fatalf("DeleteGateway: %v", err)
	}
	if depRepo.marked != 1 {
		t.Errorf("status transition ran %d time(s) for %d artifacts, want one bulk statement",
			depRepo.marked, artifacts)
	}
	if hub.batchCalls != 1 {
		t.Errorf("publish was called %d time(s) as a batch, want exactly one", hub.batchCalls)
	}
	if hub.singleCalls != 0 {
		t.Errorf("%d event(s) were published individually; that is a transaction each",
			hub.singleCalls)
	}
	if len(hub.published) != artifacts {
		t.Errorf("published %d events, want one per artifact (%d)", len(hub.published), artifacts)
	}
	if !gwRepo.deleted {
		t.Error("the gateway was not deleted")
	}
}

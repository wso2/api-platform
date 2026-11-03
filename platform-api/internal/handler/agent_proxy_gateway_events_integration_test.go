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

// Gateway events for Agent proxies, end to end over the real route -> handler ->
// service -> GatewayEventsService -> SQL EventHub stack. These assert what is
// durably recorded for each public mutation, that a recorded event reaches a
// gateway whether it is connected at the time or connects later, and that a
// failed publish is handled as it is for the other kinds.

package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
)

// faultyEventHub is the real SQL-backed EventHub with a switch that makes
// PublishEvent fail, standing in for the EventHub's DB write failing.
type faultyEventHub struct {
	eventhub.EventHub
	failPublish atomic.Bool
}

func (h *faultyEventHub) PublishEvent(gatewayID string, event eventhub.Event) error {
	if h.failPublish.Load() {
		return errors.New("injected EventHub publish failure")
	}
	return h.EventHub.PublishEvent(gatewayID, event)
}

// newFaultyEventHub starts a real EventHub over db, polling fast enough for a
// test to observe delivery.
func newFaultyEventHub(t *testing.T, db *sql.DB) *faultyEventHub {
	t.Helper()
	hub := eventhub.New(db, slog.Default(), eventhub.Config{
		PollInterval:    50 * time.Millisecond,
		CleanupInterval: time.Hour,
		RetentionPeriod: time.Hour,
	})
	if err := hub.Initialize(); err != nil {
		t.Fatalf("initialize event hub: %v", err)
	}
	// Registered after the DB's own cleanup, so it runs first: the pollers stop
	// before the DB they poll is closed.
	t.Cleanup(func() { _ = hub.Close() })
	return &faultyEventHub{EventHub: hub}
}

// recordedEvent is one persisted EventHub row, with the gateway event inside it
// decoded.
type recordedEvent struct {
	GatewayID  string
	EntityType string
	Action     string
	Type       string
	Payload    map[string]any
}

// recordedEvents returns the events persisted for gatewayUUID, oldest first.
func (e *agentDeployEnv) recordedEvents(t *testing.T, gatewayUUID string) []recordedEvent {
	t.Helper()
	rows, err := e.db.Query(`SELECT gateway_id, entity_type, action, event_data FROM events
		WHERE gateway_id = ? ORDER BY processed_timestamp, rowid`, gatewayUUID)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	defer rows.Close()

	var out []recordedEvent
	for rows.Next() {
		var ev recordedEvent
		var data string
		if err := rows.Scan(&ev.GatewayID, &ev.EntityType, &ev.Action, &data); err != nil {
			t.Fatalf("scan event: %v", err)
		}
		ev.Type, ev.Payload = decodeGatewayEvent(t, data)
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate events: %v", err)
	}
	return out
}

func decodeGatewayEvent(t *testing.T, data string) (string, map[string]any) {
	t.Helper()
	var envelope struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		t.Fatalf("event data is not a gateway event: %v (%s)", err, data)
	}
	return envelope.Type, envelope.Payload
}

// lastEvent returns the most recent event persisted for gatewayUUID.
func (e *agentDeployEnv) lastEvent(t *testing.T, gatewayUUID string) recordedEvent {
	t.Helper()
	events := e.recordedEvents(t, gatewayUUID)
	if len(events) == 0 {
		t.Fatalf("no event recorded for gateway %s", gatewayUUID)
	}
	return events[len(events)-1]
}

func (e *agentDeployEnv) agentUUID(t *testing.T) string {
	t.Helper()
	var id string
	if err := e.db.QueryRow(`SELECT uuid FROM agent_proxies WHERE handle = ? AND organization_uuid = ?`,
		e.proxy, agentProxyOrg).Scan(&id); err != nil {
		t.Fatalf("resolve agent proxy uuid: %v", err)
	}
	return id
}

func (e *agentDeployEnv) eventCount(t *testing.T) int {
	t.Helper()
	return e.countRows(t, `SELECT COUNT(*) FROM events`)
}

// assertAgentEvent checks one recorded Agent event: its type, the EventHub
// action derived from that type, and a payload carrying identifiers only.
func assertAgentEvent(t *testing.T, ev recordedEvent, wantType, wantAction string, wantPayload map[string]any) {
	t.Helper()
	if ev.EntityType != "PLATFORM_GATEWAY_EVENT" {
		t.Fatalf("entity_type = %q, want PLATFORM_GATEWAY_EVENT", ev.EntityType)
	}
	if ev.Type != wantType {
		t.Fatalf("event type = %q, want %q", ev.Type, wantType)
	}
	if ev.Action != wantAction {
		t.Fatalf("%s: persisted action = %q, want %q", wantType, ev.Action, wantAction)
	}
	gotKeys := make([]string, 0, len(ev.Payload))
	for k := range ev.Payload {
		gotKeys = append(gotKeys, k)
	}
	wantKeys := make([]string, 0, len(wantPayload))
	for k := range wantPayload {
		wantKeys = append(wantKeys, k)
	}
	sort.Strings(gotKeys)
	sort.Strings(wantKeys)
	if strings.Join(gotKeys, ",") != strings.Join(wantKeys, ",") {
		t.Fatalf("%s payload keys = %v, want exactly %v — the event carries ids only", wantType, gotKeys, wantKeys)
	}
	for k, want := range wantPayload {
		if want == nil {
			continue // present, value checked elsewhere
		}
		if ev.Payload[k] != want {
			t.Fatalf("%s payload %s = %v, want %v", wantType, k, ev.Payload[k], want)
		}
	}
}

// statusRow reads the current deployment_status for the env's gateway.
func (e *agentDeployEnv) statusRow(t *testing.T) (deploymentID, status, reason string) {
	t.Helper()
	var r sql.NullString
	if err := e.db.QueryRow(`SELECT deployment_uuid, status, status_reason FROM deployment_status WHERE gateway_uuid = ?`,
		e.gatewayUUID).Scan(&deploymentID, &status, &r); err != nil {
		t.Fatalf("read deployment status: %v", err)
	}
	return deploymentID, status, r.String
}

func TestAgentProxyGatewayEvents_EachActionRecordsItsEvent(t *testing.T) {
	env := setupAgentDeployEnv(t)
	agentUUID := env.agentUUID(t)
	if agentUUID == env.proxy {
		t.Fatal("test precondition: the artifact UUID must differ from the public handle")
	}

	deploymentID := env.deploy(t, env.gateway)
	deployed := env.lastEvent(t, env.gatewayUUID)
	assertAgentEvent(t, deployed, "agent.deployed", "CREATE",
		map[string]any{"proxyId": agentUUID, "deploymentId": deploymentID, "performedAt": nil})
	var performedAt time.Time
	if err := env.db.QueryRow(`SELECT performed_at FROM deployment_status WHERE gateway_uuid = ?`,
		env.gatewayUUID).Scan(&performedAt); err != nil {
		t.Fatalf("read performed_at: %v", err)
	}
	gotPerformedAt, err := time.Parse(time.RFC3339Nano, deployed.Payload["performedAt"].(string))
	if err != nil || !gotPerformedAt.Equal(performedAt) {
		t.Fatalf("event performedAt = %v, want the status row's concurrency token %v", deployed.Payload["performedAt"], performedAt)
	}

	itemPath := env.deploymentsPath() + "/" + deploymentID
	env.ack(t, deploymentID, "DEPLOYED", "")
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost,
		itemPath+"/undeploy?gatewayId="+env.gateway, ""), http.StatusAccepted)
	assertAgentEvent(t, env.lastEvent(t, env.gatewayUUID), "agent.undeployed", "DELETE",
		map[string]any{"proxyId": agentUUID, "deploymentId": deploymentID, "performedAt": nil})

	env.ack(t, deploymentID, "UNDEPLOYED", "")
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost,
		itemPath+"/restore?gatewayId="+env.gateway, ""), http.StatusAccepted)
	// Restore is a deployment as far as the gateway is concerned.
	assertAgentEvent(t, env.lastEvent(t, env.gatewayUUID), "agent.deployed", "CREATE",
		map[string]any{"proxyId": agentUUID, "deploymentId": deploymentID, "performedAt": nil})
}

func TestAgentProxyGatewayEvents_DeletingADeploymentRecordSendsNothing(t *testing.T) {
	env := setupAgentDeployEnv(t)
	deploymentID := env.deploy(t, env.gateway)
	env.ack(t, deploymentID, "UNDEPLOYED", "")

	before := env.eventCount(t)
	rec := callAgentProxy(t, env.handler, http.MethodDelete, env.deploymentsPath()+"/"+deploymentID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete undeployed record = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if after := env.eventCount(t); after != before {
		t.Fatalf("deleting a deployment record recorded %d event(s); it removes a record, not the Agent proxy", after-before)
	}
}

func TestAgentProxyGatewayEvents_AgentDeletionNotifiesEveryGatewayInTheOrganization(t *testing.T) {
	env := setupAgentDeployEnv(t)
	agentUUID := env.agentUUID(t)
	env.deploy(t, env.gateway)
	// A gateway this Agent proxy was never deployed to still hears the deletion;
	// one in another organization does not.
	idle := seedAgentGateway(t, env.db, agentProxyOrg, "idle-gw", "1.2.0")
	foreign := seedAgentGateway(t, env.db, agentProxyOtherOrg, "foreign-gw", "1.2.0")

	rec := callAgentProxy(t, env.handler, http.MethodDelete, agentProxyBase+"/"+env.proxy, "")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete = %d with %d body bytes, want bodyless 204", rec.Code, rec.Body.Len())
	}

	for _, gw := range []string{env.gatewayUUID, idle} {
		assertAgentEvent(t, env.lastEvent(t, gw), "agent.deleted", "DELETE", map[string]any{"proxyId": agentUUID})
	}
	if n := len(env.recordedEvents(t, foreign)); n != 0 {
		t.Fatalf("a gateway in another organization received %d event(s)", n)
	}
	// The recorded event outlives the row it describes.
	if n := env.countRows(t, `SELECT COUNT(*) FROM agent_proxies WHERE uuid = ?`, agentUUID); n != 0 {
		t.Fatalf("Agent proxy row still present after delete")
	}
}

// receiveGatewayEvent waits for the next event delivered on ch.
func receiveGatewayEvent(t *testing.T, ch <-chan eventhub.Event) (string, map[string]any) {
	t.Helper()
	select {
	case ev := <-ch:
		return decodeGatewayEvent(t, ev.EventData)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the gateway event to be delivered")
		return "", nil
	}
}

func TestAgentProxyGatewayEvents_ReachAConnectedGateway(t *testing.T) {
	env := setupAgentDeployEnv(t)
	agentUUID := env.agentUUID(t)
	if err := env.hub.RegisterGateway(env.gatewayUUID); err != nil {
		t.Fatalf("register gateway: %v", err)
	}
	ch, err := env.hub.Subscribe(env.gatewayUUID)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	deploymentID := env.deploy(t, env.gateway)
	typ, payload := receiveGatewayEvent(t, ch)
	if typ != "agent.deployed" || payload["proxyId"] != agentUUID || payload["deploymentId"] != deploymentID {
		t.Fatalf("delivered %s %v, want agent.deployed for %s/%s", typ, payload, agentUUID, deploymentID)
	}

	env.ack(t, deploymentID, "DEPLOYED", "")
	decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost,
		env.deploymentsPath()+"/"+deploymentID+"/undeploy?gatewayId="+env.gateway, ""), http.StatusAccepted)
	if typ, payload = receiveGatewayEvent(t, ch); typ != "agent.undeployed" || payload["proxyId"] != agentUUID {
		t.Fatalf("delivered %s %v, want agent.undeployed for %s", typ, payload, agentUUID)
	}

	if rec := callAgentProxy(t, env.handler, http.MethodDelete, agentProxyBase+"/"+env.proxy, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204", rec.Code)
	}
	if typ, payload = receiveGatewayEvent(t, ch); typ != "agent.deleted" || payload["proxyId"] != agentUUID {
		t.Fatalf("delivered %s %v, want agent.deleted for %s", typ, payload, agentUUID)
	}
}

func TestAgentProxyGatewayEvents_PublishedWhileDisconnectedAreDeliveredOnConnect(t *testing.T) {
	env := setupAgentDeployEnv(t)
	agentUUID := env.agentUUID(t)

	// Nothing is connected: deploy, then delete the whole Agent proxy, so the
	// deletion is delivered after the row it names is already gone.
	env.deploy(t, env.gateway)
	if rec := callAgentProxy(t, env.handler, http.MethodDelete, agentProxyBase+"/"+env.proxy, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204", rec.Code)
	}

	// Connect the way a gateway WebSocket does: register, then subscribe.
	if err := env.hub.RegisterGateway(env.gatewayUUID); err != nil {
		t.Fatalf("register gateway: %v", err)
	}
	ch, err := env.hub.Subscribe(env.gatewayUUID)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	var types []string
	for range 2 {
		typ, payload := receiveGatewayEvent(t, ch)
		if payload["proxyId"] != agentUUID {
			t.Fatalf("delivered %s for %v, want %s", typ, payload["proxyId"], agentUUID)
		}
		types = append(types, typ)
	}
	if strings.Join(types, ",") != "agent.deployed,agent.deleted" {
		t.Fatalf("delivered %v on connect, want the deployment then the deletion", types)
	}
}

// A failed publish follows the other kinds: it is logged, and the mutation is
// still reported as the success it is in the control plane. The gateway catches
// up at its next reconnect sync; a transitional status left unacknowledged is
// resolved by the deployment timeout job.

func TestAgentProxyGatewayEvents_DeployWithAFailedPublishStillSucceeds(t *testing.T) {
	env := setupAgentDeployEnv(t)
	env.hub.failPublish.Store(true)

	rec := callAgentProxy(t, env.handler, http.MethodPost, env.deploymentsPath(), deployBody("dep", env.gateway))
	body := decodeAgentProxyJSON(t, rec, http.StatusCreated)
	if body["status"] != "DEPLOYING" {
		t.Fatalf("status = %v, want DEPLOYING", body["status"])
	}
	if n := env.eventCount(t); n != 0 {
		t.Fatalf("%d event(s) recorded despite the failure", n)
	}
	if _, status, _ := env.statusRow(t); status != "DEPLOYING" {
		t.Fatalf("status row = %s, want DEPLOYING", status)
	}
}

func TestAgentProxyGatewayEvents_UndeployAndRestoreWithAFailedPublishStillSucceed(t *testing.T) {
	for _, tc := range []struct {
		action     string
		priorState string
		wantStatus string
	}{
		{"undeploy", "DEPLOYED", "UNDEPLOYING"},
		{"restore", "UNDEPLOYED", "DEPLOYING"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			env := setupAgentDeployEnv(t)
			deploymentID := env.deploy(t, env.gateway)
			env.ack(t, deploymentID, tc.priorState, "")
			before := env.eventCount(t)

			env.hub.failPublish.Store(true)
			body := decodeAgentProxyJSON(t, callAgentProxy(t, env.handler, http.MethodPost,
				env.deploymentsPath()+"/"+deploymentID+"/"+tc.action+"?gatewayId="+env.gateway, ""), http.StatusAccepted)
			if body["status"] != tc.wantStatus {
				t.Fatalf("status = %v, want %s", body["status"], tc.wantStatus)
			}
			if after := env.eventCount(t); after != before {
				t.Fatalf("%d event(s) recorded despite the failure", after-before)
			}
			if _, status, _ := env.statusRow(t); status != tc.wantStatus {
				t.Fatalf("status row = %s, want %s", status, tc.wantStatus)
			}
		})
	}
}

func TestAgentProxyGatewayEvents_DeletionWithAFailedPublishStillDeletes(t *testing.T) {
	env := setupAgentDeployEnv(t)
	agentUUID := env.agentUUID(t)
	env.deploy(t, env.gateway)
	before := env.eventCount(t)

	env.hub.failPublish.Store(true)
	rec := callAgentProxy(t, env.handler, http.MethodDelete, agentProxyBase+"/"+env.proxy, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if after := env.eventCount(t); after != before {
		t.Fatalf("%d event(s) recorded despite the failure", after-before)
	}
	if n := env.countRows(t, `SELECT COUNT(*) FROM agent_proxies WHERE uuid = ?`, agentUUID); n != 0 {
		t.Fatalf("Agent proxy row still present after delete")
	}
}

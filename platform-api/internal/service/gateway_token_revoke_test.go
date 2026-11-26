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
	"sync"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	ws "github.com/wso2/api-platform/platform-api/internal/websocket"
)

const (
	revokeTestGatewayID = "gw-uuid"
	revokedTokenID      = "token-revoked"
	activeTokenID       = "token-active"
)

// recordingTransport records what is sent on a gateway session and whether it was closed.
type recordingTransport struct {
	mu        sync.Mutex
	sent      int
	closeCode int
}

func (t *recordingTransport) Send([]byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sent++
	return nil
}

func (t *recordingTransport) Close(code int, _ string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeCode = code
	return nil
}

func (t *recordingTransport) SetReadDeadline(time.Time) error      { return nil }
func (t *recordingTransport) SetWriteDeadline(time.Time) error     { return nil }
func (t *recordingTransport) EnablePongHandler(func(string) error) {}
func (t *recordingTransport) SendPing() error                      { return nil }

func (t *recordingTransport) stats() (sent, closeCode int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sent, t.closeCode
}

// revokeTokenRepo serves the one token RevokeToken looks up and records the revoke.
type revokeTokenRepo struct {
	mockGatewayRepository
	token   *model.GatewayToken
	revoked string
}

func (r *revokeTokenRepo) GetTokenByUUID(string) (*model.GatewayToken, error) { return r.token, nil }
func (r *revokeTokenRepo) RevokeToken(tokenID, _ string) error {
	r.revoked = tokenID
	return nil
}

func newRevokeTestManager(t *testing.T) *ws.Manager {
	t.Helper()
	cfg := ws.DefaultManagerConfig()
	cfg.MetricsLogEnabled = false
	cfg.HeartbeatInterval = time.Hour
	cfg.HeartbeatTimeout = time.Hour
	m := ws.NewManager(cfg, newTestLogger())
	t.Cleanup(m.Shutdown)
	return m
}

// registerSession opens a gateway session authenticated with tokenID.
func registerSession(t *testing.T, m *ws.Manager, tokenID string) (*ws.Connection, *recordingTransport) {
	t.Helper()
	transport := &recordingTransport{}
	conn, err := m.Register(revokeTestGatewayID, transport, tokenID, "org-1")
	if err != nil {
		t.Fatalf("Register(%s) = %v", tokenID, err)
	}
	return conn, transport
}

// TestRevokeTokenClosesOnlyItsLocalSessions: once RevokeToken returns, the session this
// replica holds for the revoked token is closed, the session of the gateway's other
// active token is untouched, and the cross-replica signal is published for the token.
func TestRevokeTokenClosesOnlyItsLocalSessions(t *testing.T) {
	manager := newRevokeTestManager(t)
	revokedConn, revokedTransport := registerSession(t, manager, revokedTokenID)
	activeConn, activeTransport := registerSession(t, manager, activeTokenID)

	hub := &capturingEventHub{}
	repo := &revokeTokenRepo{
		mockGatewayRepository: mockGatewayRepository{getByNameResult: &model.Gateway{ID: revokeTestGatewayID}},
		token:                 &model.GatewayToken{ID: revokedTokenID, GatewayID: revokeTestGatewayID, Status: constants.GatewayTokenStatusActive},
	}
	svc := &GatewayService{
		gatewayRepo:          repo,
		gatewayEventsService: NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger()),
		slogger:              newTestLogger(),
	}
	svc.SetConnectionManager(manager)

	if err := svc.RevokeToken("gw-handle", revokedTokenID, "org-1", "admin"); err != nil {
		t.Fatalf("RevokeToken = %v", err)
	}

	if repo.revoked != revokedTokenID {
		t.Fatalf("revoked token = %q, want %q", repo.revoked, revokedTokenID)
	}
	if !revokedConn.IsClosed() {
		t.Fatal("session opened with the revoked token is still open after RevokeToken returned")
	}
	if _, code := revokedTransport.stats(); code != 1008 {
		t.Errorf("revoked session close code = %d, want 1008", code)
	}
	if activeConn.IsClosed() {
		t.Fatal("session opened with another active token was closed")
	}
	if _, code := activeTransport.stats(); code != 0 {
		t.Errorf("active-token session got close code %d, want none", code)
	}
	if conns := manager.GetConnections(revokeTestGatewayID); len(conns) != 1 || conns[0] != activeConn {
		t.Fatalf("remaining connections = %v, want only the active-token session", conns)
	}

	if len(hub.published) != 1 {
		t.Fatalf("published %d events, want 1 token-revoked signal", len(hub.published))
	}
	signal := hub.published[0]
	if signal.EventType != eventTypeGatewayTokenRevoked || signal.EntityID != revokedTokenID || signal.GatewayID != revokeTestGatewayID {
		t.Fatalf("signal = %+v, want a %s event for token %s of gateway %s",
			signal, eventTypeGatewayTokenRevoked, revokedTokenID, revokeTestGatewayID)
	}
}

// TestEventDispatcherClosesRevokedTokenSessions covers the cross-replica path: a replica
// that did not serve the revoke request receives the signal through the EventHub and
// closes its session for the revoked token. The signal is not forwarded to the gateway,
// and an event published after it reaches only the session of the still-active token.
func TestEventDispatcherClosesRevokedTokenSessions(t *testing.T) {
	// The signal exactly as the revoking replica publishes it.
	hub := &capturingEventHub{}
	events := NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger())
	if err := events.PublishGatewayTokenRevoked(revokeTestGatewayID, revokedTokenID); err != nil {
		t.Fatalf("PublishGatewayTokenRevoked = %v", err)
	}
	if err := events.BroadcastAPIKeyCreatedEvent(revokeTestGatewayID, "", &model.APIKeyCreatedEvent{}); err != nil {
		t.Fatalf("BroadcastAPIKeyCreatedEvent = %v", err)
	}
	signal, keyEvent := hub.published[0], hub.published[1]

	// The other replica holds both sessions of the gateway.
	manager := newRevokeTestManager(t)
	revokedConn, revokedTransport := registerSession(t, manager, revokedTokenID)
	activeConn, activeTransport := registerSession(t, manager, activeTokenID)
	dispatcher := NewEventDispatcher(&capturingEventHub{}, manager, newTestLogger())
	t.Cleanup(dispatcher.Shutdown)

	dispatcher.deliver(revokeTestGatewayID, signal)
	dispatcher.deliver(revokeTestGatewayID, keyEvent)

	if !revokedConn.IsClosed() {
		t.Fatal("session opened with the revoked token is still open after the signal")
	}
	if sent, code := revokedTransport.stats(); sent != 0 || code != 1008 {
		t.Errorf("revoked session: sent %d messages, close code %d; want 0 messages, 1008", sent, code)
	}
	if activeConn.IsClosed() {
		t.Fatal("session opened with another active token was closed by the signal")
	}
	if sent, _ := activeTransport.stats(); sent != 1 {
		t.Errorf("active-token session got %d messages, want only the API key event (signal must not be forwarded)", sent)
	}
}

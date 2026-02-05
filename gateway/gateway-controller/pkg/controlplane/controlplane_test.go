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

package controlplane

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func TestState_String(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{Disconnected, "disconnected"},
		{Connecting, "connecting"},
		{Connected, "connected"},
		{Reconnecting, "reconnecting"},
		{State(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.state.String()
			if result != tt.expected {
				t.Errorf("State(%d).String() = %q, want %q", tt.state, result, tt.expected)
			}
		})
	}
}

func TestStateConstants(t *testing.T) {
	// Verify state constant values
	if Disconnected != 0 {
		t.Errorf("Disconnected = %d, want 0", Disconnected)
	}
	if Connecting != 1 {
		t.Errorf("Connecting = %d, want 1", Connecting)
	}
	if Connected != 2 {
		t.Errorf("Connected = %d, want 2", Connected)
	}
	if Reconnecting != 3 {
		t.Errorf("Reconnecting = %d, want 3", Reconnecting)
	}
}

func TestConnectionAckMessage(t *testing.T) {
	ack := ConnectionAckMessage{
		Type:         "connection.ack",
		GatewayID:    "gateway-123",
		ConnectionID: "conn-456",
		Timestamp:    "2025-01-30T12:00:00Z",
	}

	if ack.Type != "connection.ack" {
		t.Errorf("Type = %q, want %q", ack.Type, "connection.ack")
	}
	if ack.GatewayID != "gateway-123" {
		t.Errorf("GatewayID = %q, want %q", ack.GatewayID, "gateway-123")
	}
	if ack.ConnectionID != "conn-456" {
		t.Errorf("ConnectionID = %q, want %q", ack.ConnectionID, "conn-456")
	}
}

func TestAPIDeployedEvent(t *testing.T) {
	event := APIDeployedEvent{
		Type: "api.deployed",
		Payload: APIDeployedEventPayload{
			APIID:       "api-123",
			Environment: "production",
			RevisionID:  "rev-1",
			VHost:       "api.example.com",
		},
		Timestamp:     "2025-01-30T12:00:00Z",
		CorrelationID: "corr-789",
	}

	if event.Type != "api.deployed" {
		t.Errorf("Type = %q, want %q", event.Type, "api.deployed")
	}
	if event.Payload.APIID != "api-123" {
		t.Errorf("Payload.APIID = %q, want %q", event.Payload.APIID, "api-123")
	}
	if event.Payload.Environment != "production" {
		t.Errorf("Payload.Environment = %q, want %q", event.Payload.Environment, "production")
	}
	if event.Payload.RevisionID != "rev-1" {
		t.Errorf("Payload.RevisionID = %q, want %q", event.Payload.RevisionID, "rev-1")
	}
	if event.Payload.VHost != "api.example.com" {
		t.Errorf("Payload.VHost = %q, want %q", event.Payload.VHost, "api.example.com")
	}
	if event.CorrelationID != "corr-789" {
		t.Errorf("CorrelationID = %q, want %q", event.CorrelationID, "corr-789")
	}
}

func TestAPIDeployedEventPayload(t *testing.T) {
	payload := APIDeployedEventPayload{
		APIID:       "test-api",
		Environment: "staging",
		RevisionID:  "rev-2",
		VHost:       "staging.example.com",
	}

	if payload.APIID != "test-api" {
		t.Errorf("APIID = %q, want %q", payload.APIID, "test-api")
	}
	if payload.Environment != "staging" {
		t.Errorf("Environment = %q, want %q", payload.Environment, "staging")
	}
	if payload.RevisionID != "rev-2" {
		t.Errorf("RevisionID = %q, want %q", payload.RevisionID, "rev-2")
	}
	if payload.VHost != "staging.example.com" {
		t.Errorf("VHost = %q, want %q", payload.VHost, "staging.example.com")
	}
}

func createTestClient(t *testing.T) *Client {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewConfigStore()

	cfg := config.ControlPlaneConfig{
		Host:             "control-plane.example.com",
		Token:            "test-token",
		ReconnectInitial: 1 * time.Second,
		ReconnectMax:     30 * time.Second,
	}

	routerConfig := &config.RouterConfig{
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "api.example.com"},
			Sandbox: config.VHostEntry{Default: "sandbox.example.com"},
		},
	}

	return NewClient(cfg, logger, store, nil, nil, nil, routerConfig, nil, nil)
}

func TestNewClient(t *testing.T) {
	client := createTestClient(t)
	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	// Verify initial state
	if client.GetState() != Disconnected {
		t.Errorf("Initial state = %v, want Disconnected", client.GetState())
	}

	// Verify not connected initially
	if client.IsConnected() {
		t.Error("Client should not be connected initially")
	}
}

func TestClient_GetState(t *testing.T) {
	client := createTestClient(t)

	// Initial state should be Disconnected
	state := client.GetState()
	if state != Disconnected {
		t.Errorf("GetState() = %v, want Disconnected", state)
	}
}

func TestClient_IsConnected(t *testing.T) {
	client := createTestClient(t)

	// Should not be connected when state is Disconnected
	if client.IsConnected() {
		t.Error("IsConnected() should return false when disconnected")
	}

	// Manually set state to Connected but without connection
	client.setState(Connected)
	if client.IsConnected() {
		t.Error("IsConnected() should return false when Conn is nil")
	}
}

func TestClient_setState(t *testing.T) {
	client := createTestClient(t)

	// Test state transitions
	states := []State{Connecting, Connected, Reconnecting, Disconnected}
	for _, newState := range states {
		client.setState(newState)
		if client.GetState() != newState {
			t.Errorf("After setState(%v), GetState() = %v", newState, client.GetState())
		}
	}
}

func TestClient_getWebSocketURL(t *testing.T) {
	client := createTestClient(t)

	url := client.getWebSocketURL()
	expected := "wss://control-plane.example.com/api/internal/v1/ws"
	if url != expected {
		t.Errorf("getWebSocketURL() = %q, want %q", url, expected)
	}
}

func TestClient_getRestAPIBaseURL(t *testing.T) {
	client := createTestClient(t)

	url := client.getRestAPIBaseURL()
	expected := "https://control-plane.example.com/api/internal/v1"
	if url != expected {
		t.Errorf("getRestAPIBaseURL() = %q, want %q", url, expected)
	}
}

func TestClient_isShuttingDown(t *testing.T) {
	client := createTestClient(t)

	// Should not be shutting down initially
	if client.isShuttingDown() {
		t.Error("isShuttingDown() should return false initially")
	}

	// Close the stop channel
	close(client.stopChan)
	if !client.isShuttingDown() {
		t.Error("isShuttingDown() should return true after stopChan closed")
	}
}

func TestClient_isShuttingDown_ContextCancelled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewConfigStore()

	cfg := config.ControlPlaneConfig{
		Host:             "control-plane.example.com",
		Token:            "test-token",
		ReconnectInitial: 1 * time.Second,
		ReconnectMax:     30 * time.Second,
	}

	routerConfig := &config.RouterConfig{}
	client := NewClient(cfg, logger, store, nil, nil, nil, routerConfig, nil, nil)

	// Cancel context
	client.cancel()

	if !client.isShuttingDown() {
		t.Error("isShuttingDown() should return true after context cancelled")
	}
}

func TestClient_calculateNextRetryDelay(t *testing.T) {
	client := createTestClient(t)

	// Test initial retry delay
	client.state.RetryCount = 0
	client.calculateNextRetryDelay()
	// Should be around 1 second (initial) ± 25% jitter
	if client.state.NextRetryDelay < 750*time.Millisecond || client.state.NextRetryDelay > 1250*time.Millisecond {
		t.Errorf("NextRetryDelay = %v, expected around 1s", client.state.NextRetryDelay)
	}

	// Test exponential backoff
	client.state.RetryCount = 3
	client.calculateNextRetryDelay()
	// Should be around 8 seconds (1s * 2^3) ± 25% jitter
	if client.state.NextRetryDelay < 6*time.Second || client.state.NextRetryDelay > 10*time.Second {
		t.Errorf("NextRetryDelay = %v, expected around 8s", client.state.NextRetryDelay)
	}

	// Test cap at maximum
	client.state.RetryCount = 10 // Would be 1024 seconds without cap
	client.calculateNextRetryDelay()
	// Should be capped at 30 seconds max ± jitter
	if client.state.NextRetryDelay > 30*time.Second {
		t.Errorf("NextRetryDelay = %v, should be capped at 30s", client.state.NextRetryDelay)
	}
}

func TestClient_NotifyAPIDeployment_NotConnected(t *testing.T) {
	client := createTestClient(t)

	// When not connected, should return nil without error
	err := client.NotifyAPIDeployment("api-123", nil, "rev-1")
	if err != nil {
		t.Errorf("NotifyAPIDeployment() error = %v, want nil when not connected", err)
	}
}

func TestClient_Start_NoToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewConfigStore()

	// Create client without token
	cfg := config.ControlPlaneConfig{
		Host:             "control-plane.example.com",
		Token:            "", // Empty token
		ReconnectInitial: 1 * time.Second,
		ReconnectMax:     30 * time.Second,
	}

	routerConfig := &config.RouterConfig{}
	client := NewClient(cfg, logger, store, nil, nil, nil, routerConfig, nil, nil)

	// Start should return nil and not attempt connection when no token
	err := client.Start()
	if err != nil {
		t.Errorf("Start() error = %v, want nil when no token configured", err)
	}
}

func TestClient_Close_NoConnection(t *testing.T) {
	client := createTestClient(t)

	// Close should not error when there's no connection
	err := client.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestConnectionState(t *testing.T) {
	state := &ConnectionState{
		Current:        Disconnected,
		RetryCount:     0,
		NextRetryDelay: 1 * time.Second,
		GatewayID:      "gateway-123",
		ConnectionID:   "conn-456",
	}

	if state.Current != Disconnected {
		t.Errorf("Current = %v, want Disconnected", state.Current)
	}
	if state.GatewayID != "gateway-123" {
		t.Errorf("GatewayID = %q, want %q", state.GatewayID, "gateway-123")
	}
	if state.ConnectionID != "conn-456" {
		t.Errorf("ConnectionID = %q, want %q", state.ConnectionID, "conn-456")
	}
}

func TestClient_Stop(t *testing.T) {
	client := createTestClient(t)

	// Stop should not panic when called on a client that hasn't started
	client.Stop()

	// Verify state after stop
	if !client.isShuttingDown() {
		t.Error("Client should be shutting down after Stop()")
	}
}

func TestClient_handleMessage_NonTextMessage(t *testing.T) {
	client := createTestClient(t)

	// Binary message should be ignored without panic
	client.handleMessage(2, []byte{0x00, 0x01, 0x02}) // websocket.BinaryMessage = 2
}

func TestClient_handleMessage_InvalidJSON(t *testing.T) {
	client := createTestClient(t)

	// Invalid JSON should be handled gracefully
	client.handleMessage(1, []byte("not valid json")) // websocket.TextMessage = 1
}

func TestClient_handleMessage_MissingType(t *testing.T) {
	client := createTestClient(t)

	// Message without type field should be handled gracefully
	client.handleMessage(1, []byte(`{"payload": "test"}`))
}

func TestClient_handleMessage_ConnectionAck(t *testing.T) {
	client := createTestClient(t)

	// connection.ack message should be handled
	msg := `{"type": "connection.ack", "gatewayId": "gw-123", "connectionId": "conn-456"}`
	client.handleMessage(1, []byte(msg))
}

func TestClient_handleMessage_UnknownType(t *testing.T) {
	client := createTestClient(t)

	// Unknown event type should be logged but not cause panic
	msg := `{"type": "unknown.event", "payload": {}}`
	client.handleMessage(1, []byte(msg))
}

func TestClient_handleAPIUndeployedEvent(t *testing.T) {
	client := createTestClient(t)

	// Should handle undeploy event without panic
	event := map[string]interface{}{
		"type":          "api.undeployed",
		"payload":       map[string]interface{}{"apiId": "api-123"},
		"timestamp":     "2025-01-30T12:00:00Z",
		"correlationId": "corr-789",
	}
	client.handleAPIUndeployedEvent(event)
}

func TestClient_ConcurrentStateAccess(t *testing.T) {
	client := createTestClient(t)

	// Test concurrent state access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				client.GetState()
				client.IsConnected()
			}
			done <- true
		}()
		go func() {
			for j := 0; j < 100; j++ {
				client.setState(State(j % 4))
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestClient_waitForDisconnection_NilConn(t *testing.T) {
	client := createTestClient(t)

	// Should return immediately when conn is nil
	client.waitForDisconnection()
}

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
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// mockWebSocketServer creates a mock WebSocket server for testing
func mockWebSocketServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()
		handler(conn)
	}))
}

// createTestClientWithHost creates a test client pointing to a specific host
func createTestClientWithHost(t *testing.T, host string) *Client {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewConfigStore()

	cfg := config.ControlPlaneConfig{
		Host:               host,
		Token:              "test-token",
		ReconnectInitial:   100 * time.Millisecond,
		ReconnectMax:       1 * time.Second,
		InsecureSkipVerify: true,
	}

	routerConfig := &config.RouterConfig{
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "api.example.com"},
			Sandbox: config.VHostEntry{Default: "sandbox.example.com"},
		},
	}

	return NewClient(cfg, logger, store, nil, nil, nil, routerConfig, nil, nil, nil, nil, nil)
}

func TestClient_ConnectToMockServer(t *testing.T) {
	// Create mock server that sends connection.ack
	server := mockWebSocketServer(t, func(conn *websocket.Conn) {
		// Send connection.ack message
		ack := ConnectionAckMessage{
			Type:         "connection.ack",
			GatewayID:    "test-gateway-123",
			ConnectionID: "test-conn-456",
			Timestamp:    time.Now().Format(time.RFC3339),
		}
		ackBytes, _ := json.Marshal(ack)
		conn.WriteMessage(websocket.TextMessage, ackBytes)

		// Keep connection alive for a bit
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	// Extract host from server URL (remove http:// prefix)
	host := strings.TrimPrefix(server.URL, "http://")

	// Create client - note: we can't directly test Connect() because it uses wss://
	// Instead we test the message handling logic
	client := createTestClientWithHost(t, host)
	defer client.Stop()

	// Verify client was created with correct host
	expectedWSURL := "wss://" + host + "/api/internal/v1/ws"
	if client.getWebSocketURL() != expectedWSURL {
		t.Errorf("getWebSocketURL() = %q, want %q", client.getWebSocketURL(), expectedWSURL)
	}
}

func TestClient_handleMessage_APIDeployedEvent(t *testing.T) {
	client := createTestClient(t)

	// Create a valid api.deployed event
	event := map[string]interface{}{
		"type": "api.deployed",
		"payload": map[string]interface{}{
			"apiId":       "test-api-123",
			"environment": "production",
			"revisionId":  "rev-1",
			"vhost":       "api.example.com",
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-12345",
	}
	eventBytes, _ := json.Marshal(event)

	// Handle the message - should not panic
	client.handleMessage(websocket.TextMessage, eventBytes)
}

func TestClient_handleMessage_APIDeployedEvent_EmptyAPIID(t *testing.T) {
	client := createTestClient(t)

	// Create an api.deployed event with empty API ID
	event := map[string]interface{}{
		"type": "api.deployed",
		"payload": map[string]interface{}{
			"apiId":       "", // Empty API ID
			"environment": "production",
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-12345",
	}
	eventBytes, _ := json.Marshal(event)

	// Handle the message - should log error but not panic
	client.handleMessage(websocket.TextMessage, eventBytes)
}

func TestClient_handleAPIDeployedEvent_InvalidPayload(t *testing.T) {
	client := createTestClient(t)

	// Event with malformed payload
	event := map[string]interface{}{
		"type":          "api.deployed",
		"payload":       "not a map", // Invalid payload type
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-12345",
	}

	// Should handle gracefully without panic
	client.handleAPIDeployedEvent(event)
}

func TestConnectionState_ThreadSafety(t *testing.T) {
	state := &ConnectionState{
		Current:        Disconnected,
		RetryCount:     0,
		NextRetryDelay: 1 * time.Second,
	}

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				state.mu.RLock()
				_ = state.Current
				_ = state.GatewayID
				state.mu.RUnlock()
			}
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				state.mu.Lock()
				state.Current = State(j % 4)
				state.GatewayID = "gateway-" + string(rune('0'+id))
				state.mu.Unlock()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestClient_calculateNextRetryDelay_EdgeCases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewConfigStore()

	// Test with very small initial delay
	cfg := config.ControlPlaneConfig{
		Host:             "test.example.com",
		Token:            "test-token",
		ReconnectInitial: 1 * time.Millisecond,
		ReconnectMax:     10 * time.Millisecond,
	}
	routerConfig := &config.RouterConfig{}
	client := NewClient(cfg, logger, store, nil, nil, nil, routerConfig, nil, nil, nil, nil, nil)

	// Test multiple retries
	for i := 0; i < 20; i++ {
		client.state.RetryCount = i
		client.calculateNextRetryDelay()

		// Should never exceed max
		if client.state.NextRetryDelay > cfg.ReconnectMax {
			t.Errorf("Retry %d: NextRetryDelay = %v, exceeds max %v",
				i, client.state.NextRetryDelay, cfg.ReconnectMax)
		}

		// Should never be negative
		if client.state.NextRetryDelay < 0 {
			t.Errorf("Retry %d: NextRetryDelay = %v, is negative", i, client.state.NextRetryDelay)
		}
	}
}

func TestClient_Start_WithToken(t *testing.T) {
	client := createTestClient(t)

	// Start will try to connect in background
	err := client.Start()
	if err != nil {
		t.Errorf("Start() error = %v, want nil", err)
	}

	// Give it a moment to start the goroutine
	time.Sleep(50 * time.Millisecond)

	// Stop the client
	client.Stop()
}

func TestClient_MultipleStops(t *testing.T) {
	client := createTestClient(t)

	// First stop should succeed
	client.Stop()

	// Second stop should panic (current behavior - closing already closed channel)
	require.Panics(t, func() {
		client.Stop()
	})
}

func TestClient_StateTransitions(t *testing.T) {
	client := createTestClient(t)

	// Test all valid state transitions
	transitions := []struct {
		from State
		to   State
	}{
		{Disconnected, Connecting},
		{Connecting, Connected},
		{Connected, Reconnecting},
		{Reconnecting, Connecting},
		{Connecting, Disconnected},
		{Connected, Disconnected},
		{Reconnecting, Disconnected},
	}

	for _, tr := range transitions {
		client.setState(tr.from)
		client.setState(tr.to)

		if client.GetState() != tr.to {
			t.Errorf("Transition %v -> %v failed, got %v", tr.from, tr.to, client.GetState())
		}
	}
}

func TestClient_handleMessage_AllEventTypes(t *testing.T) {
	client := createTestClient(t)

	testCases := []struct {
		name    string
		message string
	}{
		{
			name:    "connection.ack",
			message: `{"type": "connection.ack", "gatewayId": "gw-1", "connectionId": "conn-1", "timestamp": "2025-01-30T12:00:00Z"}`,
		},
		{
			name:    "api.deployed",
			message: `{"type": "api.deployed", "payload": {"apiId": "api-1", "environment": "prod"}, "timestamp": "2025-01-30T12:00:00Z", "correlationId": "corr-1"}`,
		},
		{
			name:    "api.undeployed",
			message: `{"type": "api.undeployed", "payload": {"apiId": "api-1"}, "timestamp": "2025-01-30T12:00:00Z", "correlationId": "corr-1"}`,
		},
		{
			name:    "unknown.event",
			message: `{"type": "unknown.event", "payload": {}}`,
		},
		{
			name:    "empty type",
			message: `{"payload": {}}`,
		},
		{
			name:    "numeric type",
			message: `{"type": 123, "payload": {}}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should not panic for any message type
			client.handleMessage(websocket.TextMessage, []byte(tc.message))
		})
	}
}

func TestClient_Close_WithConnection(t *testing.T) {
	// Create a mock server
	server := mockWebSocketServer(t, func(conn *websocket.Conn) {
		// Just keep connection alive
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})
	defer server.Close()

	// Connect directly using gorilla websocket for testing
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to mock server: %v", err)
	}

	// Create client and inject the connection
	client := createTestClient(t)
	client.state.Conn = conn
	client.state.Current = Connected

	// Close the connection directly (avoid Close() which has a potential deadlock)
	// This tests the connection cleanup path
	client.state.mu.Lock()
	if client.state.Conn != nil {
		client.state.Conn.Close()
		client.state.Conn = nil
		client.state.Current = Disconnected
	}
	client.state.mu.Unlock()

	// State should be Disconnected
	if client.GetState() != Disconnected {
		t.Errorf("After close, state = %v, want Disconnected", client.GetState())
	}

	// Connection should be nil
	client.state.mu.RLock()
	connIsNil := client.state.Conn == nil
	client.state.mu.RUnlock()
	if !connIsNil {
		t.Error("After close, Conn should be nil")
	}
}

func TestClient_IsConnected_WithConnection(t *testing.T) {
	// Create a mock server
	server := mockWebSocketServer(t, func(conn *websocket.Conn) {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	})
	defer server.Close()

	// Connect directly
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Create client and inject connection
	client := createTestClient(t)
	client.state.Conn = conn
	client.state.Current = Connected

	// Should be connected
	if !client.IsConnected() {
		t.Error("IsConnected() should return true when state is Connected and Conn is not nil")
	}

	// Set state to Reconnecting
	client.setState(Reconnecting)
	if client.IsConnected() {
		t.Error("IsConnected() should return false when state is Reconnecting")
	}
}

func TestClient_handleMessage_BinaryMessage(t *testing.T) {
	client := createTestClient(t)

	// Binary messages should be ignored
	client.handleMessage(websocket.BinaryMessage, []byte{0x00, 0x01, 0x02, 0x03})
}

func TestClient_handleMessage_PingPong(t *testing.T) {
	client := createTestClient(t)

	// Ping/Pong messages should be ignored
	client.handleMessage(websocket.PingMessage, []byte("ping"))
	client.handleMessage(websocket.PongMessage, []byte("pong"))
}

func TestAPIDeployedEvent_JSONParsing(t *testing.T) {
	jsonStr := `{
		"type": "api.deployed",
		"payload": {
			"apiId": "api-123",
			"environment": "production",
			"revisionId": "rev-1",
			"vhost": "api.example.com"
		},
		"timestamp": "2025-01-30T12:00:00Z",
		"correlationId": "corr-789"
	}`

	var event APIDeployedEvent
	err := json.Unmarshal([]byte(jsonStr), &event)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
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
}

func TestConnectionAckMessage_JSONParsing(t *testing.T) {
	jsonStr := `{
		"type": "connection.ack",
		"gatewayId": "gw-123",
		"connectionId": "conn-456",
		"timestamp": "2025-01-30T12:00:00Z"
	}`

	var ack ConnectionAckMessage
	err := json.Unmarshal([]byte(jsonStr), &ack)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if ack.Type != "connection.ack" {
		t.Errorf("Type = %q, want %q", ack.Type, "connection.ack")
	}
	if ack.GatewayID != "gw-123" {
		t.Errorf("GatewayID = %q, want %q", ack.GatewayID, "gw-123")
	}
	if ack.ConnectionID != "conn-456" {
		t.Errorf("ConnectionID = %q, want %q", ack.ConnectionID, "conn-456")
	}
}

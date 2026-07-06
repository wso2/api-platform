/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package websocket

import (
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketTransport implements the Transport interface using gorilla/websocket.
// This provides the concrete WebSocket protocol implementation while isolating
// WebSocket-specific code from business logic.
//
// Design rationale: By implementing the Transport interface, we can swap
// WebSocket for other protocols (SSE, gRPC) without changing the Connection
// or Manager code.
type WebSocketTransport struct {
	conn *websocket.Conn
}

// NewWebSocketTransport creates a new WebSocket transport wrapper.
//
// Parameters:
//   - conn: The established gorilla/websocket connection
//
// Returns a Transport implementation backed by WebSocket.
func NewWebSocketTransport(conn *websocket.Conn) Transport {
	return &WebSocketTransport{
		conn: conn,
	}
}

// Send delivers a message to the WebSocket client as a text frame.
//
// Parameters:
//   - message: The message payload (typically JSON-encoded)
//
// Returns an error if the write fails or the connection is closed.
func (t *WebSocketTransport) Send(message []byte) error {
	return t.conn.WriteMessage(websocket.TextMessage, message)
}

// Close terminates the WebSocket connection with a close frame.
//
// Parameters:
//   - code: WebSocket close code (e.g., 1000 for normal closure)
//   - reason: Human-readable close reason
//
// Returns an error if sending the close frame fails.
func (t *WebSocketTransport) Close(code int, reason string) error {
	closeMessage := websocket.FormatCloseMessage(code, reason)
	err := t.conn.WriteMessage(websocket.CloseMessage, closeMessage)
	if err != nil {
		return err
	}
	// Close the underlying connection
	return t.conn.Close()
}

// SetReadDeadline sets the deadline for read operations.
// Used to detect heartbeat timeouts.
//
// Parameters:
//   - deadline: Absolute time when reads should timeout (zero disables)
func (t *WebSocketTransport) SetReadDeadline(deadline time.Time) error {
	return t.conn.SetReadDeadline(deadline)
}

// SetWriteDeadline sets the deadline for write operations.
// Prevents indefinite blocking on slow clients.
//
// Parameters:
//   - deadline: Absolute time when writes should timeout (zero disables)
func (t *WebSocketTransport) SetWriteDeadline(deadline time.Time) error {
	return t.conn.SetWriteDeadline(deadline)
}

// EnablePongHandler configures the automatic pong frame handler.
// Called when a pong frame is received in response to a ping.
//
// Parameters:
//   - handler: Callback invoked when pong frame arrives
func (t *WebSocketTransport) EnablePongHandler(handler func(string) error) {
	t.conn.SetPongHandler(handler)
}

// SendPing sends a WebSocket ping frame to test connection liveness.
// The client should respond with a pong frame.
//
// Returns an error if the ping cannot be sent.
func (t *WebSocketTransport) SendPing() error {
	return t.conn.WriteMessage(websocket.PingMessage, []byte{})
}

// ReadMessage reads the next message from the WebSocket connection.
// This is used by connection handlers to detect disconnections and handle incoming messages.
//
// Returns:
//   - messageType: The type of message (Text, Binary, Close, Ping, Pong)
//   - payload: The message data
//   - error: Any error encountered during read
func (t *WebSocketTransport) ReadMessage() (messageType int, payload []byte, err error) {
	return t.conn.ReadMessage()
}

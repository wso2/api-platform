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
)

// Transport defines an abstraction layer for protocol-independent message delivery.
// This interface allows the system to switch between different transport mechanisms
// (WebSocket, Server-Sent Events, gRPC, etc.) without modifying business logic.
//
// Design rationale: The transport abstraction isolates protocol-specific code from
// event routing and connection management logic, enabling future protocol changes
// without rewriting the core system.
type Transport interface {
	// Send delivers a message to the connected client.
	// Returns an error if the send operation fails (e.g., connection closed, timeout).
	//
	// Parameters:
	//   - message: The message payload to send (typically JSON-encoded)
	//
	// The implementation should handle protocol-specific framing and encoding.
	Send(message []byte) error

	// Close terminates the transport connection gracefully.
	// The implementation should send appropriate close frames/messages as per
	// the protocol specification (e.g., WebSocket close frame with code 1000).
	//
	// Parameters:
	//   - code: Protocol-specific close code (e.g., WebSocket close codes)
	//   - reason: Human-readable reason for connection closure
	Close(code int, reason string) error

	// SetReadDeadline sets the deadline for reading from the transport.
	// Used for implementing heartbeat/keepalive timeout detection.
	//
	// Parameters:
	//   - deadline: Absolute time when read operations should timeout
	//
	// A zero time value disables the deadline.
	SetReadDeadline(deadline time.Time) error

	// SetWriteDeadline sets the deadline for writing to the transport.
	// Used to prevent indefinite blocking on send operations.
	//
	// Parameters:
	//   - deadline: Absolute time when write operations should timeout
	//
	// A zero time value disables the deadline.
	SetWriteDeadline(deadline time.Time) error

	// EnablePongHandler configures automatic handling of pong frames for heartbeat.
	// For WebSocket, this sets up the pong handler. Other transports may implement
	// equivalent keepalive mechanisms.
	//
	// Parameters:
	//   - handler: Callback function invoked when a pong frame is received
	//
	// This is called to reset read deadlines and maintain connection liveness.
	EnablePongHandler(handler func(string) error)

	// SendPing sends a ping frame to test connection liveness.
	// Returns an error if the ping cannot be sent.
	//
	// For protocols without built-in ping/pong, implementations may use
	// application-level heartbeat messages.
	SendPing() error
}

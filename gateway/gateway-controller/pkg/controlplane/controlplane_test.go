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
	"testing"
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

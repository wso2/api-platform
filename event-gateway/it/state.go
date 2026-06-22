// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

package it

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Port constants for event gateway services.
const (
	// GatewayControllerPort is the management REST API port.
	GatewayControllerPort = "9090"

	// EventGatewayAdminPort is the admin/health port of the event-gateway runtime.
	EventGatewayAdminPort = "9002"

	// WebSubPort is the HTTP WebSub listener port.
	WebSubPort = "8080"

	// WebSocketPort is the WebSocket listener port.
	WebSocketPort = "8081"

	// WebhookListenerPort is the port of the wh-listener test service.
	WebhookListenerPort = "8090"

	// GatewayManagementAPIBasePath is the base path for the management REST API.
	GatewayManagementAPIBasePath = "/api/management/v1alpha2"
)

// AuthUser holds credentials for a test user.
type AuthUser struct {
	Username string
	Password string
}

// Config holds the addresses and settings used across all test scenarios.
type Config struct {
	GatewayControllerURL string
	EventGatewayAdminURL string
	WebSubURL            string
	WebSocketURL         string
	WebhookListenerURL   string
	KafkaBrokers         []string
	KafkaUsername        string
	KafkaPassword        string
	HTTPTimeout          time.Duration
	Users                map[string]AuthUser
}

// DefaultConfig returns the default test configuration assuming services are
// mapped to localhost ports via docker-compose.dev.yaml.
func DefaultConfig() *Config {
	return &Config{
		GatewayControllerURL: fmt.Sprintf("http://localhost:%s%s", GatewayControllerPort, GatewayManagementAPIBasePath),
		EventGatewayAdminURL: fmt.Sprintf("http://localhost:%s", EventGatewayAdminPort),
		WebSubURL:            fmt.Sprintf("http://localhost:%s", WebSubPort),
		WebSocketURL:         fmt.Sprintf("ws://localhost:%s", WebSocketPort),
		WebhookListenerURL:   fmt.Sprintf("http://localhost:%s", WebhookListenerPort),
		KafkaBrokers:         []string{"localhost:29092"},
		KafkaUsername:        "egw",
		KafkaPassword:        "egw-pass",
		HTTPTimeout:          15 * time.Second,
		Users: map[string]AuthUser{
			"admin": {Username: "admin", Password: "admin"},
		},
	}
}

// TestState is the global state shared across step definitions within a scenario.
type TestState struct {
	Config     *Config
	HTTPClient *http.Client

	// lastResponse holds the most recent HTTP response.
	lastResponse *http.Response
	// lastBody holds the body bytes of the most recent HTTP response.
	lastBody []byte
	// headers is the set of request headers to include in the next request.
	headers map[string]string
	// wsConn is the active WebSocket connection, nil if none.
	wsConn *websocket.Conn
	// wsConnErr holds the error from the last WebSocket connect attempt.
	wsConnErr error
	// wsRejectionStatus holds the HTTP status code returned when a WebSocket upgrade was rejected.
	wsRejectionStatus int
}

// NewTestState creates and returns an initialised TestState.
func NewTestState(cfg *Config) *TestState {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	}
	return &TestState{
		Config: cfg,
		HTTPClient: &http.Client{
			Timeout:   cfg.HTTPTimeout,
			Transport: transport,
		},
		headers: make(map[string]string),
	}
}

// Reset clears per-scenario state.
func (s *TestState) Reset() {
	s.lastResponse = nil
	s.lastBody = nil
	s.headers = make(map[string]string)
	if s.wsConn != nil {
		_ = s.wsConn.Close()
		s.wsConn = nil
	}
	s.wsConnErr = nil
	s.wsRejectionStatus = 0
}

// SetHeader sets a request header that will be included in subsequent requests.
func (s *TestState) SetHeader(key, value string) {
	s.headers[key] = value
}

// LastResponse returns the most recent HTTP response.
func (s *TestState) LastResponse() *http.Response {
	return s.lastResponse
}

// LastBody returns the body bytes of the most recent HTTP response.
func (s *TestState) LastBody() []byte {
	return s.lastBody
}

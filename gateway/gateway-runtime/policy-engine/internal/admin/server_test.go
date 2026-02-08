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

package admin

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// =============================================================================
// NewServer Tests
// =============================================================================

// getFreePort finds an available port for testing
func getFreePort(t *testing.T) int {
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

func TestNewServer(t *testing.T) {
	port := getFreePort(t)
	cfg := &config.AdminConfig{
		Port:       port,
		AllowedIPs: []string{"127.0.0.1"},
	}
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}

	server := NewServer(cfg, k, reg)

	require.NotNil(t, server)
	assert.Equal(t, cfg, server.cfg)
	assert.NotNil(t, server.httpServer)
	assert.Equal(t, fmt.Sprintf(":%d", port), server.httpServer.Addr)
}

// =============================================================================
// Start and Stop Tests
// =============================================================================

func TestServer_StartAndStop(t *testing.T) {
	port := getFreePort(t)
	cfg := &config.AdminConfig{
		Port:       port,
		AllowedIPs: []string{"127.0.0.1", "*"},
	}
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}

	server := NewServer(cfg, k, reg)
	ctx := context.Background()

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is responding
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/config_dump", port))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Stop server
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Stop(stopCtx)
	assert.NoError(t, err)

	// Wait for Start to return
	select {
	case startErr := <-errChan:
		// Should return nil (or http.ErrServerClosed which is handled)
		assert.NoError(t, startErr)
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not stop within timeout")
	}
}

func TestServer_StartWithInvalidPort(t *testing.T) {
	// First, bind a port so it's in use
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer listener.Close()
	usedPort := listener.Addr().(*net.TCPAddr).Port

	cfg := &config.AdminConfig{
		Port:       usedPort,
		AllowedIPs: []string{"127.0.0.1"},
	}
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Definitions: make(map[string]*policy.PolicyDefinition),
		Factories:   make(map[string]policy.PolicyFactory),
	}

	server := NewServer(cfg, k, reg)

	// Start should fail because port is already in use
	ctx := context.Background()
	err = server.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "admin server error")
}

// TestExtractClientIP tests the extractClientIP function
func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		expected   string
	}{
		{
			name:       "IPv4 with port",
			remoteAddr: "192.168.1.100:54321",
			expected:   "192.168.1.100",
		},
		{
			name:       "localhost with port",
			remoteAddr: "127.0.0.1:8080",
			expected:   "127.0.0.1",
		},
		{
			name:       "IPv6 loopback with port",
			remoteAddr: "[::1]:8080",
			expected:   "::1",
		},
		{
			name:       "IPv6 full address with port",
			remoteAddr: "[2001:db8::1]:8080",
			expected:   "2001:db8::1",
		},
		{
			name:       "IP without port (edge case)",
			remoteAddr: "192.168.1.100",
			expected:   "192.168.1.100",
		},
		{
			name:       "empty address",
			remoteAddr: "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr

			result := extractClientIP(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsIPAllowed tests the isIPAllowed function
func TestIsIPAllowed(t *testing.T) {
	tests := []struct {
		name       string
		clientIP   string
		allowedIPs []string
		expected   bool
	}{
		{
			name:       "exact match - localhost",
			clientIP:   "127.0.0.1",
			allowedIPs: []string{"127.0.0.1"},
			expected:   true,
		},
		{
			name:       "exact match - specific IP",
			clientIP:   "192.168.1.100",
			allowedIPs: []string{"192.168.1.100"},
			expected:   true,
		},
		{
			name:       "match in list",
			clientIP:   "192.168.1.100",
			allowedIPs: []string{"127.0.0.1", "192.168.1.100", "10.0.0.1"},
			expected:   true,
		},
		{
			name:       "IPv6 loopback match",
			clientIP:   "::1",
			allowedIPs: []string{"127.0.0.1", "::1"},
			expected:   true,
		},
		{
			name:       "not in allowed list",
			clientIP:   "192.168.1.200",
			allowedIPs: []string{"127.0.0.1", "192.168.1.100"},
			expected:   false,
		},
		{
			name:       "empty allowed list",
			clientIP:   "192.168.1.100",
			allowedIPs: []string{},
			expected:   false,
		},
		{
			name:       "wildcard * allows all",
			clientIP:   "192.168.1.100",
			allowedIPs: []string{"*"},
			expected:   true,
		},
		{
			name:       "wildcard 0.0.0.0/0 allows all",
			clientIP:   "10.20.30.40",
			allowedIPs: []string{"0.0.0.0/0"},
			expected:   true,
		},
		{
			name:       "wildcard with other IPs",
			clientIP:   "8.8.8.8",
			allowedIPs: []string{"127.0.0.1", "*"},
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isIPAllowed(tt.clientIP, tt.allowedIPs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIPWhitelistMiddleware tests the ipWhitelistMiddleware function
func TestIPWhitelistMiddleware(t *testing.T) {
	// Create a simple handler that just returns 200 OK
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	tests := []struct {
		name           string
		allowedIPs     []string
		remoteAddr     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "allowed IP - passes through",
			allowedIPs:     []string{"127.0.0.1"},
			remoteAddr:     "127.0.0.1:54321",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name:           "blocked IP - returns 403",
			allowedIPs:     []string{"127.0.0.1"},
			remoteAddr:     "192.168.1.100:54321",
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Forbidden\n",
		},
		{
			name:           "wildcard allows any IP",
			allowedIPs:     []string{"*"},
			remoteAddr:     "8.8.8.8:54321",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name:           "IPv6 loopback allowed",
			allowedIPs:     []string{"127.0.0.1", "::1"},
			remoteAddr:     "[::1]:54321",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name:           "empty allowed list blocks all",
			allowedIPs:     []string{},
			remoteAddr:     "127.0.0.1:54321",
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Forbidden\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := ipWhitelistMiddleware(tt.allowedIPs, nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/config_dump", nil)
			req.RemoteAddr = tt.remoteAddr

			recorder := httptest.NewRecorder()
			middleware.ServeHTTP(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)
			assert.Equal(t, tt.expectedBody, recorder.Body.String())
		})
	}
}

// TestIPWhitelistMiddleware_PreservesRequestPath tests that the middleware preserves the request path
func TestIPWhitelistMiddleware_PreservesRequestPath(t *testing.T) {
	var capturedPath string
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	middleware := ipWhitelistMiddleware([]string{"127.0.0.1"}, nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/config_dump?param=value", nil)
	req.RemoteAddr = "127.0.0.1:54321"

	recorder := httptest.NewRecorder()
	middleware.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "/config_dump", capturedPath)
}

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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

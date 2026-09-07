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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
)

// generateSelfSignedCert writes a self-signed ECDSA cert/key pair for
// "localhost" to certPath/keyPath, for exercising the admin TLS listener in
// tests without depending on any repo-committed certificate material.
func generateSelfSignedCert(t *testing.T, certPath, keyPath string) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	certOut, err := os.Create(certPath)
	require.NoError(t, err)
	defer certOut.Close()
	require.NoError(t, pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}))

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)

	keyOut, err := os.Create(keyPath)
	require.NoError(t, err)
	defer keyOut.Close()
	require.NoError(t, pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))
}

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
		Policies: make(map[string]*registry.PolicyEntry),
	}

	server := NewServer(cfg, k, reg, nil, nil, nil)

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
		ConfigDump: config.ConfigDumpConfig{Enabled: true},
	}
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{
		Policies: make(map[string]*registry.PolicyEntry),
	}

	server := NewServer(cfg, k, reg, &mockXDSSyncProvider{version: "pc-v11"}, nil, nil)
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

	syncResp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/xds_sync_status", port))
	require.NoError(t, err)
	syncResp.Body.Close()
	assert.Equal(t, http.StatusOK, syncResp.StatusCode)

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
		Policies: make(map[string]*registry.PolicyEntry),
	}

	server := NewServer(cfg, k, reg, nil, nil, nil)

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

// =============================================================================
// TLS Listener Tests
// =============================================================================

// TestServer_TLSListener verifies the admin API is reachable over the
// additional TLS listener, using the PQC hybrid group first in the
// preference list, while the plaintext listener keeps serving unchanged.
func TestServer_TLSListener(t *testing.T) {
	plainPort := getFreePort(t)
	tlsPort := getFreePort(t)

	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "admin.crt")
	keyPath := filepath.Join(tmpDir, "admin.key")
	generateSelfSignedCert(t, certPath, keyPath)

	cfg := &config.AdminConfig{
		Port:       plainPort,
		AllowedIPs: []string{"127.0.0.1", "*"},
		TLS: config.AdminTLSConfig{
			Enabled:                true,
			Port:                   tlsPort,
			CertPath:               certPath,
			KeyPath:                keyPath,
			MinimumProtocolVersion: "TLS1_2",
			MaximumProtocolVersion: "TLS1_3",
			EcdhCurves:             "X25519MLKEM768,X25519,P-256",
		},
	}
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{Policies: make(map[string]*registry.PolicyEntry)}

	server := NewServer(cfg, k, reg, &mockXDSSyncProvider{version: "pc-v11"}, nil, nil)
	require.NotNil(t, server.tlsServer)

	ctx := context.Background()
	errChan := make(chan error, 1)
	go func() { errChan <- server.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// The plaintext listener is unaffected by enabling TLS.
	plainResp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", plainPort))
	require.NoError(t, err)
	plainResp.Body.Close()
	assert.Equal(t, http.StatusOK, plainResp.StatusCode)

	// The TLS listener serves the same routes, negotiating the hybrid
	// PQC group when the client offers it.
	httpsClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // test-only self-signed cert
				CurvePreferences:   []tls.CurveID{tls.X25519MLKEM768, tls.X25519},
			},
		},
	}
	tlsResp, err := httpsClient.Get(fmt.Sprintf("https://127.0.0.1:%d/health", tlsPort))
	require.NoError(t, err)
	defer tlsResp.Body.Close()
	assert.Equal(t, http.StatusOK, tlsResp.StatusCode)
	require.NotNil(t, tlsResp.TLS)
	assert.Equal(t, tls.X25519MLKEM768, tlsResp.TLS.CurveID)

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, server.Stop(stopCtx))

	select {
	case startErr := <-errChan:
		assert.NoError(t, startErr)
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not stop within timeout")
	}
}

// TestServer_TLSListener_ClassicalFallback verifies a client that doesn't
// offer the PQC hybrid group still completes the handshake against the same
// listener, falling back to the classical curve later in the preference list.
func TestServer_TLSListener_ClassicalFallback(t *testing.T) {
	plainPort := getFreePort(t)
	tlsPort := getFreePort(t)

	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "admin.crt")
	keyPath := filepath.Join(tmpDir, "admin.key")
	generateSelfSignedCert(t, certPath, keyPath)

	cfg := &config.AdminConfig{
		Port:       plainPort,
		AllowedIPs: []string{"127.0.0.1", "*"},
		TLS: config.AdminTLSConfig{
			Enabled:                true,
			Port:                   tlsPort,
			CertPath:               certPath,
			KeyPath:                keyPath,
			MinimumProtocolVersion: "TLS1_2",
			MaximumProtocolVersion: "TLS1_3",
			EcdhCurves:             "X25519MLKEM768,X25519,P-256",
		},
	}
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{Policies: make(map[string]*registry.PolicyEntry)}

	server := NewServer(cfg, k, reg, &mockXDSSyncProvider{version: "pc-v11"}, nil, nil)

	ctx := context.Background()
	errChan := make(chan error, 1)
	go func() { errChan <- server.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	httpsClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,                         //nolint:gosec // test-only self-signed cert
				CurvePreferences:   []tls.CurveID{tls.CurveP256}, // no PQC support offered
			},
		},
	}
	resp, err := httpsClient.Get(fmt.Sprintf("https://127.0.0.1:%d/health", tlsPort))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, resp.TLS)
	assert.Equal(t, tls.CurveP256, resp.TLS.CurveID)

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, server.Stop(stopCtx))

	select {
	case startErr := <-errChan:
		assert.NoError(t, startErr)
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not stop within timeout")
	}
}

// TestServer_TLSListener_InvalidEcdhCurves verifies an invalid curve name
// disables the TLS listener rather than panicking — config validation
// (Config.Validate) is the real gate and already rejects this in production.
func TestServer_TLSListener_InvalidEcdhCurves(t *testing.T) {
	cfg := &config.AdminConfig{
		Port:       getFreePort(t),
		AllowedIPs: []string{"127.0.0.1"},
		TLS: config.AdminTLSConfig{
			Enabled:                true,
			Port:                   getFreePort(t),
			CertPath:               "/nonexistent/cert.pem",
			KeyPath:                "/nonexistent/key.pem",
			MinimumProtocolVersion: "TLS1_2",
			MaximumProtocolVersion: "TLS1_3",
			EcdhCurves:             "not-a-curve",
		},
	}
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{Policies: make(map[string]*registry.PolicyEntry)}

	server := NewServer(cfg, k, reg, &mockXDSSyncProvider{version: "pc-v11"}, nil, nil)
	assert.Nil(t, server.tlsServer)
}

// TestServer_TLSListener_CertLoadFailure verifies that a cert/key pair that
// fails to load — unlike an invalid ecdh_curves/ciphers/protocol-version
// value, which config validation already rejects before NewServer ever runs
// in production — is only discoverable at Start() time, and that Start must
// surface it as an error rather than silently continuing to serve the
// plaintext listener with TLS quietly absent.
func TestServer_TLSListener_CertLoadFailure(t *testing.T) {
	plainPort := getFreePort(t)
	tlsPort := getFreePort(t)

	cfg := &config.AdminConfig{
		Port:       plainPort,
		AllowedIPs: []string{"127.0.0.1", "*"},
		TLS: config.AdminTLSConfig{
			Enabled:                true,
			Port:                   tlsPort,
			CertPath:               "/nonexistent/cert.pem",
			KeyPath:                "/nonexistent/key.pem",
			MinimumProtocolVersion: "TLS1_2",
			MaximumProtocolVersion: "TLS1_3",
			EcdhCurves:             "X25519,P-256",
		},
	}
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{Policies: make(map[string]*registry.PolicyEntry)}

	server := NewServer(cfg, k, reg, &mockXDSSyncProvider{version: "pc-v11"}, nil, nil)
	require.NotNil(t, server.tlsServer) // config itself is valid — the cert files just don't exist

	err := server.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load certificate")

	// The plaintext listener must never have started serving.
	_, dialErr := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", plainPort))
	assert.Error(t, dialErr, "plaintext listener should not be serving when TLS setup fails")
}

// TestServer_TLSListener_MinimumVersionEnforced verifies a client offering
// only a protocol version below MinimumProtocolVersion is rejected by the
// handshake rather than silently downgrading.
func TestServer_TLSListener_MinimumVersionEnforced(t *testing.T) {
	plainPort := getFreePort(t)
	tlsPort := getFreePort(t)

	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "admin.crt")
	keyPath := filepath.Join(tmpDir, "admin.key")
	generateSelfSignedCert(t, certPath, keyPath)

	cfg := &config.AdminConfig{
		Port:       plainPort,
		AllowedIPs: []string{"127.0.0.1", "*"},
		TLS: config.AdminTLSConfig{
			Enabled:                true,
			Port:                   tlsPort,
			CertPath:               certPath,
			KeyPath:                keyPath,
			MinimumProtocolVersion: "TLS1_2",
			MaximumProtocolVersion: "TLS1_3",
			EcdhCurves:             "X25519,P-256",
		},
	}
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{Policies: make(map[string]*registry.PolicyEntry)}

	server := NewServer(cfg, k, reg, &mockXDSSyncProvider{version: "pc-v11"}, nil, nil)

	ctx := context.Background()
	errChan := make(chan error, 1)
	go func() { errChan <- server.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	// A client capped at TLS 1.1 cannot complete the handshake against a
	// listener whose floor is TLS 1.2.
	httpsClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // test-only self-signed cert
				MinVersion:         tls.VersionTLS10,
				MaxVersion:         tls.VersionTLS11,
			},
		},
	}
	_, err := httpsClient.Get(fmt.Sprintf("https://127.0.0.1:%d/health", tlsPort))
	assert.Error(t, err)

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, server.Stop(stopCtx))

	select {
	case startErr := <-errChan:
		assert.NoError(t, startErr)
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not stop within timeout")
	}
}

// TestServer_TLSListener_CipherRestriction verifies a configured Ciphers
// list actually constrains which TLS 1.2 suite gets negotiated.
func TestServer_TLSListener_CipherRestriction(t *testing.T) {
	plainPort := getFreePort(t)
	tlsPort := getFreePort(t)

	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "admin.crt")
	keyPath := filepath.Join(tmpDir, "admin.key")
	generateSelfSignedCert(t, certPath, keyPath)

	cfg := &config.AdminConfig{
		Port:       plainPort,
		AllowedIPs: []string{"127.0.0.1", "*"},
		TLS: config.AdminTLSConfig{
			Enabled:                true,
			Port:                   tlsPort,
			CertPath:               certPath,
			KeyPath:                keyPath,
			MinimumProtocolVersion: "TLS1_2",
			MaximumProtocolVersion: "TLS1_2", // pin to 1.2 so CipherSuites governs selection
			Ciphers:                "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			EcdhCurves:             "X25519,P-256",
		},
	}
	k := kernel.NewKernel()
	reg := &registry.PolicyRegistry{Policies: make(map[string]*registry.PolicyEntry)}

	server := NewServer(cfg, k, reg, &mockXDSSyncProvider{version: "pc-v11"}, nil, nil)
	require.NotNil(t, server.tlsServer)

	ctx := context.Background()
	errChan := make(chan error, 1)
	go func() { errChan <- server.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	httpsClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // test-only self-signed cert
				MaxVersion:         tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, // offered but not configured server-side
				},
			},
		},
	}
	resp, err := httpsClient.Get(fmt.Sprintf("https://127.0.0.1:%d/health", tlsPort))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, resp.TLS)
	assert.Equal(t, uint16(tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256), resp.TLS.CipherSuite)

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, server.Stop(stopCtx))

	select {
	case startErr := <-errChan:
		assert.NoError(t, startErr)
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not stop within timeout")
	}
}

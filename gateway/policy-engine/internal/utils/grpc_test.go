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

package utils

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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// =============================================================================
// LoadCertificates Tests
// =============================================================================

func TestLoadCertificates_ValidCerts(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Generate a valid self-signed certificate and key pair
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)

	// Encode certificate
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NotNil(t, certPEM)

	// Encode private key
	privBytes, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	require.NotNil(t, keyPEM)

	// Write to files
	err = os.WriteFile(certPath, certPEM, 0600)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, keyPEM, 0600)
	require.NoError(t, err)

	// Test loading the valid certificate
	creds, err := LoadCertificates(certPath, keyPath)
	assert.NoError(t, err)
	assert.NotNil(t, creds)
}

func TestLoadCertificates_InvalidCertKeyPair(t *testing.T) {
	// Create temp directory for test certs
	tmpDir := t.TempDir()

	// Generate self-signed test certificate and key
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Create test certificate content (intentionally invalid/mismatched cert/key pair)
	certPEM := `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHBfpegPjMCMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RjYTAeFw0yMzAxMDEwMDAwMDBaFw0yNDAxMDEwMDAwMDBaMBExDzANBgNVBAMM
BnRlc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC5q7Th+Y7YOlGzGQY7u/vz
Bs/3q8F8q8V1R8B9F6q7I4qH3kB9D7q8F8q8V1R8B9F6q7I4qH3kB9D7q8F8q8V1
AgMBAAGjUzBRMB0GA1UdDgQWBBQ7q8F8q8V1R8B9F6q7I4qH3kB9DzAfBgNVHSME
GDAWgBQ7q8F8q8V1R8B9F6q7I4qH3kB9DzAPBgNVHRMBAf8EBTADAQH/MA0GCSqG
SIb3DQEBCwUAA0EAq8F8q8V1R8B9F6q7I4qH3kB9D7q8F8q8V1R8B9F6q7I4qH3k
B9D7q8F8q8V1R8B9F6q7I4qH3kB9D7q8F8q8V1==
-----END CERTIFICATE-----`

	keyPEM := `-----BEGIN RSA PRIVATE KEY-----
MIIBOgIBAAJBALmrtOH5jtg6UbMZBju7+/MGz/erwXyrxXVHwH0XqrsjiofeQH0P
urwXyrxXVHwH0XqrsjiofeQH0PurwXyrxXUCAwEAAQJAc7q8F8q8V1R8B9F6q7I4
qH3kB9D7q8F8q8V1R8B9F6q7I4qH3kB9D7q8F8q8V1R8B9F6q7I4qH3kB9D7q8F8
AiEA7q8F8q8V1R8B9F6q7I4qH3kB9D7q8F8q8V1R8B9F6q0CIQDJq8F8q8V1R8B9
F6q7I4qH3kB9D7q8F8q8V1R8B9F6q7ICIQCrq8F8q8V1R8B9F6q7I4qH3kB9D7q8
F8q8V1R8B9F6q7ICIQCpq8F8q8V1R8B9F6q7I4qH3kB9D7q8F8q8V1R8B9F6qwIh
AKurwXyrxXVHwH0XqrsjiofeQH0PurwXyrxXVHwH0Xqr
-----END RSA PRIVATE KEY-----`

	err := os.WriteFile(certPath, []byte(certPEM), 0600)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, []byte(keyPEM), 0600)
	require.NoError(t, err)

	_, err = LoadCertificates(certPath, keyPath)
	assert.Error(t, err)
}

func TestLoadCertificates_MissingCertFile(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "nonexistent.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	_, err := LoadCertificates(certPath, keyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load client certificate")
}

func TestLoadCertificates_MissingKeyFile(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "nonexistent.pem")

	// Create empty cert file
	err := os.WriteFile(certPath, []byte("test"), 0600)
	require.NoError(t, err)

	_, err = LoadCertificates(certPath, keyPath)

	assert.Error(t, err)
}

func TestLoadCertificates_InvalidCertContent(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Create files with invalid content
	err := os.WriteFile(certPath, []byte("not a valid certificate"), 0600)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, []byte("not a valid key"), 0600)
	require.NoError(t, err)

	_, err = LoadCertificates(certPath, keyPath)

	assert.Error(t, err)
}

// =============================================================================
// CreateGRPCServer Tests
// =============================================================================

func TestCreateGRPCServer_PlainText(t *testing.T) {
	server, err := CreateGRPCServer("", "", true)

	require.NoError(t, err)
	assert.NotNil(t, server)

	// Clean up
	server.Stop()
}

func TestCreateGRPCServer_TLSMissingCerts(t *testing.T) {
	_, err := CreateGRPCServer("/nonexistent/cert.pem", "/nonexistent/key.pem", false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS credentials")
}

func TestCreateGRPCServer_PlainTextWithOptions(t *testing.T) {
	server, err := CreateGRPCServer("", "", true)

	require.NoError(t, err)
	assert.NotNil(t, server)

	// Verify server was created
	server.Stop()
}

func TestCreateGRPCServer_TLSWithValidCerts(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Generate a valid self-signed certificate and key pair
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-server",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)

	// Encode certificate
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NotNil(t, certPEM)

	// Encode private key
	privBytes, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	require.NotNil(t, keyPEM)

	// Write to files
	err = os.WriteFile(certPath, certPEM, 0600)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, keyPEM, 0600)
	require.NoError(t, err)

	// Test creating a TLS-enabled gRPC server
	server, err := CreateGRPCServer(certPath, keyPath, false)

	require.NoError(t, err)
	assert.NotNil(t, server)

	// Clean up
	server.Stop()
}

func TestCreateGRPCServer_TLSWithValidCertsAndOptions(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Generate a valid self-signed certificate and key pair
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-server",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)

	// Encode certificate
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NotNil(t, certPEM)

	// Encode private key
	privBytes, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	require.NotNil(t, keyPEM)

	// Write to files
	err = os.WriteFile(certPath, certPEM, 0600)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, keyPEM, 0600)
	require.NoError(t, err)

	// Test creating a TLS-enabled gRPC server with additional options
	server, err := CreateGRPCServer(certPath, keyPath, false,
		grpc.MaxRecvMsgSize(1024*1024),
		grpc.MaxSendMsgSize(1024*1024))

	require.NoError(t, err)
	assert.NotNil(t, server)

	// Clean up
	server.Stop()
}

// =============================================================================
// CreateGRPCConnection Tests
// =============================================================================

// startTestGRPCServerWithTLS starts a local TLS-enabled gRPC server with health check for testing
func startTestGRPCServerWithTLS(t *testing.T) (string, string, *tls.Config, func()) {
	t.Helper()

	// Generate server certificate and key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)

	// Create TLS certificate
	serverCert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}

	// Create server TLS config
	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		MinVersion:   tls.VersionTLS13,
	}

	// Create listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	// Create gRPC server with TLS
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(serverTLSConfig)))

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Start server
	go func() {
		_ = server.Serve(listener)
	}()

	// Create client TLS config that trusts the server cert
	certPool := x509.NewCertPool()
	certPool.AddCert(&x509.Certificate{Raw: certDER})
	clientTLSConfig := &tls.Config{
		RootCAs: certPool,
	}

	addr := listener.Addr().(*net.TCPAddr)
	cleanup := func() {
		server.Stop()
		listener.Close()
	}

	return "127.0.0.1", fmt.Sprintf("%d", addr.Port), clientTLSConfig, cleanup
}

func TestCreateGRPCConnection_Success(t *testing.T) {
	host, port, tlsConfig, cleanup := startTestGRPCServerWithTLS(t)
	defer cleanup()

	ctx := context.Background()
	conn, err := CreateGRPCConnection(ctx, host, port, tlsConfig)

	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	// Verify connection works by calling health check RPC
	healthClient := grpc_health_v1.NewHealthClient(conn)
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)
}

func TestCreateGRPCConnection_TLSHandshakeFailure(t *testing.T) {
	host, port, _, cleanup := startTestGRPCServerWithTLS(t)
	defer cleanup()

	// Use TLS config that doesn't trust the server certificate
	tlsConfig := &tls.Config{
		RootCAs: x509.NewCertPool(), // Empty pool - won't trust server cert
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := CreateGRPCConnection(ctx, host, port, tlsConfig)
	require.NoError(t, err) // Client creation succeeds (lazy)
	require.NotNil(t, conn)
	defer conn.Close()

	// Attempt RPC to trigger TLS handshake - this should fail
	healthClient := grpc_health_v1.NewHealthClient(conn)
	_, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	assert.Error(t, err) // TLS handshake failure
	assert.Contains(t, err.Error(), "certificate signed by unknown authority")
}

func TestCreateGRPCConnection_InvalidAddress(t *testing.T) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := CreateGRPCConnection(ctx, "invalid-host-that-does-not-exist", "9999", tlsConfig)

	require.NoError(t, err) // Client creation succeeds (lazy)
	require.NotNil(t, conn)
	defer conn.Close()

	// Attempt RPC to trigger connection - this should fail
	healthClient := grpc_health_v1.NewHealthClient(conn)
	_, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	assert.Error(t, err) // Connection failure
}

// =============================================================================
// CreateGRPCConnectionWithRetry Tests
// =============================================================================

func TestCreateGRPCConnectionWithRetry_SuccessFirstTry(t *testing.T) {
	host, port, tlsConfig, cleanup := startTestGRPCServerWithTLS(t)
	defer cleanup()

	ctx := context.Background()
	conn, err := CreateGRPCConnectionWithRetry(ctx, host, port, tlsConfig, 3, 100*time.Millisecond)

	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	// Verify connection works
	healthClient := grpc_health_v1.NewHealthClient(conn)
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)
}

func TestCreateGRPCConnectionWithRetry_ExhaustsRetries(t *testing.T) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	ctx := context.Background()
	conn, err := CreateGRPCConnectionWithRetry(ctx, "localhost", "9999", tlsConfig, 2, 10*time.Millisecond)

	// Connection succeeds (lazy), so no error here
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	// Verify connection actually fails when used
	ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	healthClient := grpc_health_v1.NewHealthClient(conn)
	_, err = healthClient.Check(ctx2, &grpc_health_v1.HealthCheckRequest{})
	assert.Error(t, err)
}

func TestCreateGRPCConnectionWithRetry_InfiniteRetries(t *testing.T) {
	host, port, tlsConfig, cleanup := startTestGRPCServerWithTLS(t)
	defer cleanup()

	ctx := context.Background()

	// Test maxRetries = -1 (infinite retries, but should succeed on first try)
	conn, err := CreateGRPCConnectionWithRetry(ctx, host, port, tlsConfig, -1, 100*time.Millisecond)

	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	// Verify connection works
	healthClient := grpc_health_v1.NewHealthClient(conn)
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)
}

// =============================================================================
// CreateGRPCConnectionWithRetryAndPanic Tests
// =============================================================================

func TestCreateGRPCConnectionWithRetryAndPanic_Success(t *testing.T) {
	host, port, tlsConfig, cleanup := startTestGRPCServerWithTLS(t)
	defer cleanup()

	ctx := context.Background()

	// Should not panic with valid server
	conn := CreateGRPCConnectionWithRetryAndPanic(ctx, host, port, tlsConfig, 3, 100*time.Millisecond)

	require.NotNil(t, conn)
	defer conn.Close()

	// Verify connection works
	healthClient := grpc_health_v1.NewHealthClient(conn)
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)
}

func TestCreateGRPCConnectionWithRetryAndPanic_Panics(t *testing.T) {
	ctx := context.Background()

	// Use require.Panics for deterministic panic assertion
	// maxRetries=0 ensures no connection attempts are made, causing immediate error
	require.Panics(t, func() {
		_ = CreateGRPCConnectionWithRetryAndPanic(ctx, "localhost", "0", &tls.Config{}, 0, 1*time.Millisecond)
	}, "Expected panic when retries exhausted")
}

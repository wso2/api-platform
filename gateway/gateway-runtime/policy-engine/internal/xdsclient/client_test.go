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

package xdsclient

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
)

// Helper to create a minimal valid config for testing
func createValidTestConfig() *Config {
	return &Config{
		ServerAddress:         "localhost:18000",
		NodeID:                "test-node",
		Cluster:               "test-cluster",
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        5 * time.Second,
		InitialReconnectDelay: 1 * time.Second,
		MaxReconnectDelay:     60 * time.Second,
		TLSEnabled:            false,
	}
}

// Helper to create test kernel and registry (minimal setup)
func createTestKernelAndRegistry(t *testing.T) (*kernel.Kernel, *registry.PolicyRegistry) {
	t.Helper()
	
	// Initialize metrics (noop mode is fine for tests)
	metrics.Init()
	
	// Create minimal kernel
	k := kernel.NewKernel()
	
	// Get global registry
	reg := registry.GetRegistry()
	
	return k, reg
}

// TestNewClient_InvalidConfig tests that NewClient returns error for invalid config
func TestNewClient_InvalidConfig(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "Empty config",
			config: &Config{},
		},
		{
			name: "Missing server address",
			config: &Config{
				NodeID:                "test",
				Cluster:               "test",
				ConnectTimeout:        10 * time.Second,
				RequestTimeout:        5 * time.Second,
				InitialReconnectDelay: 1 * time.Second,
				MaxReconnectDelay:     60 * time.Second,
			},
		},
		{
			name: "Missing node ID",
			config: &Config{
				ServerAddress:         "localhost:18000",
				Cluster:               "test",
				ConnectTimeout:        10 * time.Second,
				RequestTimeout:        5 * time.Second,
				InitialReconnectDelay: 1 * time.Second,
				MaxReconnectDelay:     60 * time.Second,
			},
		},
		{
			name: "Negative timeout",
			config: &Config{
				ServerAddress:         "localhost:18000",
				NodeID:                "test",
				Cluster:               "test",
				ConnectTimeout:        -10 * time.Second,
				RequestTimeout:        5 * time.Second,
				InitialReconnectDelay: 1 * time.Second,
				MaxReconnectDelay:     60 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config, k, reg)
			assert.Error(t, err)
			assert.Nil(t, client)
			assert.Contains(t, err.Error(), "invalid config")
		})
	}
}

// TestNewClient_ValidConfig tests that NewClient succeeds with valid config
func TestNewClient_ValidConfig(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)
	require.NotNil(t, client)
	
	assert.Equal(t, config, client.config)
	assert.NotNil(t, client.handler)
	assert.NotNil(t, client.reconnectManager)
	assert.NotNil(t, client.ctx)
	assert.NotNil(t, client.cancel)
}

// TestGetState tests the GetState method
func TestGetState(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	// Initial state should be Disconnected
	state := client.GetState()
	assert.Equal(t, StateDisconnected, state)
}

// TestSetState tests the setState method
func TestSetState(t *testing.T) {
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	// Test state transitions
	tests := []struct {
		name     string
		newState ClientState
	}{
		{
			name:     "Connecting",
			newState: StateConnecting,
		},
		{
			name:     "Connected",
			newState: StateConnected,
		},
		{
			name:     "Reconnecting",
			newState: StateReconnecting,
		},
		{
			name:     "Disconnected",
			newState: StateDisconnected,
		},
		{
			name:     "Stopped",
			newState: StateStopped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client.setState(tt.newState)
			assert.Equal(t, tt.newState, client.GetState())
		})
	}
}

// Helper functions to generate test certificates

func generateTestCA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
			CommonName:   "Test CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err)

	ca, err = x509.ParseCertificate(caBytes)
	require.NoError(t, err)

	return ca, caPrivKey
}

func generateTestCert(t *testing.T, ca *x509.Certificate, caPrivKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2020),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "localhost",
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err)

	cert, err = x509.ParseCertificate(certBytes)
	require.NoError(t, err)

	return cert, certPrivKey
}

func writeCertToFile(t *testing.T, cert *x509.Certificate, filename string) {
	t.Helper()
	
	certOut, err := os.Create(filename)
	require.NoError(t, err)
	defer certOut.Close()

	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	require.NoError(t, err)
}

func writeKeyToFile(t *testing.T, key *rsa.PrivateKey, filename string) {
	t.Helper()
	
	keyOut, err := os.Create(filename)
	require.NoError(t, err)
	defer keyOut.Close()

	privBytes := x509.MarshalPKCS1PrivateKey(key)
	err = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes})
	require.NoError(t, err)
}

// TestLoadTLSConfig_ValidCerts tests loading valid TLS certificates
func TestLoadTLSConfig_ValidCerts(t *testing.T) {
	// Create temp directory for test certs
	tmpDir := t.TempDir()

	// Generate test certificates
	ca, caPrivKey := generateTestCA(t)
	cert, certPrivKey := generateTestCert(t, ca, caPrivKey)

	// Write certificates to files
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	caPath := filepath.Join(tmpDir, "ca.pem")

	writeCertToFile(t, cert, certPath)
	writeKeyToFile(t, certPrivKey, keyPath)
	writeCertToFile(t, ca, caPath)

	// Create client with TLS config
	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.TLSEnabled = true
	config.TLSCertPath = certPath
	config.TLSKeyPath = keyPath
	config.TLSCAPath = caPath

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	// Test loadTLSConfig
	tlsConfig, err := client.loadTLSConfig()
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)

	assert.Len(t, tlsConfig.Certificates, 1)
	assert.NotNil(t, tlsConfig.RootCAs)
	assert.Equal(t, uint16(0x0303), tlsConfig.MinVersion) // TLS 1.2
}

// TestLoadTLSConfig_InvalidCertPath tests error when cert file doesn't exist
func TestLoadTLSConfig_InvalidCertPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate valid key and CA
	ca, caPrivKey := generateTestCA(t)
	_, certPrivKey := generateTestCert(t, ca, caPrivKey)

	keyPath := filepath.Join(tmpDir, "key.pem")
	caPath := filepath.Join(tmpDir, "ca.pem")

	writeKeyToFile(t, certPrivKey, keyPath)
	writeCertToFile(t, ca, caPath)

	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.TLSEnabled = true
	config.TLSCertPath = "/nonexistent/cert.pem"
	config.TLSKeyPath = keyPath
	config.TLSCAPath = caPath

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	tlsConfig, err := client.loadTLSConfig()
	assert.Error(t, err)
	assert.Nil(t, tlsConfig)
	assert.Contains(t, err.Error(), "failed to load client certificate")
}

// TestLoadTLSConfig_InvalidKeyPath tests error when key file doesn't exist
func TestLoadTLSConfig_InvalidKeyPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate valid cert and CA
	ca, caPrivKey := generateTestCA(t)
	cert, _ := generateTestCert(t, ca, caPrivKey)

	certPath := filepath.Join(tmpDir, "cert.pem")
	caPath := filepath.Join(tmpDir, "ca.pem")

	writeCertToFile(t, cert, certPath)
	writeCertToFile(t, ca, caPath)

	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.TLSEnabled = true
	config.TLSCertPath = certPath
	config.TLSKeyPath = "/nonexistent/key.pem"
	config.TLSCAPath = caPath

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	tlsConfig, err := client.loadTLSConfig()
	assert.Error(t, err)
	assert.Nil(t, tlsConfig)
	assert.Contains(t, err.Error(), "failed to load client certificate")
}

// TestLoadTLSConfig_InvalidCAPath tests error when CA file doesn't exist
func TestLoadTLSConfig_InvalidCAPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate valid cert and key
	ca, caPrivKey := generateTestCA(t)
	cert, certPrivKey := generateTestCert(t, ca, caPrivKey)

	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	writeCertToFile(t, cert, certPath)
	writeKeyToFile(t, certPrivKey, keyPath)

	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.TLSEnabled = true
	config.TLSCertPath = certPath
	config.TLSKeyPath = keyPath
	config.TLSCAPath = "/nonexistent/ca.pem"

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	tlsConfig, err := client.loadTLSConfig()
	assert.Error(t, err)
	assert.Nil(t, tlsConfig)
	assert.Contains(t, err.Error(), "failed to read CA certificate")
}

// TestLoadTLSConfig_InvalidCAFormat tests error when CA file has invalid format
func TestLoadTLSConfig_InvalidCAFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate valid cert and key
	ca, caPrivKey := generateTestCA(t)
	cert, certPrivKey := generateTestCert(t, ca, caPrivKey)

	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	caPath := filepath.Join(tmpDir, "ca.pem")

	writeCertToFile(t, cert, certPath)
	writeKeyToFile(t, certPrivKey, keyPath)

	// Write invalid CA file
	err := os.WriteFile(caPath, []byte("not a valid certificate"), 0600)
	require.NoError(t, err)

	k, reg := createTestKernelAndRegistry(t)
	config := createValidTestConfig()
	config.TLSEnabled = true
	config.TLSCertPath = certPath
	config.TLSKeyPath = keyPath
	config.TLSCAPath = caPath

	client, err := NewClient(config, k, reg)
	require.NoError(t, err)

	tlsConfig, err := client.loadTLSConfig()
	assert.Error(t, err)
	assert.Nil(t, tlsConfig)
	assert.Contains(t, err.Error(), "failed to parse CA certificate")
}

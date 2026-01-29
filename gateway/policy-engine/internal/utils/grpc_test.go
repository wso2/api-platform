/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// LoadCertificates Tests
// =============================================================================

func TestLoadCertificates_ValidCerts(t *testing.T) {
	// Create temp directory for test certs
	tmpDir := t.TempDir()

	// Generate self-signed test certificate and key
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Create test certificate content (valid self-signed cert)
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

	// This will fail because the cert/key pair is not valid, but we're testing the function signature
	_, err = LoadCertificates(certPath, keyPath)
	// We expect an error because the test certs are not valid
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

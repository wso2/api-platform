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

package certstore

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test certificate in PEM format (self-signed test cert)
const validCertPEM = `-----BEGIN CERTIFICATE-----
MIIB+jCCAWOgAwIBAgIUCNyE284LxvMJTE+42kjmK5e1vW4wDQYJKoZIhvcNAQEL
BQAwDzENMAsGA1UEAwwEdGVzdDAeFw0yNjAxMzAwOTQ2MTJaFw0yNjAxMzEwOTQ2
MTJaMA8xDTALBgNVBAMMBHRlc3QwgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGB
ANBI4kYFzQus5qPjuzJEzTQIi6C+hNHFn42toed+2tq/jvBpveaCtSfdLgbwDhZ0
uO5jArhCh++/zfsCqLptTy9nXfvvpJ564y+2Hzp5oFrBBY9Zkohl3ubutIpOG4bO
bo/uB2RvBYZRsUIjKG/NyD9F6I55Yw3vXlcFZkMZVGqrAgMBAAGjUzBRMB0GA1Ud
DgQWBBRNy/QwZlrUz7Jr5d86yYpsoRBoCDAfBgNVHSMEGDAWgBRNy/QwZlrUz7Jr
5d86yYpsoRBoCDAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4GBAIOA
aLH5I4KNIlLP5QTK5inG3bihRVbgyFhuS8/wG7k5ONl7bPjvO+VqcXcXQ4uvOY9f
NWeEEe+FnIqCMN4nbrt/Fmimn91F/+3ZBns/Z/L9HJYLlekVPtJXGaDVF6zcj/QP
+oz8QbmWNLWZz2J+vcZG9tikpw0r9EJ2t8tKgWYx
-----END CERTIFICATE-----`

const invalidPEM = `not a valid certificate`

const nonCertPEM = `-----BEGIN PRIVATE KEY-----
MIIBVQIBADANBgkqhkiG9w0BAQEFAASCAT8wggE7AgEAAkEAuXRVVe4HRD0Ud8Dt
yy+GSZdrdyqZdCWFi+CFcN8C1uswS9xei9itB2xAI/3+p3zUJd2y1rX76kbz76Ss
6R235QIDAQABAkA9QEJWp6Q9XF8ZXvDPMPNLzCn1Gxu8FqPLbJ7L8KvC5fPvHvJa
-----END PRIVATE KEY-----`

func TestNewCertStore(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cs := NewCertStore(logger, nil, "/test/certs", "/etc/ssl/certs/ca-certificates.crt")

	assert.NotNil(t, cs)
	assert.Equal(t, "/test/certs", cs.GetCertsDir())
	assert.Nil(t, cs.GetCombinedCertificates()) // Not loaded yet
}

func TestCertStore_GetCertsDir(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name     string
		certsDir string
		expected string
	}{
		{
			name:     "Standard path",
			certsDir: "/etc/gateway/certs",
			expected: "/etc/gateway/certs",
		},
		{
			name:     "Empty path",
			certsDir: "",
			expected: "",
		},
		{
			name:     "Relative path",
			certsDir: "./certs",
			expected: "./certs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := NewCertStore(logger, nil, tt.certsDir, "")
			assert.Equal(t, tt.expected, cs.GetCertsDir())
		})
	}
}

func TestCertStore_GetCombinedCertificates_BeforeLoad(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cs := NewCertStore(logger, nil, "", "")

	// Should return nil before LoadCertificates is called
	assert.Nil(t, cs.GetCombinedCertificates())
}

func TestCertStore_ValidateCertificateData(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cs := NewCertStore(logger, nil, "", "")

	tests := []struct {
		name        string
		certData    []byte
		wantCount   int
		wantErr     bool
		errContains string
	}{
		{
			name:      "Valid single certificate",
			certData:  []byte(validCertPEM),
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:        "Invalid PEM data",
			certData:    []byte(invalidPEM),
			wantCount:   0,
			wantErr:     true,
			errContains: "no valid certificates",
		},
		{
			name:        "Non-certificate PEM (private key)",
			certData:    []byte(nonCertPEM),
			wantCount:   0,
			wantErr:     true,
			errContains: "no valid certificates",
		},
		{
			name:        "Empty data",
			certData:    []byte{},
			wantCount:   0,
			wantErr:     true,
			errContains: "no valid certificates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := cs.validateCertificateData("test-cert", tt.certData)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantCount, count)
			}
		})
	}
}

func TestCertStore_ValidateAndExtractCertificates(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cs := NewCertStore(logger, nil, "", "")

	tests := []struct {
		name        string
		filename    string
		certData    []byte
		wantCount   int
		wantErr     bool
		errContains string
	}{
		{
			name:      "Valid certificate file",
			filename:  "test.pem",
			certData:  []byte(validCertPEM),
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:        "Invalid file content",
			filename:    "invalid.pem",
			certData:    []byte("not valid pem"),
			wantCount:   0,
			wantErr:     true,
			errContains: "no valid certificates",
		},
		{
			name:        "Empty file",
			filename:    "empty.pem",
			certData:    []byte{},
			wantCount:   0,
			wantErr:     true,
			errContains: "no valid certificates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := cs.validateAndExtractCertificates(tt.filename, tt.certData)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantCount, count)
			}
		})
	}
}

func TestGenerateCertificateID(t *testing.T) {
	// Generate multiple IDs and ensure they're unique
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateCertificateID()
		assert.NotEmpty(t, id)
		assert.False(t, ids[id], "Generated duplicate ID: %s", id)
		ids[id] = true
	}
}

func TestCertStore_MultipleCertificatesInChain(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cs := NewCertStore(logger, nil, "", "")

	// Create a chain with two certificates
	certChain := validCertPEM + "\n" + validCertPEM

	count, err := cs.validateCertificateData("chain.pem", []byte(certChain))
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
)

func testServer() *Server {
	return &Server{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// HTTPS listener with no certificates: a fatal misconfiguration — certificates
// are always required, there is no self-signed fallback.
func TestBuildTLSConfig_MissingCert_Errors(t *testing.T) {
	missingDir := filepath.Join(t.TempDir(), "does-not-exist")
	_, err := testServer().buildTLSConfig(config.HTTPSListener{
		Enabled: true,
		TLS: config.ListenerTLS{
			CertFile: filepath.Join(missingDir, "cert.pem"),
			KeyFile:  filepath.Join(missingDir, "key.pem"),
		},
	})
	if err == nil {
		t.Fatal("expected an error when the HTTPS listener has no certificates")
	}
}

// HTTPS listener with unset certificate paths: rejected before any file access.
func TestBuildTLSConfig_UnsetCertPaths_Errors(t *testing.T) {
	_, err := testServer().buildTLSConfig(config.HTTPSListener{Enabled: true})
	if err == nil {
		t.Fatal("expected an error when cert_file / key_file are not configured")
	}
}

// HTTPS listener with a mounted certificate pair: loaded successfully.
func TestBuildTLSConfig_MountedCert_Loads(t *testing.T) {
	certDir := t.TempDir()
	writeTestCertPair(t, certDir)

	tlsConfig, err := testServer().buildTLSConfig(config.HTTPSListener{
		Enabled: true,
		Port:    9243,
		TLS: config.ListenerTLS{
			CertFile: filepath.Join(certDir, "cert.pem"),
			KeyFile:  filepath.Join(certDir, "key.pem"),
		},
	})
	if err != nil {
		t.Fatalf("expected mounted certificates to load, got %v", err)
	}
	if tlsConfig == nil || len(tlsConfig.Certificates) != 1 {
		t.Fatal("expected exactly one loaded certificate")
	}
}

// writeTestCertPair writes a throwaway self-signed cert.pem / key.pem into dir.
func writeTestCertPair(t *testing.T, dir string) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"Platform API Test"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(filepath.Join(dir, "cert.pem"), certPEM, 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "key.pem"), keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
}

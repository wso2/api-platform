/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package config

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
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
)

// generateXDSTestCA generates a self-signed CA and returns its cert/key
// (PEM-encoded) plus a helper that mints a leaf certificate signed by it.
func generateXDSTestCA(t *testing.T) (caCertPEM []byte, caKey *ecdsa.PrivateKey, caCert *x509.Certificate) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test xDS CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return pemBytes, priv, cert
}

// writeXDSLeafCert mints a leaf certificate signed by the given CA and
// writes its cert+key as PEM files under dir, returning their paths.
func writeXDSLeafCert(t *testing.T, dir, name string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, isServer bool) (certPath, keyPath string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	if isServer {
		template.DNSNames = []string{"localhost"}
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &priv.PublicKey, caKey)
	require.NoError(t, err)

	certPath = filepath.Join(dir, name+".crt")
	keyPath = filepath.Join(dir, name+".key")

	certOut, err := os.Create(certPath)
	require.NoError(t, err)
	defer certOut.Close()
	require.NoError(t, pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}))

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	keyOut, err := os.Create(keyPath)
	require.NoError(t, err)
	defer keyOut.Close()
	require.NoError(t, pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))

	return certPath, keyPath
}

func TestValidateXDSServerTLS(t *testing.T) {
	validCfg := func() XDSServerTLSConfig {
		return XDSServerTLSConfig{
			Enabled:                 true,
			CertFile:                "./certs/server.crt",
			KeyFile:                 "./certs/server.key",
			ClientCAFile:            "./certs/ca.crt",
			AllowedClientIdentities: []string{"spiffe://cluster.local/ns/gw/sa/envoy"},
			MinimumProtocolVersion:  "TLS1_2",
			MaximumProtocolVersion:  "TLS1_3",
			EcdhCurves:              "X25519,P-256",
		}
	}

	tests := []struct {
		name        string
		mutate      func(*XDSServerTLSConfig)
		wantErr     bool
		errContains string
	}{
		{name: "valid config", mutate: func(*XDSServerTLSConfig) {}, wantErr: false},
		{
			name:    "disabled skips all checks",
			mutate:  func(c *XDSServerTLSConfig) { *c = XDSServerTLSConfig{Enabled: false} },
			wantErr: false,
		},
		{
			name:        "missing cert file",
			mutate:      func(c *XDSServerTLSConfig) { c.CertFile = "" },
			wantErr:     true,
			errContains: "cert_file is required",
		},
		{
			name:        "missing key file",
			mutate:      func(c *XDSServerTLSConfig) { c.KeyFile = "" },
			wantErr:     true,
			errContains: "key_file is required",
		},
		{
			name:        "missing client CA file -- xDS has no server-only TLS mode",
			mutate:      func(c *XDSServerTLSConfig) { c.ClientCAFile = "" },
			wantErr:     true,
			errContains: "client_ca_file is required",
		},
		{
			name:        "empty allowed client identities is fail-closed, not a no-op allowlist",
			mutate:      func(c *XDSServerTLSConfig) { c.AllowedClientIdentities = nil },
			wantErr:     true,
			errContains: "allowed_client_identities must list at least one",
		},
		{
			name:        "bad protocol version",
			mutate:      func(c *XDSServerTLSConfig) { c.MinimumProtocolVersion = "TLS9_9" },
			wantErr:     true,
			errContains: "minimum_protocol_version",
		},
		{
			name:        "unsupported cipher",
			mutate:      func(c *XDSServerTLSConfig) { c.Ciphers = "TLS_RSA_WITH_RC4_128_SHA" },
			wantErr:     true,
			errContains: "ciphers",
		},
		{
			name:        "unsupported curve",
			mutate:      func(c *XDSServerTLSConfig) { c.EcdhCurves = "not-a-curve" },
			wantErr:     true,
			errContains: "ecdh_curves",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validCfg()
			tt.mutate(&cfg)
			err := ValidateXDSServerTLS("policy_server.tls", cfg)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuildXDSServerTLSConfig(t *testing.T) {
	dir := t.TempDir()
	_, caKey, caCert := generateXDSTestCA(t)
	caCertPath := filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(caCertPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw}), 0o600))
	serverCertPath, serverKeyPath := writeXDSLeafCert(t, dir, "server", caCert, caKey, true)

	t.Run("builds a working mTLS config", func(t *testing.T) {
		cfg := XDSServerTLSConfig{
			Enabled:                 true,
			CertFile:                serverCertPath,
			KeyFile:                 serverKeyPath,
			ClientCAFile:            caCertPath,
			AllowedClientIdentities: []string{"anything"},
			MinimumProtocolVersion:  "TLS1_2",
			MaximumProtocolVersion:  "TLS1_3",
			EcdhCurves:              "X25519,P-256",
		}
		tlsConfig, err := BuildXDSServerTLSConfig(cfg)
		require.NoError(t, err)
		require.NotNil(t, tlsConfig)

		assert.Len(t, tlsConfig.Certificates, 1)
		assert.NotNil(t, tlsConfig.ClientCAs)
		assert.Equal(t, tls.RequireAndVerifyClientCert, tlsConfig.ClientAuth)
		assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
		assert.Equal(t, uint16(tls.VersionTLS13), tlsConfig.MaxVersion)
	})

	t.Run("PQC hybrid group opt-in is reflected in CurvePreferences", func(t *testing.T) {
		cfg := XDSServerTLSConfig{
			Enabled:                 true,
			CertFile:                serverCertPath,
			KeyFile:                 serverKeyPath,
			ClientCAFile:            caCertPath,
			AllowedClientIdentities: []string{"anything"},
			MinimumProtocolVersion:  "TLS1_2",
			MaximumProtocolVersion:  "TLS1_3",
			EcdhCurves:              "X25519MLKEM768,X25519,P-256",
		}
		tlsConfig, err := BuildXDSServerTLSConfig(cfg)
		require.NoError(t, err)
		require.NotEmpty(t, tlsConfig.CurvePreferences)
		assert.Equal(t, tls.X25519MLKEM768, tlsConfig.CurvePreferences[0])
	})

	t.Run("missing cert file surfaces a clear error", func(t *testing.T) {
		cfg := XDSServerTLSConfig{
			Enabled:                true,
			CertFile:               "/nonexistent/cert.pem",
			KeyFile:                "/nonexistent/key.pem",
			ClientCAFile:           caCertPath,
			MinimumProtocolVersion: "TLS1_2",
			MaximumProtocolVersion: "TLS1_3",
			EcdhCurves:             "X25519",
		}
		_, err := BuildXDSServerTLSConfig(cfg)
		assert.Error(t, err)
	})

	t.Run("missing CA file surfaces a clear error", func(t *testing.T) {
		cfg := XDSServerTLSConfig{
			Enabled:                true,
			CertFile:               serverCertPath,
			KeyFile:                serverKeyPath,
			ClientCAFile:           "/nonexistent/ca.pem",
			MinimumProtocolVersion: "TLS1_2",
			MaximumProtocolVersion: "TLS1_3",
			EcdhCurves:             "X25519",
		}
		_, err := BuildXDSServerTLSConfig(cfg)
		assert.Error(t, err)
	})

	t.Run("garbage CA file content is rejected", func(t *testing.T) {
		badCAPath := filepath.Join(dir, "bad-ca.crt")
		require.NoError(t, os.WriteFile(badCAPath, []byte("not a certificate"), 0o600))
		cfg := XDSServerTLSConfig{
			Enabled:                true,
			CertFile:               serverCertPath,
			KeyFile:                serverKeyPath,
			ClientCAFile:           badCAPath,
			MinimumProtocolVersion: "TLS1_2",
			MaximumProtocolVersion: "TLS1_3",
			EcdhCurves:             "X25519",
		}
		_, err := BuildXDSServerTLSConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no valid certificates")
	})
}

// TestBuildXDSServerTLSConfig_PQCHandshakeNegotiation is a live TLS
// handshake (not just a config-shape assertion) proving BuildXDSServerTLSConfig's
// CurvePreferences is actually honored by crypto/tls's negotiation, and that a
// peer without PQC support still completes the handshake via classical
// fallback -- the exact interoperability guarantee post-quantum-cryptography.md
// requires ("must not hard-fail... when talking to a peer that only speaks
// classical algorithms").
func TestBuildXDSServerTLSConfig_PQCHandshakeNegotiation(t *testing.T) {
	dir := t.TempDir()
	_, caKey, caCert := generateXDSTestCA(t)
	caCertPath := filepath.Join(dir, "ca.crt")
	require.NoError(t, os.WriteFile(caCertPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw}), 0o600))
	serverCertPath, serverKeyPath := writeXDSLeafCert(t, dir, "server", caCert, caKey, true)
	clientCertPath, clientKeyPath := writeXDSLeafCert(t, dir, "client", caCert, caKey, false)

	serverCfg := XDSServerTLSConfig{
		Enabled:                 true,
		CertFile:                serverCertPath,
		KeyFile:                 serverKeyPath,
		ClientCAFile:            caCertPath,
		AllowedClientIdentities: []string{"client"},
		MinimumProtocolVersion:  "TLS1_2",
		MaximumProtocolVersion:  "TLS1_3",
		EcdhCurves:              "X25519MLKEM768,X25519,P-256",
	}
	serverTLSConfig, err := BuildXDSServerTLSConfig(serverCfg)
	require.NoError(t, err)

	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	require.NoError(t, err)
	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(caCert)

	dial := func(t *testing.T, clientCurves []tls.CurveID) tls.CurveID {
		t.Helper()
		ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLSConfig)
		require.NoError(t, err)
		defer ln.Close()

		negotiated := make(chan tls.CurveID, 1)
		errCh := make(chan error, 1)
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				errCh <- err
				return
			}
			defer conn.Close()
			tlsConn := conn.(*tls.Conn)
			if err := tlsConn.HandshakeContext(context.Background()); err != nil {
				errCh <- err
				return
			}
			negotiated <- tlsConn.ConnectionState().CurveID
			errCh <- nil
		}()

		clientTLSConfig := &tls.Config{
			Certificates:     []tls.Certificate{clientCert},
			RootCAs:          clientCAPool,
			ServerName:       "localhost",
			CurvePreferences: clientCurves,
		}
		conn, err := tls.Dial("tcp", ln.Addr().String(), clientTLSConfig)
		require.NoError(t, err)
		defer conn.Close()

		require.NoError(t, <-errCh)
		return <-negotiated
	}

	t.Run("PQC-capable peer negotiates the hybrid group", func(t *testing.T) {
		curve := dial(t, []tls.CurveID{tls.X25519MLKEM768, tls.X25519})
		assert.Equal(t, tls.X25519MLKEM768, curve)
	})

	t.Run("classical-only peer still completes the handshake", func(t *testing.T) {
		curve := dial(t, []tls.CurveID{tls.CurveP256})
		assert.Equal(t, tls.CurveP256, curve)
	})
}

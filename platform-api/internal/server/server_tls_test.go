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
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
)

func testServer() *Server {
	return &Server{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// TLS disabled: no certificate is required, even outside demo mode. This is the
// deployment where an ingress or service-mesh sidecar terminates TLS.
func TestBuildTLSConfig_Disabled_NoCertRequired(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "false")

	tlsConfig, err := testServer().buildTLSConfig(config.TLS{
		Enabled: false,
		CertDir: filepath.Join(t.TempDir(), "does-not-exist"),
	})
	if err != nil {
		t.Fatalf("expected no error when TLS is disabled, got %v", err)
	}
	if tlsConfig != nil {
		t.Fatal("expected a nil tls.Config (plain HTTP) when TLS is disabled")
	}
}

// TLS enabled outside demo mode with no certificates: still a fatal misconfiguration.
func TestBuildTLSConfig_Enabled_NonDemo_MissingCert_Errors(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "false")

	_, err := testServer().buildTLSConfig(config.TLS{
		Enabled: true,
		CertDir: filepath.Join(t.TempDir(), "does-not-exist"),
	})
	if err == nil {
		t.Fatal("expected an error when TLS is enabled outside demo mode without certificates")
	}
}

// TLS enabled in demo mode with no certificates: a self-signed pair is generated.
func TestBuildTLSConfig_Enabled_Demo_GeneratesSelfSigned(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "true")
	certDir := filepath.Join(t.TempDir(), "certs")

	tlsConfig, err := testServer().buildTLSConfig(config.TLS{Enabled: true, CertDir: certDir})
	if err != nil {
		t.Fatalf("expected self-signed generation to succeed in demo mode, got %v", err)
	}
	if tlsConfig == nil || len(tlsConfig.Certificates) != 1 {
		t.Fatal("expected exactly one generated certificate in demo mode")
	}
}

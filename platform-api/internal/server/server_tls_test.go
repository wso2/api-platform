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

// HTTPS listener outside demo mode with no certificates: a fatal misconfiguration.
func TestBuildTLSConfig_NonDemo_MissingCert_Errors(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "false")

	_, err := testServer().buildTLSConfig(config.HTTPSListener{
		Enabled: true,
		CertDir: filepath.Join(t.TempDir(), "does-not-exist"),
	})
	if err == nil {
		t.Fatal("expected an error when the HTTPS listener has no certificates outside demo mode")
	}
}

// HTTPS listener in demo mode with no certificates: a self-signed pair is generated.
func TestBuildTLSConfig_Demo_GeneratesSelfSigned(t *testing.T) {
	t.Setenv("APIP_DEMO_MODE", "true")
	certDir := filepath.Join(t.TempDir(), "certs")

	tlsConfig, err := testServer().buildTLSConfig(config.HTTPSListener{Enabled: true, Port: "9243", CertDir: certDir})
	if err != nil {
		t.Fatalf("expected self-signed generation to succeed in demo mode, got %v", err)
	}
	if tlsConfig == nil || len(tlsConfig.Certificates) != 1 {
		t.Fatal("expected exactly one generated certificate in demo mode")
	}
}

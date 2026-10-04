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

package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// An empty issuer would make the map accept only tokens with no iss, so it is refused.
func TestBuildIssuerKeyMap_RefusesEmptyIssuer(t *testing.T) {
	key := mustRSAKey(t)
	local := &config.Server{}
	local.Auth.Mode = config.AuthModeInternalToken
	local.Auth.InternalToken.SkipValidation = true
	if _, err := buildIssuerKeyMap(local, nil); err == nil || !strings.Contains(err.Error(), "auth.jwt.issuer") {
		t.Fatalf("internal_token with empty issuer: got %v", err)
	}

	idp := &config.Server{}
	idp.Auth.Mode = config.AuthModeIDP
	idp.Auth.IDP.Issuer = []string{"https://idp.example"}
	shared := &service.ServiceAccountKeys{PrivateKey: key, Current: &key.PublicKey}
	if _, err := buildIssuerKeyMap(idp, shared); err == nil || !strings.Contains(err.Error(), "auth.jwt.issuer") {
		t.Fatalf("idp with an empty shared SA issuer: got %v", err)
	}
	idp.Auth.ServiceAccount.Audience = "platform-api"
	if _, err := buildIssuerKeyMap(idp, nil); err != nil {
		t.Fatalf("idp without SA keys needs no auth.jwt.issuer: %v", err)
	}
}

func TestHasServiceAccountRole(t *testing.T) {
	if hasServiceAccountRole(nil) || hasServiceAccountRole(map[string][]string{"ap_admin": nil}) {
		t.Fatal("no ap_sa_ role reported as present")
	}
	if !hasServiceAccountRole(map[string][]string{"ap_admin": nil, "ap_sa_reader": nil}) {
		t.Fatal("ap_sa_reader not found")
	}
}

// signSATestToken mints a real SA token with keys, as /token would.
func signSATestToken(t *testing.T, keys *service.ServiceAccountKeys) string {
	t.Helper()
	cfg := &config.Server{}
	cfg.Auth.ServiceAccount = config.ServiceAccount{TokenTTL: time.Minute, Audience: "platform-api"}
	sa := &model.ServiceAccount{UUID: "0198a1b2-0000-7000-8000-000000000001", Handle: "ci-bot", ClientID: "sa_acme_ci-bot_abc123", TokenVersion: 1}
	tok, err := service.NewSATokenSigner(keys, cfg).Sign(sa, &model.Organization{ID: "org-1", Handle: "acme"}, "ap:rest_api:read")
	if err != nil {
		t.Fatal(err)
	}
	return tok.Token
}

// Each mode registers the right issuers with the right keys: a real SA token
// verifies where it should, and an IdP issuer is never verified locally.
func TestBuildIssuerKeyMap_Modes(t *testing.T) {
	login, sa, retired := mustRSAKey(t), mustRSAKey(t), mustRSAKey(t)
	pubFile := filepath.Join(t.TempDir(), "login.pub")
	der, err := x509.MarshalPKIXPublicKey(&login.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pubFile, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	shared := &service.ServiceAccountKeys{Issuer: "platform-api", PrivateKey: login, Current: &login.PublicKey}
	own := &service.ServiceAccountKeys{Issuer: "platform-api-sa", OwnIssuer: true, PrivateKey: sa, Current: &sa.PublicKey,
		Retired: []*rsa.PublicKey{&retired.PublicKey}}
	// A token signed before the key change, by the now-retired key.
	oldOwn := &service.ServiceAccountKeys{Issuer: "platform-api-sa", PrivateKey: retired, Current: &retired.PublicKey}
	// Signed for the shared issuer by a key the map does not hold.
	forged := &service.ServiceAccountKeys{Issuer: "platform-api", PrivateKey: sa, Current: &sa.PublicKey}

	t.Run("file mode, own SA issuer", func(t *testing.T) {
		cfg := &config.Server{}
		cfg.Auth.Mode = config.AuthModeFile
		cfg.Auth.JWT = config.JWT{Issuer: "platform-api", PublicKeyFile: pubFile}
		cfg.Auth.ServiceAccount.Audience = "platform-api"
		m, err := buildIssuerKeyMap(cfg, own)
		if err != nil {
			t.Fatal(err)
		}
		for name, keys := range map[string]*service.ServiceAccountKeys{"own key": own, "retired key": oldOwn, "auth.jwt key": shared} {
			if _, err := m.VerifyServiceAccountToken(signSATestToken(t, keys)); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
		if _, err := m.VerifyServiceAccountToken(signSATestToken(t, forged)); err == nil {
			t.Error("a key not loaded from public_key_file verified for the local issuer")
		}
	})

	t.Run("missing auth.jwt public key", func(t *testing.T) {
		cfg := &config.Server{}
		cfg.Auth.Mode = config.AuthModeFile
		cfg.Auth.JWT = config.JWT{Issuer: "platform-api", PublicKeyFile: filepath.Join(t.TempDir(), "missing.pub")}
		if _, err := buildIssuerKeyMap(cfg, shared); err == nil || !strings.Contains(err.Error(), "public_key_file") {
			t.Fatalf("got %v", err)
		}
	})

	// skip_validation needs no auth.jwt key file; the shared SA key still verifies.
	t.Run("skip_validation, shared key", func(t *testing.T) {
		cfg := &config.Server{}
		cfg.Auth.Mode = config.AuthModeInternalToken
		cfg.Auth.InternalToken.SkipValidation = true
		cfg.Auth.JWT.Issuer = "platform-api"
		cfg.Auth.ServiceAccount.Audience = "platform-api"
		m, err := buildIssuerKeyMap(cfg, shared)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := m.VerifyServiceAccountToken(signSATestToken(t, shared)); err != nil {
			t.Fatalf("shared-key SA token: %v", err)
		}
	})

	t.Run("idp mode, shared key", func(t *testing.T) {
		cfg := &config.Server{}
		cfg.Auth.Mode = config.AuthModeIDP
		cfg.Auth.IDP.Issuer = []string{"https://idp.example"}
		cfg.Auth.ServiceAccount.Audience = "platform-api"
		m, err := buildIssuerKeyMap(cfg, shared)
		if err != nil {
			t.Fatal(err)
		}
		if m.IsLocal("https://idp.example") || !m.IsServiceAccountToken("platform-api", "anything") {
			t.Fatal("the shared issuer must be the SA kind, the IdP issuer never local")
		}
		if _, err := m.VerifyServiceAccountToken(signSATestToken(t, shared)); err != nil {
			t.Fatalf("shared-key SA token: %v", err)
		}
	})
}

func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

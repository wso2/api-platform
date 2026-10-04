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
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// An empty issuer would make the map accept only tokens with no iss, so it is refused.
func TestBuildIssuerKeyMap_RefusesEmptyIssuer(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
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

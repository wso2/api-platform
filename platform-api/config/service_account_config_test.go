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

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateServiceAccountConfig(t *testing.T) {
	dir := t.TempDir()
	saPub, saPriv := genRSAKeyPEMs()
	saPubFile, saPrivFile := writePEMFile(dir, "sa_public.pem", saPub), writePEMFile(dir, "sa_private.pem", saPriv)
	oldPub, _ := genRSAKeyPEMs()
	oldPubFile := writePEMFile(dir, "sa_old_public.pem", oldPub)

	ownKey := func() JWT {
		return JWT{Issuer: "platform-api-sa", PublicKeyFile: saPubFile, PrivateKeyFile: saPrivFile}
	}
	tests := []struct {
		name    string
		mutate  func(a *Auth)
		wantErr string
	}{
		{"defaults are valid", func(a *Auth) {}, ""},
		{"own key is valid", func(a *Auth) { a.ServiceAccount.JWT = ownKey() }, ""},
		{"own key with a retired key", func(a *Auth) {
			a.ServiceAccount.JWT = ownKey()
			a.ServiceAccount.RetiredPublicKeyFiles = []string{oldPubFile}
		}, ""},
		{"zero token_ttl", func(a *Auth) { a.ServiceAccount.TokenTTL = 0 }, "token_ttl"},
		{"blank audience", func(a *Auth) { a.ServiceAccount.Audience = " " }, "audience"},
		{"zero poll_interval", func(a *Auth) { a.ServiceAccount.Revocation.PollInterval = 0 }, "poll_interval"},
		{"poll_interval too small to jitter", func(a *Auth) { a.ServiceAccount.Revocation.PollInterval = time.Nanosecond }, "poll_interval"},
		{"issuer without keys", func(a *Auth) { a.ServiceAccount.JWT = JWT{Issuer: "platform-api-sa"} }, "together"},
		{"keys without issuer", func(a *Auth) {
			a.ServiceAccount.JWT = JWT{PublicKeyFile: saPubFile, PrivateKeyFile: saPrivFile}
		}, "together"},
		{"issuer shared with auth.jwt", func(a *Auth) {
			a.ServiceAccount.JWT = ownKey()
			a.ServiceAccount.JWT.Issuer = a.JWT.Issuer
		}, "must differ from auth.jwt.issuer"},
		{"retired keys on the shared key", func(a *Auth) { a.ServiceAccount.RetiredPublicKeyFiles = []string{oldPubFile} }, "requires [auth.service_account.jwt]"},
		{"unreadable retired key", func(a *Auth) {
			a.ServiceAccount.JWT = ownKey()
			a.ServiceAccount.RetiredPublicKeyFiles = []string{dir + "/missing.pem"}
		}, "retired_public_key_files"},
		{"retired key equal to the current one", func(a *Auth) {
			a.ServiceAccount.JWT = ownKey()
			a.ServiceAccount.RetiredPublicKeyFiles = []string{saPubFile}
		}, "is the current signing key"},
		{"disabled skips every check", func(a *Auth) {
			a.ServiceAccount.Enabled = false
			a.ServiceAccount.TokenTTL = 0
			a.ServiceAccount.JWT = JWT{Issuer: "platform-api-sa"}
		}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := Auth{
				Mode:           AuthModeInternalToken,
				JWT:            JWT{Issuer: "platform-api", PublicKeyFile: validJWTPublicKeyFile},
				Authorization:  Authorization{Mode: AuthzModeScope},
				ServiceAccount: defaultConfig().Auth.ServiceAccount,
			}
			auth.ServiceAccount.Enabled = true
			tt.mutate(&auth)
			err := validateAuthConfig(&auth)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestFileUsersCannotUseTheServiceAccountPrefix(t *testing.T) {
	err := validateFileBasedConfig(&FileBased{
		Organization: FileBasedOrg{ID: "default", DisplayName: "Default"},
		Users:        FileBasedUsers{{Username: "sa:acme:bot:x", PasswordHash: "h", Roles: []string{"ap_admin"}}},
	}, &Authorization{RoleToScopeMapping: "/m.yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved for service accounts")
}

func TestFileUsersCannotHoldServiceAccountRoles(t *testing.T) {
	err := validateFileBasedConfig(&FileBased{
		Organization: FileBasedOrg{ID: "default", DisplayName: "Default"},
		Users:        FileBasedUsers{{Username: "alice", PasswordHash: "h", Roles: []string{"ap_sa_reader"}}},
	}, &Authorization{RoleToScopeMapping: "/m.yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service-account role")
}

func TestServiceAccountsDisabledByDefault(t *testing.T) {
	assert.False(t, defaultConfig().Auth.ServiceAccount.Enabled)
}

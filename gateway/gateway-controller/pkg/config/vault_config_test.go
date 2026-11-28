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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validResolver() VaultResolverConfig {
	return VaultResolverConfig{
		Name:    "amp-shared",
		Type:    "openbao",
		Address: "https://openbao.amp.svc:8200",
		Auth:    VaultAuthConfig{TokenFile: "/var/run/secrets/openbao/token"},
	}
}

func TestValidateVaultConfig_NoResolversIsValid(t *testing.T) {
	cfg := validConfig()
	assert.NoError(t, cfg.validateVaultConfig())
}

func TestValidateVaultConfig_Valid(t *testing.T) {
	cfg := validConfig()
	kube := validResolver()
	kube.Name = "customer"
	kube.Auth = VaultAuthConfig{Kubernetes: VaultKubernetesAuthConfig{Role: "gw", MountPath: "k8s"}}
	kube.Mount = "kv"
	kube.Namespace = "team-a"
	cfg.Controller.Vault.Resolvers = []VaultResolverConfig{validResolver(), kube}
	assert.NoError(t, cfg.validateVaultConfig())
}

func TestValidateVaultConfig_Invalid(t *testing.T) {
	tests := map[string]func(r *VaultResolverConfig){
		"bad name":           func(r *VaultResolverConfig) { r.Name = "AMP" },
		"empty name":         func(r *VaultResolverConfig) { r.Name = "" },
		"unsupported type":   func(r *VaultResolverConfig) { r.Type = "aws" },
		"missing address":    func(r *VaultResolverConfig) { r.Address = "" },
		"non-http address":   func(r *VaultResolverConfig) { r.Address = "tcp://x:8200" },
		"address with query": func(r *VaultResolverConfig) { r.Address = "https://x:8200?a=b" },
		"address with user":  func(r *VaultResolverConfig) { r.Address = "https://u:p@x:8200" },
		"bad mount":          func(r *VaultResolverConfig) { r.Mount = "a/../b" },
		"bad namespace":      func(r *VaultResolverConfig) { r.Namespace = "a//b" },
		"negative timeout":   func(r *VaultResolverConfig) { r.RequestTimeout = -time.Second },
		"no auth":            func(r *VaultResolverConfig) { r.Auth = VaultAuthConfig{} },
		"bad kube mount path": func(r *VaultResolverConfig) {
			r.Auth = VaultAuthConfig{Kubernetes: VaultKubernetesAuthConfig{Role: "r", MountPath: ".."}}
		},
		"both auth methods": func(r *VaultResolverConfig) {
			r.Auth.Kubernetes = VaultKubernetesAuthConfig{Role: "r"}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := validConfig()
			r := validResolver()
			mutate(&r)
			cfg.Controller.Vault.Resolvers = []VaultResolverConfig{r}
			assert.Error(t, cfg.validateVaultConfig())
		})
	}
}

func TestValidateVaultConfig_DuplicateNames(t *testing.T) {
	cfg := validConfig()
	cfg.Controller.Vault.Resolvers = []VaultResolverConfig{validResolver(), validResolver()}
	err := cfg.validateVaultConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than once")
}

func TestLoadConfig_VaultResolvers(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	contents := `
[[controller.vault.resolvers]]
name = "amp-shared"
type = "openbao"
address = '{{ env "APIP_GW_CONTROLLER_VAULT_ADDR" }}'
mount = "secret"
request_timeout = "5s"

[controller.vault.resolvers.auth]
token_file = "/var/run/secrets/openbao/token"
`
	require.NoError(t, os.WriteFile(configPath, []byte(contents), 0o644))
	t.Setenv("APIP_GW_CONTROLLER_VAULT_ADDR", "https://openbao.amp.svc:8200")

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.Len(t, cfg.Controller.Vault.Resolvers, 1)
	r := cfg.Controller.Vault.Resolvers[0]
	assert.Equal(t, "amp-shared", r.Name)
	assert.Equal(t, "openbao", r.Type)
	assert.Equal(t, "https://openbao.amp.svc:8200", r.Address)
	assert.Equal(t, 5*time.Second, r.RequestTimeout)
	assert.Equal(t, "/var/run/secrets/openbao/token", r.Auth.TokenFile)
}

func TestLoadConfig_InvalidVaultResolverFails(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	contents := `
[[controller.vault.resolvers]]
name = "amp-shared"
type = "openbao"
address = "https://openbao:8200"
`
	require.NoError(t, os.WriteFile(configPath, []byte(contents), 0o644))

	_, err := LoadConfig(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "controller.vault.resolvers[0].auth")
}

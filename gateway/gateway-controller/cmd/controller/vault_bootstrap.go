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

package main

import (
	"fmt"
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/vault"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/vault/openbao"
)

// buildSecretVault wraps the encrypted secret store as the default DB adapter
// and adds one adapter per configured external resolver. secretsService may be
// nil when no encryption provider is configured.
func buildSecretVault(cfg config.VaultConfig, secretsService *secrets.SecretService, log *slog.Logger) (*vault.Vault, error) {
	var local *vault.DBAdapter
	if secretsService != nil {
		local = vault.NewDBAdapter(secretsService)
	}

	resolvers := make(map[string]vault.Adapter, len(cfg.Resolvers))
	for _, r := range cfg.Resolvers {
		switch r.Type {
		case openbao.Type:
			var kube *openbao.KubernetesAuthConfig
			if r.Auth.Kubernetes.Role != "" {
				kube = &openbao.KubernetesAuthConfig{
					Role:      r.Auth.Kubernetes.Role,
					MountPath: r.Auth.Kubernetes.MountPath,
					JWTFile:   r.Auth.Kubernetes.JWTFile,
				}
			}
			kv, err := openbao.New(openbao.Config{
				Name:           r.Name,
				Address:        r.Address,
				Mount:          r.Mount,
				Namespace:      r.Namespace,
				CAFile:         r.CAFile,
				TLSSkipVerify:  r.TLSSkipVerify,
				RequestTimeout: r.RequestTimeout,
				TokenFile:      r.Auth.TokenFile,
				Kubernetes:     kube,
			}, log)
			if err != nil {
				return nil, fmt.Errorf("secret resolver %q: %w", r.Name, err)
			}
			resolvers[r.Name] = kv
		default:
			return nil, fmt.Errorf("secret resolver %q has unsupported type %q", r.Name, r.Type)
		}
	}

	v := vault.New(local, resolvers, log)
	log.Info("Gateway secret vault initialized",
		slog.Bool("local_store", local != nil),
		slog.Any("resolvers", v.ResolverNames()),
	)
	return v, nil
}

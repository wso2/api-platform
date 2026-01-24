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

package xds

import (
	"fmt"
	"log/slog"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/certstore"
)

const (
	// SecretNameUpstreamCA is the name of the SDS secret for upstream CA certificates
	SecretNameUpstreamCA = "upstream_ca_bundle"
)

// SDSSecretManager manages SDS secrets for TLS certificates
type SDSSecretManager struct {
	cache     cache.SnapshotCache
	certStore *certstore.CertStore
	logger    *slog.Logger
	nodeID    string
}

// NewSDSSecretManager creates a new SDS secret manager
// It shares the same cache and node ID as the main xDS to ensure Envoy can fetch secrets
func NewSDSSecretManager(certStore *certstore.CertStore, cache cache.SnapshotCache, nodeID string, logger *slog.Logger) *SDSSecretManager {
	return &SDSSecretManager{
		cache:     cache,
		certStore: certStore,
		logger:    logger,
		nodeID:    nodeID,
	}
}

// GetCache returns the SDS snapshot cache
func (sm *SDSSecretManager) GetCache() cache.SnapshotCache {
	return sm.cache
}

// UpdateSecrets creates and updates the SDS snapshot with certificate secrets
// This now updates the main xDS snapshot instead of a separate SDS snapshot
func (sm *SDSSecretManager) UpdateSecrets() error {
	if sm.certStore == nil {
		sm.logger.Warn("No cert store available, skipping SDS secret update")
		return nil
	}

	// Secrets are now managed as part of the main xDS snapshot
	// This method just validates that cert store is ready
	combinedCerts := sm.certStore.GetCombinedCertificates()
	if len(combinedCerts) == 0 {
		sm.logger.Warn("No certificates available in cert store")
		return nil
	}

	sm.logger.Info("Certificate store ready for SDS",
		slog.Int("cert_bytes", len(combinedCerts)),
	)

	return nil
}

// GetSecret creates the SDS secret resource for inclusion in xDS snapshot
func (sm *SDSSecretManager) GetSecret() (types.Resource, error) {
	if sm.certStore == nil {
		return nil, fmt.Errorf("no cert store available")
	}

	// Get combined certificates from cert store
	combinedCerts := sm.certStore.GetCombinedCertificates()
	if len(combinedCerts) == 0 {
		return nil, fmt.Errorf("no certificates available in cert store")
	}

	// Create SDS secret for upstream CA certificates
	secret := &tlsv3.Secret{
		Name: SecretNameUpstreamCA,
		Type: &tlsv3.Secret_ValidationContext{
			ValidationContext: &tlsv3.CertificateValidationContext{
				TrustedCa: &core.DataSource{
					Specifier: &core.DataSource_InlineBytes{
						InlineBytes: combinedCerts,
					},
				},
			},
		},
	}

	return secret, nil
}

// GetNodeID returns the node ID for SDS clients
func (sm *SDSSecretManager) GetNodeID() string {
	return sm.nodeID
}

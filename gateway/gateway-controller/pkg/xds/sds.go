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
	"os"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/certstore"
)

const (
	// SecretNameUpstreamCA is the name of the SDS secret for upstream CA certificates
	SecretNameUpstreamCA = "upstream_ca_bundle"

	// SecretNameDownstreamClientCA is the SDS secret carrying the client
	// authority pool the HTTPS listener validates client certificates against.
	SecretNameDownstreamClientCA = "downstream_client_ca"

	// SecretNameDownstreamListenerCert is the SDS secret carrying the HTTPS
	// listener's certificate and key, which keeps the private key out of the
	// Listener resource.
	SecretNameDownstreamListenerCert = "downstream_listener_cert"
)

// SDSSecretManager manages SDS secrets for TLS certificates
type SDSSecretManager struct {
	cache     cache.SnapshotCache
	certStore *certstore.CertStore
	logger    *slog.Logger
	nodeID    string

	// With httpsEnabled false the listener certificate files are never read.
	listenerCertPath string
	listenerKeyPath  string
	httpsEnabled     bool
}

// UpstreamTLSSecretRef describes one upstream definition's mTLS wiring: the
// gateway identity its cluster presents and the certificates that replace
// the gateway-wide trust bundle for it.
type UpstreamTLSSecretRef struct {
	// IdentityName is the gateway identity to present, or "" for none.
	IdentityName string
	// APIHandle and DefinitionName together name the per-upstream
	// ValidationContext secret (UpstreamCAValidationContextSecretName).
	APIHandle      string
	DefinitionName string
	// TrustedCANames lists the usage: upstream certificates trusted for this
	// definition. Empty means the gateway-wide bundle applies.
	TrustedCANames []string
}

// SDS secret-name prefixes for per-identity and per-definition secrets.
const (
	SecretNamePrefixGatewayIdentity = "gateway_identity:"
	SecretNamePrefixUpstreamCA      = "upstream_ca:"
)

// GatewayIdentitySecretName builds the SDS secret name carrying a gateway
// identity's certificate chain and private key.
func GatewayIdentitySecretName(identityName string) string {
	return SecretNamePrefixGatewayIdentity + identityName
}

// UpstreamCAValidationContextSecretName builds the SDS secret name for one
// upstream definition's trust bundle, scoped by API handle so two APIs'
// same-named definitions never collide.
func UpstreamCAValidationContextSecretName(apiHandle, definitionName string) string {
	return SecretNamePrefixUpstreamCA + apiHandle + ":" + definitionName
}

// NewSDSSecretManager creates a new SDS secret manager, sharing the same
// cache and node ID as the main xDS so Envoy can fetch secrets.
func NewSDSSecretManager(certStore *certstore.CertStore, cache cache.SnapshotCache, nodeID string, logger *slog.Logger,
	listenerCertPath, listenerKeyPath string, httpsEnabled bool) *SDSSecretManager {
	return &SDSSecretManager{
		cache:            cache,
		certStore:        certStore,
		logger:           logger,
		nodeID:           nodeID,
		listenerCertPath: listenerCertPath,
		listenerKeyPath:  listenerKeyPath,
		httpsEnabled:     httpsEnabled,
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

// GetSecrets builds every SDS secret this manager can serve. An empty bundle
// is omitted. A failure to load the client-CA pool or the listener
// certificate fails the whole snapshot, since every API shares that
// listener. A failure on a per-cluster identity or trust secret skips only
// that secret, so one API's missing certificate cannot break the others.
func (sm *SDSSecretManager) GetSecrets(upstreamTLSRefs []UpstreamTLSSecretRef) ([]types.Resource, error) {
	var secrets []types.Resource

	if upstreamSecret, err := sm.GetSecret(); err != nil {
		sm.logger.Debug("upstream_ca_bundle secret not currently available", slog.Any("error", err))
	} else {
		secrets = append(secrets, upstreamSecret)
	}

	clientCABundle, err := sm.certStore.GetClientCABundle()
	if err != nil {
		// Omitting the secret would leave the listener waiting on it
		// forever with nothing surfacing the failure.
		sm.logger.Error("Failed to load client-CA pool for downstream_client_ca secret", slog.Any("error", err))
		return nil, fmt.Errorf("failed to load client-CA pool: %w", err)
	}
	if len(clientCABundle) > 0 {
		secrets = append(secrets, &tlsv3.Secret{
			Name: SecretNameDownstreamClientCA,
			Type: &tlsv3.Secret_ValidationContext{
				ValidationContext: &tlsv3.CertificateValidationContext{
					TrustedCa: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{
							InlineBytes: clientCABundle,
						},
					},
					// Envoy still verifies the chain and reports the
					// result as connection.peer_certificate_valid;
					// ACCEPT_UNTRUSTED only keeps a failed handshake from
					// closing the connection, so mtls-auth can return a
					// 401. mtls-auth must deny on a false verdict.
					TrustChainVerification: tlsv3.CertificateValidationContext_ACCEPT_UNTRUSTED,
				},
			},
		})
	}

	// One secret per distinct gateway identity. A failed lookup skips only
	// that secret: its cluster still names it, so its connections fail
	// rather than fall back to presenting no identity.
	seenIdentities := make(map[string]bool, len(upstreamTLSRefs))
	for _, ref := range upstreamTLSRefs {
		if ref.IdentityName == "" || seenIdentities[ref.IdentityName] {
			continue
		}
		seenIdentities[ref.IdentityName] = true

		certChain, privateKey, err := sm.certStore.GetGatewayIdentityMaterial(ref.IdentityName)
		if err != nil {
			sm.logger.Error("Failed to load gateway identity material for SDS secret; skipping this secret only",
				slog.String("identity", ref.IdentityName), slog.Any("error", err))
			continue
		}
		secrets = append(secrets, &tlsv3.Secret{
			Name: GatewayIdentitySecretName(ref.IdentityName),
			Type: &tlsv3.Secret_TlsCertificate{
				TlsCertificate: &tlsv3.TlsCertificate{
					CertificateChain: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: certChain},
					},
					PrivateKey: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: privateKey},
					},
				},
			},
		})
	}

	// One secret per definition that sets trustedCAs, deduped by name
	// because a definition can back more than one cluster. A failed
	// lookup skips only that secret.
	seenValidationContexts := make(map[string]bool, len(upstreamTLSRefs))
	for _, ref := range upstreamTLSRefs {
		if len(ref.TrustedCANames) == 0 {
			continue
		}
		secretName := UpstreamCAValidationContextSecretName(ref.APIHandle, ref.DefinitionName)
		if seenValidationContexts[secretName] {
			continue
		}
		seenValidationContexts[secretName] = true

		bundle, err := sm.certStore.GetUpstreamTrustBundle(ref.TrustedCANames)
		if err != nil {
			sm.logger.Error("Failed to load per-upstream trust bundle for SDS secret; skipping this secret only",
				slog.String("api_handle", ref.APIHandle),
				slog.String("definition", ref.DefinitionName),
				slog.Any("error", err))
			continue
		}
		secrets = append(secrets, &tlsv3.Secret{
			Name: secretName,
			Type: &tlsv3.Secret_ValidationContext{
				ValidationContext: &tlsv3.CertificateValidationContext{
					TrustedCa: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: bundle},
					},
				},
			},
		})
	}

	if sm.httpsEnabled {
		certBytes, err := os.ReadFile(sm.listenerCertPath)
		if err != nil {
			sm.logger.Error("Failed to read HTTPS listener certificate for downstream_listener_cert secret",
				slog.String("path", sm.listenerCertPath), slog.Any("error", err))
			return nil, fmt.Errorf("failed to read HTTPS listener certificate: %w", err)
		}
		keyBytes, err := os.ReadFile(sm.listenerKeyPath)
		if err != nil {
			sm.logger.Error("Failed to read HTTPS listener private key for downstream_listener_cert secret",
				slog.String("path", sm.listenerKeyPath), slog.Any("error", err))
			return nil, fmt.Errorf("failed to read HTTPS listener private key: %w", err)
		}
		secrets = append(secrets, &tlsv3.Secret{
			Name: SecretNameDownstreamListenerCert,
			Type: &tlsv3.Secret_TlsCertificate{
				TlsCertificate: &tlsv3.TlsCertificate{
					CertificateChain: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: certBytes},
					},
					PrivateKey: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: keyBytes},
					},
				},
			},
		})
	}

	return secrets, nil
}

// GetNodeID returns the node ID for SDS clients
func (sm *SDSSecretManager) GetNodeID() string {
	return sm.nodeID
}

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

package xds

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/gatewayidentity"
)

// defaultClientCertificate is the certificate presented to an HTTPS backend
// whose upstream definition names no tls identity. A zero value presents
// none.
type defaultClientCertificate struct {
	// SecretName is the SDS secret carrying the certificate and key.
	SecretName string
	// IdentityName is the role: default gateway identity, or "" when the
	// HTTPS listener certificate is presented.
	IdentityName string
	// leafPEM is the presented certificate chain, read only for logging.
	leafPEM []byte
	// material is the default identity's loaded chain and key, which SDS
	// serves as is. It is nil unless IdentityName is set.
	material *identityMaterial
}

// identityMaterial is a gateway identity's PEM certificate chain and
// decrypted private key.
type identityMaterial struct {
	certChain  []byte
	privateKey []byte
}

// resolveDefaultClientCertificate picks the certificate by precedence: the
// role: default gateway identity, then the HTTPS listener certificate, then
// none. With present_default_identity off it always picks none. A default
// identity whose key cannot be loaded is skipped, so no cluster names a
// secret SDS cannot serve.
func (t *Translator) resolveDefaultClientCertificate() (defaultClientCertificate, error) {
	if !t.routerConfig.Upstream.TLS.PresentDefaultIdentity {
		return defaultClientCertificate{}, nil
	}
	if t.certStore != nil {
		identity, err := t.certStore.GetDefaultGatewayIdentity()
		if err != nil {
			return defaultClientCertificate{}, fmt.Errorf("failed to resolve the default gateway identity: %w", err)
		}
		if identity != nil {
			certChain, privateKey, err := t.certStore.GetGatewayIdentityMaterial(identity.Name)
			if err == nil {
				return defaultClientCertificate{
					SecretName:   GatewayIdentitySecretName(identity.Name),
					IdentityName: identity.Name,
					leafPEM:      certChain,
					material:     &identityMaterial{certChain: certChain, privateKey: privateKey},
				}, nil
			}
			t.logger.Error("Failed to load the default gateway identity; presenting the next choice instead",
				slog.String("identity", identity.Name), slog.Any("error", err))
		}
	}
	if t.routerConfig.HTTPSEnabled {
		// A read failure only affects logging; the SDS secret build reports it.
		leafPEM, _ := os.ReadFile(t.routerConfig.DownstreamTLS.CertPath)
		return defaultClientCertificate{
			SecretName: SecretNameDownstreamListenerCert,
			leafPEM:    leafPEM,
		}, nil
	}
	return defaultClientCertificate{}, nil
}

// logDefaultClientCertificate logs which certificate backends are presented,
// once at startup and again whenever the choice or its certificate changes.
func (t *Translator) logDefaultClientCertificate(presented defaultClientCertificate) {
	if !t.routerConfig.Upstream.TLS.PresentDefaultIdentity {
		return
	}
	digest := sha256.Sum256(presented.leafPEM)
	key := presented.SecretName + "|" + hex.EncodeToString(digest[:])
	if t.loggedDefaultClientCertificate == key {
		return
	}
	t.loggedDefaultClientCertificate = key

	var source []any
	switch {
	case presented.IdentityName != "":
		source = []any{slog.String("identity", presented.IdentityName)}
		t.logger.Info("Presenting the default gateway identity to backends whose upstream definition names no tls identity",
			source...)
	case presented.SecretName == SecretNameDownstreamListenerCert:
		source = []any{slog.String("identity", "the HTTPS listener certificate")}
		t.logger.Info("Presenting the HTTPS listener certificate to backends whose upstream definition names no tls identity",
			source...)
	default:
		t.logger.Warn("present_default_identity is on, but no client certificate is presented to backends: " +
			"no gateway identity has role: default and the HTTPS listener is disabled")
		return
	}

	leaf := firstPEMCertificate(presented.leafPEM)
	if leaf == nil {
		return
	}
	if warning := gatewayidentity.ClientAuthWarning(leaf); warning != nil {
		t.logger.Warn("The certificate presented to backends does not assert the clientAuth extended key usage; "+
			"backends that check it will refuse it",
			append(source, slog.String("subject", leaf.Subject.String()))...)
	}
}

// firstPEMCertificate parses the first CERTIFICATE block of data, or returns
// nil when there is none.
func firstPEMCertificate(data []byte) *x509.Certificate {
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return nil
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil
		}
		return cert
	}
}

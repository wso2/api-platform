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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
)

// XDSServerTLSConfig holds mutual-TLS configuration for an xDS gRPC server:
// the main Envoy-facing ADS/SDS server on server.xds_port, and the
// policy-engine-facing server on policy_server.port. Off by default -- both
// servers keep working in plaintext either way, consistent with this
// repo's "PQC/TLS is optional-but-supported, not mandatory" posture (see
// post-quantum-cryptography.md), since not every deployment's Envoy or
// policy-engine build is configured for mTLS yet.
//
// Unlike ServerTLSConfig (the REST management API's TLS listener, which is
// server-only TLS), this type has no server-only mode: xDS is a
// control-plane channel that carries per-tenant API-key hashes,
// subscription state, and full policy chains, so authenticating only the
// server side is not sufficient (go-control-plane-xds-security.md
// directive 2). Whenever Enabled is true, ClientCAFile and
// AllowedClientIdentities are both required -- see ValidateXDSServerTLS.
type XDSServerTLSConfig struct {
	// Enabled switches the xDS server from plaintext to mutual TLS on its
	// existing port (server.xds_port or policy_server.port) -- there is no
	// second listener the way ServerTLSConfig adds one for the REST API,
	// since a gRPC server serves one credential type per port.
	Enabled bool `koanf:"enabled"`

	// CertFile and KeyFile are the PEM-encoded server certificate and
	// private key this xDS server presents to connecting clients. Required
	// when Enabled.
	CertFile string `koanf:"cert_file"`
	KeyFile  string `koanf:"key_file"`

	// ClientCAFile is a PEM bundle of CA certificates trusted to sign a
	// connecting client's certificate (Envoy's or the policy-engine's).
	// Required when Enabled -- this is what makes the handshake mutual
	// rather than server-only.
	ClientCAFile string `koanf:"client_ca_file"`

	// AllowedClientIdentities is an explicit allowlist of accepted peer
	// certificate identities: a certificate's first SAN URI (e.g. a SPIFFE
	// ID) if present, otherwise its Subject CommonName -- see
	// pkg/tlsauth.PeerIdentity. A client certificate that chains to a
	// trusted CA is not by itself authorization to reach this snapshot; at
	// least one identity is required when Enabled, so this can't be
	// silently left as a no-op allowlist (go-control-plane-xds-security.md
	// directive 2).
	AllowedClientIdentities []string `koanf:"allowed_client_identities"`

	// MinimumProtocolVersion and MaximumProtocolVersion bound the
	// negotiated TLS version: one of "TLS1_0", "TLS1_1", "TLS1_2", "TLS1_3".
	// Same vocabulary as ServerTLSConfig/router.downstream_tls for
	// consistency within this file.
	MinimumProtocolVersion string `koanf:"minimum_protocol_version"`
	MaximumProtocolVersion string `koanf:"maximum_protocol_version"`

	// Ciphers is a comma-separated list of Go crypto/tls cipher suite names
	// restricting which suites this server negotiates. Empty by default --
	// Go's own secure default set/order applies. Only affects TLS 1.2 and
	// below; TLS 1.3 suite selection is not configurable in Go's crypto/tls.
	Ciphers string `koanf:"ciphers"`

	// EcdhCurves is a comma-separated list of TLS 1.3 key-exchange groups,
	// most preferred first. Defaults to the hybrid post-quantum group
	// ("X25519MLKEM768", FIPS 203 ML-KEM-768 + X25519) first, with classical
	// fallbacks after -- this server is Go's own crypto/tls (1.23+
	// implements X25519MLKEM768 natively), so an Envoy/policy-engine peer
	// that doesn't support the group simply falls back to a later classical
	// entry in this same list.
	EcdhCurves string `koanf:"ecdh_curves"`
}

// ValidateXDSServerTLS validates an XDSServerTLSConfig block, a no-op when
// Enabled is false. fieldPrefix is the dotted config path used in error
// messages (e.g. "server.xds_tls" or "policy_server.tls").
func ValidateXDSServerTLS(fieldPrefix string, cfg XDSServerTLSConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.CertFile == "" {
		return fmt.Errorf("%s.cert_file is required when %s.enabled", fieldPrefix, fieldPrefix)
	}
	if cfg.KeyFile == "" {
		return fmt.Errorf("%s.key_file is required when %s.enabled", fieldPrefix, fieldPrefix)
	}
	if cfg.ClientCAFile == "" {
		return fmt.Errorf("%s.client_ca_file is required when %s.enabled -- xDS requires mutual TLS, server-only TLS is not offered for this server", fieldPrefix, fieldPrefix)
	}
	if len(cfg.AllowedClientIdentities) == 0 {
		return fmt.Errorf("%s.allowed_client_identities must list at least one accepted peer identity when %s.enabled", fieldPrefix, fieldPrefix)
	}
	for _, identity := range cfg.AllowedClientIdentities {
		if strings.TrimSpace(identity) == "" {
			return fmt.Errorf("%s.allowed_client_identities must not contain blank identities", fieldPrefix)
		}
	}
	if err := ValidateServerTLSVersions(cfg.MinimumProtocolVersion, cfg.MaximumProtocolVersion); err != nil {
		return fmt.Errorf("%s: %w", fieldPrefix, err)
	}
	if _, err := ParseServerCiphers(cfg.Ciphers); err != nil {
		return fmt.Errorf("%s.ciphers: %w", fieldPrefix, err)
	}
	if _, err := ParseServerEcdhCurves(cfg.EcdhCurves); err != nil {
		return fmt.Errorf("%s.ecdh_curves: %w", fieldPrefix, err)
	}
	return nil
}

// BuildXDSServerTLSConfig turns a validated XDSServerTLSConfig into a
// *tls.Config enforcing mutual TLS: the server's own certificate, plus a
// client CA pool used to require and verify a client certificate
// (tls.RequireAndVerifyClientCert). This only performs the TLS handshake --
// checking the verified peer's identity against AllowedClientIdentities is
// a separate authorization step the xDS server's stream callbacks must
// still perform (see pkg/tlsauth.VerifyStreamPeer); a certificate chaining
// to a trusted CA is not by itself authorization to reach this snapshot.
//
// Callers should run ValidateXDSServerTLS first; this function re-validates
// version/cipher/curve fields defensively but does not check
// AllowedClientIdentities, which it never reads.
func BuildXDSServerTLSConfig(cfg XDSServerTLSConfig) (*tls.Config, error) {
	if err := ValidateServerTLSVersions(cfg.MinimumProtocolVersion, cfg.MaximumProtocolVersion); err != nil {
		return nil, err
	}
	minVersion, _ := ParseServerTLSVersion(cfg.MinimumProtocolVersion)
	maxVersion, _ := ParseServerTLSVersion(cfg.MaximumProtocolVersion)

	cipherSuites, err := ParseServerCiphers(cfg.Ciphers)
	if err != nil {
		return nil, err
	}

	curves, err := ParseServerEcdhCurves(cfg.EcdhCurves)
	if err != nil {
		return nil, err
	}

	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("loading xDS server certificate: %w", err)
	}

	caPEM, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("reading xDS client CA file: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("no valid certificates found in xDS client CA file %q", cfg.ClientCAFile)
	}

	return &tls.Config{
		Certificates:     []tls.Certificate{cert},
		ClientCAs:        clientCAs,
		ClientAuth:       tls.RequireAndVerifyClientCert,
		MinVersion:       minVersion,
		MaxVersion:       maxVersion,
		CipherSuites:     cipherSuites, // nil == Go's own secure default set/order
		CurvePreferences: curves,
	}, nil
}

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

package models

import "time"

// Certificate usage values. The three purposes never share a trust bundle,
// and an identity row is never a trust anchor.
const (
	// CertificateUsageUpstream marks a certificate as backend/upstream trust.
	CertificateUsageUpstream = "upstream"

	// CertificateUsageDownstream marks a certificate as a pooled client
	// certificate authority, used to authenticate API callers over mTLS.
	CertificateUsageDownstream = "downstream"

	// CertificateUsageIdentity marks a row as a gateway identity: a chain and
	// encrypted private key the gateway presents to a backend requiring
	// mutual TLS.
	CertificateUsageIdentity = "identity"
)

// Certificate role values. client and relay apply to usage: downstream
// certificates and describe how the gateway uses the authority; default
// applies to usage: identity certificates.
const (
	// CertificateRoleClient is the default role: the authority validates a
	// client certificate presented on the connection.
	CertificateRoleClient = "client"

	// CertificateRoleRelay marks an entry as a front proxy whose connection
	// vouches for a client certificate relayed in a header.
	CertificateRoleRelay = "relay"

	// CertificateRoleDefault marks the one gateway identity presented to an
	// HTTPS backend whose upstream definition names no tls identity, when
	// router.upstream.tls.present_default_identity is on.
	CertificateRoleDefault = "default"
)

// CertificateMatch narrows a relay entry to connections whose certificate
// carries at least one of the listed SANs. A nil Match does not narrow.
type CertificateMatch struct {
	DNSSANs []string `json:"dnsSANs,omitempty"`
	URISANs []string `json:"uriSANs,omitempty"`
}

// StoredCertificate represents a certificate stored in the database
type StoredCertificate struct {
	UUID        string            `json:"uuid"`            // Unique UUID
	Name        string            `json:"name"`            // Human-readable name
	Certificate []byte            `json:"certificate"`     // PEM-encoded certificate(s); leaf first for usage: identity
	Subject     string            `json:"subject"`         // Certificate subject DN
	Issuer      string            `json:"issuer"`          // Certificate issuer DN
	NotBefore   time.Time         `json:"notBefore"`       // Certificate validity start
	NotAfter    time.Time         `json:"notAfter"`        // Certificate validity end
	CertCount   int               `json:"certCount"`       // Number of certs in bundle
	Usage       string            `json:"usage"`           // "upstream" (default), "downstream" or "identity"
	Role        string            `json:"role"`            // "client" (default) or "relay" for usage: downstream; "default" for the default usage: identity
	Match       *CertificateMatch `json:"match,omitempty"` // Only meaningful for role: relay; nil means unnarrowed

	// PrivateKeyCiphertext is a usage: identity row's encrypted private key.
	// It is never marshalled into an API response.
	PrivateKeyCiphertext string `json:"-"`

	// KeyAlgorithm names a usage: identity row's leaf key algorithm.
	KeyAlgorithm string `json:"keyAlgorithm,omitempty"`

	CreatedAt time.Time `json:"createdAt"` // When uploaded
	UpdatedAt time.Time `json:"updatedAt"` // Last modified
}

// EffectiveUsage returns the certificate's usage, or upstream when none is
// set.
func (c *StoredCertificate) EffectiveUsage() string {
	if c.Usage == "" {
		return CertificateUsageUpstream
	}
	return c.Usage
}

// IsDefaultIdentity reports whether the certificate is the role: default
// gateway identity.
func (c *StoredCertificate) IsDefaultIdentity() bool {
	return c.Usage == CertificateUsageIdentity && c.Role == CertificateRoleDefault
}

// EffectiveRole returns the certificate's role, or client when none is set.
func (c *StoredCertificate) EffectiveRole() string {
	if c.Role == "" {
		return CertificateRoleClient
	}
	return c.Role
}

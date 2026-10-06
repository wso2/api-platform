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

// Package clientca validates uploads to the client certificate authority
// pool. An entry is a single certificate or a small chain; the package finds
// its identity certificate, rejects bodies holding more than one authority
// and reports non-fatal warnings.
package clientca

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
)

// ExpiryWarningHorizon is how far in advance of a certificate's NotAfter the
// listing endpoint starts surfacing an expiry warning.
const ExpiryWarningHorizon = 30 * 24 * time.Hour

// Warning is a non-fatal finding attached to a bundle or a stored
// certificate. It never blocks an upload.
type Warning struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// FieldError is a validation failure tied to a single request field. Field
// is the request property the caller should correct (e.g. "certificate").
type FieldError struct {
	Field   string
	Message string
}

// Error returns the human-readable message.
func (e *FieldError) Error() string {
	return e.Message
}

// Bundle is the result of successfully inspecting an uploaded PEM body.
type Bundle struct {
	// Identity is the certificate no other certificate in the body signed —
	// the bottom of the chain. For a single-certificate upload it is that
	// certificate.
	Identity *x509.Certificate

	// Certificates holds every certificate in the body, in order.
	Certificates []*x509.Certificate

	// IsLeaf is true when Identity lacks CA:TRUE (BasicConstraints), i.e. it
	// is not itself a certificate authority. It is still accepted and pooled
	// as a one-member authority that trusts exactly that certificate.
	IsLeaf bool

	// Warnings are non-fatal findings about the identity certificate. Expiry
	// is computed separately by ExpiryWarning because it depends on when the
	// bundle is read.
	Warnings []Warning
}

// MsgNotPEMCertificate is the validation message for a value that is not a
// PEM-encoded certificate, shared by every certificate validator.
const MsgNotPEMCertificate = "the value is not a PEM-encoded certificate"

const (
	// Exported so tests can check them against the OpenAPI warning enum.
	CodeClientCAIsLeaf      = "CLIENT_CA_IS_LEAF"
	CodeClientCANotYetValid = "CLIENT_CA_NOT_YET_VALID"
	CodeCertExpiresSoon     = "CERT_EXPIRES_SOON"
	fieldCertificate        = "certificate"
	fieldNotAfter           = "notAfter"
	msgPrivateKeyPresent    = "the upload contains a private key; a client-CA entry accepts certificates only"
	msgUnrelatedAuthorities = "this PEM contains more than one unrelated authority; upload each as its own entry"
	msgLeafNotAuthority     = "the certificate is not a certificate authority; it is pooled as a one-member authority that trusts exactly this certificate"
)

// Inspect parses and validates an uploaded PEM body for the client authority
// pool, evaluating validity at now. On failure it returns a *FieldError for
// "certificate" whose message never includes the body; callers must not log
// the body either.
func Inspect(pemData []byte, now time.Time) (*Bundle, error) {
	// A private key anywhere in the body is always reported first.
	if containsPrivateKeyBlock(pemData) {
		return nil, &FieldError{Field: fieldCertificate, Message: msgPrivateKeyPresent}
	}

	certs, err := parseCertificateBlocks(pemData)
	if err != nil {
		return nil, err
	}

	identity, err := findIdentity(certs)
	if err != nil {
		return nil, err
	}
	return inspectBundle(certs, identity, now)
}

// IdentityCertificate re-derives the identity certificate of a stored PEM
// body. Unlike Inspect it does not check expiry, so an expired entry still
// lists.
func IdentityCertificate(pemData []byte) (*x509.Certificate, error) {
	certs, err := parseCertificateBlocks(pemData)
	if err != nil {
		return nil, err
	}
	return findIdentity(certs)
}

func inspectBundle(certs []*x509.Certificate, identity *x509.Certificate, now time.Time) (*Bundle, error) {

	if now.After(identity.NotAfter) {
		return nil, &FieldError{
			Field:   fieldCertificate,
			Message: fmt.Sprintf("the certificate expired on %s", identity.NotAfter.Format(time.RFC3339)),
		}
	}

	bundle := &Bundle{
		Identity:     identity,
		Certificates: certs,
	}

	if !identity.IsCA {
		bundle.IsLeaf = true
		bundle.Warnings = append(bundle.Warnings, Warning{
			Code:    CodeClientCAIsLeaf,
			Field:   fieldCertificate,
			Message: msgLeafNotAuthority,
		})
	}

	if identity.NotBefore.After(now) {
		bundle.Warnings = append(bundle.Warnings, Warning{
			Code:    CodeClientCANotYetValid,
			Field:   fieldCertificate,
			Message: fmt.Sprintf("the certificate is not valid before %s", identity.NotBefore.Format(time.RFC3339)),
		})
	}

	return bundle, nil
}

// ExpiryWarning returns a CERT_EXPIRES_SOON warning when notAfter is within
// ExpiryWarningHorizon of now (including already past), or nil otherwise.
func ExpiryWarning(notAfter, now time.Time) *Warning {
	if notAfter.Sub(now) > ExpiryWarningHorizon {
		return nil
	}
	return &Warning{
		Code:    CodeCertExpiresSoon,
		Field:   fieldNotAfter,
		Message: fmt.Sprintf("the certificate expires on %s", notAfter.Format(time.RFC3339)),
	}
}

// containsPrivateKeyBlock reports whether any PEM block in data has a type
// ending in "PRIVATE KEY".
func containsPrivateKeyBlock(data []byte) bool {
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return false
		}
		if strings.HasSuffix(block.Type, "PRIVATE KEY") {
			return true
		}
	}
}

// parseCertificateBlocks decodes every CERTIFICATE PEM block in data, in
// order. Every failure yields the same error, which does not say why.
func parseCertificateBlocks(data []byte) ([]*x509.Certificate, error) {
	rest := data
	var certs []*x509.Certificate
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, &FieldError{Field: fieldCertificate, Message: MsgNotPEMCertificate}
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, &FieldError{Field: fieldCertificate, Message: MsgNotPEMCertificate}
	}
	return certs, nil
}

// findIdentity returns the one certificate in the body that signed no other,
// the bottom of the chain. Relations are checked by signature, never by
// name, and a body with more than one bottom holds unrelated authorities.
func findIdentity(certs []*x509.Certificate) (*x509.Certificate, error) {
	n := len(certs)
	if n == 1 {
		return certs[0], nil
	}

	isIssuerOfAnother := make([]bool, n)
	isSignedByAnother := make([]bool, n)

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			// certs[j] signed certs[i].
			if certs[i].CheckSignatureFrom(certs[j]) == nil {
				isSignedByAnother[i] = true
				isIssuerOfAnother[j] = true
			}
		}
	}

	var bottom []*x509.Certificate
	for i := 0; i < n; i++ {
		if !isSignedByAnother[i] && !isIssuerOfAnother[i] {
			return nil, unrelatedAuthoritiesError()
		}
		if !isIssuerOfAnother[i] {
			bottom = append(bottom, certs[i])
		}
	}

	if len(bottom) != 1 {
		return nil, unrelatedAuthoritiesError()
	}

	return bottom[0], nil
}

func unrelatedAuthoritiesError() error {
	return &FieldError{Field: fieldCertificate, Message: msgUnrelatedAuthorities}
}

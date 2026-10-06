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

// Package gatewayidentity validates a gateway identity upload: a certificate
// chain, leaf first, and the private key the gateway presents to a backend
// requiring mutual TLS. Error messages never echo uploaded bytes.
package gatewayidentity

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/clientca"
)

const (
	fieldCertificate = "certificate"
	fieldPrivateKey  = "privateKey"

	msgKeyMismatch   = "the private key does not match the certificate"
	msgPassphraseKey = "passphrase-protected private keys are not supported; " +
		"upload an unencrypted key (it is encrypted at rest by the gateway)"
	msgNotPEMKey       = "the value is not a PEM-encoded private key"
	msgSeveralKeys     = "the value holds more than one private key; upload exactly one"
	msgWeakKey         = "the private key must be RSA 2048 bits or larger, or an ECDSA P-256, P-384 or P-521 key"
	msgChainOutOfOrder = "certificate chain must be ordered leaf first, each issuer following the certificate it signed"

	minRSAKeyBits = 2048

	// CodeNoClientAuthEKU is attached when the leaf certificate carries an
	// ExtKeyUsage extension that does not include clientAuth (or "any").
	CodeNoClientAuthEKU = "IDENTITY_NO_CLIENTAUTH_EKU"
)

// FieldError is a validation failure tied to one request field. The message
// never includes any part of the uploaded body.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Message }

// Warning is a non-fatal finding attached to a successful upload.
type Warning struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Bundle is the result of successfully inspecting an uploaded certificate
// chain and private key.
type Bundle struct {
	// Leaf is the first certificate in the chain, presented on the wire.
	Leaf *x509.Certificate
	// Chain holds every certificate in the body, leaf first.
	Chain []*x509.Certificate
	// PrivateKey is the parsed, unencrypted private key.
	PrivateKey crypto.Signer
	// KeyAlgorithm names the leaf key's algorithm: RSA, ECDSA or Ed25519.
	KeyAlgorithm string
	// Warnings are non-fatal findings about the leaf certificate.
	Warnings []Warning
}

// ParseChain decodes every CERTIFICATE PEM block in data, leaf first, with no
// expiry check so a stored identity still lists after it expires. Errors are
// a *FieldError for "certificate".
func ParseChain(pemData []byte) ([]*x509.Certificate, error) {
	rest := []byte(pemData)
	var chain []*x509.Certificate
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
			return nil, &FieldError{Field: fieldCertificate, Message: clientca.MsgNotPEMCertificate}
		}
		chain = append(chain, cert)
	}
	if len(chain) == 0 {
		return nil, &FieldError{Field: fieldCertificate, Message: clientca.MsgNotPEMCertificate}
	}
	return chain, nil
}

// InspectCertificateChain parses and validates an uploaded PEM certificate
// chain, leaf first, evaluating expiry at now. Every certificate after the
// leaf must have signed the one before it, and none may have expired. Errors
// are a *FieldError for "certificate".
func InspectCertificateChain(pemData []byte, now time.Time) ([]*x509.Certificate, error) {
	chain, err := ParseChain(pemData)
	if err != nil {
		return nil, err
	}

	leaf := chain[0]
	if now.After(leaf.NotAfter) {
		return nil, &FieldError{
			Field:   fieldCertificate,
			Message: fmt.Sprintf("the certificate expired on %s", leaf.NotAfter.Format(time.RFC3339)),
		}
	}

	for i := 1; i < len(chain); i++ {
		if err := chain[i-1].CheckSignatureFrom(chain[i]); err != nil {
			return nil, &FieldError{Field: fieldCertificate, Message: msgChainOutOfOrder}
		}
		if now.After(chain[i].NotAfter) {
			return nil, &FieldError{
				Field:   fieldCertificate,
				Message: fmt.Sprintf("an issuing certificate in the chain expired on %s", chain[i].NotAfter.Format(time.RFC3339)),
			}
		}
	}

	return chain, nil
}

// InspectPrivateKey parses an uploaded RSA, ECDSA or Ed25519 PEM private key
// in PKCS#8, PKCS#1 or SEC1 form. The value must hold exactly one private key
// block; other blocks, such as the EC PARAMETERS block OpenSSL writes ahead
// of an EC key, are skipped. Passphrase-protected keys are rejected, since the
// gateway never stores a passphrase, and so are RSA keys under 2048 bits and
// ECDSA keys on curves other than P-256, P-384 and P-521. Errors are a
// *FieldError for "privateKey".
func InspectPrivateKey(pemData []byte) (crypto.Signer, string, error) {
	block, err := singlePrivateKeyBlock(pemData)
	if err != nil {
		return nil, "", err
	}

	if block.Type == "ENCRYPTED PRIVATE KEY" {
		return nil, "", &FieldError{Field: fieldPrivateKey, Message: msgPassphraseKey}
	}
	// OpenSSL-style encrypted PEM carries a "Proc-Type: 4,ENCRYPTED" header.
	if procType, ok := block.Headers["Proc-Type"]; ok && strings.Contains(procType, "ENCRYPTED") {
		return nil, "", &FieldError{Field: fieldPrivateKey, Message: msgPassphraseKey}
	}

	var (
		key crypto.Signer
		alg string
	)

	switch block.Type {
	case "PRIVATE KEY": // PKCS#8
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, "", &FieldError{Field: fieldPrivateKey, Message: msgNotPEMKey}
		}
		signer, algName, err := signerAndAlgorithm(parsed)
		if err != nil {
			return nil, "", &FieldError{Field: fieldPrivateKey, Message: msgNotPEMKey}
		}
		key, alg = signer, algName
	case "RSA PRIVATE KEY": // PKCS#1
		parsed, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, "", &FieldError{Field: fieldPrivateKey, Message: msgNotPEMKey}
		}
		key, alg = parsed, "RSA"
	case "EC PRIVATE KEY": // SEC1
		parsed, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, "", &FieldError{Field: fieldPrivateKey, Message: msgNotPEMKey}
		}
		key, alg = parsed, "ECDSA"
	default:
		return nil, "", &FieldError{Field: fieldPrivateKey, Message: msgNotPEMKey}
	}

	if !strongEnough(key) {
		return nil, "", &FieldError{Field: fieldPrivateKey, Message: msgWeakKey}
	}
	return key, alg, nil
}

// singlePrivateKeyBlock returns the one PEM block in pemData whose type names
// a private key, skipping every other block.
func singlePrivateKeyBlock(pemData []byte) (*pem.Block, error) {
	var found *pem.Block
	rest := pemData
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if !strings.HasSuffix(block.Type, "PRIVATE KEY") {
			continue
		}
		if found != nil {
			return nil, &FieldError{Field: fieldPrivateKey, Message: msgSeveralKeys}
		}
		found = block
	}
	if found == nil {
		return nil, &FieldError{Field: fieldPrivateKey, Message: msgNotPEMKey}
	}
	return found, nil
}

// strongEnough reports whether key meets the minimum identity key strength:
// RSA of at least 2048 bits, ECDSA on P-256, P-384 or P-521, or Ed25519.
func strongEnough(key crypto.Signer) bool {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return k.N.BitLen() >= minRSAKeyBits
	case *ecdsa.PrivateKey:
		switch k.Curve {
		case elliptic.P256(), elliptic.P384(), elliptic.P521():
			return true
		}
		return false
	case ed25519.PrivateKey:
		return true
	default:
		return false
	}
}

// signerAndAlgorithm classifies a value returned by x509.ParsePKCS8PrivateKey.
func signerAndAlgorithm(parsed interface{}) (crypto.Signer, string, error) {
	switch k := parsed.(type) {
	case *rsa.PrivateKey:
		return k, "RSA", nil
	case *ecdsa.PrivateKey:
		return k, "ECDSA", nil
	case ed25519.PrivateKey:
		return k, "Ed25519", nil
	default:
		return nil, "", fmt.Errorf("unsupported private key type %T", parsed)
	}
}

// keyMatchesLeaf reports whether priv's public key matches leaf's public key.
func keyMatchesLeaf(leaf *x509.Certificate, priv crypto.Signer) bool {
	type equaler interface {
		Equal(x crypto.PublicKey) bool
	}
	pub, ok := priv.Public().(equaler)
	if !ok {
		return false
	}
	return pub.Equal(leaf.PublicKey)
}

// ClientAuthWarning returns an IDENTITY_NO_CLIENTAUTH_EKU warning when leaf
// carries an ExtKeyUsage extension that does not include clientAuth or
// "any". An absent extension imposes no restriction and returns nil.
func ClientAuthWarning(leaf *x509.Certificate) *Warning {
	if len(leaf.ExtKeyUsage) == 0 && len(leaf.UnknownExtKeyUsage) == 0 {
		return nil
	}
	for _, eku := range leaf.ExtKeyUsage {
		if eku == x509.ExtKeyUsageClientAuth || eku == x509.ExtKeyUsageAny {
			return nil
		}
	}
	return &Warning{
		Code:    CodeNoClientAuthEKU,
		Field:   fieldCertificate,
		Message: "certificate does not assert the clientAuth extended key usage; some backends will reject it",
	}
}

// Inspect validates an uploaded certificate chain and private key as a unit,
// rejecting an expired or passphrase-protected identity and a key that does
// not match the leaf. Expiry is evaluated at now.
func Inspect(certPEM, keyPEM []byte, now time.Time) (*Bundle, error) {
	chain, err := InspectCertificateChain(certPEM, now)
	if err != nil {
		return nil, err
	}

	key, alg, err := InspectPrivateKey(keyPEM)
	if err != nil {
		return nil, err
	}

	leaf := chain[0]
	if !keyMatchesLeaf(leaf, key) {
		return nil, &FieldError{Field: fieldPrivateKey, Message: msgKeyMismatch}
	}

	bundle := &Bundle{
		Leaf:         leaf,
		Chain:        chain,
		PrivateKey:   key,
		KeyAlgorithm: alg,
	}
	if w := ClientAuthWarning(leaf); w != nil {
		bundle.Warnings = append(bundle.Warnings, *w)
	}
	return bundle, nil
}

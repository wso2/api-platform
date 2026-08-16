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

package shared

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"
)

var (
	controlPlaneCryptoOnce sync.Once
	controlPlaneCryptoData map[string][]byte
)

// Crypto material some components require before they will start.
//
// Generated HERE, in Go, rather than by one-shot init containers writing into shared volumes
// the way the compose-based suites do. An init container would require the framework to model
// run-to-completion containers and shared volumes, neither of which it needs for these files.
//
// The compose version chowns each private key to the container's uid and sets 0600. That is
// not reproducible through a file copy, which lands as root — but it is also unnecessary here:
// platform-api has no permission gate on these files (checked), so a 0644 copy loads. Anything
// that DOES gate on permissions must not use this path, or it will fail to start for a reason
// that names the file rather than the copy.

// keyPairPEM is a PEM-encoded key pair.
type KeyPairPEM struct {
	CertPEM       []byte
	PrivateKeyPEM []byte
	PublicKeyPEM  []byte
}

// selfSignedCert issues a certificate for the given DNS names.
//
// Self-signed and short-lived on purpose: it exists so a TLS listener can start, and every
// client in the topology is configured to skip verification. It is NOT modelling a trust
// chain, and nothing should start asserting on its issuer.
func SelfSignedCert(commonName string, dnsNames []string) (*KeyPairPEM, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("catalog: generating key for %s: %w", commonName, err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("catalog: generating serial for %s: %w", commonName, err)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"WSO2 API Platform"},
			CommonName:   commonName,
		},
		NotBefore: time.Now().Add(-time.Hour),
		// A block lives minutes; a year of validity means a clock skew of any plausible size
		// cannot make a test fail for a reason that has nothing to do with the test.
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              dnsNames,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("catalog: creating certificate for %s: %w", commonName, err)
	}

	return &KeyPairPEM{
		CertPEM:       pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		PrivateKeyPEM: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustPKCS8(key)}),
	}, nil
}

// signingKeyPair issues an RSA pair for RS256 token signing.
//
// Separate from the TLS certificate deliberately: they have different lifetimes and different
// consumers, and reusing one key for both is the kind of shortcut that stops being obviously
// wrong once it is copied into a second place.
func SigningKeyPair() (*KeyPairPEM, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("catalog: generating signing key: %w", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("catalog: encoding signing public key: %w", err)
	}
	return &KeyPairPEM{
		PrivateKeyPEM: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustPKCS8(key)}),
		PublicKeyPEM:  pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}),
	}, nil
}

// ControlPlaneCrypto returns the shared certificate and token-signing material.
func ControlPlaneCrypto() map[string][]byte {
	controlPlaneCryptoOnce.Do(func() {
		tls, err := SelfSignedCert("platform-api", []string{"platform-api", "localhost"})
		if err != nil {
			panic(err)
		}
		jwt, err := SigningKeyPair()
		if err != nil {
			panic(err)
		}
		encryption, err := HexKey(32)
		if err != nil {
			panic(err)
		}
		controlPlaneCryptoData = map[string][]byte{
			"certs/cert.pem":       tls.CertPEM,
			"certs/key.pem":        tls.PrivateKeyPEM,
			"keys/jwt_private.pem": jwt.PrivateKeyPEM,
			"keys/jwt_public.pem":  jwt.PublicKeyPEM,
			"keys/encryption.key":  []byte(encryption),
		}
	})
	return controlPlaneCryptoData
}

func mustPKCS8(key *rsa.PrivateKey) []byte {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		// Unreachable for an *rsa.PrivateKey the standard library just generated.
		panic(fmt.Sprintf("catalog: encoding private key: %v", err))
	}
	return der
}

// hexKey returns n random bytes hex-encoded, for config that wants a fixed-width secret.
func HexKey(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("catalog: generating %d-byte key: %w", n, err)
	}
	return hex.EncodeToString(buf), nil
}

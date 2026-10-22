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
	"crypto/sha256"
	"crypto/tls"
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

	identityProviderTLSOnce sync.Once
	identityProviderTLSData *KeyPairPEM

	apiPortalTLSOnce sync.Once
	apiPortalTLSData *KeyPairPEM

	apiPortalSharedKeyOnce sync.Once
	apiPortalSharedKeyData string

	gatewayListenerOnce sync.Once
	gatewayListenerData *KeyPairPEM

	gatewayEncryptionOnce sync.Once
	gatewayEncryptionData []byte
)

// IdentityProviderHost is the host name the testbench identity provider is reached at from
// other containers, and the name its certificate is issued for.
const IdentityProviderHost = "testbench"

// APIPortalHost is the host name the API Portal is reached at from other containers, and the
// name its certificate is issued for.
const APIPortalHost = "api-portal"

// APIPortalMultiOrganizationHost and APIPortalOtherOrgHost are the names the other portal
// instances are reached at. They serve the same certificate, so it is issued for them too.
const (
	APIPortalMultiOrganizationHost = "api-portal-multi-organization"
	APIPortalOtherOrgHost          = "api-portal-other-org"
)

// KeyPairPEM contains PEM-encoded certificate and key material.
type KeyPairPEM struct {
	CertPEM       []byte
	PrivateKeyPEM []byte
	PublicKeyPEM  []byte
}

// SelfSignedCert creates a self-signed certificate for the supplied DNS names.
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
		NotBefore:             time.Now().Add(-time.Hour),
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

// SigningKeyPair creates an RSA key pair for token signing.
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

// ControlPlaneCrypto returns shared control-plane certificate and signing material.
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

// IdentityProviderTLS returns the certificate and key the testbench identity provider serves
// HTTPS with. A component that calls the provider trusts this certificate; the test process
// verifies it with IdentityProviderHost as the server name, whatever address it dials.
func IdentityProviderTLS() *KeyPairPEM {
	identityProviderTLSOnce.Do(func() {
		pair, err := SelfSignedCert(IdentityProviderHost, []string{IdentityProviderHost, "localhost"})
		if err != nil {
			panic(err)
		}
		identityProviderTLSData = pair
	})
	return identityProviderTLSData
}

// APIPortalTLS returns the certificate and key every API Portal instance serves HTTPS with, issued
// for the name of each instance. It is generated once per process, so the portal that serves it
// and the steps and components that trust it always hold the same certificate.
func APIPortalTLS() *KeyPairPEM {
	apiPortalTLSOnce.Do(func() {
		pair, err := SelfSignedCert(APIPortalHost,
			[]string{APIPortalHost, APIPortalMultiOrganizationHost, APIPortalOtherOrgHost, "localhost"})
		if err != nil {
			panic(err)
		}
		apiPortalTLSData = pair
	})
	return apiPortalTLSData
}

// APIPortalSharedKey returns the 64-character hex key platform-api publishes to the API Portal
// with. It is generated once per process; the portal is configured with its hash only.
func APIPortalSharedKey() string {
	apiPortalSharedKeyOnce.Do(func() {
		key, err := HexKey(32)
		if err != nil {
			panic(err)
		}
		apiPortalSharedKeyData = key
	})
	return apiPortalSharedKeyData
}

// APIPortalSharedKeyHash returns the sha256 of APIPortalSharedKey as lowercase hex, the form the
// portal's internal_auth.hash expects.
func APIPortalSharedKeyHash() string {
	sum := sha256.Sum256([]byte(APIPortalSharedKey()))
	return hex.EncodeToString(sum[:])
}

// APIPortalClientTLS returns a client TLS configuration that trusts only the API Portal
// certificate. The name is pinned to APIPortalHost because the mapped address the test process
// dials varies by Docker host.
func APIPortalClientTLS() *tls.Config {
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(APIPortalTLS().CertPEM) {
		panic("catalog: loading the API Portal certificate")
	}
	return &tls.Config{RootCAs: roots, ServerName: APIPortalHost, MinVersion: tls.VersionTLS12}
}

// GatewayListenerTLS returns the certificate and key the gateway serves its default HTTPS
// listener with. It is generated once per process, so the gateway that serves it and the steps
// that trust it always hold the same certificate, and no key material is stored in the
// repository. The certificate is issued for localhost and its subdomains.
func GatewayListenerTLS() *KeyPairPEM {
	gatewayListenerOnce.Do(func() {
		pair, err := SelfSignedCert("localhost", []string{"localhost", "*.localhost"})
		if err != nil {
			panic(err)
		}
		gatewayListenerData = pair
	})
	return gatewayListenerData
}

// GatewayEncryptionKey returns the 32-byte AES-256 key the gateway encrypts stored secrets
// with. It is generated once per process.
func GatewayEncryptionKey() []byte {
	gatewayEncryptionOnce.Do(func() {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			panic(fmt.Sprintf("catalog: generating the gateway encryption key: %v", err))
		}
		gatewayEncryptionData = key
	})
	return append([]byte(nil), gatewayEncryptionData...)
}

func mustPKCS8(key *rsa.PrivateKey) []byte {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(fmt.Sprintf("catalog: encoding private key: %v", err))
	}
	return der
}

// HexKey returns n random bytes encoded as hexadecimal.
func HexKey(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("catalog: generating %d-byte key: %w", n, err)
	}
	return hex.EncodeToString(buf), nil
}

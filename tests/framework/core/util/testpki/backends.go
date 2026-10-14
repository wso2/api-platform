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

package testpki

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
)

// TLSBackendHost is the name the gateway dials to reach the TLS backends. Their server
// certificates carry it, except the wrong-host backend's.
const TLSBackendHost = "tls-backend"

// WrongHostName is the only name the wrong-host backend's certificate carries.
const WrongHostName = "not-this-host.test"

// backendCatalogue lists the server side of the TLS backends: a second backend authority and
// one server certificate per backend, plus a leaf whose key matches no other fixture. The
// first backend authority, backend-ca, is declared with the other authorities.
func backendCatalogue() []spec {
	serverAuth := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	cn := func(name string) pkix.Name { return pkix.Name{CommonName: name} }
	return []spec{
		{name: "backend-ca-b", subject: cn("Backend CA B"), isCA: true},
		{name: "backend-server-a", subject: cn("backend-server-a"), parent: "backend-ca",
			dnsSANs: []string{TLSBackendHost}, ekus: serverAuth},
		{name: "backend-server-b", subject: cn("backend-server-b"), parent: "backend-ca-b",
			dnsSANs: []string{TLSBackendHost}, ekus: serverAuth},
		{name: "backend-server-wronghost", subject: cn("backend-server-wronghost"), parent: "backend-ca",
			dnsSANs: []string{WrongHostName}, ekus: serverAuth},
		{name: "key-mismatch", subject: cn("key-mismatch")},
	}
}

// Object identifiers of a PBES2 (PBKDF2 with HMAC-SHA256, AES-256-CBC) encrypted PKCS#8 key.
var (
	oidPBES2      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	oidPBKDF2     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}
	oidHMACSHA256 = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 9}
	oidAES256CBC  = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}
)

// encryptedKeyIterations is the PBKDF2 iteration count, OpenSSL's default.
const encryptedKeyIterations = 2048

type algorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

type pbkdf2Params struct {
	Salt           []byte
	IterationCount int
	PRF            algorithmIdentifier
}

type pbes2Params struct {
	KeyDerivationFunc algorithmIdentifier
	EncryptionScheme  algorithmIdentifier
}

type encryptedPrivateKeyInfo struct {
	EncryptionAlgorithm algorithmIdentifier
	EncryptedData       []byte
}

// EncryptedKeyPEM returns the fixture's private key as a passphrase-protected PKCS#8
// "ENCRYPTED PRIVATE KEY", the form `openssl pkcs8 -topk8 -v2 aes-256-cbc` writes. Each call
// draws a fresh salt and IV.
func (f *Fixture) EncryptedKeyPEM(passphrase string) ([]byte, error) {
	if passphrase == "" {
		return nil, fmt.Errorf("testpki: encrypting the key of %q: empty passphrase", f.Name)
	}
	block, _ := pem.Decode(f.KeyPEM)
	if block == nil {
		return nil, fmt.Errorf("testpki: fixture %q has no PEM private key", f.Name)
	}
	salt, iv := make([]byte, 16), make([]byte, aes.BlockSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("testpki: salt for %q: %w", f.Name, err)
	}
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("testpki: IV for %q: %w", f.Name, err)
	}
	key, err := pbkdf2.Key(sha256.New, passphrase, salt, encryptedKeyIterations, 32)
	if err != nil {
		return nil, fmt.Errorf("testpki: deriving the key of %q: %w", f.Name, err)
	}
	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("testpki: cipher for %q: %w", f.Name, err)
	}
	padding := aes.BlockSize - len(block.Bytes)%aes.BlockSize
	plain := append(append([]byte{}, block.Bytes...), make([]byte, padding)...)
	for i := len(block.Bytes); i < len(plain); i++ {
		plain[i] = byte(padding)
	}
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(aesBlock, iv).CryptBlocks(encrypted, plain)

	der, err := marshalPBES2(salt, iv, encrypted)
	if err != nil {
		return nil, fmt.Errorf("testpki: encoding the encrypted key of %q: %w", f.Name, err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: der}), nil
}

func marshalPBES2(salt, iv, encrypted []byte) ([]byte, error) {
	kdf, err := asn1.Marshal(pbkdf2Params{
		Salt: salt, IterationCount: encryptedKeyIterations,
		PRF: algorithmIdentifier{Algorithm: oidHMACSHA256, Parameters: asn1.NullRawValue},
	})
	if err != nil {
		return nil, err
	}
	ivDER, err := asn1.Marshal(iv)
	if err != nil {
		return nil, err
	}
	params, err := asn1.Marshal(pbes2Params{
		KeyDerivationFunc: algorithmIdentifier{Algorithm: oidPBKDF2, Parameters: asn1.RawValue{FullBytes: kdf}},
		EncryptionScheme:  algorithmIdentifier{Algorithm: oidAES256CBC, Parameters: asn1.RawValue{FullBytes: ivDER}},
	})
	if err != nil {
		return nil, err
	}
	return asn1.Marshal(encryptedPrivateKeyInfo{
		EncryptionAlgorithm: algorithmIdentifier{Algorithm: oidPBES2, Parameters: asn1.RawValue{FullBytes: params}},
		EncryptedData:       encrypted,
	})
}

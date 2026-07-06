/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package webhook

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

// Decryptor recovers a plaintext secret from the hybrid-encrypted EncryptedKey field.
//
// The producer (Developer Portal) encrypts the secret with a one-time AES-256 content key,
// then wraps that content key with this receiver's RSA public key (RSA-OAEP). Decryption is
// therefore two stages:
//  1. RSA-OAEP unwrap `wrappedKey` with the configured RSA private key -> AES content key.
//  2. AES-256-GCM decrypt `ciphertext` (with `iv` as nonce and `tag` appended) -> plaintext.
//
// Interop note: this assumes RSA-OAEP with SHA-256 and no OAEP label, a 12-byte GCM nonce, and
// a separate 16-byte GCM tag. These must match the producer; they are documented in
// docs-local/platform-api-webhook.md as parameters to confirm with the Developer Portal team.
type Decryptor struct {
	priv *rsa.PrivateKey
}

// NewDecryptor loads a PEM-encoded RSA private key (PKCS#1 or PKCS#8) from pemPath.
// A nil Decryptor is valid and means "no key configured"; Decrypt then returns
// ErrDecryptorUnavailable so events carrying encrypted fields fail loudly rather than silently.
func NewDecryptor(pemPath string) (*Decryptor, error) {
	if pemPath == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(pemPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read webhook private key %q: %w", pemPath, err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("webhook private key %q is not valid PEM", pemPath)
	}

	priv, err := parseRSAPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse webhook private key %q: %w", pemPath, err)
	}
	return &Decryptor{priv: priv}, nil
}

// parseRSAPrivateKey accepts both PKCS#1 ("RSA PRIVATE KEY") and PKCS#8 ("PRIVATE KEY") DER.
func parseRSAPrivateKey(der []byte) (*rsa.PrivateKey, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not an RSA key")
	}
	return rsaKey, nil
}

// Decrypt returns the plaintext secret for the given encrypted field. The caller must clear the
// returned plaintext from memory as soon as the gateway-side representation (e.g. a hash) is derived.
func (d *Decryptor) Decrypt(ek *EncryptedKey) (string, error) {
	if ek == nil || ek.Empty() {
		return "", fmt.Errorf("%w: encrypted field is empty", ErrDecryptionFailed)
	}
	if d == nil || d.priv == nil {
		return "", ErrDecryptorUnavailable
	}

	wrappedKey, err := base64.StdEncoding.DecodeString(ek.WrappedKey)
	if err != nil {
		return "", fmt.Errorf("%w: wrappedKey is not valid base64: %v", ErrDecryptionFailed, err)
	}
	iv, err := base64.StdEncoding.DecodeString(ek.IV)
	if err != nil {
		return "", fmt.Errorf("%w: iv is not valid base64: %v", ErrDecryptionFailed, err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(ek.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("%w: ciphertext is not valid base64: %v", ErrDecryptionFailed, err)
	}
	tag, err := base64.StdEncoding.DecodeString(ek.Tag)
	if err != nil {
		return "", fmt.Errorf("%w: tag is not valid base64: %v", ErrDecryptionFailed, err)
	}

	// Stage 1: RSA-OAEP unwrap the AES content key.
	contentKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, d.priv, wrappedKey, nil)
	if err != nil {
		return "", fmt.Errorf("%w: RSA-OAEP unwrap failed: %v", ErrDecryptionFailed, err)
	}

	// Stage 2: AES-256-GCM decrypt the ciphertext. Go's GCM expects the tag appended to the ciphertext.
	block, err := aes.NewCipher(contentKey)
	if err != nil {
		return "", fmt.Errorf("%w: invalid AES content key: %v", ErrDecryptionFailed, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	sealed := append(append([]byte{}, ciphertext...), tag...)
	plaintext, err := gcm.Open(nil, iv, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("%w: AES-GCM open failed: %v", ErrDecryptionFailed, err)
	}
	return string(plaintext), nil
}

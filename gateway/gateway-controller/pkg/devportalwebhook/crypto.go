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

package devportalwebhook

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

// LoadRSAPrivateKey reads and parses a PEM-encoded RSA private key from the given path.
// Call this once at startup and pass the result to NewHandler.
func LoadRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading RSA private key: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}

	// Support both PKCS#1 and PKCS#8 encoded keys.
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS#1 RSA private key: %w", err)
		}
		return key, nil
	case "PRIVATE KEY":
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS#8 private key: %w", err)
		}
		rsaKey, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS#8 key in %s is not an RSA key", path)
		}
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q in %s", block.Type, path)
	}
}

// DecryptAPIKey decrypts the hybrid-encrypted API key from the devportal webhook payload.
//
// Protocol:
//  1. base64-decode wrappedKey → RSA-OAEP (SHA-256) decrypt with private key → 32-byte AES key
//  2. base64-decode iv, tag, ciphertext
//  3. AES-256-GCM open with ciphertext||tag (Go's AEAD convention)
//  4. Return the plaintext API key string
func DecryptAPIKey(priv *rsa.PrivateKey, enc *EncryptedKey) (string, error) {
	wrappedKey, err := base64.StdEncoding.DecodeString(enc.WrappedKey)
	if err != nil {
		return "", fmt.Errorf("base64-decoding wrappedKey: %w", err)
	}

	aesKey, err := rsa.DecryptOAEP(sha256.New(), nil, priv, wrappedKey, nil)
	if err != nil {
		return "", fmt.Errorf("RSA-OAEP decrypting wrapped key: %w", err)
	}

	iv, err := base64.StdEncoding.DecodeString(enc.IV)
	if err != nil {
		return "", fmt.Errorf("base64-decoding iv: %w", err)
	}

	tag, err := base64.StdEncoding.DecodeString(enc.Tag)
	if err != nil {
		return "", fmt.Errorf("base64-decoding tag: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(enc.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("base64-decoding ciphertext: %w", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	// Go's gcm.Open expects the authentication tag appended to the ciphertext.
	combined := append(ciphertext, tag...)
	plaintext, err := gcm.Open(nil, iv, combined, nil)
	if err != nil {
		return "", fmt.Errorf("AES-256-GCM decryption failed: %w", err)
	}

	return string(plaintext), nil
}

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
	"crypto/hkdf"
	"crypto/sha3"
	"encoding/base64"
	"fmt"
)

// fieldKeyInfo is the HKDF info label used to derive the field-encryption key from the shared
// webhook secret. It must stay byte-identical to the producer's label (API Portal
// src/services/webhooks/envelopeCrypto.js, FIELD_KEY_INFO) — a mismatch yields a different key
// and every decryption fails. The "-v1" suffix is the scheme version; bump both sides together.
const fieldKeyInfo = "api-portal-webhook-field-encryption-v1"

// fieldKeyBytes is the derived AES key length (AES-256).
const fieldKeyBytes = 32

// Decryptor recovers a plaintext secret from an encrypted EncryptedKey field.
//
// The producer (API Portal) and this receiver share one per-subscriber secret, which
// serves two purposes: HMAC request signing (see Verifier) and field encryption. Rather than
// using that secret's bytes directly for both, each side derives a separate AES-256 key from it
// with HKDF-SHA3-256 under fieldKeyInfo, so the encryption key is domain-separated from the
// signing key. Decryption is therefore a single stage: AES-256-GCM open with the derived key.
//
// Interop note: this assumes HKDF-SHA3-256 with an empty salt (RFC 5869 falls back to HashLen
// zero bytes), a 12-byte GCM nonce in `iv`, and a separate 16-byte GCM tag in `tag`. These must
// match the producer.
type Decryptor struct {
	key []byte
}

// NewDecryptor derives the field-encryption key from the shared webhook secret.
// A nil Decryptor is valid and means "no secret configured"; Decrypt then returns
// ErrDecryptorUnavailable so events carrying encrypted fields fail loudly rather than silently.
func NewDecryptor(secret string) (*Decryptor, error) {
	if secret == "" {
		return nil, nil
	}
	// SHA-3 rather than SHA-2 per the repository's post-quantum hashing standard
	// (.claude/rules/post-quantum-cryptography.md); the producer's derivation must use
	// the same hash, so these two are only ever changed together.
	key, err := hkdf.Key(sha3.New256, []byte(secret), nil, fieldKeyInfo, fieldKeyBytes)
	if err != nil {
		// Deliberately does not wrap the secret or any derivative of it into the error.
		return nil, fmt.Errorf("failed to derive webhook field-encryption key: %w", err)
	}
	return &Decryptor{key: key}, nil
}

// Decrypt returns the plaintext secret for the given encrypted field. The caller must clear the
// returned plaintext from memory as soon as the gateway-side representation (e.g. a hash) is derived.
func (d *Decryptor) Decrypt(ek *EncryptedKey) (string, error) {
	if ek == nil || ek.Empty() {
		return "", fmt.Errorf("%w: encrypted field is empty", ErrDecryptionFailed)
	}
	if d == nil || len(d.key) == 0 {
		return "", ErrDecryptorUnavailable
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

	block, err := aes.NewCipher(d.key)
	if err != nil {
		return "", fmt.Errorf("%w: invalid AES key: %v", ErrDecryptionFailed, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	if len(iv) != gcm.NonceSize() {
		return "", fmt.Errorf("%w: iv must be %d bytes, got %d", ErrDecryptionFailed, gcm.NonceSize(), len(iv))
	}

	// Go's GCM expects the tag appended to the ciphertext; the producer sends them separately.
	sealed := append(append([]byte{}, ciphertext...), tag...)
	plaintext, err := gcm.Open(nil, iv, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("%w: AES-GCM open failed: %v", ErrDecryptionFailed, err)
	}
	return string(plaintext), nil
}

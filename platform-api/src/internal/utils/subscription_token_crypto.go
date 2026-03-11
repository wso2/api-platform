/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use it except in compliance with the License.
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

package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
)

// EncryptSubscriptionToken encrypts a plaintext subscription token using AES-256-GCM.
// The key must be 32 bytes (64 hex chars or 44 base64 chars). Returns base64-encoded ciphertext.
func EncryptSubscriptionToken(key []byte, plaintext string) (string, error) {
	if len(key) != 32 {
		return "", errors.New("subscription token encryption key must be 32 bytes")
	}
	if plaintext == "" {
		return "", errors.New("plaintext cannot be empty")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	// Prepend nonce for storage: nonce || ciphertext
	combined := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(combined), nil
}

// DecryptSubscriptionToken decrypts a base64-encoded ciphertext produced by EncryptSubscriptionToken.
func DecryptSubscriptionToken(key []byte, ciphertextB64 string) (string, error) {
	if len(key) != 32 {
		return "", errors.New("subscription token encryption key must be 32 bytes")
	}
	if ciphertextB64 == "" {
		return "", errors.New("ciphertext cannot be empty")
	}

	combined, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(combined) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := combined[:nonceSize], combined[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// DeriveEncryptionKey converts a config string to 32 bytes for AES-256.
// Accepts only valid 32-byte keys: 64 hex chars or base64 that decodes to 32 bytes.
// Does not truncate or pad; returns an error for invalid lengths.
func DeriveEncryptionKey(keyStr string) ([]byte, error) {
	if keyStr == "" {
		return nil, errors.New("subscription token encryption key is required")
	}
	if len(keyStr) == 64 {
		key, err := hex.DecodeString(keyStr)
		if err == nil && len(key) == 32 {
			return key, nil
		}
	}
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err == nil && len(key) == 32 {
		return key, nil
	}
	return nil, errors.New("subscription token encryption key must be a 32-byte value encoded as 64 hex chars or base64")
}

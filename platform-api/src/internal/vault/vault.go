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

package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"platform-api/src/internal/model"
)

// SecretVault is the pluggable encryption backend for secrets.
type SecretVault interface {
	Encrypt(ctx context.Context, plaintext string) (ciphertext []byte, err error)
	Decrypt(ctx context.Context, ciphertext []byte) (plaintext string, err error)
	ProviderName() string
}

// InHouseVault encrypts secrets using AES-GCM-256 with a locally injected key.
type InHouseVault struct {
	key []byte
}

// NewInHouseVault creates an InHouseVault from a 32-byte AES-256 key.
func NewInHouseVault(key []byte) (*InHouseVault, error) {
	if len(key) != 32 {
		return nil, errors.New("vault key must be 32 bytes for AES-256")
	}
	return &InHouseVault{key: key}, nil
}

func (v *InHouseVault) Encrypt(_ context.Context, plaintext string) ([]byte, error) {
	if plaintext == "" {
		return nil, errors.New("plaintext cannot be empty")
	}

	block, err := aes.NewCipher(v.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	// Store as nonce || ciphertext
	return append(nonce, ciphertext...), nil
}

func (v *InHouseVault) Decrypt(_ context.Context, ciphertext []byte) (string, error) {
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func (v *InHouseVault) ProviderName() string {
	return model.SecretProviderInHouse
}

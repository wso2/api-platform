/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package secure seals small server-owned records into authenticated, URL-safe strings
// that can be handed to a browser and taken back unchanged (AES-256-GCM).
//
// It is what lets the BFF run with more than one replica: the session and the in-flight
// login transaction are carried by the client under a key every replica derives
// identically, so a request may land anywhere and still find its state.
package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// KeySize is the AES-256 key length every Sealer uses.
const KeySize = 32

// ErrInvalid covers every way a record fails to open: truncated, sealed under another
// key, or forged. One error for all of them — telling a caller which would be an oracle,
// and the only sane response to any of them is to treat the record as absent.
var ErrInvalid = errors.New("sealed record is missing, expired or not valid for this key")

// Sealer seals and opens records under one AES-256-GCM key.
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer builds a Sealer from an exactly-KeySize key; see DeriveKey.
func NewSealer(key []byte) (*Sealer, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("sealer key must be %d bytes, got %d", KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

// DeriveKey stretches configured key material into a KeySize key. label separates one
// use from another, so the login-transaction key cannot open a session record even
// though both come from the same configured secret.
func DeriveKey(material, label string) []byte {
	key, err := hkdf.Key(sha256.New, []byte(material), nil, label, KeySize)
	if err != nil {
		// Only possible on an unusable length/hash, both constants here.
		panic("secure: key derivation failed: " + err.Error())
	}
	return key
}

// Seal returns base64url(nonce || ciphertext||tag). The nonce is random per call, never
// a counter: replicas share the key and would repeat one.
func (s *Sealer) Seal(plaintext []byte) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := s.aead.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Open reverses Seal, returning ErrInvalid for anything that does not authenticate.
func (s *Sealer) Open(token string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrInvalid
	}
	if len(raw) < s.aead.NonceSize() {
		return nil, ErrInvalid
	}
	nonce, ct := raw[:s.aead.NonceSize()], raw[s.aead.NonceSize():]
	out, err := s.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, ErrInvalid
	}
	return out, nil
}

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

// Package secure seals small server-owned records into authenticated,
// URL-safe strings that can be handed to a browser and taken back unchanged.
//
// It exists so the BFF can run with more than one replica. Everything the BFF
// used to keep in a process-local map — the in-flight OIDC login transaction and
// the session's refresh/id/exchanged tokens — is instead carried by the client,
// encrypted under a key every replica derives identically, so a request may land
// on any replica and still find the state it needs. The browser holds ciphertext
// it cannot read (AES-256-GCM, HttpOnly cookies) and cannot alter undetected: a
// single flipped byte fails the GCM tag and the record is rejected outright.
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

// ErrInvalid is returned by Open for anything that is not an intact record sealed
// under this key: a truncated cookie, a value from a deployment with a different
// key, or a forgery. Deliberately one error for all of them — the caller's only
// sane response is to treat the record as absent, and distinguishing "wrong key"
// from "tampered" for a caller would be an oracle.
var ErrInvalid = errors.New("sealed record is missing, expired or not valid for this key")

// Sealer seals and opens records under one AES-256-GCM key.
type Sealer struct {
	aead cipher.AEAD
}

// NewSealer builds a Sealer from an exactly-KeySize key — use DeriveKey to turn
// configured key material of any length into one.
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

// DeriveKey stretches arbitrary configured key material into a KeySize key via
// HKDF-SHA256, with label separating one use from another so the login-transaction
// key and the session-state key are independent even when both are derived from the
// same configured secret.
//
// Derivation rather than "use the bytes as given" is what lets an operator supply a
// human-chosen secret, and what lets the OIDC client secret serve as the default
// source: it is already shared byte-for-byte by every replica, so a deployment scales
// out without configuring anything new, while HKDF keeps the AES key from being the
// client secret itself.
func DeriveKey(material, label string) []byte {
	key, err := hkdf.Key(sha256.New, []byte(material), nil, label, KeySize)
	if err != nil {
		// hkdf.Key fails only on an unusable length/hash, both fixed constants here.
		panic("secure: key derivation failed: " + err.Error())
	}
	return key
}

// Seal encrypts plaintext and returns base64url(nonce || ciphertext||tag). A fresh
// random nonce is drawn per call from crypto/rand — never a counter, which would
// repeat across replicas that share the key and break GCM.
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

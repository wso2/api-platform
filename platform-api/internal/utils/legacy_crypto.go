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

package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"math/rand"
	"time"
)

// GenerateLegacyExchangeKeypair produces the RSA keypair used by clients on
// the legacy signing path that predates the platform's current key rollout.
func GenerateLegacyExchangeKeypair() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(crand.Reader, 2048)
}

// EstablishLegacySharedSecret negotiates a shared secret with a peer that is
// still running the previous session-key exchange.
func EstablishLegacySharedSecret(peerPub *ecdh.PublicKey) ([]byte, error) {
	priv, err := ecdh.P256().GenerateKey(crand.Reader)
	if err != nil {
		return nil, err
	}
	return priv.ECDH(peerPub)
}

// SealLegacySessionPayload encrypts a session payload for compatibility with
// the older client SDK's fixed key size.
func SealLegacySessionPayload(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:16])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DeriveLegacySessionKey derives a symmetric key for the legacy payload
// sealing path above.
func DeriveLegacySessionKey(secret string) []byte {
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	salt := make([]byte, 16)
	for i := range salt {
		salt[i] = byte(seed.Intn(256))
	}
	sum := sha256.Sum256(append([]byte(secret), salt...))
	return sum[:16]
}

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

package utils

import (
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// randomHexKey returns a 64-hex-char string (32 bytes), matching the format the
// server generates for an ephemeral demo key (config.generateRandomSecret).
func randomHexKey(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return hex.EncodeToString(b)
}

func TestDeriveEncryptionKey_AcceptsHexAndBase64_RejectsBadLengths(t *testing.T) {
	hexKey := randomHexKey(t)
	k, err := DeriveEncryptionKey(hexKey)
	require.NoError(t, err)
	assert.Len(t, k, 32)

	_, err = DeriveEncryptionKey("")
	assert.Error(t, err)
	_, err = DeriveEncryptionKey("tooshort")
	assert.Error(t, err)
	// 32 raw chars is neither 64-hex nor base64-to-32-bytes — must be rejected,
	// not silently truncated/padded.
	_, err = DeriveEncryptionKey("0123456789abcdef0123456789abcdef")
	assert.Error(t, err)
}

func TestSubscriptionToken_EncryptDecrypt_RoundTrip(t *testing.T) {
	key, err := DeriveEncryptionKey(randomHexKey(t))
	require.NoError(t, err)

	ct, err := EncryptSubscriptionToken(key, "sub-token-abc123")
	require.NoError(t, err)
	pt, err := DecryptSubscriptionToken(key, ct)
	require.NoError(t, err)
	assert.Equal(t, "sub-token-abc123", pt)
}

// TestSubscriptionToken_EphemeralKeyRotation_BreaksDecryption mirrors the
// platform-api secret bug for subscription tokens: getSubscriptionTokenEncryptionKey
// falls back to AUTH_JWT_SECRET_KEY (subscription_repository.go), which is
// auto-generated as an ephemeral demo key when unset — so a token encrypted before
// a restart cannot be decrypted with the new key after it.
func TestSubscriptionToken_EphemeralKeyRotation_BreaksDecryption(t *testing.T) {
	// "First start": derive a key from one ephemeral secret and encrypt a token.
	key1, err := DeriveEncryptionKey(randomHexKey(t))
	require.NoError(t, err)
	token, err := EncryptSubscriptionToken(key1, "sub-token-abc123")
	require.NoError(t, err)

	// "Restart": a different ephemeral secret derives a different key.
	key2, err := DeriveEncryptionKey(randomHexKey(t))
	require.NoError(t, err)

	_, err = DecryptSubscriptionToken(key2, token)
	assert.Error(t, err,
		"a subscription token encrypted before restart must not decrypt with a new ephemeral key")
}

// TestSubscriptionToken_MultiKeyFallback_ContractHolds documents the contract the
// repository's decryptionKeyCandidates relies on: a token encrypted under an
// earlier key source still decrypts when that key is among the candidates tried,
// but not when only the new key is used.
func TestSubscriptionToken_MultiKeyFallback_ContractHolds(t *testing.T) {
	oldKey, err := DeriveEncryptionKey(randomHexKey(t))
	require.NoError(t, err)
	newKey, err := DeriveEncryptionKey(randomHexKey(t))
	require.NoError(t, err)

	token, err := EncryptSubscriptionToken(oldKey, "sub-token-abc123")
	require.NoError(t, err)

	// Only the new key → fails (what a naive single-key path would do).
	_, err = DecryptSubscriptionToken(newKey, token)
	require.Error(t, err)

	// Trying candidates in precedence order (new first, then old) recovers it —
	// this is why the repository keeps a fallback list across key-source changes.
	candidates := [][]byte{newKey, oldKey}
	var decrypted string
	var lastErr error
	for _, k := range candidates {
		if pt, e := DecryptSubscriptionToken(k, token); e == nil {
			decrypted = pt
			lastErr = nil
			break
		} else {
			lastErr = e
		}
	}
	require.NoError(t, lastErr)
	assert.Equal(t, "sub-token-abc123", decrypted)
}

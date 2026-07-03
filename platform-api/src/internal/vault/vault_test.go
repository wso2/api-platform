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

package vault

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyA and keyB are two distinct valid AES-256 keys.
var (
	keyA = bytes.Repeat([]byte{0xA5}, 32)
	keyB = bytes.Repeat([]byte{0x5A}, 32)
)

func TestNewInHouseVault_RejectsBadKeySize(t *testing.T) {
	for _, size := range []int{0, 16, 31, 33, 64} {
		_, err := NewInHouseVault(bytes.Repeat([]byte{1}, size))
		assert.Error(t, err, "key size %d must be rejected", size)
	}
	v, err := NewInHouseVault(keyA)
	require.NoError(t, err)
	assert.Equal(t, keyA, v.HashKey())
}

func TestInHouseVault_EncryptDecrypt_RoundTrip(t *testing.T) {
	v, err := NewInHouseVault(keyA)
	require.NoError(t, err)

	ct, err := v.Encrypt(context.Background(), "sk-secret-value")
	require.NoError(t, err)

	pt, err := v.Decrypt(context.Background(), ct)
	require.NoError(t, err)
	assert.Equal(t, "sk-secret-value", pt)
}

func TestInHouseVault_Encrypt_EmptyPlaintext_Errors(t *testing.T) {
	v, err := NewInHouseVault(keyA)
	require.NoError(t, err)
	_, err = v.Encrypt(context.Background(), "")
	assert.Error(t, err)
}

// A ciphertext encrypted under keyA must not decrypt under keyB — this is the
// property that makes an ephemeral/rotated key lose access to stored secrets.
func TestInHouseVault_Decrypt_WrongKey_Errors(t *testing.T) {
	vA, err := NewInHouseVault(keyA)
	require.NoError(t, err)
	ct, err := vA.Encrypt(context.Background(), "sk-secret-value")
	require.NoError(t, err)

	vB, err := NewInHouseVault(keyB)
	require.NoError(t, err)
	_, err = vB.Decrypt(context.Background(), ct)
	assert.Error(t, err, "ciphertext from keyA must not decrypt under keyB")
}

func TestInHouseVault_Decrypt_TamperedCiphertext_Errors(t *testing.T) {
	v, err := NewInHouseVault(keyA)
	require.NoError(t, err)
	ct, err := v.Encrypt(context.Background(), "sk-secret-value")
	require.NoError(t, err)

	// Flip a bit in the last byte (inside the GCM tag / ciphertext) — auth must fail.
	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[len(tampered)-1] ^= 0x01
	_, err = v.Decrypt(context.Background(), tampered)
	assert.Error(t, err, "tampered ciphertext must fail authentication")
}

func TestInHouseVault_Decrypt_ShortCiphertext_Errors(t *testing.T) {
	v, err := NewInHouseVault(keyA)
	require.NoError(t, err)
	_, err = v.Decrypt(context.Background(), []byte{0x00, 0x01})
	assert.Error(t, err, "ciphertext shorter than the nonce must be rejected")
}

func TestInHouseVault_ProviderName(t *testing.T) {
	v, err := NewInHouseVault(keyA)
	require.NoError(t, err)
	assert.NotEmpty(t, v.ProviderName())
}

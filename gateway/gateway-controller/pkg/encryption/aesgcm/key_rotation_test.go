/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package aesgcm

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKeyRotation_OldPayloadsStillDecryptAfterPrimaryRotates verifies real key
// rotation: after a new primary key is added, data encrypted under the old
// primary must still decrypt (the payload carries its key version and Decrypt
// looks the key up by version), while a provider missing the old key cannot.
func TestKeyRotation_OldPayloadsStillDecryptAfterPrimaryRotates(t *testing.T) {
	t.Setenv("APIP_GW_DEVELOPMENT_MODE", "true")
	dir := t.TempDir()
	v1 := KeyConfig{Version: "aesgcm256-v1", FilePath: filepath.Join(dir, "v1.bin")}
	v2 := KeyConfig{Version: "aesgcm256-v2", FilePath: filepath.Join(dir, "v2.bin")}

	const plaintext = "gw-secret-under-rotation"

	// v1 is the only/primary key: encrypt a payload (stamped key_version=v1).
	pV1, err := NewAESGCMProvider([]KeyConfig{v1}, testLogger())
	require.NoError(t, err)
	payloadV1, err := pV1.Encrypt([]byte(plaintext))
	require.NoError(t, err)
	require.Equal(t, "aesgcm256-v1", payloadV1.KeyVersion)

	// Rotate: v2 becomes primary (first in the list), v1 kept as a secondary
	// decryption key.
	pRotated, err := NewAESGCMProvider([]KeyConfig{v2, v1}, testLogger())
	require.NoError(t, err)

	// Old v1 payload still decrypts under the rotated provider.
	got, err := pRotated.Decrypt(payloadV1)
	require.NoError(t, err, "payload encrypted under v1 must still decrypt after rotating primary to v2")
	assert.Equal(t, plaintext, string(got))

	// New payloads are encrypted under the new primary v2.
	payloadV2, err := pRotated.Encrypt([]byte(plaintext))
	require.NoError(t, err)
	require.Equal(t, "aesgcm256-v2", payloadV2.KeyVersion)

	// A provider that only knows v1 cannot decrypt a v2 payload (missing key version).
	got2, err := pV1.Decrypt(payloadV2)
	assert.Error(t, err, "a v1-only provider must not decrypt a v2 payload")
	assert.Nil(t, got2)
}

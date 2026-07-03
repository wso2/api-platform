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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests characterise the gateway's encryption-key-across-restart behaviour,
// as a contrast to the platform-api demo-mode bug (which mints a NEW in-memory key
// on every start, so stored secrets become undecryptable after a restart).
//
// The gateway is safer: in development mode it auto-generates the key ONCE and
// PERSISTS it to a file (keymgmt.go generateKeyFile), then reloads that same file
// on subsequent starts; in production mode it refuses to start without a
// provisioned key file (covered by TestKeyManager_DevModeOffNoAutoGeneration) —
// it never silently uses an ephemeral key. So a plain restart keeps secrets
// decryptable; only losing the key file (e.g. a recreated pod with no persistent
// volume for the key dir) regenerates a new key.

const gwSecretUnderTest = "gw-super-secret-value-123"

// TestDevModeKeyPersistsAcrossRestart_SecretStaysDecryptable proves the gateway
// does NOT have the platform-api per-restart bug: the dev-mode key is persisted to
// disk and reloaded on restart, so a secret encrypted before the restart still
// decrypts after it.
func TestDevModeKeyPersistsAcrossRestart_SecretStaysDecryptable(t *testing.T) {
	t.Setenv("APIP_GW_DEVELOPMENT_MODE", "true")
	keyPath := filepath.Join(t.TempDir(), "default-aesgcm256-v1.bin")
	keyCfg := []KeyConfig{{Version: "aesgcm256-v1", FilePath: keyPath}}

	// First start: dev mode auto-generates and persists the key to disk.
	p1, err := NewAESGCMProvider(keyCfg, testLogger())
	require.NoError(t, err)
	payload, err := p1.Encrypt([]byte(gwSecretUnderTest))
	require.NoError(t, err)

	// Restart: a new provider over the same key path reloads the persisted key
	// (it regenerates only when the file is missing), so the pre-restart secret
	// still decrypts.
	p2, err := NewAESGCMProvider(keyCfg, testLogger())
	require.NoError(t, err)
	plaintext, err := p2.Decrypt(payload)
	require.NoError(t, err,
		"gateway persists its dev-mode key to disk, so stored secrets must stay decryptable across restart")
	assert.Equal(t, gwSecretUnderTest, string(plaintext))
}

// TestDevModeKeyFileLost_SecretBecomesUndecryptable documents the one narrow
// condition under which the gateway hits an analogous problem: if the persisted
// key file is lost (a fresh filesystem with no persistent volume for the key
// dir), the next start regenerates a different key and prior secrets cannot be
// decrypted. This is much narrower than platform-api, which regenerates on every
// restart even when the key file / config could have been stable.
func TestDevModeKeyFileLost_SecretBecomesUndecryptable(t *testing.T) {
	t.Setenv("APIP_GW_DEVELOPMENT_MODE", "true")
	keyPath := filepath.Join(t.TempDir(), "default-aesgcm256-v1.bin")
	keyCfg := []KeyConfig{{Version: "aesgcm256-v1", FilePath: keyPath}}

	p1, err := NewAESGCMProvider(keyCfg, testLogger())
	require.NoError(t, err)
	payload, err := p1.Encrypt([]byte(gwSecretUnderTest))
	require.NoError(t, err)

	// Simulate loss of the key file (e.g. a recreated container without a
	// persistent volume for the key directory).
	require.NoError(t, os.Remove(keyPath))

	p2, err := NewAESGCMProvider(keyCfg, testLogger())
	require.NoError(t, err)
	_, err = p2.Decrypt(payload)
	assert.Error(t, err,
		"a regenerated key (after the persisted key file is lost) must not decrypt secrets encrypted with the old key")
}

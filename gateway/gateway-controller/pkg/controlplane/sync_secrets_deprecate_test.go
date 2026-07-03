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

package controlplane

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// TestSyncSecretsIncremental_DeprecatedSecret_SkippedNotPurged characterises a
// known gap: when a secret the gateway previously synced transitions to a
// non-ACTIVE status (e.g. DEPRECATED), the incremental sync SKIPS it (status !=
// "ACTIVE") and nothing removes it — the secretSyncer interface (client.go)
// exposes only UpsertFromPlatform, with no delete/purge path. So the stale
// plaintext already stored on the gateway for that handle is retained
// indefinitely on sync.
//
// The bulk path's deprecated-skip is covered by
// TestSyncSecretsBulk_DeprecatedSecret_Skipped; this covers the incremental
// (reconnect) path and asserts the non-purge behaviour explicitly. If a removal
// path is ever added, this test should change to assert the secret is purged.
func TestSyncSecretsIncremental_DeprecatedSecret_SkippedNotPurged(t *testing.T) {
	syncer := newMockSecretSyncer()
	// The gateway already holds a value for this handle from a prior ACTIVE sync.
	syncer.upserted["openai-key"] = "sk-previously-synced"
	c := stubClient(syncer)
	populateCache(c, map[string]string{"openai-key": "hmac-sha256:old"})

	// The secret now comes back DEPRECATED (with a changed hash, to show that even
	// a change does not trigger a fetch or removal for a non-ACTIVE secret).
	metas := []utils.PlatformSecretMeta{
		{ID: "uuid-1", Handle: "openai-key", DisplayName: "OpenAI Key", Hash: "hmac-sha256:new", Status: "DEPRECATED"},
	}
	fetchValue := func(string) (string, error) {
		t.Fatalf("value must not be fetched for a non-ACTIVE secret")
		return "", nil
	}

	synced, skipped, failed := syncSecretsIncrementalFromMetas(c, metas, fetchValue)

	assert.Equal(t, 0, synced)
	assert.Equal(t, 1, skipped)
	assert.Equal(t, 0, failed)

	// The gap: the stale value is still present — nothing purges a deprecated
	// secret from the gateway's local store on sync.
	assert.Equal(t, "sk-previously-synced", syncer.upserted["openai-key"],
		"deprecated secret is skipped, not purged — stale plaintext remains on the gateway (known gap)")
}

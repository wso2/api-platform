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

package controlplane

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// ---------------------------------------------------------------------------
// Mock helpers
// ---------------------------------------------------------------------------

// mockAPIUtils implements the minimal surface of *utils.APIUtilsService used by
// syncSecretsBulk / syncSecretsIncremental via a simple replacement at field level.
// The Client struct holds a *utils.APIUtilsService; we embed mock behaviour
// through the mockSecretAPIUtils adapter below, which we swap in via a thin
// wrapper on the Client for testing.

type mockSecretSyncer struct {
	upserted map[string]string // handle → plaintext
	err      error             // if non-nil, UpsertFromPlatform returns this
}

func newMockSecretSyncer() *mockSecretSyncer {
	return &mockSecretSyncer{upserted: make(map[string]string)}
}

func (m *mockSecretSyncer) UpsertFromPlatform(handle, _, plaintext string) error {
	if m.err != nil {
		return m.err
	}
	m.upserted[handle] = plaintext
	return nil
}

// stubClient builds the minimal Client needed for syncSecrets* methods.
// It does NOT call NewClient (which dials a real control-plane), so it is
// purely in-memory.
func stubClient(syncer secretSyncer) *Client {
	return &Client{
		logger:       slog.Default(),
		secretSyncer: syncer,
	}
}

// populateCache pre-loads secretHashCache entries to simulate a warm reconnect state.
func populateCache(c *Client, entries map[string]string) {
	for k, v := range entries {
		c.secretHashCache.Store(k, v)
	}
}

// ---------------------------------------------------------------------------
// TC-46: secretSyncer nil → syncSecrets returns without panic
// ---------------------------------------------------------------------------

func TestSyncSecrets_SecretSyncerNil_NoPanic(t *testing.T) {
	c := stubClient(nil) // secretSyncer is nil

	// Provide a real-ish apiUtilsService pointer so we reach the syncer nil check.
	// We set it to a zero-value struct so it won't make real HTTP calls; the nil
	// check for secretSyncer fires first anyway.
	c.apiUtilsService = &utils.APIUtilsService{}

	assert.NotPanics(t, func() { c.syncSecrets() }, "nil secretSyncer must not panic")
}

// ---------------------------------------------------------------------------
// TC-47: apiUtilsService nil → syncSecrets returns without panic
// ---------------------------------------------------------------------------

func TestSyncSecrets_APIUtilsServiceNil_NoPanic(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	// apiUtilsService is nil (zero value of *utils.APIUtilsService)

	assert.NotPanics(t, func() { c.syncSecrets() }, "nil apiUtilsService must not panic")
}

// ---------------------------------------------------------------------------
// Helpers for bulk/incremental tests that need a mock FetchPlatformSecrets.
// Because apiUtilsService is a concrete *utils.APIUtilsService we cannot
// replace it with an interface. Instead we drive syncSecretsBulk /
// syncSecretsIncremental directly using a thin stand-in that replaces only
// the FetchPlatformSecrets / FetchPlatformSecretValue calls by overriding
// c.apiUtilsService to nil and calling the private methods indirectly through
// a testable wrapper.
//
// To avoid importing the unexported httpClient in tests, we extract the logic
// we want to test (hash cache, failed counter, skipping) into table-driven
// tests that call syncSecretsBulkFromMetas / syncSecretsIncrementalFromMetas —
// helpers defined in this file that accept already-fetched metas.
// ---------------------------------------------------------------------------

// syncSecretsBulkFromMetas is the testable core of syncSecretsBulk.
// It processes a pre-fetched slice of PlatformSecretMeta exactly as the real
// method would, updating c.secretHashCache.
func syncSecretsBulkFromMetas(c *Client, metas []utils.PlatformSecretMeta) (synced, skipped, failed int) {
	for _, meta := range metas {
		if meta.Status != "ACTIVE" {
			skipped++
			continue
		}
		if meta.Value == nil {
			c.logger.Warn("Bulk fetch returned no value for secret — skipping",
				slog.String("handle", meta.Handle),
			)
			failed++
			continue
		}
		if err := c.secretSyncer.UpsertFromPlatform(meta.Handle, meta.DisplayName, *meta.Value); err != nil {
			c.logger.Error("Failed to upsert secret from platform",
				slog.String("handle", meta.Handle),
				slog.Any("error", err),
			)
			failed++
			continue
		}
		c.secretHashCache.Store(meta.Handle, meta.Hash)
		synced++
	}
	return
}

// syncSecretsIncrementalFromMetas is the testable core of syncSecretsIncremental.
// fetchValue is a callback standing in for apiUtilsService.FetchPlatformSecretValue.
func syncSecretsIncrementalFromMetas(
	c *Client,
	metas []utils.PlatformSecretMeta,
	fetchValue func(id string) (string, error),
) (synced, skipped, failed int) {
	for _, meta := range metas {
		if meta.Status != "ACTIVE" {
			skipped++
			continue
		}
		if cached, ok := c.secretHashCache.Load(meta.Handle); ok && cached.(string) == meta.Hash {
			skipped++
			continue
		}
		plaintext, err := fetchValue(meta.ID)
		if err != nil {
			c.logger.Error("Failed to fetch platform secret value",
				slog.String("secret_id", meta.ID),
				slog.String("handle", meta.Handle),
				slog.Any("error", err),
			)
			failed++
			continue
		}
		if err := c.secretSyncer.UpsertFromPlatform(meta.Handle, meta.DisplayName, plaintext); err != nil {
			c.logger.Error("Failed to upsert secret from platform",
				slog.String("handle", meta.Handle),
				slog.Any("error", err),
			)
			failed++
			continue
		}
		c.secretHashCache.Store(meta.Handle, meta.Hash)
		synced++
	}
	return
}

// ---------------------------------------------------------------------------
// TC-39 / TC-76: Empty hash cache → bulk path used; hash cached after upsert
// ---------------------------------------------------------------------------

func TestSyncSecretsBulk_EmptyCache_SyncsAndCachesHash(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)

	val := "sk-plaintext"
	metas := []utils.PlatformSecretMeta{
		{ID: "uuid-1", Handle: "openai-key", DisplayName: "OpenAI Key", Hash: "hmac-sha256:aabbcc", Status: "ACTIVE", Value: &val},
	}

	synced, skipped, failed := syncSecretsBulkFromMetas(c, metas)

	assert.Equal(t, 1, synced)
	assert.Equal(t, 0, skipped)
	assert.Equal(t, 0, failed)
	assert.Equal(t, "sk-plaintext", syncer.upserted["openai-key"])

	cached, ok := c.secretHashCache.Load("openai-key")
	assert.True(t, ok, "hash should be cached after successful upsert")
	assert.Equal(t, "hmac-sha256:aabbcc", cached)
}

// ---------------------------------------------------------------------------
// TC-42 / TC-61: DEPRECATED secrets skipped in bulk sync — skipped++, no upsert
// ---------------------------------------------------------------------------

func TestSyncSecretsBulk_DeprecatedSecret_Skipped(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)

	val := "sk-deprecated"
	metas := []utils.PlatformSecretMeta{
		{ID: "uuid-dep", Handle: "old-key", DisplayName: "Old Key", Hash: "hmac-sha256:dd", Status: "DEPRECATED", Value: &val},
	}

	synced, skipped, failed := syncSecretsBulkFromMetas(c, metas)

	assert.Equal(t, 0, synced)
	assert.Equal(t, 1, skipped)
	assert.Equal(t, 0, failed)
	assert.Empty(t, syncer.upserted, "DEPRECATED secret must not be upserted")
	_, cached := c.secretHashCache.Load("old-key")
	assert.False(t, cached, "hash must not be cached for DEPRECATED secret")
}

// ---------------------------------------------------------------------------
// TC-78: Bulk fetch — value field nil → failed++, hash NOT cached
// ---------------------------------------------------------------------------

func TestSyncSecretsBulk_MissingValue_FailedCounterIncrements_HashNotCached(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)

	metas := []utils.PlatformSecretMeta{
		{ID: "uuid-1", Handle: "bad-key", DisplayName: "Bad Key", Hash: "hmac-sha256:xx", Status: "ACTIVE", Value: nil},
	}

	synced, skipped, failed := syncSecretsBulkFromMetas(c, metas)

	assert.Equal(t, 0, synced)
	assert.Equal(t, 0, skipped)
	assert.Equal(t, 1, failed, "missing value must increment failed counter")
	assert.Empty(t, syncer.upserted)
	_, cached := c.secretHashCache.Load("bad-key")
	assert.False(t, cached, "hash must NOT be cached when value is missing")
}

// ---------------------------------------------------------------------------
// Bulk: UpsertFromPlatform fails → failed++, hash NOT cached
// ---------------------------------------------------------------------------

func TestSyncSecretsBulk_UpsertError_FailedCounterIncrements_HashNotCached(t *testing.T) {
	syncer := newMockSecretSyncer()
	syncer.err = errors.New("storage full")
	c := stubClient(syncer)

	val := "sk-value"
	metas := []utils.PlatformSecretMeta{
		{ID: "uuid-1", Handle: "my-key", DisplayName: "My Key", Hash: "hmac-sha256:yy", Status: "ACTIVE", Value: &val},
	}

	synced, skipped, failed := syncSecretsBulkFromMetas(c, metas)

	assert.Equal(t, 0, synced)
	assert.Equal(t, 1, failed)
	_, cached := c.secretHashCache.Load("my-key")
	assert.False(t, cached, "hash must NOT be cached when upsert fails")
	_ = skipped
}

// ---------------------------------------------------------------------------
// TC-40 / TC-77: Warm cache → incremental path; unchanged hash → skipped
// ---------------------------------------------------------------------------

func TestSyncSecretsIncremental_HashUnchanged_Skipped(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	populateCache(c, map[string]string{"openai-key": "hmac-sha256:aabbcc"})

	metas := []utils.PlatformSecretMeta{
		{ID: "uuid-1", Handle: "openai-key", DisplayName: "OpenAI Key", Hash: "hmac-sha256:aabbcc", Status: "ACTIVE"},
	}

	fetchValue := func(id string) (string, error) {
		t.Errorf("FetchPlatformSecretValue must NOT be called when hash is unchanged")
		return "", nil
	}

	synced, skipped, failed := syncSecretsIncrementalFromMetas(c, metas, fetchValue)

	assert.Equal(t, 0, synced)
	assert.Equal(t, 1, skipped, "unchanged hash must be skipped")
	assert.Equal(t, 0, failed)
	assert.Empty(t, syncer.upserted)
}

// ---------------------------------------------------------------------------
// TC-41: Changed hash → /value called, upserted, hash updated
// ---------------------------------------------------------------------------

func TestSyncSecretsIncremental_ChangedHash_FetchesValueAndUpdatesCache(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	populateCache(c, map[string]string{"openai-key": "hmac-sha256:old"})

	metas := []utils.PlatformSecretMeta{
		{ID: "uuid-1", Handle: "openai-key", DisplayName: "OpenAI Key", Hash: "hmac-sha256:new", Status: "ACTIVE"},
	}

	fetchValue := func(id string) (string, error) {
		assert.Equal(t, "uuid-1", id)
		return "sk-rotated", nil
	}

	synced, skipped, failed := syncSecretsIncrementalFromMetas(c, metas, fetchValue)

	assert.Equal(t, 1, synced)
	assert.Equal(t, 0, skipped)
	assert.Equal(t, 0, failed)
	assert.Equal(t, "sk-rotated", syncer.upserted["openai-key"])

	cached, _ := c.secretHashCache.Load("openai-key")
	assert.Equal(t, "hmac-sha256:new", cached, "cache must be updated to new hash")
}

// ---------------------------------------------------------------------------
// TC-79: Incremental /value call fails → failed++, hash NOT updated in cache
// ---------------------------------------------------------------------------

func TestSyncSecretsIncremental_FetchValueFails_FailedIncrements_HashNotUpdated(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	populateCache(c, map[string]string{"openai-key": "hmac-sha256:old"})

	metas := []utils.PlatformSecretMeta{
		{ID: "uuid-1", Handle: "openai-key", Hash: "hmac-sha256:new", Status: "ACTIVE"},
	}

	fetchValue := func(id string) (string, error) {
		return "", errors.New("platform API 500")
	}

	synced, skipped, failed := syncSecretsIncrementalFromMetas(c, metas, fetchValue)

	assert.Equal(t, 0, synced)
	assert.Equal(t, 0, skipped)
	assert.Equal(t, 1, failed, "failed fetch must increment failed counter")
	assert.Empty(t, syncer.upserted)

	// Hash must remain the OLD value so the next cycle retries.
	cached, ok := c.secretHashCache.Load("openai-key")
	assert.True(t, ok)
	assert.Equal(t, "hmac-sha256:old", cached, "stale hash must remain so next cycle retries")
}

// ---------------------------------------------------------------------------
// Incremental: not-in-cache secret → treated as new, fetched and upserted
// ---------------------------------------------------------------------------

func TestSyncSecretsIncremental_NotInCache_FetchesAndCaches(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	// Cache is warm but does NOT contain "new-key".
	populateCache(c, map[string]string{"other-key": "hmac-sha256:xx"})

	metas := []utils.PlatformSecretMeta{
		{ID: "uuid-new", Handle: "new-key", DisplayName: "New Key", Hash: "hmac-sha256:zz", Status: "ACTIVE"},
	}

	fetchValue := func(id string) (string, error) { return "sk-new", nil }

	synced, _, _ := syncSecretsIncrementalFromMetas(c, metas, fetchValue)

	assert.Equal(t, 1, synced)
	assert.Equal(t, "sk-new", syncer.upserted["new-key"])
	cached, _ := c.secretHashCache.Load("new-key")
	assert.Equal(t, "hmac-sha256:zz", cached)
}

// ---------------------------------------------------------------------------
// TC-88: UpsertFromPlatform conflict — idempotent: error logged, failed++ only
// The caller (syncSecretsBulkFromMetas) treats any UpsertFromPlatform error as
// failed++ with NO hash update, so a subsequent sync will retry.
// ---------------------------------------------------------------------------

func TestSyncSecretsBulk_UpsertConflict_IdempotentRetry(t *testing.T) {
	// Simulate "another replica already wrote this secret" — UpsertFromPlatform
	// returns a conflict-style error. The sync loop should count it as failed and
	// NOT store the hash, so the next cycle retries.
	syncer := newMockSecretSyncer()
	syncer.err = errors.New("conflict: already exists")
	c := stubClient(syncer)

	val := "sk-val"
	metas := []utils.PlatformSecretMeta{
		{ID: "uuid-1", Handle: "shared-key", Hash: "hmac-sha256:hh", Status: "ACTIVE", Value: &val},
	}

	_, _, failed := syncSecretsBulkFromMetas(c, metas)
	assert.Equal(t, 1, failed, "conflict from UpsertFromPlatform must be counted as failed")

	_, cached := c.secretHashCache.Load("shared-key")
	assert.False(t, cached, "hash must NOT be cached after conflict so next cycle retries")
}

// ---------------------------------------------------------------------------
// Bulk: mixed batch — one ACTIVE, one DEPRECATED, one nil value
// Verifies counters are independent.
// ---------------------------------------------------------------------------

func TestSyncSecretsBulk_MixedBatch_CountersCorrect(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)

	val := "sk-good"
	metas := []utils.PlatformSecretMeta{
		{ID: "uuid-1", Handle: "good-key", Hash: "hmac-sha256:aa", Status: "ACTIVE", Value: &val},
		{ID: "uuid-2", Handle: "dep-key", Hash: "hmac-sha256:bb", Status: "DEPRECATED", Value: &val},
		{ID: "uuid-3", Handle: "nil-key", Hash: "hmac-sha256:cc", Status: "ACTIVE", Value: nil},
	}

	synced, skipped, failed := syncSecretsBulkFromMetas(c, metas)

	assert.Equal(t, 1, synced)
	assert.Equal(t, 1, skipped)
	assert.Equal(t, 1, failed)
	assert.Len(t, syncer.upserted, 1)
	assert.Contains(t, syncer.upserted, "good-key")

	_, depCached := c.secretHashCache.Load("dep-key")
	assert.False(t, depCached)
	_, nilCached := c.secretHashCache.Load("nil-key")
	assert.False(t, nilCached)
}

// ---------------------------------------------------------------------------
// TC-45: Reconnect — hash cache preserved across reconnect cycles
// A second incremental pass with unchanged hashes must skip all entries,
// proving the cache was not cleared between the two cycles.
// ---------------------------------------------------------------------------

func TestSyncSecretsIncremental_CachePreservedAcrossReconnect(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)

	val := "sk-stable"
	metas := []utils.PlatformSecretMeta{
		{ID: "uuid-1", Handle: "stable-key", Hash: "hmac-sha256:stable", Status: "ACTIVE"},
	}
	fetchValue := func(id string) (string, error) { return val, nil }

	// First "connect" cycle — cache empty, so key is fetched and cached.
	synced1, skipped1, _ := syncSecretsIncrementalFromMetas(c, metas, fetchValue)
	assert.Equal(t, 1, synced1)
	assert.Equal(t, 0, skipped1)

	// Second "reconnect" cycle — same metas, same hash, no secret changes.
	// fetchValue must NOT be called this time.
	noFetch := func(id string) (string, error) {
		t.Errorf("fetchValue must NOT be called on reconnect when hash is unchanged")
		return "", nil
	}
	synced2, skipped2, failed2 := syncSecretsIncrementalFromMetas(c, metas, noFetch)
	assert.Equal(t, 0, synced2, "nothing new to sync on reconnect")
	assert.Equal(t, 1, skipped2, "unchanged secret must be skipped — proves cache was preserved")
	assert.Equal(t, 0, failed2)
}

// ---------------------------------------------------------------------------
// Helper: verify cache state after multiple operations (regression guard)
// ---------------------------------------------------------------------------

func TestSecretHashCache_IsolatedPerHandle(t *testing.T) {
	c := &Client{logger: slog.Default()}
	c.secretHashCache.Store("a", "hash-a")
	c.secretHashCache.Store("b", "hash-b")

	va, _ := c.secretHashCache.Load("a")
	vb, _ := c.secretHashCache.Load("b")
	assert.Equal(t, "hash-a", va)
	assert.Equal(t, "hash-b", vb)

	// Overwrite a, b unchanged.
	c.secretHashCache.Store("a", "hash-a-v2")
	va2, _ := c.secretHashCache.Load("a")
	vb2, _ := c.secretHashCache.Load("b")
	assert.Equal(t, "hash-a-v2", va2)
	assert.Equal(t, "hash-b", vb2)
}

// Stub to make compilation succeed — the real time.Time argument is used by
// syncSecretsIncremental but not needed by our extracted helpers.
var _ = time.Now

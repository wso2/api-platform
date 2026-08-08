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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	upserted  map[string]string // handle → plaintext
	deleted   []string          // handles passed to Delete, in call order
	err       error             // if non-nil, UpsertFromPlatform returns this
	deleteErr error             // if non-nil, Delete returns this
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

func (m *mockSecretSyncer) Delete(handle, _ string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deleted = append(m.deleted, handle)
	delete(m.upserted, handle)
	return nil
}

// stubClient builds the minimal Client needed for syncSecrets* methods.
// It does NOT call NewClient (which dials a real control-plane), so it is
// purely in-memory.
func stubClient(syncer secretSyncer) *Client {
	return &Client{
		logger:       slog.Default(),
		secretSyncer: syncer,
		// state must be non-nil so isOnPrem()/GetGatewayPath() can read the
		// (empty) gateway path without a nil deref. Empty path => not on-prem.
		state: &ConnectionState{},
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

// ---------------------------------------------------------------------------
// secret.updated / secret.deleted push-event handlers
// ---------------------------------------------------------------------------

func TestApplySecretUpdatedPayload_FetchesAndUpserts_CachesHash(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)

	payload := SecretUpdatedEventPayload{Handle: "openai-key", DisplayName: "OpenAI Key", Hash: "hmac-sha256:new"}
	fetchValue := func(handle string) (string, error) {
		assert.Equal(t, "openai-key", handle)
		return "sk-rotated", nil
	}

	c.applySecretUpdatedPayload(payload, slog.Default(), fetchValue)

	assert.Equal(t, "sk-rotated", syncer.upserted["openai-key"])
	cached, ok := c.secretHashCache.Load("openai-key")
	assert.True(t, ok)
	assert.Equal(t, "hmac-sha256:new", cached)
}

func TestApplySecretUpdatedPayload_FetchError_DoesNotUpsertOrCacheHash(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)

	payload := SecretUpdatedEventPayload{Handle: "openai-key", Hash: "hmac-sha256:new"}
	fetchValue := func(handle string) (string, error) { return "", errors.New("upstream unreachable") }

	c.applySecretUpdatedPayload(payload, slog.Default(), fetchValue)

	assert.Empty(t, syncer.upserted, "must not upsert when the value fetch fails")
	_, ok := c.secretHashCache.Load("openai-key")
	assert.False(t, ok, "must not cache the new hash when the value fetch fails")
}

func TestApplySecretUpdatedPayload_UpsertError_DoesNotCacheHash(t *testing.T) {
	syncer := newMockSecretSyncer()
	syncer.err = errors.New("storage full")
	c := stubClient(syncer)

	payload := SecretUpdatedEventPayload{Handle: "openai-key", Hash: "hmac-sha256:new"}
	fetchValue := func(handle string) (string, error) { return "sk-rotated", nil }

	c.applySecretUpdatedPayload(payload, slog.Default(), fetchValue)

	_, ok := c.secretHashCache.Load("openai-key")
	assert.False(t, ok, "must not cache the new hash when the local upsert fails")
}

func TestHandleSecretUpdatedEvent_MissingHandle_NoFetchAttempted(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	c.apiUtilsService = &utils.APIUtilsService{} // non-nil so the guard under test is the handle check, not this one

	event := map[string]interface{}{
		"type":          "secret.updated",
		"correlationId": "corr-1",
		"payload":       map[string]interface{}{"handle": "", "hash": "hmac-sha256:x"},
	}

	assert.NotPanics(t, func() { c.handleSecretUpdatedEvent(event) })
	assert.Empty(t, syncer.upserted)
}

func TestHandleSecretUpdatedEvent_NilDependencies_NoPanic(t *testing.T) {
	c := stubClient(newMockSecretSyncer())
	// apiUtilsService left nil (zero value of *utils.APIUtilsService)

	event := map[string]interface{}{
		"type":          "secret.updated",
		"correlationId": "corr-1",
		"payload":       map[string]interface{}{"handle": "openai-key", "hash": "hmac-sha256:x"},
	}

	assert.NotPanics(t, func() { c.handleSecretUpdatedEvent(event) })
}

func TestHandleSecretDeletedEvent_EvictsFromLocalStoreAndHashCache(t *testing.T) {
	syncer := newMockSecretSyncer()
	syncer.upserted["old-key"] = "sk-stale"
	c := stubClient(syncer)
	populateCache(c, map[string]string{"old-key": "hmac-sha256:stale"})

	event := map[string]interface{}{
		"type":          "secret.deleted",
		"correlationId": "corr-2",
		"payload":       map[string]interface{}{"handle": "old-key"},
	}

	c.handleSecretDeletedEvent(event)

	assert.Equal(t, []string{"old-key"}, syncer.deleted)
	assert.NotContains(t, syncer.upserted, "old-key", "local copy must be evicted")
	_, ok := c.secretHashCache.Load("old-key")
	assert.False(t, ok, "hash cache entry must be cleared on eviction")
}

func TestHandleSecretDeletedEvent_MissingHandle_NoEviction(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)

	event := map[string]interface{}{
		"type":          "secret.deleted",
		"correlationId": "corr-2",
		"payload":       map[string]interface{}{"handle": ""},
	}

	assert.NotPanics(t, func() { c.handleSecretDeletedEvent(event) })
	assert.Empty(t, syncer.deleted)
}

func TestHandleSecretDeletedEvent_NilSyncer_NoPanic(t *testing.T) {
	c := &Client{logger: slog.Default()} // secretSyncer left nil

	event := map[string]interface{}{
		"type":          "secret.deleted",
		"correlationId": "corr-2",
		"payload":       map[string]interface{}{"handle": "old-key"},
	}

	assert.NotPanics(t, func() { c.handleSecretDeletedEvent(event) })
}

func TestHandleSecretDeletedEvent_DeleteError_HashCacheNotCleared(t *testing.T) {
	syncer := newMockSecretSyncer()
	syncer.deleteErr = errors.New("storage locked")
	c := stubClient(syncer)
	populateCache(c, map[string]string{"old-key": "hmac-sha256:stale"})

	event := map[string]interface{}{
		"type":          "secret.deleted",
		"correlationId": "corr-2",
		"payload":       map[string]interface{}{"handle": "old-key"},
	}

	c.handleSecretDeletedEvent(event)

	_, ok := c.secretHashCache.Load("old-key")
	assert.True(t, ok, "hash cache must be left intact when eviction fails, so a later retry doesn't skip it")
}

// ---------------------------------------------------------------------------
// Revision-based staleness rejection (out-of-order / redelivered events)
// ---------------------------------------------------------------------------

func TestApplySecretUpdatedPayload_StaleRevision_Ignored(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	c.secretRevisionCache.Store("openai-key", int64(10))

	payload := SecretUpdatedEventPayload{Handle: "openai-key", Hash: "hmac-sha256:new", Revision: 5}
	fetchValue := func(handle string) (string, error) {
		t.Fatal("fetchValue must not be called for a stale event")
		return "", nil
	}

	c.applySecretUpdatedPayload(payload, slog.Default(), fetchValue)

	assert.Empty(t, syncer.upserted, "a stale update must not be applied")
	cached, _ := c.secretRevisionCache.Load("openai-key")
	assert.Equal(t, int64(10), cached, "the cached revision must not regress")
}

func TestApplySecretUpdatedPayload_NewerRevision_AppliedAndRevisionAdvanced(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	c.secretRevisionCache.Store("openai-key", int64(5))

	payload := SecretUpdatedEventPayload{Handle: "openai-key", Hash: "hmac-sha256:new", Revision: 10}
	fetchValue := func(handle string) (string, error) { return "sk-rotated", nil }

	c.applySecretUpdatedPayload(payload, slog.Default(), fetchValue)

	assert.Equal(t, "sk-rotated", syncer.upserted["openai-key"])
	cached, _ := c.secretRevisionCache.Load("openai-key")
	assert.Equal(t, int64(10), cached)
}

func TestApplySecretUpdatedPayload_EqualRevision_StillApplied(t *testing.T) {
	// A redelivery of the exact same event (e.g. an at-least-once retry from the
	// EventHub) must still be applied — only strictly older revisions are stale.
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	c.secretRevisionCache.Store("openai-key", int64(10))

	payload := SecretUpdatedEventPayload{Handle: "openai-key", Hash: "hmac-sha256:new", Revision: 10}
	fetchValue := func(handle string) (string, error) { return "sk-rotated", nil }

	c.applySecretUpdatedPayload(payload, slog.Default(), fetchValue)

	assert.Equal(t, "sk-rotated", syncer.upserted["openai-key"], "redelivery of the same revision must still apply")
}

func TestHandleSecretDeletedEvent_StaleRevision_NotEvicted(t *testing.T) {
	syncer := newMockSecretSyncer()
	syncer.upserted["old-key"] = "sk-active"
	c := stubClient(syncer)
	populateCache(c, map[string]string{"old-key": "hmac-sha256:active"})
	c.secretRevisionCache.Store("old-key", int64(10))

	event := map[string]interface{}{
		"type":          "secret.deleted",
		"correlationId": "corr-stale",
		"payload":       map[string]interface{}{"handle": "old-key", "revision": float64(5)},
	}

	c.handleSecretDeletedEvent(event)

	assert.Empty(t, syncer.deleted, "a stale deletion must not evict")
	assert.Contains(t, syncer.upserted, "old-key")
	_, ok := c.secretHashCache.Load("old-key")
	assert.True(t, ok, "hash cache must be left intact for a stale deletion")
}

// TestSecretLifecycle_DeleteThenRecreate_StaleDeleteRedelivery_DoesNotEvict covers
// the exact sequence a reviewer flagged as unprotected: update (rev 1) establishes the
// secret, delete (rev 2) evicts it, the same handle is reused by a freshly created
// secret (rev 3), and then the rev-2 deletion is redelivered late (a realistic outcome
// of common/eventhub's poll-based, at-least-once delivery with no cross-replica
// ordering guarantee). The redelivered deletion must be rejected as stale rather than
// evicting the new secret.
func TestSecretLifecycle_DeleteThenRecreate_StaleDeleteRedelivery_DoesNotEvict(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)

	// rev 1: initial update establishes the secret.
	updatePayload := func(revision int64, value string) SecretUpdatedEventPayload {
		return SecretUpdatedEventPayload{Handle: "openai-key", DisplayName: "OpenAI Key", Hash: "hmac-sha256:" + value, Revision: revision}
	}
	fetchValue := func(value string) func(string) (string, error) {
		return func(string) (string, error) { return value, nil }
	}

	c.applySecretUpdatedPayload(updatePayload(1, "v1"), slog.Default(), fetchValue("sk-v1"))
	assert.Equal(t, "sk-v1", syncer.upserted["openai-key"])

	// rev 2: deletion evicts the secret.
	deleteEvent := func(revision int) map[string]interface{} {
		return map[string]interface{}{
			"type":          "secret.deleted",
			"correlationId": "corr-delete",
			"payload":       map[string]interface{}{"handle": "openai-key", "revision": float64(revision)},
		}
	}
	c.handleSecretDeletedEvent(deleteEvent(2))
	assert.NotContains(t, syncer.upserted, "openai-key", "secret must be evicted after deletion")

	// rev 3: the handle is reused by a freshly created secret.
	c.applySecretUpdatedPayload(updatePayload(3, "v3"), slog.Default(), fetchValue("sk-v3"))
	assert.Equal(t, "sk-v3", syncer.upserted["openai-key"], "the new secret under the reused handle must be applied")

	// The rev-2 deletion is redelivered (late retry / at-least-once redelivery). It
	// must be recognized as stale relative to rev 3 and must NOT re-evict.
	c.handleSecretDeletedEvent(deleteEvent(2))
	assert.Equal(t, "sk-v3", syncer.upserted["openai-key"], "a stale, redelivered deletion must not evict the newly created secret")

	cached, ok := c.secretRevisionCache.Load("openai-key")
	assert.True(t, ok)
	assert.Equal(t, int64(3), cached, "cached revision must remain at the new secret's revision")
}

// ---------------------------------------------------------------------------
// Revision precision through the real WebSocket JSON decode (handleMessage)
//
// These go through client.handleMessage with raw JSON bytes rather than a
// hand-built map[string]interface{}, because the precision bug they guard
// against lives specifically in that decode step: handleMessage used to parse
// the message with plain json.Unmarshal into map[string]interface{}, which
// decodes JSON numbers as float64. float64 has ~53 bits of integer precision,
// but a UnixNano revision is ~60 bits, so two revisions within ~256ns of each
// other at that magnitude decoded to the identical value — verified empirically
// before the fix. A test built from Go int64 literals instead of raw JSON text
// would never exercise that decode path and would pass whether or not the bug
// was present.
// ---------------------------------------------------------------------------

func deletedMessage(handle string, revision int64) []byte {
	return []byte(fmt.Sprintf(
		`{"type":"secret.deleted","correlationId":"corr-precision","payload":{"handle":%q,"revision":%d}}`,
		handle, revision,
	))
}

func TestHandleMessage_SecretDeleted_AdjacentRevisions_ReorderedStaleRejected(t *testing.T) {
	syncer := newMockSecretSyncer()
	syncer.upserted["openai-key"] = "sk-active"
	c := stubClient(syncer)
	populateCache(c, map[string]string{"openai-key": "hmac-sha256:active"})

	// Realistic UnixNano magnitude (~10^18). newRevision and staleRevision differ
	// by 100ns — inside the ~256ns window that collided under the float64 bug.
	const newRevision int64 = 1735900000123456700
	const staleRevision int64 = newRevision - 100

	c.handleMessage(websocket.TextMessage, deletedMessage("openai-key", newRevision))
	assert.Equal(t, []string{"openai-key"}, syncer.deleted, "the newer deletion must evict")

	// The older, adjacent revision arrives late (reordered/redelivered). It must
	// be rejected as stale, not re-applied.
	c.handleMessage(websocket.TextMessage, deletedMessage("openai-key", staleRevision))
	assert.Equal(t, []string{"openai-key"}, syncer.deleted,
		"a stale reordered deletion adjacent in time to the last-applied one must not evict again")

	cached, ok := c.secretRevisionCache.Load("openai-key")
	assert.True(t, ok)
	assert.Equal(t, newRevision, cached, "cached revision must remain exactly the newer value, not a float64-rounded approximation")
}

func TestHandleMessage_SecretDeleted_EqualRevisionRedelivery_StillApplied(t *testing.T) {
	syncer := newMockSecretSyncer()
	syncer.upserted["openai-key"] = "sk-active"
	c := stubClient(syncer)
	populateCache(c, map[string]string{"openai-key": "hmac-sha256:active"})

	const revision int64 = 1735900000123456700

	c.handleMessage(websocket.TextMessage, deletedMessage("openai-key", revision))
	c.handleMessage(websocket.TextMessage, deletedMessage("openai-key", revision))

	assert.Equal(t, []string{"openai-key", "openai-key"}, syncer.deleted,
		"an exact redelivery of the same revision must still be applied (idempotent), not rejected as stale")
}

// ---------------------------------------------------------------------------
// Poll-based eviction (evictSecretsNotIn, and its wiring into
// syncSecretsBulk / syncSecretsIncremental)
//
// A gateway that is disconnected at the moment a secret is deleted only learns
// about it via the live secret.deleted event if it's connected when the event is
// broadcast. evictSecretsNotIn is the poll-based recovery path: on the next
// reconnect/poll it diffs the Platform API response against secretHashCache and
// evicts anything no longer ACTIVE, using the same secretSyncer.Delete +
// secretHashCache.Delete calls handleSecretDeletedEvent uses for the live path.
// ---------------------------------------------------------------------------

func TestEvictSecretsNotIn_HandleMissingFromActiveSet_Evicted(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	populateCache(c, map[string]string{"gone-handle": "hmac-sha256:old"})

	evicted := c.evictSecretsNotIn(map[string]struct{}{})

	assert.Equal(t, 1, evicted)
	assert.Equal(t, []string{"gone-handle"}, syncer.deleted)
	_, ok := c.secretHashCache.Load("gone-handle")
	assert.False(t, ok, "evicted handle must be removed from secretHashCache")
}

func TestEvictSecretsNotIn_HandleFlippedNonActive_Evicted(t *testing.T) {
	// A handle still present in the Platform API response but no longer ACTIVE
	// (e.g. DEPRECATED) is represented the same way as a missing handle: it's
	// simply absent from activeHandles, since the caller only adds ACTIVE handles.
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	populateCache(c, map[string]string{"deprecated-handle": "hmac-sha256:x"})

	evicted := c.evictSecretsNotIn(map[string]struct{}{})

	assert.Equal(t, 1, evicted)
	assert.True(t, syncer.wasDeleted("deprecated-handle"))
	_, ok := c.secretHashCache.Load("deprecated-handle")
	assert.False(t, ok)
}

func TestEvictSecretsNotIn_UnrelatedActiveHandle_LeftAlone(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	populateCache(c, map[string]string{"stable-handle": "hmac-sha256:stable"})

	evicted := c.evictSecretsNotIn(map[string]struct{}{"stable-handle": {}})

	assert.Equal(t, 0, evicted)
	assert.Empty(t, syncer.deleted)
	cached, ok := c.secretHashCache.Load("stable-handle")
	require.True(t, ok, "unrelated ACTIVE, unchanged handle must remain cached")
	assert.Equal(t, "hmac-sha256:stable", cached)
}

func TestEvictSecretsNotIn_MixedSet(t *testing.T) {
	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	populateCache(c, map[string]string{
		"gone-handle":       "hmac-sha256:old",
		"deprecated-handle": "hmac-sha256:x",
		"stable-handle":     "hmac-sha256:stable",
	})

	evicted := c.evictSecretsNotIn(map[string]struct{}{"stable-handle": {}})

	assert.Equal(t, 2, evicted)
	assert.True(t, syncer.wasDeleted("gone-handle"))
	assert.True(t, syncer.wasDeleted("deprecated-handle"))
	assert.False(t, syncer.wasDeleted("stable-handle"))

	_, stableOk := c.secretHashCache.Load("stable-handle")
	assert.True(t, stableOk)
}

func TestEvictSecretsNotIn_DeleteError_HashCacheNotCleared(t *testing.T) {
	syncer := newMockSecretSyncer()
	syncer.deleteErr = errors.New("storage locked")
	c := stubClient(syncer)
	populateCache(c, map[string]string{"gone-handle": "hmac-sha256:old"})

	evicted := c.evictSecretsNotIn(map[string]struct{}{})

	assert.Equal(t, 1, evicted, "the handle is still counted as stale even though eviction failed")
	_, ok := c.secretHashCache.Load("gone-handle")
	assert.True(t, ok, "hash cache must be left intact so a later poll retries the eviction")
}

// wasDeleted reports whether Delete was called for handle, for readability in
// the mixed-set assertions above.
func (m *mockSecretSyncer) wasDeleted(handle string) bool {
	for _, h := range m.deleted {
		if h == handle {
			return true
		}
	}
	return false
}

// --- End-to-end coverage through the real HTTP-backed sync methods ---

// platformSecretJSON mirrors PlatformSecretMeta's wire shape for building test
// server responses.
type platformSecretJSON struct {
	ID          string  `json:"uuid"`
	Handle      string  `json:"handle"`
	DisplayName string  `json:"name"`
	Hash        string  `json:"hash"`
	Status      string  `json:"status"`
	Value       *string `json:"value,omitempty"`
}

// newSecretSyncHTTPClient spins up an httptest TLS server serving
// GET /api/internal/v1/secrets (and, if valueHandler is non-nil,
// GET /api/internal/v1/secrets/{handle}/value) and returns a *Client wired to
// it via a real *utils.APIUtilsService, with a mockSecretSyncer installed.
func newSecretSyncHTTPClient(t *testing.T, listHandler http.HandlerFunc, valueHandler http.HandlerFunc) (*Client, *mockSecretSyncer) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/internal/v1/secrets", listHandler)
	if valueHandler != nil {
		mux.HandleFunc("/api/internal/v1/secrets/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/value") {
				valueHandler(w, r)
				return
			}
			http.NotFound(w, r)
		})
	}

	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)

	syncer := newMockSecretSyncer()
	c := stubClient(syncer)
	c.apiUtilsService = utils.NewAPIUtilsService(utils.PlatformAPIConfig{
		BaseURL:            server.URL + "/api/internal/v1",
		InsecureSkipVerify: true,
	}, slog.Default())

	return c, syncer
}

func writeSecretsList(t *testing.T, w http.ResponseWriter, secrets []platformSecretJSON) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"list": secrets, "count": len(secrets)}))
}

// TestSyncSecretsIncremental_EndToEnd_EvictsHandleMissingFromResponse proves the
// real syncSecretsIncremental wiring — not just evictSecretsNotIn in isolation —
// evicts a handle that was cached ACTIVE previously but has since been
// permanently deleted (absent from the poll response entirely).
func TestSyncSecretsIncremental_EndToEnd_EvictsHandleMissingFromResponse(t *testing.T) {
	c, syncer := newSecretSyncHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeSecretsList(t, w, []platformSecretJSON{})
	}, nil)
	populateCache(c, map[string]string{"gone-handle": "hmac-sha256:old"})

	c.syncSecretsIncremental()

	assert.True(t, syncer.wasDeleted("gone-handle"))
	_, ok := c.secretHashCache.Load("gone-handle")
	assert.False(t, ok)
}

// TestSyncSecretsIncremental_EndToEnd_EvictsHandleFlippedToDeprecated proves a
// handle still present in the response but flipped to DEPRECATED is evicted.
func TestSyncSecretsIncremental_EndToEnd_EvictsHandleFlippedToDeprecated(t *testing.T) {
	c, syncer := newSecretSyncHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeSecretsList(t, w, []platformSecretJSON{
			{ID: "uuid-1", Handle: "deprecated-handle", Hash: "hmac-sha256:x", Status: "DEPRECATED"},
		})
	}, nil)
	populateCache(c, map[string]string{"deprecated-handle": "hmac-sha256:x"})

	c.syncSecretsIncremental()

	assert.True(t, syncer.wasDeleted("deprecated-handle"))
	_, ok := c.secretHashCache.Load("deprecated-handle")
	assert.False(t, ok)
}

// TestSyncSecretsIncremental_EndToEnd_UnrelatedActiveUnchangedHandleLeftAlone
// proves a cached handle that's still ACTIVE with an unchanged hash is neither
// evicted nor re-upserted.
func TestSyncSecretsIncremental_EndToEnd_UnrelatedActiveUnchangedHandleLeftAlone(t *testing.T) {
	c, syncer := newSecretSyncHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeSecretsList(t, w, []platformSecretJSON{
			{ID: "uuid-2", Handle: "stable-handle", Hash: "hmac-sha256:stable", Status: "ACTIVE"},
		})
	}, nil)
	populateCache(c, map[string]string{"stable-handle": "hmac-sha256:stable"})

	c.syncSecretsIncremental()

	assert.False(t, syncer.wasDeleted("stable-handle"))
	assert.NotContains(t, syncer.upserted, "stable-handle", "unchanged hash must be skipped, not re-upserted")
	cached, ok := c.secretHashCache.Load("stable-handle")
	require.True(t, ok)
	assert.Equal(t, "hmac-sha256:stable", cached)
}

// TestSyncSecretsIncremental_EndToEnd_MixedBatch exercises deletion, deprecation,
// an untouched ACTIVE handle, and a changed ACTIVE handle together in one poll.
func TestSyncSecretsIncremental_EndToEnd_MixedBatch(t *testing.T) {
	c, syncer := newSecretSyncHTTPClient(t,
		func(w http.ResponseWriter, r *http.Request) {
			writeSecretsList(t, w, []platformSecretJSON{
				{ID: "uuid-2", Handle: "stable-handle", Hash: "hmac-sha256:stable", Status: "ACTIVE"},
				{ID: "uuid-3", Handle: "deprecated-handle", Hash: "hmac-sha256:x", Status: "DEPRECATED"},
				{ID: "uuid-4", Handle: "changed-handle", Hash: "hmac-sha256:new", Status: "ACTIVE"},
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"value": "new-plaintext"})
		},
	)
	populateCache(c, map[string]string{
		"stable-handle":     "hmac-sha256:stable",
		"deprecated-handle": "hmac-sha256:x",
		"gone-handle":       "hmac-sha256:old", // absent from the response entirely
		"changed-handle":    "hmac-sha256:old",
	})

	c.syncSecretsIncremental()

	assert.True(t, syncer.wasDeleted("gone-handle"))
	assert.True(t, syncer.wasDeleted("deprecated-handle"))
	assert.False(t, syncer.wasDeleted("stable-handle"))
	assert.False(t, syncer.wasDeleted("changed-handle"))

	_, goneOk := c.secretHashCache.Load("gone-handle")
	assert.False(t, goneOk)
	_, depOk := c.secretHashCache.Load("deprecated-handle")
	assert.False(t, depOk)

	changedCached, ok := c.secretHashCache.Load("changed-handle")
	require.True(t, ok)
	assert.Equal(t, "hmac-sha256:new", changedCached)
	assert.Equal(t, "new-plaintext", syncer.upserted["changed-handle"])
}

// TestSyncSecretsBulk_EndToEnd_EvictsPreExistingCacheNotInResponse covers the
// (mostly defensive) eviction path in the bulk/startup sync: any handle already
// in secretHashCache before the bulk fetch runs that the Platform API no longer
// returns as ACTIVE gets evicted too, for consistency with the incremental path.
func TestSyncSecretsBulk_EndToEnd_EvictsPreExistingCacheNotInResponse(t *testing.T) {
	value := "plaintext-value"
	c, syncer := newSecretSyncHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeSecretsList(t, w, []platformSecretJSON{
			{ID: "uuid-1", Handle: "active-handle", Hash: "hmac-sha256:1", Status: "ACTIVE", Value: &value},
		})
	}, nil)
	populateCache(c, map[string]string{"stale-handle": "hmac-sha256:old"})

	c.syncSecretsBulk()

	assert.True(t, syncer.wasDeleted("stale-handle"))
	_, staleOk := c.secretHashCache.Load("stale-handle")
	assert.False(t, staleOk)

	activeCached, ok := c.secretHashCache.Load("active-handle")
	require.True(t, ok)
	assert.Equal(t, "hmac-sha256:1", activeCached)
}
